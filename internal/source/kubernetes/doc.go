// Package kubernetes reads Kubernetes config into graph inputs.
//
// Manifests on disk are decoded with sigs.k8s.io/yaml and the typed API
// objects in k8s.io/api. A live read, when asked for, uses client-go Get
// and List only. Create, Update, Delete, Patch, and Apply are not used.
package kubernetes
