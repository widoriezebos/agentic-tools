#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/agents/adapters/codex.sh identity
  scripts/agents/adapters/codex.sh config-identity
  scripts/agents/adapters/codex.sh signature
  scripts/agents/adapters/codex.sh enforcement-map
  scripts/agents/adapters/codex.sh contract
  scripts/agents/adapters/codex.sh probe
  scripts/agents/adapters/codex.sh output-stream --round-dir <absolute-path>
  scripts/agents/adapters/codex.sh dispatch --job <job-id> --start-gate <file>
      --instance-tag <tag> --launch-capability <opaque-capability>
  scripts/agents/adapters/codex.sh follow-up --job <job-id> --start-gate <file>
      --instance-tag <tag> --launch-capability <opaque-capability>
  scripts/agents/adapters/codex.sh cancel --job <job-id>
  scripts/agents/adapters/codex.sh selftest
  scripts/agents/adapters/codex.sh local-config-paths
USAGE
}

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/runtime-common.sh"
adapter_common_init codex

codex_version() {
  command -v codex >/dev/null 2>&1 || { echo "codex CLI is not installed" >&2; return 1; }
  codex --version 2>/dev/null | "$ms" adapter version-parse
}

codex_config_identity() {
  local version config_dir project_root
  local -a settings_files
  version=$(codex_version)
  config_dir=${CODEX_HOME:-${HOME:?}/.codex}
  project_root=$(git -C "$root" rev-parse --show-toplevel)
  settings_files=(
    "$config_dir/config.toml"
    "$project_root/.codex/config.toml"
    "/etc/codex/config.toml"
  )
  configuration_identity codex "$version" "${settings_files[@]}"
}

codex_identity() {
  local details version hash
  details=$(codex_config_identity)
  version=$(configuration_identity_field "$details" cliVersion)
  hash=$(configuration_identity_field "$details" configHash)
  printf '%s %s\n' "$version" "$hash"
}

probe() {
  local details version hash key_hashes
  details=$(codex_config_identity)
  version=$(configuration_identity_field "$details" cliVersion)
  hash=$(configuration_identity_field "$details" configHash)
  key_hashes=$(configuration_identity_field "$details" configKeyHashes)
  codex login status >/dev/null 2>&1 || {
    echo "codex authentication is unavailable; run codex login" >&2
    return 1
  }
  # The in-process app server starts before `thread.started` is emitted, so a
  # cold Codex launch needs a wider session-establishment window.
  write_capability_snapshot codex "$version" "$hash" \
    '["stdin","jsonl","file"]' \
    '{
      "resume": true,
      "sessionEstablishedSignal": true,
      "sessionEstablishedTimeoutSec": 30,
      "nativeStructuredOutput": true,
      "nativeEvents": true,
      "nativeUsage": true,
      "gracefulCancel": true,
      "hooks": true,
      "protocolServer": true,
      "nativeBudget": false
    }' \
    '{"unverified": []}' \
    "$adapter_enforcement_map" \
    "$key_hashes"
}

codex_event_field() { # events JSONL, session|turn
  "$ms" adapter codex-event --events "$1" --field "$2"
}

codex_usage() { # events JSONL, output
  "$ms" adapter codex-usage --events "$1" --output "$2"
}

# The envelope-to-sandbox/network mapping is the engine's now
# (script-adapters-06/D25, the KI-12 lesson): codex-command derives it from
# --record or --permissions instead of receiving it pre-chewed.
build_codex_command() { # dispatch|follow-up, model, workspace, schema, output, envelope flag, envelope value, session, effort
  local verb=$1 command_model=$2 command_workspace=$3 command_schema=$4 command_output=$5
  local envelope_flag=$6 envelope_value=$7 command_session=${8:-} command_effort=${9:-} token
  # `codex exec resume` has no --sandbox or -C flags: a resumed thread inherits
  # its cwd/config and takes supported per-turn overrides through -c only. The
  # argv (including the TOML-quoted model on the resume path) is assembled once
  # and read back token by token so nothing has to be requoted here.
  codex_cli_command=()
  while IFS= read -r -d '' token; do
    codex_cli_command+=("$token")
  done < <(
    "$ms" adapter codex-command \
      --verb "$verb" \
      --model "$command_model" \
      --workspace "$command_workspace" \
      --schema "$command_schema" \
      --output "$command_output" \
      --instance-tag "$instance_tag" \
      --reasoning-effort "$command_effort" \
      "$envelope_flag" "$envelope_value" \
      --session "$command_session"
  )
  (( ${#codex_cli_command[@]} > 0 )) || return 2
}

supervise() { # dispatch|follow-up and supervisor args
  local verb=$1
  shift
  prepare_supervision "$verb" "$@" || { usage; return 2; }
  local usage_file="$round_dir/usage.json"
  local cli_pid event_session event_turn reasoning_effort
  local -a command

  record_actual_workspace_write_scope
  fail_if_effective_wider_before_launch || return 1
  : >"$events"
  : >"$raw"
  # The envelope decides sandbox and network — in the engine, from the
  # record itself (KI-12: a hard-coded value made the recorded field
  # decorative).
  reasoning_effort=$(field "$record" reasoningEffort 2>/dev/null || true)
  [[ "$reasoning_effort" != null ]] || reasoning_effort=
  build_codex_command "$verb" "$requested_model" "$workspace" "$schema" "$raw" \
    --record "$record" "$requested_session" "$reasoning_effort"
  command=("${codex_cli_command[@]}")

  # The write boundary is the CLI's cwd. `codex exec` takes -C, but
  # `codex exec resume` has no such flag, so a resumed turn would otherwise
  # inherit the adapter's cwd (the metasystem root) and be free to write the whole
  # repository while the record still claimed the job worktree. Entering the
  # workspace makes the recorded boundary true on both paths. `exec` keeps the
  # pid, which custody registration depends on.
  local -a job_git_env=()
  while IFS= read -r assignment; do job_git_env+=("$assignment"); done < <(job_git_quarantine_env "$workspace")
  mark_cli_prefork || { fail_pending prefork_marker handshake; return 1; }
  ( cd "$workspace" && exec env ${job_git_env[@]+"${job_git_env[@]}"} "${command[@]}" ) <"$prompt" >"$events" 2>>"$log" &
  cli_pid=$!
  register_cli_custody "$cli_pid" || { terminate_cli_child "$cli_pid"; fail_pending custody_registration handshake; return 1; }
  while kill -0 "$cli_pid" 2>/dev/null; do
    event_session=$(codex_event_field "$events" session 2>/dev/null || true)
    if [[ -n "$event_session" ]]; then
      event_turn=$(codex_event_field "$events" turn 2>/dev/null || true)
      if ! record_handshake "$event_session" "$event_turn" "$requested_model"; then
        terminate_cli_child "$cli_pid"
        return 1
      fi
      break
    fi
    touch "$heartbeat"
    sleep 0.02
  done
  wait_for_cli "$cli_pid"
  codex_usage "$events" "$usage_file"
  event_session=$(codex_event_field "$events" session 2>/dev/null || true)
  event_turn=$(codex_event_field "$events" turn 2>/dev/null || true)
  settle_result_identity "$event_session" "$event_turn" "$requested_model" "" "$usage_file" || return 1
  complete_from_cli "$cli_status" "$usage_file" "$raw"
}

if [[ ${BASH_SOURCE[0]} != "$0" ]]; then
  return 0
fi

command_name=${1:-}
[[ -n "$command_name" ]] || { usage; exit 2; }
shift
# The declared envelope-enforcement map, served by BOTH the snapshot
# write and the side-effect-free enforcement-map verb (agnosticism B1):
# one literal, no drift.
adapter_enforcement_map='{"writeRoots":"mapped","readRoots":"notEnforced","network":"mapped"}'

case "$command_name" in
  output-stream)
    [[ ${1:-} == --round-dir && $# -eq 2 && $2 == /* ]] || { usage; exit 2; }
    printf '%s/events.jsonl\n' "${2%/}"
    ;;
  local-config-paths)
    (($# == 0)) || { usage; exit 2; }
    printf '%s\n' .codex/config.toml
    ;;
  enforcement-map)
    (($# == 0)) || { usage; exit 2; }
    printf '%s\n' "$adapter_enforcement_map"
    ;;
  contract)
    (($# == 0)) || { usage; exit 2; }
    emit_contract_snapshot codex "$adapter_enforcement_map"
    ;;
  signature)
    (($# == 0)) || { usage; exit 2; }
    printf '%s\n' \
      'match ^([^[:space:]]*/)?codex([[:space:]]|$)' \
      'exclude supervision-hook\.sh' \
      'exclude scripts/agents/adapters/codex\.sh'
    ;;
  identity)
    (($# == 0)) || { usage; exit 2; }
    codex_identity
    ;;
  config-identity)
    (($# == 0)) || { usage; exit 2; }
    codex_config_identity
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
    selftest_model=$("$root/scripts/metasystem-config.sh" get \
      --key role.default.model.codex --default '')
    METASYSTEM_ROLE_DESIGN_CRITIC_MODEL_CODEX="$selftest_model" \
      run_full_contract_selftest native
    ;;
  -h|--help) usage ;;
  *) usage; exit 2 ;;
esac
