// Package audit is the local, append-only, redacted audit log.
//
// Each external read is one JSON object on its own line. Secrets are
// redacted before the line is written. Account identifiers inside ARNs
// are redacted when the logger is asked to. The file is opened append-only
// and is never rewritten in place. The log stays on the machine.
package audit
