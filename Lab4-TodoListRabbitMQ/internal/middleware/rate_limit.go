package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type visitor struct {
	started time.Time
	count   int
}
type RateLimiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	visitors map[string]visitor
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{limit: limit, window: window, visitors: make(map[string]visitor)}
}
func (r *RateLimiter) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		now := time.Now()
		key := c.ClientIP()
		r.mu.Lock()
		v := r.visitors[key]
		if v.started.IsZero() || now.Sub(v.started) >= r.window {
			v = visitor{started: now}
		}
		v.count++
		r.visitors[key] = v
		remaining := r.limit - v.count
		if remaining < 0 {
			remaining = 0
		}
		retry := int((r.window - now.Sub(v.started)).Seconds())
		if retry < 1 {
			retry = 1
		}
		r.mu.Unlock()
		c.Header("X-Limit-Remaining", strconv.Itoa(remaining))
		if v.count > r.limit {
			c.Header("Retry-After", strconv.Itoa(retry))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}
