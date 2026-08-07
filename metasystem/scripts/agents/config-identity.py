#!/usr/bin/env python3
"""Build the versioned, canonical configuration identity for one adapter."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import sys
from datetime import date, datetime, time
from pathlib import Path
from typing import Any

try:
    import tomllib
except ModuleNotFoundError as error:  # pragma: no cover - Python 3.11 is required by the repository
    raise SystemExit(f"TOML support is unavailable: {error}")


def canonical_json(value: Any) -> str:
    return json.dumps(
        value,
        allow_nan=False,
        ensure_ascii=False,
        separators=(",", ":"),
        sort_keys=True,
    )


def normalize(value: Any) -> Any:
    if isinstance(value, dict):
        return {str(key): normalize(item) for key, item in value.items()}
    if isinstance(value, list):
        return [normalize(item) for item in value]
    if isinstance(value, (datetime, date, time)):
        return value.isoformat()
    return value


def flatten(value: dict[str, Any]) -> dict[str, Any]:
    result: dict[str, Any] = {}

    def visit(item: Any, prefix: str) -> None:
        if isinstance(item, dict) and item:
            for key in sorted(item):
                child = f"{prefix}.{key}" if prefix else key
                visit(item[key], child)
            return
        if prefix in result:
            raise ValueError(f"configuration key is ambiguous after flattening: {prefix}")
        result[prefix] = normalize(item)

    for top_level_key in sorted(value):
        visit(value[top_level_key], top_level_key)
    return result


def load_source(path: Path) -> dict[str, Any]:
    if path.suffix == ".json":
        value = json.loads(
            path.read_text(encoding="utf-8"),
            parse_constant=lambda token: (_ for _ in ()).throw(
                ValueError(f"non-JSON number {token}")
            ),
        )
    elif path.suffix == ".toml":
        value = tomllib.loads(path.read_text(encoding="utf-8"))
    else:
        raise ValueError(f"unsupported configuration source type: {path}")
    if not isinstance(value, dict):
        raise ValueError(f"configuration source must contain an object or table: {path}")
    return flatten(value)


def numeric_version(value: str) -> tuple[int, ...] | None:
    if not re.fullmatch(r"[0-9]+(?:\.[0-9]+)*", value):
        return None
    return tuple(int(part) for part in value.split("."))


def version_in_range(version: str, minimum: str, maximum: str) -> bool:
    actual = numeric_version(version)
    lower = numeric_version(minimum)
    wildcard = maximum.endswith(".x")
    upper_text = maximum[:-2] if wildcard else maximum
    upper = numeric_version(upper_text)
    if actual is None or lower is None or upper is None:
        return False
    width = max(len(actual), len(lower), len(upper))
    padded_actual = actual + (0,) * (width - len(actual))
    padded_lower = lower + (0,) * (width - len(lower))
    if padded_actual < padded_lower:
        return False
    if wildcard:
        return actual[: len(upper)] == upper
    padded_upper = upper + (0,) * (width - len(upper))
    return padded_actual <= padded_upper


def load_filter(path: Path, version: str) -> tuple[list[str], str | None]:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
        if not isinstance(value, dict) or set(value) != {"cliVersionRange", "keys"}:
            raise ValueError("top level must contain exactly cliVersionRange and keys")
        version_range = value["cliVersionRange"]
        if (
            not isinstance(version_range, dict)
            or set(version_range) != {"min", "max"}
            or not all(isinstance(version_range[name], str) for name in ("min", "max"))
        ):
            raise ValueError("cliVersionRange must contain string min and max values")
        entries = value["keys"]
        if not isinstance(entries, list):
            raise ValueError("keys must be an array")
        paths: list[str] = []
        for entry in entries:
            if (
                not isinstance(entry, dict)
                or set(entry) != {"path", "reason", "source"}
                or not all(isinstance(entry[name], str) and entry[name] for name in entry)
            ):
                raise ValueError("each key must contain non-empty path, reason, and source strings")
            paths.append(entry["path"])
        minimum, maximum = version_range["min"], version_range["max"]
        if not version_in_range(version, minimum, maximum):
            return [], f"CLI version {version} is outside filter range {minimum} through {maximum}"
        return paths, None
    except (OSError, ValueError, json.JSONDecodeError) as error:
        return [], f"filter is malformed or unparsable: {error}"


def excluded(key: str, filtered_paths: list[str]) -> bool:
    # A table path names its canonical leaf values as well as an exact scalar.
    return any(key == path or key.startswith(path + ".") for path in filtered_paths)


def build_identity(
    runtime: str,
    version: str,
    filter_path: Path,
    source_paths: list[Path],
) -> dict[str, Any]:
    flattened: dict[str, Any] = {}
    for path in source_paths:
        path = path.expanduser().resolve(strict=False)
        if not path.is_file():
            continue
        try:
            flattened.update(load_source(path))
        except (OSError, ValueError, json.JSONDecodeError, tomllib.TOMLDecodeError) as error:
            raise SystemExit(f"cannot canonicalize configuration source {path}: {error}")

    filtered_paths, warning = load_filter(filter_path, version)
    if warning:
        print(
            f"warning: {runtime} configuration filter {filter_path} {warning}; "
            "hashing all canonical configuration keys",
            file=sys.stderr,
        )
    identity_map = {
        key: value for key, value in sorted(flattened.items()) if not excluded(key, filtered_paths)
    }
    encoded = canonical_json(identity_map).encode("utf-8")
    key_hashes = {
        key: hashlib.sha256(canonical_json(value).encode("utf-8")).hexdigest()
        for key, value in identity_map.items()
    }
    return {
        "runtime": runtime,
        "cliVersion": version,
        "configHash": hashlib.sha256(encoded).hexdigest()[:24],
        "configKeyHashes": key_hashes,
    }


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--runtime", required=True)
    parser.add_argument("--version", required=True)
    parser.add_argument("--filter", required=True, type=Path)
    parser.add_argument("sources", nargs="*", type=Path)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    value = build_identity(args.runtime, args.version, args.filter, args.sources)
    print(canonical_json(value))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
