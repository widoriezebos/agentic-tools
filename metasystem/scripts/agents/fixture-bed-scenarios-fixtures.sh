#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
source "$root/scripts/agents/fixture-budget.sh"
ms=${METASYSTEM_BIN:-$root/bin/metasystem}
source "$root/scripts/agents/fixture-assert.sh"
source "$root/scripts/agents/fixture-bed-scenarios.sh"
fixture_bed_child=0
fixture_scenario=
if fixture_scenario=$(harness_fixture_bed_child_scenario fixture-bed-scenarios "$@"); then
  fixture_bed_child=1
else
  fixture_bed_child_rc=$?
  [[ $fixture_bed_child_rc -eq 1 ]] || exit "$fixture_bed_child_rc"
fi
unset METASYSTEM_FIXTURE_SCENARIO

fixture_check_failed() { # description
  printf 'fixture-bed-scenarios check failed: %s\n' "$1" >&2
  exit 1
}

if (( ! fixture_bed_child )); then
  fixture_bed_script=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/$(basename "${BASH_SOURCE[0]}")
  run_fixture_bed_scenarios fixture-bed-scenarios \
    "fixture-bed-scenarios fixtures passed (28 isolated scenarios)" \
    "$fixture_bed_script" budget-standalone budget-inherited ceiling-reaps-group ceiling-expiry-mutation signal-reaps-group \
    hang-leash \
    command-substitution-failure collects-every-failure \
    assert-keeps-going-under-errexit assert-finish-exits-from-count \
    require-stops-with-terminal-record assert-duplicate-label-refused \
    assert-records-digests assert-prints-block assert-bounds-sides \
    assert-records-jsonl assert-path-relative assert-bash32 \
    assert-counts-across-subshells assert-json-field-read-errors \
    assert-survives-broken-engine-and-unwritable-records \
    multi-line-value-stays-one-block contains-read-error-never-passes \
    passing-calls-start-no-engine fallback-records-clean-and-announce \
    require-exit-stops-with-terminal-record require-json-field-stops-with-terminal-record \
    fixture-has-no-inert-checks
fi

harness_fixture_bed_leg "$fixture_scenario"

tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-fixture-bed-scenarios.XXXXXX")
trap 'rm -rf "$tmp"' EXIT

write_inner_bed() { # destination, optional controlled scheduler clock
  local destination=$1 controlled_clock=${2:-false}
  cat >"$destination" <<'INNER'
#!/usr/bin/env bash
set -euo pipefail

if [ -n "${METASYSTEM_FIXTURE_OWNER-}" ]; then
  tag="METASYSTEM_FIXTURE_OWNER=$METASYSTEM_FIXTURE_OWNER"
  attempt_tag=
  [ -z "${METASYSTEM_FIXTURE_ATTEMPT-}" ] || attempt_tag="METASYSTEM_FIXTURE_ATTEMPT=$METASYSTEM_FIXTURE_ATTEMPT"
  if [ "${1-}" != "$tag" ]; then
    [ -z "$attempt_tag" ] || exec /bin/bash "$0" "$tag" "$attempt_tag" "$@"
    exec /bin/bash "$0" "$tag" "$@"
  fi
  if [ -n "$attempt_tag" ] && [ "${2-}" != "$attempt_tag" ]; then
    shift
    exec /bin/bash "$0" "$tag" "$attempt_tag" "$@"
  fi
  shift
  [ -z "$attempt_tag" ] || shift
fi

root=${FIXTURE_BED_SOURCE_ROOT:?}
source "$root/scripts/agents/fixture-budget.sh"
source "$root/scripts/agents/fixture-bed-scenarios.sh"
INNER
  if [[ "$controlled_clock" == true ]]; then
    cat >>"$destination" <<'INNER_CLOCK'
# These wrappers exist only in the generated fixture program and are not
# exported. The scheduler clock moves on its first liveness probe, after the
# deadline and descendants both exist; the reaper clock moves separately only
# after its TERM deadline exists.
fixture_bed_scheduler_clock_advanced=0
fixture_bed_reaper_clock_advanced=0
kill() {
  local caller=${FUNCNAME[1]:-} ready_until
  if [[ "$caller" == run_fixture_bed_scenarios && $fixture_bed_scheduler_clock_advanced -eq 0 ]]; then
    [[ ${#live_deadlines[@]} -eq 1 ]] \
      || { echo "controlled fixture-bed scheduler expected one live deadline" >&2; return 1; }
    ready_until=$((SECONDS + $(harness_fixture_cap suite-watchdog-reap)))
    while { [[ ! -s "${FIXTURE_BED_CHILD_PID_FILE:?}" ]] ||
        [[ ! -s "${FIXTURE_BED_GRANDCHILD_PID_FILE:?}" ]]; } &&
        (( SECONDS < ready_until )); do
      /bin/sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
    done
    [[ -s "$FIXTURE_BED_CHILD_PID_FILE" && -s "$FIXTURE_BED_GRANDCHILD_PID_FILE" ]] \
      || { echo "controlled fixture-bed scheduler did not observe child readiness" >&2; return 1; }
    SECONDS=${live_deadlines[0]}
    IFS= read -r fixture_bed_ready_child <"$FIXTURE_BED_CHILD_PID_FILE"
    IFS= read -r fixture_bed_ready_grandchild <"$FIXTURE_BED_GRANDCHILD_PID_FILE"
    if (( SECONDS < live_deadlines[0] )); then
      if [[ ${FIXTURE_BED_EXPECT_EXPIRY_MUTATION:-0} == 1 ]]; then
        fixture_bed_ready_child_ref=$(harness_fixture_engine_call proc ref --pid "$fixture_bed_ready_child") \
          || { echo "controlled fixture-bed scheduler lost the child identity" >&2; return 1; }
        fixture_bed_ready_grandchild_ref=$(harness_fixture_engine_call proc ref --pid "$fixture_bed_ready_grandchild") \
          || { echo "controlled fixture-bed scheduler lost the grandchild identity" >&2; return 1; }
        printf 'missing-expiry %s %s %s %s\n' "$fixture_bed_ready_child" "$fixture_bed_ready_child_ref" \
          "$fixture_bed_ready_grandchild" "$fixture_bed_ready_grandchild_ref" \
          >"${FIXTURE_BED_EXPIRY_MUTATION_WITNESS:?}"
        fixture_bed_scheduler_clock_advanced=1
        fixture_bed_reap_group "$fixture_bed_ready_child" \
          2>>"${FIXTURE_BED_EXPIRY_MUTATION_WITNESS:?}" || true
        echo "controlled fixture-bed scheduler rejected missing expiry" >&2
        return 1
      fi
      echo "controlled fixture-bed scheduler did not apply its initialized expiry" >&2
      return 1
    fi
    printf '%s %s %s %s\n' "$fixture_bed_ready_child" "$fixture_bed_ready_grandchild" \
      "$SECONDS" "${live_deadlines[0]}" >"${FIXTURE_BED_SCHEDULER_READY_FILE:?}"
    fixture_bed_scheduler_clock_advanced=1
  fi
  builtin kill "$@"
}
sleep() {
  local caller=${FUNCNAME[1]:-}
  if [[ "$caller" == fixture_bed_reap_group && $fixture_bed_reaper_clock_advanced -eq 0 ]]; then
    [[ ${deadline:-} =~ ^[0-9]+$ ]] \
      || { echo "controlled fixture-bed reaper found no initialized TERM deadline" >&2; return 1; }
    SECONDS=$deadline
    fixture_bed_reaper_clock_advanced=1
    return 0
  fi
  /bin/sleep "$@"
}
INNER_CLOCK
  fi
  cat >>"$destination" <<'INNER'
fixture_bed_publish_custodian_log() {
  [[ -z "${FIXTURE_BED_CUSTODIAN_LOG_FILE:-}" ]] \
    || printf '%s\n' "$harness_fixture_custodian_log" >"$FIXTURE_BED_CUSTODIAN_LOG_FILE"
}
if [[ -n "${FIXTURE_BED_CUSTODIAN_LOG_FILE:-}" ]]; then
  fixture_bed_prepare_hook=fixture_bed_publish_custodian_log
fi
fixture_bed_child=0
fixture_scenario=
if fixture_scenario=$(harness_fixture_bed_child_scenario fixture-bed-inner "$@"); then
  fixture_bed_child=1
else
  fixture_bed_child_rc=$?
  [[ $fixture_bed_child_rc -eq 1 ]] || exit "$fixture_bed_child_rc"
fi
unset METASYSTEM_FIXTURE_SCENARIO
if (( ! fixture_bed_child )); then
  run_fixture_bed_scenarios fixture-bed-inner "fixture-bed inner scenario passed" \
    "$0" ${FIXTURE_BED_INNER_SCENARIOS:?}
fi

case "$fixture_scenario" in
  print-scale)
    printf '%s\n' "$METASYSTEM_FIXTURE_CAP_SCALE_MILLI"
    ;;
  hang)
    harness_fixture_bed_leg hang
    trap 'exit 143' TERM
    printf '%s\n' "$$" >"${FIXTURE_BED_CHILD_PID_FILE:?}"
    bash -c 'trap "" TERM; printf "%s\n" "$$" >"$FIXTURE_BED_GRANDCHILD_PID_FILE"; exec 3<"$METASYSTEM_FIXTURE_LEASH"; read -r _ <&3' \
      bash "METASYSTEM_FIXTURE_OWNER=$METASYSTEM_FIXTURE_OWNER" &
    fixture_grandchild_pid=$!
    exec 3<"$METASYSTEM_FIXTURE_LEASH"
    read -r _ <&3
    ;;
  json-read-fails | exit-fails) ;;
  pass)
    harness_fixture_bed_leg passing-leg
    ;;
  fail-one)
    harness_fixture_bed_leg first-failing-leg
    false
    ;;
  fail-two)
    harness_fixture_bed_leg second-failing-leg
    false
    ;;
  *)
    echo "fixture-bed inner scenario is unknown: $fixture_scenario" >&2
    exit 64
    ;;
esac

# Keep the failing substitution in the same top-level if compound used by
# converted beds. The failure guard must not invent a Bash 3.2 source line.
if [[ "$fixture_scenario" == json-read-fails ]]; then
  harness_fixture_bed_leg read-broken-json
  read_fixture_json() { [[ "$1" == '{}' ]]; }
  json_value=$(read_fixture_json '{broken')
  printf '%s\n' "$json_value"
fi

if [[ "$fixture_scenario" == exit-fails ]]; then
  harness_fixture_bed_leg explicit-exit
  exit 7
fi
INNER
  chmod +x "$destination"
}

assert_no_group_survivor() { # direct child pid, grandchild pid
  local child_pid=$1 grandchild_pid=$2
  if kill -0 "$grandchild_pid" 2>/dev/null; then
    echo "fixture-bed-scenarios fixture left grandchild $grandchild_pid alive" >&2
    return 1
  fi
  if kill -0 -- "-$child_pid" 2>/dev/null; then
    echo "fixture-bed-scenarios fixture left process group $child_pid alive" >&2
    return 1
  fi
}

wait_fixture_ref_gone() { # description, pid, exact ref, optional cap name
  local description=$1 pid=$2 ref=$3 cap=${4:-suite-watchdog-reap} current deadline
  deadline=$((SECONDS + $(harness_fixture_cap "$cap")))
  while current=$(harness_fixture_engine_call proc ref --pid "$pid" 2>/dev/null) \
      && [[ "$current" == "$ref" ]] && (( SECONDS < deadline )); do
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
  done
  if current=$(harness_fixture_engine_call proc ref --pid "$pid" 2>/dev/null) && [[ "$current" == "$ref" ]]; then
    echo "fixture-bed-scenarios fixture: $description remained alive at $ref" >&2
    return 1
  fi
}

wait_fixture_log_line() { # log, fixed line fragment, optional cap name
  local log=$1 fragment=$2 cap=${3:-suite-watchdog-reap} deadline
  deadline=$((SECONDS + $(harness_fixture_cap "$cap")))
  while (( SECONDS < deadline )); do
    [[ -f "$log" ]] && grep -Fq "$fragment" "$log" && return 0
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
  done
  echo "fixture-bed-scenarios fixture: custodian log did not contain $fragment" >&2
  [[ ! -f "$log" ]] || cat "$log" >&2
  return 1
}

fixture_assert_source_path=metasystem/scripts/agents/fixture-bed-scenarios-fixtures.sh

if [[ "$fixture_scenario" == assert-keeps-going-under-errexit ]]; then
  assertion_records=$tmp/assertions.jsonl
  assertion_stderr=$tmp/assertions.stderr
  assertion_end=$tmp/assertions.reached-end
  assertion_large_file=$tmp/assertion-large-file
  assertion_cat_dir=$tmp/cat-bin
  assertion_cat_marker=$tmp/cat-called
  mkdir "$assertion_cat_dir"
  awk 'BEGIN { for (i = 0; i < 4096; i++) printf "x"; printf "needle-17\n" }' >"$assertion_large_file"
  cat >"$assertion_cat_dir/cat" <<'ASSERTION_CAT'
#!/usr/bin/env bash
printf 'called\n' >>"${FIXTURE_ASSERT_CAT_MARKER:?}"
exec /bin/cat "$@"
ASSERTION_CAT
  chmod +x "$assertion_cat_dir/cat"
  METASYSTEM_BED_FAILURES=$assertion_records
  {
    PATH="$assertion_cat_dir:$PATH" FIXTURE_ASSERT_CAT_MARKER=$assertion_cat_marker \
      assert_contains -E 'large file present pattern' 'needle-[0-9]+' "$assertion_large_file"
    PATH="$assertion_cat_dir:$PATH" FIXTURE_ASSERT_CAT_MARKER=$assertion_cat_marker \
      assert_not_contains 'large file absent pattern' absent "$assertion_large_file"
    [[ ! -e "$assertion_cat_marker" ]] || fixture_check_failed \
      "large passing file checks unexpectedly read the whole file: marker=$assertion_cat_marker"
    assert_equal 'equal failure keeps going' expected observed
    PATH="$assertion_cat_dir:$PATH" FIXTURE_ASSERT_CAT_MARKER=$assertion_cat_marker \
      assert_contains 'contains failure keeps going' absent "$assertion_large_file"
    [[ -s "$assertion_cat_marker" ]] || fixture_check_failed \
      "failing file check did not read the whole file: marker=$assertion_cat_marker"
    assert_exit 'exit failure keeps going' 0 7
    printf 'reached\n' >"$assertion_end"
  } 2>"$assertion_stderr"
  [[ -s "$assertion_end" ]] || fixture_check_failed \
    "assertions did not continue under errexit: reached-marker=$assertion_end"
  assertion_fail_count=$(grep -c '^FAIL fixture-bed-scenarios:assert-keeps-going-under-errexit ' "$assertion_stderr")
  [[ $assertion_fail_count -eq 3 ]] || fixture_check_failed \
    "failure block count observed=$assertion_fail_count expected=3"
  assertion_expected_count=$(grep -c '^  expected: ' "$assertion_stderr")
  [[ $assertion_expected_count -eq 3 ]] || fixture_check_failed \
    "expected-side count observed=$assertion_expected_count expected=3"
  assertion_observed_count=$(grep -c '^  observed: ' "$assertion_stderr")
  [[ $assertion_observed_count -eq 3 ]] || fixture_check_failed \
    "observed-side count observed=$assertion_observed_count expected=3"
  assertion_record_count=$(grep -c '"kind":"assertion"' "$assertion_records")
  [[ $assertion_record_count -eq 3 ]] || fixture_check_failed \
    "assertion record count observed=$assertion_record_count expected=3"
  exit 0
fi

if [[ "$fixture_scenario" == assert-finish-exits-from-count ]]; then
  failed_records=$tmp/finish-failed.jsonl
  failed_stderr=$tmp/finish-failed.stderr
  if (
    METASYSTEM_BED_FAILURES=$failed_records
    fixture_fail 'finish counts one failure' expected observed
    fixture_assert_finish
  ) 2>"$failed_stderr"; then
    failed_rc=0
  else
    failed_rc=$?
  fi
  [[ $failed_rc -eq 1 ]] || fixture_check_failed "failed finisher exit observed=$failed_rc expected=1"
  failed_terminal=$(tail -n 1 "$failed_records")
  failed_kind=$("$ms" json get --value "$failed_terminal" --field kind)
  [[ "$failed_kind" == terminal ]] || fixture_check_failed "failed terminal kind observed=$failed_kind expected=terminal"
  failed_by=$("$ms" json get --value "$failed_terminal" --field by)
  [[ "$failed_by" == finish ]] || fixture_check_failed "failed terminal by observed=$failed_by expected=finish"
  failed_count=$("$ms" json get --value "$failed_terminal" --field failures)
  [[ "$failed_count" == 1 ]] || fixture_check_failed "failed terminal count observed=$failed_count expected=1"

  clean_json=$tmp/finish-clean-input.json
  clean_records=$tmp/finish-clean.jsonl
  printf '%s\n' '{"field":"value\n"}' >"$clean_json"
  if (
    METASYSTEM_BED_FAILURES=$clean_records
    assert_not_contains 'clean absence' absent 'present text'
    assert_json_field 'clean JSON field' "$clean_json" field $'value\n'
    require_equal 'clean required equality' same same
    require_contains 'clean required contents' present 'present text'
    require_json_field 'clean required JSON field' "$clean_json" field $'value\n'
    require_exit 'clean required exit' 0 0
    fixture_assert_finish
  ) 2>"$tmp/finish-clean.stderr"; then
    clean_rc=0
  else
    clean_rc=$?
  fi
  [[ $clean_rc -eq 0 ]] || fixture_check_failed "clean finisher exit observed=$clean_rc expected=0"
  clean_terminal=$(tail -n 1 "$clean_records")
  clean_by=$("$ms" json get --value "$clean_terminal" --field by)
  [[ "$clean_by" == finish ]] || fixture_check_failed "clean terminal by observed=$clean_by expected=finish"
  clean_count=$("$ms" json get --value "$clean_terminal" --field failures)
  [[ "$clean_count" == 0 ]] || fixture_check_failed "clean terminal count observed=$clean_count expected=0"
  exit 0
fi

if [[ "$fixture_scenario" == require-stops-with-terminal-record ]]; then
  require_records=$tmp/require.jsonl
  require_stderr=$tmp/require.stderr
  require_continued=$tmp/require-continued
  if (
    METASYSTEM_BED_FAILURES=$require_records
    require_equal 'required precondition' ready blocked
    fixture_fail 'unreachable failure' expected observed
    printf 'continued\n' >"$require_continued"
  ) 2>"$require_stderr"; then
    require_rc=0
  else
    require_rc=$?
  fi
  [[ $require_rc -eq 1 && ! -e "$require_continued" ]] || fixture_check_failed \
    "required equality exit observed=$require_rc continuation-marker=$require_continued expected exit 1 and absent marker"
  require_line_count=$(wc -l <"$require_records")
  [[ $require_line_count -eq 2 ]] || fixture_check_failed "required equality record lines observed=$require_line_count expected=2"
  require_terminal=$(tail -n 1 "$require_records")
  require_kind=$("$ms" json get --value "$require_terminal" --field kind)
  [[ "$require_kind" == terminal ]] || fixture_check_failed "required equality terminal kind observed=$require_kind expected=terminal"
  require_by=$("$ms" json get --value "$require_terminal" --field by)
  [[ "$require_by" == require ]] || fixture_check_failed "required equality terminal by observed=$require_by expected=require"
  require_what=$("$ms" json get --value "$require_terminal" --field what)
  [[ "$require_what" == 'required precondition' ]] || fixture_check_failed \
    "required equality terminal label observed=$require_what expected=required precondition"
  require_count=$("$ms" json get --value "$require_terminal" --field failures)
  [[ "$require_count" == 1 ]] || fixture_check_failed "required equality terminal count observed=$require_count expected=1"

  abort_records=$tmp/abort.jsonl
  if (
    METASYSTEM_BED_FAILURES=$abort_records
    fixture_abort 'explicit fixture abort' ready blocked
  ) 2>"$tmp/abort.stderr"; then
    abort_rc=0
  else
    abort_rc=$?
  fi
  [[ $abort_rc -eq 1 ]] || fixture_check_failed "fixture abort exit observed=$abort_rc expected=1"
  abort_terminal=$(tail -n 1 "$abort_records")
  abort_by=$("$ms" json get --value "$abort_terminal" --field by)
  [[ "$abort_by" == abort ]] || fixture_check_failed "fixture abort terminal by observed=$abort_by expected=abort"
  abort_what=$("$ms" json get --value "$abort_terminal" --field what)
  [[ "$abort_what" == 'explicit fixture abort' ]] || fixture_check_failed \
    "fixture abort terminal label observed=$abort_what expected=explicit fixture abort"
  abort_count=$("$ms" json get --value "$abort_terminal" --field failures)
  [[ "$abort_count" == 1 ]] || fixture_check_failed "fixture abort terminal count observed=$abort_count expected=1"
  exit 0
fi

if [[ "$fixture_scenario" == assert-duplicate-label-refused ]]; then
  duplicate_records=$tmp/duplicate.jsonl
  duplicate_stderr=$tmp/duplicate.stderr
  METASYSTEM_BED_FAILURES=$duplicate_records
  duplicate_first_line=$((LINENO + 1))
  assert_equal 'one comparison label' same same 2>"$duplicate_stderr"
  duplicate_second_line=$((LINENO + 1))
  fixture_fail 'one comparison label' second-expected second-observed 2>>"$duplicate_stderr"
  duplicate_assertion_count=$(grep -c '"kind":"assertion"' "$duplicate_records" || :)
  [[ $duplicate_assertion_count -eq 0 ]] || fixture_check_failed \
    "duplicate-label assertion records observed=$duplicate_assertion_count expected=0"
  duplicate_count=$(grep -c '"kind":"duplicate-label"' "$duplicate_records")
  [[ $duplicate_count -eq 1 ]] || fixture_check_failed "duplicate-label records observed=$duplicate_count expected=1"
  duplicate_record=$(tail -n 1 "$duplicate_records")
  duplicate_what=$("$ms" json get --value "$duplicate_record" --field what)
  [[ "$duplicate_what" == 'one comparison label' ]] || fixture_check_failed \
    "duplicate-label label observed=$duplicate_what expected=one comparison label"
  duplicate_first_file=$("$ms" json get --value "$duplicate_record" --field firstFile)
  [[ "$duplicate_first_file" == "$fixture_assert_source_path" ]] || fixture_check_failed \
    "duplicate-label first file observed=$duplicate_first_file expected=$fixture_assert_source_path"
  duplicate_record_first_line=$("$ms" json get --value "$duplicate_record" --field firstLine)
  [[ "$duplicate_record_first_line" == "$duplicate_first_line" ]] || fixture_check_failed \
    "duplicate-label first line observed=$duplicate_record_first_line expected=$duplicate_first_line"
  duplicate_file=$("$ms" json get --value "$duplicate_record" --field file)
  [[ "$duplicate_file" == "$fixture_assert_source_path" ]] || fixture_check_failed \
    "duplicate-label current file observed=$duplicate_file expected=$fixture_assert_source_path"
  duplicate_record_line=$("$ms" json get --value "$duplicate_record" --field line)
  [[ "$duplicate_record_line" == "$duplicate_second_line" ]] || fixture_check_failed \
    "duplicate-label current line observed=$duplicate_record_line expected=$duplicate_second_line"

  empty_label_records=$tmp/empty-label.jsonl
  empty_label_stderr=$tmp/empty-label.stderr
  METASYSTEM_BED_FAILURES=$empty_label_records
  empty_label_first_line=$((LINENO + 1))
  assert_equal '' same same 2>"$empty_label_stderr"
  empty_label_second_line=$((LINENO + 1))
  fixture_fail '' expected observed 2>>"$empty_label_stderr"
  empty_label_record=$(tail -n 1 "$empty_label_records")
  empty_label_what=$("$ms" json get --value "$empty_label_record" --field what)
  [[ -z "$empty_label_what" ]] || fixture_check_failed "empty duplicate label was changed: observed=$empty_label_what"
  empty_label_first_file=$("$ms" json get --value "$empty_label_record" --field firstFile)
  [[ "$empty_label_first_file" == "$fixture_assert_source_path" ]] || fixture_check_failed \
    "empty-label first file observed=$empty_label_first_file expected=$fixture_assert_source_path"
  empty_label_record_first_line=$("$ms" json get --value "$empty_label_record" --field firstLine)
  [[ "$empty_label_record_first_line" == "$empty_label_first_line" ]] || fixture_check_failed \
    "empty-label first line observed=$empty_label_record_first_line expected=$empty_label_first_line"
  empty_label_record_line=$("$ms" json get --value "$empty_label_record" --field line)
  [[ "$empty_label_record_line" == "$empty_label_second_line" ]] || fixture_check_failed \
    "empty-label current line observed=$empty_label_record_line expected=$empty_label_second_line"

  locale_list=$tmp/locales.txt
  locale -a >"$locale_list"
  invalid_label_locale=
  while IFS= read -r locale_candidate; do
    case "$locale_candidate" in
      *[Uu][Tt][Ff]8*|*[Uu][Tt][Ff]-8*|*[Uu][Tt][Ff]_8*)
        invalid_label_locale=$locale_candidate
        break
        ;;
    esac
  done <"$locale_list"
  if [[ -z "$invalid_label_locale" ]]; then
    printf '%s\n' 'invalid-byte label locale case did not run: locale -a listed no UTF-8 locale'
  else
    invalid_label_records=$tmp/invalid-label.jsonl
    invalid_label_stderr=$tmp/invalid-label.stderr
    invalid_label=$'label-\xff'
    METASYSTEM_BED_FAILURES=$invalid_label_records
    LC_ALL=$invalid_label_locale assert_equal "$invalid_label" same same 2>"$invalid_label_stderr"
    LC_ALL=$invalid_label_locale fixture_fail "$invalid_label" expected observed 2>>"$invalid_label_stderr"
    invalid_label_duplicate_count=$(grep -c '"kind":"duplicate-label"' "$invalid_label_records" || :)
    [[ $invalid_label_duplicate_count -eq 1 ]] || fixture_check_failed \
      "invalid-byte duplicate records observed=$invalid_label_duplicate_count expected=1 locale=$invalid_label_locale"
    invalid_label_assertion_count=$(grep -c '"kind":"assertion"' "$invalid_label_records" || :)
    [[ $invalid_label_assertion_count -eq 0 ]] || fixture_check_failed \
      "invalid-byte assertion records observed=$invalid_label_assertion_count expected=0 locale=$invalid_label_locale"
    printf 'invalid-byte label locale case ran with LC_ALL=%s\n' "$invalid_label_locale"
  fi
  exit 0
fi

if [[ "$fixture_scenario" == assert-records-digests ]]; then
  digest_records=$tmp/digests.jsonl
  digest_stderr=$tmp/digests.stderr
  digest_expected=$(awk 'BEGIN { for (i = 0; i < 3072; i++) printf "e" }')
  digest_observed=$(awk 'BEGIN { for (i = 0; i < 3072; i++) printf "o" }')
  METASYSTEM_BED_FAILURES=$digest_records
  fixture_fail 'full values are digested' "$digest_expected" "$digest_observed" 2>"$digest_stderr"
  printf '%s' "$digest_expected" >"$tmp/digest-expected.full"
  printf '%s' "$digest_observed" >"$tmp/digest-observed.full"
  expected_digest=$("$ms" util sha256 --file "$tmp/digest-expected.full")
  observed_digest=$("$ms" util sha256 --file "$tmp/digest-observed.full")
  recorded_expected_digest=$("$ms" json get --file "$digest_records" --field expectedDigest)
  [[ "$recorded_expected_digest" == "$expected_digest" ]] || fixture_check_failed \
    "expected digest observed=$recorded_expected_digest expected=$expected_digest"
  recorded_observed_digest=$("$ms" json get --file "$digest_records" --field observedDigest)
  [[ "$recorded_observed_digest" == "$observed_digest" ]] || fixture_check_failed \
    "observed digest observed=$recorded_observed_digest expected=$observed_digest"
  exit 0
fi

if [[ "$fixture_scenario" == assert-prints-block ]]; then
  block_records=$tmp/block.jsonl
  block_stderr=$tmp/block.stderr
  block_expected=$tmp/block.expected
  METASYSTEM_BED_FAILURES=$block_records
  block_line=$((LINENO + 1))
  fixture_fail 'byte exact comparison' 'wanted bytes' 'seen bytes' 2>"$block_stderr"
  printf 'FAIL fixture-bed-scenarios:assert-prints-block %s:%s byte exact comparison\n  expected: wanted bytes\n  observed: seen bytes\n' \
    "$fixture_assert_source_path" "$block_line" >"$block_expected"
  cmp "$block_expected" "$block_stderr"
  exit 0
fi

if [[ "$fixture_scenario" == assert-bounds-sides ]]; then
  bounds_records=$tmp/bounds.jsonl
  bounds_stderr=$tmp/bounds.stderr
  long_expected=$(awk 'BEGIN { for (i = 0; i < 1536; i++) printf "\303\251" }')
  long_observed=$(awk 'BEGIN { for (i = 0; i < 1536; i++) printf "\303\270" }')
  bounded_expected=$(awk 'BEGIN { for (i = 0; i < 1024; i++) printf "\303\251" }')'[... 1024 more bytes]'
  bounded_observed=$(awk 'BEGIN { for (i = 0; i < 1024; i++) printf "\303\270" }')'[... 1024 more bytes]'
  exact_three_byte=$(awk 'BEGIN { for (i = 0; i < 2045; i++) printf "x"; printf "\342\202\254" }')
  over_three_byte=$(awk 'BEGIN { for (i = 0; i < 2046; i++) printf "x"; printf "\342\202\254" }')
  bounded_over_three_byte=$(awk 'BEGIN { for (i = 0; i < 2046; i++) printf "x" }')'[... 3 more bytes]'
  four_byte_value=$(awk 'BEGIN {
    for (i = 0; i < 2045; i++) printf "a"
    printf "\360\237\230\200"
    for (i = 0; i < 100; i++) printf "a"
  }')
  bounded_four_byte=$(awk 'BEGIN { for (i = 0; i < 2045; i++) printf "a" }')'[... 104 more bytes]'
  (
    LC_ALL=C
    [[ ${#exact_three_byte} -eq 2048 && ${#over_three_byte} -eq 2049 ]] || fixture_check_failed \
      "three-byte boundary sizes observed=${#exact_three_byte},${#over_three_byte} expected=2048,2049"
  )
  METASYSTEM_BED_FAILURES=$bounds_records
  {
    fixture_fail 'both comparison sides are bounded' "$long_expected" "$long_observed"
    fixture_fail 'exact byte boundary keeps whole character' "$exact_three_byte" "$exact_three_byte"
    fixture_fail 'split character moves before byte boundary' "$over_three_byte" "$over_three_byte"
    fixture_fail 'four-byte character moves before byte boundary' "$four_byte_value" "$four_byte_value"
  } 2>"$bounds_stderr"
  bounds_seen=0
  while IFS= read -r bounds_record; do
    bounds_what=$("$ms" json get --value "$bounds_record" --field what)
    bounds_expected=$("$ms" json get --value "$bounds_record" --field expected)
    bounds_observed=$("$ms" json get --value "$bounds_record" --field observed)
    grep -Fqx "  expected: $bounds_expected" "$bounds_stderr"
    grep -Fqx "  observed: $bounds_observed" "$bounds_stderr"
    case "$bounds_what" in
      'both comparison sides are bounded')
        [[ "$bounds_expected" == "$bounded_expected" && "$bounds_observed" == "$bounded_observed" ]] || fixture_check_failed \
          "two-byte bounds differed from expected bounded values"
        ;;
      'exact byte boundary keeps whole character')
        [[ "$bounds_expected" == "$exact_three_byte" && "$bounds_observed" == "$exact_three_byte" ]] || fixture_check_failed \
          "exact 2048-byte three-byte value was changed"
        ;;
      'split character moves before byte boundary')
        [[ "$bounds_expected" == "$bounded_over_three_byte" && "$bounds_observed" == "$bounded_over_three_byte" ]] || fixture_check_failed \
          "2049-byte three-byte value observed an incorrect boundary"
        ;;
      'four-byte character moves before byte boundary')
        [[ "$bounds_expected" == "$bounded_four_byte" && "$bounds_observed" == "$bounded_four_byte" ]] || fixture_check_failed \
          "four-byte value observed an incorrect boundary"
        ;;
    esac
    bounds_seen=$((bounds_seen + 1))
  done <"$bounds_records"
  [[ $bounds_seen -eq 4 ]] || fixture_check_failed "bounded assertion records observed=$bounds_seen expected=4"

  continuation_records=$tmp/continuation-bytes.jsonl
  continuation_stderr=$tmp/continuation-bytes.stderr
  continuation_reached=$tmp/continuation-bytes.reached
  continuation_value=$(awk 'BEGIN { for (i = 0; i < 3000; i++) printf "\200" }')
  METASYSTEM_BED_FAILURES=$continuation_records
  {
    fixture_fail 'continuation bytes do not stall the bound' "$continuation_value" observed
    printf 'reached\n' >"$continuation_reached"
  } 2>"$continuation_stderr"
  [[ -s "$continuation_reached" ]] || fixture_check_failed \
    "continuation-byte bound did not return: reached-marker=$continuation_reached"
  continuation_fail_count=$(LC_ALL=C grep -a -c '^FAIL fixture-bed-scenarios:assert-bounds-sides ' "$continuation_stderr")
  [[ $continuation_fail_count -eq 1 ]] || fixture_check_failed \
    "continuation-byte failure blocks observed=$continuation_fail_count expected=1"
  continuation_expected_side=$tmp/continuation-bytes.expected-side
  continuation_actual_side=$tmp/continuation-bytes.actual-side
  printf '  expected: ' >"$continuation_expected_side"
  awk 'BEGIN { for (i = 0; i < 2048; i++) printf "\200" }' >>"$continuation_expected_side"
  printf '%s\n' '[... 952 more bytes]' >>"$continuation_expected_side"
  LC_ALL=C sed -n '2p' "$continuation_stderr" >"$continuation_actual_side"
  cmp "$continuation_expected_side" "$continuation_actual_side"
  exit 0
fi

if [[ "$fixture_scenario" == assert-records-jsonl ]]; then
  jsonl_records=$tmp/assertions.jsonl
  jsonl_stderr=$tmp/assertions.stderr
  harness_fixture_bed_leg jsonl-record-leg
  if (
    METASYSTEM_BED_FAILURES=$jsonl_records
    fixture_fail 'JSON line context' expected observed
  ) 2>"$jsonl_stderr"; then
    :
  else
    echo "fixture assertion did not return success while writing JSON Lines" >&2
    exit 1
  fi
  jsonl_count=0
  while IFS= read -r jsonl_record; do
    jsonl_kind=$("$ms" json get --value "$jsonl_record" --field kind)
    [[ "$jsonl_kind" == assertion ]] || fixture_check_failed "JSON Lines kind observed=$jsonl_kind expected=assertion"
    jsonl_bed=$("$ms" json get --value "$jsonl_record" --field bed)
    [[ "$jsonl_bed" == fixture-bed-scenarios ]] || fixture_check_failed \
      "JSON Lines bed observed=$jsonl_bed expected=fixture-bed-scenarios"
    jsonl_scenario=$("$ms" json get --value "$jsonl_record" --field scenario)
    [[ "$jsonl_scenario" == assert-records-jsonl ]] || fixture_check_failed \
      "JSON Lines scenario observed=$jsonl_scenario expected=assert-records-jsonl"
    jsonl_leg=$("$ms" json get --value "$jsonl_record" --field leg)
    [[ "$jsonl_leg" == jsonl-record-leg ]] || fixture_check_failed \
      "JSON Lines leg observed=$jsonl_leg expected=jsonl-record-leg"
    jsonl_count=$((jsonl_count + 1))
  done <"$jsonl_records"
  [[ $jsonl_count -eq 1 ]] || fixture_check_failed "JSON Lines assertion records observed=$jsonl_count expected=1"

  numeric_records=$tmp/numeric-terminal.jsonl
  if (
    METASYSTEM_BED_FAILURES=$numeric_records
    fixture_assert_finish
  ) 2>"$tmp/numeric-terminal.stderr"; then
    :
  else
    echo "clean fixture assertion finish did not return success" >&2
    exit 1
  fi
  numeric_terminal=$(tail -n 1 "$numeric_records")
  numeric_failures=$("$ms" json get --value "$numeric_terminal" --field failures)
  [[ "$numeric_failures" == 0 ]] || fixture_check_failed "numeric terminal failures observed=$numeric_failures expected=0"
  grep -Fq '"failures":0' "$numeric_records"
  exit 0
fi

if [[ "$fixture_scenario" == assert-path-relative ]]; then
  if [[ ${FIXTURE_ASSERT_PATH_CHILD:-} == 1 ]]; then
    path_records=${METASYSTEM_BED_FAILURES:?}
    path_stderr=${FIXTURE_ASSERT_PATH_STDERR:?}
    path_line=$((LINENO + 1))
    fixture_fail 'repository relative call site' expected observed 2>"$path_stderr"
    recorded_path=$("$ms" json get --file "$path_records" --field file)
    [[ "$recorded_path" == "$fixture_assert_source_path" ]] || fixture_check_failed \
      "symlinked call site observed=$recorded_path expected=$fixture_assert_source_path"
    grep -Fqx "FAIL fixture-bed-scenarios:assert-path-relative $fixture_assert_source_path:$path_line repository relative call site" "$path_stderr"
    if grep -Fq "$fixture_assert_repository_root/" "$path_records"; then
      echo "fixture assertion record retained an absolute repository path" >&2
      exit 1
    fi
    exit 0
  fi
  path_records=$tmp/path.jsonl
  path_stderr=$tmp/path.stderr
  repository_link=$tmp/repository-link
  ln -s "$fixture_assert_repository_root" "$repository_link"
  if GIT_DIR=$tmp/poisoned-git-directory \
    FIXTURE_ASSERT_PATH_CHILD=1 FIXTURE_ASSERT_PATH_STDERR=$path_stderr \
    METASYSTEM_BED_FAILURES=$path_records \
    "$repository_link/metasystem/scripts/agents/fixture-bed-scenarios-fixtures.sh" "$@"; then
    :
  else
    echo "fixture assertion failed through a symlinked repository path with GIT_DIR exported" >&2
    exit 1
  fi
  exit 0
fi

if [[ "$fixture_scenario" == assert-bash32 ]]; then
  bash32_records=$tmp/bash32.jsonl
  bash32_stderr=$tmp/bash32.stderr
  bash32_reached=$tmp/bash32.reached
  /bin/bash -n "$root/scripts/agents/fixture-assert.sh"
  if METASYSTEM_BIN="$ms" METASYSTEM_BED_FAILURES="$bash32_records" BASH32_REACHED="$bash32_reached" \
    /bin/bash -c 'set -euo pipefail
      source "$1"
      assert_equal "Bash 3.2 duplicate label" same same
      assert_equal "Bash 3.2 duplicate label" expected observed
      long_value=$(i=0; while (( i < 1536 )); do printf "\303\251"; i=$((i + 1)); done)
      fixture_fail "Bash 3.2 byte bound" "$long_value" short
      printf "reached\n" >"$BASH32_REACHED"
      fixture_assert_finish' \
      fixture-assert-bash32 "$root/scripts/agents/fixture-assert.sh" 2>"$bash32_stderr"; then
    bash32_rc=0
  else
    bash32_rc=$?
  fi
  [[ $bash32_rc -eq 1 ]] || fixture_check_failed "Bash 3.2 finish exit observed=$bash32_rc expected=1"
  [[ -s "$bash32_reached" ]] || fixture_check_failed "Bash 3.2 helper did not reach marker=$bash32_reached"
  bash32_duplicate_count=$(grep -c '"kind":"duplicate-label"' "$bash32_records")
  [[ $bash32_duplicate_count -eq 1 ]] || fixture_check_failed \
    "Bash 3.2 duplicate records observed=$bash32_duplicate_count expected=1"
  bash32_assertion_count=$(grep -c '"kind":"assertion"' "$bash32_records")
  [[ $bash32_assertion_count -eq 1 ]] || fixture_check_failed \
    "Bash 3.2 assertion records observed=$bash32_assertion_count expected=1"
  bash32_terminal=$(tail -n 1 "$bash32_records")
  bash32_failures=$("$ms" json get --value "$bash32_terminal" --field failures)
  [[ "$bash32_failures" == 2 ]] || fixture_check_failed "Bash 3.2 failures observed=$bash32_failures expected=2"
  grep -Fq '"failures":2' "$bash32_records"
  grep -Fq '[... 1024 more bytes]' "$bash32_records"

  bash32_require_records=$tmp/bash32-require.jsonl
  if METASYSTEM_BIN="$ms" METASYSTEM_BED_FAILURES="$bash32_require_records" \
    /bin/bash -c 'set -euo pipefail; source "$1"; require_equal "Bash 3.2 required value" ready blocked; exit 90' \
      fixture-assert-bash32-require "$root/scripts/agents/fixture-assert.sh" 2>>"$bash32_stderr"; then
    bash32_require_rc=0
  else
    bash32_require_rc=$?
  fi
  [[ $bash32_require_rc -eq 1 ]] || fixture_check_failed \
    "Bash 3.2 require exit observed=$bash32_require_rc expected=1"
  bash32_require_terminal=$(tail -n 1 "$bash32_require_records")
  bash32_require_by=$("$ms" json get --value "$bash32_require_terminal" --field by)
  [[ "$bash32_require_by" == require ]] || fixture_check_failed \
    "Bash 3.2 require terminal by observed=$bash32_require_by expected=require"
  bash32_require_failures=$("$ms" json get --value "$bash32_require_terminal" --field failures)
  [[ "$bash32_require_failures" == 1 ]] || fixture_check_failed \
    "Bash 3.2 require failures observed=$bash32_require_failures expected=1"
  exit 0
fi

if [[ "$fixture_scenario" == assert-counts-across-subshells ]]; then
  subshell_records=$tmp/subshell-records.jsonl
  subshell_stderr=$tmp/subshell.stderr
  if (
    METASYSTEM_BED_FAILURES=$subshell_records
    ( assert_equal 'subshell failure' expected subshell )
    printf '%s\n' pipeline-one pipeline-two |
      while IFS= read -r pipeline_value; do
        assert_equal "pipeline failure $pipeline_value" expected "$pipeline_value"
      done
    substitution_value=$(assert_equal 'substitution failure' expected substitution)
    [[ -z "$substitution_value" ]] || fixture_check_failed \
      "substitution assertion output observed=$substitution_value expected empty"
    fixture_assert_finish
  ) 2>"$subshell_stderr"; then
    subshell_rc=0
  else
    subshell_rc=$?
  fi
  [[ $subshell_rc -eq 1 ]] || fixture_check_failed "subshell finisher exit observed=$subshell_rc expected=1"
  subshell_block_count=$(grep -c '^FAIL fixture-bed-scenarios:assert-counts-across-subshells ' "$subshell_stderr")
  [[ $subshell_block_count -eq 4 ]] || fixture_check_failed \
    "subshell failure blocks observed=$subshell_block_count expected=4"
  subshell_failure_count=0
  while IFS= read -r subshell_record; do
    subshell_kind=$("$ms" json get --value "$subshell_record" --field kind)
    case "$subshell_kind" in
      assertion|duplicate-label) subshell_failure_count=$((subshell_failure_count + 1)) ;;
    esac
  done <"$subshell_records"
  [[ $subshell_failure_count -eq 4 ]] || fixture_check_failed \
    "subshell failure records observed=$subshell_failure_count expected=4"
  subshell_terminal=$(tail -n 1 "$subshell_records")
  subshell_terminal_count=$("$ms" json get --value "$subshell_terminal" --field failures)
  [[ "$subshell_terminal_count" == "$subshell_failure_count" ]] || fixture_check_failed \
    "subshell terminal failures observed=$subshell_terminal_count expected=$subshell_failure_count"
  exit 0
fi

if [[ "$fixture_scenario" == assert-json-field-read-errors ]]; then
  json_error_records=$tmp/json-errors.jsonl
  json_error_stderr=$tmp/json-errors.stderr
  malformed_json=$tmp/malformed.json
  absent_json=$tmp/absent.json
  empty_json=$tmp/empty.json
  printf '%s\n' '{malformed' >"$malformed_json"
  printf '%s\n' '{}' >"$absent_json"
  printf '%s\n' '{"field":""}' >"$empty_json"
  METASYSTEM_BED_FAILURES=$json_error_records
  {
    assert_json_field 'missing JSON file' "$tmp/missing.json" field ''
    assert_json_field 'malformed JSON file' "$malformed_json" field ''
    assert_json_field 'absent JSON field' "$absent_json" field ''
    assert_json_field 'present empty JSON field' "$empty_json" field ''
  } 2>"$json_error_stderr"
  json_error_block_count=$(grep -c '^FAIL fixture-bed-scenarios:assert-json-field-read-errors ' "$json_error_stderr")
  [[ $json_error_block_count -eq 3 ]] || fixture_check_failed \
    "JSON read-error blocks observed=$json_error_block_count expected=3"
  json_error_record_count=$(grep -c '"kind":"assertion"' "$json_error_records")
  [[ $json_error_record_count -eq 3 ]] || fixture_check_failed \
    "JSON read-error records observed=$json_error_record_count expected=3"
  first_json_error_record=$(sed -n '1p' "$json_error_records")
  missing_observed=$("$ms" json get --value "$first_json_error_record" --field observed)
  [[ "$missing_observed" == '<json get exit 1: file missing or unparseable'* ]] || fixture_check_failed \
    "missing JSON observed=$missing_observed expected json get exit 1 cause"
  json_error_seen_malformed=0
  json_error_seen_absent=0
  while IFS= read -r json_error_record; do
    json_error_what=$("$ms" json get --value "$json_error_record" --field what)
    json_error_observed=$("$ms" json get --value "$json_error_record" --field observed)
    case "$json_error_what" in
      'malformed JSON file')
        [[ "$json_error_observed" == '<json get exit 1: file missing or unparseable'* ]] || fixture_check_failed \
          "malformed JSON observed=$json_error_observed expected json get exit 1 cause"
        json_error_seen_malformed=1
        ;;
      'absent JSON field')
        [[ "$json_error_observed" == '<json get exit 3: field absent'* ]] || fixture_check_failed \
          "absent JSON field observed=$json_error_observed expected json get exit 3 cause"
        json_error_seen_absent=1
        ;;
    esac
  done <"$json_error_records"
  [[ $json_error_seen_malformed -eq 1 && $json_error_seen_absent -eq 1 ]] || fixture_check_failed \
    "JSON error causes seen malformed=$json_error_seen_malformed absent=$json_error_seen_absent expected both 1"
  exit 0
fi

if [[ "$fixture_scenario" == assert-survives-broken-engine-and-unwritable-records ]]; then
  unwritten_records=$tmp/unwritten-record.jsonl
  unwritten_stderr=$tmp/unwritten-record.stderr
  unwritten_reached=$tmp/unwritten-record.reached
  if (
    METASYSTEM_BED_FAILURES=$unwritten_records
    mkdir "$unwritten_records"
    fixture_fail 'unwritable assertion record' expected observed
    printf 'reached\n' >"$unwritten_reached"
    rmdir "$unwritten_records"
    fixture_assert_finish
  ) 2>"$unwritten_stderr"; then
    unwritten_rc=0
  else
    unwritten_rc=$?
  fi
  [[ $unwritten_rc -eq 1 ]] || fixture_check_failed \
    "unwritten-record-only finisher exit observed=$unwritten_rc expected=1"
  [[ -s "$unwritten_reached" ]] || fixture_check_failed \
    "unwritable assertion did not continue: reached-marker=$unwritten_reached"
  unwritten_block_count=$(grep -c '^FAIL fixture-bed-scenarios:assert-survives-broken-engine-and-unwritable-records ' "$unwritten_stderr")
  [[ $unwritten_block_count -eq 1 ]] || fixture_check_failed \
    "unwritable assertion blocks observed=$unwritten_block_count expected=1"
  unwritten_marker_count=$(grep -c '^unwritten-record$' "$unwritten_records.labels.sites")
  [[ $unwritten_marker_count -ge 1 ]] || fixture_check_failed \
    "unwritten sidecar markers observed=$unwritten_marker_count expected at least 1"
  unwritten_line_count=$(wc -l <"$unwritten_records")
  [[ $unwritten_line_count -eq 1 ]] || fixture_check_failed \
    "records after restored append observed=$unwritten_line_count expected terminal only"
  unwritten_terminal=$(tail -n 1 "$unwritten_records")
  unwritten_kind=$("$ms" json get --value "$unwritten_terminal" --field kind)
  [[ "$unwritten_kind" == terminal ]] || fixture_check_failed \
    "restored terminal kind observed=$unwritten_kind expected=terminal"
  unwritten_failures=$("$ms" json get --value "$unwritten_terminal" --field failures)
  [[ "$unwritten_failures" == 0 ]] || fixture_check_failed \
    "restored terminal record count observed=$unwritten_failures expected=0"

  broken_engine=$tmp/broken-helper-engine
  broken_engine_marker=$tmp/broken-helper-engine.called
  broken_records=$tmp/broken-helper-engine.jsonl
  broken_stderr=$tmp/broken-helper-engine.stderr
  broken_reached=$tmp/broken-helper-engine.reached
  cat >"$broken_engine" <<'BROKEN_ENGINE'
#!/usr/bin/env bash
printf 'called\n' >>"${BROKEN_ENGINE_MARKER:?}"
exit 3
BROKEN_ENGINE
  chmod +x "$broken_engine"
  if METASYSTEM_BIN=$broken_engine BROKEN_ENGINE_MARKER=$broken_engine_marker \
    METASYSTEM_BED_FAILURES=$broken_records BROKEN_REACHED=$broken_reached \
    /bin/bash -c 'set -euo pipefail
      source "$1"
      fixture_fail "helper engine failure" expected observed
      printf "reached\n" >"$BROKEN_REACHED"
      fixture_assert_finish' \
      fixture-assert-broken-engine "$root/scripts/agents/fixture-assert.sh" 2>"$broken_stderr"; then
    broken_rc=0
  else
    broken_rc=$?
  fi
  [[ $broken_rc -eq 1 ]] || fixture_check_failed "broken helper engine finish exit observed=$broken_rc expected=1"
  [[ -s "$broken_reached" ]] || fixture_check_failed \
    "broken helper engine assertion did not continue: reached-marker=$broken_reached"
  [[ -s "$broken_engine_marker" ]] || fixture_check_failed \
    "helper engine stub was not started: marker=$broken_engine_marker"
  broken_block_count=$(grep -c '^FAIL fixture-bed-scenarios:assert-survives-broken-engine-and-unwritable-records ' "$broken_stderr")
  [[ $broken_block_count -eq 1 ]] || fixture_check_failed \
    "broken helper engine blocks observed=$broken_block_count expected=1"
  broken_unwritten_count=$(grep -c '^fixture assertions: record was not written: ' "$broken_stderr")
  [[ $broken_unwritten_count -ge 1 ]] || fixture_check_failed \
    "broken helper engine unwritten diagnostics observed=$broken_unwritten_count expected at least 1"

  closed_stderr_records=$tmp/closed-stderr.jsonl
  closed_stderr_reached=$tmp/closed-stderr.reached
  if METASYSTEM_BIN=$ms METASYSTEM_BED_FAILURES=$closed_stderr_records \
    CLOSED_STDERR_REACHED=$closed_stderr_reached \
    /bin/bash -c 'set -euo pipefail
      source "$1"
      fixture_fail "closed stderr still records" expected observed
      printf "reached\n" >"$CLOSED_STDERR_REACHED"' \
      fixture-assert-closed-stderr "$root/scripts/agents/fixture-assert.sh" 2>&-; then
    closed_stderr_rc=0
  else
    closed_stderr_rc=$?
  fi
  [[ $closed_stderr_rc -eq 0 ]] || fixture_check_failed \
    "closed-stderr assertion exit observed=$closed_stderr_rc expected=0"
  [[ -s "$closed_stderr_reached" ]] || fixture_check_failed \
    "closed-stderr assertion did not continue: reached-marker=$closed_stderr_reached"
  closed_stderr_count=$(grep -c '"kind":"assertion"' "$closed_stderr_records")
  [[ $closed_stderr_count -eq 1 ]] || fixture_check_failed \
    "closed-stderr assertion records observed=$closed_stderr_count expected=1"

  empty_engine_records=$tmp/empty-engine.jsonl
  empty_engine_reached=$tmp/empty-engine.reached
  if METASYSTEM_BIN= METASYSTEM_BED_FAILURES=$empty_engine_records EMPTY_ENGINE_REACHED=$empty_engine_reached \
    /bin/bash -c 'set -euo pipefail
      source "$1"
      fixture_fail "empty engine uses repository engine" expected observed
      printf "reached\n" >"$EMPTY_ENGINE_REACHED"' \
      fixture-assert-empty-engine "$root/scripts/agents/fixture-assert.sh" 2>"$tmp/empty-engine.stderr"; then
    empty_engine_rc=0
  else
    empty_engine_rc=$?
  fi
  [[ $empty_engine_rc -eq 0 ]] || fixture_check_failed "empty engine assertion exit observed=$empty_engine_rc expected=0"
  [[ -s "$empty_engine_reached" ]] || fixture_check_failed \
    "empty engine assertion did not continue: reached-marker=$empty_engine_reached"
  empty_engine_count=$(grep -c '"kind":"assertion"' "$empty_engine_records")
  [[ $empty_engine_count -eq 1 ]] || fixture_check_failed \
    "empty engine assertion records observed=$empty_engine_count expected=1"
  exit 0
fi

if [[ "$fixture_scenario" == multi-line-value-stays-one-block ]]; then
  multiline_records=$tmp/multiline.jsonl
  multiline_stderr=$tmp/multiline.stderr
  multiline_expected=$'wanted first\nwanted second'
  multiline_observed=$'outer line\nFAIL x:y other-file:1 inner\nafter inner'
  METASYSTEM_BED_FAILURES=$multiline_records
  fixture_fail 'multi-line comparison' "$multiline_expected" "$multiline_observed" 2>"$multiline_stderr"
  multiline_block_count=$(grep -c '^FAIL ' "$multiline_stderr")
  [[ $multiline_block_count -eq 1 ]] || fixture_check_failed \
    "multi-line value column-zero blocks observed=$multiline_block_count expected=1"
  grep -Fqx '            wanted second' "$multiline_stderr"
  grep -Fqx '            FAIL x:y other-file:1 inner' "$multiline_stderr"
  grep -Fqx '            after inner' "$multiline_stderr"
  multiline_record_count=$(grep -c '"kind":"assertion"' "$multiline_records")
  [[ $multiline_record_count -eq 1 ]] || fixture_check_failed \
    "multi-line value assertion records observed=$multiline_record_count expected=1"
  recorded_multiline_observed=$("$ms" json get --file "$multiline_records" --field observed)
  [[ "$recorded_multiline_observed" == "$multiline_observed" ]] || fixture_check_failed \
    "multi-line observed value changed in the JSON record"

  newline_label_records=$tmp/newline-label.jsonl
  newline_label_stderr=$tmp/newline-label.stderr
  newline_label=$'outer label\nFAIL x:y other-file:1 inner label'
  METASYSTEM_BED_FAILURES=$newline_label_records
  fixture_fail "$newline_label" expected observed 2>"$newline_label_stderr"
  newline_label_block_count=$(grep -c '^FAIL ' "$newline_label_stderr")
  [[ $newline_label_block_count -eq 1 ]] || fixture_check_failed \
    "newline label column-zero blocks observed=$newline_label_block_count expected=1"
  grep -Fqx '            FAIL x:y other-file:1 inner label' "$newline_label_stderr"
  recorded_newline_label=$("$ms" json get --file "$newline_label_records" --field what)
  [[ "$recorded_newline_label" == "$newline_label" ]] || fixture_check_failed \
    "newline label changed in the JSON record"
  exit 0
fi

if [[ "$fixture_scenario" == contains-read-error-never-passes ]]; then
  contains_error_records=$tmp/contains-errors.jsonl
  contains_error_stderr=$tmp/contains-errors.stderr
  contains_invalid_file=$tmp/contains-invalid-pattern.txt
  printf '%s\n' 'file subject' >"$contains_invalid_file"
  METASYSTEM_BED_FAILURES=$contains_error_records
  {
    TMPDIR=$tmp/missing-subject-directory \
      assert_not_contains 'not-contains rejects subject write error' forbidden 'forbidden text'
    TMPDIR=$tmp/missing-subject-directory \
      assert_contains 'contains rejects subject write error' present 'present text'
    assert_contains -E 'invalid regex on string subject' '(' 'string subject'
    assert_contains -E 'invalid regex on file subject' '(' "$contains_invalid_file"
    assert_not_contains -E 'invalid regex on negated string subject' '(' 'negated string subject'
    assert_not_contains -E 'invalid regex on negated file subject' '(' "$contains_invalid_file"
  } 2>"$contains_error_stderr"
  contains_error_block_count=$(grep -c '^FAIL fixture-bed-scenarios:contains-read-error-never-passes ' "$contains_error_stderr")
  [[ $contains_error_block_count -eq 6 ]] || fixture_check_failed \
    "contains error blocks observed=$contains_error_block_count expected=6"
  contains_error_record_count=$(grep -c '"kind":"assertion"' "$contains_error_records")
  [[ $contains_error_record_count -eq 6 ]] || fixture_check_failed \
    "contains error records observed=$contains_error_record_count expected=6"
  contains_write_error_seen=0
  contains_grep_error_seen=0
  while IFS= read -r contains_error_record; do
    contains_error_what=$("$ms" json get --value "$contains_error_record" --field what)
    contains_error_observed=$("$ms" json get --value "$contains_error_record" --field observed)
    case "$contains_error_what" in
      'not-contains rejects subject write error'|'contains rejects subject write error')
        [[ "$contains_error_observed" == '<could not create string subject file '* ]] || fixture_check_failed \
          "contains subject creation observed=$contains_error_observed expected creation error"
        contains_write_error_seen=$((contains_write_error_seen + 1))
        ;;
      'invalid regex on string subject'|'invalid regex on file subject'|\
      'invalid regex on negated string subject'|'invalid regex on negated file subject')
        [[ "$contains_error_observed" == '<grep exit '* ]] || fixture_check_failed \
          "invalid regular expression observed=$contains_error_observed expected grep exit"
        contains_grep_error_seen=$((contains_grep_error_seen + 1))
        ;;
      *) fixture_check_failed "unexpected contains error record label observed=$contains_error_what" ;;
    esac
  done <"$contains_error_records"
  [[ $contains_write_error_seen -eq 2 && $contains_grep_error_seen -eq 4 ]] || fixture_check_failed \
    "contains error kinds observed writes=$contains_write_error_seen grep=$contains_grep_error_seen expected 2 and 4"
  if ( fixture_assert_finish ) 2>>"$contains_error_stderr"; then
    contains_error_rc=0
  else
    contains_error_rc=$?
  fi
  [[ $contains_error_rc -eq 1 ]] || fixture_check_failed "contains-error finisher exit observed=$contains_error_rc expected=1"
  contains_require_records=$tmp/contains-require-error.jsonl
  if (
    METASYSTEM_BED_FAILURES=$contains_require_records
    TMPDIR=$tmp/missing-required-subject-directory \
      require_contains 'required contains rejects subject write error' present 'present text'
    exit 90
  ) 2>>"$contains_error_stderr"; then
    contains_require_rc=0
  else
    contains_require_rc=$?
  fi
  [[ $contains_require_rc -eq 1 ]] || fixture_check_failed \
    "required contains error exit observed=$contains_require_rc expected=1"
  contains_require_terminal=$(tail -n 1 "$contains_require_records")
  contains_require_by=$("$ms" json get --value "$contains_require_terminal" --field by)
  [[ "$contains_require_by" == require ]] || fixture_check_failed \
    "required contains terminal by observed=$contains_require_by expected=require"
  contains_require_failures=$("$ms" json get --value "$contains_require_terminal" --field failures)
  [[ "$contains_require_failures" == 1 ]] || fixture_check_failed \
    "required contains terminal failures observed=$contains_require_failures expected=1"
  exit 0
fi

if [[ "$fixture_scenario" == passing-calls-start-no-engine ]]; then
  counting_engine=$tmp/counting-engine
  counting_engine_starts=$tmp/counting-engine.starts
  cat >"$counting_engine" <<'COUNTING_ENGINE'
#!/usr/bin/env bash
printf 'started\n' >>"${COUNTING_ENGINE_STARTS:?}"
exit 3
COUNTING_ENGINE
  chmod +x "$counting_engine"
  METASYSTEM_BIN=$counting_engine COUNTING_ENGINE_STARTS=$counting_engine_starts \
    METASYSTEM_BED_FAILURES=$tmp/passing-50.jsonl \
    /bin/bash -c 'set -euo pipefail
      source "$1"
      passing_index=0
      while (( passing_index < 50 )); do
        assert_equal "passing call $passing_index" same same
        passing_index=$((passing_index + 1))
      done
      [[ ! -e "$COUNTING_ENGINE_STARTS" ]] || {
        printf "passing calls started the helper engine: observed marker=%s\n" "$COUNTING_ENGINE_STARTS" >&2
        exit 1
      }' \
      fixture-assert-passing-50 "$root/scripts/agents/fixture-assert.sh"
  METASYSTEM_BIN=$counting_engine COUNTING_ENGINE_STARTS=$counting_engine_starts \
    METASYSTEM_BED_FAILURES=$tmp/passing-150.jsonl \
    /bin/bash -c 'set -euo pipefail
      source "$1"
      passing_started=$SECONDS
      passing_index=0
      while (( passing_index < 150 )); do
        assert_equal "timed passing call $passing_index" same same
        passing_index=$((passing_index + 1))
      done
      [[ ! -e "$COUNTING_ENGINE_STARTS" ]] || {
        printf "timed passing calls started the helper engine: observed marker=%s\n" "$COUNTING_ENGINE_STARTS" >&2
        exit 1
      }
      printf "passing-calls-start-no-engine elapsed seconds: %s\n" "$((SECONDS - passing_started))"' \
      fixture-assert-passing-150 "$root/scripts/agents/fixture-assert.sh"
  exit 0
fi

if [[ "$fixture_scenario" == fallback-records-clean-and-announce ]]; then
  fallback_tmp=$tmp/fallback-tmp
  fallback_path_file=$tmp/fallback.path
  fallback_stderr=$tmp/fallback.stderr
  mkdir "$fallback_tmp"
  if TMPDIR=$fallback_tmp METASYSTEM_BIN=$ms FALLBACK_PATH_FILE=$fallback_path_file \
    /bin/bash -c 'set -euo pipefail
      unset METASYSTEM_BED_FAILURES
      fallback_records=$TMPDIR/metasystem-bed-failures.$$.jsonl
      printf "%s\n" "$fallback_records" >"$FALLBACK_PATH_FILE"
      printf "%s\n" "{\"kind\":\"assertion\"}" >"$fallback_records"
      printf "%s\n" "default fallback one" >"$fallback_records.labels"
      printf "call\tdefault fallback one\tstale-file\t1\n" >"$fallback_records.labels.sites"
      mkdir "$fallback_records.path-announced"
      source "$1"
      ( fixture_fail "default fallback one" expected observed )
      fixture_fail "default fallback two" expected observed
      fixture_assert_finish' \
      fixture-assert-fallback "$root/scripts/agents/fixture-assert.sh" 2>"$fallback_stderr"; then
    fallback_rc=0
  else
    fallback_rc=$?
  fi
  [[ $fallback_rc -eq 1 ]] || fixture_check_failed "fallback finisher exit observed=$fallback_rc expected=1"
  fallback_records=$(sed -n '1p' "$fallback_path_file")
  fallback_announcement_count=$(grep -c '^fixture assertion records: ' "$fallback_stderr")
  [[ $fallback_announcement_count -eq 1 ]] || fixture_check_failed \
    "fallback announcements observed=$fallback_announcement_count expected=1"
  fallback_line_count=$(wc -l <"$fallback_records")
  [[ $fallback_line_count -eq 3 ]] || fixture_check_failed \
    "fallback record lines observed=$fallback_line_count expected=3"
  fallback_first=$(sed -n '1p' "$fallback_records")
  fallback_terminal=$(tail -n 1 "$fallback_records")
  fallback_first_kind=$("$ms" json get --value "$fallback_first" --field kind)
  [[ "$fallback_first_kind" == assertion ]] || fixture_check_failed \
    "fallback first record kind observed=$fallback_first_kind expected=assertion"
  fallback_failures=$("$ms" json get --value "$fallback_terminal" --field failures)
  [[ "$fallback_failures" == 2 ]] || fixture_check_failed \
    "fallback terminal failures observed=$fallback_failures expected=2"
  exit 0
fi

if [[ "$fixture_scenario" == require-exit-stops-with-terminal-record ]]; then
  require_exit_records=$tmp/require-exit.jsonl
  require_exit_stderr=$tmp/require-exit.stderr
  require_exit_continued=$tmp/require-exit.continued
  if (
    METASYSTEM_BED_FAILURES=$require_exit_records
    require_exit 'required exit status' 0 7
    printf 'continued\n' >"$require_exit_continued"
  ) 2>"$require_exit_stderr"; then
    require_exit_rc=0
  else
    require_exit_rc=$?
  fi
  [[ $require_exit_rc -eq 1 && ! -e "$require_exit_continued" ]] || fixture_check_failed \
    "require_exit observed exit=$require_exit_rc continuation=$require_exit_continued expected exit 1 and absent marker"
  require_exit_lines=$(wc -l <"$require_exit_records")
  [[ $require_exit_lines -eq 2 ]] || fixture_check_failed \
    "require_exit record lines observed=$require_exit_lines expected=2"
  require_exit_terminal=$(tail -n 1 "$require_exit_records")
  require_exit_by=$("$ms" json get --value "$require_exit_terminal" --field by)
  [[ "$require_exit_by" == require ]] || fixture_check_failed \
    "require_exit terminal by observed=$require_exit_by expected=require"
  require_exit_failures=$("$ms" json get --value "$require_exit_terminal" --field failures)
  [[ "$require_exit_failures" == 1 ]] || fixture_check_failed \
    "require_exit terminal failures observed=$require_exit_failures expected=1"
  exit 0
fi

if [[ "$fixture_scenario" == require-json-field-stops-with-terminal-record ]]; then
  require_json_input=$tmp/require-json-input.json
  require_json_records=$tmp/require-json.jsonl
  require_json_stderr=$tmp/require-json.stderr
  require_json_continued=$tmp/require-json.continued
  printf '%s\n' '{"state":"blocked"}' >"$require_json_input"
  if (
    METASYSTEM_BED_FAILURES=$require_json_records
    require_json_field 'required JSON state' "$require_json_input" state ready
    printf 'continued\n' >"$require_json_continued"
  ) 2>"$require_json_stderr"; then
    require_json_rc=0
  else
    require_json_rc=$?
  fi
  [[ $require_json_rc -eq 1 && ! -e "$require_json_continued" ]] || fixture_check_failed \
    "require_json_field observed exit=$require_json_rc continuation=$require_json_continued expected exit 1 and absent marker"
  require_json_lines=$(wc -l <"$require_json_records")
  [[ $require_json_lines -eq 2 ]] || fixture_check_failed \
    "require_json_field record lines observed=$require_json_lines expected=2"
  require_json_terminal=$(tail -n 1 "$require_json_records")
  require_json_by=$("$ms" json get --value "$require_json_terminal" --field by)
  [[ "$require_json_by" == require ]] || fixture_check_failed \
    "require_json_field terminal by observed=$require_json_by expected=require"
  require_json_failures=$("$ms" json get --value "$require_json_terminal" --field failures)
  [[ "$require_json_failures" == 1 ]] || fixture_check_failed \
    "require_json_field terminal failures observed=$require_json_failures expected=1"
  exit 0
fi

if [[ "$fixture_scenario" == fixture-has-no-inert-checks ]]; then
  inert_checks=$tmp/inert-checks.txt
  fixture_file=$fixture_assert_repository_root/$fixture_assert_source_path
  if grep -n -E '^[[:space:]]*(\[\[.*\]\]|\(\(.*\)\))[[:space:]]*$' "$fixture_file" >"$inert_checks"; then
    echo "fixture-bed-scenarios found Bash 3.2-inert checks:" >&2
    command cat "$inert_checks" >&2
    exit 1
  else
    inert_rc=$?
  fi
  [[ $inert_rc -eq 1 ]] || fixture_check_failed \
    "inert-check source scan exit observed=$inert_rc expected=1 for no matches"
  exit 0
fi

if [[ "$fixture_scenario" == budget-standalone ]]; then
  inner=$tmp/inner-bed.sh
  output=$tmp/standalone.out
  write_inner_bed "$inner"
  (
    unset METASYSTEM_FIXTURE_CAP_SCALE METASYSTEM_FIXTURE_CAP_SCALE_MILLI
    FIXTURE_BED_SOURCE_ROOT="$root" FIXTURE_BED_INNER_SCENARIOS=print-scale \
      "$inner" "$harness_fixture_tag"
  ) >"$output" 2>&1
  standalone_milli=$(sed -n '/^[0-9][0-9]*$/{p;q;}' "$output")
  [[ "$standalone_milli" =~ ^[0-9]+$ \
      && "$standalone_milli" -ge 8000 && "$standalone_milli" -le 48000 ]] || {
    echo "fixture-bed-scenarios standalone budget was not self-initialized: ${standalone_milli:-absent}" >&2
    sed -n '1,160p' "$output" >&2
    exit 1
  }
  exit 0
fi

if [[ "$fixture_scenario" == budget-inherited ]]; then
  inner=$tmp/inner-bed.sh
  operator_output=$tmp/operator.out
  parent_output=$tmp/parent.out
  write_inner_bed "$inner"
  (
    export METASYSTEM_FIXTURE_CAP_SCALE=3
    unset METASYSTEM_FIXTURE_CAP_SCALE_MILLI
    FIXTURE_BED_SOURCE_ROOT="$root" FIXTURE_BED_INNER_SCENARIOS=print-scale \
      "$inner" "$harness_fixture_tag"
  ) >"$operator_output" 2>&1
  METASYSTEM_FIXTURE_CAP_SCALE=3 METASYSTEM_FIXTURE_CAP_SCALE_MILLI=3000 \
    FIXTURE_BED_SOURCE_ROOT="$root" FIXTURE_BED_INNER_SCENARIOS=print-scale \
    "$inner" "$harness_fixture_tag" >"$parent_output" 2>&1
  operator_milli=$(sed -n '/^[0-9][0-9]*$/{p;q;}' "$operator_output")
  [[ "$operator_milli" == 3000 ]] || fixture_check_failed \
    "operator budget scale observed=$operator_milli expected=3000"
  parent_milli=$(sed -n '/^[0-9][0-9]*$/{p;q;}' "$parent_output")
  [[ "$parent_milli" == 3000 ]] || fixture_check_failed \
    "parent budget scale observed=$parent_milli expected=3000"
  exit 0
fi

if [[ "$fixture_scenario" == ceiling-reaps-group ]]; then
  inner=$tmp/inner-bed.sh
  output=$tmp/ceiling.out
  child_file=$tmp/child.pid
  grandchild_file=$tmp/grandchild.pid
  scheduler_ready_file=$tmp/scheduler-ready
  write_inner_bed "$inner" true
  set +e
  env -u METASYSTEM_FIXTURE_ONLY \
    METASYSTEM_BED_SCENARIO_FIXTURE_TIMEOUT_SEC=1 \
    FIXTURE_BED_SOURCE_ROOT="$root" FIXTURE_BED_INNER_SCENARIOS=hang \
    FIXTURE_BED_CHILD_PID_FILE="$child_file" \
    FIXTURE_BED_GRANDCHILD_PID_FILE="$grandchild_file" \
    FIXTURE_BED_SCHEDULER_READY_FILE="$scheduler_ready_file" \
    "$inner" "$harness_fixture_tag" >"$output" 2>&1
  inner_rc=$?
  set -e
  [[ $inner_rc -eq 1 ]] || {
    echo "fixture-bed-scenarios ceiling inner bed exited $inner_rc, want 1" >&2
    sed -n '1,200p' "$output" >&2
    exit 1
  }
  [[ -s "$child_file" && -s "$grandchild_file" ]] \
    || { echo "fixture-bed-scenarios ceiling fixture did not observe both ready descendants" >&2; exit 1; }
  child_pid=$(<"$child_file")
  grandchild_pid=$(<"$grandchild_file")
  [[ -s "$scheduler_ready_file" ]] \
    || { echo "fixture-bed-scenarios ceiling fixture did not record the controlled initial scan" >&2; exit 1; }
  read -r scheduler_child scheduler_grandchild scheduler_now scheduler_deadline <"$scheduler_ready_file"
  [[ "$scheduler_child" == "$child_pid" && "$scheduler_grandchild" == "$grandchild_pid" &&
      "$scheduler_now" =~ ^[0-9]+$ && "$scheduler_deadline" =~ ^[0-9]+$ &&
      "$scheduler_now" -ge "$scheduler_deadline" ]] \
    || { echo "fixture-bed-scenarios ceiling fixture did not expire after both descendants were ready" >&2; exit 1; }
  grep -Eq '^fixture-bed-inner fixture scenario exceeded its ceiling: hang \(elapsed [0-9]+s, scaled cap [0-9]+s\)$' "$output"
  grep -Fq 'fixture-bed-inner fixture scenario hang failed while serving leg hang with status 124' "$output"
  grep -Fqx -- '- hang (rc=124)' "$output"
  grep -Fqx "group $child_pid: TERM sent" "$output"
  grep -Fqx "group $child_pid: alive after 5s grace; KILL sent" "$output"
  grep -Fqx "group $child_pid: empty" "$output"
  assert_no_group_survivor "$child_pid" "$grandchild_pid"
  exit 0
fi

if [[ "$fixture_scenario" == ceiling-expiry-mutation ]]; then
  mutation_root=$tmp/expiry-mutation-root
  mutation_script=$mutation_root/scripts/agents/fixture-bed-scenarios-fixtures.sh
  mutation_output=$tmp/expiry-mutation.out
  mutation_witness=$tmp/expiry-mutation.witness
  mutation_source=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/$(basename "${BASH_SOURCE[0]}")
  mkdir -p "$mutation_root/scripts/agents" "$mutation_root/bin"
  cp "$mutation_source" "$mutation_script"
  ln -s "$root/scripts/agents/fixture-budget.sh" "$mutation_root/scripts/agents/fixture-budget.sh"
  ln -s "$root/scripts/agents/fixture-bed-scenarios.sh" "$mutation_root/scripts/agents/fixture-bed-scenarios.sh"
  ln -s "$root/scripts/agents/fixture-assert.sh" "$mutation_root/scripts/agents/fixture-assert.sh"
  ln -s "$root/bin/metasystem" "$mutation_root/bin/metasystem"
  ln -s "$root/metasystem.conf" "$mutation_root/metasystem.conf"
  mutation_count=$(grep -Fxc '    SECONDS=${live_deadlines[0]}' "$mutation_script" || true)
  [[ "$mutation_count" -eq 1 ]] \
    || { echo "fixture-bed-scenarios mutation expected exactly one expiry assignment, found $mutation_count" >&2; exit 1; }
  sed '/^    SECONDS=${live_deadlines\[0\]}$/d' "$mutation_script" >"$mutation_script.next"
  mv "$mutation_script.next" "$mutation_script"
  chmod +x "$mutation_script"
  mutation_count_after=$(grep -Fxc '    SECONDS=${live_deadlines[0]}' "$mutation_script" || true)
  [[ "$mutation_count_after" -eq 0 ]] \
    || { echo "fixture-bed-scenarios mutation left $mutation_count_after expiry assignments" >&2; exit 1; }
  mutation_outer_rc=0
  set +e
  env -u METASYSTEM_FIXTURE_OWNER -u METASYSTEM_FIXTURE_ATTEMPT \
    METASYSTEM_FIXTURE_ONLY=ceiling-reaps-group \
    FIXTURE_BED_EXPECT_EXPIRY_MUTATION=1 \
    FIXTURE_BED_EXPIRY_MUTATION_WITNESS="$mutation_witness" \
    "$mutation_script" >"$mutation_output" 2>&1
  mutation_outer_rc=$?
  set -e
  [[ "$mutation_outer_rc" -ne 0 ]] \
    || { echo "fixture-bed-scenarios expiry mutation unexpectedly passed" >&2; cat "$mutation_output" >&2; exit 1; }
  [[ -s "$mutation_witness" ]] \
    || { echo "fixture-bed-scenarios expiry mutation omitted descendant custody" >&2; cat "$mutation_output" >&2; exit 1; }
  mutation_witness_body=$(<"$mutation_witness")
  mutation_witness_line_count=$(awk 'END { print NR }' "$mutation_witness")
  read -r mutation_reason mutation_child mutation_child_ref mutation_grandchild mutation_grandchild_ref mutation_extra \
    <<<"$mutation_witness_body"
  [[ "$mutation_witness_line_count" -eq 4 && "$mutation_reason" == missing-expiry && -z "$mutation_extra" &&
      "$mutation_witness_body" == "$(printf 'missing-expiry %s %s %s %s\ngroup %s: TERM sent\ngroup %s: alive after 5s grace; KILL sent\ngroup %s: empty' \
        "$mutation_child" "$mutation_child_ref" "$mutation_grandchild" "$mutation_grandchild_ref" \
        "$mutation_child" "$mutation_child" "$mutation_child")" ]] \
    || { echo "fixture-bed-scenarios expiry mutation wrote invalid durable cleanup evidence" >&2; exit 1; }
  wait_fixture_ref_gone "expiry mutation child" "$mutation_child" "$mutation_child_ref"
  wait_fixture_ref_gone "expiry mutation grandchild" "$mutation_grandchild" "$mutation_grandchild_ref"
  printf 'expiry mutation witness: mutation_count=%s mutation_count_after=%s outer_status=%s missing-expiry child=%s grandchild=%s descendants=gone\n' \
    "$mutation_count" "$mutation_count_after" "$mutation_outer_rc" "$mutation_child" "$mutation_grandchild"
  exit 0
fi

if [[ "$fixture_scenario" == signal-reaps-group ]]; then
  inner=$tmp/inner-bed.sh
  output=$tmp/signal.out
  child_file=$tmp/child.pid
  grandchild_file=$tmp/grandchild.pid
  write_inner_bed "$inner"
    env -u METASYSTEM_FIXTURE_ONLY \
      FIXTURE_BED_SOURCE_ROOT="$root" FIXTURE_BED_INNER_SCENARIOS=hang \
    FIXTURE_BED_CHILD_PID_FILE="$child_file" \
    FIXTURE_BED_GRANDCHILD_PID_FILE="$grandchild_file" \
    "$inner" "$harness_fixture_tag" >"$output" 2>&1 &
  inner_pid=$!
  signal_deadline=$((SECONDS + $(harness_fixture_cap bed-scenario)))
  while [[ ! -s "$grandchild_file" ]] && kill -0 "$inner_pid" 2>/dev/null \
      && (( SECONDS < signal_deadline )); do
    sleep 0.25
  done
  [[ -s "$child_file" && -s "$grandchild_file" ]] || {
    echo "fixture-bed-scenarios signal fixture did not observe the inner process group" >&2
    wait "$inner_pid" 2>/dev/null || true
    sed -n '1,200p' "$output" >&2
    exit 1
  }
  child_pid=$(<"$child_file")
  grandchild_pid=$(<"$grandchild_file")
  kill -TERM "$inner_pid"
  set +e
  wait "$inner_pid"
  inner_rc=$?
  set -e
  [[ $inner_rc -eq 143 ]] || {
    echo "fixture-bed-scenarios signaled inner bed exited $inner_rc, want 143" >&2
    sed -n '1,200p' "$output" >&2
    exit 1
  }
  grep -Fqx "group $child_pid: TERM sent" "$output"
  grep -Fqx "group $child_pid: alive after 5s grace; KILL sent" "$output"
  grep -Fqx "group $child_pid: empty" "$output"
  assert_no_group_survivor "$child_pid" "$grandchild_pid"
  exit 0
fi

if [[ "$fixture_scenario" == hang-leash ]]; then
  hang_leash_started=$SECONDS
  unset METASYSTEM_FIXTURE_ONLY
  hang_leash_outer_custodian_log=${harness_fixture_custodian_log:-}
  interrupt_owner= interrupt_child_pid= interrupt_grandchild_pid=
  leash_owner= leash_pid= custodian_owner= custodian_child=
  interrupt_custodian_log_file= leash_log_file= custodian_log_file=

  hang_leash_show_custodian_log() { # label, log path
    local label=$1 path=$2
    if [[ -n "$path" && -f "$path" ]]; then
      echo "custodian-log $label path=$path" >&2
      cat "$path" >&2
    else
      echo "custodian-log $label path=${path:-unavailable} content=unavailable" >&2
    fi
  }

  hang_leash_show_custodian_log_path() { # label, file containing log path
    local label=$1 path_file=$2 path=
    if [[ -n "$path_file" && -s "$path_file" ]]; then
      IFS= read -r path <"$path_file" || true
    fi
    hang_leash_show_custodian_log "$label" "$path"
  }

  hang_leash_diagnostics() {
    local pid
    set +e
    echo "fixture-bed-scenarios fixture: hang-leash failure diagnostics elapsed=$((SECONDS - hang_leash_started))s" >&2
    hang_leash_show_custodian_log outer "$hang_leash_outer_custodian_log"
    hang_leash_show_custodian_log_path interrupt "$interrupt_custodian_log_file"
    hang_leash_show_custodian_log_path leash "$leash_log_file"
    hang_leash_show_custodian_log_path disabled "$custodian_log_file"
    echo "pid-states:" >&2
    for pid in "$interrupt_owner" "$interrupt_child_pid" "$interrupt_grandchild_pid" \
        "$leash_owner" "$leash_pid" "$custodian_owner" "$custodian_child"; do
      [[ "$pid" =~ ^[1-9][0-9]*$ ]] || continue
      ps -o pid=,ppid=,pgid=,state=,etime=,command= -p "$pid" >&2 \
        || echo "pid $pid: gone" >&2
    done
  }

  hang_leash_exit() {
    local status=$?
    trap - EXIT
    (( status == 0 )) || hang_leash_diagnostics
    rm -rf "$tmp" 2>/dev/null || true
    exit "$status"
  }
  trap hang_leash_exit EXIT

  inner=$tmp/inner-bed.sh
  write_inner_bed "$inner"
  hang_leash_int_wait_sec=$((10 * (fixture_bed_term_grace_sec + fixture_bed_kill_grace_sec)))

  # The ordinary bed cleanup still empties the process group after an INT.
  interrupt_output=$tmp/hang-interrupt.out
  interrupt_child=$tmp/hang-interrupt-child.pid
  interrupt_grandchild=$tmp/hang-interrupt-grandchild.pid
  interrupt_custodian_log_file=$tmp/hang-interrupt-custodian.log-path
  interrupt_launcher_probe_status=0
  interrupt_launcher_probe_output=$(harness_fixture_engine_call proc default-signals -- \
    /bin/sh -c 'trap - INT; kill -INT $$; echo alive' 2>&1) \
    || interrupt_launcher_probe_status=$?
  if [[ "$interrupt_launcher_probe_output" == *alive* ]] \
      || [[ "$interrupt_launcher_probe_status" =~ ^(2|126|127)$ ]]; then
    interrupt_launcher_probe_reason=$interrupt_launcher_probe_output
    [[ -n "$interrupt_launcher_probe_reason" ]] \
      || interrupt_launcher_probe_reason="exit code $interrupt_launcher_probe_status"
    echo "fixture-bed-scenarios fixture: hang-leash INT leg cannot deliver INT through the launcher: $interrupt_launcher_probe_reason" >&2
    exit 1
  fi
  set -m
  # A non-interactive shell's background job inherits INT and QUIT ignored, and Bash cannot trap a signal ignored at entry.
  FIXTURE_BED_SOURCE_ROOT="$root" FIXTURE_BED_INNER_SCENARIOS=hang \
    FIXTURE_BED_CHILD_PID_FILE="$interrupt_child" \
    FIXTURE_BED_GRANDCHILD_PID_FILE="$interrupt_grandchild" \
    FIXTURE_BED_CUSTODIAN_LOG_FILE="$interrupt_custodian_log_file" \
    "$harness_fixture_engine" proc default-signals -- \
    "$inner" "$harness_fixture_tag" >"$interrupt_output" 2>&1 9>&- &
  interrupt_owner=$!
  set +m
  harness_fixture_hold_pid "$interrupt_owner"
  interrupt_deadline=$((SECONDS + hang_leash_int_wait_sec))
  while [[ ! -s "$interrupt_child" || ! -s "$interrupt_grandchild" ]] \
      && kill -0 "$interrupt_owner" 2>/dev/null && (( SECONDS < interrupt_deadline )); do
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
  done
  [[ -s "$interrupt_child" && -s "$interrupt_grandchild" ]] || {
    echo "fixture-bed-scenarios fixture: hang-leash INT leg did not publish both pids" >&2
    cat "$interrupt_output" >&2
    exit 1
  }
  interrupt_child_pid=$(<"$interrupt_child")
  interrupt_grandchild_pid=$(<"$interrupt_grandchild")
  harness_fixture_hold_pid "$interrupt_child_pid"
  harness_fixture_hold_pid "$interrupt_grandchild_pid"
  interrupt_grandchild_ref=$(harness_fixture_engine_call proc ref --pid "$interrupt_grandchild_pid")
  kill -INT "$interrupt_owner"
  interrupt_deadline=$((SECONDS + hang_leash_int_wait_sec))
  while kill -0 "$interrupt_owner" 2>/dev/null && (( SECONDS < interrupt_deadline )); do
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
  done
  if kill -0 "$interrupt_owner" 2>/dev/null; then
    echo "fixture-bed-scenarios fixture: hang-leash INT leg did not exit after INT" >&2
    cat "$interrupt_output" >&2
    fixture_bed_reap_group "$interrupt_owner" || true
    wait "$interrupt_owner" 2>/dev/null || true
    exit 1
  fi
  interrupt_status=0
  wait "$interrupt_owner" || interrupt_status=$?
  [[ $interrupt_status -eq 130 ]] || {
    echo "fixture-bed-scenarios fixture: hang-leash INT bed exited $interrupt_status, want 130" >&2
    cat "$interrupt_output" >&2
    exit 1
  }
  wait_fixture_ref_gone "hang grandchild after bed INT" \
    "$interrupt_grandchild_pid" "$interrupt_grandchild_ref"

  leash_child=$tmp/leash-child.sh
  cat >"$leash_child" <<'LEASH_CHILD'
#!/bin/sh
if [ -n "${METASYSTEM_FIXTURE_OWNER-}" ]; then
  tag="METASYSTEM_FIXTURE_OWNER=$METASYSTEM_FIXTURE_OWNER"
  attempt_tag=
  [ -z "${METASYSTEM_FIXTURE_ATTEMPT-}" ] || attempt_tag="METASYSTEM_FIXTURE_ATTEMPT=$METASYSTEM_FIXTURE_ATTEMPT"
  if [ "${1-}" != "$tag" ]; then
    [ -z "$attempt_tag" ] || exec /bin/sh "$0" "$tag" "$attempt_tag" "$@"
    exec /bin/sh "$0" "$tag" "$@"
  fi
  if [ -n "$attempt_tag" ] && [ "${2-}" != "$attempt_tag" ]; then
    shift
    exec /bin/sh "$0" "$tag" "$attempt_tag" "$@"
  fi
  shift
  [ -z "$attempt_tag" ] || shift
fi
printf '%s\n' "$$" >"${FIXTURE_CHILD_PID_FILE:?}"
trap '' TERM
exec 3<"${METASYSTEM_FIXTURE_LEASH:?}"
read -r _ <&3
LEASH_CHILD
  chmod +x "$leash_child"

  # Closing the owner's leash releases the reader before the custodian needs
  # to signal it; the completed log is re-read on every poll.
  leash_pid_file=$tmp/leash-fast.pid
  leash_log_file=$tmp/leash-fast.log-path
  bash -c '
    set -euo pipefail
    root=$1 child=$2 pid_file=$3 log_file=$4
    source "$root/scripts/agents/fixture-budget.sh"
    harness_fixture_owner "$root"
    harness_fixture_key hang-leash-fast
    printf "%s\n" "$harness_fixture_custodian_log" >"$log_file"
    METASYSTEM_FIXTURE_OWNER="$harness_fixture_key_value" \
      FIXTURE_CHILD_PID_FILE="$pid_file" "$child" "$harness_fixture_tag" 9>&- &
    child_pid=$!
    harness_fixture_hold_pid "$child_pid"
    wait "$child_pid"
  ' bash "$root" "$leash_child" "$leash_pid_file" "$leash_log_file" 9>&- &
  leash_owner=$!
  harness_fixture_hold_pid "$leash_owner"
  leash_deadline=$((SECONDS + $(harness_fixture_cap bed-scenario)))
  while [[ ! -s "$leash_pid_file" || ! -s "$leash_log_file" ]] \
      && kill -0 "$leash_owner" 2>/dev/null && (( SECONDS < leash_deadline )); do
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
  done
  [[ -s "$leash_pid_file" && -s "$leash_log_file" ]] || {
    echo "fixture-bed-scenarios fixture: leash-fast owner did not publish its child and log" >&2
    exit 1
  }
  leash_pid=$(<"$leash_pid_file")
  leash_log=$(<"$leash_log_file")
  harness_fixture_hold_pid "$leash_pid"
  leash_ref=$(harness_fixture_engine_call proc ref --pid "$leash_pid")
  kill -KILL "$leash_owner"
  wait "$leash_owner" 2>/dev/null || true
  wait_fixture_ref_gone "leashed child after owner KILL" "$leash_pid" "$leash_ref" suite-watchdog-wait
  wait_fixture_log_line "$leash_log" "action=complete" suite-watchdog-wait
  if grep -Fq "action=kill pid=$leash_pid" "$leash_log"; then
    echo "fixture-bed-scenarios fixture: custodian signaled leashed child $leash_pid instead of observing its exit" >&2
    cat "$leash_log" >&2
    exit 1
  fi

  # With the leash removed, the same ownership record leaves the custodian as
  # the safety and its log names the child it kills.
  custodian_pid_file=$tmp/leash-disabled.pid
  custodian_log_file=$tmp/leash-disabled.log-path
  bash -c '
    set -euo pipefail
    root=$1 child=$2 pid_file=$3 log_file=$4
    source "$root/scripts/agents/fixture-budget.sh"
    harness_fixture_owner "$root"
    harness_fixture_key hang-leash-disabled
    printf "%s\n" "$harness_fixture_custodian_log" >"$log_file"
    tags=("$harness_fixture_tag")
    if [[ -n "${METASYSTEM_FIXTURE_ATTEMPT:-}" ]]; then
      tags+=("METASYSTEM_FIXTURE_ATTEMPT=$METASYSTEM_FIXTURE_ATTEMPT")
    fi
    METASYSTEM_FIXTURE_OWNER="$harness_fixture_key_value" \
      METASYSTEM_FIXTURE_LEASH= FIXTURE_CHILD_PID_FILE="$pid_file" \
      /bin/sh -c "$(sed "s|exec 3<.*|while :; do sleep 1; done|; /read -r _ <&3/d" "$child")" \
      sh "${tags[@]}" 9>&- &
    child_pid=$!
    harness_fixture_hold_pid "$child_pid"
    wait "$child_pid"
  ' bash "$root" "$leash_child" "$custodian_pid_file" "$custodian_log_file" 9>&- &
  custodian_owner=$!
  harness_fixture_hold_pid "$custodian_owner"
  custodian_deadline=$((SECONDS + $(harness_fixture_cap bed-scenario)))
  while [[ ! -s "$custodian_pid_file" || ! -s "$custodian_log_file" ]] \
      && kill -0 "$custodian_owner" 2>/dev/null && (( SECONDS < custodian_deadline )); do
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
  done
  [[ -s "$custodian_pid_file" && -s "$custodian_log_file" ]] || {
    echo "fixture-bed-scenarios fixture: leash-disabled owner did not publish its child and log" >&2
    exit 1
  }
  custodian_child=$(<"$custodian_pid_file")
  custodian_log=$(<"$custodian_log_file")
  harness_fixture_hold_pid "$custodian_child"
  custodian_ref=$(harness_fixture_engine_call proc ref --pid "$custodian_child")
  kill -KILL "$custodian_owner"
  wait "$custodian_owner" 2>/dev/null || true
  wait_fixture_log_line "$custodian_log" "action=kill pid=$custodian_child"
  wait_fixture_ref_gone "leash-disabled child after owner KILL" "$custodian_child" "$custodian_ref"
  exit 0
fi

if [[ "$fixture_scenario" == command-substitution-failure ]]; then
  inner=$tmp/inner-bed.sh
  output=$tmp/command-substitution.out
  write_inner_bed "$inner"
  set +e
  FIXTURE_BED_SOURCE_ROOT="$root" FIXTURE_BED_INNER_SCENARIOS='json-read-fails exit-fails' \
    "$inner" "$harness_fixture_tag" >"$output" 2>&1
  inner_rc=$?
  set -e
  [[ $inner_rc -eq 1 ]] || {
    echo "fixture-bed-scenarios JSON-read inner bed exited $inner_rc, want 1" >&2
    sed -n '1,200p' "$output" >&2
    exit 1
  }
  grep -Fq 'fixture-bed-inner fixture scenario json-read-fails failed while serving leg read-broken-json with status 1' "$output" \
    || { echo "JSON-read failure did not name its scenario and leg" >&2; sed -n '1,200p' "$output" >&2; exit 1; }
  if grep -Fq 'failed while serving leg read-broken-json at line ' "$output"; then
    echo "JSON-read failure claimed an unproven Bash 3.2 line" >&2
    sed -n '1,200p' "$output" >&2
    exit 1
  fi
  grep -Fq 'fixture-bed-inner fixture scenario exit-fails failed while serving leg explicit-exit with status 7' "$output" \
    || { echo "explicit exit did not name its scenario, leg and status" >&2; sed -n '1,200p' "$output" >&2; exit 1; }
  if grep -Fq 'failed while serving leg explicit-exit at line ' "$output"; then
    echo "explicit exit claimed an unproven Bash 3.2 line" >&2
    sed -n '1,200p' "$output" >&2
    exit 1
  fi
  exit 0
fi

if [[ "$fixture_scenario" == collects-every-failure ]]; then
  inner=$tmp/inner-bed.sh
  output=$tmp/collects-every-failure.out
  write_inner_bed "$inner"
  set +e
  FIXTURE_BED_SOURCE_ROOT="$root" FIXTURE_BED_INNER_SCENARIOS='fail-one pass fail-two' \
    "$inner" "$harness_fixture_tag" >"$output" 2>&1
  inner_rc=$?
  set -e
  [[ $inner_rc -eq 1 ]] || {
    echo "fixture-bed-scenarios multi-failure inner bed exited $inner_rc, want 1" >&2
    sed -n '1,240p' "$output" >&2
    exit 1
  }
  grep -Fqx -- '- fail-one (rc=1)' "$output" \
    || { echo "multi-failure inner bed omitted fail-one" >&2; sed -n '1,240p' "$output" >&2; exit 1; }
  grep -Fqx -- '- fail-two (rc=1)' "$output" \
    || { echo "multi-failure inner bed omitted fail-two" >&2; sed -n '1,240p' "$output" >&2; exit 1; }
  grep -Eq '^fixture-bed-inner fixture scenario passed: pass \([0-9]+s\)$' "$output" \
    || { echo "multi-failure inner bed did not run its passing scenario" >&2; sed -n '1,240p' "$output" >&2; exit 1; }
  exit 0
fi

echo "fixture-bed-scenarios fixture is unknown: $fixture_scenario" >&2
exit 64
