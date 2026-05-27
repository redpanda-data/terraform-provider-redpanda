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

package acc

import (
	"os"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda"
)

// ProviderCfgIDSecretVars carries the client_id / client_secret variables
// every test config consumes via ConfigVariables.
var ProviderCfgIDSecretVars = config.Variables{
	"client_id":     config.StringVariable(os.Getenv(redpanda.ClientIDEnv)),
	"client_secret": config.StringVariable(os.Getenv(redpanda.ClientSecretEnv)),
}
