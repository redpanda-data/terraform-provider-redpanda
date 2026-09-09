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

package schemagen

import "testing"

// TestToProtoGoName pins protoc-gen-go's GoCamelCase rules, which the
// generated accessors must match byte for byte: an underscore before a digit
// survives (rp_0_pods is Rp_0Pods), one before a lowercase letter is dropped,
// and a letter after a digit is upper-cased.
func TestToProtoGoName(t *testing.T) {
	cases := map[string]string{
		"rp_0_pods":                              "Rp_0Pods",
		"rp_2_vnet":                              "Rp_2Vnet",
		"rp_egress_vnet":                         "RpEgressVnet",
		"sys_pods":                               "SysPods",
		"k8s_master_ipv4_range":                  "K8SMasterIpv4Range",
		"psc_v2_nat_subnet_name":                 "PscV2NatSubnetName",
		"redpanda_connect_api_assigned_identity": "RedpandaConnectApiAssignedIdentity",
		"name":                                   "Name",
	}
	for in, want := range cases {
		if got := toProtoGoName(in); got != want {
			t.Errorf("toProtoGoName(%q) = %q, want %q", in, got, want)
		}
	}
}
