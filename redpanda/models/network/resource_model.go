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

// Package network contains the model for the network resource.
package network

import (
	"context"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// ResourceModel represents the Terraform schema for the network resource.
type ResourceModel struct {
	Name                     types.String   `tfsdk:"name"`
	ResourceGroupID          types.String   `tfsdk:"resource_group_id"`
	CloudProvider            types.String   `tfsdk:"cloud_provider"`
	Region                   types.String   `tfsdk:"region"`
	CidrBlock                types.String   `tfsdk:"cidr_block"`
	ID                       types.String   `tfsdk:"id"`
	ClusterType              types.String   `tfsdk:"cluster_type"`
	CustomerManagedResources types.Object   `tfsdk:"customer_managed_resources"`
	Timeouts                 timeouts.Value `tfsdk:"timeouts"`
}

// GetUpdatedModel updates the Network ResourceModel using a provided Network proto object. It returns an updated model for convenience
func (r *ResourceModel) GetUpdatedModel(ctx context.Context, nw *controlplanev1.Network) (*ResourceModel, diag.Diagnostics) {
	r.CloudProvider = types.StringValue(utils.CloudProviderToString(nw.GetCloudProvider()))
	r.ClusterType = types.StringValue(utils.ClusterTypeToString(nw.GetClusterType()))
	r.ID = types.StringValue(nw.GetId())
	r.Name = types.StringValue(nw.GetName())
	r.Region = types.StringValue(nw.GetRegion())
	r.ResourceGroupID = types.StringValue(nw.GetResourceGroupId())

	if nw.GetCidrBlock() != "" && nw.CidrBlock != "0.0.0.0/0" {
		r.CidrBlock = types.StringValue(nw.GetCidrBlock())
	}

	r.CustomerManagedResources = getCMRNull()
	if nw.HasCustomerManagedResources() {
		diags := r.generateModelCMR(ctx, nw.GetCustomerManagedResources())
		if diags.HasError() {
			diags.AddError("error generating network CustomerManagedResources", "")
			return nil, diags
		}
	}
	// for convenience, return the model
	return r, nil
}

// GetID returns the ID of the Network ResourceModel
func (r *ResourceModel) GetID() string {
	return r.ID.ValueString()
}

// GetNetworkCreate composes a Network Create request from the model data
func (r *ResourceModel) GetNetworkCreate(ctx context.Context) (*controlplanev1.NetworkCreate, diag.Diagnostics) {
	var diags diag.Diagnostics
	cp, err := utils.StringToCloudProvider(r.CloudProvider.ValueString())
	if err != nil {
		diags.Append(diag.NewErrorDiagnostic("error getting cloud provider for network", err.Error()))
	}
	ct, err := utils.StringToClusterType(r.ClusterType.ValueString())
	if err != nil {
		diags.Append(diag.NewErrorDiagnostic("error getting cluster type for network", err.Error()))
	}

	// Return early if we have errors so far
	if diags.HasError() {
		return nil, diags
	}

	cmr, d := r.generateNetworkCMR(ctx)
	if d.HasError() {
		diags.Append(d...)
		diags.AddError("error attempting to generate Network Customer Managed Resources", "")
		return nil, diags
	}
	return &controlplanev1.NetworkCreate{
		Name:                     r.Name.ValueString(),
		CidrBlock:                r.CidrBlock.ValueString(),
		Region:                   r.Region.ValueString(),
		CloudProvider:            cp,
		ResourceGroupId:          r.ResourceGroupID.ValueString(),
		ClusterType:              ct,
		CustomerManagedResources: cmr,
	}, diags
}

func (r *ResourceModel) generateNetworkCMR(ctx context.Context) (*controlplanev1.Network_CustomerManagedResources, diag.Diagnostics) {
	cmr := &controlplanev1.Network_CustomerManagedResources{}

	if r.CustomerManagedResources.IsNull() {
		return nil, nil
	}

	// If CustomerManagedResources is not null, process it
	switch r.CloudProvider.ValueString() {
	case "aws":
		var cmrObj types.Object
		if d := r.CustomerManagedResources.As(ctx, &cmrObj, basetypes.ObjectAsOptions{
			UnhandledNullAsEmpty:    true,
			UnhandledUnknownAsEmpty: true,
		}); d.HasError() {
			d.AddError("failed to get CustomerManagedResources from network model", "could not get CustomerManagedResources from network model")
			return nil, d
		}
		awsCMR, err := generateNetworkAWSCMR(ctx, cmrObj)
		if err != nil {
			var dgs diag.Diagnostics
			dgs.AddError("failed to generate AWS CustomerManagedResources", err.Error())
			return nil, dgs
		}
		cmr.SetAws(awsCMR)
		return cmr, nil
	case "gcp":
		var cmrObj types.Object
		if d := r.CustomerManagedResources.As(ctx, &cmrObj, basetypes.ObjectAsOptions{
			UnhandledNullAsEmpty:    true,
			UnhandledUnknownAsEmpty: true,
		}); d.HasError() {
			d.AddError("failed to get CustomerManagedResources from network model", "could not get CustomerManagedResources from network model")
			return nil, d
		}
		gcpCMR, err := generateNetworkGCPCMR(ctx, cmrObj)
		if err != nil {
			var dgs diag.Diagnostics
			dgs.AddError("failed to generate GCP CustomerManagedResources", err.Error())
			return nil, dgs
		}
		cmr.SetGcp(gcpCMR)
		return cmr, nil
	default:
		return nil, nil
	}
}

func (r *ResourceModel) generateModelCMR(ctx context.Context, cmr *controlplanev1.Network_CustomerManagedResources) diag.Diagnostics {
	var diags diag.Diagnostics
	switch {
	case cmr.HasAws():
		awsObj, d := generateModelAWSCMR(ctx, cmr.GetAws())
		if d.HasError() {
			diags.AddError("failed to generate AWS CustomerManagedResources object", "could not create AWS CustomerManagedResources object")
			diags.Append(d...)
			return diags
		}
		cmrObj, d := types.ObjectValue(getCMRType(), map[string]attr.Value{
			"aws": awsObj,
			"gcp": types.ObjectNull(getGcpType()),
		})
		if d.HasError() {
			diags.AddError("failed to generate CMR object", "could not create CMR object")
			diags.Append(d...)
			return diags
		}
		r.CustomerManagedResources = cmrObj
	case cmr.HasGcp():
		gcpObj, d := generateModelGCPCMR(cmr.GetGcp())
		if d.HasError() {
			diags.AddError("failed to generate GCP CustomerManagedResources object", "could not create GCP CustomerManagedResources object")
			diags.Append(d...)
			return diags
		}
		// Create final CMR object and assign to output
		cmrObj, d := types.ObjectValue(getCMRType(), map[string]attr.Value{
			"gcp": gcpObj,
			"aws": types.ObjectNull(getAwsType()),
		})
		if d.HasError() {
			diags.AddError("failed to generate CMR object", "could not create CMR object")
			diags.Append(d...)
			return diags
		}
		r.CustomerManagedResources = cmrObj
	default:
		// TODO should this be an error declaring the cloud provider unsupported?
		r.CustomerManagedResources = getCMRNull()
		return nil
	}
	return nil
}
