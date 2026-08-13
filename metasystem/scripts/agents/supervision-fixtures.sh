#!/usr/bin/env bash
set -euo pipefail

# Focused fixtures for section 3.11. Every wait in this file goes through a
# named ceiling so a broken supervisor fails loudly instead of hanging (IL-1).

source_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
source "$source_root/scripts/agents/fixture-budget.sh"
harness_fixture_budget_init "$source_root"
fixture_ceiling_sec=$(harness_fixture_cap supervision-wait)

tmp=$(mktemp -d)
owned_pids=()
fixture_harness_roots=()

ms="${METASYSTEM_BIN:-$source_root/bin/metasystem}"
[[ -x "$ms" ]] || { echo "supervision fixtures: binary absent; run the go gate first" >&2; exit 1; }

process_started_at() {
  "$ms" proc started-at --pid "$1"
}

process_identity_alive() { # pid, start
  "$ms" proc alive --pid "$1" --start-time "$2" >/dev/null 2>&1
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
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
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
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
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
  if [[ -n "${METASYSTEM_KEEP_SUPERVISION_FIXTURE:-}" ]]; then
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
    "$source_root/scripts/metasystem-config.sh" "$source_root/metasystem.conf" "$repo/scripts/../" 2>/dev/null || true
  # The previous copy places metasystem.conf under scripts/; put each asset at its
  # actual shipped path explicitly.
  cp "$source_root/scripts/watch-background-jobs.sh" "$repo/scripts/"
  cp "$source_root/scripts/metasystem-config.sh" "$repo/scripts/"
  cp "$source_root/metasystem.conf" "$repo/metasystem.conf"
  perl -0pi -e 's/^metasystem\.runtimes=.*$/metasystem.runtimes=fake/m; s|^evidence\.root=.*$|evidence.root='"$evidence"'|m; s/^watch\.interval-sec=.*$/watch.interval-sec=1/m; s/^census\.log-max-bytes=.*$/census.log-max-bytes=350/m; s/^model\.tier\.1=.*$/model.tier.1=fake:fake-model/m; s/^model\.tier\.2=.*$/model.tier.2=/m; s/^model\.tier\.3=.*$/model.tier.3=/m; s/^role\.default\.runtime=.*$/role.default.runtime=fake/m; s/^role\.default\.model\.codex=.*$/role.default.model.fake=fake-model/m; s/^role\.default\.model\.(?:claude|devin)=.*\n//mg; s/\.runtime=(?:codex|devin)$/\.runtime=fake/mg; s/\.model\.(?:codex|devin)=.*$/\.model.fake=fake-model/mg' "$repo/metasystem.conf"
  grep -q '^watch\.interval-sec=' "$repo/metasystem.conf" || printf 'watch.interval-sec=1\n' >>"$repo/metasystem.conf"
  grep -q '^census\.log-max-bytes=' "$repo/metasystem.conf" || printf 'census.log-max-bytes=350\n' >>"$repo/metasystem.conf"
  git -C "$repo" init -q -b main
  git -C "$repo" add .
  git -C "$repo" -c user.name=metasystem -c user.email=metasystem.invalid commit -qm fixture
  # Stage the engine the way production ships it: an untracked build artifact
  # at <repo>/bin/metasystem, added after the base commit.
  mkdir -p "$repo/bin"
  cp "$ms" "$repo/bin/metasystem"
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

# The ordinary operator layout keeps the vendored metasystem one directory below
# the Git toplevel. Its primary branch uses the shipped configuration and real
# process sources: no fake census table, fake identity table, or rewritten
# metasystem.conf. Restricted hosts keep the same path regression through the
# existing fake-process fallback below.
operator_scope=$tmp/operator-repo
operator_harness=$operator_scope/metasystem
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
git -C "$operator_scope" init -q -b main
git -C "$operator_scope" add metasystem
git -C "$operator_scope" -c user.name=metasystem -c user.email=metasystem.invalid commit -qm fixture
fixture_harness_roots+=("$operator_harness")
operator_arm=$operator_harness/scripts/agents/arm-supervision.sh
operator_engine=$operator_harness/bin/metasystem

# A plain shell with no matching ancestor must refuse, but the refusal tells an
# operator both supported ways forward. Restricting discovery to the fake
# signature makes this deterministic without replacing any process source.
set +e
(
  cd "$operator_harness"
  METASYSTEM_AGENT_RUNTIME=fake "$operator_arm" --repo "$PWD"
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

operator_env=(env -u METASYSTEM_CENSUS_PROCESS_FILE -u METASYSTEM_FAKE_PROCESS_IDENTITY_FILE
  -u METASYSTEM_FAKE_AGENT_ANCESTOR_PID -u METASYSTEM_AGENT_RUNTIME
  -u METASYSTEM_SESSION_ID -u METASYSTEM_INSTANCE_TAG -u METASYSTEM_WATCH_INTERVAL_SEC
  -u METASYSTEM_CENSUS_LOG_MAX_BYTES -u METASYSTEM_CENSUS_MAX_INTERVAL_SHARE_PERCENT)
if [[ -n "${METASYSTEM_SUPERVISION_OPERATOR_FIXTURE_FAKE:-}" ]] \
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
  perl -0pi -e 's/^metasystem\.runtimes=.*$/metasystem.runtimes=fake/m; s/^watch\.interval-sec=.*$/watch.interval-sec=1/m' "$operator_harness/metasystem.conf"
  operator_env=(env METASYSTEM_CENSUS_PROCESS_FILE="$operator_process_fixture"
    METASYSTEM_FAKE_PROCESS_IDENTITY_FILE="$operator_identity_fixture")
fi
operator_start=$("${operator_env[@]}" "$operator_engine" proc started-at --pid "$$")
(
  cd "$operator_harness"
  "${operator_env[@]}" \
    "$operator_arm" --repo "$PWD" --session operator-path --pid "$$" \
      --start-time "$operator_start" --tag operator-path
) >"$tmp/operator-arm.out" 2>&1 &
operator_driver=$!
operator_driver_start=$("${operator_env[@]}" "$operator_engine" proc started-at --pid "$operator_driver")
owned_pids+=("$operator_driver:$operator_driver_start")
wait_for_child_exit "nested ordinary operator arming" "$operator_driver"
grep -Eq "(^|[[:space:]])ARMED repo=$operator_scope([[:space:]]|$)" "$tmp/operator-arm.out" \
  || { cat "$tmp/operator-arm.out" >&2; exit 1; }
[[ -s "$operator_harness/artifacts/agents/supervision/last-census.json" ]] \
  || { echo "nested operator path did not write the metasystem registry" >&2; exit 1; }
[[ ! -e "$operator_scope/artifacts" ]] \
  || { echo "nested operator path wrote registries at the Git toplevel" >&2; exit 1; }
if grep -Eq 'metasystem\.runtimes has no signature adapters|No such file or directory' "$tmp/operator-arm.out"; then
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
  echo "nested operator hook configuration still resolves metasystem scripts from the Git toplevel" >&2
  exit 1
fi
"${operator_env[@]}" "$operator_arm" --repo "$operator_harness" --shutdown >/dev/null 2>&1

echo "nested ordinary operator supervision fixture passed" >&2

repo=$tmp/repo
mkdir -p "$repo"
make_repo "$repo"
arm="$repo/scripts/agents/arm-supervision.sh"
# One writer per checkout. Phases that arm a DIFFERENT main than the phase
# before them release the checkout first, the way a departing main does.
# Supervision components never hold the lease, so dropping the lease record
# is a complete release and leaves the running supervision set untouched.
release_checkout() { # repo
  rm -f "$1/artifacts/agents/mains/worktree-lease.json" \
        "$1/artifacts/agents/mains/reaped-after-claim.json"
}

# Control-plane writes belong to the checkout's announced holder, so a fixture
# that drives one must BE a main rather than borrow whatever class its ambient
# ancestry happens to produce. Ambient class is not stable: the same phase
# classifies as HUMAN from a terminal, as DELEGATE under an agent that runs the
# suite, and as DELEGATE again whenever a census fixture puts a simulated agent
# command on a real ancestor's process identifier. Announcing this shell claims
# the checkout as a side effect, which is what a starting main does.
become_main() { # repo, session
  local repo=$1 session=$2
  "$repo/bin/metasystem" lease announce --root "$repo" \
    --session "$session" --pid $$ --start "$(process_started_at $$)" \
    --tag "fixture-$session" --runtime fake >/dev/null
}
watcher="$repo/scripts/watch-background-jobs.sh"
census_engine="$repo/bin/metasystem"
process_fixture=$repo/process-fixture.json
identity_fixture=$repo/process-identities.json
printf '[]\n' >"$process_fixture"
printf '{}\n' >"$identity_fixture"

# Census cost must be proportional to agent-shaped processes, not to the host
# process count. The retired python leg counted the module's cwd-resolver
# calls under a stubbed ps; the engine enforces the same rule structurally
# (internal/census/production.go resolves cwds only for MATCHED processes).
# What stays observable from outside is the classification boundary: feed
# 1,000 unrelated rows and two agent rows through the fixture enumeration
# and the inventory must contain exactly the two agent processes.
python3 - "$repo" "$tmp/enumerate-filter-resolve-procs.json" <<'PY'
import json, sys
repo, output = sys.argv[1], sys.argv[2]
rows = [
    {"pid": 50000 + index, "ppid": 1, "pgid": 50000 + index, "pidStartedAt": 100,
     "argv": f"/usr/bin/non-agent-{index} --flag value", "cwd": repo, "cwdError": False, "alive": True}
    for index in range(1000)
]
rows.append({"pid": 61001, "ppid": 1, "pgid": 61001, "pidStartedAt": 100,
             "argv": "metasystem-fake-agent first", "cwd": repo, "cwdError": False, "alive": True})
rows.append({"pid": 61002, "ppid": 1, "pgid": 61002, "pidStartedAt": 100,
             "argv": "/tool/metasystem-fake-agent second", "cwd": repo, "cwdError": False, "alive": True})
json.dump(rows, open(output, "w"))
PY
METASYSTEM_CENSUS_PROCESS_FILE="$tmp/enumerate-filter-resolve-procs.json" \
  "$census_engine" proc census --root "$repo" --repo "$repo" \
  --fingerprint ordering-fixture --interval 60 \
  --output "$tmp/enumerate-filter-resolve.json" >/dev/null
python3 - "$tmp/enumerate-filter-resolve.json" <<'PY'
import json, sys
value = json.load(open(sys.argv[1]))
assert [item["pid"] for item in value["inventory"]] == [61001, 61002], value["inventory"]
assert isinstance(value["durationMs"], int) and value["durationMs"] >= 0
PY
echo "enumerate-filter-resolve census fixture passed" >&2

export METASYSTEM_CENSUS_PROCESS_FILE="$process_fixture"
export METASYSTEM_FAKE_PROCESS_IDENTITY_FILE="$identity_fixture"
export METASYSTEM_WATCH_INTERVAL_SEC=1
export METASYSTEM_CENSUS_LOG_MAX_BYTES=350

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
  positive=$runtime
  # The real CLIs are invoked by their bare command word, so that word is the
  # honest positive for claude, codex and devin. The fake runtime has no CLI:
  # its processes carry the fixture name, and a bare-word positive was exactly
  # the looseness that let commit-message prose in a tool shell match (KI-14).
  [[ "$runtime" == fake ]] && positive=metasystem-fake-agent
  "$census_engine" proc signature-check --adapter "$adapter" --positive "$positive" \
    --lookalike "metasystem-${runtime}-lookalike" >/dev/null
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
  if "$census_engine" proc signature-check --adapter "$bad_adapter" --positive tie \
      --lookalike lookalike >/dev/null 2>&1; then
    echo "S4-7: $failure signature adapter did not fail closed" >&2
    exit 1
  fi
done

# S4-8: omitted identity arguments have an executable rule. Run arming below a
# fake agent-signature ancestor and require inferred session/process identity.
cat >"$repo/metasystem-fake-agent" <<'SH'
#!/usr/bin/env bash
METASYSTEM_FAKE_AGENT_ANCESTOR_PID=$$ "$1" --repo "$2"
while [[ ! -e "$3" ]]; do sleep "${METASYSTEM_FIXTURE_POLL_INTERVAL_SEC:?}"; done
SH
chmod +x "$repo/metasystem-fake-agent"
infer_release=$tmp/inferred-agent.release
# One writer per checkout: this phase arms a different main than the phase
# before it, so it releases the checkout first the way a departing main does.
# Supervision components never hold the lease, so removing the lease record
# once supervision is stopped is a complete release.
release_checkout "$repo"
METASYSTEM_SESSION_ID=inferred-session METASYSTEM_AGENT_RUNTIME=fake \
  "$repo/metasystem-fake-agent" "$arm" "$repo" "$infer_release" >"$tmp/inferred-arm.out" 2>&1 &
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
import hashlib, json, sys
from pathlib import Path
last = json.loads(Path(sys.argv[1]).read_text())
state_bytes = Path(sys.argv[2]).read_bytes()
state = json.loads(state_bytes)
assert last["schemaVersion"] == 2 and last["writer"] == "watch-background-jobs.sh"
assert last["verdict"] in {"SUCCESS", "CENSUS-FAILED"}
assert isinstance(last["completedAtEpoch"], int) and isinstance(last["intervalSec"], int)
assert isinstance(last["durationMs"], int) and last["durationMs"] >= 0
assert isinstance(last["fingerprint"], str) and last["fingerprint"]
assert last["generation"] == state["generation"]
assert last["stateDigest"] == hashlib.sha256(state_bytes).hexdigest()
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
  || { echo "S4-5: duplicate writer refusal did not name the live owner" >&2
       echo "--- second writer said:" >&2; cat "$tmp/second-writer.out" >&2
       echo "--- lock state:" >&2; ls -la "$repo/artifacts/agents/supervision/census-writer.d" >&2 2>/dev/null || true
       cat "$repo/artifacts/agents/supervision/census-writer.d/owner.json" >&2 2>/dev/null || echo "(no owner.json)" >&2
       exit 1; }

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
METASYSTEM_CENSUS_PROCESS_FILE="$warning_process_fixture" \
METASYSTEM_FAKE_PROCESS_IDENTITY_FILE="$warning_identity_fixture" \
METASYSTEM_CENSUS_MAX_INTERVAL_SHARE_PERCENT=50 \
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
release_checkout "$repo"
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
  "$raw_pid|1|$raw_pid|$raw_start|metasystem-fake-agent raw|$repo" \
  "$announced_pid|1|$announced_pid|$announced_start|metasystem-fake-agent announced|$repo" \
  "$custody_pid|1|$custody_pid|$custody_start|metasystem-fake-agent child --tag metasystem-job-owned|$repo" \
  "$worktree_pid|1|$worktree_pid|$worktree_start|metasystem-fake-agent worktree|$repo/artifacts/agents/worktrees/w" \
  "$peer_pid|1|$peer_pid|$peer_start|metasystem-fake-agent peer|$tmp/peer" \
  "$argv_pid|1|$argv_pid|$argv_start|metasystem-fake-agent --workspace=$repo/not-created/yet|$tmp/peer" \
  "$unresolved_pid|1|$unresolved_pid|$unresolved_start|metasystem-fake-agent --repo=$repo|UNRESOLVED" \
  "$relative_pid|1|$relative_pid|$relative_start|metasystem-fake-agent --workspace=../repo/not-created/relative|$tmp/peer"
mkdir -p "$repo/artifacts/agents/mains" "$repo/artifacts/agents/jobs"
python3 - "$repo/artifacts/agents/mains/announced-$announced_pid.json" "$announced_pid" "$announced_start" <<'PY'
import json, sys
from datetime import datetime, timezone
json.dump({"sessionId":"announced","pid":int(sys.argv[2]),"pidStartedAt":int(sys.argv[3]),"pgid":int(sys.argv[2]),"runtime":"fake","instanceTag":"main-announced","announcedAt":datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")}, open(sys.argv[1], "w"))
PY
supervisor_start=$(process_started_at "$$")
supervisor_pgid=$(python3 -c 'import os; print(os.getpgid(int(__import__("sys").argv[1])))' "$$")
mkdir -p "$repo/artifacts/agents/hb"
python3 - "$repo/artifacts/agents/jobs/owned.json" "$repo/artifacts/agents/hb/owned" "$repo" \
  "$custody_pid" "$custody_start" "$$" "$supervisor_start" "$supervisor_pgid" \
  "$(harness_fixture_semantic_cap dormant-job-minutes)" <<'PY'
import json, sys
record,heartbeat,repo,child,child_start,supervisor,supervisor_start,supervisor_pgid,cap_min=sys.argv[1:]
supervisor,supervisor_start,supervisor_pgid=int(supervisor),int(supervisor_start),int(supervisor_pgid)
tag="metasystem-job-owned"
json.dump({"jobId":"owned","status":"running","runtime":"fake","workspaceRoot":repo,"pid":supervisor,"pidStartedAt":supervisor_start,"pgid":supervisor_pgid,"instanceTag":tag,"ownershipProof":{"pid":supervisor,"pidStartedAt":supervisor_start,"pgid":supervisor_pgid,"instanceTag":tag},"startedAt":"2099-01-01T00:00:00Z","capMin":int(cap_min),"custodyProcesses":[{"pid":int(child),"pidStartedAt":int(child_start)-1,"instanceTag":tag}]}, open(record, "w"))
json.dump({"pid":supervisor,"instanceTag":tag},open(heartbeat,"w"))
PY
# The reaper proves the record's custodian (this shell) by pid+start+tag.
# This shell's real command line does not carry the job tag, so the fixture
# identity source supplies it — the same one-source override the census
# uses; kernel death still vetoes it.
python3 - "$identity_fixture" "$$" "$supervisor_start" <<'PY'
import json, sys
from pathlib import Path
path, pid, started = Path(sys.argv[1]), sys.argv[2], int(sys.argv[3])
value = json.loads(path.read_text())
value[pid] = {"pidStartedAt": started,
              "command": "fixture-supervisor metasystem-job-owned"}
path.write_text(json.dumps(value) + "\n")
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
# The engine surfaces the per-process unresolved cwd in the verdict's
# diagnostics (the shell watcher's census.log transcript retired with it).
python3 - "$last" "$unresolved_pid" <<'PY' \
  || { echo "S4-6: unresolved cwd was not surfaced per process" >&2; exit 1; }
import json, sys
value = json.load(open(sys.argv[1]))
needle = f"UNRESOLVED-CWD pid={sys.argv[2]}"
raise SystemExit(0 if any(needle in item for item in value.get("diagnostics", [])) else 1)
PY

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
# The engine surfaces the enumeration failure in the verdict's errors
# (the shell watcher's census.log transcript retired with it).
wait_until "S4-6 enumeration surfaced" bash -c \
  '[[ -n $(python3 -c '\''import json,sys; print("".join(e for e in json.load(open(sys.argv[1])).get("errors",[]) if "enumeration" in e))'\'' "$1" 2>/dev/null) ]]' _ "$last"
printf '[]\n' >"$process_fixture"
wait_until "census recovery" bash -c '[[ $(python3 -c '\''import json,sys; print(json.load(open(sys.argv[1]))["verdict"])'\'' "$1") == SUCCESS ]]' _ "$last"

race_pid=41009
previous_epoch=$(json_field "$last" completedAtEpoch)
write_process_fixture "$process_fixture" "$race_pid|1|$race_pid|$now|metasystem-fake-agent raced|$repo"
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
write_process_fixture "$process_fixture" "$bad_argv_pid|1|$bad_argv_pid|$now|metasystem-fake-agent --workspace='|$repo"
wait_until "S4-6 unreadable argv" bash -c 'python3 - "$1" <<'\''PY'\''
import json,sys
v=json.load(open(sys.argv[1])); raise SystemExit(0 if v["verdict"]=="CENSUS-FAILED" and any(x.startswith("argv-unreadable:") for x in v["errors"]) else 1)
PY' _ "$last"

bad_start_pid=41011
write_process_fixture "$process_fixture" "$bad_start_pid|1|$bad_start_pid|-1|metasystem-fake-agent bad-start|$repo"
wait_until "S4-6 unreadable start time" bash -c 'python3 - "$1" <<'\''PY'\''
import json,sys
v=json.load(open(sys.argv[1])); raise SystemExit(0 if v["verdict"]=="CENSUS-FAILED" and any(x.startswith("start-time-unreadable:") for x in v["errors"]) else 1)
PY' _ "$last"
printf '[]\n' >"$process_fixture"
wait_until "S4-6 partial-failure recovery" bash -c '[[ $(python3 -c '\''import json,sys; print(json.load(open(sys.argv[1]))["verdict"])'\'' "$1") == SUCCESS ]]' _ "$last"

# Fingerprint includes scripts, signatures, and relevant config. Mutating a
# static identity owner invalidates the old verdict (S4-3).
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
# The refusal message names the resolved path, and on macOS the temp
# directory is reached through a symlink, so the expectation must resolve too.
gate_repo_real=$gate_repo
mkdir -p "$gate_repo"
make_repo "$gate_repo"
brief=$gate_repo/brief.md
sed 's/^Working Mode:.*/Working Mode: design/' "$gate_repo/scripts/agents/templates/brief.md" >"$brief"
"$gate_repo/scripts/agents/adapters/fake.sh" probe >/dev/null
become_main "$gate_repo" gate-session
dispatch_fails() { # name, expected
  local name=$1 expected=$2
  set +e
  "$gate_repo/scripts/agents/dispatch.sh" dispatch --role design-critic --brief "$brief" --job-id "$name" >"$tmp/$name.out" 2>&1
  rc=$?
  set -e
  [[ $rc -ne 0 ]] || { echo "dispatch accepted $name census" >&2; exit 1; }
  grep -Fq "$expected" "$tmp/$name.out" || { cat "$tmp/$name.out" >&2; exit 1; }
}
dispatch_succeeds() { # name
  local name=$1
  "$gate_repo/scripts/agents/dispatch.sh" dispatch --role design-critic --brief "$brief" --job-id "$name" \
    >"$tmp/$name.out" 2>&1 \
    || { echo "dispatch refused $name census" >&2; cat "$tmp/$name.out" >&2; exit 1; }
}
set_gate_census() { # age, interval, fingerprint
  python3 - "$gate_repo/artifacts/agents/supervision/last-census.json" \
    "$gate_repo/artifacts/agents/supervision/state.json" "$1" "$2" "$3" <<'PY'
import json, sys, time
from pathlib import Path
census_path, state_path = Path(sys.argv[1]), Path(sys.argv[2])
value = json.loads(census_path.read_text()); state = json.loads(state_path.read_text())
value.update({
    "completedAtEpoch": int(time.time()) - int(sys.argv[3]),
    "intervalSec": int(sys.argv[4]),
    "verdict": "SUCCESS",
    "fingerprint": sys.argv[5],
    "generation": state["generation"],
})
census_path.write_text(json.dumps(value) + "\n")
PY
}
assert_stale_shape() { # name, window
  local name=$1 window=$2 output="$tmp/$1.out"
  grep -Eq 'age=[0-9]+s' "$output" \
    && grep -Fq "window=${window}s" "$output" \
    && grep -Fq 'retry in a moment' "$output" \
    && grep -Fq "re-arm with $gate_repo_real/scripts/agents/arm-supervision.sh --repo $gate_repo_real if supervision is dead" "$output" \
    || { echo "stale census refusal did not carry the common diagnostic shape" >&2; cat "$output" >&2; exit 1; }
}
dispatch_fails no-census 'census verdict is absent'
mkdir -p "$gate_repo/artifacts/agents/supervision"
cp "$state" "$gate_repo/artifacts/agents/supervision/state.json"
cp "$last" "$gate_repo/artifacts/agents/supervision/last-census.json"
gate_fingerprint=$($gate_repo/scripts/agents/arm-supervision.sh fingerprint --repo "$gate_repo")
cp "$gate_repo/artifacts/agents/supervision/state.json" "$tmp/gate-state.json"
perl -0pi -e 's/^watch\.stale-min=.*$/watch.stale-min=21/m' "$gate_repo/metasystem.conf"
[[ "$($gate_repo/scripts/agents/arm-supervision.sh fingerprint --repo "$gate_repo")" != "$gate_fingerprint" ]] \
  || { echo "S4-3: relevant configuration did not alter the fingerprint" >&2; exit 1; }
git -C "$gate_repo" show HEAD:metasystem.conf >"$gate_repo/metasystem.conf"
python3 - "$gate_repo/artifacts/agents/supervision/state.json" <<'PY'
import json,sys
p=sys.argv[1]; v=json.load(open(p)); v["owner"]["instanceTag"] += "-changed"; json.dump(v,open(p,"w"))
PY
[[ "$($gate_repo/scripts/agents/arm-supervision.sh fingerprint --repo "$gate_repo")" == "$gate_fingerprint" ]] \
  || { echo "S4-3: supervisor instance identity altered the static fingerprint" >&2; exit 1; }
cp "$tmp/gate-state.json" "$gate_repo/artifacts/agents/supervision/state.json"
printf '\n# watcher fingerprint fixture\n' >>"$gate_repo/scripts/watch-background-jobs.sh"
[[ "$($gate_repo/scripts/agents/arm-supervision.sh fingerprint --repo "$gate_repo")" != "$gate_fingerprint" ]] \
  || { echo "S4-3: watcher code did not alter the fingerprint" >&2; exit 1; }
git -C "$gate_repo" show HEAD:scripts/watch-background-jobs.sh >"$gate_repo/scripts/watch-background-jobs.sh"
chmod +x "$gate_repo/scripts/watch-background-jobs.sh"

# Freshness is one capped window. Inside proceeds; the exact boundary and one
# second past refuse with age, window, and both remedies. A configured interval
# above the cap still refuses at 180 seconds.
set_gate_census 0 10 "$gate_fingerprint"
dispatch_succeeds inside-census-window
set_gate_census 20 10 "$gate_fingerprint"
dispatch_fails census-window-boundary 'census verdict is stale'
gate_repo_real=$(cd "$gate_repo" && pwd -P)
assert_stale_shape census-window-boundary 20
set_gate_census 21 10 "$gate_fingerprint"
dispatch_fails stale-census 'census verdict is stale'
assert_stale_shape stale-census 20
perl -0pi -e 's/^watch\.interval-sec=.*$/watch.interval-sec=200/m' "$gate_repo/metasystem.conf"
capped_fingerprint=$($gate_repo/scripts/agents/arm-supervision.sh fingerprint --repo "$gate_repo")
set_gate_census 180 200 "$capped_fingerprint"
dispatch_fails capped-census-window 'census verdict is stale'
assert_stale_shape capped-census-window 180
git -C "$gate_repo" show HEAD:metasystem.conf >"$gate_repo/metasystem.conf"

# A census from the prior arming generation uses that same refusal shape and
# additionally names both generations.
set_gate_census 0 10 "$gate_fingerprint"
python3 - "$gate_repo/artifacts/agents/supervision/state.json" <<'PY'
import json, sys
from pathlib import Path
path = Path(sys.argv[1]); value = json.loads(path.read_text()); value["generation"] += 1
path.write_text(json.dumps(value) + "\n")
PY
dispatch_fails stale-census-generation 'census verdict is stale'
assert_stale_shape stale-census-generation 20
grep -Fq 'censusGeneration=' "$tmp/stale-census-generation.out" \
  && grep -Fq 'armedGeneration=' "$tmp/stale-census-generation.out" \
  || { echo "generation-stale refusal did not name both generations" >&2; cat "$tmp/stale-census-generation.out" >&2; exit 1; }
cp "$tmp/gate-state.json" "$gate_repo/artifacts/agents/supervision/state.json"
set_gate_census 0 10 "$gate_fingerprint"
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
# The hook reports on the main it belongs to, so it must run as one: the fake
# runtime's ancestor variable names this shell, which become_main announced and
# which therefore holds this checkout. Without it the hook correctly reports
# that it is a read-only advisor and never reaches the drift it is asked about.
printf '{"session_id":"stale-surface","cwd":"%s","hook_event_name":"Stop"}\n' "$gate_repo" \
  | METASYSTEM_FAKE_AGENT_ANCESTOR_PID=$$ \
    "$gate_repo/scripts/agents/supervision-hook.sh" fake stop >"$tmp/stale-surface.out"
grep -Fq 'code changed since arming' "$tmp/stale-surface.out" \
  || { echo "S4-3/S4-4: end-turn hook hid fingerprint drift" >&2; exit 1; }
grep -Fq 'owner' "$tmp/stale-surface.out" && grep -Fq 'not running' "$tmp/stale-surface.out" \
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
# The engine owner narrates the failing observation and the relaunch to
# owner.ndjson (the shell owner's supervisor.log retired with it).
grep -q '"observation":"failing"' "$repo/artifacts/agents/supervision/owner.ndjson" \
  && grep -q '"relaunch"' "$repo/artifacts/agents/supervision/owner.ndjson" \
  || { echo "watcher death was not surfaced" >&2; exit 1; }
reaper_pid=$(json_field "$state" components.reaper.pid)
reaper_start=$(json_field "$state" components.reaper.pidStartedAt)
watcher_before_reaper_kill=$(json_field "$state" components.watcher.pid)
stop_owned_pid "S4-4 reaper" "$reaper_pid" "$reaper_start"
wait_until "S4-4 reaper recovery" component_replaced reaper "$reaper_pid"
wait_until "S4-4 reaper recovery replaces set" component_replaced watcher "$watcher_before_reaper_kill"
# Surfaced the same way: a failing observation followed by a relaunch in the
# owner's narration (three generations now exist, so at least two relaunches).
[[ $(grep -c '"relaunch"' "$repo/artifacts/agents/supervision/owner.ndjson") -ge 2 ]] \
  || { echo "reaper death was not surfaced" >&2; exit 1; }

stop_owned_pid "supervision owner for takeover" "$owner_before" "$owner_start"
wait_until "proven-dead owner" bash -c '! "$1" census alive --pid "$2" --start-time "$3" >/dev/null 2>&1' _ "$census_engine" "$owner_before" "$owner_start"
printf '[]\n' >"$process_fixture"
release_checkout "$repo"
"$arm" --repo "$repo" --session takeover --pid "$$" --start-time "$(process_started_at "$$")" --tag takeover-main >/dev/null
new_owner=$(json_field "$repo/artifacts/agents/supervision/lock.d/owner.json" pid)
[[ "$new_owner" != "$owner_before" ]] || { echo "stale supervision lock was not taken over" >&2; exit 1; }

# Dead announcements are pruned; SessionEnd retires its own; log rotation and
# end-of-turn UNTRACKED/stale-supervisor surfacing remain visible.
dead_pid=$(python3 - <<'PY'
import subprocess,sys
p=subprocess.Popen(
    [sys.executable,"-c","import signal; signal.pause()"],
    stdin=subprocess.DEVNULL,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,
    start_new_session=True,
)
print(p.pid)
PY
)
dead_start=$(process_started_at "$dead_pid")
owned_pids+=("$dead_pid:$dead_start")
release_checkout "$repo"
"$arm" --repo "$repo" --session dead-main --pid "$dead_pid" --start-time "$dead_start" --tag dead-main >/dev/null
stop_owned_pid "dead announcement" "$dead_pid" "$dead_start"
wait_until "dead announcement pruning" test ! -e "$repo/artifacts/agents/mains/dead-main-$dead_pid.json"

write_process_fixture "$process_fixture" "$raw_pid|1|$raw_pid|$raw_start|metasystem-fake-agent raw|$repo"
wait_until "UNTRACKED end-turn source" inventory_has "$last" UNTRACKED "$raw_pid"
# The main that armed this checkout last was the dead one this phase buried, so
# this shell takes the checkout back before ending a turn in it: an end-of-turn
# report belongs to the main that holds the checkout.
become_main "$repo" surface
printf '{"session_id":"surface","cwd":"%s","hook_event_name":"Stop"}\n' "$repo" \
  | METASYSTEM_FAKE_AGENT_ANCESTOR_PID=$$ \
    "$repo/scripts/agents/supervision-hook.sh" fake stop >"$tmp/surface.out"
grep -q 'UNTRACKED' "$tmp/surface.out" || { echo "end-of-turn hook hid UNTRACKED" >&2; exit 1; }

# S4-15: a stop event inside a repository is NEVER silent. Silence reads
# identically to "still running", to "finished", and to "the hook never
# fired", which is the ambiguity this check exists to remove. Whatever the
# state — untracked agents, stale supervision, running jobs, or nothing at
# all — the hook must say something, and when nothing is left it must say so
# in plain words.
idle_repo=$tmp/idle-hook
mkdir -p "$idle_repo/artifacts/agents/jobs" "$idle_repo/artifacts/agents/supervision" "$idle_repo/plans" "$idle_repo/scripts" "$idle_repo/bin"
cp -R "$source_root/scripts/agents" "$idle_repo/scripts/"
cp "$source_root/metasystem.conf" "$idle_repo/"
cp "$ms" "$idle_repo/bin/metasystem"
# The hook refuses outside a git repository, correctly: it reports on a
# repository's work. The sandbox must be one.
git -C "$idle_repo" init -q -b main
printf '{"session_id":"idle","cwd":"%s","hook_event_name":"Stop"}\n' "$idle_repo" \
  | "$idle_repo/scripts/agents/supervision-hook.sh" fake stop >"$tmp/idle.out" 2>/dev/null || true
[[ -s "$tmp/idle.out" ]] \
  || { echo "turn-end hook was silent inside a repository" >&2; exit 1; }
grep -q 'systemMessage' "$tmp/idle.out" \
  || { echo "turn-end hook emitted no surfaced message" >&2; cat "$tmp/idle.out" >&2; exit 1; }

# And the idle wording itself, exercised directly on the branch that produces
# it, so the sentence a human reads cannot silently change.
grep -Fq 'NOTHING LEFT TO WORK ON' "$idle_repo/scripts/agents/supervision-hook.sh" \
  && grep -Fq 'STILL WORKING' "$idle_repo/scripts/agents/supervision-hook.sh" \
  || { echo "turn-end hook lost one of its two plain-words states" >&2; exit 1; }

# S4-12: a turn that ends while a plan still names an unblocked next step and
# nothing is in flight must say so. Continuation is the one part of the loop no
# prompt can guarantee, so it is checked rather than requested in prose.
# The reporter also treats a running gate as work in flight. That is a fact
# about the whole machine -- and this suite IS a gate run -- so the fixture
# states it rather than racing it, and asserts the plan-reading it is about.
export METASYSTEM_GATES_RUNNING=0
open_work_root=$tmp/open-work
mkdir -p "$open_work_root/plans" "$open_work_root/artifacts/agents/jobs"
cat >"$open_work_root/plans/stream.md" <<'EOF'
- Waiting on the human: nothing blocking
- Next step: dispatch the runner
EOF
[[ "$("$ms" report open-work --repo "$open_work_root")" == "OPEN-WORK plans/stream.md: dispatch the runner" ]] \
  || { echo "open work with nothing in flight was not reported" >&2; exit 1; }
printf '{"status":"running"}\n' >"$open_work_root/artifacts/agents/jobs/live.json"
[[ -z "$("$ms" report open-work --repo "$open_work_root")" ]] \
  || { echo "open work was reported while a job was in flight" >&2; exit 1; }
rm -f "$open_work_root/artifacts/agents/jobs/live.json"
cat >"$open_work_root/plans/stream.md" <<'EOF'
- Waiting on the human: D-9, which model tier to use
- Next step: dispatch the runner
EOF
[[ -z "$("$ms" report open-work --repo "$open_work_root")" ]] \
  || { echo "a stream waiting on the human was reported as open work" >&2; exit 1; }
cat >"$open_work_root/plans/stream.md" <<'EOF'
- Waiting on the human: nothing blocking
- Next step: none
EOF
[[ -z "$("$ms" report open-work --repo "$open_work_root")" ]] \
  || { echo "a settled next step was reported as open work" >&2; exit 1; }

# A gate run is work in flight, and the reporter believes a gate marker only
# while the process it names is alive. Asking the process table for anything
# that MENTIONS the gate script reported "STILL WORKING: the test gates" with
# no gate running anywhere -- and since a running gate counts as work in
# flight, that false yes silenced this whole report.
gate_probe_root=$tmp/gate-probe
mkdir -p "$gate_probe_root/plans" "$gate_probe_root/artifacts/agents/jobs"
cat >"$gate_probe_root/plans/stream.md" <<'EOF'
- Waiting on the human: nothing blocking
- Next step: dispatch the runner
EOF
gate_probe_markers=$gate_probe_root/artifacts/agents/supervision/gate-runs
(
  unset METASYSTEM_GATES_RUNNING
  [[ -n "$("$ms" report open-work --repo "$gate_probe_root")" ]] \
    || { echo "open work went unreported with no gate marker at all" >&2; exit 1; }
  "$ms" gate register --root "$gate_probe_root" \
    --gate fixture-gate.sh --pid $$ >/dev/null
  [[ -z "$("$ms" report open-work --repo "$gate_probe_root")" ]] \
    || { echo "a live gate run was not counted as work in flight" >&2; exit 1; }
  python3 - "$gate_probe_markers" <<'PY'
import json, sys
from pathlib import Path
directory = Path(sys.argv[1])
for path in directory.glob("*.json"):
    value = json.loads(path.read_text())
    value.update({"pid": 999999, "pidStartedAt": 1})
    (directory / "999999.json").write_text(json.dumps(value) + "\n")
    path.unlink()
PY
  [[ -n "$("$ms" report open-work --repo "$gate_probe_root")" ]] \
    || { echo "a gate marker whose process is dead still hid open work" >&2; exit 1; }
  [[ -z "$(ls -A "$gate_probe_markers")" ]] \
    || { echo "a gate marker whose process is dead was not pruned" >&2; exit 1; }
)

# S4-13: a plan whose own account of itself contradicts the job records. A check
# that reads a stale plan reports the wrong work as confidently as the right
# work, which is how this session lost a turn.
cat >"$open_work_root/plans/stream.md" <<'EOF'
- In flight right now: job design-critic-20260101t000000z-aaaa
- Waiting on the human: nothing blocking
- Next step: none
EOF
"$ms" report open-work --repo "$open_work_root" \
  | grep -q 'STALE-PLAN plans/stream.md: claims work in flight while no job is running' \
  || { echo "a plan claiming absent work was not reported stale" >&2; exit 1; }
printf '{"jobId":"design-critic-20260101t000000z-aaaa-r3","status":"running"}\n' \
  >"$open_work_root/artifacts/agents/jobs/live.json"
[[ -z "$("$ms" report open-work --repo "$open_work_root" | grep STALE-PLAN)" ]] \
  || { echo "a plan naming the chain root of a live round was called stale" >&2; exit 1; }
cat >"$open_work_root/plans/other.md" <<'EOF'
- In flight right now: nothing
- Waiting on the human: nothing blocking
- Next step: none
EOF
[[ -z "$("$ms" report open-work --repo "$open_work_root" | grep 'STALE-PLAN plans/other.md')" ]] \
  || { echo "an idle stream was called stale because another stream had a job" >&2; exit 1; }
rm -f "$open_work_root/plans/other.md" "$open_work_root/artifacts/agents/jobs/live.json"
cp "$source_root/plans/README.md" "$open_work_root/plans/README.md"
rm -f "$open_work_root/plans/stream.md"
[[ -z "$("$ms" report open-work --repo "$open_work_root")" ]] \
  || { echo "the plans README was mistaken for a stream" >&2; exit 1; }
# The armed engine watcher publishes verdicts, not a census.log; the
# transcript log and its byte-capped rotation belong to the shell job
# watcher's --census mode, so rotation is proven there, in its own sandbox
# (the armed repository's census-writer lock is held by the live watcher).
rotation_repo=$tmp/rotation-repo
make_repo "$rotation_repo"
rotation_supervision=$rotation_repo/artifacts/agents/supervision
mkdir -p "$rotation_supervision" "$rotation_repo/artifacts/agents/jobs"
printf '[]\n' >"$rotation_repo/processes.json"
touch "$rotation_supervision/jobs.state"
# A healthy engine pass prints nothing, so the transcript only grows on
# warnings; a 1ms interval budget makes every pass a CENSUS-SLOW warning,
# which is exactly the content the transcript exists to keep.
rotation_passes=0
until [[ -f "$rotation_supervision/census.log.1" ]]; do
  (( rotation_passes < 40 )) \
    || { echo "census log rotation never happened after $rotation_passes passes" >&2; exit 1; }
  METASYSTEM_CENSUS_PROCESS_FILE="$rotation_repo/processes.json" \
  METASYSTEM_CENSUS_INTERVAL_MS=1 \
    "$rotation_repo/scripts/watch-background-jobs.sh" \
      --dir "$rotation_repo/artifacts/agents/jobs" --scope "$rotation_repo" \
      --state "$rotation_supervision/jobs.state" --interval 1 --once --census \
      --supervision-dir "$rotation_supervision" \
      --heartbeat "$rotation_supervision/watcher.heartbeat.json" \
      --instance-tag rotation-fixture >/dev/null 2>&1 || true
  rotation_passes=$((rotation_passes + 1))
done

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
(cd "$foreign/repo" && git init -q -b main .)
mkdir -p "$foreign/repo/metasystem/scripts/agents" "$foreign/repo/metasystem/artifacts/agents/supervision/lock.d"
cp "$source_root/scripts/agents/arm-supervision.sh" \
  "$source_root/scripts/agents/preflight-commands.sh" \
  "$foreign/repo/metasystem/scripts/agents/"
# Shutting supervision down is a control-plane write, so the sandbox needs the
# engine (census identity, lease classification); without it this sandbox
# refuses for the wrong reason and the foreign-owner rule below is never
# actually exercised.
mkdir -p "$foreign/repo/metasystem/bin"
cp "$ms" "$foreign/repo/metasystem/bin/metasystem"
foreign_sleep_pid=$(
  bash -c '"$1" util hold --tag metasystem-foreign-owner >/dev/null 2>&1 & echo $!' _ "$ms"
)
foreign_start=$(process_started_at "$foreign_sleep_pid")
owned_pids+=("$foreign_sleep_pid:$foreign_start")
python3 - "$foreign/repo/metasystem/artifacts/agents/supervision/lock.d/owner.json" \
  "$foreign_sleep_pid" "$foreign_start" <<'PY'
import json, sys
from pathlib import Path
Path(sys.argv[1]).write_text(json.dumps({
  "pid": int(sys.argv[2]), "pidStartedAt": int(sys.argv[3]),
  "instanceTag": "metasystem-supervision-owner-some-other-checkout-1-2",
  "acquiredAt": "1970-01-01T00:00:00Z",
}) + "\n")
PY
set +e
"$foreign/repo/metasystem/scripts/agents/arm-supervision.sh" --repo "$foreign/repo" --shutdown \
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

# S4-14: the stop hook refuses a turn that ends with open work, once, and leaves
# evidence that it ran. A report was ignorable and was ignored four times in one
# session; an unbounded refusal is the loop the design forbids.
stop_root=$tmp/stop-hook
mkdir -p "$stop_root/plans" "$stop_root/artifacts/agents/jobs" "$stop_root/artifacts/agents/supervision" "$stop_root/scripts/agents"
cp "$source_root/scripts/agents/supervision-hook.sh" \
   "$source_root/scripts/agents/arm-supervision.sh" "$stop_root/scripts/agents/"
# The hook resolves its engine as <root>/bin/metasystem; the open-work,
# stop-block, identity, and lease helpers it used to need as .py files all
# live inside it now.
mkdir -p "$stop_root/bin"
cp "$ms" "$stop_root/bin/metasystem"
cat >"$stop_root/plans/stream.md" <<'FIXTURE'
- In flight right now: nothing
- Waiting on the human: nothing blocking
- Next step: dispatch the runner
FIXTURE
git -C "$stop_root" init -q -b main
stop_payload=$(printf '{"session_id":"t","cwd":"%s","hook_event_name":"Stop"}' "$stop_root")
first=$(printf '%s' "$stop_payload" | bash "$stop_root/scripts/agents/supervision-hook.sh" claude stop)
printf '%s' "$first" | grep -q '"decision":"block"' \
  || { echo "the stop hook did not refuse a turn ending with open work" >&2; echo "$first" >&2; exit 1; }
second=$(printf '%s' "$stop_payload" | bash "$stop_root/scripts/agents/supervision-hook.sh" claude stop)
printf '%s' "$second" | grep -q '"decision":"block"' \
  && { echo "the stop hook refused the same open work twice, which is the loop the design forbids" >&2; exit 1; }
[[ -s "$stop_root/artifacts/agents/supervision/hooks.log" ]] \
  || { echo "the stop hook left no evidence that it ran" >&2; exit 1; }
cat >"$stop_root/plans/stream.md" <<'FIXTURE'
- In flight right now: nothing
- Waiting on the human: nothing blocking
- Next step: none
FIXTURE
settled=$(printf '%s' "$stop_payload" | bash "$stop_root/scripts/agents/supervision-hook.sh" claude stop)
printf '%s' "$settled" | grep -q '"decision":"block"' \
  && { echo "the stop hook refused a turn with no open work" >&2; exit 1; }

echo "supervision fixtures passed (S4-1 through S4-14)"
