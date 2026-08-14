#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/agents/hosts/devin.sh start-turn --mission <id> --turn-id <id>
      --prompt <file> --result <file> [--resume-session <sid>]
      --instance-tag <tag>
USAGE
}

host_runtime=devin
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/host-common.sh"


host_parse_start_turn "$@"
host_require_cli devin

turn_dir=$(cd "$(dirname "$prompt")" && pwd -P)
turn_record="$turn_dir/turn.json"
raw="$turn_dir/raw.out"
return_path="$turn_dir/return.json"
transcript="$turn_dir/transcript.atif.json"
cumulative="$turn_dir/session-usage.json"
config_file="$turn_dir/devin-config.json"
usage_path="$turn_dir/usage.json"
log="$turn_dir/host.log"
schema="$root/scripts/agents/schemas/orchestrator.schema.json"
permissions="$root/scripts/agents/permissions/workspace.json"
model=$("$ms" json get --file "$turn_record" --field model)

# The host edits the repository it is advancing, so it runs write-capable:
# accept-edits with the workspace permission preset. A host confined to `auto`
# with edit and exec denied could not move a mission at all.
#
# The boundary those roots describe is NOT enforced on this runtime: --sandbox
# is refused by this organisation's policy, and a shell command writes outside
# the declared write root. The human accepted that residual globally on
# 2026-08-08; the capability snapshot declares it, and this host does not
# pretend otherwise.
"$ms" host devin-config --root "$root" --output "$config_file"

# This runtime has no schema flag, so the schema goes in the prompt or the model
# invents field names -- the exact failure seen on the delegate side. The
# dispatcher's prompt file is left untouched as evidence; the CLI reads the
# augmented copy.
devin_prompt="$turn_dir/prompt.devin.md"
{
  cat "$prompt"
  printf '\n\n# Return schema, exact\n\n'
  printf 'Your reply must be ONE JSON object valid against this schema and nothing\n'
  printf 'else: no prose before or after it, no code fence, and no property this\n'
  printf 'schema does not name. Every property listed in "required" must be present.\n\n'
  cat "$schema"
} >"$devin_prompt"

devin_command=(
  devin -p
  --prompt-file "$devin_prompt"
  --respect-workspace-trust false
  --model "$model"
  --permission-mode accept-edits
  --config "$config_file"
  --export "$transcript"
)
[[ -z "$resume_session" ]] || devin_command+=(-r "$resume_session")

set +e
(
  cd "$root"
  "${devin_command[@]}" >"$raw" 2>"$log"
)
cli_status=$?
set -e
[[ -f "$raw" ]] || : >"$raw"

# Devin has no native structured output, so the return is whatever the turn
# printed, kept only when it parses as an object. Anything else leaves the
# return absent for the runner's own validation to report, rather than being
# passed on as a return that is not one.
"$ms" host devin-return --raw "$raw" --output "$return_path"

# final_metrics is CUMULATIVE for a session (a resumed turn reports the session
# total, not its own), and mission and benchmark consumers ADD turn records, so
# each turn publishes the delta against its predecessor's stored totals. A turn
# whose predecessor artifact is missing publishes unavailable rather than a
# number that would double-count every earlier turn in every aggregate.
# The predecessor is keyed by SESSION, not by whichever session-usage.json is
# newest. Picking the most recent across all turns subtracted an unrelated
# turn's totals when the immediate predecessor's artifact was missing; a
# per-session store makes the delta deterministic. The store sits in the turns
# parent so it survives across a mission's turns.
session_store="$(cd "$turn_dir/.." && pwd -P)/.session-usage"
mkdir -p "$session_store"
session_key=$("$ms" util slug "${resume_session:-}")
previous_cumulative=
if [[ -n "$resume_session" && -f "$session_store/$session_key.json" ]]; then
  previous_cumulative="$session_store/$session_key.json"
fi
expect_previous=0
[[ -n "$resume_session" ]] && expect_previous=1
"$ms" host devin-usage --transcript "$transcript" --usage "$usage_path" \
  --cumulative "$cumulative" --previous "${previous_cumulative:-}" \
  --expect-previous="$expect_previous"

session=$("$ms" json get --file "$transcript" --field session_id --default "" 2>/dev/null || true)
# The turn outcome is the engine's one adjudication (`host finish`,
# script-adapters-10/D26); this script propagates its exit taxonomy.
finish_rc=0
"$ms" host finish --result "$result" --session "$session" --usage-file "$usage_path" \
  --raw "$raw" --return-path "$return_path" --cli-status "$cli_status" --require-reply || finish_rc=$?
# Publish this turn's cumulative totals into the per-session store so the next
# turn of THIS session subtracts the right predecessor. Keyed by the observed
# session, written only on a completion that names one.
if (( finish_rc == 0 )) && [[ -s "$cumulative" ]]; then
  completed_key=$("$ms" util slug "$session")
  cp "$cumulative" "$session_store/$completed_key.json" 2>/dev/null || true
fi
exit "$finish_rc"
