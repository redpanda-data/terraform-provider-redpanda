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

package cluster

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models"
)

// generateMinimalModel populates a Cluster model with only enough state for Terraform to
// track an existing cluster and to delete it, if necessary. Used in creation to track
// partially created clusters, and on reading to null out cluster that are found in the
// deleting state and force them to be recreated.
func generateMinimalModel(clusterID string) models.Cluster {
	return models.Cluster{
		Name:                     types.StringNull(),
		ConnectionType:           types.StringNull(),
		CloudProvider:            types.StringNull(),
		ClusterType:              types.StringNull(),
		RedpandaVersion:          types.StringNull(),
		ThroughputTier:           types.StringNull(),
		Region:                   types.StringNull(),
		AllowDeletion:            types.BoolValue(true),
		Tags:                     types.MapNull(types.StringType),
		ResourceGroupID:          types.StringNull(),
		NetworkID:                types.StringNull(),
		ID:                       types.StringValue(clusterID),
		ReadReplicaClusterIDs:    types.ListNull(types.StringType),
		Zones:                    types.ListNull(types.StringType),
		ClusterAPIURL:            types.StringNull(),
		CustomerManagedResources: types.ObjectNull(cmrType),
		KafkaAPI:                 types.ObjectNull(kafkaAPIType),
		HTTPProxy:                types.ObjectNull(httpProxyType),
		SchemaRegistry:           types.ObjectNull(schemaRegistryType),
		AwsPrivateLink:           types.ObjectNull(awsPrivateLinkType),
		GcpPrivateServiceConnect: types.ObjectNull(gcpPrivateServiceConnectType),
		AzurePrivateLink:         types.ObjectNull(azurePrivateLinkType),
	}
}

func getObjectFromAttributes(ctx context.Context, key string, att map[string]attr.Value, diags diag.Diagnostics) (types.Object, diag.Diagnostics) {
	attVal, ok := att[key].(basetypes.ObjectValue)
	if !ok {
		return types.ObjectNull(map[string]attr.Type{}), append(diags, diag.NewErrorDiagnostic(fmt.Sprintf("%s not found", key), "object is missing or malformed"))
	}
	var keyVal types.Object
	if err := attVal.As(ctx, &keyVal, basetypes.ObjectAsOptions{
		UnhandledNullAsEmpty:    true,
		UnhandledUnknownAsEmpty: true,
	}); err != nil {
		return types.ObjectNull(map[string]attr.Type{}), append(diags, diag.NewErrorDiagnostic(fmt.Sprintf("%s not found", key), "value is missing or malformed"))
	}
	return keyVal, diags
}

func getStringFromAttributes(key string, att map[string]attr.Value, diags diag.Diagnostics) (string, diag.Diagnostics) {
	attVal, ok := att[key].(basetypes.ObjectValue)
	if !ok {
		diags.AddError(fmt.Sprintf("%s not found", key), "object is missing or malformed")
		return "", diags
	}
	rt, ok := attVal.Attributes()["arn"].(types.String)
	if !ok {
		diags.AddError(fmt.Sprintf("%s not found", key), "string is missing or malformed")
		return "", diags
	}
	return rt.ValueString(), diags
}

func getBoolFromAttributes(key string, att map[string]attr.Value, diags diag.Diagnostics) (bool, diag.Diagnostics) {
	attVal, ok := att[key].(types.Bool)
	if !ok {
		return false, append(diags, diag.NewErrorDiagnostic(fmt.Sprintf("%s not found", key), "bool is missing or malformed"))
	}
	return attVal.ValueBool(), diags
}

func getListFromAttributes(key string, att map[string]attr.Value, diags diag.Diagnostics) (types.List, diag.Diagnostics) {
	attVal, ok := att[key].(types.List)
	if !ok {
		return types.ListNull(types.StringType), append(diags, diag.NewErrorDiagnostic(fmt.Sprintf("%s not found", key), "list is missing or malformed"))
	}
	return attVal, diags
}
