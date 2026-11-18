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
	"slices"

	controlplanev1 "buf.build/gen/go/redpandadata/cloud/protocolbuffers/go/redpanda/api/controlplane/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	gatewayUnspecified = controlplanev1.NetworkAccessMode_NETWORK_ACCESS_MODE_UNSPECIFIED
	gatewayPrivate     = controlplanev1.NetworkAccessMode_NETWORK_ACCESS_MODE_PRIVATE
	gatewayPublic      = controlplanev1.NetworkAccessMode_NETWORK_ACCESS_MODE_PUBLIC
)

// createAPIGatewayAccess mirrors the create path of cloudv2's
// handleAPIGatewayAccess, the console defaulter and validateConsole: an
// unspecified mode means "not set" and resolves to private when the cluster
// has a private Kafka listener, public otherwise, so a cluster created here
// never reads back unspecified.
func (f *ClusterFake) createAPIGatewayAccess(in *controlplanev1.ClusterCreate) (controlplanev1.NetworkAccessMode, error) {
	mode := in.GetApiGatewayAccess()
	if err := f.apiGatewayAccessSettable(mode); err != nil {
		return mode, err
	}
	privateAccess := in.GetConnectionType() == connTypePrivate
	if conns := in.GetKafkaApi().GetConnections(); len(conns) > 0 {
		_, privateAccess = connectionPresence(conns)
	}
	if mode == gatewayUnspecified {
		mode = gatewayPublic
		if privateAccess {
			mode = gatewayPrivate
		}
	}
	return mode, validateAPIGatewayAccess(mode, privateAccess)
}

// updateAPIGatewayAccess returns the mode an update leaves the cluster with.
// It mirrors the update path: an unspecified mode under the mask keeps the stored one, and
// the result is judged against the listeners the update leaves the cluster
// with, so it runs after the connections update. In an organization with
// dual listeners enabled the control plane moves the gateway of a public-only
// cluster to public on every update (DefaultConsoleConnectivity) before it
// validates, so a private gateway requested there is dropped, not rejected.
func (f *ClusterFake) updateAPIGatewayAccess(cl *controlplanev1.Cluster, upd *controlplanev1.ClusterUpdate, paths []string) (controlplanev1.NetworkAccessMode, error) {
	mode := cl.GetApiGatewayAccess()
	if err := f.apiGatewayAccessSettable(upd.GetApiGatewayAccess()); err != nil {
		return mode, err
	}
	masked := slices.Contains(paths, "api_gateway_access")
	if masked && upd.GetApiGatewayAccess() != gatewayUnspecified {
		mode = upd.GetApiGatewayAccess()
	}
	privateAccess := cl.GetConnectionType() == connTypePrivate
	if !f.PublicPrivateListenersDisabled && !privateAccess {
		mode = gatewayPublic
	}
	if err := validateAPIGatewayAccess(mode, privateAccess); err != nil {
		return mode, err
	}
	if masked && mode == gatewayPublic && f.NetworkLookup != nil &&
		f.NetworkLookup(cl.GetNetworkId()).GetEgressSpec().GetCloudProvider() != nil {
		return mode, status.Error(codes.InvalidArgument, "Network has an egress spec, which requires a private cluster")
	}
	return mode, nil
}

// apiGatewayAccessSettable mirrors the feature gate in cloudv2's
// handleAPIGatewayAccess, which looks at the request value alone: the mask
// plays no part, and an unspecified mode always passes.
func (f *ClusterFake) apiGatewayAccessSettable(requested controlplanev1.NetworkAccessMode) error {
	if requested != gatewayUnspecified && f.SetAPIGatewayAccessDisabled {
		return status.Error(codes.PermissionDenied, "Setting API gateway access is not enabled for this organization")
	}
	return nil
}

// validateAPIGatewayAccess mirrors cloudv2's validateConsole.
func validateAPIGatewayAccess(mode controlplanev1.NetworkAccessMode, privateAccess bool) error {
	if mode == gatewayPrivate && !privateAccess {
		return status.Error(codes.InvalidArgument, "API gateway access must not be private for clusters with public network")
	}
	return nil
}
