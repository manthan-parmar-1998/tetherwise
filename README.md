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

1. **Read-only is proven at startup.** Before any cloud call, the process must show that it cannot mutate. That proof will live in `internal/safety`. It is not written yet.
2. **It never mutates.** No creates, deletes, updates, puts, or modifies, through the AWS APIs or through client-go. The `forbidigo` rules in [`.golangci.yml`](.golangci.yml) reject those calls at lint time. That is the static half of this rule. It is not a substitute for the startup proof.
3. **AI never holds credentials.** An assistant, MCP server, or plugin may see a snapshot the CLI already produced. It does not receive cloud keys, kubeconfigs, or session tokens.
4. **The audit log is append-only and redacted.** It stays on the machine. Secrets and account identifiers do not land in it. Entries are not rewritten in place.
5. **Local-first.** Graph, check, and report run on the operator's machine. Nothing in this design phones home.
6. **Demos use synthetic data only.** Do not commit real state files, kubeconfigs, account identifiers, or customer names. The lab uses made-up teams and services.

## Status

This repository is the prerequisite scaffold. The core is not implemented.

| Command | Behavior |
| --- | --- |
| `tetherwise version` | Prints the version injected at link time. |
| `tetherwise graph` | Prints `not implemented yet` and exits non-zero. |
| `tetherwise check` | Prints `not implemented yet` and exits non-zero. |

Each package under `internal/` is a `doc.go` with the responsibility and a `TODO(manthan)`. The maintainer is writing that code as a series of hard-way lessons. Public library types wait for the v0.4 plugin SDK under `pkg/`. Until then the code stays in `internal/`.

## Roadmap

- **v0.1** — `graph`, `check`, and the safety gate, plus the lab.
- **v0.2** — MCP server (snapshot-only), a Prometheus plugin, and a GitHub Action.
- **v0.3** — OpenTelemetry dependencies, multi-account, Backstage.
- **v0.4** — Plugin protocol and SDK, OPA examples.
- **v0.5** — Incident mode, HolmesGPT.
- **v0.6+** — Community plugins for Azure, GCP, and Datadog.

## Layout

```
cmd/tetherwise/                 CLI skeleton (cobra, viper)
internal/model/                 domain types (stub)
internal/source/terraform/      Terraform state and plan (stub)
internal/source/kubernetes/     Kubernetes config (stub)
internal/metrics/cloudwatch/    CloudWatch headroom (stub)
internal/graph/                 dependency graph (stub)
internal/check/                 exposure and confidence (stub)
internal/safety/                startup read-only proof (stub)
internal/report/                markdown comment and report.json (stub)
internal/audit/                 append-only redacted log (stub)
lab/                            Floci, for a synthetic shared-OpenSearch incident
docs/                           hard-way lessons, starting with prerequisites
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

Config loading is wired (`--config`, `./tetherwise.yaml`, `$HOME/.config/tetherwise/tetherwise.yaml`, and `TETHERWISE_` environment variables). The keys themselves are not defined yet.

## Lab

[lab/](lab/README.md) runs [Floci](https://floci.io/) as the local lab, so a later lesson can recreate a synthetic shared-OpenSearch overload. Future CI contract tests target the official [AWS Emulator](https://aws.amazon.com/blogs/aws/test-your-applications-against-aws-locally-with-aws-emulator/). Neither emulator evaluates IAM faithfully, so the read-only safety gate still needs a final test against a throwaway real AWS account. None of that is wired into CI.

## Releases

Pushing a `v*` tag runs [`.github/workflows/release.yml`](.github/workflows/release.yml). GoReleaser publishes binaries, `checksums.txt`, an SBOM for each archive and for the source tarball, and a keyless cosign signature over the checksum file. There is no signing key in the repository. Verification steps are in [`.goreleaser.yaml`](.goreleaser.yaml).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Commits need a Developer Certificate of Origin `Signed-off-by` trailer (`git commit -s`). The project has one maintainer; see [GOVERNANCE.md](GOVERNANCE.md) and [MAINTAINERS.md](MAINTAINERS.md).

## License

[Apache-2.0](LICENSE).
