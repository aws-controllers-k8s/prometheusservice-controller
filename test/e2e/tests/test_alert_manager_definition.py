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

"""Integration tests for the Amazon Managed Prometheus (AMP) Alert Manager Definitions resource
"""

from dataclasses import replace
import logging
import time
import pytest
import yaml

from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_prometheusservice_resource
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e.bootstrap_resources import get_bootstrap_resources
from e2e import condition

RESOURCE_KIND = "alertmanagerdefinition"
RESOURCE_PLURAL = "alertmanagerdefinitions"

CREATE_WAIT_AFTER_SECONDS = 100
MODIFY_WAIT_AFTER_SECONDS = 10
MAX_WAIT_FOR_SYNCED_MINUTES = 10
UPDATE_WAIT_AFTER_SECONDS = 20
DELETE_WAIT_AFTER_SECONDS = 60

@pytest.fixture(scope="module")
def workspace_resource():
        resource_name = random_suffix_name("amp-workspace", 24)
        resources = get_bootstrap_resources()

        replacements = REPLACEMENT_VALUES.copy()
        replacements['WORKSPACE_ALIAS'] = resource_name

        resource_data = load_prometheusservice_resource(
            "workspace",
            additional_replacements=replacements,
        )
        
        workspace_ref = k8s.CustomResourceReference(
            CRD_GROUP, CRD_VERSION, "workspaces",
            resource_name, namespace="default",
        )

        # Create workspace
        k8s.create_custom_resource(workspace_ref, resource_data)
        workspace_resource = k8s.wait_resource_consumed_by_controller(workspace_ref)

        assert workspace_resource is not None
        assert k8s.get_resource_exists(workspace_ref)

        assert k8s.wait_on_condition(workspace_ref, "ACK.ResourceSynced", "True", wait_periods=MAX_WAIT_FOR_SYNCED_MINUTES)
        assert 'workspaceID' in workspace_resource['status']

        yield (workspace_ref, workspace_resource)

        _, deleted = k8s.delete_custom_resource(workspace_ref)
        assert deleted

@service_marker
@pytest.mark.canary
class TestAlertManagerDefinition:
    def get_alert_manager_definition(self, prometheusservice_client, workspace_id: str) -> dict:
        try:
            resp = prometheusservice_client.describe_alert_manager_definition(
                workspaceId=workspace_id
            )
            return resp

        except Exception as e:
            logging.debug(e)
            return None

    def test_successful_crud_alert_manager_definition(self, prometheusservice_client, workspace_resource):
        sns_topic_name = get_bootstrap_resources().AlertManagerSNSTopic.name
        sns_topic_arn = get_bootstrap_resources().AlertManagerSNSTopic.arn
        resource_name = random_suffix_name("alert-manager-definition", 30)

        # Create the workspace where the alert manager definition will be stored. 
        (_, workspace_res) = workspace_resource
        workspace_id = workspace_res['status']['workspaceID']

        # First load the yaml file that is for the alert manager definition that will be used within the resource.
        # This is a valid configuration.
        config_replacements = REPLACEMENT_VALUES.copy()
        config_replacements['SNS_TOPIC_NAME'] = sns_topic_name
        config_replacements['SNS_TOPIC_ARN'] = sns_topic_arn
        configuration_data = load_prometheusservice_resource(
            "alert_manager_configuration",
            additional_replacements=config_replacements,
        )
        # Convert the configuration to a string
        configuration_str = str(yaml.dump(configuration_data))
        # For replacing the value is the main YAML file, we need to indent the configuration 
        configuration_str_indented = configuration_str.replace('\n', '\n    ')

        # Now, load the full CR
        replacements = REPLACEMENT_VALUES.copy()
        replacements['WORKSPACE_ID'] = workspace_id
        replacements['ALERT_MANAGER_DEFINITION_NAME'] = resource_name
        replacements['CONFIGURATION'] = configuration_str_indented

        resource_data = load_prometheusservice_resource(
            "alert_manager_definition",
            additional_replacements=replacements,
        )

        am_ref = k8s.CustomResourceReference(
            CRD_GROUP, CRD_VERSION, RESOURCE_PLURAL,
            resource_name, namespace="default",
        )

        # Create the valid alert manager definition
        k8s.create_custom_resource(am_ref, resource_data)
        am_resource = k8s.wait_resource_consumed_by_controller(am_ref)

        assert k8s.get_resource_exists(am_ref)
        assert am_resource is not None
        assert am_resource['spec']['workspaceID'] == workspace_id
        assert 'configuration' in am_resource['spec']
        assert am_resource['spec']['configuration'] == configuration_str
        condition.assert_not_synced(am_ref)

        assert k8s.wait_on_condition(am_ref, "ACK.ResourceSynced", "True", wait_periods=MAX_WAIT_FOR_SYNCED_MINUTES)

        # After the resource is synced, assert that alert manager definition is active
        latest = self.get_alert_manager_definition(prometheusservice_client, workspace_id)
        assert latest is not None
        assert latest['alertManagerDefinition'] is not None
        assert 'status' in latest['alertManagerDefinition']
        assert 'statusCode' in latest['alertManagerDefinition']['status']
        assert latest['alertManagerDefinition']['status']['statusCode'] == 'ACTIVE'
        assert 'data' in latest['alertManagerDefinition']
        # Since it is base64 encoded, the responding configuration will be in bytes and needs to be converted 
        assert latest['alertManagerDefinition']['data'].decode('UTF-8') == configuration_str

        # The resource status should be updated to ACTIVE instead of CREATING.
        am_resource = k8s.get_resource(am_ref)
        assert am_resource is not None
        assert 'status' in am_resource
        assert 'statusCode' in am_resource['status']
        assert am_resource['status']['statusCode'] == 'ACTIVE'
        condition.assert_synced(am_ref)

        # Now, we update the resource with a new INVALID configuration. 
        # This kind of invalid configuration doesn't result in a validationexcpetion from the http request. 
        # It instead fails the async update. 

        # For the new alert config, change one of the sns topic name from the previous configuration
        # Even if the SNS topic name doesn't exist it won't result in an error
        config_replacements['SNS_TOPIC_NAME'] = sns_topic_name + "-updated"
        configuration_data = load_prometheusservice_resource(
            "alert_manager_configuration",
            additional_replacements=config_replacements,
        )
        # Convert the configuration to a string
        new_alert_config = str(yaml.dump(configuration_data))        
        
        updates = {
            "spec": {"configuration": new_alert_config},
        }

        res= k8s.patch_custom_resource(am_ref, updates)

        # A successful update could take a little while to complete. 
        # As a intermediate step, the status should be updated to "UPDATING"
        # shorly after the update call was made. 
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)
        am_resource = k8s.get_resource(am_ref)
        assert am_resource is not None
        assert 'status' in am_resource
        assert 'statusCode' in am_resource['status']
        assert am_resource['status']['statusCode'] == 'UPDATING'

        assert k8s.wait_on_condition(am_ref, "ACK.ResourceSynced", "True", wait_periods=MAX_WAIT_FOR_SYNCED_MINUTES)

        # After the resource is synced, assert that alert manager is active
        latest = self.get_alert_manager_definition(prometheusservice_client, workspace_id)
        assert latest is not None
        assert 'status' in latest['alertManagerDefinition']
        assert 'statusCode' in latest['alertManagerDefinition']['status']
        assert latest['alertManagerDefinition']['status']['statusCode'] == 'ACTIVE'
        assert 'data' in latest['alertManagerDefinition']
        assert latest['alertManagerDefinition']['data'].decode('UTF-8') == new_alert_config

        # After updating the resource should be back to active 
        am_resource = k8s.get_resource(am_ref)
        assert am_resource is not None
        assert 'status' in am_resource
        assert 'statusCode' in am_resource['status']
        assert am_resource['status']['statusCode'] == 'ACTIVE'
        condition.assert_synced(am_ref)


        # Delete the alert manager definition
        _, deleted = k8s.delete_custom_resource(am_ref)
        assert deleted

        # Verify that it is being deleted on the server side
        latest = self.get_alert_manager_definition(prometheusservice_client, workspace_id)
        assert 'status' in latest['alertManagerDefinition']
        assert 'statusCode' in latest['alertManagerDefinition']['status']
        assert latest['alertManagerDefinition']['status']['statusCode'] == 'DELETING'     

        time.sleep(DELETE_WAIT_AFTER_SECONDS)
        latest = self.get_alert_manager_definition(prometheusservice_client, workspace_id)
        assert latest is None
    
    def test_failed_alert_manager_creation(self, prometheusservice_client, workspace_resource):
        # The resource creation can fail 2 ways:
        #   1) AMP returns an http error right away such as a validationexception
        #   2) successfull HTTP request, alert manager is "CREATING" then a while after status is "CREATION_FAILED"
        #       - Can happen for both internal AMP errors and validation errors.  

        # The first error is a regular exception that the controller handles the same for all controllers. 
        # In this test, we will be testing the 2nd error where the creation doesn't fail right away. 

        sns_topic_name = get_bootstrap_resources().AlertManagerSNSTopic.name
        sns_topic_arn = get_bootstrap_resources().AlertManagerSNSTopic.arn
        resource_name = random_suffix_name("alert-manager-definition", 30)

        # Create the workspace where the alert manager definition will be stored. 
        (_, workspace_res) = workspace_resource
        workspace_id = workspace_res['status']['workspaceID']

        # First load the yaml file that is for the alert manager definition that will be used within the resource 
        config_replacements = REPLACEMENT_VALUES.copy()
        config_replacements['SNS_TOPIC_NAME'] = sns_topic_name
        config_replacements['SNS_TOPIC_ARN'] = sns_topic_arn
        configuration_data = load_prometheusservice_resource(
            "invalid_alert_manager_configuration",
            additional_replacements=config_replacements,
        )
        # Convert the configuration to a string
        configuration_str = str(yaml.dump(configuration_data))
        # For replacing the value is the main YAML file, we need to indent the configuration 
        configuration_str_indented = configuration_str.replace('\n', '\n    ')

        replacements = REPLACEMENT_VALUES.copy()
        replacements['WORKSPACE_ID'] = workspace_id
        replacements['ALERT_MANAGER_DEFINITION_NAME'] = resource_name
        replacements['CONFIGURATION'] = configuration_str_indented

        resource_data = load_prometheusservice_resource(
            "alert_manager_definition",
            additional_replacements=replacements,
        )

        am_ref = k8s.CustomResourceReference(
            CRD_GROUP, CRD_VERSION, RESOURCE_PLURAL,
            resource_name, namespace="default",
        )

        # Create the alert manager definition
        k8s.create_custom_resource(am_ref, resource_data)
        am_resource = k8s.wait_resource_consumed_by_controller(am_ref)

        assert k8s.get_resource_exists(am_ref)
        assert am_resource is not None
        assert 'status' in am_resource
        assert 'statusCode' in am_resource['status']
        assert am_resource['status']['statusCode'] == 'CREATING'
        assert am_resource['spec'] is not None
        assert 'workspaceID' in am_resource['spec']
        assert am_resource['spec']['workspaceID'] == workspace_id
        assert 'configuration' in am_resource['spec']
        assert am_resource['spec']['configuration'] == configuration_str

        condition.assert_not_synced(am_ref)
        assert k8s.wait_on_condition(am_ref, "ACK.ResourceSynced", "True", wait_periods=MAX_WAIT_FOR_SYNCED_MINUTES)

        # After the resource is synced, assert that workspace is active
        latest = self.get_alert_manager_definition(prometheusservice_client, workspace_id)
        assert latest is not None
        assert latest['alertManagerDefinition'] is not None
        assert 'status' in latest['alertManagerDefinition']
        assert 'statusCode' in latest['alertManagerDefinition']['status']
        assert latest['alertManagerDefinition']['status']['statusCode'] == 'CREATION_FAILED'
        # At this point we don't expect the latest server response to have a configuration that is the same 
        # as defined in the spec because the creation is failed, and the API would set the config to be nil. 
        # We treat it the same as a regular validation exception where the spec remains the same as desired.

        # The status field for the resource should also be updated to creation failed
        am_resource = k8s.get_resource(am_ref)
        assert am_resource is not None
        assert 'status' in am_resource
        assert 'statusCode' in am_resource['status']
        assert am_resource['status']['statusCode'] == 'CREATION_FAILED'
        condition.assert_synced(am_ref)

    
        # Next, we want to update it to a valid configuration.
        # Load configuration
        config_replacements = REPLACEMENT_VALUES.copy()
        config_replacements['SNS_TOPIC_NAME'] = sns_topic_name
        config_replacements['SNS_TOPIC_ARN'] = sns_topic_arn
        configuration_data = load_prometheusservice_resource(
            "alert_manager_configuration",
            additional_replacements=config_replacements,
        )
        # Convert the configuration to a string
        configuration_str = str(yaml.dump(configuration_data))

        updates = {
            "spec": {"configuration": configuration_str},
        }

        res= k8s.patch_custom_resource(am_ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(am_ref, "ACK.ResourceSynced", "True", wait_periods=MAX_WAIT_FOR_SYNCED_MINUTES)

        # After the resource is synced, assert that workspace is active
        latest = self.get_alert_manager_definition(prometheusservice_client, workspace_id)
        assert latest is not None
        assert 'status' in latest['alertManagerDefinition']
        assert 'statusCode' in latest['alertManagerDefinition']['status']
        assert latest['alertManagerDefinition']['status']['statusCode'] == 'ACTIVE'
        assert 'data' in latest['alertManagerDefinition']
        # Now that the configuration is valid, the server side and desired resource should match.
        assert latest['alertManagerDefinition']['data'].decode('UTF-8') == configuration_str

        # Clean up
        _, deleted = k8s.delete_custom_resource(am_ref)
        assert deleted


    def test_failed_alert_manager_update(self, prometheusservice_client, workspace_resource):
        # Similar to the failed creation, the update can fail 2 ways
        #   1) AMP returns an http error right away such as a validationexception.
        #   2) successfull HTTP request, alert manager is "UPDATING" then a while after status is "UPDATE_FAILED"
        #       - Can happen for both internal AMP errors and validation errors.  
        
        # The first error is a regular exception that the controller handles the same for all controllers. 
        # In this test, we will be testing the 2nd error where the update doesn't fail right away. 
 
        sns_topic_name = get_bootstrap_resources().AlertManagerSNSTopic.name
        sns_topic_arn = get_bootstrap_resources().AlertManagerSNSTopic.arn
        resource_name = random_suffix_name("alert-manager-definition", 30)


        # Create the workspace where the alert manager definition will be stored. 
        (_, workspace_res) = workspace_resource
        workspace_id = workspace_res['status']['workspaceID']
        
        # First load the yaml file that is for the alert manager definition that will be used within the resource.
        config_replacements = REPLACEMENT_VALUES.copy()
        config_replacements['SNS_TOPIC_NAME'] = sns_topic_name
        config_replacements['SNS_TOPIC_ARN'] = sns_topic_arn
        configuration_data = load_prometheusservice_resource(
            "alert_manager_configuration",
            additional_replacements=config_replacements,
        )
        # Convert the configuration to a string
        configuration_str = str(yaml.dump(configuration_data))
        # For replacing the value is the main YAML file, we need to indent the configuration 
        configuration_str_indented = configuration_str.replace('\n', '\n    ')

        replacements = REPLACEMENT_VALUES.copy()
        replacements['WORKSPACE_ID'] = workspace_id
        replacements['ALERT_MANAGER_DEFINITION_NAME'] = resource_name
        replacements['CONFIGURATION'] = configuration_str_indented
           
        resource_data = load_prometheusservice_resource(
            "alert_manager_definition",
            additional_replacements=replacements,
        )

        am_ref = k8s.CustomResourceReference(
            CRD_GROUP, CRD_VERSION, RESOURCE_PLURAL,
            resource_name, namespace="default",
        )

        k8s.create_custom_resource(am_ref, resource_data)
        am_resource = k8s.wait_resource_consumed_by_controller(am_ref)

        assert k8s.get_resource_exists(am_ref)
        assert am_resource is not None
        assert 'status' in am_resource
        assert 'statusCode' in am_resource['status']
        assert am_resource['status']['statusCode'] == 'CREATING'
        assert am_resource['spec'] is not None
        assert 'workspaceID' in am_resource['spec']
        assert am_resource['spec']['workspaceID'] == workspace_id
        condition.assert_not_synced(am_ref)

        assert k8s.wait_on_condition(am_ref, "ACK.ResourceSynced", "True", wait_periods=MAX_WAIT_FOR_SYNCED_MINUTES)

        # After the resource is synced, assert that workspace is active
        latest = self.get_alert_manager_definition(prometheusservice_client, workspace_id)
        assert latest is not None
        assert latest['alertManagerDefinition'] is not None
        assert 'status' in latest['alertManagerDefinition']
        assert 'statusCode' in latest['alertManagerDefinition']['status']
        assert latest['alertManagerDefinition']['status']['statusCode'] == 'ACTIVE'
        assert 'data' in latest['alertManagerDefinition']
        assert latest['alertManagerDefinition']['data'].decode('UTF-8') == configuration_str


        # Verify that the resource was also updated to active status
        am_resource = k8s.get_resource(am_ref)
        assert am_resource is not None
        assert 'status' in am_resource
        assert 'statusCode' in am_resource['status']
        assert am_resource['status']['statusCode'] == 'ACTIVE'
        condition.assert_synced(am_ref)


        # To make the update, first load the invalid configuration
        config_replacements = REPLACEMENT_VALUES.copy()
        config_replacements['SNS_TOPIC_NAME'] = sns_topic_name
        config_replacements['SNS_TOPIC_ARN'] = sns_topic_arn
        configuration_data = load_prometheusservice_resource(
            "invalid_alert_manager_configuration",
            additional_replacements=config_replacements,
        )
        # Convert the configuration to a string
        invalid_configuration_str = str(yaml.dump(configuration_data))


        updates = {
            "spec": {"configuration": invalid_configuration_str},
        }

        k8s.patch_custom_resource(am_ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        assert k8s.wait_on_condition(am_ref, "ACK.ResourceSynced", "True", wait_periods=MAX_WAIT_FOR_SYNCED_MINUTES)

        latest = self.get_alert_manager_definition(prometheusservice_client, workspace_id)
        assert latest is not None
        assert 'status' in latest['alertManagerDefinition']
        assert 'statusCode' in latest['alertManagerDefinition']['status']
        assert latest['alertManagerDefinition']['status']['statusCode'] == 'UPDATE_FAILED'
        # At this point, the latest will not have the invalid configuration, so we don't 
        # check that its equal to the invalid configuration. Since AMP only stores 
        # the most recent valid configuration. 

        # Update with a valid configuration
        updates = {
            "spec": {"configuration": configuration_str},
        }

        k8s.patch_custom_resource(am_ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)
        assert k8s.wait_on_condition(am_ref, "ACK.ResourceSynced", "True", wait_periods=MAX_WAIT_FOR_SYNCED_MINUTES)

        # After the resource is synced, assert that information matches
        latest = self.get_alert_manager_definition(prometheusservice_client, workspace_id)
        assert latest is not None
        assert 'status' in latest['alertManagerDefinition']
        assert 'statusCode' in latest['alertManagerDefinition']['status']
        assert latest['alertManagerDefinition']['status']['statusCode'] == 'ACTIVE'
        assert 'data' in latest['alertManagerDefinition']
        assert latest['alertManagerDefinition']['data'].decode('UTF-8') == configuration_str

        _, deleted = k8s.delete_custom_resource(am_ref)
        assert deleted

    def test_creating_two_alert_manager_for_one_workspace(self, prometheusservice_client, workspace_resource):
        # There can only be one alert manager definition per workspace. 
        # If two are created, the second resource should result in a terminal error. 
   
        sns_topic_name = get_bootstrap_resources().AlertManagerSNSTopic.name
        sns_topic_arn = get_bootstrap_resources().AlertManagerSNSTopic.arn
        resource_name = random_suffix_name("alert-manager-definition", 30)

        # Create the workspace where the alert manager definition will be stored. 
        (_, workspace_res) = workspace_resource
        workspace_id = workspace_res['status']['workspaceID']

        # First load the yaml file that is for the alert manager definition that will be used within the resource 
        config_replacements = REPLACEMENT_VALUES.copy()
        config_replacements['SNS_TOPIC_NAME'] = sns_topic_name
        config_replacements['SNS_TOPIC_ARN'] = sns_topic_arn
        configuration_data = load_prometheusservice_resource(
            "alert_manager_configuration",
            additional_replacements=config_replacements,
        )
        # Convert the configuration to a string
        configuration_str = str(yaml.dump(configuration_data))
        # For replacing the value is the main YAML file, we need to indent the configuration 
        configuration_str_indented = configuration_str.replace('\n', '\n    ')


        replacements = REPLACEMENT_VALUES.copy()
        replacements['WORKSPACE_ID'] = workspace_id
        replacements['ALERT_MANAGER_DEFINITION_NAME'] = resource_name
        replacements['CONFIGURATION'] = configuration_str_indented
        resource_data = load_prometheusservice_resource(
            "alert_manager_definition",
            additional_replacements=replacements,
        )

        am_ref_1 = k8s.CustomResourceReference(
            CRD_GROUP, CRD_VERSION, RESOURCE_PLURAL,
            resource_name, namespace="default",
        )

        # Create an alert manager definition
        k8s.create_custom_resource(am_ref_1, resource_data)
        am_resource = k8s.wait_resource_consumed_by_controller(am_ref_1)

        assert k8s.get_resource_exists(am_ref_1)
        assert am_resource is not None
        assert 'status' in am_resource
        assert 'statusCode' in am_resource['status']
        assert am_resource['status']['statusCode'] == 'CREATING'
        assert am_resource['spec'] is not None
        assert 'workspaceID' in am_resource['spec']
        assert am_resource['spec']['workspaceID'] == workspace_id
        condition.assert_not_synced(am_ref_1)

        # Create the second definition (Same workspace ID)
        replacements['ALERT_MANAGER_DEFINITION_NAME'] = resource_name + '-new'
        resource_data = load_prometheusservice_resource(
            "alert_manager_definition",
            additional_replacements=replacements,
        )
        am_ref_2 = k8s.CustomResourceReference(
            CRD_GROUP, CRD_VERSION, RESOURCE_PLURAL,
            resource_name + '-new', namespace="default",
        )

        k8s.create_custom_resource(am_ref_2, resource_data)
        am_resource = k8s.wait_resource_consumed_by_controller(am_ref_2)

        assert k8s.get_resource_exists(am_ref_1)
        assert k8s.get_resource_exists(am_ref_2)
        
        time.sleep(CREATE_WAIT_AFTER_SECONDS)

        condition.assert_synced(am_ref_1)

        # The second resource should have a terminal error
        condition.assert_type_status(am_ref_2, condition.CONDITION_TYPE_TERMINAL, True)
        condition.assert_not_synced(am_ref_2)

        _, deleted = k8s.delete_custom_resource(am_ref_1)
        assert deleted

        _, deleted = k8s.delete_custom_resource(am_ref_2)
        assert deleted