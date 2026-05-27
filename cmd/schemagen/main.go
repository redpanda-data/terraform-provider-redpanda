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
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/redpanda-data/terraform-provider-redpanda/internal/apidesc"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/bufdeps"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/schemagen"
)

func main() {
	var (
		cloudv2      = flag.String("cloudv2", "", "Path to cloudv2 repo root (or set CLOUDV2_ROOT env var)")
		console      = flag.String("console", "", "Path to console repo root (or set CONSOLE_ROOT env var). Only needed for -update-deps.")
		protoPkg     = flag.String("proto-pkg", "", "Proto package path (e.g. redpanda/api/controlplane/v1)")
		message      = flag.String("message", "", "Name of the proto message (e.g. Cluster)")
		configPath   = flag.String("config", "", "Path to YAML schema config file")
		funcName     = flag.String("func", "", "Name of the generated schema function")
		schemaType   = flag.String("type", "resource", "Schema type: 'resource' or 'datasource'")
		output       = flag.String("output", "", "Output file path")
		pkgName      = flag.String("package", "", "Package name (derived from output path if not set)")
		todoFlag     = flag.Bool("todo", false, "Add uncovered proto fields as todo: true entries in the YAML config")
		apiDescPath  = flag.String("api-descriptions", "", "Path to apidescriptions.yaml (default: auto-detected relative to go.mod)")
		modelOutput  = flag.String("model-output", "", "If set, also write the resource/data-source model struct to this path")
		modelPackage = flag.String("model-package", "", "Package name for the model file (derived from -model-output dir if unset)")
		convOutput   = flag.String("conv-output", "", "If set, also write generated flatten/expand functions to this path. Requires schema.yaml api: block.")
		protoImport  = flag.String("proto-import", "", "Go import path for the proto package containing the request/response types (required with -conv-output)")
		protoAlias   = flag.String("proto-alias", "", "Go import alias for the proto package (defaults to last segment of -proto-import)")
		updateDeps   = flag.Bool("update-deps", false, "Rewrite internal/buf_dependencies.yaml from current cloudv2/console HEAD + go.mod, then exit without generating any code")
	)
	flag.Parse()

	if os.Getenv("SCHEMAGEN_TODO") == "1" {
		*todoFlag = true
	}

	if *updateDeps {
		if err := runUpdateDeps(*cloudv2, *console); err != nil {
			log.Fatalf("schemagen -update-deps: %v", err)
		}
		return
	}

	cloudv2Root := resolveCloudv2Root(*cloudv2)
	if cloudv2Root == "" {
		log.Fatal("schemagen: cloudv2 repo not found — set -cloudv2 flag, CLOUDV2_ROOT env, or place cloudv2 as a sibling directory")
	}

	if err := assertPinnedCheckout(cloudv2Root); err != nil {
		log.Fatalf("schemagen: %v", err)
	}

	var extraImportPaths []string
	if path := resolveConsoleProtoPath(cloudv2Root); path != "" {
		extraImportPaths = append(extraImportPaths, path)
	}

	if err := run(cloudv2Root, *protoPkg, *message, *configPath, *funcName, *schemaType, *output, *pkgName, *apiDescPath, *modelOutput, *modelPackage, *convOutput, *protoImport, *protoAlias, *todoFlag, extraImportPaths); err != nil {
		log.Fatalf("schemagen: %v", err)
	}
}

// assertPinnedCheckout fails the per-resource codegen run when the local
// cloudv2 checkout has drifted from internal/buf_dependencies.yaml. Console
// is consumed through the .build/console-protos buf-export tree so its git
// SHA is checked only by cmd/apidesc-import.
func assertPinnedCheckout(cloudv2Root string) error {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return fmt.Errorf("resolve repo root: %w", err)
	}
	deps, err := bufdeps.Read(bufdeps.DefaultPath(repoRoot))
	if err != nil {
		return fmt.Errorf("read pin file: %w", err)
	}
	return bufdeps.AssertCheckoutAt(cloudv2Root, deps.Cloudv2.SHA, "cloudv2")
}

// runUpdateDeps rewrites internal/buf_dependencies.yaml from current state.
// Reads HEAD of cloudv2 and console git checkouts plus the BSR commit IDs
// embedded in go.mod's buf-gen pseudo-versions, then writes everything to
// the pin file. Exits without doing any code generation.
func runUpdateDeps(cloudv2Flag, consoleFlag string) error {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return fmt.Errorf("resolve repo root: %w", err)
	}
	cloudv2Root := resolveCloudv2Root(cloudv2Flag)
	if cloudv2Root == "" {
		return errors.New("cloudv2 repo not found — set -cloudv2 flag, CLOUDV2_ROOT env, or place cloudv2 as a sibling directory")
	}
	consoleRoot := resolveConsoleRoot(consoleFlag, cloudv2Root)
	if consoleRoot == "" {
		return errors.New("console repo not found — set -console flag, CONSOLE_ROOT env, or place console as a sibling directory")
	}

	cloudSHA, err := bufdeps.HeadSHA(cloudv2Root)
	if err != nil {
		return fmt.Errorf("cloudv2: %w", err)
	}
	consoleSHA, err := bufdeps.HeadSHA(consoleRoot)
	if err != nil {
		return fmt.Errorf("console: %w", err)
	}
	modules, err := bufdeps.LoadBufModulesFromGoMod(filepath.Join(repoRoot, "go.mod"))
	if err != nil {
		return fmt.Errorf("buf-gen modules: %w", err)
	}

	// Preserve Remote URLs from existing pin if present; otherwise default.
	prev, _ := bufdeps.Read(bufdeps.DefaultPath(repoRoot))
	cloud := bufdeps.Source{Remote: "github.com/redpanda-data/cloudv2", SHA: cloudSHA}
	console := bufdeps.Source{Remote: "github.com/redpanda-data/console", SHA: consoleSHA}
	if prev != nil {
		if prev.Cloudv2.Remote != "" {
			cloud.Remote = prev.Cloudv2.Remote
		}
		if prev.Console.Remote != "" {
			console.Remote = prev.Console.Remote
		}
	}

	deps := &bufdeps.Deps{Cloudv2: cloud, Console: console, BufModules: modules}
	if err := bufdeps.Write(bufdeps.DefaultPath(repoRoot), deps); err != nil {
		return err
	}
	log.Printf("wrote %s: cloudv2=%s console=%s, %d buf modules",
		bufdeps.RelPath, shortSHA(cloudSHA), shortSHA(consoleSHA), len(modules))
	return nil
}

func shortSHA(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

func run(cloudv2Root, protoPkg, messageName, configPath, funcName, schemaType, output, pkgName, apiDescPath, modelOutput, modelPkg, convOutput, protoImport, protoAlias string, todoMode bool, extraImportPaths []string) error {
	if protoPkg == "" {
		return errors.New("-proto-pkg is required")
	}
	if messageName == "" {
		return errors.New("-message is required")
	}
	if configPath == "" {
		return errors.New("-config is required")
	}
	if funcName == "" {
		return errors.New("-func is required")
	}
	if output == "" {
		return errors.New("-output is required")
	}
	if schemaType != "resource" && schemaType != schemagen.SchemaTypeDatasource {
		return fmt.Errorf("-type must be 'resource' or 'datasource', got %q", schemaType)
	}

	if pkgName == "" {
		absOutput, err := filepath.Abs(output)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %w", err)
		}
		pkgName = filepath.Base(filepath.Dir(absOutput))
	}

	log.Printf("Compiling proto %s.%s from %s", protoPkg, messageName, cloudv2Root)
	files, err := schemagen.CompileProtoFiles(cloudv2Root, protoPkg, extraImportPaths)
	if err != nil {
		return fmt.Errorf("failed to compile proto: %w", err)
	}
	proto, _, err := schemagen.ExtractMessage(files, messageName, protoPkg)
	if err != nil {
		return fmt.Errorf("failed to extract proto message: %w", err)
	}
	log.Printf("Extracted %d fields from %s", len(proto.Fields), proto.Name)
	protoLookup := func(name string) (*schemagen.ProtoMessage, error) {
		msg, _, err := schemagen.ExtractMessage(files, name, protoPkg)
		return msg, err
	}

	log.Printf("Loading config from %s", configPath)
	cfg, err := schemagen.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	log.Printf("Config has %d field overrides", len(cfg.Fields))

	uncovered := schemagen.FindUncoveredFields(proto, cfg)
	if todoMode {
		goldenPath := filepath.Join("testdata", pkgName+"_"+schemaType+"_schema.golden")
		accepted, gerr := schemagen.ParseGoldenPaths(goldenPath)
		switch {
		case gerr != nil:
			log.Printf("WARNING: could not load golden %s: %v — marking all uncovered fields", goldenPath, gerr)
		case accepted == nil:
			log.Printf("No golden found at %s — marking all uncovered fields (bootstrap)", goldenPath)
		default:
			before := len(uncovered)
			uncovered = schemagen.FilterUncoveredByGolden(uncovered, accepted)
			log.Printf("Golden %s has %d accepted paths; filtered %d uncovered → %d truly-new", goldenPath, len(accepted), before, len(uncovered))
		}
	}
	if len(uncovered) > 0 {
		if todoMode {
			log.Printf("Adding %d uncovered proto fields as todo entries to %s", len(uncovered), configPath)
			if err := schemagen.WriteTodos(configPath, uncovered); err != nil {
				return fmt.Errorf("failed to write todos: %w", err)
			}
			cfg, err = schemagen.LoadConfig(configPath)
			if err != nil {
				return fmt.Errorf("failed to reload config after adding todos: %w", err)
			}
		} else {
			for _, uf := range uncovered {
				fmt.Fprintf(os.Stderr, "WARNING: proto field %q has no config entry — run with -todo to add it\n", uf.Path)
			}
		}
	}

	apiIndex, err := loadAPIDescriptions(apiDescPath)
	if err != nil {
		return fmt.Errorf("failed to load api descriptions: %w", err)
	}
	if apiIndex != nil {
		log.Printf("Loaded %d api description entries from %s", apiIndex.Len(), apiIndex.Source)
	}

	attrs, extraImports, stats, mergeErrs := schemagen.Merge(proto, cfg, schemaType, apiIndex)
	if len(mergeErrs) > 0 {
		for _, e := range mergeErrs {
			fmt.Fprintf(os.Stderr, "schemagen: merge error: %v\n", e)
		}
		return fmt.Errorf("schemagen: %d merge error(s) — yaml references fields that no longer exist on the proto, or names a missing validator", len(mergeErrs))
	}
	log.Printf("Merged into %d attributes", len(attrs))
	if apiIndex != nil && cfg.APISchema != "" && stats.Attempted > 0 {
		pct := 100 * stats.Matched / stats.Attempted
		log.Printf("apidesc: %d/%d attrs matched from %s (%d%%)",
			stats.Matched, stats.Attempted, cfg.APISchema, pct)
		if stats.Matched == 0 {
			fmt.Fprintf(os.Stderr,
				"WARNING: api_schema=%q produced 0 description matches — check that the schema name exists in %s\n",
				cfg.APISchema, apiIndex.Source)
		}
	}

	hasTimeouts := len(cfg.Timeouts) > 0
	needsContext := hasTimeouts

	emitProtoValidator := schemaType != schemagen.SchemaTypeDatasource

	if err := schemagen.ApplyAPIDefaults(cfg, schemaType); err != nil {
		return fmt.Errorf("apply api defaults: %w", err)
	}

	imports := schemagen.ResolveImports(attrs, schemaType, hasTimeouts, extraImports)

	desc := cfg.Message
	if desc == "" {
		name := strings.ToUpper(pkgName[:1]) + pkgName[1:]
		if schemaType == schemagen.SchemaTypeDatasource {
			desc = name + " data source"
		} else {
			desc = name + " resource"
		}
	}

	data := schemagen.TemplateData{
		License:      schemagen.LicenseHeader(),
		Package:      pkgName,
		FuncName:     funcName,
		SchemaType:   schemaType,
		Description:  desc,
		NeedsContext: needsContext,
		Imports:      imports,
		Attributes:   attrs,
		HasTimeouts:  hasTimeouts,
		IsDatasource: schemaType == schemagen.SchemaTypeDatasource,
		Version:      cfg.Version,
	}
	for _, t := range cfg.Timeouts {
		switch strings.ToLower(t) {
		case "create":
			data.TimeoutsCreate = true
		case "read":
			data.TimeoutsRead = true
		case "update":
			data.TimeoutsUpdate = true
		case "delete":
			data.TimeoutsDelete = true
		default:
			return fmt.Errorf("unknown timeout %q", t)
		}
	}

	src, err := schemagen.Generate(data)
	if err != nil {
		return fmt.Errorf("code generation failed: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(output), 0o750); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	if err := os.WriteFile(output, src, 0o600); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	log.Printf("Generated %s (%d bytes)", output, len(src))

	if emitProtoValidator {
		if modelOutput == "" {
			return errors.New("proto validator emission requires -model-output so the validator can reference the model package")
		}
		absModel, err := filepath.Abs(modelOutput)
		if err != nil {
			return fmt.Errorf("resolve model output abs: %w", err)
		}
		modelImport, err := goImportPathForFile(absModel)
		if err != nil {
			return fmt.Errorf("derive model import path: %w", err)
		}
		modelAlias := pkgName + "model"
		validatorPath := filepath.Join(filepath.Dir(output), "proto_validator_gen.go")
		valSrc, err := schemagen.GenerateProtoValidator(schemagen.ProtoValidatorData{
			License:         schemagen.LicenseHeader(),
			Package:         pkgName,
			ResourceType:    cfg.API.ResourceType,
			ModelImport:     modelImport,
			ModelAlias:      modelAlias,
			ModelType:       schemagen.ModelTypeResource,
			ExpandFunc:      "ExpandCreate",
			ResourceLabel:   pkgName,
			PreValidateHook: cfg.API.PreValidateHook,
			PostExpandHook:  cfg.API.PostExpandHook,
			SkippedFields:   schemagen.CollectSkippedValidationFields(cfg),
		})
		if err != nil {
			return fmt.Errorf("proto validator generation: %w", err)
		}
		if err := os.WriteFile(validatorPath, valSrc, 0o600); err != nil {
			return fmt.Errorf("write proto validator: %w", err)
		}
		log.Printf("Generated %s (%d bytes)", validatorPath, len(valSrc))
	}

	if modelOutput != "" {
		resolvedModelPkg := modelPkg
		if resolvedModelPkg == "" {
			absModel, err := filepath.Abs(modelOutput)
			if err != nil {
				return fmt.Errorf("resolve model output path: %w", err)
			}
			resolvedModelPkg = filepath.Base(filepath.Dir(absModel))
		}
		modelSrc, err := schemagen.GenerateModel(attrs, hasTimeouts, schemaType, resolvedModelPkg, resolvedModelPkg)
		if err != nil {
			return fmt.Errorf("model generation failed: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(modelOutput), 0o750); err != nil {
			return fmt.Errorf("create model output directory: %w", err)
		}
		if err := os.WriteFile(modelOutput, modelSrc, 0o600); err != nil {
			return fmt.Errorf("write model output: %w", err)
		}
		log.Printf("Generated %s (%d bytes)", modelOutput, len(modelSrc))
	}

	if convOutput != "" {
		if protoImport == "" {
			return errors.New("-conv-output requires -proto-import")
		}
		alias := protoAlias
		if alias == "" {
			alias = filepath.Base(protoImport)
		}
		convPkg := modelPkg
		if convPkg == "" && modelOutput != "" {
			absModel, err := filepath.Abs(modelOutput)
			if err != nil {
				return fmt.Errorf("resolve model output abs: %w", err)
			}
			convPkg = filepath.Base(filepath.Dir(absModel))
		}
		if convPkg == "" {
			absConv, err := filepath.Abs(convOutput)
			if err != nil {
				return fmt.Errorf("resolve conv output abs: %w", err)
			}
			convPkg = filepath.Base(filepath.Dir(absConv))
		}
		conv, err := schemagen.PlanFlattenExpand(attrs, cfg, proto, convPkg, alias, protoImport, schemaType, protoLookup)
		if err != nil {
			return fmt.Errorf("plan flatten/expand: %w", err)
		}
		convSrc, err := schemagen.GenerateFlattenExpand(conv)
		if err != nil {
			return fmt.Errorf("flatten/expand generation: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(convOutput), 0o750); err != nil {
			return fmt.Errorf("create conv output directory: %w", err)
		}
		if err := os.WriteFile(convOutput, convSrc, 0o600); err != nil {
			return fmt.Errorf("write conv output: %w", err)
		}
		log.Printf("Generated %s (%d bytes)", convOutput, len(convSrc))
	}

	return nil
}

func goImportPathForFile(absPath string) (string, error) {
	dir := filepath.Dir(absPath)
	cur := dir
	for {
		goModPath := filepath.Join(cur, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			data, err := os.ReadFile(filepath.Clean(goModPath))
			if err != nil {
				return "", fmt.Errorf("read %s: %w", goModPath, err)
			}
			module := parseModulePath(string(data))
			if module == "" {
				return "", fmt.Errorf("no module directive in %s", goModPath)
			}
			rel, err := filepath.Rel(cur, dir)
			if err != nil {
				return "", fmt.Errorf("relative path: %w", err)
			}
			if rel == "." {
				return module, nil
			}
			return module + "/" + filepath.ToSlash(rel), nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", errors.New("go.mod not found walking up from file")
		}
		cur = parent
	}
}

func parseModulePath(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module"))
		}
	}
	return ""
}

func loadAPIDescriptions(apiDescPath string) (*apidesc.Index, error) {
	path := apiDescPath
	if path == "" {
		root, err := findRepoRoot()
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: cannot locate repo root for api descriptions: %v\n", err)
			return nil, nil
		}
		path = filepath.Join(root, "internal", "schemagen", "data", "apidescriptions.yaml")
	}
	idx, err := apidesc.Load(path)
	if err != nil {
		return nil, err
	}
	if idx == nil {
		fmt.Fprintf(os.Stderr, "WARNING: api descriptions file not found at %s — descriptions will fall back to mechanical defaults\n", path)
	}
	return idx, nil
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("go.mod not found walking up from cwd")
		}
		dir = parent
	}
}

func resolveCloudv2Root(flagValue string) string {
	if flagValue != "" {
		if _, err := os.Stat(filepath.Join(flagValue, "proto", "public", "cloud")); err == nil {
			return flagValue
		}
		log.Printf("WARNING: -cloudv2 %s does not contain proto/public/cloud", flagValue)
	}

	if envRoot := os.Getenv("CLOUDV2_ROOT"); envRoot != "" {
		if _, err := os.Stat(filepath.Join(envRoot, "proto", "public", "cloud")); err == nil {
			log.Printf("Using cloudv2 from CLOUDV2_ROOT: %s", envRoot)
			return envRoot
		}
	}

	for _, rel := range []string{"../cloudv2", "../../cloudv2", "../../../cloudv2"} {
		if _, err := os.Stat(filepath.Join(rel, "proto", "public", "cloud")); err == nil {
			log.Printf("Using cloudv2 from relative path: %s", rel)
			return rel
		}
	}

	return ""
}

// resolveConsoleRoot locates the local console git checkout. Only used by
// -update-deps; the regular codegen path doesn't need console's git tree
// (it reads the buf-exported tree at .build/console-protos).
func resolveConsoleRoot(flagValue, cloudv2Root string) string {
	candidates := []string{flagValue, os.Getenv("CONSOLE_ROOT")}
	if cloudv2Root != "" {
		candidates = append(candidates, filepath.Join(filepath.Dir(cloudv2Root), "console"))
	}
	candidates = append(candidates, "../console", "../../console", "../../../console")
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(c, ".git")); err == nil {
			return c
		}
	}
	return ""
}

func resolveConsoleProtoPath(cloudv2Root string) string {
	if repoRoot, err := findRepoRoot(); err == nil {
		exportDir := filepath.Join(repoRoot, ".build", "console-protos")
		if _, statErr := os.Stat(exportDir); statErr == nil {
			log.Printf("Using console protos from buf export: %s", exportDir)
			return exportDir
		}
	}
	parent := filepath.Dir(cloudv2Root)
	consoleProto := filepath.Join(parent, "console", "proto")
	if _, err := os.Stat(consoleProto); err == nil {
		log.Printf("Using console protos from working tree: %s (run `task generate:console-protos` for buf-resolved tree)", consoleProto)
		return consoleProto
	}
	return ""
}
