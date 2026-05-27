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
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/config"
)

// ResourceBase factors out the Metadata/Schema/Configure dispatch that every
// resource implements identically. Embed it in a concrete resource type and
// initialize via NewResourceBase. The concrete type still owns Create/Read/
// Update/Delete and any optional framework interfaces (ModifyPlan,
// ConfigValidators, ImportState).
type ResourceBase struct {
	CpCl *cloud.ControlPlaneClientSet

	typeName string
	schemaFn func(context.Context) rschema.Schema
	extra    func(config.Resource)
}

// NewResourceBase constructs a ResourceBase. extra is optional; pass nil when
// the resource needs no clients beyond CpCl.
func NewResourceBase(typeName string, schemaFn func(context.Context) rschema.Schema, extra func(config.Resource)) ResourceBase {
	return ResourceBase{typeName: typeName, schemaFn: schemaFn, extra: extra}
}

// Metadata implements resource.Resource.
func (b *ResourceBase) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = b.typeName
}

// Schema implements resource.Resource.
func (b *ResourceBase) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = b.schemaFn(ctx)
}

// Configure wires CpCl from the provider's config.Resource and invokes the
// per-resource extra hook (if any) to wire additional clients.
func (b *ResourceBase) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	p, ok := req.ProviderData.(config.Resource)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected config.Resource, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	b.CpCl = cloud.NewControlPlaneClientSet(p.ControlPlaneConnection)
	if b.extra != nil {
		b.extra(p)
	}
}

// DataSourceBase factors out the Metadata/Schema/Configure dispatch for
// datasources. Embed it in a concrete datasource type and initialize via
// NewDataSourceBase. The concrete type still owns Read.
type DataSourceBase struct {
	CpCl *cloud.ControlPlaneClientSet

	typeName string
	schemaFn func(context.Context) dschema.Schema
	extra    func(config.Datasource)
}

// NewDataSourceBase constructs a DataSourceBase. extra is optional; pass nil
// when the datasource needs no clients beyond CpCl.
func NewDataSourceBase(typeName string, schemaFn func(context.Context) dschema.Schema, extra func(config.Datasource)) DataSourceBase {
	return DataSourceBase{typeName: typeName, schemaFn: schemaFn, extra: extra}
}

// Metadata implements datasource.DataSource.
func (b *DataSourceBase) Metadata(_ context.Context, _ datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = b.typeName
}

// Schema implements datasource.DataSource.
func (b *DataSourceBase) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = b.schemaFn(ctx)
}

// Configure wires CpCl from the provider's config.Datasource and invokes the
// per-datasource extra hook (if any) to wire additional clients.
func (b *DataSourceBase) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	p, ok := req.ProviderData.(config.Datasource)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected config.Datasource, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	b.CpCl = cloud.NewControlPlaneClientSet(p.ControlPlaneConnection)
	if b.extra != nil {
		b.extra(p)
	}
}
