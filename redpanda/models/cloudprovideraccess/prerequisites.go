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

package cloudprovideraccess

import (
	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// PrerequisitesModel is the redpanda_cloud_provider_access_prerequisites
// data source model.
type PrerequisitesModel struct {
	CloudProvider types.String           `tfsdk:"cloud_provider"`
	AWS           *AWSPrerequisitesModel `tfsdk:"aws"`
}

// AWSPrerequisitesModel holds the values an AWS IAM role trust policy needs
// before a cloud provider access can use the role.
type AWSPrerequisitesModel struct {
	PrincipalARN types.String `tfsdk:"principal_arn"`
	ExternalID   types.String `tfsdk:"external_id"`
}

// FlattenPrerequisites sets the provider-specific block from the response,
// leaving the blocks of other providers null.
func FlattenPrerequisites(m *PrerequisitesModel, resp *controlplanev1.GetCloudProviderAccessPrerequisitesResponse) {
	m.AWS = nil
	if aws := resp.GetAws(); aws != nil {
		m.AWS = &AWSPrerequisitesModel{
			PrincipalARN: types.StringValue(aws.GetPrincipalArn()),
			ExternalID:   types.StringValue(aws.GetExternalId()),
		}
	}
}
