# tetherwise

[![CI](https://github.com/manthan-parmar-1998/tetherwise/actions/workflows/ci.yml/badge.svg)](https://github.com/manthan-parmar-1998/tetherwise/actions/workflows/ci.yml)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/manthan-parmar-1998/tetherwise/badge)](https://scorecard.dev/viewer/?uri=github.com/manthan-parmar-1998/tetherwise)

An infrastructure change rarely fails alone. A Terraform plan that raises indexing on a shared OpenSearch domain, or a manifest that points one more service at an RDS instance already near its connection ceiling, can take down a team that never touched the pull request. The diff shows what changed. It does not show the neighbors, or how close those shared resources are to their limits.

tetherwise is a read-only command-line tool for that question, asked before merge: **who else breaks?**

## What it is

- A local CLI with `graph`, `check`, and `version`.
- A dependency graph built from Terraform state and plan, Kubernetes config, and an optional hand-written edges file.
- A reading of shared resources (OpenSearch, RDS, MSK) and how close they are to their limits, using CloudWatch.
- A report of *exposure*, with a confidence level on each dependency, as a markdown pull-request comment and a versioned `report.json`.

It must never mutate cloud resources.

## What it is not

- An incident tool. Incident mode is a later milestone, and the investigation stays with [HolmesGPT](https://github.com/HolmesGPT/holmesgpt).
- A cost tool. Cost stays with [Infracost](https://www.infracost.io/).
- A policy engine. Admit and deny stay with [OPA](https://www.openpolicyagent.org/).
- A substitute for the systems it reads. It does not apply Terraform, it does not reconcile Kubernetes, and it does not change a cloud resource to find out what would happen.

## Safety principles

These are non-negotiable. A change that breaks one of them is a defect, including in a demo.

1. **Read-only is proven at startup.** Before any cloud call, the process must show that it cannot mutate. That proof is `internal/safety`, described in [docs/safety.md](docs/safety.md).
2. **It never mutates.** No creates, deletes, updates, puts, or modifies, through the AWS APIs or through client-go. The `forbidigo` rules in [`.golangci.yml`](.golangci.yml) reject those calls at lint time. That is the static half of this rule. It is not a substitute for the startup proof.
3. **AI never holds credentials.** An assistant, MCP server, or plugin may see a snapshot the CLI already produced. It does not receive cloud keys, kubeconfigs, or session tokens.
4. **The audit log is append-only and redacted.** It stays on the machine. Secrets and account identifiers do not land in it. Entries are not rewritten in place.
5. **Local-first.** Graph, check, and report run on the operator's machine. Nothing in this design phones home.
6. **Demos use synthetic data only.** Do not commit real state files, kubeconfigs, account identifiers, or customer names. The lab uses made-up teams and services.

## Status

v0.1 builds the graph, runs the check, and refuses to start when the caller is allowed to mutate. Public library types still wait for the v0.4 plugin SDK under `pkg/`. Until then the code stays in `internal/`.

| Command | Behavior |
| --- | --- |
| `tetherwise version` | Prints the version injected at link time. |
| `tetherwise graph` | Writes a versioned snapshot from Terraform, Kubernetes, and an edges file. |
| `tetherwise check` | Reports exposure for a plan. Exit 0, or exit 2 when `--fail-on high` and the verdict is `high-exposure`. |

How the four nouns, the sources, and the verdict fit together is in [docs/architecture.md](docs/architecture.md).

## Quickstart

The offline lab is the shared OpenSearch incident: a plan widens an EKS log filter and raises retention, and Atlas and Keel both ship logs to `search-lab`.

```sh
make demo
```

`make demo` prints the pull-request comment. This is that comment, from the fixtures in [lab/scenarios/opensearch-shared-overload](lab/scenarios/opensearch-shared-overload/README.md):

```markdown
## tetherwise

**Verdict:** high-exposure

This report names who else depends on the shared resources a change reaches, and the headroom on those resources.

### Change

| Resource | Action | Attributes |
| --- | --- | --- |
| aws:logs:log-group/eks-lab | update | retention_in_days |
| aws:logs:subscription-filter/eks-logs | update | filter_pattern |

### Who else is exposed

| Service | Team | Owner | Shared resource | Relationship | Confidence | Headroom |
| --- | --- | --- | --- | --- | --- | --- |
| atlas/atlas-indexer | Atlas | atlas-oncall | aws:opensearch:domain/search-lab | connects-to | high | FreeStorageSpace -14480 (6000/20480 Megabytes) |
| atlas/atlas-indexer | Atlas | atlas-oncall | aws:opensearch:domain/search-lab | writes-logs-to | high | FreeStorageSpace -14480 (6000/20480 Megabytes) |
| keel/keel-checkout | Keel | keel-oncall | aws:opensearch:domain/search-lab | connects-to | high | FreeStorageSpace -14480 (6000/20480 Megabytes) |
| keel/keel-checkout | Keel | keel-oncall | aws:opensearch:domain/search-lab | reads-from | high | FreeStorageSpace -14480 (6000/20480 Megabytes) |
| keel/keel-checkout | Keel | keel-oncall | aws:opensearch:domain/search-lab | writes-logs-to | high | FreeStorageSpace -14480 (6000/20480 Megabytes) |
| keel/keel-query | Keel | keel-oncall | aws:opensearch:domain/search-lab | connects-to | high | FreeStorageSpace -14480 (6000/20480 Megabytes) |
| keel/keel-query | Keel | keel-oncall | aws:opensearch:domain/search-lab | reads-from | high | FreeStorageSpace -14480 (6000/20480 Megabytes) |
| keel/keel-query | Keel | keel-oncall | aws:opensearch:domain/search-lab | writes-logs-to | high | FreeStorageSpace -14480 (6000/20480 Megabytes) |

### Shared resources

| Resource | Teams | Headroom |
| --- | --- | --- |
| aws:opensearch:domain/search-lab | Atlas, Keel | CPUUtilization 40 (40/80 Percent); ClusterStatus.red 1 (0/1 Count); FreeStorageSpace -14480 (6000/20480 Megabytes); JVMMemoryPressure -7 (92/85 Percent) |

### Why

- update aws:logs:log-group/eks-lab changes retention_in_days.
- update aws:logs:subscription-filter/eks-logs changes filter_pattern.
- aws:opensearch:domain/search-lab is shared by Atlas, Keel.
- FreeStorageSpace on aws:opensearch:domain/search-lab has headroom -14480 (current 6000, threshold 20480).
- JVMMemoryPressure on aws:opensearch:domain/search-lab has headroom -7 (current 92, threshold 85).
- Verdict is high-exposure because a shared resource is at or past a limit, or is being deleted or replaced.

### Coverage

- Sources loaded: edges-file, fixture, kubernetes, terraform
- Edge sources: edges-file, fluentbit, k8s-env, terraform
- Resources with no owner: aws:logs:log-group/eks-lab, aws:logs:subscription-filter/eks-logs
- Low-confidence edges: none
```

Headroom is negative on free storage and on JVM memory pressure. Keel did not touch the pull request. The report does not say how much load the filter change will add.

```sh
make build
./bin/tetherwise graph \
  --tf-plan lab/scenarios/opensearch-shared-overload/plan.json \
  --k8s lab/scenarios/opensearch-shared-overload/k8s \
  --edges lab/scenarios/opensearch-shared-overload/tetherwise.edges.yaml \
  -o snapshot.json
./bin/tetherwise check lab/scenarios/opensearch-shared-overload/plan.json \
  --snapshot snapshot.json \
  --metrics fixture:lab/scenarios/opensearch-shared-overload/metrics.json \
  --format markdown \
  --fail-on high
```

The last command exits 2 because the verdict is `high-exposure`. Exit 1 is an error. Exit 0 is a completed check that was not asked to fail.

## Roadmap

- **v0.1** — `graph`, `check`, and the safety gate, plus the lab.
- **v0.2** — MCP server (snapshot-only), a Prometheus plugin, and a GitHub Action.
- **v0.3** — OpenTelemetry dependencies, multi-account, Backstage.
- **v0.4** — Plugin protocol and SDK, OPA examples.
- **v0.5** — Incident mode, HolmesGPT.
- **v0.6+** — Community plugins for Azure, GCP, and Datadog.

## Layout

```
cmd/tetherwise/                 CLI (graph, check, version)
internal/model/                 resource, dependency, limit, change
internal/source/terraform/      terraform show -json plan and state
internal/source/kubernetes/     manifests and optional read-only list
internal/source/edges/          tetherwise.edges.yaml
internal/metrics/cloudwatch/    GetMetricData and recorded fixtures
internal/graph/                 merge, snapshot, shared resources
internal/check/                 exposure, confidence, coverage
internal/safety/                startup read-only proof
internal/report/                markdown comment and report.json
internal/audit/                 append-only redacted log
deploy/                         minimal IAM policy and ClusterRole
lab/                            offline fixtures and optional Floci
docs/                           architecture, safety, edges schema
```

## Develop

Install the tools in [docs/00-prerequisites.md](docs/00-prerequisites.md).

```sh
make build
make test
make lint
make vuln
make fmt
./bin/tetherwise version
```

`make build` passes `-X main.version`, `-X main.commit`, and `-X main.date`. A release build sets the same variables from the git tag. See [`.goreleaser.yaml`](.goreleaser.yaml).

Config loading is wired (`--config`, `./tetherwise.yaml`, `$HOME/.config/tetherwise/tetherwise.yaml`, and `TETHERWISE_` environment variables). A `thresholds` block overrides the metric limits. `--i-know` and `--audit` bind to the same file.

## Lab

[lab/](lab/README.md) runs [Floci](https://floci.io/) as the local lab. The shared-OpenSearch scenario also runs offline, with no emulator, via `make demo`. An optional script in that scenario talks to Floci on port 4566 and refuses any other endpoint. Future CI contract tests target the official [AWS Emulator](https://aws.amazon.com/blogs/aws/test-your-applications-against-aws-locally-with-aws-emulator/). Neither emulator evaluates IAM faithfully, so the read-only safety gate still needs a final test against a throwaway real AWS account. That test is not a CI job.

## Releases

Pushing a `v*` tag runs [`.github/workflows/release.yml`](.github/workflows/release.yml). GoReleaser publishes binaries, `checksums.txt`, an SBOM for each archive and for the source tarball, and a keyless cosign signature over the checksum file. There is no signing key in the repository. Verification steps are in [`.goreleaser.yaml`](.goreleaser.yaml).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Commits need a Developer Certificate of Origin `Signed-off-by` trailer (`git commit -s`). The project has one maintainer; see [GOVERNANCE.md](GOVERNANCE.md) and [MAINTAINERS.md](MAINTAINERS.md).

## License

[Apache-2.0](LICENSE).
