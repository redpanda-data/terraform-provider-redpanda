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

var commonDescriptions = map[string]string{
	"allow_deletion":      "Resource will only be deleted when allow_deletion is set to true. Otherwise deletion will fail with a related error.",
	"cluster_api_url":     "The cluster API URL.",
	"id":                  "Unique identifier of the resource.",
	"password":            "Password for authentication.",
	"password_wo":         "Password for authentication (write-only, not stored in state). Requires Terraform 1.11+.",
	"password_wo_version": "Version number for write-only password. Increment to trigger a password update when using password_wo.",
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
