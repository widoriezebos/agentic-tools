#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/assert-critique-closed.sh --findings <return.json> --dispositions <file>

Joins the canonical findings array from a critic return JSON against the
Markdown dispositions table on finding id.

Required dispositions table header:
| Finding id | Disposition | Reasoning and evidence | Amendment |

Exit codes: 0 closed; 1 open or unjoinable; 2 usage.
USAGE
}

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"
findings=
dispositions=

while (($#)); do
  case "$1" in
    --findings)
      [[ $# -ge 2 && -z "$findings" ]] || { usage; exit 2; }
      findings=$2
      shift 2
      ;;
    --dispositions)
      [[ $# -ge 2 && -z "$dispositions" ]] || { usage; exit 2; }
      dispositions=$2
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      usage
      exit 2
      ;;
  esac
done

[[ -n "$findings" && -n "$dispositions" ]] || { usage; exit 2; }

exec "$ms" validate critique-closed --findings "$findings" --dispositions "$dispositions"
