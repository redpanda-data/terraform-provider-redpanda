// Copyright 2026 Redpanda Data, Inc.
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

package mock

import (
	"context"

	"google.golang.org/grpc"
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
