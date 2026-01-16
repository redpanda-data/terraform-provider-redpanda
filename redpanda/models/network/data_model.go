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

package network

import (
	"context"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// GetID returns the ID of the Network DataModel
func (r *DataModel) GetID() string {
	return r.ID.ValueString()
}

// GetUpdatedModel updates the Network DataModel using a provided Network proto object. It returns an updated model for convenience
func (r *DataModel) GetUpdatedModel(ctx context.Context, nw *controlplanev1.Network) (*DataModel, diag.Diagnostics) {
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

// TODO consider pattern with config also passed in and reconciled (probably not needed here)

// GetNetworkCreate composes a Network Create request from the model data
func (r *DataModel) GetNetworkCreate(ctx context.Context) (*controlplanev1.NetworkCreate, diag.Diagnostics) {
	var diags diag.Diagnostics
	cp, err := utils.StringToCloudProvider(r.CloudProvider.ValueString())
	if err != nil {
		diags.Append(diag.NewErrorDiagnostic("error getting cloud provider for network", err.Error()))
	}
	ct, err := utils.StringToClusterType(r.ClusterType.ValueString())
	if err != nil {
		diags.Append(diag.NewErrorDiagnostic("error getting cluster type for network", err.Error()))
	}
	cmr, d := r.generateNetworkCMR(ctx)
	if d.HasError() {
		d.AddError("error attempting to generate Network Customer Managed Resources", "")
		return nil, d
	}
	return &controlplanev1.NetworkCreate{
		Name:                     r.Name.ValueString(),
		CidrBlock:                r.CidrBlock.ValueString(),
		Region:                   r.Region.ValueString(),
		CloudProvider:            cp,
		ResourceGroupId:          r.ResourceGroupID.ValueString(),
		ClusterType:              ct,
		CustomerManagedResources: cmr,
	}, nil
}

func (r *DataModel) generateNetworkCMR(ctx context.Context) (*controlplanev1.Network_CustomerManagedResources, diag.Diagnostics) {
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

func (r *DataModel) generateModelCMR(ctx context.Context, cmr *controlplanev1.Network_CustomerManagedResources) diag.Diagnostics {
	var diags diag.Diagnostics
	switch {
	case cmr.HasAws():
		awsObj, d := generateModelAWSCMR(ctx, cmr.GetAws())
		if d.HasError() {
			// you may get some ide notifications about potential nil calls but this is entirely safe and I have thoroughly tested it
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
