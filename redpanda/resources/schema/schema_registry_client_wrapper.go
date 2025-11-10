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

package schema

import (
	"context"

	"github.com/twmb/franz-go/pkg/sr"
)

// schemaRegistryClientWrapper wraps sr.Client to implement SRClienter
type schemaRegistryClientWrapper struct {
	client *sr.Client
}

// newSchemaRegistryClientWrapper creates a new wrapper around an sr.Client
func newSchemaRegistryClientWrapper(client *sr.Client) SRClienter {
	return &schemaRegistryClientWrapper{client: client}
}

func (w *schemaRegistryClientWrapper) CreateSchema(ctx context.Context, subject string, schema sr.Schema) (sr.SubjectSchema, error) {
	return w.client.CreateSchema(ctx, subject, schema)
}

func (w *schemaRegistryClientWrapper) SchemaByVersion(ctx context.Context, subject string, version int) (sr.SubjectSchema, error) {
	return w.client.SchemaByVersion(ctx, subject, version)
}

func (w *schemaRegistryClientWrapper) Schemas(ctx context.Context, subject string) ([]sr.SubjectSchema, error) {
	return w.client.Schemas(ctx, subject)
}

func (w *schemaRegistryClientWrapper) DeleteSubject(ctx context.Context, subject string, how sr.DeleteHow) ([]int, error) {
	return w.client.DeleteSubject(ctx, subject, how)
}

func (w *schemaRegistryClientWrapper) SetCompatibility(ctx context.Context, c sr.SetCompatibility, subjects ...string) []sr.CompatibilityResult {
	return w.client.SetCompatibility(ctx, c, subjects...)
}

func (w *schemaRegistryClientWrapper) Compatibility(ctx context.Context, subjects ...string) []sr.CompatibilityResult {
	return w.client.Compatibility(ctx, subjects...)
}
