#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage: scripts/assert-turn-prompt.sh --file <prompt> --turn <turn-directory>

Validates an assembled unattended host-turn prompt against its canonical turn
record and the shipped orchestrator preamble.

Exit codes: 0 pass; 1 validation failure; 2 usage.
USAGE
}

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"
file=
turn=

while (($#)); do
  case "$1" in
    --file) [[ $# -ge 2 ]] || { usage; exit 2; }; file=$2; shift 2 ;;
    --turn) [[ $# -ge 2 ]] || { usage; exit 2; }; turn=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) usage; exit 2 ;;
  esac
done

[[ -n "$file" && -n "$turn" ]] || { usage; exit 2; }

exec "$ms" validate turn-prompt --root "$root" --file "$file" --turn "$turn"
