# cmd/enumgen

Consolidates proto enum mappers into a single package (`redpanda/utils/enums`) so generated Flatten/Expand code can reference one canonical `XToString` / `StringToX` per enum. Replaces ad-hoc per-resource mappers.

## Inputs

- Proto packages it scans (controlplanev1, dataplanev1, etc.) for `enum` types.
- `redpanda/resources/codegen.yaml` `enum_carveouts:` section listing enums whose v1.9.0 state form differs from the default (e.g. lowercase `"public"` vs the proto-prefixed `"CONNECTION_TYPE_PUBLIC"`). For each carve-out, enumgen skips emission and expects a hand-rolled mapper in `redpanda/utils/enums/handrolled.go`. Parity between the carve-out list and `handrolled.go`'s exported `XToString` / `StringToX` pairs is enforced at gen time.

## Flags

- `-cloudv2 <path>`: cloudv2 repo root. Also via `CLOUDV2_ROOT` env.
- `-codegen <path>`: codegen.yaml location (default `redpanda/resources/codegen.yaml`).
- `-handrolled <path>`: handrolled.go location for the parity check (default `redpanda/utils/enums/handrolled.go`).
- `-out <path>`: output file (default `redpanda/utils/enums/enums_gen.go`).

## Output

- `redpanda/utils/enums/enums_gen.go` — auto-generated mappers.
- `redpanda/utils/enums/handrolled.go` — carve-outs (committed; updated when a new carve-out is needed).

## Typical usage

Runs as part of `task generate:schemas`. Re-run only when proto enum sets change or a state-form mismatch surfaces (manual-test cycle finding).
