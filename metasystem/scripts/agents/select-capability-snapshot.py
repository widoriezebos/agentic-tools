#!/usr/bin/env python3
"""Select and validate the capability snapshot for one dispatch."""

from __future__ import annotations

import argparse
import json
import re
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


def load_snapshot(path: Path, runtime: str, version: str) -> tuple[datetime, dict[str, Any]] | None:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
        captured = datetime.fromisoformat(value["capturedAt"].replace("Z", "+00:00"))
    except (OSError, ValueError, KeyError, TypeError):
        return None
    if value.get("runtime") != runtime or value.get("cliVersion") != version:
        return None
    return captured, value


def valid_key_hashes(value: Any) -> bool:
    return isinstance(value, dict) and all(
        isinstance(key, str)
        and isinstance(digest, str)
        and re.fullmatch(r"[0-9a-f]{64}", digest)
        for key, digest in value.items()
    )


def changed_key_suffix(
    current_hashes: dict[str, str],
    snapshots: list[tuple[datetime, Path, dict[str, Any]]],
) -> str:
    if not snapshots:
        return ""
    _captured, _path, previous = max(snapshots, key=lambda item: (item[0], item[1].name))
    previous_hashes = previous.get("configKeyHashes")
    if not valid_key_hashes(previous_hashes):
        return ""
    changed = sorted(
        key
        for key in set(current_hashes) | set(previous_hashes)
        if current_hashes.get(key) != previous_hashes.get(key)
    )
    if not changed:
        return ""
    return "; changed configuration keys: " + ", ".join(changed)


def parse_identity(raw: str, runtime: str) -> tuple[str, str, dict[str, str]]:
    try:
        value = json.loads(raw)
    except (TypeError, ValueError) as error:
        raise SystemExit(f"{runtime} adapter returned a malformed configuration identity: {error}")
    if not isinstance(value, dict) or value.get("runtime") != runtime:
        raise SystemExit(f"{runtime} adapter returned a malformed configuration identity")
    version = value.get("cliVersion")
    config_hash = value.get("configHash")
    key_hashes = value.get("configKeyHashes")
    if (
        not isinstance(version, str)
        or not re.fullmatch(r"[A-Za-z0-9._-]+", version)
        or not isinstance(config_hash, str)
        or not re.fullmatch(r"[A-Za-z0-9._-]+", config_hash)
        or not valid_key_hashes(key_hashes)
    ):
        raise SystemExit(f"{runtime} adapter returned a malformed configuration identity")
    return version, config_hash, key_hashes


def select(args: argparse.Namespace) -> None:
    root = args.root.resolve()
    version, config_hash, current_hashes = parse_identity(args.identity, args.runtime)
    directory = root / "artifacts" / "agents" / "capabilities"
    all_snapshots: list[tuple[datetime, Path, dict[str, Any]]] = []
    if directory.exists():
        for path in directory.glob(f"{args.runtime}-{version}-*.json"):
            loaded = load_snapshot(path, args.runtime, version)
            if loaded is not None:
                captured, value = loaded
                all_snapshots.append((captured, path, value))
    candidates = [
        item for item in all_snapshots if item[2].get("configHash") == config_hash
    ]
    if not candidates:
        suffix = changed_key_suffix(current_hashes, all_snapshots)
        raise SystemExit(
            f"no capability snapshot matches {args.runtime} {version} {config_hash}{suffix}; "
            f"run {args.runtime} adapter probe"
        )
    captured, path, snapshot = max(candidates, key=lambda item: (item[0], item[1].name))
    age_days = (
        datetime.now(timezone.utc) - captured.astimezone(timezone.utc)
    ).total_seconds() / 86400
    if age_days > args.max_age:
        raise SystemExit(
            f"capability snapshot is stale ({age_days:.1f} days); "
            f"re-run {args.runtime} adapter probe"
        )

    requirements_path = root / "scripts" / "agents" / "roles" / f"{args.role}.requirements.json"
    try:
        requirements = json.loads(requirements_path.read_text(encoding="utf-8"))
        envelope = json.loads(args.envelope.read_text(encoding="utf-8"))
    except (OSError, ValueError) as error:
        raise SystemExit(f"cannot evaluate capabilities: {error}")
    caps = snapshot.get("capabilities", {})
    envelope_enforcement = snapshot.get("envelopeEnforcement")
    if (
        not isinstance(envelope_enforcement, dict)
        or set(envelope_enforcement) != {"writeRoots", "readRoots", "network"}
        or any(value not in {"mapped", "notEnforced"} for value in envelope_enforcement.values())
    ):
        raise SystemExit(
            "capability snapshot has no valid envelope enforcement declaration; re-run adapter probe"
        )
    handshake_timeout = caps.get("sessionEstablishedTimeoutSec", 2)
    if (
        isinstance(handshake_timeout, bool)
        or not isinstance(handshake_timeout, int)
        or not 1 <= handshake_timeout <= 60
    ):
        raise SystemExit("capability snapshot has an invalid session-established timeout")
    missing = [name for name in requirements.get("required", []) if caps.get(name) is not True]
    if missing:
        raise SystemExit("required runtime capabilities are absent: " + ", ".join(sorted(missing)))
    fallbacks = []
    for name, declaration in requirements.get("optional", {}).items():
        if caps.get(name) is not True:
            fallbacks.append({"capability": name, "fallback": declaration.get("fallback")})
    waivers = requirements.get("waivers", {})
    unverified = snapshot.get("permissions", {}).get("unverified", [])
    for field in unverified:
        if envelope.get(field) == "deny" and args.runtime not in waivers.get(field, []):
            raise SystemExit(
                f"runtime cannot verify restrictive permission field {field}; "
                "add an explicit role waiver or choose another runtime"
            )
    result = {
        "path": str(path.relative_to(root)),
        "fallbacks": fallbacks,
        "sessionEstablishedSignal": caps.get("sessionEstablishedSignal") is True,
        "sessionEstablishedTimeoutSec": handshake_timeout,
        "resume": caps.get("resume") is True,
    }
    args.output.write_text(json.dumps(result, indent=2, sort_keys=True) + "\n", encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", required=True, type=Path)
    parser.add_argument("--runtime", required=True)
    parser.add_argument("--role", required=True)
    parser.add_argument("--identity", required=True)
    parser.add_argument("--max-age", required=True, type=int)
    parser.add_argument("--envelope", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    if args.max_age < 0:
        raise SystemExit("capability snapshot maximum age must be non-negative")
    select(args)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
