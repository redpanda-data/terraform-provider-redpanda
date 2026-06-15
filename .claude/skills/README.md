# Project skills

Skills committed here apply to anyone working in this repo via Claude Code. Each subdir is one skill, with a `SKILL.md` containing the YAML frontmatter that Claude Code reads on session start.

## Skills

- **`add-redpanda-resource/`** — scaffolding a brand-new resource or datasource (new `redpanda/resources/<name>/` directory).
- **`extend-redpanda-resource/`** — adding or modifying fields on an existing resource or datasource.
- **`resolve-redpanda-bug/`** — diagnosing and fixing a bug, either from a user report or via a proactive coverage audit. Calibrated for the "mock-loop passes but live fails" bug class.
- **`live-acctest-orchestration.md`** — driving a multi-target live-acc campaign against preprod (per-target logs, notifications, bug delegation, cleanup discipline). A single-file skill, not a subdir.
- **`_shared/`** — focused reference docs (schema-authoring, codegen-workflow, crud-glue, provider-registration, testing-tiers, docs-and-examples, manual-validation) that the entry skills cross-reference. Not invoked directly.

## Working directories

- `.claude/` — committed (this directory).
- `manual-tests/` — gitignored. Sonnet exploration findings, audit reports, manual test workspaces, scratch consolidations all land there.
- `.logs/` — gitignored. Reserved for log capture (monitor-logs skill, test outputs).

If you spawn exploration agents while running a skill, point them at `manual-tests/<topic>/`. The skill files document this convention in each "Plan first" section.

## Editing skills

Project skills are committed and shared. Treat edits like code changes:

1. Make the edit on a branch.
2. Run `task ready` before commit (docs + lint + `go mod tidy`).
3. Get review like any other PR — skills shape how future Claude instances behave, so changes have leverage.

The `_shared/` files are referenced by multiple entry skills; a one-line change there propagates everywhere. Be careful with sweeping edits.
