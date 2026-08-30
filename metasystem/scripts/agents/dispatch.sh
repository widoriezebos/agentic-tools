#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/agents/dispatch.sh status --job <job-id>
  scripts/agents/dispatch.sh watch --job <job-id>
  scripts/agents/dispatch.sh cancel --job <job-id>
  scripts/agents/dispatch.sh close --job <root-id> [--runner-closed] [--reconcile-evidence <job-id>]
  scripts/agents/dispatch.sh reap [--job <job-id>]

Exit codes: 0 success/completed; 2 usage; 3 failed; 4 timeout;
5 vanished; 6 unknown status job; 7 malformed status record; 8 cancelled;
10 critique cap exhausted and waiting on a human raise.
USAGE
}

die() { last_die_message=$2; echo "$2" >&2; exit "$1"; }

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
repo_scope=$(git -C "$root" rev-parse --show-toplevel 2>/dev/null) \
  || die 1 "metasystem installation is not inside a git repository: $root"
repo_scope=$(cd "$repo_scope" && pwd -P)
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"
# Operator launches have one public owner. This shell keeps custody callbacks
# but refuses the retired authority-bearing grammar by naming its replacement.
if [[ ${BASH_SOURCE[0]} == "$0" && -z "${METASYSTEM_DELEGATE_INTERNAL:-}" ]]; then
  case "${1:-dispatch}" in
    dispatch|follow-up|cancel|--*)
      printf '%s\n' '{"outcome":"REFUSED-REQUEST","headline":"refused","detail":"the legacy dispatch authority grammar was removed; use metasystem delegate"}'
      exit 2 ;;
  esac
fi
config="$root/scripts/metasystem-config.sh"
source "$root/scripts/agents/checkout-execution-guard.sh"
agents="$root/artifacts/agents"
jobs="$agents/jobs"
heartbeats="$agents/hb"
locks="$agents/locks"
record_locks="$agents/record-locks"
capabilities="$agents/capabilities"
worktrees="$agents/worktrees"
process_instance_tag=
cap_authority_lock_held=0
goal_revision_lock_held=0
goal_revision_lock_dir=
goal_revision_lock_goal=
goal_revision_lock_revision=0
stop_cancel_authorized=
exit_cleanup_job=
exit_cleanup_chain=
exit_cleanup_authorization=
exit_cleanup_message=
exit_cleanup_lifecycle=
# Flight-recorder witness (docs/design/flight-recorder.md). emit_event never fails.
if [[ -f "$(dirname "${BASH_SOURCE[0]}")/emit-event.sh" ]]; then
  source "$(dirname "${BASH_SOURCE[0]}")/emit-event.sh"
else
  emit_event() { :; }
fi
emit_component() { # D-3a attribution: the only shell reaper left is dispatch-held
  printf dispatch
}
reap_verdict_events() { # job, verdict, reason, cas_rc, cas_out
  local job=$1 verdict=$2 reason=$3 cas_rc=$4 observed=$5 job_mission
  job_mission=$(json_field "$agents/jobs/$job.json" mission 2>/dev/null || true)
  if [[ "$cas_rc" == 0 ]]; then
    emit_event "$(emit_component)" job-verdict "jobId=$job" "missionId=$job_mission" "verdict=$verdict" "reason=$reason" "summary=$reason"
  elif [[ "$cas_rc" == 3 ]]; then
    observed=${observed#observed=}
    emit_event "$(emit_component)" verdict-refused "jobId=$job" "missionId=$job_mission" "attempted=$verdict" "observed=${observed:-unknown}" "summary=CAS refused: wanted $verdict, found ${observed:-unknown}"
  fi
}
# How long the reaper waits past a record's handshake budget before calling a
# unfinished-handshake job process-lost. See the handshake branch in reap_one_locked.
handshake_backstop_grace_sec=2
arm_supervision="$root/scripts/agents/arm-supervision.sh"
mission_fence() { local fence_verb=$1; shift; "$ms" mission "fence-$fence_verb" "$@"; }
entry_caller_pid=$$
current_claim_epoch=
current_main_id=
current_caller_class=
lease_reentry=0

record_delegate_outcome() { # outcome, headline, optional detail, optional job
  local outcome=$1 headline=$2 detail=${3:-} outcome_job=${4:-}
  [[ -n "${METASYSTEM_DELEGATE_OUTCOME_FILE:-}" ]] || return 0
  "$ms" json object "outcome=$outcome" "headline=$headline" "detail=$detail" "jobId=$outcome_job" \
    >"$METASYSTEM_DELEGATE_OUTCOME_FILE"
}

record_delegate_outcome_raw() { # already encoded JSON
  [[ -n "${METASYSTEM_DELEGATE_OUTCOME_FILE:-}" ]] || return 0
  printf '%s\n' "$1" >"$METASYSTEM_DELEGATE_OUTCOME_FILE"
}

die() {
  last_die_message=$2
  if [[ -n "${METASYSTEM_DELEGATE_OUTCOME_FILE:-}" && ! -s "$METASYSTEM_DELEGATE_OUTCOME_FILE" ]]; then
    record_delegate_outcome REFUSED-INTERNAL refused "$2" "${job:-${child:-}}"
  fi
  echo "$2" >&2
  exit "$1"
}

valid_id() { [[ "$1" =~ ^[a-z0-9][a-z0-9-]*$ ]]; }
now_iso() { date -u +%Y-%m-%dT%H:%M:%SZ; }
sha256_file() { "$ms" util sha256 --file "$1"; }

dispatch_fixture_wait_cap() { # base seconds; normal dispatch remains 1x
  local base=$1 scale_milli=${METASYSTEM_FIXTURE_CAP_SCALE_MILLI:-1000}
  [[ "$base" =~ ^[1-9][0-9]*$ && "$scale_milli" =~ ^[1-9][0-9]*$ ]] \
    || die 2 "dispatch wait cap inputs must be positive integers"
  printf '%s\n' "$(( (base * scale_milli + 999) / 1000 ))"
}

milliseconds_to_sleep() { # positive integer milliseconds
  local milliseconds=$1
  [[ "$milliseconds" =~ ^[1-9][0-9]*$ ]] \
    || die 2 "poll interval must be a positive integer in milliseconds"
  printf '%d.%03d\n' "$((milliseconds / 1000))" "$((milliseconds % 1000))"
}

report_plan_drift() {
  # Surfaced here as well as at end of turn, because the end-of-turn hook needs
  # a runtime that fires hooks and only one of the three has ever been observed
  # doing so. Every agent that delegates passes through this function whatever
  # runtime it is, so a plan contradicting the job records cannot stay invisible
  # on Codex or Devin merely because their hooks are unproven. Reporting only:
  # a stale plan is never a reason to refuse work.
  local output
  output=$("$ms" report open-work --repo "$root" 2>/dev/null | grep '^STALE-PLAN' || true)
  [[ -z "$output" ]] || printf '%s\n' "$output" >&2
}

require_fresh_census() {
  # One verb, one verdict (script-orchestration-12): freshness AND the
  # fingerprint match are both the engine's judgment now.
  local verdict="$agents/supervision/last-census.json" state="$agents/supervision/state.json"
  [[ -f "$verdict" ]] || die 1 "dispatch refused: census verdict is absent; run $arm_supervision --repo $repo_scope"
  "$ms" job census-fresh --verdict "$verdict" --state "$state" \
    --arm "$arm_supervision" --repo "$repo_scope" --root "$root" || exit $?
}

json_field() { # file, dotted field
  "$ms" json get --file "$1" --field "$2"
}

json_value() { # json string, dotted field
  "$ms" json get --value "$1" --field "$2"
}

record_cas() { # job, expected status, target status, patch file
  local cas_rc=0
  if [[ -n "$stop_cancel_authorized" ]]; then
    "$ms" job record-cas --root "$root" --job "$1" --expect "$2" --status "$3" --patch "$4" || cas_rc=$?
  else
    "$0" __record-cas --job "$1" --expect "$2" --status "$3" --patch "$4" || cas_rc=$?
  fi
  # FRCC-010: witness the lifecycle transitions post-CAS. Only genuine
  # status changes emit; metadata updates (expect == target) stay silent.
  if [[ "$cas_rc" == 0 && "$2" != "$3" ]]; then
    case "$3" in
      pending|running)
        emit_event dispatch "job-$3" "jobId=$1" \
          "missionId=$(json_field "$agents/jobs/$1.json" mission 2>/dev/null || true)" \
          "summary=transition $2 -> $3" ;;
    esac
  fi
  # One-shot by contract: twenty call sites mktemp a patch and none cleaned it,
  # which is how record-locks reached 142k files. The wrapper is the one point
  # that always runs.
  rm -f -- "$4" 2>/dev/null || true
  return "$cas_rc"
}

record_cas_preserve_patch() { # job, expected status, target status, patch file
  # Launch ownership keeps its exact identity patch until a lost compare has
  # re-proved that the just-launched process is still at the recorded pid.
  "$0" __record-cas --job "$1" --expect "$2" --status "$3" --patch "$4"
}

record_create() { # job, source json
  "$0" __record-create --job "$1" --source "$2" && emit_event dispatch job-created "jobId=$1" "summary=record created"
}

# A setup refusal releases the mission slot it reserved. When claim-launch
# already created a pending-setup record, the exiting dispatch also fails that
# reservation. The compare-and-swap no-ops once the record advanced past
# pending-setup, so a successful dispatch is untouched.
fail_setup_husk() { # job id
  local husk_job=$1 husk_patch
  [[ -n "$husk_job" ]] || return 0
  if [[ ! -f "$jobs/$husk_job.json" ]]; then
    if [[ -n "${mission:-}" ]]; then
      mission_fence release-job --repo "$root" --mission "$mission" --job "$husk_job" >/dev/null 2>&1 || true
    fi
    return 0
  fi
  husk_patch=$(mktemp "${TMPDIR:-/tmp}/metasystem-husk-fail.XXXXXX")
  # Classify the refusal for the flight recorder: rep 1 of
  # bm-1-20260813t132947z needed mktemp file sizes as its primary
  # diagnostic instrument because refused dispatches emitted nothing.
  local refusal_class=setup
  case "${last_die_message:-}" in
    *"mission fence"*) refusal_class=fence ;;
    *worktree*) refusal_class=worktree ;;
    *permission*) refusal_class=envelope ;;
    *snapshot*|*capabilit*) refusal_class=capability ;;
  esac
  emit_event "$(emit_component)" job-refused "jobId=$husk_job" "missionId=${mission:-}" \
    "reasonClass=$refusal_class" "summary=dispatch refused ($refusal_class): ${last_die_message:-unknown}" || true
  printf '{"error":"dispatch-refused","phase":"setup","refusalClass":"%s"}\n' "$refusal_class" >"$husk_patch"
  "$ms" job record-cas --root "$root" --job "$husk_job" \
    --expect pending-setup --status failed --patch "$husk_patch" >/dev/null 2>&1 || true
  rm -f "$husk_patch"
  # A dispatch that died in setup never started a process: its fence
  # reservation must not keep counting against fence.jobs and holding a
  # concurrency slot (rep 1 of bm-1-20260813t132947z lost half its signed
  # job budget to exactly this).
  if [[ -n "${mission:-}" ]]; then
    mission_fence release-job --repo "$root" --mission "$mission" --job "$husk_job" >/dev/null 2>&1 || true
  fi
}

release_unpublished_authorization() { # job id
  local authorized_job=$1
  [[ -n "$authorized_job" && -n "${mission:-}" ]] || return 0
  mission_fence release-job --repo "$root" --mission "$mission" --job "$authorized_job" >/dev/null 2>&1 || true
}

record_setup() { # job, complete source json
  "$0" __record-setup --job "$1" --source "$2" && emit_event dispatch job-setup "jobId=$1" "summary=setup complete"
}

lease_entry_check() {
  local result
  result=$("$ms" lease require-holder --root "$root" --caller-pid "$entry_caller_pid") \
    || exit $?
  current_claim_epoch=$("$ms" json get --value "$result" --field claimEpoch --default "")
  current_main_id=$("$ms" json get --value "$result" --field mainId --default "")
  current_caller_class=$("$ms" json get --value "$result" --field class)
}

lease_run_held() { # expected epoch (empty for human), command...
  local expected=$1
  shift
  if [[ "${current_caller_class:-}" == STEWARD ]]; then
    # A dead worker holds no lease; the steward's authority is
    # enforced per-write by the internal entries' checks.
    "$@"
    return
  fi
  if [[ -n "$expected" ]]; then
    "$ms" lease run-held --root "$root" --caller-pid "$entry_caller_pid" \
      --expected-epoch "$expected" -- "$@"
  else
    "$ms" lease run-held --root "$root" --caller-pid "$entry_caller_pid" -- "$@"
  fi
}

internal_authority() { # control-plane authority mode, optional job id
  local mode=$1 job=${2:-} result
  result=$("$ms" lease classify --root "$root" --caller-pid "$entry_caller_pid") \
    || die 1 "control-plane write refused: caller classification failed"
  if [[ -n "$job" ]]; then
    "$ms" job authority-check \
      --mode "$mode" --classification "$result" --job "$job"
  else
    "$ms" job authority-check \
      --mode "$mode" --classification "$result"
  fi
}

# The four-way liveness verdict lives in `proc classify`
# (script-orchestration-09): live, stale, dead, or unknown, one semantics
# for every ladder rung instead of two-way ps probes that turned an
# unreadable process table into process-lost on the kill-capable path.
tag_state() { # pid, tag -> live, stale, dead, or unknown
  [[ "$1" =~ ^[1-9][0-9]*$ ]] || { printf 'dead\n'; return; }
  "$ms" proc classify --pid "$1" --tag "$2"
}

process_exists() { # pid; permission denied still proves the pid exists
  # An empty or null pid (a record mid-handshake) is simply not an existing
  # process; without this guard the flag parser prints a usage dump to
  # stderr on every such probe (noise, not a defect — the verdict was
  # already correct).
  [[ "$1" =~ ^[1-9][0-9]*$ ]] || return 1
  "$ms" proc exists --pid "$1"
}

lock_owner_state() { # pid, tag -> live, dead, stale, or unknown
  tag_state "$1" "$2"
}

job_supervisor_matches() { # record
  local record=$1 pid pgid tag runtime heartbeat proof_pid proof_pgid proof_tag proof_source proof_time
  pid=$(json_field "$record" pid 2>/dev/null || true)
  tag=$(json_field "$record" instanceTag 2>/dev/null || true)
  case "$(tag_state "$pid" "$tag")" in
    live) return 0 ;;
    # Indeterminacy never acts: an uninspectable supervisor is not a dead
    # one, so the kill-capable callers defer exactly as arm-supervision's
    # identity ladder does (script-orchestration-09).
    unknown) return 0 ;;
  esac
  runtime=$(json_field "$record" runtime 2>/dev/null || true)
  [[ "$runtime" == fake ]] || return 1
  "$ms" proc alive --identity-file "$record" --root "$root" >/dev/null 2>&1 || return 1
  pgid=$(json_field "$record" pgid 2>/dev/null || true)
  proof_pid=$(json_field "$record" ownershipProof.pid 2>/dev/null || true)
  proof_pgid=$(json_field "$record" ownershipProof.pgid 2>/dev/null || true)
  proof_tag=$(json_field "$record" ownershipProof.instanceTag 2>/dev/null || true)
  proof_source=$(json_field "$record" ownershipProof.source 2>/dev/null || true)
  proof_time=$(json_field "$record" ownershipProof.provenAt 2>/dev/null || true)
  heartbeat="$heartbeats/$(json_field "$record" jobId 2>/dev/null || true)"
  [[ "$proof_pid" == "$pid" && "$proof_pgid" == "$pgid" && "$proof_tag" == "$tag" \
    && "$proof_source" == trusted-launcher && -n "$proof_time" && -f "$heartbeat" ]] || return 1
  [[ "$(json_field "$heartbeat" pid 2>/dev/null || true)" == "$pid" \
    && "$(json_field "$heartbeat" instanceTag 2>/dev/null || true)" == "$tag" ]]
}

group_alive() { # pgid
  local pgid=$1
  [[ "$pgid" =~ ^[1-9][0-9]*$ ]] || return 1
  "$ms" proc group-exists --pgid "$pgid"
}

group_owned() { # record, optional pgid
  local record=$1 pgid=${2:-} tag
  [[ -n "$pgid" ]] || pgid=$(json_field "$record" pgid 2>/dev/null || true)
  tag=$(json_field "$record" instanceTag 2>/dev/null || true)
  [[ "$pgid" =~ ^[1-9][0-9]*$ && "$pgid" -gt 1 ]] || return 1
  "$ms" proc group-owned --pgid "$pgid" --tag "$tag" --root "$root" --record "$record"
}

wind_down_one_group() { # record, pgid
  local record=$1 pgid=$2 until
  group_alive "$pgid" || return 0
  group_owned "$record" "$pgid" || { echo "refusing to signal unowned process group $pgid" >&2; return 1; }
  kill -TERM -- "-$pgid" 2>/dev/null || true
  until=$(( $(date +%s) + 2 ))
  while group_alive "$pgid" && (( $(date +%s) < until )); do sleep 0.05; done
  if group_alive "$pgid"; then
    group_owned "$record" "$pgid" || { echo "lost ownership proof for process group $pgid" >&2; return 1; }
    kill -KILL -- "-$pgid" 2>/dev/null || true
  fi
  until=$(( $(date +%s) + 2 ))
  while group_alive "$pgid" && (( $(date +%s) < until )); do sleep 0.05; done
  group_alive "$pgid" && { echo "process group $pgid survived KILL" >&2; return 1; }
  return 0
}

wind_down_group() { # record
  local record=$1 groups pgid refused=0
  groups=$("$ms" job custody-groups --record "$record") || return 1
  while IFS= read -r pgid; do
    [[ -z "$pgid" ]] && continue
    if ! wind_down_one_group "$record" "$pgid"; then
      refused=1
    fi
  done <<<"$groups"
  # When this shell launched the supervisor directly, reap its terminal wait
  # status now so a zombie group leader cannot masquerade as a live writer.
  wait "$(json_field "$record" pid 2>/dev/null || true)" 2>/dev/null || true
  (( refused == 0 ))
}

# One primitive for both directory locks, because both got the same two rules
# wrong. A claim publishes the directory and its owner in ONE step: a directory
# rename replaces only an EMPTY directory, so it claims an absent lock, heals an
# ownerless husk left by an older crash, and refuses an owned one. Creating the
# directory first and writing the owner second left a window in which a
# contender read an ownerless lock and refused. A release frees only a lock this
# process still owns, and never fails when it no longer does -- a release that
# deletes whatever it finds hands a live owner's lock to a third writer.
# The whole protocol (staged rename, holder liveness, husk healing) lives
# in internal/dispatch/ownerlock.go behind this one verb.
owner_lock() { # claim|release, directory, pid, tag -> 0 done, 3 busy, 4 not-owner
  "$ms" job owner-lock --command "$1" --dir "$2" --pid "$3" --tag "$4"
}


acquire_chain_lock() { # root id
  local chain=$1 dir="$locks/$1.d" status=0 pid tag owner_state
  mkdir -p "$locks"
  owner_lock claim "$dir" "$$" "$process_instance_tag" || status=$?
  (( status == 0 )) && return 0
  pid=$(json_field "$dir/owner.json" pid 2>/dev/null || true)
  tag=$(json_field "$dir/owner.json" instanceTag 2>/dev/null || true)
  [[ -n "$pid" ]] || die 1 "chain lock has no owner lease: $dir"
  owner_state=$(lock_owner_state "$pid" "$tag")
  [[ "$owner_state" != unknown ]] || die 1 "chain lock owner liveness cannot be verified: $chain"
	die 1 "chain is busy: $chain"
}

# A process-creating command queues behind the short chain section so a
# repeated operation can be resolved by its launch claim. Mechanical lock
# failures still refuse immediately; only a standing owner is waited out.
acquire_launch_chain_lock() { # root id
  local chain=$1 dir="$locks/$1.d" status maximum deadline holder_pid holder_tag holder
  mkdir -p "$locks"
  maximum=$(dispatch_fixture_wait_cap 10)
  deadline=$((SECONDS + maximum))
  while true; do
    status=0
    owner_lock claim "$dir" "$$" "$process_instance_tag" || status=$?
    case "$status" in
      0) return 0 ;;
      3)
        if (( SECONDS >= deadline )); then
          holder_pid=$(json_field "$dir/owner.json" pid 2>/dev/null || true)
          holder_tag=$(json_field "$dir/owner.json" instanceTag 2>/dev/null || true)
          holder=unreadable
          [[ -z "$holder_pid$holder_tag" ]] || holder="pid=${holder_pid:-unknown},tag=${holder_tag:-unknown}"
          record_delegate_outcome LOCK_BUSY refused "rank=chain key=$chain holder=$holder retry=retry-after-the-named-holder-releases" "${job:-${child:-}}"
          die 1 "LOCK_BUSY rank=chain key=$chain holder=$holder retry=retry-after-the-named-holder-releases"
        fi
        sleep 0.05
        ;;
      *) die 1 "cannot acquire launch chain lock: $chain" ;;
    esac
  done
}

release_chain_lock() { # root id
  [[ -n "$1" ]] || return 0
  local status=0
  owner_lock release "$locks/$1.d" "$$" "$process_instance_tag" || status=$?
  (( status == 4 )) && die 1 "refusing to release another owner's chain lock"
  return 0
}

acquire_goal_revision_lock() { # goal id, revision
  local goal_id=$1 revision=$2 maximum started deadline elapsed holder_pid holder_tag holder
  goal_revision_lock_dir=$("$ms" job goal-lock-path --root "$root" --goal "$goal_id" --revision "$revision") \
    || die 1 "cannot resolve goal-revision lock for $goal_id revision $revision"
  maximum=$(dispatch_fixture_wait_cap 10)
  started=$SECONDS
  deadline=$(( SECONDS + maximum ))
  while ! owner_lock claim "$goal_revision_lock_dir" "$$" "$process_instance_tag"; do
    if (( SECONDS >= deadline )); then
      elapsed=$((SECONDS - started))
      holder_pid=$(json_field "$goal_revision_lock_dir/owner.json" pid 2>/dev/null || true)
      holder_tag=$(json_field "$goal_revision_lock_dir/owner.json" instanceTag 2>/dev/null || true)
      holder=unreadable
      [[ -z "$holder_pid$holder_tag" ]] || holder="pid=${holder_pid:-unknown},tag=${holder_tag:-unknown}"
      record_delegate_outcome LOCK_BUSY refused "rank=goal-revision key=$goal_id/r$revision holder=$holder retry=retry-after-the-named-holder-releases elapsed=${elapsed}s cap=${maximum}s" "${job:-${child:-}}"
      die 1 "LOCK_BUSY rank=goal-revision key=$goal_id/r$revision holder=$holder retry=retry-after-the-named-holder-releases elapsed=${elapsed}s cap=${maximum}s"
    fi
    sleep 0.05
  done
  goal_revision_lock_held=1
  goal_revision_lock_goal=$goal_id
  goal_revision_lock_revision=$revision
}

release_goal_revision_lock() {
  (( goal_revision_lock_held )) || return 0
  local status=0
  owner_lock release "$goal_revision_lock_dir" "$$" "$process_instance_tag" || status=$?
  (( status == 4 )) && die 1 "refusing to release another owner's goal-revision lock"
  goal_revision_lock_held=0
  goal_revision_lock_dir=
  goal_revision_lock_goal=
  goal_revision_lock_revision=0
}

acquire_lifecycle_lock() { # job id; nonzero means a live owner has it
  mkdir -p "$record_locks"
  owner_lock claim "$record_locks/$1.lifecycle.d" "$$" "$process_instance_tag"
}

acquire_lifecycle_lock_until() { # job id, maximum wait seconds
  local job=$1 base=$2 maximum started deadline elapsed directory holder_pid holder_tag holder
  directory="$record_locks/$job.lifecycle.d"
  maximum=$(dispatch_fixture_wait_cap "$base")
  started=$SECONDS
  deadline=$(( SECONDS + maximum ))
  while ! acquire_lifecycle_lock "$job"; do
    if (( SECONDS >= deadline )); then
      elapsed=$((SECONDS - started))
      holder_pid=$(json_field "$directory/owner.json" pid 2>/dev/null || true)
      holder_tag=$(json_field "$directory/owner.json" instanceTag 2>/dev/null || true)
      holder=unreadable
      [[ -z "$holder_pid$holder_tag" ]] || holder="pid=${holder_pid:-unknown},tag=${holder_tag:-unknown}"
      echo "LOCK_BUSY rank=job-lifecycle key=$job holder=$holder retry=retry-after-the-named-holder-releases elapsed=${elapsed}s cap=${maximum}s" >&2
      return 1
    fi
    sleep 0.05
  done
}

release_lifecycle_lock() { # job id
  owner_lock release "$record_locks/$1.lifecycle.d" "$$" "$process_instance_tag" || true
}

release_exit_lifecycle() {
  [[ -n "$exit_cleanup_lifecycle" ]] || return 0
  release_lifecycle_lock "$exit_cleanup_lifecycle"
  exit_cleanup_lifecycle=
}

acquire_cap_authority_lock() {
  # Identity-bearing owner lock (script-orchestration-01/D18): the old bare
  # mkdir spinlock had no owner and no healer, so a SIGKILLed holder bricked
  # every dispatch AND arming until a human ran rmdir. The owner-lock verb
  # heals a provably dead holder's husk and keeps an unprovable one (B1) —
  # the same protocol the lifecycle and chain locks above already use.
  local directory="$agents/supervision/cap-authority.lock.d" maximum started deadline elapsed holder_pid holder_tag holder
  mkdir -p "${directory%/*}"
  maximum=$(dispatch_fixture_wait_cap 10)
  started=$SECONDS
  deadline=$((SECONDS + maximum))
  while ! owner_lock claim "$directory" "$$" "$process_instance_tag"; do
    if (( SECONDS >= deadline )); then
      elapsed=$((SECONDS - started))
      holder_pid=$(json_field "$directory/owner.json" pid 2>/dev/null || true)
      holder_tag=$(json_field "$directory/owner.json" instanceTag 2>/dev/null || true)
      holder=unreadable
      [[ -z "$holder_pid$holder_tag" ]] || holder="pid=${holder_pid:-unknown},tag=${holder_tag:-unknown}"
      record_delegate_outcome LOCK_BUSY refused "rank=cap-authority key=repository holder=$holder retry=retry-after-the-named-holder-releases elapsed=${elapsed}s cap=${maximum}s" "${job:-${child:-}}"
      die 1 "LOCK_BUSY rank=cap-authority key=repository holder=$holder retry=retry-after-the-named-holder-releases elapsed=${elapsed}s cap=${maximum}s"
    fi
    sleep 0.05
  done
  cap_authority_lock_held=1
}

release_cap_authority_lock() {
  (( cap_authority_lock_held )) || return 0
  local status=0
  owner_lock release "$agents/supervision/cap-authority.lock.d" "$$" "$process_instance_tag" || status=$?
  (( status == 4 )) && die 1 "refusing to release another owner's cap-authority lock"
  cap_authority_lock_held=0
}

require_goal_admission() {
  local output result=0 lineage=${METASYSTEM_OWNER_LINEAGE:-}
  local -a args=(--root "$root")
  [[ -z "$lineage" ]] || args+=(--stop-lineage "$lineage")
  set +e
  output=$("$ms" job goal-admission "${args[@]}" 2>&1)
  result=$?
  set -e
  [[ -z "$output" ]] || printf '%s\n' "$output" >&2
  case "$result" in
    0) return 0 ;;
    10)
      record_delegate_outcome REFUSED-BUDGET refused "$output" "${job:-${child:-}}"
      # Breach-stop owns the next locks. Drop every admission lock before it
      # enters cancellation so the stop path cannot wait on this dispatcher.
      release_cap_authority_lock
      release_goal_revision_lock
      if [[ -n "$exit_cleanup_chain" ]]; then
        release_chain_lock "$exit_cleanup_chain"
        exit_cleanup_chain=
      fi
      run_breach_stop_routes
      die 1 "dispatch refused: breach-stop closed admission and wound down the breached revision" ;;
    9)
      record_delegate_outcome REFUSED-BUDGET refused "$output"
      die 1 "dispatch refused by the goal admission verdict above; supply or revise the governing structured budget before another round" ;;
    *)
      record_delegate_outcome BUDGET_UNKNOWN refused "${output:-the governing goal admission could not be evaluated}" "${job:-${child:-}}"
      die 1 "dispatch refused because the governing goal admission could not be evaluated" ;;
  esac
}

run_breach_stop_routes() {
  local routes goal_id revision prior_stop failure batch stop_id
  routes=$("$ms" job breach-stop-routes --root "$root") \
    || die 1 "breach-stop routes could not be resolved"
  [[ -n "$routes" ]] || die 1 "goal admission required breach-stop but supplied no stoppable route"
  while IFS=$'\t' read -r goal_id revision prior_stop failure; do
    [[ -n "$goal_id" ]] || continue
    [[ -z "$failure" ]] || die 1 "breach-stop for $goal_id revision $revision is indeterminate: $failure"
    batch=$("$ms" job breach-stop --root "$root" --goal "$goal_id" --revision "$revision") \
      || die 1 "breach-stop could not close $goal_id revision $revision"
    stop_id=$(json_value "$batch" stopId)
    internal_breach_stop_run "$stop_id"
  done <<<"$routes"
}

require_goal_revision_admission() { # proposed cap minutes
  local proposed=$1 output result=0 batch stop_id
  [[ -n "${goal:-}" ]] || return 0
  set +e
  output=$("$ms" job goal-revision-admission --root "$root" --goal "$goal" \
    --revision "$goal_revision" --proposed-cap "$proposed" 2>&1)
  result=$?
  set -e
  [[ -z "$output" ]] || printf '%s\n' "$output" >&2
  case "$result" in
    0) return 0 ;;
    10)
      record_delegate_outcome REFUSED-BUDGET refused "$output" "${job:-${child:-}}"
      # Stop takes the goal lock without the lower-ranked cap lock.
      release_cap_authority_lock
      release_goal_revision_lock
      batch=$("$ms" job breach-stop --root "$root" --goal "$goal" --revision "$goal_revision") \
        || die 1 "breach-stop could not close $goal revision $goal_revision"
      stop_id=$(json_value "$batch" stopId)
      release_chain_lock "$exit_cleanup_chain"
      exit_cleanup_chain=
      internal_breach_stop_run "$stop_id"
      die 1 "dispatch refused: breach-stop $stop_id closed the launch fence and completed its cancellation pass" ;;
    9)
      record_delegate_outcome REFUSED-BUDGET refused "$output" "${job:-${child:-}}"
      die 1 "dispatch refused by the exact goal revision admission verdict above" ;;
    *)
      record_delegate_outcome BUDGET_UNKNOWN refused "${output:-exact goal revision admission could not be evaluated}" "${job:-${child:-}}"
      die 1 "dispatch refused because exact goal revision admission could not be evaluated" ;;
  esac
}

require_slice_admission() { # proposed cap minutes, recorded approval reference, goal, revision
  local proposed=$1 approval=${2:-} approval_goal=${3:-} approval_revision=${4:-0} output result=0
  local -a args=(--root "$root" --cap-min "$proposed")
  [[ -z "$approval" ]] || args+=(--approved-ref "$approval")
  [[ -z "$approval_goal" ]] || args+=(--goal "$approval_goal" --goal-revision "$approval_revision")
  set +e
  output=$("$ms" job slice-admission "${args[@]}" 2>&1)
  result=$?
  set -e
  [[ -z "$output" ]] || printf '%s\n' "$output" >&2
  case "$result" in
    0) return 0 ;;
    9)
      record_delegate_outcome_raw "$output"
      die 1 "dispatch refused by the slice-cap admission verdict above" ;;
    *)
      record_delegate_outcome SLICE_ADMISSION_UNKNOWN refused "${output:-slice-cap admission could not be evaluated}" "${job:-${child:-}}"
      die 1 "dispatch refused because slice-cap admission could not be evaluated" ;;
  esac
}

config_get() { "$config" get "$@"; }

canonical_model() { "$ms" config canonical-model "$1"; }

# The cap chain, its origin answer, and the unsigned-mission-cap refusal
# live in `job resolve-cap` (script-orchestration-03): the origin now comes
# from the resolver's own precedence instead of a shadow probe, and the
# fence-authority refusal is the engine's decision.
resolve_nonmission_cap() { # role, runtime, canonical model, explicit override, output json
  "$ms" job resolve-cap --conf "$root/metasystem.conf" --role "$1" --runtime "$2" --model "$3" \
    ${4:+--requested "$4"} --output "$5"
}

refuse_unsigned_mission_cap_override() { # role, runtime, canonical model
  "$ms" job resolve-cap --conf "$root/metasystem.conf" --role "$1" --runtime "$2" --model "$3" --mission
}

attested_watcher_ceiling() {
  "$ms" job watcher-ceiling --state "$agents/supervision/state.json"
}

brief_mode() { # brief
  "$ms" job brief-mode --brief "$1"
}

# Roster resolution, tier ranking, and escalation classification moved to
# `job resolve-roster` (script-orchestration-02) — including the lessons the
# retired shell helpers carried: tiers read through the MERGED resolver so
# .local entries stay visible, enumeration unbounded so no fixed cap drops a
# tier, and a gap refuses instead of silently truncating the ranking.

signed_dispatch_envelope_allows() { # mission id, exact runtime:model pair
  "$ms" mission contract-envelope-allows --root "$root" --mission "$1" --pair "$2"
}

confirm_escalation() { # roster pair, requested pair, displayed cost direction
  local roster_pair=$1 requested_pair=$2 cost_direction=$3 confirmation name
  printf 'Roster resolution: %s\n' "$roster_pair" >&2
  printf 'Requested pair: %s\n' "$requested_pair" >&2
  printf 'Cost direction: %s\n' "$cost_direction" >&2
  printf 'Type APPROVE <name> to confirm: ' >&2
  IFS= read -r confirmation || confirmation=
  if [[ "$confirmation" != "APPROVE "* ]]; then
    die 1 "escalation approval declined; re-run without the override, or repeat from an interactive TTY with --approve-escalation and type APPROVE <name>"
  fi
  name=${confirmation#APPROVE }
  if [[ -z "$name" || "$name" =~ ^[[:space:]] || "$name" =~ [[:space:]]$ || "$name" =~ [[:cntrl:]] ]]; then
    die 1 "escalation approval declined; type APPROVE followed by a non-empty name without leading, trailing, or control characters"
  fi
  printf '%s\n' "$name"
}

validate_mission() { # mission id, lease path
  "$ms" job validate-mission --root "$root" --mission "$1" --lease "$2"
}

resolve_mission() { # explicit id; prints mission|lease|turn or ||
  local explicit=$1 env_id=${METASYSTEM_MISSION_ID:-} env_lease=${METASYSTEM_MISSION_LEASE:-}
  local env_turn=${METASYSTEM_MISSION_TURN:-} mission lease
  if [[ -n "$env_id" || -n "$env_lease" ]]; then
    [[ -n "$env_id" && -n "$env_lease" ]] || die 1 "ambiguous inherited mission context: both METASYSTEM_MISSION_ID and METASYSTEM_MISSION_LEASE are required"
  fi
  [[ -z "$env_turn" || -n "$env_id" || -n "$explicit" ]] \
    || die 1 "ambiguous inherited mission context: METASYSTEM_MISSION_TURN requires a mission (METASYSTEM_MISSION_ID with METASYSTEM_MISSION_LEASE, or --mission)"
  [[ -z "$env_turn" ]] || valid_id "$env_turn" || die 1 "invalid inherited mission turn id"
  if [[ -n "$explicit" && -n "$env_id" && "$explicit" != "$env_id" ]]; then
    die 1 "ambiguous mission context: --mission and METASYSTEM_MISSION_ID disagree"
  fi
  mission=${explicit:-$env_id}
  if [[ -z "$mission" ]]; then printf '||\n'; return; fi
  lease=${env_lease:-$agents/missions/$mission/lease.json}
  validate_mission "$mission" "$lease" || die 1 "mission $mission does not have a live, matching lease"
  printf '%s|%s|%s\n' "$mission" "$lease" "$env_turn"
}

expand_permissions() { # requested value, workspace root, worktree flag, output
  local requested=$1 workspace=$2 is_worktree=$3 output=$4 source preset network_floor
  case "$requested" in
    "$root/scripts/agents/permissions/"*.json)
      source=$requested; preset=${requested##*/}; preset=${preset%.json}
      ;;
    *)
      if [[ -f "$requested" ]]; then source=$requested; preset=custom; else source="$root/scripts/agents/permissions/$requested.json"; preset=$requested; fi
      ;;
  esac
  [[ -f "$source" ]] || die 1 "unknown permissions preset or envelope file: $requested"
  # A repository may deny network to every delegate regardless of preset. A
  # benchmark target sets this, because an agent that can reach the internet can
  # download a solution and the measurement stops meaning anything. It only ever
  # narrows: a repository cannot grant access a preset withholds.
  network_floor=$(config_get --key dispatch.permissions.network --default '')
  case "$network_floor" in ''|deny|allow) ;; *) die 1 "dispatch.permissions.network must be deny or allow" ;; esac
  "$ms" job expand-permissions --source "$source" --repo "$repo_scope" \
    --workspace "$workspace" --worktree "$is_worktree" --preset "$preset" \
    --network-floor "$network_floor" --output "$output"
}

permission_envelope_requests_writes() { # preset name or envelope file
  local requested=$1 source write_roots
  if [[ -f "$requested" ]]; then source=$requested; else source="$root/scripts/agents/permissions/$requested.json"; fi
  [[ -f "$source" ]] || return 1
  write_roots=$("$ms" json get --file "$source" --field writeRoots 2>/dev/null) || return 1
  [[ "$write_roots" == \[*\] ]] || return 1
  [[ "$write_roots" != '[]' ]]
}

is_review_role() { # role
  [[ "$1" == code-critic || "$1" == design-critic || "$1" == warden ]]
}

select_snapshot() { # runtime, role, requested envelope, output json
  local runtime=$1 role=$2 envelope=$3 output=$4 adapter="$root/scripts/agents/adapters/$1.sh" identity max_age
  [[ -x "$adapter" ]] || die 1 "runtime adapter is not installed: $runtime"
  identity=$($adapter config-identity) || die 1 "could not read $runtime adapter configuration identity"
  max_age=$(config_get --key capability.snapshot-max-age-days --default 30)
  [[ "$max_age" =~ ^[0-9]+$ ]] || die 1 "capability.snapshot-max-age-days must be a non-negative integer"
  local select_err
  select_err=$(mktemp "${TMPDIR:-/tmp}/metasystem-select-err.XXXXXX")
  if "$ms" job snapshot-select \
      --root "$root" --runtime "$runtime" --role "$role" --identity "$identity" \
      --max-age "$max_age" --envelope "$envelope" --output "$output" 2>"$select_err"; then
    cat "$select_err" >&2; rm -f "$select_err"
    return 0
  fi
  # Self-heal ONLY a genuine snapshot MISS — absent or stale. A CLI that
  # rewrites its own config mid-run (KI-19's class) moves the identity
  # hash and strands every snapshot; that must cost one probe, not a
  # husked dispatch that burned a fence slot (rep 1 of
  # bm-1-20260813t132947z lost two dispatches to it). A select that FOUND
  # a snapshot and refused on policy — an unenforceable envelope field —
  # must stand: a fresh probe would launder the unverified state away.
  if ! grep -qE 'no capability snapshot matches|capability snapshot is stale' "$select_err"; then
    cat "$select_err" >&2; rm -f "$select_err"
    return 1
  fi
  cat "$select_err" >&2; rm -f "$select_err"
  "$adapter" probe >/dev/null || die 1 "capability snapshot missed and the $runtime adapter probe failed"
  identity=$($adapter config-identity) || die 1 "could not read $runtime adapter configuration identity"
  "$ms" job snapshot-select \
    --root "$root" --runtime "$runtime" --role "$role" --identity "$identity" \
    --max-age "$max_age" --envelope "$envelope" --output "$output"
}

root_job_id() { # job record
  "$ms" adapter root-job --jobs "$jobs" --job "$1"
}

latest_chain_record() { # root job
  "$ms" job latest-chain-record --jobs "$jobs" --root "$1"
}

launch_adapter() { # runtime verb job tag capability
  local runtime=$1 verb=$2 job=$3 tag=$4 launch_capability=$5 gate="$heartbeats/$job.start" adapter="$root/scripts/agents/adapters/$runtime.sh" pid patch cap started deadline elapsed poll_sleep handshake_budget handshake_deadline proven_at
  local adapter_command
  local -a ownership_args
  poll_sleep=$(milliseconds_to_sleep "${METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS:-20}")
  mkdir -p "$heartbeats"
  # No spawn for a record that already left pending (a cancel can
  # land between setup and launch); the authoritative close is the
  # ownership CAS below, but not starting at all is cheaper than
  # starting and killing.
  [[ "$(json_field "$jobs/$job.json" status)" == pending ]] || return 1
  adapter_command=("$adapter" "$verb" --job "$job" --start-gate "$gate" --instance-tag "$tag" --launch-capability "$launch_capability")
  if (( checkout_execution_guard_held )); then
    adapter_command=("$root/scripts/agents/checkout-execution-guard.sh" run-member \
      --root "$checkout_execution_guard_root" --engine "$checkout_execution_guard_engine" -- "${adapter_command[@]}")
  fi
  if (( checkout_execution_guard_held )); then
    pid=$("$ms" supervise launch-detached --cwd "$root" \
      --execution-guard-root "$checkout_execution_guard_root" --execution-guard-owner "dispatch job $job" \
      --env "GIT_AUTHOR_NAME=$job" --env "GIT_AUTHOR_EMAIL=$job@metasystem.invalid" -- \
      "${adapter_command[@]}") || return 1
  else
    pid=$("$ms" supervise launch-detached --cwd "$root" \
      --env "GIT_AUTHOR_NAME=$job" --env "GIT_AUTHOR_EMAIL=$job@metasystem.invalid" -- \
      "${adapter_command[@]}") || return 1
  fi
  cap=$(dispatch_fixture_wait_cap 5)
  started=$SECONDS
  deadline=$((SECONDS + cap))
  patch=$(mktemp "$record_locks/launch.XXXXXX")
  # The handshake deadline is stamped at launch, because that is when the
  # dispatcher starts waiting. Measuring from record creation would spend the
  # budget during setup and let the reaper overwrite the dispatcher's verdict.
  handshake_budget=$(json_field "$jobs/$job.json" sessionEstablishedTimeoutSec 2>/dev/null || echo 0)
  proven_at=$(now_iso)
  handshake_deadline=
  if [[ "$handshake_budget" =~ ^[1-9][0-9]*$ ]]; then
    handshake_deadline=$(( $(date +%s) + handshake_budget ))
  fi
  ownership_args=(job ownership-patch --root "$root" --output "$patch" --pid "$pid" --pgid "$pid" --instance-tag "$tag" --proven-at "$proven_at")
  [[ -z "$handshake_deadline" ]] || ownership_args+=(--handshake-deadline "$handshake_deadline")
  until "$ms" "${ownership_args[@]}" 2>/dev/null; do
    if (( SECONDS >= deadline )); then
      elapsed=$((SECONDS - started))
      echo "adapter start identity ceiling reached for $job (elapsed: ${elapsed}s; scaled cap: ${cap}s)" >&2
      rm -f -- "$patch"
      return 1
    fi
    sleep "$poll_sleep"
  done
  # A lost ownership CAS means the record moved on under us (a
  # cancel concluded it mid-launch): the process this launch just
  # started must die NOW, not linger until its start-gate timeout —
  # the record must never claim an outcome while an unrecorded
  # group lives on. Killing needs IDENTITY, never a number: the CAS
  # can wait long enough for the gated adapter to time out and the
  # pid to be reused, so every signal re-proves the launch-captured
  # start identity first, the same ladder wind_down_group climbs.
  if ! record_cas_preserve_patch "$job" pending pending "$patch"; then
    if "$ms" proc alive --identity-file "$patch" --root "$root" >/dev/null 2>&1; then
      kill -TERM -- "-$pid" 2>/dev/null || true
      local lost_until=$(( $(date +%s) + 2 ))
      while "$ms" proc alive --identity-file "$patch" --root "$root" >/dev/null 2>&1 \
        && (( $(date +%s) < lost_until )); do sleep 0.05; done
      if "$ms" proc alive --identity-file "$patch" --root "$root" >/dev/null 2>&1; then
        kill -KILL -- "-$pid" 2>/dev/null || true
      fi
    fi
    rm -f -- "$patch"
    return 1
  fi
  rm -f -- "$patch"
  touch "$gate"
}

await_handshake() { # job, maximum session-established seconds, dispatch claim epoch
  local job=$1 timeout=$2 claim_epoch=${3:-} record="$jobs/$1.json" deadline status session poll_sleep
  [[ "$timeout" =~ ^[1-9][0-9]*$ && "$timeout" -le 60 ]] || return 1
  poll_sleep=$(milliseconds_to_sleep "${METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS:-50}")
  # The deadline stamped at launch, so the waiter and the reaper's backstop
  # work from ONE number. Computing a fresh one here started the clock again
  # after setup had already run, which put this verdict later than the backstop
  # that is supposed to defer to it -- and the backstop won every time.
  deadline=$(json_field "$record" handshakeDeadline 2>/dev/null || true)
  [[ "$deadline" =~ ^[1-9][0-9]*$ ]] || deadline=$(( $(date +%s) + timeout ))
  while (( $(date +%s) <= deadline )); do
    if [[ -f "$record" ]]; then
      status=$(json_field "$record" status 2>/dev/null || true)
      session=$(json_field "$record" sessionId 2>/dev/null || true)
      case "$status" in
        running|completed)
          [[ -f "$jobs/$job.log" && "$session" != null && -n "$session" ]] && return 0
          ;;
        failed|cancelled|timeout) return 1 ;;
      esac
    fi
    sleep "$poll_sleep"
  done
  lease_run_held "$claim_epoch" "$0" __handshake-timeout --job "$job" || true
  return 1
}

wait_for_job() { # job
  local job=$1 record="$jobs/$1.json" status
  touch "$heartbeats/$job.waiting"
  while true; do
    [[ -f "$record" ]] || return 5
    status=$(json_field "$record" status 2>/dev/null || true)
    case "$status" in
      completed|failed|timeout|cancelled)
        lease_run_held "$current_claim_epoch" "$0" __reap-held --job "$job" \
          || return 3
        case "$status" in completed) return 0 ;; failed) return 3 ;; timeout) return 4 ;; cancelled) return 8 ;; esac
        ;;
      pending|running)
        if ! lease_run_held "$current_claim_epoch" "$0" __reap-held --job "$job"; then
          [[ -f "$record" ]] || return 5
          return 3
        fi
        [[ -f "$record" ]] || return 5
        sleep 0.1
        ;;
      *) return 5 ;;
    esac
  done
}

aggregate_chain_usage() { # root id
  local chain=$1 record="$jobs/$1.json" status patch
  [[ -f "$record" ]] || return 0
  status=$(json_field "$record" status 2>/dev/null || true)
  case "$status" in completed|failed|timeout|cancelled) ;; *) return 0 ;; esac
  patch=$(mktemp "$record_locks/usage.XXXXXX")
  local aggregate_rc=0
  "$ms" job chain-usage --jobs "$jobs" --root "$chain" --output "$patch" || aggregate_rc=$?
  if (( aggregate_rc == 7 )); then
    rm -f -- "$patch" 2>/dev/null || true
    return 0
  fi
  record_cas "$chain" "$status" "$status" "$patch" || true
}

aggregate_mission_usage() { # job record
  local record=$1 mission
  mission=$(json_field "$record" mission 2>/dev/null || true)
  [[ -n "$mission" && "$mission" != null ]] || return 0
  mission_fence aggregate-usage --repo "$root" --mission "$mission"
}

mirror_fail() { # job, reason — durable trace beside the jobs it failed for
  printf '%s %s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$1" "$2" \
    >>"${jobs%/jobs}/mirror-failures.log" 2>/dev/null || true
  echo "cannot mirror $1: $2" >&2
}

mirror_record() { # job
  local job=$1 record="$jobs/$1.json" status evidence result patch root_id
  [[ -f "$record" ]] || return 0
  status=$(json_field "$record" status 2>/dev/null || true)
  case "$status" in completed|failed|timeout|cancelled) ;; *) return 0 ;; esac
  evidence=$(config_get --key evidence.root --default '')
  [[ "$evidence" == /* ]] || { mirror_fail "$job" "evidence.root must be absolute"; return 1; }
  root_id=$(root_job_id "$job") || return 1
  result=$(mktemp "$record_locks/mirror-result.XXXXXX")
  if ! "$ms" job mirror --repo "$root" --checkout "$repo_scope" --evidence "$evidence" \
      --root-job "$root_id" --job "$job" --result "$result"; then
    mirror_fail "$job" "copy or verification failed (see stderr above)"
    return 1
  fi
  if [[ "$(json_field "$result" unchanged)" == true \
      && "$(json_field "$record" mirror.manifest 2>/dev/null || true)" == "$(json_field "$result" manifest)" ]]; then
    rm -f -- "$result" 2>/dev/null || true
    return 0
  fi
  patch=$(mktemp "$record_locks/mirror-patch.XXXXXX")
  printf '{"mirror":%s}\n' "$(cat "$result")" >"$patch"
  # The stamp lands ONLY on the job that was actually mirrored. Stamping
  # the whole chain from one job's mirror wrote durability claims for
  # evidence that never landed: a follow-up round carried the root's
  # stamp while its own artifacts were absent from the manifest, and the
  # close then refused a chain every record of which CLAIMED to be
  # mirrored (rep 1 of cohort bm-1-20260813t113617z, KI-6 round 3).
  record_cas "$job" "$status" "$status" "$patch" || return 1
  rm -f -- "$result" 2>/dev/null || true
}

reap_one_locked() { # job
  local job=$1 record="$jobs/$1.json" status phase pid tag facts budget_expired patch root_id mission record_epoch lease_epoch refusal_reason refusal_error truncated_by
  [[ -f "$record" ]] || return 0
  status=$(json_field "$record" status 2>/dev/null || true)
  case "$status" in
    completed|failed|timeout|cancelled)
      root_id=$(root_job_id "$job" 2>/dev/null || true)
      [[ -n "$root_id" ]] && aggregate_chain_usage "$root_id"
      aggregate_mission_usage "$record" || true
      mirror_record "$job" || true
      return
      ;;
    pending-setup|pending|running) ;;
    *) return ;;
  esac
  # One verb call yields every record-only reap fact: pending-setup
  # abandonment, the handshake window, reconciliation readiness, and budget
  # expiry. Budget expiry is the SAME decision code the supervision reaper
  # runs, so the two can never disagree about one record.
  facts=$("$ms" job reap-facts --record "$record" --grace "$handshake_backstop_grace_sec") || return 1
  pid=$(json_field "$record" pid 2>/dev/null || true)
  phase=$(json_field "$record" phase 2>/dev/null || true)
  # A cancellation that won before any process identity was published has no
  # group to kill and no death to prove. Conclude the visible marker directly.
  if [[ ( -z "$pid" || "$pid" == null ) && "$phase" == cancelling ]]; then
    patch=$(mktemp "$record_locks/cancelled-before-launch.XXXXXX")
    printf '{"error":null,"phase":"supervision"}\n' >"$patch"
    record_cas "$job" "$status" cancelled "$patch" >/dev/null 2>&1 || true
    mirror_record "$job" || true
    return 0
  fi
  if [[ "$status" == pending-setup ]]; then
    if [[ "$("$ms" json get --value "$facts" --field reconciliationDue)" == true ]]; then
      "$ms" job reconcile-reservation --root "$root" --job "$job" >/dev/null
    fi
    return 0
  fi
  tag=$(json_field "$record" instanceTag 2>/dev/null || true)
  # A job is inside its handshake while it has no session, whether its record
  # still says pending or an adapter has already moved it to running. The
  # dispatcher owns the handshake verdict: it is the process that was waiting,
  # and it names the failure handshake_timeout. The reaper is the backstop for
  # a dispatcher that is no longer there, so it waits out the dispatcher's own
  # deadline -- the one stamped at launch -- rather than recomputing a
  # different one from the record's creation time.
  if [[ "$("$ms" json get --value "$facts" --field handshakeWaiting)" == true ]]; then
    # The window defers to a dispatcher that is still waiting. A supervisor
    # that is provably gone will never complete a handshake, so there is
    # nothing left to defer to and waiting out the budget only delays the true
    # diagnosis.
    #
    # "Provably gone" needs the record to name a supervisor first. Between
    # creating the record and the adapter publishing its identity there is no
    # pid to match, and treating that absence as death reaped every job in its
    # own launch window -- the supervisor had not died, it had not arrived.
    if [[ -z "$pid" || "$pid" == null ]] || job_supervisor_matches "$record"; then
      return
    fi
  fi
  # Without a primary identity there is no process death to judge and no group
  # to wind down. A due fingerprinted reservation belongs to nonce-wide
  # reconciliation; a legacy identityless record remains deferred.
  if [[ -z "$pid" || "$pid" == null ]]; then
    if [[ "$("$ms" json get --value "$facts" --field reconciliationDue)" == true ]]; then
      "$ms" job reconcile-reservation --root "$root" --job "$job" >/dev/null
    fi
    return 0
  fi
  # The cap is judged BEFORE process liveness. An expired budget is a fact of
  # the record alone (startedAt + capMin); whether the job's process happens to
  # be dead by the time a reaper looks is scheduling noise. Judging liveness
  # first made the verdict a race: the same expired job read timeout from the
  # waiting dispatcher but process-lost from the standing reaper whenever its
  # process had already exited -- two different verdicts for one fact, and the
  # fence's job-cap-min refusal was skipped on the losing side.
  if [[ "$("$ms" json get --value "$facts" --field budgetExpired)" == true ]]; then
    budget_expired=1
  else
    budget_expired=0
  fi
  # The priority applies only to a job that actually RAN: a pending job's
  # budget never started burning, its legal failure is process-lost or
  # handshake_timeout, and pending->timeout is not a lawful transition.
  if [[ "$status" != running || "$budget_expired" != 1 ]]; then
    if ! job_supervisor_matches "$record"; then
      wind_down_group "$record" || return 1
      if recollect_lost_return "$job" "$record" "$status"; then return; fi
      patch=$(mktemp "$record_locks/lost.XXXXXX")
      printf '{"error":"process-lost","phase":"supervision","groupDeathProvenAt":"%s"}\n' "$(now_iso)" >"$patch"
      cas_out=$(record_cas "$job" "$status" failed "$patch" 2>/dev/null) && cas_rc=0 || cas_rc=$?
      reap_verdict_events "$job" failed process-lost "$cas_rc" "$cas_out"
      mirror_record "$job" || true
      return
    fi
  fi
  if [[ "$budget_expired" == 1 ]]; then
    wind_down_group "$record" || return 1
    patch=$(mktemp "$record_locks/timeout.XXXXXX")
    printf '{"error":"budget-cap","phase":"supervision","groupDeathProvenAt":"%s"}\n' "$(now_iso)" >"$patch"
    cas_out=$(record_cas "$job" "$status" timeout "$patch" 2>/dev/null) && cas_rc=0 || cas_rc=$?
    reap_verdict_events "$job" timeout budget-cap "$cas_rc" "$cas_out"
    mission=$(json_field "$record" mission 2>/dev/null || true)
    if [[ $cas_rc -eq 0 && -n "$mission" && "$mission" != null ]]; then
      truncated_by=$(json_field "$record" capResolution.truncatedBy 2>/dev/null || true)
      refusal_reason=job-cap-min
      [[ "$truncated_by" == wall-clock ]] && refusal_reason=wall-clock-hours
      if ! refusal_error=$(mission_fence refuse --repo "$root" --mission "$mission" --reason "$refusal_reason" 2>&1 >/dev/null); then
        refusal_error=${refusal_error//$'\n'/ }
        printf 'MISSION-FENCE-ASK-FAILED mission=%s job=%s error=%s\n' "$mission" "$job" "$refusal_error" >&2
      fi
      aggregate_mission_usage "$record" || true
    fi
    mirror_record "$job" || true
  fi
}

# recollect_lost_return (return-recollection-on-process-lost, D64 at the
# job level): a job dying process-lost with a complete, schema-valid
# return.json in its newest round delivered its work — the return is
# adjudicated and the job concludes completed with recollection
# provenance. Only a truly absent or invalid return stays failed. A raw
# candidate that never reached normalization cannot validate post-mortem
# (the schema demands the session observation), so it lawfully stays lost.
recollect_lost_return() { # job id, record path, current status
  local job=$1 record=$2 status=$3 role round_dir round_base best=0 return_file patch usage_arg cas_out cas_rc
  [[ "$status" == running ]] || return 1
  role=$(json_field "$record" role 2>/dev/null || true)
  round_dir=""
  local candidate_dir
  for candidate_dir in "$agents/$job/rounds"/*/; do
    [[ -d "$candidate_dir" ]] || continue
    round_base=$(basename "$candidate_dir")
    [[ "$round_base" =~ ^[0-9]+$ ]] || continue
    if (( round_base > best )); then best=$round_base; round_dir=${candidate_dir%/}; fi
  done
  [[ -n "$round_dir" && -s "$round_dir/return.json" ]] || return 1
  "$ms" validate return-complete --root "$root" --role "$role" --file "$round_dir/return.json"     >/dev/null 2>&1 || return 1
  usage_arg=""
  [[ ! -s "$round_dir/usage.json" ]] || usage_arg="$round_dir/usage.json"
  patch=$(mktemp "$record_locks/recollect.XXXXXX")
  "$ms" adapter result-patch --output "$patch" --error null --phase supervision --usage "$usage_arg"
  "$ms" json set --file "$patch" --field "recollectedAt=$(now_iso)"     --field recollectedFrom=process-lost
  cas_out=$(record_cas "$job" "$status" completed "$patch" 2>/dev/null) && cas_rc=0 || cas_rc=$?
  rm -f "$patch"
  (( cas_rc == 0 )) || return 1
  reap_verdict_events "$job" completed recollected 0 "$cas_out"
  mirror_record "$job" || true
  return 0
}

reap_one() { # job
  local job=$1 result
  # An explicit reap has no next tick: skipping a busy lock silently would
  # return success to a caller whose job was never looked at, so the wait is
  # bounded and a timeout is a real failure.
  acquire_lifecycle_lock_until "$job" 5 || return 1
  set +e
  reap_one_locked "$job"
  result=$?
  set -e
  release_lifecycle_lock "$job"
  return "$result"
}

dispatch_job() {
  local role= brief= mode_override= runtime_override= model_override= job= reviews= workspace= permissions_override= mission_override= cap_override= serving_goal=0 stream= goal= steward_intent= steward_mode=0 steward_tuple=
  local use_worktree=0 workspace_selected=0 wait=0 approve_escalation=0 mode runtime model requested_model roster_runtime roster_model roster_pair requested_pair
  local overridden=false mission_data mission lease mission_turn canonical model_key cap_resolution tiers_present=false escalation_required=0
  local cost_direction= approval_name= approved_at= approved_ref= roster_json=
  local permission_name permission_json permission_digest tool_policy snapshot_json snapshot_path fallbacks signal handshake_budget resume_cap input_bytes input_hash payload round_dir record_json launch_mode goal_revision=0 goal_binding goal_machine= goal_claim_epoch= proposed_cap=0 reservation_claim_epoch=
  local occupancy_preparation claim_output claim_outcome claim_rc=0 launch_capability= cap operation_brief_hash prompt_temp composition_temp composition_output composition_rc=0 preflight_output preflight_outcome preflight_rc=0 replay_operation=0 destructive_reach= reasoning_effort=
  local -a product_root_args=() composition_source_args=()
  while (($#)); do
    case "$1" in
      --role) [[ $# -ge 2 ]] || { usage; exit 2; }; role=$2; shift 2 ;;
      --brief) [[ $# -ge 2 ]] || { usage; exit 2; }; brief=$2; shift 2 ;;
      --mode) [[ $# -ge 2 ]] || { usage; exit 2; }; mode_override=$2; shift 2 ;;
      --runtime) [[ $# -ge 2 ]] || { usage; exit 2; }; runtime_override=$2; shift 2 ;;
      --model) [[ $# -ge 2 ]] || { usage; exit 2; }; model_override=$2; shift 2 ;;
      --job-id) [[ $# -ge 2 ]] || { usage; exit 2; }; job=$2; shift 2 ;;
      --reviews) [[ $# -ge 2 ]] || { usage; exit 2; }; reviews=$2; shift 2 ;;
      --workspace) [[ $# -ge 2 ]] || { usage; exit 2; }; workspace=$2; workspace_selected=1; shift 2 ;;
      --worktree) use_worktree=1; shift ;;
      --permissions) [[ $# -ge 2 ]] || { usage; exit 2; }; permissions_override=$2; shift 2 ;;
      --mission) [[ $# -ge 2 ]] || { usage; exit 2; }; mission_override=$2; shift 2 ;;
      --stream) [[ $# -ge 2 ]] || { usage; exit 2; }; stream=$2; shift 2 ;;
      --goal) [[ $# -ge 2 ]] || { usage; exit 2; }; goal=$2; shift 2 ;;
      --destructive-reach) [[ $# -ge 2 ]] || { usage; exit 2; }; destructive_reach=$2; shift 2 ;;
      --cap-min) [[ $# -ge 2 ]] || { usage; exit 2; }; cap_override=$2; shift 2 ;;
      --approved-ref) [[ $# -ge 2 ]] || { usage; exit 2; }; approved_ref=$2; shift 2 ;;
      --source) [[ $# -ge 2 ]] || { usage; exit 2; }; composition_source_args+=(--source "$2"); shift 2 ;;
      --approve-escalation) approve_escalation=1; shift ;;
      --serving-goal) serving_goal=1; shift ;;
      --steward-intent) [[ $# -ge 2 ]] || { usage; exit 2; }; steward_intent=$2; shift 2 ;;
      --wait) wait=1; shift ;;
      *) usage; exit 2 ;;
    esac
  done
  if [[ -n "$steward_intent" ]]; then
    # The unattended continuation: every launch input comes from the
    # consumed authorization; nothing here is caller-selectable.
    [[ -z "$role$brief$runtime_override$model_override$job$permissions_override$mission_override$workspace$reviews$mode_override$stream$goal$cap_override$approved_ref" && $use_worktree -eq 0 && $serving_goal -eq 0 && $wait -eq 0 && $approve_escalation -eq 0 ]] \
      || die 2 "--steward-intent admits no other selection flags; the authorization decides"
    # An inherited mission scope would refuse or rescope the detached
    # continuation; the authorization is the whole context.
    unset "${!METASYSTEM_MISSION_@}"
    steward_tuple=$("$ms" steward authorize-dispatch --repo "$root" \
      --caller-pid "$entry_caller_pid" --intent "$steward_intent") \
      || die 1 "steward continuation refused: the authorization did not verify"
    role=$(json_value "$steward_tuple" role)
    brief=$(json_value "$steward_tuple" brief)
    job=$(json_value "$steward_tuple" jobId)
    runtime_override=$(json_value "$steward_tuple" runtime)
    model_override=$(json_value "$steward_tuple" model)
    permissions_override=$(json_value "$steward_tuple" permissions)
    use_worktree=1
    steward_mode=1
    destructive_reach=DESTRUCTIVE-REACH
  fi
  destructive_reach=${destructive_reach:-${METASYSTEM_DISPATCH_FIXTURE_HAZARD:-}}
  [[ -n "$role" && -f "$brief" && ( "$destructive_reach" == MECHANICAL || "$destructive_reach" == DESIGN-BEARING || "$destructive_reach" == DESTRUCTIVE-REACH ) ]] || { usage; exit 2; }
  [[ -z "$goal" ]] || valid_id "$goal" || die 2 "invalid goal id: $goal"
  [[ -f "$root/scripts/agents/roles/$role.md" && -f "$root/scripts/agents/roles/$role.requirements.json" ]] || die 1 "unknown dispatch role: $role"
  # Source admission is a pure preflight. A forbidden assertion refuses
  # before a runtime probe, worktree, branch, quarantine, lock, or job exists.
  if (( ${#composition_source_args[@]} )); then
    set +e
    composition_output=$("$ms" job compose-role-packet --validate-only --root "$root" --role "$role" \
      "${composition_source_args[@]}")
    composition_rc=$?
    set -e
    if (( composition_rc != 0 )); then
      [[ -z "$composition_output" ]] || { record_delegate_outcome_raw "$composition_output"; printf '%s\n' "$composition_output"; }
      return "$composition_rc"
    fi
  fi
  checkout_execution_guard_acquire "dispatch ${job:-$role}" \
    || die 1 "dispatch refused: checkout execution guard acquisition failed"
  trap 'checkout_execution_guard_release || true' EXIT
  if [[ -n "${METASYSTEM_CHECKOUT_EXECUTION_GUARD_FIXTURE:-}" ]]; then
    checkout_execution_guard_fixture_wait
    checkout_execution_guard_release
    trap - EXIT
    return 0
  fi
  if [[ "$role" == code-critic || "$role" == warden ]]; then
    [[ -n "$reviews" ]] || die 2 "$role dispatch requires --reviews <implementer-job-id>"
    valid_id "$reviews" || die 2 "invalid implementer job id for --reviews: $reviews"
    [[ -f "$jobs/$reviews.json" ]] || die 1 "$role dispatch cannot review unknown implementer job: $reviews"
    [[ "$(json_field "$jobs/$reviews.json" role 2>/dev/null || true)" == implementer ]] \
      || die 1 "$role dispatch --reviews must name an implementer job: $reviews"
  elif [[ "$role" == verifier && -n "$reviews" ]]; then
    valid_id "$reviews" || die 2 "invalid implementer job id for --reviews: $reviews"
    [[ -f "$jobs/$reviews.json" ]] || die 1 "verifier dispatch cannot review unknown implementer job: $reviews"
    [[ "$(json_field "$jobs/$reviews.json" role 2>/dev/null || true)" == implementer ]] \
      || die 1 "verifier dispatch --reviews must name an implementer job: $reviews"
  elif [[ -n "$reviews" ]]; then
    die 2 "--reviews is only valid for the code-critic, warden, and verifier roles"
  fi
  [[ ! ( $use_worktree -eq 1 && -n "$workspace" ) ]] || die 2 "--workspace and --worktree are mutually exclusive"
  if (( approve_escalation )) && { [[ ! -t 0 ]] || [[ ! -t 2 ]]; }; then
    die 1 "--approve-escalation requires an interactive TTY; remove the flag or re-run the same dispatch from a TTY"
  fi
  mode=$(brief_mode "$brief") || die 1 "brief must contain exactly one filled Working Mode header"
  [[ -z "$mode_override" || "$mode_override" == "$mode" ]] || die 1 "--mode contradicts the brief's Working Mode header"
  # --serving-goal resolves BEFORE any job state exists (goal-system
  # GOAL-08): the Go core reads the goal through the parser and a missing
  # usable Current goal refuses the whole dispatch, leaving nothing behind.
  goal_section=
  if [[ "$serving_goal" == 1 ]]; then
    goal_section=$("$ms" job serving-goal --root "$root") \
      || die 3 "no current goal to project (--serving-goal)"
  fi
  if [[ $steward_mode -eq 1 ]]; then
    # No lease exists to hold — the worker is provably dead; the
    # internal entries enforce the steward's one-job authority.
    current_caller_class=STEWARD
    current_claim_epoch=
    current_main_id=
  else
    lease_entry_check
  fi

  # The roster, tier, and escalation DECISIONS live in the engine
  # (script-orchestration-02); this block relays the verb's result and
  # keeps only the approval ladder below.
  roster_json=$("$ms" job resolve-roster --conf "$root/metasystem.conf" --role "$role" --mode "$mode" \
    ${runtime_override:+--runtime-override "$runtime_override"} \
    ${model_override:+--model-override "$model_override"}) || exit 1
  roster_runtime=$(json_value "$roster_json" rosterRuntime)
  roster_model=$(json_value "$roster_json" rosterModel)
  roster_pair=$(json_value "$roster_json" rosterPair)
  runtime=$(json_value "$roster_json" runtime)
  model=$(json_value "$roster_json" model)
  requested_pair=$(json_value "$roster_json" requestedPair)
  requested_model=$model
  overridden=$(json_value "$roster_json" overridden)
  tiers_present=$(json_value "$roster_json" tiersPresent)
  cost_direction=$(json_value "$roster_json" costDirection)
  escalation_required=0
  [[ "$(json_value "$roster_json" escalationRequired)" == true ]] && escalation_required=1

  mission_data=$(resolve_mission "$mission_override")
  IFS='|' read -r mission lease mission_turn <<<"$mission_data"
  # Mission provenance is COMPLETE or refused (host-implementer wall): a
  # mission-scoped dispatch binds the turn it runs in and the stream it
  # serves, because the wall's authorizations bind that whole tuple. A
  # stream without a mission is meaningless.
  if [[ -n "$stream" ]]; then
    [[ -n "$mission" ]] || die 2 "--stream requires a mission context"
    valid_id "$stream" || die 2 "invalid mission stream id: $stream"
  fi
  if [[ -n "$mission" ]]; then
    [[ -n "$mission_turn" ]] || die 2 "mission dispatch requires a runner turn (METASYSTEM_MISSION_TURN is not set); dispatch from inside the mission host turn"
    [[ -n "$stream" ]] || die 2 "mission dispatch requires --stream <mission-stream-id> naming the stream this job serves"
  fi
  if (( escalation_required )); then
    if (( approve_escalation )); then
      approval_name=$(confirm_escalation "$roster_pair" "$requested_pair" "$cost_direction")
      approved_at=$(now_iso)
    elif [[ -n "$mission" ]] && signed_dispatch_envelope_allows "$mission" "$requested_pair"; then
      :
    elif [[ "$tiers_present" == false ]]; then
      die 1 "dispatch escalation refused: roster resolves to $roster_pair, requested pair is $requested_pair, and model tiers are absent. Configure model.tier.* to rank both pairs, add $requested_pair to a signed envelope.dispatch-allow mission contract, or re-run from a TTY with --approve-escalation."
    else
      die 1 "dispatch escalation refused: roster resolves to $roster_pair, requested pair is $requested_pair, cost direction is $cost_direction. Remove the override to use $roster_pair, add $requested_pair to a signed envelope.dispatch-allow mission contract, or re-run from a TTY with --approve-escalation."
    fi
  elif (( approve_escalation )); then
    die 1 "--approve-escalation is unnecessary because the requested pair does not require escalation approval; remove the flag"
  fi

  permission_name=${permissions_override:-$(config_get --key "dispatch.permissions.$role" --default none)}
  if [[ "$role" == warden ]]; then
    # The warden holds the no-pen seat: its write authority is bound by
    # the role, never by a caller flag, a config key, or a file that
    # shadows a preset name — the shipped zero-write preset is forced
    # by its absolute repository path.
    [[ -z "$permissions_override" ]] || die 2 "the warden role dispatches with the zero-write preset; --permissions cannot change it"
    permission_name="$root/scripts/agents/permissions/none.json"
  fi
  if (( use_worktree == 0 && workspace_selected == 0 )) \
      && permission_envelope_requests_writes "$permission_name"; then
    use_worktree=1
  fi

  operation_brief_hash=$(sha256_file "$brief")
  if [[ -n "$goal" ]]; then
    goal_binding=$("$ms" job goal-binding --root "$root" --goal "$goal") \
      || die 1 "cannot bind delegate operation to accepted goal $goal stop authority"
    goal_revision=$(json_value "$goal_binding" goalRevision)
    goal_machine=$(json_value "$goal_binding" machineId)
    goal_claim_epoch=$(json_value "$goal_binding" claimEpoch)
    [[ -z "$current_claim_epoch" || "$current_claim_epoch" == "$goal_claim_epoch" ]] \
      || die 1 "goal $goal revision $goal_revision belongs to claim epoch $goal_claim_epoch, not current epoch $current_claim_epoch"
  fi
  if [[ -z "$job" ]]; then
    job=$("$ms" job operation-id --goal "$goal" --goal-revision "$goal_revision" \
      --dispatch-mode fresh --role "$role" --brief-digest "$operation_brief_hash") \
      || die 1 "could not derive the delegate operation identity"
  fi
  valid_id "$job" || die 2 "invalid job id: $job"
  # A new operation has no retry fingerprint to compare. Run the global
  # budget gate before process setup; standing operations defer it until the
  # exact v2 preflight has established replay versus mismatch.
  if [[ ! -e "$jobs/$job.json" ]]; then
    require_goal_admission
  fi
  # Preconditions before the id is reserved keep a refused launch from leaving
  # a pending-setup husk that consumes the caller's chosen name.
  require_fresh_census
  report_plan_drift
  reservation_claim_epoch=${goal_claim_epoch:-$current_claim_epoch}
  mkdir -p "$jobs" "$record_locks" "$capabilities" "$worktrees"
  acquire_launch_chain_lock "$job"
  exit_cleanup_job=$job
  exit_cleanup_chain=$job
  exit_cleanup_authorization=
  trap 'code=$?; if (( code != 0 )); then fail_setup_husk "$exit_cleanup_job"; release_unpublished_authorization "$exit_cleanup_authorization"; fi; release_cap_authority_lock; release_exit_lifecycle; release_goal_revision_lock; release_chain_lock "$exit_cleanup_chain"; checkout_execution_guard_release || true' EXIT
  # A payload without a reservation belongs to another operation. A payload
  # beside a reservation is resolved by claim-launch as the same operation.
  [[ -e "$jobs/$job.json" || ! -e "$agents/$job" ]] \
    || die 1 "job payload collision: $job"
  if [[ -n "$goal" ]]; then
    acquire_goal_revision_lock "$goal" "$goal_revision"
  fi

  if (( use_worktree )); then
    launch_mode=worktree
    workspace="$worktrees/$job"
    if [[ ! -e "$jobs/$job.json" ]]; then
      [[ ! -e "$workspace" ]] || die 1 "job worktree already exists: $workspace"
      git -C "$repo_scope" worktree add -q -b "agent/$job" "$workspace" HEAD || die 1 "could not create job worktree"
    # QUARANTINE OBJECT STORE (issue #5): the delegate's git writes its
    # loose objects into a private directory INSIDE the worktree (the
    # sandbox already grants it) and reads the shared store through the
    # alternates mechanism — the shared objects/ stays read-only to the
    # delegate, so a hostile job cannot delete or corrupt the repository's
    # object database. The engine links the quarantine into the shared
    # store's alternates so conformance and merge can read the delegate's
    # commits. This mirrors git's own receive-pack quarantine.
    # The quarantine lives in the worktree's PRIVATE GIT DIR — already a
    # delegate write root, and OUTSIDE the shippable projection, so the
    # conformance snapshot's add -A can never sweep loose objects into
    # the review (round-2 F1: at the worktree root they appeared
    # untracked and polluted the tree).
      worktree_gitdir=$(git -C "$workspace" rev-parse --absolute-git-dir) \
        || die 1 "could not resolve the worktree git dir"
      quarantine="$worktree_gitdir/objects-quarantine"
      mkdir -p "$quarantine" || die 1 "could not create the quarantine object store"
      common_objects=$(git -C "$repo_scope" rev-parse --git-common-dir)/objects
      [[ "$common_objects" = /* ]] || common_objects="$repo_scope/$common_objects"
      mkdir -p "$common_objects/info"
      grep -qxF "$quarantine" "$common_objects/info/alternates" 2>/dev/null \
        || echo "$quarantine" >>"$common_objects/info/alternates" \
        || die 1 "could not link the quarantine into the shared object store"
    fi
  else
    launch_mode=shared-checkout
    workspace=${workspace:-$repo_scope}
    workspace=$(cd "$workspace" && pwd -P) || die 1 "workspace does not exist: $workspace"
  fi
  if is_review_role "$role" && (( use_worktree == 0 )) && [[ "$workspace" == "$repo_scope" ]] \
      && permission_envelope_requests_writes "$permission_name"; then
    die 2 "$role live-checkout write refusal (incident class: critic-workspace-custody): a review role could modify product bytes in the coordinator's tree; pass --worktree to keep its writes quarantined"
  fi
  permission_json=$(mktemp "$record_locks/permissions.XXXXXX")
  expand_permissions "$permission_name" "$workspace" "$use_worktree" "$permission_json"
  permission_digest=$(sha256_file "$permission_json")
  tool_policy=$(json_field "$permission_json" tools)
  snapshot_json=$(mktemp "$record_locks/snapshot.XXXXXX")
  select_snapshot "$runtime" "$role" "$permission_json" "$snapshot_json"
  read_snapshot_fields "$snapshot_json"

  # The resolved section joins the brief BEFORE the hash (goal-system
  # GOAL-08): it is part of the recorded bytes and survives every
  # fallback rebuild.
  if [[ -n "$goal_section" ]]; then
    local brief_with_goal
    brief_with_goal=$(mktemp "$record_locks/brief.XXXXXX")
    cat "$brief" > "$brief_with_goal"
    printf '\n%s' "$goal_section" >> "$brief_with_goal"
    brief=$brief_with_goal
  fi

  payload="$agents/$job"; round_dir="$payload/rounds/1"
  prompt_temp=$(mktemp "$record_locks/composed-packet.XXXXXX")
  composition_temp=$(mktemp "$record_locks/composition.XXXXXX")
  set +e
  composition_output=$("$ms" job compose-role-packet --root "$root" --role "$role" --brief "$brief" \
    --job "$job" --runtime "$runtime" --model "$model" --tool-policy "$tool_policy" --round 1 --mission "$mission" \
    --destructive-reach "$destructive_reach" \
    --output "$prompt_temp" --composition "$composition_temp" \
    "${composition_source_args[@]+"${composition_source_args[@]}"}")
  composition_rc=$?
  set -e
  if (( composition_rc != 0 )); then
    [[ -z "$composition_output" ]] || { record_delegate_outcome_raw "$composition_output"; printf '%s\n' "$composition_output"; }
    return "$composition_rc"
  fi
  input_bytes=$(enforce_inline_input_limit "$prompt_temp" brief)
  input_hash=$(sha256_file "$prompt_temp")

  local output_stream
  output_stream=$("$root/scripts/agents/adapters/$runtime.sh" output-stream --round-dir "$round_dir") \
    || die 1 "$runtime adapter could not resolve its child output stream"

  product_root_args=(--product-root "$workspace")
  acquire_cap_authority_lock
  cap_resolution=$(mktemp "$record_locks/cap-resolution.XXXXXX")
  model_key=$(canonical_model "$model")
  [[ -n "$model_key" ]] || die 1 "requested model has no canonical cap-key form"
  if [[ -e "$jobs/$job.json" && -n "$mission" ]]; then
    cap=${cap_override:-$(json_field "$jobs/$job.json" capMin)}
    "$ms" job cap-resolution --cap "$cap" --rule repeated-operation \
      --origin existing-reservation --output "$cap_resolution"
  else
    exit_cleanup_authorization=$job
    authorize_job_cap "$job" "$role" "$runtime" "$model_key" "$mission" "$cap_override" dispatch "$cap_resolution"
  fi
  cap=$(json_field "$cap_resolution" capMin)
  set +e
  preflight_output=$("$ms" job claim-launch --preflight --root "$root" --opid "$job" \
    --operation-id "$job" \
    --session "$runtime:$job" --dispatch-mode fresh --resumed-session "" \
    --runtime "$runtime" --model "$model" --role "$role" \
    --reviews "$reviews" \
    --launch-mode "$launch_mode" --permission-envelope-digest "$permission_digest" \
    "${product_root_args[@]+"${product_root_args[@]}"}" \
    --cap-min "$cap" --conf "$root/metasystem.conf" --input-hash "$input_hash" \
    --goal "$goal" --goal-revision "$goal_revision" \
    --destructive-reach "$destructive_reach" --adapter-verb dispatch)
  preflight_rc=$?
  set -e
  preflight_outcome=$(json_value "$preflight_output" outcome 2>/dev/null || true)
  if (( preflight_rc != 0 )); then
    [[ -z "$preflight_output" ]] || { record_delegate_outcome_raw "$preflight_output"; printf '%s\n' "$preflight_output"; }
    return "$preflight_rc"
  fi
  [[ "$preflight_outcome" != PREFLIGHT-MATCHED ]] || replay_operation=1
  if (( replay_operation == 0 )); then
    require_goal_admission
    require_goal_revision_admission "$cap"
    require_slice_admission "$cap" "$approved_ref" "$goal" "$goal_revision"
  fi
  if ! acquire_lifecycle_lock_until "$job" 5; then
    record_delegate_outcome LOCK_BUSY refused "rank=job-lifecycle key=$job retry=retry-after-the-named-holder-releases" "$job"
    die 1 "LOCK_BUSY rank=job-lifecycle key=$job retry=retry-after-the-named-holder-releases"
  fi
  exit_cleanup_lifecycle=$job
  occupancy_preparation=$(mktemp "$record_locks/claim-occupancy.XXXXXX")
  "$ms" job claim-occupancy-prepare --root "$root" --session "$runtime:$job" \
    --output "$occupancy_preparation"
  claim_output=$("$ms" job claim-launch --root "$root" --opid "$job" \
    --operation-id "$job" \
    --session "$runtime:$job" --dispatch-mode fresh --resumed-session "" \
    --runtime "$runtime" --model "$model" --role "$role" \
    --reviews "$reviews" \
    --launch-mode "$launch_mode" --permission-envelope-digest "$permission_digest" \
    "${product_root_args[@]+"${product_root_args[@]}"}" \
    --cap-min "$cap" --conf "$root/metasystem.conf" --input-hash "$input_hash" \
    --main-id "$current_main_id" --claim-epoch "$reservation_claim_epoch" --goal "$goal" \
    --goal-revision "$goal_revision" --machine-id "$goal_machine" --approved-ref "$approved_ref" \
    --destructive-reach "$destructive_reach" --adapter-verb dispatch \
    --creator-pid "$$" --occupancy-preparation "$occupancy_preparation") || claim_rc=$?
  rm -f "$occupancy_preparation"
  claim_outcome=$(json_value "$claim_output" outcome 2>/dev/null || true)
  [[ -n "$claim_outcome" ]] \
    || { printf '%s\n' "$claim_output" >&2; return 1; }
  if [[ "$claim_outcome" != WON ]]; then
    record_delegate_outcome_raw "$claim_output"
    [[ ! -f "$jobs/$job.json" ]] || exit_cleanup_job=
    printf '%s\n' "$claim_output"
    return "$claim_rc"
  fi
  (( claim_rc == 0 )) || return "$claim_rc"
  launch_capability=$(json_value "$claim_output" evidence.launchCapability)
  [[ -n "$launch_capability" ]] || die 1 "claim-launch won without an adapter launch capability"
  exit_cleanup_authorization=
  release_cap_authority_lock

  mkdir -p "$round_dir"
  cp "$brief" "$payload/brief.md"
  mv "$prompt_temp" "$round_dir/prompt.md"
  mv "$composition_temp" "$round_dir/composition.json"

  record_json=$(mktemp "$record_locks/record.XXXXXX")
  reasoning_effort=$(json_field "$round_dir/composition.json" configurationObligations.builderReasoningEffort)
  "$ms" job build-record --output "$record_json" --job "$job" --role "$role" \
    --mission "$mission" --mission-turn "$mission_turn" --stream "$stream" \
    --root "$root" --runtime "$runtime" \
    --workspace "$workspace" --cap-resolution "$cap_resolution" --model "$model" \
    --overridden "$overridden" --snapshot "$snapshot_path" \
    --input-bytes "$input_bytes" --input-hash "$input_hash" \
    --permissions "$permission_json" --fallbacks "$fallbacks" --signal "$signal" \
    --handshake-budget "$handshake_budget" --approval-name "$approval_name" \
    --approved-at "$approved_at" --roster-pair "$roster_pair" \
    --requested-pair "$requested_pair" --cost-direction "$cost_direction" \
    --reviews "$reviews" --goal "$goal" --goal-revision "$goal_revision" \
    --machine-id "$goal_machine" --approved-ref "$approved_ref" \
    --destructive-reach "$destructive_reach" --reasoning-effort "$reasoning_effort" \
    --main-id "$current_main_id" --claim-epoch "$reservation_claim_epoch" \
    --composition "$round_dir/composition.json" \
    --launch-mode "$launch_mode" \
    "${product_root_args[@]+"${product_root_args[@]}"}" \
    --output-stream "$output_stream"
  rm -f "$cap_resolution"
  finalize_and_launch "$job" "$job" "$record_json" "$runtime" dispatch "$handshake_budget" "$wait" "$launch_capability"
}

# The authorize-and-launch tail, shared by dispatch_job and follow_up
# (script-orchestration-13): the two ~70-line copies had already drifted —
# follow_up fed dispatch.max-inline-input-kb straight into arithmetic, so a
# malformed value died as a bash arithmetic error instead of the intended
# refusal. One copy now; the drift resolved toward the validating side.

authorize_job_cap() { # job id, role, runtime, model key, mission, explicit override, refusal noun, output json
  local job=$1 role=$2 runtime=$3 model_key=$4 mission=$5 override=$6 noun=$7 output=$8 cap watch_cap cap_result cap_args
  if [[ -n "$mission" ]]; then
    refuse_unsigned_mission_cap_override "$role" "$runtime" "$model_key"
    cap_args=(authorize-cap --repo "$root" --mission "$mission" --job "$job" --runtime "$runtime" --model "$model_key")
    [[ -z "$override" ]] || cap_args+=(--requested "$override")
    if ! cap_result=$(mission_fence "${cap_args[@]}" 2>&1); then
      die 1 "mission $noun refused by the mission fence: $cap_result"
    fi
    printf '%s\n' "$cap_result" >"$output"
  else
    resolve_nonmission_cap "$role" "$runtime" "$model_key" "$override" "$output"
  fi
  cap=$(json_field "$output" capMin)
  [[ "$cap" =~ ^[1-9][0-9]*$ ]] || die 1 "dispatch cap authority returned an invalid capMin"
  watch_cap=$(attested_watcher_ceiling)
  (( cap < watch_cap )) \
    || die 1 "dispatch cap ${cap}m must stay below the live watcher's attested ${watch_cap}m ceiling; re-arm supervision with --rearm --max-cap $cap"
}

enforce_inline_input_limit() { # content file, refusal hint (brief|message); prints the byte count
  local content=$1 hint=$2 max_kb bytes
  max_kb=$(config_get --key dispatch.max-inline-input-kb --default 64)
  [[ "$max_kb" =~ ^[1-9][0-9]*$ ]] || die 1 "dispatch.max-inline-input-kb must be a positive integer"
  bytes=$(wc -c <"$content" | tr -d ' ')
  (( bytes <= max_kb * 1024 )) || die 1 "inline input exceeds dispatch.max-inline-input-kb; pass a file reference in the $hint"
  printf '%s\n' "$bytes"
}

read_snapshot_fields() { # snapshot json — sets snapshot_path, fallbacks, signal, handshake_budget, resume_cap
  snapshot_path=$(json_field "$1" path)
  fallbacks=$(json_field "$1" fallbacks)
  signal=$(json_field "$1" sessionEstablishedSignal)
  handshake_budget=$(json_field "$1" sessionEstablishedTimeoutSec)
  resume_cap=$(json_field "$1" resume 2>/dev/null || true)
}

finalize_and_launch() { # job id, chain id, record json, runtime, adapter verb, handshake budget, wait flag, capability
  local job=$1 chain=$2 record_json=$3 runtime=$4 adapter_verb=$5 budget=$6 wait_flag=$7 launch_capability=$8 patch launch_rc=0 tag
  lease_run_held "$current_claim_epoch" "$0" __record-setup --job "$job" --source "$record_json"
  release_cap_authority_lock
  # Launch returns only after the adapter has published the exact process
  # identity. The chain and goal locks therefore cover the final fence read,
  # reservation, spawn, and identity publication as one ranked interval.
  tag=$(json_field "$jobs/$job.json" instanceTag)
  [[ -n "$tag" && "$tag" != null ]] || die 1 "job $job record carries no reservation instance tag"
  lease_run_held "$current_claim_epoch" "$0" __launch --runtime "$runtime" --verb "$adapter_verb" \
    --job "$job" --tag "$tag" --launch-capability "$launch_capability" || launch_rc=$?
  release_exit_lifecycle
  release_goal_revision_lock
  release_chain_lock "$chain"
  exit_cleanup_chain=
  trap 'checkout_execution_guard_release || true' EXIT
  if (( launch_rc != 0 )); then
    record_delegate_outcome LAUNCH-FAILED refused "the adapter launch failed after reservation" "$job"
    patch=$(mktemp "$record_locks/launch-failed.XXXXXX"); printf '{"error":"launch_failed"}\n' >"$patch"
    lease_run_held "$current_claim_epoch" "$0" __record-cas --job "$job" --expect pending --status failed --patch "$patch" || true
    rm -f "$patch"; return 3
  fi
  if ! await_handshake "$job" "$budget" "$current_claim_epoch"; then
    record_delegate_outcome HANDSHAKE-FAILED refused "the runtime did not establish its session within the recorded deadline" "$job"
    return 3
  fi
  if (( wait_flag )); then wait_for_job "$job"; return $?; fi
  printf '%s\n' "$job"
  # The exact waiter command (monitor facility, MON-04; backlog item 1
  # as the human wrote it): the agent never invents a polling loop.
  printf 'watch it with: scripts/agents/dispatch.sh watch --job %s\n' "$job" >&2
}

# Critique rounds use the same proven record, adapter, detached supervisor,
# custody registration, and terminal writer as normal delegate jobs. This
# narrow entry only supplies the critique driver's stable session key and
# omits the regular dispatch command's roster and review-chain policy.
watch_job() { # --job <id>
  local job= watched_root
  while (( $# )); do
    case "$1" in
      --job) [[ $# -ge 2 ]] || { usage; exit 2; }; job=$2; shift 2 ;;
      *) usage; exit 2 ;;
    esac
  done
  [[ -n "$job" ]] || { usage; exit 2; }
  # A job with no record is knowable NOW: watching it to the timeout
  # would conflate "never existed" with "still silent" — the refusal
  # answers fast and says so (vanished, exit 5).
  if [[ ! -e "$jobs/$job.json" ]]; then
    echo "watch: no job record for $job — it never existed here or was reaped" >&2
    exit 5
  fi
  watched_root=$(json_field "$jobs/$job.json" workspaceRoot 2>/dev/null || true)
  [[ -n "$watched_root" && "$watched_root" != null ]] || watched_root=$root
  # The Go core blocks to terminal and holds the waiter record the turn
  # verdict reads; the printer only tails the watched workspace's suite
  # journal, and exit codes ride through verbatim.
  "$ms" job watch --root "$root" --job "$job" --caller-pid $$ \
    --progress-root "$watched_root"
}

cleanup_follow_up_message() {
  [[ -z "$exit_cleanup_message" ]] || rm -f -- "$exit_cleanup_message"
  exit_cleanup_message=
}

append_critique_open_ids() { # source message, output message, critic root
  local source=$1 output=$2 critic_root=$3 open_ids
  open_ids=$("$ms" job critique-open-finding-ids --repo "$root" --root-job "$critic_root") \
    || die 1 "could not read the critic chain's canonical open finding identifiers"
  cat "$source" >"$output"
  printf '\n\n# Canonical critique register carry\n\nOpen finding identifiers:\n' >>"$output"
  if [[ -z "$open_ids" ]]; then
    printf '%s\n' '- none' >>"$output"
  else
    while IFS= read -r finding_id; do
      [[ -z "$finding_id" ]] || printf -- '- %s\n' "$finding_id"
    done <<<"$open_ids" >>"$output"
  fi
}

follow_up() {
  local job= message= wait=0 root_id latest status error session role runtime model model_key workspace reviewed_commit round child payload round_dir cap_resolution permission_json permission_digest tool_policy snapshot_json snapshot_path fallbacks signal handshake_budget resume_cap record_json mission mission_data lease mission_turn goal reviews=
  local resume_mode=resumed adapter_verb=follow-up delivery_content parent_round launch_mode goal_revision=0 goal_binding goal_machine= goal_claim_epoch= proposed_cap=0 reservation_claim_epoch= approved_ref= operation_override= operation_id operation_parent operation_brief_hash standing_child_record= destructive_reach=
  local occupancy_preparation claim_output claim_outcome claim_rc=0 launch_capability= cap resumed_for_claim input_bytes input_hash prompt_temp composition_temp composition_output composition_rc=0 preflight_output preflight_outcome preflight_rc=0 replay_operation=0
  local repeated_follow_up=0 parent_job fresh_context_temp=
  local -a product_root_args=() continuation_args=()
  while (($#)); do
    case "$1" in
      --job) [[ $# -ge 2 ]] || { usage; exit 2; }; job=$2; shift 2 ;;
      --message) [[ $# -ge 2 ]] || { usage; exit 2; }; message=$2; shift 2 ;;
	  --operation-id) [[ $# -ge 2 ]] || { usage; exit 2; }; operation_override=$2; shift 2 ;;
      --approved-ref) [[ $# -ge 2 ]] || { usage; exit 2; }; approved_ref=$2; shift 2 ;;
      --wait) wait=1; shift ;;
      *) usage; exit 2 ;;
    esac
  done
  valid_id "$job" && [[ -f "$message" && -f "$jobs/$job.json" ]] || { usage; exit 2; }
  operation_brief_hash=$(sha256_file "$message")
  lease_entry_check
  require_fresh_census
  report_plan_drift
  root_id=$(root_job_id "$job") || die 1 "cannot resolve the job chain"
  acquire_launch_chain_lock "$root_id"
  # The trap must reference a GLOBAL: when set -e unwinds the owning
  # function, bash 5.x has already popped its locals when the EXIT trap
  # runs, and under set -u the trap dies on the expansion before releasing
  # anything (the Linux cap-authority leak, go-production-grade Phase 1).
  exit_cleanup_chain=$root_id
  trap 'cleanup_follow_up_message; release_goal_revision_lock; release_chain_lock "$exit_cleanup_chain"' EXIT
  # A worktree chain reads its own branch, not main: a follow-up citing files
  # amended on main after the branch point describes files the delegate does
  # not have. This lesson (KI-9's complement) was violated three times as
  # prose before becoming this check.
  if worktree_path=$(json_field "$jobs/$root_id.json" workspaceRoot 2>/dev/null) \
      && [[ -n "$worktree_path" && "$worktree_path" != null && -d "$worktree_path" ]]; then
    trunk=$(git -C "$root" branch --show-current 2>/dev/null || true)
    behind=0
    [[ -n "$trunk" ]] && behind=$(git -C "$worktree_path" rev-list --count "HEAD..$trunk" 2>/dev/null || echo 0)
    if (( behind > 0 )); then
      echo "WORKTREE-BEHIND: the chain worktree is $behind commit(s) behind main; if this follow-up cites amended files, merge main into $worktree_path first" >&2
    fi
  fi
  [[ "$(json_field "$jobs/$root_id.json" chainClosed 2>/dev/null || true)" != true ]] || die 1 "job chain is closed"
  latest=$(latest_chain_record "$root_id") || die 1 "cannot find the newest chain record"
  status=$(json_field "$latest" status); error=$(json_field "$latest" error 2>/dev/null || true)
  if [[ "$status" == completed || ( "$status" == failed && "$error" == protocol_error ) ]]; then
    round=$(( $(json_field "$latest" round) + 1 )); child="$root_id-r$round"
    [[ ! -e "$jobs/$child.json" ]] || die 1 "follow-up job id collision: $child"
  elif [[ "$status" == pending-setup || "$status" == pending || "$status" == running ]] \
      && [[ "$(json_field "$latest" dispatchMode 2>/dev/null || true)" == follow-up ]]; then
    repeated_follow_up=1
    child=$(json_field "$latest" jobId)
	standing_child_record="$jobs/$child.json"
    round=$(json_field "$latest" round 2>/dev/null || true)
    if [[ ! "$round" =~ ^([2-9]|[1-9][0-9]+)$ ]]; then
      round=${child#"$root_id-r"}
    fi
    [[ "$round" =~ ^([2-9]|[1-9][0-9]+)$ && "$child" == "$root_id-r$round" ]] \
      || die 1 "the active follow-up has no valid round identity"
    parent_job=$(json_field "$latest" parentJob 2>/dev/null || true)
    if [[ -z "$parent_job" || "$parent_job" == null ]]; then
      if (( round == 2 )); then
        parent_job=$root_id
      else
        parent_job="$root_id-r$((round - 1))"
      fi
    fi
    valid_id "$parent_job" && [[ -f "$jobs/$parent_job.json" ]] \
      || die 1 "the active follow-up has no readable parent record"
    latest="$jobs/$parent_job.json"
  else
    die 1 "follow-up requires the newest record to be completed or failed with protocol_error; use a fresh dispatch after pending, running, timeout, or process-lost"
  fi
  operation_parent=$(basename "${latest%.json}")
  session=$(json_field "$latest" sessionId 2>/dev/null || true)
  [[ -n "$session" && "$session" != null ]] || die 1 "follow-up has no resumable session id; use the fresh-context embed fallback"
  # The recorded parent session is part of every follow-up's identity. A
  # runtime without native resume still derives its embedded context from that
  # session, even though the adapter opens a new session for the child.
  resumed_for_claim=$session
  role=$(json_field "$latest" role); runtime=$(json_field "$latest" runtime); model=$(json_field "$latest" requestedModel)
  reviews=$(json_field "$latest" reviews 2>/dev/null || true); [[ "$reviews" == null ]] && reviews=
  destructive_reach=$(json_field "$latest" destructiveReach 2>/dev/null || true)
  [[ "$destructive_reach" == MECHANICAL || "$destructive_reach" == DESIGN-BEARING || "$destructive_reach" == DESTRUCTIVE-REACH ]] \
    || die 1 "follow-up chain has no admitted destructiveReach class; start a fresh typed delegate chain"
  if [[ -z "$approved_ref" ]]; then
    approved_ref=$(json_field "$latest" approvedRef 2>/dev/null || true); [[ "$approved_ref" == null ]] && approved_ref=
  fi
  goal=$(json_field "$latest" goalId 2>/dev/null || true); [[ "$goal" == null ]] && goal=
  workspace=$(json_field "$latest" workspaceRoot)
  round=$(( $(json_field "$latest" round) + 1 )); child="$root_id-r$round"
  if [[ -n "$goal" ]]; then
    goal_binding=$("$ms" job goal-binding --root "$root" --goal "$goal") \
      || die 1 "cannot bind follow-up $child to accepted goal $goal stop authority"
    goal_revision=$(json_value "$goal_binding" goalRevision)
    goal_machine=$(json_value "$goal_binding" machineId)
    goal_claim_epoch=$(json_value "$goal_binding" claimEpoch)
    [[ -z "$current_claim_epoch" || "$current_claim_epoch" == "$goal_claim_epoch" ]] \
      || die 1 "goal $goal revision $goal_revision belongs to claim epoch $goal_claim_epoch, not current epoch $current_claim_epoch"
    acquire_goal_revision_lock "$goal" "$goal_revision"
  fi
	if [[ -n "$operation_override" ]]; then
	  operation_id=$operation_override
	else
	  operation_id=$("$ms" job operation-id --goal "$goal" --goal-revision "$goal_revision" \
	    --dispatch-mode follow-up --role "$role" --brief-digest "$operation_brief_hash" --parent-job "$operation_parent") \
	    || die 1 "could not derive the follow-up operation identity"
	fi
	valid_id "$operation_id" || die 2 "invalid follow-up operation id: $operation_id"
	if (( repeated_follow_up )) && [[ "$(json_field "$standing_child_record" operationId 2>/dev/null || true)" != "$operation_id" ]]; then
	  record_delegate_outcome REFUSED-OPID-MISMATCH refused "the active follow-up is bound to another v2 operation identity" "$child"
	  return 1
	fi
  reservation_claim_epoch=${goal_claim_epoch:-$current_claim_epoch}
  launch_mode=$(json_field "$latest" launchMode 2>/dev/null || true)
  if [[ "$launch_mode" != worktree && "$launch_mode" != shared-checkout ]]; then
    case "$workspace/" in
      "$worktrees/"*) launch_mode=worktree ;;
      *) launch_mode=shared-checkout ;;
    esac
  fi
  # Mission provenance refuses BEFORE the child reservation exists (round-4
  # critique R4-F1: a refusal after record-create strands the -rN husk and a
  # retry collides). The verb's own stderr names the exact refusal — a
  # pre-wall chain and a re-provisioned mission are different errors.
  mission=$(json_field "$latest" mission 2>/dev/null || true); [[ "$mission" == null ]] && mission=
  mission_turn=
  if [[ -n "$mission" ]]; then
    mission_data=$(resolve_mission "$mission")
    IFS='|' read -r mission lease mission_turn <<<"$mission_data"
    incarnation_verdict=$("$ms" job verify-chain-incarnation --root "$root" --mission "$mission" --parent "$latest" 2>&1) \
      || die 1 "${incarnation_verdict:-mission chain incarnation check failed}"
    [[ -n "$mission_turn" ]] || die 2 "mission follow-up requires a runner turn (METASYSTEM_MISSION_TURN is not set); follow up from inside the mission host turn"
  fi
  if [[ "$role" == design-critic ]]; then
    # A critic's workspace is one of two different things, and treating them
    # alike broke the second. A WORKTREE of this repository is synchronised to
    # this repository's HEAD, because the design under review moved. A workspace
    # that is its OWN repository -- a benchmark target, a scratch checkout -- has
    # its own history, and this repository's HEAD is not a commit it has ever
    # heard of: merging it produced "not something we can merge". The test that
    # told them apart was "is the path different", which every separate
    # repository also satisfies. The test is now shared history.
    local workspace_git harness_git
    workspace_git=$( (cd "$workspace" && git rev-parse --git-common-dir 2>/dev/null) || true)
    harness_git=$( (cd "$repo_scope" && git rev-parse --git-common-dir 2>/dev/null) || true)
    [[ -n "$workspace_git" ]] && workspace_git=$( (cd "$workspace" && cd "$workspace_git" && pwd -P) 2>/dev/null || true)
    [[ -n "$harness_git" ]] && harness_git=$( (cd "$repo_scope" && cd "$harness_git" && pwd -P) 2>/dev/null || true)
    if [[ -n "$workspace_git" && "$workspace_git" == "$harness_git" ]]; then
      reviewed_commit=$(git -C "$repo_scope" rev-parse HEAD) \
        || die 1 "design-critic follow-up cannot resolve the current commit"
      if [[ "$(cd "$workspace" && pwd -P)" != "$repo_scope" ]]; then
        [[ -z "$(git -C "$workspace" status --porcelain)" ]] \
          || die 1 "design-critic follow-up cannot synchronize a dirty critic worktree"
        git -C "$workspace" merge --ff-only -q "$reviewed_commit" \
          || die 1 "design-critic follow-up cannot fast-forward its worktree to current commit $reviewed_commit"
      fi
    else
      # An independent repository reviews its own head, and nothing is merged
      # into it: this dispatcher does not own its history.
      reviewed_commit=$(git -C "$workspace" rev-parse HEAD) \
        || die 1 "design-critic follow-up cannot resolve the workspace commit"
    fi
  fi
  # The completed critic attempt is folded and the cap is checked while no
  # successor record exists. A terminal human raise therefore cannot strand a
  # pending-setup husk, and retrying the same round id remains possible.
  # A repeated wrapper claims a round the winner already folded and advanced;
  # folding it again would refuse on the successor's own record. The carry
  # appendix is a pure register read, so the repeated wrapper still appends
  # it - reconstructing the winner's exact delivered message, which is what
  # the claim fingerprint's input hash must equal.
  if [[ "$role" == design-critic || "$role" == code-critic || "$role" == warden ]]; then
    if (( repeated_follow_up == 0 )); then
      register_outcome=$(lease_run_held "$current_claim_epoch" "$0" __critique-register-advance \
        --root-job "$root_id" --round-job "$(basename "${latest%.json}")") \
        || die 1 "could not fold the latest critic attempt into its canonical register"
    fi
    exit_cleanup_message=$(mktemp "${TMPDIR:-/tmp}/metasystem-critique-follow.XXXXXX")
    append_critique_open_ids "$message" "$exit_cleanup_message" "$root_id"
    message=$exit_cleanup_message
  fi
  if (( repeated_follow_up == 0 )) \
      && [[ "$role" == implementer || "$role" == design-critic || "$role" == code-critic || "$role" == warden ]]; then
    set +e
    exhaustion_outcome=$(lease_run_held "$current_claim_epoch" "$0" __critique-exhaustion-advance \
      --root-job "$root_id" --role "$role" --message "$message" --successor "$child" 2>&1)
    exhaustion_rc=$?
    set -e
    (( exhaustion_rc == 0 )) || die "$exhaustion_rc" "$exhaustion_outcome"
  fi
  mkdir -p "$record_locks"
  exit_cleanup_job=$child
  exit_cleanup_chain=$root_id
  exit_cleanup_authorization=
  trap 'code=$?; if (( code != 0 )); then fail_setup_husk "$exit_cleanup_job"; release_unpublished_authorization "$exit_cleanup_authorization"; fi; cleanup_follow_up_message; release_cap_authority_lock; release_exit_lifecycle; release_goal_revision_lock; release_chain_lock "$exit_cleanup_chain"' EXIT
  permission_json=$(mktemp "$record_locks/follow-permissions.XXXXXX")
  json_field "$latest" permissions.requested >"$permission_json"
  permission_digest=$(sha256_file "$permission_json")
  tool_policy=$(json_field "$permission_json" tools)
  snapshot_json=$(mktemp "$record_locks/follow-snapshot.XXXXXX")
  select_snapshot "$runtime" "$role" "$permission_json" "$snapshot_json"
  read_snapshot_fields "$snapshot_json"
  payload="$agents/$root_id"; round_dir="$payload/rounds/$round"
  delivery_content=$message
  if [[ "$resume_cap" != true ]]; then
    resume_mode=fresh-context
    adapter_verb=dispatch
    parent_round=$(json_field "$latest" round)
    continuation_args=(
      --continuation "prior-brief=$payload/brief.md"
      --continuation "prior-return=$payload/rounds/$parent_round/return.json"
    )
  fi
  prompt_temp=$(mktemp "$record_locks/follow-composed-packet.XXXXXX")
  composition_temp=$(mktemp "$record_locks/follow-composition.XXXXXX")
  set +e
  composition_output=$("$ms" job compose-role-packet --root "$root" --role "$role" --brief "$delivery_content" \
    --job "$child" --runtime "$runtime" --model "$model" --tool-policy "$tool_policy" --round "$round" --mission "$mission" \
    --destructive-reach "$destructive_reach" \
    --output "$prompt_temp" --composition "$composition_temp" \
    "${continuation_args[@]+"${continuation_args[@]}"}")
  composition_rc=$?
  set -e
  if (( composition_rc != 0 )); then
    [[ -z "$composition_output" ]] || { record_delegate_outcome_raw "$composition_output"; printf '%s\n' "$composition_output"; }
    return "$composition_rc"
  fi
  input_bytes=$(enforce_inline_input_limit "$prompt_temp" message)
  input_hash=$(sha256_file "$prompt_temp")
  local output_stream
  output_stream=$("$root/scripts/agents/adapters/$runtime.sh" output-stream --round-dir "$round_dir") \
    || die 1 "$runtime adapter could not resolve its child output stream"

  product_root_args=(--product-root "$workspace")
  acquire_cap_authority_lock
  cap_resolution=$(mktemp "$record_locks/follow-cap-resolution.XXXXXX")
  model_key=$(canonical_model "$model")
  [[ -n "$model_key" ]] || die 1 "requested model has no canonical cap-key form"
  if (( repeated_follow_up )) && [[ -n "$mission" ]]; then
    cap=$(json_field "$jobs/$child.json" capMin)
    "$ms" job cap-resolution --cap "$cap" --rule repeated-operation \
      --origin existing-reservation --output "$cap_resolution"
  else
    exit_cleanup_authorization=$child
    authorize_job_cap "$child" "$role" "$runtime" "$model_key" "$mission" "" follow-up "$cap_resolution"
  fi
  cap=$(json_field "$cap_resolution" capMin)
  set +e
  preflight_output=$("$ms" job claim-launch --preflight --root "$root" --opid "$child" \
	--operation-id "$operation_id" \
    --session "$runtime:$session" --dispatch-mode follow-up --resumed-session "$resumed_for_claim" \
    --runtime "$runtime" --model "$model" --role "$role" \
    --reviews "$reviews" \
    --launch-mode "$launch_mode" --permission-envelope-digest "$permission_digest" \
    "${product_root_args[@]+"${product_root_args[@]}"}" \
    --cap-min "$cap" --conf "$root/metasystem.conf" --input-hash "$input_hash" \
    --goal "$goal" --goal-revision "$goal_revision" \
    --destructive-reach "$destructive_reach" --adapter-verb "$adapter_verb")
  preflight_rc=$?
  set -e
  preflight_outcome=$(json_value "$preflight_output" outcome 2>/dev/null || true)
  if (( preflight_rc != 0 )); then
    [[ -z "$preflight_output" ]] || { record_delegate_outcome_raw "$preflight_output"; printf '%s\n' "$preflight_output"; }
    return "$preflight_rc"
  fi
  [[ "$preflight_outcome" != PREFLIGHT-MATCHED ]] || replay_operation=1
  if (( replay_operation == 0 )); then
    require_goal_admission
    require_goal_revision_admission "$cap"
    require_slice_admission "$cap" "$approved_ref" "$goal" "$goal_revision"
  fi
  if ! acquire_lifecycle_lock_until "$child" 5; then
    record_delegate_outcome LOCK_BUSY refused "rank=job-lifecycle key=$child retry=retry-after-the-named-holder-releases" "$child"
    die 1 "LOCK_BUSY rank=job-lifecycle key=$child retry=retry-after-the-named-holder-releases"
  fi
  exit_cleanup_lifecycle=$child
  occupancy_preparation=$(mktemp "$record_locks/follow-claim-occupancy.XXXXXX")
  "$ms" job claim-occupancy-prepare --root "$root" --session "$runtime:$session" \
    --output "$occupancy_preparation"
  claim_output=$("$ms" job claim-launch --root "$root" --opid "$child" \
	--operation-id "$operation_id" \
    --session "$runtime:$session" --dispatch-mode follow-up --resumed-session "$resumed_for_claim" \
    --runtime "$runtime" --model "$model" --role "$role" \
    --reviews "$reviews" \
    --launch-mode "$launch_mode" --permission-envelope-digest "$permission_digest" \
    "${product_root_args[@]+"${product_root_args[@]}"}" \
    --cap-min "$cap" --conf "$root/metasystem.conf" --input-hash "$input_hash" \
    --main-id "$current_main_id" --claim-epoch "$reservation_claim_epoch" --goal "$goal" \
    --goal-revision "$goal_revision" --machine-id "$goal_machine" --approved-ref "$approved_ref" \
    --destructive-reach "$destructive_reach" --adapter-verb "$adapter_verb" \
    --creator-pid "$$" --occupancy-preparation "$occupancy_preparation") || claim_rc=$?
  rm -f "$occupancy_preparation"
  claim_outcome=$(json_value "$claim_output" outcome 2>/dev/null || true)
  if [[ -z "$claim_outcome" ]]; then
    [[ -z "$fresh_context_temp" ]] || rm -f "$fresh_context_temp"
    printf '%s\n' "$claim_output" >&2
    return 1
  fi
  if [[ "$claim_outcome" != WON ]]; then
    record_delegate_outcome_raw "$claim_output"
    [[ -z "$fresh_context_temp" ]] || rm -f "$fresh_context_temp"
    [[ ! -f "$jobs/$child.json" ]] || exit_cleanup_job=
    printf '%s\n' "$claim_output"
    return "$claim_rc"
  fi
  (( claim_rc == 0 )) || return "$claim_rc"
  launch_capability=$(json_value "$claim_output" evidence.launchCapability)
  [[ -n "$launch_capability" ]] || die 1 "claim-launch won without an adapter launch capability"
  exit_cleanup_authorization=
  release_cap_authority_lock

  mkdir -p "$round_dir"
  if [[ -n "$fresh_context_temp" ]]; then
    delivery_content="$round_dir/fresh-context.md"
    mv "$fresh_context_temp" "$delivery_content"
    fresh_context_temp=
  fi
  mv "$prompt_temp" "$round_dir/prompt.md"
  mv "$composition_temp" "$round_dir/composition.json"

  record_json=$(mktemp "$record_locks/follow-record.XXXXXX")
  "$ms" job build-follow-record --output "$record_json" --parent "$latest" \
    --job "$child" --operation-id "$operation_id" --round "$round" --parent-job "$(basename "${latest%.json}")" \
    --snapshot "$snapshot_path" --fallbacks "$fallbacks" --signal "$signal" \
    --handshake-budget "$handshake_budget" --resume-mode "$resume_mode" \
    --input-bytes "$input_bytes" --input-hash "$input_hash" \
    --mission-turn "$mission_turn" --main-id "$current_main_id" \
    --claim-epoch "$reservation_claim_epoch" --cap-resolution "$cap_resolution" \
    --root "$root" --goal-revision "$goal_revision" --approved-ref "$approved_ref" \
    --destructive-reach "$destructive_reach" \
    --composition "$round_dir/composition.json" \
    --launch-mode "$launch_mode" --output-stream "$output_stream"
  rm -f "$cap_resolution"
  cleanup_follow_up_message
  finalize_and_launch "$child" "$root_id" "$record_json" "$runtime" "$adapter_verb" "$handshake_budget" "$wait" "$launch_capability"
}

status_job() {
  local job= status
  [[ ${1:-} == --job && $# -eq 2 ]] || { usage; exit 2; }; job=$2
  valid_id "$job" || { usage; exit 2; }
  [[ -e "$jobs/$job.json" ]] || { echo "status: no job record for $job" >&2; return 6; }
  status=$(json_field "$jobs/$job.json" status 2>/dev/null || true)
  case "$status" in
    pending|running|completed|failed|timeout|cancelled)
      printf '%s\n' "$status"
      surface_census_verdict >&2
      ;;
    *) return 7 ;;
  esac
}

surface_census_verdict() {
  local verdict="$agents/supervision/last-census.json" value completed age
  if [[ ! -f "$verdict" ]]; then echo "CENSUS verdict=ABSENT"; return; fi
  value=$(json_field "$verdict" verdict 2>/dev/null || echo UNREADABLE)
  completed=$(json_field "$verdict" completedAtEpoch 2>/dev/null || echo 0)
  age=$(( $(date +%s) - completed ))
  printf 'CENSUS verdict=%s age=%ss fingerprint=%s\n' "$value" "$age" "$(json_field "$verdict" fingerprint 2>/dev/null || echo unavailable)"
}

cancel_job() {
  local job= cancel_status cancel_pid
  [[ ${1:-} == --job && $# -eq 2 ]] || { usage; exit 2; }; job=$2
  valid_id "$job" && [[ -f "$jobs/$job.json" ]] || die 1 "unknown job: $job"
  lease_entry_check
  # A reservation husk (and any record that never published a process) has
  # no adapter supervisor to negotiate with: routing it through the
  # runtime's cancel died resolving machinery that does not exist, and the
  # human's stop silently lost to a later setup (cancellation delta review
  # round 6). The internal gate owns that shape end to end — it marks,
  # concludes, and its cancelled status makes RecordSetup refuse forever.
  cancel_status=$(json_field "$jobs/$job.json" status 2>/dev/null || true)
  cancel_pid=$(json_field "$jobs/$job.json" pid 2>/dev/null || true)
  if [[ "$cancel_status" == pending-setup || -z "$cancel_pid" || "$cancel_pid" == null ]]; then
    lease_run_held "$current_claim_epoch" "$0" __cancel-owned --job "$job"
    return
  fi
  lease_run_held "$current_claim_epoch" \
    "$root/scripts/agents/adapters/$(json_field "$jobs/$job.json" runtime).sh" cancel --job "$job"
}

close_chain() {
  local job= root_id root_record status patch runner_closed=false reconcile_evidence=
  [[ ${1:-} == --job && $# -ge 2 ]] || { usage; exit 2; }; job=$2; shift 2
  while (($#)); do
    case "$1" in
      --runner-closed) runner_closed=true; shift ;;
      --reconcile-evidence)
        [[ $# -ge 2 && -z "$reconcile_evidence" ]] || { usage; exit 2; }
        reconcile_evidence=$2; shift 2 ;;
      *) usage; exit 2 ;;
    esac
  done
  lease_entry_check
  valid_id "$job" && [[ -f "$jobs/$job.json" ]] || die 1 "unknown job: $job"
  root_id=$(root_job_id "$job") || die 1 "cannot resolve job chain"
  [[ "$root_id" == "$job" ]] || die 1 "close requires the root job id: $root_id"
  acquire_chain_lock "$root_id"
  # The trap must reference a GLOBAL: when set -e unwinds the owning
  # function, bash 5.x has already popped its locals when the EXIT trap
  # runs, and under set -u the trap dies on the expansion before releasing
  # anything (the Linux cap-authority leak, go-production-grade Phase 1).
  exit_cleanup_chain=$root_id
  trap 'release_chain_lock "$exit_cleanup_chain"' EXIT
  root_record="$jobs/$root_id.json"
  if [[ -n "$reconcile_evidence" ]]; then
    valid_id "$reconcile_evidence" || die 2 "invalid review evidence job id: $reconcile_evidence"
    lease_run_held "$current_claim_epoch" "$0" __review-reference-reconcile \
      --root-job "$root_id" --evidence-job "$reconcile_evidence"
  fi
  # Closing asserts the chain's evidence is durable, so make it durable
  # first for EVERY terminal member: a reap mirrors only its own job's
  # round, and follow-up rounds otherwise reach this point unmirrored.
  # Best-effort per member; close-check below remains the authority and
  # refuses precisely when a mirror could not land.
  "$ms" job chain-members --jobs "$jobs" --root "$root_id" --terminal-only \
    | while IFS='|' read -r chain_job chain_status; do
      mirror_record "$chain_job" || true
    done
  "$ms" job close-check --repo "$root" --root "$root_id"
  status=$(json_field "$root_record" status)
  patch=$(mktemp "$record_locks/close.XXXXXX")
  if [[ "$runner_closed" == true ]]; then
    printf '{"chainClosed":true,"runnerClosed":true}\n' >"$patch"
  else
    printf '{"chainClosed":true}\n' >"$patch"
  fi
  lease_run_held "$current_claim_epoch" "$0" __record-cas --job "$root_id" \
    --expect "$status" --status "$status" --patch "$patch"
  rm -f "$patch"
  release_chain_lock "$root_id"; trap - EXIT
}

# The STANDING reaper mode died here (script-orchestration-08/D19): nothing
# in production ever launched `reap --interval`, Go owns the standing sweep
# (supervise component reaper), and its shadow verdicts — stale-claim-epoch,
# abandoned-setup — live in internal/lease/sweep.go and the Go reapers. What
# remains is the lease-held single-shot reap that wait_for_job and the
# mission drain actually use. A kill-capable shell daemon mode must not come
# back: the standing-reaper ruling denies shell reapers kill authority.
reap_jobs() {
  local job=
  while (($#)); do
    case "$1" in
      --job) [[ $# -ge 2 ]] || { usage; exit 2; }; job=$2; shift 2 ;;
      *) usage; exit 2 ;;
    esac
  done
  [[ -z "$job" ]] || valid_id "$job" || { usage; exit 2; }
  if [[ $lease_reentry -eq 0 ]]; then
    lease_entry_check
    if [[ -n "$job" ]]; then
      lease_run_held "$current_claim_epoch" "$0" __reap-held --job "$job"
    else
      lease_run_held "$current_claim_epoch" "$0" __reap-held
    fi
    return
  fi
  if [[ -n "$job" ]]; then reap_one "$job"; else
    mkdir -p "$jobs"
    # One failing reap must not starve the jobs after it in sort order
    # (script-orchestration-07): visit every record, then report the
    # sweep's verdict — the contract the Go reaper already documents.
    sweep_failed=0
    for record in "$jobs"/*.json; do
      [[ -f "$record" ]] || continue
      reap_one "$(basename "${record%.json}")" || sweep_failed=1
    done
    if (( sweep_failed != 0 )); then
      echo "reap sweep finished with failures (see above)" >&2
      exit 1
    fi
  fi
}

internal_register_custody() {
  local job= pid= started
  while (($#)); do
    case "$1" in --job) job=$2; shift 2 ;; --pid) pid=$2; shift 2 ;; *) exit 2 ;; esac
  done
  valid_id "$job" && [[ "$pid" =~ ^[1-9][0-9]*$ && -f "$jobs/$job.json" ]] || exit 2
  started=$("$ms" proc started-at --pid "$pid") || exit 1
  # The read-dedupe-append-write runs under the session and record locks in one
  # verb, so custody registration cannot race a status or occupancy transition.
  # The caller's observed start second stays a binding cross-check (L11 form).
  "$ms" job custody-add --root "$root" --job "$job" --pid "$pid" --pid-started "$started"
}

internal_handshake() {
  local job= session= turn= model= effective= signal=
  while (($#)); do
    case "$1" in
      --job) job=$2; shift 2 ;; --session) session=$2; shift 2 ;; --turn) turn=$2; shift 2 ;;
      --model) model=$2; shift 2 ;; --effective) effective=$2; shift 2 ;; --signal) signal=$2; shift 2 ;;
      *) exit 2 ;;
    esac
  done
  valid_id "$job" && [[ -f "$effective" && -f "$jobs/$job.json" ]] || exit 2
  patch=$(mktemp "$record_locks/handshake-patch.XXXXXX")
  "$ms" job handshake-eval --record "$jobs/$job.json" --effective "$effective" \
    --session "$session" --turn "$turn" --model "$model" --signal "$signal" \
    --output "$patch" || exit 1
  target=$(json_field "$patch" target)
  body=$(mktemp "$record_locks/handshake-body.XXXXXX")
  json_field "$patch" patch >"$body"
  record_cas "$job" pending "$target" "$body"
  [[ "$target" == running ]]
}

internal_cancel() {
  local job=$1 record="$jobs/$1.json" status patch cancel_pid cancel_pgid cancel_mission
  [[ -f "$record" ]] || exit 1
  process_instance_tag=${process_instance_tag:-$job}
  acquire_lifecycle_lock_until "$job" 5 || exit 1
  status=$(json_field "$record" status)
  case "$status" in pending-setup|pending|running) ;; *) release_lifecycle_lock "$job"; return 0 ;; esac
  # The marker lands BEFORE the kill: a reaper pass that concludes the
  # dead group before our own swap below still reads the cancel and
  # concludes cancelled — the kill-before-mark window is closed. An
  # unmarked kill would reopen it, so a marker that cannot land stops
  # the cancel: a lost compare means the job went terminal under us
  # (nothing left to cancel); any other failure refuses by name.
  patch=$(mktemp "$record_locks/cancelling.XXXXXX"); printf '{"phase":"cancelling"}\n' >"$patch"
  if ! record_cas "$job" "$status" "$status" "$patch"; then
    status=$(json_field "$record" status)
    case "$status" in
      pending-setup|pending|running)
        # An already-marked record is a PRIOR cancel's footprint (a
        # crash between mark and conclude, or a concurrent retry):
        # the mark is in place, so this cancel proceeds to finish
        # the job the earlier one started. Anything else refuses —
        # killing an unmarked job reopens the window.
        if [[ "$(json_field "$record" phase 2>/dev/null || true)" == cancelling ]]; then
          :
        else
          # Between the two reads above a reaper can conclude the
          # record cancelled; one fresh status read separates "the
          # cancel already succeeded" from a genuine mark failure.
          status=$(json_field "$record" status)
          case "$status" in
            pending-setup|pending|running) release_lifecycle_lock "$job"; die 1 "cancel could not mark $job; refusing to kill an unmarked job" ;;
            *) release_lifecycle_lock "$job"; return 0 ;;
          esac
        fi ;;
      *) release_lifecycle_lock "$job"; return 0 ;;
    esac
  fi
  cancel_pid=$(json_field "$record" pid 2>/dev/null || true)
  cancel_pgid=$(json_field "$record" pgid 2>/dev/null || true)
  if [[ "$cancel_pgid" =~ ^[0-9]+$ && "$cancel_pgid" -gt 1 ]]; then
    wind_down_group "$record" || { release_lifecycle_lock "$job"; exit 1; }
  elif [[ "$cancel_pid" =~ ^[0-9]+$ && "$cancel_pid" -gt 0 ]]; then
    release_lifecycle_lock "$job"
    die 1 "cancel refused: $job has a recorded process but no primary process group"
  fi
  patch=$(mktemp "$record_locks/cancel.XXXXXX")
  # json_field renders a JSON null as the string "null": a death stamp needs
  # a real signalable group id, so the predicate is numeric and above one.
  if [[ "$cancel_pgid" =~ ^[0-9]+$ && "$cancel_pgid" -gt 1 ]]; then
    printf '{"error":null,"phase":"cancelled","groupDeathProvenAt":"%s"}\n' "$(now_iso)" >"$patch"
  else
    # No group was ever recorded: the record must not claim a death
    # that never happened — cancelled-before-launch is its own
    # honest shape.
    printf '{"error":null,"phase":"cancelled"}\n' >"$patch"
  fi
  if ! record_cas "$job" "$status" cancelled "$patch"; then
    if [[ -n "$stop_cancel_authorized" ]]; then
      release_lifecycle_lock "$job"
      return 1
    fi
  fi
  # The cancelled reservation's fence slot is not abandoned debt: a
  # mission-bound job released here exactly as a failed setup husk is.
  cancel_mission=$(json_field "$record" mission 2>/dev/null || true)
  if [[ -n "$cancel_mission" && "$cancel_mission" != null ]]; then
    mission_fence release-job --repo "$root" --mission "$cancel_mission" --job "$job" >/dev/null 2>&1 || true
  fi
  [[ -n "$stop_cancel_authorized" ]] || mirror_record "$job" || true
  release_lifecycle_lock "$job"
}

internal_breach_stop_run() { # stop id
  local stop_id=$1 verdict state job reconcile_rc
  while :; do
    set +e
    verdict=$("$ms" job stop-batch-reconcile --root "$root" --stop "$stop_id" 2>&1)
    reconcile_rc=$?
    set -e
    [[ "$reconcile_rc" == 0 ]] \
      || die 1 "breach-stop $stop_id is indeterminate: $verdict"
    state=$(json_value "$verdict" state)
    [[ "$state" != COMPLETE ]] || return 0
    while IFS= read -r job; do
      [[ -n "$job" ]] || continue
      "$ms" job stop-cancel-authorize --root "$root" --stop "$stop_id" --job "$job" \
        || die 1 "breach-stop $stop_id lost cancellation authority for $job"
      stop_cancel_authorized=$stop_id
      internal_cancel "$job"
      stop_cancel_authorized=
    done < <("$ms" job stop-batch-pending --root "$root" --stop "$stop_id")
  done
}

internal_breach_stop_goal() {
  local goal_id= revision= batch stop_id
  while (($#)); do
    case "$1" in
      --goal) goal_id=$2; shift 2 ;;
      --revision) revision=$2; shift 2 ;;
      *) exit 2 ;;
    esac
  done
  valid_id "$goal_id" && [[ "$revision" =~ ^[1-9][0-9]*$ ]] || exit 2
  internal_authority stop-custodian
  batch=$("$ms" job breach-stop --root "$root" --goal "$goal_id" --revision "$revision") \
    || die 1 "breach-stop could not close $goal_id revision $revision"
  stop_id=$(json_value "$batch" stopId)
  internal_breach_stop_run "$stop_id"
  printf 'stop=%s state=COMPLETE\n' "$stop_id"
}

internal_reap_held() {
  internal_authority holder-only
  lease_reentry=1
  # The native owner lock refuses an empty instance tag (the retired python
  # accepted one, which silently disabled holder-liveness discrimination).
  # This internal entry has no __lock-owner re-exec, so default to the verb
  # name — a token genuinely present in this process's command line.
  process_instance_tag=${process_instance_tag:-__reap-held}
  reap_jobs "$@"
}

internal_launch() {
  local runtime= verb= job= tag= launch_capability=
  while (($#)); do
    case "$1" in
      --runtime) runtime=$2; shift 2 ;;
      --verb) verb=$2; shift 2 ;;
      --job) job=$2; shift 2 ;;
      --tag) tag=$2; shift 2 ;;
      --launch-capability) launch_capability=$2; shift 2 ;;
      *) exit 2 ;;
    esac
  done
  [[ -n "$runtime" && ( "$verb" == dispatch || "$verb" == follow-up ) \
    && -n "$job" && -n "$tag" && -n "$launch_capability" ]] || exit 2
  internal_authority holder-only "$job"
  launch_adapter "$runtime" "$verb" "$job" "$tag" "$launch_capability"
}

internal_critique_mutation() { # job verb, mutation flags
  local verb=$1 previous= argument root_job=
  shift
  for argument in "$@"; do
    if [[ "$previous" == --root-job ]]; then
      root_job=$argument
      break
    fi
    previous=$argument
  done
  valid_id "$root_job" || exit 2
  internal_authority holder-only "$root_job"
  "$ms" job "$verb" --repo "$root" "$@"
}

internal_handshake_timeout() {
  local job=
  [[ ${1:-} == --job && $# -eq 2 ]] || exit 2
  job=$2
  internal_authority holder-only "$job"
  local record="$jobs/$job.json" patch status session
  # An adapter that starts and then never signals a session leaves the record
  # in running, not pending. Writing the verdict only from pending meant this
  # -- the exact case the handshake timeout exists for -- wrote nothing at all,
  # and the reaper's backstop later called it process-lost instead.
  local attempt
  status=$(json_field "$record" status 2>/dev/null || true)
  # This write is the dispatcher's verdict on its own wait. Every step of it
  # used to fail silently, so a job that ended up diagnosed by the reaper's
  # backstop instead gave no clue which step dropped it. The job log says.
  printf '%s handshake-timeout entered status=%s\n' "$(now_iso)" "$status" >>"$jobs/$job.log"
  case "$status" in pending|running) ;; *) return 0 ;; esac
  # Stand down BEFORE killing anything if a session already landed. The waiter
  # gave up at the deadline and the adapter can record a session a moment later;
  # a session in the record means the wait was won, just late. Winding down the
  # group first killed that live, successful turn before this check could see it.
  session=$(json_field "$record" sessionId 2>/dev/null || true)
  if [[ -n "$session" && "$session" != null ]]; then
    printf '%s handshake-timeout stood down; session %s landed before wind-down\n' \
      "$(now_iso)" "$session" >>"$jobs/$job.log"
    return 0
  fi
  # Record the verdict BEFORE killing the group, not after. Winding down first
  # left a gap in which the reaper swept, saw the freshly-killed supervisor as
  # process-lost, and wrote that before this verdict landed -- the dispatcher
  # owns the handshake verdict, so it claims the record first and kills the
  # now-condemned group second. Compare-and-swap, so retry on a losing compare:
  # an adapter moves the record pending->running in exactly this window. The
  # record wrapper deletes the patch after each call, so each attempt makes its
  # own.
  local recorded=0
  for attempt in 1 2 3; do
    status=$(json_field "$record" status 2>/dev/null || true)
    case "$status" in
      pending|running) ;;
      *)
        printf '%s handshake-timeout stood down; record is already %s\n' "$(now_iso)" "$status" >>"$jobs/$job.log"
        return 0
        ;;
    esac
    session=$(json_field "$record" sessionId 2>/dev/null || true)
    if [[ -n "$session" && "$session" != null ]]; then
      printf '%s handshake-timeout stood down; session %s landed while it was being written\n' \
        "$(now_iso)" "$session" >>"$jobs/$job.log"
      return 0
    fi
    patch=$(mktemp "$record_locks/handshake.XXXXXX")
    printf '{"error":"handshake_timeout","phase":"handshake"}\n' >"$patch"
    if record_cas "$job" "$status" failed "$patch"; then
      printf '%s handshake-timeout recorded from %s\n' "$(now_iso)" "$status" >>"$jobs/$job.log"
      recorded=1
      break
    fi
  done
  if (( ! recorded )); then
    printf '%s handshake-timeout lost three compares; the record kept changing\n' "$(now_iso)" >>"$jobs/$job.log"
    return 1
  fi
  # The verdict stands; cleaning up the stalled group is best-effort now.
  wind_down_group "$record" \
    || printf '%s handshake-timeout recorded, but the group did not wind down cleanly\n' "$(now_iso)" >>"$jobs/$job.log"
}

# Tests source the dispatch functions with process and signalling probes
# replaced by deterministic fixtures. A sourced dispatcher never runs a public
# command; production invocations continue through the command router below.
if [[ ${BASH_SOURCE[0]} != "$0" ]]; then
  return 0
fi

# Lock-owning public commands re-exec once so their lease tag is part of the
# process command line and a contender can distinguish this process from PID
# reuse. Internal adapter callbacks never acquire a chain lock.
if [[ ${1:-} == __lock-owner ]]; then
  [[ $# -ge 3 ]] || exit 2
  process_instance_tag=$2; shift 2
elif [[ ${1:-} != __* ]]; then
  public=${1:-dispatch}
  [[ "$public" == dispatch || "$public" == follow-up || "$public" == close || "$public" == reap || "$public" == --* ]] && {
    tag="metasystem-lock-$$-$(date +%s)"
    exec "$0" __lock-owner "$tag" "$@"
  }
fi

command=${1:-}
if [[ "$command" == --* ]]; then command=dispatch; else shift || true; fi
case "$command" in
  dispatch) dispatch_job "$@" ;;
  watch) watch_job "$@" ;;
  follow-up) follow_up "$@" ;;
  status) status_job "$@" ;;
  cancel) cancel_job "$@" ;;
  close) close_chain "$@" ;;
  reap) reap_jobs "$@" ;;
  __record-create)
    # The steward's admission is per-job: the authority check must
    # see which job this record creates.
    rc_job=
    if [[ ${1:-} == --job && $# -ge 2 ]]; then rc_job=$2; fi
    internal_authority holder-only "$rc_job"
    "$ms" job record-create --root "$root" "$@"
    ;;
  __record-setup)
    rs_job=
    if [[ ${1:-} == --job && $# -ge 2 ]]; then rs_job=$2; fi
    internal_authority holder-only "$rs_job"
    "$ms" job record-setup --root "$root" "$@"
    ;;
  __record-cas)
    [[ ${1:-} == --job && $# -ge 2 ]] || exit 2
    internal_authority record-writer "$2"
    "$ms" job record-cas --root "$root" "$@"
    ;;
  __protocol-error)
    [[ ${1:-} == --job && $# -ge 2 ]] || exit 2
    internal_authority adapter-writer "$2"
    "$ms" job record-protocol-error --root "$root" "$@"
    ;;
  __repair-claim)
    [[ ${1:-} == --job && $# -ge 2 ]] || exit 2
    internal_authority record-writer "$2"
    "$ms" job repair-claim --root "$root" "$@"
    ;;
  __critique-register-advance) internal_critique_mutation critique-register-advance "$@" ;;
  __critique-exhaustion-advance) internal_critique_mutation critique-exhaustion-advance "$@" ;;
  __review-reference-reconcile) internal_critique_mutation review-reference-reconcile "$@" ;;
  __launch) internal_launch "$@" ;;
  __handshake-timeout) internal_handshake_timeout "$@" ;;
  __reap-held) internal_reap_held "$@" ;;
  __handshake)
    [[ ${1:-} == --job && $# -ge 2 ]] || exit 2
    internal_authority adapter-writer "$2"
    internal_handshake "$@"
    ;;
  __cancel-owned)
    [[ ${1:-} == --job && $# -eq 2 ]] || exit 2
    internal_authority holder-only "$2"
    internal_cancel "$2"
    ;;
  __breach-stop-goal) internal_breach_stop_goal "$@" ;;
  __register-custody)
    [[ ${1:-} == --job && $# -ge 2 ]] || exit 2
    internal_authority adapter-writer "$2"
    internal_register_custody "$@"
    ;;
  -h|--help) usage ;;
  *) usage; exit 2 ;;
esac
