package serverlessprivatelink

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/stretchr/testify/assert"
)

func TestGenerateServerlessPrivateLinkRequest(t *testing.T) {
	ctx := context.Background()

	// Create AWS config object
	allowedPrincipals, _ := types.ListValue(types.StringType, []attr.Value{
		types.StringValue("arn:aws:iam::123456789012:root"),
		types.StringValue("arn:aws:iam::123456789012:user/testuser"),
	})

	awsConfig, _ := types.ObjectValue(
		getAWSConfigType(),
		map[string]attr.Value{
			"allowed_principals": allowedPrincipals,
		},
	)

	cloudProviderConfig, _ := types.ObjectValue(
		getCloudProviderConfigType(),
		map[string]attr.Value{
			"aws": awsConfig,
		},
	)

	type args struct {
		model models.ServerlessPrivateLink
	}
	tests := []struct {
		name    string
		args    args
		want    *controlplanev1.ServerlessPrivateLinkCreate
		wantErr bool
	}{
		{
			name: "valid_aws_config",
			args: args{
				model: models.ServerlessPrivateLink{
					Name:                types.StringValue("test-private-link"),
					ResourceGroupID:     types.StringValue("test-rg-id"),
					CloudProvider:       types.StringValue("aws"),
					ServerlessRegion:    types.StringValue("pro-us-east-1"),
					CloudProviderConfig: cloudProviderConfig,
				},
			},
			want: &controlplanev1.ServerlessPrivateLinkCreate{
				Name:             "test-private-link",
				ResourceGroupId:  "test-rg-id",
				Cloudprovider:    controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
				ServerlessRegion: "pro-us-east-1",
				CloudProviderConfig: &controlplanev1.ServerlessPrivateLinkCreate_AwsConfig{
					AwsConfig: &controlplanev1.ServerlessPrivateLinkCreate_AWS{
						AllowedPrincipals: []string{
							"arn:aws:iam::123456789012:root",
							"arn:aws:iam::123456789012:user/testuser",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing_aws_config",
			args: args{
				model: models.ServerlessPrivateLink{
					Name:                types.StringValue("test-private-link"),
					ResourceGroupID:     types.StringValue("test-rg-id"),
					CloudProvider:       types.StringValue("aws"),
					ServerlessRegion:    types.StringValue("pro-us-east-1"),
					CloudProviderConfig: types.ObjectNull(getCloudProviderConfigType()),
				},
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateServerlessPrivateLinkRequest(ctx, tt.args.model)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateServerlessPrivateLinkRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				fmt.Println("got")
				spew.Dump(got)
				fmt.Println("want")
				spew.Dump(tt.want)
				t.Errorf("GenerateServerlessPrivateLinkRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateModel(t *testing.T) {
	ctx := context.Background()

	spl := &controlplanev1.ServerlessPrivateLink{
		Id:               "test-id",
		Name:             "test-name",
		ResourceGroupId:  "test-rg-id",
		Cloudprovider:    controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS,
		ServerlessRegion: "pro-us-east-1",
		State:            controlplanev1.ServerlessPrivateLink_STATE_READY,
		CloudProviderConfig: &controlplanev1.ServerlessPrivateLink_AwsConfig{
			AwsConfig: &controlplanev1.ServerlessPrivateLink_AWS{
				AllowedPrincipals: []string{"arn:aws:iam::123456789012:root"},
			},
		},
		Status: &controlplanev1.ServerlessPrivateLinkStatus{
			CloudProvider: &controlplanev1.ServerlessPrivateLinkStatus_Aws{
				Aws: &controlplanev1.ServerlessPrivateLinkStatus_AWS{
					VpcEndpointServiceName: "com.amazonaws.vpce.us-east-1.vpce-svc-123456",
				},
			},
		},
	}

	model, err := generateModel(ctx, spl, types.BoolValue(false))
	assert.NoError(t, err)

	if model.ID.ValueString() != "test-id" {
		t.Errorf("Expected ID to be 'test-id', got %s", model.ID.ValueString())
	}

	if model.Name.ValueString() != "test-name" {
		t.Errorf("Expected Name to be 'test-name', got %s", model.Name.ValueString())
	}

	if model.CloudProvider.ValueString() != "aws" {
		t.Errorf("Expected CloudProvider to be 'aws', got %s", model.CloudProvider.ValueString())
	}

	if model.State.ValueString() != "STATE_READY" {
		t.Errorf("Expected State to be 'STATE_READY', got %s", model.State.ValueString())
	}

	if model.CloudProviderConfig.IsNull() {
		t.Error("Expected CloudProviderConfig to not be null")
	}

	if model.Status.IsNull() {
		t.Error("Expected Status to not be null")
	}

	// Verify the nested AWS config is present
	configAttrs := model.CloudProviderConfig.Attributes()
	awsAttr, ok := configAttrs["aws"]
	if !ok {
		t.Error("Expected aws attribute in CloudProviderConfig")
	}
	awsObj, ok := awsAttr.(types.Object)
	if !ok || awsObj.IsNull() {
		t.Error("Expected aws config to be non-null object")
	}

	// Verify the nested AWS status is present
	statusAttrs := model.Status.Attributes()
	awsStatusAttr, ok := statusAttrs["aws"]
	if !ok {
		t.Error("Expected aws attribute in Status")
	}
	awsStatusObj, ok := awsStatusAttr.(types.Object)
	if !ok || awsStatusObj.IsNull() {
		t.Error("Expected aws status to be non-null object")
	}
}
