# Docs and examples

How `task docs`, example HCL files, and `templates/` interact.

## Three example trees

| Path | Purpose | Required for |
|------|---------|--------------|
| `examples/<name>/main.tf` + `variables.tf` | Live acc test fixture | Tier 3 acc tests via `config.StaticDirectory` |
| `examples/docs/<name>/main.tf` | Doc-page example | Referenced from `templates/resources/<name>.md.tmpl` via `{{ tffile }}` |
| `examples/datasource/<name>/main.tf` | Datasource doc example | Only if the resource has a datasource |

Colocated integration tests (Tier 2) use **inline HCL strings** built by helper functions like `awsDedicatedConfig(name, extras)` — no examples directory update needed for integration-tier work.

## `task docs`

Regenerates `docs/resources/<name>.md` and `docs/data-sources/<name>.md` via `tfplugindocs`. Already part of `task ready` (which is the pre-commit gate).

**Never hand-edit `docs/*.md`** — files are regenerated and the diff will fail CI.

The schema section (`## Schema`) is auto-populated from each attribute's `Description` field at runtime. Everything outside `{{ .SchemaMarkdown }}` in the template is hand-authored.

## Templates (`templates/resources/<name>.md.tmpl`)

**Optional but recommended.** If absent, tfplugindocs generates a default doc with no custom sections. Currently 15 resources have templates; `serverless_private_link` doesn't and still gets a generated doc.

Add a template when you want:
- Front-matter customization
- Hand-written prose above the schema
- Import section with example syntax
- Notes on sensitive fields (e.g. client_secret handling)
- API reference link

Datasource templates live in `templates/data-sources/<name>.md.tmpl`. Most datasources omit the template and use the tfplugindocs default.

## Datasource attribute naming

Per memory `feedback_verify_datasource_schema.md`: datasource attribute names often diverge from resource names — they're not automatic mirrors. Before authoring `examples/datasource/<name>/main.tf` or any datasource HCL, grep `docs/data-sources/<name>.md` to confirm the exact attribute names. A `terraform plan` against a wrongly-named datasource attribute fails noisily; the fix is faster if you check first.

## Description quality

`docs/resources/<name>.md`'s schema section is only as good as the attribute descriptions. Sources, in order of precedence:

1. Inline `description:` override in `schema.yaml`
2. `internal/schemagen/data/apidescriptions.yaml` (imported from cloudv2 by `task generate:apidescriptions`)
3. Empty (the apidesc match drops; visible in regen output as `apidesc: X/Y attrs matched`)

Memory `project_description_redundancy_diagnostic.md`: a classifier flags inline descriptions that duplicate the apidesc entry. Remove those — they're dead weight.

When extending: if the new proto field has a description in cloudv2, `task generate:apidescriptions` will pull it. If not (or if the proto-side description is poor), add an inline `description:` in `schema.yaml`.

## When to update example files when extending

- **`examples/docs/<name>/main.tf`** — only when the new field is Required, or when omitting it would make the example misleading. For optional/computed server-default fields, leave alone.
- **`examples/<name>/main.tf`** + `variables.tf` — only if the live acc test exercises the new field. Most scalar field additions don't need this.
- **No new template files for extends** — existing `<name>.md.tmpl` stays; the regenerated `## Schema` section picks up the new attribute automatically.

## When adding a new resource

- Write `examples/<name>/main.tf` with at least one `redpanda_<name>` block using realistic values.
- Write `examples/<name>/variables.tf` declaring any inputs the acc test plumbs through `ConfigVariables`.
- Write `examples/docs/<name>/main.tf` — usually the same content minus the variable indirection.
- Write `templates/resources/<name>.md.tmpl` — copy from a similar-shape resource (e.g. `serviceaccount.md.tmpl` for control-plane CRUD).
- Run `task docs` and verify `docs/resources/<name>.md` looks right.

## See also

- [schema-authoring](schema-authoring.md) — where `description:` overrides live
- [testing-tiers](testing-tiers.md) — Tier 2 inline HCL vs Tier 3 examples directory
