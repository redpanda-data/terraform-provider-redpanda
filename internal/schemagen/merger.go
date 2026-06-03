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
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/redpanda-data/terraform-provider-redpanda/internal/apidesc"
)

// Merge combines proto fields with YAML config to produce a final list of
// SchemaAttrs ready for code generation. Returns the attrs, any extra imports
// needed (from validators/defaults), API description coverage stats, and any
// merge-time errors. The caller (cmd/schemagen) treats a non-empty errs slice
// as a hard failure — errors surface yaml entries that have drifted out of
// alignment with the proto (unknown rename target, missing from_proto field,
// unknown validator).
//
// Description precedence (highest wins):
//  1. YAML config `description:` field
//  2. apiIndex lookup by "<cfg.APISchema>.<proto.path>" (when both are set)
//  3. mechanical defaults from descriptions.go
//
// When apiIndex is nil or cfg.APISchema is empty, layer 2 is skipped.
func Merge(proto *ProtoMessage, cfg *Config, schemaType string, apiIndex *apidesc.Index) (attrs []SchemaAttr, extraImports []string, stats apidesc.Stats, errs []error) {
	opts := mapOptions{
		computedDefault:  cfg.ComputedDefault,
		deriveValidators: schemaType != SchemaTypeDatasource,
	}
	attrs = mapProtoFields(proto, "", opts)

	applyTimestampDeny(&attrs)

	if apiIndex != nil && cfg.APISchema != "" {
		stats = applyAPIDescriptions(attrs, cfg.APISchema, "", apiIndex)
	}

	applyAutoDescriptions(attrs, "")

	mc := &mergeCtx{
		extraImports:     &extraImports,
		isDatasource:     schemaType == SchemaTypeDatasource,
		errs:             &errs,
		deriveValidators: opts.deriveValidators,
		resourceLabel:    classifierResourceLabel(cfg),
		source:           cfg.Source(),
	}
	applyFieldConfigs(&attrs, cfg.Fields, proto, "", mc)

	applyAutoPlanModifiers(attrs, nil, mc, "")

	if opts.deriveValidators {
		appendValidatorDescriptions(attrs, proto, "")
	}

	sortAttrs(attrs)

	return attrs, extraImports, stats, errs
}

var deniedTimestampFields = map[string]bool{
	"created_at":  true,
	"updated_at":  true,
	"deleted_at":  true,
	"create_time": true,
	"update_time": true,
	"delete_time": true,
}

func applyTimestampDeny(attrs *[]SchemaAttr) {
	for name := range deniedTimestampFields {
		removeAttr(attrs, name)
	}
	for i := range *attrs {
		a := &(*attrs)[i]
		if len(a.NestedAttrs) == 0 {
			continue
		}
		applyTimestampDeny(&a.NestedAttrs)
	}
}

func applyAPIDescriptions(attrs []SchemaAttr, rootSchema, parentPath string, idx *apidesc.Index) apidesc.Stats {
	var s apidesc.Stats
	for i := range attrs {
		s.Attempted++
		path := rootSchema + "." + attrs[i].Name
		if parentPath != "" {
			path = rootSchema + "." + parentPath + "." + attrs[i].Name
		}
		if attrs[i].Description == "" {
			if desc, ok := idx.Lookup(path); ok {
				attrs[i].Description = desc
				s.Matched++
			}
		}
		if len(attrs[i].NestedAttrs) > 0 {
			child := attrs[i].Name
			if parentPath != "" {
				child = parentPath + "." + attrs[i].Name
			}
			cs := applyAPIDescriptions(attrs[i].NestedAttrs, rootSchema, child, idx)
			s.Attempted += cs.Attempted
			s.Matched += cs.Matched
		}
	}
	return s
}

type mergeCtx struct {
	extraImports     *[]string
	isDatasource     bool
	errs             *[]error
	deriveValidators bool
	ancestors        []ancestorFrame
	resourceLabel    string
	source           string
}

func (mc *mergeCtx) errorf(format string, args ...any) {
	err := fmt.Errorf(format, args...)
	if mc.source != "" {
		err = fmt.Errorf("%s: %w", mc.source, err)
	}
	*mc.errs = append(*mc.errs, err)
}

func applyFieldConfigs(attrs *[]SchemaAttr, fields map[string]FieldConfig, protoCtx *ProtoMessage, parentPath string, mc *mergeCtx) {
	for name, fc := range fields { //nolint:gocritic // rangeValCopy acceptable for config structs
		path := joinPath(parentPath, name)

		if deniedTimestampFields[name] {
			fmt.Fprintf(os.Stderr, "WARNING: yaml entry %q references a server-managed timestamp field that is globally excluded — drop it\n", name)
			continue
		}

		if fc.Exclude {
			removeAttr(attrs, name)
			continue
		}

		if fc.ProtoOnly {
			if !protoHasField(protoCtx, name) {
				mc.errorf("yaml %s: proto_only — proto %s has no field named %q", path, protoMessageName(protoCtx), name)
				continue
			}
			if fc.Extra || fc.Todo || fc.FromProto != "" || fc.FlattenVia != "" || fc.ExpandVia != "" {
				mc.errorf("yaml %s: proto_only is mutually exclusive with extra/todo/from_proto/flatten_via/expand_via", path)
				continue
			}
			removeAttr(attrs, name)
			continue
		}

		if fc.Todo {
			if findAttrDirect(attrs, name) == nil {
				mc.errorf("yaml %s: stale todo — proto %s no longer has a field named %q (remove this todo entry)", path, protoMessageName(protoCtx), name)
				continue
			}
			fmt.Fprintf(os.Stderr, "WARNING: proto field %q not yet implemented — TODO: add schema support\n", name)
			removeAttr(attrs, name)
			continue
		}

		if fc.Extra || fc.Deprecated {
			if fc.FromProto != "" && !protoHasField(protoCtx, fc.FromProto) {
				mc.errorf("yaml %s: from_proto %q does not match any field on proto %s", path, fc.FromProto, protoMessageName(protoCtx))
			}
			synth := syntheticToSchemaAttr(name, fc, path, mc)
			if synth.Description == "" {
				synth.Description = generateDescription(name, "", synth.AttrType)
			}

			if len(fc.Fields) > 0 {
				mc.ancestors = append(mc.ancestors, frameForAttr(&synth))
				if fc.Synthetic {
					applySyntheticFields(&synth.NestedAttrs, fc.Fields, path, mc)
				} else {
					applyFieldConfigs(&synth.NestedAttrs, fc.Fields, protoCtx, path, mc)
				}
				mc.ancestors = mc.ancestors[:len(mc.ancestors)-1]
			}
			*attrs = append(*attrs, synth)
			continue
		}

		if fc.Synthetic {
			synth := syntheticToSchemaAttr(name, fc, path, mc)
			if fc.Type == "" && len(fc.Fields) > 0 {
				synth.AttrType = AttrTypeSingleNested
				synth.ElementType = ""
			}
			if synth.Description == "" {
				synth.Description = generateDescription(name, "", synth.AttrType)
			}
			if len(fc.Fields) > 0 {
				mc.ancestors = append(mc.ancestors, frameForAttr(&synth))
				applySyntheticFields(&synth.NestedAttrs, fc.Fields, path, mc)
				mc.ancestors = mc.ancestors[:len(mc.ancestors)-1]
			}
			if existing := findAttrDirect(attrs, name); existing != nil {
				*existing = synth
			} else {
				*attrs = append(*attrs, synth)
			}
			continue
		}

		attr := findAttrDirect(attrs, name)
		if attr == nil {
			mc.errorf("yaml %s: no matching proto field on %s — yaml entry is stale or misnamed", path, protoMessageName(protoCtx))
			continue
		}

		applyFieldConfig(attr, path, fc, mc)

		if mc.deriveValidators && !fc.SkipProtoValidation {
			if pf := protoCtx.FindField(name); pf != nil && pf.ValidateRules.GetRequired() {
				if fc.Optional != nil && *fc.Optional && !fc.ComputedOnly {
					mc.errorf("yaml %s: proto declares (buf.validate.field).required = true but yaml sets optional: true — drop the override, or mark computed_only if the value is server-populated", path)
				}
			}
		}

		if len(fc.Fields) > 0 {
			mc.ancestors = append(mc.ancestors, frameForAttr(attr))
			applyFieldConfigs(&attr.NestedAttrs, fc.Fields, childProtoMessage(protoCtx, name), path, mc)
			mc.ancestors = mc.ancestors[:len(mc.ancestors)-1]
		}

		if fc.Rename != "" {
			attr.Name = fc.Rename
		}
	}
}

func findAttrDirect(attrs *[]SchemaAttr, name string) *SchemaAttr {
	for i := range *attrs {
		if (*attrs)[i].Name == name {
			return &(*attrs)[i]
		}
	}
	return nil
}

type mapOptions struct {
	computedDefault  bool
	deriveValidators bool
}

func mapProtoFields(msg *ProtoMessage, prefix string, opts mapOptions) []SchemaAttr {
	if msg == nil {
		return nil
	}
	var attrs []SchemaAttr
	for i := range msg.Fields {
		f := &msg.Fields[i]
		attr := SchemaAttr{
			Name:      f.Name,
			ProtoName: f.Name,
			Optional:  !opts.computedDefault,
			Computed:  true,
		}

		switch f.Cardinality {
		case KindMap:
			if f.Nested != nil {
				attr.AttrType = AttrTypeMapNested
				attr.NestedAttrs = mapProtoFields(f.Nested, joinPath(prefix, f.Name), opts)
			} else {
				attr.AttrType = AttrTypeMap
				attr.ElementType = elementTypeForKind(f.MapValKind)
			}
		case "repeated":
			if f.Nested != nil {
				attr.AttrType = AttrTypeListNested
				attr.NestedAttrs = mapProtoFields(f.Nested, joinPath(prefix, f.Name), opts)
			} else {
				attr.AttrType = AttrTypeList
				attr.ElementType = elementTypeForKind(f.Kind)
			}
		default:
			switch f.Kind {
			case KindMessage:
				attr.AttrType = AttrTypeSingleNested
				attr.NestedAttrs = mapProtoFields(f.Nested, joinPath(prefix, f.Name), opts)
			case "status":
				attr.AttrType = AttrTypeSingleNested
				attr.NestedAttrs = []SchemaAttr{
					{Name: "code", AttrType: AttrTypeInt32, Computed: true},
					{Name: "message", AttrType: AttrTypeString, Computed: true},
				}
			case "timestamp", "duration", "json_struct":
				attr.AttrType = goTypeForProtoKind(f.Kind)
			default:
				attr.AttrType = goTypeForProtoKind(f.Kind)
			}
		}

		if opts.deriveValidators && f.ValidateRules != nil {
			applyProtoValidatorDerivations(&attr, f, opts.computedDefault)
		}

		if f.Kind == "enum" && len(f.EnumValues) > 0 {
			attr.EnumValues = append(attr.EnumValues, f.EnumValues...)
		}

		attrs = append(attrs, attr)
	}
	return attrs
}

func applyProtoValidatorDerivations(attr *SchemaAttr, f *ProtoField, computedDefault bool) {
	if f.ValidateRules.GetRequired() && !computedDefault {
		attr.Required = true
		attr.Optional = false
		attr.Computed = false
	}
}

func joinDescription(base, suffix string) string {
	base = strings.TrimRight(base, " ")
	if base == "" {
		return suffix
	}
	last := base[len(base)-1]
	if last != '.' && last != '!' && last != '?' && last != ':' {
		base += "."
	}
	return base + " " + suffix
}

func appendValidatorDescriptions(attrs []SchemaAttr, proto *ProtoMessage, parentPath string) {
	if proto == nil {
		return
	}
	for i := range attrs {
		f := proto.FindField(attrs[i].ProtoName)
		if f == nil {
			continue
		}
		if f.ValidateRules != nil {
			path := joinPath(parentPath, attrs[i].ProtoName)
			if summary := constraintSummary(f.ValidateRules, path); summary != "" {
				attrs[i].Description = joinDescription(attrs[i].Description, summary)
			}
		}
		if len(attrs[i].NestedAttrs) > 0 && f.Nested != nil {
			appendValidatorDescriptions(attrs[i].NestedAttrs, f.Nested, joinPath(parentPath, attrs[i].ProtoName))
		}
	}
}

func applyFieldConfig(attr *SchemaAttr, path string, fc FieldConfig, mc *mergeCtx) {
	if fc.ForceType != "" {
		attr.AttrType = fc.ForceType
		if !strings.Contains(fc.ForceType, "Nested") {
			attr.NestedAttrs = nil
		}
	}

	if fc.Required {
		attr.Required = true
		attr.Optional = false
		attr.Computed = false
	}
	if fc.ComputedOnly {
		attr.Required = false
		attr.Optional = false
		attr.Computed = true
		propagateFlags(attr.NestedAttrs, false, false, true)
	}

	if len(fc.PlanModifiers) > 0 && !mc.isDatasource {
		attr.PlanModifierNames = fc.PlanModifiers
	}

	if fc.Optional != nil {
		attr.Optional = *fc.Optional
		if *fc.Optional {
			attr.Required = false
		}
	}
	if fc.Computed != nil {
		attr.Computed = *fc.Computed
		if *fc.Computed {
			attr.Required = false
		}
	}

	if len(fc.PlanModifiers) > 0 && !mc.isDatasource {
		diagnoseOverride(mc.resourceLabel, path, fc.PlanModifiers, mc.ancestors, attr.Optional && attr.Computed)
	}

	if fc.Sensitive {
		attr.Sensitive = true
	}
	if fc.WriteOnly {
		attr.WriteOnly = true
	}
	if fc.DeprecationMessage != "" {
		attr.DeprecationMessage = fc.DeprecationMessage
	}

	if fc.Description != "" {
		attr.Description = fc.Description
	}

	attr.Validators = resolveValidatorList(fc.ValidatorNames(), path, attr.AttrType, mc.extraImports, mc)

	if fc.Default != nil {
		expr, imports := resolveDefault(fc.Default, attr.AttrType)
		attr.Default = expr
		*mc.extraImports = append(*mc.extraImports, imports...)
	}

	if fc.MinimalDefault != nil {
		attr.MinimalDefault = contingentDefaultExpr(fc.MinimalDefault, attr.AttrType)
	}

	if fc.FlattenSkip {
		attr.FlattenSkip = true
	}
}

func applySyntheticFields(attrs *[]SchemaAttr, fields map[string]FieldConfig, parentPath string, mc *mergeCtx) {
	names := make([]string, 0, len(fields))
	for n := range fields {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		fc := fields[name]
		path := joinPath(parentPath, name)
		if fc.Exclude {
			continue
		}
		if deniedTimestampFields[name] {
			fmt.Fprintf(os.Stderr, "WARNING: yaml entry %q references a server-managed timestamp field that is globally excluded — drop it\n", name)
			continue
		}
		synth := syntheticToSchemaAttr(name, fc, path, mc)
		if fc.Type == "" && len(fc.Fields) > 0 {
			synth.AttrType = AttrTypeSingleNested
			synth.ElementType = ""
		}
		if synth.Description == "" {
			synth.Description = generateDescription(name, "", synth.AttrType)
		}
		if len(fc.Fields) > 0 {
			mc.ancestors = append(mc.ancestors, frameForAttr(&synth))
			applySyntheticFields(&synth.NestedAttrs, fc.Fields, path, mc)
			mc.ancestors = mc.ancestors[:len(mc.ancestors)-1]
		}
		*attrs = append(*attrs, synth)
	}
}

func syntheticToSchemaAttr(name string, fc FieldConfig, path string, mc *mergeCtx) SchemaAttr {
	attrType, elemType := resolveCompositeType(fc.Type)

	if fc.Type == "" && len(fc.Fields) > 0 {
		attrType = AttrTypeSingleNested
		elemType = ""
	}
	attr := SchemaAttr{
		Name:               name,
		AttrType:           attrType,
		ElementType:        elemType,
		Description:        fc.Description,
		Required:           fc.Required,
		Optional:           fc.Optional != nil && *fc.Optional,
		Computed:           fc.Computed != nil && *fc.Computed,
		Sensitive:          fc.Sensitive,
		WriteOnly:          fc.WriteOnly,
		DeprecationMessage: fc.DeprecationMessage,
	}

	if fc.ComputedOnly {
		attr.Computed = true
		attr.Optional = false
	}
	if len(fc.PlanModifiers) > 0 && !mc.isDatasource {
		attr.PlanModifierNames = fc.PlanModifiers
		diagnoseOverride(mc.resourceLabel, path, fc.PlanModifiers, mc.ancestors, attr.Optional && attr.Computed)
	}

	attr.Validators = resolveValidatorList(fc.ValidatorNames(), name, attrType, mc.extraImports, mc)

	if fc.Default != nil {
		expr, imports := resolveDefault(fc.Default, attrType)
		attr.Default = expr
		*mc.extraImports = append(*mc.extraImports, imports...)
	}

	if fc.MinimalDefault != nil {
		attr.MinimalDefault = contingentDefaultExpr(fc.MinimalDefault, attrType)
	}

	if fc.ElementType != "" && attr.ElementType == "" {
		attr.ElementType = elementTypeForKind(fc.ElementType)
	}

	if fc.FlattenSkip {
		attr.FlattenSkip = true
	}

	return attr
}

func resolveCompositeType(t string) (attrType, elemType string) {
	switch t {
	case "list_string":
		return AttrTypeList, elemTypeString
	case "list_int":
		return AttrTypeList, elemTypeInt64
	case "list_bool":
		return AttrTypeList, elemTypeBool
	case KindList:
		return AttrTypeList, ""
	case KindSet:
		return AttrTypeSet, ""
	case "map_string":
		return AttrTypeMap, elemTypeString
	case "map":
		return AttrTypeMap, ""
	case "object":
		return AttrTypeSingleNested, ""
	case "list_nested":
		return AttrTypeListNested, ""
	case "map_nested":
		return AttrTypeMapNested, ""
	case "set_nested":
		return AttrTypeSetNested, ""
	default:
		return goTypeForProtoKind(t), ""
	}
}

func resolveDefault(val any, attrType string) (expr string, imports []string) {
	switch v := val.(type) {
	case bool:
		return fmt.Sprintf("booldefault.StaticBool(%v)", v),
			[]string{"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"}
	case string:
		return fmt.Sprintf("stringdefault.StaticString(%q)", v),
			[]string{"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"}
	case int:
		switch attrType {
		case AttrTypeInt32:
			return fmt.Sprintf("int32default.StaticInt32(%d)", v),
				[]string{"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32default"}
		default:
			return fmt.Sprintf("int64default.StaticInt64(%d)", v),
				[]string{"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"}
		}
	case float64:
		return fmt.Sprintf("float64default.StaticFloat64(%v)", v),
			[]string{"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64default"}
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

func applyAutoDescriptions(attrs []SchemaAttr, parentPath string) {
	for i := range attrs {
		if attrs[i].Description == "" {
			attrs[i].Description = generateDescription(attrs[i].Name, parentPath, attrs[i].AttrType)
		}
		if len(attrs[i].NestedAttrs) > 0 {
			childPath := attrs[i].Name
			if parentPath != "" {
				childPath = parentPath + "." + attrs[i].Name
			}
			applyAutoDescriptions(attrs[i].NestedAttrs, childPath)
		}
	}
}

// UncoveredField represents a proto field with no YAML config entry.
type UncoveredField struct {
	Path string
}

// FindUncoveredFields returns proto fields that have no corresponding entry
// in the YAML config. Walks the proto tree recursively, checking each field
// against the config at the same nesting level.
func FindUncoveredFields(proto *ProtoMessage, cfg *Config) []UncoveredField {
	var uncovered []UncoveredField
	findUncovered(proto, cfg.Fields, "", &uncovered)
	return uncovered
}

func findUncovered(msg *ProtoMessage, fields map[string]FieldConfig, prefix string, uncovered *[]UncoveredField) {
	if msg == nil {
		return
	}
	for i := range msg.Fields {
		f := &msg.Fields[i]
		path := f.Name
		if prefix != "" {
			path = prefix + "." + f.Name
		}

		fc, hasConfig := fields[f.Name]

		if deniedTimestampFields[f.Name] {
			continue
		}
		if !hasConfig {
			*uncovered = append(*uncovered, UncoveredField{Path: path})
		} else if fc.Exclude || fc.Todo || fc.ComputedOnly {
			continue
		}

		if f.Kind == KindMessage && f.Nested != nil {
			var childFields map[string]FieldConfig
			if hasConfig {
				childFields = fc.Fields
			}
			findUncovered(f.Nested, childFields, path, uncovered)
		}
	}
}

func sortAttrs(attrs []SchemaAttr) {
	sort.Slice(attrs, func(i, j int) bool {
		pi, pj := attrSortPriority(attrs[i]), attrSortPriority(attrs[j])
		if pi != pj {
			return pi < pj
		}
		return attrs[i].Name < attrs[j].Name
	})
	for i := range attrs {
		sortAttrs(attrs[i].NestedAttrs)
	}
}

func attrSortPriority(a SchemaAttr) int {
	switch {
	case a.Required:
		return 0
	case a.Optional && !a.Computed:
		return 1
	case a.Optional && a.Computed:
		return 2
	default:
		return 3
	}
}

func removeAttr(attrs *[]SchemaAttr, name string) {
	for i := range *attrs {
		if (*attrs)[i].Name == name {
			*attrs = append((*attrs)[:i], (*attrs)[i+1:]...)
			return
		}
	}
}

func propagateFlags(attrs []SchemaAttr, required, optional, computed bool) {
	for i := range attrs {
		attrs[i].Required = required
		attrs[i].Optional = optional
		attrs[i].Computed = computed
		propagateFlags(attrs[i].NestedAttrs, required, optional, computed)
	}
}

func applyAutoPlanModifiers(attrs []SchemaAttr, ancestors []ancestorFrame, mc *mergeCtx, parentPath string) {
	if mc.isDatasource {
		return
	}
	for i := range attrs {
		childPath := joinPath(parentPath, attrs[i].Name)
		names := attrs[i].PlanModifierNames

		if attrs[i].Computed && attrs[i].Default == "" && !containsStateNullModifier(names) {
			leafIsUserInput := attrs[i].Optional && attrs[i].Computed
			verdict := chooseStateModifier(ancestors, leafIsUserInput)
			debugClassify(mc.resourceLabel, childPath, verdict, ancestors, leafIsUserInput)
			names = append(names, verdict)
		}
		if len(names) > 0 {
			expr, err := planModifierExpr(attrs[i].AttrType, names)
			if err != nil {
				mc.errorf("plan modifiers for %q: %w", childPath, err)
			} else {
				attrs[i].PlanModifiers = expr
			}
		}
		childAncestors := append(append([]ancestorFrame{}, ancestors...), frameForAttr(&attrs[i]))
		applyAutoPlanModifiers(attrs[i].NestedAttrs, childAncestors, mc, childPath)
	}
}

func containsStateNullModifier(names []string) bool {
	for _, n := range names {
		if n == modUseStateForUnknown || n == modUseNonNullStateForUnknown || n == modNone {
			return true
		}
		if def, ok := planModifierRegistry[n]; ok && def.subsumesStateNullAxis {
			return true
		}
	}
	return false
}

func planModifierExpr(attrType string, modifiers []string) (string, error) {
	if len(modifiers) == 0 {
		return "", nil
	}
	pkg, err := planModifierPkgForAttr(attrType)
	if err != nil {
		return "", err
	}
	inners := make([]string, 0, len(modifiers))
	for _, m := range modifiers {
		if m == modNone {
			// Sentinel: suppresses the auto-added state modifier and emits
			// nothing on its own.
			continue
		}
		if def, ok := planModifierRegistry[m]; ok {
			inners = append(inners, def.expr(pkg))
			continue
		}
		inners = append(inners, fmt.Sprintf("%splanmodifier.%s()", pkg, m))
	}
	if len(inners) == 0 {
		return "", nil
	}
	return fmt.Sprintf("[]planmodifier.%s{%s}",
		strings.ToUpper(pkg[:1])+pkg[1:], strings.Join(inners, ", ")), nil
}

func planModifierPkgForAttr(attrType string) (string, error) {
	switch attrType {
	case AttrTypeString:
		return KindString, nil
	case AttrTypeBool:
		return KindBool, nil
	case AttrTypeInt32:
		return KindInt32, nil
	case AttrTypeInt64:
		return KindInt64, nil
	case AttrTypeFloat64:
		return "float64", nil
	case AttrTypeNumber:
		return "number", nil
	case AttrTypeList, AttrTypeListNested:
		return KindList, nil
	case AttrTypeSet, AttrTypeSetNested:
		return KindSet, nil
	case AttrTypeMap, AttrTypeMapNested:
		return KindMap, nil
	case AttrTypeSingleNested, "ObjectAttribute":
		return "object", nil
	default:
		return "", fmt.Errorf("schemagen: unsupported AttrType %q for planModifierPkgForAttr", attrType)
	}
}

func joinPath(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

func resolveValidatorList(names []string, path, attrType string, extraImports *[]string, mc *mergeCtx) string {
	if len(names) == 0 {
		return ""
	}
	valAttrType := validatorAttrType(attrType)
	var exprs []string
	var directSlice string
	for _, n := range names {
		expr, imports, returnsSlice, err := resolveValidator(n, path, valAttrType)
		if err != nil {
			mc.errorf("yaml %s: %w", path, err)
			continue
		}
		if returnsSlice && len(names) == 1 {
			directSlice = expr
		} else {
			exprs = append(exprs, expr)
		}
		*extraImports = append(*extraImports, imports...)
	}
	if directSlice != "" {
		return directSlice
	}
	if len(exprs) > 0 {
		*extraImports = append(*extraImports, "github.com/hashicorp/terraform-plugin-framework/schema/validator")
		return wrapValidatorSlice(valAttrType, exprs)
	}
	return ""
}

func protoMessageName(m *ProtoMessage) string {
	if m == nil || m.Name == "" {
		return "<unknown>"
	}
	return m.Name
}

func protoHasField(m *ProtoMessage, name string) bool {
	if m == nil {
		return true
	}
	return m.FindField(name) != nil
}

func childProtoMessage(parent *ProtoMessage, name string) *ProtoMessage {
	f := parent.FindField(name)
	if f == nil {
		return nil
	}
	return f.Nested
}
