package service

import (
    "strings"
)

// SanitizeUserLabel returns a lowercase label suitable for kubeconfig user names.
// It replaces any character outside [a-z0-9._-] with '-'.
// Multiple consecutive separators are collapsed to a single '-'.
// Leading/trailing separators are trimmed.
func SanitizeUserLabel(s string) string {
    if s == "" { return "" }
    // lower case for consistency
    s = strings.ToLower(strings.TrimSpace(s))
    var b strings.Builder
    prevDash := false
    for _, r := range s {
        if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
            // allow as-is
            b.WriteRune(r)
            prevDash = (r == '-')
            continue
        }
        // replace other characters (including spaces and '@') with '-'
        if !prevDash {
            b.WriteByte('-')
            prevDash = true
        }
    }
    out := b.String()
    out = strings.Trim(out, "-")
    if out == "" { return "user" }
    return out
}

