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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
)

var basetypesObjOpts = basetypes.ObjectAsOptions{UnhandledNullAsEmpty: true, UnhandledUnknownAsEmpty: true}

func nullClientOptions(t *testing.T) types.Object {
	t.Helper()
	co := shadowlinkmodel.ClientOptionsModel{
		BootstrapServers:                types.ListNull(types.StringType),
		ClientID:                        types.StringNull(),
		SourceClusterID:                 types.StringNull(),
		TLS:                             types.ObjectNull(shadowlinkmodel.TLSModel{}.AttributeTypes()),
		Authentication:                  types.ObjectNull(shadowlinkmodel.AuthModel{}.AttributeTypes()),
		MetadataMaxAgeMs:                types.Int32Null(),
		EffectiveMetadataMaxAgeMs:       types.Int32Null(),
		ConnectionTimeoutMs:             types.Int32Null(),
		EffectiveConnectionTimeoutMs:    types.Int32Null(),
		RetryBackoffMs:                  types.Int32Null(),
		EffectiveRetryBackoffMs:         types.Int32Null(),
		FetchWaitMaxMs:                  types.Int32Null(),
		EffectiveFetchWaitMaxMs:         types.Int32Null(),
		FetchMinBytes:                   types.Int32Null(),
		EffectiveFetchMinBytes:          types.Int32Null(),
		FetchMaxBytes:                   types.Int32Null(),
		EffectiveFetchMaxBytes:          types.Int32Null(),
		FetchPartitionMaxBytes:          types.Int32Null(),
		EffectiveFetchPartitionMaxBytes: types.Int32Null(),
	}
	obj, diags := types.ObjectValueFrom(context.Background(), shadowlinkmodel.ClientOptionsModel{}.AttributeTypes(), &co)
	require.False(t, diags.HasError(), "%v", diags)
	return obj
}

func clientOptionsWith(t *testing.T, mutate func(*shadowlinkmodel.ClientOptionsModel)) types.Object {
	t.Helper()
	co := shadowlinkmodel.ClientOptionsModel{
		BootstrapServers:                types.ListNull(types.StringType),
		ClientID:                        types.StringNull(),
		SourceClusterID:                 types.StringNull(),
		TLS:                             types.ObjectNull(shadowlinkmodel.TLSModel{}.AttributeTypes()),
		Authentication:                  types.ObjectNull(shadowlinkmodel.AuthModel{}.AttributeTypes()),
		MetadataMaxAgeMs:                types.Int32Null(),
		EffectiveMetadataMaxAgeMs:       types.Int32Null(),
		ConnectionTimeoutMs:             types.Int32Null(),
		EffectiveConnectionTimeoutMs:    types.Int32Null(),
		RetryBackoffMs:                  types.Int32Null(),
		EffectiveRetryBackoffMs:         types.Int32Null(),
		FetchWaitMaxMs:                  types.Int32Null(),
		EffectiveFetchWaitMaxMs:         types.Int32Null(),
		FetchMinBytes:                   types.Int32Null(),
		EffectiveFetchMinBytes:          types.Int32Null(),
		FetchMaxBytes:                   types.Int32Null(),
		EffectiveFetchMaxBytes:          types.Int32Null(),
		FetchPartitionMaxBytes:          types.Int32Null(),
		EffectiveFetchPartitionMaxBytes: types.Int32Null(),
	}
	mutate(&co)
	obj, diags := types.ObjectValueFrom(context.Background(), shadowlinkmodel.ClientOptionsModel{}.AttributeTypes(), &co)
	require.False(t, diags.HasError(), "%v", diags)
	return obj
}

func TestModel_BuildCreateRequest_ScramAuth(t *testing.T) {
	ctx := context.Background()

	authObj, d := types.ObjectValue(shadowlinkmodel.AuthModel{}.AttributeTypes(), map[string]attr.Value{
		"mechanism": types.StringValue("scram-sha-512"),
		"username":  types.StringValue("svc"),
		"password":  types.StringValue("${secrets.MY_PW}"),
	})
	require.False(t, d.HasError())
	co := clientOptionsWith(t, func(c *shadowlinkmodel.ClientOptionsModel) { c.Authentication = authObj })

	m := shadowlinkmodel.ResourceModel{
		Name:             types.StringValue("link-1"),
		ShadowRedpandaID: types.StringValue("shadow-id"),
		SourceRedpandaID: types.StringValue("source-id"),
		ClientOptions:    co,
	}

	req, diags := m.BuildCreateRequest(ctx)
	require.False(t, diags.HasError(), "%v", diags)

	assert.Equal(t, "link-1", req.GetName())
	assert.Equal(t, "shadow-id", req.GetShadowRedpandaId())
	assert.Equal(t, "source-id", req.GetSourceRedpandaId())
	require.NotNil(t, req.GetClientOptions())
	scram := req.GetClientOptions().GetAuthenticationConfiguration().GetScramConfiguration()
	require.NotNil(t, scram)
	assert.Equal(t, "svc", scram.GetUsername())
	assert.Equal(t, "${secrets.MY_PW}", scram.GetPassword())
	assert.Equal(t, corev2.ScramMechanism_SCRAM_MECHANISM_SCRAM_SHA_512, scram.GetScramMechanism())
}

func TestModel_BuildCreateRequest_BootstrapServersInsteadOfSource(t *testing.T) {
	ctx := context.Background()

	bootstrap, d := types.ListValue(types.StringType, []attr.Value{types.StringValue("kafka-1:9092"), types.StringValue("kafka-2:9092")})
	require.False(t, d.HasError())
	co := clientOptionsWith(t, func(c *shadowlinkmodel.ClientOptionsModel) {
		c.BootstrapServers = bootstrap
		c.SourceClusterID = types.StringValue("cluster-uuid")
	})

	m := shadowlinkmodel.ResourceModel{
		Name:             types.StringValue("link-2"),
		ShadowRedpandaID: types.StringValue("shadow-id"),
		ClientOptions:    co,
	}
	req, diags := m.BuildCreateRequest(ctx)
	require.False(t, diags.HasError())

	assert.Equal(t, []string{"kafka-1:9092", "kafka-2:9092"}, req.GetClientOptions().GetBootstrapServers())
	assert.Equal(t, "cluster-uuid", req.GetClientOptions().GetSourceClusterId())
	assert.Empty(t, req.GetSourceRedpandaId())
}

func TestModel_BuildCreateRequest_KafkaTuningFields(t *testing.T) {
	ctx := context.Background()

	co := clientOptionsWith(t, func(c *shadowlinkmodel.ClientOptionsModel) {
		c.MetadataMaxAgeMs = types.Int32Value(20000)
		c.ConnectionTimeoutMs = types.Int32Value(2000)
		c.RetryBackoffMs = types.Int32Value(200)
		c.FetchWaitMaxMs = types.Int32Value(1000)
		c.FetchMinBytes = types.Int32Value(1024)
		c.FetchMaxBytes = types.Int32Value(33554432)
		c.FetchPartitionMaxBytes = types.Int32Value(2097152)
	})

	m := shadowlinkmodel.ResourceModel{
		Name:             types.StringValue("link-tuning"),
		ShadowRedpandaID: types.StringValue("shadow-id"),
		SourceRedpandaID: types.StringValue("source-id"),
		ClientOptions:    co,
	}
	req, diags := m.BuildCreateRequest(ctx)
	require.False(t, diags.HasError())
	c := req.GetClientOptions()
	assert.Equal(t, int32(20000), c.GetMetadataMaxAgeMs())
	assert.Equal(t, int32(2000), c.GetConnectionTimeoutMs())
	assert.Equal(t, int32(200), c.GetRetryBackoffMs())
	assert.Equal(t, int32(1000), c.GetFetchWaitMaxMs())
	assert.Equal(t, int32(1024), c.GetFetchMinBytes())
	assert.Equal(t, int32(33554432), c.GetFetchMaxBytes())
	assert.Equal(t, int32(2097152), c.GetFetchPartitionMaxBytes())
}

func TestModel_BuildCreateRequest_TopicMetadataSync(t *testing.T) {
	ctx := context.Background()

	filter, d := types.ObjectValue(shadowlinkmodel.NameFilterModel{}.AttributeTypes(), map[string]attr.Value{
		"pattern_type": types.StringValue("PATTERN_TYPE_PREFIX"),
		"filter_type":  types.StringValue("FILTER_TYPE_INCLUDE"),
		"name":         types.StringValue("orders-"),
	})
	require.False(t, d.HasError())
	filtersList, d := types.ListValue(types.ObjectType{AttrTypes: shadowlinkmodel.NameFilterModel{}.AttributeTypes()}, []attr.Value{filter})
	require.False(t, d.HasError())
	props, d := types.ListValue(types.StringType, []attr.Value{types.StringValue("compression.type")})
	require.False(t, d.HasError())
	startOffset, d := types.ObjectValue(shadowlinkmodel.StartOffsetModel{}.AttributeTypes(), map[string]attr.Value{
		"at_earliest":  types.BoolValue(true),
		"at_latest":    types.BoolNull(),
		"at_timestamp": types.StringNull(),
	})
	require.False(t, d.HasError())
	tm := shadowlinkmodel.TopicMetadataSyncOptionsModel{
		Interval:                     types.StringValue("45s"),
		EffectiveInterval:            types.StringNull(),
		AutoCreateShadowTopicFilters: filtersList,
		SyncedShadowTopicProperties:  props,
		ExcludeDefault:               types.BoolValue(true),
		StartOffset:                  startOffset,
		Paused:                       types.BoolValue(false),
	}
	tmObj, diags := types.ObjectValueFrom(ctx, shadowlinkmodel.TopicMetadataSyncOptionsModel{}.AttributeTypes(), &tm)
	require.False(t, diags.HasError(), "%v", diags)

	m := shadowlinkmodel.ResourceModel{
		Name:                     types.StringValue("link-tm"),
		ShadowRedpandaID:         types.StringValue("shadow-id"),
		SourceRedpandaID:         types.StringValue("source-id"),
		ClientOptions:            nullClientOptions(t),
		TopicMetadataSyncOptions: tmObj,
	}
	req, diags := m.BuildCreateRequest(ctx)
	require.False(t, diags.HasError(), "%v", diags)

	got := req.GetTopicMetadataSyncOptions()
	require.NotNil(t, got)
	assert.Equal(t, durationpb.New(45*1_000_000_000).AsDuration(), got.GetInterval().AsDuration())
	require.Len(t, got.GetAutoCreateShadowTopicFilters(), 1)
	f := got.GetAutoCreateShadowTopicFilters()[0]
	assert.Equal(t, corev2.PatternType_PATTERN_TYPE_PREFIX, f.GetPatternType())
	assert.Equal(t, corev2.FilterType_FILTER_TYPE_INCLUDE, f.GetFilterType())
	assert.Equal(t, "orders-", f.GetName())
	assert.Equal(t, []string{"compression.type"}, got.GetSyncedShadowTopicProperties())
	assert.True(t, got.GetExcludeDefault())
	require.NotNil(t, got.GetStartAtEarliest())
}

func TestModel_BuildCreateRequest_SecuritySync_ACLFilter(t *testing.T) {
	ctx := context.Background()

	rf, d := types.ObjectValue(shadowlinkmodel.ACLResourceFilterModel{}.AttributeTypes(), map[string]attr.Value{
		"resource_type": types.StringValue("ACL_RESOURCE_TOPIC"),
		"pattern_type":  types.StringValue("ACL_PATTERN_LITERAL"),
		"name":          types.StringValue("orders"),
	})
	require.False(t, d.HasError())
	af, d := types.ObjectValue(shadowlinkmodel.ACLAccessFilterModel{}.AttributeTypes(), map[string]attr.Value{
		"principal":       types.StringValue("User:svc"),
		"operation":       types.StringValue("ACL_OPERATION_READ"),
		"permission_type": types.StringValue("ACL_PERMISSION_TYPE_ALLOW"),
		"host":            types.StringValue("*"),
	})
	require.False(t, d.HasError())
	filter, d := types.ObjectValue(shadowlinkmodel.ACLFilterModel{}.AttributeTypes(), map[string]attr.Value{
		"resource_filter": rf,
		"access_filter":   af,
	})
	require.False(t, d.HasError())
	filters, d := types.ListValue(types.ObjectType{AttrTypes: shadowlinkmodel.ACLFilterModel{}.AttributeTypes()}, []attr.Value{filter})
	require.False(t, d.HasError())
	sec := shadowlinkmodel.SecuritySyncOptionsModel{
		Interval:          types.StringNull(),
		EffectiveInterval: types.StringNull(),
		Paused:            types.BoolValue(false),
		ACLFilters:        filters,
	}
	secObj, diags := types.ObjectValueFrom(ctx, shadowlinkmodel.SecuritySyncOptionsModel{}.AttributeTypes(), &sec)
	require.False(t, diags.HasError(), "%v", diags)

	m := shadowlinkmodel.ResourceModel{
		Name:                types.StringValue("link-sec"),
		ShadowRedpandaID:    types.StringValue("shadow-id"),
		SourceRedpandaID:    types.StringValue("source-id"),
		ClientOptions:       nullClientOptions(t),
		SecuritySyncOptions: secObj,
	}
	req, diags := m.BuildCreateRequest(ctx)
	require.False(t, diags.HasError(), "%v", diags)

	got := req.GetSecuritySyncOptions()
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

func TestModel_BuildCreateRequest_SchemaRegistryShadowing(t *testing.T) {
	ctx := context.Background()
	srModel := shadowlinkmodel.SchemaRegistrySyncOptionsModel{
		ShadowSchemaRegistryTopic: types.BoolValue(true),
	}
	srObj, diags := types.ObjectValueFrom(ctx, shadowlinkmodel.SchemaRegistrySyncOptionsModel{}.AttributeTypes(), &srModel)
	require.False(t, diags.HasError())

	m := shadowlinkmodel.ResourceModel{
		Name:                      types.StringValue("link-sr"),
		ShadowRedpandaID:          types.StringValue("shadow-id"),
		SourceRedpandaID:          types.StringValue("source-id"),
		ClientOptions:             nullClientOptions(t),
		SchemaRegistrySyncOptions: srObj,
	}
	req, diags := m.BuildCreateRequest(ctx)
	require.False(t, diags.HasError())
	assert.NotNil(t, req.GetSchemaRegistrySyncOptions().GetShadowSchemaRegistryTopic(), "expected shadow_schema_registry_topic oneof to be set")
}

func TestModel_GetUpdatedModel_PreservesPlaintextPasswordFromPriorState(t *testing.T) {
	ctx := context.Background()

	priorAuth, d := types.ObjectValue(shadowlinkmodel.AuthModel{}.AttributeTypes(), map[string]attr.Value{
		"mechanism": types.StringValue("scram-sha-256"),
		"username":  types.StringValue("svc"),
		"password":  types.StringValue("${secrets.MY_PW}"),
	})
	require.False(t, d.HasError())
	priorCO := clientOptionsWith(t, func(c *shadowlinkmodel.ClientOptionsModel) { c.Authentication = priorAuth })

	m := &shadowlinkmodel.ResourceModel{ClientOptions: priorCO}
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
					},
				},
			},
		},
	}

	persist, diags := m.GetUpdatedModel(ctx, apiResp)
	require.False(t, diags.HasError(), "%v", diags)

	var auth shadowlinkmodel.AuthModel
	var co shadowlinkmodel.ClientOptionsModel
	require.False(t, persist.ClientOptions.As(ctx, &co, basetypesObjOpts).HasError())
	require.False(t, co.Authentication.As(ctx, &auth, basetypesObjOpts).HasError())
	assert.Equal(t, "${secrets.MY_PW}", auth.Password.ValueString(), "plaintext password must come from prior state, not API")
	assert.Equal(t, "scram-sha-256", auth.Mechanism.ValueString())
	assert.Equal(t, "STATE_ACTIVE", persist.State.ValueString())
}

// --- BuildUpdateRequest field-mask diff tests ---

// allNullModel returns a ResourceModel where every block-typed attribute is typed-null.
// Use as a baseline; mutate fields to drive specific BuildUpdateRequest scenarios.
func allNullModel(t *testing.T) shadowlinkmodel.ResourceModel {
	t.Helper()
	return shadowlinkmodel.ResourceModel{
		ID:                        types.StringValue("link-id"),
		ClientOptions:             types.ObjectNull(shadowlinkmodel.ClientOptionsModel{}.AttributeTypes()),
		TopicMetadataSyncOptions:  types.ObjectNull(shadowlinkmodel.TopicMetadataSyncOptionsModel{}.AttributeTypes()),
		ConsumerOffsetSyncOptions: types.ObjectNull(shadowlinkmodel.ConsumerOffsetSyncOptionsModel{}.AttributeTypes()),
		SecuritySyncOptions:       types.ObjectNull(shadowlinkmodel.SecuritySyncOptionsModel{}.AttributeTypes()),
		SchemaRegistrySyncOptions: types.ObjectNull(shadowlinkmodel.SchemaRegistrySyncOptionsModel{}.AttributeTypes()),
	}
}

func tmsPaused(t *testing.T, paused bool) types.Object {
	t.Helper()
	tm := shadowlinkmodel.TopicMetadataSyncOptionsModel{
		Interval:                     types.StringNull(),
		EffectiveInterval:            types.StringNull(),
		AutoCreateShadowTopicFilters: types.ListNull(types.ObjectType{AttrTypes: shadowlinkmodel.NameFilterModel{}.AttributeTypes()}),
		SyncedShadowTopicProperties:  types.ListNull(types.StringType),
		ExcludeDefault:               types.BoolNull(),
		StartOffset:                  types.ObjectNull(shadowlinkmodel.StartOffsetModel{}.AttributeTypes()),
		Paused:                       types.BoolValue(paused),
	}
	obj, diags := types.ObjectValueFrom(context.Background(), shadowlinkmodel.TopicMetadataSyncOptionsModel{}.AttributeTypes(), &tm)
	require.False(t, diags.HasError(), "%v", diags)
	return obj
}

func TestModel_BuildUpdateRequest_NoChange(t *testing.T) {
	ctx := context.Background()
	prior := allNullModel(t)
	plan := allNullModel(t)

	req, paths, diags := plan.BuildUpdateRequest(ctx, &prior)
	require.False(t, diags.HasError(), "%v", diags)
	assert.Empty(t, paths, "no diff should yield no paths")
	assert.Equal(t, "link-id", req.GetId())
}

func TestModel_BuildUpdateRequest_OnlyClientOptionsChanged(t *testing.T) {
	ctx := context.Background()
	prior := allNullModel(t)
	plan := allNullModel(t)
	plan.ClientOptions = clientOptionsWith(t, func(c *shadowlinkmodel.ClientOptionsModel) {
		c.MetadataMaxAgeMs = types.Int32Value(15000)
	})

	req, paths, diags := plan.BuildUpdateRequest(ctx, &prior)
	require.False(t, diags.HasError(), "%v", diags)
	assert.Equal(t, []string{"client_options"}, paths)
	require.NotNil(t, req.GetClientOptions())
	assert.Equal(t, int32(15000), req.GetClientOptions().GetMetadataMaxAgeMs())
	assert.Nil(t, req.GetTopicMetadataSyncOptions(), "untouched block must not be in request")
	assert.Nil(t, req.GetConsumerOffsetSyncOptions())
	assert.Nil(t, req.GetSecuritySyncOptions())
	assert.Nil(t, req.GetSchemaRegistrySyncOptions())
}

func TestModel_BuildUpdateRequest_OnlyTopicMetadataChanged(t *testing.T) {
	ctx := context.Background()
	prior := allNullModel(t)
	prior.TopicMetadataSyncOptions = tmsPaused(t, false)
	plan := allNullModel(t)
	plan.TopicMetadataSyncOptions = tmsPaused(t, true)

	req, paths, diags := plan.BuildUpdateRequest(ctx, &prior)
	require.False(t, diags.HasError(), "%v", diags)
	assert.Equal(t, []string{"topic_metadata_sync_options"}, paths)
	require.NotNil(t, req.GetTopicMetadataSyncOptions())
	assert.True(t, req.GetTopicMetadataSyncOptions().GetPaused())
	assert.Nil(t, req.GetClientOptions(), "untouched block must not be in request")
}

func TestModel_BuildUpdateRequest_MultipleBlocksChanged(t *testing.T) {
	ctx := context.Background()
	prior := allNullModel(t)
	plan := allNullModel(t)
	plan.ClientOptions = clientOptionsWith(t, func(c *shadowlinkmodel.ClientOptionsModel) {
		c.MetadataMaxAgeMs = types.Int32Value(20000)
	})
	plan.TopicMetadataSyncOptions = tmsPaused(t, true)

	_, paths, diags := plan.BuildUpdateRequest(ctx, &prior)
	require.False(t, diags.HasError(), "%v", diags)
	assert.ElementsMatch(t, []string{"client_options", "topic_metadata_sync_options"}, paths)
}

// Plan-Unknown for a block must NOT cause the path to be sent — proto field-mask semantics
// would otherwise zero out server-managed defaults whenever the framework re-marks an
// Optional+Computed block as Unknown.
func TestModel_BuildUpdateRequest_PlanUnknownIsSkipped(t *testing.T) {
	ctx := context.Background()
	prior := allNullModel(t)
	prior.ClientOptions = clientOptionsWith(t, func(c *shadowlinkmodel.ClientOptionsModel) {
		c.MetadataMaxAgeMs = types.Int32Value(10000)
	})
	plan := allNullModel(t)
	plan.ClientOptions = types.ObjectUnknown(shadowlinkmodel.ClientOptionsModel{}.AttributeTypes())

	req, paths, diags := plan.BuildUpdateRequest(ctx, &prior)
	require.False(t, diags.HasError(), "%v", diags)
	assert.Empty(t, paths, "Unknown plan value must be treated as 'no change'")
	assert.Nil(t, req.GetClientOptions())
}

// Plan-Null when prior was non-null IS a real change: user wants to clear the block.
func TestModel_BuildUpdateRequest_PlanNullClearsPriorBlock(t *testing.T) {
	ctx := context.Background()
	prior := allNullModel(t)
	prior.ClientOptions = clientOptionsWith(t, func(c *shadowlinkmodel.ClientOptionsModel) {
		c.MetadataMaxAgeMs = types.Int32Value(10000)
	})
	plan := allNullModel(t) // plan.ClientOptions stays null

	req, paths, diags := plan.BuildUpdateRequest(ctx, &prior)
	require.False(t, diags.HasError(), "%v", diags)
	assert.Equal(t, []string{"client_options"}, paths)
	assert.Nil(t, req.GetClientOptions(), "null plan + path means 'clear it'")
}

// Sub-attribute change inside an otherwise-equal block flips the parent Object's Equal
// to false and ships the whole block — proto field-mask only operates at top-level paths.
func TestModel_BuildUpdateRequest_SubAttrChangeShipsParent(t *testing.T) {
	ctx := context.Background()
	prior := allNullModel(t)
	prior.ClientOptions = clientOptionsWith(t, func(c *shadowlinkmodel.ClientOptionsModel) {
		c.MetadataMaxAgeMs = types.Int32Value(10000)
		c.RetryBackoffMs = types.Int32Value(100)
	})
	plan := allNullModel(t)
	plan.ClientOptions = clientOptionsWith(t, func(c *shadowlinkmodel.ClientOptionsModel) {
		c.MetadataMaxAgeMs = types.Int32Value(20000) // changed
		c.RetryBackoffMs = types.Int32Value(100)     // unchanged
	})

	req, paths, diags := plan.BuildUpdateRequest(ctx, &prior)
	require.False(t, diags.HasError(), "%v", diags)
	assert.Equal(t, []string{"client_options"}, paths)
	require.NotNil(t, req.GetClientOptions())
	assert.Equal(t, int32(20000), req.GetClientOptions().GetMetadataMaxAgeMs())
	assert.Equal(t, int32(100), req.GetClientOptions().GetRetryBackoffMs())
}
