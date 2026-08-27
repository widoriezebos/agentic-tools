#!/usr/bin/env bash

# Fixture timing policy:
# - expected time is configured through the fixture interval overrides below;
# - every harness cap is at least 10x the expected duration it guards;
# - this file is the only owner of harness cap values; and
# - load scaling is extra headroom, never the reason an idle run passes.

# Fixture configuration edits use one Bash 3.2-compatible AWK path. The
# adjacent temporary file keeps a failed edit away from the source, and the
# original mode is restored before the completed file is renamed into place.
conf_edit() { # file, action, pattern, optional replacement; or file, awk, awk arguments
  local file=$1 action=$2 pattern=${3-} replacement=${4-}
  local staged mode line_count final_byte final_terminated=0 status=0
  if mode=$(stat -f '%Lp' "$file" 2>/dev/null); then
    :
  elif mode=$(stat -c '%a' "$file" 2>/dev/null); then
    :
  else
    echo "conf_edit: cannot read mode for $file" >&2
    return 1
  fi
  line_count=$(awk 'END { print NR + 0 }' "$file") || return 1
  final_byte=$(tail -c 1 "$file"; printf x) || return 1
  [[ "$final_byte" == $'\nx' ]] && final_terminated=1
  staged=$(mktemp "$(dirname "$file")/.conf-edit.XXXXXX") || return 1
  if [[ "$action" == awk ]]; then
    shift 2
    awk -v conf_edit_line_count="$line_count" \
      -v conf_edit_final_terminated="$final_terminated" \
      "$@" "$file" >"$staged" || status=$?
  else
    awk -v action="$action" -v pattern="$pattern" -v replacement="$replacement" \
      -v line_count="$line_count" -v final_terminated="$final_terminated" '
      function replace_literal(text, needle, value,    output, position) {
        if (needle == "") return text
        output = ""
        while ((position = index(text, needle)) != 0) {
          output = output substr(text, 1, position - 1) value
          text = substr(text, position + length(needle))
        }
        return output text
      }
      function emit(text, terminated) {
        printf "%s", text
        if (terminated) printf "%s", ORS
      }
      BEGIN {
        if (action != "replace-line-first" && action != "replace-lines" &&
            action != "delete-line-first" && action != "delete-lines" &&
            action != "insert-after-first" && action != "insert-after" &&
            action != "replace-literal") {
          print "conf_edit: unknown action: " action > "/dev/stderr"
          exit 2
        }
      }
      {
        record_terminated = (FNR < line_count || final_terminated)
        matches = (action == "replace-literal" ? 0 : ($0 ~ pattern))
        if (action == "replace-literal") {
          $0 = replace_literal($0, pattern, replacement)
        } else if (matches && (action == "delete-lines" ||
                   (action == "delete-line-first" && !changed)) &&
                   record_terminated) {
          changed = 1
          next
        } else if (matches && (action == "replace-lines" ||
                   (action == "replace-line-first" && !changed))) {
          changed = 1
          $0 = replacement
        }
        emit($0, record_terminated)
        if (matches && (action == "insert-after" ||
            (action == "insert-after-first" && !changed && record_terminated))) {
          changed = 1
          emit(replacement, 1)
        }
      }
    ' "$file" >"$staged" || status=$?
  fi
  if [[ $status -ne 0 ]]; then
    rm -f "$staged"
    return "$status"
  fi
  if chmod "$mode" "$staged"; then
    :
  else
    status=$?
    rm -f "$staged"
    return "$status"
  fi
  if mv "$staged" "$file"; then
    return 0
  else
    status=$?
    rm -f "$staged"
    return "$status"
  fi
}

harness_fixture_base_cap() { # named harness cap
  local name=$1 base
  case "$name" in
    calibration-census) base=30 ;;
    supervision-wait)
      base=${METASYSTEM_SUPERVISION_FIXTURE_TIMEOUT_SEC:-12}
      [[ "$base" =~ ^[1-9][0-9]*$ && "$base" -ge 12 && "$base" -le 60 ]] \
        || { echo "METASYSTEM_SUPERVISION_FIXTURE_TIMEOUT_SEC must be 12..60" >&2; return 1; }
      ;;
    mission-process-wait) base=5 ;;
    mission-end-state) base=10 ;;
    agent-command)
      base=${METASYSTEM_AGENT_FIXTURE_TIMEOUT_SEC:-20}
      [[ "$base" =~ ^[1-9][0-9]*$ && "$base" -ge 20 && "$base" -le 120 ]] \
        || { echo "METASYSTEM_AGENT_FIXTURE_TIMEOUT_SEC must be an integer from 20 through 120" >&2; return 1; }
      ;;
    agent-status) base=5 ;;
    agent-cleanup) base=7 ;;
    agent-driver-stop) base=2 ;;
    adapter-handshake) base=2 ;;
    runner-git-lock) base=8 ;;
    go-owner-wait) base=8 ;;
    go-owner-crashloop) base=30 ;;
    checkout-execution-guard) base=10 ;;
    *) echo "unknown fixture cap: $name" >&2; return 1 ;;
  esac
  printf '%s\n' "$base"
}

harness_fixture_semantic_cap() { # named product cap used as fixture input
  case "$1" in
    # Dormancy comes from the fixture job's FUTURE startedAt (2099), not
    # from an absurd cap: the cap must stay below any derivable watcher
    # ceiling or the caps interlock rightly refuses to arm over it.
    dormant-job-minutes) printf '120\n' ;;
    mission-job-minutes) printf '5\n' ;;
    mission-turn-minutes) printf '5\n' ;;
    minimum-minutes) printf '1\n' ;;
    dispatch-envelope-minutes) printf '120\n' ;;
    dispatch-over-envelope-minutes) printf '121\n' ;;
    watcher-config-minutes) printf '9\n' ;;
    watcher-nonfiring-minutes) printf '600\n' ;;
    watcher-firing-minutes) printf '5\n' ;;
    *) echo "unknown fixture semantic cap: $1" >&2; return 1 ;;
  esac
}

harness_fixture_milliseconds_to_seconds() { # positive integer milliseconds
  local milliseconds=$1
  [[ "$milliseconds" =~ ^[1-9][0-9]*$ ]] \
    || { echo "fixture interval must be a positive integer in milliseconds: $milliseconds" >&2; return 1; }
  printf '%d.%03d\n' "$((milliseconds / 1000))" "$((milliseconds % 1000))"
}

harness_fixture_budget_init() { # metasystem root
  local harness_root=$1 resolved calibration_cap interval_name interval_value
  calibration_cap=$(harness_fixture_base_cap calibration-census) || return 1
  if [[ -n "${METASYSTEM_FIXTURE_CAP_SCALE:-}" ]]; then
    # The scale must be a plain decimal from 1 through 48 — the same
    # ceiling the automatic probe may resolve, because CHILD harnesses
    # (nested validations, the supervision/mission/fingerprint
    # sub-suites) re-initialize and read the parent's exported scale
    # through this branch: a validator tighter than the probe's range
    # made every automatic scale above the old bound kill its own
    # children (verification round 4, finding 2).
    resolved=$(awk -v s="$METASYSTEM_FIXTURE_CAP_SCALE" 'BEGIN {
      if (s !~ /^[0-9]+(\.[0-9]+)?$/) { print "METASYSTEM_FIXTURE_CAP_SCALE must be a decimal from 1 through 48" > "/dev/stderr"; exit 1 }
      v = s + 0
      if (v < 1 || v > 48) { print "METASYSTEM_FIXTURE_CAP_SCALE must be a decimal from 1 through 48" > "/dev/stderr"; exit 1 }
      m = v * 1000; mi = int(m); if (mi < m) mi++
      printf "%s %d\n", v, mi
    }') || return 1
  else
    # Calibrate by timing one live census scan under the calibration cap, then
    # derive the cap scale from the elapsed milliseconds.
    local ms_bin probe_dir probe_started probe_pid probe_deadline probe_elapsed_ms probe_scale
    ms_bin="${METASYSTEM_BIN:-$harness_root/bin/metasystem}"
    probe_dir=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-fixture-probe.XXXXXX") || return 1
    probe_started=$("$ms_bin" util now-ns)
    "$ms_bin" proc census --repo "$harness_root" --root "$harness_root" \
      --fingerprint fixture-budget-probe --interval 60 \
      --output "$probe_dir/census.json" >/dev/null 2>&1 &
    probe_pid=$!
    probe_deadline=$((SECONDS + calibration_cap))
    while kill -0 "$probe_pid" 2>/dev/null && (( SECONDS < probe_deadline )); do
      sleep 0.05
    done
    if kill -0 "$probe_pid" 2>/dev/null; then
      kill "$probe_pid" 2>/dev/null || true
      wait "$probe_pid" 2>/dev/null || true
      rm -rf "$probe_dir"
      echo "fixture calibration timed out: census probe (scaled cap: ${calibration_cap}s)" >&2
      return 1
    fi
    wait "$probe_pid" 2>/dev/null || true
    rm -rf "$probe_dir"
    probe_elapsed_ms=$(( ($("$ms_bin" util now-ns) - probe_started) / 1000000 ))
    (( probe_elapsed_ms >= 1 )) || probe_elapsed_ms=1
    probe_scale=$(( (probe_elapsed_ms + 249) / 250 ))
    # Floor 8 (was 3): the probe is one-shot, so a calm probe moment
    # followed by later contention starved the caps; a higher floor
    # costs a passing run nothing (waits return on condition, not cap).
    (( probe_scale < 8 )) && probe_scale=8
    # The cap is a HANG DETECTOR, not a machine-speed assertion (the
    # human's ruling, 2026-08-17: a quiet machine must not be a suite
    # requirement). The old ceiling of 12 turned shared-machine
    # contention into fixture timeouts: a one-shot probe calibrated at
    # one moment, then a neighbor workload (another session's JVM, the
    # desktop itself) pushed borderline waits past 12x and two green
    # trees failed at exactly the cap. Waits poll and return the
    # moment their condition holds, so a wider ceiling does not slow a
    # CONVERGING fixture — it bounds how long a genuinely hung one
    # takes to name itself (e.g. the 12s supervision-wait base: old
    # worst 144s, new worst 576s, reached only by a real hang on a
    # maximally-loaded box). The one true cost: NEGATIVE fixtures that
    # deliberately consume a full timeout (no-session-signal's fake
    # handshake) stretch with the floor — seconds, accepted knowingly.
    (( probe_scale > 48 )) && probe_scale=48
    resolved="$probe_scale $((probe_scale * 1000))"
  fi

  read -r METASYSTEM_FIXTURE_CAP_SCALE METASYSTEM_FIXTURE_CAP_SCALE_MILLI <<EOF
$resolved
EOF
  export METASYSTEM_FIXTURE_CAP_SCALE METASYSTEM_FIXTURE_CAP_SCALE_MILLI

  : "${METASYSTEM_FIXTURE_POLL_INTERVAL_MS:=20}"
  : "${METASYSTEM_CENSUS_INTERVAL_MS:=50}"
  : "${METASYSTEM_WATCH_POLL_INTERVAL_MS:=500}"
  : "${METASYSTEM_HEARTBEAT_INTERVAL_MS:=20}"
  : "${METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS:=20}"
  # The drain's reap cadence is decoupled from the heartbeat in production
  # (seconds-scale default); fixtures drain fast jobs and want reaps at
  # heartbeat speed.
  : "${METASYSTEM_DRAIN_REAP_INTERVAL_MS:=$METASYSTEM_HEARTBEAT_INTERVAL_MS}"
  for interval_name in \
      METASYSTEM_FIXTURE_POLL_INTERVAL_MS \
      METASYSTEM_CENSUS_INTERVAL_MS \
      METASYSTEM_WATCH_POLL_INTERVAL_MS \
      METASYSTEM_HEARTBEAT_INTERVAL_MS \
      METASYSTEM_DRAIN_REAP_INTERVAL_MS \
      METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS; do
    interval_value=${!interval_name}
    [[ "$interval_value" =~ ^[1-9][0-9]*$ ]] \
      || { echo "$interval_name must be a positive integer in milliseconds" >&2; return 1; }
  done
  METASYSTEM_FIXTURE_POLL_INTERVAL_SEC=$(
    harness_fixture_milliseconds_to_seconds "$METASYSTEM_FIXTURE_POLL_INTERVAL_MS"
  ) || return 1
  export METASYSTEM_FIXTURE_POLL_INTERVAL_MS METASYSTEM_FIXTURE_POLL_INTERVAL_SEC \
    METASYSTEM_CENSUS_INTERVAL_MS METASYSTEM_WATCH_POLL_INTERVAL_MS \
    METASYSTEM_HEARTBEAT_INTERVAL_MS METASYSTEM_DRAIN_REAP_INTERVAL_MS \
    METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS
}

harness_fixture_scaled_cap() { # positive integer base seconds
  local base=$1
  [[ "$base" =~ ^[1-9][0-9]*$ ]] \
    || { echo "fixture base cap must be a positive integer: $base" >&2; return 1; }
  [[ "${METASYSTEM_FIXTURE_CAP_SCALE_MILLI:-}" =~ ^[1-9][0-9]*$ ]] \
    || { echo "fixture cap scale is not initialized" >&2; return 1; }
  printf '%s\n' "$(( (base * METASYSTEM_FIXTURE_CAP_SCALE_MILLI + 999) / 1000 ))"
}

harness_fixture_cap() { # named harness cap
  local base
  base=$(harness_fixture_base_cap "$1") || return 1
  harness_fixture_scaled_cap "$base"
}
