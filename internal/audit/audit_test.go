package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAppendAndRedact(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "logs", "audit.jsonl")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	logger, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	logger.now = func() time.Time { return time.Date(2026, 11, 22, 1, 2, 3, 0, time.UTC) }
	target := "arn:aws:iam::123456789012:role/lab password=hunter2 AKIAIOSFODNN7EXAMPLE"
	if err := logger.Record("sts", "GetCallerIdentity", target); err != nil {
		t.Fatal(err)
	}
	if err := logger.Close(); err != nil {
		t.Fatal(err)
	}
	logger2, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	logger2.now = func() time.Time { return time.Date(2026, 11, 22, 1, 2, 4, 0, time.UTC) }
	if err := logger2.Record("cloudwatch", "GetMetricData", "aws:opensearch:domain/search-lab"); err != nil {
		t.Fatal(err)
	}
	if err := logger2.Close(); err != nil {
		t.Fatal(err)
	}
	buf, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(buf)
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %q", text)
	}
	if !strings.Contains(lines[0], "REDACTED") || strings.Contains(lines[0], "123456789012") || strings.Contains(lines[0], "hunter2") || strings.Contains(lines[0], "AKIAIOSFODNN7EXAMPLE") {
		t.Fatalf("redaction failed: %s", lines[0])
	}
	if !strings.Contains(lines[0], "2026-11-22T01:02:03Z") {
		t.Fatalf("time = %s", lines[0])
	}
	if !strings.Contains(lines[1], "search-lab") {
		t.Fatalf("second line = %s", lines[1])
	}
}

func TestRedactAccountsOptional(t *testing.T) {
	t.Parallel()
	in := "arn:aws:es:us-east-1:123456789012:domain/search-lab token: abc"
	if got := Redact(in, false); strings.Contains(got, "123456789012") == false || strings.Contains(got, "abc") {
		t.Fatalf("optional accounts: %s", got)
	}
	if got := Redact(in, true); strings.Contains(got, "123456789012") || !strings.Contains(got, "token") || !strings.Contains(got, "REDACTED") {
		t.Fatalf("accounts on: %s", got)
	}
	if Redact("aws:opensearch:domain/search-lab", true) != "aws:opensearch:domain/search-lab" {
		t.Fatal("resource id changed")
	}
}

func TestOpenRejects(t *testing.T) {
	t.Parallel()
	if _, err := Open("", true); err == nil {
		t.Fatal("empty path")
	}
	logger, err := Open(filepath.Join(t.TempDir(), "a.jsonl"), false)
	if err != nil {
		t.Fatal(err)
	}
	if err := logger.Record("", "Get", "x"); err == nil {
		t.Fatal("empty service")
	}
	if err := logger.Close(); err != nil {
		t.Fatal(err)
	}
	if err := logger.Close(); err != nil {
		t.Fatal("second close")
	}
	var none *Logger
	if err := none.Record("sts", "GetCallerIdentity", "x"); err != nil {
		t.Fatal(err)
	}
	if err := none.Close(); err != nil {
		t.Fatal(err)
	}
}
