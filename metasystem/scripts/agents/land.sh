#!/usr/bin/env bash
# Each named command owns its output file and its captured status. Errexit is
# deliberately absent because the driver, not an implicit shell branch, must
# decide whether a push rejection is retryable and which exit code reaches the
# caller.
set -uo pipefail

usage() {
  echo "Usage: scripts/agents/land.sh -m <message-file-or-heredoc> [--goal <id>] [--carried <opid>] [--chain <root-job> [--recertification <record> --test-receipt <path>] [--direct-fix register-carriage] | --direct-fix register-carriage | --direct-fix exact-revert --revert-of <commit> | --direct-fix tier-1 --root-job <job-id> (--test-receipt <path> | --tests <legacy-command>)] [--staged-only | <pathspec>...] [--ratchet <path>] [--allow-new-plan] [--skip-transport]" >&2
}

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P) || exit $?
cd "$root" || exit $?
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"
testing_contract=$($ms config conf-value --file "$root/metasystem.conf" --key testing.contract 2>/dev/null || true)

# Carried mode owns one lease epoch from the first fetch through the final
# transport. The hidden entry is passed to commit.sh so that child never tries
# to nest a second run-held lock.
landing_carried_requested=0
for argument in "$@"; do
  [[ "$argument" != -- ]] || break
  [[ "$argument" != --carried ]] || landing_carried_requested=1
done
lease_epoch=
if [[ ${1:-} == __lease-held ]]; then
  shift
  lease_epoch=${1:-}
  [[ -n "$lease_epoch" ]] || exit 2
  shift
  if [[ "$lease_epoch" =~ ^[1-9][0-9]*$ ]]; then
    "$ms" lease require-holder --root "$root" --caller-pid "$$" --expected-epoch "$lease_epoch" >/dev/null || exit $?
  else
    [[ "$lease_epoch" == human ]] || exit 2
    "$ms" lease require-holder --root "$root" --caller-pid "$$" >/dev/null || exit $?
  fi
elif (( landing_carried_requested )); then
  lease_result=$("$ms" lease require-holder --root "$root" --caller-pid "$$") || exit $?
  lease_epoch=$("$ms" json get --value "$lease_result" --field claimEpoch --default "")
  if [[ -n "$lease_epoch" ]]; then
    exec "$ms" lease run-held --root "$root" --caller-pid "$$" --expected-epoch "$lease_epoch" \
      -- "$0" __lease-held "$lease_epoch" "$@"
  fi
  exec "$ms" lease run-held --root "$root" --caller-pid "$$" \
    -- "$0" __lease-held human "$@"
fi

message_source=
staged_only=0
allow_new_plan=0
skip_transport=0
ratchet=
landing_chain=
landing_direct_fix=
landing_revert_of=
landing_goal=
landing_goal_set=0
landing_root_job=
landing_tests=
landing_test_receipt=
landing_recertification=
landing_carried=
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
    --chain)
      [[ $# -ge 2 && -z "$landing_chain" ]] || { usage; exit 2; }
      landing_chain=$2
      shift 2
      ;;
    --direct-fix)
      [[ $# -ge 2 && -z "$landing_direct_fix" ]] || { usage; exit 2; }
      landing_direct_fix=$2
      shift 2
      ;;
    --revert-of)
      [[ $# -ge 2 && -z "$landing_revert_of" ]] || { usage; exit 2; }
      landing_revert_of=$2
      shift 2
      ;;
    --goal)
      [[ $# -ge 2 && $landing_goal_set -eq 0 ]] || { usage; exit 2; }
      landing_goal=$2
      landing_goal_set=1
      shift 2
      ;;
    --root-job)
      [[ $# -ge 2 && -z "$landing_root_job" ]] || { usage; exit 2; }
      landing_root_job=$2
      shift 2
      ;;
    --tests)
      [[ $# -ge 2 && -z "$landing_tests" ]] || { usage; exit 2; }
      landing_tests=$2
      shift 2
      ;;
    --test-receipt)
      [[ $# -ge 2 && -z "$landing_test_receipt" ]] || { usage; exit 2; }
      landing_test_receipt=$2
      shift 2
      ;;
    --recertification)
      [[ $# -ge 2 && -z "$landing_recertification" ]] || { usage; exit 2; }
      landing_recertification=$2
      shift 2
      ;;
    --carried)
      [[ $# -ge 2 && -z "$landing_carried" ]] || { usage; exit 2; }
      landing_carried=$2
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

set +e
brain_fence=$("$ms" brain fence --root "$root" --act land)
brain_fence_rc=$?
set -e
if [[ $brain_fence_rc -eq 2 ]]; then
  "$ms" json get --value "$brain_fence" --field detail >&2
  exit 2
elif [[ $brain_fence_rc -ne 0 ]]; then
  echo "land refused: brain fence failed" >&2
  exit 1
fi
set +e

[[ -n "$message_source" ]] || { usage; exit 2; }
if (( staged_only && ${#pathspecs[@]} > 0 )); then
  echo "land refused: --staged-only cannot be combined with pathspecs" >&2
  exit 2
fi
if (( ! staged_only && ${#pathspecs[@]} == 0 )) && [[ -z "$landing_carried" ]]; then
  echo "land refused: name pathspecs or choose --staged-only" >&2
  exit 2
fi
if [[ -n "$landing_tests" && -n "$landing_test_receipt" ]]; then
  echo "land refused: --tests and --test-receipt cannot be combined; remove --test-receipt for a tier-1 landing, or remove --tests for a receipted chain landing" >&2
  usage
  exit 2
fi
if [[ -n "$landing_test_receipt" && -z "$landing_chain" && "$landing_direct_fix" != tier-1 && -z "$landing_carried" ]]; then
  echo "land refused: --test-receipt belongs with --chain or --direct-fix tier-1" >&2
  usage
  exit 2
fi
if [[ -n "$landing_carried" && ( $landing_goal_set -eq 0 || -z "$landing_goal" ) ]]; then
  echo "land refused: --carried requires --goal" >&2
  exit 2
fi
if [[ -n "$landing_recertification" && -z "$landing_chain" ]]; then
  echo "land refused: --recertification requires --chain <root-job>" >&2
  exit 2
fi
if [[ -n "$landing_recertification" && -n "$landing_direct_fix" && "$landing_direct_fix" != register-carriage ]]; then
  echo "land refused: --recertification combines only with the existing register-carriage class" >&2
  exit 2
fi
if [[ -n "$landing_recertification" && -z "$landing_test_receipt" ]]; then
  echo "land refused: a recertified landing requires a fresh --test-receipt for the actual candidate" >&2
  exit 2
fi
if [[ "$landing_direct_fix" == tier-1 ]]; then
  [[ $landing_goal_set -eq 1 && -n "$landing_goal" && -n "$landing_root_job" && ( -n "$landing_tests" || -n "$landing_test_receipt" ) ]] || {
    echo "land refused: --direct-fix tier-1 requires --goal, --root-job, and either --test-receipt or legacy --tests" >&2
    exit 2
  }
elif [[ -n "$landing_root_job" || -n "$landing_tests" ]]; then
  echo "land refused: --root-job and --tests belong only to --direct-fix tier-1" >&2
  exit 2
fi

if [[ -n "$landing_chain" && "$landing_chain" =~ ^[a-z0-9][a-z0-9-]*$ ]]; then
  chain_gate_width=$("$ms" json get \
    --file "$root/artifacts/agents/jobs/$landing_chain.json" \
    --field gateWidth --default area 2>/dev/null || true)
  if [[ "$chain_gate_width" == full && -z "$landing_test_receipt" ]]; then
    if [[ -n "$testing_contract" ]]; then
      echo "land refused: chain $landing_chain requires sufficient schema-2 testing evidence; run metasystem landing test-receipt --root . --tree <whole-project-tree> --mode auto and pass it with --test-receipt" >&2
    else
      echo "land refused: legacy full-width chain $landing_chain requires its full-battery receipt" >&2
    fi
    exit 2
  fi
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
  local status=$?
  local abandon_output abandon_rc
  local -a abandon_args
  if (( ${carry_abandon_armed:-0} )) && [[ -n "${carried_row:-}" ]]; then
    local why=${carry_stop_reason:-"carried landing exited before its push (status $status)"}
    if [[ -n "${carried_entry:-}" ]]; then
      "$ms" goal carried --root "$root" --entry "$carried_entry" >/dev/null 2>&1 || true
    fi
    abandon_args=(goal carrying --root "$root" --id "$landing_goal" --abandon "$carried_row" --why "$why")
    abandon_output=$("$ms" "${abandon_args[@]}" 2>&1)
    abandon_rc=$?
    if (( abandon_rc != 0 )); then
      printf 'carried landing could not close its reservation: %s\n' "$abandon_output" >&2
    fi
  fi
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
  carry_stop_reason="step $step_name failed with exit $rc: $(tail -n 1 "$step_output" 2>/dev/null || true)"
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
      if (( ${#removed_bare_ids[@]} > 0 )); then
        for removed_id in "${removed_bare_ids[@]}"; do
          if [[ "$added_id" == "$removed_id" ]]; then
            paired=1
            break
          fi
        done
      fi
      (( paired )) && continue

      already_offending=0
      if (( ${#offending_bare_ids[@]} > 0 )); then
        for removed_id in "${offending_bare_ids[@]}"; do
          if [[ "$added_id" == "$removed_id" ]]; then
            already_offending=1
            break
          fi
        done
      fi
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
  if [[ -n "$landing_carried" && "$branch" != main ]]; then
    echo "carry asks: the carried landing lands main; you are on $branch" >&2
    return 3
  fi
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
  if (( ${#pathspecs[@]} == 0 )); then
    git diff --check --
  else
    git diff --check -- "${pathspecs[@]}"
  fi
}

stage_changes() {
  local untracked
  if (( ! staged_only )); then
    if (( ${#pathspecs[@]} == 0 )); then
      echo "land refused: name pathspecs or choose --staged-only" >&2
      return 2
    fi
    git add -- "${pathspecs[@]}" || return $?
  fi
  if git diff --cached --quiet --; then
    echo "land refused: the caller-selected staging set is empty" >&2
    return 2
  fi
  if ! git diff --quiet --; then
    echo "land refused: unstaged changes remain after staging; transport requires a clean tree after commit" >&2
    return 2
  fi
  untracked=$(git ls-files --others --exclude-standard) || return $?
  if [[ -n "$untracked" ]]; then
    echo "land refused: untracked paths remain after staging; transport requires a clean tree after commit" >&2
    printf '  %s\n' "$untracked" >&2
    return 2
  fi
}

commit_changes() {
  local arguments=(-F "$message_file")
  if [[ -n "$ratchet" ]]; then
    arguments=(--ratchet "$ratchet" "${arguments[@]}")
  fi
  [[ -z "$landing_chain" ]] || arguments=(--chain "$landing_chain" "${arguments[@]}")
  [[ -z "$landing_direct_fix" ]] || arguments=(--direct-fix "$landing_direct_fix" "${arguments[@]}")
  [[ -z "$landing_revert_of" ]] || arguments=(--revert-of "$landing_revert_of" "${arguments[@]}")
  (( landing_goal_set )) && arguments=(--goal "$landing_goal" "${arguments[@]}")
  [[ -z "$landing_root_job" ]] || arguments=(--root-job "$landing_root_job" "${arguments[@]}")
  [[ -z "$landing_test_receipt" ]] || arguments=(--test-receipt "$landing_test_receipt" "${arguments[@]}")
  [[ -z "$landing_recertification" ]] || arguments=(--recertification "$landing_recertification" "${arguments[@]}")
  if [[ -n "$landing_carried" ]]; then
    arguments=(--carried "$landing_carried" --ledger-tip "$carried_ledger_tip" \
      --carried-by "$carried_by" --carried-past "$carried_past" "${arguments[@]}")
    bash "$root/scripts/agents/commit.sh" __lease-held "$lease_epoch" "${arguments[@]}"
  else
    bash "$root/scripts/agents/commit.sh" "${arguments[@]}"
  fi
}

staged_candidate_tree() {
  local candidate_tree prefix
  candidate_tree=$(git -C "$root" write-tree) || return $?
  prefix=$(git -C "$root" rev-parse --show-prefix) || return $?
  if [[ -n "$prefix" ]]; then
    candidate_tree=$(git -C "$root" rev-parse "$candidate_tree:${prefix%/}") || return $?
  fi
  printf '%s\n' "$candidate_tree"
}

staged_project_tree() {
  git -C "$root" write-tree
}

check_supplied_test_receipt() {
  local candidate_tree receipt_tree receipt_schema receipt_failure= receipt_workspace candidate_workspace verify_output verify_status
  local receipt_workspace_present=0
  local -a verify_arguments
  # A caller-provided --test-receipt belongs to one exact staged candidate.
  receipt_schema=$($ms json get --file "$landing_test_receipt" --field schemaVersion 2>/dev/null || true)
  if [[ "$receipt_schema" == 2 ]] && "$ms" json get --file "$landing_test_receipt" --field workspace >/dev/null 2>&1; then
    receipt_workspace_present=1
  fi
  if [[ "$receipt_schema" == 2 ]]; then
    candidate_tree=$(staged_project_tree) || return $?
  else
    candidate_tree=$(staged_candidate_tree) || return $?
  fi
  if [[ ! -e "$landing_test_receipt" ]]; then
    receipt_failure=missing
  elif [[ ! -f "$landing_test_receipt" || ! -r "$landing_test_receipt" ]]; then
    receipt_failure=unreadable
  elif ! receipt_tree=$("$ms" json get --file "$landing_test_receipt" --field tree 2>/dev/null); then
    receipt_failure="no tree field"
  fi
  if [[ -n "$receipt_failure" ]]; then
    echo "land refused: the receipt at $landing_test_receipt cannot be read as a landing receipt ($receipt_failure)" >&2
    return 2
  fi
  if [[ "$receipt_tree" == "$candidate_tree" ]]; then
    return 0
  fi
  if (( ! receipt_workspace_present )); then
    echo "land refused: the receipt at $landing_test_receipt names tree $receipt_tree but the staged candidate is $candidate_tree; make the receipt against this exact candidate" >&2
    return 2
  fi
  if receipt_workspace=$("$ms" landing workspace --root "$root" --tree "$receipt_tree") &&
     candidate_workspace=$("$ms" landing workspace --root "$root" --tree "$candidate_tree") &&
     [[ "$receipt_workspace" == "$candidate_workspace" ]]; then
    return 0
  fi
  verify_arguments=(test verify --root "$root" --tree "$candidate_tree" --mode auto --purpose delivery)
  (( landing_goal_set )) && verify_arguments+=(--goal "$landing_goal")
  verify_output=$("$ms" "${verify_arguments[@]}" 2>&1)
  verify_status=$?
  if (( verify_status == 0 )); then
    return 0
  fi
  echo "land refused: the receipt at $landing_test_receipt names tree $receipt_tree but the staged candidate is $candidate_tree; make the receipt against this exact candidate" >&2
  printf '%s\n' "$verify_output" | tail -n 20 >&2
  return 2
}

create_test_receipt() {
  local candidate_tree
  [[ -n "$landing_tests" ]] || return 0
  candidate_tree=$(staged_candidate_tree) || return $?
  "$ms" landing test-receipt --root "$root" --tree "$candidate_tree" --command "$landing_tests" || return $?
  landing_test_receipt="$root/artifacts/agents/landing/receipts/$candidate_tree.json"
}

require_clean_after_commit() {
  local status
  status=$(git status --porcelain --untracked-files=normal) || return $?
  if [[ -n "$status" ]]; then
    echo "land refused: commit succeeded but the tree is not clean, so transport will not start" >&2
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

recert_target=
recert_source_ref=
recert_merged_ref=
recert_candidate_tree=
recert_candidate_commit=

load_recertification_transport_facts() {
  local repository_top record_path
  [[ -n "$landing_recertification" ]] || return 0
  repository_top=$(git -C "$root" rev-parse --show-toplevel) || return $?
  [[ "$landing_recertification" != /* ]] || {
    echo "land refused: --recertification must be the canonical repository-relative path" >&2
    return 2
  }
  record_path=$repository_top/$landing_recertification
  recert_target=$("$ms" json get --file "$record_path" --field targetCommit 2>/dev/null || true)
  recert_source_ref=$("$ms" json get --file "$record_path" --field sourceAnchorRef 2>/dev/null || true)
  recert_merged_ref=$("$ms" json get --file "$record_path" --field mergedAnchorRef 2>/dev/null || true)
  if [[ -z "$recert_target" ]]; then
    # An explicitly selected but unreadable proof is still parked after the
    # Go evaluator names its refusal. The frozen local target is the only
    # target identity available for that diagnostic record.
    recert_target=$(git -C "$root" rev-parse HEAD^{commit}) || return $?
  fi
}

park_recertified() { # original reason, detail
  local reason=$1 detail=$2 park_output park_rc
  local -a arguments=(landing park --root "$root" --chain "$landing_chain" \
    --target "$recert_target" --reason "$reason" --detail "$detail" \
    --recertification "$landing_recertification")
  [[ -z "$recert_candidate_commit" ]] || arguments+=(--candidate-commit "$recert_candidate_commit")
  [[ -z "$recert_source_ref" ]] || arguments+=(--recovery-ref "$recert_source_ref")
  [[ -z "$recert_merged_ref" ]] || arguments+=(--recovery-ref "$recert_merged_ref")
  park_output=$("$ms" "${arguments[@]}" 2>&1)
  park_rc=$?
  if (( park_rc == 0 )); then
    echo "PARKED"
    printf '%s\n' "$park_output"
    exit 1
  fi
  echo "PARK-FAILED cause=$reason" >&2
  printf '%s\n' "$park_output" >&2
  exit 1
}

check_recertification_target() {
  local local_head remote_head
  local_head=$(git -C "$root" rev-parse HEAD^{commit}) || return $?
  remote_head=$(git -C "$root" rev-parse "refs/remotes/origin/$branch^{commit}") || return $?
  if [[ "$local_head" != "$recert_target" || "$remote_head" != "$recert_target" ]]; then
    echo "chain-recertification-target-moved: local=$local_head remote=$remote_head expected=$recert_target" >&2
    return 1
  fi
}

recertification_refusal_from_output() {
  LC_ALL=C sed -nE 's/.*code=([a-z][a-z0-9-]*).*/\1/p' "$step_output" | head -n 1
}

verify_recertified_commit() {
  local current parent tree
  current=$(git -C "$root" rev-parse HEAD^{commit}) || return $?
  parent=$(git -C "$root" rev-parse HEAD^1) || return $?
  tree=$(git -C "$root" rev-parse HEAD^{tree}) || return $?
  if [[ "$parent" != "$recert_target" || "$tree" != "$recert_candidate_tree" ]]; then
    echo "chain-recertification-target-moved: committed parent/tree $parent/$tree differ from $recert_target/$recert_candidate_tree" >&2
    return 1
  fi
  recert_candidate_commit=$current
}

verify_current_testing_proof() {
  local tree
  local -a arguments
  [[ -n "$testing_contract" ]] || return 0
  tree=$(git -C "$root" rev-parse HEAD^{tree}) || return $?
  arguments=(test verify --root "$root" --tree "$tree" --mode auto --purpose delivery)
  (( landing_goal_set )) && arguments+=(--goal "$landing_goal")
  "$ms" "${arguments[@]}"
}

carry_ask() { # exact human-visible ask
  carry_stop_reason=$1
  printf '%s\n' "$1" >&2
  exit 3
}

fixture_pause() { # named carried transaction seam
	local wanted=${METASYSTEM_LAND_FIXTURE_PAUSE:-} crash=${METASYSTEM_LAND_FIXTURE_CRASH:-} fatal=${METASYSTEM_LAND_FIXTURE_KILL:-} runtime
	[[ "$wanted" == "$1" || "$crash" == "$1" || "$fatal" == "$1" ]] || return 0
  runtime=$("$ms" config conf-value --file "$root/metasystem.conf" --key metasystem.runtimes 2>/dev/null || true)
  [[ "$runtime" == fake ]] || return 0
	if [[ "$fatal" == "$1" ]]; then
	  printf 'FIXTURE-KILL %s pid=%s\n' "$1" "$$"
	  kill -KILL "$$"
	  exit 137
	fi
	if [[ "$crash" == "$1" ]]; then
	  printf 'FIXTURE-CRASH %s pid=%s\n' "$1" "$$"
	  kill -TERM "$$"
	fi
  printf 'FIXTURE-PAUSE %s pid=%s\n' "$1" "$$"
  while :; do sleep 1; done
}

goal_fetch_for_carry() {
  local output rc
  output=$("$ms" goal fetch --root "$root" 2>&1)
  rc=$?
  if (( rc != 0 )); then
    printf '%s\n' "$output" >&2
    exit 3
  fi
  carried_ledger_tip=${output#*tip=}
  carried_ledger_tip=${carried_ledger_tip%% *}
  [[ "$carried_ledger_tip" =~ ^[0-9a-f]{40}$ ]] || carry_ask "goal fetch returned no accepted ledger tip: $output"
}

read_carry_status() {
  local output rc
  output=$("$ms" landing carry-status --root "$root" --carried "$landing_carried" \
    --goal "$landing_goal" --ledger-tip "$carried_ledger_tip" --json 2>&1)
  rc=$?
  if (( rc != 0 )); then
    printf '%s\n' "$output" >&2
    exit 3
  fi
  carried_word=$("$ms" json get --value "$output" --field word 2>/dev/null || true)
  carried_consumption=$("$ms" json get --value "$output" --field consumption 2>/dev/null || true)
  carried_reservation=$("$ms" json get --value "$output" --field reservation 2>/dev/null || true)
  carried_intent=$("$ms" json get --value "$output" --field intent 2>/dev/null || true)
  carried_counselor=$("$ms" json get --value "$output" --field counselor --default "" 2>/dev/null || true)
  carried_past=$("$ms" json get --value "$output" --field past --default "" 2>/dev/null || true)
  carried_by=$("$ms" json get --value "$output" --field by --default "" 2>/dev/null || true)
  carried_workspace=$("$ms" json get --value "$output" --field workspace --default "" 2>/dev/null || true)
  carried_source=$("$ms" json get --value "$output" --field source --default "" 2>/dev/null || true)
  [[ -n "$carried_word" && -n "$carried_consumption" && -n "$carried_reservation" ]] \
    || carry_ask "carry status was incomplete; fetch the ledger and rerun"
}

carry_forward_staged() { # release reservation row on an ask when supplied
  local release=${1:-} base wip rebase_rc conflicts candidate_tree candidate_workspace
  if [[ $(git rev-parse HEAD) != $(git rev-parse refs/remotes/origin/main) ]]; then
    base=$(git rev-parse HEAD) || exit $?
    wip=$(git commit-tree "$(git write-tree)" -p HEAD -m "carried wip") || exit $?
    git update-ref HEAD "$wip" "$base" || exit $?
    git rebase refs/remotes/origin/main >"$step_output" 2>&1
    rebase_rc=$?
    if (( rebase_rc != 0 )); then
      conflicts=$(git diff --name-only --diff-filter=U | paste -sd, -)
      git rebase --abort >/dev/null 2>&1 || exit $?
      git reset --soft "$base" || exit $?
      if [[ -n "$release" ]]; then
        "$ms" goal carrying --root "$root" --id "$landing_goal" --abandon "$release" \
          --why "carried landing stopped at rebase conflict" >/dev/null 2>&1 || true
      fi
      carry_ask "rebase conflict on ${conflicts:-unknown paths}; resolve by hand against origin/main, stage, rerun"
    fi
    git reset --soft refs/remotes/origin/main || exit $?
  fi
  candidate_tree=$(staged_project_tree) || exit $?
  candidate_workspace=$("$ms" landing workspace --root "$root" --tree "$candidate_tree" 2>/dev/null || true)
  if [[ "$candidate_workspace" != "$carried_workspace" ]]; then
    if [[ -n "$release" ]]; then
      "$ms" goal carrying --root "$root" --id "$landing_goal" --abandon "$release" \
        --why "origin moved the carried workspace" >/dev/null 2>&1 || true
    fi
    if [[ "$carried_source" == answer ]]; then
      "$ms" channel ask --root "$root" --goal "$landing_goal" --kind carry \
        --wants "carry workspace=$candidate_workspace goal=$landing_goal past=$carried_past" \
        --fact "origin moved the candidate workspace after the channel carry word" >/dev/null 2>&1 || true
    fi
    carry_ask "word workspace=$carried_workspace candidate workspace=$candidate_workspace; run goal carry --root . --id $landing_goal --by ${carried_by#human:} --tree $candidate_tree --past $carried_past --why 'origin moved the carried workspace' --supersede $landing_carried"
  fi
}

reserve_carry() {
  local tree output rc
  tree=$(staged_project_tree) || exit $?
  output=$("$ms" goal carrying --root "$root" --id "$landing_goal" --ref "$landing_carried" \
    --tree "$tree" --by "$carried_by" 2>&1)
  rc=$?
  if (( rc != 0 )); then
    printf '%s\n' "$output" >&2
    exit 3
  fi
  carried_row=${output#carrying=}
  carried_row=${carried_row%% ledger=*}
  carried_ledger_tip=${output##* ledger=}
  [[ -n "$carried_row" && "$carried_ledger_tip" =~ ^[0-9a-f]{40}$ ]] \
    || carry_ask "reservation returned an incomplete row: $output"
  carry_abandon_armed=1
}

trailer_value() { # commit, key
  local commit=$1 key=$2 values count
  values=$(git log -1 --format=%B "$commit" | sed -n "s/^${key}: //p") || return $?
  count=$(printf '%s\n' "$values" | sed '/^$/d' | wc -l | tr -d ' ')
  [[ "$count" == 1 ]] || return 1
  printf '%s\n' "$values"
}

verify_local_carried_commit() { # commit
  local commit=$1 tree workspace message key count carry goal_count
  tree=$(git rev-parse "$commit^{tree}") || return $?
  workspace=$("$ms" landing workspace --root "$root" --tree "$tree") || return $?
  if [[ "$workspace" != "$carried_workspace" ]]; then
    if [[ "$carried_source" == answer ]]; then
      "$ms" channel ask --root "$root" --goal "$landing_goal" --kind carry \
        --wants "carry workspace=$workspace goal=$landing_goal past=$carried_past" \
        --fact "the recovered commit workspace differs from the channel carry word" >/dev/null 2>&1 || true
    fi
    carry_ask "word workspace=$carried_workspace candidate workspace=$workspace; run goal carry --root . --id $landing_goal --by ${carried_by#human:} --tree $tree --past $carried_past --why 'the recovered carried workspace changed' --supersede $landing_carried"
  fi
  message=$(git log -1 --format=%B "$commit") || return $?
  for key in Carry Carried-By Carried-Tree Carried-Past Carried-Battery Carried-Judge Carried-Ledger; do
    count=$(LC_ALL=C grep -Ec "^${key}:" <<<"$message" || true)
    [[ "$count" == 1 ]] || {
      echo "land refused: local carried commit $commit has $count $key trailers; expected exactly one" >&2
      return 1
    }
  done
  carry=$(trailer_value "$commit" Carry) || return 1
  [[ "$carry" == "$landing_carried" ]] || {
    echo "land refused: local carried commit names Carry: $carry, not $landing_carried" >&2
    return 1
  }
  goal_count=$(grep -Fxc -- "Goal-Item: $landing_goal" <<<"$message" || true)
  [[ "$goal_count" == 1 ]] || {
    echo "land refused: local carried commit must have exactly one Goal-Item: $landing_goal" >&2
    return 1
  }
}

ensure_recovery_reservation() {
  case "$carried_reservation" in
    "reservation: open:"*) carried_row=${carried_reservation#reservation: open:} ;;
    "reservation: expired:"*) carry_ask "word $landing_carried expired; issue a fresh goal carry; local commit remains at HEAD" ;;
    *)
      reserve_carry
      fetch_origin || carry_ask "origin fetch failed after restoring the carry reservation"
      if [[ $(git rev-parse HEAD) != $(git rev-parse refs/remotes/origin/main) ]]; then
        git rebase refs/remotes/origin/main >"$step_output" 2>&1
        if (( $? != 0 )); then
          git rebase --abort >/dev/null 2>&1 || true
          carry_ask "rebase conflict while recovering local carried commit; resolve by hand against origin/main and rerun"
        fi
      fi
      ;;
  esac
}

create_carried_intent() {
  local commit=$1 tree tree_line battery_line judge_line battery missing=- failing=- judge judge_tree= digest live_failure= output rc
  tree=$(git rev-parse "$commit^{tree}") || exit $?
  tree_line=$(trailer_value "$commit" Carried-Tree) || exit 1
  battery_line=$(trailer_value "$commit" Carried-Battery) || exit 1
  judge_line=$(trailer_value "$commit" Carried-Judge) || exit 1
  carried_past=$(trailer_value "$commit" Carried-Past) || exit 1
  carried_by=$(trailer_value "$commit" Carried-By) || exit 1
  carried_ledger_tip=$(trailer_value "$commit" Carried-Ledger) || exit 1
  carried_workspace=${tree_line#workspace=}
  carried_workspace=${carried_workspace%% project=*}
  battery=${battery_line%% *}
  [[ "$battery_line" != *" missing="* ]] || { missing=${battery_line#* missing=}; missing=${missing%% *}; }
  [[ "$battery_line" != *" failing="* ]] || { failing=${battery_line#* failing=}; failing=${failing%% *}; }
  judge=${judge_line%% *}
  digest=${judge_line#* sha256=}; digest=${digest%% *}
  [[ "$judge_line" != *" tree="* ]] || { judge_tree=${judge_line#* tree=}; judge_tree=${judge_tree%% *}; }
  [[ "$judge_line" != *" live-failure="* ]] || { live_failure=${judge_line#* live-failure=}; live_failure=${live_failure%% *}; }
  intent_args=(goal carrying --root "$root" --id "$landing_goal" --ref "$landing_carried" \
    --carrying "$carried_row" --commit "$commit" --tree "$tree" --workspace "$carried_workspace" \
    --past "$carried_past" --battery "$battery" --missing "$missing" --failing "$failing" \
    --judge "$judge" --judge-digest "$digest" --ledger "$carried_ledger_tip" \
    --by "$carried_by" --owner-pid "$$")
  [[ -z "$judge_tree" ]] || intent_args+=(--judge-tree "$judge_tree")
  [[ -z "$live_failure" ]] || intent_args+=(--live-failure "$live_failure")
  output=$("$ms" "${intent_args[@]}" 2>&1)
  rc=$?
  if (( rc != 0 )); then
    printf '%s\n' "$output" >&2
    exit "$rc"
  fi
  carried_entry=${output#carrying=}
  carried_entry=${carried_entry%% ledger=*}
  [[ -n "$carried_entry" ]] || { echo "land refused: carried intent returned no entry" >&2; exit 1; }
}

print_carried_advisory() { # commit
  local commit=$1 judge_line live_failure=- ordinary battery_line sufficient=false
  local missing=- failing=- goal_text exception_count=0 finding
  judge_line=$(trailer_value "$commit" Carried-Judge) || exit 1
  ordinary=$(trailer_value "$commit" Landing-Provenance-Verdict) || exit 1
  battery_line=$(trailer_value "$commit" Carried-Battery) || exit 1
  if [[ "$judge_line" == *" live-failure="* ]]; then
    live_failure=${judge_line#* live-failure=}
    live_failure=${live_failure%% *}
  fi
  if [[ "$battery_line" == green ]]; then
    sufficient=true
  else
    [[ "$battery_line" != *" missing="* ]] || { missing=${battery_line#* missing=}; missing=${missing%% *}; }
    [[ "$battery_line" != *" failing="* ]] || { failing=${battery_line#* failing=}; failing=${failing%% *}; }
  fi
  goal_text=$(git show "$carried_ledger_tip:./plans/goals/$landing_goal.md") || exit $?
  exception_count=$(sed -n 's/^- BudgetExceptions: \([0-9][0-9]*\)$/\1/p' <<<"$goal_text" | head -1)
  [[ "$exception_count" =~ ^[0-9]+$ ]] || exception_count=0
  printf 'carried reservation: %s\n' "$carried_row"
  printf 'carried ledger: %s\n' "$carried_ledger_tip"
  printf 'carried judge: %s\n' "$judge_line"
  printf 'carried live failure: %s\n' "$live_failure"
  printf 'carried ordinary verdict: %s\n' "$ordinary"
  printf 'carried testing result: sufficient=%s missing=%s failing=%s uncovered=- discrepancies=-\n' \
    "$sufficient" "$missing" "$failing"
  finding="carried:$commit"
  [[ "$battery_line" == green ]] || finding+=:battery-red
  printf 'carried obligation finding: %s\n' "$finding"
  printf 'carried exception count after this one: %s\n' "$((exception_count + 1))"
}

finish_carried_publication() {
  local commit=$1
  create_carried_intent "$commit"
	print_carried_advisory "$commit"
  fixture_pause before-push
  run_step "push carried commit to origin (single attempt)" push_origin
  push_rc=$?
  if (( push_rc != 0 )); then
    if push_was_moving_origin_rejection; then
      carry_ask "origin moved during the push; rerun land.sh --carried $landing_carried"
    fi
    fail_step "$push_rc"
  fi
  carry_abandon_armed=0
  fixture_pause after-push
  run_required_step "complete carried goal record" "$ms" goal carried --root "$root" --entry "$carried_entry"
  fixture_pause after-record
  if (( ! skip_transport )); then
    run_required_step "sync transport" bash "$root/scripts/agents/sync-transport.sh" "$branch"
  fi
}

run_carried_landing() {
  local commit
  run_step "fetch origin for carried landing" fetch_origin
  if (( $? != 0 )); then
    carry_ask "the code remote could not be fetched; repair origin and rerun land.sh --carried $landing_carried"
  fi
  goal_fetch_for_carry
  read_carry_status
  case "$carried_consumption" in
    superseded:*)
      replacement=${carried_consumption#superseded:}
      carry_ask "word $landing_carried was superseded by $replacement; land under it: land.sh --carried $replacement"
      ;;
    ledger:*)
      if [[ "$carried_counselor" == "counselor: missing" ]]; then
        run_required_step "repair carried counselor record" "$ms" goal carried --root "$root" --repair-counselor --ref "$landing_carried"
      fi
      if (( ! skip_transport )); then
        run_required_step "sync transport" bash "$root/scripts/agents/sync-transport.sh" "$branch"
      fi
      printf 'already recorded in the goal ledger as %s\n' "${carried_consumption#ledger:}"
      return 0
      ;;
    origin:*)
      commit=${carried_consumption#origin:}
      printf 'already landed as %s; completing the record\n' "$commit"
      if [[ "$carried_intent" == carrying:* ]]; then
        carried_entry=${carried_intent#carrying:}
        run_required_step "complete carried goal record" "$ms" goal carried --root "$root" --entry "$carried_entry"
      else
        run_required_step "rebuild carried goal record" "$ms" goal carried --root "$root" \
          --id "$landing_goal" --ref "$landing_carried" --rebuild-from-commit "$commit"
      fi
      if (( ! skip_transport )); then
        run_required_step "sync transport" bash "$root/scripts/agents/sync-transport.sh" "$branch"
      fi
      return 0
      ;;
    local:*)
      commit=${carried_consumption#local:}
      if [[ $(git rev-parse HEAD) != $(git rev-parse refs/remotes/origin/main) ]]; then
        git rebase refs/remotes/origin/main >"$step_output" 2>&1
        if (( $? != 0 )); then
          git rebase --abort >/dev/null 2>&1 || true
          carry_ask "rebase conflict while recovering local carried commit; resolve by hand against origin/main and rerun"
        fi
        commit=$(git rev-parse HEAD)
      fi
      ensure_recovery_reservation
      carry_abandon_armed=1
      commit=$(git rev-parse HEAD)
      verify_local_carried_commit "$commit" || exit $?
      finish_carried_publication "$commit"
      return 0
      ;;
  esac
  case "$carried_word" in
    ok) ;;
    expired) carry_ask "word $landing_carried expired; issue a fresh goal carry" ;;
    missing) carry_ask "carry word $landing_carried is missing on goal $landing_goal; fetch the ledger" ;;
    unproven) carry_ask "carry word $landing_carried is not proven; issue it from a verified terminal" ;;
    *) carry_ask "carry word $landing_carried has unknown state $carried_word" ;;
  esac
  [[ "$carried_consumption" == none ]] || carry_ask "word $landing_carried has unsupported consumption state $carried_consumption"
  run_required_step "stage caller paths" stage_changes
  carry_forward_staged
  fixture_pause carry-forward
  reserve_carry
  fixture_pause reservation
  run_required_step "fetch origin after carry reservation" fetch_origin
  carry_forward_staged "$carried_row"
  fixture_pause second-carry-forward
  run_required_step "commit" commit_changes
  commit=$(git rev-parse HEAD)
  verify_local_carried_commit "$commit" || exit $?
  finish_carried_publication "$commit"
}

# Only the flag grants the hook acknowledgment. An inherited shell setting is
# not evidence that this landing's caller chose to include a new plan.
unset METASYSTEM_ALLOW_NEW_PLAN
if (( allow_new_plan )); then
  export METASYSTEM_ALLOW_NEW_PLAN=1
fi

run_required_step "verify checks" verify_checks
run_required_step "load recertification transport facts" load_recertification_transport_facts
if [[ -n "$landing_recertification" ]]; then
  run_required_step "fetch origin before recertified commit" fetch_origin
  run_step "freeze recertified target before commit" check_recertification_target
  target_rc=$?
  if (( target_rc != 0 )); then
    target_detail=$(tail -n 1 "$step_output")
    park_recertified chain-recertification-target-moved "$target_detail"
  fi
fi
if [[ -n "$landing_carried" ]]; then
  run_carried_landing
  exit $?
fi
run_required_step "stage caller paths" stage_changes
if [[ -n "$landing_test_receipt" ]]; then
  if [[ -n "$landing_recertification" ]]; then
    run_step "test receipt for staged candidate" check_supplied_test_receipt
    receipt_rc=$?
    if (( receipt_rc != 0 )); then
      receipt_reason=chain-recertification-test-command-refused
      [[ "${chain_gate_width:-}" != full ]] || receipt_reason=chain-full-gate-refused
      receipt_detail=$(tail -n 1 "$step_output")
      park_recertified "$receipt_reason" "$receipt_detail"
    fi
  else
    run_required_step "test receipt for staged candidate" check_supplied_test_receipt
  fi
fi
run_required_step "tier-1 test receipt" create_test_receipt
if [[ -n "$landing_recertification" ]]; then
  recert_candidate_tree=$(staged_project_tree) || fail_step $?
  run_step "recheck recertified target before commit" check_recertification_target
  target_rc=$?
  if (( target_rc != 0 )); then
    target_detail=$(tail -n 1 "$step_output")
    park_recertified chain-recertification-target-moved "$target_detail"
  fi
  run_step "commit" commit_changes
  commit_rc=$?
  if (( commit_rc != 0 )); then
    refusal=$(recertification_refusal_from_output || true)
    if [[ -n "$refusal" ]]; then
      refusal_detail=$(tail -n 1 "$step_output")
      park_recertified "$refusal" "$refusal_detail"
    fi
    fail_step "$commit_rc"
  fi
  run_step "verify recertified commit parent and tree" verify_recertified_commit
  commit_verify_rc=$?
  if (( commit_verify_rc != 0 )); then
    commit_verify_detail=$(tail -n 1 "$step_output")
    park_recertified chain-recertification-target-moved "$commit_verify_detail"
  fi
else
  run_required_step "commit" commit_changes
fi
if [[ -n "$landing_recertification" ]]; then
  run_step "verify clean after commit" require_clean_after_commit
  clean_rc=$?
  if (( clean_rc != 0 )); then
    clean_detail=$(tail -n 1 "$step_output")
    park_recertified chain-recertification-source-changed "$clean_detail"
  fi
else
  run_required_step "verify clean after commit" require_clean_after_commit
fi
if [[ -n "$landing_recertification" ]]; then
  run_step "verify shared testing proof before recertified push" verify_current_testing_proof
  proof_rc=$?
  if (( proof_rc != 0 )); then
    proof_detail=$(tail -n 1 "$step_output")
    park_recertified chain-recertification-test-command-refused "$proof_detail"
  fi
  run_step "push recertified commit to origin (single attempt)" push_origin
  push_rc=$?
  if (( push_rc != 0 )); then
    if push_was_moving_origin_rejection; then
      push_detail=$(tail -n 1 "$step_output")
      park_recertified chain-recertification-target-moved "$push_detail"
    fi
    fail_step "$push_rc"
  fi
else
  run_required_step "fetch origin" fetch_origin
  run_required_step "rebase onto origin/$branch" rebase_origin
  run_required_step "verify shared testing proof after rebase" verify_current_testing_proof

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
    run_required_step "verify shared testing proof after retry rebase" verify_current_testing_proof
    push_attempt=$((push_attempt + 1))
  done
fi

if (( ! skip_transport )); then
  run_required_step "sync transport" bash "$root/scripts/agents/sync-transport.sh" "$branch"
fi
