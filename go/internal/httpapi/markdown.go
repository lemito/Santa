package httpapi

import (
	"bytes"
	"html/template"
	"sync"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

var (
	markdownEngine = goldmark.New(goldmark.WithExtensions(extension.GFM))
	markdownOnce   sync.Once
	markdownPolicy *bluemonday.Policy
)

func safeMarkdown(source string) template.HTML {
	if source == "" {
		return ""
	}
	markdownOnce.Do(func() {
		p := bluemonday.NewPolicy()
		p.AllowElements("p", "br", "strong", "em", "del", "code", "pre", "blockquote",
			"ul", "ol", "li", "h1", "h2", "h3", "h4", "a", "img", "hr",
			"table", "thead", "tbody", "tr", "th", "td")
		p.AllowAttrs("href", "title").OnElements("a")
		p.AllowAttrs("src", "alt", "title").OnElements("img")
		p.AllowURLSchemes("http", "https")
		p.AllowRelativeURLs(false)
		p.RequireNoFollowOnLinks(true)
		p.RequireNoReferrerOnLinks(true)
		markdownPolicy = p
	})
	var out bytes.Buffer
	if err := markdownEngine.Convert([]byte(source), &out); err != nil {
		return ""
	}
	return template.HTML(markdownPolicy.SanitizeBytes(out.Bytes()))
}
