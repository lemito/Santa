package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClearSessionCookies(t *testing.T) {
	w := httptest.NewRecorder()
	clearSessionCookies(w, true)
	cookies := w.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("got %d cookies, want 2", len(cookies))
	}
	for _, cookie := range cookies {
		if cookie.MaxAge != -1 || cookie.Value != "" || cookie.Path != "/" || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode {
			t.Fatalf("cookie %#v was not cleared safely", cookie)
		}
	}
}
