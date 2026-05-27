// Copyright 2025 Redpanda Data, Inc.
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

package secret

import (
	"context"

	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ResourceModel represents the Terraform schema for the secret resource.
type ResourceModel struct {
	AllowDeletion     types.Bool   `tfsdk:"allow_deletion"`
	ClusterAPIURL     types.String `tfsdk:"cluster_api_url"`
	ID                types.String `tfsdk:"id"`
	Labels            types.Map    `tfsdk:"labels"`
	Name              types.String `tfsdk:"name"`
	Scopes            types.Set    `tfsdk:"scopes"`
	SecretData        types.String `tfsdk:"secret_data"`
	SecretDataVersion types.Int64  `tfsdk:"secret_data_version"`
}

// serverManagedLabelKeys names label keys that the Cloud API auto-injects
// on Create. The provider strips them in Flatten so the user's planned
// `labels` map round-trips cleanly through Refresh — otherwise the
// framework reports `Provider produced inconsistent result after apply`.
var serverManagedLabelKeys = map[string]bool{"owner": true}

func stripServerManagedLabels(in map[string]string) map[string]string {
	if len(in) == 0 {
		return in
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		if serverManagedLabelKeys[k] {
			continue
		}
		out[k] = v
	}
	return out
}

// GetUpdatedModel populates a Secret resource model from the dataplane Secret response.
func GetUpdatedModel(ctx context.Context, s *dataplanev1.Secret) *ResourceModel {
	out := &ResourceModel{
		Name: types.StringValue(s.GetId()),
		ID:   types.StringValue(s.GetId()),
	}

	scopes := s.GetScopes()
	scopeStrs := make([]string, 0, len(scopes))
	for _, sc := range scopes {
		scopeStrs = append(scopeStrs, sc.String())
	}
	scopesVal, _ := types.SetValueFrom(ctx, types.StringType, scopeStrs)
	out.Scopes = scopesVal

	labels := stripServerManagedLabels(s.GetLabels())
	if len(labels) == 0 {
		out.Labels = types.MapNull(types.StringType)
	} else {
		labelsVal, _ := types.MapValueFrom(ctx, types.StringType, labels)
		out.Labels = labelsVal
	}
	return out
}

// StringsToScopes converts a Terraform set of scope-name strings to proto Scope enum values.
func StringsToScopes(ctx context.Context, l types.Set) ([]dataplanev1.Scope, diag.Diagnostics) {
	if l.IsNull() || l.IsUnknown() {
		return nil, nil
	}
	var ss []string
	diags := l.ElementsAs(ctx, &ss, false)
	if diags.HasError() {
		return nil, diags
	}
	out := make([]dataplanev1.Scope, 0, len(ss))
	for _, s := range ss {
		v, ok := dataplanev1.Scope_value[s]
		if !ok {
			diags.AddError("invalid scope", "unknown scope value: "+s)
			return nil, diags
		}
		out = append(out, dataplanev1.Scope(v))
	}
	return out, diags
}
