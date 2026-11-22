# Edges file

`tetherwise.edges.yaml` declares dependencies the scanners cannot see. Every edge is confidence `high` and source `edges-file`. The file does not set those fields.

```yaml
apiVersion: tetherwise.io/v1
kind: Edges
edges:
  - from: k8s:deployment/keel/keel-query
    to: aws:opensearch:domain/search-lab
    kind: reads-from
```

| Field | Required | Meaning |
| --- | --- | --- |
| `apiVersion` | yes | `tetherwise.io/v1` |
| `kind` | yes | `Edges` |
| `edges` | yes | A list. Empty is valid. |
| `edges[].from` | yes | Consumer resource id. |
| `edges[].to` | yes | Dependency resource id. Must differ from `from`. |
| `edges[].kind` | yes | `writes-logs-to`, `reads-from`, or `connects-to`. |

Resource ids use the form in [architecture.md](architecture.md): `provider:kind/name`.

A dependency whose endpoint is not already in the graph is stored as an identity stub. A later source can fill in the team, the owner, and attributes. The first non-empty value wins.
