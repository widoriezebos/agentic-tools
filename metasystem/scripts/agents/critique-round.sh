#!/usr/bin/env bash
# The executable covenant's critique half: one driver any agent can
# invoke to run a review round and keep its evidence. Rounds are
# repo-adjacent artifacts, not one session's scratchpad: the input
# packet, the verdict, and stderr all land under artifacts/ named by
# chain and round, so the NEXT agent (or machine) can read the whole
# chain's history.
#
# Usage: critique-round.sh <chain-name> <round> <input-file>
#        [--model gpt-5.6-sol] [--effort xhigh]
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
chain=${1:?chain name}; round=${2:?round number}; input=${3:?input packet}
shift 3
model=gpt-5.6-sol
effort=xhigh
while (($#)); do
  case "$1" in
    --model) model=${2:?}; shift 2 ;;
    --effort) effort=${2:?}; shift 2 ;;
    *) echo "critique-round: unknown flag $1" >&2; exit 2 ;;
  esac
done
[[ -f "$input" ]] || { echo "critique-round: no input packet at $input" >&2; exit 2; }
command -v codex >/dev/null || { echo "critique-round: codex is not installed" >&2; exit 1; }

dir="$root/artifacts/agents/critiques/$chain"
mkdir -p "$dir"
cp "$input" "$dir/r$round-input.md"
out="$dir/r$round-output.md"
errlog="$dir/r$round-stderr.log"

codex exec --model "$model" -c model_reasoning_effort="\"$effort\"" \
  --sandbox read-only --skip-git-repo-check -o "$out" - <"$input" 2>"$errlog"

echo "round archived: $out"
head -1 "$out"
