package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestHandlerRejectsUnsupportedMethods(t *testing.T) {
	s := &Server{Templates: &Templates{}}
	r := httptest.NewRequest(http.MethodDelete, "/health", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d, want 405", w.Code)
	}
}

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

func TestTemplatesLoad(t *testing.T) {
	if _, err := LoadTemplates("../../templates"); err != nil {
		t.Fatal(err)
	}
}

func TestTemplatesEscapeGameFieldsAndKeepRequiredControls(t *testing.T) {
	templates, err := LoadTemplates("../../templates")
	if err != nil {
		t.Fatal(err)
	}
	data := struct {
		Page          string
		Title         string
		Authenticated bool
		CSRF          string
		Games         []Game
		Game          *Game
	}{Page: "home", Title: "Home", Authenticated: true, CSRF: "csrf", Games: []Game{{ID: "game", Title: `<script>alert(1)</script>`, State: "Draft"}}}
	var output strings.Builder
	if err := templates.T.ExecuteTemplate(&output, "layout", data); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "<script>alert") || !strings.Contains(output.String(), `name="csrfToken"`) {
		t.Fatal("template escaped content or required CSRF control incorrectly")
	}
}
