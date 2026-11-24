package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestIDAndSecurityHeaders(t *testing.T) {
	s := &Server{}
	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	id := w.Header().Get("X-Request-ID")
	if id == "" || strings.ContainsAny(id, "\r\n") {
		t.Fatalf("invalid request id %q", id)
	}
	for key, want := range map[string]string{
		"Cache-Control":          "no-store",
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "no-referrer",
	} {
		if got := w.Header().Get(key); got != want {
			t.Fatalf("%s=%q, want %q", key, got, want)
		}
	}
}

func TestRequestIDAcceptsSafeIncomingValue(t *testing.T) {
	s := &Server{}
	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	r.Header.Set("X-Request-ID", "client-123")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if got := w.Header().Get("X-Request-ID"); got != "client-123" {
		t.Fatalf("request id=%q, want client-123", got)
	}

	r = httptest.NewRequest(http.MethodGet, "/health", nil)
	r.Header.Set("X-Request-ID", strings.Repeat("x", 200))
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if got := w.Header().Get("X-Request-ID"); got == strings.Repeat("x", 200) || got == "" {
		t.Fatalf("unsafe request id was accepted: %q", got)
	}
}
