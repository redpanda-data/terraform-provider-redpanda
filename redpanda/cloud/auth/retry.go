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

package auth

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

const (
	retryMax     = 3
	retryWaitMin = 200 * time.Millisecond
	retryWaitMax = 30 * time.Second
)

func newRetryClient() *http.Client {
	cl := retryablehttp.NewClient()
	cl.RetryMax = retryMax
	cl.RetryWaitMin = retryWaitMin
	cl.RetryWaitMax = retryWaitMax
	cl.CheckRetry = checkRetry
	cl.Logger = nil
	return cl.StandardClient()
}

func checkRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	shouldRetry, retryErr := retryablehttp.DefaultRetryPolicy(ctx, resp, err)
	if retryErr != nil || !shouldRetry {
		return shouldRetry, retryErr
	}
	if resp != nil && resp.StatusCode == http.StatusTooManyRequests {
		if wait, ok := parseRetryAfter(resp.Header.Get("Retry-After")); ok && wait > retryWaitMax {
			return false, fmt.Errorf("token endpoint returned 429; retry-after %v exceeds max wait %v", wait, retryWaitMax)
		}
	}
	return true, nil
}

func parseRetryAfter(s string) (time.Duration, bool) {
	if s == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(s); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second, true
	}
	return 0, false
}
