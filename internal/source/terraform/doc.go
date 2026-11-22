// Package terraform reads Terraform state and plan JSON into graph inputs.
//
// It parses the documents produced by `terraform show -json` with
// hashicorp/terraform-json. It reads files the user points at. It does not
// run terraform apply and it does not call a cloud API.
package terraform
