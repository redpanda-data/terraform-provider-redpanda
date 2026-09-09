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

package fakes

import (
	"context"
	"slices"
	"testing"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// TestNetworkFake_UpdateNetworkPublicSubnets pins the fake's UpdateNetwork to
// the control-plane contract for the one field settable after create (cloudv2
// apps/controlplane-api/internal/services/network/network_service.go
// validatePublicSubnetsWriteOnce and validateCustomerManagedResourcesUpdateTargetsAWS;
// apps/public-api-go/internal/services/network/v1/network_service.go
// isUpdatableCustomerManagedResourcesMaskPath). A fake that accepts a cleared
// or changed set, or a mask naming any other customer-managed leaf, would let
// the provider pass tests the real API rejects.
func TestNetworkFake_UpdateNetworkPublicSubnets(t *testing.T) {
	const (
		id   = "net00000000000000001"
		arnA = "arn:aws:ec2:us-east-1:123456789012:subnet/subnet-0aaa1111bbbb2222c"
		arnB = "arn:aws:ec2:us-east-1:123456789012:subnet/subnet-0ddd3333eeee4444f"
	)

	awsNetwork := func(public ...string) *controlplanev1.Network {
		aws := &controlplanev1.Network_CustomerManagedResources_AWS{
			ManagementBucket: &controlplanev1.CustomerManagedAWSCloudStorageBucket{Arn: "arn:aws:s3:::tfrp-bv-mgmt"},
			DynamodbTable:    &controlplanev1.CustomerManagedDynamoDBTable{Arn: "arn:aws:dynamodb:us-east-1:123456789012:table/tfrp-bv-ddb"},
			Vpc:              &controlplanev1.CustomerManagedAWSVPC{Arn: "arn:aws:ec2:us-east-1:123456789012:vpc/vpc-0abc1234def56789a"},
			PrivateSubnets:   &controlplanev1.CustomerManagedAWSSubnets{Arns: []string{"arn:aws:ec2:us-east-1:123456789012:subnet/subnet-0abc1234def56789a"}},
		}
		if len(public) > 0 {
			aws.PublicSubnets = &controlplanev1.CustomerManagedAWSSubnets{Arns: public}
		}
		return &controlplanev1.Network{
			Id: id, Name: "n", CloudProvider: controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
			ClusterType: controlplanev1.Cluster_TYPE_BYOC, State: controlplanev1.Network_STATE_READY,
			CustomerManagedResources: &controlplanev1.Network_CustomerManagedResources{
				CloudProvider: &controlplanev1.Network_CustomerManagedResources_Aws{Aws: aws},
			},
		}
	}
	gcpNetwork := func() *controlplanev1.Network {
		return &controlplanev1.Network{
			Id: id, Name: "n", CloudProvider: controlplanev1.CloudProvider_CLOUD_PROVIDER_GCP,
			ClusterType: controlplanev1.Cluster_TYPE_BYOC, State: controlplanev1.Network_STATE_READY,
			CustomerManagedResources: &controlplanev1.Network_CustomerManagedResources{
				CloudProvider: &controlplanev1.Network_CustomerManagedResources_Gcp{
					Gcp: &controlplanev1.Network_CustomerManagedResources_GCP{NetworkName: "vpc", NetworkProjectId: "proj"},
				},
			},
		}
	}
	update := func(public ...string) *controlplanev1.NetworkUpdate {
		aws := &controlplanev1.Network_UpdatableCustomerManagedResources_AWS{}
		if len(public) > 0 {
			aws.PublicSubnets = &controlplanev1.CustomerManagedAWSSubnets{Arns: public}
		}
		return &controlplanev1.NetworkUpdate{
			Id: id,
			CustomerManagedResources: &controlplanev1.Network_UpdatableCustomerManagedResources{
				CloudProvider: &controlplanev1.Network_UpdatableCustomerManagedResources_Aws{Aws: aws},
			},
		}
	}
	storedPublic := func(nw *controlplanev1.Network) []string {
		return nw.GetCustomerManagedResources().GetAws().GetPublicSubnets().GetArns()
	}

	cases := []struct {
		name    string
		seed    *controlplanev1.Network
		update  *controlplanev1.NetworkUpdate
		mask    []string
		wantErr codes.Code
		want    []string
	}{
		{
			name:   "unset to set via top-level mask",
			seed:   awsNetwork(),
			update: update(arnA, arnB),
			mask:   []string{"customer_managed_resources"},
			want:   []string{arnA, arnB},
		},
		{
			name:   "unset to set via aws mask",
			seed:   awsNetwork(),
			update: update(arnA),
			mask:   []string{"customer_managed_resources.aws"},
			want:   []string{arnA},
		},
		{
			name:   "unset to set via leaf mask",
			seed:   awsNetwork(),
			update: update(arnA),
			mask:   []string{"customer_managed_resources.aws.public_subnets"},
			want:   []string{arnA},
		},
		{
			// Set comparison: the same subnets in another order is not a change.
			name:   "same set reordered accepted",
			seed:   awsNetwork(arnA, arnB),
			update: update(arnB, arnA),
			mask:   []string{"customer_managed_resources"},
			want:   []string{arnB, arnA},
		},
		{
			name:    "change once set rejected",
			seed:    awsNetwork(arnA),
			update:  update(arnB),
			mask:    []string{"customer_managed_resources"},
			wantErr: codes.FailedPrecondition,
			want:    []string{arnA},
		},
		{
			name:    "clear once set rejected",
			seed:    awsNetwork(arnA),
			update:  update(),
			mask:    []string{"customer_managed_resources"},
			wantErr: codes.FailedPrecondition,
			want:    []string{arnA},
		},
		{
			// Only the three public_subnets spellings map to a write; any other
			// customer-managed sub-path is refused rather than silently ignored.
			name:    "other customer-managed leaf in mask rejected",
			seed:    awsNetwork(),
			update:  update(arnA),
			mask:    []string{"customer_managed_resources.aws.vpc"},
			wantErr: codes.InvalidArgument,
		},
		{
			name:    "customer-managed update on a GCP network rejected",
			seed:    gcpNetwork(),
			update:  update(arnA),
			mask:    []string{"customer_managed_resources"},
			wantErr: codes.InvalidArgument,
		},
		{
			// A mask that does not name customer_managed_resources leaves the
			// subnets alone even if the payload carries them.
			name:   "unmasked payload ignored",
			seed:   awsNetwork(),
			update: update(arnA),
			mask:   []string{"egress_spec"},
			want:   nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := NewNetworkFake(NewOperationFake())
			f.mu.Lock()
			f.networks[id] = tc.seed
			f.mu.Unlock()
			_, err := f.UpdateNetwork(context.Background(), &controlplanev1.UpdateNetworkRequest{
				Network:    tc.update,
				UpdateMask: &fieldmaskpb.FieldMask{Paths: tc.mask},
			})
			if tc.wantErr != codes.OK {
				if status.Code(err) != tc.wantErr {
					t.Fatalf("UpdateNetwork: got error %v, want code %v", err, tc.wantErr)
				}
			} else if err != nil {
				t.Fatalf("UpdateNetwork: %v", err)
			}
			resp, err := f.GetNetwork(context.Background(), &controlplanev1.GetNetworkRequest{Id: id})
			if err != nil {
				t.Fatalf("GetNetwork: %v", err)
			}
			if got := storedPublic(resp.GetNetwork()); !slices.Equal(got, tc.want) {
				t.Fatalf("stored public subnets = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNetworkFake_UpdateNetworkNotFound(t *testing.T) {
	f := NewNetworkFake(NewOperationFake())
	_, err := f.UpdateNetwork(context.Background(), &controlplanev1.UpdateNetworkRequest{
		Network:    &controlplanev1.NetworkUpdate{Id: "net00000000000000009"},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"customer_managed_resources"}},
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("UpdateNetwork on unknown id: got %v, want NotFound", err)
	}
}

func azureNetworkCMR() *controlplanev1.Network_CustomerManagedResources {
	sub := func(n string) *controlplanev1.Network_CustomerManagedResources_Azure_Subnets_Subnet {
		return &controlplanev1.Network_CustomerManagedResources_Azure_Subnets_Subnet{Name: n}
	}
	return &controlplanev1.Network_CustomerManagedResources{
		CloudProvider: &controlplanev1.Network_CustomerManagedResources_Azure_{
			Azure: &controlplanev1.Network_CustomerManagedResources_Azure{
				ManagementBucket: &controlplanev1.CustomerManagedAzureBucketSpec{StorageAccountName: "mgmtsa", StorageContainerName: "mgmt"},
				Vnet: &controlplanev1.Network_CustomerManagedResources_Azure_Vnet{
					Name:          "vnet",
					ResourceGroup: &controlplanev1.CustomerManagedAzureResourceGroupSpec{Name: "net-rg"},
				},
				Subnets: &controlplanev1.Network_CustomerManagedResources_Azure_Subnets{
					RpAgent: sub("agent"), Rp_0Pods: sub("rp0p"), Rp_0Vnet: sub("rp0v"), Rp_1Pods: sub("rp1p"), Rp_1Vnet: sub("rp1v"),
					Rp_2Pods: sub("rp2p"), Rp_2Vnet: sub("rp2v"), RpConnectPods: sub("rcp"), RpConnectVnet: sub("rcv"),
					SysPods: sub("sysp"), SysVnet: sub("sysv"), RpEgressVnet: sub("egress"), KafkaConnectPods: sub("kcp"), KafkaConnectVnet: sub("kcv"),
				},
			},
		},
	}
}

// TestNetworkFake_AzureCMRRoundTrip pins that every Azure customer-managed
// leaf written on create reads back on get, rp_egress_vnet included. This is
// the contract the provider is built against; a control plane that drops the
// egress subnet on read is a control-plane bug, not something the fake models.
func TestNetworkFake_AzureCMRRoundTrip(t *testing.T) {
	f := NewNetworkFake(NewOperationFake())
	op, err := f.CreateNetwork(context.Background(), &controlplanev1.CreateNetworkRequest{Network: &controlplanev1.NetworkCreate{
		Name:                     "net",
		CloudProvider:            controlplanev1.CloudProvider_CLOUD_PROVIDER_AZURE,
		ClusterType:              controlplanev1.Cluster_TYPE_BYOC,
		Region:                   "eastus",
		CustomerManagedResources: azureNetworkCMR(),
	}})
	if err != nil {
		t.Fatalf("CreateNetwork: %v", err)
	}
	got, err := f.GetNetwork(context.Background(), &controlplanev1.GetNetworkRequest{Id: op.GetOperation().GetResourceId()})
	if err != nil {
		t.Fatalf("GetNetwork: %v", err)
	}
	az := got.GetNetwork().GetCustomerManagedResources().GetAzure()
	if az == nil {
		t.Fatal("Azure arm did not read back")
	}
	if n := az.GetSubnets().GetRpEgressVnet().GetName(); n != "egress" {
		t.Errorf("rp_egress_vnet: got %q, want %q", n, "egress")
	}
	if n := az.GetSubnets().GetRp_2Vnet().GetName(); n != "rp2v" {
		t.Errorf("rp_2_vnet: got %q, want %q", n, "rp2v")
	}
	if n := az.GetVnet().GetResourceGroup().GetName(); n != "net-rg" {
		t.Errorf("vnet.resource_group: got %q, want %q", n, "net-rg")
	}
	// The public read mapper (cloudv2 apps/public-api-go network mapper) always
	// builds management_bucket.resource_group, with an empty name when the
	// request carried none; a fake that echoes nil hides that on refresh.
	if rg := az.GetManagementBucket().GetResourceGroup(); rg == nil {
		t.Error("management_bucket.resource_group: got nil, want an empty spec like the control plane returns")
	} else if rg.GetName() != "" {
		t.Errorf("management_bucket.resource_group.name: got %q, want empty for an omitted block", rg.GetName())
	}
	if got.GetNetwork().GetCidrBlock() != "0.0.0.0/0" {
		t.Errorf("cidr_block: got %q, want the control plane's 0.0.0.0/0 placeholder", got.GetNetwork().GetCidrBlock())
	}
}

// TestNetworkFake_CMRRequiresBYOC mirrors cloudv2's
// validateCustomerManagedResources (controlplane-api network_service.go): a
// network carrying customer_managed_resources must be TYPE_BYOC.
func TestNetworkFake_CMRRequiresBYOC(t *testing.T) {
	f := NewNetworkFake(NewOperationFake())
	_, err := f.CreateNetwork(context.Background(), &controlplanev1.CreateNetworkRequest{Network: &controlplanev1.NetworkCreate{
		Name:                     "net",
		CloudProvider:            controlplanev1.CloudProvider_CLOUD_PROVIDER_AZURE,
		ClusterType:              controlplanev1.Cluster_TYPE_DEDICATED,
		Region:                   "eastus",
		CustomerManagedResources: azureNetworkCMR(),
	}})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("CreateNetwork with CMR on a dedicated network: got %v, want InvalidArgument", err)
	}
}
