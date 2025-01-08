// Copyright 2023 Redpanda Data, Inc.
//
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package models

import (
	"context"
	"reflect"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Network represents the Terraform schema for the network resource.
type Network struct {
	Name                     types.String                 `tfsdk:"name"`
	ResourceGroupID          types.String                 `tfsdk:"resource_group_id"`
	CloudProvider            types.String                 `tfsdk:"cloud_provider"`
	Region                   types.String                 `tfsdk:"region"`
	CidrBlock                types.String                 `tfsdk:"cidr_block"`
	ID                       types.String                 `tfsdk:"id"`
	ClusterType              types.String                 `tfsdk:"cluster_type"`
	CustomerManagedResources CustomerManagedResourcesType `tfsdk:"customer_managed_resources"`
}

// AWSResources represents the AWS resources that are managed by the customer
type AWSResources struct {
	ManagementBucket *AWSBucket
	DynamoDBTable    *AWSDynamoDBTable
	VPC              *AWSVPC
	PrivateSubnets   *AWSSubnets
	PublicSubnets    *AWSSubnets
}

// AWSBucket contains the ARN of an S3 bucket
type AWSBucket struct {
	ARN string
}

// AWSDynamoDBTable contains the ARN of a DynamoDB table
type AWSDynamoDBTable struct {
	ARN string
}

// AWSVPC contains the ARN of a VPC
type AWSVPC struct {
	ARN string
}

// AWSSubnets contains the ARNs of subnets
type AWSSubnets struct {
	ARNs []string
}
