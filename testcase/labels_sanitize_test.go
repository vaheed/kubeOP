package testcase

import (
    "testing"
    "kubeop/internal/service"
)

func TestSanitizeUserLabel(t *testing.T) {
    cases := map[string]string{
        "Alice":                    "alice",
        "Alice Smith":              "alice-smith",
        "alice@example.com":        "alice-example.com",
        "UPPER_lower.123":          "upper_lower.123",
        "weird@@@chars__--..":      "weird-chars__--..",
        "   spaced   out   ":       "spaced-out",
        "":                          "",
    }
    for in, want := range cases {
        got := service.SanitizeUserLabel(in)
        if got != want {
            t.Fatalf("SanitizeUserLabel(%q) = %q, want %q", in, got, want)
        }
    }
}
