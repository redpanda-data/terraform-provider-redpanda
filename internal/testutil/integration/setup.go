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

// Package integration is the shared harness for hand-rolled integration
// tests. It provides a Setup helper that wires a fresh in-memory bufconn
// fake into a Terraform ProviderFactories map, plus a small set of
// scenario-builder functions that produce resource.TestStep values with the
// canonical PreApply / PostApplyPostRefresh wiring.
//
// The library is intentionally dumb: it does not generate HCL, pick test
// values, or walk schemas. Per-resource tests author their own HCL and
// state-check lists; the library removes the boilerplate of constructing
// ConfigPlanChecks and the standard ExpectEmptyPlan post-apply assertion.
package integration

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/provider"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/mock"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda"
)

// Setup returns a fresh mock.Server and the ProviderFactories map that wires
// it into a resource.TestCase. Mirrors the boilerplate at the top of every
// hand-written integration_<r>_test.go: sets the acceptance-test-mode env
// var, allocates a new mock server, and registers a "redpanda" factory that
// dials the bufconn listener with auth skipped.
//
// NOTE: Setup calls t.Setenv, which is incompatible with t.Parallel. Tests
// that need cross-test parallelism should set REDPANDA_TF_ACCEPTANCE_TEST_MODE
// at the package level (e.g. in a TestMain) and skip this helper.
func Setup(t *testing.T) (srv *mock.Server, factories map[string]func() (tfprotov6.ProviderServer, error)) {
	t.Helper()
	t.Setenv("REDPANDA_TF_ACCEPTANCE_TEST_MODE", "1")
	srv = mock.New(t)
	return srv, Factories(srv, redpanda.WithSkipAuth(), redpanda.WithByocRunner(srv.Byoc))
}

// SetupWithProviderAuth is Setup without WithSkipAuth and with the REDPANDA_*
// credential environment variables cleared, so provider Configure resolves
// credentials the way the released binary does. Only an access_token keeps a
// test in-process: the OAuth endpoint is not faked, so a client_id and
// client_secret pair reaches the real token endpoint on the first RPC. Every
// provider configuration in a test config shares the one reattached server:
// the first credentialed Configure's connection and token serve every later
// one, while the per-configuration provider data follows the last Configure.
// A default-plus-alias config therefore cannot be tested here; Terraform runs
// one plugin process per configuration.
func SetupWithProviderAuth(t *testing.T) (srv *mock.Server, factories map[string]func() (tfprotov6.ProviderServer, error)) {
	t.Helper()
	t.Setenv("REDPANDA_TF_ACCEPTANCE_TEST_MODE", "1")
	t.Setenv("REDPANDA_TOKEN_CACHE_DISABLE", "1")
	t.Setenv(redpanda.ClientIDEnv, "")
	t.Setenv(redpanda.ClientSecretEnv, "")
	t.Setenv(redpanda.AccessTokenEnv, "")
	srv = mock.New(t)
	return srv, Factories(srv, redpanda.WithByocRunner(srv.Byoc))
}

// Factories registers a "redpanda" provider factory that dials srv over
// bufconn, with any further provider options applied.
func Factories(srv *mock.Server, opts ...redpanda.Option) map[string]func() (tfprotov6.ProviderServer, error) {
	providerOpts := []provider.Option{provider.WithProviderOption(redpanda.WithDialer(srv.Dialer()...))}
	for _, o := range opts {
		providerOpts = append(providerOpts, provider.WithProviderOption(o))
	}
	return provider.ProtoV6ProviderFactories(context.Background(), "pre", "test", providerOpts...)
}
