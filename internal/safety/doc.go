// Package safety proves the process is read-only before any cloud call.
//
// TODO(manthan): At startup, refuse to continue unless the process can
// show it has no mutating client and no path that would apply, create,
// update, or delete. An assistant, MCP server, or plugin must never
// receive credentials; enforce that here when those integrations exist.
//
// The forbidigo rules in .golangci.yml are the static half of this rule.
// They are not a substitute for the startup proof.
package safety
