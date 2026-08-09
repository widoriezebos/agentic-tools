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

if [[ ${1:-} != __lease-held ]]; then
  lease_result=$(scripts/agents/worktree-lease.py --root "$metasystem_gc_root" \
    require-holder --caller-pid "$$") || exit $?
  lease_epoch=$(python3 -c 'import json,sys; v=json.loads(sys.argv[1]); print("" if v.get("claimEpoch") is None else v["claimEpoch"])' "$lease_result")
  if [[ -n "$lease_epoch" ]]; then
    exec scripts/agents/worktree-lease.py --root "$metasystem_gc_root" run-held \
      --caller-pid "$$" --expected-epoch "$lease_epoch" -- "$0" __lease-held "$lease_epoch"
  fi
  exec scripts/agents/worktree-lease.py --root "$metasystem_gc_root" run-held \
    --caller-pid "$$" -- "$0" __lease-held human
fi
shift
expected_epoch=${1:-}
[[ -n "$expected_epoch" ]] || exit 2
shift
if [[ "$expected_epoch" =~ ^[1-9][0-9]*$ ]]; then
  scripts/agents/worktree-lease.py --root "$metasystem_gc_root" require-holder \
    --caller-pid "$$" --expected-epoch "$expected_epoch" >/dev/null
else
  [[ "$expected_epoch" == human ]] || exit 2
  scripts/agents/worktree-lease.py --root "$metasystem_gc_root" require-holder \
    --caller-pid "$$" >/dev/null
fi

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
    # An OPEN chain is working state even when every round is terminal: the
    # orchestrator is adjudicating between rounds, and the collector once ate
    # an active critique chain mid-conversation because this check was
    # missing. Only an explicitly closed chain is history.
    root = next((r for r in records if r.get("jobId") == chain), None)
    if root is None or root.get("chainClosed") is not True:
        kept.append((chain, "chain not closed; working state")); continue
    manifest_candidates = [evidence / "agents" / chain / "manifest.json"]
    manifest_candidates.extend(sorted((evidence / "agents").glob(f"*/{chain}/manifest.json")))
    mirrored_path = (root.get("mirror") or {}).get("path") if isinstance(root.get("mirror"), dict) else None
    manifest_path = next((path for path in manifest_candidates if path.parent == Path(mirrored_path or "")), None)
    if manifest_path is None:
        manifest_path = next((path for path in manifest_candidates if path.exists()), manifest_candidates[0])
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

# Job records are the registry while work is recent: the staleness check reads
# them for its chain window and the census joins custody through them. Past
# that window, a terminal chain's records serve only history, and history is
# the mirror, which already holds every record file. Locally: live-only.
import os, time
grace = float(os.environ.get("METASYSTEM_CHAIN_GRACE_SECONDS", "5400"))
for record_path in sorted(jobs.glob("*.json")):
    try:
        record = json.loads(record_path.read_text())
    except (OSError, ValueError):
        continue
    if record.get("status") not in TERMINAL:
        continue
    root_chain = re.sub(r"-r[0-9]+$", "", record_path.stem)
    if (agents / root_chain).exists():
        continue  # chain payload not collected yet; records stay with it
    manifest_candidates = [evidence / "agents" / root_chain / "manifest.json"]
    manifest_candidates.extend(sorted((evidence / "agents").glob(f"*/{root_chain}/manifest.json")))
    manifest_path = next((path for path in manifest_candidates if path.exists()), manifest_candidates[0])
    if not manifest_path.exists():
        continue
    manifest_files = json.loads(manifest_path.read_text())["files"]
    entry = manifest_files.get(f"jobs/{record_path.name}")
    if entry is None:
        continue
    mirrored_at = json.loads(manifest_path.read_text()).get("updatedAt", "")
    try:
        from datetime import datetime, timezone
        age = time.time() - datetime.strptime(mirrored_at, "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=timezone.utc).timestamp()
    except ValueError:
        continue
    if age <= grace:
        continue
    try:
        record_path.unlink()
    except FileNotFoundError:
        pass

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
        # Two collectors can run at once (the stop hook fires one per turn
        # end); a peer having removed an entry first is success, not an error.
        try:
            if entry.name.endswith((".lock", ".lifecycle.d")):
                if job_status(job) in TERMINAL:
                    (shutil.rmtree if entry.is_dir() else Path.unlink)(entry); residue += 1
            elif entry.is_file() and time.time() - entry.stat().st_mtime > 3600:
                entry.unlink(); residue += 1
        except FileNotFoundError:
            pass
caps = agents / "capabilities"
if caps.is_dir():
    # Newest means BY CAPTURE, not by filename: the config hash precedes the
    # date in the name, so a lexicographic sort once deleted the current
    # snapshot and kept a stale one, and the next dispatch refused. Sort by
    # the date and sequence components the writer stamps.
    def capture_key(snap):
        parts = snap.stem.rsplit("-", 2)
        return (parts[-2], parts[-1]) if len(parts) == 3 else ("", "")
    # Group by runtime+version+CONFIG HASH, which is what dispatch matches on.
    # Grouping by runtime+version alone treats snapshots for different configs
    # as superseding each other, so a config change silently deleted the very
    # snapshot the next dispatch needed and every dispatch then refused.
    newest = {}
    for snap in caps.glob("*.json"):
        identity = snap.name.rsplit("-", 2)[0]  # runtime-version-confighash
        newest.setdefault(identity, []).append(snap)
    for identity, snaps in newest.items():
        for snap in sorted(snaps, key=capture_key)[:-1]:
            snap.unlink(); residue += 1

# Empty directories are confusion, not placeholders: every writer here
# mkdir-ps before writing, so a directory with nothing in it carries no
# information and comes back the moment it is needed. Bottom-up, so nested
# empties collapse in one pass; the supervision dir is skipped while armed.
# The spine stays even when empty: the census and watcher read these every
# interval, and pruning an empty jobs/ silenced the census entirely (no
# directory, no verdict, no arming). Only per-job ephemera collapse.
SPINE = {"jobs", "capabilities", "mains", "record-locks", "supervision"}
for directory in sorted((p for p in agents.rglob("*") if p.is_dir()), reverse=True):
    if directory.name in SPINE or "supervision" in directory.parts:
        continue
    try:
        directory.rmdir()  # only succeeds when empty
        residue += 1
    except OSError:
        pass

# Flight-recorder archives (plans/flight-recorder.md D-4): copy-then-keep is
# the norm. Delete a local archive ONLY when BOTH hold: a verified durable
# copy exists AND the filename age is >= 14 days. Age comes from the filename
# timestamp, never mtime.
import shutil
from datetime import datetime, timezone
_fr_checkout = Path(sys.argv[1])  # fresh names: the heredoc rebinds root/evidence in loops above
_fr_evidence = Path(sys.argv[2])
archive_dir = _fr_checkout / "artifacts" / "agents" / "events-archive"
events_root = _fr_evidence / "events" / _fr_checkout.name
stamp_re = re.compile(r"^events-(\d{8}T\d{6}Z)")
if archive_dir.is_dir():
    for item in sorted(archive_dir.glob("events-*.jsonl")):
        match = stamp_re.match(item.name)
        if not match:
            continue
        durable = events_root / item.name
        try:
            if not durable.exists() or durable.stat().st_size != item.stat().st_size:
                events_root.mkdir(parents=True, exist_ok=True)
                shutil.copy2(item, durable)
            copied = durable.exists() and durable.stat().st_size == item.stat().st_size
        except OSError:
            copied = False  # keep local, retry next pass
        try:
            age_days = (datetime.now(timezone.utc) - datetime.strptime(
                match.group(1), "%Y%m%dT%H%M%SZ").replace(tzinfo=timezone.utc)).days
        except ValueError:
            continue
        if copied and age_days >= 14:
            item.unlink(missing_ok=True)
            print(f"collected events archive {item.name}")

for chain in collected:
    print(f"collected {chain}")
print(f"residue removed: {residue} heartbeat, lock, temp and superseded-snapshot entries")
for chain, reason in kept:
    print(f"kept      {chain}: {reason}")
print(f"evidence-gc: {len(collected)} collected, {len(kept)} kept")
PY
