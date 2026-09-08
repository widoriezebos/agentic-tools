#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"
hook=$root/scripts/agents/supervision-hook.sh
[[ -x "$ms" ]] || { echo "runtime hook fixture: proof engine is absent" >&2; exit 1; }
source "$root/scripts/agents/fixture-budget.sh"
harness_fixture_budget_init "$root"
fixture_cap=$(harness_fixture_cap supervision-hook-evidence)
fixture_ledger=01ARZ3NDEKTSV4RRFFQ69G5FAV

tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-runtime-hook-fixture.XXXXXX")
owned_pids=()
owned_paths=()
cleanup() {
  local index pid command
  for index in "${!owned_pids[@]}"; do
    pid=${owned_pids[$index]}
    kill -0 "$pid" 2>/dev/null || continue
    command=$(ps -p "$pid" -o command= 2>/dev/null || true)
    if [[ "$command" == *"${owned_paths[$index]}"* ]]; then
      kill -TERM "$pid" 2>/dev/null || true
      wait "$pid" 2>/dev/null || true
    fi
  done
  rm -rf "$tmp"
}
trap cleanup EXIT

clean_git() {
  env -u GIT_DIR -u GIT_WORK_TREE -u GIT_COMMON_DIR -u GIT_INDEX_FILE \
    -u GIT_CEILING_DIRECTORIES -u GIT_DISCOVERY_ACROSS_FILESYSTEM \
    -u GIT_OBJECT_DIRECTORY -u GIT_ALTERNATE_OBJECT_DIRECTORIES \
    -u GIT_CONFIG -u GIT_CONFIG_PARAMETERS -u GIT_CONFIG_COUNT \
    -u GIT_CONFIG_GLOBAL -u GIT_CONFIG_SYSTEM -u GIT_CONFIG_NOSYSTEM \
    -u GIT_GRAFT_FILE -u GIT_SHALLOW_FILE -u GIT_REPLACE_REF_BASE \
    -u GIT_IMPLICIT_WORK_TREE -u GIT_NO_REPLACE_OBJECTS -u GIT_PREFIX \
    git "$@"
}

make_repo() { # repository
  local repo=$1
  mkdir -p "$repo"
  clean_git init -q -b main "$repo"
  clean_git -C "$repo" config user.name fixture
  clean_git -C "$repo" config user.email fixture@example.invalid
  clean_git -C "$repo" config metasystem.goal.machine runtime-hook-fixture
  mkdir -p "$repo/plans/goals"
  printf 'fixture\n' >"$repo/tracked.txt"
  printf '# Backlog\n\n- Identity: %s\n- FormatVersion: 1\n- SyncMode: local\n- Revision: 1\n' \
    "$fixture_ledger" >"$repo/plans/goals/backlog.md"
  clean_git -C "$repo" add tracked.txt plans/goals/backlog.md
  clean_git -C "$repo" commit -qm fixture
  clean_git -C "$repo" update-ref refs/metasystem/goals/accepted HEAD
}

seed_coordinator_state() { # state root
  local state=$1
  mkdir -p "$state/artifacts/agents/mains" "$state/artifacts/agents/session-stops" \
    "$state/artifacts/agents/steward"
  printf '{"sessionId":"equal-session","runtime":"claude"}\n' >"$state/artifacts/agents/mains/coordinator.json"
  printf '{"holderMainId":"coordinator","claimEpoch":7}\n' >"$state/artifacts/agents/mains/worktree-lease.json"
  printf '{"turn":"coordinator-unchanged"}\n' >"$state/artifacts/agents/turn-verdict-state.json"
  printf '{"authorizationId":"coordinator-unchanged"}\n' >"$state/artifacts/agents/session-stops/equal-session.json"
  printf '{"cursor":17,"digest":"coordinator-unchanged"}\n' >"$state/artifacts/agents/steward/digest-cursor.json"
  printf '{"schema":1,"ledger":"%s","machine":"runtime-hook-fixture","declaredBy":"Wido","declaredAt":"2026-09-07T00:00:00Z"}\n' \
    "$fixture_ledger" >"$state/artifacts/agents/brain.json"
  printf '{"line":"BRAIN: runtime-hook-fixture for ledger %s since 2026-09-07T00:00:00Z","producedAt":"2026-09-07T00:00:00Z","lastPostedAt":"2026-09-07T00:00:00Z"}\n' \
    "$fixture_ledger" >"$state/artifacts/agents/brain-status.json"
  printf '{"schema":1,"cursor":17,"prefixSha256":"%064d"}\n' 0 \
    >"$state/artifacts/agents/steward/narrator-digest-brain-cursor.json"
  [[ "$("$ms" json get --value "$("$ms" brain show --root "$state")" --field state)" == declared ]] \
    || { echo "runtime hook fixture did not seed a valid declared brain" >&2; return 1; }
}

state_snapshot() { # state root
  local state=$1
  find "$state/artifacts/agents" -type f -print | LC_ALL=C sort | \
    while IFS= read -r path; do
      shasum -a 256 "$path"
    done
}

write_job_owner() { # state root, job, pid
  local state=$1 job=$2 pid=$3 probe sec micro ticks boot exact_fields
  probe=$("$ms" proc probe --pid "$pid")
  sec=$("$ms" json get --value "$probe" --field startedAtUnix)
  micro=$("$ms" json get --value "$probe" --field startedAtUnixMicro)
  ticks=$("$ms" json get --value "$probe" --field startTicks)
  boot=$("$ms" json get --value "$probe" --field bootId --default '')
  exact_fields=$(printf ',"pidStartedAtExactMicro":%s' "$micro")
  if [[ "$ticks" =~ ^[1-9][0-9]*$ && -n "$boot" ]]; then
    exact_fields=$(printf ',"pidStartTicks":%s,"bootId":"%s"' "$ticks" "$boot")
  fi
  mkdir -p "$state/artifacts/agents/jobs"
  printf '{"jobId":"%s","pid":%s,"pidStartedAt":%s%s,"custodyProcesses":[]}\n' \
    "$job" "$pid" "$sec" "$exact_fields" >"$state/artifacts/agents/jobs/$job.json"
}

wait_owned() { # fixture name, pids...
  local name=$1 deadline pid status=0
  shift
  deadline=$((SECONDS + fixture_cap))
  while :; do
    local live=0
    for pid in "$@"; do
      kill -0 "$pid" 2>/dev/null && live=1
    done
    (( live )) || break
    if (( SECONDS >= deadline )); then
      echo "$name exceeded its ${fixture_cap}s fixture ceiling" >&2
      return 1
    fi
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
  done
  for pid in "$@"; do
    wait "$pid" || status=$?
  done
  (( status == 0 )) || { echo "$name child failed with status $status" >&2; return 1; }
}

assert_no_brain_effect_calls() { # call log, fixture name
  local log=$1 name=$2
  if grep -E '^(brain boot|brain digest-advance|channel status)( |$)' "$log" >"$tmp/unexpected-brain-calls"; then
    echo "$name reached a brain state effect after it should have skipped" >&2
    cat "$tmp/unexpected-brain-calls" >&2
    return 1
  fi
}

make_identity_engine() { # output engine
  local output=$1
  cat >"$output" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
[[ -z ${METASYSTEM_RUNTIME_HOOK_CALL_LOG:-} ]] || printf '%s\n' "$*" >>"$METASYSTEM_RUNTIME_HOOK_CALL_LOG"
if [[ ${1:-} == proc && ${2:-} == find-ancestor ]]; then
  sleep "${RUNTIME_HOOK_DELAY:-0}"
  printf '{"runtime":"%s","pid":1,"pidStartedAt":1}\n' "${RUNTIME_HOOK_IDENTITY_RUNTIME:?}"
  exit 0
fi
exec "${RUNTIME_HOOK_REAL_ENGINE:?}" "$@"
SH
  chmod +x "$output"
}

TestImportedClaudeHookSkipsDevinOutsideExecutionRoster() {
  local repo=$tmp/imported workspace_state before after engine event output
  make_repo "$repo"
  seed_coordinator_state "$repo"
  workspace_state=$(state_snapshot "$repo")
  before=$workspace_state
  engine=$tmp/all-host-engine
  make_identity_engine "$engine"
  : >"$tmp/all-host-calls.log"
  for event in start receipt stop end; do
    output=$(printf '{"session_id":"equal-session","cwd":"%s"}\n' "$repo" | \
      METASYSTEM_RUNTIME_HOOK_CALL_LOG="$tmp/all-host-calls.log" METASYSTEM_BIN="$engine" \
      RUNTIME_HOOK_REAL_ENGINE="$ms" RUNTIME_HOOK_IDENTITY_RUNTIME=devin RUNTIME_HOOK_DELAY=0 \
      bash "$hook" claude "$event")
    [[ -z "$output" ]] || { echo "imported Claude $event hook was not silent: $output" >&2; return 1; }
  done
  after=$(state_snapshot "$repo")
  [[ "$after" == "$before" ]] || { echo "foreign imported hooks changed coordinator or brain state" >&2; return 1; }
  grep -Fq -- '--all-hosts' "$tmp/all-host-calls.log" \
    || { echo "foreign-runtime guard did not use the all-host registry selector" >&2; return 1; }
  assert_no_brain_effect_calls "$tmp/all-host-calls.log" "foreign imported Claude hooks"
}

TestUnhintedLocalDelegateSkipsBeforeBrainEffects() {
  local repo=$tmp/unhinted-local engine before after event output
  make_repo "$repo"
  seed_coordinator_state "$repo"
  write_job_owner "$repo" job-unhinted-local "$$"
  before=$(state_snapshot "$repo")
  engine=$tmp/unhinted-local-engine
  make_identity_engine "$engine"
  : >"$tmp/unhinted-local-calls.log"
  for event in start receipt stop end; do
    output=$(printf '{"session_id":"equal-session","cwd":"%s"}\n' "$repo" | \
      METASYSTEM_RUNTIME_HOOK_CALL_LOG="$tmp/unhinted-local-calls.log" METASYSTEM_BIN="$engine" \
      RUNTIME_HOOK_REAL_ENGINE="$ms" RUNTIME_HOOK_IDENTITY_RUNTIME=claude RUNTIME_HOOK_DELAY=0 \
      bash "$hook" claude "$event")
    [[ -z "$output" ]] || { echo "unhinted local delegate $event hook was not silent: $output" >&2; return 1; }
  done
  after=$(state_snapshot "$repo")
  [[ "$after" == "$before" ]] || { echo "unhinted local delegate hooks changed coordinator or brain state" >&2; return 1; }
  grep -Fq 'lease hook-delegate' "$tmp/unhinted-local-calls.log" \
    || { echo "unhinted local delegate did not use the local custody owner" >&2; return 1; }
  if grep -Fq 'proc find-ancestor' "$tmp/unhinted-local-calls.log"; then
    echo "unhinted local delegate reached runtime discovery after custody matched" >&2
    return 1
  fi
  assert_no_brain_effect_calls "$tmp/unhinted-local-calls.log" "unhinted local delegate hooks"
}

TestRuntimeHookGuardDelayStillEmitsOneStopVerdictWithinTimeout() {
  local repo=$tmp/delayed engine before after output started elapsed
  make_repo "$repo"
  seed_coordinator_state "$repo"
  before=$(state_snapshot "$repo")
  engine=$tmp/delayed-engine
  make_identity_engine "$engine"
  : >"$tmp/delayed-calls.log"
  started=$SECONDS
  output=$(printf '{"session_id":"equal-session","cwd":"%s"}\n' "$repo" | \
    METASYSTEM_RUNTIME_HOOK_CALL_LOG="$tmp/delayed-calls.log" METASYSTEM_BIN="$engine" \
    RUNTIME_HOOK_REAL_ENGINE="$ms" RUNTIME_HOOK_IDENTITY_RUNTIME=devin RUNTIME_HOOK_DELAY=2 \
    bash "$hook" claude stop)
  elapsed=$((SECONDS - started))
  [[ -z "$output" && "$elapsed" -lt 15 ]] \
    || { echo "delayed runtime guard did not finish as one silent Stop result in budget (elapsed ${elapsed}s): $output" >&2; return 1; }
  after=$(state_snapshot "$repo")
  [[ "$after" == "$before" ]] || { echo "delayed guard changed coordinator or brain state" >&2; return 1; }
  assert_no_brain_effect_calls "$tmp/delayed-calls.log" "delayed foreign-runtime guard"
}

prepare_registry() { # state/installation root
  local registry=$1
  make_repo "$registry"
  mkdir -p "$registry/bin"
  cp "$ms" "$registry/bin/metasystem-real"
  cat >"$registry/bin/metasystem" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
[[ -z ${METASYSTEM_RUNTIME_HOOK_CALL_LOG:-} ]] || printf '%s\n' "$*" >>"$METASYSTEM_RUNTIME_HOOK_CALL_LOG"
exec "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/metasystem-real" "$@"
SH
  chmod +x "$registry/bin/metasystem" "$registry/bin/metasystem-real"
  printf 'metasystem.runtimes=none\n' >"$registry/metasystem.conf"
  seed_coordinator_state "$registry"
}

make_provider_child() { # script path
  local child=$1
  cat >"$child" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
[[ -z ${HOOK_GATE:-} ]] || {
  deadline=$((SECONDS + 30))
  while [[ ! -e "$HOOK_GATE" ]]; do
    (( SECONDS < deadline )) || { echo "provider child gate timed out" >&2; exit 1; }
    sleep 0.05
  done
}
cd "${HOOK_WORKSPACE:?}"
for event in start receipt stop end; do
  output=$(printf '{"session_id":"equal-session","cwd":"%s"}\n' "$HOOK_WORKSPACE" | \
    bash "${HOOK_ENTRY:?}" "${HOOK_RUNTIME:?}" "$event")
  [[ -z "$output" ]] || { echo "$HOOK_RUNTIME $event was not silent: $output" >&2; exit 1; }
  printf '%s %s equal-session\n' "$HOOK_RUNTIME" "$event" >>"${HOOK_TRACE:?}"
done
SH
  chmod +x "$child"
}

TestConcurrentRuntimeChildrenAndDetachedCustodyIsolation() {
  local registry=$tmp/registry repo=$tmp/provider-repo linked_one=$tmp/provider-linked-one
  local linked_two=$tmp/provider-linked-two before after runtime workspace job child pid gate
  local pids=()
  prepare_registry "$registry"
  make_repo "$repo"
  clean_git -C "$repo" worktree add -q -b fixture-linked-one "$linked_one"
  clean_git -C "$repo" worktree add -q -b fixture-linked-two "$linked_two"
  for job in job-claude job-codex job-devin; do
    write_job_owner "$registry" "$job" "$$"
  done
  before=$(state_snapshot "$registry")
  : >"$tmp/concurrent-hook-trace.log"
  : >"$tmp/concurrent-hook-calls.log"
  local index=0
  for runtime in claude codex devin; do
    case "$index" in 0) workspace=$repo ;; 1) workspace=$linked_one ;; *) workspace=$linked_two ;; esac
    job=job-$runtime
    child=$tmp/$runtime-provider-child
    make_provider_child "$child"
    METASYSTEM_BIN="$registry/bin/metasystem" \
      METASYSTEM_HOOK_DELEGATE_STATE_ROOT="$registry" \
      METASYSTEM_HOOK_DELEGATE_INSTALLATION_ROOT="$registry" \
      METASYSTEM_HOOK_DELEGATE_JOB="$job" HOOK_ENTRY="$hook" \
      HOOK_RUNTIME="$runtime" HOOK_WORKSPACE="$workspace" \
      METASYSTEM_RUNTIME_HOOK_CALL_LOG="$tmp/concurrent-hook-calls.log" \
      HOOK_TRACE="$tmp/concurrent-hook-trace.log" "$child" &
    pid=$!
    pids+=("$pid")
    owned_pids+=("$pid")
    owned_paths+=("$child")
    index=$((index + 1))
  done
  wait_owned concurrent-runtime-children "${pids[@]}"
  after=$(state_snapshot "$registry")
  [[ "$after" == "$before" ]] || { echo "concurrent runtime children changed coordinator or brain state" >&2; return 1; }
  [[ $(wc -l <"$tmp/concurrent-hook-trace.log") -eq 12 ]] \
    || { echo "concurrent runtime children did not complete all lifecycle events" >&2; return 1; }
  assert_no_brain_effect_calls "$tmp/concurrent-hook-calls.log" "hinted runtime children"

  # A child whose adapter ancestor is gone remains isolated by its own exact
  # custody identity. The gate holds it until that identity is published.
  child=$tmp/detached-provider-child
  gate=$tmp/detached.gate
  make_provider_child "$child"
  METASYSTEM_BIN="$registry/bin/metasystem" \
    METASYSTEM_HOOK_DELEGATE_STATE_ROOT="$registry" \
    METASYSTEM_HOOK_DELEGATE_INSTALLATION_ROOT="$registry" \
    METASYSTEM_HOOK_DELEGATE_JOB=job-detached HOOK_ENTRY="$hook" \
    HOOK_RUNTIME=codex HOOK_WORKSPACE="$linked_one" HOOK_GATE="$gate" \
    METASYSTEM_RUNTIME_HOOK_CALL_LOG="$tmp/concurrent-hook-calls.log" \
    HOOK_TRACE="$tmp/detached-hook-trace.log" "$child" &
  pid=$!
  owned_pids+=("$pid")
  owned_paths+=("$child")
  write_job_owner "$registry" job-detached "$pid"
  before=$(state_snapshot "$registry")
  : >"$gate"
  wait_owned detached-child-custody "$pid"
  after=$(state_snapshot "$registry")
  [[ "$after" == "$before" ]] || { echo "detached child custody changed coordinator or brain state" >&2; return 1; }
  assert_no_brain_effect_calls "$tmp/concurrent-hook-calls.log" "detached runtime child"
}

TestColdAndMissionHostsAreNotSuppressedByDelegateClassification() {
  local cold=$tmp/cold-main mission=$tmp/mission-main engine output
  make_repo "$cold"
  make_repo "$mission"
  mkdir -p "$mission/artifacts/agents/missions/mission-one"
  printf '{"host":{"pid":1,"pidStartedAt":1}}\n' >"$mission/artifacts/agents/missions/mission-one/state.json"
  engine=$tmp/cold-engine
  cat >"$engine" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
if [[ ${1:-} == proc && ${2:-} == find-ancestor ]]; then
  printf '{"runtime":"claude","pid":%s,"pidStartedAt":%s}\n' \
    "${RUNTIME_HOOK_MAIN_PID:?}" "${RUNTIME_HOOK_MAIN_STARTED:?}"
  exit 0
fi
if [[ ${1:-} == lease && ${2:-} == hook-delegate ]]; then
  exit 3
fi
if [[ ${1:-} == lease && ${2:-} == classify ]]; then
  printf '{"class":"MAIN","mainId":"main-fixture","holder":true}\n'
  exit 0
fi
if [[ ${1:-} == up ]]; then
  printf '%s\n' "$*" >>"${METASYSTEM_RUNTIME_HOOK_UP_LOG:?}"
  printf 'fixture up\n'
  exit 0
fi
exec "${RUNTIME_HOOK_REAL_ENGINE:?}" "$@"
SH
  chmod +x "$engine"
  : >"$tmp/cold-up.log"
  for output in "$cold" "$mission"; do
    printf '{"session_id":"equal-session","cwd":"%s"}\n' "$output" | \
      METASYSTEM_RUNTIME_HOOK_UP_LOG="$tmp/cold-up.log" METASYSTEM_BIN="$engine" \
      RUNTIME_HOOK_REAL_ENGINE="$ms" RUNTIME_HOOK_MAIN_PID="$$" \
      RUNTIME_HOOK_MAIN_STARTED="$("$ms" proc started-at --pid $$)" \
      bash "$hook" claude start >/dev/null
  done
  [[ $(wc -l <"$tmp/cold-up.log") -eq 2 ]] \
    || { echo "cold or mission host was suppressed instead of reaching enrollment" >&2; return 1; }

  prepare_registry "$tmp/forged-registry"
  mkdir -p "$tmp/forged-registry/artifacts/agents/jobs"
  printf '{"jobId":"job-forged","pid":1,"pidStartedAt":1}\n' \
    >"$tmp/forged-registry/artifacts/agents/jobs/job-forged.json"
  if printf '{"session_id":"equal-session","cwd":"%s"}\n' "$cold" | \
      METASYSTEM_HOOK_DELEGATE_STATE_ROOT="$tmp/forged-registry" \
      METASYSTEM_HOOK_DELEGATE_INSTALLATION_ROOT="$tmp/forged-registry" \
      METASYSTEM_HOOK_DELEGATE_JOB=job-forged \
      bash "$hook" claude start >/dev/null 2>&1; then
    echo "a forged delegate hint suppressed a fresh host" >&2
    return 1
  fi
}

TestDevinDeliveryRepairHookIsolation() {
  local registry=$tmp/devin-repair-registry repo=$tmp/devin-repair-repo
  local linked=$tmp/devin-repair-linked before after workspace round_dir output trace_count
  prepare_registry "$registry"
  make_repo "$repo"
  clean_git -C "$repo" worktree add -q -b fixture-devin-repair "$linked"
  write_job_owner "$registry" job-devin-repair "$$"
  before=$(state_snapshot "$registry")
  : >"$tmp/devin-repair-hook-trace.log"
  : >"$tmp/devin-repair-hook-calls.log"

  # Use the adapter's actual secondary launch function. The shared adapter
  # owner initializes the same canonical context that prepare_supervision uses;
  # the exec'd repair child must inherit it without a coordinator export.
  source "$root/scripts/agents/adapters/runtime-common.sh"
  initialize_hook_delegate_context "$registry" job-devin-repair
  eval "$(awk '/^runtime_repair_invoke\(\)/ { copy=1 } copy { print } copy && /^}/ { exit }' "$root/scripts/agents/adapters/devin.sh")"
  mkdir -p "$tmp/devin-shim"
  cat >"$tmp/devin-shim/devin" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
export_path=
while (($#)); do
  case "$1" in
    --export) export_path=$2; shift 2 ;;
    *) shift ;;
  esac
done
for event in start receipt stop end; do
  output=$(printf '{"session_id":"equal-session","cwd":"%s"}\n' "$PWD" | \
    bash "${HOOK_ENTRY:?}" devin "$event")
  [[ -z "$output" ]] || { echo "Devin repair $event hook was not silent: $output" >&2; exit 1; }
  printf 'devin-repair %s equal-session\n' "$event" >>"${HOOK_TRACE:?}"
done
[[ -z "$export_path" ]] || printf '{"session_id":"equal-session"}\n' >"$export_path"
printf 'repair child completed\n'
SH
  chmod +x "$tmp/devin-shim/devin"
  PATH="$tmp/devin-shim:$PATH"
  export PATH HOOK_ENTRY="$hook" HOOK_TRACE="$tmp/devin-repair-hook-trace.log"
  export METASYSTEM_RUNTIME_HOOK_CALL_LOG="$tmp/devin-repair-hook-calls.log"
  session_id=equal-session
  requested_model=fixture-model
  permission_mode=auto
  config_file=$tmp/devin-repair-config.json
  printf '{}\n' >"$config_file"
  for workspace in "$repo" "$linked"; do
    round_dir=$(mktemp -d "$tmp/devin-repair-round.XXXXXX")
    log=$round_dir/adapter.log
    printf 'repair the delivery\n' >"$round_dir/repair-1.prompt.md"
    output=$round_dir/repair-1.out
    runtime_repair_invoke "$round_dir/repair-1.prompt.md" "$output" \
      || { echo "actual Devin delivery-repair launch failed for $workspace" >&2; return 1; }
    grep -Fq 'repair child completed' "$output" \
      || { echo "actual Devin delivery-repair child did not complete" >&2; return 1; }
  done
  trace_count=$(wc -l <"$tmp/devin-repair-hook-trace.log")
  [[ "$trace_count" -eq 8 ]] \
    || { echo "Devin repair launches emitted $trace_count lifecycle traces, want 8" >&2; return 1; }
  after=$(state_snapshot "$registry")
  [[ "$after" == "$before" ]] \
    || { echo "Devin delivery repair changed coordinator announcement, lease, turn, authorization, digest, or brain state" >&2; return 1; }
  assert_no_brain_effect_calls "$tmp/devin-repair-hook-calls.log" "Devin delivery repair hooks"
}

TestImportedClaudeHookSkipsDevinOutsideExecutionRoster
TestUnhintedLocalDelegateSkipsBeforeBrainEffects
TestRuntimeHookGuardDelayStillEmitsOneStopVerdictWithinTimeout
TestConcurrentRuntimeChildrenAndDetachedCustodyIsolation
TestColdAndMissionHostsAreNotSuppressedByDelegateClassification
TestDevinDeliveryRepairHookIsolation

echo "runtime hook declared-brain all-host, unhinted and hinted delegate, deadline, cold-host, forged-hint, detached-custody, and Devin delivery-repair isolation fixtures passed"
