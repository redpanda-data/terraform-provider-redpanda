// Copyright 2025 Redpanda Data, Inc.
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

package schema

import (
	"fmt"
	"sort"
	"strings"

	"github.com/emicklei/proto"
)

// ProtobufBodiesEquivalent reports whether two PROTOBUF schema bodies represent
// the same schema after canonicalization. Schema Registry canonicalizes
// protobuf on write — it reorders top-level definitions (enums before
// messages) and fully-qualifies in-package type references — so the stored
// body differs textually from the user's input even when nothing changed.
//
// Both bodies are parsed and reduced to an order-independent canonical form
// with type references normalized to their package-relative shape. Any parse
// failure or unsupported construct yields false, so the caller falls back to
// the registry body (no behavior change versus storing the registry form).
func ProtobufBodiesEquivalent(a, b string) bool {
	if a == b {
		return true
	}
	ca, err := canonicalizeProtobuf(a)
	if err != nil {
		return false
	}
	cb, err := canonicalizeProtobuf(b)
	if err != nil {
		return false
	}
	return ca == cb
}

func canonicalizeProtobuf(src string) (string, error) {
	def, err := proto.NewParser(strings.NewReader(src)).Parse()
	if err != nil {
		return "", err
	}
	pkg := protoPackageName(def.Elements)
	parts, err := canonProtoElements(def.Elements, pkg)
	if err != nil {
		return "", err
	}
	return strings.Join(parts, "\n"), nil
}

func protoPackageName(elems []proto.Visitee) string {
	for _, e := range elems {
		if p, ok := e.(*proto.Package); ok {
			return p.Name
		}
	}
	return ""
}

// canonProtoElements renders each semantic element to a self-describing string
// and returns them sorted, making the result independent of declaration order.
// Comments are non-semantic and skipped; unsupported constructs error out.
func canonProtoElements(elems []proto.Visitee, pkg string) ([]string, error) {
	out := make([]string, 0, len(elems))
	for _, e := range elems {
		switch v := e.(type) {
		case *proto.Comment:
			// non-semantic
		case *proto.Syntax:
			out = append(out, "syntax="+v.Value)
		case *proto.Package:
			out = append(out, "package="+v.Name)
		case *proto.Import:
			out = append(out, "import="+v.Kind+":"+v.Filename)
		case *proto.Option:
			out = append(out, "option="+v.Name+"="+renderProtoLiteral(v.Constant))
		case *proto.Message:
			inner, err := canonProtoElements(v.Elements, pkg)
			if err != nil {
				return nil, err
			}
			kind := "message"
			if v.IsExtend {
				kind = "extend"
			}
			out = append(out, kind+" "+v.Name+"{"+strings.Join(inner, ";")+"}")
		case *proto.Enum:
			inner, err := canonProtoElements(v.Elements, pkg)
			if err != nil {
				return nil, err
			}
			out = append(out, "enum "+v.Name+"{"+strings.Join(inner, ";")+"}")
		case *proto.EnumField:
			out = append(out, fmt.Sprintf("enumvalue:%s=%d", v.Name, v.Integer))
		case *proto.NormalField:
			label := ""
			switch {
			case v.Repeated:
				label = "repeated "
			case v.Optional:
				label = "optional "
			case v.Required:
				label = "required "
			default:
			}
			out = append(out, fmt.Sprintf("field:%s%s %s=%d%s",
				label, normProtoType(pkg, v.Type), v.Name, v.Sequence, renderFieldOptions(v.Options)))
		case *proto.MapField:
			out = append(out, fmt.Sprintf("map:<%s,%s> %s=%d%s",
				v.KeyType, normProtoType(pkg, v.Type), v.Name, v.Sequence, renderFieldOptions(v.Options)))
		case *proto.Oneof:
			inner, err := canonProtoElements(v.Elements, pkg)
			if err != nil {
				return nil, err
			}
			out = append(out, "oneof "+v.Name+"{"+strings.Join(inner, ";")+"}")
		case *proto.OneOfField:
			out = append(out, fmt.Sprintf("field:%s %s=%d%s",
				normProtoType(pkg, v.Type), v.Name, v.Sequence, renderFieldOptions(v.Options)))
		case *proto.Reserved:
			out = append(out, renderReserved(v))
		default:
			return nil, fmt.Errorf("unsupported protobuf element %T", e)
		}
	}
	sort.Strings(out)
	return out, nil
}

// normProtoType reduces a type reference to its package-relative form so that
// the registry's fully-qualified ".pkg.Type" and the user's "Type" compare
// equal. A leading dot is stripped, then the enclosing package prefix.
func normProtoType(pkg, typ string) string {
	t := strings.TrimPrefix(typ, ".")
	if pkg != "" {
		t = strings.TrimPrefix(t, pkg+".")
	}
	return t
}

func renderFieldOptions(opts []*proto.Option) string {
	if len(opts) == 0 {
		return ""
	}
	rendered := make([]string, 0, len(opts))
	for _, o := range opts {
		rendered = append(rendered, o.Name+"="+renderProtoLiteral(o.Constant))
	}
	sort.Strings(rendered)
	return "[" + strings.Join(rendered, ",") + "]"
}

func renderReserved(r *proto.Reserved) string {
	parts := make([]string, 0, len(r.Ranges)+len(r.FieldNames))
	for _, rng := range r.Ranges {
		if rng.Max {
			parts = append(parts, fmt.Sprintf("%d-max", rng.From))
		} else {
			parts = append(parts, fmt.Sprintf("%d-%d", rng.From, rng.To))
		}
	}
	parts = append(parts, r.FieldNames...)
	sort.Strings(parts)
	return "reserved:" + strings.Join(parts, ",")
}

func renderProtoLiteral(lit proto.Literal) string {
	switch {
	case len(lit.OrderedMap) > 0:
		entries := make([]string, 0, len(lit.OrderedMap))
		for _, nl := range lit.OrderedMap {
			entries = append(entries, nl.Name+":"+renderProtoLiteral(*nl.Literal))
		}
		sort.Strings(entries)
		return "{" + strings.Join(entries, ",") + "}"
	case len(lit.Array) > 0:
		entries := make([]string, 0, len(lit.Array))
		for _, l := range lit.Array {
			entries = append(entries, renderProtoLiteral(*l))
		}
		return "[" + strings.Join(entries, ",") + "]"
	default:
		return lit.Source
	}
}
