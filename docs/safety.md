# Safety

tetherwise is read-only. Two mechanisms enforce that, and neither replaces the other.

## Static ban

[`.golangci.yml`](../.golangci.yml) uses forbidigo with type analysis. AWS SDK methods whose names start with a mutating verb (`Create`, `Delete`, `Update`, `Put`, `Start`, and the rest of the list) are rejected. client-go methods `Create`, `Update`, `Delete`, `Patch`, `Apply`, `Put`, and `Post` are rejected. A local helper named `Create` is not affected, and an import alias does not hide a banned method.

`Get`, `List`, `Describe`, and CloudWatch `GetMetricData` stay allowed. Do not silence a hit with `//nolint:forbidigo`. If a real read API is misnamed, change the pattern in a reviewed pull request.

One field on the CloudWatch request is named `StartTime`. That is a timestamp, not a call to `StartInstances`. The client sets that field by name so the method ban can keep matching `Start*`. The ban itself is unchanged.

## Startup proof

`internal/safety` runs before a cloud or cluster read.

When AWS credentials are configured (environment variables or a credentials file; the instance metadata service is not probed), the process calls:

1. `sts:GetCallerIdentity`
2. `iam:SimulatePrincipalPolicy` for a fixed list of mutating actions (`es:DeleteDomain`, `rds:ModifyDBInstance`, `kafka:DeleteCluster`, `cloudwatch:PutMetricData`, and the others in `MutatingActions`)

If any simulated action is allowed, the process refuses to run. If identity or simulation fails, it also refuses: read-only has to be shown, not assumed. `--i-know` continues anyway and prints a warning. Use it only in a disposable environment. Floci does not evaluate IAM, so the optional lab script passes `--i-know` and refuses to run unless `AWS_ENDPOINT_URL` is the local emulator.

When `--k8s-live` is set, the process submits a `SelfSubjectAccessReview` for `create`, `update`, `delete`, and `patch` on deployments, configmaps, and pods. An allowed verb refuses the run, with the same `--i-know` escape hatch. The review is posted with `RESTClient.Verb("create")`. That is how the API server accepts a review. It does not create a workload, and it is not a client-go `Create` method, so the static ban still sees a forbidden `Create` if someone adds one.

No AWS credentials and no live cluster means there is nothing to prove, and the offline demo does not contact a network.

## What the process is allowed to call

[`deploy/iam-policy.json`](../deploy/iam-policy.json) is the minimal policy:

- `sts:GetCallerIdentity`
- `iam:SimulatePrincipalPolicy`
- `cloudwatch:GetMetricData`, `cloudwatch:GetMetricStatistics`, `cloudwatch:ListMetrics`

[`deploy/clusterrole.yaml`](../deploy/clusterrole.yaml) allows `get`, `list`, and `watch` on workloads and ConfigMaps. It allows `create` only on `selfsubjectaccessreviews`, because that is the review itself.

## Audit log

`internal/audit` appends one JSON object per external read: time, service, operation, target. The file is opened with `O_APPEND` and mode `0600`. Records are not rewritten. Secret assignments and `AKIA` access-key ids are redacted. Twelve-digit account ids inside ARNs are redacted unless the logger is opened with that redaction off. The log stays on the machine. Pass `--audit` to choose the path. Nothing is written when the flag is absent.

## What this does not prove

Floci and the AWS Emulator do not evaluate IAM the way AWS does. A call AWS would deny can succeed in either one. The last check of this gate is a throwaway real account: no production data, credentials that can be discarded, and no standing CI job pointed at that account.
