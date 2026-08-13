#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/agents/adapters/fake.sh identity
  scripts/agents/adapters/fake.sh config-identity
  scripts/agents/adapters/fake.sh signature
  scripts/agents/adapters/fake.sh probe [--profile current|old|unverified-network]
      [--age-days N]
  scripts/agents/adapters/fake.sh dispatch --job <job-id> --start-gate <file>
      --instance-tag <tag>
  scripts/agents/adapters/fake.sh follow-up --job <job-id> --start-gate <file>
      --instance-tag <tag>
  scripts/agents/adapters/fake.sh cancel --job <job-id>
  scripts/agents/adapters/fake.sh selftest
  scripts/agents/adapters/fake.sh local-config-paths

The simulator reads FAKE:<behavior> markers from the assembled prompt.
Supported behaviors include malformed-return, missing-session-id,
resume-collision, concurrent-turn, cancel-race, process-loss, timeout,
no-session-signal, handshake-failure, no-event-stream, hook-unavailable,
interrupted-atomic-write, nested-agent-events, effective-wider,
effective-narrower, and mirror-failure. A Fake-Argument: line is captured as
data and never executed.
USAGE
}

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd -P)
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"
dispatch="$root/scripts/agents/dispatch.sh"
agents="$root/artifacts/agents"
jobs="$agents/jobs"

field() { # json file, dotted field
  "$ms" json get --file "$1" --field "$2"
}

parse_supervisor_args() {
  job= gate= instance_tag=
  while (($#)); do
    case "$1" in
      --job) [[ $# -ge 2 ]] || { usage; exit 2; }; job=$2; shift 2 ;;
      --start-gate) [[ $# -ge 2 ]] || { usage; exit 2; }; gate=$2; shift 2 ;;
      --instance-tag) [[ $# -ge 2 ]] || { usage; exit 2; }; instance_tag=$2; shift 2 ;;
      *) usage; exit 2 ;;
    esac
  done
  [[ -n "$job" && -n "$gate" && -n "$instance_tag" ]] || { usage; exit 2; }
}

behavior_present() { grep -Fqi "FAKE:$1" "$prompt"; }

fake_guarded_write() { # permissions JSON, target path
  "$ms" adapter fake-guarded-write --permissions "$1" --target "$2"
}

fake_guarded_network_call() { # permissions JSON, host, port
  "$ms" adapter fake-guarded-network --permissions "$1" --host "$2" --port "$3"
}

probe_fake_envelope_mechanism() {
  local probe_dir permissions target result write_status network_status
  probe_dir=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-fake-envelope-probe.XXXXXX")
  permissions="$probe_dir/permissions.json"
  target="$probe_dir/denied-write.txt"
  result=${METASYSTEM_FAKE_ENVELOPE_PROBE_RESULT:-}
  printf '{"readRoots":[],"writeRoots":[],"network":"deny"}\n' >"$permissions"
  set +e
  fake_guarded_write "$permissions" "$target"
  write_status=$?
  fake_guarded_network_call "$permissions" 127.0.0.1 9
  network_status=$?
  set -e
  if [[ $write_status -ne 77 || $network_status -ne 77 || -e "$target" ]]; then
    echo "fake envelope mechanism did not refuse a denied write and network call" >&2
    rm -rf "$probe_dir"
    return 1
  fi
  if [[ -n "$result" ]]; then
    printf '{"network":{"exitStatus":%d,"observed":"denied"},"writeRoots":{"exitStatus":%d,"observed":"denied"}}\n' \
      "$network_status" "$write_status" >"$result"
  fi
  rm -rf "$probe_dir"
}

fixture_milliseconds_to_sleep() { # positive integer milliseconds
  local milliseconds=$1
  [[ "$milliseconds" =~ ^[1-9][0-9]*$ ]] \
    || { echo "fake adapter interval must be a positive integer in milliseconds" >&2; return 2; }
  printf '%d.%03d\n' "$((milliseconds / 1000))" "$((milliseconds % 1000))"
}

cas_terminal() { # target, error, phase
  local target=$1 error=$2 phase=$3 patch usage
  patch="$round_dir/terminal-patch.json"
  usage="$round_dir/fake-usage.json"
  "$ms" adapter fake-usage --output "$usage"
  "$ms" adapter result-patch --output "$patch" --error "$error" --phase "$phase" --usage "$usage"
  "$dispatch" __record-cas --job "$job" --expect running --status "$target" --patch "$patch" || {
    status=$?
    [[ $status -eq 3 ]] || return "$status"
  }
}

write_valid_return() {
  "$ms" adapter fake-return --record "$record" --prompt "$prompt" \
    --output "$round_dir/return.json"
  printf '# Fake return\n\nCanonical JSON: return.json\n' >"$round_dir/return.md"
}

complete_valid() {
  local violation="$round_dir/protocol-violation.txt"
  write_valid_return
  if "$root/scripts/assert-return-complete.sh" --job "$job" >"$violation" 2>&1; then
    rm -f "$violation"
    cas_terminal completed null completed
  else
    cat "$violation" >>"$log"
    "$dispatch" __protocol-error --job "$job" --expect running --violation-file "$violation"
  fi
}

supervise() { # verb and remaining args
  local verb=$1 gate_poll heartbeat_sleep; shift
  parse_supervisor_args "$@"
  record="$jobs/$job.json"
  gate_poll=$(fixture_milliseconds_to_sleep "${METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS:-10}")
  heartbeat_sleep=$(fixture_milliseconds_to_sleep "${METASYSTEM_HEARTBEAT_INTERVAL_MS:-200}")
  gate_deadline=$(( $(date +%s) + ${METASYSTEM_HOST_START_GATE_TIMEOUT_SEC:-10} ))
  while [[ ! -e "$gate" ]]; do
    (( $(date +%s) <= gate_deadline )) || { echo "start gate never opened: $gate" >&2; exit 1; }
    sleep "$gate_poll"
  done
  round=$(field "$record" round)
  root_job=$("$ms" adapter root-job --jobs "$jobs" --job "$job")
  round_dir="$agents/$root_job/rounds/$round"
  prompt="$round_dir/prompt.md"
  log="$jobs/$job.log"
  raw="$round_dir/raw.out"
  events="$round_dir/events.jsonl"
  heartbeat="$agents/hb/$job"
  mkdir -p "$round_dir" "$(dirname "$heartbeat")"
  printf 'fake supervisor started value=%s\n' "$instance_tag" >"$log"
  printf 'fake raw output\n' >"$raw"
  printf '{"pid":%s,"pgid":%s,"instanceTag":"%s"}\n' "$$" "$$" "$instance_tag" >"$heartbeat"

  if behavior_present pending-process-loss; then
    printf '{"lost":true}\n' >"$heartbeat"
    kill -KILL "$$"
  fi

  if behavior_present no-session-signal; then
    printf 'ordinary output without a session-established event\n' >>"$log"
    "$ms" util hold --tag "$instance_tag" &
    wait
  fi
  if behavior_present handshake-failure; then
    patch="$round_dir/handshake-failure.json"
    printf '{"error":"authentication_failed","phase":"handshake"}\n' >"$patch"
    "$dispatch" __record-cas --job "$job" --expect pending --status failed --patch "$patch" || true
    exit 1
  fi

  effective="$round_dir/effective-permissions.json"
  "$ms" adapter effective-init --record "$record" --output "$effective"
  if behavior_present effective-wider; then
    "$ms" adapter fake-effective-network --effective "$effective" --network allow
  fi
  if behavior_present effective-narrower; then
    "$ms" adapter fake-effective-network --effective "$effective" --network deny
  fi
  session="fake-session-$root_job"
  [[ "$verb" == dispatch && "$round" -gt 1 ]] && session="fake-session-$root_job-fresh-$round"
  [[ "$verb" == follow-up ]] && session=$(field "$record" sessionId)
  behavior_present missing-session-id && session=
  signal=$(field "$record" sessionEstablishedSignal)
  "$dispatch" __handshake --job "$job" --session "$session" --turn "fake-turn-$round" \
    --model "$(field "$record" requestedModel)" --effective "$effective" --signal "$signal" || exit 1
  if ! behavior_present no-event-stream; then
    printf '{"event":"session-established","sessionId":"%s","round":%s}\n' "$session" "$round" >>"$events"
  fi

  if [[ "$verb" == follow-up ]]; then
    expected="fake-session-$root_job"
    [[ "$session" == "$expected" ]] || { cas_terminal failed resume_collision resume; exit 1; }
  fi
  if behavior_present resume-collision; then cas_terminal failed resume_collision resume; exit 1; fi
  if behavior_present process-loss; then
    "$ms" util hold --tag "$instance_tag" --stopped-file "$round_dir/child.stopped" &
    printf '%s\n' "$!" >"$round_dir/child.pid"
    printf '{"lost":true}\n' >"$heartbeat"
    kill -KILL "$$"
  fi
  if behavior_present timeout || behavior_present concurrent-turn; then
    "$ms" util hold --tag "$instance_tag" --stopped-file "$round_dir/child.stopped" &
    printf '%s\n' "$!" >"$round_dir/child.pid"
    while true; do touch "$heartbeat"; sleep "$heartbeat_sleep"; done
  fi
  if behavior_present cancel-race; then
    trap 'complete_valid; exit 0' TERM
    "$ms" util hold --tag "$instance_tag" --stopped-file "$round_dir/child.stopped" &
    printf '%s\n' "$!" >"$round_dir/child.pid"
    while true; do touch "$heartbeat"; sleep "$heartbeat_sleep"; done
  fi
  if behavior_present malformed-return; then
    printf '{malformed\n' >"$round_dir/return.json"
    printf 'malformed return\n' >>"$log"
    violation="$round_dir/protocol-violation.txt"
    if "$root/scripts/assert-return-complete.sh" --job "$job" >"$violation" 2>&1; then
      cas_terminal completed null completed
    else
      cat "$violation" >>"$log"
      "$dispatch" __protocol-error --job "$job" --expect running --violation-file "$violation"
    fi
    exit 0
  fi
  if behavior_present interrupted-atomic-write; then
    printf '{"status":"corrupt' >"$agents/record-locks/$job.interrupted"
  fi
  if behavior_present nested-agent-events; then
    printf '{"event":"agent.completed","agent":"nested","topLevel":false}\n' >>"$events"
    printf '{"event":"turn.completed","agent":"root","topLevel":true}\n' >>"$events"
  elif ! behavior_present no-event-stream; then
    printf '{"event":"turn.completed","topLevel":true}\n' >>"$events"
  fi
  if behavior_present hook-unavailable; then printf 'hooks unavailable; polling fallback used\n' >>"$log"; fi
  if behavior_present mirror-failure; then touch "$agents/$root_job/.mirror-fail-once"; fi
  argument=$(sed -n 's/^Fake-Argument:[[:space:]]*//p' "$prompt" | head -1 || true)
  [[ -z "$argument" ]] || printf 'provider argument value=%s\n' "$argument" >>"$raw"
  complete_valid
}

probe() {
  # Fault hook for the snapshot self-heal fixtures: an unhealable probe.
  [[ -z "${METASYSTEM_FAKE_PROBE_FAIL:-}" ]] || { echo "scripted probe failure" >&2; return 1; }
  local profile=current age_days=0
  while (($#)); do
    case "$1" in
      --profile) [[ $# -ge 2 ]] || { usage; exit 2; }; profile=$2; shift 2 ;;
      --age-days) [[ $# -ge 2 ]] || { usage; exit 2; }; age_days=$2; shift 2 ;;
      *) usage; exit 2 ;;
    esac
  done
  case "$profile" in current|old|unverified-network) ;; *) usage; exit 2 ;; esac
  [[ "$age_days" =~ ^[0-9]+$ ]] || { usage; exit 2; }
  probe_fake_envelope_mechanism
  # The simulator's handshake window scales with measured load like every other
  # fixture ceiling; a fixed two-second default is a red gate on a busy machine.
  local handshake
  # shellcheck source=../fixture-budget.sh
  . "$root/scripts/agents/fixture-budget.sh"
  harness_fixture_budget_init "$root" || return 1
  handshake=$(harness_fixture_cap adapter-handshake) || return 1
  (( handshake <= 60 )) || handshake=60
  "$ms" adapter fake-capability-snapshot --dir "$agents/capabilities" \
    --profile "$profile" --age-days "$age_days" --handshake-sec "$handshake"
}

command=${1:-}
[[ -n "$command" ]] || { usage; exit 2; }
shift
case "$command" in
  local-config-paths)
    (($# == 0)) || { usage; exit 2; }
    ;;
  signature)
    (($# == 0)) || { usage; exit 2; }
    printf '%s\n' \
      'match (^|[[:space:]/-])metasystem-fake-agent([[:space:]]|$)' \
      'exclude supervision-hook\.sh' \
      'exclude scripts/agents/adapters/fake\.sh'
    ;;
  identity)
    (($# == 0)) || { usage; exit 2; }
    # The fake has no runtime configuration inputs. Its deterministic identity
    # is deliberately fixed so fixtures can select snapshots without probing.
    printf 'fake-1 fake-config-v1\n'
    ;;
  config-identity)
    (($# == 0)) || { usage; exit 2; }
    printf '%s\n' '{"cliVersion":"fake-1","configHash":"fake-config-v1","configKeyHashes":{},"runtime":"fake"}'
    ;;
  probe) probe "$@" ;;
  dispatch|follow-up) supervise "$command" "$@" ;;
  cancel)
    [[ ${1:-} == --job && $# -eq 2 ]] || { usage; exit 2; }
    "$dispatch" __cancel-owned --job "$2"
    ;;
  selftest)
    (($# == 0)) || { usage; exit 2; }
    "$0" identity >/dev/null
    "$0" probe >/dev/null
    selftest_dir=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-fake-selftest.XXXXXX")
    selftest_id="fake-selftest-$(date -u +%Y%m%dt%H%M%Sz)-$$"
    sed 's/^Working Mode:.*/Working Mode: design/' "$root/scripts/agents/templates/brief.md" >"$selftest_dir/brief.md"
    "$dispatch" dispatch --role design-critic --brief "$selftest_dir/brief.md" --runtime fake --permissions none --job-id "$selftest_id" --wait
    cp "$root/scripts/agents/templates/follow-up.md" "$selftest_dir/follow.md"
    "$dispatch" follow-up --job "$selftest_id" --message "$selftest_dir/follow.md" --wait
    sed 's/^Working Mode:.*/Working Mode: design/' "$root/scripts/agents/templates/brief.md" >"$selftest_dir/cancel.md"
    printf '\nFAKE:timeout\n' >>"$selftest_dir/cancel.md"
    "$dispatch" dispatch --role design-critic --brief "$selftest_dir/cancel.md" --runtime fake --permissions none --job-id "$selftest_id-cancel" >/dev/null
    "$dispatch" cancel --job "$selftest_id-cancel"
    mkdir -p "$agents/selftests"
    "$ms" adapter fake-selftest-record \
      --output "$agents/selftests/$selftest_id.json" --job "$selftest_id"
    echo "fake adapter selftest passed: full protocol sequence and denied-envelope mechanism probes"
    ;;
  -h|--help) usage ;;
  *) usage; exit 2 ;;
esac
