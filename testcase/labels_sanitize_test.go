package testcase

import (
	"kubeop/internal/service"
	"testing"
)

func TestSanitizeUserLabel(t *testing.T) {
	cases := map[string]string{
		"Alice":               "alice",
		"Alice Smith":         "alice-smith",
		"alice@example.com":   "alice-example.com",
		"UPPER_lower.123":     "upper_lower.123",
		"weird@@@chars__--..": "weird-chars__--..",
		"   spaced   out   ":  "spaced-out",
		"":                    "",
	}
	for in, want := range cases {
		got := service.SanitizeUserLabel(in)
		if got != want {
			t.Fatalf("SanitizeUserLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolveUserLabel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		inName   string
		inEmail  string
		inUserID string
		expected string
	}{
		{name: "prefers name", inName: "Alice Smith", inEmail: "ignored@example.com", inUserID: "user-1234", expected: "alice-smith"},
		{name: "falls back to email", inName: "", inEmail: "bob+ops@example.com", inUserID: "bob-1234", expected: "bob-ops-example.com"},
		{name: "falls back to id", inName: "", inEmail: "", inUserID: "abcdef123456", expected: "user-abcdef12"},
		{name: "empty everything", inName: "", inEmail: "", inUserID: "", expected: "user"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := service.ResolveUserLabel(tc.inName, tc.inEmail, tc.inUserID)
			if got != tc.expected {
				t.Fatalf("ResolveUserLabel(%q,%q,%q) = %q, want %q", tc.inName, tc.inEmail, tc.inUserID, got, tc.expected)
			}
		})
	}
}
