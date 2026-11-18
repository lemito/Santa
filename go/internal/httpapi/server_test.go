package httpapi

import (
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestRateLimiterConcurrentFirstUse(t *testing.T) {
	r := NewRateLimiter(1, time.Minute)
	var wg sync.WaitGroup
	results := make(chan bool, 32)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- r.Allow("127.0.0.1") }()
	}
	wg.Wait()
	close(results)
	allowed := 0
	for v := range results {
		if v {
			allowed++
		}
	}
	if allowed != 1 {
		t.Fatalf("allowed=%d, want 1", allowed)
	}
}

func TestSameOrigin(t *testing.T) {
	r := httptest.NewRequest("POST", "https://santa.example/account/login", nil)
	if !sameOrigin(r) {
		t.Fatal("missing Origin should be accepted")
	}
	r.Header.Set("Origin", "https://santa.example")
	if !sameOrigin(r) {
		t.Fatal("same origin rejected")
	}
	r.Header.Set("Origin", "https://evil.example")
	if sameOrigin(r) {
		t.Fatal("cross origin accepted")
	}
}

func TestTemplatesLoad(t *testing.T) {
	if _, err := LoadTemplates("../../templates"); err != nil {
		t.Fatal(err)
	}
}
