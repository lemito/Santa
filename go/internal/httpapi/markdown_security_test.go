package httpapi

import (
	"strings"
	"testing"
)

func TestSafeMarkdownRejectsRemoteImages(t *testing.T) {
	html := string(safeMarkdown("![tracking](https://tracker.example/pixel.gif)"))
	if strings.Contains(html, "tracker.example") || strings.Contains(html, "<img") {
		t.Fatalf("remote image survived sanitization: %s", html)
	}
}
