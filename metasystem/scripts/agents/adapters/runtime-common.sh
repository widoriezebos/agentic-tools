#!/usr/bin/env bash

# Shared lifecycle plumbing for the real CLI adapters. Runtime command lines,
# event parsing, permission construction, and identity stay in each adapter.

adapter_common_init() { # runtime
  runtime=$1
  root=$(cd "$(dirname "${BASH_SOURCE[1]}")/../../.." && pwd -P)
  ms="${METASYSTEM_BIN:-$root/bin/metasystem}"
  dispatch="$root/scripts/agents/dispatch.sh"
  agents="$root/artifacts/agents"
  jobs="$agents/jobs"
}

field() { # json file, dotted field
  "$ms" json get --file "$1" --field "$2"
}

parse_supervisor_args() {
  job= gate= instance_tag=
  while (($#)); do
    case "$1" in
      --job) [[ $# -ge 2 ]] || return 2; job=$2; shift 2 ;;
      --start-gate) [[ $# -ge 2 ]] || return 2; gate=$2; shift 2 ;;
      --instance-tag) [[ $# -ge 2 ]] || return 2; instance_tag=$2; shift 2 ;;
      *) return 2 ;;
    esac
  done
  [[ -n "$job" && -n "$gate" && -n "$instance_tag" ]]
}

root_job_id() { # job id
  "$ms" adapter root-job --jobs "$jobs" --job "$1"
}

adapter_milliseconds_to_sleep() { # positive integer milliseconds
  local milliseconds=$1
  [[ "$milliseconds" =~ ^[1-9][0-9]*$ ]] \
    || { echo "$runtime adapter interval must be a positive integer in milliseconds" >&2; return 2; }
  printf '%d.%03d\n' "$((milliseconds / 1000))" "$((milliseconds % 1000))"
}

prepare_supervision() { # dispatch|follow-up and supervisor args
  local gate_poll
  adapter_verb=$1
  shift
  parse_supervisor_args "$@" || return 2
  record="$jobs/$job.json"
  gate_poll=$(adapter_milliseconds_to_sleep "${METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS:-10}") || return 2
  # Capped like every host's wait (script-adapters-11): a dispatcher that
  # dies before opening the gate must not leave an immortal supervisor
  # sleeping forever with no heartbeat and no handshake deadline.
  gate_deadline=$(( $(date +%s) + ${METASYSTEM_HOST_START_GATE_TIMEOUT_SEC:-10} ))
  while [[ ! -e "$gate" ]]; do
    (( $(date +%s) <= gate_deadline )) || { echo "start gate never opened: $gate" >&2; return 1; }
    sleep "$gate_poll"
  done
  round=$(field "$record" round)
  root_job=$(root_job_id "$job")
  round_dir="$agents/$root_job/rounds/$round"
  prompt="$round_dir/prompt.md"
  log="$jobs/$job.log"
  raw="$round_dir/raw.out"
  events="$round_dir/events.jsonl"
  heartbeat="$agents/hb/$job"
  effective="$round_dir/effective-permissions.json"
  schema="$round_dir/return-schema.v2.json"
  "$ms" schema materialize --root "$root" --role "$(field "$record" role)" \
    --version 2 --output "$schema"
  workspace=$(field "$record" workspaceRoot)
  requested_model=$(field "$record" requestedModel)
  requested_session=$(field "$record" sessionId 2>/dev/null || true)
  [[ "$requested_session" != null ]] || requested_session=
  handshake_done=0
  mkdir -p "$round_dir" "$(dirname "$heartbeat")"
  printf '%s adapter supervisor started value=%s\n' "$runtime" "$instance_tag" >"$log"
  printf '{"pid":%s,"pgid":%s,"instanceTag":"%s"}\n' "$$" "$$" "$instance_tag" >"$heartbeat"
  "$ms" adapter effective-init --record "$record" --output "$effective"
}

register_cli_custody() { # child pid
  local child_pid=$1 deadline=$((SECONDS + 5)) poll_sleep
  poll_sleep=$(adapter_milliseconds_to_sleep "${METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS:-20}") || return 2
  while kill -0 "$child_pid" 2>/dev/null; do
    if "$dispatch" __register-custody --job "$job" --pid "$child_pid"; then return 0; fi
    (( SECONDS < deadline )) || {
      echo "$runtime child custody registration ceiling reached for pid $child_pid" >&2
      return 1
    }
    sleep "$poll_sleep"
  done
  echo "$runtime child exited before custody identity was recorded" >&2
  return 1
}

record_actual_workspace_write_scope() {
  "$ms" adapter effective-workspace --effective "$effective" --workspace "$workspace"
}

fail_if_effective_wider_before_launch() {
  local mismatch
  mismatch=$("$ms" adapter permission-check --record "$record" --effective "$effective")
  [[ -z "$mismatch" ]] || {
    fail_pending "permissions_mismatch:$mismatch" handshake
    return 1
  }
}

record_handshake() { # session, turn, effective model
  local session=$1 turn=${2:-} model=${3:-$requested_model} signal
  [[ -n "$session" ]] || return 1
  if [[ -n "$requested_session" && "$session" != "$requested_session" ]]; then
    fail_pending resume_collision resume
    return 1
  fi
  signal=$(field "$record" sessionEstablishedSignal)
  "$dispatch" __handshake --job "$job" --session "$session" --turn "$turn" \
    --model "$model" --effective "$effective" --signal "$signal" || return 1
  session_id=$session
  effective_model=$model
  handshake_done=1
}

# The three-way decision after the CLI exits (script-adapters-09/D28), one
# copy of the protocol state machine: a late handshake adopts the observed
# session, an observed session that differs from the handshaken one is a
# resume collision, and a completed result's model is recorded when the
# runtime reports one (a runtime with no result model passes an empty
# result_model and the branch stays quiet — a capability fact, not drift).
settle_result_identity() { # observed session, observed turn, model for a late handshake, completed result model, usage file
  local observed_session=$1 observed_turn=$2 handshake_model=$3 result_model=$4 usage_file=$5
  if (( ! handshake_done )) && [[ -n "$observed_session" ]]; then
    record_handshake "$observed_session" "$observed_turn" "$handshake_model" || return 1
  elif (( handshake_done )) && [[ -n "$observed_session" && "$observed_session" != "$session_id" ]]; then
    finish_running failed resume_collision resume "$usage_file"
    return 1
  elif (( handshake_done )) && [[ -n "$result_model" ]]; then
    record_result_effective_model "$result_model" || return 1
  fi
}

record_result_effective_model() { # effective model reported by the completed runtime result
  local model=$1 patch="$round_dir/result-model-patch.json"
  [[ -n "$model" ]] || return 2
  "$ms" adapter model-patch --output "$patch" --model "$model"
  "$dispatch" __record-cas --job "$job" --expect running --status running --patch "$patch" || return 1
  effective_model=$model
}

write_patch() { # output, error|null, phase, usage file
  "$ms" adapter result-patch --output "$1" --error "$2" --phase "$3" --usage "$4"
}

fail_pending() { # error, phase, optional usage file
  local error=$1 phase=$2 usage_file=${3:-} patch status
  patch="$round_dir/pending-failure.json"
  write_patch "$patch" "$error" "$phase" "$usage_file"
  set +e
  "$dispatch" __record-cas --job "$job" --expect pending --status failed --patch "$patch"
  status=$?
  set -e
  [[ $status -eq 0 || $status -eq 3 ]]
}

finish_running() { # completed|failed, error|null, phase, usage file
  local target=$1 error=$2 phase=$3 usage_file=$4 patch status
  patch="$round_dir/terminal-patch.json"
  write_patch "$patch" "$error" "$phase" "$usage_file"
  set +e
  "$dispatch" __record-cas --job "$job" --expect running --status "$target" --patch "$patch"
  status=$?
  set -e
  [[ $status -eq 0 || $status -eq 3 ]]
}

finish_protocol_error() { # violation file
  local violation_file=$1 status
  set +e
  "$dispatch" __protocol-error --job "$job" --expect running --violation-file "$violation_file"
  status=$?
  set -e
  [[ $status -eq 0 || $status -eq 3 ]]
}

# F4 (D32): the custodian enforces the record's OWN deadlines from inside.
# The deadlines are immutable once stamped, so they are read ONCE, bounded;
# a record that cannot be read within the bound fails closed (sweep + die)
# rather than waiting unbounded on an unreadable record. handshakeDeadline
# is epoch seconds; capDeadline is a UTC ISO second, compared lexically
# against a same-format now — sound for one fixed format.
cache_record_deadlines() {
  [[ -z "${deadlines_cached:-}" ]] || return 0
  local attempt
  for attempt in 1 2 3; do
    if handshake_deadline_epoch=$("$ms" json get --file "$record" --field handshakeDeadline --default '' 2>/dev/null); then
      cap_deadline_iso=$("$ms" json get --file "$record" --field capDeadline --default '' 2>/dev/null) || cap_deadline_iso=
      deadlines_cached=1
      [[ "$handshake_deadline_epoch" =~ ^[1-9][0-9]*$ ]] || handshake_deadline_epoch=
      return 0
    fi
    sleep 0.2
  done
  printf '%s deadline cache failed: record unreadable; failing closed\n' \
    "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >>"$log" 2>/dev/null || true
  sweep_kill_domain || true
  exit 1
}

# The kill domain is this supervisor's own process group minus itself
# (membership survives reparenting, so orphaned grandchildren stay
# enumerable). Per-pid sweeps; the return value is the DEATH PROOF: 0 only
# when no member but this process remains. An indeterminable enumeration
# refuses — a sweep must never act on an undercount.
sweep_kill_domain() {
  local signal members pid deadline
  for signal in TERM KILL; do
    members=$("$ms" proc group-members --pgid $$ --except $$) || return 1
    [[ -n "$members" ]] || return 0
    while IFS= read -r pid; do
      [[ -n "$pid" ]] && kill -"$signal" "$pid" 2>/dev/null || true
    done <<<"$members"
    deadline=$(( $(date +%s) + 2 ))
    while (( $(date +%s) < deadline )); do
      members=$("$ms" proc group-members --pgid $$ --except $$) || return 1
      [[ -n "$members" ]] || return 0
      sleep 0.05
    done
  done
  members=$("$ms" proc group-members --pgid $$ --except $$) || return 1
  [[ -z "$members" ]]
}

# One expired deadline's enforcement: stand down BEFORE ANY SIGNAL on the
# in-process handshake state (sessionId is published by THIS process, so
# the check is single-writer and deterministic); sweep; land the terminal
# verdict only behind the death proof. An unproven domain leaves the
# record nonterminal — emitted, retried next tick, census-visible.
enforce_expired_deadline() { # handshake|cap, cli child pid
  local kind=$1 cli_child=$2
  if [[ "$kind" == handshake ]] && (( handshake_done )); then
    return 1 # the wait was won; nothing to enforce, nothing signaled
  fi
  # The direct child FIRST, through the path that also REAPS it: a
  # non-interactive shell reaps a background child only at an explicit
  # wait, and an unreaped zombie keeps its group membership — the sweep
  # could never prove a domain that still holds our own zombie.
  terminate_cli_child "$cli_child"
  if ! sweep_kill_domain; then
    printf '%s %s-deadline sweep left the kill domain unproven; record stays nonterminal\n' \
      "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$kind" >>"$log" 2>/dev/null || true
    return 1
  fi
  if [[ "$kind" == handshake ]]; then
    fail_pending handshake_timeout handshake || true
  else
    # The reaper's and waiter's spelling: one record reads one way.
    finish_running timeout budget-cap supervision "" || true
  fi
  printf '%s %s deadline enforced by the custodian (D32)\n' \
    "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$kind" >>"$log" 2>/dev/null || true
  exit 0
}

check_record_deadlines() { # cli child pid; one tick's verdicts; may not return
  local cli_child=$1 now_epoch now_iso
  cache_record_deadlines
  if [[ -n "${handshake_deadline_epoch:-}" ]] && (( ! handshake_done )); then
    now_epoch=$(date +%s)
    (( now_epoch <= handshake_deadline_epoch )) || enforce_expired_deadline handshake "$cli_child" || true
  fi
  if [[ -n "${cap_deadline_iso:-}" ]] && (( handshake_done )); then
    now_iso=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    [[ ! "$now_iso" > "$cap_deadline_iso" ]] || enforce_expired_deadline cap "$cli_child" || true
  fi
}

wait_for_cli() { # child pid; sets cli_status and keeps liveness sidecars fresh
  local child=$1 tick=0 heartbeat_sleep
  heartbeat_sleep=$(adapter_milliseconds_to_sleep "${METASYSTEM_HEARTBEAT_INTERVAL_MS:-100}") || return 2
  while kill -0 "$child" 2>/dev/null; do
    touch "$heartbeat"
    check_record_deadlines "$child"
    tick=$((tick + 1))
    (( tick % 10 != 0 )) || touch "$log"
    sleep "$heartbeat_sleep"
  done
  set +e
  wait "$child"
  cli_status=$?
  set -e
}

terminate_cli_child() { # exact child pid owned by this adapter
  local child=$1 deadline
  kill -TERM "$child" 2>/dev/null || true
  deadline=$(( $(date +%s) + 2 ))
  while kill -0 "$child" 2>/dev/null && (( $(date +%s) < deadline )); do sleep 0.05; done
  kill -KILL "$child" 2>/dev/null || true
  wait "$child" 2>/dev/null || true
}

# normalize_return is plumbing the fixtures also consume directly; the
# adjudication verb runs the same normalization internally.
normalize_return() { # candidate file, optional transcript file
  local candidate=$1 transcript=${2:-}
  "$ms" adapter normalize-return --candidate "$candidate" --transcript "$transcript" \
    --record "$record" --output "$round_dir/return.json" \
    --markdown "$round_dir/return.md" --session "$session_id"
}

# The terminal-outcome state machine lives in `adapter adjudicate-turn`
# (script-adapters-01/D24): the engine validates the candidate, chooses
# every error code and phase name, and decides whether the one bounded
# repair turn runs. This file keeps only what must stay shell — the CAS
# through dispatch.sh's lease-held re-exec, the repair CLI launch, and the
# per-runtime usage/settle hooks. One repair attempt, in the SAME session:
# a reply can be perfect work in the wrong shape, but the harness never
# renames fields to make a return validate.
adjudicate_turn() { # stage, extra flags...
  local stage=$1; shift
  "$ms" adapter adjudicate-turn --stage "$stage" --root "$root" --job "$job" \
    --record "$record" --session "${session_id:-}" --schema "$schema" \
    --return "$round_dir/return.json" --markdown "$round_dir/return.md" \
    --violation "$round_dir/protocol-violation.txt" \
    --repair-prompt "$round_dir/repair-1.prompt.md" "$@"
}

complete_from_cli() { # cli status, usage file, candidate file, optional transcript
  local status=$1 usage_file=$2 candidate=$3 transcript=${4:-} violation="$round_dir/protocol-violation.txt"
  local repair_available=0 settle_available=0 verdict repair_rc=0
  local initial_args=(--cli-status "$status" --candidate "$candidate" --transcript "$transcript")
  if declare -F runtime_repair_turn >/dev/null 2>&1 \
      && (( ${return_repairs:-0} == 0 )) && [[ -n "${session_id:-}" ]]; then
    repair_available=1
    initial_args+=(--repair-available)
  fi
  declare -F runtime_settle_after_repair >/dev/null 2>&1 && settle_available=1
  (( handshake_done )) && initial_args+=(--handshake-done)

  verdict=$(adjudicate_turn initial "${initial_args[@]}")
  case "$verdict" in
    "finish completed null completed")
      rm -f "$violation"
      finish_running completed null completed "$usage_file"
      return 0 ;;
    fail-pending\ *)
      set -- $verdict
      fail_pending "$2" "$3" "$usage_file"
      return 1 ;;
    finish\ *)
      set -- $verdict
      finish_running "$2" "$3" "$4" "$usage_file"
      return 1 ;;
    protocol-error)
      cat "$violation" >>"$log"
      finish_protocol_error "$violation"
      return 1 ;;
    repair) ;;
    *)
      printf 'adjudicate-turn returned an unknown verdict: %s\n' "$verdict" >>"$log"
      return 1 ;;
  esac

  cat "$violation" >>"$log"
  printf '%s return repair attempt 1: reply did not validate, asking again in session %s\n' \
    "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$session_id" >>"$log"
  return_repairs=1
  runtime_repair_turn "$round_dir/repair-1.prompt.md" "$round_dir/repair-1.out" || repair_rc=$?
  # The repair RAN, whether or not it produced a usable reply. Record it and
  # re-fence its usage BEFORE branching on the outcome: a failed repair still
  # spent provider budget on a cumulative-usage runtime.
  record_return_repairs 1
  if declare -F runtime_usage_after_repair >/dev/null 2>&1; then
    runtime_usage_after_repair "$usage_file" || true
  fi
  local after_args=(--repair-rc "$repair_rc" --repair-candidate "$round_dir/repair-1.out")
  (( settle_available )) && after_args+=(--settle-available)
  verdict=$(adjudicate_turn after-repair "${after_args[@]}")
  if [[ "$verdict" == settle ]]; then
    # The repair transcript is now the final turn, so it — not the
    # pre-repair transcript — is authoritative for session and model
    # identity.
    if runtime_settle_after_repair; then
      verdict=$(adjudicate_turn settle-result --ok)
    else
      verdict=$(adjudicate_turn settle-result)
    fi
  fi
  case "$verdict" in
    "finish completed null completed")
      printf '%s return repaired in session %s; the first reply is kept as evidence\n' \
        "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$session_id" >>"$log"
      rm -f "$violation"
      finish_running completed null completed "$usage_file"
      return 0 ;;
    finish\ *)
      set -- $verdict
      finish_running "$2" "$3" "$4" "$usage_file"
      return 1 ;;
    *)
      cat "$violation" >>"$log"
      finish_protocol_error "$violation"
      return 1 ;;
  esac
}

record_return_repairs() { # count
  local patch="$round_dir/repair-count-patch.json"
  "$ms" adapter repairs-patch --output "$patch" --count "$1"
  "$dispatch" __record-cas --job "$job" --expect running --status running --patch "$patch" || true
}

configuration_identity() { # runtime version declared settings files
  local identity_runtime=$1 identity_version=$2 identity_filter
  shift 2
  # The filter file is DECLARED, never constructed (agnosticism code
  # critique finding 1): the bytes validation checks must be the bytes
  # live identity hashes. Fail closed when the runtime declares none.
  identity_filter=$("$ms" runtime config-identity-filter "$identity_runtime") || {
    echo "no config identity filter declared for $identity_runtime" >&2
    return 1
  }
  "$ms" config identity \
    --runtime "$identity_runtime" \
    --version "$identity_version" \
    --filter "$root/scripts/agents/adapters/$identity_filter" \
    "$@"
}

configuration_identity_field() { # identity JSON, field
  "$ms" json get --value "$1" --field "$2"
}

write_capability_snapshot() { # runtime version hash transports caps permissions envelope-enforcement per-key-hashes
  local snapshot_runtime=$1 version=$2 config_hash=$3 transports=$4 caps=$5 permissions=$6
  local envelope_enforcement=$7 config_key_hashes=$8
  mkdir -p "$agents/capabilities"
  "$ms" adapter capability-snapshot \
    --dir "$agents/capabilities" \
    --runtime "$snapshot_runtime" \
    --version "$version" \
    --config-hash "$config_hash" \
    --transports "$transports" \
    --capabilities "$caps" \
    --permissions "$permissions" \
    --envelope-enforcement "$envelope_enforcement" \
    --config-key-hashes "$config_key_hashes"
}

# The full-contract self-test lives in Go (`adapter selftest-run`,
# script-adapters-05/D27): the orchestration execs dispatch.sh and this
# adapter script, and the decisions — model validation, the denial taxonomy,
# session equality, the parsed evidence assertions — are the verb's. The
# adapter keeps only its runtime-specific knobs: how long one turn may take
# (a property of the RUNTIME, not the contract) and whether a denied tool
# ends the turn (those runtimes run the permission legs as separate turns).
run_full_contract_selftest() { # native|unavailable|metered, optional probe name
  local usage_expectation=$1 probe=${2:-} extra=()
  [[ -n "$probe" ]] && extra+=(--probe "$probe")
  [[ "${selftest_denial_ends_turn:-0}" == 1 ]] && extra+=(--denial-ends-turn)
  "$ms" adapter selftest-run --root "$root" --runtime "$runtime" --adapter "$0" \
    --usage "$usage_expectation" --turn-ceiling-sec "${selftest_turn_ceiling_sec:-240}" \
    ${extra[@]+"${extra[@]}"}
}
