package cloud

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func clampLimit(limit, period float64) rate.Limit {
	return rate.Limit(math.Floor(limit / period))
}

func TestRateLimiter_Limiter(t *testing.T) {
	tests := []struct {
		name            string
		initialLimit    int
		headerLimit     string
		remainingValues []int
		expectedLimit   rate.Limit
		expectedBurst   int
		invokerError    error
		expectedError   error
	}{
		{
			name:            "Normal update",
			initialLimit:    200,
			headerLimit:     "limit=200,remaining=%d,reset=30",
			remainingValues: []int{75, 74, 73, 72, 71},
			expectedLimit:   clampLimit(200.0, limitPeriod),
			expectedBurst:   int(200 / burstPeriod),
			invokerError:    nil,
			expectedError:   nil,
		},
		{
			name:            "Rate limit exceeded",
			initialLimit:    100,
			headerLimit:     "limit=100,remaining=%d,reset=15",
			remainingValues: []int{5, 4, 3, 2, 1, 0},
			expectedLimit:   clampLimit(100.0, limitPeriod),
			expectedBurst:   int(100 / burstPeriod),
			invokerError:    nil,
			expectedError:   nil,
		},
		{
			name:            "Invalid header",
			initialLimit:    100,
			headerLimit:     "invalid=header=asdrf",
			remainingValues: []int{0},
			expectedLimit:   1,
			expectedBurst:   10,
			invokerError:    nil,
			expectedError:   nil,
		},
		{
			name:            "Missing header element",
			initialLimit:    100,
			headerLimit:     "limit=100,remaining=%d",
			remainingValues: []int{5, 4, 3, 2, 1, 0},
			expectedLimit:   1,
			expectedBurst:   10,
			invokerError:    nil,
			expectedError:   nil,
		},
		{
			name:            "Invoker error", // validates that the invoker mock is working
			initialLimit:    100,
			headerLimit:     "",
			remainingValues: []int{0},
			expectedLimit:   clampLimit(100.0, limitPeriod),
			expectedBurst:   int(100 / burstPeriod),
			invokerError:    errors.New("invoker error"),
			expectedError:   errors.New("invoker error"),
		},
		{
			name:            "No rate limit headers",
			initialLimit:    100,
			headerLimit:     "",
			remainingValues: []int{0},
			expectedLimit:   1,
			expectedBurst:   10,
			invokerError:    nil,
			expectedError:   nil,
		},
		{
			name:            "Malformed header",
			initialLimit:    100,
			headerLimit:     "limit=monkey,remaining=%d,reset=soon",
			remainingValues: []int{0},
			expectedLimit:   1,
			expectedBurst:   10,
			invokerError:    nil,
			expectedError:   nil,
		},
		{
			name:            "Malformed header missing contents",
			initialLimit:    100,
			headerLimit:     "monkey=1,remaining=%d,reset=soon",
			remainingValues: []int{0},
			expectedLimit:   1,
			expectedBurst:   10,
			invokerError:    nil,
			expectedError:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := newRateLimiter(tt.initialLimit)
			var lastErr error
			for _, remaining := range tt.remainingValues {
				var headerLimit string
				if tt.headerLimit != "" {
					headerLimit = fmt.Sprintf(tt.headerLimit, remaining)
				}
				mockInvoker := func(_ context.Context, _ string, _, _ any, _ *grpc.ClientConn, opts ...grpc.CallOption) error {
					if tt.headerLimit != "" {
						header := metadata.MD{
							"ratelimit": []string{headerLimit},
						}
						for _, opt := range opts {
							if headerOpt, ok := opt.(grpc.HeaderCallOption); ok {
								*headerOpt.HeaderAddr = header
								break
							}
						}
					}
					return tt.invokerError
				}

				lastErr = rl.Limiter(context.Background(), "/test.Method", nil, nil, nil, mockInvoker)
				if lastErr != nil {
					break
				}
			}

			if (lastErr != nil) != (tt.expectedError != nil) {
				t.Errorf("Expected error %v, got %v", tt.expectedError, lastErr)
			}
			if tt.expectedError != nil && lastErr != nil && lastErr.Error() != tt.expectedError.Error() {
				t.Errorf("Expected error message %q, got %q", tt.expectedError, lastErr)
			}

			if rl.limiter.Limit() != tt.expectedLimit {
				t.Errorf("Expected limit to be %v, got %v", tt.expectedLimit, rl.limiter.Limit())
			}

			if rl.limiter.Burst() != tt.expectedBurst {
				t.Errorf("Expected burst to be %v, got %v", tt.expectedBurst, rl.limiter.Burst())
			}
		})
	}
}

func TestParseRateLimit(t *testing.T) {
	tests := []struct {
		name          string
		header        string
		expectedLimit int
		expectedRem   int
		expectedReset time.Duration
		expectError   bool
	}{
		{
			name:          "Valid header",
			header:        "limit=200,remaining=75,reset=30",
			expectedLimit: 200,
			expectedRem:   75,
			expectedReset: 30 * time.Second,
			expectError:   false,
		},
		{
			name:        "Missing field",
			header:      "limit=200,remaining=75",
			expectError: true,
		},
		{
			name:        "Invalid value",
			header:      "limit=monkey,remaining=75,reset=30",
			expectError: true,
		},
		{
			name:          "Extra field",
			header:        "limit=200,remaining=75,reset=30,extra=10",
			expectedLimit: 200,
			expectedRem:   75,
			expectedReset: 30 * time.Second,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit, remaining, reset, err := parseRateLimit(tt.header)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected an error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if limit != tt.expectedLimit {
					t.Errorf("Expected limit %d, got %d", tt.expectedLimit, limit)
				}
				if remaining != tt.expectedRem {
					t.Errorf("Expected remaining %d, got %d", tt.expectedRem, remaining)
				}
				if reset != tt.expectedReset {
					t.Errorf("Expected reset %v, got %v", tt.expectedReset, reset)
				}
			}
		})
	}
}
