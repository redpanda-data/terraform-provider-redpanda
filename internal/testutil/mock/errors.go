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

package mock

import (
	"context"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// OverrideOnce arranges that the next call to the given gRPC full method name
// (e.g. "/redpanda.api.controlplane.v1.ResourceGroupService/CreateResourceGroup")
// returns the given err instead of dispatching to the fake. The override is
// consumed on the first matching call; subsequent calls fall through to the
// fake. Calling OverrideOnce multiple times for the same method replaces the
// pending error.
func (s *Server) OverrideOnce(fullMethod string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pendingOverrides[fullMethod] = err
}

// CallCount returns how many times the given gRPC full method name has been
// invoked since the Server was constructed. Use it to assert that a no-op
// provider Update skips its backend mutation RPC entirely (e.g. an
// allow_deletion-only flip must not call UpdateServerlessCluster).
func (s *Server) CallCount(fullMethod string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.callCounts[fullMethod]
}

// countingInterceptor returns a server unary interceptor that records every
// incoming call by full method name. Chained ahead of overrideInterceptor so
// error-injected calls are still counted.
func (s *Server) countingInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		s.mu.Lock()
		s.callCounts[info.FullMethod]++
		s.mu.Unlock()
		return handler(ctx, req)
	}
}

// overrideInterceptor returns a server unary interceptor that consults the
// pending-overrides map; on hit, returns the registered error and clears the
// entry. Chained ahead of validatingInterceptor so per-test error injection
// bypasses validation.
func (s *Server) overrideInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		s.mu.Lock()
		if err, ok := s.pendingOverrides[info.FullMethod]; ok {
			delete(s.pendingOverrides, info.FullMethod)
			s.mu.Unlock()
			return nil, err
		}
		s.mu.Unlock()
		return handler(ctx, req)
	}
}

// SetDenyMissing switches how the server answers for a resource that no longer
// exists. On, it answers the way the real control plane does: production checks
// the caller's permission on the resource before it looks the resource up, and
// a deleted resource has no permissions left, so the answer is PermissionDenied
// naming the resource, never NotFound. It is off by default because most tests
// predate it.
func (s *Server) SetDenyMissing(on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.denyMissing = on
}

// DeniedOn is the control plane's PermissionDenied for the resource id, for use
// with OverrideOnce where the resource exists and the caller may not act on it.
func DeniedOn(id string) error {
	st, err := status.New(codes.PermissionDenied, "Missing required permission read").WithDetails(&errdetails.ErrorInfo{
		Reason:   "REASON_PERMISSION_DENIED",
		Domain:   "redpanda.com/controlplane",
		Metadata: map[string]string{"resource_id": id},
	})
	if err != nil {
		return err
	}
	return st.Err()
}

// denyMissingInterceptor rewrites NotFound on a request addressed by ID into
// the control plane's PermissionDenied while SetDenyMissing is on.
func (s *Server) denyMissingInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		s.mu.Lock()
		on := s.denyMissing
		s.mu.Unlock()
		byID, ok := req.(interface{ GetId() string })
		if !on || !ok || byID.GetId() == "" || status.Code(err) != codes.NotFound {
			return resp, err
		}
		return nil, DeniedOn(byID.GetId())
	}
}
