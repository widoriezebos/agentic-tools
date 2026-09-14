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

# The transport selector mirrors the delegate adapter's devin_transport()
# (D81/D82) on the SAME configuration key: the dispatch flip extends to
# host turns. An ABSENT key still resolves legacy (pre-flip
# configurations keep their meaning, D61's waiver stands there); an
# unreadable configuration or an unrecognized value REFUSES — a broken
# config must never fail open into the dangerous path.
host_transport=
host_transport_value=$(
  "$root/scripts/metasystem-config.sh" get --key dispatch.transport.devin --default legacy 2>/dev/null
) || {
  echo "devin host: transport configuration unreadable" >&2
  exit 3
}
case "$host_transport_value" in
  legacy|acp) host_transport=$host_transport_value ;;
  *) echo "devin host: transport configuration invalid: $host_transport_value" >&2; exit 3 ;;
esac

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

if [[ "$host_transport" == acp ]]; then
  # The ACP host turn: the same wire the delegate rides (the script owns
  # fifos and the server child; `acp turn` owns the wire), without the
  # dispatch record machinery a delegate carries — the host's evidence is
  # its turn directory and the result envelope's transport pin. The
  # session mode comes from the host permission envelope's tools grade,
  # so the session carries the v1 grades rather than the dangerous mode.
  outcome_file="$turn_dir/acp-outcome.json"
  journal_file="$turn_dir/acp-journal.log"
  session_file="$turn_dir/acp-session-id"
  "$ms" acp preflight --envelope-file "$permissions" >>"$log" 2>&1 \
    || { echo "devin host: ACP preflight refused the permission envelope" >&2; exit 3; }
  acp_grade=$("$ms" json get --file "$permissions" --field tools --default "")
  acp_mode=$("$ms" acp mode --runtime devin --tools "$acp_grade" 2>>"$log") \
    || { echo "devin host: no ACP session mode maps to tools grade '$acp_grade'" >&2; exit 3; }
  expected_protocol=$("$ms" json get \
    --value "$("$ms" runtime acp-expectation devin)" \
    --field expectedProtocolVersion --default "") || expected_protocol=
  [[ -n "$expected_protocol" ]] \
    || { echo "devin host: the runtime registry declares no ACP protocol expectation" >&2; exit 3; }
  acp_nonce=$("$ms" util token-hex --bytes 6)
  server_out="$turn_dir/acp-$acp_nonce-out"
  server_in="$turn_dir/acp-$acp_nonce-in"
  mkfifo "$server_out" "$server_in" \
    || { echo "devin host: cannot create the ACP fifo pair" >&2; exit 3; }
  # Wire plumbing is not evidence: the pair is removed on every exit so a
  # later evidence-tree copy never meets a named pipe (KI-42).
  acp_fifo_cleanup() { rm -f -- "$server_out" "$server_in"; }
  trap acp_fifo_cleanup EXIT
  # argv0 devin-host-acp: the census signature distinguishes the HOST's
  # server child from both the raw CLI helper and the delegate-side server.
  ( cd "$root" && exec -a devin-host-acp "$(command -v devin)" acp >"$server_out" <"$server_in" 2>>"$log" ) &
  acp_server_pid=$!
  acp_turn_args=(
    acp turn --server-out "$server_out" --server-in "$server_in"
    --journal "$journal_file" --workspace "$root"
    --envelope-file "$permissions" --prompt-file "$devin_prompt"
    --mode "$acp_mode" --expected-protocol "$expected_protocol"
    --session-file "$session_file"
  )
  [[ -z "$resume_session" ]] || acp_turn_args+=(--load-session "$resume_session")
  set +e
  "$ms" "${acp_turn_args[@]}" >"$outcome_file" 2>>"$log"
  cli_status=$?
  set -e
  kill -TERM "$acp_server_pid" 2>/dev/null || true
  wait "$acp_server_pid" 2>/dev/null || true
  acp_fifo_cleanup
  trap - EXIT
  [[ -f "$raw" ]] || : >"$raw"

  accepted_reply=""
  if (( cli_status == 0 )) && [[ -s "$outcome_file" ]]; then
    acp_row=$("$ms" json get --file "$outcome_file" --field row --default "")
    if [[ "$acp_row" == delivered ]]; then
      # The wire candidate is the reply; the raw capture doubles as the
      # accepted snapshot path finish and the extractor read.
      "$ms" json get --file "$outcome_file" --field candidate --default "" >"$raw"
      accepted_reply="$raw"
      "$ms" host devin-return --raw "$raw" --output "$return_path"
    fi
  fi
  "$ms" adapter acp-usage --usage "$usage_path" --outcome "$outcome_file" >>"$log" 2>&1 || true
  session=""
  [[ ! -s "$session_file" ]] || session=$(head -1 "$session_file")
  finish_rc=0
  "$ms" host finish --result "$result" --session "$session" --usage-file "$usage_path" \
    --raw "$raw" --return-path "$return_path" --accepted-reply "$accepted_reply" \
    --cli-status "$cli_status" --require-reply --transport acp || finish_rc=$?
  exit "$finish_rc"
fi

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
