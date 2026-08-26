#!/usr/bin/env bash
set -euo pipefail

source_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-record-protocol.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
repository="$tmp/repository"
fixture_root="$repository/metasystem"
mkdir -p "$fixture_root/scripts/agents" "$fixture_root/artifacts/agents/jobs" \
  "$fixture_root/artifacts/agents/record-locks"
git -C "$repository" init -q -b main
cp "$source_root/scripts/agents/dispatch.sh" "$fixture_root/scripts/agents/dispatch.sh"
# The copied dispatcher resolves its engine as <fixture>/bin/metasystem; the
# lease, identity, and authority helpers it used to need as .py files live
# inside that one binary now.
mkdir -p "$fixture_root/bin"
cp "$source_root/bin/metasystem" "$fixture_root/bin/metasystem"
dispatch="$fixture_root/scripts/agents/dispatch.sh"
# The record create is a control-plane write. Under an agent-run
# suite the ambient ancestry classifies UNTRUSTED in this sandbox,
# so this shell announces itself as the sandbox's main — what a
# starting main does; a terminal run passed as HUMAN and still does.
printf 'metasystem.runtimes=fake\nrole.default.model.fake=fake-model\n' > "$fixture_root/metasystem.conf"
"$fixture_root/bin/metasystem" lease announce --root "$fixture_root" \
  --session record-protocol --pid $$ \
  --start "$("$fixture_root/bin/metasystem" proc started-at --pid $$)" \
  --tag fixture-record-protocol --runtime fake >/dev/null

# Dispatch setup persists the epoch-tagged pending-setup identity first. The
# setup completion may fill the record only when main identity and claim epoch
# still match that first locked record.
# Everything behavioral retired to internal/dispatch/record_test.go
# (script-fixtures-012/D43): reservation persistence, setup completion,
# epoch-mismatch refusal, protocol-error stamping and idempotency were
# verified one-to-one, and the one novel property — no reader ever sees
# status=failed without its protocolError object, plus the .tmp-residue
# check — was PORTED as TestRecordProtocolErrorNeverExposesFailedWithout-
# Violation (a live goroutine reader under -race, stronger than the
# python poller it replaces). This file keeps the one thing Go cannot
# prove: the __record-create dispatch path forwards.

cat >"$tmp/smoke.json" <<'JSON'
{"jobId":"smoke-job","status":"pending-setup","mainId":"main-smoke","claimEpoch":1}
JSON
"$dispatch" __record-create --job smoke-job --source "$tmp/smoke.json"
grep -Fq '"jobId": "smoke-job"' "$fixture_root/artifacts/agents/jobs/smoke-job.json" \
  || { echo "record-protocol smoke: __record-create did not persist the reservation" >&2; exit 1; }

echo "record and protocol-error fixtures: PASSED"
