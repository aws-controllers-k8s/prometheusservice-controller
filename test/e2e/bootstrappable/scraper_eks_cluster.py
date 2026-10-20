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

"""Bootstraps an EKS cluster that an AMP scraper can collect metrics from.

The shared `acktest.bootstrapping.eks.Cluster` helper cannot be used directly:
CreateScraper requires the cluster to have private endpoint access and an
access-entry-capable authentication mode, neither of which that helper sets, and
it always attaches a managed node group. A scraper only needs the cluster, two
subnets in different Availability Zones and a security group in order to reach
ACTIVE, so this bootstrappable skips the node group and the ~40 minutes its
waiter allows for.
"""

import boto3

from dataclasses import dataclass, field
from typing import List, Union

from acktest import resources
from acktest.bootstrapping import Bootstrappable
from acktest.bootstrapping.iam import Role
from acktest.bootstrapping.vpc import VPC


@dataclass
class ScraperEKSCluster(Bootstrappable):
    # Inputs
    name_prefix: str

    # Subresources
    vpc: VPC = field(init=False, default=None)
    cluster_role: Role = field(init=False, default=None)

    # Outputs
    name: Union[str, None] = field(init=False, default=None)
    arn: Union[str, None] = field(init=False, default=None)
    subnet_ids: List[str] = field(init=False, default_factory=lambda: [])
    security_group_ids: List[str] = field(init=False, default_factory=lambda: [])

    def __post_init__(self):
        # Two public subnets land in different AZs, which the scraper requires. The
        # self-referencing ingress rule lets the scraper reach the cluster endpoint.
        self.vpc = VPC(
            f"{self.name_prefix}-vpc",
            num_public_subnet=2,
            security_group_self_referencing_ingress=True,
        )
        self.cluster_role = Role(
            f"{self.name_prefix}-cluster-role",
            "eks.amazonaws.com",
            managed_policies=["arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"],
        )

    @property
    def eks_client(self):
        return boto3.client("eks", region_name=self.region)

    @property
    def ec2_client(self):
        return boto3.client("ec2", region_name=self.region)

    def bootstrap(self):
        super().bootstrap()

        # Private endpoint access resolves through a private hosted zone, so the
        # VPC needs DNS hostnames as well as DNS support.
        self.ec2_client.modify_vpc_attribute(
            VpcId=self.vpc.vpc_id, EnableDnsSupport={"Value": True}
        )
        self.ec2_client.modify_vpc_attribute(
            VpcId=self.vpc.vpc_id, EnableDnsHostnames={"Value": True}
        )

        self.name = resources.random_suffix_name(self.name_prefix, 63)
        self.subnet_ids = self.vpc.public_subnets.subnet_ids
        self.security_group_ids = [self.vpc.security_group.group_id]

        cluster = self.eks_client.create_cluster(
            name=self.name,
            roleArn=self.cluster_role.arn,
            resourcesVpcConfig={
                "subnetIds": self.subnet_ids,
                "securityGroupIds": self.security_group_ids,
                "endpointPrivateAccess": True,
                "endpointPublicAccess": True,
            },
            # API_AND_CONFIG_MAP lets AMP create the scraper's access entry itself,
            # which is the supported alternative to editing the aws-auth ConfigMap.
            accessConfig={"authenticationMode": "API_AND_CONFIG_MAP"},
        )
        self.arn = cluster["cluster"]["arn"]

        self.eks_client.get_waiter("cluster_active").wait(name=self.name)

    def cleanup(self):
        if self.name is not None:
            self.eks_client.delete_cluster(name=self.name)
            self.eks_client.get_waiter("cluster_deleted").wait(name=self.name)

        super().cleanup()
