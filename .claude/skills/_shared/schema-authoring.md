# Schema authoring (`schema.yaml`)

The schema YAML drives everything downstream — framework schema, model struct, Flatten/Expand, golden contract, docs. Authoring discipline lives here; codegen mechanics live in [codegen-workflow](codegen-workflow.md).

Per memory `project_schemagen_scope.md`: schemagen produces **schema + model + Flatten/Expand only**. CRUD is always hand-written (see [crud-glue](crud-glue.md)). Don't push schemagen scope into CRUD territory.

**Heads-up: not every resource is schemagen-driven.** Per memory `project_roleassignment_kafka_api.md`, `redpanda/resources/roleassignment/` is intentionally hand-written — it talks to the Kafka API surface, not a dataplane proto. Schema changes there go directly into the hand-written files; don't propose a `schema.yaml` migration.

## File locations

- Resource: `redpanda/resources/<name>/schema.yaml`
- Datasource: `redpanda/resources/<name>/schema_datasource.yaml`
- Global config (enum carve-outs, exclude list): `redpanda/resources/codegen.yaml`

## Top-level keys

```yaml
api_schema: redpanda.api.controlplane.v1.<MessageName>   # proto source — required
tf_name: <terraform_resource_name>                        # e.g. service_account — required
strip_openapi_prefix: v1.                                 # strip prefix from OpenAPI refs (some resources)
timeouts: [create, update, delete]                        # resource; datasource uses [read]
computed_default: true                                    # datasource only — flips Required→Computed
api:                                                       # RPC overrides; minimal block usually fine
  update:
    return_payload: true                                  # update RPC returns updated resource
  post_expand_hook: <pkg>.<Func>                          # mutate request after expand
```

## Field-level directives

### Lifecycle (mutually-exclusive groups)

- `computed_only: true` — server-populates; no user input
- `optional: true, computed: true` — mutable user input with server default (most common)
- `optional: true` (no `computed`) — immutable user input; pair with `plan_modifiers: [RequiresReplace]`
- `required: true` — user must set; **breaking change to add to an existing resource**

### Conversion control

- `extra: true` — TF-only field, no proto echo. Needs `type:` and often `flatten_via:`/`expand_via:`. Example: `cluster.allow_deletion`, `shadowlink.allow_deletion`.
- `proto_only: true` — proto field kept out of TF schema (e.g. `dataplane_api`, `type`). Generator excludes from struct entirely.
- `from_proto: <field_name>` — TF attribute name differs from proto field name. Used for rename (`cluster_type` ← proto `type`) and for canonical/deprecated aliases (`tags` ← `cloud_provider_tags`).
- `flatten_via: <funcName>` / `expand_via: <FuncName>` — hand-written conversion. Function lives in `redpanda/models/<name>/conv.go`. Used when proto↔TF shape doesn't match (e.g. `tagsFromProto` for map flattening).
- `flatten_skip: true` — field is Create-only in proto (server never echoes it on Get). Generator omits from Flatten; resource code must inject it manually after Create and carry forward in Read/Update. Example: `serviceaccount.client_secret`.
- `flatten_from_prev: true` — restore prior user value over API-canonicalized echo (e.g. `pipeline.resources` where API canonicalizes memory_shares/cpu_shares). Generator emits prev-fallback in Flatten.
- `force_type: BoolAttribute` — coerce attribute type. Used for presence-only oneof variants (e.g. `maintenance_window_config.anytime`).
- `todo: true` — proto field exists but TF surface deferred. `task generate:todo` auto-adds these for new proto fields. To surface the field, **remove** `todo: true` and add the correct lifecycle.

### Validators

Validator registry lives at `internal/schemagen/validators/`. Common ones:

- `OneOf[T]` (auto-derived from proto `buf.validate` enum rules — usually no manual entry needed)
- `RequireTrue` — for oneof-presence bool fields where `false` is unrepresentable in proto (`shadowlink.start_at_earliest`)
- `Format(uuid|url|email|...)` (auto-derived from proto `buf.validate.string.format`)

Add manually only when auto-derivation can't infer the rule.

### Plan modifiers

Most common:
- `[RequiresReplace]` — pair with immutable `optional`-only fields
- `[UseStateForUnknown]` — anchor server-defaulted optional+computed fields against perpetual diffs (rarely needed manually — classifier emits it for top-level optional+computed)

Memory `feedback_server_default_optional_computed.md`: proto-presence fields with server-populated defaults are `optional+computed+UseStateForUnknown`, not optional-only with Flatten workarounds.

### Sensitive / output formatting

- `sensitive: true` — marks attribute sensitive in TF state output (e.g. `serviceaccount.client_secret`)
- `description: "..."` — inline override for the apidescriptions.yaml description. Use only for gaps; apidesc is the default source. Memory `project_description_redundancy_diagnostic.md`: classifier detects redundant overrides.

## Deprecation / canonical-alias pattern

There is **no `deprecated_for:` directive** in schemagen — the README mentions it but the Go code doesn't implement it (drift to fix). Aliasing is hand-wired:

1. Mark the proto field name `proto_only: true` (e.g. `cloud_provider_tags`).
2. Add an `extra: true` TF attribute with `from_proto: <proto_name>` for the canonical TF name (e.g. `tags`).
3. Write `flatten_via: <funcName>` and `expand_via: <FuncName>` in `redpanda/models/<name>/conv.go` to handle the conversion both ways.
4. Mark the deprecated alias `deprecated: true` if surfaced as a separate TF attribute for a deprecation window.

See `redpanda/resources/cluster/schema.yaml` `tags` field for the working example.

## Datasource specifics

- `computed_default: true` — flips all attributes to Computed unless overridden
- `timeouts: [read]` instead of CRUD timeouts
- Often a leaner subset of the resource's fields; `exclude: true` drops fields that shouldn't surface in the datasource shape

## Design principle: prefer per-field declarative directives

Per memory `feedback_prefer_per_field_directives.md`: when a reconciliation or override pattern needs to be applied uniformly across multiple fields, propose a per-field declarative YAML directive first. Resource-wide flags or post-hooks are escape hatches, not defaults. Adding a hook for one field reads to a reviewer as "this is a special case" — adding a directive reads as "this is the pattern; here are the fields it applies to."

## Don'ts

- **Don't hand-edit `*_gen.go` files** — fix the generator at `internal/schemagen/` or the YAML, then regenerate (memory `feedback_no_manual_codegen_fixes.md`).
- **Don't reorder existing entries** — diff noise hides the real change. When extending, insert into the existing tree in proximity-relevant order.
- **Don't add `description:` overrides that duplicate apidesc.** Run `task generate:apidescriptions` first; the classifier will flag duplicates as INFO.

## See also

- [codegen-workflow](codegen-workflow.md) — how to regenerate and what files change
- [crud-glue](crud-glue.md) — when YAML directives require hand-written CRUD support
