#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "usage: $0 --source <path> --artifact <path> [--jdwp-port <port>]" >&2
}

source_path=
artifact_path=
jdwp_port=
while (($#)); do
  case "$1" in
    --source) source_path=${2:-}; shift 2 ;;
    --artifact) artifact_path=${2:-}; shift 2 ;;
    --jdwp-port) jdwp_port=${2:-}; shift 2 ;;
    *) usage; exit 2 ;;
  esac
done

[[ -n "$source_path" && -n "$artifact_path" ]] || { usage; exit 2; }
[[ -e "$source_path" ]] || { echo "missing source: $source_path" >&2; exit 1; }
[[ -e "$artifact_path" ]] || { echo "missing artifact: $artifact_path" >&2; exit 1; }
[[ ! "$source_path" -nt "$artifact_path" ]] || { echo "stale artifact: source is newer" >&2; exit 1; }

if [[ -n "$jdwp_port" ]]; then
  command -v lsof >/dev/null || { echo "lsof is required for --jdwp-port" >&2; exit 1; }
  lsof -nP -iTCP:"$jdwp_port" -sTCP:LISTEN >/dev/null || { echo "no listener on JDWP port $jdwp_port" >&2; exit 1; }
fi

echo "debug preflight passed"
