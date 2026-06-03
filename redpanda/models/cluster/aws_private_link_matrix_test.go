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

package cluster

import (
	"context"
	"testing"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Regression matrix for aws_private_link's Flatten behavior under the
// Optional+Computed + UseStateForUnknown contract. The framework's plan
// modifier handles the apply-time consistency check, so Flatten always
// defers to the proto when proto.HasAwsPrivateLink() is true — including
// the case where prev.AWSPrivateLink is Null (the framework's zero-init
// on the first Read after ImportState).

func buildClusterWithAWSPL(has bool) *controlplanev1.Cluster {
	c := &controlplanev1.Cluster{}
	if has {
		c.AwsPrivateLink = &controlplanev1.Cluster_AWSPrivateLink{
			Enabled:           true,
			ConnectConsole:    true,
			AllowedPrincipals: []string{"arn:aws:iam::123:root"},
		}
	}
	return c
}

func TestFlattenAWSPrivateLink_NilPrev_UsesProto(t *testing.T) {
	proto := buildClusterWithAWSPL(true)
	out, diags := Flatten(context.Background(), proto, nil)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags.Errors())
	}
	if out.AWSPrivateLink.IsNull() {
		t.Fatal("nil prev with proto.HasAwsPrivateLink()=true: got Null, want populated object")
	}
}

func TestFlattenAWSPrivateLink_NilPrev_NoProto_ReturnsNull(t *testing.T) {
	proto := buildClusterWithAWSPL(false)
	out, diags := Flatten(context.Background(), proto, nil)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags.Errors())
	}
	if !out.AWSPrivateLink.IsNull() {
		t.Fatal("nil prev with no proto: got populated, want Null")
	}
}

// Post-import regression case. The framework writes id+allow_deletion in
// ImportState and zero-initializes everything else, so prev.AWSPrivateLink
// is Null on the first Read. Optional+Computed lets Flatten defer to the
// proto here.
func TestFlattenAWSPrivateLink_NullPrev_HasProto_UsesProto(t *testing.T) {
	proto := buildClusterWithAWSPL(true)
	prev := &ResourceModel{AWSPrivateLink: types.ObjectNull(AWSPrivateLinkAttrTypes())}
	out, diags := Flatten(context.Background(), proto, prev)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags.Errors())
	}
	if out.AWSPrivateLink.IsNull() {
		t.Fatal("null prev with proto: got Null, want populated (Optional+Computed defers to proto)")
	}
}

// Refresh-with-no-proto: server stopped returning the block, but prev has
// a populated value. The else-branch in Flatten preserves prev so plan
// doesn't churn the field to null between refreshes.
func TestFlattenAWSPrivateLink_PopulatedPrev_NoProto_PreservesPrev(t *testing.T) {
	populated, d := Flatten(context.Background(), buildClusterWithAWSPL(true), nil)
	if d.HasError() {
		t.Fatalf("unexpected setup diagnostics: %v", d.Errors())
	}
	prev := &ResourceModel{AWSPrivateLink: populated.AWSPrivateLink}
	out, diags := Flatten(context.Background(), buildClusterWithAWSPL(false), prev)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags.Errors())
	}
	if out.AWSPrivateLink.IsNull() {
		t.Fatal("populated prev with no proto: got Null, want preserved prev value")
	}
}

// supported_regions must flatten a nil/empty server slice to Null, never to an
// empty list. The provider's old conv produced [] here, which mismatched the
// null planned value and tripped "Provider produced inconsistent result after
// apply: was null, but now cty.ListValEmpty". A proto3 repeated field
// also can't distinguish empty from absent on the wire, so the server's "[]"
// arrives as a nil slice — this is the only boundary the provider controls.
func TestFlattenAWSPrivateLink_SupportedRegions_NilToNull(t *testing.T) {
	pl := &controlplanev1.Cluster_AWSPrivateLink{Enabled: true}
	out, diags := FlattenAWSPrivateLink(context.Background(), pl, nil)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags.Errors())
	}
	if !out.SupportedRegions.IsNull() {
		t.Fatalf("nil supported_regions: got %v, want Null (must not be [])", out.SupportedRegions)
	}
}

func TestFlattenAWSPrivateLink_SupportedRegions_PopulatedToList(t *testing.T) {
	pl := &controlplanev1.Cluster_AWSPrivateLink{
		Enabled:          true,
		SupportedRegions: []string{"us-east-1", "us-west-2"},
	}
	out, diags := FlattenAWSPrivateLink(context.Background(), pl, nil)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags.Errors())
	}
	if out.SupportedRegions.IsNull() {
		t.Fatal("populated supported_regions: got Null, want list")
	}
	var got []string
	if d := out.SupportedRegions.ElementsAs(context.Background(), &got, false); d.HasError() {
		t.Fatalf("ElementsAs: %v", d.Errors())
	}
	if len(got) != 2 || got[0] != "us-east-1" || got[1] != "us-west-2" {
		t.Fatalf("supported_regions = %v, want [us-east-1 us-west-2]", got)
	}
}
