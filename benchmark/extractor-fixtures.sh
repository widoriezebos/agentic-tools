#!/usr/bin/env bash
set -euo pipefail

kit=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
fixture=$(mktemp -d "$kit/.extractor-fixture.XXXXXX")

cleanup() {
  case "$fixture" in
    "$kit"/.extractor-fixture.*) rm -rf -- "$fixture" ;;
    *) echo "refusing to remove unexpected fixture path: $fixture" >&2; return 1 ;;
  esac
}
trap cleanup EXIT

python3 - "$fixture" "$kit" <<'PY'
import copy
import json
import shutil
import subprocess
import sys
from pathlib import Path

fixture = Path(sys.argv[1])
root = Path(sys.argv[2])
head = subprocess.check_output(["git", "-C", str(root), "rev-parse", "HEAD"], text=True).strip()


def write_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")


spec = fixture / "spec"
write_json(spec / "manifest.json", {
    "id": "extractor-fixture",
    "version": "1.0",
    "metrics": {
        "acceptance": {"domain": [0, 1], "direction": "max", "bound": 1}
    },
    "noiseFloors": {"acceptance": 0},
})

kit_version = (root / "kit-version").read_text(encoding="utf-8").strip()

base = fixture / "base"
mission = base / "artifacts" / "agents" / "missions" / "fixture"
agents = base / "artifacts" / "agents"
base.mkdir(parents=True, exist_ok=True)
(base / "metasystem.conf").write_text(
    "role.default.runtime=fake\n"
    "role.default.model.fake=gpt-test\n"
    "role.implementer.runtime=fake\n"
    "role.implementer.model.fake=gpt-test\n",
    encoding="utf-8",
)
(mission / "mission.contract.md").parent.mkdir(parents=True, exist_ok=True)
(mission / "mission.contract.md").write_text(
    "# Mission Contract: fixture\n\n"
    "```mission\n"
    "fence.wall-clock-hours=1\n"
    "fence.cycles=2\n"
    "fence.jobs=2\n"
    "fence.concurrency=1\n"
    "fence.job-cap-min=10\n"
    "host.runtime=fake\n"
    "host.model=host-test\n"
    "host.turn-cap-min=15\n"
    "stream.build=Build the fixture.\n"
    "```\n\n"
    "```mission-seal\n"
    f"sealed.baseline.candidate-sha={head}\n"
    "```\n",
    encoding="utf-8",
)

state = {
    "schemaVersion": 1,
    "missionId": "fixture",
    "branch": "main",
    "status": "completed",
    "parkReason": None,
    "gatePassed": True,
    "streams": {"build": {"goal": "Build the fixture.", "state": "done", "reason": "done", "answeredAsk": None}},
    "fences": {"startedAt": "2026-08-05T00:00:00Z", "cycles": 1, "jobs": 1, "activeJobs": 0, "usage": []},
    "turnLog": [{
        "cycle": 1,
        "accepted": [{"kind": "dispatched", "value": {"jobId": "implementer-fixture", "role": "implementer", "stream": "build"}}],
        "certified": [{"jobId": "implementer-fixture", "verdict": "accepted", "evidence": "fixture"}],
    }],
    "waitingList": [],
    "runnerLease": None,
    "ledger": {"path": str(mission / "ledger.md"), "cycles": 1},
    "integrity": {
        "sequence": 0,
        "previousHash": None,
        "hash": "0" * 64,
        "history": [],
        "recoveryOf": None,
    },
}
write_json(mission / "state.json", state)
(mission / "ledger.md").write_text(
    "# Mission Ledger\n\n- Cycle budget: 2\n- No-gain budget: 1\n\n"
    "### Cycle 1\n- Classification: contract-improved; candidate-sha=fixture; observed=acceptance=1\n",
    encoding="utf-8",
)
write_json(mission / "fences.json", {"schemaVersion": 1, "missionId": "fixture", "cycles": 1, "startedAt": "2026-08-05T00:00:00Z", "reservations": {}})
(mission / "grader.out").write_text("metric=acceptance=1\n", encoding="utf-8")
write_json(agents / "benchmark-identity.json", {
    "schemaVersion": 1,
    "benchmarkSpecId": "extractor-fixture",
    "benchmarkSpecVersion": "1.0",
    "measuringKitVersion": kit_version,
    "candidateSha": head,
    "cohortId": "extractor-fixture-cohort",
    "repetitionIndex": 1,
    "repetitionCount": 1,
    "machineFingerprint": {
        "os": "fixture-os",
        "cpuModel": "fixture-cpu",
        "coreCount": 1,
    },
    "measuringMetasystemSha": head,
    "proposalId": None,
    "createdAt": "2026-08-05T00:00:00Z",
})

turn_dir = mission / "turns" / "fixture-t1"
write_json(turn_dir / "turn.json", {
    "cycle": 1,
    "endedAt": "2026-08-05T00:02:00Z",
    "error": None,
    "missionId": "fixture",
    "model": "host-test",
    "outcome": "completed",
    "result": {
        "outcome": "completed",
        "rawPath": str(turn_dir / "raw.out"),
        "returnPath": str(turn_dir / "return.json"),
        "sessionId": "host-session",
        "usage": {
            "availability": "native",
            "cachedInputTokens": 20,
            "cost": {"amount": 1.25, "currency": "USD"},
            "inputTokens": 10,
            "outputTokens": 30,
            "providerUnits": None,
            "reasoningTokens": None,
        },
    },
    "runtime": "fake",
    "startedAt": "2026-08-05T00:00:00Z",
    "status": "completed",
    "turnCapMin": 15,
    "turnId": "fixture-t1",
})
write_json(turn_dir / "return.json", {
    "turnId": "fixture-t1",
    "missionId": "fixture",
    "cycle": 1,
    "dispatched": [{"jobId": "implementer-fixture", "role": "implementer", "stream": "build"}],
    "certified": [{"jobId": "implementer-fixture", "verdict": "accepted", "evidence": "fixture"}],
    "streamUpdatesRequested": [{"streamId": "build", "requestedState": "done", "reason": "done"}],
    "askCandidates": [],
    "factsForLedger": ["fixture completed"],
    "gaps": [],
    "identity": {"runtime": "fake", "model": "host-test", "sessionId": None},
})
(turn_dir / "prompt.md").write_text("fixture host prompt\n", encoding="utf-8")
(turn_dir / "raw.out").write_text("fixture\n", encoding="utf-8")

job = {
    "jobId": "implementer-fixture",
    "role": "implementer",
    "mission": "fixture",
    "runtime": "fake",
    "round": 1,
    "parentJob": None,
    "status": "completed",
    "error": None,
    "capMin": 10,
    "sessionId": "delegate-session",
    "requestedModel": "gpt-test",
    "effectiveModel": "gpt-test",
    "startedAt": "2026-08-05T00:00:10Z",
    "endedAt": "2026-08-05T00:00:40Z",
    "usage": {
        "availability": "native",
        "cachedInputTokens": 5,
        "cost": {"amount": 0.5, "currency": "USD"},
        "inputTokens": 15,
        "outputTokens": 25,
        "providerUnits": None,
        "reasoningTokens": 2,
    },
    "chainClosed": True,
}
write_json(agents / "jobs" / "implementer-fixture.json", job)
round_dir = agents / "implementer-fixture" / "rounds" / "1"
write_json(round_dir / "return.json", {
    "jobId": "implementer-fixture",
    "round": 1,
    "runtime": "fake",
    "sessionId": "delegate-session",
    "model": {"requested": "gpt-test", "effective": "gpt-test"},
    "evidence": [{"command": "fixture", "observed": "pass", "level": "ran"}],
    "gaps": [],
    "mode": "implement",
    "riskiestPart": "fixture",
    "diffBoundary": ["fixture"],
    "whatWasDone": "fixture",
})
(round_dir / "prompt.md").write_text("fixture delegate prompt\n", encoding="utf-8")
(round_dir / "raw.out").write_text("fixture delegate transcript\n", encoding="utf-8")

supervision = agents / "supervision"
supervision.mkdir(parents=True, exist_ok=True)
(supervision / "census.log").write_text("CUSTODY pid=1 start=1 runtime=fake registry=fixture argv=fake\n", encoding="utf-8")
(supervision / "watcher.log").write_text("ARMED watcher fixture\n", encoding="utf-8")
write_json(supervision / "last-census.json", {
    "schemaVersion": 1,
    "writer": "watch-background-jobs.sh",
    "verdict": "SUCCESS",
    "completedAt": "2026-08-05T00:01:00Z",
    "completedAtEpoch": 1785888060,
    "durationMs": 12,
    "intervalSec": 60,
    "fingerprint": "fixture",
    "counts": {"CUSTODY": 1, "ANNOUNCED": 0, "UNTRACKED": 0},
    "inventory": [],
    "diagnostics": [],
    "errors": [],
})

variants = {
    "valid": None,
    "everyJobTerminal": "job-status",
    "everyChainClosed": "chain-open",
    "zeroUntracked": "untracked",
    "fencesEnforced": "fence",
    "delegationFloorMet": "delegation",
    "rosterPinned": "roster",
    "evidenceSetComplete": "missing-ledger",
}
for name, mutation in variants.items():
    destination = fixture / name
    shutil.copytree(base, destination)
    mission_copy = destination / "artifacts" / "agents" / "missions" / "fixture"
    agents_copy = destination / "artifacts" / "agents"
    if mutation == "job-status":
        value = json.loads((agents_copy / "jobs" / "implementer-fixture.json").read_text())
        value["status"] = "running"
        write_json(agents_copy / "jobs" / "implementer-fixture.json", value)
    elif mutation == "chain-open":
        value = json.loads((agents_copy / "jobs" / "implementer-fixture.json").read_text())
        value["chainClosed"] = False
        write_json(agents_copy / "jobs" / "implementer-fixture.json", value)
    elif mutation == "untracked":
        (agents_copy / "supervision" / "census.log").write_text("UNTRACKED pid=2 start=2 runtime=fake registry=none argv=fake\n", encoding="utf-8")
    elif mutation == "fence":
        value = json.loads((mission_copy / "state.json").read_text())
        value["fences"]["cycles"] = 3
        write_json(mission_copy / "state.json", value)
    elif mutation == "delegation":
        value = json.loads((mission_copy / "state.json").read_text())
        value["turnLog"][0]["certified"] = []
        write_json(mission_copy / "state.json", value)
    elif mutation == "roster":
        value = json.loads((agents_copy / "jobs" / "implementer-fixture.json").read_text())
        value["requestedModel"] = "wrong-model"
        value["effectiveModel"] = "wrong-model"
        write_json(agents_copy / "jobs" / "implementer-fixture.json", value)
        returned = json.loads((agents_copy / "implementer-fixture" / "rounds" / "1" / "return.json").read_text())
        returned["model"] = {"requested": "wrong-model", "effective": "wrong-model"}
        write_json(agents_copy / "implementer-fixture" / "rounds" / "1" / "return.json", returned)
    elif mutation == "missing-ledger":
        (mission_copy / "ledger.md").unlink()
PY

for name in valid everyJobTerminal everyChainClosed zeroUntracked fencesEnforced delegationFloorMet rosterPinned evidenceSetComplete; do
  mission="$fixture/$name/artifacts/agents/missions/fixture"
  out="$fixture/$name-scorecard.json"
  "$kit/extract.sh" "$mission" --spec "$fixture/spec" --out "$out" >/dev/null
  python3 - "$out" "$name" <<'PY'
import json
import sys
from pathlib import Path

scorecard = json.loads(Path(sys.argv[1]).read_text())
variant = sys.argv[2]
gates = {gate["name"]: gate["passed"] for gate in scorecard["runValidity"]["gates"]}
if variant == "valid":
    assert scorecard["runValidity"]["valid"] is True, scorecard["runValidity"]
    assert all(value is True for value in gates.values()), gates
    assert scorecard["identity"]["measuringKitVersion"] == "0.1.0"
    assert scorecard["identity"]["cohortId"] == "extractor-fixture-cohort"
    assert scorecard["identity"]["candidateSha"] is not None
    assert scorecard["machineFingerprint"]["cpuModel"] == "fixture-cpu"
else:
    assert scorecard["runValidity"]["valid"] is False, scorecard["runValidity"]
    assert gates[variant] is False, gates
    expected_false = {variant}
    if variant == "everyJobTerminal":
        # The same sole implementer cannot be both non-terminal and satisfy the
        # completed-and-certified delegation floor.
        expected_false.add("delegationFloorMet")
    assert {name for name, value in gates.items() if value is not True} == expected_false, gates
assert Path(sys.argv[1]).with_suffix(".md").is_file()
PY
done

echo "benchmark extractor fixtures: PASS"
