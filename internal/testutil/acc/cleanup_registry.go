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

package acc

import (
	"context"
	"fmt"
	"sync"
)

// ResourceKind controls cleanup ordering. Lower values are cleaned up first
// (reverse-dependency order: shadow_links → clusters → serverless_private_links
// → networks → resource_groups). A serverless cluster references its private
// link, so the cluster must be deleted before the link — the backend rejects
// deleting a private link still associated with a cluster.
type ResourceKind int

// Resource kinds, ordered for reverse-dependency cleanup.
const (
	KindShadowLink            ResourceKind = 0
	KindCluster               ResourceKind = 1
	KindServerlessPrivateLink ResourceKind = 2
	KindNetwork               ResourceKind = 3
	KindResourceGroup         ResourceKind = 4
)

// Cleanupable is satisfied by anything that can delete itself.
type Cleanupable interface {
	Cleanup(ctx context.Context) error
}

// CleanupFunc adapts a plain function to the Cleanupable interface.
type CleanupFunc func(ctx context.Context) error

// Cleanup implements Cleanupable.
func (f CleanupFunc) Cleanup(ctx context.Context) error { return f(ctx) }

type registration struct {
	kind ResourceKind
	r    Cleanupable
}

// Registry tracks resources registered during a test run for deterministic
// teardown via TestMain.
type Registry struct {
	mu    sync.Mutex
	items []registration
}

// Register adds r to the registry under the given kind.
func (reg *Registry) Register(kind ResourceKind, r Cleanupable) {
	reg.mu.Lock()
	reg.items = append(reg.items, registration{kind: kind, r: r})
	reg.mu.Unlock()
}

// Cleanup tears down all tracked resources in reverse-dependency order.
// Errors are logged to stdout but do not halt cleanup of subsequent resources.
func (reg *Registry) Cleanup(ctx context.Context) {
	reg.mu.Lock()
	items := make([]registration, len(reg.items))
	copy(items, reg.items)
	reg.mu.Unlock()

	buckets := make(map[ResourceKind][]Cleanupable)
	for _, item := range items {
		buckets[item.kind] = append(buckets[item.kind], item.r)
	}
	for _, kind := range []ResourceKind{
		KindShadowLink,
		KindCluster,
		KindServerlessPrivateLink,
		KindNetwork,
		KindResourceGroup,
	} {
		for _, r := range buckets[kind] {
			if err := r.Cleanup(ctx); err != nil {
				fmt.Printf("acc cleanup: %v\n", err)
			}
		}
	}
}

// global is the package-level registry used by per-package TestMain cleanup.
var global Registry

// Register adds r to the package-global registry under kind.
func Register(kind ResourceKind, r Cleanupable) {
	global.Register(kind, r)
}

// Cleanup invokes Cleanup on the package-global registry. Called from
// per-package TestMain after m.Run() returns.
func Cleanup(ctx context.Context) {
	global.Cleanup(ctx)
}
