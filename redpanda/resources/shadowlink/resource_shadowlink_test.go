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

package shadowlink

import (
	"context"
	"testing"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	corev2 "buf.build/gen/go/redpandadata/core/protocolbuffers/go/redpanda/core/admin/v2"
	commonv1 "buf.build/gen/go/redpandadata/core/protocolbuffers/go/redpanda/core/common/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	shadowlinkmodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/shadowlink"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var basetypesObjOpts = basetypes.ObjectAsOptions{UnhandledNullAsEmpty: true, UnhandledUnknownAsEmpty: true}

// scramClientOptions wraps scram_configuration{username,password,mechanism}
// inside authentication_configuration inside client_options. Used to drive
// auth-shape assertions on ExpandCreate / Flatten.
func scramClientOptions(t *testing.T, username, password, mechanism string) types.Object {
	t.Helper()
	scram, d := types.ObjectValue(shadowlinkmodel.ClientOptionsAuthenticationConfigurationScramConfigurationAttrTypes(), map[string]attr.Value{
		"username":        types.StringValue(username),
		"password":        types.StringValue(password),
		"scram_mechanism": types.StringValue(mechanism),
		"password_set":    types.BoolNull(),
	})
	require.False(t, d.HasError(), "%v", d)

	auth, d := types.ObjectValue(shadowlinkmodel.ClientOptionsAuthenticationConfigurationAttrTypes(), map[string]attr.Value{
		"scram_configuration": scram,
		"plain_configuration": types.ObjectNull(shadowlinkmodel.ClientOptionsAuthenticationConfigurationPlainConfigurationAttrTypes()),
	})
	require.False(t, d.HasError(), "%v", d)

	co := shadowlinkmodel.ClientOptionsModel{
		AuthenticationConfiguration: auth,
		BootstrapServers:            types.ListNull(types.StringType),
		ClientID:                    types.StringNull(),
		SourceClusterID:             types.StringNull(),
		TLSSettings:                 types.ObjectNull(shadowlinkmodel.ClientOptionsTLSSettingsAttrTypes()),
	}
	obj, diags := types.ObjectValueFrom(context.Background(), shadowlinkmodel.ClientOptionsAttrTypes(), &co)
	require.False(t, diags.HasError(), "%v", diags)
	return obj
}

func TestUnit_ShadowLink_ExpandCreate_ScramAuth(t *testing.T) {
	ctx := context.Background()
	m := &shadowlinkmodel.ResourceModel{
		Name:             types.StringValue("link-1"),
		ShadowRedpandaID: types.StringValue("shadow-id"),
		ClientOptions:    scramClientOptions(t, "svc", "${secrets.MY_PW}", "SCRAM_SHA_512"),
	}

	req, diags := shadowlinkmodel.ExpandCreate(ctx, m)
	require.False(t, diags.HasError(), "%v", diags)

	sl := req.GetShadowLink()
	require.NotNil(t, sl)
	assert.Equal(t, "link-1", sl.GetName())
	assert.Equal(t, "shadow-id", sl.GetShadowRedpandaId())

	scram := sl.GetClientOptions().GetAuthenticationConfiguration().GetScramConfiguration()
	require.NotNil(t, scram)
	assert.Equal(t, "svc", scram.GetUsername())
	assert.Equal(t, "${secrets.MY_PW}", scram.GetPassword())
	assert.Equal(t, corev2.ScramMechanism_SCRAM_MECHANISM_SCRAM_SHA_512, scram.GetScramMechanism())
}

func TestUnit_ShadowLink_ExpandCreate_BootstrapServersInsteadOfSourceID(t *testing.T) {
	ctx := context.Background()
	bootstrap, d := types.ListValue(types.StringType, []attr.Value{
		types.StringValue("kafka-1:9092"),
		types.StringValue("kafka-2:9092"),
	})
	require.False(t, d.HasError())

	co := shadowlinkmodel.ClientOptionsModel{
		AuthenticationConfiguration: types.ObjectNull(shadowlinkmodel.ClientOptionsAuthenticationConfigurationAttrTypes()),
		BootstrapServers:            bootstrap,
		ClientID:                    types.StringNull(),
		SourceClusterID:             types.StringValue("cluster-uuid"),
		TLSSettings:                 types.ObjectNull(shadowlinkmodel.ClientOptionsTLSSettingsAttrTypes()),
	}
	coObj, diags := types.ObjectValueFrom(ctx, shadowlinkmodel.ClientOptionsAttrTypes(), &co)
	require.False(t, diags.HasError(), "%v", diags)

	m := &shadowlinkmodel.ResourceModel{
		Name:             types.StringValue("link-2"),
		ShadowRedpandaID: types.StringValue("shadow-id"),
		ClientOptions:    coObj,
	}
	req, expandDiags := shadowlinkmodel.ExpandCreate(ctx, m)
	require.False(t, expandDiags.HasError(), "%v", expandDiags)
	sl := req.GetShadowLink()
	assert.Equal(t, []string{"kafka-1:9092", "kafka-2:9092"}, sl.GetClientOptions().GetBootstrapServers())
	assert.Equal(t, "cluster-uuid", sl.GetClientOptions().GetSourceClusterId())
	assert.Empty(t, sl.GetSourceRedpandaId())
}

func TestUnit_ShadowLink_Flatten_PreservesSensitivePasswordFromPriorState(t *testing.T) {
	ctx := context.Background()
	prev := &shadowlinkmodel.ResourceModel{
		ClientOptions: scramClientOptions(t, "svc", "${secrets.MY_PW}", "SCRAM_SHA_256"),
	}

	apiResp := &controlplanev1.ShadowLink{
		Id:               "link-id",
		Name:             "link-1",
		ShadowRedpandaId: "shadow-id",
		State:            controlplanev1.ShadowLink_STATE_ACTIVE,
		ClientOptions: &controlplanev1.ShadowLinkClientOptions{
			AuthenticationConfiguration: &corev2.AuthenticationConfiguration{
				Authentication: &corev2.AuthenticationConfiguration_ScramConfiguration{
					ScramConfiguration: &corev2.ScramConfig{
						Username:       "svc",
						ScramMechanism: corev2.ScramMechanism_SCRAM_MECHANISM_SCRAM_SHA_256,
						PasswordSet:    true,
					},
				},
			},
		},
	}

	persist, diags := shadowlinkmodel.Flatten(ctx, apiResp, prev)
	require.False(t, diags.HasError(), "%v", diags)
	// preserveSensitiveFromPrev is the resource-side hook the Create/Read/
	// Update handlers run after Flatten — the schemagen-emitted Flatten
	// alone doesn't carry sensitive nested fields across, so the resource
	// code threads them through.
	preserveDiags := preserveSensitiveFromPrev(ctx, persist, prev)
	require.False(t, preserveDiags.HasError(), "%v", preserveDiags)

	var co shadowlinkmodel.ClientOptionsModel
	require.False(t, persist.ClientOptions.As(ctx, &co, basetypesObjOpts).HasError())
	var auth shadowlinkmodel.ClientOptionsAuthenticationConfigurationModel
	require.False(t, co.AuthenticationConfiguration.As(ctx, &auth, basetypesObjOpts).HasError())
	var scram shadowlinkmodel.ClientOptionsAuthenticationConfigurationScramConfigurationModel
	require.False(t, auth.ScramConfiguration.As(ctx, &scram, basetypesObjOpts).HasError())

	assert.Equal(t, "${secrets.MY_PW}", scram.Password.ValueString(), "plaintext password must come from prior state, not API")
	assert.Equal(t, "SCRAM_SHA_256", scram.ScramMechanism.ValueString())
	assert.Equal(t, "ACTIVE", persist.State.ValueString())
}

func TestUnit_ShadowLink_ExpandCreate_SecurityACLFilter(t *testing.T) {
	ctx := context.Background()
	rf, d := types.ObjectValue(shadowlinkmodel.SecuritySyncOptionsAclFiltersResourceFilterAttrTypes(), map[string]attr.Value{
		"resource_type": types.StringValue("TOPIC"),
		"pattern_type":  types.StringValue("LITERAL"),
		"name":          types.StringValue("orders"),
	})
	require.False(t, d.HasError())
	af, d := types.ObjectValue(shadowlinkmodel.SecuritySyncOptionsAclFiltersAccessFilterAttrTypes(), map[string]attr.Value{
		"principal":       types.StringValue("User:svc"),
		"operation":       types.StringValue("READ"),
		"permission_type": types.StringValue("ALLOW"),
		"host":            types.StringValue("*"),
	})
	require.False(t, d.HasError())
	filter, d := types.ObjectValue(shadowlinkmodel.SecuritySyncOptionsAclFiltersAttrTypes(), map[string]attr.Value{
		"resource_filter": rf,
		"access_filter":   af,
	})
	require.False(t, d.HasError())
	filters, d := types.ListValue(types.ObjectType{AttrTypes: shadowlinkmodel.SecuritySyncOptionsAclFiltersAttrTypes()}, []attr.Value{filter})
	require.False(t, d.HasError())

	sec := shadowlinkmodel.SecuritySyncOptionsModel{
		Interval:   types.StringNull(),
		Paused:     types.BoolValue(false),
		AclFilters: filters,
	}
	secObj, diags := types.ObjectValueFrom(ctx, shadowlinkmodel.SecuritySyncOptionsAttrTypes(), &sec)
	require.False(t, diags.HasError(), "%v", diags)

	m := &shadowlinkmodel.ResourceModel{
		Name:                types.StringValue("link-sec"),
		ShadowRedpandaID:    types.StringValue("shadow-id"),
		SourceRedpandaID:    types.StringValue("source-id"),
		ClientOptions:       types.ObjectNull(shadowlinkmodel.ClientOptionsAttrTypes()),
		SecuritySyncOptions: secObj,
	}
	req, diags := shadowlinkmodel.ExpandCreate(ctx, m)
	require.False(t, diags.HasError(), "%v", diags)

	got := req.GetShadowLink().GetSecuritySyncOptions()
	require.NotNil(t, got)
	require.Len(t, got.GetAclFilters(), 1)
	f := got.GetAclFilters()[0]
	assert.Equal(t, commonv1.ACLResource_ACL_RESOURCE_TOPIC, f.GetResourceFilter().GetResourceType())
	assert.Equal(t, commonv1.ACLPattern_ACL_PATTERN_LITERAL, f.GetResourceFilter().GetPatternType())
	assert.Equal(t, "orders", f.GetResourceFilter().GetName())
	assert.Equal(t, "User:svc", f.GetAccessFilter().GetPrincipal())
	assert.Equal(t, commonv1.ACLOperation_ACL_OPERATION_READ, f.GetAccessFilter().GetOperation())
	assert.Equal(t, commonv1.ACLPermissionType_ACL_PERMISSION_TYPE_ALLOW, f.GetAccessFilter().GetPermissionType())
}

func TestUnit_ShadowLink_ExpandCreate_SchemaRegistryShadowingOneofPresence(t *testing.T) {
	ctx := context.Background()
	sr := shadowlinkmodel.SchemaRegistrySyncOptionsModel{
		ShadowSchemaRegistryTopic: types.BoolValue(true),
	}
	srObj, diags := types.ObjectValueFrom(ctx, shadowlinkmodel.SchemaRegistrySyncOptionsAttrTypes(), &sr)
	require.False(t, diags.HasError(), "%v", diags)

	m := &shadowlinkmodel.ResourceModel{
		Name:                      types.StringValue("link-sr"),
		ShadowRedpandaID:          types.StringValue("shadow-id"),
		SourceRedpandaID:          types.StringValue("source-id"),
		ClientOptions:             types.ObjectNull(shadowlinkmodel.ClientOptionsAttrTypes()),
		SchemaRegistrySyncOptions: srObj,
	}
	req, expandDiags := shadowlinkmodel.ExpandCreate(ctx, m)
	require.False(t, expandDiags.HasError(), "%v", expandDiags)
	assert.NotNil(t, req.GetShadowLink().GetSchemaRegistrySyncOptions().GetShadowSchemaRegistryTopic(),
		"expected shadow_schema_registry_topic oneof to be set")
}

// allNullModel returns a baseline where every block-typed attribute is typed-null.
func allNullModel() *shadowlinkmodel.ResourceModel {
	return &shadowlinkmodel.ResourceModel{
		ID:                        types.StringValue("link-id"),
		ClientOptions:             types.ObjectNull(shadowlinkmodel.ClientOptionsAttrTypes()),
		TopicMetadataSyncOptions:  types.ObjectNull(shadowlinkmodel.TopicMetadataSyncOptionsAttrTypes()),
		ConsumerOffsetSyncOptions: types.ObjectNull(shadowlinkmodel.ConsumerOffsetSyncOptionsAttrTypes()),
		SecuritySyncOptions:       types.ObjectNull(shadowlinkmodel.SecuritySyncOptionsAttrTypes()),
		SchemaRegistrySyncOptions: types.ObjectNull(shadowlinkmodel.SchemaRegistrySyncOptionsAttrTypes()),
	}
}

// clientOptionsWithMetadataAge returns a populated client_options Object with
// the given metadata_max_age_ms — used to drive field-mask diff scenarios.
func clientOptionsWithMetadataAge(t *testing.T, ageMs int32) types.Object {
	t.Helper()
	co := shadowlinkmodel.ClientOptionsModel{
		AuthenticationConfiguration: types.ObjectNull(shadowlinkmodel.ClientOptionsAuthenticationConfigurationAttrTypes()),
		BootstrapServers:            types.ListNull(types.StringType),
		ClientID:                    types.StringNull(),
		SourceClusterID:             types.StringNull(),
		TLSSettings:                 types.ObjectNull(shadowlinkmodel.ClientOptionsTLSSettingsAttrTypes()),
		MetadataMaxAgeMs:            types.Int32Value(ageMs),
	}
	obj, diags := types.ObjectValueFrom(context.Background(), shadowlinkmodel.ClientOptionsAttrTypes(), &co)
	require.False(t, diags.HasError(), "%v", diags)
	return obj
}

func diffMask(t *testing.T, plan, state *shadowlinkmodel.ResourceModel) (diffed *controlplanev1.ShadowLinkUpdate, paths []string) {
	t.Helper()
	ctx := context.Background()
	planPayload, d := shadowlinkmodel.ExpandUpdate(ctx, plan)
	require.False(t, d.HasError(), "%v", d)
	statePayload, d := shadowlinkmodel.ExpandUpdate(ctx, state)
	require.False(t, d.HasError(), "%v", d)
	diffed, mask := utils.GenerateProtobufDiffAndUpdateMask(planPayload, statePayload)
	return diffed, mask.GetPaths()
}

func TestUnit_ShadowLink_UpdateDiff_NoChange(t *testing.T) {
	_, paths := diffMask(t, allNullModel(), allNullModel())
	assert.Empty(t, paths, "no diff should yield no paths")
}

func TestUnit_ShadowLink_UpdateDiff_OnlyClientOptionsChanged(t *testing.T) {
	plan := allNullModel()
	plan.ClientOptions = clientOptionsWithMetadataAge(t, 15000)
	diffed, paths := diffMask(t, plan, allNullModel())
	assert.Equal(t, []string{"client_options"}, paths)
	require.NotNil(t, diffed.GetClientOptions())
	assert.Equal(t, int32(15000), diffed.GetClientOptions().GetMetadataMaxAgeMs())
}

func TestUnit_ShadowLink_UpdateDiff_SubAttrChangeShipsParent(t *testing.T) {
	plan := allNullModel()
	plan.ClientOptions = clientOptionsWithMetadataAge(t, 20000)
	state := allNullModel()
	state.ClientOptions = clientOptionsWithMetadataAge(t, 10000)
	diffed, paths := diffMask(t, plan, state)
	assert.Equal(t, []string{"client_options"}, paths)
	assert.Equal(t, int32(20000), diffed.GetClientOptions().GetMetadataMaxAgeMs())
}
