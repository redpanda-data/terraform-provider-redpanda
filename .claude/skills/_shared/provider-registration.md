# Provider registration

How a new resource or datasource gets wired into the provider. **N/A for extend** — only relevant when scaffolding a new resource type.

## Three parallel registrations

A new resource needs three separate registration edits:

1. **Provider** — `redpanda/redpanda.go`
2. **Codegen** — `redpanda/resources/schemagen.go` (a new `//go:generate` line)
3. **Golden test** — `redpanda/resources/schema_golden_test.go`

Miss any one and the resource won't actually work end-to-end. All three are pre-existing for any resource being extended.

## 1. `redpanda/redpanda.go`

Two slices: `Resources()` (around `:509-530`) and `DataSources()` (around `:494-506`). Each entry is a single-line factory lambda:

```go
func() resource.Resource { return <name>.New<Name>() },
```

Multi-line form exists historically (first entry, `resourcegroup`); don't copy. **Single-line is the convention.**

Also update the import block (`:38-58`):

```go
"github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/<name>"
```

If the package name collides with a framework or stdlib identifier (e.g. `schema`), alias it:

```go
<name>resource "github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/<name>"
```

The existing `schema` package uses the alias `schemaresource` for exactly this reason.

**Ordering:** neither slice is sorted. Appending at the end is the established pattern. Don't bother re-sorting.

## 2. `redpanda/resources/schemagen.go`

The central `//go:generate` registry. Each resource/datasource gets one line:

```go
//go:generate go run github.com/.../cmd/schemagen \
//   -proto-pkg <pkg> -message <Msg> -config <name>/schema.yaml \
//   -func Resource<Name>Schema -type <Name> \
//   -output <name>/schema_resource_gen.go -package <name> \
//   -model-output ../models/<name>/resource_model_gen.go -model-package <name> \
//   -conv-output ../models/<name>/conv_gen.go \
//   -proto-import <proto-import-path> -proto-alias <alias>
```

Copy an existing line for a resource of the same shape and edit. The full flag set is documented in [codegen-workflow](codegen-workflow.md).

For a datasource, add a second line with `schema_datasource.yaml` + `-func DataSource<Name>Schema` + datasource output paths.

## 3. `redpanda/resources/schema_golden_test.go`

Add a new entry to the `tests` slice (around `:64-91`):

```go
{name: "<name>_resource", schema: <name>.Resource<Name>Schema(ctx).Schema},
{name: "<name>_datasource", schema: <name>.DataSource<Name>Schema(ctx).Schema}, // if applicable
```

Then run `task generate:golden` to create the baseline `redpanda/resources/testdata/<name>_(resource|datasource)_schema.golden` file. Without the baseline, `task test:unit` fails on first run.

## See also

- [codegen-workflow](codegen-workflow.md) — the full `//go:generate` flag set
- [testing-tiers](testing-tiers.md) — `TestSchemaGolden` and where the baseline matters
