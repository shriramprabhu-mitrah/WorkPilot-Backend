package utils

import (
	"regexp"
	"strings"
)

var nonAlphanumericRegex = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify converts a string into a URL-safe lowercase alphanumeric slug.
func Slugify(s string) string {
	s = strings.ToLower(s)
	s = nonAlphanumericRegex.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}
