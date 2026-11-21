// Package audit is the local, append-only, redacted audit log.
//
// TODO(manthan): Append one record per run. Redact secrets and account
// identifiers before they touch disk. Never rewrite or truncate an
// existing record in place, and never ship the log off the machine.
package audit
