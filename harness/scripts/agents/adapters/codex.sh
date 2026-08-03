#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/agents/adapters/codex.sh identity
  scripts/agents/adapters/codex.sh probe
  scripts/agents/adapters/codex.sh dispatch --job <job-id> --start-gate <file>
      --instance-tag <tag>
  scripts/agents/adapters/codex.sh follow-up --job <job-id> --start-gate <file>
      --instance-tag <tag>
  scripts/agents/adapters/codex.sh cancel --job <job-id>
  scripts/agents/adapters/codex.sh selftest
USAGE
}

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/runtime-common.sh"
adapter_common_init codex

codex_version() {
  command -v codex >/dev/null 2>&1 || { echo "codex CLI is not installed" >&2; return 1; }
  codex --version 2>/dev/null | python3 -c '
import re, sys
text = sys.stdin.read()
match = re.search(r"[0-9]+(?:\.[0-9A-Za-z_-]+)+", text)
if not match: raise SystemExit("could not parse codex CLI version")
print(match.group(0))
'
}

codex_identity() {
  local version hash config_dir
  local -a settings_files
  version=$(codex_version)
  config_dir=${CODEX_HOME:-${HOME:?}/.codex}
  settings_files=(
    "$config_dir/config.toml"
    "$root/.codex/config.toml"
    "/etc/codex/config.toml"
  )
  hash=$(configuration_hash "${settings_files[@]}")
  printf '%s %s\n' "$version" "$hash"
}

probe() {
  local version hash
  read -r version hash <<<"$(codex_identity)"
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
      "sessionEstablishedTimeoutSec": 10,
      "nativeStructuredOutput": true,
      "nativeEvents": true,
      "nativeUsage": true,
      "gracefulCancel": true,
      "hooks": true,
      "protocolServer": true,
      "nativeBudget": false
    }' \
    '{"unverified": []}'
}

codex_event_field() { # events JSONL, session|turn
  python3 - "$1" "$2" <<'PY'
import json, sys
from pathlib import Path
path, wanted = Path(sys.argv[1]), sys.argv[2]
if not path.is_file():
    raise SystemExit(1)
for raw in path.read_text(encoding="utf-8", errors="replace").splitlines():
    try:
        event = json.loads(raw)
    except json.JSONDecodeError:
        continue
    kind = event.get("type")
    if wanted == "session" and kind in {"thread.started", "thread.created", "session.created"}:
        value = event.get("thread_id") or event.get("session_id") or event.get("id")
        if isinstance(value, str) and value:
            print(value)
            raise SystemExit(0)
    if wanted == "turn" and kind in {"turn.started", "turn.created"}:
        value = event.get("turn_id") or event.get("id")
        if isinstance(value, str) and value:
            print(value)
            raise SystemExit(0)
raise SystemExit(1)
PY
}

codex_usage() { # events JSONL, output
  python3 - "$1" "$2" <<'PY'
import json, sys
from pathlib import Path
last = {}
for raw in Path(sys.argv[1]).read_text(encoding="utf-8", errors="replace").splitlines():
    try:
        event = json.loads(raw)
    except json.JSONDecodeError:
        continue
    usage = event.get("usage")
    if isinstance(usage, dict):
        last = usage

def first_present(names):
    for name in names:
        if name in last and last[name] is not None:
            return last[name]
    return None

value = {
    "availability": "native",
    "inputTokens": first_present(("input_tokens", "inputTokens")),
    "cachedInputTokens": first_present(("cached_input_tokens", "cachedInputTokens")),
    "outputTokens": first_present(("output_tokens", "outputTokens")),
    "reasoningTokens": first_present(
        ("reasoning_output_tokens", "reasoning_tokens", "reasoningTokens")
    ),
    "cost": None,
    "providerUnits": None,
}
Path(sys.argv[2]).write_text(json.dumps(value, sort_keys=True) + "\n", encoding="utf-8")
PY
}

toml_string() {
  python3 - "$1" <<'PY'
import json, sys
print(json.dumps(sys.argv[1]))
PY
}

supervise() { # dispatch|follow-up and supervisor args
  local verb=$1
  shift
  prepare_supervision "$verb" "$@" || { usage; return 2; }
  local usage_file="$round_dir/usage.json"
  local sandbox_mode cli_pid event_session event_turn model_toml
  local -a command

  record_actual_workspace_write_scope
  fail_if_effective_wider_before_launch || return 1
  : >"$events"
  : >"$raw"
  if [[ $(field "$record" permissions.requested.writeRoots) == '[]' ]]; then
    sandbox_mode=read-only
  else
    sandbox_mode=workspace-write
  fi

  if [[ "$verb" == dispatch ]]; then
    command=(
      codex exec --json
      -m "$requested_model"
      --sandbox "$sandbox_mode"
      -C "$workspace"
      -c 'approval_policy="never"'
      -c 'sandbox_workspace_write.network_access=false'
      --output-schema "$schema"
      -o "$raw"
      -
    )
  else
    # `codex exec resume` has no --sandbox or -C flags. The thread inherits its
    # cwd/config; supported per-turn overrides travel only through -c.
    model_toml=$(toml_string "$requested_model")
    command=(
      codex exec resume --json
      -c "model=$model_toml"
      -c "sandbox_mode=\"$sandbox_mode\""
      -c 'approval_policy="never"'
      -c 'sandbox_workspace_write.network_access=false'
      --output-schema "$schema"
      -o "$raw"
      "$requested_session"
      -
    )
  fi

  "${command[@]}" <"$prompt" >"$events" 2>>"$log" &
  cli_pid=$!
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
  if (( ! handshake_done )) && [[ -n "$event_session" ]]; then
    record_handshake "$event_session" "$event_turn" "$requested_model" || return 1
  elif (( handshake_done )) && [[ -n "$event_session" && "$event_session" != "$session_id" ]]; then
    finish_running failed resume_collision resume "$usage_file"
    return 1
  fi
  complete_from_cli "$cli_status" "$usage_file" "$raw"
}

command_name=${1:-}
[[ -n "$command_name" ]] || { usage; exit 2; }
shift
case "$command_name" in
  identity)
    (($# == 0)) || { usage; exit 2; }
    codex_identity
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
    selftest_model=$("$root/scripts/harness-config.sh" get \
      --key role.default.model.codex --default '')
    HARNESS_ROLE_DESIGN_CRITIC_MODEL_CODEX="$selftest_model" \
      run_full_contract_selftest native
    ;;
  -h|--help) usage ;;
  *) usage; exit 2 ;;
esac
