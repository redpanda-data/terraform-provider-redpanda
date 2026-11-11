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
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	grpccodes "google.golang.org/grpc/codes"
)

// ControlPlaneGone reports whether err, returned by a control-plane call
// addressed to the resource id, means that resource no longer exists.
//
// The control plane checks the caller's permission on a resource before it
// looks the resource up, and a deleted resource has no permissions left, so
// what comes back for it is PermissionDenied naming the resource, not NotFound.
// That answer is identical to the one for a resource the credentials cannot
// read, so it is accepted as gone only together with a warning that says so.
// A denial that does not name id (an organization-wide one, say) is not.
func ControlPlaneGone(kind, id string, err error) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics
	if err == nil {
		return false, diags
	}
	if IsNotFound(err) {
		return true, diags
	}
	if !DeniedOnResource(id, err) {
		return false, diags
	}
	diags.AddWarning(
		fmt.Sprintf("%s %s is being treated as deleted", kind, id),
		deniedMeansGoneOrUnreadable+" Terraform is dropping it from state. If it still exists, grant these credentials access to it and import it again.",
	)
	return true, diags
}

const deniedMeansGoneOrUnreadable = "Redpanda Cloud answered PermissionDenied for it, which is its answer both for a resource that has been deleted and for one these credentials cannot read."

// DeniedOnResource reports whether err is the control plane's PermissionDenied
// for the resource id itself.
func DeniedOnResource(id string, err error) bool {
	if id == "" || err == nil {
		return false
	}
	st, ok := serverStatus(err)
	if !ok || st.Code() != grpccodes.PermissionDenied {
		return false
	}
	for _, d := range st.Details() {
		if info, ok := d.(*errdetails.ErrorInfo); ok && info.GetMetadata()["resource_id"] == id {
			return true
		}
	}
	return false
}

// ControlPlaneReadError renders err for a data source lookup of id. A data
// source cannot drop itself from state, so a denial stays an error; this adds
// what the bare PermissionDenied leaves out.
func ControlPlaneReadError(id string, err error) string {
	detail := DeserializeGrpcError(err)
	if DeniedOnResource(id, err) {
		detail += "\n\n" + deniedMeansGoneOrUnreadable
	}
	return detail
}

// ControlPlaneGoneOnDelete is ControlPlaneGone for the error of a delete call.
// A denied delete is also what a caller who may read but not delete gets for a
// resource that is still there, so the denial counts as gone only when lookup,
// a read of the same resource, comes back gone as well.
func ControlPlaneGoneOnDelete(kind, id string, err error, lookup func() error) (bool, diag.Diagnostics) {
	if err == nil || IsNotFound(err) {
		return err != nil, nil
	}
	if !DeniedOnResource(id, err) {
		return false, nil
	}
	return ControlPlaneGone(kind, id, lookup())
}
