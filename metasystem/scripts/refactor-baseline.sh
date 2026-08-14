#!/usr/bin/env bash
set -euo pipefail

# Phase A shim (script-misc-2/D29): flag parsing and config plumbing stay
# here, every decision lives in `validate refactor-baseline`.

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/refactor-baseline.sh record --gate "gate command that passed" [--file <path>]
  scripts/refactor-baseline.sh check [--file <path>] [--max-age-minutes N] [--max-commits N]

record: store the current clean, committed HEAD as the trusted refactor
baseline after the project's acceptance gate passed. The gate command is
declared, not verified; recording without a passing gate creates a false
baseline. Commit the baseline file with the next checkpoint.

check: allow a new refactor edit batch only when the worktree is clean, the
trusted baseline is an ancestor of HEAD, and the cadence backstop (age and
commit count since the baseline) is not exceeded. Dirt consisting solely of
the baseline file itself is tolerated, so running check right after record
never blocks; commit the file with the next checkpoint. The --file path is
normalized against the repository root and must live inside the repository,
because git cannot see dirt outside it.

Defaults: --file plans/refactor-baseline. Cadence resolves from flags, then
environment, then metasystem.conf, then built-ins: --max-age-minutes 1440 and
--max-commits 40.

Exit codes: 0 safe; 1 blocked; 2 usage or environment error.
USAGE
}

file=plans/refactor-baseline
max_age_minutes=
max_commits=
max_age_minutes_set=0
max_commits_set=0
gate=

cmd=${1:-}
[[ -n "$cmd" ]] || { usage; exit 2; }
shift

while (($#)); do
  case "$1" in
    --file) file=${2:-}; shift 2 ;;
    --gate) gate=${2:-}; shift 2 ;;
    --max-age-minutes) max_age_minutes=${2:-}; max_age_minutes_set=1; shift 2 ;;
    --max-commits) max_commits=${2:-}; max_commits_set=1; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) usage; exit 2 ;;
  esac
done

case "$cmd" in
  record|check) ;;
  *) usage; exit 2 ;;
esac

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"
config="$root/scripts/metasystem-config.sh"
age_args=(get --key refactor.max-age-minutes --default 1440)
commit_args=(get --key refactor.max-commits --default 40)
(( max_age_minutes_set )) && age_args+=(--flag "$max_age_minutes")
(( max_commits_set )) && commit_args+=(--flag "$max_commits")
max_age_minutes=$("$config" "${age_args[@]}")
max_commits=$("$config" "${commit_args[@]}")

exec "$ms" validate refactor-baseline --command "$cmd" --file "$file" \
  --gate "$gate" --max-age-minutes "$max_age_minutes" --max-commits "$max_commits"
