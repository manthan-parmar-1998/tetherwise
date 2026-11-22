#!/usr/bin/env bash
# Fail unless every non-merge commit in the given git rev-list range has a
# Signed-off-by trailer that matches the commit author exactly.
#
# Usage: scripts/check-dco.sh <rev-list-range>
# Examples:
#   scripts/check-dco.sh origin/main..HEAD
#   scripts/check-dco.sh <sha>^!
set -euo pipefail

range="${1:?usage: scripts/check-dco.sh <rev-list-range>}"

missing=0
while read -r sha; do
  [ -n "$sha" ] || continue
  nparents=$(git show -s --format=%P "$sha" | awk '{print NF}')
  if [ "$nparents" -ge 2 ]; then
    echo "skip merge $sha"
    continue
  fi
  expected=$(git show -s --format='Signed-off-by: %an <%ae>' "$sha")
  body=$(git show -s --format=%B "$sha")
  if ! grep -Fxq -- "$expected" <<<"$body"; then
    echo "::error::Commit ${sha} is missing a DCO sign-off matching the author."
    echo "  expected: ${expected}"
    echo "  Add it with: git commit -s"
    missing=1
  else
    echo "ok $sha"
  fi
done < <(git rev-list "$range")

exit "$missing"
