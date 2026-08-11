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

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd -P)
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"

atomic_result() { # result path, session, outcome, usage JSON path, raw, return or empty
  "$ms" host result-write --result "$1" --session "$2" --outcome "$3" \
    --usage-file "$4" --raw "$5" --return-path "${6:-}"
}

wait_for_start_gate() {
  local gate=${METASYSTEM_HOST_START_GATE:-} cap=${METASYSTEM_HOST_START_GATE_TIMEOUT_SEC:-10} started=$SECONDS
  local poll_ms=${METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS:-20} poll_sleep
  [[ -z "$gate" ]] && return 0
  [[ "$cap" =~ ^[1-9][0-9]*$ ]] || return 3
  [[ "$poll_ms" =~ ^[1-9][0-9]*$ ]] || return 3
  printf -v poll_sleep '%d.%03d' "$((poll_ms / 1000))" "$((poll_ms % 1000))"
  while [[ ! -e "$gate" ]]; do
    (( SECONDS - started < cap )) || return 3
    sleep "$poll_sleep"
  done
}

command_name=${1:-}
if [[ "$command_name" == -h || "$command_name" == --help ]]; then usage; exit 0; fi
[[ "$command_name" == start-turn ]] || { usage; exit 2; }
shift
mission= turn_id= prompt= result= resume_session= instance_tag=
while (($#)); do
  case "$1" in
    --mission) [[ $# -ge 2 ]] || { usage; exit 2; }; mission=$2; shift 2 ;;
    --turn-id) [[ $# -ge 2 ]] || { usage; exit 2; }; turn_id=$2; shift 2 ;;
    --prompt) [[ $# -ge 2 ]] || { usage; exit 2; }; prompt=$2; shift 2 ;;
    --result) [[ $# -ge 2 ]] || { usage; exit 2; }; result=$2; shift 2 ;;
    --resume-session) [[ $# -ge 2 ]] || { usage; exit 2; }; resume_session=$2; shift 2 ;;
    --instance-tag) [[ $# -ge 2 ]] || { usage; exit 2; }; instance_tag=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) usage; exit 2 ;;
  esac
done
[[ "$mission" =~ ^[a-z0-9][a-z0-9-]*$ && "$turn_id" =~ ^[a-z0-9][a-z0-9-]*$ ]] || { usage; exit 2; }
[[ -f "$prompt" && -n "$result" && -n "$instance_tag" ]] || { usage; exit 2; }
command -v devin >/dev/null 2>&1 || { echo "devin CLI is not installed" >&2; exit 3; }
wait_for_start_gate || { echo "devin host start gate was not released" >&2; exit 3; }

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
if (( cli_status != 0 )); then
  atomic_result "$result" "$session" failed "$usage_path" "$raw" ""
  exit 3
fi
# Exit 0 with no reply is this runtime's shape for "could not do it": the turn
# ends the moment a tool is denied. Treating it as success would hand the runner
# an empty return and blame the wrong thing.
if [[ ! -s "$raw" ]]; then
  atomic_result "$result" "$session" failed "$usage_path" "$raw" ""
  exit 3
fi
# The adapter is a witness, not a judge: a rotated session is reported in the
# result envelope and judged once, at the runner's adjudication. Only a
# MISSING session stays this adapter's own fault signal (exit 6).
if [[ -z "$session" ]]; then
  atomic_result "$result" "$session" unresumable "$usage_path" "$raw" "$return_path"
  exit 6
fi
# Publish this turn's cumulative totals into the per-session store so the next
# turn of THIS session subtracts the right predecessor. Keyed by the observed
# session, written only on a completion that names one.
if [[ -s "$cumulative" ]]; then
  completed_key=$("$ms" util slug "$session")
  cp "$cumulative" "$session_store/$completed_key.json" 2>/dev/null || true
fi
atomic_result "$result" "$session" completed "$usage_path" "$raw" "$return_path"
