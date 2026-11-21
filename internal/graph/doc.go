// Package graph builds the dependency graph.
//
// TODO(manthan): Merge Terraform, Kubernetes, and an optional hand-written
// edges file into one graph, and answer which other resources a change
// reaches. This package must not call a cloud API.
package graph
