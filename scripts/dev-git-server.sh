#!/usr/bin/env bash
# P2-A: serve every e2e fixture repo to kind nodes via git daemon.
# Fixtures are copied to a temp workdir so the tracked tree stays pristine.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
EXPORT_DIR="${GIT_EXPORT_DIR:-/tmp/tenara-git}"
PORT="${GIT_DAEMON_PORT:-9418}"

mkdir -p "$EXPORT_DIR"
for src in "$ROOT"/e2e/fixtures/repos/*; do
  name=$(basename "$src")
  work=$(mktemp -d)
  cp -R "$src"/. "$work"/
  bare="$EXPORT_DIR/$name.git"
  rm -rf "$bare"
  git init -q --bare "$bare"
  git -C "$work" init -q
  git -C "$work" add -A
  git -C "$work" -c user.email=qa@tenara.local -c user.name=qa commit -qm "seed $name" || true
  git -C "$work" push -q "$bare" HEAD:main 2>/dev/null || true
  rm -rf "$work"
  echo "exported: $name.git"
done

pkill -f "git-daemon $PORT" 2>/dev/null || true
git daemon --base-path="$EXPORT_DIR" --export-all --reuseaddr --port="$PORT" \
  > /tmp/tenara-git-daemon.log 2>&1 &
disown
GW=$(docker network inspect kind --format '{{(index .IPAM.Config 0).Gateway}}' 2>/dev/null || echo '<gateway>')
echo "git daemon ready on :$PORT"
echo "kind nodes use: git://$GW:$PORT/<fixture>.git"
