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

package fakes

import (
	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// validateCreateCustomerManagedResources mirrors cloudv2's
// validateCustomerManagedResources (controlplane-api redpanda_service.go) on
// create. nw is the cluster's network when the fake can resolve it; nil skips
// the cross-resource rules so a test that seeds only clusters still creates.
func validateCreateCustomerManagedResources(in *controlplanev1.ClusterCreate, nw *controlplanev1.Network) error {
	cmr := in.GetCustomerManagedResources()
	if nw != nil && nw.GetCustomerManagedResources() != nil && cmr == nil {
		return status.Errorf(codes.InvalidArgument,
			"cluster does not have customer managed resources but network %v has customer managed resources", nw.GetId())
	}
	if cmr == nil {
		return nil
	}
	if in.GetType() != controlplanev1.Cluster_TYPE_BYOC {
		return status.Errorf(codes.InvalidArgument,
			"expect cluster_type to be CLUSTER_TYPE_BYOC if customer_managed_resources is set, actual cluster type: %v", in.GetType())
	}
	if cmr.GetGcp() != nil && in.GetCloudProvider() != controlplanev1.CloudProvider_CLOUD_PROVIDER_GCP {
		return status.Errorf(codes.InvalidArgument,
			"provider is set to %v, but customer_managed_resources is for different provider", in.GetCloudProvider())
	}
	if nw != nil && nw.GetCustomerManagedResources() == nil {
		return status.Errorf(codes.InvalidArgument,
			"cluster has customer managed resources but its network %v does not have customer managed resources", nw.GetId())
	}
	return nil
}
