#!/usr/bin/env bash
# Compatibility plumbing for the engine-owned dependency audit.
set -euo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
if [[ "${1:-}" == --self-test ]]; then
  cd "$root"
  exec go test ./internal/audit -run '^(TestShellCommandWords|TestAuditDependencies|TestShellAssignment)'
fi
exec "${METASYSTEM_BIN:-$root/bin/metasystem}" audit dependency-ratchet --root "$root" "$@"
