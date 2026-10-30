# Review policy

How code review works in this repo: `/review` (the orchestration in `.claude/skills/review/SKILL.md`) runs `reviewer` agents (`.claude/agents/reviewer.md`) as finders and verifiers. The rules a change is judged against live in `CLAUDE.md` and the skills. This file says what a finding is, which angles every review covers, how candidates are verified, and how the result is reported.

## Posture: recall first, then verify

A review has two jobs, done by different agents. Finders surface every candidate they can attach a concrete failure scenario to, including ones they only half believe. Verifiers decide. A finder that quietly drops a suspicion because it is "probably fine" is the main way real bugs reach the humans, so finders never self-filter on certainty.

The review always covers the whole change: every commit on the branch against `main`, plus the working tree. A re-review after fixes is a full pass over the current state, not a pass over the delta. The brief may say what moved since last time, but the scope never shrinks to it.

Lines the diff did not touch are in scope when they sit in a function the diff did touch. The change re-exposes them.

## Finder angles

Each angle is one finder, run independently. No angle's conclusion suppresses another's. A line that two angles reach by different routes keeps both entries until dedup.

Generic:

- **A. Hunk read.** Every hunk, line by line, then the whole function around it. For each line: which input, state, ordering, or platform makes it wrong? A condition that is inverted or tests the wrong thing, a boundary off by one, nil dereference on a path the surrounding code shows is reachable, swallowed errors, wrong variable after copy-paste, a check against zero that should be a check for absence.
- **B. Removed behaviour.** For every deleted or replaced line, name the invariant it enforced and find where the new code re-establishes it. A guard, an error path, a validation, or a test with no successor is a candidate.
- **C. Callers and callees.** For every changed function, find its callers and check each against the new precondition, return shape, error, or ordering. Then check the callees: does another change in the same diff make a call unsafe?
- **D. Language and framework pitfalls.** Go: nil-map write, loop-variable capture, shadowed `err`, pointer to range variable, mutex scope. terraform-plugin-framework: `ValueBool()`/`ValueString()` on an unknown value return the zero value, so a flag fed from another resource's output is silently false at plan time; `Read` has no config, only prior state; `Configure` runs during validate with nil `ProviderData`; a plan modifier runs only on configs the validators accept; `UseStateForUnknown` carries stale values after a replace.
- **E. Wrappers and adapters.** A new or changed type that wraps another (connection pool, client wrapper, retrying transport, logging writer) forwards every method the callers use and delegates to the inner value rather than reaching back through a global or a registry.
- **Reuse.** Code that rebuilds a helper the repo already has. Name the helper.
- **Simplification.** Derivable state, near-duplicate copies, dead code left behind, nesting that a guard clause removes. Name the simpler form.
- **Efficiency.** Repeated I/O, sequential work that is independent, blocking work added to provider startup or to every Configure. Name the cheaper form.
- **Altitude.** A special case layered on shared code where a change to the mechanism would have fixed the class. Name the deeper fix.
- **Conventions.** A `CLAUDE.md` rule the diff breaks, with the rule and the line both quoted. No preferences the files do not state. The personal `~/.claude/CLAUDE.md` is out of scope: a committed review must not depend on one maintainer's private rules.

Repo-specific:

- **F. Schema contract.** For every attribute the diff adds or changes, open the proto in `../cloudv2` or `../console` and compare the read, create, and update messages: writability comes from the write shapes, not from the field name or `field_behavior`. Open the resource's public API read mapper (`../cloudv2/apps/public-api-go/internal/services/<service>/v1/mapper.go`) for every nested message: one the mapper builds unconditionally is always present on read, so its attribute is Required or Optional+Computed, never plain Optional. Mappers differ per resource for the same message; check the one for the resource in the diff. `oneof` arms are not `Computed` when the user picks them. `RequiresReplace` is present exactly on create-only fields. A retired nested attribute keeps a deprecated tombstone that fails at plan (see `internal/schemagen/CLAUDE.md`).
- **G. Fakes and fixtures.** For every change under `internal/testutil/mock/fakes/`, compare what the fake stores and returns against the read mapper field by field: a fake that echoes the create request where the control plane synthesizes a status block, a default, or an unconditional nested message lets a broken provider pass. A fake that rejects what the backend accepts, or accepts what it rejects, is a candidate. For every test change, ask what state the fixture models and whether production can produce it. A diagnostic an earlier gate makes unreachable is a candidate: it reads as coverage. A deleted or skipped test is a candidate unless the change explains it.
- **H. External contracts and environment.** The provider is correct only relative to systems it does not own. For auth and cloud-credential changes: the semantics of the Terraform provider users run alongside (azurerm, google, aws) for the same environment variables, including precedence between the two naming families and what a managed runtime injects (AKS workload-identity webhook, GitHub Actions OIDC endpoints, Cloud Shell). For tags, status fields, and defaults: what the control plane has started writing since the branch was cut (`git log` in `../cloudv2` on the mapper and service). For retries and timeouts: what the backend returns in the failure mode being handled. Fetch the upstream source with `curl -s` against the raw GitHub file and quote it; a claim about another system's behaviour without a quote is PLAUSIBLE at best.
- **I. Stated versus actual behaviour.** Every promise in the diff is a claim to test: template and docs text, schema descriptions, error and diagnostic messages, comments, commit bodies, and the PR description. Trace each promise to the code path that honours it, and name the configuration or environment where it does not hold.

## Mechanical pre-check

Before any angle runs, the orchestrator lists the changed files and `git ls-files` for secret-shaped paths: `.env*`, `*.pem`, `*.key`, `*.p12`, `*.pfx`, `*.jks`, `*.tfvars`, anything under `secrets/` or `credentials/`, and JSON or YAML named `*credentials*` or `*service-account*`. Names ending in `.example`, `.sample`, `.tmpl`, or `.dist` are templates and do not count. Source files whose names merely contain `secret` or `token` (the `redpanda_secret` resource, the token source) are provider code. A hit is the first finding, by path only, with no content read. Finders also report any credential-shaped literal (key, bearer token, client secret, private key, cloud account ID, real org or cluster ID) by path and line, never by value.

## Verification

Candidates pointing at the same line and mechanism are deduplicated; whichever entry has the sharper scenario survives. Each survivor goes to one verifier, which returns exactly one verdict:

- **CONFIRMED.** The verifier names the state under which it fires and what goes wrong, and quotes the line. Reproducing it by running code (a scratch `go test`, `go run`, or an integration-tier run) is the strongest form and is recorded with the command.
- **PLAUSIBLE.** The mechanism is real; the trigger depends on timing, environment, or configuration the verifier could not pin down. The verdict names the observation that would settle it.
- **REFUTED.** Only with proof constructible from the code: the quoted line says otherwise, a type or invariant makes it impossible, a guard elsewhere in this diff handles it, or it has no observable effect.

PLAUSIBLE is the default whenever the state can occur in practice: a race, a nil on a seldom-taken but reachable path, an unknown value at plan time, a retry storm, a lost anchor in a pattern. "Speculative" and "depends on runtime" are not grounds to refute.

Reading the code carefully is not verification of a semantic claim about a framework or an external system. A claim that a framework method behaves some way is CONFIRMED by running it or quoting its source, not by reasoning about it.

## Ranking and filters

Secrets first. Then correctness (A to E, F, G, H, I) over conventions over cleanup (reuse, simplification, efficiency, altitude). Within a group, by blast radius: silent wrong state or data loss, then a failed apply, then a wrong plan or diagnostic, then everything else.

Drop only: duplicates, REFUTED verdicts, style preferences no file states, and behaviour changes the PR description declares as intended. Something the linter or `task docs` would also catch still stands if it is a real defect; note the gate that would have caught it.

## Report

The orchestrator reports through the `ReportFindings` tool, one call, most severe first: `file`, `line`, `summary` (one sentence, the defect), `short_summary` (at most 60 characters, the claim only), `failure_scenario` (inputs and state, then the wrong outcome; for cleanup, the concrete cost), `category` (`correctness`, `schema-contract`, `fakes-fixtures`, `external-contract`, `stated-behaviour`, `conventions`, `reuse`, `simplification`, `efficiency`, `altitude`, `secrets`), and `verdict`. Paths are repo-relative. After the call, the reply restates each finding on one line as `path:line, summary`, and names the level the review ran at. No praise, no summary of what the PR does, no list of things found clean: a clean list established by reading certifies nothing. An empty review is reported as an empty findings call.
