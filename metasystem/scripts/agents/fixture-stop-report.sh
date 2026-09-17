#!/usr/bin/env bash

# Read the immutable report bound to one provider Stop payload. The engine
# verifies the payload shape, response record, report bytes, and identity.
fixture_stop_status_report() ( # provider payload file or JSON, installed root, runtime, session
  local payload=$1 installation=$2 expected_runtime=$3 expected_session=$4
  local engine payload_file staged_payload=
  trap 'rm -f "$staged_payload"' EXIT

  engine=$installation/bin/metasystem
  [[ -f "$engine" && ! -L "$engine" ]] || return 1
  if [[ -f "$payload" && ! -L "$payload" ]]; then
    payload_file=$payload
  else
    staged_payload=$(mktemp "${TMPDIR:-/tmp}/metasystem-stop-payload.XXXXXX") || return 1
    printf '%s' "$payload" >"$staged_payload" || return 1
    payload_file=$staged_payload
  fi

  "$engine" report stop-response --root "$installation" --payload-file "$payload_file" \
    --runtime "$expected_runtime" --session "$expected_session"
)
