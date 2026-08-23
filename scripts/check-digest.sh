#!/usr/bin/env bash
# R3 digest-only gate: fail if any chart/values image reference uses a
# floating tag (:latest, version tag) instead of a sha256 digest.
set -euo pipefail

DIR="${1:-deploy/helm}"
fail=0

while IFS= read -r f; do
  # any "tag:" line whose value is not a sha256 digest is a violation
  while IFS= read -r -u 3 line; do
    echo "VIOLATION in $f: $line"
    fail=1
  done 3< <(grep -nE 'tag:[[:space:]]*"?([A-Za-z0-9._-]+)"?[[:space:]]*$' "$f" | grep -v 'sha256:')
  # any image: line containing ":latest" or a repo:tag form (repo@sha256 is fine)
  while IFS= read -r line; do
    echo "VIOLATION in $f: $line"
    fail=1
  done < <(grep -nE 'image:.*:(latest|[A-Za-z0-9._-]+-[0-9])' "$f" || true)
done < <(find "$DIR" -name '*.yaml')

if [ "$fail" -ne 0 ]; then exit 1; fi
echo "digest-only check passed for $DIR"
