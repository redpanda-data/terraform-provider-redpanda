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

package cloudtest

import (
	"context"
	"net"
	"testing"

	"buf.build/gen/go/redpandadata/cloud/grpc/go/redpanda/api/controlplane/v1/controlplanev1grpc"
	"buf.build/gen/go/redpandadata/dataplane/grpc/go/redpanda/api/dataplane/v1/dataplanev1grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const bufconnBufSize = 1024 * 1024

// Fake is the handle returned by Start/StartWithDataplane. Inspect
// server-side state via the methods on this struct (e.g. ClusterCount).
type Fake struct {
	cpLis *bufconn.Listener
	cpSrv *grpc.Server
	dpLis *bufconn.Listener
	dpSrv *grpc.Server

	cs  *clusterServer
	os  *operationServer
	scs *serverlessClusterServer
	spl *serverlessPrivateLinkServer
	ps  *pipelineServer
}

// Start spins up an in-process fake control plane and returns it together
// with a *grpc.ClientConn dialed to the bufconn. Pass the conn to
// redpanda.NewWithTestConn. Cleanup is registered on t; the conn uses
// insecure creds and never opens a network socket.
//
// The fake registers: ClusterService, OperationService,
// ServerlessClusterService, ServerlessPrivateLinkService. For resources
// that dial the dataplane (pipeline), use StartWithDataplane instead.
func Start(t *testing.T) (*Fake, *grpc.ClientConn) {
	t.Helper()
	fake, cpConn, _ := start(t, false)
	return fake, cpConn
}

// StartWithDataplane additionally spins up a dataplane bufconn hosting
// PipelineService. Returns the fake, the control-plane conn, and the
// dataplane conn. Pass both to redpanda.NewWithTestConnAndDataplane.
func StartWithDataplane(t *testing.T) (fake *Fake, cpConn, dpConn *grpc.ClientConn) {
	t.Helper()
	return start(t, true)
}

func start(t *testing.T, withDataplane bool) (fake *Fake, cpConn, dpConn *grpc.ClientConn) {
	cpLis := bufconn.Listen(bufconnBufSize)
	cpSrv := grpc.NewServer()

	cs := newClusterServer()
	os := newOperationServer()
	scs := newServerlessClusterServer()
	spl := newServerlessPrivateLinkServer()
	controlplanev1grpc.RegisterClusterServiceServer(cpSrv, cs)
	controlplanev1grpc.RegisterOperationServiceServer(cpSrv, os)
	controlplanev1grpc.RegisterServerlessClusterServiceServer(cpSrv, scs)
	controlplanev1grpc.RegisterServerlessPrivateLinkServiceServer(cpSrv, spl)

	go func() {
		// Errors after GracefulStop are expected; the cleanup hook owns
		// the shutdown sequence.
		_ = cpSrv.Serve(cpLis)
	}()

	var err error
	cpConn, err = dialBufconn(cpLis)
	if err != nil {
		t.Fatalf("cloudtest: dialing control-plane bufconn: %v", err)
	}

	fake = &Fake{cpLis: cpLis, cpSrv: cpSrv, cs: cs, os: os, scs: scs, spl: spl}

	if withDataplane {
		dpLis := bufconn.Listen(bufconnBufSize)
		dpSrv := grpc.NewServer()
		ps := newPipelineServer()
		dataplanev1grpc.RegisterPipelineServiceServer(dpSrv, ps)
		go func() {
			_ = dpSrv.Serve(dpLis)
		}()
		dpConn, err = dialBufconn(dpLis)
		if err != nil {
			t.Fatalf("cloudtest: dialing dataplane bufconn: %v", err)
		}
		fake.dpLis = dpLis
		fake.dpSrv = dpSrv
		fake.ps = ps
	}

	t.Cleanup(func() {
		_ = cpConn.Close()
		cpSrv.GracefulStop()
		_ = cpLis.Close()
		if dpConn != nil {
			_ = dpConn.Close()
			fake.dpSrv.GracefulStop()
			_ = fake.dpLis.Close()
		}
	})

	return fake, cpConn, dpConn
}

func dialBufconn(lis *bufconn.Listener) (*grpc.ClientConn, error) {
	dialer := func(_ context.Context, _ string) (net.Conn, error) {
		return lis.Dial()
	}
	return grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
}

// ClusterCount returns the number of clusters currently in the fake's
// in-memory store.
func (f *Fake) ClusterCount() int {
	f.cs.mu.Lock()
	defer f.cs.mu.Unlock()
	return len(f.cs.clusters)
}

// ServerlessClusterCount returns the number of serverless clusters in
// the fake's in-memory store.
func (f *Fake) ServerlessClusterCount() int {
	f.scs.mu.Lock()
	defer f.scs.mu.Unlock()
	return len(f.scs.clusters)
}

// ServerlessPrivateLinkCount returns the number of serverless private
// links in the fake's in-memory store.
func (f *Fake) ServerlessPrivateLinkCount() int {
	f.spl.mu.Lock()
	defer f.spl.mu.Unlock()
	return len(f.spl.links)
}

// PipelineCount returns the number of pipelines in the fake's
// in-memory store. Panics if the fake was started via Start rather
// than StartWithDataplane (pipeline service isn't registered).
func (f *Fake) PipelineCount() int {
	f.ps.mu.Lock()
	defer f.ps.mu.Unlock()
	return len(f.ps.pipelines)
}
