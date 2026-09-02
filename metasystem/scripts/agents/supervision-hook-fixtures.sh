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

# Execute the installed Stop launcher itself with an impossible project path.
# Its fallback must remain independent of reaching the repository hook.
stop_launcher=$(python3 - "$root/scripts/enforcement/claude-code-hooks.json" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as source:
    config = json.load(source)
print(config["hooks"]["Stop"][0]["hooks"][1]["command"])
PY
)
launcher_rc=0
CLAUDE_PROJECT_DIR="$tmp/missing-project" bash -c "$stop_launcher" \
  >"$tmp/launcher-failclosed.out" 2>"$tmp/launcher-failclosed.err" || launcher_rc=$?
(( launcher_rc == 0 )) \
  || { echo "Claude Stop launcher fail-closed fixture returned $launcher_rc" >&2; exit 1; }
grep -Fq '"decision":"block"' "$tmp/launcher-failclosed.out" \
  || { echo "Claude Stop launcher failure emitted no blocking decision" >&2; cat "$tmp/launcher-failclosed.out" >&2; exit 1; }
grep -Fq 'launcher failed before a safe verdict' "$tmp/launcher-failclosed.out" \
  || { echo "Claude Stop launcher failure omitted its diagnostic" >&2; cat "$tmp/launcher-failclosed.out" >&2; exit 1; }

missing_engine_evidence_ready() {
  grep -Fq '"decision":"block"' "$tmp/missing.out" \
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
  && { echo "supervision hook missing-engine fixture returned a non-blocking health message" >&2; exit 1; }
grep -Fq '"decision":"block"' "$tmp/missing.out" \
  || { echo "supervision hook missing-engine fixture emitted no blocking decision" >&2; exit 1; }
grep -Fq 'engine missing' "$tmp/missing.out" \
  || { echo "supervision hook missing-engine fixture omitted its remedy" >&2; exit 1; }

line_root=$tmp/line-root
mkdir -p "$line_root/scripts/agents" "$line_root/bin" "$line_root/plans"
cp "$hook" "$line_root/scripts/agents/supervision-hook.sh"
printf '%s\n' '#!/usr/bin/env bash' 'exit 0' >"$line_root/scripts/agents/evidence-gc.sh"
chmod +x "$line_root/scripts/agents/evidence-gc.sh"
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

failure_engine=$tmp/failure-engine
cat >"$failure_engine" <<'SH'
#!/usr/bin/env bash
step=${METASYSTEM_STOP_FAILURE_STEP:-}
if [[ $step == runtime-list && ${1:-} == runtime && ${2:-} == list ]]; then
  echo "fixture runtime registry failure" >&2
  exit 41
fi
if [[ ${1:-} == up ]]; then
  exit 0
fi
if [[ $step == hook-attempt && ${1:-} == steward && ${2:-} == hook-attempt ]]; then
  echo "fixture hook-attempt failure" >&2
  exit 42
fi
if [[ $step == turn-verdict && ${1:-} == report && ${2:-} == turn-verdict ]]; then
  echo "fixture turn-verdict failure" >&2
  exit 43
fi
if [[ $step == malformed-verdict && ${1:-} == report && ${2:-} == turn-verdict ]]; then
  printf '%s\n' '{"shouldBlock":false}'
  exit 0
fi
if [[ $step == partial-output && ${1:-} == json && ${2:-} == object ]]; then
  printf '%s' '{"systemMessage":'
  exit 0
fi
if [[ $step == partial-output && ${1:-} == json && ${2:-} == get && ${3:-} == --value && ${4:-} == '{"systemMessage":' ]]; then
  printf '%s\n' 'fixture accepted a truncated child response'
  exit 0
fi
exec "${METASYSTEM_STOP_FAILURE_REAL_ENGINE:?}" "$@"
SH
chmod +x "$failure_engine"

assert_failure_blocks() { # fixture name, injected step, expected reason fragment
  local fixture_name=$1 failure_step=$2 reason_fragment=$3 failure_rc=0
  METASYSTEM_BIN="$failure_engine" \
    METASYSTEM_STOP_FAILURE_REAL_ENGINE="$line_root/bin/metasystem" \
    METASYSTEM_STOP_FAILURE_STEP="$failure_step" \
    bash "$line_root/scripts/agents/supervision-hook.sh" claude stop <"$tmp/line-payload.json" \
      >"$tmp/failure-$fixture_name.out" 2>"$tmp/failure-$fixture_name.err" || failure_rc=$?
  (( failure_rc == 0 )) \
    || { echo "supervision hook $fixture_name failure fixture returned $failure_rc" >&2; exit 1; }
  grep -Fq '"decision":"block"' "$tmp/failure-$fixture_name.out" \
    || { echo "supervision hook $fixture_name failure fixture emitted no blocking decision" >&2; cat "$tmp/failure-$fixture_name.out" >&2; exit 1; }
  grep -Fq "$reason_fragment" "$tmp/failure-$fixture_name.out" \
    || { echo "supervision hook $fixture_name failure fixture omitted its reason" >&2; cat "$tmp/failure-$fixture_name.out" >&2; exit 1; }
}

# An uncaught early command error is converted by the deadline parent; a
# deliberately captured pre-verdict failure is converted before the verdict;
# and both an erroring and a malformed verdict refuse the Stop.
assert_failure_blocks runtime-list runtime-list 'could not prove that stopping is safe'
assert_failure_blocks hook-attempt hook-attempt 'attempt evidence could not be recorded'
assert_failure_blocks turn-verdict turn-verdict 'turn-verdict unavailable'
assert_failure_blocks malformed-verdict malformed-verdict 'turn verdict was unreadable'
assert_failure_blocks partial-output partial-output 'could not prove that stopping is safe'

# The verdict owns its state-file decoding. A present but unreadable state file
# must return its structured uncertainty block through the real hook.
printf '%s\n' '{malformed' >"$line_root/artifacts/agents/turn-verdict-state.json"
assert_failure_blocks unreadable-state none 'cannot prove that stopping is safe'
rm -f "$line_root/artifacts/agents/turn-verdict-state.json"

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
[[ "$killed_rc" -eq 0 ]] \
  || { echo "supervision hook kill fixture did not convert the worker failure to a provider response" >&2; exit 1; }
grep -Fq '"decision":"block"' "$tmp/killed.out" \
  || { echo "supervision hook kill fixture emitted no blocking decision" >&2; cat "$tmp/killed.out" >&2; exit 1; }
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

# Delaying the first engine operation proves that pre-verdict work and the
# verdict share one end-to-end Stop budget. The wrapper finishes on its own
# shortly after the deadline, so the fixture leaves no long-running process.
deadline_engine=$tmp/deadline-engine
cat >"$deadline_engine" <<'SH'
#!/usr/bin/env bash
if [[ ${1:-} == runtime && ${2:-} == list ]]; then
  sleep 4.5
fi
exec "${METASYSTEM_DEADLINE_REAL_ENGINE:?}" "$@"
SH
chmod +x "$deadline_engine"
deadline_started=$SECONDS
deadline_rc=0
METASYSTEM_BIN="$deadline_engine" METASYSTEM_DEADLINE_REAL_ENGINE="$ms" \
  bash "$hook" claude stop <"$tmp/line-payload.json" \
    >"$tmp/deadline.out" 2>"$tmp/deadline.err" || deadline_rc=$?
deadline_elapsed=$((SECONDS - deadline_started))
(( deadline_rc == 0 )) \
  || { echo "supervision hook deadline fixture did not exit successfully: $deadline_rc" >&2; exit 1; }
(( deadline_elapsed < 5 )) \
  || { echo "supervision hook deadline fixture exceeded the provider's five-second budget: ${deadline_elapsed}s" >&2; exit 1; }
grep -Fq '"decision":"block"' "$tmp/deadline.out" \
  || { echo "supervision hook deadline fixture emitted no blocking response" >&2; cat "$tmp/deadline.out" >&2; exit 1; }
grep -Fq 'deadline expired before a safe turn verdict' "$tmp/deadline.out" \
  || { echo "supervision hook deadline fixture did not name its fail-closed timeout" >&2; cat "$tmp/deadline.out" >&2; exit 1; }

missing_template_outer=$tmp/missing-template-root
missing_template_root=$missing_template_outer/metasystem
mkdir -p "$missing_template_outer/development" "$missing_template_root/scripts/agents"
printf '%s\n' 'template marker' >"$missing_template_outer/development/metasystem-design.md"
cp "$hook" "$missing_template_root/scripts/agents/supervision-hook.sh"
git -C "$missing_template_outer" init -q -b main
printf '{"session_id":"missing-template","cwd":"%s","hook_event_name":"Stop"}\n' \
  "$missing_template_outer" >"$tmp/missing-template-payload.json"
missing_template_rc=0
METASYSTEM_BIN="$failure_engine" \
  METASYSTEM_STOP_FAILURE_REAL_ENGINE="$line_root/bin/metasystem" \
  METASYSTEM_STOP_FAILURE_STEP=none \
  bash "$missing_template_root/scripts/agents/supervision-hook.sh" claude stop \
    <"$tmp/missing-template-payload.json" >"$tmp/missing-template.out" 2>"$tmp/missing-template.err" \
    || missing_template_rc=$?
(( missing_template_rc == 0 )) \
  || { echo "missing template state fixture returned $missing_template_rc" >&2; exit 1; }
grep -Fq '"decision":"block"' "$tmp/missing-template.out" \
  || { echo "missing template state fixture emitted no blocking decision" >&2; cat "$tmp/missing-template.out" >&2; exit 1; }

template_outer=$tmp/template-root
template_root=$template_outer/metasystem
mkdir -p "$template_outer/development" "$template_root/bin" "$template_root/plans" \
  "$template_root/scripts/agents/adapters" "$template_root/artifacts/agents/session-stops"
printf '%s\n' 'template marker' >"$template_outer/development/metasystem-design.md"
printf '%s\n' 'metasystem.runtimes=none' >"$template_root/metasystem.conf"
printf '%s\n' '# Goals' '' \
  '## Queued goal: template-backlog — Keep the template seat moving' \
  '- Origin: main' \
  '- Next step: Claim and dispatch the template backlog.' >"$template_root/plans/goals.md"
cp "$ms" "$template_root/bin/metasystem"
cp "$hook" "$template_root/scripts/agents/supervision-hook.sh"
printf '%s\n' '#!/usr/bin/env bash' 'exit 0' >"$template_root/scripts/agents/evidence-gc.sh"
chmod +x "$template_root/scripts/agents/evidence-gc.sh"
cp "$root/scripts/agents/adapters/claude.sh" "$template_root/scripts/agents/adapters/claude.sh"
git -C "$template_outer" init -q -b main
git -C "$template_outer" config user.name fixture
git -C "$template_outer" config user.email fixture@example.invalid
git -C "$template_outer" add development metasystem/metasystem.conf metasystem/plans/goals.md
git -C "$template_outer" commit -qm fixture

template_session=template-human
template_started=$("$template_root/bin/metasystem" proc started-at --pid $$)
template_announcement=$("$template_root/bin/metasystem" lease announce --root "$template_root" \
  --session "$template_session" --pid $$ --start "$template_started" \
  --tag template-holder --runtime claude)
template_main=$("$template_root/bin/metasystem" json get --file "$template_announcement" --field mainId)
template_epoch=$("$template_root/bin/metasystem" json get \
  --file "$template_root/artifacts/agents/mains/worktree-lease.json" --field claimEpoch)
template_pid=$("$template_root/bin/metasystem" json get --file "$template_announcement" --field pid)
template_pid_started=$("$template_root/bin/metasystem" json get --file "$template_announcement" --field pidStartedAt)
template_ticks=$("$template_root/bin/metasystem" json get --file "$template_announcement" --field pidStartTicks 2>/dev/null || true)
template_boot=$("$template_root/bin/metasystem" json get --file "$template_announcement" --field bootId 2>/dev/null || true)
template_runtime=$("$template_root/bin/metasystem" json get --file "$template_announcement" --field runtime)
template_tag=$("$template_root/bin/metasystem" json get --file "$template_announcement" --field instanceTag)
template_command_hash=$("$template_root/bin/metasystem" json get --file "$template_announcement" --field commandHash)
template_announced=$("$template_root/bin/metasystem" json get --file "$template_announcement" --field announcedAt)
template_pgid=$("$template_root/bin/metasystem" json get --file "$template_announcement" --field pgid)
template_identity_json=
template_human_json=
if [[ -n "$template_ticks" && -n "$template_boot" ]]; then
  template_identity_json=$(printf ',"pidStartTicks":%s,"bootId":"%s"' "$template_ticks" "$template_boot")
  template_human_json=$template_identity_json
fi
template_lifecycle_json=$(printf \
  '{"sessionId":"%s","mainId":"%s","pid":%s,"pidStartedAt":%s%s,"runtime":"%s","instanceTag":"%s","commandHash":"%s","announcedAt":"%s","pgid":%s}' \
  "$template_session" "$template_main" "$template_pid" "$template_pid_started" "$template_identity_json" \
  "$template_runtime" "$template_tag" "$template_command_hash" "$template_announced" "$template_pgid")
template_lifecycle=$(printf '%s' "$template_lifecycle_json" | "$template_root/bin/metasystem" util sha256)
printf \
  '{"schemaVersion":3,"authorizationId":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","sessionId":"%s","holderMainId":"%s","claimEpoch":%s,"by":"Wido","writtenAt":"2000-01-01T00:00:00Z","expiresAt":"2099-01-01T00:00:00Z","human":{"pid":%s,"pidStartedAt":%s%s},"humanAuthorityProof":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","sessionLifecycle":"%s"}\n' \
  "$template_session" "$template_main" "$template_epoch" "$template_pid" "$template_pid_started" "$template_human_json" "$template_lifecycle" \
  >"$template_root/artifacts/agents/session-stops/$template_session.json"

template_engine=$tmp/template-engine
cat >"$template_engine" <<'SH'
#!/usr/bin/env bash
if [[ ${1:-} == up ]]; then
  exit 0
fi
if [[ ${1:-} == proc && ${2:-} == find-ancestor ]]; then
  printf '{"pid":%s,"pidStartedAt":%s}\n' \
    "${METASYSTEM_TEMPLATE_MAIN_PID:?}" "${METASYSTEM_TEMPLATE_MAIN_STARTED:?}"
  exit 0
fi
exec "${METASYSTEM_TEMPLATE_REAL_ENGINE:?}" "$@"
SH
chmod +x "$template_engine"
printf '{"session_id":"%s","cwd":"%s","hook_event_name":"Stop"}\n' \
  "$template_session" "$template_outer" >"$tmp/template-payload.json"
template_human_rc=0
METASYSTEM_BIN="$template_engine" METASYSTEM_TEMPLATE_REAL_ENGINE="$template_root/bin/metasystem" \
  METASYSTEM_TEMPLATE_MAIN_PID="$template_pid" METASYSTEM_TEMPLATE_MAIN_STARTED="$template_pid_started" \
  bash "$template_root/scripts/agents/supervision-hook.sh" claude stop <"$tmp/template-payload.json" \
    >"$tmp/template-human.out" 2>"$tmp/template-human.err" || template_human_rc=$?
(( template_human_rc == 0 )) \
  || { echo "template attended-human Stop returned $template_human_rc" >&2; cat "$tmp/template-human.err" >&2; exit 1; }
if grep -Fq '"decision":"block"' "$tmp/template-human.out"; then
  echo "template attended-human authorization did not end quietly" >&2
  cat "$tmp/template-human.out" >&2
  exit 1
fi
grep -Fq 'SESSION STOP authorized once by Wido' "$tmp/template-human.out" \
  || { echo "template attended-human authorization did not reach the nested state root" >&2; cat "$tmp/template-human.out" >&2; exit 1; }
[[ ! -e "$template_root/artifacts/agents/session-stops/$template_session.json" ]] \
  || { echo "template attended-human authorization was not consumed" >&2; exit 1; }
[[ -f "$template_root/artifacts/agents/turn-verdict-state.json" ]] \
  || { echo "template turn verdict did not write its nested state root" >&2; exit 1; }
[[ ! -e "$template_outer/artifacts/agents/turn-verdict-state.json" ]] \
  || { echo "template turn verdict split state into the containing Git root" >&2; exit 1; }

template_agent_rc=0
METASYSTEM_BIN="$template_engine" METASYSTEM_TEMPLATE_REAL_ENGINE="$template_root/bin/metasystem" \
  METASYSTEM_TEMPLATE_MAIN_PID="$template_pid" METASYSTEM_TEMPLATE_MAIN_STARTED="$template_pid_started" \
  bash "$template_root/scripts/agents/supervision-hook.sh" claude stop <"$tmp/template-payload.json" \
    >"$tmp/template-agent.out" 2>"$tmp/template-agent.err" || template_agent_rc=$?
(( template_agent_rc == 0 )) \
  || { echo "template honest-agent Stop returned $template_agent_rc" >&2; cat "$tmp/template-agent.err" >&2; exit 1; }
grep -Fq '"decision":"block"' "$tmp/template-agent.out" \
  && grep -Fq 'IDLE WITH BACKLOG' "$tmp/template-agent.out" \
  && grep -Fq 'template-backlog' "$tmp/template-agent.out" \
  || { echo "template honest agent was allowed to leave claimable backlog" >&2; cat "$tmp/template-agent.out" >&2; exit 1; }

# SessionEnd spends an unused authorization before announcement retirement.
# The wrapper deliberately leaves the announcement in place, reproducing the
# failed-retirement path without allowing the marker to reach a later Stop.
template_marker_path=$template_root/artifacts/agents/session-stops/$template_session.json
printf \
  '{"schemaVersion":3,"authorizationId":"cccccccccccccccccccccccccccccccc","sessionId":"%s","holderMainId":"%s","claimEpoch":%s,"by":"Wido","writtenAt":"2000-01-01T00:00:00Z","expiresAt":"2099-01-01T00:00:00Z","human":{"pid":%s,"pidStartedAt":%s%s},"humanAuthorityProof":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","sessionLifecycle":"%s"}\n' \
  "$template_session" "$template_main" "$template_epoch" "$template_pid" "$template_pid_started" "$template_human_json" "$template_lifecycle" \
  >"$template_marker_path"
cp "$template_marker_path" "$tmp/template-ended-marker.json"
printf '{"session_id":"%s","cwd":"%s","hook_event_name":"SessionEnd"}\n' \
  "$template_session" "$template_outer" >"$tmp/template-end-payload.json"
METASYSTEM_BIN="$template_engine" METASYSTEM_TEMPLATE_REAL_ENGINE="$template_root/bin/metasystem" \
  METASYSTEM_TEMPLATE_MAIN_PID="$template_pid" METASYSTEM_TEMPLATE_MAIN_STARTED="$template_pid_started" \
  bash "$template_root/scripts/agents/supervision-hook.sh" claude end <"$tmp/template-end-payload.json" \
    >"$tmp/template-end.out" 2>"$tmp/template-end.err" \
  || { echo "template SessionEnd could not retire its unused authorization" >&2; cat "$tmp/template-end.err" >&2; exit 1; }
[[ ! -e "$template_marker_path" ]] \
  || { echo "template SessionEnd left its unused authorization in place" >&2; exit 1; }
[[ -e "$template_announcement" ]] \
  || { echo "template SessionEnd fixture did not preserve the failed-retirement announcement" >&2; exit 1; }

cp "$tmp/template-ended-marker.json" "$template_marker_path"
METASYSTEM_BIN="$template_engine" METASYSTEM_TEMPLATE_REAL_ENGINE="$template_root/bin/metasystem" \
  METASYSTEM_TEMPLATE_MAIN_PID="$template_pid" METASYSTEM_TEMPLATE_MAIN_STARTED="$template_pid_started" \
  bash "$template_root/scripts/agents/supervision-hook.sh" claude stop <"$tmp/template-payload.json" \
    >"$tmp/template-replay.out" 2>"$tmp/template-replay.err" \
  || { echo "template replay check failed to return a Stop decision" >&2; cat "$tmp/template-replay.err" >&2; exit 1; }
grep -Fq '"decision":"block"' "$tmp/template-replay.out" \
  && grep -Fq 'cannot replay' "$tmp/template-replay.out" \
  || { echo "template SessionEnd marker authorized a later Stop" >&2; cat "$tmp/template-replay.out" >&2; exit 1; }

echo "supervision hook launcher, runtime membership, fail-closed pre-verdict, verdict, and partial-output errors, unreadable state, narrator digest delivery, current-turn freshness, killed-attempt history, emission evidence, end-to-end deadline, missing-engine refusal, template holder-state, and SessionEnd no-replay fixtures passed"
