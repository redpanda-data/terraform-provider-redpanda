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

package acc

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/provider"
)

// ProtoV6Factories is the provider factory map consumed by every
// resource.TestCase via ProtoV6ProviderFactories.
var ProtoV6Factories map[string]func() (tfprotov6.ProviderServer, error)

func init() {
	ProtoV6Factories = localBuildFactories()
}

// localBuildFactories publishes the in-process build under the address the
// lane configs pin, registry.terraform.io/redpanda-data/redpanda. Without the
// namespace override plugin-testing reattaches it as hashicorp/redpanda, so
// a config that pins the registry source never reaches the local build and
// Terraform selects the released provider instead.
func localBuildFactories() map[string]func() (tfprotov6.ProviderServer, error) {
	_ = os.Setenv(resource.EnvTfAccProviderNamespace, "redpanda-data")
	return provider.ProtoV6ProviderFactories(context.Background(), CloudEnv, "test")
}
