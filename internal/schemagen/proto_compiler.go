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
	"context"
	"fmt"
	"path/filepath"
	"strings"

	bufvalidate "buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/linker"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const maxNestingDepth = 8

// CompileProto compiles a .proto file using protocompile and extracts both
// field structure and comments for a named message. Returns the same
// ProtoMessage type used by the merger, plus a map of field paths to
// descriptions extracted from proto comments.
//
// For multi-message lookups (flat-request walking, per-RPC payload type
// divergence) call CompileProtoFiles + ExtractMessage instead of compiling
// the package once per message.
func CompileProto(cloudv2Root, protoPkg, messageName string, extraImportPaths []string) (*ProtoMessage, map[string]string, error) {
	files, err := CompileProtoFiles(cloudv2Root, protoPkg, extraImportPaths)
	if err != nil {
		return nil, nil, err
	}
	return ExtractMessage(files, messageName, protoPkg)
}

// CompileProtoFiles compiles every .proto file under cloudv2Root/proto/.../<protoPkg>
// (plus any extraImportPaths) and returns the resolved file set. Files that
// fail to compile (e.g. because of unresolvable imports) are skipped with a
// log line — same forgiving behavior the original CompileProto used.
func CompileProtoFiles(cloudv2Root, protoPkg string, extraImportPaths []string) (linker.Files, error) {
	importPaths := []string{
		filepath.Join(cloudv2Root, "proto", "public", "cloud"),
		filepath.Join(cloudv2Root, "proto", "public", "common"),
		filepath.Join(cloudv2Root, "tools", "proto", "wellknown"),
	}
	importPaths = append(importPaths, extraImportPaths...)

	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
			ImportPaths: importPaths,
		}),
		SourceInfoMode: protocompile.SourceInfoStandard,
	}

	protoFiles := findProtoFiles(importPaths, protoPkg)
	if len(protoFiles) == 0 {
		return nil, fmt.Errorf("no .proto files found for package %s", protoPkg)
	}

	var files linker.Files
	for _, pf := range protoFiles {
		compiled, err := compiler.Compile(context.Background(), pf)
		if err != nil {
			return nil, fmt.Errorf("failed to compile %s: %w", pf, err)
		}
		files = append(files, compiled...)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("failed to compile any .proto files in %s", protoPkg)
	}
	return files, nil
}

// ExtractMessage finds messageName within an already-compiled file set and
// builds a ProtoMessage. Supports nested-message names like
// "CreateUserRequest_User" that map to "CreateUserRequest" containing "User".
// protoPkg is informational, used only for error reporting.
func ExtractMessage(files linker.Files, messageName, protoPkg string) (*ProtoMessage, map[string]string, error) {
	var msgDesc protoreflect.MessageDescriptor
	var msgFile protoreflect.FileDescriptor
	for _, f := range files {
		msgDesc = findMessage(f.Messages(), messageName)
		if msgDesc != nil {
			msgFile = f
			break
		}
	}
	if msgDesc == nil {
		return nil, nil, fmt.Errorf("message %s not found in %s", messageName, protoPkg)
	}
	descs := make(map[string]string)
	rootPkg := string(msgFile.Package())
	msg, err := extractMessageFromDescriptor(msgDesc, msgFile, "", descs, 0, rootPkg)
	if err != nil {
		return nil, nil, err
	}
	return msg, descs, nil
}

func extractMessageFromDescriptor(
	md protoreflect.MessageDescriptor,
	file protoreflect.FileDescriptor,
	prefix string,
	descs map[string]string,
	depth int,
	rootPkg string,
) (*ProtoMessage, error) {
	if depth > maxNestingDepth {
		return nil, fmt.Errorf("proto descriptor at %q exceeded maxNestingDepth=%d", msgPath(prefix, string(md.Name())), maxNestingDepth)
	}

	msg := &ProtoMessage{
		Name:   string(md.Name()),
		GoName: messageGoName(md),
	}

	if pkg := string(md.ParentFile().Package()); pkg != "" && pkg != rootPkg {
		if ext, ok := ExternalProtoPackages[pkg]; ok {
			msg.ExternalPkgAlias = ext.Alias
			msg.ExternalPkgImport = ext.GoImport
		}
	}

	fields := md.Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		pf, err := extractFieldFromDescriptor(fd, file, prefix, descs, depth, rootPkg)
		if err != nil {
			return nil, err
		}
		msg.Fields = append(msg.Fields, pf)
	}

	return msg, nil
}

func msgPath(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

func extractFieldFromDescriptor(
	fd protoreflect.FieldDescriptor,
	file protoreflect.FileDescriptor,
	prefix string,
	descs map[string]string,
	depth int,
	rootPkg string,
) (ProtoField, error) {
	name := string(fd.Name())
	path := name
	if prefix != "" {
		path = prefix + "." + name
	}

	loc := file.SourceLocations().ByDescriptor(fd)
	comment := strings.TrimSpace(loc.LeadingComments)
	if comment != "" {
		comment = cleanComment(comment)
		descs[path] = comment
	}

	pf := ProtoField{
		Name:        name,
		Cardinality: "singular",
	}

	if opts := fd.Options(); opts != nil {
		rules, err := extractFieldRules(opts)
		if err != nil {
			return ProtoField{}, fmt.Errorf("field %q: %w", path, err)
		}
		if rules != nil {
			pf.ValidateRules = rules
		}
	}

	if fd.IsMap() {
		pf.Cardinality = KindMap
		pf.MapKeyKind = kindToString(fd.MapKey().Kind())
		mapVal := fd.MapValue()
		pf.MapValKind = kindToString(mapVal.Kind())
		if mapVal.Kind() == protoreflect.MessageKind {
			nested, err := extractMessageFromDescriptor(mapVal.Message(), file, path, descs, depth+1, rootPkg)
			if err != nil {
				return ProtoField{}, err
			}
			pf.Nested = nested
		}
		return pf, nil
	}
	if fd.IsList() {
		pf.Cardinality = KindRepeated
		pf.Kind = kindToString(fd.Kind())
		if fd.Kind() == protoreflect.MessageKind {
			nested, err := extractMessageFromDescriptor(fd.Message(), file, path, descs, depth+1, rootPkg)
			if err != nil {
				return ProtoField{}, err
			}
			pf.Nested = nested
		}
		return pf, nil
	}

	pf.IsOptional = fd.HasOptionalKeyword()

	if oo := fd.ContainingOneof(); oo != nil && !oo.IsSynthetic() {
		pf.OneofName = string(oo.Name())
	}

	switch fd.Kind() {
	case protoreflect.MessageKind:
		fullName := string(fd.Message().FullName())
		if wk, ok := wellKnownProtoTypes[fullName]; ok {
			pf.Kind = wk

			if wk == "status" {
				pf.Nested = &ProtoMessage{
					Name:              "Status",
					GoName:            "Status",
					ExternalPkgAlias:  "status",
					ExternalPkgImport: "google.golang.org/genproto/googleapis/rpc/status",
					Fields: []ProtoField{
						{Name: "code", Kind: KindInt32, Cardinality: "singular"},
						{Name: "message", Kind: KindString, Cardinality: "singular"},
					},
				}
			}
			if isScalarWrapperKind(wk) {
				pf.IsOptional = true
				pf.IsScalarWrapper = true
			}
		} else {
			pf.Kind = KindMessage
			nested, err := extractMessageFromDescriptor(fd.Message(), file, path, descs, depth+1, rootPkg)
			if err != nil {
				return ProtoField{}, err
			}
			pf.Nested = nested
		}
	case protoreflect.EnumKind:
		pf.Kind = "enum"
		pf.EnumName = string(fd.Enum().FullName())
		pf.EnumGoName = enumGoName(fd.Enum())
		pf.EnumProtoPkg = string(fd.Enum().ParentFile().Package())

		vals := fd.Enum().Values()
		for j := 0; j < vals.Len(); j++ {
			pf.EnumValues = append(pf.EnumValues, string(vals.Get(j).Name()))
		}
	default:
		pf.Kind = kindToString(fd.Kind())
	}

	return pf, nil
}

func extractFieldRules(opts proto.Message) (*bufvalidate.FieldRules, error) {
	if opts == nil {
		return nil, nil
	}
	var extMsg proto.Message
	opts.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if fd.IsExtension() && string(fd.FullName()) == "buf.validate.field" {
			extMsg = v.Message().Interface()
			return false
		}
		return true
	})
	if extMsg == nil {
		return nil, nil
	}
	b, err := proto.Marshal(extMsg)
	if err != nil {
		return nil, fmt.Errorf("proto validate-rule extraction: marshal: %w", err)
	}
	rules := &bufvalidate.FieldRules{}
	if err := proto.Unmarshal(b, rules); err != nil {
		return nil, fmt.Errorf("proto validate-rule extraction: unmarshal: %w", err)
	}
	return rules, nil
}

var wellKnownProtoTypes = map[string]string{
	"google.protobuf.Timestamp":   "timestamp",
	"google.protobuf.Duration":    "duration",
	"google.protobuf.Struct":      "json_struct",
	"google.rpc.Status":           "status",
	"google.protobuf.BoolValue":   KindBool,
	"google.protobuf.StringValue": KindString,
	"google.protobuf.Int32Value":  KindInt32,
	"google.protobuf.Int64Value":  KindInt64,
	"google.protobuf.UInt32Value": KindInt64,
	"google.protobuf.UInt64Value": KindInt64,
	"google.protobuf.FloatValue":  KindFloat,
	"google.protobuf.DoubleValue": KindDouble,
}

func isScalarWrapperKind(k string) bool {
	switch k {
	case KindBool, KindString, KindInt32, KindInt64, KindFloat, KindDouble:
		return true
	default:
		return false
	}
}

// ExternalProtoPackage maps a proto package to its generated Go import. Shared
// by the schema/model walker and cmd/enumgen as the single source of truth.
type ExternalProtoPackage struct {
	GoImport       string
	Alias          string
	FunctionPrefix string
}

// ExternalProtoPackages is the canonical proto-package → Go-import registry.
var ExternalProtoPackages = map[string]ExternalProtoPackage{
	"redpanda.core.admin.v2":       {GoImport: "buf.build/gen/go/redpandadata/core/protocolbuffers/go/redpanda/core/admin/v2", Alias: "corev2"},
	"redpanda.core.common.v1":      {GoImport: "buf.build/gen/go/redpandadata/core/protocolbuffers/go/redpanda/core/common/v1", Alias: "commonv1", FunctionPrefix: "Common"},
	"redpanda.api.dataplane.v1":    {GoImport: "buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go/redpanda/api/dataplane/v1", Alias: "dataplanev1"},
	"redpanda.api.controlplane.v1": {GoImport: "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1", Alias: "controlplanev1"},
}

func kindToString(k protoreflect.Kind) string {
	switch k {
	case protoreflect.StringKind:
		return KindString
	case protoreflect.BoolKind:
		return KindBool
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return KindInt32
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind,
		protoreflect.Uint32Kind, protoreflect.Uint64Kind, protoreflect.Fixed32Kind, protoreflect.Fixed64Kind:
		return KindInt64
	case protoreflect.FloatKind:
		return "float"
	case protoreflect.DoubleKind:
		return "double"
	case protoreflect.BytesKind:
		return "bytes"
	case protoreflect.EnumKind:
		return protoKindEnum
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return KindMessage
	default:
		return KindString
	}
}

func cleanComment(s string) string {
	lines := strings.Split(s, "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	result := strings.Join(cleaned, " ")
	result = strings.TrimSuffix(result, ".")
	return result
}

func messageGoName(m protoreflect.MessageDescriptor) string {
	parts := []string{string(m.Name())}
	parent := m.Parent()
	for parent != nil {
		md, ok := parent.(protoreflect.MessageDescriptor)
		if !ok {
			break
		}
		parts = append([]string{string(md.Name())}, parts...)
		parent = md.Parent()
	}
	return strings.Join(parts, "_")
}

func enumGoName(e protoreflect.EnumDescriptor) string {
	parts := []string{string(e.Name())}
	parent := e.Parent()
	for parent != nil {
		md, ok := parent.(protoreflect.MessageDescriptor)
		if !ok {
			break
		}
		parts = append([]string{string(md.Name())}, parts...)
		parent = md.Parent()
	}
	return strings.Join(parts, "_")
}

func findMessage(msgs protoreflect.MessageDescriptors, name string) protoreflect.MessageDescriptor {
	for i := 0; i < msgs.Len(); i++ {
		if string(msgs.Get(i).Name()) == name {
			return msgs.Get(i)
		}
	}

	parts := strings.SplitN(name, "_", 2)
	if len(parts) == 2 {
		for i := 0; i < msgs.Len(); i++ {
			if string(msgs.Get(i).Name()) == parts[0] {
				return findMessage(msgs.Get(i).Messages(), parts[1])
			}
		}
	}

	return nil
}

func findProtoFiles(importPaths []string, protoPkg string) []string {
	for _, root := range importPaths {
		dir := filepath.Join(root, protoPkg)
		entries, err := filepath.Glob(filepath.Join(dir, "*.proto"))
		if err != nil || len(entries) == 0 {
			continue
		}
		var relPaths []string
		for _, e := range entries {
			relPaths = append(relPaths, filepath.Join(protoPkg, filepath.Base(e)))
		}
		return relPaths
	}
	return nil
}
