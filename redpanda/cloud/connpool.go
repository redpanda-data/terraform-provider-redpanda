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
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// ConnPool maintains a pool of gRPC connections keyed by URL. This allows
// multiple Terraform resources targeting the same cluster to share a single
// gRPC connection rather than each opening their own, avoiding connection
// storms that cause i/o timeouts under parallel resource operations.
type ConnPool struct {
	authToken        string
	providerVersion  string
	terraformVersion string

	mu    sync.Mutex
	conns map[string]*grpc.ClientConn
}

// NewConnPool creates a new connection pool with the given authentication and
// version parameters.
func NewConnPool(authToken, providerVersion, terraformVersion string) *ConnPool {
	return &ConnPool{
		authToken:        authToken,
		providerVersion:  providerVersion,
		terraformVersion: terraformVersion,
		conns:            make(map[string]*grpc.ClientConn),
	}
}

// GetConnection returns a shared gRPC connection for the given URL, creating
// one if it doesn't already exist.
func (p *ConnPool) GetConnection(url string) (*grpc.ClientConn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if conn, ok := p.conns[url]; ok {
		if conn.GetState() != connectivity.Shutdown {
			return conn, nil
		}
		// Connection is permanently closed; discard and create a new one.
		delete(p.conns, url)
	}

	conn, err := SpawnConn(url, p.authToken, p.providerVersion, p.terraformVersion)
	if err != nil {
		return nil, err
	}
	p.conns[url] = conn
	return conn, nil
}

// CloseAll closes all connections in the pool.
func (p *ConnPool) CloseAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for url, conn := range p.conns {
		_ = conn.Close()
		delete(p.conns, url)
	}
}
