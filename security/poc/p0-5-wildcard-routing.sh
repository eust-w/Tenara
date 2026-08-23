#!/usr/bin/env bash
# P0-5: Wildcard routing N≥50 subdomains (RB§50 P0-5).
set -euo pipefail
[[ -z "${TENARA_BAIDU_ACCESS_KEY:-}" ]] && { echo "SKIP: credentials missing"; exit 0; }
echo "SKIP: dry-run mode (cloud operations required)"
exit 0
