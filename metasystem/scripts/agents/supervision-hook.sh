#!/usr/bin/env bash
# Plumbing stub for the runtime lifecycle hook (plans/designs/verbs-object-action.md
# section 3.3). Runtime settings name this path; the hook is the engine's
# `internal hook RUNTIME EVENT` entry, which the stub locates and execs. When
# no engine serves the entry (absent, not executable, or older than it) the
# stub starts one detached rebuild and answers with the fixed degraded
# response, exit 0; Claude's tool gate runs on the existing engine meanwhile.

# BEGIN GENERATED degraded forms (internal/hooks renders them; do not edit)
hook_stop_engine_missing='{"systemMessage":"Task unknown; Stop allowed; needs supervision repair; engine missing. Rebuild it with go run ./cmd/devgate build. Status unavailable."}'
hook_stop_bootstrap_failed='{"systemMessage":"Task unknown; Stop allowed; needs supervision repair; hook-bootstrap-failed. The steward must restore supervision. Status unavailable."}'
hook_start_engine_missing='{"systemMessage":"Metasystem engine missing: this session received no role context; if this checkout is a declared brain it is uninstructed until the engine is rebuilt: run go run ./cmd/devgate build in the metasystem installation, then start a new session"}'
# END GENERATED degraded forms

hook_source=${BASH_SOURCE[0]}
case "$hook_source" in */*) hook_dir=${hook_source%/*} ;; *) hook_dir=. ;; esac
hook_installation=$(builtin cd -- "$hook_dir/../.." 2>/dev/null && builtin pwd -P) || hook_installation=

# A linked worktree runs its primary checkout's engine, found by Git
# common-directory identity with inherited Git steering removed. Claude's tool
# gate needs no mapping: any engine that serves the entry reads its cache.
hook_git() {
  env -u GIT_DIR -u GIT_WORK_TREE -u GIT_COMMON_DIR -u GIT_INDEX_FILE -u GIT_CEILING_DIRECTORIES \
    -u GIT_DISCOVERY_ACROSS_FILESYSTEM -u GIT_OBJECT_DIRECTORY -u GIT_ALTERNATE_OBJECT_DIRECTORIES \
    -u GIT_CONFIG -u GIT_CONFIG_PARAMETERS -u GIT_CONFIG_COUNT -u GIT_CONFIG_GLOBAL -u GIT_CONFIG_SYSTEM \
    -u GIT_CONFIG_NOSYSTEM -u GIT_GRAFT_FILE -u GIT_SHALLOW_FILE -u GIT_REPLACE_REF_BASE \
    -u GIT_IMPLICIT_WORK_TREE -u GIT_NO_REPLACE_OBJECTS -u GIT_PREFIX git "$@"
}
hook_world=$hook_installation
if [[ -n "$hook_installation" && ( ${2-} != tool || ! -x "$hook_installation/bin/metasystem" ) ]]; then
  hook_ids=$(hook_git -C "$hook_installation" rev-parse --path-format=absolute --git-dir --git-common-dir 2>/dev/null) || hook_ids=
  hook_git_dir=${hook_ids%%$'\n'*}
  hook_git_common=${hook_ids#*$'\n'}
  if [[ "$hook_ids" == *$'\n'* && "$hook_git_dir" != "$hook_git_common" && "${hook_git_common##*/}" == .git ]]; then
    hook_top=$(hook_git -C "$hook_installation" rev-parse --show-toplevel 2>/dev/null) || hook_top=
    hook_top=$(builtin cd -- "$hook_top" 2>/dev/null && builtin pwd -P) || hook_top=
    hook_primary=$(builtin cd -- "${hook_git_common%/*}" 2>/dev/null && builtin pwd -P) || hook_primary=
    if [[ -n "$hook_top" && -n "$hook_primary" ]]; then
      case "$hook_installation" in
        "$hook_top") hook_world=$hook_primary ;;
        "$hook_top"/*) hook_world=$hook_primary/${hook_installation#"$hook_top"/} ;;
      esac
    fi
  fi
fi
hook_canonical=$hook_world/bin/metasystem
hook_engine=${METASYSTEM_BIN:-$hook_canonical}
hook_runnable=0
[[ -n "$hook_world" && -x "$hook_canonical" && -x "$hook_engine" ]] && hook_runnable=1

# The rebuild never runs inside the event. The fence is a link naming its
# holder's pid: the stub claims it, starts the build in its own process
# group, points the fence at the builder and answers at once. A dead holder
# leaves the fence stale, so a stub killed anywhere, or a finished build,
# blocks nothing. Only an installation that carries its build rebuilds.
hook_fence=$hook_world/artifacts/agents/hook-bootstrap.fence
hook_point_fence() { ln -s "$1" "$hook_fence.$$" 2>/dev/null && mv -f "$hook_fence.$$" "$hook_fence"; }
hook_bootstrap() {
  local holder
  [[ -n "$hook_world" && -d "$hook_world/cmd/devgate" ]] || return 1
  mkdir -p "$hook_world/artifacts/agents" 2>/dev/null && [[ ! -d "$hook_fence" || -L "$hook_fence" ]] || return 1
  if ! ln -s "$$" "$hook_fence" 2>/dev/null; then
    holder=$(readlink "$hook_fence" 2>/dev/null) || holder=
    [[ "$holder" =~ ^[1-9][0-9]*$ ]] && kill -0 "$holder" 2>/dev/null && return 0
    hook_point_fence "$$" || return 1
  fi
  set -m
  nohup bash -c 'builtin printf "hook bootstrap: builder %s starts the engine build\n" "$$"; builtin cd -- "$1" || exit 1
    exec go run ./cmd/devgate build' \
    hook-bootstrap "$hook_world" >>"$hook_world/artifacts/agents/hook-bootstrap.log" 2>&1 </dev/null &
  set +m
  hook_point_fence "$!"
}

# Without the entry the stub still refuses what the hook never serves, as the
# former script did: a malformed runtime name, an unknown event, or a tool
# call from a runtime other than Claude. SessionEnd never rebuilds.
hook_command=(internal hook "$@")
if (( ! hook_runnable )) || ! "$hook_engine" internal hook --accepts >/dev/null 2>&1; then
  case ${2-} in
    tool) [[ ${1-} == claude ]] || exit 2 ;;
    receipt | stop | end) [[ ${1-} =~ ^[a-z][a-z0-9-]{0,31}$ ]] || exit 2 ;;
    start) ;;
    *) exit 2 ;;
  esac
  hook_command=()
  case ${2-} in
    tool) [[ -n "${METASYSTEM_HOOK_DELEGATE_JOB:-}" ]] || (( ! hook_runnable )) || hook_command=(adapter claude-tool-gate --root "$hook_world") ;;
    start | stop | receipt) hook_bootstrap || true ;;
  esac
fi
if [[ ${#hook_command[@]} -gt 0 ]]; then
  METASYSTEM_HOOK_SCRIPT=$hook_source exec "$hook_engine" "${hook_command[@]}"
fi
case ${2-} in
  stop) if (( hook_runnable )); then builtin printf '%s\n' "$hook_stop_bootstrap_failed"; else builtin printf '%s\n' "$hook_stop_engine_missing"; fi ;;
  start) builtin printf '%s\n' "$hook_start_engine_missing" ;;
esac
exit 0
