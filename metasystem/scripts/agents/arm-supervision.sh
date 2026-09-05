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

# The supervision fixture records every ordinary arm attempt before this
# compatibility process disappears into `metasystem up`. The fixture's final
# self-check rejects a seat registry home or the fixture-bed suite identity;
# production invocations carry no audit path and retain the exec-only shape.
if [[ -n "${METASYSTEM_SUPERVISION_FIXTURE_AUDIT:-}" ]]; then
  fixture_shutdown=0
  fixture_main_pid=
  fixture_previous=
  for fixture_argument in "$@"; do
    if [[ "$fixture_previous" == --pid ]]; then
      fixture_main_pid=$fixture_argument
    fi
    [[ "$fixture_argument" != --shutdown ]] || fixture_shutdown=1
    fixture_previous=$fixture_argument
  done
  if (( ! fixture_shutdown )); then
    printf '%s\t%s\n' "${METASYSTEM_SUPERVISION_REGISTRY_HOME:-}" "${fixture_main_pid:--}" \
      >>"$METASYSTEM_SUPERVISION_FIXTURE_AUDIT"
  fi
fi

exec "$ms" up --metasystem-root "$harness_root" "$@"
