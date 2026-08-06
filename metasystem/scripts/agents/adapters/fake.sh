#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/agents/adapters/fake.sh identity
  scripts/agents/adapters/fake.sh signature
  scripts/agents/adapters/fake.sh probe [--profile current|old|unverified-network]
      [--age-days N]
  scripts/agents/adapters/fake.sh dispatch --job <job-id> --start-gate <file>
      --instance-tag <tag>
  scripts/agents/adapters/fake.sh follow-up --job <job-id> --start-gate <file>
      --instance-tag <tag>
  scripts/agents/adapters/fake.sh cancel --job <job-id>
  scripts/agents/adapters/fake.sh selftest

The simulator reads FAKE:<behavior> markers from the assembled prompt.
Supported behaviors include malformed-return, missing-session-id,
resume-collision, concurrent-turn, cancel-race, process-loss, timeout,
no-session-signal, handshake-failure, no-event-stream, hook-unavailable,
interrupted-atomic-write, nested-agent-events, effective-wider,
effective-narrower, and mirror-failure. A Fake-Argument: line is captured as
data and never executed.
USAGE
}

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd -P)
dispatch="$root/scripts/agents/dispatch.sh"
agents="$root/artifacts/agents"
jobs="$agents/jobs"

field() {
  python3 - "$1" "$2" <<'PY'
import json, sys
from pathlib import Path
value = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
for part in sys.argv[2].split("."): value = value[part]
if value is None: print("null")
elif isinstance(value, bool): print("true" if value else "false")
elif isinstance(value, (dict, list)): print(json.dumps(value, separators=(",", ":")))
else: print(value)
PY
}

parse_supervisor_args() {
  job= gate= instance_tag=
  while (($#)); do
    case "$1" in
      --job) [[ $# -ge 2 ]] || { usage; exit 2; }; job=$2; shift 2 ;;
      --start-gate) [[ $# -ge 2 ]] || { usage; exit 2; }; gate=$2; shift 2 ;;
      --instance-tag) [[ $# -ge 2 ]] || { usage; exit 2; }; instance_tag=$2; shift 2 ;;
      *) usage; exit 2 ;;
    esac
  done
  [[ -n "$job" && -n "$gate" && -n "$instance_tag" ]] || { usage; exit 2; }
}

behavior_present() { grep -Fqi "FAKE:$1" "$prompt"; }

fake_guarded_write() { # permissions JSON, target path
  python3 - "$1" "$2" <<'PY'
import json, sys
from pathlib import Path

permissions = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
target = Path(sys.argv[2]).resolve()
for raw_root in permissions.get("writeRoots", []):
    root = Path(raw_root).resolve()
    try:
        target.relative_to(root)
    except ValueError:
        continue
    target.write_text("fake envelope write probe\n", encoding="utf-8")
    raise SystemExit(0)
raise SystemExit(77)
PY
}

fake_guarded_network_call() { # permissions JSON, host, port
  [[ $(field "$1" network) == allow ]] || return 77
  python3 - "$2" "$3" <<'PY'
import socket, sys

with socket.create_connection((sys.argv[1], int(sys.argv[2])), timeout=1) as connection:
    connection.sendall(b"GET /fake-envelope-probe HTTP/1.0\r\n\r\n")
PY
}

probe_fake_envelope_mechanism() {
  local probe_dir permissions target result write_status network_status
  probe_dir=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-fake-envelope-probe.XXXXXX")
  permissions="$probe_dir/permissions.json"
  target="$probe_dir/denied-write.txt"
  result=${METASYSTEM_FAKE_ENVELOPE_PROBE_RESULT:-}
  printf '{"readRoots":[],"writeRoots":[],"network":"deny"}\n' >"$permissions"
  set +e
  fake_guarded_write "$permissions" "$target"
  write_status=$?
  fake_guarded_network_call "$permissions" 127.0.0.1 9
  network_status=$?
  set -e
  if [[ $write_status -ne 77 || $network_status -ne 77 || -e "$target" ]]; then
    echo "fake envelope mechanism did not refuse a denied write and network call" >&2
    rm -rf "$probe_dir"
    return 1
  fi
  if [[ -n "$result" ]]; then
    python3 - "$result" "$write_status" "$network_status" <<'PY'
import json, sys
from pathlib import Path

Path(sys.argv[1]).write_text(json.dumps({
    "writeRoots": {"observed": "denied", "exitStatus": int(sys.argv[2])},
    "network": {"observed": "denied", "exitStatus": int(sys.argv[3])},
}, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
  fi
  rm -rf "$probe_dir"
}

fixture_milliseconds_to_sleep() { # positive integer milliseconds
  local milliseconds=$1
  [[ "$milliseconds" =~ ^[1-9][0-9]*$ ]] \
    || { echo "fake adapter interval must be a positive integer in milliseconds" >&2; return 2; }
  printf '%d.%03d\n' "$((milliseconds / 1000))" "$((milliseconds % 1000))"
}

cas_terminal() { # target, error, phase
  local target=$1 error=$2 phase=$3 patch
  patch="$round_dir/terminal-patch.json"
  python3 - "$patch" "$error" "$phase" <<'PY'
import json, sys
from pathlib import Path
error = None if sys.argv[2] == "null" else sys.argv[2]
usage = {
  "availability": "native", "inputTokens": 11, "cachedInputTokens": 2,
  "outputTokens": 7, "reasoningTokens": None, "cost": None,
  "providerUnits": {"name": "fake-unit", "value": 1},
}
Path(sys.argv[1]).write_text(json.dumps({"error": error, "phase": sys.argv[3], "usage": usage}) + "\n")
PY
  "$dispatch" __record-cas --job "$job" --expect running --status "$target" --patch "$patch" || {
    status=$?
    [[ $status -eq 3 ]] || return "$status"
  }
}

write_valid_return() {
  local return_file="$round_dir/return.json"
  python3 - "$record" "$prompt" "$return_file" <<'PY'
import json, sys
from pathlib import Path
record = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
mode = "implement"
for line in Path(sys.argv[2]).read_text(encoding="utf-8").splitlines():
    if line.startswith("Working Mode:"):
        mode = line.split(":", 1)[1].strip()
        break
common = {
  "jobId": record["jobId"], "round": record["round"], "runtime": "fake",
  "sessionId": record["sessionId"],
  "model": {"requested": record["requestedModel"], "effective": record["effectiveModel"]},
  "evidence": [{"command": "fake protocol simulator", "observed": "canned role return", "level": "ran"}],
  "gaps": [], "mode": mode,
}
role = record["role"]
if role in {"design-critic", "code-critic"}:
    common.update({"findings": [], "verdictMaterialCount": 0})
elif role == "implementer":
    common.update({"riskiestPart": "fake boundary", "diffBoundary": [], "whatWasDone": "simulated implementation"})
elif role == "verifier":
    common.update({"riskiestPart": "fake boundary", "whatWasDone": "simulated verification"})
elif role == "investigator":
    common.update({
      "frozenFrame": "simulated frozen frame",
      "theories": [{"statement": "fixture theory", "evidenceFor": "marker", "evidenceAgainst": "none"}],
      "classifications": ["falsified-continue"], "stopLoss": {"triggered": False, "trigger": None},
    })
else:
    raise SystemExit(f"unsupported fake role: {role}")
Path(sys.argv[3]).write_text(json.dumps(common, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
  printf '# Fake return\n\nCanonical JSON: return.json\n' >"$round_dir/return.md"
}

complete_valid() {
  write_valid_return
  if "$root/scripts/assert-return-complete.sh" --job "$job" >>"$log" 2>&1; then
    cas_terminal completed null completed
  else
    cas_terminal failed protocol_error validation
  fi
}

supervise() { # verb and remaining args
  local verb=$1 gate_poll heartbeat_sleep; shift
  parse_supervisor_args "$@"
  record="$jobs/$job.json"
  gate_poll=$(fixture_milliseconds_to_sleep "${METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS:-10}")
  heartbeat_sleep=$(fixture_milliseconds_to_sleep "${METASYSTEM_HEARTBEAT_INTERVAL_MS:-200}")
  while [[ ! -e "$gate" ]]; do sleep "$gate_poll"; done
  round=$(field "$record" round)
  root_job=$(python3 - "$jobs" "$job" <<'PY'
import json, sys
from pathlib import Path
jobs = Path(sys.argv[1]); job = sys.argv[2]
while True:
    value = json.loads((jobs / f"{job}.json").read_text())
    if value.get("parentJob") is None: print(job); break
    job = value["parentJob"]
PY
  )
  round_dir="$agents/$root_job/rounds/$round"
  prompt="$round_dir/prompt.md"
  log="$jobs/$job.log"
  raw="$round_dir/raw.out"
  events="$round_dir/events.jsonl"
  heartbeat="$agents/hb/$job"
  mkdir -p "$round_dir" "$(dirname "$heartbeat")"
  printf 'fake supervisor started value=%s\n' "$instance_tag" >"$log"
  printf 'fake raw output\n' >"$raw"
  printf '{"pid":%s,"pgid":%s,"instanceTag":"%s"}\n' "$$" "$$" "$instance_tag" >"$heartbeat"

  if behavior_present pending-process-loss; then
    printf '{"lost":true}\n' >"$heartbeat"
    kill -KILL "$$"
  fi

  if behavior_present no-session-signal; then
    printf 'ordinary output without a session-established event\n' >>"$log"
    python3 -c 'import signal; signal.pause()' "$instance_tag" &
    wait
  fi
  if behavior_present handshake-failure; then
    patch="$round_dir/handshake-failure.json"
    printf '{"error":"authentication_failed","phase":"handshake"}\n' >"$patch"
    "$dispatch" __record-cas --job "$job" --expect pending --status failed --patch "$patch" || true
    exit 1
  fi

  effective="$round_dir/effective-permissions.json"
  python3 - "$record" "$effective" <<'PY'
import json, sys
from pathlib import Path
record = json.loads(Path(sys.argv[1]).read_text())
value = dict(record["permissions"]["requested"])
Path(sys.argv[2]).write_text(json.dumps(value, indent=2, sort_keys=True) + "\n")
PY
  if behavior_present effective-wider; then
    python3 - "$effective" <<'PY'
import json, sys
from pathlib import Path
path = Path(sys.argv[1]); value = json.loads(path.read_text()); value["network"] = "allow"; path.write_text(json.dumps(value) + "\n")
PY
  fi
  if behavior_present effective-narrower; then
    python3 - "$effective" <<'PY'
import json, sys
from pathlib import Path
path = Path(sys.argv[1]); value = json.loads(path.read_text()); value["network"] = "deny"; path.write_text(json.dumps(value) + "\n")
PY
  fi
  session="fake-session-$root_job"
  [[ "$verb" == dispatch && "$round" -gt 1 ]] && session="fake-session-$root_job-fresh-$round"
  [[ "$verb" == follow-up ]] && session=$(field "$record" sessionId)
  behavior_present missing-session-id && session=
  signal=$(field "$record" sessionEstablishedSignal)
  "$dispatch" __handshake --job "$job" --session "$session" --turn "fake-turn-$round" \
    --model "$(field "$record" requestedModel)" --effective "$effective" --signal "$signal" || exit 1
  if ! behavior_present no-event-stream; then
    printf '{"event":"session-established","sessionId":"%s","round":%s}\n' "$session" "$round" >>"$events"
  fi

  if [[ "$verb" == follow-up ]]; then
    expected="fake-session-$root_job"
    [[ "$session" == "$expected" ]] || { cas_terminal failed resume_collision resume; exit 1; }
  fi
  if behavior_present resume-collision; then cas_terminal failed resume_collision resume; exit 1; fi
  if behavior_present process-loss; then
    python3 -c 'import signal,sys; from pathlib import Path; signal.signal(signal.SIGTERM, lambda *_: (Path(sys.argv[1]).write_text("stopped\n"), sys.exit(0))); signal.pause()' "$round_dir/child.stopped" "$instance_tag" &
    printf '%s\n' "$!" >"$round_dir/child.pid"
    printf '{"lost":true}\n' >"$heartbeat"
    kill -KILL "$$"
  fi
  if behavior_present timeout || behavior_present concurrent-turn; then
    python3 -c 'import signal,sys; from pathlib import Path; signal.signal(signal.SIGTERM, lambda *_: (Path(sys.argv[1]).write_text("stopped\n"), sys.exit(0))); signal.pause()' "$round_dir/child.stopped" "$instance_tag" &
    printf '%s\n' "$!" >"$round_dir/child.pid"
    while true; do touch "$heartbeat"; sleep "$heartbeat_sleep"; done
  fi
  if behavior_present cancel-race; then
    trap 'complete_valid; exit 0' TERM
    python3 -c 'import signal,sys; from pathlib import Path; signal.signal(signal.SIGTERM, lambda *_: (Path(sys.argv[1]).write_text("stopped\n"), sys.exit(0))); signal.pause()' "$round_dir/child.stopped" "$instance_tag" &
    printf '%s\n' "$!" >"$round_dir/child.pid"
    while true; do touch "$heartbeat"; sleep "$heartbeat_sleep"; done
  fi
  if behavior_present malformed-return; then
    printf '{malformed\n' >"$round_dir/return.json"
    printf 'malformed return\n' >>"$log"
    if "$root/scripts/assert-return-complete.sh" --job "$job" >>"$log" 2>&1; then
      cas_terminal completed null completed
    else
      cas_terminal failed protocol_error validation
    fi
    exit 0
  fi
  if behavior_present interrupted-atomic-write; then
    printf '{"status":"corrupt' >"$agents/record-locks/$job.interrupted"
  fi
  if behavior_present nested-agent-events; then
    printf '{"event":"agent.completed","agent":"nested","topLevel":false}\n' >>"$events"
    printf '{"event":"turn.completed","agent":"root","topLevel":true}\n' >>"$events"
  elif ! behavior_present no-event-stream; then
    printf '{"event":"turn.completed","topLevel":true}\n' >>"$events"
  fi
  if behavior_present hook-unavailable; then printf 'hooks unavailable; polling fallback used\n' >>"$log"; fi
  if behavior_present mirror-failure; then touch "$agents/$root_job/.mirror-fail-once"; fi
  argument=$(sed -n 's/^Fake-Argument:[[:space:]]*//p' "$prompt" | head -1 || true)
  [[ -z "$argument" ]] || printf 'provider argument value=%s\n' "$argument" >>"$raw"
  complete_valid
}

probe() {
  local profile=current age_days=0
  while (($#)); do
    case "$1" in
      --profile) [[ $# -ge 2 ]] || { usage; exit 2; }; profile=$2; shift 2 ;;
      --age-days) [[ $# -ge 2 ]] || { usage; exit 2; }; age_days=$2; shift 2 ;;
      *) usage; exit 2 ;;
    esac
  done
  case "$profile" in current|old|unverified-network) ;; *) usage; exit 2 ;; esac
  [[ "$age_days" =~ ^[0-9]+$ ]] || { usage; exit 2; }
  probe_fake_envelope_mechanism
  mkdir -p "$agents/capabilities"
  # The simulator's handshake window scales with measured load like every other
  # fixture ceiling; a fixed two-second default is a red gate on a busy machine.
  local handshake
  # shellcheck source=../fixture-budget.sh
  . "$root/scripts/agents/fixture-budget.sh"
  harness_fixture_budget_init "$root" || return 1
  handshake=$(harness_fixture_cap adapter-handshake) || return 1
  (( handshake <= 60 )) || handshake=60
  python3 - "$agents/capabilities" "$profile" "$age_days" "$handshake" <<'PY'
import json, re, sys
from datetime import datetime, timedelta, timezone
from pathlib import Path
directory, profile, age_days, handshake = Path(sys.argv[1]), sys.argv[2], int(sys.argv[3]), int(sys.argv[4])
captured = datetime.now(timezone.utc) - timedelta(days=age_days)
date = captured.strftime("%Y%m%d")
prefix = f"fake-fake-1-fake-config-v1-{date}-"
sequences = []
for path in directory.glob(prefix + "*.json"):
    match = re.fullmatch(re.escape(prefix) + r"(\d{3})\.json", path.name)
    if match: sequences.append(int(match.group(1)))
sequence = max(sequences, default=0) + 1
path = directory / f"{prefix}{sequence:03d}.json"
enabled = profile == "current"
capabilities = {
  "resume": enabled, "sessionEstablishedSignal": enabled,
  "nativeStructuredOutput": enabled, "nativeEvents": enabled,
  "nativeUsage": enabled, "gracefulCancel": enabled, "hooks": enabled,
  "protocolServer": enabled, "nativeBudget": enabled,
  "sessionEstablishedTimeoutSec": handshake,
}
permissions = {"unverified": ["network"] if profile == "unverified-network" else []}
envelope_enforcement = {
  "writeRoots": "mapped",
  "readRoots": "notEnforced",
  "network": "notEnforced" if profile == "unverified-network" else "mapped",
}
value = {
  "runtime": "fake", "cliVersion": "fake-1", "configHash": "fake-config-v1",
  "capturedAt": captured.strftime("%Y-%m-%dT%H:%M:%SZ"), "sequence": sequence,
  "transports": ["stdin", "file"], "capabilities": capabilities, "permissions": permissions,
  "envelopeEnforcement": envelope_enforcement, "profile": profile,
}
with path.open("x", encoding="utf-8") as handle:
    json.dump(value, handle, indent=2, sort_keys=True); handle.write("\n")
print(path)
PY
}

command=${1:-}
[[ -n "$command" ]] || { usage; exit 2; }
shift
case "$command" in
  signature)
    (($# == 0)) || { usage; exit 2; }
    printf '%s\n' \
      'match (^|[[:space:]/-])metasystem-fake-agent([[:space:]]|$)' \
      'exclude supervision-hook\.sh' \
      'exclude scripts/agents/adapters/fake\.sh'
    ;;
  identity)
    (($# == 0)) || { usage; exit 2; }
    # The fake has no runtime configuration inputs. Its deterministic identity
    # is deliberately fixed so fixtures can select snapshots without probing.
    printf 'fake-1 fake-config-v1\n'
    ;;
  probe) probe "$@" ;;
  dispatch|follow-up) supervise "$command" "$@" ;;
  cancel)
    [[ ${1:-} == --job && $# -eq 2 ]] || { usage; exit 2; }
    "$dispatch" __cancel-owned --job "$2"
    ;;
  selftest)
    (($# == 0)) || { usage; exit 2; }
    "$0" identity >/dev/null
    "$0" probe >/dev/null
    selftest_dir=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-fake-selftest.XXXXXX")
    selftest_id="fake-selftest-$(date -u +%Y%m%dt%H%M%Sz)-$$"
    sed 's/^Working Mode:.*/Working Mode: design/' "$root/scripts/agents/templates/brief.md" >"$selftest_dir/brief.md"
    "$dispatch" dispatch --role design-critic --brief "$selftest_dir/brief.md" --runtime fake --permissions none --job-id "$selftest_id" --wait
    cp "$root/scripts/agents/templates/follow-up.md" "$selftest_dir/follow.md"
    "$dispatch" follow-up --job "$selftest_id" --message "$selftest_dir/follow.md" --wait
    sed 's/^Working Mode:.*/Working Mode: design/' "$root/scripts/agents/templates/brief.md" >"$selftest_dir/cancel.md"
    printf '\nFAKE:timeout\n' >>"$selftest_dir/cancel.md"
    "$dispatch" dispatch --role design-critic --brief "$selftest_dir/cancel.md" --runtime fake --permissions none --job-id "$selftest_id-cancel" >/dev/null
    "$dispatch" cancel --job "$selftest_id-cancel"
    mkdir -p "$agents/selftests"
    python3 - "$agents/selftests/$selftest_id.json" "$selftest_id" <<'PY'
import json, sys
from datetime import datetime, timezone
from pathlib import Path
Path(sys.argv[1]).write_text(json.dumps({
  "runtime": "fake", "job": sys.argv[2], "passedAt": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
  "provenBehaviorally": [
    "dispatch", "return-validation", "resume-identity", "cancel", "usage-extraction",
    "denied-write", "denied-network",
  ],
  "constructedOnly": ["readRoots", "approvals", "tools"],
}, indent=2, sort_keys=True) + "\n")
PY
    echo "fake adapter selftest passed: full protocol sequence and denied-envelope mechanism probes"
    ;;
  -h|--help) usage ;;
  *) usage; exit 2 ;;
esac
