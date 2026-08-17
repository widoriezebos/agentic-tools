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

# The host edits AND executes in the repository it is advancing — an
# orchestrator's turn is mostly shell (dispatch, gates, git reads).
# The first live devin-hosted mission (bm-2d rep 1, 2026-08-17) proved
# accept-edits insufficient: non-interactive devin REJECTED the first
# confirmable exec call ("rejected a tool call that requires
# confirmation... Use --permission-mode dangerous"), the host exited 3,
# and the turn failed before doing anything. On this CLI a working
# non-interactive host requires the dangerous mode — the same waiver
# the legacy delegate path carried.
#
# The boundary those roots describe is NOT enforced on this runtime: --sandbox
# is refused by this organisation's policy, and a shell command writes outside
# the declared write root. The human accepted that residual globally on
# 2026-08-08 (and D83 confines devin hosts to the VM precisely because
# of it); the capability snapshot declares it, and this host does not
# pretend otherwise.
"$ms" host devin-config --root "$root" --output "$config_file"

# This runtime has no schema flag, so the schema goes in the prompt or the model
# invents field names -- the exact failure seen on the delegate side. The
# dispatcher's prompt file is left untouched as evidence; the CLI reads the
# augmented copy.
devin_prompt="$turn_dir/prompt.devin.md"
# The named return path (D64 phase 2): this model delivers by writing
# files; the prompt names the exact path and the walk below reads it.
host_return_file="$turn_dir/devin-return.json"
if [[ -e "$host_return_file" ]]; then
  echo "stale named return file from a crashed earlier attempt: $host_return_file" >&2
  exit 3
fi
"$ms" adapter devin-prompt --prompt "$prompt" --schema "$schema" --output "$devin_prompt" \
  --return-file "$host_return_file"

devin_command=(
  devin -p
  --prompt-file "$devin_prompt"
  --respect-workspace-trust false
  --model "$model"
  --permission-mode dangerous
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

# The delivery walk (D64 phase 2): stdout, the named file, then the
# transcript's designated writes — the engine decides, this script routes.
# On delivery the accepted snapshot feeds the return extraction and the
# finish judgment; with nothing delivered the old raw path stands and the
# runner's validation reports the absence.
collect_rc=0 collect_json="" accepted_reply=""
set +e
collect_json=$("$ms" host devin-collect --root "$root" --turn-record "$turn_record" \
  --turn-dir "$turn_dir" --workspace "$root" --stdout "$raw" \
  --named "$host_return_file" --transcript "$transcript")
collect_rc=$?
set -e
if (( collect_rc == 0 )); then
  accepted_reply=$("$ms" json get --value "$collect_json" --field reply)
  "$ms" host devin-return --raw "$accepted_reply" --output "$return_path"
else
  "$ms" host devin-return --raw "$raw" --output "$return_path"
fi

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
  --raw "$raw" --return-path "$return_path" --accepted-reply "$accepted_reply" \
  --cli-status "$cli_status" --require-reply || finish_rc=$?
# Publish this turn's cumulative totals into the per-session store so the next
# turn of THIS session subtracts the right predecessor. Keyed by the observed
# session, written only on a completion that names one.
if (( finish_rc == 0 )) && [[ -s "$cumulative" ]]; then
  completed_key=$("$ms" util slug "$session")
  cp "$cumulative" "$session_store/$completed_key.json" 2>/dev/null || true
fi
exit "$finish_rc"
