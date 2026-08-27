#!/usr/bin/env bash

checkout_execution_guard_held=0
checkout_execution_guard_engine=
checkout_execution_guard_engine_temp=
checkout_execution_guard_owner=
checkout_execution_guard_root=$root
if [[ -n "${METASYSTEM_CHECKOUT_EXECUTION_GUARD_FIXTURE:-}" ]]; then
  checkout_execution_guard_root=${METASYSTEM_CHECKOUT_EXECUTION_GUARD_ROOT:-$root}
fi

checkout_execution_guard_prepare_engine() {
  checkout_execution_guard_engine=${ms:-${METASYSTEM_BIN:-$root/bin/metasystem}}
  if [[ -x "$checkout_execution_guard_engine" ]]; then
    return 0
  fi
  if [[ -f "$root/go.mod" ]] \
      && grep -qs '^module github.com/widoriezebos/agentic-tools/metasystem$' "$root/go.mod" \
      && command -v go >/dev/null 2>&1; then
    checkout_execution_guard_engine_temp=$(mktemp "${TMPDIR:-/tmp}/metasystem-checkout-guard.XXXXXX") || return 1
    (cd "$root" && CGO_ENABLED=0 go build -o "$checkout_execution_guard_engine_temp" ./cmd/metasystem) \
      || { rm -f "$checkout_execution_guard_engine_temp"; checkout_execution_guard_engine_temp=; return 1; }
    checkout_execution_guard_engine=$checkout_execution_guard_engine_temp
    return 0
  fi
  echo "checkout execution guard: no metasystem engine is available to acquire the guard" >&2
  return 1
}

checkout_execution_guard_acquire() { # human-readable owner
  local owner=$1 wait_min progress_sec result guard_out guard_err guard_rc=0 fixture_attempted
  checkout_execution_guard_owner=$owner
  checkout_execution_guard_prepare_engine || return 1
  wait_min=$("$checkout_execution_guard_engine" config get \
    --conf "$root/metasystem.conf" --key watch.cap-min --default 180) || return 1
  progress_sec=$("$checkout_execution_guard_engine" config get \
    --conf "$root/metasystem.conf" --key watch.interval-sec --default 60) || return 1
  [[ "$wait_min" =~ ^[1-9][0-9]*$ ]] \
    || { echo "checkout execution guard: watch.cap-min must be a positive integer" >&2; return 1; }
  [[ "$progress_sec" =~ ^[1-9][0-9]*$ ]] \
    || { echo "checkout execution guard: watch.interval-sec must be a positive integer" >&2; return 1; }
  guard_err=$(mktemp "${TMPDIR:-/tmp}/metasystem-checkout-guard-error.XXXXXX") || return 1
  guard_out=$(mktemp "${TMPDIR:-/tmp}/metasystem-checkout-guard-output.XXXXXX") \
    || { rm -f "$guard_err"; return 1; }
  if [[ -n "${METASYSTEM_CHECKOUT_EXECUTION_GUARD_FIXTURE:-}" ]]; then
    fixture_attempted=$("$checkout_execution_guard_engine" json get \
      --file "$METASYSTEM_CHECKOUT_EXECUTION_GUARD_FIXTURE" --field attempted) \
      || { rm -f "$guard_out" "$guard_err"; return 2; }
    [[ "$fixture_attempted" == /* ]] \
      || { rm -f "$guard_out" "$guard_err"; return 2; }
    touch "$fixture_attempted"
  fi
  "$checkout_execution_guard_engine" gate guard-acquire \
    --root "$checkout_execution_guard_root" --owner "$owner" --wait-sec "$((wait_min * 60))" \
    --progress-sec "$progress_sec" >"$guard_out" 2>"$guard_err" || guard_rc=$?
  if (( guard_rc == 2 )); then
    rm -f "$guard_out" "$guard_err"
    echo "checkout execution guard: existing engine does not know gate guard-acquire; proceeding until this run rebuilds it" >&2
    checkout_execution_guard_held=0
    return 0
  fi
  cat "$guard_err" >&2
  rm -f "$guard_err"
  if (( guard_rc != 0 )); then rm -f "$guard_out"; return "$guard_rc"; fi
  IFS= read -r result <"$guard_out" || { rm -f "$guard_out"; return 1; }
  rm -f "$guard_out"
  case "$result" in
    acquired|joined) checkout_execution_guard_held=1 ;;
    *) echo "checkout execution guard: engine returned an unknown acquisition result: $result" >&2; return 1 ;;
  esac
}

checkout_execution_guard_release() {
  local release_rc=0
  if (( checkout_execution_guard_held )); then
    "$checkout_execution_guard_engine" gate guard-release --root "$checkout_execution_guard_root" || release_rc=$?
    checkout_execution_guard_held=0
  fi
  if [[ -n "$checkout_execution_guard_engine_temp" ]]; then
    rm -f "$checkout_execution_guard_engine_temp"
    checkout_execution_guard_engine_temp=
  fi
  return "$release_rc"
}

# The process fixture drives the production entrypoints through this explicit
# control record. It exercises only guard participation and never labels its
# zero exit as a validation or dispatch verdict.
checkout_execution_guard_fixture_wait() {
  local control=${METASYSTEM_CHECKOUT_EXECUTION_GUARD_FIXTURE:-}
  local ready release cap deadline child_control child_brief child_pid= child_rc=0
  local detach_ready detach_release detach_command
  [[ -n "$control" ]] || return 1
  [[ -f "$control" ]] || { echo "checkout execution guard fixture: control record is absent" >&2; return 2; }
  ready=$("$checkout_execution_guard_engine" json get --file "$control" --field ready) || return 2
  release=$("$checkout_execution_guard_engine" json get --file "$control" --field release) || return 2
  cap=$("$checkout_execution_guard_engine" json get --file "$control" --field capSec) || return 2
  [[ "$ready" == /* && "$release" == /* && "$cap" =~ ^[1-9][0-9]*$ ]] \
    || { echo "checkout execution guard fixture: invalid control record" >&2; return 2; }
  touch "$ready"
  detach_ready=$("$checkout_execution_guard_engine" json get --file "$control" --field detachReady 2>/dev/null || true)
  detach_release=$("$checkout_execution_guard_engine" json get --file "$control" --field detachRelease 2>/dev/null || true)
  if [[ -n "$detach_ready$detach_release" ]]; then
    [[ "$detach_ready" == /* && "$detach_release" == /* ]] \
      || { echo "checkout execution guard fixture: detached controls are incomplete" >&2; return 2; }
    detach_command=("$root/scripts/agents/checkout-execution-guard.sh" run-member \
      --root "$checkout_execution_guard_root" --engine "$checkout_execution_guard_engine" -- \
      "$root/scripts/agents/checkout-execution-guard-fixtures.sh" __wait-only \
      "$detach_ready" "$detach_release" "$cap")
    "$checkout_execution_guard_engine" supervise launch-detached --cwd "$root" \
      --execution-guard-root "$checkout_execution_guard_root" --execution-guard-owner "dispatch fixture detached member" \
      -- "${detach_command[@]}" >/dev/null
    return 0
  fi
  child_control=$("$checkout_execution_guard_engine" json get --file "$control" --field childControl 2>/dev/null || true)
  child_brief=$("$checkout_execution_guard_engine" json get --file "$control" --field childBrief 2>/dev/null || true)
  if [[ -n "$child_control$child_brief" ]]; then
    [[ -f "$child_control" && -f "$child_brief" ]] \
      || { echo "checkout execution guard fixture: nested dispatch controls are incomplete" >&2; return 2; }
    METASYSTEM_CHECKOUT_EXECUTION_GUARD_FIXTURE=$child_control \
      METASYSTEM_BIN="$checkout_execution_guard_engine" \
      "$root/scripts/agents/dispatch.sh" dispatch --role implementer --brief "$child_brief" &
    child_pid=$!
  fi
  deadline=$((SECONDS + cap))
  while [[ ! -e "$release" ]]; do
    (( SECONDS < deadline )) \
      || { echo "checkout execution guard fixture: release wait expired after ${cap}s" >&2; return 1; }
    sleep 0.05
  done
  if [[ -n "$child_pid" ]]; then
    wait "$child_pid" || child_rc=$?
    (( child_rc == 0 )) || return "$child_rc"
  fi
  return 0
}

checkout_execution_guard_run_member() {
  local guard_root= guard_engine= child= status=0
  while (($#)); do
    case "$1" in
      --root) guard_root=$2; shift 2 ;;
      --engine) guard_engine=$2; shift 2 ;;
      --) shift; break ;;
      *) echo "checkout execution guard wrapper: unknown argument $1" >&2; return 2 ;;
    esac
  done
  [[ -n "$guard_root" && -x "$guard_engine" && $# -gt 0 ]] \
    || { echo "checkout execution guard wrapper: root, engine, and command are required" >&2; return 2; }
  member_release() {
    "$guard_engine" gate guard-release --root "$guard_root" >/dev/null 2>&1 || true
  }
  trap 'member_release' EXIT
  trap 'exit 143' TERM
  trap 'exit 130' INT
  "$@" &
  child=$!
  wait "$child" || status=$?
  trap - TERM INT
  member_release
  trap - EXIT
  return "$status"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  set -euo pipefail
  command=${1:-}
  shift || true
  case "$command" in
    run-member) checkout_execution_guard_run_member "$@" ;;
    *) echo "usage: checkout-execution-guard.sh run-member --root R --engine PATH -- COMMAND..." >&2; exit 2 ;;
  esac
fi
