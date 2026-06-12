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

// Package secret contains the implementation of the Secret resource following the Terraform framework interfaces.
package secret

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"buf.build/gen/go/redpandadata/dataplane/grpc/go/redpanda/api/dataplane/v1/dataplanev1grpc"
	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/modelconv"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/base"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	secretmodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/secret"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// Per-RPC retry budget for dataplane calls (e.g., the freshly-provisioned-cluster
// DNS-propagation window). Sized to fit ~5 attempts under Retry's 1s→60s cap.
const dataplaneRetryTimeout = 2 * time.Minute

var (
	_ resource.Resource                = &Secret{}
	_ resource.ResourceWithConfigure   = &Secret{}
	_ resource.ResourceWithImportState = &Secret{}
)

// Secret represents the Secret Terraform resource.
type Secret struct {
	base.ResourceBase

	SecretClient dataplanev1grpc.SecretServiceClient

	resData config.Resource
}

// NewSecret constructs a Secret resource.
func NewSecret() *Secret {
	s := &Secret{}
	s.ResourceBase = base.NewResourceBase(
		"redpanda_secret",
		func(context.Context) rschema.Schema { return ResourceSecretSchema() },
		func(p config.Resource) { s.resData = p },
	)
	return s
}

// Create creates a Secret resource.
func (s *Secret) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var model secretmodel.ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &model)...)

	var cfg secretmodel.ResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := s.createSecretClient(ctx, model.ClusterAPIURL.ValueString()); err != nil {
		resp.Diagnostics.AddError("failed to create secret client", utils.DeserializeGrpcError(err))
		return
	}

	scopes, diags := secretmodel.StringsToScopes(ctx, model.Scopes)
	resp.Diagnostics.Append(diags...)
	labels, diags := modelconv.MapToStrings(ctx, model.Labels)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var createdSecret *dataplanev1.Secret
	err := utils.Retry(ctx, dataplaneRetryTimeout, func() *utils.RetryError {
		created, rpcErr := s.SecretClient.CreateSecret(ctx, &dataplanev1.CreateSecretRequest{
			Id:         model.Name.ValueString(),
			Labels:     labels,
			Scopes:     scopes,
			SecretData: []byte(cfg.SecretData.ValueString()),
		})
		if rpcErr == nil {
			createdSecret = created.GetSecret()
			return nil
		}
		// Adopt the existing secret on AlreadyExists from a prior retry's lost response.
		if utils.IsAlreadyExists(rpcErr) {
			if got, getErr := s.SecretClient.GetSecret(ctx, &dataplanev1.GetSecretRequest{Id: model.Name.ValueString()}); getErr == nil && got.GetSecret() != nil {
				createdSecret = got.GetSecret()
				return nil
			}
			return utils.NonRetryableError(rpcErr)
		}
		// Probe before retrying so the next attempt doesn't trip AlreadyExists.
		if utils.IsUnavailable(rpcErr) {
			if got, getErr := s.SecretClient.GetSecret(ctx, &dataplanev1.GetSecretRequest{Id: model.Name.ValueString()}); getErr == nil && got.GetSecret() != nil {
				createdSecret = got.GetSecret()
				return nil
			}
			return utils.RetryableError(rpcErr)
		}
		return utils.NonRetryableError(rpcErr)
	})
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to create secret %q", model.Name.ValueString()), utils.DeserializeGrpcError(err))
		return
	}

	persist, persistDiags := secretmodel.GetUpdatedModel(ctx, createdSecret)
	resp.Diagnostics.Append(persistDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	persist.ClusterAPIURL = model.ClusterAPIURL
	persist.AllowDeletion = model.AllowDeletion
	persist.SecretDataVersion = effectiveVersion(model.SecretDataVersion)

	resp.Diagnostics.Append(resp.State.Set(ctx, persist)...)
}

// Read reads the state of the Secret resource.
func (s *Secret) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var model secretmodel.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := model.Name.ValueString()

	if err := s.createSecretClient(ctx, model.ClusterAPIURL.ValueString()); err != nil {
		action, diags := utils.HandleGracefulRemoval(ctx, "secret", name, model.AllowDeletion, err, "create secret client")
		resp.Diagnostics.Append(diags...)
		if action == utils.RemoveFromState {
			resp.State.RemoveResource(ctx)
		}
		return
	}

	var got *dataplanev1.GetSecretResponse
	err := utils.Retry(ctx, dataplaneRetryTimeout, func() *utils.RetryError {
		var rpcErr error
		got, rpcErr = s.SecretClient.GetSecret(ctx, &dataplanev1.GetSecretRequest{Id: name})
		if rpcErr != nil {
			if utils.IsUnavailable(rpcErr) {
				return utils.RetryableError(rpcErr)
			}
			return utils.NonRetryableError(rpcErr)
		}
		return nil
	})
	if err != nil {
		action, diags := utils.HandleGracefulRemoval(ctx, "secret", name, model.AllowDeletion, err, "get secret")
		resp.Diagnostics.Append(diags...)
		if action == utils.RemoveFromState {
			resp.State.RemoveResource(ctx)
		}
		return
	}

	persist, persistDiags := secretmodel.GetUpdatedModel(ctx, got.GetSecret())
	resp.Diagnostics.Append(persistDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	persist.ClusterAPIURL = model.ClusterAPIURL
	persist.AllowDeletion = model.AllowDeletion
	persist.SecretDataVersion = effectiveVersion(model.SecretDataVersion)

	resp.Diagnostics.Append(resp.State.Set(ctx, persist)...)
}

// Update updates the Secret resource.
func (s *Secret) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state secretmodel.ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	var cfg secretmodel.ResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	versionChanged := !plan.SecretDataVersion.Equal(state.SecretDataVersion)
	scopesChanged := !plan.Scopes.Equal(state.Scopes)

	if !versionChanged && !scopesChanged {
		// No server-side change. Just persist any TF-only metadata.
		state.AllowDeletion = plan.AllowDeletion
		resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
		return
	}

	if err := s.createSecretClient(ctx, plan.ClusterAPIURL.ValueString()); err != nil {
		resp.Diagnostics.AddError("failed to create secret client", utils.DeserializeGrpcError(err))
		return
	}

	scopes, diags := secretmodel.StringsToScopes(ctx, plan.Scopes)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := &dataplanev1.UpdateSecretRequest{
		Id:     plan.Name.ValueString(),
		Scopes: scopes,
	}
	if versionChanged {
		updateReq.SecretData = []byte(cfg.SecretData.ValueString())
	}

	var updated *dataplanev1.UpdateSecretResponse
	err := utils.Retry(ctx, dataplaneRetryTimeout, func() *utils.RetryError {
		var rpcErr error
		updated, rpcErr = s.SecretClient.UpdateSecret(ctx, updateReq)
		if rpcErr != nil {
			if utils.IsUnavailable(rpcErr) {
				return utils.RetryableError(rpcErr)
			}
			return utils.NonRetryableError(rpcErr)
		}
		return nil
	})
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to update secret %q", plan.Name.ValueString()), utils.DeserializeGrpcError(err))
		return
	}

	persist, persistDiags := secretmodel.GetUpdatedModel(ctx, updated.GetSecret())
	resp.Diagnostics.Append(persistDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	persist.ClusterAPIURL = plan.ClusterAPIURL
	persist.AllowDeletion = plan.AllowDeletion
	persist.SecretDataVersion = effectiveVersion(plan.SecretDataVersion)

	resp.Diagnostics.Append(resp.State.Set(ctx, persist)...)
}

// Delete deletes the Secret resource.
func (s *Secret) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var model secretmodel.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := model.Name.ValueString()

	if !model.AllowDeletion.IsNull() && !model.AllowDeletion.ValueBool() {
		resp.Diagnostics.AddError("secret deletion not allowed", "allow_deletion is set to false")
		return
	}

	if err := s.createSecretClient(ctx, model.ClusterAPIURL.ValueString()); err != nil {
		_, diags := utils.HandleGracefulRemoval(ctx, "secret", name, model.AllowDeletion, err, "create secret client")
		resp.Diagnostics.Append(diags...)
		return
	}

	err := utils.Retry(ctx, dataplaneRetryTimeout, func() *utils.RetryError {
		_, rpcErr := s.SecretClient.DeleteSecret(ctx, &dataplanev1.DeleteSecretRequest{Id: name})
		if rpcErr != nil {
			if utils.IsUnavailable(rpcErr) {
				return utils.RetryableError(rpcErr)
			}
			return utils.NonRetryableError(rpcErr)
		}
		return nil
	})
	if err != nil {
		_, diags := utils.HandleGracefulRemoval(ctx, "secret", name, model.AllowDeletion, err, "delete secret")
		resp.Diagnostics.Append(diags...)
		return
	}
}

// ImportState imports the Secret resource. Format: <name>,<cluster_id>
func (s *Secret) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ",", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			fmt.Sprintf("wrong import ID format: %v", req.ID),
			"Import ID format is <secret_name>,<cluster_id>",
		)
		return
	}
	name, clusterID := parts[0], parts[1]

	dataplaneURL, err := s.CpCl.DataplaneURLForCluster(ctx, clusterID)
	if err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("failed to resolve dataplane URL for cluster %q; import format is <secret_name>,<cluster_id>", clusterID),
			err.Error(),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), types.StringValue(name))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(name))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("cluster_api_url"), types.StringValue(dataplaneURL))...)
	resp.Diagnostics.Append(utils.ImportStateBoolFromSchemaDefault(ctx, ResourceSecretSchema(), &resp.State, "allow_deletion")...)
}

func (s *Secret) createSecretClient(ctx context.Context, clusterURL string) error {
	if s.SecretClient != nil {
		return nil
	}
	if clusterURL == "" {
		return errors.New("unable to create client with empty target cluster API URL")
	}
	if s.resData.DataplaneConnPool == nil {
		return errors.New("provider not configured: dataplane connection pool is nil")
	}
	conn, err := s.resData.DataplaneConnPool.GetConnection(ctx, clusterURL)
	if err != nil {
		return fmt.Errorf("unable to open a connection with the cluster API: %v", err)
	}
	s.SecretClient = dataplanev1grpc.NewSecretServiceClient(conn)
	return nil
}

func effectiveVersion(v types.Int64) types.Int64 {
	if v.IsNull() || v.IsUnknown() {
		return types.Int64Value(0)
	}
	return v
}
