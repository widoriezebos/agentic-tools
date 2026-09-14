#!/usr/bin/env bash

# Read the immutable report named by one provider Stop payload. The embedded
# identity must bind the report to that command, installation, runtime, and
# session before a fixture may inspect its contents.
fixture_stop_status_report() ( # provider payload file or JSON, installed root, runtime, session
  local payload=$1 installation=$2 expected_runtime=$3 expected_session=$4
  local engine decision decision_rc=0 field=systemMessage line line_one line_two command
  local command_name report_verb status_verb id_flag report_alias extra report marker_count identity_line identity_json
  local canonical_installation identity_installation identity_runtime identity_session
  local identity_session_key identity_attempt payload_shape staged_payload= report_file=
  local alias_file alias_name report_id alias_digest canonical_report actual_digest
  local payload_args=()
  trap 'rm -f "$staged_payload" "$report_file"' EXIT

  engine=$installation/bin/metasystem
  [[ -f "$engine" && ! -L "$engine" ]] || return 1
  if [[ -f "$payload" && ! -L "$payload" ]]; then
    payload_args=(--file "$payload")
  else
    staged_payload=$(mktemp "${TMPDIR:-/tmp}/metasystem-stop-payload.XXXXXX") || return 1
    printf '%s' "$payload" >"$staged_payload" || return 1
    payload_args=(--file "$staged_payload")
  fi
  decision=$("$engine" json get "${payload_args[@]}" --field decision 2>/dev/null) || decision_rc=$?
  if [[ "$decision" == block ]]; then
    field=reason
    payload_shape=$("$engine" json strip "${payload_args[@]}" --key decision --key reason) || return 1
    [[ "$payload_shape" == '{}' ]] || return 1
    ! "$engine" json get "${payload_args[@]}" --field systemMessage >/dev/null 2>&1 || return 1
  else
    (( decision_rc != 0 )) || return 1
    payload_shape=$("$engine" json strip "${payload_args[@]}" --key systemMessage) || return 1
    [[ "$payload_shape" == '{}' ]] || return 1
    ! "$engine" json get "${payload_args[@]}" --field reason >/dev/null 2>&1 || return 1
  fi
  line=$("$engine" json get "${payload_args[@]}" --field "$field") || return 1
  [[ "$line" != *$'\r'* && $(grep -c '^' <<<"$line") -eq 2 ]] || return 1
  line_one=${line%%$'\n'*}
  line_two=${line#*$'\n'}
  [[ "$line_one" == 'Just completed: '* && "$line_one" == *. ]] || return 1
  (( $(printf '%s' "$line_one" | wc -c | tr -d ' ') <= 144 )) || return 1
  (( $(printf '%s' "$line_two" | wc -c | tr -d ' ') <= 256 )) || return 1
  (( $(printf '%s' "$line" | wc -c | tr -d ' ') <= 401 )) || return 1

  if [[ "$decision" == block ]]; then
    [[ "$line_two" == *'; Stop blocked;'* && "$line_two" != *'; Stop allowed;'* && "$line_two" == *'; Do not stop. Run this command; read and act on its report: metasystem report stop-status --id '* && "$line_two" != *'; Report: '* ]] || return 1
    command=${line_two##*; Do not stop. Run this command; read and act on its report: }
  else
    [[ "$line_two" == *'; Stop allowed;'* && "$line_two" != *'; Stop blocked;'* && "$line_two" == *'; Report: metasystem report stop-status --id '* && "$line_two" != *'; Do not stop. Run this command;'* ]] || return 1
    command=${line_two##*; Report: }
  fi
  read -r command_name report_verb status_verb id_flag report_alias extra <<<"$command"
  [[ "$command_name" == metasystem && "$report_verb" == report && "$status_verb" == stop-status &&
     "$id_flag" == --id && "$report_alias" =~ ^[0-9a-f]{1,32}$ && -z "$extra" ]] || return 1
  report_file=$(mktemp "${TMPDIR:-/tmp}/metasystem-stop-report.XXXXXX") || return 1
  "$engine" "$report_verb" "$status_verb" "$id_flag" "$report_alias" >"$report_file" || return 1
  report=$(command cat "$report_file") || return 1

  marker_count=$(printf '%s\n' "$report" |
    awk '/^<!-- metasystem-stop-report-v1 .* -->$/ { count++ } END { print count + 0 }')
  [[ "$marker_count" == 1 ]] || return 1
  identity_line=$(printf '%s\n' "$report" |
    sed -n '/^<!-- metasystem-stop-report-v1 .* -->$/p')
  identity_json=${identity_line#'<!-- metasystem-stop-report-v1 '}
  identity_json=${identity_json%' -->'}
  canonical_installation=$(cd "$installation" && pwd -P) || return 1
  identity_installation=$("$engine" json get --value "$identity_json" --field installation) || return 1
  identity_runtime=$("$engine" json get --value "$identity_json" --field runtime) || return 1
  identity_session=$("$engine" json get --value "$identity_json" --field session) || return 1
  identity_session_key=$("$engine" json get --value "$identity_json" --field sessionKey) || return 1
  identity_attempt=$("$engine" json get --value "$identity_json" --field attempt) || return 1
  report_id=$identity_session_key-$identity_attempt
  [[ "$identity_installation" == "$canonical_installation" &&
     "$identity_runtime" == "$expected_runtime" &&
     "$identity_session" == "$expected_session" &&
     "$identity_attempt" == "$report_alias"* ]] || return 1

  alias_file=$canonical_installation/artifacts/agents/supervision/stop-verdicts/aliases/$report_alias.json
  [[ -f "$alias_file" && ! -L "$alias_file" ]] || return 1
  alias_name=$("$engine" json get --file "$alias_file" --field alias) || return 1
  [[ "$alias_name" == "$report_alias" &&
     $("$engine" json get --file "$alias_file" --field state) == published &&
     $("$engine" json get --file "$alias_file" --field reportId) == "$report_id" ]] || return 1
  alias_digest=$("$engine" json get --file "$alias_file" --field sha256) || return 1
  canonical_report=$canonical_installation/artifacts/agents/supervision/stop-verdicts/$report_id.md
  [[ -f "$canonical_report" && ! -L "$canonical_report" ]] || return 1
  cmp -s "$report_file" "$canonical_report" || return 1
  actual_digest=$("$engine" util sha256 <"$canonical_report") || return 1
  [[ "$actual_digest" == "$alias_digest" ]] || return 1
  grep -Fq $'## Console text\n\n```text\n'"$line"$'\n```' "$canonical_report" || return 1

  printf '%s\n' "$report"
)
