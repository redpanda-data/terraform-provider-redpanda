package utils

import (
	"sort"
	"testing"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1beta2"
	"github.com/stretchr/testify/assert"
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
