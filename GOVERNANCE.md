# Governance

tetherwise has one maintainer.

[Manthan Parmar](https://github.com/manthan-parmar-1998) (@manthan-parmar-1998) decides what merges, what a roadmap line means, and when a release is cut. There is no steering committee and no separate committer group. The current list is [MAINTAINERS.md](MAINTAINERS.md).

## How a change lands

1. Open an issue for anything larger than a small fix.
2. A pull request follows [CONTRIBUTING.md](CONTRIBUTING.md), including the Developer Certificate of Origin sign-off.
3. The maintainer reviews and merges.

Disagreement is settled by the maintainer, in the open, on the issue or the pull request.

## Maintainers

Adding or removing a maintainer is a decision of the current maintainer. It is recorded by a pull request that updates [MAINTAINERS.md](MAINTAINERS.md) and, if the rules change, this file.

## Conduct

The [Code of Conduct](CODE_OF_CONDUCT.md) applies to issues, pull requests, and any other project space. The maintainer enforces it.

## License

The project is Apache-2.0. See [LICENSE](LICENSE). Incoming contributions are covered by the Developer Certificate of Origin, described in [CONTRIBUTING.md](CONTRIBUTING.md).

## Safety outranks convenience

The safety principles in the README are part of how this project is governed. The maintainer will not merge a change that mutates cloud or cluster resources, gives credentials to a model, or commits real infrastructure data.
