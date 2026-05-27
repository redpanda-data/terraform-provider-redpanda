// Copyright 2026 Redpanda Data, Inc.
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

package validators

import (
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// principalPrefixPattern enforces the dataplane SecurityService contract that
// role-assignment principals carry a type prefix. Valid prefixes are
// "User:" for end users and "Group:" for IdP groups. "RedpandaRole:" is an
// ACL-side principal, not a role member, and is rejected here.
var principalPrefixPattern = regexp.MustCompile(`^(User|Group):.+$`)

// PrincipalPrefix returns a validator that rejects role-assignment
// principals lacking a "User:" or "Group:" prefix. Without this, bare
// principals (e.g. "alice") are silently canonicalized server-side to
// "User:alice", producing Read drift the provider can't reconcile.
func PrincipalPrefix() validator.String {
	return stringvalidator.RegexMatches(
		principalPrefixPattern,
		`principal must be prefixed with "User:" or "Group:"`,
	)
}

// CanonicalizePrincipal returns the canonical form a Kafka-style principal
// would take on the wire: an input already carrying a "User:", "Group:",
// or "RedpandaRole:" prefix is returned unchanged; any other input has
// "User:" prepended. Empty string is returned unchanged so callers can
// preserve "no value yet" semantics — the validator rejects empty before
// runtime canonicalization ever sees one.
//
// Used by the role_assignment Read and Delete handlers to bring legacy
// bare-form state in line with the server's canonical representation. The
// validator and this function share the same prefix-detection rule.
func CanonicalizePrincipal(s string) string {
	if s == "" {
		return s
	}
	switch {
	case strings.HasPrefix(s, "User:"),
		strings.HasPrefix(s, "Group:"),
		strings.HasPrefix(s, "RedpandaRole:"):
		return s
	default:
		return "User:" + s
	}
}
