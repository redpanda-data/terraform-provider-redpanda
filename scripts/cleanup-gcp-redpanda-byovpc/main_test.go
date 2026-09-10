// Copyright 2026 Redpanda Data, Inc.
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

import "testing"

// Role IDs below are real, captured from hallowed-ray-376320 while cleaning up
// the exhausted 300-custom-role limit. One exemplar per family the BYOC agent
// and BYOVPC module actually emit.
func TestMatchesRedpandaRole(t *testing.T) {
	tests := []struct {
		name   string
		roleID string
		want   bool
	}{
		{"byoc agent", "RedpandaAgentd15gpcm3m3s60hr09300", true},
		{"byoc agent oxla storage", "RedpandaAgentOxlaStoraged7sfkn58nt7h826tjlq0", true},
		{"byoc utility", "redpandaUtilityD3pbdh3bahp0cr4s505gUstv", true},
		{"byoc operator", "redpandaOperatorD3pbdh3bahp0cr4s505gU894", true},
		{"byoc connect", "redpandaConnectD3pbdh3bahp0cr4s505g9sx5", true},
		{"byoc connect api", "redpandaConnectApiD3pbdh3bahp0cr4s505g32ja", true},
		{"byoc connectors", "redpandaConnectorsCtc6e82524aq005vbh3g4vsu", true},
		{"byoc console secret manager", "redpandaConsoleSecretManagerCtc6e82524aq005vbh3gGrpl", true},
		{"byoc cluster secrets reader", "redpandaclustersecretsreaderD3pbdh3bahp0cr4s505gRbdh", true},
		{"policy materializer", "policyMaterializerD9kaj3sm225c8b39qa7g", true},
		{"policy materializer list", "policyMaterializerListD9kaj3sm225c8b39qa7g", true},
		{"byovpc module agent", "redpanda_agent_role_123", true},
		{"byovpc module gke utility", "redpanda_gke_utility_role_123", true},
		{"byovpc test user", "byovpc_test_user_role_123", true},

		{"devex role is protected", "devexCustomRole", false},
		{"devex substring anywhere is protected", "redpandaDevexHelper", false},
		{"unrelated custom role", "billingViewer", false},
		{"unrelated role naming a cloud primitive", "customStorageAdmin", false},
		{"redpanda not at the start", "acmeRedpandaHelper", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesRedpandaRole(tt.roleID, "redpanda"); got != tt.want {
				t.Errorf("matchesRedpandaRole(%q) = %v, want %v", tt.roleID, got, tt.want)
			}
		})
	}
}

func TestMatchesRedpandaRoleIsCaseInsensitive(t *testing.T) {
	// the agent mixes cases across families, so a case-sensitive prefix check
	// silently skips every RedpandaAgent* role
	for _, roleID := range []string{
		"RedpandaAgentd15gpcm3m3s60hr09300",
		"redpandaagentd15gpcm3m3s60hr09300",
		"REDPANDAAGENTD15GPCM3M3S60HR09300",
	} {
		if !matchesRedpandaRole(roleID, "redpanda") {
			t.Errorf("matchesRedpandaRole(%q) = false, want true", roleID)
		}
	}
}

func TestMatchesRedpandaRoleHonorsCommonPrefix(t *testing.T) {
	if !matchesRedpandaRole("acmeThing", "acme") {
		t.Error(`matchesRedpandaRole("acmeThing", "acme") = false, want true`)
	}
	if matchesRedpandaRole("acmeThing", "redpanda") {
		t.Error(`matchesRedpandaRole("acmeThing", "redpanda") = true, want false`)
	}
}

func TestRoleIDFromName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			"qualified project role",
			"projects/hallowed-ray-376320/roles/RedpandaAgentd15gpcm3m3s60hr09300",
			"RedpandaAgentd15gpcm3m3s60hr09300",
		},
		{"bare id", "RedpandaAgentd15gpcm3m3s60hr09300", "RedpandaAgentd15gpcm3m3s60hr09300"},
		{"unseparated input returned unchanged", "roles/editor", "roles/editor"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := roleIDFromName(tt.in); got != tt.want {
				t.Errorf("roleIDFromName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
