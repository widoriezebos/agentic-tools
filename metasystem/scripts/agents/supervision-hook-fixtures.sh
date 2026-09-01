#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"
hook=$root/scripts/agents/supervision-hook.sh
[[ -x "$ms" ]] \
  || { echo "supervision hook fixture: binary absent; run the go gate first" >&2; exit 1; }
source "$root/scripts/agents/fixture-budget.sh"
harness_fixture_budget_init "$root"
hook_evidence_cap=$(harness_fixture_cap supervision-hook-evidence)

tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-supervision-hook-fixture.XXXXXX")
hook_process_pid=
hook_process_path=
stop_hook_process() {
  local command stop_deadline
  [[ -n "$hook_process_pid" ]] || return 0
  if kill -0 "$hook_process_pid" 2>/dev/null; then
    command=$(ps -p "$hook_process_pid" -o command= 2>/dev/null || true)
    [[ "$command" == *"$hook_process_path"* ]] \
      || { echo "supervision hook fixture could not prove ownership of timed-out process $hook_process_pid" >&2; return 1; }
    kill -TERM "$hook_process_pid"
    stop_deadline=$((SECONDS + 5))
    while kill -0 "$hook_process_pid" 2>/dev/null && (( SECONDS < stop_deadline )); do
      sleep 0.05
    done
    if kill -0 "$hook_process_pid" 2>/dev/null; then
      command=$(ps -p "$hook_process_pid" -o command= 2>/dev/null || true)
      [[ "$command" == *"$hook_process_path"* ]] \
        || { echo "supervision hook fixture lost ownership proof while stopping process $hook_process_pid" >&2; return 1; }
      kill -KILL "$hook_process_pid"
    fi
  fi
  wait "$hook_process_pid" 2>/dev/null || true
  hook_process_pid=
  hook_process_path=
}
cleanup() {
  stop_hook_process || true
  rm -rf "$tmp"
}
trap cleanup EXIT

missing_engine_evidence_ready() {
  grep -Fq 'HEALTH unknown' "$tmp/missing.out" \
    && grep -Fq 'engine missing' "$tmp/missing.out"
}

wait_for_missing_engine_evidence() { # hook process
  local hook_pid=$1 deadline=$((SECONDS + hook_evidence_cap))
  until missing_engine_evidence_ready; do
    if ! kill -0 "$hook_pid" 2>/dev/null; then
      missing_engine_evidence_ready && return 0
      return 1
    fi
    (( SECONDS < deadline )) \
      || { echo "supervision hook missing-engine fixture made no expected evidence before the ${hook_evidence_cap}s hang failsafe" >&2; return 2; }
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
  done
}

chat_line_evidence_ready() {
  [[ -f "$hook_evidence" ]] \
    && grep -Fq 'HEALTH ' "$tmp/line.out" \
    && grep -Fq 'NARRATOR DIGEST since last check-in' "$tmp/line.out" \
    && grep -Fq 'A landing moved the repository storyline to commit abc123' "$tmp/line.out" \
    && grep -Fq '"result": "OK"' "$hook_evidence" \
    && grep -Fq '"outcome": "EMITTED"' "$hook_evidence"
}

wait_for_chat_line_evidence() { # hook process
  local hook_pid=$1 deadline=$((SECONDS + hook_evidence_cap))
  until chat_line_evidence_ready; do
    if ! kill -0 "$hook_pid" 2>/dev/null; then
      chat_line_evidence_ready && return 0
      return 1
    fi
    (( SECONDS < deadline )) \
      || { echo "supervision hook chat-line fixture made no expected evidence before the ${hook_evidence_cap}s hang failsafe" >&2; return 2; }
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
  done
}

# The registry producer writes the matching runtime first and then more than a
# pipe buffer of declarations. A membership reader must let the registry query
# finish; a short-reading consumer can otherwise turn a valid match into the
# producer's broken-pipe status under pipefail.
fixture_engine=$tmp/metasystem
cat >"$fixture_engine" <<'SH'
#!/usr/bin/env bash
if [[ ${1:-} == runtime && ${2:-} == list ]]; then
  awk 'BEGIN {
    print "claude"
    for (item = 0; item < 16384; item++) {
      print "runtime-membership-padding-" item
    }
  }'
  exit $?
fi
exec "${METASYSTEM_RUNTIME_MEMBERSHIP_REAL_ENGINE:?}" "$@"
SH
chmod +x "$fixture_engine"

printf '{"session_id":"fixture","cwd":"/","hook_event_name":"Stop"}\n' >"$tmp/payload.json"
membership_rc=0
METASYSTEM_BIN="$fixture_engine" METASYSTEM_RUNTIME_MEMBERSHIP_REAL_ENGINE="$ms" \
  bash "$hook" claude stop <"$tmp/payload.json" \
    >"$tmp/hook.out" 2>"$tmp/hook.err" || membership_rc=$?
if [[ $membership_rc != 0 ]]; then
  echo "supervision hook runtime-membership fixture failed: a registered runtime was refused (exit $membership_rc)" >&2
  sed -n '1,40p' "$tmp/hook.err" >&2
  exit 1
fi

missing_rc=0
METASYSTEM_BIN="$tmp/missing-engine" bash "$hook" claude stop <"$tmp/payload.json" \
  >"$tmp/missing.out" 2>"$tmp/missing.err" &
hook_process_pid=$!
hook_process_path=$hook
missing_evidence_wait_rc=0
wait_for_missing_engine_evidence "$hook_process_pid" || missing_evidence_wait_rc=$?
if (( missing_evidence_wait_rc == 2 )); then
  stop_hook_process
  exit 1
fi
wait "$hook_process_pid" || missing_rc=$?
hook_process_pid=
hook_process_path=
[[ "$missing_rc" -eq 0 ]] || { echo "supervision hook missing-engine fixture returned $missing_rc" >&2; exit 1; }
grep -Fq 'HEALTH unknown' "$tmp/missing.out" \
  || { echo "supervision hook missing-engine fixture was silent" >&2; exit 1; }
grep -Fq 'engine missing' "$tmp/missing.out" \
  || { echo "supervision hook missing-engine fixture omitted its remedy" >&2; exit 1; }

line_root=$tmp/line-root
mkdir -p "$line_root/scripts/agents" "$line_root/bin" "$line_root/plans"
cp "$hook" "$line_root/scripts/agents/supervision-hook.sh"
cp "$ms" "$line_root/bin/metasystem"
printf '%s\n' 'metasystem.runtimes=none' >"$line_root/metasystem.conf"
printf '# Goals\n\n## Goal-free: declared 2026-08-28T00:00:00Z by human over fixture\n' >"$line_root/plans/goals.md"
git -C "$line_root" init -q -b main
git -C "$line_root" config user.name fixture
git -C "$line_root" config user.email fixture@example.invalid
git -C "$line_root" add metasystem.conf plans/goals.md
git -C "$line_root" commit -qm fixture
mkdir -p "$line_root/records"
printf '%s\n' '2026-08-29T10:00:00Z HIGHLIGHT — A landing moved the repository storyline to commit abc123. (source: commit abc123)' \
  >"$line_root/records/narrator-digest.log"
printf '{"session_id":"line-fixture","cwd":"%s","hook_event_name":"Stop"}\n' "$line_root" >"$tmp/line-payload.json"
hook_evidence=$line_root/artifacts/agents/steward/components/supervision-hook.json
line_rc=0
bash "$line_root/scripts/agents/supervision-hook.sh" claude stop <"$tmp/line-payload.json" \
  >"$tmp/line.out" 2>"$tmp/line.err" &
hook_process_pid=$!
hook_process_path=$line_root/scripts/agents/supervision-hook.sh
line_evidence_wait_rc=0
wait_for_chat_line_evidence "$hook_process_pid" || line_evidence_wait_rc=$?
if (( line_evidence_wait_rc == 2 )); then
  stop_hook_process
  exit 1
fi
wait "$hook_process_pid" || line_rc=$?
hook_process_pid=
hook_process_path=
[[ "$line_rc" -eq 0 ]] || { echo "supervision hook chat-line fixture returned $line_rc" >&2; exit 1; }
grep -Fq 'HEALTH ' "$tmp/line.out" \
  || { echo "supervision hook chat-line fixture emitted no health verdict" >&2; exit 1; }
grep -Fq 'NARRATOR DIGEST since last check-in' "$tmp/line.out" \
  || { echo "supervision hook chat-line fixture omitted the pending narrator digest" >&2; exit 1; }
grep -Fq 'A landing moved the repository storyline to commit abc123' "$tmp/line.out" \
  || { echo "supervision hook chat-line fixture omitted the digest event" >&2; exit 1; }
if grep -Fq 'hook-freshness=dead' "$tmp/line.out"; then
  echo "supervision hook chat-line fixture judged its own current attempt dead" >&2
  exit 1
fi
grep -Fq '"result": "OK"' "$hook_evidence" \
  || { echo "supervision hook chat-line fixture recorded no successful completion" >&2; exit 1; }
grep -Fq '"outcome": "EMITTED"' "$hook_evidence" \
  || { echo "supervision hook chat-line fixture did not record EMITTED" >&2; exit 1; }

printf '{"session_id":"line-fixture-two","cwd":"%s","hook_event_name":"Stop"}\n' "$line_root" >"$tmp/line-payload-two.json"
bash "$line_root/scripts/agents/supervision-hook.sh" claude stop <"$tmp/line-payload-two.json" \
  >"$tmp/line-two.out" 2>"$tmp/line-two.err" \
  || { echo "supervision hook second digest check-in failed" >&2; exit 1; }
if grep -Fq 'A landing moved the repository storyline to commit abc123' "$tmp/line-two.out"; then
  echo "supervision hook repeated a digest after its check-in cursor advanced" >&2
  exit 1
fi

kill_engine=$tmp/kill-engine
cat >"$kill_engine" <<'SH'
#!/usr/bin/env bash
if [[ ${1:-} == steward && ${2:-} == hook-attempt ]]; then
  hook_pid=
  previous=
  for argument in "$@"; do
    if [[ $previous == --pid ]]; then
      hook_pid=$argument
      break
    fi
    previous=$argument
  done
  result=$("${METASYSTEM_KILL_REAL_ENGINE:?}" "$@") || exit $?
  printf '%s\n' "$result"
  kill -KILL "$hook_pid"
  exit 137
fi
exec "${METASYSTEM_KILL_REAL_ENGINE:?}" "$@"
SH
chmod +x "$kill_engine"
printf '{"session_id":"killed-fixture","cwd":"%s","hook_event_name":"Stop"}\n' "$line_root" >"$tmp/killed-payload.json"
set +e
METASYSTEM_BIN="$kill_engine" METASYSTEM_KILL_REAL_ENGINE="$ms" \
  bash "$line_root/scripts/agents/supervision-hook.sh" claude stop <"$tmp/killed-payload.json" \
  >"$tmp/killed.out" 2>"$tmp/killed.err"
killed_rc=$?
set -e
[[ "$killed_rc" -ne 0 ]] \
  || { echo "supervision hook kill fixture did not stop between attempt and emission" >&2; exit 1; }
grep -Fq '"outcome": "ATTEMPTING"' "$hook_evidence" \
  || { echo "supervision hook kill fixture left no unresolved attempt" >&2; exit 1; }

printf '{"session_id":"after-kill-fixture","cwd":"%s","hook_event_name":"Stop"}\n' "$line_root" >"$tmp/after-kill-payload.json"
bash "$line_root/scripts/agents/supervision-hook.sh" claude stop <"$tmp/after-kill-payload.json" \
  >"$tmp/after-kill.out" 2>"$tmp/after-kill.err" \
  || { echo "supervision hook could not run after an interrupted attempt" >&2; exit 1; }
grep -Fq 'INTERRUPTED_BY_NEXT_TURN' "$hook_evidence" \
  || { echo "the next hook turn erased the killed attempt instead of retaining failed history" >&2; exit 1; }
grep -Fq 'hook-freshness=dead' "$tmp/after-kill.out" \
  || { echo "the next hook line did not judge the prior interrupted turn" >&2; exit 1; }
grep -Fq '"outcome": "EMITTED"' "$hook_evidence" \
  || { echo "the post-kill hook turn did not complete its own emission" >&2; exit 1; }

echo "supervision hook runtime membership, narrator digest delivery, current-turn freshness, killed-attempt history, emission evidence, and loud missing-engine fixtures passed"
