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

package network

import (
	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type cidrBlockProto interface {
	GetCidrBlock() string
	GetCustomerManagedResources() *controlplanev1.Network_CustomerManagedResources
}

func cidrBlockFromProto(proto cidrBlockProto) types.String {
	if proto.GetCustomerManagedResources() != nil && proto.GetCidrBlock() == "0.0.0.0/0" {
		return types.StringNull()
	}
	return types.StringValue(proto.GetCidrBlock())
}
