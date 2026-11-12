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
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestDelete_LookupBeforeDelete pins what Delete makes of the lookup it runs
// before asking the control plane to delete anything. A destroy through
// Terraform cannot reach it with a failing lookup, because the refresh fails
// first.
func TestDelete_LookupBeforeDelete(t *testing.T) {
	ctx := context.Background()
	state := minimalStateRequest(ctx, t, destroyPlan).State
	cases := []struct {
		name        string
		err         error
		wantSummary string
		wantDetail  string
	}{
		{"not found is already deleted", status.Error(codes.NotFound, "cluster not found"), "", ""},
		{"denial naming the cluster", mock.DeniedOn("cl-1"), "Redpanda Cloud denied access to cluster cl-1", "terraform state rm"},
		{"denial naming something else", mock.DeniedOn("other"), "failed to read cluster", ""},
		{"internal error", status.Error(codes.Internal, "boom"), "failed to read cluster", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeClusterService{getErr: tc.err}
			resp := &resource.DeleteResponse{State: state}
			clusterWith(plainRunner{}, svc).Delete(ctx, resource.DeleteRequest{State: state}, resp)

			if svc.deleted {
				t.Error("DeleteCluster was reached")
			}
			errs := resp.Diagnostics.Errors()
			if tc.wantSummary == "" {
				if len(errs) != 0 {
					t.Fatalf("want no error, got %v", errs)
				}
				return
			}
			if len(errs) != 1 {
				t.Fatalf("want one error, got %v", errs)
			}
			if !strings.Contains(errs[0].Summary(), tc.wantSummary) {
				t.Errorf("summary %q does not contain %q", errs[0].Summary(), tc.wantSummary)
			}
			if !strings.Contains(errs[0].Detail(), tc.wantDetail) {
				t.Errorf("detail %q does not contain %q", errs[0].Detail(), tc.wantDetail)
			}
			if tc.wantDetail == "" && strings.Contains(errs[0].Detail(), "terraform state rm") {
				t.Errorf("unexpected state advice in %q", errs[0].Detail())
			}
		})
	}
}
