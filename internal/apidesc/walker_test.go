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

package apidesc

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newSpec(schemas map[string]*Schema) *Spec {
	s := &Spec{}
	s.Components.Schemas = schemas
	return s
}

func TestFlatten_Simple(t *testing.T) {
	spec := newSpec(map[string]*Schema{
		"Cluster": {
			Description: "A Redpanda cluster.",
			Properties: map[string]*Schema{
				"name":    {Type: "string", Description: "Unique name of the cluster."},
				"id":      {Type: "string", Description: "ID of the cluster."},
				"enabled": {Type: "boolean", Description: "Whether the cluster is enabled."},
			},
		},
	})

	tree, warns, err := Flatten(map[string]*Spec{"test.yaml": spec})
	if err != nil {
		t.Fatalf("Flatten: %v", err)
	}
	if len(warns) != 0 {
		t.Errorf("unexpected warnings: %v", warns)
	}

	cluster := tree["Cluster"]
	if cluster == nil {
		t.Fatal("Cluster not in tree")
	}
	if cluster.Description != "A Redpanda cluster." {
		t.Errorf("Cluster.description: got %q", cluster.Description)
	}
	assertField(t, cluster, "name", "Unique name of the cluster.")
	assertField(t, cluster, "id", "ID of the cluster.")
	assertField(t, cluster, "enabled", "Whether the cluster is enabled.")
}

func TestFlatten_TitleFallback(t *testing.T) {
	spec := newSpec(map[string]*Schema{
		"ResourceGroup": {
			Title: "A resource group.",
			Properties: map[string]*Schema{
				"id": {Type: "string", Title: "ID of the resource group"},
				"name": {
					Type:        "string",
					Description: "The unique name.",
					Title:       "Should not win.",
				},
			},
		},
	})

	tree, _, err := Flatten(map[string]*Spec{"test.yaml": spec})
	if err != nil {
		t.Fatalf("Flatten: %v", err)
	}
	rg := tree["ResourceGroup"]
	if rg.Description != "A resource group." {
		t.Errorf("title fallback for schema: got %q", rg.Description)
	}
	assertField(t, rg, "id", "ID of the resource group")

	assertField(t, rg, "name", "The unique name.")
}

func TestFlatten_NestedRef(t *testing.T) {
	spec := newSpec(map[string]*Schema{
		"Cluster": {
			Description: "Cluster root.",
			Properties: map[string]*Schema{
				"aws_private_link": {Ref: refPrefix + "AWSPrivateLink"},
			},
		},
		"AWSPrivateLink": {
			Description: "AWS PrivateLink configuration.",
			Properties: map[string]*Schema{
				"enabled":            {Type: "boolean", Description: "Whether enabled."},
				"allowed_principals": {Type: "array", Description: "The ARN list."},
			},
		},
	})

	tree, _, err := Flatten(map[string]*Spec{"test.yaml": spec})
	if err != nil {
		t.Fatalf("Flatten: %v", err)
	}
	cluster := tree["Cluster"]
	apl := cluster.Fields["aws_private_link"]
	if apl == nil {
		t.Fatal("aws_private_link missing")
	}

	if apl.Description != "AWS PrivateLink configuration." {
		t.Errorf("ref fallback description: got %q", apl.Description)
	}
	assertField(t, apl, "enabled", "Whether enabled.")
	assertField(t, apl, "allowed_principals", "The ARN list.")
}

func TestFlatten_ItemsRef(t *testing.T) {
	spec := newSpec(map[string]*Schema{
		"PrivateLinkStatus": {
			Properties: map[string]*Schema{
				"vpc_endpoint_connections": {
					Type:        "array",
					Description: "List of connections.",
					Items:       &Schema{Ref: refPrefix + "VPCEndpointConnection"},
				},
			},
		},
		"VPCEndpointConnection": {
			Description: "A VPC endpoint connection.",
			Properties: map[string]*Schema{
				"connection_id": {Type: "string", Description: "Connection ID."},
			},
		},
	})

	tree, _, err := Flatten(map[string]*Spec{"test.yaml": spec})
	if err != nil {
		t.Fatalf("Flatten: %v", err)
	}
	status := tree["PrivateLinkStatus"]
	vec := status.Fields["vpc_endpoint_connections"]
	if vec == nil {
		t.Fatal("vpc_endpoint_connections missing")
	}
	if vec.Description != "List of connections." {
		t.Errorf("array description: got %q", vec.Description)
	}

	assertField(t, vec, "connection_id", "Connection ID.")
}

func TestFlatten_AdditionalPropertiesRef(t *testing.T) {
	spec := newSpec(map[string]*Schema{
		"Root": {
			Properties: map[string]*Schema{
				"tags_by_id": {
					Type:                 "object",
					Description:          "Map of tags by ID.",
					AdditionalProperties: &Schema{Ref: refPrefix + "Tag"},
				},
			},
		},
		"Tag": {
			Properties: map[string]*Schema{
				"label": {Type: "string", Description: "Tag label."},
			},
		},
	})

	tree, _, err := Flatten(map[string]*Spec{"test.yaml": spec})
	if err != nil {
		t.Fatalf("Flatten: %v", err)
	}
	root := tree["Root"]
	tags := root.Fields["tags_by_id"]
	if tags == nil {
		t.Fatal("tags_by_id missing")
	}
	if tags.Description != "Map of tags by ID." {
		t.Errorf("map description: got %q", tags.Description)
	}
	assertField(t, tags, "label", "Tag label.")
}

func TestFlatten_BareRefInheritsDescription(t *testing.T) {
	spec := newSpec(map[string]*Schema{
		"Cluster": {
			Properties: map[string]*Schema{
				"state": {Ref: refPrefix + "Cluster.State"},
			},
		},
		"Cluster.State": {
			Type:        "string",
			Description: "Current cluster state.",
		},
	})

	tree, _, err := Flatten(map[string]*Spec{"test.yaml": spec})
	if err != nil {
		t.Fatalf("Flatten: %v", err)
	}
	cluster := tree["Cluster"]
	assertField(t, cluster, "state", "Current cluster state.")
}

func TestFlatten_CycleGuard(t *testing.T) {
	spec := newSpec(map[string]*Schema{
		"A": {
			Description: "A.",
			Properties: map[string]*Schema{
				"child": {Ref: refPrefix + "B"},
			},
		},
		"B": {
			Description: "B.",
			Properties: map[string]*Schema{
				"back": {Ref: refPrefix + "A"},
			},
		},
	})

	tree, _, err := Flatten(map[string]*Spec{"test.yaml": spec})
	if err != nil {
		t.Fatalf("Flatten: %v", err)
	}

	if tree["A"] == nil || tree["B"] == nil {
		t.Fatal("missing schemas")
	}

	childB := tree["A"].Fields["child"]
	if childB == nil {
		t.Fatal("A.child missing")
	}
	if childB.Description != "B." {
		t.Errorf("A.child description: got %q", childB.Description)
	}
}

func TestFlatten_CollisionSameValue(t *testing.T) {
	a := newSpec(map[string]*Schema{
		"Shared": {
			Properties: map[string]*Schema{
				"name": {Type: "string", Description: "The name."},
			},
		},
	})
	b := newSpec(map[string]*Schema{
		"Shared": {
			Properties: map[string]*Schema{
				"name": {Type: "string", Description: "The name."},
			},
		},
	})
	_, _, err := Flatten(map[string]*Spec{"a.yaml": a, "b.yaml": b})
	if err != nil {
		t.Errorf("same-value collision should be OK, got: %v", err)
	}
}

func TestFlatten_CollisionDifferentValue(t *testing.T) {
	winner := newSpec(map[string]*Schema{
		"Shared": {
			Properties: map[string]*Schema{
				"name": {Type: "string", Description: "The name in controlplane."},
			},
		},
	})
	loser := newSpec(map[string]*Schema{
		"Shared": {
			Properties: map[string]*Schema{
				"name": {Type: "string", Description: "The name in dataplane."},
			},
		},
	})
	tree, warns, err := Flatten(map[string]*Spec{
		"cloudv2/openapi.controlplane.prod.yaml": winner,
		"cloudv2/openapi.dataplane.prod.yaml":    loser,
	})
	if err != nil {
		t.Fatalf("Flatten: %v", err)
	}
	if got := tree["Shared"].Fields["name"].Description; got != "The name in controlplane." {
		t.Errorf("precedence: got %q, want controlplane value", got)
	}
	foundCollisionWarning := false
	for _, w := range warns {
		if strings.Contains(w, "collision") && strings.Contains(w, "dataplane") {
			foundCollisionWarning = true
			break
		}
	}
	if !foundCollisionWarning {
		t.Errorf("expected collision warning naming dataplane, got warnings: %v", warns)
	}
}

func TestFlatten_CrossSourceCollision(t *testing.T) {
	cloud := newSpec(map[string]*Schema{
		"ACL.Operation": {
			Description: "Operation (cloudv2).",
		},
	})
	console := newSpec(map[string]*Schema{
		"ACL.Operation": {
			Description: "Operation (console).",
		},
	})
	tree, warns, err := Flatten(map[string]*Spec{
		"cloudv2/openapi.dataplane.prod.yaml": cloud,
		"console/openapi.yaml":                console,
	})
	if err != nil {
		t.Fatalf("Flatten: %v", err)
	}
	if got := tree["ACL.Operation"].Description; got != "Operation (cloudv2)." {
		t.Errorf("precedence: got %q, want cloudv2 value", got)
	}
	foundWarning := false
	for _, w := range warns {
		if strings.Contains(w, "console/openapi.yaml") {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Errorf("expected collision warning naming console/openapi.yaml, got: %v", warns)
	}
}

func TestFlatten_Normalization(t *testing.T) {
	spec := newSpec(map[string]*Schema{
		"X": {
			Properties: map[string]*Schema{
				"multi": {
					Type: "string",
					Description: `Line one.
Line two.
    Line three with leading spaces.`,
				},
			},
		},
	})
	tree, _, err := Flatten(map[string]*Spec{"x.yaml": spec})
	if err != nil {
		t.Fatalf("Flatten: %v", err)
	}
	got := tree["X"].Fields["multi"].Description
	want := "Line one. Line two. Line three with leading spaces."
	if got != want {
		t.Errorf("normalization: got %q, want %q", got, want)
	}
}

func TestFlatten_UnresolvedRef(t *testing.T) {
	spec := newSpec(map[string]*Schema{
		"A": {
			Properties: map[string]*Schema{
				"missing": {Ref: refPrefix + "DoesNotExist"},
			},
		},
	})
	_, warns, err := Flatten(map[string]*Spec{"a.yaml": spec})
	if err != nil {
		t.Fatalf("Flatten: %v", err)
	}
	found := false
	for _, w := range warns {
		if strings.Contains(w, "DoesNotExist") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning about unresolved ref, got: %v", warns)
	}
}

func TestLoadAndLookup_RoundTrip(t *testing.T) {
	schemas := map[string]*Node{
		"Cluster": {
			Description: "A cluster.",
			Fields: map[string]*Node{
				"name": {Description: "Cluster name."},
				"aws_private_link": {
					Description: "APL config.",
					Fields: map[string]*Node{
						"enabled": {Description: "Whether enabled."},
					},
				},
			},
		},
	}
	data, err := Encode(&File{Schemas: schemas}, "# test header")
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "apidescriptions.yaml")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	idx, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if idx == nil {
		t.Fatal("Load returned nil index")
	}

	cases := []struct {
		path string
		want string
	}{
		{"Cluster", "A cluster."},
		{"Cluster.name", "Cluster name."},
		{"Cluster.aws_private_link", "APL config."},
		{"Cluster.aws_private_link.enabled", "Whether enabled."},
	}
	for _, c := range cases {
		got, ok := idx.Lookup(c.path)
		if !ok {
			t.Errorf("Lookup(%q): not found", c.path)
			continue
		}
		if got != c.want {
			t.Errorf("Lookup(%q): got %q, want %q", c.path, got, c.want)
		}
	}

	if _, ok := idx.Lookup("Cluster.missing"); ok {
		t.Error("Lookup(Cluster.missing): expected not found")
	}
}

func TestLoad_MissingFileReturnsNil(t *testing.T) {
	idx, err := Load(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if err != nil {
		t.Errorf("missing file should return nil err, got: %v", err)
	}
	if idx != nil {
		t.Error("missing file should return nil index")
	}
}

func TestIndex_NilSafe(t *testing.T) {
	var idx *Index
	if _, ok := idx.Lookup("anything"); ok {
		t.Error("nil index lookup should return false")
	}
	if idx.Len() != 0 {
		t.Error("nil index Len should be 0")
	}
}

func TestEncode_DeterministicOrdering(t *testing.T) {
	schemas := map[string]*Node{
		"Z": {Description: "Z."},
		"A": {
			Description: "A.",
			Fields: map[string]*Node{
				"z": {Description: "z."},
				"a": {Description: "a."},
			},
		},
	}
	a, err := Encode(&File{Schemas: schemas}, "")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Encode(&File{Schemas: schemas}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Error("encoding not deterministic")
	}

	idxA := bytes.Index(a, []byte("A:"))
	idxZ := bytes.Index(a, []byte("Z:"))
	if idxA < 0 || idxZ < 0 || idxA > idxZ {
		t.Errorf("expected A before Z, got:\n%s", a)
	}
}

func assertField(t *testing.T, parent *Node, name, wantDesc string) {
	t.Helper()
	child := parent.Fields[name]
	if child == nil {
		t.Errorf("field %q missing", name)
		return
	}
	if child.Description != wantDesc {
		t.Errorf("field %q description: got %q, want %q", name, child.Description, wantDesc)
	}
}
