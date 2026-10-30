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

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// RequireBearerToken makes every later gRPC call fail with Unauthenticated,
// and every later Schema Registry request without Basic credentials fail with
// 401, unless the request carries exactly this bearer token. Without it the
// server accepts any token or none, so a test that must prove the provider
// sent a specific credential has to opt in. Rejected gRPC calls are still
// counted by CallCount.
func (s *Server) RequireBearerToken(token string) {
	s.mu.Lock()
	s.requiredBearer = "Bearer " + token
	s.mu.Unlock()
	s.SR.RequireBearerToken(token)
}

func (s *Server) authInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		s.mu.Lock()
		want := s.requiredBearer
		s.mu.Unlock()
		if want == "" {
			return handler(ctx, req)
		}
		got := metadata.ValueFromIncomingContext(ctx, "authorization")
		if len(got) != 1 || got[0] != want {
			return nil, status.Errorf(codes.Unauthenticated, "authorization metadata %q does not carry the required bearer token", got)
		}
		return handler(ctx, req)
	}
}
