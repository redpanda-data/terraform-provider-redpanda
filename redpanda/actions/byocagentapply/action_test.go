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

package byocagentapply

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
)

func TestAction_Configure_NoCredentials(t *testing.T) {
	a := &Action{}
	resp := &action.ConfigureResponse{}
	a.Configure(context.Background(), action.ConfigureRequest{ProviderData: config.Resource{NoCredentials: true}}, resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error diagnostic for a provider configured without credentials")
	}
	if got := resp.Diagnostics.Errors()[0].Summary(); got != config.NoCredentialsSummary {
		t.Fatalf("summary: got %q, want %q", got, config.NoCredentialsSummary)
	}
	if a.cpCl != nil || a.byoc != nil {
		t.Fatal("clients must not be wired when the provider has no credentials")
	}
}
