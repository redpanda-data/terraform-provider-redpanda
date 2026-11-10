// Copyright 2023 Redpanda Data, Inc.
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
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestExpandCreate_SendsRedpandaVersion(t *testing.T) {
	for _, tc := range []struct {
		name    string
		version types.String
		want    string
	}{
		{"major.minor", types.StringValue("26.2"), "26.2"},
		{"install pack version", types.StringValue("26.2.20261110174043"), "26.2.20261110174043"},
		{"unset", types.StringNull(), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := GenerateMinimalResourceModel(types.StringValue("id"), ResourceModel{}.Timeouts)
			m.RedpandaVersion = tc.version
			req, diags := ExpandCreate(context.Background(), m)
			if diags.HasError() {
				t.Fatalf("ExpandCreate: %v", diags)
			}
			if got := req.GetCluster().GetRedpandaVersion(); got != tc.want {
				t.Fatalf("redpanda_version sent = %q, want %q", got, tc.want)
			}
		})
	}
}
