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
	"strings"
	"testing"
)

func TestGenerateProtoValidator(t *testing.T) {
	src, err := GenerateProtoValidator(ProtoValidatorData{
		License:       "",
		Package:       "cluster",
		ResourceType:  "Cluster",
		ModelImport:   "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/cluster",
		ModelAlias:    "clustermodel",
		ModelType:     ModelTypeResource,
		ExpandFunc:    "ExpandCreate",
		ResourceLabel: "cluster",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := string(src)
	for _, want := range []string{
		"package cluster",
		"type protoValidator struct{}",
		"func (*Cluster) ConfigValidators(",
		"clustermodel \"github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/cluster\"",
		"var m clustermodel.ResourceModel",
		"payload, eDiags := clustermodel.ExpandCreate(ctx, &m)",
		"rpvalidate.Validate(path.Empty(), payload)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("generated source missing %q\n--- got ---\n%s", want, got)
		}
	}
}

// A non-empty InnerGetter validates the unwrapped Create payload so violation
// paths stay schema-relative and match the skip-set.
func TestGenerateProtoValidator_InnerGetter(t *testing.T) {
	src, err := GenerateProtoValidator(ProtoValidatorData{
		Package:       "cluster",
		ResourceType:  "Cluster",
		ModelImport:   "github.com/redpanda-data/terraform-provider-redpanda/redpanda/models/cluster",
		ModelAlias:    "clustermodel",
		ModelType:     ModelTypeResource,
		ExpandFunc:    "ExpandCreate",
		ResourceLabel: "cluster",
		InnerGetter:   "GetCluster",
		SkippedFields: []string{"rpsql"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := "rpvalidate.Validate(path.Empty(), payload.GetCluster(),"; !strings.Contains(string(src), want) {
		t.Errorf("generated source missing %q\n--- got ---\n%s", want, src)
	}
}

func TestProtoValidatorInnerGetter(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  *Config
		want string
	}{
		{"wrapped", &Config{API: &APIConfig{Create: &RPCConfig{PayloadField: "cluster"}}}, "GetCluster"},
		{"snake wrapper", &Config{API: &APIConfig{Create: &RPCConfig{PayloadField: "resource_group"}}}, "GetResourceGroup"},
		{"flat", &Config{API: &APIConfig{Create: &RPCConfig{}}}, ""},
		{"no create rpc", &Config{}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ProtoValidatorInnerGetter(tc.cfg, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}
