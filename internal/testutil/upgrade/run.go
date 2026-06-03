//go:build upgrade

package upgrade

import (
	"context"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
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

// CreateAndRunMigrationTest runs the two-step upgrade with DISTINCT configs:
// step0Config applied by the released provider, then step1Config re-planned by
// the local build (PlanOnly, ExpectNonEmptyPlan: false). Use when the upgrade
// legitimately changes a value's representation — e.g. cluster_api_url migrating
// from the legacy host:443 form the released provider stored to the canonical
// https://host the local build expects — so the post-upgrade config differs from
// what Step 0 wrote. An empty Step 1 plan proves the migration reconciled in
// place rather than forcing replacement.
func CreateAndRunMigrationTest(t *testing.T, step0Config, step1Config string) {
	t.Helper()
	PreCheck(t)
	resource.Test(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config:            step0Config,
				ExternalProviders: ExternalProviders(),
			},
			{
				Config:                   step1Config,
				ProtoV6ProviderFactories: localFactories(),
				PlanOnly:                 true,
				ExpectNonEmptyPlan:       false,
			},
		},
	})
}

// CreateAndRunMigrationApplyTest runs step 0 (released provider, step0Config)
// then step 1 (local build, step1Config) as a full apply, asserting the given
// pre-apply plan checks. Use instead of CreateAndRunMigrationTest when the
// upgrade legitimately updates an unrelated managed resource — e.g. a
// self-provisioned cluster that gained a defaulted provider-only field
// (allow_deletion) between the released and local schemas, which a whole-plan
// empty assertion would trip. Assert the resources under test individually via
// step1PreApply (typically ExpectResourceAction(addr, ResourceActionNoop) for
// each), and let step 1 apply so the field is set and teardown succeeds.
func CreateAndRunMigrationApplyTest(t *testing.T, step0Config, step1Config string, step1PreApply []plancheck.PlanCheck) {
	t.Helper()
	PreCheck(t)
	resource.Test(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config:            step0Config,
				ExternalProviders: ExternalProviders(),
			},
			{
				Config:                   step1Config,
				ProtoV6ProviderFactories: localFactories(),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: step1PreApply,
				},
			},
		},
	})
}

func localFactories() map[string]func() (tfprotov6.ProviderServer, error) {
	cloudEnv := os.Getenv("REDPANDA_CLOUD_ENVIRONMENT")
	if cloudEnv == "" {
		cloudEnv = "pre"
	}
	return provider.ProtoV6ProviderFactories(context.Background(), cloudEnv, "test")
}
