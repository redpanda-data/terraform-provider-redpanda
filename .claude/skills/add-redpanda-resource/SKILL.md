---
name: add-redpanda-resource
description: Use when scaffolding a brand-new resource or datasource in the terraform-provider-redpanda repo. Triggers on "add resource", "new datasource", "scaffold redpanda <thing>", or any work that creates a fresh redpanda/resources/<name>/ directory. Covers schema yaml, schemagen wiring, model generation, hand-written CRUD glue, provider registration, unit tests, acceptance tests, examples, and docs end-to-end. For modifying an existing resource use extend-redpanda-resource instead.
---

# Adding a resource or datasource

The pipeline is **YAML → schemagen → generated Go → hand-written CRUD glue + tests**. Author one `schema.yaml` (and optionally `schema_datasource.yaml`), wire it into the central `//go:generate` registry, run `task generate:models`, then hand-write CRUD methods and tests.

Each phase has a focused reference doc under `../_shared/`. Don't try to hold the whole flow in one head — open the relevant `_shared/` file when you reach each step.

## 0. Confirm scope

A fresh resource means a new `redpanda/resources/<name>/` directory. If the package already exists, **stop and use `extend-redpanda-resource` instead** — the workflows are deliberately different.

Confirm with the user:
- Resource name (the `redpanda_X` Terraform name, e.g. `redpanda_service_account`)
- Whether you're adding a resource, a datasource, or both
- The proto message (`redpanda.api.controlplane.v1.<Msg>`) — usually obvious from the cloudv2 sibling repo
- Cluster scope: control-plane (cluster lifecycle, billing) vs dataplane (per-cluster: topic, user, acl)

## Plan first: use the sonnet exploration pattern

Per memory `feedback_sonnet_agent_exploration_pattern.md` — scaffolding a new resource touches every phase (schema, codegen, CRUD, registration, tests, examples, docs, manual validation). Don't try to hold all of that in working memory.

1. Spawn sonnet agents in parallel to explore. One per phase the user hasn't already nailed down, e.g.:
   - "How does serviceaccount handle the `flatten_skip:` + Create-only pattern? Find the exact code."
   - "What does the existing cluster fake in `internal/testutil/mock/fakes/` look like? What contract does a new fake need to satisfy?"
   - "Pick the closest existing resource shape (control-plane vs dataplane, with/without datasource, async/sync RPC) and document its file layout."
2. Have agents write findings to `manual-tests/<name>-research/` (gitignored).
3. Consolidate into `manual-tests/<name>-research/CONSOLIDATED.md` with a proposed file-by-file plan, commit shape, and any open questions.
4. **Present the plan to the user and get sign-off** before any YAML, codegen, or test work.
5. Then execute against the consolidated facts.

Skip this only for resources that are clearly minimal (single scalar attribute, no nesting, sync RPC, no async) AND you can name the closest analog in the existing code from memory. For most new resources, the exploration pass saves more time than it costs.

## 1. Schema YAML

Author `redpanda/resources/<name>/schema.yaml`. See [`../_shared/schema-authoring.md`](../_shared/schema-authoring.md) for the directive index, lifecycle classifications, validators, and the deprecation/alias pattern.

Datasources also need `schema_datasource.yaml` with `computed_default: true`.

Pull descriptions from cloudv2 by running `task generate:apidescriptions` first if the proto change is new. Inline `description:` overrides only for gaps.

## 2. Provider registration

Three parallel registration edits — see [`../_shared/provider-registration.md`](../_shared/provider-registration.md):

1. `redpanda/redpanda.go` — append factory lambda to `Resources()` and/or `DataSources()`, add import
2. `redpanda/resources/schemagen.go` — append a new `//go:generate` line
3. `redpanda/resources/schema_golden_test.go` — append a new entry to the `tests` slice

## 3. Generate

```bash
task generate:models
```

See [`../_shared/codegen-workflow.md`](../_shared/codegen-workflow.md) for what files get produced, classifier diagnostics to watch for, and the golden baseline workflow.

Four files appear per directive:
- `redpanda/resources/<name>/schema_resource_gen.go`
- `redpanda/resources/<name>/proto_validator_gen.go`
- `redpanda/models/<name>/resource_model_gen.go`
- `redpanda/models/<name>/conv_gen.go`

Then create the golden baseline:

```bash
task generate:golden    # requires user approval per memory feedback_golden_files.md
```

## 4. CRUD glue

Hand-write `redpanda/resources/<name>/resource_<name>.go` (and `datasource_<name>.go` if applicable). See [`../_shared/crud-glue.md`](../_shared/crud-glue.md) for the resource skeleton, Create/Read/Update/Delete/ImportState patterns, FieldMask update flow, async polling, and the `flatten_skip:` + Create-only field pattern.

For hand-written conversion functions referenced by `flatten_via:` / `expand_via:` directives, write them in `redpanda/models/<name>/conv.go`.

## 5. Tests

### Hard rule: every leaf in the schema must be exercised in Tier 2

Per memory `feedback_test_all_leaves.md`: every attribute that appears in the resource's `*.golden` file — at every nesting level — must be covered by the colocated integration test (Tier 2). Coverage means at least a `stateChecks` assertion (value or explicit null) in the Create step, plus an UpdateLeaf step if mutable, plus a NestedMatrix entry if it lives inside a block.

Missing leaves cause `Error: Provider produced inconsistent result after apply` at runtime — opaque, painful for users, and only surfaces when someone happens to set the uncovered attribute. The integration tier exists specifically to catch these before they ship.

**Verification step after regen**: list the leaves from `redpanda/resources/testdata/<name>_resource_schema.golden`. Walk each one and confirm test coverage. Don't skip this step.

If a leaf genuinely cannot be exercised by the bufconn fake (e.g. requires real-cluster state the fake can't model), **stop and inform the user** before proceeding:

> "I cannot cover leaf `X.Y.Z` in Tier 2 because [specific reason]. Skipping it leaves a code path unverified — users setting this attribute may hit `Provider produced inconsistent result after apply`. Do you authorize the skip?"

Authorization does not carry forward to other leaves. Ask once per leaf.

### Tier structure

Four tiers, top to bottom — see [`../_shared/testing-tiers.md`](../_shared/testing-tiers.md):

- **Tier 1** — Model unit tests in `redpanda/models/<name>/*_test.go` (internal package). Test hand-written `conv.go` functions and any Flatten/Expand behavior with nuance.
- **Tier 2** — Colocated integration test in `redpanda/resources/<name>/integration_<name>_test.go` (external `<name>_test` package). The day-to-day tier. Add a fake to `internal/testutil/mock/fakes/<service>.go` and register in `mock.Server`. Use `integration.Setup(t)` and the step builders. Cover `Create → NoopReapply → Update → RequiresReplace → Import → Error paths`.
- **Tier 3** — Colocated live-acc test in `redpanda/resources/<name>/acc_<name>_test.go` (build tag `live_test && (all || <name>)`). Add fixtures to `internal/testutil/acc/fixtures.go`, a task target to `.tasks/test.yml`. Sweeper in `internal/testutil/acc/sweep/` only if the resource can leak infra.
- **Tier 4** — Global live-acc (`redpanda/tests/runner_*_test.go`) — usually skip; most new resources don't need this.

Tier 2 covers most CRUD coverage that used to require live cluster work. Tier 3 is for behavior the bufconn fake can't model (real API timing, rate limits, server-enforced constraints).

## 6. Examples and docs

See [`../_shared/docs-and-examples.md`](../_shared/docs-and-examples.md). Add:

- `examples/<name>/main.tf` + `variables.tf` — acc test fixture
- `examples/docs/<name>/main.tf` — doc-page example
- `examples/datasource/<name>/main.tf` — if datasource exists
- `templates/resources/<name>.md.tmpl` — optional but recommended; copy from a similar-shape resource

Then `task docs` to regenerate `docs/resources/<name>.md`.

## 7. Pre-commit gate

```bash
task ready    # docs + lint + go mod tidy
```

If lint fails after codegen, `task lint:fix`. If golden fails, regen via `task generate:golden` (with user approval) and commit the baseline diff in the same commit as the schema.

## 8. Manual validation

See [`../_shared/manual-validation.md`](../_shared/manual-validation.md). For greenfield resources, **skip the upgrade scenario (§0a-0c)** — there's no prior state to migrate. Run the standard CRUD sequence (§1-10):

1. Plan → Apply → feature check → No-op plan
2. Update mutable field → Apply → No-op plan
3. State rm → Import → No-op plan
4. Drift mutation → Plan detects

Use the user-level `manual-test-redpanda-resource` skill for the detailed recipe and templates.

## When you get stuck or surprised

Per memory `feedback_ask_when_unsure.md`: when the actual state diverges from what your plan assumed (a file isn't where you expected, an existing pattern doesn't match what you were going to copy, codegen output isn't what you predicted), **stop and ask** with concrete options. Don't guess your way out — guesses compound, and the right answer is usually a small clarification away.

Per memory `feedback_test_first_bug_proof.md`: if you find a bug during the add work (existing behavior is wrong, not just incomplete), write a failing test that proves it first. Show the user the red, then discuss fix scope separately. Don't roll the bug fix into the add silently.

## Don'ts (project-specific)

- **Don't hand-edit `*_gen.go` files** — fix the generator or the YAML.
- **Don't add `//nolint` or `#nosec`** without explicit user approval (memory `feedback_no_nolint_without_permission.md`).
- **Don't reorder existing schema YAML entries** when editing — diff noise hides real changes.
- **Don't add code comments** beyond load-bearing traps (memory `feedback_no_code_comments.md`).
- **Don't `git push`** without explicit per-push approval (memory `feedback_no_push_without_permission.md`).
- **Don't run `task generate:golden` without explicit user approval** — goldens are sacred (memory `feedback_golden_files.md`).
- **Don't fabricate parallel test functions** — for a new resource, write the *minimum* set of test functions needed to cover the four tiers. If you find yourself adding `TestIntegration_<Name>_Extra` or `TestIntegration_<Name>_v2`, stop and ask whether it belongs as additional steps in the primary lifecycle test (memory `feedback_extend_existing_tests.md` — the rule applies less aggressively for new resources, but the duplication antipattern is the same).
- **Don't skip leaves in Tier 2 integration tests** — every leaf in the resource's golden must be exercised. Skipping a leaf requires explicit user authorization after they're informed of the runtime inconsistency risk (memory `feedback_test_all_leaves.md`).

## Commit shape

Per memory `feedback_review_shape_commits.md`:
- Generated files (`*_gen.go`, `.golden`, `.description`, regenerated `docs/`) get their own commit, **placed last** so reviewers can skim and skip.
- For new-resource scaffolding, prefer one bundled PR over several small ones (memory `feedback_single_pr_with_folded_fixes.md`).

Suggested commit order:
1. Schema YAML + provider registration + schemagen line + golden_test entry
2. CRUD glue + hand-written conv.go (if any)
3. Tests (Tier 1, 2, 3 as applicable)
4. Examples + templates
5. **Generated:** `*_gen.go` + `*.golden` + `*.description` + `docs/`
