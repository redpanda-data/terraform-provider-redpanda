package serverlesscluster

import (
	"fmt"
	"reflect"
	"testing"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	serverlessclustermodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/serverlesscluster"
)

func TestGenerateServerlessClusterRequest(t *testing.T) {
	type args struct {
		model serverlessclustermodel.ResourceModel
	}
	tests := []struct {
		name string
		args args
		want *controlplanev1.ServerlessClusterCreate
	}{
		{
			name: "validate_schema",
			args: args{
				model: serverlessclustermodel.ResourceModel{
					Name:             types.StringValue("testname"),
					ServerlessRegion: types.StringValue("pro-us-east-1"),
					ResourceGroupID:  types.StringValue("testrg"),
					PrivateLinkID:    types.StringNull(),
					NetworkingConfig: types.ObjectNull(nil),
				},
			},
			want: &controlplanev1.ServerlessClusterCreate{
				Name:             "testname",
				ServerlessRegion: "pro-us-east-1",
				ResourceGroupId:  "testrg",
			},
		},
		{
			name: "with_private_link_id",
			args: args{
				model: serverlessclustermodel.ResourceModel{
					Name:             types.StringValue("testname"),
					ServerlessRegion: types.StringValue("pro-us-east-1"),
					ResourceGroupID:  types.StringValue("testrg"),
					PrivateLinkID:    types.StringValue("abcdefghij1234567890"),
					NetworkingConfig: types.ObjectNull(nil),
				},
			},
			want: &controlplanev1.ServerlessClusterCreate{
				Name:             "testname",
				ServerlessRegion: "pro-us-east-1",
				ResourceGroupId:  "testrg",
				PrivateLinkId:    stringPtr("abcdefghij1234567890"),
			},
		},
		{
			name: "with_networking_config",
			args: args{
				model: serverlessclustermodel.ResourceModel{
					Name:             types.StringValue("testname"),
					ServerlessRegion: types.StringValue("pro-us-east-1"),
					ResourceGroupID:  types.StringValue("testrg"),
					PrivateLinkID:    types.StringNull(),
					NetworkingConfig: mustObjectValue(
						map[string]attr.Type{
							"private": types.StringType,
							"public":  types.StringType,
						},
						map[string]attr.Value{
							"private": types.StringValue("STATE_ENABLED"),
							"public":  types.StringValue("STATE_DISABLED"),
						},
					),
				},
			},
			want: &controlplanev1.ServerlessClusterCreate{
				Name:             "testname",
				ServerlessRegion: "pro-us-east-1",
				ResourceGroupId:  "testrg",
				NetworkingConfig: &controlplanev1.ServerlessNetworkingConfig{
					Private: controlplanev1.ServerlessNetworkingConfig_STATE_ENABLED,
					Public:  controlplanev1.ServerlessNetworkingConfig_STATE_DISABLED,
				},
			},
		},
		{
			name: "with_all_fields",
			args: args{
				model: serverlessclustermodel.ResourceModel{
					Name:             types.StringValue("testname"),
					ServerlessRegion: types.StringValue("pro-us-east-1"),
					ResourceGroupID:  types.StringValue("testrg"),
					PrivateLinkID:    types.StringValue("abcdefghij1234567890"),
					NetworkingConfig: mustObjectValue(
						map[string]attr.Type{
							"private": types.StringType,
							"public":  types.StringType,
						},
						map[string]attr.Value{
							"private": types.StringValue("STATE_ENABLED"),
							"public":  types.StringValue("STATE_UNSPECIFIED"),
						},
					),
				},
			},
			want: &controlplanev1.ServerlessClusterCreate{
				Name:             "testname",
				ServerlessRegion: "pro-us-east-1",
				ResourceGroupId:  "testrg",
				PrivateLinkId:    stringPtr("abcdefghij1234567890"),
				NetworkingConfig: &controlplanev1.ServerlessNetworkingConfig{
					Private: controlplanev1.ServerlessNetworkingConfig_STATE_ENABLED,
					Public:  controlplanev1.ServerlessNetworkingConfig_STATE_UNSPECIFIED,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := GenerateServerlessClusterRequest(tt.args.model); !reflect.DeepEqual(got, tt.want) {
				fmt.Println("got")
				spew.Dump(got)
				fmt.Println("want")
				spew.Dump(tt.want)
				t.Errorf("GenerateServerlessClusterRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Helper functions for tests
func stringPtr(s string) *string {
	return &s
}

func mustObjectValue(attrTypes map[string]attr.Type, attrs map[string]attr.Value) types.Object {
	obj, diags := types.ObjectValue(attrTypes, attrs)
	if diags.HasError() {
		panic(fmt.Sprintf("failed to create object value: %v", diags))
	}
	return obj
}

func TestGenerateServerlessClusterUpdateRequest(t *testing.T) {
	type args struct {
		model serverlessclustermodel.ResourceModel
	}
	tests := []struct {
		name string
		args args
		want *controlplanev1.UpdateServerlessClusterRequest
	}{
		{
			name: "update_private_link_id",
			args: args{
				model: serverlessclustermodel.ResourceModel{
					ID:               types.StringValue("cluster123"),
					Name:             types.StringValue("testname"),
					ServerlessRegion: types.StringValue("pro-us-east-1"),
					ResourceGroupID:  types.StringValue("testrg"),
					PrivateLinkID:    types.StringValue("abcdefghij1234567890"),
					NetworkingConfig: types.ObjectNull(nil),
				},
			},
			want: &controlplanev1.UpdateServerlessClusterRequest{
				Id:            "cluster123",
				PrivateLinkId: stringPtr("abcdefghij1234567890"),
			},
		},
		{
			name: "update_networking_config",
			args: args{
				model: serverlessclustermodel.ResourceModel{
					ID:               types.StringValue("cluster123"),
					Name:             types.StringValue("testname"),
					ServerlessRegion: types.StringValue("pro-us-east-1"),
					ResourceGroupID:  types.StringValue("testrg"),
					PrivateLinkID:    types.StringNull(),
					NetworkingConfig: mustObjectValue(
						map[string]attr.Type{
							"private": types.StringType,
							"public":  types.StringType,
						},
						map[string]attr.Value{
							"private": types.StringValue("STATE_ENABLED"),
							"public":  types.StringValue("STATE_DISABLED"),
						},
					),
				},
			},
			want: &controlplanev1.UpdateServerlessClusterRequest{
				Id: "cluster123",
				NetworkingConfig: &controlplanev1.ServerlessNetworkingConfig{
					Private: controlplanev1.ServerlessNetworkingConfig_STATE_ENABLED,
					Public:  controlplanev1.ServerlessNetworkingConfig_STATE_DISABLED,
				},
			},
		},
		{
			name: "update_all_fields",
			args: args{
				model: serverlessclustermodel.ResourceModel{
					ID:               types.StringValue("cluster123"),
					Name:             types.StringValue("testname"),
					ServerlessRegion: types.StringValue("pro-us-east-1"),
					ResourceGroupID:  types.StringValue("testrg"),
					PrivateLinkID:    types.StringValue("abcdefghij1234567890"),
					NetworkingConfig: mustObjectValue(
						map[string]attr.Type{
							"private": types.StringType,
							"public":  types.StringType,
						},
						map[string]attr.Value{
							"private": types.StringValue("STATE_ENABLED"),
							"public":  types.StringValue("STATE_UNSPECIFIED"),
						},
					),
				},
			},
			want: &controlplanev1.UpdateServerlessClusterRequest{
				Id:            "cluster123",
				PrivateLinkId: stringPtr("abcdefghij1234567890"),
				NetworkingConfig: &controlplanev1.ServerlessNetworkingConfig{
					Private: controlplanev1.ServerlessNetworkingConfig_STATE_ENABLED,
					Public:  controlplanev1.ServerlessNetworkingConfig_STATE_UNSPECIFIED,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := GenerateServerlessClusterUpdateRequest(tt.args.model); !reflect.DeepEqual(got, tt.want) {
				fmt.Println("got")
				spew.Dump(got)
				fmt.Println("want")
				spew.Dump(tt.want)
				t.Errorf("GenerateServerlessClusterUpdateRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}
