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

package clustermask

import (
	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// cmrTopLevelPath is the bare field name GenerateProtobufDiffAndUpdateMask emits
// when the customer_managed_resources object differs. The control plane accepts
// CMR only at leaf granularity, so ExpandCustomerManagedResourceLeaves rewrites
// this into the specific changed leaf keys.
const cmrTopLevelPath = "customer_managed_resources"

// cmrLeaf pairs an updatable customer_managed_resources leaf's mask key (the
// cloudv2 pathMap key the control plane matches on) with the extractor that
// reads its scalar value from a CustomerManagedResourcesUpdate.
type cmrLeaf struct {
	path string
	get  func(*controlplanev1.CustomerManagedResourcesUpdate) string
}

// cmrUpdatableLeaves mirrors the control-plane-updatable CMR entries in
// cloudv2's pathMap / multiListenersPathMap (mapper.go). Only leaves our
// generated ExpandUpdate populates are listed. Every key is the public proto
// field name; cloudv2 maps the rpsql_* keys onto its internal oxla_* spec paths
// on its side (see mapper.go). Keep in sync with cloudv2 when bumping the pin.
var cmrUpdatableLeaves = []cmrLeaf{
	{"customer_managed_resources.gcp.psc_nat_subnet_name", func(c *controlplanev1.CustomerManagedResourcesUpdate) string {
		return c.GetGcp().GetPscNatSubnetName()
	}},
	{"customer_managed_resources.gcp.rpsql_api_service_account.email", func(c *controlplanev1.CustomerManagedResourcesUpdate) string {
		return c.GetGcp().GetRpsqlApiServiceAccount().GetEmail()
	}},
	{"customer_managed_resources.gcp.rpsql_service_account.email", func(c *controlplanev1.CustomerManagedResourcesUpdate) string {
		return c.GetGcp().GetRpsqlServiceAccount().GetEmail()
	}},
	{"customer_managed_resources.gcp.rpsql_cloud_storage_bucket.name", func(c *controlplanev1.CustomerManagedResourcesUpdate) string {
		return c.GetGcp().GetRpsqlCloudStorageBucket().GetName()
	}},
	{"customer_managed_resources.gcp.rpsql_secret_manager_prefix", func(c *controlplanev1.CustomerManagedResourcesUpdate) string {
		return c.GetGcp().GetRpsqlSecretManagerPrefix()
	}},
	{"customer_managed_resources.aws.redpanda_connect_node_group_instance_profile.arn", func(c *controlplanev1.CustomerManagedResourcesUpdate) string {
		return c.GetAws().GetRedpandaConnectNodeGroupInstanceProfile().GetArn()
	}},
	{"customer_managed_resources.aws.redpanda_connect_security_group.arn", func(c *controlplanev1.CustomerManagedResourcesUpdate) string {
		return c.GetAws().GetRedpandaConnectSecurityGroup().GetArn()
	}},
	{"customer_managed_resources.aws.rpsql_node_group_instance_profile.arn", func(c *controlplanev1.CustomerManagedResourcesUpdate) string {
		return c.GetAws().GetRpsqlNodeGroupInstanceProfile().GetArn()
	}},
	{"customer_managed_resources.aws.rpsql_security_group.arn", func(c *controlplanev1.CustomerManagedResourcesUpdate) string {
		return c.GetAws().GetRpsqlSecurityGroup().GetArn()
	}},
	{"customer_managed_resources.aws.rpsql_cloud_storage_bucket.arn", func(c *controlplanev1.CustomerManagedResourcesUpdate) string {
		return c.GetAws().GetRpsqlCloudStorageBucket().GetArn()
	}},
}

// CMRUpdatableLeafPaths returns the mask keys in cmrUpdatableLeaves. schemagen
// checks them against the update payload so a pin bump that renames or drops a
// leaf fails the build instead of silently dropping the edit at runtime.
func CMRUpdatableLeafPaths() []string {
	out := make([]string, 0, len(cmrUpdatableLeaves))
	for _, l := range cmrUpdatableLeaves {
		out = append(out, l.path)
	}
	return out
}

// ExpandCustomerManagedResourceLeaves rewrites, in place, a bare
// customer_managed_resources mask path into the specific leaf paths (cloudv2
// pathMap keys) whose value differs between plan and state. It is a no-op when
// the mask carries no CMR path. Immutable CMR leaves never reach here — they
// force RequiresReplace — so only the control-plane-updatable leaves are
// considered. See cmrUpdatableLeaves.
func ExpandCustomerManagedResourceLeaves(fm *fieldmaskpb.FieldMask, plan, state *controlplanev1.CustomerManagedResourcesUpdate) {
	if fm == nil {
		return
	}
	found := false
	for _, p := range fm.Paths {
		if p == cmrTopLevelPath {
			found = true
			break
		}
	}
	if !found {
		return
	}
	var changed []string
	for _, leaf := range cmrUpdatableLeaves {
		if leaf.get(plan) != leaf.get(state) {
			changed = append(changed, leaf.path)
		}
	}
	out := make([]string, 0, len(fm.Paths)+len(changed))
	for _, p := range fm.Paths {
		if p == cmrTopLevelPath {
			out = append(out, changed...)
			continue
		}
		out = append(out, p)
	}
	fm.Paths = out
}
