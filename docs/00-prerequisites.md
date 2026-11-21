# Prerequisites

Install these before the later lessons. This page is the tool list. It does not build the graph or run a check.

Versions below are the ones this scaffold was written against (November 2026).

## Go

Install Go 1.27.4, or a newer 1.27 patch, from <https://go.dev/dl/>.

`go.mod` sets `toolchain go1.27.4`. A Go that still supports toolchains (1.21 or newer) will download 1.27.4 on its own. Installing 1.27.4 yourself is the path these lessons assume.

Verify:

```sh
go version
```

You want `go1.27.4` or a newer 1.27 patch.

## Docker or Podman

Install Docker 23 or newer (<https://docs.docker.com/get-docker/>), or Podman (<https://podman.io/docs/installation>). The Floci image layers use zstd, which older Docker engines cannot pull. You also need Compose v2 (`docker compose`, not the standalone `docker-compose` binary).

The lab mounts a container socket so the emulator can start OpenSearch, RDS, and MSK containers. On Docker that socket is `/var/run/docker.sock`. On Podman, point the lab compose file at the Podman socket.

Verify:

```sh
docker version
docker compose version
```

## kind

Install kind from <https://kind.sigs.k8s.io/docs/user/quick-start/#installation>. The lab in this repository does not start a cluster yet. Later lessons will.

Verify:

```sh
kind version
```

## kubectl

Install kubectl from <https://kubernetes.io/docs/tasks/tools/>.

Verify:

```sh
kubectl version --client
```

## AWS CLI

Install AWS CLI v2 from <https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html>. The lab talks to the local emulator. Do not point these lessons at a real account.

Verify:

```sh
aws --version
```

## Terraform

Install Terraform from <https://developer.hashicorp.com/terraform/install>. tetherwise reads state and plan files. It does not run `terraform apply`.

Verify:

```sh
terraform version
```

## Floci CLI

Floci is the local AWS emulator. Install the CLI from <https://floci.io/floci/getting-started/installation/>:

```sh
brew install floci-io/floci/floci
```

or the upstream installer:

```sh
curl -fsSL https://floci.io/install.sh | sh
```

Verify:

```sh
floci version
```

The lab can also be started with Docker Compose alone. `eval $(floci env)` needs this CLI. The same variables can be exported by hand; see [lab/README.md](../lab/README.md).

## golangci-lint

Install golangci-lint v2.15.0 from <https://golangci-lint.run/docs/welcome/install/>. The config is v2. A v1 binary will reject `.golangci.yml`.

```sh
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
```

Verify:

```sh
golangci-lint version
```

You want `2.15.0`.

## For `make vuln`

govulncheck is not required to read the lessons. `make vuln` runs it at the version pinned in the Makefile:

```sh
go install golang.org/x/vuln/cmd/govulncheck@v1.10.0
```

`make vuln` also runs it with `go run` at that version, so a separate install is optional.

## make

`make` is required for `make build`, `make test`, `make lint`, `make vuln`, and `make fmt`. It ships with the usual build tools on Linux and macOS.

Verify:

```sh
make --version
```
