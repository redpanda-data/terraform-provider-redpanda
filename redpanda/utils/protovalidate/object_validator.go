// Copyright 2023 Redpanda Data, Inc.
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

package protovalidate

import (
	"errors"

	"buf.build/go/protovalidate"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"google.golang.org/protobuf/proto"
)

// Validate runs the global protovalidate instance against msg and returns
// Terraform diagnostics for each violation. The violations are attached at
// rootPath — the Path that the framework passed to the ObjectValidator —
// without per-field path translation; the violation's field path is
// appended to the diagnostic summary instead.
//
// skipFields is a static list of proto field paths (e.g. "name",
// "cluster_configuration.custom_properties") whose violations should be
// suppressed. Each entry matches the field path protovalidate reports for
// a violation; matching violations are dropped before being turned into
// diagnostics. Used for fields whose buf.validate.field rule rejects
// configs that the TF schema legitimately allows (per-field opt-out from
// proto-driven validation).
//
// Returns nil diagnostics if msg validates cleanly. Non-validation errors
// from protovalidate (e.g. a malformed CEL program, registry mismatch)
// surface as a single error diagnostic so callers can see the failure
// instead of silently treating it as "passed".
func Validate(rootPath path.Path, msg proto.Message, skipFields ...string) diag.Diagnostics {
	var diags diag.Diagnostics
	err := protovalidate.Validate(msg)
	if err == nil {
		return diags
	}
	var vErr *protovalidate.ValidationError
	if !errors.As(err, &vErr) {
		diags.AddAttributeError(rootPath, "proto validation setup error", err.Error())
		return diags
	}
	skip := make(map[string]bool, len(skipFields))
	for _, f := range skipFields {
		skip[f] = true
	}
	for _, v := range vErr.Violations {
		field := protovalidate.FieldPathString(v.Proto.GetField())
		if skip[field] {
			continue
		}
		msg := v.Proto.GetMessage()
		if msg == "" {
			msg = v.Proto.GetRuleId()
		}
		if field == "" {
			diags.AddAttributeError(rootPath, "proto validation failed", msg)
		} else {
			diags.AddAttributeError(rootPath, "proto validation failed: "+field, msg)
		}
	}
	return diags
}
