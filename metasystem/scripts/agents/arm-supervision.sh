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
# self-check rejects a seat registry home or any main identity outside the
# scenario's fixture bed; production invocations carry no audit path and retain
# the exec-only shape.
if [[ -n "${METASYSTEM_SUPERVISION_FIXTURE_AUDIT:-}" ]]; then
  fixture_shutdown=0
  fixture_main_pid=
  fixture_main_in_bed=-
  fixture_previous=
  for fixture_argument in "$@"; do
    if [[ "$fixture_previous" == --pid ]]; then
      fixture_main_pid=$fixture_argument
    fi
    [[ "$fixture_argument" != --shutdown ]] || fixture_shutdown=1
    fixture_previous=$fixture_argument
  done
  if (( ! fixture_shutdown )); then
    if [[ "$fixture_main_pid" =~ ^[1-9][0-9]*$ && "${METASYSTEM_SUPERVISION_FIXTURE_BED_PID:-}" =~ ^[1-9][0-9]*$ ]]; then
      fixture_main_in_bed=0
      fixture_ancestor=$fixture_main_pid
      for ((fixture_depth = 0; fixture_depth < 128 && fixture_ancestor > 1; fixture_depth++)); do
        if [[ "$fixture_ancestor" == "$METASYSTEM_SUPERVISION_FIXTURE_BED_PID" ]]; then
          fixture_main_in_bed=1
          break
        fi
        fixture_parent=$(ps -p "$fixture_ancestor" -o ppid= 2>/dev/null | tr -d '[:space:]')
        [[ "$fixture_parent" =~ ^[1-9][0-9]*$ && "$fixture_parent" != "$fixture_ancestor" ]] || break
        fixture_ancestor=$fixture_parent
      done
    fi
    printf '%s\t%s\t%s\t%s\n' "${METASYSTEM_SUPERVISION_REGISTRY_HOME:-}" "${fixture_main_pid:--}" arm-supervision "$fixture_main_in_bed" \
      >>"$METASYSTEM_SUPERVISION_FIXTURE_AUDIT"
  fi
fi

exec "$ms" up --metasystem-root "$harness_root" "$@"
