package testcase

import (
	"testing"

	"go.uber.org/zap"

	"kubeop/internal/service"
)

func TestDNSLogFieldsForTest_AppendedIDs(t *testing.T) {
	extras := []zap.Field{zap.String("host", "app.example.com")}
	fields := service.DNSLogFieldsForTest("proj123", "app456", extras...)
	if len(fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(fields))
	}
	if fields[0].Key != "host" || fields[0].String != "app.example.com" {
		t.Fatalf("unexpected first field: %#v", fields[0])
	}
	if fields[1].Key != "project_id" || fields[1].String != "proj123" {
		t.Fatalf("expected project_id field, got %#v", fields[1])
	}
	if fields[2].Key != "app_id" || fields[2].String != "app456" {
		t.Fatalf("expected app_id field, got %#v", fields[2])
	}
	if len(extras) != 1 {
		t.Fatalf("expected extras slice to remain length 1, got %d", len(extras))
	}
}

func TestDNSLogFieldsForTest_EmptyIDs(t *testing.T) {
	fields := service.DNSLogFieldsForTest("", "", zap.String("foo", "bar"))
	if len(fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(fields))
	}
	if fields[0].Key != "foo" || fields[0].String != "bar" {
		t.Fatalf("unexpected field value: %#v", fields[0])
	}
}
