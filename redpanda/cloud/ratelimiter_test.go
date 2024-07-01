package cloud

import (
	"context"
	"errors"
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
		name                string
		initialLimit        int
		headerLimit         string
		expectedLimit       rate.Limit
		invokerError        error
		expectedError       error
		consecutiveCalls    int
		expectedMinDuration time.Duration
	}{
		{
			name:                "Normal update",
			initialLimit:        200,
			headerLimit:         "limit=200,remaining=75,reset=30",
			expectedLimit:       clampLimit(200.0, limitPeriod),
			invokerError:        nil,
			expectedError:       nil,
			consecutiveCalls:    10,
			expectedMinDuration: time.Second * 7,
		},
		{
			name:                "Rate limit exceeded",
			initialLimit:        100,
			headerLimit:         "limit=10,remaining=0,reset=30",
			expectedLimit:       clampLimit(10.0, limitPeriod),
			invokerError:        nil,
			expectedError:       nil,
			consecutiveCalls:    5,
			expectedMinDuration: time.Second * 29,
		},
		{
			name:                "Invalid header",
			initialLimit:        100,
			headerLimit:         "invalid=header",
			expectedLimit:       clampLimit(100.0, limitPeriod),
			invokerError:        nil,
			expectedError:       errors.New("failed to parse rate limit header: incomplete rate limit header: missing required fields"),
			consecutiveCalls:    1,
			expectedMinDuration: 0,
		},
		{
			name:                "Invoker error",
			initialLimit:        100,
			headerLimit:         "",
			expectedLimit:       clampLimit(100.0, limitPeriod),
			invokerError:        errors.New("invoker error"),
			expectedError:       errors.New("invoker error"),
			consecutiveCalls:    1,
			expectedMinDuration: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := newRateLimiter(tt.initialLimit)

			mockInvoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
				if tt.headerLimit != "" {
					header := metadata.MD{
						"ratelimit": []string{tt.headerLimit},
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

			start := time.Now()
			var lastErr error
			for i := 0; i < tt.consecutiveCalls; i++ {
				lastErr = rl.Limiter(context.Background(), "/test.Method", nil, nil, nil, mockInvoker)
				if lastErr != nil {
					break
				}
			}
			duration := time.Since(start)

			if (lastErr != nil) != (tt.expectedError != nil) {
				t.Errorf("Expected error %v, got %v", tt.expectedError, lastErr)
			}
			if tt.expectedError != nil && lastErr != nil && lastErr.Error() != tt.expectedError.Error() {
				t.Errorf("Expected error message %q, got %q", tt.expectedError, lastErr)
			}

			if rl.limiter.Limit() != tt.expectedLimit {
				t.Errorf("Expected limit to be %v, got %v", tt.expectedLimit, rl.limiter.Limit())
			}

			if duration < tt.expectedMinDuration {
				t.Errorf("Expected minimum duration of %v, but got %v", tt.expectedMinDuration, duration)
			}
		})
	}
}
