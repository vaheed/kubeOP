package util

import (
	"regexp"
	"strings"
)

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify converts a string to a DNS-1123 compatible slug: lowercase, hyphen-separated, max 63 chars.
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonAlnum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 63 {
		s = s[:63]
	}
	if s == "" {
		s = "ns"
	}
	return s
}
