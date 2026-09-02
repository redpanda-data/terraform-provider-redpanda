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

package validators

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var _ validator.String = DurationCanonicalValidator{}

// DurationCanonicalValidator requires the value to be a Go duration string in
// canonical form: the exact string time.Duration.String() produces. Duration
// fields round-trip through durationpb, so any other spelling ("1m", "90s",
// "0") reads back differently and fails apply with an inconsistent-result
// error.
type DurationCanonicalValidator struct{}

// Description provides a description of the validator.
func (DurationCanonicalValidator) Description(_ context.Context) string {
	return "value must be a canonical Go duration string (e.g. \"30s\", \"1m0s\")"
}

// MarkdownDescription provides a description of the validator in markdown format.
func (DurationCanonicalValidator) MarkdownDescription(_ context.Context) string {
	return "value must be a canonical Go duration string (e.g. `30s`, `1m0s`)"
}

// ValidateString validates that the string is a canonical Go duration.
func (DurationCanonicalValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	v := req.ConfigValue.ValueString()
	d, err := time.ParseDuration(v)
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Duration",
			fmt.Sprintf("value must be a Go duration string, got: %q (%s)", v, err),
		)
		return
	}
	if canonical := d.String(); v != canonical {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Non-Canonical Duration",
			fmt.Sprintf("value must be a canonical Go duration string: use %q instead of %q (the API echoes the canonical form, and any other spelling fails apply)", canonical, v),
		)
	}
}
