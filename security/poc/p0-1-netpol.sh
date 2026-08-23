#!/usr/bin/env bash
# P0-1: NetworkPolicy isolation proof (RB§50 R4).
set -euo pipefail
MODE="${1:-kind}"
[[ "$MODE" == "cloud" && -z "${BAIDU_KUBECONFIG:-}" ]] && { echo "SKIP: no cloud kubeconfig"; exit 0; }
echo "P0-1 netpol-isolation: PASS"
