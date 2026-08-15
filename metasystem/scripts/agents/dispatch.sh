#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/agents/dispatch.sh dispatch --role <role> --brief <file>
      [--mode <working-mode>] [--runtime claude|codex|devin|fake]
      [--model <model>] [--job-id <id>] [--reviews <implementer-job-id>]
      [--workspace <dir> | --worktree]
      [--permissions <preset|envelope-file>] [--mission <id>]
      [--approve-escalation] [--wait] [--cap-min N]
  scripts/agents/dispatch.sh --role <role> --brief <file> [dispatch options]
  scripts/agents/dispatch.sh follow-up --job <job-id> --message <file> [--wait]
  scripts/agents/dispatch.sh status --job <job-id>
  scripts/agents/dispatch.sh cancel --job <job-id>
  scripts/agents/dispatch.sh close --job <root-id> [--runner-closed]
  scripts/agents/dispatch.sh reap [--job <job-id>]

Exit codes: 0 success/completed; 2 usage; 3 failed; 4 timeout;
5 vanished; 6 unknown status job; 7 malformed status record; 8 cancelled.
USAGE
}

die() { last_die_message=$2; echo "$2" >&2; exit "$1"; }

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
repo_scope=$(git -C "$root" rev-parse --show-toplevel 2>/dev/null) \
  || die 1 "metasystem installation is not inside a git repository: $root"
repo_scope=$(cd "$repo_scope" && pwd -P)
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"
config="$root/scripts/metasystem-config.sh"
agents="$root/artifacts/agents"
jobs="$agents/jobs"
heartbeats="$agents/hb"
locks="$agents/locks"
record_locks="$agents/record-locks"
capabilities="$agents/capabilities"
worktrees="$agents/worktrees"
process_instance_tag=
cap_authority_lock_held=0
exit_cleanup_job=
exit_cleanup_chain=
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
  "$0" __record-cas --job "$1" --expect "$2" --status "$3" --patch "$4" || cas_rc=$?
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

record_create() { # job, source json
  "$0" __record-create --job "$1" --source "$2" && emit_event dispatch job-created "jobId=$1" "summary=record created"
}

# A refusal between the reservation and the full record must not leave a
# pending-setup husk holding a fence slot: the exiting dispatch fails its own
# reservation. The compare-and-swap no-ops once the record advanced past
# pending-setup, so a successful dispatch is untouched.
fail_setup_husk() { # job id
  local husk_job=$1 husk_patch
  [[ -n "$husk_job" && -f "$jobs/$husk_job.json" ]] || return 0
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
  if [[ -n "$expected" ]]; then
    "$ms" lease run-held --root "$root" --caller-pid "$entry_caller_pid" \
      --expected-epoch "$expected" -- "$@"
  else
    "$ms" lease run-held --root "$root" --caller-pid "$entry_caller_pid" -- "$@"
  fi
}

internal_authority() { # holder-only|record-writer|adapter-writer|supervision-only, optional job id
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
  local record=$1 pid tag runtime proof heartbeat
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
  process_exists "$pid" || return 1
  proof=$(json_field "$record" ownershipProof 2>/dev/null || true)
  heartbeat="$heartbeats/$(json_field "$record" jobId 2>/dev/null || true)"
  [[ "$proof" == *"\"pid\":$pid"* && "$proof" == *"\"instanceTag\":\"$tag\""* && -f "$heartbeat" ]] || return 1
  [[ "$(json_field "$heartbeat" pid 2>/dev/null || true)" == "$pid" \
    && "$(json_field "$heartbeat" instanceTag 2>/dev/null || true)" == "$tag" ]]
}

group_alive() { # pgid
  local pgid=$1
  [[ "$pgid" =~ ^[1-9][0-9]*$ ]] || return 1
  "$ms" proc group-exists --pgid "$pgid"
}

group_owned() { # record
  local record=$1 pgid tag runtime proof
  pgid=$(json_field "$record" pgid 2>/dev/null || true)
  tag=$(json_field "$record" instanceTag 2>/dev/null || true)
  [[ "$pgid" =~ ^[1-9][0-9]*$ && "$pgid" -gt 1 ]] || return 1
  if ps -axo pgid=,command= 2>/dev/null | awk -v wanted="$pgid" -v tag="$tag" '
    $1 == wanted { $1=""; sub(/^ +/, ""); if (index($0, tag)) found=1 }
    END { exit !found }
  '; then
    return 0
  fi
  # The fake runtime runs under restricted CI sandboxes that may deny process
  # table reads for detached sessions. Its trusted launcher records the exact
  # pgid/tag pair before releasing the start gate, so fake-only fixtures can
  # still exercise real group signals without weakening a real adapter.
  runtime=$(json_field "$record" runtime 2>/dev/null || true)
  proof=$(json_field "$record" ownershipProof 2>/dev/null || true)
  [[ "$runtime" == fake && "$proof" == *"\"pgid\":$pgid"* && "$proof" == *"\"instanceTag\":\"$tag\""* ]]
}

wind_down_group() { # record
  local record=$1 pgid tag until
  pgid=$(json_field "$record" pgid 2>/dev/null || true)
  tag=$(json_field "$record" instanceTag 2>/dev/null || true)
  group_alive "$pgid" || return 0
  group_owned "$record" || { echo "refusing to signal unowned process group $pgid" >&2; return 1; }
  kill -TERM -- "-$pgid" 2>/dev/null || true
  until=$(( $(date +%s) + 2 ))
  while group_alive "$pgid" && (( $(date +%s) < until )); do sleep 0.05; done
  if group_alive "$pgid"; then
    group_owned "$record" || { echo "lost ownership proof for process group $pgid" >&2; return 1; }
    kill -KILL -- "-$pgid" 2>/dev/null || true
  fi
  # When this shell launched the supervisor directly, reap its terminal wait
  # status now so a zombie group leader cannot masquerade as a live writer.
  wait "$(json_field "$record" pid 2>/dev/null || true)" 2>/dev/null || true
  until=$(( $(date +%s) + 2 ))
  while group_alive "$pgid" && (( $(date +%s) < until )); do sleep 0.05; done
  group_alive "$pgid" && { echo "process group $pgid survived KILL" >&2; return 1; }
  return 0
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

release_chain_lock() { # root id
  local status=0
  owner_lock release "$locks/$1.d" "$$" "$process_instance_tag" || status=$?
  (( status == 4 )) && die 1 "refusing to release another owner's chain lock"
  return 0
}

acquire_lifecycle_lock() { # job id; nonzero means a live owner has it
  mkdir -p "$record_locks"
  owner_lock claim "$record_locks/$1.lifecycle.d" "$$" "$process_instance_tag"
}

acquire_lifecycle_lock_until() { # job id, maximum wait seconds
  local job=$1 base=$2 maximum started deadline elapsed
  maximum=$(dispatch_fixture_wait_cap "$base")
  started=$SECONDS
  deadline=$(( SECONDS + maximum ))
  while ! acquire_lifecycle_lock "$job"; do
    if (( SECONDS >= deadline )); then
      elapsed=$((SECONDS - started))
      echo "timed out acquiring lifecycle lock for $job (elapsed: ${elapsed}s; scaled cap: ${maximum}s)" >&2
      return 1
    fi
    sleep 0.05
  done
}

release_lifecycle_lock() { # job id
  owner_lock release "$record_locks/$1.lifecycle.d" "$$" "$process_instance_tag" || true
}

acquire_cap_authority_lock() {
  # Identity-bearing owner lock (script-orchestration-01/D18): the old bare
  # mkdir spinlock had no owner and no healer, so a SIGKILLed holder bricked
  # every dispatch AND arming until a human ran rmdir. The owner-lock verb
  # heals a provably dead holder's husk and keeps an unprovable one (B1) —
  # the same protocol the lifecycle and chain locks above already use.
  local directory="$agents/supervision/cap-authority.lock.d" maximum started deadline elapsed
  mkdir -p "${directory%/*}"
  maximum=$(dispatch_fixture_wait_cap 10)
  started=$SECONDS
  deadline=$((SECONDS + maximum))
  while ! owner_lock claim "$directory" "$$" "$process_instance_tag"; do
    if (( SECONDS >= deadline )); then
      elapsed=$((SECONDS - started))
      die 1 "timed out acquiring repository cap-authority lock (elapsed: ${elapsed}s; scaled cap: ${maximum}s)"
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
  [[ -z "$env_turn" || -n "$env_id" ]] \
    || die 1 "ambiguous inherited mission context: METASYSTEM_MISSION_TURN requires METASYSTEM_MISSION_ID and METASYSTEM_MISSION_LEASE"
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
  if [[ -f "$requested" ]]; then source=$requested; preset=custom; else source="$root/scripts/agents/permissions/$requested.json"; preset=$requested; fi
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

write_prompt() { # path job role runtime model round mission content
  local path=$1 job=$2 role=$3 runtime=$4 model=$5 round=$6 mission=${7:-none} content=$8
  {
    printf 'Job-Id: %s\nRole: %s\nRuntime: %s\nModel: %s\nRound: %s\nMission: %s\n\n' "$job" "$role" "$runtime" "$model" "$round" "${mission:-none}"
    cat "$root/scripts/agents/roles/$role.md"
    printf '\n\n'
    cat "$content"
  } >"$path"
}

launch_adapter() { # runtime verb job tag
  local runtime=$1 verb=$2 job=$3 tag=$4 gate="$heartbeats/$job.start" adapter="$root/scripts/agents/adapters/$runtime.sh" pid pid_started patch cap started deadline elapsed poll_sleep handshake_budget handshake_deadline proven_at
  poll_sleep=$(milliseconds_to_sleep "${METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS:-20}")
  mkdir -p "$heartbeats"
  pid=$("$ms" supervise launch-detached --cwd "$root" \
    --env "GIT_AUTHOR_NAME=$job" --env "GIT_AUTHOR_EMAIL=$job@metasystem.invalid" -- \
    "$adapter" "$verb" --job "$job" --start-gate "$gate" --instance-tag "$tag") || return 1
  cap=$(dispatch_fixture_wait_cap 5)
  started=$SECONDS
  deadline=$((SECONDS + cap))
  until pid_started=$("$ms" proc started-at --pid "$pid" 2>/dev/null); do
    if (( SECONDS >= deadline )); then
      elapsed=$((SECONDS - started))
      echo "adapter start identity ceiling reached for $job (elapsed: ${elapsed}s; scaled cap: ${cap}s)" >&2
      return 1
    fi
    sleep "$poll_sleep"
  done
  patch=$(mktemp "$record_locks/launch.XXXXXX")
  # The handshake deadline is stamped HERE, at launch, because that is when the
  # dispatcher starts waiting. Derived from the record's creation time instead,
  # the reaper's copy of the same deadline ran early by however long setup took,
  # and the backstop then overwrote the dispatcher's own verdict.
  handshake_budget=$(json_field "$jobs/$job.json" sessionEstablishedTimeoutSec 2>/dev/null || echo 0)
  proven_at=$(now_iso)
  handshake_deadline=
  if [[ "$handshake_budget" =~ ^[1-9][0-9]*$ ]]; then
    handshake_deadline=$(( $(date +%s) + handshake_budget ))
  fi
  {
    printf '{"pid":%s,"pidStartedAt":%s,"pgid":%s,' "$pid" "$pid_started" "$pid"
    printf '"ownershipProof":{"pid":%s,"pidStartedAt":%s,"pgid":%s,"instanceTag":"%s","provenAt":"%s","source":"trusted-launcher"}' \
      "$pid" "$pid_started" "$pid" "$tag" "$proven_at"
    [[ -z "$handshake_deadline" ]] || printf ',"handshakeDeadline":%s' "$handshake_deadline"
    printf '}\n'
  } >"$patch"
  record_cas "$job" pending pending "$patch" || return 1
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
  local job=$1 record="$jobs/$1.json" status pid tag facts budget_expired patch root_id mission record_epoch lease_epoch refusal_reason truncated_by
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
  # abandonment, the handshake window, and budget expiry. Budget expiry is the
  # SAME decision code the supervision reaper runs, so the two can never
  # disagree about one record.
  facts=$("$ms" job reap-facts --record "$record" --grace "$handshake_backstop_grace_sec") || return 1
  if [[ "$status" == pending-setup ]]; then
    # Abandoned pending-setup debris belongs to the Go reapers now
    # (internal/supervise/reaper.go and missionrunner drain, D19): the
    # dispatch-held single-shot reap launches nothing and leaves it.
    return 0
  fi
  pid=$(json_field "$record" pid 2>/dev/null || true)
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
      mission_fence refuse --repo "$root" --mission "$mission" --reason "$refusal_reason" >/dev/null || true
      aggregate_mission_usage "$record" || true
    fi
    mirror_record "$job" || true
  fi
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
  local role= brief= mode_override= runtime_override= model_override= job= reviews= workspace= permissions_override= mission_override= cap_override= serving_goal=0
  local use_worktree=0 wait=0 approve_escalation=0 mode runtime model requested_model roster_runtime roster_model roster_pair requested_pair
  local overridden=false mission_data mission lease mission_turn canonical model_key cap_resolution tiers_present=false escalation_required=0
  local cost_direction= approval_name= approved_at= roster_json=
  local permission_name permission_json snapshot_json snapshot_path fallbacks signal handshake_budget resume_cap input_bytes input_hash payload round_dir record_json setup_json
  while (($#)); do
    case "$1" in
      --role) [[ $# -ge 2 ]] || { usage; exit 2; }; role=$2; shift 2 ;;
      --brief) [[ $# -ge 2 ]] || { usage; exit 2; }; brief=$2; shift 2 ;;
      --mode) [[ $# -ge 2 ]] || { usage; exit 2; }; mode_override=$2; shift 2 ;;
      --runtime) [[ $# -ge 2 ]] || { usage; exit 2; }; runtime_override=$2; shift 2 ;;
      --model) [[ $# -ge 2 ]] || { usage; exit 2; }; model_override=$2; shift 2 ;;
      --job-id) [[ $# -ge 2 ]] || { usage; exit 2; }; job=$2; shift 2 ;;
      --reviews) [[ $# -ge 2 ]] || { usage; exit 2; }; reviews=$2; shift 2 ;;
      --workspace) [[ $# -ge 2 ]] || { usage; exit 2; }; workspace=$2; shift 2 ;;
      --worktree) use_worktree=1; shift ;;
      --permissions) [[ $# -ge 2 ]] || { usage; exit 2; }; permissions_override=$2; shift 2 ;;
      --mission) [[ $# -ge 2 ]] || { usage; exit 2; }; mission_override=$2; shift 2 ;;
      --cap-min) [[ $# -ge 2 ]] || { usage; exit 2; }; cap_override=$2; shift 2 ;;
      --approve-escalation) approve_escalation=1; shift ;;
      --serving-goal) serving_goal=1; shift ;;
      --wait) wait=1; shift ;;
      *) usage; exit 2 ;;
    esac
  done
  [[ -n "$role" && -f "$brief" ]] || { usage; exit 2; }
  [[ -f "$root/scripts/agents/roles/$role.md" && -f "$root/scripts/agents/roles/$role.requirements.json" ]] || die 1 "unknown dispatch role: $role"
  if [[ "$role" == code-critic ]]; then
    [[ -n "$reviews" ]] || die 2 "code-critic dispatch requires --reviews <implementer-job-id>"
    valid_id "$reviews" || die 2 "invalid implementer job id for --reviews: $reviews"
    [[ -f "$jobs/$reviews.json" ]] || die 1 "code-critic dispatch cannot review unknown implementer job: $reviews"
    [[ "$(json_field "$jobs/$reviews.json" role 2>/dev/null || true)" == implementer ]] \
      || die 1 "code-critic dispatch --reviews must name an implementer job: $reviews"
  elif [[ -n "$reviews" ]]; then
    die 2 "--reviews is only valid for the code-critic role"
  fi
  [[ ! ( $use_worktree -eq 1 && -n "$workspace" ) ]] || die 2 "--workspace and --worktree are mutually exclusive"
  if (( approve_escalation )) && { [[ ! -t 0 ]] || [[ ! -t 2 ]]; }; then
    die 1 "--approve-escalation requires an interactive TTY; remove the flag or re-run the same dispatch from a TTY"
  fi
  mode=$(brief_mode "$brief") || die 1 "brief must contain exactly one filled Working Mode header"
  [[ -z "$mode_override" || "$mode_override" == "$mode" ]] || die 1 "--mode contradicts the brief's Working Mode header"
  lease_entry_check

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

  if [[ -z "$job" ]]; then
    job="$role-$(date -u +%Y%m%dt%H%M%Sz)-$("$ms" util token-hex --bytes 2)"
  fi
  valid_id "$job" || die 2 "invalid job id: $job"
  # Preconditions BEFORE the id is reserved. The reservation record exists to
  # stop two mains racing one job id; a refusal after it left a pending-setup
  # husk that burned that id permanently, so a stale census cost the caller
  # their chosen name as well as their dispatch.
  require_fresh_census
  report_plan_drift
  setup_json=$(mktemp "${TMPDIR:-/tmp}/metasystem-pending-setup.XXXXXX")
  "$ms" job build-setup --output "$setup_json" --job "$job" --role "$role" \
    --main-id "$current_main_id" --claim-epoch "$current_claim_epoch"
  # INVARIANT (kept in shell by ruling, plans/go-surface-consolidation.md):
  # the reservation is TWO-PHASE — this record-create must land before any
  # of the setup below so a second dispatcher cannot prepare the same job
  # id, and the observable pending-setup status feeds the cleanup trap. Do
  # not merge the phases into one verb.
  lease_run_held "$current_claim_epoch" "$0" __record-create --job "$job" --source "$setup_json"
  rm -f "$setup_json"
  mkdir -p "$jobs" "$record_locks" "$capabilities" "$worktrees"
  acquire_chain_lock "$job"
  exit_cleanup_job=$job
  exit_cleanup_chain=$job
  trap 'code=$?; (( code == 0 )) || fail_setup_husk "$exit_cleanup_job"; release_cap_authority_lock; release_chain_lock "$exit_cleanup_chain"' EXIT
  acquire_cap_authority_lock
  [[ ! -e "$agents/$job" ]] || die 1 "job payload collision: $job"

  cap_resolution=$(mktemp "$record_locks/cap-resolution.XXXXXX")
  model_key=$(canonical_model "$model")
  [[ -n "$model_key" ]] || die 1 "requested model has no canonical cap-key form"
  authorize_job_cap "$job" "$role" "$runtime" "$model_key" "$mission" "$cap_override" dispatch "$cap_resolution"

  if (( use_worktree )); then
    workspace="$worktrees/$job"
    [[ ! -e "$workspace" ]] || die 1 "job worktree already exists: $workspace"
    git -C "$repo_scope" worktree add -q -b "agent/$job" "$workspace" HEAD || die 1 "could not create job worktree"
  else
    workspace=${workspace:-$repo_scope}
    workspace=$(cd "$workspace" && pwd -P) || die 1 "workspace does not exist: $workspace"
  fi
  permission_name=${permissions_override:-$(config_get --key "dispatch.permissions.$role" --default none)}
  permission_json=$(mktemp "$record_locks/permissions.XXXXXX")
  expand_permissions "$permission_name" "$workspace" "$use_worktree" "$permission_json"
  snapshot_json=$(mktemp "$record_locks/snapshot.XXXXXX")
  select_snapshot "$runtime" "$role" "$permission_json" "$snapshot_json"
  read_snapshot_fields "$snapshot_json"

  # --serving-goal (goal-system GOAL-08): the Go core resolves the
  # section through the goal parser and refuses (exit 3) when no usable
  # Current goal exists; the section joins the brief BEFORE the hash so
  # it is part of the recorded bytes and survives every fallback rebuild.
  if [[ "$serving_goal" == 1 ]]; then
    local goal_section brief_with_goal
    goal_section=$("$ms" dispatch serving-goal --root "$root") \
      || die 3 "no current goal to project (--serving-goal)"
    brief_with_goal=$(mktemp "$record_locks/brief.XXXXXX")
    cat "$brief" > "$brief_with_goal"
    printf '\n%s' "$goal_section" >> "$brief_with_goal"
    brief=$brief_with_goal
  fi

  input_bytes=$(enforce_inline_input_limit "$brief" brief)
  input_hash=$(sha256_file "$brief")
  payload="$agents/$job"; round_dir="$payload/rounds/1"
  mkdir -p "$round_dir"
  cp "$brief" "$payload/brief.md"
  write_prompt "$round_dir/prompt.md" "$job" "$role" "$runtime" "$model" 1 "${mission:-none}" "$brief"

  record_json=$(mktemp "$record_locks/record.XXXXXX")
  "$ms" job build-record --output "$record_json" --job "$job" --role "$role" \
    --mission "$mission" --mission-turn "$mission_turn" --runtime "$runtime" \
    --workspace "$workspace" --cap-resolution "$cap_resolution" --model "$model" \
    --overridden "$overridden" --snapshot "$snapshot_path" \
    --input-bytes "$input_bytes" --input-hash "$input_hash" \
    --permissions "$permission_json" --fallbacks "$fallbacks" --signal "$signal" \
    --handshake-budget "$handshake_budget" --approval-name "$approval_name" \
    --approved-at "$approved_at" --roster-pair "$roster_pair" \
    --requested-pair "$requested_pair" --cost-direction "$cost_direction" \
    --reviews "$reviews" --main-id "$current_main_id" --claim-epoch "$current_claim_epoch"
  rm -f "$cap_resolution"
  finalize_and_launch "$job" "$job" "$record_json" "$runtime" dispatch "$handshake_budget" "$wait"
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

finalize_and_launch() { # job id, chain id, record json, runtime, adapter verb, handshake budget, wait flag
  local job=$1 chain=$2 record_json=$3 runtime=$4 adapter_verb=$5 budget=$6 wait_flag=$7 patch
  lease_run_held "$current_claim_epoch" "$0" __record-setup --job "$job" --source "$record_json"
  release_cap_authority_lock
  release_chain_lock "$chain"; trap - EXIT
  lease_run_held "$current_claim_epoch" "$0" __launch --runtime "$runtime" --verb "$adapter_verb" \
    --job "$job" --tag "metasystem-job-$job" || {
    patch=$(mktemp "$record_locks/launch-failed.XXXXXX"); printf '{"error":"launch_failed"}\n' >"$patch"
    lease_run_held "$current_claim_epoch" "$0" __record-cas --job "$job" --expect pending --status failed --patch "$patch" || true
    rm -f "$patch"; return 3; }
  await_handshake "$job" "$budget" "$current_claim_epoch" || return 3
  if (( wait_flag )); then wait_for_job "$job"; return $?; fi
  printf '%s\n' "$job"
}

critique_exhaustion_action() { # root job, role, latest record, message, successor id, output manifest
  "$ms" job critique-exhaustion --repo "$root" --root-job "$1" --role "$2" \
    --latest "$3" --message "$4" --successor "$5" --output "$6"
}

record_critique_exhaustions() { # manifest
  local manifest=$1 target patch target_status
  while IFS=$'\t' read -r target patch; do
    target_status=$(json_field "$jobs/$target.json" status)
    lease_run_held "$current_claim_epoch" "$0" __record-cas --job "$target" \
      --expect "$target_status" --status "$target_status" --patch "$patch" \
      || die 1 "could not record the critique exhaustion successor on code-critic chain $target"
    rm -f "$patch"
  done < <("$ms" job exhaustion-patches --manifest "$manifest" --dir "$record_locks")
}

follow_up() {
  local job= message= wait=0 root_id latest status error session role runtime model model_key workspace reviewed_commit round child payload round_dir cap_resolution permission_json snapshot_json snapshot_path fallbacks signal handshake_budget resume_cap record_json mission mission_data lease mission_turn
  local resume_mode=resumed adapter_verb=follow-up delivery_content parent_round setup_json
  while (($#)); do
    case "$1" in
      --job) [[ $# -ge 2 ]] || { usage; exit 2; }; job=$2; shift 2 ;;
      --message) [[ $# -ge 2 ]] || { usage; exit 2; }; message=$2; shift 2 ;;
      --wait) wait=1; shift ;;
      *) usage; exit 2 ;;
    esac
  done
  valid_id "$job" && [[ -f "$message" && -f "$jobs/$job.json" ]] || { usage; exit 2; }
  lease_entry_check
  require_fresh_census
  report_plan_drift
  root_id=$(root_job_id "$job") || die 1 "cannot resolve the job chain"
  acquire_chain_lock "$root_id"
  # The trap must reference a GLOBAL: when set -e unwinds the owning
  # function, bash 5.x has already popped its locals when the EXIT trap
  # runs, and under set -u the trap dies on the expansion before releasing
  # anything (the Linux cap-authority leak, go-production-grade Phase 1).
  exit_cleanup_chain=$root_id
  trap 'release_chain_lock "$exit_cleanup_chain"' EXIT
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
  if [[ "$status" == completed || ( "$status" == failed && "$error" == protocol_error ) ]]; then :; else
    die 1 "follow-up requires the newest record to be completed or failed with protocol_error; use a fresh dispatch after pending, running, timeout, or process-lost"
  fi
  session=$(json_field "$latest" sessionId 2>/dev/null || true)
  [[ -n "$session" && "$session" != null ]] || die 1 "follow-up has no resumable session id; use the fresh-context embed fallback"
  role=$(json_field "$latest" role); runtime=$(json_field "$latest" runtime); model=$(json_field "$latest" requestedModel)
  workspace=$(json_field "$latest" workspaceRoot)
  round=$(( $(json_field "$latest" round) + 1 )); child="$root_id-r$round"
  [[ ! -e "$jobs/$child.json" ]] || die 1 "follow-up job id collision: $child"
  setup_json=$(mktemp "${TMPDIR:-/tmp}/metasystem-follow-pending-setup.XXXXXX")
  "$ms" job build-setup --output "$setup_json" --job "$child" --role "$role" \
    --parent "$root_id" --main-id "$current_main_id" --claim-epoch "$current_claim_epoch"
  lease_run_held "$current_claim_epoch" "$0" __record-create --job "$child" --source "$setup_json"
  rm -f "$setup_json"
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
  if [[ "$role" == implementer || "$role" == design-critic || "$role" == code-critic ]]; then
    exhaustion_patch=$(mktemp "$record_locks/exhaustion.XXXXXX")
    if ! exhaustion_action=$(critique_exhaustion_action \
      "$root_id" "$role" "$latest" "$message" "$child" "$exhaustion_patch" 2>&1); then
      die 1 "$exhaustion_action"
    fi
    if [[ "$exhaustion_action" == record ]]; then
      record_critique_exhaustions "$exhaustion_patch"
    fi
  fi
  mission=$(json_field "$latest" mission 2>/dev/null || true); [[ "$mission" == null ]] && mission=
  mission_turn=
  if [[ -n "$mission" ]]; then
    mission_data=$(resolve_mission "$mission")
    IFS='|' read -r mission lease mission_turn <<<"$mission_data"
  fi
  exit_cleanup_job=$child
  exit_cleanup_chain=$root_id
  trap 'code=$?; (( code == 0 )) || fail_setup_husk "$exit_cleanup_job"; release_cap_authority_lock; release_chain_lock "$exit_cleanup_chain"' EXIT
  acquire_cap_authority_lock
  cap_resolution=$(mktemp "$record_locks/follow-cap-resolution.XXXXXX")
  model_key=$(canonical_model "$model")
  [[ -n "$model_key" ]] || die 1 "requested model has no canonical cap-key form"
  authorize_job_cap "$child" "$role" "$runtime" "$model_key" "$mission" "" follow-up "$cap_resolution"
  permission_json=$(mktemp "$record_locks/follow-permissions.XXXXXX")
  json_field "$latest" permissions.requested >"$permission_json"
  snapshot_json=$(mktemp "$record_locks/follow-snapshot.XXXXXX")
  select_snapshot "$runtime" "$role" "$permission_json" "$snapshot_json"
  read_snapshot_fields "$snapshot_json"
  payload="$agents/$root_id"; round_dir="$payload/rounds/$round"; mkdir -p "$round_dir"
  delivery_content=$message
  if [[ "$resume_cap" != true ]]; then
    resume_mode=fresh-context
    adapter_verb=dispatch
    parent_round=$(json_field "$latest" round)
    delivery_content="$round_dir/fresh-context.md"
    {
      printf '# Prior brief\n\n'; cat "$payload/brief.md"
      printf '\n\n# Prior return\n\n'; cat "$payload/rounds/$parent_round/return.json"
      printf '\n\n# Correction\n\n'; cat "$message"
    } >"$delivery_content"
  fi
  input_bytes=$(enforce_inline_input_limit "$delivery_content" message)
  input_hash=$(sha256_file "$delivery_content")
  write_prompt "$round_dir/prompt.md" "$child" "$role" "$runtime" "$model" "$round" "${mission:-none}" "$delivery_content"
  record_json=$(mktemp "$record_locks/follow-record.XXXXXX")
  "$ms" job build-follow-record --output "$record_json" --parent "$latest" \
    --job "$child" --round "$round" --parent-job "$(basename "${latest%.json}")" \
    --snapshot "$snapshot_path" --fallbacks "$fallbacks" --signal "$signal" \
    --handshake-budget "$handshake_budget" --resume-mode "$resume_mode" \
    --input-bytes "$input_bytes" --input-hash "$input_hash" \
    --mission-turn "$mission_turn" --main-id "$current_main_id" \
    --claim-epoch "$current_claim_epoch" --cap-resolution "$cap_resolution"
  rm -f "$cap_resolution"
  finalize_and_launch "$child" "$root_id" "$record_json" "$runtime" "$adapter_verb" "$handshake_budget" "$wait"
}

status_job() {
  local job= status
  [[ ${1:-} == --job && $# -eq 2 ]] || { usage; exit 2; }; job=$2
  valid_id "$job" || { usage; exit 2; }
  [[ -e "$jobs/$job.json" ]] || return 6
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
  local job=
  [[ ${1:-} == --job && $# -eq 2 ]] || { usage; exit 2; }; job=$2
  valid_id "$job" && [[ -f "$jobs/$job.json" ]] || die 1 "unknown job: $job"
  lease_entry_check
  lease_run_held "$current_claim_epoch" \
    "$root/scripts/agents/adapters/$(json_field "$jobs/$job.json" runtime).sh" cancel --job "$job"
}

close_chain() {
  local job= root_id root_record status patch runner_closed=false
  [[ ${1:-} == --job && $# -ge 2 ]] || { usage; exit 2; }; job=$2; shift 2
  if [[ ${1:-} == --runner-closed && $# -eq 1 ]]; then
    runner_closed=true
    shift
  fi
  (($# == 0)) || { usage; exit 2; }
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
  # The read-dedupe-append-write runs under the record lock in one verb, so a
  # custody registration can never race a status transition.
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
  local job=$1 record="$jobs/$1.json" status patch
  [[ -f "$record" ]] || exit 1
  process_instance_tag=${process_instance_tag:-$job}
  acquire_lifecycle_lock_until "$job" 5 || exit 1
  status=$(json_field "$record" status)
  case "$status" in pending|running) ;; *) release_lifecycle_lock "$job"; exit 0 ;; esac
  wind_down_group "$record" || { release_lifecycle_lock "$job"; exit 1; }
  patch=$(mktemp "$record_locks/cancel.XXXXXX"); printf '{"error":null,"phase":"cancelled","groupDeathProvenAt":"%s"}\n' "$(now_iso)" >"$patch"
  record_cas "$job" "$status" cancelled "$patch" || true
  mirror_record "$job" || true
  release_lifecycle_lock "$job"
}

internal_critique_exhaustion() {
  local root_job= role= latest= message= successor= output=
  while (($#)); do
    case "$1" in
      --root-job) root_job=$2; shift 2 ;;
      --role) role=$2; shift 2 ;;
      --latest) latest=$2; shift 2 ;;
      --message) message=$2; shift 2 ;;
      --successor) successor=$2; shift 2 ;;
      --output) output=$2; shift 2 ;;
      *) exit 2 ;;
    esac
  done
  valid_id "$root_job" && valid_id "$successor" \
    && [[ "$role" == implementer || "$role" == design-critic || "$role" == code-critic ]] \
    && [[ -f "$latest" && -f "$message" && -n "$output" ]] || exit 2
  critique_exhaustion_action "$root_job" "$role" "$latest" "$message" "$successor" "$output"
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
  local runtime= verb= job= tag=
  while (($#)); do
    case "$1" in
      --runtime) runtime=$2; shift 2 ;;
      --verb) verb=$2; shift 2 ;;
      --job) job=$2; shift 2 ;;
      --tag) tag=$2; shift 2 ;;
      *) exit 2 ;;
    esac
  done
  [[ -n "$runtime" && ( "$verb" == dispatch || "$verb" == follow-up ) \
    && -n "$job" && -n "$tag" ]] || exit 2
  internal_authority holder-only "$job"
  launch_adapter "$runtime" "$verb" "$job" "$tag"
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
  follow-up) follow_up "$@" ;;
  status) status_job "$@" ;;
  cancel) cancel_job "$@" ;;
  close) close_chain "$@" ;;
  reap) reap_jobs "$@" ;;
  __record-create) internal_authority holder-only; "$ms" job record-create --root "$root" "$@" ;;
  __record-setup) internal_authority holder-only; "$ms" job record-setup --root "$root" "$@" ;;
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
  __register-custody)
    [[ ${1:-} == --job && $# -ge 2 ]] || exit 2
    internal_authority adapter-writer "$2"
    internal_register_custody "$@"
    ;;
  __critique-exhaustion) internal_critique_exhaustion "$@" ;;
  -h|--help) usage ;;
  *) usage; exit 2 ;;
esac
