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
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	// shadowLinkNamePattern enforces DNS-1123 subdomain naming (≤63 chars).
	shadowLinkNamePattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	// secretRefPattern enforces the ${secrets.<ID>} format for sensitive fields
	// that must reference a dataplane secret instead of a plaintext value.
	secretRefPattern = regexp.MustCompile(`^\$\{secrets\.[a-zA-Z0-9_-]+\}$`)
	// rfc3339Pattern accepts RFC3339 timestamps for start_at_timestamp.
	rfc3339Pattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$`)
	// durationPattern accepts Go-style durations ("30s", "5m", "1h30m").
	durationPattern = regexp.MustCompile(`^\d+(\.\d+)?(ns|us|µs|ms|s|m|h)([+\-]?\d+(\.\d+)?(ns|us|µs|ms|s|m|h))*$`)
)

// nameFilterAttribute returns the NameFilter ListNestedAttribute used by sync option blocks.
func nameFilterAttribute(_ context.Context, description string) schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		Description:   description,
		Optional:      true,
		Computed:      true,
		PlanModifiers: []planmodifier.List{listplanmodifier.UseNonNullStateForUnknown()},
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"pattern_type": schema.StringAttribute{
					Description: "The matching pattern type. `PATTERN_TYPE_LITERAL` must match the filter exactly. `PATTERN_TYPE_PREFIX` matches anything that starts with filter. `PATTERN_TYPE_PREFIXED` matches anything that starts with filter (alias of `PATTERN_TYPE_PREFIX`).",
					Required:    true,
					Validators: []validator.String{
						stringvalidator.OneOf("PATTERN_TYPE_LITERAL", "PATTERN_TYPE_PREFIX", "PATTERN_TYPE_PREFIXED"),
					},
				},
				"filter_type": schema.StringAttribute{
					Description: "What type of filter this is, include or exclude. `FILTER_TYPE_INCLUDE` includes the items that match the filter; `FILTER_TYPE_EXCLUDE` excludes them.",
					Required:    true,
					Validators: []validator.String{
						stringvalidator.OneOf("FILTER_TYPE_INCLUDE", "FILTER_TYPE_EXCLUDE"),
					},
				},
				"name": schema.StringAttribute{
					Description: "The resource name, or `*`. Note if `*`, must be the only character and `pattern_type` must be `PATTERN_TYPE_LITERAL`.",
					Required:    true,
				},
			},
		},
	}
}

// ResourceShadowLinkSchema returns the schema for the ShadowLink resource.
func ResourceShadowLinkSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Description: "ShadowLink configures asynchronous data replication from a source Redpanda cluster to a shadow (destination) cluster. The link is configuration on the shadow cluster — no infrastructure is provisioned. Each broker in the shadow cluster runs internal replication tasks that pull from the source over the standard Kafka API. The source cluster is unaware of the link aside from increased fetch traffic. The shadow cluster must have `enable_shadow_linking=true` set in its `cluster_configuration.custom_properties_json`.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description:   "Human-readable name for the shadow link. Must be unique. Must follow Kubernetes DNS-1123 subdomain naming convention: lowercase alphanumeric characters, hyphens allowed; must start and end with alphanumeric character; maximum 63 characters. Immutable.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators: []validator.String{
					stringvalidator.RegexMatches(shadowLinkNamePattern, "must follow DNS-1123 subdomain naming (^[a-z0-9]([-a-z0-9]*[a-z0-9])?$)"),
					stringvalidator.LengthBetween(1, 63),
				},
			},
			"shadow_redpanda_id": schema.StringAttribute{
				Description:   "Shadow Redpanda cluster ID where the shadow link is created. This ID is immutable.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"source_redpanda_id": schema.StringAttribute{
				Description:   "Source Redpanda cluster ID. This field is optional. If provided, fetches bootstrap server information. Mutually exclusive with `client_options.bootstrap_servers` — exactly one must be set. Immutable: the proto only accepts it on `ShadowLinkCreate` and never returns it on Read, so a change forces destroy and recreate.",
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},

			"client_options": schema.SingleNestedAttribute{
				Description:   "ShadowLinkClientOptions configures the Kafka client connection settings.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Object{objectplanmodifier.UseNonNullStateForUnknown()},
				Attributes: map[string]schema.Attribute{
					"bootstrap_servers": schema.ListAttribute{
						Description:   "Bootstrap servers for the source cluster. Required if source Redpanda ID is not provided (mutually exclusive with `source_redpanda_id`).",
						Optional:      true,
						Computed:      true,
						ElementType:   types.StringType,
						PlanModifiers: []planmodifier.List{listplanmodifier.UseNonNullStateForUnknown()},
					},
					"client_id": schema.StringAttribute{
						Description:   "Client ID for the connection. Read-only.",
						Computed:      true,
						PlanModifiers: []planmodifier.String{stringplanmodifier.UseNonNullStateForUnknown()},
					},
					"source_cluster_id": schema.StringAttribute{
						Description:   "Source cluster ID. If provided, this is the expected ID of the source cluster: if it does not match, the connection will be rejected. Must match the `ClusterId` field returned in the Kafka Metadata response.",
						Optional:      true,
						Computed:      true,
						PlanModifiers: []planmodifier.String{stringplanmodifier.UseNonNullStateForUnknown()},
					},
					"tls": schema.SingleNestedAttribute{
						Description: "TLSSettings configures TLS encryption.",
						Optional:    true,
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Description:   "Enable TLS.",
								Optional:      true,
								Computed:      true,
								PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseNonNullStateForUnknown()},
							},
							"ca": schema.StringAttribute{
								Description: "The CA certificate for TLS.",
								Optional:    true,
							},
							"cert": schema.StringAttribute{
								Description: "Cert is the certificate for TLS. Key and Cert are optional but if one is provided, then both must be provided.",
								Optional:    true,
							},
							"key": schema.StringAttribute{
								Description: "The private key for TLS. Key and Cert are optional but if one is provided, then both must be provided. Must reference a dataplane secret in the form `${secrets.<NAME>}` (validated at plan time).",
								Optional:    true,
								Sensitive:   true,
								Validators: []validator.String{
									stringvalidator.RegexMatches(secretRefPattern, "must reference a dataplane secret in the form ${secrets.<ID>}"),
								},
							},
							"do_not_set_sni_hostname": schema.BoolAttribute{
								Description:   "Do not set SNI hostname.",
								Optional:      true,
								Computed:      true,
								PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseNonNullStateForUnknown()},
							},
						},
					},
					"authentication": schema.SingleNestedAttribute{
						Description: "Authentication config. Supports SASL/SCRAM and SASL/PLAIN.",
						Optional:    true,
						Attributes: map[string]schema.Attribute{
							"mechanism": schema.StringAttribute{
								Description: "SASL mechanism. One of `scram-sha-256`, `scram-sha-512`, or `plain`. The first two map to `ScramConfig` with the matching `ScramMechanism`; `plain` maps to `PlainConfig`.",
								Required:    true,
								Validators: []validator.String{
									stringvalidator.OneOf("scram-sha-256", "scram-sha-512", "plain"),
								},
							},
							"username": schema.StringAttribute{
								Description: "SCRAM/PLAIN username. The user must exist on the source cluster with at least `CLUSTER:DESCRIBE`, `TOPIC:DESCRIBE`, and `TOPIC:READ` ACLs for the topics being shadowed.",
								Required:    true,
							},
							"password": schema.StringAttribute{
								Description: "Password. Must reference a dataplane secret on the **shadow** cluster in the form `${secrets.<NAME>}` (validated at plan time). Use `redpanda_secret` to provision the secret.",
								Required:    true,
								Sensitive:   true,
								Validators: []validator.String{
									stringvalidator.RegexMatches(secretRefPattern, "must reference a dataplane secret in the form ${secrets.<ID>}"),
								},
							},
						},
					},
					// Kafka client tuning fields (proto: ShadowLinkClientOptions).
					"metadata_max_age_ms": schema.Int32Attribute{
						Description:   "Metadata refresh interval in milliseconds (defaults to 10000ms if 0).",
						Optional:      true,
						Computed:      true,
						PlanModifiers: []planmodifier.Int32{int32planmodifier.UseNonNullStateForUnknown()},
					},
					"effective_metadata_max_age_ms": schema.Int32Attribute{
						Description:   "The effective metadata max age in milliseconds. Read-only.",
						Computed:      true,
						PlanModifiers: []planmodifier.Int32{int32planmodifier.UseNonNullStateForUnknown()},
					},
					"connection_timeout_ms": schema.Int32Attribute{
						Description:   "Connection timeout in milliseconds (defaults to 1000ms if 0).",
						Optional:      true,
						Computed:      true,
						PlanModifiers: []planmodifier.Int32{int32planmodifier.UseNonNullStateForUnknown()},
					},
					"effective_connection_timeout_ms": schema.Int32Attribute{
						Description:   "The effective connection timeout in milliseconds. Read-only.",
						Computed:      true,
						PlanModifiers: []planmodifier.Int32{int32planmodifier.UseNonNullStateForUnknown()},
					},
					"retry_backoff_ms": schema.Int32Attribute{
						Description:   "Retry backoff in milliseconds (defaults to 100ms if 0).",
						Optional:      true,
						Computed:      true,
						PlanModifiers: []planmodifier.Int32{int32planmodifier.UseNonNullStateForUnknown()},
					},
					"effective_retry_backoff_ms": schema.Int32Attribute{
						Description:   "The effective retry backoff in milliseconds. Read-only.",
						Computed:      true,
						PlanModifiers: []planmodifier.Int32{int32planmodifier.UseNonNullStateForUnknown()},
					},
					"fetch_wait_max_ms": schema.Int32Attribute{
						Description:   "Maximum time to wait for fetch requests in milliseconds (defaults to 500ms if 0).",
						Optional:      true,
						Computed:      true,
						PlanModifiers: []planmodifier.Int32{int32planmodifier.UseNonNullStateForUnknown()},
					},
					"effective_fetch_wait_max_ms": schema.Int32Attribute{
						Description:   "The effective fetch wait max in milliseconds. Read-only.",
						Computed:      true,
						PlanModifiers: []planmodifier.Int32{int32planmodifier.UseNonNullStateForUnknown()},
					},
					"fetch_min_bytes": schema.Int32Attribute{
						Description:   "Minimum bytes to fetch (defaults to 5242880 bytes / 5 MiB if 0).",
						Optional:      true,
						Computed:      true,
						PlanModifiers: []planmodifier.Int32{int32planmodifier.UseNonNullStateForUnknown()},
					},
					"effective_fetch_min_bytes": schema.Int32Attribute{
						Description:   "The effective fetch min bytes. Read-only.",
						Computed:      true,
						PlanModifiers: []planmodifier.Int32{int32planmodifier.UseNonNullStateForUnknown()},
					},
					"fetch_max_bytes": schema.Int32Attribute{
						Description:   "Maximum bytes to fetch (defaults to 20971520 bytes / 20 MiB if 0).",
						Optional:      true,
						Computed:      true,
						PlanModifiers: []planmodifier.Int32{int32planmodifier.UseNonNullStateForUnknown()},
					},
					"effective_fetch_max_bytes": schema.Int32Attribute{
						Description:   "The effective fetch max bytes. Read-only.",
						Computed:      true,
						PlanModifiers: []planmodifier.Int32{int32planmodifier.UseNonNullStateForUnknown()},
					},
					"fetch_partition_max_bytes": schema.Int32Attribute{
						Description:   "Maximum bytes per partition to fetch (defaults to 1048576 bytes / 1 MiB if 0).",
						Optional:      true,
						Computed:      true,
						PlanModifiers: []planmodifier.Int32{int32planmodifier.UseNonNullStateForUnknown()},
					},
					"effective_fetch_partition_max_bytes": schema.Int32Attribute{
						Description:   "The effective fetch partition max bytes. Read-only.",
						Computed:      true,
						PlanModifiers: []planmodifier.Int32{int32planmodifier.UseNonNullStateForUnknown()},
					},
				},
			},

			"topic_metadata_sync_options": schema.SingleNestedAttribute{
				Description:   "Options for syncing topic metadata.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Object{objectplanmodifier.UseNonNullStateForUnknown()},
				Attributes: map[string]schema.Attribute{
					"interval": schema.StringAttribute{
						Description:   "How often to sync metadata, as a Go duration string (e.g. `30s`, `5m`). If 0 provided, defaults to 30 seconds.",
						Optional:      true,
						Computed:      true,
						PlanModifiers: []planmodifier.String{stringplanmodifier.UseNonNullStateForUnknown()},
						Validators: []validator.String{
							stringvalidator.RegexMatches(durationPattern, "must be a Go duration string (e.g. 30s, 5m, 1h)"),
						},
					},
					"effective_interval": schema.StringAttribute{
						Description:   "The effective interval for the task. Read-only.",
						Computed:      true,
						PlanModifiers: []planmodifier.String{stringplanmodifier.UseNonNullStateForUnknown()},
					},
					"auto_create_shadow_topic_filters": nameFilterAttribute(ctx,
						"List of filters that indicate which topics should be automatically created as shadow topics on the shadow cluster. This only controls automatic creation of shadow topics and does not affect the state of the mirror topic once it is created. Literal filters for `__consumer_offsets`, `_redpanda.audit_log` and `_schemas` will be rejected, as well as prefix filters to match topics prefixed with `_redpanda` or `__redpanda`. Wildcard `*` is permitted only for literal filters and will not match any topics that start with `_redpanda` or `__redpanda`. If users wish to shadow topics that start with `_redpanda` or `__redpanda`, they should provide a literal filter for those topics."),
					"synced_shadow_topic_properties": schema.ListAttribute{
						Description:   "List of topic properties that should be synced from the source topic. The following properties will always be replicated: `partition_count`, `max.message.bytes`, `cleanup.policy`, `timestamp.type`. The following properties are not allowed to be replicated and adding them to this list will result in an error: `redpanda.remote.readreplica`, `redpanda.remote.recovery`, `redpanda.remote.allowgaps`, `redpanda.virtual.cluster.id`, `redpanda.leaders.preference`, `redpanda.storage.mode`. This list is in addition to the default properties that will be synced — see `exclude_default`.",
						Optional:      true,
						Computed:      true,
						ElementType:   types.StringType,
						PlanModifiers: []planmodifier.List{listplanmodifier.UseNonNullStateForUnknown()},
					},
					"exclude_default": schema.BoolAttribute{
						Description:   "If false (default), the following topic properties are synced by default: `compression.type`, `retention.bytes`, `retention.ms`, `delete.retention.ms`, replication factor, `min.compaction.lag.ms`, `max.compaction.lag.ms`. If true, only the properties listed in `synced_shadow_topic_properties` will be synced.",
						Optional:      true,
						Computed:      true,
						PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseNonNullStateForUnknown()},
					},
					"start_offset": schema.SingleNestedAttribute{
						Description:   "The starting offset for new shadow topic partitions. Defaults to earliest. Only applies if the shadow partition is empty. Exactly one of the three sub-fields may be set.",
						Optional:      true,
						Computed:      true,
						PlanModifiers: []planmodifier.Object{objectplanmodifier.UseNonNullStateForUnknown()},
						Attributes: map[string]schema.Attribute{
							"at_earliest": schema.BoolAttribute{
								Description:   "Enables data replication from the earliest offset on the source topic/partition.",
								Optional:      true,
								Computed:      true,
								PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseNonNullStateForUnknown()},
							},
							"at_latest": schema.BoolAttribute{
								Description:   "Enables data replication from the latest offset on the source topic/partition.",
								Optional:      true,
								Computed:      true,
								PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseNonNullStateForUnknown()},
							},
							"at_timestamp": schema.StringAttribute{
								Description:   "Enables data replication from the first offset on the source topic/partition where the record's timestamp is at or after the specified RFC3339 timestamp.",
								Optional:      true,
								Computed:      true,
								PlanModifiers: []planmodifier.String{stringplanmodifier.UseNonNullStateForUnknown()},
								Validators: []validator.String{
									stringvalidator.RegexMatches(rfc3339Pattern, "must be RFC3339 (e.g. 2024-01-01T00:00:00Z)"),
								},
							},
						},
					},
					"paused": schema.BoolAttribute{
						Description:   "Allows user to pause the topic sync task. If paused, then the task will enter the 'paused' state and not sync topics or their properties from the source cluster.",
						Optional:      true,
						Computed:      true,
						PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseNonNullStateForUnknown()},
					},
				},
			},

			"consumer_offset_sync_options": schema.SingleNestedAttribute{
				Description:   "Options for syncing consumer offsets.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Object{objectplanmodifier.UseNonNullStateForUnknown()},
				Attributes: map[string]schema.Attribute{
					"interval": schema.StringAttribute{
						Description:   "Sync interval as a Go duration string (e.g. `30s`). If 0 provided, defaults to 30 seconds.",
						Optional:      true,
						Computed:      true,
						PlanModifiers: []planmodifier.String{stringplanmodifier.UseNonNullStateForUnknown()},
						Validators: []validator.String{
							stringvalidator.RegexMatches(durationPattern, "must be a Go duration string"),
						},
					},
					"effective_interval": schema.StringAttribute{
						Description:   "The effective interval for the task. Read-only.",
						Computed:      true,
						PlanModifiers: []planmodifier.String{stringplanmodifier.UseNonNullStateForUnknown()},
					},
					"paused": schema.BoolAttribute{
						Description:   "Allows user to pause the consumer offset sync task. If paused, then the task will enter the 'paused' state and not sync consumer offsets from the source cluster.",
						Optional:      true,
						Computed:      true,
						PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseNonNullStateForUnknown()},
					},
					"group_filters": nameFilterAttribute(ctx, "The filters."),
				},
			},

			"security_sync_options": schema.SingleNestedAttribute{
				Description:   "Options for syncing security settings.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Object{objectplanmodifier.UseNonNullStateForUnknown()},
				Attributes: map[string]schema.Attribute{
					"interval": schema.StringAttribute{
						Description:   "Sync interval as a Go duration string. If 0 provided, defaults to 30 seconds.",
						Optional:      true,
						Computed:      true,
						PlanModifiers: []planmodifier.String{stringplanmodifier.UseNonNullStateForUnknown()},
						Validators: []validator.String{
							stringvalidator.RegexMatches(durationPattern, "must be a Go duration string"),
						},
					},
					"effective_interval": schema.StringAttribute{
						Description:   "The effective interval for the task. Read-only.",
						Computed:      true,
						PlanModifiers: []planmodifier.String{stringplanmodifier.UseNonNullStateForUnknown()},
					},
					"paused": schema.BoolAttribute{
						Description:   "Allows user to pause the security settings sync task. If paused, then the task will enter the 'paused' state and will not sync security settings from the source cluster.",
						Optional:      true,
						Computed:      true,
						PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseNonNullStateForUnknown()},
					},
					"acl_filters": schema.ListNestedAttribute{
						Description:   "ACL filters.",
						Optional:      true,
						Computed:      true,
						PlanModifiers: []planmodifier.List{listplanmodifier.UseNonNullStateForUnknown()},
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"resource_filter": schema.SingleNestedAttribute{
									Description: "A filter to match ACLs for resources.",
									Required:    true,
									Attributes: map[string]schema.Attribute{
										"resource_type": schema.StringAttribute{
											Description: "The ACL resource type to match. `ACL_RESOURCE_ANY` is a wildcard for selecting any ACL resource; `ACL_RESOURCE_CLUSTER` is cluster-wide; `ACL_RESOURCE_GROUP`, `ACL_RESOURCE_TOPIC`, `ACL_RESOURCE_TXN_ID`, `ACL_RESOURCE_SR_SUBJECT`, `ACL_RESOURCE_SR_REGISTRY`, `ACL_RESOURCE_SR_ANY` cover the named resources.",
											Required:    true,
											Validators: []validator.String{
												stringvalidator.OneOf(
													"ACL_RESOURCE_ANY",
													"ACL_RESOURCE_CLUSTER",
													"ACL_RESOURCE_GROUP",
													"ACL_RESOURCE_TOPIC",
													"ACL_RESOURCE_TXN_ID",
													"ACL_RESOURCE_SR_SUBJECT",
													"ACL_RESOURCE_SR_REGISTRY",
													"ACL_RESOURCE_SR_ANY",
												),
											},
										},
										"pattern_type": schema.StringAttribute{
											Description: "The ACL pattern type. `ACL_PATTERN_ANY` matches any pattern; `ACL_PATTERN_LITERAL` matches a literal string; `ACL_PATTERN_PREFIXED` matches a prefix; `ACL_PATTERN_MATCH` serves as a catch-all for all the names of a topic the principal is authorized to access.",
											Required:    true,
											Validators: []validator.String{
												stringvalidator.OneOf("ACL_PATTERN_ANY", "ACL_PATTERN_LITERAL", "ACL_PATTERN_PREFIXED", "ACL_PATTERN_MATCH"),
											},
										},
										"name": schema.StringAttribute{
											Description:   "Name. If not given, will default to match all items in `resource_type`. Note that asterisk `*` is literal and matches resource ACLs that are named `*`.",
											Optional:      true,
											Computed:      true,
											PlanModifiers: []planmodifier.String{stringplanmodifier.UseNonNullStateForUnknown()},
										},
									},
								},
								"access_filter": schema.SingleNestedAttribute{
									Description: "Filter an ACL based on its access.",
									Required:    true,
									Attributes: map[string]schema.Attribute{
										"principal": schema.StringAttribute{
											Description:   "The name of the principal. If not set, will default to match all principals with the specified `operation` and `permission_type`.",
											Optional:      true,
											Computed:      true,
											PlanModifiers: []planmodifier.String{stringplanmodifier.UseNonNullStateForUnknown()},
										},
										"operation": schema.StringAttribute{
											Description: "The ACL operation to match. One of `ACL_OPERATION_ANY`, `ACL_OPERATION_READ`, `ACL_OPERATION_WRITE`, `ACL_OPERATION_CREATE`, `ACL_OPERATION_REMOVE`, `ACL_OPERATION_ALTER`, `ACL_OPERATION_DESCRIBE`, `ACL_OPERATION_CLUSTER_ACTION`, `ACL_OPERATION_DESCRIBE_CONFIGS`, `ACL_OPERATION_ALTER_CONFIGS`, `ACL_OPERATION_IDEMPOTENT_WRITE`.",
											Required:    true,
											Validators: []validator.String{
												stringvalidator.OneOf(
													"ACL_OPERATION_ANY",
													"ACL_OPERATION_READ",
													"ACL_OPERATION_WRITE",
													"ACL_OPERATION_CREATE",
													"ACL_OPERATION_REMOVE",
													"ACL_OPERATION_ALTER",
													"ACL_OPERATION_DESCRIBE",
													"ACL_OPERATION_CLUSTER_ACTION",
													"ACL_OPERATION_DESCRIBE_CONFIGS",
													"ACL_OPERATION_ALTER_CONFIGS",
													"ACL_OPERATION_IDEMPOTENT_WRITE",
												),
											},
										},
										"permission_type": schema.StringAttribute{
											Description: "The permission type. One of `ACL_PERMISSION_TYPE_ANY`, `ACL_PERMISSION_TYPE_ALLOW`, `ACL_PERMISSION_TYPE_DENY`.",
											Required:    true,
											Validators: []validator.String{
												stringvalidator.OneOf("ACL_PERMISSION_TYPE_ANY", "ACL_PERMISSION_TYPE_ALLOW", "ACL_PERMISSION_TYPE_DENY"),
											},
										},
										"host": schema.StringAttribute{
											Description:   "The host to match. If not set, will default to match all hosts with the specified `operation` and `permission_type`. Note that the asterisk `*` is literal and matches hosts that are set to `*`.",
											Optional:      true,
											Computed:      true,
											PlanModifiers: []planmodifier.String{stringplanmodifier.UseNonNullStateForUnknown()},
										},
									},
								},
							},
						},
						Validators: []validator.List{
							listvalidator.SizeAtLeast(0),
						},
					},
				},
			},

			"schema_registry_sync_options": schema.SingleNestedAttribute{
				Description:   "Options for how the Schema Registry is synced. The proto exposes a single oneof case (`shadow_schema_registry_topic`) modeled here as a bool.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Object{objectplanmodifier.UseNonNullStateForUnknown()},
				Attributes: map[string]schema.Attribute{
					"shadow_schema_registry_topic": schema.BoolAttribute{
						Description:   "If true, the Shadow Link will attempt to add the `_schemas` topic to the list of Shadow Topics as long as (1) the `_schemas` topic exists on the source cluster and (2) the `_schemas` topic does not exist on the shadow cluster, or it is empty. If either condition is not met, the `_schemas` topic will not be shadowed. Unsetting this flag will not remove the `_schemas` topic from shadowing if it has already been added. Once made a shadow topic, the `_schemas` topic is replicated byte-for-byte; to stop shadowing it, unset this field then either fail-over the topic or delete it.",
						Optional:      true,
						Computed:      true,
						PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseNonNullStateForUnknown()},
					},
				},
			},

			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Shadow link ID. Read-only.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"state": schema.StringAttribute{
				Computed:    true,
				Description: "Current state of the shadow link. One of `STATE_CREATING`, `STATE_CREATION_FAILED`, `STATE_ACTIVE`, `STATE_PAUSED`, `STATE_DELETING`, `STATE_DELETION_FAILED`. Read-only.",
			},
			"reason": schema.StringAttribute{
				Computed:    true,
				Description: "Reason provides additional context for the current state. Read-only.",
			},
			"allow_deletion": schema.BoolAttribute{
				Description: "Allows deletion of the shadow link. Defaults to false.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"timeouts": timeouts.Attributes(ctx, timeouts.Opts{
				Create: true,
				Update: true,
				Delete: true,
			}),
		},
	}
}

// ConfigValidators enforces the source_redpanda_id XOR client_options.bootstrap_servers rule.
func (*ShadowLink) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot("source_redpanda_id"),
			path.MatchRoot("client_options").AtName("bootstrap_servers"),
		),
	}
}
