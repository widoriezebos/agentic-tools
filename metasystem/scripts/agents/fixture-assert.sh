#!/usr/bin/env bash

# Fixture assertions report comparison evidence without allowing errexit to
# stop a scenario before its other independent checks have run.

fixture_assert_resolve_physical_file() { # path
  local fixture_assert_candidate=$1
  local fixture_assert_directory fixture_assert_target
  case "$fixture_assert_candidate" in
    /*) ;;
    *) fixture_assert_candidate=$PWD/$fixture_assert_candidate ;;
  esac
  while [[ -L "$fixture_assert_candidate" ]]; do
    fixture_assert_directory=$(cd -P "$(dirname "$fixture_assert_candidate")" 2>/dev/null && pwd -P) || return 1
    fixture_assert_target=$(readlink "$fixture_assert_candidate") || return 1
    case "$fixture_assert_target" in
      /*) fixture_assert_candidate=$fixture_assert_target ;;
      *) fixture_assert_candidate=$fixture_assert_directory/$fixture_assert_target ;;
    esac
  done
  fixture_assert_directory=$(cd -P "$(dirname "$fixture_assert_candidate")" 2>/dev/null && pwd -P) || return 1
  printf '%s/%s\n' "$fixture_assert_directory" "$(basename "$fixture_assert_candidate")"
}

fixture_assert_helper_file=$(fixture_assert_resolve_physical_file "${BASH_SOURCE[0]}") || fixture_assert_helper_file=${BASH_SOURCE[0]}
fixture_assert_metasystem_root=$(cd -P "$(dirname "$fixture_assert_helper_file")/../.." && pwd -P)
if fixture_assert_repository_root=$(env -u GIT_DIR -u GIT_WORK_TREE git -C "$fixture_assert_metasystem_root" rev-parse --show-toplevel 2>/dev/null); then
  fixture_assert_repository_root=$(cd -P "$fixture_assert_repository_root" && pwd -P)
else
  fixture_assert_repository_root=$fixture_assert_metasystem_root
fi
fixture_assert_engine=${METASYSTEM_BIN:-"$fixture_assert_metasystem_root/bin/metasystem"}
fixture_assert_source_spelling=${BASH_SOURCE[1]-}
fixture_assert_source_physical=
if [[ -n "$fixture_assert_source_spelling" ]]; then
  fixture_assert_source_physical=$(fixture_assert_resolve_physical_file "$fixture_assert_source_spelling" 2>/dev/null) || fixture_assert_source_physical=
fi

fixture_assert_record_file=
fixture_assert_sidecar_file=
fixture_assert_sidecar_sites_file=
fixture_assert_record_file_is_default=0
fixture_assert_record_file_printed=0
fixture_assert_fallback_prepared=0
fixture_assert_unwritten_record=0
fixture_assert_last_failed=0
fixture_assert_relative_file=
fixture_assert_duplicate_found=0
fixture_assert_duplicate_first_file=
fixture_assert_duplicate_first_line=
fixture_assert_registry_error=
fixture_assert_terminal_written=0
fixture_assert_failure_count=0
fixture_assert_digest_tmpdir=${TMPDIR:-/tmp}

fixture_assert_prepare_record_paths() {
  local fixture_assert_wanted fixture_assert_default
  fixture_assert_default=${TMPDIR:-/tmp}/metasystem-bed-failures.$$.jsonl
  fixture_assert_wanted=${METASYSTEM_BED_FAILURES-$fixture_assert_default}
  if [[ -z "${METASYSTEM_BED_FAILURES+x}" ]] && (( ! fixture_assert_fallback_prepared )); then
    if (( ${BASH_SUBSHELL:-0} == 0 )); then
      # A reused process identifier must not inherit another process's assertions.
      rm -f "$fixture_assert_default" "$fixture_assert_default.labels" \
        "$fixture_assert_default.labels.sites" "$fixture_assert_default.path-announced" \
        2>/dev/null || :
      rmdir "$fixture_assert_default.path-announced" 2>/dev/null || :
      fixture_assert_fallback_prepared=1
    fi
  fi
  if [[ "$fixture_assert_wanted" != "$fixture_assert_record_file" ]]; then
    fixture_assert_record_file=$fixture_assert_wanted
    fixture_assert_sidecar_file=$fixture_assert_record_file.labels
    fixture_assert_sidecar_sites_file=$fixture_assert_sidecar_file.sites
    fixture_assert_record_file_printed=0
    if [[ -z "${METASYSTEM_BED_FAILURES+x}" ]]; then
      fixture_assert_record_file_is_default=1
    else
      fixture_assert_record_file_is_default=0
    fi
  fi
}

# Preparing the fallback while the source shell is not a subshell prevents a
# first assertion inside a pipeline from deleting records written beside it.
fixture_assert_prepare_record_paths

fixture_assert_announce_record_file() {
  local fixture_assert_announcement
  fixture_assert_prepare_record_paths
  if (( fixture_assert_record_file_is_default && ! fixture_assert_record_file_printed )); then
    fixture_assert_announcement=$fixture_assert_record_file.path-announced
    if mkdir "$fixture_assert_announcement" 2>/dev/null; then
      if printf 'fixture assertion records: %s\n' "$fixture_assert_record_file" >&2; then :; fi
    elif [[ ! -d "$fixture_assert_announcement" ]]; then
      # A missing announcement marker must not hide the fallback record path.
      if printf 'fixture assertion records: %s\n' "$fixture_assert_record_file" >&2; then :; fi
    fi
    fixture_assert_record_file_printed=1
  fi
}

fixture_assert_one_line() { # value
  local fixture_assert_text=$1
  fixture_assert_text=${fixture_assert_text//$'\n'/; }
  fixture_assert_text=${fixture_assert_text//$'\r'/; }
  printf '%s' "$fixture_assert_text"
}

fixture_assert_mark_unwritten() { # reason
  local fixture_assert_reason
  fixture_assert_prepare_record_paths
  fixture_assert_unwritten_record=1
  fixture_assert_reason=$(fixture_assert_one_line "$1")
  if printf 'fixture assertions: record was not written: %s\n' "$fixture_assert_reason" >&2; then :; fi
  if [[ -n "$fixture_assert_sidecar_sites_file" ]]; then
    if printf '%s\n' 'unwritten-record' 2>/dev/null >>"$fixture_assert_sidecar_sites_file"; then
      :
    fi
  fi
}

fixture_assert_set_relative_file() { # source path
  local fixture_assert_source=$1 fixture_assert_physical=
  if [[ -n "$fixture_assert_source_physical" && "$fixture_assert_source" == "$fixture_assert_source_spelling" ]]; then
    fixture_assert_physical=$fixture_assert_source_physical
  else
    fixture_assert_physical=$(fixture_assert_resolve_physical_file "$fixture_assert_source" 2>/dev/null) || fixture_assert_physical=$fixture_assert_source
  fi
  case "$fixture_assert_physical" in
    "$fixture_assert_repository_root"/*)
      fixture_assert_relative_file=${fixture_assert_physical#"$fixture_assert_repository_root"/}
      ;;
    *)
      fixture_assert_relative_file=$fixture_assert_physical
      ;;
  esac
}

fixture_assert_bound() { # value
  local LC_ALL=C
  local fixture_assert_value=$1 fixture_assert_bytes fixture_assert_cut fixture_assert_byte fixture_assert_byte_value
  local fixture_assert_offset
  fixture_assert_bytes=${#fixture_assert_value}
  if (( fixture_assert_bytes > 2048 )); then
    fixture_assert_cut=2048
    fixture_assert_offset=2048
    while (( fixture_assert_offset >= 2045 )); do
      fixture_assert_byte=${fixture_assert_value:$fixture_assert_offset:1}
      printf -v fixture_assert_byte_value '%d' "'$fixture_assert_byte"
      if (( fixture_assert_byte_value < 0 )); then
        fixture_assert_byte_value=$((fixture_assert_byte_value + 256))
      fi
      if (( fixture_assert_byte_value < 128 || fixture_assert_byte_value >= 192 )); then
        fixture_assert_cut=$fixture_assert_offset
        break
      fi
      fixture_assert_offset=$((fixture_assert_offset - 1))
    done
    printf '%s[... %s more bytes]' "${fixture_assert_value:0:$fixture_assert_cut}" \
      "$((fixture_assert_bytes - fixture_assert_cut))"
  else
    printf '%s' "$fixture_assert_value"
  fi
}

fixture_assert_bound_into() { # output variable, value
  local fixture_assert_destination=$1 fixture_assert_bounded_value
  fixture_assert_bounded_value=$(fixture_assert_bound "$2"; printf '\034')
  fixture_assert_bounded_value=${fixture_assert_bounded_value%?}
  printf -v "$fixture_assert_destination" '%s' "$fixture_assert_bounded_value"
}

fixture_assert_render_value() { # output variable, value
  local fixture_assert_destination=$1 fixture_assert_rendered_value
  fixture_assert_bound_into fixture_assert_rendered_value "$2"
  fixture_assert_rendered_value=${fixture_assert_rendered_value//$'\n'/$'\n            '}
  printf -v "$fixture_assert_destination" '%s' "$fixture_assert_rendered_value"
}

fixture_assert_print_failure() { # what, expected, observed, file, line
  local fixture_assert_expected fixture_assert_observed fixture_assert_label
  fixture_assert_render_value fixture_assert_expected "$2"
  fixture_assert_render_value fixture_assert_observed "$3"
  fixture_assert_label=${1//$'\n'/$'\n            '}
  if printf 'FAIL %s:%s %s:%s %s\n  expected: %s\n  observed: %s\n' \
    "${METASYSTEM_FIXTURE_BED-}" "${METASYSTEM_FIXTURE_SCENARIO_NAME-}" \
    "$4" "$5" "$fixture_assert_label" "$fixture_assert_expected" "$fixture_assert_observed" >&2; then :; fi
  fixture_assert_announce_record_file
}

fixture_assert_engine_object() { # output variable, key=value...
  local fixture_assert_destination=$1 fixture_assert_output fixture_assert_status
  shift
  if fixture_assert_output=$("$fixture_assert_engine" json object "$@" 2>&1); then
    printf -v "$fixture_assert_destination" '%s' "$fixture_assert_output"
    return 0
  else
    fixture_assert_status=$?
    fixture_assert_registry_error="json object exited $fixture_assert_status"
    if [[ -n "$fixture_assert_output" ]]; then
      fixture_assert_registry_error="$fixture_assert_registry_error: $(fixture_assert_one_line "$fixture_assert_output")"
    fi
    return 1
  fi
}

fixture_assert_append_record() { # record
  fixture_assert_prepare_record_paths
  if printf '%s\n' "$1" 2>/dev/null >>"$fixture_assert_record_file"; then
    return 0
  fi
  fixture_assert_mark_unwritten "could not append to $fixture_assert_record_file"
  return 1
}

fixture_assert_encode_registry_value() { # output variable, value
  local fixture_assert_destination=$1 fixture_assert_encoded_value=$2
  fixture_assert_encoded_value=${fixture_assert_encoded_value//\\/\\\\}
  fixture_assert_encoded_value=${fixture_assert_encoded_value//$'\n'/\\n}
  fixture_assert_encoded_value=${fixture_assert_encoded_value//$'\t'/\\t}
  printf -v "$fixture_assert_destination" '%s' "$fixture_assert_encoded_value"
}

fixture_assert_decode_registry_value() { # output variable, encoded value
  local fixture_assert_destination=$1 fixture_assert_decoded_value
  printf -v fixture_assert_decoded_value '%b' "$2"
  printf -v "$fixture_assert_destination" '%s' "$fixture_assert_decoded_value"
}

fixture_assert_find_label() { # what
  local fixture_assert_wanted fixture_assert_status fixture_assert_site_kind fixture_assert_site_label
  local fixture_assert_site_file fixture_assert_site_line fixture_assert_site_row fixture_assert_site_rest
  fixture_assert_duplicate_found=0
  fixture_assert_duplicate_first_file=
  fixture_assert_duplicate_first_line=
  fixture_assert_registry_error=
  fixture_assert_prepare_record_paths
  fixture_assert_encode_registry_value fixture_assert_wanted "$1"
  [[ -e "$fixture_assert_sidecar_file" ]] || return 0
  [[ -r "$fixture_assert_sidecar_file" ]] || {
    fixture_assert_registry_error="could not read label registry $fixture_assert_sidecar_file"
    return 0
  }
  if LC_ALL=C grep -F -x -- "$fixture_assert_wanted" "$fixture_assert_sidecar_file" >/dev/null 2>&1; then
    fixture_assert_duplicate_found=1
  else
    fixture_assert_status=$?
    if (( fixture_assert_status == 1 )); then
      return 0
    fi
    fixture_assert_registry_error="could not search label registry $fixture_assert_sidecar_file: grep exited $fixture_assert_status"
    return 0
  fi
  if [[ ! -r "$fixture_assert_sidecar_sites_file" ]]; then
    fixture_assert_registry_error="could not read label call sites $fixture_assert_sidecar_sites_file"
    return 0
  fi
  while IFS= read -r fixture_assert_site_row || [[ -n "$fixture_assert_site_row" ]]; do
    fixture_assert_site_kind=${fixture_assert_site_row%%$'\t'*}
    fixture_assert_site_rest=${fixture_assert_site_row#*$'\t'}
    [[ "$fixture_assert_site_kind" == call ]] || continue
    fixture_assert_site_label=${fixture_assert_site_rest%%$'\t'*}
    fixture_assert_site_rest=${fixture_assert_site_rest#*$'\t'}
    [[ "$fixture_assert_site_label" == "$fixture_assert_wanted" ]] || continue
    fixture_assert_site_file=${fixture_assert_site_rest%%$'\t'*}
    fixture_assert_site_line=${fixture_assert_site_rest#*$'\t'}
    fixture_assert_decode_registry_value fixture_assert_duplicate_first_file "$fixture_assert_site_file"
    fixture_assert_duplicate_first_line=$fixture_assert_site_line
    return 0
  done <"$fixture_assert_sidecar_sites_file"
  fixture_assert_registry_error="label registry has no call site for $fixture_assert_wanted"
}

fixture_assert_report_registry_error() {
  if [[ -n "$fixture_assert_registry_error" ]]; then
    fixture_assert_mark_unwritten "$fixture_assert_registry_error"
    fixture_assert_registry_error=
  fi
}

fixture_assert_write_label_call() { # what, file, line
  local fixture_assert_encoded_label fixture_assert_encoded_file
  fixture_assert_prepare_record_paths
  fixture_assert_encode_registry_value fixture_assert_encoded_label "$1"
  fixture_assert_encode_registry_value fixture_assert_encoded_file "$2"
  if ! printf 'call\t%s\t%s\t%s\n' "$fixture_assert_encoded_label" "$fixture_assert_encoded_file" "$3" \
    2>/dev/null >>"$fixture_assert_sidecar_sites_file"; then
    fixture_assert_mark_unwritten "could not append to label call sites $fixture_assert_sidecar_sites_file"
    return 0
  fi
  if printf '%s\n' "$fixture_assert_encoded_label" 2>/dev/null >>"$fixture_assert_sidecar_file"; then
    return 0
  fi
  fixture_assert_mark_unwritten "could not append to label registry $fixture_assert_sidecar_file"
  return 0
}

fixture_assert_leg() {
  local fixture_assert_leg_file=${METASYSTEM_FIXTURE_LEG_FILE-}
  if [[ -n "$fixture_assert_leg_file" && -r "$fixture_assert_leg_file" ]]; then
    IFS=$' \t' read -r fixture_assert_context_leg fixture_assert_ignored <"$fixture_assert_leg_file" || :
    printf '%s' "${fixture_assert_context_leg-}"
  fi
}

fixture_assert_digest_file() { # output variable, value, temp file
  local fixture_assert_destination=$1 fixture_assert_value=$2 fixture_assert_temp=$3
  local fixture_assert_output fixture_assert_status
  if ! printf '%s' "$fixture_assert_value" >"$fixture_assert_temp"; then
    fixture_assert_mark_unwritten "could not write digest input $fixture_assert_temp"
    return 1
  fi
  if fixture_assert_output=$("$fixture_assert_engine" util sha256 --file "$fixture_assert_temp" 2>&1); then
    printf -v "$fixture_assert_destination" '%s' "$fixture_assert_output"
    return 0
  else
    fixture_assert_status=$?
    if [[ -n "$fixture_assert_output" ]]; then
      fixture_assert_mark_unwritten "util sha256 exited $fixture_assert_status: $(fixture_assert_one_line "$fixture_assert_output")"
    else
      fixture_assert_mark_unwritten "util sha256 exited $fixture_assert_status"
    fi
    return 1
  fi
}

fixture_assert_record_assertion() { # what, expected, observed, file, line
  local fixture_assert_expected fixture_assert_observed fixture_assert_expected_digest fixture_assert_observed_digest
  local fixture_assert_temp fixture_assert_record fixture_assert_leg_value
  fixture_assert_bound_into fixture_assert_expected "$2"
  fixture_assert_bound_into fixture_assert_observed "$3"
  if ! fixture_assert_temp=$(mktemp "$fixture_assert_digest_tmpdir/metasystem-fixture-assert.XXXXXX" 2>/dev/null); then
    fixture_assert_mark_unwritten "could not create a digest input file in $fixture_assert_digest_tmpdir"
    return 0
  fi
  if ! fixture_assert_digest_file fixture_assert_expected_digest "$2" "$fixture_assert_temp"; then
    rm -f "$fixture_assert_temp" || :
    return 0
  fi
  if ! fixture_assert_digest_file fixture_assert_observed_digest "$3" "$fixture_assert_temp"; then
    rm -f "$fixture_assert_temp" || :
    return 0
  fi
  rm -f "$fixture_assert_temp" || :
  fixture_assert_leg_value=$(fixture_assert_leg)
  fixture_assert_registry_error=
  if ! fixture_assert_engine_object fixture_assert_record \
    kind=assertion "bed=${METASYSTEM_FIXTURE_BED-}" \
    "scenario=${METASYSTEM_FIXTURE_SCENARIO_NAME-}" "leg=$fixture_assert_leg_value" \
    "file=$4" "line=$5" "what=$1" "expected=$fixture_assert_expected" \
    "observed=$fixture_assert_observed" "expectedDigest=$fixture_assert_expected_digest" \
    "observedDigest=$fixture_assert_observed_digest"; then
    fixture_assert_report_registry_error
    return 0
  fi
  fixture_assert_append_record "$fixture_assert_record" || :
}

fixture_assert_record_duplicate() { # what, first file, first line, file, line
  local fixture_assert_record
  fixture_assert_registry_error=
  if ! fixture_assert_engine_object fixture_assert_record \
    kind=duplicate-label "bed=${METASYSTEM_FIXTURE_BED-}" \
    "scenario=${METASYSTEM_FIXTURE_SCENARIO_NAME-}" "what=$1" \
    "firstFile=$2" "firstLine=$3" "file=$4" "line=$5"; then
    fixture_assert_report_registry_error
    return 0
  fi
  fixture_assert_append_record "$fixture_assert_record" || :
}

fixture_assert_handle() { # what, expected, observed, passed, source, line
  local fixture_assert_what=$1 fixture_assert_expected=$2 fixture_assert_observed=$3
  local fixture_assert_passed=$4 fixture_assert_source=$5 fixture_assert_line=$6
  local fixture_assert_pending_registry_error fixture_assert_duplicate_observed
  fixture_assert_set_relative_file "$fixture_assert_source"
  fixture_assert_find_label "$fixture_assert_what"
  fixture_assert_pending_registry_error=$fixture_assert_registry_error
  fixture_assert_last_failed=0
  if (( fixture_assert_duplicate_found )); then
    fixture_assert_duplicate_observed="label reused; first call at $fixture_assert_duplicate_first_file:$fixture_assert_duplicate_first_line"
    fixture_assert_print_failure "$fixture_assert_what" 'unique label' "$fixture_assert_duplicate_observed" \
      "$fixture_assert_relative_file" "$fixture_assert_line"
    fixture_assert_last_failed=1
  elif (( ! fixture_assert_passed )); then
    fixture_assert_print_failure "$fixture_assert_what" "$fixture_assert_expected" "$fixture_assert_observed" \
      "$fixture_assert_relative_file" "$fixture_assert_line"
    fixture_assert_last_failed=1
  fi
  fixture_assert_registry_error=$fixture_assert_pending_registry_error
  fixture_assert_report_registry_error
  fixture_assert_write_label_call "$fixture_assert_what" "$fixture_assert_relative_file" "$fixture_assert_line"
  if (( fixture_assert_duplicate_found )); then
    fixture_assert_record_duplicate "$fixture_assert_what" "$fixture_assert_duplicate_first_file" \
      "$fixture_assert_duplicate_first_line" "$fixture_assert_relative_file" "$fixture_assert_line"
  elif (( ! fixture_assert_passed )); then
    fixture_assert_record_assertion "$fixture_assert_what" "$fixture_assert_expected" "$fixture_assert_observed" \
      "$fixture_assert_relative_file" "$fixture_assert_line"
  fi
  return 0
}

fixture_assert_load_file() { # output variable, file
  local fixture_assert_destination=$1 fixture_assert_file=$2 fixture_assert_content fixture_assert_status
  if fixture_assert_content=$(command cat -- "$fixture_assert_file"; fixture_assert_status=$?; printf '\034'; exit "$fixture_assert_status"); then
    fixture_assert_content=${fixture_assert_content%?}
    printf -v "$fixture_assert_destination" '%s' "$fixture_assert_content"
    return 0
  fi
  return 1
}

fixture_assert_contains_impl() { # require flag, negate flag, source, line, arguments...
  local fixture_assert_required=$1 fixture_assert_negated=$2 fixture_assert_source=$3 fixture_assert_line=$4
  local fixture_assert_mode=-F fixture_assert_what fixture_assert_pattern fixture_assert_subject
  local fixture_assert_match=0 fixture_assert_grep_status fixture_assert_observed fixture_assert_passed=0
  local fixture_assert_error=0 fixture_assert_subject_file= fixture_assert_temp_output fixture_assert_temp_status
  shift 4
  if [[ ${1-} == -E ]]; then
    fixture_assert_mode=-E
    shift
  fi
  fixture_assert_what=$1
  fixture_assert_pattern=$2
  fixture_assert_subject=$3
  if [[ -f "$fixture_assert_subject" ]]; then
    if grep "$fixture_assert_mode" -- "$fixture_assert_pattern" "$fixture_assert_subject" >/dev/null 2>&1; then
      fixture_assert_match=1
      fixture_assert_grep_status=0
    else
      fixture_assert_grep_status=$?
    fi
    if (( fixture_assert_grep_status > 1 )); then
      fixture_assert_observed="<grep exit $fixture_assert_grep_status while reading $fixture_assert_subject>"
      fixture_assert_error=1
    elif (( fixture_assert_match == fixture_assert_negated )); then
      if ! fixture_assert_load_file fixture_assert_observed "$fixture_assert_subject"; then
        fixture_assert_observed="<could not read $fixture_assert_subject>"
        fixture_assert_error=1
      fi
    else
      fixture_assert_observed=
    fi
  else
    fixture_assert_observed=$fixture_assert_subject
    if fixture_assert_temp_output=$(mktemp "${TMPDIR:-/tmp}/metasystem-fixture-assert-subject.XXXXXX" 2>&1); then
      fixture_assert_subject_file=$fixture_assert_temp_output
    else
      fixture_assert_temp_status=$?
      fixture_assert_observed="<could not create string subject file in ${TMPDIR:-/tmp}: mktemp exited $fixture_assert_temp_status"
      if [[ -n "$fixture_assert_temp_output" ]]; then
        fixture_assert_observed="$fixture_assert_observed: $(fixture_assert_one_line "$fixture_assert_temp_output")"
      fi
      fixture_assert_observed="$fixture_assert_observed>"
      fixture_assert_error=1
    fi
    if (( ! fixture_assert_error )); then
      if ! printf '%s' "$fixture_assert_subject" 2>/dev/null >"$fixture_assert_subject_file"; then
        fixture_assert_observed="<could not write string subject to $fixture_assert_subject_file>"
        fixture_assert_error=1
      elif grep "$fixture_assert_mode" -- "$fixture_assert_pattern" "$fixture_assert_subject_file" >/dev/null 2>&1; then
        fixture_assert_match=1
        fixture_assert_grep_status=0
      else
        fixture_assert_grep_status=$?
        if (( fixture_assert_grep_status > 1 )); then
          fixture_assert_observed="<grep exit $fixture_assert_grep_status while reading string subject>"
          fixture_assert_error=1
        fi
      fi
    fi
    if [[ -n "$fixture_assert_subject_file" ]]; then
      rm -f "$fixture_assert_subject_file" || :
    fi
  fi
  if (( ! fixture_assert_error )); then
    if (( fixture_assert_negated )); then
      fixture_assert_passed=$((! fixture_assert_match))
    else
      fixture_assert_passed=$fixture_assert_match
    fi
  fi
  if (( fixture_assert_negated )); then
    fixture_assert_handle "$fixture_assert_what" "does not contain: $fixture_assert_pattern" \
      "$fixture_assert_observed" "$fixture_assert_passed" "$fixture_assert_source" "$fixture_assert_line"
  else
    fixture_assert_handle "$fixture_assert_what" "contains: $fixture_assert_pattern" \
      "$fixture_assert_observed" "$fixture_assert_passed" "$fixture_assert_source" "$fixture_assert_line"
  fi
  if (( fixture_assert_required && fixture_assert_last_failed )); then
    fixture_assert_write_terminal require "$fixture_assert_what"
    exit 1
  fi
  return 0
}

fixture_assert_json_impl() { # require flag, source, line, what, file, field, expected
  local fixture_assert_required=$1 fixture_assert_source=$2 fixture_assert_line=$3
  local fixture_assert_what=$4 fixture_assert_file=$5 fixture_assert_field=$6 fixture_assert_expected=$7
  local fixture_assert_captured fixture_assert_status fixture_assert_observed fixture_assert_reason fixture_assert_passed=0
  if fixture_assert_captured=$("$fixture_assert_engine" json get --file "$fixture_assert_file" --field "$fixture_assert_field" 2>&1; fixture_assert_status=$?; printf '\034'; exit "$fixture_assert_status"); then
    fixture_assert_captured=${fixture_assert_captured%?}
    fixture_assert_observed=${fixture_assert_captured%$'\n'}
    [[ "$fixture_assert_observed" == "$fixture_assert_expected" ]] && fixture_assert_passed=1
  else
    fixture_assert_status=$?
    fixture_assert_captured=${fixture_assert_captured%?}
    fixture_assert_captured=${fixture_assert_captured%$'\n'}
    case "$fixture_assert_status" in
      1) fixture_assert_reason='file missing or unparseable' ;;
      3) fixture_assert_reason='field absent' ;;
      *) fixture_assert_reason='json get failed' ;;
    esac
    fixture_assert_observed="<json get exit $fixture_assert_status: $fixture_assert_reason"
    if [[ -n "$fixture_assert_captured" ]]; then
      fixture_assert_observed="$fixture_assert_observed; stderr: $fixture_assert_captured"
    fi
    fixture_assert_observed="$fixture_assert_observed>"
  fi
  fixture_assert_handle "$fixture_assert_what" "$fixture_assert_expected" "$fixture_assert_observed" \
    "$fixture_assert_passed" "$fixture_assert_source" "$fixture_assert_line"
  if (( fixture_assert_required && fixture_assert_last_failed )); then
    fixture_assert_write_terminal require "$fixture_assert_what"
    exit 1
  fi
  return 0
}

fixture_assert_count_failures() {
  local fixture_assert_output fixture_assert_status
  fixture_assert_failure_count=0
  fixture_assert_prepare_record_paths
  [[ -e "$fixture_assert_record_file" ]] || return 0
  if [[ ! -r "$fixture_assert_record_file" ]]; then
    fixture_assert_mark_unwritten "could not read records file $fixture_assert_record_file"
    return 0
  fi
  if fixture_assert_output=$(grep -F -c \
    -e '"kind":"assertion"' -e '"kind":"duplicate-label"' \
    "$fixture_assert_record_file" 2>&1); then
    fixture_assert_failure_count=$fixture_assert_output
    return 0
  fi
  fixture_assert_status=$?
  if (( fixture_assert_status == 1 )) && [[ "$fixture_assert_output" =~ ^[0-9]+$ ]]; then
    fixture_assert_failure_count=$fixture_assert_output
    return 0
  fi
  fixture_assert_mark_unwritten "could not count records in $fixture_assert_record_file: grep exited $fixture_assert_status"
}

fixture_assert_has_unwritten() {
  local fixture_assert_grep_status
  (( fixture_assert_unwritten_record )) && return 0
  fixture_assert_prepare_record_paths
  [[ -e "$fixture_assert_sidecar_sites_file" ]] || return 1
  if grep -Fqx 'unwritten-record' "$fixture_assert_sidecar_sites_file" >/dev/null 2>&1; then
    return 0
  else
    fixture_assert_grep_status=$?
    (( fixture_assert_grep_status == 1 )) && return 1
    fixture_assert_mark_unwritten "could not read assertion state $fixture_assert_sidecar_sites_file"
    return 0
  fi
}

fixture_assert_write_terminal() { # by, what
  local fixture_assert_by=$1 fixture_assert_what=$2 fixture_assert_base fixture_assert_record
  local fixture_assert_count
  fixture_assert_terminal_written=0
  fixture_assert_count_failures
  fixture_assert_count=$fixture_assert_failure_count
  if [[ ! "$fixture_assert_count" =~ ^[0-9]+$ ]]; then
    fixture_assert_mark_unwritten "terminal failure count is not a non-negative integer: $fixture_assert_count"
    return 0
  fi
  fixture_assert_registry_error=
  if ! fixture_assert_engine_object fixture_assert_base \
    kind=terminal "bed=${METASYSTEM_FIXTURE_BED-}" \
    "scenario=${METASYSTEM_FIXTURE_SCENARIO_NAME-}" "by=$fixture_assert_by" "what=$fixture_assert_what"; then
    fixture_assert_report_registry_error
    return 0
  fi
  case "$fixture_assert_base" in
    *'}') fixture_assert_record=${fixture_assert_base%'}'},\"failures\":$fixture_assert_count} ;;
    *)
      fixture_assert_mark_unwritten 'json object did not end the terminal record with a closing brace'
      return 0
      ;;
  esac
  if fixture_assert_append_record "$fixture_assert_record"; then
    fixture_assert_terminal_written=1
  fi
  return 0
}

assert_equal() {
  local fixture_assert_passed=0
  [[ "$2" == "$3" ]] && fixture_assert_passed=1
  fixture_assert_handle "$1" "$2" "$3" "$fixture_assert_passed" \
    "${BASH_SOURCE[1]-$fixture_assert_source_spelling}" "${BASH_LINENO[0]-0}"
}

assert_contains() {
  fixture_assert_contains_impl 0 0 "${BASH_SOURCE[1]-$fixture_assert_source_spelling}" "${BASH_LINENO[0]-0}" "$@"
}

assert_not_contains() {
  fixture_assert_contains_impl 0 1 "${BASH_SOURCE[1]-$fixture_assert_source_spelling}" "${BASH_LINENO[0]-0}" "$@"
}

assert_json_field() {
  fixture_assert_json_impl 0 "${BASH_SOURCE[1]-$fixture_assert_source_spelling}" "${BASH_LINENO[0]-0}" "$@"
}

assert_exit() {
  local fixture_assert_passed=0
  [[ "$2" == "$3" ]] && fixture_assert_passed=1
  fixture_assert_handle "$1" "$2" "$3" "$fixture_assert_passed" \
    "${BASH_SOURCE[1]-$fixture_assert_source_spelling}" "${BASH_LINENO[0]-0}"
}

require_equal() {
  local fixture_assert_passed=0
  [[ "$2" == "$3" ]] && fixture_assert_passed=1
  fixture_assert_handle "$1" "$2" "$3" "$fixture_assert_passed" \
    "${BASH_SOURCE[1]-$fixture_assert_source_spelling}" "${BASH_LINENO[0]-0}"
  if (( fixture_assert_last_failed )); then
    fixture_assert_write_terminal require "$1"
    exit 1
  fi
}

require_contains() {
  fixture_assert_contains_impl 1 0 "${BASH_SOURCE[1]-$fixture_assert_source_spelling}" "${BASH_LINENO[0]-0}" "$@"
}

require_json_field() {
  fixture_assert_json_impl 1 "${BASH_SOURCE[1]-$fixture_assert_source_spelling}" "${BASH_LINENO[0]-0}" "$@"
}

require_exit() {
  local fixture_assert_passed=0
  [[ "$2" == "$3" ]] && fixture_assert_passed=1
  fixture_assert_handle "$1" "$2" "$3" "$fixture_assert_passed" \
    "${BASH_SOURCE[1]-$fixture_assert_source_spelling}" "${BASH_LINENO[0]-0}"
  if (( fixture_assert_last_failed )); then
    fixture_assert_write_terminal require "$1"
    exit 1
  fi
}

fixture_fail() {
  fixture_assert_handle "$1" "$2" "$3" 0 \
    "${BASH_SOURCE[1]-$fixture_assert_source_spelling}" "${BASH_LINENO[0]-0}"
}

fixture_abort() {
  fixture_assert_handle "$1" "$2" "$3" 0 \
    "${BASH_SOURCE[1]-$fixture_assert_source_spelling}" "${BASH_LINENO[0]-0}"
  fixture_assert_write_terminal abort "$1"
  exit 1
}

fixture_assert_finish() {
  local fixture_assert_finish_count fixture_assert_finish_unwritten=0
  fixture_assert_write_terminal finish ''
  fixture_assert_finish_count=$fixture_assert_failure_count
  fixture_assert_has_unwritten && fixture_assert_finish_unwritten=1
  if (( fixture_assert_finish_count > 0 || fixture_assert_finish_unwritten || ! fixture_assert_terminal_written )); then
    exit 1
  fi
  return 0
}
