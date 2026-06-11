// Copyright 2026 Redpanda Data, Inc.
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

package fakes

import (
	"context"
	"slices"
	"sync"
	"sync/atomic"

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1/controlplanev1grpc"
	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/clustermask"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const clusterIDBase uint64 = 0x8000_0000_0000_0000

func schemaRegistryURL(override string) string {
	if override != "" {
		return override
	}
	return "https://mock.schema-registry.redpanda.cloud"
}

// ClusterFake is a stateful in-memory implementation of ClusterService.
//
// The cluster CRUD flow diverges from the other async fakes — the provider
// uses RetryGetCluster (not AreWeDoneYet) for both Create and Delete, polling
// GetCluster until terminal state. So:
//
//   - CreateCluster stores the cluster in STATE_READY immediately and returns
//     an Operation whose ResourceId the provider extracts; the Operation is
//     never polled (CreateCluster's returned op skips Operation.Set).
//   - DeleteCluster removes the stored cluster; subsequent GetCluster returns
//     NotFound, which RetryGetCluster recognizes as termination.
//   - UpdateCluster is the only path that uses AreWeDoneYet — the returned
//     Operation IS published via completedOp.
//
// UpdateCluster honors UpdateMask via proto reflection on top-level fields,
// matching what utils.GenerateProtobufDiffAndUpdateMask emits.
type ClusterFake struct {
	controlplanev1grpc.UnimplementedClusterServiceServer

	op       *OperationFake
	mu       sync.Mutex
	clusters map[string]*controlplanev1.Cluster
	seq      atomic.Uint64
	srURL    string

	// CreateMutator, when set, is applied to the freshly built cluster just
	// before it is stored, letting a test simulate server-side defaulting of
	// computed fields the provider did not send. Fires only at create.
	CreateMutator func(*controlplanev1.Cluster)
}

// NewClusterFake returns an empty fake bound to op.
func NewClusterFake(op *OperationFake) *ClusterFake {
	return &ClusterFake{op: op, clusters: map[string]*controlplanev1.Cluster{}}
}

// Seed inserts a pre-built cluster directly into the fake's store. Used by
// dependent-resource tests (schema, schema_registry_acl) that need a cluster
// to exist without going through CreateCluster's TestStep cycle. If the fake
// has an SR URL configured (via SetSchemaRegistryURL) and the seeded cluster
// has no SchemaRegistry.Url, the configured URL is applied.
func (f *ClusterFake) Seed(cl *controlplanev1.Cluster) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.srURL != "" {
		if cl.SchemaRegistry == nil {
			cl.SchemaRegistry = &controlplanev1.Cluster_SchemaRegistryStatus{}
		}
		cl.SchemaRegistry.Url = f.srURL
	}
	f.clusters[cl.GetId()] = cl
}

// SetClusterByID inserts a minimal Cluster with the given id and dataplane URL.
// Call from ImportRoundTrip tests to allow ImportState's ClusterForID lookup to
// succeed without a real controlplane.
func (f *ClusterFake) SetClusterByID(id, dataplaneURL string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.clusters[id] = &controlplanev1.Cluster{
		Id:    id,
		State: controlplanev1.Cluster_STATE_READY,
		DataplaneApi: &controlplanev1.Cluster_DataplaneAPI{
			Url: dataplaneURL,
		},
	}
}

// SetSchemaRegistryURL records the SR URL for use by Seed and CreateCluster,
// and overwrites SchemaRegistry.Url on every already-stored cluster. mock.New
// calls this after starting the SR httptest server.
func (f *ClusterFake) SetSchemaRegistryURL(url string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.srURL = url
	for _, cl := range f.clusters {
		if cl.SchemaRegistry == nil {
			cl.SchemaRegistry = &controlplanev1.Cluster_SchemaRegistryStatus{}
		}
		cl.SchemaRegistry.Url = url
	}
}

// CreateCluster stores a new cluster pre-populated with every Computed-only
// surface the provider's Flatten reads (dataplane_api.url="bufnet",
// kafka_api, http_proxy, schema_registry, redpanda_console, prometheus,
// current/desired_redpanda_version). State is STATE_READY so RetryGetCluster's
// Create-side poll terminates on the first call.
func (f *ClusterFake) CreateCluster(_ context.Context, req *controlplanev1.CreateClusterRequest) (*controlplanev1.CreateClusterOperation, error) {
	in := req.GetCluster()
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "cluster is required")
	}
	id := xidLike(clusterIDBase + f.seq.Add(1))
	now := timestamppb.Now()
	f.mu.Lock()
	srURL := schemaRegistryURL(f.srURL)
	f.mu.Unlock()
	cl := &controlplanev1.Cluster{
		Id:                     id,
		Name:                   in.GetName(),
		ResourceGroupId:        in.GetResourceGroupId(),
		CloudProvider:          in.GetCloudProvider(),
		ConnectionType:         in.GetConnectionType(),
		Type:                   in.GetType(),
		NetworkId:              in.GetNetworkId(),
		Region:                 in.GetRegion(),
		Zones:                  append([]string(nil), in.GetZones()...),
		ThroughputTier:         in.GetThroughputTier(),
		State:                  controlplanev1.Cluster_STATE_READY,
		CreatedAt:              now,
		UpdatedAt:              now,
		ApiGatewayAccess:       in.GetApiGatewayAccess(),
		CurrentRedpandaVersion: "24.3.1",
		DesiredRedpandaVersion: "24.3.1",
		RedpandaNodeCount:      in.GetRedpandaNodeCount(),
		KafkaApi: specToClusterKafkaAPI(in.GetKafkaApi(),
			[]string{"mock-broker-0.mock.redpanda.cloud:9092"}),
		HttpProxy: specToClusterHTTPProxy(in.GetHttpProxy(),
			"https://mock.http-proxy.redpanda.cloud"),
		SchemaRegistry: specToClusterSchemaRegistry(in.GetSchemaRegistry(), srURL),
		RedpandaConsole: &controlplanev1.Cluster_RedpandaConsole{
			Url: "https://mock.console.redpanda.cloud",
		},
		Prometheus: &controlplanev1.Cluster_Prometheus{
			Url: "https://mock.prometheus.redpanda.cloud",
		},
		DataplaneApi: &controlplanev1.Cluster_DataplaneAPI{
			Url: "bufnet",
		},
		CustomerManagedResources: in.GetCustomerManagedResources(),
		MaintenanceWindowConfig:  in.GetMaintenanceWindowConfig(),
		ReadReplicaClusterIds:    append([]string(nil), in.GetReadReplicaClusterIds()...),
		CloudProviderTags:        in.GetCloudProviderTags(),
	}
	if cs := in.GetCloudStorage(); cs != nil {
		cl.CloudStorage = &controlplanev1.Cluster_CloudStorage{
			SkipDestroy: cs.GetSkipDestroy(),
		}
	}
	if in.HasAwsPrivateLink() {
		spec := in.GetAwsPrivateLink()
		cl.SetAwsPrivateLink(&controlplanev1.Cluster_AWSPrivateLink{
			Enabled:           spec.GetEnabled(),
			AllowedPrincipals: append([]string(nil), spec.GetAllowedPrincipals()...),
			ConnectConsole:    spec.GetConnectConsole(),
			SupportedRegions:  append([]string(nil), spec.GetSupportedRegions()...),
		})
	}
	if in.HasGcpPrivateServiceConnect() {
		spec := in.GetGcpPrivateServiceConnect()
		cl.SetGcpPrivateServiceConnect(&controlplanev1.Cluster_GCPPrivateServiceConnect{
			Enabled:             spec.GetEnabled(),
			GlobalAccessEnabled: spec.GetGlobalAccessEnabled(),
			ConsumerAcceptList:  append([]*controlplanev1.GCPPrivateServiceConnectConsumer(nil), spec.GetConsumerAcceptList()...),
		})
	}
	if in.HasRpsql() {
		cl.SetRpsql(rpsqlStatus(in.GetRpsql(), in.GetZones()))
	}

	if f.CreateMutator != nil {
		f.CreateMutator(cl)
	}

	f.mu.Lock()
	f.clusters[id] = cl
	f.mu.Unlock()

	// Provider extracts only ResourceId; never polls this op. Skip Operation.Set
	// since CreateCluster uses RetryGetCluster (not AreWeDoneYet) for completion
	// detection.
	op := &controlplanev1.Operation{
		Id:         "op-create-" + id,
		State:      controlplanev1.Operation_STATE_COMPLETED,
		ResourceId: &id,
	}
	return &controlplanev1.CreateClusterOperation{Operation: op}, nil
}

// GetCluster returns the stored cluster or NotFound.
func (f *ClusterFake) GetCluster(_ context.Context, req *controlplanev1.GetClusterRequest) (*controlplanev1.GetClusterResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cl, ok := f.clusters[req.GetId()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "cluster %q not found", req.GetId())
	}
	return &controlplanev1.GetClusterResponse{Cluster: cl}, nil
}

// UpdateCluster applies req.UpdateMask.Paths against the stored record via
// proto reflection. Each top-level path emitted by
// utils.GenerateProtobufDiffAndUpdateMask writes through; unmasked fields
// keep their prior value. Publishes a completed Operation for AreWeDoneYet.
func (f *ClusterFake) UpdateCluster(_ context.Context, req *controlplanev1.UpdateClusterRequest) (*controlplanev1.UpdateClusterOperation, error) {
	upd := req.GetCluster()
	if upd == nil {
		return nil, status.Error(codes.InvalidArgument, "cluster is required")
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	cl, ok := f.clusters[upd.GetId()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "cluster %q not found", upd.GetId())
	}

	// Fields whose wire type differs between ClusterUpdate and Cluster are
	// handled explicitly; the remaining type-matched fields use proto reflection.
	dstR := cl.ProtoReflect()
	srcR := upd.ProtoReflect()
	for _, path := range req.GetUpdateMask().GetPaths() {
		switch path {
		case "kafka_api":
			if upd.HasKafkaApi() {
				cl.KafkaApi = specToClusterKafkaAPI(upd.GetKafkaApi(),
					cl.GetKafkaApi().GetSeedBrokers())
			}
		case "http_proxy":
			if upd.HasHttpProxy() {
				cl.HttpProxy = specToClusterHTTPProxy(upd.GetHttpProxy(),
					cl.GetHttpProxy().GetUrl())
			}
		case "schema_registry":
			if upd.HasSchemaRegistry() {
				cl.SchemaRegistry = specToClusterSchemaRegistry(upd.GetSchemaRegistry(),
					cl.GetSchemaRegistry().GetUrl())
			}
		case "aws_private_link":
			if upd.HasAwsPrivateLink() {
				spec := upd.GetAwsPrivateLink()
				cl.SetAwsPrivateLink(&controlplanev1.Cluster_AWSPrivateLink{
					Enabled:           spec.GetEnabled(),
					AllowedPrincipals: append([]string(nil), spec.GetAllowedPrincipals()...),
					ConnectConsole:    spec.GetConnectConsole(),
					SupportedRegions:  append([]string(nil), spec.GetSupportedRegions()...),
				})
			}
		case "rpsql.enabled", "rpsql.replicas", "rpsql.zones":
			// The provider expands the top-level "rpsql" mask into these granular
			// paths; the diff payload still carries the full rpsql message.
			if upd.HasRpsql() {
				effective := oxlaEffectiveZones(upd.GetRpsql(), cl.GetZones())
				// validateOxlaZones: the zone must be one of the cluster's zones
				// (checked before immutability, matching the control plane).
				if len(cl.GetZones()) > 0 {
					for _, z := range effective {
						if !slices.Contains(cl.GetZones(), z) {
							return nil, status.Errorf(codes.InvalidArgument,
								"oxla zone %q is not one of the cluster zones", z)
						}
					}
				}
				// validateOxlaZonesImmutable: zones are immutable once set; only
				// the one-time populate from empty is allowed.
				if existing := cl.GetRpsql().GetZones(); len(existing) > 0 &&
					!slices.Equal(existing, effective) {
					return nil, status.Error(codes.InvalidArgument,
						"Redpanda SQL zones are immutable and cannot be changed after creation")
				}
				cl.SetRpsql(rpsqlStatus(upd.GetRpsql(), cl.GetZones()))
			}
		case "kafka_connect.enabled":
			// The control plane maps kafka_connect only at leaf granularity
			// (kafka_connect.enabled); there is no top-level "kafka_connect" entry.
			// Copy the (proto-deprecated) kafka_connect message via reflection to
			// avoid the deprecated typed accessors.
			kcFD := srcR.Descriptor().Fields().ByName("kafka_connect")
			if kcFD != nil && srcR.Has(kcFD) {
				dstR.Set(dstR.Descriptor().Fields().ByName("kafka_connect"), srcR.Get(kcFD))
			}
		case "gcp_private_service_connect":
			if upd.HasGcpPrivateServiceConnect() {
				spec := upd.GetGcpPrivateServiceConnect()
				cl.SetGcpPrivateServiceConnect(&controlplanev1.Cluster_GCPPrivateServiceConnect{
					Enabled:             spec.GetEnabled(),
					GlobalAccessEnabled: spec.GetGlobalAccessEnabled(),
					ConsumerAcceptList:  append([]*controlplanev1.GCPPrivateServiceConnectConsumer(nil), spec.GetConsumerAcceptList()...),
				})
			}
		case "cloud_storage":
			if upd.HasCloudStorage() {
				if cl.CloudStorage == nil {
					cl.CloudStorage = &controlplanev1.Cluster_CloudStorage{}
				}
				cl.CloudStorage.SkipDestroy = upd.GetCloudStorage().GetSkipDestroy()
			}
		case "cluster_configuration":
			if upd.HasClusterConfiguration() {
				uc := upd.GetClusterConfiguration()
				if cl.ClusterConfiguration == nil {
					cl.ClusterConfiguration = &controlplanev1.Cluster_ClusterConfiguration{}
				}
				cl.ClusterConfiguration.CustomProperties = uc.GetCustomProperties()
			}
		default:
			// Mirror the control plane: it translates the public mask through its
			// pathMap (cloudv2 .../services/cluster/v1/mapper.go) and silently
			// DROPS any path lacking a mapping. Several object fields (rpsql,
			// kafka_connect, kafka_api) have NO top-level pathMap entry — the API
			// accepts them only at leaf granularity. Applying an un-mapped
			// top-level path here by reflection would let a wrong (un-expanded)
			// mask pass tests the real API would reject, so apply only top-level
			// paths the backend actually accepts (generated from cloudv2's pathMap).
			if !clustermask.AcceptedTopLevel[path] {
				continue
			}
			dstFD := dstR.Descriptor().Fields().ByName(protoreflect.Name(path))
			srcFD := srcR.Descriptor().Fields().ByName(protoreflect.Name(path))
			if dstFD == nil || srcFD == nil {
				continue
			}
			dstR.Set(dstFD, srcR.Get(srcFD))
		}
	}
	cl.UpdatedAt = timestamppb.Now()

	return &controlplanev1.UpdateClusterOperation{Operation: completedOp(f.op, upd.GetId())}, nil
}

// rpsqlStatus mirrors the write-shape RPSql onto the read-shape record,
// assigning a mock endpoint URL when enabled (the real control plane
// populates url on provisioning; it stays empty while disabled).
func rpsqlStatus(spec *controlplanev1.RPSql, clusterZones []string) *controlplanev1.RPSql {
	if spec == nil {
		return nil
	}
	out := &controlplanev1.RPSql{
		Enabled:  spec.GetEnabled(),
		Replicas: spec.GetReplicas(),
		Zones:    append([]string(nil), oxlaEffectiveZones(spec, clusterZones)...),
	}
	if out.Enabled {
		out.Url = "https://mock.rpsql.redpanda.cloud"
		out.Version = "mock-rpsql-v1"
	}
	return out
}

// oxlaEffectiveZones mirrors the control-plane defaulter: enabling Redpanda
// SQL with no zones assigns the first cluster zone.
func oxlaEffectiveZones(spec *controlplanev1.RPSql, clusterZones []string) []string {
	if spec.GetEnabled() && len(spec.GetZones()) == 0 && len(clusterZones) > 0 {
		return clusterZones[:1]
	}
	return spec.GetZones()
}

// specToClusterKafkaAPI converts a write-shape KafkaAPISpec to the read-shape
// Cluster_KafkaAPI, preserving the given seed brokers and copying mtls/sasl.
func specToClusterKafkaAPI(spec *controlplanev1.KafkaAPISpec, seedBrokers []string) *controlplanev1.Cluster_KafkaAPI {
	out := &controlplanev1.Cluster_KafkaAPI{
		SeedBrokers: seedBrokers,
	}
	if spec != nil {
		out.Mtls = spec.GetMtls()
		out.Sasl = spec.GetSasl()
	}
	return out
}

// specToClusterHTTPProxy converts a write-shape HTTPProxySpec to the
// read-shape Cluster_HTTPProxyStatus, preserving url and copying mtls/sasl.
func specToClusterHTTPProxy(spec *controlplanev1.HTTPProxySpec, url string) *controlplanev1.Cluster_HTTPProxyStatus {
	out := &controlplanev1.Cluster_HTTPProxyStatus{Url: url}
	if spec != nil {
		out.Mtls = spec.GetMtls()
		out.Sasl = spec.GetSasl()
	}
	return out
}

// specToClusterSchemaRegistry converts a write-shape SchemaRegistrySpec to the
// read-shape Cluster_SchemaRegistryStatus, preserving url and copying mtls/sasl.
func specToClusterSchemaRegistry(spec *controlplanev1.SchemaRegistrySpec, url string) *controlplanev1.Cluster_SchemaRegistryStatus {
	out := &controlplanev1.Cluster_SchemaRegistryStatus{Url: url}
	if spec != nil {
		out.Mtls = spec.GetMtls()
		out.Sasl = spec.GetSasl()
	}
	return out
}

// DeleteCluster removes the stored cluster. The provider's Delete polls
// GetCluster via RetryGetCluster; once the cluster is gone from the map,
// GetCluster returns NotFound and RetryGetCluster terminates. No Operation
// is published for the same reason as Create.
func (f *ClusterFake) DeleteCluster(_ context.Context, req *controlplanev1.DeleteClusterRequest) (*controlplanev1.DeleteClusterOperation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.clusters[req.GetId()]; !ok {
		return nil, status.Errorf(codes.NotFound, "cluster %q not found", req.GetId())
	}
	delete(f.clusters, req.GetId())
	id := req.GetId()
	op := &controlplanev1.Operation{
		Id:         "op-delete-" + id,
		State:      controlplanev1.Operation_STATE_COMPLETED,
		ResourceId: &id,
	}
	return &controlplanev1.DeleteClusterOperation{Operation: op}, nil
}
