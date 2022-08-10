# Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License"). You may
# not use this file except in compliance with the License. A copy of the
# License is located at
#
#	 http://aws.amazon.com/apache2.0/
#
# or in the "license" file accompanying this file. This file is distributed
# on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
# express or implied. See the License for the specific language governing
# permissions and limitations under the License.

"""Integration tests for the Amazon Managed Prometheus (AMP) Rule Groups Namespace resource
"""

import logging
import time
import pytest

from acktest.k8s import resource as k8s
from acktest.k8s import condition
from acktest import tags as tags
from acktest.resources import random_suffix_name
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_prometheusservice_resource
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e.bootstrap_resources import get_bootstrap_resources


RESOURCE_KIND = "rulegroupsnamespace"
RESOURCE_PLURAL = "rulegroupsnamespaces"


MAX_WAIT_FOR_SYNCED_MINUTES = 10
UPDATE_WAIT_AFTER_SECONDS = 5
DELETE_WAIT_AFTER_SECONDS = 60
CREATE_WAIT_AFTER_SECONDS = 90


@service_marker
@pytest.mark.canary
class TestRuleGroupsNamespace:

    def create_workspace(self, prometheusservice_client, workspace_alias) -> str:
        try:
            resp = prometheusservice_client.create_workspace(
                alias=workspace_alias
            )
            workspace_id = resp['workspaceId']

            return workspace_id

        except Exception as e:
            logging.debug(e)
            return None

    def delete_workspace(self, prometheusservice_client, workspace_id):
        try:
            response = prometheusservice_client.delete_workspace(
                workspaceId=workspace_id
            )
            return response
        except Exception as e:
            logging.debug(e)
            return None

    def get_rule_groups_namespace(self, prometheusservice_client, workspace_id: str, name: str) -> dict:
        try:
            resp = prometheusservice_client.describe_rule_groups_namespace(
                workspaceId=workspace_id,
                name=name
            )
            return resp

        except Exception as e:
            logging.debug(e)
            return None


    def assert_server_side_status(self, result_from_server, status):
        assert 'status' in result_from_server['ruleGroupsNamespace']
        assert 'statusCode' in result_from_server['ruleGroupsNamespace']['status']
        assert result_from_server['ruleGroupsNamespace']['status']['statusCode'] == status


    def test_successful_crud_rule_groups_namespace(self, prometheusservice_client):
        workspace_alias = random_suffix_name("amp-workspace", 24) 
        resource_name = random_suffix_name("rule-groups-namespace", 30)

        # Create the workspace where the rule groups will be stored. 
        workspace_id = self.create_workspace(prometheusservice_client, workspace_alias)

        # Below is the rule groups configuration that is part of the yaml resource file
        expected_rule_group = '''\
groups:
- name: test
  rules:
  - record: metric:recording_rule
    expr: avg(rate(container_cpu_usage_seconds_total[5m]))
- name: alert-test
  rules:
  - alert: metric:alerting_rule
    expr: avg(rate(container_cpu_usage_seconds_total[5m])) > 0
    for: 2m
'''

        replacements = REPLACEMENT_VALUES.copy()
        replacements['WORKSPACE_ID'] = workspace_id
        replacements['RESOURCE_NAME'] = resource_name
        replacements['RULE_GROUPS_NAME'] = resource_name

        resource_data = load_prometheusservice_resource(
            "rule_groups_namespace",
            additional_replacements=replacements,
        )
     
        rule_ref = k8s.CustomResourceReference(
            CRD_GROUP, CRD_VERSION, RESOURCE_PLURAL,
            resource_name, namespace="default",
        )

        k8s.create_custom_resource(rule_ref, resource_data)
        resource = k8s.wait_resource_consumed_by_controller(rule_ref)


        assert k8s.get_resource_exists(rule_ref)
        assert resource is not None
        assert 'status' in resource
        assert 'status' in resource['status']
        assert 'statusCode' in resource['status']['status']
        assert resource['status']['status']['statusCode'] == 'CREATING'
        assert resource['spec'] is not None
        assert 'workspaceID' in resource['spec']
        assert resource['spec']['workspaceID'] == workspace_id
        assert resource['spec']['configuration'] == expected_rule_group
        condition.assert_not_synced(rule_ref)


        assert k8s.wait_on_condition(rule_ref, "ACK.ResourceSynced", "True", wait_periods=MAX_WAIT_FOR_SYNCED_MINUTES)

        # Before we update the rule group CR below, we need to check that the
        # rule groups status field in the CR has been updated to active,
        # which does not happen right away after the initial creation.
        # The CR's `Status.Status.StatusCode` should be updated because the CR
        # is requeued on successful reconciliation loops and subsequent
        # reconciliation loops call ReadOne and should update the CR's Status
        # with the latest observed information. 
        resource = k8s.get_resource(rule_ref)
        assert resource is not None
        assert 'status' in resource
        assert 'status' in resource['status']
        assert 'statusCode' in resource['status']['status']
        assert resource['status']['status']['statusCode'] == 'ACTIVE'
        condition.assert_synced(rule_ref)


        # Next, we verify that the AMP server-side rule groups values are the same as
        # defined in the CR. Afterwards, we modify the spec and verify that the AMP 
        # server-side resource shows the new value of the field. 
        latest = self.get_rule_groups_namespace(prometheusservice_client, workspace_id, resource_name)
        assert latest is not None
        assert latest['ruleGroupsNamespace'] is not None
        assert 'data' in latest['ruleGroupsNamespace']
        assert latest['ruleGroupsNamespace']['data'].decode('utf-8') == expected_rule_group
        assert 'name' in latest['ruleGroupsNamespace']
        assert latest['ruleGroupsNamespace']['name'] == resource_name
        assert latest['ruleGroupsNamespace']['tags']['k1'] == 'v1'
        assert latest['ruleGroupsNamespace']['tags']['k2'] == 'v2'
        self.assert_server_side_status(latest, 'ACTIVE')

        # First, we will perform an update that includes changing the configuration. This results  
        # in an update that is performed asynchronously. When the call is made, the status is first 
        # updated to `UPDATING` and then should resync until `ACTIVE`.
        updated_rule_group = '''\
groups:
- name: test
  rules:
  - record: metric:recording_rule
    expr: avg(rate(container_cpu_usage_seconds_total[5m]))
- name: alert-updated
  rules:
  - alert: metric:alerting_rule
    expr: avg(rate(container_cpu_usage_seconds_total[5m])) > 0
    for: 10m
'''

        tag_update = {
            "k1": "v1_updated",
            "k2": None,
            "k3": "v3",
        }

        expected_tags = {
            "k1": "v1_updated",
            "k3": "v3",
        }

        updates = {
            "spec": {"configuration": updated_rule_group, "tags": tag_update},
        }

        k8s.patch_custom_resource(rule_ref, updates)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        resource = k8s.get_resource(rule_ref)
        assert resource is not None
        assert 'status' in resource
        assert 'status' in resource['status']
        assert 'statusCode' in resource['status']['status']
        assert resource['status']['status']['statusCode'] == 'UPDATING'

        # Wait until the update finishes
        assert k8s.wait_on_condition(rule_ref, "ACK.ResourceSynced", "True", wait_periods=MAX_WAIT_FOR_SYNCED_MINUTES)

        # Verify that the server side resource matches after the updates. 
        latest = self.get_rule_groups_namespace(prometheusservice_client, workspace_id, resource_name)
        assert latest is not None
        assert latest['ruleGroupsNamespace'] is not None
        assert 'data' in latest['ruleGroupsNamespace']
        assert latest['ruleGroupsNamespace']['data'].decode('utf-8') == updated_rule_group
        tags.assert_equal_without_ack_tags(latest['ruleGroupsNamespace']['tags'],expected_tags)
        self.assert_server_side_status(latest, 'ACTIVE')

        # When performing an update to anything except the configuration, the update should
        # be instant. If successful, the status should never change from ACTIVE to anything else. 
        # For this update, we will remove all the tags. 
        tag_update = {
            "k1": None,
            "k3": None,
        }
        
        expected_tags = {}

        updates = {
            "spec": {"tags":  tag_update},
        }
        k8s.patch_custom_resource(rule_ref, updates)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)
        condition.assert_synced(rule_ref)

        # After resource is synced again, assert that patches are reflected in the AWS resource
        latest = self.get_rule_groups_namespace(prometheusservice_client, workspace_id, resource_name)
        assert latest is not None
        tags.assert_equal_without_ack_tags((latest['ruleGroupsNamespace']['tags']), expected_tags)
        self.assert_server_side_status(latest, 'ACTIVE')


        # Delete the resource
        _, deleted = k8s.delete_custom_resource(rule_ref)
        assert deleted
        # Deletion can take some time. First the status will be in `Deleting` state and eventually should
        # be deleted. 
        latest = self.get_rule_groups_namespace(prometheusservice_client, workspace_id, resource_name)
        self.assert_server_side_status(latest, 'DELETING')
        time.sleep(DELETE_WAIT_AFTER_SECONDS)
        latest = self.get_rule_groups_namespace(prometheusservice_client, workspace_id, resource_name)
        assert latest is None

        # Clean-up the workspace used for the test.  
        self.delete_workspace(prometheusservice_client, workspace_id)

    
    def test_creating_two_namespaces_with_same_name(self, prometheusservice_client):
        # The resource does not allow for two rule groups namespaces to have the same name.
        # If two are created with the same name, the second resource should result in an error. 
        workspace_alias = random_suffix_name("amp-workspace", 24) 
        resource_name = random_suffix_name("rule-groups-namespace", 30)

        # Create the workspace where the rule groups will be stored. 
        workspace_id = self.create_workspace(prometheusservice_client, workspace_alias)

        replacements = REPLACEMENT_VALUES.copy()
        replacements['WORKSPACE_ID'] = workspace_id
        replacements['RESOURCE_NAME'] = resource_name
        replacements['RULE_GROUPS_NAME'] = resource_name

        resource_data = load_prometheusservice_resource(
            "rule_groups_namespace",
            additional_replacements=replacements,
        )
     
        rule_ref_1 = k8s.CustomResourceReference(
            CRD_GROUP, CRD_VERSION, RESOURCE_PLURAL,
            resource_name, namespace="default",
        )
             
        k8s.create_custom_resource(rule_ref_1, resource_data)
        resource = k8s.wait_resource_consumed_by_controller(rule_ref_1)

        # Validate that the first one is created successfully 
        assert k8s.get_resource_exists(rule_ref_1)
        assert resource is not None
        assert 'status' in resource
        assert 'status' in resource['status']
        assert 'statusCode' in resource['status']['status']
        assert resource['status']['status']['statusCode'] == 'CREATING'
        assert resource['spec'] is not None
        assert 'workspaceID' in resource['spec']
        assert resource['spec']['workspaceID'] == workspace_id
        condition.assert_not_synced(rule_ref_1)

        # The second resource
        new_resource_name = resource_name + "-new"
        replacements['RESOURCE_NAME'] = new_resource_name
        resource_data = load_prometheusservice_resource(
            "rule_groups_namespace",
            additional_replacements=replacements,
        )
     
        rule_ref_2 = k8s.CustomResourceReference(
            CRD_GROUP, CRD_VERSION, RESOURCE_PLURAL,
            new_resource_name, namespace="default",
        )

        # Create the second resource with the same Spec.Name field
        k8s.create_custom_resource(rule_ref_2, resource_data)
        k8s.wait_resource_consumed_by_controller(rule_ref_2)
        assert k8s.get_resource_exists(rule_ref_2)
    
        # The second resource should be in terminal status and not synced because 
        # This resource already exists but is not managed by this CR.
        condition.assert_type_status(rule_ref_2, condition.CONDITION_TYPE_TERMINAL, True)
        condition.assert_not_synced(rule_ref_2)

        time.sleep(CREATE_WAIT_AFTER_SECONDS)

        # The original resource should still be synced
        condition.assert_synced(rule_ref_1)
        
        # The second resource should remain in terminal error and not synced. 
        condition.assert_type_status(rule_ref_2, condition.CONDITION_TYPE_TERMINAL, True)
        condition.assert_not_synced(rule_ref_2)

        # Clean up the resource
        _, deleted = k8s.delete_custom_resource(rule_ref_1)
        assert deleted

        _, deleted = k8s.delete_custom_resource(rule_ref_2)
        assert deleted

        # Clean-up the workspace used for the test.  
        self.delete_workspace(prometheusservice_client, workspace_id)