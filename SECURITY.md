# Security policy

tetherwise is a read-only tool. A path that mutates a cloud or cluster
resource, a path that hands credentials to a model, or a log that keeps a
secret are in scope for this policy, including before the first release.

## Reporting

Report vulnerabilities in private. Do not open a public issue, and do not
include exploit details in a pull request.

Use GitHub private vulnerability reporting:

<https://github.com/manthan-parmar-1998/tetherwise/security/advisories/new>

The repository owner has to enable private vulnerability reporting (Settings,
Code security, Private vulnerability reporting) before that page accepts
reports. Until it does, email the maintainer at parmarmanthan1313@gmail.com.

The maintainer will acknowledge the report and coordinate a fix before public
disclosure. There is no release yet, so the fix will land on `main` first.

## Supported versions

| Version | Supported |
| --- | --- |
| `main` | yes |
| tagged releases | none yet |

After the first release, only the latest minor line is supported.

## What to include

- The version (`tetherwise version`) or the commit.
- The smallest synthetic case that shows the problem.
- Whether the issue is a mutation, a credential leak, or something else.

Do not send real Terraform state, kubeconfigs, account identifiers, or
production metrics.
