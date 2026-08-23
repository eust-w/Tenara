#!/usr/bin/env bash
# Runtime boundary assertion suite A (RB§44 §15 §16 R10).
# Prereq: kind cluster; at least one fixture app deployed.
# Usage: ./verify-runtime.sh <app_id> <namespace>
set -euo pipefail

APP_ID="${1:?usage: $0 <app_id> <namespace>}"
NS="${2:?usage: $0 <app_id> <namespace>}"
PASS=0; FAIL=0

ok()   { PASS=$((PASS+1)); echo "  ok   $1"; }
fail() { FAIL=$((FAIL+1)); echo "  FAIL $1"; }

check() { if eval "$2"; then ok "$1"; else fail "$1"; fi }

echo "== Runtime boundary assertions for $APP_ID in $NS =="

# 1. Namespace exists with correct labels
check "ns exists" \
  'kubectl get ns "'$NS'" --no-headers | grep -qv "NotFound"'
check "ns label tenara.io/app-id" \
  'kubectl get ns "'$NS'" -o json | jq -e ".metadata.labels[\"tenara.io/app-id\"] != null"'
check "ns label managed-by=tenara" \
  'kubectl get ns "'$NS'" -o json | jq -e ".metadata.labels[\"tenara.io/managed-by\"] == \"tenara\""'

# 2. ServiceAccount exists without automount
check "SA tenant exists" \
  'kubectl get sa tenant -n "'$NS'" --no-headers | grep -v NotFound'
check "SA automountServiceAccountToken=false" \
  'kubectl get sa tenant -n "'$NS'" -o json | jq -e ".automountServiceAccountToken == false"'

# 3. Tenant pods have automountServiceAccountToken=false (pod level)
check "pods automountServiceAccountToken=false" \
  'kubectl get pods -n "'$NS'" -o json | jq -e "[.items[].spec.automountServiceAccountToken] | all(. == false)"'

# 4. SecurityContext hardened per RB§15
check "containers runAsNonRoot" \
  'kubectl get pods -n "'$NS'" -o json | jq -e "[.items[].spec.containers[].securityContext.runAsNonRoot] | all(. == true)"'

# 5. NetworkPolicy default-deny exists
check "default-deny ingress netpol exists" \
  'kubectl get netpol deny-all-ingress -n "'$NS'" --no-headers | grep -v NotFound'

# 6. ResourceQuota exists
check "resourcequota exists" \
  'kubectl get resourcequota tenant-free -n "'$NS'" --no-headers | grep -v NotFound'

echo ""
echo "Summary: PASS=$PASS FAIL=$FAIL"
[[ $FAIL -eq 0 ]] && echo "RUNTIME_ASSERTIONS_PASS" || { echo "RUNTIME_ASSERTIONS_FAIL"; exit 1; }
