#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/agents/hosts/claude.sh start-turn --mission <id> --turn-id <id>
      --prompt <file> --result <file> [--resume-session <sid>]
USAGE
}

host_runtime=claude
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/host-common.sh"


host_parse_start_turn "$@"
host_require_cli claude

turn_dir=$(cd "$(dirname "$prompt")" && pwd -P)
turn_record="$turn_dir/turn.json"
raw="$turn_dir/raw.out"
return_path="$turn_dir/return.json"
provider_result="$turn_dir/claude-result.json"
usage_path="$turn_dir/usage.json"
log="$turn_dir/host.log"
schema="$root/scripts/agents/schemas/orchestrator.schema.json"
model=$("$ms" json get --file "$turn_record" --field model)
# One home for the argv and budget policy (`adapter claude-command`,
# script-adapters-02/D25): host mode is the record-less call — acceptEdits
# with the full tools, exactly the policy this file used to hardcode.
claude_command_args=(--model "$model" --schema "$schema")
[[ -z "$resume_session" ]] || claude_command_args+=(--session "$resume_session")
command_file="$turn_dir/claude-command.nul"
if ! "$ms" adapter claude-command "${claude_command_args[@]}" >"$command_file" 2>>"$log"; then
  echo "claude host native budget configuration is invalid" >&2
  exit 3
fi
claude_command=()
while IFS= read -r -d '' token; do claude_command+=("$token"); done <"$command_file"
(( ${#claude_command[@]} > 0 )) || { echo "claude host argv assembly failed" >&2; exit 3; }
set +e
(
  cd "$root"
  "${claude_command[@]}" <"$prompt" >"$provider_result" 2>"$log"
)
cli_status=$?
set -e
cp "$provider_result" "$raw" 2>/dev/null || : >"$raw"

"$ms" host claude-result --provider "$provider_result" --return "$return_path" --usage "$usage_path"
session=$("$ms" json get --file "$provider_result" --field session_id --default "" 2>/dev/null || true)
# The turn outcome is the engine's one adjudication (`host finish`,
# script-adapters-10/D26); this script propagates its exit taxonomy.
"$ms" host finish --result "$result" --session "$session" --usage-file "$usage_path" \
  --raw "$raw" --return-path "${return_path:-}" --cli-status "$cli_status"
exit $?
