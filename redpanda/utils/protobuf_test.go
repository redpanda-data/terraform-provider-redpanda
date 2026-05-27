package utils

import (
	"sort"
	"testing"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateProtobufDiffAndUpdateMask(t *testing.T) {
	testCases := []struct {
		name         string
		old          *controlplanev1.ClusterUpdate
		new          *controlplanev1.ClusterUpdate
		expected     *controlplanev1.ClusterUpdate
		expectedMask []string
	}{
		{
			name:         "empty messages",
			old:          &controlplanev1.ClusterUpdate{},
			new:          &controlplanev1.ClusterUpdate{},
			expected:     &controlplanev1.ClusterUpdate{},
			expectedMask: nil,
		},
		{
			name: "same primitive field",
			old: &controlplanev1.ClusterUpdate{
				Name: "testname",
			},
			new: &controlplanev1.ClusterUpdate{
				Name: "testname",
			},
			expected:     &controlplanev1.ClusterUpdate{},
			expectedMask: nil,
		},
		{
			name: "different name",
			old: &controlplanev1.ClusterUpdate{
				Name: "testname",
			},
			new: &controlplanev1.ClusterUpdate{
				Name: "testnamechanged",
			},
			expected: &controlplanev1.ClusterUpdate{
				Name: "testnamechanged",
			},
			expectedMask: []string{"name"},
		},
		{
			name: "different name, same privatelink",
			old: &controlplanev1.ClusterUpdate{
				Name: "testname",
				AwsPrivateLink: &controlplanev1.AWSPrivateLinkSpec{
					Enabled: true,
				},
			},
			new: &controlplanev1.ClusterUpdate{
				Name: "testnamechanged",
				AwsPrivateLink: &controlplanev1.AWSPrivateLinkSpec{
					Enabled: true,
				},
			},
			expected: &controlplanev1.ClusterUpdate{
				Name: "testnamechanged",
			},
			expectedMask: []string{"name"},
		},
		{
			name: "same name, different privatelink",
			old: &controlplanev1.ClusterUpdate{
				Name: "testname",
			},
			new: &controlplanev1.ClusterUpdate{
				Name: "testname",
				AwsPrivateLink: &controlplanev1.AWSPrivateLinkSpec{
					Enabled: true,
				},
			},
			expected: &controlplanev1.ClusterUpdate{
				AwsPrivateLink: &controlplanev1.AWSPrivateLinkSpec{
					Enabled: true,
				},
			},
			expectedMask: []string{"aws_private_link"},
		},
		{
			name: "different name, different privatelink",
			old: &controlplanev1.ClusterUpdate{
				Name: "testname",
			},
			new: &controlplanev1.ClusterUpdate{
				Name: "testnamechanged",
				AwsPrivateLink: &controlplanev1.AWSPrivateLinkSpec{
					Enabled: true,
				},
			},
			expected: &controlplanev1.ClusterUpdate{
				Name: "testnamechanged",
				AwsPrivateLink: &controlplanev1.AWSPrivateLinkSpec{
					Enabled: true,
				},
			},
			expectedMask: []string{"aws_private_link", "name"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result *controlplanev1.ClusterUpdate // assert type is correct
			result, resultMask := GenerateProtobufDiffAndUpdateMask(tc.new, tc.old)
			assert.EqualExportedValues(t, tc.expected, result)

			var resultPaths []string
			if resultMask != nil {
				resultPaths = resultMask.Paths
				sort.Strings(resultPaths)
			}
			sort.Strings(tc.expectedMask)
			assert.Equal(t, tc.expectedMask, resultPaths)
		})
	}
}

// Returned payload must be the plan message untouched (full field set)
// while the mask names only the diffed fields — required for APIs whose
// buf.validate.field rules run on the whole message regardless of FieldMask.
func TestPlanPayloadWithUpdateMask(t *testing.T) {
	t.Run("plan payload preserved when unchanged field is required", func(t *testing.T) {
		// name must stay populated when only aws_private_link changes —
		// sparse helper would zero it out; this helper must not.
		old := &controlplanev1.ClusterUpdate{Name: "svc"}
		newMsg := &controlplanev1.ClusterUpdate{
			Name:           "svc",
			AwsPrivateLink: &controlplanev1.AWSPrivateLinkSpec{Enabled: true},
		}

		payload, mask := PlanPayloadWithUpdateMask(newMsg, old)

		// Payload is the plan message — same pointer, no field stripping.
		assert.Same(t, newMsg, payload, "PlanPayloadWithUpdateMask must return the plan message unchanged")
		assert.Equal(t, "svc", payload.GetName(), "Name must remain populated")
		assert.True(t, payload.GetAwsPrivateLink().GetEnabled(), "AwsPrivateLink must remain populated")

		// Mask names only the diffed field.
		require.NotNil(t, mask)
		assert.Equal(t, []string{"aws_private_link"}, mask.Paths)
	})

	t.Run("mask paths match GenerateProtobufDiffAndUpdateMask", func(t *testing.T) {
		// The new helper reuses the sparse helper's mask-building logic.
		// Equivalence on the mask side is the contract we want to lock in
		// — only the payload shape differs.
		old := &controlplanev1.ClusterUpdate{Name: "old"}
		newMsg := &controlplanev1.ClusterUpdate{
			Name:           "new",
			AwsPrivateLink: &controlplanev1.AWSPrivateLinkSpec{Enabled: true},
		}

		_, sparseMask := GenerateProtobufDiffAndUpdateMask(newMsg, old)
		_, fullMask := PlanPayloadWithUpdateMask(newMsg, old)

		sparsePaths := append([]string{}, sparseMask.Paths...)
		fullPaths := append([]string{}, fullMask.Paths...)
		sort.Strings(sparsePaths)
		sort.Strings(fullPaths)
		assert.Equal(t, sparsePaths, fullPaths)
	})

	t.Run("no diff produces empty mask but preserves full payload", func(t *testing.T) {
		old := &controlplanev1.ClusterUpdate{Name: "svc"}
		newMsg := &controlplanev1.ClusterUpdate{Name: "svc"}

		payload, mask := PlanPayloadWithUpdateMask(newMsg, old)

		assert.Same(t, newMsg, payload)
		assert.Equal(t, "svc", payload.GetName(), "Name preserved even when nothing changed")
		require.NotNil(t, mask)
		assert.Empty(t, mask.Paths)
	})
}
