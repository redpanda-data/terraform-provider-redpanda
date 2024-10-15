// Copyright 2024 Redpanda Data, Inc.
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

package network

import (
	controlplanev1beta2 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

func generateModel(nw *controlplanev1beta2.Network) *models.Network {
	return &models.Network{
		CidrBlock:       types.StringValue(nw.CidrBlock),
		CloudProvider:   types.StringValue(utils.CloudProviderToString(nw.CloudProvider)),
		ClusterType:     types.StringValue(utils.ClusterTypeToString(nw.ClusterType)),
		ID:              types.StringValue(nw.Id),
		Name:            types.StringValue(nw.Name),
		Region:          types.StringValue(nw.Region),
		ResourceGroupID: types.StringValue(nw.ResourceGroupId),
	}
}
