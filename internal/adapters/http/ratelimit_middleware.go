package http

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter Middleware creates middleware to limit the rate of requests per IP.
func RateLimiterMiddleware(rdb *redis.Client, limit int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			// We get the client's IP address
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			
			// Using the Sliding Window Algorithm Based on Redis Sorted Set
			key := fmt.Sprintf("ratelimit:%s", ip)
			now := time.Now().UnixNano()
			windowStart := now - window.Nanoseconds()

			// 1. Delete all old entries (that have gone beyond the window)
			rdb.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(windowStart, 10))
			// 2. Add the current request
			rdb.ZAdd(ctx, key, redis.Z{Score: float64(now), Member: now})
			// 3. Count the number of requests in the window
			count, err := rdb.ZCard(ctx, key).Result()
			if err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			if int(count) > limit {
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}