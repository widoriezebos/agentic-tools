#!/usr/bin/env bash
set -euo pipefail

# The production command inventory is a contract: name anything missing
# before arming touches state (go-production-grade Phase 1).
bash "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/preflight-commands.sh" || exit 1

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/agents/arm-supervision.sh --repo <root> [--session <id>]
      [--pid <pid>] [--start-time <epoch>] [--tag <tag>]
      [--rearm] [--max-cap <minutes>]
  scripts/agents/arm-supervision.sh fingerprint --repo <root>

Arming order is fixed: announce the session process; acquire or join the
per-repository supervisor lock; start missing functions; wait for a complete
census; verify watcher, reaper, and census; print ARMED.

An ordinary arm joins a live supervisor and never changes its ceiling.
--rearm replaces the live set after refusing any derived ceiling below a
currently reserved delegate-job cap. --max-cap participates in the config-only
ceiling derivation; the loaded watcher ceiling is the maximum cap plus 30.

When session identity options are omitted, --pid is the immediate
agent-signature ancestor, --start-time is read from the census identity source,
--session is METASYSTEM_SESSION_ID or session-<pid>, and --tag is
METASYSTEM_INSTANCE_TAG or metasystem-main-<runtime>-<sanitized-session>. If no
agent-signature ancestor can be proven, arming refuses.
USAGE
}

die() { echo "$2" >&2; exit "$1"; }

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
script_path=$script_dir/$(basename "${BASH_SOURCE[0]}")
harness_root=$(cd "$script_dir/../.." && pwd -P)
ms="${METASYSTEM_BIN:-$harness_root/bin/metasystem}"
config=$harness_root/scripts/metasystem-config.sh
watcher=$harness_root/scripts/watch-background-jobs.sh
dispatch=$harness_root/scripts/agents/dispatch.sh
agents=$harness_root/artifacts/agents
cap_authority_lock_held=0

now_iso() { date -u +%Y-%m-%dT%H:%M:%SZ; }

# The ceiling derivation lives in `supervise derive-ceiling`
# (script-orchestration-04/D20): the supervision contract's core number is
# the engine's arithmetic now, beside the blocking-reserved-cap fence that
# consumes it.
derive_watcher_ceiling() { # optional declared maximum cap
  "$ms" supervise derive-ceiling --conf "$harness_root/metasystem.conf" ${1:+--max-cap "$1"}
}

blocking_reserved_cap() { # proposed watcher ceiling; prints job|cap for first blocker
  "$ms" supervise blocking-reserved-cap --agents "$agents" --ceiling "$1"
}

supervision_wait_cap() { # base seconds; fixture validation may export a scale
  local base=$1 scale_milli=${METASYSTEM_FIXTURE_CAP_SCALE_MILLI:-1000}
  [[ "$base" =~ ^[1-9][0-9]*$ && "$scale_milli" =~ ^[1-9][0-9]*$ ]] \
    || die 2 "supervision wait cap inputs must be positive integers"
  printf '%s\n' "$(( (base * scale_milli + 999) / 1000 ))"
}

# owner_lock relays the identity-bearing lock verb: 0 done, 3 busy, 4
# not-owner (the dispatch.sh convention). The tag is this script's own name
# — it appears in the armer's argv, which is what the custodian rule probes
# on a live pid. A constant tag means a pid recycled by ANOTHER armer reads
# alive and the lock waits out its deadline instead of healing: fail-closed
# in the rare collision, never a wrong takeover.
owner_lock() { # claim|release, directory, pid, tag
  "$ms" job owner-lock --command "$1" --dir "$2" --pid "$3" --tag "$4"
}

acquire_cap_authority_lock() {
  # Identity-bearing owner lock (script-orchestration-01/D18): the old bare
  # mkdir spinlock had no owner and no healer, so a SIGKILLed armer bricked
  # every dispatch AND arming until a human ran rmdir.
  local directory="$agents/supervision/cap-authority.lock.d" maximum started deadline elapsed
  mkdir -p "${directory%/*}"
  maximum=$(supervision_wait_cap 10)
  started=$SECONDS
  deadline=$((SECONDS + maximum))
  while ! owner_lock claim "$directory" "$$" "arm-supervision.sh"; do
    if (( SECONDS >= deadline )); then
      elapsed=$((SECONDS - started))
      die 1 "timed out acquiring repository cap-authority lock (elapsed: ${elapsed}s; scaled cap: ${maximum}s)"
    fi
    sleep 0.05
  done
  cap_authority_lock_held=1
}

release_cap_authority_lock() {
  (( cap_authority_lock_held )) || return 0
  local status=0
  owner_lock release "$agents/supervision/cap-authority.lock.d" "$$" "arm-supervision.sh" || status=$?
  (( status == 4 )) && die 1 "refusing to release another owner's cap-authority lock"
  cap_authority_lock_held=0
}

milliseconds_to_sleep() { # positive integer milliseconds
  local milliseconds=$1
  [[ "$milliseconds" =~ ^[1-9][0-9]*$ ]] \
    || die 2 "supervision interval must be a positive integer in milliseconds"
  printf '%d.%03d\n' "$((milliseconds / 1000))" "$((milliseconds % 1000))"
}

resolve_repo() {
  local supplied=$1 top
  top=$(git -C "$supplied" rev-parse --show-toplevel 2>/dev/null) \
    || die 2 "--repo is not inside a git repository: $supplied"
  (cd "$top" && pwd -P)
}

sanitize() { # value
  "$ms" util slug "$1"
}

json_field() { # file, dotted field
  "$ms" json get --file "$1" --field "$2"
}

identity_alive() { # pid, start, optional tag, optional start-ticks, optional boot-id
  local pid=$1 start=$2 tag=${3:-} ticks=${4:-} boot=${5:-} command pair_args=()
  # The clock-step-immune pair (issue #1 sweep 3): when present, proc alive
  # compares ticks+bootId, so a btime step on a time-synced guest cannot read
  # this live process as dead (KI-37). Absent (owner-lock reads, darwin) it is
  # the seconds comparison, unchanged.
  [[ -n "$ticks" && "$ticks" != 0 && -n "$boot" ]] && pair_args=(--start-ticks "$ticks" --boot-id "$boot")
  "$ms" proc alive --pid "$pid" --start-time "$start" "${pair_args[@]+"${pair_args[@]}"}" --root "$harness_root" >/dev/null 2>&1 || return 1
  [[ -z "$tag" ]] && return 0
  # Through the engine's one identity source (script-fixtures-007/D47):
  # the raw ps read here bypassed the fixture table every other reader
  # honors. live = tag proven on the argv; unknown = argv not observable,
  # conservatively live — inability to inspect a tag is never proof that
  # permits takeover or signalling; stale or dead = not this identity.
  local verdict
  verdict=$("$ms" proc classify --pid "$pid" --tag "$tag" 2>/dev/null || echo unknown)
  case "$verdict" in live|unknown) return 0 ;; *) return 1 ;; esac
}

atomic_json_identity() { # path, pid, start, tag, acquired-at
  "$ms" supervise write-owner-identity --path "$1" --pid "$2" --start "$3" --tag "$4" --acquired-at "$5"
}

# An adopted or fixture copy may lack the emitter; a failed `source` under
# set -e kills the script before || can catch it, so test first.
if [[ -f "$(dirname "${BASH_SOURCE[0]}")/emit-event.sh" ]]; then
  source "$(dirname "${BASH_SOURCE[0]}")/emit-event.sh"
else
  emit_event() { :; }
fi

rotate_event_stream() { # harness root -- only on the ESTABLISHING path (D-4)
  local stream="$1/artifacts/agents/events.jsonl" archive_dir="$1/artifacts/agents/events-archive"
  [[ -s "$stream" ]] || return 0
  mkdir -p "$archive_dir"
  local stamp name n=1
  stamp=$(date -u +%Y%m%dT%H%M%SZ)
  name="$archive_dir/events-$stamp-$$.jsonl"
  while [[ -e "$name" ]]; do n=$((n+1)); name="$archive_dir/events-$stamp-$$-$n.jsonl"; done
  mv "$stream" "$name" 2>/dev/null || return 0
  METASYSTEM_HARNESS_ROOT="$1" emit_event arming stream-rotated "previousPath=${name#"$1"/}" "summary=rotated at arming"
}

write_announcement() { # repo, session, pid, start, tag, runtime, optional lineage, optional ticks, optional boot
  if [[ -n "${7:-}" ]]; then
    "$ms" lease announce --root "$1" --session "$2" --pid "$3" \
      --start "$4" --tag "$5" --runtime "$6" \
      $( [[ -n "${8:-}" && "${8:-}" != 0 && -n "${9:-}" ]] && printf -- '--start-ticks %s --boot-id %s' "$8" "$9" ) --owner-lineage "$7" \
      $( [[ -n "${8:-}" && "${8:-}" != 0 && -n "${9:-}" ]] && printf -- '--start-ticks %s --boot-id %s' "$8" "$9" )
  else
    "$ms" lease announce --root "$1" --session "$2" --pid "$3" \
      --start "$4" --tag "$5" --runtime "$6" \
      $( [[ -n "${8:-}" && "${8:-}" != 0 && -n "${9:-}" ]] && printf -- '--start-ticks %s --boot-id %s' "$8" "$9" )
  fi
}

retire_announcement() { # repo, session, pid, start
  "$ms" lease retire --root "$1" --session "$2" --pid "$3" --start "$4"
}

stop_identity() { # name, pid, start, tag
  local name=$1 pid=$2 start=$3 tag=$4 cap started deadline elapsed kill_cap
  identity_alive "$pid" "$start" "$tag" || return 0
  kill -TERM -- "-$pid" 2>/dev/null || kill -TERM "$pid" 2>/dev/null || true
  cap=$(supervision_wait_cap 5)
  started=$SECONDS
  deadline=$((SECONDS + cap))
  while identity_alive "$pid" "$start" "$tag"; do
    if (( SECONDS >= deadline )); then
      elapsed=$((SECONDS - started))
      echo "supervision stop ceiling reached: $name pid=$pid (elapsed: ${elapsed}s; scaled cap: ${cap}s); sending KILL" >&2
      kill -KILL -- "-$pid" 2>/dev/null || kill -KILL "$pid" 2>/dev/null || true
      kill_cap=$(supervision_wait_cap 1)
      deadline=$((SECONDS + kill_cap))
      while identity_alive "$pid" "$start" "$tag" && (( SECONDS < deadline )); do sleep 0.05; done
      identity_alive "$pid" "$start" "$tag" && return 1
      wait "$pid" 2>/dev/null || true
      return 0
    fi
    sleep 0.05
  done
  wait "$pid" 2>/dev/null || true
}

launch_detached() { # output pid variable name, log path, command...
  local __name=$1 log=$2 detached_pid
  shift 2
  detached_pid=$("$ms" supervise launch-detached --log "$log" -- "$@") || return 1
  printf -v "$__name" '%s' "$detached_pid"
}

wait_for_start_identity() { # name, pid
  local name=$1 pid=$2 cap started deadline elapsed value
  cap=$(supervision_wait_cap 5)
  started=$SECONDS
  deadline=$((SECONDS + cap))
  while (( SECONDS < deadline )); do
    if value=$("$ms" proc started-at --pid "$pid" 2>/dev/null); then printf '%s\n' "$value"; return 0; fi
    sleep 0.02
  done
  elapsed=$((SECONDS - started))
  echo "supervision start identity ceiling reached: $name pid=$pid (elapsed: ${elapsed}s; scaled cap: ${cap}s)" >&2
  return 1
}

# A launched component is not yet a supervising one. The staleness rule reads
# heartbeats, and a component that has not written its first heartbeat looks
# exactly like one that stopped writing them -- so a launch that returns at
# spawn lets the supervisor declare its own newborn component stale and replace
# it. Every replacement stops the running set, and each of those gaps is a
# moment with no census writer at all. A launch therefore completes only once
# both components have heartbeat under the identity it just started.
#
# Waiting for a heartbeat is not waiting for a live process, so this also stops
# the moment the component dies: a process killed before its first heartbeat
# never produces one, and blocking for the full ceiling on it would strand the
# supervisor exactly when it is needed to replace the set.
wait_for_first_heartbeat() { # name, heartbeat file, instance tag, pid, start
  local name=$1 file=$2 tag=$3 pid=$4 start=$5 cap started deadline elapsed
  cap=$(supervision_wait_cap 5)
  started=$SECONDS
  deadline=$((SECONDS + cap))
  while (( SECONDS < deadline )); do
    [[ "$(json_field "$file" instanceTag 2>/dev/null || true)" == "$tag" ]] && return 0
    if ! identity_alive "$pid" "$start" "$tag"; then
      echo "supervision component died before its first heartbeat: $name pid=$pid" >&2
      return 1
    fi
    sleep 0.02
  done
  elapsed=$((SECONDS - started))
  echo "supervision first-heartbeat ceiling reached: $name (elapsed: ${elapsed}s; scaled cap: ${cap}s)" >&2
  return 1
}

read_component_identity() { # state, component => pid start tag
  "$ms" supervise component-identity --state "$1" --component "$2"
}

stop_recorded_components() {
  local state=$1/artifacts/agents/supervision/state.json component pid start tag
  [[ -f "$state" ]] || return 0
  for component in watcher reaper; do
    # The read itself must be guarded (script-orchestration-11): on a
    # corrupt or partial state the substitution yields nothing, read
    # returns 1 on EOF, and errexit killed the whole takeover with zero
    # diagnostics — exactly where recovery matters most.
    read -r pid start tag < <(read_component_identity "$state" "$component" 2>/dev/null || true) || true
    [[ -n "${pid:-}" ]] && stop_identity "$component" "$pid" "$start" "$tag" || true
    pid=; start=; tag=
  done
}


verify_armed() { # repo, owner pid/start/tag
  # One attempt's verdict is the engine's (`supervise verify-armed`,
  # script-orchestration-10/D21): the same freshness judgment dispatch's
  # census gate renders, computed once. This script keeps the retry loop
  # and the timeout message.
  local repo=$1 owner_pid=$2 owner_start=$3 owner_tag=$4 interval cap started deadline elapsed
  interval=$("$config" get --key watch.interval-sec --default 60)
  cap=$(supervision_wait_cap "$((interval + 10))")
  started=$SECONDS
  deadline=$((SECONDS + cap))
  while (( SECONDS < deadline )); do
    if "$ms" supervise verify-armed --agents "$agents" --owner-pid "$owner_pid" \
        --owner-start "$owner_start" --owner-tag "$owner_tag" --interval "$interval"; then
      return 0
    fi
    sleep 0.05
  done
  elapsed=$((SECONDS - started))
  echo "supervision arming timed out: first complete census (elapsed: ${elapsed}s; scaled cap: ${cap}s): watcher, reaper, and a fresh successful census did not verify" >&2
  return 1
}

arm_repository() {
  # Supervision and everything arming does NEVER carry the driver's execution
  # id: a joined watcher cannot receive one, so none may depend on timing --
  # and the lease events emitted DURING arming must not leak it (FRCC-006).
  unset METASYSTEM_EXECUTION_ID
  local repo= session= pid= start= tag= runtime=${METASYSTEM_AGENT_RUNTIME:-} retire=0 shutdown=0 lease_held=0 rearm=0 max_cap= watcher_cap= blocker= ancestor safe announcement
  local owner_lineage=${METASYSTEM_OWNER_LINEAGE:-}
  local owner_cap owner_started owner_deadline elapsed expected_owner_prefix
  while (($#)); do
    case "$1" in
      --repo) [[ $# -ge 2 ]] || { usage; exit 2; }; repo=$2; shift 2 ;;
      --session) [[ $# -ge 2 ]] || { usage; exit 2; }; session=$2; shift 2 ;;
      --pid) [[ $# -ge 2 ]] || { usage; exit 2; }; pid=$2; shift 2 ;;
      --start-time) [[ $# -ge 2 ]] || { usage; exit 2; }; start=$2; shift 2 ;;
      --tag) [[ $# -ge 2 ]] || { usage; exit 2; }; tag=$2; shift 2 ;;
      --retire) retire=1; shift ;; --shutdown) shutdown=1; shift ;;
      --lease-held) lease_held=1; shift ;;
      --owner-lineage) [[ $# -ge 2 ]] || { usage; exit 2; }; owner_lineage=$2; shift 2 ;;
      --rearm) rearm=1; shift ;;
      --max-cap) [[ $# -ge 2 ]] || { usage; exit 2; }; max_cap=$2; shift 2 ;;
      -h|--help) usage; exit 0 ;; *) usage; exit 2 ;;
    esac
  done
  [[ -n "$repo" ]] || { usage; exit 2; }
  repo=$(resolve_repo "$repo")
  [[ -x "$ms" ]] \
    || die 1 "metasystem binary is not executable"
  if (( shutdown )); then
    if (( ! lease_held )); then
      lease_result=$("$ms" lease require-holder --root "$harness_root" --caller-pid "$$") || exit $?
      lease_epoch=$("$ms" json get --value "$lease_result" --field claimEpoch --default "")
      if [[ -n "$lease_epoch" ]]; then
        exec "$ms" lease run-held --root "$harness_root" --caller-pid "$$" \
          --expected-epoch "$lease_epoch" -- "$script_path" --repo "$repo" --shutdown --lease-held
      fi
      exec "$ms" lease run-held --root "$harness_root" --caller-pid "$$" -- \
        "$script_path" --repo "$repo" --shutdown --lease-held
    fi
    "$ms" lease require-holder --root "$harness_root" --caller-pid "$$" >/dev/null
    lock=$agents/supervision/lock.d/owner.json
    [[ -f "$lock" ]] || exit 0
    owner_pid=$(json_field "$lock" pid); owner_start=$(json_field "$lock" pidStartedAt); owner_tag=$(json_field "$lock" instanceTag)
    # The tag carries the repository the owner was armed for. A record copied
    # from another checkout names a live process this repository does not own,
    # so shutting it down would kill a stranger's supervisor: refuse instead.
    expected_owner_prefix="metasystem-supervision-owner-$(sanitize "$(git -C "$harness_root" rev-parse --show-toplevel 2>/dev/null || true)")-"
    [[ "$owner_tag" == "$expected_owner_prefix"* ]] \
      || die 1 "supervision lock names an owner armed for another repository ($owner_tag); refusing to stop a process this repository does not own"
    stop_identity owner "$owner_pid" "$owner_start" "$owner_tag"
    # The stopped owner's lock is a husk now: this shutdown proved the exact
    # identity dead, so retiring the lock is safe and leaves the checkout
    # cleanly disarmed rather than parked on a provably-dead record.
    if [[ "$(json_field "$lock" pid 2>/dev/null || true)" == "$owner_pid" ]]; then
      rm -f "$lock"
      rmdir "$agents/supervision/lock.d" 2>/dev/null || true
    fi
    exit 0
  fi
  if [[ -z "$pid" ]]; then
    if ! ancestor=$("$ms" proc find-ancestor --repo "$repo" --pid "$PPID" ${runtime:+--runtime "$runtime"} 2>&1); then
      die 1 "cannot infer arming identity: no agent-signature ancestor was proven. Pass --pid <agent-pid> and --start-time <epoch-seconds>, or run from a session whose ancestor matches a configured runtime signature. Detail: $ancestor"
    fi
    pid=$("$ms" json get --value "$ancestor" --field pid)
    [[ -n "$runtime" ]] || runtime=$("$ms" json get --value "$ancestor" --field runtime)
  fi
  [[ "$pid" =~ ^[1-9][0-9]*$ ]] || die 2 "--pid must be a positive integer"
  # ONE read of the live pid gives the second AND the clock-step-immune pair,
  # so no btime step can land between reading the start and verifying it
  # (KI-37 / issue #1 sweep 3). A supplied --start-time is kept as the recorded
  # second; the pair is always this fresh probe's.
  pair_line=$("$ms" proc started-at --emit pair --pid "$pid") || die 1 "cannot read pid start identity"
  read -r probe_start start_ticks boot_id <<<"$pair_line"
  [[ "$boot_id" == "-" ]] && boot_id=""
  start=${start:-$probe_start}
  [[ "$start" =~ ^[1-9][0-9]*$ ]] || die 2 "--start-time must be epoch seconds"
  identity_alive "$pid" "$start" "" "$start_ticks" "$boot_id" || die 1 "announcement pid identity is not live"
  session=${session:-${METASYSTEM_SESSION_ID:-session-$pid}}
  runtime=${runtime:-unknown}
  safe=$(sanitize "$session")
  tag=${tag:-${METASYSTEM_INSTANCE_TAG:-metasystem-main-$runtime-$safe}}
  [[ -n "$tag" ]] || die 2 "--tag cannot be empty"
  if (( retire )); then retire_announcement "$harness_root" "$session" "$pid" "$start"; exit 0; fi
  watcher_cap=$(derive_watcher_ceiling "$max_cap")
  # The owner publishes state.json from what it is handed: the config
  # interval, the supervision fingerprint, and the derived cap. Arming
  # verifies exactly these against the watcher's census and heartbeat, so
  # they must travel to the owner rather than defaulting inside it.
  watch_interval=$("$config" get --key watch.interval-sec --default 60)
  [[ "$watch_interval" =~ ^[1-9][0-9]*$ ]] || die 1 "watch.interval-sec must be a positive integer"
  armed_fingerprint=$("$ms" supervise fingerprint --root "$harness_root" --repo "$repo") \
    || die 1 "could not compute the supervision fingerprint"

  # Fixed arming step 1: registry write precedes lock acquisition and census.
  announcement=$(write_announcement "$harness_root" "$session" "$pid" "$start" "$tag" "$runtime" "$owner_lineage" "$start_ticks" "$boot_id")
  "$ms" lease require-holder --root "$harness_root" --caller-pid "$pid" >/dev/null
  supervision=$agents/supervision
  mkdir -p "$supervision"
  printf '%s announcement-written registry=%s pid=%s start=%s\n' "$(now_iso)" "$announcement" "$pid" "$start" >>"$supervision/arming.log"

  lock_dir=$supervision/lock.d; owner_file=$lock_dir/owner.json
  owner_tag="metasystem-supervision-owner-$(sanitize "$(git -C "$repo" rev-parse --show-toplevel)")-$(date +%s)-$$"
  trap 'release_cap_authority_lock' EXIT
  acquire_cap_authority_lock
  if (( rearm )); then
    blocker=$(blocking_reserved_cap "$watcher_cap")
    if [[ -n "$blocker" ]]; then
      IFS='|' read -r blocking_job blocking_cap <<<"$blocker"
      die 1 "supervision re-arm refused: derived ${watcher_cap}m ceiling is not strictly above reserved cap ${blocking_cap}m for job $blocking_job"
    fi
  fi
  if (( rearm )) && [[ -f "$owner_file" ]]; then
    owner_pid=$(json_field "$owner_file" pid) || die 1 "supervision lock owner is malformed"
    owner_start=$(json_field "$owner_file" pidStartedAt) || die 1 "supervision lock owner is malformed"
    existing_tag=$(json_field "$owner_file" instanceTag) || die 1 "supervision lock owner is malformed"
    identity_alive "$owner_pid" "$owner_start" "$existing_tag" \
      || die 1 "supervision re-arm refused: existing owner identity is not live"
    stop_identity owner "$owner_pid" "$owner_start" "$existing_tag" \
      || die 1 "supervision re-arm refused: existing owner did not stop; replacement was not established"
  fi
  if mkdir "$lock_dir" 2>/dev/null; then
    blocker=$(blocking_reserved_cap "$watcher_cap")
    if [[ -n "$blocker" ]]; then
      IFS='|' read -r blocking_job blocking_cap <<<"$blocker"
      rmdir "$lock_dir" 2>/dev/null || true
      die 1 "supervision establishment refused: derived ${watcher_cap}m ceiling is not strictly above reserved cap ${blocking_cap}m for job $blocking_job"
    fi
    # The process is launched only after this arming call owns the repository
    # lock. This preserves the fixed order and avoids speculative supervisors.
    gate=$supervision/owner-gate.$$.$RANDOM
    launch_detached candidate_pid "$supervision/owner.log" "$ms" supervise owner --repo "$harness_root" --scope "$repo" --gate "$gate" --tag "$owner_tag" --interval "$watch_interval" --fingerprint "$armed_fingerprint" --watcher-cap "$watcher_cap"
    candidate_start=$(wait_for_start_identity owner-candidate "$candidate_pid") || {
      rmdir "$lock_dir" 2>/dev/null || true
      die 1 "could not start supervision owner"
    }
    atomic_json_identity "$owner_file" "$candidate_pid" "$candidate_start" "$owner_tag" "$(now_iso)"
    touch "$gate"
    owner_pid=$candidate_pid; owner_start=$candidate_start
  else
    # mkdir and owner.json cannot be one filesystem operation. Give the lock
    # winner a bounded publication window; an ownerless lock still refuses
    # because no process identity exists to prove dead.
    owner_cap=$(supervision_wait_cap 5)
    owner_started=$SECONDS
    owner_deadline=$((SECONDS + owner_cap))
    while [[ ! -f "$owner_file" && $SECONDS -lt $owner_deadline ]]; do sleep 0.02; done
    if [[ ! -f "$owner_file" ]]; then
      elapsed=$((SECONDS - owner_started))
      die 1 "supervision lock join timed out: owner identity (elapsed: ${elapsed}s; scaled cap: ${owner_cap}s); refusing unproven takeover"
    fi
    owner_pid=$(json_field "$owner_file" pid) || die 1 "supervision lock owner is malformed"
    owner_start=$(json_field "$owner_file" pidStartedAt) || die 1 "supervision lock owner is malformed"
    existing_tag=$(json_field "$owner_file" instanceTag) || die 1 "supervision lock owner is malformed"
    if identity_alive "$owner_pid" "$owner_start" "$existing_tag"; then
      (( ! rearm )) \
        || die 1 "supervision re-arm refused: another live owner won replacement; refusing to join it"
      owner_tag=$existing_tag
    else
      # Takeover is legal only after exact pid+start identity is proven dead.
      blocker=$(blocking_reserved_cap "$watcher_cap")
      if [[ -n "$blocker" ]]; then
        IFS='|' read -r blocking_job blocking_cap <<<"$blocker"
        die 1 "supervision takeover refused: derived ${watcher_cap}m ceiling is not strictly above reserved cap ${blocking_cap}m for job $blocking_job"
      fi
      stop_recorded_components "$harness_root"
      rm "$owner_file"
      rmdir "$lock_dir" || die 1 "supervision lock takeover lost a race"
      mkdir "$lock_dir" || die 1 "supervision lock takeover lost a race"
      gate=$supervision/owner-gate.$$.$RANDOM
      launch_detached candidate_pid "$supervision/owner.log" "$ms" supervise owner --repo "$harness_root" --scope "$repo" --gate "$gate" --tag "$owner_tag" --interval "$watch_interval" --fingerprint "$armed_fingerprint" --watcher-cap "$watcher_cap"
      candidate_start=$(wait_for_start_identity takeover-owner "$candidate_pid") || {
        rmdir "$lock_dir" 2>/dev/null || true
        die 1 "could not start takeover owner"
      }
      atomic_json_identity "$owner_file" "$candidate_pid" "$candidate_start" "$owner_tag" "$(now_iso)"
      touch "$gate"
      owner_pid=$candidate_pid; owner_start=$candidate_start
    fi
  fi
  verify_armed "$repo" "$owner_pid" "$owner_start" "$owner_tag" || exit 1
  # The arming log proves the fixed order: the announcement precedes the
  # first verified census, so an arming session can never census itself
  # as UNTRACKED.
  printf '%s first-census-complete repo=%s owner=%s\n' "$(now_iso)" "$repo" "$owner_pid" >>"$supervision/arming.log"
  release_cap_authority_lock
  trap - EXIT
  printf '%s ARMED repo=%s owner=%s start=%s tag=%s announcement=%s\n' "$(now_iso)" "$repo" "$owner_pid" "$owner_start" "$owner_tag" "$announcement"
}

case ${1:-} in
  __owner) echo "the supervision owner runs as 'metasystem supervise owner'; arming launches it directly" >&2; exit 2 ;;
  fingerprint)
    shift
    [[ ${1:-} == --repo && $# -eq 2 ]] || { usage; exit 2; }
    repo=$(resolve_repo "$2")
    "$ms" supervise fingerprint --root "$harness_root" --repo "$repo"
    ;;
  *) arm_repository "$@" ;;
esac
