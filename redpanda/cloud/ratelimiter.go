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
	limitPeriod = 60.0
	burstPeriod = 10.0
)

type rateLimiter struct {
	limiter *rate.Limiter
}

func newRateLimiter(limit int) *rateLimiter {
	return &rateLimiter{
		limiter: rate.NewLimiter(rate.Limit(limit/limitPeriod), limit/burstPeriod),
	}
}

func parseRateLimit(header string) (limit, remaining int, reset time.Duration, err error) {
	for _, part := range strings.Split(header, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "limit":
			limit, err = strconv.Atoi(kv[1])
			if err != nil {
				return 0, 0, 0, fmt.Errorf("invalid limit value: %v", err)
			}
		case "remaining":
			remaining, err = strconv.Atoi(kv[1])
			if err != nil {
				return 0, 0, 0, fmt.Errorf("invalid remaining value: %v", err)
			}
		case "reset":
			seconds, err := strconv.Atoi(kv[1])
			if err != nil {
				return 0, 0, 0, fmt.Errorf("invalid reset value: %v", err)
			}
			reset = time.Duration(seconds) * time.Second
		}
	}
	return limit, remaining, reset, nil
}

func (r *rateLimiter) Limiter(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	var header metadata.MD
	if err := invoker(ctx, method, req, reply, cc, append(opts, grpc.Header(&header))...); err != nil {
		return err
	}

	if rateLimitHeader := header.Get("ratelimit"); len(rateLimitHeader) > 0 {
		tflog.Debug(ctx, "parsing rate limit headers")
		limit, remaining, reset, err := parseRateLimit(rateLimitHeader[0])
		if err != nil {
			return fmt.Errorf("failed to parse rate limit header: %v", err)
		}

		tflog.Debug(ctx, "setting limit and burst")
		r.limiter.SetLimit(rate.Limit(limit / limitPeriod))
		r.limiter.SetBurst(limit / burstPeriod)
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
	}
	return r.limiter.Wait(ctx)
}
