// Copyright 2025 Redpanda Data, Inc.
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

package acc

import (
	"fmt"
	"os"
	"strings"
)

// DataplaneGates records which dataplane resources a fixture declares, so an
// acceptance runner that drops a resource drops its steps with it.
//
// Coverage placement: serverless is the default home because it stands up in
// seconds. Dedicated alone carries role and role_assignment because the console
// SecurityService is Unimplemented on serverless. BYOC carries only topic, as a
// canary that the dataplane is reachable. BYOVPC carries none: its dataplane is
// reachable only from inside the VPC, which CI cannot offer. This lives here
// rather than in examples/ because those files are embedded into published docs.
type DataplaneGates struct {
	User              bool
	ACL               bool
	SchemaRegistryACL bool
	Schema            bool
	Role              bool
	Topic             bool
	Pipeline          bool
	PasswordWo        bool
	Secret            bool

	// TopicConfigurable is set when the fixture threads var.topic_configuration
	// into the topic. The topic-config regression steps mutate that variable, so
	// a fixture that hardcodes its configuration cannot satisfy them — gating on
	// Topic alone would emit steps whose assertions can never pass.
	TopicConfigurable bool

	// ACLResourceName is whichever ACL the fixture declares.
	ACLResourceName string
}

// Any reports whether the fixture declares any dataplane resource at all.
func (g DataplaneGates) Any() bool {
	return g.User || g.ACL || g.SchemaRegistryACL || g.Schema ||
		g.Role || g.Topic || g.Pipeline || g.Secret
}

// SniffDataplaneGates reads dir/main.tf and reports the dataplane resources it
// declares.
func SniffDataplaneGates(dir string) (DataplaneGates, error) {
	content, err := os.ReadFile(dir + "/main.tf") // #nosec G304 -- dir is controlled by test constants
	if err != nil {
		return DataplaneGates{}, fmt.Errorf("failed to read fixture main.tf: %w", err)
	}
	return ParseDataplaneGates(string(content)), nil
}

// ParseDataplaneGates reports the dataplane resources declared in HCL source.
func ParseDataplaneGates(hcl string) DataplaneGates {
	// Comment-stripped first: a commented-out resource declares nothing, and
	// matching it would gate steps on for a fixture that cannot satisfy them.
	tf := StripHCLComments(hcl)

	g := DataplaneGates{
		User:              strings.Contains(tf, `resource "redpanda_user" "test"`),
		SchemaRegistryACL: strings.Contains(tf, `resource "redpanda_schema_registry_acl" "read_product"`),
		Schema:            strings.Contains(tf, `resource "redpanda_schema" "user_schema"`),
		Role:              strings.Contains(tf, `resource "redpanda_role" "developer"`),
		Topic:             strings.Contains(tf, `resource "redpanda_topic" "test"`),
		Pipeline:          strings.Contains(tf, `resource "redpanda_pipeline" "test"`),
		PasswordWo:        strings.Contains(tf, "var.user_password_wo"),
		Secret:            strings.Contains(tf, `resource "redpanda_secret" "test"`),
		TopicConfigurable: strings.Contains(tf, "var.topic_configuration"),
	}

	// Resolve the ACL by name rather than defaulting: the flip steps assert
	// against whichever ACL this returns, so naming one the fixture does not
	// declare would assert against nothing. First match wins.
	for _, candidate := range []struct {
		label string
		name  string
	}{
		{"topic_access", TopicAccessACLResourceName},
		{"cluster_admin", ClusterAdminACLResourceName},
		{"role_topic_read", RoleTopicReadACLResourceName},
	} {
		if strings.Contains(tf, `resource "redpanda_acl" "`+candidate.label+`"`) {
			g.ACL = true
			g.ACLResourceName = candidate.name
			break
		}
	}
	return g
}

// StripHCLComments removes whole-line `#` and `//` comments so resource sniffing
// sees only live declarations.
func StripHCLComments(s string) string {
	var b strings.Builder
	for line := range strings.SplitSeq(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") {
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}
