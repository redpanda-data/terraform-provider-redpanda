# Testing Methodology — terraform-provider-redpanda

Synthesis of how six mature Terraform providers (AWS, HashiCorp Cloud Platform,
Terraform Cloud / Enterprise, Confluent Cloud, MongoDB Atlas, Snowflake) handle
testing, with concrete recommendations for this repo.

Per-provider research notes live in `.claude/testing-research/`.

---

## 1. What we already have

| Layer | Today | Status |
|---|---|---|
| Unit (`task test:unit`) | `redpanda/resources/*/resource_*_test.go` with gomock clients from `redpanda/mocks/` | Good. Ahead of HCP, Confluent, TFE, AWS, Snowflake on this axis. |
| Live acceptance | `redpanda/tests/acc_*_test.go` (23 files), gated by `REDPANDA_CLIENT_ID` + cloud creds | Works, but is the *only* tier between unit and prod. Slow, expensive, flaky. |
| Sweepers | `task cleanup:aws:ci`, `task cleanup:redpanda` (separate binaries / shell entrypoints) | External to `go test`. Not wired into `TestMain`. |
| CI | Single BuildKite `pipeline.yml` | No fan-out, no path-filtering, no label triggers. |

The gap is the **middle tier**. Every provider we surveyed that has solved the
"acceptance tests are too expensive to run on every PR" problem solved it the
same way: by adding a tier that runs the full `resource.TestCase` machinery
against a mocked transport.

---

## 2. Survey at a glance

| Provider | Tiers | Mock approach | Sweepers | PR-time acc cost | Plugin Framework |
|---|---|---|---|---|---|
| AWS | unit + acc | None (real AWS only) | `internal/sweep/awsv2/` per-service, nightly TeamCity | $0 (acc is nightly only) | Mid-migration |
| HCP | unit + acc | None | None — leaks accepted | Per-product GH Actions fan-out | Mid-migration |
| TFE | unit + acc | Trivial mock surface, mostly real API | Single root sweeper (`tst-terraform*` orgs cascade-delete) | Cheap (one shared `tflocal` instance, 8-shard split) | Mid-migration, mux factory |
| Confluent | unit + **integration (WireMock)** + live-acc | WireMock + Testcontainers, JSON fixtures in `internal/testdata/<resource>/` | None | $0 (PRs hit WireMock; live promoted manually) | SDKv2 only |
| MongoDB Atlas | unit + **MacT** + acc + migration | Custom `httpmock` RoundTripper + YAML capture/replay + goldie | Per-package `TestMain → acc.Run(m)`; no `AddTestSweepers` | $0 (PRs run `^TestAccMockable` only) | Mid-migration, muxed |
| Snowflake | unit + SDK integration + acc + functional + arch | None at acc/integration tier; suffix-from-SHA isolation | `pkg/sdk/sweepers_test.go` as a regular Go test (prefix + suffix + age) | Real account on every PR | Mid-migration POC |

Two patterns are clearly worth stealing:

1. **A integration tier that runs `resource.TestCase` against a mocked
   transport.** Confluent and MongoDB Atlas both run the full Terraform test
   engine end-to-end inside the test process. This is where they get most of
   their leverage.
2. **PR-time acc that costs $0; live acc gated by explicit human action.**
   The integration tier exists so PRs are guarded by something stronger than
   unit tests, without burning real cloud resources.

---

## 3. Headline patterns to adopt

### 3.1 Add a integration tier (proto-based fake)

The single biggest win. Two approaches in the wild:

- **MongoDB Atlas (MacT)** — custom `httpmock` RoundTripper with YAML
  capture/replay. Fixture *is* the contract. Refresh by re-running against
  a real cluster.
- **Confluent** — WireMock 2.32 in a Testcontainers container, programmed
  via `walkerus/go-wiremock` with stateful Scenarios.

We use a third approach that suits our connect-go + proto stack better:
**a typed Go fake server**.

Why fake server is the primary:
- We have connect-go handler interfaces. A fake compiles against those, so a
  proto refactor breaks the fake at the compile site.
- A stateful in-memory fake handles Create-then-Read, retries, and polling
  naturally. Replay needs ordering machinery (`AllowOutOfOrder`,
  `duplicate_responses`, `response_index`) to paper over the same flows.
- New `TestIntegration*` tests are pure Go edits; no cluster spin-up to author.

### 3.2 Sweepers: deterministic, per-job

We do **not** use `resource.AddTestSweepers`. That mechanism is built for a
scheduled-sweeper world we explicitly don't want. Instead:

- **Per-package `TestMain`** (Atlas pattern). `TestMain(m)` calls `m.Run()`
  then `acc.Cleanup()`, which walks a package-level shared-resource registry
  and tears down anything still tracked. Tests register their resources via
  the helpers in `internal/testutil/acc/`; ad-hoc `t.Cleanup` calls remain
  fine for resources outside the registry.
- **CI job postscript**. Every live-acc job ends with `task cleanup:<cloud>:ci`
  and `task cleanup:redpanda` as an always-runs final step. Belt-and-suspenders
  for `TestMain` failures or crashed test processes.

There is no scheduled sweeper. There is no `-sweep=<region>` flag. Reviewing
the actual cloud state is a manual-testing-rig concern, not a CI concern.

### 3.3 Commit-SHA suffix on every test name

`TF_TEST_OBJECT_SUFFIX=$BUILDKITE_COMMIT` (Snowflake's pattern, our env var).
Lets concurrent runs share one cloud account without colliding *and* lets
the cleanup postscript distinguish "in-flight from this PR" from "leaked
from a previous run." We're already partway there with `tfrp-*` random
suffixes; pinning to the commit SHA is a small change with a real payoff.

### 3.4 Build-tag groups (Confluent pattern)

```go
//go:build live_test && (all || cluster_aws)
```

Replaces the proliferating per-cloud taskfile targets with declarative file
metadata. CI selects slices via `-tags=live_test,cluster_aws`. Group names
map 1:1 to the labels that trigger live runs (§5.5).

We have ~10 taskfile targets today (`test:cluster:aws`, `:byoc:aws`,
`:byovpc:aws`, `:serverless:aws:public`, etc.). Build-tag groups subsume
that and add free compile-time exclusion.

### 3.5 Mux factory for the framework (TFE / Atlas pattern)

`internal/provider/provider_test.go` in TFE is the canonical example:
package-level `testAccMuxedProviders` initialized in `init()`, wraps the
framework provider in `tf6muxserver.NewMuxServer`. We're plugin-framework
only today, so we don't need the SDKv2 → v6 upgrade leg — but wrapping in
a mux *now* costs nothing and means any future migration leg is free.

Production `main.go` uses the same factory. The only test-side override
is `ConfigureContextFunc` (or equivalent) to inject env-derived creds and
stash the client for assertion helpers.

### 3.6 `isAcceptanceTestMode` plumbed through the client

Confluent's smallest, highest-leverage change. The provider config exposes
`isAcceptanceTestMode` (set when `TF_ACC=1`). Every retry / poll /
"wait-for-backend" call site calls `SleepIfNotTestMode(d, mode)` which
collapses to 500 ms in mock mode and the real duration in live mode.

Without this, a mocked CRUD test still pays the wall-clock cost of every
poll loop. `redpanda/utils/retry.go` is the obvious plumb point.

### 3.7 Consolidated TestCases (HCP / Atlas pattern)

HCP's `contributing/writing-tests.md` is explicit: "reuses one test config
to test the Vault cluster resource, the Vault cluster datasource, and the
dependent Vault cluster admin token resource. This helps speed up the
acceptance test runtime by creating a Vault cluster, the most time-intensive
resource, only once."

A redpanda cluster is 30-50 min and ~$10-15. We should be aggressive about
consolidating resource + datasource + dependent-resource scenarios into one
`TestCase` with many `TestStep`s. We already partially do this in
`acc_cluster_aws_test.go`; codify it as policy.

### 3.8 Provider-upgrade tests (`upgrade` package — Atlas pattern)

```go
upgrade.CreateAndRunTest(t, basicTestCase(t))
```

Step 0 fetches the previously-released provider from the Registry via
`ExternalProviders(versionConstraint())`, step 1 swaps in the local build
with `ExpectEmptyPlan`. Authoring an upgrade test becomes a one-liner.

Worth adding for the cases where a schema change could cause planned diffs
on upgrade — exactly the failure mode in
`project_id_requires_replace_bug.md`.

### 3.9 Test-naming convention is load-bearing

Every provider we surveyed uses a name-prefix taxonomy that CI greps over.
We commit to:

- `TestUnit*` — unit tier (no `TF_ACC`).
- `TestIntegration*` — integration tier (no creds, no `TF_ACC`, runs against fake
  server via `resource.UnitTest`).
- `TestAcc*` — live acceptance against real cloud + real Redpanda Cloud.
- `TestUpgrade*` — provider-upgrade tests against the previously-released provider.

Snowflake's `pkg/architests/` (Go tests that fail CI if a function has the
wrong prefix) is overkill at our size. CODEOWNERS + review suffices.

---

## 4. Patterns NOT to adopt

- **AWS-style "no mocking ever" policy.** They have paid AWS infra and 30+
  owning teams; we don't.
- **AWS-style 40+ GitHub Actions workflows.** `task ready` covers most of
  the equivalent locally.
- **AWS's `@Testing(...)` annotation soup driving codegen.** `schemagen`
  YAML already plays that role.
- **TeamCity (AWS) / Semaphore (Confluent)** for CI orchestration.
  BuildKite is enough.
- **Drift-detection tests against a canary** (Confluent pattern). Out of
  scope — the manual testing rig is the discovery point for drift.
- **Nightly cron** for live-acc, sweepers, or anything else. Live runs are
  human-triggered.
- **`resource.AddTestSweepers`**. We want deterministic per-job cleanup,
  not a separate scheduled sweep tier.
- **Snowflake's 229 flat files in one directory.** Discoverability suffers.
- **Snowflake's numeric file prefixes (`0_*`, `10_*`, `20_*`).** Doesn't
  scale past a few phases.
- **HCP's "no sweepers" policy.** They get away with it because HCP doesn't
  bill for leaked staging clusters. We do.
- **TFE's exclusive reliance on CI-matrix parallelism (no `t.Parallel`).**
  Right for them (free workspaces); wrong for us (paid clusters).
- **TFE's 4,242-line `resource_test.go` with 66 near-identical HCL
  builders.** Inline `fmt.Sprintf` HCL is fine, but at that scale you need
  a `*_base` composition pattern (AWS's `ConfigCompose`) or a builder layer.
- **Snowflake's `bettertestspoc` typed HCL builders** — *for now*. The
  cost-benefit only kicks in past ~20 resources with very deep schemas.
- **Confluent's `/state-sync` dummy endpoint** for cross-container WireMock
  state. Cleverness that bites maintainers.
- **MongoDB Atlas's package-level `sharedInfo` mutable state**. Forbids
  cross-package shared clusters; fragile under parallel test execution.

---

## 5. Recommended methodology

### 5.1 Test tiers

| Tier | Name prefix | Build tag | Creds | When |
|---|---|---|---|---|
| Unit | `TestUnit*` | none | none | Every PR, every push |
| Integration | `TestIntegration*` | none | none | Every PR |
| Live acceptance | `TestAcc*` | `live_test && (all \| <group>)` | `TF_ACC=1` + cloud + Redpanda | PR label, by hand |
| Provider upgrade | `TestUpgrade*` | `upgrade` | `TF_ACC=1` + cloud + Redpanda | PR label, by hand |

`TF_ACC=1` is the line between "doesn't touch a Terraform runtime" and
"runs `resource.TestCase`." The framework enforces it for us via
`resource.Test`.

No nightly, no drift checks, no scheduled sweeper. Every cloud-touching run
is human-triggered via a PR label (§5.5).

### 5.2 Directory layout (colocated)

Every test that targets a single resource lives in that resource's package.
Cross-resource scenarios (matrix runners that exercise multiple resource
types in one apply) stay in `redpanda/tests/`.

```
redpanda/
  resources/<resource>/
    resource_<resource>.go
    resource_<resource>_test.go              # unit (TestUnit*) — gomock
    integration_<resource>_test.go                  # integration (TestIntegration*) — fake server
    acc_<resource>_<cloud>_test.go           # live (TestAcc*) — per-cloud variants
    upgrade_<resource>_test.go               # provider-upgrade (TestUpgrade*)
  tests/
    runner_*_test.go                         # cross-resource matrix runners (cross-cutting only)
  internal/testutil/
    acc/      factory.go, pre_check.go, skip.go, cleanup_registry.go
    mock/     server.go, fakes/<service>.go
    integration/  setup.go, steps.go
    upgrade/  external_providers.go
```

### 5.3 Mocking strategy

- **Unit tier**: gomock clients from `redpanda/mocks/`. Cut at the Go
  interface. Keep doing what works.

- **Integration tier**: bufconn-backed gRPC fake server. Per-service fakes
  live under `internal/testutil/mock/fakes/` (`cluster.go`, `network.go`,
  `topic.go`, `user.go`, `acl.go`, `pipeline.go`, `secret.go`, `shadow_link.go`,
  `serverless_cluster.go`, `serverless_private_link.go`, `service_account.go`,
  `resource_group.go`, `region.go`, `serverless_region.go`,
  `throughput_tier.go`, `operation.go`, `security.go`, `schema_registry.go`).
  Each implements the gRPC server interface for one service and holds
  in-memory state. The provider's dialer is pointed at the bufconn listener
  during test configuration. There is **no recording step** — the typed
  handler interface and the proto schema together define the wire contract.
  Fakes are stateful by design: Create stores, Read retrieves, Update
  mutates, Delete removes.

- **Live tier**: real Redpanda Cloud + real AWS/GCP/Azure. Unchanged.

### 5.3a Integration tier (current implementation)

The integration tier runs the full `terraform-plugin-testing` harness
against an in-process gRPC fake (bufconn) and an `httptest.Server` for
Schema Registry REST. No cloud credentials required.

**Test files and naming**

- Files: `redpanda/resources/<r>/integration_<r>_test.go`
- Functions: `TestIntegration_<Resource>_<Scenario>`
- Package: `<r>_test` (external package, breaks the root-provider import cycle)

**Running**

```sh
# all resources
task test:unit

# single resource
task test:unit -- -run TestIntegration_<Resource>
```

Integration tests use `resource.UnitTest` and are included in `task test:unit`.
No `TF_ACC=1` or cloud credentials required.

**Harness library — `internal/testutil/integration/`**

`Setup(t)` wires the env var and provider factory to a bufconn-backed
`mock.Server` and returns the server plus a provider factories map ready
for `resource.TestCase`.

Step builders:

| Builder | What it asserts |
|---|---|
| `CreateStep(addr, cfg, stateChecks)` | `ExpectResourceAction(Create)` pre-apply, `ExpectEmptyPlan` post-apply |
| `NoopReapplyStep(addr, cfg, stateChecks)` | `ExpectEmptyPlan` both before and after apply |
| `UpdateLeafStep(addr, cfg, stateChecks)` | `ExpectResourceAction(Update)` pre-apply, `ExpectEmptyPlan` post-apply |
| `RequiresReplaceStep(addr, cfg, stateChecks)` | `ExpectResourceAction(DestroyDeferred)` pre-apply, `ExpectEmptyPlan` post-apply |
| `RequiresReplaceIfStep(addr, cfg, triggersReplace, stateChecks)` | Replace or Update depending on `triggersReplace` |
| `ImportRoundTripStep(addr, idFunc, verifyIgnore)` | Import then verify state matches |
| `ErrorPathStep(srv, method, code, cfg, errPattern)` | gRPC error propagated as expected Terraform error |
| `RESTErrorPathStep(sr, method, path, status, body, cfg, errPattern)` | HTTP error from Schema Registry REST propagated correctly |

**Mutation assertion discipline**

Every leaf mutation must:
1. Pin state across the two steps with `statecheck.CompareValue` using
   `compare.ValuesSame()` or `compare.ValuesDiffer()`, sharing one
   `statecheck.CompareValuePair` instance via `AddStateValue` so both
   steps reference the same stored value.
2. Guard the plan with `plancheck.ExpectResourceAction` before apply.
3. Assert `plancheck.ExpectEmptyPlan` after apply.

**When to use integration vs live acc**

Use integration for: schema correctness, Flatten/Expand conversions, plan
modifiers (RequiresReplace, UseStateForUnknown), error propagation.

Use live acc for: real cluster lifecycle, BYOC/BYOVPC infra, eventual-
consistency timing, anything that genuinely requires a running Redpanda
Cloud cluster.

### 5.4 Sweepers: deterministic, per-job

Two layers, both deterministic, both run at the end of every job:

- **Per-package `TestMain`** in `redpanda/tests/` (and any other package
  that hits cloud). `TestMain(m)` calls `m.Run()` then
  `acc.Cleanup(ctx)`, which walks a registry of resources created via
  `internal/testutil/acc/` helpers and tears them down in dependency
  order. Test bodies don't need explicit `t.Cleanup` for registry-managed
  resources; ad-hoc `t.Cleanup` remains fine for everything else.

- **CI postscript**: every live-acc job's final step (`always-runs: true`)
  is `task cleanup:<cloud>:ci` + `task cleanup:redpanda`. Existing
  taskfile entries already do the work; we just wire them as the
  job-terminator.

No `resource.AddTestSweepers`. No `make sweep`. If something leaks past
both layers, the manual testing rig is the discovery point.

### 5.5 CI gating (label-triggered)

- **Every PR push**: unit + integration + lint. $0, minutes.
- **`ci-ready` label**: triggers the standard live-acc gate (cluster + network + service_account + datasource_cluster) against AWS + GCP.
- **`ci-ready-byoc` label**: triggers the BYOC + BYOVPC live-acc suite (AWS + GCP).
- **`ci-ready-serverless` label**: triggers the serverless live-acc suite.
- Every live-acc job ends with the cleanup postscript (§5.4) regardless of test outcome.
- Provider-upgrade tests (`TestUpgrade_*`) currently run only via `task test:upgrade` locally; a pipeline step is not yet wired (tracked as a follow-up).

No cron. No nightly. No matrix expansion the human didn't ask for.

### 5.6 Fixtures (HCL)

- Inline `fmt.Sprintf` HCL with indexed placeholders (`%[1]q`, `%[2]s`) is
  the default. AWS pattern.
- `ConfigCompose(base, specific)` helpers in `internal/testutil/acc/`
  share scaffolding (resource_group, network) across tests.
- One named resource per HCL block (`resource "redpanda_cluster" "test"`)
  so `resourceName` is stable.
- Defer typed HCL builders (Snowflake `bettertestspoc`) until a concrete
  pain point demands them.

### 5.7 Cost controls

- `isAcceptanceTestMode` flag through the provider config (§3.6).
- Per-cloud service-principal isolation (HCP pattern): one Redpanda Cloud
  project + one IAM role per cloud, secrets scoped per label-triggered
  workflow.
- Shared cluster reuse for Kafka-level resources (topics, ACLs, users,
  roles, schemas): `KAFKA_CLUSTER_ID` env override on the
  `acc.ProjectExecution`-style helper so a developer can iterate locally
  without paying for create+destroy.
- Consolidated `TestCase`s (§3.7): one cluster, many `TestStep`s
  exercising resource + datasource + dependent resources.

### 5.8 Provider-upgrade tests

- `internal/testutil/upgrade/external_providers.go` exposes
  `upgrade.CreateAndRunTest(t, basicTestCase)`. Step 0 uses
  `ExternalProviders` to fetch the previously-released `redpanda-data/redpanda`
  from the Registry, version from `REDPANDA_LAST_VERSION`. Step 1 swaps in
  the local build with `ExpectEmptyPlan`.
- One `TestUpgrade*` per resource that has had a schema change in the last
  two minor releases.
- Pre-flight rejects `TF_CLI_CONFIG_FILE` (Atlas pattern) so a dev-override
  doesn't silently mask the released provider.

### 5.9 Skipping

Env-var gates and the `live_test` / `upgrade` build tags. Helpers in
`internal/testutil/acc/skip.go`:

- `acc.SkipUnlessAWS(t)`, `SkipUnlessGCP(t)`, `SkipUnlessAzure(t)`
- `acc.SkipUnlessByoc(t)`, `SkipUnlessByovpc(t)`
- `acc.SkipUntil(t, "2026-09-01", "broken upstream, recheck Q3")` —
  self-destructing skip (TFE pattern); becomes `t.Fatalf` past the date.

### 5.10 Plugin Framework specifics

- Mux factory in `internal/provider/factory.go`, used by both `main.go`
  and every test (TFE pattern). Costs nothing today, makes any future
  migration leg free.
- Use `terraform-plugin-testing` `plancheck` and `statecheck` for
  assertions beyond raw attribute matches. HCP's
  `plancheck.ExpectResourceAction(name, plancheck.ResourceActionUpdate)`
  is the canonical "this step is an in-place update, not a replacement"
  guard.
- `ImportStateVerifyIdentifierAttribute` for resources with non-`id`
  import keys.

---

## 6. Migration plan

Ordered by impact-per-effort, not chronologically. Each step is
independently shippable.

1. **Mux factory** in `internal/provider/factory.go`. Touch `main.go` and
   the existing `redpanda/tests/` provider setup. Trivial; unblocks 4-6.
2. **Colocate existing live tests**. Move per-resource `acc_*_test.go`
   files from `redpanda/tests/` into `redpanda/resources/<resource>/`
   so subsequent steps land in the right layout from the start. Cross-
   resource `runner_*_test.go` files stay in `redpanda/tests/`. Done
   early so integration work in 4-7 doesn't need to be moved later.
3. **`isAcceptanceTestMode` flag** through the provider config and
   `redpanda/utils/retry.go`. Required before integration runs in reasonable
   wall-clock time.
4. **Proto-based fake server skeleton** in `internal/testutil/mock/`.
   Start with `controlplanev1` (resource_group, network, region/regions,
   cluster). Stateful in-memory store per fake.
5. **First `TestIntegration*`** end-to-end on `redpanda_resource_group` (smallest
   surface, cheapest fake). Prove the round-trip works through the real
   framework provider.
6. **Backfill integration** for the cheap, fast resources we already test
   live: `redpanda_topic`, `redpanda_acl`, `redpanda_user`,
   `redpanda_role`, `redpanda_role_assignment`, `redpanda_schema`,
   `redpanda_schema_registry_acl`, `redpanda_service_account`,
   `redpanda_secret`.
7. **Per-package `TestMain` + cleanup registry** in
   `internal/testutil/acc/`. Migrate the (now-colocated) `acc_*_test.go`
   files to register resources via the registry; remove ad-hoc
   `t.Cleanup` where the registry covers it.
8. **CI postscript cleanup**. Add an always-runs final step to every
   live-acc and migration job: `task cleanup:<cloud>:ci` +
   `task cleanup:redpanda`.
9. **Build-tag groups**. Convert taskfile targets so
   `task test:cluster:aws` becomes
   `go test -tags=live_test,cluster_aws ./redpanda/resources/cluster/...`
   under the hood; the user-facing taskfile names stay stable.
10. **Label-triggered live workflows** in BuildKite: `ci-ready` (standard
    cluster + network + service_account + datasource_cluster), `ci-ready-byoc`
    (BYOC + BYOVPC), and `ci-ready-serverless` (serverless suite). Each maps
    to one build-tag group + the cleanup postscript.
11. **`upgrade` package** with `internal/testutil/upgrade/external_providers.go`
    and one `TestUpgrade*` per resource that has had a schema change in the
    last two minor releases. Pre-flight rejects `TF_CLI_CONFIG_FILE`.

---

## 7. References

Per-provider notes:
- `.claude/testing-research/aws.md`
- `.claude/testing-research/hcp.md`
- `.claude/testing-research/tfe.md`
- `.claude/testing-research/confluent.md`
- `.claude/testing-research/mongodbatlas.md`
- `.claude/testing-research/snowflake.md`

Canonical external docs:
- AWS `docs/running-and-writing-acceptance-tests.md`
- HCP `contributing/writing-tests.md`
- Confluent `docs/LIVE_TESTING_TEMPLATE.md`, `docs/DRIFT_TEST_TEMPLATE.md`
- MongoDB Atlas `contributing/testing-best-practices.md`
- Snowflake `CONTRIBUTING.md`, `pkg/acceptance/bettertestspoc/README.md`
