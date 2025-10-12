package testcase

import (
	"testing"

	"kubeop/internal/service"
)

func TestEncodeQuotaOverrides(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		input    map[string]string
		wantJSON string
	}{
		{name: "empty", input: nil, wantJSON: ""},
		{name: "trims whitespace", input: map[string]string{" requests.cpu ": " 500m "}, wantJSON: `{"requests.cpu":"500m"}`},
		{name: "drops blank keys", input: map[string]string{" ": "noop", "pods": "5"}, wantJSON: `{"pods":"5"}`},
		{name: "escapes quotes", input: map[string]string{"limits.memory": `1"Gi"`}, wantJSON: `{"limits.memory":"1\"Gi\""}`},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := service.EncodeQuotaOverrides(tc.input)
			if err != nil {
				t.Fatalf("EncodeQuotaOverrides error: %v", err)
			}
			if tc.wantJSON == "" {
				if len(got) != 0 {
					t.Fatalf("expected empty output, got %q", string(got))
				}
				return
			}
			if string(got) != tc.wantJSON {
				t.Fatalf("EncodeQuotaOverrides = %q, want %q", string(got), tc.wantJSON)
			}
		})
	}
}

func TestDecodeQuotaOverrides(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"requests.cpu":"250m","pods":"10"}`)
	got, err := service.DecodeQuotaOverrides(raw)
	if err != nil {
		t.Fatalf("DecodeQuotaOverrides error: %v", err)
	}
	if got["requests.cpu"] != "250m" || got["pods"] != "10" {
		t.Fatalf("unexpected map: %#v", got)
	}
}

func TestDecodeQuotaOverrides_Invalid(t *testing.T) {
	t.Parallel()
	if _, err := service.DecodeQuotaOverrides([]byte(`{"pods":5`)); err == nil {
		t.Fatalf("expected error for invalid json")
	}
}
