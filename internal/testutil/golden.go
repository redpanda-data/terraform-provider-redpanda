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

package testutil

import (
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/redpanda-data/terraform-provider-redpanda/internal/fileutil"
	"gopkg.in/yaml.v3"
)

// Update controls whether golden/description files are regenerated.
var Update = flag.Bool("update", false, "overwrite golden/description files with current schema output")

// Descriptions enables description comparison against .description files.
var Descriptions = flag.Bool("descriptions", false, "also compare field descriptions against .description files")

// AssertGolden compares actual against the contents of path. If -update is
// set, writes actual to path instead.
func AssertGolden(t *testing.T, path, actual string) {
	t.Helper()

	if *Update {
		_ = os.MkdirAll(filepath.Dir(path), 0o750)
		if err := os.WriteFile(path, []byte(actual), 0o600); err != nil {
			t.Fatalf("failed to write %s: %v", path, err)
		}
		t.Logf("updated %s", path)
		return
	}

	want, err := fileutil.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s (run with -update to create): %v", path, err)
	}

	if diff := cmp.Diff(string(want), actual); diff != "" {
		t.Errorf("mismatch in %s (-want +got):\n%s", filepath.Base(path), diff)
	}
}

// DumpSchema produces a deterministic YAML dump of a schema. Accepts both
// resource/schema.Schema and datasource/schema.Schema. Captures every
// structural detail except descriptions.
func DumpSchema(s any) string {
	sv := reflect.ValueOf(s)
	attrsField := sv.FieldByName("Attributes")
	if !attrsField.IsValid() || attrsField.Kind() != reflect.Map {
		return "error: no Attributes field found\n"
	}

	out := &yamlSchema{}

	for _, key := range attrsField.MapKeys() {
		name := key.String()
		if name == "timeouts" {
			out.HasTimeouts = true
			continue
		}
		attrVal := attrsField.MapIndex(key)
		if attrVal.Kind() == reflect.Interface {
			attrVal = attrVal.Elem()
		}
		out.Attributes = append(out.Attributes, buildYAMLAttr(name, attrVal))
	}
	sort.Slice(out.Attributes, func(i, j int) bool {
		return out.Attributes[i].Name < out.Attributes[j].Name
	})

	data, err := yaml.Marshal(out)
	if err != nil {
		return "error: " + err.Error() + "\n"
	}
	return string(data)
}

// DumpDescriptions produces a deterministic YAML dump of all field
// descriptions in a schema. Only includes fields that have a non-empty
// description. Uses the same nested structure as DumpSchema for readability.
func DumpDescriptions(s any) string {
	sv := reflect.ValueOf(s)
	attrsField := sv.FieldByName("Attributes")
	if !attrsField.IsValid() || attrsField.Kind() != reflect.Map {
		return "error: no Attributes field found\n"
	}

	out := &yamlDescriptions{}
	if df := sv.FieldByName("Description"); df.IsValid() && df.Kind() == reflect.String && df.String() != "" {
		out.Description = df.String()
	}

	for _, key := range attrsField.MapKeys() {
		name := key.String()
		if name == "timeouts" {
			continue
		}
		attrVal := attrsField.MapIndex(key)
		if attrVal.Kind() == reflect.Interface {
			attrVal = attrVal.Elem()
		}
		if a := buildDescAttr(name, attrVal); a != nil {
			out.Attributes = append(out.Attributes, *a)
		}
	}
	sort.Slice(out.Attributes, func(i, j int) bool {
		return out.Attributes[i].Name < out.Attributes[j].Name
	})

	data, err := yaml.Marshal(out)
	if err != nil {
		return "error: " + err.Error() + "\n"
	}
	return string(data)
}

type yamlDescriptions struct {
	Description string     `yaml:"description,omitempty"`
	Attributes  []descAttr `yaml:"attributes,omitempty"`
}

type descAttr struct {
	Name        string     `yaml:"name"`
	Description string     `yaml:"description,omitempty"`
	Attributes  []descAttr `yaml:"attributes,omitempty"`
}

func buildDescAttr(name string, v reflect.Value) *descAttr {
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	desc := getString(v, "Description")
	nested := getNestedDescs(v)

	if desc == "" && len(nested) == 0 {
		return nil
	}

	return &descAttr{
		Name:        name,
		Description: desc,
		Attributes:  nested,
	}
}

func getNestedDescs(v reflect.Value) []descAttr {
	af := v.FieldByName("Attributes")
	if !af.IsValid() || af.Kind() != reflect.Map {
		nof := v.FieldByName("NestedObject")
		if nof.IsValid() && nof.Kind() == reflect.Struct {
			af = nof.FieldByName("Attributes")
		}
	}
	if !af.IsValid() || af.Kind() != reflect.Map {
		return nil
	}

	var attrs []descAttr
	for _, key := range af.MapKeys() {
		val := af.MapIndex(key)
		if val.Kind() == reflect.Interface {
			val = val.Elem()
		}
		if a := buildDescAttr(key.String(), val); a != nil {
			attrs = append(attrs, *a)
		}
	}
	sort.Slice(attrs, func(i, j int) bool {
		return attrs[i].Name < attrs[j].Name
	})
	return attrs
}

type yamlSchema struct {
	HasTimeouts bool       `yaml:"has_timeouts,omitempty"`
	Attributes  []yamlAttr `yaml:"attributes"`
}

type yamlAttr struct {
	Name               string     `yaml:"name"`
	Type               string     `yaml:"type"`
	Required           bool       `yaml:"required,omitempty"`
	Optional           bool       `yaml:"optional,omitempty"`
	Computed           bool       `yaml:"computed,omitempty"`
	Sensitive          bool       `yaml:"sensitive,omitempty"`
	WriteOnly          bool       `yaml:"write_only,omitempty"`
	DeprecationMessage string     `yaml:"deprecation_message,omitempty"`
	Validators         []string   `yaml:"validators,omitempty"`
	PlanModifiers      []string   `yaml:"plan_modifiers,omitempty"`
	Default            string     `yaml:"default,omitempty"`
	ElementType        string     `yaml:"element_type,omitempty"`
	Attributes         []yamlAttr `yaml:"attributes,omitempty"`
}

func buildYAMLAttr(name string, v reflect.Value) yamlAttr {
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	a := yamlAttr{
		Name:               name,
		Type:               v.Type().Name(),
		Required:           getBool(v, "Required"),
		Optional:           getBool(v, "Optional"),
		Computed:           getBool(v, "Computed"),
		Sensitive:          getBool(v, "Sensitive"),
		WriteOnly:          getBool(v, "WriteOnly"),
		DeprecationMessage: getString(v, "DeprecationMessage"),
		Validators:         sliceTypeNames(v, "Validators"),
		PlanModifiers:      sliceTypeNames(v, "PlanModifiers"),
	}

	if df := v.FieldByName("Default"); df.IsValid() && df.Kind() == reflect.Interface && !df.IsNil() {
		a.Default = df.Elem().Type().String()
	}
	if ef := v.FieldByName("ElementType"); ef.IsValid() && ef.Kind() == reflect.Interface && !ef.IsNil() {
		a.ElementType = ef.Elem().Type().String()
	}

	nested := getNestedAttrs(v)
	sort.Slice(nested, func(i, j int) bool {
		return nested[i].Name < nested[j].Name
	})
	a.Attributes = nested

	return a
}

func getNestedAttrs(v reflect.Value) []yamlAttr {
	af := v.FieldByName("Attributes")
	if !af.IsValid() || af.Kind() != reflect.Map {
		nof := v.FieldByName("NestedObject")
		if nof.IsValid() && nof.Kind() == reflect.Struct {
			af = nof.FieldByName("Attributes")
		}
	}
	if !af.IsValid() || af.Kind() != reflect.Map {
		return nil
	}

	var attrs []yamlAttr
	for _, key := range af.MapKeys() {
		val := af.MapIndex(key)
		if val.Kind() == reflect.Interface {
			val = val.Elem()
		}
		attrs = append(attrs, buildYAMLAttr(key.String(), val))
	}
	return attrs
}

func sliceTypeNames(v reflect.Value, fieldName string) []string {
	f := v.FieldByName(fieldName)
	if !f.IsValid() || f.Kind() != reflect.Slice || f.Len() == 0 {
		return nil
	}
	names := make([]string, f.Len())
	for i := 0; i < f.Len(); i++ {
		elem := f.Index(i)
		if elem.Kind() == reflect.Interface {
			elem = elem.Elem()
		}
		names[i] = elem.Type().String()
	}
	return names
}

func getBool(v reflect.Value, name string) bool {
	f := v.FieldByName(name)
	return f.IsValid() && f.Kind() == reflect.Bool && f.Bool()
}

func getString(v reflect.Value, name string) string {
	f := v.FieldByName(name)
	if f.IsValid() && f.Kind() == reflect.String {
		return f.String()
	}
	return ""
}
