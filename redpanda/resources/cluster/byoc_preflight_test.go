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

package cluster

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1/controlplanev1grpc"
	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"google.golang.org/grpc"
)

// fakeClusterService answers GetCluster with one cluster and records whether
// DeleteCluster was reached. The embedded nil interface panics on any other
// call, which withoutControlPlane turns into a test failure.
type fakeClusterService struct {
	controlplanev1grpc.ClusterServiceClient
	cluster *controlplanev1.Cluster
	deleted bool
}

func (f *fakeClusterService) GetCluster(context.Context, *controlplanev1.GetClusterRequest, ...grpc.CallOption) (*controlplanev1.GetClusterResponse, error) {
	return &controlplanev1.GetClusterResponse{Cluster: f.cluster}, nil
}

func (f *fakeClusterService) DeleteCluster(context.Context, *controlplanev1.DeleteClusterRequest, ...grpc.CallOption) (*controlplanev1.DeleteClusterOperation, error) {
	f.deleted = true
	return nil, errors.New("delete reached the control plane")
}

// azureByocCluster is the control plane's view of an Azure BYOC cluster whose
// create did not finish.
func azureByocCluster() *controlplanev1.Cluster {
	return &controlplanev1.Cluster{
		Id:            "cl-1",
		Type:          controlplanev1.Cluster_TYPE_BYOC,
		CloudProvider: controlplanev1.CloudProvider_CLOUD_PROVIDER_AZURE,
		State:         controlplanev1.Cluster_STATE_CREATING_AGENT,
	}
}

func clusterWith(runner utils.ByocRunner, svc *fakeClusterService) *Cluster {
	c := &Cluster{Byoc: runner}
	if svc != nil {
		c.CpCl = &cloud.ControlPlaneClientSet{Cluster: svc}
	}
	return c
}

// preflightRunner is a byoc runner whose preflight answer is scripted and
// whose calls are recorded.
type preflightRunner struct {
	err     error
	checked []string
}

func (*preflightRunner) RunByoc(context.Context, string, string, func(string)) error { return nil }

func (r *preflightRunner) CheckCloudConfig(cloud string) error {
	r.checked = append(r.checked, cloud)
	return r.err
}

// plainRunner offers no preflight.
type plainRunner struct{}

func (plainRunner) RunByoc(context.Context, string, string, func(string)) error { return nil }

// Plan shapes for byocPreflightRequest: a create plan has no state, a destroy
// plan has no plan, and an update plan has both.
const (
	createPlan  = "create"
	updatePlan  = "update"
	destroyPlan = "destroy"
)

// byocPreflightRequest builds a plan of the given shape for a cluster whose
// only known attributes are cluster_type, cloud_provider, and allow_deletion.
func byocPreflightRequest(ctx context.Context, t *testing.T, clusterType string, cloudProvider tftypes.Value, shape string) resource.ModifyPlanRequest {
	t.Helper()
	return byocPreflightRequestAttrs(ctx, t, map[string]tftypes.Value{
		"cluster_type":   tftypes.NewValue(tftypes.String, clusterType),
		"cloud_provider": cloudProvider,
	}, shape)
}

// minimalStateRequest builds the plan shape a failed create leaves behind:
// the state carries the id and allow_deletion and nothing else.
func minimalStateRequest(ctx context.Context, t *testing.T, shape string) resource.ModifyPlanRequest {
	t.Helper()
	return byocPreflightRequestAttrs(ctx, t, nil, shape)
}

func byocPreflightRequestAttrs(ctx context.Context, t *testing.T, attrs map[string]tftypes.Value, shape string) resource.ModifyPlanRequest {
	t.Helper()
	s := ResourceClusterSchema(ctx)
	objType := mustTFType[tftypes.Object](t, s.Type().TerraformType(ctx))
	set := map[string]tftypes.Value{
		"id":             tftypes.NewValue(tftypes.String, "cl-1"),
		"allow_deletion": tftypes.NewValue(tftypes.Bool, true),
	}
	for k, v := range attrs {
		set[k] = v
	}
	obj := nullsExcept(objType, set)
	null := tftypes.NewValue(objType, nil)
	plan, state := obj, obj
	switch shape {
	case createPlan:
		state = null
	case destroyPlan:
		plan = null
	default:
	}
	return resource.ModifyPlanRequest{
		Config: tfsdk.Config{Schema: s, Raw: plan},
		Plan:   tfsdk.Plan{Schema: s, Raw: plan},
		State:  tfsdk.State{Schema: s, Raw: state},
	}
}

// requirePreflightDiag checks that diags carry exactly one preflight error
// mentioning want, or none when want is empty. Other diagnostics are ignored:
// an update plan runs the rest of ModifyPlan, whose private-state writes
// cannot be satisfied by a unit-test request.
func requirePreflightDiag(t *testing.T, diags diag.Diagnostics, want string) {
	t.Helper()
	var preflight []diag.Diagnostic
	for _, d := range diags.Errors() {
		if strings.Contains(d.Summary(), "byoc plugin") {
			preflight = append(preflight, d)
		}
	}
	if want == "" {
		if len(preflight) != 0 {
			t.Fatalf("unexpected preflight diagnostics: %v", preflight)
		}
		return
	}
	if len(preflight) != 1 {
		t.Fatalf("preflight diagnostics = %v, want exactly one", preflight)
	}
	if !strings.Contains(preflight[0].Detail(), want) {
		t.Fatalf("preflight diagnostic detail = %q, want it to mention %q", preflight[0].Detail(), want)
	}
}

// withoutControlPlane runs fn against a resource that has no control-plane
// client, so any control-plane call before the preflight panics and fails
// the test with a clear message instead of a stack trace.
func withoutControlPlane(t *testing.T, step string, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("%s reached the control plane before the byoc preflight: %v", step, r)
		}
	}()
	fn()
}

// TestModifyPlan_ByocPreflight pins that a BYOC cluster's create and
// destroy plans run the byoc runner's preflight and fail on its error, while
// dedicated clusters, an unknown cloud provider, and a runner with no
// preflight are left alone.
func TestModifyPlan_ByocPreflight(t *testing.T) {
	ctx := context.Background()
	azure := tftypes.NewValue(tftypes.String, "azure")
	missing := errors.New("azure_subscription_id must be set on the provider")
	cases := []struct {
		name        string
		clusterType string
		cloud       tftypes.Value
		shape       string
		runner      utils.ByocRunner
		wantErr     string
		wantChecked []string
	}{
		{"byoc create plan fails on a failing preflight", "byoc", azure, createPlan, &preflightRunner{err: missing}, "azure_subscription_id", []string{"azure"}},
		{"byoc destroy plan fails on a failing preflight", "byoc", azure, destroyPlan, &preflightRunner{err: missing}, "azure_subscription_id", []string{"azure"}},
		{"byoc create plan passes a passing preflight", "byoc", azure, createPlan, &preflightRunner{}, "", []string{"azure"}},
		// Update and no-change plans never run the plugin, so a workstation
		// holding only Redpanda credentials can still plan against the cluster.
		{"byoc update plan is not checked", "byoc", azure, updatePlan, &preflightRunner{err: missing}, "", nil},
		{"dedicated cluster is not checked", "dedicated", azure, createPlan, &preflightRunner{err: missing}, "", nil},
		{"unknown cloud provider is left to apply", "byoc", tftypes.NewValue(tftypes.String, tftypes.UnknownValue), createPlan, &preflightRunner{err: missing}, "", nil},
		{"runner without a preflight is not asked", "byoc", azure, createPlan, plainRunner{}, "", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Cluster{Byoc: tc.runner}
			req := byocPreflightRequest(ctx, t, tc.clusterType, tc.cloud, tc.shape)
			resp := &resource.ModifyPlanResponse{Plan: req.Plan}
			withoutControlPlane(t, "ModifyPlan", func() { c.ModifyPlan(ctx, req, resp) })
			requirePreflightDiag(t, resp.Diagnostics, tc.wantErr)
			if pr, ok := tc.runner.(*preflightRunner); ok && !slices.Equal(pr.checked, tc.wantChecked) {
				t.Errorf("preflight checked %v, want %v", pr.checked, tc.wantChecked)
			}
		})
	}

	// A failed create leaves a state with no cluster_type or cloud_provider;
	// the destroy plan then takes them from the control plane.
	t.Run("destroy plan with minimal state reads the cluster from the control plane", func(t *testing.T) {
		runner := &preflightRunner{err: missing}
		c := clusterWith(runner, &fakeClusterService{cluster: azureByocCluster()})
		req := minimalStateRequest(ctx, t, destroyPlan)
		resp := &resource.ModifyPlanResponse{Plan: req.Plan}
		withoutControlPlane(t, "ModifyPlan", func() { c.ModifyPlan(ctx, req, resp) })
		requirePreflightDiag(t, resp.Diagnostics, "azure_subscription_id")
		if !slices.Equal(runner.checked, []string{"azure"}) {
			t.Errorf("preflight checked %v, want [azure]", runner.checked)
		}
	})
	t.Run("destroy plan with minimal state and no control-plane client is left to apply", func(t *testing.T) {
		runner := &preflightRunner{err: missing}
		c := clusterWith(runner, nil)
		req := minimalStateRequest(ctx, t, destroyPlan)
		resp := &resource.ModifyPlanResponse{Plan: req.Plan}
		withoutControlPlane(t, "ModifyPlan", func() { c.ModifyPlan(ctx, req, resp) })
		requirePreflightDiag(t, resp.Diagnostics, "")
		if len(runner.checked) != 0 {
			t.Errorf("preflight checked %v, want nothing", runner.checked)
		}
	})
}

// TestCreate_ByocPreflightBeforeControlPlane pins that Create runs the
// preflight before asking the control plane for anything, so a plan made on
// another machine cannot create a cluster this one cannot finish.
func TestCreate_ByocPreflightBeforeControlPlane(t *testing.T) {
	ctx := context.Background()
	runner := &preflightRunner{err: errors.New("azure_subscription_id must be set on the provider")}
	c := &Cluster{Byoc: runner}
	req := byocPreflightRequest(ctx, t, "byoc", tftypes.NewValue(tftypes.String, "azure"), createPlan)
	resp := &resource.CreateResponse{State: req.State}
	withoutControlPlane(t, "Create", func() {
		c.Create(ctx, resource.CreateRequest{Config: req.Config, Plan: req.Plan}, resp)
	})
	requirePreflightDiag(t, resp.Diagnostics, "azure_subscription_id")
	if !slices.Equal(runner.checked, []string{"azure"}) {
		t.Errorf("preflight checked %v, want [azure]", runner.checked)
	}
}

// TestDelete_ByocPreflightBeforeDeleteCluster pins that Delete runs the
// preflight after reading the cluster from the control plane, whose type and
// cloud provider are present even in the minimal state a failed create
// leaves, and before asking the control plane to delete anything.
func TestDelete_ByocPreflightBeforeDeleteCluster(t *testing.T) {
	ctx := context.Background()
	missing := errors.New("azure_subscription_id must be set on the provider")
	full := byocPreflightRequest(ctx, t, "byoc", tftypes.NewValue(tftypes.String, "azure"), destroyPlan)
	minimal := minimalStateRequest(ctx, t, destroyPlan)
	cases := []struct {
		name        string
		state       tfsdk.State
		err         error
		wantErr     string
		wantDeleted bool
	}{
		{"full state, failing preflight", full.State, missing, "azure_subscription_id", false},
		{"minimal state after a failed create, failing preflight", minimal.State, missing, "azure_subscription_id", false},
		{"passing preflight reaches DeleteCluster", minimal.State, nil, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner := &preflightRunner{err: tc.err}
			svc := &fakeClusterService{cluster: azureByocCluster()}
			c := clusterWith(runner, svc)
			resp := &resource.DeleteResponse{State: tc.state}
			withoutControlPlane(t, "Delete", func() {
				c.Delete(ctx, resource.DeleteRequest{State: tc.state}, resp)
			})
			requirePreflightDiag(t, resp.Diagnostics, tc.wantErr)
			if !slices.Equal(runner.checked, []string{"azure"}) {
				t.Errorf("preflight checked %v, want [azure]", runner.checked)
			}
			if svc.deleted != tc.wantDeleted {
				t.Errorf("DeleteCluster reached = %v, want %v", svc.deleted, tc.wantDeleted)
			}
		})
	}
}
