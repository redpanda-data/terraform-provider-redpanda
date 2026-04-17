// Copyright 2024 Redpanda Data, Inc.
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

package cluster

// Unit-level Create / Read / Update round-trip tests for the Cluster
// resource. Calls (*Cluster).Create / Read / Update directly with mocked
// gRPC service clients, asserting that every config-driven field
// round-trips through the mapper functions.

import (
	"context"
	"testing"

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1/controlplanev1grpc"
	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud/cloudtest"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/mocks"
	clustermodel "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/cluster"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
)

const fakeClusterID = cloudtest.FakeClusterID

func newMockedCluster(t *testing.T) (*Cluster, *mocks.MockClusterServiceClient, *mocks.MockOperationServiceClient) {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockCluster := mocks.NewMockClusterServiceClient(ctrl)
	mockOp := mocks.NewMockOperationServiceClient(ctrl)

	c := &Cluster{
		CpCl: &cloud.ControlPlaneClientSet{
			Cluster:   mockCluster,
			Operation: mockOp,
		},
		// Byoc intentionally nil; tests use Type=dedicated to avoid the BYOC
		// code path. If a test enters the BYOC branch, the resulting nil
		// pointer panic is the diagnostic — don't paper it over.
	}
	return c, mockCluster, mockOp
}

func expectCreateOnce(t *testing.T, m *mocks.MockClusterServiceClient, out **controlplanev1.Cluster) {
	t.Helper()
	m.EXPECT().
		CreateCluster(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req *controlplanev1.CreateClusterRequest, _ ...grpc.CallOption) (*controlplanev1.CreateClusterOperation, error) {
			echoed := simulateClusterServerEcho(req.GetCluster())
			*out = echoed
			return &controlplanev1.CreateClusterOperation{
				Operation: &controlplanev1.Operation{
					Id:         "op-create-fake",
					ResourceId: stringPtr(echoed.GetId()),
					State:      controlplanev1.Operation_STATE_COMPLETED,
				},
			}, nil
		}).
		Times(1)
}

func expectGetClusterAlways(t *testing.T, m *mocks.MockClusterServiceClient, out **controlplanev1.Cluster) {
	t.Helper()
	m.EXPECT().
		GetCluster(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ *controlplanev1.GetClusterRequest, _ ...grpc.CallOption) (*controlplanev1.GetClusterResponse, error) {
			return &controlplanev1.GetClusterResponse{Cluster: *out}, nil
		}).
		AnyTimes()
}

// expectUpdateOnce makes UpdateCluster swap *out for postUpdate and
// return a completed operation. Pair with expectGetOperationCompleted so
// the AreWeDoneYet polling loop exits.
func expectUpdateOnce(t *testing.T, m *mocks.MockClusterServiceClient, out **controlplanev1.Cluster, postUpdate *controlplanev1.Cluster) {
	t.Helper()
	m.EXPECT().
		UpdateCluster(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ *controlplanev1.UpdateClusterRequest, _ ...grpc.CallOption) (*controlplanev1.UpdateClusterOperation, error) {
			*out = postUpdate
			return &controlplanev1.UpdateClusterOperation{
				Operation: &controlplanev1.Operation{
					Id:         "op-update-fake",
					ResourceId: stringPtr(postUpdate.GetId()),
					State:      controlplanev1.Operation_STATE_COMPLETED,
				},
			}, nil
		}).
		Times(1)
}

func expectGetOperationCompleted(t *testing.T, m *mocks.MockOperationServiceClient) {
	t.Helper()
	m.EXPECT().
		GetOperation(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req *controlplanev1.GetOperationRequest, _ ...grpc.CallOption) (*controlplanev1.GetOperationResponse, error) {
			return &controlplanev1.GetOperationResponse{
				Operation: &controlplanev1.Operation{
					Id:    req.GetId(),
					State: controlplanev1.Operation_STATE_COMPLETED,
				},
			}, nil
		}).
		AnyTimes()
}

func stringPtr(s string) *string { return &s }

func simulateClusterServerEcho(req *controlplanev1.ClusterCreate) *controlplanev1.Cluster {
	return cloudtest.EchoFromClusterCreate(req)
}

// timeoutsAttrTypes mirrors the schema's timeouts attribute. Required so
// tfsdk.Plan.Set's type check accepts a typed-null timeouts.Value built
// against this shape.
var timeoutsAttrTypes = map[string]attr.Type{
	"create": types.StringType,
	"delete": types.StringType,
	"update": types.StringType,
}

func nullTimeouts() timeouts.Value {
	return timeouts.Value{Object: types.ObjectNull(timeoutsAttrTypes)}
}

func baselineClusterPlan() *clustermodel.ResourceModel {
	m := clustermodel.GenerateMinimalResourceModel("", nullTimeouts())
	m.ID = types.StringNull()
	m.Name = types.StringValue("test-cluster")
	m.CloudProvider = types.StringValue("aws")
	m.ClusterType = types.StringValue("dedicated")
	m.ConnectionType = types.StringValue("public")
	m.ThroughputTier = types.StringValue("tier-1-aws-v2-arm")
	m.Region = types.StringValue("us-east-1")
	m.ResourceGroupID = types.StringValue("rg-test")
	m.NetworkID = types.StringValue("net-test")
	m.RedpandaVersion = types.StringValue("v24.1.1")
	m.AllowDeletion = types.BoolValue(true)
	m.Zones = types.ListValueMust(types.StringType, []attr.Value{types.StringValue("use1-az1")})
	return m
}

func echoFromPlan(t *testing.T, plan *clustermodel.ResourceModel) *controlplanev1.Cluster {
	t.Helper()
	spec, diags := plan.GetClusterCreate(context.Background())
	require.False(t, diags.HasError(), "GetClusterCreate: %v", diags)
	return simulateClusterServerEcho(spec)
}

// modelAttrTypes caches the attribute-type maps for the cluster
// ResourceModel's nested object attributes. Built once at init from a
// baseline ResourceModel so tests can construct typed-null nested objects
// without reaching into the model package's unexported helpers.
type modelAttrTypes struct {
	awsPrivateLink          map[string]attr.Type
	awsPrivateLinkStatus    map[string]attr.Type
	gcpPrivateServiceConn   map[string]attr.Type
	azurePrivateLink        map[string]attr.Type
	kafkaAPI                map[string]attr.Type
	httpProxy               map[string]attr.Type
	schemaRegistry          map[string]attr.Type
	mtls                    map[string]attr.Type
	sasl                    map[string]attr.Type
	allSeedBrokers          map[string]attr.Type
	allUrls                 map[string]attr.Type
	kafkaConnect            map[string]attr.Type
	maintenanceWindow       map[string]attr.Type
	dayHour                 map[string]attr.Type
	clusterConfiguration    map[string]attr.Type
	prometheus              map[string]attr.Type
	redpandaConsole         map[string]attr.Type
	cloudStorage            map[string]attr.Type
	stateDescription        map[string]attr.Type
	customerManagedResource map[string]attr.Type
}

var attrTypes = func() *modelAttrTypes {
	base := clustermodel.GenerateMinimalResourceModel("", timeouts.Value{})
	t := &modelAttrTypes{
		awsPrivateLink:          base.AWSPrivateLink.AttributeTypes(context.Background()),
		gcpPrivateServiceConn:   base.GCPPrivateServiceConnect.AttributeTypes(context.Background()),
		azurePrivateLink:        base.AzurePrivateLink.AttributeTypes(context.Background()),
		kafkaAPI:                base.KafkaAPI.AttributeTypes(context.Background()),
		httpProxy:               base.HTTPProxy.AttributeTypes(context.Background()),
		schemaRegistry:          base.SchemaRegistry.AttributeTypes(context.Background()),
		kafkaConnect:            base.KafkaConnect.AttributeTypes(context.Background()),
		maintenanceWindow:       base.MaintenanceWindowConfig.AttributeTypes(context.Background()),
		clusterConfiguration:    base.ClusterConfiguration.AttributeTypes(context.Background()),
		prometheus:              base.Prometheus.AttributeTypes(context.Background()),
		redpandaConsole:         base.RedpandaConsole.AttributeTypes(context.Background()),
		cloudStorage:            base.CloudStorage.AttributeTypes(context.Background()),
		stateDescription:        base.StateDescription.AttributeTypes(context.Background()),
		customerManagedResource: base.CustomerManagedResources.AttributeTypes(context.Background()),
	}
	t.awsPrivateLinkStatus = t.awsPrivateLink["status"].(types.ObjectType).AttrTypes
	t.mtls = t.kafkaAPI["mtls"].(types.ObjectType).AttrTypes
	t.sasl = t.kafkaAPI["sasl"].(types.ObjectType).AttrTypes
	t.allSeedBrokers = t.kafkaAPI["all_seed_brokers"].(types.ObjectType).AttrTypes
	t.allUrls = t.httpProxy["all_urls"].(types.ObjectType).AttrTypes
	t.dayHour = t.maintenanceWindow["day_hour"].(types.ObjectType).AttrTypes
	return t
}()

func schemaForCluster(t *testing.T, c *Cluster) schema.Schema {
	t.Helper()
	var sr resource.SchemaResponse
	c.Schema(context.Background(), resource.SchemaRequest{}, &sr)
	require.False(t, sr.Diagnostics.HasError(), "Schema diagnostics: %v", sr.Diagnostics)
	return sr.Schema
}

func buildPlan(t *testing.T, c *Cluster, model *clustermodel.ResourceModel) tfsdk.Plan {
	t.Helper()
	s := schemaForCluster(t, c)
	plan := tfsdk.Plan{Schema: s}
	diags := plan.Set(context.Background(), model)
	require.False(t, diags.HasError(), "Plan.Set: %v", diags)
	return plan
}

func buildState(t *testing.T, c *Cluster, model *clustermodel.ResourceModel) tfsdk.State {
	t.Helper()
	s := schemaForCluster(t, c)
	state := tfsdk.State{Schema: s}
	diags := state.Set(context.Background(), model)
	require.False(t, diags.HasError(), "State.Set: %v", diags)
	return state
}

func buildNullState(t *testing.T, c *Cluster) tfsdk.State {
	t.Helper()
	s := schemaForCluster(t, c)
	return tfsdk.State{
		Schema: s,
		Raw:    tftypes.NewValue(s.Type().TerraformType(context.Background()), nil),
	}
}

// buildConfig mirrors plan into a Config. Required because the resource's
// Create / Update read GCPGlobalAccessEnabled from req.Config.
func buildConfig(t *testing.T, c *Cluster, plan tfsdk.Plan) tfsdk.Config {
	t.Helper()
	s := schemaForCluster(t, c)
	return tfsdk.Config{Schema: s, Raw: plan.Raw}
}

func runCreate(t *testing.T, c *Cluster, mockSvc *mocks.MockClusterServiceClient, plan *clustermodel.ResourceModel) *clustermodel.ResourceModel {
	t.Helper()
	ctx := context.Background()

	var echoed *controlplanev1.Cluster
	expectCreateOnce(t, mockSvc, &echoed)
	expectGetClusterAlways(t, mockSvc, &echoed)

	tfPlan := buildPlan(t, c, plan)
	resp := &resource.CreateResponse{State: buildNullState(t, c)}
	c.Create(ctx, resource.CreateRequest{
		Plan:   tfPlan,
		Config: buildConfig(t, c, tfPlan),
	}, resp)
	require.False(t, resp.Diagnostics.HasError(), "Create diagnostics: %v", resp.Diagnostics)

	state := &clustermodel.ResourceModel{}
	diags := resp.State.Get(ctx, state)
	require.False(t, diags.HasError(), "State.Get: %v", diags)
	return state
}

func runRead(t *testing.T, c *Cluster, mockSvc *mocks.MockClusterServiceClient, priorState *clustermodel.ResourceModel, echoedCluster *controlplanev1.Cluster) *clustermodel.ResourceModel {
	t.Helper()
	ctx := context.Background()

	echoedRef := echoedCluster
	expectGetClusterAlways(t, mockSvc, &echoedRef)

	resp := &resource.ReadResponse{State: buildState(t, c, priorState)}
	c.Read(ctx, resource.ReadRequest{State: buildState(t, c, priorState)}, resp)
	require.False(t, resp.Diagnostics.HasError(), "Read diagnostics: %v", resp.Diagnostics)

	if resp.State.Raw.IsNull() {
		// Resource was removed (e.g. cluster gone). Caller asserts.
		return nil
	}
	state := &clustermodel.ResourceModel{}
	diags := resp.State.Get(ctx, state)
	require.False(t, diags.HasError(), "State.Get: %v", diags)
	return state
}

// runUpdate drives (*Cluster).Update. When plan equals priorState the
// resource skips the UpdateCluster RPC, so passing postEchoed == priorEchoed
// is valid for that case. expectEmptyMask asserts the computed mask matches
// the caller's expectation — passing true on a mutation that should produce
// a real Update would silently degrade the test to a baseline-vs-baseline
// no-op (see H5 in the production-readiness review).
func runUpdate(t *testing.T, c *Cluster, mockSvc *mocks.MockClusterServiceClient, mockOp *mocks.MockOperationServiceClient, plan, priorState *clustermodel.ResourceModel, priorEchoed, postEchoed *controlplanev1.Cluster, expectEmptyMask bool) *clustermodel.ResourceModel {
	t.Helper()
	ctx := context.Background()

	// Generate the update mask up-front to decide whether the framework will
	// invoke UpdateCluster. Mirrors the gate at resource_cluster.go:215.
	updateReq, diags := plan.GetClusterUpdateRequest(ctx, priorState)
	require.False(t, diags.HasError(), "GetClusterUpdateRequest: %v", diags)

	mask := updateReq.GetUpdateMask().GetPaths()
	require.Equal(t, expectEmptyMask, len(mask) == 0,
		"update-mask invariant: expectEmptyMask=%v but actual paths=%v", expectEmptyMask, mask)

	echoedRef := priorEchoed
	if len(mask) != 0 {
		expectUpdateOnce(t, mockSvc, &echoedRef, postEchoed)
		expectGetOperationCompleted(t, mockOp)
	}
	expectGetClusterAlways(t, mockSvc, &echoedRef)

	tfPlan := buildPlan(t, c, plan)
	tfState := buildState(t, c, priorState)
	resp := &resource.UpdateResponse{State: tfState}

	c.Update(ctx, resource.UpdateRequest{
		Plan:   tfPlan,
		State:  tfState,
		Config: buildConfig(t, c, tfPlan),
	}, resp)
	require.False(t, resp.Diagnostics.HasError(), "Update diagnostics: %v", resp.Diagnostics)

	state := &clustermodel.ResourceModel{}
	diags = resp.State.Get(ctx, state)
	require.False(t, diags.HasError(), "State.Get: %v", diags)
	return state
}

var (
	_ controlplanev1grpc.ClusterServiceClient   = (*mocks.MockClusterServiceClient)(nil)
	_ controlplanev1grpc.OperationServiceClient = (*mocks.MockOperationServiceClient)(nil)
)

// roundTripCase is one configuration in the shared Create / Read / Update
// matrix. skipReason, when non-empty, marks the case as known-broken so
// the suite stays green while documenting the bug. expectEmptyMaskOnUpdate
// opts a case in to "the from-baseline Update produces an empty mask"
// — set true only when the mutation is wire-equivalent to no change
// (e.g. kafka_connect.enabled=false, see C2 in resource_model.go:1421).
type roundTripCase struct {
	name                    string
	mutate                  func(*clustermodel.ResourceModel)
	skipReason              string
	expectEmptyMaskOnUpdate bool
}

// configDrivenFields lists the ResourceModel fields the round-trip
// assertion compares. Computed-only fields are excluded — they can vary
// freely between plan and state.
var configDrivenFields = []struct {
	name string
	get  func(*clustermodel.ResourceModel) attr.Value
}{
	{"Name", func(m *clustermodel.ResourceModel) attr.Value { return m.Name }},
	{"CloudProvider", func(m *clustermodel.ResourceModel) attr.Value { return m.CloudProvider }},
	{"ClusterType", func(m *clustermodel.ResourceModel) attr.Value { return m.ClusterType }},
	{"ConnectionType", func(m *clustermodel.ResourceModel) attr.Value { return m.ConnectionType }},
	{"ThroughputTier", func(m *clustermodel.ResourceModel) attr.Value { return m.ThroughputTier }},
	{"Region", func(m *clustermodel.ResourceModel) attr.Value { return m.Region }},
	{"ResourceGroupID", func(m *clustermodel.ResourceModel) attr.Value { return m.ResourceGroupID }},
	{"NetworkID", func(m *clustermodel.ResourceModel) attr.Value { return m.NetworkID }},
	{"Zones", func(m *clustermodel.ResourceModel) attr.Value { return m.Zones }},
	{"AWSPrivateLink", func(m *clustermodel.ResourceModel) attr.Value { return m.AWSPrivateLink }},
	{"GCPPrivateServiceConnect", func(m *clustermodel.ResourceModel) attr.Value { return m.GCPPrivateServiceConnect }},
	{"AzurePrivateLink", func(m *clustermodel.ResourceModel) attr.Value { return m.AzurePrivateLink }},
	{"KafkaAPI", func(m *clustermodel.ResourceModel) attr.Value { return m.KafkaAPI }},
	{"HTTPProxy", func(m *clustermodel.ResourceModel) attr.Value { return m.HTTPProxy }},
	{"SchemaRegistry", func(m *clustermodel.ResourceModel) attr.Value { return m.SchemaRegistry }},
	{"KafkaConnect", func(m *clustermodel.ResourceModel) attr.Value { return m.KafkaConnect }},
	{"MaintenanceWindowConfig", func(m *clustermodel.ResourceModel) attr.Value { return m.MaintenanceWindowConfig }},
	{"ClusterConfiguration", func(m *clustermodel.ResourceModel) attr.Value { return m.ClusterConfiguration }},
}

// assertConfigDrivenRoundTrip compares plan and state field-by-field.
// Fields that are null in plan are skipped: this harness models "user
// didn't configure" as null, but real Terraform models it as Unknown,
// which the framework lets the provider satisfy with any concrete value.
// The acceptance suite covers the Unknown-vs-known plan-modifier paths.
func assertConfigDrivenRoundTrip(t *testing.T, plan, state *clustermodel.ResourceModel) {
	t.Helper()
	for _, f := range configDrivenFields {
		t.Run(f.name, func(t *testing.T) {
			pv := f.get(plan)
			sv := f.get(state)
			if pv.IsNull() {
				return
			}
			require.True(t, pv.Equal(sv),
				"%s should round-trip\n  plan:  %v\n  state: %v", f.name, pv, sv)
		})
	}
}

// roundTripCases is the shared matrix of cluster configurations the
// Create / Read / Update suites iterate over. Add a new entry to extend
// coverage for a new schema sub-tree.
func roundTripCases() []roundTripCase {
	return []roundTripCase{
		{
			name:                    "minimal_aws_dedicated_public",
			mutate:                  func(*clustermodel.ResourceModel) {},
			expectEmptyMaskOnUpdate: true, // no mutation -> identical to baseline
		},
		{
			name: "aws_private_link_disabled_block",
			mutate: func(m *clustermodel.ResourceModel) {
				m.ConnectionType = types.StringValue("private")
				m.AWSPrivateLink = mustObject(attrTypes.awsPrivateLink, map[string]attr.Value{
					"enabled":            types.BoolValue(false),
					"connect_console":    types.BoolValue(false),
					"allowed_principals": types.ListValueMust(types.StringType, []attr.Value{}),
					"status":             types.ObjectNull(attrTypes.awsPrivateLinkStatus),
					"supported_regions":  types.ListValueMust(types.StringType, []attr.Value{}),
				})
			},
		},
		{
			name: "aws_private_link_enabled_block",
			mutate: func(m *clustermodel.ResourceModel) {
				m.ConnectionType = types.StringValue("private")
				m.AWSPrivateLink = mustObject(attrTypes.awsPrivateLink, map[string]attr.Value{
					"enabled":         types.BoolValue(true),
					"connect_console": types.BoolValue(true),
					"allowed_principals": types.ListValueMust(types.StringType, []attr.Value{
						types.StringValue("arn:aws:iam::123456789012:root"),
					}),
					"status":            expectedFakeAwsPrivateLinkStatus(),
					"supported_regions": types.ListValueMust(types.StringType, []attr.Value{}),
				})
			},
		},
		{
			name: "kafka_connect_disabled",
			mutate: func(m *clustermodel.ResourceModel) {
				m.KafkaConnect = mustObject(attrTypes.kafkaConnect, map[string]attr.Value{
					"enabled": types.BoolValue(false),
				})
			},
			// kafka_connect={enabled:false} is wire-equivalent to "unset" on
			// proto3, so the reflection-based mask diff sees no change vs
			// the baseline which has kafka_connect=null. The C2 fix in
			// resource_model.go:1421 preserves the user's plan presence
			// in state despite the empty mask.
			expectEmptyMaskOnUpdate: true,
		},
		{
			name: "kafka_connect_enabled",
			mutate: func(m *clustermodel.ResourceModel) {
				m.KafkaConnect = mustObject(attrTypes.kafkaConnect, map[string]attr.Value{
					"enabled": types.BoolValue(true),
				})
			},
		},
		{
			name: "maintenance_window_anytime",
			mutate: func(m *clustermodel.ResourceModel) {
				m.MaintenanceWindowConfig = mustObject(attrTypes.maintenanceWindow, map[string]attr.Value{
					"day_hour":    types.ObjectNull(attrTypes.dayHour),
					"anytime":     types.BoolValue(true),
					"unspecified": types.BoolNull(),
				})
			},
		},
		{
			name: "maintenance_window_unspecified",
			mutate: func(m *clustermodel.ResourceModel) {
				m.MaintenanceWindowConfig = mustObject(attrTypes.maintenanceWindow, map[string]attr.Value{
					"day_hour":    types.ObjectNull(attrTypes.dayHour),
					"anytime":     types.BoolNull(),
					"unspecified": types.BoolValue(true),
				})
			},
		},
		{
			name: "maintenance_window_day_hour",
			mutate: func(m *clustermodel.ResourceModel) {
				dh := mustObject(attrTypes.dayHour, map[string]attr.Value{
					"hour_of_day": types.Int32Value(14),
					"day_of_week": types.StringValue("MONDAY"),
				})
				m.MaintenanceWindowConfig = mustObject(attrTypes.maintenanceWindow, map[string]attr.Value{
					"day_hour":    dh,
					"anytime":     types.BoolNull(),
					"unspecified": types.BoolNull(),
				})
			},
		},
	}
}

func mustObject(attrTypes map[string]attr.Type, attrValues map[string]attr.Value) types.Object {
	o, diags := types.ObjectValue(attrTypes, attrValues)
	if diags.HasError() {
		panic(diags)
	}
	return o
}

// expectedFakeAwsPrivateLinkStatus mirrors the synthetic Status produced by
// cloudtest.fakeAwsPrivateLinkStatus. Keep in lockstep with the fake.
func expectedFakeAwsPrivateLinkStatus() types.Object {
	return mustObject(attrTypes.awsPrivateLinkStatus, map[string]attr.Value{
		"service_id":                    types.StringValue("vpce-svc-fake"),
		"service_name":                  types.StringValue("com.amazonaws.vpce.fake"),
		"service_state":                 types.StringValue("Available"),
		"kafka_api_seed_port":           types.Int32Value(30292),
		"schema_registry_seed_port":     types.Int32Value(30081),
		"redpanda_proxy_seed_port":      types.Int32Value(30282),
		"kafka_api_node_base_port":      types.Int32Value(32092),
		"redpanda_proxy_node_base_port": types.Int32Value(32082),
		"console_port":                  types.Int32Value(9000),
		"vpc_endpoint_connections":      types.ListValueMust(attrTypes.awsPrivateLinkStatus["vpc_endpoint_connections"].(types.ListType).ElemType, []attr.Value{}),
	})
}

func TestCluster_Create_RoundTrip(t *testing.T) {
	for _, tc := range roundTripCases() {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skipReason != "" {
				t.Skip(tc.skipReason)
			}
			c, mockSvc, _ := newMockedCluster(t)
			plan := baselineClusterPlan()
			tc.mutate(plan)

			state := runCreate(t, c, mockSvc, plan)
			assertConfigDrivenRoundTrip(t, plan, state)
		})
	}
}

func TestCluster_Read_RoundTrip(t *testing.T) {
	for _, tc := range roundTripCases() {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skipReason != "" {
				t.Skip(tc.skipReason)
			}
			c, mockSvc, _ := newMockedCluster(t)
			plan := baselineClusterPlan()
			tc.mutate(plan)
			plan.ID = types.StringValue(fakeClusterID)

			echoed := echoFromPlan(t, plan)
			state := runRead(t, c, mockSvc, plan, echoed)
			require.NotNil(t, state, "Read should not have removed the resource")

			assertConfigDrivenRoundTrip(t, plan, state)
		})
	}
}

// TestCluster_Update_NoOpRoundTrip drives Update with plan == prior state.
// The cluster resource short-circuits past UpdateCluster on an empty mask,
// so this only exercises the post-Update Read mapping.
func TestCluster_Update_NoOpRoundTrip(t *testing.T) {
	for _, tc := range roundTripCases() {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skipReason != "" {
				t.Skip(tc.skipReason)
			}
			c, mockSvc, mockOp := newMockedCluster(t)
			plan := baselineClusterPlan()
			tc.mutate(plan)
			plan.ID = types.StringValue(fakeClusterID)

			priorEchoed := echoFromPlan(t, plan)
			// plan == priorState -> mask is always empty in this suite.
			state := runUpdate(t, c, mockSvc, mockOp, plan, plan, priorEchoed, priorEchoed, true)

			assertConfigDrivenRoundTrip(t, plan, state)
		})
	}
}

// TestCluster_Update_FromBaselineRoundTrip drives Update with a real diff
// (prior = baseline, plan = mutated). Some mutations land on fields the
// update-mask builder considers immutable; for those the resource skips
// UpdateCluster and the assertion still verifies the no-diff Read agrees.
func TestCluster_Update_FromBaselineRoundTrip(t *testing.T) {
	for _, tc := range roundTripCases() {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skipReason != "" {
				t.Skip(tc.skipReason)
			}
			c, mockSvc, mockOp := newMockedCluster(t)

			priorState := baselineClusterPlan()
			priorState.ID = types.StringValue(fakeClusterID)

			plan := baselineClusterPlan()
			tc.mutate(plan)
			plan.ID = types.StringValue(fakeClusterID)

			priorEchoed := echoFromPlan(t, priorState)
			postEchoed := echoFromPlan(t, plan)
			state := runUpdate(t, c, mockSvc, mockOp, plan, priorState, priorEchoed, postEchoed, tc.expectEmptyMaskOnUpdate)

			assertConfigDrivenRoundTrip(t, plan, state)
		})
	}
}

// TestCluster_Update_AwsPrivateLinkToggleRoundTrip drives an Update that
// toggles aws_private_link.enabled from true to false. Catches mapper
// asymmetries on the toggle path.
func TestCluster_Update_AwsPrivateLinkToggleRoundTrip(t *testing.T) {
	enabled := func(m *clustermodel.ResourceModel) {
		m.ConnectionType = types.StringValue("private")
		m.AWSPrivateLink = mustObject(attrTypes.awsPrivateLink, map[string]attr.Value{
			"enabled":         types.BoolValue(true),
			"connect_console": types.BoolValue(true),
			"allowed_principals": types.ListValueMust(types.StringType, []attr.Value{
				types.StringValue("arn:aws:iam::123456789012:root"),
			}),
			"status":            types.ObjectNull(attrTypes.awsPrivateLinkStatus),
			"supported_regions": types.ListValueMust(types.StringType, []attr.Value{}),
		})
	}
	disabled := func(m *clustermodel.ResourceModel) {
		m.ConnectionType = types.StringValue("private")
		m.AWSPrivateLink = mustObject(attrTypes.awsPrivateLink, map[string]attr.Value{
			"enabled":            types.BoolValue(false),
			"connect_console":    types.BoolValue(false),
			"allowed_principals": types.ListValueMust(types.StringType, []attr.Value{}),
			"status":             types.ObjectNull(attrTypes.awsPrivateLinkStatus),
			"supported_regions":  types.ListValueMust(types.StringType, []attr.Value{}),
		})
	}

	c, mockSvc, mockOp := newMockedCluster(t)

	priorState := baselineClusterPlan()
	enabled(priorState)
	priorState.ID = types.StringValue(fakeClusterID)

	plan := baselineClusterPlan()
	disabled(plan)
	plan.ID = types.StringValue(fakeClusterID)

	priorEchoed := echoFromPlan(t, priorState)
	postEchoed := echoFromPlan(t, plan)

	// AWS PL toggle enabled→disabled produces a non-empty mask via the
	// aws_private_link path.
	state := runUpdate(t, c, mockSvc, mockOp, plan, priorState, priorEchoed, postEchoed, false)
	assertConfigDrivenRoundTrip(t, plan, state)
}
