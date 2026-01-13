// Copyright 2023 Redpanda Data, Inc.
//
//	Licensed under the Apache License, Version 2.0 (the "License");
//	you may not use this file except in compliance with the License.
//	You may obtain a copy of the License at
//
//	  http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS,
//	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//	See the License for the specific language governing permissions and
//	limitations under the License.

// Package pipeline contains the model for the pipeline resource.
package pipeline

import (
	"context"

	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// ResourceModel represents the Terraform schema for the pipeline resource.
type ResourceModel struct {
	ID             types.String   `tfsdk:"id"`
	ClusterAPIURL  types.String   `tfsdk:"cluster_api_url"`
	DisplayName    types.String   `tfsdk:"display_name"`
	Description    types.String   `tfsdk:"description"`
	ConfigYaml     types.String   `tfsdk:"config_yaml"`
	State          types.String   `tfsdk:"state"`
	URL            types.String   `tfsdk:"url"`
	Resources      types.Object   `tfsdk:"resources"`
	ServiceAccount types.Object   `tfsdk:"service_account"`
	Tags           types.Map      `tfsdk:"tags"`
	AllowDeletion  types.Bool     `tfsdk:"allow_deletion"`
	Timeouts       timeouts.Value `tfsdk:"timeouts"`
}

// Resources defines the structure for pipeline resource allocation.
type Resources struct {
	MemoryShares types.String `tfsdk:"memory_shares"`
	CpuShares    types.String `tfsdk:"cpu_shares"`
}

// ServiceAccount defines the structure for pipeline service account credentials.
type ServiceAccount struct {
	ClientID      types.String `tfsdk:"client_id"`
	ClientSecret  types.String `tfsdk:"client_secret"`
	SecretVersion types.Int64  `tfsdk:"secret_version"`
}

// ContingentFields contains fields that preserve plan/prior values.
type ContingentFields struct {
	ClusterAPIURL  types.String
	AllowDeletion  types.Bool
	Resources      types.Object
	ServiceAccount types.Object
	State          types.String
	Timeouts       timeouts.Value
}

// GetUpdatedModel populates the ResourceModel from a protobuf pipeline response.
func (r *ResourceModel) GetUpdatedModel(ctx context.Context, pipeline *dataplanev1.Pipeline, contingent ContingentFields) (*ResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	currentAPIState := StateToString(pipeline.GetState())
	var stateToStore string
	if !contingent.State.IsNull() && !contingent.State.IsUnknown() {
		priorStateVal := contingent.State.ValueString()
		if StatesEquivalent(priorStateVal, currentAPIState) {
			stateToStore = priorStateVal
		} else {
			stateToStore = DesiredStateFromAPIState(currentAPIState)
		}
	} else {
		stateToStore = DesiredStateFromAPIState(currentAPIState)
	}

	r.ID = types.StringValue(pipeline.GetId())
	r.ClusterAPIURL = contingent.ClusterAPIURL
	r.DisplayName = types.StringValue(pipeline.GetDisplayName())
	r.Description = types.StringValue(pipeline.GetDescription())
	r.ConfigYaml = types.StringValue(pipeline.GetConfigYaml())
	r.State = types.StringValue(stateToStore)
	r.URL = types.StringValue(pipeline.GetUrl())
	r.AllowDeletion = contingent.AllowDeletion
	r.Timeouts = contingent.Timeouts

	// Handle resources
	resourcesObj, d := r.generateModelResources(pipeline, contingent.Resources)
	diags.Append(d...)
	r.Resources = resourcesObj

	// Handle service account
	serviceAccountObj, d := r.generateModelServiceAccount(pipeline, contingent.ServiceAccount)
	diags.Append(d...)
	r.ServiceAccount = serviceAccountObj

	// Handle tags
	tagsMap, d := r.generateModelTags(ctx, pipeline)
	diags.Append(d...)
	r.Tags = tagsMap

	return r, diags
}

// generateModelResources converts pipeline resources from API to Terraform types.
func (*ResourceModel) generateModelResources(pipeline *dataplanev1.Pipeline, plannedResources types.Object) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics

	switch {
	case !plannedResources.IsNull() && !plannedResources.IsUnknown():
		return plannedResources, diags
	case pipeline.HasResources():
		res := pipeline.GetResources()
		resourcesObj, d := types.ObjectValue(GetResourcesType(), map[string]attr.Value{
			FieldMemoryShares: types.StringValue(res.GetMemoryShares()),
			FieldCPUShares:    types.StringValue(res.GetCpuShares()),
		})
		diags.Append(d...)
		return resourcesObj, diags
	default:
		return types.ObjectNull(GetResourcesType()), diags
	}
}

// generateModelServiceAccount converts pipeline service account from API to Terraform types.
// Since client_secret is write-only (not returned by API), we preserve the planned value.
func (*ResourceModel) generateModelServiceAccount(pipeline *dataplanev1.Pipeline, plannedServiceAccount types.Object) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Always preserve the planned value if set, since client_secret is write-only
	if !plannedServiceAccount.IsNull() && !plannedServiceAccount.IsUnknown() {
		return plannedServiceAccount, diags
	}

	// If API returns a service account (with client_id), create object with null secret and version
	if pipeline.HasServiceAccount() {
		sa := pipeline.GetServiceAccount()
		serviceAccountObj, d := types.ObjectValue(GetServiceAccountType(), map[string]attr.Value{
			FieldClientID:      types.StringValue(sa.GetClientId()),
			FieldClientSecret:  types.StringNull(),
			FieldSecretVersion: types.Int64Null(),
		})
		diags.Append(d...)
		return serviceAccountObj, diags
	}

	return types.ObjectNull(GetServiceAccountType()), diags
}

// generateModelTags converts pipeline tags from API to Terraform types.
func (*ResourceModel) generateModelTags(ctx context.Context, pipeline *dataplanev1.Pipeline) (types.Map, diag.Diagnostics) {
	var diags diag.Diagnostics

	if pipeline.GetTags() != nil && len(pipeline.GetTags()) > 0 {
		tagsMap, d := types.MapValueFrom(ctx, types.StringType, pipeline.GetTags())
		diags.Append(d...)
		return tagsMap, diags
	}
	return types.MapNull(types.StringType), diags
}

// ExtractResources converts Resources from Terraform model to API type.
func (r *ResourceModel) ExtractResources(ctx context.Context) (*dataplanev1.Pipeline_Resources, diag.Diagnostics) {
	var diags diag.Diagnostics

	if r.Resources.IsNull() || r.Resources.IsUnknown() {
		return nil, diags
	}

	var resources Resources
	diags.Append(r.Resources.As(ctx, &resources, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return nil, diags
	}

	result := &dataplanev1.Pipeline_Resources{}
	if !resources.MemoryShares.IsNull() && !resources.MemoryShares.IsUnknown() {
		result.MemoryShares = resources.MemoryShares.ValueString()
	}
	if !resources.CpuShares.IsNull() && !resources.CpuShares.IsUnknown() {
		result.CpuShares = resources.CpuShares.ValueString()
	}

	return result, diags
}

// ExtractServiceAccount converts ServiceAccount from Terraform model to API type.
func (r *ResourceModel) ExtractServiceAccount(ctx context.Context) (*dataplanev1.Pipeline_ServiceAccount, diag.Diagnostics) {
	var diags diag.Diagnostics

	if r.ServiceAccount.IsNull() || r.ServiceAccount.IsUnknown() {
		return nil, diags
	}

	var serviceAccount ServiceAccount
	diags.Append(r.ServiceAccount.As(ctx, &serviceAccount, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return nil, diags
	}

	result := &dataplanev1.Pipeline_ServiceAccount{}
	if !serviceAccount.ClientID.IsNull() && !serviceAccount.ClientID.IsUnknown() {
		result.ClientId = serviceAccount.ClientID.ValueString()
	}
	if !serviceAccount.ClientSecret.IsNull() && !serviceAccount.ClientSecret.IsUnknown() {
		result.ClientSecret = serviceAccount.ClientSecret.ValueString()
	}

	return result, diags
}

// ExtractTags converts Tags from Terraform model to API type.
func (r *ResourceModel) ExtractTags(ctx context.Context) (map[string]string, diag.Diagnostics) {
	var diags diag.Diagnostics

	if r.Tags.IsNull() || r.Tags.IsUnknown() {
		return nil, diags
	}

	tags := make(map[string]string)
	diags.Append(r.Tags.ElementsAs(ctx, &tags, false)...)

	return tags, diags
}
