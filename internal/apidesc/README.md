# apidesc

OpenAPI description loader. Reads OpenAPI specs (cloudv2 + console), walks `components.schemas`, and produces a queryable index of per-field descriptions. Used by schemagen to fill in attribute descriptions the proto doesn't carry.

## Exports

- `LoadSpec(path) (*Spec, error)`: parse one OpenAPI YAML.
- `Flatten(specs) (tree, warnings, error)`: flatten N specs into a single `map[string]*Node` keyed by `<rootSchema>.<dotted.field.path>`.
- `FilterByRoots(tree, roots)`: narrow the tree to a subset of root schemas (e.g. only the schemas a given resource references).
- `Load(path) (*Index, error)` / `Encode(file, headerComment) ([]byte, error)`: read / write the bundled `apidescriptions.yaml`.
- `Index.Lookup(path) (description, bool)`: per-field lookup used by `internal/schemagen/merger.go::applyAPIDescriptions`.

## Consumers

- `cmd/apidesc-import` — uses `LoadSpec` + `Flatten` + `Encode` to refresh the bundle.
- `cmd/schemagen` — uses `Load` to read the bundle at gen time.
- `internal/schemagen/merger.go` — uses `Index.Lookup` to fill empty attribute descriptions.

No dependency on the rest of `internal/schemagen`. Self-contained.
