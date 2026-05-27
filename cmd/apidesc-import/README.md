# cmd/apidesc-import

Builds `internal/schemagen/data/apidescriptions.yaml` from upstream OpenAPI specs in the cloudv2 and console repos. The bundled YAML is what `cmd/schemagen` reads to fill in attribute descriptions the proto doesn't carry.

## Flags

- `-repo <path>`: local cloudv2 checkout. Required unless `-spec-dir` is set.
- `-spec-dir <path>`: directory of cloudv2 `openapi.*.yaml` files. Defaults to `<repo>/proto/gen/openapi`.
- `-console-repo <path>`: local console checkout. Optional; falls back to `CONSOLE_ROOT` env or `../console`.
- `-console-spec-dir <path>`: directory of console `openapi.*.yaml` files. Defaults to `<console-repo>/proto/gen/openapi`.
- `-output <path>`: target file. Defaults to `<terraform-provider-redpanda>/internal/schemagen/data/apidescriptions.yaml`.
- `-resource-dir <path>`: directory of resource subdirs containing `schema*.yaml`, used to scope `api_schema:` filtering. Defaults to `<repo>/redpanda/resources`.
- `-include <glob>`: filename glob for spec files (may be repeated; default `openapi*.yaml`).
- `-no-filter`: skip `api_schema:` scoping and emit every schema in the openapi specs (debugging only).
- `-quiet`: suppress per-file progress output.

## Typical usage

```
task generate:apidescriptions
```

Runs only when upstream OpenAPI specs drift (proto comment changes, new fields, etc.). The output's header pins the source commits so reviewers can confirm provenance.

## Output shape

YAML index keyed by root schema → dotted field path → description. Filtered to the union of `api_schema:` roots referenced by `redpanda/resources/*/schema.yaml` (currently 13 schemas).
