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
	"testing"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	rpcstatus "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestDescribeStatus(t *testing.T) {
	info, err := anypb.New(&errdetails.ErrorInfo{Reason: "QUOTA_MISSING", Domain: "redpanda.com", Metadata: map[string]string{"quota": "cpus"}})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		st   *rpcstatus.Status
		want string
	}{
		{"nil", nil, ""},
		{"ok with no message", &rpcstatus.Status{Code: int32(codes.OK)}, ""},
		{"code only", &rpcstatus.Status{Code: int32(codes.Internal)}, "Internal"},
		{"code and message", &rpcstatus.Status{Code: int32(codes.FailedPrecondition), Message: "agent never registered"}, "FailedPrecondition: agent never registered"},
		{"error info detail", &rpcstatus.Status{Code: int32(codes.ResourceExhausted), Message: "quota", Details: []*anypb.Any{info}}, "ResourceExhausted: quota [reason=QUOTA_MISSING domain=redpanda.com quota=cpus]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DescribeStatus(tc.st); got != tc.want {
				t.Fatalf("DescribeStatus = %q, want %q", got, tc.want)
			}
		})
	}
}
