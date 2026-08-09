# Sourceable flight-recorder emitter for shell components.
#
# Contract (plans/flight-recorder.md D-3): emit_event NEVER fails its caller,
# whatever is broken -- missing python3, missing helper, unwritable stream.
# Every lookup is individually guarded and the last token is `|| true`.
#
# The WRITER is the calling process (D-1c): this function passes the caller's
# pid and start time and owns the per-process sequence counter. A new process
# is a new writer with a fresh counter, by design.
#
# Usage: emit_event <component> <event> [key=value ...]
#   Reserved keys: level, summary, missionId, jobId, turnId, cohortId,
#   repetitionIndex, executionId, ref; anything else is a payload field.

# Pure parameter expansion: sourcing must not run external commands, so a
# caller with a broken PATH can still source this file silently.
_metasystem_event_source_dir=${BASH_SOURCE[0]%/*}
_metasystem_event_root=${METASYSTEM_HARNESS_ROOT:-${_metasystem_event_source_dir%/scripts/agents}}
_metasystem_event_helper="${_metasystem_event_root}/scripts/agents/emit-event.py"
_metasystem_event_seq=0
_metasystem_event_started=""

emit_event() {
  local component=${1:-unknown} event=${2:-unknown}
  shift 2 2>/dev/null || true
  [[ -n "$_metasystem_event_root" ]] || return 0
  [[ -f "$_metasystem_event_helper" ]] || return 0
  command -v python3 >/dev/null 2>&1 || return 0
  if [[ -z "$_metasystem_event_started" ]]; then
    _metasystem_event_started=$(python3 "${_metasystem_event_root}/scripts/agents/process-census.py" started-at --pid $$ 2>/dev/null) || true
    [[ "$_metasystem_event_started" =~ ^[0-9]+$ ]] || _metasystem_event_started=0
  fi
  _metasystem_event_seq=$(( _metasystem_event_seq + 1 ))
  python3 "$_metasystem_event_helper" \
    "root=$_metasystem_event_root" \
    "component=$component" \
    "event=$event" \
    "pid=$$" \
    "pidStartedAt=$_metasystem_event_started" \
    "seq=$_metasystem_event_seq" \
    "$@" >/dev/null 2>&1 || true
}
