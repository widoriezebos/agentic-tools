#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/frontier.sh record --score N --eval "eval command that produced it" \
      [--direction max|min] [--artifact <path>] [--min-delta D] \
      [--max-age-minutes M] [--force] [--file <path>]
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
METASYSTEM_FRONTIER_MIN_DELTA, the stored value, then 0.

The direction is persisted the same way: max (higher is better, the
default) or min (lower is better, for latency and cost goals). record
resolves --direction, METASYSTEM_FRONTIER_DIRECTION, then the stored value;
changing a stored direction requires --force, because it re-baselines what
the frontier means. challenge accepts no direction input and uses only the
persisted value, so a flag or environment variable cannot reinterpret a
recorded frontier at comparison time.

Dynamic systems can declare a measurement window with --max-age-minutes
(persisted the same way; env METASYSTEM_FRONTIER_MAX_AGE_MINUTES). When the
recorded frontier is older than the window, comparisons are refused:
the environment may have shifted, so re-baseline with record --force and
record the reason in the owning plan. No window means scores never expire.

Defaults: --file plans/frontier.
Exit codes: 0 ok / beats frontier; 1 blocked / does not beat / expired; 2 usage error.
USAGE
}

file=plans/frontier
min_delta_flag= max_age_flag= direction_flag=
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
    --max-age-minutes) max_age_flag=${2:-}; shift 2 ;;
    --direction) direction_flag=${2:-}; shift 2 ;;
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
  elif [[ -n "${METASYSTEM_FRONTIER_MIN_DELTA:-}" ]]; then d=$METASYSTEM_FRONTIER_MIN_DELTA
  elif [[ -n "$1" ]]; then d=$1
  else d=0; fi
  [[ "$d" =~ $numeric ]] || { echo "invalid noise floor: $d" >&2; exit 2; }
  printf '%s' "$d"
}

resolve_window() { # $1 = stored value, possibly empty; empty result means no window
  local w
  if [[ -n "$max_age_flag" ]]; then w=$max_age_flag
  elif [[ -n "${METASYSTEM_FRONTIER_MAX_AGE_MINUTES:-}" ]]; then w=$METASYSTEM_FRONTIER_MAX_AGE_MINUTES
  else w=$1; fi
  [[ -z "$w" || "$w" =~ ^[0-9]+$ ]] || { echo "invalid measurement window: $w" >&2; exit 2; }
  printf '%s' "$w"
}

frontier_expired() { # $1 = window (may be empty), uses recorded_epoch from the file
  local epoch age
  [[ -n "$1" ]] || return 1
  epoch=$(read_field recorded_epoch)
  [[ "$epoch" =~ ^[0-9]+$ ]] || { echo "frontier file is malformed: $file" >&2; exit 2; }
  age=$(( ( $(date -u +%s) - epoch ) / 60 ))
  (( age > $1 ))
}

resolve_direction() { # $1 = stored value, possibly empty; record-time resolution
  local d
  if [[ -n "$direction_flag" ]]; then d=$direction_flag
  elif [[ -n "${METASYSTEM_FRONTIER_DIRECTION:-}" ]]; then d=$METASYSTEM_FRONTIER_DIRECTION
  elif [[ -n "$1" ]]; then d=$1
  else d=max; fi
  [[ "$d" == max || "$d" == min ]] || { echo "invalid direction: $d (max or min)" >&2; exit 2; }
  printf '%s' "$d"
}

beats() { # beats <candidate> <incumbent> <delta> <direction>
  if [[ "$4" == min ]]; then
    awk -v n="$1" -v o="$2" -v d="$3" 'BEGIN { exit (n < o - d) ? 0 : 1 }'
  else
    awk -v n="$1" -v o="$2" -v d="$3" 'BEGIN { exit (n > o + d) ? 0 : 1 }'
  fi
}

case "$cmd" in
  record)
    [[ "$score" =~ $numeric ]] || { echo "record requires a numeric --score" >&2; exit 2; }
    [[ -n "$eval_cmd" ]] || { echo "record requires --eval with the evaluation command that produced the score" >&2; exit 2; }
    git rev-parse --is-inside-work-tree >/dev/null 2>&1 || { echo "not inside a git repository" >&2; exit 2; }
    [[ -z "$(git status --porcelain)" ]] || { echo "worktree is dirty; a frontier must be an exact committed state" >&2; exit 1; }
    stored_delta= stored_window= stored_direction=
    if [[ -f "$file" ]]; then
      stored_delta=$(read_field min_delta)
      stored_window=$(read_field max_age_minutes)
      stored_direction=$(read_field direction)
    fi
    min_delta=$(resolve_min_delta "$stored_delta")
    window=$(resolve_window "$stored_window")
    direction=$(resolve_direction "$stored_direction")
    if [[ -f "$file" && $force -eq 0 ]]; then
      effective_stored=${stored_direction:-max}
      if [[ "$direction" != "$effective_stored" ]]; then
        echo "direction $direction differs from the recorded frontier's $effective_stored; a direction change re-baselines the frontier, so use --force and record the reason in the owning plan" >&2
        exit 1
      fi
      if frontier_expired "$window"; then
        echo "recorded frontier is older than its measurement window; the environment may have shifted. Re-baseline with --force and record the reason in the owning plan" >&2
        exit 1
      fi
      old=$(read_field score)
      [[ "$old" =~ $numeric ]] || { echo "frontier file is malformed: $file" >&2; exit 2; }
      beats "$score" "$old" "$min_delta" "$direction" || {
        echo "score $score does not beat the recorded frontier $old by more than $min_delta (direction $direction); use challenge first, or --force only to re-baseline after an evaluation change" >&2
        exit 1
      }
    fi
    mkdir -p "$(dirname "$file")"
    {
      echo "sha=$(git rev-parse HEAD)"
      echo "recorded_epoch=$(date -u +%s)"
      echo "score=$score"
      echo "min_delta=$min_delta"
      echo "direction=$direction"
      echo "max_age_minutes=$window"
      echo "eval=$eval_cmd"
      echo "artifact=$artifact"
    } >"$file"
    echo "frontier recorded: score $score at $(git rev-parse --short HEAD)"
    echo "commit $file with the frontier checkpoint"
    ;;
  challenge)
    [[ "$score" =~ $numeric ]] || { echo "challenge requires a numeric --score" >&2; exit 2; }
    [[ -z "$direction_flag" ]] || { echo "challenge uses only the persisted direction; change it with record --force, never at comparison time" >&2; exit 2; }
    [[ -f "$file" ]] || { echo "no frontier recorded at $file; record the baseline first" >&2; exit 1; }
    old=$(read_field score)
    [[ "$old" =~ $numeric ]] || { echo "frontier file is malformed: $file" >&2; exit 2; }
    window=$(resolve_window "$(read_field max_age_minutes)")
    if frontier_expired "$window"; then
      echo "frontier expired: recorded score $old is outside its measurement window; the environment may have shifted. Re-baseline with record --force before comparing candidates" >&2
      exit 1
    fi
    min_delta=$(resolve_min_delta "$(read_field min_delta)")
    if grep -q '^direction=' "$file"; then
      direction=$(read_field direction)
      [[ "$direction" == max || "$direction" == min ]] \
        || { echo "frontier file has a malformed direction: '$direction'" >&2; exit 2; }
    else
      direction=max
    fi
    if beats "$score" "$old" "$min_delta" "$direction"; then
      echo "new frontier: $score beats $old by more than $min_delta (direction $direction); preserve this exact state before iterating"
      exit 0
    fi
    if beats "$score" "$old" 0 "$direction"; then
      echo "within noise floor: $score improves on $old but not by more than $min_delta; treat as noise" >&2
    else
      echo "does not beat frontier: $score against $old (direction $direction)" >&2
    fi
    exit 1
    ;;
  status)
    [[ -f "$file" ]] || { echo "no frontier recorded at $file"; exit 0; }
    cat "$file"
    grep -q '^direction=.' "$file" || echo "direction=max"
    ;;
  *)
    usage
    exit 2
    ;;
esac
