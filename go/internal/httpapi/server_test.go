package httpapi

import (
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestParseExclusions(t *testing.T) {
	pairs, err := parseExclusions(" Alice@example.com, BOB@example.com \n\n")
	if err != nil {
		t.Fatalf("parseExclusions() error = %v", err)
	}
	if len(pairs) != 1 || pairs[0].First != "alice@example.com" || pairs[0].Second != "bob@example.com" {
		t.Fatalf("pairs = %#v", pairs)
	}
}

func TestParseExclusionsRejectsMalformedLines(t *testing.T) {
	for _, input := range []string{"alice@example.com", "alice@example.com,bob@example.com,carol@example.com", ",bob@example.com", "alice@example.com,"} {
		if _, err := parseExclusions(input); err == nil || err.Error() != "invalid_exclusion" {
			t.Errorf("parseExclusions(%q) error = %v, want invalid_exclusion", input, err)
		}
	}
}

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
