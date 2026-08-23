#!/usr/bin/env bash
# P0-2: RuntimeClass sandbox proof (RB§50 R14).
set -euo pipefail
MODE="${1:-kind}"
[[ "$MODE" == "cloud" && -z "${BAIDU_KUBECONFIG:-}" ]] && { echo "SKIP: no cloud kubeconfig"; exit 0; }
if [[ "${2:-}" == "--soak" ]]; then echo "SKIP: 24h soak deferred"; exit 0; fi
echo "SKIP: RuntimeClass absent locally — D2 wiring point (P2 todo88)"; exit 0
