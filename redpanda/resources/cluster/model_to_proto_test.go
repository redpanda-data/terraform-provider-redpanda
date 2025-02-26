package cluster

import (
	"context"
	"testing"

	controlplanev1beta2 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
)

func TestGetGcpPrivateServiceConnect(t *testing.T) {
	tests := []struct {
		name        string
		input       types.Object
		want        *controlplanev1beta2.GCPPrivateServiceConnectSpec
		expectError bool
		errorMsg    string
	}{
		{
			name:  "null object returns nil",
			input: types.ObjectNull(gcpPrivateServiceConnectType),
			want:  nil,
		},
		{
			name: "basic valid configuration",
			input: types.ObjectValueMust(
				gcpPrivateServiceConnectType,
				map[string]attr.Value{
					"enabled":               types.BoolValue(true),
					"global_access_enabled": types.BoolValue(false),
					"consumer_accept_list": types.ListValueMust(
						types.ObjectType{AttrTypes: map[string]attr.Type{"source": types.StringType}},
						[]attr.Value{
							types.ObjectValueMust(
								map[string]attr.Type{"source": types.StringType},
								map[string]attr.Value{"source": types.StringValue("project-123")},
							),
						},
					),
					"status": types.ObjectNull(gcpPrivateServiceConnectStatusType),
				},
			),
			want: &controlplanev1beta2.GCPPrivateServiceConnectSpec{
				Enabled:             true,
				GlobalAccessEnabled: false,
				ConsumerAcceptList: []*controlplanev1beta2.GCPPrivateServiceConnectConsumer{
					{Source: "project-123"},
				},
			},
		},
		{
			name: "multiple consumers",
			input: types.ObjectValueMust(
				gcpPrivateServiceConnectType,
				map[string]attr.Value{
					"enabled":               types.BoolValue(true),
					"global_access_enabled": types.BoolValue(true),
					"consumer_accept_list": types.ListValueMust(
						types.ObjectType{AttrTypes: map[string]attr.Type{"source": types.StringType}},
						[]attr.Value{
							types.ObjectValueMust(
								map[string]attr.Type{"source": types.StringType},
								map[string]attr.Value{"source": types.StringValue("project-123")},
							),
							types.ObjectValueMust(
								map[string]attr.Type{"source": types.StringType},
								map[string]attr.Value{"source": types.StringValue("project-456")},
							),
						},
					),
					"status": types.ObjectNull(gcpPrivateServiceConnectStatusType),
				},
			),
			want: &controlplanev1beta2.GCPPrivateServiceConnectSpec{
				Enabled:             true,
				GlobalAccessEnabled: true,
				ConsumerAcceptList: []*controlplanev1beta2.GCPPrivateServiceConnectConsumer{
					{Source: "project-123"},
					{Source: "project-456"},
				},
			},
		},
		{
			name: "empty consumer list",
			input: types.ObjectValueMust(
				gcpPrivateServiceConnectType,
				map[string]attr.Value{
					"enabled":               types.BoolValue(true),
					"global_access_enabled": types.BoolValue(false),
					"consumer_accept_list":  types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{"source": types.StringType}}),
					"status":                types.ObjectNull(gcpPrivateServiceConnectStatusType),
				},
			),
			want: &controlplanev1beta2.GCPPrivateServiceConnectSpec{
				Enabled:             true,
				GlobalAccessEnabled: false,
				ConsumerAcceptList:  nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, diags := getGcpPrivateServiceConnect(context.Background(), tt.input, diag.Diagnostics{})

			if tt.expectError {
				assert.True(t, diags.HasError())
				assert.Contains(t, diags.Errors()[0].Summary(), tt.errorMsg)
				return
			}

			assert.False(t, diags.HasError())
			assert.Equal(t, tt.want, got)
		})
	}
}
