// Copyright 2024 Redpanda Data, Inc.
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

package base

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
	"golang.org/x/oauth2"
)

func testTS(token string) oauth2.TokenSource {
	return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
}

func TestResourceBase_Metadata(t *testing.T) {
	b := NewResourceBase("redpanda_thing", func(context.Context) rschema.Schema { return rschema.Schema{} }, nil)
	resp := &resource.MetadataResponse{}
	b.Metadata(context.Background(), resource.MetadataRequest{}, resp)
	if resp.TypeName != "redpanda_thing" {
		t.Fatalf("TypeName: got %q, want redpanda_thing", resp.TypeName)
	}
}

func TestResourceBase_Schema(t *testing.T) {
	want := rschema.Schema{Description: "marker"}
	b := NewResourceBase("redpanda_thing", func(context.Context) rschema.Schema { return want }, nil)
	resp := &resource.SchemaResponse{}
	b.Schema(context.Background(), resource.SchemaRequest{}, resp)
	if resp.Schema.Description != "marker" {
		t.Fatalf("Schema.Description: got %q, want marker", resp.Schema.Description)
	}
}

func TestResourceBase_Configure(t *testing.T) {
	t.Run("nil ProviderData is a no-op", func(t *testing.T) {
		b := NewResourceBase("redpanda_thing", func(context.Context) rschema.Schema { return rschema.Schema{} }, nil)
		resp := &resource.ConfigureResponse{}
		b.Configure(context.Background(), resource.ConfigureRequest{ProviderData: nil}, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
		}
		if b.CpCl != nil {
			t.Fatal("CpCl should remain nil when ProviderData is nil")
		}
	})

	t.Run("wrong ProviderData type yields a diagnostic", func(t *testing.T) {
		b := NewResourceBase("redpanda_thing", func(context.Context) rschema.Schema { return rschema.Schema{} }, nil)
		resp := &resource.ConfigureResponse{}
		b.Configure(context.Background(), resource.ConfigureRequest{ProviderData: "not a config"}, resp)
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected an error diagnostic, got none")
		}
		if b.CpCl != nil {
			t.Fatal("CpCl should remain nil on type mismatch")
		}
	})

	t.Run("correct ProviderData wires CpCl", func(t *testing.T) {
		b := NewResourceBase("redpanda_thing", func(context.Context) rschema.Schema { return rschema.Schema{} }, nil)
		resp := &resource.ConfigureResponse{}
		b.Configure(context.Background(), resource.ConfigureRequest{ProviderData: config.Resource{}}, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
		}
		if b.CpCl == nil {
			t.Fatal("CpCl should be set after Configure")
		}
	})

	t.Run("extra hook fires on successful configure", func(t *testing.T) {
		var got config.Resource
		called := false
		b := NewResourceBase(
			"redpanda_thing",
			func(context.Context) rschema.Schema { return rschema.Schema{} },
			func(p config.Resource) { called = true; got = p },
		)
		resp := &resource.ConfigureResponse{}
		ts := testTS("tok")
		in := config.Resource{TokenSource: ts}
		b.Configure(context.Background(), resource.ConfigureRequest{ProviderData: in}, resp)
		if !called {
			t.Fatal("extra hook was not called")
		}
		if got.TokenSource != ts {
			t.Fatalf("extra hook received wrong value: got %+v", got)
		}
	})

	t.Run("extra hook does not fire when ProviderData is nil", func(t *testing.T) {
		called := false
		b := NewResourceBase(
			"redpanda_thing",
			func(context.Context) rschema.Schema { return rschema.Schema{} },
			func(config.Resource) { called = true },
		)
		resp := &resource.ConfigureResponse{}
		b.Configure(context.Background(), resource.ConfigureRequest{ProviderData: nil}, resp)
		if called {
			t.Fatal("extra hook should not be called when ProviderData is nil")
		}
	})
}

func TestDataSourceBase_Metadata(t *testing.T) {
	b := NewDataSourceBase("redpanda_thing", func(context.Context) dschema.Schema { return dschema.Schema{} }, nil)
	resp := &datasource.MetadataResponse{}
	b.Metadata(context.Background(), datasource.MetadataRequest{}, resp)
	if resp.TypeName != "redpanda_thing" {
		t.Fatalf("TypeName: got %q, want redpanda_thing", resp.TypeName)
	}
}

func TestDataSourceBase_Schema(t *testing.T) {
	want := dschema.Schema{Description: "marker"}
	b := NewDataSourceBase("redpanda_thing", func(context.Context) dschema.Schema { return want }, nil)
	resp := &datasource.SchemaResponse{}
	b.Schema(context.Background(), datasource.SchemaRequest{}, resp)
	if resp.Schema.Description != "marker" {
		t.Fatalf("Schema.Description: got %q, want marker", resp.Schema.Description)
	}
}

func TestDataSourceBase_Configure(t *testing.T) {
	t.Run("nil ProviderData is a no-op", func(t *testing.T) {
		b := NewDataSourceBase("redpanda_thing", func(context.Context) dschema.Schema { return dschema.Schema{} }, nil)
		resp := &datasource.ConfigureResponse{}
		b.Configure(context.Background(), datasource.ConfigureRequest{ProviderData: nil}, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
		}
		if b.CpCl != nil {
			t.Fatal("CpCl should remain nil when ProviderData is nil")
		}
	})

	t.Run("wrong ProviderData type yields a diagnostic", func(t *testing.T) {
		b := NewDataSourceBase("redpanda_thing", func(context.Context) dschema.Schema { return dschema.Schema{} }, nil)
		resp := &datasource.ConfigureResponse{}
		b.Configure(context.Background(), datasource.ConfigureRequest{ProviderData: 42}, resp)
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected an error diagnostic, got none")
		}
		if b.CpCl != nil {
			t.Fatal("CpCl should remain nil on type mismatch")
		}
	})

	t.Run("correct ProviderData wires CpCl", func(t *testing.T) {
		b := NewDataSourceBase("redpanda_thing", func(context.Context) dschema.Schema { return dschema.Schema{} }, nil)
		resp := &datasource.ConfigureResponse{}
		b.Configure(context.Background(), datasource.ConfigureRequest{ProviderData: config.Datasource{}}, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
		}
		if b.CpCl == nil {
			t.Fatal("CpCl should be set after Configure")
		}
	})

	t.Run("extra hook fires on successful configure", func(t *testing.T) {
		var got config.Datasource
		called := false
		b := NewDataSourceBase(
			"redpanda_thing",
			func(context.Context) dschema.Schema { return dschema.Schema{} },
			func(p config.Datasource) { called = true; got = p },
		)
		resp := &datasource.ConfigureResponse{}
		ts := testTS("tok")
		in := config.Datasource{TokenSource: ts}
		b.Configure(context.Background(), datasource.ConfigureRequest{ProviderData: in}, resp)
		if !called {
			t.Fatal("extra hook was not called")
		}
		if got.TokenSource != ts {
			t.Fatalf("extra hook received wrong value: got %+v", got)
		}
	})
}
