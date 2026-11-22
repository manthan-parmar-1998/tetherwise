# Contributing

tetherwise is Apache-2.0. Contributions are welcome. The maintainer is listed in [MAINTAINERS.md](MAINTAINERS.md), and decisions follow [GOVERNANCE.md](GOVERNANCE.md).

By participating you agree to the [Code of Conduct](CODE_OF_CONDUCT.md).

## Developer Certificate of Origin

Every commit must include a `Signed-off-by` trailer. That trailer is the [Developer Certificate of Origin](https://developercertificate.org/):

```
Developer Certificate of Origin
Version 1.1

Copyright (C) 2004, 2006 The Linux Foundation and its contributors.

Everyone is permitted to copy and distribute verbatim copies of this
license document, but changing it is not allowed.

Developer's Certificate of Origin 1.1

By making a contribution to this project, I certify that:

(a) The contribution was created in whole or in part by me and I
    have the right to submit it under the open source license
    indicated in the file; or

(b) The contribution is based upon previous work that, to the best
    of my knowledge, is covered under an appropriate open source
    license and I have the right under that license to submit that
    work with modifications, whether created in whole or in part
    by me, under the same open source license (unless I am
    permitted to submit under a different license), as indicated
    in the file; or

(c) The contribution was provided directly to me by some other
    person who certified (a), (b) or (c) and I have not modified
    it.

(d) I understand and agree that this project and the contribution
    are public and that a record of the contribution (including all
    personal information I submit with it, including my sign-off) is
    maintained indefinitely and may be redistributed consistent with
    this project or the open source license(s) involved.
```

Add the trailer with `-s`:

```sh
git commit -s
```

The line must match the commit author exactly:

```
Signed-off-by: Your Name <you@example.com>
```

Set `user.name` and `user.email` to the identity you want on the trailer. A sign-off for a different name or email does not count.

This repository checks the trailer in [`.github/workflows/dco.yml`](.github/workflows/dco.yml). The check is named `DCO / check`. It skips merge commits and rejects any other commit whose trailer does not match the author.

A squash merge has to keep a `Signed-off-by` line in the squash commit message. GitHub does not copy the trailers from the individual commits unless the person merging puts them in the message. A merge commit is skipped by the check. A rebase merge keeps the original commits, trailers included.

The maintainer should also install the [DCO GitHub App](https://github.com/apps/dco) on this repository and require it in branch protection. The app is the usual required check. The workflow covers the repository until the app is installed, and it keeps working if the app is removed. Installing the app is a one-time step only the repository owner can do.

## What to work on

The packages under `internal/` are stubs. Each `doc.go` says what that package is for and leaves a `TODO(manthan)` for the maintainer, who is writing the core in the style of a hard-way lesson. Open an issue before filling one of those packages in.

Do not add real infrastructure: no production Terraform state, no kubeconfigs, no account identifiers, no customer or employer names. The lab uses synthetic names. Keep it that way.

## Read-only

tetherwise never mutates cloud or cluster resources. [`.golangci.yml`](.golangci.yml) enforces that with `forbidigo`:

- AWS SDK v1 and v2 methods whose names start with a mutating verb (`Create`, `Delete`, `Update`, `Put`, `Modify`, and the others listed in the config) are rejected.
- client-go methods `Create`, `Update`, `Delete`, `Patch`, `Apply`, `Put`, and `Post` are rejected.
- Read methods (`Get`, `List`, `Describe`, `Head`, `Watch`, CloudWatch `GetMetric*`) stay allowed.

Do not hide a hit with `//nolint:forbidigo`. If a read API is mis-classified because its name starts with one of those verbs, change the pattern in a reviewed pull request.

The linter is the static half of the rule. The startup proof that the process is read-only will live in `internal/safety`. It is not written yet.

## Build

Install the tools in [docs/00-prerequisites.md](docs/00-prerequisites.md), then:

```sh
make build
make test
make lint
make vuln
make fmt
```

`make build` injects the version with `-ldflags` (`main.version`, `main.commit`, `main.date`).

```sh
./bin/tetherwise version
./bin/tetherwise graph   # prints "not implemented yet" and exits non-zero
./bin/tetherwise check   # prints "not implemented yet" and exits non-zero
```

## Pull requests

Use the pull request template. One concern per pull request is easier to review. The maintainer merges.
