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

package protovalidate

import (
	"errors"

	"buf.build/go/protovalidate"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"google.golang.org/protobuf/proto"
)

// Validate reports protovalidate violations on msg as attribute errors at
// rootPath, naming the proto field path in the summary instead of translating it.
// skipFields drops violations for rules the schema deliberately lets users break.
// A protovalidate setup error becomes a diagnostic so it can never read as a pass.
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
