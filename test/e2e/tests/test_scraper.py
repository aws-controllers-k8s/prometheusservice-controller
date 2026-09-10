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

"""Integration tests for the Amazon Managed Prometheus (AMP) Scraper resource
"""

import json
import logging
import time
import pytest

from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_prometheusservice_resource
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e.bootstrap_resources import get_bootstrap_resources

RESOURCE_PLURAL = "scrapers"

MAX_WAIT_FOR_SYNCED_MINUTES = 20
CREATE_WAIT_AFTER_SECONDS = 30
UPDATE_WAIT_AFTER_SECONDS = 30
DELETE_WAIT_AFTER_SECONDS = 30


@pytest.fixture(scope="module")
def workspace_resource():
    """The scraper's destination workspace."""
    resource_name = random_suffix_name("amp-scraper-ws", 24)

    replacements = REPLACEMENT_VALUES.copy()
    replacements["WORKSPACE_ALIAS"] = resource_name

    resource_data = load_prometheusservice_resource(
        "workspace",
        additional_replacements=replacements,
    )

    ref = k8s.CustomResourceReference(
        CRD_GROUP, CRD_VERSION, "workspaces",
        resource_name, namespace="default",
    )
    k8s.create_custom_resource(ref, resource_data)
    resource = k8s.wait_resource_consumed_by_controller(ref)
    assert resource is not None
    assert k8s.wait_on_condition(
        ref, "ACK.ResourceSynced", "True", wait_periods=MAX_WAIT_FOR_SYNCED_MINUTES
    )

    yield (ref, resource_name)

    _, deleted = k8s.delete_custom_resource(ref)
    assert deleted


@pytest.fixture(scope="module")
def scraper(workspace_resource):
    _, workspace_name = workspace_resource
    cluster = get_bootstrap_resources().ScraperEKSCluster

    resource_name = random_suffix_name("amp-scraper", 24)

    replacements = REPLACEMENT_VALUES.copy()
    replacements["SCRAPER_ALIAS"] = resource_name
    replacements["WORKSPACE_NAME"] = workspace_name
    replacements["EKS_CLUSTER_ARN"] = cluster.arn
    replacements["EKS_SUBNET_IDS"] = json.dumps(cluster.subnet_ids)
    replacements["EKS_SECURITY_GROUP_IDS"] = json.dumps(cluster.security_group_ids)

    resource_data = load_prometheusservice_resource(
        "scraper",
        additional_replacements=replacements,
    )

    ref = k8s.CustomResourceReference(
        CRD_GROUP, CRD_VERSION, RESOURCE_PLURAL,
        resource_name, namespace="default",
    )
    k8s.create_custom_resource(ref, resource_data)
    resource = k8s.wait_resource_consumed_by_controller(ref)
    assert resource is not None

    yield (ref, resource_name)

    if k8s.get_resource_exists(ref):
        _, deleted = k8s.delete_custom_resource(ref)
        assert deleted


@service_marker
@pytest.mark.canary
class TestScraper:
    def describe_scraper(self, prometheusservice_client, scraper_id: str):
        try:
            return prometheusservice_client.describe_scraper(scraperId=scraper_id)[
                "scraper"
            ]
        except Exception as e:
            logging.debug(e)
            return None

    def test_crud_scraper(self, prometheusservice_client, scraper):
        ref, resource_name = scraper

        assert k8s.wait_on_condition(
            ref, "ACK.ResourceSynced", "True", wait_periods=MAX_WAIT_FOR_SYNCED_MINUTES
        )

        resource = k8s.get_resource(ref)
        scraper_id = resource["status"]["scraperID"]
        assert scraper_id is not None
        assert resource["status"]["status"]["statusCode"] == "ACTIVE"
        # Only DescribeScraper returns these, so they prove the read-only status
        # fields are wired up.
        assert resource["status"]["roleARN"] is not None

        latest = self.describe_scraper(prometheusservice_client, scraper_id)
        assert latest is not None
        assert latest["alias"] == resource_name
        assert latest["source"]["eksConfiguration"]["clusterArn"] == (
            get_bootstrap_resources().ScraperEKSCluster.arn
        )
        assert latest["tags"]["k1"] == "v1"

        # The configuration is stored as a base64 blob by AWS but surfaced as a
        # plain string, so the round trip must not report drift.
        assert "scrape_interval: 30s" in resource["spec"]["configuration"]
        assert b"scrape_interval: 30s" in latest["scrapeConfiguration"][
            "configurationBlob"
        ]

        # Update the alias and the configuration.
        new_alias = resource_name + "-updated"
        updates = {
            "spec": {
                "alias": new_alias,
                "configuration": "global:\n  scrape_interval: 60s\nscrape_configs: []\n",
            }
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)
        assert k8s.wait_on_condition(
            ref, "ACK.ResourceSynced", "True", wait_periods=MAX_WAIT_FOR_SYNCED_MINUTES
        )

        latest = self.describe_scraper(prometheusservice_client, scraper_id)
        assert latest["alias"] == new_alias
        assert b"scrape_interval: 60s" in latest["scrapeConfiguration"][
            "configurationBlob"
        ]

        # Update the tags, which are reconciled through TagResource/UntagResource
        # rather than UpdateScraper.
        k8s.patch_custom_resource(ref, {"spec": {"tags": {"k1": "v1updated"}}})
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)
        assert k8s.wait_on_condition(
            ref, "ACK.ResourceSynced", "True", wait_periods=MAX_WAIT_FOR_SYNCED_MINUTES
        )
        latest = self.describe_scraper(prometheusservice_client, scraper_id)
        assert latest["tags"]["k1"] == "v1updated"
        assert "k2" not in latest["tags"]

        # UpdateScraper accepts no source, so changing it must go terminal rather
        # than reconcile forever.
        k8s.patch_custom_resource(
            ref,
            {
                "spec": {
                    "source": {
                        "eksConfiguration": {
                            "clusterARN": get_bootstrap_resources().ScraperEKSCluster.arn,
                            "subnetIDs": get_bootstrap_resources().ScraperEKSCluster.subnet_ids,
                            "securityGroupIDs": [],
                        }
                    }
                }
            },
        )
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)
        assert k8s.wait_on_condition(
            ref, "ACK.Terminal", "True", wait_periods=MAX_WAIT_FOR_SYNCED_MINUTES
        )
        # The source must not have changed in AWS.
        latest = self.describe_scraper(prometheusservice_client, scraper_id)
        assert latest["source"]["eksConfiguration"]["securityGroupIds"] == (
            get_bootstrap_resources().ScraperEKSCluster.security_group_ids
        )

        _, deleted = k8s.delete_custom_resource(ref)
        assert deleted
        time.sleep(DELETE_WAIT_AFTER_SECONDS)
        assert not k8s.get_resource_exists(ref)
