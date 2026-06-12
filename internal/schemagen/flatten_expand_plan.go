// Copyright 2023 Redpanda Data, Inc.
//
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package schemagen

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

const (
	protoKindEnum       = "enum"
	protoKindTimestamp  = "timestamp"
	protoKindDuration   = "duration"
	protoKindBytes      = "bytes"
	protoKindJSONStruct = "json_struct"
	protoKindStatus     = "status"
	goTypeFloat64       = "float64"
)

// scalarShape holds the per-TF-AttrType strings needed to emit Flatten/Expand
// for primitive scalars (String/Bool/Int32/Int64/Float64).
type scalarShape struct {
	tfTypeWord    string // types.<Word>Value / types.<Word>Null, also the receiver name in the method-expression callback
	zeroSentinel  string // RHS of the "v != X" check in the heuristic FlattenStmt
	expandValueFn string // method to call when the proto field is required
}

var scalarShapes = map[string]scalarShape{
	AttrTypeString:  {"String", `""`, ".ValueString()"},
	AttrTypeBool:    {"Bool", "false", ".ValueBool()"},
	AttrTypeInt32:   {"Int32", "0", ".ValueInt32()"},
	AttrTypeInt64:   {"Int64", "0", ".ValueInt64()"},
	AttrTypeFloat64: {"Float64", "0.0", ".ValueFloat64()"},
}

// wrapperSuffix returns ".GetValue()" for fields whose proto type is a
// google.protobuf.{Bool,String,Int32,Int64,Float,Double}Value — the generated
// proto getter returns *wrapperpb.XValue and we need the inner scalar.
func wrapperSuffix(pf *ProtoField) string {
	if pf != nil && pf.IsScalarWrapper {
		return ".GetValue()"
	}
	return ""
}

// renderScalarHeuristicFlatten emits the three-arm FlattenStmt used when a TF
// attribute is Optional-only but the proto field carries no presence: prefer
// the proto value when non-zero, fall back to prev (preserving user
// null-vs-zero intent), then null. Used for every scalar AttrType.
func renderScalarHeuristicFlatten(conv *FieldConversion, sh scalarShape) string {
	return fmt.Sprintf(
		"if v := proto.Get%s(); v != %s {\n\t\tm.%s = types.%sValue(v)\n\t} else if prev != nil && !prev.%s.IsUnknown() {\n\t\tm.%s = prev.%s\n\t} else {\n\t\tm.%s = types.%sNull()\n\t}",
		conv.ProtoGoName, sh.zeroSentinel,
		conv.GoName, sh.tfTypeWord,
		conv.GoName, conv.GoName, conv.GoName,
		conv.GoName, sh.tfTypeWord,
	)
}

// scalarListBindings returns the emission strings needed to call the generic
// modelconv.ListFromSliceWithDiags / ListToSliceWithDiags helpers for a
// repeated proto scalar of the given pf.Kind: the framework attr.Type literal
// (e.g. "types.StringType") and the Go element type (e.g. "string").
// Returns an error for any pf.Kind the helpers don't yet support.
func scalarListBindings(kind string) (elemTypeExpr, goType string, err error) {
	switch kind {
	case KindString:
		return "types.StringType", "string", nil
	case KindInt32:
		return "types.Int32Type", "int32", nil
	case KindInt64:
		return "types.Int64Type", "int64", nil
	case KindBool:
		return "types.BoolType", "bool", nil
	case KindDouble, KindFloat:
		return "types.Float64Type", "float64", nil
	default:
		return "", "", fmt.Errorf("list of %s not yet supported by flatten/expand generator", kind)
	}
}

// scalarExpandExpr returns the Expand RHS for a TF scalar attribute. Proto
// fields with presence (proto3 optional OR scalar wrapper) route through
// PointerOrNil so unset round-trips as nil; everything else takes the bare
// .ValueX() form.
func scalarExpandExpr(conv *FieldConversion, pf *ProtoField, sh scalarShape) string {
	if pf != nil && pf.IsOptional {
		return fmt.Sprintf("utils.PointerOrNil(m.%s, types.%s.Value%s)",
			conv.GoName, sh.tfTypeWord, sh.tfTypeWord)
	}
	return fmt.Sprintf("m.%s%s", conv.GoName, sh.expandValueFn)
}

// emitScalarFlattenExpand fills conv.FlattenExpr/FlattenStmt and conv.ExpandExpr
// for a TF scalar attribute. Three regimes:
//
//   - proto carries presence (proto3 optional keyword OR scalar wrapper) →
//     direct types.XValue expression; the template's three-arm emission
//     handles prev / null at the call site.
//   - TF Optional-only + proto without presence → heuristic FlattenStmt
//     using the type's zero value as a "no signal" sentinel, with prev
//     fallback for the null-vs-zero distinction.
//   - everything else (Required, Optional+Computed) → bare types.XValue.
func emitScalarFlattenExpand(conv *FieldConversion, pf *ProtoField, a *SchemaAttr, sh scalarShape) {
	conv.Kind = FieldKindScalar
	if conv.FlattenExpr == "" && conv.FlattenStmt == "" {
		if a.Optional && !a.Computed && !pf.IsOptional {
			conv.FlattenStmt = renderScalarHeuristicFlatten(conv, sh)
		} else {
			conv.FlattenExpr = fmt.Sprintf("types.%sValue(proto.Get%s()%s)",
				sh.tfTypeWord, conv.ProtoGoName, wrapperSuffix(pf))
		}
	}
	if conv.ExpandExpr == "" {
		conv.ExpandExpr = scalarExpandExpr(conv, pf, sh)
	}
}

// ProtoLookup resolves additional proto messages by name (e.g. flat-request
// types or per-RPC payload types) using whatever file set the caller has
// already compiled. Pass nil when no extra lookups are needed; planning will
// error if a path actually requires one.
type ProtoLookup func(messageName string) (*ProtoMessage, error)

// ApplyAPIDefaults materializes cfg.API and fills convention-based defaults
// from cfg.TFName for any field the yaml didn't declare:
//
//   - cfg.API.Create/Update/Delete: instantiated for every op in `supported`
//     (defaults [create, read, update, delete] for resources, [read] for
//     datasources, minus exclude_operations). Each gets
//     RPC = "<Verb><TFName>" and Request = "<Verb><TFName>Request".
//   - cfg.API.ResponseInterface: defaulted to {Name: "<TFName>Response"}.
//   - cfg.API.ResourceType: defaulted to cfg.TFName.
//
// Yaml-declared values always win. TFName is required when any CUD op is
// supported; datasources tolerate empty TFName when response_interface is
// fully declared.
//
// Exported so cmd/schemagen can run defaulting before emitting the
// proto-validator (which needs cfg.API.ResourceType). Idempotent;
// PlanFlattenExpand calls it again internally.
func ApplyAPIDefaults(cfg *Config, schemaType string) error {
	supported, err := cfg.SupportedOperations(schemaType)
	if err != nil {
		return err
	}
	return applyAPIDefaults(cfg, supported)
}

func applyAPIDefaults(cfg *Config, supported map[string]bool) error {
	if cfg.API == nil {
		cfg.API = &APIConfig{}
	}
	needsTFName := supported["create"] || supported["update"] || supported["delete"]
	if needsTFName && cfg.TFName == "" {
		return errors.New("schema.yaml: tf_name is required (canonical PascalCase Terraform resource identifier — drives RPC name defaults)")
	}
	type opSlot struct {
		verb string
		key  string
		ref  **RPCConfig
	}
	for _, slot := range []opSlot{
		{"Create", "create", &cfg.API.Create},
		{"Update", "update", &cfg.API.Update},
		{"Delete", "delete", &cfg.API.Delete},
	} {
		if !supported[slot.key] {
			continue
		}
		if *slot.ref == nil {
			*slot.ref = &RPCConfig{}
		}
		if (*slot.ref).RPC == "" {
			(*slot.ref).RPC = slot.verb + cfg.TFName
		}
		if (*slot.ref).Request == "" {
			(*slot.ref).Request = slot.verb + cfg.TFName + "Request"
		}
	}
	if cfg.API.ResponseInterface == nil && cfg.TFName != "" {
		cfg.API.ResponseInterface = &ResponseInterfaceConfig{
			Name: cfg.TFName + "Response",
		}
	} else if cfg.API.ResponseInterface != nil && cfg.TFName != "" {
		if cfg.API.ResponseInterface.Name == "" {
			cfg.API.ResponseInterface.Name = cfg.TFName + "Response"
		}
	}

	if cfg.API.ResourceType == "" && cfg.TFName != "" {
		cfg.API.ResourceType = cfg.TFName
	}

	return nil
}

func inferRPCPayload(rpc *RPCConfig, lookup ProtoLookup) error {
	if rpc == nil || rpc.Request == "" {
		return nil
	}
	if rpc.PayloadField != "" || rpc.PayloadType != "" {
		return nil
	}
	if lookup == nil {
		return nil
	}
	msg, err := lookup(rpc.Request)
	if err != nil {
		return fmt.Errorf("infer payload for rpc %s: looking up %s: %w", rpc.RPC, rpc.Request, err)
	}
	if msg == nil {
		return nil
	}
	var messageFields []*ProtoField
	for i := range msg.Fields {
		f := &msg.Fields[i]
		if f.Kind != KindMessage || f.Nested == nil {
			continue
		}
		if f.Nested.Name == "FieldMask" {
			continue
		}
		if f.OneofName != "" {
			continue
		}
		messageFields = append(messageFields, f)
	}
	if len(messageFields) != 1 {
		return nil
	}
	sole := messageFields[0]
	payloadType := sole.Nested.GoName
	if payloadType == "" {
		payloadType = sole.Nested.Name
	}
	if !payloadTypeMatchesRequestConvention(rpc.Request, payloadType) {
		return nil
	}
	rpc.PayloadField = sole.Name
	rpc.PayloadType = payloadType
	return nil
}

func payloadTypeMatchesRequestConvention(requestName, payloadType string) bool {
	if requestName == "" || payloadType == "" {
		return false
	}

	if strings.HasPrefix(payloadType, requestName+"_") {
		return true
	}

	base := strings.TrimSuffix(requestName, "Request")
	for _, verb := range []string{"Create", "Update", "Delete", "List", "Get"} {
		if strings.HasPrefix(base, verb) {
			base = strings.TrimPrefix(base, verb)
			break
		}
	}
	if base != "" && strings.HasPrefix(payloadType, base) {
		return true
	}
	return false
}

// PlanFlattenExpand builds the ConversionData for a resource by walking the
// merged schema attrs, config, and proto, applying the conversion-keys from
// schema.yaml.
//
// pkgName is the package the generated file lives in (e.g., "user").
// protoAlias is the import alias used for the proto package (e.g., "dataplanev1").
// protoImport is the full import path of the proto package.
// schemaType is "resource" or "datasource"; datasources skip Expander
// emission and only produce Flatten.
// lookup may be nil; when nil, multi-field flat requests and per-RPC
// payload-type lookups will fail with a clear error.
func PlanFlattenExpand(
	attrs []SchemaAttr,
	cfg *Config,
	proto *ProtoMessage,
	pkgName, protoAlias, protoImport, schemaType string,
	lookup ProtoLookup,
) (ConversionData, error) {
	supported, err := cfg.SupportedOperations(schemaType)
	if err != nil {
		return ConversionData{}, err
	}
	if err := applyAPIDefaults(cfg, supported); err != nil {
		return ConversionData{}, err
	}

	for _, rpc := range []*RPCConfig{cfg.API.Create, cfg.API.Update, cfg.API.Delete} {
		if err := inferRPCPayload(rpc, lookup); err != nil {
			return ConversionData{}, err
		}
	}

	modelType := ModelTypeResource
	flattenFunc := "Flatten"
	ifacePrefix := ""
	if schemaType == SchemaTypeDatasource {
		modelType = ModelTypeDatasource
		flattenFunc = "FlattenData"
		ifacePrefix = DatasourcePrefix
	}
	data := ConversionData{
		Package:       pkgName,
		ProtoImport:   protoImport,
		ProtoAlias:    protoAlias,
		ModelTypeName: modelType,
		FlattenFunc:   flattenFunc,

		HasTimeouts: len(cfg.Timeouts) > 0,
	}
	data.ExtraImports = append(data.ExtraImports, protoImport)

	protoByName := map[string]*ProtoField{}
	if proto != nil {
		for i := range proto.Fields {
			protoByName[proto.Fields[i].Name] = &proto.Fields[i]
		}
	}

	switch {
	case cfg.API.ResponseInterface != nil:
		ifaceName := ifacePrefix + cfg.API.ResponseInterface.Name
		data.ResponseInterface = &ResponseInterfaceData{
			Name: ifaceName,
		}
		data.FlattenArg = ifaceName
	case cfg.API.Create != nil && cfg.API.Create.PayloadType != "":

		data.FlattenArg = "*" + protoAlias + "." + cfg.API.Create.PayloadType
	default:
		return ConversionData{}, errors.New("api: block has no response_interface and no create.payload_type — cannot determine Flatten input type")
	}

	for i := range attrs {
		a := &attrs[i]
		fc := configForAttr(cfg, a)
		if fc != nil && fc.Exclude {
			continue
		}
		conv, err := planField(a, fc, protoByName, protoAlias, ifacePrefix, true)
		if err != nil {
			return ConversionData{}, fmt.Errorf("plan field %q: %w", a.Name, err)
		}
		if conv == nil {
			continue
		}
		data.RootFieldConversions = append(data.RootFieldConversions, *conv)
		if conv.Preserve {
			data.HasPreservedFields = true
		}
	}

	if data.ResponseInterface != nil {
		methods, err := inferResponseMethods(data.RootFieldConversions, protoByName, protoAlias)
		if err != nil {
			return ConversionData{}, err
		}
		extra, err := protoOnlyResponseMethods(cfg, protoByName, protoAlias, methods)
		if err != nil {
			return ConversionData{}, err
		}
		methods = append(methods, extra...)
		sort.Slice(methods, func(i, j int) bool { return methods[i].Signature < methods[j].Signature })
		data.ResponseInterface.Methods = methods
	}

	if err := planNestedPreserveBlocks(attrs, cfg, ifacePrefix, &data); err != nil {
		return ConversionData{}, err
	}

	nestedSeen := map[string]bool{}
	data.ExternalProtoImports = map[string]string{}
	if proto != nil {
		if err := planNestedTypes(attrs, proto.Fields, cfg.Fields, protoAlias, ifacePrefix, nestedSeen, &data.NestedFlatteners, data.ExternalProtoImports); err != nil {
			return ConversionData{}, err
		}
	}

	canonicalNestedType := map[string]string{}
	for i := range data.NestedFlatteners {
		nf := &data.NestedFlatteners[i]
		canonicalNestedType[nf.FuncSuffix] = nf.ProtoTypeBare
	}

	for _, op := range []struct {
		key  string
		name string
		rpc  *RPCConfig
	}{
		{"Create", "create", cfg.API.Create},
		{"Update", "update", cfg.API.Update},
		{"Delete", "delete", cfg.API.Delete},
	} {
		if !supported[op.name] {
			continue
		}
		if op.rpc == nil || op.rpc.Request == "" {
			continue
		}
		exp, err := planExpander(op.key, op.rpc, attrs, cfg, protoByName, protoAlias, ifacePrefix, lookup)
		if err != nil {
			return ConversionData{}, fmt.Errorf("plan %s expander: %w", op.key, err)
		}

		if err := planPerRPCNestedDivergence(op.key, &exp, op.rpc, attrs, cfg, protoAlias, ifacePrefix, canonicalNestedType, lookup, &data.NestedFlatteners); err != nil {
			return ConversionData{}, fmt.Errorf("plan %s per-RPC divergence: %w", op.key, err)
		}
		data.Expanders = append(data.Expanders, exp)
	}

	if usesModelConv(data) {
		data.ExtraImports = append(data.ExtraImports,
			"github.com/redpanda-data/terraform-provider-redpanda/internal/modelconv")
	}
	if dataReferencesPkg(data, "utils.") || emitsWithMask(data) {
		data.ExtraImports = append(data.ExtraImports,
			"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils")
	}
	if emitsWithMask(data) {
		data.ExtraImports = append(data.ExtraImports,
			"google.golang.org/protobuf/types/known/fieldmaskpb")
	}
	if dataReferencesPkg(data, "enums.") {
		data.ExtraImports = append(data.ExtraImports,
			"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils/enums")
	}

	annotateUsesCtx(&data)

	return data, nil
}

func annotateUsesCtx(data *ConversionData) {
	data.FlattenUsesCtx = conversionsUseCtx(data.RootFieldConversions, false) || len(data.NestedPreserveBlocks) > 0
	for i := range data.Expanders {
		data.Expanders[i].UsesCtx = conversionsUseCtx(data.Expanders[i].Conversions, true)
	}
	for i := range data.NestedFlatteners {
		nf := &data.NestedFlatteners[i]
		nf.FlattenUsesCtx = conversionsUseCtx(nf.Conversions, false)
		nf.ExpandUsesCtx = conversionsUseCtx(nf.Conversions, true)
	}
}

// conversionsUseCtx reports whether any conversion's generated body references
// ctx. expand selects the Expand statement/expression pair; otherwise the
// Flatten pair is checked.
func conversionsUseCtx(convs []FieldConversion, expand bool) bool {
	for i := range convs {
		stmt, expr := convs[i].FlattenStmt, convs[i].FlattenExpr
		if expand {
			stmt, expr = convs[i].ExpandStmt, convs[i].ExpandExpr
		}
		if strings.Contains(stmt, "ctx") || strings.Contains(expr, "ctx") {
			return true
		}
	}
	return false
}

func planNestedTypes(
	attrs []SchemaAttr,
	pfs []ProtoField,
	fcs map[string]FieldConfig,
	protoAlias string,
	prefix string,
	seen map[string]bool,
	out *[]NestedFlattener,
	externalImports map[string]string,
) error {
	pfByName := map[string]*ProtoField{}
	for i := range pfs {
		pfByName[pfs[i].Name] = &pfs[i]
	}
	for i := range attrs {
		a := &attrs[i]
		if !isNestedMessage(a.AttrType) {
			continue
		}
		pf, ok := pfByName[a.Name]
		if !ok || pf.Nested == nil {
			continue
		}

		funcSuffix := prefix + pathToPascal(a.Name)
		modelTypeName := funcSuffix + "Model"
		if seen[modelTypeName] {
			continue
		}
		seen[modelTypeName] = true

		var nestedFc map[string]FieldConfig
		if fc, ok := fcs[a.Name]; ok {
			nestedFc = fc.Fields
		}
		nestedProtoByName := map[string]*ProtoField{}
		for j := range pf.Nested.Fields {
			nestedProtoByName[pf.Nested.Fields[j].Name] = &pf.Nested.Fields[j]
		}

		var conversions []FieldConversion
		for j := range a.NestedAttrs {
			child := &a.NestedAttrs[j]
			var childFc *FieldConfig
			if cfg, ok := nestedFc[child.Name]; ok {
				childFc = &cfg
			}
			conv, err := planField(child, childFc, nestedProtoByName, protoAlias, funcSuffix, false)
			if err != nil {
				return fmt.Errorf("plan nested field %q.%q: %w", a.Name, child.Name, err)
			}
			if conv == nil {
				continue
			}

			if cp, ok := nestedProtoByName[child.Name]; ok && cp.OneofName != "" {
				conv.IsOneofVariant = true
			}
			conversions = append(conversions, *conv)
		}

		alias := protoAlias
		if pf.Nested.ExternalPkgAlias != "" {
			alias = pf.Nested.ExternalPkgAlias
			if externalImports != nil && pf.Nested.ExternalPkgImport != "" {
				externalImports[alias] = pf.Nested.ExternalPkgImport
			}
		}
		protoTypeBare := alias + "." + pf.Nested.GoName
		if pf.Nested.GoName == "" {
			protoTypeBare = alias + "." + pf.Nested.Name
		}
		*out = append(*out, NestedFlattener{
			TypeName:      modelTypeName,
			FuncSuffix:    funcSuffix,
			ProtoType:     "*" + protoTypeBare,
			ProtoTypeBare: protoTypeBare,
			Conversions:   conversions,
			EmitFlatten:   true,
			EmitExpand:    true,
		})

		if err := planNestedTypes(a.NestedAttrs, pf.Nested.Fields, nestedFc, protoAlias, funcSuffix, seen, out, externalImports); err != nil {
			return err
		}
	}
	return nil
}

func planField(a *SchemaAttr, fc *FieldConfig, protoByName map[string]*ProtoField, protoAlias, nestedPrefix string, isRoot bool) (*FieldConversion, error) {
	conv := &FieldConversion{
		GoName: toGoFieldName(a.Name),
		Tag:    a.Name,
	}
	if fc != nil {
		if fc.FlattenSkip && fc.FlattenVia != "" {
			return nil, fmt.Errorf("attribute %q: flatten_skip and flatten_via are mutually exclusive", a.Name)
		}
		if fc.FlattenFromPrev {
			if fc.FlattenSkip {
				return nil, fmt.Errorf("attribute %q: flatten_from_prev and flatten_skip are mutually exclusive (flatten_skip already preserves from prev)", a.Name)
			}
			if fc.FlattenVia != "" {
				return nil, fmt.Errorf("attribute %q: flatten_from_prev and flatten_via are mutually exclusive (custom flatten cannot be combined with prev override)", a.Name)
			}
			if fc.ComputedOnly {
				return nil, fmt.Errorf("attribute %q: flatten_from_prev requires user-writable field — computed_only fields have no user-supplied prev value to restore", a.Name)
			}
		}
	}
	nullExpr, err := NullExpr(a, NullExprOptions{HelperPrefix: nestedPrefix})
	if err != nil {
		return nil, fmt.Errorf("field %q: %w", a.Name, err)
	}
	flattenSkip := fc != nil && fc.FlattenSkip
	expandSkip := fc != nil && fc.ExpandSkip
	overrideFromPrev := fc != nil && fc.FlattenFromPrev
	preserveDefault := ""
	if fc != nil && fc.Default != nil {
		preserveDefault = contingentDefaultExpr(fc.Default, a.AttrType)
	}
	defer func() {
		if conv == nil {
			return
		}
		if flattenSkip {
			conv.FlattenExpr = ""
			conv.FlattenStmt = ""
			conv.Preserve = true
			if conv.NullExpr == "" {
				conv.NullExpr = nullExpr
			}
		}
		if expandSkip {
			conv.ExpandExpr = ""
		}
		if overrideFromPrev {
			conv.OverrideFromPrev = true
		}
		if preserveDefault != "" {
			conv.PreserveDefault = preserveDefault
		}
	}()

	if fc != nil {
		if fc.FromProto != "" {
			pf, ok := protoByName[fc.FromProto]
			if !ok {
				return nil, fmt.Errorf("from_proto %q does not match any proto field", fc.FromProto)
			}
			conv.ProtoField = pf.Name
			conv.ProtoGoName = toProtoGoName(pf.Name)
			conv.Kind = FieldKindCustom
			var flattenDefault, expandDefault string
			switch {
			case pf.Kind == protoKindJSONStruct:
				flattenDefault = fmt.Sprintf("modelconv.StringFromProtoStruct(proto.Get%s())", conv.ProtoGoName)
				expandDefault = fmt.Sprintf("modelconv.ProtoStructFromStringWithDiags(m.%s, &diags)", conv.GoName)
			case pf.Cardinality == KindMap && pf.MapValKind == KindString:
				flattenDefault = fmt.Sprintf("modelconv.MapFromStringsWithDiags(ctx, proto.Get%s(), &diags)", conv.ProtoGoName)
				expandDefault = fmt.Sprintf("modelconv.MapToStringsWithDiags(ctx, m.%s, &diags)", conv.GoName)
			case pf.Kind == protoKindEnum:
				conv.Kind = FieldKindEnum
				fnName := enumsFunctionName(pf)
				flattenDefault = fmt.Sprintf("types.StringValue(enums.%sToString(proto.Get%s()))", fnName, conv.ProtoGoName)
				expandDefault = fmt.Sprintf("enums.StringTo%s(m.%s.ValueString())", fnName, conv.GoName)
			default:
				flattenDefault = fmt.Sprintf("types.StringValue(proto.Get%s())", conv.ProtoGoName)
			}
			if fc.FlattenVia != "" {
				conv.FlattenExpr = fmt.Sprintf("%s(proto)", fc.FlattenVia)
			} else {
				conv.FlattenExpr = flattenDefault
			}
			if fc.ExpandVia != "" {
				conv.ExpandExpr = fmt.Sprintf("m.%s()", fc.ExpandVia)
			} else {
				conv.ExpandExpr = expandDefault
			}
			return conv, nil
		}
		if fc.FlattenVia != "" || fc.ExpandVia != "" {
			conv.Kind = FieldKindCustom
			if fc.FlattenVia != "" {
				conv.FlattenExpr = fmt.Sprintf("%s(proto)", fc.FlattenVia)
			}
			if fc.ExpandVia != "" {
				conv.ExpandExpr = fmt.Sprintf("m.%s()", fc.ExpandVia)
			}

			if pf, ok := protoByName[lookupProtoName(a)]; ok {
				conv.ProtoField = pf.Name
				conv.ProtoGoName = toProtoGoName(pf.Name)
			}
		}
		if fc.Extra && fc.FromProto == "" && fc.FlattenVia == "" && fc.ExpandVia == "" {
			conv.Preserve = true
			conv.NullExpr = nullExpr
			return conv, nil
		}
	}

	pf, ok := protoByName[lookupProtoName(a)]
	if !ok {
		if conv.FlattenExpr == "" && conv.ExpandExpr == "" {
			return nil, nil
		}
		return conv, nil
	}
	conv.ProtoField = pf.Name
	conv.ProtoGoName = toProtoGoName(pf.Name)
	conv.HasPresence = pf.IsOptional || (fc != nil && fc.HasPresence)

	if conv.HasPresence {
		conv.NullExpr = nullExpr
	}

	if pf.Kind == protoKindEnum {
		conv.Kind = FieldKindEnum
		_ = protoAlias
		fnName := enumsFunctionName(pf)
		if conv.FlattenExpr == "" {
			conv.FlattenExpr = fmt.Sprintf("types.StringValue(enums.%sToString(proto.Get%s()))",
				fnName, conv.ProtoGoName)
		}
		if conv.ExpandExpr == "" {
			conv.ExpandExpr = fmt.Sprintf("enums.StringTo%s(m.%s.ValueString())", fnName, conv.GoName)
		}
		return conv, nil
	}

	if a.AttrType == AttrTypeBool && pf.Kind == KindMessage && pf.OneofName != "" &&
		(pf.Nested == nil || len(pf.Nested.Fields) == 0) {
		conv.Kind = FieldKindCustom
		conv.IsOneofVariant = true

		variantAlias := protoAlias
		if pf.Nested != nil && pf.Nested.ExternalPkgAlias != "" {
			variantAlias = pf.Nested.ExternalPkgAlias
		}
		variantType := variantAlias + "." + pf.Nested.GoName
		if pf.Nested.GoName == "" {
			variantType = variantAlias + "." + pf.Nested.Name
		}
		if conv.FlattenExpr == "" {
			conv.FlattenExpr = fmt.Sprintf("modelconv.BoolFromOneofPresence(proto.Has%s())", conv.ProtoGoName)
		}
		if conv.ExpandStmt == "" {
			conv.ExpandStmt = fmt.Sprintf("if m.%s.ValueBool() {\n\t\tout.Set%s(&%s{})\n\t}",
				conv.GoName, conv.ProtoGoName, variantType)
		}
		return conv, nil
	}

	switch a.AttrType {
	case AttrTypeString:
		switch pf.Kind {
		case protoKindTimestamp:

			conv.Kind = FieldKindTimestamp
			if conv.FlattenExpr == "" {
				conv.FlattenExpr = fmt.Sprintf("modelconv.StringFromTimestamp(proto.Get%s())", conv.ProtoGoName)
			}
			if conv.ExpandExpr == "" {
				conv.ExpandExpr = fmt.Sprintf("modelconv.TimestampFromStringWithDiags(m.%s, &diags)", conv.GoName)
			}
		case protoKindDuration:

			conv.Kind = FieldKindDuration
			if conv.FlattenExpr == "" {
				conv.FlattenExpr = fmt.Sprintf("modelconv.StringFromDuration(proto.Get%s())", conv.ProtoGoName)
			}
			if conv.ExpandExpr == "" {
				conv.ExpandExpr = fmt.Sprintf("modelconv.DurationFromStringWithDiags(m.%s, &diags)", conv.GoName)
			}
		case protoKindJSONStruct:

			conv.Kind = FieldKindCustom
			if conv.FlattenExpr == "" {
				conv.FlattenExpr = fmt.Sprintf("modelconv.StringFromProtoStruct(proto.Get%s())", conv.ProtoGoName)
			}
			if conv.ExpandExpr == "" {
				conv.ExpandExpr = fmt.Sprintf("modelconv.ProtoStructFromStringWithDiags(m.%s, &diags)", conv.GoName)
			}
		default:
			emitScalarFlattenExpand(conv, pf, a, scalarShapes[AttrTypeString])
		}
	case AttrTypeBool, AttrTypeInt32, AttrTypeInt64, AttrTypeFloat64:
		emitScalarFlattenExpand(conv, pf, a, scalarShapes[a.AttrType])
	case AttrTypeNumber:

		conv.Kind = FieldKindScalar
		switch pf.Kind {
		case KindInt32:
			if conv.FlattenExpr == "" {
				conv.FlattenExpr = fmt.Sprintf("utils.Int32ToNumber(proto.Get%s())", conv.ProtoGoName)
			}
			if conv.ExpandExpr == "" {
				if pf.IsOptional {
					conv.ExpandExpr = fmt.Sprintf("utils.NumberToInt32OrNil(m.%s)", conv.GoName)
				} else {
					conv.ExpandExpr = fmt.Sprintf("*utils.NumberToInt32(m.%s)", conv.GoName)
				}
			}
		default:
			return nil, fmt.Errorf("attribute %q: NumberAttribute requires int32 proto field, got %s (int64 not yet supported)", a.Name, pf.Kind)
		}
	case AttrTypeList:

		if pf.Cardinality != KindRepeated {
			return nil, fmt.Errorf("attribute %q: AttrTypeList but proto field is not repeated", a.Name)
		}

		elemTypeExpr, goType, err := scalarListBindings(pf.Kind)
		if err != nil {
			return nil, fmt.Errorf("attribute %q: %w", a.Name, err)
		}
		conv.Kind = FieldKindListScalar
		if conv.FlattenExpr == "" && conv.FlattenStmt == "" {
			if a.Optional {
				// proto3 repeated erases empty-vs-absent on the wire: a planned
				// [] reads back as nil and would flatten to null, tripping
				// inconsistent-result. Carry a known-empty prev through.
				conv.FlattenStmt = fmt.Sprintf(
					"m.%s = modelconv.ListFromSliceWithDiags(ctx, proto.Get%s(), %s, &diags)\n\tif prev != nil {\n\t\tm.%s = modelconv.ListCarryKnownEmpty(m.%s, prev.%s)\n\t}",
					conv.GoName, conv.ProtoGoName, elemTypeExpr,
					conv.GoName, conv.GoName, conv.GoName,
				)
			} else {
				conv.FlattenExpr = fmt.Sprintf("modelconv.ListFromSliceWithDiags(ctx, proto.Get%s(), %s, &diags)", conv.ProtoGoName, elemTypeExpr)
			}
		}
		if conv.ExpandExpr == "" {
			conv.ExpandExpr = fmt.Sprintf("modelconv.ListToSliceWithDiags[%s](ctx, m.%s, &diags)", goType, conv.GoName)
		}
	case AttrTypeListNested:

		conv.Kind = FieldKindListMessage
		attrTypesFn := nestedPrefix + pathToPascal(a.Name) + "AttrTypes"
		flattenFn := "Flatten" + nestedPrefix + pathToPascal(a.Name)
		expandFn := "Expand" + nestedPrefix + pathToPascal(a.Name)
		if conv.FlattenExpr == "" {
			conv.FlattenExpr = fmt.Sprintf(
				"modelconv.ListFromObjectsWithDiags(ctx, proto.Get%s(), %s(), %s, &diags)",
				conv.ProtoGoName, attrTypesFn, flattenFn,
			)
		}
		if conv.ExpandExpr == "" {
			conv.ExpandExpr = fmt.Sprintf(
				"modelconv.ListToObjectsWithDiags(ctx, m.%s, %s, &diags)",
				conv.GoName, expandFn,
			)
		}
	case AttrTypeSingleNested:

		conv.Kind = FieldKindSingleNested
		fieldPascal := pathToPascal(a.Name)
		attrTypesFn := nestedPrefix + fieldPascal + "AttrTypes"
		flattenFn := "Flatten" + nestedPrefix + fieldPascal
		expandFn := "Expand" + nestedPrefix + fieldPascal
		if conv.FlattenExpr == "" {
			// Thread prev through the nested Flatten so its leaf-level
			// preservation logic has a source of truth for proto3 null-vs-empty
			// ambiguity.
			//
			// At the root level the parent type is the resource's root model
			// (*ResourceModel / *DataModel) and we use the AsX method on it.
			// At deeper levels we use the Decode<Path> free function emitted
			// by collectSubConverters — it is already nil-safe and handles
			// the IsNull / IsUnknown checks.
			var prevExpr string
			if isRoot {
				// nestedPrefix is the schema-type prefix ("" for resource,
				// "Data" for datasource); the nested model type is e.g.
				// "AWSPrivateLinkModel" / "DataAWSPrivateLinkModel". The AsX
				// method on the root model is named without the schema prefix.
				prevExpr = fmt.Sprintf(
					"func() *%s%sModel { v, _ := prev.As%s(ctx); return v }()",
					nestedPrefix, fieldPascal, fieldPascal,
				)
			} else {
				decodeFn := "Decode" + nestedPrefix + fieldPascal
				nestedModelType := nestedPrefix + fieldPascal + "Model"
				prevExpr = fmt.Sprintf(
					"func() *%s { v, _ := %s(ctx, prev); return v }()",
					nestedModelType, decodeFn,
				)
			}
			conv.FlattenExpr = fmt.Sprintf(
				"modelconv.ObjectFromMessageWithDiagsAndPrev(ctx, proto.Get%s(), %s, %s(), %s, &diags)",
				conv.ProtoGoName, prevExpr, attrTypesFn, flattenFn,
			)
		}
		if conv.ExpandExpr == "" {
			conv.ExpandExpr = fmt.Sprintf(
				"modelconv.ObjectToMessageWithDiags(ctx, m.%s, %s, &diags)",
				conv.GoName, expandFn,
			)
		}
	case AttrTypeMap:

		if pf.Cardinality != KindMap {
			return nil, fmt.Errorf("attribute %q: AttrTypeMap but proto field is not map", a.Name)
		}
		if pf.MapValKind == KindMessage {
			return nil, fmt.Errorf("attribute %q: map of message not yet supported by flatten/expand generator", a.Name)
		}

		switch pf.MapValKind {
		case KindString:
			conv.Kind = FieldKindMapScalar
			if conv.FlattenExpr == "" {
				conv.FlattenExpr = fmt.Sprintf("modelconv.MapFromStringsWithDiags(ctx, proto.Get%s(), &diags)", conv.ProtoGoName)
			}
			if conv.ExpandExpr == "" {
				conv.ExpandExpr = fmt.Sprintf("modelconv.MapToStringsWithDiags(ctx, m.%s, &diags)", conv.GoName)
			}
		default:
			return nil, fmt.Errorf("attribute %q: map<string, %s> not yet supported by flatten/expand generator", a.Name, pf.MapValKind)
		}
	case AttrTypeMapNested, AttrTypeSetNested:
		return nil, fmt.Errorf("attribute %q: kind %s not yet supported by flatten/expand generator",
			a.Name, a.AttrType)
	default:
		conv.Kind = FieldKindScalar
		if conv.FlattenExpr == "" {
			conv.FlattenExpr = fmt.Sprintf("types.StringValue(proto.Get%s())", conv.ProtoGoName)
		}
		if conv.ExpandExpr == "" {
			conv.ExpandExpr = fmt.Sprintf("m.%s.ValueString()", conv.GoName)
		}
	}

	return conv, nil
}

func planFlatField(a *SchemaAttr, fc *FieldConfig, pf *ProtoField, protoAlias, nestedPrefix string) (*FieldConversion, error) {
	conv := &FieldConversion{
		GoName:      toGoFieldName(a.Name),
		Tag:         a.Name,
		ProtoField:  pf.Name,
		ProtoGoName: toProtoGoName(pf.Name),
	}
	if fc != nil && fc.ExpandSkip {
		return nil, nil
	}
	if fc != nil && fc.ExpandVia != "" {
		conv.ExpandExpr = fmt.Sprintf("m.%s()", fc.ExpandVia)
		return conv, nil
	}
	if pf != nil && pf.Kind == protoKindEnum {
		conv.ExpandExpr = fmt.Sprintf("enums.StringTo%s(m.%s.ValueString())", enumsFunctionName(pf), conv.GoName)
		return conv, nil
	}
	switch a.AttrType {
	case AttrTypeString, AttrTypeBool, AttrTypeInt32, AttrTypeInt64, AttrTypeFloat64:
		conv.ExpandExpr = scalarExpandExpr(conv, pf, scalarShapes[a.AttrType])
	case AttrTypeNumber:

		if pf.IsOptional {
			conv.ExpandExpr = fmt.Sprintf("utils.NumberToInt32OrNil(m.%s)", conv.GoName)
		} else {
			conv.ExpandExpr = fmt.Sprintf("*utils.NumberToInt32(m.%s)", conv.GoName)
		}
	case AttrTypeList:

		_, goType, err := scalarListBindings(pf.Kind)
		if err != nil {
			return nil, fmt.Errorf("flat field %q: %w", a.Name, err)
		}
		conv.ExpandExpr = fmt.Sprintf("modelconv.ListToSliceWithDiags[%s](ctx, m.%s, &diags)", goType, conv.GoName)
	case AttrTypeMap:
		if pf.Cardinality != KindMap {
			return nil, fmt.Errorf("flat field %q: AttrTypeMap but proto field is not map", a.Name)
		}
		if pf.MapValKind == KindMessage {
			return nil, fmt.Errorf("flat field %q: map of message not yet supported", a.Name)
		}
		switch pf.MapValKind {
		case KindString:
			conv.ExpandExpr = fmt.Sprintf("modelconv.MapToStringsWithDiags(ctx, m.%s, &diags)", conv.GoName)
		default:
			return nil, fmt.Errorf("flat field %q: map<string, %s> not yet supported", a.Name, pf.MapValKind)
		}
	case AttrTypeSingleNested:

		expandFn := "Expand" + nestedPrefix + pathToPascal(a.Name)
		conv.ExpandExpr = fmt.Sprintf("modelconv.ObjectToMessageWithDiags(ctx, m.%s, %s, &diags)", conv.GoName, expandFn)
	case AttrTypeListNested:

		expandFn := "Expand" + nestedPrefix + pathToPascal(a.Name)
		conv.ExpandExpr = fmt.Sprintf("modelconv.ListToObjectsWithDiags(ctx, m.%s, %s, &diags)", conv.GoName, expandFn)
	default:
		return nil, fmt.Errorf("flat field %q: AttrType %s not yet supported in flat requests", a.Name, a.AttrType)
	}
	_ = protoAlias
	return conv, nil
}

func planExpander(kind string, rpc *RPCConfig, attrs []SchemaAttr, cfg *Config, protoByName map[string]*ProtoField, protoAlias, nestedPrefix string, lookup ProtoLookup) (Expander, error) {
	exp := Expander{
		FuncName:      "Expand" + kind,
		RequestType:   "*" + protoAlias + "." + rpc.Request,
		ReturnPayload: rpc.ReturnPayload,
	}

	if rpc.ReturnPayload && rpc.PayloadType != "" {
		exp.RequestType = "*" + protoAlias + "." + rpc.PayloadType
	}

	if rpc.DiffMask != "" {
		if kind != "Update" {
			return Expander{}, fmt.Errorf("diff_mask is only valid on the update RPC, not %s", kind)
		}
		switch rpc.DiffMask {
		case "sparse":
			exp.MaskHelper = "GenerateProtobufDiffAndUpdateMask"
		case "full":
			exp.MaskHelper = "PlanPayloadWithUpdateMask"
		default:
			return Expander{}, fmt.Errorf("diff_mask must be \"sparse\" or \"full\", got %q", rpc.DiffMask)
		}
		exp.EmitWithMask = true
	}
	if rpc.PayloadField == "" {
		if lookup == nil {
			return Expander{}, fmt.Errorf("rpc %s is a flat request — caller must supply a ProtoLookup to resolve %s", rpc.RPC, rpc.Request)
		}
		reqMsg, err := lookup(rpc.Request)
		if err != nil {
			return Expander{}, fmt.Errorf("look up flat request type %s: %w", rpc.Request, err)
		}
		flatProtoByName := map[string]*ProtoField{}
		for i := range reqMsg.Fields {
			flatProtoByName[reqMsg.Fields[i].Name] = &reqMsg.Fields[i]
		}

		attrByTag := map[string]*SchemaAttr{}
		for i := range attrs {
			attrByTag[lookupProtoName(&attrs[i])] = &attrs[i]
		}
		exp.IsFlat = true
		for i := range reqMsg.Fields {
			pf := &reqMsg.Fields[i]
			tag := pf.Name
			if mapped, ok := rpc.FlatFieldMap[pf.Name]; ok {
				tag = mapped
			}
			a, ok := attrByTag[tag]
			if !ok {
				return Expander{}, fmt.Errorf("flat request %s: proto field %q has no matching TF attr (looking for tag %q; declare flat_field_map if the names differ)", rpc.Request, pf.Name, tag)
			}

			conv, err := planFlatField(a, configForAttr(cfg, a), pf, protoAlias, nestedPrefix)
			if err != nil {
				return Expander{}, fmt.Errorf("plan flat field %q: %w", a.Name, err)
			}
			if conv == nil || conv.ExpandExpr == "" {
				continue
			}

			if pf.OneofName != "" {
				conv.IsOneofVariant = true
			}
			exp.Conversions = append(exp.Conversions, *conv)
		}
		return exp, nil
	}
	exp.PayloadField = toProtoGoName(rpc.PayloadField)
	exp.PayloadType = "*" + protoAlias + "." + rpc.PayloadType

	payloadProtoByName := protoByName
	if lookup != nil && rpc.PayloadType != "" {
		if msg, err := lookup(rpc.PayloadType); err == nil && msg != nil {
			payloadProtoByName = make(map[string]*ProtoField, len(msg.Fields))
			for i := range msg.Fields {
				payloadProtoByName[msg.Fields[i].Name] = &msg.Fields[i]
			}
		}
	}

	for i := range attrs {
		a := &attrs[i]
		fc := configForAttr(cfg, a)
		if fc != nil && fc.Exclude {
			continue
		}
		if fc != nil && fc.Extra && fc.ExpandVia == "" && fc.FromProto == "" {
			continue
		}

		protoName := a.ProtoName
		if protoName == "" {
			protoName = a.Name
		}
		if fc != nil && fc.FromProto != "" {
			protoName = fc.FromProto
		}
		pf, ok := payloadProtoByName[protoName]
		if !ok {
			continue
		}
		// Expander context: only ExpandExpr is rendered, so isRoot=false (no prev in scope).
		conv, err := planField(a, fc, payloadProtoByName, protoAlias, nestedPrefix, false)
		if err != nil {
			return Expander{}, err
		}
		if conv == nil || conv.ProtoField == "" || conv.ExpandExpr == "" {
			continue
		}

		if pf.OneofName != "" {
			conv.IsOneofVariant = true
		}
		exp.Conversions = append(exp.Conversions, *conv)
	}
	return exp, nil
}

func planPerRPCNestedDivergence(
	kind string,
	exp *Expander,
	rpc *RPCConfig,
	attrs []SchemaAttr,
	cfg *Config,
	protoAlias string,
	nestedPrefix string,
	canonicalNestedType map[string]string,
	lookup ProtoLookup,
	out *[]NestedFlattener,
) error {
	if lookup == nil {
		return nil
	}

	probeType := rpc.PayloadType
	if exp.IsFlat || rpc.PayloadField == "" {
		probeType = rpc.Request
	}
	if probeType == "" {
		return nil
	}

	payloadFieldByName := map[string]*ProtoField{}
	if msg, err := lookup(probeType); err == nil && msg != nil {
		for i := range msg.Fields {
			payloadFieldByName[msg.Fields[i].Name] = &msg.Fields[i]
		}
	}
	if len(payloadFieldByName) == 0 {
		return nil
	}
	attrByName := map[string]*SchemaAttr{}
	for i := range attrs {
		attrByName[attrs[i].Name] = &attrs[i]
	}

	seen := map[string]bool{}
	for ci := range exp.Conversions {
		conv := &exp.Conversions[ci]
		if conv.ProtoField == "" {
			continue
		}
		a, ok := attrByName[conv.Tag]
		if !ok {
			continue
		}
		if a.AttrType != AttrTypeSingleNested && a.AttrType != AttrTypeListNested {
			continue
		}
		pf := payloadFieldByName[conv.ProtoField]
		if pf == nil || pf.Nested == nil {
			continue
		}
		funcSuffix := nestedPrefix + pathToPascal(a.Name)
		canonicalProtoType := canonicalNestedType[funcSuffix]

		alias := protoAlias
		if pf.Nested.ExternalPkgAlias != "" {
			alias = pf.Nested.ExternalPkgAlias
		}
		payloadProtoType := alias + "." + pf.Nested.GoName
		if pf.Nested.GoName == "" {
			payloadProtoType = alias + "." + pf.Nested.Name
		}
		if canonicalProtoType == "" || canonicalProtoType == payloadProtoType {
			continue
		}

		perRPCSuffix := kind + funcSuffix
		conv.ExpandFuncSuffix = perRPCSuffix

		conv.ExpandExpr = strings.ReplaceAll(conv.ExpandExpr, "Expand"+funcSuffix, "Expand"+perRPCSuffix)

		if seen[perRPCSuffix] {
			continue
		}
		seen[perRPCSuffix] = true

		nestedConvs, err := planPerRPCNestedConversions(a, pf, protoAlias, perRPCSuffix, cfg, canonicalNestedType, seen, kind, out)
		if err != nil {
			return fmt.Errorf("plan per-RPC conversions for %q (RPC %s): %w", a.Name, kind, err)
		}
		modelTypeName := funcSuffix + "Model"
		protoTypeBare := payloadProtoType
		*out = append(*out, NestedFlattener{
			TypeName:      modelTypeName,
			FuncSuffix:    perRPCSuffix,
			ProtoType:     "*" + protoTypeBare,
			ProtoTypeBare: protoTypeBare,
			Conversions:   nestedConvs,
			EmitFlatten:   false,
			EmitExpand:    true,
		})
	}
	return nil
}

func planPerRPCNestedConversions(
	a *SchemaAttr,
	pf *ProtoField,
	protoAlias string,
	perRPCSuffix string,
	cfg *Config,
	canonicalNestedType map[string]string,
	seen map[string]bool,
	kind string,
	out *[]NestedFlattener,
) ([]FieldConversion, error) {
	nestedProtoByName := map[string]*ProtoField{}
	for i := range pf.Nested.Fields {
		nestedProtoByName[pf.Nested.Fields[i].Name] = &pf.Nested.Fields[i]
	}
	var nestedFc map[string]FieldConfig
	if fc := configForField(cfg, a.Name); fc != nil {
		nestedFc = fc.Fields
	}
	var conversions []FieldConversion
	for j := range a.NestedAttrs {
		child := &a.NestedAttrs[j]
		var childFc *FieldConfig
		if fc, ok := nestedFc[child.Name]; ok {
			childFc = &fc
		}
		conv, err := planField(child, childFc, nestedProtoByName, protoAlias, perRPCSuffix, false)
		if err != nil {
			return nil, fmt.Errorf("plan nested field %q.%q: %w", a.Name, child.Name, err)
		}
		if conv == nil {
			continue
		}
		if cp, ok := nestedProtoByName[child.Name]; ok && cp.OneofName != "" {
			conv.IsOneofVariant = true
		}
		conversions = append(conversions, *conv)

		if child.AttrType != AttrTypeSingleNested && child.AttrType != AttrTypeListNested {
			continue
		}
		childPF := nestedProtoByName[child.Name]
		if childPF == nil || childPF.Nested == nil {
			continue
		}
		childSuffix := perRPCSuffix + pathToPascal(child.Name)
		if seen[childSuffix] {
			continue
		}
		seen[childSuffix] = true

		childAlias := protoAlias
		if childPF.Nested.ExternalPkgAlias != "" {
			childAlias = childPF.Nested.ExternalPkgAlias
		}
		childProtoTypeBare := childAlias + "." + childPF.Nested.GoName
		if childPF.Nested.GoName == "" {
			childProtoTypeBare = childAlias + "." + childPF.Nested.Name
		}
		childConvs, err := planPerRPCNestedConversions(child, childPF, protoAlias, childSuffix, cfg, canonicalNestedType, seen, kind, out)
		if err != nil {
			return nil, fmt.Errorf("plan per-RPC child %q.%q: %w", a.Name, child.Name, err)
		}

		modelSuffix := strings.TrimPrefix(childSuffix, kind)
		*out = append(*out, NestedFlattener{
			TypeName:      modelSuffix + "Model",
			FuncSuffix:    childSuffix,
			ProtoType:     "*" + childProtoTypeBare,
			ProtoTypeBare: childProtoTypeBare,
			Conversions:   childConvs,
			EmitFlatten:   false,
			EmitExpand:    true,
		})
	}
	return conversions, nil
}

// protoOnlyResponseMethods returns ResponseMethod entries for proto fields
// marked `proto_only: true` in the config — these fields emit no TF attribute
// but their proto getter must remain on the synthesized response interface so
// flatten_via helpers can call them.
func protoOnlyResponseMethods(cfg *Config, protoByName map[string]*ProtoField, protoAlias string, existing []ResponseMethod) ([]ResponseMethod, error) {
	seen := make(map[string]bool, len(existing))
	for _, m := range existing {
		seen[m.Signature] = true
	}
	var out []ResponseMethod
	for fieldName := range cfg.Fields {
		if !cfg.Fields[fieldName].ProtoOnly {
			continue
		}
		pf, ok := protoByName[fieldName]
		if !ok {
			return nil, fmt.Errorf("proto_only %q: proto has no field named %q", fieldName, fieldName)
		}
		ret, err := protoFieldReturnType(pf, protoAlias)
		if err != nil {
			return nil, fmt.Errorf("proto_only %q: %w", fieldName, err)
		}
		if pf.Kind == protoKindEnum {
			ret = protoAlias + "." + enumGoTypeName(pf)
		}
		if ret == "" {
			continue
		}
		sig := fmt.Sprintf("Get%s() %s", toProtoGoName(pf.Name), ret)
		if seen[sig] {
			continue
		}
		seen[sig] = true
		out = append(out, ResponseMethod{Signature: sig})
	}
	return out, nil
}

func inferResponseMethods(convs []FieldConversion, protoByName map[string]*ProtoField, protoAlias string) ([]ResponseMethod, error) {
	seen := map[string]bool{}
	var out []ResponseMethod
	for i := range convs {
		c := &convs[i]
		if c.ProtoField == "" || c.ProtoGoName == "" {
			continue
		}
		if c.FlattenExpr == "" && c.FlattenStmt == "" {
			continue
		}
		pf, ok := protoByName[c.ProtoField]
		if !ok {
			continue
		}
		ret, err := protoFieldReturnType(pf, protoAlias)
		if err != nil {
			return nil, fmt.Errorf("response method for %q: %w", c.ProtoField, err)
		}
		if pf.Kind == protoKindEnum {
			ret = protoAlias + "." + enumGoTypeName(pf)
		}
		if ret == "" {
			continue
		}
		sig := fmt.Sprintf("Get%s() %s", c.ProtoGoName, ret)
		if !seen[sig] {
			seen[sig] = true
			out = append(out, ResponseMethod{Signature: sig})
		}
		if c.HasPresence {
			hsig := fmt.Sprintf("Has%s() bool", c.ProtoGoName)
			if !seen[hsig] {
				seen[hsig] = true
				out = append(out, ResponseMethod{Signature: hsig})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Signature < out[j].Signature })
	return out, nil
}

// protoFieldReturnType returns the Go return-type string for a generated
// accessor on `pf`. It returns "" (with nil error) only for protoKindEnum,
// where callers must override the result with the resolved enum Go type
// (see inferResponseMethods / protoOnlyResponseMethods). Every other
// unrecognised or malformed Kind returns a non-nil error so codegen fails
// loudly instead of silently dropping the field's accessor.
func protoFieldReturnType(pf *ProtoField, protoAlias string) (string, error) {
	switch pf.Cardinality {
	case KindMap:
		if pf.MapValKind == KindMessage && pf.Nested != nil {
			return "map[string]*" + protoAlias + "." + protoMessageGoName(pf.Nested), nil
		}
		sg, err := scalarProtoGoType(pf.MapValKind)
		if err != nil {
			return "", err
		}
		return "map[string]" + sg, nil
	case KindRepeated:
		if pf.Kind == KindMessage && pf.Nested != nil {
			return "[]*" + protoAlias + "." + protoMessageGoName(pf.Nested), nil
		}
		sg, err := scalarProtoGoType(pf.Kind)
		if err != nil {
			return "", err
		}
		return "[]" + sg, nil
	default:
	}
	switch pf.Kind {
	case KindString:
		return KindString, nil
	case KindBool:
		return KindBool, nil
	case KindInt32:
		return KindInt32, nil
	case KindInt64:
		return KindInt64, nil
	case KindFloat:
		return "float32", nil
	case KindDouble:
		return goTypeFloat64, nil
	case protoKindBytes:
		return "[]byte", nil
	case protoKindTimestamp:
		return "*timestamppb.Timestamp", nil
	case protoKindDuration:
		return "*durationpb.Duration", nil
	case protoKindJSONStruct:
		return "*structpb.Struct", nil
	case KindMessage, protoKindStatus:
		if pf.Nested == nil {
			return "", fmt.Errorf("schemagen: %s field %q has no nested message info", pf.Kind, pf.Name)
		}
		alias := protoAlias
		if pf.Nested.ExternalPkgAlias != "" {
			alias = pf.Nested.ExternalPkgAlias
		}
		return "*" + alias + "." + protoMessageGoName(pf.Nested), nil
	case protoKindEnum:
		return "", nil
	default:
		return "", fmt.Errorf("schemagen: unrecognised proto Kind %q for field %q", pf.Kind, pf.Name)
	}
}

func protoMessageGoName(m *ProtoMessage) string {
	if m.GoName != "" {
		return m.GoName
	}
	return m.Name
}

func scalarProtoGoType(kind string) (string, error) {
	switch kind {
	case KindString:
		return KindString, nil
	case KindBool:
		return KindBool, nil
	case KindInt32:
		return KindInt32, nil
	case KindInt64:
		return KindInt64, nil
	case KindFloat:
		return "float32", nil
	case KindDouble:
		return goTypeFloat64, nil
	case protoKindBytes:
		return "[]byte", nil
	default:
		return "", fmt.Errorf("schemagen: unsupported scalar proto Kind %q", kind)
	}
}

func configForField(cfg *Config, name string) *FieldConfig {
	if cfg == nil || cfg.Fields == nil {
		return nil
	}
	if fc, ok := cfg.Fields[name]; ok {
		return &fc
	}
	return nil
}

func configForAttr(cfg *Config, a *SchemaAttr) *FieldConfig {
	if fc := configForField(cfg, a.Name); fc != nil {
		return fc
	}
	if a.ProtoName != "" && a.ProtoName != a.Name {
		return configForField(cfg, a.ProtoName)
	}
	return nil
}

func lookupProtoName(a *SchemaAttr) string {
	if a.ProtoName != "" {
		return a.ProtoName
	}
	return a.Name
}

func toProtoGoName(name string) string {
	parts := strings.Split(name, "_")
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		runes := []rune(p)
		for i := range runes {
			switch {
			case i == 0:
				runes[i] = upperRune(runes[i])
			case runes[i-1] >= '0' && runes[i-1] <= '9':
				runes[i] = upperRune(runes[i])
			default:
			}
		}
		b.WriteString(string(runes))
	}
	return b.String()
}

func enumGoTypeName(pf *ProtoField) string {
	if pf.EnumGoName != "" {
		return pf.EnumGoName
	}
	if i := strings.LastIndex(pf.EnumName, "."); i >= 0 {
		return pf.EnumName[i+1:]
	}
	return pf.EnumName
}

func enumPackageFunctionPrefix(protoPkg string) string {
	if protoPkg == "redpanda.core.common.v1" {
		return "Common"
	}
	return ""
}

var enumUseUnderscoredName = map[string]bool{
	"redpanda.api.controlplane.v1.ShadowLink_State": true,
}

func enumsFunctionName(pf *ProtoField) string {
	prefix := enumPackageFunctionPrefix(pf.EnumProtoPkg)
	name := enumGoTypeName(pf)
	key := pf.EnumProtoPkg + "." + name
	if enumUseUnderscoredName[key] {
		return prefix + name
	}
	return prefix + strings.ReplaceAll(name, "_", "")
}

func upperRune(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - ('a' - 'A')
	}
	return r
}

func contingentDefaultExpr(val any, attrType string) string {
	switch v := val.(type) {
	case bool:
		return fmt.Sprintf("types.BoolValue(%v)", v)
	case string:
		return fmt.Sprintf("types.StringValue(%q)", v)
	case int:
		switch attrType {
		case AttrTypeInt32:
			return fmt.Sprintf("types.Int32Value(%d)", v)
		default:
			return fmt.Sprintf("types.Int64Value(%d)", v)
		}
	case float64:
		return fmt.Sprintf("types.Float64Value(%v)", v)
	}
	return ""
}

func needsModelConv(k FieldKind) bool {
	return k == FieldKindListScalar || k == FieldKindListMessage ||
		k == FieldKindMapScalar || k == FieldKindMapMessage ||
		k == FieldKindSingleNested ||
		k == FieldKindTimestamp || k == FieldKindDuration
}

func usesModelConv(data ConversionData) bool {
	for i := range data.RootFieldConversions {
		if needsModelConv(data.RootFieldConversions[i].Kind) {
			return true
		}
	}
	for i := range data.Expanders {
		convs := data.Expanders[i].Conversions
		for j := range convs {
			if needsModelConv(convs[j].Kind) {
				return true
			}
		}
	}
	return false
}

func emitsWithMask(data ConversionData) bool {
	for i := range data.Expanders {
		if data.Expanders[i].EmitWithMask {
			return true
		}
	}
	return false
}

// dataReferencesPkg reports whether any generated conversion body references
// the package selector sel (e.g. "utils." or "enums."). Flatten bodies are
// checked on root conversions and emit-flatten nested conversions; expand
// bodies on expanders and emit-expand nested conversions.
func dataReferencesPkg(data ConversionData, sel string) bool {
	for i := range data.RootFieldConversions {
		c := &data.RootFieldConversions[i]
		if strings.Contains(c.FlattenExpr, sel) || strings.Contains(c.FlattenStmt, sel) {
			return true
		}
	}
	for i := range data.Expanders {
		convs := data.Expanders[i].Conversions
		for j := range convs {
			if strings.Contains(convs[j].ExpandExpr, sel) || strings.Contains(convs[j].ExpandStmt, sel) {
				return true
			}
		}
	}
	for i := range data.NestedFlatteners {
		nf := &data.NestedFlatteners[i]
		for j := range nf.Conversions {
			c := &nf.Conversions[j]
			if nf.EmitFlatten && (strings.Contains(c.FlattenExpr, sel) || strings.Contains(c.FlattenStmt, sel)) {
				return true
			}
			if nf.EmitExpand && (strings.Contains(c.ExpandExpr, sel) || strings.Contains(c.ExpandStmt, sel)) {
				return true
			}
		}
	}
	return false
}

func planNestedPreserveBlocks(attrs []SchemaAttr, cfg *Config, nestedPrefix string, data *ConversionData) error {
	if cfg == nil {
		return nil
	}
	for i := range attrs {
		a := &attrs[i]
		if a.AttrType != AttrTypeSingleNested {
			continue
		}
		containerFC := configForAttr(cfg, a)
		if containerFC == nil || len(containerFC.Fields) == 0 {
			continue
		}
		var children []NestedPreserveChild
		var subBlocks []NestedPreserveSubBlock
		for j := range a.NestedAttrs {
			inner := &a.NestedAttrs[j]
			innerFC, ok := containerFC.Fields[inner.Name]
			if !ok {
				continue
			}
			if nestedPreserveEligible(&innerFC) {
				children = append(children, NestedPreserveChild{
					GoName: pathToPascal(inner.Name),
				})
				continue
			}
			// Recurse one level: if inner is a SingleNested container, look
			// for eligible leaves inside it.
			if inner.AttrType != AttrTypeSingleNested || len(innerFC.Fields) == 0 {
				continue
			}
			var subChildren []NestedPreserveChild
			var subSubBlocks []NestedPreserveSubSubBlock
			for k := range inner.NestedAttrs {
				leaf := &inner.NestedAttrs[k]
				leafFC, ok2 := innerFC.Fields[leaf.Name]
				if !ok2 {
					continue
				}
				if nestedPreserveEligible(&leafFC) {
					subChildren = append(subChildren, NestedPreserveChild{
						GoName: pathToPascal(leaf.Name),
					})
					continue
				}
				// Recurse a second level: SubSubBlock
				if leaf.AttrType != AttrTypeSingleNested || len(leafFC.Fields) == 0 {
					continue
				}
				var subSubChildren []NestedPreserveChild
				for l := range leaf.NestedAttrs {
					grandLeaf := &leaf.NestedAttrs[l]
					grandLeafFC, ok3 := leafFC.Fields[grandLeaf.Name]
					if !ok3 || !nestedPreserveEligible(&grandLeafFC) {
						continue
					}
					subSubChildren = append(subSubChildren, NestedPreserveChild{
						GoName: pathToPascal(grandLeaf.Name),
					})
				}
				if len(subSubChildren) == 0 {
					continue
				}
				subSubPascal := pathToPascal(leaf.Name)
				containerPascal2 := pathToPascal(a.Name)
				subPascal2 := pathToPascal(inner.Name)
				subSubBlocks = append(subSubBlocks, NestedPreserveSubSubBlock{
					SubSubContainerGoName: subSubPascal,
					DecodeFunc:            "Decode" + containerPascal2 + subPascal2 + subSubPascal,
					ToObjectFunc:          nestedPrefix + containerPascal2 + subPascal2 + subSubPascal + "ToObject",
					Children:              subSubChildren,
				})
			}
			if len(subChildren) == 0 && len(subSubBlocks) == 0 {
				continue
			}
			subPascal := pathToPascal(inner.Name)
			containerPascalInner := pathToPascal(a.Name)
			subBlocks = append(subBlocks, NestedPreserveSubBlock{
				SubContainerGoName: subPascal,
				DecodeFunc:         "Decode" + containerPascalInner + subPascal,
				ToObjectFunc:       nestedPrefix + containerPascalInner + subPascal + "ToObject",
				Children:           subChildren,
				SubSubBlocks:       subSubBlocks,
			})
		}
		if len(children) == 0 && len(subBlocks) == 0 {
			continue
		}
		containerPascal := pathToPascal(a.Name)
		data.NestedPreserveBlocks = append(data.NestedPreserveBlocks, NestedPreserveBlock{
			ContainerGoName: containerPascal,
			AsFunc:          "As" + containerPascal,
			ToObjectFunc:    nestedPrefix + containerPascal + "ToObject",
			Children:        children,
			SubBlocks:       subBlocks,
		})
	}
	return nil
}

func nestedPreserveEligible(fc *FieldConfig) bool {
	if fc == nil {
		return false
	}
	if fc.WriteOnly || fc.Sensitive {
		return true
	}
	if fc.Extra {
		return true
	}
	return false
}
