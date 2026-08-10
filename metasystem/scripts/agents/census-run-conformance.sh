#!/usr/bin/env bash
# Differential conformance for the census RUN verdict (the capstone of the
# census port, plans/go-migration.md Phase 1): a recorded bundle — process
# table + supervision state + announcements + custody records — is fed to
# BOTH process-census.py and the Go binary, and the normalized verdicts must
# be identical. This is the recorded-input differential the plan requires.
#
# This IS the seam-1 retirement artifact's sibling: the FULL census verb now
# exists; census-go-fixtures.sh (the seam-1 trigger) lands when the watcher
# is switched to it. This file proves the verdict conforms.
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
bin="$root/bin/metasystem"
[[ -x "$bin" ]] || { echo "census run conformance: binary absent" >&2; exit 1; }

tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-census-run.XXXXXX")
trap 'rm -rf "$tmp"' EXIT

# A sandbox metasystem root: runtimes=fake (the fixture-path guard), the fake
# adapter, a recorded process table, supervision state, an announcement, and a
# live custody job — a realistic classification bundle.
sandbox="$tmp/root"
mkdir -p "$sandbox/scripts/agents/adapters" "$sandbox/artifacts/agents/mains" \
         "$sandbox/artifacts/agents/jobs" "$sandbox/artifacts/agents/supervision"
cp "$root/scripts/agents/adapters/fake.sh" "$sandbox/scripts/agents/adapters/fake.sh"
cp "$root/scripts/agents/process-census.py" "$sandbox/scripts/agents/process-census.py"
printf 'metasystem.runtimes=fake\nwatch.interval-sec=60\n' > "$sandbox/metasystem.conf"
# The census reads state/announcements/custody from METASYSTEM_ROOT via the
# python's module-level constant; both engines are pointed at the sandbox.
repo="$sandbox"

now=$(date +%s)
# A recorded process table: an announced main, a custody child, an untracked
# agent, and an out-of-scope process — all fake-agent shaped, in-scope by cwd.
cat > "$tmp/procs.json" <<JSON
[
  {"pid":4101,"ppid":1,"pgid":4101,"pidStartedAt":$now,"argv":"metasystem-fake-agent announced","cwd":"$repo","cwdError":false,"alive":true},
  {"pid":4102,"ppid":1,"pgid":4102,"pidStartedAt":$now,"argv":"metasystem-fake-agent custody","cwd":"$repo","cwdError":false,"alive":true},
  {"pid":4103,"ppid":1,"pgid":4103,"pidStartedAt":$now,"argv":"metasystem-fake-agent untracked","cwd":"$repo","cwdError":false,"alive":true},
  {"pid":4104,"ppid":1,"pgid":4104,"pidStartedAt":$now,"argv":"metasystem-fake-agent outside","cwd":"$tmp/elsewhere","cwdError":false,"alive":true}
]
JSON
mkdir -p "$tmp/elsewhere"
# Supervision state (valid): owner + watcher + reaper.
cat > "$repo/artifacts/agents/supervision/state.json" <<JSON
{"generation":3,"owner":{"pid":9001,"pidStartedAt":$now,"instanceTag":"owner-tag"},
 "components":{"watcher":{"pid":9002,"pidStartedAt":$now,"instanceTag":"watcher-tag"},
 "reaper":{"pid":9003,"pidStartedAt":$now,"instanceTag":"reaper-tag"}}}
JSON
# An announcement for pid 4101.
cat > "$repo/artifacts/agents/mains/session-4101.json" <<JSON
{"sessionId":"s","pid":4101,"pidStartedAt":$now,"pgid":4101,"runtime":"fake","instanceTag":"main-4101","announcedAt":"2026-08-10T00:00:00Z"}
JSON
# A live custody job for pid 4102.
cat > "$repo/artifacts/agents/jobs/job-4102.json" <<JSON
{"jobId":"j","status":"running","instanceTag":"job-4102","pid":4102,"pidStartedAt":$now}
JSON

fp="deadbeef"

normalize() { # strip the fields that legitimately differ run-to-run
  python3 - "$1" <<'PY'
import json, sys
v = json.load(open(sys.argv[1]))
for k in ("completedAt", "completedAtEpoch", "durationMs"):
    v.pop(k, None)
print(json.dumps(v, sort_keys=True, indent=2))
PY
}

METASYSTEM_CENSUS_PROCESS_FILE="$tmp/procs.json" \
  python3 "$sandbox/scripts/agents/process-census.py" census --repo "$repo" \
  --fingerprint "$fp" --interval 60 --output "$tmp/py-verdict.json" >/dev/null 2>&1 || true
METASYSTEM_CENSUS_PROCESS_FILE="$tmp/procs.json" \
  "$bin" census run --repo "$repo" --root "$repo" --fingerprint "$fp" --interval 60 \
  --output "$tmp/go-verdict.json"

if ! diff <(normalize "$tmp/py-verdict.json") <(normalize "$tmp/go-verdict.json") >"$tmp/diff.out"; then
  echo "census run conformance FAILED: verdicts differ" >&2
  cat "$tmp/diff.out" >&2
  exit 1
fi
echo "census run conformance: PASSED (go verdict == python verdict on the recorded bundle)"
