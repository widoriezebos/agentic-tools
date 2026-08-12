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

identity_before=$(
  helper --runtime codex --version 0.146.0 \
    --filter "$root/scripts/agents/adapters/codex-config-filter.v1.json" "$config"
)
cat >"$config" <<'TOML'
model = "gpt-5.6-sol"

[notice]
hide_rate_limit_model_nudge = true

[notice.model_migrations]
"gpt-5.2" = "gpt-5.6-sol"

[tui.model_availability_nux]
"gpt-5.6-sol" = 2
TOML
identity_bookkeeping=$(
  helper --runtime codex --version 0.146.0 \
    --filter "$root/scripts/agents/adapters/codex-config-filter.v1.json" "$config"
)

python3 - "$identity_before" "$identity_bookkeeping" <<'PY'
import json, sys
before, after = map(json.loads, sys.argv[1:])
assert before["configHash"] == after["configHash"]
assert before["configKeyHashes"] == after["configKeyHashes"]
assert set(before["configKeyHashes"]) == {"model"}
PY

sed 's/gpt-5.6-sol/gpt-5.6-terra/' "$config" >"$tmp/config-changed.toml"
identity_changed=$(
  helper --runtime codex --version 0.146.0 \
    --filter "$root/scripts/agents/adapters/codex-config-filter.v1.json" "$tmp/config-changed.toml"
)
python3 - "$identity_before" "$identity_changed" <<'PY'
import json, sys
before, after = map(json.loads, sys.argv[1:])
assert before["configHash"] != after["configHash"]
assert before["configKeyHashes"]["model"] != after["configKeyHashes"]["model"]
PY

# JSON and TOML source bytes are not themselves the identity. Equivalent JSON
# objects flatten and canonicalize to the same map even when ordering and
# whitespace differ.
full_filter="$tmp/full-filter.json"
cat >"$full_filter" <<'JSON'
{"cliVersionRange":{"min":"1.0.0","max":"1.0.x"},"keys":[]}
JSON
printf '%s\n' '{"nested":{"beta":2,"alpha":1},"model":"same"}' >"$tmp/a.json"
printf '%s\n' '{' '  "model": "same",' '  "nested": {"alpha": 1, "beta": 2}' '}' >"$tmp/b.json"
canonical_a=$(helper --runtime fixture --version 1.0.0 --filter "$full_filter" "$tmp/a.json")
canonical_b=$(helper --runtime fixture --version 1.0.0 --filter "$full_filter" "$tmp/b.json")
python3 - "$canonical_a" "$canonical_b" <<'PY'
import json, sys
left, right = map(json.loads, sys.argv[1:])
assert left["configHash"] == right["configHash"]
assert left["configKeyHashes"] == right["configKeyHashes"]
assert set(left["configKeyHashes"]) == {"model", "nested.alpha", "nested.beta"}
PY

# A malformed filter and an out-of-range filter both warn and hash the full
# canonical map. Their result must equal the valid empty-filter identity.
printf '{not-json\n' >"$tmp/malformed-filter.json"
malformed=$(
  helper --runtime fixture --version 1.0.0 --filter "$tmp/malformed-filter.json" \
    "$tmp/a.json" 2>"$tmp/malformed.err"
)
out_of_range=$(
  helper --runtime codex --version 0.147.0 \
    --filter "$root/scripts/agents/adapters/codex-config-filter.v1.json" \
    "$tmp/a.json" 2>"$tmp/out-of-range.err"
)
full=$(helper --runtime fixture --version 1.0.0 --filter "$full_filter" "$tmp/a.json")
grep -Fq 'hashing all canonical configuration keys' "$tmp/malformed.err"
grep -Fq 'outside filter range' "$tmp/out-of-range.err"
python3 - "$malformed" "$out_of_range" "$full" <<'PY'
import json, sys
malformed, out_of_range, full = map(json.loads, sys.argv[1:])
assert malformed["configHash"] == full["configHash"]
assert out_of_range["configHash"] == full["configHash"]
assert malformed["configKeyHashes"] == full["configKeyHashes"]
assert out_of_range["configKeyHashes"] == full["configKeyHashes"]
PY

# The capability refusal compares the snapshot's per-key hashes with the
# current identity and names the behavior-changing canonical key.
fixture_root="$tmp/selector-root"
mkdir -p "$fixture_root/artifacts/agents/capabilities" "$fixture_root/scripts/agents/roles"
printf '%s\n' '{"required":[],"optional":{},"waivers":{}}' \
  >"$fixture_root/scripts/agents/roles/implementer.requirements.json"
printf '%s\n' '{"readRoots":[],"writeRoots":[],"network":"deny"}' >"$tmp/envelope.json"
python3 - "$fixture_root" "$identity_before" <<'PY'
import json, sys
from datetime import datetime, timezone
from pathlib import Path
root, identity = Path(sys.argv[1]), json.loads(sys.argv[2])
captured = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
path = root / "artifacts" / "agents" / "capabilities" / (
    f"codex-0.146.0-{identity['configHash']}-20260807-001.json"
)
value = {
    "runtime": "codex", "cliVersion": "0.146.0",
    "configHash": identity["configHash"],
    "configKeyHashes": identity["configKeyHashes"],
    "capturedAt": captured, "sequence": 1,
    "capabilities": {"sessionEstablishedTimeoutSec": 2},
    "permissions": {"unverified": []},
    "envelopeEnforcement": {
        "writeRoots": "mapped", "readRoots": "notEnforced", "network": "mapped"
    },
}
path.write_text(json.dumps(value) + "\n", encoding="utf-8")
PY
if selector --root "$fixture_root" --runtime codex --role implementer \
  --identity "$identity_changed" --max-age 30 --envelope "$tmp/envelope.json" \
  --output "$tmp/selected.json" 2>"$tmp/refusal.err"; then
  echo "configuration identity fixture: changed model unexpectedly selected an old snapshot" >&2
  exit 1
fi
grep -Fq 'changed configuration keys: model' "$tmp/refusal.err"
if grep -Fq 'notice' "$tmp/refusal.err"; then
  echo "configuration identity fixture: refusal named filtered bookkeeping churn" >&2
  exit 1
fi

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
