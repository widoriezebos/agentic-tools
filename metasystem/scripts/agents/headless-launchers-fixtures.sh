#!/bin/bash
set -eu

root=$(cd "$(dirname "$0")/../.." && pwd -P)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/headless-launchers.XXXXXX")
child_pids=

cleanup() {
  for pid_file in "$tmp"/run/fake-claude-*.pid; do
    if [ -f "$pid_file" ]; then
      child_pids="$child_pids $(cat "$pid_file")"
    fi
  done
  for pid in $child_pids; do
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
  done
  rm -rf "$tmp"
}

trap cleanup EXIT

fail() {
  echo "$1" >&2
  exit 1
}

wait_file() {
  file=$1
  message=$2
  gate=${3:-}
  start=$SECONDS
  if [ -n "$gate" ]; then
    : >"$gate"
  fi
  while [ ! -s "$file" ]; do
    if [ $((SECONDS - start)) -ge 30 ]; then
      fail "$message"
    fi
    sleep 0.01
  done
}

mkdir -p "$tmp/bin" "$tmp/out" "$tmp/projects/project" "$tmp/run"

cat >"$tmp/bin/claude" <<'EOF'
#!/bin/bash
printf '%s\n' "$$" >"$FAKE_PID_DIR/fake-claude-$$.pid"
if [ -n "${FAKE_WAIT_GATE:-}" ]; then
  start=$SECONDS
  while [ ! -e "$FAKE_WAIT_GATE" ]; do
    if [ $((SECONDS - start)) -ge 30 ]; then
      exit 1
    fi
    sleep 0.01
  done
fi
printf 'window=%s\nargv=%s\n' "${CLAUDE_CODE_AUTO_COMPACT_WINDOW:-missing}" "$*" >"$FAKE_CLAUDE_LOG"
cat >/dev/null
printf '{"session_id":"sid","is_error":false,"num_turns":1,"result":"tail one\\ntail two"}\n'
EOF

cat >"$tmp/bin/codex" <<'EOF'
#!/bin/bash
printf 'cwd=%s\n' "$(pwd -P)" >"$FAKE_CODEX_LOG"
if [ /dev/stdin -ef /dev/null ]; then
  echo 'stdin-is-devnull=yes' >>"$FAKE_CODEX_LOG"
else
  echo 'stdin-is-devnull=no' >>"$FAKE_CODEX_LOG"
fi
i=0
output_last=
task=
while [ $# -gt 0 ]; do
  i=$((i + 1))
  printf 'arg.%s=%s\n' "$i" "$1" >>"$FAKE_CODEX_LOG"
  if [ "$1" = -o ] && [ $# -ge 2 ]; then
    output_last=$2
  fi
  task=$1
  shift
done
printf '%s' "$task" >"$FAKE_CODEX_TASK"
[ -z "$output_last" ] || printf 'fake last message\n' >"$output_last"
mkdir -p "$FAKE_WORKTREE/metasystem/artifacts/reports"
printf 'VERDICT: land\n' >"$FAKE_WORKTREE/metasystem/artifacts/reports/$FAKE_CRITIQUE"
exit "${FAKE_CODEX_EXIT:-0}"
EOF

chmod +x "$tmp/bin/claude" "$tmp/bin/codex"

printf 'design prompt\n' >"$tmp/prompt"
env -u CLAUDE_CODE_AUTO_COMPACT_WINDOW \
  PATH="$tmp/bin:$PATH" \
  FAKE_CLAUDE_LOG="$tmp/claude-1.log" \
  FAKE_PID_DIR="$tmp/run" \
  FAKE_WAIT_GATE="$tmp/claude-1.release" \
  "$root/scripts/agents/headless-design-launch.sh" \
    one "$tmp/prompt" --out "$tmp/out" --root "$tmp/run" \
    --model chosen --resume session >/dev/null
wait_file \
  "$tmp/claude-1.log" \
  'design launcher did not create the default-window log' \
  "$tmp/claude-1.release"
grep -Fq 'window=1000000' "$tmp/claude-1.log" \
  || fail 'design launcher omitted the compact window'
grep -Fq -- '--model chosen' "$tmp/claude-1.log" \
  || fail 'design launcher lost --model'
grep -Fq -- '--resume session' "$tmp/claude-1.log" \
  || fail 'design launcher lost --resume'

cd "$tmp"
CLAUDE_CODE_AUTO_COMPACT_WINDOW=765432 \
  PATH="$tmp/bin:$PATH" \
  FAKE_CLAUDE_LOG="$tmp/claude-2.log" \
  FAKE_PID_DIR="$tmp/run" \
  "$root/scripts/agents/headless-design-launch.sh" \
    two "$tmp/prompt" --out out --root run >/dev/null
cd "$root"
wait_file \
  "$tmp/claude-2.log" \
  'design launcher did not create the override-window log'
grep -Fq 'window=765432' "$tmp/claude-2.log" \
  || fail 'design launcher lost the compact-window override'
if grep -Fq -- '--resume' "$tmp/claude-2.log"; then
  fail 'design launcher passed --resume without an option'
fi

: >"$tmp/empty"
set +e
PATH="$tmp/bin:$PATH" \
  "$root/scripts/agents/headless-design-launch.sh" \
    empty "$tmp/empty" --out "$tmp/out" >/dev/null 2>&1
rc=$?
set -e
[ $rc -eq 2 ] \
  || fail 'design launcher did not return 2 for an empty prompt'
[ -f "$tmp/out/one-design.pid" ] \
  && [ -f "$tmp/out/one-design.json" ] \
  && [ -f "$tmp/out/one-design.err" ] \
  || fail 'design launcher omitted an output file'

printf '99999999\n' >"$tmp/out/w-design.pid"
printf '{"session_id":"waitsid","is_error":false,"num_turns":2,"result":"done"}\n' \
  >"$tmp/out/w-design.json"
printf 'page words\n' >"$tmp/page"
printf '{"type":"user","message":"prompt says compact_boundary"}\n' \
  >"$tmp/projects/project/waitsid.jsonl"
wait_out=$("$root/scripts/agents/headless-design-wait.sh" \
  w "$tmp/page" --out "$tmp/out" --poll 0 --projects "$tmp/projects")
printf '%s\n' "$wait_out" | grep -Fq 'real compactions: 0' \
  || fail 'wait launcher counted prompt text as a compaction'
printf '{"type":"system","subtype":"compact_boundary"}\n' \
  >>"$tmp/projects/project/waitsid.jsonl"
wait_out=$("$root/scripts/agents/headless-design-wait.sh" \
  w "$tmp/page" --out "$tmp/out" --poll 0 --projects "$tmp/projects")
printf '%s\n' "$wait_out" | grep -Fq 'real compactions: 1' \
  || fail 'wait launcher missed a real compaction'

worktree="$tmp/worktree"
mkdir -p "$worktree/metasystem"
worktree=$(cd "$worktree" && pwd -P)
printf 'page\n' >"$tmp/design.md"
printf 'brief\n' >"$tmp/brief.md"
printf 'task line one\ntask line two\n' >"$tmp/task"

cd "$tmp"
crit_out=$(PATH="$tmp/bin:$PATH" \
  FAKE_CODEX_LOG="$tmp/codex-default.log" \
  FAKE_CODEX_TASK="$tmp/codex-default.task" \
  FAKE_CRITIQUE=crit-critique-r2.md \
  FAKE_WORKTREE="$worktree" \
  "$root/scripts/agents/codex-critique-launch.sh" \
    crit goal "$tmp/design.md" "$tmp/brief.md" "$tmp/task" \
    --out out --round 2 --worktree worktree)
cd "$root"

grep -Fxq 'arg.1=exec' "$tmp/codex-default.log" \
  || fail 'critique launcher did not use codex exec'
grep -Fxq 'arg.2=-m' "$tmp/codex-default.log" \
  || fail 'critique launcher did not pass -m'
grep -Fxq 'arg.3=gpt-5.6-sol' "$tmp/codex-default.log" \
  || fail 'critique launcher did not use the default model'
grep -Fxq 'arg.4=-C' "$tmp/codex-default.log" \
  && grep -Fxq "arg.5=$worktree/metasystem" "$tmp/codex-default.log" \
  || fail 'critique launcher did not target the worktree metasystem'
grep -Fxq 'arg.6=-s' "$tmp/codex-default.log" \
  && grep -Fxq 'arg.7=workspace-write' "$tmp/codex-default.log" \
  || fail 'critique launcher did not use the workspace-write sandbox'
crit_out_dir=$(cd "$tmp/out" && pwd -P)
grep -Fxq 'arg.8=-o' "$tmp/codex-default.log" \
  && grep -Fxq "arg.9=$crit_out_dir/crit-crit-r2.last" "$tmp/codex-default.log" \
  && [ -f "$tmp/out/crit-crit-r2.last" ] \
  && [ -f "$tmp/out/crit-crit-r2.log" ] \
  || fail 'critique launcher omitted the Codex output files'
grep -Fxq "cwd=$worktree/metasystem" "$tmp/codex-default.log" \
  || fail 'critique launcher ran codex outside the worktree metasystem'
grep -Fxq 'stdin-is-devnull=yes' "$tmp/codex-default.log" \
  || fail 'critique launcher did not connect codex stdin to /dev/null'
cmp -s "$tmp/task" "$tmp/codex-default.task" \
  || fail 'critique launcher changed the task text'
[ "$(cat "$tmp/out/crit-crit-r2.done")" = completed ] \
  || fail 'critique launcher did not write completed after codex exit 0'
cmp -s \
  "$worktree/metasystem/artifacts/reports/crit-critique-r2.md" \
  "$tmp/out/crit-critique-r2.md" \
  || fail 'critique launcher did not copy the critique intact'
printf '%s\n' "$crit_out" | grep -Fq 'codex exit code 0' \
  || fail 'critique launcher did not report codex exit 0'

cd "$tmp"
crit_fail_out=$(PATH="$tmp/bin:$PATH" \
  FAKE_CODEX_LOG="$tmp/codex-override.log" \
  FAKE_CODEX_TASK="$tmp/codex-override.task" \
  FAKE_CODEX_EXIT=3 \
  FAKE_CRITIQUE=critfail-critique-r3.md \
  FAKE_WORKTREE="$worktree" \
  "$root/scripts/agents/codex-critique-launch.sh" \
    critfail goal "$tmp/design.md" "$tmp/brief.md" "$tmp/task" \
    --out out --round 3 --worktree worktree --model gpt-6-astra)
cd "$root"

grep -Fxq 'arg.3=gpt-6-astra' "$tmp/codex-override.log" \
  || fail 'critique launcher lost the model override'
[ "$(cat "$tmp/out/critfail-crit-r3.done")" = failed ] \
  || fail 'critique launcher did not write failed after codex exit 3'
printf '%s\n' "$crit_fail_out" | grep -Fq 'codex exit code 3' \
  || fail 'critique launcher did not report codex exit 3'

set +e
poll_usage=$("$root/scripts/agents/codex-critique-launch.sh" \
  critpoll goal "$tmp/design.md" "$tmp/brief.md" "$tmp/task" \
  --out "$tmp/out" --worktree "$worktree" --poll 0 2>&1)
poll_rc=$?
set -e
[ "$poll_rc" -eq 2 ] \
  && printf '%s\n' "$poll_usage" | grep -Fq -- '--poll is not supported' \
  || fail 'critique launcher did not reject --poll with current usage'

cat >"$tmp/r1" <<'EOF'
Critic for revision 1 of the design; review round 1 of 3. artifacts/reports/demo-critique-r1.md in this worktree's report. page-design-r1.md (untracked). Its brief is artifacts/reports/demo-design-brief-r1.md and more. The design was written against a checkout 65 code files behind this worktree's origin/main: a cited line number that is merely shifted is NOT a finding; a cited symbol, branch, file or behaviour that does not exist at origin/main IS.
EOF
"$root/scripts/agents/critique-round-task.sh" demo 2 "$tmp/r1" "$tmp/r2"
grep -Fq 'review round 2 of 3' "$tmp/r2" \
  && grep -Fq "Round 1's critique" "$tmp/r2" \
  || fail 'round 2 task missed its replacements or paragraph'
"$root/scripts/agents/critique-round-task.sh" \
  demo 3 "$tmp/r2" "$tmp/r3" --constraints 7
grep -Fq 'review round 3 of 3, the LAST round' "$tmp/r3" \
  && grep -Fq 'constraints C1-7' "$tmp/r3" \
  && grep -Fq "Round 2's critique" "$tmp/r3" \
  || fail 'round 3 task missed its replacements or paragraph'
printf 'stale task\n' >"$tmp/stale"
if "$root/scripts/agents/critique-round-task.sh" \
  demo 3 "$tmp/stale" "$tmp/bad" --constraints 7 >/dev/null 2>&1; then
  fail 'round task accepted missing required text'
fi

cat >"$tmp/current-r1" <<'EOF'
You are the DESIGN CRITIC for revision 1 of the design of goal demo, review round 1 of 3. Read the critic role first. Write ONLY one new file: artifacts/reports/demo-critique-r1.md in this worktree's metasystem directory.

Do not run broad tests or edit the design.

The design is plans/page-design-r1.md (untracked here, written today by a headless design delegate). Its brief is artifacts/reports/demo-design-brief-r1.md and the shared rules are beside it. The design was written against commit abc123; this worktree may be newer. A shifted line is not a finding, but a missing symbol or behaviour is.

Binding, check conformance, do not re-argue: the goal's DONE conditions.

Attack especially:
1. Whether every rule has a witness and named mutation.
2. Whether each unit stays within its changed-line ceiling.

Report findings sorted by materiality. End with one verdict line.
EOF

if ! "$root/scripts/agents/critique-round-task.sh" \
  demo 2 "$tmp/current-r1" "$tmp/current-r2"; then
  fail 'round 2 rejected a current-format round 1 task'
fi
sed \
  -e 's/for revision 1 of/for revision 2 of/' \
  -e 's/review round 1 of 3/review round 2 of 3/' \
  -e 's/demo-critique-r1.md/demo-critique-r2.md/' \
  -e 's/-design-r1.md/-design-r2.md/' \
  -e 's/demo-design-brief-r1.md/demo-design-brief-r2.md/' \
  -e 's/written against a checkout 65 code files behind this worktree.s origin\/main: a cited line number that is merely shifted is NOT a finding; a cited symbol, branch, file or behaviour that does not exist at origin\/main IS./revision 2 was written in this worktree against its own tree: a cited symbol, branch, file, line or behaviour that does not exist here IS a finding./' \
  "$tmp/current-r1" >"$tmp/current-r2-source"
cat >>"$tmp/current-r2-source" <<'EOF'

Round 1's critique is artifacts/reports/demo-critique-r1.md in this worktree. The page's "Fold of critique r1" section claims FIXED, REFUTED or DEFERRED for each round-1 finding. Check every claim FIRST: a FIXED that does not actually fix the failure is material; a REFUTED whose file:line evidence is wrong is material; a DEFERRED that leaves a DONE condition unmet is material. Do not re-raise a round-1 finding that was correctly fixed or correctly refuted. Then attack the content the fold added, with the same attack list.
EOF
cmp -s "$tmp/current-r2" "$tmp/current-r2-source" \
  || fail 'round 2 output differs from the source sed result'

template="$root/scripts/agents/templates/design-common.md"
grep -Fq 'defaults: 60 tool calls, 4000 words' "$template" \
  || fail 'design template lost the default ceilings'
grep -Fq 'A launching brief may override these defaults' "$template" \
  || fail 'design template lost the launching-brief ceiling override'
grep -Fq 'read the page that supersedes' "$template" \
  || fail 'design template lost the superseding-page rule'
grep -Fq 'say which unit is first' "$template" \
  || fail 'design template lost the first-unit rule'
grep -Fq 'the last message is exactly one line' "$template" \
  || fail 'design template lost the exact-last-message rule'

cat >"$tmp/lock.sh" <<'EOF'
cat >"$STEP_DIR/step-1.sh" <<'STEP'
#!/bin/bash
cat >/dev/null
echo 1 >>"$LOCK_LOG"
touch "$STEP_DIR/quit"
STEP
cat >"$STEP_DIR/step-2.sh" <<'STEP'
#!/bin/bash
echo 2 >>"$LOCK_LOG"
STEP
cat >"$STEP_DIR/step-10.sh" <<'STEP'
#!/bin/bash
echo 10 >>"$LOCK_LOG"
STEP
cat >"$STEP_DIR/step-3-backup.sh" <<'STEP'
#!/bin/bash
echo bad >>"$LOCK_LOG"
STEP

testrun_lock_acquire() {
  printf 'lock seat=%s\n' "$1" >>"$LOCK_LOG"
  [ "${LOCK_FAIL:-0}" != 1 ]
}

testrun_lock_release() {
  echo release >>"$LOCK_LOG"
}
EOF

mkdir -p "$tmp/steps"
: >"$tmp/lock.log"
cd "$tmp"
if ! STEP_DIR="$tmp/steps" \
  LOCK_LOG="$tmp/lock.log" \
  "$root/scripts/agents/landing-lane-worker.sh" \
    lane --dir steps --lock-lib lock.sh --seat builder \
    --idle 3600 --poll 0 --repo "$root" >/dev/null; then
  fail 'lane worker did not stop successfully on the quit marker'
fi
cd "$root"

[ "$(head -1 "$tmp/lock.log")" != 1 ] \
  || fail 'lane worker ran a step before lock acquisition'
[ "$(head -1 "$tmp/lock.log")" = 'lock seat=builder' ] \
  || fail 'lane worker did not pass the requested lock seat'
[ "$(paste -sd, "$tmp/lock.log")" = 'lock seat=builder,1,2,10,release' ] \
  || fail 'lane step swallowed later steps from its pass'

: >"$tmp/lock.log"
if STEP_DIR="$tmp/steps" \
  LOCK_LOG="$tmp/lock.log" \
  LOCK_FAIL=1 \
  "$root/scripts/agents/landing-lane-worker.sh" \
    lane --dir "$tmp/steps" --lock-lib "$tmp/lock.sh" \
    --idle 3600 --poll 0 --repo "$root" >/dev/null 2>&1; then
  fail 'lane worker accepted a lock failure'
fi
[ "$(paste -sd, "$tmp/lock.log")" = 'lock seat=lane,release' ] \
  || fail 'lane worker did not release the lock on an error exit'

jq -e \
  '.groups[] | select(.id == "section/headless-launchers-fixtures") |
    if .cwd == "metasystem" then .inputs == ["metasystem/scripts/**"]
    elif .cwd == "." then .inputs == ["scripts/**"]
    else false end' \
  "$root/testing.json" >/dev/null \
  || fail 'headless launcher group inputs do not cover all scripts'

if grep -nE '(^|[;&|[:space:]])node([[:space:]]|$)' \
  "$root/scripts/agents/codex-critique-launch.sh" >"$tmp/banned-script-interpreter"; then
  cat "$tmp/banned-script-interpreter" >&2
  fail 'critique launcher contains a banned interpreter command'
fi

echo 'headless launchers fixtures: PASSED'
