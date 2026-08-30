#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "Usage: scripts/agents/enumerate-suite.sh --report <path> [--selector <path>]" >&2
}

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
selector="$script_dir/validate-section-selector.sh"
report=
while [[ $# -gt 0 ]]; do
  case $1 in
    --report)
      [[ $# -ge 2 ]] || { usage; exit 2; }
      report=$2
      shift 2
      ;;
    --selector)
      [[ $# -ge 2 ]] || { usage; exit 2; }
      selector=$2
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      usage
      exit 2
      ;;
  esac
done
[[ -n "$report" ]] || { usage; exit 2; }
[[ -f "$selector" ]] \
  || { echo "enumeration selector does not exist: $selector" >&2; exit 2; }

case $report in
  /*) ;;
  *) report="$PWD/$report" ;;
esac
report_dir=$(dirname "$report")
mkdir -p "$report_dir"

work=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-enumerate.XXXXXX")
report_tmp=$(mktemp "$report_dir/.metasystem-enumeration.XXXXXX")
cleanup() {
  rm -rf "$work"
  [[ ! -e "$report_tmp" ]] || rm -f "$report_tmp"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

section_list="$work/sections"
if ! bash "$selector" list >"$section_list"; then
  echo "enumeration selector could not list its sections: $selector" >&2
  exit 2
fi
[[ -s "$section_list" ]] \
  || { echo "enumeration selector listed no sections: $selector" >&2; exit 2; }

printf 'format\tmetasystem-enumeration-v1\n' >"$report_tmp"
printf 'columns\tkind\tid\tname\tstatus\texit_code\tfailure_tail\n' >>"$report_tmp"

sections_run=0
sections_failed=0
sections_gated=0
sections_invalid=0
failed_names=()
gated_names=()
invalid_names=()
engine_dependency=unproven
enumeration_progress_path=${METASYSTEM_ENUMERATION_PROGRESS_PATH:-}
enumeration_progress_suite=${METASYSTEM_ENUMERATION_PROGRESS_SUITE:-}
enumeration_progress_depth=${METASYSTEM_ENUMERATION_PROGRESS_DEPTH:-0}
enumeration_progress_coordinated=${METASYSTEM_ENUMERATION_PROGRESS_COORDINATED:-0}
if [[ "$enumeration_progress_coordinated" != 1 ]]; then
  enumeration_progress_path=
fi
if [[ -n "$enumeration_progress_path" ]]; then
  [[ "$enumeration_progress_path" == /* \
    && -n "$enumeration_progress_suite" \
    && "$enumeration_progress_depth" =~ ^[0-9]+$ ]] \
    || { echo "enumeration progress coordinates are invalid" >&2; exit 2; }
fi
append_enumeration_progress() { # section, start|end
  local section=$1 event=$2 at
  [[ -n "$enumeration_progress_path" ]] || return 0
  at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  printf '{"suite":"%s","section":"%s","event":"%s","at":"%s","depth":%d}\n' \
    "$enumeration_progress_suite" "$section" "$event" "$at" \
    "$enumeration_progress_depth" >>"$enumeration_progress_path"
}
while IFS=$'\t' read -r section_id section_name extra; do
  if [[ -z "$section_id" || -z "$section_name" || -n "${extra:-}" \
    || ! "$section_id" =~ ^[a-z0-9][a-z0-9-]*$ ]]; then
    echo "enumeration selector emitted an invalid section row: $section_id" >&2
    exit 2
  fi
  section_log="$work/$section_id.log"
  section_stage_results="$work/$section_id.stage-results.tsv"
  section_rc=0
  recorded_tail=
  selector_rc=0
  append_enumeration_progress "$section_id" start
  METASYSTEM_ENUMERATION_ENGINE_DEPENDENCY=$engine_dependency \
    METASYSTEM_ENUMERATION_STAGE_RESULTS_OUT=$section_stage_results \
    METASYSTEM_ENUMERATION_PROGRESS_DRIVER=$enumeration_progress_coordinated \
    bash "$selector" run "$section_id" >"$section_log" 2>&1 || selector_rc=$?
  append_enumeration_progress "$section_id" end

  section_status=
  recorded_count=0
  if [[ -f "$section_stage_results" ]]; then
    recorded_count=$(awk -F '\t' -v selected="$section_id" \
      '$1 == "section" && $2 == selected { count++ } END { print count + 0 }' \
      "$section_stage_results")
    if [[ "$recorded_count" == 1 ]]; then
      section_status=$(awk -F '\t' -v selected="$section_id" \
        '$1 == "section" && $2 == selected { print $3 }' "$section_stage_results")
      section_rc=$(awk -F '\t' -v selected="$section_id" \
        '$1 == "section" && $2 == selected { print $4 }' "$section_stage_results")
      recorded_tail=$(awk -F '\t' -v selected="$section_id" \
        '$1 == "section" && $2 == selected { print $5 }' "$section_stage_results")
      case "$section_status:$selector_rc" in
        pass:0 | gated:0 | fail:1) ;;
        *) section_status=invalid; section_rc=$selector_rc ;;
      esac
    else
      section_status=invalid
      section_rc=$selector_rc
    fi
  elif [[ $selector_rc -eq 0 ]]; then
    section_status=pass
  elif [[ $selector_rc -eq 2 ]]; then
    section_status=invalid
    section_rc=$selector_rc
  else
    section_status=fail
    section_rc=$selector_rc
  fi

  case "$section_status" in
    fail)
      sections_failed=$((sections_failed + 1))
      failed_names+=("$section_name")
      ;;
    gated)
      sections_gated=$((sections_gated + 1))
      gated_names+=("$section_name")
      ;;
    invalid)
      sections_invalid=$((sections_invalid + 1))
      invalid_names+=("$section_name")
      ;;
  esac
  if [[ "$section_id" == go-engine-gate ]]; then
    if [[ "$section_status" == pass ]]; then
      engine_dependency=ready
    else
      engine_dependency=failed
    fi
  fi
  sections_run=$((sections_run + 1))

  failure_tail=
  if [[ ( "$section_status" == gated || "$section_status" == fail ) \
    && -n "${recorded_tail:-}" ]]; then
    failure_tail=$recorded_tail
  elif [[ "$section_status" == fail || "$section_status" == invalid ]]; then
    failure_tail=$(tail -n 20 "$section_log" | awk '
      BEGIN { first = 1 }
      {
        gsub(/\\/, "\\\\")
        gsub(/\t/, "\\t")
        gsub(/\r/, "\\r")
        if (!first) printf "\\n"
        printf "%s", $0
        first = 0
      }
      END { if (!first) printf "\\n" }
    ')
  fi
  printf 'section\t%s\t%s\t%s\t%d\t%s\n' \
    "$section_id" "$section_name" "$section_status" "$section_rc" "$failure_tail" \
    >>"$report_tmp"
  printf 'enumeration: %s: %s (exit %d)\n' \
    "$section_name" "$section_status" "$section_rc" >&2
done <"$section_list"

mv "$report_tmp" "$report"
failed_list=none
if (( sections_failed > 0 )); then
  failed_list=
  for failed_name in "${failed_names[@]}"; do
    if [[ -n "$failed_list" ]]; then
      failed_list="$failed_list, $failed_name"
    else
      failed_list=$failed_name
    fi
  done
fi
printf 'Enumeration complete: %d sections run / %d failed; failed sections: %s\n' \
  "$sections_run" "$sections_failed" "$failed_list"
printf 'Enumeration dependency outcomes: %d gated; invalid sections: %d\n' \
  "$sections_gated" "$sections_invalid"
(( sections_invalid == 0 )) || exit 2
(( sections_failed == 0 )) || exit 1
