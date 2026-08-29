#!/usr/bin/env bash
set -euo pipefail

# Compatibility plumbing for callers that have not moved to the top-level
# operator verb yet. Arming decisions and process custody live in `metasystem
# up`; this script resolves the checkout-local binary and transfers control.
script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
harness_root=$(cd "$script_dir/../.." && pwd -P)
ms="${METASYSTEM_BIN:-$harness_root/bin/metasystem}"

if [[ ${1:-} == fingerprint ]]; then
  shift
  exec "$ms" supervise fingerprint --root "$harness_root" "$@"
fi

exec "$ms" up --metasystem-root "$harness_root" "$@"
