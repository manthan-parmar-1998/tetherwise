// Package graph builds the dependency graph.
//
// Terraform, Kubernetes, and an optional edges file merge into one
// in-memory adjacency list. Edges carry a kind, a confidence, and every
// source that observed them. The package does not call a cloud API.
//
// gonum is a possible fit for unlabeled graphs. These edges are labeled
// and can repeat between the same pair with different kinds, so the graph
// is a map plus a slice rather than a gonum graph.
package graph
