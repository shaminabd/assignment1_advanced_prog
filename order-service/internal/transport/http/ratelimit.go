package http

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"
)

func RateLimitMiddleware(client *goredis.Client) gin.HandlerFunc {
	limit := int64(10)
	if raw := os.Getenv("RATE_LIMIT_REQUESTS"); raw != "" {
		if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	window := time.Minute
	if raw := os.Getenv("RATE_LIMIT_WINDOW"); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			window = parsed
		}
	}

	return func(ctx *gin.Context) {
		key := fmt.Sprintf("ratelimit:ip:%s", ctx.ClientIP())
		count, err := client.Incr(context.Background(), key).Result()
		if err != nil {
			ctx.Next()
			return
		}

		if count == 1 {
			_ = client.Expire(context.Background(), key, window).Err()
		}

		if count > limit {
			ctx.Header("Retry-After", strconv.FormatInt(int64(window.Seconds()), 10))
			ctx.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}

		ctx.Next()
	}
}
