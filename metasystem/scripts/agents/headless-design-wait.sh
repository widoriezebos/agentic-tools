#!/bin/bash
set -u
usage() { echo "usage: $0 <tag> <page> --out <dir> [--poll <seconds>] [--projects <dir>]" >&2; exit 2; }
[ $# -ge 4 ] || usage
tag=$1 page=$2; shift 2
out= poll=30 projects=$HOME/.claude/projects
while [ $# -gt 0 ]; do
  case $1 in --out|--poll|--projects) [ $# -ge 2 ] || usage; key=$1; value=$2; shift 2;; *) usage;; esac
  case $key in --out) out=$value;; --poll) poll=$value;; --projects) projects=$value;; esac
done
[ -n "$out" ] || usage
pid=$(cat "$out/$tag-design.pid") || exit 2
while kill -0 "$pid" 2>/dev/null; do sleep "$poll"; done
json=$out/$tag-design.json
sid=$(jq -r .session_id "$json" 2>/dev/null)
echo "design-$tag exited $(date -u +%H:%M:%SZ) error=$(jq -r .is_error "$json" 2>/dev/null) turns=$(jq -r .num_turns "$json" 2>/dev/null)"
jq -r .result "$json" 2>/dev/null | tail -2
compactions=$(find "$projects" -type f -name "$sid.jsonl" -exec jq -c 'select(.type=="system" and .subtype=="compact_boundary")' {} + 2>/dev/null | wc -l | tr -d ' ')
echo "real compactions: $compactions"
wc -lw "$page" 2>&1 | head -1
