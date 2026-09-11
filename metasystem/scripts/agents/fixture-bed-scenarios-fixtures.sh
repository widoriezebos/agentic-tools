#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
source "$root/scripts/agents/fixture-budget.sh"
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
if (( ! fixture_bed_child )); then
  fixture_bed_script=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/$(basename "${BASH_SOURCE[0]}")
  run_fixture_bed_scenarios fixture-bed-scenarios \
    "fixture-bed-scenarios fixtures passed (4 isolated scenarios)" \
    "$fixture_bed_script" budget-standalone budget-inherited ceiling-reaps-group signal-reaps-group
fi

tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-fixture-bed-scenarios.XXXXXX")
trap 'rm -rf "$tmp"' EXIT

write_inner_bed() { # destination
  local destination=$1
  cat >"$destination" <<'INNER'
#!/usr/bin/env bash
set -euo pipefail

root=${FIXTURE_BED_SOURCE_ROOT:?}
source "$root/scripts/agents/fixture-budget.sh"
source "$root/scripts/agents/fixture-bed-scenarios.sh"
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
    "$0" "${FIXTURE_BED_INNER_SCENARIO:?}"
fi

case "$fixture_scenario" in
  print-scale)
    printf '%s\n' "$METASYSTEM_FIXTURE_CAP_SCALE_MILLI"
    ;;
  hang)
    printf '%s\n' "$$" >"${FIXTURE_BED_CHILD_PID_FILE:?}"
    bash -c 'trap "" TERM; sleep 600' &
    fixture_grandchild_pid=$!
    printf '%s\n' "$fixture_grandchild_pid" >"${FIXTURE_BED_GRANDCHILD_PID_FILE:?}"
    sleep 600
    ;;
  *)
    echo "fixture-bed inner scenario is unknown: $fixture_scenario" >&2
    exit 64
    ;;
esac
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

if [[ "$fixture_scenario" == budget-standalone ]]; then
  inner=$tmp/inner-bed.sh
  output=$tmp/standalone.out
  write_inner_bed "$inner"
  (
    unset METASYSTEM_FIXTURE_CAP_SCALE METASYSTEM_FIXTURE_CAP_SCALE_MILLI
    FIXTURE_BED_SOURCE_ROOT="$root" FIXTURE_BED_INNER_SCENARIO=print-scale \
      "$inner"
  ) >"$output" 2>&1
  standalone_milli=$(sed -n '/^[0-9][0-9]*$/p' "$output" | head -1)
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
    FIXTURE_BED_SOURCE_ROOT="$root" FIXTURE_BED_INNER_SCENARIO=print-scale \
      "$inner"
  ) >"$operator_output" 2>&1
  METASYSTEM_FIXTURE_CAP_SCALE=3 METASYSTEM_FIXTURE_CAP_SCALE_MILLI=3000 \
    FIXTURE_BED_SOURCE_ROOT="$root" FIXTURE_BED_INNER_SCENARIO=print-scale \
    "$inner" >"$parent_output" 2>&1
  [[ $(sed -n '/^[0-9][0-9]*$/p' "$operator_output" | head -1) == 3000 ]]
  [[ $(sed -n '/^[0-9][0-9]*$/p' "$parent_output" | head -1) == 3000 ]]
  exit 0
fi

if [[ "$fixture_scenario" == ceiling-reaps-group ]]; then
  inner=$tmp/inner-bed.sh
  output=$tmp/ceiling.out
  child_file=$tmp/child.pid
  grandchild_file=$tmp/grandchild.pid
  write_inner_bed "$inner"
  set +e
  METASYSTEM_BED_SCENARIO_FIXTURE_TIMEOUT_SEC=1 \
    FIXTURE_BED_SOURCE_ROOT="$root" FIXTURE_BED_INNER_SCENARIO=hang \
    FIXTURE_BED_CHILD_PID_FILE="$child_file" \
    FIXTURE_BED_GRANDCHILD_PID_FILE="$grandchild_file" \
    "$inner" >"$output" 2>&1
  inner_rc=$?
  set -e
  [[ $inner_rc -eq 1 ]] || {
    echo "fixture-bed-scenarios ceiling inner bed exited $inner_rc, want 1" >&2
    sed -n '1,200p' "$output" >&2
    exit 1
  }
  child_pid=$(<"$child_file")
  grandchild_pid=$(<"$grandchild_file")
  grep -Eq '^fixture-bed-inner fixture scenario exceeded its ceiling: hang \(elapsed [0-9]+s, scaled cap [0-9]+s\)$' "$output"
  grep -Fqx "group $child_pid: TERM sent" "$output"
  grep -Fqx "group $child_pid: alive after 5s grace; KILL sent" "$output"
  grep -Fqx "group $child_pid: empty" "$output"
  assert_no_group_survivor "$child_pid" "$grandchild_pid"
  exit 0
fi

if [[ "$fixture_scenario" == signal-reaps-group ]]; then
  inner=$tmp/inner-bed.sh
  output=$tmp/signal.out
  child_file=$tmp/child.pid
  grandchild_file=$tmp/grandchild.pid
  write_inner_bed "$inner"
  FIXTURE_BED_SOURCE_ROOT="$root" FIXTURE_BED_INNER_SCENARIO=hang \
    FIXTURE_BED_CHILD_PID_FILE="$child_file" \
    FIXTURE_BED_GRANDCHILD_PID_FILE="$grandchild_file" \
    "$inner" >"$output" 2>&1 &
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

echo "fixture-bed-scenarios fixture is unknown: $fixture_scenario" >&2
exit 64
