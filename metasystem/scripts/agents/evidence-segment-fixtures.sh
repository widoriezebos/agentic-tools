#!/usr/bin/env bash
set -euo pipefail

source_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-evidence-segments.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
evidence="$tmp/evidence"
mkdir -p "$evidence"

# The per-checkout segment leg (two checkouts of one chain mirror into
# distinct evidence segments) is a Go test with the delegate lifecycle:
# internal/delegation TestTwoCheckoutsOfOneChainMirrorIntoDistinctEvidenceSegments.

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
printf 'evidence.root=%s\nmetasystem.runtimes=fake\nrole.default.model.fake=fake-model\n' "$evidence" >"$legacy_root/metasystem.conf"
# The held GC is a control-plane write and ambient ancestry classifies UNTRUSTED
# under an agent-run suite.
"$legacy_root/bin/metasystem" lease announce --root "$legacy_root" \
  --session evidence-legacy --pid $$ \
  --start "$("$legacy_root/bin/metasystem" proc started-at --pid $$)" \
  --tag fixture-evidence-legacy --runtime fake >/dev/null
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
legacy_payload="$legacy_root/artifacts/agents/legacy-chain/brief.md"
legacy_digest=$("$source_root/bin/metasystem" util sha256 --file "$legacy_payload")
legacy_bytes=$(($(wc -c <"$legacy_payload")))
legacy_manifest="$evidence/agents/legacy-chain/manifest.json"
legacy_staged=$(mktemp "$(dirname "$legacy_manifest")/.manifest.XXXXXX")
printf '{"rootJob":"legacy-chain","files":{"brief.md":{"sha256":"%s","bytes":%s}},"updatedAt":"%s"}\n' \
  "$legacy_digest" "$legacy_bytes" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >"$legacy_staged"
mv "$legacy_staged" "$legacy_manifest"
"$legacy_root/scripts/agents/evidence-gc.sh" __lease-held human >"$tmp/legacy-gc.out"
grep -Fq 'collected legacy-chain' "$tmp/legacy-gc.out"
[[ ! -e "$legacy_root/artifacts/agents/legacy-chain" ]] || {
  echo "evidence segment fixture: legacy manifest payload was not collected" >&2
  exit 1
}

echo "evidence segment fixtures: PASSED"
