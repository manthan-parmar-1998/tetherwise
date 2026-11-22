# Shared OpenSearch overload

A synthetic incident. No real team, service, account, or domain is named here.

## Story

Team Atlas runs `atlas-indexer`. Team Keel runs `keel-query` and `keel-checkout`. All three ship logs through Fluent Bit to one OpenSearch domain, `search-lab`. Keel also reads the domain for product search and typeahead. That read is declared in `tetherwise.edges.yaml` because the manifests do not say "read" on their own.

`search-lab` is already near its limits. Free storage is below the floor, and JVM memory pressure is above 85 percent. A pull request widens the EKS log subscription filter from error-only to everything, and raises log retention from 7 days to 30. The services in the diff are the log group and the filter. The services that feel it are Atlas and Keel.

tetherwise reports that exposure. It does not predict how much extra load the filter will add.

## Run it offline

From the repository root:

```sh
make demo
```

The comment it prints is checked in as `expected-comment.md`. The verdict is `high-exposure`.

```sh
make build
./bin/tetherwise check lab/scenarios/opensearch-shared-overload/plan.json \
  --k8s lab/scenarios/opensearch-shared-overload/k8s \
  --edges lab/scenarios/opensearch-shared-overload/tetherwise.edges.yaml \
  --metrics fixture:lab/scenarios/opensearch-shared-overload/metrics.json \
  --format markdown \
  --fail-on high
```

Exit code 2 means the verdict was `high-exposure` and `--fail-on high` was set.

## Files

| File | Role |
| --- | --- |
| `plan.json` | `terraform show -json` plan. The filter pattern and the retention both change. |
| `k8s/` | Atlas and Keel workloads, each with Fluent Bit pointed at `search-lab`. |
| `tetherwise.edges.yaml` | Keel's reads, confidence high. |
| `metrics.json` | Recorded CloudWatch samples. |
| `expected-comment.md` | Golden pull-request comment. |
| `online.sh` | Optional. Local Floci only. |

## Names

`Atlas`, `Keel`, `atlas-indexer`, `keel-query`, `keel-checkout`, and `search-lab` are synthetic. Do not replace them with a real account, a real endpoint, or a snapshot of production state.
