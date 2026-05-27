//go:build upgrade

package upgrade

import (
	"context"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/provider"
)

// CreateAndRunTest runs the Atlas-style two-step provider-upgrade test:
//
//   - Step 0: apply with the released provider via ExternalProviders.
//   - Step 1: re-plan with the local build via ProtoV6ProviderFactories,
//     assert empty plan (PlanOnly: true, ExpectNonEmptyPlan: false).
//
// The caller passes a TestCase whose Steps[0].Config holds the resource
// configuration to apply. CreateAndRunTest injects ExternalProviders onto
// Step 0 and appends Step 1; callers must not set ProtoV6ProviderFactories
// on their own steps.
func CreateAndRunTest(t *testing.T, tc resource.TestCase) {
	t.Helper()
	PreCheck(t)

	if len(tc.Steps) == 0 {
		t.Fatal("upgrade.CreateAndRunTest: TestCase has no Steps")
	}

	tc.Steps[0].ExternalProviders = ExternalProviders()

	tc.Steps = append(tc.Steps, resource.TestStep{
		Config:                   tc.Steps[0].Config,
		ProtoV6ProviderFactories: localFactories(),
		PlanOnly:                 true,
		ExpectNonEmptyPlan:       false,
	})

	resource.Test(t, tc)
}

func localFactories() map[string]func() (tfprotov6.ProviderServer, error) {
	cloudEnv := os.Getenv("REDPANDA_CLOUD_ENVIRONMENT")
	if cloudEnv == "" {
		cloudEnv = "pre"
	}
	return provider.ProtoV6ProviderFactories(context.Background(), cloudEnv, "test")
}
