// Copyright 2024 Redpanda Data, Inc.
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

package cloud

import (
	"errors"
	"sync"

	"golang.org/x/sync/singleflight"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// ConnPool maintains a pool of gRPC connections keyed by URL. This allows
// multiple Terraform resources targeting the same cluster to share a single
// gRPC connection rather than each opening their own, avoiding connection
// storms that cause i/o timeouts under parallel resource operations.
//
// The pool uses singleflight to deduplicate concurrent connection attempts for
// the same URL, and only holds the mutex for fast map operations — never across
// network calls. This ensures that a slow dial to one cluster does not block
// connections to other clusters.
type ConnPool struct {
	authToken        string
	providerVersion  string
	terraformVersion string

	mu        sync.Mutex
	conns     map[string]*grpc.ClientConn
	sf        singleflight.Group
	spawnFunc func(url, authToken, providerVersion, terraformVersion string) (*grpc.ClientConn, error)
}

// NewConnPool creates a new connection pool with the given authentication and
// version parameters.
func NewConnPool(authToken, providerVersion, terraformVersion string) *ConnPool {
	return &ConnPool{
		authToken:        authToken,
		providerVersion:  providerVersion,
		terraformVersion: terraformVersion,
		conns:            make(map[string]*grpc.ClientConn),
		spawnFunc:        SpawnConn,
	}
}

// isHealthy returns true if the connection is in a usable state.
func isHealthy(conn *grpc.ClientConn) bool {
	state := conn.GetState()
	return state != connectivity.Shutdown && state != connectivity.TransientFailure
}

// GetConnection returns a shared gRPC connection for the given URL, creating
// one if it doesn't already exist. Unhealthy connections (Shutdown or
// TransientFailure) are evicted and replaced.
func (p *ConnPool) GetConnection(url string) (*grpc.ClientConn, error) {
	// Fast path: return an existing healthy connection.
	p.mu.Lock()
	if conn, ok := p.conns[url]; ok {
		if isHealthy(conn) {
			p.mu.Unlock()
			return conn, nil
		}
		delete(p.conns, url)
		go conn.Close()
	}
	p.mu.Unlock()

	// Slow path: use singleflight so only one goroutine dials per URL.
	v, err, _ := p.sf.Do(url, func() (any, error) {
		// Double-check: another caller in this singleflight group may have
		// already stored a connection.
		p.mu.Lock()
		if conn, ok := p.conns[url]; ok {
			if isHealthy(conn) {
				p.mu.Unlock()
				return conn, nil
			}
			delete(p.conns, url)
			go conn.Close()
		}
		p.mu.Unlock()

		conn, err := p.spawnFunc(url, p.authToken, p.providerVersion, p.terraformVersion)
		if err != nil {
			return nil, err
		}

		p.mu.Lock()
		p.conns[url] = conn
		p.mu.Unlock()
		return conn, nil
	})
	if err != nil {
		return nil, err
	}
	conn, ok := v.(*grpc.ClientConn)
	if !ok {
		return nil, errors.New("unexpected type from connection pool singleflight")
	}
	return conn, nil
}

// CloseAll closes all connections in the pool. This method exists for
// completeness but is intentionally not called during normal operation — the
// Terraform plugin framework has no provider-level teardown hook, so
// connections are cleaned up when the process exits.
func (p *ConnPool) CloseAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for url, conn := range p.conns {
		_ = conn.Close()
		delete(p.conns, url)
	}
}
