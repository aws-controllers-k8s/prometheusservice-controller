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

RESOURCE_KIND = "loggingconfiguration"
RESOURCE_PLURAL = "loggingconfigurations"

MODIFY_WAIT_AFTER_SECONDS = 10
MAX_WAIT_FOR_SYNCED_MINUTES = 10
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

        assert k8s.wait_on_condition(workspace_ref, "Ready", "True", wait_periods=MAX_WAIT_FOR_SYNCED_MINUTES)
        assert 'workspaceID' in workspace_resource['status']

        yield (workspace_ref, workspace_resource)

        _, deleted = k8s.delete_custom_resource(workspace_ref)
        assert deleted

@service_marker
@pytest.mark.canary
class TestLoggingConfiguration:
    def get_logging_configuration(self, prometheusservice_client, workspace_id: str) -> dict:
        try:
            resp = prometheusservice_client.describe_logging_configuration(
                workspaceId=workspace_id
            )
            return resp

        except Exception as e:
            logging.debug(e)
            return None

    def test_successful_crud_logging_configuration(self, prometheusservice_client, workspace_resource):
        resource_name = random_suffix_name("logging-configuration", 30)

        # Create the workspace where the logging configuration definition will be stored. 
        (_, workspace_res) = workspace_resource
        workspace_id = workspace_res['status']['workspaceID']

        # Set the log group ARN
        log_group_arn = get_bootstrap_resources().LoggingConfigurationLogGroup1.arn

        # Now, load the full CR
        replacements = REPLACEMENT_VALUES.copy()
        replacements['WORKSPACE_ID'] = workspace_id
        replacements['LOGGING_CONFIGURATION_NAME'] = resource_name
        replacements['LOG_GROUP_ARN'] = log_group_arn

        resource_data = load_prometheusservice_resource(
            "logging_configuration",
            additional_replacements=replacements,
        )

        lc_ref = k8s.CustomResourceReference(
            CRD_GROUP, CRD_VERSION, RESOURCE_PLURAL,
            resource_name, namespace="default",
        )

        # Create logging configuration
        k8s.create_custom_resource(lc_ref, resource_data)
        lc_resource = k8s.wait_resource_consumed_by_controller(lc_ref)

        assert k8s.get_resource_exists(lc_ref)
        assert lc_resource is not None
        assert 'status' in lc_resource
        assert 'statusCode' in lc_resource['status']
        assert lc_resource['status']['statusCode'] == 'CREATING'
        assert lc_resource['spec']['workspaceID'] == workspace_id
        assert lc_resource['spec']['logGroupARN'] == log_group_arn
        condition.assert_not_ready(lc_ref)

        # Wait for the resource to get synced
        assert k8s.wait_on_condition(lc_ref, "Ready", "True", wait_periods=MAX_WAIT_FOR_SYNCED_MINUTES)

        # After the resource is synced, assert that workspace is active
        latest = self.get_logging_configuration(prometheusservice_client, lc_resource['spec']['workspaceID'])
        assert latest is not None
        assert latest['loggingConfiguration']['status']['statusCode'] == 'ACTIVE'
        assert latest['loggingConfiguration']['logGroupArn'] == log_group_arn
        assert latest['loggingConfiguration']['workspace'] == workspace_id

        # Before we update the logging configuration CR below, we need to check that the
        # logging configuration status field in the CR has been updated to active,
        # which does not happen right away after the initial creation.
        # The CR's `Status.StatusCode` should be updated because the CR
        # is requeued on successful reconciliation loops and subsequent
        # reconciliation loops call ReadOne and should update the CR's Status
        # with the latest observed information. 
        lc_resource = k8s.get_resource(lc_ref)
        assert lc_resource is not None
        assert 'status' in lc_resource
        assert 'statusCode' in lc_resource['status']
        assert lc_resource['status']['statusCode'] == 'ACTIVE'
        condition.assert_ready(lc_ref)

        # Update the log group ARN
        new_log_group_arn = get_bootstrap_resources().LoggingConfigurationLogGroup2.arn
        updates = {
            "spec": {"logGroupARN": new_log_group_arn},
        }

        res= k8s.patch_custom_resource(lc_ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)
        # wait for the resource to get synced after the patch
        assert k8s.wait_on_condition(lc_ref, "Ready", "True", wait_periods=MAX_WAIT_FOR_SYNCED_MINUTES)
        latest = self.get_logging_configuration(prometheusservice_client, lc_resource['spec']['workspaceID'])
        assert latest is not None
        assert latest['loggingConfiguration']['logGroupArn'] == new_log_group_arn
        assert latest['loggingConfiguration']['workspace'] == workspace_id


        _, deleted = k8s.delete_custom_resource(lc_ref)
        assert deleted
        logging_configuration = self.get_logging_configuration(prometheusservice_client, lc_resource['spec']['workspaceID'])
        assert logging_configuration is None

        time.sleep(DELETE_WAIT_AFTER_SECONDS)