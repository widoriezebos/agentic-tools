#!/usr/bin/env bash
set -euo pipefail

host_runtime=codex
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/host-common.sh"
# The runtime adapter owns Codex command construction, permission mapping,
# event identity, and typed usage. It sources runtime-common.sh and is guarded
# so hosts can reuse those contracts without entering its dispatch verb loop.
source "$root/scripts/agents/adapters/codex.sh"

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/agents/hosts/codex.sh start-turn --mission <id> --turn-id <id>
      --prompt <file> --result <file> [--resume-session <sid>]
USAGE
}


host_parse_start_turn "$@"
host_require_cli codex

turn_dir=$(cd "$(dirname "$prompt")" && pwd -P)
turn_record="$turn_dir/turn.json"
raw="$turn_dir/raw.out"
return_path="$turn_dir/return.json"
events="$turn_dir/events.jsonl"
usage_path="$turn_dir/usage.json"
log="$turn_dir/host.log"
schema="$root/scripts/agents/schemas/orchestrator.schema.json"
permissions="$root/scripts/agents/permissions/workspace.json"
model=$(field "$turn_record" model)
if [[ -z "$resume_session" ]]; then
  adapter_verb=dispatch
else
  adapter_verb=follow-up
fi
build_codex_command "$adapter_verb" "$model" "$root" "$schema" "$raw" \
  --permissions "$permissions" "$resume_session"

set +e
(
  # `codex exec resume` takes no -C. Entering the workspace before invocation
  # keeps resumed turns inside the same boundary as first turns.
  cd "$root"
  "${codex_cli_command[@]}" <"$prompt" >"$events" 2>>"$log"
)
cli_status=$?
set -e
[[ -f "$raw" ]] || : >"$raw"
codex_usage "$events" "$usage_path"
if [[ -s "$raw" ]]; then cp "$raw" "$return_path"; fi
session=$(codex_event_field "$events" session 2>/dev/null || true)
# The turn outcome is the engine's one adjudication (`host finish`,
# script-adapters-10/D26); this script propagates its exit taxonomy.
"$ms" host finish --result "$result" --session "$session" --usage-file "$usage_path" \
  --raw "$raw" --return-path "$return_path" --cli-status "$cli_status"
exit $?
