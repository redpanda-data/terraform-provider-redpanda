# schemagen

YAML-driven proto-to-schema codegen pipeline. Reads protobuf descriptors plus per-resource `schema.yaml` configs and emits Terraform schemas, models, Flatten/Expand conversions, and proto-validators.

## Inputs

- Protobuf descriptors compiled via `protocompile` from the cloudv2 + console proto trees.
- Per-resource `redpanda/resources/<name>/schema.yaml` declaring field overrides (`required`, `computed`, `validators`, `plan_modifiers`, `flatten_from_prev`, `flatten_via`, etc.).
- `internal/apidesc` index (`internal/schemagen/data/apidescriptions.yaml`) for OpenAPI field descriptions. Yaml `description:` overrides are not supported; terraform-only fields and provider-behavior exceptions use the curated tables in `descriptions.go` (`commonDescriptions`, `scopedDescriptions`).

## Outputs (per resource)

- `redpanda/resources/<name>/schema_resource_gen.go` and `schema_datasource_gen.go`
- `redpanda/resources/<name>/proto_validator_gen.go`
- `redpanda/models/<name>/resource_model_gen.go`, `data_model_gen.go`, `conv_gen.go`, `data_conv_gen.go`
- Golden snapshots under `redpanda/resources/testdata/`

## Driving it

`task generate:models` runs the full pipeline (`go generate ./redpanda/resources/...` then `go generate ./redpanda/models/...`). `task generate:schemas` runs only the resources pass.

## Authoring guide

See `.claude/skills/add-redpanda-resource/SKILL.md` + `reference.md`.
