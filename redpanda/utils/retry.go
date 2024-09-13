// Copyright 2023 Redpanda Data, Inc.
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
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Retry will retry a function with a delay between each invocation until it no longer returns
// an error, the timeout is reached, or the context is cancelled.
// Similar to https://pkg.go.dev/github.com/hashicorp/terraform-plugin-sdk/v2@v2.34.0/helper/retry#RetryContext
// but with no backoff and control over the delay between each invocation of the function.
func Retry(ctx context.Context, timeout, waitUnit time.Duration, f func() *RetryError) error {
	startTime := time.Now()
	endTime := startTime.Add(timeout)
	for {
		// Get the latest result
		err := f()
		if err == nil {
			return nil
		}
		tflog.Info(ctx, fmt.Sprint(err))
		if !err.Retryable {
			return err.Err
		}

		// Check if the timeout has been reached
		if time.Now().After(endTime) {
			return &TimeoutError{Timeout: timeout, Wrapped: err.Err}
		}

		// Wait for a certain duration before checking again
		sleeper := time.NewTimer(waitUnit)
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
		return &RetryError{Err: fmt.Errorf("RetryableError was passed a nil error"), Retryable: false}
	}
	return &RetryError{Err: err, Retryable: true}
}

// NonRetryableError is a helper to create a RetryError that's _not_ retryable
func NonRetryableError(err error) *RetryError {
	if err == nil {
		return &RetryError{Err: fmt.Errorf("NonRetryableError was passed a nil error"), Retryable: false}
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
