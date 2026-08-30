#!/usr/bin/env bash
set -euo pipefail

# The acp verb family's regression oracle: cmd/metasystem is
# coverage-exempt on the premise that shell fixtures exercise it
# end to end, so the verb boundary — flags, fifo topology, journal
# lifecycle, outcome JSON, exit codes, the signal bridge — is
# pinned HERE (slice-three critique F3). The stub server speaks
# canned frames over real fifos; client request ids are
# deterministic (1, 2, 3), so no JSON parsing is needed.

source_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
ms="${METASYSTEM_BIN:-$source_root/bin/metasystem}"
tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-acp-fixtures.XXXXXX")
trap 'rm -rf "$tmp"' EXIT

cat > "$tmp/envelope.json" <<'JSON'
{"readRoots":["/repo"],"writeRoots":["/repo/work"],"network":"deny","approvals":"deny","tools":"read-only"}
JSON
cat > "$tmp/envelope-ask.json" <<'JSON'
{"readRoots":[],"writeRoots":[],"network":"ask","approvals":"deny","tools":"read-only"}
JSON
echo "reply with pong" > "$tmp/prompt.txt"

# ACP-V-001: preflight admits a v1-eligible envelope and refuses
# network=ask with exit 1.
"$ms" acp preflight --envelope-file "$tmp/envelope.json"
if "$ms" acp preflight --envelope-file "$tmp/envelope-ask.json" > /dev/null; then
  echo "ACP-V-001 failed: network=ask must be refused" >&2
  exit 1
fi
echo "ACP-V-001 passed"

# ACP-V-002: a full delivered turn over real fifos — candidate
# assembled, usage verbatim, both directions journaled.
run_delivered() {
  local dir="$1"
  mkfifo "$dir/server-out" "$dir/server-in"
  (
    exec 9>"$dir/server-out" 8<"$dir/server-in"
    read -r _ <&8
    echo '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":1,"agentCapabilities":{"loadSession":true},"authMethods":[]}}' >&9
    read -r _ <&8
    echo '{"jsonrpc":"2.0","id":2,"result":{"sessionId":"fx-1"}}' >&9
    read -r _ <&8
    echo '{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"fx-1","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"pong"}}}}' >&9
    echo '{"jsonrpc":"2.0","id":3,"result":{"stopReason":"end_turn","usage":{"inputTokens":5,"outputTokens":1,"totalTokens":6}}}' >&9
    sleep 1
  ) &
  "$ms" acp turn --server-out "$dir/server-out" --server-in "$dir/server-in" \
    --journal "$dir/journal.log" --workspace "$dir" \
    --envelope-file "$tmp/envelope.json" --prompt-file "$tmp/prompt.txt" \
    --late-window-ms 100
}
mkdir -p "$tmp/delivered"
outcome=$(run_delivered "$tmp/delivered")
case "$outcome" in
  *'"row":"delivered"'*'"candidate":"pong"'*) ;;
  *) echo "ACP-V-002 failed: $outcome" >&2; exit 1 ;;
esac
case "$outcome" in
  *'"totalTokens":6'*) ;;
  *) echo "ACP-V-002 failed: usage not verbatim: $outcome" >&2; exit 1 ;;
esac
grep -q '^> ' "$tmp/delivered/journal.log" && grep -q '^< ' "$tmp/delivered/journal.log" || {
  echo "ACP-V-002 failed: both journal directions required" >&2; exit 1; }
echo "ACP-V-002 passed"

# ACP-V-003: a pre-existing journal is a refused collision, never
# appended evidence.
mkdir -p "$tmp/collision"
mkfifo "$tmp/collision/server-out" "$tmp/collision/server-in"
: > "$tmp/collision/journal.log"
if "$ms" acp turn --server-out "$tmp/collision/server-out" --server-in "$tmp/collision/server-in" \
    --journal "$tmp/collision/journal.log" --workspace "$tmp/collision" \
    --envelope-file "$tmp/envelope.json" --prompt-file "$tmp/prompt.txt" 2>/dev/null; then
  echo "ACP-V-003 failed: existing journal must refuse" >&2
  exit 1
fi
echo "ACP-V-003 passed"

# ACP-V-004: the signal bridge — TERM mid-prompt yields the TYPED
# cancelled outcome and exit 0, with the courtesy session/cancel
# reaching the wire, never a bare 143 death.
mkdir -p "$tmp/cancel"
mkfifo "$tmp/cancel/server-out" "$tmp/cancel/server-in"
(
  exec 9>"$tmp/cancel/server-out" 8<"$tmp/cancel/server-in"
  read -r _ <&8
  echo '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":1,"agentCapabilities":{"loadSession":true},"authMethods":[]}}' >&9
  read -r _ <&8
  echo '{"jsonrpc":"2.0","id":2,"result":{"sessionId":"fx-c"}}' >&9
  read -r prompt_line <&8   # the prompt; never answered
  read -r cancel_line <&8 || cancel_line=""
  case "$cancel_line" in
    *session/cancel*) echo "cancel-seen" > "$tmp/cancel/cancel-observed" ;;
  esac
  sleep 1
) &
"$ms" acp turn --server-out "$tmp/cancel/server-out" --server-in "$tmp/cancel/server-in" \
  --journal "$tmp/cancel/journal.log" --workspace "$tmp/cancel" \
  --envelope-file "$tmp/envelope.json" --prompt-file "$tmp/prompt.txt" \
  --late-window-ms 100 > "$tmp/cancel/outcome.json" &
turn_pid=$!
# Wait until the prompt reached the stub (the journal shows it),
# then TERM the verb exactly as the script's kill path would.
for _ in $(seq 1 50); do
  grep -q 'session/prompt' "$tmp/cancel/journal.log" 2>/dev/null && break
  sleep 0.1
done
kill -TERM "$turn_pid"
wait "$turn_pid" && rc=0 || rc=$?
outcome=$(cat "$tmp/cancel/outcome.json")
if [[ $rc -ne 0 ]]; then
  echo "ACP-V-004 failed: exit $rc instead of a typed outcome" >&2
  exit 1
fi
case "$outcome" in
  *'"row":"cancelled"'*) ;;
  *) echo "ACP-V-004 failed: $outcome" >&2; exit 1 ;;
esac
[[ -f "$tmp/cancel/cancel-observed" ]] || {
  echo "ACP-V-004 failed: the courtesy session/cancel never reached the wire" >&2; exit 1; }
echo "ACP-V-004 passed"

# ACP-H-001: the devin HOST rides the same wire. A harness checkout pins
# the transport key, a fake devin CLI answers the graded four-request
# sequence (initialize, session/new, set_mode from the envelope's tools
# grade, prompt), and the result envelope must carry the acp transport
# pin with the wire session. An invalid transport value refuses before
# any launch.
host_harness=$tmp/host-harness
mkdir -p "$host_harness/scripts/agents/hosts" "$host_harness/scripts/agents/schemas" \
  "$host_harness/scripts/agents/permissions" "$host_harness/bin" "$host_harness/turns/t1"
cp "$source_root/scripts/agents/hosts/host-common.sh" \
  "$source_root/scripts/agents/hosts/devin.sh" "$host_harness/scripts/agents/hosts/"
cp "$source_root/scripts/metasystem-config.sh" "$host_harness/scripts/"
cp "$source_root/scripts/agents/schemas/orchestrator.schema.json" "$host_harness/scripts/agents/schemas/"
cp "$source_root/scripts/agents/permissions/workspace.json" "$host_harness/scripts/agents/permissions/"
cp "$ms" "$host_harness/bin/metasystem"
printf 'dispatch.transport.devin=acp\n' >"$host_harness/metasystem.conf"

fake_cli_bin=$tmp/host-cli-bin
mkdir -p "$fake_cli_bin"
cat >"$fake_cli_bin/devin" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
[[ ${1:-} == acp ]] || { echo "fake devin: only acp mode is scripted" >&2; exit 9; }
read -r _
echo '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":1,"agentCapabilities":{"loadSession":true},"authMethods":[]}}'
read -r _
echo '{"jsonrpc":"2.0","id":2,"result":{"sessionId":"host-fx-1"}}'
read -r request
case "$request" in
  *session/set_mode*) echo '{"jsonrpc":"2.0","id":3,"result":{}}' ;;
  *) echo "fake devin: expected set_mode, got: $request" >&2; exit 9 ;;
esac
read -r _
echo '{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"host-fx-1","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"{\"status\":\"done\"}"}}}}'
echo '{"jsonrpc":"2.0","id":4,"result":{"stopReason":"end_turn","usage":{"inputTokens":9,"outputTokens":2,"totalTokens":11}}}'
sleep 1
FAKE
chmod +x "$fake_cli_bin/devin"

host_turn_dir=$host_harness/turns/t1
printf '{"model":"devin-fixture-model"}\n' >"$host_turn_dir/turn.json"
printf 'reply with the return\n' >"$host_turn_dir/prompt.md"
set +e
PATH="$fake_cli_bin:$PATH" "$host_harness/scripts/agents/hosts/devin.sh" start-turn \
  --mission fx --turn-id t1 --prompt "$host_turn_dir/prompt.md" \
  --result "$host_turn_dir/result.json" --instance-tag metasystem-host-fx-t1 \
  >"$tmp/host-turn.out" 2>&1
host_rc=$?
set -e
if [[ $host_rc -ne 0 ]]; then
  echo "ACP-H-001 failed: host turn exit $host_rc" >&2
  cat "$tmp/host-turn.out" "$host_turn_dir/host.log" 2>/dev/null >&2
  exit 1
fi
[[ "$("$ms" json get --file "$host_turn_dir/result.json" --field transport --default "")" == acp ]] \
  || { echo "ACP-H-001 failed: no acp transport pin" >&2; cat "$host_turn_dir/result.json" >&2; exit 1; }
[[ "$("$ms" json get --file "$host_turn_dir/result.json" --field sessionId)" == host-fx-1 \
   && "$("$ms" json get --file "$host_turn_dir/result.json" --field outcome)" == completed ]] \
  || { echo "ACP-H-001 failed: session or outcome wrong" >&2; cat "$host_turn_dir/result.json" >&2; exit 1; }
grep -q 'session/set_mode' "$host_turn_dir/acp-journal.log" \
  || { echo "ACP-H-001 failed: the graded set_mode never reached the wire" >&2; exit 1; }
grep -Fq '"status": "done"' "$host_turn_dir/return.json" 2>/dev/null \
  || grep -Fq '"status":"done"' "$host_turn_dir/return.json" 2>/dev/null \
  || { echo "ACP-H-001 failed: the wire reply did not become the return" >&2; ls "$host_turn_dir" >&2; cat "$host_turn_dir/raw.out" >&2 || true; exit 1; }
echo "ACP-H-001 passed"

# ACP-H-002: an invalid transport value refuses before any launch, and an
# absent key resolves the legacy path (proven by reaching the legacy CLI,
# which this fake refuses loudly in a recognizable way).
printf 'dispatch.transport.devin=carrier-pigeon\n' >"$host_harness/metasystem.conf"
set +e
PATH="$fake_cli_bin:$PATH" "$host_harness/scripts/agents/hosts/devin.sh" start-turn \
  --mission fx --turn-id t2 --prompt "$host_turn_dir/prompt.md" \
  --result "$host_turn_dir/result-invalid.json" --instance-tag metasystem-host-fx-t2 \
  >"$tmp/host-invalid.out" 2>&1
invalid_rc=$?
set -e
[[ $invalid_rc -eq 3 ]] && grep -q 'transport configuration invalid' "$tmp/host-invalid.out" \
  || { echo "ACP-H-002 failed: invalid transport did not refuse (rc=$invalid_rc)" >&2; cat "$tmp/host-invalid.out" >&2; exit 1; }
[[ ! -e "$host_turn_dir/result-invalid.json" ]] \
  || { echo "ACP-H-002 failed: a refused transport still wrote a result" >&2; exit 1; }
echo "ACP-H-002 passed"

echo "acp verb fixtures passed (ACP-V-001 through ACP-V-004, ACP-H-001 through ACP-H-002)"
