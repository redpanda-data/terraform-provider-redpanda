# Codegen workflow

Regenerating after schema.yaml changes, and what to look for in the diff.

## Commands

```bash
task generate:models           # schemas → models; the workhorse
task generate:apidescriptions  # pull new field descriptions from cloudv2; run first if proto changed
task generate:todo             # auto-add `todo: true` entries for new proto fields
task generate:golden           # update golden baselines (requires explicit user approval)
task generate:clean            # delete all *_gen.go files (then regen)
```

`task ready` (run before every commit) chains `docs:default + lint:default + build:tidy`. Run `task generate:models` before `task ready` when schemas change. Per memory `feedback_lint_regularly.md`, run lint frequently during dev — don't save it all for the pre-commit pass.

## Proto sources

Per memory `project_proto_sources.md`: schemagen resolves protos from **two** sibling repos, not just one.

- **`../cloudv2`** — controlplane protos: Cluster, Network, ResourceGroup, Region, ServerlessCluster, etc.
- **`../console`** — dataplane protos: User, Topic, ACL, Schema, SchemaRegistryACL, Pipeline, etc.

The `-cloudv2` flag (or `CLOUDV2_ROOT` env var) points the schemagen binary at cloudv2; console is resolved via cloudv2's buf dependency on `buf.build/redpandadata/dataplane`. Per memory `reference_redpanda_repos.md`, the canonical local paths are:

- `~/GolandProjects/cloudv2`
- `~/GolandProjects/console`
- `~/GolandProjects/documentation` (for terminology cross-reference when authoring `Description:` fields)

If the sibling checkout is missing or stale, `task generate:models` fails with proto resolution errors. Fix the checkout, don't paper over the failure.

## What gets produced per resource

A `//go:generate` line in `redpanda/resources/schemagen.go` emits **four** files (not five — `resource_model_gen_test.go` does not exist):

1. `redpanda/resources/<name>/schema_resource_gen.go` — framework schema function
2. `redpanda/resources/<name>/proto_validator_gen.go` — `ConfigValidators()` method on the resource struct
3. `redpanda/models/<name>/resource_model_gen.go` — `ResourceModel` struct with `tfsdk:""` tags, `AttrType` tables, `GenerateMinimalResourceModel`, `AsX`/`XToObject` round-trip helpers
4. `redpanda/models/<name>/conv_gen.go` — `Flatten`, `ExpandCreate`, `ExpandUpdate`, all sub-converters

Datasources produce the equivalents in `schema_datasource_gen.go` / `datasource_model_gen.go`.

`go generate ./redpanda/models/...` (second pass in `task generate:models`) is effectively a no-op — the models directory has no `//go:generate` directives. All model files are written by the schemagen binary via `-model-output` and `-conv-output` flags.

## Reviewing the diff

A clean small-field-extend diff touches exactly:
- One new attribute block in `schema_resource_gen.go`
- One new struct field in `resource_model_gen.go`
- One new `if proto.HasX() ... else if prev ... else null` block in `conv_gen.go`'s Flatten
- The new field in `GenerateMinimalResourceModel`'s defaults
- The new field included in `ExpandCreate`/`ExpandUpdate` struct literals (if proto-mapped)

**Any other file touched by the diff is a red flag** — investigate before committing. A one-line YAML change can silently cascade into every nested sub-converter in `conv_gen.go`. Read the full `conv_gen.go` diff, not just the struct field.

## Classifier diagnostics

`internal/schemagen/classifier.go` emits to stderr during `task generate:models`:

- `INFO classifier: ...` — your override matches what the classifier would emit anyway. The override is redundant; remove it.
- `WARN classifier: ...` — your override **conflicts** with what the classifier inferred. Investigate before continuing.

Pipe regen output through `grep -E 'INFO|WARN'` to surface these. Per memory `feedback_golden_files.md`, never swallow warnings.

`apidesc: X/Y attrs matched` also appears — a drop in the ratio after adding a field means the description is missing. Either run `task generate:apidescriptions` or add an inline `description:` override.

## Golden files (`*.golden`)

These are **sacred** — memory `feedback_golden_files.md`. Never edit by hand to make a test pass. They pin the schema contract that drifts silently otherwise. (Descriptions are not golden-tested; they flow from `apidescriptions.yaml` and are validated via `task docs`.)

When a golden test fails, per memory `feedback_show_golden_diffs.md`: **paste the raw diff to the user before any summary or interpretation.** The user needs to see exactly which lines changed before deciding whether the change is intentional or a regression. Running `task generate:golden` to "fix" a failing test without showing the diff first is exactly the wrong move.

### Workflow

```bash
# Verify nothing has drifted (the assertion path)
go test ./redpanda/resources/ -run TestSchemaGolden

# Update goldens for legitimate schema changes (requires user approval)
task generate:golden
# Or targeted to a single resource:
go test ./redpanda/resources/ -run "^TestSchemaGolden$/^<name>_(resource|datasource)$" -update
```

When extending: review the `.golden` diff line-by-line. A small YAML change should produce a tight golden diff (one added attribute line). If the diff is larger than expected, the YAML edit had side effects — back out and investigate.

## Golden baseline for a brand-new resource

A new resource needs a baseline golden before `task test:unit` will pass:

1. Add a new entry to the `tests` slice in `redpanda/resources/schema_golden_test.go` (around `:64-91`).
2. Run `task generate:golden` to create `redpanda/resources/testdata/<name>_(resource|datasource)_schema.golden`.
3. Commit the baseline in the same commit as the schema YAML.

## Schema golden baseline missing on extend

If you're extending a resource whose `*.golden` doesn't exist, the parity check has no anchor — generation can silently change the schema users see on upgrade. **Establish the baseline from `main` first** before any YAML edit:

```bash
git worktree add /tmp/tfrp-baseline main
cd /tmp/tfrp-baseline
task generate:golden
cp redpanda/resources/testdata/<name>_*.golden <repo>/redpanda/resources/testdata/
```

Then commit the baseline separately as a "no behavioral change" commit before starting the extend work.

## Schemagen CLI flags (in `//go:generate` directives)

Actual flags from `cmd/schemagen/main.go`:

```
-proto-pkg, -message, -config, -func, -type, -output, -package,
-todo, -api-descriptions, -model-output, -model-package, -conv-output,
-proto-import, -proto-alias, -cloudv2
```

Flags that **do not exist** (don't use): `-model-test-output`, `-resource-import`.

`CLOUDV2_ROOT` env var or `-cloudv2` flag points at the cloudv2 sibling repo. Defaults to `../cloudv2`.

## Other codegen binaries

- `cmd/enumgen/` — produces `internal/schemagen/enums/enums_gen.go` (registered enum carve-outs)
- `cmd/apidesc-import/` — produces `internal/schemagen/data/apidescriptions.yaml` from `../cloudv2` checkout

## See also

- [schema-authoring](schema-authoring.md) — what to put in the YAML in the first place
- [testing-tiers](testing-tiers.md) — golden test is one of several test tiers
