// Copyright 2026 Redpanda Data, Inc.
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
	"strings"
	"testing"

	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
)

func replacementProto() *ProtoMessage {
	return &ProtoMessage{
		Name: "Thing",
		Fields: []ProtoField{
			{Name: "region", Kind: KindString, Cardinality: "singular", ValidateRules: fieldRulesStringLen(1, 64)},
			{Name: "name", Kind: KindString, Cardinality: "singular"},
			{Name: "partition_count", Kind: KindInt32, Cardinality: "singular"},
			{Name: "cmr", Kind: KindMessage, Cardinality: "singular", Nested: &ProtoMessage{
				Name: "CMR",
				Fields: []ProtoField{
					{Name: "arn", Kind: KindString, Cardinality: "singular"},
					{Name: "label", Kind: KindString, Cardinality: "singular"},
				},
			}},
		},
	}
}

func replacementConfig() *Config {
	tru := true
	return &Config{
		Fields: map[string]FieldConfig{
			"region":          {PlanModifiers: []string{"RequiresReplace"}},
			"partition_count": {Optional: &tru, Computed: &tru, PlanModifiers: []string{"RequiresReplaceIfShrinking"}},
			"cmr": {Optional: &tru, Fields: map[string]FieldConfig{
				"arn": {PlanModifiers: []string{"RequiresReplace"}},
			}},
		},
	}
}

func descriptionsByPath(attrs []SchemaAttr, parent string, into map[string]string) {
	for i := range attrs {
		path := joinPath(parent, attrs[i].Name)
		into[path] = attrs[i].Description
		descriptionsByPath(attrs[i].NestedAttrs, path, into)
	}
}

// TestMerge_ReplacementWarning_FollowsRequiresReplace pins that the docs
// warning lands on exactly the attributes whose final plan modifiers force
// replacement, nested ones included, and after the validator sentences.
func TestMerge_ReplacementWarning_FollowsRequiresReplace(t *testing.T) {
	attrs, _, _, errs := Merge(replacementProto(), replacementConfig(), "resource", nil)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	got := map[string]string{}
	descriptionsByPath(attrs, "", got)

	for _, path := range []string{"region", "cmr.arn"} {
		if !strings.HasSuffix(got[path], utils.ReplacementWarning) {
			t.Errorf("%s: description %q does not end with the replacement warning", path, got[path])
		}
	}
	for _, path := range []string{"name", "cmr", "cmr.label"} {
		if strings.Contains(got[path], utils.ReplacementWarning) {
			t.Errorf("%s: description %q carries a replacement warning without RequiresReplace", path, got[path])
		}
	}
	if !strings.Contains(got["region"], "Length must be between 1 and 64. "+utils.ReplacementWarning) {
		t.Errorf("region: validator sentence must precede the warning, got %q", got["region"])
	}
}

// TestMerge_ReplacementWarning_ConditionalUsesModifierText pins that a
// conditional replace modifier documents its own trigger instead of the
// unconditional sentence, which would be false for edits the predicate lets
// through.
func TestMerge_ReplacementWarning_ConditionalUsesModifierText(t *testing.T) {
	attrs, _, _, errs := Merge(replacementProto(), replacementConfig(), "resource", nil)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	got := map[string]string{}
	descriptionsByPath(attrs, "", got)

	desc := got["partition_count"]
	if !strings.HasSuffix(desc, "Decreasing partition count requires recreating the topic.") {
		t.Errorf("partition_count: description %q does not end with the shrink sentence", desc)
	}
	if strings.Contains(desc, utils.ReplacementWarning) {
		t.Errorf("partition_count: description %q carries the unconditional warning", desc)
	}
}

// TestMerge_ReplacementWarning_SkipsDatasource pins that datasource
// descriptions never gain the warning: datasources carry no plan modifiers.
func TestMerge_ReplacementWarning_SkipsDatasource(t *testing.T) {
	attrs, _, _, errs := Merge(replacementProto(), replacementConfig(), SchemaTypeDatasource, nil)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	got := map[string]string{}
	descriptionsByPath(attrs, "", got)
	for path, desc := range got {
		if strings.Contains(desc, utils.ReplacementWarning) || strings.Contains(desc, "recreating the topic") {
			t.Errorf("%s: datasource description %q carries a replacement warning", path, desc)
		}
	}
}
