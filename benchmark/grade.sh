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
exec "$grader" "$target"
