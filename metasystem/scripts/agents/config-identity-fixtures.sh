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

# Behavior parity lives in Go under the gate (script-fixtures-014/D41):
# TestConfigIdentityIgnoresBookkeepingChurn, ...ChangeSensitive,
# ...StableAcrossEquivalentJSON, ...MalformedAndOutOfRangeFilterHashesFullMap
# (internal/config) and TestSelectNoMatchNamesChangedKeys
# (internal/capability). This file keeps one CLI smoke — the flag
# forwarding is the script-side property — and the executable-appendix
# pin over the SHIPPED filter files, which is data, not behavior.
identity=$(helper --runtime codex --version 0.146.0 \
  --filter "$root/scripts/agents/adapters/codex-config-filter.v1.json" <"$config")
[[ -n "$identity" ]] || { echo "config identity smoke: empty identity" >&2; exit 1; }
identity_again=$(helper --runtime codex --version 0.146.0 \
  --filter "$root/scripts/agents/adapters/codex-config-filter.v1.json" <"$config")
[[ "$identity" == "$identity_again" ]] \
  || { echo "config identity smoke: identity not deterministic across calls" >&2; exit 1; }

# The appendix itself is executable policy; pin its exact version-one data.
python3 - "$root/scripts/agents/adapters" <<'PY'
import json, sys
from pathlib import Path
root = Path(sys.argv[1])
codex = json.loads((root / "codex-config-filter.v1.json").read_text())
claude = json.loads((root / "claude-config-filter.v1.json").read_text())
devin = json.loads((root / "devin-config-filter.v1.json").read_text())
assert codex["cliVersionRange"] == {"min": "0.146.0", "max": "0.146.x"}
assert [entry["path"] for entry in codex["keys"]] == [
    "notice", "notice.model_migrations", "tui.model_availability_nux"
]
assert all("KI-19" in entry["source"] for entry in codex["keys"])
assert claude == {"cliVersionRange": {"min": "2.1.0", "max": "2.1.x"}, "keys": []}
assert devin == {
    "cliVersionRange": {"min": "3000.3.27", "max": "3000.3.27"}, "keys": []
}
PY

echo "configuration identity fixtures: PASSED"
