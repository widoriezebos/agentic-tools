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
#        [--role design-critic|code-critic]
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
if (( $# < 3 )); then
  echo "usage: critique-round.sh <chain-name> <round> <input-file> [--model M] [--effort E] [--role design-critic|code-critic]" >&2
  echo "runs one review round and archives packet, verdict, and stderr under artifacts/agents/critiques/<chain>/" >&2
  [[ ${1:-} == --help || ${1:-} == -h ]] && exit 0
  exit 2
fi
chain=$1; round=$2; input=$3
shift 3
model=gpt-5.6-sol
effort=xhigh
role=design-critic
while (($#)); do
  case "$1" in
    --model) model=${2:?}; shift 2 ;;
    --effort) effort=${2:?}; shift 2 ;;
    --role) role=${2:?}; shift 2 ;;
    *) echo "critique-round: unknown flag $1" >&2; exit 2 ;;
  esac
done
[[ -f "$input" ]] || { echo "critique-round: no input packet at $input" >&2; exit 2; }
[[ "$chain" =~ ^[a-z0-9][a-z0-9-]*$ ]] || { echo "critique-round: invalid chain name $chain" >&2; exit 2; }
[[ "$round" =~ ^[1-9][0-9]*$ ]] || { echo "critique-round: round must be a positive integer" >&2; exit 2; }
[[ "$role" == design-critic || "$role" == code-critic ]] \
  || { echo "critique-round: role must be design-critic or code-critic" >&2; exit 2; }

dir="$root/artifacts/agents/critiques/$chain"
mkdir -p "$dir"
cp "$input" "$dir/r$round-input.md"
out="$dir/r$round-output.md"
errlog="$dir/r$round-stderr.log"
runtime=codex
if [[ "${METASYSTEM_CRITIQUE_FIXTURE_RUNTIME:-}" == fake ]]; then runtime=fake; fi
ms=${METASYSTEM_BIN:-$root/bin/metasystem}
job="critique-$(date -u +%Y%m%dt%H%M%Sz)-$$-$("$ms" util token-hex --bytes 3)"
session="critique:$chain"

round_rc=0
"$root/scripts/agents/dispatch.sh" custodial-exec --job "$job" --session "$session" \
  --role "$role" --input "$input" --runtime "$runtime" --model "$model" \
  --effort "$effort" 2>"$errlog" || round_rc=$?
if [[ -f "$root/artifacts/agents/jobs/$job.log" ]]; then
  sed '1d' "$root/artifacts/agents/jobs/$job.log" >>"$errlog"
fi
raw="$root/artifacts/agents/$job/rounds/1/raw.out"
if [[ -f "$raw" ]]; then cp "$raw" "$out"; else : >"$out"; fi
(( round_rc == 0 )) || exit "$round_rc"

echo "round archived: $out"
head -1 "$out"
