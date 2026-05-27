# CRUD glue (hand-written `resource_<name>.go`)

Schemagen produces schema + model + Flatten/Expand. CRUD methods (`Create`, `Read`, `Update`, `Delete`, `ImportState`) stay hand-written — this file is the canonical guide.

Per memory `project_schemagen_scope.md`: this scope split is intentional and durable — CRUD is always hand-written. Don't propose moving CRUD into schemagen. Per memory `feedback_no_manual_codegen_fixes.md`: if Flatten/Expand misses a case, fix the generator (or the YAML directive that drives it), don't paper over by hand-editing `conv_gen.go`.

## Default position: zero CRUD changes when extending

When adding a normal proto-mapped field (scalar, nested, enum, list) to an existing resource, `Flatten`/`ExpandCreate`/`ExpandUpdate` are fully regenerated. **Touch nothing in `resource_<name>.go`.**

Touch CRUD only when the new field has one of:

1. `extra: true` (TF-only, no proto echo) — needs population logic
2. `flatten_via:` or `expand_via:` — the referenced function must exist or be written in `redpanda/models/<name>/conv.go`
3. FieldMask Update behavior change (rare; cluster only)
4. `ImportState` default for an `extra:` field — use `utils.ImportStateBoolFromSchemaDefault`

## Resource skeleton (new resource)

```go
package <name>

import (
    "context"

    "github.com/hashicorp/terraform-plugin-framework/resource"
    "github.com/redpanda-data/terraform-provider-redpanda/redpanda/resources/base"
    "github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
    <name>model "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/<name>"
)

type <Name> struct {
    base.ResourceBase
}

var (
    _ resource.Resource                = &<Name>{}
    _ resource.ResourceWithConfigure   = &<Name>{}
    _ resource.ResourceWithImportState = &<Name>{}
)

func New<Name>() resource.Resource {
    return &<Name>{
        ResourceBase: base.NewResourceBase("redpanda_<name>", Resource<Name>Schema, nil),
    }
}
```

Third arg to `NewResourceBase` is `nil` for control-plane resources and a `clientFactory` for dataplane resources.

## Create

```go
func (r *<Name>) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
    var plan <name>model.ResourceModel
    if resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...); resp.Diagnostics.HasError() {
        return
    }

    request := plan.ExpandCreate(ctx, &resp.Diagnostics)
    if resp.Diagnostics.HasError() {
        return
    }

    op, err := r.CpCl.<Name>.Create<Name>(ctx, request)
    if err != nil {
        resp.Diagnostics.AddError("failed to create <name>", utils.DeserializeGrpcError(err))
        return
    }

    // For async resources, poll utils.AreWeDoneYet(ctx, op, timeout, ...) before Flatten.

    var model <name>model.ResourceModel
    if err := model.Flatten(ctx, op.GetResponse().GetResource(), nil); err != nil { /* handle */ }

    resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}
```

## Read

```go
func (r *<Name>) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
    var state <name>model.ResourceModel
    if resp.Diagnostics.Append(req.State.Get(ctx, &state)...); resp.Diagnostics.HasError() {
        return
    }

    got, err := r.CpCl.<Name>ForID(ctx, state.ID.ValueString())
    if utils.IsNotFound(err) {
        resp.State.RemoveResource(ctx)   // 404 → out-of-band delete; never an error
        return
    }
    if err != nil {
        resp.Diagnostics.AddError("failed to read <name>", utils.DeserializeGrpcError(err))
        return
    }

    var model <name>model.ResourceModel
    if err := model.Flatten(ctx, got, &state); err != nil { /* handle */ }

    resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}
```

## Update (FieldMask pattern)

For RPCs that accept `update_mask`:

```go
planExpand := plan.ExpandUpdate(ctx, &resp.Diagnostics)
stateExpand := state.ExpandUpdate(ctx, &resp.Diagnostics)
if resp.Diagnostics.HasError() { return }

req, mask, err := utils.PlanPayloadWithUpdateMask(planExpand, stateExpand)
if err != nil { /* handle */ }
if len(mask.GetPaths()) == 0 {
    // No mutable diff; fall back to Read for stability
    return r.Read(ctx, ...)
}
req.UpdateMask = mask

op, err := r.CpCl.<Name>.Update<Name>(ctx, req)
// ... handle response, Flatten, set state
```

For cluster, use `utils.GenerateProtobufDiffAndUpdateMask` instead — it's the cluster-specific variant that handles the deeper nesting.

## Delete

Straightforward — call the Delete RPC, handle `IsNotFound` as already-gone (no error).

## ImportState

Most resources:

```go
func (r *<Name>) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
    resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
```

For resources with `extra: true` bool fields that need a default on import:

```go
utils.ImportStateBoolFromSchemaDefault(ctx, path.Root("allow_deletion"), false, resp)
```

For composite-key import (e.g. ACL), parse `req.ID` and call `resp.State.SetAttribute` for each key.

## Hand-written conversion (`redpanda/models/<name>/conv.go`)

When YAML uses `flatten_via:` / `expand_via:`, write the named functions in `conv.go`:

```go
// flatten_via target
func tagsFromProto(proto *cpb.Resource) basetypes.MapValue { ... }

// expand_via target — method on ResourceModel
func (m *ResourceModel) TagsForProto() map[string]string { ... }
```

The generated code calls these by name; the signatures are conventional but not enforced by codegen — match an existing example.

## The `flatten_skip:` + Create-only field pattern

When a proto field is populated only in the Create response and never echoed by Get/Update (e.g. `serviceaccount.client_secret`):

1. YAML: `flatten_skip: true, sensitive: true, computed_only: true`
2. Generator omits the field from Flatten.
3. **Create**: after `Flatten(ctx, createResp, nil)`, manually inject the value: `model.ClientSecret = types.StringValue(op.GetClientSecret())`.
4. **Read/Update**: carry the prior value forward — `model.ClientSecret = state.ClientSecret`.

The auto-preserve infrastructure doesn't cover this case; the resource must handle it explicitly.

## Error message convention

`resp.Diagnostics.AddError("failed to <verb> <resource_name>", utils.DeserializeGrpcError(err))`. Verbs: create, read, update, delete, import. Use lowercase, no terminal period.

## Datasource shape

Datasources have `Read` only — no Create/Update/Delete/ImportState. The `Read` pattern is the same as the resource's `Read` minus the `RemoveResource` branch — on not-found, call `resp.Diagnostics.AddError`, don't silently drop.

Per memory `feedback_verify_datasource_schema.md`: **datasource attribute names often diverge from resource attribute names** — they're not automatic mirrors. Before authoring datasource HCL (for examples, tests, or docs), grep `docs/data-sources/<name>.md` to confirm the exact attribute names. Don't assume parity with the resource.

## See also

- [schema-authoring](schema-authoring.md) — which YAML directives trigger CRUD attention
- [provider-registration](provider-registration.md) — wiring the new resource type into the provider
