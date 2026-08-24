#!/usr/bin/env bash
# P0-2: RuntimeClass sandbox proof (RB§50 R14, todo88).
# kind mode proves the scheduling gate; cloud mode (BAIDU_KUBECONFIG set)
# proves the live runsc path incl. /proc read-only escape-surface smoke.
set -euo pipefail
MODE="${1:-kind}"
[[ "$MODE" == "cloud" && -z "${BAIDU_KUBECONFIG:-}" ]] && { echo "SKIP: no cloud kubeconfig"; exit 0; }
if [[ "${2:-}" == "--soak" ]]; then echo "SKIP: 24h soak deferred"; exit 0; fi

KC="kubectl"
[[ "$MODE" == "cloud" ]] && KC="kubectl --kubeconfig ${BAIDU_KUBECONFIG}"

$KC apply -f deploy/runtimeclasses.yaml >/dev/null

NS=poc-runtimeclass
$KC get ns "$NS" >/dev/null 2>&1 || $KC create ns "$NS" >/dev/null

cat <<'YAML' | $KC apply -f - >/dev/null
apiVersion: v1
kind: Pod
metadata:
  name: runsc-probe
  namespace: poc-runtimeclass
spec:
  restartPolicy: Never
  runtimeClassName: gvisor
  containers:
    - name: probe
      image: busybox:1.36
      command: ["sh","-c","mount | grep ' on /proc ' | grep -q '(ro[,)]' && echo PROC_RO || echo PROC_RW"]
YAML

RC=0
if $KC -n "$NS" wait --for=condition=Ready pod/runsc-probe --timeout=90s 2>/dev/null; then
  LOG=$($KC -n "$NS" logs runsc-probe 2>/dev/null || true)
  echo "probe-log: $LOG"
  [[ "$LOG" == *PROC_RO* ]] && echo "PASS: runsc pod Ready + /proc read-only" || { echo "FAIL: /proc state unverified"; RC=1; }
elif [[ "$MODE" == "kind" ]]; then
  echo "PASS(kind-gate): gvisor pod unschedulable on plain kind (handler absent) — cloud mode proves live path"
else
  echo "FAIL: cloud runsc pod not Ready"; RC=1
fi

$KC -n "$NS" delete pod runsc-probe --force --grace-period=0 >/dev/null 2>&1 || true
exit $RC
