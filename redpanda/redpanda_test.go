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

package redpanda

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
)

func TestProviderConfigure(t *testing.T) {
	ctx := context.Background()

	rp := New(ctx, "ign", "test")()
	rp.Schema(ctx, provider.SchemaRequest{}, &provider.SchemaResponse{})

	if d := providerSchema().ValidateImplementation(ctx); d.HasError() {
		t.Fatalf("unexpected error in provider schema: %s", d)
	}
}
