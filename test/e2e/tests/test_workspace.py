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

"""Integration tests for the Amazon Managed Prometheus (AMP) Workspace resource
"""

import logging
import time
import pytest

from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_prometheusservice_resource
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e.bootstrap_resources import get_bootstrap_resources
from e2e import condition


RESOURCE_KIND = "Workspace"
RESOURCE_PLURAL = "workspaces"

CREATE_WAIT_AFTER_SECONDS = 30
MODIFY_WAIT_AFTER_SECONDS = 10
MAX_WAIT_FOR_SYNCED_MINUTES = 10

@service_marker
@pytest.mark.canary
class TestWorkspace:

    def get_workspace(self, prometheusservice_client, workspaceID: str) -> dict:
        try:
            resp = prometheusservice_client.describe_workspace(
                workspaceId=workspaceID
            )
            return resp

        except Exception as e:
            logging.debug(e)
            return None
    
    def test_crud_workspace(self, prometheusservice_client):
        resource_name = random_suffix_name("amp-workspace", 24)
        resources = get_bootstrap_resources()

        replacements = REPLACEMENT_VALUES.copy()
        replacements['WORKSPACE_ALIAS'] = resource_name

        resource_data = load_prometheusservice_resource(
            "workspace",
            additional_replacements=replacements,
        )
        
        workspace_ref = k8s.CustomResourceReference(
            CRD_GROUP, CRD_VERSION, RESOURCE_PLURAL,
            resource_name, namespace="default",
        )

        # Create workspace
        k8s.create_custom_resource(workspace_ref, resource_data)
        workspace_resource = k8s.wait_resource_consumed_by_controller(workspace_ref)


        assert k8s.get_resource_exists(workspace_ref)
        assert workspace_resource is not None
        assert 'status' in workspace_resource
        assert 'status' in workspace_resource['status']
        assert 'statusCode' in workspace_resource['status']['status']
        assert workspace_resource['status']['status']['statusCode'] == 'CREATING'
        assert 'workspaceID' in workspace_resource['status']
        condition.assert_not_synced(workspace_ref)

        # Wait for the resource to get synced
        assert k8s.wait_on_condition(workspace_ref, "ACK.ResourceSynced", "True", wait_periods=MAX_WAIT_FOR_SYNCED_MINUTES)

        # After the resource is synced, assert that workspace is active
        latest = self.get_workspace(prometheusservice_client, workspace_resource['status']['workspaceID'])
        assert latest is not None
        print(latest, flush=True)
        assert latest['workspace']['status']['statusCode'] == 'ACTIVE'
        print(latest, flush=True)

        # Before we update the workspace CR below, we need to check that the
        # workspace status field in the CR has been updated to active,
        # which does not happen right away after the initial creation.
        # The CR's `Status.Status.StatusCode` should be updated because the CR
        # is requeued on successful reconciliation loops and subsequent
        # reconciliation loops call ReadOne and should update the CR's Status
        # with the latest observed information. 
        workspace_resource = k8s.get_resource(workspace_ref)
        assert workspace_resource is not None
        assert 'status' in workspace_resource
        assert 'status' in workspace_resource['status']
        assert 'statusCode' in workspace_resource['status']['status']
        assert workspace_resource['status']['status']['statusCode'] == 'ACTIVE'
        condition.assert_synced(workspace_ref)

        # Next, we verify that the AMP server-side workspace values are the same as
        # defined in the CR. Afterwards, we modify the spec and verify that the AMP 
        # server-side resource shows the new value of the field. 
        latest = self.get_workspace(prometheusservice_client, workspace_resource['status']['workspaceID'])
        assert latest is not None
        assert latest['workspace']['alias'] == resource_name
        assert latest['workspace']['tags']['k1'] == 'v1'
        assert latest['workspace']['tags']['k2'] == 'v2'
        assert 'k3' not in latest['workspace']['tags']
    
        new_alias = resource_name + "_updated"
        
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
            "spec": {"alias": new_alias, "tags":  tag_update},
        }

        k8s.patch_custom_resource(workspace_ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        # wait for the resource to get synced after the patch
        assert k8s.wait_on_condition(workspace_ref, "ACK.ResourceSynced", "True", wait_periods=MAX_WAIT_FOR_SYNCED_MINUTES)
        latest = self.get_workspace(prometheusservice_client, workspace_resource['status']['workspaceID'])
        assert latest is not None
        assert latest['workspace']['alias'] == new_alias
        assert latest['workspace']['tags'] == expected_tags


        # Next we update the tags again, but this time we try to remove all tags
        tag_update = {
            "k1": None,
            "k3": None,
        }
        
        expected_tags = {}

        updates = {
            "spec": {"tags":  tag_update},
        }
        k8s.patch_custom_resource(workspace_ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        # wait for the resource to get synced after the patch
        assert k8s.wait_on_condition(workspace_ref, "ACK.ResourceSynced", "True", wait_periods=MAX_WAIT_FOR_SYNCED_MINUTES)

        # After resource is synced again, assert that patches are reflected in the AWS resource
        latest = self.get_workspace(prometheusservice_client, workspace_resource['status']['workspaceID'])
        assert latest is not None
        assert latest['workspace']['tags'] == expected_tags


        _, deleted = k8s.delete_custom_resource(workspace_ref)
        assert deleted


   