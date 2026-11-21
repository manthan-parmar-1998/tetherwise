## Summary

<!-- What changed, and why. -->

## Safety

- [ ] This change does not mutate cloud or cluster resources.
- [ ] No `//nolint:forbidigo` was added to hide a mutating AWS SDK or client-go call.
- [ ] No real infrastructure data, account identifiers, kubeconfigs, or credentials are included.
- [ ] Demos and fixtures use synthetic names only.

## Developer Certificate of Origin

- [ ] Every commit has a `Signed-off-by` trailer that matches the commit author (`git commit -s`).

## Checks

- [ ] `make build test lint` passes locally, or this change is documentation only.
