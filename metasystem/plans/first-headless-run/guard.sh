#!/usr/bin/env bash
# Guard for mission birth-token: the build and vet must stay green while
# the gate climbs. Emits metric=build=1 when both pass, 0 otherwise. Exit
# 0 means "measurement ran"; the floor (1) is judged by the runner.
set -uo pipefail
if go build ./... >/dev/null 2>&1 && go vet ./internal/dispatch/ >/dev/null 2>&1; then
  echo "metric=build=1"
else
  echo "metric=build=0"
fi
exit 0
