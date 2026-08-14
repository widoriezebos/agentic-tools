#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/agents/adapters/devin.sh identity
  scripts/agents/adapters/devin.sh config-identity
  scripts/agents/adapters/devin.sh signature
  scripts/agents/adapters/devin.sh probe
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
    '{"writeRoots":"notEnforced","readRoots":"notEnforced","network":"notEnforced"}' \
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
      --cumulative "$3" --previous "${4:-}" --expect-previous
  else
    "$ms" adapter devin-usage --usage "$1" --transcript "$2" \
      --cumulative "$3" --previous "${4:-}"
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
devin_settle() { # transcript, extra verb flags -> prints model; rc 1 = not certified
  "$ms" adapter devin-settle --transcript "$1" --session "${session_id:-}" \
    --round-dir "$round_dir" "${@:2}"
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
    "${previous:-}" "${expect_previous:-0}"
}

# The repair turn the shared contract calls when a reply does not validate. It
# resumes the SAME session, so the delegate still has everything it read and
# concluded; only the shape is being asked for again. Same model, same envelope,
# same config -- a repair that changed any of those would not be a repair.
runtime_settle_after_repair() { # -> nonzero when the repair cannot be confirmed
  local repair_transcript="$round_dir/transcript.repair-1.atif.json" observed settle_rc=0
  observed=$(devin_settle "$repair_transcript" --require-transcript) || settle_rc=$?
  # The observed model records even when certification fails: the record
  # must reflect what the transcript actually named.
  [[ -z "$observed" ]] || record_result_effective_model "$observed" || true
  return "$settle_rc"
}

runtime_repair_turn() { # prompt file, output file
  local repair_prompt=$1 output=$2 repair_status
  [[ -n "${session_id:-}" && -n "${config_file:-}" ]] || return 1
  set +e
  ( cd "$workspace" && exec devin -p \
      --prompt-file "$repair_prompt" \
      --respect-workspace-trust false \
      --model "$requested_model" \
      --permission-mode "${permission_mode:-auto}" \
      --config "$config_file" \
      -r "$session_id" \
      --export "$round_dir/transcript.repair-1.atif.json" ) >"$output" 2>>"$log"
  repair_status=$?
  set -e
  # A repair that exits nonzero has failed, even if it printed something that
  # happens to parse: a failed provider call must not be turned into a completed
  # job by a lucky-looking stdout. Both a clean exit AND non-empty output are
  # required.
  (( repair_status == 0 )) && [[ -s "$output" ]]
}

supervise() { # dispatch|follow-up and supervisor args
  local verb=$1
  shift
  prepare_supervision "$verb" "$@" || { usage; return 2; }
  config_file="$round_dir/devin-config.json"
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
  "$ms" adapter devin-prompt --prompt "$prompt" --schema "$schema" --output "$devin_prompt"

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
  # dangerous; a role with no write roots gets `auto` with edit and exec denied,
  # a write-capable role gets `accept-edits`, and `dangerous` is never used.
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

  (
    cd "$workspace"
    # Existing Devin hooks can backfill this file from their stable session_id
    # payload. The baseline remains `devin list --format json`, because the
    # adapter cannot install repository hooks into a delegate worktree.
    export METASYSTEM_DEVIN_SESSION_SIGNAL="$signal_file"
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
  devin_usage "$usage_file" "$transcript" "$cumulative" "$previous" "$expect_previous"
  settle_observed=$(devin_settle "$transcript") || settle_failed=1
  [[ -z "$settle_observed" ]] || record_result_effective_model "$settle_observed" || true
  # The transcript is authoritative for session identity once the turn ends.
  if (( handshake_done )) && (( ${settle_failed:-0} )); then
    finish_running failed session_identity_disagreement delivery "$usage_file"
    return 1
  fi

  # Exit 0 with no reply is this runtime's shape for "could not do it", so it
  # is named rather than left to fail later as a missing return. Which failure
  # writer applies depends on where the record got to (the engine's stage
  # decision, script-adapters-01/D24): a turn that never correlated is still
  # pending; a turn that correlated and then produced nothing is running.
  if (( cli_status == 0 )) && [[ ! -s "$raw" ]]; then
    local empty_args=() verdict
    (( handshake_done )) && empty_args+=(--handshake-done)
    verdict=$(adjudicate_turn empty-reply ${empty_args[@]+"${empty_args[@]}"})
    set -- $verdict
    if [[ "$1" == fail-pending ]]; then
      fail_pending "$2" "$3" "$usage_file"
    else
      finish_running "$2" "$3" "$4" "$usage_file"
    fi
    return 1
  fi
  complete_from_cli "$cli_status" "$usage_file" "$raw" "$transcript"
}

command_name=${1:-}
[[ -n "$command_name" ]] || { usage; exit 2; }
shift
case "$command_name" in
  local-config-paths)
    (($# == 0)) || { usage; exit 2; }
    printf '%s\n' .devin/config.json .devin/config.local.json .devin/hooks.v1.json
    ;;
  signature)
    (($# == 0)) || { usage; exit 2; }
    printf '%s\n' \
      'match ^([^[:space:]]*/)?devin([[:space:]]|$)' \
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
    run_full_contract_selftest metered 1
    ;;
  -h|--help) usage ;;
  *) usage; exit 2 ;;
esac
