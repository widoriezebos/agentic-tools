#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/agents/mission-runner.sh start --mission <id> [--foreground]
  scripts/agents/mission-runner.sh resume --mission <id> [--foreground]
  scripts/agents/mission-runner.sh status --mission <id>
  scripts/agents/mission-runner.sh answer --mission <id> --ask <ask-id> --answer <text>
USAGE
}

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"

case "${1:-}" in
  -h|--help) usage; exit 0 ;;
  "") usage; exit 2 ;;
esac

verb=$1
shift
exec "$ms" mission "$verb" --root "$root" "$@"
