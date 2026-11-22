// Package check turns a graph, a plan's changes, and optional headroom
// into an exposure report.
//
// It traces which other teams share a resource the change can reach and
// attaches headroom. It does not predict load and it does not mutate
// cloud or cluster state.
package check
