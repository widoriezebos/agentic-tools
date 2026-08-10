#!/usr/bin/env bash
# Differential conformance for the census fingerprint port: the Go digest
# must equal process-census.py's byte-for-byte on the real repo (strict
# port, plans/go-migration.md Phase 1). Not the seam-1 retirement artifact.
set -euo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
bin="$root/bin/metasystem"
[[ -x "$bin" ]] || { echo "fingerprint conformance: binary absent" >&2; exit 1; }
go_fp=$("$bin" census fingerprint --repo "$root" --root "$root")
py_fp=$(python3 "$root/scripts/agents/process-census.py" fingerprint --repo "$root")
if [[ "$go_fp" != "$py_fp" ]]; then
  echo "fingerprint conformance FAILED: go=$go_fp python=$py_fp" >&2
  exit 1
fi
echo "census fingerprint conformance: PASSED (go == python: $go_fp)"
