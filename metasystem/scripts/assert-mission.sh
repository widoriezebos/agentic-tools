#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/assert-mission.sh --file <mission.contract.md>
  scripts/assert-mission.sh --seal --file <mission.contract.md>
  scripts/assert-mission.sh --preflight --file <mission.contract.md>

Default mode validates authored structure and value domains. --seal runs the
gate once with instruments restored from gate.ref, appends the generated seal,
and prints the contract hash. The human adds the approval line after sealing.
--preflight verifies the sealed and approved bytes, origin provenance, frozen
gate runnability, exposure freshness, supervision, census, and lease marker.
USAGE
}

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"
mode=validate
file=

while (($#)); do
  case "$1" in
    --file) [[ $# -ge 2 ]] || { usage; exit 2; }; file=$2; shift 2 ;;
    --seal) [[ "$mode" == validate ]] || { usage; exit 2; }; mode=seal; shift ;;
    --preflight) [[ "$mode" == validate ]] || { usage; exit 2; }; mode=preflight; shift ;;
    -h|--help) usage; exit 0 ;;
    *) usage; exit 2 ;;
  esac
done

[[ -n "$file" && -f "$file" ]] || { echo "missing --file mission contract" >&2; exit 2; }
exec "$ms" mission-contract "$mode" --file "$file"
