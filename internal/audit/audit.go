package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

// Event is one external read.
type Event struct {
	Time      string `json:"time"`
	Service   string `json:"service"`
	Operation string `json:"operation"`
	Target    string `json:"target"`
}

// Logger appends events to a JSONL file.
type Logger struct {
	mu             sync.Mutex
	file           *os.File
	redactAccounts bool
	now            func() time.Time
}

// Open creates or appends to path. The file mode is 0600.
// redactAccounts replaces 12-digit account ids inside ARNs.
func Open(path string, redactAccounts bool) (*Logger, error) {
	if path == "" {
		return nil, fmt.Errorf("audit log path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create audit directory: %w", err)
	}
	// The path is the operator's local audit log, not a request parameter.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // G304: operator-selected audit path
	if err != nil {
		return nil, fmt.Errorf("open audit log: %w", err)
	}
	return &Logger{file: f, redactAccounts: redactAccounts, now: time.Now}, nil
}

// Record appends one event. It does not rewrite earlier lines.
func (l *Logger) Record(service, operation, target string) error {
	if l == nil {
		return nil
	}
	if service == "" || operation == "" {
		return fmt.Errorf("audit event needs a service and an operation")
	}
	event := Event{
		Time:      l.now().UTC().Format(time.RFC3339),
		Service:   Redact(service, l.redactAccounts),
		Operation: Redact(operation, l.redactAccounts),
		Target:    Redact(target, l.redactAccounts),
	}
	buf, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode audit event: %w", err)
	}
	buf = append(buf, '\n')
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, err := l.file.Write(buf); err != nil {
		return fmt.Errorf("append audit event: %w", err)
	}
	if err := l.file.Sync(); err != nil {
		return fmt.Errorf("sync audit log: %w", err)
	}
	return nil
}

// Close closes the file.
func (l *Logger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	err := l.file.Close()
	l.file = nil
	if err != nil {
		return fmt.Errorf("close audit log: %w", err)
	}
	return nil
}

var (
	arnAccount  = regexp.MustCompile(`(arn:aws[a-zA-Z0-9-]*:[^:\s]*:[^:\s]*:)(\d{12})(:)`)
	secretValue = regexp.MustCompile(`(?i)\b((?:aws_)?(?:password|secret|session_token|api_key|access_key|token))(\s*[:=]\s*)("[^"]*"|'[^']*'|\S+)`)
	accessKeyID = regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)
)

// Redact removes secret assignments and access-key ids.
// When accounts is true it also replaces ARN account ids.
func Redact(s string, accounts bool) string {
	s = secretValue.ReplaceAllString(s, "${1}${2}REDACTED")
	s = accessKeyID.ReplaceAllString(s, "REDACTED")
	if accounts {
		s = arnAccount.ReplaceAllString(s, "${1}REDACTED${3}")
	}
	return s
}
