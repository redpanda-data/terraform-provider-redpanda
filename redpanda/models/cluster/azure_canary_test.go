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

package cluster

import (
	"context"
	"testing"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils/enums"
)

// TestUnit_Cluster_Azure_SchemaCompiles exercises the Azure-specific
// Flatten/Expand code paths (azure_private_link, cloud_storage.azure,
// cloud_provider="azure")
func TestUnit_Cluster_Azure_SchemaCompiles(t *testing.T) {
	ctx := context.Background()

	proto := &controlplanev1.Cluster{
		CloudProvider: controlplanev1.CloudProvider_CLOUD_PROVIDER_AZURE,
		Region:        "westus2",
		AzurePrivateLink: &controlplanev1.Cluster_AzurePrivateLink{
			Enabled:              true,
			ConnectConsole:       true,
			AllowedSubscriptions: []string{"00000000-0000-0000-0000-000000000001"},
		},
		CloudStorage: &controlplanev1.Cluster_CloudStorage{
			CloudProvider: &controlplanev1.Cluster_CloudStorage_Azure_{
				Azure: &controlplanev1.Cluster_CloudStorage_Azure{
					ContainerName:      "rp-container",
					StorageAccountName: "rpstorageacct",
					AllowedIps:         []string{"203.0.113.0/24"},
				},
			},
		},
	}

	m, diags := Flatten(ctx, proto, nil)
	if diags.HasError() {
		t.Fatalf("Flatten: unexpected diagnostics: %v", diags.Errors())
	}

	if got := m.CloudProvider.ValueString(); got != enums.CloudProviderStringAzure {
		t.Errorf("CloudProvider: got %q, want %q", got, enums.CloudProviderStringAzure)
	}
	if m.AzurePrivateLink.IsNull() {
		t.Error("AzurePrivateLink: got Null, want populated object")
	}
	if m.CloudStorage.IsNull() {
		t.Error("CloudStorage: got Null, want populated object (Azure oneof should land in the model)")
	}

	req, diags := ExpandCreate(ctx, m)
	if diags.HasError() {
		t.Fatalf("ExpandCreate: unexpected diagnostics: %v", diags.Errors())
	}
	if req.GetCluster().GetCloudProvider() != controlplanev1.CloudProvider_CLOUD_PROVIDER_AZURE {
		t.Errorf("ExpandCreate CloudProvider: got %v, want CLOUD_PROVIDER_AZURE",
			req.GetCluster().GetCloudProvider())
	}
	if req.GetCluster().GetAzurePrivateLink() == nil {
		t.Error("ExpandCreate AzurePrivateLink: got nil, want non-nil")
	}
	if req.GetCluster().GetCloudStorage().GetAzure() == nil {
		t.Error("ExpandCreate CloudStorage.Azure oneof: got nil, want non-nil")
	}
}
