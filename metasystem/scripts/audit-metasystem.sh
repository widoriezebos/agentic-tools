#!/usr/bin/env bash
# Shim (records/kill-shell/kill-shell.md Phase A): the audit's decisions live in the
# engine; this file relays the historical calling convention and env knobs.
# Pure-bash path resolution: the suite proves this runs under a minimal
# PATH, so no external command may appear here.
set -euo pipefail
root=${1:-.}
src=${BASH_SOURCE[0]}
[[ "$src" == */* ]] || src=./$src
here=$(cd -- "${src%/*}/.." && pwd -P)
ms="${METASYSTEM_BIN:-$here/bin/metasystem}"
args=(--root "$root")
[[ -n "${METASYSTEM_MAX_ALWAYS_LOADED_WORDS:-}" ]] && args+=(--max-always-loaded-words "$METASYSTEM_MAX_ALWAYS_LOADED_WORDS")
[[ -n "${METASYSTEM_AUDIT_ALLOW_PLACEHOLDERS:-}" ]] && args+=(--allow-placeholders)
exec "$ms" audit metasystem "${args[@]}"
