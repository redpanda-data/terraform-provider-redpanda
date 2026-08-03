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
	"testing"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
)

const (
	testRoleARN = "arn:aws:iam::123456789012:role/redpanda-cross-account"
	testExtID   = "a845616f-0484-4506-9638-45fe28f34865"
)

func TestFlatten(t *testing.T) {
	proto := &controlplanev1.CloudProviderAccess{
		Id:            "cpa1234567890abcdefg",
		Name:          "prod-aws-account",
		CloudProvider: controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
		Config: &controlplanev1.CloudProviderAccess_Aws{
			Aws: &controlplanev1.AWSCloudProviderAccess{
				RoleArn:    testRoleARN,
				ExternalId: testExtID,
			},
		},
		State: controlplanev1.CloudProviderAccess_STATE_ACTIVE,
	}

	m, diags := Flatten(context.Background(), proto, nil)
	require.False(t, diags.HasError())
	require.Equal(t, "cpa1234567890abcdefg", m.ID.ValueString())
	require.Equal(t, "prod-aws-account", m.Name.ValueString())
	require.Equal(t, "aws", m.CloudProvider.ValueString())
	require.Equal(t, "ACTIVE", m.State.ValueString())
	require.NotNil(t, m.AWS)
	require.Equal(t, testRoleARN, m.AWS.RoleARN.ValueString())
	require.Equal(t, testExtID, m.AWS.ExternalID.ValueString())
}

func TestFlattenCarriesPrevAWSWhenResponseOmitsIt(t *testing.T) {
	proto := &controlplanev1.CloudProviderAccess{
		Id:            "cpa1234567890abcdefg",
		Name:          "prod-aws-account",
		CloudProvider: controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
		State:         controlplanev1.CloudProviderAccess_STATE_ACTIVE,
	}
	prev := &ResourceModel{
		AWS: &AWSModel{
			RoleARN:    types.StringValue(testRoleARN),
			ExternalID: types.StringValue(testExtID),
		},
	}

	m, diags := Flatten(context.Background(), proto, prev)
	require.False(t, diags.HasError())
	require.NotNil(t, m.AWS, "planned aws block must be carried forward when the response omits it")
	require.Equal(t, testRoleARN, m.AWS.RoleARN.ValueString())
}

func TestFlattenNoPrevNoAWS(t *testing.T) {
	proto := &controlplanev1.CloudProviderAccess{
		Id:    "cpa1234567890abcdefg",
		Name:  "prod-aws-account",
		State: controlplanev1.CloudProviderAccess_STATE_PENDING,
	}

	m, diags := Flatten(context.Background(), proto, nil)
	require.False(t, diags.HasError())
	require.Nil(t, m.AWS)
	require.Equal(t, "PENDING", m.State.ValueString())
}

func TestExpandCreate(t *testing.T) {
	m := &ResourceModel{
		Name:          types.StringValue("prod-aws-account"),
		CloudProvider: types.StringValue("aws"),
		AWS: &AWSModel{
			RoleARN: types.StringValue(testRoleARN),
		},
	}

	req, diags := ExpandCreate(context.Background(), m)
	require.False(t, diags.HasError())
	payload := req.GetCloudProviderAccess()
	require.Equal(t, "prod-aws-account", payload.GetName())
	require.Equal(t, controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS, payload.GetCloudProvider())
	require.Equal(t, testRoleARN, payload.GetAws().GetRoleArn())
	require.Empty(t, payload.GetAws().GetExternalId(), "external_id is server-derived and must not be sent on create")
}

func TestExpandCreateWithoutAWSBlock(t *testing.T) {
	m := &ResourceModel{
		Name:          types.StringValue("prod-aws-account"),
		CloudProvider: types.StringValue("aws"),
	}

	req, diags := ExpandCreate(context.Background(), m)
	require.False(t, diags.HasError())
	require.Nil(t, req.GetCloudProviderAccess().GetAws())
}

func TestExpandDelete(t *testing.T) {
	m := &ResourceModel{ID: types.StringValue("cpa1234567890abcdefg")}

	req, diags := ExpandDelete(context.Background(), m)
	require.False(t, diags.HasError())
	require.Equal(t, "cpa1234567890abcdefg", req.GetId())
}

func TestFlattenExpandRoundTrip(t *testing.T) {
	proto := &controlplanev1.CloudProviderAccess{
		Id:            "cpa1234567890abcdefg",
		Name:          "prod-aws-account",
		CloudProvider: controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
		Config: &controlplanev1.CloudProviderAccess_Aws{
			Aws: &controlplanev1.AWSCloudProviderAccess{
				RoleArn:    testRoleARN,
				ExternalId: testExtID,
			},
		},
		State: controlplanev1.CloudProviderAccess_STATE_ACTIVE,
	}

	m, diags := Flatten(context.Background(), proto, nil)
	require.False(t, diags.HasError())

	req, diags := ExpandCreate(context.Background(), m)
	require.False(t, diags.HasError())
	payload := req.GetCloudProviderAccess()
	require.Equal(t, proto.GetName(), payload.GetName())
	require.Equal(t, proto.GetCloudProvider(), payload.GetCloudProvider())
	require.Equal(t, proto.GetAws().GetRoleArn(), payload.GetAws().GetRoleArn())
}
