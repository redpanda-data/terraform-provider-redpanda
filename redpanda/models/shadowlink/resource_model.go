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
	"fmt"
	"time"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	corev2 "buf.build/gen/go/redpandadata/core/protocolbuffers/go/redpanda/core/admin/v2"
	commonv1 "buf.build/gen/go/redpandadata/core/protocolbuffers/go/redpanda/core/common/v1"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// --- nested model types ---

// TLSModel mirrors the tls block under client_options.
type TLSModel struct {
	Enabled             types.Bool   `tfsdk:"enabled"`
	CA                  types.String `tfsdk:"ca"`
	Cert                types.String `tfsdk:"cert"`
	Key                 types.String `tfsdk:"key"`
	DoNotSetSNIHostname types.Bool   `tfsdk:"do_not_set_sni_hostname"`
}

// AttributeTypes returns the schema attribute types for TLSModel.
func (TLSModel) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"enabled":                 types.BoolType,
		"ca":                      types.StringType,
		"cert":                    types.StringType,
		"key":                     types.StringType,
		"do_not_set_sni_hostname": types.BoolType,
	}
}

// AuthModel mirrors the authentication block under client_options.
type AuthModel struct {
	Mechanism types.String `tfsdk:"mechanism"`
	Username  types.String `tfsdk:"username"`
	Password  types.String `tfsdk:"password"`
}

// AttributeTypes returns the schema attribute types for AuthModel.
func (AuthModel) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"mechanism": types.StringType,
		"username":  types.StringType,
		"password":  types.StringType,
	}
}

// ClientOptionsModel mirrors the client_options nested attribute.
type ClientOptionsModel struct {
	BootstrapServers                types.List   `tfsdk:"bootstrap_servers"`
	ClientID                        types.String `tfsdk:"client_id"`
	SourceClusterID                 types.String `tfsdk:"source_cluster_id"`
	TLS                             types.Object `tfsdk:"tls"`
	Authentication                  types.Object `tfsdk:"authentication"`
	MetadataMaxAgeMs                types.Int32  `tfsdk:"metadata_max_age_ms"`
	EffectiveMetadataMaxAgeMs       types.Int32  `tfsdk:"effective_metadata_max_age_ms"`
	ConnectionTimeoutMs             types.Int32  `tfsdk:"connection_timeout_ms"`
	EffectiveConnectionTimeoutMs    types.Int32  `tfsdk:"effective_connection_timeout_ms"`
	RetryBackoffMs                  types.Int32  `tfsdk:"retry_backoff_ms"`
	EffectiveRetryBackoffMs         types.Int32  `tfsdk:"effective_retry_backoff_ms"`
	FetchWaitMaxMs                  types.Int32  `tfsdk:"fetch_wait_max_ms"`
	EffectiveFetchWaitMaxMs         types.Int32  `tfsdk:"effective_fetch_wait_max_ms"`
	FetchMinBytes                   types.Int32  `tfsdk:"fetch_min_bytes"`
	EffectiveFetchMinBytes          types.Int32  `tfsdk:"effective_fetch_min_bytes"`
	FetchMaxBytes                   types.Int32  `tfsdk:"fetch_max_bytes"`
	EffectiveFetchMaxBytes          types.Int32  `tfsdk:"effective_fetch_max_bytes"`
	FetchPartitionMaxBytes          types.Int32  `tfsdk:"fetch_partition_max_bytes"`
	EffectiveFetchPartitionMaxBytes types.Int32  `tfsdk:"effective_fetch_partition_max_bytes"`
}

// AttributeTypes returns the schema attribute types for ClientOptionsModel.
func (ClientOptionsModel) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"bootstrap_servers":                   types.ListType{ElemType: types.StringType},
		"client_id":                           types.StringType,
		"source_cluster_id":                   types.StringType,
		"tls":                                 types.ObjectType{AttrTypes: TLSModel{}.AttributeTypes()},
		"authentication":                      types.ObjectType{AttrTypes: AuthModel{}.AttributeTypes()},
		"metadata_max_age_ms":                 types.Int32Type,
		"effective_metadata_max_age_ms":       types.Int32Type,
		"connection_timeout_ms":               types.Int32Type,
		"effective_connection_timeout_ms":     types.Int32Type,
		"retry_backoff_ms":                    types.Int32Type,
		"effective_retry_backoff_ms":          types.Int32Type,
		"fetch_wait_max_ms":                   types.Int32Type,
		"effective_fetch_wait_max_ms":         types.Int32Type,
		"fetch_min_bytes":                     types.Int32Type,
		"effective_fetch_min_bytes":           types.Int32Type,
		"fetch_max_bytes":                     types.Int32Type,
		"effective_fetch_max_bytes":           types.Int32Type,
		"fetch_partition_max_bytes":           types.Int32Type,
		"effective_fetch_partition_max_bytes": types.Int32Type,
	}
}

// NameFilterModel mirrors a redpanda.core.admin.v2.NameFilter.
type NameFilterModel struct {
	PatternType types.String `tfsdk:"pattern_type"`
	FilterType  types.String `tfsdk:"filter_type"`
	Name        types.String `tfsdk:"name"`
}

// AttributeTypes returns the schema attribute types for NameFilterModel.
func (NameFilterModel) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"pattern_type": types.StringType,
		"filter_type":  types.StringType,
		"name":         types.StringType,
	}
}

// StartOffsetModel mirrors the start_offset oneof under topic_metadata_sync_options.
type StartOffsetModel struct {
	AtEarliest  types.Bool   `tfsdk:"at_earliest"`
	AtLatest    types.Bool   `tfsdk:"at_latest"`
	AtTimestamp types.String `tfsdk:"at_timestamp"`
}

// AttributeTypes returns the schema attribute types for StartOffsetModel.
func (StartOffsetModel) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"at_earliest":  types.BoolType,
		"at_latest":    types.BoolType,
		"at_timestamp": types.StringType,
	}
}

// TopicMetadataSyncOptionsModel mirrors topic_metadata_sync_options.
type TopicMetadataSyncOptionsModel struct {
	Interval                     types.String `tfsdk:"interval"`
	EffectiveInterval            types.String `tfsdk:"effective_interval"`
	AutoCreateShadowTopicFilters types.List   `tfsdk:"auto_create_shadow_topic_filters"`
	SyncedShadowTopicProperties  types.List   `tfsdk:"synced_shadow_topic_properties"`
	ExcludeDefault               types.Bool   `tfsdk:"exclude_default"`
	StartOffset                  types.Object `tfsdk:"start_offset"`
	Paused                       types.Bool   `tfsdk:"paused"`
}

// AttributeTypes returns the schema attribute types for TopicMetadataSyncOptionsModel.
func (TopicMetadataSyncOptionsModel) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"interval":                         types.StringType,
		"effective_interval":               types.StringType,
		"auto_create_shadow_topic_filters": types.ListType{ElemType: types.ObjectType{AttrTypes: NameFilterModel{}.AttributeTypes()}},
		"synced_shadow_topic_properties":   types.ListType{ElemType: types.StringType},
		"exclude_default":                  types.BoolType,
		"start_offset":                     types.ObjectType{AttrTypes: StartOffsetModel{}.AttributeTypes()},
		"paused":                           types.BoolType,
	}
}

// ConsumerOffsetSyncOptionsModel mirrors consumer_offset_sync_options.
type ConsumerOffsetSyncOptionsModel struct {
	Interval          types.String `tfsdk:"interval"`
	EffectiveInterval types.String `tfsdk:"effective_interval"`
	Paused            types.Bool   `tfsdk:"paused"`
	GroupFilters      types.List   `tfsdk:"group_filters"`
}

// AttributeTypes returns the schema attribute types for ConsumerOffsetSyncOptionsModel.
func (ConsumerOffsetSyncOptionsModel) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"interval":           types.StringType,
		"effective_interval": types.StringType,
		"paused":             types.BoolType,
		"group_filters":      types.ListType{ElemType: types.ObjectType{AttrTypes: NameFilterModel{}.AttributeTypes()}},
	}
}

// ACLResourceFilterModel mirrors core.admin.v2.ACLResourceFilter.
type ACLResourceFilterModel struct {
	ResourceType types.String `tfsdk:"resource_type"`
	PatternType  types.String `tfsdk:"pattern_type"`
	Name         types.String `tfsdk:"name"`
}

// AttributeTypes returns the schema attribute types for ACLResourceFilterModel.
func (ACLResourceFilterModel) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"resource_type": types.StringType,
		"pattern_type":  types.StringType,
		"name":          types.StringType,
	}
}

// ACLAccessFilterModel mirrors core.admin.v2.ACLAccessFilter.
type ACLAccessFilterModel struct {
	Principal      types.String `tfsdk:"principal"`
	Operation      types.String `tfsdk:"operation"`
	PermissionType types.String `tfsdk:"permission_type"`
	Host           types.String `tfsdk:"host"`
}

// AttributeTypes returns the schema attribute types for ACLAccessFilterModel.
func (ACLAccessFilterModel) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"principal":       types.StringType,
		"operation":       types.StringType,
		"permission_type": types.StringType,
		"host":            types.StringType,
	}
}

// ACLFilterModel mirrors core.admin.v2.ACLFilter.
type ACLFilterModel struct {
	ResourceFilter types.Object `tfsdk:"resource_filter"`
	AccessFilter   types.Object `tfsdk:"access_filter"`
}

// AttributeTypes returns the schema attribute types for ACLFilterModel.
func (ACLFilterModel) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"resource_filter": types.ObjectType{AttrTypes: ACLResourceFilterModel{}.AttributeTypes()},
		"access_filter":   types.ObjectType{AttrTypes: ACLAccessFilterModel{}.AttributeTypes()},
	}
}

// SecuritySyncOptionsModel mirrors security_sync_options.
type SecuritySyncOptionsModel struct {
	Interval          types.String `tfsdk:"interval"`
	EffectiveInterval types.String `tfsdk:"effective_interval"`
	Paused            types.Bool   `tfsdk:"paused"`
	ACLFilters        types.List   `tfsdk:"acl_filters"`
}

// AttributeTypes returns the schema attribute types for SecuritySyncOptionsModel.
func (SecuritySyncOptionsModel) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"interval":           types.StringType,
		"effective_interval": types.StringType,
		"paused":             types.BoolType,
		"acl_filters":        types.ListType{ElemType: types.ObjectType{AttrTypes: ACLFilterModel{}.AttributeTypes()}},
	}
}

// SchemaRegistrySyncOptionsModel mirrors schema_registry_sync_options.
type SchemaRegistrySyncOptionsModel struct {
	ShadowSchemaRegistryTopic types.Bool `tfsdk:"shadow_schema_registry_topic"`
}

// AttributeTypes returns the schema attribute types for SchemaRegistrySyncOptionsModel.
func (SchemaRegistrySyncOptionsModel) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"shadow_schema_registry_topic": types.BoolType,
	}
}

// --- state constructors ---

// GenerateMinimalResourceModel creates a minimal ResourceModel with only enough state for Terraform
// to track an existing shadow link and to delete it, if necessary. Used during Create when the
// operation fails after the API has accepted the resource, and during Read when the resource is
// found in a transient state that should force destroy-recreate.
func GenerateMinimalResourceModel(id string, timeout timeouts.Value) *ResourceModel {
	return &ResourceModel{
		ID:                        types.StringValue(id),
		AllowDeletion:             types.BoolValue(true),
		Name:                      types.StringNull(),
		ShadowRedpandaID:          types.StringNull(),
		SourceRedpandaID:          types.StringNull(),
		State:                     types.StringNull(),
		Reason:                    types.StringNull(),
		ClientOptions:             types.ObjectNull(ClientOptionsModel{}.AttributeTypes()),
		TopicMetadataSyncOptions:  types.ObjectNull(TopicMetadataSyncOptionsModel{}.AttributeTypes()),
		ConsumerOffsetSyncOptions: types.ObjectNull(ConsumerOffsetSyncOptionsModel{}.AttributeTypes()),
		SecuritySyncOptions:       types.ObjectNull(SecuritySyncOptionsModel{}.AttributeTypes()),
		SchemaRegistrySyncOptions: types.ObjectNull(SchemaRegistrySyncOptionsModel{}.AttributeTypes()),
		Timeouts:                  timeout,
	}
}

// --- request builders ---

var objAsOpts = basetypes.ObjectAsOptions{UnhandledNullAsEmpty: true, UnhandledUnknownAsEmpty: true}

// BuildCreateRequest converts the TF model into a ShadowLinkCreate proto.
func (r *ResourceModel) BuildCreateRequest(ctx context.Context) (*controlplanev1.ShadowLinkCreate, diag.Diagnostics) {
	var diags diag.Diagnostics
	out := &controlplanev1.ShadowLinkCreate{
		Name:             r.Name.ValueString(),
		ShadowRedpandaId: r.ShadowRedpandaID.ValueString(),
	}
	if !r.SourceRedpandaID.IsNull() && !r.SourceRedpandaID.IsUnknown() {
		out.SourceRedpandaId = r.SourceRedpandaID.ValueString()
	}

	co, d := r.buildClientOptions(ctx)
	diags.Append(d...)
	out.ClientOptions = co

	tm, d := buildTopicMetadataSyncOptions(ctx, r.TopicMetadataSyncOptions)
	diags.Append(d...)
	out.TopicMetadataSyncOptions = tm

	cs, d := buildConsumerOffsetSyncOptions(ctx, r.ConsumerOffsetSyncOptions)
	diags.Append(d...)
	out.ConsumerOffsetSyncOptions = cs

	ss, d := buildSecuritySyncOptions(ctx, r.SecuritySyncOptions)
	diags.Append(d...)
	out.SecuritySyncOptions = ss

	sr, d := buildSchemaRegistrySyncOptions(ctx, r.SchemaRegistrySyncOptions)
	diags.Append(d...)
	out.SchemaRegistrySyncOptions = sr
	return out, diags
}

// BuildUpdateRequest builds the ShadowLinkUpdate proto and the field-mask paths covering only the
// top-level blocks that changed between prior state and the plan. A block whose plan value is
// Unknown is treated as "no change" so the request never zeroes out server-managed defaults; a
// block whose plan value is Null but state was non-null is included so the user can clear it.
func (r *ResourceModel) BuildUpdateRequest(ctx context.Context, prior *ResourceModel) (*controlplanev1.ShadowLinkUpdate, []string, diag.Diagnostics) {
	var diags diag.Diagnostics
	out := &controlplanev1.ShadowLinkUpdate{Id: r.ID.ValueString()}
	var paths []string

	if !r.ClientOptions.IsUnknown() && !r.ClientOptions.Equal(prior.ClientOptions) {
		co, d := r.buildClientOptions(ctx)
		diags.Append(d...)
		out.ClientOptions = co
		paths = append(paths, "client_options")
	}
	if !r.TopicMetadataSyncOptions.IsUnknown() && !r.TopicMetadataSyncOptions.Equal(prior.TopicMetadataSyncOptions) {
		tm, d := buildTopicMetadataSyncOptions(ctx, r.TopicMetadataSyncOptions)
		diags.Append(d...)
		out.TopicMetadataSyncOptions = tm
		paths = append(paths, "topic_metadata_sync_options")
	}
	if !r.ConsumerOffsetSyncOptions.IsUnknown() && !r.ConsumerOffsetSyncOptions.Equal(prior.ConsumerOffsetSyncOptions) {
		cs, d := buildConsumerOffsetSyncOptions(ctx, r.ConsumerOffsetSyncOptions)
		diags.Append(d...)
		out.ConsumerOffsetSyncOptions = cs
		paths = append(paths, "consumer_offset_sync_options")
	}
	if !r.SecuritySyncOptions.IsUnknown() && !r.SecuritySyncOptions.Equal(prior.SecuritySyncOptions) {
		ss, d := buildSecuritySyncOptions(ctx, r.SecuritySyncOptions)
		diags.Append(d...)
		out.SecuritySyncOptions = ss
		paths = append(paths, "security_sync_options")
	}
	if !r.SchemaRegistrySyncOptions.IsUnknown() && !r.SchemaRegistrySyncOptions.Equal(prior.SchemaRegistrySyncOptions) {
		sr, d := buildSchemaRegistrySyncOptions(ctx, r.SchemaRegistrySyncOptions)
		diags.Append(d...)
		out.SchemaRegistrySyncOptions = sr
		paths = append(paths, "schema_registry_sync_options")
	}
	return out, paths, diags
}

func (r *ResourceModel) buildClientOptions(ctx context.Context) (*controlplanev1.ShadowLinkClientOptions, diag.Diagnostics) {
	var diags diag.Diagnostics
	if r.ClientOptions.IsNull() || r.ClientOptions.IsUnknown() {
		return nil, diags
	}
	var co ClientOptionsModel
	diags.Append(r.ClientOptions.As(ctx, &co, objAsOpts)...)
	if diags.HasError() {
		return nil, diags
	}

	out := &controlplanev1.ShadowLinkClientOptions{}
	if !co.BootstrapServers.IsNull() && !co.BootstrapServers.IsUnknown() {
		var servers []string
		diags.Append(co.BootstrapServers.ElementsAs(ctx, &servers, false)...)
		out.BootstrapServers = servers
	}
	if !co.SourceClusterID.IsNull() && !co.SourceClusterID.IsUnknown() {
		out.SourceClusterId = co.SourceClusterID.ValueString()
	}
	if !co.TLS.IsNull() && !co.TLS.IsUnknown() {
		var tls TLSModel
		diags.Append(co.TLS.As(ctx, &tls, objAsOpts)...)
		out.TlsSettings = &controlplanev1.TLSSettings{
			Enabled:             tls.Enabled.ValueBool(),
			Ca:                  tls.CA.ValueString(),
			Cert:                tls.Cert.ValueString(),
			Key:                 tls.Key.ValueString(),
			DoNotSetSniHostname: tls.DoNotSetSNIHostname.ValueBool(),
		}
	}
	if !co.Authentication.IsNull() && !co.Authentication.IsUnknown() {
		var auth AuthModel
		diags.Append(co.Authentication.As(ctx, &auth, objAsOpts)...)
		ac, d := buildAuthentication(auth)
		diags.Append(d...)
		out.AuthenticationConfiguration = ac
	}
	out.MetadataMaxAgeMs = co.MetadataMaxAgeMs.ValueInt32()
	out.ConnectionTimeoutMs = co.ConnectionTimeoutMs.ValueInt32()
	out.RetryBackoffMs = co.RetryBackoffMs.ValueInt32()
	out.FetchWaitMaxMs = co.FetchWaitMaxMs.ValueInt32()
	out.FetchMinBytes = co.FetchMinBytes.ValueInt32()
	out.FetchMaxBytes = co.FetchMaxBytes.ValueInt32()
	out.FetchPartitionMaxBytes = co.FetchPartitionMaxBytes.ValueInt32()
	return out, diags
}

func buildAuthentication(a AuthModel) (*corev2.AuthenticationConfiguration, diag.Diagnostics) {
	var diags diag.Diagnostics
	switch a.Mechanism.ValueString() {
	case "scram-sha-256":
		return &corev2.AuthenticationConfiguration{
			Authentication: &corev2.AuthenticationConfiguration_ScramConfiguration{
				ScramConfiguration: &corev2.ScramConfig{
					Username:       a.Username.ValueString(),
					Password:       a.Password.ValueString(),
					ScramMechanism: corev2.ScramMechanism_SCRAM_MECHANISM_SCRAM_SHA_256,
				},
			},
		}, diags
	case "scram-sha-512":
		return &corev2.AuthenticationConfiguration{
			Authentication: &corev2.AuthenticationConfiguration_ScramConfiguration{
				ScramConfiguration: &corev2.ScramConfig{
					Username:       a.Username.ValueString(),
					Password:       a.Password.ValueString(),
					ScramMechanism: corev2.ScramMechanism_SCRAM_MECHANISM_SCRAM_SHA_512,
				},
			},
		}, diags
	case "plain":
		return &corev2.AuthenticationConfiguration{
			Authentication: &corev2.AuthenticationConfiguration_PlainConfiguration{
				PlainConfiguration: &corev2.PlainConfig{
					Username: a.Username.ValueString(),
					Password: a.Password.ValueString(),
				},
			},
		}, diags
	default:
		diags.AddError("invalid authentication mechanism", fmt.Sprintf("unsupported mechanism %q", a.Mechanism.ValueString()))
		return nil, diags
	}
}

func buildTopicMetadataSyncOptions(ctx context.Context, obj types.Object) (*corev2.TopicMetadataSyncOptions, diag.Diagnostics) {
	var diags diag.Diagnostics
	if obj.IsNull() || obj.IsUnknown() {
		return nil, diags
	}
	var m TopicMetadataSyncOptionsModel
	diags.Append(obj.As(ctx, &m, objAsOpts)...)
	if diags.HasError() {
		return nil, diags
	}
	out := &corev2.TopicMetadataSyncOptions{
		ExcludeDefault: m.ExcludeDefault.ValueBool(),
		Paused:         m.Paused.ValueBool(),
	}
	if d := durationFromString(m.Interval); d != nil {
		out.Interval = d
	}
	if !m.AutoCreateShadowTopicFilters.IsNull() && !m.AutoCreateShadowTopicFilters.IsUnknown() {
		filters, d := nameFiltersFromList(ctx, m.AutoCreateShadowTopicFilters)
		diags.Append(d...)
		out.AutoCreateShadowTopicFilters = filters
	}
	if !m.SyncedShadowTopicProperties.IsNull() && !m.SyncedShadowTopicProperties.IsUnknown() {
		var props []string
		diags.Append(m.SyncedShadowTopicProperties.ElementsAs(ctx, &props, false)...)
		out.SyncedShadowTopicProperties = props
	}
	if !m.StartOffset.IsNull() && !m.StartOffset.IsUnknown() {
		var s StartOffsetModel
		diags.Append(m.StartOffset.As(ctx, &s, objAsOpts)...)
		switch {
		case s.AtEarliest.ValueBool():
			out.StartOffset = &corev2.TopicMetadataSyncOptions_StartAtEarliest{StartAtEarliest: &corev2.TopicMetadataSyncOptions_EarliestOffset{}}
		case s.AtLatest.ValueBool():
			out.StartOffset = &corev2.TopicMetadataSyncOptions_StartAtLatest{StartAtLatest: &corev2.TopicMetadataSyncOptions_LatestOffset{}}
		case !s.AtTimestamp.IsNull() && !s.AtTimestamp.IsUnknown() && s.AtTimestamp.ValueString() != "":
			t, err := time.Parse(time.RFC3339, s.AtTimestamp.ValueString())
			if err != nil {
				diags.AddError("invalid start_offset.at_timestamp", err.Error())
			} else {
				out.StartOffset = &corev2.TopicMetadataSyncOptions_StartAtTimestamp{StartAtTimestamp: timestamppb.New(t)}
			}
		default:
			// no start-offset variant set
		}
	}
	return out, diags
}

func buildConsumerOffsetSyncOptions(ctx context.Context, obj types.Object) (*corev2.ConsumerOffsetSyncOptions, diag.Diagnostics) {
	var diags diag.Diagnostics
	if obj.IsNull() || obj.IsUnknown() {
		return nil, diags
	}
	var m ConsumerOffsetSyncOptionsModel
	diags.Append(obj.As(ctx, &m, objAsOpts)...)
	if diags.HasError() {
		return nil, diags
	}
	out := &corev2.ConsumerOffsetSyncOptions{
		Paused: m.Paused.ValueBool(),
	}
	if d := durationFromString(m.Interval); d != nil {
		out.Interval = d
	}
	if !m.GroupFilters.IsNull() && !m.GroupFilters.IsUnknown() {
		filters, d := nameFiltersFromList(ctx, m.GroupFilters)
		diags.Append(d...)
		out.GroupFilters = filters
	}
	return out, diags
}

func buildSecuritySyncOptions(ctx context.Context, obj types.Object) (*corev2.SecuritySettingsSyncOptions, diag.Diagnostics) {
	var diags diag.Diagnostics
	if obj.IsNull() || obj.IsUnknown() {
		return nil, diags
	}
	var m SecuritySyncOptionsModel
	diags.Append(obj.As(ctx, &m, objAsOpts)...)
	if diags.HasError() {
		return nil, diags
	}
	out := &corev2.SecuritySettingsSyncOptions{
		Paused: m.Paused.ValueBool(),
	}
	if d := durationFromString(m.Interval); d != nil {
		out.Interval = d
	}
	if !m.ACLFilters.IsNull() && !m.ACLFilters.IsUnknown() {
		var entries []ACLFilterModel
		diags.Append(m.ACLFilters.ElementsAs(ctx, &entries, false)...)
		filters := make([]*corev2.ACLFilter, 0, len(entries))
		for _, e := range entries {
			f := &corev2.ACLFilter{}
			if !e.ResourceFilter.IsNull() && !e.ResourceFilter.IsUnknown() {
				var rf ACLResourceFilterModel
				diags.Append(e.ResourceFilter.As(ctx, &rf, objAsOpts)...)
				f.ResourceFilter = &corev2.ACLResourceFilter{
					ResourceType: aclResourceFromString(rf.ResourceType.ValueString()),
					PatternType:  aclPatternFromString(rf.PatternType.ValueString()),
					Name:         rf.Name.ValueString(),
				}
			}
			if !e.AccessFilter.IsNull() && !e.AccessFilter.IsUnknown() {
				var af ACLAccessFilterModel
				diags.Append(e.AccessFilter.As(ctx, &af, objAsOpts)...)
				f.AccessFilter = &corev2.ACLAccessFilter{
					Principal:      af.Principal.ValueString(),
					Operation:      aclOperationFromString(af.Operation.ValueString()),
					PermissionType: aclPermissionFromString(af.PermissionType.ValueString()),
					Host:           af.Host.ValueString(),
				}
			}
			filters = append(filters, f)
		}
		out.AclFilters = filters
	}
	return out, diags
}

func buildSchemaRegistrySyncOptions(ctx context.Context, obj types.Object) (*corev2.SchemaRegistrySyncOptions, diag.Diagnostics) {
	var diags diag.Diagnostics
	if obj.IsNull() || obj.IsUnknown() {
		return nil, diags
	}
	var m SchemaRegistrySyncOptionsModel
	diags.Append(obj.As(ctx, &m, objAsOpts)...)
	if diags.HasError() {
		return nil, diags
	}
	out := &corev2.SchemaRegistrySyncOptions{}
	if m.ShadowSchemaRegistryTopic.ValueBool() {
		out.SchemaRegistryShadowingMode = &corev2.SchemaRegistrySyncOptions_ShadowSchemaRegistryTopic_{
			ShadowSchemaRegistryTopic: &corev2.SchemaRegistrySyncOptions_ShadowSchemaRegistryTopic{},
		}
	}
	return out, diags
}

func nameFiltersFromList(ctx context.Context, l types.List) ([]*corev2.NameFilter, diag.Diagnostics) {
	var diags diag.Diagnostics
	var entries []NameFilterModel
	diags.Append(l.ElementsAs(ctx, &entries, false)...)
	out := make([]*corev2.NameFilter, 0, len(entries))
	for _, e := range entries {
		out = append(out, &corev2.NameFilter{
			PatternType: patternTypeFromString(e.PatternType.ValueString()),
			FilterType:  filterTypeFromString(e.FilterType.ValueString()),
			Name:        e.Name.ValueString(),
		})
	}
	return out, diags
}

// --- enum helpers ---

func patternTypeFromString(s string) corev2.PatternType {
	if v, ok := corev2.PatternType_value[s]; ok {
		return corev2.PatternType(v)
	}
	return corev2.PatternType_PATTERN_TYPE_UNSPECIFIED
}

func filterTypeFromString(s string) corev2.FilterType {
	if v, ok := corev2.FilterType_value[s]; ok {
		return corev2.FilterType(v)
	}
	return corev2.FilterType_FILTER_TYPE_UNSPECIFIED
}

func aclResourceFromString(s string) commonv1.ACLResource {
	if v, ok := commonv1.ACLResource_value[s]; ok {
		return commonv1.ACLResource(v)
	}
	return commonv1.ACLResource_ACL_RESOURCE_UNSPECIFIED
}

func aclPatternFromString(s string) commonv1.ACLPattern {
	if v, ok := commonv1.ACLPattern_value[s]; ok {
		return commonv1.ACLPattern(v)
	}
	return commonv1.ACLPattern_ACL_PATTERN_UNSPECIFIED
}

func aclOperationFromString(s string) commonv1.ACLOperation {
	if v, ok := commonv1.ACLOperation_value[s]; ok {
		return commonv1.ACLOperation(v)
	}
	return commonv1.ACLOperation_ACL_OPERATION_UNSPECIFIED
}

func aclPermissionFromString(s string) commonv1.ACLPermissionType {
	if v, ok := commonv1.ACLPermissionType_value[s]; ok {
		return commonv1.ACLPermissionType(v)
	}
	return commonv1.ACLPermissionType_ACL_PERMISSION_TYPE_UNSPECIFIED
}

// durationFromString parses a Go duration string into a *durationpb.Duration. Returns nil for empty/invalid.
func durationFromString(s types.String) *durationpb.Duration {
	if s.IsNull() || s.IsUnknown() || s.ValueString() == "" {
		return nil
	}
	d, err := time.ParseDuration(s.ValueString())
	if err != nil {
		return nil
	}
	return durationpb.New(d)
}

// durationToString formats a Duration into a Go duration string, or null on nil.
func durationToString(d *durationpb.Duration) types.String {
	if d == nil {
		return types.StringNull()
	}
	return types.StringValue(d.AsDuration().String())
}

// preserveDuration keeps the user's string form ("1m") instead of Go's canonical "1m0s" when both parse equal.
func preserveDuration(d *durationpb.Duration, prior types.String) types.String {
	if d == nil {
		return types.StringNull()
	}
	apiStr := d.AsDuration().String()
	if prior.IsNull() || prior.IsUnknown() || prior.ValueString() == "" {
		return types.StringValue(apiStr)
	}
	priorD, err := time.ParseDuration(prior.ValueString())
	if err != nil {
		return types.StringValue(apiStr)
	}
	if priorD == d.AsDuration() {
		return prior
	}
	return types.StringValue(apiStr)
}

// --- Read helpers (proto → state) ---

// GetUpdatedModel populates the model from the API ShadowLink response, preserving sensitive prior-state inputs.
func (r *ResourceModel) GetUpdatedModel(ctx context.Context, sl *controlplanev1.ShadowLink) (*ResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	r.ID = types.StringValue(sl.GetId())
	r.Name = types.StringValue(sl.GetName())
	r.ShadowRedpandaID = types.StringValue(sl.GetShadowRedpandaId())
	r.State = types.StringValue(sl.GetState().String())
	r.Reason = types.StringValue(sl.GetReason())

	co, d := buildClientOptionsObject(ctx, sl.GetClientOptions(), r.ClientOptions)
	diags.Append(d...)
	r.ClientOptions = co

	tm, d := topicMetadataSyncOptionsObject(ctx, sl.GetTopicMetadataSyncOptions(), r.TopicMetadataSyncOptions)
	diags.Append(d...)
	r.TopicMetadataSyncOptions = tm

	cs, d := consumerOffsetSyncOptionsObject(ctx, sl.GetConsumerOffsetSyncOptions(), r.ConsumerOffsetSyncOptions)
	diags.Append(d...)
	r.ConsumerOffsetSyncOptions = cs

	ss, d := securitySyncOptionsObject(ctx, sl.GetSecuritySyncOptions(), r.SecuritySyncOptions)
	diags.Append(d...)
	r.SecuritySyncOptions = ss

	sr, d := schemaRegistrySyncOptionsObject(ctx, sl.GetSchemaRegistrySyncOptions())
	diags.Append(d...)
	r.SchemaRegistrySyncOptions = sr
	return r, diags
}

// buildClientOptionsObject builds the client_options Object, preserving plaintext secrets from prior state.
func buildClientOptionsObject(ctx context.Context, co *controlplanev1.ShadowLinkClientOptions, prior types.Object) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	attrTypes := ClientOptionsModel{}.AttributeTypes()
	if co == nil {
		return types.ObjectNull(attrTypes), diags
	}

	bootstrapList := utils.StringSliceToTypeListOrNull(co.GetBootstrapServers())

	// Pull prior values for sensitive fields so plan-after-apply is clean.
	var priorCO ClientOptionsModel
	hasPrior := !prior.IsNull() && !prior.IsUnknown()
	if hasPrior {
		diags.Append(prior.As(ctx, &priorCO, objAsOpts)...)
	}

	// TLS
	tlsObj := types.ObjectNull(TLSModel{}.AttributeTypes())
	if t := co.GetTlsSettings(); t != nil {
		var priorTLS TLSModel
		hasPriorTLS := hasPrior && !priorCO.TLS.IsNull() && !priorCO.TLS.IsUnknown()
		if hasPriorTLS {
			diags.Append(priorCO.TLS.As(ctx, &priorTLS, objAsOpts)...)
		}
		obj, d := types.ObjectValue(TLSModel{}.AttributeTypes(), map[string]attr.Value{
			"enabled":                 types.BoolValue(t.GetEnabled()),
			"ca":                      preserveString(t.GetCa(), hasPriorTLS, priorTLS.CA),
			"cert":                    preserveString(t.GetCert(), hasPriorTLS, priorTLS.Cert),
			"key":                     preserveSensitive(hasPriorTLS, priorTLS.Key),
			"do_not_set_sni_hostname": types.BoolValue(t.GetDoNotSetSniHostname()),
		})
		diags.Append(d...)
		tlsObj = obj
	}

	// Authentication
	authObj := types.ObjectNull(AuthModel{}.AttributeTypes())
	if a := co.GetAuthenticationConfiguration(); a != nil {
		var mechanism, username string
		switch {
		case a.GetScramConfiguration() != nil:
			scram := a.GetScramConfiguration()
			username = scram.GetUsername()
			switch scram.GetScramMechanism() {
			case corev2.ScramMechanism_SCRAM_MECHANISM_SCRAM_SHA_256:
				mechanism = "scram-sha-256"
			case corev2.ScramMechanism_SCRAM_MECHANISM_SCRAM_SHA_512:
				mechanism = "scram-sha-512"
			default:
				// SCRAM_MECHANISM_UNSPECIFIED or unknown
			}
		case a.GetPlainConfiguration() != nil:
			plain := a.GetPlainConfiguration()
			username = plain.GetUsername()
			mechanism = "plain"
		default:
			// no auth variant set
		}
		var priorAuth AuthModel
		hasPriorAuth := hasPrior && !priorCO.Authentication.IsNull() && !priorCO.Authentication.IsUnknown()
		if hasPriorAuth {
			diags.Append(priorCO.Authentication.As(ctx, &priorAuth, objAsOpts)...)
		}
		obj, d := types.ObjectValue(AuthModel{}.AttributeTypes(), map[string]attr.Value{
			"mechanism": preserveString(mechanism, hasPriorAuth, priorAuth.Mechanism),
			"username":  preserveString(username, hasPriorAuth, priorAuth.Username),
			"password":  preserveSensitive(hasPriorAuth, priorAuth.Password),
		})
		diags.Append(d...)
		authObj = obj
	}

	out, d := types.ObjectValue(attrTypes, map[string]attr.Value{
		"bootstrap_servers":                   bootstrapList,
		"client_id":                           preserveString(co.GetClientId(), hasPrior, priorCO.ClientID),
		"source_cluster_id":                   preserveString(co.GetSourceClusterId(), hasPrior, priorCO.SourceClusterID),
		"tls":                                 tlsObj,
		"authentication":                      authObj,
		"metadata_max_age_ms":                 types.Int32Value(co.GetMetadataMaxAgeMs()),
		"effective_metadata_max_age_ms":       types.Int32Value(co.GetEffectiveMetadataMaxAgeMs()),
		"connection_timeout_ms":               types.Int32Value(co.GetConnectionTimeoutMs()),
		"effective_connection_timeout_ms":     types.Int32Value(co.GetEffectiveConnectionTimeoutMs()),
		"retry_backoff_ms":                    types.Int32Value(co.GetRetryBackoffMs()),
		"effective_retry_backoff_ms":          types.Int32Value(co.GetEffectiveRetryBackoffMs()),
		"fetch_wait_max_ms":                   types.Int32Value(co.GetFetchWaitMaxMs()),
		"effective_fetch_wait_max_ms":         types.Int32Value(co.GetEffectiveFetchWaitMaxMs()),
		"fetch_min_bytes":                     types.Int32Value(co.GetFetchMinBytes()),
		"effective_fetch_min_bytes":           types.Int32Value(co.GetEffectiveFetchMinBytes()),
		"fetch_max_bytes":                     types.Int32Value(co.GetFetchMaxBytes()),
		"effective_fetch_max_bytes":           types.Int32Value(co.GetEffectiveFetchMaxBytes()),
		"fetch_partition_max_bytes":           types.Int32Value(co.GetFetchPartitionMaxBytes()),
		"effective_fetch_partition_max_bytes": types.Int32Value(co.GetEffectiveFetchPartitionMaxBytes()),
	})
	diags.Append(d...)
	return out, diags
}

func topicMetadataSyncOptionsObject(ctx context.Context, t *corev2.TopicMetadataSyncOptions, prior types.Object) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	attrTypes := TopicMetadataSyncOptionsModel{}.AttributeTypes()
	if t == nil {
		return types.ObjectNull(attrTypes), diags
	}
	var priorInterval types.String
	if !prior.IsNull() && !prior.IsUnknown() {
		var pm TopicMetadataSyncOptionsModel
		diags.Append(prior.As(ctx, &pm, objAsOpts)...)
		priorInterval = pm.Interval
	}
	filters, d := nameFiltersToList(ctx, t.GetAutoCreateShadowTopicFilters())
	diags.Append(d...)
	props := utils.StringSliceToTypeListOrNull(t.GetSyncedShadowTopicProperties())
	startOffset, d := startOffsetObject(t)
	diags.Append(d...)
	obj, d := types.ObjectValue(attrTypes, map[string]attr.Value{
		"interval":                         preserveDuration(t.GetInterval(), priorInterval),
		"effective_interval":               durationToString(t.GetEffectiveInterval()),
		"auto_create_shadow_topic_filters": filters,
		"synced_shadow_topic_properties":   props,
		"exclude_default":                  types.BoolValue(t.GetExcludeDefault()),
		"start_offset":                     startOffset,
		"paused":                           types.BoolValue(t.GetPaused()),
	})
	diags.Append(d...)
	return obj, diags
}

func startOffsetObject(t *corev2.TopicMetadataSyncOptions) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	attrTypes := StartOffsetModel{}.AttributeTypes()
	atEarliest := types.BoolNull()
	atLatest := types.BoolNull()
	atTimestamp := types.StringNull()
	switch {
	case t.GetStartAtEarliest() != nil:
		atEarliest = types.BoolValue(true)
	case t.GetStartAtLatest() != nil:
		atLatest = types.BoolValue(true)
	case t.GetStartAtTimestamp() != nil:
		atTimestamp = types.StringValue(t.GetStartAtTimestamp().AsTime().Format(time.RFC3339))
	default:
		// No oneof case set — return all-null object so plan can populate.
		return types.ObjectNull(attrTypes), diags
	}
	obj, d := types.ObjectValue(attrTypes, map[string]attr.Value{
		"at_earliest":  atEarliest,
		"at_latest":    atLatest,
		"at_timestamp": atTimestamp,
	})
	diags.Append(d...)
	return obj, diags
}

func consumerOffsetSyncOptionsObject(ctx context.Context, c *corev2.ConsumerOffsetSyncOptions, prior types.Object) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	attrTypes := ConsumerOffsetSyncOptionsModel{}.AttributeTypes()
	if c == nil {
		return types.ObjectNull(attrTypes), diags
	}
	var priorInterval types.String
	if !prior.IsNull() && !prior.IsUnknown() {
		var pm ConsumerOffsetSyncOptionsModel
		diags.Append(prior.As(ctx, &pm, objAsOpts)...)
		priorInterval = pm.Interval
	}
	filters, d := nameFiltersToList(ctx, c.GetGroupFilters())
	diags.Append(d...)
	obj, d := types.ObjectValue(attrTypes, map[string]attr.Value{
		"interval":           preserveDuration(c.GetInterval(), priorInterval),
		"effective_interval": durationToString(c.GetEffectiveInterval()),
		"paused":             types.BoolValue(c.GetPaused()),
		"group_filters":      filters,
	})
	diags.Append(d...)
	return obj, diags
}

func securitySyncOptionsObject(ctx context.Context, s *corev2.SecuritySettingsSyncOptions, prior types.Object) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	attrTypes := SecuritySyncOptionsModel{}.AttributeTypes()
	if s == nil {
		return types.ObjectNull(attrTypes), diags
	}
	var priorInterval types.String
	if !prior.IsNull() && !prior.IsUnknown() {
		var pm SecuritySyncOptionsModel
		diags.Append(prior.As(ctx, &pm, objAsOpts)...)
		priorInterval = pm.Interval
	}
	filterType := types.ObjectType{AttrTypes: ACLFilterModel{}.AttributeTypes()}
	filterVals := make([]attr.Value, 0, len(s.GetAclFilters()))
	for _, f := range s.GetAclFilters() {
		rfObj := types.ObjectNull(ACLResourceFilterModel{}.AttributeTypes())
		if rf := f.GetResourceFilter(); rf != nil {
			obj, d := types.ObjectValue(ACLResourceFilterModel{}.AttributeTypes(), map[string]attr.Value{
				"resource_type": types.StringValue(rf.GetResourceType().String()),
				"pattern_type":  types.StringValue(rf.GetPatternType().String()),
				"name":          types.StringValue(rf.GetName()),
			})
			diags.Append(d...)
			rfObj = obj
		}
		afObj := types.ObjectNull(ACLAccessFilterModel{}.AttributeTypes())
		if af := f.GetAccessFilter(); af != nil {
			obj, d := types.ObjectValue(ACLAccessFilterModel{}.AttributeTypes(), map[string]attr.Value{
				"principal":       types.StringValue(af.GetPrincipal()),
				"operation":       types.StringValue(af.GetOperation().String()),
				"permission_type": types.StringValue(af.GetPermissionType().String()),
				"host":            types.StringValue(af.GetHost()),
			})
			diags.Append(d...)
			afObj = obj
		}
		fObj, d := types.ObjectValue(ACLFilterModel{}.AttributeTypes(), map[string]attr.Value{
			"resource_filter": rfObj,
			"access_filter":   afObj,
		})
		diags.Append(d...)
		filterVals = append(filterVals, fObj)
	}
	filtersList, d := types.ListValue(filterType, filterVals)
	diags.Append(d...)
	obj, d := types.ObjectValue(attrTypes, map[string]attr.Value{
		"interval":           preserveDuration(s.GetInterval(), priorInterval),
		"effective_interval": durationToString(s.GetEffectiveInterval()),
		"paused":             types.BoolValue(s.GetPaused()),
		"acl_filters":        filtersList,
	})
	diags.Append(d...)
	return obj, diags
}

func schemaRegistrySyncOptionsObject(_ context.Context, sr *corev2.SchemaRegistrySyncOptions) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	attrTypes := SchemaRegistrySyncOptionsModel{}.AttributeTypes()
	if sr == nil {
		return types.ObjectNull(attrTypes), diags
	}
	enabled := sr.GetShadowSchemaRegistryTopic() != nil
	obj, d := types.ObjectValue(attrTypes, map[string]attr.Value{
		"shadow_schema_registry_topic": types.BoolValue(enabled),
	})
	diags.Append(d...)
	return obj, diags
}

func nameFiltersToList(_ context.Context, filters []*corev2.NameFilter) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := types.ObjectType{AttrTypes: NameFilterModel{}.AttributeTypes()}
	vals := make([]attr.Value, 0, len(filters))
	for _, f := range filters {
		obj, d := types.ObjectValue(NameFilterModel{}.AttributeTypes(), map[string]attr.Value{
			"pattern_type": types.StringValue(f.GetPatternType().String()),
			"filter_type":  types.StringValue(f.GetFilterType().String()),
			"name":         types.StringValue(f.GetName()),
		})
		diags.Append(d...)
		vals = append(vals, obj)
	}
	l, d := types.ListValue(elemType, vals)
	diags.Append(d...)
	return l, diags
}

// preserveString returns null if the API returned "" and the prior state was null, avoiding inconsistent-after-apply.
func preserveString(apiVal string, hasPrior bool, priorVal types.String) types.String {
	if apiVal != "" {
		return types.StringValue(apiVal)
	}
	if hasPrior && !priorVal.IsNull() && !priorVal.IsUnknown() && priorVal.ValueString() != "" {
		return priorVal
	}
	return types.StringNull()
}

// preserveSensitive returns the prior-state value for fields the API never echoes back (passwords, TLS key).
func preserveSensitive(hasPrior bool, priorVal types.String) types.String {
	if hasPrior && !priorVal.IsNull() && !priorVal.IsUnknown() {
		return priorVal
	}
	return types.StringNull()
}
