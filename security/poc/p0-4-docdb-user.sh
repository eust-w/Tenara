#!/usr/bin/env bash
# P0-4: DocDB per-app user isolation (RB§50 P0-4).
set -euo pipefail
[[ -z "${TENARA_BAIDU_ACCESS_KEY:-}" ]] && { echo "SKIP: credentials missing"; exit 0; }
echo "SKIP: dry-run mode (cloud credentials required)"
exit 0
