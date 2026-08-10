#!/usr/bin/env bash
set -euo pipefail

source_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-record-protocol.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
repository="$tmp/repository"
fixture_root="$repository/metasystem"
mkdir -p "$fixture_root/scripts/agents" "$fixture_root/artifacts/agents/jobs" \
  "$fixture_root/artifacts/agents/record-locks"
git -C "$repository" init -q
cp "$source_root/scripts/agents/dispatch.sh" "$fixture_root/scripts/agents/dispatch.sh"
# The copied dispatcher resolves its engine as <fixture>/bin/metasystem; the
# lease, identity, and authority helpers it used to need as .py files live
# inside that one binary now.
mkdir -p "$fixture_root/bin"
cp "$source_root/bin/metasystem" "$fixture_root/bin/metasystem"
dispatch="$fixture_root/scripts/agents/dispatch.sh"

# Dispatch setup persists the epoch-tagged pending-setup identity first. The
# setup completion may fill the record only when main identity and claim epoch
# still match that first locked record.
cat >"$tmp/setup-first.json" <<'JSON'
{"jobId":"setup-job","status":"pending-setup","mainId":"main-a","claimEpoch":7}
JSON
"$dispatch" __record-create \
  --job setup-job --source "$tmp/setup-first.json"
python3 - "$fixture_root/artifacts/agents/jobs/setup-job.json" <<'PY'
import json, sys
value = json.load(open(sys.argv[1]))
assert value == {"jobId": "setup-job", "status": "pending-setup", "mainId": "main-a", "claimEpoch": 7}
PY
cat >"$tmp/setup-complete.json" <<'JSON'
{"jobId":"setup-job","status":"pending","mainId":"main-a","claimEpoch":7,"sideEffectsComplete":true}
JSON
"$dispatch" __record-setup \
  --job setup-job --source "$tmp/setup-complete.json"

cat >"$tmp/stale-first.json" <<'JSON'
{"jobId":"stale-setup","status":"pending-setup","mainId":"main-a","claimEpoch":7}
JSON
cat >"$tmp/stale-complete.json" <<'JSON'
{"jobId":"stale-setup","status":"pending","mainId":"main-b","claimEpoch":8}
JSON
"$dispatch" __record-create \
  --job stale-setup --source "$tmp/stale-first.json"
if "$dispatch" __record-setup \
  --job stale-setup --source "$tmp/stale-complete.json" 2>"$tmp/stale.err"; then
  echo "record fixture: setup crossed a claim epoch" >&2
  exit 1
fi
grep -Fq 'invalid setup transition' "$tmp/stale.err"
python3 - "$fixture_root/artifacts/agents/jobs/stale-setup.json" <<'PY'
import json, sys
value = json.load(open(sys.argv[1]))
assert value["status"] == "pending-setup"
assert value["mainId"] == "main-a" and value["claimEpoch"] == 7
PY

# A protocol error is the round record's terminal transition and typed error
# in one atomic replacement. A tight reader may see the old or new record, but
# never a failed/protocol_error record without its keyed protocolError object.
cat >"$fixture_root/artifacts/agents/jobs/chain.json" <<'JSON'
{"jobId":"chain","round":1,"status":"completed","parentJob":null}
JSON
cat >"$fixture_root/artifacts/agents/jobs/chain-r2.json" <<'JSON'
{"jobId":"chain-r2","round":2,"status":"running","parentJob":"chain","error":null}
JSON
violation='return has wrong runtime'
printf '%s\n' "$violation" >"$tmp/violation.txt"
python3 - "$fixture_root/artifacts/agents/jobs/chain-r2.json" "$tmp/reader.stop" "$tmp/reader.bad" <<'PY' &
import json, sys, time
from pathlib import Path
record, stop, bad = map(Path, sys.argv[1:])
while not stop.exists():
    try:
        value = json.loads(record.read_text(encoding="utf-8"))
    except (OSError, ValueError):
        bad.write_text("record was absent or partial\n", encoding="utf-8")
        break
    if value.get("status") == "failed" and (
        value.get("error") != "protocol_error" or not isinstance(value.get("protocolError"), dict)
    ):
        bad.write_text("terminal state was visible without its protocol error\n", encoding="utf-8")
        break
    time.sleep(0.0005)
PY
reader=$!
"$dispatch" __protocol-error \
  --job chain-r2 --expect running --violation-file "$tmp/violation.txt"
# Repeating the same validation is idempotent even though the record is now
# terminal; it does not append or relocate another chain-level entry.
"$dispatch" __protocol-error \
  --job chain-r2 --expect running --violation-file "$tmp/violation.txt"
touch "$tmp/reader.stop"
wait "$reader"
[[ ! -e "$tmp/reader.bad" ]] || { cat "$tmp/reader.bad" >&2; exit 1; }

python3 - "$fixture_root/artifacts/agents/jobs/chain.json" \
  "$fixture_root/artifacts/agents/jobs/chain-r2.json" "$violation" <<'PY'
import hashlib, json, sys
root = json.load(open(sys.argv[1]))
round_record = json.load(open(sys.argv[2]))
violation = sys.argv[3]
expected = hashlib.sha256(f"chain-r2{2}{violation}".encode()).hexdigest()[:16]
assert "protocolError" not in root
assert round_record["status"] == "failed"
assert round_record["error"] == "protocol_error"
assert round_record["phase"] == "validation"
assert round_record["protocolError"]["key"] == expected
assert round_record["protocolError"]["violation"] == violation
assert round_record["protocolError"]["detectedAt"].endswith("Z")
PY
if find "$fixture_root/artifacts/agents/record-locks" -name '*.tmp' -print -quit | grep -q .; then
  echo "record fixture: an interrupted atomic-write temporary file remained" >&2
  exit 1
fi

echo "record and protocol-error fixtures: PASSED"
