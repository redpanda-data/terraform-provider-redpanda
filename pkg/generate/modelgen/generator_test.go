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
	"testing"
)

func TestToGoFieldName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Basic cases
		{name: "simple lowercase", input: "name", expected: "Name"},
		{name: "simple with underscore", input: "first_name", expected: "FirstName"},
		{name: "multiple underscores", input: "my_long_field_name", expected: "MyLongFieldName"},

		// Abbreviation cases
		{name: "id alone", input: "id", expected: "ID"},
		{name: "id suffix", input: "resource_group_id", expected: "ResourceGroupID"},
		{name: "id prefix", input: "id_field", expected: "IDField"},

		{name: "url alone", input: "url", expected: "URL"},
		{name: "url suffix", input: "cluster_api_url", expected: "ClusterAPIURL"},

		{name: "api alone", input: "api", expected: "API"},
		{name: "api in middle", input: "cluster_api_url", expected: "ClusterAPIURL"},

		// Cloud provider abbreviations
		{name: "aws alone", input: "aws", expected: "AWS"},
		{name: "aws prefix", input: "aws_private_link", expected: "AWSPrivateLink"},

		{name: "gcp alone", input: "gcp", expected: "GCP"},
		{name: "gcp prefix", input: "gcp_private_service_connect", expected: "GCPPrivateServiceConnect"},
		{name: "gcp global access", input: "gcp_global_access_enabled", expected: "GCPGlobalAccessEnabled"},

		// Other abbreviations
		{name: "http", input: "http_proxy", expected: "HTTPProxy"},
		{name: "tls", input: "tls_enabled", expected: "TLSEnabled"},
		{name: "ssl", input: "ssl_certificate", expected: "SSLCertificate"},
		{name: "dns", input: "dns_name", expected: "DNSName"},
		{name: "vpc", input: "vpc_id", expected: "VPCID"},
		{name: "arn", input: "role_arn", expected: "RoleARN"},
		{name: "json", input: "json_config", expected: "JSONConfig"},
		{name: "xml", input: "xml_data", expected: "XMLData"},
		{name: "sql", input: "sql_query", expected: "SQLQuery"},
		{name: "ssh", input: "ssh_key", expected: "SSHKey"},
		{name: "uri", input: "uri_path", expected: "URIPath"},
		{name: "cmr", input: "cmr_enabled", expected: "CMREnabled"},

		// Edge cases
		{name: "empty string", input: "", expected: ""},
		{name: "single char", input: "a", expected: "A"},
		{name: "already capitalized abbreviation", input: "ID", expected: "ID"},
		{name: "double underscore", input: "foo__bar", expected: "FooBar"},
		{name: "trailing underscore", input: "foo_", expected: "Foo"},
		{name: "leading underscore", input: "_foo", expected: "Foo"},

		// Real-world examples from the codebase
		{name: "read_replica_cluster_ids", input: "read_replica_cluster_ids", expected: "ReadReplicaClusterIds"},
		{name: "kafka_api", input: "kafka_api", expected: "KafkaAPI"},
		{name: "schema_registry", input: "schema_registry", expected: "SchemaRegistry"},
		{name: "azure_private_link", input: "azure_private_link", expected: "AzurePrivateLink"},
		{name: "redpanda_version", input: "redpanda_version", expected: "RedpandaVersion"},
		{name: "throughput_tier", input: "throughput_tier", expected: "ThroughputTier"},
		{name: "customer_managed_resources", input: "customer_managed_resources", expected: "CustomerManagedResources"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toGoFieldName(tt.input)
			if result != tt.expected {
				t.Errorf("toGoFieldName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGoTypeForAttribute(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// All known types from typeMap
		{name: "StringAttribute", input: "StringAttribute", expected: "types.String"},
		{name: "BoolAttribute", input: "BoolAttribute", expected: "types.Bool"},
		{name: "Int32Attribute", input: "Int32Attribute", expected: "types.Int32"},
		{name: "Int64Attribute", input: "Int64Attribute", expected: "types.Int64"},
		{name: "NumberAttribute", input: "NumberAttribute", expected: "types.Number"},
		{name: "Float64Attribute", input: "Float64Attribute", expected: "types.Float64"},
		{name: "ListAttribute", input: "ListAttribute", expected: "types.List"},
		{name: "SetAttribute", input: "SetAttribute", expected: "types.Set"},
		{name: "MapAttribute", input: "MapAttribute", expected: "types.Map"},
		{name: "ObjectAttribute", input: "ObjectAttribute", expected: "types.Object"},
		{name: "SingleNestedAttribute", input: "SingleNestedAttribute", expected: "types.Object"},
		{name: "ListNestedAttribute", input: "ListNestedAttribute", expected: "types.List"},
		{name: "SetNestedAttribute", input: "SetNestedAttribute", expected: "types.Set"},
		{name: "MapNestedAttribute", input: "MapNestedAttribute", expected: "types.Map"},
		{name: "DynamicAttribute", input: "DynamicAttribute", expected: "types.Dynamic"},

		// Unknown types should default to types.String
		{name: "unknown type", input: "UnknownAttribute", expected: "types.String"},
		{name: "empty string", input: "", expected: "types.String"},
		{name: "random string", input: "FooBar", expected: "types.String"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := goTypeForAttribute(tt.input)
			if result != tt.expected {
				t.Errorf("goTypeForAttribute(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
