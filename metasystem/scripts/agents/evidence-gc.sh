#!/usr/bin/env bash
# Collect chain evidence that is already durable elsewhere.
#
# The standing rule has two halves: raw run evidence under gitignored
# artifacts/ is mirrored to the durable evidence root before it counts as
# disposable — and disposable evidence eventually gets disposed of. The second
# half never ran anywhere until a human browsing the template found months of
# closed critique chains living beside live state. artifacts/ holds LIVE
# state; history belongs to the evidence root, outside the repository.
#
# Safety: a chain is collected only when every job of it is terminal, and
# every file under its payload directory is listed in the mirror's manifest
# with a matching hash. Anything else is refused loudly, never skipped
# silently. Job records (jobs/*.json) always stay: they are the registry.
set -euo pipefail
metasystem_gc_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
cd "$metasystem_gc_root"

evidence=$(scripts/metasystem-config.sh get --key evidence.root --default '' 2>/dev/null || true)
[[ "$evidence" == /* ]] || { echo "evidence-gc refused: evidence.root is not configured" >&2; exit 1; }

python3 - "$metasystem_gc_root" "$evidence" <<'PY'
import hashlib
import json
import re
import sys
from pathlib import Path

root, evidence = Path(sys.argv[1]), Path(sys.argv[2])
agents = root / "artifacts" / "agents"
jobs = agents / "jobs"
TERMINAL = {"completed", "failed", "timeout", "cancelled"}
RESERVED = {"jobs", "worktrees", "record-locks", "supervision", "mains", "capabilities", "missions"}

def digest(path):
    value = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1 << 20), b""):
            value.update(chunk)
    return value.hexdigest()

collected, kept = [], []
for chain_dir in sorted(p for p in agents.iterdir() if p.is_dir() and p.name not in RESERVED):
    chain = chain_dir.name
    records = [json.loads(p.read_text()) for p in jobs.glob(f"{chain}*.json")
               if re.fullmatch(re.escape(chain) + r"(-r[0-9]+)?", p.stem)]
    if not records:
        kept.append((chain, "no job records; not this tool's to judge")); continue
    if any(r.get("status") not in TERMINAL for r in records):
        kept.append((chain, "a round is still live")); continue
    manifest_path = evidence / "agents" / chain / "manifest.json"
    if not manifest_path.exists():
        kept.append((chain, "no mirror manifest")); continue
    files = json.loads(manifest_path.read_text())["files"]
    unaccounted = []
    for path in sorted(p for p in chain_dir.rglob("*") if p.is_file()):
        relative = path.relative_to(chain_dir).as_posix()
        entry = files.get(relative)
        if entry is None or entry.get("sha256") not in (digest(path), entry.get("sha256") if relative.startswith("jobs/") else None):
            if entry is None or entry.get("sha256") != digest(path):
                unaccounted.append(relative)
    if unaccounted:
        kept.append((chain, f"mirror does not account for: {', '.join(unaccounted[:3])}")); continue
    import shutil
    shutil.rmtree(chain_dir)
    for log in jobs.glob(f"{chain}*.log"):
        if f"jobs/{log.name}" in files and files[f"jobs/{log.name}"].get("sha256") == digest(log):
            log.unlink()
    collected.append(chain)

# Beyond chain payloads, three residue classes accumulate per terminal job and
# nobody else cleans them: heartbeats, lock files and lifecycle dirs, and the
# mktemp leftovers of interrupted operations. And capability snapshots
# supersede each other: only the newest per runtime+version can ever be read
# again, because dispatch matches the CURRENT probe fingerprint.
import shutil, time
def job_status(job):
    path = jobs / f"{job}.json"
    try:
        return json.loads(path.read_text()).get("status")
    except (OSError, ValueError):
        return None

residue = 0
hb = agents / "hb"
if hb.is_dir():
    for entry in sorted(hb.iterdir()):
        job = entry.name.removesuffix(".start").removesuffix(".waiting")
        if job_status(job) in TERMINAL:
            entry.unlink(missing_ok=True); residue += 1
locks_dir = agents / "record-locks"
if locks_dir.is_dir():
    for entry in sorted(locks_dir.iterdir()):
        job = entry.name.removesuffix(".lock").removesuffix(".lifecycle.d")
        if entry.name.endswith((".lock", ".lifecycle.d")):
            if job_status(job) in TERMINAL:
                (shutil.rmtree if entry.is_dir() else Path.unlink)(entry); residue += 1
        elif entry.is_file() and time.time() - entry.stat().st_mtime > 3600:
            entry.unlink(); residue += 1
caps = agents / "capabilities"
if caps.is_dir():
    newest = {}
    for snap in sorted(caps.glob("*.json")):
        stem = snap.name.rsplit("-", 2)[0]  # runtime-version-confighash
        runtime_version = "-".join(stem.split("-")[:2])
        newest.setdefault(runtime_version, []).append(snap)
    for runtime_version, snaps in newest.items():
        for snap in snaps[:-1]:
            snap.unlink(); residue += 1

for chain in collected:
    print(f"collected {chain}")
print(f"residue removed: {residue} heartbeat, lock, temp and superseded-snapshot entries")
for chain, reason in kept:
    print(f"kept      {chain}: {reason}")
print(f"evidence-gc: {len(collected)} collected, {len(kept)} kept")
PY
