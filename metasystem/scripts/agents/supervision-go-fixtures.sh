#!/usr/bin/env bash
# Owner-alone fixtures for the GO supervision engine (records/misc/go-migration.md,
# Phase 0 acceptance). These drive the RUNNING BINARY through the design's
# owner-alone Proof rows in scratch checkouts — the acceptance that a unit
# test against fakes cannot give, and that the overnight overclaim skipped.
#
# Non-owner Proof rows (gate, janitor, custody, cohort ledger, registry-lock
# crash cases) are DEFERRED to their phases and named at the end so the set
# is closed, per GO-MIG-R3-001/R2-008.
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
bin=${METASYSTEM_BIN:-$root/bin/metasystem}
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
failure_dir=${METASYSTEM_FIXTURE_FAILURE_DIR:-$root/artifacts/agents/fixture-failures/supervision-go-$(date -u +%Y%m%dT%H%M%SZ)-$$}

preserve_failure_diagnostics() {
  local scenario repo destination source
  mkdir -p "$failure_dir"
  [[ -z "${registry:-}" || ! -f "$registry" ]] || cp "$registry" "$failure_dir/registry.jsonl"
  for scenario in establish purpose purpose.gone superseded breaker; do
    repo="$tmp/$scenario"
    destination="$failure_dir/$scenario"
    for source in \
      "$repo/owner.out" \
      "$repo/artifacts/agents/supervision/owner.ndjson" \
      "$repo/artifacts/agents/supervision/state.json" \
      "$repo/artifacts/agents/supervision/last-census.json"; do
      [[ ! -f "$source" ]] || {
        mkdir -p "$destination"
        cp "$source" "$destination/${source##*/}"
      }
    done
  done
  printf 'go supervision fixture diagnostics retained: %s\n' "$failure_dir" >&2
}

cleanup() {
  local status=$?
  trap - EXIT
  if (( status != 0 )); then
    preserve_failure_diagnostics || true
  fi
  # Kill only what THIS run launched; the tag carries our pid. TERM lets the
  # owner perform its ordinary teardown; KILL is the bounded backstop when the
  # host cannot enumerate enough process identity for that teardown.
  pkill -TERM -f "$fixture_tag-" 2>/dev/null || true
  pkill -KILL -f "$fixture_tag-" 2>/dev/null || true
  rm -rf "$tmp" 2>/dev/null || true
  rm -rf "$tmp"
  exit "$status"
}
trap cleanup EXIT

fail() { echo "go supervision fixture failed: $1" >&2; exit 1; }

# The standing watcher separates repository state from its installation root.
# Supply the smallest real installation its fingerprint and fake-runtime
# signature require, and an authorized empty process universe. The scenario
# repositories below stay independent state/scope roots.
fixture_root="$tmp/fixture-install"
mkdir -p "$fixture_root/bin"
printf '%s\n' 'metasystem.runtimes=fake' 'watch.interval-sec=1' >"$fixture_root/metasystem.conf"
cp "$bin" "$fixture_root/bin/metasystem"
for dependency in \
  scripts/agents/arm-supervision.sh \
  scripts/agents/dispatch.sh \
  scripts/agents/adapters/runtime-common.sh \
  scripts/agents/adapters/fake.sh \
  scripts/watch-background-jobs.sh; do
  mkdir -p "$fixture_root/${dependency%/*}"
  cp "$root/$dependency" "$fixture_root/$dependency"
done
process_fixture="$tmp/processes.json"
printf '[]\n' >"$process_fixture"
export METASYSTEM_CENSUS_PROCESS_FILE="$process_fixture"
watcher_cap=$("$bin" supervise derive-ceiling --conf "$fixture_root/metasystem.conf")

# arm launches the owner the way arm-supervision.sh will: create the lock
# dir, launch the owner with a start gate, publish owner.json naming its
# pid, signal the gate. Echoes the owner pid.
arm() { # repo, tag, registry, interval
  local repo=$1 tag=$2 registry=$3 interval=$4 pid start gate fingerprint
  mkdir -p "$repo/artifacts/agents/supervision/lock.d"
  gate="$repo/artifacts/agents/supervision/start.gate"
  fingerprint=$("$bin" supervise fingerprint --root "$fixture_root" --repo "$repo") \
    || fail "cannot compute owner fingerprint"
  "$bin" supervise owner --repo "$repo" --tag "$tag" --interval "$interval" \
    --metasystem-root "$fixture_root" --scope "$repo" \
    --fingerprint "$fingerprint" --watcher-cap "$watcher_cap" \
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

# state_ready: the state carries the go engine stamp at generation 1 with
# exactly the watcher, reaper and landing-owner components (json strip of all
# three leaving {} plus all present is set equality, no more and no less).
state_ready() { # state file
  local state=$1
  [[ "$("$bin" json get --file "$state" --field engine 2>/dev/null)" == go ]] || return 1
  [[ "$("$bin" json get --file "$state" --field generation 2>/dev/null)" == 1 ]] || return 1
  "$bin" json get --file "$state" --field components.watcher >/dev/null 2>&1 || return 1
  "$bin" json get --file "$state" --field components.reaper >/dev/null 2>&1 || return 1
  "$bin" json get --file "$state" --field components.landing-owner >/dev/null 2>&1 || return 1
  "$bin" json get --file "$state" --field components 2>/dev/null >"$tmp/state-components.json" || return 1
  [[ "$("$bin" json strip --file "$tmp/state-components.json" --key watcher --key reaper --key landing-owner 2>/dev/null)" == '{}' ]] || return 1
}

census_generation_one() { # census file
  [[ "$("$bin" json get --file "$1" --field generation 2>/dev/null)" == 1 ]] || return 1
  [[ "$("$bin" json get --file "$1" --field scanSeq 2>/dev/null)" =~ ^[1-9][0-9]*$ ]] || return 1
  return 0
}

census_generation_one_at_least() { # census file, minimum scan sequence
  local sequence
  census_generation_one "$1" || return 1
  sequence=$("$bin" json get --file "$1" --field scanSeq 2>/dev/null) || return 1
  (( sequence >= $2 )) || return 1
  return 0
}

# last_owner_exit prints the newest registry "exited" event for an owner
# tag; an exited line that fails to parse fails the caller's check.
last_owner_exit() { # registry, ownerTag
  local line owner_tag found=""
  while IFS= read -r line; do
    case "$line" in *'"event":"exited"'*) ;; *) continue ;; esac
    owner_tag=$("$bin" json get --value "$line" --field ownerTag --default "") || return 1
    [[ "$owner_tag" == "$2" ]] && found=$line
  done <"$1"
  [[ -n "$found" ]] && printf '%s\n' "$found"
}

# --- ESTABLISH + PUBLISH: the owner arms, publishes engine-stamped state
#     with its components at one generation, and stays stable (no churn).
repo1="$tmp/establish"; mkdir -p "$repo1"
owner1=$(arm "$repo1" "$fixture_tag-establish" "$registry" 1)
wait_until "$owner_wait" "state published with components" \
  state_ready "$repo1/artifacts/agents/supervision/state.json"
census1="$repo1/artifacts/agents/supervision/last-census.json"
wait_until "$owner_wait" "generation-one census publication" census_generation_one "$census1"
initial_scan_seq=$("$bin" json get --file "$census1" --field scanSeq)
wait_until "$owner_wait" "two later generation-one census publications" \
  census_generation_one_at_least "$census1" "$((initial_scan_seq + 2))"
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
purpose_exit=$(last_owner_exit "$registry" "$fixture_tag-purpose") \
  && [[ "$("$bin" json get --value "$purpose_exit" --field reason)" == purpose-gone ]] \
  && [[ "$("$bin" json get --value "$purpose_exit" --field teardownComplete)" == true ]] \
  || fail "no purpose-gone terminal with complete teardown"
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
super_exit=$(last_owner_exit "$registry" "$fixture_tag-super") \
  && [[ "$("$bin" json get --value "$super_exit" --field reason)" == superseded ]] \
  || fail "no superseded terminal"
echo "go supervision: superseded voluntary exit PASSED" >&2

# --- OBSERVABILITY: the cycle trace narrates the decision basis (the
#     extreme-observability ruling) — the establish owner's log carries the
#     three D-1 reads and a verdict on every line.
trace="$repo1/artifacts/agents/supervision/owner.ndjson"
[[ -s "$trace" ]] || fail "no cycle trace was written"
# Every non-blank line must parse; every verdict-bearing line must also name
# the root and currency reads, and at least one verdict-bearing line exists.
armed_lines=0
while IFS= read -r trace_line; do
  [[ -n "${trace_line//[[:space:]]/}" ]] || continue
  trace_verdict=$("$bin" json get --value "$trace_line" --field verdict --default "") \
    || fail "cycle trace does not narrate the decision basis"
  [[ -n "$trace_verdict" ]] || continue
  [[ -n "$("$bin" json get --value "$trace_line" --field root --default "")" ]] \
    && [[ -n "$("$bin" json get --value "$trace_line" --field currency --default "")" ]] \
    || fail "cycle trace does not narrate the decision basis"
  armed_lines=$((armed_lines + 1))
done <"$trace"
(( armed_lines > 0 )) || fail "cycle trace does not narrate the decision basis"
echo "go supervision: cycle-trace observability PASSED" >&2

# --- CRASH-LOOP BREAKER (D-2, RC-2): components that beat once then die
#     every cycle trip the breaker at N=5; the owner gives up with a
#     complete teardown and an honest terminal — the OTHER half of the
#     KI-32 fix (a self-heal that cannot bound itself is the incident).
repo4="$tmp/breaker"; mkdir -p "$repo4"
printf 'metasystem.runtimes=fake\n' >"$repo4/metasystem.conf"
owner4=$(METASYSTEM_GO_COMPONENT_CRASH_ON_START=1 arm "$repo4" "$fixture_tag-breaker" "$registry" 1)
# At interval 1s and N=5, giving-up lands within ~10s even with backoff
# (backoff gates relaunches, never observations — SLC-R3-005).
wait_until "$owner_crashloop" "owner gives up on the crash loop" bash -c '! kill -0 "$1" 2>/dev/null' _ "$owner4"
breaker_exit=$(last_owner_exit "$registry" "$fixture_tag-breaker") \
  && [[ "$("$bin" json get --value "$breaker_exit" --field reason)" == giving-up ]] \
  && [[ "$("$bin" json get --value "$breaker_exit" --field teardownComplete)" == true ]] \
  || fail "no giving-up terminal with complete teardown"
! pgrep -f "$fixture_tag-breaker" >/dev/null 2>&1 || fail "a component survived giving-up teardown"
echo "go supervision: crash-loop breaker giving-up PASSED" >&2

# DEFERRED Proof rows, named so the acceptance set is closed
# (GO-MIG-R3-001): breaker-at-five and one-clock backoff need a
# fail-on-purpose component (Phase 0 completion); ceiling stop-the-set,
# write-ahead gating, and launched-append retry likewise; the gate,
# janitor, custody, cohort-ledger, and registry-lock-crash rows are
# Phase 0b/1/3 and carry their own fixture files.
echo "go supervision fixtures: PASSED (owner-alone: establish, purpose-gone, superseded, observability, crash-loop breaker)"
