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

# Hook frameworks commonly insert `/bin/sh -c` between the agent and this
# script. Start above that shell so the runtime name in the hook command itself
# cannot become a false signature match.
search_pid=$(ps -p "$PPID" -o ppid= 2>/dev/null | tr -d ' ' || true)
[[ "$search_pid" =~ ^[1-9][0-9]*$ ]] || search_pid=$PPID
identity=$("$ms" census find-ancestor --repo "$repo" --pid "$search_pid" --runtime "$runtime" 2>/dev/null || true)
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

count_running_work() { # sets: running, running_details, elsewhere
  running=0
  running_details=""
  elsewhere=""
  for record in "$harness_root/artifacts/agents/jobs"/*.json; do
    [[ -e "$record" ]] || break
    if grep -q '"status": *"\(pending\|running\)"' "$record" 2>/dev/null; then
      running=$((running + 1))
      # Name the worker: a count alone tells the human nothing actionable.
      local job_id job_role job_status job_runtime detail
      job_id=$("$ms" json get --file "$record" --field jobId --default "${record##*/}")
      job_role=$("$ms" json get --file "$record" --field role --default "?")
      job_status=$("$ms" json get --file "$record" --field status --default "?")
      job_runtime=$("$ms" json get --file "$record" --field runtime --default "?")
      detail="$job_role $job_id [$job_status, $job_runtime]"
      running_details="${running_details:+$running_details; }$detail"
    fi
  done
  # A mission started from this repository usually runs in ANOTHER repository
  # (a benchmark target, a scratch repo). Reporting only this directory's
  # jobs told the human "nothing is running" while their benchmark was in
  # full flight, which is worse than saying nothing at all.
  local other
  while IFS= read -r other; do
    [[ -n "$other" ]] || continue
    elsewhere="$elsewhere $other"
  done < <(pgrep -f 'mission(-runner)? run-loop' 2>/dev/null \
    | while read -r pid; do
        ps -p "$pid" -o command= 2>/dev/null \
          | sed -n 's|.*--root [^ ]*/\([^/ ]*\) .*|\1|p; s|.*--root [^ ]*/\([^/ ]*\)$|\1|p' | head -1
      done | sort -u)
}

work_sentence() {
  count_running_work
  local missions="" active=""
  [[ -n "$elsewhere" ]] && missions=$(printf '%s' "$elsewhere" | sed 's/^ //; s/ /, /g')
  (( running )) && active="$running helper agent(s): $running_details"
  [[ -n "$missions" ]] && active="${active:+$active, and }a mission still going in $missions"
  # The orchestrator's own gate runs are neither delegate jobs nor missions,
  # and they are exactly what a human sees mid-flight and asks about.
  if pgrep -f 'validate-metasystem\.sh|validate-kit\.sh' >/dev/null 2>&1; then
    active="${active:+$active, and }the test gates"
  fi
  if [[ -n "$active" ]]; then
    printf 'STILL WORKING: %s.' "$active"
  elif [[ -n "${open_work:-}" ]]; then
    # Never assert an idle the open-work checker just contradicted.
    printf 'NO AGENTS RUNNING — but a plan still names open work, listed below.'
  else
    printf 'NOTHING LEFT TO WORK ON that the metasystem tracks: no delegate jobs, no missions, no gate runs, and no plan names open work. (Work outside the metasystem — e.g. an agent harness'\''s own background tasks — is not visible here.)'
  fi
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
  message=$("$ms" supervise watchdog-report --repo "$harness_root" 2>/dev/null || true)
  # The same discipline the stop-block holds — a report is ignorable and was
  # ignored — applies to the watchdog: an UNCHANGED report was surfaced once
  # and repeating it every turn is noise, so it is suppressed until it
  # changes (a component dies, recovers, or the census goes stale a
  # different way). The state is keyed by session, like the stop block.
  watchdog_state="$harness_root/artifacts/agents/supervision/watchdog-surfaced-$session.state"
  if [[ -n "$message" && -f "$watchdog_state" ]] \
    && [[ "$message" == "$(cat "$watchdog_state" 2>/dev/null)" ]]; then
    message=""
  elif [[ -n "$message" ]]; then
    mkdir -p "${watchdog_state%/*}" && printf '%s' "$message" >"$watchdog_state" 2>/dev/null || true
  else
    rm -f "$watchdog_state" 2>/dev/null || true
  fi
  # Continuation is the one part of the loop no prompt can guarantee, so it is
  # checked here rather than asked for in prose: a turn ending while a plan
  # still names an unblocked next step and nothing is in flight says so.
  open_work=$("$ms" report open-work --repo "$harness_root" 2>/dev/null || true)
  [[ -z "$open_work" ]] || message=$(printf '%s%s%s' "$message" "${message:+$'\n'}" "$open_work")
  [[ -z "$protocol_message" ]] || message=$(printf '%s%s%s' "$message" "${message:+$'\n'}" "$protocol_message")

  # Leave evidence that this ran. Without it there is no telling a hook that
  # fired and found nothing from one that never fired, which is the confusion
  # that let this repository run for days with its hooks uninstalled.
  supervision_dir="$harness_root/artifacts/agents/supervision"
  mkdir -p "$supervision_dir"
  # Hygiene runs itself: every turn end collects what the mirror already
  # holds, so residue never waits for a human to wander in with a flashlight.
  # Best-effort by design; a repo without an evidence root is politely refused
  # by the collector and that is fine.
  "$script_dir/evidence-gc.sh" >>"$supervision_dir/hooks.log" 2>&1 || true

  printf '%s stop open=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    "$(printf '%s' "$message" | grep -c '^OPEN-WORK' || true)" \
    >>"$supervision_dir/hooks.log" 2>/dev/null || true

  # Refuse to end the turn while work is open, once. A report is ignorable and
  # was ignored. An unbounded block is the loop the design forbids, so the same
  # open work never blocks twice: the second attempt passes and the human
  # decides.
  # Keyed by session: with a shared file, a peer session's refusal consumed
  # this session's one block, and the same open work then sailed through.
  blocked_state="$supervision_dir/stop-block-$session.state"
  open_only=$(printf '%s' "$message" | grep '^OPEN-WORK' || true)
  if [[ -n "$open_only" ]]; then
    signature=$(printf '%s' "$open_only" | shasum | cut -d' ' -f1)
    if [[ "$(cat "$blocked_state" 2>/dev/null || true)" != "$signature" ]]; then
      printf '%s' "$signature" >"$blocked_state" 2>/dev/null || true
      "$ms" report stop-block "$message"
      exit 0
    fi
  else
    rm -f "$blocked_state" 2>/dev/null || true
  fi

  # Busy-or-not is answered on every single turn end, warnings or not. A
  # warning used to REPLACE the answer, so a human asking "is it still
  # working?" got a sentence about watchdog internals instead. Warnings are
  # added to the answer now, never substituted for it.
  if [[ -n "$message" ]]; then
    surface_json "$(work_sentence)
$message"
  else
    # Say so when there is nothing to say. Silence is ambiguous to a human:
    # it reads the same whether work is finished, still running, or the hook
    # never fired at all. An explicit idle line distinguishes all three, and
    # it costs one sentence at the end of a turn.
    # No pipelines here: under pipefail a grep that matches nothing fails the
    # whole assignment, and set -e then kills the hook before it can report —
    # the silent-exit failure this very check exists to make visible.
    surface_json "$(work_sentence)"
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
