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
	"bytes"
	"fmt"
	"go/format"
	"sort"
	"strings"
	"text/template"
)

// ConversionData holds everything the flatten/expand templates need to render.
type ConversionData struct {
	License           string
	Package           string
	ProtoImport       string
	ProtoAlias        string
	ResponseInterface *ResponseInterfaceData
	ExtraImports      []string

	ExternalProtoImports map[string]string
	Imports              []ImportEntry

	ModelTypeName string

	FlattenFunc      string
	FlattenArg       string
	NestedFlatteners []NestedFlattener

	Expanders []Expander

	RootFieldConversions []FieldConversion

	HasPreservedFields bool

	HasTimeouts bool

	NestedPreserveBlocks []NestedPreserveBlock

	FlattenUsesCtx bool
}

// NestedPreserveBlock describes one nested SingleNested object whose
// write_only / sensitive / extra children must be carried forward from prev.
// The container is extracted from both prev and m, leaves are copied from
// prev's typed struct into m's, and the result is re-encoded back into m.X
// via the ToObject helper.
type NestedPreserveBlock struct {
	ContainerGoName string

	AsFunc string

	ToObjectFunc string

	Children []NestedPreserveChild

	// SubBlocks holds 2nd-level nested containers whose sensitive/write-only
	// leaves must also be carried forward. Each sub-block is decoded from the
	// parent's typed struct, updated, and re-encoded back.
	SubBlocks []NestedPreserveSubBlock
}

// NestedPreserveSubBlock is a single-level nested container inside a
// NestedPreserveBlock's typed struct.
type NestedPreserveSubBlock struct {
	SubContainerGoName string

	// DecodeFunc is a free-standing function that decodes the sub-container
	// from the parent struct, e.g. "DecodeClientOptionsTLSSettings".
	DecodeFunc string

	ToObjectFunc string

	Children []NestedPreserveChild

	// SubSubBlocks holds a third level of nested containers.
	SubSubBlocks []NestedPreserveSubSubBlock
}

// NestedPreserveSubSubBlock is a third-level nested container.
type NestedPreserveSubSubBlock struct {
	SubSubContainerGoName string

	DecodeFunc string

	ToObjectFunc string

	Children []NestedPreserveChild
}

// NestedPreserveChild names one leaf restored from prev's nested struct.
type NestedPreserveChild struct {
	GoName string
}

// ImportEntry is a single import line. Alias may be empty for unaliased imports.
type ImportEntry struct {
	Alias string
	Path  string
}

// ResponseInterfaceData captures the synthetic response interface to emit.
type ResponseInterfaceData struct {
	Name    string
	Methods []ResponseMethod
}

// ResponseMethod is one method signature on the response interface.
type ResponseMethod struct {
	Signature string
}

// NestedFlattener emits Flatten<Name> / Expand<Name> for one nested message type.
//
// When a nested field's proto type differs across RPCs (e.g. cluster's
// kafka_api is *Cluster_KafkaAPI on Read but *KafkaAPISpec on Create),
// multiple NestedFlatteners are produced for the same TypeName: one for
// the canonical Flatten emission (read shape) and one per divergent RPC
// for Expand emission. EmitFlatten / EmitExpand gate which functions a
// given entry produces.
type NestedFlattener struct {
	TypeName      string
	FuncSuffix    string
	ProtoType     string
	ProtoTypeBare string
	Conversions   []FieldConversion

	EmitFlatten bool
	EmitExpand  bool

	FlattenUsesCtx bool
	ExpandUsesCtx  bool
}

// Expander emits one ExpandCreate / ExpandUpdate / ExpandDelete function.
type Expander struct {
	FuncName     string
	RequestType  string
	PayloadField string
	PayloadType  string
	IsFlat       bool

	ReturnPayload bool
	Conversions   []FieldConversion

	UsesCtx bool
}

// FieldConversion describes one field's flatten + expand emission.
//
// The two directions are independent: a TF-only contingent field has only a
// flatten line; a write-only password has a custom expand expression.
type FieldConversion struct {
	GoName      string
	Tag         string
	ProtoField  string
	ProtoGoName string
	Kind        FieldKind

	FlattenExpr string

	ExpandExpr string

	FlattenStmt string

	ExpandStmt string

	Preserve bool

	OverrideFromPrev bool

	PreserveDefault string

	HasPresence bool

	NullExpr string

	IsOneofVariant bool

	ExpandFuncSuffix string
}

// FieldKind is a coarse category controlling template emission shape.
type FieldKind int

// Field kinds tracked by the planner. These drive template choices and
// detect which shared helper packages need imports.
const (
	FieldKindScalar FieldKind = iota
	FieldKindEnum
	FieldKindTimestamp
	FieldKindDuration
	FieldKindListScalar
	FieldKindListMessage
	FieldKindMapScalar
	FieldKindMapMessage
	FieldKindSingleNested
	FieldKindCustom
)

const flattenExpandTmpl = `// Code generated by schemagen. DO NOT EDIT.

{{.License}}package {{.Package}}

import (
{{- range .Imports}}
	{{if .Alias}}{{.Alias}} {{end}}"{{.Path}}"
{{- end}}
)

{{- if .ResponseInterface}}

type {{.ResponseInterface.Name}} interface {
{{- range .ResponseInterface.Methods}}
	{{.Signature}}
{{- end}}
}
{{- end}}

// {{.FlattenFunc}} populates a *{{.ModelTypeName}} from the API response payload.
// The prev *{{.ModelTypeName}} arg carries forward TF-only / sensitive / write-only
// fields that the response cannot supply; pass nil when no prior state exists
// (e.g., during ImportState).
func {{.FlattenFunc}}({{if .FlattenUsesCtx}}ctx{{else}}_{{end}} context.Context, proto {{.FlattenArg}}, prev *{{.ModelTypeName}}) (*{{.ModelTypeName}}, diag.Diagnostics) {
	var diags diag.Diagnostics
	m := &{{.ModelTypeName}}{}
{{- range .RootFieldConversions}}
{{- if .Preserve}}
{{- /* preserved-only field — only set inside the prev guard below */}}
{{- else if .FlattenStmt}}
	{{.FlattenStmt}}
{{- else if .FlattenExpr}}
{{- if .HasPresence}}
	if proto.Has{{.ProtoGoName}}() {
		m.{{.GoName}} = {{.FlattenExpr}}
	} else if prev != nil && !prev.{{.GoName}}.IsUnknown() {
		m.{{.GoName}} = prev.{{.GoName}}
{{- if .NullExpr}}
	} else {
		m.{{.GoName}} = {{.NullExpr}}
{{- end}}
	}
{{- else}}
	m.{{.GoName}} = {{.FlattenExpr}}
{{- end}}
{{- end}}
{{- end}}
{{- range .RootFieldConversions}}
{{- if .Preserve}}
	if prev != nil && !prev.{{.GoName}}.IsUnknown() {
		m.{{.GoName}} = prev.{{.GoName}}
{{- if .NullExpr}}
	} else {
		m.{{.GoName}} = {{.NullExpr}}
{{- end}}
	}
{{- end}}
{{- end}}
{{- if .HasTimeouts}}
	if prev != nil {
		m.Timeouts = prev.Timeouts
	}
{{- end}}
{{- range .RootFieldConversions}}
{{- if .PreserveDefault}}
	if m.{{.GoName}}.IsNull() || m.{{.GoName}}.IsUnknown() {
		m.{{.GoName}} = {{.PreserveDefault}}
	}
{{- end}}
{{- end}}
{{- if .NestedPreserveBlocks}}
	// Restore nested write_only and non-deprecated extra fields from prev —
	// the nested Flatten functions are called via a generic helper that
	// can't see prev, so without this block these leaves come back null
	// from the API echo.
{{- range .NestedPreserveBlocks}}
{{- $container := .ContainerGoName}}
	if prev != nil {
		if prev{{$container}}Nested, prev{{$container}}Diags := prev.{{.AsFunc}}(ctx); !prev{{$container}}Diags.HasError() && prev{{$container}}Nested != nil {
			if cur{{$container}}Nested, cur{{$container}}Diags := m.{{.AsFunc}}(ctx); !cur{{$container}}Diags.HasError() && cur{{$container}}Nested != nil {
{{- range .Children}}
				if !prev{{$container}}Nested.{{.GoName}}.IsUnknown() {
					cur{{$container}}Nested.{{.GoName}} = prev{{$container}}Nested.{{.GoName}}
				}
{{- end}}
{{- range .SubBlocks}}
{{- $sub := .SubContainerGoName}}
				if prev{{$sub}}Nested, prev{{$sub}}Diags := {{.DecodeFunc}}(ctx, prev{{$container}}Nested); !prev{{$sub}}Diags.HasError() && prev{{$sub}}Nested != nil {
					if cur{{$sub}}Nested, cur{{$sub}}Diags := {{.DecodeFunc}}(ctx, cur{{$container}}Nested); !cur{{$sub}}Diags.HasError() && cur{{$sub}}Nested != nil {
{{- range .Children}}
						if !prev{{$sub}}Nested.{{.GoName}}.IsUnknown() {
							cur{{$sub}}Nested.{{.GoName}} = prev{{$sub}}Nested.{{.GoName}}
						}
{{- end}}
{{- range .SubSubBlocks}}
{{- $subsub := .SubSubContainerGoName}}
						if prev{{$subsub}}Nested, prev{{$subsub}}Diags := {{.DecodeFunc}}(ctx, prev{{$sub}}Nested); !prev{{$subsub}}Diags.HasError() && prev{{$subsub}}Nested != nil {
							if cur{{$subsub}}Nested, cur{{$subsub}}Diags := {{.DecodeFunc}}(ctx, cur{{$sub}}Nested); !cur{{$subsub}}Diags.HasError() && cur{{$subsub}}Nested != nil {
{{- range .Children}}
								if !prev{{$subsub}}Nested.{{.GoName}}.IsUnknown() {
									cur{{$subsub}}Nested.{{.GoName}} = prev{{$subsub}}Nested.{{.GoName}}
								}
{{- end}}
								if {{$subsub | toLower}}Obj, {{$subsub | toLower}}ObjDiags := {{.ToObjectFunc}}(ctx, cur{{$subsub}}Nested); !{{$subsub | toLower}}ObjDiags.HasError() {
									cur{{$sub}}Nested.{{$subsub}} = {{$subsub | toLower}}Obj
								}
							}
						}
{{- end}}
						if {{$sub | toLower}}Obj, {{$sub | toLower}}ObjDiags := {{.ToObjectFunc}}(ctx, cur{{$sub}}Nested); !{{$sub | toLower}}ObjDiags.HasError() {
							cur{{$container}}Nested.{{$sub}} = {{$sub | toLower}}Obj
						}
					}
				}
{{- end}}
				if {{$container | toLower}}Obj, {{$container | toLower}}ObjDiags := {{.ToObjectFunc}}(ctx, cur{{$container}}Nested); !{{$container | toLower}}ObjDiags.HasError() {
					m.{{$container}} = {{$container | toLower}}Obj
				}
			}
		}
	}
{{- end}}
{{- end}}
{{- range .RootFieldConversions}}
{{- if .OverrideFromPrev}}
	if prev != nil && !prev.{{.GoName}}.IsNull() && !prev.{{.GoName}}.IsUnknown() {
		m.{{.GoName}} = prev.{{.GoName}}
	}
{{- end}}
{{- end}}
	return m, diags
}

{{- range .Expanders}}

// {{.FuncName}} renders a *{{$.ModelTypeName}} into the proto request envelope for
// the corresponding RPC.
func {{.FuncName}}({{if .UsesCtx}}ctx{{else}}_{{end}} context.Context, m *{{$.ModelTypeName}}) ({{.RequestType}}, diag.Diagnostics) {
	var diags diag.Diagnostics
{{- if .IsFlat}}
	req := &{{trimPtr .RequestType}}{
{{- range .Conversions}}
{{- if and .ExpandExpr (not .IsOneofVariant)}}
		{{.ProtoGoName}}: {{.ExpandExpr}},
{{- end}}
{{- end}}
	}
{{- range .Conversions}}
{{- if and .ExpandExpr .IsOneofVariant}}
	if v := {{.ExpandExpr}}; v != nil {
		req.Set{{.ProtoGoName}}(v)
	}
{{- end}}
{{- end}}
	return req, diags
{{- else}}
	payload := &{{trimPtr .PayloadType}}{
{{- range .Conversions}}
{{- if and .ExpandExpr (not .IsOneofVariant)}}
		{{.ProtoGoName}}: {{.ExpandExpr}},
{{- end}}
{{- end}}
	}
{{- range .Conversions}}
{{- if and .ExpandExpr .IsOneofVariant}}
	if v := {{.ExpandExpr}}; v != nil {
		payload.Set{{.ProtoGoName}}(v)
	}
{{- end}}
{{- end}}
{{- if .ReturnPayload}}
	return payload, diags
{{- else}}
	req := &{{trimPtr .RequestType}}{
		{{.PayloadField}}: payload,
	}
	return req, diags
{{- end}}
{{- end}}
}
{{- end}}

{{- range .NestedFlatteners}}
{{- if .EmitFlatten}}

// Flatten{{.FuncSuffix}} converts a single proto {{.ProtoTypeBare}} into the
// corresponding nested model. The prev *{{.TypeName}} arg carries forward
// TF-only / sensitive / write-only fields and resolves the proto3
// null-vs-empty ambiguity for Optional-only scalar leaves (Required leaves
// flatten directly); pass nil when no prior nested state is available.
func Flatten{{.FuncSuffix}}({{if .FlattenUsesCtx}}ctx{{else}}_{{end}} context.Context, proto {{.ProtoType}}, prev *{{.TypeName}}) ({{.TypeName}}, diag.Diagnostics) {
	var diags diag.Diagnostics
	_ = prev
	m := {{.TypeName}}{}
{{- range .Conversions}}
{{- if .FlattenStmt}}
	{{.FlattenStmt}}
{{- else if .FlattenExpr}}
{{- if .HasPresence}}
	if proto.Has{{.ProtoGoName}}() {
		m.{{.GoName}} = {{.FlattenExpr}}
	} else if prev != nil && !prev.{{.GoName}}.IsUnknown() {
		m.{{.GoName}} = prev.{{.GoName}}
{{- if .NullExpr}}
	} else {
		m.{{.GoName}} = {{.NullExpr}}
{{- end}}
	}
{{- else}}
	m.{{.GoName}} = {{.FlattenExpr}}
{{- end}}
{{- end}}
{{- end}}
	return m, diags
}
{{- end}}
{{- if .EmitExpand}}

// Expand{{.FuncSuffix}} renders a nested model back into the proto type.
func Expand{{.FuncSuffix}}({{if .ExpandUsesCtx}}ctx{{else}}_{{end}} context.Context, m *{{.TypeName}}) ({{.ProtoType}}, diag.Diagnostics) {
	var diags diag.Diagnostics
	if m == nil {
		return nil, diags
	}
	out := &{{.ProtoTypeBare}}{
{{- range .Conversions}}
{{- if and .ExpandExpr (not .IsOneofVariant) (not .ExpandStmt)}}
		{{.ProtoGoName}}: {{.ExpandExpr}},
{{- end}}
{{- end}}
	}
{{- range .Conversions}}
{{- if .ExpandStmt}}
	{{.ExpandStmt}}
{{- else if and .ExpandExpr .IsOneofVariant}}
	if v := {{.ExpandExpr}}; v != nil {
		out.Set{{.ProtoGoName}}(v)
	}
{{- end}}
{{- end}}
	return out, diags
}
{{- end}}
{{- end}}
`

// GenerateFlattenExpand renders flatten_gen.go + expand_gen.go (combined into
// one file for now; can be split later if it becomes too large) from the
// merged schema attrs and the API config.
func GenerateFlattenExpand(data ConversionData) ([]byte, error) {
	if data.License == "" {
		data.License = LicenseHeader()
	}

	plain := map[string]bool{
		"context": true,
		"github.com/hashicorp/terraform-plugin-framework/diag":  true,
		"github.com/hashicorp/terraform-plugin-framework/types": true,
	}
	for _, imp := range data.ExtraImports {
		if imp != data.ProtoImport {
			plain[imp] = true
		}
	}
	plainList := make([]string, 0, len(plain))
	for k := range plain {
		plainList = append(plainList, k)
	}
	sort.Strings(plainList)

	data.Imports = make([]ImportEntry, 0, len(plainList)+1)
	for _, p := range plainList {
		data.Imports = append(data.Imports, ImportEntry{Path: p})
	}

	funcMap := template.FuncMap{
		"trimPtr": func(s string) string { return strings.TrimPrefix(s, "*") },
		"toLower": func(s string) string {
			if s == "" {
				return s
			}
			return strings.ToLower(s[:1]) + s[1:]
		},
	}
	tmpl, err := template.New("flatten_expand").Funcs(funcMap).Parse(flattenExpandTmpl)
	if err != nil {
		return nil, fmt.Errorf("parse flatten/expand template: %w", err)
	}

	if data.ProtoImport != "" && referencesProtoAlias(data) {
		data.Imports = append(data.Imports, ImportEntry{Alias: data.ProtoAlias, Path: data.ProtoImport})
	}

	if len(data.ExternalProtoImports) > 0 {
		extAliases := make([]string, 0, len(data.ExternalProtoImports))
		for alias := range data.ExternalProtoImports {
			extAliases = append(extAliases, alias)
		}
		sort.Strings(extAliases)
		for _, alias := range extAliases {
			data.Imports = append(data.Imports, ImportEntry{Alias: alias, Path: data.ExternalProtoImports[alias]})
		}
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute flatten/expand template: %w", err)
	}
	src, err := format.Source(buf.Bytes())
	if err != nil {
		return buf.Bytes(), fmt.Errorf("format generated flatten/expand: %w\n%s", err, buf.String())
	}
	return src, nil
}

func referencesProtoAlias(data ConversionData) bool {
	alias := data.ProtoAlias + "."
	if strings.Contains(data.FlattenArg, alias) {
		return true
	}
	if data.ResponseInterface != nil {
		for i := range data.ResponseInterface.Methods {
			if strings.Contains(data.ResponseInterface.Methods[i].Signature, alias) {
				return true
			}
		}
	}
	for i := range data.Expanders {
		if strings.Contains(data.Expanders[i].RequestType, alias) ||
			strings.Contains(data.Expanders[i].PayloadType, alias) {
			return true
		}
	}
	for i := range data.RootFieldConversions {
		c := &data.RootFieldConversions[i]
		if strings.Contains(c.FlattenExpr, alias) || strings.Contains(c.FlattenStmt, alias) ||
			strings.Contains(c.ExpandExpr, alias) {
			return true
		}
	}
	return false
}
