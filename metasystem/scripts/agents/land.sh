#!/usr/bin/env bash
# Each named command owns its output file and its captured status. Errexit is
# deliberately absent because the driver, not an implicit shell branch, must
# decide whether a push rejection is retryable and which exit code reaches the
# caller.
set -uo pipefail

usage() {
  echo "Usage: scripts/agents/land.sh -m <message-file-or-heredoc> [--staged-only | <pathspec>...] [--ratchet <path>] [--allow-new-plan] [--skip-transport]" >&2
}

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P) || exit $?
cd "$root" || exit $?

message_source=
staged_only=0
allow_new_plan=0
skip_transport=0
ratchet=
pathspecs=()

while (( $# )); do
  case "$1" in
    -m)
      [[ $# -ge 2 && -z "$message_source" ]] || { usage; exit 2; }
      message_source=$2
      shift 2
      ;;
    --staged-only)
      staged_only=1
      shift
      ;;
    --allow-new-plan)
      allow_new_plan=1
      shift
      ;;
    --skip-transport)
      skip_transport=1
      shift
      ;;
    --ratchet)
      [[ $# -ge 2 && -z "$ratchet" ]] || { usage; exit 2; }
      ratchet=$2
      shift 2
      ;;
    --)
      shift
      while (( $# )); do
        pathspecs+=("$1")
        shift
      done
      ;;
    -*)
      echo "land refused: unknown option: $1" >&2
      usage
      exit 2
      ;;
    *)
      pathspecs+=("$1")
      shift
      ;;
  esac
done

[[ -n "$message_source" ]] || { usage; exit 2; }
if (( staged_only && ${#pathspecs[@]} > 0 )); then
  echo "land refused: --staged-only cannot be combined with pathspecs" >&2
  exit 2
fi
if (( ! staged_only && ${#pathspecs[@]} == 0 )); then
  echo "land refused: name pathspecs or choose --staged-only" >&2
  exit 2
fi

message_file=$message_source
owned_message=
if [[ "$message_source" == - ]]; then
  owned_message=$(mktemp "${TMPDIR:-/tmp}/metasystem-land-message.XXXXXX") || exit $?
  cat >"$owned_message" || { rc=$?; rm -f -- "$owned_message"; exit "$rc"; }
  message_file=$owned_message
elif [[ ! -f "$message_source" || ! -r "$message_source" ]]; then
  echo "land refused: commit message file is not readable: $message_source" >&2
  exit 2
fi

step_output=$(mktemp "${TMPDIR:-/tmp}/metasystem-land-step.XXXXXX") || {
  rc=$?
  [[ -z "$owned_message" ]] || rm -f -- "$owned_message"
  exit "$rc"
}
cleanup() {
  rm -f -- "$step_output"
  [[ -z "$owned_message" ]] || rm -f -- "$owned_message"
}
trap cleanup EXIT

step_name=
run_step() { # name, command...
  step_name=$1
  shift
  printf '== STEP: %s\n' "$step_name"
  : >"$step_output"
  "$@" >"$step_output" 2>&1
  step_rc=$?
  if (( step_rc == 0 )); then
    echo "-- ok"
    return 0
  fi
  return "$step_rc"
}

fail_step() { # exit code
  local rc=$1
  printf '!! STEP FAILED: %s (exit %s)\n' "$step_name" "$rc" >&2
  if [[ "$step_name" == "coverage delta for staged Go packages" ]]; then
    # Every failing package must reach the caller in one refusal.
    cat "$step_output" >&2
  else
    tail -n 40 "$step_output" >&2
  fi
  exit "$rc"
}

run_required_step() { # name, command...
  run_step "$@"
  local rc=$?
  (( rc == 0 )) || fail_step "$rc"
}

branch=
check_rulings_id_mints() {
  local changed_paths path rulings_diff line
  local added_id removed_id paired already_offending
  local -a removed_bare_ids=()
  local -a offending_bare_ids=()

  changed_paths=$(git diff --cached --name-only --) || return $?
  while IFS= read -r path; do
    case "$path" in
      metasystem/memory/rulings.md|memory/rulings.md)
        ;;
      *) continue ;;
    esac

    rulings_diff=$(git diff --cached --no-ext-diff --no-textconv -- "$path") || return $?
    removed_bare_ids=()
    while IFS= read -r line; do
      if [[ "$line" =~ ^-\|\ (R-[0-9]+[a-z]?)\ \| ]]; then
        removed_bare_ids+=("${BASH_REMATCH[1]}")
      fi
    done <<<"$rulings_diff"

    while IFS= read -r line; do
      [[ "$line" =~ ^\+\|\ (R-[0-9]+[a-z]?)\ \| ]] || continue
      added_id=${BASH_REMATCH[1]}
      paired=0
      for removed_id in "${removed_bare_ids[@]}"; do
        if [[ "$added_id" == "$removed_id" ]]; then
          paired=1
          break
        fi
      done
      (( paired )) && continue

      already_offending=0
      for removed_id in "${offending_bare_ids[@]}"; do
        if [[ "$added_id" == "$removed_id" ]]; then
          already_offending=1
          break
        fi
      done
      (( already_offending )) || offending_bare_ids+=("$added_id")
    done <<<"$rulings_diff"
  done <<<"$changed_paths"

  if (( ${#offending_bare_ids[@]} > 0 )); then
    echo "land refused: new rulings ids must be machine-suffixed (R-<n>-<machine>); see the register header (a rewritten historical row must keep its id; only new mints need the suffix)" >&2
    printf '  %s\n' "${offending_bare_ids[@]}" >&2
    return 2
  fi
}

verify_checks() {
  branch=$(git symbolic-ref --quiet --short HEAD) || {
    echo "land refused: HEAD is not on a branch" >&2
    return 2
  }
  # Register-id minting law (register-id-minting): a NEW rulings entry
  # must carry a machine-suffixed id (R-<n>-<machine>). Two machines
  # minting the bare same number in one hour is how R-15 and R-20
  # collided; the suffix makes collision impossible by construction. A
  # historical bare id is not minted again when its row is rewritten.
  check_rulings_id_mints || return $?
  if (( staged_only )); then
    git diff --cached --check --
    return $?
  fi
  if ! git diff --cached --quiet --; then
    echo "land refused: pathspec mode requires an empty index; use --staged-only for an existing staged set" >&2
    return 2
  fi
  git diff --check -- "${pathspecs[@]}"
}

stage_changes() {
  local untracked
  if (( ! staged_only )); then
    git add -- "${pathspecs[@]}" || return $?
  fi
  if git diff --cached --quiet --; then
    echo "land refused: the caller-selected staging set is empty" >&2
    return 2
  fi
  if ! git diff --quiet --; then
    echo "land refused: unstaged changes remain after staging; rebase requires a clean tree after commit" >&2
    return 2
  fi
  untracked=$(git ls-files --others --exclude-standard) || return $?
  if [[ -n "$untracked" ]]; then
    echo "land refused: untracked paths remain after staging; rebase requires a clean tree after commit" >&2
    printf '  %s\n' "$untracked" >&2
    return 2
  fi
}

commit_changes() {
  local arguments=(-F "$message_file")
  if [[ -n "$ratchet" ]]; then
    arguments=(--ratchet "$ratchet" "${arguments[@]}")
  fi
  bash "$root/scripts/agents/commit.sh" "${arguments[@]}"
}

require_clean_after_commit() {
  local status
  status=$(git status --porcelain --untracked-files=normal) || return $?
  if [[ -n "$status" ]]; then
    echo "land refused: commit succeeded but the tree is not clean, so rebase will not start" >&2
    printf '%s\n' "$status" >&2
    return 1
  fi
}

fetch_origin() {
  git fetch --quiet origin "+refs/heads/$branch:refs/remotes/origin/$branch"
}

rebase_origin() {
  git rebase "refs/remotes/origin/$branch"
}

push_origin() {
  LC_ALL=C git push --porcelain origin "refs/heads/$branch:refs/heads/$branch"
}

push_was_moving_origin_rejection() {
  LC_ALL=C grep -Eq '\[rejected\].*\((non-fast-forward|fetch first)\)|non-fast-forward|fetch first|cannot lock ref .*is at .*but expected' "$step_output"
}

# Only the flag grants the hook acknowledgment. An inherited shell setting is
# not evidence that this landing's caller chose to include a new plan.
unset METASYSTEM_ALLOW_NEW_PLAN
if (( allow_new_plan )); then
  export METASYSTEM_ALLOW_NEW_PLAN=1
fi

run_required_step "verify checks" verify_checks
run_required_step "stage caller paths" stage_changes
run_required_step "commit" commit_changes
run_required_step "verify clean after commit" require_clean_after_commit
run_required_step "fetch origin" fetch_origin
run_required_step "rebase onto origin/$branch" rebase_origin

push_attempt=1
push_limit=3
while (( push_attempt <= push_limit )); do
  run_step "push origin (attempt $push_attempt of $push_limit)" push_origin
  push_rc=$?
  if (( push_rc == 0 )); then
    break
  fi
  if ! push_was_moving_origin_rejection || (( push_attempt == push_limit )); then
    fail_step "$push_rc"
  fi
  printf -- '-- retryable rejection: %s (exit %s)\n' "$step_name" "$push_rc"
  tail -n 40 "$step_output"
  printf -- '-- origin moved during push; fetching and rebasing before retry %s of %s\n' \
    "$((push_attempt + 1))" "$push_limit"
  run_required_step "fetch origin after push attempt $push_attempt" fetch_origin
  run_required_step "rebase onto origin/$branch after push attempt $push_attempt" rebase_origin
  push_attempt=$((push_attempt + 1))
done

if (( ! skip_transport )); then
  run_required_step "sync transport" bash "$root/scripts/agents/sync-transport.sh" "$branch"
fi
