package httpapi

import (
	"fmt"
	"testing"
	"time"
)

func TestRateLimiterBoundedKeys(t *testing.T) {
	r := NewRateLimiter(1, time.Minute)
	for i := 0; i < defaultRateLimiterMaxKeys+100; i++ {
		r.Allow(fmt.Sprintf("client-%d", i))
	}
	if len(r.hits) > defaultRateLimiterMaxKeys {
		t.Fatalf("rate limiter retained %d keys, max %d", len(r.hits), defaultRateLimiterMaxKeys)
	}
}
