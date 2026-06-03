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

// Package mock provides the bufconn-backed gRPC fake server that powers the
// TestIntegration_* acceptance tier. Tests construct a fresh Server per test, wire
// its Dialer() into the provider factory, and exercise the real
// terraform-plugin-framework provider against in-memory fakes.
package mock

import (
	"context"
	"net"
	"sort"
	"sync"
	"testing"

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1/controlplanev1grpc"
	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1beta2/controlplanev1beta2grpc"
	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/iam/v1/iamv1grpc"
	"buf.build/gen/go/redpandadata/dataplane/grpc/go/redpanda/api/console/v1alpha1/consolev1alpha1grpc"
	"buf.build/gen/go/redpandadata/dataplane/grpc/go/redpanda/api/dataplane/v1/dataplanev1grpc"
	"buf.build/go/protovalidate"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/mock/fakes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
)

const bufSize = 1024 * 1024

// Server is the in-memory mock backend for a single TestIntegration_* test. The
// zero value is not usable; call New.
type Server struct {
	lis              *bufconn.Listener
	grpc             *grpc.Server
	mu               sync.Mutex
	pendingOverrides map[string]error
	callCounts       map[string]int

	// ResourceGroup is the stateful fake for the ResourceGroupService RPCs.
	// Tests may read its store or invoke OverrideOnce on its method names.
	ResourceGroup *fakes.ResourceGroupFake

	// ACL is the stateful fake for the dataplane ACLService RPCs.
	ACL *fakes.ACLFake

	// User is the stateful fake for the dataplane UserService RPCs.
	User *fakes.UserFake

	// Secret is the stateful fake for the dataplane SecretService RPCs.
	Secret *fakes.SecretFake

	// Security is the stateful fake for the console SecurityService RPCs.
	// Backs both redpanda_role and redpanda_role_assignment.
	Security *fakes.SecurityFake

	// ServiceAccount is the stateful fake for the IAM ServiceAccountService
	// RPCs. Models the write-only client_secret contract.
	ServiceAccount *fakes.ServiceAccountFake

	// Topic is the stateful fake for the dataplane TopicService RPCs.
	Topic *fakes.TopicFake

	// Pipeline is the stateful fake for the dataplane PipelineService RPCs.
	Pipeline *fakes.PipelineFake

	// Operation backs the OperationService polled by AreWeDoneYet for async
	// controlplane mutations on shadow_link, serverless_private_link,
	// serverless_cluster, network, and cluster Update. (cluster Create/Delete
	// poll cluster state directly via RetryGetCluster and do not call
	// Operation.Set.) Async fakes call Set to publish their operation state;
	// the provider's polling loop reads via GetOperation.
	Operation *fakes.OperationFake

	// Region is the read-only fake for RegionService. Backs both the
	// redpanda_region and redpanda_regions datasources; seeded with AWS + GCP.
	Region *fakes.RegionFake

	// ServerlessRegion is the read-only fake for ServerlessRegionService.
	// Backs the redpanda_serverless_regions datasource; seeded with AWS + GCP.
	ServerlessRegion *fakes.ServerlessRegionFake

	// ThroughputTier is the read-only fake for the controlplane v1beta2
	// ThroughputTierService. Backs the redpanda_throughput_tiers datasource;
	// seeded with AWS + GCP tiers.
	ThroughputTier *fakes.ThroughputTierFake

	// Network is the async fake for NetworkService. Create + Delete publish
	// completed Operations via Operation.Set; no Update RPC (schema marks
	// every configurable field RequiresReplace).
	Network *fakes.NetworkFake

	// ServerlessPrivateLink is the async fake for ServerlessPrivateLinkService.
	// All three mutating RPCs publish completed Operations; Update has no
	// FieldMask (full replacement of the aws_config oneof variant).
	ServerlessPrivateLink *fakes.ServerlessPrivateLinkFake

	// ServerlessCluster is the async fake for ServerlessClusterService. All
	// three mutating RPCs publish completed Operations; Update has no FieldMask
	// (networking_config, tags, private_link_id are full-replacement
	// top-level fields).
	ServerlessCluster *fakes.ServerlessClusterFake

	// ShadowLink is the async fake for ShadowLinkService. All three mutating
	// RPCs publish completed Operations; UpdateShadowLink honors the
	// FieldMask via proto reflection (top-level paths only — matches what
	// utils.GenerateProtobufDiffAndUpdateMask emits).
	ShadowLink *fakes.ShadowLinkFake

	// Cluster is the async fake for ClusterService. CreateCluster and
	// DeleteCluster diverge from the other async fakes — the provider uses
	// RetryGetCluster (polling GetCluster), not AreWeDoneYet, for completion
	// detection. Only UpdateCluster goes through AreWeDoneYet. dataplane_api.url
	// is populated with the "bufnet" sentinel.
	Cluster *fakes.ClusterFake

	// SR is the httptest-backed Schema Registry + ACL fake. Backs both
	// redpanda_schema and redpanda_schema_registry_acl via REST over HTTP.
	// Not registered as a gRPC service; mounted on its own httptest.Server
	// and exposed to the provider via cluster.SchemaRegistry.Url.
	SR *fakes.SchemaRegistryFake
}

// New constructs a Server, registers the fakes, starts the gRPC server on a
// bufconn listener, and arranges teardown via t.Cleanup.
func New(t testing.TB) *Server {
	t.Helper()
	v, err := protovalidate.New()
	if err != nil {
		t.Fatalf("protovalidate.New: %v", err)
	}
	opFake := fakes.NewOperationFake()
	s := &Server{
		lis:                   bufconn.Listen(bufSize),
		pendingOverrides:      map[string]error{},
		callCounts:            map[string]int{},
		ResourceGroup:         fakes.NewResourceGroupFake(),
		ACL:                   fakes.NewACLFake(),
		User:                  fakes.NewUserFake(),
		Secret:                fakes.NewSecretFake(),
		Security:              fakes.NewSecurityFake(),
		ServiceAccount:        fakes.NewServiceAccountFake(),
		Topic:                 fakes.NewTopicFake(),
		Pipeline:              fakes.NewPipelineFake(),
		Operation:             opFake,
		Region:                fakes.NewRegionFake(),
		ServerlessRegion:      fakes.NewServerlessRegionFake(),
		ThroughputTier:        fakes.NewThroughputTierFake(),
		Network:               fakes.NewNetworkFake(opFake),
		ServerlessPrivateLink: fakes.NewServerlessPrivateLinkFake(opFake),
		ServerlessCluster:     fakes.NewServerlessClusterFake(opFake),
		ShadowLink:            fakes.NewShadowLinkFake(opFake),
		Cluster:               fakes.NewClusterFake(opFake),
		SR:                    fakes.NewSchemaRegistryFake(t),
	}
	s.Cluster.SetSchemaRegistryURL(s.SR.BaseURL())
	s.grpc = grpc.NewServer(grpc.ChainUnaryInterceptor(
		s.countingInterceptor(),
		s.overrideInterceptor(),
		validatingInterceptor(v),
	))
	controlplanev1grpc.RegisterResourceGroupServiceServer(s.grpc, s.ResourceGroup)
	dataplanev1grpc.RegisterACLServiceServer(s.grpc, s.ACL)
	dataplanev1grpc.RegisterUserServiceServer(s.grpc, s.User)
	dataplanev1grpc.RegisterSecretServiceServer(s.grpc, s.Secret)
	consolev1alpha1grpc.RegisterSecurityServiceServer(s.grpc, s.Security)
	iamv1grpc.RegisterServiceAccountServiceServer(s.grpc, s.ServiceAccount)
	dataplanev1grpc.RegisterTopicServiceServer(s.grpc, s.Topic)
	dataplanev1grpc.RegisterPipelineServiceServer(s.grpc, s.Pipeline)
	controlplanev1grpc.RegisterOperationServiceServer(s.grpc, s.Operation)
	controlplanev1grpc.RegisterRegionServiceServer(s.grpc, s.Region)
	controlplanev1grpc.RegisterServerlessRegionServiceServer(s.grpc, s.ServerlessRegion)
	controlplanev1beta2grpc.RegisterThroughputTierServiceServer(s.grpc, s.ThroughputTier)
	controlplanev1grpc.RegisterNetworkServiceServer(s.grpc, s.Network)
	controlplanev1grpc.RegisterServerlessPrivateLinkServiceServer(s.grpc, s.ServerlessPrivateLink)
	controlplanev1grpc.RegisterServerlessClusterServiceServer(s.grpc, s.ServerlessCluster)
	controlplanev1grpc.RegisterShadowLinkServiceServer(s.grpc, s.ShadowLink)
	controlplanev1grpc.RegisterClusterServiceServer(s.grpc, s.Cluster)

	go func() {
		_ = s.grpc.Serve(s.lis)
	}()
	t.Cleanup(func() {
		s.grpc.GracefulStop()
		_ = s.lis.Close()
	})
	t.Cleanup(func() {
		s.mu.Lock()
		leftover := make([]string, 0, len(s.pendingOverrides))
		for k := range s.pendingOverrides {
			leftover = append(leftover, k)
		}
		s.mu.Unlock()
		if len(leftover) > 0 {
			sort.Strings(leftover)
			t.Errorf("Server: %d OverrideOnce entry(ies) registered but never consumed (likely method name typo): %v", len(leftover), leftover)
		}
	})
	return s
}

// Dialer returns the grpc.DialOption slice that routes a *grpc.ClientConn
// through the in-memory bufconn listener. Pass to redpanda.WithDialer.
func (s *Server) Dialer() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return s.lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
}

// validatingInterceptor returns a server unary interceptor that runs
// protovalidate against every incoming request, returning InvalidArgument on
// violation. This buys the integration tier free coverage of every
// buf.validate.field / buf.validate.message rule baked into the proto
// descriptors.
func validatingInterceptor(v protovalidate.Validator) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if pm, ok := req.(proto.Message); ok {
			if err := v.Validate(pm); err != nil {
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
		}
		return handler(ctx, req)
	}
}
