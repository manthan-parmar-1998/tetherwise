# Architecture

tetherwise v0.1 answers one question before merge: who else is exposed if this change lands? It reports exposure and how close a shared resource is to a limit. It does not predict the load the change will add.

## Four nouns

Everything the tool stores is one of four types in `internal/model`.

**Resource.** A cloud-neutral id, a kind, a provider, a name, an optional owner and team, and string attributes. Ids look like `aws:opensearch:domain/search-lab` or `k8s:deployment/atlas/atlas-indexer`. The provider is the part before the first colon, the kind is the part before the first slash, and the name is the rest. A kind may itself contain colons (`opensearch:domain`). A name may contain slashes (`atlas/atlas-indexer`).

**Dependency.** A directed edge from a consumer to something it uses. `kind` is `writes-logs-to`, `reads-from`, or `connects-to`. `source` is where the edge was observed (`terraform`, `k8s-env`, `fluentbit`, `vector`, `edges-file`). `confidence` is `high`, `medium`, or `low`, and it belongs on the edge. When two observations describe the same from, to, and kind, the graph keeps the higher confidence and records every source.

**Limit.** One metric on one resource: threshold, current value, and headroom. Headroom is positive when the resource is inside the limit. For a lower-is-better metric (CPU, JVM memory pressure, connections, disk used) headroom is threshold minus current. For a higher-is-better metric (free storage, free memory) headroom is current minus threshold.

**Change.** A resource, an action (`create`, `update`, `delete`, `replace`), and the attribute names whose values differ. `replace` is a Terraform delete-then-create or create-then-delete.

JSON from these types is stable: struct field order, sorted map keys, and callers sort slices before they marshal a document.

## Sources

| Source | What it reads | Confidence |
| --- | --- | --- |
| `terraform` | `terraform show -json` plan and state, via `hashicorp/terraform-json` | high for expression references and `depends_on`; medium for an endpoint or security-group value that matches another resource |
| `k8s-env` | env vars on Deployments, StatefulSets, DaemonSets, and Pods | high when the value contains a known endpoint attribute; medium when a DNS label matches the resource name |
| `fluentbit` | Fluent Bit `[OUTPUT]` blocks in a mounted ConfigMap | same matching rules, kind `writes-logs-to` |
| `vector` | Vector TOML sinks of type `elasticsearch` or `opensearch` | same matching rules, kind `writes-logs-to` |
| `edges-file` | `tetherwise.edges.yaml` | always high |

Kubernetes manifests are decoded with `sigs.k8s.io/yaml` into the typed objects in `k8s.io/api`. Team and owner come from the `team` and `owner` labels, then from the same keys under annotations, then from `tetherwise.io/team` and `tetherwise.io/owner`. A live read is optional (`--k8s-live`) and uses client-go List only.

The edges file schema is in [edges.md](edges.md).

Terraform types map to the ids above for OpenSearch, RDS, MSK, security groups, log groups, and subscription filters. Other `aws_*` resources get a generic id. Attributes whose names look like secrets are dropped. Data sources are skipped.

## Graph

`internal/graph` is an in-memory adjacency list. Edges are labeled and a pair of resources can have more than one kind of edge, so this is not a graph database and not a gonum graph.

`graph` merges the sources above. The first non-empty owner, team, and attribute value wins. A dependency endpoint that is not already a resource is inserted as an identity stub so a later source can fill it in.

Upstream follows edges from consumer to dependency. Downstream follows them the other way. Both are transitive. A shared resource is one whose direct consumers belong to two or more teams. A consumer with neither team nor owner does not count. Comparison is case-insensitive.

Snapshots are JSON with `schema_version` `v0.1`.

## Check

`tetherwise check` takes a plan and a graph.

The blast radius of a change is:

1. The changed resources.
2. Everything they depend on (upstream).
3. Everything that depends on them (downstream), and everything those dependents themselves depend on.

A shared resource in that set is in play. The services in the report are its direct consumers, other than the resources the plan itself is changing. Headroom for those shared resources is attached when a metrics provider was asked.

Verdicts:

| Verdict | When |
| --- | --- |
| `info` | The change does not reach a resource another team depends on. |
| `warn` | It reaches a shared resource whose headroom is healthy or unknown, or it reaches another team through a resource that is not shared. |
| `high-exposure` | A shared resource in the blast radius is at or past a limit, or the change deletes or replaces that shared resource. |

A limit is tight when headroom is zero or negative, or when headroom divided by the threshold is under 0.15. That fraction is the "near the limit" band. It is not a prediction of the new load.

## Coverage

Every result says what the check could see:

- which sources were loaded
- which sources contributed edges
- which resources have neither owner nor team
- which edges are low confidence

A resource with no owner cannot be counted as a team, so a gap here is a reason the shared-resource answer might be incomplete.

## Report

`internal/report` writes a markdown pull-request comment and `report.json`. Both carry the verdict, the changes, the exposed services, headroom, and coverage. `report.json` has `schema_version` `v0.1`.

## Metrics

`internal/metrics/cloudwatch` implements `Provider`. The CloudWatch implementation calls `GetMetricData` only, for:

| Kind | Metrics |
| --- | --- |
| `opensearch:domain` | FreeStorageSpace, JVMMemoryPressure, CPUUtilization, ClusterStatus.red |
| `rds:instance` | CPUUtilization, FreeableMemory, DatabaseConnections, FreeStorageSpace |
| `msk:cluster` | KafkaDataLogsDiskUsed, CpuUser |

Thresholds have defaults (OpenSearch JVM memory pressure 85 percent, free storage floor 20 GiB, and the rest in `DefaultThresholds`). A `thresholds` block in `tetherwise.yaml` overrides a non-zero field. `fixture:<path>` replays a recorded file so the lab and the tests run with no network.

OpenSearch queries add `ClientId` when the resource has an `account_id` or `client_id` attribute. MSK queries add `Broker ID` when `broker_id` is set.

## What is deliberately absent

No graph database. No plugin system. No network listener. No load model. Public library types stay out of `pkg/` until the v0.4 SDK.
