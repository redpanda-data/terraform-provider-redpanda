// Copyright 2026 Redpanda Data, Inc.
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
	"strings"
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

// ClusterFake is a stateful in-memory ClusterService. The provider polls
// GetCluster for Create and Delete rather than the Operation, so Create stores
// the cluster READY without publishing its op and Delete makes GetCluster
// return NotFound; only Update publishes an Operation for AreWeDoneYet.
// UpdateMask is honored on top-level fields, matching what
// utils.GenerateProtobufDiffAndUpdateMask emits.
type ClusterFake struct {
	controlplanev1grpc.UnimplementedClusterServiceServer

	op       *OperationFake
	mu       sync.Mutex
	clusters map[string]*controlplanev1.Cluster
	seq      atomic.Uint64
	srURL    string

	// dualModel tracks clusters created or updated through the connections
	// field, the fake's analogue of the control plane's -pub/-prv listener
	// name suffix detection (usesDualListenerModel).
	dualModel map[string]bool

	// CreateMutator, when set, is applied to the freshly built cluster just
	// before it is stored, letting a test simulate server-side defaulting of
	// computed fields the provider did not send. Fires only at create.
	CreateMutator func(*controlplanev1.Cluster)

	// PublicPrivateListenersDisabled models an org WITHOUT the
	// enable-public-private-listeners feature flag: any request using
	// connections gets PermissionDenied, mirroring cloudv2's
	// checkPublicPrivateListenersFeatureFlagInUse. Zero value (flag on)
	// matches the org the tests model.
	PublicPrivateListenersDisabled bool
}

// NewClusterFake returns an empty fake bound to op.
func NewClusterFake(op *OperationFake) *ClusterFake {
	return &ClusterFake{op: op, clusters: map[string]*controlplanev1.Cluster{}, dualModel: map[string]bool{}}
}

// FlipToDualOutOfBand rewrites the stored cluster with the given name as if
// Redpanda fleet operations converted it to the dual listener model outside
// the public API: every service carries public+private SASL listeners with a
// projected sasl echo, connection_type reads back private (cloudv2
// mapper.getConnectionType: any private kafka listener wins), and the cluster
// is marked dual-model. Returns false when no cluster has that name.
func (f *ClusterFake) FlipToDualOutOfBand(name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for id, cl := range f.clusters {
		if cl.GetName() != name {
			continue
		}
		specs := []*controlplanev1.ConnectionSpec{
			{Type: connTypePublic, Auth: &controlplanev1.AuthSpec{Mode: controlplanev1.AuthMode_AUTH_MODE_SASL}},
			{Type: connTypePrivate, Auth: &controlplanev1.AuthSpec{Mode: controlplanev1.AuthMode_AUTH_MODE_SASL}},
		}
		cl.GetKafkaApi().Connections = connectionStatusesFromSpec(svcKafkaAPI, specs)
		cl.GetKafkaApi().Sasl = &controlplanev1.SASLSpec{Enabled: true}
		cl.GetHttpProxy().Connections = connectionStatusesFromSpec(svcHTTPProxy, specs)
		cl.GetHttpProxy().Sasl = &controlplanev1.SASLSpec{Enabled: true}
		cl.GetSchemaRegistry().Connections = connectionStatusesFromSpec(svcSchemaRegistry, specs)
		cl.GetSchemaRegistry().Sasl = &controlplanev1.SASLSpec{Enabled: true}
		cl.ConnectionType = connTypePrivate
		f.dualModel[id] = true
		return true
	}
	return false
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
	if err := f.validateCreateConnections(in); err != nil {
		return nil, err
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
			[]string{"mock-broker-0.mock.redpanda.cloud:9092"}, nil),
		HttpProxy: specToClusterHTTPProxy(in.GetHttpProxy(),
			"https://mock.http-proxy.redpanda.cloud", nil),
		SchemaRegistry: specToClusterSchemaRegistry(in.GetSchemaRegistry(), srURL, nil),
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
		// The control plane always reports which provider backs cloud storage.
		switch in.GetCloudProvider() {
		case controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS:
			cl.CloudStorage.SetAws(&controlplanev1.Cluster_CloudStorage_AWS{Arn: "arn:aws:s3:::tfrp-fake-cloud-storage"})
		case controlplanev1.CloudProvider_CLOUD_PROVIDER_GCP:
			cl.CloudStorage.SetGcp(&controlplanev1.Cluster_CloudStorage_GCP{Name: "tfrp-fake-cloud-storage"})
		default:
			// Azure carries extra fields; model it when a test needs it.
		}
	}
	if spec := in.GetAwsPrivateLink(); spec.GetEnabled() {
		cl.SetAwsPrivateLink(&controlplanev1.Cluster_AWSPrivateLink{
			Enabled:           spec.GetEnabled(),
			AllowedPrincipals: append([]string(nil), spec.GetAllowedPrincipals()...),
			ConnectConsole:    spec.GetConnectConsole(),
			SupportedRegions:  append([]string(nil), spec.GetSupportedRegions()...),
		})
	}
	if spec := in.GetGcpPrivateServiceConnect(); spec.GetEnabled() {
		cl.SetGcpPrivateServiceConnect(&controlplanev1.Cluster_GCPPrivateServiceConnect{
			Enabled:             spec.GetEnabled(),
			GlobalAccessEnabled: spec.GetGlobalAccessEnabled(),
			ConsumerAcceptList:  append([]*controlplanev1.GCPPrivateServiceConnectConsumer(nil), spec.GetConsumerAcceptList()...),
		})
	}
	if spec := in.GetAzurePrivateLink(); spec.GetEnabled() {
		cl.SetAzurePrivateLink(&controlplanev1.Cluster_AzurePrivateLink{
			Enabled:              spec.GetEnabled(),
			AllowedSubscriptions: append([]string(nil), spec.GetAllowedSubscriptions()...),
			ConnectConsole:       spec.GetConnectConsole(),
		})
	}
	// Every non-Azure cluster reads back a non-nil rpsql block, disabled or not.
	cl.SetRpsql(rpsqlReadStatus(in.GetRpsql(), in.GetCloudProvider(), in.GetZones()))
	// Mirror cloudv2 clearOxlaCMROnDisable (also called on the create path): a
	// cluster created without Redpanda SQL enabled cannot retain rpsql CMR fields.
	clearRpsqlCMROnDisable(cl)
	// Mirror cloudv2 redpandaConnectToPublic: redpanda_connect is populated on
	// every cluster (Connect ships with each install pack), not only when the
	// spec sets it.
	cl.SetRedpandaConnect(redpandaConnectStatus(in.GetRedpandaConnect()))
	// Mirror the GCP-only intent input (gcp_enable_global_access_api_gateway on
	// the write shape) onto the reported status field (different read-shape name).
	if in.GetCloudProvider() == controlplanev1.CloudProvider_CLOUD_PROVIDER_GCP {
		cl.SetGcpGlobalAccessApiGatewayEnabled(in.GetGcpEnableGlobalAccessApiGateway())
	}

	// connections[] is ALWAYS projected on read, legacy clusters included
	// (cloudv2 mapper.go: read-only derived view of the listeners; only the
	// connections INPUT is FF-gated).
	dual := len(in.GetKafkaApi().GetConnections()) > 0
	if dual {
		cl.GetKafkaApi().Connections = connectionStatusesFromSpec(svcKafkaAPI, in.GetKafkaApi().GetConnections())
		cl.GetHttpProxy().Connections = connectionStatusesFromSpec(svcHTTPProxy, in.GetHttpProxy().GetConnections())
		cl.GetSchemaRegistry().Connections = connectionStatusesFromSpec(svcSchemaRegistry, in.GetSchemaRegistry().GetConnections())
		cl.ConnectionType = connectionsClusterType(cl.GetKafkaApi().GetConnections(), in.GetConnectionType())
		// GET always projects a non-nil sasl block per service (mirrors
		// RedpandaListenersToPublic), which is what forces read-modify-write
		// clients onto leaf-granular update masks.
		cl.GetKafkaApi().Sasl = &controlplanev1.SASLSpec{Enabled: hasSASLConnection(in.GetKafkaApi().GetConnections())}
		cl.GetHttpProxy().Sasl = &controlplanev1.SASLSpec{Enabled: hasSASLConnection(in.GetHttpProxy().GetConnections())}
		cl.GetSchemaRegistry().Sasl = &controlplanev1.SASLSpec{Enabled: hasSASLConnection(in.GetSchemaRegistry().GetConnections())}
	} else {
		cl.GetKafkaApi().Connections = legacyConnectionProjection(in.GetConnectionType(), in.GetKafkaApi().GetMtls().GetEnabled(), "mock-broker-0.mock.redpanda.cloud:9092")
		cl.GetHttpProxy().Connections = legacyConnectionProjection(in.GetConnectionType(), in.GetHttpProxy().GetMtls().GetEnabled(), "https://mock.http-proxy.redpanda.cloud")
		cl.GetSchemaRegistry().Connections = legacyConnectionProjection(in.GetConnectionType(), in.GetSchemaRegistry().GetMtls().GetEnabled(), srURL)
	}

	if f.CreateMutator != nil {
		f.CreateMutator(cl)
	}

	f.mu.Lock()
	f.clusters[id] = cl
	if dual {
		f.dualModel[id] = true
	}
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

	// Mirror cloudv2 validateRPSqlCMRImmutability: reject a changed already-set
	// rpsql CMR leaf while Redpanda SQL is (or becomes) enabled. Runs before any
	// mutation so a rejection leaves the stored record untouched, matching the CP
	// validating the merged spec before persisting.
	if err := rpsqlCMRImmutability(cl, upd, req.GetUpdateMask().GetPaths()); err != nil {
		return nil, err
	}
	if err := rpsqlCMRRequiredOnEnable(cl, upd, req.GetUpdateMask().GetPaths()); err != nil {
		return nil, err
	}

	// Dual listener mode: run the connections guards and application before
	// the generic mask loop (mirrors the control plane validating and mapping
	// connections ahead of the legacy listener paths). Services the
	// connections update owns are skipped below.
	ownedByConnections, err := f.applyConnectionsUpdate(cl, upd, req.GetUpdateMask().GetPaths())
	if err != nil {
		return nil, err
	}

	// Fields whose wire type differs between ClusterUpdate and Cluster are
	// handled explicitly; the remaining type-matched fields use proto reflection.
	dstR := cl.ProtoReflect()
	srcR := upd.ProtoReflect()
	for _, path := range req.GetUpdateMask().GetPaths() {
		if connectionsOwnedPath(ownedByConnections, path) {
			continue
		}
		if strings.HasPrefix(path, "customer_managed_resources.") {
			// The provider expands the top-level "customer_managed_resources" mask
			// into the specific control-plane-updatable leaf paths (see
			// internal/clustermask.ExpandCustomerManagedResourceLeaves). Mirror
			// cloudv2's handleCustomerManagedResources: apply the changed leaf from
			// the update payload onto the stored read-shape record.
			applyCMRLeafUpdate(cl, upd.GetCustomerManagedResources(), path)
			continue
		}
		switch path {
		case "kafka_api":
			if upd.HasKafkaApi() {
				cl.KafkaApi = specToClusterKafkaAPI(upd.GetKafkaApi(),
					cl.GetKafkaApi().GetSeedBrokers(), cl.GetKafkaApi())
				// Legacy path only (dual services are owned above): keep the
				// always-on connections projection in step with the merged block.
				cl.GetKafkaApi().Connections = legacyConnectionProjection(cl.GetConnectionType(),
					cl.GetKafkaApi().GetMtls().GetEnabled(), firstOrEmpty(cl.GetKafkaApi().GetSeedBrokers()))
			}
		case "http_proxy":
			if upd.HasHttpProxy() {
				cl.HttpProxy = specToClusterHTTPProxy(upd.GetHttpProxy(),
					cl.GetHttpProxy().GetUrl(), cl.GetHttpProxy())
				cl.GetHttpProxy().Connections = legacyConnectionProjection(cl.GetConnectionType(),
					cl.GetHttpProxy().GetMtls().GetEnabled(), cl.GetHttpProxy().GetUrl())
			}
		case "schema_registry":
			if upd.HasSchemaRegistry() {
				cl.SchemaRegistry = specToClusterSchemaRegistry(upd.GetSchemaRegistry(),
					cl.GetSchemaRegistry().GetUrl(), cl.GetSchemaRegistry())
				cl.GetSchemaRegistry().Connections = legacyConnectionProjection(cl.GetConnectionType(),
					cl.GetSchemaRegistry().GetMtls().GetEnabled(), cl.GetSchemaRegistry().GetUrl())
			}
		case "aws_private_link":
			// The read drops the block entirely when disabled (mapper.go gates it
			// on PrivateLinkService.GetEnabled()); mirror that so a disable update
			// clears the stored block rather than storing a disabled one.
			if spec := upd.GetAwsPrivateLink(); spec.GetEnabled() {
				cl.SetAwsPrivateLink(&controlplanev1.Cluster_AWSPrivateLink{
					Enabled:           spec.GetEnabled(),
					AllowedPrincipals: append([]string(nil), spec.GetAllowedPrincipals()...),
					ConnectConsole:    spec.GetConnectConsole(),
					SupportedRegions:  append([]string(nil), spec.GetSupportedRegions()...),
				})
			} else if upd.HasAwsPrivateLink() {
				cl.SetAwsPrivateLink(nil)
			}
		case "azure_private_link":
			// azure_private_link's ClusterUpdate wire type (AzurePrivateLinkSpec)
			// differs from the read Cluster_AzurePrivateLink, so it needs an
			// explicit case: the default reflection branch would panic setting a
			// mismatched message type (as it does for aws/gcp private link).
			// Disabled reads back as no block (see aws_private_link).
			if spec := upd.GetAzurePrivateLink(); spec.GetEnabled() {
				cl.SetAzurePrivateLink(&controlplanev1.Cluster_AzurePrivateLink{
					Enabled:              spec.GetEnabled(),
					AllowedSubscriptions: append([]string(nil), spec.GetAllowedSubscriptions()...),
					ConnectConsole:       spec.GetConnectConsole(),
				})
			} else if upd.HasAzurePrivateLink() {
				cl.SetAzurePrivateLink(nil)
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
				// the one-time populate from empty is allowed. Disabling clears
				// the zones, which is not a "zone change" to block.
				if existing := cl.GetRpsql().GetZones(); upd.GetRpsql().GetEnabled() &&
					len(existing) > 0 && !slices.Equal(existing, effective) {
					return nil, status.Error(codes.InvalidArgument,
						"Redpanda SQL zones are immutable and cannot be changed after creation")
				}
				cl.SetRpsql(rpsqlReadStatus(upd.GetRpsql(), cl.GetCloudProvider(), cl.GetZones()))
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
			// Disabled reads back as no block (see aws_private_link).
			if spec := upd.GetGcpPrivateServiceConnect(); spec.GetEnabled() {
				cl.SetGcpPrivateServiceConnect(&controlplanev1.Cluster_GCPPrivateServiceConnect{
					Enabled:             spec.GetEnabled(),
					GlobalAccessEnabled: spec.GetGlobalAccessEnabled(),
					ConsumerAcceptList:  append([]*controlplanev1.GCPPrivateServiceConnectConsumer(nil), spec.GetConsumerAcceptList()...),
				})
			} else if upd.HasGcpPrivateServiceConnect() {
				cl.SetGcpPrivateServiceConnect(nil)
			}
		case "redpanda_connect.allowed_destination_cidr_ports":
			// LeafExpansions sends this granular path for redpanda_connect updates.
			// Echo through the read-mapper mirror so port_end=0 normalizes.
			if upd.HasRedpandaConnect() {
				rc := cl.GetRedpandaConnect()
				if rc == nil {
					rc = redpandaConnectStatus(nil)
				}
				rc.AllowedDestinationCidrPorts = normalizeCidrPorts(upd.GetRedpandaConnect().GetAllowedDestinationCidrPorts())
				cl.SetRedpandaConnect(rc)
			}
		case "gcp_enable_global_access_api_gateway":
			// Write-shape intent maps onto the differently-named read-shape status.
			cl.SetGcpGlobalAccessApiGatewayEnabled(upd.GetGcpEnableGlobalAccessApiGateway())
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
			// kafka_connect, kafka_api) have NO top-level pathMap entry: the API
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
	// Mirror cloudv2 clearOxlaCMROnDisable (called on the merged spec before
	// persisting): a disabled cluster cannot retain rpsql CMR fields.
	clearRpsqlCMROnDisable(cl)
	cl.UpdatedAt = timestamppb.Now()

	return &controlplanev1.UpdateClusterOperation{Operation: completedOp(f.op, upd.GetId())}, nil
}

// applyCMRLeafUpdate mirrors cloudv2's handleCustomerManagedResources: it copies
// the changed control-plane-updatable CMR leaf from the update payload
// (CustomerManagedResourcesUpdate, rpsql_* fields) onto the stored read-shape
// record. Every leaf is keyed by its public proto field name (cloudv2 maps the
// rpsql_* keys onto its internal oxla_* spec paths on its side).
func applyCMRLeafUpdate(cl *controlplanev1.Cluster, upd *controlplanev1.CustomerManagedResourcesUpdate, path string) {
	if upd == nil {
		return
	}
	cmr := cl.GetCustomerManagedResources()
	if cmr == nil {
		cmr = &controlplanev1.CustomerManagedResources{}
		cl.SetCustomerManagedResources(cmr)
	}
	switch path {
	case "customer_managed_resources.gcp.psc_nat_subnet_name":
		cmrReadGCP(cmr).SetPscNatSubnetName(upd.GetGcp().GetPscNatSubnetName())
	case "customer_managed_resources.gcp.rpsql_api_service_account.email":
		cmrReadGCP(cmr).SetRpsqlApiServiceAccount(&controlplanev1.GCPServiceAccount{Email: upd.GetGcp().GetRpsqlApiServiceAccount().GetEmail()})
	case "customer_managed_resources.gcp.rpsql_service_account.email":
		cmrReadGCP(cmr).SetRpsqlServiceAccount(&controlplanev1.GCPServiceAccount{Email: upd.GetGcp().GetRpsqlServiceAccount().GetEmail()})
	case "customer_managed_resources.gcp.rpsql_cloud_storage_bucket.name":
		cmrReadGCP(cmr).SetRpsqlCloudStorageBucket(&controlplanev1.CustomerManagedGoogleCloudStorageBucket{Name: upd.GetGcp().GetRpsqlCloudStorageBucket().GetName()})
	case "customer_managed_resources.gcp.rpsql_secret_manager_prefix":
		cmrReadGCP(cmr).SetRpsqlSecretManagerPrefix(upd.GetGcp().GetRpsqlSecretManagerPrefix())
	case "customer_managed_resources.aws.redpanda_connect_node_group_instance_profile.arn":
		cmrReadAWS(cmr).SetRedpandaConnectNodeGroupInstanceProfile(&controlplanev1.AWSInstanceProfile{Arn: upd.GetAws().GetRedpandaConnectNodeGroupInstanceProfile().GetArn()})
	case "customer_managed_resources.aws.redpanda_connect_security_group.arn":
		cmrReadAWS(cmr).SetRedpandaConnectSecurityGroup(&controlplanev1.AWSSecurityGroup{Arn: upd.GetAws().GetRedpandaConnectSecurityGroup().GetArn()})
	case "customer_managed_resources.aws.rpsql_node_group_instance_profile.arn":
		cmrReadAWS(cmr).SetRpsqlNodeGroupInstanceProfile(&controlplanev1.AWSInstanceProfile{Arn: upd.GetAws().GetRpsqlNodeGroupInstanceProfile().GetArn()})
	case "customer_managed_resources.aws.rpsql_security_group.arn":
		cmrReadAWS(cmr).SetRpsqlSecurityGroup(&controlplanev1.AWSSecurityGroup{Arn: upd.GetAws().GetRpsqlSecurityGroup().GetArn()})
	case "customer_managed_resources.aws.rpsql_cloud_storage_bucket.arn":
		cmrReadAWS(cmr).SetRpsqlCloudStorageBucket(&controlplanev1.CustomerManagedAWSCloudStorageBucket{Arn: upd.GetAws().GetRpsqlCloudStorageBucket().GetArn()})
	default:
		// Unknown CMR leaf path: no-op, mirroring the control plane dropping an
		// unmapped mask path.
	}
}

// cmrReadGCP returns the read-shape GCP sub-record, creating it if absent.
func cmrReadGCP(cmr *controlplanev1.CustomerManagedResources) *controlplanev1.CustomerManagedResources_GCP {
	gcp := cmr.GetGcp()
	if gcp == nil {
		gcp = &controlplanev1.CustomerManagedResources_GCP{}
		cmr.SetGcp(gcp)
	}
	return gcp
}

// cmrReadAWS returns the read-shape AWS sub-record, creating it if absent.
func cmrReadAWS(cmr *controlplanev1.CustomerManagedResources) *controlplanev1.CustomerManagedResources_AWS {
	aws := cmr.GetAws()
	if aws == nil {
		aws = &controlplanev1.CustomerManagedResources_AWS{}
		cmr.SetAws(aws)
	}
	return aws
}

// rpsqlImmutableLeaf pairs an rpsql CMR leaf's public mask path with readers for
// its existing (read-shape) and incoming (update-shape) scalar. Mirrors the field
// lists in cloudv2 pkg/rpsqlcmr/{aws,gcp}.go.
type rpsqlImmutableLeaf struct {
	path    string
	name    string
	old     func(*controlplanev1.CustomerManagedResources) string
	updated func(*controlplanev1.CustomerManagedResourcesUpdate) string
}

var rpsqlImmutableLeaves = []rpsqlImmutableLeaf{
	{
		"customer_managed_resources.aws.rpsql_node_group_instance_profile.arn", "rpsql_node_group_instance_profile",
		func(c *controlplanev1.CustomerManagedResources) string {
			return c.GetAws().GetRpsqlNodeGroupInstanceProfile().GetArn()
		},
		func(c *controlplanev1.CustomerManagedResourcesUpdate) string {
			return c.GetAws().GetRpsqlNodeGroupInstanceProfile().GetArn()
		},
	},
	{
		"customer_managed_resources.aws.rpsql_security_group.arn", "rpsql_security_group",
		func(c *controlplanev1.CustomerManagedResources) string {
			return c.GetAws().GetRpsqlSecurityGroup().GetArn()
		},
		func(c *controlplanev1.CustomerManagedResourcesUpdate) string {
			return c.GetAws().GetRpsqlSecurityGroup().GetArn()
		},
	},
	{
		"customer_managed_resources.aws.rpsql_cloud_storage_bucket.arn", "rpsql_cloud_storage_bucket",
		func(c *controlplanev1.CustomerManagedResources) string {
			return c.GetAws().GetRpsqlCloudStorageBucket().GetArn()
		},
		func(c *controlplanev1.CustomerManagedResourcesUpdate) string {
			return c.GetAws().GetRpsqlCloudStorageBucket().GetArn()
		},
	},
	{
		"customer_managed_resources.gcp.rpsql_api_service_account.email", "rpsql_api_service_account",
		func(c *controlplanev1.CustomerManagedResources) string {
			return c.GetGcp().GetRpsqlApiServiceAccount().GetEmail()
		},
		func(c *controlplanev1.CustomerManagedResourcesUpdate) string {
			return c.GetGcp().GetRpsqlApiServiceAccount().GetEmail()
		},
	},
	{
		"customer_managed_resources.gcp.rpsql_service_account.email", "rpsql_service_account",
		func(c *controlplanev1.CustomerManagedResources) string {
			return c.GetGcp().GetRpsqlServiceAccount().GetEmail()
		},
		func(c *controlplanev1.CustomerManagedResourcesUpdate) string {
			return c.GetGcp().GetRpsqlServiceAccount().GetEmail()
		},
	},
	{
		"customer_managed_resources.gcp.rpsql_cloud_storage_bucket.name", "rpsql_cloud_storage_bucket",
		func(c *controlplanev1.CustomerManagedResources) string {
			return c.GetGcp().GetRpsqlCloudStorageBucket().GetName()
		},
		func(c *controlplanev1.CustomerManagedResourcesUpdate) string {
			return c.GetGcp().GetRpsqlCloudStorageBucket().GetName()
		},
	},
	{
		"customer_managed_resources.gcp.rpsql_secret_manager_prefix", "rpsql_secret_manager_prefix",
		func(c *controlplanev1.CustomerManagedResources) string {
			return c.GetGcp().GetRpsqlSecretManagerPrefix()
		},
		func(c *controlplanev1.CustomerManagedResourcesUpdate) string {
			return c.GetGcp().GetRpsqlSecretManagerPrefix()
		},
	},
}

// rpsqlCMRImmutability mirrors cloudv2 validateRPSqlCMRImmutability
// (redpanda_service.go): while Redpanda SQL is enabled in the merged state, an
// already-set (non-empty) rpsql CMR leaf cannot change to a different value.
// The gate is the post-update enabled state: an update may toggle rpsql.enabled
// in the same request. Only masked leaves are compared (an unmasked leaf keeps
// its old value, so it can never violate). empty->value (first set) and no-op
// same-value writes are allowed.
func rpsqlCMRImmutability(cl *controlplanev1.Cluster, upd *controlplanev1.ClusterUpdate, paths []string) error {
	mergedEnabled := cl.GetRpsql().GetEnabled()
	if slices.Contains(paths, "rpsql.enabled") && upd.HasRpsql() {
		mergedEnabled = upd.GetRpsql().GetEnabled()
	}
	if !mergedEnabled {
		return nil
	}
	oldCMR := cl.GetCustomerManagedResources()
	newCMR := upd.GetCustomerManagedResources()
	for _, leaf := range rpsqlImmutableLeaves {
		if !slices.Contains(paths, leaf.path) {
			continue
		}
		if oldV := leaf.old(oldCMR); oldV != "" && oldV != leaf.updated(newCMR) {
			return status.Errorf(codes.InvalidArgument,
				"%s is immutable while Redpanda SQL is enabled", leaf.name)
		}
	}
	return nil
}

// rpsqlCMRRequiredOnEnable mirrors cloudv2 validateRPSqlCMRFields (mapper.go):
// when the merged state enables Redpanda SQL on a BYOVPC cluster, every rpsql
// CMR leaf of the cluster's arm must be non-empty in the effective spec, the
// existing value overlaid with the masked update delta. Runs before any
// mutation so a rejection leaves the stored record untouched.
func rpsqlCMRRequiredOnEnable(cl *controlplanev1.Cluster, upd *controlplanev1.ClusterUpdate, paths []string) error {
	mergedEnabled := cl.GetRpsql().GetEnabled()
	if slices.Contains(paths, "rpsql.enabled") && upd.HasRpsql() {
		mergedEnabled = upd.GetRpsql().GetEnabled()
	}
	if !mergedEnabled {
		return nil
	}
	oldCMR := cl.GetCustomerManagedResources()
	newCMR := upd.GetCustomerManagedResources()
	var arm string
	switch {
	case oldCMR.GetAws() != nil:
		arm = "customer_managed_resources.aws."
	case oldCMR.GetGcp() != nil:
		arm = "customer_managed_resources.gcp."
	default:
		return nil
	}
	for _, leaf := range rpsqlImmutableLeaves {
		if !strings.HasPrefix(leaf.path, arm) {
			continue
		}
		effective := leaf.old(oldCMR)
		if slices.Contains(paths, leaf.path) {
			effective = leaf.updated(newCMR)
		}
		if effective == "" {
			return status.Errorf(codes.InvalidArgument,
				"%s is required when Redpanda SQL is enabled in BYOVPC mode", leaf.name)
		}
	}
	return nil
}

// clearRpsqlCMROnDisable mirrors cloudv2 clearOxlaCMROnDisable: when Redpanda SQL
// is not enabled, the control plane nulls the rpsql CMR leaves so a disabled
// cluster cannot retain them. The surrounding (non-rpsql) CMR is untouched.
func clearRpsqlCMROnDisable(cl *controlplanev1.Cluster) {
	if cl.GetRpsql().GetEnabled() {
		return
	}
	cmr := cl.GetCustomerManagedResources()
	if cmr == nil {
		return
	}
	if aws := cmr.GetAws(); aws != nil {
		aws.SetRpsqlNodeGroupInstanceProfile(nil)
		aws.SetRpsqlSecurityGroup(nil)
		aws.SetRpsqlCloudStorageBucket(nil)
	}
	if gcp := cmr.GetGcp(); gcp != nil {
		gcp.SetRpsqlApiServiceAccount(nil)
		gcp.SetRpsqlServiceAccount(nil)
		gcp.SetRpsqlCloudStorageBucket(nil)
		gcp.SetRpsqlSecretManagerPrefix("")
	}
}

// redpandaConnectStatus mirrors cloudv2 redpandaConnectToPublic: the read
// shape always carries the install-pack Connect version, and stored cidr
// ports normalize on the way out.
func redpandaConnectStatus(spec *controlplanev1.Cluster_RedpandaConnect) *controlplanev1.Cluster_RedpandaConnect {
	return &controlplanev1.Cluster_RedpandaConnect{
		Version:                     "mock-connect-v1",
		AllowedDestinationCidrPorts: normalizeCidrPorts(spec.GetAllowedDestinationCidrPorts()),
	}
}

// normalizeCidrPorts mirrors cloudv2 cidrPortsInternalToPublic: a stored
// port_end of 0 (single-port rule) reads back as port_start.
func normalizeCidrPorts(src []*controlplanev1.Cluster_CidrPort) []*controlplanev1.Cluster_CidrPort {
	out := make([]*controlplanev1.Cluster_CidrPort, 0, len(src))
	for _, p := range src {
		portEnd := p.GetPortEnd()
		if portEnd == 0 {
			portEnd = p.GetPortStart()
		}
		out = append(out, &controlplanev1.Cluster_CidrPort{
			Cidr:      p.GetCidr(),
			PortStart: p.GetPortStart(),
			PortEnd:   portEnd,
		})
	}
	return out
}

// rpsqlReadStatus mirrors cloudv2 ApplyRedpandaOxlaSpec (defaulter.go) +
// redpandaSqlToPublic (mapper.go): every NON-Azure cluster reads back a non-nil
// rpsql block even when Redpanda SQL is disabled or omitted, because the
// defaulter stores a bare disabled spec (replicas 0, no zones); url/version are
// populated only once enabled (server derives on provisioning). Enabling with
// replicas 0 defaults to 1 (oxlaDefaultReplicasCount). Azure reads back nil:
// ApplyRedpandaOxlaSpec early-returns for Azure.
func rpsqlReadStatus(spec *controlplanev1.RPSql, provider controlplanev1.CloudProvider, clusterZones []string) *controlplanev1.RPSql {
	if provider == controlplanev1.CloudProvider_CLOUD_PROVIDER_AZURE {
		return nil
	}
	if !spec.GetEnabled() {
		// Disabled: the defaulter replaces the whole spec with a bare
		// {Enabled: false}, so replicas, zones, url and version are all cleared.
		return &controlplanev1.RPSql{Enabled: false}
	}
	replicas := spec.GetReplicas()
	if replicas == 0 {
		replicas = 1
	}
	return &controlplanev1.RPSql{
		Enabled:  true,
		Replicas: replicas,
		Zones:    append([]string(nil), oxlaEffectiveZones(spec, clusterZones)...),
		Url:      "https://mock.rpsql.redpanda.cloud",
		Version:  "mock-rpsql-v1",
	}
}

// oxlaEffectiveZones mirrors the control-plane defaulter: enabling Redpanda
// SQL with no zones assigns the first cluster zone.
func oxlaEffectiveZones(spec *controlplanev1.RPSql, clusterZones []string) []string {
	if spec.GetEnabled() && len(spec.GetZones()) == 0 && len(clusterZones) > 0 {
		return clusterZones[:1]
	}
	return spec.GetZones()
}

// keepUnsent mirrors the control plane's legacy listener update: a payload
// that omits mtls or sasl leaves the stored block as it is (cloudv2 mapper
// apiListenersPublicToPrivateForUpdateCluster returns early on a nil block).
func keepUnsent[T any](sent, stored *T) *T {
	if sent == nil {
		return stored
	}
	return sent
}

// specToClusterKafkaAPI converts a write-shape KafkaAPISpec to the read-shape
// Cluster_KafkaAPI, preserving the given seed brokers; mtls and sasl come
// from the spec when sent and from prev otherwise.
func specToClusterKafkaAPI(spec *controlplanev1.KafkaAPISpec, seedBrokers []string, prev *controlplanev1.Cluster_KafkaAPI) *controlplanev1.Cluster_KafkaAPI {
	return &controlplanev1.Cluster_KafkaAPI{
		SeedBrokers: seedBrokers,
		Mtls:        keepUnsent(spec.GetMtls(), prev.GetMtls()),
		Sasl:        keepUnsent(spec.GetSasl(), prev.GetSasl()),
	}
}

// specToClusterHTTPProxy converts a write-shape HTTPProxySpec to the
// read-shape Cluster_HTTPProxyStatus, preserving url; mtls and sasl come from
// the spec when sent and from prev otherwise.
func specToClusterHTTPProxy(spec *controlplanev1.HTTPProxySpec, url string, prev *controlplanev1.Cluster_HTTPProxyStatus) *controlplanev1.Cluster_HTTPProxyStatus {
	return &controlplanev1.Cluster_HTTPProxyStatus{
		Url:  url,
		Mtls: keepUnsent(spec.GetMtls(), prev.GetMtls()),
		Sasl: keepUnsent(spec.GetSasl(), prev.GetSasl()),
	}
}

// specToClusterSchemaRegistry converts a write-shape SchemaRegistrySpec to the
// read-shape Cluster_SchemaRegistryStatus, preserving url; mtls and sasl come
// from the spec when sent and from prev otherwise.
func specToClusterSchemaRegistry(spec *controlplanev1.SchemaRegistrySpec, url string, prev *controlplanev1.Cluster_SchemaRegistryStatus) *controlplanev1.Cluster_SchemaRegistryStatus {
	return &controlplanev1.Cluster_SchemaRegistryStatus{
		Url:  url,
		Mtls: keepUnsent(spec.GetMtls(), prev.GetMtls()),
		Sasl: keepUnsent(spec.GetSasl(), prev.GetSasl()),
	}
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
