#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage: scripts/agents/assert-conformance.sh --job <job-id>

Computes the delegate worktree's base-to-working-tree diff after adding
untracked paths as intent-to-add in an isolated index. Writes diff.patch into
the job's immutable round directory, rejects plans/ or agent-control-plane
changes, and checks the implementer return's diffBoundary claim against the
computed paths.

Exit codes: 0 conforming; 1 conformance failure; 2 usage.
USAGE
}

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
job=
[[ ${1:-} == --job && $# -eq 2 ]] || { usage; exit 2; }
job=$2
[[ "$job" =~ ^[a-z0-9][a-z0-9-]*$ ]] || { usage; exit 2; }
record="$root/artifacts/agents/jobs/$job.json"
[[ -f "$record" ]] || { echo "conformance failure: unknown job: $job" >&2; exit 1; }

facts=$(python3 - "$root" "$record" <<'PY'
import json, sys
from pathlib import Path
root, record_path = Path(sys.argv[1]), Path(sys.argv[2])
try: record = json.loads(record_path.read_text(encoding="utf-8"))
except (OSError, ValueError) as error: raise SystemExit(f"malformed job record: {error}")
if record.get("role") != "implementer": raise SystemExit("conformance review is only defined for implementer records")
job = record.get("jobId"); parent = record.get("parentJob"); root_job = job; seen = set()
while parent is not None:
    if parent in seen: raise SystemExit("parent chain contains a cycle")
    seen.add(parent)
    value = json.loads((root / "artifacts" / "agents" / "jobs" / f"{parent}.json").read_text())
    root_job = parent; parent = value.get("parentJob")
print(record["workspaceRoot"])
print(record["baseSha"])
print(record["round"])
print(root_job)
PY
) || { echo "conformance failure: could not resolve job facts" >&2; exit 1; }
workspace=$(printf '%s\n' "$facts" | sed -n '1p')
base_sha=$(printf '%s\n' "$facts" | sed -n '2p')
round=$(printf '%s\n' "$facts" | sed -n '3p')
root_job=$(printf '%s\n' "$facts" | sed -n '4p')
round_dir="$root/artifacts/agents/$root_job/rounds/$round"
return_file="$round_dir/return.json"
diff_file="$round_dir/diff.patch"
[[ -d "$round_dir" && -f "$return_file" ]] || { echo "conformance failure: round return is missing" >&2; exit 1; }
git -C "$workspace" cat-file -e "$base_sha^{commit}" 2>/dev/null \
  || { echo "conformance failure: baseSha is not a commit in the job workspace" >&2; exit 1; }

# An isolated index gives intent-to-add visibility without changing the
# delegate's real staging choices. The resulting git diff includes committed
# checkpoints, tracked edits, deletions, and previously untracked files.
index=$(mktemp "${TMPDIR:-/tmp}/harness-conformance-index.XXXXXX")
trap 'rm -f "$index"' EXIT
GIT_INDEX_FILE="$index" git -C "$workspace" read-tree HEAD
GIT_INDEX_FILE="$index" git -C "$workspace" add --intent-to-add -A -- .
GIT_INDEX_FILE="$index" git -C "$workspace" diff --binary "$base_sha" -- >"$diff_file"
paths_file=$(mktemp "${TMPDIR:-/tmp}/harness-conformance-paths.XXXXXX")
GIT_INDEX_FILE="$index" git -C "$workspace" diff --name-only -z "$base_sha" -- >"$paths_file"

python3 - "$workspace" "$return_file" "$paths_file" <<'PY'
import json, os, sys
from pathlib import Path
workspace, return_file, paths_file = Path(sys.argv[1]), Path(sys.argv[2]), Path(sys.argv[3])
paths = [os.fsdecode(item) for item in paths_file.read_bytes().split(b"\0") if item]
violations = []
for path in paths:
    normalized = path.replace("\\", "/")
    if normalized == "plans" or normalized.startswith("plans/"):
        violations.append(f"trusted plans/ state changed: {path}")
    if normalized == "artifacts/agents" or normalized.startswith("artifacts/agents/"):
        violations.append(f"agent control plane changed: {path}")

# The control plane is normally gitignored, so inspect it directly as well.
control = workspace / "artifacts" / "agents"
if control.exists() and any(path.is_file() for path in control.rglob("*")):
    violations.append("agent control plane contains delegate-created files")

try: result = json.loads(return_file.read_text(encoding="utf-8"))
except (OSError, ValueError) as error:
    violations.append(f"return.json is unreadable: {error}")
    result = {}
claim = result.get("diffBoundary")
if not isinstance(claim, list) or not all(isinstance(item, str) for item in claim):
    violations.append("return diffBoundary is not an array of paths")
elif sorted(set(claim)) != sorted(set(paths)):
    violations.append(f"Diff-boundary claim does not match computed diff: claimed={sorted(set(claim))!r} computed={sorted(set(paths))!r}")
for violation in violations: print(f"conformance failure: {violation}", file=sys.stderr)
raise SystemExit(1 if violations else 0)
PY
