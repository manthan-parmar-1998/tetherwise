# Shared OpenSearch overload

A synthetic incident for the lab. No real team, service, account, or domain is named here. The fixtures that will reproduce it are not written yet.

## Story

Two teams share one OpenSearch domain.

- Team Atlas runs `atlas-indexer`. It writes documents into the domain `search-lab`.
- Team Keel runs `keel-query` and `keel-checkout`. Both read `search-lab`. Checkout uses it for typeahead. Query uses it for product search.

`search-lab` is already near its limits. JVM memory pressure is high, and the search thread pool is rejecting work. Atlas opens a pull request that raises indexer throughput in Terraform and points a new Kubernetes worker at the same domain.

The question for tetherwise, which it does not answer yet:

- `keel-query` and `keel-checkout` are exposed, even though those services are not in the diff.
- The domain is close to the CloudWatch limits that matter for this overload.
- Each edge carries a confidence level: seen in Terraform state or plan, seen in Kubernetes config, or supplied by the hand-written edges file.

## Names

`Atlas`, `Keel`, `atlas-indexer`, `keel-query`, `keel-checkout`, and `search-lab` are synthetic. Do not replace them with a real account, a real endpoint, or a snapshot of production state.

## What will land in this directory

Manthan will add the Terraform, the Kubernetes manifests, the edges file, and the synthetic CloudWatch samples while building `internal/source/terraform`, `internal/source/kubernetes`, `internal/metrics/cloudwatch`, `internal/graph`, and `internal/check`. This file is only the story.
