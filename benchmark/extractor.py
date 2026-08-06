#!/usr/bin/env python3
"""Turn one mission evidence tree into the canonical benchmark scorecard."""

from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import re
import statistics
import subprocess
import sys
import tempfile
from collections import Counter, defaultdict
from decimal import Decimal, InvalidOperation
from pathlib import Path
from typing import Any


BENCHMARK_DIR = Path(__file__).resolve().parent
SCHEMA_DIR = BENCHMARK_DIR / "schemas"
EVIDENCE_SCHEMA_DIR = SCHEMA_DIR / "evidence"
TERMINAL_JOB_STATUSES = {"completed", "failed", "timeout", "cancelled"}
CRITIC_ROLES = {"design-critic", "code-critic"}
LEDGER_CLASSES = ("contract-improved", "unresolved", "no-progress")
METRIC_LINE = re.compile(r"^(metric|watch)=([a-z0-9][a-z0-9_-]*)=(-?(?:\d+(?:\.\d*)?|\.\d+))$")


class ExtractionError(RuntimeError):
    pass


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        prog="benchmark/extract.sh",
        usage="benchmark/extract.sh <run-evidence-root> --spec <spec-dir> --out <scorecard.json>",
    )
    parser.add_argument("run_evidence_root", type=Path)
    parser.add_argument("--spec", required=True, type=Path)
    parser.add_argument("--out", required=True, type=Path)
    args = parser.parse_args()
    if args.out.suffix != ".json":
        parser.error("--out must name a .json file")
    return args


def json_type_matches(value: Any, expected: str) -> bool:
    return {
        "object": isinstance(value, dict),
        "array": isinstance(value, list),
        "string": isinstance(value, str),
        "integer": isinstance(value, int) and not isinstance(value, bool),
        "number": isinstance(value, (int, float)) and not isinstance(value, bool),
        "boolean": isinstance(value, bool),
        "null": value is None,
    }.get(expected, False)


def schema_violations(value: Any, schema: dict[str, Any], path: str = "$") -> list[str]:
    violations: list[str] = []
    expected = schema.get("type")
    if expected is not None:
        choices = expected if isinstance(expected, list) else [expected]
        if not any(json_type_matches(value, choice) for choice in choices):
            return [f"{path} must be {' or '.join(choices)}"]
    if "const" in schema and value != schema["const"]:
        violations.append(f"{path} must equal {schema['const']!r}")
    if "enum" in schema and value not in schema["enum"]:
        violations.append(f"{path} is not one of the allowed values")
    if isinstance(value, str):
        if "minLength" in schema and len(value) < schema["minLength"]:
            violations.append(f"{path} is shorter than {schema['minLength']}")
        if "pattern" in schema and re.fullmatch(schema["pattern"], value) is None:
            violations.append(f"{path} does not match {schema['pattern']!r}")
        if schema.get("format") == "date-time":
            try:
                parse_time(value)
            except (TypeError, ValueError):
                violations.append(f"{path} is not an ISO-8601 date-time")
    if isinstance(value, (int, float)) and not isinstance(value, bool):
        if "minimum" in schema and value < schema["minimum"]:
            violations.append(f"{path} is below minimum {schema['minimum']}")
    if isinstance(value, dict):
        properties = schema.get("properties", {})
        for name in schema.get("required", []):
            if name not in value:
                violations.append(f"{path}.{name} is required")
        if schema.get("additionalProperties") is False:
            for name in value:
                if name not in properties:
                    violations.append(f"{path}.{name} is not allowed")
        additional = schema.get("additionalProperties")
        for name, child in value.items():
            if name in properties:
                violations.extend(schema_violations(child, properties[name], f"{path}.{name}"))
            elif isinstance(additional, dict):
                violations.extend(schema_violations(child, additional, f"{path}.{name}"))
        property_names = schema.get("propertyNames")
        if isinstance(property_names, dict):
            for name in value:
                violations.extend(schema_violations(name, property_names, f"{path}.<property>") )
        if "minProperties" in schema and len(value) < schema["minProperties"]:
            violations.append(f"{path} has fewer than {schema['minProperties']} properties")
    if isinstance(value, list):
        if "items" in schema:
            for index, child in enumerate(value):
                violations.extend(schema_violations(child, schema["items"], f"{path}[{index}]"))
        if schema.get("uniqueItems"):
            encoded = [json.dumps(child, sort_keys=True) for child in value]
            if len(encoded) != len(set(encoded)):
                violations.append(f"{path} must contain unique items")
    return violations


def read_schema(name: str) -> dict[str, Any]:
    path = SCHEMA_DIR / name
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as error:
        raise ExtractionError(f"pinned schema is unreadable: {path}: {error}") from error


def parse_time(raw: str) -> dt.datetime:
    if raw.endswith("Z"):
        raw = raw[:-1] + "+00:00"
    parsed = dt.datetime.fromisoformat(raw)
    if parsed.tzinfo is None:
        raise ValueError("timestamp has no timezone")
    return parsed.astimezone(dt.timezone.utc)


def decimal_number(raw: str) -> int | float:
    value = Decimal(raw)
    if not value.is_finite():
        raise InvalidOperation(raw)
    if value == value.to_integral_value():
        return int(value)
    return float(value)


def ratio(used: int | float, ceiling: int | float | None) -> float | None:
    if ceiling is None or ceiling == 0:
        return None
    return float(Decimal(str(used)) / Decimal(str(ceiling)))


def camel_case(raw: str) -> str:
    parts = raw.split("_")
    return parts[0] + "".join(part[:1].upper() + part[1:] for part in parts[1:])


def median_number(values: list[int | float]) -> int | float | None:
    if not values:
        return None
    result = statistics.median(values)
    return int(result) if float(result).is_integer() else result


def run_git(repo: Path, *arguments: str) -> str:
    result = subprocess.run(
        ["git", "-C", str(repo), *arguments],
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=False,
    )
    if result.returncode != 0:
        raise ExtractionError((result.stderr or result.stdout).strip() or "git command failed")
    return result.stdout.strip()


def locate_mission_root(root: Path) -> Path:
    if (root / "state.json").is_file() or root.name:
        if (root / "state.json").exists() or (root / "ledger.md").exists() or (root / "turns").exists():
            return root
    candidates = sorted(root.glob("missions/*/state.json")) + sorted(root.glob("artifacts/agents/missions/*/state.json"))
    if len(candidates) == 1:
        return candidates[0].parent
    raise ExtractionError("run evidence root does not identify exactly one mission")


def locate_layout(mission_root: Path) -> tuple[Path, Path]:
    parts = mission_root.parts
    if len(parts) >= 4 and parts[-3:-1] == ("agents", "missions"):
        agents_root = mission_root.parent.parent
        target_root = agents_root.parent.parent
        return agents_root, target_root
    if mission_root.parent.name == "missions":
        agents_root = mission_root.parent.parent
        return agents_root, agents_root.parent.parent
    agents_root = mission_root / "artifacts" / "agents"
    return agents_root, mission_root


def locate_contract(mission_root: Path, target_root: Path, mission_id: str) -> Path | None:
    candidates = (
        mission_root / "mission.contract.md",
        mission_root / "contract.md",
        target_root / f"mission-{mission_id}.contract.md",
        target_root / "plans" / f"mission-{mission_id}.contract.md",
    )
    return next((path for path in candidates if path.is_file()), None)


def parse_contract(path: Path) -> tuple[dict[str, str], dict[str, str]]:
    text = path.read_text(encoding="utf-8")
    blocks = re.findall(r"```mission\s*\n(.*?)\n```", text, flags=re.DOTALL)
    if len(blocks) != 1:
        raise ExtractionError("mission contract must contain exactly one mission block")
    values: dict[str, str] = {}
    for line_number, raw in enumerate(blocks[0].splitlines(), 1):
        if not raw.strip() or raw.lstrip().startswith("#"):
            continue
        if "=" not in raw:
            raise ExtractionError(f"mission contract line {line_number} has no equals sign")
        name, value = raw.split("=", 1)
        if name in values:
            raise ExtractionError(f"mission contract repeats {name}")
        values[name] = value
    sealed_blocks = re.findall(r"```mission-seal\s*\n(.*?)\n```", text, flags=re.DOTALL)
    sealed: dict[str, str] = {}
    if len(sealed_blocks) == 1:
        for raw in sealed_blocks[0].splitlines():
            if "=" in raw:
                name, value = raw.split("=", 1)
                sealed[name] = value
    return values, sealed


def read_config(path: Path) -> dict[str, str]:
    values: dict[str, str] = {}
    for raw in path.read_text(encoding="utf-8").splitlines():
        if not raw.strip() or raw.lstrip().startswith("#") or "=" not in raw:
            continue
        name, value = raw.split("=", 1)
        values[name.strip()] = value.strip()
    return values


def resolve_roster(config: dict[str, str], contract: dict[str, str]) -> dict[str, Any]:
    roles = set()
    for name in config:
        match = re.fullmatch(r"role\.([a-z0-9-]+)\.runtime", name)
        if match and match.group(1) != "default":
            roles.add(match.group(1))
    delegates = []
    for role in sorted(roles):
        runtime = config.get(f"role.{role}.runtime", config.get("role.default.runtime"))
        if not runtime or runtime == "main":
            continue
        model = config.get(f"role.{role}.model.{runtime}", config.get(f"role.default.model.{runtime}"))
        delegates.append({"role": role, "runtime": runtime, "model": model})
    return {
        "host": {"runtime": contract.get("host.runtime"), "model": contract.get("host.model")},
        "delegates": delegates,
    }


class Extractor:
    def __init__(self, run_root: Path, spec_dir: Path) -> None:
        self.requested_root = run_root.resolve()
        self.spec_dir = spec_dir.resolve()
        self.mission_root = locate_mission_root(self.requested_root)
        self.agents_root, self.target_root = locate_layout(self.mission_root)
        self.gaps: list[str] = []
        self.evidence_errors: list[str] = []
        self.state: dict[str, Any] | None = None
        self.contract: dict[str, str] = {}
        self.sealed: dict[str, str] = {}
        self.manifest: dict[str, Any] | None = None
        self.turns: list[dict[str, Any]] = []
        self.jobs: list[dict[str, Any]] = []
        self.job_returns: dict[str, dict[str, Any]] = {}
        self.git_root: Path | None = None

    def gap(self, name: str, reason: str) -> None:
        message = f"{name}: {reason}"
        if message not in self.gaps:
            self.gaps.append(message)

    def evidence_error(self, label: str, reason: str) -> None:
        message = f"{label}: {reason}"
        if message not in self.evidence_errors:
            self.evidence_errors.append(message)
        self.gap(f"evidence.{label}", reason)

    def load_json(self, path: Path, label: str, schema_name: str | None = None) -> dict[str, Any] | None:
        try:
            value = json.loads(path.read_text(encoding="utf-8"))
        except FileNotFoundError:
            self.evidence_error(label, "file is missing")
            return None
        except (OSError, json.JSONDecodeError) as error:
            self.evidence_error(label, f"file is unreadable JSON: {error}")
            return None
        if not isinstance(value, dict):
            self.evidence_error(label, "JSON root is not an object")
            return None
        if schema_name:
            schema = read_schema(f"evidence/{schema_name}")
            violations = schema_violations(value, schema)
            if violations:
                self.evidence_error(label, "; ".join(violations[:8]))
                return None
        return value

    def load_inputs(self) -> None:
        state_path = self.mission_root / "state.json"
        self.state = self.load_json(state_path, "missionState", "mission-state.schema.json")
        mission_id = self.state.get("missionId") if self.state else self.mission_root.name
        contract_path = locate_contract(self.mission_root, self.target_root, str(mission_id))
        if contract_path is None:
            self.evidence_error("missionContract", "file is missing")
        else:
            try:
                self.contract, self.sealed = parse_contract(contract_path)
            except (OSError, ExtractionError) as error:
                self.evidence_error("missionContract", str(error))

        manifest_path = self.spec_dir / "manifest.json"
        if not self.spec_dir.is_dir():
            self.evidence_error("benchmarkSpec", "spec directory is missing")
        else:
            self.manifest = self.load_json(manifest_path, "benchmarkSpec")
            if self.manifest is not None and not isinstance(self.manifest.get("id"), str):
                self.evidence_error("benchmarkSpec", "manifest id is missing")
                self.manifest = None
            elif self.manifest is not None and not isinstance(self.manifest.get("version"), str):
                self.evidence_error("benchmarkSpec", "manifest version is missing")
                self.manifest = None

        turns_dir = self.mission_root / "turns"
        if not turns_dir.is_dir():
            self.evidence_error("turns", "turns directory is missing")
        else:
            for turn_dir in sorted(path for path in turns_dir.iterdir() if path.is_dir()):
                turn = self.load_json(turn_dir / "turn.json", f"turns.{turn_dir.name}.turn", "turn.schema.json")
                returned = self.load_json(turn_dir / "return.json", f"turns.{turn_dir.name}.return", "orchestrator.schema.json")
                if not (turn_dir / "prompt.md").is_file():
                    self.evidence_error(f"turns.{turn_dir.name}.prompt", "prompt.md is missing")
                if turn is not None:
                    turn["_directory"] = turn_dir
                    turn["_return"] = returned
                    self.turns.append(turn)
            if not self.turns:
                self.evidence_error("turns", "no parseable turn records exist")

        mission_id = self.state.get("missionId") if self.state else self.mission_root.name
        jobs_dir = self.agents_root / "jobs"
        if jobs_dir.is_dir():
            for path in sorted(jobs_dir.glob("*.json")):
                try:
                    raw = json.loads(path.read_text(encoding="utf-8"))
                except (OSError, json.JSONDecodeError):
                    # An unjoinable record is not attributable to this mission.
                    continue
                if not isinstance(raw, dict) or raw.get("mission") != mission_id:
                    continue
                violations = schema_violations(raw, read_schema("evidence/job-record.schema.json"))
                if violations:
                    self.evidence_error(f"jobs.{path.stem}", "; ".join(violations[:8]))
                    continue
                self.jobs.append(raw)
        expected_jobs = self.state.get("fences", {}).get("jobs") if self.state else None
        if isinstance(expected_jobs, int) and expected_jobs > 0 and not self.jobs:
            self.evidence_error("jobs", "mission state records jobs but no matching job record exists")
        self.load_job_returns()

        try:
            root = run_git(self.target_root, "rev-parse", "--show-toplevel")
            self.git_root = Path(root)
        except ExtractionError as error:
            self.evidence_error("targetGitHistory", str(error))

        ledger_path = self.mission_root / "ledger.md"
        if not ledger_path.is_file():
            self.evidence_error("ledger", "file is missing")
        census_log = self.agents_root / "supervision" / "census.log"
        last_census = self.agents_root / "supervision" / "last-census.json"
        if not census_log.is_file() and not last_census.is_file():
            self.evidence_error("census", "neither census.log nor last-census.json exists")
        watcher_log = self.agents_root / "supervision" / "watcher.log"
        if not watcher_log.is_file():
            self.evidence_error("watcher", "watcher.log is missing")
        grader_path = self.mission_root / "grader.out"
        if not grader_path.is_file():
            self.evidence_error("graderOutput", "grader.out is missing")

    def load_job_returns(self) -> None:
        by_id = {job.get("jobId"): job for job in self.jobs if isinstance(job.get("jobId"), str)}
        for job in self.jobs:
            job_id = job.get("jobId")
            role = job.get("role")
            round_number = job.get("round")
            if not isinstance(job_id, str) or not isinstance(role, str) or not isinstance(round_number, int):
                continue
            if job.get("status") != "completed":
                continue
            root = job
            seen = {job_id}
            while root.get("parentJob") is not None:
                parent = root.get("parentJob")
                if not isinstance(parent, str) or parent in seen or parent not in by_id:
                    self.evidence_error(f"jobs.{job_id}.chain", "parent chain is missing or cyclic")
                    break
                seen.add(parent)
                root = by_id[parent]
            root_id = root.get("jobId")
            round_dir = self.agents_root / str(root_id) / "rounds" / str(round_number)
            return_path = round_dir / "return.json"
            if not (round_dir / "prompt.md").is_file():
                self.evidence_error(f"jobs.{job_id}.prompt", "prompt.md is missing")
            if not (round_dir / "raw.out").is_file():
                self.evidence_error(f"jobs.{job_id}.transcript", "raw.out is missing")
            schema_name = f"{role}.schema.json"
            if not (EVIDENCE_SCHEMA_DIR / schema_name).is_file():
                self.evidence_error(f"jobs.{job_id}.return", f"no pinned schema exists for role {role}")
                continue
            returned = self.load_json(return_path, f"jobs.{job_id}.return", schema_name)
            if returned is None:
                continue
            mismatches = [name for name in ("jobId", "round", "runtime", "sessionId") if returned.get(name) != job.get(name)]
            if mismatches:
                self.evidence_error(f"jobs.{job_id}.return", "identity mismatch at " + ", ".join(mismatches))
                continue
            model = returned.get("model")
            if isinstance(model, dict) and (
                model.get("requested") != job.get("requestedModel")
                or model.get("effective") != job.get("effectiveModel")
            ):
                self.evidence_error(f"jobs.{job_id}.return", "model identity mismatch")
                continue
            if role in CRITIC_ROLES:
                findings = returned.get("findings", [])
                material_count = sum(1 for finding in findings if finding.get("material") is True)
                if returned.get("verdictMaterialCount") != material_count:
                    self.evidence_error(f"jobs.{job_id}.return", "verdictMaterialCount does not match findings")
                    continue
            self.job_returns[job_id] = returned

    def identity(self) -> tuple[dict[str, Any], dict[str, Any] | None]:
        mission_id = self.state.get("missionId") if self.state else self.mission_root.name
        candidate_sha = None
        if self.git_root is not None:
            try:
                candidate_sha = run_git(
                    self.git_root,
                    "log",
                    "-1",
                    "--format=%H",
                    "HEAD",
                    "--",
                    ".",
                    ":(exclude)benchmark/results/**",
                ) or None
            except ExtractionError as error:
                self.gap("identity.candidateSha", str(error))
        if candidate_sha is None:
            self.gap("identity.candidateSha", "target Git history does not provide a candidate commit")

        config_path = self.target_root / "metasystem.conf"
        roster = {"host": {"runtime": self.contract.get("host.runtime"), "model": self.contract.get("host.model")}, "delegates": []}
        if config_path.is_file():
            try:
                roster = resolve_roster(read_config(config_path), self.contract)
            except OSError as error:
                self.gap("identity.roster", f"target metasystem.conf is unreadable: {error}")
        else:
            self.gap("identity.roster", "target metasystem.conf is missing")

        fence_fields = {
            "cycles": "fence.cycles",
            "jobs": "fence.jobs",
            "concurrency": "fence.concurrency",
            "jobCapMinutes": "fence.job-cap-min",
            "wallClockHours": "fence.wall-clock-hours",
            "hostTurnCapMinutes": "host.turn-cap-min",
        }
        fences: dict[str, int | float | None] = {}
        for output_name, contract_name in fence_fields.items():
            raw = self.contract.get(contract_name)
            try:
                fences[output_name] = decimal_number(raw) if raw is not None else None
            except (InvalidOperation, ValueError):
                fences[output_name] = None
            if fences[output_name] is None:
                self.gap(f"identity.fences.{output_name}", f"mission contract has no numeric {contract_name}")

        identity = {
            "missionId": mission_id,
            "benchmarkSpecId": self.manifest.get("id") if self.manifest else None,
            "benchmarkSpecVersion": self.manifest.get("version") if self.manifest else None,
            "measuringKitVersion": None,
            "candidateSha": candidate_sha,
            "cohortId": None,
            "repetitionIndex": None,
            "repetitionCount": None,
            "roster": roster,
            "fences": fences,
            "measuringMetasystemSha": None,
        }
        self.gap("identity.measuringKitVersion", "the evidence set has no measuring-kit version record")
        self.gap("identity.cohortId", "the evidence set has no cohort record")
        self.gap("identity.repetitionIndex", "the evidence set has no repetition index")
        self.gap("identity.repetitionCount", "the evidence set has no repetition count")
        self.gap("identity.measuringMetasystemSha", "the evidence set has no measuring-metasystem commit record")
        if self.manifest is None:
            self.gap("identity.benchmarkSpec", "benchmark spec id and version are unavailable")

        machine = None
        self.gap("machineFingerprint", "OS, CPU model, and core count were not logged with the run")
        return identity, machine

    def job_chains(self) -> dict[str, list[dict[str, Any]]]:
        by_id = {job.get("jobId"): job for job in self.jobs}
        chains: dict[str, list[dict[str, Any]]] = defaultdict(list)
        for job in self.jobs:
            root = job
            seen = set()
            while isinstance(root.get("parentJob"), str) and root["parentJob"] in by_id and root["parentJob"] not in seen:
                seen.add(root["parentJob"])
                root = by_id[root["parentJob"]]
            chains[str(root.get("jobId"))].append(job)
        for records in chains.values():
            records.sort(key=lambda value: value.get("round", 0))
        return dict(chains)

    def tracking_metric(self) -> tuple[dict[str, Any], bool | None, str]:
        census_log = self.agents_root / "supervision" / "census.log"
        last_census_path = self.agents_root / "supervision" / "last-census.json"
        findings = 0
        sources = 0
        if census_log.is_file():
            sources += 1
            try:
                findings += sum(1 for line in census_log.read_text(encoding="utf-8").splitlines() if line.startswith("UNTRACKED "))
            except OSError as error:
                self.evidence_error("census", f"census.log is unreadable: {error}")
        if last_census_path.is_file():
            census = self.load_json(last_census_path, "lastCensus", "census.schema.json")
            if census is not None:
                sources += 1
                # census.log is the historical stream. Count the last snapshot only when it is the sole source.
                if not census_log.is_file():
                    findings += int(census["counts"]["UNTRACKED"])
        if sources == 0:
            return ({"name": "tracking", "measurementClass": "validity", "sourceOwner": "candidate", "untrackedFindingCount": None}, None, "census evidence is unavailable")
        return (
            {"name": "tracking", "measurementClass": "validity", "sourceOwner": "candidate", "untrackedFindingCount": findings},
            findings == 0,
            f"observed {findings} UNTRACKED census finding(s)",
        )

    def fence_metric(self, identity: dict[str, Any]) -> tuple[dict[str, Any], bool | None, str]:
        fences = identity["fences"]
        state_fences = self.state.get("fences", {}) if self.state else {}
        cycles = state_fences.get("cycles")
        jobs_used = state_fences.get("jobs")
        start = None
        end = None
        try:
            start = parse_time(state_fences.get("startedAt"))
            end_values = []
            for item in self.turns + self.jobs:
                if isinstance(item.get("endedAt"), str):
                    end_values.append(parse_time(item["endedAt"]))
            end = max(end_values) if end_values else None
        except (TypeError, ValueError):
            pass
        wall_seconds = int((end - start).total_seconds()) if start is not None and end is not None else None

        intervals: list[tuple[dt.datetime, dt.datetime]] = []
        job_wall_clock = []
        job_cap_ok = True
        for job in self.jobs:
            try:
                began = parse_time(job["startedAt"])
                ended = parse_time(job["endedAt"])
            except (KeyError, TypeError, ValueError):
                job_cap_ok = False
                continue
            intervals.append((began, ended))
            cap = job.get("capMin")
            seconds = int((ended - began).total_seconds())
            contract_cap = fences.get("jobCapMinutes")
            job_wall_clock.append({
                "jobId": job.get("jobId"),
                "wallClockSeconds": seconds,
                "wallClockSecondsCeiling": contract_cap * 60 if isinstance(contract_cap, (int, float)) else None,
                "wallClockSecondsUsedRatio": ratio(seconds, contract_cap * 60) if isinstance(contract_cap, (int, float)) else None,
            })
            if (
                not isinstance(cap, (int, float))
                or not isinstance(contract_cap, (int, float))
                or cap > contract_cap
                or seconds > contract_cap * 60
            ):
                job_cap_ok = False
        events: list[tuple[dt.datetime, int]] = []
        for began, ended in intervals:
            events.extend(((began, 1), (ended, -1)))
        active = 0
        peak = 0
        for _, change in sorted(events, key=lambda item: (item[0], item[1])):
            active += change
            peak = max(peak, active)

        host_cap_ok = True
        host_turn_wall_clock = []
        for turn in self.turns:
            try:
                seconds = (parse_time(turn["endedAt"]) - parse_time(turn["startedAt"])).total_seconds()
            except (KeyError, TypeError, ValueError):
                host_cap_ok = False
                continue
            cap = turn.get("turnCapMin")
            contract_cap = fences.get("hostTurnCapMinutes")
            host_turn_wall_clock.append({
                "turnId": turn.get("turnId"),
                "wallClockSeconds": int(seconds),
                "wallClockSecondsCeiling": contract_cap * 60 if isinstance(contract_cap, (int, float)) else None,
                "wallClockSecondsUsedRatio": ratio(seconds, contract_cap * 60) if isinstance(contract_cap, (int, float)) else None,
            })
            if (
                not isinstance(cap, (int, float))
                or not isinstance(contract_cap, (int, float))
                or cap > contract_cap
                or seconds > contract_cap * 60
            ):
                host_cap_ok = False

        comparisons = (
            isinstance(cycles, int) and isinstance(fences.get("cycles"), (int, float)) and cycles <= fences["cycles"],
            isinstance(jobs_used, int) and isinstance(fences.get("jobs"), (int, float)) and jobs_used <= fences["jobs"],
            isinstance(fences.get("concurrency"), (int, float)) and peak <= fences["concurrency"],
            wall_seconds is not None and isinstance(fences.get("wallClockHours"), (int, float)) and wall_seconds <= fences["wallClockHours"] * 3600,
            job_cap_ok,
            host_cap_ok,
        )
        passed = all(comparisons) if all(value is not None for value in comparisons) else None
        metric = {
            "name": "fenceEconomy",
            "measurementClass": "watch",
            "sourceOwner": "candidate",
            "cyclesUsed": cycles,
            "cyclesCeiling": fences.get("cycles"),
            "cyclesUsedRatio": ratio(cycles, fences.get("cycles")) if isinstance(cycles, int) else None,
            "jobsUsed": jobs_used,
            "jobsCeiling": fences.get("jobs"),
            "jobsUsedRatio": ratio(jobs_used, fences.get("jobs")) if isinstance(jobs_used, int) else None,
            "peakConcurrentJobs": peak,
            "concurrencyCeiling": fences.get("concurrency"),
            "candidateWallClockSeconds": wall_seconds,
            "wallClockSecondsCeiling": fences.get("wallClockHours") * 3600 if isinstance(fences.get("wallClockHours"), (int, float)) else None,
            "wallClockSecondsUsedRatio": ratio(wall_seconds, fences.get("wallClockHours") * 3600) if wall_seconds is not None and isinstance(fences.get("wallClockHours"), (int, float)) else None,
            "kitWallClockSeconds": None,
            "jobCapMinutes": fences.get("jobCapMinutes"),
            "hostTurnCapMinutes": fences.get("hostTurnCapMinutes"),
            "jobWallClockSecondsByJob": job_wall_clock,
            "hostTurnWallClockSecondsByTurn": host_turn_wall_clock,
        }
        self.gap("mechanicalBehavior.fenceEconomy.kitWallClockSeconds", "the benchmark runner did not log its kit-side wall clock")
        return metric, passed, "all observable cycle, job, concurrency, wall-clock, and per-process caps were enforced" if passed else "one or more observable fence limits were exceeded or unavailable"

    def delegation_metric(self, chains: dict[str, list[dict[str, Any]]]) -> tuple[dict[str, Any], bool | None, str]:
        streams = sorted(self.state.get("streams", {})) if self.state else []
        dispatched: dict[str, list[str]] = defaultdict(list)
        certified: set[str] = set()
        if self.state:
            for turn in self.state.get("turnLog", []):
                for accepted in turn.get("accepted", []):
                    if accepted.get("kind") == "dispatched":
                        value = accepted.get("value", {})
                        if value.get("role") == "implementer" and isinstance(value.get("stream"), str) and isinstance(value.get("jobId"), str):
                            dispatched[value["stream"]].append(value["jobId"])
                for item in turn.get("certified", []):
                    if item.get("verdict") == "accepted" and isinstance(item.get("jobId"), str):
                        certified.add(item["jobId"])
        jobs_by_id = {job.get("jobId"): job for job in self.jobs}
        results = []
        for stream in streams:
            qualifying = []
            for job_id in dispatched.get(stream, []):
                job = jobs_by_id.get(job_id)
                if job and job.get("status") == "completed" and job_id in certified:
                    qualifying.append(job_id)
            results.append({"streamId": stream, "qualifyingImplementerJobIds": sorted(qualifying), "met": bool(qualifying)})
        passed = all(item["met"] for item in results) if results else None
        if not streams:
            self.gap("mechanicalBehavior.delegationFloor", "mission state has no streams to join")
        metric = {
            "name": "delegationFloor",
            "measurementClass": "validity",
            "sourceOwner": "candidate",
            "streams": results,
        }
        failed = [item["streamId"] for item in results if not item["met"]]
        detail = "at least one completed and certified implementer job exists per stream" if passed else "streams without a completed and certified implementer job: " + ", ".join(failed)
        return metric, passed, detail

    def roster_gate(self, identity: dict[str, Any]) -> tuple[bool | None, str]:
        expected = {item["role"]: item for item in identity["roster"]["delegates"]}
        problems = []
        for turn in self.turns:
            host = identity["roster"]["host"]
            if turn.get("runtime") != host.get("runtime") or turn.get("model") != host.get("model"):
                problems.append(f"host turn {turn.get('turnId')} differs from pinned host roster")
        for job in self.jobs:
            role = job.get("role")
            resolution = expected.get(role)
            if resolution is None:
                problems.append(f"job {job.get('jobId')} role {role} has no pinned roster resolution")
            elif job.get("runtime") != resolution.get("runtime") or job.get("requestedModel") != resolution.get("model"):
                problems.append(f"job {job.get('jobId')} requested runtime/model differs from roster")
            if job.get("effectiveModel") != job.get("requestedModel"):
                problems.append(f"job {job.get('jobId')} effective model differs from requested model")
        if not identity["roster"]["host"].get("runtime") or not identity["roster"]["host"].get("model"):
            return None, "host roster is unavailable"
        return not problems, "; ".join(problems) if problems else "host and every job match the pinned roster"

    def validity(self, identity: dict[str, Any], chains: dict[str, list[dict[str, Any]]], tracking_passed: bool | None, tracking_detail: str, fence_passed: bool | None, fence_detail: str, delegation_passed: bool | None, delegation_detail: str) -> dict[str, Any]:
        all_terminal = all(job.get("status") in TERMINAL_JOB_STATUSES for job in self.jobs)
        open_chains = [root for root, records in chains.items() if records[0].get("chainClosed") is not True]
        all_closed = not open_chains
        roster_passed, roster_detail = self.roster_gate(identity)
        evidence_passed = not self.evidence_errors
        gates = [
            {"name": "everyJobTerminal", "passed": all_terminal, "sourceOwner": "candidate", "detail": "every mission job is terminal" if all_terminal else "one or more mission jobs are non-terminal"},
            {"name": "everyChainClosed", "passed": all_closed, "sourceOwner": "candidate", "detail": "every mission job chain is closed" if all_closed else "open chains: " + ", ".join(open_chains)},
            {"name": "zeroUntracked", "passed": tracking_passed, "sourceOwner": "candidate", "detail": tracking_detail},
            {"name": "fencesEnforced", "passed": fence_passed, "sourceOwner": "candidate", "detail": fence_detail},
            {"name": "delegationFloorMet", "passed": delegation_passed, "sourceOwner": "candidate", "detail": delegation_detail},
            {"name": "rosterPinned", "passed": roster_passed, "sourceOwner": "candidate", "detail": roster_detail},
            {"name": "evidenceSetComplete", "passed": evidence_passed, "sourceOwner": "kit", "detail": "all required evidence parsed under the pinned schemas" if evidence_passed else "; ".join(self.evidence_errors)},
        ]
        reasons = [f"{gate['name']}: {gate['detail']}" for gate in gates if gate["passed"] is not True]
        return {"valid": not reasons, "reasons": reasons, "gates": gates}

    def product_metrics(self) -> tuple[list[dict[str, Any]], list[dict[str, Any]]]:
        grader_path = self.mission_root / "grader.out"
        values: dict[str, int | float] = {}
        watch_values: dict[str, int | float] = {}
        if grader_path.is_file():
            try:
                for line_number, raw in enumerate(grader_path.read_text(encoding="utf-8").splitlines(), 1):
                    if not raw.strip():
                        continue
                    match = METRIC_LINE.fullmatch(raw.strip())
                    if match is None:
                        self.evidence_error("graderOutput", f"line {line_number} does not use metric/watch grammar")
                        continue
                    try:
                        number = decimal_number(match.group(3))
                    except InvalidOperation:
                        self.evidence_error("graderOutput", f"line {line_number} is not finite")
                        continue
                    target = values if match.group(1) == "metric" else watch_values
                    target[match.group(2)] = number
            except OSError as error:
                self.evidence_error("graderOutput", f"grader.out is unreadable: {error}")

        declared = self.manifest.get("metrics", {}) if self.manifest else {}
        noises = self.manifest.get("noiseFloors") if self.manifest else None
        noises = noises if isinstance(noises, dict) else {}
        metrics = []
        for raw_name, definition in declared.items() if isinstance(declared, dict) else []:
            definition = definition if isinstance(definition, dict) else {}
            direction = definition.get("direction") if definition.get("direction") in {"min", "max"} else None
            bound = definition.get("bound") if isinstance(definition.get("bound"), (int, float)) and not isinstance(definition.get("bound"), bool) else None
            noise = noises.get(raw_name) if isinstance(noises.get(raw_name), (int, float)) and not isinstance(noises.get(raw_name), bool) else None
            observed = values.get(raw_name)
            verdict = "no-verdict"
            if observed is not None and bound is not None and direction == "max":
                verdict = "pass" if observed >= bound else "fail"
            elif observed is not None and bound is not None and direction == "min":
                verdict = "pass" if observed <= bound else "fail"
            metric = {
                "name": camel_case(raw_name),
                "rawName": raw_name,
                "measurementClass": "constraint",
                "sourceOwner": "kit",
                "value": observed,
                "unit": "seconds" if raw_name.endswith("_seconds") else "count" if raw_name.endswith("_count") else "ratio" if definition.get("domain") == [0.0, 1.0] or definition.get("domain") == [0, 1] else "scalar",
                "direction": direction,
                "floor": bound if direction == "max" else None,
                "ceiling": bound if direction == "min" else None,
                "noiseFloor": noise,
                "verdict": verdict,
            }
            metrics.append(metric)
            if observed is None:
                self.gap(f"productMetrics.{metric['name']}.value", "grader output does not contain the declared metric")
            if direction is None or bound is None:
                self.gap(f"productMetrics.{metric['name']}.floorOrCeiling", "spec manifest has no executable direction and bound")
            if noise is None:
                self.gap(f"productMetrics.{metric['name']}.noiseFloor", "spec manifest has no noise floor")

        extras = []
        for raw_name in sorted(set(values) - set(declared)):
            extras.append({
                "name": camel_case(raw_name),
                "rawName": raw_name,
                "measurementClass": "watch",
                "sourceOwner": "kit",
                "value": values[raw_name],
                "unit": "seconds" if raw_name.endswith("_seconds") else "count" if raw_name.endswith("_count") else "scalar",
            })
        for raw_name, value in sorted(watch_values.items()):
            extras.append({"name": camel_case(raw_name), "rawName": raw_name, "measurementClass": "watch", "sourceOwner": "kit", "value": value, "unit": "count" if raw_name.endswith("_count") else "scalar"})
        if self.manifest is None:
            self.gap("productMetrics", "no spec manifest is available")
        return metrics, extras

    def protocol_and_rework(self, chains: dict[str, list[dict[str, Any]]]) -> list[dict[str, Any]]:
        if not chains:
            protocol_ratio = None
            max_followups = None
            followups: list[dict[str, Any]] = []
            self.gap("mechanicalBehavior.protocolConformance.ratio", "no delegate job returns exist")
            self.gap("mechanicalBehavior.rework.maximumFollowUpRoundsPerJob", "no delegate job chains exist")
        else:
            conforming = 0
            followups = []
            for root, records in sorted(chains.items()):
                no_protocol_retry = all(job.get("error") != "protocol_error" for job in records)
                all_returns_pass = all(job.get("jobId") in self.job_returns for job in records)
                if no_protocol_retry and all_returns_pass:
                    conforming += 1
                followups.append({"jobId": root, "followUpRounds": max(0, len(records) - 1)})
            protocol_ratio = conforming / len(chains)
            max_followups = max(item["followUpRounds"] for item in followups)
        self.gap("mechanicalBehavior.protocolConformance.floor", "the kit has no recorded protocol-conformance floor")
        self.gap("mechanicalBehavior.protocolConformance.noiseFloor", "the kit has no recorded protocol-conformance noise floor")
        self.gap("mechanicalBehavior.rework.ceiling", "the kit has no recorded rework ceiling")
        self.gap("mechanicalBehavior.rework.noiseFloor", "the kit has no recorded rework noise floor")
        return [
            {
                "name": "protocolConformance",
                "measurementClass": "constraint",
                "sourceOwner": "candidate",
                "ratio": protocol_ratio,
                "direction": "max",
                "floor": None,
                "ceiling": None,
                "noiseFloor": None,
            },
            {
                "name": "rework",
                "measurementClass": "constraint",
                "sourceOwner": "candidate",
                "maximumFollowUpRoundsPerJob": max_followups,
                "followUpRoundsByJob": followups,
                "direction": "min",
                "floor": None,
                "ceiling": None,
                "noiseFloor": None,
            },
        ]

    def critique_metric(self, chains: dict[str, list[dict[str, Any]]]) -> dict[str, Any]:
        critic_roots = [root for root, records in chains.items() if records and records[0].get("role") in CRITIC_ROLES]
        closed_by_join = None if critic_roots else 0
        if critic_roots:
            self.gap("mechanicalBehavior.critiqueDiscipline.closedByJoinCount", "critic job records do not log the findings-to-dispositions join result")
        return {
            "name": "critiqueDiscipline",
            "measurementClass": "watch",
            "sourceOwner": "candidate",
            "critiqueChainCount": len(critic_roots),
            "closedByJoinCount": closed_by_join,
        }

    def progress_metric(self) -> dict[str, Any]:
        path = self.mission_root / "ledger.md"
        classifications: list[str] = []
        if path.is_file():
            try:
                for raw in path.read_text(encoding="utf-8").splitlines():
                    match = re.match(r"^- Classification: ([a-z-]+)(?:;|$)", raw)
                    if match:
                        classifications.append(match.group(1))
            except OSError as error:
                self.evidence_error("ledger", f"file is unreadable: {error}")
        unknown = sorted(set(classifications) - set(LEDGER_CLASSES))
        if unknown:
            self.evidence_error("ledger", "unknown classifications: " + ", ".join(unknown))
        counts = Counter(classifications)
        longest = current = 0
        for classification in classifications:
            current = current + 1 if classification == "no-progress" else 0
            longest = max(longest, current)
        return {
            "name": "progressShape",
            "measurementClass": "watch",
            "sourceOwner": "candidate",
            "contractImprovedCount": counts["contract-improved"],
            "unresolvedCount": counts["unresolved"],
            "noProgressCount": counts["no-progress"],
            "longestNoProgressStreakCycles": longest,
        }

    def costs(self) -> list[dict[str, Any]]:
        grouped: dict[tuple[str, str], dict[str, Any]] = {}
        sources: list[tuple[str, str, Any]] = []
        for turn in self.turns:
            usage = turn.get("result", {}).get("usage")
            if isinstance(usage, dict):
                sources.append((str(turn.get("runtime")), "orchestrator", usage))
        for job in self.jobs:
            if isinstance(job.get("usage"), dict):
                sources.append((str(job.get("runtime")), str(job.get("role")), job["usage"]))
        for provider, role, usage in sources:
            target = grouped.setdefault((provider, role), {"provider": provider, "role": role, "amounts": defaultdict(Decimal), "tokens": Counter(), "providerUnits": defaultdict(Decimal)})
            cost = usage.get("cost")
            if isinstance(cost, dict) and isinstance(cost.get("currency"), str) and isinstance(cost.get("amount"), (int, float)):
                target["amounts"][cost["currency"]] += Decimal(str(cost["amount"]))
            for field in ("inputTokens", "cachedInputTokens", "outputTokens", "reasoningTokens"):
                if isinstance(usage.get(field), int):
                    target["tokens"][field] += usage[field]
            provider_units = usage.get("providerUnits")
            if isinstance(provider_units, dict):
                for unit, value in provider_units.items():
                    if isinstance(unit, str) and isinstance(value, (int, float)):
                        target["providerUnits"][unit] += Decimal(str(value))
        result = []
        for (_, _), value in sorted(grouped.items()):
            result.append({
                "provider": value["provider"],
                "role": value["role"],
                "measurementClass": "constraint",
                "sourceOwner": "candidate",
                "amounts": [{"currency": currency, "amount": decimal_number(str(amount))} for currency, amount in sorted(value["amounts"].items())],
                "tokens": dict(sorted(value["tokens"].items())),
                "providerUnits": [{"unit": unit, "value": decimal_number(str(amount))} for unit, amount in sorted(value["providerUnits"].items())],
                "direction": "min",
                "ceiling": None,
                "noiseFloor": None,
            })
            self.gap(f"costPerProvider.{value['provider']}.{value['role']}.ceiling", "the kit/spec has no provider-and-role cost ceiling")
            self.gap(f"costPerProvider.{value['provider']}.{value['role']}.noiseFloor", "the kit/spec has no provider-and-role cost noise floor")
        if not result:
            self.gap("costPerProvider", "no native usage record supplies cost or typed provider units")
        return result

    def prompt_watch(self) -> dict[str, Any]:
        paths = [turn["_directory"] / "prompt.md" for turn in self.turns]
        for job in self.jobs:
            root = job
            by_id = {item.get("jobId"): item for item in self.jobs}
            while isinstance(root.get("parentJob"), str) and root["parentJob"] in by_id:
                root = by_id[root["parentJob"]]
            paths.append(self.agents_root / str(root.get("jobId")) / "rounds" / str(job.get("round")) / "prompt.md")
        sizes = []
        for path in paths:
            try:
                sizes.append(path.stat().st_size)
            except OSError:
                self.gap("watches.assembledPromptBytes", f"prompt is missing: {path.name}")
        return {
            "name": "assembledPromptBytes",
            "measurementClass": "watch",
            "sourceOwner": "candidate",
            "promptCount": len(sizes),
            "totalBytes": sum(sizes),
            "minimumBytes": min(sizes) if sizes else None,
            "medianBytes": median_number(sizes),
            "maximumBytes": max(sizes) if sizes else None,
        }

    def census_watch(self) -> dict[str, Any]:
        path = self.agents_root / "supervision" / "last-census.json"
        values: list[int] = []
        if path.is_file():
            census = self.load_json(path, "lastCensus", "census.schema.json")
            if census is not None:
                start = self.state.get("fences", {}).get("startedAt") if self.state else None
                end_values = [turn.get("endedAt") for turn in self.turns if isinstance(turn.get("endedAt"), str)]
                try:
                    inside = parse_time(start) <= parse_time(census["completedAt"]) <= max(parse_time(value) for value in end_values)
                except (KeyError, TypeError, ValueError):
                    inside = False
                if inside:
                    values.append(census["durationMs"])
        if not values:
            self.gap("watches.censusScanMilliseconds", "no timestamped census duration falls inside the run window")
        return {
            "name": "censusScanMilliseconds",
            "measurementClass": "watch",
            "sourceOwner": "candidate",
            "scanCount": len(values),
            "minimumMilliseconds": min(values) if values else None,
            "medianMilliseconds": median_number(values),
            "maximumMilliseconds": max(values) if values else None,
        }

    def commit_watch(self, candidate_sha: str | None) -> dict[str, Any]:
        sizes: list[int] = []
        commits: list[str] = []
        base = self.sealed.get("sealed.baseline.candidate-sha")
        if self.git_root is not None and candidate_sha and base:
            try:
                commits = run_git(self.git_root, "log", "--format=%H", "--reverse", f"{base}..{candidate_sha}").splitlines()
                for commit in commits:
                    raw = run_git(self.git_root, "show", "--numstat", "--format=", commit)
                    changed = 0
                    for line in raw.splitlines():
                        columns = line.split("\t", 2)
                        if len(columns) >= 2 and columns[0].isdigit() and columns[1].isdigit():
                            changed += int(columns[0]) + int(columns[1])
                    sizes.append(changed)
            except ExtractionError as error:
                self.gap("watches.commitShape", str(error))
        else:
            self.gap("watches.commitShape", "target Git history or sealed baseline commit is unavailable")
        return {
            "name": "commitShape",
            "measurementClass": "watch",
            "sourceOwner": "kit",
            "commitCount": len(commits) if commits or (base and candidate_sha) else None,
            "changedLinesPerCommit": sizes,
            "minimumChangedLines": min(sizes) if sizes else None,
            "medianChangedLines": median_number(sizes),
            "maximumChangedLines": max(sizes) if sizes else None,
            "totalChangedLines": sum(sizes) if sizes else (0 if commits == [] and base and candidate_sha else None),
        }

    def build(self) -> dict[str, Any]:
        self.load_inputs()
        identity, machine = self.identity()
        chains = self.job_chains()
        tracking, tracking_passed, tracking_detail = self.tracking_metric()
        fence, fence_passed, fence_detail = self.fence_metric(identity)
        delegation, delegation_passed, delegation_detail = self.delegation_metric(chains)
        product, grader_watches = self.product_metrics()
        mechanical = self.protocol_and_rework(chains)
        mechanical.extend((tracking, self.critique_metric(chains), delegation, fence, self.progress_metric()))
        costs = self.costs()
        watches = grader_watches + [
            self.prompt_watch(),
            self.census_watch(),
            self.commit_watch(identity["candidateSha"]),
            {"name": "validationSuiteWallClockSeconds", "measurementClass": "watch", "sourceOwner": "kit", "wallClockSeconds": None},
            {"name": "metasystemLineCount", "measurementClass": "watch", "sourceOwner": "kit", "lineCount": None},
        ]
        self.gap("watches.validationSuiteWallClockSeconds", "validation-suite wall clock was not logged with the run")
        self.gap("watches.metasystemLineCount", "the evidence set has no pinned metasystem source-path list for line counting")
        self.gap("judgedScores", "behavior-judge output is absent; the judged layer is not estimated by the mechanical extractor")
        validity = self.validity(identity, chains, tracking_passed, tracking_detail, fence_passed, fence_detail, delegation_passed, delegation_detail)
        return {
            "schemaVersion": 1,
            "identity": identity,
            "runValidity": validity,
            "productMetrics": product,
            "mechanicalBehaviorMetrics": mechanical,
            "judgedScores": [],
            "judge": None,
            "costPerProvider": costs,
            "machineFingerprint": machine,
            "watches": watches,
            "gaps": self.gaps,
        }


def format_cell(value: Any) -> str:
    if value is None:
        return "—"
    if isinstance(value, bool):
        return "yes" if value else "no"
    if isinstance(value, (dict, list)):
        return "`" + json.dumps(value, sort_keys=True, separators=(",", ":")) + "`"
    return str(value).replace("|", "\\|")


def markdown_projection(scorecard: dict[str, Any]) -> str:
    identity = scorecard["identity"]
    lines = [
        f"# Benchmark scorecard: {identity['missionId']}",
        "",
        f"**Run validity:** {'valid' if scorecard['runValidity']['valid'] else 'invalid'}",
        "",
        "## Identity",
        "",
        "| Field | Value |",
        "| --- | --- |",
    ]
    for name in ("benchmarkSpecId", "benchmarkSpecVersion", "measuringKitVersion", "candidateSha", "cohortId", "repetitionIndex", "repetitionCount", "measuringMetasystemSha"):
        lines.append(f"| {name} | {format_cell(identity[name])} |")
    lines += ["", "## Run-validity gates", "", "| Gate | Passed | Source owner | Detail |", "| --- | --- | --- | --- |"]
    for gate in scorecard["runValidity"]["gates"]:
        lines.append(f"| {gate['name']} | {format_cell(gate['passed'])} | {gate['sourceOwner']} | {format_cell(gate['detail'])} |")
    lines += ["", "## Product constraints", "", "| Metric | Value | Direction | Floor | Ceiling | Noise floor | Verdict |", "| --- | ---: | --- | ---: | ---: | ---: | --- |"]
    for metric in scorecard["productMetrics"]:
        lines.append(f"| {metric['name']} | {format_cell(metric['value'])} | {format_cell(metric['direction'])} | {format_cell(metric['floor'])} | {format_cell(metric['ceiling'])} | {format_cell(metric['noiseFloor'])} | {metric['verdict']} |")
    lines += ["", "## Mechanical behavior", "", "| Metric | Class | Source owner | Measurement |", "| --- | --- | --- | --- |"]
    for metric in scorecard["mechanicalBehaviorMetrics"]:
        measurement = {name: value for name, value in metric.items() if name not in {"name", "measurementClass", "sourceOwner"}}
        lines.append(f"| {metric['name']} | {metric['measurementClass']} | {metric['sourceOwner']} | {format_cell(measurement)} |")
    lines += ["", "## Costs by provider and role", "", "| Provider | Role | Amounts | Tokens |", "| --- | --- | --- | --- |"]
    for cost in scorecard["costPerProvider"]:
        lines.append(f"| {cost['provider']} | {cost['role']} | {format_cell(cost['amounts'])} | {format_cell(cost['tokens'])} |")
    lines += ["", "## Watches", "", "| Watch | Source owner | Measurement |", "| --- | --- | --- |"]
    for watch in scorecard["watches"]:
        measurement = {name: value for name, value in watch.items() if name not in {"name", "measurementClass", "sourceOwner"}}
        lines.append(f"| {watch['name']} | {watch['sourceOwner']} | {format_cell(measurement)} |")
    lines += ["", "## Logging gaps", ""]
    if scorecard["gaps"]:
        lines.extend(f"- {gap}" for gap in scorecard["gaps"])
    else:
        lines.append("None.")
    return "\n".join(lines) + "\n"


def atomic_write(path: Path, text: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor, temporary = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    try:
        with os.fdopen(descriptor, "w", encoding="utf-8") as handle:
            handle.write(text)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
    except Exception:
        try:
            os.unlink(temporary)
        except OSError:
            pass
        raise


def main() -> int:
    args = parse_args()
    run_root = args.run_evidence_root.resolve()
    if not run_root.is_dir():
        print(f"extractor error: run evidence root is not a directory: {run_root}", file=sys.stderr)
        return 2
    try:
        scorecard = Extractor(run_root, args.spec).build()
        violations = schema_violations(scorecard, read_schema("scorecard.schema.json"))
        if violations:
            raise ExtractionError("generated scorecard violates its schema: " + "; ".join(violations[:12]))
        output = args.out.resolve()
        markdown = output.with_suffix(".md")
        atomic_write(markdown, markdown_projection(scorecard))
        atomic_write(output, json.dumps(scorecard, indent=2, sort_keys=True) + "\n")
    except (ExtractionError, OSError, ValueError) as error:
        print(f"extractor error: {error}", file=sys.stderr)
        return 1
    print(output)
    print(markdown)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
