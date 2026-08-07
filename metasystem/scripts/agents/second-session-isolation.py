#!/usr/bin/env python3
"""Copy adapter-declared local configuration and audit worktree isolation."""

from __future__ import annotations

import argparse
import os
import shutil
from pathlib import Path


def declared_paths(manifest: Path) -> list[Path]:
    result: list[Path] = []
    for raw in manifest.read_text(encoding="utf-8").splitlines():
        if not raw:
            continue
        relative = Path(raw)
        if relative.is_absolute() or ".." in relative.parts:
            raise SystemExit(f"adapter local-config-path is unsafe: {raw}")
        result.append(relative)
    return result


def copy_and_audit(args: argparse.Namespace) -> Path:
    source_root = args.source_root.resolve()
    destination_root = args.destination_root.resolve()
    paths = declared_paths(args.manifest)
    for relative in paths:
        source = source_root / relative
        target = destination_root / relative
        if not source.exists():
            continue
        if target.exists() or target.is_symlink():
            continue
        target.parent.mkdir(parents=True, exist_ok=True)
        if source.is_dir():
            shutil.copytree(source, target, symlinks=False, copy_function=shutil.copy2)
        else:
            shutil.copy2(source.resolve(), target, follow_symlinks=True)

    for relative in paths:
        target = destination_root / relative
        if not target.exists():
            continue
        resolved = target.resolve()
        try:
            resolved.relative_to(destination_root)
        except ValueError:
            raise SystemExit(f"isolation audit failed: {relative} resolves outside the new worktree")
        try:
            resolved.relative_to(source_root)
        except ValueError:
            pass
        else:
            raise SystemExit(
                f"isolation audit failed: {relative} still resolves into the primary checkout"
            )

    harness_relative = Path(os.path.relpath(args.harness_root.resolve(), source_root))
    new_harness = (destination_root / harness_relative).resolve()
    if new_harness == args.harness_root.resolve():
        raise SystemExit("isolation audit failed: both sessions resolve one metasystem artifacts root")
    if not new_harness.is_dir():
        raise SystemExit(f"isolation audit failed: the new harness is absent: {new_harness}")
    return new_harness


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--source-root", required=True, type=Path)
    parser.add_argument("--destination-root", required=True, type=Path)
    parser.add_argument("--manifest", required=True, type=Path)
    parser.add_argument("--harness-root", required=True, type=Path)
    return parser.parse_args()


def main() -> int:
    print(copy_and_audit(parse_args()))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
