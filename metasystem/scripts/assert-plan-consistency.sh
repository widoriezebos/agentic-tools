#!/usr/bin/env bash
set -euo pipefail

# Finds a rule stated in more than one place whose statements disagree.
#
# Eight of the nine rounds of one design critique found nothing else: a rule
# corrected in the paragraph being edited while the same claim survived in a
# summary table, a condition list, a rationale, or another document. A critique
# round is the wrong instrument for that. It costs minutes and real tokens to
# notice that two sentences in one repository contradict each other, and the
# failure is not specific to any agent: anyone editing a design across several
# documents will do it.
#
# The check is deliberately narrow. It does not understand prose. It knows two
# things a plan can state in several places and get wrong in exactly one way:
#
#   1. A metric named in a table row must not also be named as removed,
#      replaced, or superseded elsewhere without that row saying so.
#   2. A term the plans declare retired, through a `RETIRED:` marker, must not
#      still be prescribed anywhere as though it were current.
#
# Everything else is left to readers, because a checker that guesses at meaning
# produces false alarms, and a check people learn to ignore is worse than none.

usage() {
  cat <<'USAGE' >&2
Usage: scripts/assert-plan-consistency.sh [--plans-dir <directory>]

Fails when a plan prescribes a term another plan has retired.

A plan retires a term by writing, anywhere in its text:

  RETIRED: <term> -- <what replaced it>

Every other plan is then checked for that term. A mention is allowed only on a
line that also carries one of: RETIRED, SUPERSEDED, "replaces", "replaced",
"no longer", or "removed", which is how a document explains a change rather
than prescribing the old rule.

Exit codes: 0 consistent; 1 a retired term is still prescribed; 2 usage.
USAGE
}

plans_dir=""
while (($#)); do
  case "$1" in
    --plans-dir) [[ $# -ge 2 ]] || { usage; exit 2; }; plans_dir=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) usage; exit 2 ;;
  esac
done

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"
[[ -n "$plans_dir" ]] || plans_dir="$root/plans"
[[ -d "$plans_dir" ]] || { echo "no such plans directory: $plans_dir" >&2; exit 2; }

exec "$ms" validate plan-consistency --plans-dir "$plans_dir"

