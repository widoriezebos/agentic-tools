#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage: scripts/agents/check-preamble-quotes.sh [--roles-dir <directory>]

Checks every Markdown role preamble in the directory. A verbatim quote uses
the convention below, defined here once for every preamble:

  <!-- quote source="path/from/metasystem/root" -->
  byte-exact content copied from that source
  <!-- /quote -->

The content bytes, including Markdown punctuation and internal line endings,
must occur unchanged and contiguously in the named source document. The final
line ending before the closing marker is marker framing, not quote content.

Exit codes: 0 pass; 1 drift or malformed quote; 2 usage.
USAGE
}

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"
roles_dir="$root/scripts/agents/roles"

while (($#)); do
  case "$1" in
    --roles-dir) [[ $# -ge 2 ]] || { usage; exit 2; }; roles_dir=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) usage; exit 2 ;;
  esac
done

exec "$ms" validate preamble-quotes --root "$root" --roles-dir "$roles_dir"
