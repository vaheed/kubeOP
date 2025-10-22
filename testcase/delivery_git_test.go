package testcase

import (
	"testing"

	"kubeop/internal/delivery"
)

func TestValidateCheckoutPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty", input: "", want: ""},
		{name: "dot", input: ".", want: ""},
		{name: "relative clean", input: "manifests/base", want: "manifests/base"},
		{name: "leading dot slash", input: "./overlays/dev", want: "overlays/dev"},
		{name: "trim whitespace", input: "  nested/configs  ", want: "nested/configs"},
		{name: "reject absolute", input: "/etc/passwd", wantErr: true},
		{name: "reject parent", input: "../secret", wantErr: true},
		{name: "reject parent prefix", input: "../../escape", wantErr: true},
		{name: "reject windows drive", input: "C:/configs", wantErr: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := delivery.ValidateCheckoutPath(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("ValidateCheckoutPath(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
