#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage: scripts/agents/assert-conformance.sh --stage review|merge --job <job-id>

The review stage computes the implementer worktree's exact review object. A
temporary index contains every tracked file plus every untracked, unignored
file; ignored files are excluded. It writes diff.patch and review.json without
changing the worktree's real index. The merge stage repeats conformance checks
and requires either a mechanically valid waiver or a closed, independent
code-critic chain over the branch's final committed tree.

Exit codes: 0 conforming; 1 conformance failure; 2 usage.
USAGE
}

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
config="$root/scripts/metasystem-config.sh"
stage=
job=
while (($#)); do
  case "$1" in
    --stage) [[ $# -ge 2 && -z "$stage" ]] || { usage; exit 2; }; stage=$2; shift 2 ;;
    --job) [[ $# -ge 2 && -z "$job" ]] || { usage; exit 2; }; job=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) usage; exit 2 ;;
  esac
done
[[ "$stage" == review || "$stage" == merge ]] || { usage; exit 2; }
[[ "$job" =~ ^[a-z0-9][a-z0-9-]*$ ]] || { usage; exit 2; }
record="$root/artifacts/agents/jobs/$job.json"
[[ -f "$record" ]] || { echo "conformance failure: unknown job: $job" >&2; exit 1; }

facts=$(python3 - "$root" "$record" <<'PY'
import json, sys
from pathlib import Path

root, record_path = Path(sys.argv[1]), Path(sys.argv[2])
try:
    record = json.loads(record_path.read_text(encoding="utf-8"))
except (OSError, ValueError) as error:
    raise SystemExit(f"malformed job record: {error}")
if record.get("role") != "implementer":
    raise SystemExit("conformance review is only defined for implementer records")
job = record.get("jobId")
parent = record.get("parentJob")
root_job = job
seen = set()
while parent is not None:
    if not isinstance(parent, str) or parent in seen:
        raise SystemExit("parent chain is malformed or contains a cycle")
    seen.add(parent)
    parent_path = root / "artifacts" / "agents" / "jobs" / f"{parent}.json"
    try:
        value = json.loads(parent_path.read_text(encoding="utf-8"))
    except (OSError, ValueError) as error:
        raise SystemExit(f"parent job record {parent!r} is unreadable: {error}")
    root_job = parent
    parent = value.get("parentJob")
print(record["workspaceRoot"])
print(record["baseSha"])
print(record["round"])
print(root_job)
PY
) || { echo "conformance failure: could not resolve implementer job facts" >&2; exit 1; }
workspace=$(printf '%s\n' "$facts" | sed -n '1p')
base_sha=$(printf '%s\n' "$facts" | sed -n '2p')
round=$(printf '%s\n' "$facts" | sed -n '3p')
root_job=$(printf '%s\n' "$facts" | sed -n '4p')
round_dir="$root/artifacts/agents/$root_job/rounds/$round"
return_file="$round_dir/return.json"
diff_file="$round_dir/diff.patch"
review_file="$round_dir/review.json"
[[ -d "$round_dir" && -f "$return_file" ]] \
  || { echo "conformance failure: implementer round return is missing" >&2; exit 1; }
git -C "$workspace" cat-file -e "$base_sha^{commit}" 2>/dev/null \
  || { echo "conformance failure: baseSha is not a commit in the implementer workspace" >&2; exit 1; }

# This is the canonical worktree-content helper. A full add in an isolated
# index includes tracked files and untracked files that are not ignored. Git's
# normal ignore rules leave ignored untracked files out. No real staging choice
# or worktree file is changed.
snapshot_dir=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-conformance.XXXXXX")
trap 'rm -rf "$snapshot_dir"' EXIT
index="$snapshot_dir/index"
paths_file="$snapshot_dir/paths"
numstat_file="$snapshot_dir/numstat"
GIT_INDEX_FILE="$index" git -C "$workspace" read-tree HEAD
GIT_INDEX_FILE="$index" git -C "$workspace" add -A -- .
reviewed_tree=$(GIT_INDEX_FILE="$index" git -C "$workspace" write-tree)
GIT_INDEX_FILE="$index" git -C "$workspace" diff --cached --binary --no-renames "$base_sha" -- >"$diff_file"
GIT_INDEX_FILE="$index" git -C "$workspace" diff --cached --name-only -z --no-renames "$base_sha" -- >"$paths_file"
GIT_INDEX_FILE="$index" git -C "$workspace" diff --cached --numstat --no-renames "$base_sha" -- >"$numstat_file"

install_prefix=$(git -C "$root" rev-parse --show-prefix 2>/dev/null || true)
install_prefix=${install_prefix%/}

python3 - "$workspace" "$return_file" "$paths_file" "$install_prefix" <<'PY'
import json, os, sys
from pathlib import Path

workspace, return_file, paths_file = map(Path, sys.argv[1:4])
install_prefix = sys.argv[4].replace("\\", "/").strip("/")
paths = [os.fsdecode(item) for item in paths_file.read_bytes().split(b"\0") if item]
violations = []


def installation_path(path):
    normalized = path.replace("\\", "/").strip("/")
    if install_prefix and normalized.startswith(install_prefix + "/"):
        return normalized[len(install_prefix) + 1 :]
    return normalized


for path in paths:
    normalized = installation_path(path)
    if normalized == "plans" or normalized.startswith("plans/"):
        violations.append(f"trusted plans/ state changed: {path}")
    if normalized == "artifacts/agents" or normalized.startswith("artifacts/agents/"):
        violations.append(f"agent control plane changed: {path}")

# The control plane is normally ignored and therefore absent from the index.
# Inspect the corresponding installation path in the delegate worktree too.
control = workspace / install_prefix / "artifacts" / "agents"
if control.exists() and any(path.is_file() for path in control.rglob("*")):
    violations.append("agent control plane contains delegate-created files")

try:
    result = json.loads(return_file.read_text(encoding="utf-8"))
except (OSError, ValueError) as error:
    violations.append(f"return.json is unreadable: {error}")
    result = {}
claim = result.get("diffBoundary")
if not isinstance(claim, list) or not all(isinstance(item, str) for item in claim):
    violations.append("return diffBoundary is not an array of paths")
elif sorted(set(claim)) != sorted(set(paths)):
    violations.append(
        "Diff-boundary claim does not match computed diff: "
        f"claimed={sorted(set(claim))!r} computed={sorted(set(paths))!r}"
    )
for violation in violations:
    print(f"conformance failure: {violation}", file=sys.stderr)
raise SystemExit(1 if violations else 0)
PY

if [[ "$stage" == review ]]; then
  python3 - "$review_file" "$job" "$reviewed_tree" "$(basename "$diff_file")" <<'PY'
import json, sys
from pathlib import Path

path = Path(sys.argv[1])
value = {
    "diffArtifact": sys.argv[4],
    "implementerJob": sys.argv[2],
    "reviewedTree": sys.argv[3],
}
path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
  printf 'reviewedTree=%s\ndiffArtifact=%s\n' "$reviewed_tree" "$diff_file"
  exit 0
fi

final_tree=$(git -C "$workspace" rev-parse 'HEAD^{tree}' 2>/dev/null) \
  || { echo "conformance failure: implementer branch has no final committed tree" >&2; exit 1; }
code_critic_runtime=$("$config" get --key role.code-critic.runtime --default __missing__ 2>/dev/null || printf '__missing__\n')
independence=$("$config" get --key independence --default '' 2>/dev/null || true)

python3 - "$root" "$record" "$paths_file" "$numstat_file" "$install_prefix" \
  "$final_tree" "$code_critic_runtime" "$independence" "$root_job" <<'PY'
import json
import os
import re
import sys
from pathlib import Path

(
    root,
    record_path,
    paths_path,
    numstat_path,
) = map(Path, sys.argv[1:5])
install_prefix, final_tree, configured_runtime, independence, implementer_root = sys.argv[5:]
jobs_dir = root / "artifacts" / "agents" / "jobs"
job_id_pattern = re.compile(r"^[a-z0-9][a-z0-9-]*$")

# One canonical list owns the paths that can steer agent behavior. Directory
# entries end in a slash and match their whole subtree.
INSTRUCTION_BEARING_PATHS = (
    "AGENTS.md",
    "wow.md",
    "CLAUDE.md",
    "docs/project-rules.md",
    "skills/",
    "scripts/",
    "schemas/",
    "templates/",
    "roles/",
)


def load(path, description):
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError) as error:
        raise ValueError(f"{description} is unreadable: {error}") from error
    if not isinstance(value, dict):
        raise ValueError(f"{description} is not a JSON object")
    return value


try:
    implementer = load(record_path, "implementer job record")
except ValueError as error:
    print(f"conformance failure: {error}", file=sys.stderr)
    raise SystemExit(1)


def installation_path(path):
    normalized = path.replace("\\", "/").strip("/")
    prefix = install_prefix.replace("\\", "/").strip("/")
    if prefix and normalized.startswith(prefix + "/"):
        return normalized[len(prefix) + 1 :]
    return normalized


def is_instruction_bearing(path):
    normalized = installation_path(path)
    for entry in INSTRUCTION_BEARING_PATHS:
        if entry.endswith("/"):
            directory = entry[:-1]
            if normalized == directory or normalized.startswith(entry):
                return True
        elif normalized == entry:
            return True
    return False


paths = [os.fsdecode(item) for item in paths_path.read_bytes().split(b"\0") if item]
waiver = implementer.get("critiqueWaived")
if waiver is not None:
    violations = []
    if not isinstance(waiver, dict) or set(waiver) != {"class"}:
        violations.append("critiqueWaived must be an object containing exactly the claimed class")
        waiver_class = None
    else:
        waiver_class = waiver.get("class")
    if waiver_class != "prose-under-30":
        violations.append(
            f"unsupported critique waiver class {waiver_class!r}; the only class is prose-under-30"
        )
    non_markdown = [path for path in paths if not installation_path(path).endswith(".md")]
    instruction_paths = [path for path in paths if is_instruction_bearing(path)]
    if non_markdown:
        violations.append(f"prose-under-30 includes non-Markdown paths: {non_markdown!r}")
    if instruction_paths:
        violations.append(
            "prose-under-30 touches instruction-bearing paths that are never waivable: "
            f"{instruction_paths!r}"
        )
    changed_lines = 0
    binary = False
    for line in numstat_path.read_text(encoding="utf-8").splitlines():
        fields = line.split("\t", 2)
        if len(fields) < 2 or not fields[0].isdigit() or not fields[1].isdigit():
            binary = True
            continue
        changed_lines += int(fields[0]) + int(fields[1])
    if binary:
        violations.append("prose-under-30 contains a binary or uncountable diff")
    if changed_lines > 30:
        violations.append(
            f"prose-under-30 changes {changed_lines} lines; the maximum is 30 additions plus deletions"
        )
    if violations:
        for violation in violations:
            print(f"conformance failure: critique waiver mismatch: {violation}", file=sys.stderr)
        raise SystemExit(1)

    def stream_for(root_job):
        brief = root / "artifacts" / "agents" / root_job / "brief.md"
        try:
            for line in brief.read_text(encoding="utf-8").splitlines():
                if line.startswith("Mission Stream:"):
                    return line.split(":", 1)[1].strip() or "standalone"
        except OSError:
            pass
        return "standalone"

    current_stream = stream_for(implementer_root)
    count = 0
    for candidate_path in jobs_dir.glob("*.json"):
        try:
            candidate = load(candidate_path, "job record")
        except ValueError:
            continue
        if candidate.get("role") != "implementer" or candidate.get("parentJob") is not None:
            continue
        claim = candidate.get("critiqueWaived")
        if isinstance(claim, dict) and claim.get("class") == "prose-under-30":
            if stream_for(candidate.get("jobId", "")) == current_stream:
                count += 1
    print(
        "critique waiver accepted and counted: "
        f"class=prose-under-30 stream={current_stream!r} count={count} changedLines={changed_lines}"
    )
    raise SystemExit(0)


records = {}
record_errors = []
for path in jobs_dir.glob("*.json"):
    try:
        value = load(path, f"job record {path.stem!r}")
    except ValueError as error:
        record_errors.append(str(error))
        continue
    if value.get("jobId") == path.stem:
        records[path.stem] = value


def chain_root(job_id):
    seen = set()
    current = job_id
    while True:
        if current in seen or current not in records:
            return None
        seen.add(current)
        parent = records[current].get("parentJob")
        if parent is None:
            return current
        if not isinstance(parent, str):
            return None
        current = parent


implementation_ids = {
    job_id for job_id in records if chain_root(job_id) == implementer_root
}
critic_roots = [
    value
    for value in records.values()
    if value.get("role") == "code-critic"
    and value.get("parentJob") is None
    and value.get("reviews") in implementation_ids
]

if not critic_roots:
    print(
        "conformance failure: merge requires a code-critic chain whose reviews field names "
        f"implementer job {implementer.get('jobId')!r}; dispatch that role with "
        f"--reviews {implementer.get('jobId')}",
        file=sys.stderr,
    )
    if configured_runtime == "__missing__":
        print(
            "conformance failure: the code-critic role is unconfigured; set the exact key "
            "role.code-critic.runtime",
            file=sys.stderr,
        )
    raise SystemExit(1)


def successor_text(successor_job):
    record = records.get(successor_job)
    if record is None:
        return None
    root_job = chain_root(successor_job)
    round_number = record.get("round")
    if root_job is None or not isinstance(round_number, int):
        return None
    candidates = [
        root / "artifacts" / "agents" / root_job / "brief.md",
        root / "artifacts" / "agents" / root_job / "rounds" / str(round_number) / "prompt.md",
    ]
    parts = []
    for path in candidates:
        try:
            parts.append(path.read_text(encoding="utf-8"))
        except OSError:
            continue
    return "\n".join(parts) if parts else None


def enumerates(text, finding_id):
    if text is None:
        return False
    pattern = rf"(?<![A-Za-z0-9_-]){re.escape(finding_id)}(?![A-Za-z0-9_-])"
    return re.search(pattern, text) is not None


candidate_diagnostics = []
for critic_root in critic_roots:
    critic_id = critic_root["jobId"]
    failures = []
    members = [
        value for job_id, value in records.items() if chain_root(job_id) == critic_id
    ]
    if not members:
        failures.append("has no readable rounds")
        candidate_diagnostics.append((critic_id, failures))
        continue
    final = max(members, key=lambda value: value.get("round", -1))
    final_round = final.get("round")
    if critic_root.get("chainClosed") is not True:
        failures.append("is not closed")
    if final.get("status") != "completed":
        failures.append(f"final round status is {final.get('status')!r}, not completed")
    return_path = root / "artifacts" / "agents" / critic_id / "rounds" / str(final_round) / "return.json"
    try:
        result = load(return_path, f"code-critic chain {critic_id!r} final return")
    except ValueError as error:
        failures.append(str(error))
        result = {}

    findings = result.get("findings")
    material_ids = []
    if isinstance(findings, list):
        material_ids = [
            item.get("id", "<unnamed>")
            for item in findings
            if isinstance(item, dict) and item.get("material") is True
        ]
    else:
        failures.append("final return has no findings array")
    verdict = result.get("verdictMaterialCount")
    if material_ids or verdict != 0:
        failures.append(
            "final round still has material findings despite any dispositions: "
            + (", ".join(material_ids) if material_ids else f"reported count {verdict!r}")
        )

    exhaustions = critic_root.get("critiqueExhaustions", [])
    if not isinstance(exhaustions, list):
        failures.append("critiqueExhaustions is not an array")
        exhaustions = []
    if len(exhaustions) > 1:
        failures.append(
            "a second critique exhaustion is refused outright; waiting on the human is the only remedy"
        )
    valid_exhaustions = []
    for index, exhaustion in enumerate(exhaustions):
        expected = {"round", "openFindingIds", "successorJobId"}
        if not isinstance(exhaustion, dict) or set(exhaustion) != expected:
            failures.append(
                f"critiqueExhaustions[{index}] must contain exactly round, openFindingIds, and successorJobId"
            )
            continue
        exhausted_round = exhaustion.get("round")
        open_ids = exhaustion.get("openFindingIds")
        successor = exhaustion.get("successorJobId")
        if not isinstance(exhausted_round, int) or isinstance(exhausted_round, bool) or exhausted_round < 1:
            failures.append(f"critiqueExhaustions[{index}].round must be a positive round number")
        if (
            not isinstance(open_ids, list)
            or not open_ids
            or not all(isinstance(item, str) and item for item in open_ids)
            or len(open_ids) != len(set(open_ids))
        ):
            failures.append(
                f"critiqueExhaustions[{index}].openFindingIds must be a nonempty list of unique finding identifiers"
            )
            continue
        if not isinstance(successor, str) or not job_id_pattern.fullmatch(successor):
            failures.append(f"critiqueExhaustions[{index}].successorJobId is not a valid job identifier")
            continue
        text = successor_text(successor)
        missing = [finding_id for finding_id in open_ids if not enumerates(text, finding_id)]
        if missing:
            failures.append(
                f"successor job {successor!r} brief does not enumerate open findings: {', '.join(missing)}"
            )
        if not isinstance(final_round, int) or final_round <= exhausted_round or material_ids or verdict != 0:
            failures.append(
                f"critique exhausted at round {exhausted_round} with open material findings: {', '.join(open_ids)}"
            )
        valid_exhaustions.append((exhausted_round, open_ids))
    if (
        len(valid_exhaustions) == 1
        and isinstance(final_round, int)
        and final_round >= valid_exhaustions[0][0] + 3
        and (material_ids or verdict not in (0, None))
    ):
        failures.append(
            "a second critique exhaustion is refused outright; waiting on the human is the only remedy"
        )

    reviewed_tree = result.get("reviewedTree")
    if reviewed_tree != final_tree:
        failures.append(
            f"reviewed tree {reviewed_tree!r} is stale; the implementer branch final committed tree is {final_tree!r}"
        )

    implementer_model = implementer.get("effectiveModel")
    critic_model = final.get("effectiveModel")
    if not isinstance(implementer_model, str) or not implementer_model:
        failures.append(f"implementer job {implementer.get('jobId')!r} has no effective model")
    if not isinstance(critic_model, str) or not critic_model:
        failures.append(f"code-critic chain {critic_id!r} final round has no effective model")
    if (
        isinstance(implementer_model, str)
        and implementer_model
        and implementer_model == critic_model
        and independence != "session-only"
    ):
        failures.append(
            f"independence refused: implementer job {implementer.get('jobId')!r} uses effective model "
            f"{implementer_model!r}, and code-critic chain {critic_id!r} uses effective model "
            f"{critic_model!r}; remedy one is to dispatch a critic on a different model; remedy two is "
            "to declare independence=session-only in configuration"
        )

    if not failures:
        print(
            f"merge critique accepted: implementer job {implementer.get('jobId')} and "
            f"code-critic chain {critic_id} agree on tree {final_tree}"
        )
        raise SystemExit(0)
    candidate_diagnostics.append((critic_id, failures))

for critic_id, failures in candidate_diagnostics:
    for failure in failures:
        print(f"conformance failure: code-critic chain {critic_id}: {failure}", file=sys.stderr)
raise SystemExit(1)
PY
