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
# The filter path comes from the REGISTRY (agnosticism code critique
# finding 1): prove the declared file is the file live identity hashes.
codex_filter=$("$root/bin/metasystem" runtime config-identity-filter codex)
[[ "$codex_filter" == "codex-config-filter.v1.json" ]] \
  || { echo "declared codex filter drifted: $codex_filter" >&2; exit 1; }
cmp -s "$root/scripts/agents/adapters/$codex_filter" "$root/scripts/agents/adapters/codex-config-filter.v1.json" \
  || { echo "declared filter bytes differ from the shipped file" >&2; exit 1; }
identity=$(helper --runtime codex --version 0.146.0 \
  --filter "$root/scripts/agents/adapters/$codex_filter" <"$config")
[[ -n "$identity" ]] || { echo "config identity smoke: empty identity" >&2; exit 1; }
identity_again=$(helper --runtime codex --version 0.146.0 \
  --filter "$root/scripts/agents/adapters/$codex_filter" <"$config")
[[ "$identity" == "$identity_again" ]] \
  || { echo "config identity smoke: identity not deterministic across calls" >&2; exit 1; }

# Print one top-level element per line from the engine's compact rendering
# of a JSON array (or one "key":value member per line from an object). The
# walk is depth- and string-aware, so elements may nest objects and arrays
# — the flat-object splitter in supervision-fixtures.sh cannot. Compact
# rendering escapes control characters, so no element carries a newline.
json_elements() { # compact JSON array or object
  printf '%s' "$1" | awk '
    {
      n = length($0)
      first = substr($0, 1, 1)
      if (n < 2 || (first != "[" && first != "{")) exit 1
      depth = 0; instring = 0; escaped = 0; start = 2
      for (i = 2; i < n; i++) {
        ch = substr($0, i, 1)
        if (instring) {
          if (escaped) escaped = 0
          else if (ch == "\\") escaped = 1
          else if (ch == "\"") instring = 0
        } else if (ch == "\"") instring = 1
        else if (ch == "{" || ch == "[") depth++
        else if (ch == "}" || ch == "]") depth--
        else if (ch == "," && depth == 0) {
          print substr($0, start, i - start)
          start = i + 1
        }
      }
      if (n > 2) print substr($0, start, n - start)
    }'
}

# Canonical (engine-rendered) compact form of a JSON literal, so equality
# checks compare one encoder's bytes against the same encoder's bytes.
canonical_json() { # JSON text
  "$ms" json get --value "{\"root\":$1}" --field root
}

# The appendix itself is executable policy; pin its exact version-one data.
adapters="$root/scripts/agents/adapters"
codex_range=$("$ms" json get --file "$adapters/codex-config-filter.v1.json" --field cliVersionRange)
[[ "$codex_range" == "$(canonical_json '{"min": "0.146.0", "max": "0.146.x"}')" ]] \
  || { echo "codex filter version range drifted: $codex_range" >&2; exit 1; }
codex_keys=$("$ms" json get --file "$adapters/codex-config-filter.v1.json" --field keys)
codex_paths=$(while IFS= read -r filter_entry; do
    "$ms" json get --value "$filter_entry" --field path
  done < <(json_elements "$codex_keys"))
[[ "$codex_paths" == $'notice\nnotice.model_migrations\ntui.model_availability_nux' ]] \
  || { echo "codex filter key paths drifted: $codex_paths" >&2; exit 1; }
while IFS= read -r filter_entry; do
  [[ "$("$ms" json get --value "$filter_entry" --field source)" == *KI-19* ]] \
    || { echo "codex filter key lost its KI-19 source: $filter_entry" >&2; exit 1; }
done < <(json_elements "$codex_keys")
[[ "$(canonical_json "$(cat "$adapters/claude-config-filter.v1.json")")" == \
   "$(canonical_json '{"cliVersionRange": {"min": "2.1.0", "max": "2.1.x"}, "keys": []}')" ]] \
  || { echo "claude filter drifted from its version-one data" >&2; exit 1; }
[[ "$(canonical_json "$(cat "$adapters/devin-config-filter.v1.json")")" == \
   "$(canonical_json '{"cliVersionRange": {"min": "3000.3.27", "max": "3000.3.27"}, "keys": []}')" ]] \
  || { echo "devin filter drifted from its version-one data" >&2; exit 1; }

echo "configuration identity fixtures: PASSED"
