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

package serviceaccount

import (
	"testing"

	iamv1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/iam/v1"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Update payload must carry the original `name` when only `description`
// changed: ServiceAccountUpdate.name's buf.validate rules run on the whole
// message regardless of FieldMask, so a zero name fails string.min_len + regex.
func TestUnit_ServiceAccount_Update_PreservesNameOnDescriptionOnlyChange(t *testing.T) {
	state := &iamv1.ServiceAccountUpdate{
		Name:        "svc-acct",
		Description: "old",
	}
	plan := &iamv1.ServiceAccountUpdate{
		Name:        "svc-acct",
		Description: "new",
	}

	payload, mask := utils.PlanPayloadWithUpdateMask(plan, state)

	require.NotNil(t, payload)
	assert.Equal(t, "svc-acct", payload.GetName(),
		"name must remain populated on a description-only update — otherwise the server's string.min_len rule rejects the request")
	assert.Equal(t, "new", payload.GetDescription())

	require.NotNil(t, mask)
	assert.Equal(t, []string{"description"}, mask.Paths,
		"mask must only flag the changed field")
}
