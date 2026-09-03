#!/usr/bin/env bash
# Gate for mission birth-token (goal job-record-birth-token).
# Emits one metric: the number of BirthToken-named Go fixtures in
# internal/dispatch that PASS. The design names six; the mission reaches
# its target when all six exist and are green. Runs from the metasystem
# project root inside the materialized candidate worktree. Exit 0 means
# "measurement ran" regardless of the threshold; a non-zero exit is a
# measurement failure.
set -uo pipefail
passes=$(go test ./internal/dispatch/ -run 'BirthToken' -json -count=1 2>/dev/null \
  | grep '"Test":' | grep -c '"Action":"pass"' || true)
passes=${passes:-0}
echo "metric=birth-token-fixtures=${passes}"
exit 0
