package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConfiguredBodyLimitRejectsOversizedRequest(t *testing.T) {
	s := &Server{BodyLimit: 4}
	r := httptest.NewRequest(http.MethodPost, "/health", strings.NewReader("12345"))
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d, want 413", w.Code)
	}
}
