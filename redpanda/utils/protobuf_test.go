package utils

import (
	"sort"
	"testing"

	controlplanev1beta2 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
	"github.com/stretchr/testify/assert"
)

func TestGenerateProtobufDiffAndUpdateMask(t *testing.T) {
	testCases := []struct {
		name         string
		old          *controlplanev1beta2.ClusterUpdate
		new          *controlplanev1beta2.ClusterUpdate
		expected     *controlplanev1beta2.ClusterUpdate
		expectedMask []string
	}{
		{
			name:         "empty messages",
			old:          &controlplanev1beta2.ClusterUpdate{},
			new:          &controlplanev1beta2.ClusterUpdate{},
			expected:     &controlplanev1beta2.ClusterUpdate{},
			expectedMask: nil,
		},
		{
			name: "same primitive field",
			old: &controlplanev1beta2.ClusterUpdate{
				Name: "testname",
			},
			new: &controlplanev1beta2.ClusterUpdate{
				Name: "testname",
			},
			expected:     &controlplanev1beta2.ClusterUpdate{},
			expectedMask: nil,
		},
		{
			name: "different name",
			old: &controlplanev1beta2.ClusterUpdate{
				Name: "testname",
			},
			new: &controlplanev1beta2.ClusterUpdate{
				Name: "testnamechanged",
			},
			expected: &controlplanev1beta2.ClusterUpdate{
				Name: "testnamechanged",
			},
			expectedMask: []string{"name"},
		},
		{
			name: "different name, same privatelink",
			old: &controlplanev1beta2.ClusterUpdate{
				Name: "testname",
				AwsPrivateLink: &controlplanev1beta2.AWSPrivateLinkSpec{
					Enabled: true,
				},
			},
			new: &controlplanev1beta2.ClusterUpdate{
				Name: "testnamechanged",
				AwsPrivateLink: &controlplanev1beta2.AWSPrivateLinkSpec{
					Enabled: true,
				},
			},
			expected: &controlplanev1beta2.ClusterUpdate{
				Name: "testnamechanged",
			},
			expectedMask: []string{"name"},
		},
		{
			name: "same name, different privatelink",
			old: &controlplanev1beta2.ClusterUpdate{
				Name: "testname",
			},
			new: &controlplanev1beta2.ClusterUpdate{
				Name: "testname",
				AwsPrivateLink: &controlplanev1beta2.AWSPrivateLinkSpec{
					Enabled: true,
				},
			},
			expected: &controlplanev1beta2.ClusterUpdate{
				AwsPrivateLink: &controlplanev1beta2.AWSPrivateLinkSpec{
					Enabled: true,
				},
			},
			expectedMask: []string{"aws_private_link"},
		},
		{
			name: "different name, different privatelink",
			old: &controlplanev1beta2.ClusterUpdate{
				Name: "testname",
			},
			new: &controlplanev1beta2.ClusterUpdate{
				Name: "testnamechanged",
				AwsPrivateLink: &controlplanev1beta2.AWSPrivateLinkSpec{
					Enabled: true,
				},
			},
			expected: &controlplanev1beta2.ClusterUpdate{
				Name: "testnamechanged",
				AwsPrivateLink: &controlplanev1beta2.AWSPrivateLinkSpec{
					Enabled: true,
				},
			},
			expectedMask: []string{"aws_private_link", "name"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result *controlplanev1beta2.ClusterUpdate // assert type is correct
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
