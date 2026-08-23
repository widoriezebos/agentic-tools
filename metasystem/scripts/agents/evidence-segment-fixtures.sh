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
  git -C "$checkout" init -q -b main
  cp "$source_root/scripts/agents/dispatch.sh" "$fixture_root/scripts/agents/dispatch.sh"
  # The copied dispatcher resolves its engine as <fixture>/bin/metasystem;
  # the retired .py helpers live inside that one binary now.
  mkdir -p "$fixture_root/bin"
  cp "$source_root/bin/metasystem" "$fixture_root/bin/metasystem"
  cp "$source_root/scripts/metasystem-config.sh" "$fixture_root/scripts/metasystem-config.sh"
  printf 'evidence.root=%s\nmetasystem.runtimes=fake\n' "$evidence" >"$fixture_root/metasystem.conf"
  # The held reap is a control-plane write. Under an agent-run suite
  # the ambient ancestry classifies UNTRUSTED in this sandbox, so
  # this shell announces itself as the sandbox's main — what a
  # starting main does; a terminal run passed as HUMAN and still does.
  "$fixture_root/bin/metasystem" lease announce --root "$fixture_root" \
    --session evidence-segment --pid $$ \
    --start "$("$fixture_root/bin/metasystem" proc started-at --pid $$)" \
    --tag fixture-evidence-segment --runtime fake >/dev/null
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
# Two checkouts of the same chain must land in two distinct evidence
# segments, each carrying a complete manifest.
destinations=()
for destination in "$evidence"/agents/*/segment-chain; do
  [[ -d "$destination" ]] || continue
  destinations+=("$destination")
done
[[ ${#destinations[@]} -eq 2 ]] || {
  echo "evidence segment fixture: expected two mirrored segments, found ${#destinations[@]}" >&2
  exit 1
}
[[ "$(basename "$(dirname "${destinations[0]}")")" != "$(basename "$(dirname "${destinations[1]}")")" ]] || {
  echo "evidence segment fixture: both checkouts mirrored into the same segment" >&2
  exit 1
}
for destination in "${destinations[@]}"; do
  manifest="$destination/manifest.json"
  [[ "$("$source_root/bin/metasystem" json get --file "$manifest" --field rootJob)" == segment-chain ]] || {
    echo "evidence segment fixture: $manifest does not name its root job" >&2
    exit 1
  }
  # Manifest keys carry dots and slashes, so membership is checked against
  # the engine's canonical rendering of the files object; no manifest value
  # can spell a quoted key followed by a colon.
  manifest_files=$("$source_root/bin/metasystem" json get --file "$manifest" --field files)
  for manifest_key in brief.md rounds/1/raw.out jobs/segment-chain.json; do
    [[ "$manifest_files" == *"\"$manifest_key\":"* ]] || {
      echo "evidence segment fixture: $manifest does not cover $manifest_key" >&2
      exit 1
    }
  done
done

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
printf 'evidence.root=%s\nmetasystem.runtimes=fake\n' "$evidence" >"$legacy_root/metasystem.conf"
# Same announcement as mirror_checkout: the held GC is a
# control-plane write and ambient ancestry classifies UNTRUSTED
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
