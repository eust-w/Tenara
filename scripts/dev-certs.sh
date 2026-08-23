#!/usr/bin/env bash
# Issue a wildcard dev certificate for *.127.0.0.1.nip.io via mkcert and load it
# into the cluster as secret tenara-wildcard-tls (ns tenara-system). Idempotent.
set -euo pipefail

CERT_DIR="${CERT_DIR:-$(mktemp -d)}"
CAROOT="$(mkcert -CAROOT)" export CAROOT

mkcert -install >/dev/null 2>&1 || echo "warn: mkcert -install failed (system trust not updated; use --cacert with mkcert rootCA)"
mkcert -cert-file "$CERT_DIR/tls.crt" -key-file "$CERT_DIR/tls.key" "*.127.0.0.1.nip.io"

kubectl create namespace tenara-system --dry-run=client -o yaml | kubectl apply -f -
kubectl create secret tls tenara-wildcard-tls \
  --namespace tenara-system \
  --cert "$CERT_DIR/tls.crt" \
  --key "$CERT_DIR/tls.key" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "tls secret tenara-wildcard-tls ready"
