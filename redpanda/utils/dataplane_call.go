// Copyright 2025 Redpanda Data, Inc.
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

package utils

import (
	"context"
	"time"
)

// DataplaneCall runs one RPC against a per-cluster endpoint under a single
// shared retry policy.
//
// Per-cluster endpoints are not serving the moment a cluster reports Ready, so
// the first calls against a fresh cluster come back as transient failures that
// say nothing about the request. Every resource used to decide for itself what
// to do about that, and they disagreed: some retried, some did not, and the ones
// that did classified differently. Each disagreement was found by a live failure.
//
// The policy applied here:
//
//	success                      → return the value
//	non-transient error          → fail immediately
//	transient error              → retry until the budget is spent
//	AlreadyExists, first attempt → fail; the resource predates this call
//	AlreadyExists, later attempt → adopt via the probe, else fail
//
// The AlreadyExists split matters. Only a retry can have landed and lost its
// response; on the first attempt it means something else created the resource,
// and adopting it would put a resource Terraform did not create under its
// management, where a later destroy would remove it.
//
// A probe, where supplied, recognises a resource an earlier attempt created. It
// only runs from the second attempt onwards, for the same reason.
//
// Use DataplaneCallOnce for a call that deliberately must not retry.
func DataplaneCall[T any](ctx context.Context, do func(context.Context) (T, error), opts ...CallOption[T]) (T, error) {
	cfg := callConfig[T]{
		budget:   DefaultDataplaneRetryTimeout,
		classify: IsTransientDataplaneError,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	var result T
	attempt := 0
	err := Retry(ctx, cfg.budget, func() *RetryError {
		attempt++
		value, rpcErr := do(ctx)
		if rpcErr == nil {
			result = value
			return nil
		}

		if IsAlreadyExists(rpcErr) {
			if adopted, ok := cfg.adopt(ctx, attempt); ok {
				result = adopted
				return nil
			}
			return NonRetryableError(rpcErr)
		}

		if !cfg.classify(rpcErr) {
			return NonRetryableError(rpcErr)
		}

		// The failed attempt may still have landed.
		if adopted, ok := cfg.adopt(ctx, attempt); ok {
			result = adopted
			return nil
		}
		return RetryableError(rpcErr)
	})
	return result, err
}

// DataplaneCallOnce runs an RPC with no retry. It exists so that opting out is
// explicit and greppable rather than an omission, and so the guard test that
// keeps RPCs inside this file can tell the two apart.
func DataplaneCallOnce[T any](ctx context.Context, do func(context.Context) (T, error)) (T, error) {
	return do(ctx)
}

// CallOption configures DataplaneCall.
type CallOption[T any] func(*callConfig[T])

type callConfig[T any] struct {
	budget   time.Duration
	classify func(error) bool
	probe    func(context.Context) (T, bool)
}

// adopt reports a resource an earlier attempt created. It never runs on the
// first attempt: nothing this call did could have created anything yet, so a hit
// would be someone else's resource.
func (c callConfig[T]) adopt(ctx context.Context, attempt int) (T, bool) {
	var zero T
	if c.probe == nil || attempt < 2 {
		return zero, false
	}
	return c.probe(ctx)
}

// WithProbe supplies a lookup that reports whether the resource already exists,
// used to recognise an earlier attempt whose response was lost. Give it to any
// create whose retry would otherwise duplicate, or trip AlreadyExists.
func WithProbe[T any](probe func(context.Context) (T, bool)) CallOption[T] {
	return func(c *callConfig[T]) { c.probe = probe }
}

// WithBudget overrides how long the call may keep retrying.
func WithBudget[T any](d time.Duration) CallOption[T] {
	return func(c *callConfig[T]) { c.budget = d }
}

// WithClassifier overrides which errors count as transient. The default is
// IsTransientDataplaneError, which reads gRPC status codes; Schema Registry
// resources speak HTTP and need their own.
func WithClassifier[T any](classify func(error) bool) CallOption[T] {
	return func(c *callConfig[T]) { c.classify = classify }
}
