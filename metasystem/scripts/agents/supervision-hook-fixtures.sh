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
osascript_calls=$tmp/osascript.log
mkdir -p "$tmp/notify-shim"
cat >"$tmp/notify-shim/osascript" <<'OSASCRIPT_SHIM'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"${METASYSTEM_FIXTURE_OSASCRIPT_CALLS:?}"
exit 97
OSASCRIPT_SHIM
chmod +x "$tmp/notify-shim/osascript"
export METASYSTEM_FIXTURE_OSASCRIPT_CALLS=$osascript_calls
export PATH="$tmp/notify-shim:$PATH"
hook_process_pid=
hook_process_path=
brain_fake_pid=
brain_fake_path=
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
  if [[ -n "$brain_fake_pid" ]] && kill -0 "$brain_fake_pid" 2>/dev/null; then
    brain_fake_command=$(ps -p "$brain_fake_pid" -o command= 2>/dev/null || true)
    if [[ "$brain_fake_command" == *"$brain_fake_path"* ]]; then
      kill -TERM "$brain_fake_pid" 2>/dev/null || true
      wait "$brain_fake_pid" 2>/dev/null || true
    fi
  fi
  rm -rf "$tmp"
}
trap cleanup EXIT

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

# Brain SessionStart: build one declared adopted-style checkout, let the real
# engine decide the role, and keep the hook a one-object relay.
brain_origin=$tmp/brain-origin.git
brain_repo=$tmp/brain-repo
brain_registry=$tmp/brain-registry
mkdir -p "$brain_registry"
git init -q --bare "$brain_origin"
git init -q -b main "$brain_repo"
git -C "$brain_repo" config user.name fixture
git -C "$brain_repo" config user.email fixture@example.invalid
git -C "$brain_repo" config metasystem.goal.machine brain-hook
git -C "$brain_repo" remote add origin "$brain_origin"
mkdir -p "$brain_repo/plans" "$brain_repo/scripts/agents" "$brain_repo/records/misc"
cp "$root/scripts/agents/pre-commit-guard.sh" "$brain_repo/scripts/agents/"
cp -R "$root/scripts/agents/adapters" "$brain_repo/scripts/agents/"
cp "$root/records/misc/fleet-coordinator-brain-role-packet.md" "$brain_repo/records/misc/"
cat >"$brain_repo/plans/goals.md" <<'BRAIN_LEDGER'
# Goals

## Current goal: brain-hook-goal — Exercise the brain start hook
- Origin: main
- Next step: Ask Wido one question.
BRAIN_LEDGER
brain_ledger=$(cat "$brain_repo/plans/goals.md" && printf x); brain_ledger=${brain_ledger%x}
"$ms" json object ledger="$brain_ledger" sha256="$(shasum -a 256 "$brain_repo/plans/goals.md" | cut -d' ' -f1)" \
  >"$brain_repo/plans/goals-accepted.json"
"$ms" json set --file "$brain_repo/plans/goals-accepted.json" --int schemaVersion=1
printf '%s\n' 'metasystem.runtimes=fake' >"$brain_repo/metasystem.conf"
git -C "$brain_repo" add plans scripts records metasystem.conf
git -C "$brain_repo" commit -qm 'brain hook legacy ledger'
git -C "$brain_repo" push -q origin main
brain_fixture_started=$("$ms" proc started-at --pid "$$")
# Positive brain starts can run beneath a different real host. Present the
# fixture shell's exact identity as Claude and forward every other verb.
brain_identity_engine=$tmp/brain-identity-engine
cat >"$brain_identity_engine" <<'BRAIN_IDENTITY_ENGINE'
#!/usr/bin/env bash
if [[ ${1:-} == proc && ${2:-} == find-ancestor ]]; then
	printf '{"runtime":"claude","pid":%s,"pidStartedAt":%s}\n' "${METASYSTEM_BRAIN_FIXTURE_PID:?}" "${METASYSTEM_BRAIN_FIXTURE_STARTED:?}"
	exit 0
fi
exec "${METASYSTEM_BRAIN_REAL_ENGINE:?}" "$@"
BRAIN_IDENTITY_ENGINE
chmod +x "$brain_identity_engine"
run_brain_hook() { # hook, payload, stdout, stderr, optional real engine
  local brain_hook=$1 brain_payload=$2 brain_stdout=$3 brain_stderr=$4
  local real_engine="${5:-$ms}"
  METASYSTEM_BIN=$brain_identity_engine METASYSTEM_BRAIN_REAL_ENGINE=$real_engine \
    METASYSTEM_BRAIN_FIXTURE_PID=$$ METASYSTEM_BRAIN_FIXTURE_STARTED=$brain_fixture_started \
    METASYSTEM_SUPERVISION_REGISTRY_HOME=$brain_registry \
    bash "$brain_hook" claude start <"$brain_payload" >"$brain_stdout" 2>"$brain_stderr"
}
"$ms" lease announce --root "$brain_repo" --session brain-hook-fixture --pid "$$" \
  --start "$brain_fixture_started" --tag brain-hook-fixture --runtime claude --owner-lineage fixture-lineage >/dev/null
brain_source_digest=$("$ms" goal source-digest --root "$brain_repo")
cat >"$tmp/brain-manifest.md" <<BRAIN_MANIFEST
# Queue amendments

MIGRATION_EPOCH: 2026-09-07T00:00:00Z
REVIEWED_SOURCE_SHA256: $brain_source_digest
BRAIN_MANIFEST
METASYSTEM_OWNER_LINEAGE=fixture-lineage "$ms" goal migrate --root "$brain_repo" \
  --source-digest "$brain_source_digest" --manifest "$tmp/brain-manifest.md" --by Wido >/dev/null
git -C "$brain_repo" fetch -q origin
git -C "$brain_repo" reset -q --hard origin/main
git -C "$brain_repo" update-ref refs/metasystem/goals/accepted origin/main
METASYSTEM_OWNER_LINEAGE=fixture-lineage "$ms" goal release --root "$brain_repo" --id brain-hook-goal >/dev/null
git -C "$brain_repo" fetch -q origin
git -C "$brain_repo" reset -q --hard origin/main
git -C "$brain_repo" update-ref refs/metasystem/goals/accepted origin/main
METASYSTEM_SUPERVISION_REGISTRY_HOME=$brain_registry "$ms" brain declare --root "$brain_repo" \
  --by Wido --fixture-human-authority >/dev/null
cp "$brain_repo/artifacts/agents/brain.json" "$tmp/valid-brain.json"

brain_fake_path=$tmp/brain-fake
mkdir -p "$brain_fake_path"
"$ms" channel fake serve --dir "$brain_fake_path" >"$tmp/brain-fake.log" 2>&1 &
brain_fake_pid=$!
brain_fake_deadline=$((SECONDS + 30))
while [[ ! -s "$brain_fake_path/base-url" ]]; do
  (( SECONDS < brain_fake_deadline )) || { echo "brain hook fake channel did not start" >&2; exit 1; }
  sleep 0.05
done
cat >>"$brain_repo/metasystem.conf.local" <<BRAIN_CHANNEL
channel.destination.fleet.adapter=fake
channel.destination.fleet.fake.dir=$brain_fake_path
channel.poll-timeout-sec=15
BRAIN_CHANNEL
brain_qid=$(METASYSTEM_OWNER_LINEAGE=fixture-lineage "$ms" channel ask --root "$brain_repo" \
  --goal brain-hook-goal --kind other --fact 'The brain needs Wido.' \
  --option 'answer: continue' --recommend answer --wants 'Wido decides the next step')
brain_fake_command=$(ps -p "$brain_fake_pid" -o command= 2>/dev/null || true)
[[ "$brain_fake_command" == *"$brain_fake_path"* ]] \
  || { echo "brain hook fixture could not prove fake channel ownership" >&2; exit 1; }
kill -TERM "$brain_fake_pid"
wait "$brain_fake_pid" 2>/dev/null || true
brain_fake_pid=
printf '%s\n' '2026-09-07T00:00:00Z HIGHLIGHT — brain startup digest line (source: fixture startup)' \
  >"$brain_repo/records/narrator-digest.log"
printf '{"session_id":"brain-start","cwd":"%s","source":"startup"}\n' "$brain_repo" >"$tmp/brain-start.json"
run_brain_hook "$hook" "$tmp/brain-start.json" "$tmp/brain-start.out" "$tmp/brain-start.err"
[[ $(wc -l <"$tmp/brain-start.out" | tr -d ' ') -eq 1 ]] \
  || { echo "declared brain start emitted more than one object" >&2; cat "$tmp/brain-start.out" >&2; exit 1; }
brain_context_sentinel=$("$ms" json get --file "$tmp/brain-start.out" --field hookSpecificOutput.additionalContext; printf x)
brain_context=${brain_context_sentinel%x}; brain_context=${brain_context%$'\n'}
[[ "$("$ms" json get --file "$tmp/brain-start.out" --field hookSpecificOutput.hookEventName)" == SessionStart ]] \
  || { echo "declared brain start omitted the registry event name" >&2; exit 1; }
(( $(printf '%s' "$brain_context" | wc -c | tr -d ' ') <= 10000 )) || { echo "brain start context exceeded registry bound" >&2; exit 1; }
grep -Fq '# The brain seat: role packet and standing instruction' <<<"$brain_context" \
  && grep -Fq 'brain startup digest line' <<<"$brain_context" \
  && grep -Fq "$brain_qid" <<<"$brain_context" \
  || { echo "declared brain start omitted packet, digest, or ask" >&2; exit 1; }
[[ -f "$brain_repo/artifacts/agents/steward/narrator-digest-brain-cursor.json" ]] \
  || { echo "brain start did not advance its digest cursor" >&2; exit 1; }
[[ ! -e "$brain_repo/artifacts/agents/steward/narrator-digest-cursor.json" ]] \
  || { echo "brain start changed the human digest cursor" >&2; exit 1; }
[[ -f "$brain_repo/artifacts/agents/brain-status.json" ]] \
  || { echo "brain start did not write brain-status.json" >&2; exit 1; }

# Template layout: the repository is the git toplevel while its metasystem
# directory owns the engine, state, role packet, and narrator digest.
brain_template_repo=$tmp/brain-template-repo
brain_template_root=$brain_template_repo/metasystem
mkdir -p "$brain_template_repo/development" "$brain_template_root/bin" \
  "$brain_template_root/scripts/agents" "$brain_template_root/records/misc" \
  "$brain_template_root/artifacts/agents"
printf '%s\n' '# Template fixture marker' >"$brain_template_repo/development/metasystem-design.md"
cp "$ms" "$brain_template_root/bin/metasystem"
cp "$hook" "$brain_template_root/scripts/agents/supervision-hook.sh"
cp -R "$root/scripts/agents/adapters" "$brain_template_root/scripts/agents/"
cp "$brain_repo/metasystem.conf" "$brain_template_root/metasystem.conf"
cp -R "$brain_repo/plans" "$brain_template_repo/"
cp "$root/records/misc/fleet-coordinator-brain-role-packet.md" "$brain_template_root/records/misc/"
cp "$tmp/valid-brain.json" "$brain_template_root/artifacts/agents/brain.json"
printf '%s\n' '2026-09-07T00:00:30Z HIGHLIGHT — template-layout brain digest line (source: fixture template)' \
  >"$brain_template_root/records/narrator-digest.log"
git -C "$brain_template_repo" init -q -b main
git -C "$brain_template_repo" config user.name fixture
git -C "$brain_template_repo" config user.email fixture@example.invalid
git -C "$brain_template_repo" config metasystem.goal.machine brain-hook
git -C "$brain_template_repo" add development metasystem plans
git -C "$brain_template_repo" commit -qm 'template-layout brain hook fixture'
git -C "$brain_template_repo" update-ref refs/metasystem/goals/accepted HEAD
printf '{"session_id":"brain-template","cwd":"%s","source":"startup"}\n' "$brain_template_repo" \
  >"$tmp/brain-template.json"
run_brain_hook "$brain_template_root/scripts/agents/supervision-hook.sh" \
  "$tmp/brain-template.json" "$tmp/brain-template.out" "$tmp/brain-template.err" \
  "$brain_template_root/bin/metasystem"
[[ $(wc -l <"$tmp/brain-template.out" | tr -d ' ') -eq 1 ]] \
  || { echo "template-layout brain start emitted more than one object" >&2; cat "$tmp/brain-template.out" >&2; exit 1; }
brain_template_context=$("$ms" json get --file "$tmp/brain-template.out" --field hookSpecificOutput.additionalContext)
[[ "$("$ms" json get --file "$tmp/brain-template.out" --field hookSpecificOutput.hookEventName)" == SessionStart ]] \
  && grep -Fq '# The brain seat: role packet and standing instruction' <<<"$brain_template_context" \
  && grep -Fq 'template-layout brain digest line' <<<"$brain_template_context" \
  || { echo "template-layout start omitted the event name, packet, or installation digest" >&2; cat "$tmp/brain-template.out" >&2; exit 1; }

printf '%s\n' '2026-09-07T00:01:00Z HIGHLIGHT — brain compact digest line (source: fixture compact)' \
  >>"$brain_repo/records/narrator-digest.log"
printf '{"session_id":"brain-compact","cwd":"%s","source":"compact"}\n' "$brain_repo" >"$tmp/brain-compact.json"
run_brain_hook "$hook" "$tmp/brain-compact.json" "$tmp/brain-compact.out" "$tmp/brain-compact.err"
brain_compact_context=$("$ms" json get --file "$tmp/brain-compact.out" --field hookSpecificOutput.additionalContext)
grep -Fq '# The brain seat: role packet and standing instruction' <<<"$brain_compact_context" \
  && grep -Fq 'brain compact digest line' <<<"$brain_compact_context" \
  || { echo "compact start omitted packet or new digest" >&2; exit 1; }
grep -Fq 'brain startup digest line' <<<"$brain_compact_context" \
  && { echo "compact start replayed the already advanced digest" >&2; exit 1; }
brain_registry_sources=$("$ms" runtime start-context claude)
brain_registry_sources=${brain_registry_sources##*sources=}
brain_registry_matcher=${brain_registry_sources//,/|}
grep -Fq "\"matcher\": \"$brain_registry_matcher\"" "$root/scripts/enforcement/claude-code-hooks.json" \
  || { echo "shipped compact matcher differs from registry sources" >&2; exit 1; }

METASYSTEM_SUPERVISION_REGISTRY_HOME=$brain_registry "$ms" brain withdraw --root "$brain_repo" \
  --by Wido --fixture-human-authority >/dev/null
printf '{"session_id":"brain-undeclared","cwd":"%s","source":"startup"}\n' "$brain_repo" >"$tmp/brain-undeclared.json"
run_brain_hook "$hook" "$tmp/brain-undeclared.json" "$tmp/brain-undeclared.out" "$tmp/brain-undeclared.err"
if "$ms" json get --file "$tmp/brain-undeclared.out" --field hookSpecificOutput.additionalContext >/dev/null 2>&1; then
  echo "undeclared start carried brain context" >&2
  exit 1
fi
grep -Eq 'brain startup digest line|brain compact digest line|The brain seat: role packet' "$tmp/brain-undeclared.out" \
  && { echo "undeclared start leaked brain input text" >&2; exit 1; }

printf '%s\n' '{broken' >"$brain_repo/artifacts/agents/brain.json"
run_brain_hook "$hook" "$tmp/brain-start.json" "$tmp/brain-corrupt.out" "$tmp/brain-corrupt.err"
brain_corrupt_context=$("$ms" json get --file "$tmp/brain-corrupt.out" --field hookSpecificOutput.additionalContext)
grep -Fq '## The standing instruction' <<<"$brain_corrupt_context" \
  && grep -Fq "this checkout's brain declaration is unreadable" <<<"$brain_corrupt_context" \
  || { echo "corrupt brain start omitted standing instruction or remedy" >&2; exit 1; }
cp "$tmp/valid-brain.json" "$brain_repo/artifacts/agents/brain.json"
"$ms" json set --file "$brain_repo/artifacts/agents/brain.json" --field ledger=ZZZZZZZZZZZZZZZZZZZZZZZZZZ
run_brain_hook "$hook" "$tmp/brain-start.json" "$tmp/brain-wrong-ledger.out" "$tmp/brain-wrong-ledger.err"
brain_wrong_context=$("$ms" json get --file "$tmp/brain-wrong-ledger.out" --field hookSpecificOutput.additionalContext)
grep -Fq 'not this checkout' <<<"$brain_wrong_context" \
  || { echo "wrong-ledger corrupt start omitted its reason" >&2; exit 1; }

cp "$tmp/valid-brain.json" "$brain_repo/artifacts/agents/brain.json"
printf '{}' | METASYSTEM_BIN="$tmp/does-not-exist" bash "$hook" claude start >"$tmp/brain-no-engine-declared.out"
rm "$brain_repo/artifacts/agents/brain.json"
printf '{}' | METASYSTEM_BIN="$tmp/does-not-exist" bash "$hook" claude start >"$tmp/brain-no-engine-undeclared.out"
for no_engine in "$tmp/brain-no-engine-declared.out" "$tmp/brain-no-engine-undeclared.out"; do
  [[ $(wc -l <"$no_engine" | tr -d ' ') -eq 1 ]] \
    && grep -Fq 'Metasystem engine missing' "$no_engine" \
    && grep -Fq 'go-build.sh' "$no_engine" \
    || { echo "missing-engine start omitted its one rebuild notice" >&2; exit 1; }
  grep -Fq 'hookSpecificOutput' "$no_engine" \
    && { echo "missing-engine start injected context" >&2; exit 1; }
done

cp "$tmp/valid-brain.json" "$brain_repo/artifacts/agents/brain.json"
printf '%s\n' '2026-09-07T00:02:00Z LOWLIGHT — unadvanced failure digest (source: fixture failure)' \
  >>"$brain_repo/records/narrator-digest.log"
brain_cursor_before=$(shasum -a 256 "$brain_repo/artifacts/agents/steward/narrator-digest-brain-cursor.json" | cut -d' ' -f1)
brain_failure_engine=$tmp/brain-failure-engine
cat >"$brain_failure_engine" <<'BRAIN_FAILURE_ENGINE'
#!/usr/bin/env bash
if [[ ${1:-} == brain && ${2:-} == boot ]]; then
  case ${METASYSTEM_BRAIN_FAILURE_MODE:?} in
    exit) echo 'fixture boot exit' >&2; exit 1 ;;
    sleep) sleep 30; exit 0 ;;
    invalid) echo 'not json'; exit 0 ;;
  esac
	fi
if [[ ${1:-} == proc && ${2:-} == find-ancestor ]]; then
	printf '{"runtime":"claude","pid":%s,"pidStartedAt":%s}\n' "${METASYSTEM_BRAIN_FIXTURE_PID:?}" "${METASYSTEM_BRAIN_FIXTURE_STARTED:?}"
	exit 0
fi
if [[ ${1:-} == up ]]; then
	printf '%s\n' "${METASYSTEM_BRAIN_FAILURE_MODE:?}" >>"${METASYSTEM_BRAIN_ARMING_LOG:?}"
	printf '%s\n' 'UP aggregate=SUCCESS'
	exit 0
fi
exec "${METASYSTEM_BRAIN_REAL_ENGINE:?}" "$@"
BRAIN_FAILURE_ENGINE
chmod +x "$brain_failure_engine"
brain_arming_log=$tmp/brain-failure-arming.log
for failure_mode in exit sleep invalid; do
  METASYSTEM_BIN=$brain_failure_engine METASYSTEM_BRAIN_REAL_ENGINE=$ms \
    METASYSTEM_BRAIN_FAILURE_MODE=$failure_mode METASYSTEM_SUPERVISION_REGISTRY_HOME=$brain_registry \
    METASYSTEM_BRAIN_FIXTURE_PID=$$ METASYSTEM_BRAIN_FIXTURE_STARTED=$brain_fixture_started \
    METASYSTEM_BRAIN_ARMING_LOG=$brain_arming_log \
    bash "$hook" claude start <"$tmp/brain-start.json" \
      >"$tmp/brain-failure-$failure_mode.out" 2>"$tmp/brain-failure-$failure_mode.err"
  failure_out=$tmp/brain-failure-$failure_mode.out
  [[ $(wc -l <"$failure_out" | tr -d ' ') -eq 1 ]] \
    && grep -Fq 'Metasystem brain boot failed' "$failure_out" \
    && grep -Fq 'metasystem brain boot --root' "$failure_out" \
    || { echo "brain boot $failure_mode failure omitted its one by-hand notice" >&2; cat "$failure_out" >&2; exit 1; }
  grep -Fq 'hookSpecificOutput' "$failure_out" \
    && { echo "brain boot $failure_mode failure injected context" >&2; exit 1; }
  [[ "$brain_cursor_before" == "$(shasum -a 256 "$brain_repo/artifacts/agents/steward/narrator-digest-brain-cursor.json" | cut -d' ' -f1)" ]] \
    || { echo "brain boot $failure_mode failure advanced the cursor" >&2; exit 1; }
  grep -Fxq "$failure_mode" "$brain_arming_log" \
    || { echo "brain boot $failure_mode failure skipped the supervision arming leg" >&2; exit 1; }
done

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

# The open-work marker belongs to the verdict inputs rather than one runtime
# session. Exercise it through the same commands the ordinary Stop worker and
# deadline parent call so either path preserves the once-only refusal.
open_work_root=$tmp/open-work-root
mkdir -p "$open_work_root/plans" "$open_work_root/artifacts/agents/jobs"
printf '%s\n' 'metasystem.runtimes=none' >"$open_work_root/metasystem.conf"
printf '# Goals\n\n## Goal-free: declared 2026-09-07T00:00:00Z by human over fixture\n' \
  >"$open_work_root/plans/goals.md"
printf '%s\n' '- Next step: Finish the fixture work' '- Waiting on the human: none' \
  '- In flight right now: none' >"$open_work_root/plans/open.md"
git -C "$open_work_root" init -q -b main
git -C "$open_work_root" config user.name fixture
git -C "$open_work_root" config user.email fixture@example.invalid
git -C "$open_work_root" add metasystem.conf plans
git -C "$open_work_root" commit -qm fixture

first_open_verdict=$("$ms" report turn-verdict --root "$open_work_root" --session open-first)
[[ $("$ms" json get --value "$first_open_verdict" --field shouldBlock) == true ]] \
  || { echo "the first open-work line did not refuse the turn" >&2; echo "$first_open_verdict" >&2; exit 1; }
second_open_verdict=$("$ms" report turn-verdict --root "$open_work_root" --session open-second)
[[ $("$ms" json get --value "$second_open_verdict" --field shouldBlock) == false ]] \
  && [[ $second_open_verdict == *'OPEN WORK (1)'* ]] \
  || { echo "the durable open-work line blocked a second session or disappeared" >&2; echo "$second_open_verdict" >&2; exit 1; }

assert_open_work_seen_reset() { # name, record bytes
  local name=$1 body=$2 verdict display
  printf '%s\n' "$body" >"$open_work_root/artifacts/agents/supervision/open-work-seen.json"
  verdict=$("$ms" report turn-verdict --root "$open_work_root" --session "open-bad-$name")
  display=$("$ms" json get --value "$verdict" --field display)
  [[ $("$ms" json get --value "$verdict" --field shouldBlock) == true ]] \
    && [[ $display == *'OPEN-WORK-SEEN-RESET '* ]] \
    && [[ $display == *'open-work-seen.json'* ]] \
    || { echo "the $name open-work seen-state did not reset visibly to a first refusal" >&2; echo "$verdict" >&2; exit 1; }
}
assert_open_work_seen_reset malformed '{not json'
assert_open_work_seen_reset foreign-schema '{"schemaVersion":2,"plans":{}}'
assert_open_work_seen_reset null-plans '{"schemaVersion":1,"plans":null}'
assert_open_work_seen_reset digest-mismatch \
  '{"schemaVersion":1,"plans":{"plans/open.md":{"wrong":{"line":"OPEN-WORK plans/open.md: old","firstAt":"2026-09-07T08:00:00Z"}}}}'

"$ms" report stop-block --open-work-root "$open_work_root" 'deadline fixture' >/dev/null
deadline_open_verdict=$("$ms" report turn-verdict --root "$open_work_root" --session open-after-deadline)
[[ $("$ms" json get --value "$deadline_open_verdict" --field shouldBlock) == false ]] \
  || { echo "the deadline-parent path lost the open-work marker" >&2; echo "$deadline_open_verdict" >&2; exit 1; }

printf '%s\n' '- Next step: Work first observed during an expiry' '- Waiting on the human: none' \
  '- In flight right now: none' >"$open_work_root/plans/open.md"
"$ms" report stop-block --open-work-root "$open_work_root" 'deadline fixture' >/dev/null
deadline_new_open_verdict=$("$ms" report turn-verdict --root "$open_work_root" --session open-new-after-deadline)
[[ $("$ms" json get --value "$deadline_new_open_verdict" --field shouldBlock) == true ]] \
  || { echo "the deadline-parent path spent a newly observed open-work refusal" >&2; echo "$deadline_new_open_verdict" >&2; exit 1; }

printf '%s\n' '{"jobId":"fixture-chain","status":"completed","chainClosed":false}' \
  >"$open_work_root/artifacts/agents/jobs/fixture-chain.json"
printf '%s\n' '{"jobId":"fixture-chain-r2","status":"pending-setup"}' \
  >"$open_work_root/artifacts/agents/jobs/fixture-chain-r2.json"
printf '%s\n' '- Next step: Changed work while the chain is open' '- Waiting on the human: none' \
  '- In flight right now: fixture-chain' >"$open_work_root/plans/open.md"
chain_open_verdict=$("$ms" report turn-verdict --root "$open_work_root" --session open-chain)
[[ $("$ms" json get --value "$chain_open_verdict" --field shouldBlock) == false ]] \
  && [[ $chain_open_verdict == *'STILL WORKING:'* ]] \
  || { echo "an open chain did not suppress the turn verdict's open-work refusal" >&2; echo "$chain_open_verdict" >&2; exit 1; }
rm -f "$open_work_root/artifacts/agents/jobs/fixture-chain.json" \
  "$open_work_root/artifacts/agents/jobs/fixture-chain-r2.json"

printf '%s\n' '{"jobId":"fixture-running","status":"running"}' \
  >"$open_work_root/artifacts/agents/jobs/fixture-running.json"
printf '%s\n' '- Next step: Changed work while the job runs' '- Waiting on the human: none' \
  '- In flight right now: fixture-running' >"$open_work_root/plans/open.md"
running_open_verdict=$("$ms" report turn-verdict --root "$open_work_root" --session open-running)
[[ $("$ms" json get --value "$running_open_verdict" --field shouldBlock) == false ]] \
  && [[ $running_open_verdict == *'STILL WORKING:'* ]] \
  || { echo "a running checkout job did not suppress the open-work refusal" >&2; echo "$running_open_verdict" >&2; exit 1; }

rm -f "$open_work_root/artifacts/agents/jobs/fixture-running.json"
printf '%s\n' '- Next step: <one line, required>' '- Waiting on the human: none' \
  '- In flight right now: none' >"$open_work_root/plans/open.md"
template_open_verdict=$("$ms" report turn-verdict --root "$open_work_root" --session open-template)
template_open_display=$("$ms" json get --value "$template_open_verdict" --field display)
[[ $template_open_display == *'TEMPLATE-UNFILLED plans/open.md: <one line, required>'* ]] \
  && [[ $template_open_display != *'OPEN-WORK plans/open.md'* ]] \
  || { echo "the plan placeholder was not classified as template-unfilled" >&2; echo "$template_open_verdict" >&2; exit 1; }

mkdir -p "$line_root/artifacts/agents/runs"
printf '%s\n' '- Next step: Complete the bounded refusal fixture' '- Waiting on the human: none' \
  '- In flight right now: none' >"$line_root/plans/bounded.md"
for run_number in $(seq 1 200); do
	run_offset=$((run_number - 1))
	start_minute=$((run_offset / 60))
	start_second=$((run_offset % 60))
	end_offset=$((run_offset + 300))
	end_minute=$((end_offset / 60))
	end_second=$((end_offset % 60))
	printf '{"schemaVersion":1,"runId":"bounded-run-%03d","kind":"suite","display":"bounded refusal fixture","custody":"wrapped","generation":1,"pid":null,"pidStartedAt":null,"pgid":null,"launchNonce":"efefefefefefefefefefefefefefefef","log":"/tmp/bounded-refusal.log","startedAt":"2026-08-01T10:%02d:%02dZ","sessionId":"bounded-fixture","goalId":"","staleAfterMin":30,"windDownMin":10,"endedAt":"2026-08-01T10:%02d:%02dZ","terminalSeq":%d,"evidence":{"mode":"exit-sidecar"},"expect":{"green":"","red":"inspect the full verdict","hung":"","unknown":""},"status":"red","acked":false}\n' \
		"$run_number" "$start_minute" "$start_second" "$end_minute" "$end_second" "$run_number" >"$line_root/artifacts/agents/runs/bounded-run-$(printf '%03d' "$run_number").json"
done

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
line_reason=$("$ms" json get --value "$(tail -1 "$tmp/line.out")" --field reason)
line_reason_runes=$(printf '%s' "$line_reason" | wc -m | tr -d ' ')
(( line_reason_runes <= 4000 )) \
  || { echo "supervision hook reason has $line_reason_runes characters, above the 4000-rune bound" >&2; exit 1; }
[[ $line_reason == *"stop-verdicts/line-fixture.txt"* ]] \
  && [[ -f "$line_root/artifacts/agents/supervision/stop-verdicts/line-fixture.txt" ]] \
  || { echo "bounded hook reason did not name its full stop-verdicts artifact" >&2; cat "$tmp/line.out" >&2; exit 1; }
[[ $line_reason == *"200 runs went red, oldest bounded-run-001"* ]] \
  || { echo "bounded hook reason omitted the two-hundred-run red summary" >&2; cat "$tmp/line.out" >&2; exit 1; }
grep -Fq 'bounded-run-200' "$line_root/artifacts/agents/supervision/stop-verdicts/line-fixture.txt" \
  || { echo "full stop-verdicts artifact omitted bounded-run-200" >&2; exit 1; }
if grep -Fq 'hook-freshness=dead' "$tmp/line.out"; then
  echo "supervision hook chat-line fixture judged its own current attempt dead" >&2
  exit 1
fi
grep -Fq '"result": "OK"' "$hook_evidence" \
  || { echo "supervision hook chat-line fixture recorded no successful completion" >&2; exit 1; }
grep -Fq '"outcome": "EMITTED"' "$hook_evidence" \
  || { echo "supervision hook chat-line fixture did not record EMITTED" >&2; exit 1; }
grep -Eq 'stop response decision=[a-z]+ elapsed=[0-9]+s$' \
  "$line_root/artifacts/agents/supervision/hooks.log" \
  || { echo "supervision hook chat-line fixture did not log its elapsed Stop response" >&2; exit 1; }
grep -Eq '"lastStopElapsedSec": [0-9]+' "$hook_evidence" \
  || { echo "supervision hook chat-line fixture did not record its elapsed Stop seconds" >&2; exit 1; }

compat_root=$tmp/compat-root
cp -R "$line_root" "$compat_root"
compat_evidence=$compat_root/artifacts/agents/steward/components/supervision-hook.json
compat_engine=$tmp/compat-engine
compat_trace=$tmp/compat-engine.trace
cat >"$compat_engine" <<'SH'
#!/usr/bin/env bash
if [[ ${1:-} == steward && ${2:-} == hook-complete ]]; then
  for argument in "$@"; do
    if [[ $argument == --elapsed-sec ]]; then
      printf '%s\n' flagged-refused >>"${METASYSTEM_HOOK_COMPAT_TRACE:?}"
      exit 2
    fi
  done
  printf '%s\n' bare-called >>"${METASYSTEM_HOOK_COMPAT_TRACE:?}"
fi
exec "${METASYSTEM_HOOK_COMPAT_REAL_ENGINE:?}" "$@"
SH
chmod +x "$compat_engine"
printf '{"session_id":"line-fixture-compat","cwd":"%s","hook_event_name":"Stop"}\n' "$compat_root" >"$tmp/line-payload-compat.json"
METASYSTEM_BIN="$compat_engine" METASYSTEM_HOOK_COMPAT_REAL_ENGINE="$ms" METASYSTEM_HOOK_COMPAT_TRACE="$compat_trace" \
  bash "$compat_root/scripts/agents/supervision-hook.sh" claude stop <"$tmp/line-payload-compat.json" \
    >"$tmp/line-compat.out" 2>"$tmp/line-compat.err" \
  || { echo "supervision hook old-engine compatibility fixture failed" >&2; exit 1; }
[[ $(grep -Fc flagged-refused "$compat_trace" || true) -eq 1 ]] \
  || { echo "supervision hook old-engine compatibility fixture did not refuse exactly one elapsed flag" >&2; exit 1; }
[[ $(grep -Fc bare-called "$compat_trace" || true) -eq 1 ]] \
  || { echo "supervision hook old-engine compatibility fixture did not retry exactly once without the elapsed flag" >&2; exit 1; }
compat_result=$("$ms" json get --file "$compat_evidence" --field result)
compat_outcome=$("$ms" json get --file "$compat_evidence" --field outcome)
[[ $compat_result == OK && $compat_outcome == EMITTED ]] \
  || { echo "supervision hook old-engine compatibility retry did not complete the current hook attempt" >&2; exit 1; }
if grep -Fq '"lastStopElapsedSec"' "$compat_evidence"; then
  echo "supervision hook old-engine compatibility retry retained an unsupported Stop measurement" >&2
  exit 1
fi

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
if [[ $step == arming-failure && ${1:-} == up ]]; then
  echo "ENROLLMENT_DRIFT: run 'metasystem steward restart' from an agent-free terminal" >&2
  exit 44
fi
if [[ $step == rearm-success && ${1:-} == up ]]; then
  printf '%s\n' 'up outcome=armed authority=writer re-armed="generation=9 previous=8 engine=abc1234 landed=def5678"'
  exit 0
fi
if [[ $step == rearm-failure && ${1:-} == up ]]; then
  printf '%s\n' 'up outcome=failed re-armed="generation=9 previous=8 engine=abc1234 landed=def5678" component=steward-runner'
  exit 44
fi
if [[ ${1:-} == up ]]; then
  exit 0
fi
if [[ $step == health-failure && ${1:-} == health ]]; then
  exit 45
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

# A re-arm notice is keyed from the aggregate line on both hook entry paths.
# Successful session start emits it, Stop carries it beside either verdict,
# and a later failure cannot erase the already-persisted fact.
rearm_root=$tmp/rearm-root
mkdir -p "$rearm_root/bin" "$rearm_root/plans" "$rearm_root/scripts/agents/adapters"
cp "$ms" "$rearm_root/bin/metasystem"
cp "$hook" "$rearm_root/scripts/agents/supervision-hook.sh"
cp "$root/scripts/agents/adapters/fake.sh" "$rearm_root/scripts/agents/adapters/fake.sh"
printf '%s\n' '#!/usr/bin/env bash' 'exit 0' >"$rearm_root/scripts/agents/evidence-gc.sh"
chmod +x "$rearm_root/scripts/agents/evidence-gc.sh" "$rearm_root/scripts/agents/adapters/fake.sh"
printf '%s\n' 'metasystem.runtimes=fake' >"$rearm_root/metasystem.conf"
printf '# Goals\n\n## Goal-free: declared 2026-08-28T00:00:00Z by human over fixture\n' >"$rearm_root/plans/goals.md"
git -C "$rearm_root" init -q -b main
git -C "$rearm_root" config user.name fixture
git -C "$rearm_root" config user.email fixture@example.invalid
git -C "$rearm_root" add metasystem.conf plans/goals.md
git -C "$rearm_root" commit -qm fixture
rearm_agent=$tmp/metasystem-fake-agent
cat >"$rearm_agent" <<'SH'
#!/usr/bin/env bash
cd "${METASYSTEM_REARM_GIT_ROOT:?}"
bash "${METASYSTEM_REARM_HOOK:?}" fake "${METASYSTEM_REARM_EVENT:?}"
SH
chmod +x "$rearm_agent"
run_rearm_hook() { # injected up outcome, event, payload, stdout, stderr
  METASYSTEM_BIN="$failure_engine" \
    METASYSTEM_STOP_FAILURE_REAL_ENGINE="$rearm_root/bin/metasystem" \
    METASYSTEM_STOP_FAILURE_STEP="$1" \
    METASYSTEM_REARM_GIT_ROOT="$rearm_root" \
    METASYSTEM_REARM_HOOK="$rearm_root/scripts/agents/supervision-hook.sh" \
    METASYSTEM_REARM_EVENT="$2" \
    "$rearm_agent" <"$3" >"$4" 2>"$5"
}

printf '{"session_id":"rearm-start","cwd":"%s","hook_event_name":"SessionStart"}\n' "$rearm_root" \
  >"$tmp/rearm-start-payload.json"
run_rearm_hook rearm-success start "$tmp/rearm-start-payload.json" \
    "$tmp/rearm-start.out" "$tmp/rearm-start.err" \
  || { echo "successful re-arm SessionStart fixture failed" >&2; cat "$tmp/rearm-start.err" >&2; exit 1; }
grep -Fq 'Metasystem re-armed the rebuilt engine:' "$tmp/rearm-start.out" \
  || { echo "successful SessionStart hid the re-arm notice" >&2; cat "$tmp/rearm-start.out" >&2; exit 1; }

run_rearm_hook rearm-failure start "$tmp/rearm-start-payload.json" \
    "$tmp/rearm-start-failure.out" "$tmp/rearm-start-failure.err" \
  || { echo "failed re-arm SessionStart fixture failed" >&2; cat "$tmp/rearm-start-failure.err" >&2; exit 1; }
grep -Fq 'Metasystem supervision arming failed:' "$tmp/rearm-start-failure.out" \
  && grep -Fq 'Metasystem re-armed the rebuilt engine:' "$tmp/rearm-start-failure.out" \
  || { echo "failed SessionStart did not surface both the failure and re-arm notice" >&2; cat "$tmp/rearm-start-failure.out" >&2; exit 1; }

printf '{"session_id":"rearm-stop","cwd":"%s","hook_event_name":"Stop"}\n' "$rearm_root" \
  >"$tmp/rearm-stop-payload.json"
run_rearm_hook rearm-success stop "$tmp/rearm-stop-payload.json" \
    "$tmp/rearm-stop.out" "$tmp/rearm-stop.err" \
  || { echo "successful re-arm Stop fixture failed" >&2; cat "$tmp/rearm-stop.err" >&2; exit 1; }
grep -Fq 'Metasystem re-armed the rebuilt engine:' "$tmp/rearm-stop.out" \
  || { echo "Stop hid the successful re-arm notice" >&2; cat "$tmp/rearm-stop.out" >&2; exit 1; }

printf '{"session_id":"rearm-failure","cwd":"%s","hook_event_name":"Stop"}\n' "$rearm_root" \
  >"$tmp/rearm-failure-payload.json"
run_rearm_hook rearm-failure stop "$tmp/rearm-failure-payload.json" \
    "$tmp/rearm-failure.out" "$tmp/rearm-failure.err" \
  || { echo "failed re-arm Stop fixture failed" >&2; cat "$tmp/rearm-failure.err" >&2; exit 1; }
grep -Fq 'supervision arming failed' "$tmp/rearm-failure.out" \
  && grep -Fq 'Metasystem re-armed the rebuilt engine:' "$tmp/rearm-failure.out" \
  || { echo "failed Stop did not surface both the failure and re-arm notice" >&2; cat "$tmp/rearm-failure.out" >&2; exit 1; }
if grep -Fq 'Metasystem re-armed the rebuilt engine:' "$tmp/line.out"; then
  echo "a hook run without the aggregate key invented a re-arm notice" >&2
  exit 1
fi

# Failures outside the seat block once per cause and session, then surface the
# exact cause, remedy, and occurrence count without another decision block.
printf '{"session_id":"external-failure","cwd":"%s","hook_event_name":"Stop"}\n' "$line_root" \
  >"$tmp/external-failure-payload.json"
for occurrence in 1 2; do
  METASYSTEM_BIN="$failure_engine" \
    METASYSTEM_STOP_FAILURE_REAL_ENGINE="$line_root/bin/metasystem" \
    METASYSTEM_STOP_FAILURE_STEP=arming-failure \
    bash "$line_root/scripts/agents/supervision-hook.sh" claude stop \
      <"$tmp/external-failure-payload.json" >"$tmp/arming-$occurrence.out" 2>"$tmp/arming-$occurrence.err" \
    || { echo "supervision hook arming-failure occurrence $occurrence failed" >&2; exit 1; }
done
grep -Fq '"decision":"block"' "$tmp/arming-1.out" \
  || { echo "first arming failure did not block" >&2; cat "$tmp/arming-1.out" >&2; exit 1; }
if grep -Fq '"decision":"block"' "$tmp/arming-2.out"; then
  echo "repeated arming failure blocked the same session again" >&2
  cat "$tmp/arming-2.out" >&2
  exit 1
fi
grep -Fq 'occurrence 2' "$tmp/arming-2.out" \
  && grep -Fq 'supervision arming failed' "$tmp/arming-2.out" \
  && grep -Fq "ENROLLMENT_DRIFT: run 'metasystem steward restart' from an agent-free terminal" "$tmp/arming-2.out" \
  || { echo "repeated arming failure omitted its cause, count, or exact remedy" >&2; cat "$tmp/arming-2.out" >&2; exit 1; }

METASYSTEM_BIN="$failure_engine" \
  METASYSTEM_STOP_FAILURE_REAL_ENGINE="$line_root/bin/metasystem" \
  METASYSTEM_STOP_FAILURE_STEP=health-failure \
  bash "$line_root/scripts/agents/supervision-hook.sh" claude stop \
    <"$tmp/external-failure-payload.json" >"$tmp/different-cause.out" 2>"$tmp/different-cause.err" \
  || { echo "supervision hook different-cause fixture failed" >&2; exit 1; }
grep -Fq '"decision":"block"' "$tmp/different-cause.out" \
  && grep -Fq 'health engine returned no verdict' "$tmp/different-cause.out" \
  || { echo "a different external cause did not block the same session" >&2; cat "$tmp/different-cause.out" >&2; exit 1; }

printf '{"session_id":"external-failure-fresh","cwd":"%s","hook_event_name":"Stop"}\n' "$line_root" \
  >"$tmp/external-failure-fresh-payload.json"
METASYSTEM_BIN="$failure_engine" \
  METASYSTEM_STOP_FAILURE_REAL_ENGINE="$line_root/bin/metasystem" \
  METASYSTEM_STOP_FAILURE_STEP=arming-failure \
  bash "$line_root/scripts/agents/supervision-hook.sh" claude stop \
    <"$tmp/external-failure-fresh-payload.json" >"$tmp/fresh-session.out" 2>"$tmp/fresh-session.err" \
  || { echo "supervision hook fresh-session fixture failed" >&2; exit 1; }
grep -Fq '"decision":"block"' "$tmp/fresh-session.out" \
  || { echo "a new session did not start with a fresh refusal allowance" >&2; cat "$tmp/fresh-session.out" >&2; exit 1; }
arming_record=$line_root/artifacts/agents/supervision/stop-refusals/external-failure.json
[[ $($line_root/bin/metasystem json get --file "$arming_record" --field schemaVersion) == 1 ]] \
  && [[ $($line_root/bin/metasystem json get --file "$arming_record" --field sessionId) == external-failure ]] \
  || { echo "external refusal record omitted its schema or session" >&2; cat "$arming_record" >&2; exit 1; }

printf '{"session_id":"broken-refusal-record","cwd":"%s","hook_event_name":"Stop"}\n' "$line_root" \
  >"$tmp/broken-refusal-payload.json"
broken_refusal_record=$line_root/artifacts/agents/supervision/stop-refusals/broken-refusal-record.json
mkdir -p "$(dirname "$broken_refusal_record")"
printf '%s\n' '{broken' >"$broken_refusal_record"
METASYSTEM_BIN="$failure_engine" \
  METASYSTEM_STOP_FAILURE_REAL_ENGINE="$line_root/bin/metasystem" \
  METASYSTEM_STOP_FAILURE_STEP=arming-failure \
  bash "$line_root/scripts/agents/supervision-hook.sh" claude stop \
    <"$tmp/broken-refusal-payload.json" >"$tmp/broken-refusal.out" 2>"$tmp/broken-refusal.err" \
  || { echo "supervision hook broken-refusal-record fixture failed" >&2; exit 1; }
if grep -Fq '"decision":"block"' "$tmp/broken-refusal.out"; then
  echo "an unreadable refusal record recreated a blocking loop" >&2
  cat "$tmp/broken-refusal.out" >&2
  exit 1
fi
grep -Fq 'stop-refusal record failure' "$tmp/broken-refusal.out" \
  || { echo "an unreadable refusal record was not surfaced" >&2; cat "$tmp/broken-refusal.out" >&2; exit 1; }

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

# Delaying the return from the recorded hook attempt proves that pre-verdict
# work and the verdict share one end-to-end Stop budget. The wrapper finishes
# on its own shortly after the deadline, so the fixture leaves no long-running
# process and the parent can close the exact attempt it stopped.
deadline_engine=$tmp/deadline-engine
cat >"$deadline_engine" <<'SH'
#!/usr/bin/env bash
if [[ ${1:-} == steward && ${2:-} == hook-attempt ]]; then
  "${METASYSTEM_DEADLINE_REAL_ENGINE:?}" "$@" || exit $?
  hook_parent=$PPID
  delay_started=$SECONDS
  while kill -0 "$hook_parent" 2>/dev/null && (( SECONDS - delay_started < 61 )); do
    sleep 0.05
  done
  if kill -0 "$hook_parent" 2>/dev/null; then
    echo "supervision hook deadline fixture delay ceiling reached (elapsed: $((SECONDS - delay_started))s; cap: 61s)" >&2
    exit 70
  fi
  exit 0
fi
exec "${METASYSTEM_DEADLINE_REAL_ENGINE:?}" "$@"
SH
chmod +x "$deadline_engine"
deadline_started=$SECONDS
deadline_rc=0
METASYSTEM_BIN="$deadline_engine" METASYSTEM_DEADLINE_REAL_ENGINE="$ms" \
  bash "$line_root/scripts/agents/supervision-hook.sh" claude stop <"$tmp/line-payload.json" \
    >"$tmp/deadline.out" 2>"$tmp/deadline.err" || deadline_rc=$?
deadline_elapsed=$((SECONDS - deadline_started))
(( deadline_rc == 0 )) \
  || { echo "supervision hook deadline fixture did not exit successfully: $deadline_rc" >&2; exit 1; }
(( deadline_elapsed < 60 )) \
  || { echo "supervision hook deadline fixture exceeded the provider's sixty-second budget: ${deadline_elapsed}s" >&2; exit 1; }
grep -Fq '"decision":"block"' "$tmp/deadline.out" \
  || { echo "supervision hook deadline fixture emitted no blocking response" >&2; cat "$tmp/deadline.out" >&2; exit 1; }
grep -Fq 'deadline expired before a safe turn verdict' "$tmp/deadline.out" \
  || { echo "supervision hook deadline fixture did not name its fail-closed timeout" >&2; cat "$tmp/deadline.out" >&2; exit 1; }
grep -Eq 'stop response outcome=deadline-expired-block elapsed=5[0-9]s$' \
  "$line_root/artifacts/agents/supervision/hooks.log" \
  || { echo "supervision hook deadline fixture did not log its elapsed deadline refusal" >&2; exit 1; }
[[ $($ms json get --file "$hook_evidence" --field outcome) == DEADLINE_EXPIRED ]] \
  || { echo "supervision hook deadline fixture did not complete the expired attempt" >&2; exit 1; }
grep -Eq '"lastStopElapsedSec": 5[0-9]' "$hook_evidence" \
  && grep -Eq '"stopElapsedSec": 5[0-9]' "$hook_evidence" \
  || { echo "supervision hook deadline fixture did not retain the elapsed expiry measurement" >&2; exit 1; }
deadline_second_rc=0
METASYSTEM_BIN="$deadline_engine" METASYSTEM_DEADLINE_REAL_ENGINE="$ms" \
  bash "$line_root/scripts/agents/supervision-hook.sh" claude stop <"$tmp/line-payload.json" \
    >"$tmp/deadline-second.out" 2>"$tmp/deadline-second.err" || deadline_second_rc=$?
(( deadline_second_rc == 0 )) \
  || { echo "second supervision hook deadline fixture returned $deadline_second_rc" >&2; exit 1; }
if grep -Fq '"decision":"block"' "$tmp/deadline-second.out"; then
  echo "repeated deadline overrun blocked the same session again" >&2
  cat "$tmp/deadline-second.out" >&2
  exit 1
fi
grep -Fq 'occurrence 2' "$tmp/deadline-second.out" \
  && grep -Fq 'stop deadline expired' "$tmp/deadline-second.out" \
  && grep -Fq 'A human or steward must restore supervision outside this seat, then retry.' "$tmp/deadline-second.out" \
  || { echo "repeated deadline overrun omitted its cause, count, or remedy" >&2; cat "$tmp/deadline-second.out" >&2; exit 1; }

# A restricted host may prove that the worker still exists without exposing
# its command line. The parent must keep the Stop deadline but must not signal
# or wait for a process whose ownership it cannot verify.
empty_ps_dir=$tmp/empty-ps-shim
empty_ps_deadline_root=$tmp/empty-ps-deadline
mkdir -p "$empty_ps_dir" "$empty_ps_deadline_root"
cat >"$empty_ps_dir/ps" <<'SH'
#!/usr/bin/env bash
exit 0
SH
chmod +x "$empty_ps_dir/ps"
printf '{"session_id":"empty-ps-deadline-fixture","cwd":"%s","hook_event_name":"Stop"}\n' \
  "$line_root" >"$tmp/empty-ps-deadline-payload.json"
empty_ps_log_start=$(wc -l <"$line_root/artifacts/agents/supervision/hooks.log")
empty_ps_started=$SECONDS
empty_ps_rc=0
PATH="$empty_ps_dir:$PATH" TMPDIR="$empty_ps_deadline_root" \
  METASYSTEM_BIN="$deadline_engine" METASYSTEM_DEADLINE_REAL_ENGINE="$ms" \
  bash "$line_root/scripts/agents/supervision-hook.sh" claude stop \
    <"$tmp/empty-ps-deadline-payload.json" \
    >"$tmp/empty-ps-deadline.out" 2>"$tmp/empty-ps-deadline.err" || empty_ps_rc=$?
empty_ps_elapsed=$((SECONDS - empty_ps_started))
(( empty_ps_rc == 0 )) \
  || { echo "empty-ps supervision hook deadline fixture returned $empty_ps_rc" >&2; exit 1; }
(( empty_ps_elapsed < 60 )) \
  || { echo "empty-ps supervision hook deadline fixture exceeded the provider's sixty-second budget: ${empty_ps_elapsed}s" >&2; exit 1; }
grep -Fq '"decision":"block"' "$tmp/empty-ps-deadline.out" \
  || { echo "empty-ps supervision hook deadline fixture emitted no blocking response" >&2; cat "$tmp/empty-ps-deadline.out" >&2; exit 1; }
grep -Fq 'deadline expired before a safe turn verdict' "$tmp/empty-ps-deadline.out" \
  || { echo "empty-ps supervision hook deadline fixture did not name its fail-closed timeout" >&2; cat "$tmp/empty-ps-deadline.out" >&2; exit 1; }
sed -n "$((empty_ps_log_start + 1)),\$p" "$line_root/artifacts/agents/supervision/hooks.log" \
  | grep -Fq 'stop response outcome=deadline-expired-block' \
  || { echo "empty-ps supervision hook deadline fixture did not log its deadline refusal" >&2; exit 1; }
empty_ps_worker=$(sed -n 's/^stop deadline: worker \([0-9][0-9]*\) left running, command line unverifiable$/\1/p' \
  "$tmp/empty-ps-deadline.err" | tail -1)
[[ "$empty_ps_worker" =~ ^[0-9]+$ ]] \
  || { echo "empty-ps supervision hook deadline fixture did not identify its unverifiable live worker" >&2; cat "$tmp/empty-ps-deadline.err" >&2; exit 1; }
empty_ps_deadline_dirs=("$empty_ps_deadline_root"/metasystem-stop-deadline.*)
(( ${#empty_ps_deadline_dirs[@]} == 1 )) && [[ -d "${empty_ps_deadline_dirs[0]}" ]] \
  || { echo "empty-ps supervision hook deadline fixture did not retain exactly one worker directory" >&2; exit 1; }
hook_process_pid=$empty_ps_worker
hook_process_path=$line_root/scripts/agents/supervision-hook.sh
empty_ps_worker_deadline=$((SECONDS + 70))
while kill -0 "$empty_ps_worker" 2>/dev/null; do
  if (( SECONDS >= empty_ps_worker_deadline )); then
    stop_hook_process || true
    echo "empty-ps supervision hook deadline fixture worker remained alive past its seventy-second cleanup ceiling" >&2
    exit 1
  fi
  sleep 0.05
done
hook_process_pid=
hook_process_path=
grep -Fq 'supervision hook: emitted the health line but could not record completion' \
  "${empty_ps_deadline_dirs[0]}/stderr" \
  || { echo "empty-ps supervision hook deadline fixture did not prove the unsignalled worker continued after the parent returned" >&2; cat "${empty_ps_deadline_dirs[0]}/stderr" >&2; exit 1; }

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

nested_outer=$tmp/nested-root
nested_root=$nested_outer/metasystem
mkdir -p "$nested_outer/development" "$nested_root/bin" "$nested_root/scripts/agents/adapters"
printf '%s\n' 'template marker' >"$nested_outer/development/metasystem-design.md"
printf '%s\n' 'metasystem.runtimes=fake' >"$nested_root/metasystem.conf"
cp "$ms" "$nested_root/bin/metasystem"
cp "$hook" "$nested_root/scripts/agents/supervision-hook.sh"
cp "$root/scripts/agents/adapters/fake.sh" "$nested_root/scripts/agents/adapters/fake.sh"
git -C "$nested_outer" init -q -b main
printf '{"session_id":"nested-installation","cwd":"%s","hook_event_name":"SessionStart"}\n' \
  "$nested_outer" >"$tmp/nested-payload.json"

nested_engine=$tmp/nested-engine
cat >"$nested_engine" <<'SH'
#!/usr/bin/env bash
if [[ ${1:-} == up ]]; then
  printf '%s\n' "$*" >>"${METASYSTEM_NESTED_UP_LOG:?}"
  exit 0
fi
exec "${METASYSTEM_NESTED_REAL_ENGINE:?}" "$@"
SH
chmod +x "$nested_engine"

nested_agent=$tmp/metasystem-fake-agent
cat >"$nested_agent" <<'SH'
#!/usr/bin/env bash
cd "${METASYSTEM_NESTED_GIT_ROOT:?}"
bash "${METASYSTEM_NESTED_HOOK:?}" fake start
SH
chmod +x "$nested_agent"
nested_rc=0
METASYSTEM_BIN="$nested_engine" \
  METASYSTEM_NESTED_REAL_ENGINE="$nested_root/bin/metasystem" \
  METASYSTEM_NESTED_UP_LOG="$tmp/nested-up.log" \
  METASYSTEM_NESTED_GIT_ROOT="$nested_outer" \
  METASYSTEM_NESTED_HOOK="$nested_root/scripts/agents/supervision-hook.sh" \
  "$nested_agent" <"$tmp/nested-payload.json" \
    >"$tmp/nested.out" 2>"$tmp/nested.err" || nested_rc=$?
(( nested_rc == 0 )) \
  || { echo "nested installation ancestor fixture returned $nested_rc" >&2; cat "$tmp/nested.err" >&2; exit 1; }
if grep -Fq 'could not identify the immediate' "$tmp/nested.out"; then
  echo "nested installation ancestor fixture could not identify its fake agent" >&2
  cat "$tmp/nested.out" >&2
  exit 1
fi
[[ -s "$tmp/nested-up.log" ]] \
  || { echo "nested installation ancestor fixture did not reach arming" >&2; exit 1; }

template_outer=$tmp/template-root
template_root=$template_outer/metasystem
mkdir -p "$template_outer/development" "$template_root/bin" "$template_root/plans" \
  "$template_root/scripts/agents/adapters" "$template_root/scripts/agents/roles" \
  "$template_root/scripts/agents/schemas" "$template_root/scripts/agents/permissions" \
  "$template_root/artifacts/agents/session-stops" "$template_root/artifacts/agents/steward"
printf '%s\n' 'template marker' >"$template_outer/development/metasystem-design.md"
printf '%s\n' \
  'metasystem.runtimes=claude' \
  'role.steward-continuation.runtime=claude' \
  'role.steward-continuation.model.claude=fixture' >"$template_root/metasystem.conf"
printf '%s\n' '# Goals' '' \
  '## Queued goal: template-backlog — Keep the template seat moving' \
  '- Origin: main' \
  '- Next step: Claim and dispatch the template backlog.' >"$template_root/plans/goals.md"
cp "$ms" "$template_root/bin/metasystem"
cp "$hook" "$template_root/scripts/agents/supervision-hook.sh"
printf '%s\n' '#!/usr/bin/env bash' 'exit 0' >"$template_root/scripts/agents/evidence-gc.sh"
chmod +x "$template_root/scripts/agents/evidence-gc.sh"
cp "$root/scripts/agents/adapters/claude.sh" "$template_root/scripts/agents/adapters/claude.sh"
cp "$root/scripts/agents/roles/steward-continuation.md" "$template_root/scripts/agents/roles/steward-continuation.md"
cp "$root/scripts/agents/roles/steward-continuation.requirements.json" \
  "$template_root/scripts/agents/roles/steward-continuation.requirements.json"
cp "$root/scripts/agents/schemas/steward-continuation.schema.json" \
  "$template_root/scripts/agents/schemas/steward-continuation.schema.json"
cp "$root/scripts/agents/permissions/workspace.json" "$template_root/scripts/agents/permissions/workspace.json"
template_engine_digest=$("$template_root/bin/metasystem" util sha256 --file "$template_root/bin/metasystem")
template_identity_root=$(cd "$template_root" && pwd -P)
template_identity_engine=$(cd "$template_root/bin" && pwd -P)/metasystem
printf '{"repoIdentity":"%s","generation":1,"installPath":"%s","installDigest":"sha256:%s","mintedAt":"1970-01-01T00:00:00Z","enrollment":"fixture"}\n' \
  "$template_identity_root" "$template_identity_engine" "$template_engine_digest" \
  >"$template_root/artifacts/agents/steward/identity.json"
chmod 0600 "$template_root/artifacts/agents/steward/identity.json"
git -C "$template_outer" init -q -b main
git -C "$template_outer" config user.name fixture
git -C "$template_outer" config user.email fixture@example.invalid
git -C "$template_outer" config metasystem.goal.machine template-machine
git -C "$template_outer" add development metasystem/metasystem.conf metasystem/plans/goals.md metasystem/scripts/agents
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
if [[ ${1:-} == report && ${2:-} == turn-verdict && -n ${METASYSTEM_TEMPLATE_REPORT_LOG:-} ]]; then
  printf '%s\n' "$*" >>"$METASYSTEM_TEMPLATE_REPORT_LOG"
fi
if [[ ${1:-} == proc && ${2:-} == find-ancestor ]]; then
  printf '{"runtime":"claude","pid":%s,"pidStartedAt":%s}\n' \
    "${METASYSTEM_TEMPLATE_MAIN_PID:?}" "${METASYSTEM_TEMPLATE_MAIN_STARTED:?}"
  exit 0
fi
exec "${METASYSTEM_TEMPLATE_REAL_ENGINE:?}" "$@"
SH
chmod +x "$template_engine"
printf '{"session_id":"%s","cwd":"%s","hook_event_name":"Stop"}\n' \
  "$template_session" "$template_outer" >"$tmp/template-payload.json"
printf '{"session_id":"%s","cwd":"%s","hook_event_name":"Stop","stop_hook_active":true}\n' \
  "$template_session" "$template_outer" >"$tmp/template-repeat-payload.json"
template_human_rc=0
METASYSTEM_BIN="$template_engine" METASYSTEM_TEMPLATE_REAL_ENGINE="$template_root/bin/metasystem" \
  METASYSTEM_TEMPLATE_REPORT_LOG="$tmp/template-report.log" \
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
[[ -f "$template_root/artifacts/agents/supervision/hooks.log" ]] \
  || { echo "template hook trail did not write its nested state root" >&2; exit 1; }
[[ ! -e "$template_outer/artifacts/agents/supervision/hooks.log" ]] \
  || { echo "template hook trail split state into the containing Git root" >&2; exit 1; }

template_agent_rc=0
METASYSTEM_BIN="$template_engine" METASYSTEM_TEMPLATE_REAL_ENGINE="$template_root/bin/metasystem" \
  METASYSTEM_TEMPLATE_REPORT_LOG="$tmp/template-report.log" \
  METASYSTEM_TEMPLATE_MAIN_PID="$template_pid" METASYSTEM_TEMPLATE_MAIN_STARTED="$template_pid_started" \
  bash "$template_root/scripts/agents/supervision-hook.sh" claude stop <"$tmp/template-payload.json" \
    >"$tmp/template-agent.out" 2>"$tmp/template-agent.err" || template_agent_rc=$?
(( template_agent_rc == 0 )) \
  || { echo "template honest-agent Stop returned $template_agent_rc" >&2; cat "$tmp/template-agent.err" >&2; exit 1; }
grep -Fq '"decision":"block"' "$tmp/template-agent.out" \
  && grep -Fq 'IDLE WITH BACKLOG' "$tmp/template-agent.out" \
  && grep -Fq 'template-backlog' "$tmp/template-agent.out" \
  || { echo "template honest agent was allowed to leave claimable backlog" >&2; cat "$tmp/template-agent.out" >&2; exit 1; }
if grep -Fq 'This refusal does not repeat for the same work' "$tmp/template-agent.out"; then
  echo "the counted idle refusal retained the false open-work promise" >&2
  cat "$tmp/template-agent.out" >&2
  exit 1
fi

METASYSTEM_BIN="$template_engine" METASYSTEM_TEMPLATE_REAL_ENGINE="$template_root/bin/metasystem" \
  METASYSTEM_TEMPLATE_REPORT_LOG="$tmp/template-report.log" \
  METASYSTEM_TEMPLATE_MAIN_PID="$template_pid" METASYSTEM_TEMPLATE_MAIN_STARTED="$template_pid_started" \
  bash "$template_root/scripts/agents/supervision-hook.sh" claude stop <"$tmp/template-repeat-payload.json" \
    >"$tmp/template-agent-repeat.out" 2>"$tmp/template-agent-repeat.err" \
  || { echo "template repeated open-work Stop returned an error" >&2; exit 1; }
grep -Fq '"decision":"block"' "$tmp/template-agent-repeat.out" \
  && grep -Fq 'refusal 2 of 3' "$tmp/template-agent-repeat.out" \
  && grep -Fq 'stop_hook_active=true' "$tmp/template-agent-repeat.out" \
  || { echo "the second seat-owned refusal was not counted" >&2; cat "$tmp/template-agent-repeat.out" >&2; exit 1; }
grep -Fq -- '--stop-hook-active=true' "$tmp/template-report.log" \
  || { echo "the Stop hook did not pass stop_hook_active to turn-verdict" >&2; cat "$tmp/template-report.log" >&2; exit 1; }

METASYSTEM_BIN="$template_engine" METASYSTEM_TEMPLATE_REAL_ENGINE="$template_root/bin/metasystem" \
  METASYSTEM_TEMPLATE_REPORT_LOG="$tmp/template-report.log" \
  METASYSTEM_TEMPLATE_MAIN_PID="$template_pid" METASYSTEM_TEMPLATE_MAIN_STARTED="$template_pid_started" \
  bash "$template_root/scripts/agents/supervision-hook.sh" claude stop <"$tmp/template-repeat-payload.json" \
    >"$tmp/template-agent-third.out" 2>"$tmp/template-agent-third.err" \
  || { echo "template third open-work Stop returned an error" >&2; exit 1; }
if grep -Fq '"decision":"block"' "$tmp/template-agent-third.out"; then
  echo "the third unchanged backlog refusal did not end the turn" >&2
  cat "$tmp/template-agent-third.out" >&2
  exit 1
fi
grep -Fq 'reached the bound of 3' "$tmp/template-agent-third.out" \
  && grep -Fq 'selected goal template-backlog and deferred its claim' "$tmp/template-agent-third.out" \
  && grep -Fq 'prepared steward continuation intent' "$tmp/template-agent-third.out" \
  && grep -Fq 'the turn will end' "$tmp/template-agent-third.out" \
  && grep -Fq 'stop_hook_active=true' "$tmp/template-agent-third.out" \
  || { echo "the third refusal did not explain its bounded escalation" >&2; cat "$tmp/template-agent-third.out" >&2; exit 1; }
template_intent=$(find "$template_root/artifacts/agents/steward/intents" -type f -name '*.json' -print -quit)
[[ -n "$template_intent" ]] \
  && grep -Fq '"goal": "template-backlog"' "$template_intent" \
  && grep -Fq '"reason": "seatIdle"' "$template_intent" \
  && grep -Fq '"claimNeeded": true' "$template_intent" \
  && grep -Fq '"seatClaimEpoch":' "$template_intent" \
  || { echo "the third Stop did not leave the seat-idle continuation intent on disk" >&2; exit 1; }

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
if grep -Fq 'SESSION STOP authorized once' "$tmp/template-replay.out"; then
  echo "template SessionEnd marker authorized a later Stop" >&2
  cat "$tmp/template-replay.out" >&2
  exit 1
fi
grep -Fq 'cannot replay' "$tmp/template-replay.out" \
  && grep -Fq 'reached the bound of 3' "$tmp/template-replay.out" \
  || { echo "template SessionEnd no-replay evidence was lost beside the bounded idle escalation" >&2; cat "$tmp/template-replay.out" >&2; exit 1; }

[[ ! -s "$osascript_calls" ]] \
  || { echo "supervision hook fixture invoked osascript" >&2; cat "$osascript_calls" >&2; exit 1; }
echo "supervision hook launcher, runtime membership, fail-closed pre-verdict, re-arm visibility, external failure block-once records, verdict and partial-output errors, unreadable state, narrator digest delivery, current-turn freshness, killed-attempt history, emission evidence, end-to-end deadline block-once behavior, missing-engine refusal, nested installation ancestry, bounded template backlog escalation, template holder-state, and SessionEnd no-replay fixtures passed"
