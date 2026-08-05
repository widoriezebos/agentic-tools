#!/usr/bin/env bash
set -euo pipefail

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

config=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/metasystem-config.sh
age_args=(get --key refactor.max-age-minutes --default 1440)
commit_args=(get --key refactor.max-commits --default 40)
(( max_age_minutes_set )) && age_args+=(--flag "$max_age_minutes")
(( max_commits_set )) && commit_args+=(--flag "$max_commits")
max_age_minutes=$("$config" "${age_args[@]}")
max_commits=$("$config" "${commit_args[@]}")

git rev-parse --is-inside-work-tree >/dev/null 2>&1 || { echo "not inside a git repository" >&2; exit 2; }

toplevel=$(git rev-parse --show-toplevel)
file_dir=$(dirname "$file")
mkdir -p "$file_dir"
abs_file="$(cd "$file_dir" && pwd -P)/$(basename "$file")"
[[ "$abs_file" == "$toplevel"/* ]] || { echo "baseline file must live inside the repository so git can see its dirt: $file" >&2; exit 2; }
rel_file=${abs_file#"$toplevel"/}

worktree_dirty() { [[ -n "$(git status --porcelain)" ]]; }

worktree_dirty_beyond_baseline() {
  # -z porcelain is NUL-delimited and never C-quoted, so paths with spaces
  # or non-ASCII bytes compare literally. A rename's second record carries no
  # status prefix and therefore reads as foreign dirt, which is the safe
  # direction: it blocks.
  git -C "$toplevel" status --porcelain --untracked-files=all -z | tr '\0' '\n' \
    | awk -v rel="$rel_file" 'substr($0, 4) != rel { found = 1 } END { exit found ? 0 : 1 }'
}

case "$cmd" in
  record)
    [[ -n "$gate" ]] || { echo "record requires --gate with the acceptance gate command that passed" >&2; exit 2; }
    if worktree_dirty; then
      echo "worktree is dirty; a baseline must be an exact committed state" >&2
      exit 1
    fi
    sha=$(git rev-parse HEAD)
    now=$(date -u +%s)
    {
      echo "sha=$sha"
      echo "recorded_epoch=$now"
      echo "gate=$gate"
    } >"$file"
    echo "trusted baseline recorded: $sha"
    echo "commit $file now or with the next checkpoint; check ignores this file's own dirt"
    ;;
  check)
    [[ -f "$file" ]] || { echo "no trusted baseline at $file; run the acceptance gate, then record it" >&2; exit 1; }
    sha=$(sed -n 's/^sha=//p' "$file" | head -1)
    recorded_epoch=$(sed -n 's/^recorded_epoch=//p' "$file" | head -1)
    [[ -n "$sha" && "$recorded_epoch" =~ ^[0-9]+$ ]] || { echo "baseline file is malformed: $file" >&2; exit 2; }
    git rev-parse --verify --quiet "${sha}^{commit}" >/dev/null || { echo "baseline commit $sha is unknown to this repository" >&2; exit 1; }
    if worktree_dirty_beyond_baseline; then
      echo "worktree is dirty beyond the baseline file; commit, stash, or revert before a new refactor batch" >&2
      exit 1
    fi
    git merge-base --is-ancestor "$sha" HEAD || { echo "baseline $sha is not an ancestor of HEAD; history diverged from the trusted baseline" >&2; exit 1; }
    commits_since=$(git rev-list --count "${sha}..HEAD")
    age_minutes=$(( ( $(date -u +%s) - recorded_epoch ) / 60 ))
    if (( commits_since > max_commits )); then
      echo "cadence backstop due: $commits_since commits since the trusted baseline (max $max_commits); run the acceptance gate" >&2
      exit 1
    fi
    if (( age_minutes > max_age_minutes )); then
      echo "cadence backstop due: baseline is $age_minutes minutes old (max $max_age_minutes); run the acceptance gate" >&2
      exit 1
    fi
    echo "refactor baseline safe: $sha ($commits_since commits, $age_minutes minutes since acceptance)"
    ;;
  *)
    usage
    exit 2
    ;;
esac
