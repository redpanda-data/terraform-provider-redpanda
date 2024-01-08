package models

import "github.com/hashicorp/terraform-plugin-framework/types"

type Topic struct {
	Name              types.String          `tfsdk:"name"`
	PartitionCount    types.Number          `tfsdk:"partition_count"`
	ReplicationFactor types.Number          `tfsdk:"replication_factor"`
	Configuration     []*TopicConfiguration `tfsdk:"configuration"`
	AllowDeletion     types.Bool            `tfsdk:"allow_deletion"`
}

type TopicConfiguration struct {
	Name           types.String
	Type           types.String
	Value          types.String
	Source         types.String
	IsReadOnly     types.Bool
	IsSensitive    types.Bool
	ConfigSynonyms []*TopicConfigSynonym
	Documentation  types.String
}

type TopicConfigSynonym struct {
	Name   types.String
	Value  types.String
	Source types.String
}
