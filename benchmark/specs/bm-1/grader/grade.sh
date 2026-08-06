#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
  echo "usage: $0 <path-to-produced-repository>" >&2
  exit 2
fi

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
exec python3 "$SCRIPT_DIR/grader.py" "$1"
