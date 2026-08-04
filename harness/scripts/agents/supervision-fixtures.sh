#!/usr/bin/env bash
set -euo pipefail

# Focused fixtures for section 3.11. Every wait in this file goes through a
# named ceiling so a broken supervisor fails loudly instead of hanging (IL-1).

source_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
source "$source_root/scripts/agents/fixture-budget.sh"
harness_fixture_budget_init "$source_root"
fixture_base_cap_sec=${HARNESS_SUPERVISION_FIXTURE_TIMEOUT_SEC:-12}
[[ "$fixture_base_cap_sec" =~ ^[1-9][0-9]*$ && "$fixture_base_cap_sec" -le 60 ]] \
  || { echo "HARNESS_SUPERVISION_FIXTURE_TIMEOUT_SEC must be 1..60" >&2; exit 1; }
fixture_ceiling_sec=$(harness_fixture_scaled_cap "$fixture_base_cap_sec")

tmp=$(mktemp -d)
owned_pids=()
fixture_harness_roots=()

process_started_at() {
  "$source_root/scripts/agents/process-census.py" started-at --pid "$1"
}

process_identity_alive() { # pid, start
  "$source_root/scripts/agents/process-census.py" alive --pid "$1" --start-time "$2" >/dev/null 2>&1
}

wait_until() { # name, shell predicate...
  local name=$1 started=$SECONDS deadline=$((SECONDS + fixture_ceiling_sec)) elapsed
  shift
  until "$@"; do
    if (( SECONDS >= deadline )); then
      elapsed=$((SECONDS - started))
      echo "supervision fixture timed out: $name (elapsed: ${elapsed}s; scaled cap: ${fixture_ceiling_sec}s)" >&2
      return 1
    fi
    sleep 0.05
  done
}

stop_owned_pid() { # name, pid, start
  local name=$1 pid=$2 start=$3 started deadline elapsed
  process_identity_alive "$pid" "$start" || return 0
  kill -TERM "$pid" 2>/dev/null || true
  started=$SECONDS
  deadline=$((SECONDS + fixture_ceiling_sec))
  while process_identity_alive "$pid" "$start"; do
    if (( SECONDS >= deadline )); then
      elapsed=$((SECONDS - started))
      echo "supervision fixture stop timed out: $name pid=$pid (elapsed: ${elapsed}s; scaled cap: ${fixture_ceiling_sec}s)" >&2
      kill -KILL "$pid" 2>/dev/null || true
      return 1
    fi
    sleep 0.05
  done
  wait "$pid" 2>/dev/null || true
}

wait_for_child_exit() { # name, child pid
  local name=$1 pid=$2 result
  if ! wait_until "$name" bash -c '! kill -0 "$1" 2>/dev/null' _ "$pid"; then
    kill -TERM "$pid" 2>/dev/null || true
    return 124
  fi
  # The child is already observed dead under the named ceiling above; this
  # wait only reaps its immediately available status.
  if wait "$pid"; then result=0; else result=$?; fi
  return "$result"
}

cleanup() {
  local harness_path tuple pid start
  if [[ -n "${operator_harness:-}" && -x "$operator_harness/scripts/agents/arm-supervision.sh" ]]; then
    if declare -p operator_env >/dev/null 2>&1; then
      "${operator_env[@]}" "$operator_harness/scripts/agents/arm-supervision.sh" \
        --repo "$operator_harness" --shutdown >/dev/null 2>&1 || true
    else
      "$operator_harness/scripts/agents/arm-supervision.sh" \
        --repo "$operator_harness" --shutdown >/dev/null 2>&1 || true
    fi
  fi
  for harness_path in ${fixture_harness_roots[@]+"${fixture_harness_roots[@]}"}; do
    if [[ -x "$harness_path/scripts/agents/arm-supervision.sh" ]]; then
      "$harness_path/scripts/agents/arm-supervision.sh" --repo "$harness_path" --shutdown >/dev/null 2>&1 || true
    fi
  done
  for tuple in ${owned_pids[@]+"${owned_pids[@]}"}; do
    IFS=: read -r pid start <<<"$tuple"
    stop_owned_pid cleanup "$pid" "$start" >/dev/null 2>&1 || true
  done
  if [[ -n "${HARNESS_KEEP_SUPERVISION_FIXTURE:-}" ]]; then
    echo "kept supervision fixture: $tmp" >&2
  else
    rm -rf "$tmp"
  fi
}
trap cleanup EXIT

make_repo() { # destination
  local repo=$1 evidence=$tmp/evidence-$(basename "$repo")
  fixture_harness_roots+=("$repo")
  mkdir -p "$repo/scripts"
  cp -R "$source_root/scripts/agents" "$repo/scripts/"
  cp "$source_root/scripts/watch-background-jobs.sh" \
    "$source_root/scripts/harness-config.sh" "$source_root/harness.conf" "$repo/scripts/../" 2>/dev/null || true
  # The previous copy places harness.conf under scripts/; put each asset at its
  # actual shipped path explicitly.
  cp "$source_root/scripts/watch-background-jobs.sh" "$repo/scripts/"
  cp "$source_root/scripts/harness-config.sh" "$repo/scripts/"
  cp "$source_root/harness.conf" "$repo/harness.conf"
  perl -0pi -e 's/^harness\.runtimes=.*$/harness.runtimes=fake/m; s|^evidence\.root=.*$|evidence.root='"$evidence"'|m; s/^watch\.interval-sec=.*$/watch.interval-sec=1/m; s/^census\.log-max-bytes=.*$/census.log-max-bytes=350/m; s/^model\.tier\.1=.*$/model.tier.1=fake:fake-model/m; s/^model\.tier\.2=.*$/model.tier.2=/m; s/^model\.tier\.3=.*$/model.tier.3=/m; s/^role\.default\.runtime=.*$/role.default.runtime=fake/m; s/^role\.default\.model\.codex=.*$/role.default.model.fake=fake-model/m; s/^role\.default\.model\.(?:claude|devin)=.*\n//mg; s/\.runtime=(?:codex|devin)$/\.runtime=fake/mg; s/\.model\.(?:codex|devin)=.*$/\.model.fake=fake-model/mg' "$repo/harness.conf"
  grep -q '^watch\.interval-sec=' "$repo/harness.conf" || printf 'watch.interval-sec=1\n' >>"$repo/harness.conf"
  grep -q '^census\.log-max-bytes=' "$repo/harness.conf" || printf 'census.log-max-bytes=350\n' >>"$repo/harness.conf"
  git -C "$repo" init -q
  git -C "$repo" add .
  git -C "$repo" -c user.name=harness -c user.email=harness.invalid commit -qm fixture
}

json_field() { # file, dotted field
  python3 - "$1" "$2" <<'PY'
import json, sys
value = json.load(open(sys.argv[1]))
for part in sys.argv[2].split("."):
    value = value[int(part)] if isinstance(value, list) else value[part]
if isinstance(value, (dict, list)): print(json.dumps(value, separators=(",", ":")))
elif isinstance(value, bool): print("true" if value else "false")
elif value is not None: print(value)
PY
}

inventory_has() { # last-census, class, pid
  python3 - "$1" "$2" "$3" <<'PY'
import json, sys
value = json.load(open(sys.argv[1]))
raise SystemExit(0 if any(str(item.get("pid")) == sys.argv[3] and item.get("class") == sys.argv[2] for item in value.get("inventory", [])) else 1)
PY
}

write_process_fixture() { # output followed by pid|ppid|pgid|started|argv|cwd rows
  local output=$1
  shift
  python3 - "$output" "$@" <<'PY'
import json, sys
from pathlib import Path
rows = []
for raw in sys.argv[2:]:
    pid, ppid, pgid, started, argv, cwd = raw.split("|", 5)
    rows.append({"pid": int(pid), "ppid": int(ppid), "pgid": int(pgid),
                 "pidStartedAt": int(started), "argv": argv,
                 "cwd": None if cwd == "UNRESOLVED" else cwd,
                 "cwdError": cwd == "UNRESOLVED", "alive": True})
Path(sys.argv[1]).write_text(json.dumps(rows, indent=2) + "\n")
PY
}

# The ordinary operator layout keeps the vendored harness one directory below
# the Git toplevel. Its primary branch uses the shipped configuration and real
# process sources: no fake census table, fake identity table, or rewritten
# harness.conf. Restricted hosts keep the same path regression through the
# existing fake-process fallback below.
operator_scope=$tmp/operator-repo
operator_harness=$operator_scope/harness
mkdir -p "$operator_scope"
# Copy the shipped tree only. artifacts/ holds this session's live runtime
# state, including the supervision lock naming the operator's own owner pid;
# carrying it into the sandbox makes the fixture's shutdown stop a supervisor
# it does not own, which silently disarms the machine running the suite.
mkdir -p "$operator_harness"
(cd "$source_root" && for entry in * .[!.]*; do
  [[ -e "$entry" ]] || continue
  [[ "$entry" == artifacts ]] && continue
  cp -R "$entry" "$operator_harness/"
done)
operator_scope=$(cd "$operator_scope" && pwd -P)
operator_harness=$(cd "$operator_harness" && pwd -P)
git -C "$operator_scope" init -q
git -C "$operator_scope" add harness
git -C "$operator_scope" -c user.name=harness -c user.email=harness.invalid commit -qm fixture
fixture_harness_roots+=("$operator_harness")
operator_arm=$operator_harness/scripts/agents/arm-supervision.sh
operator_census=$operator_harness/scripts/agents/process-census.py

# A plain shell with no matching ancestor must refuse, but the refusal tells an
# operator both supported ways forward. Restricting discovery to the fake
# signature makes this deterministic without replacing any process source.
set +e
(
  cd "$operator_harness"
  HARNESS_AGENT_RUNTIME=fake "$operator_arm" --repo "$PWD"
) >"$tmp/operator-identity.out" 2>&1 &
operator_identity_driver=$!
wait_for_child_exit "nested operator identity refusal" "$operator_identity_driver"
operator_identity_rc=$?
set -e
[[ $operator_identity_rc -ne 0 ]] \
  || { echo "nested operator identity inference unexpectedly succeeded" >&2; exit 1; }
grep -Fq 'Pass --pid <agent-pid> and --start-time <epoch-seconds>' "$tmp/operator-identity.out" \
  && grep -Fq 'ancestor matches a configured runtime signature' "$tmp/operator-identity.out" \
  || { echo "nested operator identity refusal is not actionable" >&2; cat "$tmp/operator-identity.out" >&2; exit 1; }

operator_env=(env -u HARNESS_CENSUS_PROCESS_FILE -u HARNESS_FAKE_PROCESS_IDENTITY_FILE
  -u HARNESS_FAKE_AGENT_ANCESTOR_PID -u HARNESS_AGENT_RUNTIME
  -u HARNESS_SESSION_ID -u HARNESS_INSTANCE_TAG -u HARNESS_WATCH_INTERVAL_SEC
  -u HARNESS_CENSUS_LOG_MAX_BYTES -u HARNESS_CENSUS_MAX_INTERVAL_SHARE_PERCENT)
if [[ -n "${HARNESS_SUPERVISION_OPERATOR_FIXTURE_FAKE:-}" ]] \
    || ! ps -p "$$" -o lstart= >/dev/null 2>&1 \
    || ! ps -axo pid=,ppid=,pgid=,lstart=,command= >/dev/null 2>&1; then
  # Restricted execution sandboxes may prohibit ps entirely. Keep the nested
  # path/registry regression active through the existing restricted-CI source;
  # hosts with an ordinary process table always take the unconstructed branch.
  echo "nested ordinary operator fixture: real process source unavailable; using restricted-CI identity source" >&2
  operator_process_fixture=$tmp/operator-processes.json
  operator_identity_fixture=$tmp/operator-identities.json
  printf '[]\n' >"$operator_process_fixture"
  printf '{}\n' >"$operator_identity_fixture"
  perl -0pi -e 's/^harness\.runtimes=.*$/harness.runtimes=fake/m; s/^watch\.interval-sec=.*$/watch.interval-sec=1/m' "$operator_harness/harness.conf"
  operator_env=(env HARNESS_CENSUS_PROCESS_FILE="$operator_process_fixture"
    HARNESS_FAKE_PROCESS_IDENTITY_FILE="$operator_identity_fixture")
fi
operator_start=$("${operator_env[@]}" "$operator_census" started-at --pid "$$")
(
  cd "$operator_harness"
  "${operator_env[@]}" \
    "$operator_arm" --repo "$PWD" --session operator-path --pid "$$" \
      --start-time "$operator_start" --tag operator-path
) >"$tmp/operator-arm.out" 2>&1 &
operator_driver=$!
operator_driver_start=$("${operator_env[@]}" "$operator_census" started-at --pid "$operator_driver")
owned_pids+=("$operator_driver:$operator_driver_start")
wait_for_child_exit "nested ordinary operator arming" "$operator_driver"
grep -Eq "(^|[[:space:]])ARMED repo=$operator_scope([[:space:]]|$)" "$tmp/operator-arm.out" \
  || { cat "$tmp/operator-arm.out" >&2; exit 1; }
[[ -s "$operator_harness/artifacts/agents/supervision/last-census.json" ]] \
  || { echo "nested operator path did not write the harness registry" >&2; exit 1; }
[[ ! -e "$operator_scope/artifacts" ]] \
  || { echo "nested operator path wrote registries at the Git toplevel" >&2; exit 1; }
if grep -Eq 'harness\.runtimes has no signature adapters|No such file or directory' "$tmp/operator-arm.out"; then
  cat "$tmp/operator-arm.out" >&2
  exit 1
fi
(
  cd "$operator_harness"
  "$operator_arm" fingerprint --repo "$PWD"
) >/dev/null
printf '{"session_id":"operator-path","cwd":"%s","hook_event_name":"Stop"}\n' "$operator_harness" \
  | (cd "$operator_harness" && "${operator_env[@]}" scripts/agents/supervision-hook.sh fake stop) \
  >"$tmp/operator-hook.out"
if grep -Fq '$(git rev-parse --show-toplevel)/scripts/agents/supervision-hook.sh' \
    "$operator_harness"/scripts/enforcement/*hooks.json; then
  echo "nested operator hook configuration still resolves harness scripts from the Git toplevel" >&2
  exit 1
fi
"${operator_env[@]}" "$operator_arm" --repo "$operator_harness" --shutdown >/dev/null 2>&1

echo "nested ordinary operator supervision fixture passed" >&2

repo=$tmp/repo
mkdir -p "$repo"
make_repo "$repo"
arm="$repo/scripts/agents/arm-supervision.sh"
watcher="$repo/scripts/watch-background-jobs.sh"
census="$repo/scripts/agents/process-census.py"
process_fixture=$repo/process-fixture.json
identity_fixture=$repo/process-identities.json
printf '[]\n' >"$process_fixture"
printf '{}\n' >"$identity_fixture"

# Census cost must be proportional to agent-shaped processes, not to the host
# process count. Feed one ps snapshot with 1,000 unrelated rows and two agent
# rows, then count the cwd resolver calls made by the real census pipeline.
python3 - "$census" "$repo" "$tmp/enumerate-filter-resolve.json" <<'PY'
import importlib.util
import os
import subprocess
import sys
from pathlib import Path

helper, repo, output = Path(sys.argv[1]), Path(sys.argv[2]), Path(sys.argv[3])
spec = importlib.util.spec_from_file_location("fixture_process_census", helper)
module = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = module
spec.loader.exec_module(module)
real_run = subprocess.run
rows = [
    f"{50000 + index} 1 {50000 + index} Mon Aug  4 12:34:56 2026 /usr/bin/non-agent-{index} --flag value"
    for index in range(1000)
]
rows.extend([
    "61001 1 61001 Mon Aug  4 12:34:56 2026 harness-fake-agent first",
    "61002 1 61002 Mon Aug  4 12:34:56 2026 /tool/harness-fake-agent second",
])
ps_output = "\n".join(rows) + "\n"

def fixture_run(command, *args, **kwargs):
    if command[:2] == ["ps", "-axo"]:
        return subprocess.CompletedProcess(command, 0, ps_output, "")
    return real_run(command, *args, **kwargs)

cwd_resolutions = []
module.subprocess.run = fixture_run
module.configured_signatures = lambda: {
    "fake": ([r"(^|[[:space:]/-])harness-fake-agent([[:space:]]|$)"], [], "fixture\n")
}
module.resolve_cwd = lambda pid: (cwd_resolutions.append(pid) or (str(repo), False))
module.live_custody = lambda: []
module.announcements = lambda fixture_by_pid, errors: []
os.environ.pop("HARNESS_CENSUS_PROCESS_FILE", None)
module.run_census(repo, "ordering-fixture", 60, output)
assert cwd_resolutions == [61001, 61002], cwd_resolutions
value = __import__("json").loads(output.read_text())
assert [item["pid"] for item in value["inventory"]] == [61001, 61002]
assert isinstance(value["durationMs"], int) and value["durationMs"] >= 0
PY
echo "enumerate-filter-resolve census fixture passed" >&2

export HARNESS_CENSUS_PROCESS_FILE="$process_fixture"
export HARNESS_FAKE_PROCESS_IDENTITY_FILE="$identity_fixture"
export HARNESS_WATCH_INTERVAL_SEC=1
export HARNESS_CENSUS_LOG_MAX_BYTES=350

# S4-7: all four adapters own a strict, line-oriented POSIX-ERE signature
# grammar. Exclude wins ties, malformed declarations fail the whole census,
# and lookalikes from each runtime stay out.
for runtime in claude codex devin fake; do
  adapter="$source_root/scripts/agents/adapters/$runtime.sh"
  signature=$($adapter signature)
  [[ -n "$signature" ]] || { echo "S4-7: $runtime returned no signatures" >&2; exit 1; }
  while IFS= read -r line; do
    [[ "$line" == match\ * || "$line" == exclude\ * ]] \
      || { echo "S4-7: malformed $runtime signature line: $line" >&2; exit 1; }
    printf '' | grep -Eq -- "${line#* }" || [[ $? -eq 1 ]] \
      || { echo "S4-7: invalid POSIX ERE from $runtime" >&2; exit 1; }
  done <<<"$signature"
  "$census" signature-check --adapter "$adapter" --positive "$runtime" \
    --lookalike "harness-${runtime}-lookalike" >/dev/null
done
for failure in malformed invalid-ere adapter-failed exclude-tie; do
  bad_adapter="$tmp/signature-$failure.sh"
  case "$failure" in
    malformed) printf '#!/usr/bin/env bash\nprintf "bogus .*\\n"\n' >"$bad_adapter" ;;
    invalid-ere) printf '#!/usr/bin/env bash\nprintf "match [\\n"\n' >"$bad_adapter" ;;
    adapter-failed) printf '#!/usr/bin/env bash\nexit 9\n' >"$bad_adapter" ;;
    exclude-tie) printf '#!/usr/bin/env bash\nprintf "match ^tie$\\nexclude ^tie$\\n"\n' >"$bad_adapter" ;;
  esac
  chmod +x "$bad_adapter"
  if "$census" signature-check --adapter "$bad_adapter" --positive tie \
      --lookalike lookalike >/dev/null 2>&1; then
    echo "S4-7: $failure signature adapter did not fail closed" >&2
    exit 1
  fi
done

# S4-8: omitted identity arguments have an executable rule. Run arming below a
# fake agent-signature ancestor and require inferred session/process identity.
cat >"$repo/harness-fake-agent" <<'SH'
#!/usr/bin/env bash
HARNESS_FAKE_AGENT_ANCESTOR_PID=$$ "$1" --repo "$2"
while [[ ! -e "$3" ]]; do sleep 0.05; done
SH
chmod +x "$repo/harness-fake-agent"
infer_release=$tmp/inferred-agent.release
HARNESS_SESSION_ID=inferred-session HARNESS_AGENT_RUNTIME=fake \
  "$repo/harness-fake-agent" "$arm" "$repo" "$infer_release" >"$tmp/inferred-arm.out" 2>&1 &
infer_driver=$!
infer_driver_start=$(process_started_at "$infer_driver")
owned_pids+=("$infer_driver:$infer_driver_start")
wait_until "S4-8 inferred announcement" bash -c 'compgen -G "$1/artifacts/agents/mains/inferred-session-*.json" >/dev/null' _ "$repo"
touch "$infer_release"
wait_for_child_exit "S4-8 inferred arming" "$infer_driver"
grep -Eq '(^|[[:space:]])ARMED repo=' "$tmp/inferred-arm.out" \
  || { cat "$tmp/inferred-arm.out" >&2; exit 1; }

last="$repo/artifacts/agents/supervision/last-census.json"
state="$repo/artifacts/agents/supervision/state.json"
wait_until "first complete census" test -s "$last"
[[ "$(json_field "$last" verdict)" == SUCCESS ]] || { echo "first census did not succeed" >&2; exit 1; }

# S4-5 and S4-10: the authoritative verdict is single-writer, schema-versioned,
# identifies its owner and instances, and the cross-component fields exist.
python3 - "$last" "$state" <<'PY'
import json, sys
last, state = (json.load(open(path)) for path in sys.argv[1:])
assert last["schemaVersion"] == 1 and last["writer"] == "watch-background-jobs.sh"
assert last["verdict"] in {"SUCCESS", "CENSUS-FAILED"}
assert isinstance(last["completedAtEpoch"], int) and isinstance(last["intervalSec"], int)
assert isinstance(last["durationMs"], int) and last["durationMs"] >= 0
assert isinstance(last["fingerprint"], str) and last["fingerprint"]
assert set(last["counts"]) == {"CUSTODY", "ANNOUNCED", "UNTRACKED"}
assert set(state["components"]) == {"watcher", "reaper"}
for component in state["components"].values():
    assert set(component) >= {"pid", "pidStartedAt", "instanceTag", "heartbeat"}
PY
if "$watcher" --dir "$repo/artifacts/agents/jobs" --scope "$repo" \
    --state "$tmp/second-writer.state" --interval 1 --once --census \
    --supervision-dir "$repo/artifacts/agents/supervision" \
    --heartbeat "$tmp/second-writer.heartbeat" --instance-tag second-writer \
    >"$tmp/second-writer.out" 2>&1; then
  echo "S4-5: a second live census writer was accepted" >&2
  exit 1
fi
grep -Fq 'live census writer already owns' "$tmp/second-writer.out" \
  || { echo "S4-5: duplicate writer refusal did not name the live owner" >&2; exit 1; }

# Duration is part of the authoritative artifact, and the watcher names an
# over-interval scan as a supervision defect instead of silently looping it.
warning_repo=$tmp/warning-repo
make_repo "$warning_repo"
perl -0pi -e 's/(^  signature\)\n)/$1    sleep 1.1\n/m' \
  "$warning_repo/scripts/agents/adapters/fake.sh"
warning_supervision=$warning_repo/artifacts/agents/supervision
warning_process_fixture=$warning_repo/processes.json
warning_identity_fixture=$warning_repo/identities.json
mkdir -p "$warning_supervision" "$warning_repo/artifacts/agents/jobs"
printf '[]\n' >"$warning_process_fixture"
printf '{}\n' >"$warning_identity_fixture"
touch "$warning_supervision/jobs.state"
python3 - "$warning_supervision/state.json" <<'PY'
import json, sys
value = {
    "owner": {"pid": 71001, "pidStartedAt": 1, "instanceTag": "warning-owner"},
    "components": {
        "watcher": {"pid": 71002, "pidStartedAt": 1, "instanceTag": "warning-watcher"},
        "reaper": {"pid": 71003, "pidStartedAt": 1, "instanceTag": "warning-reaper"},
    },
}
json.dump(value, open(sys.argv[1], "w"))
PY
HARNESS_CENSUS_PROCESS_FILE="$warning_process_fixture" \
HARNESS_FAKE_PROCESS_IDENTITY_FILE="$warning_identity_fixture" \
HARNESS_CENSUS_MAX_INTERVAL_SHARE_PERCENT=50 \
  "$warning_repo/scripts/watch-background-jobs.sh" \
    --dir "$warning_repo/artifacts/agents/jobs" --scope "$warning_repo" \
    --state "$warning_supervision/jobs.state" --interval 1 --once --census \
    --supervision-dir "$warning_supervision" \
    --heartbeat "$warning_supervision/watcher.heartbeat.json" \
    --instance-tag warning-fixture >"$tmp/slow-census.out" 2>&1
grep -Fq 'WARNING CENSUS-SLOW' "$tmp/slow-census.out" \
  && grep -Fq 'defect=scan-exceeds-interval' "$tmp/slow-census.out" \
  || { echo "slow census was not surfaced as a supervision defect" >&2; cat "$tmp/slow-census.out" >&2; exit 1; }
python3 - "$warning_supervision/last-census.json" <<'PY'
import json, sys
value = json.load(open(sys.argv[1]))
assert value["durationMs"] > value["intervalSec"] * 1000, value
PY

grep -Fq 'pidStartedAt' "$repo/scripts/agents/dispatch.sh" \
  || { echo "S4-1: job record does not carry pidStartedAt" >&2; exit 1; }
grep -Fq 'pidStartedAt' "$source_root/docs/orchestration.md" \
  || { echo "S4-1/S4-10: host-turn contract does not document pidStartedAt" >&2; exit 1; }
for owner_asset in \
  scripts/agents/dispatch.sh scripts/watch-background-jobs.sh \
  scripts/agents/adapters/runtime-common.sh scripts/agents/supervision-hook.sh \
  scripts/enforcement/claude-code-hooks.json scripts/enforcement/codex-hooks.json \
  scripts/enforcement/devin-hooks.json; do
  [[ -f "$source_root/$owner_asset" ]] \
    || { echo "S4-10: cross-component owner asset is missing: $owner_asset" >&2; exit 1; }
done

# Double arming joins the same repository set and duplicate starts collapse.
owner_before=$(json_field "$repo/artifacts/agents/supervision/lock.d/owner.json" pid)
owner_start=$(json_field "$repo/artifacts/agents/supervision/lock.d/owner.json" pidStartedAt)
"$arm" --repo "$repo" --session duplicate --pid "$$" \
  --start-time "$(process_started_at "$$")" --tag fixture-main >/dev/null
"$arm" --repo "$repo" --session duplicate --pid "$$" \
  --start-time "$(process_started_at "$$")" --tag fixture-main >/dev/null
owner_after=$(json_field "$repo/artifacts/agents/supervision/lock.d/owner.json" pid)
[[ "$owner_before" == "$owner_after" ]] || { echo "double arming replaced a live set" >&2; exit 1; }
[[ $(find "$repo/artifacts/agents/mains" -name 'duplicate-*.json' | wc -l | tr -d ' ') -eq 1 ]] \
  || { echo "duplicate session starts did not collapse" >&2; exit 1; }

# Complete three-class inventory, self-announcement before first census, stale
# custody rejection (S4-2), worktree scope inclusion, peer exclusion, embedded
# and not-yet-created argv paths (S4-9), and per-process unresolved cwd (S4-6).
mkdir -p "$repo/artifacts/agents/worktrees/w" "$tmp/peer"
now=$(date +%s)
raw_pid=41001 announced_pid=41002 custody_pid=41003 worktree_pid=41004 peer_pid=41005 argv_pid=41006 unresolved_pid=41007 relative_pid=41008
raw_start=$((now - 10)); announced_start=$((now - 9)); custody_start=$((now - 8)); worktree_start=$((now - 7)); peer_start=$((now - 6)); argv_start=$((now - 5)); unresolved_start=$((now - 4)); relative_start=$((now - 3))
write_process_fixture "$process_fixture" \
  "$raw_pid|1|$raw_pid|$raw_start|harness-fake-agent raw|$repo" \
  "$announced_pid|1|$announced_pid|$announced_start|harness-fake-agent announced|$repo" \
  "$custody_pid|1|$custody_pid|$custody_start|harness-fake-agent child --tag harness-job-owned|$repo" \
  "$worktree_pid|1|$worktree_pid|$worktree_start|harness-fake-agent worktree|$repo/artifacts/agents/worktrees/w" \
  "$peer_pid|1|$peer_pid|$peer_start|harness-fake-agent peer|$tmp/peer" \
  "$argv_pid|1|$argv_pid|$argv_start|harness-fake-agent --workspace=$repo/not-created/yet|$tmp/peer" \
  "$unresolved_pid|1|$unresolved_pid|$unresolved_start|harness-fake-agent --repo=$repo|UNRESOLVED" \
  "$relative_pid|1|$relative_pid|$relative_start|harness-fake-agent --workspace=../repo/not-created/relative|$tmp/peer"
mkdir -p "$repo/artifacts/agents/mains" "$repo/artifacts/agents/jobs"
python3 - "$repo/artifacts/agents/mains/announced-$announced_pid.json" "$announced_pid" "$announced_start" <<'PY'
import json, sys
from datetime import datetime, timezone
json.dump({"sessionId":"announced","pid":int(sys.argv[2]),"pidStartedAt":int(sys.argv[3]),"pgid":int(sys.argv[2]),"runtime":"fake","instanceTag":"main-announced","announcedAt":datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")}, open(sys.argv[1], "w"))
PY
supervisor_start=$(process_started_at "$$")
supervisor_pgid=$(python3 -c 'import os; print(os.getpgid(int(__import__("sys").argv[1])))' "$$")
mkdir -p "$repo/artifacts/agents/hb"
python3 - "$repo/artifacts/agents/jobs/owned.json" "$repo/artifacts/agents/hb/owned" "$repo" "$custody_pid" "$custody_start" "$$" "$supervisor_start" "$supervisor_pgid" <<'PY'
import json, sys
record,heartbeat,repo,child,child_start,supervisor,supervisor_start,supervisor_pgid=sys.argv[1:]
supervisor,supervisor_start,supervisor_pgid=int(supervisor),int(supervisor_start),int(supervisor_pgid)
tag="harness-job-owned"
json.dump({"jobId":"owned","status":"running","runtime":"fake","workspaceRoot":repo,"pid":supervisor,"pidStartedAt":supervisor_start,"pgid":supervisor_pgid,"instanceTag":tag,"ownershipProof":{"pid":supervisor,"pidStartedAt":supervisor_start,"pgid":supervisor_pgid,"instanceTag":tag},"startedAt":"2099-01-01T00:00:00Z","capMin":999999,"custodyProcesses":[{"pid":int(child),"pidStartedAt":int(child_start)-1,"instanceTag":tag}]}, open(record, "w"))
json.dump({"pid":supervisor,"instanceTag":tag},open(heartbeat,"w"))
PY
wait_until "three-class census" inventory_has "$last" ANNOUNCED "$announced_pid"
inventory_has "$last" UNTRACKED "$raw_pid"
inventory_has "$last" UNTRACKED "$custody_pid"
inventory_has "$last" UNTRACKED "$worktree_pid"
inventory_has "$last" UNTRACKED "$argv_pid"
inventory_has "$last" UNTRACKED "$unresolved_pid"
inventory_has "$last" UNTRACKED "$relative_pid"
if python3 - "$last" "$peer_pid" <<'PY'
import json, sys
v=json.load(open(sys.argv[1])); raise SystemExit(0 if any(str(x.get("pid"))==sys.argv[2] for x in v["inventory"]) else 1)
PY
then echo "peer repository process entered scope" >&2; exit 1; fi
grep -q "UNRESOLVED-CWD.*pid=$unresolved_pid" "$repo/artifacts/agents/supervision/census.log" \
  || { echo "S4-6: unresolved cwd was not surfaced per process" >&2; exit 1; }

# Correcting the child start alone must not gain custody while the third join
# key is wrong; only the exact pid+start+tag triple may classify the real CLI
# child as CUSTODY (S4-2).
python3 - "$repo/artifacts/agents/jobs/owned.json" "$custody_start" <<'PY'
import json, sys
p=sys.argv[1]; v=json.load(open(p)); v["custodyProcesses"][0]["pidStartedAt"]=int(sys.argv[2]); v["custodyProcesses"][0]["instanceTag"]="wrong-tag"; json.dump(v,open(p,"w"))
PY
previous_epoch=$(json_field "$last" completedAtEpoch)
wait_until "S4-2 wrong-tag census pass" bash -c '[[ $(python3 -c '\''import json,sys; print(json.load(open(sys.argv[1]))["completedAtEpoch"])'\'' "$1") -gt "$2" ]]' _ "$last" "$previous_epoch"
inventory_has "$last" UNTRACKED "$custody_pid" \
  || { echo "S4-2: wrong instanceTag gained custody" >&2; exit 1; }
python3 - "$repo/artifacts/agents/jobs/owned.json" <<'PY'
import json, sys
p=sys.argv[1]; v=json.load(open(p)); v["custodyProcesses"][0]["instanceTag"]=v["instanceTag"]; json.dump(v,open(p,"w"))
PY
wait_until "S4-2 child custody exact join" inventory_has "$last" CUSTODY "$custody_pid"

# CENSUS-FAILED covers total enumeration failure and unresolved scope, while a
# process-table race that has already exited is a named exclusion (S4-6).
printf '{broken\n' >"$process_fixture"
wait_until "S4-6 enumeration failure" bash -c '[[ $(python3 -c '\''import json,sys; print(json.load(open(sys.argv[1]))["verdict"])'\'' "$1" 2>/dev/null) == CENSUS-FAILED ]]' _ "$last"
wait_until "S4-6 enumeration surfaced" grep -q 'enumeration' \
  "$repo/artifacts/agents/supervision/census.log" "$repo/artifacts/agents/supervision/census.log.1"
printf '[]\n' >"$process_fixture"
wait_until "census recovery" bash -c '[[ $(python3 -c '\''import json,sys; print(json.load(open(sys.argv[1]))["verdict"])'\'' "$1") == SUCCESS ]]' _ "$last"

race_pid=41009
previous_epoch=$(json_field "$last" completedAtEpoch)
write_process_fixture "$process_fixture" "$race_pid|1|$race_pid|$now|harness-fake-agent raced|$repo"
python3 - "$process_fixture" <<'PY'
import json,sys
p=sys.argv[1]; v=json.load(open(p)); v[0]["alive"]=False; json.dump(v,open(p,"w"))
PY
wait_until "S4-6 raced-exit census" bash -c 'python3 - "$1" "$2" <<'\''PY'\''
import json,sys
v=json.load(open(sys.argv[1])); raise SystemExit(0 if v["completedAtEpoch"] > int(sys.argv[2]) and any(x.startswith("RACED-EXIT") for x in v["diagnostics"]) else 1)
PY' _ "$last" "$previous_epoch"
[[ "$(json_field "$last" verdict)" == SUCCESS ]] \
  || { echo "S4-6: exited process race failed the census" >&2; exit 1; }

bad_argv_pid=41010
write_process_fixture "$process_fixture" "$bad_argv_pid|1|$bad_argv_pid|$now|harness-fake-agent --workspace='|$repo"
wait_until "S4-6 unreadable argv" bash -c 'python3 - "$1" <<'\''PY'\''
import json,sys
v=json.load(open(sys.argv[1])); raise SystemExit(0 if v["verdict"]=="CENSUS-FAILED" and any(x.startswith("argv-unreadable:") for x in v["errors"]) else 1)
PY' _ "$last"

bad_start_pid=41011
write_process_fixture "$process_fixture" "$bad_start_pid|1|$bad_start_pid|-1|harness-fake-agent bad-start|$repo"
wait_until "S4-6 unreadable start time" bash -c 'python3 - "$1" <<'\''PY'\''
import json,sys
v=json.load(open(sys.argv[1])); raise SystemExit(0 if v["verdict"]=="CENSUS-FAILED" and any(x.startswith("start-time-unreadable:") for x in v["errors"]) else 1)
PY' _ "$last"
printf '[]\n' >"$process_fixture"
wait_until "S4-6 partial-failure recovery" bash -c '[[ $(python3 -c '\''import json,sys; print(json.load(open(sys.argv[1]))["verdict"])'\'' "$1") == SUCCESS ]]' _ "$last"

# Fingerprint includes scripts, signatures, relevant config, and the live set
# identities. Mutating a signature owner invalidates the old verdict (S4-3).
fingerprint_before=$(json_field "$last" fingerprint)
printf '\n# signature fingerprint fixture\n' >>"$repo/scripts/agents/adapters/fake.sh"
fingerprint_after=$($arm fingerprint --repo "$repo")
[[ "$fingerprint_before" != "$fingerprint_after" ]] \
  || { echo "S4-3: adapter change did not alter expected fingerprint" >&2; exit 1; }
git -C "$repo" show HEAD:scripts/agents/adapters/fake.sh >"$repo/scripts/agents/adapters/fake.sh.restored"
mv "$repo/scripts/agents/adapters/fake.sh.restored" "$repo/scripts/agents/adapters/fake.sh"
chmod +x "$repo/scripts/agents/adapters/fake.sh"

# Dispatch refuses absent, stale, failed, and fingerprint-mismatched verdicts.
gate_repo=$tmp/gate-repo
mkdir -p "$gate_repo"
make_repo "$gate_repo"
brief=$gate_repo/brief.md
sed 's/^Working Mode:.*/Working Mode: design/' "$gate_repo/scripts/agents/templates/brief.md" >"$brief"
"$gate_repo/scripts/agents/adapters/fake.sh" probe >/dev/null
dispatch_fails() { # name, expected
  local name=$1 expected=$2
  set +e
  "$gate_repo/scripts/agents/dispatch.sh" dispatch --role design-critic --brief "$brief" --job-id "$name" >"$tmp/$name.out" 2>&1
  rc=$?
  set -e
  [[ $rc -ne 0 ]] || { echo "dispatch accepted $name census" >&2; exit 1; }
  grep -Fq "$expected" "$tmp/$name.out" || { cat "$tmp/$name.out" >&2; exit 1; }
}
dispatch_fails no-census 'census verdict is absent'
mkdir -p "$gate_repo/artifacts/agents/supervision"
cp "$state" "$gate_repo/artifacts/agents/supervision/state.json"
cp "$last" "$gate_repo/artifacts/agents/supervision/last-census.json"
gate_fingerprint=$($gate_repo/scripts/agents/arm-supervision.sh fingerprint --repo "$gate_repo")
cp "$gate_repo/artifacts/agents/supervision/state.json" "$tmp/gate-state.json"
perl -0pi -e 's/^watch\.stale-min=.*$/watch.stale-min=21/m' "$gate_repo/harness.conf"
[[ "$($gate_repo/scripts/agents/arm-supervision.sh fingerprint --repo "$gate_repo")" != "$gate_fingerprint" ]] \
  || { echo "S4-3: relevant configuration did not alter the fingerprint" >&2; exit 1; }
git -C "$gate_repo" show HEAD:harness.conf >"$gate_repo/harness.conf"
python3 - "$gate_repo/artifacts/agents/supervision/state.json" <<'PY'
import json,sys
p=sys.argv[1]; v=json.load(open(p)); v["owner"]["instanceTag"] += "-changed"; json.dump(v,open(p,"w"))
PY
[[ "$($gate_repo/scripts/agents/arm-supervision.sh fingerprint --repo "$gate_repo")" != "$gate_fingerprint" ]] \
  || { echo "S4-3: supervisor instance identity did not alter the fingerprint" >&2; exit 1; }
cp "$tmp/gate-state.json" "$gate_repo/artifacts/agents/supervision/state.json"
printf '\n# watcher fingerprint fixture\n' >>"$gate_repo/scripts/watch-background-jobs.sh"
[[ "$($gate_repo/scripts/agents/arm-supervision.sh fingerprint --repo "$gate_repo")" != "$gate_fingerprint" ]] \
  || { echo "S4-3: watcher code did not alter the fingerprint" >&2; exit 1; }
git -C "$gate_repo" show HEAD:scripts/watch-background-jobs.sh >"$gate_repo/scripts/watch-background-jobs.sh"
chmod +x "$gate_repo/scripts/watch-background-jobs.sh"
python3 - "$gate_repo/artifacts/agents/supervision/last-census.json" <<'PY'
import json, sys, time
p=sys.argv[1]; v=json.load(open(p)); v["completedAtEpoch"]=int(time.time())-100; json.dump(v,open(p,"w"))
PY
dispatch_fails stale-census 'census verdict is stale'
python3 - "$gate_repo/artifacts/agents/supervision/last-census.json" <<'PY'
import json, sys, time
p=sys.argv[1]; v=json.load(open(p)); v.update({"completedAtEpoch":int(time.time()),"verdict":"CENSUS-FAILED"}); json.dump(v,open(p,"w"))
PY
dispatch_fails failed-census 'CENSUS-FAILED'
python3 - "$gate_repo/artifacts/agents/supervision/last-census.json" <<'PY'
import json, sys, time
p=sys.argv[1]; v=json.load(open(p)); v.update({"completedAtEpoch":int(time.time()),"verdict":"SUCCESS","fingerprint":"wrong"}); json.dump(v,open(p,"w"))
PY
dispatch_fails fingerprint-census 'fingerprint does not match'
python3 - "$gate_repo/artifacts/agents/supervision/state.json" <<'PY'
import json,sys
p=sys.argv[1]; v=json.load(open(p)); v["owner"]["pid"]=999999; v["owner"]["pidStartedAt"]=1; json.dump(v,open(p,"w"))
PY
printf '{"session_id":"stale-surface","cwd":"%s","hook_event_name":"Stop"}\n' "$gate_repo" \
  | "$gate_repo/scripts/agents/supervision-hook.sh" fake stop >"$tmp/stale-surface.out"
grep -Fq 'STALE-SUPERVISOR census fingerprint' "$tmp/stale-surface.out" \
  || { echo "S4-3/S4-4: end-turn hook hid fingerprint drift" >&2; exit 1; }
grep -Fq 'STALE-SUPERVISOR component=owner' "$tmp/stale-surface.out" \
  || { echo "S4-4: end-turn hook hid a dead supervision owner" >&2; exit 1; }

# Continuous supervision has one owner and one cadence. Killing either
# component is detected and the whole instance-fingerprinted set is replaced
# (S4-4); a proven-dead lock owner is taken over.
component_replaced() { # component, old pid
  [[ "$(json_field "$state" "components.$1.pid")" != "$2" ]]
}
watcher_pid=$(json_field "$state" components.watcher.pid)
watcher_start=$(json_field "$state" components.watcher.pidStartedAt)
reaper_before_watcher_kill=$(json_field "$state" components.reaper.pid)
stop_owned_pid "S4-4 watcher" "$watcher_pid" "$watcher_start"
wait_until "S4-4 watcher recovery" component_replaced watcher "$watcher_pid"
wait_until "S4-4 watcher recovery replaces set" component_replaced reaper "$reaper_before_watcher_kill"
grep -q 'STALE-SUPERVISOR component=watcher' "$repo/artifacts/agents/supervision/supervisor.log" \
  || { echo "watcher death was not surfaced" >&2; exit 1; }
reaper_pid=$(json_field "$state" components.reaper.pid)
reaper_start=$(json_field "$state" components.reaper.pidStartedAt)
watcher_before_reaper_kill=$(json_field "$state" components.watcher.pid)
stop_owned_pid "S4-4 reaper" "$reaper_pid" "$reaper_start"
wait_until "S4-4 reaper recovery" component_replaced reaper "$reaper_pid"
wait_until "S4-4 reaper recovery replaces set" component_replaced watcher "$watcher_before_reaper_kill"
grep -q 'STALE-SUPERVISOR component=reaper' "$repo/artifacts/agents/supervision/supervisor.log" \
  || { echo "reaper death was not surfaced" >&2; exit 1; }

stop_owned_pid "supervision owner for takeover" "$owner_before" "$owner_start"
wait_until "proven-dead owner" bash -c '! "$1" alive --pid "$2" --start-time "$3" >/dev/null 2>&1' _ "$census" "$owner_before" "$owner_start"
printf '[]\n' >"$process_fixture"
"$arm" --repo "$repo" --session takeover --pid "$$" --start-time "$(process_started_at "$$")" --tag takeover-main >/dev/null
new_owner=$(json_field "$repo/artifacts/agents/supervision/lock.d/owner.json" pid)
[[ "$new_owner" != "$owner_before" ]] || { echo "stale supervision lock was not taken over" >&2; exit 1; }

# Dead announcements are pruned; SessionEnd retires its own; log rotation and
# end-of-turn UNTRACKED/stale-supervisor surfacing remain visible.
dead_pid=$(python3 - <<'PY'
import subprocess,sys
p=subprocess.Popen(
    [sys.executable,"-c","import time; time.sleep(30)"],
    stdin=subprocess.DEVNULL,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,
    start_new_session=True,
)
print(p.pid)
PY
)
dead_start=$(process_started_at "$dead_pid")
owned_pids+=("$dead_pid:$dead_start")
"$arm" --repo "$repo" --session dead-main --pid "$dead_pid" --start-time "$dead_start" --tag dead-main >/dev/null
stop_owned_pid "dead announcement" "$dead_pid" "$dead_start"
wait_until "dead announcement pruning" test ! -e "$repo/artifacts/agents/mains/dead-main-$dead_pid.json"

write_process_fixture "$process_fixture" "$raw_pid|1|$raw_pid|$raw_start|harness-fake-agent raw|$repo"
wait_until "UNTRACKED end-turn source" inventory_has "$last" UNTRACKED "$raw_pid"
printf '{"session_id":"surface","cwd":"%s","hook_event_name":"Stop"}\n' "$repo" \
  | "$repo/scripts/agents/supervision-hook.sh" fake stop >"$tmp/surface.out"
grep -q 'UNTRACKED' "$tmp/surface.out" || { echo "end-of-turn hook hid UNTRACKED" >&2; exit 1; }
wait_until "census log rotation" test -f "$repo/artifacts/agents/supervision/census.log.1"

# The arming event log proves write-announcement precedes the first census and
# therefore never labels the arming session UNTRACKED.
python3 - "$repo/artifacts/agents/supervision/arming.log" <<'PY'
import sys
lines=open(sys.argv[1]).read().splitlines()
announce=min(i for i,x in enumerate(lines) if "announcement-written" in x)
census=min(i for i,x in enumerate(lines) if "first-census-complete" in x)
assert announce < census
PY

# S4-11: a sandbox must never stop a supervisor another checkout armed. The
# operator sandbox above is built without artifacts/ for exactly this reason;
# both halves are asserted, because either one alone leaves the suite able to
# disarm the machine it runs on.
[[ ! -e "$operator_harness/artifacts/agents/supervision/lock.d/owner.json" ]] \
  || { echo "operator sandbox carries the operator's supervision lock" >&2; exit 1; }

foreign=$tmp/foreign-owner
mkdir -p "$foreign/repo"
(cd "$foreign/repo" && git init -q .)
mkdir -p "$foreign/repo/harness/scripts/agents" "$foreign/repo/harness/artifacts/agents/supervision/lock.d"
cp "$source_root/scripts/agents/arm-supervision.sh" "$foreign/repo/harness/scripts/agents/"
cp "$source_root/scripts/agents/process-census.py" "$foreign/repo/harness/scripts/agents/"
foreign_sleep_pid=$(
  bash -c 'exec -a harness-foreign-owner sleep 120 >/dev/null 2>&1 & echo $!'
)
foreign_start=$(process_started_at "$foreign_sleep_pid")
owned_pids+=("$foreign_sleep_pid:$foreign_start")
python3 - "$foreign/repo/harness/artifacts/agents/supervision/lock.d/owner.json" \
  "$foreign_sleep_pid" "$foreign_start" <<'PY'
import json, sys
from pathlib import Path
Path(sys.argv[1]).write_text(json.dumps({
  "pid": int(sys.argv[2]), "pidStartedAt": int(sys.argv[3]),
  "instanceTag": "harness-supervision-owner-some-other-checkout-1-2",
  "acquiredAt": "1970-01-01T00:00:00Z",
}) + "\n")
PY
set +e
"$foreign/repo/harness/scripts/agents/arm-supervision.sh" --repo "$foreign/repo" --shutdown \
  >"$tmp/foreign-shutdown.out" 2>&1
foreign_status=$?
set -e
(( foreign_status != 0 )) \
  || { echo "shutdown accepted a lock armed for another repository" >&2; exit 1; }
grep -Fq 'another repository' "$tmp/foreign-shutdown.out" \
  || { echo "foreign-owner refusal did not name the cause" >&2; cat "$tmp/foreign-shutdown.out" >&2; exit 1; }
kill -0 "$foreign_sleep_pid" 2>/dev/null \
  || { echo "shutdown stopped a process another checkout owned" >&2; exit 1; }
stop_owned_pid "foreign owner" "$foreign_sleep_pid" "$foreign_start" >/dev/null 2>&1 || true

echo "supervision fixtures passed (S4-1 through S4-11)"
