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
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/redpanda-data/terraform-provider-redpanda/internal/apidesc"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/bufdeps"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/cmdutil"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/fileutil"
	"gopkg.in/yaml.v3"
)

type source struct {
	name    string
	repo    string
	specDir string
	commit  string
}

type scopeInfo struct {
	before int
	roots  []string
}

func main() {
	var (
		repo           = flag.String("repo", "", "Path to local cloudv2 checkout (required unless -spec-dir is set)")
		specDir        = flag.String("spec-dir", "", "Directory containing cloudv2 openapi.*.yaml files (default: <repo>/proto/gen/openapi)")
		consoleRepo    = flag.String("console-repo", "", "Path to local console checkout (optional; falls back to CONSOLE_ROOT env or ../console)")
		consoleSpecDir = flag.String("console-spec-dir", "", "Directory containing console openapi.*.yaml files (default: <console-repo>/proto/gen/openapi)")
		output         = flag.String("output", "", "Output path (default: <terraform-provider-redpanda>/internal/schemagen/data/apidescriptions.yaml)")
		resourceDir    = flag.String("resource-dir", "", "Directory containing resource subdirs with schema*.yaml files (default: <repo>/redpanda/resources)")
		noFilter       = flag.Bool("no-filter", false, "Skip api_schema: scoping and emit every schema in the openapi specs (debugging only)")
		includes       multiFlag
		quiet          = flag.Bool("quiet", false, "Suppress per-file progress output")
	)
	flag.Var(&includes, "include", "Filename glob for spec files (may be repeated; default: openapi*.yaml; .prod.yaml variants preferred)")
	flag.Parse()

	if err := run(*repo, *specDir, *consoleRepo, *consoleSpecDir, *output, *resourceDir, *noFilter, includes, *quiet); err != nil {
		log.Fatalf("apidesc-import: %v", err)
	}
}

func run(cloudRepo, cloudSpecDir, consoleRepo, consoleSpecDir, output, resourceDirFlag string, noFilter bool, includes []string, quiet bool) error {
	cloud, err := resolveSource("cloudv2", cloudRepo, cloudSpecDir)
	if err != nil {
		return fmt.Errorf("cloudv2: %w", err)
	}
	if cloud == nil {
		return errors.New("-repo is required (or provide -spec-dir)")
	}
	sources := []*source{cloud}

	console, err := resolveConsoleSource(consoleRepo, consoleSpecDir, cloud.repo)
	if err != nil {
		return fmt.Errorf("console: %w", err)
	}
	if console != nil {
		sources = append(sources, console)
	} else {
		log.Print("console not found — proceeding with cloudv2 only (set -console-repo, CONSOLE_ROOT, or place console as sibling)")
	}

	repoRoot, err := cmdutil.FindRepoRoot()
	if err != nil {
		return fmt.Errorf("resolve repo root: %w", err)
	}

	if err := assertPinnedSHAs(repoRoot, cloud, console); err != nil {
		return err
	}

	outPath := output
	if outPath == "" {
		outPath = filepath.Join(repoRoot, "internal", "schemagen", "data", "apidescriptions.yaml")
	}

	resourceDir := resourceDirFlag
	if resourceDir == "" {
		resourceDir = filepath.Join(repoRoot, "redpanda", "resources")
	}

	specs := make(map[string]*apidesc.Spec)
	var fileList []string
	for _, src := range sources {
		files, err := selectSpecFiles(src.specDir, includes)
		if err != nil {
			return fmt.Errorf("%s: %w", src.name, err)
		}
		if len(files) == 0 {
			return fmt.Errorf("%s: no spec files found in %s", src.name, src.specDir)
		}
		sort.Strings(files)
		for _, f := range files {
			spec, err := apidesc.LoadSpec(f)
			if err != nil {
				return err
			}
			qualified := src.name + "/" + filepath.Base(f)
			specs[qualified] = spec
			fileList = append(fileList, qualified)
			if !quiet {
				log.Printf("loaded %s (%d schemas)", qualified, len(spec.Components.Schemas))
			}
		}
	}

	tree, warnings, err := apidesc.Flatten(specs)
	if err != nil {
		return err
	}
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "WARNING: %s\n", w)
	}

	var scope *scopeInfo
	if !noFilter {
		refs, err := collectAPISchemas(resourceDir)
		if err != nil {
			return fmt.Errorf("scan resource yamls: %w", err)
		}
		if len(refs) == 0 {
			return fmt.Errorf("found no api_schema: references in %s/*/schema*.yaml — refusing to write a full dump (use -no-filter to override)", resourceDir)
		}
		roots := sortedRefKeys(refs)
		for _, r := range roots {
			ref := refs[r]
			if _, ok := tree[r]; ok {
				continue
			}

			if ref.prefix != "" {
				if node, ok := tree[ref.prefix+r]; ok {
					tree[r] = node
					delete(tree, ref.prefix+r)
					continue
				}
			}
			return fmt.Errorf("%s references api_schema %q but it is absent from the openapi index; either fix the reference or add the schema to cloudv2/console", ref.files[0], r)
		}
		before := len(tree)
		tree = apidesc.FilterByRoots(tree, roots)
		log.Printf("scoped: %d -> %d top-level schemas (kept: %s)", before, len(tree), strings.Join(roots, ", "))
		scope = &scopeInfo{before: before, roots: roots}
	}

	schemaCount := len(tree)
	descCount := countDescriptions(tree)

	sort.Strings(fileList)
	header := buildHeader(sources, fileList, schemaCount, descCount, scope)

	data, err := apidesc.Encode(&apidesc.File{Schemas: tree}, header)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o750); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	if err := os.WriteFile(outPath, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}

	log.Printf("wrote %s (%d schemas, %d non-empty descriptions, %d bytes)",
		outPath, schemaCount, descCount, len(data))
	return nil
}

type apiSchemaRef struct {
	files  []string
	prefix string
}

func collectAPISchemas(resourceDir string) (map[string]*apiSchemaRef, error) {
	info, err := os.Stat(resourceDir)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", resourceDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", resourceDir)
	}

	pattern := filepath.Join(resourceDir, "*", "schema*.yaml")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob %q: %w", pattern, err)
	}
	sort.Strings(matches)

	out := make(map[string]*apiSchemaRef)
	for _, p := range matches {
		data, err := fileutil.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
		var header struct {
			APISchema          string   `yaml:"api_schema"`
			APIWriteSchemas    []string `yaml:"api_write_schemas"`
			StripOpenAPIPrefix string   `yaml:"strip_openapi_prefix"`
		}
		if err := yaml.Unmarshal(data, &header); err != nil {
			return nil, fmt.Errorf("parse %s: %w", p, err)
		}
		if header.APISchema == "" {
			continue
		}
		register := func(root string) error {
			ref, ok := out[root]
			if !ok {
				ref = &apiSchemaRef{prefix: header.StripOpenAPIPrefix}
				out[root] = ref
			}
			ref.files = append(ref.files, p)
			if ref.prefix == "" && header.StripOpenAPIPrefix != "" {
				ref.prefix = header.StripOpenAPIPrefix
			}
			if header.StripOpenAPIPrefix != "" && ref.prefix != "" && ref.prefix != header.StripOpenAPIPrefix {
				return fmt.Errorf("api_schema %q has conflicting strip_openapi_prefix values (%q vs %q)",
					root, ref.prefix, header.StripOpenAPIPrefix)
			}
			return nil
		}
		// Read shape (primary) plus any write-shape fallback roots.
		for _, root := range append([]string{header.APISchema}, header.APIWriteSchemas...) {
			if err := register(root); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

func sortedRefKeys(m map[string]*apiSchemaRef) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func resolveSource(name, repo, specDir string) (*source, error) {
	if repo == "" && specDir == "" {
		return nil, nil
	}
	dir := specDir
	if dir == "" {
		dir = filepath.Join(repo, "proto", "gen", "openapi")
	}
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("spec dir: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("spec dir %s is not a directory", dir)
	}
	return &source{
		name:    name,
		repo:    repo,
		specDir: dir,
		commit:  gitRevParse(repo),
	}, nil
}

func resolveConsoleSource(flagRepo, flagSpecDir, cloudRepo string) (*source, error) {
	if flagRepo != "" || flagSpecDir != "" {
		if !hasConsoleSpecDir(flagRepo, flagSpecDir) {
			log.Printf("console: -console-repo=%q not found, continuing without console", flagRepo)
			return nil, nil
		}
		return resolveSource("console", flagRepo, flagSpecDir)
	}

	if env := os.Getenv("CONSOLE_ROOT"); env != "" {
		src, err := resolveSource("console", env, "")
		if err != nil {
			log.Printf("CONSOLE_ROOT=%s invalid: %v", env, err)
		} else if src != nil {
			return src, nil
		}
	}

	if cloudRepo != "" {
		sibling := filepath.Join(filepath.Dir(cloudRepo), "console")
		if info, err := os.Stat(filepath.Join(sibling, "proto", "gen", "openapi")); err == nil && info.IsDir() {
			return resolveSource("console", sibling, "")
		}
	}

	return nil, nil
}

func selectSpecFiles(dir string, includes []string) ([]string, error) {
	if len(includes) == 0 {
		includes = []string{"openapi*.yaml"}
	}

	seen := make(map[string]bool)
	for _, pattern := range includes {
		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			return nil, fmt.Errorf("glob %q: %w", pattern, err)
		}
		for _, m := range matches {
			seen[m] = true
		}
	}

	for f := range seen {
		if strings.HasSuffix(f, ".prod.yaml") {
			continue
		}
		prod := strings.TrimSuffix(f, ".yaml") + ".prod.yaml"
		if seen[prod] {
			delete(seen, f)
		}
	}

	out := make([]string, 0, len(seen))
	for f := range seen {
		out = append(out, f)
	}
	return out, nil
}

func hasConsoleSpecDir(repo, specDir string) bool {
	dir := specDir
	if dir == "" {
		if repo == "" {
			return false
		}
		dir = filepath.Join(repo, "proto", "gen", "openapi")
	}
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}

func countDescriptions(tree map[string]*apidesc.Node) int {
	var n int
	for _, node := range tree {
		n += countNode(node)
	}
	return n
}

func countNode(node *apidesc.Node) int {
	if node == nil {
		return 0
	}
	n := 0
	if node.Description != "" {
		n = 1
	}
	for _, child := range node.Fields {
		n += countNode(child)
	}
	return n
}

// assertPinnedSHAs fails the run when local cloudv2 / console checkouts diverge
// from the SHAs recorded in internal/buf_dependencies.yaml. It also warns when
// go.mod's buf-gen module BSR commit IDs diverge from the pinned set.
// apidesc-import is a pure consumer of the pin file; bumps go through
// `task generate:bump-deps`.
func assertPinnedSHAs(repoRoot string, cloud, console *source) error {
	deps, err := bufdeps.Read(bufdeps.DefaultPath(repoRoot))
	if err != nil {
		return fmt.Errorf("read pin file: %w", err)
	}
	if cloud.repo != "" {
		if err := bufdeps.AssertCheckoutAt(cloud.repo, deps.Cloudv2.SHA, "cloudv2"); err != nil {
			return err
		}
	}
	if console != nil && console.repo != "" {
		if err := bufdeps.AssertCheckoutAt(console.repo, deps.Console.SHA, "console"); err != nil {
			return err
		}
	}
	current, err := bufdeps.LoadBufModulesFromGoMod(filepath.Join(repoRoot, "go.mod"))
	if err != nil {
		log.Printf("WARNING: could not read buf-gen modules from go.mod: %v", err)
		return nil
	}
	for _, msg := range bufdeps.CompareBufModules(deps.BufModules, current) {
		log.Printf("WARNING: %s", msg)
	}
	return nil
}

func gitRevParse(repo string) string {
	if repo == "" {
		return "unknown"
	}
	cmd := exec.CommandContext(context.Background(), "git", "-C", repo, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func buildHeader(sources []*source, files []string, schemaCount, descCount int, scope *scopeInfo) string {
	var b strings.Builder
	b.WriteString("# Code generated by cmd/apidesc-import. DO NOT EDIT.\n")
	b.WriteString("#\n")
	b.WriteString("# Sources:\n")
	for _, s := range sources {
		fmt.Fprintf(&b, "#   %s at %s\n", s.name, s.commit)
	}
	b.WriteString("# Files:\n")
	for _, f := range files {
		fmt.Fprintf(&b, "#   - %s\n", f)
	}
	fmt.Fprintf(&b, "# Schemas: %d  Descriptions: %d\n", schemaCount, descCount)
	if scope != nil {
		fmt.Fprintf(&b, "# Scoped: %d of %d roots from resource api_schema: refs (%s)\n",
			len(scope.roots), scope.before, strings.Join(scope.roots, ", "))
	}
	b.WriteString("#\n")
	b.WriteString("# Precedence layer: scopedDescriptions (descriptions.go) > this file > common/mechanical defaults\n")
	b.WriteString("# Regenerate: task generate:apidescriptions\n")
	b.WriteString("\n")
	return b.String()
}

type multiFlag []string

func (m *multiFlag) String() string     { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error { *m = append(*m, v); return nil }
