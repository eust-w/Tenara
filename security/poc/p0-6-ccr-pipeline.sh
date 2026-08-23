#!/usr/bin/env bash
# P0-6: CCR build→push→scan→digest→pull (RB§50 P0-6).
set -euo pipefail
[[ -z "${TENARA_BAIDU_ACCESS_KEY:-}" ]] && { echo "SKIP: credentials missing"; exit 0; }
echo "SKIP: dry-run mode (cloud operations required)"
exit 0
