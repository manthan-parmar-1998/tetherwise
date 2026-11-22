// Package safety proves the process is read-only before any cloud call.
//
// When AWS credentials are configured, Prove calls sts:GetCallerIdentity
// and iam:SimulatePrincipalPolicy for a fixed list of mutating actions.
// If any of those actions is allowed, Prove refuses to continue unless
// the caller passed --i-know. When a Kubernetes client will be used, Prove
// submits SelfSubjectAccessReviews for create, update, delete, and patch
// on workloads. The review itself is posted with the HTTP verb "create"
// because that is how the API server accepts a review; it does not create
// a workload. forbidigo cannot see the verb string, and the call site uses
// RESTClient.Verb so the static ban on client-go Create stays in force.
//
// Floci and the AWS Emulator do not evaluate IAM the way AWS does. The
// last check of this gate is a throwaway real account. See docs/safety.md.
package safety
