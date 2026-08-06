#!/usr/bin/env python3
"""Mission contract parser, sealer, and preflight library."""

from __future__ import annotations

import argparse
import fnmatch
import hashlib
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import time
from dataclasses import dataclass
from datetime import datetime, timezone
from decimal import Decimal
from pathlib import Path
from typing import Iterable


METASYSTEM_ROOT = Path(__file__).resolve().parents[2]
ID = r"[a-z0-9][a-z0-9-]*"
DECIMAL = r"-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?"
NONNEGATIVE_DECIMAL = r"(?:0|[1-9][0-9]*)(?:\.[0-9]+)?"
POSITIVE_DECIMAL = r"(?:0\.[0-9]*[1-9][0-9]*|[1-9][0-9]*(?:\.[0-9]+)?)"
APPROVAL_RE = re.compile(
    r"^Approval: name=([^;\n]+); date=(\d{4}-\d{2}-\d{2}); contract-sha256=([0-9a-f]{64})$"
)
METRIC_RE = re.compile(rf"^metric=({ID})=({DECIMAL})$")

SCALARS = {
    "gate.command",
    "gate.ref",
    "gate.paths",
    "truth.paths",
    "truth.certification",
    "gate.direction",
    "guard.cadence",
    "ledger.cycle-budget",
    "ledger.no-gain-budget",
    "fence.wall-clock-hours",
    "fence.cycles",
    "fence.jobs",
    "fence.concurrency",
    "fence.job-cap-min",
    "host.runtime",
    "host.model",
    "host.turn-cap-min",
    "exposure",
}
INTEGER_KEYS = {
    "guard.cadence",
    "ledger.cycle-budget",
    "ledger.no-gain-budget",
    "fence.cycles",
    "fence.jobs",
    "fence.concurrency",
    "fence.job-cap-min",
    "host.turn-cap-min",
}
FENCE_KEYS = (
    "fence.wall-clock-hours",
    "fence.cycles",
    "fence.jobs",
    "fence.concurrency",
    "fence.job-cap-min",
)


class ContractError(RuntimeError):
    pass


@dataclass(frozen=True)
class Contract:
    path: Path
    text: str
    values: dict[str, str]
    sealed: dict[str, str]
    approval: re.Match[str] | None


def fail(message: str) -> None:
    raise ContractError(message)


def git(repo: Path, *args: str, check: bool = True, text: bool = True) -> str | bytes:
    result = subprocess.run(
        ["git", "-C", str(repo), *args],
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=text,
    )
    if check and result.returncode != 0:
        detail = result.stderr.strip() if text else result.stderr.decode(errors="replace").strip()
        fail(f"git {' '.join(args)} failed: {detail}")
    return result.stdout


def repository_for(path: Path) -> Path:
    result = subprocess.run(
        ["git", "-C", str(path.parent), "rev-parse", "--show-toplevel"],
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    if result.returncode != 0:
        fail("mission contract is not inside a git repository")
    return Path(result.stdout.strip()).resolve()


def fenced_blocks(text: str, language: str) -> list[str]:
    pattern = re.compile(rf"^```{re.escape(language)}[ \t]*\n(.*?)^```[ \t]*$", re.MULTILINE | re.DOTALL)
    return pattern.findall(text)


def parse_key_values(block: str, label: str) -> dict[str, str]:
    values: dict[str, str] = {}
    for number, raw in enumerate(block.splitlines(), 1):
        if not raw.strip():
            continue
        if "=" not in raw:
            fail(f"{label} line {number} is not key=value")
        key, value = raw.split("=", 1)
        if key != key.strip() or not key or value != value.strip() or not value:
            fail(f"{label} line {number} has an empty or whitespace-padded key/value")
        if key in values:
            fail(f"{label} repeats key: {key}")
        values[key] = value
    return values


def section_body(text: str, heading: str) -> str:
    matches = list(re.finditer(rf"^# {re.escape(heading)}[ \t]*$", text, re.MULTILINE))
    if len(matches) != 1:
        fail(f"contract must contain exactly one '# {heading}' section")
    start = matches[0].end()
    next_heading = re.search(r"^# .+$", text[start:], re.MULTILINE)
    end = start + next_heading.start() if next_heading else len(text)
    body = text[start:end]
    body = re.sub(r"```(?:mission|mission-seal)[ \t]*\n.*?^```[ \t]*$", "", body, flags=re.MULTILINE | re.DOTALL)
    if not body.strip():
        fail(f"'# {heading}' must contain prose")
    return body


def preauthorizable_categories(project_root: Path) -> set[str]:
    policy = project_root / "docs" / "project-rules.md"
    try:
        text = policy.read_text(encoding="utf-8")
    except OSError as error:
        fail(f"cannot read envelope policy: {error}")
    categories: set[str] = set()
    for raw in text.splitlines():
        match = re.match(rf"^\| `({ID})` \|.*\| (yes|no) \|", raw)
        if match and match.group(2) == "yes":
            categories.add(match.group(1))
    if not categories:
        fail("docs/project-rules.md marks no mission envelope category pre-authorizable")
    return categories


def validate_globs(value: str, key: str) -> list[str]:
    result = []
    for item in value.split(","):
        item = item.strip()
        if not item or item.startswith("/") or "\\" in item:
            fail(f"{key} must be comma-separated repository-relative globs")
        parts = Path(item).parts
        if any(part in {"", ".", ".."} for part in parts) or any(ord(char) < 32 for char in item):
            fail(f"{key} contains an unsafe repository-relative glob: {item}")
        result.append(item)
    return result


def validate_literal_tokens(value: str, key: str) -> list[str]:
    tokens = [item.strip() for item in value.split(",")]
    if not tokens or any(not item for item in tokens):
        fail(f"{key} must have a bounded comma-separated literal-token list")
    unbounded = {"all", "any", "everything", "unbounded", "unlimited"}
    for token in tokens:
        if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._/@:+<>=-]*", token):
            fail(f"{key} has an unbounded or non-literal token: {token}")
        if token.casefold() in unbounded:
            fail(f"{key} has an unbounded or non-literal token: {token}")
    return tokens


def validate_dispatch_allow(value: str) -> list[str]:
    pairs = validate_literal_tokens(value, "envelope.dispatch-allow")
    for pair in pairs:
        runtime, separator, model = pair.partition(":")
        if (
            separator != ":"
            or not re.fullmatch(ID, runtime)
            or not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._/-]*", model)
        ):
            fail(
                "envelope.dispatch-allow must be a comma-separated list of exact runtime:model pairs"
            )
    return pairs


def validate_contract(contract: Contract, project_root: Path) -> None:
    for heading in ("Intent", "Non-goals", "Initial streams"):
        section_body(contract.text, heading)

    values = contract.values
    missing = sorted(SCALARS - set(values))
    if missing:
        fail("mission contract is missing required key(s): " + ", ".join(missing))

    thresholds: dict[str, str] = {}
    noises: dict[str, str] = {}
    guards: dict[str, dict[str, str]] = {}
    streams: dict[str, str] = {}
    envelopes: dict[str, str] = {}
    allowed_patterns = (
        re.compile(rf"^gate\.threshold\.({ID})$"),
        re.compile(rf"^gate\.noise-floor\.({ID})$"),
        re.compile(rf"^guard\.({ID})\.(command|floor|noise)$"),
        re.compile(rf"^stream\.({ID})$"),
        re.compile(rf"^envelope\.({ID})$"),
    )
    for key, value in values.items():
        if key in SCALARS:
            continue
        match = allowed_patterns[0].fullmatch(key)
        if match:
            thresholds[match.group(1)] = value
            continue
        match = allowed_patterns[1].fullmatch(key)
        if match:
            noises[match.group(1)] = value
            continue
        match = allowed_patterns[2].fullmatch(key)
        if match:
            guards.setdefault(match.group(1), {})[match.group(2)] = value
            continue
        match = allowed_patterns[3].fullmatch(key)
        if match:
            streams[match.group(1)] = value
            continue
        match = allowed_patterns[4].fullmatch(key)
        if match:
            envelopes[match.group(1)] = value
            continue
        fail(f"mission contract has an unknown key: {key}")

    if not thresholds:
        fail("mission contract must declare at least one gate.threshold.<metric>")
    if set(thresholds) != set(noises):
        fail("every gate threshold must have exactly one matching noise floor")
    for metric, threshold in thresholds.items():
        if not re.fullmatch(rf"(?:>=|<=|>|<){DECIMAL}", threshold):
            fail(f"gate.threshold.{metric} must be >=, <=, >, or < followed by a decimal")
        if not re.fullmatch(NONNEGATIVE_DECIMAL, noises[metric]):
            fail(f"gate.noise-floor.{metric} must be a non-negative decimal")

    if not guards:
        fail("mission contract must declare at least one guard")
    for name, fields in guards.items():
        if set(fields) != {"command", "floor", "noise"}:
            fail(f"guard.{name} must declare command, floor, and noise")
        if not fields["command"].strip():
            fail(f"guard.{name}.command must not be empty")
        if not re.fullmatch(DECIMAL, fields["floor"]):
            fail(f"guard.{name}.floor must be a decimal")
        if not re.fullmatch(NONNEGATIVE_DECIMAL, fields["noise"]):
            fail(f"guard.{name}.noise must be a non-negative decimal")

    if not streams or any(not goal.strip() for goal in streams.values()):
        fail("mission contract must declare at least one non-empty stream.<id>")
    if not envelopes:
        fail("mission contract must declare at least one bounded envelope.<category>")
    if "tier-move" in envelopes:
        fail(
            "envelope.tier-move is retired; use envelope.dispatch-allow with exact runtime:model pairs"
        )
    permitted = preauthorizable_categories(project_root)
    for category, bound in envelopes.items():
        if category not in permitted:
            fail(f"envelope category is not marked pre-authorizable: {category}")
        if category == "dispatch-allow":
            validate_dispatch_allow(bound)
        else:
            validate_literal_tokens(bound, f"envelope.{category}")

    if values["truth.certification"] not in {"candidate", "certified"}:
        fail("truth.certification must be candidate or certified")
    if values["gate.direction"] not in {"max", "min"}:
        fail("gate.direction must be max or min")
    if not values["gate.command"].strip() or "\x00" in values["gate.command"]:
        fail("gate.command must be one non-empty command")
    if values["gate.ref"].startswith("-") or not re.fullmatch(r"[^\s\x00]+", values["gate.ref"]):
        fail("gate.ref must be one non-empty git commit-ish")
    validate_globs(values["gate.paths"], "gate.paths")
    validate_globs(values["truth.paths"], "truth.paths")
    for key in INTEGER_KEYS:
        if not re.fullmatch(r"[1-9][0-9]*", values[key]):
            fail(f"{key} must be a positive integer")
    if not re.fullmatch(POSITIVE_DECIMAL, values["fence.wall-clock-hours"]):
        fail("fence.wall-clock-hours must be a positive decimal")
    for key in ("host.runtime", "host.model"):
        if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._:/-]*", values[key]):
            fail(f"{key} must be one literal id")
    if not re.fullmatch(r"[A-Z]{3}:(?:0|[1-9][0-9]*)(?:\.[0-9]+)?", values["exposure"]):
        fail("exposure must be a human-priced amount in CURRENCY:amount form")

    if contract.approval:
        name = contract.approval.group(1)
        if name != name.strip() or any(ord(character) < 32 for character in name):
            fail("approval name has leading/trailing whitespace or control characters")
        try:
            datetime.strptime(contract.approval.group(2), "%Y-%m-%d")
        except ValueError:
            fail("approval date is not a real YYYY-MM-DD date")


def read_contract(path: Path) -> Contract:
    try:
        text = path.read_text(encoding="utf-8")
    except OSError as error:
        fail(f"cannot read contract: {error}")
    authored = fenced_blocks(text, "mission")
    seals = fenced_blocks(text, "mission-seal")
    if len(authored) != 1:
        fail("contract must contain exactly one fenced mission key=value block")
    if len(seals) > 1:
        fail("contract contains more than one generated mission-seal block")
    approval_lines = [line for line in text.splitlines() if line.startswith("Approval:")]
    if len(approval_lines) > 1:
        fail("contract contains more than one approval line")
    approval = APPROVAL_RE.fullmatch(approval_lines[0]) if approval_lines else None
    if approval_lines and approval is None:
        fail("approval line has invalid grammar")
    return Contract(
        path=path,
        text=text,
        values=parse_key_values(authored[0], "mission block"),
        sealed=parse_key_values(seals[0], "mission-seal block") if seals else {},
        approval=approval,
    )


def canonical_signed_bytes(text: str) -> bytes:
    lines = [line.rstrip(" \t") for line in text.splitlines() if not line.startswith("Approval:")]
    canonical = "\n".join(lines).rstrip()
    return canonical.encode("utf-8")


def contract_hash(text: str) -> str:
    return hashlib.sha256(canonical_signed_bytes(text)).hexdigest()


def tree_paths(repo: Path, ref: str) -> list[str]:
    output = git(repo, "ls-tree", "-r", "--name-only", "-z", ref, text=False)
    assert isinstance(output, bytes)
    return [item.decode("utf-8", errors="strict") for item in output.split(b"\0") if item]


def expand_paths(repo: Path, project_root: Path, ref: str, globs: Iterable[str], label: str) -> list[str]:
    candidates = tree_paths(repo, ref)
    try:
        prefix = project_root.resolve().relative_to(repo.resolve()).as_posix()
    except ValueError:
        fail("metasystem project root is outside its git repository")
    prefix = "" if prefix == "." else prefix.rstrip("/") + "/"
    project_candidates = {
        path[len(prefix) :]: path for path in candidates if not prefix or path.startswith(prefix)
    }
    selected: set[str] = set()
    for pattern in globs:
        matches = [full for relative, full in project_candidates.items() if fnmatch.fnmatchcase(relative, pattern)]
        if not matches:
            fail(f"{label} glob matches no path at gate.ref: {pattern}")
        selected.update(matches)
    return sorted(selected)


def manifest_hash(repo: Path, ref: str, paths: Iterable[str]) -> str:
    digest = hashlib.sha256()
    for path in paths:
        content = git(repo, "show", f"{ref}:{path}", text=False)
        assert isinstance(content, bytes)
        digest.update(path.encode("utf-8"))
        digest.update(b"\0")
        digest.update(hashlib.sha256(content).digest())
        digest.update(b"\0")
    return digest.hexdigest()


def threshold_passes(expression: str, value: Decimal) -> bool:
    match = re.fullmatch(rf"(>=|<=|>|<)({DECIMAL})", expression)
    assert match
    target = Decimal(match.group(2))
    return {
        ">=": value >= target,
        "<=": value <= target,
        ">": value > target,
        "<": value < target,
    }[match.group(1)]


def run_gate(contract: Contract, repo: Path, project_root: Path) -> tuple[str, dict[str, str], int]:
    values = contract.values
    gate_ref = str(git(repo, "rev-parse", f"{values['gate.ref']}^{{commit}}")).strip()
    branch = contract.sealed.get("candidate.branch") or str(git(repo, "branch", "--show-current")).strip()
    if not branch:
        fail("candidate must be a named branch")
    candidate_sha = str(git(repo, "rev-parse", f"{branch}^{{commit}}")).strip()
    gate_paths = expand_paths(repo, project_root, gate_ref, validate_globs(values["gate.paths"], "gate.paths"), "gate.paths")
    truth_paths = expand_paths(repo, project_root, gate_ref, validate_globs(values["truth.paths"], "truth.paths"), "truth.paths")
    restored = sorted(set(gate_paths + truth_paths))
    scratch = Path(tempfile.mkdtemp(prefix="mission-gate."))
    worktree = scratch / "candidate"
    try:
        git(repo, "worktree", "add", "--detach", "--quiet", str(worktree), candidate_sha)
        git(worktree, "checkout", "--quiet", gate_ref, "--", *restored)
        prefix = project_root.resolve().relative_to(repo.resolve())
        command_root = worktree / prefix
        timeout_seconds = int(values["fence.job-cap-min"]) * 60
        try:
            result = subprocess.run(
                ["bash", "-lc", values["gate.command"]],
                cwd=command_root,
                stdout=subprocess.PIPE,
                stderr=subprocess.STDOUT,
                text=True,
                timeout=timeout_seconds,
                check=False,
            )
        except subprocess.TimeoutExpired:
            fail(f"gate measurement exceeded named fence.job-cap-min ceiling ({values['fence.job-cap-min']}m)")
        if result.returncode != 0:
            fail(f"gate measurement failed with exit {result.returncode}")
        metrics: dict[str, str] = {}
        for line in result.stdout.splitlines():
            match = METRIC_RE.fullmatch(line.strip())
            if match:
                metrics[match.group(1)] = match.group(2)
        names = {key.removeprefix("gate.threshold.") for key in values if key.startswith("gate.threshold.")}
        missing = sorted(names - set(metrics))
        if missing:
            fail("gate output omitted declared metric(s): " + ", ".join(missing))
        failures = sum(
            not threshold_passes(values[f"gate.threshold.{name}"], Decimal(metrics[name])) for name in names
        )
        return candidate_sha, {name: metrics[name] for name in sorted(names)}, failures
    finally:
        if worktree.exists():
            git(repo, "worktree", "remove", "--force", str(worktree), check=False)
        shutil.rmtree(scratch, ignore_errors=True)


def expected_seal(contract: Contract, repo: Path, project_root: Path, run_baseline: bool) -> dict[str, str]:
    values = contract.values
    gate_ref = str(git(repo, "rev-parse", f"{values['gate.ref']}^{{commit}}")).strip()
    gate_paths = expand_paths(repo, project_root, gate_ref, validate_globs(values["gate.paths"], "gate.paths"), "gate.paths")
    truth_paths = expand_paths(repo, project_root, gate_ref, validate_globs(values["truth.paths"], "truth.paths"), "truth.paths")
    branch = contract.sealed.get("candidate.branch") or str(git(repo, "branch", "--show-current")).strip()
    if not branch:
        fail("candidate must be a named branch")
    seal = {
        "sealed.version": "1",
        "candidate.branch": branch,
        "sealed.gate-ref-sha": gate_ref,
        "sealed.gate-integrity.sha256": manifest_hash(repo, gate_ref, gate_paths),
        "sealed.truth-integrity.sha256": manifest_hash(repo, gate_ref, truth_paths),
        "sealed.baseline.failure-identifiers": "unavailable",
    }
    for key in FENCE_KEYS:
        seal[f"sealed.exposure.{key}"] = values[key]
    echo = ",".join(f"{key}={values[key]}" for key in FENCE_KEYS)
    seal["sealed.exposure.statement"] = f"{values['exposure']}|{echo}"
    if run_baseline:
        candidate_sha, metrics, failures = run_gate(contract, repo, project_root)
        seal["sealed.baseline.candidate-sha"] = candidate_sha
        seal["sealed.baseline.failure-count"] = str(failures)
        for name, value in metrics.items():
            seal[f"sealed.baseline.{name}"] = value
    return seal


def seal_contract(contract: Contract, repo: Path, project_root: Path) -> str:
    if contract.sealed:
        fail("contract is already sealed")
    if contract.approval:
        fail("seal must run before approval is added")
    seal = expected_seal(contract, repo, project_root, run_baseline=True)
    seal["sealed.at"] = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    ordered = [
        "sealed.version",
        "sealed.at",
        "candidate.branch",
        "sealed.gate-ref-sha",
        "sealed.gate-integrity.sha256",
        "sealed.truth-integrity.sha256",
        "sealed.baseline.candidate-sha",
        "sealed.baseline.failure-count",
        "sealed.baseline.failure-identifiers",
    ]
    ordered += sorted(key for key in seal if key.startswith("sealed.baseline.") and key not in ordered)
    ordered += [f"sealed.exposure.{key}" for key in FENCE_KEYS]
    ordered += ["sealed.exposure.statement"]
    block = "\n".join(f"{key}={seal[key]}" for key in ordered)
    updated = contract.text.rstrip() + f"\n\n```mission-seal\n{block}\n```\n"
    temporary = contract.path.with_name(contract.path.name + ".seal.tmp")
    try:
        with temporary.open("w", encoding="utf-8") as handle:
            handle.write(updated)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, contract.path)
        directory = os.open(contract.path.parent, os.O_RDONLY)
        try:
            os.fsync(directory)
        finally:
            os.close(directory)
    finally:
        temporary.unlink(missing_ok=True)
    return contract_hash(updated)


def verify_seal(contract: Contract, repo: Path, project_root: Path) -> None:
    if not contract.sealed:
        fail("preflight refused: contract is unsealed")
    required = expected_seal(contract, repo, project_root, run_baseline=False)
    scalar_required = {
        **required,
        "sealed.at": contract.sealed.get("sealed.at", ""),
        "sealed.baseline.candidate-sha": contract.sealed.get("sealed.baseline.candidate-sha", ""),
        "sealed.baseline.failure-count": contract.sealed.get("sealed.baseline.failure-count", ""),
    }
    metric_names = sorted(key.removeprefix("gate.threshold.") for key in contract.values if key.startswith("gate.threshold."))
    for name in metric_names:
        scalar_required[f"sealed.baseline.{name}"] = contract.sealed.get(f"sealed.baseline.{name}", "")
    if set(contract.sealed) != set(scalar_required):
        fail("preflight refused: generated seal keys are missing or unexpected")
    for key, expected in required.items():
        actual = contract.sealed.get(key)
        if actual != expected:
            if key.startswith("sealed.exposure."):
                fail(f"preflight refused: exposure is stale at {key}")
            fail(f"preflight refused: seal integrity mismatch at {key}")
    try:
        datetime.strptime(contract.sealed["sealed.at"], "%Y-%m-%dT%H:%M:%SZ")
    except (KeyError, ValueError):
        fail("preflight refused: sealed.at is invalid")
    if not re.fullmatch(r"[0-9a-f]{40,64}", contract.sealed["sealed.baseline.candidate-sha"]):
        fail("preflight refused: baseline candidate sha is invalid")
    if not re.fullmatch(r"[0-9]+", contract.sealed["sealed.baseline.failure-count"]):
        fail("preflight refused: baseline failure count is invalid")
    for name in metric_names:
        if not re.fullmatch(DECIMAL, contract.sealed[f"sealed.baseline.{name}"]):
            fail(f"preflight refused: baseline metric is invalid: {name}")


def verify_approval(contract: Contract) -> None:
    if contract.approval is None:
        fail("preflight refused: contract is unsigned")
    actual = contract_hash(contract.text)
    if contract.approval.group(3) != actual:
        fail("preflight refused: approval hash does not match the sealed bytes")


def verify_origin(contract: Contract, repo: Path) -> None:
    fetch = subprocess.run(
        ["git", "-C", str(repo), "fetch", "--quiet", "origin"],
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    if fetch.returncode != 0:
        fail(f"preflight refused: origin fetch failed: {fetch.stderr.strip()}")
    remote_head = str(git(repo, "symbolic-ref", "refs/remotes/origin/HEAD")).strip()
    if not remote_head.startswith("refs/remotes/origin/"):
        fail("preflight refused: origin default branch is not declared")
    relative = contract.path.resolve().relative_to(repo).as_posix()
    result = subprocess.run(
        ["git", "-C", str(repo), "show", f"{remote_head}:{relative}"],
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if result.returncode != 0 or result.stdout != contract.path.read_bytes():
        fail("preflight refused: signed contract bytes are absent from fetched origin default branch")


def process_has_tag(project_root: Path, pid: int, started: int, tag: str) -> bool:
    if pid <= 1 or started < 1 or not tag:
        return False
    try:
        os.kill(pid, 0)
    except OSError:
        return False
    helper = project_root / "scripts" / "agents" / "process-census.py"
    if helper.exists():
        identity = subprocess.run(
            [str(helper), "alive", "--pid", str(pid), "--start-time", str(started)],
            check=False,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        if identity.returncode != 0:
            return False
    try:
        result = subprocess.run(
            ["ps", "-p", str(pid), "-o", "command="],
            check=False,
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
            text=True,
        )
    except OSError:
        fixture = os.environ.get("METASYSTEM_MISSION_PROCESS_IDENTITY_FILE")
        if not fixture:
            return False
        try:
            identities = json.loads(Path(fixture).read_text(encoding="utf-8"))
            return identities[str(pid)]["pidStartedAt"] == started and tag in identities[str(pid)]["command"]
        except (OSError, ValueError, KeyError, TypeError):
            return False
    return result.returncode == 0 and tag in result.stdout


def verify_supervision(project_root: Path) -> None:
    directory = project_root / "artifacts" / "agents" / "supervision"
    try:
        state = json.loads((directory / "state.json").read_text(encoding="utf-8"))
        census = json.loads((directory / "last-census.json").read_text(encoding="utf-8"))
    except (OSError, ValueError) as error:
        fail(f"preflight refused: supervisor set is unarmed: {error}")
    interval = state.get("intervalSec")
    if isinstance(interval, bool) or not isinstance(interval, int) or interval < 1:
        fail("preflight refused: supervisor interval is invalid")
    now = int(time.time())
    for name in ("watcher", "reaper"):
        component = state.get("components", {}).get(name, {})
        pid, started, tag = component.get("pid"), component.get("pidStartedAt"), component.get("instanceTag")
        heartbeat_path = Path(component.get("heartbeat", ""))
        try:
            heartbeat = json.loads(heartbeat_path.read_text(encoding="utf-8"))
        except (OSError, ValueError):
            fail(f"preflight refused: {name} is not armed")
        if not isinstance(pid, int) or not isinstance(started, int) or not process_has_tag(project_root, pid, started, tag):
            fail(f"preflight refused: {name} process identity is not live")
        if heartbeat.get("function") != name or heartbeat.get("pid") != pid or heartbeat.get("pidStartedAt") != started:
            fail(f"preflight refused: {name} heartbeat identity does not match")
        observed = heartbeat.get("observedAtEpoch")
        age = now - observed if isinstance(observed, int) else interval * 3
        if age < -5 or age > interval * 2 + 2:
            fail(f"preflight refused: {name} heartbeat is stale")
    completed = census.get("completedAtEpoch")
    census_age = now - completed if isinstance(completed, int) else interval + 1
    if census.get("verdict") != "SUCCESS" or census_age < -5 or census_age > interval:
        fail("preflight refused: census is absent, failed, or stale")
    fingerprint = subprocess.run(
        [str(project_root / "scripts" / "agents" / "arm-supervision.sh"), "fingerprint", "--repo", str(project_root)],
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    if fingerprint.returncode != 0 or fingerprint.stdout.strip() != state.get("fingerprint"):
        fail("preflight refused: supervisor fingerprint does not match live code")
    if census.get("fingerprint") != state.get("fingerprint"):
        fail("preflight refused: census fingerprint does not match supervisor set")


def mission_id_from_path(path: Path) -> str:
    match = re.fullmatch(rf"mission-({ID})\.contract\.md", path.name)
    if not match:
        fail("mission contract filename must be mission-<mission-id>.contract.md")
    return match.group(1)


def verify_lease_acquirable(contract: Contract, project_root: Path) -> None:
    mission = mission_id_from_path(contract.path)
    directory = project_root / "artifacts" / "agents" / "missions" / mission
    marker = directory / "lease.d"
    directory.mkdir(parents=True, exist_ok=True)
    try:
        marker.mkdir()
    except FileExistsError:
        fail("preflight refused: mission lease is not acquirable")
    else:
        marker.rmdir()


def preflight(contract: Contract, repo: Path, project_root: Path) -> None:
    verify_seal(contract, repo, project_root)
    verify_approval(contract)
    verify_origin(contract, repo)
    run_gate(contract, repo, project_root)
    verify_supervision(project_root)
    verify_lease_acquirable(contract, project_root)


def project_root_for(contract: Path, repo: Path) -> Path:
    try:
        contract.relative_to(METASYSTEM_ROOT)
        METASYSTEM_ROOT.relative_to(repo)
    except ValueError:
        return repo
    return METASYSTEM_ROOT


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("mode", choices=("validate", "seal", "preflight"))
    parser.add_argument("--file", required=True, type=Path)
    args = parser.parse_args()
    path = args.file.resolve()
    try:
        contract = read_contract(path)
        repo = repository_for(path)
        project_root = project_root_for(path, repo)
        validate_contract(contract, project_root)
        if args.mode == "seal":
            print(seal_contract(contract, repo, project_root))
        elif args.mode == "preflight":
            preflight(contract, repo, project_root)
            print(f"mission preflight passed: {mission_id_from_path(path)}")
        else:
            print(f"mission contract valid: {path}")
    except ContractError as error:
        print(str(error), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
