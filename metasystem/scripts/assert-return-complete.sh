#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/assert-return-complete.sh --role <role> --file <return.json>
  scripts/assert-return-complete.sh --job <job-id>

Validates a canonical agent return against the shipped schema for its role.
The job form reads artifacts/agents/jobs/<job-id>.json, finds that chain's
round return, and also checks jobId, round, runtime, and sessionId identity.

Exit codes: 0 pass; 1 validation failure; 2 usage.
USAGE
}

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"
role=
file=
job=

while (($#)); do
  case "$1" in
    --role) [[ $# -ge 2 ]] || { usage; exit 2; }; role=$2; shift 2 ;;
    --file) [[ $# -ge 2 ]] || { usage; exit 2; }; file=$2; shift 2 ;;
    --job) [[ $# -ge 2 ]] || { usage; exit 2; }; job=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) usage; exit 2 ;;
  esac
done

if [[ -n "$job" ]]; then
  [[ -z "$role" && -z "$file" && "$job" =~ ^[a-z0-9][a-z0-9-]*$ ]] || { usage; exit 2; }
  mode=job
else
  [[ -n "$role" && -n "$file" ]] || { usage; exit 2; }
  mode=role
fi

case "$role" in
  ""|orchestrator|design-critic|implementer|code-critic|verifier|investigator|behavior-judge) ;;
  *) echo "violation: unknown role: $role" >&2; exit 1 ;;
esac

if [[ "$mode" == job ]]; then
  exec "$ms" validate return-complete --root "$root" --job "$job"
fi
exec "$ms" validate return-complete --root "$root" --role "$role" --file "$file"
