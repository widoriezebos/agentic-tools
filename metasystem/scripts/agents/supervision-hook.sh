#!/usr/bin/env bash
set -euo pipefail

# Resolution order is part of the hook boundary: (1) reject malformed event
# and runtime names without an engine; (2) a missing engine blocks a Stop
# because no safe verdict can be made; (3) with the engine present, reject an
# unregistered runtime; (4) expand the runtime's optional session environment
# indirectly, never with eval; (5) resolve cwd from the payload, then that
# environment variable's nonempty value, then PWD. The runtime argument's
# shape remains open to newly registered runtimes. The recovery-only scheduler
# entry is operator-owned and can be printed with `metasystem up
# --print-scheduler-entry`; this hook never installs host state.
runtime=${1:-}
event=${2:-}
[[ "$runtime" =~ ^[a-z][a-z0-9-]{0,31}$ ]] || exit 2
case "$event" in start|stop|end) ;; *) exit 2 ;; esac

emit_raw_stop_block() {
  printf '%s\n' '{"decision":"block","reason":"Metasystem could not prove that stopping is safe; stopping is refused."}'
}
raw_missing_engine_stop='{"decision":"block","reason":"Metasystem engine missing, so stopping safety cannot be judged; reinstall or rebuild bin/metasystem before stopping."}'

# Claude Code eventually caps repeated Stop-hook blocks. This hook accepts that
# harness boundary and does not try to defeat it; true impossibility is owned by
# goal idle-every-runtime-enforcement through runtime-independent steward
# re-engagement.

# Claude gives the complete Stop hook five seconds. Run the whole Stop body as
# one supervised child so arming, health, digest, watchdog, ledger fetch, and
# verdict time all spend the same budget. The parent retains one second to
# emit a provider-level refusal and exit successfully when the child overruns.
if [[ "$event" == stop && "${METASYSTEM_STOP_DEADLINE_PARENT:-}" != "$PPID" ]]; then
  deadline_dir=
  deadline_dir=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-stop-deadline.XXXXXX" 2>/dev/null) \
    || deadline_dir=$(mktemp -d "/tmp/metasystem-stop-deadline.XXXXXX" 2>/dev/null) \
    || true
  if [[ -z "$deadline_dir" ]]; then
    printf '%s\n' '{"systemMessage":"Metasystem could not stage the Stop payload or update its refusal record; stopping is allowed so this hook failure cannot repeat forever."}'
    exit 0
  fi
  deadline_stdout=$deadline_dir/stdout
  deadline_stderr=$deadline_dir/stderr
  deadline_payload=$deadline_dir/payload
  deadline_script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
  deadline_harness_root=$(cd "$deadline_script_dir/../.." && pwd -P)
  deadline_validator="${METASYSTEM_BIN:-$deadline_harness_root/bin/metasystem}"
  deadline_canonical=$deadline_harness_root/bin/metasystem
  if ! command cat >"$deadline_payload"; then
    printf '%s\n' '{"systemMessage":"Metasystem could not stage the Stop payload or update its refusal record; stopping is allowed so this hook failure cannot repeat forever."}'
    rm -f "$deadline_stdout" "$deadline_stderr" "$deadline_payload" || true
    rmdir "$deadline_dir" 2>/dev/null || true
    exit 0
  fi
  deadline_started=$SECONDS
  METASYSTEM_STOP_DEADLINE_PARENT=$$ bash "${BASH_SOURCE[0]}" "$runtime" "$event" \
    <"$deadline_payload" >"$deadline_stdout" 2>"$deadline_stderr" &
  deadline_worker=$!
  deadline_expires=$((deadline_started + 4))

  # Resolve record coordinates alongside the worker, never ahead of it. The
  # engine parser is authoritative when it finishes inside the worker's wait;
  # the restricted shell parser keeps the timeout path independent of a slow
  # engine and accepts the ordinary unescaped session and cwd payload shape.
  deadline_resolution=$deadline_dir/resolution
  deadline_resolution_ready=$deadline_dir/resolution.ready
  deadline_resolver=
  if [[ -x "$deadline_canonical" ]]; then
    (
      resolver_session=$("$deadline_canonical" json get --file "$deadline_payload" --field session_id 2>/dev/null) || exit 1
      resolver_cwd=$("$deadline_canonical" json get --file "$deadline_payload" --field cwd 2>/dev/null) || exit 1
      printf '%s\n%s\n' "$resolver_session" "$resolver_cwd" >"$deadline_resolution"
      mv "$deadline_resolution" "$deadline_resolution_ready"
    ) &
    deadline_resolver=$!
  fi
  deadline_session=$(sed -n 's/.*"session_id"[[:space:]]*:[[:space:]]*"\([^"\\]*\)".*/\1/p' "$deadline_payload" | head -1)
  deadline_cwd=$(sed -n 's/.*"cwd"[[:space:]]*:[[:space:]]*"\([^"\\]*\)".*/\1/p' "$deadline_payload" | head -1)
  [[ -n "$deadline_session" ]] || deadline_session="session-$$"
  deadline_record=
  deadline_record_failure="the installed canonical engine and shell fallback could not resolve the Stop session and repository"
  deadline_coordinates_from_engine=false
  deadline_resolve_record() {
    local slug
    [[ -n "$deadline_session" && -n "$deadline_cwd" ]] || return 1
    deadline_repo=$(git -C "$deadline_cwd" rev-parse --show-toplevel 2>/dev/null || true)
    [[ -n "$deadline_repo" ]] || return 1
    deadline_repo=$(cd "$deadline_repo" && pwd -P)
    slug=$(printf '%s' "$deadline_session" | tr '[:upper:]' '[:lower:]' |
      sed -E 's/[^a-z0-9._-]+/-/g; s/^[-.]+//; s/[-.]+$//')
    [[ -n "$slug" ]] || slug=session
    deadline_record="$deadline_repo/artifacts/agents/supervision/stop-refusals/$slug.json"
    deadline_record_failure=
  }
  deadline_resolve_record || true
  deadline_log_stop_outcome() {
    local outcome=$1 supervision_dir
    [[ -n "${deadline_repo:-}" ]] || return 0
    supervision_dir="$deadline_repo/artifacts/agents/supervision"
    mkdir -p "$supervision_dir" || true
    printf '%s stop response outcome=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
      "$outcome" >>"$supervision_dir/hooks.log" 2>/dev/null || true
  }
  deadline_capture_engine_coordinates() {
    local first second
    [[ "$deadline_coordinates_from_engine" == false && -f "$deadline_resolution_ready" ]] || return 0
    first=$(sed -n '1p' "$deadline_resolution_ready")
    second=$(sed -n '2p' "$deadline_resolution_ready")
    if [[ -n "$first" && -n "$second" ]]; then
      deadline_session=$first
      deadline_cwd=$second
      deadline_record=
      deadline_record_failure="the installed canonical engine could not resolve the Stop repository"
      deadline_resolve_record || true
    fi
    deadline_coordinates_from_engine=true
  }
  deadline_stop_resolver() {
    local resolver_command
    [[ -n "$deadline_resolver" ]] || return 0
    if kill -0 "$deadline_resolver" 2>/dev/null; then
      resolver_command=$(ps -p "$deadline_resolver" -o command= 2>/dev/null || true)
      if [[ "$resolver_command" == *"${BASH_SOURCE[0]}"* || "$resolver_command" == *supervision-hook.sh* ]]; then
        kill -KILL "$deadline_resolver" 2>/dev/null || true
      fi
    fi
    wait "$deadline_resolver" 2>/dev/null || true
    deadline_resolver=
  }
  deadline_running() {
    local state
    state=$(ps -p "$deadline_worker" -o stat= 2>/dev/null || true)
    [[ -n "$state" && "$state" != Z* ]]
  }
  while deadline_running && (( SECONDS < deadline_expires )); do
    deadline_capture_engine_coordinates
    sleep 0.05
  done
  deadline_capture_engine_coordinates
  if ! deadline_running; then
    deadline_stop_resolver
    deadline_rc=0
    wait "$deadline_worker" || deadline_rc=$?
    command cat "$deadline_stderr" >&2 || true
    deadline_decision=
    deadline_reason=
    deadline_message=
    deadline_decision_rc=0
    deadline_reason_rc=0
    deadline_message_rc=0
    deadline_shape_rc=0
    deadline_valid=false
    if (( deadline_rc == 0 )) && [[ -x "$deadline_validator" ]]; then
      deadline_decision=$("$deadline_validator" json get --file "$deadline_stdout" --field decision 2>/dev/null) \
        || deadline_decision_rc=$?
      deadline_reason=$("$deadline_validator" json get --file "$deadline_stdout" --field reason 2>/dev/null) \
        || deadline_reason_rc=$?
      deadline_message=$("$deadline_validator" json get --file "$deadline_stdout" --field systemMessage 2>/dev/null) \
        || deadline_message_rc=$?
      deadline_unknown=$("$deadline_validator" json strip --file "$deadline_stdout" \
        --key decision --key reason --key systemMessage 2>/dev/null) || deadline_shape_rc=$?
      deadline_reason_object=$("$deadline_validator" json strip --file "$deadline_stdout" \
        --key decision --key systemMessage 2>/dev/null) || deadline_shape_rc=$?
      deadline_message_object=$("$deadline_validator" json strip --file "$deadline_stdout" \
        --key decision --key reason 2>/dev/null) || deadline_shape_rc=$?
      deadline_reason_string=false
      deadline_message_string=false
      if (( deadline_reason_rc == 0 )) && grep -q '^  "reason": "' <<<"$deadline_reason_object"; then
        deadline_reason_string=true
      fi
      if (( deadline_message_rc == 0 )) && grep -q '^  "systemMessage": "' <<<"$deadline_message_object"; then
        deadline_message_string=true
      fi
      if (( deadline_shape_rc == 0 )) && [[ "$deadline_unknown" == '{}' ]] &&
          { [[ "$deadline_decision" == block && -n "$deadline_reason" && "$deadline_reason_string" == true &&
               ( "$deadline_message_rc" -ne 0 || ( -n "$deadline_message" && "$deadline_message_string" == true ) ) ]] ||
            [[ "$deadline_decision_rc" -ne 0 && "$deadline_reason_rc" -ne 0 && -n "$deadline_message" &&
               "$deadline_message_string" == true ]]; }; then
        deadline_valid=true
      fi
    elif (( deadline_rc == 0 )); then
      deadline_raw=$(command cat "$deadline_stdout" 2>/dev/null || true)
      [[ "$deadline_raw" == "$raw_missing_engine_stop" ]] && deadline_valid=true
    fi
    if (( deadline_rc != 0 )) || [[ "$deadline_valid" != true ]]; then
      deadline_log_stop_outcome invalid-worker-output-block
      emit_raw_stop_block
    else
      command cat "$deadline_stdout" || true
    fi
    rm -f "$deadline_stdout" "$deadline_stderr" "$deadline_payload" \
      "$deadline_resolution" "$deadline_resolution_ready" || true
    rmdir "$deadline_dir" 2>/dev/null || true
    exit 0
  fi

  deadline_stop_resolver
  deadline_command=$(ps -p "$deadline_worker" -o command= 2>/dev/null || true)
  if [[ "$deadline_command" == *"${BASH_SOURCE[0]}"* || "$deadline_command" == *supervision-hook.sh* ]]; then
    kill -TERM "$deadline_worker" 2>/dev/null || true
  fi
  for _deadline_stop_attempt in {1..10}; do
    deadline_running || break
    sleep 0.02
  done
  if deadline_running; then
    deadline_command=$(ps -p "$deadline_worker" -o command= 2>/dev/null || true)
    if [[ "$deadline_command" == *"${BASH_SOURCE[0]}"* || "$deadline_command" == *supervision-hook.sh* ]]; then
      kill -KILL "$deadline_worker" 2>/dev/null || true
    fi
  fi
  wait "$deadline_worker" 2>/dev/null || true
  deadline_cause='stop deadline expired'
  deadline_remedy='A human or steward must restore supervision outside this seat, then retry.'
  deadline_detail='Metasystem Stop deadline expired before a safe turn verdict; stopping is refused.'
  deadline_response=
  if [[ -n "$deadline_record" ]]; then
    deadline_response=$("$deadline_canonical" report stop-block \
      --refusal-record "$deadline_record" --session "$deadline_session" \
      --cause "$deadline_cause" --remedy "$deadline_remedy" "$deadline_detail" 2>/dev/null) || \
      deadline_record_failure="the stop-refusal record could not be read or atomically updated"
  fi
  if [[ -n "$deadline_response" && -z "$deadline_record_failure" ]]; then
    deadline_log_stop_outcome deadline-expired-block
    printf '%s\n' "$deadline_response"
  else
    deadline_log_stop_outcome deadline-expired-record-failure-allow
    printf '%s\n' '{"systemMessage":"Metasystem could not update the stop-refusal record; stopping is allowed so record failure cannot recreate the refusal loop. Cause: stop deadline expired. Remedy: A human or steward must restore supervision outside this seat, then retry."}'
  fi
  rm -f "$deadline_stdout" "$deadline_stderr" "$deadline_payload" \
    "$deadline_resolution" "$deadline_resolution_ready" || true
  rmdir "$deadline_dir" 2>/dev/null || true
  exit 0
fi

# Executables resolve before payload work so a missing engine can return the
# provider-level Stop refusal without depending on temporary storage.
script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
harness_root=$(cd "$script_dir/../.." && pwd -P)
ms="${METASYSTEM_BIN:-$harness_root/bin/metasystem}"
if [[ ! -x "$ms" ]]; then
  if [[ "$event" == stop ]]; then
    printf '%s\n' "$raw_missing_engine_stop"
  fi
  exit 0
fi
registered_runtimes=$("$ms" runtime list) || {
  runtime_list_rc=$?
  echo "supervision hook refused: runtime registry query failed (exit $runtime_list_rc)" >&2
  exit "$runtime_list_rc"
}
grep -Fxq "$runtime" <<<"$registered_runtimes" || {
  echo "supervision hook refused: runtime '$runtime' is not registered" >&2
  exit 2
}

payload=$(mktemp "${TMPDIR:-/tmp}/metasystem-supervision-hook.XXXXXX")
trap 'rm -f "$payload"' EXIT
cat >"$payload"
if [[ "$event" == stop ]]; then
  # Missing optional payload fields retain their documented fallbacks, but an
  # unreadable payload must not be mistaken for an empty one at turn end.
  "$ms" json get --file "$payload" --field __metasystem_shape_probe --default "" >/dev/null
fi

read_payload() {
  "$ms" json get --file "$payload" --field "$1" 2>/dev/null || true
}

cwd=$(read_payload cwd)
if [[ -z "$cwd" ]]; then
  session_env_rc=0
  session_env=$("$ms" runtime session-env "$runtime" 2>/dev/null) || session_env_rc=$?
  if (( session_env_rc == 0 )) && [[ "$session_env" =~ ^[A-Z][A-Z0-9_]*$ && -n "${!session_env:-}" ]]; then
    cwd=${!session_env}
  elif (( session_env_rc <= 1 )); then
    # exit 1 is the DECLARED absent capability: fall back to PWD.
    cwd=$PWD
  else
    # An operational query failure must not run Stop decisions against
    # a guessed cwd. A Stop parent converts this silent return to a refusal.
    exit 0
  fi
fi
repo=$(git -C "$cwd" rev-parse --show-toplevel 2>/dev/null) || exit 0
repo=$(cd "$repo" && pwd -P)
# Repository scope and mutable metasystem state coincide after adoption. In
# the self-hosting template, the repository contains the installation and the
# installation owns its state. Resolve that distinction once so holder
# classification and the turn verdict cannot inspect different lease trees.
state_root=$repo
template_marker=$repo/development/metasystem-design.md
template_installation=$repo/metasystem
if [[ -f "$template_marker" ]]; then
  if [[ "$harness_root" != "$template_installation" || ! -f "$harness_root/metasystem.conf" ]]; then
    [[ "$event" != stop ]] || exit 1
  else
    state_root=$harness_root
  fi
fi
session=$(read_payload session_id)
[[ -n "$session" ]] || session="session-$PPID"
# Session hygiene happens ONCE at this boundary (goal-system GOAL-04):
# the runtime's string is untrusted input; anything not matching the safe
# shape becomes its sha256 hex, and every downstream use rides the result.
if ! [[ "$session" =~ ^[A-Za-z0-9._-]{1,128}$ ]]; then
  session=$(printf '%s' "$session" | "$ms" util sha256)
fi

hook_generation=
hook_attempt_seq=
health_line=
checkin_tail=
digest_message=
digest_cursor=
digest_prefix=
hook_evidence_failure=
stop_failure=
record_stop_failure() { # fixed diagnostic
  [[ -n "$stop_failure" ]] || stop_failure=$1
}
if [[ "$event" == stop ]]; then
  turn_key_rc=0
  turn_key=$({ printf '%s\n' "$session"; command cat "$payload"; } | "$ms" util sha256) || turn_key_rc=$?
  if (( turn_key_rc != 0 )) || [[ -z "$turn_key" ]]; then
    hook_evidence_failure="HEALTH unknown — hook-freshness=unknown (turn evidence could not be prepared)"
    record_stop_failure "turn evidence could not be prepared"
  else
    hook_attempt_rc=0
    hook_attempt=$(
      "$ms" steward hook-attempt --repo "$repo" --pid "$$" --turn-key "$turn_key" 2>/dev/null
    ) || hook_attempt_rc=$?
    if (( hook_attempt_rc != 0 )) || [[ -z "$hook_attempt" ]]; then
      hook_evidence_failure="HEALTH unknown — hook-freshness=unknown (attempt evidence could not be recorded)"
      record_stop_failure "attempt evidence could not be recorded"
    else
      hook_generation=$("$ms" json get --value "$hook_attempt" --field generation 2>/dev/null || true)
      hook_attempt_seq=$("$ms" json get --value "$hook_attempt" --field attemptSeq 2>/dev/null || true)
      if ! [[ "$hook_generation" =~ ^[1-9][0-9]*$ && "$hook_attempt_seq" =~ ^[1-9][0-9]*$ ]]; then
        hook_evidence_failure="HEALTH unknown — hook-freshness=unknown (attempt evidence was unreadable)"
        record_stop_failure "attempt evidence was unreadable"
      fi
    fi
  fi
fi

# Runtime signatures are anchored on the executable, so an intermediate
# `/bin/sh -c` does not impersonate the runtime merely because its arguments
# name this hook. Start at the immediate parent and let the process owner walk.
identity=$("$ms" proc find-ancestor --repo "$repo" --pid "$PPID" --runtime "$runtime" 2>/dev/null || true)
main_id=
main_class=
main_holder=false
identity_pid=
identity_started=
if [[ -n "$identity" ]]; then
  identity_pid=$("$ms" json get --value "$identity" --field pid 2>/dev/null || true)
  identity_started=$("$ms" json get --value "$identity" --field pidStartedAt 2>/dev/null || true)
  if ! [[ "$identity_pid" =~ ^[1-9][0-9]*$ && "$identity_started" =~ ^[1-9][0-9]*$ ]]; then
    record_stop_failure "the runtime identity was unreadable"
    identity=
    identity_pid=
    identity_started=
  fi
else
  # Recorded fallback: a hook may run in a test harness or runtime wrapper
  # whose authenticated main was announced explicitly. Classification returns
  # that exact announcement; an unannounced process gains nothing here.
  parent_view_rc=0
  parent_view=$("$ms" lease classify --root "$state_root" --metasystem-root "$harness_root" --caller-pid "$PPID" 2>/dev/null) || parent_view_rc=$?
  parent_class=$("$ms" json get --value "$parent_view" --field class 2>/dev/null || true)
  if (( parent_view_rc != 0 )) || [[ -z "$parent_class" ]]; then
    record_stop_failure "the fallback runtime identity could not be classified"
  elif [[ "$parent_class" == MAIN ]]; then
    identity_pid=$("$ms" json get --value "$parent_view" --field announcement.pid 2>/dev/null || true)
    identity_started=$("$ms" json get --value "$parent_view" --field announcement.pidStartedAt 2>/dev/null || true)
    [[ "$identity_pid" =~ ^[1-9][0-9]*$ && "$identity_started" =~ ^[1-9][0-9]*$ ]] \
      && identity=recorded-main
    [[ -n "$identity" ]] || record_stop_failure "the fallback runtime identity was unreadable"
  fi
fi
if [[ -n "$identity_pid" ]]; then
  lease_view_rc=0
  lease_view=$("$ms" lease classify --root "$state_root" --metasystem-root "$harness_root" --caller-pid "$identity_pid" 2>/dev/null) || lease_view_rc=$?
  if (( lease_view_rc != 0 )) || [[ -z "$lease_view" ]]; then
    record_stop_failure "the checkout holder could not be classified"
  else
    main_id=$("$ms" json get --value "$lease_view" --field mainId 2>/dev/null || true)
    main_class=$("$ms" json get --value "$lease_view" --field class 2>/dev/null || true)
    main_holder=$("$ms" json get --value "$lease_view" --field holder 2>/dev/null || true)
    if [[ -z "$main_class" || ( "$main_holder" != true && "$main_holder" != false ) ]]; then
      record_stop_failure "the checkout holder classification was unreadable"
    fi
  fi
fi

surface_json() { # message
  local rendered parsed
  rendered=$("$ms" json object "systemMessage=$1")
  parsed=$("$ms" json get --value "$rendered" --field systemMessage)
  [[ -n "$rendered" && -n "$parsed" ]] || return 1
  printf '%s\n' "$rendered"
}

stop_block_json() { # system message, reason
  local rendered decision reason
  rendered=$("$ms" report stop-block --system-message "$1" "$2")
  decision=$("$ms" json get --value "$rendered" --field decision)
  reason=$("$ms" json get --value "$rendered" --field reason)
  [[ "$decision" == block && -n "$reason" ]] || return 1
  printf '%s\n' "$rendered"
}

external_stop_json() { # system message, reason, cause, remedy
  "$ms" report stop-block --system-message "$1" \
    --refusal-record "$stop_refusal_record" --session "$session" \
    --cause "$3" --remedy "$4" "$2"
}

tag="metasystem-main-$runtime-$("$ms" util slug "$session")"
up_failure=
if [[ "$event" == stop ]]; then
  up_rc=0
  if [[ -n "$identity_pid" ]]; then
    up_output=$(METASYSTEM_AGENT_RUNTIME="$runtime" "$ms" up --metasystem-root "$harness_root" \
      --repo "$repo" --session "$session" --pid "$identity_pid" --start-time "$identity_started" \
      --tag "$tag" 2>&1) || up_rc=$?
  else
    # A Stop call with no session identity still drives the restricted verify
    # and recovery path. It gains no announcement or checkout lease authority.
    up_output=$(METASYSTEM_AGENT_RUNTIME="$runtime" "$ms" up --metasystem-root "$harness_root" \
      --repo "$repo" --recover-only --if-down 2>&1) || up_rc=$?
  fi
  if (( up_rc != 0 )); then
    up_failure="Metasystem supervision arming failed: $(printf '%s' "$up_output" | tail -1)"
    record_stop_failure "supervision arming failed"
  fi
  health_rc=0
  health_line=$("$ms" health --hook-preview --repo "$repo" --metasystem-root "$harness_root" 2>/dev/null) || health_rc=$?
  if (( health_rc > 2 )) || [[ -z "$health_line" ]]; then
    health_line="HEALTH unknown — hook-freshness=unknown (the health engine returned no verdict)"
    record_stop_failure "the health engine returned no verdict"
  fi
  digest_rc=0
  digest_json=$("$ms" steward digest-pending --repo "$repo" 2>&1) || digest_rc=$?
  if (( digest_rc == 0 )); then
    digest_message_rc=0
    digest_cursor_rc=0
    digest_prefix_rc=0
    digest_message=$("$ms" json get --value "$digest_json" --field message 2>/dev/null) || digest_message_rc=$?
    digest_cursor=$("$ms" json get --value "$digest_json" --field cursor 2>/dev/null) || digest_cursor_rc=$?
    digest_prefix=$("$ms" json get --value "$digest_json" --field prefixSha256 2>/dev/null) || digest_prefix_rc=$?
    if (( digest_message_rc != 0 || digest_cursor_rc != 0 || digest_prefix_rc != 0 )); then
      digest_message="NARRATOR DIGEST unavailable: the digest state was unreadable"
      record_stop_failure "the narrator digest state was unreadable"
    fi
  else
    digest_message="NARRATOR DIGEST unavailable: ${digest_json//$'\n'/ }"
    record_stop_failure "the narrator digest could not be read"
  fi
  checkin_tail=$health_line
  [[ -z "$digest_message" ]] || checkin_tail="$checkin_tail
$digest_message"
fi

emit_stop_payload() { # response
  response=$1
  stop_decision=$("$ms" json get --value "$response" --field decision 2>/dev/null || true)
  [[ -n "$stop_decision" ]] || stop_decision=allow
  supervision_dir="$repo/artifacts/agents/supervision"
  mkdir -p "$supervision_dir" || true
  printf '%s stop response decision=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    "$stop_decision" >>"$supervision_dir/hooks.log" 2>/dev/null || true
  response_file_rc=0
  response_file=$(mktemp "${TMPDIR:-/tmp}/metasystem-supervision-response.XXXXXX") || response_file_rc=$?
  if (( response_file_rc != 0 )) || [[ -z "$response_file" ]]; then
    command printf '%s\n' "$response" || true
    "$ms" steward hook-complete --repo "$repo" --generation "$hook_generation" \
      --attempt "$hook_attempt_seq" --result ERROR --outcome PAYLOAD_STAGE_FAILED >/dev/null 2>&1 || true
    return 0
  fi
  if ! printf '%s\n' "$response" >"$response_file"; then
    command printf '%s\n' "$response" || true
    "$ms" steward hook-complete --repo "$repo" --generation "$hook_generation" \
      --attempt "$hook_attempt_seq" --result ERROR --outcome PAYLOAD_STAGE_FAILED >/dev/null 2>&1 || true
    rm -f "$response_file"
    return 0
  fi
  if ! command printf '%s\n' "$response"; then
    "$ms" steward hook-complete --repo "$repo" --generation "$hook_generation" \
      --attempt "$hook_attempt_seq" --result ERROR --outcome EMISSION_FAILED \
      --health-line "$health_line" --payload-file "$response_file" >/dev/null 2>&1 || true
    rm -f "$response_file"
    return 0
  fi
  if [[ -n "$digest_message" && "$digest_cursor" =~ ^[0-9]+$ && "$digest_prefix" =~ ^[0-9a-f]{64}$ ]]; then
    if ! "$ms" steward digest-advance --repo "$repo" --cursor "$digest_cursor" \
      --prefix-sha256 "$digest_prefix" >/dev/null 2>&1; then
      echo "supervision hook: emitted the narrator digest but could not advance its check-in cursor" >&2
    fi
  fi
  if ! "$ms" steward hook-complete --repo "$repo" --generation "$hook_generation" \
      --attempt "$hook_attempt_seq" --result OK --outcome EMITTED \
      --health-line "$health_line" --payload-file "$response_file" >/dev/null 2>&1; then
    echo "supervision hook: emitted the health line but could not record completion" >&2
  fi
  rm -f "$response_file"
}

emit_failed_stop() { # diagnostic
  local refusal_rc remedy
  failure_detail="Metasystem could not prove that stopping is safe: $1"
  remedy='A human or steward must restore supervision outside this seat, then retry.'
  if [[ "$1" == 'supervision arming failed' && -n "$up_failure" ]]; then
    remedy=$up_failure
  fi
  refusal_rc=0
  response=$(external_stop_json "$checkin_tail" "$failure_detail" "$1" "$remedy" 2>/dev/null) || refusal_rc=$?
  if (( refusal_rc != 0 )) || [[ -z "$response" ]]; then
    response=$(surface_json "Metasystem stop-refusal record failure: the record could not be read or atomically updated. Stopping is allowed so record failure cannot recreate the refusal loop.
Cause: $1
Remedy: $remedy" 2>/dev/null) || \
      response='{"systemMessage":"Metasystem could not update the stop-refusal record; stopping is allowed so record failure cannot recreate the refusal loop."}'
  fi
  emit_stop_payload "$response"
}

if [[ "$event" == stop ]]; then
  stop_refusal_slug=$("$ms" util slug "$session")
  stop_refusal_record="$repo/artifacts/agents/supervision/stop-refusals/$stop_refusal_slug.json"
  protocol_message=
  protocol_counts='{}'
  if [[ -n "$main_id" ]]; then
    protocol_growth_rc=0
    protocol_growth=$("$ms" lease protocol-growth --root "$state_root" --main-id "$main_id" 2>/dev/null) || protocol_growth_rc=$?
    if (( protocol_growth_rc != 0 )); then
      record_stop_failure "the holder protocol state could not be read"
    elif [[ -n "$protocol_growth" ]]; then
      protocol_message_rc=0
      protocol_counts_rc=0
      protocol_message=$("$ms" json get --value "$protocol_growth" --field message 2>/dev/null) || protocol_message_rc=$?
      protocol_counts=$("$ms" json get --value "$protocol_growth" --field counts 2>/dev/null) || protocol_counts_rc=$?
      if (( protocol_message_rc != 0 || protocol_counts_rc != 0 )); then
        record_stop_failure "the holder protocol state was unreadable"
      fi
    fi
  fi
  if [[ -n "$stop_failure" ]]; then
    emit_failed_stop "$stop_failure"
    exit 0
  fi
  # "Advisor" is a positive finding, not a fallback. It means an announced main
  # of THIS checkout is not the one holding it. A caller that could not be
  # identified at all is not an advisor -- it is unclassified, and answering it
  # with OWNED-ELSEWHERE replaces the entire turn-end report, including the
  # refusal to walk away from open work, with a sentence about ownership.
  if [[ "$main_class" == MAIN && "$main_holder" != true ]]; then
    advisor_message="OWNED-ELSEWHERE: this main is a read-only advisor in this checkout. To write independently, run scripts/agents/second-session.sh."
	[[ -z "$up_failure" ]] || advisor_message="$advisor_message
$up_failure"
	[[ -z "$hook_evidence_failure" ]] || advisor_message="$advisor_message
$hook_evidence_failure"
    [[ -z "$protocol_message" ]] || advisor_message="$advisor_message
$protocol_message"
    response=$(surface_json "$advisor_message
$checkin_tail")
    emit_stop_payload "$response"
    [[ -z "$main_id" || -z "$identity_pid" || -z "$protocol_message" ]] || \
      "$ms" lease protocol-advance --root "$state_root" --main-id "$main_id" \
        --caller-pid "$identity_pid" --counts "$protocol_counts" >/dev/null 2>&1 || true
    exit 0
  fi
  if [[ -n "$identity_pid" ]]; then
    renew_rc=0
    "$ms" lease renew --root "$state_root" --caller-pid "$identity_pid" >/dev/null 2>&1 || renew_rc=$?
    (( renew_rc == 0 )) || record_stop_failure "the checkout holder lease could not be renewed"
  fi

  # The WATCHDOG path calls the verdict like every other path (only the
  # advisor early-exit above bypasses it): the report's text stays
  # hook-side, its DIGEST rides to the verb, and the verb's surfaceWatchdog
  # answer decides exactly-once surfacing across concurrent Stop calls
  # (goal-system GOAL-04; the loose per-session state files are retired).
  watchdog_rc=0
  watchdog_text=$("$ms" supervise watchdog-report --repo "$repo" 2>/dev/null) || watchdog_rc=$?
  (( watchdog_rc == 0 )) || record_stop_failure "the supervision watchdog state could not be read"
  watchdog_digest=
  if [[ -n "$watchdog_text" ]]; then
    watchdog_digest_rc=0
    watchdog_digest=$(printf '%s' "$watchdog_text" | "$ms" util sha256) || watchdog_digest_rc=$?
    (( watchdog_digest_rc == 0 )) || record_stop_failure "the supervision watchdog evidence could not be prepared"
  fi

  # Leave evidence that this ran. Without it there is no telling a hook that
  # fired and found nothing from one that never fired, which is the confusion
  # that let this repository run for days with its hooks uninstalled.
  supervision_dir="$repo/artifacts/agents/supervision"
  mkdir -p "$supervision_dir"
  evidence_gc_rc=0
  "$script_dir/evidence-gc.sh" >>"$supervision_dir/hooks.log" 2>&1 || evidence_gc_rc=$?
  (( evidence_gc_rc == 0 )) || record_stop_failure "the hook evidence state could not be maintained"

  if [[ -n "$stop_failure" ]]; then
    emit_failed_stop "$stop_failure"
    exit 0
  fi

  # ONE structured decision (goal-system GOAL-05): the verdict verb owns
  # open work, the goal clause, precedence, block-once state, and the
  # all-clear. Every representable state is exit 0 with JSON; a nonzero
  # exit is I/O failure and this hook emits a provider-level refusal — never
  # silence, and never an all-clear it cannot vouch for.
  verdict_stderr=$(mktemp "${TMPDIR:-/tmp}/metasystem-verdict-err.XXXXXX")
  if verdict=$("$ms" report turn-verdict --root "$state_root" \
      --session "$session" --watchdog-surfaced "$watchdog_digest" \
      --main-id "$main_id" 2>"$verdict_stderr"); then
    rm -f "$verdict_stderr"
    should_block_rc=0
    display_rc=0
    surface_watchdog_rc=0
    should_block=$("$ms" json get --value "$verdict" --field shouldBlock 2>/dev/null) || should_block_rc=$?
    display=$("$ms" json get --value "$verdict" --field display 2>/dev/null) || display_rc=$?
    surface_watchdog=$("$ms" json get --value "$verdict" --field surfaceWatchdog 2>/dev/null) || surface_watchdog_rc=$?
    if (( should_block_rc != 0 || display_rc != 0 || surface_watchdog_rc != 0 )) || [[ -z "$display" ]] ||
        [[ ( "$should_block" != true && "$should_block" != false ) ||
           ( "$surface_watchdog" != true && "$surface_watchdog" != false ) ]]; then
      emit_failed_stop "the turn verdict was unreadable"
      exit 0
    fi

    printf '%s stop verdict block=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
      "$should_block" >>"$supervision_dir/hooks.log" 2>/dev/null || true

	extras=$up_failure
	[[ -z "$hook_evidence_failure" ]] || extras=$(printf '%s%s%s' "$extras" "${extras:+$'\n'}" "$hook_evidence_failure")
	[[ "$surface_watchdog" != true || -z "$watchdog_text" ]] || extras=$(printf '%s%s%s' "$extras" "${extras:+$'\n'}" "$watchdog_text")
    [[ -z "$protocol_message" ]] || extras=$(printf '%s%s%s' "$extras" "${extras:+$'\n'}" "$protocol_message")

    if [[ "$should_block" == true ]]; then
      # The display is the block reason byte-verbatim; watchdog and
      # protocol text stay in the non-blocking channel and never enter
      # the reason.
      blocking_message=$checkin_tail
      [[ -z "$extras" ]] || blocking_message="$extras
$blocking_message"
      response=$(stop_block_json "$blocking_message" "$display")
    elif [[ -n "$extras" ]]; then
      response=$(surface_json "$display
$extras
$checkin_tail")
    else
      response=$(surface_json "$display
$checkin_tail")
    fi
  else
    degraded_line=$(tail -1 "$verdict_stderr" 2>/dev/null || true)
    rm -f "$verdict_stderr"
    printf '%s stop verdict unavailable\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
      >>"$supervision_dir/hooks.log" 2>/dev/null || true
    degraded_message="turn-verdict unavailable: ${degraded_line:-no diagnostic}"
	[[ -z "$up_failure" ]] || degraded_message="$degraded_message
$up_failure"
	[[ -z "$hook_evidence_failure" ]] || degraded_message="$degraded_message
$hook_evidence_failure"
    [[ -z "$protocol_message" ]] || degraded_message="$degraded_message
$protocol_message"
    response=$(stop_block_json "$checkin_tail" "$degraded_message")
  fi
  emit_stop_payload "$response"
  [[ -z "$main_id" || -z "$identity_pid" || -z "$protocol_message" ]] || \
    "$ms" lease protocol-advance --root "$state_root" --main-id "$main_id" \
      --caller-pid "$identity_pid" --counts "$protocol_counts" >/dev/null 2>&1 || true
  exit 0
fi

# The second visibility channel runs before EVERY exit from here on:
# a session's start names anything the steward could not deliver,
# and the unidentified-agent branch is the degraded case that needs
# it most.
pending_line=$("$ms" steward pending --repo "$repo" 2>/dev/null || true)
[[ -n "$pending_line" ]] && surface_json "Steward incidents pending: $pending_line"

if [[ "$event" == end ]]; then
  session_end_rc=0
  "$ms" session end --root "$state_root" --session "$session" >/dev/null 2>&1 || session_end_rc=$?
  if (( session_end_rc != 0 )); then
    surface_json "Metasystem could not durably retire this session's unused stop authorization; later stops must treat it as unsafe."
  fi
  if [[ -z "$identity" ]]; then
    surface_json "Metasystem supervision could not identify the immediate $runtime agent process; arming was refused."
    exit 0
  fi
  pid=$identity_pid
  started=$identity_started
  METASYSTEM_AGENT_RUNTIME="$runtime" "$ms" up --metasystem-root "$harness_root" \
    --repo "$repo" --session "$session" --pid "$pid" --start-time "$started" \
    --tag "$tag" --retire >/dev/null 2>&1 || true
  exit 0
fi

if [[ -z "$identity" ]]; then
  surface_json "Metasystem supervision could not identify the immediate $runtime agent process; arming was refused."
  exit 0
fi
pid=$identity_pid
started=$identity_started

if output=$(METASYSTEM_AGENT_RUNTIME="$runtime" "$ms" up --metasystem-root "$harness_root" \
    --repo "$repo" --session "$session" --pid "$pid" --start-time "$started" \
    --tag "$tag" 2>&1); then
  # The watchdog revives with the first metasystem activity on this
  # machine: `up` verifies the owner, watcher, steward, announcement, and
  # lease as one idempotent transaction.
  exit 0
fi
surface_json "Metasystem supervision arming failed: $(printf '%s' "$output" | tail -1)"
