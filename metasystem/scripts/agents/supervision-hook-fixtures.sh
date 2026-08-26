#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"
hook=$root/scripts/agents/supervision-hook.sh
[[ -x "$ms" ]] \
  || { echo "supervision hook fixture: binary absent; run the go gate first" >&2; exit 1; }

tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-supervision-hook-fixture.XXXXXX")
cleanup() { rm -rf "$tmp"; }
trap cleanup EXIT

# The registry producer writes the matching runtime first and then more than a
# pipe buffer of declarations. A membership reader must let the registry query
# finish; a short-reading consumer can otherwise turn a valid match into the
# producer's broken-pipe status under pipefail.
fixture_engine=$tmp/metasystem
cat >"$fixture_engine" <<'SH'
#!/usr/bin/env bash
if [[ ${1:-} == runtime && ${2:-} == list ]]; then
  awk 'BEGIN {
    print "claude"
    for (item = 0; item < 16384; item++) {
      print "runtime-membership-padding-" item
    }
  }'
  exit $?
fi
exec "${METASYSTEM_RUNTIME_MEMBERSHIP_REAL_ENGINE:?}" "$@"
SH
chmod +x "$fixture_engine"

printf '{"session_id":"fixture","cwd":"/","hook_event_name":"Stop"}\n' >"$tmp/payload.json"
membership_rc=0
METASYSTEM_BIN="$fixture_engine" METASYSTEM_RUNTIME_MEMBERSHIP_REAL_ENGINE="$ms" \
  bash "$hook" claude stop <"$tmp/payload.json" \
    >"$tmp/hook.out" 2>"$tmp/hook.err" || membership_rc=$?
if [[ $membership_rc != 0 ]]; then
  echo "supervision hook runtime-membership fixture failed: a registered runtime was refused (exit $membership_rc)" >&2
  sed -n '1,40p' "$tmp/hook.err" >&2
  exit 1
fi

echo "supervision hook runtime-membership fixture passed"
