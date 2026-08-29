#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/agents/adapters/devin.sh identity
  scripts/agents/adapters/devin.sh config-identity
  scripts/agents/adapters/devin.sh signature
  scripts/agents/adapters/devin.sh enforcement-map
  scripts/agents/adapters/devin.sh contract
  scripts/agents/adapters/devin.sh probe
  scripts/agents/adapters/devin.sh output-stream --round-dir <absolute-path>
  scripts/agents/adapters/devin.sh dispatch --job <job-id> --start-gate <file>
      --instance-tag <tag>
  scripts/agents/adapters/devin.sh follow-up --job <job-id> --start-gate <file>
      --instance-tag <tag>
  scripts/agents/adapters/devin.sh cancel --job <job-id>
  scripts/agents/adapters/devin.sh selftest
  scripts/agents/adapters/devin.sh local-config-paths
USAGE
}

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/runtime-common.sh"
adapter_common_init devin

# Measured, not guessed: a design-critic turn on swe-1-7 ran roughly five
# minutes and 400k tokens. The shared 240s default reports a failure while the
# job is still working, which is worse than waiting.
selftest_turn_ceiling_sec=900

# This runtime ends the turn when a tool is denied: no reply, no report. The
# permission leg therefore runs its attempts as their own turns rather than
# asking one turn to be denied and then describe the denial.
selftest_denial_ends_turn=1

devin_version() {
  command -v devin >/dev/null 2>&1 || { echo "devin CLI is not installed" >&2; return 1; }
  devin --version 2>/dev/null | "$ms" adapter version-parse
}

devin_config_identity() {
  local version config_dir project_root
  local -a settings_files
  version=$(devin_version)
  config_dir=${XDG_CONFIG_HOME:-${HOME:?}/.config}
  project_root=$(git -C "$root" rev-parse --show-toplevel)
  settings_files=(
    "$config_dir/devin/config.json"
    "$config_dir/devin/hooks.v1.json"
    "$project_root/.devin/config.json"
    "$project_root/.devin/config.local.json"
    "$project_root/.devin/hooks.v1.json"
  )
  configuration_identity devin "$version" "${settings_files[@]}"
}

devin_identity() {
  local details version hash
  details=$(devin_config_identity)
  version=$(configuration_identity_field "$details" cliVersion)
  hash=$(configuration_identity_field "$details" configHash)
  printf '%s %s\n' "$version" "$hash"
}

probe() {
  local details version hash key_hashes
  details=$(devin_config_identity)
  version=$(configuration_identity_field "$details" cliVersion)
  hash=$(configuration_identity_field "$details" configHash)
  key_hashes=$(configuration_identity_field "$details" configKeyHashes)
  devin auth status >/dev/null 2>&1 || {
    echo "devin authentication is unavailable; run devin auth login" >&2
    return 1
  }
  write_capability_snapshot devin "$version" "$hash" \
    '["file","stdout","atif","acp"]' \
    '{
      "resume": true,
      "sessionEstablishedSignal": false,
      "sessionEstablishedTimeoutSec": 30,
      "nativeStructuredOutput": false,
      "nativeEvents": false,
      "nativeUsage": false,
      "gracefulCancel": false,
      "hooks": true,
      "protocolServer": true,
      "nativeBudget": false
    }' \
    '{"unverified": ["readRoots", "writeRoots", "network"]}' \
    "$adapter_enforcement_map" \
    "$key_hashes"
}

build_devin_config() { # config output, provenance output
  # --config REPLACES the user configuration rather than layering on it, and a
  # file without the onboarding marker makes the CLI print a welcome banner
  # into the turn's stdout. So the job's config is the user's file with the
  # permissions block swapped in: the organisation id and onboarding state
  # survive, and nothing the user set is silently dropped. Replacing the
  # permissions member (never merging it) is the only safe direction, because
  # merging can only widen what the job may attempt.
  "$ms" adapter devin-config --record "$record" --output "$1" --provenance "$2"
}

list_devin_sessions() { # output file
  local output=$1
  set +e
  (cd "$workspace" && devin list --format json) >"$output" 2>>"$log"
  local status=$?
  set -e
  return "$status"
}

# `devin list --format json` reports `id`/`short_id` and no session_id at all,
# so the previous parser matched nothing. A session is this turn's only when it
# is absent from the pre-launch baseline AND its working directory is this
# turn's workspace. Two candidates refuse rather than pick by timestamp: two
# launches in one directory cannot be told apart, and guessing records a peer's
# session as this job's identity.
new_devin_session() { # before list, current list, optional hook signal, workspace
  "$ms" adapter devin-session \
    --before "$1" --current "$2" --signal "$3" --workspace "${4:-}"
}

devin_usage() { # output, transcript, cumulative-out, previous-cumulative-or-empty, expect-previous(0|1)
  # The transcript's final_metrics are CUMULATIVE for the session, not per turn:
  # a first turn reported 12833 prompt tokens and its resumed successor reported
  # 25799 — the session total. Chain, mission, and benchmark consumers ADD round
  # records, so publishing the totals would count every earlier turn again in
  # every later round. Each round publishes the delta and stores the cumulative
  # figures for its successor to subtract.
  #
  # A resumed round whose predecessor artifact is missing publishes UNAVAILABLE
  # rather than the session totals. An aggregator never reads a job log, so
  # "wrong but explained in a log" is just wrong. An enterprise Devin reports
  # ACU instead of tokens; ACU rides in providerUnits, metered by name, never
  # mixed into a token field or dressed up as cost.
  if [[ "${5:-0}" == 1 ]]; then
    "$ms" adapter devin-usage --usage "$1" --transcript "$2" \
      --snapshot "${6:-}" --cumulative "$3" --previous "${4:-}" --expect-previous
  else
    "$ms" adapter devin-usage --usage "$1" --transcript "$2" \
      --snapshot "${6:-}" --cumulative "$3" --previous "${4:-}"
  fi
}

previous_round_artifact() { # file name -> path of the previous round's copy, if any
  local name=$1 previous=$((round - 1)) candidate
  (( previous >= 1 )) || return 0
  candidate="$agents/$root_job/rounds/$previous/$name"
  [[ -f "$candidate" ]] && printf '%s\n' "$candidate"
  return 0
}


# Session certification and the effective-model fallback are the engine's
# (`adapter devin-settle`, script-adapters-07/D26): it prints the derived
# model and its exit is the certification verdict; record writes stay here.
devin_settle() { # transcript, snapshot, extra verb flags -> prints model; rc 1 = not certified
  "$ms" adapter devin-settle --transcript "$1" --snapshot "${2:-}" \
    --session "${session_id:-}" --round-dir "$round_dir" "${@:3}"
}

# Recompute this round's usage from the REPAIR transcript when a repair
# happened. The repair resumed the same session, so its transcript carries the
# session totals including the repair turn; usage was first computed from the
# pre-repair transcript, which would drop the repair's spend from this round and
# charge it to the next. `previous`, `expect_previous`, and `cumulative` are
# supervise's locals, in scope here while supervise runs.
runtime_usage_after_repair() { # usage file
  local usage_file=$1 repair_transcript="$round_dir/transcript.repair-1.atif.json"
  # No repair transcript: the repair's spend cannot be read, so this round's
  # usage cannot be trusted as complete. Record it unavailable rather than
  # leaving the pre-repair figure standing and undercounting provider budget.
  if [[ ! -s "$repair_transcript" ]]; then
    "$ms" adapter usage-unavailable --output "$usage_file"
    return 0
  fi
  devin_usage "$usage_file" "$repair_transcript" "${cumulative:-$round_dir/session-usage.json}" \
    "${previous:-}" "${expect_previous:-0}" "$round_dir/transcript.repair.snapshot"
}

# The repair turn the shared contract calls when a reply does not validate. It
# resumes the SAME session, so the delegate still has everything it read and
# concluded; only the shape is being asked for again. Same model, same envelope,
# same config -- a repair that changed any of those would not be a repair.
runtime_settle_after_repair() { # -> nonzero when the repair cannot be confirmed
  local repair_transcript="$round_dir/transcript.repair-1.atif.json" observed settle_rc=0
  observed=$(devin_settle "$repair_transcript" "$round_dir/transcript.repair.snapshot" --require-transcript) || settle_rc=$?
  # The observed model records even when certification fails: the record
  # must reflect what the transcript actually named.
  [[ -z "$observed" ]] || record_result_effective_model "$observed" || true
  return "$settle_rc"
}

# The raw repair invocation: reports the provider's exit alone, so the
# delivery walk can judge file-only deliveries (D64). The malformed-repair
# contract keeps its stricter gate in the wrapper below.
runtime_repair_invoke() { # prompt file, output file -> provider exit
  local repair_prompt=$1 output=$2 repair_status
  [[ -n "${session_id:-}" && -n "${config_file:-}" ]] || return 1
  set +e
  ( cd "$workspace" && while IFS= read -r a; do export "${a?}"; done < <(job_git_quarantine_env "$workspace"); exec devin -p \
      --prompt-file "$repair_prompt" \
      --respect-workspace-trust false \
      --model "$requested_model" \
      --permission-mode "${permission_mode:-auto}" \
      --config "$config_file" \
      -r "$session_id" \
      --export "$round_dir/transcript.repair-1.atif.json" ) >"$output" 2>>"$log"
  repair_status=$?
  set -e
  return "$repair_status"
}

runtime_repair_turn() { # prompt file, output file
  local rc=0
  runtime_repair_invoke "$1" "$2" || rc=$?
  # A repair that exits nonzero has failed, even if it printed something that
  # happens to parse: a failed provider call must not be turned into a completed
  # job by a lucky-looking stdout. Both a clean exit AND non-empty output are
  # required.
  (( rc == 0 )) && [[ -s "$2" ]]
}

# The transport selector (D81/D82): metasystem.conf's
# dispatch.transport.devin. THE FLIP LANDED 2026-08-24 (D82 satisfied
# by the sealed bm-2d cohort under the corrected kit): the shipped
# template configuration now declares acp, so the default a fresh
# checkout resolves is ACP. An ABSENT key still resolves legacy
# (pre-flip configurations keep their meaning); an unreadable
# configuration or an
# unrecognized value REFUSES — a broken config must never fail
# open into D61's dangerous path (P3 critique F3). Prints legacy
# or acp; nonzero is the refusal.
devin_transport() {
  local value rc=0
  value=$("$root/scripts/metasystem-config.sh" get --key dispatch.transport.devin --default legacy 2>>"${log:-/dev/null}") || rc=$?
  if (( rc != 0 )); then
    echo "transport-config-unreadable"
    return 1
  fi
  case "$value" in
    legacy|acp) echo "$value"; return 0 ;;
  esac
  echo "transport-config-invalid:$value"
  return 1
}

# The ACP transport path: the script owns fifos, custody, and
# killing; `acp turn` owns the wire (records/acp/acp-transport-design.md).
# The session id surfaces MID-TURN through the session file so the
# handshake is recorded inside the dispatcher's deadline (critique
# F1); the client runs backgrounded under wait_for_cli's heartbeat
# and deadline machinery with BOTH children registered (F2);
# delivery rides the collector's EXCLUSIVE acp channel; the legacy
# repair command is unreachable from here.
record_acp_transport_pin() { # best-effort after handshake; the refusal guards enforce
  local patch="$round_dir/transport-patch.json"
  "$ms" adapter transport-patch --output "$patch" --transport acp >>"$log" 2>&1 \
    && "$dispatch" __record-cas --job "$job" --expect running --status running --patch "$patch" >>"$log" 2>&1 \
    || true
}

supervise_acp() { # dispatch|follow-up and supervisor args
  local verb=$1
  shift
  prepare_supervision "$verb" "$@" || { usage; return 2; }
  local usage_file="$round_dir/usage.json"
  local outcome_file="$round_dir/acp-outcome.json"
  local journal_file="$round_dir/acp-journal.log"
  local session_file="$round_dir/acp-session-id"
  local attempt_nonce server_out server_in
  local server_pid client_pid handshake_done=0 session_from_wire= row= mode= grade= expected_protocol=
  local devin_prompt="$round_dir/prompt.devin.md"

  # A chain never switches transports (D82 fix-forward): a record
  # already born on one transport refuses the other.
  local recorded_transport
  recorded_transport=$("$ms" json get --value "$(cat "$record")" --field transport --default "")
  if [[ -n "$recorded_transport" && "$recorded_transport" != acp ]]; then
    fail_pending transport_switch_refused setup
    return 1
  fi

  devin_return_file="$round_dir/devin-return.json"
  if [[ -e "$devin_return_file" ]]; then
    fail_pending stale_named_return setup
    return 1
  fi
  # The same augmented prompt as the legacy path: its delivery
  # instruction already demands the JSON as the FINAL MESSAGE too,
  # which is exactly the acp channel.
  "$ms" adapter devin-prompt --prompt "$prompt" --schema "$schema" \
    --output "$devin_prompt" --return-file "$devin_return_file"
  record_actual_workspace_write_scope
  fail_if_effective_wider_before_launch || return 1
  : >"$events"

  if ! "$ms" acp preflight --envelope-file "$effective" >>"$log" 2>&1; then
    fail_pending acp_preflight_refused setup
    return 1
  fi
  grade=$("$ms" json get --value "$(cat "$effective")" --field tools --default "")
  mode=$("$ms" acp mode --runtime devin --tools "$grade" 2>>"$log") || {
    fail_pending acp_mode_unmapped setup
    return 1
  }
  # The registry's declaration reaches the launch (critique F4):
  # the expectation is queried, never re-defaulted downstream.
  expected_protocol=$("$ms" json get \
    --value "$("$ms" runtime acp-expectation devin)" \
    --field expectedProtocolVersion --default "") || expected_protocol=
  if [[ -z "$expected_protocol" ]]; then
    fail_pending acp_expectation_missing setup
    return 1
  fi

  # Per-attempt fifo names (critique F2): never reused, never able
  # to collide with an earlier blocked generation's endpoints.
  attempt_nonce=$("$ms" util token-hex --bytes 6)
  server_out="$round_dir/acp-$attempt_nonce-out"
  server_in="$round_dir/acp-$attempt_nonce-in"
  mkfifo "$server_out" "$server_in" || { fail_pending acp_fifo_setup setup; return 1; }
  # Transport hygiene (KI-42): the fifo pair is wire plumbing, not
  # evidence — a named pipe left in the round dir breaks any later
  # evidence-tree copy (the benchmark grader died on one). Both
  # endpoints dead ⇒ the pair is inert and removed at every exit.
  acp_fifo_cleanup() { rm -f -- "$server_out" "$server_in"; }
  # Redirection order is the deadlock contract: the server opens
  # stdout FIRST (pairing the client's read-side-first open), then
  # stdin (pairing the client's write side).
  # argv0 devin-delegate-acp: the census signature distinguishes THIS
  # delegate-side server from the host CLI's internal raw `devin acp`
  # helper (issue #12) — the binary ignores argv0, the classifier reads it.
  mark_cli_prefork || { acp_fifo_cleanup; fail_pending prefork_marker handshake; return 1; }
  ( cd "$workspace" && while IFS= read -r a; do export "${a?}"; done < <(job_git_quarantine_env "$workspace"); exec -a devin-delegate-acp "$(command -v devin)" acp >"$server_out" <"$server_in" 2>>"$log" ) &
  server_pid=$!
  register_cli_custody "$server_pid" || { terminate_cli_child "$server_pid"; acp_fifo_cleanup; fail_pending custody_registration handshake; return 1; }

  local -a turn_args
  turn_args=(
    acp turn --server-out "$server_out" --server-in "$server_in"
    --journal "$journal_file" --workspace "$workspace"
    --envelope-file "$effective" --prompt-file "$devin_prompt"
    --mode "$mode" --expected-protocol "$expected_protocol"
    --session-file "$session_file"
  )
  # Follow-up rides session/load (ACP-to-ACP proven by the wire
  # probe, step B); the transport never switches mid-chain.
  [[ "$verb" != follow-up ]] || turn_args+=(--load-session "$requested_session")
  mark_cli_prefork || { terminate_cli_child "$server_pid"; acp_fifo_cleanup; fail_pending prefork_marker handshake; return 1; }
  "$ms" "${turn_args[@]}" >"$outcome_file" 2>>"$log" &
  client_pid=$!
  register_cli_custody "$client_pid" || { terminate_cli_child "$client_pid"; terminate_cli_child "$server_pid"; acp_fifo_cleanup; fail_pending custody_registration handshake; return 1; }
  printf '{"type":"acp-launched","server_pid":%s,"client_pid":%s,"mode":"%s"}\n' \
    "$server_pid" "$client_pid" "$mode" >>"$events"

  # The handshake loop: the session id lands in the session file at
  # setup success — minutes before the turn settles — and a server
  # that dies before setup bounds the missing-peer path (F2).
  while kill -0 "$client_pid" 2>/dev/null; do
    if (( ! handshake_done )) && [[ -s "$session_file" ]]; then
      session_from_wire=$(head -1 "$session_file")
      if ! record_handshake "$session_from_wire" "" ""; then
        terminate_cli_child "$client_pid"
        terminate_cli_child "$server_pid"
        acp_fifo_cleanup
        return 1
      fi
      handshake_done=1
      printf '{"type":"session-correlated","session_id":"%s","predicate":"acp-wire-typed"}\n' \
        "$session_from_wire" >>"$events"
      record_acp_transport_pin
    fi
    if (( ! handshake_done )) && ! kill -0 "$server_pid" 2>/dev/null; then
      terminate_cli_child "$client_pid"
      acp_fifo_cleanup
      fail_pending acp_server_died handshake
      return 1
    fi
    (( handshake_done )) && break
    touch "$heartbeat"
    sleep 0.05
  done
  wait_for_cli "$client_pid"
  terminate_cli_child "$server_pid" || true
  acp_fifo_cleanup
  printf 'acp client exit status=%s\n' "$cli_status" >>"$log"

  # Terminal-writer discipline (critique F7): before the handshake
  # the record is pending; after it the record is running.
  acp_terminal() { # reason, phase
    if (( handshake_done )); then
      finish_running failed "$1" "$2" "$usage_file"
    else
      fail_pending "$1" "$2"
    fi
  }

  if (( cli_status != 0 )) || [[ ! -s "$outcome_file" ]]; then
    acp_terminal acp_client_mechanical delivery
    return 1
  fi
  if ! "$ms" adapter acp-usage --usage "$usage_file" --outcome "$outcome_file" >>"$log" 2>&1; then
    # A usage owner that cannot even write UNAVAILABLE is a
    # mechanical failure, never silently null usage (critique F9).
    acp_terminal acp_usage_mechanical delivery
    return 1
  fi
  row=$("$ms" json get --value "$(cat "$outcome_file")" --field row --default "")
  # The session comes from the WIRE for both verbs: a follow-up
  # whose load failed has no sessionId in its outcome, and a
  # fabricated handshake is worse than a failed one (critique F6).
  if (( ! handshake_done )); then
    session_from_wire=$("$ms" json get --value "$(cat "$outcome_file")" --field sessionId --default "")
    if [[ -n "$session_from_wire" ]]; then
      record_handshake "$session_from_wire" "" "" && handshake_done=1
      (( ! handshake_done )) || record_acp_transport_pin
    fi
  fi
  case "$row" in
    delivered) ;;
    auth-required)
      acp_terminal acp_auth_required handshake
      return 1 ;;
    version-mismatch|setup-error)
      acp_terminal "acp_$(printf '%s' "$row" | tr '-' '_')" setup
      return 1 ;;
    protocol-error)
      # Post-handshake protocol deaths are delivery-phase facts;
      # only a pre-session one is setup (critique F7).
      if (( handshake_done )); then
        acp_terminal acp_protocol_error delivery
      else
        acp_terminal acp_protocol_error setup
      fi
      return 1 ;;
    *)
      acp_terminal "acp_$(printf '%s' "$row" | tr '-' '_')" delivery
      return 1 ;;
  esac
  if (( ! handshake_done )); then
    acp_terminal acp_delivered_without_session handshake
    return 1
  fi

  local collect_rc=0 collect_json reply_path
  set +e
  collect_json=$("$ms" adapter devin-collect --root "$root" --job "$job" \
    --round-dir "$round_dir" --workspace "$workspace" \
    --acp-outcome "$outcome_file" \
    --record "$record" --attempt initial --session "$session_from_wire")
  collect_rc=$?
  set -e
  case "$collect_rc" in
    0)
      reply_path=$(printf '%s' "$collect_json" | "$ms" json get --value "$collect_json" --field reply)
      complete_from_cli 0 "$usage_file" "$reply_path" ""
      return $? ;;
    3)
      # Repair on ACP is disabled pre-claim (the spec's first
      # no-repair case): no claim is written, the failure is named.
      finish_running failed acp_undelivered delivery "$usage_file"
      return 1 ;;
    *)
      finish_running failed collect_mechanical delivery "$usage_file"
      return 1 ;;
  esac
}

supervise() { # dispatch|follow-up and supervisor args
  local verb=$1 transport
  if ! transport=$(devin_transport); then
    echo "devin transport refused: $transport" >&2
    return 1
  fi
  if [[ "$transport" == acp ]]; then
    shift
    supervise_acp "$verb" "$@"
    return $?
  fi
  shift
  prepare_supervision "$verb" "$@" || { usage; return 2; }
  local acp_born
  acp_born=$("$ms" json get --value "$(cat "$record")" --field transport --default "")
  if [[ "$acp_born" == acp ]]; then
    # D82 fix-forward: a chain born on ACP never rides legacy,
    # whatever the flag says today.
    fail_pending transport_switch_refused setup
    return 1
  fi
  # The CLI has no free-form metadata flag. Its per-turn config path is already
  # an argv field and is private to this round, so naming that file with the
  # instance tag gives ownership checks the same exact positional proof.
  config_file="$round_dir/$instance_tag"
  local transcript="$round_dir/transcript.atif.json"
  local before_sessions="$round_dir/devin-sessions-before.json"
  local current_sessions="$round_dir/devin-sessions-current.json"
  local signal_file="$round_dir/devin-session-signal.json"
  local usage_file="$round_dir/usage.json"
  local cli_pid output_seen=0 resolved_session
  local -a command

  # This runtime has no native structured output: the other two adapters hand
  # the CLI a schema flag (--json-schema, --output-schema) and Devin has
  # neither, so a Devin delegate was told only "return schema-valid JSON" in
  # prose and invented its own field names -- a real turn came back with
  # `description` where the schema says `command`, `observed`, and `level`.
  # The schema itself goes in the prompt, since that is the only channel this
  # runtime has. The dispatcher's prompt stays untouched as evidence; the
  # augmented copy is what the CLI reads.
  local devin_prompt="$round_dir/prompt.devin.md"
  # The named return file is the delivery channel this model actually uses:
  # it finishes by writing files, not by final message (D62), so the prompt
  # names a deterministic path inside the round evidence and the collect
  # step below reads it whenever stdout comes back empty.
  devin_return_file="$round_dir/devin-return.json"
  # A pre-existing named file is evidence of a crashed earlier attempt:
  # surfaced, never clobbered (D64).
  if [[ -e "$devin_return_file" ]]; then
    fail_pending stale_named_return setup
    return 1
  fi
  "$ms" adapter devin-prompt --prompt "$prompt" --schema "$schema" \
    --output "$devin_prompt" --return-file "$devin_return_file"

  record_actual_workspace_write_scope
  fail_if_effective_wider_before_launch || return 1
  : >"$events"
  : >"$raw"
  permission_mode=$(build_devin_config "$config_file" "$round_dir/devin-config-provenance.json")
  # A baseline that failed to list is a refusal, not an empty baseline: with no
  # baseline every pre-existing session looks new, which is how a peer's session
  # becomes this job's recorded identity.
  list_devin_sessions "$before_sessions" || {
    fail_pending session_baseline_unavailable handshake
    return 1
  }
  # A command that succeeded but emitted unparseable JSON is not an empty
  # baseline: reading it as empty makes every pre-existing session look new,
  # which is the peer-attribution bug the baseline exists to prevent.
  if ! "$ms" util json-validate --file "$before_sessions"; then
    fail_pending session_baseline_unreadable handshake
    return 1
  fi
  # `autonomous` is not a mode this CLI offers, and --sandbox asks for a mode
  # this organisation's policy refuses outright, so every dispatch that passed
  # them failed before it began. The modes are auto, accept-edits, smart, and
  # dangerous; since D61 (the human's waiver, 2026-08-15) every dispatch runs
  # `dangerous` — the graded modes turned envelope refusals into sessions
  # that ended without delivering.
  command=(
    devin -p
    --prompt-file "$devin_prompt"
    --respect-workspace-trust false
    --model "$requested_model"
    --permission-mode "$permission_mode"
    --config "$config_file"
    --export "$transcript"
  )
  if [[ "$verb" == follow-up ]]; then
    # D2 residual gate: the documented `-r <session-id>` mapping is complete,
    # but exact live resume behavior remains acceptance evidence for the
    # user's Devin machine. The selftest below requires the same id in round 2.
    command+=(-r "$requested_session")
  fi

  mark_cli_prefork || { fail_pending prefork_marker handshake; return 1; }
  (
    cd "$workspace"
    # Existing Devin hooks can backfill this file from their stable session_id
    # payload. The baseline remains `devin list --format json`, because the
    # adapter cannot install repository hooks into a delegate worktree.
    export METASYSTEM_DEVIN_SESSION_SIGNAL="$signal_file"
    while IFS= read -r a; do export "${a?}"; done < <(job_git_quarantine_env "$workspace")
    exec "${command[@]}" >"$raw" 2>>"$log"
  ) &
  cli_pid=$!
  register_cli_custody "$cli_pid" || { terminate_cli_child "$cli_pid"; fail_pending custody_registration handshake; return 1; }

  # Correlation polls from LAUNCH, not from first output. On this runtime stdout
  # is the final reply, so waiting for output would let the handshake complete
  # only as the turn ends -- and a turn that produces no reply at all would
  # never correlate, then be reported as a missing session instead of as the
  # empty reply it was.
  while kill -0 "$cli_pid" 2>/dev/null; do
    if [[ "$verb" == follow-up ]]; then
      # A resumed turn correlates nothing: its session is the one being resumed
      # and is in the baseline by definition, so no "new session" exists to find.
      resolved_session=$requested_session
    else
      list_devin_sessions "$current_sessions" || true
      local correlate_rc=0
      resolved_session=$(new_devin_session "$before_sessions" "$current_sessions" "$signal_file" "$workspace" 2>>"$log") || correlate_rc=$?
      # Exit 3 is two new sessions in this workspace at once: the adapter cannot
      # tell which is this turn's, and guessing records a peer's session as this
      # job's identity. That is a named refusal, not a keep-polling empty result.
      # (3, not 2: the package reserves 2 for usage errors — D15/cli-6.)
      if (( correlate_rc == 3 )); then
        fail_pending ambiguous_session_correlation handshake
        terminate_cli_child "$cli_pid"
        return 1
      fi
    fi
    if [[ -n "$resolved_session" ]]; then
      if ! record_handshake "$resolved_session" "" "$requested_model"; then
        terminate_cli_child "$cli_pid"
        return 1
      fi
      printf '{"type":"session-correlated","session_id":"%s","predicate":"listed-for-this-workspace-plus-live-process"}\n' \
        "$resolved_session" >>"$events"
      break
    fi
    touch "$heartbeat"
    sleep 0.05
  done
  wait_for_cli "$cli_pid"
  # D2 residual gate: Devin CLI exit-code meanings are undocumented. Until the
  # user-run selftest records them, zero means candidate success and every
  # nonzero value is preserved as the adapter's generic runtime_error path.
  printf 'devin cli exit status=%s\n' "$cli_status" >>"$log"
  local cumulative="$round_dir/session-usage.json" previous= expect_previous=0 settle_observed= settle_failed=0
  if [[ "$verb" == follow-up ]]; then
    previous=$(previous_round_artifact session-usage.json)
    expect_previous=1
  fi
  # The transcript ceiling is checked up front (D64): usage, settlement,
  # and collection all read the attempt snapshot, and an over-ceiling
  # export is its own named terminal — never identity disagreement,
  # never an empty reply, never a paid repair.
  if [[ -s "$transcript" ]] && (( $(wc -c <"$transcript") > 8388608 )); then
    finish_running failed transcript_oversize delivery "$usage_file"
    return 1
  fi
  attempt_snapshot="$round_dir/transcript.initial.snapshot"
  devin_usage "$usage_file" "$transcript" "$cumulative" "$previous" "$expect_previous" "$attempt_snapshot"
  settle_observed=$(devin_settle "$transcript" "$attempt_snapshot") || settle_failed=1
  [[ -z "$settle_observed" ]] || record_result_effective_model "$settle_observed" || true
  # The transcript is authoritative for session identity once the turn ends.
  if (( handshake_done )) && (( ${settle_failed:-0} )); then
    finish_running failed session_identity_disagreement delivery "$usage_file"
    return 1
  fi

  # THE DELIVERY WALK (D64): every decision — presence bars, candidate
  # selection, the designation rule, watermark, provenance — is the
  # engine's (`adapter devin-collect`); this shell only routes verdicts.
  if (( cli_status == 0 )) && (( ! handshake_done )) && [[ ! -s "$raw" ]]; then
    # No session and empty stdout: the presence scan decides between the
    # two pinned outcomes without any validation.
    local presence
    presence=$("$ms" adapter devin-collect --presence-only --root "$root" --job "$job" \
      --round-dir "$round_dir" --record "$record" --stdout "$raw" \
      --named "${devin_return_file:-}") || true
    if [[ "$presence" == *'"candidatesPresent":true'* ]]; then
      fail_pending handshake_missing_session_id handshake "$usage_file"
      return 1
    fi
    fail_pending empty_reply delivery "$usage_file"
    return 1
  fi

  if (( cli_status == 0 )) && (( handshake_done )); then
    local collect_rc=0 collect_json reply_path
    set +e
    collect_json=$("$ms" adapter devin-collect --root "$root" --job "$job" \
      --round-dir "$round_dir" --workspace "$workspace" --stdout "$raw" \
      --named "${devin_return_file:-}" --transcript "$transcript" \
      --record "$record" --attempt initial --session "$session_id")
    collect_rc=$?
    set -e
    case "$collect_rc" in
      0)
        reply_path=$(printf '%s' "$collect_json" | "$ms" json get --value "$collect_json" --field reply)
        complete_from_cli "$cli_status" "$usage_file" "$reply_path" "$transcript"
        return $? ;;
      3)
        devin_delivery_repair "$usage_file"
        return $? ;;
      5)
        finish_running failed transcript_oversize delivery "$usage_file"
        return 1 ;;
      *)
        finish_running failed collect_mechanical delivery "$usage_file"
        return 1 ;;
    esac
  fi
  complete_from_cli "$cli_status" "$usage_file" "$raw" "$transcript"
}

# The delivery repair (D64): adjudication recommends, the durable claim
# is won BEFORE the paid call, the repair CLI's exit is reported
# separately, and delivery is judged by the post-repair collect walk.
devin_delivery_repair() { # usage file
  local usage_file=$1 verdict claim_rc=0 repair_rc=0
  local repair_named="$round_dir/devin-return.repair-1.json"
  local repair_transcript="$round_dir/transcript.repair-1.atif.json"
  local repair_args=(--handshake-done --named-repair-path "$repair_named")
  (( ${return_repairs:-0} == 0 )) && [[ -n "${session_id:-}" ]] && repair_args+=(--repair-available)
  verdict=$(adjudicate_turn empty-delivery "${repair_args[@]}")
  if [[ "$verdict" != delivery-repair ]]; then
    set -- $verdict
    if [[ "$1" == fail-pending ]]; then
      fail_pending "$2" "$3" "$usage_file"
    else
      finish_running "$2" "$3" "$4" "$usage_file"
    fi
    return 1
  fi

  set +e
  "$dispatch" __repair-claim --job "$job" >>"$log" 2>&1
  claim_rc=$?
  set -e
  if (( claim_rc == 3 )); then
    # The repair is spent (this round or the malformed flow): the pinned
    # empty outcome stands.
    finish_running failed empty_reply delivery "$usage_file"
    return 1
  elif (( claim_rc != 0 )); then
    finish_running failed collect_mechanical delivery "$usage_file"
    return 1
  fi
  return_repairs=1
  printf '%s delivery repair attempt 1: no return was delivered, asking session %s to write %s\n' \
    "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$session_id" "$repair_named" >>"$log"

  runtime_repair_invoke "$round_dir/repair-1.prompt.md" "$round_dir/repair-1.out" || repair_rc=$?
  runtime_usage_after_repair "$usage_file" || true
  if (( repair_rc != 0 )); then
    printf 'delivery repair provider call failed rc=%s\n' "$repair_rc" >"$round_dir/protocol-violation.txt"
    finish_protocol_error "$round_dir/protocol-violation.txt"
    return 1
  fi

  local collect_rc=0 collect_json reply_path
  set +e
  collect_json=$("$ms" adapter devin-collect --root "$root" --job "$job" \
    --round-dir "$round_dir" --workspace "$workspace" \
    --stdout "$round_dir/repair-1.out" --named "$repair_named" \
    --transcript "$repair_transcript" --record "$record" \
    --attempt repair --session "$session_id")
  collect_rc=$?
  set -e
  if (( collect_rc != 0 )); then
    printf 'delivery repair produced no qualifying return (collect rc=%s)\n' "$collect_rc" >"$round_dir/protocol-violation.txt"
    finish_protocol_error "$round_dir/protocol-violation.txt"
    return 1
  fi
  # Delivered: the repaired session settles before the pipeline runs,
  # exactly as the repaired-session path orders it today.
  if ! runtime_settle_after_repair; then
    finish_running failed session_identity_disagreement delivery "$usage_file"
    return 1
  fi
  reply_path=$(printf '%s' "$collect_json" | "$ms" json get --value "$collect_json" --field reply)
  complete_from_cli 0 "$usage_file" "$reply_path" ""
}

command_name=${1:-}
[[ -n "$command_name" ]] || { usage; exit 2; }
shift
# The declared envelope-enforcement map, served by BOTH the snapshot
# write and the side-effect-free enforcement-map verb (agnosticism B1):
# one literal, no drift.
adapter_enforcement_map='{"writeRoots":"notEnforced","readRoots":"notEnforced","network":"notEnforced"}'

case "$command_name" in
  output-stream)
    [[ ${1:-} == --round-dir && $# -eq 2 && $2 == /* ]] || { usage; exit 2; }
    transport=$(devin_transport) || { echo "devin transport refused: $transport" >&2; exit 1; }
    if [[ "$transport" == acp ]]; then
      printf '%s/acp-outcome.json\n' "${2%/}"
    else
      printf '%s/raw.out\n' "${2%/}"
    fi
    ;;
  local-config-paths)
    (($# == 0)) || { usage; exit 2; }
    printf '%s\n' .devin/config.json .devin/config.local.json .devin/hooks.v1.json
    ;;
  enforcement-map)
    (($# == 0)) || { usage; exit 2; }
    printf '%s\n' "$adapter_enforcement_map"
    ;;
  contract)
    (($# == 0)) || { usage; exit 2; }
    emit_contract_snapshot devin "$adapter_enforcement_map"
    ;;
  signature)
    (($# == 0)) || { usage; exit 2; }
    # The RAW `devin acp` helper is the HOST CLI's own internal child
    # (issue #12: `devin -p` spawns it between the announced main and
    # every tool shell, so the ancestry walk classified the orchestrator
    # DELEGATE and refused every dispatch). It is excluded; the DELEGATE
    # ACP server this adapter launches carries the distinguishable argv0
    # devin-delegate-acp, matched by its own line, so delegate tool
    # shells still classify DELEGATE.
    printf '%s\n' \
      'match ^([^[:space:]]*/)?devin([[:space:]]|$)' \
      'match ^([^[:space:]]*/)?devin-delegate-acp([[:space:]]|$)' \
      'exclude ^([^[:space:]]*/)?devin[[:space:]]+acp([[:space:]]|$)' \
      'exclude supervision-hook\.sh' \
      'exclude scripts/agents/adapters/devin\.sh'
    ;;
  identity)
    (($# == 0)) || { usage; exit 2; }
    devin_identity
    ;;
  config-identity)
    (($# == 0)) || { usage; exit 2; }
    devin_config_identity
    ;;
  probe)
    (($# == 0)) || { usage; exit 2; }
    probe
    ;;
  dispatch|follow-up) supervise "$command_name" "$@" ;;
  cancel)
    [[ ${1:-} == --job && $# -eq 2 ]] || { usage; exit 2; }
    "$dispatch" __cancel-owned --job "$2"
    ;;
  selftest)
    (($# == 0)) || { usage; exit 2; }
    # `metered`, not `native` or `unavailable`: which of those a Devin turn
    # reports depends on the ACCOUNT, not the runtime. A consumer account
    # reports token counts; an enterprise one reports ACU and no tokens at all.
    # Both must pass, and both must leave the turn measured by something the
    # mission fence can meter.
    run_full_contract_selftest metered symlinked-skill-discovery
    ;;
  -h|--help) usage ;;
  *) usage; exit 2 ;;
esac
