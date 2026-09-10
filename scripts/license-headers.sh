#!/usr/bin/env bash
# Inserts the canonical license header into hand-written Go files that lack one
# and normalizes existing headers. The year is the file's creation year: an
# existing year is kept, a new header gets the current year. Generated files
# are owned by their generators and skipped.
set -euo pipefail
cd "$(dirname "$0")/.."

YEAR=$(date +%Y)
read -r -d '' HEADER <<'H' || true
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.
H

changed=0
while IFS= read -r f; do
  tmp=$(mktemp)
  awk -v year="$YEAR" -v header="${HEADER//$'\n'/\\n}" '
    BEGIN { state = "prefix" }
    state == "prefix" {
      if ($0 ~ /^\/\/(go:build|[[:space:]]*\+build)/ || ($0 == "" && seenTag)) { seenTag = 1; print; next }
      state = "head"
    }
    state == "head" {
      if (match($0, /^\/\/ Copyright [0-9][0-9][0-9][0-9] Redpanda Data, Inc\./)) {
        year = substr($0, 14, 4); state = "skip"; next
      }
      print "// Copyright " year " Redpanda Data, Inc."; print "//"; print header; print ""
      state = "body"
    }
    state == "skip" {
      if ($0 ~ /limitations under the License\./) { state = "skipblank" }
      next
    }
    state == "skipblank" {
      if ($0 == "") next
      print "// Copyright " year " Redpanda Data, Inc."; print "//"; print header; print ""
      state = "body"
    }
    { print }
  ' "$f" > "$tmp"
  if ! cmp -s "$f" "$tmp"; then cp "$tmp" "$f"; changed=$((changed+1)); echo "updated $f"; fi
  rm -f "$tmp"
done < <(find main.go redpanda internal cmd scripts -name '*.go' ! -name '*_gen.go' ! -path '*/mocks/*' ! -path '*/testdata/*')
echo "license-headers: $changed file(s) updated"
