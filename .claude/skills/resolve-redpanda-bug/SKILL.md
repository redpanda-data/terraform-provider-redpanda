---
name: resolve-redpanda-bug
description: Use when diagnosing a bug in terraform-provider-redpanda — whether a user reports a runtime failure ("expected empty plan but resource has planned action(s)", "WARN ... removing from state", "Provider produced inconsistent result after apply") OR a proactive coverage audit finds a function whose purpose is uncovered at one or more test tiers. Walks the mock/fake parity investigation, layered-defense design, red-test-first sequencing, and tier-by-tier regression-guard placement. Covers the specific "mock-loop passes but live run fails" bug class that has been the dominant failure mode in this repo. For new feature work use add-redpanda-resource or extend-redpanda-resource instead.
---

# Resolving a bug in terraform-provider-redpanda

The dominant failure mode in this repo is **fake-vs-real-system parity drift**: the bufconn fake stores or transforms a value differently from how the real Redpanda Cloud backend does, CI stays green, and the bug surfaces only when a user hits the path. Both early bug-hunt reports (`role_assignment` bare-principal, `topic mergeWithPlannedConfig`) trace to this same root cause. This skill is calibrated for that class.

The same playbook applies to test-infrastructure bugs (sweepers, fakes, helper utilities) — anywhere a real-vs-mock contract drift can hide. The `resource_group` sweeper UUID bug is the canonical test-infra example: cleanup orphans surviving `task cleanup:redpanda:dry` is a Track-A trigger just like a `Provider produced inconsistent result` from a live cluster run.

Two entry tracks below — pick the one that matches what triggered the work, then converge on the shared diagnostic playbook from Phase 2 onward.

## Two entry tracks

### Track A: starting from a user bug report

You have runtime evidence: logs from `task test:cluster:aws`, a customer ticket, a failing live test. Triggers include:

- `expected empty plan, but redpanda_X has planned action(s): [create]` (state-vs-server drift)
- `[WARN] redpanda: X not found, removing from state` followed by phantom recreate
- `Error: Provider produced inconsistent result after apply`
- `terraform import` produces state that won't `terraform destroy` cleanly
- Tests pass in `task test:unit` but fail in `task test:cluster:aws`
- `task cleanup:redpanda:dry` shows orphan resources after a passing test run (sweeper failure)

### Track B: starting from a proactive coverage audit

You're auditing a function — typically one whose comment / commit-message says it exists to bridge "real broker vs mock" behavior, or one with a gotests-emitted stub next to a hand-written test, or one the user has explicitly flagged. The bug may or may not exist yet; the failure mode is "tier-by-tier silent, latent oracle gap."

Both tracks converge here.

## Phase 0: Plan first

Per memory `feedback_sonnet_agent_exploration_pattern.md` — when the bug touches code you don't know cold, spawn sonnet exploration agents in parallel before diving in. Useful exploration questions:

- "Walk `<resource>/<file>.go` end-to-end. What does Read do on not-found? On equality checks? On state set?"
- "What does the fake at `internal/testutil/mock/fakes/<service>.go` do on the relevant RPCs? Find the half-implemented paths."
- "Has there been a recent contract-tightening commit? `git log --all -p -S '<keyword>' -- <file>`"
- "What does the cloudv2 or console backend actually do to the value in question? Search proto comments, backend handler files."

Have findings written to `manual-tests/<bug-slug>/`. Compose a one-page consolidated report. Get user sign-off on the plan before writing tests or code.

## Phase 1: Read the function under test end-to-end

Don't grep, don't skim — read the whole CRUD method or merge function. For each step in the function, note:

- Verbatim string comparisons on values sourced from state vs server (`a == b` on strings)
- Branches that exist specifically to bridge real-vs-fake differences (synthesis, strip, reinstate, prev-fallback)
- Implicit assumptions about value shape (prefixes, case, normalization) that aren't enforced by the schema

A 200-line CRUD method takes 5 minutes to read. Time spent here pays for itself ten times over in the next phase.

## Phase 2: Track A — confirm the theory; Track B — define the contract

**Track A — confirm.** Four checks in parallel:

1. **Runtime contract**: read the resource handler end-to-end (Phase 1). Confirm the user's theory matches what the code actually does.
2. **Git history of the file**: `git log --all -p -S "<keyword>" -- <path>`. Look for recent contract tightenings (e.g. "normalizePrincipal removed entirely"). The bug is often a fixture or test that lags behind a deliberate provider change. The schema description usually matches the new contract; the test fixtures usually don't.
3. **Real backend behavior**: search proto comments and cloudv2/console backend handlers for the relevant normalization or shaping behavior. Per memory `project_proto_sources.md`: controlplane = `~/GolandProjects/cloudv2`, dataplane = `~/GolandProjects/console`. Confirm what the real API does to the value (`strings.HasPrefix`, `strings.ToLower`, canonical-form-builder, etc.).
4. **Sibling check**: if the function is one of N similarly-shaped helpers (sweepers, fakes, validators, resource CRUD methods sharing a pattern), grep the other N-1 for the same pattern. If they got it right, your fix is one line and the bug is scoped. If they all got it wrong, the fix is repo-wide and the report has a different shape. The resource_group sweeper UUID bug was found this way — the four sibling sweepers (`network.go`, `cluster.go`, `serverless_private_link.go`, `shadow_link.go`) all used `GetId()` correctly; only `resource_group.go` had `rg.Name`.

If steps 1–4 all confirm the theory, proceed to Phase 3. If any contradict, **stop and ask the user** (memory `feedback_ask_when_unsure.md`) — the theory needs revision before you write tests.

**Track B — define.** You don't have a runtime failure to verify; you have a function whose behavior needs proving. For each branch identified in Phase 1, articulate:

- What real-system behavior is this branch bridging?
- What would the fake need to emit for this branch to be reachable from a Tier 2 integration test?

This list IS the contract. Phase 3 audits whether anything currently emits those signals.

## Phase 3: Audit the fake against every RPC the production calls

This is where the bug class lives. The pattern:

```
SecurityFake.UpdateRoleMembership: stores verbatim
real backend: canonicalizes to User:<name>
→ every test using bare "alice" passes against the fake
→ same test fails against the real backend
→ the fake is a self-confirming oracle for a contract it can't enforce
```

Or:

```
TopicFake.CreateTopic: ignores req.Topic.Configs entirely
real broker: stores them
→ every test that creates a topic WITH config relies on Update path
→ the Create-with-configs path is never exercised
→ the merge function's branch for "config set at create-time" is structurally untested
```

Walk the fake against every RPC the production calls — not just the ones in your bug's flow. **Every untouched RPC is a latent oracle gap.** Note them; you'll fix them with the bug.

**Carve-out: proto-level constraints are already enforced.** The mock server registers `validatingInterceptor` (`internal/testutil/mock/server.go`) which runs `protovalidate` against every incoming RPC. Any `buf.validate.field` rule in the proto descriptor — `[string.uuid]`, regex, length, range, required — is enforced at the bufconn boundary before the fake ever sees the request. For those constraints, fake parity is automatic; **don't hand-patch the fake to mirror what the interceptor already does.** Fake-author parity is only required for behavior NOT expressible in proto annotations: canonicalization (role_assignment principal-prefix), defaulting (server-populated fields), shaping (id-vs-name swaps in `DeleteX` requests), stateful semantics (write-only fields, async polling).

## Phase 4: Audit test coverage tier by tier

Per [`../_shared/testing-tiers.md`](../_shared/testing-tiers.md), the four tiers are:

1. **Tier 1 — model unit** (`redpanda/models/<name>/*_test.go`): does any test exercise the function's branches directly with proto stubs?
2. **Tier 2 — colocated integration** (`integration_<name>_test.go`, bufconn fake): is the bug's path reachable here? Often the answer is no because the fake doesn't simulate the real-system behavior the branch exists to bridge.
3. **Tier 3 — colocated live-acc** (`acc_<name>_test.go`): rarely the answer for dataplane resources — see Phase 6.
4. **Tier 4 — cluster runner** (`redpanda/tests/runner_test.go`): the home for cluster-dependent dataplane coverage.

For each tier, ask: "if this bug shipped, would this tier catch it?" If the answer is no at any tier, that tier needs a regression guard.

## Phase 5: Write red tests first

Per memory `feedback_test_first_bug_proof.md`: write the failing test before the fix. Show the user the red. Then discuss fix scope separately.

Per memory `feedback_extend_existing_tests.md`: extend existing tests, do not create new ones. New test functions or files require explicit per-case user authorization. Open the closest existing test and add a subtest or step.

Red tests typically come in pairs:

- **Validator / plan-time test** — proves the contract is enforced at the input boundary. Usually `ExpectError` matching the error message.
- **Fake parity test** — proves the fake mirrors the real backend's data shaping. Pure-Go table-driven test against the fake's mutation paths.

Run both. Confirm both fail in the expected way. Show the user the output. Get sign-off on the fix scope before proceeding.

## Phase 6: Lay out the layered defenses

The two case studies converged on 4–5 layers of defense per bug. Use this as the template:

1. **Validator at the input boundary (plan-time)** — extract into `redpanda/validators/<name>.go` with a named function (per `redpanda/validators/cidr.go`, `password.go`). Inline `stringvalidator.RegexMatches` in schema is the wrong default — even for hand-written schemas. Add a dedicated `<name>_test.go` with table-driven cases including edge cases (empty, almost-valid, prefix-with-no-content).
2. **Self-heal at the API boundary** — Read canonicalizes the value before comparison and writes the canonical form back to state. Delete sends the canonical form on the outgoing RPC. This protects legacy state that already has the wrong shape.
3. **Fake parity** — fix the fake to mirror the real backend's data shaping. Add a `<fake>_test.go` pinning the rule so future fake authors can't silently regress.
4. **Import format extension** (if relevant) — if `terraform import` produces state that can't refresh-clean (a null RequiresReplace field, a wrong-form value), extend the ImportState parser to accept the additional context. Follow `shadowlink/resource_shadowlink.go:330`'s `<id>|<aux>` pipe-suffix pattern; `|` avoids ambiguity with `:` inside prefixed principal values. Add unit tests for the new parse including empty cases and embedded-colon URLs.
5. **Tier-by-tier regression guards** — add tests at every tier that was structurally silent on the bug. The aim is "if any future commit weakens the validator, breaks the canonicalization helper, breaks the import-ID parse, or makes the fake stop mirroring the real backend, one of these layers fires."

If the bug is purely a coverage gap (Track B, no actual production bug found), the only layers needed are 3 + 5: fake parity + tier-by-tier guards.

Similarly, a **one-line typo in a leaf helper** (sweeper, validator, fake constructor) with no contract change typically needs only two layers: the fix itself plus one in-process regression test that exercises the helper end-to-end through the relevant interceptor chain. The `resource_group` sweeper UUID bug is the canonical example: `rg.Name` → `rg.GetId()` + a `TestSweepResourceGroup_SendsUUIDNotName` that drives the sweeper through bufconn + `validatingInterceptor`. Five layers would be over-engineering; two is calibrated.

## Phase 7: Always ask "what about state already in this shape?"

This is the question that catches the most second-order bugs. A validator gates new shapes; it does **nothing** for state that already drifted. Before declaring the bug fixed, walk through:

- **Refresh** against legacy state: does Read silently remove the resource (the original bug) or self-heal it?
- **Update** against legacy state: does the planned mutation account for the legacy shape?
- **Destroy** against legacy state: does the outgoing RPC use the canonical form (the backend may or may not accept the legacy form; safe assumption is to canonicalize on the way out)?
- **Import** against legacy state: does the ImportState path produce state that can refresh-clean, or does it leave a null field that triggers RequiresReplace on the next plan?

Per the role-assignment report: validator-as-input-gate doesn't protect existing state. If the bug class is "state-vs-server shape drift," you need two fixes — a gate to prevent it going forward, and a self-healer for state that already drifted.

## Phase 8: Don't work around test mechanics that expose production bugs

The role-assignment Phase 8/9 trap, distilled: I had a test that needed to seed legacy-shape state, exercise ImportState, then assert NoOp on a follow-up plan step. The follow-up step kept failing with `[delete create]`. My first move was to pivot to `ImportStateCheck` to bypass the follow-up step. The user pulled me back: "the follow-up step failing is a real production UX bug — anyone importing a `redpanda_role_assignment` in real life hits the same `cluster_api_url` null/RequiresReplace trap and can't even cleanly `terraform destroy` afterward."

The lesson: when you're routing around a test mechanic to make an assertion pass, **the mechanic is usually exposing a real production bug**. Fix the production code so the test can exercise the realistic flow. In that case: extend the ImportState ID format to accept a `|<cluster_api_url>` suffix.

If you find yourself reaching for `ImportStateCheck` instead of `ImportState + follow-up Config step`, stop and ask: "is the follow-up step failing because of a real UX issue?" Almost always, yes.

## Phase 9: Cascading fixes

After the validator lands, every existing test feeding the bare/wrong shape will fail with the validation error — those tests were oracles for a contract that no longer exists. Walk them and update:

- Integration test fixtures (`integration_<name>_test.go`): bulk-replace bare → canonical in `mockXConfig` helpers and `expectedID` assertions.
- Live acc fixtures (`examples/<name>/*.tf`, `examples/cluster/*/main.tf`, `examples/docs/<name>/main.tf`, `redpanda/tests/testdata/...`): grep-and-replace. Confirm with the user before bulk-substituting.
- Stale doc templates (`templates/resources/<name>.md.tmpl`): rewrite any sentences that describe the old contract. Run `task docs` to regenerate `docs/resources/<name>.md`.

Cascading fixes go in their own commit, separate from the production fix. Reviewers should be able to see "this is the fix" and "this is the fixture update" as distinct units.

## Hard rule: extend existing tests, especially at the acceptance tier

Per memory `feedback_extend_existing_tests.md` (general) and `feedback_dataplane_extends_existing_runner.md` (acceptance-tier specific, the sharper rule):

- At Tier 1 and Tier 2: extend existing test functions, don't create new ones. Adding a `stateChecks` entry, a subtest under `t.Run`, or an extra step is the default. New test functions require explicit per-case user authorization.
- **At Tier 4 (cluster runner) for dataplane resources, this rule is absolute.** A `task test:cluster:aws` run takes 45 minutes to 2 hours because the control-plane infrastructure (resource_group + network + cluster) is expensive to provision. Dataplane operations (topic, user, acl, role, role_assignment, schema, schemaregistryacl, serviceaccount) take seconds once the cluster is up. **Never create a new acc test file for a dataplane resource that spins up its own cluster — always extend the existing `testRunner` in `redpanda/tests/runner_test.go`** with steps inside the existing `if hasFoo { ... }` blocks (or add a new `hasNewResource` flag).
- Use unique resource names with feature-suffixes (`redpanda_topic.merge_strip_test`, not `redpanda_topic.test`) so steps added to a shared cluster runner don't collide.
- For shape changes between steps, pair flip-to-new + flip-back-to-original. The pair gives new-shape coverage, an explicit empty-plan guard, AND keeps downstream sentinel/import/destroy steps coherent.

Carve-out: colocated `acc_<name>_test.go` is fine for resources that don't need a cluster (control-plane only — `serviceaccount`, `resourcegroup`) or that legitimately need their own infra (shadow_link provisions two clusters by design). For dataplane resources, it's runner-extension or nothing.

## Hard rule: leaf coverage and golden discipline still apply

Per memory `feedback_test_all_leaves.md`: when the bug fix adds or modifies an attribute (validator, default, plan modifier), confirm every affected leaf is exercised in Tier 2. Per memory `feedback_show_golden_diffs.md`: if a golden test fails as a side effect, paste the raw diff to the user before any "fix." Per memory `feedback_golden_files.md`: never run `task generate:golden` without explicit user approval.

## Don'ts (project-specific)

- **Don't hand-patch generated files** (memory `feedback_no_manual_codegen_fixes.md`) — if Flatten/Expand misses a case, fix the YAML directive or the generator.
- **Don't add `//nolint` or `#nosec`** without explicit user approval (memory `feedback_no_nolint_without_permission.md`).
- **Don't add code comments** beyond load-bearing traps (memory `feedback_no_code_comments.md`).
- **Don't `git push`** without explicit per-push approval (memory `feedback_no_push_without_permission.md`).
- **Don't add inline `stringvalidator.RegexMatches` in schemas** when a named validator in `redpanda/validators/<name>.go` would do the same job — the named-validator pattern is house style.
- **Don't declare a state-shape-drift bug fixed** without checking what happens to state that's already wrong (Phase 7).
- **Don't pivot to a test-mechanic workaround** when the workaround is papering over a real UX bug (Phase 8).
- **Don't spin up a new cluster** for dataplane resource acc coverage — extend `testRunner` (memory `feedback_dataplane_extends_existing_runner.md`).

## Commit shape

Per memory `feedback_review_shape_commits.md` and `feedback_single_pr_with_folded_fixes.md`. For a bug fix that lands multiple layers, suggested commit order:

1. Validator + named-validator file + unit test
2. Production fix: Read self-heal + Delete canonicalize + ImportState extension (if any)
3. Fake parity fix + fake-parity test
4. Tier 2 integration tests (regression guards)
5. Tier 4 runner extension (if dataplane resource and cluster-dependent behavior)
6. Cascading fixture + template + docs updates
7. **Generated:** any regenerated `*_gen.go`, `*.golden`, `*.description`, `docs/`

Reviewer reads the production fix in commit 2, sees the regression guards in commits 3–5, and skips the bulk fixture updates in 6 and the generated diff in 7. Three distinct review concerns, separated cleanly.

## Phase 10: Commit the bug fix (and only the bug fix)

Once the fix is verified — red tests now green, regression guards in place, `task ready` clean — commit the bug fix to the branch. **Commit only the bug-fix artifacts.** Do NOT include any skill or memory edits in this commit; those land separately after the report (Phase 11) and require explicit per-file user approval.

Per memory `feedback_no_push_without_permission.md`: commit locally; do not push without explicit per-push approval. Commit ordering follows the [Commit shape](#commit-shape) guidance below — typically 5–7 commits covering validator → production fix → fake parity → tests → cascading updates → generated.

### The load-bearing commit message: production-fix commit

Most commits in the sequence can use terse one-line messages (`test(role-assignment): pin principal canonicalization`, `chore(docs): regenerate role_assignment.md`). The **production-fix commit** is the load-bearing one — reviewers and future bug hunters read this first via `git log -p` to understand what changed and why. Use this four-section structure:

```
<area>: <one-line imperative summary, no terminal period>

How found: <one-paragraph summary of the original evidence — log fragment,
test failure, customer ticket, or "audit of <function> at HEAD" — including
the actual error message or symptom that triggered the hunt>

Cause: <one-paragraph summary of root cause. Not the symptom, not the fix —
the underlying contract violation, shape drift, or state-vs-server divergence>

Resolution: <one-paragraph summary of the actual code change>

Regression guards: <list of tests added or extended that would catch a
repeat, with file:line references>
```

Example (synthesized from the resource_group sweeper bug):

```
test/sweep: send UUID not name on DeleteResourceGroup

How found: task test:cluster:aws left tfrp-acc-testaws-xF0W and ~16 sibling
RGs orphaned in pre-prod. cluster_aws.log:
    acc cleanup: unable to sweep resource group: ...
    code = InvalidArgument desc = validation error: id: value must be a valid UUID

Cause: internal/testutil/acc/sweep/resource_group.go passed proto.Name where
DeleteResourceGroupRequest.Id expects a UUID. The [string.uuid] buf.validate
rule on the proto descriptor rejects anything that isn't a UUID at the
validatingInterceptor boundary, before the handler runs. Four sibling
sweepers (network, cluster, serverless_private_link, shadow_link) all
correctly used GetId(); only resource_group had the typo. Bug arrived as-is
in 00b43a8; never previously invoked correctly.

Resolution: rg.Name → rg.GetId() in resource_group.go:40.

Regression guards:
  - internal/testutil/acc/sweep/resource_group_test.go (new):
    TestSweepResourceGroup_SendsUUIDNotName drives the sweeper end-to-end
    through bufconn + validatingInterceptor. If the typo returns (or any
    other malformed DeleteResourceGroupRequest.Id ships), this test fails
    with the same "must be a valid UUID" error the live run produced — in
    milliseconds, no cloud cost.
```

The four-section structure makes the commit function as a mini-postmortem readable in `git log -p`. The full long-form report (Phase 11) lives in `manual-tests/bug hunt reports/<slug>.md` and goes into much more detail (dead ends, mid-flight corrections, sibling-audit results, fake-vs-real reasoning); the commit message captures the load-bearing essence so a future Claude or human reading `git log` understands the change without leaving the terminal.

### What NOT to include in the bug-fix commits

- **Skill edits** (`.claude/skills/*`) — land in separate commits after Phase 11's report and per-file human approval
- **Memory edits** (`~/.claude/projects/.../memory/*`) — separate, also after approval
- **Unrelated cleanups** discovered during the hunt — flag via `spawn_task` or queue for a follow-up PR; don't bundle into the bug fix
- **Speculative refactors** the bug "made you think of" — out of scope; the bug fix should be the smallest change that resolves the symptom + lays the regression guards

Memory `feedback_single_pr_with_folded_fixes.md`: fixes for code introduced on the *same branch* fold into the introducing commit. Bug fixes against code on main are their own commit sequence — don't fold them into unrelated branch work.

## Phase 11: Write the bug-hunt report and use it to update this skill

After the bug is verified fixed, write a narrative report at `manual-tests/bug hunt reports/<slug>.md`. Existing reports in that directory are the template — the `Role assignment.md` and `Topic mergeWithPlannedConfig.md` files this skill was distilled from are the canonical examples.

Report shape (narrative, not bullet-point summary — the value is in capturing *how*, not just *what*):

- **What the user saw** — the original evidence: log fragments, error messages, runtime symptoms
- **What the function was supposed to do** — read end-to-end of the function/path under test, explain its purpose
- **Phase-by-phase investigation** — what you checked, in what order, including dead ends and mid-flight corrections (the role-assignment report's Phase 5 "first mistake: inline regex" is exactly the kind of dead end worth capturing)
- **The mock/fake parity gap** — if applicable, the specific RPC path that lied
- **Red tests first** — what you wrote, what failure looked like
- **Layered defenses landed** — the 4–5 layer template, mapped to files and test functions
- **Files touched** — categorized (production / test infra / fixtures / docs / memory)
- **Distilled lessons / "pattern for a future skill file"** — the trigger phrases and steps that, with the next bug of the same shape, would let a future Claude move faster

Future Claude instances learn diagnostic moves by reading prior reports, not by reading the skill in isolation. The reports are the long-form ground truth; the skill is the compressed playbook.

### Feed the report back into this skill

Once the report is written, **read it against this SKILL.md** and look for:

- **New trigger phrases or symptoms** for the "Two entry tracks" section
- **Diagnostic moves that worked but aren't in the phases above** — fold them into the appropriate phase
- **Dead ends that wasted time** — add to the Don'ts so the next Claude doesn't repeat them
- **New test-mechanic traps** (like the ImportState/RequiresReplace one in role-assignment Phase 8) — extend Phase 8 with the new pattern
- **New bug classes** that don't fit the existing "mock parity" framing — note whether they belong as a sub-track in this skill or warrant a separate skill

### Feed the report back into the broader skills surface

Bug-hunt reports are the highest-quality calibration source the repo has. Lessons rarely belong in this skill alone — they almost always touch one or more of the other skills. **Read the report against every other skill in `.claude/skills/`** and identify edits worth proposing:

- **Mock-infrastructure facts** (like the `validatingInterceptor` carve-out, new fake hooks, new mock-server registration steps) → [`../_shared/testing-tiers.md`](../_shared/testing-tiers.md)
- **Schema-authoring traps** (a YAML directive that caused unexpected flatten/expand output, a validator pattern, a deprecation-alias subtlety) → [`../_shared/schema-authoring.md`](../_shared/schema-authoring.md)
- **CRUD patterns surfaced during the fix** (new flatten_via/expand_via convention, a new ImportState ID-format pattern, a self-heal idiom, error-message convention) → [`../_shared/crud-glue.md`](../_shared/crud-glue.md)
- **New test-infra helpers or seed hooks** (`SetServerInjectedConfig`, `SeedRoleWithMembers`, sweep helpers) → [`../_shared/testing-tiers.md`](../_shared/testing-tiers.md)
- **Codegen workflow nuance** (a `task generate:apidescriptions` pre-step, a golden-update incantation, a stderr signal worth grepping) → [`../_shared/codegen-workflow.md`](../_shared/codegen-workflow.md)
- **Docs / example gotchas** (datasource attribute-name divergence, template phrasing that ages badly with contract changes) → [`../_shared/docs-and-examples.md`](../_shared/docs-and-examples.md)
- **Manual-validation patterns** (a new upgrade-scenario step, a new dev-overrides trap) → [`../_shared/manual-validation.md`](../_shared/manual-validation.md)
- **Workflow changes for the resource-authoring flows** (a phase that should be added or reordered in the add/extend skills) → [`../add-redpanda-resource/SKILL.md`](../add-redpanda-resource/SKILL.md), [`../extend-redpanda-resource/SKILL.md`](../extend-redpanda-resource/SKILL.md)
- **Durable preferences or anti-patterns** the user surfaced during the hunt → propose a new memory file under `~/.claude/projects/.../memory/` and an entry in `MEMORY.md`

### Surface the proposed edits to the human operator

**Present all proposed edits — this skill plus any other skills plus any memory updates — to the user as a single batch with a brief rationale per file.** The format:

> Based on the `<bug-slug>` report, I propose these skill/memory updates:
>
> - **`resolve-redpanda-bug/SKILL.md`** — adding `<symptom>` to Track A; new Phase-3 carve-out about `<topic>`. Rationale: <one sentence>.
> - **`_shared/testing-tiers.md`** — documenting `<new helper>` introduced in `<file>`. Rationale: <one sentence>.
> - **`extend-redpanda-resource/SKILL.md`** — no change; the bug was in `<area>` not covered by this skill.
> - **New memory `feedback_<slug>.md`** — captures the rule `<rule>` the user articulated during the hunt.
>
> Approve, redirect, or request changes before I apply.

**Wait for explicit approval before applying any edit.** These files are committed code that shapes future Claude behavior — a thoughtless edit propagates across every future session in this repo. The human operator in the loop is non-negotiable: even if every individual change looks small, the user makes the call on which lessons are durable and which were one-off context. Approval is per-file, not per-batch — the user may approve the bug-skill edit and defer the memory addition, or vice versa.

If a proposed edit conflicts with an existing rule or contradicts something the user has said before, **flag the conflict explicitly** in the proposal. Don't quietly overwrite — the contradiction is often where the most useful conversation happens.

This feedback loop is what makes the skills self-improving over time. Without it, they stay frozen at composition time and degrade as the codebase, fake infrastructure, and bug shapes evolve. With it, each bug hunt makes the next one cheaper across the entire skills surface — not just the bug skill.

## When you're done

Seven questions to answer before declaring the bug fully closed:

1. **Red tests passed?** The tests that proved the bug before the fix should now be green. If you stash-pop the fix, do they go red again?
2. **Legacy state covered?** Phase 7 question — what happens to state that already has the wrong shape?
3. **Fake parity restored?** Every RPC the production calls should be modeled faithfully (Phase 3).
4. **Tier coverage at every level?** If a future commit weakens the validator, breaks the canonicalize helper, breaks the import parse, or makes the fake regress to verbatim storage, **which test fires first?** If the answer is "none," there's a tier-by-tier guard missing.
5. **Cascading updates clean?** Fixtures, templates, docs all match the new contract. `task ready` clean.
6. **Bug fix committed (and only the bug fix)?** Phase 10 — production fix commit has the four-section structured message (how found / cause / resolution / regression guards). Skill and memory edits are NOT in any of the fix commits.
7. **Bug-hunt report written and skill updates proposed?** Phase 11 — the report lives in `manual-tests/bug hunt reports/<slug>.md`, and any updates to this skill, the other skills under `.claude/skills/`, or memory have been proposed to the user as a single batch with rationales. Approval is per-file; the user makes the call.

If all seven answer yes, summarize the layers landed and the files touched (the role-assignment report's "What's now in the test infrastructure" section is a good template) in the PR description.
