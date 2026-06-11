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
)

// commonDescriptions is the shared text for terraform-only fields, one entry
// per field name, used on every resource that declares the field. Yaml
// description overrides were removed; TF-only text lives here, proto-backed
// text comes from apidescriptions.yaml, and provider-behavior exceptions live
// in scopedDescriptions.
var commonDescriptions = map[string]string{
	"allow_deletion":         "Whether Terraform may destroy this resource. Defaults to false; set to true to enable destruction. After `terraform import`, defaults to false — set to true in your config before running `terraform destroy`.",
	"cluster_api_url":        "The cluster API URL. Changing this will prevent deletion of the resource on the existing cluster. It is generally a better idea to delete an existing resource and create a new one than to change this value unless you are planning to do state imports.",
	"cluster_type":           "Cluster type. Type is immutable and can only be set on cluster creation. Can be either byoc or dedicated.",
	"configuration":          "A map of string key/value pairs of topic configurations.",
	"custom_properties_json": "Custom cluster configuration properties in JSON format.",
	"delete_acls":            "Whether to delete the ACLs bound to the role when the role is deleted. Defaults to false.",
	"id":                     "Unique identifier of the resource.",
	"password":               "Password for authentication.",
	"password_wo":            "Password (write-only, not stored in state). Requires Terraform 1.11+. Either password or password_wo must be set.",
	"password_wo_version":    "Version number for password_wo. Increment this value to trigger a password update when using password_wo.",
	"secret_version":         "Version number for client_secret. Increment to trigger a secret update.",
}

// scopedDescriptions carries the few descriptions that can never come from
// apidescriptions.yaml: provider-side behavior (state filtering, Terraform
// plan semantics) and computed-output variants of fields whose shared text
// describes an input. Keyed "<api_schema>.<proto field path>" so one entry
// covers a resource and its datasource. Beats apidesc by design. Entries that
// are genuinely API facts (constraints, enum values, doc links) should move
// upstream into cloudv2 proto comments and be dropped here on a pin bump.
var scopedDescriptions = map[string]string{
	"Cluster.tags":                                     "Tags placed on cloud resources. Server-managed keys (prefixed with `redpanda-`) are filtered out of state.",
	"Cluster.cluster_api_url":                          "The URL of the cluster's data plane API.",
	"Cluster.cloud_storage.skip_destroy":               "If true, cloud storage is not deleted when the cluster is destroyed.",
	"Cluster.cloud_storage.azure.container_name":       "Name of the Azure storage container.",
	"Cluster.cloud_storage.azure.storage_account_name": "Name of the Azure storage account.",
	"Cluster.cloud_storage.gcp.name":                   "Name of the GCP storage bucket.",
	"Network.state":                                    "Current state of the network.",
	"ServerlessCluster.console_url":                    "Public Console URL for the serverless cluster.",
	"ServerlessCluster.console_private_url":            "Private Console URL for the serverless cluster.",
	"CreateTopicRequest.Topic.partition_count":         "The number of partitions for the topic. Increases are fully supported without data loss. Decreases will destroy and recreate the topic if allow_deletion is set to true (defaults to false).",
	"CreateUserRequest.User.mechanism":                 "Which authentication method to use. See https://docs.redpanda.com/current/manage/security/authentication/ for more information.",
	"Pipeline.state":                                   "Desired state of the pipeline: 'running' or 'stopped'. The provider will ensure the pipeline reaches this state after create/update operations.",
	"ServerlessCluster.cluster_api_url":                "The URL of the dataplane API for the serverless cluster.",
	"ServerlessCluster.private_link_id":                "Private link ID for the serverless cluster. Must be set if private networking is enabled.",
	"ServerlessCluster.networking_config.private":      "Private network state. Valid values: STATE_UNSPECIFIED, STATE_DISABLED, STATE_ENABLED.",
	"ServerlessCluster.networking_config.public":       "Public network state. Valid values: STATE_UNSPECIFIED, STATE_DISABLED, STATE_ENABLED.",
	"ServerlessPrivateLink.aws_config":                 "AWS-specific configuration. Required when cloud_provider is `aws`.",
}

// curatedDescription resolves a synthetic/extra attribute's description:
// scoped entry first (provider-behavior exceptions), then the shared
// plain-name table / mechanical default via generateDescription.
func curatedDescription(scope, path, name, attrType string) string {
	if scope != "" {
		if desc, ok := scopedDescriptions[scope+"."+path]; ok {
			return desc
		}
	}
	return generateDescription(name, "", attrType)
}

var abbreviations = map[string]string{
	"id": "ID", "url": "URL", "uri": "URI", "api": "API",
	"http": "HTTP", "json": "JSON", "xml": "XML", "sql": "SQL",
	"ssh": "SSH", "tls": "TLS", "ssl": "SSL", "aws": "AWS",
	"gcp": "GCP", "vpc": "VPC", "dns": "DNS", "arn": "ARN",
	"mtls": "mTLS", "sasl": "SASL", "psc": "PSC", "nat": "NAT",
	"aks": "AKS", "gke": "GKE", "k8s": "Kubernetes",
	"ipv4": "IPv4", "cidr": "CIDR",
}

func generateDescription(name, parentPath, attrType string) string {
	if desc, ok := commonDescriptions[name]; ok && parentPath == "" {
		return desc
	}

	parent := humanize(parentPath)

	switch {
	case name == "id" && parentPath == "":
		return "ID of the resource"
	case name == "id":
		return parent + " ID"
	case strings.HasSuffix(name, "_id"):
		return humanize(name)
	case name == "enabled":
		return "Whether " + parent + " is enabled"
	case name == "url":
		return parent + " URL"
	case strings.HasSuffix(name, "_url"):
		return humanize(name)
	case name == "name" && parent != "":
		return "Name of the " + lowFirst(parent)
	case name == "name":
		return "Name"
	case strings.HasSuffix(name, "_name"):
		return humanize(name)
	case name == "arn":
		return "ARN for the " + lowFirst(parent)
	case name == "email":
		return "Email address for the " + lowFirst(parent)
	case name == "created_at":
		return "Timestamp of when the " + lowFirst(parent) + " was created"
	case name == "updated_at":
		return "Timestamp of when the " + lowFirst(parent) + " was last updated"
	case name == "deleted_at":
		return "Timestamp of when the " + lowFirst(parent) + " was deleted"
	case strings.HasSuffix(name, "_at"):
		return humanize(name)
	default:
		// no name-pattern match — try AttrType-specific defaults below
	}

	switch attrType {
	case AttrTypeSingleNested:
		return humanize(name) + " configuration"
	case AttrTypeListNested:
		return "List of " + lowFirst(humanize(name))
	case AttrTypeList:
		return humanize(name)
	case AttrTypeMap, AttrTypeMapNested:
		return humanize(name)
	case AttrTypeBool:
		return "Whether " + lowFirst(humanize(name)) + " is enabled"
	default:
		// no AttrType-specific description — fall through to generic humanize(name)
	}

	return humanize(name)
}

func humanize(s string) string {
	if s == "" {
		return ""
	}

	if i := strings.LastIndex(s, "."); i >= 0 {
		s = s[i+1:]
	}
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if abbr, ok := abbreviations[strings.ToLower(p)]; ok {
			parts[i] = abbr
		} else if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

func lowFirst(s string) string {
	if s == "" {
		return s
	}

	firstWord := strings.SplitN(s, " ", 2)[0]
	for _, abbr := range abbreviations {
		if firstWord == abbr {
			return s
		}
	}
	return strings.ToLower(s[:1]) + s[1:]
}
