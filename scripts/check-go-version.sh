#!/usr/bin/env bash
# GOTOOLCHAIN=auto makes the go.mod directive a floor, but a pin left behind
# still builds, lints, and releases on an older Go until someone notices.
set -euo pipefail
cd "$(dirname "$0")/.."

want=$(awk '$1 == "go" { print $2; exit }' go.mod)
fail=0

check() {
  if [ "$2" != "$want" ]; then
    echo "go toolchain drift: $1 is '$2', go.mod says '$want'" >&2
    fail=1
  fi
}

check "Taskfile.yml GO_VERSION" "$(awk '$1 == "GO_VERSION:" { print $2; exit }' Taskfile.yml)"

# A variant suffix such as -alpine is not part of the pin.
images=$(grep -oE 'golang:[0-9][^"[:space:]]*' .buildkite/pipeline.yml | sed 's/-.*//' || true)
if [ -z "$images" ]; then
  echo "no golang image found in .buildkite/pipeline.yml" >&2
  fail=1
fi
n=0
while IFS= read -r img; do
  n=$((n + 1))
  check ".buildkite/pipeline.yml image #$n" "${img#golang:}"
done <<<"$images"

check "tools/Dockerfile FROM" "$(sed -nE 's/^FROM golang:([0-9.]+).*/\1/p' tools/Dockerfile)"

for mod in scripts/cleanup-redpanda-byovpc scripts/cleanup-gcp-redpanda-byovpc; do
  check "$mod/go.mod" "$(awk '$1 == "go" { print $2; exit }' "$mod/go.mod")"
done

if [ "$fail" = 0 ]; then
  echo "go toolchain pins agree on $want"
fi
exit "$fail"
