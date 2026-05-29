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

// Package utils contains multiple utility functions used across the Redpanda's
// terraform codebase
package utils

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sync/atomic"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// retryInitialWait and retryMaxWait are package-level atomics (not const) so
// tests can shrink the backoff floor + ceiling to sub-millisecond without
// changing production timing. Atomic int64 nanoseconds; production callers
// keep the 1s/60s defaults. Reads happen from Retry which runs in resource
// CRUD flows under t.Parallel; writes happen from SetTestModeWaits when
// REDPANDA_TF_ACCEPTANCE_TEST_MODE=1 is observed in provider Configure().
var (
	retryInitialWait atomic.Int64
	retryMaxWait     atomic.Int64
)

func init() {
	retryInitialWait.Store(int64(time.Second))
	retryMaxWait.Store(int64(time.Minute))
}

// SetTestModeWaits collapses retry backoff to microseconds for integration tests.
func SetTestModeWaits() {
	retryInitialWait.Store(int64(time.Microsecond))
	retryMaxWait.Store(int64(10 * time.Microsecond))
}

// Retry will retry a function with a delay between each invocation until it no longer returns
// an error, the timeout is reached, or the context is cancelled.
// It uses exponential backoff between each call of the function, up to a max of 1 minute, and
// resets the delay whenever the function returns an error with a different string.
// Similar to https://pkg.go.dev/github.com/hashicorp/terraform-plugin-sdk/v2@v2.34.0/helper/retry#RetryContext
func Retry(ctx context.Context, timeout time.Duration, f func() *RetryError) error {
	initialWaitUnit := time.Duration(retryInitialWait.Load())
	maxWaitUnit := time.Duration(retryMaxWait.Load())
	startTime := time.Now()
	endTime := startTime.Add(timeout)
	waitUnit := initialWaitUnit
	lastErrorMessage := ""
	attempt := 0
	for {
		// Get the latest result
		err := f()
		if err == nil {
			return nil
		}
		if !err.Retryable {
			return err.Err
		}

		// Check if the timeout has been reached
		if time.Now().After(endTime) {
			return &TimeoutError{Timeout: timeout, Wrapped: err.Err}
		}
		reset := err.Err.Error() != lastErrorMessage
		if reset {
			lastErrorMessage = err.Err.Error()
			waitUnit = initialWaitUnit
		} else {
			waitUnit *= 2
		}
		if waitUnit > maxWaitUnit {
			waitUnit = maxWaitUnit
		}
		jittered := waitUnit/2 + time.Duration(rand.Int64N(int64(waitUnit))) //nolint:gosec // math/rand/v2 is fine for retry jitter; not security-sensitive
		attempt++
		tflog.Debug(ctx, "retrying after transient error", map[string]any{
			"attempt": attempt,
			"wait":    jittered.String(),
			"elapsed": time.Since(startTime).String(),
			"reset":   reset,
			"error":   err.Err.Error(),
		})
		sleeper := time.NewTimer(jittered)
		select {
		case <-ctx.Done():
			if !sleeper.Stop() {
				<-sleeper.C
			}
		case <-sleeper.C:
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}

// RetryError is the required return type of functions passed to Retry
type RetryError struct {
	Err       error
	Retryable bool
}

// RetryableError is a helper to create a RetryError that's retryable
func RetryableError(err error) *RetryError {
	if err == nil {
		return &RetryError{Err: errors.New("RetryableError was passed a nil error"), Retryable: false}
	}
	return &RetryError{Err: err, Retryable: true}
}

// NonRetryableError is a helper to create a RetryError that's _not_ retryable
func NonRetryableError(err error) *RetryError {
	if err == nil {
		return &RetryError{Err: errors.New("NonRetryableError was passed a nil error"), Retryable: false}
	}
	return &RetryError{Err: err, Retryable: false}
}

// TimeoutError is returned when Retry times out
type TimeoutError struct {
	Timeout time.Duration
	Wrapped error
}

func (err *TimeoutError) Error() string {
	return fmt.Sprintf("timed out after %v: %v", err.Timeout, err.Wrapped)
}

func (err *TimeoutError) Unwrap() error {
	return err.Wrapped
}
