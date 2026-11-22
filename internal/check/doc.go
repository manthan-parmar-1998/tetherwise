// Package check turns a graph and metric headroom into exposure findings.
//
// TODO(manthan): For each dependency a change can reach, report exposure
// and a confidence level. Confidence should reflect where the edge came
// from and how fresh the headroom sample is. This package must not mutate
// cloud or cluster state.
package check
