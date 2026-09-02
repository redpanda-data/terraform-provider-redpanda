#!/usr/bin/env bash
# Fails on Go comments that carry process context instead of code context.
# --report additionally lists comment blocks of REPORT_MIN_LINES or more.
set -euo pipefail
cd "$(dirname "$0")/.."

TREES=(main.go redpanda internal cmd scripts)
FIND=(find "${TREES[@]}" -name '*.go' ! -name '*_gen.go' ! -path '*/mocks/*' ! -path '*/testdata/*')

PATTERN='\b(ENG|K8S|CORE|DEVEX|OPS)-[0-9]+\b'
PATTERN+='|\bPR ?#?[0-9]+\b|\bpull request\b|\bthis (PR|commit|branch|session|conversation)\b'
PATTERN+='|\bper (discussion|review|feedback|the (thread|call|sync|chat|reviewer))\b|\bas (discussed|agreed)\b'
PATTERN+='|[Cc]laude|\bAI[: ]|\bagent:'

if [ "${1:-}" = "--report" ]; then
  min="${REPORT_MIN_LINES:-12}"
  "${FIND[@]}" | xargs awk -v min="$min" '
    FNR==1 { n=0; lic=0 }
    /^[[:space:]]*\/\// { n++; if ($0 ~ /Copyright|limitations under the License/) lic=1; next }
    /`\/\/ Copyright/ { n=0; lic=1; next }
    { if (n >= min && !lic) print n"\t"FILENAME":"FNR-n; n=0; lic=0 }
    ENDFILE { if (n >= min && !lic) print n"\t"FILENAME":"FNR-n+1 }' | sort -rn
  exit 0
fi

hits=$("${FIND[@]}" | xargs grep -nE '^\s*//' | grep -E "$PATTERN" || true)
if [ -n "$hits" ]; then
  echo "lint-comments: process context in Go comments (ticket/PR refs, review chatter, session context):"
  echo "$hits"
  exit 1
fi
echo "lint-comments: clean"
