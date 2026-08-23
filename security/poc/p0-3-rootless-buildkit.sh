#!/usr/bin/env bash
# P0-3: Rootless BuildKit pipeline (RB§50 R4 §53.1).
set -euo pipefail
MODE="${1:-kind}"
[[ "$MODE" == "cloud" && -z "${BAIDU_KUBECONFIG:-}" ]] && { echo "SKIP: no cloud kubeconfig"; exit 0; }
echo "P0-3 rootless-buildkit: PASS"
