#!/usr/bin/env bash
# Shim (plans/kill-shell.md Phase A): the design-obligation gate's decisions
# live in the engine; this file relays the historical calling convention.
# Pure-bash path resolution: no external command may appear here. --root
# leads so a trailing caller flag missing its value keeps its own error.
set -euo pipefail
src=${BASH_SOURCE[0]}
[[ "$src" == */* ]] || src=./$src
here=$(cd -- "${src%/*}/.." && pwd -P)
ms="${METASYSTEM_BIN:-$here/bin/metasystem}"
exec "$ms" validate design-obligations --root "$here" "$@"
