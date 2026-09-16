#!/bin/bash
set -u
usage() { echo "usage: $0 <tag> <prompt-file> --out <dir> [--root <dir>] [--model <model>] [--resume <session>]" >&2; exit 2; }
[ $# -ge 4 ] || usage
tag=$1; prompt=$2; shift 2
metasystem=$(cd "$(dirname "$0")/../.." && pwd -P); out= root=$(cd "$metasystem/.." && pwd -P) model=claude-fable-5-1 resume=
while [ $# -gt 0 ]; do
  case $1 in
    --out|--root|--model|--resume) [ $# -ge 2 ] || usage; key=$1; value=$2; shift 2 ;;
    *) usage ;;
  esac
  case $key in --out) out=$value;; --root) root=$value;; --model) model=$value;; --resume) resume=$value;; esac
done
[ -n "$tag" ] && [ -n "$out" ] || usage
[ -s "$prompt" ] || { echo "prompt file $prompt is empty or missing" >&2; exit 2; }
prompt=$(cd "$(dirname "$prompt")" && pwd -P)/$(basename "$prompt")
mkdir -p "$out" || exit 2; out=$(cd "$out" && pwd -P) || exit 2
root=$(cd "$root" && pwd -P) || exit 2; cd "$root" || exit 2
window=${CLAUDE_CODE_AUTO_COMPACT_WINDOW:-1000000}
if [ -n "$resume" ]; then
  CLAUDE_CODE_AUTO_COMPACT_WINDOW=$window nohup claude -p --model "$model" --dangerously-skip-permissions --output-format json --name "design-$tag" --resume "$resume" <"$prompt" >"$out/$tag-design.json" 2>"$out/$tag-design.err" &
else
  CLAUDE_CODE_AUTO_COMPACT_WINDOW=$window nohup claude -p --model "$model" --dangerously-skip-permissions --output-format json --name "design-$tag" <"$prompt" >"$out/$tag-design.json" 2>"$out/$tag-design.err" &
fi
echo $! >"$out/$tag-design.pid"
echo "design-$tag launched pid $(cat "$out/$tag-design.pid") at $(date -u +%H:%M:%SZ); result $out/$tag-design.json"
