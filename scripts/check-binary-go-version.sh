#!/usr/bin/env bash
# goreleaser post-build hook: a binary built by any Go other than the go.mod
# directive never reaches the registry.
set -euo pipefail
cd "$(dirname "$0")/.."

bin=${1:?usage: check-binary-go-version.sh <binary>}
want="go$(awk '$1 == "go" { print $2; exit }' go.mod)"
got=$(go version -m "$bin" | awk 'NR == 1 { print $2 }')

if [ "$got" != "$want" ]; then
  echo "$bin was built with $got, go.mod says $want" >&2
  exit 1
fi
echo "$bin built with $got"
