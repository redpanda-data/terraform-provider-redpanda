# CLAUDE.md

This file guides Claude Code in the terraform-provider-redpanda repository.

## Skills

Invoke the matching skill **before** starting work in its area. Do not attempt the work without loading it first.

| Skill | Invoke when |
|---|---|
| `add-redpanda-resource` | Creating a new `redpanda/resources/<name>/` package (resource or datasource) |
| `extend-redpanda-resource` | Adding or changing fields on an existing resource or datasource |
| `resolve-redpanda-bug` | Diagnosing any runtime failure ("expected empty plan", "inconsistent result after apply", "removing from state") or a test-coverage gap |
| `manual-test-redpanda-resource` | Validating a change against a live cluster; the live step of the two skills above |
| `writing-code-comments` | Writing or editing any comment, and reviewing a diff that adds or changes one |
| `review` (user-invoked `/review [N]`) | Reviewing a PR or the branch; runs the read-only `reviewer` agent against `.claude/review-policy.md` |

User-level skills some maintainers carry (`monitor-logs`, `branch-loc-count`) are not committed; if present they trigger on `/monitor-logs` and `/branch-loc-count`.

`.claude/skills/README.md` lists the `_shared/` reference docs and `live-acctest-orchestration.md`. `.claude/review-policy.md` defines the signal bar for code review.

## Essential Commands

Task runner: [Task](https://taskfile.dev). Use `task` directly (no wrapper).

### Safe to run without asking
- `task ready` — hooks + docs + lint + `go mod tidy`; run before every commit. `task hooks:install` points git at `scripts/hooks/`, whose `commit-msg` enforces the commit policy below.
- `task pr:status -- [N]` — bounded triage summary on stdout: failing CI jobs with their extracted failure lines, then unresolved review threads. Full job logs land in `.logs/pr-<N>/<job>.log.cld`; grep those, never cat them. Buildkite access needs `BUILDKITE_API_TOKEN`.
- `task lint` / `task lint:fix` — golangci-lint, preceded by `scripts/lint-comments.sh`
- `task lint:comments` — the comment check alone; `task lint:comments -- --report` lists comment blocks of 12+ lines
- `task lint:headers` — insert or normalize license headers. The year is the file's creation year and is never bumped; `goheader` in `task lint` accepts any year but rejects a missing or malformed header. Generators keep an existing file's year on regeneration.
- `task build:tidy` — `go mod tidy` only
- `task test:unit` — race-detector unit tests, no cloud creds. `integration_*_test.go` files are gated by the `integration` build tag and excluded here.
- `task test:integration` — race-detector colocated integration tier; bufconn-backed CRUD / import / drift flows in `redpanda/resources/*/integration_*_test.go`. Built with `-tags=integration` and filtered to `TestIntegration_*`. No cloud creds.
- `task build`, `task build:install`, `task docs`, `task mock`, `task generate:*` (see Codegen below)

### Live acceptance tests: get explicit approval before every run
All of these need `REDPANDA_CLIENT_ID` + `REDPANDA_CLIENT_SECRET` plus cloud-provider creds, create real resources in the preprod org, and bill it. A hung run leaves resources behind that the framework cannot destroy (see Troubleshooting).

- `task test:network` — network resource only, no cluster. Minutes. The cheapest live check and the cheapest upgrade-entry check.
- `task test:serverless:aws:public` / `:aws:private` / `:aws:both` / `:gcp` / `:regions` / `:privatelink` — serverless cluster; minutes.
- `task test:service_account` / `task test:shadowlink` — focused tests, need an existing cluster.
- `task test:datasource:cluster` — creates a dedicated cluster and reads it. Well over an hour.
- `task test:cluster:aws` / `:gcp` — dedicated cluster lifecycle. Well over an hour; create timeouts are 150m for a reason.
- `task test:byoc:aws` / `:gcp` — BYOC cluster. Longest lane; also needs the BYOC service-account creds.
- `task test:byovpc:aws` / `:gcp` — provisions VPC infra with Terraform first, then the cluster, then tears down. Longest and most likely to leak infra.

Every acceptance test starts with a **provider-upgrade entry**: step 0 applies its config with the released provider from the registry (pin via `REDPANDA_LAST_VERSION`, default latest), step 1 re-plans with the local build and requires an empty plan, then the normal steps run on the local build. `REDPANDA_UPGRADE_ENTRY=off` disables the entry (needed for local runs with dev_overrides, configs the released provider can't parse yet, and release-validation runs that want the local build's create path exercised live). There is no standalone upgrade tier.

Prefer reusing an existing test cluster over creating one. Create fresh only when cluster lifecycle is the thing under test.

### Cleanup: destructive, get explicit approval
- `task cleanup:aws:ci` — nuke BYOVPC AWS resources (auto-approve, CI-safe)
- `task cleanup:aws` — same but interactive
- `task cleanup:aws:nuke` — nuke all non-default VPCs in test regions (manual, last resort)
- `task cleanup:gcp` / `:gcp:ci` / `:gcp:dry` — GCP BYOVPC sweeper (interactive / auto-approve / preview)
- `task cleanup:redpanda` — delete stale `tfrp-*` clusters from Redpanda Cloud; also deletes a cluster another maintainer is keeping alive if it matches the prefix
- `task cleanup:redpanda:dry` — preview only; run this first

### Codegen / docs
- `task generate:models` — regenerate enum mappers (`redpanda/utils/enums/enums_gen.go` via `cmd/enumgen`), then `redpanda/models/*/*_gen.go` and `redpanda/resources/**/*_gen.go` (schema + `proto_validator_gen.go`) from `schema.yaml`. enumgen runs first because the generated flatten/expand references the `enums.*` mappers; it is pinned to `internal/buf_dependencies.yaml` like schemagen.
- `task generate:enums` — regenerate only the enum mappers
- `task generate:clean` — delete all generated `*_gen.go` files under `redpanda/resources/`, `redpanda/models/`, and `redpanda/utils/enums/`
- `task docs` — regenerate `docs/` via tfplugindocs
- `task mock` — regenerate `redpanda/mocks/*`

### Build / install / release
- `task build` — build `terraform-provider-redpanda` binary
- `task build:install` — install to local Terraform plugin cache
- `task local:cluster:aws:apply` / `:destroy` — apply an example stack locally (creates a real cluster; approval-gated like the acc tests)

## Working directories convention

- `.claude/` — **committed**. Holds project skills (`.claude/skills/`), the review policy, and any shared Claude Code project settings. `.claude/settings.local.json` is per-user and gitignored explicitly.
- `manual-tests/` — **gitignored**. Local working directory for everything ephemeral: manual test workspaces (`manual-tests/<resource>/`), audit reports, bug-hunt notes, sonnet-agent exploration findings, scratch consolidations. Skills point here for any artifact that isn't a contract.
- `.logs/` — gitignored. Reserved for log capture (monitor-logs skill, test-output dumps). Don't use for scratch documents.

When a skill spawns sonnet exploration agents or generates audit reports, they go to `manual-tests/<topic>/`, not `.claude/` and not `.logs/`.

## Repo map (the parts that aren't guessable)

`ls` covers the obvious trees. These are the ones that mislead:

- **`redpanda/resources/schemagen.go` is the only `//go:generate` registry.** Per-resource packages carry no directives; adding a resource means adding a line here. `redpanda/resources/codegen.yaml` holds the global config (enum carve-outs, exclude list).
- **`*_gen.go` is generated, everywhere.** `redpanda/models/*/`, `redpanda/resources/**/`, `redpanda/utils/enums/`. Hand edits are overwritten by `task generate:models`; fix the generator (`internal/schemagen/`, `cmd/schemagen/`, `cmd/enumgen/`) instead. `*.golden` files next to them pin the schema contract.
- **Not every resource is schemagen'd.** `roleassignment`, `schema`, `schemaregistryacl`, `secret`, and the list datasources (`regions`, `serverlessregions`, `throughputtiers`) are hand-written end to end. The registry in `schemagen.go` is the authoritative list of what is generated.
- **Two test trees, split by tier, not by resource.** `redpanda/resources/<r>/integration_*_test.go` (package `<r>_test`, build tag `integration`) is the in-process bufconn tier against `internal/testutil/mock/fakes/`. `redpanda/tests/` is live-only. Shared harness code is `internal/testutil/{acc,integration,mock,upgrade}/`.
- **The fakes decide what the integration tier can catch.** A fake that doesn't populate what the control plane populates, or accepts what it rejects, makes a broken provider pass. Read `internal/testutil/mock/fakes/` before trusting a green integration run.
- **`cmd/` is tooling, not the provider.** The provider entrypoint is `main.go` at the root, wiring `redpanda/redpanda.go` (`New`). `cmd/` holds the codegen binaries (`schemagen`, `enumgen`, `apidesc-import`) and `prstatus`. `internal/schemagen/` has its own `CLAUDE.md`.
- **`docs/` is generated** from schema descriptions plus `examples/` via `templates/`. `examples/` is the input; `docs/` is the output.
- **The API lives in sibling checkouts.** `../cloudv2` (control plane protos) and `../console` (dataplane). The schemagen buf pin in `internal/buf_dependencies.yaml` selects which proto revision codegen sees.
- **`redpanda/tests/testdata/network/{aws,gcp}/`** are the BYOVPC infra-producer Terraform stacks the byovpc lanes apply before the cluster.
- **`scripts/cleanup-*`** are the sweeper binaries behind `task cleanup:*`.
- `redpanda/cloud/` (gRPC client, `connpool`, `ratelimiter`), `redpanda/kclients/` (Kafka API / schema registry clients), `redpanda/validators/` (named-validator house style), `redpanda/mocks/` (gomock clients for unit tests) are what their names say.

## Testing

Three tiers. Pick the narrowest that exercises the behavior.

| Tier | Where | When to use | Creds |
|------|-------|-------------|-------|
| Unit | `*_test.go` next to the code (no `//go:build integration`) | Pure logic, mapping, validation; uses `redpanda/mocks/` gomock clients. Run via `task test:unit`. | None |
| Colocated integration | `redpanda/resources/<r>/integration_*_test.go` (external `<r>_test` package, `//go:build integration`); shared helpers in `internal/testutil/{acc,integration,mock,upgrade}/` | Per-resource CRUD / import / drift flows in isolation. Run via `task test:integration`. | `REDPANDA_CLIENT_ID/SECRET` for live, none for mock variants |
| Live acc | `redpanda/tests/*_test.go` via `task test:<scope>` | Cluster lifecycle, BYOC, BYOVPC, cross-resource end-to-end flows | `REDPANDA_CLIENT_ID/SECRET` + cloud-provider creds |

Prefer unit tests with gomock clients for anything that can be exercised in-process. Reach for live acc only when the behavior genuinely requires a real cluster.

### Fixtures must model reachable state

Be thorough, but don't test the impossible. A fixture that constructs a state production can't produce proves nothing, and it passes — which is worse than failing, because it reads as coverage.

Build fixtures the way production builds them. If a type has an invariant — a flag set whenever a collection is populated, a field the server always returns — honor it, or you are asserting against a shape that will never reach the code.

When tightening an invariant turns a test red, ask first whether the fixture was modeling something impossible. Usually it was: **fix the fixture, don't relax the invariant.** That the test failed is the invariant working.

A test modeling an unreachable state is a *fixture* bug, not a redundant test. Correct it in place — deleting or skipping it still needs explicit approval, same as any other test.

The same failure wears other costumes, all seen in this repo:

- a fake that doesn't populate what the control plane always populates, so the provider is never asked to handle it
- a golden that agrees with the generated output because both are wrong
- a diagnostic that never fires, read as "clean" when it was inert

If a test passes, make it fail once on purpose to prove it was testing something.

## Before implementing a feature

Read the API and the fake first. Both have repeatedly been the source of bugs that no test tier caught.

**The API (`../cloudv2`, `../console`).** Read the actual proto, not just the field list:

- Compare the **read, create, and update messages**. A field's writability is whatever the write shapes say — a field absent from both is server-owned, and one present only on create needs `RequiresReplace`. Don't infer it from the field name or from `field_behavior` annotations, which have proven unreliable here.
- Note `oneof` blocks. Arms are mutually exclusive, must not be `Computed` when the user selects them, and each pair needs an arm-switch test.
- Note which messages are **shared** between read and write shapes. Where they are, diffing tells you nothing and the yaml annotation carries the decision.
- Check `buf.validate` rules and whether the control plane rejects a change in some states (not just whether the field is sendable).

**The fake (`internal/testutil/mock/fakes/`).** A lenient fake makes tests pass against a broken provider. Before trusting a green run, confirm the fake:

- populates every field the control plane derives (status blocks, `effective_*` mirrors, generated IDs, fingerprints) — on **both** Create and Update
- masks on Read whatever the real backend masks (passwords, keys)
- rejects what the real backend rejects

## Code Generation

Models and schemas: run `task generate:models` after editing a `schema.yaml` (or `schema_datasource.yaml`) under `redpanda/resources/<resource>/`. Review the `*_gen.go` diff before committing. The `//go:generate` directives are registered centrally in `redpanda/resources/schemagen.go`.

Golden files (`*.golden`) are **sacred** — never modify without explicit user approval. They pin the schema contract that drifts silently without this guardrail.

Never swallow warnings or errors from codegen. Surface them.

## Documentation

`task docs` regenerates `docs/` from schema descriptions + `examples/` via tfplugindocs. Never hand-edit `docs/*.md` — they will be overwritten.

If you change a schema `Description` field, run `task docs` and commit the diff in the same commit.

## Git & PR Workflow

Never use `git checkout`, `git restore`, or `git reset --hard` to discard files without explicit user instruction. The maintainer supervises this checkout and keeps in-progress work in it; a revert that looks unrelated to your task can erase hours of theirs. If you see unexpected changes, mention them and ask.

Never `git push` (including `--force` / `--force-with-lease`) without explicit approval for that specific push. A push is public and irrevocable: forks and notification emails carry it within seconds, and every push triggers the CI matrix. Approval for one push does not carry forward — ask again before every subsequent push. Batch-committing is fine; batch-pushing is not.

Before every commit: `task ready` (or at minimum `task lint`).

### Commit messages

`type(scope): summary`. Title at most 72 characters, imperative, lowercase after the colon, no trailing period. Check `git log --oneline -- <path>` and match the scope the area already uses.

- Types: `feat`, `fix`, `test`, `docs`, `chore`, `ci`. Append `!` after the scope for a breaking schema change.
- Scope is the resource package or subsystem: `cluster`, `topic`, `schemagen`, `acc`, `deps`, `generated`.
- One logical change per commit. Regenerated output (`*_gen.go`, goldens, `docs/`) is its own `chore(generated): ...` commit, last in the series. Fixes to code introduced on the branch fold into the introducing commit; fixes to pre-existing main code stay separate.
- Body only when the title cannot carry the why. Wrap at 72. State the reason and the non-obvious consequence; never restate the diff. A test commit names the behavior pinned, not the bug's history.
- No ticket IDs, PR numbers, session context, or customer identifiers in the title or body. The squash-merge `(#N)` suffix is added by GitHub.
- Trailers are the only structured metadata: `Co-Authored-By:` and `Claude-Session:` as the harness emits them. No ad-hoc trailers; that content goes in the body as prose.

### Pull requests

- Title is the commit title for a single-commit PR, otherwise a `type(scope): summary` for the whole change.
- Body leads with what a user of the provider sees differently, then what changed and why, then which test tiers ran and which were skipped. Point reviewers at the generated commit so they can skip it.
- Nothing from the session that is not in the repo: no customer names, logs, tickets, or internal threads.

### GitHub PR comments
- All comments on PR: `gh api repos/redpanda-data/terraform-provider-redpanda/pulls/<N>/comments --paginate`
- Single comment by ID: `gh api repos/redpanda-data/terraform-provider-redpanda/pulls/comments/<ID>`
- Comment ID comes from the URL fragment: `#discussion_r2382392371` → ID `2382392371`

## Conventions

- Comments: default to none. Before writing one, answer "what does this tell a future reader that the code doesn't?" Keep a non-obvious why, a non-local consequence, or a pointer a reader can't reconstruct. Delete narration that restates the code, change history ("previously", "now", "used to"), ticket and PR numbers, session context, perishable measurements, and commented-out code. Present tense, stating the current rule. `scripts/lint-comments.sh` fails on the unambiguous markers; the `writing-code-comments` skill has the full gate.
- Never add `//nolint`, `#nosec`, or any linter suppression without explicit user approval.
- `allow_deletion=false` in cluster tests is intentional and acts as a canary for testing failures. Fix the upstream failure, don't flip the flag.
- When reporting/fixing bugs: write a failing test first, show the red, *then* discuss fix scope separately.
- Clarifying questions from the user ("wait, is that correct?") are not pushback. Verify and answer — don't withdraw a correct finding.
- Never delete files or large blocks of code without summarizing and asking first.
- Synthetic identifiers only in tests, fixtures, examples, and docs: no real customer org names, cluster IDs, or emails.

## Troubleshooting

- **Acc test hangs or leaves dangling cloud resources**: kill the test process, then `task cleanup:aws:ci` (plus `task cleanup:redpanda` for stale clusters). Don't wait for the test framework to destroy — it often can't.
- **Lint fails after codegen**: `task lint:fix`, then review the diff.
- **Docs CI fails**: you changed a schema description; run `task docs` and commit the regenerated files.
- **Local provider testing**: `task build:install` copies the binary into `.terraform.d/plugins/…`; point your `.terraformrc` at that dev override to consume the local build.

## Local test cycle

When the user asks for a "local test cycle" (or equivalent phrasing — "run the local tests", "do a test cycle", etc.), run the following sequentially in this order, even if an earlier one fails:

1. `task test:unit`
2. Golden tests
3. `task docs`
4. `task lint`
5. `task test:integration`

Don't skip ahead if one fails — keep going and report each result, including pass/fail counts, key failures, and how long each task took.
