#!/usr/bin/env python3
"""BM-1 v0.1 held-out grader."""

from __future__ import annotations

import hashlib
import json
import os
import re
import shutil
import statistics
import subprocess
import sys
import tempfile
import time
import xml.etree.ElementTree as ET
from dataclasses import dataclass
from pathlib import Path
from typing import Callable


GRADER_DIR = Path(__file__).resolve().parent
SEED_DIR = GRADER_DIR.parent / "seed"
NAME_RE = re.compile(r"[A-Za-z0-9][A-Za-z0-9_.-]*\Z")
TEST_ID_RE = re.compile(r"[A-Za-z_$][\w$]*(?:\.[A-Za-z_$][\w$]*)*#[A-Za-z_$][\w$]*\Z")


@dataclass(frozen=True)
class ProcessResult:
    code: int
    stdout: str
    stderr: str
    seconds: float


@dataclass(frozen=True)
class CheckResult:
    requirement: int
    check_id: str
    passed: bool


def run_process(
    argv: list[str], cwd: Path, timeout: float = 30.0, env: dict[str, str] | None = None
) -> ProcessResult:
    started = time.monotonic()
    try:
        completed = subprocess.run(
            argv,
            cwd=cwd,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            timeout=timeout,
            env=env,
            check=False,
        )
        return ProcessResult(
            completed.returncode,
            completed.stdout,
            completed.stderr,
            time.monotonic() - started,
        )
    except (OSError, subprocess.TimeoutExpired) as exc:
        return ProcessResult(-999, "", str(exc), time.monotonic() - started)


def copy_repository(source: Path, destination: Path) -> None:
    shutil.copytree(
        source,
        destination,
        symlinks=False,
        ignore=shutil.ignore_patterns(".git", "target", ".taskrun-cache", "__pycache__"),
    )


def remove_cache(case_dir: Path) -> None:
    cache = case_dir / ".taskrun-cache"
    if cache.is_dir():
        shutil.rmtree(cache)
    elif cache.exists() or cache.is_symlink():
        cache.unlink()


def directory_snapshot(path: Path) -> dict[str, str]:
    if not path.exists():
        return {}
    result: dict[str, str] = {}
    for item in sorted(path.rglob("*")):
        relative = item.relative_to(path).as_posix()
        if item.is_symlink():
            result[relative] = "link:" + os.readlink(item)
        elif item.is_file():
            result[relative] = hashlib.sha256(item.read_bytes()).hexdigest()
        elif item.is_dir():
            result[relative + "/"] = "directory"
    return result


def write_config(case_dir: Path, tasks: dict[str, dict], name: str = "tasks.json") -> Path:
    path = case_dir / name
    path.write_text(json.dumps({"tasks": tasks}, separators=(",", ":")), encoding="utf-8")
    return path


def parse_text_run(output: str) -> tuple[list[str], dict[str, str], dict[str, int]] | None:
    lines = output.splitlines()
    if not lines:
        return None
    match = re.fullmatch(r"summary ran=(\d+) cached=(\d+) failed=(\d+) blocked=(\d+)", lines[-1])
    if match is None:
        return None
    order: list[str] = []
    states: dict[str, str] = {}
    for line in lines[:-1]:
        task = re.fullmatch(r"(ran|cached|failed|blocked) ([A-Za-z0-9][A-Za-z0-9_.-]*)", line)
        if task is None or task.group(2) in states:
            return None
        states[task.group(2)] = task.group(1)
        order.append(task.group(2))
    counts = dict(zip(("ran", "cached", "failed", "blocked"), map(int, match.groups())))
    if sum(counts.values()) != len(order):
        return None
    if any(list(states.values()).count(state) != count for state, count in counts.items()):
        return None
    return order, states, counts


def parse_text_plan(output: str) -> tuple[list[str], int] | None:
    lines = output.splitlines()
    if not lines:
        return None
    match = re.fullmatch(r"summary planned=(\d+)", lines[-1])
    if match is None:
        return None
    order: list[str] = []
    for line in lines[:-1]:
        task = re.fullmatch(r"plan ([A-Za-z0-9][A-Za-z0-9_.-]*)", line)
        if task is None:
            return None
        order.append(task.group(1))
    return (order, int(match.group(1))) if len(order) == int(match.group(1)) else None


def parse_json_run(output: str) -> tuple[list[str], dict[str, str], dict[str, int]] | None:
    try:
        value = json.loads(output)
    except (json.JSONDecodeError, TypeError):
        return None
    if not isinstance(value, dict) or set(value) != {"order", "tasks", "summary"}:
        return None
    order, tasks, summary = value["order"], value["tasks"], value["summary"]
    if not isinstance(order, list) or not all(isinstance(item, str) for item in order):
        return None
    if not isinstance(tasks, dict) or set(tasks) != set(order):
        return None
    if any(state not in {"ran", "cached", "failed", "blocked"} for state in tasks.values()):
        return None
    if not isinstance(summary, dict) or set(summary) != {"ran", "cached", "failed", "blocked"}:
        return None
    if any(type(value) is not int or value < 0 for value in summary.values()):
        return None
    if any(list(tasks.values()).count(state) != summary[state] for state in summary):
        return None
    return order, tasks, summary


def parse_json_plan(output: str) -> tuple[list[str], int] | None:
    try:
        value = json.loads(output)
    except (json.JSONDecodeError, TypeError):
        return None
    if not isinstance(value, dict) or set(value) != {"order", "summary"}:
        return None
    order, summary = value["order"], value["summary"]
    if not isinstance(order, list) or not all(isinstance(item, str) for item in order):
        return None
    if not isinstance(summary, dict) or set(summary) != {"planned"}:
        return None
    planned = summary["planned"]
    return (order, planned) if type(planned) is int and planned == len(order) else None


def pom_facts(pom: Path) -> tuple[int, int]:
    if not pom.is_file():
        return 0, 0
    try:
        root = ET.parse(pom).getroot()
    except (ET.ParseError, OSError):
        return 0, 0

    def local(tag: str) -> str:
        return tag.rsplit("}", 1)[-1]

    release = 0
    for element in root.iter():
        if local(element.tag) in {"maven.compiler.release", "release"} and element.text:
            text = element.text.strip()
            if text.isdigit():
                release = max(release, int(text))
    dependency_count = 0
    for dependencies in root:
        if local(dependencies.tag) != "dependencies":
            continue
        for dependency in dependencies:
            if local(dependency.tag) != "dependency":
                continue
            scope = "compile"
            for child in dependency:
                if local(child.tag) == "scope" and child.text:
                    scope = child.text.strip()
            if scope in {"compile", "runtime"}:
                dependency_count += 1
    return release, dependency_count


class Battery:
    def __init__(self, repository: Path, jar: Path, root: Path) -> None:
        self.repository = repository
        self.jar = jar
        self.root = root
        self.counter = 0
        self.checks: list[CheckResult] = []

    def new_case(self, label: str) -> Path:
        self.counter += 1
        path = self.root / f"{self.counter:03d}-{label}"
        path.mkdir()
        return path

    def run(self, case_dir: Path, *args: str, timeout: float = 30.0) -> ProcessResult:
        if not self.jar.is_file():
            return ProcessResult(-999, "", "taskrun.jar unavailable", 0.0)
        return run_process(["java", "-jar", str(self.jar), *args], case_dir, timeout)

    def check(self, requirement: int, check_id: str, test: Callable[[], bool]) -> bool:
        try:
            passed = bool(test())
        except Exception:
            passed = False
        self.checks.append(CheckResult(requirement, check_id, passed))
        return passed


def exercise_acceptance(battery: Battery, build_clean: bool, dependency_count: int) -> None:
    def named_selection() -> bool:
        case = battery.new_case("named-selection")
        write_config(case, {
            "dep": {"command": "sh -c 'echo dep > dep.out'", "outputs": ["dep.out"]},
            "selected": {"command": "sh -c 'cat dep.out > selected.out'", "outputs": ["selected.out"], "deps": ["dep"]},
            "outside": {"command": "sh -c 'echo no > outside.out'", "outputs": ["outside.out"]},
        })
        result = battery.run(case, "run", "selected")
        parsed = parse_text_run(result.stdout)
        return result.code == 0 and parsed is not None and set(parsed[0]) == {"dep", "selected"} and not (case / "outside.out").exists()

    def all_selection() -> bool:
        case = battery.new_case("all-selection")
        write_config(case, {"a": {"command": "true"}, "b": {"command": "true"}})
        result = battery.run(case, "run")
        parsed = parse_text_run(result.stdout)
        return result.code == 0 and parsed is not None and set(parsed[0]) == {"a", "b"}

    def option_boundary() -> bool:
        case = battery.new_case("option-boundary")
        write_config(case, {"a": {"command": "true"}})
        return battery.run(case, "--dry-run", "run").code == 2 and battery.run(case, "run", "a", "--force").code == 2

    def absent_task() -> bool:
        case = battery.new_case("absent-task")
        write_config(case, {"a": {"command": "true"}})
        return battery.run(case, "run", "missing").code == 2

    battery.check(1, "r01-named-selection", named_selection)
    battery.check(1, "r01-all-selection", all_selection)
    battery.check(1, "r01-option-boundary", option_boundary)
    battery.check(1, "r01-absent-task", absent_task)

    def default_file() -> bool:
        case = battery.new_case("default-file")
        write_config(case, {"a": {"command": "true"}})
        return battery.run(case, "run").code == 0

    def explicit_file() -> bool:
        case = battery.new_case("explicit-file")
        write_config(case, {"a": {"command": "true"}}, "alternate.json")
        result = battery.run(case, "run", "--file", "alternate.json")
        parsed = parse_text_run(result.stdout)
        return result.code == 0 and parsed is not None and parsed[0] == ["a"]

    battery.check(2, "r02-default-file", default_file)
    battery.check(2, "r02-explicit-file", explicit_file)

    def dry_no_execute() -> bool:
        case = battery.new_case("dry-no-execute")
        write_config(case, {"a": {"command": "sh -c 'echo bad > marker'", "outputs": ["marker"]}})
        result = battery.run(case, "run", "--dry-run")
        return result.code == 0 and parse_text_plan(result.stdout) == (["a"], 1) and not (case / "marker").exists() and not (case / ".taskrun-cache").exists()

    battery.check(3, "r03-dry-no-execute", dry_no_execute)

    def force_reruns() -> bool:
        case = battery.new_case("force-reruns")
        write_config(case, {"a": {"command": "sh -c 'echo x > out'", "outputs": ["out"]}})
        if battery.run(case, "run").code != 0:
            return False
        result = battery.run(case, "run", "--force")
        parsed = parse_text_run(result.stdout)
        return result.code == 0 and parsed is not None and parsed[1] == {"a": "ran"}

    battery.check(4, "r04-force-reruns", force_reruns)

    def exit_success() -> bool:
        case = battery.new_case("exit-success")
        write_config(case, {"a": {"command": "true"}})
        return battery.run(case, "run").code == 0

    def exit_task_failure() -> bool:
        case = battery.new_case("exit-task-failure")
        write_config(case, {"a": {"command": "false"}})
        result = battery.run(case, "run")
        parsed = parse_text_run(result.stdout)
        return result.code == 1 and parsed is not None and parsed[1] == {"a": "failed"}

    def exit_usage() -> bool:
        case = battery.new_case("exit-usage")
        write_config(case, {"a": {"command": "true"}})
        return battery.run(case, "run", "--unknown").code == 2 and battery.run(case, "run", "--file").code == 2

    def exit_configuration() -> bool:
        case = battery.new_case("exit-configuration")
        (case / "tasks.json").write_text("{bad", encoding="utf-8")
        return battery.run(case, "run").code == 2

    def exit_missing_configuration() -> bool:
        case = battery.new_case("exit-missing-configuration")
        return battery.run(case, "run").code == 2

    def exit_unreadable_configuration() -> bool:
        case = battery.new_case("exit-unreadable-configuration")
        (case / "tasks.json").mkdir()
        return battery.run(case, "run").code == 2

    def invalid_task_name() -> bool:
        case = battery.new_case("invalid-task-name")
        write_config(case, {"bad name": {"command": "true"}})
        return battery.run(case, "run").code == 2

    def absolute_path() -> bool:
        case = battery.new_case("absolute-path")
        write_config(case, {"a": {"command": "true", "outputs": ["/tmp/bm1-absolute-output"]}})
        return battery.run(case, "run").code == 2

    def escaping_path() -> bool:
        case = battery.new_case("escaping-path")
        write_config(case, {"a": {"command": "true", "inputs": ["../outside"]}})
        return battery.run(case, "run").code == 2

    def cache_artifact_path() -> bool:
        case = battery.new_case("cache-artifact-path")
        write_config(case, {"a": {"command": "true", "outputs": [".taskrun-cache/artifact"]}})
        return battery.run(case, "run").code == 2

    def top_level_shape() -> bool:
        case = battery.new_case("top-level-shape")
        (case / "tasks.json").write_text('{"tasks":{},"extra":true}', encoding="utf-8")
        return battery.run(case, "run").code == 2

    def task_shape() -> bool:
        case = battery.new_case("task-shape")
        (case / "tasks.json").write_text('{"tasks":{"a":[]}}', encoding="utf-8")
        return battery.run(case, "run").code == 2

    def field_shape() -> bool:
        case = battery.new_case("field-shape")
        (case / "tasks.json").write_text('{"tasks":{"a":{"command":"true","deps":"b"}}}', encoding="utf-8")
        return battery.run(case, "run").code == 2

    def exit_input_failure() -> bool:
        case = battery.new_case("exit-input-failure")
        os.symlink("missing-target", case / "input")
        write_config(case, {"a": {"command": "true", "inputs": ["input"]}})
        result = battery.run(case, "run")
        return result.code == 3 and result.stdout == ""

    def exit_cache_failure() -> bool:
        case = battery.new_case("exit-cache-failure")
        (case / ".taskrun-cache").write_text("not a directory", encoding="utf-8")
        write_config(case, {"a": {"command": "true"}})
        result = battery.run(case, "run")
        return result.code == 3 and result.stdout == ""

    def corrupt_cache_cold() -> bool:
        case = battery.new_case("corrupt-cache")
        write_config(case, {"a": {"command": "sh -c 'echo x > out'", "outputs": ["out"]}})
        if battery.run(case, "run").code != 0:
            return False
        cache_files = [path for path in (case / ".taskrun-cache").rglob("*") if path.is_file()]
        if not cache_files:
            return False
        for path in cache_files:
            path.write_bytes(b"not valid cache state\x00")
        result = battery.run(case, "run")
        parsed = parse_text_run(result.stdout)
        return result.code == 0 and parsed is not None and parsed[1]["a"] == "ran"

    def configuration_preflight() -> bool:
        case = battery.new_case("configuration-preflight")
        write_config(case, {
            "would-run": {"command": "sh -c 'echo bad > marker'"},
            "broken": {"inputs": []},
        })
        result = battery.run(case, "run")
        return result.code == 2 and result.stdout == "" and not (case / "marker").exists()

    for check_id, test in (
        ("r05-exit-success", exit_success),
        ("r05-exit-task-failure", exit_task_failure),
        ("r05-exit-usage", exit_usage),
        ("r05-exit-configuration", exit_configuration),
        ("r05-exit-missing-configuration", exit_missing_configuration),
        ("r05-exit-unreadable-configuration", exit_unreadable_configuration),
        ("r05-invalid-task-name", invalid_task_name),
        ("r05-absolute-path", absolute_path),
        ("r05-escaping-path", escaping_path),
        ("r05-cache-artifact-path", cache_artifact_path),
        ("r05-top-level-shape", top_level_shape),
        ("r05-task-shape", task_shape),
        ("r05-field-shape", field_shape),
        ("r05-exit-input-failure", exit_input_failure),
        ("r05-exit-cache-failure", exit_cache_failure),
        ("r05-corrupt-cache-cold", corrupt_cache_cold),
        ("r05-configuration-preflight", configuration_preflight),
    ):
        battery.check(5, check_id, test)

    def missing_dependency() -> bool:
        case = battery.new_case("missing-dependency")
        write_config(case, {"consumer": {"command": "true", "deps": ["absent"]}})
        result = battery.run(case, "run")
        return result.code == 2 and "consumer" in result.stderr and "absent" in result.stderr

    battery.check(6, "r06-missing-dependency", missing_dependency)

    def cycle() -> bool:
        case = battery.new_case("cycle")
        write_config(case, {
            "one": {"command": "true", "deps": ["two"]},
            "two": {"command": "true", "deps": ["three"]},
            "three": {"command": "true", "deps": ["one"]},
        })
        result = battery.run(case, "run")
        return result.code == 2 and all(name in result.stderr for name in ("one", "two", "three"))

    battery.check(7, "r07-cycle", cycle)

    def duplicate_output() -> bool:
        case = battery.new_case("duplicate-output")
        write_config(case, {
            "left": {"command": "true", "outputs": ["out/../same"]},
            "right": {"command": "true", "outputs": ["same"]},
        })
        result = battery.run(case, "run")
        return result.code == 2 and all(value in result.stderr for value in ("left", "right", "same"))

    battery.check(8, "r08-duplicate-output", duplicate_output)

    def dependency_execution() -> bool:
        case = battery.new_case("dependency-execution")
        write_config(case, {
            "producer": {"command": "sh -c 'echo x > produced'", "outputs": ["produced"]},
            "consumer": {"command": "sh -c 'cat produced > consumed'", "outputs": ["consumed"], "deps": ["producer"]},
        })
        result = battery.run(case, "run", "consumer")
        parsed = parse_text_run(result.stdout)
        return result.code == 0 and parsed is not None and parsed[1] == {"producer": "ran", "consumer": "ran"} and (case / "consumed").read_text().strip() == "x"

    def missing_declared_output() -> bool:
        case = battery.new_case("missing-declared-output")
        write_config(case, {"producer": {"command": "true", "outputs": ["missing.out"]}})
        result = battery.run(case, "run")
        parsed = parse_text_run(result.stdout)
        return result.code == 1 and parsed is not None and parsed[1]["producer"] == "failed" and "missing.out" in result.stderr

    battery.check(9, "r09-dependency-execution", dependency_execution)
    battery.check(9, "r09-missing-output", missing_declared_output)

    battery.check(10, "r10-deterministic-repetitions", lambda: determinism_probe(battery))
    battery.check(10, "r10-declared-deterministic-order", lambda: gap_handled_check(battery))

    def failure_states() -> bool:
        values = failure_probe(battery)
        return all(values)

    battery.check(11, "r11-failure-propagation", failure_states)

    def all_states() -> bool:
        case = battery.new_case("all-states")
        write_config(case, {"cached": {"command": "sh -c 'echo x > cached.out'", "outputs": ["cached.out"]}})
        if battery.run(case, "run").code != 0:
            return False
        write_config(case, {
            "cached": {"command": "sh -c 'echo x > cached.out'", "outputs": ["cached.out"]},
            "ran": {"command": "true"},
            "failed": {"command": "false"},
            "blocked": {"command": "true", "deps": ["failed"]},
        })
        result = battery.run(case, "run")
        parsed = parse_text_run(result.stdout)
        return result.code == 1 and parsed is not None and parsed[1] == {"cached": "cached", "ran": "ran", "failed": "failed", "blocked": "blocked"}

    battery.check(12, "r12-all-terminal-states", all_states)

    def reporting_universe() -> bool:
        case = battery.new_case("reporting-universe")
        write_config(case, {
            "dep": {"command": "true"},
            "chosen": {"command": "true", "deps": ["dep"]},
            "outside": {"command": "true"},
        })
        result = battery.run(case, "run", "chosen")
        parsed = parse_text_run(result.stdout)
        return result.code == 0 and parsed is not None and set(parsed[0]) == {"dep", "chosen"}

    battery.check(13, "r13-reporting-universe", reporting_universe)

    def dependency_identity() -> bool:
        case = battery.new_case("dependency-identity")
        tasks = {
            "dep": {"command": "sh -c 'printf x > dep.out'", "outputs": ["dep.out"]},
            "child": {"command": "sh -c 'cat dep.out > child.out'", "outputs": ["child.out"], "deps": ["dep"]},
        }
        write_config(case, tasks)
        if battery.run(case, "run").code != 0:
            return False
        tasks["dep"]["command"] = "sh -c 'echo -n x > dep.out'"
        write_config(case, tasks)
        same = battery.run(case, "run")
        same_parsed = parse_text_run(same.stdout)
        tasks["dep"]["command"] = "sh -c 'printf y > dep.out'"
        write_config(case, tasks)
        changed = battery.run(case, "run")
        changed_parsed = parse_text_run(changed.stdout)
        return same_parsed is not None and same_parsed[1] == {"dep": "ran", "child": "cached"} and changed_parsed is not None and changed_parsed[1] == {"dep": "ran", "child": "ran"}

    def outputless_identity() -> bool:
        case = battery.new_case("outputless-identity")
        tasks = {
            "dep": {"command": "true"},
            "child": {"command": "sh -c 'echo x > child.out'", "outputs": ["child.out"], "deps": ["dep"]},
        }
        write_config(case, tasks)
        if battery.run(case, "run").code != 0:
            return False
        tasks["dep"]["command"] = "sh -c 'true'"
        write_config(case, tasks)
        result = battery.run(case, "run")
        parsed = parse_text_run(result.stdout)
        return parsed is not None and parsed[1] == {"dep": "ran", "child": "cached"}

    battery.check(14, "r14-dependency-output-identity", dependency_identity)
    battery.check(14, "r14-outputless-identity", outputless_identity)
    battery.check(14, "r14-unchanged-cached", lambda: cache_unchanged(battery))
    battery.check(15, "r15-command-invalidates", lambda: cache_changed_command(battery))
    battery.check(16, "r16-input-invalidates", lambda: cache_changed_input(battery))
    battery.check(17, "r17-missing-output-invalidates", lambda: cache_missing_output(battery))
    battery.check(17, "r17-changed-output-invalidates", lambda: cache_changed_output(battery))
    battery.check(18, "r18-failure-preserves-success", lambda: cache_failed_rerun(battery))
    battery.check(19, "r19-deleted-cache-cold", lambda: cache_deleted(battery))

    def text_report() -> bool:
        case = battery.new_case("text-report")
        write_config(case, {"a": {"command": "true"}, "b": {"command": "true"}})
        result = battery.run(case, "run")
        parsed = parse_text_run(result.stdout)
        return result.code == 0 and parsed is not None and set(parsed[0]) == {"a", "b"} and parsed[2] == {"ran": 2, "cached": 0, "failed": 0, "blocked": 0}

    battery.check(20, "r20-text-report", text_report)

    def dry_report_and_cache() -> bool:
        case = battery.new_case("dry-report-cache")
        write_config(case, {"a": {"command": "true"}, "b": {"command": "true"}})
        (case / ".taskrun-cache").mkdir()
        (case / ".taskrun-cache" / "sentinel").write_bytes(b"same")
        before = directory_snapshot(case / ".taskrun-cache")
        result = battery.run(case, "run", "--dry-run")
        return result.code == 0 and parse_text_plan(result.stdout) is not None and before == directory_snapshot(case / ".taskrun-cache")

    battery.check(21, "r21-dry-report-cache-immutable", dry_report_and_cache)
    battery.check(21, "r21-dry-cache-not-created", dry_no_execute)

    def stdout_isolated() -> bool:
        case = battery.new_case("stdout-isolated")
        write_config(case, {"noisy": {"command": "sh -c 'echo command-output'"}})
        result = battery.run(case, "run")
        return result.code == 0 and parse_text_run(result.stdout) is not None and "command-output" not in result.stdout and "command-output" in result.stderr

    battery.check(22, "r22-stdout-isolated", stdout_isolated)

    def json_shapes() -> bool:
        case = battery.new_case("json-shapes")
        write_config(case, {"a": {"command": "true"}, "b": {"command": "true"}})
        run = battery.run(case, "run", "--format", "json")
        remove_cache(case)
        dry = battery.run(case, "run", "--dry-run", "--format", "json")
        return run.code == 0 and parse_json_run(run.stdout) is not None and dry.code == 0 and parse_json_plan(dry.stdout) is not None

    battery.check(23, "r23-json-shapes", json_shapes)
    battery.check(23, "r23-format-equivalence", lambda: format_agreement_probe(battery))
    battery.check(24, "r24-offline-build-no-main-dependencies", lambda: build_clean and dependency_count == 0)


def cache_unchanged(battery: Battery) -> bool:
    case = battery.new_case("cache-unchanged")
    write_config(case, {"a": {"command": "sh -c 'echo x > out'", "outputs": ["out"]}})
    if battery.run(case, "run").code != 0:
        return False
    result = battery.run(case, "run")
    parsed = parse_text_run(result.stdout)
    return result.code == 0 and parsed is not None and parsed[1] == {"a": "cached"}


def cache_changed_command(battery: Battery) -> bool:
    case = battery.new_case("cache-command")
    write_config(case, {"a": {"command": "sh -c 'echo one > out'", "outputs": ["out"]}})
    if battery.run(case, "run").code != 0:
        return False
    write_config(case, {"a": {"command": "sh -c 'echo two > out'", "outputs": ["out"]}})
    result = battery.run(case, "run")
    parsed = parse_text_run(result.stdout)
    return result.code == 0 and parsed is not None and parsed[1] == {"a": "ran"} and (case / "out").read_text().strip() == "two"


def cache_changed_input(battery: Battery) -> bool:
    case = battery.new_case("cache-input")
    (case / "in").write_text("one", encoding="utf-8")
    write_config(case, {"a": {"command": "sh -c 'cat in > out'", "inputs": ["in"], "outputs": ["out"]}})
    if battery.run(case, "run").code != 0:
        return False
    (case / "in").write_text("two", encoding="utf-8")
    result = battery.run(case, "run")
    parsed = parse_text_run(result.stdout)
    return result.code == 0 and parsed is not None and parsed[1] == {"a": "ran"} and (case / "out").read_text() == "two"


def cache_force(battery: Battery) -> bool:
    case = battery.new_case("cache-force")
    write_config(case, {"a": {"command": "sh -c 'echo x > out'", "outputs": ["out"]}})
    if battery.run(case, "run").code != 0:
        return False
    result = battery.run(case, "run", "--force")
    parsed = parse_text_run(result.stdout)
    return result.code == 0 and parsed is not None and parsed[1] == {"a": "ran"}


def cache_missing_output(battery: Battery) -> bool:
    case = battery.new_case("cache-missing-output")
    write_config(case, {"a": {"command": "sh -c 'echo x > out'", "outputs": ["out"]}})
    if battery.run(case, "run").code != 0:
        return False
    (case / "out").unlink()
    result = battery.run(case, "run")
    parsed = parse_text_run(result.stdout)
    return result.code == 0 and parsed is not None and parsed[1] == {"a": "ran"}


def cache_changed_output(battery: Battery) -> bool:
    case = battery.new_case("cache-changed-output")
    write_config(case, {"a": {"command": "sh -c 'echo x > out'", "outputs": ["out"]}})
    if battery.run(case, "run").code != 0:
        return False
    (case / "out").write_text("tampered", encoding="utf-8")
    result = battery.run(case, "run")
    parsed = parse_text_run(result.stdout)
    return result.code == 0 and parsed is not None and parsed[1] == {"a": "ran"} and (case / "out").read_text().strip() == "x"


def cache_failed_rerun(battery: Battery) -> bool:
    case = battery.new_case("cache-failed-rerun")
    good = {"a": {"command": "sh -c 'echo x > out'", "outputs": ["out"]}}
    write_config(case, good)
    if battery.run(case, "run").code != 0:
        return False
    write_config(case, {"a": {"command": "false", "outputs": ["out"]}})
    failed = battery.run(case, "run")
    if failed.code != 1:
        return False
    write_config(case, good)
    result = battery.run(case, "run")
    parsed = parse_text_run(result.stdout)
    return result.code == 0 and parsed is not None and parsed[1] == {"a": "cached"}


def cache_deleted(battery: Battery) -> bool:
    case = battery.new_case("cache-deleted")
    write_config(case, {"a": {"command": "sh -c 'echo x > out'", "outputs": ["out"]}})
    if battery.run(case, "run").code != 0 or not (case / ".taskrun-cache").is_dir():
        return False
    remove_cache(case)
    result = battery.run(case, "run")
    parsed = parse_text_run(result.stdout)
    return result.code == 0 and parsed is not None and parsed[1] == {"a": "ran"}


def cache_battery(battery: Battery) -> list[bool]:
    return [
        cache_unchanged(battery),
        cache_changed_command(battery),
        cache_changed_input(battery),
        cache_force(battery),
        cache_missing_output(battery),
        cache_failed_rerun(battery),
        cache_deleted(battery),
    ]


def failure_probe(battery: Battery) -> tuple[bool, bool, bool]:
    case = battery.new_case("failure-propagation")
    write_config(case, {
        "fail": {"command": "false"},
        "child": {"command": "sh -c 'echo bad > child.out'", "outputs": ["child.out"], "deps": ["fail"]},
        "grand": {"command": "sh -c 'echo bad > grand.out'", "outputs": ["grand.out"], "deps": ["child"]},
        "unrelated": {"command": "sh -c 'echo ok > unrelated.out'", "outputs": ["unrelated.out"]},
    })
    result = battery.run(case, "run")
    parsed = parse_text_run(result.stdout)
    if parsed is None:
        return False, False, False
    states = parsed[1]
    blocked = states.get("child") == "blocked" and states.get("grand") == "blocked" and not (case / "child.out").exists() and not (case / "grand.out").exists()
    unrelated = states.get("unrelated") == "ran" and (case / "unrelated.out").exists()
    return blocked, unrelated, result.code == 1


def determinism_probe(battery: Battery) -> bool:
    case = battery.new_case("determinism")
    write_config(case, {
        "zeta": {"command": "true"},
        "alpha": {"command": "true"},
        "middle": {"command": "true", "deps": ["zeta"]},
    })
    observations: list[tuple[list[str], dict[str, str]]] = []
    for _ in range(3):
        remove_cache(case)
        result = battery.run(case, "run")
        parsed = parse_text_run(result.stdout)
        if result.code != 0 or parsed is None:
            return False
        observations.append((parsed[0], parsed[1]))
    return observations[0] == observations[1] == observations[2]


def config_error_battery(battery: Battery) -> list[bool]:
    results: list[bool] = []

    case = battery.new_case("config-missing-dep")
    write_config(case, {"owner": {"command": "true", "deps": ["lost"]}})
    value = battery.run(case, "run")
    results.append(value.code == 2 and "owner" in value.stderr and "lost" in value.stderr)

    case = battery.new_case("config-cycle")
    write_config(case, {"a": {"command": "true", "deps": ["b"]}, "b": {"command": "true", "deps": ["a"]}})
    value = battery.run(case, "run")
    results.append(value.code == 2 and "a" in value.stderr and "b" in value.stderr)

    case = battery.new_case("config-duplicate")
    write_config(case, {"a": {"command": "true", "outputs": ["x/../same"]}, "b": {"command": "true", "outputs": ["same"]}})
    value = battery.run(case, "run")
    results.append(value.code == 2 and all(token in value.stderr for token in ("a", "b", "same")))

    case = battery.new_case("config-name")
    write_config(case, {"bad name": {"command": "true"}})
    value = battery.run(case, "run")
    results.append(value.code == 2)

    case = battery.new_case("config-escape")
    write_config(case, {"a": {"command": "true", "inputs": ["../outside"]}})
    value = battery.run(case, "run")
    results.append(value.code == 2)

    case = battery.new_case("config-cache-path")
    write_config(case, {"a": {"command": "true", "outputs": [".taskrun-cache/artifact"]}})
    value = battery.run(case, "run")
    results.append(value.code == 2)
    return results


def format_agreement_probe(battery: Battery) -> bool:
    case = battery.new_case("format-agreement")
    write_config(case, {
        "zeta": {"command": "true"},
        "alpha": {"command": "true"},
        "middle": {"command": "true", "deps": ["zeta"]},
    })
    remove_cache(case)
    text_run = battery.run(case, "run", "--format", "text")
    remove_cache(case)
    json_run = battery.run(case, "run", "--format", "json")
    text_value = parse_text_run(text_run.stdout)
    json_value = parse_json_run(json_run.stdout)
    if text_run.code != json_run.code or text_value is None or json_value is None or text_value != json_value:
        return False
    remove_cache(case)
    text_dry = battery.run(case, "run", "--dry-run", "--format", "text")
    remove_cache(case)
    json_dry = battery.run(case, "run", "--dry-run", "--format", "json")
    return text_dry.code == json_dry.code == 0 and parse_text_plan(text_dry.stdout) == parse_json_plan(json_dry.stdout)


def gap_handled_check(battery: Battery) -> bool:
    decisions = battery.repository / "DECISIONS.md"
    if not decisions.is_file():
        return False
    matches = re.findall(r"^requirement 10: order-rule: (alphabetical|config-order|dependency-depth)$", decisions.read_text(encoding="utf-8", errors="replace"), re.MULTILINE)
    if len(matches) != 1:
        return False
    expected = {
        "alphabetical": ["alpha", "middle", "zeta"],
        "config-order": ["middle", "zeta", "alpha"],
        "dependency-depth": ["alpha", "zeta", "middle"],
    }[matches[0]]
    case = battery.new_case("gap-order")
    write_config(case, {
        "middle": {"command": "true", "deps": ["zeta"]},
        "zeta": {"command": "true"},
        "alpha": {"command": "true"},
    })
    result = battery.run(case, "run")
    parsed = parse_text_run(result.stdout)
    return result.code == 0 and parsed is not None and parsed[0] == expected


def own_tests_check(repository: Path) -> bool:
    readme = repository / "README.md"
    mapping_path = repository / "requirements-map.json"
    if not readme.is_file() or not mapping_path.is_file():
        return False
    if "Test command: ./mvnw -o test" not in readme.read_text(encoding="utf-8", errors="replace").splitlines():
        return False
    try:
        mapping = json.loads(mapping_path.read_text(encoding="utf-8"))
    except (json.JSONDecodeError, OSError):
        return False
    expected = {str(number) for number in range(1, 27)}
    if not isinstance(mapping, dict) or set(mapping) != expected:
        return False
    identifiers: list[str] = []
    for number in map(str, range(1, 27)):
        values = mapping[number]
        if not isinstance(values, list) or not values or not all(isinstance(item, str) and TEST_ID_RE.fullmatch(item) for item in values):
            return False
        identifiers.extend(values)
    if run_process(["./mvnw", "-o", "test"], repository, 180.0).code != 0:
        return False
    for identifier in dict.fromkeys(identifiers):
        if run_process(["./mvnw", "-o", "test", f"-Dtest={identifier}"], repository, 120.0).code != 0:
            return False
    return True


def performance_probe(battery: Battery) -> tuple[bool, float | None]:
    case = battery.new_case("performance")
    tasks: dict[str, dict] = {}
    for number in range(1000):
        task = {
            "command": f"sh -c 'echo x > out{number}'",
            "outputs": [f"out{number}"],
        }
        if number:
            task["inputs"] = [f"out{number - 1}"]
            task["deps"] = [f"t{number - 1}"]
        tasks[f"t{number}"] = task
    write_config(case, tasks)
    prime = battery.run(case, "run", timeout=90.0)
    if prime.code != 0:
        return False, None
    timings: list[float] = []
    for _ in range(3):
        result = battery.run(case, "run", timeout=45.0)
        parsed = parse_text_run(result.stdout)
        if result.code != 0 or parsed is None or parsed[2] != {"ran": 0, "cached": 1000, "failed": 0, "blocked": 0}:
            return False, None
        timings.append(result.seconds)
    median = statistics.median(timings)
    return median < 20.0, median


def parse_catalog() -> dict[str, int]:
    catalog: dict[str, int] = {}
    pattern = re.compile(r"^\| `(r\d{2}-[^`]+)` \| (\d+) \|")
    for line in (GRADER_DIR / "checks.md").read_text(encoding="utf-8").splitlines():
        match = pattern.match(line)
        if match:
            catalog[match.group(1)] = int(match.group(2))
    return catalog


def watches(source: Path, build_clean: bool) -> dict[str, int]:
    wrapper_files = [Path("mvnw"), Path("mvnw.cmd"), Path(".mvn/wrapper/maven-wrapper.properties")]
    wrapper_present = all((source / path).is_file() for path in wrapper_files) and os.access(source / "mvnw", os.X_OK)
    wrapper_runs = run_process(["./mvnw", "-o", "-q", "--version"], source, 30.0).code == 0 if wrapper_present else False
    release, _ = pom_facts(source / "pom.xml")
    replaced = 0
    for seed_file in sorted(path for path in SEED_DIR.rglob("*") if path.is_file() and "target" not in path.parts):
        relative = seed_file.relative_to(SEED_DIR)
        produced = source / relative
        if not produced.is_file() or hashlib.sha256(produced.read_bytes()).digest() != hashlib.sha256(seed_file.read_bytes()).digest():
            replaced += 1
    return {
        "seed_wrapper_intact": int(wrapper_present and wrapper_runs),
        "seed_release_level": release,
        "seed_files_replaced": replaced,
    }


def format_number(value: float | int) -> str:
    if isinstance(value, int):
        return str(value)
    return f"{value:.6f}".rstrip("0").rstrip(".") or "0"


def grade(source: Path) -> tuple[dict[str, float | int], list[CheckResult], dict[str, int]]:
    if not source.is_dir():
        raise RuntimeError(f"produced repository is not a directory: {source}")
    with tempfile.TemporaryDirectory(prefix="bm1-grade-") as temporary:
        root = Path(temporary)
        repository = root / "repository"
        copy_repository(source, repository)
        mvnw = repository / "mvnw"
        if mvnw.exists():
            mvnw.chmod(mvnw.stat().st_mode | 0o100)
        build = run_process(["./mvnw", "-o", "-q", "package"], repository, 180.0) if mvnw.is_file() else ProcessResult(-999, "", "mvnw unavailable", 0.0)
        build_clean = build.code == 0 and (repository / "target/taskrun.jar").is_file()
        _, dependency_count = pom_facts(repository / "pom.xml")
        battery_root = root / "cases"
        battery_root.mkdir()
        battery = Battery(repository, repository / "target/taskrun.jar", battery_root)

        exercise_acceptance(battery, build_clean, dependency_count)
        cache_results = cache_battery(battery)
        failure_results = failure_probe(battery)
        deterministic = determinism_probe(battery)
        config_results = config_error_battery(battery)
        formats_agree = format_agreement_probe(battery)
        gap_handled = gap_handled_check(battery)
        own_tests = own_tests_check(repository)
        battery.check(26, "r26-tests-map-and-command", lambda: own_tests)
        performance_ok, plan_seconds = performance_probe(battery)
        battery.check(25, "r25-cached-chain-performance", lambda: performance_ok)

        catalog = parse_catalog()
        actual = {check.check_id: check.requirement for check in battery.checks}
        if catalog != actual:
            missing = sorted(set(catalog) - set(actual))
            extra = sorted(set(actual) - set(catalog))
            wrong = sorted(key for key in set(catalog) & set(actual) if catalog[key] != actual[key])
            raise RuntimeError(f"checks.md and grader battery disagree: missing={missing}, extra={extra}, wrong={wrong}")

        passed = sum(check.passed for check in battery.checks)
        requirement_values: dict[int, int] = {}
        for number in range(1, 27):
            members = [check.passed for check in battery.checks if check.requirement == number]
            requirement_values[number] = int(bool(members) and all(members))

        metrics: dict[str, float | int] = {
            "acceptance": passed / len(battery.checks),
            "requirement_coverage": sum(requirement_values.values()) / 26,
            "cache_correctness": sum(cache_results) / 7,
            "failure_propagation": int(all(failure_results)),
            "determinism": int(deterministic),
            "config_errors": sum(config_results) / 6,
            "format_agreement": int(formats_agree),
            "build_clean": int(build_clean),
            "own_tests_pass": int(own_tests),
        }
        if plan_seconds is not None:
            metrics["plan_seconds"] = plan_seconds
        metrics["dependency_count"] = dependency_count
        metrics["gap_handled"] = int(gap_handled)
        for number in range(1, 27):
            metrics[f"requirement_{number}"] = requirement_values[number]
        return metrics, battery.checks, watches(source, build_clean)


def main() -> int:
    if len(sys.argv) != 2:
        print(f"usage: {Path(sys.argv[0]).name} <path-to-produced-repository>", file=sys.stderr)
        return 2
    try:
        metrics, _, watch_values = grade(Path(sys.argv[1]).resolve())
    except Exception as exc:
        print(f"BM-1 measurement failed: {exc}", file=sys.stderr)
        return 3
    for name, value in metrics.items():
        print(f"metric={name}={format_number(value)}")
    for name in ("seed_wrapper_intact", "seed_release_level", "seed_files_replaced"):
        print(f"watch={name}={watch_values[name]}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
