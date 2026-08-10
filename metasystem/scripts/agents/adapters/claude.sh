#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/agents/adapters/claude.sh identity
  scripts/agents/adapters/claude.sh config-identity
  scripts/agents/adapters/claude.sh signature
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
    '{"writeRoots":"mapped","readRoots":"mapped","network":"mapped"}' \
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
  local permission_mode tools allowed_tools schema_json max_budget max_turns cli_pid
  local signalled_session signalled_model result_session result_model
  local -a command read_roots
  read_roots=()

  record_actual_workspace_write_scope
  fail_if_effective_wider_before_launch || return 1
  : >"$events"
  build_claude_settings "$settings_file"
  schema_json=$("$ms" host json-compact --file "$schema")
  if [[ $(field "$record" permissions.requested.writeRoots) == '[]' ]]; then
    permission_mode=dontAsk
    tools=Read,Glob,Grep
    allowed_tools=Read,Glob,Grep
  else
    permission_mode=acceptEdits
    tools=Bash,Edit,Write,Read,Glob,Grep,NotebookEdit
    allowed_tools=Bash,Edit,Write,Read,Glob,Grep,NotebookEdit
  fi
  max_budget=${METASYSTEM_CLAUDE_MAX_BUDGET_USD:-5.00}
  max_turns=${METASYSTEM_CLAUDE_MAX_TURNS:-50}
  [[ "$max_budget" =~ ^[0-9]+([.][0-9]+)?$ && "$max_budget" != 0 && "$max_budget" != 0.0 ]] \
    || { fail_pending invalid_native_budget handshake; return 1; }
  [[ "$max_turns" =~ ^[1-9][0-9]*$ ]] \
    || { fail_pending invalid_native_turn_limit handshake; return 1; }

  command=(
    claude -p --output-format json --model "$requested_model"
    --json-schema "$schema_json"
    --permission-mode "$permission_mode"
    --tools "$tools"
    --allowedTools "$allowed_tools"
    --settings "$settings_file"
    --max-budget-usd "$max_budget"
    --max-turns "$max_turns"
  )
  if [[ $(field "$record" permissions.requested.writeRoots) == '[]' ]]; then
    while IFS= read -r read_root; do
      read_roots+=("$read_root")
    done < <("$ms" adapter claude-read-roots --record "$record")
  fi
  for read_root in "${read_roots[@]}"; do command+=(--add-dir "$read_root"); done
  [[ "$verb" == dispatch ]] || command+=(--resume "$requested_session")

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
  if (( ! handshake_done )) && [[ -n "$result_session" ]]; then
    record_handshake "$result_session" "" "$result_model" || return 1
  elif (( handshake_done )) && [[ -n "$result_session" && "$result_session" != "$session_id" ]]; then
    finish_running failed resume_collision resume "$usage_file"
    return 1
  elif (( handshake_done )) && [[ -n "$result_model" ]]; then
    record_result_effective_model "$result_model" || return 1
  fi
  complete_from_cli "$cli_status" "$usage_file" "$result_file"
}

command_name=${1:-}
[[ -n "$command_name" ]] || { usage; exit 2; }
shift
case "$command_name" in
  local-config-paths)
    (($# == 0)) || { usage; exit 2; }
    printf '%s\n' .claude/settings.json .claude/settings.local.json
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
