package serverlesscluster

import (
	"time"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	serverlessclustermodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/serverlesscluster"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

// generateModel populates the Cluster model to be persisted to state for Create, Read and Update operations. It is also indirectly used by Import
func generateModel(cluster *controlplanev1.ServerlessCluster) *serverlessclustermodel.ResourceModel {
	output := &serverlessclustermodel.ResourceModel{
		Name:             types.StringValue(cluster.Name),
		ServerlessRegion: types.StringValue(cluster.ServerlessRegion),
		ResourceGroupID:  types.StringValue(cluster.ResourceGroupId),
		ID:               types.StringValue(cluster.Id),
	}

	// Deprecated cluster_api_url (kept for backward compatibility)
	if cluster.DataplaneApi != nil {
		output.ClusterAPIURL = types.StringValue(cluster.DataplaneApi.Url)
	}

	// Set private_link_id if present
	if cluster.PrivateLinkId != nil {
		output.PrivateLinkID = types.StringValue(*cluster.PrivateLinkId)
	} else {
		output.PrivateLinkID = types.StringNull()
	}

	// Set networking_config if present
	if cluster.NetworkingConfig != nil {
		networkingConfigObj, _ := types.ObjectValue(
			map[string]attr.Type{
				"private": types.StringType,
				"public":  types.StringType,
			},
			map[string]attr.Value{
				"private": types.StringValue(cluster.NetworkingConfig.Private.String()),
				"public":  types.StringValue(cluster.NetworkingConfig.Public.String()),
			},
		)
		output.NetworkingConfig = networkingConfigObj
	} else {
		output.NetworkingConfig = types.ObjectNull(map[string]attr.Type{
			"private": types.StringType,
			"public":  types.StringType,
		})
	}

	// Set kafka_api
	if cluster.KafkaApi != nil {
		seedBrokers := make([]attr.Value, len(cluster.KafkaApi.SeedBrokers))
		for i, broker := range cluster.KafkaApi.SeedBrokers {
			seedBrokers[i] = types.StringValue(broker)
		}
		privateSeedBrokers := make([]attr.Value, len(cluster.KafkaApi.PrivateSeedBrokers))
		for i, broker := range cluster.KafkaApi.PrivateSeedBrokers {
			privateSeedBrokers[i] = types.StringValue(broker)
		}

		kafkaAPIObj, _ := types.ObjectValue(
			map[string]attr.Type{
				"seed_brokers":         types.ListType{ElemType: types.StringType},
				"private_seed_brokers": types.ListType{ElemType: types.StringType},
			},
			map[string]attr.Value{
				"seed_brokers":         types.ListValueMust(types.StringType, seedBrokers),
				"private_seed_brokers": types.ListValueMust(types.StringType, privateSeedBrokers),
			},
		)
		output.KafkaAPI = kafkaAPIObj
	} else {
		output.KafkaAPI = types.ObjectNull(map[string]attr.Type{
			"seed_brokers":         types.ListType{ElemType: types.StringType},
			"private_seed_brokers": types.ListType{ElemType: types.StringType},
		})
	}

	// Set schema_registry
	if cluster.SchemaRegistry != nil {
		schemaRegistryObj, _ := types.ObjectValue(
			map[string]attr.Type{
				"url":         types.StringType,
				"private_url": types.StringType,
			},
			map[string]attr.Value{
				"url":         types.StringValue(cluster.SchemaRegistry.Url),
				"private_url": types.StringValue(cluster.SchemaRegistry.PrivateUrl),
			},
		)
		output.SchemaRegistry = schemaRegistryObj
	} else {
		output.SchemaRegistry = types.ObjectNull(map[string]attr.Type{
			"url":         types.StringType,
			"private_url": types.StringType,
		})
	}

	// Set dataplane_api
	if cluster.DataplaneApi != nil {
		dataplaneAPIObj, _ := types.ObjectValue(
			map[string]attr.Type{
				"url":         types.StringType,
				"private_url": types.StringType,
			},
			map[string]attr.Value{
				"url":         types.StringValue(cluster.DataplaneApi.Url),
				"private_url": types.StringValue(cluster.DataplaneApi.PrivateUrl),
			},
		)
		output.DataplaneAPI = dataplaneAPIObj
	} else {
		output.DataplaneAPI = types.ObjectNull(map[string]attr.Type{
			"url":         types.StringType,
			"private_url": types.StringType,
		})
	}

	// Set state
	output.State = types.StringValue(cluster.GetState().String())

	// Set tags
	if tags := cluster.GetTags(); len(tags) > 0 {
		output.Tags = utils.StringMapToTypeMap(tags)
	} else {
		output.Tags = types.MapNull(types.StringType)
	}

	// Set planned_deletion
	plannedDeletionType := map[string]attr.Type{
		"delete_after": types.StringType,
		"reason":       types.StringType,
	}
	if cluster.HasPlannedDeletion() {
		pd := cluster.GetPlannedDeletion()
		deleteAfter := types.StringNull()
		if pd.GetDeleteAfter() != nil {
			deleteAfter = types.StringValue(pd.GetDeleteAfter().AsTime().Format(time.RFC3339))
		}
		pdObj, _ := types.ObjectValue(plannedDeletionType, map[string]attr.Value{
			"delete_after": deleteAfter,
			"reason":       types.StringValue(pd.GetReason()),
		})
		output.PlannedDeletion = pdObj
	} else {
		output.PlannedDeletion = types.ObjectNull(plannedDeletionType)
	}

	// Set console URLs
	output.ConsoleURL = types.StringValue(cluster.ConsoleUrl)
	output.ConsolePrivateURL = types.StringValue(cluster.ConsolePrivateUrl)

	// Set prometheus
	if cluster.Prometheus != nil {
		prometheusObj, _ := types.ObjectValue(
			map[string]attr.Type{
				"url":         types.StringType,
				"private_url": types.StringType,
			},
			map[string]attr.Value{
				"url":         types.StringValue(cluster.Prometheus.Url),
				"private_url": types.StringValue(cluster.Prometheus.PrivateUrl),
			},
		)
		output.Prometheus = prometheusObj
	} else {
		output.Prometheus = types.ObjectNull(map[string]attr.Type{
			"url":         types.StringType,
			"private_url": types.StringType,
		})
	}

	return output
}

// generateDataModel populates the DataModel for datasource operations
func generateDataModel(cluster *controlplanev1.ServerlessCluster) *serverlessclustermodel.DataModel {
	output := &serverlessclustermodel.DataModel{
		Name:             types.StringValue(cluster.Name),
		ServerlessRegion: types.StringValue(cluster.ServerlessRegion),
		ResourceGroupID:  types.StringValue(cluster.ResourceGroupId),
		ID:               types.StringValue(cluster.Id),
	}
	if cluster.DataplaneApi != nil {
		output.ClusterAPIURL = types.StringValue(cluster.DataplaneApi.Url)
	}
	return output
}
