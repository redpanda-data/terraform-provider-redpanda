package serverlessprivatelink

import (
	"context"
	"errors"
	"fmt"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

const (
	aws = "aws"
)

// generateModel populates the ServerlessPrivateLink model to be persisted to state for Create, Read and Update operations.
func generateModel(ctx context.Context, spl *controlplanev1.ServerlessPrivateLink, allowDeletion types.Bool) (*models.ServerlessPrivateLink, error) {
	output := &models.ServerlessPrivateLink{
		Name:             types.StringValue(spl.Name),
		ResourceGroupID:  types.StringValue(spl.ResourceGroupId),
		CloudProvider:    types.StringValue(utils.CloudProviderToString(spl.Cloudprovider)),
		ServerlessRegion: types.StringValue(spl.ServerlessRegion),
		ID:               types.StringValue(spl.Id),
		State:            types.StringValue(spl.State.String()),
		AllowDeletion:    allowDeletion,
	}

	// Set timestamps if present
	if spl.CreatedAt != nil {
		output.CreatedAt = types.StringValue(spl.CreatedAt.AsTime().Format("2006-01-02T15:04:05Z07:00"))
	}
	if spl.UpdatedAt != nil {
		output.UpdatedAt = types.StringValue(spl.UpdatedAt.AsTime().Format("2006-01-02T15:04:05Z07:00"))
	}

	// Generate cloud provider config based on which variant is present
	cloudProviderConfig, err := generateModelCloudProviderConfig(ctx, spl)
	if err != nil {
		return nil, fmt.Errorf("failed to generate cloud provider config: %w", err)
	}
	output.CloudProviderConfig = cloudProviderConfig

	// Generate status based on which variant is present
	status, err := generateModelStatus(ctx, spl)
	if err != nil {
		return nil, fmt.Errorf("failed to generate status: %w", err)
	}
	output.Status = status

	return output, nil
}

// generateModelCloudProviderConfig converts protobuf cloud provider config to Terraform object
func generateModelCloudProviderConfig(_ context.Context, spl *controlplanev1.ServerlessPrivateLink) (types.Object, error) {
	cloudProvider := utils.CloudProviderToString(spl.Cloudprovider)

	if cloudProvider == aws && spl.HasAwsConfig() {
		awsConfig := spl.GetAwsConfig()
		awsObj, err := generateModelAWSConfig(awsConfig)
		if err != nil {
			return types.ObjectNull(getCloudProviderConfigType()), fmt.Errorf("failed to generate AWS config: %w", err)
		}

		configObj, diags := types.ObjectValue(
			getCloudProviderConfigType(),
			map[string]attr.Value{
				aws: awsObj,
			},
		)
		if diags.HasError() {
			return types.ObjectNull(getCloudProviderConfigType()), fmt.Errorf("failed to create cloud provider config object: %s", diags.Errors()[0].Summary())
		}
		return configObj, nil
	}

	return types.ObjectNull(getCloudProviderConfigType()), fmt.Errorf("failed to generate cloud provider config object: %s", cloudProvider)
}

// generateModelStatus converts protobuf status to Terraform object
func generateModelStatus(_ context.Context, spl *controlplanev1.ServerlessPrivateLink) (types.Object, error) {
	if spl.Status == nil {
		return types.ObjectNull(getStatusType()), nil
	}

	if awsStatus := spl.Status.GetAws(); awsStatus != nil {
		awsObj, err := generateModelAWSStatus(awsStatus)
		if err != nil {
			return types.ObjectNull(getStatusType()), fmt.Errorf("failed to generate AWS status: %w", err)
		}
		statusObj, diags := types.ObjectValue(
			getStatusType(),
			map[string]attr.Value{
				aws: awsObj,
			},
		)
		if diags.HasError() {
			return types.ObjectNull(getStatusType()), fmt.Errorf("failed to create status object: %s", diags.Errors()[0].Summary())
		}
		return statusObj, nil
	}

	return types.ObjectNull(getStatusType()), nil
}

// generateModelAWSConfig converts protobuf AWS config to Terraform object
func generateModelAWSConfig(config *controlplanev1.ServerlessPrivateLink_AWS) (types.Object, error) {
	principals := make([]attr.Value, len(config.AllowedPrincipals))
	for i, p := range config.AllowedPrincipals {
		principals[i] = types.StringValue(p)
	}

	principalsList, diags := types.ListValue(types.StringType, principals)
	if diags.HasError() {
		return types.ObjectNull(getAWSConfigType()), fmt.Errorf("failed to create principals list: %s", diags.Errors()[0].Summary())
	}

	obj, diags := types.ObjectValue(
		getAWSConfigType(),
		map[string]attr.Value{
			"allowed_principals": principalsList,
		},
	)
	if diags.HasError() {
		return types.ObjectNull(getAWSConfigType()), fmt.Errorf("failed to create AWS config object: %s", diags.Errors()[0].Summary())
	}

	return obj, nil
}

// generateModelAWSStatus converts protobuf AWS status to Terraform object
func generateModelAWSStatus(status *controlplanev1.ServerlessPrivateLinkStatus_AWS) (types.Object, error) {
	// Convert availability zones from proto to Terraform list
	var azList types.List
	if len(status.AvailabilityZones) > 0 {
		zones := make([]attr.Value, len(status.AvailabilityZones))
		for i, zone := range status.AvailabilityZones {
			zones[i] = types.StringValue(zone)
		}
		var diags diag.Diagnostics
		azList, diags = types.ListValue(types.StringType, zones)
		if diags.HasError() {
			return types.ObjectNull(getAWSStatusType()), fmt.Errorf("failed to create availability zones list: %s", diags.Errors()[0].Summary())
		}
	} else {
		azList = types.ListNull(types.StringType)
	}

	obj, diags := types.ObjectValue(
		getAWSStatusType(),
		map[string]attr.Value{
			"vpc_endpoint_service_name": types.StringValue(status.VpcEndpointServiceName),
			"availability_zones":        azList,
		},
	)
	if diags.HasError() {
		return types.ObjectNull(getAWSStatusType()), fmt.Errorf("failed to create AWS status object: %s", diags.Errors()[0].Summary())
	}

	return obj, nil
}

// GenerateServerlessPrivateLinkRequest converts Terraform model to protobuf for create operation
func GenerateServerlessPrivateLinkRequest(ctx context.Context, model models.ServerlessPrivateLink) (*controlplanev1.ServerlessPrivateLinkCreate, error) {
	req := &controlplanev1.ServerlessPrivateLinkCreate{
		Name:             model.Name.ValueString(),
		ResourceGroupId:  model.ResourceGroupID.ValueString(),
		ServerlessRegion: model.ServerlessRegion.ValueString(),
	}

	// Set cloud provider enum
	provider, err := utils.StringToCloudProvider(model.CloudProvider.ValueString())
	if err != nil {
		return nil, fmt.Errorf("invalid cloud provider: %w", err)
	}
	req.Cloudprovider = provider

	// Extract and set cloud provider config
	cloudProviderConfig := model.CloudProviderConfig
	if cloudProviderConfig.IsNull() || cloudProviderConfig.IsUnknown() {
		return nil, errors.New("invalid cloud provider")
	}

	// Set cloud provider config based on which is present
	switch model.CloudProvider.ValueString() {
	case aws:
		awsConfig, err := extractAWSConfigFromCloudProviderConfig(ctx, cloudProviderConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to parse AWS config: %w", err)
		}
		req.CloudProviderConfig = &controlplanev1.ServerlessPrivateLinkCreate_AwsConfig{
			AwsConfig: awsConfig,
		}
	default:
		return nil, fmt.Errorf("unsupported cloud provider: %s", model.CloudProvider.ValueString())
	}

	return req, nil
}

// GenerateServerlessPrivateLinkUpdateRequest converts Terraform model to protobuf for update operation
func GenerateServerlessPrivateLinkUpdateRequest(ctx context.Context, model models.ServerlessPrivateLink) (*controlplanev1.UpdateServerlessPrivateLinkRequest, error) {
	req := &controlplanev1.UpdateServerlessPrivateLinkRequest{
		Id: model.ID.ValueString(),
	}

	// Extract and set cloud provider config
	cloudProviderConfig := model.CloudProviderConfig
	if cloudProviderConfig.IsNull() || cloudProviderConfig.IsUnknown() {
		return req, nil
	}

	// Set cloud provider config based on which is present
	if model.CloudProvider.ValueString() == aws {
		awsConfig, err := extractAWSUpdateConfigFromCloudProviderConfig(ctx, cloudProviderConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to parse AWS config: %w", err)
		}
		req.CloudProviderConfig = &controlplanev1.UpdateServerlessPrivateLinkRequest_AwsConfig{
			AwsConfig: awsConfig,
		}
		return req, nil
	}

	return nil, fmt.Errorf("unsupported cloud provider: %s", model.CloudProvider.ValueString())
}

// extractAWSConfigFromCloudProviderConfig extracts AWS config from the cloud_provider_config object for create
func extractAWSConfigFromCloudProviderConfig(_ context.Context, configObj types.Object) (*controlplanev1.ServerlessPrivateLinkCreate_AWS, error) {
	attrs := configObj.Attributes()
	awsAttr, ok := attrs[aws]
	if !ok {
		return nil, errors.New("aws config not found in cloud_provider_config")
	}

	awsObj, ok := awsAttr.(types.Object)
	if !ok || awsObj.IsNull() {
		return nil, errors.New("aws config is required when cloud_provider is 'aws'")
	}

	awsAttrs := awsObj.Attributes()
	allowedPrincipalsAttr, ok := awsAttrs["allowed_principals"]
	if !ok {
		return nil, errors.New("allowed_principals not found in AWS config")
	}

	allowedPrincipalsList, ok := allowedPrincipalsAttr.(types.List)
	if !ok {
		return nil, errors.New("allowed_principals is not a list")
	}

	principals := make([]string, 0, len(allowedPrincipalsList.Elements()))
	for _, elem := range allowedPrincipalsList.Elements() {
		if strElem, ok := elem.(types.String); ok && !strElem.IsNull() {
			principals = append(principals, strElem.ValueString())
		}
	}

	return &controlplanev1.ServerlessPrivateLinkCreate_AWS{
		AllowedPrincipals: principals,
	}, nil
}

// extractAWSUpdateConfigFromCloudProviderConfig extracts AWS config from the cloud_provider_config object for update
func extractAWSUpdateConfigFromCloudProviderConfig(_ context.Context, configObj types.Object) (*controlplanev1.UpdateServerlessPrivateLinkRequest_AWS, error) {
	attrs := configObj.Attributes()
	awsAttr, ok := attrs[aws]
	if !ok {
		return nil, errors.New("aws config not found in cloud_provider_config")
	}

	awsObj, ok := awsAttr.(types.Object)
	if !ok || awsObj.IsNull() {
		return nil, errors.New("aws config is required when cloud_provider is 'aws'")
	}

	awsAttrs := awsObj.Attributes()
	allowedPrincipalsAttr, ok := awsAttrs["allowed_principals"]
	if !ok {
		return nil, errors.New("allowed_principals not found in AWS config")
	}

	allowedPrincipalsList, ok := allowedPrincipalsAttr.(types.List)
	if !ok {
		return nil, errors.New("allowed_principals is not a list")
	}

	principals := make([]string, 0, len(allowedPrincipalsList.Elements()))
	for _, elem := range allowedPrincipalsList.Elements() {
		if strElem, ok := elem.(types.String); ok && !strElem.IsNull() {
			principals = append(principals, strElem.ValueString())
		}
	}

	return &controlplanev1.UpdateServerlessPrivateLinkRequest_AWS{
		AllowedPrincipals: principals,
	}, nil
}

// Type definitions

func getCloudProviderConfigType() map[string]attr.Type {
	return map[string]attr.Type{
		aws: types.ObjectType{AttrTypes: getAWSConfigType()},
	}
}

func getAWSConfigType() map[string]attr.Type {
	return map[string]attr.Type{
		"allowed_principals": types.ListType{ElemType: types.StringType},
	}
}

func getStatusType() map[string]attr.Type {
	return map[string]attr.Type{
		aws: types.ObjectType{AttrTypes: getAWSStatusType()},
	}
}

func getAWSStatusType() map[string]attr.Type {
	return map[string]attr.Type{
		"vpc_endpoint_service_name": types.StringType,
		"availability_zones":        types.ListType{ElemType: types.StringType},
	}
}
