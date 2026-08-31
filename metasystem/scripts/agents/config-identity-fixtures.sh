#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"
[[ -x "$ms" ]] || { echo "config identity fixtures: binary absent; run the go gate first" >&2; exit 1; }
helper() { "$ms" config identity "$@"; }
selector() { "$ms" job snapshot-select "$@"; }
tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-config-identity.XXXXXX")
trap 'rm -rf "$tmp"' EXIT

config="$tmp/config.toml"
cat >"$config" <<'TOML'
model = "gpt-5.6-sol"

[notice]
hide_rate_limit_model_nudge = false

[notice.model_migrations]
"gpt-5.2" = "gpt-5.3-codex"

[tui.model_availability_nux]
"gpt-5.6-sol" = 1
TOML

# Canonicalization behavior lives in the internal/config Go tests. This file
# keeps one command-line smoke proving identity determinism through the built
# executable.
identity=$(helper --runtime codex --version 0.146.0 "$config")
[[ -n "$identity" ]] || { echo "config identity smoke: empty identity" >&2; exit 1; }
identity_again=$(helper --runtime codex --version 0.146.0 "$config")
[[ "$identity" == "$identity_again" ]] \
  || { echo "config identity smoke: identity not deterministic across calls" >&2; exit 1; }

echo "configuration identity fixtures: PASSED"
