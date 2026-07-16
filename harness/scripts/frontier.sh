#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/frontier.sh record --score N --eval "eval command that produced it" \
      [--artifact <path>] [--force] [--file <path>]
  scripts/frontier.sh challenge --score N [--min-delta D] [--file <path>]
  scripts/frontier.sh status [--file <path>]

record: store the current clean, committed HEAD as the best-known frontier
for a measured improvement goal. Refuses a score that does not beat the
existing frontier by more than the noise floor unless --force is given
(re-baselining after an evaluation change; record the reason in the owning
plan). Scores and eval commands are declared, not measured here.

challenge: exit 0 only when the candidate score beats the recorded frontier
by more than the noise floor. Run this before treating any run as an
improvement or preserving a new frontier.

The noise floor is persisted in the frontier file at record time and used
as the default for every later challenge and record, so forgetting the flag
cannot silently drop the floor to zero. Resolution order: --min-delta flag,
HARNESS_FRONTIER_MIN_DELTA, the stored value, then 0.

Defaults: --file plans/frontier.
Exit codes: 0 ok / beats frontier; 1 blocked / does not beat; 2 usage error.
USAGE
}

file=plans/frontier
min_delta_flag=
score= eval_cmd= artifact= force=0

cmd=${1:-}
[[ -n "$cmd" ]] || { usage; exit 2; }
shift

while (($#)); do
  case "$1" in
    --file) file=${2:-}; shift 2 ;;
    --score) score=${2:-}; shift 2 ;;
    --eval) eval_cmd=${2:-}; shift 2 ;;
    --artifact) artifact=${2:-}; shift 2 ;;
    --min-delta) min_delta_flag=${2:-}; shift 2 ;;
    --force) force=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) usage; exit 2 ;;
  esac
done

numeric='^-?[0-9]+([.][0-9]+)?$'

read_field() { sed -n "s/^$1=//p" "$file" | head -1; }

resolve_min_delta() { # $1 = stored value, possibly empty
  local d
  if [[ -n "$min_delta_flag" ]]; then d=$min_delta_flag
  elif [[ -n "${HARNESS_FRONTIER_MIN_DELTA:-}" ]]; then d=$HARNESS_FRONTIER_MIN_DELTA
  elif [[ -n "$1" ]]; then d=$1
  else d=0; fi
  [[ "$d" =~ $numeric ]] || { echo "invalid noise floor: $d" >&2; exit 2; }
  printf '%s' "$d"
}

beats() { # beats <candidate> <incumbent> <delta>
  awk -v n="$1" -v o="$2" -v d="$3" 'BEGIN { exit (n > o + d) ? 0 : 1 }'
}

case "$cmd" in
  record)
    [[ "$score" =~ $numeric ]] || { echo "record requires a numeric --score" >&2; exit 2; }
    [[ -n "$eval_cmd" ]] || { echo "record requires --eval with the evaluation command that produced the score" >&2; exit 2; }
    git rev-parse --is-inside-work-tree >/dev/null 2>&1 || { echo "not inside a git repository" >&2; exit 2; }
    [[ -z "$(git status --porcelain)" ]] || { echo "worktree is dirty; a frontier must be an exact committed state" >&2; exit 1; }
    stored_delta=
    if [[ -f "$file" ]]; then
      stored_delta=$(read_field min_delta)
    fi
    min_delta=$(resolve_min_delta "$stored_delta")
    if [[ -f "$file" && $force -eq 0 ]]; then
      old=$(read_field score)
      [[ "$old" =~ $numeric ]] || { echo "frontier file is malformed: $file" >&2; exit 2; }
      beats "$score" "$old" "$min_delta" || {
        echo "score $score does not beat the recorded frontier $old by more than $min_delta; use challenge first, or --force only to re-baseline after an evaluation change" >&2
        exit 1
      }
    fi
    mkdir -p "$(dirname "$file")"
    {
      echo "sha=$(git rev-parse HEAD)"
      echo "recorded_epoch=$(date -u +%s)"
      echo "score=$score"
      echo "min_delta=$min_delta"
      echo "eval=$eval_cmd"
      echo "artifact=$artifact"
    } >"$file"
    echo "frontier recorded: score $score at $(git rev-parse --short HEAD)"
    echo "commit $file with the frontier checkpoint"
    ;;
  challenge)
    [[ "$score" =~ $numeric ]] || { echo "challenge requires a numeric --score" >&2; exit 2; }
    [[ -f "$file" ]] || { echo "no frontier recorded at $file; record the baseline first" >&2; exit 1; }
    old=$(read_field score)
    [[ "$old" =~ $numeric ]] || { echo "frontier file is malformed: $file" >&2; exit 2; }
    min_delta=$(resolve_min_delta "$(read_field min_delta)")
    if beats "$score" "$old" "$min_delta"; then
      echo "new frontier: $score beats $old by more than $min_delta; preserve this exact state before iterating"
      exit 0
    fi
    if beats "$score" "$old" 0; then
      echo "within noise floor: $score is above $old but not by more than $min_delta; treat as noise" >&2
    else
      echo "below frontier: $score does not beat $old" >&2
    fi
    exit 1
    ;;
  status)
    [[ -f "$file" ]] || { echo "no frontier recorded at $file"; exit 0; }
    cat "$file"
    ;;
  *)
    usage
    exit 2
    ;;
esac
