#!/usr/bin/env bash
# benchmark/grade.sh --case <caseId>@<caseVersion> <produced-repository>
#
# Runs the CASE's held-out grader against a produced repository. The grader
# comes from the pinned case version tree (git archive of its object id),
# so it is exactly the grader the case version registered — never a working
# copy. Prints the grader's metric lines on stdout, exactly as the grader does.
set -euo pipefail
kit=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
usage() { echo "usage: benchmark/grade.sh --case <id>@<version> <produced-repository>" >&2; }
case_ref= target=
while (($#)); do
  case "$1" in
    --case) [[ $# -ge 2 ]] || { usage; exit 2; }; case_ref=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) target=$1; shift ;;
  esac
done
[[ -n "$case_ref" && -n "$target" ]] || { usage; exit 2; }
[[ "$case_ref" == *@* ]] || { echo "grade refused: --case must be pinned as <id>@<version>" >&2; exit 2; }
[[ -d "$target" ]] || { echo "grade refused: produced repository does not exist: $target" >&2; exit 2; }
ident=${case_ref%@*}
version=${case_ref#*@}
scratch=$(mktemp -d)
trap 'rm -rf -- "$scratch"' EXIT
tree=$(python3 - "$kit" "$ident" "$version" <<'PY'
import sys
from pathlib import Path
sys.path.insert(0, sys.argv[1])
import pairs
print(pairs.resolve_case(Path(sys.argv[1]), sys.argv[2], sys.argv[3])["tree"])
PY
) || exit 2
kit_top=$(git -C "$kit" rev-parse --show-toplevel)
git -C "$kit_top" archive "$tree" | tar -x -C "$scratch"
grader_rel=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["grader"]["path"].rstrip("/"))' "$scratch/case.json")
grader=$scratch/$grader_rel/grade.sh
[[ -x "$grader" ]] || { echo "grade refused: case $case_ref ships no executable grader at $grader_rel/grade.sh" >&2; exit 2; }

# Stage the produced repository for the grader with non-regular
# files excluded (KI-42): a live mission's ACP transport leaves its
# fifo pair in the round artifacts, and a named pipe cannot be
# copied by the grader's own evidence walk — the whole grading step
# died on it (bm-2dc rep 2, 2026-08-24). The registered grader is
# immutable (versions.lock), so the KIT stages: regular files,
# directories, and symlinks ride; fifos, sockets, and devices are
# skipped WITH A LOGGED NOTE — never silently (the kit's no-silent-
# caps rule). The stage is a copy, so the evidence itself is never
# touched.
stage=$scratch/staged-target
skipped=$(python3 - "$target" "$stage" <<'STAGEPY'
import os, shutil, stat, sys
source, stage = sys.argv[1], sys.argv[2]
skipped = []
for root, dirs, files in os.walk(source):
    rel = os.path.relpath(root, source)
    dest_dir = os.path.join(stage, rel) if rel != "." else stage
    os.makedirs(dest_dir, exist_ok=True)
    for name in list(dirs):
        src = os.path.join(root, name)
        if os.path.islink(src):
            os.symlink(os.readlink(src), os.path.join(dest_dir, name))
            dirs.remove(name)
    for name in files:
        src = os.path.join(root, name)
        dst = os.path.join(dest_dir, name)
        mode = os.lstat(src).st_mode
        if stat.S_ISLNK(mode):
            os.symlink(os.readlink(src), dst)
        elif stat.S_ISREG(mode):
            shutil.copy2(src, dst)
        else:
            skipped.append(os.path.join(rel, name) if rel != "." else name)
for path in skipped:
    print(path)
STAGEPY
) || { echo "grade refused: staging the produced repository failed" >&2; exit 1; }
if [[ -n "$skipped" ]]; then
  count=$(printf '%s\n' "$skipped" | grep -c .)
  echo "grade note: $count non-regular file(s) excluded from the staged evidence (named pipes/sockets/devices):" >&2
  printf '%s\n' "$skipped" | sed 's/^/  /' >&2
fi
exec "$grader" "$stage"
