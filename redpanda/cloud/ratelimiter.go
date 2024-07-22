package cloud

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	limitPeriod = 60.0 // api resets rate limit at 60s
	burstPeriod = 10.0 // allows bursts of up to 50 requests to reduce hammering of the api
)

type rateLimiter struct {
	limiter *rate.Limiter
}

func newRateLimiter(limit int) *rateLimiter {
	return &rateLimiter{
		limiter: rate.NewLimiter(rate.Limit(limit/limitPeriod), limit/burstPeriod),
	}
}

// parseRateLimit expects a header as defined in https://datatracker.ietf.org/doc/html/draft-ietf-httpapi-ratelimit-headers
func parseRateLimit(header string) (limit, remaining int, reset time.Duration, err error) {
	parts := strings.Split(header, ",")
	if len(parts) != 3 {
		return 0, 0, 0, fmt.Errorf("invalid rate limit header: %s", header) // we expect limit, remaining and reset
	}

	var limitSet, remainingSet, resetSet bool
	for _, part := range parts {
		kv := strings.Split(part, "=")
		if len(kv) != 2 { // invalid header contents skip example "limit=1=remaining"
			continue
		}

		key, value := strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1])
		intValue, err := strconv.Atoi(value)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("invalid %s value: %v", key, err)
		}

		switch key {
		case "limit":
			limit = intValue
			limitSet = true
		case "remaining":
			remaining = intValue
			remainingSet = true
		case "reset":
			reset = time.Duration(intValue) * time.Second
			resetSet = true
		}
	}

	if !limitSet || !remainingSet || !resetSet {
		return 0, 0, 0, fmt.Errorf("missing required rate limit information: %s", header)
	}

	return limit, remaining, reset, nil
}

// Limiter is a grpc.UnaryClientInterceptor that updates the rate limiter based on the rate limit headers returned by the server
// malformed or otherwise incorrect headers are discarded with errors logged but non-halting
// messages without rate limits are considered valid
func (r *rateLimiter) Limiter(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	var header metadata.MD
	if err := invoker(ctx, method, req, reply, cc, append(opts, grpc.Header(&header))...); err != nil {
		return err
	}

	rateLimitHeader := header.Get("ratelimit")
	if len(rateLimitHeader) == 0 {
		return nil // no rate limit headers
	}

	limit, remaining, reset, err := parseRateLimit(rateLimitHeader[0])
	if err != nil {
		// if the parser returns an error we log it but otherwise treat it the same as not having a ratelimit header
		tflog.Warn(ctx, "failed to parse rate limit header", map[string]any{
			"error":  err.Error(),
			"header": rateLimitHeader[0],
		})
		// lint:ignore nilerr logging and not returning error as it is not fail worthy
		return nil
	}

	newLimit := rate.Limit(limit / limitPeriod)
	newBurst := limit / burstPeriod

	if r.limiter.Limit() != newLimit || r.limiter.Burst() != newBurst {
		tflog.Debug(ctx, "updating rate limiter", map[string]any{
			"new_limit": newLimit,
			"new_burst": newBurst,
		})
		r.limiter.SetLimit(newLimit)
		r.limiter.SetBurst(newBurst)
	}

	if remaining == 0 && reset > 0 {
		tflog.Warn(ctx, "rate limit exceeded", map[string]any{
			"limit":     limit,
			"remaining": remaining,
			"reset":     reset,
		})
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(reset + 1*time.Second):
		}
	}

	tflog.Debug(ctx, "rate limit updated", map[string]any{
		"limit":     limit,
		"remaining": remaining,
		"reset":     reset,
	})
	return r.limiter.Wait(ctx)
}
