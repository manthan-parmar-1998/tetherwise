#!/bin/sh
# Optional online path against a local Floci emulator.
# The supported demo is offline: `make demo`.
# This script refuses to talk to anything except a loopback Floci endpoint.
set -eu

endpoint="${AWS_ENDPOINT_URL:-}"
case "$endpoint" in
  http://localhost:4566|http://127.0.0.1:4566) ;;
  *)
    echo "refusing: set AWS_ENDPOINT_URL to http://127.0.0.1:4566 (local Floci only)" >&2
    exit 1
    ;;
esac

aws sts get-caller-identity

# Floci does not evaluate IAM. A principal that looks allowed to mutate
# makes the read-only proof refuse. --i-know is the lab escape hatch.
echo "Floci does not evaluate IAM. tetherwise will be run with --i-know." >&2

root=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
exec tetherwise check "$root/plan.json" \
  --k8s "$root/k8s" \
  --edges "$root/tetherwise.edges.yaml" \
  --metrics cloudwatch \
  --format markdown \
  --fail-on high \
  --i-know
