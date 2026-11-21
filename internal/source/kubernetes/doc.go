// Package kubernetes reads Kubernetes config into graph inputs.
//
// TODO(manthan): Discover workloads and the shared resources they reference,
// from manifests on disk and, when asked, from a read-only API client.
// Get, List, and Watch are in scope. Create, Update, Delete, Patch, and
// Apply are not. The forbidigo rules in .golangci.yml reject those calls.
package kubernetes
