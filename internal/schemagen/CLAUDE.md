# schemagen

YAML-driven proto-to-schema generator. `redpanda/resources/<r>/schema.yaml` plus the pinned proto descriptors (`internal/buf_dependencies.yaml`) produce `schema_*_gen.go`, `proto_validator_gen.go`, and the model `*_gen.go` / `conv_gen.go` files. Invocations are registered in `redpanda/resources/schemagen.go`; `task generate:models` runs enumgen first, then this.

## Directives are the contract

`config.go` is the source of truth: the `Config`, `APIConfig`, `RPCConfig`, and `FieldConfig` struct tags are the complete set of yaml keys, and unknown keys are rejected. Do not learn directives from `README.md` or older yaml; grep the struct.

Reach for them in this order, and stop at the first that fits:

1. Shape: `required`, `optional`, `computed`, `computed_only`, `sensitive`, `write_only`, `default`, `minimal_default`, `validator`, `plan_modifiers`.
2. Presence and drift: `has_presence`, `flatten_from_prev`, `echo_unwrap`.
3. Naming and lifecycle: `rename`, `deprecated` + `deprecation_message`, `exclude`, `todo`, `synthetic`, `extra`, `force_type`, `element_type`.
4. Conversion escape hatches: `flatten_via`, `expand_via`, `from_proto`, `expand_proto_name`, `flatten_skip`, `expand_skip`, `proto_only`, `updatable_out_of_band`.
5. Hooks on the `api` block (`pre_validate_hook`, `post_expand_hook`) only when no per-field directive can express it.

Deprecation aliases have no directive; they are built from `proto_only` + `from_proto` + `flatten_via` by hand.

## Retiring an attribute: tombstone, do not exclude

Terraform ignores an unknown key inside a nested attribute, so `exclude: true` on a nested field that a release already shipped makes existing configs silently lose it. Keep the field as a tombstone: `synthetic: true`, `optional: true`, a `deprecation_message` naming the replacement, and `fields:` whose leaf names and `type:` match the released schema so the old config still parses. Then make it fail at plan:

- Field still on the proto: `synthetic` alone. The generated conversion still expands it, so the control plane's `buf.validate` CEL rule fires through `proto_validator_gen.go` with the API's own message. `shadowlink` `shadow_schema_registry_api.tls_settings.tls_file_settings` is the example.
- Field never on the proto: add `extra: true`, `deprecated: true`, and a hand `validator:` that rejects any non-null value with the migration text. `serverlessprivatelink` `cloud_provider_config` is the example.

Do not combine `extra`/`deprecated` with a proto-backed field: `merger.go` appends the synthetic attribute next to the proto-derived one instead of replacing it, and generation aborts with `no lifecycle declared` on the proto copy. Pin every tombstone with a plan-time `ExpectError` integration test; the schema golden cannot tell a deliberate removal from an accidental one.

## Rules the generator encodes

- Descriptions come from `internal/apidesc` (`data/apidescriptions.yaml`). A yaml `description:` is rejected. Terraform-only fields use the tables in `descriptions.go`.
- `RequiresReplace` is derived from the update RPC's mask contract (`mask_contract.go`, `writeshape.go`), never from proto `IMMUTABLE` annotations. `updatable_out_of_band` is the opt-out for fields mutated by a side RPC.
- Plan modifiers emit `UseStateForUnknown` before `RequiresReplace`; the framework nulls unknowns first, and the reverse order arms a replace on every plan.
- A `oneof` arm the user selects is never `Computed`.
- A field the server defaults is `Optional` + `Computed` + `UseStateForUnknown`, not `Optional` with a flatten workaround.
- `merger.go` returns errors as a hard failure for `cmd/schemagen`; warnings are printed and must be read, never silenced.

## Tests

- `testdata/*.golden` pin generator output; `go test ./internal/schemagen/ -run TestFlattenExpandGolden -update-golden` rewrites them and needs explicit approval first, like every golden.
- `redpanda/resources/testdata/` holds the schema goldens checked by `redpanda/resources/schema_golden_test.go`.
- After any change here: `task generate:models`, then review every `*_gen.go` diff before committing.
