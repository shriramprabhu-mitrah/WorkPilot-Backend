package utils

import "github.com/microcosm-cc/bluemonday"

var htmlSanitizer = func() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.AllowElements("p", "br", "strong", "b", "em", "i", "u", "s", "ul", "ol", "li", "blockquote", "code", "pre")
	p.AllowAttrs("href").OnElements("a")
	p.AllowAttrs("title").OnElements("a")
	p.AllowAttrs("src", "alt", "title").OnElements("img")
	p.RequireNoFollowOnLinks(true)
	return p
}()

func SanitizeHTML(raw string) string {
	if raw == "" {
		return ""
	}
	return htmlSanitizer.Sanitize(raw)
}
