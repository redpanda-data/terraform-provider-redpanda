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
	"sort"
	"strings"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	rpcstatus "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// DescribeStatus renders a control-plane state_description for a message:
// the status code always, the message when present, and any ErrorInfo
// detail's reason, domain and metadata. The public API blanks the message
// for failures the provisioner did not classify as external, so the code is
// often all that survives, and the ErrorInfo detail carries the specifics
// when the message does survive. An absent or OK status renders as empty.
func DescribeStatus(st *rpcstatus.Status) string {
	if st == nil || (st.GetCode() == int32(codes.OK) && st.GetMessage() == "") {
		return ""
	}
	var b strings.Builder
	b.WriteString(status.FromProto(st).Code().String())
	if msg := st.GetMessage(); msg != "" {
		b.WriteString(": ")
		b.WriteString(msg)
	}
	for _, detail := range st.GetDetails() {
		var info errdetails.ErrorInfo
		if err := detail.UnmarshalTo(&info); err != nil {
			continue
		}
		parts := []string{}
		if info.GetReason() != "" {
			parts = append(parts, "reason="+info.GetReason())
		}
		if info.GetDomain() != "" {
			parts = append(parts, "domain="+info.GetDomain())
		}
		keys := make([]string, 0, len(info.GetMetadata()))
		for k := range info.GetMetadata() {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s=%s", k, info.GetMetadata()[k]))
		}
		if len(parts) > 0 {
			b.WriteString(" [" + strings.Join(parts, " ") + "]")
		}
	}
	return b.String()
}
