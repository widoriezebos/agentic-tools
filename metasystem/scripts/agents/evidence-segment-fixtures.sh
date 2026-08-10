#!/usr/bin/env bash
set -euo pipefail

source_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-evidence-segments.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
evidence="$tmp/evidence"
mkdir -p "$evidence"

mirror_checkout() { # checkout directory
  local checkout=$1 fixture_root=$1/metasystem
  mkdir -p "$fixture_root/scripts/agents" "$fixture_root/scripts" \
    "$fixture_root/artifacts/agents/jobs" "$fixture_root/artifacts/agents/segment-chain/rounds/1"
  git -C "$checkout" init -q
  cp "$source_root/scripts/agents/dispatch.sh" "$fixture_root/scripts/agents/dispatch.sh"
  # The copied dispatcher resolves its engine as <fixture>/bin/metasystem;
  # the retired .py helpers live inside that one binary now.
  mkdir -p "$fixture_root/bin"
  cp "$source_root/bin/metasystem" "$fixture_root/bin/metasystem"
  cp "$source_root/scripts/metasystem-config.sh" "$fixture_root/scripts/metasystem-config.sh"
  printf 'evidence.root=%s\n' "$evidence" >"$fixture_root/metasystem.conf"
  cat >"$fixture_root/artifacts/agents/jobs/segment-chain.json" <<'JSON'
{
  "jobId": "segment-chain",
  "parentJob": null,
  "round": 1,
  "status": "completed",
  "runtime": "fake",
  "capabilitySnapshot": "artifacts/agents/capabilities/absent.json",
  "mirror": null
}
JSON
  printf 'checkout=%s\n' "$checkout" >"$fixture_root/artifacts/agents/segment-chain/brief.md"
  printf 'round evidence\n' >"$fixture_root/artifacts/agents/segment-chain/rounds/1/raw.out"
  "$fixture_root/scripts/agents/dispatch.sh" __reap-held --job segment-chain >/dev/null
}

mirror_checkout "$tmp/checkout-a"
mirror_checkout "$tmp/checkout-b"
python3 - "$evidence" <<'PY'
import json, sys
from pathlib import Path
evidence = Path(sys.argv[1])
destinations = sorted((evidence / "agents").glob("*/segment-chain"))
assert len(destinations) == 2, destinations
assert destinations[0].parent.name != destinations[1].parent.name
for destination in destinations:
    manifest = json.loads((destination / "manifest.json").read_text(encoding="utf-8"))
    assert manifest["rootJob"] == "segment-chain"
    assert "brief.md" in manifest["files"]
    assert "rounds/1/raw.out" in manifest["files"]
    assert "jobs/segment-chain.json" in manifest["files"]
PY

# The collector continues to understand the pre-segmentation location
# evidence/agents/<chain>/manifest.json. A closed terminal payload covered by
# that legacy manifest is still collected.
legacy_root="$tmp/legacy-checkout/metasystem"
mkdir -p "$legacy_root/scripts/agents" "$legacy_root/scripts" \
  "$legacy_root/artifacts/agents/jobs" "$legacy_root/artifacts/agents/legacy-chain" \
  "$evidence/agents/legacy-chain"
cp "$source_root/scripts/agents/evidence-gc.sh" "$legacy_root/scripts/agents/evidence-gc.sh"
mkdir -p "$legacy_root/bin"
cp "$source_root/bin/metasystem" "$legacy_root/bin/metasystem"
cp "$source_root/scripts/metasystem-config.sh" "$legacy_root/scripts/metasystem-config.sh"
printf 'evidence.root=%s\n' "$evidence" >"$legacy_root/metasystem.conf"
cat >"$legacy_root/artifacts/agents/jobs/legacy-chain.json" <<JSON
{
  "jobId": "legacy-chain",
  "parentJob": null,
  "status": "completed",
  "chainClosed": true,
  "mirror": {"path": "$evidence/agents/legacy-chain"}
}
JSON
printf 'legacy payload\n' >"$legacy_root/artifacts/agents/legacy-chain/brief.md"
python3 - "$legacy_root/artifacts/agents/legacy-chain/brief.md" \
  "$evidence/agents/legacy-chain/manifest.json" <<'PY'
import hashlib, json, sys
from datetime import datetime, timezone
from pathlib import Path
payload, manifest = map(Path, sys.argv[1:])
digest = hashlib.sha256(payload.read_bytes()).hexdigest()
manifest.write_text(json.dumps({
    "rootJob": "legacy-chain",
    "files": {"brief.md": {"sha256": digest, "bytes": payload.stat().st_size}},
    "updatedAt": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
}, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
"$legacy_root/scripts/agents/evidence-gc.sh" __lease-held human >"$tmp/legacy-gc.out"
grep -Fq 'collected legacy-chain' "$tmp/legacy-gc.out"
[[ ! -e "$legacy_root/artifacts/agents/legacy-chain" ]] || {
  echo "evidence segment fixture: legacy manifest payload was not collected" >&2
  exit 1
}

echo "evidence segment fixtures: PASSED"
