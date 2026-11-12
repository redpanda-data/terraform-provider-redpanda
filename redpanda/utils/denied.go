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
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	grpccodes "google.golang.org/grpc/codes"
)

// DeniedOnResource reports whether err is the control plane's PermissionDenied
// for the resource id itself.
//
// The control plane checks the caller's permission on a resource before it
// looks the resource up, and a deleted resource has no permissions left, so
// this is what a deleted resource answers instead of NotFound. Credentials
// without access get the identical answer, so it is never taken as proof the
// resource is gone.
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

// ControlPlaneResourceError is the diagnostic for err, returned by a
// control-plane call a managed resource made about itself. A denial naming the
// resource is explained, along with the way out of each of its two causes; any
// other error is reported under summary as it came.
func ControlPlaneResourceError(summary, kind, id string, err error) diag.Diagnostic {
	return controlPlaneError(summary, kind, id, err, fmt.Sprintf(
		"so the provider cannot tell which happened and has left Terraform state unchanged.\n\n"+
			"If %s %s was deleted outside Terraform, remove it from state with `terraform state rm <resource address>`; `terraform state list` shows the address. "+
			"Otherwise check the role bindings of the client credentials the provider is configured with.",
		kind, id))
}

// ControlPlaneDataSourceError is ControlPlaneResourceError for a data source
// lookup, which has no state to remove.
func ControlPlaneDataSourceError(summary, kind, id string, err error) diag.Diagnostic {
	return controlPlaneError(summary, kind, id, err, fmt.Sprintf(
		"so the provider cannot tell which happened.\n\n"+
			"Check that %s %s still exists and that the role bindings of the client credentials the provider is configured with grant access to it.",
		kind, id))
}

func controlPlaneError(summary, kind, id string, err error, advice string) diag.Diagnostic {
	original := DeserializeGrpcError(err)
	if !DeniedOnResource(id, err) {
		return diag.NewErrorDiagnostic(summary, original)
	}
	return diag.NewErrorDiagnostic(
		fmt.Sprintf("Redpanda Cloud denied access to %s %s", kind, id),
		"Redpanda Cloud answers PermissionDenied both when the client credentials lack permission on a resource and when the resource no longer exists, "+
			advice+"\n\nOriginal error: "+original,
	)
}

// ControlPlaneNotFound reports whether err, returned by a control-plane call,
// says the resource does not exist. Only the status code is consulted: the
// client wraps a lookup error with the resource ID, and IsNotFound's substring
// heuristics read an ID containing "404" as NotFound whatever the server
// actually answered.
func ControlPlaneNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.As(err, &NotFoundError{}) {
		return true
	}
	st, ok := serverStatus(err)
	return ok && st.Code() == grpccodes.NotFound
}
