#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/agents/adapters/claude.sh identity
  scripts/agents/adapters/claude.sh config-identity
  scripts/agents/adapters/claude.sh signature
  scripts/agents/adapters/claude.sh enforcement-map
  scripts/agents/adapters/claude.sh probe
  scripts/agents/adapters/claude.sh dispatch --job <job-id> --start-gate <file>
      --instance-tag <tag>
  scripts/agents/adapters/claude.sh follow-up --job <job-id> --start-gate <file>
      --instance-tag <tag>
  scripts/agents/adapters/claude.sh cancel --job <job-id>
  scripts/agents/adapters/claude.sh selftest
  scripts/agents/adapters/claude.sh local-config-paths
USAGE
}

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/runtime-common.sh"
adapter_common_init claude

claude_version() {
  command -v claude >/dev/null 2>&1 || { echo "claude CLI is not installed" >&2; return 1; }
  claude --version 2>/dev/null | "$ms" adapter version-parse
}

claude_config_identity() {
  local version config_dir project_root
  local -a settings_files
  version=$(claude_version)
  config_dir=${CLAUDE_CONFIG_DIR:-${HOME:?}/.claude}
  project_root=$(git -C "$root" rev-parse --show-toplevel)
  # Declared configuration identity: the user, project, project-local, and
  # host-managed settings sources Claude merges for a session launched here.
  settings_files=(
    "$config_dir/settings.json"
    "$project_root/.claude/settings.json"
    "$project_root/.claude/settings.local.json"
    "/Library/Application Support/ClaudeCode/managed-settings.json"
    "/etc/claude-code/managed-settings.json"
  )
  configuration_identity claude "$version" "${settings_files[@]}"
}

claude_identity() {
  local details version hash
  details=$(claude_config_identity)
  version=$(configuration_identity_field "$details" cliVersion)
  hash=$(configuration_identity_field "$details" configHash)
  printf '%s %s\n' "$version" "$hash"
}

probe() {
  local details version hash key_hashes
  details=$(claude_config_identity)
  version=$(configuration_identity_field "$details" cliVersion)
  hash=$(configuration_identity_field "$details" configHash)
  key_hashes=$(configuration_identity_field "$details" configKeyHashes)
  claude auth status >/dev/null 2>&1 || {
    echo "claude authentication is unavailable; run claude auth login" >&2
    return 1
  }
  write_capability_snapshot claude "$version" "$hash" \
    '["stdin","file","json","stream-json"]' \
    '{
      "resume": true,
      "sessionEstablishedSignal": true,
      "nativeStructuredOutput": true,
      "nativeEvents": true,
      "nativeUsage": true,
      "gracefulCancel": true,
      "hooks": true,
      "protocolServer": true,
      "nativeBudget": true
    }' \
    '{"unverified": []}' \
    "$adapter_enforcement_map" \
    "$key_hashes"
}

build_claude_settings() { # output settings
  # The emitted SessionStart hook runs the metasystem session-signal verb, which
  # signals session establishment back to this adapter.
  "$ms" adapter claude-settings --record "$record" --output "$1" --metasystem-bin "$ms"
}

claude_usage() { # result JSON, usage output
  "$ms" adapter claude-usage --result "$1" --output "$2"
}

claude_result_field() { # result JSON, field
  "$ms" adapter claude-result-field --result "$1" --field "$2"
}

supervise() { # dispatch|follow-up and supervisor args
  local verb=$1
  shift
  prepare_supervision "$verb" "$@" || { usage; return 2; }
  local settings_file="$round_dir/claude-settings.json"
  local signal_file="$round_dir/claude-session-signal.json"
  local result_file="$round_dir/claude-result.json"
  local usage_file="$round_dir/usage.json"
  local cli_pid command_status
  local signalled_session signalled_model result_session result_model
  local -a command

  record_actual_workspace_write_scope
  fail_if_effective_wider_before_launch || return 1
  : >"$events"
  build_claude_settings "$settings_file"
  # The argv, the envelope's mode/tool mapping, and the budget policy are
  # the engine's (`adapter claude-command`, script-adapters-02/D25); this
  # adapter reads the tokens back NUL by NUL like codex.sh does. Exit 3 is
  # an invalid budget, 4 an invalid turn limit.
  local -a claude_args
  local command_file="$round_dir/claude-command.nul"
  claude_args=(--record "$record" --model "$requested_model" --schema "$schema" --settings "$settings_file")
  [[ "$verb" == dispatch ]] || claude_args+=(--session "$requested_session")
  command_status=0
  "$ms" adapter claude-command "${claude_args[@]}" >"$command_file" 2>>"$log" || command_status=$?
  case "$command_status" in
    0) ;;
    3) fail_pending invalid_native_budget handshake; return 1 ;;
    4) fail_pending invalid_native_turn_limit handshake; return 1 ;;
    *) fail_pending runtime_error handshake; return 1 ;;
  esac
  command=()
  while IFS= read -r -d '' token; do command+=("$token"); done <"$command_file"
  (( ${#command[@]} > 0 )) || { fail_pending runtime_error handshake; return 1; }

  (
    cd "$workspace"
    export METASYSTEM_CLAUDE_SESSION_SIGNAL="$signal_file"
    export METASYSTEM_CLAUDE_EVENTS="$events"
    exec "${command[@]}" <"$prompt" >"$result_file" 2>>"$log"
  ) &
  cli_pid=$!
  register_cli_custody "$cli_pid" || { terminate_cli_child "$cli_pid"; fail_pending custody_registration handshake; return 1; }

  while kill -0 "$cli_pid" 2>/dev/null; do
    if [[ -s "$signal_file" ]]; then
      signalled_session=$(field "$signal_file" session_id)
      signalled_model=$(field "$signal_file" model 2>/dev/null || true)
      [[ -n "$signalled_model" && "$signalled_model" != null ]] || signalled_model=$requested_model
      if ! record_handshake "$signalled_session" "" "$signalled_model"; then
        terminate_cli_child "$cli_pid"
        return 1
      fi
      break
    fi
    touch "$heartbeat"
    sleep 0.02
  done
  wait_for_cli "$cli_pid"
  cp "$result_file" "$raw" 2>/dev/null || : >"$raw"
  "$ms" adapter claude-append-result --result "$result_file" --events "$events"
  claude_usage "$result_file" "$usage_file"

  result_session=$(claude_result_field "$result_file" session_id 2>/dev/null || true)
  result_model=$(claude_result_field "$result_file" model 2>/dev/null || true)
  settle_result_identity "$result_session" "" "$result_model" "$result_model" "$usage_file" || return 1
  complete_from_cli "$cli_status" "$usage_file" "$result_file"
}

command_name=${1:-}
[[ -n "$command_name" ]] || { usage; exit 2; }
shift
# The declared envelope-enforcement map, served by BOTH the snapshot
# write and the side-effect-free enforcement-map verb (agnosticism B1):
# one literal, no drift.
adapter_enforcement_map='{"writeRoots":"mapped","readRoots":"mapped","network":"mapped"}'

case "$command_name" in
  local-config-paths)
    (($# == 0)) || { usage; exit 2; }
    printf '%s\n' .claude/settings.json .claude/settings.local.json
    ;;
  enforcement-map)
    (($# == 0)) || { usage; exit 2; }
    printf '%s\n' "$adapter_enforcement_map"
    ;;
  signature)
    (($# == 0)) || { usage; exit 2; }
    printf '%s\n' \
      'match ^([^[:space:]]*/)?claude([[:space:]]|$)' \
      'exclude claude-session-signal\.py' \
      'exclude supervision-hook\.sh' \
      'exclude scripts/agents/adapters/claude\.sh'
    ;;
  identity)
    (($# == 0)) || { usage; exit 2; }
    claude_identity
    ;;
  config-identity)
    (($# == 0)) || { usage; exit 2; }
    claude_config_identity
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
    run_full_contract_selftest native
    ;;
  -h|--help) usage ;;
  *) usage; exit 2 ;;
esac
