#!/usr/bin/env bash
set -euo pipefail

# F4/D32 fixtures: the custodian's own-deadline enforcement, driven against
# the REAL runtime-common.sh with a stub dispatch and a real child process.
# The supervisor-crash leg (dead custodian finalized by the standing reaper)
# is proven in Go by internal/supervise/reaper_test.go's core transitions —
# a dead custodian is the reaper's existing case, not new behavior.

source_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
ms_real="${METASYSTEM_BIN:-$source_root/bin/metasystem}"
[[ -x "$ms_real" ]] || { echo "adapter deadline fixtures: binary absent; run the go gate first" >&2; exit 1; }
tmp=$(mktemp -d)
child_pid=
driver_pid=
cleanup() {
  [[ -z "$child_pid" ]] || kill -KILL "$child_pid" 2>/dev/null || true
  [[ -z "$driver_pid" ]] || kill -KILL "$driver_pid" 2>/dev/null || true
  rm -rf "$tmp"
}
trap cleanup EXIT
passed=()
pass_fixture() { passed+=("$1"); echo "$1 passed" >&2; }

cat >"$tmp/stub-dispatch.sh" <<'EOF'
#!/usr/bin/env bash
echo "$@" >>"$STUB_CAS_LOG"
# Simulate a lost CAS (the waiter's verdict landed first): rc 3 from the
# first attempt on. fail_pending/finish_running treat 3 as settled.
[[ -z "${STUB_CAS_LOSES:-}" ]] || exit 3
exit 0
EOF
chmod +x "$tmp/stub-dispatch.sh"

# A pass-through ms wrapper whose `proc group-members` reports a phantom
# member forever: the domain can never be proven dead.
cat >"$tmp/phantom-ms.sh" <<EOF
#!/usr/bin/env bash
if [[ "\$1 \$2" == "proc group-members" ]]; then echo 99999; exit 0; fi
exec "$ms_real" "\$@"
EOF
chmod +x "$tmp/phantom-ms.sh"

cat >"$tmp/drive.sh" <<EOF
#!/usr/bin/env bash
set -euo pipefail
mode=\$1 dir=\$2
export STUB_CAS_LOG="\$dir/cas.log"; : >"\$STUB_CAS_LOG"
job=f4fix; record="\$dir/job.json"; round_dir="\$dir"; log="\$dir/job.log"; heartbeat="\$dir/hb"
ms="\${DRIVER_MS:-$ms_real}"; dispatch="$tmp/stub-dispatch.sh"
requested_model=m; requested_session=; session_id=; effective="\$dir/eff.json"; echo '{}' >"\$effective"
case "\$mode" in
  cap|race|survivor) handshake_done=1; printf '{"jobId":"f4fix","status":"running","capDeadline":"2020-01-01T00:00:00Z"}\n' >"\$record" ;;
  handshake)         handshake_done=0; printf '{"jobId":"f4fix","status":"pending","handshakeDeadline":5}\n' >"\$record" ;;
  standdown)         handshake_done=1; printf '{"jobId":"f4fix","status":"running","handshakeDeadline":5}\n' >"\$record" ;;
esac
source "$source_root/scripts/agents/adapters/runtime-common.sh"
sleep 30 & child=\$!
echo "\$child" >"\$dir/child.pid"
METASYSTEM_HEARTBEAT_INTERVAL_MS=50 wait_for_cli "\$child"
echo done >"\$dir/normal-return"
EOF
chmod +x "$tmp/drive.sh"

run_driver() { # mode, dir, extra env as KEY=VALUE...
  local mode=$1 dir=$2; shift 2
  mkdir -p "$dir"
  # Job control makes the driver a PROCESS-GROUP LEADER, reproducing the
  # production topology (launch-detached): its child joins its group and
  # the own-group-minus-self sweep has a real domain to prove.
  set -m
  env "$@" bash "$tmp/drive.sh" "$mode" "$dir" &
  driver_pid=$!
  set +m
}

wait_driver() { # cap seconds; returns the driver's rc
  local cap=$1 rc
  local deadline=$((SECONDS + cap))
  while kill -0 "$driver_pid" 2>/dev/null; do
    (( SECONDS < deadline )) || { echo "driver did not settle within ${cap}s" >&2; return 99; }
    sleep 0.1
  done
  set +e; wait "$driver_pid"; rc=$?; set -e
  driver_pid=
  return $rc
}

# ADPT-DL-001: an expired cap kills the child and lands running->timeout
# exactly once, and the supervisor's turn ends there.
d="$tmp/cap"; run_driver cap "$d"
wait_driver 15 || { echo "ADPT-DL-001: enforcement did not settle cleanly" >&2; exit 1; }
child_pid=$(cat "$d/child.pid")
kill -0 "$child_pid" 2>/dev/null && { echo "ADPT-DL-001: the child survived cap enforcement" >&2; exit 1; }
child_pid=
[[ ! -f "$d/normal-return" ]] || { echo "ADPT-DL-001: the wait returned instead of ending the turn" >&2; exit 1; }
[[ $(grep -c "expect running --status timeout" "$d/cas.log") -eq 1 ]] \
  || { echo "ADPT-DL-001: expected exactly one running->timeout CAS: $(cat "$d/cas.log")" >&2; exit 1; }
grep -q "cap deadline enforced by the custodian" "$d/job.log" \
  || { echo "ADPT-DL-001: enforcement did not say itself in the job log" >&2; exit 1; }
pass_fixture ADPT-DL-001

# ADPT-DL-002: an expired handshake deadline (no session ever recorded)
# lands pending->failed through the handshake_timeout path.
d="$tmp/handshake"; run_driver handshake "$d"
wait_driver 15 || { echo "ADPT-DL-002: enforcement did not settle cleanly" >&2; exit 1; }
child_pid=$(cat "$d/child.pid")
kill -0 "$child_pid" 2>/dev/null && { echo "ADPT-DL-002: the child survived handshake enforcement" >&2; exit 1; }
child_pid=
[[ $(grep -c "expect pending --status failed" "$d/cas.log") -eq 1 ]] \
  || { echo "ADPT-DL-002: expected exactly one pending->failed CAS: $(cat "$d/cas.log")" >&2; exit 1; }
pass_fixture ADPT-DL-002

# ADPT-DL-003: a won handshake stands down BEFORE any signal — zero CAS,
# zero signals, the wait continues undisturbed.
d="$tmp/standdown"; run_driver standdown "$d"
sleep 1
child_pid=$(cat "$d/child.pid" 2>/dev/null || true)
[[ -n "$child_pid" ]] && kill -0 "$child_pid" 2>/dev/null \
  || { echo "ADPT-DL-003: the child should still be running under a won handshake" >&2; exit 1; }
[[ ! -s "$d/cas.log" ]] || { echo "ADPT-DL-003: a won handshake attempted a CAS: $(cat "$d/cas.log")" >&2; exit 1; }
kill "$child_pid" 2>/dev/null || true; child_pid=
wait_driver 10 || true
[[ -f "$d/normal-return" ]] || { echo "ADPT-DL-003: the wait did not return normally after the child ended" >&2; exit 1; }
pass_fixture ADPT-DL-003

# ADPT-DL-004: when the waiter's verdict already landed, the supervisor's
# CAS loses (rc 3) and the turn still settles with exactly one attempt —
# one record, one verdict, no retry storm.
d="$tmp/race"; run_driver race "$d" STUB_CAS_LOSES=1
wait_driver 15 || { echo "ADPT-DL-004: the lost-CAS path did not settle cleanly" >&2; exit 1; }
child_pid=$(cat "$d/child.pid")
kill -0 "$child_pid" 2>/dev/null && { echo "ADPT-DL-004: the child survived" >&2; exit 1; }
child_pid=
[[ $(wc -l <"$d/cas.log" | tr -d ' ') -eq 1 ]] \
  || { echo "ADPT-DL-004: expected exactly one CAS attempt: $(cat "$d/cas.log")" >&2; exit 1; }
pass_fixture ADPT-DL-004

# ADPT-DL-005: a kill domain that cannot be proven dead (a phantom member
# survives every sweep) leaves the record NONTERMINAL: no CAS, the decline
# said in the log, the wait still standing.
d="$tmp/survivor"; run_driver survivor "$d" DRIVER_MS="$tmp/phantom-ms.sh"
deadline=$((SECONDS + 20))
until grep -q "sweep left the kill domain unproven" "$d/job.log" 2>/dev/null; do
  (( SECONDS < deadline )) || { echo "ADPT-DL-005: the unproven domain was never declared" >&2; exit 1; }
  sleep 0.2
done
[[ ! -s "$d/cas.log" ]] || { echo "ADPT-DL-005: an unproven domain still landed a CAS: $(cat "$d/cas.log")" >&2; exit 1; }
child_pid=$(cat "$d/child.pid" 2>/dev/null || true)
kill -KILL "$driver_pid" 2>/dev/null || true; wait "$driver_pid" 2>/dev/null || true; driver_pid=
[[ -z "$child_pid" ]] || kill -KILL "$child_pid" 2>/dev/null || true; child_pid=
pass_fixture ADPT-DL-005

echo "adapter deadline fixtures passed (${#passed[@]} legs)"
