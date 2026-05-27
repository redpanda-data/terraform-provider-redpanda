# cmd/schemagen

CLI driver for the schemagen pipeline (`internal/schemagen`). Invoked once per resource by the centralised `//go:generate` directives in `redpanda/resources/schemagen.go`. Not designed for standalone hand-invocation — the directives pass every flag this binary needs; start there if you want to understand how a given resource is emitted.

## Flags

Inputs:
- `-cloudv2 <path>`: cloudv2 repo root. Also via `CLOUDV2_ROOT` env; defaults to a sibling-checkout lookup.
- `-proto-pkg <path>`: proto package path (e.g. `redpanda/api/controlplane/v1`).
- `-message <name>`: name of the proto message (e.g. `Cluster`).
- `-config <path>`: resource `schema.yaml`.
- `-api-descriptions <path>`: override the `apidescriptions.yaml` location (default: auto-detected relative to go.mod).
- `-type <resource|datasource>`: schema variant being emitted (default `resource`).

Schema output:
- `-func <name>`: name of the generated schema function.
- `-output <path>`: output file path for the schema.
- `-package <name>`: Go package name (defaults from `-output` dir).

Model output (optional):
- `-model-output <path>`: if set, also write the model struct here.
- `-model-package <name>`: Go package for the model file (defaults from `-model-output` dir).

Conversion output (optional, requires the resource `schema.yaml` `api:` block):
- `-conv-output <path>`: if set, write generated Flatten/Expand code here.
- `-proto-import <path>`: Go import path for the proto package (required with `-conv-output`).
- `-proto-alias <name>`: Go import alias for the proto package (defaults to the last segment of `-proto-import`).

Authoring helpers:
- `-todo`: emit `todo: true` stubs for proto fields not yet covered by the yaml. Also via `SCHEMAGEN_TODO=1` env.

## Typical usage

```
task generate:schemas                    # all resources via go generate
SCHEMAGEN_TODO=1 task generate:schemas   # backfill yaml stubs after a proto bump
```
