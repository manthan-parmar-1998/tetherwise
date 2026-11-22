## tetherwise

**Verdict:** high-exposure

This report names who else depends on the shared resources a change reaches, and the headroom on those resources.

### Change

| Resource | Action | Attributes |
| --- | --- | --- |
| aws:logs:subscription-filter/eks-logs | update | filter_pattern |

### Who else is exposed

| Service | Team | Owner | Shared resource | Relationship | Confidence | Headroom |
| --- | --- | --- | --- | --- | --- | --- |
| keel/keel-query | Keel | keel-oncall | aws:opensearch:domain/search-lab | reads-from | high | FreeStorageSpace -14480 (6000/20480 Megabytes) |

### Shared resources

| Resource | Teams | Headroom |
| --- | --- | --- |
| aws:opensearch:domain/search-lab | Atlas, Keel | JVMMemoryPressure -7 (92/85 Percent); FreeStorageSpace -14480 (6000/20480 Megabytes) |

### Why

- update aws:logs:subscription-filter/eks-logs changes filter_pattern.
- aws:opensearch:domain/search-lab is shared by Atlas, Keel.
- Verdict is high-exposure because a shared resource is at or past a limit, or is being deleted or replaced.

### Coverage

- Sources loaded: edges-file, terraform
- Edge sources: edges-file, terraform
- Resources with no owner: aws:logs:subscription-filter/eks-logs
- Low-confidence edges: none
