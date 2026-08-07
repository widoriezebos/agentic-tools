#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  benchmark/run-cohort.sh --spec <spec-id-or-dir> --repetitions <N> [--proposal <id>]
  benchmark/run-cohort.sh --resume <cohort-id>

The first form creates a cohort and provisions its first fresh target. Each
invocation stops at one human seal/sign boundary. After the printed contract
has an Approval line, invoke the printed --resume command. A resumed cohort
runs that repetition through grading and extraction, then either provisions
the next fresh target or completes the cohort.
USAGE
}

die() { echo "$2" >&2; exit "$1"; }

kit=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
top=$(cd "$kit/.." && pwd -P)
results=$kit/results
# Cohort runs live under the same trials root as ad-hoc provisioning:
# $METASYSTEM_TRIALS_ROOT, else benchmark/trials-root.local, else the
# kit-local .runs directory (the historical default).
runs=$(python3 - "$kit" <<'PY'
import os, sys
from pathlib import Path
kit = Path(sys.argv[1])
env = os.environ.get("METASYSTEM_TRIALS_ROOT", "").strip()
local = kit / "trials-root.local"
if env:
    print(Path(env).expanduser() / "cohorts")
elif local.is_file():
    lines = local.read_text().splitlines()
    line = lines[0].strip() if lines and lines[0].strip() else ""
    print((Path(line).expanduser() / "cohorts") if line else (kit / ".runs"))
else:
    print(kit / ".runs")
PY
)
spec_arg=
proposal_id=
repetitions=
resume_id=

while (($#)); do
  case "$1" in
    --spec) [[ $# -ge 2 ]] || { usage; exit 2; }; spec_arg=$2; shift 2 ;;
    --proposal) [[ $# -ge 2 ]] || { usage; exit 2; }; proposal_id=$2; shift 2 ;;
    --repetitions) [[ $# -ge 2 ]] || { usage; exit 2; }; repetitions=$2; shift 2 ;;
    --resume) [[ $# -ge 2 ]] || { usage; exit 2; }; resume_id=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) usage; exit 2 ;;
  esac
done

if [[ -n "$resume_id" ]]; then
  [[ -z "$spec_arg" && -z "$proposal_id" && -z "$repetitions" ]] || { usage; exit 2; }
  [[ "$resume_id" =~ ^[a-z0-9][a-z0-9-]*$ ]] || die 2 "cohort resume refused: invalid cohort id"
else
  [[ -n "$spec_arg" && -n "$repetitions" ]] || { usage; exit 2; }
  [[ "$repetitions" =~ ^[1-9][0-9]*$ ]] || die 2 "cohort start refused: repetitions must be a positive integer"
  [[ -z "$proposal_id" || "$proposal_id" =~ ^[a-z0-9][a-z0-9-]*$ ]] \
    || die 2 "cohort start refused: invalid proposal id"
fi

atomic_state() { # state path, phase, repetition index
  python3 - "$1" "$2" "$3" <<'PY'
import json
import os
import sys
import tempfile
from pathlib import Path

path = Path(sys.argv[1])
value = json.loads(path.read_text(encoding="utf-8"))
value["phase"] = sys.argv[2]
value["repetitionIndex"] = int(sys.argv[3])
descriptor, temporary = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
with os.fdopen(descriptor, "w", encoding="utf-8") as handle:
    json.dump(value, handle, indent=2, sort_keys=True)
    handle.write("\n")
    handle.flush()
    os.fsync(handle.fileno())
os.replace(temporary, path)
PY
}

prepare_repetition() { # cohort id, spec dir, repetition, state path
  local cohort_id=$1 spec=$2 repetition=$3 state_path=$4
  local run_dir=$runs/$cohort_id target=$runs/$cohort_id/targets/$repetition
  local record=$results/cohorts/$cohort_id.json provision_log=$run_dir/provision-$repetition.out
  [[ ! -e "$target" && ! -L "$target" ]] \
    || die 1 "cohort provision refused: target already exists: $target"
  mkdir -p "$run_dir/targets"
  if ! "$kit/provision.sh" --spec "$spec" --target "$target" >"$provision_log"; then
    die 1 "cohort provision failed: see $provision_log"
  fi

  mkdir -p "$target/artifacts/agents"
  python3 - "$record" "$target/artifacts/agents/benchmark-identity.json" "$repetition" <<'PY'
import datetime as dt
import json
import os
import sys
import tempfile
from pathlib import Path

cohort = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
output = Path(sys.argv[2])
value = {
    "schemaVersion": 1,
    "benchmarkSpecId": cohort["benchmarkSpecId"],
    "benchmarkSpecVersion": cohort["benchmarkSpecVersion"],
    "measuringKitVersion": cohort["measuringKitVersion"],
    "candidateSha": cohort["candidateSha"],
    "cohortId": cohort["cohortId"],
    "repetitionIndex": int(sys.argv[3]),
    "repetitionCount": cohort["repetitionCount"],
    "machineFingerprint": cohort["machineFingerprint"],
    "measuringMetasystemSha": cohort["measuringMetasystemSha"],
    "proposalId": cohort["proposalId"],
    "createdAt": dt.datetime.now(dt.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
}
descriptor, temporary = tempfile.mkstemp(prefix=f".{output.name}.", dir=output.parent)
with os.fdopen(descriptor, "w", encoding="utf-8") as handle:
    json.dump(value, handle, indent=2, sort_keys=True)
    handle.write("\n")
    handle.flush()
    os.fsync(handle.fileno())
os.replace(temporary, output)
PY

  local mission_id contract_rel contract
  mission_id=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8"))["id"])' "$spec/manifest.json")
  contract_rel=plans/mission-$mission_id.contract.md
  contract=$target/$contract_rel
  atomic_state "$state_path" awaiting-approval "$repetition"
  printf 'Review %s\n' "$contract"
  printf 'Seal it: (cd %q && scripts/assert-mission.sh --seal --file %q)\n' "$target" "$contract_rel"
  printf 'Sign it: add an Approval line using the printed hash, then commit and push the signed contract.\n'
  printf 'Resume it: %q --resume %q\n' "$kit/run-cohort.sh" "$cohort_id"
}

if [[ -z "$resume_id" ]]; then
  if [[ -d "$spec_arg" ]]; then
    spec=$(cd "$spec_arg" && pwd -P)
  else
    spec=$kit/specs/$spec_arg
    [[ -d "$spec" ]] || die 2 "cohort start refused: unknown spec: $spec_arg"
    spec=$(cd "$spec" && pwd -P)
  fi
  case "$spec" in
    "$kit"/specs/*) ;;
    *) die 2 "cohort start refused: spec must be under $kit/specs" ;;
  esac
  manifest=$spec/manifest.json
  [[ -f "$manifest" ]] || die 2 "cohort start refused: spec manifest is missing: $manifest"
  kit_version=$(tr -d '\r\n' <"$kit/kit-version")
  [[ -n "$kit_version" && $(wc -l <"$kit/kit-version" | tr -d ' ') == 1 ]] \
    || die 1 "cohort start refused: kit-version must contain one non-empty line"
  candidate_sha=$(git -C "$top" log -1 --format=%H HEAD -- . ':(exclude)benchmark/results/**')
  [[ "$candidate_sha" =~ ^[0-9a-f]{40}$ ]] || die 1 "cohort start refused: candidate sha is unavailable"
  machine=$($kit/system-fingerprint.py)
  created_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  spec_id=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8"))["id"])' "$manifest")
  [[ "$spec_id" =~ ^[a-z0-9][a-z0-9-]*$ ]] || die 2 "cohort start refused: manifest id is invalid"
  if [[ -n "$proposal_id" ]]; then
    [[ -f "$results/proposals/$proposal_id.json" ]] \
      || die 2 "cohort start refused: proposal does not exist: $proposal_id"
  fi
  cohort_id=$spec_id-$(date -u +%Y%m%dt%H%M%Sz)-$$
  record=$results/cohorts/$cohort_id.json
  state_path=$runs/$cohort_id/state.json
  [[ ! -e "$record" && ! -e "$state_path" ]] || die 1 "cohort start refused: generated cohort id already exists"
  mkdir -p "$results/cohorts" "$runs/$cohort_id"
  python3 - "$manifest" "$record" "$state_path" "$cohort_id" "$kit_version" \
    "$candidate_sha" "$proposal_id" "$repetitions" "$machine" "$created_at" <<'PY'
import json
import os
import sys
import tempfile
from pathlib import Path

manifest = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
record_path = Path(sys.argv[2])
state_path = Path(sys.argv[3])
proposal = sys.argv[7] or None
record = {
    "schemaVersion": 1,
    "cohortId": sys.argv[4],
    "benchmarkSpecId": manifest["id"],
    "benchmarkSpecVersion": manifest["version"],
    "measuringKitVersion": sys.argv[5],
    "proposalId": proposal,
    "repetitionCount": int(sys.argv[8]),
    "machineFingerprint": json.loads(sys.argv[9]),
    "roster": manifest["roster"],
    "fences": manifest["fences"],
    "candidateSha": sys.argv[6],
    "measuringMetasystemSha": sys.argv[6],
    "createdAt": sys.argv[10],
}
state = {
    "schemaVersion": 1,
    "cohortId": record["cohortId"],
    "specId": record["benchmarkSpecId"],
    "proposalId": proposal,
    "repetitionCount": record["repetitionCount"],
    "candidateSha": record["candidateSha"],
    "createdAt": record["createdAt"],
    "phase": "new",
    "repetitionIndex": 1,
}
for path, value in ((record_path, record), (state_path, state)):
    descriptor, temporary = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    with os.fdopen(descriptor, "w", encoding="utf-8") as handle:
        json.dump(value, handle, indent=2, sort_keys=True)
        handle.write("\n")
        handle.flush()
        os.fsync(handle.fileno())
    os.replace(temporary, path)
PY
  prepare_repetition "$cohort_id" "$spec" 1 "$state_path"
  exit 0
fi

cohort_id=$resume_id
state_path=$runs/$cohort_id/state.json
record=$results/cohorts/$cohort_id.json
[[ -f "$state_path" && -f "$record" ]] || die 2 "cohort resume refused: unknown cohort: $cohort_id"
state_facts=$(python3 - "$state_path" <<'PY'
import json, sys
value = json.load(open(sys.argv[1], encoding="utf-8"))
for name in ("specId", "phase", "repetitionIndex", "repetitionCount", "candidateSha"):
    print(value[name])
PY
)
spec_id=$(printf '%s\n' "$state_facts" | sed -n '1p')
phase=$(printf '%s\n' "$state_facts" | sed -n '2p')
repetition=$(printf '%s\n' "$state_facts" | sed -n '3p')
repetition_count=$(printf '%s\n' "$state_facts" | sed -n '4p')
candidate_sha=$(printf '%s\n' "$state_facts" | sed -n '5p')
spec=$kit/specs/$spec_id
manifest=$spec/manifest.json
[[ -f "$manifest" ]] || die 1 "cohort resume refused: recorded spec is unavailable: $spec_id"
target=$runs/$cohort_id/targets/$repetition
mission_id=$spec_id
contract=$target/plans/mission-$mission_id.contract.md

if [[ "$phase" == complete ]]; then
  printf 'Cohort complete: %s\n' "$record"
  exit 0
fi
if [[ "$phase" == new ]]; then
  prepare_repetition "$cohort_id" "$spec" "$repetition" "$state_path"
  exit 0
fi
[[ -d "$target" ]] || die 1 "cohort resume refused: repetition target is missing: $target"

if [[ "$phase" == awaiting-approval ]]; then
  [[ -f "$contract" ]] || die 1 "cohort resume refused: mission contract is missing: $contract"
  grep -qE '^Approval:[[:space:]]' "$contract" \
    || die 1 "cohort resume refused: repetition $repetition contract has no Approval line"
  if [[ ! -f "$target/artifacts/agents/missions/$mission_id/state.json" ]]; then
    if ! (cd "$target" && scripts/agents/mission-runner.sh start --mission "$mission_id"); then
      die 1 "cohort resume refused: mission start failed for repetition $repetition"
    fi
  fi
  atomic_state "$state_path" mission-running "$repetition"
  phase=mission-running
fi

if [[ "$phase" == mission-running ]]; then
  while true; do
    set +e
    status_output=$(cd "$target" && scripts/agents/mission-runner.sh status --mission "$mission_id" 2>&1)
    status_code=$?
    set -e
    case "$status_code" in
      0) sleep 2 ;;
      10|11) break ;;
      *) die 1 "cohort wait failed for repetition $repetition: $status_output" ;;
    esac
  done
  atomic_state "$state_path" grading "$repetition"
  phase=grading
fi

if [[ "$phase" == grading ]]; then
  mission_root=$target/artifacts/agents/missions/$mission_id
  grader_rel=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8"))["grader"]["path"])' "$manifest")
  grader=$spec/$grader_rel/grade.sh
  [[ -x "$grader" ]] || die 1 "cohort grading refused: held-out grader is not executable: $grader"
  grader_tmp=$mission_root/.grader.out.$$
  set +e
  "$grader" "$target" >"$grader_tmp" 2>"$mission_root/grader.err"
  grader_code=$?
  set -e
  if ((grader_code != 0)); then
    die 1 "cohort grading failed for repetition $repetition: see $mission_root/grader.err"
  fi
  mv "$grader_tmp" "$mission_root/grader.out"
  scorecard_dir=$results/$candidate_sha/$cohort_id
  scorecard=$scorecard_dir/$repetition.json
  [[ ! -e "$scorecard" ]] || die 1 "cohort extraction refused: scorecard already exists: $scorecard"
  mkdir -p "$scorecard_dir"
  "$kit/extract.sh" "$target" --spec "$spec" --out "$scorecard" >"$runs/$cohort_id/extract-$repetition.out"
  python3 - "$scorecard" "$cohort_id" "$repetition" "$repetition_count" "$candidate_sha" <<'PY'
import json
import sys
value = json.load(open(sys.argv[1], encoding="utf-8"))["identity"]
expected = {
    "cohortId": sys.argv[2],
    "repetitionIndex": int(sys.argv[3]),
    "repetitionCount": int(sys.argv[4]),
    "candidateSha": sys.argv[5],
}
if any(value.get(name) != fact for name, fact in expected.items()):
    raise SystemExit("cohort extraction refused: scorecard identity differs from stamped identity")
PY
  if ((repetition < repetition_count)); then
    next_repetition=$((repetition + 1))
    atomic_state "$state_path" new "$next_repetition"
    prepare_repetition "$cohort_id" "$spec" "$next_repetition" "$state_path"
  else
    atomic_state "$state_path" complete "$repetition"
    printf 'Cohort complete: %s\n' "$record"
  fi
  exit 0
fi

die 1 "cohort resume refused: unknown state phase: $phase"
