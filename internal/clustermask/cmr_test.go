// Copyright 2026 Redpanda Data, Inc.
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

package clustermask

import (
	"reflect"
	"testing"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

func gcpCMR(mutate func(*controlplanev1.CustomerManagedResourcesUpdate_GCP)) *controlplanev1.CustomerManagedResourcesUpdate {
	gcp := &controlplanev1.CustomerManagedResourcesUpdate_GCP{
		PscNatSubnetName:         "nat-a",
		RpsqlSecretManagerPrefix: "tfrp-a",
		RpsqlApiServiceAccount:   &controlplanev1.GCPServiceAccount{Email: "api-a@x.iam"},
		RpsqlServiceAccount:      &controlplanev1.GCPServiceAccount{Email: "sa-a@x.iam"},
		RpsqlCloudStorageBucket:  &controlplanev1.CustomerManagedGoogleCloudStorageBucket{Name: "bucket-a"},
	}
	if mutate != nil {
		mutate(gcp)
	}
	out := &controlplanev1.CustomerManagedResourcesUpdate{}
	out.SetGcp(gcp)
	return out
}

func awsCMR(mutate func(*controlplanev1.CustomerManagedResourcesUpdate_AWS)) *controlplanev1.CustomerManagedResourcesUpdate {
	aws := &controlplanev1.CustomerManagedResourcesUpdate_AWS{
		RedpandaConnectNodeGroupInstanceProfile: &controlplanev1.AWSInstanceProfile{Arn: "arn:rc-ng-a"},
		RedpandaConnectSecurityGroup:            &controlplanev1.AWSSecurityGroup{Arn: "arn:rc-sg-a"},
		RpsqlNodeGroupInstanceProfile:           &controlplanev1.AWSInstanceProfile{Arn: "arn:rpsql-ng-a"},
		RpsqlSecurityGroup:                      &controlplanev1.AWSSecurityGroup{Arn: "arn:rpsql-sg-a"},
		RpsqlCloudStorageBucket:                 &controlplanev1.CustomerManagedAWSCloudStorageBucket{Arn: "arn:rpsql-bkt-a"},
	}
	if mutate != nil {
		mutate(aws)
	}
	out := &controlplanev1.CustomerManagedResourcesUpdate{}
	out.SetAws(aws)
	return out
}

func TestExpandCustomerManagedResourceLeaves(t *testing.T) {
	tests := []struct {
		name        string
		in          []string
		plan, state *controlplanev1.CustomerManagedResourcesUpdate
		want        []string
	}{
		{
			name:  "no CMR path is a no-op",
			in:    []string{"name", "throughput_tier"},
			plan:  gcpCMR(nil),
			state: gcpCMR(nil),
			want:  []string{"name", "throughput_tier"},
		},
		{
			name:  "gcp psc_nat_subnet_name change",
			in:    []string{"customer_managed_resources"},
			plan:  gcpCMR(func(g *controlplanev1.CustomerManagedResourcesUpdate_GCP) { g.PscNatSubnetName = "nat-b" }),
			state: gcpCMR(nil),
			want:  []string{"customer_managed_resources.gcp.psc_nat_subnet_name"},
		},
		{
			name: "gcp rpsql trio + secret prefix change",
			in:   []string{"customer_managed_resources"},
			plan: gcpCMR(func(g *controlplanev1.CustomerManagedResourcesUpdate_GCP) {
				g.RpsqlApiServiceAccount = &controlplanev1.GCPServiceAccount{Email: "api-b@x.iam"}
				g.RpsqlServiceAccount = &controlplanev1.GCPServiceAccount{Email: "sa-b@x.iam"}
				g.RpsqlCloudStorageBucket = &controlplanev1.CustomerManagedGoogleCloudStorageBucket{Name: "bucket-b"}
				g.RpsqlSecretManagerPrefix = "tfrp-b"
			}),
			state: gcpCMR(nil),
			want: []string{
				"customer_managed_resources.gcp.rpsql_api_service_account.email",
				"customer_managed_resources.gcp.rpsql_service_account.email",
				"customer_managed_resources.gcp.rpsql_cloud_storage_bucket.name",
				"customer_managed_resources.gcp.rpsql_secret_manager_prefix",
			},
		},
		{
			name: "aws rpsql leaves key by public rpsql_* name",
			in:   []string{"customer_managed_resources"},
			plan: awsCMR(func(a *controlplanev1.CustomerManagedResourcesUpdate_AWS) {
				a.RpsqlNodeGroupInstanceProfile = &controlplanev1.AWSInstanceProfile{Arn: "arn:rpsql-ng-b"}
				a.RpsqlSecurityGroup = &controlplanev1.AWSSecurityGroup{Arn: "arn:rpsql-sg-b"}
				a.RpsqlCloudStorageBucket = &controlplanev1.CustomerManagedAWSCloudStorageBucket{Arn: "arn:rpsql-bkt-b"}
			}),
			state: awsCMR(nil),
			want: []string{
				"customer_managed_resources.aws.rpsql_node_group_instance_profile.arn",
				"customer_managed_resources.aws.rpsql_security_group.arn",
				"customer_managed_resources.aws.rpsql_cloud_storage_bucket.arn",
			},
		},
		{
			name: "aws redpanda_connect change",
			in:   []string{"customer_managed_resources"},
			plan: awsCMR(func(a *controlplanev1.CustomerManagedResourcesUpdate_AWS) {
				a.RedpandaConnectSecurityGroup = &controlplanev1.AWSSecurityGroup{Arn: "arn:rc-sg-b"}
			}),
			state: awsCMR(nil),
			want:  []string{"customer_managed_resources.aws.redpanda_connect_security_group.arn"},
		},
		{
			name:  "other mask paths preserved, CMR path replaced in place",
			in:    []string{"name", "customer_managed_resources", "throughput_tier"},
			plan:  gcpCMR(func(g *controlplanev1.CustomerManagedResourcesUpdate_GCP) { g.PscNatSubnetName = "nat-b" }),
			state: gcpCMR(nil),
			want:  []string{"name", "customer_managed_resources.gcp.psc_nat_subnet_name", "throughput_tier"},
		},
		{
			name:  "CMR path but no updatable leaf differs drops the bare path",
			in:    []string{"customer_managed_resources"},
			plan:  gcpCMR(nil),
			state: gcpCMR(nil),
			want:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := &fieldmaskpb.FieldMask{Paths: append([]string(nil), tt.in...)}
			ExpandCustomerManagedResourceLeaves(fm, tt.plan, tt.state)
			if !reflect.DeepEqual(fm.Paths, tt.want) {
				t.Errorf("Paths =\n  %v\nwant\n  %v", fm.Paths, tt.want)
			}
		})
	}
}
