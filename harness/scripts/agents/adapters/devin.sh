#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/agents/adapters/devin.sh identity
  scripts/agents/adapters/devin.sh signature
  scripts/agents/adapters/devin.sh probe
  scripts/agents/adapters/devin.sh dispatch --job <job-id> --start-gate <file>
      --instance-tag <tag>
  scripts/agents/adapters/devin.sh follow-up --job <job-id> --start-gate <file>
      --instance-tag <tag>
  scripts/agents/adapters/devin.sh cancel --job <job-id>
  scripts/agents/adapters/devin.sh selftest
USAGE
}

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/runtime-common.sh"
adapter_common_init devin

devin_version() {
  command -v devin >/dev/null 2>&1 || { echo "devin CLI is not installed" >&2; return 1; }
  devin --version 2>/dev/null | python3 -c '
import re, sys
text = sys.stdin.read()
match = re.search(r"[0-9]+(?:\.[0-9A-Za-z_-]+)+", text)
if not match: raise SystemExit("could not parse devin CLI version")
print(match.group(0))
'
}

devin_identity() {
  local version hash config_dir
  local -a settings_files
  version=$(devin_version)
  config_dir=${XDG_CONFIG_HOME:-${HOME:?}/.config}
  settings_files=(
    "$config_dir/devin/config.json"
    "$config_dir/devin/hooks.v1.json"
    "$root/.devin/config.json"
    "$root/.devin/config.local.json"
    "$root/.devin/hooks.v1.json"
  )
  hash=$(configuration_hash "${settings_files[@]}")
  printf '%s %s\n' "$version" "$hash"
}

probe() {
  local version hash
  read -r version hash <<<"$(devin_identity)"
  devin auth status >/dev/null 2>&1 || {
    echo "devin authentication is unavailable; run devin auth login" >&2
    return 1
  }
  write_capability_snapshot devin "$version" "$hash" \
    '["file","stdout","atif","acp"]' \
    '{
      "resume": true,
      "sessionEstablishedSignal": false,
      "nativeStructuredOutput": false,
      "nativeEvents": false,
      "nativeUsage": false,
      "gracefulCancel": false,
      "hooks": true,
      "protocolServer": true,
      "nativeBudget": false
    }' \
    '{"unverified": []}'
}

build_devin_config() { # output
  python3 - "$record" "$1" <<'PY'
import json, sys
from pathlib import Path

record = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
requested = record["permissions"]["requested"]
read_roots = requested["readRoots"]
write_roots = requested["writeRoots"]
allow = ["read", "grep", "glob"]
allow.extend(f"Read({root}/**)" for root in read_roots)
deny = ["Fetch(*)", "mcp__*"]
if write_roots:
    allow.extend(["edit", "exec"])
    allow.extend(f"Write({root}/**)" for root in write_roots)
else:
    deny.extend(["edit", "exec", "Write(**)"])
value = {
    "permissions": {"allow": allow, "ask": [], "deny": deny},
    "sandbox": {
        # A non-resolving allowlist sentinel activates allowlist mode without
        # granting a usable network destination.
        "allowed_domains": ["harness.invalid"],
        "denied_domains": [],
        "network_mode": "limited",
        "excluded": {"allow": [], "ask": [], "deny": ["Exec(*)"]},
    },
}
Path(sys.argv[2]).write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
}

list_devin_sessions() { # output file
  local output=$1
  set +e
  (cd "$workspace" && devin list --format json) >"$output" 2>>"$log"
  local status=$?
  set -e
  return "$status"
}

new_devin_session() { # before list, current list, optional hook signal
  python3 - "$1" "$2" "$3" <<'PY'
import json, sys
from pathlib import Path

before_path, current_path, signal_path = map(Path, sys.argv[1:])

def load(path, fallback):
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return fallback

signal = load(signal_path, {})
for key in ("session_id", "sessionId", "id"):
    value = signal.get(key) if isinstance(signal, dict) else None
    if isinstance(value, str) and value:
        print(value)
        raise SystemExit(0)

def records(value):
    if isinstance(value, dict):
        if any(key in value for key in ("session_id", "sessionId")):
            yield value
        for child in value.values():
            yield from records(child)
    elif isinstance(value, list):
        for child in value:
            yield from records(child)

def session_id(record):
    for key in ("session_id", "sessionId", "id"):
        value = record.get(key)
        if isinstance(value, str) and value:
            return value
    return None

before = {session_id(item) for item in records(load(before_path, []))}
candidates = []
for item in records(load(current_path, [])):
    identifier = session_id(item)
    if not identifier or identifier in before:
        continue
    timestamp = ""
    for key in ("created_at", "createdAt", "updated_at", "updatedAt", "timestamp"):
        if isinstance(item.get(key), str):
            timestamp = item[key]
            break
    candidates.append((timestamp, identifier))
if candidates:
    print(max(candidates)[1])
    raise SystemExit(0)
raise SystemExit(1)
PY
}

devin_usage() { # output
  # D2 residual gate: ordinary `devin -p` has no confirmed per-session token
  # or cost fields. Keep usage explicitly unavailable until the human-run
  # selftest observes and documents a stable local shape; cloud ACUs do not
  # get estimated or mixed into this local record.
  python3 - "$1" <<'PY'
import json, sys
from pathlib import Path
Path(sys.argv[1]).write_text(json.dumps({
    "availability": "unavailable",
    "inputTokens": None,
    "cachedInputTokens": None,
    "outputTokens": None,
    "reasoningTokens": None,
    "cost": None,
    "providerUnits": None,
}, sort_keys=True) + "\n", encoding="utf-8")
PY
}

supervise() { # dispatch|follow-up and supervisor args
  local verb=$1
  shift
  prepare_supervision "$verb" "$@" || { usage; return 2; }
  local config_file="$round_dir/devin-config.json"
  local transcript="$round_dir/transcript.atif.json"
  local before_sessions="$round_dir/devin-sessions-before.json"
  local current_sessions="$round_dir/devin-sessions-current.json"
  local signal_file="$round_dir/devin-session-signal.json"
  local usage_file="$round_dir/usage.json"
  local cli_pid output_seen=0 resolved_session
  local -a command

  record_actual_workspace_write_scope
  fail_if_effective_wider_before_launch || return 1
  : >"$events"
  : >"$raw"
  build_devin_config "$config_file"
  list_devin_sessions "$before_sessions" || printf '[]\n' >"$before_sessions"
  command=(
    devin -p
    --prompt-file "$prompt"
    --respect-workspace-trust false
    --model "$requested_model"
    --permission-mode autonomous
    --sandbox
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
    export HARNESS_DEVIN_SESSION_SIGNAL="$signal_file"
    exec "${command[@]}" >"$raw" 2>>"$log"
  ) &
  cli_pid=$!
  register_cli_custody "$cli_pid" || { terminate_cli_child "$cli_pid"; fail_pending custody_registration handshake; return 1; }

  while kill -0 "$cli_pid" 2>/dev/null; do
    [[ -s "$raw" ]] && output_seen=1
    if (( output_seen )); then
      if [[ "$verb" == follow-up ]]; then
        resolved_session=$requested_session
      else
        list_devin_sessions "$current_sessions" || true
        resolved_session=$(new_devin_session "$before_sessions" "$current_sessions" "$signal_file" 2>/dev/null || true)
      fi
      if [[ -n "$resolved_session" ]]; then
        # Devin's snapshot declares the weak predicate: first output, the
        # process still alive, and a session id correlated by list/hook.
        if ! record_handshake "$resolved_session" "" "$requested_model"; then
          terminate_cli_child "$cli_pid"
          return 1
        fi
        printf '{"type":"session-correlated","session_id":"%s","predicate":"first-output-plus-live-process"}\n' \
          "$resolved_session" >>"$events"
        break
      fi
    fi
    touch "$heartbeat"
    sleep 0.05
  done
  wait_for_cli "$cli_pid"
  # D2 residual gate: Devin CLI exit-code meanings are undocumented. Until the
  # user-run selftest records them, zero means candidate success and every
  # nonzero value is preserved as the adapter's generic runtime_error path.
  printf 'devin cli exit status=%s\n' "$cli_status" >>"$log"
  devin_usage "$usage_file"

  complete_from_cli "$cli_status" "$usage_file" "$raw" "$transcript"
}

command_name=${1:-}
[[ -n "$command_name" ]] || { usage; exit 2; }
shift
case "$command_name" in
  signature)
    (($# == 0)) || { usage; exit 2; }
    printf '%s\n' \
      'match (^|[[:space:]])([^[:space:]]*/)?devin([[:space:]]|$)' \
      'exclude supervision-hook\.sh' \
      'exclude scripts/agents/adapters/devin\.sh'
    ;;
  identity)
    (($# == 0)) || { usage; exit 2; }
    devin_identity
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
    # D2 residual gate: this live run is the acceptance point for exact resume
    # behavior and symlinked `.agents/skills` discovery on the user's Devin
    # machine; neither is silently promoted from documentation to observation.
    run_full_contract_selftest unavailable 1
    ;;
  -h|--help) usage ;;
  *) usage; exit 2 ;;
esac
