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

package validators

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// PSCConsumerSourceValidator rejects a gcp_private_service_connect
// consumer_accept_list source carrying a "projects/" resource-name prefix.
// The GCP Compute API's projectIdOrNum field wants a bare project ID or
// number; a "projects/..." value is silently accepted by the control plane
// (it has no validation rule for this field) and then bricks cluster
// creation in the GCP agent with PROJECT_NOT_FOUND. Catching it at plan
// time is the only place the provider can surface it before apply.
type PSCConsumerSourceValidator struct{}

// Description returns a plain-text description of the validator's behavior.
func (PSCConsumerSourceValidator) Description(_ context.Context) string {
	return `must be a bare GCP project ID or number, not a "projects/..." resource name`
}

// MarkdownDescription returns a markdown description of the validator's behavior.
func (v PSCConsumerSourceValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

// ValidateString rejects values containing a path separator, which a bare GCP
// project ID or number never does.
func (PSCConsumerSourceValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	v := req.ConfigValue.ValueString()
	if strings.Contains(v, "/") {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid PSC consumer source",
			fmt.Sprintf(`%q must be a bare GCP project ID or number (e.g. "my-project" or "123456789012"), not a "projects/..." resource name`, v),
		)
	}
}
