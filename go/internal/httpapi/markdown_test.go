package httpapi

import (
	"strings"
	"testing"
)

func TestSafeMarkdownImagesAndXSS(t *testing.T) {
	html := string(safeMarkdown("## Подарок\n\n**готов**\n\n![Ёлка](https://example.test/tree.png)\n\n![bad](javascript:alert(1))\n\n![data](data:text/html;base64,PHNjcmlwdD4=)\n\n<script>alert(1)</script>"))
	if !strings.Contains(html, "<h2>") || !strings.Contains(html, "<strong>готов</strong>") {
		t.Fatalf("Markdown formatting missing: %s", html)
	}
	if strings.Contains(html, "<img") || strings.Contains(html, "https://example.test/tree.png") {
		t.Fatalf("remote image survived sanitization: %s", html)
	}
	lower := strings.ToLower(html)
	if strings.Contains(lower, "javascript:") || strings.Contains(lower, "data:") || strings.Contains(lower, "<script") {
		t.Fatalf("unsafe content survived: %s", html)
	}
}
