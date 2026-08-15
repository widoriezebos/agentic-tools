#!/usr/bin/env bash
set -euo pipefail

runtime=${1:-}
event=${2:-}
case "$runtime" in claude|codex|devin|fake) ;; *) exit 2 ;; esac
case "$event" in start|stop|end) ;; *) exit 2 ;; esac

payload=$(mktemp "${TMPDIR:-/tmp}/metasystem-supervision-hook.XXXXXX")
trap 'rm -f "$payload"' EXIT
cat >"$payload"

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
harness_root=$(cd "$script_dir/../.." && pwd -P)
ms="${METASYSTEM_BIN:-$harness_root/bin/metasystem}"

read_payload() {
  "$ms" json get --file "$payload" --field "$1" 2>/dev/null || true
}

cwd=$(read_payload cwd)
[[ -n "$cwd" ]] || cwd=${CLAUDE_PROJECT_DIR:-${DEVIN_PROJECT_DIR:-$PWD}}
repo=$(git -C "$cwd" rev-parse --show-toplevel 2>/dev/null) || exit 0
repo=$(cd "$repo" && pwd -P)
arm=$script_dir/arm-supervision.sh
[[ -x "$ms" && -x "$arm" ]] || exit 0
session=$(read_payload session_id)
[[ -n "$session" ]] || session="session-$PPID"
# Session hygiene happens ONCE at this boundary (goal-system GOAL-04):
# the runtime's string is untrusted input; anything not matching the safe
# shape becomes its sha256 hex, and every downstream use rides the result.
if ! [[ "$session" =~ ^[A-Za-z0-9._-]{1,128}$ ]]; then
  session=$(printf '%s' "$session" | "$ms" util sha256)
fi

# Hook frameworks commonly insert `/bin/sh -c` between the agent and this
# script. Start above that shell so the runtime name in the hook command itself
# cannot become a false signature match.
search_pid=$(ps -p "$PPID" -o ppid= 2>/dev/null | tr -d ' ' || true)
[[ "$search_pid" =~ ^[1-9][0-9]*$ ]] || search_pid=$PPID
identity=$("$ms" proc find-ancestor --repo "$repo" --pid "$search_pid" --runtime "$runtime" 2>/dev/null || true)
main_id=
main_class=
main_holder=false
identity_pid=
if [[ -n "$identity" ]]; then
  identity_pid=$("$ms" json get --value "$identity" --field pid)
  lease_view=$("$ms" lease classify --root "$harness_root" --caller-pid "$identity_pid" 2>/dev/null || true)
  if [[ -n "$lease_view" ]]; then
    main_id=$("$ms" json get --value "$lease_view" --field mainId 2>/dev/null || true)
    main_class=$("$ms" json get --value "$lease_view" --field class 2>/dev/null || true)
    main_holder=$("$ms" json get --value "$lease_view" --field holder 2>/dev/null || true)
  fi
fi

surface_json() { # message
  "$ms" json object "systemMessage=$1"
}

if [[ "$event" == stop ]]; then
  protocol_message=
  protocol_counts='{}'
  if [[ -n "$main_id" ]]; then
    protocol_growth=$("$ms" lease protocol-growth --root "$harness_root" --main-id "$main_id" 2>/dev/null || true)
    if [[ -n "$protocol_growth" ]]; then
      protocol_message=$("$ms" json get --value "$protocol_growth" --field message)
      protocol_counts=$("$ms" json get --value "$protocol_growth" --field counts)
    fi
  fi
  # "Advisor" is a positive finding, not a fallback. It means an announced main
  # of THIS checkout is not the one holding it. A caller that could not be
  # identified at all is not an advisor -- it is unclassified, and answering it
  # with OWNED-ELSEWHERE replaces the entire turn-end report, including the
  # refusal to walk away from open work, with a sentence about ownership.
  if [[ "$main_class" == MAIN && "$main_holder" != true ]]; then
    advisor_message="OWNED-ELSEWHERE: this main is a read-only advisor in this checkout. To write independently, run scripts/agents/second-session.sh."
    [[ -z "$protocol_message" ]] || advisor_message="$advisor_message
$protocol_message"
    surface_json "$advisor_message"
    [[ -z "$main_id" || -z "$identity_pid" || -z "$protocol_message" ]] || \
      "$ms" lease protocol-advance --root "$harness_root" --main-id "$main_id" \
        --caller-pid "$identity_pid" --counts "$protocol_counts" >/dev/null 2>&1 || true
    exit 0
  fi
  [[ -z "$identity_pid" ]] || \
    "$ms" lease renew --root "$harness_root" --caller-pid "$identity_pid" >/dev/null 2>&1 || true

  # The WATCHDOG path calls the verdict like every other path (only the
  # advisor early-exit above bypasses it): the report's text stays
  # hook-side, its DIGEST rides to the verb, and the verb's surfaceWatchdog
  # answer decides exactly-once surfacing across concurrent Stop calls
  # (goal-system GOAL-04; the loose per-session state files are retired).
  watchdog_text=$("$ms" supervise watchdog-report --repo "$harness_root" 2>/dev/null || true)
  watchdog_digest=
  [[ -z "$watchdog_text" ]] || watchdog_digest=$(printf '%s' "$watchdog_text" | "$ms" util sha256)

  # Leave evidence that this ran. Without it there is no telling a hook that
  # fired and found nothing from one that never fired, which is the confusion
  # that let this repository run for days with its hooks uninstalled.
  supervision_dir="$harness_root/artifacts/agents/supervision"
  mkdir -p "$supervision_dir"
  "$script_dir/evidence-gc.sh" >>"$supervision_dir/hooks.log" 2>&1 || true

  # ONE structured decision (goal-system GOAL-05): the verdict verb owns
  # open work, the goal clause, precedence, block-once state, and the
  # all-clear. Every representable state is exit 0 with JSON; a nonzero
  # exit is I/O failure and this hook's own FIXED degraded message takes
  # over — never silence, and never an all-clear it cannot vouch for. The
  # old 2>/dev/null||true suppression was the named defect this removes.
  verdict_stderr=$(mktemp "${TMPDIR:-/tmp}/metasystem-verdict-err.XXXXXX")
  if verdict=$("$ms" report turn-verdict --root "$harness_root" \
      --session "$session" --watchdog-surfaced "$watchdog_digest" \
      --main-id "$main_id" 2>"$verdict_stderr"); then
    rm -f "$verdict_stderr"
    should_block=$("$ms" json get --value "$verdict" --field shouldBlock)
    display=$("$ms" json get --value "$verdict" --field display)
    surface_watchdog=$("$ms" json get --value "$verdict" --field surfaceWatchdog)

    printf '%s stop verdict block=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
      "$should_block" >>"$supervision_dir/hooks.log" 2>/dev/null || true

    extras=
    [[ "$surface_watchdog" == true && -n "$watchdog_text" ]] && extras=$watchdog_text
    [[ -z "$protocol_message" ]] || extras=$(printf '%s%s%s' "$extras" "${extras:+$'\n'}" "$protocol_message")

    if [[ "$should_block" == true ]]; then
      # The display is the block reason byte-verbatim; watchdog and
      # protocol text stay in the non-blocking channel and never enter
      # the reason.
      "$ms" report stop-block "$display"
    elif [[ -n "$extras" ]]; then
      surface_json "$display
$extras"
    else
      surface_json "$display"
    fi
  else
    degraded_line=$(tail -1 "$verdict_stderr" 2>/dev/null || true)
    rm -f "$verdict_stderr"
    printf '%s stop verdict unavailable\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
      >>"$supervision_dir/hooks.log" 2>/dev/null || true
    degraded_message="turn-verdict unavailable: ${degraded_line:-no diagnostic}"
    [[ -z "$protocol_message" ]] || degraded_message="$degraded_message
$protocol_message"
    surface_json "$degraded_message"
  fi
  [[ -z "$main_id" || -z "$identity_pid" || -z "$protocol_message" ]] || \
    "$ms" lease protocol-advance --root "$harness_root" --main-id "$main_id" \
      --caller-pid "$identity_pid" --counts "$protocol_counts" >/dev/null 2>&1 || true
  exit 0
fi

if [[ -z "$identity" ]]; then
  surface_json "Metasystem supervision could not identify the immediate $runtime agent process; arming was refused."
  exit 0
fi
pid=$("$ms" json get --value "$identity" --field pid)
started=$("$ms" json get --value "$identity" --field pidStartedAt)
# The tag suffix is slugged the same way as arm-supervision.sh, so the arming
# and hook paths derive the identical instance tag.
tag="metasystem-main-$runtime-$("$ms" util slug "$session")"

if [[ "$event" == end ]]; then
  METASYSTEM_AGENT_RUNTIME="$runtime" "$arm" --repo "$repo" --session "$session" --pid "$pid" --start-time "$started" --tag "$tag" --retire >/dev/null 2>&1 || true
  exit 0
fi

if output=$(METASYSTEM_AGENT_RUNTIME="$runtime" "$arm" --repo "$repo" --session "$session" --pid "$pid" --start-time "$started" --tag "$tag" 2>&1); then
  exit 0
fi
surface_json "Metasystem supervision arming failed: $(printf '%s' "$output" | tail -1)"
