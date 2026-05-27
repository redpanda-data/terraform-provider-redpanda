---
name: extend-redpanda-resource
description: Use when adding or modifying fields/attributes on an EXISTING resource or datasource in terraform-provider-redpanda. Triggers on "add field X to redpanda_<name>", "extend the topic resource", "new attribute on redpanda_cluster datasource". For brand-new resources or datasources (no existing package), use add-redpanda-resource instead.
---

# Extending an existing resource or datasource

Adding a field looks simple — it isn't. Schema-evolution failure modes (force-replacement on existing state, perpetual `(known after apply)` diffs from server-returns-null fields, breaking-change upgrades for existing users) don't surface until users upgrade. The upgrade scenario in §6 is the load-bearing check; don't skip it.

For brand-new resource scaffolding (no existing `redpanda/resources/<name>/` package), **stop and use `add-redpanda-resource` instead.**

## Hard rule: extend existing tests, do not create new ones

Per memory `feedback_extend_existing_tests.md` — this is the rule agents most reliably break.

When extending a resource, the test default is **modify the existing test**, not create a new one. Add a `stateChecks` entry. Append a step. Extend a table-driven case. Open the closest existing test function and grow it.

Creating a new test function (`TestIntegration_<Name>_NewFieldBehavior`) or a new test file (`integration_<name>_v2_test.go`, `field_<x>_test.go`) is **only permitted** when both of the following hold:

1. There is an extremely compelling justification — the new behavior is genuinely orthogonal and would require restructuring multiple existing tests to fit. "It's cleaner as a separate test" is not sufficient.
2. The user is **explicitly informed of this rule** in the current session and **authorizes** the new test before you create it.

Implicit authorization from earlier in the session does not carry forward. Ask again, in this specific case, before creating a new test artifact.

If you find yourself reaching for a new `TestIntegration_<Name>_*` function: stop, open the existing `TestIntegration_<Name>_CreateAndRefresh` or `_NestedMatrix_*`, and add to it. Almost every case fits.

## Plan first: use the sonnet exploration pattern for non-trivial extends

Per memory `feedback_sonnet_agent_exploration_pattern.md` — when the extend is non-trivial (touches a complex nested block, requires careful YAML directive selection, has cross-resource implications, or you don't already know the relevant code paths cold):

1. Spawn 1–2 sonnet agents in parallel to explore. Common phases to delegate:
   - Schema/codegen: "How is field X currently modeled? Which directives apply? Where does Flatten/Expand land?"
   - Tests: "Which existing tests cover the surrounding behavior? Where should the new field's assertion live?"
   - Cross-resource: "Does this field have analogs in other resources? What patterns do they use?"
2. Have agents write findings to `manual-tests/<topic>/` (gitignored).
3. Consolidate into `manual-tests/<topic>/CONSOLIDATED.md` with a proposed plan + commit shape.
4. **Present the plan to the user and get sign-off** before any YAML edit or regen.
5. Then execute.

Skip this for trivial extends (one scalar field with an obvious lifecycle and no nested implications) — but err on the side of using it. Catching a wrong YAML directive at the plan stage is much cheaper than catching it after a golden update.

## 0. Confirm golden baseline exists

Before any YAML edit, confirm `redpanda/resources/testdata/<name>_(resource|datasource)_schema.golden` exists for every schema you're touching. Without an anchored baseline, the `TestSchemaGolden` parity check has no anchor and a "successful" generation can silently change the schema users will see on upgrade.

If a golden is missing (resource pre-dates schemagen or never got one wired in):

```bash
git worktree add /tmp/tfrp-baseline main
cd /tmp/tfrp-baseline
task generate:golden
cp redpanda/resources/testdata/<name>_*.golden <repo>/redpanda/resources/testdata/
```

Commit the baseline as a "no behavioral change" commit before starting the extend work.

## 1. Schema YAML edit

Edit `redpanda/resources/<name>/schema.yaml` (and `schema_datasource.yaml` if also extending the datasource). See [`../_shared/schema-authoring.md`](../_shared/schema-authoring.md) for directive choices.

Extend-specific discipline:

- **Don't reorder existing entries.** Insert into the existing tree in proximity-relevant order — diff noise hides the real change.
- **Required: true on a new attribute is a breaking change** for existing users. Confirm with the user before adding. Use `optional: true` or `optional+computed` instead.
- **If the field was `todo: true`**, the edit is a replacement: remove `todo: true` and add the correct lifecycle.
- **Server-defaulted proto fields** → `optional+computed+UseStateForUnknown` (memory `feedback_server_default_optional_computed.md`). The classifier auto-emits `UseStateForUnknown` for top-level optional+computed; only override when the server returns null and would create perpetual churn.

If the proto field is new, `task generate:apidescriptions` first to pull the description from cloudv2. Add inline `description:` only for gaps.

## 2. Regenerate

```bash
task generate:models
```

See [`../_shared/codegen-workflow.md`](../_shared/codegen-workflow.md) for diff review discipline.

A clean small-field extend touches exactly:
- One new attribute in `schema_resource_gen.go`
- One new struct field in `resource_model_gen.go`
- One new Flatten block in `conv_gen.go`
- The new field in `GenerateMinimalResourceModel` and `ExpandCreate`/`ExpandUpdate`

**Anything else changed in the diff is a red flag** — investigate before committing. A one-line YAML change can cascade silently into nested sub-converters.

Verify the golden:

```bash
go test ./redpanda/resources/ -run TestSchemaGolden
```

If it fails legitimately, update with explicit user approval:

```bash
go test ./redpanda/resources/ -run "^TestSchemaGolden$/^<name>_(resource|datasource)$" -update -descriptions
```

Review the golden diff line-by-line. One new attribute line, one new description line. Larger diff = unintended cascade.

## 3. CRUD glue — usually none

See [`../_shared/crud-glue.md`](../_shared/crud-glue.md). For normal proto-mapped fields (scalar, nested, enum, list), `Flatten`/`ExpandCreate`/`ExpandUpdate` are fully regenerated. **Touch nothing in `resource_<name>.go`.**

Touch CRUD only when the new field:

1. Is `extra: true` (TF-only, no proto echo) — needs population in Create and/or ImportState
2. Has a custom `flatten_via:` / `expand_via:` — write the function in `redpanda/models/<name>/conv.go`
3. Changes FieldMask Update behavior (rare; cluster only)
4. Needs `ImportState` default — use `utils.ImportStateBoolFromSchemaDefault` for bool extras

For most field additions, you skip this section entirely.

## 4. Provider registration

N/A for extend. The factory function, `//go:generate` line, and golden_test entry are pre-existing.

## 5. Tests

See [`../_shared/testing-tiers.md`](../_shared/testing-tiers.md). **Re-read the "Hard rule: extend existing tests" section at the top of this file before touching any test.** New test functions or files require explicit user authorization.

### Hard rule: the new leaf must be exercised in Tier 2 — no exceptions without informed authorization

Per memory `feedback_test_all_leaves.md`: the new attribute you're adding must be covered by the colocated integration test (Tier 2). Coverage means at minimum a `stateChecks` assertion in the Create step (value or explicit null), plus exercise in an UpdateLeaf step if mutable, plus a NestedMatrix entry if it sits inside a block.

This rule is non-negotiable because skipped leaves cause `Error: Provider produced inconsistent result after apply` at runtime — only surfaced when a user happens to set the attribute. The integration tier exists specifically to catch these before they ship.

Additionally, before considering the extend complete, **audit the surrounding leaves**: a refactor or YAML edit can silently drop a `stateChecks` line. List leaves from `redpanda/resources/testdata/<name>_resource_schema.golden` and walk each to confirm coverage is still present. Missing coverage for a pre-existing leaf is just as much of a regression as a missing test for the new one.

If the new leaf or a surrounding leaf genuinely cannot be exercised in Tier 2 (the bufconn fake doesn't model the required state), **stop and inform the user** before proceeding:

> "I cannot cover leaf `X.Y.Z` in Tier 2 because [specific reason]. Skipping it leaves a code path unverified — users setting this attribute may hit `Provider produced inconsistent result after apply`. Do you authorize the skip?"

Authorization is per-leaf and does not carry forward.

### The extend pattern by tier

- **Tier 1 (model unit, `redpanda/models/<name>/*_test.go`)** — extend existing `*_matrix_test.go` files. Add a new matrix test file *only* with user authorization, and *only* when the new field has behavioral nuance (null-vs-present, prev-preservation, UseStateForUnknown contract) that can't fit into an existing matrix. For routine scalar fields, skip Tier 1 — Tier 2 covers it.
- **Tier 2 (colocated integration, `integration_<name>_test.go`)** — the workhorse for extend tests.
  - **Default**: add a `stateChecks` entry to the existing `TestIntegration_<Name>_CreateAndRefresh` (or equivalent primary test) for presence/value assertions.
  - **Default for mutable fields**: add a step to an existing `TestIntegration_<Name>_UpdateLeaf_*` function, not a new function.
  - **Default for nested-block fields**: add to the existing `TestIntegration_<Name>_NestedMatrix_<Block>_Dense`.
  - **Adding a new test function** (`TestIntegration_<Name>_UpdateLeaf_<NewField>`) requires user authorization per the hard rule above. Explain to the user why an existing function can't fit before asking.
  - **Never create a new `integration_<name>_test.go` file** when one exists.
- **Tier 3 (colocated live-acc, `acc_<name>_test.go`)** — usually unchanged for extend. Only update if the new field requires real cluster behavior. Same hard rule applies: extend, don't duplicate.
- **Upgrade test (`upgrade_<name>_test.go`)** — the existing config should **not** include the new field. Empty plan with field absent from config is the load-bearing assertion. If you genuinely need a field-set upgrade scenario, request authorization for a new test function before writing it.

## 6. Review pass

Before any commit, read every diff hunk line-by-line:

- Schema YAML: tight, surgical, no reordering
- `conv_gen.go`: only the expected new Flatten/Expand blocks
- `*.golden`: one new attribute, one new description
- `docs/<name>.md`: the new field shows up in `## Schema`
- Tests: integration test covers the new field's primary behavior

This pass is more important on extend than on add because the surrounding code is already production. A small unwanted cascade is easy to ship by accident.

## 7. Examples and docs

See [`../_shared/docs-and-examples.md`](../_shared/docs-and-examples.md). Most extends:

- **No `examples/<name>/main.tf` change** — existing config still works unless the field is Required (which it shouldn't be on extend).
- **No `examples/docs/<name>/main.tf` change** — leave alone unless the new field is prominently user-facing and Required-like.
- **`task docs` always required** — the regenerated `docs/resources/<name>.md` will pick up the new attribute in the schema section automatically. Commit the regen diff in the same commit as the schema YAML.

## 8. Pre-commit gate

```bash
task ready    # docs + lint + go mod tidy
```

Per memory `feedback_lint_before_commit.md`. Address any lint findings immediately.

## 9. Manual validation — upgrade scenario (load-bearing)

See [`../_shared/manual-validation.md`](../_shared/manual-validation.md). The §6 upgrade scenario from the user-level `manual-test-redpanda-resource` skill is the **only** check that cannot be replaced by automated tests:

**0a.** Apply with released provider, config **without** the new field. Represents existing users.

**0b.** Switch to dev binary (`hashicorp/redpanda` via dev_overrides), run `terraform state replace-provider`, re-plan. Must show **No changes**. Any diff = backwards-compat break.

**0c.** Add the new field to config, apply. Subsequent plan reports No changes.

What "no spurious diff" means in 0b:

- `optional+computed` with `UseStateForUnknown` — anchors null-in-state; no diff
- `computed_only` — appears as `(known after apply)` on first refresh, stable after; no diff
- `extra: true` — null in old state; default value must match null. Non-null default = backwards-compat bug

`terraform state replace-provider registry.terraform.io/redpanda-data/redpanda hashicorp/redpanda` is the most common trip-hazard. Without it, plan fails with "Missing required provider."

Cluster reuse is even more important for extend than for add — per memory `feedback_keep_cluster_alive.md`, never destroy between cycles. Per `feedback_summary_md_detail.md`, log a detailed `manual-tests/SUMMARY.md` entry tagged as "extension test."

## When you get stuck or surprised

Per memory `feedback_ask_when_unsure.md`: when the actual state diverges from what your plan assumed (a directive doesn't behave as expected, the regen diff isn't what you predicted, a test you thought existed doesn't), **stop and ask** with concrete options. Extend work is small-blast-radius by nature — there's no premium on charging ahead with a misread.

Per memory `feedback_test_first_bug_proof.md`: if you find a bug in the existing resource during extend work (not just an incomplete feature), write a failing test that proves it first. Show the user the red, then discuss fix scope separately. Don't roll the bug fix into the extend silently.

## Don'ts (project-specific)

- **Don't hand-edit `*_gen.go` files** — fix the generator or the YAML.
- **Don't add `//nolint` or `#nosec`** without explicit user approval (memory `feedback_no_nolint_without_permission.md`).
- **Don't run `task generate:golden` without explicit user approval** — goldens are sacred (memory `feedback_golden_files.md`).
- **Don't add code comments** beyond load-bearing traps (memory `feedback_no_code_comments.md`).
- **Don't `git push`** without explicit per-push approval (memory `feedback_no_push_without_permission.md`).
- **Don't add `Required: true`** to a new attribute on an existing resource without confirming with the user — it's a breaking change.
- **Don't create new test files or functions** without explicit user authorization in the current session — extend existing tests instead (memory `feedback_extend_existing_tests.md`).
- **Don't ship a new leaf without Tier 2 coverage** — every leaf in the resource's golden must be exercised in the integration test. Skipping requires explicit per-leaf authorization after the user is informed of the inconsistency-bug risk (memory `feedback_test_all_leaves.md`).

## Commit shape

Per memory `feedback_single_pr_with_folded_fixes.md`: fixes for code introduced on the same branch fold into the introducing commit. Per `feedback_review_shape_commits.md`: generated files get their own commit, placed last.

Suggested commit order for a field-add extend:

1. Schema YAML edit
2. Hand-written conv.go function (if any) + CRUD touch (if any)
3. Tests (Tier 2 update + optional Tier 1 matrix)
4. **Generated:** `*_gen.go` + `*.golden` + `*.description` + `docs/`
