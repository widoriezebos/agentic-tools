#!/usr/bin/env bash
# Shim (plans/kill-shell.md Phase A): the audit's decisions live in the
# engine; this file relays the historical calling convention and env knobs.
set -euo pipefail
root=${1:-.}
here=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
ms="${METASYSTEM_BIN:-$here/bin/metasystem}"
args=(--root "$root")
[[ -n "${METASYSTEM_MAX_ALWAYS_LOADED_WORDS:-}" ]] && args+=(--max-always-loaded-words "$METASYSTEM_MAX_ALWAYS_LOADED_WORDS")
[[ -n "${METASYSTEM_AUDIT_ALLOW_PLACEHOLDERS:-}" ]] && args+=(--allow-placeholders)
exec "$ms" audit metasystem "${args[@]}"
