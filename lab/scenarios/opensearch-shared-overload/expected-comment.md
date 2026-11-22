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
