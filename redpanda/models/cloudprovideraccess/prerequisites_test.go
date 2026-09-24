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
	"testing"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestFlattenPrerequisites(t *testing.T) {
	resp := &controlplanev1.GetCloudProviderAccessPrerequisitesResponse{}
	resp.SetAws(&controlplanev1.AWSCloudProviderAccessPrerequisites{
		ExternalId:   "a845616f-0484-4506-9638-45fe28f34865",
		PrincipalArn: "arn:aws:iam::123456789012:role/redpanda-ops-worker",
	})
	m := &PrerequisitesModel{CloudProvider: types.StringValue("aws")}
	FlattenPrerequisites(m, resp)
	if m.AWS == nil {
		t.Fatal("aws is nil, want the AWS prerequisites")
	}
	if got := m.AWS.PrincipalARN.ValueString(); got != "arn:aws:iam::123456789012:role/redpanda-ops-worker" {
		t.Errorf("principal_arn = %q", got)
	}
	if got := m.AWS.ExternalID.ValueString(); got != "a845616f-0484-4506-9638-45fe28f34865" {
		t.Errorf("external_id = %q", got)
	}
	if got := m.CloudProvider.ValueString(); got != "aws" {
		t.Errorf("cloud_provider = %q, want the configured value kept", got)
	}
}

func TestFlattenPrerequisitesWithoutAWS(t *testing.T) {
	m := &PrerequisitesModel{AWS: &AWSPrerequisitesModel{}}
	FlattenPrerequisites(m, &controlplanev1.GetCloudProviderAccessPrerequisitesResponse{})
	if m.AWS != nil {
		t.Fatalf("aws = %+v, want nil when the response has no AWS arm", m.AWS)
	}
}
