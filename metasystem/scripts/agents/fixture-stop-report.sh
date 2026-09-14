#!/usr/bin/env bash

# Read the immutable report named by one provider Stop payload. The embedded
# identity must bind the report to that command, installation, runtime, and
# session before a fixture may inspect its contents.
fixture_stop_status_report() ( # provider payload file or JSON, installed root, runtime, session
  local payload=$1 installation=$2 expected_runtime=$3 expected_session=$4
  local engine decision decision_rc=0 field=systemMessage line command_name report_verb status_verb
  local id_flag report_id extra report marker_count identity_line identity_json
  local canonical_installation identity_installation identity_runtime identity_session
  local identity_session_key identity_attempt payload_shape staged_payload=
  local payload_args=()

  engine=$installation/bin/metasystem
  [[ -f "$engine" && ! -L "$engine" ]] || return 1
  if [[ -f "$payload" && ! -L "$payload" ]]; then
    payload_args=(--file "$payload")
  else
    staged_payload=$(mktemp "${TMPDIR:-/tmp}/metasystem-stop-payload.XXXXXX") || return 1
    trap 'rm -f "$staged_payload"' EXIT
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
  [[ "$line" != *$'\n'* && "$line" != *$'\r'* ]] || return 1
  (( $(printf '%s' "$line" | wc -c | tr -d ' ') <= 256 )) || return 1

  read -r command_name report_verb status_verb id_flag report_id extra <<<"${line##*; status: }"
  [[ "$command_name" == metasystem && "$report_verb" == report && "$status_verb" == stop-status &&
     "$id_flag" == --id && "$report_id" =~ ^[0-9a-f]{64}-[0-9a-f]{32}$ && -z "$extra" ]] || return 1
  report=$("$engine" "$report_verb" "$status_verb" "$id_flag" "$report_id") || return 1

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
  [[ "$identity_installation" == "$canonical_installation" &&
     "$identity_runtime" == "$expected_runtime" &&
     "$identity_session" == "$expected_session" &&
     "$identity_session_key-$identity_attempt" == "$report_id" ]] || return 1

  printf '%s\n' "$report"
)
