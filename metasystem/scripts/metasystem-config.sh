#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/metasystem-config.sh get --key <key> [--mode <mode>] \
      [--flag <value>] [--default <value>]
  scripts/metasystem-config.sh keys --prefix <prefix>
  scripts/metasystem-config.sh validate

Resolution order for get is an explicitly supplied flag, the mechanically
derived METASYSTEM_ environment variable, the metasystem.conf.local override,
a mode-scoped key, the plain key, then the explicit default. Dots and dashes
in a key become underscores in its upper-cased environment name.

Exit codes: 0 success; 1 missing value or invalid configuration; 2 usage.
USAGE
}

die() { echo "$2" >&2; exit "$1"; }

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"
config="$root/metasystem.conf"

command=${1:-}
[[ -n "$command" ]] || { usage; exit 2; }
shift

case "$command" in
  get)
    exec "$ms" config get --conf "$config" "$@"
    ;;
  keys)
    # Callers name the family by prefix; the verb calls it --matching.
    prefix=
    while (($#)); do
      case "$1" in
        --prefix) [[ $# -ge 2 ]] || { usage; exit 2; }; prefix=$2; shift 2 ;;
        *) usage; exit 2 ;;
      esac
    done
    [[ -n "$prefix" ]] || die 2 "keys requires --prefix"
    exec "$ms" config keys --conf "$config" --matching "$prefix"
    ;;
  validate)
    (($# == 0)) || { usage; exit 2; }
    repo_scope=$(git -C "$root" rev-parse --show-toplevel 2>/dev/null || true)
    [[ -n "$repo_scope" ]] && repo_scope=$(cd "$repo_scope" && pwd -P)
    exec "$ms" config validate --conf "$config" --repo "${repo_scope:-$root}"
    ;;
  -h|--help)
    usage
    ;;
  *)
    usage
    exit 2
    ;;
esac
