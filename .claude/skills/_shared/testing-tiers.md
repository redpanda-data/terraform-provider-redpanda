# Testing tiers

Four distinct test tiers, each with a specific cost/coverage tradeoff. Most resource work touches the bottom three; the top tier (live cluster lifecycle) is reserved for cluster/network/BYOVPC.

| Tier | Path | Package | Build tag | Run via | Creds |
|------|------|---------|-----------|---------|-------|
| 1. Model unit | `redpanda/models/<name>/*_test.go` | `package <name>` (internal) | none | `task test:unit` | none |
| 2. Colocated integration | `redpanda/resources/<name>/integration_<name>_test.go` | `package <name>_test` (external) | none | `task test:unit` | none |
| 3. Colocated live-acc | `redpanda/resources/<name>/acc_<name>_test.go` | `package <name>_test` | `live_test && (all \|\| <name>)` | `task test:<name>` | `REDPANDA_CLIENT_ID/SECRET` + cloud |
| 4. Global live-acc | `redpanda/tests/runner_<name>_test.go` | `package tests` | `live_test && (all \|\| <name>_<provider>)` | `task test:cluster:aws` etc. | live |

Upgrade tests are a parallel track: `redpanda/resources/<name>/upgrade_<name>_test.go`, build tag `upgrade`, run via `task test:upgrade`.

Memory `project_colocated_test_cycle.md`: colocated test files must use external `<name>_test` package — internal `<name>` package creates a root-provider import cycle. Model tests (Tier 1) use internal package because they don't import the provider.

## Tier 1: Model unit (`redpanda/models/<name>/*_test.go`)

Test `Flatten`/`Expand` directly with proto stubs. Hand-written — `task generate:models` does **not** produce `*_gen_test.go`. Common patterns:

- `*_matrix_test.go` — table-driven matrix for one field's behavior (null vs present, prev-preservation, server-default round-trip). Example: `redpanda/models/cluster/gcp_global_access_matrix_test.go`.
- `conv_test.go` — tests for hand-written `flatten_via:` / `expand_via:` functions in `conv.go`.

When extending: add a matrix test only when the new field has behavioral nuance (null-vs-present, prev-preservation, UseStateForUnknown contract). For routine scalar fields, the integration tier (Tier 2) covers it.

## Tier 2: Colocated integration (`integration_<name>_test.go`)

The day-to-day test tier. Real Terraform lifecycle (`Create → NoopReapply → Update → RequiresReplace → Import`) against an in-memory gRPC fake — no cloud creds, no real cluster, runs with `task test:unit`.

### Leaf-coverage rule (non-negotiable)

Per memory `feedback_test_all_leaves.md`: **every leaf in the resource's `*.golden` file must be exercised here.** Coverage means a `stateChecks` assertion (value or explicit null) in the Create step, plus exercise in an UpdateLeaf step if mutable, plus a NestedMatrix entry if nested.

Missing leaves cause `Error: Provider produced inconsistent result after apply` at runtime — opaque, painful, only surfaces when a user happens to set the uncovered attribute. The integration tier is specifically designed to catch these before they ship.

Verification step: after regen, list leaves from `redpanda/resources/testdata/<name>_resource_schema.golden` and walk each to confirm coverage exists. Don't skip this audit on extends — a YAML edit can silently drop a `stateChecks` line.

Skipping a leaf requires explicit user authorization per-leaf after they're informed of the inconsistency-bug risk. Implicit authorization does not carry forward.

### Setup pattern

```go
package <name>_test

import (
    "github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/integration"
    "github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/mock"
    "github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestIntegration_<Name>_CreateAndRefresh(t *testing.T) {
    srv, factories := integration.Setup(t)  // wires bufconn fake into provider
    _ = srv

    resource.UnitTest(t, resource.TestCase{
        ProtoV6ProviderFactories: factories,
        Steps: []resource.TestStep{
            integration.CreateStep(addr, config, stateChecks),
            integration.NoopReapplyStep(addr, config, stateChecks),
            integration.UpdateLeafStep(addr, updatedConfig, stateChecks),
            integration.ImportRoundTripStep(addr, idFunc, verifyIgnore),
        },
    })
}
```

### Step builders (`internal/testutil/integration/steps.go`)

- `CreateStep(addr, config, stateChecks)` — Create + assert empty plan after
- `NoopReapplyStep(addr, config, stateChecks)` — assert action == `Noop`
- `UpdateLeafStep(addr, config, stateChecks)` — Update + assert action == `Update`
- `RequiresReplaceStep(addr, config, stateChecks)` — assert action == `DestroyBeforeCreate`
- `RequiresReplaceIfStep(addr, config, triggersReplace, stateChecks)` — conditional replace
- `ImportRoundTripStep(addr, idFunc, verifyIgnore)` — ImportState + round-trip verify
- `ErrorPathStep(srv, method, code, config, errPattern)` — inject gRPC error via `srv.OverrideOnce`
- `RESTErrorPathStep(...)` — HTTP error for schema registry

Each step asserts `plancheck.ExpectEmptyPlan()` automatically.

### When extending — modify first, create only with authorization

Per memory `feedback_extend_existing_tests.md`: **default to modifying existing tests, not creating new ones.** Agents reliably break this rule and produce duplicate coverage.

- **Default**: add a `stateChecks` entry to the existing `TestIntegration_<Name>_CreateAndRefresh` (or equivalent primary test).
- **Default for mutable fields**: append a step to an existing `TestIntegration_<Name>_UpdateLeaf_*` function.
- **Default for nested-block fields**: add to existing `TestIntegration_<Name>_NestedMatrix_<Block>_Dense`.

Creating a new test function (`TestIntegration_<Name>_UpdateLeaf_<NewField>`) is only allowed when (a) there is an extremely compelling justification that an existing function genuinely can't fit, AND (b) the user is informed of this rule and authorizes the new function in the current session. Implicit authorization does not carry forward — ask each time.

Never create a new `integration_<name>_test.go` file when one exists.

### Adding a new resource — fake registration

A new control-plane resource needs a fake added to `internal/testutil/mock/fakes/<service>.go` and registered in `mock.Server` (`internal/testutil/mock/server.go`). The fake implements the gRPC service interface against an in-memory store.

For dataplane resources (topic, user, acl, schema*), the dataplane fake infrastructure is in `internal/testutil/mock/fakes/dataplane/`.

## Tier 3: Colocated live-acc (`acc_<name>_test.go`)

Real provider against Redpanda Cloud. Use only for behavior that the integration tier cannot exercise: real API rate limits, server-enforced constraints, async timing.

When a live acc test hangs or leaves dangling cloud resources, per memory `feedback_use_taskfile_cleanup.md`: **kill the test process and run `task cleanup:aws:ci`** (or `cleanup:gcp:ci`) directly. Don't wait on the test framework's destroy — it often can't complete cleanup once the test process is wedged.

Memory `feedback_allow_deletion.md`: cluster tests set `allow_deletion = false` intentionally — it's a canary that surfaces test failures rather than masking them. When a cluster test fails because `allow_deletion = false` blocks destroy, **fix the upstream failure that caused destroy to be needed**, don't flip the flag.

### Setup pattern

```go
//go:build live_test && (all || <name>)
// +build live_test,all live_test,<name>

package <name>_test

import (
    "github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/acc"
)

func TestAcc_<Name>_Basic(t *testing.T) {
    name := acc.RandomName("<prefix>")
    acc.PreCheck(t)  // requires REDPANDA_CLIENT_ID/SECRET

    resource.Test(t, resource.TestCase{
        ProtoV6ProviderFactories: acc.ProtoV6Factories,
        Steps: []resource.TestStep{
            { ConfigDirectory: config.StaticDirectory(acc.<Name>Dir), ConfigVariables: ... },
        },
    })
}
```

### Fixtures (`internal/testutil/acc/fixtures.go`)

Add two constants when adding a new resource:
- `<Name>Dir = "examples/<name>"` — path to acc test fixture directory
- `<Name>ResourceName = "redpanda_<name>.test"` — resource address used in checks

### Sweepers (`internal/testutil/acc/sweep/<resource>.go`)

Only for infra resources that can leak and need ordered teardown (cluster, network, resource_group, serverless_private_link, shadow_link). Simple CRUD resources (user, service account, topic) do **not** need a sweeper.

Sweeper-bearing packages also need a `testmain_test.go` calling `acc.Cleanup(ctx)` after `m.Run()`.

### Task target

Each colocated acc test needs an entry in `.tasks/test.yml`:

```yaml
<name>:
  desc: "Live acc tests for redpanda_<name>"
  cmds:
    - TF_ACC=true go test -count=1 -tags=live_test,<name> -v -timeout=30m ./redpanda/resources/<name>/...
```

## Tier 4: Global live-acc (`redpanda/tests/runner_*_test.go`)

Cluster lifecycle, BYOC, BYOVPC, serverless — multi-resource end-to-end flows that need a real cluster running. New resources rarely add to this tier unless they're cluster-adjacent infra.

Memory `project_missing_resource_pkg_unit_tests.md`: cluster/network/serverless_cluster/serverless_private_link still need resource-pkg gomock CRUD tests — followup after the integration tier is fully fleshed out.

## Upgrade tests (`upgrade_<name>_test.go`)

```go
//go:build upgrade
package <name>_test
```

Runs with `task test:upgrade` or `task test:upgrade:smoke`. Pattern: create with released provider, plan with HEAD, assert empty plan.

For a new resource, no upgrade test is needed (no prior state exists). For extends, the existing upgrade config should **not** include the new field — exercising the case where existing state lacks the new attribute is the load-bearing assertion.

## Hierarchy: where does this test go?

```
Is it pure Flatten/Expand logic with proto stubs?
  → Tier 1 (model_test)

Is it a Terraform lifecycle assertion (Create/Update/Import) without cloud APIs?
  → Tier 2 (integration_test)

Is the behavior under test only observable against a real cluster?
  → Tier 3 (acc_test), or rarely Tier 4 (runner_test)

Is the test "does v1.9.0 state plan empty against HEAD?"
  → upgrade_test
```

## See also

- [manual-validation](manual-validation.md) — live cluster smoke testing, complements but doesn't replace test tiers
- [codegen-workflow](codegen-workflow.md) — `TestSchemaGolden` baseline workflow
