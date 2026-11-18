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

package enums_test

import (
	"testing"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	corev2 "buf.build/gen/go/redpandadata/core/protocolbuffers/go/redpanda/core/admin/v2"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils/enums"
	"github.com/stretchr/testify/assert"
)

// TestShadowLinkFilterEnumSpellings pins the contract between the schema
// OneOf validators on the shadowlink filter enums and the generated mappers:
// every spelling the schema accepts must round-trip String→proto→String
// unchanged, or applies fail with an inconsistent-result error. It also pins
// why the aliased and proto-long spellings are excluded from OneOf.
func TestShadowLinkFilterEnumSpellings(t *testing.T) {
	t.Run("advertised filter_type spellings round-trip", func(t *testing.T) {
		for _, s := range []string{"INCLUDE", "EXCLUDE"} {
			e := enums.StringToFilterType(s)
			assert.NotEqual(t, corev2.FilterType_FILTER_TYPE_UNSPECIFIED, e, "spelling %q maps to UNSPECIFIED", s)
			assert.Equal(t, s, enums.FilterTypeToString(e), "spelling %q does not round-trip", s)
		}
	})

	t.Run("advertised pattern_type spellings round-trip", func(t *testing.T) {
		for _, s := range []string{"LITERAL", "PREFIX"} {
			e := enums.StringToPatternType(s)
			assert.NotEqual(t, corev2.PatternType_PATTERN_TYPE_UNSPECIFIED, e, "spelling %q maps to UNSPECIFIED", s)
			assert.Equal(t, s, enums.PatternTypeToString(e), "spelling %q does not round-trip", s)
		}
	})

	t.Run("aliased PREFIXED cannot round-trip", func(t *testing.T) {
		e := enums.StringToPatternType("PREFIXED")
		assert.Equal(t, corev2.PatternType_PATTERN_TYPE_PREFIX, e)
		assert.Equal(t, "PREFIX", enums.PatternTypeToString(e))
	})

	t.Run("proto-long spellings coerce to UNSPECIFIED", func(t *testing.T) {
		assert.Equal(t, corev2.FilterType_FILTER_TYPE_UNSPECIFIED, enums.StringToFilterType("FILTER_TYPE_INCLUDE"))
		assert.Equal(t, corev2.PatternType_PATTERN_TYPE_UNSPECIFIED, enums.StringToPatternType("PATTERN_TYPE_LITERAL"))
	})
}

// TestAPIGatewayAccessSpellings pins the contract between the OneOf validator
// on redpanda_cluster.api_gateway_access and the mapper: each accepted
// spelling round-trips unchanged, and every other spelling is "not set", so
// the validator is the only thing standing between it and a dropped value.
func TestAPIGatewayAccessSpellings(t *testing.T) {
	for _, s := range []string{"NETWORK_ACCESS_MODE_PRIVATE", "NETWORK_ACCESS_MODE_PUBLIC"} {
		e := enums.StringToNetworkAccessMode(s)
		assert.NotEqual(t, controlplanev1.NetworkAccessMode_NETWORK_ACCESS_MODE_UNSPECIFIED, e, "spelling %q maps to UNSPECIFIED", s)
		assert.Equal(t, s, enums.NetworkAccessModeToString(e), "spelling %q does not round-trip", s)
	}
	for _, s := range []string{"private", "PRIVATE", "Public", "network_access_mode_private", ""} {
		assert.Equal(t, controlplanev1.NetworkAccessMode_NETWORK_ACCESS_MODE_UNSPECIFIED, enums.StringToNetworkAccessMode(s), "spelling %q", s)
	}
}
