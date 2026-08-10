#!/usr/bin/env bash
# Differential conformance for the census SIGNATURE classifier (the first
# slice of the census port, plans/go-migration.md Phase 1). It proves the Go
# RE2 classifier is indistinguishable from the python reference — grep -E,
# exactly what process-census.py uses — over the REAL adapter patterns and a
# corpus built to stress word boundaries, paths, and the KI-14 exclusions.
#
# This is NOT the seam-1 retirement artifact: seam 1 (the go watcher shelling
# to python census) retires only when the FULL census verb lands
# (scripts/agents/census-go-fixtures.sh). This file proves one slice.
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
bin="$root/bin/metasystem"
[[ -x "$bin" ]] || { echo "census conformance: binary absent; run the go gate first" >&2; exit 1; }

tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-census-conf.XXXXXX")
trap 'rm -rf "$tmp"' EXIT

# The reference classifier: grep -E per pattern, first-runtime-wins,
# exclude-wins — the exact algorithm of process-census.py signature_processes,
# using the same grep engine the python subprocess-calls. This is the oracle.
python3 - "$root" "$bin" <<'PY'
import json, subprocess, sys
from pathlib import Path

root, binary = Path(sys.argv[1]), sys.argv[2]

# The real signature declarations, in adapter order (order is load-bearing).
runtimes = ["claude", "codex", "devin", "fake"]
signatures = []
for rt in runtimes:
    out = subprocess.run([str(root / "scripts/agents/adapters" / f"{rt}.sh"), "signature"],
                         capture_output=True, text=True, check=True).stdout
    matches, excludes = [], []
    for line in out.splitlines():
        kind, _, pattern = line.partition(" ")
        (matches if kind == "match" else excludes).append(pattern)
    signatures.append({"runtime": rt, "matches": matches, "excludes": excludes})

# A corpus that stresses the classifier: real command shapes, path prefixes,
# word-boundary traps, the KI-14 excluded shapes, and near-misses.
argvs = [
    "claude --allow-dangerously-skip-permissions",
    "/usr/local/bin/claude serve",
    "python3 /repo/scripts/agents/adapters/claude-session-signal.py",
    "bash /repo/scripts/agents/supervision-hook.sh claude stop",
    "bash scripts/agents/adapters/claude.sh probe",
    "echo declaudetest",                 # 'claude' mid-word: no match
    "codex exec --json -m gpt-5.6-sol",
    "/opt/codex serve",
    "bash scripts/agents/adapters/codex.sh dispatch",
    "devin --version",
    "grep -rn devin /repo",              # 'devin' as an argument word
    "metasystem-fake-agent first",
    "/tool/metasystem-fake-agent second",
    "bash scripts/agents/adapters/fake.sh signature",
    "a-metasystem-fake-agent-lookalike run",
    "vim notes-about-claude-and-codex.md",
    "",                                  # empty argv
    "claude",                            # bare word, end-of-string boundary
]

# The Go classification.
job = {"signatures": signatures, "argvs": argvs}
go_out = subprocess.run([binary, "census", "classify"], input=json.dumps(job),
                        capture_output=True, text=True, check=True).stdout
go_assign = {a["index"]: a["runtime"] for a in json.loads(go_out)["assignments"]}

# The reference classification via grep -E, matching signature_processes:
# per runtime in order, an argv is assigned if some match hits and no exclude
# hits, first runtime wins (setdefault).
def grep_hits(pattern, lines):
    proc = subprocess.run(["grep", "-E", "-n", "--", pattern],
                          input="\n".join(lines) + "\n", capture_output=True, text=True)
    if proc.returncode == 1:
        return set()
    if proc.returncode != 0:
        raise SystemExit(f"reference grep failed on {pattern!r}: {proc.stderr}")
    hits = set()
    for raw in proc.stdout.splitlines():
        n, sep, _ = raw.partition(":")
        if sep and n.isdigit():
            hits.add(int(n) - 1)
    return hits

ref_assign = {}
for sig in signatures:
    matched, excluded = set(), set()
    for p in sig["excludes"]:
        excluded |= grep_hits(p, argvs)
    for p in sig["matches"]:
        matched |= grep_hits(p, argvs)
    for i in matched - excluded:
        ref_assign.setdefault(i, sig["runtime"])

if go_assign != ref_assign:
    print("census signature conformance FAILED: Go and grep -E disagree", file=sys.stderr)
    for i, argv in enumerate(argvs):
        g, r = go_assign.get(i), ref_assign.get(i)
        if g != r:
            print(f"  argv[{i}]={argv!r}: go={g!r} reference(grep -E)={r!r}", file=sys.stderr)
    raise SystemExit(1)

print(f"census signature conformance: {len(argvs)} argvs, Go RE2 == grep -E on the real patterns", file=sys.stderr)
PY

echo "census signature conformance: PASSED"
