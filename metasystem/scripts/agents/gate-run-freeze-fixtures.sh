#!/usr/bin/env bash
set -euo pipefail

# Fast integration proof for the isolated controller. The subject repository
# carries a fixture engine and validator, so clone/evidence/Git interference
# is exercised without recursively running the milestone battery.
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
real_engine=$root/bin/metasystem
[[ -x "$real_engine" ]] \
  || { echo "gate-run-freeze fixture: real engine absent; run the Go gate first" >&2; exit 1; }
export BATTERY_REAL_ENGINE=$real_engine

# Print one top-level element per line from the engine's compact rendering of
# a JSON array. The walk is depth- and string-aware, so commas in nested values
# or quoted strings do not split an inventory row.
json_elements() { # compact JSON array
  printf '%s' "$1" | awk '
    {
      n = length($0)
      if (n < 2 || substr($0, 1, 1) != "[") exit 1
      depth = 0; instring = 0; escaped = 0; start = 2
      for (i = 2; i < n; i++) {
        ch = substr($0, i, 1)
        if (instring) {
          if (escaped) escaped = 0
          else if (ch == "\\") escaped = 1
          else if (ch == "\"") instring = 0
        } else if (ch == "\"") instring = 1
        else if (ch == "{" || ch == "[") depth++
        else if (ch == "}" || ch == "]") depth--
        else if (ch == "," && depth == 0) {
          print substr($0, start, i - start)
          start = i + 1
        }
      }
      if (n > 2) print substr($0, start, n - start)
    }'
}

revoked_directory() { # directory stem below the fixture temp root
  local stem=$1 candidate
  for candidate in "$tmp/$stem".revoked.*; do
    [[ -d "$candidate" ]] || continue
    printf '%s\n' "$candidate"
    return 0
  done
}

tmp=$(mktemp -d)
enumeration_pid=
publication_failure_bystander=
recycled_pid_sentinel=
publication_retention_launch_pid=
assisted_pid=
cleanup() {
  if [[ -n "$enumeration_pid" ]]; then
    kill "$enumeration_pid" 2>/dev/null || true
    wait "$enumeration_pid" 2>/dev/null || true
  fi
  if [[ -n "$assisted_pid" ]]; then
    kill -KILL "$assisted_pid" 2>/dev/null || true
    wait "$assisted_pid" 2>/dev/null || true
  fi
  if [[ -d "$tmp" ]]; then
    while IFS= read -r owner; do
      pid=$("$real_engine" json get --file "$owner" --field pid 2>/dev/null || true)
      [[ "$pid" =~ ^[1-9][0-9]*$ ]] && kill -TERM "$pid" 2>/dev/null || true
    done < <(find "$tmp" -path '*/artifacts/agents/supervision/lock.d/owner.json' -type f 2>/dev/null)
    while IFS= read -r pidfile; do
      pid=$(cat "$pidfile" 2>/dev/null || true)
      [[ "$pid" =~ ^[1-9][0-9]*$ ]] && kill -KILL "$pid" 2>/dev/null || true
    done < <(find "$tmp" \( -name 'validator-pid*' -o -name 'validator-child-pid*' \) -type f 2>/dev/null)
  fi
	if [[ -n "$publication_failure_bystander" ]]; then
		kill -KILL "$publication_failure_bystander" 2>/dev/null || true
		wait "$publication_failure_bystander" 2>/dev/null || true
	fi
	if [[ -n "$recycled_pid_sentinel" ]]; then
		kill -KILL "$recycled_pid_sentinel" 2>/dev/null || true
		wait "$recycled_pid_sentinel" 2>/dev/null || true
	fi
	if [[ -n "$publication_retention_launch_pid" ]]; then
		kill -KILL "$publication_retention_launch_pid" 2>/dev/null || true
	fi
	rm -rf -- "$tmp" 2>/dev/null || true
}
trap cleanup EXIT
fixture_failed() { # exit code, source line, failed command
  local rc=$1 line=$2 command=$3
  case $- in *e*) ;; *) return 0 ;; esac
  trap - ERR
  printf 'gate-run-freeze fixture failed at line %s (exit %s): %s\n' \
    "$line" "$rc" "$command" >&2
  exit "$rc"
}
trap 'fixture_failed "$?" "$LINENO" "$BASH_COMMAND"' ERR

repo=$tmp/live
evidence=$tmp/evidence
remote=$tmp/remote.git
mkdir -p "$repo/metasystem/scripts/agents/adapters" "$repo/metasystem/bin" \
	"$repo/metasystem/plans" "$evidence"
cp "$root/scripts/agents/milestone-battery.sh" "$repo/metasystem/scripts/agents/"
cp "$root/scripts/agents/battery.conf.local.template" "$repo/metasystem/scripts/agents/"
cp "$root/scripts/agents/dispatch.sh" "$repo/metasystem/scripts/agents/"
cp "$root/scripts/agents/checkout-execution-guard.sh" "$repo/metasystem/scripts/agents/"
cp "$root/scripts/watch-background-jobs.sh" "$repo/metasystem/scripts/"
printf '# fixture fingerprint input\n' >"$repo/metasystem/scripts/agents/adapters/runtime-common.sh"
cat >"$repo/metasystem/scripts/agents/adapters/fake.sh" <<'ADAPTER'
#!/usr/bin/env bash
set -euo pipefail
[[ ${1:-} == signature ]] || exit 2
printf 'match (^|[[:space:]/-])metasystem-fake-agent([[:space:]]|$)\n'
ADAPTER
chmod +x "$repo/metasystem/scripts/agents/adapters/fake.sh"

cat >"$repo/metasystem/scripts/agents/fake-engine.sh" <<'ENGINE'
#!/usr/bin/env bash
set -euo pipefail
family=${1:-}; verb=${2:-}; shift 2 || true
value() { local flag=$1; shift; while (($#)); do [[ "$1" == "$flag" ]] && { printf '%s' "$2"; return; }; shift; done; }
case "$family/$verb" in
  proc/group-members)
    pgid=$(value --pgid "$@")
    leader_path=${BATTERY_VALIDATOR_PID_PATH:-}
    child_path=${BATTERY_VALIDATOR_CHILD_PID_PATH:-}
    leader=
    if [[ -n "$leader_path" && -f "$leader_path" ]]; then
      leader=$(cat "$leader_path")
    fi
    [[ -z "$leader_path" || ! -e "$leader_path.quiesced" ]] || exit 0
    if [[ -z "$leader_path" && "${BATTERY_FIXTURE_SEAMS:-0}" != 1 ]]; then
      exec "$BATTERY_REAL_ENGINE" proc group-members "$@"
    fi
    if [[ -n "$leader" && "$leader" != "$pgid" && "${BATTERY_FIXTURE_SEAMS:-0}" != 1 ]]; then
      exec "$BATTERY_REAL_ENGINE" proc group-members "$@"
    fi
    # Every member the fixture creates is published through these two files.
    # Reading those known identities keeps this bed portable to hosts that
    # deny process-table enumeration while production still uses the real census.
    for pid_path in "$leader_path" "$child_path"; do
      [[ -n "$pid_path" && -f "$pid_path" ]] || continue
      pid=$(cat "$pid_path")
      [[ "$pid" =~ ^[1-9][0-9]*$ ]] || continue
      identity=$("$BATTERY_REAL_ENGINE" proc probe --pid "$pid" 2>/dev/null) || continue
      liveness=$("$BATTERY_REAL_ENGINE" json get --value "$identity" --field liveness 2>/dev/null) || continue
      [[ "$liveness" == alive ]] && printf '%s\n' "$pid"
    done
    exit 0 ;;
  json/set)
    file=$(value --file "$@")
    if [[ "${BATTERY_FAKE_TEARDOWN_APPENDIX_FAIL:-0}" == 1 && "$file" == */.teardown.*.json ]]; then
      echo 'fixture teardown appendix write failed' >&2
      exit 92
    fi
    exec "$BATTERY_REAL_ENGINE" json set --file "$file" "$@" ;;
  gate/weight-reset)
    root=$(value --root "$@")
    case "${BATTERY_FAKE_RESET_MODE:-normal}" in
      open) echo 'fixture reset write failed' >&2; exit 1 ;;
      appendix-pending)
        state=$root/artifacts/agents/battery-weight.json
        envelope=$("$BATTERY_REAL_ENGINE" json get --file "$state" \
          --field checkpoint.repairDestination)
        chmod 500 "$envelope"
        set +e
        "$BATTERY_REAL_ENGINE" gate weight-reset --root "$root" "$@"
        rc=$?
        set -e
        chmod 755 "$envelope"
        exit "$rc" ;;
      normal) exec "$BATTERY_REAL_ENGINE" gate weight-reset --root "$root" "$@" ;;
      *) exit 2 ;;
    esac ;;
  *) exec "$BATTERY_REAL_ENGINE" "$family" "$verb" "$@" ;;
esac
ENGINE
chmod +x "$repo/metasystem/scripts/agents/fake-engine.sh"

cat >"$repo/metasystem/scripts/agents/go-build.sh" <<'BUILD'
#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
[[ ${1:-} == --out && -n ${2:-} ]] || exit 2
[[ "${BATTERY_FAKE_BUILD_FAIL:-0}" != 1 ]] || { echo 'fixture build failure' >&2; exit 89; }
cp "$root/scripts/agents/fake-engine.sh" "$2"
chmod +x "$2"
BUILD
chmod +x "$repo/metasystem/scripts/agents/go-build.sh"

cat >"$repo/metasystem/scripts/validate-metasystem.sh" <<'VALIDATE'
#!/usr/bin/env bash
set -euo pipefail
class_out=${METASYSTEM_BATTERY_RUN_CLASS_OUT:-}
class_writer=${METASYSTEM_BATTERY_ROOT_CLASS_WRITER:-0}
stage_results_out=${METASYSTEM_VALIDATION_STAGE_RESULTS_OUT:-}
stage_results_writer=${METASYSTEM_VALIDATION_STAGE_RESULTS_WRITER:-0}
[[ "$class_writer" == 1 && "$class_out" == /* ]]
[[ "$stage_results_writer" == 1 && "$stage_results_out" == /* \
  && ! -e "$stage_results_out" && ! -L "$stage_results_out" ]]
for witness_var in METASYSTEM_GATE_WITNESS METASYSTEM_GATE_WITNESS_ROOT \
  METASYSTEM_GATE_WITNESS_RUN METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE \
  METASYSTEM_GATE_WITNESS_WRITE METASYSTEM_GATE_WITNESS_CONTROLLER_PID \
  METASYSTEM_GATE_WITNESS_CONTROLLER_STARTED_AT METASYSTEM_GATE_WITNESS_CONTROLLER_START_TICKS \
  METASYSTEM_GATE_WITNESS_CONTROLLER_BOOT_ID; do
  [[ -z "${!witness_var:-}" ]]
done
unset METASYSTEM_BATTERY_RUN_CLASS_OUT METASYSTEM_BATTERY_ROOT_CLASS_WRITER \
  METASYSTEM_VALIDATION_STAGE_RESULTS_OUT METASYSTEM_VALIDATION_STAGE_RESULTS_WRITER
class_stage=${class_out}.stage.$$
case "${BATTERY_FAKE_RUN_CLASS:-FULL}" in
  FULL|WITNESS-ASSISTED) fixture_run_class=${BATTERY_FAKE_RUN_CLASS:-FULL} ;;
  *) exit 2 ;;
esac
umask 077
printf 'format\tmetasystem-validation-stage-results-v1\n' >"$stage_results_out"
printf 'columns\tkind\tid\tstatus\texit_code\tfailure_tail\n' >>"$stage_results_out"
chmod 600 "$stage_results_out"
printf '%s\n' "$fixture_run_class" >"$class_stage"
chmod 600 "$class_stage"
mv "$class_stage" "$class_out"
metasystem=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
checkout=$(cd "$metasystem/.." && pwd -P)
[[ -z "${BATTERY_VALIDATION_LOG_MARKER:-}" ]] || printf '%s\n' "$BATTERY_VALIDATION_LOG_MARKER"
validator_child=
json_elements() { # compact JSON array
  printf '%s' "$1" | awk '
    {
      n = length($0)
      if (n < 2 || substr($0, 1, 1) != "[") exit 1
      depth = 0; instring = 0; escaped = 0; start = 2
      for (i = 2; i < n; i++) {
        ch = substr($0, i, 1)
        if (instring) {
          if (escaped) escaped = 0
          else if (ch == "\\") escaped = 1
          else if (ch == "\"") instring = 0
        } else if (ch == "\"") instring = 1
        else if (ch == "{" || ch == "[") depth++
        else if (ch == "}" || ch == "]") depth--
        else if (ch == "," && depth == 0) {
          print substr($0, start, i - start)
          start = i + 1
        }
      }
      if (n > 2) print substr($0, start, n - start)
    }'
}
census_has_validator() { # report, validator pid, checkout root, excluded live root
  local report=$1 wanted_pid=$2 checkout_root=$3 live_root=$4
  local inventory row row_pid row_cwd matched=0
  [[ "$("$BATTERY_REAL_ENGINE" json get --file "$report" --field verdict \
    --default __MISSING__)" == SUCCESS ]] || return 1
  inventory=$("$BATTERY_REAL_ENGINE" json get --file "$report" --field inventory) \
    || return 1
  while IFS= read -r row; do
    row_pid=$("$BATTERY_REAL_ENGINE" json get --value "$row" --field pid) || return 1
    row_cwd=$("$BATTERY_REAL_ENGINE" json get --value "$row" --field cwd) || return 1
    [[ "$row_cwd" != "$live_root"* ]] || return 1
    if [[ "$row_pid" == "$wanted_pid" && "$row_cwd" == "$checkout_root"* ]]; then
      matched=1
    fi
  done < <(json_elements "$inventory")
  [[ $matched == 1 ]]
}
cleanup_validator() {
  [[ -z "$validator_child" ]] || kill "$validator_child" 2>/dev/null || true
  [[ -z "$validator_child" ]] || wait "$validator_child" 2>/dev/null || true
  [[ -z "${BATTERY_VALIDATOR_PID_PATH:-}" ]] || : >"$BATTERY_VALIDATOR_PID_PATH.quiesced"
}
if [[ "${BATTERY_VALIDATOR_CHILD_SURVIVES_TERM:-0}" == 1 ]]; then
  trap '' TERM
else
  trap cleanup_validator EXIT INT TERM
fi
if [[ -n "${BATTERY_SEAM_OBSERVATION:-}" ]]; then
  printf 'hold=%s\npublication=%s\nstage=%s\nlocator=%s\nstall=%s\n' \
    "${BATTERY_LAUNCH_HOLD_FILE:-<unset>}" \
    "${BATTERY_VALIDATOR_PGID_FILE:-<unset>}" \
    "${BATTERY_VALIDATOR_PGID_STAGE:-<unset>}" \
    "${BATTERY_VALIDATOR_LAUNCH_PID_FILE:-<unset>}" \
    "${BATTERY_VALIDATOR_PUBLICATION_STALL_DIR:-<unset>}" >"$BATTERY_SEAM_OBSERVATION"
  exit 73
fi
[[ -z "${BATTERY_VALIDATOR_PID_PATH:-}" ]] || rm -f -- "$BATTERY_VALIDATOR_PID_PATH.quiesced"
if [[ "${BATTERY_VALIDATOR_DELAY_READY:-0}" == 1 ]]; then
  printf '%s\n' "$checkout" >"$BATTERY_CLONE_PATH"
  printf '%s\n' "$$" >"$BATTERY_VALIDATOR_PID_PATH"
  bash -c 'trap "" TERM; exec -a metasystem-fake-agent sleep 300' &
  validator_child=$!
  printf '%s\n' "$validator_child" >"$BATTERY_VALIDATOR_CHILD_PID_PATH"
  for ((attempt=0; attempt<3000; attempt++)); do sleep 0.01; done
  touch "$BATTERY_READY"
  for _ in $(seq 1 2000); do [[ -e "$BATTERY_RELEASE" ]] && exit 0; sleep 0.01; done
  exit 1
fi
[[ ! -e "$checkout/uncommitted.txt" && ! -e "$checkout/ignored.secret" ]]
[[ "$(cat "$metasystem/plans/goals.md")" == 'subject ledger' ]]
[[ "$(cat "$metasystem/memory/receipts.log")" == 'subject receipts' ]]
[[ ! -e "$metasystem/artifacts/agents/live-only" ]]
! grep -q 'LIVE-SECRET' "$metasystem/metasystem.conf.local"
grep -q '^evidence.root=.*/isolated-evidence$' "$metasystem/metasystem.conf.local"
[[ "${HOME:-}" == "${BATTERY_EXPECT_HOME:-}" ]]
[[ -n "${GOCACHE:-}" && -n "${METASYSTEM_SUPERVISION_REGISTRY_HOME:-}" ]]
isolated_registry=$METASYSTEM_SUPERVISION_REGISTRY_HOME/.metasystem/armed-checkouts.jsonl
[[ -s "$isolated_registry" ]]
grep -Fq "$metasystem" "$isolated_registry"
! grep -Fq "$BATTERY_LIVE_ROOT" "$isolated_registry"
owner=$metasystem/artifacts/agents/supervision/lock.d/owner.json
[[ -f "$owner" ]]
owner_pid=$("$BATTERY_REAL_ENGINE" json get --file "$owner" --field pid)
kill -0 "$owner_pid"
[[ -f "$metasystem/artifacts/agents/supervision/last-census.json" ]]
! grep -Fq "$BATTERY_LIVE_ROOT" "$metasystem/artifacts/agents/supervision/last-census.json"
printf '%s\n' "$checkout" >"$BATTERY_CLONE_PATH"
printf '%s\n' "$METASYSTEM_SUPERVISION_REGISTRY_HOME" >"$BATTERY_REGISTRY_PATH"
[[ -z "${BATTERY_VALIDATOR_PID_PATH:-}" ]] || printf '%s\n' "$$" >"$BATTERY_VALIDATOR_PID_PATH"
if [[ "${BATTERY_VALIDATOR_CHILD_SURVIVES_TERM:-0}" == 1 ]]; then
  bash -c 'trap "" TERM; exec -a metasystem-fake-agent sleep 300' &
else
  bash -c 'exec -a metasystem-fake-agent sleep 300' &
fi
validator_child=$!
[[ -z "${BATTERY_VALIDATOR_CHILD_PID_PATH:-}" ]] || printf '%s\n' "$validator_child" >"$BATTERY_VALIDATOR_CHILD_PID_PATH"
validator_started=$("$BATTERY_REAL_ENGINE" proc started-at --pid "$validator_child")
process_fixture=$metasystem/artifacts/agents/supervision/process-fixture.json
process_fixture_stage=$(mktemp "$metasystem/artifacts/agents/supervision/.process-fixture.XXXXXX")
printf '[{"pid":%s,"ppid":%s,"pgid":%s,"pidStartedAt":%s,"argv":"metasystem-fake-agent sleep 300","cwd":"%s","cwdError":false,"alive":true}]\n' \
  "$validator_child" "$$" "$$" "$validator_started" "$metasystem" >"$process_fixture_stage"
mv "$process_fixture_stage" "$process_fixture"
for ((attempt=0; attempt<400; attempt++)); do
  if census_has_validator \
    "$metasystem/artifacts/agents/supervision/last-census.json" \
    "$validator_child" "$checkout" "$BATTERY_LIVE_ROOT"
  then break; fi
  sleep 0.01
done
census_has_validator \
  "$metasystem/artifacts/agents/supervision/last-census.json" \
  "$validator_child" "$checkout" "$BATTERY_LIVE_ROOT"
[[ ! -f "$BATTERY_LIVE_REGISTRY" ]] || ! grep -Fq "$checkout" "$BATTERY_LIVE_REGISTRY"
touch "$BATTERY_READY"
for _ in $(seq 1 2000); do [[ -e "$BATTERY_RELEASE" ]] && exit 0; sleep 0.01; done
echo 'fixture release timed out' >&2
exit 1
VALIDATE
chmod +x "$repo/metasystem/scripts/validate-metasystem.sh"

cat >"$repo/metasystem/scripts/agents/arm-supervision.sh" <<'ARM'
#!/usr/bin/env bash
set -euo pipefail
[[ -z "${BATTERY_TEARDOWN_CALL:-}" ]] || { printf '%q ' "$@" >"$BATTERY_TEARDOWN_CALL"; printf '\n' >>"$BATTERY_TEARDOWN_CALL"; }
repo=
for ((i=1; i<=$#; i++)); do
  if [[ "${!i}" == --repo ]]; then j=$((i+1)); repo=${!j}; fi
done
[[ -n "$repo" ]]
[[ "${BATTERY_FAKE_ARM_NOOP:-0}" != 1 ]] || exit 0
harness=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
owner=$harness/artifacts/agents/supervision/lock.d/owner.json
if [[ " $* " == *' --shutdown '* ]]; then
  if [[ "${BATTERY_FAKE_TEARDOWN_FAIL:-0}" == 1 ]]; then
    echo "fixture forced teardown failure" >&2
    exit 91
  fi
  [[ -f "$owner" ]] || exit 0
  pid=$("$BATTERY_REAL_ENGINE" json get --file "$owner" --field pid)
  kill -TERM "$pid" 2>/dev/null || true
  for ((attempt=0; attempt<400; attempt++)); do
    kill -0 "$pid" 2>/dev/null || break
    sleep 0.01
  done
  if kill -0 "$pid" 2>/dev/null; then
    kill -KILL "$pid" 2>/dev/null || true
    sleep 0.05
  fi
  kill -0 "$pid" 2>/dev/null && { echo "fixture owner did not stop" >&2; exit 1; }
  rm -rf -- "${owner%/owner.json}"
else
  mkdir -p "${owner%/owner.json}"
  process_fixture=$harness/artifacts/agents/supervision/process-fixture.json
  printf '[]\n' >"$process_fixture"
  gate=$harness/artifacts/agents/supervision/owner-gate.$$
  tag=fixture-battery-owner-$$
  fingerprint=$("$BATTERY_REAL_ENGINE" supervise fingerprint --root "$harness" --repo "$repo")
  METASYSTEM_CENSUS_PROCESS_FILE="$process_fixture" \
    "$BATTERY_REAL_ENGINE" supervise owner --repo "$harness" --scope "$repo" \
    --tag "$tag" --interval 1 --watcher-cap 1 --fingerprint "$fingerprint" --gate "$gate" \
    >"$harness/artifacts/agents/supervision/owner-fixture.log" 2>&1 &
  pid=$!
  started=$("$BATTERY_REAL_ENGINE" proc started-at --pid "$pid")
  printf '{"pid":%s,"pidStartedAt":%s,"instanceTag":"%s"}\n' "$pid" "$started" "$tag" >"$owner"
  touch "$gate"
  registry=$METASYSTEM_SUPERVISION_REGISTRY_HOME/.metasystem/armed-checkouts.jsonl
  # The owner's very first census can land before its own state
  # publication and record a startup failure; arming is complete only
  # once a census SUCCEEDED, never merely once the files exist.
  for ((attempt=0; attempt<1000; attempt++)); do
    [[ -s "$registry" && -s "$harness/artifacts/agents/supervision/state.json" ]] \
      && grep -q '"verdict": *"SUCCESS"' "$harness/artifacts/agents/supervision/last-census.json" 2>/dev/null && break
    kill -0 "$pid" 2>/dev/null || { cat "$harness/artifacts/agents/supervision/owner-fixture.log" >&2; exit 1; }
    sleep 0.01
  done
  [[ -s "$registry" && -s "$harness/artifacts/agents/supervision/state.json" ]]
  grep -q '"verdict": *"SUCCESS"' "$harness/artifacts/agents/supervision/last-census.json"
  [[ -z "${BATTERY_OWNER_PID_PATH:-}" ]] || printf '%s\n' "$pid" >"$BATTERY_OWNER_PID_PATH"
fi
ARM
chmod +x "$repo/metasystem/scripts/agents/arm-supervision.sh"

printf 'evidence.root=<placeholder>\nmetasystem.runtimes=fake\nrole.default.model.fake=fake-model\n' >"$repo/metasystem/metasystem.conf"
printf 'subject ledger\n' >"$repo/metasystem/plans/goals.md"
mkdir -p "$repo/metasystem/memory"
printf 'subject receipts\n' >"$repo/metasystem/memory/receipts.log"
printf 'metasystem.conf.local\nbin/\nartifacts/\n*.secret\n' >"$repo/metasystem/.gitignore"
printf 'sentinel-live-binary\n' >"$repo/metasystem/bin/metasystem"
chmod +x "$repo/metasystem/bin/metasystem"
git -C "$repo" init -q -b main
git -C "$repo" config user.name fixture
git -C "$repo" config user.email fixture@example.invalid
git -C "$repo" add .
git -C "$repo" commit -qm subject
subject=$(git -C "$repo" rev-parse HEAD)
git init --bare -q "$remote"
git -C "$repo" remote add origin "$remote"

run_nested_failure_copy_fixture() {
  local nested_failure_tree nested_no_rg_bin command_name red_subject red_envelope copied_command
  local rejected_link symlink_subject symlink_runs symlink_rc symlink_retained
  nested_failure_tree=$repo/metasystem/artifacts/agents/suite-failures/nested-validation-failure
  nested_no_rg_bin=$nested_failure_tree/no-rg-bin
  mkdir -p "$nested_no_rg_bin"
  for command_name in cat find grep sort tr wc; do
    cp "$(command -v "$command_name")" "$nested_no_rg_bin/$command_name"
    chmod +x "$nested_no_rg_bin/$command_name"
  done
  printf 'nested validation failure\n' >"$nested_failure_tree/validation.log"
  git -C "$repo" add -f -- metasystem/artifacts/agents/suite-failures/nested-validation-failure
  git -C "$repo" commit -qm nested-validation-failure
  red_subject=$(git -C "$repo" rev-parse HEAD)
  BATTERY_FAKE_ARM_NOOP=1 \
    "$repo/metasystem/scripts/agents/milestone-battery.sh" \
      --subject "$red_subject" --evidence-root "$evidence" --force-red \
      >"$tmp/red.out" 2>"$tmp/red.err" || true
  red_envelope=$(sed -n 's/.* envelope=\(.*\)$/\1/p' "$tmp/red.out" "$tmp/red.err" | tail -1)
  [[ -n "$red_envelope" && -f "$red_envelope/report.json" \
     && -f "$red_envelope/abandoned.json" && -s "$red_envelope/copy-digests.nul" ]]
  [[ -f "$red_envelope/outcome.json" ]]
  [[ -f "$red_envelope/failure-artifacts/suite-failures/forced-red/failure.txt" ]]
  [[ -f "$red_envelope/failure-artifacts/suite-failures/nested-validation-failure/validation.log" ]]
  tr '\0' '\n' <"$red_envelope/copy-digests.nul" >"$tmp/red-digests.txt"
  for command_name in cat find grep sort tr wc; do
    copied_command=$red_envelope/failure-artifacts/suite-failures/nested-validation-failure/no-rg-bin/$command_name
    [[ -x "$copied_command" && ! -L "$copied_command" ]]
    grep -Fqx "failure-artifacts/suite-failures/nested-validation-failure/no-rg-bin/$command_name" \
      "$tmp/red-digests.txt"
  done
  [[ "$("$real_engine" json get --file "$red_envelope/report.json" --field verdict)" == red ]]
  [[ "$("$real_engine" json get --file "$red_envelope/report.json" --field copyDigestManifest)" == copy-digests.nul ]]
  [[ ! -e "$red_envelope/reset.json" ]]

  # Symlink evidence remains terminal and names the rejected entry so an
  # operator can locate the artifact that prevented verified publication.
  rejected_link=$nested_failure_tree/no-rg-bin/rg
  ln -s grep "$rejected_link"
  git -C "$repo" add -f -- metasystem/artifacts/agents/suite-failures/nested-validation-failure
  git -C "$repo" commit -qm nested-validation-symlink
  symlink_subject=$(git -C "$repo" rev-parse HEAD)
  symlink_runs=$tmp/symlink-runs
  mkdir -p "$symlink_runs"
  set +e
  TMPDIR="$symlink_runs" BATTERY_FAKE_ARM_NOOP=1 \
    "$repo/metasystem/scripts/agents/milestone-battery.sh" \
      --subject "$symlink_subject" --evidence-root "$evidence" --force-red \
      >"$tmp/symlink.out" 2>"$tmp/symlink.err"
  symlink_rc=$?
  set -e
  [[ $symlink_rc != 0 ]]
  grep -Fq 'milestone battery evidence copy refused symlink:' "$tmp/symlink.err"
  grep -Fq 'failure-artifacts/suite-failures/nested-validation-failure/no-rg-bin/rg' \
    "$tmp/symlink.err"
  symlink_retained=$(sed -n 's/.* path=\([^ ]*\) envelope=.*/\1/p' "$tmp/symlink.err" | tail -1)
  [[ "$symlink_retained" == "$symlink_runs"/*/subject && -d "$symlink_retained" ]]
  rm -rf -- "${symlink_retained%/subject}"
}

if [[ "${BATTERY_NESTED_FAILURE_COPY_FIXTURE_ONLY:-0}" == 1 ]]; then
  run_nested_failure_copy_fixture
  echo "gate-run-freeze nested-failure copy fixture passed"
  exit 0
fi

# The validator publishes its process group before entering the clone. The
# hold keeps the controller between launch and its PID assignments while a
# same-group descendant starts, making the launching state deterministic.
launch_abort_runs=$tmp/launch-abort-runs
launch_abort_bin=$tmp/launch-abort-bin
launch_abort_evidence=$tmp/launch-abort-evidence
launch_abort_ready=$tmp/launch-abort-ready
launch_abort_release=$tmp/launch-abort-release
launch_abort_clone_path=$tmp/launch-abort-clone-path
launch_abort_validator_pid_path=$tmp/launch-abort-validator-pid
launch_abort_child_pid_path=$tmp/launch-abort-validator-child-pid
launch_abort_hold=$tmp/launch-abort-hold
mkdir -p "$launch_abort_runs" "$launch_abort_bin" "$launch_abort_evidence"
touch "$launch_abort_hold"
cat >"$launch_abort_bin/go" <<'GO'
#!/usr/bin/env bash
set -euo pipefail
case "$*" in
  'env GOCACHE') printf '%s\n' "$BATTERY_FIXTURE_GOCACHE" ;;
  'version') printf 'go version go1.fixture darwin/arm64\n' ;;
  'env GOOS GOARCH GOFLAGS GOWORK GOEXPERIMENT CGO_ENABLED GOTOOLCHAIN')
    printf 'darwin\narm64\n\noff\n\n0\nauto\n' ;;
  *) echo "fixture go: unsupported command: $*" >&2; exit 86 ;;
esac
GO
chmod +x "$launch_abort_bin/go"
env PATH="$launch_abort_bin:$PATH" TMPDIR="$launch_abort_runs" \
  BATTERY_FIXTURE_GOCACHE="$tmp/fixture-gocache" BATTERY_FAKE_ARM_NOOP=1 \
  BATTERY_FIXTURE_SEAMS=1 \
  BATTERY_VALIDATOR_DELAY_READY=1 BATTERY_VALIDATOR_CHILD_SURVIVES_TERM=1 \
  BATTERY_LAUNCH_HOLD_FILE="$launch_abort_hold" \
  BATTERY_READY="$launch_abort_ready" \
  BATTERY_RELEASE="$launch_abort_release" BATTERY_CLONE_PATH="$launch_abort_clone_path" \
  BATTERY_VALIDATOR_PID_PATH="$launch_abort_validator_pid_path" \
  BATTERY_VALIDATOR_CHILD_PID_PATH="$launch_abort_child_pid_path" \
  "$repo/metasystem/scripts/agents/milestone-battery.sh" --subject "$subject" \
    --evidence-root "$launch_abort_evidence" \
    >"$tmp/launch-abort.out" 2>"$tmp/launch-abort.err" &
launch_abort_pid=$!
launch_abort_pgid_file=
for _ in $(seq 1 2000); do
  launch_abort_pgid_file=$(find "$launch_abort_runs" -name validator.pgid -type f -print -quit)
  [[ -n "$launch_abort_pgid_file" && -s "$launch_abort_child_pid_path" ]] && break
  kill -0 "$launch_abort_pid" 2>/dev/null || break
  sleep 0.01
done
[[ -n "$launch_abort_pgid_file" && -s "$launch_abort_child_pid_path" ]] \
  || { echo "gate-run-freeze fixture: launch-time validator identity was not published" >&2; cat "$tmp/launch-abort.err" >&2; exit 1; }
[[ ! -e "$launch_abort_ready" ]] \
  || { echo "gate-run-freeze fixture: launch-time abort missed the delayed ready window" >&2; exit 1; }
# A live hold is reachable only after validator_launching becomes 1 and before
# validator_active becomes 1 and the recorded PID fields are assigned.
[[ -e "$launch_abort_hold" ]]
launch_abort_pgid=$("$real_engine" json get --file "$launch_abort_pgid_file" --field pid)
launch_abort_validator=$(cat "$launch_abort_validator_pid_path")
launch_abort_child=$(cat "$launch_abort_child_pid_path")
[[ "$launch_abort_pgid" == "$launch_abort_validator" ]]
kill -0 -- "-$launch_abort_pgid" 2>/dev/null
kill -0 "$launch_abort_validator" 2>/dev/null
kill -0 "$launch_abort_child" 2>/dev/null
kill -TERM "$launch_abort_pid"
rm -f -- "$launch_abort_hold"
set +e
wait "$launch_abort_pid"
launch_abort_rc=$?
set -e
[[ $launch_abort_rc == 130 ]] \
  || {
    echo "gate-run-freeze fixture: launch-time abort exited $launch_abort_rc instead of 130" >&2
    cat "$tmp/launch-abort.err" >&2
    launch_abort_failed_envelope=$(sed -n 's/.* envelope=\(.*\)$/\1/p' \
      "$tmp/launch-abort.out" "$tmp/launch-abort.err" | tail -1)
    launch_abort_failed_clone=$(sed -n 's/.* path=\([^ ]*\) envelope=.*/\1/p' \
      "$tmp/launch-abort.out" "$tmp/launch-abort.err" | tail -1)
    [[ ! -f "$launch_abort_failed_envelope/setup.log" ]] \
      || tail -80 "$launch_abort_failed_envelope/setup.log" >&2
    [[ ! -f "$launch_abort_failed_envelope/validation.log" ]] \
      || tail -80 "$launch_abort_failed_envelope/validation.log" >&2
    [[ ! -f "${launch_abort_failed_clone%/subject}/setup.log" ]] \
      || tail -80 "${launch_abort_failed_clone%/subject}/setup.log" >&2
    [[ ! -f "${launch_abort_failed_clone%/subject}/validation.log" ]] \
      || tail -80 "${launch_abort_failed_clone%/subject}/validation.log" >&2
    exit 1
  }
for _ in $(seq 1 400); do
  kill -0 -- "-$launch_abort_pgid" 2>/dev/null || break
  sleep 0.01
done
! kill -0 -- "-$launch_abort_pgid" 2>/dev/null
! kill -0 "$launch_abort_validator" 2>/dev/null
! kill -0 "$launch_abort_child" 2>/dev/null
[[ ! -e "$launch_abort_ready" ]]
launch_aborted_clone=$(cat "$launch_abort_clone_path")
[[ ! -e "$launch_aborted_clone" ]] \
  || { echo "gate-run-freeze fixture: launch-time abort retained clone $launch_aborted_clone" >&2; cat "$tmp/launch-abort.err" >&2; exit 1; }
launch_abort_envelope=$(sed -n 's/.* envelope=\(.*\)$/\1/p' "$tmp/launch-abort.out" "$tmp/launch-abort.err" | tail -1)
[[ -f "$launch_abort_envelope/report.json" && -f "$launch_abort_envelope/abandoned.json" \
   && -f "$launch_abort_envelope/outcome.json" && -f "$launch_abort_envelope/teardown.json" \
   && -f "$launch_abort_envelope/setup.log" \
   && -f "$launch_abort_envelope/stage-results.tsv" \
   && ! -e "$launch_abort_envelope/reset.json" ]] \
  || { echo "gate-run-freeze fixture: launch-time abort evidence is incomplete (envelope=$launch_abort_envelope)" >&2; cat "$tmp/launch-abort.err" >&2; exit 1; }
! grep -q '^validatorPid=' "$launch_abort_envelope/setup.log" \
  || { echo "gate-run-freeze fixture: launch-time abort recorded the validator as active" >&2; exit 1; }
grep -Fq 'clone=removed' "$tmp/launch-abort.err" \
  || { echo "gate-run-freeze fixture: launch-time abort did not report clone removal" >&2; cat "$tmp/launch-abort.err" >&2; exit 1; }
if [[ "${BATTERY_LAUNCH_ABORT_FIXTURE_ONLY:-0}" == 1 ]]; then
  echo "gate-run-freeze launch-abort fixture passed"
  exit 0
fi

# A failed process-group publication exits before entering the clone. The
# controller must recognize that the launch job is gone, avoid adopting its
# recyclable PID, and finish abort teardown without the publication poll.
# Publication revocation renames the configured final path's entire parent.
# Seam callers must dedicate that parent; every enabled path below does so.
publication_failure_runs=$tmp/publication-failure-runs
publication_failure_evidence=$tmp/publication-failure-evidence
publication_failure_locked=$tmp/publication-failure-locked
publication_failure_hold=$tmp/publication-failure-hold
publication_failure_stage=$tmp/publication-failure.stage
publication_failure_final=$publication_failure_locked/validator.pgid
publication_failure_validator_pid_path=$tmp/publication-failure-validator-pid
publication_failure_child_pid_path=$tmp/publication-failure-validator-child-pid
publication_failure_clone_path=$tmp/publication-failure-clone-path
publication_failure_ready=$tmp/publication-failure-ready
publication_failure_release=$tmp/publication-failure-release
publication_failure_signal=$tmp/publication-failure-bystander-signaled
mkdir -p "$publication_failure_runs" "$publication_failure_evidence" "$publication_failure_locked"
chmod 500 "$publication_failure_locked"
touch "$publication_failure_hold"
(
  trap 'touch "$publication_failure_signal"; exit 90' TERM INT HUP
  while :; do sleep 1; done
) &
publication_failure_bystander=$!
env PATH="$launch_abort_bin:$PATH" TMPDIR="$publication_failure_runs" \
  BATTERY_FIXTURE_GOCACHE="$tmp/fixture-gocache" BATTERY_FAKE_ARM_NOOP=1 \
  BATTERY_FIXTURE_SEAMS=1 \
  BATTERY_LAUNCH_HOLD_FILE="$publication_failure_hold" \
  BATTERY_VALIDATOR_PGID_STAGE="$publication_failure_stage" \
  BATTERY_VALIDATOR_PGID_FILE="$publication_failure_final" \
  BATTERY_VALIDATOR_DELAY_READY=1 BATTERY_READY="$publication_failure_ready" \
  BATTERY_RELEASE="$publication_failure_release" BATTERY_CLONE_PATH="$publication_failure_clone_path" \
  BATTERY_VALIDATOR_PID_PATH="$publication_failure_validator_pid_path" \
  BATTERY_VALIDATOR_CHILD_PID_PATH="$publication_failure_child_pid_path" \
  "$repo/metasystem/scripts/agents/milestone-battery.sh" --subject "$subject" \
    --evidence-root "$publication_failure_evidence" \
    >"$tmp/publication-failure.out" 2>"$tmp/publication-failure.err" &
publication_failure_pid=$!
for _ in $(seq 1 2000); do
  [[ -s "$publication_failure_stage" ]] && break
  kill -0 "$publication_failure_pid" 2>/dev/null || break
  sleep 0.01
done
[[ -s "$publication_failure_stage" && ! -e "$publication_failure_final" ]] \
  || { echo "gate-run-freeze fixture: process-group publication did not fail" >&2; cat "$tmp/publication-failure.err" >&2; exit 1; }
publication_failure_launch_pid=$("$real_engine" json get --file "$publication_failure_stage" --field pid)
for _ in $(seq 1 400); do
  kill -0 "$publication_failure_launch_pid" 2>/dev/null || break
  sleep 0.01
done
! kill -0 "$publication_failure_launch_pid" 2>/dev/null
[[ -e "$publication_failure_hold" && ! -e "$publication_failure_validator_pid_path" \
   && ! -e "$publication_failure_child_pid_path" && ! -e "$publication_failure_clone_path" ]]
publication_failure_started=$(date +%s)
kill -TERM "$publication_failure_pid"
rm -f -- "$publication_failure_hold"
set +e
wait "$publication_failure_pid"
publication_failure_rc=$?
set -e
publication_failure_ended=$(date +%s)
[[ $publication_failure_rc == 130 ]] \
  || {
    echo "gate-run-freeze fixture: publication-failure abort exited $publication_failure_rc instead of 130" >&2
    cat "$tmp/publication-failure.err" >&2
    publication_failure_failed_envelope=$(sed -n 's/.* envelope=\(.*\)$/\1/p' \
      "$tmp/publication-failure.out" "$tmp/publication-failure.err" | tail -1)
    publication_failure_failed_clone=$(sed -n 's/.* path=\([^ ]*\) envelope=.*/\1/p' \
      "$tmp/publication-failure.out" "$tmp/publication-failure.err" | tail -1)
    [[ ! -f "$publication_failure_failed_envelope/setup.log" ]] \
      || tail -80 "$publication_failure_failed_envelope/setup.log" >&2
    [[ ! -f "$publication_failure_failed_envelope/validation.log" ]] \
      || tail -80 "$publication_failure_failed_envelope/validation.log" >&2
    [[ ! -f "${publication_failure_failed_clone%/subject}/setup.log" ]] \
      || tail -80 "${publication_failure_failed_clone%/subject}/setup.log" >&2
    [[ ! -f "${publication_failure_failed_clone%/subject}/validation.log" ]] \
      || tail -80 "${publication_failure_failed_clone%/subject}/validation.log" >&2
    exit 1
  }
publication_failure_elapsed=$((publication_failure_ended - publication_failure_started))
if (( publication_failure_elapsed >= 3 )); then
  echo "gate-run-freeze fixture: publication-failure finalizer took ${publication_failure_elapsed}s" >&2
  cat "$tmp/publication-failure.err" >&2
  exit 1
fi
echo "gate-run-freeze publication-failure finalizer elapsed ${publication_failure_elapsed}s"
kill -0 "$publication_failure_bystander" 2>/dev/null
[[ ! -e "$publication_failure_signal" ]]
publication_failure_envelope=$(sed -n 's/.* envelope=\(.*\)$/\1/p' "$tmp/publication-failure.out" "$tmp/publication-failure.err" | tail -1)
[[ -f "$publication_failure_envelope/report.json" \
   && -f "$publication_failure_envelope/abandoned.json" \
   && -f "$publication_failure_envelope/outcome.json" \
   && -f "$publication_failure_envelope/teardown.json" \
   && ! -e "$publication_failure_envelope/reset.json" ]] \
  || { echo "gate-run-freeze fixture: publication-failure evidence is incomplete (envelope=$publication_failure_envelope)" >&2; cat "$tmp/publication-failure.err" >&2; exit 1; }
publication_failure_revoked=$(revoked_directory publication-failure-locked)
[[ ! -e "$publication_failure_locked" && -d "$publication_failure_revoked" ]] \
  || { echo "gate-run-freeze fixture: failed publication directory was not revoked" >&2; cat "$tmp/publication-failure.err" >&2; exit 1; }
grep -Fq 'clone=removed' "$tmp/publication-failure.err" \
  || { echo "gate-run-freeze fixture: publication-failure abort did not report clone removal" >&2; cat "$tmp/publication-failure.err" >&2; exit 1; }
kill -KILL "$publication_failure_bystander" 2>/dev/null || true
wait "$publication_failure_bystander" 2>/dev/null || true
publication_failure_bystander=
if [[ "${BATTERY_PUBLICATION_FAILURE_FIXTURE_ONLY:-0}" == 1 ]]; then
  echo "gate-run-freeze publication-failure fixture passed"
  exit 0
fi

# A stalled wrapper has not published and has not entered the clone. Teardown
# must revoke the configured publication directory before it can refuse the
# wrapper; releasing it after that transition must make publication fail, and
# teardown cannot finalize refusal until it has consumed the wrapper's exit.
revocation_race_runs=$tmp/revocation-race-runs
revocation_race_evidence=$tmp/revocation-race-evidence
revocation_race_identity=$tmp/revocation-race-identity
revocation_race_stall=$tmp/revocation-race-stall
revocation_race_launch_hold=$tmp/revocation-race-launch-hold
revocation_race_stage=$revocation_race_identity/.validator-pgid.stage
revocation_race_final=$revocation_race_identity/validator.pgid
revocation_race_clone_path=$tmp/revocation-race-clone-path
revocation_race_validator_pid_path=$tmp/revocation-race-validator-pid
revocation_race_child_pid_path=$tmp/revocation-race-validator-child-pid
revocation_race_ready=$tmp/revocation-race-ready
revocation_race_release=$tmp/revocation-race-validator-release
revocation_race_validation_marker=revocation-race-validator-entered-clone
mkdir -p "$revocation_race_runs" "$revocation_race_evidence" \
  "$revocation_race_identity" "$revocation_race_stall"
touch "$revocation_race_launch_hold"
env PATH="$launch_abort_bin:$PATH" TMPDIR="$revocation_race_runs" \
  BATTERY_FIXTURE_GOCACHE="$tmp/fixture-gocache" BATTERY_FAKE_ARM_NOOP=1 \
  BATTERY_FIXTURE_SEAMS=1 \
  BATTERY_LAUNCH_HOLD_FILE="$revocation_race_launch_hold" \
  BATTERY_VALIDATOR_PUBLICATION_STALL_DIR="$revocation_race_stall" \
  BATTERY_VALIDATOR_PGID_STAGE="$revocation_race_stage" \
  BATTERY_VALIDATOR_PGID_FILE="$revocation_race_final" \
  BATTERY_VALIDATOR_DELAY_READY=1 BATTERY_READY="$revocation_race_ready" \
  BATTERY_RELEASE="$revocation_race_release" BATTERY_CLONE_PATH="$revocation_race_clone_path" \
  BATTERY_VALIDATOR_PID_PATH="$revocation_race_validator_pid_path" \
  BATTERY_VALIDATOR_CHILD_PID_PATH="$revocation_race_child_pid_path" \
  BATTERY_VALIDATION_LOG_MARKER="$revocation_race_validation_marker" \
  "$repo/metasystem/scripts/agents/milestone-battery.sh" --subject "$subject" \
    --evidence-root "$revocation_race_evidence" \
    >"$tmp/revocation-race.out" 2>"$tmp/revocation-race.err" &
revocation_race_controller=$!
for _ in $(seq 1 2000); do
  [[ -s "$revocation_race_stall/launch.pid" && -e "$revocation_race_stall/waiting" ]] && break
  [[ -e "$revocation_race_clone_path" ]] && break
  kill -0 "$revocation_race_controller" 2>/dev/null || break
  sleep 0.01
done
if [[ ! -s "$revocation_race_stall/launch.pid" || ! -e "$revocation_race_stall/waiting" ]]; then
  kill -TERM "$revocation_race_controller" 2>/dev/null || true
  rm -f -- "$revocation_race_launch_hold"
  wait "$revocation_race_controller" 2>/dev/null || true
  echo "gate-run-freeze fixture: validator wrapper did not stall before publication" >&2
  cat "$tmp/revocation-race.err" >&2
  exit 1
fi
revocation_race_launch_pid=$(cat "$revocation_race_stall/launch.pid")
kill -0 "$revocation_race_launch_pid" 2>/dev/null
[[ -e "$revocation_race_launch_hold" && ! -e "$revocation_race_stage" \
   && ! -e "$revocation_race_final" && ! -e "$revocation_race_clone_path" ]]
kill -TERM "$revocation_race_controller"
revocation_race_revoked=
for _ in $(seq 1 1200); do
  revocation_race_revoked=$(revoked_directory revocation-race-identity)
  [[ ! -e "$revocation_race_identity" && -n "$revocation_race_revoked" ]] && break
  kill -0 "$revocation_race_controller" 2>/dev/null || break
  sleep 0.01
done
[[ ! -e "$revocation_race_identity" && -n "$revocation_race_revoked" \
   && -d "$revocation_race_revoked" ]] \
  || {
    touch "$revocation_race_stall/release"
    rm -f -- "$revocation_race_launch_hold"
    kill -TERM "$revocation_race_controller" 2>/dev/null || true
    wait "$revocation_race_controller" 2>/dev/null || true
    echo "gate-run-freeze fixture: teardown did not revoke the configured publication directory" >&2
    cat "$tmp/revocation-race.err" >&2
    exit 1
  }
touch "$revocation_race_stall/release"
rm -f -- "$revocation_race_launch_hold"
set +e
wait "$revocation_race_controller"
revocation_race_rc=$?
set -e
[[ $revocation_race_rc == 130 ]]
for _ in $(seq 1 400); do
  kill -0 "$revocation_race_launch_pid" 2>/dev/null || break
  sleep 0.01
done
! kill -0 "$revocation_race_launch_pid" 2>/dev/null
[[ -e "$revocation_race_stall/publication-attempted" \
   && -e "$revocation_race_stall/exit-observed" \
   && ! -e "$revocation_race_stall/signaled" \
   && ! -e "$revocation_race_revoked/.validator-pgid.stage" \
   && ! -e "$revocation_race_revoked/validator.pgid" \
   && ! -e "$revocation_race_clone_path" \
   && ! -e "$revocation_race_validator_pid_path" \
   && ! -e "$revocation_race_child_pid_path" \
   && ! -e "$revocation_race_ready" ]]
revocation_race_envelope=$(sed -n 's/.* envelope=\(.*\)$/\1/p' \
  "$tmp/revocation-race.out" "$tmp/revocation-race.err" | tail -1)
[[ -f "$revocation_race_envelope/report.json" \
   && -f "$revocation_race_envelope/abandoned.json" \
   && -f "$revocation_race_envelope/outcome.json" \
   && -f "$revocation_race_envelope/teardown.json" \
   && ! -e "$revocation_race_envelope/reset.json" ]]
grep -Fq 'No such file or directory' "$revocation_race_envelope/validation.log"
[[ "$(find "$revocation_race_envelope/teardown.json" \
  -newer "$revocation_race_stall/exit-observed" -print)" \
  == "$revocation_race_envelope/teardown.json" ]] \
  || { echo "gate-run-freeze fixture: wrapper exit was not observed before teardown completion" >&2; exit 1; }
! grep -Fq "$revocation_race_validation_marker" "$revocation_race_envelope/validation.log"
grep -Fq 'clone=removed' "$tmp/revocation-race.err"
if [[ "${BATTERY_REVOCATION_RACE_FIXTURE_ONLY:-0}" == 1 ]]; then
  echo "gate-run-freeze revocation-race fixture passed"
  exit 0
fi

# Once the controller has recorded the validator, teardown must apply the same
# identity gate to that active path before TERM and again before group KILL.
recorded_abort_runs=$tmp/recorded-abort-runs
recorded_abort_evidence=$tmp/recorded-abort-evidence
recorded_abort_ready=$tmp/recorded-abort-ready
recorded_abort_release=$tmp/recorded-abort-release
recorded_abort_clone_path=$tmp/recorded-abort-clone-path
recorded_abort_validator_pid_path=$tmp/recorded-abort-validator-pid
recorded_abort_child_pid_path=$tmp/recorded-abort-validator-child-pid
mkdir -p "$recorded_abort_runs" "$recorded_abort_evidence"
env PATH="$launch_abort_bin:$PATH" TMPDIR="$recorded_abort_runs" \
  BATTERY_FIXTURE_GOCACHE="$tmp/fixture-gocache" BATTERY_FAKE_ARM_NOOP=1 \
  BATTERY_VALIDATOR_DELAY_READY=1 BATTERY_VALIDATOR_CHILD_SURVIVES_TERM=1 \
  BATTERY_READY="$recorded_abort_ready" BATTERY_RELEASE="$recorded_abort_release" \
  BATTERY_CLONE_PATH="$recorded_abort_clone_path" \
  BATTERY_VALIDATOR_PID_PATH="$recorded_abort_validator_pid_path" \
  BATTERY_VALIDATOR_CHILD_PID_PATH="$recorded_abort_child_pid_path" \
  "$repo/metasystem/scripts/agents/milestone-battery.sh" --subject "$subject" \
    --evidence-root "$recorded_abort_evidence" \
    >"$tmp/recorded-abort.out" 2>"$tmp/recorded-abort.err" &
recorded_abort_controller=$!
recorded_abort_setup=
for _ in $(seq 1 2000); do
  recorded_abort_setup=$(find "$recorded_abort_runs" -name setup.log -type f -print -quit)
  if [[ -n "$recorded_abort_setup" ]] \
    && grep -q '^validatorPid=' "$recorded_abort_setup" 2>/dev/null; then
    break
  fi
  kill -0 "$recorded_abort_controller" 2>/dev/null || break
  sleep 0.01
done
[[ -n "$recorded_abort_setup" ]] \
  && grep -q '^validatorPid=' "$recorded_abort_setup" \
  || { echo "gate-run-freeze fixture: active validator identity was not recorded" >&2; cat "$tmp/recorded-abort.err" >&2; exit 1; }
recorded_abort_validator=$(cat "$recorded_abort_validator_pid_path")
recorded_abort_child=$(cat "$recorded_abort_child_pid_path")
kill -0 "$recorded_abort_validator" 2>/dev/null
kill -0 "$recorded_abort_child" 2>/dev/null
[[ ! -e "$recorded_abort_ready" ]]
kill -TERM "$recorded_abort_controller"
set +e
wait "$recorded_abort_controller"
recorded_abort_rc=$?
set -e
[[ $recorded_abort_rc == 130 ]]
! kill -0 "$recorded_abort_validator" 2>/dev/null
! kill -0 "$recorded_abort_child" 2>/dev/null
recorded_abort_envelope=$(sed -n 's/.* envelope=\(.*\)$/\1/p' \
  "$tmp/recorded-abort.out" "$tmp/recorded-abort.err" | tail -1)
[[ -f "$recorded_abort_envelope/report.json" \
   && -f "$recorded_abort_envelope/abandoned.json" \
   && -f "$recorded_abort_envelope/outcome.json" \
   && -f "$recorded_abort_envelope/teardown.json" \
   && ! -e "$recorded_abort_envelope/reset.json" ]]
if [[ "${BATTERY_RECORDED_ABORT_FIXTURE_ONLY:-0}" == 1 ]]; then
  echo "gate-run-freeze recorded-abort fixture passed"
  exit 0
fi

# Launch-control seams from an inherited environment stay inert unless their
# master gate is explicit, including across the public controller's exec.
seam_hygiene_runs=$tmp/seam-hygiene-runs
seam_hygiene_evidence=$tmp/seam-hygiene-evidence
seam_hygiene_hold=$tmp/seam-hygiene-hold
seam_hygiene_stage=$tmp/seam-hygiene.stage
seam_hygiene_publication=$tmp/seam-hygiene.json
seam_hygiene_locator=$tmp/seam-hygiene-launch-locator
seam_hygiene_stall=$tmp/seam-hygiene-stall
seam_hygiene_observation=$tmp/seam-hygiene-observation
mkdir -p "$seam_hygiene_runs" "$seam_hygiene_evidence" "$seam_hygiene_stall"
touch "$seam_hygiene_hold"
printf 'operator-stage\n' >"$seam_hygiene_stage"
printf 'operator-publication\n' >"$seam_hygiene_publication"
printf 'operator-locator\n' >"$seam_hygiene_locator"
env PATH="$launch_abort_bin:$PATH" TMPDIR="$seam_hygiene_runs" \
  BATTERY_FIXTURE_GOCACHE="$tmp/fixture-gocache" BATTERY_FAKE_ARM_NOOP=1 \
  BATTERY_LAUNCH_HOLD_FILE="$seam_hygiene_hold" \
  BATTERY_VALIDATOR_PGID_STAGE="$seam_hygiene_stage" \
  BATTERY_VALIDATOR_PGID_FILE="$seam_hygiene_publication" \
  BATTERY_VALIDATOR_LAUNCH_PID_FILE="$seam_hygiene_locator" \
  BATTERY_VALIDATOR_PUBLICATION_STALL_DIR="$seam_hygiene_stall" \
  BATTERY_SEAM_OBSERVATION="$seam_hygiene_observation" \
  "$repo/metasystem/scripts/agents/milestone-battery.sh" --subject "$subject" \
    --evidence-root "$seam_hygiene_evidence" \
    >"$tmp/seam-hygiene.out" 2>"$tmp/seam-hygiene.err" &
seam_hygiene_controller=$!
for _ in $(seq 1 800); do
  kill -0 "$seam_hygiene_controller" 2>/dev/null || break
  sleep 0.01
done
if kill -0 "$seam_hygiene_controller" 2>/dev/null; then
  kill -TERM "$seam_hygiene_controller" 2>/dev/null || true
  rm -f -- "$seam_hygiene_hold"
  wait "$seam_hygiene_controller" 2>/dev/null || true
  echo "gate-run-freeze fixture: inherited launch-control seam delayed the controller" >&2
  exit 1
fi
set +e
wait "$seam_hygiene_controller"
seam_hygiene_rc=$?
set -e
[[ $seam_hygiene_rc != 0 ]]
[[ "$(cat "$seam_hygiene_observation")" == $'hold=<unset>\npublication=<unset>\nstage=<unset>\nlocator=<unset>\nstall=<unset>' ]]
[[ "$(cat "$seam_hygiene_stage")" == operator-stage ]]
[[ "$(cat "$seam_hygiene_publication")" == operator-publication ]]
[[ "$(cat "$seam_hygiene_locator")" == operator-locator ]]
seam_hygiene_envelope=$(sed -n 's/.* envelope=\(.*\)$/\1/p' \
  "$tmp/seam-hygiene.out" "$tmp/seam-hygiene.err" | tail -1)
[[ -f "$seam_hygiene_envelope/report.json" \
   && -f "$seam_hygiene_envelope/abandoned.json" \
   && -f "$seam_hygiene_envelope/outcome.json" \
   && -f "$seam_hygiene_envelope/teardown.json" ]]
rm -f -- "$seam_hygiene_hold"
if [[ "${BATTERY_SEAM_HYGIENE_FIXTURE_ONLY:-0}" == 1 ]]; then
  echo "gate-run-freeze seam-hygiene fixture passed"
  exit 0
fi

# A stale publication is the exact shape of PID reuse: its numeric locator can
# name a live bystander while its start identity names the departed validator.
# This leg fails any implementation that signals the numeric PID without
# verifying the published identity immediately before the signal.
recycled_pid_runs=$tmp/recycled-pid-runs
recycled_pid_evidence=$tmp/recycled-pid-evidence
recycled_pid_hold=$tmp/recycled-pid-hold
recycled_pid_identity_dir=$tmp/recycled-pid-identity
recycled_pid_stage=$recycled_pid_identity_dir/.validator-pgid.stage
recycled_pid_publication=$recycled_pid_identity_dir/validator.pgid
recycled_pid_locator=$tmp/recycled-pid-launch-locator
recycled_pid_signal=$tmp/recycled-pid-sentinel-signaled
recycled_pid_fabricated=$tmp/recycled-pid.fabricated
mkdir -p "$recycled_pid_runs" "$recycled_pid_evidence" "$recycled_pid_identity_dir"
touch "$recycled_pid_hold"
(
  trap 'touch "$recycled_pid_signal"; exit 90' TERM INT HUP
  while :; do sleep 1; done
) &
recycled_pid_sentinel=$!
recycled_pid_identity=$("$real_engine" proc probe --pid "$recycled_pid_sentinel")
recycled_pid_started=$("$real_engine" json get --value "$recycled_pid_identity" --field startedAtUnix)
recycled_pid_ticks=$("$real_engine" json get --value "$recycled_pid_identity" --field startTicks)
recycled_pid_boot=$("$real_engine" json get --value "$recycled_pid_identity" --field bootId)
printf '%s\n' "$recycled_pid_sentinel" >"$recycled_pid_locator"
env PATH="$launch_abort_bin:$PATH" TMPDIR="$recycled_pid_runs" \
  BATTERY_FIXTURE_GOCACHE="$tmp/fixture-gocache" BATTERY_FAKE_ARM_NOOP=1 \
  BATTERY_FIXTURE_SEAMS=1 BATTERY_LAUNCH_HOLD_FILE="$recycled_pid_hold" \
  BATTERY_VALIDATOR_PGID_STAGE="$recycled_pid_stage" \
  BATTERY_VALIDATOR_PGID_FILE="$recycled_pid_publication" \
  BATTERY_VALIDATOR_LAUNCH_PID_FILE="$recycled_pid_locator" \
  "$repo/metasystem/scripts/agents/milestone-battery.sh" --subject "$subject" \
    --evidence-root "$recycled_pid_evidence" \
    >"$tmp/recycled-pid.out" 2>"$tmp/recycled-pid.err" &
recycled_pid_controller=$!
for _ in $(seq 1 2000); do
  [[ -s "$recycled_pid_publication" ]] && break
  kill -0 "$recycled_pid_controller" 2>/dev/null || break
  sleep 0.01
done
[[ -s "$recycled_pid_publication" ]] \
  || { echo "gate-run-freeze fixture: validator identity was not published for PID-reuse proof" >&2; cat "$tmp/recycled-pid.err" >&2; exit 1; }
recycled_pid_departed=$("$real_engine" json get --file "$recycled_pid_publication" --field pid)
[[ "$recycled_pid_departed" != "$recycled_pid_sentinel" ]]
for _ in $(seq 1 400); do
  kill -0 "$recycled_pid_departed" 2>/dev/null || break
  sleep 0.01
done
! kill -0 "$recycled_pid_departed" 2>/dev/null
recycled_pid_argv=$("$real_engine" json get --value "$recycled_pid_identity" --field argv)
recycled_pid_boot=$("$real_engine" json get --value "$recycled_pid_identity" --field bootId)
recycled_pid_liveness=$("$real_engine" json get --value "$recycled_pid_identity" --field liveness)
recycled_pid_probe_pid=$("$real_engine" json get --value "$recycled_pid_identity" --field pid)
recycled_pid_stamp=$("$real_engine" json get --value "$recycled_pid_identity" --field startedAt)
recycled_pid_started_micro=$("$real_engine" json get --value "$recycled_pid_identity" --field startedAtUnixMicro)
[[ "$recycled_pid_probe_pid" == "$recycled_pid_sentinel" && "$recycled_pid_liveness" == alive ]]
recycled_pid_fabricated_started=$((recycled_pid_started + 1))
recycled_pid_fabricated_micro=$((recycled_pid_started_micro + 1000000))
recycled_pid_fabricated_ticks=$recycled_pid_ticks
if (( recycled_pid_fabricated_ticks > 0 )); then
  recycled_pid_fabricated_ticks=$((recycled_pid_fabricated_ticks + 1))
fi
if recycled_pid_date=$(date -r "$recycled_pid_fabricated_started" '+%Y-%m-%dT%H:%M:%S' 2>/dev/null) \
  && recycled_pid_zone=$(date -r "$recycled_pid_fabricated_started" '+%z' 2>/dev/null); then
  :
elif recycled_pid_date=$(date -d "@$recycled_pid_fabricated_started" '+%Y-%m-%dT%H:%M:%S' 2>/dev/null) \
  && recycled_pid_zone=$(date -d "@$recycled_pid_fabricated_started" '+%z' 2>/dev/null); then
  :
else
  echo "gate-run-freeze fixture: could not format fabricated process identity timestamp" >&2
  exit 1
fi
if [[ "$recycled_pid_stamp" == *Z ]]; then
  printf -v recycled_pid_fabricated_stamp '%s.%06dZ' \
    "$recycled_pid_date" "$((recycled_pid_fabricated_micro % 1000000))"
else
  printf -v recycled_pid_fabricated_stamp '%s.%06d%s:%s' \
    "$recycled_pid_date" "$((recycled_pid_fabricated_micro % 1000000))" \
    "${recycled_pid_zone%??}" "${recycled_pid_zone#???}"
fi
printf '{"argv":%s,"bootId":"%s","liveness":"%s","pid":%s,"startTicks":%s,"startedAt":"%s","startedAtUnix":%s,"startedAtUnixMicro":%s}\n' \
  "$recycled_pid_argv" "$recycled_pid_boot" "$recycled_pid_liveness" \
  "$recycled_pid_probe_pid" "$recycled_pid_fabricated_ticks" \
  "$recycled_pid_fabricated_stamp" "$recycled_pid_fabricated_started" \
  "$recycled_pid_fabricated_micro" >"$recycled_pid_fabricated"
mv "$recycled_pid_fabricated" "$recycled_pid_publication"
[[ "$("$real_engine" json get --file "$recycled_pid_publication" --field pid)" == "$recycled_pid_sentinel" ]]
[[ "$("$real_engine" json get --file "$recycled_pid_publication" --field startedAtUnix)" != "$recycled_pid_started" ]]
kill -TERM "$recycled_pid_controller"
rm -f -- "$recycled_pid_hold"
set +e
wait "$recycled_pid_controller"
recycled_pid_rc=$?
set -e
[[ $recycled_pid_rc == 130 ]]
recycled_pair_args=()
if (( recycled_pid_ticks > 0 )); then
  recycled_pair_args=(--start-ticks "$recycled_pid_ticks" --boot-id "$recycled_pid_boot")
fi
"$real_engine" proc alive --pid "$recycled_pid_sentinel" \
  --start-time "$recycled_pid_started" \
  "${recycled_pair_args[@]+"${recycled_pair_args[@]}"}" --root "$repo/metasystem"
[[ ! -e "$recycled_pid_signal" ]]
recycled_pid_envelope=$(sed -n 's/.* envelope=\(.*\)$/\1/p' \
  "$tmp/recycled-pid.out" "$tmp/recycled-pid.err" | tail -1)
[[ -f "$recycled_pid_envelope/report.json" \
   && -f "$recycled_pid_envelope/abandoned.json" \
   && -f "$recycled_pid_envelope/outcome.json" \
   && -f "$recycled_pid_envelope/teardown.json" \
   && ! -e "$recycled_pid_envelope/reset.json" ]]
recycled_pid_revoked=$(revoked_directory recycled-pid-identity)
[[ ! -e "$recycled_pid_identity_dir" && -d "$recycled_pid_revoked" ]]
grep -Fq 'clone=removed' "$tmp/recycled-pid.err"
kill -KILL "$recycled_pid_sentinel" 2>/dev/null || true
wait "$recycled_pid_sentinel" 2>/dev/null || true
recycled_pid_sentinel=
if [[ "${BATTERY_RECYCLED_PID_FIXTURE_ONLY:-0}" == 1 ]]; then
  echo "gate-run-freeze recycled-pid fixture passed"
  exit 0
fi

# Toolchain discovery fails before the run directory exists, but after the
# durable publication path owns all later bootstrap failures.
toolchain_fail_bin=$tmp/toolchain-fail-bin
toolchain_fail_evidence=$tmp/toolchain-fail-evidence
mkdir -p "$toolchain_fail_bin" "$toolchain_fail_evidence"
cat >"$toolchain_fail_bin/go" <<'GO'
#!/usr/bin/env bash
echo 'fixture GOCACHE lookup failed' >&2
exit 86
GO
chmod +x "$toolchain_fail_bin/go"
set +e
PATH="$toolchain_fail_bin:$PATH" \
  "$repo/metasystem/scripts/agents/milestone-battery.sh" --subject "$subject" --evidence-root "$toolchain_fail_evidence" \
  >"$tmp/toolchain-fail.out" 2>"$tmp/toolchain-fail.err"
toolchain_fail_rc=$?
set -e
[[ $toolchain_fail_rc != 0 ]]
toolchain_fail_envelope=$(sed -n 's/.* envelope=\(.*\)$/\1/p' "$tmp/toolchain-fail.out" "$tmp/toolchain-fail.err" | tail -1)
[[ -n "$toolchain_fail_envelope" && -f "$toolchain_fail_envelope/report.json" \
   && -f "$toolchain_fail_envelope/outcome.json" && -f "$toolchain_fail_envelope/teardown.json" \
   && -f "$toolchain_fail_envelope/clone.log" && ! -s "$toolchain_fail_envelope/clone.log" ]]
grep -Fq 'fixture GOCACHE lookup failed' "$tmp/toolchain-fail.err"
grep -Fq '"result":"run-directory-not-created"' "$toolchain_fail_envelope/teardown.json"

# Temporary-directory creation has the same evidence owner. The Go stub keeps
# this leg independent of the host toolchain; mktemp fails before clone setup.
preclone_fail_bin=$tmp/preclone-fail-bin
preclone_fail_evidence=$tmp/preclone-fail-evidence
mkdir -p "$preclone_fail_bin" "$preclone_fail_evidence"
cat >"$preclone_fail_bin/go" <<'GO'
#!/usr/bin/env bash
set -euo pipefail
[[ "$*" == 'env GOCACHE' ]] || exit 86
printf '%s\n' "$BATTERY_FIXTURE_GOCACHE"
GO
cat >"$preclone_fail_bin/mktemp" <<'MKTEMP'
#!/usr/bin/env bash
echo 'fixture run-directory creation failed' >&2
exit 87
MKTEMP
chmod +x "$preclone_fail_bin/go" "$preclone_fail_bin/mktemp"
set +e
PATH="$preclone_fail_bin:$PATH" BATTERY_FIXTURE_GOCACHE="$tmp/fixture-gocache" \
  "$repo/metasystem/scripts/agents/milestone-battery.sh" --subject "$subject" --evidence-root "$preclone_fail_evidence" \
  >"$tmp/preclone-fail.out" 2>"$tmp/preclone-fail.err"
preclone_fail_rc=$?
set -e
[[ $preclone_fail_rc != 0 ]]
preclone_fail_envelope=$(sed -n 's/.* envelope=\(.*\)$/\1/p' "$tmp/preclone-fail.out" "$tmp/preclone-fail.err" | tail -1)
[[ -n "$preclone_fail_envelope" && -f "$preclone_fail_envelope/report.json" \
   && -f "$preclone_fail_envelope/outcome.json" && -f "$preclone_fail_envelope/teardown.json" \
   && -f "$preclone_fail_envelope/clone.log" && ! -s "$preclone_fail_envelope/clone.log" ]]
grep -Fq 'fixture run-directory creation failed' "$tmp/preclone-fail.err"
grep -Fq '"result":"run-directory-not-created"' "$preclone_fail_envelope/teardown.json"
[[ "$("$real_engine" json get --file "$preclone_fail_envelope/report.json" --field subjectSHA)" == "$subject" ]]
[[ "$("$real_engine" json get --file "$preclone_fail_envelope/report.json" --field setupExit)" != 0 ]]
[[ "$("$real_engine" json get --file "$preclone_fail_envelope/report.json" --field validationExit)" == -1 ]]
[[ "$("$real_engine" json get --file "$preclone_fail_envelope/report.json" --field verdict)" == bootstrap-failed ]]

# Clone failures publish the same run-scoped core evidence before the temporary
# run directory is removed.
real_git=$(command -v git)
clone_fail_bin=$tmp/clone-fail-bin
clone_fail_evidence=$tmp/clone-fail-evidence
mkdir -p "$clone_fail_bin" "$clone_fail_evidence"
cat >"$clone_fail_bin/git" <<'GIT'
#!/usr/bin/env bash
set -euo pipefail
if [[ ${1:-} == clone ]]; then
  echo 'fixture clone creation failed' >&2
  exit 88
fi
exec "$BATTERY_REAL_GIT" "$@"
GIT
chmod +x "$clone_fail_bin/git"
set +e
PATH="$clone_fail_bin:$PATH" BATTERY_REAL_GIT="$real_git" \
  "$repo/metasystem/scripts/agents/milestone-battery.sh" --subject "$subject" --evidence-root "$clone_fail_evidence" \
  >"$tmp/clone-fail.out" 2>"$tmp/clone-fail.err"
clone_fail_rc=$?
set -e
[[ $clone_fail_rc != 0 ]]
clone_fail_envelope=$(sed -n 's/.* envelope=\(.*\)$/\1/p' "$tmp/clone-fail.out" "$tmp/clone-fail.err" | tail -1)
[[ -n "$clone_fail_envelope" && -f "$clone_fail_envelope/report.json" \
   && -f "$clone_fail_envelope/outcome.json" && -f "$clone_fail_envelope/teardown.json" \
   && -f "$clone_fail_envelope/clone.log" ]]
[[ "$("$real_engine" json get --file "$clone_fail_envelope/report.json" --field subjectSHA)" == "$subject" ]]
[[ "$("$real_engine" json get --file "$clone_fail_envelope/report.json" --field setupExit)" != 0 ]]
[[ "$("$real_engine" json get --file "$clone_fail_envelope/report.json" --field validationExit)" == -1 ]]
[[ "$("$real_engine" json get --file "$clone_fail_envelope/report.json" --field verdict)" == bootstrap-failed ]]
grep -Fq 'fixture clone creation failed' "$clone_fail_envelope/clone.log"
if [[ "${BATTERY_BOOTSTRAP_FIXTURE_ONLY:-0}" == 1 ]]; then
  echo "gate-run-freeze bootstrap fixture passed"
  exit 0
fi

# A live wrapper with no publication after revocation is not safe to signal or
# dismiss. The bounded wait must retain its clone and record the publication
# revocation failure while the wrapper remains stalled outside the clone.
publication_retention_runs=$tmp/publication-retention-runs
publication_retention_evidence=$tmp/publication-retention-evidence
publication_retention_identity=$tmp/publication-retention-identity
publication_retention_stall=$tmp/publication-retention-stall
publication_retention_launch_hold=$tmp/publication-retention-launch-hold
publication_retention_stage=$publication_retention_identity/.validator-pgid.stage
publication_retention_final=$publication_retention_identity/validator.pgid
publication_retention_clone_path=$tmp/publication-retention-clone-path
publication_retention_validator_pid_path=$tmp/publication-retention-validator-pid
publication_retention_child_pid_path=$tmp/publication-retention-validator-child-pid
publication_retention_ready=$tmp/publication-retention-ready
publication_retention_release=$tmp/publication-retention-validator-release
mkdir -p "$publication_retention_runs" "$publication_retention_evidence" \
  "$publication_retention_identity" "$publication_retention_stall"
touch "$publication_retention_launch_hold"
env PATH="$launch_abort_bin:$PATH" TMPDIR="$publication_retention_runs" \
  BATTERY_FIXTURE_GOCACHE="$tmp/fixture-gocache" BATTERY_FAKE_ARM_NOOP=1 \
  BATTERY_FIXTURE_SEAMS=1 \
  BATTERY_LAUNCH_HOLD_FILE="$publication_retention_launch_hold" \
  BATTERY_VALIDATOR_PUBLICATION_STALL_DIR="$publication_retention_stall" \
  BATTERY_VALIDATOR_PGID_STAGE="$publication_retention_stage" \
  BATTERY_VALIDATOR_PGID_FILE="$publication_retention_final" \
  BATTERY_VALIDATOR_DELAY_READY=1 BATTERY_READY="$publication_retention_ready" \
  BATTERY_RELEASE="$publication_retention_release" \
  BATTERY_CLONE_PATH="$publication_retention_clone_path" \
  BATTERY_VALIDATOR_PID_PATH="$publication_retention_validator_pid_path" \
  BATTERY_VALIDATOR_CHILD_PID_PATH="$publication_retention_child_pid_path" \
  "$repo/metasystem/scripts/agents/milestone-battery.sh" --subject "$subject" \
    --evidence-root "$publication_retention_evidence" \
    >"$tmp/publication-retention.out" 2>"$tmp/publication-retention.err" &
publication_retention_controller=$!
for _ in $(seq 1 2000); do
  [[ -s "$publication_retention_stall/launch.pid" \
    && -e "$publication_retention_stall/waiting" ]] && break
  kill -0 "$publication_retention_controller" 2>/dev/null || break
  sleep 0.01
done
if [[ ! -s "$publication_retention_stall/launch.pid" \
  || ! -e "$publication_retention_stall/waiting" ]]; then
  kill -TERM "$publication_retention_controller" 2>/dev/null || true
  touch "$publication_retention_stall/release"
  rm -f -- "$publication_retention_launch_hold"
  wait "$publication_retention_controller" 2>/dev/null || true
  echo "gate-run-freeze fixture: validator wrapper did not stall for retention" >&2
  cat "$tmp/publication-retention.err" >&2
  exit 1
fi
publication_retention_launch_pid=$(cat "$publication_retention_stall/launch.pid")
kill -0 "$publication_retention_launch_pid" 2>/dev/null
[[ -e "$publication_retention_launch_hold" \
   && ! -e "$publication_retention_stall/publication-attempted" \
   && ! -e "$publication_retention_stall/signaled" \
   && ! -e "$publication_retention_stall/exit-observed" \
   && ! -e "$publication_retention_stage" \
   && ! -e "$publication_retention_final" \
   && ! -e "$publication_retention_clone_path" ]]
kill -TERM "$publication_retention_controller"
publication_retention_revoked=
for _ in $(seq 1 1200); do
  publication_retention_revoked=$(revoked_directory publication-retention-identity)
  [[ ! -e "$publication_retention_identity" \
    && -n "$publication_retention_revoked" ]] && break
  kill -0 "$publication_retention_controller" 2>/dev/null || break
  sleep 0.01
done
[[ ! -e "$publication_retention_identity" \
   && -n "$publication_retention_revoked" \
   && -d "$publication_retention_revoked" ]] \
  || {
    touch "$publication_retention_stall/release"
    rm -f -- "$publication_retention_launch_hold"
    kill -TERM "$publication_retention_controller" 2>/dev/null || true
    wait "$publication_retention_controller" 2>/dev/null || true
    echo "gate-run-freeze fixture: retention path did not revoke the publication directory" >&2
    cat "$tmp/publication-retention.err" >&2
    exit 1
  }
set +e
wait "$publication_retention_controller"
publication_retention_rc=$?
set -e
[[ $publication_retention_rc == 130 ]]
publication_retention_envelope=$(sed -n 's/.* envelope=\(.*\)$/\1/p' \
  "$tmp/publication-retention.out" "$tmp/publication-retention.err" | tail -1)
[[ -f "$publication_retention_envelope/report.json" \
   && -f "$publication_retention_envelope/abandoned.json" \
   && -f "$publication_retention_envelope/outcome.json" \
   && -f "$publication_retention_envelope/teardown.json" \
   && ! -e "$publication_retention_envelope/reset.json" ]]
publication_retention_result=$("$real_engine" json get \
  --file "$publication_retention_envelope/teardown.json" --field result) \
  || { echo "gate-run-freeze fixture: publication-retention teardown omitted its result" >&2; cat "$publication_retention_envelope/teardown.json" >&2; cat "$tmp/publication-retention.err" >&2; exit 1; }
publication_retention_retained=$("$real_engine" json get \
  --file "$publication_retention_envelope/teardown.json" --field retainedPath) \
  || { echo "gate-run-freeze fixture: publication-retention teardown omitted its retained path" >&2; cat "$publication_retention_envelope/teardown.json" >&2; cat "$tmp/publication-retention.err" >&2; exit 1; }
[[ "$publication_retention_result" == validator-publication-revocation-failed \
   && "$publication_retention_retained" == "$publication_retention_runs"/*/subject \
   && -d "$publication_retention_retained" ]]
kill -0 "$publication_retention_launch_pid" 2>/dev/null
[[ ! -e "$publication_retention_stall/publication-attempted" \
   && ! -e "$publication_retention_stall/signaled" \
   && ! -e "$publication_retention_stall/exit-observed" \
   && ! -e "$publication_retention_revoked/.validator-pgid.stage" \
   && ! -e "$publication_retention_revoked/validator.pgid" \
   && ! -e "$publication_retention_clone_path" \
   && ! -e "$publication_retention_validator_pid_path" \
   && ! -e "$publication_retention_child_pid_path" \
   && ! -e "$publication_retention_ready" ]]
grep -Fq 'retained clone' "$tmp/publication-retention.err"
touch "$publication_retention_stall/release"
rm -f -- "$publication_retention_launch_hold"
for _ in $(seq 1 400); do
  kill -0 "$publication_retention_launch_pid" 2>/dev/null || break
  sleep 0.01
done
! kill -0 "$publication_retention_launch_pid" 2>/dev/null
[[ -e "$publication_retention_stall/publication-attempted" \
   && ! -e "$publication_retention_stall/signaled" ]]
publication_retention_launch_pid=
rm -rf -- "${publication_retention_retained%/subject}"
if [[ "${BATTERY_PUBLICATION_RETENTION_FIXTURE_ONLY:-0}" == 1 ]]; then
  echo "gate-run-freeze publication-retention fixture passed"
  exit 0
fi

printf 'never copied\n' >"$repo/uncommitted.txt"
printf 'never copied\n' >"$repo/ignored.secret"
printf 'LIVE-SECRET\n' >"$repo/metasystem/metasystem.conf.local"
printf 'live ledger mutation\n' >>"$repo/metasystem/plans/goals.md"
printf 'live receipt mutation\n' >>"$repo/metasystem/memory/receipts.log"
mkdir -p "$repo/metasystem/artifacts/agents/live-only"
printf 'live artifact\n' >"$repo/metasystem/artifacts/agents/live-only/state"
live_binary_before=$(shasum -a 256 "$repo/metasystem/bin/metasystem" | awk '{print $1}')
inventory_before=$(git -C "$repo" worktree list --porcelain | sed -n 's/^worktree //p')
ready=$tmp/ready release=$tmp/release clone_path=$tmp/clone-path registry_path=$tmp/registry-path
live_registry=$tmp/live-registry/.metasystem/armed-checkouts.jsonl
teardown_call=$tmp/teardown-call
owner_pid_path=$tmp/owner-pid
validator_pid_path=$tmp/validator-pid
validator_child_pid_path=$tmp/validator-child-pid
export BATTERY_EXPECT_HOME="${HOME:-}"
export BATTERY_LIVE_ROOT=$repo
export BATTERY_LIVE_REGISTRY=$live_registry
export BATTERY_TEARDOWN_CALL=$teardown_call
export BATTERY_VALIDATOR_PID_PATH=$validator_pid_path
export BATTERY_VALIDATOR_CHILD_PID_PATH=$validator_child_pid_path
mkdir -p "${live_registry%/*}"
inventory_hash=$({ printf '%s\n' "$inventory_before"; } | { shasum -a 256 2>/dev/null || sha256sum; } | awk '{print $1}')
enumeration_log=$tmp/mission-wall-inventory-hashes
enumeration_stop=$tmp/mission-wall-stop
(
  while [[ ! -e "$enumeration_stop" ]]; do
    git -C "$repo" worktree list --porcelain | sed -n 's/^worktree //p' \
      | { shasum -a 256 2>/dev/null || sha256sum; } | awk '{print $1}' \
      >>"$enumeration_log"
    sleep 0.01
  done
) &
enumeration_pid=$!

BATTERY_READY=$ready BATTERY_RELEASE=$release BATTERY_CLONE_PATH=$clone_path \
BATTERY_REGISTRY_PATH=$registry_path BATTERY_EXPECT_HOME="${HOME:-}" \
BATTERY_LIVE_ROOT=$repo BATTERY_LIVE_REGISTRY=$live_registry \
BATTERY_TEARDOWN_CALL=$teardown_call BATTERY_OWNER_PID_PATH=$owner_pid_path \
METASYSTEM_GATE_WITNESS=/borrowed/witness.json \
METASYSTEM_GATE_WITNESS_ROOT=/borrowed METASYSTEM_GATE_WITNESS_RUN=borrowed-run \
METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE=ENGINE \
METASYSTEM_GATE_WITNESS_WRITE=/borrowed/write.json \
METASYSTEM_GATE_WITNESS_CONTROLLER_PID=1 METASYSTEM_GATE_WITNESS_CONTROLLER_STARTED_AT=1 \
METASYSTEM_GATE_WITNESS_CONTROLLER_START_TICKS=1 METASYSTEM_GATE_WITNESS_CONTROLLER_BOOT_ID=borrowed \
  "$repo/metasystem/scripts/agents/milestone-battery.sh" --subject "$subject" \
    --evidence-root "$evidence" >"$tmp/first.out" 2>"$tmp/first.err" &
first_pid=$!
for _ in $(seq 1 2000); do [[ -e "$ready" ]] && break; kill -0 "$first_pid" 2>/dev/null || break; sleep 0.01; done
[[ -e "$ready" ]] || {
  echo "gate-run-freeze fixture: isolated validator never started" >&2
  cat "$tmp/first.err" >&2
  first_failed_envelope=$(sed -n 's/.* envelope=\(.*\)$/\1/p' "$tmp/first.out" "$tmp/first.err" | tail -1)
  first_failed_clone=$(sed -n 's/.* path=\([^ ]*\) envelope=.*/\1/p' "$tmp/first.out" "$tmp/first.err" | tail -1)
  [[ ! -f "$first_failed_envelope/setup.log" ]] || tail -100 "$first_failed_envelope/setup.log" >&2
  [[ ! -f "$first_failed_envelope/validation.log" ]] || tail -100 "$first_failed_envelope/validation.log" >&2
  [[ ! -f "${first_failed_clone%/subject}/setup.log" ]] || tail -100 "${first_failed_clone%/subject}/setup.log" >&2
  [[ ! -f "${first_failed_clone%/subject}/validation.log" ]] || tail -100 "${first_failed_clone%/subject}/validation.log" >&2
  exit 1
}

isolated_clone=$(cat "$clone_path")
[[ "$(git -C "$isolated_clone" rev-parse HEAD)" == "$subject" ]]
[[ -z "$(git -C "$isolated_clone" symbolic-ref -q HEAD || true)" ]]
[[ "$(git -C "$repo" worktree list --porcelain | sed -n 's/^worktree //p')" == "$inventory_before" ]]
[[ "$(cat "$registry_path")" == */supervision-home ]]

# Every named live operation remains executable while validation is paused.
printf 'live commit\n' >"$repo/live-commit.txt"
git -C "$repo" add live-commit.txt metasystem/plans/goals.md metasystem/memory/receipts.log
git -C "$repo" commit -qm live-commit
git -C "$repo" checkout -qb interference
printf 'checkout and rebase\n' >"$repo/interference.txt"
git -C "$repo" add interference.txt && git -C "$repo" commit -qm interference
git -C "$repo" rebase main >/dev/null
git -C "$repo" checkout -q main
git -C "$repo" push -q origin main
# This append is the observable mutation owned by a goal verb; because the
# battery detached before it, the subject retains only "subject ledger".
printf 'concurrent goal verb write\n' >>"$repo/metasystem/plans/goals.md"
printf 'LIVE-SECRET-EDITED\n' >"$repo/metasystem/metasystem.conf.local"
live_arm_call=$tmp/live-arm-call
live_registry_home=$tmp/live-registry
METASYSTEM_SUPERVISION_REGISTRY_HOME=$live_registry_home BATTERY_TEARDOWN_CALL=$live_arm_call \
  "$repo/metasystem/scripts/agents/arm-supervision.sh" --repo "$repo"
[[ -f "$repo/metasystem/artifacts/agents/supervision/lock.d/owner.json" ]]
live_census=$repo/metasystem/artifacts/agents/supervision/last-census.json
[[ "$("$real_engine" json get --file "$live_census" --field verdict)" == SUCCESS ]]
live_census_inventory=$("$real_engine" json get --file "$live_census" --field inventory)
while IFS= read -r live_census_row; do
  live_census_cwd=$("$real_engine" json get --value "$live_census_row" --field cwd)
  [[ "$live_census_cwd" != "$isolated_clone"* ]]
done < <(json_elements "$live_census_inventory")
grep -Fq "$repo/metasystem" "$live_registry"
! grep -Fq "$isolated_clone" "$live_registry"
METASYSTEM_SUPERVISION_REGISTRY_HOME=$live_registry_home BATTERY_TEARDOWN_CALL=$live_arm_call \
  "$repo/metasystem/scripts/agents/arm-supervision.sh" --repo "$repo" --shutdown
[[ ! -e "$repo/metasystem/artifacts/agents/supervision/lock.d" ]]

# A second entry reaches the shared checkpoint and refuses without disturbing
# the first runner.
set +e
BATTERY_EXPECT_HOME="${HOME:-}" "$repo/metasystem/scripts/agents/milestone-battery.sh" \
  --subject "$subject" --evidence-root "$evidence" >"$tmp/second.out" 2>"$tmp/second.err"
second_rc=$?
set -e
[[ $second_rc == 3 ]]
kill -0 "$first_pid"

touch "$release"
wait "$first_pid"
touch "$enumeration_stop"
wait "$enumeration_pid"
enumeration_pid=
[[ "$(sort -u "$enumeration_log")" == "$inventory_hash" ]]
grep -Fq "subject=$subject" "$tmp/first.out"
envelope=$(sed -n 's/.* envelope=\(.*\)$/\1/p' "$tmp/first.out")
[[ -f "$envelope/report.json" && -f "$envelope/outcome.json" \
   && -f "$envelope/reset.json" && -f "$envelope/teardown.json" \
   && -f "$envelope/run-class.txt" \
   && -f "$envelope/supervision-registry.jsonl" && -f "$envelope/last-census.json" ]]
for report_field in runId subjectSHA runClass surfaceProjection surfacePolicyVersion \
  surfaceDigest toolchainIdentity startedAt endedAt validationExit copyResult \
  copyDigestManifest verdict validationLog failureArtifacts; do
  "$real_engine" json get --file "$envelope/report.json" --field "$report_field" >/dev/null
done
[[ "$("$real_engine" json get --file "$envelope/report.json" --field subjectSHA)" == "$subject" ]]
[[ "$("$real_engine" json get --file "$envelope/report.json" --field surfaceProjection)" == LANDING ]]
[[ "$("$real_engine" json get --file "$envelope/report.json" --field validationExit)" == 0 ]]
[[ "$("$real_engine" json get --file "$envelope/report.json" --field copyResult)" == verified ]]
[[ "$(cat "$envelope/run-class.txt")" == FULL ]]
[[ "$("$real_engine" json get --file "$envelope/report.json" --field runClass)" == FULL ]]
[[ "$("$real_engine" json get --file "$envelope/outcome.json" --field runClass)" == FULL ]]
[[ "$("$real_engine" json get --file "$envelope/reset.json" --field runClass)" == FULL ]]
registry_row_count=0
registry_relaunch_found=0
while IFS= read -r registry_row; do
  [[ -n "$registry_row" ]] || continue
  registry_row_count=$((registry_row_count + 1))
  registry_event=$("$real_engine" json get --value "$registry_row" --field event --default '')
  registry_checkout=$("$real_engine" json get --value "$registry_row" --field checkoutPath --default '')
  if [[ "$registry_event" == relaunched \
    && "$registry_checkout" == "$isolated_clone/metasystem" ]]; then
    registry_relaunch_found=1
  fi
  [[ "$registry_checkout" != "$repo" ]]
done <"$envelope/supervision-registry.jsonl"
[[ $registry_row_count -gt 0 && $registry_relaunch_found == 1 ]]
[[ -s "$envelope/copy-digests.nul" ]]
grep -Fq -- "--repo $isolated_clone --shutdown" "$teardown_call"
[[ ! -e "$isolated_clone" ]]
first_owner_pid=$(cat "$owner_pid_path")
first_validator_pid=$(cat "$validator_pid_path")
first_validator_child_pid=$(cat "$validator_child_pid_path")
! kill -0 "$first_owner_pid" 2>/dev/null
! kill -0 "$first_validator_pid" 2>/dev/null
! kill -0 "$first_validator_child_pid" 2>/dev/null
[[ "$(git -C "$repo" worktree list --porcelain | sed -n 's/^worktree //p')" == "$inventory_before" ]]
[[ "$(shasum -a 256 "$repo/metasystem/bin/metasystem" | awk '{print $1}')" == "$live_binary_before" ]]

# The root classification, not a green exit alone, controls checkpoint
# consumption. This fixture seam makes the otherwise-complete root report
# imported proof and pins the non-subtracting terminal path.
printf '1\t0\tdocs/witness-assisted.md\0' | "$real_engine" gate weight-add \
  --root "$repo/metasystem" --commit witness-assisted >/dev/null
assisted_weight_before=$("$real_engine" json get \
  --file "$repo/metasystem/artifacts/agents/battery-weight.json" --field accumulated)
assisted_ready=$tmp/assisted-ready
assisted_release=$tmp/assisted-release
assisted_clone_path=$tmp/assisted-clone-path
assisted_registry_path=$tmp/assisted-registry-path
assisted_teardown_call=$tmp/assisted-teardown-call
assisted_owner_pid_path=$tmp/assisted-owner-pid
assisted_validator_pid_path=$tmp/assisted-validator-pid
assisted_validator_child_pid_path=$tmp/assisted-validator-child-pid
BATTERY_FAKE_RUN_CLASS=WITNESS-ASSISTED \
BATTERY_READY=$assisted_ready BATTERY_RELEASE=$assisted_release \
BATTERY_CLONE_PATH=$assisted_clone_path BATTERY_REGISTRY_PATH=$assisted_registry_path \
BATTERY_TEARDOWN_CALL=$assisted_teardown_call BATTERY_OWNER_PID_PATH=$assisted_owner_pid_path \
BATTERY_VALIDATOR_PID_PATH=$assisted_validator_pid_path \
BATTERY_VALIDATOR_CHILD_PID_PATH=$assisted_validator_child_pid_path \
  "$repo/metasystem/scripts/agents/milestone-battery.sh" --subject "$subject" \
    --evidence-root "$evidence" >"$tmp/assisted.out" 2>"$tmp/assisted.err" &
assisted_pid=$!
for _ in $(seq 1 2000); do
  [[ -e "$assisted_ready" ]] && break
  kill -0 "$assisted_pid" 2>/dev/null || break
  sleep 0.01
done
[[ -e "$assisted_ready" ]] \
  || { echo "gate-run-freeze fixture: witness-assisted validator never started" >&2; exit 1; }
touch "$assisted_release"
set +e
wait "$assisted_pid"
assisted_rc=$?
set -e
assisted_pid=
[[ $assisted_rc == 1 ]]
assisted_envelope=$(sed -n 's/.* envelope=\(.*\)$/\1/p' \
  "$tmp/assisted.out" "$tmp/assisted.err" | tail -1)
[[ -f "$assisted_envelope/report.json" && -f "$assisted_envelope/outcome.json" \
   && -f "$assisted_envelope/run-class.txt" && -f "$assisted_envelope/abandoned.json" \
   && ! -e "$assisted_envelope/reset.json" ]]
[[ "$(cat "$assisted_envelope/run-class.txt")" == WITNESS-ASSISTED ]]
[[ "$("$real_engine" json get --file "$assisted_envelope/report.json" --field runClass)" == WITNESS-ASSISTED ]]
[[ "$("$real_engine" json get --file "$assisted_envelope/outcome.json" --field runClass)" == WITNESS-ASSISTED ]]
[[ "$("$real_engine" json get --file "$repo/metasystem/artifacts/agents/battery-weight.json" --field accumulated)" \
   == "$assisted_weight_before" ]]
[[ "$("$real_engine" json get --file "$repo/metasystem/artifacts/agents/battery-weight.json" \
  --field checkpoint --default __MISSING__)" == __MISSING__ ]]
if [[ "${BATTERY_RUN_CLASS_FIXTURE_ONLY:-0}" == 1 ]]; then
  echo "gate-run-freeze run-class fixture passed"
  exit 0
fi

# A red envelope accepts a retained nested validation tree when its PATH tools
# are regular executables, and its digest manifest covers the copied evidence.
run_nested_failure_copy_fixture

# A controller setup failure still publishes a complete minimal envelope from
# the EXIT controller and removes the clone only after teardown.json lands.
set +e
BATTERY_FAKE_BUILD_FAIL=1 \
  "$repo/metasystem/scripts/agents/milestone-battery.sh" --subject "$subject" --evidence-root "$evidence" \
  >"$tmp/setup-fail.out" 2>"$tmp/setup-fail.err"
setup_fail_rc=$?
set -e
[[ $setup_fail_rc != 0 ]]
setup_fail_envelope=$(sed -n 's/.* envelope=\(.*\)$/\1/p' "$tmp/setup-fail.out" "$tmp/setup-fail.err" | tail -1)
[[ -f "$setup_fail_envelope/report.json" && -f "$setup_fail_envelope/outcome.json" \
   && -f "$setup_fail_envelope/teardown.json" ]]
[[ "$("$real_engine" json get --file "$setup_fail_envelope/report.json" --field setupExit)" != 0 ]]
[[ "$("$real_engine" json get --file "$setup_fail_envelope/report.json" --field validationExit)" == -1 ]]
[[ "$("$real_engine" json get --file "$setup_fail_envelope/report.json" --field verdict)" == setup-failed ]]
grep -Fq 'clone=removed' "$tmp/setup-fail.err"

# A pre-publication reset failure leaves the checkpoint open. The already
# published stage-one envelope proves publication preceded the reset attempt.
set +e
env BATTERY_READY=$ready BATTERY_RELEASE=$release BATTERY_CLONE_PATH=$clone_path \
  BATTERY_REGISTRY_PATH=$registry_path BATTERY_EXPECT_HOME="${HOME:-}" \
  BATTERY_LIVE_ROOT=$repo BATTERY_LIVE_REGISTRY=$live_registry \
  BATTERY_TEARDOWN_CALL=$teardown_call BATTERY_FAKE_RESET_MODE=open \
  "$repo/metasystem/scripts/agents/milestone-battery.sh" --subject "$subject" --evidence-root "$evidence" \
  >"$tmp/reset-open.out" 2>"$tmp/reset-open.err"
reset_open_rc=$?
set -e
[[ $reset_open_rc != 0 ]]
grep -Fq 'green/reset-unrecorded' "$tmp/reset-open.out" "$tmp/reset-open.err"
reset_open_envelope=$(sed -n 's/.* envelope=\(.*\)$/\1/p' "$tmp/reset-open.out" "$tmp/reset-open.err" | tail -1)
[[ -f "$reset_open_envelope/report.json" && -f "$reset_open_envelope/outcome.json" \
   && ! -e "$reset_open_envelope/reset.json" ]]
grep -Fq 'green/reset-unrecorded' "$reset_open_envelope/outcome.json"
weight_state=$repo/metasystem/artifacts/agents/battery-weight.json
[[ "$("$real_engine" json get --file "$weight_state" --field checkpoint \
  --default __MISSING__)" != __MISSING__ ]]
[[ "$("$real_engine" json get --file "$weight_state" --field pendingReset \
  --default __MISSING__)" == __MISSING__ ]]
rm -f -- "$repo/metasystem/artifacts/agents/battery-weight.json" \
  "$repo/metasystem/artifacts/agents/battery-weight.flock"

# A consumed reset whose appendix is pending is distinct from an open reset
# failure. The real state owner's retry behavior is covered by Go fault tests;
# this leg pins the controller's outcome class and non-green exit.
set +e
env BATTERY_READY=$ready BATTERY_RELEASE=$release BATTERY_CLONE_PATH=$clone_path \
  BATTERY_REGISTRY_PATH=$registry_path BATTERY_EXPECT_HOME="${HOME:-}" \
  BATTERY_LIVE_ROOT=$repo BATTERY_LIVE_REGISTRY=$live_registry \
  BATTERY_TEARDOWN_CALL=$teardown_call BATTERY_FAKE_RESET_MODE=appendix-pending \
  "$repo/metasystem/scripts/agents/milestone-battery.sh" --subject "$subject" --evidence-root "$evidence" \
  >"$tmp/reset-pending.out" 2>"$tmp/reset-pending.err"
reset_pending_rc=$?
set -e
[[ $reset_pending_rc != 0 ]]
grep -Fq 'green/reset-appendix-pending' "$tmp/reset-pending.out" "$tmp/reset-pending.err"
reset_pending_envelope=$(sed -n 's/.* envelope=\(.*\)$/\1/p' "$tmp/reset-pending.out" "$tmp/reset-pending.err" | tail -1)
[[ -f "$reset_pending_envelope/report.json" && -f "$reset_pending_envelope/outcome.json" \
   && ! -e "$reset_pending_envelope/reset.json" ]]

# Teardown is an appendix after the otherwise-final verdict. A failed exact
# shutdown keeps the clone and every earlier appendix, returns nonzero, and
# does not rewrite the green validation/reset facts.
set +e
env BATTERY_READY=$ready BATTERY_RELEASE=$release BATTERY_CLONE_PATH=$clone_path \
  BATTERY_REGISTRY_PATH=$registry_path BATTERY_EXPECT_HOME="${HOME:-}" \
  BATTERY_LIVE_ROOT=$repo BATTERY_LIVE_REGISTRY=$live_registry \
  BATTERY_TEARDOWN_CALL=$teardown_call BATTERY_FAKE_TEARDOWN_FAIL=1 \
  "$repo/metasystem/scripts/agents/milestone-battery.sh" --subject "$subject" --evidence-root "$evidence" \
  >"$tmp/teardown-fail.out" 2>"$tmp/teardown-fail.err"
teardown_fail_rc=$?
set -e
[[ $teardown_fail_rc != 0 ]] \
  || { echo "gate-run-freeze fixture: teardown failure exited green" >&2; exit 1; }
grep -Fq 'retained clone' "$tmp/teardown-fail.err" \
  || { echo "gate-run-freeze fixture: teardown failure did not report the retained clone" >&2; exit 1; }
teardown_fail_envelope=$(sed -n 's/.* envelope=\(.*\)$/\1/p' "$tmp/teardown-fail.out" "$tmp/teardown-fail.err" | tail -1)
[[ -f "$teardown_fail_envelope/report.json" && -f "$teardown_fail_envelope/outcome.json" \
   && -f "$teardown_fail_envelope/reset.json" \
   && -f "$teardown_fail_envelope/teardown.json" ]] \
  || { echo "gate-run-freeze fixture: teardown failure lost a required evidence appendix (envelope=$teardown_fail_envelope)" >&2; exit 1; }
grep -Fq 'recorded-process-shutdown-failed' "$teardown_fail_envelope/teardown.json" \
  || { echo "gate-run-freeze fixture: teardown.json did not record the shutdown failure" >&2; exit 1; }
retained=$(sed -n 's/.* path=\([^ ]*\) envelope=.*/\1/p' "$tmp/teardown-fail.err" | tail -1)
[[ -n "$retained" && -d "$retained" ]] \
  || { echo "gate-run-freeze fixture: the reported teardown-failure clone was not retained (path=$retained)" >&2; exit 1; }
retained_owner=$("$real_engine" json get --file "$retained/metasystem/artifacts/agents/supervision/lock.d/owner.json" --field pid)
kill -TERM "$retained_owner" 2>/dev/null || true
for ((attempt=0; attempt<400; attempt++)); do kill -0 "$retained_owner" 2>/dev/null || break; sleep 0.01; done
kill -0 "$retained_owner" 2>/dev/null && kill -KILL "$retained_owner" 2>/dev/null || true
rm -rf -- "${retained%/subject}"

# A teardown-appendix publication failure happens after successful exact
# shutdown and clone removal. It returns nonzero and must not claim a retained
# path that no longer exists.
set +e
env BATTERY_FAKE_TEARDOWN_APPENDIX_FAIL=1 \
  "$repo/metasystem/scripts/agents/milestone-battery.sh" --subject "$subject" --evidence-root "$evidence" \
  >"$tmp/teardown-appendix.out" 2>"$tmp/teardown-appendix.err"
teardown_appendix_rc=$?
set -e
[[ $teardown_appendix_rc != 0 ]]
grep -Fq 'teardown evidence incomplete; clone already removed' "$tmp/teardown-appendix.err" \
  || { echo "gate-run-freeze fixture: teardown-appendix failure did not report removed-clone state" >&2; cat "$tmp/teardown-appendix.err" >&2; exit 1; }
! grep -Fq 'retained clone' "$tmp/teardown-appendix.err"
teardown_appendix_envelope=$(sed -n 's/.* envelope=\(.*\)$/\1/p' "$tmp/teardown-appendix.out" "$tmp/teardown-appendix.err" | tail -1)
[[ -f "$teardown_appendix_envelope/report.json" && -f "$teardown_appendix_envelope/outcome.json" \
   && -f "$teardown_appendix_envelope/reset.json" && ! -e "$teardown_appendix_envelope/teardown.json" ]]

# TERM is an explicit non-green terminal: the validator process group and the
# recorded supervision owner stop before abandonment, then stage one and
# abandoned.json survive while reset never appears.
abort_ready=$tmp/abort-ready
abort_release=$tmp/abort-release
abort_validator_pid=$tmp/abort-validator-pid
abort_validator_child_pid=$tmp/abort-validator-child-pid
env BATTERY_READY=$abort_ready BATTERY_RELEASE=$abort_release BATTERY_CLONE_PATH=$clone_path \
  BATTERY_REGISTRY_PATH=$registry_path BATTERY_EXPECT_HOME="${HOME:-}" \
  BATTERY_LIVE_ROOT=$repo BATTERY_LIVE_REGISTRY=$live_registry \
  BATTERY_TEARDOWN_CALL=$teardown_call BATTERY_VALIDATOR_PID_PATH=$abort_validator_pid \
  BATTERY_VALIDATOR_CHILD_PID_PATH=$abort_validator_child_pid \
  BATTERY_VALIDATOR_CHILD_SURVIVES_TERM=1 \
  "$repo/metasystem/scripts/agents/milestone-battery.sh" --subject "$subject" --evidence-root "$evidence" \
  >"$tmp/abort.out" 2>"$tmp/abort.err" &
abort_pid=$!
for _ in $(seq 1 2000); do [[ -e "$abort_ready" ]] && break; kill -0 "$abort_pid" 2>/dev/null || break; sleep 0.01; done
[[ -e "$abort_ready" ]]
kill -TERM "$abort_pid"
set +e
wait "$abort_pid"
abort_rc=$?
set -e
[[ $abort_rc == 130 ]]
abort_envelope=$(sed -n 's/.* envelope=\(.*\)$/\1/p' "$tmp/abort.out" "$tmp/abort.err" | tail -1)
[[ -f "$abort_envelope/report.json" && -f "$abort_envelope/abandoned.json" \
   && ! -e "$abort_envelope/reset.json" ]]
abort_validator=$(cat "$abort_validator_pid")
abort_validator_child=$(cat "$abort_validator_child_pid")
! kill -0 "$abort_validator" 2>/dev/null
for _ in $(seq 1 400); do
  kill -0 "$abort_validator_child" 2>/dev/null || break
  sleep 0.01
done
! kill -0 "$abort_validator_child" 2>/dev/null
aborted_clone=$(cat "$clone_path")
[[ ! -e "$aborted_clone" ]]
grep -Fq 'clone=removed' "$tmp/abort.err"

# A stage-one copy failure retains the clone and performs no reset.
copy_bad=$tmp/copy-bad
mkdir -p "$copy_bad"
printf 'not a directory\n' >"$copy_bad/suite-failures"
set +e
"$repo/metasystem/scripts/agents/milestone-battery.sh" --subject "$subject" --evidence-root "$copy_bad" --force-red \
  >"$tmp/copy.out" 2>"$tmp/copy.err"
copy_rc=$?
set -e
[[ $copy_rc != 0 ]]
grep -Fq 'evidence-incomplete' "$tmp/copy.err"
retained=$(sed -n 's/.* path=\([^ ]*\) envelope=.*/\1/p' "$tmp/copy.err" | tail -1)
[[ -d "$retained" ]]
rm -rf -- "${retained%/subject}"

echo "gate-run-freeze fixtures passed"
