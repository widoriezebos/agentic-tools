#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/agents/adapters/claude.sh identity
  scripts/agents/adapters/claude.sh signature
  scripts/agents/adapters/claude.sh probe
  scripts/agents/adapters/claude.sh dispatch --job <job-id> --start-gate <file>
      --instance-tag <tag>
  scripts/agents/adapters/claude.sh follow-up --job <job-id> --start-gate <file>
      --instance-tag <tag>
  scripts/agents/adapters/claude.sh cancel --job <job-id>
  scripts/agents/adapters/claude.sh selftest
USAGE
}

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/runtime-common.sh"
adapter_common_init claude

claude_version() {
  command -v claude >/dev/null 2>&1 || { echo "claude CLI is not installed" >&2; return 1; }
  claude --version 2>/dev/null | python3 -c '
import re, sys
text = sys.stdin.read()
match = re.search(r"[0-9]+(?:\.[0-9A-Za-z_-]+)+", text)
if not match: raise SystemExit("could not parse claude CLI version")
print(match.group(0))
'
}

claude_identity() {
  local version hash config_dir
  local -a settings_files
  version=$(claude_version)
  config_dir=${CLAUDE_CONFIG_DIR:-${HOME:?}/.claude}
  # Declared configuration identity: the user, project, project-local, and
  # host-managed settings sources Claude merges for a session launched here.
  settings_files=(
    "$config_dir/settings.json"
    "$root/.claude/settings.json"
    "$root/.claude/settings.local.json"
    "/Library/Application Support/ClaudeCode/managed-settings.json"
    "/etc/claude-code/managed-settings.json"
  )
  hash=$(configuration_hash "${settings_files[@]}")
  printf '%s %s\n' "$version" "$hash"
}

probe() {
  local version hash
  read -r version hash <<<"$(claude_identity)"
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
    '{"unverified": []}'
}

build_claude_settings() { # output settings, hook helper
  local output=$1 hook_helper=$2
  python3 - "$record" "$output" "$hook_helper" <<'PY'
import json, shlex, sys
from pathlib import Path

record = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
output, hook_helper = Path(sys.argv[2]), str(Path(sys.argv[3]).resolve())
requested = record["permissions"]["requested"]
write_roots = requested["writeRoots"]
network = requested.get("network", "deny")
allow = ["Read", "Glob", "Grep"]
deny = []
if network == "allow":
    allow.extend(["WebFetch", "WebSearch"])
else:
    deny.extend(["WebFetch", "WebSearch"])
if write_roots:
    allow.extend(["Bash", "Edit", "Write", "NotebookEdit"])
else:
    deny.extend(["Bash", "Edit", "Write", "NotebookEdit"])
settings = {
    "permissions": {"allow": allow, "ask": [], "deny": deny},
    "sandbox": {
        "enabled": True,
        "failIfUnavailable": True,
        "autoAllowBashIfSandboxed": True,
        "allowUnsandboxedCommands": False,
        "filesystem": {"allowWrite": write_roots},
        # An empty allowlist with an empty denylist permits ordinary egress;
        # the non-resolving sentinel makes every usable destination unavailable.
        # The tool layer above agrees with whichever is chosen.
        "network": ({"allowedDomains": [], "deniedDomains": []} if network == "allow"
                    else {"allowedDomains": ["metasystem.invalid"], "deniedDomains": []}),
    },
    "hooks": {
        "SessionStart": [{
            "matcher": "startup|resume",
            "hooks": [{
                "type": "command",
                "command": "python3 " + shlex.quote(hook_helper),
                "timeout": 5,
            }],
        }],
    },
}
output.write_text(json.dumps(settings, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
}

claude_usage() { # result JSON, usage output
  python3 - "$1" "$2" <<'PY'
import json, sys
from pathlib import Path

try:
    result = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
except (OSError, json.JSONDecodeError):
    result = {}
usage = result.get("usage") if isinstance(result.get("usage"), dict) else {}
cost = result.get("total_cost_usd")
value = {
    "availability": "native",
    "inputTokens": usage.get("input_tokens"),
    "cachedInputTokens": usage.get("cache_read_input_tokens"),
    "outputTokens": usage.get("output_tokens"),
    "reasoningTokens": usage.get("reasoning_tokens"),
    "cost": {"amount": cost, "currency": "USD"} if isinstance(cost, (int, float)) else None,
    "providerUnits": None,
}
Path(sys.argv[2]).write_text(json.dumps(value, sort_keys=True) + "\n", encoding="utf-8")
PY
}

claude_result_field() { # result JSON, field
  python3 - "$1" "$2" <<'PY'
import json, sys
from pathlib import Path
try:
    value = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
except (OSError, json.JSONDecodeError):
    raise SystemExit(1)
field = sys.argv[2]
if field == "model":
    models = value.get("modelUsage")
    result = next(iter(models), "") if isinstance(models, dict) else value.get("model", "")
else:
    result = value.get(field, "")
if result is not None:
    print(result)
PY
}

supervise() { # dispatch|follow-up and supervisor args
  local verb=$1
  shift
  prepare_supervision "$verb" "$@" || { usage; return 2; }
  local settings_file="$round_dir/claude-settings.json"
  local signal_file="$round_dir/claude-session-signal.json"
  local result_file="$round_dir/claude-result.json"
  local usage_file="$round_dir/usage.json"
  local hook_helper="$root/scripts/agents/adapters/claude-session-signal.py"
  local permission_mode tools allowed_tools schema_json max_budget max_turns cli_pid
  local signalled_session signalled_model result_session result_model
  local -a command read_roots
  read_roots=()

  record_actual_workspace_write_scope
  fail_if_effective_wider_before_launch || return 1
  : >"$events"
  build_claude_settings "$settings_file" "$hook_helper"
  schema_json=$(python3 - "$schema" <<'PY'
import json, sys
from pathlib import Path
print(json.dumps(json.loads(Path(sys.argv[1]).read_text(encoding="utf-8")), separators=(",", ":")))
PY
  )
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
    done < <(python3 - "$record" <<'PY'
import json, sys
from pathlib import Path
value = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
for root in value["permissions"]["requested"]["readRoots"]:
    if root != value["workspaceRoot"]:
        print(root)
PY
    )
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
  python3 - "$result_file" "$events" <<'PY'
import json, sys
from pathlib import Path
try:
    value = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
except (OSError, json.JSONDecodeError):
    raise SystemExit(0)
with Path(sys.argv[2]).open("a", encoding="utf-8") as handle:
    handle.write(json.dumps(value, separators=(",", ":")) + "\n")
PY
  claude_usage "$result_file" "$usage_file"

  result_session=$(claude_result_field "$result_file" session_id 2>/dev/null || true)
  result_model=$(claude_result_field "$result_file" model 2>/dev/null || true)
  [[ -n "$result_model" ]] || result_model=$requested_model
  if (( ! handshake_done )) && [[ -n "$result_session" ]]; then
    record_handshake "$result_session" "" "$result_model" || return 1
  elif (( handshake_done )) && [[ -n "$result_session" && "$result_session" != "$session_id" ]]; then
    finish_running failed resume_collision resume "$usage_file"
    return 1
  fi
  complete_from_cli "$cli_status" "$usage_file" "$result_file"
}

command_name=${1:-}
[[ -n "$command_name" ]] || { usage; exit 2; }
shift
case "$command_name" in
  signature)
    (($# == 0)) || { usage; exit 2; }
    printf '%s\n' \
      'match (^|[[:space:]])([^[:space:]]*/)?claude([[:space:]]|$)' \
      'exclude claude-session-signal\.py' \
      'exclude supervision-hook\.sh' \
      'exclude scripts/agents/adapters/claude\.sh'
    ;;
  identity)
    (($# == 0)) || { usage; exit 2; }
    claude_identity
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
