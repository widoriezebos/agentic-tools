#!/usr/bin/env bash

# These parallel declarations are copied into the supervision hook so its
# last-resort Stop responses remain available when every other file is absent.
# BEGIN SOURCE degraded Stop forms
declare -ar degraded_stop_cause_keys=(
  unreadable-output
  engine-missing
  engine-skew
  staging-failed
  deadline-expired
  bootstrap-failed
  bare
)
declare -ar degraded_stop_cause_texts=(
  stop-hook-output-was-unreadable
  'engine missing'
  'engine does not answer path state-root'
  'payload staging failed'
  'stop deadline expired'
  hook-bootstrap-failed
  ''
)
declare -ar degraded_stop_cause_remedies=(
  'The steward must restore supervision.'
  'Rebuild bin/metasystem.'
  'Rebuild bin/metasystem.'
  'The steward must restore supervision.'
  'The steward must restore supervision.'
  'The steward must restore supervision.'
  ''
)
declare -ar degraded_stop_cause_qualifiers=(
  'condition-log-failed,no-resolved-checkout'
  ''
  ''
  ''
  'record-update-failed,condition-log-failed,no-resolved-checkout'
  ''
  ''
)
declare -ar degraded_stop_qualifier_keys=(
  record-update-failed
  condition-log-failed
  no-resolved-checkout
)
declare -ar degraded_stop_qualifier_texts=(
  'record update failed'
  'condition log failed'
  'no resolved checkout'
)

# degraded_stop_form renders one fixed provider payload without consulting the
# environment, filesystem, engine, or any command outside the shell.
degraded_stop_form() { # allowed|blocked cause-key [qualifier-key...]
  local outcome=${1-} cause=${2-} cause_index=-1 index qualifier qualifier_index
  local cause_text remedy admitted qualifiers= message outcome_word
  local requested=(false false false)
  (( $# >= 2 )) || return 2
  shift 2
  case "$outcome" in
    allowed) outcome_word=allowed ;;
    blocked) outcome_word=blocked ;;
    *) return 2 ;;
  esac
  for index in "${!degraded_stop_cause_keys[@]}"; do
    if [[ "${degraded_stop_cause_keys[$index]}" == "$cause" ]]; then
      cause_index=$index
      break
    fi
  done
  (( cause_index >= 0 )) || return 2
  [[ "$outcome" != blocked || "$cause" == bare ]] || return 2
  cause_text=${degraded_stop_cause_texts[$cause_index]}
  remedy=${degraded_stop_cause_remedies[$cause_index]}
  admitted=${degraded_stop_cause_qualifiers[$cause_index]}
  for qualifier in "$@"; do
    qualifier_index=-1
    for index in "${!degraded_stop_qualifier_keys[@]}"; do
      if [[ "${degraded_stop_qualifier_keys[$index]}" == "$qualifier" ]]; then
        qualifier_index=$index
        break
      fi
    done
    (( qualifier_index >= 0 )) || return 2
    case ",$admitted," in
      *",$qualifier,"*) ;;
      *) return 2 ;;
    esac
    requested[$qualifier_index]=true
  done
  for index in "${!degraded_stop_qualifier_keys[@]}"; do
    [[ "${requested[$index]}" != true ]] || qualifiers="$qualifiers; ${degraded_stop_qualifier_texts[$index]}"
  done
  message="Task unknown; Stop $outcome_word; needs supervision repair;"
  if [[ "$cause" == bare ]]; then
    message="$message Status unavailable."
  else
    message="$message $cause_text$qualifiers. $remedy Status unavailable."
  fi
  if [[ "$outcome" == blocked ]]; then
    builtin printf '{"decision":"block","reason":"%s"}\n' "$message"
  else
    builtin printf '{"systemMessage":"%s"}\n' "$message"
  fi
}
# END SOURCE degraded Stop forms

degraded_stop_source_block() {
  local line inside=false found=false block=
  while IFS= read -r line || [[ -n "$line" ]]; do
    if [[ "$line" == '# BEGIN SOURCE degraded Stop forms' ]]; then
      inside=true
      found=true
      continue
    fi
    if [[ "$line" == '# END SOURCE degraded Stop forms' ]]; then
      inside=false
      break
    fi
    [[ "$inside" == true ]] || continue
    block=$block$line$'\n'
  done <"${BASH_SOURCE[0]}"
  [[ "$found" == true && "$inside" == false ]] || return 1
  degraded_stop_block=${block%$'\n'}
}

degraded_stop_list() {
  local cause cause_index admitted qualifier_index qualifier bit count maximum mask
  local payload joined
  local selected=()
  for cause_index in "${!degraded_stop_cause_keys[@]}"; do
    cause=${degraded_stop_cause_keys[$cause_index]}
    if [[ "$cause" == bare ]]; then
      payload=$(degraded_stop_form allowed bare) || return
      builtin printf 'allowed\tbare\t\t%s\n' "$payload"
      payload=$(degraded_stop_form blocked bare) || return
      builtin printf 'blocked\tbare\t\t%s\n' "$payload"
      continue
    fi
    admitted=${degraded_stop_cause_qualifiers[$cause_index]}
    count=0
    for qualifier in "${degraded_stop_qualifier_keys[@]}"; do
      case ",$admitted," in
        *",$qualifier,"*) count=$((count + 1)) ;;
      esac
    done
    maximum=$((1 << count))
    mask=0
    while (( mask < maximum )); do
      selected=()
      joined=
      bit=0
      for qualifier_index in "${!degraded_stop_qualifier_keys[@]}"; do
        qualifier=${degraded_stop_qualifier_keys[$qualifier_index]}
        case ",$admitted," in
          *",$qualifier,"*)
            if (( mask & (1 << bit) )); then
              selected+=("$qualifier")
              [[ -z "$joined" ]] || joined=$joined,
              joined=$joined$qualifier
            fi
            bit=$((bit + 1))
            ;;
        esac
      done
      payload=$(degraded_stop_form allowed "$cause" "${selected[@]}") || return
      builtin printf 'allowed\t%s\t%s\t%s\n' "$cause" "$joined" "$payload"
      mask=$((mask + 1))
    done
  done
}

degraded_stop_expected_hook() { # path; result in degraded_stop_expected
  local path=$1 line inside=false begins=0 ends=0 output=
  degraded_stop_source_block || return 1
  [[ -r "$path" ]] || return 1
  while IFS= read -r line || [[ -n "$line" ]]; do
    if [[ "$line" == '# BEGIN GENERATED degraded Stop forms' ]]; then
      begins=$((begins + 1))
      inside=true
      output=$output$line$'\n'$degraded_stop_block$'\n'
      continue
    fi
    if [[ "$line" == '# END GENERATED degraded Stop forms' ]]; then
      ends=$((ends + 1))
      inside=false
      output=$output$line$'\n'
      continue
    fi
    [[ "$inside" == true ]] || output=$output$line$'\n'
  done <"$path"
  (( begins == 1 && ends == 1 )) || return 1
  degraded_stop_expected=$output
}

degraded_stop_expected_template() { # path; result in degraded_stop_expected
  local path=$1 line matches=0 output= payload escaped tail
  local prefix='            "command": "(bash scripts/agents/supervision-hook.sh claude stop) || '
  [[ -r "$path" ]] || return 1
  payload=$(degraded_stop_form allowed bootstrap-failed) || return 1
  escaped=${payload//\\/\\\\}
  escaped=${escaped//\"/\\\"}
  tail="printf '%s\\\\n' '$escaped'"
  while IFS= read -r line || [[ -n "$line" ]]; do
    case "$line" in
      "$prefix"*)
        matches=$((matches + 1))
        line=$prefix$tail'",'
        ;;
    esac
    output=$output$line$'\n'
  done <"$path"
  (( matches == 1 )) || return 1
  degraded_stop_expected=$output
}

degraded_stop_copy_status() { # relative path expected content check|sync
  local relative=$1 expected=$2 operation=$3 path current
  path=$degraded_stop_root/$relative
  [[ -r "$path" ]] || {
    builtin printf 'stale generated copy: %s\n' "$relative" >&2
    return 1
  }
  current=$(<"$path")
  if [[ "$current" == "${expected%$'\n'}" ]]; then
    return 0
  fi
  if [[ "$operation" == sync ]]; then
    builtin printf '%s' "$expected" >"$path" || return 1
  else
    builtin printf 'stale generated copy: %s\n' "$relative" >&2
    return 1
  fi
}

degraded_stop_sync_or_check() { # check|sync root
  local operation=$1 hook_relative=scripts/agents/supervision-hook.sh
  local template_relative=scripts/enforcement/claude-code-hooks.json failed=false
  if degraded_stop_expected_hook "$degraded_stop_root/$hook_relative"; then
    degraded_stop_copy_status "$hook_relative" "$degraded_stop_expected" "$operation" || failed=true
  else
    builtin printf 'stale generated copy: %s\n' "$hook_relative" >&2
    failed=true
  fi
  if degraded_stop_expected_template "$degraded_stop_root/$template_relative"; then
    degraded_stop_copy_status "$template_relative" "$degraded_stop_expected" "$operation" || failed=true
  else
    builtin printf 'stale generated copy: %s\n' "$template_relative" >&2
    failed=true
  fi
  [[ "$failed" == false ]]
}

degraded_stop_main() {
  local mode=${1-} script_dir root
  shift || true
  case "$mode" in
    --list)
      (( $# == 0 )) || return 64
      degraded_stop_list
      ;;
    --sync | --check)
      if (( $# == 0 )); then
        script_dir=${BASH_SOURCE[0]%/*}
        [[ "$script_dir" != "${BASH_SOURCE[0]}" ]] || script_dir=.
        root=$(cd "$script_dir/../.." && builtin pwd -P) || return 1
      elif (( $# == 2 )) && [[ "$1" == --root ]]; then
        root=$(cd "$2" && builtin pwd -P) || return 1
      else
        return 64
      fi
      degraded_stop_root=$root
      degraded_stop_sync_or_check "${mode#--}"
      ;;
    *) return 64 ;;
  esac
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  degraded_stop_main "$@"
fi
