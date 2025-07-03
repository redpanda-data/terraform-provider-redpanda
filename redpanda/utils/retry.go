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
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Retry will retry a function with a delay between each invocation until it no longer returns
// an error, the timeout is reached, or the context is cancelled.
// It uses exponential backoff between each call of the function, up to a max of 1 minute, and
// resets the delay whenever the function returns an error with a different string.
// Similar to https://pkg.go.dev/github.com/hashicorp/terraform-plugin-sdk/v2@v2.34.0/helper/retry#RetryContext
func Retry(ctx context.Context, timeout time.Duration, f func() *RetryError) error {
	const (
		initialWaitUnit = time.Second
		maxWaitUnit     = time.Minute
	)
	startTime := time.Now()
	endTime := startTime.Add(timeout)
	waitUnit := initialWaitUnit
	lastErrorMessage := ""
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

		// Wait between checks using exponential backoff. If the error message has
		// changed, reset to the initial value
		if err.Err.Error() != lastErrorMessage {
			tflog.Info(ctx, fmt.Sprintf(
				"Resetting retry wait time because current error %q is different from last error %q",
				err.Err.Error(),
				lastErrorMessage,
			))
			lastErrorMessage = err.Err.Error()
			waitUnit = initialWaitUnit
		} else {
			waitUnit *= 2
		}
		if waitUnit > maxWaitUnit {
			waitUnit = maxWaitUnit
		}
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
