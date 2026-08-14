#!/usr/bin/env bash
# Owner-alone fixtures for the GO supervision engine (plans/go-migration.md,
# Phase 0 acceptance). These drive the RUNNING BINARY through the design's
# owner-alone Proof rows in scratch checkouts — the acceptance that a unit
# test against fakes cannot give, and that the overnight overclaim skipped.
#
# Non-owner Proof rows (gate, janitor, custody, cohort ledger, registry-lock
# crash cases) are DEFERRED to their phases and named at the end so the set
# is closed, per GO-MIG-R3-001/R2-008.
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
bin="$root/bin/metasystem"
[[ -x "$bin" ]] || { echo "go supervision fixtures: binary absent; run the go gate first" >&2; exit 1; }

# Caps come from their single declared owner (script-fixtures-017): the
# wait ceilings scale with the machine like every other fixture's.
source "$root/scripts/agents/fixture-budget.sh"
harness_fixture_budget_init "$root"
owner_wait=$(harness_fixture_cap go-owner-wait)
owner_crashloop=$(harness_fixture_cap go-owner-crashloop)

tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-go-supervision.XXXXXX")
# The tag prefix is unique per run (script-fixtures-016): a bare
# pkill -f "gofix-" reached ANY process on the machine whose command
# mentioned the string — another checkout's fixture run included.
fixture_tag="gofix-$$"
cleanup() {
  # Kill only what THIS run launched; the tag carries our pid.
  pkill -f "$fixture_tag-" 2>/dev/null || true
  rm -rf "$tmp"
}
trap cleanup EXIT

fail() { echo "go supervision fixture failed: $1" >&2; exit 1; }

# arm launches the owner the way arm-supervision.sh will: create the lock
# dir, launch the owner with a start gate, publish owner.json naming its
# pid, signal the gate. Echoes the owner pid.
arm() { # repo, tag, registry, interval
  local repo=$1 tag=$2 registry=$3 interval=$4 pid start gate
  mkdir -p "$repo/artifacts/agents/supervision/lock.d"
  gate="$repo/artifacts/agents/supervision/start.gate"
  "$bin" supervise owner --repo "$repo" --tag "$tag" --interval "$interval" \
    --registry "$registry" --gate "$gate" >"$repo/owner.out" 2>&1 &
  pid=$!
  start=$("$bin" proc started-at --pid "$pid") || fail "cannot read owner start time"
  printf '{"pid":%s,"pidStartedAt":%s,"instanceTag":"%s"}\n' "$pid" "$start" "$tag" \
    > "$repo/artifacts/agents/supervision/lock.d/owner.json"
  touch "$gate"
  printf '%s\n' "$pid"
}

wait_until() { # seconds, description, command...
  local deadline=$(( SECONDS + $1 )); shift
  local description=$1; shift
  until "$@"; do
    (( SECONDS < deadline )) || fail "timeout: $description"
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
  done
}

registry="$tmp/registry.jsonl"

# --- ESTABLISH + PUBLISH: the owner arms, publishes engine-stamped state
#     with its components at one generation, and stays stable (no churn).
repo1="$tmp/establish"; mkdir -p "$repo1"
owner1=$(arm "$repo1" "$fixture_tag-establish" "$registry" 1)
wait_until "$owner_wait" "state published with components" bash -c '
  python3 - "$1" <<PY 2>/dev/null || exit 1
import json,sys
d=json.load(open(sys.argv[1]))
raise SystemExit(0 if d.get("engine")=="go" and d.get("generation")==1 and set(d.get("components",{}))=={"watcher","reaper"} else 1)
PY' _ "$repo1/artifacts/agents/supervision/state.json"
# Stability: after several intervals the generation is still 1 (no churn).
# A literal on purpose: this is an assertion window, not a wait ceiling —
# cap scaling applies to ceilings only (script-fixtures-017).
sleep 3
gens=$(grep -o '"generation":[0-9]*' "$registry" | sort -u | tr '\n' ' ')
[[ "$gens" == '"generation":1 ' ]] || fail "owner churned generations: $gens"
kill "$owner1" 2>/dev/null || true
echo "go supervision: establish + stable publication PASSED" >&2

# --- PURPOSE-GONE: the checkout root vanishes; the owner exits purpose-gone
#     with teardownComplete true and an honest terminal (the KI-32 defect
#     designed away, observed in the running binary).
repo2="$tmp/purpose"; mkdir -p "$repo2"
owner2=$(arm "$repo2" "$fixture_tag-purpose" "$registry" 1)
wait_until "$owner_wait" "purpose owner established" bash -c '[[ -f "$1/artifacts/agents/supervision/state.json" ]]' _ "$repo2"
mv "$repo2" "$repo2.gone"   # atomic: root definitively absent, no writer race
wait_until "$owner_wait" "owner exits on purpose-gone" bash -c '! kill -0 "$1" 2>/dev/null' _ "$owner2"
python3 - "$registry" "$fixture_tag-purpose" <<'PY' || fail "no purpose-gone terminal with complete teardown"
import json, sys
exits = [json.loads(l) for l in open(sys.argv[1]) if '"event":"exited"' in l]
mine = [e for e in exits if e.get("ownerTag") == sys.argv[2]]
ok = mine and mine[-1]["reason"] == "purpose-gone" and mine[-1]["teardownComplete"] is True
raise SystemExit(0 if ok else 1)
PY
# No component of this owner survives its teardown.
! pgrep -f "$fixture_tag-purpose" >/dev/null 2>&1 || fail "a component survived purpose-gone teardown"
echo "go supervision: purpose-gone exit + none-survive teardown PASSED" >&2

# --- SUPERSEDED: another identity takes the lock while the checkout persists;
#     the owner leaves voluntarily (SLC-R3-003) with complete teardown.
repo3="$tmp/superseded"; mkdir -p "$repo3"
owner3=$(arm "$repo3" "$fixture_tag-super" "$registry" 1)
wait_until "$owner_wait" "superseded owner established" bash -c '[[ -f "$1/artifacts/agents/supervision/state.json" ]]' _ "$repo3"
printf '{"pid":999999,"pidStartedAt":1,"instanceTag":"a-successor"}\n' \
  > "$repo3/artifacts/agents/supervision/lock.d/owner.json"
wait_until "$owner_wait" "owner exits on supersession" bash -c '! kill -0 "$1" 2>/dev/null' _ "$owner3"
python3 - "$registry" "$fixture_tag-super" <<'PY' || fail "no superseded terminal"
import json, sys
exits = [json.loads(l) for l in open(sys.argv[1]) if '"event":"exited"' in l]
mine = [e for e in exits if e.get("ownerTag") == sys.argv[2]]
raise SystemExit(0 if mine and mine[-1]["reason"] == "superseded" else 1)
PY
echo "go supervision: superseded voluntary exit PASSED" >&2

# --- OBSERVABILITY: the cycle trace narrates the decision basis (the
#     extreme-observability ruling) — the establish owner's log carries the
#     three D-1 reads and a verdict on every line.
trace="$repo1/artifacts/agents/supervision/owner.ndjson"
[[ -s "$trace" ]] || fail "no cycle trace was written"
python3 - "$trace" <<'PY' || fail "cycle trace does not narrate the decision basis"
import json, sys
lines = [json.loads(l) for l in open(sys.argv[1]) if l.strip()]
armed = [l for l in lines if l.get("verdict")]
ok = armed and all(l.get("root") and l.get("currency") and l.get("verdict") for l in armed)
raise SystemExit(0 if ok else 1)
PY
echo "go supervision: cycle-trace observability PASSED" >&2

# --- CRASH-LOOP BREAKER (D-2, RC-2): components that beat once then die
#     every cycle trip the breaker at N=5; the owner gives up with a
#     complete teardown and an honest terminal — the OTHER half of the
#     KI-32 fix (a self-heal that cannot bound itself is the incident).
repo4="$tmp/breaker"; mkdir -p "$repo4"
owner4=$(METASYSTEM_GO_COMPONENT_CRASH_ON_START=1 arm "$repo4" "$fixture_tag-breaker" "$registry" 1)
# At interval 1s and N=5, giving-up lands within ~10s even with backoff
# (backoff gates relaunches, never observations — SLC-R3-005).
wait_until "$owner_crashloop" "owner gives up on the crash loop" bash -c '! kill -0 "$1" 2>/dev/null' _ "$owner4"
python3 - "$registry" "$fixture_tag-breaker" <<'PY' || fail "no giving-up terminal with complete teardown"
import json, sys
exits = [json.loads(l) for l in open(sys.argv[1]) if '"event":"exited"' in l]
mine = [e for e in exits if e.get("ownerTag") == sys.argv[2]]
ok = mine and mine[-1]["reason"] == "giving-up" and mine[-1]["teardownComplete"] is True
raise SystemExit(0 if ok else 1)
PY
! pgrep -f "$fixture_tag-breaker" >/dev/null 2>&1 || fail "a component survived giving-up teardown"
echo "go supervision: crash-loop breaker giving-up PASSED" >&2

# DEFERRED Proof rows, named so the acceptance set is closed
# (GO-MIG-R3-001): breaker-at-five and one-clock backoff need a
# fail-on-purpose component (Phase 0 completion); ceiling stop-the-set,
# write-ahead gating, and launched-append retry likewise; the gate,
# janitor, custody, cohort-ledger, and registry-lock-crash rows are
# Phase 0b/1/3 and carry their own fixture files.
echo "go supervision fixtures: PASSED (owner-alone: establish, purpose-gone, superseded, observability, crash-loop breaker)"
