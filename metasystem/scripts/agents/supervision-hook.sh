#!/usr/bin/env bash
set -euo pipefail

# The pinned resolution contract (agnosticism B1, ric critiques r1-9,
# r2-8, r3-7): (1) shell-owned SYNTAX refusals — the event name and the
# runtime argument's registry-grammar SHAPE (no binary needed for a
# shape check; the closed name list is gone, so a future runtime needs
# no edit here); (2) executable resolution — a missing engine OR arm
# script stays benign exit 0, as before; (3) with both present,
# registry MEMBERSHIP — an unknown runtime exits 2; (4) the runtime's
# OPTIONAL session environment via the registry, expanded indirectly,
# never eval; (5) cwd resolution: payload cwd, then the declared
# variable's nonempty value, then PWD. The optional recovery-only scheduler
# entry is operator-owned and can be printed with `metasystem up
# --print-scheduler-entry`; this hook never installs host state.
runtime=${1:-}
event=${2:-}
[[ "$runtime" =~ ^[a-z][a-z0-9-]{0,31}$ ]] || exit 2
case "$event" in start|stop|end) ;; *) exit 2 ;; esac

# Executables resolve BEFORE any payload work (B1 critique finding 9:
# a missing engine with an unusable TMPDIR must still exit 0 benign).
script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
harness_root=$(cd "$script_dir/../.." && pwd -P)
ms="${METASYSTEM_BIN:-$harness_root/bin/metasystem}"
if [[ ! -x "$ms" ]]; then
  if [[ "$event" == stop ]]; then
    printf '%s\n' '{"systemMessage":"HEALTH unknown — hook-freshness=unknown (metasystem engine missing; reinstall or rebuild bin/metasystem)"}'
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
    # a guessed cwd (finding 9): benign no-op.
    exit 0
  fi
fi
repo=$(git -C "$cwd" rev-parse --show-toplevel 2>/dev/null) || exit 0
repo=$(cd "$repo" && pwd -P)
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
if [[ "$event" == stop ]]; then
  turn_key_rc=0
  turn_key=$({ printf '%s\n' "$session"; command cat "$payload"; } | "$ms" util sha256) || turn_key_rc=$?
  if (( turn_key_rc != 0 )) || [[ -z "$turn_key" ]]; then
	hook_evidence_failure="HEALTH unknown — hook-freshness=unknown (turn evidence could not be prepared)"
  else
	hook_attempt_rc=0
	hook_attempt=$(
	  "$ms" steward hook-attempt --repo "$repo" --pid "$$" --turn-key "$turn_key" 2>/dev/null
	) || hook_attempt_rc=$?
	if (( hook_attempt_rc != 0 )) || [[ -z "$hook_attempt" ]]; then
	  hook_evidence_failure="HEALTH unknown — hook-freshness=unknown (attempt evidence could not be recorded)"
	else
	  hook_generation=$("$ms" json get --value "$hook_attempt" --field generation 2>/dev/null || true)
	  hook_attempt_seq=$("$ms" json get --value "$hook_attempt" --field attemptSeq 2>/dev/null || true)
	  if ! [[ "$hook_generation" =~ ^[1-9][0-9]*$ && "$hook_attempt_seq" =~ ^[1-9][0-9]*$ ]]; then
		hook_evidence_failure="HEALTH unknown — hook-freshness=unknown (attempt evidence was unreadable)"
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
  identity_pid=$("$ms" json get --value "$identity" --field pid)
  identity_started=$("$ms" json get --value "$identity" --field pidStartedAt)
else
  # Recorded fallback: a hook may run in a test harness or runtime wrapper
  # whose authenticated main was announced explicitly. Classification returns
  # that exact announcement; an unannounced process gains nothing here.
  parent_view=$("$ms" lease classify --root "$repo" --metasystem-root "$harness_root" --caller-pid "$PPID" 2>/dev/null || true)
  if [[ "$("$ms" json get --value "$parent_view" --field class 2>/dev/null || true)" == MAIN ]]; then
    identity_pid=$("$ms" json get --value "$parent_view" --field announcement.pid 2>/dev/null || true)
    identity_started=$("$ms" json get --value "$parent_view" --field announcement.pidStartedAt 2>/dev/null || true)
    [[ "$identity_pid" =~ ^[1-9][0-9]*$ && "$identity_started" =~ ^[1-9][0-9]*$ ]] \
      && identity=recorded-main
  fi
fi
if [[ -n "$identity_pid" ]]; then
  lease_view=$("$ms" lease classify --root "$repo" --metasystem-root "$harness_root" --caller-pid "$identity_pid" 2>/dev/null || true)
  if [[ -n "$lease_view" ]]; then
    main_id=$("$ms" json get --value "$lease_view" --field mainId 2>/dev/null || true)
    main_class=$("$ms" json get --value "$lease_view" --field class 2>/dev/null || true)
    main_holder=$("$ms" json get --value "$lease_view" --field holder 2>/dev/null || true)
  fi
fi

surface_json() { # message
  "$ms" json object "systemMessage=$1"
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
  fi
  health_rc=0
  health_line=$("$ms" health --hook-preview --repo "$repo" --metasystem-root "$harness_root" 2>/dev/null) || health_rc=$?
  if (( health_rc > 2 )) || [[ -z "$health_line" ]]; then
    health_line="HEALTH unknown — hook-freshness=unknown (the health engine returned no verdict)"
  fi
  digest_rc=0
  digest_json=$("$ms" steward digest-pending --repo "$repo" 2>&1) || digest_rc=$?
  if (( digest_rc == 0 )); then
    digest_message=$("$ms" json get --value "$digest_json" --field message 2>/dev/null || true)
    digest_cursor=$("$ms" json get --value "$digest_json" --field cursor 2>/dev/null || true)
    digest_prefix=$("$ms" json get --value "$digest_json" --field prefixSha256 2>/dev/null || true)
  else
    digest_message="NARRATOR DIGEST unavailable: ${digest_json//$'\n'/ }"
  fi
  checkin_tail=$health_line
  [[ -z "$digest_message" ]] || checkin_tail="$checkin_tail
$digest_message"
fi

emit_stop_payload() { # response
  response=$1
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

if [[ "$event" == stop ]]; then
  protocol_message=
  protocol_counts='{}'
  if [[ -n "$main_id" ]]; then
    protocol_growth=$("$ms" lease protocol-growth --root "$repo" --main-id "$main_id" 2>/dev/null || true)
    if [[ -n "$protocol_growth" ]]; then
      protocol_message=$("$ms" json get --value "$protocol_growth" --field message)
      protocol_counts=$("$ms" json get --value "$protocol_growth" --field counts)
    fi
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
      "$ms" lease protocol-advance --root "$repo" --main-id "$main_id" \
        --caller-pid "$identity_pid" --counts "$protocol_counts" >/dev/null 2>&1 || true
    exit 0
  fi
  [[ -z "$identity_pid" ]] || \
    "$ms" lease renew --root "$repo" --caller-pid "$identity_pid" >/dev/null 2>&1 || true

  # The WATCHDOG path calls the verdict like every other path (only the
  # advisor early-exit above bypasses it): the report's text stays
  # hook-side, its DIGEST rides to the verb, and the verb's surfaceWatchdog
  # answer decides exactly-once surfacing across concurrent Stop calls
  # (goal-system GOAL-04; the loose per-session state files are retired).
  watchdog_text=$("$ms" supervise watchdog-report --repo "$repo" 2>/dev/null || true)
  watchdog_digest=
  [[ -z "$watchdog_text" ]] || watchdog_digest=$(printf '%s' "$watchdog_text" | "$ms" util sha256)

  # Leave evidence that this ran. Without it there is no telling a hook that
  # fired and found nothing from one that never fired, which is the confusion
  # that let this repository run for days with its hooks uninstalled.
  supervision_dir="$repo/artifacts/agents/supervision"
  mkdir -p "$supervision_dir"
  "$script_dir/evidence-gc.sh" >>"$supervision_dir/hooks.log" 2>&1 || true

  # ONE structured decision (goal-system GOAL-05): the verdict verb owns
  # open work, the goal clause, precedence, block-once state, and the
  # all-clear. Every representable state is exit 0 with JSON; a nonzero
  # exit is I/O failure and this hook's own FIXED degraded message takes
  # over — never silence, and never an all-clear it cannot vouch for. The
  # old 2>/dev/null||true suppression was the named defect this removes.
  verdict_stderr=$(mktemp "${TMPDIR:-/tmp}/metasystem-verdict-err.XXXXXX")
  if verdict=$("$ms" report turn-verdict --root "$repo" \
      --session "$session" --watchdog-surfaced "$watchdog_digest" \
      --main-id "$main_id" 2>"$verdict_stderr"); then
    rm -f "$verdict_stderr"
    should_block=$("$ms" json get --value "$verdict" --field shouldBlock)
    display=$("$ms" json get --value "$verdict" --field display)
    surface_watchdog=$("$ms" json get --value "$verdict" --field surfaceWatchdog)

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
      response=$("$ms" report stop-block --system-message "$blocking_message" "$display")
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
    response=$(surface_json "$degraded_message
$checkin_tail")
  fi
  emit_stop_payload "$response"
  [[ -z "$main_id" || -z "$identity_pid" || -z "$protocol_message" ]] || \
    "$ms" lease protocol-advance --root "$repo" --main-id "$main_id" \
      --caller-pid "$identity_pid" --counts "$protocol_counts" >/dev/null 2>&1 || true
  exit 0
fi

# The second visibility channel runs before EVERY exit from here on:
# a session's start names anything the steward could not deliver,
# and the unidentified-agent branch is the degraded case that needs
# it most.
pending_line=$("$ms" steward pending --repo "$repo" 2>/dev/null || true)
[[ -n "$pending_line" ]] && surface_json "Steward incidents pending: $pending_line"

if [[ -z "$identity" ]]; then
  surface_json "Metasystem supervision could not identify the immediate $runtime agent process; arming was refused."
  exit 0
fi
pid=$identity_pid
started=$identity_started
if [[ "$event" == end ]]; then
  METASYSTEM_AGENT_RUNTIME="$runtime" "$ms" up --metasystem-root "$harness_root" \
    --repo "$repo" --session "$session" --pid "$pid" --start-time "$started" \
    --tag "$tag" --retire >/dev/null 2>&1 || true
  exit 0
fi

if output=$(METASYSTEM_AGENT_RUNTIME="$runtime" "$ms" up --metasystem-root "$harness_root" \
    --repo "$repo" --session "$session" --pid "$pid" --start-time "$started" \
    --tag "$tag" 2>&1); then
  # The watchdog revives with the first metasystem activity on this
  # machine: `up` verifies the owner, watcher, steward, announcement, and
  # lease as one idempotent transaction.
  exit 0
fi
surface_json "Metasystem supervision arming failed: $(printf '%s' "$output" | tail -1)"
