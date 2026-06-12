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

package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/redpanda-data/terraform-provider-redpanda/internal/cmdutil"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/schemagen"
	"google.golang.org/protobuf/reflect/protoreflect"
	"gopkg.in/yaml.v3"
)

var protoPackages = []string{
	"redpanda/api/controlplane/v1",
	"redpanda/api/dataplane/v1",
	"redpanda/core/admin/v2",
	"redpanda/core/common/v1",
}

type enumInfo struct {
	GoName          string
	ProtoPkg        string
	GoImportPath    string
	GoImportAlias   string
	FunctionPrefix  string
	Prefix          string
	UnspecifiedName string
	ValueNames      []string
}

type codegenConfig struct {
	Exclude       []string       `yaml:"exclude"`
	EnumCarveouts []enumCarveout `yaml:"enum_carveouts"`
}

type enumCarveout struct {
	Enum   string `yaml:"enum"`
	Reason string `yaml:"reason"`
}

func main() {
	var (
		cloudv2     = flag.String("cloudv2", "", "Path to cloudv2 repo (or set CLOUDV2_ROOT)")
		out         = flag.String("out", "redpanda/utils/enums/enums_gen.go", "Output path for generated mapper file")
		codegenPath = flag.String("codegen", "redpanda/resources/codegen.yaml", "Path to codegen.yaml")
		handrolled  = flag.String("handrolled", "redpanda/utils/enums/handrolled.go", "Path to handrolled.go (for carve-out parity check)")
	)
	flag.Parse()

	cloudv2Root := cmdutil.ResolveCloudv2Root(*cloudv2)
	if cloudv2Root == "" {
		log.Fatal("enumgen: cloudv2 repo not found — set -cloudv2 flag or CLOUDV2_ROOT env var")
	}
	if err := cmdutil.AssertCloudv2Pinned(cloudv2Root); err != nil {
		log.Fatalf("enumgen: %v", err)
	}
	var extraImportPaths []string
	if consoleProtoPath := resolveConsoleProtoPath(cloudv2Root); consoleProtoPath != "" {
		extraImportPaths = append(extraImportPaths, consoleProtoPath)
	}

	cfg, err := loadCodegenConfig(*codegenPath)
	if err != nil {
		log.Fatalf("enumgen: load codegen config: %v", err)
	}

	carveoutSet := make(map[string]string, len(cfg.EnumCarveouts))
	for _, c := range cfg.EnumCarveouts {
		carveoutSet[funcNameRoot(c.Enum)] = c.Reason
	}

	if err := verifyCarveoutParity(*handrolled, carveoutSet); err != nil {
		log.Fatalf("enumgen: %v", err)
	}

	enums := make(map[string]enumInfo)
	for _, pkg := range protoPackages {
		log.Printf("enumgen: walking %s", pkg)
		files, err := schemagen.CompileProtoFiles(cloudv2Root, pkg, extraImportPaths)
		if err != nil {
			log.Fatalf("enumgen: compile %s: %v", pkg, err)
		}
		for _, f := range files {
			walkEnums(f, enums, carveoutSet)
		}
	}

	if err := emitFile(*out, enums); err != nil {
		log.Fatalf("enumgen: emit: %v", err)
	}
	log.Printf("enumgen: wrote %d enum mappers to %s", len(enums), *out)
}

func walkEnums(f protoreflect.FileDescriptor, enums map[string]enumInfo, carveouts map[string]string) {
	walkEnumsIn(f.Enums(), string(f.Package()), enums, carveouts)
	walkMessagesForEnums(f.Messages(), string(f.Package()), enums, carveouts)
}

func walkMessagesForEnums(msgs protoreflect.MessageDescriptors, pkg string, enums map[string]enumInfo, carveouts map[string]string) {
	for i := 0; i < msgs.Len(); i++ {
		m := msgs.Get(i)
		walkEnumsIn(m.Enums(), pkg, enums, carveouts)
		walkMessagesForEnums(m.Messages(), pkg, enums, carveouts)
	}
}

func walkEnumsIn(es protoreflect.EnumDescriptors, pkg string, enums map[string]enumInfo, carveouts map[string]string) {
	for i := 0; i < es.Len(); i++ {
		e := es.Get(i)
		goName := enumGoName(e)
		info := buildEnumInfo(e, pkg)
		if info.GoImportPath == "" {
			log.Printf("enumgen: skipping enum %s in unknown proto package %q", goName, pkg)
			continue
		}

		fnName := info.FunctionPrefix + funcNameRoot(goName)
		if _, ok := carveouts[fnName]; ok {
			continue
		}

		key := fmt.Sprintf("%s.%s", info.GoImportAlias, goName)
		if _, ok := enums[key]; ok {
			continue
		}
		enums[key] = info
	}
}

func buildEnumInfo(e protoreflect.EnumDescriptor, pkg string) enumInfo {
	info := enumInfo{
		GoName:   enumGoName(e),
		ProtoPkg: pkg,
	}
	if gi, ok := schemagen.ExternalProtoPackages[pkg]; ok {
		info.GoImportPath = gi.GoImport
		info.GoImportAlias = gi.Alias
		info.FunctionPrefix = gi.FunctionPrefix
	}
	vals := e.Values()
	for i := 0; i < vals.Len(); i++ {
		info.ValueNames = append(info.ValueNames, string(vals.Get(i).Name()))
	}
	info.Prefix = longestCommonPrefix(info.ValueNames)
	for _, n := range info.ValueNames {
		if strings.HasSuffix(n, "_UNSPECIFIED") {
			info.UnspecifiedName = n
			break
		}
	}
	if info.UnspecifiedName == "" && len(info.ValueNames) > 0 {
		info.UnspecifiedName = info.ValueNames[0]
	}
	return info
}

func funcNameRoot(goName string) string {
	return strings.ReplaceAll(goName, "_", "")
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

func longestCommonPrefix(values []string) string {
	if len(values) == 0 {
		return ""
	}
	prefix := values[0]
	for _, v := range values[1:] {
		for !strings.HasPrefix(v, prefix) {
			prefix = prefix[:len(prefix)-1]
			if prefix == "" {
				return ""
			}
		}
	}

	if i := strings.LastIndex(prefix, "_"); i >= 0 {
		return prefix[:i+1]
	}
	return prefix
}

func emitFile(path string, enums map[string]enumInfo) error {
	keys := make([]string, 0, len(enums))
	for k := range enums {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	prefFn := make(map[string]string, len(keys))
	for _, k := range keys {
		e := enums[k]
		prefFn[k] = e.FunctionPrefix + funcNameRoot(e.GoName)
	}

	byFnName := make(map[string][]string)
	for _, k := range keys {
		byFnName[prefFn[k]] = append(byFnName[prefFn[k]], k)
	}

	resolvedFn := make(map[string]string, len(keys))
	for _, k := range keys {
		resolvedFn[k] = prefFn[k]
	}
	for fn, ks := range byFnName {
		if len(ks) <= 1 {
			continue
		}

		anyResolved := false
		for _, k := range ks {
			e := enums[k]
			if strings.Contains(e.GoName, "_") {
				resolvedFn[k] = e.FunctionPrefix + e.GoName
				anyResolved = true
			}
		}
		if !anyResolved {
			return fmt.Errorf("enumgen residual collision on %q across %s — all colliders have unsplittable Go names; add a carve-out or codegen.yaml skip",
				fn, strings.Join(ks, ", "))
		}
	}

	importSet := make(map[string]string)
	for _, k := range keys {
		e := enums[k]
		importSet[e.GoImportPath] = e.GoImportAlias
	}
	importPaths := make([]string, 0, len(importSet))
	for p := range importSet {
		importPaths = append(importPaths, p)
	}
	sort.Strings(importPaths)

	var b strings.Builder
	b.WriteString(`// Copyright 2023 Redpanda Data, Inc.
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

// Code generated by cmd/enumgen. DO NOT EDIT.

package enums

import (
	"strings"

`)
	for _, p := range importPaths {
		fmt.Fprintf(&b, "\t%s %q\n", importSet[p], p)
	}
	b.WriteString(")\n\n")

	for _, k := range keys {
		e := enums[k]
		typeRef := fmt.Sprintf("%s.%s", e.GoImportAlias, e.GoName)

		constParent := e.GoName
		if idx := strings.LastIndex(e.GoName, "_"); idx >= 0 {
			constParent = e.GoName[:idx]
		}
		unspecRef := fmt.Sprintf("%s.%s_%s", e.GoImportAlias, constParent, e.UnspecifiedName)
		fnName := resolvedFn[k]
		fmt.Fprintf(&b, "// %sToString maps a proto enum value to its TF string form.\n", fnName)
		fmt.Fprintf(&b, "func %sToString(e %s) string {\n", fnName, typeRef)
		if e.Prefix != "" {
			fmt.Fprintf(&b, "\treturn strings.TrimPrefix(e.String(), %q)\n", e.Prefix)
		} else {
			fmt.Fprintf(&b, "\treturn e.String()\n")
		}
		fmt.Fprintf(&b, "}\n\n")

		fmt.Fprintf(&b, "// StringTo%s maps a TF string back to the proto enum.\n", fnName)
		fmt.Fprintf(&b, "// Returns the UNSPECIFIED value for unknown inputs.\n")
		fmt.Fprintf(&b, "func StringTo%s(s string) %s {\n", fnName, typeRef)
		fmt.Fprintf(&b, "\tif v, ok := %s_value[%q+s]; ok {\n", typeRef, e.Prefix)
		fmt.Fprintf(&b, "\t\treturn %s(v)\n", typeRef)
		fmt.Fprintf(&b, "\t}\n")
		fmt.Fprintf(&b, "\treturn %s\n", unspecRef)
		fmt.Fprintf(&b, "}\n\n")
	}

	src, err := format.Source([]byte(b.String()))
	if err != nil {
		if writeErr := os.WriteFile(path, []byte(b.String()), 0o600); writeErr != nil {
			return fmt.Errorf("gofmt: %w (and writing unformatted output to %s for debugging also failed: %v)", err, path, writeErr)
		}
		return fmt.Errorf("gofmt: %w (wrote unformatted output to %s)", err, path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	if err := os.WriteFile(path, src, 0o600); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}

func loadCodegenConfig(path string) (*codegenConfig, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	var cfg codegenConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func verifyCarveoutParity(handrolledPath string, carveouts map[string]string) error {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, handrolledPath, nil, parser.SkipObjectResolution)
	if err != nil {
		return fmt.Errorf("parse %s: %w", handrolledPath, err)
	}

	have := make(map[string]struct{ toString, fromString bool })
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || !fn.Name.IsExported() {
			continue
		}
		name := fn.Name.Name
		switch {
		case strings.HasSuffix(name, "ToString"):
			enum := strings.TrimSuffix(name, "ToString")
			entry := have[enum]
			entry.toString = true
			have[enum] = entry
		case strings.HasPrefix(name, "StringTo"):
			enum := strings.TrimPrefix(name, "StringTo")
			entry := have[enum]
			entry.fromString = true
			have[enum] = entry
		}
	}

	for c := range carveouts {
		entry := have[c]
		if !entry.toString || !entry.fromString {
			return fmt.Errorf("carve-out %q missing %sToString or StringTo%s in %s",
				c, c, c, handrolledPath)
		}
	}

	for name, entry := range have {
		if !entry.toString || !entry.fromString {
			continue
		}
		if _, ok := carveouts[name]; !ok {
			return fmt.Errorf("handrolled.go has %sToString/StringTo%s but %q is not listed in codegen.yaml enum_carveouts",
				name, name, name)
		}
	}
	return nil
}

func resolveConsoleProtoPath(cloudv2Root string) string {
	if repoRoot, err := cmdutil.FindRepoRoot(); err == nil {
		exportDir := filepath.Join(repoRoot, ".build", "console-protos")
		if info, statErr := os.Stat(exportDir); statErr == nil && info.IsDir() {
			return exportDir
		}
	}
	candidates := []string{
		filepath.Join(filepath.Dir(cloudv2Root), "console", "proto"),
		"../console/proto",
		"../../console/proto",
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	return ""
}
