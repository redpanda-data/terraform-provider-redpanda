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

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ validator.String = NameFilterWildcardValidator{}

// NameFilterWildcardValidator enforces the core NameFilter contract on a
// filter's name: the wildcard "*" matches every name and is valid only with
// pattern_type "LITERAL". The control plane and the agent pass the filter
// through unvalidated, so plan time is the last place this can fail before the
// shadow link exists. An asterisk inside a longer name is a literal character
// and is left alone.
type NameFilterWildcardValidator struct{}

// Description returns a plain-text description of the validator's behavior.
func (NameFilterWildcardValidator) Description(_ context.Context) string {
	return `the wildcard name "*" requires pattern_type "LITERAL"`
}

// MarkdownDescription returns a markdown description of the validator's behavior.
func (v NameFilterWildcardValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

// ValidateString defers while the sibling pattern_type is unknown rather than
// raise a false positive at plan time.
func (NameFilterWildcardValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() || req.ConfigValue.ValueString() != "*" {
		return
	}

	var pattern types.String
	if diags := req.Config.GetAttribute(ctx, req.Path.ParentPath().AtName("pattern_type"), &pattern); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	if pattern.IsUnknown() {
		return
	}
	if pattern.ValueString() == "LITERAL" {
		return
	}
	resp.Diagnostics.AddAttributeError(req.Path,
		"Invalid Name Filter",
		fmt.Sprintf(`the wildcard name "*" matches every name and requires pattern_type "LITERAL", got %q`, pattern.ValueString()))
}
