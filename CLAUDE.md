# CLAUDE.md

This file guides Claude Code in the terraform-provider-redpanda repository.

## Essential Commands

Task runner: [Task](https://taskfile.dev). Use `task` directly (no wrapper).

### Pre-commit / quality
- `task ready` — docs + lint + `go mod tidy` (run before every commit)
- `task lint` / `task lint:fix`
- `task build:tidy` — `go mod tidy` only

### Tests
- `task test:unit` — race-detector unit tests, no cloud creds. `integration_*_test.go` files are gated by the `integration` build tag and excluded here.
- `task test:integration` — race-detector colocated integration tier; bufconn-backed CRUD / import / drift flows in `redpanda/resources/*/integration_*_test.go`. Built with `-tags=integration` and filtered to `TestIntegration_*`. No cloud creds.
- `task test:upgrade:smoke` — provider-upgrade regression tests; no cluster (resource_group + network)
- `task test:upgrade` — full provider-upgrade test suite (build tag `upgrade`)
- `task test:cluster:aws` / `:gcp` — live cluster acc tests
- `task test:byoc:aws` / `:gcp` — BYOC acc tests
- `task test:byovpc:aws` — provisions infra, runs test, tears down
- `task test:serverless:aws:public` / `:aws:private` / `:aws:both` / `:gcp` / `:regions` / `:privatelink`
- `task test:datasource:cluster` — datasource acc test (creates cluster + reads)
- `task test:network` — network resource acc test
- `task test:service_account` / `task test:shadowlink` — focused acc tests for these resources

Live acc tests require `REDPANDA_CLIENT_ID` + `REDPANDA_CLIENT_SECRET` and cloud-provider creds.

### Cleanup (for stuck/leaked resources)
- `task cleanup:aws:ci` — nuke BYOVPC AWS resources (auto-approve, CI-safe)
- `task cleanup:aws` — same but interactive
- `task cleanup:aws:nuke` — nuke all non-default VPCs in test regions (manual, last resort)
- `task cleanup:gcp` / `:gcp:ci` / `:gcp:dry` — GCP BYOVPC sweeper (interactive / auto-approve / preview)
- `task cleanup:redpanda` — delete stale `tfrp-*` clusters from Redpanda Cloud
- `task cleanup:redpanda:dry` — preview only

### Codegen / docs
- `task generate:models` — regenerate `redpanda/models/*/*_gen.go` and `redpanda/resources/**/schema_*_gen.go` from `schema.yaml`
- `task generate:clean` — delete all generated `*_gen.go` files under `redpanda/resources/` and `redpanda/models/`
- `task docs` — regenerate `docs/` via tfplugindocs
- `task mock` — regenerate `redpanda/mocks/*`

### Build / install / release
- `task build` — build `terraform-provider-redpanda` binary
- `task build:install` — install to local Terraform plugin cache
- `task local:cluster:aws:apply` / `:destroy` — apply an example stack locally

## Working directories convention

- `.claude/` — **committed**. Holds project skills (`.claude/skills/`) and any shared Claude Code project settings. `.claude/settings.local.json` is per-user and gitignored explicitly.
- `manual-tests/` — **gitignored**. Local working directory for everything ephemeral: manual test workspaces (`manual-tests/<resource>/`), audit reports, bug-hunt notes, sonnet-agent exploration findings, scratch consolidations. Skills point here for any artifact that isn't a contract.
- `.logs/` — gitignored. Reserved for log capture (monitor-logs skill, test-output dumps). Don't use for scratch documents.

When a skill spawns sonnet exploration agents or generates audit reports, they go to `manual-tests/<topic>/`, not `.claude/` and not `.logs/`.

## Repository Architecture

- `redpanda/redpanda.go` — provider entry: `New`
- `redpanda/cloud/` — gRPC client, `connpool`, `ratelimiter`, `controlplane.go`
- `redpanda/models/` — Terraform state ⇔ proto conversion. `*_gen.go` files are produced by codegen; do not hand-edit
- `redpanda/resources/<resource>/` — resource + datasource implementations (acl, cluster, network, pipeline, region, regions, resourcegroup, role, roleassignment, schema, schemaregistryacl, secret, serverlesscluster, serverlessprivatelink, serverlessregions, serviceaccount, shadowlink, throughputtiers, topic, user)
- `redpanda/resources/schemagen.go` — central `//go:generate` registry for all schemagen invocations
- `redpanda/resources/codegen.yaml` — global schemagen config (enum carve-outs, exclude list)
- `redpanda/tests/` — cluster-lifecycle live acc tests; BYOVPC infra-producer Terraform stacks live at `redpanda/tests/testdata/network/{aws,gcp}/`
- `redpanda/mocks/` — gomock-style client mocks (unit tests)
- `redpanda/utils/` — shared helpers (retry, backoff, etc.)
- `internal/schemagen/` — YAML-driven proto-to-schema generator (drives `*_gen.go` model + schema files via `task generate:models`)
- `internal/testutil/{acc,integration,mock,upgrade}/` — shared test infrastructure for colocated resource-package tests
- `cmd/schemagen/`, `cmd/enumgen/`, `cmd/apidesc-import/` — codegen binaries (schema, enum, API descriptor import)
- `scripts/cleanup-redpanda-byovpc/`, `scripts/cleanup-gcp-redpanda-byovpc/` — BYOVPC sweeper binaries (used by `task cleanup:aws:*` / `cleanup:gcp*`)
- `docs/` — generated by tfplugindocs; regenerate with `task docs`
- `examples/` — example `.tf` configs referenced by docs
- `templates/` — tfplugindocs templates

## Testing

Three tiers. Pick the narrowest that exercises the behavior.

| Tier | Where | When to use | Creds |
|------|-------|-------------|-------|
| Unit | `*_test.go` next to the code (no `//go:build integration`) | Pure logic, mapping, validation; uses `redpanda/mocks/` gomock clients. Run via `task test:unit`. | None |
| Colocated integration | `redpanda/resources/<r>/integration_*_test.go` (external `<r>_test` package, `//go:build integration`); shared helpers in `internal/testutil/{acc,integration,mock,upgrade}/` | Per-resource CRUD / import / drift flows in isolation. Run via `task test:integration`. | `REDPANDA_CLIENT_ID/SECRET` for live, none for mock variants |
| Live acc | `redpanda/tests/*_test.go` via `task test:<scope>` | Cluster lifecycle, BYOC, BYOVPC, cross-resource end-to-end flows | `REDPANDA_CLIENT_ID/SECRET` + cloud-provider creds |

Prefer unit tests with gomock clients for anything that can be exercised in-process. Reach for live acc only when the behavior genuinely requires a real cluster.

## Code Generation

Models and schemas: run `task generate:models` after editing a `schema.yaml` (or `schema_datasource.yaml`) under `redpanda/resources/<resource>/`. Review the `*_gen.go` diff before committing. The `//go:generate` directives are registered centrally in `redpanda/resources/schemagen.go`.

Golden files (`*.golden`) and `.description` files are **sacred** — never modify without explicit user approval. They pin contracts that drift silently without these guardrails.

Never swallow warnings or errors from codegen. Surface them.

## Documentation

`task docs` regenerates `docs/` from schema descriptions + `examples/` via tfplugindocs. Never hand-edit `docs/*.md` — they will be overwritten.

If you change a schema `Description` field, run `task docs` and commit the diff in the same commit.

## Git & PR Workflow

Never use `git checkout`, `git restore`, or `git reset --hard` to discard files without explicit user instruction. The user may have intentional in-progress work.

Never `git push` (including `--force` / `--force-with-lease`) without explicit approval for that specific push. Approval for one push does not carry forward — ask again before every subsequent push. Batch-committing is fine; batch-pushing is not.

Before every commit: `task ready` (or at minimum `task lint`).

### GitHub PR comments
- All comments on PR: `gh api repos/redpanda-data/terraform-provider-redpanda/pulls/<N>/comments --paginate`
- Single comment by ID: `gh api repos/redpanda-data/terraform-provider-redpanda/pulls/comments/<ID>`
- Comment ID comes from the URL fragment: `#discussion_r2382392371` → ID `2382392371`

## Conventions

- Comments: short, functional, essential. No background prose, no narration, no multi-paragraph explanations.
- Never add `//nolint`, `#nosec`, or any linter suppression without explicit user approval.
- `allow_deletion=false` in cluster tests is intentional and acts as a canary for testing failures. Fix the upstream failure, don't flip the flag.
- When reporting/fixing bugs: write a failing test first, show the red, *then* discuss fix scope separately.
- Clarifying questions from the user ("wait, is that correct?") are not pushback. Verify and answer — don't withdraw a correct finding.
- Never delete files or large blocks of code without summarizing and asking first.

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
4. Description tests
5. `task lint`
6. `task test:integration`

Don't skip ahead if one fails — keep going and report each result, including pass/fail counts, key failures, and how long each task took.
