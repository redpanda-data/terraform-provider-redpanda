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

package utils

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// DefaultBoolValue follows the TF framework bool type.
type DefaultBoolValue struct {
	Value    bool
	Desc     string
	MarkDesc string
}

// Description return the description string.
func (d *DefaultBoolValue) Description(_ context.Context) string {
	return d.Desc
}

// MarkdownDescription returns the markdown description string.
func (d *DefaultBoolValue) MarkdownDescription(_ context.Context) string {
	return d.MarkDesc
}

// DefaultBool sets the default bool value in resp.PlanValue.
func (d *DefaultBoolValue) DefaultBool(_ context.Context, _ defaults.BoolRequest, resp *defaults.BoolResponse) {
	resp.PlanValue = types.BoolValue(d.Value)
}
