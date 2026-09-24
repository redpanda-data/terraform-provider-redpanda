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

package utils

import (
	"context"
	"time"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
)

// byocAgentNetworkReadTimeout bounds the retries of a transient network read
// that decides who runs a BYOC cluster's agent.
const byocAgentNetworkReadTimeout = 2 * time.Minute

// ByocAgentCanBeRedpandaManaged reports whether a cluster of type t on cloud
// provider cp can be on a network with a cloud provider access. Only AWS BYOC
// networks carry one, so every other cluster keeps running the byoc plugin
// locally, or has no agent, without a network read.
func ByocAgentCanBeRedpandaManaged(t controlplanev1.Cluster_Type, cp controlplanev1.CloudProvider) bool {
	return t == controlplanev1.Cluster_TYPE_BYOC && cp == controlplanev1.CloudProvider_CLOUD_PROVIDER_AWS
}

// NetworkUsesCloudProviderAccess reads the network once and reports whether it
// was created with a cloud provider access. The control plane then provisions
// and destroys the BYOC agent of the clusters on it, and running rpk byoc for
// them would act with the caller's own cloud credentials instead of the role
// Redpanda assumes.
func NetworkUsesCloudProviderAccess(ctx context.Context, client cloud.CpClientSet, networkID string) (bool, error) {
	nw, err := client.NetworkForID(ctx, networkID)
	if err != nil {
		return false, err
	}
	return nw.GetCloudProviderAccessId() != "", nil
}

// ByocAgentManagedByRedpanda reports whether the control plane runs the agent
// of a cluster of type t on cloud provider cp in the network, retrying a
// transient network read. Clusters that cannot be on a cloud provider access
// network answer false without a read.
func ByocAgentManagedByRedpanda(ctx context.Context, client cloud.CpClientSet, t controlplanev1.Cluster_Type, cp controlplanev1.CloudProvider, networkID string) (bool, error) {
	if !ByocAgentCanBeRedpandaManaged(t, cp) {
		return false, nil
	}
	var managed bool
	err := Retry(ctx, byocAgentNetworkReadTimeout, func() *RetryError {
		var err error
		managed, err = NetworkUsesCloudProviderAccess(ctx, client, networkID)
		switch {
		case err == nil:
			return nil
		case IsTransientServerError(err):
			return RetryableError(err)
		default:
			return NonRetryableError(err)
		}
	})
	return managed, err
}
