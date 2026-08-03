// Copyright 2026 Redpanda Data, Inc.
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

package cloudprovideraccess

import (
	"context"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils/enums"
)

// CloudProviderAccessResponse is the interface satisfied by the proto
// CloudProviderAccess message returned from Get/Create RPCs.
type CloudProviderAccessResponse interface {
	GetId() string
	GetName() string
	GetCloudProvider() controlplanev1.CloudProvider
	GetAws() *controlplanev1.AWSCloudProviderAccess
	GetState() controlplanev1.CloudProviderAccess_State
}

// Flatten populates a *ResourceModel from the API response payload.
// The prev *ResourceModel arg carries forward TF-only fields the response
// cannot supply (timeouts) and the aws block when the response omits it;
// pass nil when no prior state exists (e.g., during ImportState).
func Flatten(_ context.Context, proto CloudProviderAccessResponse, prev *ResourceModel) (*ResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	m := &ResourceModel{}
	if prev != nil {
		m.Timeouts = prev.Timeouts
	}
	m.ID = types.StringValue(proto.GetId())
	m.Name = types.StringValue(proto.GetName())
	m.CloudProvider = types.StringValue(enums.CloudProviderToString(proto.GetCloudProvider()))
	m.State = types.StringValue(enums.CloudProviderAccessStateToString(proto.GetState()))
	if aws := proto.GetAws(); aws != nil {
		m.AWS = &AWSModel{
			RoleARN:    types.StringValue(aws.GetRoleArn()),
			ExternalID: types.StringValue(aws.GetExternalId()),
		}
	} else if prev != nil {
		// Carry the planned block forward if the response omits it, so a
		// known planned value never turns null in state ("Provider produced
		// inconsistent result after apply").
		m.AWS = prev.AWS
	}
	return m, diags
}

// ExpandCreate renders a *ResourceModel into the proto request envelope for
// the CreateCloudProviderAccess RPC.
func ExpandCreate(_ context.Context, m *ResourceModel) (*controlplanev1.CreateCloudProviderAccessRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	payload := &controlplanev1.CloudProviderAccessCreate{
		Name:          m.Name.ValueString(),
		CloudProvider: enums.StringToCloudProvider(m.CloudProvider.ValueString()),
	}
	if m.AWS != nil {
		payload.Config = &controlplanev1.CloudProviderAccessCreate_Aws{
			Aws: &controlplanev1.AWSCloudProviderAccess{
				RoleArn: m.AWS.RoleARN.ValueString(),
			},
		}
	}
	return &controlplanev1.CreateCloudProviderAccessRequest{
		CloudProviderAccess: payload,
	}, diags
}

// ExpandDelete renders a *ResourceModel into the proto request envelope for
// the DeleteCloudProviderAccess RPC.
func ExpandDelete(_ context.Context, m *ResourceModel) (*controlplanev1.DeleteCloudProviderAccessRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	return &controlplanev1.DeleteCloudProviderAccessRequest{
		Id: m.ID.ValueString(),
	}, diags
}
