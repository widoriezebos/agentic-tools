#!/usr/bin/env python3
"""Compare two explicit benchmark cohorts without network access."""

from __future__ import annotations

import argparse
import datetime as dt
import json
import math
import os
import re
import statistics
import subprocess
import sys
import tempfile
from decimal import Decimal
from pathlib import Path
from typing import Any

from extractor import read_schema, schema_violations


KIT = Path(__file__).resolve().parent
TOP = KIT.parent
RESULTS = KIT / "results"
ID_RE = re.compile(r"^[a-z0-9][a-z0-9-]*$")
SHA_RE = re.compile(r"^[0-9a-f]{40}$")


class CompareError(RuntimeError):
    pass


def arguments() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        prog="benchmark/compare.sh",
        usage="benchmark/compare.sh <baseline-cohort-id> <candidate-cohort-id>\n"
              "       benchmark/compare.sh --configurations <cohort-id> <cohort-id> [...]   (report, no verdict)",
    )
    parser.add_argument("--configurations", action="store_true",
                        help="report several cohorts of the SAME case version under DIFFERENT configurations side by side; never a verdict")
    parser.add_argument("cohort_ids", nargs="+")
    parsed = parser.parse_args()
    for value in parsed.cohort_ids:
        if ID_RE.fullmatch(value) is None:
            parser.error(f"invalid cohort id: {value}")
    if parsed.configurations:
        if len(parsed.cohort_ids) < 2:
            parser.error("--configurations needs at least two cohort ids")
        return parsed
    if len(parsed.cohort_ids) != 2:
        parser.error("exactly two cohort ids: <baseline-cohort-id> <candidate-cohort-id>")
    parsed.baseline_cohort_id, parsed.candidate_cohort_id = parsed.cohort_ids
    if parsed.baseline_cohort_id == parsed.candidate_cohort_id:
        parser.error("baseline and candidate cohort ids must differ")
    return parsed


def load_object(path: Path, label: str) -> dict[str, Any]:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except FileNotFoundError as error:
        raise CompareError(f"{label} is missing: {path}") from error
    except (OSError, json.JSONDecodeError) as error:
        raise CompareError(f"{label} is unreadable JSON: {error}") from error
    if not isinstance(value, dict):
        raise CompareError(f"{label} root is not an object")
    return value


def timestamp(raw: Any, label: str) -> dt.datetime:
    if not isinstance(raw, str):
        raise CompareError(f"{label} is not a timestamp")
    try:
        parsed = dt.datetime.fromisoformat(raw.replace("Z", "+00:00"))
    except ValueError as error:
        raise CompareError(f"{label} is not an ISO-8601 timestamp") from error
    if parsed.tzinfo is None:
        raise CompareError(f"{label} has no timezone")
    return parsed.astimezone(dt.timezone.utc)


def git(*args: str, check: bool = True) -> subprocess.CompletedProcess[str]:
    result = subprocess.run(
        ["git", "-C", str(TOP), *args],
        check=False,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if check and result.returncode != 0:
        detail = (result.stderr or result.stdout).strip() or "git command failed"
        raise CompareError(detail)
    return result


def number(value: Any) -> bool:
    return isinstance(value, (int, float)) and not isinstance(value, bool) and math.isfinite(value)


def median(values: list[int | float]) -> int | float:
    result = statistics.median(values)
    return int(result) if float(result).is_integer() else result


def subtract(left: int | float, right: int | float) -> int | float:
    result = Decimal(str(left)) - Decimal(str(right))
    return int(result) if result == result.to_integral_value() else float(result)


def canonical(value: Any) -> str:
    return json.dumps(value, sort_keys=True, separators=(",", ":"))


def atomic_text(path: Path, value: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor, temporary = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    try:
        with os.fdopen(descriptor, "w", encoding="utf-8") as handle:
            handle.write(value)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
    except Exception:
        try:
            os.unlink(temporary)
        except OSError:
            pass
        raise


PAIR_FIELDS = ("caseId", "caseVersion", "caseTree", "configId", "configVersion", "configTree")


def aliases() -> dict[str, Any]:
    try:
        return json.loads((KIT / "aliases.json").read_text(encoding="utf-8")).get("aliases", {})
    except (OSError, json.JSONDecodeError):
        return {}


def normalize_pair(value: dict[str, Any], side: str) -> dict[str, Any]:
    """A schema-2 record names the pair and its pins. A schema-1 record names
    a retired spec id: it is resolved through aliases.json to the pair it ran,
    keeps legacyId/legacyVersionLabel, and has no pins (None) — such a record
    can never be verdict-compared against a pinned one, only against another
    legacy record of the same alias (the tuple carries the None pins)."""
    if value.get("schemaVersion") == 2:
        return value
    legacy = value.get("benchmarkSpecId")
    entry = aliases().get(legacy)
    if entry is None:
        raise CompareError(f"{side} cohort record names an unknown legacy spec id {legacy!r}: no alias resolves it")
    value = dict(value)
    value.update({"caseId": entry["case"], "caseVersion": entry["caseVersion"], "caseTree": None,
                  "configId": entry["config"], "configVersion": entry["configVersion"], "configTree": None,
                  "legacyId": legacy, "legacyVersionLabel": value.get("benchmarkSpecVersion")})
    return value


def cohort_record(cohort_id: str, side: str) -> dict[str, Any]:
    value = load_object(RESULTS / "cohorts" / f"{cohort_id}.json", f"{side} cohort record")
    version = value.get("schemaVersion")
    if version == 2:
        required = {"schemaVersion", "cohortId", *PAIR_FIELDS, "measuringKitVersion", "proposalId", "repetitionCount", "machineFingerprint", "roster", "createdAt"}
    else:
        required = {"schemaVersion", "cohortId", "benchmarkSpecId", "benchmarkSpecVersion", "measuringKitVersion", "proposalId", "repetitionCount", "machineFingerprint", "roster", "createdAt"}
    missing = sorted(required - set(value))
    if missing:
        raise CompareError(f"{side} cohort record is missing: {', '.join(missing)}")
    if version not in (1, 2) or value.get("cohortId") != cohort_id:
        raise CompareError(f"{side} cohort record identity is invalid")
    value = normalize_pair(value, side)
    count = value.get("repetitionCount")
    if not isinstance(count, int) or isinstance(count, bool) or count < 1:
        raise CompareError(f"{side} cohort repetitionCount is invalid")
    if not isinstance(value.get("machineFingerprint"), dict) or not isinstance(value.get("roster"), dict):
        raise CompareError(f"{side} cohort machine fingerprint or roster is invalid")
    timestamp(value.get("createdAt"), f"{side} cohort createdAt")
    return value


def scorecard_directory(cohort_id: str, side: str) -> tuple[str, Path]:
    matches: list[tuple[str, Path]] = []
    if RESULTS.is_dir():
        for candidate in RESULTS.iterdir():
            if candidate.is_dir() and SHA_RE.fullmatch(candidate.name):
                path = candidate / cohort_id
                if path.is_dir():
                    matches.append((candidate.name, path))
    if len(matches) != 1:
        raise CompareError(f"{side} cohort must resolve to exactly one candidate-sha directory")
    return matches[0]


def comparability_tuple(card: dict[str, Any]) -> dict[str, Any]:
    """Design §3: the pair (case and configuration versions) and their pinned
    object ids, plus everything the tuple always carried. Equal tuples are
    necessary for a metasystem verdict; nothing here loosens the old checks."""
    identity = card["identity"]
    return {
        "caseId": identity.get("caseId"),
        "caseVersion": identity.get("caseVersion"),
        "caseTree": identity.get("caseTree"),
        "configId": identity.get("configId"),
        "configVersion": identity.get("configVersion"),
        "configTree": identity.get("configTree"),
        "measuringKitVersion": identity["measuringKitVersion"],
        "roster": identity["roster"],
        "fences": identity["fences"],
        "repetitionCount": identity["repetitionCount"],
        "machineFingerprint": card["machineFingerprint"],
        "measuringMetasystemSha": identity["measuringMetasystemSha"],
    }


def scorecards(record: dict[str, Any], side: str) -> tuple[str, list[dict[str, Any]], list[str]]:
    cohort_id = record["cohortId"]
    directory_sha, directory = scorecard_directory(cohort_id, side)
    count = record["repetitionCount"]
    expected = {f"{index}.json" for index in range(1, count + 1)}
    actual = {path.name for path in directory.glob("*.json")}
    if actual != expected:
        raise CompareError(
            f"{side} cohort is incomplete: expected {sorted(expected)}, found {sorted(actual)}"
        )
    schema = read_schema("scorecard.schema.json")
    values: list[dict[str, Any]] = []
    paths: list[str] = []
    for index in range(1, count + 1):
        path = directory / f"{index}.json"
        card = load_object(path, f"{side} scorecard {index}")
        violations = schema_violations(card, schema)
        if violations:
            raise CompareError(
                f"{side} scorecard {index} violates scorecard.schema.json: "
                + "; ".join(violations[:12])
            )
        identity = card["identity"]
        expected_identity = {
            "caseId": record["caseId"],
            "caseVersion": record["caseVersion"],
            "caseTree": record["caseTree"],
            "configId": record["configId"],
            "configVersion": record["configVersion"],
            "configTree": record["configTree"],
            "measuringKitVersion": record["measuringKitVersion"],
            "candidateSha": directory_sha,
            "cohortId": cohort_id,
            "repetitionIndex": index,
            "repetitionCount": count,
        }
        mismatches = [name for name, expected_value in expected_identity.items() if identity.get(name) != expected_value]
        if mismatches:
            raise CompareError(f"{side} scorecard {index} identity mismatch: {', '.join(mismatches)}")
        if card["machineFingerprint"] != record["machineFingerprint"]:
            raise CompareError(f"{side} scorecard {index} machine fingerprint differs from its cohort record")
        if card["runValidity"].get("valid") is not True:
            reasons = card["runValidity"].get("reasons", [])
            raise CompareError(f"{side} scorecard {index} runValidity is invalid: {canonical(reasons)}")
        values.append(card)
        paths.append(str(path.relative_to(TOP)))
    first_tuple = comparability_tuple(values[0])
    for index, card in enumerate(values[1:], 2):
        if comparability_tuple(card) != first_tuple:
            raise CompareError(f"{side} cohort comparability tuple changes at repetition {index}")
    if record.get("candidateSha") is not None and record["candidateSha"] != directory_sha:
        raise CompareError(f"{side} cohort candidateSha differs from its scorecard directory")
    return directory_sha, values, paths


def attest(candidate_sha: str) -> dict[str, Any]:
    value = load_object(
        RESULTS / "attestations" / f"{candidate_sha}.json",
        "candidate attestation",
    )
    if value.get("schemaVersion") != 1:
        raise CompareError("candidate attestation schemaVersion is invalid")
    if value.get("candidateSha") != candidate_sha:
        raise CompareError("candidate attestation sha does not match the candidate cohort")
    if value.get("conclusion") != "success":
        raise CompareError("candidate attestation is not green")
    if value.get("source") not in {"local", "ci"}:
        raise CompareError("candidate attestation source is invalid")
    if not isinstance(value.get("command"), str) or not value["command"]:
        raise CompareError("candidate attestation command is missing")
    timestamp(value.get("timestamp"), "candidate attestation timestamp")
    machine = value.get("machineFingerprint")
    if not isinstance(machine, dict) or set(machine) != {"os", "cpuModel", "coreCount"}:
        raise CompareError("candidate attestation machine fingerprint is incomplete")
    if not isinstance(machine.get("os"), str) or not machine["os"]:
        raise CompareError("candidate attestation OS is invalid")
    if not isinstance(machine.get("cpuModel"), str) or not machine["cpuModel"]:
        raise CompareError("candidate attestation CPU model is invalid")
    if not isinstance(machine.get("coreCount"), int) or isinstance(machine["coreCount"], bool) or machine["coreCount"] < 1:
        raise CompareError("candidate attestation core count is invalid")
    if value["source"] == "ci" and not isinstance(value.get("ciRecord"), dict):
        raise CompareError("CI attestation does not embed its prefetched record")
    return value


def proposal(record: dict[str, Any], candidate_sha: str) -> tuple[dict[str, Any], str]:
    proposal_id = record.get("proposalId")
    if not isinstance(proposal_id, str) or ID_RE.fullmatch(proposal_id) is None:
        raise CompareError("candidate cohort does not name a valid proposal")
    path = RESULTS / "proposals" / f"{proposal_id}.json"
    value = load_object(path, "candidate proposal")
    if value.get("id", proposal_id) != proposal_id:
        raise CompareError("candidate proposal id does not match its filename")
    if not isinstance(value.get("targetMetric"), str) or not value["targetMetric"]:
        raise CompareError("candidate proposal targetMetric is missing")
    if value.get("direction") not in {"min", "max"}:
        raise CompareError("candidate proposal direction is invalid")
    if value.get("schemaVersion", 1) == 2:
        pairs = value.get("benchmarks")
        if not isinstance(pairs, list) or not pairs or not all(
            isinstance(item, dict) and all(isinstance(item.get(k), str) and item[k] for k in ("case", "caseVersion", "config", "configVersion")) for item in pairs
        ):
            raise CompareError("candidate proposal benchmarks are invalid: each names case, caseVersion, config, configVersion")
    else:
        specs = value.get("specs")
        if not isinstance(specs, list) or not specs or not all(isinstance(item, str) for item in specs):
            raise CompareError("candidate proposal specs are invalid")
        table = aliases()
        unknown = [item for item in specs if item not in table]
        if unknown:
            raise CompareError(f"candidate proposal (schema 1) names spec ids with no alias: {', '.join(unknown)}")
        value = dict(value)
        value["benchmarks"] = [
            {"case": table[item]["case"], "caseVersion": table[item]["caseVersion"], "config": table[item]["config"], "configVersion": table[item]["configVersion"], "legacyId": item}
            for item in specs
        ]
    for name in ("candidateBranch", "author"):
        if not isinstance(value.get(name), str) or not value[name]:
            raise CompareError(f"candidate proposal {name} is missing")
    proposal_date = value.get("date")
    try:
        if not isinstance(proposal_date, str):
            raise ValueError("missing")
        dt.date.fromisoformat(proposal_date)
    except ValueError as error:
        raise CompareError("candidate proposal date is invalid") from error
    relative = str(path.relative_to(TOP))
    dirty = git("diff", "--quiet", "HEAD", "--", relative, check=False)
    if dirty.returncode != 0:
        raise CompareError("candidate proposal has uncommitted changes")
    commit = git("log", "-1", "--format=%H", "HEAD", "--", relative).stdout.strip()
    if SHA_RE.fullmatch(commit) is None:
        raise CompareError("candidate proposal has no commit")
    ancestry = git("merge-base", "--is-ancestor", commit, candidate_sha, check=False)
    if ancestry.returncode != 0:
        raise CompareError("candidate proposal commit is not an ancestor of candidate sha")
    committed_at = timestamp(
        git("show", "-s", "--format=%cI", commit).stdout.strip(),
        "candidate proposal commit time",
    )
    if committed_at >= timestamp(record["createdAt"], "candidate cohort createdAt"):
        raise CompareError("candidate proposal commit does not predate the cohort record")
    return value, commit


def pinned_documents(record: dict[str, Any]) -> tuple[dict[str, Any], dict[str, Any], bool]:
    """The case.json and configuration a cohort ran, read from the git objects
    its record pins. Without pins (a legacy record) the alias's current
    versions are read and the caller is told so."""
    if record.get("caseTree") and record.get("configTree"):
        case_blob = git("cat-file", "-p", f"{record['caseTree']}:case.json", check=False)
        config_blob = git("cat-file", "-p", record["configTree"], check=False)
        if case_blob.returncode != 0 or config_blob.returncode != 0:
            raise CompareError(
                "the cohort's pinned case or configuration object is unreachable in this clone; "
                f"fetch its measuring commit ({record.get('measuringMetasystemSha')}) and retry"
            )
        try:
            return json.loads(case_blob.stdout), json.loads(config_blob.stdout), True
        except json.JSONDecodeError as error:
            raise CompareError(f"pinned case or configuration document is not valid JSON: {error}") from error
    case_path = KIT / "cases" / str(record["caseId"]) / str(record["caseVersion"]) / "case.json"
    config_path = KIT / "configurations" / str(record["configId"]) / f"{record['configVersion']}.json"
    return load_object(case_path, "case document"), load_object(config_path, "configuration document"), False


def metric_map(card: dict[str, Any]) -> dict[str, dict[str, Any]]:
    result: dict[str, dict[str, Any]] = {}
    for metric in card["productMetrics"]:
        key = f"product.{metric['rawName']}"
        result[key] = {
            "metric": key,
            "name": metric["rawName"],
            "scorecardBlock": "productMetrics",
            "measurement": "value",
            "measurementClass": metric["measurementClass"],
            "sourceOwner": metric["sourceOwner"],
            "value": metric["value"],
            "direction": metric["direction"],
            "floor": metric["floor"],
            "ceiling": metric["ceiling"],
            "noiseFloor": metric["noiseFloor"],
        }
    ignored = {
        "name",
        "measurementClass",
        "sourceOwner",
        "direction",
        "floor",
        "ceiling",
        "noiseFloor",
    }
    for metric in card["mechanicalBehaviorMetrics"]:
        if metric.get("measurementClass") != "constraint":
            continue
        measurements = [name for name, value in metric.items() if name not in ignored and (number(value) or value is None)]
        for measurement in measurements:
            key = f"mechanical.{metric['name']}.{measurement}"
            result[key] = {
                "metric": key,
                "name": metric["name"],
                "scorecardBlock": "mechanicalBehaviorMetrics",
                "measurement": measurement,
                "measurementClass": "constraint",
                "sourceOwner": metric["sourceOwner"],
                "value": metric[measurement],
                "direction": metric.get("direction"),
                "floor": metric.get("floor"),
                "ceiling": metric.get("ceiling"),
                "noiseFloor": metric.get("noiseFloor"),
            }
    for cost in card["costPerProvider"]:
        prefix = f"cost.{cost['provider']}.{cost['role']}"
        measurements: list[tuple[str, Any]] = []
        for item in cost["amounts"]:
            if isinstance(item, dict):
                measurements.append((f"amount.{item.get('currency')}", item.get("amount")))
        for name, value in cost["tokens"].items():
            measurements.append((f"tokens.{name}", value))
        for item in cost["providerUnits"]:
            if isinstance(item, dict):
                measurements.append((f"providerUnit.{item.get('unit')}", item.get("value")))
        for measurement, value in measurements:
            key = f"{prefix}.{measurement}"
            result[key] = {
                "metric": key,
                "name": f"{cost['provider']}:{cost['role']}",
                "scorecardBlock": "costPerProvider",
                "measurement": measurement,
                "measurementClass": "constraint",
                "sourceOwner": cost["sourceOwner"],
                "value": value,
                "direction": cost["direction"],
                "floor": None,
                "ceiling": cost["ceiling"],
                "noiseFloor": cost["noiseFloor"],
            }
    return result


def compare_metrics(
    baseline: list[dict[str, Any]],
    candidate: list[dict[str, Any]],
    eligible: bool,
) -> list[dict[str, Any]]:
    baseline_maps = [metric_map(card) for card in baseline]
    candidate_maps = [metric_map(card) for card in candidate]
    keys = sorted(set().union(*(set(value) for value in baseline_maps + candidate_maps)))
    output: list[dict[str, Any]] = []
    metadata_fields = (
        "name",
        "scorecardBlock",
        "measurement",
        "measurementClass",
        "sourceOwner",
        "direction",
        "floor",
        "ceiling",
        "noiseFloor",
    )
    for key in keys:
        entries = [mapping.get(key) for mapping in baseline_maps + candidate_maps]
        present = [entry for entry in entries if entry is not None]
        reference = present[0]
        for entry in present[1:]:
            if any(canonical(entry.get(field)) != canonical(reference.get(field)) for field in metadata_fields):
                raise CompareError(f"metric metadata changes across cohorts: {key}")
        baseline_values = [mapping.get(key, {}).get("value") for mapping in baseline_maps]
        candidate_values = [mapping.get(key, {}).get("value") for mapping in candidate_maps]
        values_complete = all(number(value) for value in baseline_values + candidate_values)
        baseline_median = median(baseline_values) if values_complete else None
        candidate_median = median(candidate_values) if values_complete else None
        delta = subtract(candidate_median, baseline_median) if values_complete else None
        verdict = "no-verdict"
        reasons: list[str] = []
        absolute_passed: bool | None = None
        direction = reference.get("direction")
        floor = reference.get("floor")
        ceiling = reference.get("ceiling")
        noise = reference.get("noiseFloor")
        if not eligible:
            reasons.append("the benchmark is comparison-ineligible")
        if not values_complete:
            reasons.append("one or more repetitions have no scalar measurement")
        if direction not in {"min", "max"}:
            reasons.append("direction is missing")
        relevant_bound = floor if direction == "max" else ceiling if direction == "min" else None
        if not number(relevant_bound):
            reasons.append("absolute floor or ceiling is missing")
        if not number(noise) or noise < 0:
            reasons.append("noise floor is missing or invalid")
        comparison_ready = (
            eligible
            and values_complete
            and direction in {"min", "max"}
            and number(relevant_bound)
            and number(noise)
            and noise >= 0
        )
        if values_complete:
            absolute_passed = all(
                (not number(floor) or value >= floor) and (not number(ceiling) or value <= ceiling)
                for value in candidate_values
            )
            if eligible and number(relevant_bound) and absolute_passed is False:
                verdict = "regressed"
                reasons.append("a candidate repetition breaches an absolute bound")
        if not reasons and values_complete:
            if direction == "max" and delta > noise:
                verdict = "improved"
                reasons.append("candidate median improved beyond the noise floor")
            elif direction == "max" and delta < -noise:
                verdict = "regressed"
                reasons.append("candidate median regressed beyond the noise floor")
            elif direction == "min" and delta < -noise:
                verdict = "improved"
                reasons.append("candidate median improved beyond the noise floor")
            elif direction == "min" and delta > noise:
                verdict = "regressed"
                reasons.append("candidate median regressed beyond the noise floor")
            else:
                reasons.append("median delta does not exceed the noise floor")
        output.append({
            "metric": key,
            "name": reference["name"],
            "scorecardBlock": reference["scorecardBlock"],
            "measurement": reference["measurement"],
            "measurementClass": reference["measurementClass"],
            "sourceOwner": reference["sourceOwner"],
            "direction": direction,
            "floor": floor,
            "ceiling": ceiling,
            "noiseFloor": noise,
            "baselineValues": baseline_values,
            "candidateValues": candidate_values,
            "baselineMedian": baseline_median,
            "candidateMedian": candidate_median,
            "delta": delta,
            "candidateAbsoluteBoundsPassed": absolute_passed,
            "comparisonReady": comparison_ready,
            "verdict": verdict,
            "reasons": reasons,
        })
    return output


def configurations_report(cohort_ids: list[str]) -> int:
    """Design §3, second axis: the same case version under different
    configurations, side by side, as a REPORT — it never emits a verdict.
    Every cohort must name the same caseId@caseVersion (and caseTree when
    pinned), kit version, machine fingerprint and measuring sha; the
    configurations may (and should) differ."""
    rows = []
    for cohort_id in cohort_ids:
        record = cohort_record(cohort_id, cohort_id)
        _, cards, _ = scorecards(record, cohort_id)
        rows.append((record, cards))
    keys = ("caseId", "caseVersion", "caseTree")
    first = rows[0][0]
    for record, _ in rows[1:]:
        for key in keys:
            if record.get(key) != first.get(key):
                raise CompareError(f"cohorts do not share {key}: {first.get(key)} vs {record.get(key)} — a configuration report holds the case constant")
        for key in ("measuringKitVersion", "measuringMetasystemSha"):
            if record.get(key) != first.get(key):
                raise CompareError(f"cohorts do not share {key}; the report holds it constant")
        if record.get("machineFingerprint") != first.get("machineFingerprint"):
            raise CompareError("cohorts do not share a machine fingerprint; the report holds it constant")
    metric_names: list[str] = []
    for _, cards in rows:
        for metric in cards[0]["productMetrics"]:
            if metric["name"] not in metric_names:
                metric_names.append(metric["name"])
    lines = [f"# Configuration report: {first['caseId']}@{first['caseVersion']}", "",
             f"Kit {first['measuringKitVersion']}, measuring sha {first['measuringMetasystemSha']}. No verdict: configurations are the independent variable.", "",
             "| Metric | " + " | ".join(f"{r['configId']}@{r['configVersion']} ({r['cohortId']})" for r, _ in rows) + " |",
             "| --- | " + " | ".join("---:" for _ in rows) + " |"]
    for name in metric_names:
        cells = []
        for _, cards in rows:
            values = [metric_map(card).get(name, {}).get("value") for card in cards]
            cells.append(str(median(values)) if all(number(v) for v in values) else "n/a")
        lines.append(f"| {name} | " + " | ".join(cells) + " |")
    print("\n".join(lines))
    return 0


def markdown(document: dict[str, Any]) -> str:
    lines = [
        f"# Benchmark comparison: {document['baselineCohortId']} vs {document['candidateCohortId']}",
        "",
        f"**Verdict:** {document['verdict']}",
        "",
        f"**Comparison eligible:** {'yes' if document['comparisonEligible'] else 'no'}",
        "",
    ]
    if document["eligibilityReasons"]:
        lines.extend(["## Eligibility", ""] + [f"- {reason}" for reason in document["eligibilityReasons"]] + [""])
    lines.extend([
        "## Constraint metrics",
        "",
        "| Metric | Baseline median | Candidate median | Delta | Direction | Bounds | Noise | Verdict |",
        "| --- | ---: | ---: | ---: | --- | --- | ---: | --- |",
    ])
    for metric in document["metrics"]:
        bounds = f"floor={metric['floor']}, ceiling={metric['ceiling']}"
        lines.append(
            f"| {metric['metric']} | {metric['baselineMedian']} | {metric['candidateMedian']} | "
            f"{metric['delta']} | {metric['direction']} | {bounds} | {metric['noiseFloor']} | {metric['verdict']} |"
        )
    lines.extend([
        "",
        "Judged scores were not used. Their absence is not an error.",
        "",
    ])
    return "\n".join(lines)


def main() -> int:
    args = arguments()
    if args.configurations:
        try:
            return configurations_report(args.cohort_ids)
        except (CompareError, OSError, ValueError) as error:
            print(f"compare error: {error}", file=sys.stderr)
            return 1
    try:
        baseline_record = cohort_record(args.baseline_cohort_id, "baseline")
        candidate_record = cohort_record(args.candidate_cohort_id, "candidate")
        if baseline_record.get("proposalId") is not None:
            raise CompareError("baseline cohort must not name a proposal")
        baseline_sha, baseline_cards, baseline_paths = scorecards(baseline_record, "baseline")
        candidate_sha, candidate_cards, candidate_paths = scorecards(candidate_record, "candidate")
        baseline_tuple = comparability_tuple(baseline_cards[0])
        candidate_tuple = comparability_tuple(candidate_cards[0])
        if baseline_tuple != candidate_tuple:
            differing = sorted(name for name in baseline_tuple if baseline_tuple[name] != candidate_tuple[name])
            raise CompareError("cohort comparability tuple mismatch: " + ", ".join(differing))
        candidate_attestation = attest(candidate_sha)
        proposal_value, proposal_commit = proposal(candidate_record, candidate_sha)
        compared = {k: baseline_record[k] for k in ("caseId", "caseVersion", "configId", "configVersion")}
        named = any(
            (item["case"], item["caseVersion"], item["config"], item["configVersion"]) == (compared["caseId"], compared["caseVersion"], compared["configId"], compared["configVersion"])
            for item in proposal_value["benchmarks"]
        )
        if not named:
            raise CompareError("candidate proposal does not name the compared benchmark (case version under configuration version)")

        # Eligibility is judged from the RUN's pinned objects, never from
        # current paths (design §7, r6 J-2): the case document from the
        # pinned tree, the configuration from the pinned blob. A legacy
        # (schema-1) cohort has no pins and is read through the alias's
        # current objects, reported as such.
        case_doc, config_doc, pinned = pinned_documents(baseline_record)
        major = str(case_doc.get("version", "")).split(".", 1)[0]
        eligible = case_doc.get("comparisonEligible") is True and major != "0" and config_doc.get("purpose") == "capability"
        eligibility_reasons: list[str] = []
        if not eligible:
            if config_doc.get("purpose") != "capability":
                eligibility_reasons.append(f"configuration {compared['configId']}@{compared['configVersion']} is an orchestration-health probe; its runs are never verdict-eligible")
            else:
                eligibility_reasons.append(str(case_doc.get("comparisonEligibleNote") or "the case is comparison-ineligible"))
        if not pinned:
            eligibility_reasons.append("legacy cohort without pinned object ids: eligibility read from the alias's current objects")
        metrics = compare_metrics(baseline_cards, candidate_cards, eligible)
        target_name = proposal_value["targetMetric"]
        target_matches = [
            metric for metric in metrics
            if target_name in {metric["metric"], metric["name"]}
        ]
        if len(target_matches) != 1:
            overall = "no-verdict"
            target_metric = None
            eligibility_reasons.append("the proposal target does not resolve to exactly one scalar constraint metric")
        else:
            target_metric = target_matches[0]["metric"]
            if target_matches[0]["direction"] != proposal_value["direction"]:
                raise CompareError("candidate proposal direction differs from the target metric")
            if any(metric["verdict"] == "regressed" for metric in metrics):
                overall = "regressed"
            elif any(metric["comparisonReady"] is not True for metric in metrics):
                overall = "no-verdict"
                if eligible:
                    eligibility_reasons.append("one or more constraint metrics lacks comparison inputs")
            elif eligible and target_matches[0]["verdict"] == "improved":
                overall = "improved"
            else:
                overall = "no-verdict"
        document = {
            "schemaVersion": 2,
            "createdAt": dt.datetime.now(dt.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
            "baselineCohortId": args.baseline_cohort_id,
            "candidateCohortId": args.candidate_cohort_id,
            "baselineCandidateSha": baseline_sha,
            "candidateSha": candidate_sha,
            "proposalId": candidate_record["proposalId"],
            "proposalCommit": proposal_commit,
            "targetMetric": target_metric,
            "benchmark": compared,
            "comparisonEligible": eligible,
            "eligibilityReasons": eligibility_reasons,
            "comparabilityTuple": baseline_tuple,
            "cohorts": {
                "baseline": {"record": baseline_record, "scorecards": baseline_paths},
                "candidate": {"record": candidate_record, "scorecards": candidate_paths},
            },
            "attestation": candidate_attestation,
            "metrics": metrics,
            "judgedScoresUsed": False,
            "verdict": overall,
        }
        output = RESULTS / "compares" / f"{args.baseline_cohort_id}-vs-{args.candidate_cohort_id}.json"
        projection = output.with_suffix(".md")
        atomic_text(output, json.dumps(document, indent=2, sort_keys=True) + "\n")
        atomic_text(projection, markdown(document))
    except (CompareError, OSError, ValueError) as error:
        print(f"compare refused: {error}", file=sys.stderr)
        return 1
    print(output)
    print(projection)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
