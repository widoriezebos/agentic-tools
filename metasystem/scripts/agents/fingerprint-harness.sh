#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage: scripts/agents/fingerprint-harness.sh [--iterations N]

Build a fixture-fast sandbox repository, force one supervision heal per
iteration, and report dispatch refusals caused by the census gate.
USAGE
}

iterations=20
while (($#)); do
  case "$1" in
    --iterations)
      [[ $# -ge 2 ]] || { usage; exit 2; }
      iterations=$2
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      usage
      exit 2
      ;;
  esac
done
[[ "$iterations" =~ ^[1-9][0-9]*$ ]] || { echo "--iterations must be a positive integer" >&2; exit 2; }

source_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
source "$source_root/scripts/agents/fixture-budget.sh"
: "${METASYSTEM_FIXTURE_POLL_INTERVAL_MS:=10}"
: "${METASYSTEM_CENSUS_INTERVAL_MS:=250}"
: "${METASYSTEM_WATCH_POLL_INTERVAL_MS:=50}"
: "${METASYSTEM_HEARTBEAT_INTERVAL_MS:=20}"
: "${METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS:=20}"
harness_fixture_budget_init "$source_root"
fixture_ceiling_sec=$(harness_fixture_cap supervision-wait)

tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-fingerprint-harness.XXXXXX")
repo=$tmp/repo
paused_pid=

cleanup() {
  if [[ -n "$paused_pid" ]]; then kill -CONT "$paused_pid" 2>/dev/null || true; fi
  if [[ -x "$repo/scripts/agents/arm-supervision.sh" ]]; then
    "$repo/scripts/agents/arm-supervision.sh" --repo "$repo" --shutdown >/dev/null 2>&1 || true
  fi
  if [[ -n "${METASYSTEM_KEEP_FINGERPRINT_FIXTURE:-}" ]]; then
    echo "kept fingerprint harness fixture: $tmp" >&2
  else
    rm -rf "$tmp"
  fi
}
trap cleanup EXIT

wait_until() { # description, command...
  local description=$1 started=$SECONDS deadline=$((SECONDS + fixture_ceiling_sec)) elapsed
  shift
  until "$@"; do
    if (( SECONDS >= deadline )); then
      elapsed=$((SECONDS - started))
      echo "fingerprint harness timed out: $description (elapsed: ${elapsed}s; scaled cap: ${fixture_ceiling_sec}s)" >&2
      return 1
    fi
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
  done
}

json_field() { # file, dotted field
  python3 - "$1" "$2" <<'PY'
import json, sys
value = json.load(open(sys.argv[1]))
for part in sys.argv[2].split("."):
    value = value[part]
print(value)
PY
}

component_healed() { # state, component, old pid, old generation
  python3 - "$1" "$2" "$3" "$4" <<'PY'
import json, sys
value = json.load(open(sys.argv[1]))
component = value["components"][sys.argv[2]]
raise SystemExit(0 if component["pid"] != int(sys.argv[3]) and value["generation"] > int(sys.argv[4]) else 1)
PY
}

census_matches_snapshot() { # census, exact state snapshot
  python3 - "$1" "$2" <<'PY'
import hashlib, json, sys
from pathlib import Path
census = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
state_bytes = Path(sys.argv[2]).read_bytes()
state = json.loads(state_bytes)
expected_digest = hashlib.sha256(state_bytes).hexdigest()
raise SystemExit(0 if census.get("verdict") == "SUCCESS"
                 and census.get("generation") == state.get("generation")
                 and census.get("stateDigest") == expected_digest else 1)
PY
}

watcher_pass_complete() { # heartbeat, census
  python3 - "$1" "$2" <<'PY'
import json, sys
heartbeat = json.load(open(sys.argv[1]))
census = json.load(open(sys.argv[2]))
raise SystemExit(0 if heartbeat.get("observedAtEpoch", 0) >= census.get("completedAtEpoch", 1) else 1)
PY
}

prove_process_ownership() { # pid, start, tag
  local pid=$1 start=$2 tag=$3 command
  "$census" alive --pid "$pid" --start-time "$start" >/dev/null || return 1
  command=$(ps -p "$pid" -o command= 2>/dev/null || true)
  [[ "$command" == *"$tag"* || "$command" == *"$repo"* ]]
}

backdate_census_generation() { # census, generation
  python3 - "$1" "$2" <<'PY'
import json, os, sys, tempfile, time
from pathlib import Path
path, generation = Path(sys.argv[1]), int(sys.argv[2])
value = json.loads(path.read_text(encoding="utf-8"))
value["generation"] = generation
value["completedAtEpoch"] = int(time.time())
fd, temporary = tempfile.mkstemp(prefix=path.name + ".", suffix=".tmp", dir=path.parent)
with os.fdopen(fd, "w", encoding="utf-8") as handle:
    json.dump(value, handle, indent=2, sort_keys=True)
    handle.write("\n")
    handle.flush()
    os.fsync(handle.fileno())
os.replace(temporary, path)
PY
}

assert_backdated_refusal() { # output, census generation, armed generation
  local output=$1 census_generation=$2 armed_generation=$3
  grep -Fq 'dispatch refused: census verdict is stale (age=' "$output" \
    && grep -Fq "censusGeneration=$census_generation" "$output" \
    && grep -Fq "armedGeneration=$armed_generation" "$output" \
    && grep -Fq 'retry in a moment' "$output" \
    && grep -Fq 're-arm with ' "$output" \
    && grep -Fq 'if supervision is dead' "$output"
}

mkdir -p "$repo/scripts" "$repo/docs"
cp -R "$source_root/scripts/agents" "$repo/scripts/"
cp "$source_root/scripts/metasystem-config.sh" \
  "$source_root/scripts/assert-mission.sh" \
  "$source_root/scripts/assert-stop-loss.sh" \
  "$source_root/scripts/assert-return-complete.sh" \
  "$source_root/scripts/assert-turn-prompt.sh" \
  "$source_root/scripts/watch-background-jobs.sh" "$repo/scripts/"
cp "$source_root/docs/project-rules.md" "$repo/docs/"
cp "$source_root/metasystem.conf" "$repo/"
perl -0pi -e 's/^metasystem\.runtimes=.*$/metasystem.runtimes=fake/m; s|^evidence\.root=.*$|evidence.root='"$tmp/evidence"'|m; s/^watch\.interval-sec=.*$/watch.interval-sec=1/m; s/^role\.default\.runtime=.*$/role.default.runtime=fake/m; s/^role\.default\.model\.codex=.*$/role.default.model.fake=fake-model/m; s/^role\.default\.model\.(?:claude|devin)=.*\n//mg; s/^role\.(code-critic|investigator)\.runtime=main$/role.$1.runtime=fake/mg; s/\.runtime=(?:codex|devin)$/\.runtime=fake/mg; s/\.model\.(?:codex|devin)=.*$/\.model.fake=fake-model/mg' "$repo/metasystem.conf"
printf '\nmodel.tier.1=fake:fake-model\n' >>"$repo/metasystem.conf"
git -C "$repo" init -q
git -C "$repo" add .
git -C "$repo" -c user.name=metasystem -c user.email=metasystem.invalid commit -qm fixture

process_fixture=$tmp/processes.json
identity_fixture=$tmp/process-identities.json
printf '[]\n' >"$process_fixture"
printf '{}\n' >"$identity_fixture"
export METASYSTEM_CENSUS_PROCESS_FILE=$process_fixture
export METASYSTEM_FAKE_PROCESS_IDENTITY_FILE=$identity_fixture

arm=$repo/scripts/agents/arm-supervision.sh
census=$repo/scripts/agents/process-census.py
dispatch=$repo/scripts/agents/dispatch.sh
state=$repo/artifacts/agents/supervision/state.json
last=$repo/artifacts/agents/supervision/last-census.json
brief=$tmp/brief.md
sed 's/^Working Mode:.*/Working Mode: design/' "$repo/scripts/agents/templates/brief.md" >"$brief"
"$repo/scripts/agents/adapters/fake.sh" probe >/dev/null
main_start=$("$census" started-at --pid "$$")

refusals=0
last_published_generation=0
for ((iteration = 1; iteration <= iterations; iteration++)); do
  METASYSTEM_AGENT_RUNTIME=fake "$arm" --repo "$repo" \
    --session "fingerprint-$iteration" --pid "$$" --start-time "$main_start" \
    --tag "metasystem-main-fake-fingerprint-$iteration" >/dev/null

  armed_generation=$(json_field "$state" generation)
  if (( last_published_generation > 0 && armed_generation <= last_published_generation )); then
    echo "fingerprint harness generation repeated after fresh arming: previous=$last_published_generation current=$armed_generation" >&2
    exit 1
  fi

  old_pid=$(json_field "$state" components.watcher.pid)
  old_start=$(json_field "$state" components.watcher.pidStartedAt)
  old_tag=$(json_field "$state" components.watcher.instanceTag)
  old_generation=$(json_field "$state" generation)
  prove_process_ownership "$old_pid" "$old_start" "$old_tag" \
    || { echo "fingerprint harness could not prove watcher ownership before kill" >&2; exit 1; }
  kill -TERM "$old_pid"
  wait_until "iteration $iteration component heal" \
    component_healed "$state" watcher "$old_pid" "$old_generation"
  healed_generation=$(json_field "$state" generation)
  (( healed_generation > old_generation )) \
    || { echo "fingerprint harness generation did not strictly increase across heal" >&2; exit 1; }

  snapshot=$tmp/state-after-heal-$iteration.json
  cp "$state" "$snapshot"
  wait_until "iteration $iteration census snapshot provenance" \
    census_matches_snapshot "$last" "$snapshot"

  output=$tmp/dispatch-$iteration.out
  set +e
  "$dispatch" dispatch --role design-critic --brief "$brief" \
    --job-id "fingerprint-$iteration" --wait >"$output" 2>&1
  status=$?
  set -e
  if grep -Fq 'dispatch refused: census fingerprint does not match' "$output"; then
    refusals=$((refusals + 1))
  elif grep -Fq 'dispatch refused: census verdict is stale' "$output"; then
    refusals=$((refusals + 1))
  elif (( status != 0 )); then
    echo "fingerprint harness dispatch failed outside the fingerprint gate" >&2
    cat "$output" >&2
    exit 1
  fi

  watcher_pid=$(json_field "$state" components.watcher.pid)
  watcher_start=$(json_field "$state" components.watcher.pidStartedAt)
  watcher_tag=$(json_field "$state" components.watcher.instanceTag)
  watcher_heartbeat=$(json_field "$state" components.watcher.heartbeat)
  wait_until "iteration $iteration completed watcher census pass" \
    watcher_pass_complete "$watcher_heartbeat" "$last"
  prove_process_ownership "$watcher_pid" "$watcher_start" "$watcher_tag" \
    || { echo "fingerprint harness could not prove watcher ownership before pause" >&2; exit 1; }
  paused_pid=$watcher_pid
  kill -STOP "$paused_pid"
  backdated_generation=$((healed_generation - 1))
  backdate_census_generation "$last" "$backdated_generation"
  backdated_output=$tmp/backdated-$iteration.out
  set +e
  "$dispatch" dispatch --role design-critic --brief "$brief" \
    --job-id "fingerprint-backdated-$iteration" --wait >"$backdated_output" 2>&1
  backdated_status=$?
  set -e
  (( backdated_status != 0 )) \
    && assert_backdated_refusal "$backdated_output" "$backdated_generation" "$healed_generation" \
    || { echo "fingerprint harness did not get the complete back-dated generation refusal" >&2; cat "$backdated_output" >&2; exit 1; }
  kill -CONT "$paused_pid"
  paused_pid=
  cp "$state" "$snapshot"
  wait_until "iteration $iteration census recovery after back-date" \
    census_matches_snapshot "$last" "$snapshot"

  "$arm" --repo "$repo" --shutdown >/dev/null
  METASYSTEM_AGENT_RUNTIME=fake "$arm" --repo "$repo" \
    --session "fingerprint-restart-$iteration" --pid "$$" --start-time "$main_start" \
    --tag "metasystem-main-fake-fingerprint-restart-$iteration" >/dev/null
  restarted_generation=$(json_field "$state" generation)
  (( restarted_generation > healed_generation )) \
    || { echo "fingerprint harness generation repeated after owner restart: previous=$healed_generation current=$restarted_generation" >&2; exit 1; }
  state_digest_before_join=$(shasum -a 256 "$state" | awk '{print $1}')
  METASYSTEM_AGENT_RUNTIME=fake "$arm" --repo "$repo" \
    --session "fingerprint-join-$iteration" --pid "$$" --start-time "$main_start" \
    --tag "metasystem-main-fake-fingerprint-join-$iteration" >/dev/null
  joined_generation=$(json_field "$state" generation)
  state_digest_after_join=$(shasum -a 256 "$state" | awk '{print $1}')
  [[ "$joined_generation" == "$restarted_generation" && "$state_digest_after_join" == "$state_digest_before_join" ]] \
    || { echo "fingerprint harness live-owner join republished supervision state" >&2; exit 1; }
  last_published_generation=$restarted_generation
  "$arm" --repo "$repo" --shutdown >/dev/null
done

(( refusals == 0 )) \
  || { echo "fingerprint harness observed $refusals ordinary dispatch refusals after the fix" >&2; exit 1; }
printf 'fingerprint harness: iterations=%s refusals=%s\n' "$iterations" "$refusals"
