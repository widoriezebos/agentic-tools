#!/usr/bin/env bash

runtime=${1-}
event=${2-}

# SessionStart owns its outcome before it asks the filesystem, Git, an engine,
# or temporary storage for anything. These values use shell builtins only, so
# the EXIT trap can always explain an ordinary runtime failure.
start_boundary_active=false
start_terminal_complete=false
start_finishing=false
start_published=false
start_arming_started=false
start_signal_status=143
start_deferred_signal_status=0
start_payload=
start_boot_dir=
start_boot_pid=
start_prepared_context=
start_notices=
start_context_kind=none
start_context_payload=
start_context_field=
start_context_event=
start_context_shape_ready=false
start_arming_rearmed=false
start_arming_status=0
start_wait_recovery_status=0
start_deferred_identity_read=false
start_brain_failure=
start_brain_delivery=false
start_brain_declaration_sha=
start_brain_digest_emitted=false
start_brain_digest_cursor=
start_brain_digest_prefix=
start_last_resort='{"systemMessage":"Metasystem SessionStart could not produce its response: role context delivery is unconfirmed; a declared brain may be uninstructed. Rebuild bin/metasystem with scripts/agents/go-build.sh, then start a new session."}'
start_bookkeeping_notice='Metasystem SessionStart published its response but could not finish delivery bookkeeping. Repair supervision from the owning installation, then start a new session; context may repeat.'
start_holder_context_notice="Metasystem supervision could not identify the immediate $runtime agent process; arming was refused."

start_json_escape() { # text; result in start_escaped
  local LC_ALL=C input=${1-} output= character code encoded
  while [[ -n "$input" ]]; do
    character=${input:0:1}
    input=${input:1}
    case "$character" in
      '"') output=$output'\"' ;;
      '\') output=$output'\\' ;;
      $'\b') output=$output'\b' ;;
      $'\f') output=$output'\f' ;;
      $'\n') output=$output'\n' ;;
      $'\r') output=$output'\r' ;;
      $'\t') output=$output'\t' ;;
      *)
        builtin printf -v code '%d' "'$character"
        if (( code < 0 )); then
          code=$((code + 256))
        fi
        if (( code < 32 )); then
          builtin printf -v encoded '\\u%04x' "$code"
          output=$output$encoded
        else
          output=$output$character
        fi
        ;;
    esac
  done
  start_escaped=$output
}

start_cleanup() {
  local failed=false
  if [[ -n "${start_boot_pid:-}" ]]; then
    if builtin kill -0 "$start_boot_pid" 2>/dev/null; then
      builtin kill -TERM "$start_boot_pid" 2>/dev/null || failed=true
      if builtin kill -0 "$start_boot_pid" 2>/dev/null; then
        builtin kill -KILL "$start_boot_pid" 2>/dev/null || failed=true
      fi
    fi
    builtin wait "$start_boot_pid" 2>/dev/null || :
    start_boot_pid=
  fi
  if [[ -n "${start_boot_dir:-}" ]]; then
    if ! command rm -f "$start_boot_dir/stdout" "$start_boot_dir/stderr" 2>/dev/null; then
      failed=true
    fi
    if ! command rmdir "$start_boot_dir" 2>/dev/null; then
      failed=true
    fi
    start_boot_dir=
  fi
  if [[ -n "${start_payload:-}" ]]; then
    if ! command rm -f "$start_payload" 2>/dev/null; then
      failed=true
    fi
    start_payload=
  fi
  [[ "$failed" == false ]]
}

start_notice_json() { # catalog key; result in start_notice and start_notice_status
  local key=${1-}
  start_notice_status=0
  case "$key" in
    engine-missing)
      start_notice='{"systemMessage":"Metasystem engine missing: this session received no role context; if this checkout is a declared brain it is uninstructed until the engine is rebuilt: run scripts/agents/go-build.sh, then start a new session"}' ;;
    engine-skew)
      start_notice='{"systemMessage":"Metasystem engine does not answer path state-root: this session received no role context; if this checkout is a declared brain it is uninstructed. Rebuild bin/metasystem with scripts/agents/go-build.sh, then start a new session."}' ;;
    installation-directory)
      start_notice='{"systemMessage":"Metasystem SessionStart could not locate its installation directory: this session received no role context; if this checkout is a declared brain it is uninstructed. Restore access to the installed hook and its parent directories. Then start a new session."}' ;;
    checkout-identification)
      start_notice='{"systemMessage":"Metasystem SessionStart could not identify the checkout and its primary installation: this session received no role context; if this checkout is a declared brain it is uninstructed. Restore Git and access to the checkout and its primary metasystem installation. Then start a new session."}' ;;
    resolved-directory)
      start_notice='{"systemMessage":"Metasystem SessionStart could not open the resolved installation directory: this session received no role context; if this checkout is a declared brain it is uninstructed. Restore access to the installation directory returned by the engine. Then start a new session."}' ;;
    installation-validation)
      start_notice='{"systemMessage":"Metasystem SessionStart could not validate the metasystem installation: this session received no role context; if this checkout is a declared brain it is uninstructed. Restore a complete, readable metasystem installation and rebuild bin/metasystem with scripts/agents/go-build.sh. Then start a new session."}' ;;
    payload-storage)
      start_notice='{"systemMessage":"Metasystem SessionStart could not stage its input: this session received no role context; if this checkout is a declared brain it is uninstructed. Restore writable temporary storage and free space. Then start a new session."}' ;;
    boot-storage)
      start_notice='{"systemMessage":"Metasystem SessionStart could not stage brain context: this session received no role context; if this checkout is a declared brain it is uninstructed. Restore writable temporary storage and free space. Then start a new session."}' ;;
    start-preparation)
      start_notice='{"systemMessage":"Metasystem SessionStart could not prepare the session identity and context: this session received no role context; if this checkout is a declared brain it is uninstructed. Restore the installed shell tools and rebuild bin/metasystem with scripts/agents/go-build.sh. Then start a new session."}' ;;
    response-rendering|unexpected-termination)
      start_notice=$start_last_resort ;;
    payload-read)
      start_notice='{"systemMessage":"Metasystem SessionStart could not read its session input: this session received no role context; if this checkout is a declared brain it is uninstructed. Repair the SessionStart hook input and rebuild bin/metasystem with scripts/agents/go-build.sh. Then start a new session."}' ;;
    pending-read)
      start_notice='{"systemMessage":"Metasystem SessionStart could not read pending steward incidents: this session received no role context; if this checkout is a declared brain it is uninstructed. Restore access to the steward incident records and repair unreadable records. Then start a new session."}' ;;
    holder-read)
      start_notice='{"systemMessage":"Metasystem SessionStart could not read checkout holder identity: this session received no role context; if this checkout is a declared brain it is uninstructed. Restore readable checkout custody records and restart the owning runtime. Then start a new session."}' ;;
    invocation-invalid)
      start_notice_status=2
      start_notice='{"systemMessage":"Metasystem SessionStart could not accept the runtime invocation: this session received no role context; if this checkout is a declared brain it is uninstructed. Repair the installed hook registration. Then start a new session."}' ;;
    runtime-unregistered)
      start_notice_status=2
      start_notice='{"systemMessage":"Metasystem SessionStart could not find the runtime in its registry: this session received no role context; if this checkout is a declared brain it is uninstructed. Repair the installed hook registration. Then start a new session."}' ;;
    runtime-registry)
      start_notice='{"systemMessage":"Metasystem SessionStart could not read the runtime registry: this session received no role context; if this checkout is a declared brain it is uninstructed. Rebuild bin/metasystem with scripts/agents/go-build.sh and restore the installed runtime declarations. Then start a new session."}' ;;
    context-contract)
      start_notice='{"systemMessage":"Metasystem SessionStart could not read the runtime context contract: this session received no role context; if this checkout is a declared brain it is uninstructed. Rebuild bin/metasystem with scripts/agents/go-build.sh and restore the installed runtime declarations. Then start a new session."}' ;;
    custody-unreadable)
      start_notice_status=1
      start_notice='{"systemMessage":"Metasystem SessionStart could not authenticate delegate custody: this session received no role context; if this checkout is a declared brain it is uninstructed. Restore the recorded delegate custody and restart through its launcher. Then start a new session."}' ;;
    process-identity)
      start_notice='{"systemMessage":"Metasystem SessionStart could not identify the owning runtime process: this session received no role context; if this checkout is a declared brain it is uninstructed. Restart through the installed runtime launcher. Then start a new session."}' ;;
    brain-boot)
      start_notice='{"systemMessage":"Metasystem brain boot failed: this session received no role context; if this checkout is a declared brain it is uninstructed. Run metasystem brain boot --root <checkout> --repo <checkout> by hand and rebuild if it fails. Then start a new session."}' ;;
    brain-timeout)
      start_notice='{"systemMessage":"Metasystem brain boot failed (timeout): this session received no role context; if this checkout is a declared brain it is uninstructed. Run metasystem brain boot --root <checkout> --repo <checkout> by hand and rebuild if it fails. Then start a new session."}' ;;
    arming)
      start_notice='{"systemMessage":"Metasystem supervision arming failed: this session received no role context; if this checkout is a declared brain it is uninstructed. Repair supervision from the owning installation and run metasystem up there. Then start a new session."}' ;;
    wait-recovery)
      start_notice='{"systemMessage":"Metasystem SessionStart could not read durable wait recovery rows: this session received no role context; if this checkout is a declared brain it is uninstructed. Repair supervision from the owning installation and run metasystem up there. Then start a new session."}' ;;
    temporary-cleanup)
      start_notice='{"systemMessage":"Metasystem SessionStart could not remove its temporary files: this session received no role context; if this checkout is a declared brain it is uninstructed. Restore temporary-directory access and remove leftover metasystem hook temporary files. Then start a new session."}' ;;
    interrupted)
      start_notice_status=$start_signal_status
      start_notice='{"systemMessage":"Metasystem SessionStart could not finish because it was interrupted: this session received no role context; if this checkout is a declared brain it is uninstructed. Restore the runtime session. Then start a new session."}' ;;
    *) return 1 ;;
  esac
}

start_append_notice_text() { # JSON object, fixed JSON-safe text; result in start_notice
  local object=$1 text=$2
  start_notice=${object%\"\}}'\n'"$text"'"}'
}

start_build_response() { # completion reason; result in start_response
  local reason=$1 rendered_tail
  case "$reason" in
    context-ready)
      [[ -n "$start_prepared_context" && "$start_context_kind" == channel &&
          "$start_context_shape_ready" == true && -n "$start_context_field" &&
          -n "$start_context_event" && -n "$start_context_payload" ]] || return 1 ;;
    screen-context-ready)
      [[ -z "$start_prepared_context" && "$start_context_kind" == screen && -n "$start_notices" ]] || return 1 ;;
    notices-ready)
      [[ -z "$start_prepared_context" && "$start_context_kind" == none && -n "$start_notices" ]] || return 1 ;;
    healthy-no-context)
      [[ -z "$start_prepared_context" && "$start_context_kind" == none && -z "$start_notices" ]] || return 1
      start_response='{}'
      return 0 ;;
    *) return 1 ;;
  esac
  if [[ -n "$start_notices" ]]; then
    start_json_escape "$start_notices"
    if [[ -n "$start_prepared_context" ]]; then
      rendered_tail=${start_prepared_context#\{}
      start_response='{"systemMessage":"'"$start_escaped"'",'"$rendered_tail"
    else
      start_response='{"systemMessage":"'"$start_escaped"'"}'
    fi
  else
    start_response=$start_prepared_context
  fi
  [[ -n "$start_response" ]]
}

start_emergency() {
  builtin trap - EXIT HUP INT TERM
  if [[ "${start_published:-false}" == true ]]; then
    start_cleanup || :
    builtin printf '%s\n' "$start_bookkeeping_notice" >&2 || :
    start_terminal_complete=true
    builtin exit 1
  fi
  if [[ -w /dev/fd/1 ]] && builtin printf '%s\n' "$start_last_resort" 2>/dev/null; then
    start_terminal_complete=true
    builtin exit 0
  fi
  builtin printf '%s\n' "$start_last_resort" >&2 || :
  start_terminal_complete=true
  builtin exit 74
}

start_finish() { # notice <catalog-key> | intentional <declared-reason>
  local family=${1-} key=${2-} response= status=0 skip_line= cleanup_complete=false
  if [[ "${start_finishing:-false}" == true ]]; then
    start_emergency
  fi
  start_finishing=true
  start_deferred_signal_status=0
  builtin trap 'start_defer_signal 129' HUP
  builtin trap 'start_defer_signal 130' INT
  builtin trap 'start_defer_signal 143' TERM

  if [[ "${start_published:-false}" == true ]]; then
    start_cleanup || :
    builtin printf '%s\n' "$start_bookkeeping_notice" >&2 || :
    start_terminal_complete=true
    builtin trap - EXIT HUP INT TERM
    builtin exit 1
  fi

  case "$family" in
    notice)
      if ! start_notice_json "$key"; then
        start_emergency
      fi
      response=$start_notice
      status=$start_notice_status
      if [[ ( "$key" == brain-boot || "$key" == brain-timeout ) && -n "$start_notices" ]]; then
        start_json_escape "$start_notices"
        start_append_notice_text "$response" "$start_escaped"
        response=$start_notice
      fi
      if [[ "$key" == arming && "$start_arming_rearmed" == true ]]; then
        start_append_notice_text "$response" 'Metasystem re-armed the rebuilt engine: the engine reported a completed re-arm before the later failure.'
        response=$start_notice
      fi
      if ! start_cleanup; then
        if [[ "$key" != temporary-cleanup && "$response" != "$start_last_resort" ]]; then
          start_append_notice_text "$response" 'Metasystem SessionStart also could not remove its temporary files. Restore temporary-directory access and remove leftover metasystem hook temporary files.'
          response=$start_notice
        fi
      fi
      cleanup_complete=true
      if [[ "$start_arming_started" == true && "$response" != "$start_last_resort" ]]; then
        start_append_notice_text "$response" 'Supervision may have been partly initialized.'
        response=$start_notice
      fi
      ;;
    intentional)
      case "$key" in
        authenticated-delegate)
          skip_line='Metasystem SessionStart intentionally skipped: authenticated delegate; its launcher owns context.' ;;
        foreign-runtime)
          skip_line='Metasystem SessionStart intentionally skipped: another runtime owns this process.' ;;
        context-ready|screen-context-ready|notices-ready|healthy-no-context)
          if ! start_build_response "$key"; then
            start_emergency
          fi
          response=$start_response ;;
        *) start_emergency ;;
      esac
      ;;
    *) start_emergency ;;
  esac

  if [[ -n "$skip_line" ]]; then
    if ! start_cleanup; then
      start_emergency
    fi
  fi

  if (( start_deferred_signal_status != 0 )); then
    start_signal_status=$start_deferred_signal_status
    start_deferred_signal_status=0
    if ! start_notice_json interrupted; then
      start_emergency
    fi
    response=$start_notice
    status=$start_notice_status
    family=notice
    key=interrupted
    skip_line=
    if [[ "$start_arming_started" == true ]]; then
      start_append_notice_text "$response" 'Supervision may have been partly initialized.'
      response=$start_notice
    fi
  fi

  if [[ -n "$skip_line" ]]; then
    if ! builtin printf '%s\n' "$skip_line" >&2; then
      if builtin printf '%s\n' "$start_last_resort"; then
        start_terminal_complete=true
        builtin trap - EXIT HUP INT TERM
        builtin exit 74
      fi
      start_emergency
    fi
    start_published=true
    builtin trap 'start_signal 129' HUP
    builtin trap 'start_signal 130' INT
    builtin trap 'start_signal 143' TERM
    if (( start_deferred_signal_status != 0 )); then
      start_cleanup || :
      builtin printf '%s\n' "$start_bookkeeping_notice" >&2 || :
      start_terminal_complete=true
      builtin trap - EXIT HUP INT TERM
      builtin exit 1
    fi
    start_terminal_complete=true
    builtin trap - EXIT HUP INT TERM
    builtin exit 0
  fi

  if [[ ! -w /dev/fd/1 ]] || ! builtin printf '%s\n' "$response" 2>/dev/null; then
    builtin printf '%s\n' "$start_last_resort" >&2 || :
    start_terminal_complete=true
    builtin trap - EXIT HUP INT TERM
    builtin exit 74
  fi
  start_published=true
  builtin trap 'start_signal 129' HUP
  builtin trap 'start_signal 130' INT
  builtin trap 'start_signal 143' TERM
  if (( start_deferred_signal_status != 0 )); then
    start_cleanup || :
    builtin printf '%s\n' "$start_bookkeeping_notice" >&2 || :
    start_terminal_complete=true
    builtin trap - EXIT HUP INT TERM
    builtin exit 1
  fi

  if [[ "$family" == intentional && "$start_brain_delivery" == true ]]; then
    if [[ "$start_brain_digest_emitted" == true ]]; then
      if ! "$ms" brain start-delivered --root "$repo" --repo "$repo" \
          --declaration-sha256 "$start_brain_declaration_sha" \
          --digest-cursor "$start_brain_digest_cursor" \
          --digest-prefix-sha256 "$start_brain_digest_prefix" >/dev/null 2>&1; then
        builtin printf '%s\n' "$start_bookkeeping_notice" >&2 || :
        status=1
      fi
    elif ! "$ms" brain start-delivered --root "$repo" --repo "$repo" \
        --declaration-sha256 "$start_brain_declaration_sha" >/dev/null 2>&1; then
      builtin printf '%s\n' "$start_bookkeeping_notice" >&2 || :
      status=1
    fi
  fi
  if [[ "$cleanup_complete" != true ]] && ! start_cleanup; then
    if (( status == 0 )); then
      builtin printf '%s\n' "$start_bookkeeping_notice" >&2 || :
    fi
    status=1
  fi
  start_terminal_complete=true
  builtin trap - EXIT HUP INT TERM
  builtin exit "$status"
}

start_exit_trap() {
  local actual_status=${1-1}
  : "$actual_status"
  if [[ "${start_terminal_complete:-false}" != true ]]; then
    start_finish notice unexpected-termination
  fi
}

start_signal() {
  start_signal_status=$1
  if [[ "${start_published:-false}" == true ]]; then
    start_finish notice interrupted
  fi
  start_finish notice interrupted
}

start_defer_signal() {
  start_deferred_signal_status=$1
}

if [[ "$event" == start ]]; then
  start_boundary_active=true
  builtin trap 'start_exit_trap "$?"' EXIT
  builtin trap 'start_signal 129' HUP
  builtin trap 'start_signal 130' INT
  builtin trap 'start_signal 143' TERM
fi

set -euo pipefail

# Resolution order is part of the hook boundary: (1) SessionStart initializes
# one termination and publication owner before every fallible operation; other
# malformed event and runtime names are rejected without an engine; (2) the physical installation
# is mapped to its primary counterpart when it is a linked worktree, using Git
# common-directory identity with inherited Git steering removed, and every
# identification failure is benign rather than a guess; (3) that installation's
# own engine must exist even when an override runs the turn, and a missing
# engine allows a Stop under a fixed degraded notice and reports missing role
# context on SessionStart; (4) an unregistered
# runtime exits 2; (5) the running engine validates that installation through
# path state-root, whose exit-1 refusal is benign while any other failure allows
# a Stop under a fixed engine-and-hook-skew notice and reports missing role
# context on SessionStart; (6) the Stop-deadline parent
# uses the same resolver and one engine for session parsing, installation
# validation, response validation, and refusal records, and never blocks on its
# own; (7) every state consumer uses the one world named repo, without deriving
# it again from a marker, path, or configuration. The runtime argument's
# shape remains open to newly registered runtimes. The recovery-only scheduler
# entry is operator-owned and can be printed with `metasystem up
# --print-scheduler-entry`; this hook never installs host state.

# The deadline parent's own notices never depend on an engine: the engine
# is the thing most likely to be missing, crashed or mid-replacement when the
# parent has to speak. Fixed text only (no quotes or backslashes).
emit_fixed_json_notice() { # fixed lines, joined by JSON newlines
  local text=$1 line
  shift
  for line in "$@"; do text="$text"'\n'"$line"; done
  printf '{"systemMessage":"%s"}\n' "$text"
}
emit_raw_stop_allowance() { # optional fixed prefix line (plain text, no quotes or backslashes)
  local qualifiers=${1:-}
  printf '{"systemMessage":"Task unknown; Stop allowed; needs supervision repair; stop-hook-output-was-unreadable%s. The steward must restore supervision. Status unavailable."}\n' "$qualifiers"
}
raw_missing_engine_stop='{"systemMessage":"Task unknown; Stop allowed; needs supervision repair; engine missing. Rebuild bin/metasystem. Status unavailable."}'
raw_engine_skew_stop='{"systemMessage":"Task unknown; Stop allowed; needs supervision repair; engine does not answer path state-root. Rebuild bin/metasystem. Status unavailable."}'
internal_skip_result='METASYSTEM_INTERNAL_HOOK_SKIP_V1'

# Git steering inherited from another repository must not redirect checkout
# identification. The compiled state-root authority removes the same names.
hook_git() {
  env -u GIT_DIR -u GIT_WORK_TREE -u GIT_COMMON_DIR -u GIT_INDEX_FILE \
    -u GIT_CEILING_DIRECTORIES -u GIT_DISCOVERY_ACROSS_FILESYSTEM \
    -u GIT_OBJECT_DIRECTORY -u GIT_ALTERNATE_OBJECT_DIRECTORIES \
    -u GIT_CONFIG -u GIT_CONFIG_PARAMETERS -u GIT_CONFIG_COUNT \
    -u GIT_CONFIG_GLOBAL -u GIT_CONFIG_SYSTEM -u GIT_CONFIG_NOSYSTEM \
    -u GIT_GRAFT_FILE -u GIT_SHALLOW_FILE -u GIT_REPLACE_REF_BASE \
    -u GIT_IMPLICIT_WORK_TREE -u GIT_NO_REPLACE_OBJECTS -u GIT_PREFIX \
    git "$@"
}

# Print the physical installation that governs this hook. Every linked
# worktree maps once to the same relative installation beneath its primary
# checkout because the engine arms no linked worktree. Repository
# identification must succeed; a failed query never becomes proof that the
# candidate is an ordinary checkout.
hook_world_installation() {
  local git_ids git_dir git_common primary_top wt_top rel world_installation
  world_installation=$harness_root
  git_ids=$(hook_git -C "$harness_root" rev-parse --path-format=absolute \
    --git-dir --git-common-dir 2>/dev/null) || return 1
  [[ "$git_ids" == *$'\n'* ]] || return 1
  git_dir=${git_ids%%$'\n'*}
  git_common=${git_ids#*$'\n'}
  [[ -n "$git_dir" && -n "$git_common" ]] || return 1
  if [[ "$git_dir" != "$git_common" ]]; then
    [[ "$(basename -- "$git_common")" == .git ]] || return 1
    primary_top=$(cd -- "$(dirname -- "$git_common")" 2>/dev/null && pwd -P) || return 1
    wt_top=$(hook_git -C "$harness_root" rev-parse --show-toplevel 2>/dev/null) || return 1
    wt_top=$(cd -- "$wt_top" 2>/dev/null && pwd -P) || return 1
    case "$harness_root" in
      "$wt_top") rel= ;;
      "$wt_top"/*) rel=${harness_root#"$wt_top"/} ;;
      *) return 1 ;;
    esac
    world_installation=$primary_top${rel:+/$rel}
    [[ -d "$world_installation/scripts/agents" ]] || return 1
  fi
  printf '%s\n' "$world_installation"
}

hook_root_is_one_line() {
  [[ -n "$1" && "$1" != *$'\n'* && "$1" != *$'\r'* ]]
}

start_physical_directory() {
  builtin cd -- "$1" 2>/dev/null && builtin pwd -P
}

start_capture_defer_brain_failure() {
  case ${1-} in
    brain-boot) start_defer_brain_failure brain-boot ;;
    brain-timeout) start_defer_brain_failure brain-timeout ;;
    *) return 1 ;;
  esac
}

start_capture() { # failure key, destination, policy, argv
  local failure_key=$1 destination=$2 policy=$3 captured= capture_status=0
  shift 3
  if [[ "$policy" == arming ]]; then
    start_arming_started=true
  fi
  if [[ "$policy" == arming || "$policy" == wait-recovery ]]; then
    captured=$(builtin trap - EXIT HUP INT TERM; "$@" 2>&1) && capture_status=0 || capture_status=$?
  elif captured=$(builtin trap - EXIT HUP INT TERM; "$@"); then
    capture_status=0
  else
    capture_status=$?
  fi
  case "$policy" in
    required)
      if (( capture_status != 0 )); then
        if start_capture_defer_brain_failure "$failure_key"; then return 0; fi
        start_finish notice "$failure_key"
      fi
      ;;
    required-nonempty)
      if (( capture_status != 0 )) || [[ -z "$captured" ]]; then
        start_finish notice "$failure_key"
      fi
      ;;
    shell-required)
      if (( capture_status != 0 )) || [[ "$captured" != *x ]]; then
        if start_capture_defer_brain_failure "$failure_key"; then return 0; fi
        start_finish notice "$failure_key"
      fi
      captured=${captured%x}
      ;;
    shell-required-nonempty)
      if (( capture_status != 0 )) || [[ "$captured" != *x ]]; then
        if start_capture_defer_brain_failure "$failure_key"; then return 0; fi
        start_finish notice "$failure_key"
      fi
      captured=${captured%x}
      if [[ -z "$captured" ]]; then
        if start_capture_defer_brain_failure "$failure_key"; then return 0; fi
        start_finish notice "$failure_key"
      fi
      ;;
    identity-optional)
      if (( capture_status != 0 )); then
        start_deferred_identity_read=true
        captured=
      fi
      ;;
    identity-required)
      if (( capture_status != 0 )); then
        start_deferred_identity_read=true
        captured=
      fi
      ;;
    identity-required-nonempty)
      if (( capture_status != 0 )) || [[ -z "$captured" ]]; then
        start_deferred_identity_read=true
        captured=
      fi
      ;;
    identity-shell-required)
      if (( capture_status != 0 )) || [[ "$captured" != *x ]]; then
        start_deferred_identity_read=true
        captured=
      else
        captured=${captured%x}
      fi
      ;;
    arming)
      start_arming_status=$capture_status
      if [[ "$captured" == *' re-armed='* ]]; then
        start_arming_rearmed=true
      fi
      ;;
    one-line)
      if (( capture_status != 0 )) || ! hook_root_is_one_line "$captured"; then
        start_finish notice "$failure_key"
      fi
      ;;
    state-root)
      if (( capture_status == 1 )); then
        start_finish notice installation-validation
      elif (( capture_status != 0 )) || ! hook_root_is_one_line "$captured"; then
        start_finish notice engine-skew
      fi
      ;;
    json-optional-field)
      if (( capture_status == 3 )); then
        captured=
      elif (( capture_status != 0 )) || [[ "$captured" != *x ]]; then
        if start_capture_defer_brain_failure "$failure_key"; then return 0; fi
        start_finish notice "$failure_key"
      else
        captured=${captured%x}
      fi
      ;;
    delegate-custody)
      if (( capture_status == 3 )); then
        captured=false
      elif (( capture_status != 0 )) ||
          ! [[ "$captured" =~ ^\{\"delegate\":true,\"jobId\":\"[a-z0-9][a-z0-9-]*\",\"matchedPid\":[1-9][0-9]*,\"comparisonMode\":\"(darwin-microseconds|linux-ticks-boot-id|legacy-seconds)\"\}$ ]]; then
        start_finish notice "$failure_key"
      else
        captured=true
      fi
      ;;
    context-channel)
      if (( capture_status == 1 )); then
        captured=
      elif (( capture_status != 0 )); then
        start_finish notice "$failure_key"
      fi
      ;;
    wait-recovery)
      start_wait_recovery_status=$capture_status
      if (( capture_status == 64 )); then
        captured=
      fi
      ;;
    allow-empty)
      if (( capture_status != 0 )); then
        if start_capture_defer_brain_failure "$failure_key"; then return 0; fi
        start_finish notice "$failure_key"
      fi
      ;;
    *) start_finish notice unexpected-termination ;;
  esac
  builtin printf -v "$destination" '%s' "$captured"
}

start_stage_payload() {
  if ! command cat >"$start_payload"; then
    start_finish notice payload-storage
  fi
}

start_hash_text() {
  builtin printf '%s' "$1" | "$ms" util sha256
}

start_json_value_with_sentinel() {
  "$@" && builtin printf x
}

collect_start_notice() {
  if [[ -n "$start_notices" ]]; then
    start_notices=$start_notices$'\n'$1
  else
    start_notices=$1
  fi
}

start_defer_brain_failure() {
  case ${1-} in
    brain-boot)
      start_brain_failure=brain-boot
      ;;
    brain-timeout)
      start_brain_failure=brain-timeout
      ;;
    *) start_finish notice unexpected-termination ;;
  esac
  start_context_kind=none
  start_context_payload=
}

start_finish_prepared() {
  if [[ "$start_brain_failure" == brain-boot ]]; then
    start_finish notice brain-boot
  elif [[ "$start_brain_failure" == brain-timeout ]]; then
    start_finish notice brain-timeout
  elif [[ "$start_context_kind" == channel ]]; then
    start_finish intentional context-ready
  elif [[ "$start_context_kind" == screen ]]; then
    start_finish intentional screen-context-ready
  elif [[ -n "$start_notices" ]]; then
    start_finish intentional notices-ready
  else
    start_finish intentional healthy-no-context
  fi
}

start_prepare_context_object() {
  local rest key leaf event_object rendered probe event_field event_probe
  start_prepared_context=
  start_context_shape_ready=false
  [[ -n "$start_context_payload" && -n "$start_context_field" ]] || return 0
  if [[ "$start_context_kind" != channel || -z "$start_context_event" ]]; then
    start_finish notice response-rendering
  fi
  rest=$start_context_field
  key=${rest##*.}
  start_capture response-rendering leaf required-nonempty "$ms" json object "$key=$start_context_payload"
  rest=${rest%.*}
  if [[ -n "$start_context_event" ]]; then
    start_capture response-rendering event_object required-nonempty "$ms" json object "hookEventName=$start_context_event"
    leaf=${leaf%\}},${event_object#\{}
  fi
  while [[ -n "$rest" ]]; do
    key=${rest##*.}
    leaf="{\"$key\":$leaf}"
    [[ "$rest" == *.* ]] || break
    rest=${rest%.*}
  done
  rendered=$leaf
  start_capture response-rendering probe shell-required start_json_value_with_sentinel \
    "$ms" json get --value "$rendered" --field "$start_context_field" --shell-safe
  if [[ "$probe" != "$start_context_payload" ]]; then
    start_finish notice response-rendering
  fi
  event_field=${start_context_field%.*}.hookEventName
  start_capture response-rendering event_probe shell-required start_json_value_with_sentinel \
    "$ms" json get --value "$rendered" --field "$event_field" --shell-safe
  if [[ "$event_probe" != "$start_context_event" ]]; then
    start_finish notice response-rendering
  fi
  start_prepared_context=$rendered
  start_context_shape_ready=true
}

start_preserve_boot_stderr() {
  if [[ -s "$start_boot_dir/stderr" ]]; then
    if ! command cat "$start_boot_dir/stderr" >&2; then
      builtin printf '%s\n' 'Metasystem SessionStart could not relay the brain boot diagnostic.' >&2 || :
    fi
  fi
}

start_stop_boot_child() {
  local command_line=
  start_capture brain-timeout command_line allow-empty ps -p "$start_boot_pid" -o command=
  [[ -z "$start_brain_failure" ]] || return 0
  if [[ "$command_line" != *"$ms"* ]]; then
    start_defer_brain_failure brain-timeout
    return 0
  fi
  if ! builtin kill -TERM "$start_boot_pid" 2>/dev/null; then
    start_defer_brain_failure brain-timeout
    return 0
  fi
}

start_prepare_brain() {
  local boot_out boot_err boot_status=0 boot_timeout=false boot_started
  local declared unknown state bytes sections_valid section section_state
  start_capture boot-storage start_boot_dir required-nonempty command mktemp -d "${TMPDIR:-/tmp}/metasystem-brain-boot-hook.XXXXXX"
  boot_out=$start_boot_dir/stdout
  boot_err=$start_boot_dir/stderr
  "$ms" brain boot --read-only --root "$repo" --repo "$repo" --bytes "$start_context_bytes" \
    --deadline-ms "$start_brain_deadline_ms" >"$boot_out" 2>"$boot_err" &
  start_boot_pid=$!
  boot_started=$SECONDS
  while builtin kill -0 "$start_boot_pid" 2>/dev/null && (( SECONDS - boot_started < start_brain_wait_sec )); do
    if ! command sleep 0.05; then
      start_defer_brain_failure brain-boot
      return 0
    fi
  done
  if builtin kill -0 "$start_boot_pid" 2>/dev/null; then
    boot_timeout=true
    start_stop_boot_child
    [[ -z "$start_brain_failure" ]] || return 0
    if ! command sleep 0.2; then
      start_defer_brain_failure brain-timeout
      return 0
    fi
    if builtin kill -0 "$start_boot_pid" 2>/dev/null; then
      if ! builtin kill -KILL "$start_boot_pid" 2>/dev/null; then
        start_defer_brain_failure brain-timeout
        return 0
      fi
    fi
  fi
  builtin wait "$start_boot_pid" || boot_status=$?
  start_boot_pid=
  if [[ "$boot_timeout" == true ]]; then
    start_preserve_boot_stderr
    start_defer_brain_failure brain-timeout
    return 0
  elif (( boot_status != 0 )) || [[ ! -s "$boot_out" ]]; then
    start_preserve_boot_stderr
    start_defer_brain_failure brain-boot
    return 0
  fi

  start_capture brain-boot declared required "$ms" json get --file "$boot_out" --field declared
  [[ -z "$start_brain_failure" ]] || return 0
  if [[ "$declared" == false ]]; then
    start_capture brain-boot unknown required "$ms" json strip --file "$boot_out" --key declared
    [[ -z "$start_brain_failure" ]] || return 0
    if [[ "$unknown" != '{}' ]]; then
      start_defer_brain_failure brain-boot
      return 0
    fi
    start_context_kind=none
    return 0
  fi
  if [[ "$declared" != true ]]; then
    start_defer_brain_failure brain-boot
    return 0
  fi
  start_capture brain-boot unknown required "$ms" json strip --file "$boot_out" --key declared --key state \
    --key payload --key bytes --key sections --key digestEmitted --key digestCursor \
    --key digestPrefixSha256 --key declarationSha256
  [[ -z "$start_brain_failure" ]] || return 0
  if [[ "$unknown" != '{}' ]]; then
    start_defer_brain_failure brain-boot
    return 0
  fi
  start_capture brain-boot state shell-required start_json_value_with_sentinel \
    "$ms" json get --file "$boot_out" --field state --shell-safe
  [[ -z "$start_brain_failure" ]] || return 0
  start_capture brain-boot bytes required "$ms" json get --file "$boot_out" --field bytes
  [[ -z "$start_brain_failure" ]] || return 0
  start_capture brain-boot start_brain_digest_emitted required "$ms" json get --file "$boot_out" --field digestEmitted
  [[ -z "$start_brain_failure" ]] || return 0
  start_capture brain-boot start_brain_digest_cursor required "$ms" json get --file "$boot_out" --field digestCursor
  [[ -z "$start_brain_failure" ]] || return 0
  start_capture brain-boot start_brain_digest_prefix json-optional-field start_json_value_with_sentinel \
    "$ms" json get --file "$boot_out" --field digestPrefixSha256 --shell-safe
  [[ -z "$start_brain_failure" ]] || return 0
  start_capture brain-boot start_brain_declaration_sha json-optional-field start_json_value_with_sentinel \
    "$ms" json get --file "$boot_out" --field declarationSha256 --shell-safe
  [[ -z "$start_brain_failure" ]] || return 0
  start_capture brain-boot start_context_payload shell-required-nonempty start_json_value_with_sentinel \
    "$ms" json get --file "$boot_out" --field payload --shell-safe
  [[ -z "$start_brain_failure" ]] || return 0
  sections_valid=true
  for section in asks held fleet digest; do
    start_capture brain-boot section_state shell-required start_json_value_with_sentinel \
      "$ms" json get --file "$boot_out" --field "sections.$section" --shell-safe
    [[ -z "$start_brain_failure" ]] || return 0
    case "$section_state" in complete|cut|skipped|error) ;; *) sections_valid=false ;; esac
  done
  if [[ ( "$state" != declared && "$state" != corrupt ) || ! "$bytes" =~ ^[0-9]+$ ||
        ( "$start_brain_digest_emitted" != true && "$start_brain_digest_emitted" != false ) ||
        ! "$start_brain_digest_cursor" =~ ^[0-9]+$ || "$sections_valid" != true ||
        -z "$start_context_payload" ]]; then
    start_defer_brain_failure brain-boot
    return 0
  fi
  if [[ "$start_brain_digest_emitted" == true && ! "$start_brain_digest_prefix" =~ ^[0-9a-f]{64}$ ]]; then
    start_defer_brain_failure brain-boot
    return 0
  fi
  if [[ "$state" == declared ]]; then
    if ! [[ "$start_brain_declaration_sha" =~ ^[0-9a-f]{64}$ ]]; then
      start_defer_brain_failure brain-boot
      return 0
    fi
    start_brain_delivery=true
  else
    if [[ -n "$start_brain_declaration_sha" ]]; then
      start_defer_brain_failure brain-boot
      return 0
    fi
  fi
  if [[ -n "$start_context_field" ]]; then
    start_context_kind=channel
  else
    start_context_kind=screen
    start_notices='this runtime has no session-context channel; the packet reached the screen only'
    collect_start_notice "$start_context_payload"
    if (( ${#start_notices} > 2048 )); then
      start_notices=${start_notices:0:2048}
    fi
    start_context_payload=
  fi
}

start_main() {
  local source_path source_parent registered_runtimes delegate_result repo_result payload_probe session
  local start_source start_session_absent
  local identity identity_runtime identity_pid identity_started parent_view parent_class parent_runtime
  local process_identity_failed=false
  local start_context_decl start_context_rc start_context_sources pending_line
  local up_output up_aggregate waiting_lines hash_result

  if ! [[ "$runtime" =~ ^[a-z][a-z0-9-]{0,31}$ ]]; then
    start_finish notice invocation-invalid
  fi

  delegate_state_hint=${METASYSTEM_HOOK_DELEGATE_STATE_ROOT:-}
  delegate_installation_hint=${METASYSTEM_HOOK_DELEGATE_INSTALLATION_ROOT:-}
  delegate_job_hint=${METASYSTEM_HOOK_DELEGATE_JOB:-}
  if [[ -n "$delegate_state_hint$delegate_installation_hint$delegate_job_hint" ]]; then
    if [[ -z "$delegate_state_hint" || -z "$delegate_installation_hint" || -z "$delegate_job_hint" ||
          ! -x "$delegate_installation_hint/bin/metasystem" ]]; then
      start_finish notice custody-unreadable
    fi
    start_capture custody-unreadable delegate_result delegate-custody \
      "$delegate_installation_hint/bin/metasystem" lease hook-delegate \
      --root "$delegate_state_hint" --metasystem-root "$delegate_installation_hint" \
      --job "$delegate_job_hint" --caller-pid "$PPID"
    if [[ "$delegate_result" == true ]]; then
      start_finish intentional authenticated-delegate
    fi
    start_finish notice custody-unreadable
  fi

  source_path=${BASH_SOURCE[0]}
  case "$source_path" in
    */*) source_parent=${source_path%/*} ;;
    *) source_parent=. ;;
  esac
  start_capture installation-directory script_dir required-nonempty start_physical_directory "$source_parent"
  start_capture installation-directory harness_root required-nonempty start_physical_directory "$script_dir/../.."
  start_capture checkout-identification world_installation required-nonempty hook_world_installation
  canonical=$world_installation/bin/metasystem
  ms=${METASYSTEM_BIN:-$canonical}
  if [[ ! -x "$canonical" || ! -x "$ms" ]]; then
    start_finish notice engine-missing
  fi
  start_capture runtime-registry registered_runtimes required-nonempty "$ms" runtime list
  case $'\n'$registered_runtimes$'\n' in
    *$'\n'$runtime$'\n'*) ;;
    *) start_finish notice runtime-unregistered ;;
  esac

  start_capture payload-storage start_payload required-nonempty command mktemp "${TMPDIR:-/tmp}/metasystem-supervision-hook.XXXXXX"
  start_stage_payload
  start_capture engine-skew repo_result state-root "$ms" path state-root "$world_installation"
  start_capture resolved-directory repo required-nonempty start_physical_directory "$repo_result"

  start_capture payload-read payload_probe required "$ms" json strip --file "$start_payload" --key __metasystem_shape_probe
  : "$payload_probe"
  start_capture payload-read session shell-required start_json_value_with_sentinel \
    "$ms" json get --value "$payload_probe" --field session_id --default '' --shell-safe
  start_source=
  case "$payload_probe" in
    *$'\n  "source":'*)
      start_capture payload-read start_source shell-required-nonempty start_json_value_with_sentinel \
        "$ms" json get --value "$payload_probe" --field source --shell-safe
      ;;
  esac
  start_session_absent=false
  if [[ -z "$session" ]]; then
    start_session_absent=true
    session="session-$PPID"
  fi
  if ! [[ "$session" =~ ^[A-Za-z0-9._-]{1,128}$ ]]; then
    start_capture start-preparation hash_result required-nonempty start_hash_text "$session"
    session=$hash_result
  fi

  start_capture custody-unreadable delegate_result delegate-custody "$ms" lease hook-delegate \
    --root "$repo" --metasystem-root "$world_installation" --caller-pid "$PPID"
  if [[ "$delegate_result" == true ]]; then
    start_finish intentional authenticated-delegate
  fi

  if [[ "$runtime" == fake ]]; then
    start_capture process-identity identity identity-optional "$ms" proc find-ancestor \
      --repo "$world_installation" --pid "$PPID" --runtime fake
  else
    start_capture process-identity identity identity-optional "$ms" proc find-ancestor \
      --repo "$world_installation" --pid "$PPID" --all-hosts
  fi
  identity_pid=
  identity_started=
  if [[ -n "$identity" ]]; then
    start_capture process-identity identity_runtime identity-shell-required start_json_value_with_sentinel \
      "$ms" json get --value "$identity" --field runtime --shell-safe
    if [[ "$start_deferred_identity_read" != true && "$identity_runtime" != "$runtime" ]]; then
      start_finish intentional foreign-runtime
    fi
    if [[ "$start_deferred_identity_read" != true ]]; then
      start_capture process-identity identity_pid identity-required "$ms" json get --value "$identity" --field pid
      start_capture process-identity identity_started identity-required "$ms" json get --value "$identity" --field pidStartedAt
    fi
    if ! [[ "$identity_pid" =~ ^[1-9][0-9]*$ && "$identity_started" =~ ^[1-9][0-9]*$ ]]; then
      start_deferred_identity_read=true
      identity_pid=
      identity_started=
    fi
  else
    process_identity_failed=$start_deferred_identity_read
    start_deferred_identity_read=false
    start_capture holder-read parent_view identity-required-nonempty "$ms" lease classify \
      --root "$repo" --metasystem-root "$world_installation" --caller-pid "$PPID"
    if [[ "$start_deferred_identity_read" != true ]]; then
      start_capture holder-read parent_class identity-shell-required start_json_value_with_sentinel \
        "$ms" json get --value "$parent_view" --field class --shell-safe
    fi
    if [[ "$start_deferred_identity_read" != true ]]; then
      if [[ "$parent_class" != MAIN ]]; then
        start_deferred_identity_read=true
      fi
    fi
    if [[ "$start_deferred_identity_read" != true ]]; then
      start_capture holder-read parent_runtime identity-shell-required start_json_value_with_sentinel \
        "$ms" json get --value "$parent_view" --field announcement.runtime --shell-safe
    fi
    if [[ "$start_deferred_identity_read" != true ]]; then
      if [[ "$parent_runtime" != "$runtime" ]]; then
        start_finish intentional foreign-runtime
      fi
      start_capture holder-read identity_pid identity-required "$ms" json get --value "$parent_view" --field announcement.pid
      start_capture holder-read identity_started identity-required "$ms" json get --value "$parent_view" --field announcement.pidStartedAt
    fi
    if [[ "$start_deferred_identity_read" != true ]] &&
        ! [[ "$identity_pid" =~ ^[1-9][0-9]*$ && "$identity_started" =~ ^[1-9][0-9]*$ ]]; then
      start_deferred_identity_read=true
    fi
    if [[ "$process_identity_failed" == true ]]; then
      start_deferred_identity_read=true
    fi
  fi

  start_capture context-contract start_context_decl context-channel "$ms" runtime start-context "$runtime"
  start_context_field=
  start_context_event=
  start_context_bytes=2048
  if [[ -n "$start_context_decl" ]]; then
    start_context_field=${start_context_decl#field=}
    start_context_field=${start_context_field%% event=*}
    start_context_event=${start_context_decl#* event=}
    start_context_event=${start_context_event%% bytes=*}
    start_context_bytes=${start_context_decl#* bytes=}
    start_context_bytes=${start_context_bytes%% sources=*}
    start_context_sources=${start_context_decl#* sources=}
    if ! [[ "$start_context_field" =~ ^[A-Za-z][A-Za-z0-9]*(\.[A-Za-z][A-Za-z0-9]*)+$ &&
          "$start_context_event" =~ ^[A-Za-z][A-Za-z0-9]{0,63}$ &&
          "$start_context_bytes" =~ ^[0-9]+$ && "$start_context_bytes" -ge 2048 &&
          -n "$start_context_sources" ]]; then
      start_finish notice context-contract
    fi
  fi

  start_capture pending-read pending_line allow-empty "$ms" steward pending --repo "$repo"
  [[ -z "$pending_line" ]] || collect_start_notice "Steward incidents pending: $pending_line"

  start_brain_deadline_ms=${METASYSTEM_BRAIN_BOOT_DEADLINE_MS:-5000}
  [[ "$start_brain_deadline_ms" =~ ^[1-9][0-9]*$ ]] || start_brain_deadline_ms=5000
  start_brain_wait_sec=$((start_brain_deadline_ms / 1000 + 3))
  start_prepare_brain
  start_prepare_context_object

  if [[ "$start_deferred_identity_read" == true ]]; then
    collect_start_notice "$start_holder_context_notice"
  else
    start_capture arming up_output arming start_up "$runtime" "$session" "$identity_pid" "$identity_started"
    up_aggregate=${up_output##*$'\n'}
    if (( start_arming_status != 0 )); then
      collect_start_notice "Metasystem supervision arming failed: $up_aggregate"
      if [[ "$up_aggregate" == *' re-armed='* ]]; then
        collect_start_notice "Metasystem re-armed the rebuilt engine: $up_aggregate"
      fi
    elif [[ "$up_aggregate" == *' re-armed='* ]]; then
      collect_start_notice "Metasystem re-armed the rebuilt engine: $up_aggregate"
    fi
  fi
  # A revived session may already hold the checkout when process discovery or
  # arming is unavailable. The session-start verb matches this session identifier
  # against the holder's announced session before returning durable wait rows.
  start_capture wait-recovery waiting_lines wait-recovery "$ms" session start --root "$repo" --session "$session"
  if (( start_wait_recovery_status == 0 )); then
    [[ -z "$waiting_lines" ]] || collect_start_notice "$waiting_lines"
  elif (( start_wait_recovery_status != 64 )); then
    waiting_lines=${waiting_lines//$'\n'/'; '}
    collect_start_notice "Metasystem could not read durable wait recovery rows for this session: $waiting_lines"
  else
    : # A non-holder session has no recovery rows to advertise.
  fi
  start_finish_prepared
}

start_up() {
  local start_runtime=$1 start_session=$2 start_pid=$3 start_started=$4
  if [[ "$start_session_absent" == true ]]; then
    METASYSTEM_AGENT_RUNTIME="$start_runtime" "$ms" up --metasystem-root "$world_installation" \
      --repo "$repo" --session "$start_session" --pid "$start_pid" --start-time "$start_started" \
      --tag "${tag:-$start_runtime:$start_pid}" --no-runtime-session --start-source "$start_source"
  else
    METASYSTEM_AGENT_RUNTIME="$start_runtime" "$ms" up --metasystem-root "$world_installation" \
      --repo "$repo" --session "$start_session" --pid "$start_pid" --start-time "$start_started" \
      --tag "${tag:-$start_runtime:$start_pid}" --runtime-session "$start_session" --start-source "$start_source"
  fi
}

if [[ "$event" == start ]]; then
  start_main
  start_finish notice unexpected-termination
fi
# SessionStart cannot pass this dispatcher.

[[ "$runtime" =~ ^[a-z][a-z0-9-]{0,31}$ ]] || exit 2
case "$event" in receipt|stop|end) ;; *) exit 2 ;; esac

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P) || exit 0
harness_root=$(cd "$script_dir/../.." && pwd -P) || exit 0

# Claude Code marks a repeated Stop hook with stop_hook_active. The verdict
# uses that evidence in its bounded three-refusal counter and attempts the
# local steward handoff at the bound. The provider may enforce its own retry
# cap independently of this hook-owned bound.

# The Stop timeout is our sixty-second budget. Registration templates under
# metasystem/scripts/enforcement ship it, adopt.sh installs it into each
# runtime's live settings, and it matches the runtime's own default. This
# parent gives the worker fifty-seven seconds for arming, health, digest,
# watchdog, ledger fetch, and verdict work, then retains three seconds to emit
# a provider-level refusal. This block owns the budget.
if [[ "$event" == stop && "${METASYSTEM_STOP_DEADLINE_PARENT:-}" != "$PPID" ]]; then
  deadline_budget_sec=60
  deadline_worker_sec=$((deadline_budget_sec - 3))
  deadline_started_epoch=$(date -u +%s)
  deadline_dir=
  deadline_dir=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-stop-deadline.XXXXXX" 2>/dev/null) \
    || deadline_dir=$(mktemp -d "/tmp/metasystem-stop-deadline.XXXXXX" 2>/dev/null) \
    || true
  if [[ -z "$deadline_dir" ]]; then
    printf '%s\n' '{"systemMessage":"Task unknown; Stop allowed; needs supervision repair; payload staging failed. The steward must restore supervision. Status unavailable."}'
    exit 0
  fi
  deadline_stdout=$deadline_dir/stdout
  deadline_stderr=$deadline_dir/stderr
  deadline_payload=$deadline_dir/payload
  if ! command cat >"$deadline_payload"; then
    printf '%s\n' '{"systemMessage":"Task unknown; Stop allowed; needs supervision repair; payload staging failed. The steward must restore supervision. Status unavailable."}'
    rm -f "$deadline_stdout" "$deadline_stderr" "$deadline_payload" || true
    rmdir "$deadline_dir" 2>/dev/null || true
    exit 0
  fi
  deadline_started=$SECONDS
  METASYSTEM_STOP_DEADLINE_PARENT=$$ METASYSTEM_STOP_DEADLINE_STARTED=$deadline_started_epoch \
    bash "${BASH_SOURCE[0]}" "$runtime" "$event" \
    <"$deadline_payload" >"$deadline_stdout" 2>"$deadline_stderr" &
  deadline_worker=$!
  deadline_expires=$((deadline_started + deadline_worker_sec))

  deadline_installation=
  deadline_canonical=
  deadline_engine=
  if deadline_installation=$(hook_world_installation); then
    deadline_canonical=$deadline_installation/bin/metasystem
    deadline_engine=${METASYSTEM_BIN:-$deadline_canonical}
    if [[ ! -x "$deadline_canonical" || ! -x "$deadline_engine" ]]; then
      deadline_engine=
    fi
  fi

  # Resolve record coordinates alongside the worker, never ahead of it. The
  # session and root use separate files so either answer can be absent without
  # changing the other, and the ready marker is published after both files.
  deadline_resolution_session=$deadline_dir/resolution.session
  deadline_resolution_root=$deadline_dir/resolution.root
  deadline_resolution_ready=$deadline_dir/resolution.ready
  deadline_resolver=
  if [[ -n "$deadline_engine" ]]; then
    (
      resolver_session=$("$deadline_engine" json get --file "$deadline_payload" --field session_id 2>/dev/null) || resolver_session=
      resolver_root=$("$deadline_engine" path state-root "$deadline_installation" 2>/dev/null) || resolver_root=
      printf '%s' "$resolver_session" >"$deadline_resolution_session"
      printf '%s' "$resolver_root" >"$deadline_resolution_root"
      : >"$deadline_resolution_ready"
    ) &
    deadline_resolver=$!
  fi
  deadline_session=$(sed -n 's/.*"session_id"[[:space:]]*:[[:space:]]*"\([^"\\]*\)".*/\1/p' "$deadline_payload" | head -1)
  [[ -n "$deadline_session" ]] || deadline_session="session-$$"
  deadline_repo=
  deadline_record=
  deadline_record_failure=
  deadline_coordinates_from_engine=false
  deadline_resolve_record() {
    local slug
    [[ -n "$deadline_session" && -n "$deadline_repo" ]] || return 1
    slug=$(printf '%s' "$deadline_session" | tr '[:upper:]' '[:lower:]' |
      sed -E 's/[^a-z0-9._-]+/-/g; s/^[-.]+//; s/[-.]+$//')
    [[ -n "$slug" ]] || slug=session
    deadline_record="$deadline_repo/artifacts/agents/supervision/stop-refusals/$slug.json"
  }
  deadline_log_stop_outcome() {
    local outcome measured_elapsed supervision_dir deadline_now_epoch deadline_elapsed_sec
    [[ -n "${deadline_repo:-}" ]] || return 0
    outcome=$1
    measured_elapsed=${2:-}
    supervision_dir="$deadline_repo/artifacts/agents/supervision"
    mkdir -p "$supervision_dir" 2>/dev/null || true
    if [[ "$measured_elapsed" =~ ^[0-9]+$ ]]; then
      deadline_elapsed_sec=$((10#$measured_elapsed))
    else
      deadline_now_epoch=$(date -u +%s)
      deadline_elapsed_sec=$((deadline_now_epoch - deadline_started_epoch))
      (( deadline_elapsed_sec >= 0 )) || deadline_elapsed_sec=0
    fi
    printf '%s stop response outcome=%s elapsed=%ss\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
      "$outcome" "$deadline_elapsed_sec" >>"$supervision_dir/hooks.log" 2>/dev/null || true
  }
  # One hook-log line per infrastructure condition the parent decides on its
  # own (the worker's output unreadable, the deadline expired). The root is
  # set only after the resolver answers; a condition on an unresolved
  # payload must still emit its allowance, so a failed line is said in the
  # notice, never fatal.
  deadline_log_failure=
  deadline_log_stop_condition() { # cause code, component
    local supervision_root deadline_log deadline_end
    deadline_log_failure=
    deadline_end=$((deadline_started_epoch + deadline_budget_sec))
    supervision_root=${deadline_repo:-}
    deadline_log="$supervision_root/artifacts/agents/supervision/hooks.log"
    if [[ -z "$supervision_root" ]]; then
      deadline_log_failure='the infrastructure stop condition could not be logged: the payload named no checkout'
    elif ! mkdir -p "$(dirname "$deadline_log")" 2>/dev/null; then
      deadline_log_failure='the infrastructure stop condition log directory could not be prepared'
    elif ! printf 'stop-condition infrastructure %s %s - %s degraded-allow\n' "$1" "$2" \
        "$deadline_end" >>"$deadline_log" 2>/dev/null; then
      deadline_log_failure='the infrastructure stop condition could not be appended to the hook log'
    fi
  }
  deadline_capture_engine_coordinates() {
    local resolved_session resolved_root
    [[ "$deadline_coordinates_from_engine" == false && -f "$deadline_resolution_ready" ]] || return 0
    resolved_session=$(command cat "$deadline_resolution_session" 2>/dev/null || true)
    resolved_root=$(command cat "$deadline_resolution_root" 2>/dev/null || true)
    if [[ -n "$resolved_session" ]]; then
      deadline_session=$resolved_session
    fi
    if hook_root_is_one_line "$resolved_root"; then
      deadline_repo=$(cd -- "$resolved_root" 2>/dev/null && pwd -P) || deadline_repo=
    fi
    deadline_record=
    deadline_resolve_record || true
    deadline_coordinates_from_engine=true
  }
  deadline_stop_resolver() {
    local resolver_command
    [[ -n "$deadline_resolver" ]] || return 0
    if kill -0 "$deadline_resolver" 2>/dev/null; then
      resolver_command=$(ps -p "$deadline_resolver" -o command= 2>/dev/null || true)
      if [[ "$resolver_command" == *"${BASH_SOURCE[0]}"* || "$resolver_command" == *supervision-hook.sh* ]]; then
        kill -KILL "$deadline_resolver" 2>/dev/null || true
      fi
    fi
    wait "$deadline_resolver" 2>/dev/null || true
    deadline_resolver=
  }
  deadline_running() {
    kill -0 "$deadline_worker" 2>/dev/null
  }
  while deadline_running && (( SECONDS < deadline_expires )); do
    deadline_capture_engine_coordinates
    sleep 0.05
  done
  deadline_capture_engine_coordinates
  if ! deadline_running; then
    deadline_rc=0
    wait "$deadline_worker" || deadline_rc=$?
    command cat "$deadline_stderr" >&2 || true
    deadline_decision=
    deadline_reason=
    deadline_message=
    deadline_decision_rc=0
    deadline_reason_rc=0
    deadline_message_rc=0
    deadline_shape_rc=0
    deadline_valid=false
    deadline_raw=$(command cat "$deadline_stdout" 2>/dev/null || true)
    if (( deadline_rc == 0 )) && [[ "$deadline_raw" == "$internal_skip_result" ]]; then
      deadline_stop_resolver
      deadline_capture_engine_coordinates
      rm -f "$deadline_stdout" "$deadline_stderr" "$deadline_payload" \
        "$deadline_resolution_session" "$deadline_resolution_root" "$deadline_resolution_ready" || true
      rmdir "$deadline_dir" 2>/dev/null || true
      exit 0
    fi
    if (( deadline_rc == 0 )) && [[ -n "$deadline_engine" ]]; then
      deadline_decision=$("$deadline_engine" json get --file "$deadline_stdout" --field decision 2>/dev/null) \
        || deadline_decision_rc=$?
      deadline_reason=$("$deadline_engine" json get --file "$deadline_stdout" --field reason 2>/dev/null) \
        || deadline_reason_rc=$?
      deadline_message=$("$deadline_engine" json get --file "$deadline_stdout" --field systemMessage 2>/dev/null) \
        || deadline_message_rc=$?
      deadline_unknown=$("$deadline_engine" json strip --file "$deadline_stdout" \
        --key decision --key reason --key systemMessage 2>/dev/null) || deadline_shape_rc=$?
      deadline_reason_object=$("$deadline_engine" json strip --file "$deadline_stdout" \
        --key decision --key systemMessage 2>/dev/null) || deadline_shape_rc=$?
      deadline_message_object=$("$deadline_engine" json strip --file "$deadline_stdout" \
        --key decision --key reason 2>/dev/null) || deadline_shape_rc=$?
      deadline_reason_string=false
      deadline_message_string=false
      if (( deadline_reason_rc == 0 )) && grep -q '^  "reason": "' <<<"$deadline_reason_object"; then
        deadline_reason_string=true
      fi
      if (( deadline_message_rc == 0 )) && grep -q '^  "systemMessage": "' <<<"$deadline_message_object"; then
        deadline_message_string=true
      fi
      if (( deadline_shape_rc == 0 )) && [[ "$deadline_unknown" == '{}' ]] &&
          { [[ "$deadline_decision" == block && -n "$deadline_reason" && "$deadline_reason_string" == true &&
               ( "$deadline_message_rc" -ne 0 || ( -n "$deadline_message" && "$deadline_message_string" == true ) ) ]] ||
            [[ "$deadline_decision_rc" -ne 0 && "$deadline_reason_rc" -ne 0 && -n "$deadline_message" &&
               "$deadline_message_string" == true ]]; }; then
        deadline_valid=true
      fi
    elif (( deadline_rc == 0 )); then
      deadline_raw=$(command cat "$deadline_stdout" 2>/dev/null || true)
      [[ "$deadline_raw" == "$raw_missing_engine_stop" ]] && deadline_valid=true
    fi
    if (( deadline_rc != 0 )) || [[ "$deadline_valid" != true ]]; then
      while [[ ! -f "$deadline_resolution_ready" ]] &&
          [[ -n "$deadline_resolver" ]] && kill -0 "$deadline_resolver" 2>/dev/null &&
          (( SECONDS < deadline_expires )); do
        sleep 0.05
      done
      deadline_capture_engine_coordinates
      deadline_stop_resolver
      deadline_capture_engine_coordinates
      deadline_log_stop_condition stop-hook-output-was-unreadable stop-worker
      deadline_log_stop_outcome invalid-worker-output-allow
      deadline_qualifiers=
      [[ -z "$deadline_log_failure" ]] || deadline_qualifiers='; condition log failed'
      [[ -n "$deadline_repo" ]] || deadline_qualifiers="$deadline_qualifiers; no resolved checkout"
      emit_raw_stop_allowance "$deadline_qualifiers"
    else
      deadline_stop_resolver
      deadline_capture_engine_coordinates
      command cat "$deadline_stdout" || true
    fi
    rm -f "$deadline_stdout" "$deadline_stderr" "$deadline_payload" \
      "$deadline_resolution_session" "$deadline_resolution_root" "$deadline_resolution_ready" || true
    rmdir "$deadline_dir" 2>/dev/null || true
    exit 0
  fi

  # The worker publishes its provider response before recording completion.
  # A complete response is already a safe decision even when completion
  # bookkeeping uses the rest of the Stop budget. Validate it before stopping
  # the worker so a real verdict is never replaced by a deadline refusal.
  deadline_check_published() {
    deadline_published=false
    deadline_published_decision=
    deadline_published_reason=
    deadline_published_message=
    deadline_published_decision_rc=0
    deadline_published_reason_rc=0
    deadline_published_message_rc=0
    deadline_published_shape_rc=0
    if [[ -z "$deadline_engine" || ! -s "$deadline_stdout" ]]; then
      return 0
    fi
    deadline_published_decision=$("$deadline_engine" json get --file "$deadline_stdout" --field decision 2>/dev/null) \
      || deadline_published_decision_rc=$?
    deadline_published_reason=$("$deadline_engine" json get --file "$deadline_stdout" --field reason 2>/dev/null) \
      || deadline_published_reason_rc=$?
    deadline_published_message=$("$deadline_engine" json get --file "$deadline_stdout" --field systemMessage 2>/dev/null) \
      || deadline_published_message_rc=$?
    deadline_published_unknown=$("$deadline_engine" json strip --file "$deadline_stdout" \
      --key decision --key reason --key systemMessage 2>/dev/null) || deadline_published_shape_rc=$?
    deadline_published_reason_object=$("$deadline_engine" json strip --file "$deadline_stdout" \
      --key decision --key systemMessage 2>/dev/null) || deadline_published_shape_rc=$?
    deadline_published_message_object=$("$deadline_engine" json strip --file "$deadline_stdout" \
      --key decision --key reason 2>/dev/null) || deadline_published_shape_rc=$?
    deadline_published_reason_string=false
    deadline_published_message_string=false
    if (( deadline_published_reason_rc == 0 )) && grep -q '^  "reason": "' <<<"$deadline_published_reason_object"; then
      deadline_published_reason_string=true
    fi
    if (( deadline_published_message_rc == 0 )) && grep -q '^  "systemMessage": "' <<<"$deadline_published_message_object"; then
      deadline_published_message_string=true
    fi
    if (( deadline_published_shape_rc == 0 )) && [[ "$deadline_published_unknown" == '{}' ]] &&
        { [[ "$deadline_published_decision" == block && -n "$deadline_published_reason" &&
             "$deadline_published_reason_string" == true ]] ||
          [[ "$deadline_published_decision_rc" -ne 0 && "$deadline_published_reason_rc" -ne 0 &&
             -n "$deadline_published_message" && "$deadline_published_message_string" == true ]]; }; then
      deadline_published=true
    fi
  }
  deadline_check_published

  deadline_stop_resolver
  deadline_capture_engine_coordinates
  deadline_worker_signalled=false
  deadline_worker_waited=false
  deadline_command=$(ps -p "$deadline_worker" -o command= 2>/dev/null || true)
  if [[ "$deadline_command" == *"${BASH_SOURCE[0]}"* || "$deadline_command" == *supervision-hook.sh* ]]; then
    if kill -TERM "$deadline_worker" 2>/dev/null; then
      deadline_worker_signalled=true
    fi
  fi
  for _deadline_stop_attempt in {1..10}; do
    deadline_running || break
    sleep 0.02
  done
  if deadline_running; then
    deadline_command=$(ps -p "$deadline_worker" -o command= 2>/dev/null || true)
    if [[ "$deadline_command" == *"${BASH_SOURCE[0]}"* || "$deadline_command" == *supervision-hook.sh* ]]; then
      if kill -KILL "$deadline_worker" 2>/dev/null; then
        deadline_worker_signalled=true
      fi
    fi
  fi
  if [[ "$deadline_worker_signalled" == true ]] || ! deadline_running; then
    wait "$deadline_worker" 2>/dev/null || true
    deadline_worker_waited=true
    if [[ "$deadline_worker_signalled" == false ]]; then
      command cat "$deadline_stderr" >&2 || true
      deadline_check_published
      if [[ "$deadline_published" == true ]]; then
        command cat "$deadline_stdout" || true
        rm -f "$deadline_stdout" "$deadline_stderr" "$deadline_payload" \
          "$deadline_resolution_session" "$deadline_resolution_root" "$deadline_resolution_ready" || true
        rmdir "$deadline_dir" 2>/dev/null || true
        exit 0
      fi
    fi
  else
    printf 'stop deadline: worker %s left running, command line unverifiable\n' "$deadline_worker" >&2
  fi
  deadline_now_epoch=$(date -u +%s)
  deadline_elapsed_sec=$((deadline_now_epoch - deadline_started_epoch))
  (( deadline_elapsed_sec >= 0 )) || deadline_elapsed_sec=0
  if [[ -n "${deadline_repo:-}" && -n "$deadline_engine" && -x "$deadline_engine" ]]; then
    "$deadline_engine" steward hook-expire --repo "$deadline_repo" \
      --elapsed-sec "$deadline_elapsed_sec" >/dev/null 2>&1 || true
  fi
  if [[ "$deadline_published" == true ]]; then
    command cat "$deadline_stdout" || true
    if [[ "$deadline_worker_waited" == true ]]; then
      rm -f "$deadline_stdout" "$deadline_stderr" "$deadline_payload" \
        "$deadline_resolution_session" "$deadline_resolution_root" "$deadline_resolution_ready" || true
      rmdir "$deadline_dir" 2>/dev/null || true
    fi
    exit 0
  fi
  deadline_cause='stop deadline expired'
  deadline_remedy='A human or steward must restore supervision outside this seat, then retry.'
  deadline_detail='Metasystem Stop deadline expired before a turn verdict; stopping is allowed with degraded infrastructure.'
  deadline_response=
  if [[ -z "$deadline_record" ]]; then
    deadline_record_failure="the stop-refusal record coordinates could not be resolved"
  elif [[ -z "$deadline_engine" ]]; then
    deadline_record_failure="the stop-refusal record engine was unavailable"
  else
    # The refusal command owns the locked occurrence record. Its former public
    # response is deliberately replaced by the one-line unavailable form.
    deadline_response=$("$deadline_engine" report stop-block \
      --class infrastructure \
      --refusal-record "$deadline_record" --session "$deadline_session" \
      --open-work-root "$deadline_repo" \
      --cause "$deadline_cause" --remedy "$deadline_remedy" "$deadline_detail" 2>/dev/null) || \
      deadline_record_failure="the stop-refusal record could not be read or atomically updated"
  fi
  deadline_log_stop_condition stop-deadline-expired stop-deadline
  deadline_qualifiers=
  [[ -z "$deadline_record_failure" ]] || deadline_qualifiers='; record update failed'
  [[ -z "$deadline_log_failure" ]] || deadline_qualifiers="$deadline_qualifiers; condition log failed"
  [[ -n "$deadline_repo" ]] || deadline_qualifiers="$deadline_qualifiers; no resolved checkout"
  if [[ -n "$deadline_record_failure" ]]; then
    deadline_log_stop_outcome deadline-expired-record-failure-allow "$deadline_elapsed_sec"
  else
    deadline_log_stop_outcome deadline-expired-allow "$deadline_elapsed_sec"
  fi
  printf '{"systemMessage":"Task unknown; Stop allowed; needs supervision repair; stop deadline expired%s. The steward must restore supervision. Status unavailable."}\n' "$deadline_qualifiers"
  if [[ "$deadline_worker_waited" == true ]]; then
    rm -f "$deadline_stdout" "$deadline_stderr" "$deadline_payload" \
      "$deadline_resolution_session" "$deadline_resolution_root" "$deadline_resolution_ready" || true
    rmdir "$deadline_dir" 2>/dev/null || true
  fi
  exit 0
fi

stop_started_epoch=${METASYSTEM_STOP_DEADLINE_STARTED:-}
if [[ "$stop_started_epoch" =~ ^[0-9]+$ ]]; then
  stop_started_epoch=$((10#$stop_started_epoch))
else
  stop_started_epoch=$(date -u +%s)
fi

intentional_hook_skip() {
  if [[ "$event" == stop && "${METASYSTEM_STOP_DEADLINE_PARENT:-}" == "$PPID" ]]; then
    printf '%s\n' "$internal_skip_result"
  fi
  exit 0
}

# A job-bound adapter supplies evidence coordinates before launching any
# provider child. The coordinates never authorize a skip: the launcher's
# engine must match this live ancestry to that exact job's recorded custody.
delegate_state_hint=${METASYSTEM_HOOK_DELEGATE_STATE_ROOT:-}
delegate_installation_hint=${METASYSTEM_HOOK_DELEGATE_INSTALLATION_ROOT:-}
delegate_job_hint=${METASYSTEM_HOOK_DELEGATE_JOB:-}
if [[ -n "$delegate_state_hint$delegate_installation_hint$delegate_job_hint" ]]; then
  if [[ -z "$delegate_state_hint" || -z "$delegate_installation_hint" || -z "$delegate_job_hint" ||
        ! -x "$delegate_installation_hint/bin/metasystem" ]]; then
    echo "supervision hook refused: delegate context hint is incomplete or its engine is unavailable" >&2
    exit 1
  fi
  delegate_hint_rc=0
  delegate_hint_result=$("$delegate_installation_hint/bin/metasystem" lease hook-delegate \
    --root "$delegate_state_hint" --metasystem-root "$delegate_installation_hint" \
    --job "$delegate_job_hint" --caller-pid "$PPID" 2>/dev/null) || delegate_hint_rc=$?
  if (( delegate_hint_rc == 0 )) && [[ "$delegate_hint_result" == *'"delegate":true'* ]]; then
    intentional_hook_skip
  fi
  echo "supervision hook refused: supplied delegate context did not authenticate this process ancestry" >&2
  exit 1
fi
# Worktree mapping locates the installation before executable resolution. The
# installation's own engine proves its provenance even when an override runs.
world_installation=$(hook_world_installation) || exit 0
canonical=$world_installation/bin/metasystem
ms=${METASYSTEM_BIN:-$canonical}
if [[ ! -x "$canonical" || ! -x "$ms" ]]; then
  if [[ "$event" == stop ]]; then
    printf '%s\n' "$raw_missing_engine_stop"
  elif [[ "$event" == start ]]; then
    printf '%s\n' '{"systemMessage":"Metasystem engine missing: this session received no role context; if this checkout is a declared brain it is uninstructed until the engine is rebuilt: run scripts/agents/go-build.sh, then start a new session"}'
  fi
  exit 0
fi
registered_runtimes=$("$ms" runtime list) || {
  runtime_list_rc=$?
  echo "supervision hook refused: runtime registry query failed (exit $runtime_list_rc)" >&2
  exit "$runtime_list_rc"
}
grep -Fxq "$runtime" <<<"$registered_runtimes" || {
  echo "supervision hook refused: runtime '$runtime' is not registered" >&2
  exit 2
}

payload=$(mktemp "${TMPDIR:-/tmp}/metasystem-supervision-hook.XXXXXX")
stop_work_dir=
if [[ "$event" == stop ]]; then
  stop_work_dir=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-stop-presentation.XXXXXX") || {
    printf '%s\n' '{"systemMessage":"Task unknown; Stop allowed; needs supervision repair; payload staging failed. The steward must restore supervision. Status unavailable."}'
    exit 0
  }
fi
trap 'rm -f "$payload"; [[ -z "$stop_work_dir" ]] || rm -rf "$stop_work_dir"' EXIT
cat >"$payload"
if [[ "$event" == stop ]]; then
  # Missing optional payload fields retain their documented fallbacks, but an
  # unreadable payload must not be mistaken for an empty one at turn end.
  "$ms" json get --file "$payload" --field __metasystem_shape_probe --default "" >/dev/null
fi

read_payload() {
  "$ms" json get --file "$payload" --field "$1" 2>/dev/null || true
}

repo_rc=0
repo=$("$ms" path state-root "$world_installation" 2>/dev/null) || repo_rc=$?
if (( repo_rc == 1 )); then
  exit 0
elif (( repo_rc != 0 )) || ! hook_root_is_one_line "$repo"; then
  if [[ "$event" == stop ]]; then
    printf '%s\n' "$raw_engine_skew_stop"
  fi
  exit 0
fi
repo=$(cd -- "$repo" 2>/dev/null && pwd -P) || exit 0
session=$(read_payload session_id)
transcript_path=$("$ms" json get --file "$payload" --field transcript_path --default '' --shell-safe 2>/dev/null || true)
session_absent=false
if [[ -z "$session" ]]; then
  session_absent=true
  session="session-$PPID"
fi
stop_hook_active=false
if [[ "$event" == stop && "$(read_payload stop_hook_active)" == true ]]; then
  stop_hook_active=true
fi
# Session hygiene happens ONCE at this boundary (goal-system GOAL-04):
# the runtime's string is untrusted input; anything not matching the safe
# shape becomes its sha256 hex, and every downstream use rides the result.
if ! [[ "$session" =~ ^[A-Za-z0-9._-]{1,128}$ ]]; then
  session=$(printf '%s' "$session" | "$ms" util sha256)
fi
runtime_session_args=(--runtime-session "$session")
if [[ "$session_absent" == true ]]; then
  runtime_session_args=(--no-runtime-session)
fi

report_turn_verdict() {
  if [[ "$session_absent" == true ]]; then
    "$ms" report turn-verdict --session-absent "$@"
  else
    "$ms" report turn-verdict "$@"
  fi
}

hook_generation=
hook_attempt_seq=
health_line=
checkin_tail=
digest_message=
digest_cursor=
digest_prefix=
hook_evidence_failure=
stop_failure=
stop_failure_component=
stop_failure_code=
facts_failure=
verdict_file=
facts_file=
extras=
stop_conditions=()
stop_cause_code() {
  printf '%s' "$1" | tr '[:upper:]' '[:lower:]' | sed -E 's/[^a-z0-9]+/-/g; s/^-+//; s/-+$//; s/^the-//'
}
record_stop_failure() { # fixed diagnostic, component
  local code
  code=$(stop_cause_code "$1")
  stop_conditions+=("$code|$2")
  if [[ -z "$stop_failure" ]]; then
    stop_failure=$1
    stop_failure_component=$2
    stop_failure_code=$code
  fi
}

# Runtime signatures are anchored on the executable, so an intermediate
# `/bin/sh -c` does not impersonate the runtime merely because its arguments
# name this hook. Start at the immediate parent and let the process owner walk.
# The adapter declarations belong to the installation, not the checkout.
local_delegate_rc=0
local_delegate=$("$ms" lease hook-delegate --root "$repo" --metasystem-root "$world_installation" \
  --caller-pid "$PPID" 2>/dev/null) || local_delegate_rc=$?
if (( local_delegate_rc == 0 )) && [[ "$local_delegate" == *'"delegate":true'* ]]; then
  intentional_hook_skip
elif (( local_delegate_rc != 0 && local_delegate_rc != 3 )); then
  echo "supervision hook refused: local delegate custody evidence was unreadable" >&2
  exit 1
fi

if [[ "$runtime" == fake ]]; then
  # The synthetic fixture runtime is deliberately outside the adoptable-host
  # registry. Keep its existing exact signature and fixtureauth ancestry path;
  # real provider hooks continue to discover foreign hosts through --all-hosts.
  identity=$("$ms" proc find-ancestor --repo "$world_installation" --pid "$PPID" --runtime fake 2>/dev/null || true)
else
  identity=$("$ms" proc find-ancestor --repo "$world_installation" --pid "$PPID" --all-hosts 2>/dev/null || true)
fi
main_id=
main_class=
main_holder=false
seat_machine=
seat_lineage=
seat_claim_epoch=0
identity_pid=
identity_started=
if [[ -n "$identity" ]]; then
  identity_runtime=$("$ms" json get --value "$identity" --field runtime 2>/dev/null || true)
  if [[ -z "$identity_runtime" ]]; then
    record_stop_failure "the runtime identity was unreadable" runtime-identity
    identity=
  elif [[ "$identity_runtime" != "$runtime" ]]; then
    intentional_hook_skip
  fi
  if [[ -n "$identity" ]]; then
    identity_pid=$("$ms" json get --value "$identity" --field pid 2>/dev/null || true)
    identity_started=$("$ms" json get --value "$identity" --field pidStartedAt 2>/dev/null || true)
    if ! [[ "$identity_pid" =~ ^[1-9][0-9]*$ && "$identity_started" =~ ^[1-9][0-9]*$ ]]; then
      record_stop_failure "the runtime identity was unreadable" runtime-identity
      identity=
      identity_pid=
      identity_started=
    fi
  fi
else
  # Recorded fallback: a hook may run in a test harness or runtime wrapper
  # whose authenticated main was announced explicitly. Classification returns
  # that exact announcement; an unannounced process gains nothing here.
  parent_view_rc=0
  parent_view=$("$ms" lease classify --root "$repo" --metasystem-root "$world_installation" --caller-pid "$PPID" 2>/dev/null) || parent_view_rc=$?
  parent_class=$("$ms" json get --value "$parent_view" --field class 2>/dev/null || true)
  if (( parent_view_rc != 0 )) || [[ -z "$parent_class" ]]; then
    record_stop_failure "the fallback runtime identity could not be classified" runtime-identity
  elif [[ "$parent_class" == MAIN ]]; then
    parent_runtime=$("$ms" json get --value "$parent_view" --field announcement.runtime 2>/dev/null || true)
    if [[ -z "$parent_runtime" ]]; then
      record_stop_failure "the fallback runtime identity was unreadable" runtime-identity
    elif [[ "$parent_runtime" != "$runtime" ]]; then
      intentional_hook_skip
    else
      identity_pid=$("$ms" json get --value "$parent_view" --field announcement.pid 2>/dev/null || true)
      identity_started=$("$ms" json get --value "$parent_view" --field announcement.pidStartedAt 2>/dev/null || true)
      [[ "$identity_pid" =~ ^[1-9][0-9]*$ && "$identity_started" =~ ^[1-9][0-9]*$ ]] \
        && identity=recorded-main
      [[ -n "$identity" ]] || record_stop_failure "the fallback runtime identity was unreadable" runtime-identity
    fi
  fi
fi

# SessionStart is one relay object. Every existing start notice is collected
# here; the engine alone decides whether a brain payload exists.
start_notices=
start_context_field=
start_context_event=
start_context_payload=
brain_digest_emitted=false
brain_digest_cursor=
brain_digest_prefix=
collect_start_notice() { # message
  start_notices=$(printf '%s%s%s' "$start_notices" "${start_notices:+$'\n'}" "$1")
}

emit_start_payload() {
  local response context_json event_json key rest rendered
  [[ "$event" == start ]] || return 0
  if [[ -n "$start_notices" ]]; then
    response=$("$ms" json object "systemMessage=$start_notices") || return 1
  else
    response='{}'
  fi
  if [[ -n "$start_context_payload" && -n "$start_context_field" ]]; then
    rest=$start_context_field
    key=${rest##*.}
    context_json=$("$ms" json object "$key=$start_context_payload") || return 1
    rest=${rest%.*}
    if [[ -n "$start_context_event" ]]; then
      event_json=$("$ms" json object "hookEventName=$start_context_event") || return 1
      context_json="${context_json%\}},${event_json#\{}"
    fi
    while [[ -n "$rest" ]]; do
      key=${rest##*.}
      context_json="{\"$key\":$context_json}"
      [[ "$rest" == *.* ]] || break
      rest=${rest%.*}
    done
    if [[ "$response" == '{}' ]]; then
      response=$context_json
    else
      rendered=${context_json#\{}
      response="${response%\}},$rendered"
    fi
  fi
  printf '%s\n' "$response"
  if [[ "$brain_digest_emitted" == true && "$brain_digest_cursor" =~ ^[0-9]+$ &&
        "$brain_digest_prefix" =~ ^[0-9a-f]{64}$ ]]; then
    "$ms" brain digest-advance --root "$repo" --repo "$repo" \
      --cursor "$brain_digest_cursor" --prefix-sha256 "$brain_digest_prefix" >/dev/null 2>&1 || \
      echo "supervision hook: emitted the brain digest but could not advance its cursor" >&2
  fi
}

surface_json() { # message
  local rendered parsed
  if [[ "$event" == start ]]; then
    collect_start_notice "$1"
    return 0
  fi
  rendered=$("$ms" json object "systemMessage=$1")
  parsed=$("$ms" json get --value "$rendered" --field systemMessage)
  [[ -n "$rendered" && -n "$parsed" ]] || return 1
  printf '%s\n' "$rendered"
}

if [[ "$event" == start ]]; then
  start_context_rc=0
  start_context_decl=$("$ms" runtime start-context "$runtime" 2>/dev/null) || start_context_rc=$?
  if (( start_context_rc == 0 )); then
    start_context_field=${start_context_decl#field=}
    start_context_field=${start_context_field%% event=*}
    start_context_event=${start_context_decl#* event=}
    start_context_event=${start_context_event%% bytes=*}
    start_context_bytes=${start_context_decl#* bytes=}
    start_context_bytes=${start_context_bytes%% sources=*}
    if ! [[ "$start_context_field" =~ ^[A-Za-z][A-Za-z0-9]*(\.[A-Za-z][A-Za-z0-9]*)+$ &&
          "$start_context_event" =~ ^[A-Za-z][A-Za-z0-9]{0,63}$ &&
          "$start_context_bytes" =~ ^[0-9]+$ && "$start_context_bytes" -ge 2048 ]]; then
      start_context_rc=2
    fi
  elif (( start_context_rc == 1 )); then
    start_context_field=
    start_context_event=
    start_context_bytes=2048
  fi

  brain_boot_dir=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-brain-boot-hook.XXXXXX") || exit 1
  brain_boot_out=$brain_boot_dir/stdout
  brain_boot_err=$brain_boot_dir/stderr
  brain_boot_rc=0
  brain_boot_timeout=false
  if (( start_context_rc > 1 )); then
    brain_boot_rc=$start_context_rc
    printf '%s\n' 'runtime start-context declaration unreadable' >"$brain_boot_err"
  else
    # The boot deadline is 5 s unless the environment lowers it (fixtures do,
    # so a boot that must outlast the deadline costs seconds, not half a
    # minute); the hook grants three seconds of grace beyond it.
    brain_boot_deadline_ms=${METASYSTEM_BRAIN_BOOT_DEADLINE_MS:-5000}
    [[ "$brain_boot_deadline_ms" =~ ^[1-9][0-9]*$ ]] || brain_boot_deadline_ms=5000
    brain_boot_wait_sec=$(( brain_boot_deadline_ms / 1000 + 3 ))
    "$ms" brain boot --root "$repo" --repo "$repo" --bytes "$start_context_bytes" \
      --deadline-ms "$brain_boot_deadline_ms" >"$brain_boot_out" 2>"$brain_boot_err" &
    brain_boot_pid=$!
    brain_boot_started=$SECONDS
    while kill -0 "$brain_boot_pid" 2>/dev/null && (( SECONDS - brain_boot_started < brain_boot_wait_sec )); do
      sleep 0.05
    done
    if kill -0 "$brain_boot_pid" 2>/dev/null; then
      brain_boot_timeout=true
      brain_boot_command=$(ps -p "$brain_boot_pid" -o command= 2>/dev/null || true)
      if [[ "$brain_boot_command" == *"$ms"* ]]; then
        kill -TERM "$brain_boot_pid" 2>/dev/null || true
        sleep 0.2
        if kill -0 "$brain_boot_pid" 2>/dev/null; then
          brain_boot_command=$(ps -p "$brain_boot_pid" -o command= 2>/dev/null || true)
          [[ "$brain_boot_command" == *"$ms"* ]] && kill -KILL "$brain_boot_pid" 2>/dev/null || true
        fi
      fi
    fi
    wait "$brain_boot_pid" || brain_boot_rc=$?
  fi

  brain_boot_valid=false
  brain_declared=
  if [[ "$brain_boot_timeout" == false && "$brain_boot_rc" -eq 0 && -s "$brain_boot_out" ]]; then
    brain_declared=$("$ms" json get --file "$brain_boot_out" --field declared 2>/dev/null || true)
    brain_unknown=$("$ms" json strip --file "$brain_boot_out" --key declared --key state \
      --key payload --key bytes --key sections --key digestEmitted --key digestCursor \
      --key digestPrefixSha256 2>/dev/null || true)
    if [[ "$brain_unknown" == '{}' && "$brain_declared" == false ]]; then
      brain_false_unknown=$("$ms" json strip --file "$brain_boot_out" --key declared 2>/dev/null || true)
      [[ "$brain_false_unknown" == '{}' ]] && brain_boot_valid=true
    elif [[ "$brain_unknown" == '{}' && "$brain_declared" == true ]]; then
      brain_state=$("$ms" json get --file "$brain_boot_out" --field state 2>/dev/null || true)
      brain_bytes=$("$ms" json get --file "$brain_boot_out" --field bytes 2>/dev/null || true)
      brain_digest_emitted=$("$ms" json get --file "$brain_boot_out" --field digestEmitted 2>/dev/null || true)
      brain_digest_cursor=$("$ms" json get --file "$brain_boot_out" --field digestCursor 2>/dev/null || true)
      brain_digest_prefix=$("$ms" json get --file "$brain_boot_out" --field digestPrefixSha256 2>/dev/null || true)
      brain_payload_with_sentinel=$("$ms" json get --file "$brain_boot_out" --field payload 2>/dev/null; printf x) || true
      brain_payload=${brain_payload_with_sentinel%x}
      brain_payload=${brain_payload%$'\n'}
      brain_sections_valid=true
      for brain_section in asks held fleet digest; do
        brain_section_state=$("$ms" json get --file "$brain_boot_out" --field "sections.$brain_section" 2>/dev/null || true)
        case "$brain_section_state" in complete|cut|skipped|error) ;; *) brain_sections_valid=false ;; esac
      done
      if [[ ( "$brain_state" == declared || "$brain_state" == corrupt ) &&
            "$brain_bytes" =~ ^[0-9]+$ && "$brain_digest_cursor" =~ ^[0-9]+$ &&
            ( "$brain_digest_prefix" =~ ^[0-9a-f]{64}$ || ( "$brain_digest_emitted" == false && -z "$brain_digest_prefix" ) ) &&
            ( "$brain_digest_emitted" == true || "$brain_digest_emitted" == false ) &&
            "$brain_sections_valid" == true && -n "$brain_payload" ]]; then
        brain_boot_valid=true
        if [[ -n "$start_context_field" ]]; then
          start_context_payload=$brain_payload
        else
          screen_only='this runtime has no session-context channel; the packet reached the screen only'
          # The screen-only channel is exactly 2,048 bytes. The engine's
          # minimum preserves phase one at the front, so any necessary cut
          # removes optional tail text only.
          start_notices="$screen_only
$brain_payload"
      start_notice_bytes=$(printf '%s' "$start_notices" | wc -c | tr -d ' ')
      if (( start_notice_bytes > 2048 )); then
        start_notices=$(printf '%s' "$start_notices" | LC_ALL=C head -c 2048)
      fi
        fi
      fi
    fi
  fi
  if [[ "$brain_boot_valid" != true ]]; then
    if [[ "$brain_boot_timeout" == true ]]; then
      brain_boot_result=timeout
    else
      brain_boot_result="exit $brain_boot_rc"
    fi
    brain_boot_tail=$(tail -c 500 "$brain_boot_err" 2>/dev/null | tr '\r\n' '  ' || true)
    failure="Metasystem brain boot failed ($brain_boot_result; stderr tail: $brain_boot_tail): this session received no role context; if this checkout is a declared brain it is uninstructed: run metasystem brain boot --root $repo --repo $repo by hand and rebuild if it fails"
    collect_start_notice "$failure"
  fi
  rm -f "$brain_boot_out" "$brain_boot_err" || true
  rmdir "$brain_boot_dir" 2>/dev/null || true
fi

# Only after runtime and delegate context are decided may Stop publish its
# attempt evidence. The parent treats no other empty or malformed result as a
# skip, preserving the existing fail-closed deadline behavior.
if [[ "$event" == stop ]]; then
  turn_key_rc=0
  turn_key=$({ printf '%s\n' "$session"; command cat "$payload"; } | "$ms" util sha256) || turn_key_rc=$?
  if (( turn_key_rc != 0 )) || [[ -z "$turn_key" ]]; then
    hook_evidence_failure="HEALTH unknown — hook-freshness=unknown (turn evidence could not be prepared)"
    record_stop_failure "turn evidence could not be prepared" turn-evidence
  else
    hook_attempt_rc=0
    hook_attempt=$("$ms" steward hook-attempt --repo "$repo" --pid "$$" --turn-key "$turn_key" 2>/dev/null) || hook_attempt_rc=$?
    if (( hook_attempt_rc != 0 )) || [[ -z "$hook_attempt" ]]; then
      hook_evidence_failure="HEALTH unknown — hook-freshness=unknown (attempt evidence could not be recorded)"
      record_stop_failure "attempt evidence could not be recorded" turn-evidence
    else
      hook_generation=$("$ms" json get --value "$hook_attempt" --field generation 2>/dev/null || true)
      hook_attempt_seq=$("$ms" json get --value "$hook_attempt" --field attemptSeq 2>/dev/null || true)
      if ! [[ "$hook_generation" =~ ^[1-9][0-9]*$ && "$hook_attempt_seq" =~ ^[1-9][0-9]*$ ]]; then
        hook_evidence_failure="HEALTH unknown — hook-freshness=unknown (attempt evidence was unreadable)"
        record_stop_failure "attempt evidence was unreadable" turn-evidence
      fi
    fi
  fi
fi
if [[ -n "$identity_pid" ]]; then
  lease_view_rc=0
  lease_view=$("$ms" lease classify --root "$repo" --metasystem-root "$world_installation" --caller-pid "$identity_pid" 2>/dev/null) || lease_view_rc=$?
  if (( lease_view_rc != 0 )) || [[ -z "$lease_view" ]]; then
    record_stop_failure "the checkout holder could not be classified" checkout-holder
  else
    main_id=$("$ms" json get --value "$lease_view" --field mainId 2>/dev/null || true)
    main_class=$("$ms" json get --value "$lease_view" --field class 2>/dev/null || true)
    main_holder=$("$ms" json get --value "$lease_view" --field holder 2>/dev/null || true)
    seat_claim_epoch=$("$ms" json get --value "$lease_view" --field claimEpoch --default 0 2>/dev/null || true)
    seat_lineage=$("$ms" json get --value "$lease_view" --field announcement.ownerLineage --default '' 2>/dev/null || true)
    [[ -n "$seat_lineage" ]] || seat_lineage=$main_id
    seat_machine=$(hook_git -C "$repo" config --get metasystem.goal.machine 2>/dev/null || true)
    if [[ -z "$main_class" || ( "$main_holder" != true && "$main_holder" != false ) ]]; then
      record_stop_failure "the checkout holder classification was unreadable" checkout-holder
    fi
  fi
fi

stop_block_json() { # system message, reason, bounded idle
  local rendered decision reason
  if [[ "${3:-false}" == true ]]; then
    rendered=$("$ms" report stop-block --bounded-idle --system-message "$1" "$2")
  else
    rendered=$("$ms" report stop-block --system-message "$1" "$2")
  fi
  decision=$("$ms" json get --value "$rendered" --field decision)
  reason=$("$ms" json get --value "$rendered" --field reason)
  [[ "$decision" == block && -n "$reason" ]] || return 1
  printf '%s\n' "$rendered"
}

external_stop_json() { # system message, reason, cause, remedy
  "$ms" report stop-block --class infrastructure --system-message "$1" \
    --refusal-record "$stop_refusal_record" --session "$session" \
    --cause "$3" --remedy "$4" --arming-result "$up_failure_result" "$2"
}

if [[ "$event" == receipt ]]; then
  receipt_rc=0
  bash "$script_dir/../receipt.sh" check >/dev/null 2>&1 || receipt_rc=$?
  if (( receipt_rc == 1 )); then
    surface_json "Metasystem retro due: run scripts/receipt.sh check for details, then skills/retro."
  elif (( receipt_rc != 0 )); then
    surface_json "Metasystem receipt check errored; run scripts/receipt.sh check to see why."
  fi
  exit 0
fi

tag="metasystem-main-$runtime-$("$ms" util slug "$session")"
up_failure=
up_failure_result=
up_notice=
if [[ "$event" == stop ]]; then
  up_rc=0
  arming_stderr=$stop_work_dir/arming.stderr
  if [[ -n "$identity_pid" ]]; then
    up_output=$(METASYSTEM_AGENT_RUNTIME="$runtime" "$ms" up --metasystem-root "$world_installation" \
      --repo "$repo" --session "$session" --pid "$identity_pid" --start-time "$identity_started" \
      --tag "$tag" "${runtime_session_args[@]}" 2>"$arming_stderr") || up_rc=$?
  else
    # A Stop call with no session identity still drives the restricted verify
    # and recovery path. It gains no announcement or checkout lease authority.
    up_output=$(METASYSTEM_AGENT_RUNTIME="$runtime" "$ms" up --metasystem-root "$world_installation" \
      --repo "$repo" --recover-only --if-down "${runtime_session_args[@]}" 2>"$arming_stderr") || up_rc=$?
  fi
  up_aggregate=$(printf '%s' "$up_output" | tail -1)
  if [[ "$up_aggregate" == *" re-armed="* ]]; then
    up_notice="Metasystem re-armed the rebuilt engine: $up_aggregate"
  elif [[ "$up_aggregate" == "up outcome=stopped"* ]]; then
    up_stopped_component=$(printf '%s\n' "$up_output" | sed -n 's/^component=stopped outcome=standing detail="\(.*\)"$/\1/p' | tail -1)
    up_stopped_remedy=$(printf '%s\n' "$up_aggregate" | sed -n 's/^up outcome=stopped remedy="\(.*\)"$/\1/p')
    if [[ "$up_stopped_component" == "stop incomplete "* || "$up_stopped_component" == "stop unfinished "* ]]; then
      up_notice="Metasystem $up_stopped_component; run: $up_stopped_remedy."
    else
      up_notice="Metasystem is stopped for this checkout; start again with $up_stopped_remedy."
    fi
  fi
  if (( up_rc != 0 )); then
    up_component_lines=$(printf '%s\n' "$up_output" | sed -n '/^component=/p')
    up_failure_result="$up_component_lines${up_component_lines:+$'\n'}$up_aggregate"
    arming_diagnostic=$(command cat "$arming_stderr" 2>/dev/null || true)
    [[ -z "$arming_diagnostic" ]] || up_failure_result="$up_failure_result${up_failure_result:+$'\n'}$arming_diagnostic"
    up_failure="Metasystem supervision arming failed:
$up_failure_result"
    record_stop_failure "supervision arming failed" supervision-arming
  fi
  arming_capture=$stop_work_dir/arming.txt
  printf '%s' "$up_output" >"$arming_capture" || record_stop_failure "the supervision arming result could not be captured" supervision-arming
  health_rc=0
  health_capture=$stop_work_dir/health.json
  "$ms" health --hook-preview --format=json --repo "$repo" --metasystem-root "$world_installation" >"$health_capture" 2>/dev/null || health_rc=$?
  health_line=$("$ms" json get --file "$health_capture" --field line 2>/dev/null || true)
  if (( health_rc > 2 )) || [[ -z "$health_line" ]]; then
    health_line="HEALTH unknown — hook-freshness=unknown (the health engine returned no verdict)"
    record_stop_failure "the health engine returned no verdict" health
  fi
  digest_rc=0
  digest_json=$("$ms" steward digest-pending --repo "$repo" 2>&1) || digest_rc=$?
  if (( digest_rc == 0 )); then
    digest_message_rc=0
    digest_cursor_rc=0
    digest_prefix_rc=0
    digest_message=$("$ms" json get --value "$digest_json" --field message 2>/dev/null) || digest_message_rc=$?
    digest_cursor=$("$ms" json get --value "$digest_json" --field cursor 2>/dev/null) || digest_cursor_rc=$?
    digest_prefix=$("$ms" json get --value "$digest_json" --field prefixSha256 2>/dev/null) || digest_prefix_rc=$?
    if (( digest_message_rc != 0 || digest_cursor_rc != 0 || digest_prefix_rc != 0 )); then
      digest_message="NARRATOR DIGEST unavailable: the digest state was unreadable"
      record_stop_failure "the narrator digest state was unreadable" narrator
    fi
  else
    digest_message="NARRATOR DIGEST unavailable: ${digest_json//$'\n'/ }"
    record_stop_failure "the narrator digest could not be read" narrator
  fi
  checkin_tail=$health_line
  [[ -z "$up_notice" ]] || checkin_tail="$up_notice
$checkin_tail"
  [[ -z "$digest_message" ]] || checkin_tail="$checkin_tail
$digest_message"
  digest_capture=$stop_work_dir/digest.txt
  printf '%s' "$digest_message" >"$digest_capture" || record_stop_failure "the narrator digest could not be captured" narrator
  receipt_capture=$stop_work_dir/receipt.txt
  receipt_stderr=$stop_work_dir/receipt.stderr
  receipt_rc=0
  bash "$script_dir/../receipt.sh" check >"$receipt_capture" 2>"$receipt_stderr" || receipt_rc=$?
fi

complete_stop_attempt() { # completion arguments
  local completion_rc=0
  "$ms" steward hook-complete "$@" --elapsed-sec "$stop_elapsed_sec" >/dev/null 2>&1 || completion_rc=$?
  if (( completion_rc == 2 )); then
    completion_rc=0
    "$ms" steward hook-complete "$@" >/dev/null 2>&1 || completion_rc=$?
  fi
  return "$completion_rc"
}

stop_delivery_ready=false
presentation_file=
fallback_stop_payload() { # retained control
  if [[ "$1" == true ]]; then
    response=$("$ms" json object decision=block reason='Task unknown; Stop blocked; needs supervision repair; Status unavailable.')
  else
    response=$("$ms" json object systemMessage='Task unknown; Stop allowed; needs supervision repair; Status unavailable.')
  fi
  stop_delivery_ready=false
}

present_stop_payload() { # retained shouldBlock, optional judgment-free advisor mode
  local retained_block=$1 advisor_mode=${2:-false}
  local input_file mapped_file failure_file notice_file attempt_token compose_rc present_rc map_rc
  local stop_input_args
  stop_delivery_ready=false
  if [[ "$advisor_mode" != true && ! -s "$verdict_file" ]]; then
    fallback_stop_payload "$retained_block"
    return 0
  fi
  attempt_token=$("$ms" util token-hex --bytes 16 2>/dev/null || true)
  if ! [[ "$attempt_token" =~ ^[0-9a-f]{32}$ ]]; then
    fallback_stop_payload "$retained_block"
    return 0
  fi
  input_file=$stop_work_dir/presentation-input.json
  presentation_file=$stop_work_dir/presentation.json
  mapped_file=$stop_work_dir/provider-output.json
  failure_file=$stop_work_dir/failures.txt
  notice_file=$stop_work_dir/notices.txt
  printf '%s' "$stop_failure" >"$failure_file" || true
  printf '%s' "${extras:-}" >"$notice_file" || true
  stop_input_args=(report stop-input --root "$repo" --runtime "$runtime" --session "$session"
    --attempt "$attempt_token" --main-id "$main_id" --machine "$seat_machine" \
    --lineage "$seat_lineage" --claim-epoch "$seat_claim_epoch")
  if [[ "$advisor_mode" == true ]]; then
    stop_input_args+=(--advisor)
  else
    stop_input_args+=(--verdict-file "$verdict_file" --facts-file "$facts_file" --completion-file "$completion_file")
  fi
  stop_input_args+=(--health-file "$health_capture" \
    --digest-file "$digest_capture" --digest-cursor-prefix "$digest_prefix" \
    --receipt-file "$receipt_capture" --receipt-stderr-file "$receipt_stderr" --receipt-exit "$receipt_rc" \
    --arming-file "$arming_capture" --arming-stderr-file "$arming_stderr" --arming-exit "$up_rc" \
    --notice-file "$notice_file" --failure-file "$failure_file" --output-file "$input_file")
  compose_rc=0
  "$ms" "${stop_input_args[@]}" 2>>"$supervision_dir/hooks.log" || compose_rc=$?
  if (( compose_rc != 0 )); then
    fallback_stop_payload "$retained_block"
    return 0
  fi
  present_rc=0
  "$ms" report stop-present --root "$repo" --input-file "$input_file" --output-file "$presentation_file" \
    2>>"$supervision_dir/hooks.log" || present_rc=$?
  if (( present_rc != 0 )); then
    fallback_stop_payload "$retained_block"
    return 0
  fi
  map_rc=0
  "$ms" adapter stop-output --runtime "$runtime" --input-file "$presentation_file" --output-file "$mapped_file" \
    2>>"$supervision_dir/hooks.log" || map_rc=$?
  if (( map_rc != 0 )); then
    fallback_stop_payload "$retained_block"
    return 0
  fi
  response=$(command cat "$mapped_file")
  stop_delivery_ready=true
}

emit_stop_payload() { # response
  response=$1
  stop_now_epoch=$(date -u +%s)
  stop_elapsed_sec=$((stop_now_epoch - stop_started_epoch))
  (( stop_elapsed_sec >= 0 )) || stop_elapsed_sec=0
  stop_decision=$("$ms" json get --value "$response" --field decision 2>/dev/null || true)
  [[ -n "$stop_decision" ]] || stop_decision=allow
  # The evidence trail sits beside the rest of the supervision state.
  supervision_dir="$repo/artifacts/agents/supervision"
  mkdir -p "$supervision_dir" 2>/dev/null || true
  printf '%s stop response decision=%s elapsed=%ss\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    "$stop_decision" "$stop_elapsed_sec" >>"$supervision_dir/hooks.log" 2>/dev/null || true
  response_file_rc=0
  response_file=$(mktemp "${TMPDIR:-/tmp}/metasystem-supervision-response.XXXXXX") || response_file_rc=$?
  if (( response_file_rc != 0 )) || [[ -z "$response_file" ]]; then
    command printf '%s\n' "$response" || true
    complete_stop_attempt --repo "$repo" --generation "$hook_generation" \
      --attempt "$hook_attempt_seq" --result ERROR --outcome PAYLOAD_STAGE_FAILED \
      || true
    return 0
  fi
  if ! printf '%s\n' "$response" >"$response_file"; then
    command printf '%s\n' "$response" || true
    complete_stop_attempt --repo "$repo" --generation "$hook_generation" \
      --attempt "$hook_attempt_seq" --result ERROR --outcome PAYLOAD_STAGE_FAILED \
      || true
    rm -f "$response_file"
    return 0
  fi
  if ! command printf '%s\n' "$response"; then
    complete_stop_attempt --repo "$repo" --generation "$hook_generation" \
      --attempt "$hook_attempt_seq" --result ERROR --outcome EMISSION_FAILED \
      --health-line "$health_line" --payload-file "$response_file" \
      || true
    rm -f "$response_file"
    return 0
  fi
  if [[ "$stop_delivery_ready" == true && -n "$digest_message" && "$digest_cursor" =~ ^[0-9]+$ && "$digest_prefix" =~ ^[0-9a-f]{64}$ ]]; then
    if ! "$ms" steward digest-advance --repo "$repo" --cursor "$digest_cursor" \
      --prefix-sha256 "$digest_prefix" >/dev/null 2>&1; then
      echo "supervision hook: emitted the narrator digest but could not advance its check-in cursor" >&2
    fi
  fi
  if [[ "$stop_delivery_ready" != true ]]; then
    # Presentation failure is component evidence; the retained verdict still
    # decides whether this Stop is blocked or allowed.
    complete_stop_attempt --repo "$repo" --generation "$hook_generation" \
      --attempt "$hook_attempt_seq" --result ERROR --outcome PRESENTATION_UNAVAILABLE \
      --payload-file "$response_file" || true
    rm -f "$response_file"
    return 0
  fi
  report_id=$("$ms" json get --file "$presentation_file" --field report.id 2>/dev/null || true)
  report_alias=$("$ms" json get --file "$presentation_file" --field report.alias 2>/dev/null || true)
  report_path=$("$ms" json get --file "$presentation_file" --field report.path 2>/dev/null || true)
  report_sha=$("$ms" json get --file "$presentation_file" --field report.sha256 2>/dev/null || true)
  if ! complete_stop_attempt --repo "$repo" --generation "$hook_generation" \
      --attempt "$hook_attempt_seq" --result OK --outcome EMITTED \
      --health-line "$health_line" --payload-file "$response_file" \
      --installation "$repo" --report-id "$report_id" --report-alias "$report_alias" --report-path "$report_path" --report-sha256 "$report_sha"; then
    echo "supervision hook: emitted the health line but could not record completion" >&2
  fi
  rm -f "$response_file"
}

compose_failed_stop() { # cause, verdict display, verdict decision, non-blocking detail
  local cause=$1 verdict_display=$2 verdict_should_block=$3 verdict_extras=$4
  local refusal_rc remedy refusal_decision refusal_message refusal_message_rc
  local refusal_system_message refusal_report_tail repeated_message refusal_suffix message_length suffix_length prefix_length
  local record_failure_message blocking_message allowed_message
  failure_detail="$verdict_display

Infrastructure condition, not a refusal (the steward owns repair): $cause"
  remedy='A human or steward must restore supervision outside this seat, then retry.'
  if [[ "$cause" == 'supervision arming failed' && -n "$up_failure_result" ]]; then
    remedy=$up_failure_result
  fi
  refusal_system_message="$verdict_extras${verdict_extras:+$'\n'}$checkin_tail"
  refusal_report_tail=$refusal_system_message
  [[ -z "$up_failure_result" ]] || refusal_report_tail="$refusal_report_tail${refusal_report_tail:+$'\n'}$up_failure_result"
  refusal_rc=0
  response=$(external_stop_json "$refusal_system_message" "$failure_detail" "$cause" "$remedy" 2>/dev/null) || refusal_rc=$?
  if (( refusal_rc != 0 )) || [[ -z "$response" ]]; then
    record_failure_message="Metasystem stop-refusal record failure: the record could not be read or atomically updated. Stopping is allowed so record failure cannot recreate the refusal loop.
Cause: $cause
Remedy: $remedy"
    extras=$(printf '%s%s%s' "$extras" "${extras:+$'\n'}" "$record_failure_message")
    if [[ "$verdict_should_block" == true ]]; then
      blocking_message=$record_failure_message
      [[ -z "$verdict_extras" ]] || blocking_message="$blocking_message
$verdict_extras"
      [[ -z "$checkin_tail" ]] || blocking_message="$blocking_message
$checkin_tail"
      response=$(stop_block_json "$blocking_message" "$verdict_display")
    else
      allowed_message="$record_failure_message
$verdict_display"
      [[ -z "$verdict_extras" ]] || allowed_message="$allowed_message
$verdict_extras"
      [[ -z "$checkin_tail" ]] || allowed_message="$allowed_message
$checkin_tail"
      response=$(surface_json "$allowed_message")
    fi
    return 0
  fi

  refusal_decision=$("$ms" json get --value "$response" --field decision 2>/dev/null || true)
  [[ "$refusal_decision" != block ]] || return 0
  refusal_message_rc=0
  refusal_message=$("$ms" json get --value "$response" --field systemMessage 2>/dev/null) || refusal_message_rc=$?
  if (( refusal_message_rc != 0 )) || [[ -z "$refusal_message" ]]; then
    record_failure_message="Metasystem stop-refusal record failure: the record returned an unreadable repeated-failure response. Stopping is allowed so record failure cannot recreate the refusal loop.
Cause: $cause
Remedy: $remedy"
    extras=$(printf '%s%s%s' "$extras" "${extras:+$'\n'}" "$record_failure_message")
    if [[ "$verdict_should_block" == true ]]; then
      blocking_message=$record_failure_message
      [[ -z "$verdict_extras" ]] || blocking_message="$blocking_message
$verdict_extras"
      [[ -z "$checkin_tail" ]] || blocking_message="$blocking_message
$checkin_tail"
      response=$(stop_block_json "$blocking_message" "$verdict_display")
    else
      allowed_message="$record_failure_message
$verdict_display"
      [[ -z "$verdict_extras" ]] || allowed_message="$allowed_message
$verdict_extras"
      [[ -z "$checkin_tail" ]] || allowed_message="$allowed_message
$checkin_tail"
      response=$(surface_json "$allowed_message")
    fi
    return 0
  fi

  # StopRefusal appends the supplied system message to its repeated-failure
  # notice. Split that known suffix so the repeated notice can precede the
  # ordinary verdict envelope without duplicating extras or the health line.
  repeated_message=$refusal_message
  if [[ -n "$refusal_report_tail" ]]; then
    refusal_suffix=$'\n'"$refusal_report_tail"
    message_length=${#refusal_message}
    suffix_length=${#refusal_suffix}
    if (( message_length >= suffix_length )); then
      prefix_length=$((message_length - suffix_length))
      if [[ "${refusal_message:$prefix_length}" == "$refusal_suffix" ]]; then
        repeated_message=${refusal_message:0:$prefix_length}
      fi
    fi
  fi
  [[ -z "$repeated_message" ]] || extras=$(printf '%s%s%s' \
    "$extras" "${extras:+$'\n'}" "$repeated_message")

  if [[ "$verdict_should_block" == true ]]; then
    blocking_message=$repeated_message
    [[ -z "$verdict_extras" ]] || blocking_message="$blocking_message
$verdict_extras"
    [[ -z "$checkin_tail" ]] || blocking_message="$blocking_message
$checkin_tail"
    response=$(stop_block_json "$blocking_message" "$verdict_display")
  else
    allowed_message="$repeated_message
$verdict_display"
    [[ -z "$verdict_extras" ]] || allowed_message="$allowed_message
$verdict_extras"
    [[ -z "$checkin_tail" ]] || allowed_message="$allowed_message
$checkin_tail"
    response=$(surface_json "$allowed_message")
  fi
}

if [[ "$event" == stop ]]; then
  stop_refusal_slug=$("$ms" util slug "$session")
  stop_refusal_record="$repo/artifacts/agents/supervision/stop-refusals/$stop_refusal_slug.json"
  protocol_message=
  protocol_counts='{}'
  if [[ -n "$main_id" ]]; then
    protocol_growth_rc=0
    protocol_growth=$("$ms" lease protocol-growth --root "$repo" --main-id "$main_id" 2>/dev/null) || protocol_growth_rc=$?
    if (( protocol_growth_rc != 0 )); then
      record_stop_failure "the holder protocol state could not be read" holder-protocol
    elif [[ -n "$protocol_growth" ]]; then
      protocol_message_rc=0
      protocol_counts_rc=0
      protocol_message=$("$ms" json get --value "$protocol_growth" --field message 2>/dev/null) || protocol_message_rc=$?
      protocol_counts=$("$ms" json get --value "$protocol_growth" --field counts 2>/dev/null) || protocol_counts_rc=$?
      if (( protocol_message_rc != 0 || protocol_counts_rc != 0 )); then
        record_stop_failure "the holder protocol state was unreadable" holder-protocol
      fi
    fi
  fi
  # The evidence trail sits beside the rest of the supervision state. One
  # hook-log line per infrastructure condition, in every outcome: the
  # advisor's allowance below appends its lines too.
  supervision_dir="$repo/artifacts/agents/supervision"
  mkdir -p "$supervision_dir" 2>/dev/null || true
  hook_log_failure=
  append_stop_condition() { # class, cause code, component, outcome
    local generation=${hook_generation:--} deadline_end=$((stop_started_epoch + 60))
    if ! printf 'stop-condition %s %s %s %s %s %s\n' "$1" "$2" "$3" \
        "$generation" "$deadline_end" "$4" >>"$supervision_dir/hooks.log" 2>/dev/null; then
      hook_log_failure="the infrastructure stop condition could not be appended to the hook log"
    fi
  }
  # "Advisor" is a positive finding, not a fallback. It means an announced main
  # of THIS checkout is not the one holding it. A caller that could not be
  # identified at all is not an advisor -- it is unclassified, and answering it
  # with OWNED-ELSEWHERE replaces the entire turn-end report, including the
  # refusal to walk away from open work, with a sentence about ownership.
  if [[ "$main_class" == MAIN && "$main_holder" != true ]]; then
    advisor_message="OWNED-ELSEWHERE: this main is a read-only advisor in this checkout. To write independently, run scripts/agents/second-session.sh."
    [[ -z "$up_failure" ]] || advisor_message="$advisor_message
$up_failure"
    [[ -z "$hook_evidence_failure" ]] || advisor_message="$advisor_message
$hook_evidence_failure"
    [[ -z "$protocol_message" ]] || advisor_message="$advisor_message
$protocol_message"
    if (( ${#stop_conditions[@]} > 0 )); then
      for stop_condition in "${stop_conditions[@]}"; do
        append_stop_condition infrastructure "${stop_condition%%|*}" "${stop_condition#*|}" degraded-allow
      done
    fi
    [[ -z "$hook_log_failure" ]] || advisor_message="$advisor_message
$hook_log_failure"
    extras=$advisor_message
    present_stop_payload false true
    emit_stop_payload "$response"
    [[ "$stop_delivery_ready" != true || -z "$main_id" || -z "$identity_pid" || -z "$protocol_message" ]] || \
      "$ms" lease protocol-advance --root "$repo" --main-id "$main_id" \
        --caller-pid "$identity_pid" --counts "$protocol_counts" >/dev/null 2>&1 || true
    exit 0
  fi
  if [[ -n "$identity_pid" ]]; then
    renew_rc=0
    "$ms" lease renew --root "$repo" --caller-pid "$identity_pid" >/dev/null 2>&1 || renew_rc=$?
    (( renew_rc == 0 )) || record_stop_failure "the checkout holder lease could not be renewed" holder-lease
  fi

  # The WATCHDOG path calls the verdict like every other path (only the
  # advisor early-exit above bypasses it): the report's text stays
  # hook-side, its DIGEST rides to the verb, and the verb's surfaceWatchdog
  # answer decides exactly-once surfacing across concurrent Stop calls
  # (goal-system GOAL-04; the loose per-session state files are retired).
  watchdog_rc=0
  watchdog_text=$("$ms" supervise watchdog-report --repo "$repo" 2>/dev/null) || watchdog_rc=$?
  (( watchdog_rc == 0 )) || record_stop_failure "the supervision watchdog state could not be read" watchdog
  watchdog_digest=
  if [[ -n "$watchdog_text" ]]; then
    watchdog_digest_rc=0
    watchdog_digest=$(printf '%s' "$watchdog_text" | "$ms" util sha256) || watchdog_digest_rc=$?
    (( watchdog_digest_rc == 0 )) || record_stop_failure "the supervision watchdog evidence could not be prepared" watchdog
  fi

  # Leave evidence that this ran. Without it there is no telling a hook that
  # fired and found nothing from one that never fired, which is the confusion
  # that let this repository run for days with its hooks uninstalled.
  evidence_gc_rc=0
  "$world_installation/scripts/agents/evidence-gc.sh" \
    >>"$supervision_dir/hooks.log" 2>&1 || evidence_gc_rc=$?
  (( evidence_gc_rc == 0 )) || record_stop_failure "the hook evidence state could not be maintained" hook-evidence

  # ONE structured decision (goal-system GOAL-05): the verdict verb owns
  # open work, the goal clause, precedence, block-once state, and the
  # all-clear. Every representable state is exit 0 with JSON; a nonzero
  # exit is I/O failure and this hook emits a provider-level refusal — never
  # silence, and never an all-clear it cannot vouch for.
  verdict_stderr=$(mktemp "${TMPDIR:-/tmp}/metasystem-verdict-err.XXXXXX")
  verdict_file=$stop_work_dir/verdict.json
  facts_file=$stop_work_dir/facts.json
  completion_file=$stop_work_dir/completion.json
  verdict_readable=false
  degraded_line=
  completion_failure=
  if verdict=$(report_turn_verdict --root "$repo" \
      --session "$session" --watchdog-surfaced "$watchdog_digest" \
      --main-id "$main_id" --stop-hook-active="$stop_hook_active" \
      --transcript "$transcript_path" --runtime "$runtime" \
      --facts-file "$facts_file" --completion-file "$completion_file" 2>"$verdict_stderr"); then
    printf '%s\n' "$verdict" >"$verdict_file"
    if [[ ! -s "$facts_file" ]]; then
      facts_failure=$(tail -1 "$verdict_stderr" 2>/dev/null || true)
      record_stop_failure "the frozen judgment facts were unavailable" judgment-facts
    fi
    if [[ ! -s "$completion_file" ]]; then
      completion_failure=$(tail -1 "$verdict_stderr" 2>/dev/null || true)
      record_stop_failure "the completion observation was unavailable" completion-observation
    fi
    rm -f "$verdict_stderr"
    should_block_rc=0
    display_rc=0
    surface_watchdog_rc=0
    idle_refusal_rc=0
    brain_status_due_rc=0
    should_block=$("$ms" json get --value "$verdict" --field shouldBlock 2>/dev/null) || should_block_rc=$?
    display=$("$ms" json get --value "$verdict" --field display 2>/dev/null) || display_rc=$?
    surface_watchdog=$("$ms" json get --value "$verdict" --field surfaceWatchdog 2>/dev/null) || surface_watchdog_rc=$?
    idle_refusal=$("$ms" json get --value "$verdict" --field idleRefusal 2>/dev/null) || idle_refusal_rc=$?
    verdict_class=seat-actionable
    if class_value=$("$ms" json get --value "$verdict" --field class 2>/dev/null); then
      verdict_class=$class_value
    fi
    count_spent=true
    if count_spent_value=$("$ms" json get --value "$verdict" --field countSpent 2>/dev/null); then
      count_spent=$count_spent_value
    fi
    brain_status_due=$("$ms" json get --value "$verdict" --field brainStatusDue 2>/dev/null) || brain_status_due_rc=$?
    if (( should_block_rc != 0 || display_rc != 0 || surface_watchdog_rc != 0 || idle_refusal_rc != 0 || brain_status_due_rc != 0 )) || [[ -z "$display" ]] ||
        [[ ( "$should_block" != true && "$should_block" != false ) ||
           ( "$surface_watchdog" != true && "$surface_watchdog" != false ) ||
           ( "$idle_refusal" != true && "$idle_refusal" != false ) ||
           ( "$brain_status_due" != true && "$brain_status_due" != false ) ||
           ( "$count_spent" != true && "$count_spent" != false ) ||
           ( "$verdict_class" != infrastructure && "$verdict_class" != seat-actionable && "$verdict_class" != idle-with-backlog ) ]]; then
      degraded_line='the turn verdict was unreadable'
    else
      verdict_readable=true
    fi
  else
    degraded_line=$(tail -1 "$verdict_stderr" 2>/dev/null || true)
    rm -f "$verdict_stderr"
  fi

  if (( ${#stop_conditions[@]} > 0 )); then
    for stop_condition in "${stop_conditions[@]}"; do
      append_stop_condition infrastructure "${stop_condition%%|*}" "${stop_condition#*|}" degraded-allow
    done
  fi
  if [[ "$verdict_readable" == true && "$verdict_class" == infrastructure ]]; then
    verdict_cause=$("$ms" json get --value "$verdict" --field causeCode 2>/dev/null || true)
    verdict_component=$("$ms" json get --value "$verdict" --field component 2>/dev/null || true)
    append_stop_condition infrastructure "${verdict_cause:-turn-verdict-unavailable}" "${verdict_component:-verdict-state}" degraded-allow
    # The verdict's own state could not be read or written: the notice says
    # so in fixed words, names the owner, and carries the detail; it never
    # reads as an all-clear.
    display="turn-verdict degraded: stopping is allowed on degraded infrastructure; the steward owns repair. Cause: ${verdict_cause:-turn-verdict-unavailable}. Component: ${verdict_component:-verdict-state}.
$display"
  fi
  if [[ "$verdict_readable" == true && "$verdict_class" == idle-with-backlog && "$should_block" == true && "$count_spent" == false ]]; then
    append_stop_condition idle-with-backlog idle-refusal-count-not-spent verdict-state refused-uncounted
  fi
  if [[ "$verdict_readable" != true ]]; then
    append_stop_condition infrastructure turn-verdict-unavailable verdict-state degraded-allow
  fi

  if [[ "$verdict_readable" == true ]]; then
    printf '%s stop verdict block=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
      "$should_block" >>"$supervision_dir/hooks.log" 2>/dev/null || true

    brain_post_failure=
    if [[ "$brain_status_due" == true ]]; then
      brain_post_before=$("$ms" json get --file "$repo/artifacts/agents/brain-status.json" --field lastPostedAt --default '' 2>/dev/null || true)
      brain_post_stderr=$(mktemp "${TMPDIR:-/tmp}/metasystem-brain-status-post.XXXXXX")
      brain_post_rc=0
      "$ms" channel status --post --root "$repo" >/dev/null 2>"$brain_post_stderr" || brain_post_rc=$?
      brain_post_after=$("$ms" json get --file "$repo/artifacts/agents/brain-status.json" --field lastPostedAt --default '' 2>/dev/null || true)
      if (( brain_post_rc != 0 )); then
        brain_post_reason=$(tail -1 "$brain_post_stderr" 2>/dev/null || true)
        [[ -n "$brain_post_reason" ]] || brain_post_reason="channel status exited $brain_post_rc"
        brain_post_failure="the brain's status line was not published: $brain_post_reason"
      elif [[ -z "$brain_post_after" || "$brain_post_after" == "$brain_post_before" ]]; then
        brain_post_failure="the brain's status line was not published: no channel provider is configured"
      fi
      rm -f "$brain_post_stderr"
    fi

    extras=
    [[ -z "$up_notice" ]] || extras=$(printf '%s%s%s' "$extras" "${extras:+$'\n'}" "$up_notice")
    [[ -z "$hook_evidence_failure" ]] || extras=$(printf '%s%s%s' "$extras" "${extras:+$'\n'}" "$hook_evidence_failure")
    [[ -z "$facts_failure" ]] || extras=$(printf '%s%s%s' "$extras" "${extras:+$'\n'}" "$facts_failure")
    [[ -z "$completion_failure" ]] || extras=$(printf '%s%s%s' "$extras" "${extras:+$'\n'}" "$completion_failure")
    [[ "$surface_watchdog" != true || -z "$watchdog_text" ]] || extras=$(printf '%s%s%s' "$extras" "${extras:+$'\n'}" "$watchdog_text")
    [[ -z "$protocol_message" ]] || extras=$(printf '%s%s%s' "$extras" "${extras:+$'\n'}" "$protocol_message")
    [[ -z "$brain_post_failure" ]] || extras=$(printf '%s%s%s' "$extras" "${extras:+$'\n'}" "$brain_post_failure")

    [[ -z "$hook_log_failure" ]] || extras=$(printf '%s%s%s' "$extras" "${extras:+$'\n'}" "$hook_log_failure")

    if [[ -n "$stop_failure" ]]; then
      # This call records the stable refusal cause and occurrence. The shared
      # presenter below owns the public response.
      compose_failed_stop "$stop_failure" "$display" "$should_block" "$extras"
    fi
    present_stop_payload "$should_block"
  else
    printf '%s stop verdict unavailable\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
      >>"$supervision_dir/hooks.log" 2>/dev/null || true
    degraded_message="turn-verdict unavailable: stopping is allowed on degraded infrastructure; the steward owns repair. ${degraded_line:-no diagnostic}"
    [[ -z "$up_failure" ]] || degraded_message="$degraded_message
$up_failure"
    [[ -z "$hook_evidence_failure" ]] || degraded_message="$degraded_message
$hook_evidence_failure"
    [[ -z "$protocol_message" ]] || degraded_message="$degraded_message
$protocol_message"
    [[ -z "$hook_log_failure" ]] || degraded_message="$degraded_message
$hook_log_failure"
    response=$(surface_json "$degraded_message
$checkin_tail")
    fallback_stop_payload false
  fi
  emit_stop_payload "$response"
  [[ "$stop_delivery_ready" != true || -z "$main_id" || -z "$identity_pid" || -z "$protocol_message" ]] || \
    "$ms" lease protocol-advance --root "$repo" --main-id "$main_id" \
      --caller-pid "$identity_pid" --counts "$protocol_counts" >/dev/null 2>&1 || true
  exit 0
fi

# The second visibility channel runs before EVERY exit from here on:
# a session's start names anything the steward could not deliver,
# and the unidentified-agent branch is the degraded case that needs
# it most.
pending_line=$("$ms" steward pending --repo "$repo" 2>/dev/null || true)
[[ -n "$pending_line" ]] && surface_json "Steward incidents pending: $pending_line"

if [[ "$event" == end ]]; then
  session_end_rc=0
  "$ms" session end --root "$repo" --session "$session" >/dev/null 2>&1 || session_end_rc=$?
  if (( session_end_rc != 0 )); then
    surface_json "Metasystem could not durably retire this session's unused stop authorization; later stops must treat it as unsafe."
  fi
  if [[ -z "$identity" ]]; then
    surface_json "Metasystem supervision could not identify the immediate $runtime agent process; arming was refused."
    exit 0
  fi
  pid=$identity_pid
  started=$identity_started
  METASYSTEM_AGENT_RUNTIME="$runtime" "$ms" up --metasystem-root "$world_installation" \
    --repo "$repo" --session "$session" --pid "$pid" --start-time "$started" \
    --tag "$tag" "${runtime_session_args[@]}" --retire >/dev/null 2>&1 || true
  exit 0
fi

if [[ -z "$identity" ]]; then
  surface_json "Metasystem supervision could not identify the immediate $runtime agent process; arming was refused."
  emit_start_payload
  exit 0
fi
pid=$identity_pid
started=$identity_started

if output=$(METASYSTEM_AGENT_RUNTIME="$runtime" "$ms" up --metasystem-root "$world_installation" \
    --repo "$repo" --session "$session" --pid "$pid" --start-time "$started" \
    --tag "$tag" "${runtime_session_args[@]}" 2>&1); then
  # The watchdog revives with the first metasystem activity on this
  # machine: `up` verifies the owner, watcher, steward, announcement, and
  # lease as one idempotent transaction.
  up_aggregate=$(printf '%s' "$output" | tail -1)
  if [[ "$up_aggregate" == *" re-armed="* ]]; then
    surface_json "Metasystem re-armed the rebuilt engine: $up_aggregate"
  fi
  waiting_lines_rc=0
  waiting_lines=$("$ms" session start --root "$repo" --session "$session" 2>&1) || waiting_lines_rc=$?
  if (( waiting_lines_rc == 0 )) && [[ -n "$waiting_lines" ]]; then
    collect_start_notice "$waiting_lines"
  elif (( waiting_lines_rc == 64 )); then
    : # A non-holder session has no recovery rows to advertise.
  elif (( waiting_lines_rc != 0 )); then
    waiting_lines=${waiting_lines//$'\n'/'; '}
    collect_start_notice "Metasystem could not read durable wait recovery rows for this session: $waiting_lines"
  fi
  emit_start_payload
  exit 0
fi
up_aggregate=$(printf '%s' "$output" | tail -1)
up_message="Metasystem supervision arming failed: $up_aggregate"
if [[ "$up_aggregate" == *" re-armed="* ]]; then
  up_message="$up_message
Metasystem re-armed the rebuilt engine: $up_aggregate"
fi
surface_json "$up_message"
emit_start_payload
