#!/usr/bin/env bash
# Plumbing stub for the runtime lifecycle hook (plans/designs/verbs-object-action.md
# section 3.3). Installed runtime settings in every checkout name this path; the
# hook itself is the engine's `internal hook RUNTIME EVENT` entry. The stub only
# locates that engine and hands it the invocation. When no engine serves the
# entry (absent, not executable, or older than the entry) it rebuilds once
# under its fence and retries; when that fails it prints the fixed degraded
# response and exits 0. It makes no metasystem decision and calls no other
# engine verb.

# BEGIN GENERATED degraded forms (internal/hooks renders them; do not edit)
hook_stop_engine_missing='{"systemMessage":"Task unknown; Stop allowed; needs supervision repair; engine missing. Rebuild bin/metasystem. Status unavailable."}'
hook_stop_bootstrap_failed='{"systemMessage":"Task unknown; Stop allowed; needs supervision repair; hook-bootstrap-failed. The steward must restore supervision. Status unavailable."}'
hook_start_engine_missing='{"systemMessage":"Metasystem engine missing: this session received no role context; if this checkout is a declared brain it is uninstructed until the engine is rebuilt: run scripts/agents/go-build.sh, then start a new session"}'
# END GENERATED degraded forms

hook_source=${BASH_SOURCE[0]}
case "$hook_source" in
  */*) hook_dir=${hook_source%/*} ;;
  *) hook_dir=. ;;
esac
hook_installation=$(builtin cd -- "$hook_dir/../.." 2>/dev/null && builtin pwd -P) || hook_installation=

# A linked worktree runs its primary checkout's engine, found by Git
# common-directory identity with inherited Git steering removed. Claude's tool
# gate needs no mapping: any engine that serves the entry reads its cache.
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

hook_accepts() {
  [[ -n "$hook_world" && -x "$hook_canonical" && -x "$hook_engine" ]] || return 1
  "$hook_engine" internal hook --accepts >/dev/null 2>&1
}

# One rebuild under its fence, only in an installation that carries its build:
# a second hook finding the fence held does not wait or build, and a fence
# older than thirty minutes outlived its build.
hook_bootstrap() {
  local fence=$hook_world/artifacts/agents/hook-bootstrap.fence
  local log=$hook_world/artifacts/agents/hook-bootstrap.log
  local status=0
  [[ -n "$hook_world" && ( -d "$hook_world/cmd/devgate" || -f "$hook_world/scripts/agents/go-build.sh" ) ]] || return 1
  mkdir -p "$hook_world/artifacts/agents" 2>/dev/null || return 1
  if ! mkdir "$fence" 2>/dev/null; then
    [[ -n $(find "$fence" -maxdepth 0 -mmin +30 2>/dev/null) ]] || return 1
    rmdir "$fence" 2>/dev/null && mkdir "$fence" 2>/dev/null || return 1
  fi
  if [[ -d "$hook_world/cmd/devgate" ]]; then
    (builtin cd -- "$hook_world" && go run ./cmd/devgate build) >>"$log" 2>&1 </dev/null || status=$?
  else
    (builtin cd -- "$hook_world" && bash scripts/agents/go-build.sh) >>"$log" 2>&1 </dev/null || status=$?
  fi
  rmdir "$fence" 2>/dev/null
  return "$status"
}

if hook_accepts || { [[ ${2-} != tool ]] && hook_bootstrap && hook_accepts; }; then
  METASYSTEM_HOOK_SCRIPT=$hook_source exec "$hook_engine" internal hook "$@"
fi

# Without an engine the stub still refuses an invocation the hook never
# serves, as the former script did before any engine work: a malformed runtime
# name, an unknown event, or a tool call from a runtime other than Claude.
case ${2-} in
  tool) [[ ${1-} == claude ]] || exit 2 ;;
  receipt | stop | end) [[ ${1-} =~ ^[a-z][a-z0-9-]{0,31}$ ]] || exit 2 ;;
  start) ;;
  *) exit 2 ;;
esac

case ${2-} in
  stop)
    if [[ -x "$hook_canonical" && -x "$hook_engine" ]]; then
      builtin printf '%s\n' "$hook_stop_bootstrap_failed"
    else
      builtin printf '%s\n' "$hook_stop_engine_missing"
    fi
    ;;
  start) builtin printf '%s\n' "$hook_start_engine_missing" ;;
esac
exit 0
