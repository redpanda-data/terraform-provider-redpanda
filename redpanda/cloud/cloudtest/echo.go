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

// Package cloudtest provides an in-process fake Redpanda Cloud control
// plane for tests. Test-only; do not import from production code.
package cloudtest

import (
	"time"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	dataplanev1 "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// FakeClusterID is the synthetic cluster ID assigned to every cluster
// created via the fake. Stable so tests can hardcode it.
const FakeClusterID = "rp-fake000000000000000"

// FakeCreatedAt is the synthetic creation timestamp baked into every
// echoed cluster. Stable so test fixtures can compare against it.
var FakeCreatedAt = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

// EchoFromClusterCreate produces a *Cluster mirroring the config-driven
// fields of a ClusterCreate spec. Extend this helper rather than forking
// it — every test-mode round-trip flows through here, so a single source
// of truth keeps unit and acceptance tests in lockstep.
func EchoFromClusterCreate(req *controlplanev1.ClusterCreate) *controlplanev1.Cluster {
	if req == nil {
		return nil
	}
	c := &controlplanev1.Cluster{
		Id:                FakeClusterID,
		Name:              req.GetName(),
		ResourceGroupId:   req.GetResourceGroupId(),
		ThroughputTier:    req.GetThroughputTier(),
		Type:              req.GetType(),
		ConnectionType:    req.GetConnectionType(),
		NetworkId:         req.GetNetworkId(),
		CloudProvider:     req.GetCloudProvider(),
		Region:            req.GetRegion(),
		Zones:             req.GetZones(),
		State:             controlplanev1.Cluster_STATE_READY,
		CreatedAt:         timestamppb.New(FakeCreatedAt),
		CloudProviderTags: req.GetCloudProviderTags(),
		// Always populate so the framework's "all values must be known
		// after apply" check passes for the Computed cluster_api_url.
		DataplaneApi: &controlplanev1.Cluster_DataplaneAPI{
			Url: "https://api-fake.byoc.test.redpanda.com",
		},
	}
	if req.RedpandaVersion != nil {
		c.CurrentRedpandaVersion = req.GetRedpandaVersion()
	}
	// kafka_api / http_proxy / schema_registry are intentionally synthesized
	// unconditionally: the schema marks them Optional+Computed, and the real
	// control plane returns these blocks populated for every dedicated
	// cluster regardless of whether the user supplied mtls/sasl. Mtls/Sasl
	// sub-fields stay conditional so a sasl-only or mtls-only cluster
	// reads back the way the user configured it.
	c.KafkaApi = &controlplanev1.Cluster_KafkaAPI{}
	if req.KafkaApi != nil {
		c.KafkaApi.Mtls = req.GetKafkaApi().GetMtls()
		c.KafkaApi.Sasl = req.GetKafkaApi().GetSasl()
	}
	c.HttpProxy = &controlplanev1.Cluster_HTTPProxyStatus{}
	if req.HttpProxy != nil {
		c.HttpProxy.Mtls = req.GetHttpProxy().GetMtls()
		c.HttpProxy.Sasl = req.GetHttpProxy().GetSasl()
	}
	c.SchemaRegistry = &controlplanev1.Cluster_SchemaRegistryStatus{}
	if req.SchemaRegistry != nil {
		c.SchemaRegistry.Mtls = req.GetSchemaRegistry().GetMtls()
		c.SchemaRegistry.Sasl = req.GetSchemaRegistry().GetSasl()
	}
	// Real CP omits aws_private_link when disabled.
	if req.AwsPrivateLink != nil && req.GetAwsPrivateLink().GetEnabled() {
		c.AwsPrivateLink = &controlplanev1.Cluster_AWSPrivateLink{
			Enabled:           req.GetAwsPrivateLink().GetEnabled(),
			ConnectConsole:    req.GetAwsPrivateLink().GetConnectConsole(),
			AllowedPrincipals: req.GetAwsPrivateLink().GetAllowedPrincipals(),
			SupportedRegions:  req.GetAwsPrivateLink().GetSupportedRegions(),
			Status:            fakeAwsPrivateLinkStatus(req.GetAwsPrivateLink().GetEnabled()),
		}
	}
	if req.GcpPrivateServiceConnect != nil {
		c.GcpPrivateServiceConnect = &controlplanev1.Cluster_GCPPrivateServiceConnect{
			Enabled:             req.GetGcpPrivateServiceConnect().GetEnabled(),
			GlobalAccessEnabled: req.GetGcpPrivateServiceConnect().GetGlobalAccessEnabled(),
			ConsumerAcceptList:  req.GetGcpPrivateServiceConnect().GetConsumerAcceptList(),
		}
	}
	if req.AzurePrivateLink != nil {
		c.AzurePrivateLink = &controlplanev1.Cluster_AzurePrivateLink{
			Enabled:              req.GetAzurePrivateLink().GetEnabled(),
			ConnectConsole:       req.GetAzurePrivateLink().GetConnectConsole(),
			AllowedSubscriptions: req.GetAzurePrivateLink().GetAllowedSubscriptions(),
		}
	}
	if req.MaintenanceWindowConfig != nil {
		c.MaintenanceWindowConfig = req.GetMaintenanceWindowConfig()
	}
	// Real backend always returns console + prometheus endpoints for
	// ready clusters. Populating them keeps the schema's
	// UseNonNullStateForUnknown parents null-safe.
	c.RedpandaConsole = &controlplanev1.Cluster_RedpandaConsole{
		Url: "https://console-fake.byoc.test.redpanda.com",
	}
	c.Prometheus = &controlplanev1.Cluster_Prometheus{
		Url: "https://prometheus-fake.byoc.test.redpanda.com/api/v1/metrics",
	}
	if req.KafkaConnect != nil { //nolint:staticcheck // Field is deprecated but still supported
		c.KafkaConnect = req.GetKafkaConnect() //nolint:staticcheck // Field is deprecated but still supported
	}
	if req.ClusterConfiguration != nil {
		// Only custom_properties round-trips; computed_properties is
		// server-only.
		c.ClusterConfiguration = &controlplanev1.Cluster_ClusterConfiguration{
			CustomProperties: req.GetClusterConfiguration().GetCustomProperties(),
		}
	}
	RecomputeClusterEndpoints(c)
	return c
}

// RecomputeClusterEndpoints populates the URL fields under
// kafka_api.all_seed_brokers, http_proxy.all_urls, and
// schema_registry.all_urls. The sasl / mtls fields are always set; the
// private_link_sasl / private_link_mtls fields are set only when a
// private-link service is enabled on the cluster, mirroring the real
// control plane's behavior. Must be called after every CRUD operation so
// the URL fields stay in lockstep with the cluster's private-link state —
// without that, plan-modifier bugs that depend on the URL transition
// (e.g. UseStateForUnknown carrying stale URLs forward across a PL
// toggle) are not reproducible against the fake.
func RecomputeClusterEndpoints(c *controlplanev1.Cluster) {
	if c == nil {
		return
	}
	pl := privateLinkActive(c)

	const (
		seedSasl     = "seed-fake.byoc.test.redpanda.com:9092"
		seedMtls     = "seed-fake.byoc.test.redpanda.com:9094"
		seedPLSasl   = "seed-fake.byoc.test.redpanda.com:30292"
		seedPLMtls   = "seed-fake.byoc.test.redpanda.com:30294"
		proxySasl    = "https://pandaproxy-fake.byoc.test.redpanda.com:443"
		proxyMtls    = "https://pandaproxy-fake.byoc.test.redpanda.com:8082"
		proxyPLSasl  = "https://pandaproxy-fake.byoc.test.redpanda.com:30282"
		proxyPLMtls  = "https://pandaproxy-fake.byoc.test.redpanda.com:30284"
		schemaSasl   = "https://schema-registry-fake.byoc.test.redpanda.com:443"
		schemaMtls   = "https://schema-registry-fake.byoc.test.redpanda.com:8081"
		schemaPLSasl = "https://schema-registry-fake.byoc.test.redpanda.com:30081"
		schemaPLMtls = "https://schema-registry-fake.byoc.test.redpanda.com:30083"
	)

	if c.KafkaApi == nil {
		c.KafkaApi = &controlplanev1.Cluster_KafkaAPI{}
	}
	c.KafkaApi.AllSeedBrokers = &controlplanev1.SeedBrokers{
		Sasl: seedSasl,
		Mtls: seedMtls,
	}
	if pl {
		c.KafkaApi.AllSeedBrokers.PrivateLinkSasl = seedPLSasl
		c.KafkaApi.AllSeedBrokers.PrivateLinkMtls = seedPLMtls
	}

	if c.HttpProxy == nil {
		c.HttpProxy = &controlplanev1.Cluster_HTTPProxyStatus{}
	}
	c.HttpProxy.AllUrls = &controlplanev1.Endpoints{
		Sasl: proxySasl,
		Mtls: proxyMtls,
	}
	if pl {
		c.HttpProxy.AllUrls.PrivateLinkSasl = proxyPLSasl
		c.HttpProxy.AllUrls.PrivateLinkMtls = proxyPLMtls
	}

	if c.SchemaRegistry == nil {
		c.SchemaRegistry = &controlplanev1.Cluster_SchemaRegistryStatus{}
	}
	c.SchemaRegistry.AllUrls = &controlplanev1.Endpoints{
		Sasl: schemaSasl,
		Mtls: schemaMtls,
	}
	if pl {
		c.SchemaRegistry.AllUrls.PrivateLinkSasl = schemaPLSasl
		c.SchemaRegistry.AllUrls.PrivateLinkMtls = schemaPLMtls
	}
}

// fakeAwsPrivateLinkStatus returns a synthetic, populated AWS PrivateLink
// Status when the PL is enabled, nil when disabled. Tests that exercise the
// null→populated Status transition (H3: inconsistent result after apply when
// parent object's UseStateForUnknown short-circuits child modifiers) rely on
// this mirroring real control-plane behavior.
func fakeAwsPrivateLinkStatus(enabled bool) *controlplanev1.Cluster_AWSPrivateLink_Status {
	if !enabled {
		return nil
	}
	return &controlplanev1.Cluster_AWSPrivateLink_Status{
		ServiceId:                 "vpce-svc-fake",
		ServiceName:               "com.amazonaws.vpce.fake",
		ServiceState:              "Available",
		KafkaApiSeedPort:          30292,
		SchemaRegistrySeedPort:    30081,
		RedpandaProxySeedPort:     30282,
		KafkaApiNodeBasePort:      32092,
		RedpandaProxyNodeBasePort: 32082,
		ConsolePort:               9000,
	}
}

func privateLinkActive(c *controlplanev1.Cluster) bool {
	if c.GetAwsPrivateLink().GetEnabled() {
		return true
	}
	if c.GetGcpPrivateServiceConnect().GetEnabled() {
		return true
	}
	if c.GetAzurePrivateLink().GetEnabled() {
		return true
	}
	return false
}

// EchoFromServerlessClusterCreate produces a *ServerlessCluster
// mirroring the config-driven fields of a ServerlessClusterCreate
// spec, plus synthetic Computed endpoint URLs. Golden-model assumption:
// the real backend populates cluster_api_url, kafka_api endpoints,
// schema_registry URLs, dataplane_api URLs, console URLs, and
// prometheus URLs for any ready serverless cluster.
func EchoFromServerlessClusterCreate(id string, spec *controlplanev1.ServerlessClusterCreate) *controlplanev1.ServerlessCluster {
	c := &controlplanev1.ServerlessCluster{
		Id:               id,
		Name:             spec.GetName(),
		ServerlessRegion: spec.GetServerlessRegion(),
		ResourceGroupId:  spec.GetResourceGroupId(),
		State:            controlplanev1.ServerlessCluster_STATE_READY,
		Tags:             spec.GetTags(),
		NetworkingConfig: spec.GetNetworkingConfig(),
		KafkaApi: &controlplanev1.ServerlessCluster_KafkaAPI{
			SeedBrokers:        []string{"seed-scl-fake.byoc.test.redpanda.com:9092"},
			PrivateSeedBrokers: []string{"seed-scl-fake-private.byoc.test.redpanda.com:9092"},
		},
		SchemaRegistry: &controlplanev1.ServerlessCluster_SchemaRegistryStatus{
			Url:        "https://sr-scl-fake.byoc.test.redpanda.com:443",
			PrivateUrl: "https://sr-scl-fake-private.byoc.test.redpanda.com:443",
		},
		DataplaneApi: &controlplanev1.ServerlessCluster_DataplaneAPI{
			Url:        "https://dp-scl-fake.byoc.test.redpanda.com:443",
			PrivateUrl: "https://dp-scl-fake-private.byoc.test.redpanda.com:443",
		},
		Prometheus: &controlplanev1.ServerlessCluster_Prometheus{
			Url:        "https://prom-scl-fake.byoc.test.redpanda.com/api/v1/metrics",
			PrivateUrl: "https://prom-scl-fake-private.byoc.test.redpanda.com/api/v1/metrics",
		},
		ConsoleUrl:        "https://console-scl-fake.byoc.test.redpanda.com",
		ConsolePrivateUrl: "https://console-scl-fake-private.byoc.test.redpanda.com",
	}
	if spec.PrivateLinkId != nil {
		c.PrivateLinkId = spec.PrivateLinkId
	}
	return c
}

// EchoFromServerlessPrivateLinkCreate produces a *ServerlessPrivateLink
// from a Create spec, populating Computed status.aws fields. Golden-model
// assumption: the real backend fills in vpc_endpoint_service_name and
// availability_zones for any ready AWS serverless private link.
func EchoFromServerlessPrivateLinkCreate(id string, spec *controlplanev1.ServerlessPrivateLinkCreate) *controlplanev1.ServerlessPrivateLink {
	l := &controlplanev1.ServerlessPrivateLink{
		Id:               id,
		Name:             spec.GetName(),
		ResourceGroupId:  spec.GetResourceGroupId(),
		Cloudprovider:    spec.GetCloudprovider(),
		ServerlessRegion: spec.GetServerlessRegion(),
		State:            controlplanev1.ServerlessPrivateLink_STATE_READY,
	}
	if aws := spec.GetAwsConfig(); aws != nil {
		l.CloudProviderConfig = &controlplanev1.ServerlessPrivateLink_AwsConfig{
			AwsConfig: &controlplanev1.ServerlessPrivateLink_AWS{
				AllowedPrincipals: aws.GetAllowedPrincipals(),
			},
		}
	}
	l.Status = &controlplanev1.ServerlessPrivateLinkStatus{
		CloudProvider: &controlplanev1.ServerlessPrivateLinkStatus_Aws{
			Aws: &controlplanev1.ServerlessPrivateLinkStatus_AWS{
				VpcEndpointServiceName: "com.amazonaws.vpce.spl-fake",
				AvailabilityZones:      []string{"use1-az1", "use1-az2"},
			},
		},
	}
	return l
}

// EchoFromPipelineCreate produces a *Pipeline from a PipelineCreate
// spec, populating Computed url/status fields. Golden-model assumption:
// the real backend emits an HTTP URL and at least an empty Status
// object for any pipeline that's been created successfully.
func EchoFromPipelineCreate(id string, spec *dataplanev1.PipelineCreate) *dataplanev1.Pipeline {
	return &dataplanev1.Pipeline{
		Id:          id,
		DisplayName: spec.GetDisplayName(),
		Description: spec.GetDescription(),
		ConfigYaml:  spec.GetConfigYaml(),
		Resources:   spec.GetResources(),
		Tags:        spec.GetTags(),
		Url:         "https://pipeline-" + id + ".dp-fake.byoc.test.redpanda.com",
		State:       dataplanev1.Pipeline_STATE_RUNNING,
		Status:      &dataplanev1.Pipeline_Status{},
	}
}
