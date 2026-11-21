// Package terraform reads Terraform state and plan into graph inputs.
//
// TODO(manthan): Parse state and plan JSON into resources and edges.
// Read files the user points at. Never run terraform apply, and never call
// a cloud API from this package.
package terraform
