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

// Package shadowlink implements the redpanda_shadow_link resource.
package shadowlink

import (
	"context"
	"fmt"
	"strings"
	"time"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/base"
	shadowlinkmodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/shadowlink"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

var objOpts = basetypes.ObjectAsOptions{UnhandledNullAsEmpty: true, UnhandledUnknownAsEmpty: true}

// preserveSensitiveFromPrev copies sensitive fields (tls key, scram/plain
// password, schema-registry basic password and PEM key) from prev into state
// after Flatten. The Read API masks these, so Flatten alone would zero them
// and force perpetual drift.
func preserveSensitiveFromPrev(ctx context.Context, state, prev *shadowlinkmodel.ResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if prev == nil {
		return diags
	}
	diags.Append(preserveClientOptionsSecrets(ctx, state, prev)...)
	diags.Append(preserveSchemaRegistrySecrets(ctx, state, prev)...)
	return diags
}

func preserveClientOptionsSecrets(ctx context.Context, state, prev *shadowlinkmodel.ResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if prev.ClientOptions.IsNull() || prev.ClientOptions.IsUnknown() {
		return diags
	}
	if state.ClientOptions.IsNull() || state.ClientOptions.IsUnknown() {
		return diags
	}

	var prevCO, stateCO shadowlinkmodel.ClientOptionsModel
	diags.Append(prev.ClientOptions.As(ctx, &prevCO, objOpts)...)
	diags.Append(state.ClientOptions.As(ctx, &stateCO, objOpts)...)
	if diags.HasError() {
		return diags
	}

	// tls_settings.key
	if !prevCO.TLSSettings.IsNull() && !stateCO.TLSSettings.IsNull() {
		var prevTLS, stateTLS shadowlinkmodel.ClientOptionsTLSSettingsModel
		diags.Append(prevCO.TLSSettings.As(ctx, &prevTLS, objOpts)...)
		diags.Append(stateCO.TLSSettings.As(ctx, &stateTLS, objOpts)...)
		if !diags.HasError() && !prevTLS.Key.IsNull() && !prevTLS.Key.IsUnknown() {
			stateTLS.Key = prevTLS.Key
			obj, d := types.ObjectValueFrom(ctx, shadowlinkmodel.ClientOptionsTLSSettingsAttrTypes(), &stateTLS)
			diags.Append(d...)
			if !d.HasError() {
				stateCO.TLSSettings = obj
			}
		}
	}

	// authentication_configuration.{scram,plain}_configuration.password
	if !prevCO.AuthenticationConfiguration.IsNull() && !stateCO.AuthenticationConfiguration.IsNull() {
		var prevAuth, stateAuth shadowlinkmodel.ClientOptionsAuthenticationConfigurationModel
		diags.Append(prevCO.AuthenticationConfiguration.As(ctx, &prevAuth, objOpts)...)
		diags.Append(stateCO.AuthenticationConfiguration.As(ctx, &stateAuth, objOpts)...)
		if !diags.HasError() {
			preserveAuthPassword(ctx, &stateAuth, &prevAuth, &diags)
			obj, d := types.ObjectValueFrom(ctx, shadowlinkmodel.ClientOptionsAuthenticationConfigurationAttrTypes(), &stateAuth)
			diags.Append(d...)
			if !d.HasError() {
				stateCO.AuthenticationConfiguration = obj
			}
		}
	}

	obj, d := types.ObjectValueFrom(ctx, shadowlinkmodel.ClientOptionsAttrTypes(), &stateCO)
	diags.Append(d...)
	if !d.HasError() {
		state.ClientOptions = obj
	}
	return diags
}

func preserveAuthPassword(ctx context.Context, state, prev *shadowlinkmodel.ClientOptionsAuthenticationConfigurationModel, diags *diag.Diagnostics) {
	if !prev.ScramConfiguration.IsNull() && !state.ScramConfiguration.IsNull() {
		state.ScramConfiguration = preserveMaskedAttr(ctx, state.ScramConfiguration, prev.ScramConfiguration,
			shadowlinkmodel.ClientOptionsAuthenticationConfigurationScramConfigurationAttrTypes(), "password", diags)
	}
	if !prev.PlainConfiguration.IsNull() && !state.PlainConfiguration.IsNull() {
		state.PlainConfiguration = preserveMaskedAttr(ctx, state.PlainConfiguration, prev.PlainConfiguration,
			shadowlinkmodel.ClientOptionsAuthenticationConfigurationPlainConfigurationAttrTypes(), "password", diags)
	}
}

func preserveMaskedAttr(_ context.Context, stateObj, prevObj types.Object, attrTypes map[string]attr.Type, name string, diags *diag.Diagnostics) types.Object {
	stateAttrs := stateObj.Attributes()
	prevAttrs := prevObj.Attributes()
	prevVal, ok := prevAttrs[name].(types.String)
	if !ok || prevVal.IsNull() || prevVal.IsUnknown() {
		return stateObj
	}
	stateAttrs[name] = prevVal
	obj, d := types.ObjectValue(attrTypes, stateAttrs)
	diags.Append(d...)
	if d.HasError() {
		return stateObj
	}
	return obj
}

// preserveSchemaRegistrySecrets restores the shadow_schema_registry_api
// credentials the Read API masks: auth_options.basic.password and
// tls_settings.tls_pem_settings.key.
func preserveSchemaRegistrySecrets(ctx context.Context, state, prev *shadowlinkmodel.ResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if prev.SchemaRegistrySyncOptions.IsNull() || prev.SchemaRegistrySyncOptions.IsUnknown() {
		return diags
	}
	if state.SchemaRegistrySyncOptions.IsNull() || state.SchemaRegistrySyncOptions.IsUnknown() {
		return diags
	}

	var prevSR, stateSR shadowlinkmodel.SchemaRegistrySyncOptionsModel
	diags.Append(prev.SchemaRegistrySyncOptions.As(ctx, &prevSR, objOpts)...)
	diags.Append(state.SchemaRegistrySyncOptions.As(ctx, &stateSR, objOpts)...)
	if diags.HasError() {
		return diags
	}
	if prevSR.ShadowSchemaRegistryAPI.IsNull() || prevSR.ShadowSchemaRegistryAPI.IsUnknown() {
		return diags
	}
	if stateSR.ShadowSchemaRegistryAPI.IsNull() || stateSR.ShadowSchemaRegistryAPI.IsUnknown() {
		return diags
	}

	var prevAPI, stateAPI shadowlinkmodel.SchemaRegistrySyncOptionsShadowSchemaRegistryAPIModel
	diags.Append(prevSR.ShadowSchemaRegistryAPI.As(ctx, &prevAPI, objOpts)...)
	diags.Append(stateSR.ShadowSchemaRegistryAPI.As(ctx, &stateAPI, objOpts)...)
	if diags.HasError() {
		return diags
	}

	preserveBasicPassword(ctx, &stateAPI, &prevAPI, &diags)
	preservePemKey(ctx, &stateAPI, &prevAPI, &diags)
	if diags.HasError() {
		return diags
	}

	apiObj, d := types.ObjectValueFrom(ctx, shadowlinkmodel.SchemaRegistrySyncOptionsShadowSchemaRegistryAPIAttrTypes(), &stateAPI)
	diags.Append(d...)
	if d.HasError() {
		return diags
	}
	stateSR.ShadowSchemaRegistryAPI = apiObj

	srObj, d := types.ObjectValueFrom(ctx, shadowlinkmodel.SchemaRegistrySyncOptionsAttrTypes(), &stateSR)
	diags.Append(d...)
	if !d.HasError() {
		state.SchemaRegistrySyncOptions = srObj
	}
	return diags
}

func preserveBasicPassword(ctx context.Context, state, prev *shadowlinkmodel.SchemaRegistrySyncOptionsShadowSchemaRegistryAPIModel, diags *diag.Diagnostics) {
	if prev.AuthOptions.IsNull() || prev.AuthOptions.IsUnknown() || state.AuthOptions.IsNull() || state.AuthOptions.IsUnknown() {
		return
	}
	var prevAuth, stateAuth shadowlinkmodel.SchemaRegistrySyncOptionsShadowSchemaRegistryAPIAuthOptionsModel
	diags.Append(prev.AuthOptions.As(ctx, &prevAuth, objOpts)...)
	diags.Append(state.AuthOptions.As(ctx, &stateAuth, objOpts)...)
	if diags.HasError() || prevAuth.Basic.IsNull() || stateAuth.Basic.IsNull() {
		return
	}
	stateAuth.Basic = preserveMaskedAttr(ctx, stateAuth.Basic, prevAuth.Basic,
		shadowlinkmodel.SchemaRegistrySyncOptionsShadowSchemaRegistryAPIAuthOptionsBasicAttrTypes(), "password", diags)
	obj, d := types.ObjectValueFrom(ctx, shadowlinkmodel.SchemaRegistrySyncOptionsShadowSchemaRegistryAPIAuthOptionsAttrTypes(), &stateAuth)
	diags.Append(d...)
	if !d.HasError() {
		state.AuthOptions = obj
	}
}

func preservePemKey(ctx context.Context, state, prev *shadowlinkmodel.SchemaRegistrySyncOptionsShadowSchemaRegistryAPIModel, diags *diag.Diagnostics) {
	if prev.TLSSettings.IsNull() || prev.TLSSettings.IsUnknown() || state.TLSSettings.IsNull() || state.TLSSettings.IsUnknown() {
		return
	}
	var prevTLS, stateTLS shadowlinkmodel.SchemaRegistrySyncOptionsShadowSchemaRegistryAPITLSSettingsModel
	diags.Append(prev.TLSSettings.As(ctx, &prevTLS, objOpts)...)
	diags.Append(state.TLSSettings.As(ctx, &stateTLS, objOpts)...)
	if diags.HasError() || prevTLS.TLSPemSettings.IsNull() || stateTLS.TLSPemSettings.IsNull() {
		return
	}
	stateTLS.TLSPemSettings = preserveMaskedAttr(ctx, stateTLS.TLSPemSettings, prevTLS.TLSPemSettings,
		shadowlinkmodel.SchemaRegistrySyncOptionsShadowSchemaRegistryAPITLSSettingsTLSPemSettingsAttrTypes(), "key", diags)
	obj, d := types.ObjectValueFrom(ctx, shadowlinkmodel.SchemaRegistrySyncOptionsShadowSchemaRegistryAPITLSSettingsAttrTypes(), &stateTLS)
	diags.Append(d...)
	if !d.HasError() {
		state.TLSSettings = obj
	}
}

var (
	_ resource.Resource                = &ShadowLink{}
	_ resource.ResourceWithConfigure   = &ShadowLink{}
	_ resource.ResourceWithImportState = &ShadowLink{}
)

// ShadowLink represents the ShadowLink Terraform resource.
type ShadowLink struct {
	base.ResourceBase
}

// NewShadowLink constructs a ShadowLink resource.
func NewShadowLink() *ShadowLink {
	s := &ShadowLink{}
	s.ResourceBase = base.NewResourceBase("redpanda_shadow_link", ResourceShadowLinkSchema, nil)
	return s
}

// Create creates a ShadowLink resource.
func (s *ShadowLink) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan shadowlinkmodel.ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := plan.Timeouts.Create(ctx, 30*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq, expandDiags := shadowlinkmodel.ExpandCreate(ctx, &plan)
	resp.Diagnostics.Append(expandDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Same hook the schemagen-emitted proto-validator calls — single
	// source of truth for payload mutations (source_redpanda_id is
	// `extra: true` because it only exists on the Create payload, not
	// the read ShadowLink; the hook lifts it onto the payload).
	resp.Diagnostics.Append(shadowlinkmodel.ThreadCreateExtras(ctx, &plan, createReq)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createResp, err := s.CpCl.ShadowLink.CreateShadowLink(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("failed to create shadow link", utils.DeserializeGrpcError(err))
		return
	}

	op := createResp.GetOperation()
	tflog.Info(ctx, "creating shadow link", map[string]any{"id": op.GetResourceId()})
	if err := utils.AreWeDoneYet(ctx, op, createTimeout, s.CpCl.Operation); err != nil {
		resp.Diagnostics.Append(resp.State.Set(ctx, shadowlinkmodel.GenerateMinimalResourceModel(types.StringValue(op.GetResourceId()), plan.Timeouts))...)
		resp.Diagnostics.AddError("failed waiting for shadow link creation", utils.DeserializeGrpcError(err))
		return
	}

	sl, err := s.CpCl.ShadowLinkForID(ctx, op.GetResourceId())
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read shadow link %s", op.GetResourceId()), utils.DeserializeGrpcError(err))
		return
	}
	state, flatDiags := shadowlinkmodel.Flatten(ctx, sl, &plan)
	resp.Diagnostics.Append(flatDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Timeouts = plan.Timeouts
	state.AllowDeletion = plan.AllowDeletion
	state.SourceRedpandaID = plan.SourceRedpandaID
	resp.Diagnostics.Append(preserveSensitiveFromPrev(ctx, state, &plan)...)

	tflog.Info(ctx, "shadow link created", map[string]any{"id": sl.GetId()})
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Read reads the ShadowLink resource.
func (s *ShadowLink) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var model shadowlinkmodel.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sl, err := s.CpCl.ShadowLinkForID(ctx, model.ID.ValueString())
	if err != nil {
		if utils.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read shadow link %s", model.ID.ValueString()), utils.DeserializeGrpcError(err))
		return
	}
	if sl.GetState() == controlplanev1.ShadowLink_STATE_DELETING {
		resp.Diagnostics.Append(resp.State.Set(ctx, shadowlinkmodel.GenerateMinimalResourceModel(types.StringValue(sl.GetId()), model.Timeouts))...)
		resp.Diagnostics.AddWarning(fmt.Sprintf("shadow link %s is in state %s", sl.GetId(), sl.GetState()), "")
		return
	}

	tflog.Debug(ctx, "read shadow link", map[string]any{"id": sl.GetId(), "state": sl.GetState().String()})
	state, flatDiags := shadowlinkmodel.Flatten(ctx, sl, &model)
	resp.Diagnostics.Append(flatDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Timeouts = model.Timeouts
	state.AllowDeletion = model.AllowDeletion
	state.SourceRedpandaID = model.SourceRedpandaID
	resp.Diagnostics.Append(preserveSensitiveFromPrev(ctx, state, &model)...)

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Update updates the ShadowLink resource.
func (s *ShadowLink) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state shadowlinkmodel.ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Info(ctx, "updating shadow link", map[string]any{"id": plan.ID.ValueString()})

	diffedPayload, mask, expandDiags := shadowlinkmodel.ExpandUpdateWithMask(ctx, &plan, &state)
	resp.Diagnostics.Append(expandDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	diffedPayload.Id = plan.ID.ValueString()

	if len(mask.Paths) > 0 {
		updateTimeout, diags := plan.Timeouts.Update(ctx, 30*time.Minute)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		updateResp, err := s.CpCl.ShadowLink.UpdateShadowLink(ctx, &controlplanev1.UpdateShadowLinkRequest{
			ShadowLink: diffedPayload,
			UpdateMask: mask,
		})
		if err != nil {
			resp.Diagnostics.AddError("failed to update shadow link", utils.DeserializeGrpcError(err))
			return
		}
		if err := utils.AreWeDoneYet(ctx, updateResp.GetOperation(), updateTimeout, s.CpCl.Operation); err != nil {
			resp.Diagnostics.AddError("failed waiting for shadow link update", utils.DeserializeGrpcError(err))
			return
		}
	}

	sl, err := s.CpCl.ShadowLinkForID(ctx, plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to read shadow link %s", plan.ID.ValueString()), utils.DeserializeGrpcError(err))
		return
	}
	newState, flatDiags := shadowlinkmodel.Flatten(ctx, sl, &plan)
	resp.Diagnostics.Append(flatDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	newState.Timeouts = plan.Timeouts
	newState.AllowDeletion = plan.AllowDeletion
	newState.SourceRedpandaID = plan.SourceRedpandaID
	resp.Diagnostics.Append(preserveSensitiveFromPrev(ctx, newState, &plan)...)

	tflog.Info(ctx, "shadow link updated", map[string]any{"id": plan.ID.ValueString()})
	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

// Delete deletes the ShadowLink resource.
func (s *ShadowLink) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var model shadowlinkmodel.ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !model.AllowDeletion.IsNull() && !model.AllowDeletion.ValueBool() {
		resp.Diagnostics.AddError("shadow link deletion not allowed", "allow_deletion is set to false")
		return
	}
	tflog.Info(ctx, "deleting shadow link", map[string]any{"id": model.ID.ValueString()})

	deleteTimeout, diags := model.Timeouts.Delete(ctx, 30*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	delResp, err := s.CpCl.ShadowLink.DeleteShadowLink(ctx, &controlplanev1.DeleteShadowLinkRequest{
		Id: model.ID.ValueString(),
	})
	if err != nil {
		if utils.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("failed to delete shadow link", utils.DeserializeGrpcError(err))
		return
	}
	if err := utils.AreWeDoneYet(ctx, delResp.GetOperation(), deleteTimeout, s.CpCl.Operation); err != nil {
		resp.Diagnostics.AddError("failed waiting for shadow link deletion", utils.DeserializeGrpcError(err))
		return
	}
	tflog.Info(ctx, "shadow link deleted", map[string]any{"id": model.ID.ValueString()})
}

// ImportState parses a composite import ID of the form "<id>|<source_redpanda_id>".
// The "|" separator avoids collision with xid characters ([a-v0-9]).
// source_redpanda_id is a Create-only field absent from the read-shape proto;
// the composite ID lets it survive a round-trip import.
// Usage: terraform import redpanda_shadow_link.example <id>|<source_redpanda_id>
func (*ShadowLink) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "|", 2)
	id := parts[0]
	sourceRedpandaID := ""
	if len(parts) == 2 {
		sourceRedpandaID = parts[1]
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(id))...)
	if sourceRedpandaID != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("source_redpanda_id"), types.StringValue(sourceRedpandaID))...)
	}
	resp.Diagnostics.Append(utils.ImportStateBoolFromSchemaDefault(ctx, ResourceShadowLinkSchema(ctx), &resp.State, "allow_deletion")...)
}
