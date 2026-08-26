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
failed_names=()
while IFS=$'\t' read -r section_id section_name extra; do
  if [[ -z "$section_id" || -z "$section_name" || -n "${extra:-}" \
    || ! "$section_id" =~ ^[a-z0-9][a-z0-9-]*$ ]]; then
    echo "enumeration selector emitted an invalid section row: $section_id" >&2
    exit 2
  fi
  section_log="$work/$section_id.log"
  section_rc=0
  if bash "$selector" run "$section_id" >"$section_log" 2>&1; then
    section_status=pass
  else
    section_rc=$?
    section_status=fail
    sections_failed=$((sections_failed + 1))
    failed_names+=("$section_name")
  fi
  sections_run=$((sections_run + 1))

  failure_tail=
  if [[ "$section_status" == fail ]]; then
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
(( sections_failed == 0 )) || exit 1
