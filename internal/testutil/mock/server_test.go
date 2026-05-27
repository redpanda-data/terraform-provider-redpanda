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

package mock_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/redpanda-data/terraform-provider-redpanda/internal/testutil/mock"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda/cloud"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// spyTB wraps a testing.TB and records Errorf calls without forwarding the
// failure to the underlying TB. Used to assert that a t.Cleanup func fires the
// expected diagnostic message without actually failing the outer test.
type spyTB struct {
	testing.TB
	mu       sync.Mutex
	errors   []string
	cleanups []func()
}

func (s *spyTB) Errorf(format string, args ...any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errors = append(s.errors, fmt.Sprintf(format, args...))
}

func (*spyTB) Helper() {}

func (s *spyTB) Cleanup(f func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanups = append(s.cleanups, f)
}

// runCleanups executes registered cleanups in LIFO order, mirroring Go's
// testing.T cleanup semantics.
func (s *spyTB) runCleanups() {
	s.mu.Lock()
	cleanups := s.cleanups
	s.cleanups = nil
	s.mu.Unlock()
	for i := len(cleanups) - 1; i >= 0; i-- {
		cleanups[i]()
	}
}

func (s *spyTB) allErrors() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return strings.Join(s.errors, "; ")
}

func (s *spyTB) hasError() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.errors) > 0
}

// TestServer_OverrideOnce_UnconsumedFails asserts that an OverrideOnce entry
// registered for a method that is never called produces a test-failure message
// at cleanup time. Without this safety net a typo'd gRPC full method name
// causes the fake to return success and an ExpectError step fails with a
// misleading "expected error, got nil" instead of "override never consumed".
func TestServer_OverrideOnce_UnconsumedFails(t *testing.T) {
	spy := &spyTB{TB: t}

	srv := mock.New(spy)
	srv.OverrideOnce("/wrong.service.v1.WrongService/WrongMethod", status.Error(codes.Internal, "injected"))

	spy.runCleanups()

	if !spy.hasError() {
		t.Fatal("expected cleanup to report unconsumed OverrideOnce entry, but no error was recorded")
	}
	got := spy.allErrors()
	if !strings.Contains(got, "WrongMethod") {
		t.Errorf("error message should mention the unconsumed method name; got: %s", got)
	}
	if !strings.Contains(got, "never consumed") {
		t.Errorf("error message should mention 'never consumed'; got: %s", got)
	}
}

// TestServer_OverrideOnce_ConsumedNoCleanupError asserts that when an override
// IS consumed by a matching RPC call the cleanup does NOT fire a failure. This
// is the non-regression half of the safety-net check.
func TestServer_OverrideOnce_ConsumedNoCleanupError(t *testing.T) {
	spy := &spyTB{TB: t}

	srv := mock.New(spy)

	const method = "/redpanda.api.controlplane.v1.ResourceGroupService/CreateResourceGroup"
	srv.OverrideOnce(method, status.Error(codes.Internal, "injected"))

	conn, err := grpc.NewClient("passthrough:///bufnet", srv.Dialer()...)
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	client := cloud.NewControlPlaneClientSet(conn)
	// We expect an error here because the override returns Internal; that's fine — we just
	// need the interceptor to consume the entry.
	_, _ = client.CreateResourceGroup(context.Background(), "consume-override-test")

	spy.runCleanups()

	if spy.hasError() {
		t.Errorf("expected no cleanup error after override was consumed, got: %s", spy.allErrors())
	}
}

// TestServer_OverrideOnce_UnconsumedReportsAllLeftovers asserts that the error
// message includes every unconsumed method, not just the first one.
func TestServer_OverrideOnce_UnconsumedReportsAllLeftovers(t *testing.T) {
	spy := &spyTB{TB: t}

	srv := mock.New(spy)
	srv.OverrideOnce("/svc.v1.Svc/MethodA", status.Error(codes.Internal, "a"))
	srv.OverrideOnce("/svc.v1.Svc/MethodB", status.Error(codes.Internal, "b"))

	spy.runCleanups()

	if !spy.hasError() {
		t.Fatal("expected cleanup errors for both unconsumed overrides")
	}
	got := spy.allErrors()
	if !strings.Contains(got, "MethodA") || !strings.Contains(got, "MethodB") {
		t.Errorf("error message should list all unconsumed methods; got: %s", got)
	}
}
