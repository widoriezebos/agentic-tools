#!/usr/bin/env bash
set -euo pipefail

# The landing chain is exercised against ordinary repositories and local bare
# remotes. Only the commit wrapper is reduced to its Git and ancestry-token
# boundaries so the fixture proves the driver's ordering without invoking the
# repository's independent static proof for every leg.
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
source_engine=$root/bin/metasystem
[[ -x "$source_engine" ]] \
  || { echo "land fixture: current source engine is absent; run the Go gate first" >&2; exit 1; }

# Every leg creates standalone repositories. Their object writes stay local;
# inherited object-store steering must not leak into fixtures.
unset GIT_OBJECT_DIRECTORY GIT_ALTERNATE_OBJECT_DIRECTORIES

source "$root/scripts/agents/fixture-budget.sh"
source "$root/scripts/agents/fixture-bed-scenarios.sh"
fixture_bed_child=0
fixture_scenario=
if fixture_scenario=$(harness_fixture_bed_child_scenario land "$@"); then
  fixture_bed_child=1
else
  fixture_bed_child_rc=$?
  [[ $fixture_bed_child_rc -eq 1 ]] || exit "$fixture_bed_child_rc"
fi
unset METASYSTEM_FIXTURE_SCENARIO
if (( ! fixture_bed_child )); then
  fixture_bed_script=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/$(basename "${BASH_SOURCE[0]}")
  run_fixture_bed_scenarios land "land fixtures passed (25 isolated legs)" \
    "$fixture_bed_script" push-retry step-failure new-plan goal tier-one full-width-chain build-stamp \
    brain-land-refuses brain-absent-node-proceeds ledger-move-lands records-move-lands \
    input-move-refuses receipt-cutover carried-fresh carried-prefixed carried-second carried-red-battery \
    carried-intent-failure carried-crash-local carried-asks carried-ledger-path carried-crash \
    carried-two-seat carried-debt-abandoned carried-debt-expired
fi

tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-land.XXXXXX")
tmp=$(cd "$tmp" && pwd -P)
real_git=$(command -v git)
receipt_environment=()
receipt_runner_checkouts=()
receipt_runner_engines=()
receipt_runner_pids=()
receipt_runner_identity_files=()
receipt_runner_registries=()
receipt_runner_stop_logs=()

is_workspace_receipt_scenario() {
  case "$fixture_scenario" in
    ledger-move-lands | records-move-lands | input-move-refuses | receipt-cutover) return 0 ;;
    *) return 1 ;;
  esac
}

is_carried_scenario() {
  case "$fixture_scenario" in
    carried-fresh | carried-prefixed | carried-second | carried-red-battery | carried-intent-failure | carried-crash-local | carried-asks | carried-ledger-path | carried-crash | carried-two-seat | carried-debt-abandoned | carried-debt-expired) return 0 ;;
    *) return 1 ;;
  esac
}

is_carried_two_seat_scenario() {
  case "$fixture_scenario" in
    carried-two-seat | carried-debt-abandoned | carried-debt-expired) return 0 ;;
    *) return 1 ;;
  esac
}

prepare_receipt_environment() { # process identity file, registry
  local identity_file=$1 registry=$2 name value
  receipt_environment=()
  for name in GOCACHE GOMODCACHE GOPATH GOROOT HOME LANG LC_ALL PATH SYSTEMROOT TEMP TMP TMPDIR TZ; do
    if value=$(printenv "$name" 2>/dev/null); then
      receipt_environment+=("$name=$value")
    fi
  done
  receipt_environment+=(
    "METASYSTEM_FAKE_PROCESS_IDENTITY_FILE=$identity_file"
    "METASYSTEM_SUPERVISION_REGISTRY_HOME=$registry"
    "METASYSTEM_OWNER_LINEAGE=land-receipt-fixture"
  )
}

receipt_env_run() {
  env -i "${receipt_environment[@]}" "$@"
}

receipt_checkout_env_run() { # checkout, command...
  local checkout=$1 entry receipt_path= receipt_path_found=0
  shift
  for entry in "${receipt_environment[@]}"; do
    if [[ "$entry" == PATH=* ]]; then
      receipt_path=${entry#PATH=}
      receipt_path_found=1
      break
    fi
  done
  (( receipt_path_found )) || {
    echo "land $fixture_scenario fixture: receipt environment has no PATH entry" >&2
    return 1
  }
  receipt_env_run env PATH="$checkout/bin:$receipt_path" "$@"
}

stop_receipt_runner() {
  local index checkout engine runner_pid identity_file registry stop_log
  local stop_rc wait_deadline runner_dead=1
  index=$((${#receipt_runner_checkouts[@]} - 1))
  (( index >= 0 )) || return 0
  checkout=${receipt_runner_checkouts[$index]}
  engine=${receipt_runner_engines[$index]}
  runner_pid=${receipt_runner_pids[$index]}
  identity_file=${receipt_runner_identity_files[$index]}
  registry=${receipt_runner_registries[$index]}
  stop_log=${receipt_runner_stop_logs[$index]}
  prepare_receipt_environment "$identity_file" "$registry"
  if receipt_env_run "$engine" steward disarm --repo "$checkout" \
      >"$stop_log" 2>&1; then
    stop_rc=0
  else
    stop_rc=$?
  fi
  if [[ "$runner_pid" =~ ^[1-9][0-9]*$ ]]; then
    wait_deadline=$((SECONDS + 5))
    while kill -0 "$runner_pid" 2>/dev/null && (( SECONDS < wait_deadline )); do
      sleep 0.25
    done
    if kill -0 "$runner_pid" 2>/dev/null; then
      echo "land $fixture_scenario fixture: steward runner pid $runner_pid survived steward disarm --repo" >&2
      runner_dead=0
    fi
  fi
  if (( runner_dead )); then
    receipt_runner_checkouts=("${receipt_runner_checkouts[@]:0:$index}")
    receipt_runner_engines=("${receipt_runner_engines[@]:0:$index}")
    receipt_runner_pids=("${receipt_runner_pids[@]:0:$index}")
    receipt_runner_identity_files=("${receipt_runner_identity_files[@]:0:$index}")
    receipt_runner_registries=("${receipt_runner_registries[@]:0:$index}")
    receipt_runner_stop_logs=("${receipt_runner_stop_logs[@]:0:$index}")
    echo "land $fixture_scenario fixture: steward runner pid ${runner_pid:-unknown} stopped with no survivor"
  fi
  if (( stop_rc != 0 )); then
    echo "land $fixture_scenario fixture: steward disarm --repo failed for $checkout" >&2
    sed -n '1,200p' "$stop_log" >&2
    return 1
  fi
  (( runner_dead ))
}

select_receipt_runner_environment() { # checkout
  local checkout=$1 index
  for ((index = ${#receipt_runner_checkouts[@]} - 1; index >= 0; index--)); do
    if [[ "${receipt_runner_checkouts[$index]}" == "$checkout" ]]; then
      prepare_receipt_environment "${receipt_runner_identity_files[$index]}" \
        "${receipt_runner_registries[$index]}"
      return 0
    fi
  done
  echo "land $fixture_scenario fixture: no armed receipt runner for $checkout" >&2
  return 1
}

cleanup_land_fixture() {
  local status=$? cleanup_status=0 before stop_status
  trap - EXIT
  while (( ${#receipt_runner_checkouts[@]} )); do
    before=${#receipt_runner_checkouts[@]}
    if stop_receipt_runner; then
      :
    else
      stop_status=$?
      cleanup_status=$stop_status
      (( ${#receipt_runner_checkouts[@]} < before )) || break
    fi
  done
  rm -rf "$tmp"
  (( cleanup_status == 0 )) || status=1
  exit "$status"
}
trap cleanup_land_fixture EXIT

clear_independent_fixture_context() {
  unset METASYSTEM_PROOF_CONTROL_ROOT METASYSTEM_PROOF_ATTEMPT \
    METASYSTEM_PROOF_RUN_ROOT METASYSTEM_PROOF_RUN_ID \
    METASYSTEM_PROOF_RECORD_KEY METASYSTEM_PROOF_CREATION_CLAIM \
    METASYSTEM_PROOF_AUTH_BIN METASYSTEM_HOOK_DELEGATE_STATE_ROOT \
    METASYSTEM_HOOK_DELEGATE_INSTALLATION_ROOT METASYSTEM_HOOK_DELEGATE_JOB \
    METASYSTEM_SUITE_PROGRESS_ACTIVE METASYSTEM_SUITE_PROGRESS_ROOT \
    METASYSTEM_SUITE_PROGRESS_LOG METASYSTEM_ENUMERATION_ENGINE_DEPENDENCY \
    METASYSTEM_ENUMERATION_STAGE_RESULTS_OUT
}

make_leg() { # name
  leg_root=$tmp/$1
  leg_seed_repo=$leg_root/seed
  leg_seed=$leg_seed_repo
  leg_remote=$leg_root/origin.git
  leg_local_repo=$leg_root/local
  leg_peer_repo=$leg_root/peer
  leg_local=$leg_local_repo
  leg_peer=$leg_peer_repo
  if [[ "$1" == carried-prefixed ]]; then
    leg_seed=$leg_seed_repo/metasystem
    leg_local=$leg_local_repo/metasystem
    leg_peer=$leg_peer_repo/metasystem
  fi
  mkdir -p "$leg_seed/scripts/agents" "$leg_seed/plans" "$leg_seed/bin"
  cp "$root/scripts/agents/land.sh" "$leg_seed/scripts/agents/land.sh"
  cp "$root/scripts/agents/coverage-delta.sh" "$leg_seed/scripts/agents/coverage-delta.sh"
  cp "$root/scripts/agents/pre-commit-guard.sh" "$leg_seed/scripts/agents/pre-commit-guard.sh"
  cp "$root/scripts/agents/sync-transport.sh" "$leg_seed/scripts/agents/sync-transport.sh"
  if ! is_workspace_receipt_scenario; then
    cp "$source_engine" "$leg_seed/bin/metasystem"
  fi
  cat >"$leg_seed/scripts/agents/commit.sh" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
token=$root/artifacts/agents/mains/worktree-commit-token.json
mkdir -p "${token%/*}"
started=$("$root/bin/metasystem" proc started-at --pid $$)
nonce=$("$root/bin/metasystem" util token-hex --bytes 16)
"$root/bin/metasystem" lease commit-token \
  --path "$token" --pid $$ --start "$started" --nonce "$nonce"
trap 'rm -f -- "$token"' EXIT
goal=
chain=
direct_fix=
root_job=
test_receipt=
commit_args=()
while (( $# )); do
  case "$1" in
    --goal)
      [[ $# -ge 2 && -z "$goal" ]] || exit 2
      goal=$2
      shift 2
      ;;
    --chain)
      [[ $# -ge 2 && -z "$chain" ]] || exit 2
      chain=$2
      shift 2
      ;;
    --direct-fix)
      [[ $# -ge 2 && -z "$direct_fix" ]] || exit 2
      direct_fix=$2
      shift 2
      ;;
    --root-job)
      [[ $# -ge 2 && -z "$root_job" ]] || exit 2
      root_job=$2
      shift 2
      ;;
    --test-receipt)
      [[ $# -ge 2 && -z "$test_receipt" ]] || exit 2
      test_receipt=$2
      shift 2
      ;;
    *)
      commit_args+=("$1")
      shift
      ;;
  esac
done
if [[ -n "${LAND_FIXTURE_GOAL_LOG:-}" ]]; then
  printf '%s\n' "$goal" >"$LAND_FIXTURE_GOAL_LOG"
fi
if [[ -n "${LAND_FIXTURE_TIER_ONE_LOG:-}" ]]; then
  printf 'directFix=%s\nrootJob=%s\ntestReceipt=%s\n' \
    "$direct_fix" "$root_job" "$test_receipt" >"$LAND_FIXTURE_TIER_ONE_LOG"
  [[ -f "$test_receipt" ]] || exit 82
fi
if [[ -n "$chain" ]]; then
  candidate_tree=$(git -C "$root" write-tree)
  prefix=$(git -C "$root" rev-parse --show-prefix)
  if [[ -n "$prefix" ]]; then
    candidate_tree=$(git -C "$root" rev-parse "$candidate_tree:${prefix%/}")
  fi
  landing_args=(landing observe --root "$root" --tree "$candidate_tree" --chain "$chain")
  [[ -z "$goal" ]] || landing_args+=(--goal "$goal")
  [[ -z "$test_receipt" ]] || landing_args+=(--test-receipt "$test_receipt")
  machine_nickname=$(git -C "$root" config --get metasystem.goal.machine)
  landing_args+=(--actor "${machine_nickname}+${METASYSTEM_OWNER_LINEAGE:-human}")
  landing_observation=$("$root/bin/metasystem" "${landing_args[@]}")
  landing_provenance=$("$root/bin/metasystem" json get --value "$landing_observation" --field provenance)
  landing_verdict=$("$root/bin/metasystem" json get --value "$landing_observation" --field verdictTrailer)
  [[ "$landing_verdict" == "pass bar=a" ]] || {
    echo "land fixture commit refused: $landing_verdict" >&2
    exit 83
  }
  commit_args+=(--trailer "Landing-Provenance: $landing_provenance")
  commit_args+=(--trailer "Landing-Provenance-Verdict: $landing_verdict")
  if [[ -n "${LAND_FIXTURE_CHAIN_LOG:-}" ]]; then
    printf 'chain=%s\ntestReceipt=%s\nverdict=%s\n' \
      "$chain" "$test_receipt" "$landing_verdict" >"$LAND_FIXTURE_CHAIN_LOG"
  fi
fi
git commit "${commit_args[@]}"
git show --no-renames --numstat -z --format= HEAD \
  | "$root/bin/metasystem" gate weight-add --root "$root" \
      --commit "$(git rev-parse --short HEAD)"
SH
	if is_carried_scenario; then
	  cp "$root/scripts/agents/commit.sh" "$leg_seed/scripts/agents/commit.sh"
	  cp "$root/scripts/agents/path-classes.txt" "$leg_seed/scripts/agents/path-classes.txt"
	  cp "$root/scripts/agents/landing-classes.json" "$leg_seed/scripts/agents/landing-classes.json"
	  cp "$root/scripts/agents/landing-promotion.json" "$leg_seed/scripts/agents/landing-promotion.json"
	  mkdir -p "$leg_seed/memory"
	  cp "$root/memory/rulings.md" "$leg_seed/memory/rulings.md"
	  printf 'install:payload.txt behavior\ninstall:payload-b.txt behavior\n' >>"$leg_seed/scripts/agents/path-classes.txt"
	  carried_fixture_engine=$leg_root/carried-engine
	  printf -v carried_fixture_engine_q '%q' "$carried_fixture_engine"
	  cat >"$leg_seed/scripts/agents/go-build.sh" <<SH
#!/usr/bin/env bash
set -euo pipefail
[[ "\${1:-}" == --trimpath && "\${2:-}" == --out && -n "\${3:-}" ]]
cp $carried_fixture_engine_q "\$3"
chmod +x "\$3"
SH
	fi
  if is_workspace_receipt_scenario; then
    mkdir -p "$leg_seed/memory"
    cp "$root/scripts/agents/path-classes.txt" "$leg_seed/scripts/agents/path-classes.txt"
    cp "$root/scripts/agents/landing-classes.json" "$leg_seed/scripts/agents/landing-classes.json"
    cp "$root/scripts/agents/landing-promotion.json" "$leg_seed/scripts/agents/landing-promotion.json"
    cp "$root/memory/rulings.md" "$leg_seed/memory/rulings.md"
    cat >"$leg_seed/scripts/agents/go-build.sh" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
[[ "${1:-}" == --trimpath && "${2:-}" == --out && -n "${3:-}" ]]
candidate_fixture_engine=$(command -v metasystem || true)
[[ -n "$candidate_fixture_engine" ]] || {
  echo "fixture candidate build: no metasystem executable is available on PATH" >&2
  exit 1
}
cp "$candidate_fixture_engine" "$3"
chmod +x "$3"
SH
    cat >"$leg_seed/metasystem.conf" <<'CONF'
testing.contract=testing.json
metasystem.runtimes=fake
dispatch.cap-min=1
dispatch.cap-max=120
CONF
    cat >"$leg_seed/testing.json" <<'JSON'
{
  "schemaVersion": 1,
  "projectRisk": {"severity": 1, "exposure": 1, "reversibility": "revert", "detection": "immediate", "recovery": "bounded"},
  "surfaces": [
    {"id": "application", "paths": ["testing.json", "metasystem.conf", "plans/**", "records/**", "scripts/**"], "dependsOn": [], "standard": ["policy-protection", "candidate-smoke"], "deep": [], "critical": ["application-proof"]},
    {"id": "proof-and-landing", "paths": ["payload.txt"], "dependsOn": ["application"], "standard": ["policy-protection", "candidate-smoke"], "deep": [], "critical": ["application-proof"]}
  ],
  "groups": [
    {"id": "policy-protection", "kind": "unit", "adapter": "command", "cwd": ".", "inputs": ["payload.txt", "scripts/**"], "outputs": ["policy-reports"], "tools": [], "obligations": ["application-proof"], "platforms": ["any"], "targetMs": 1000, "argv": ["sh", "-c", "mkdir -p policy-reports; printf '%s\\n' '<testsuite><testcase classname=\"application\" name=\"policy\"/></testsuite>' >policy-reports/result.xml"], "reports": ["policy-reports"], "format": "junit-xml", "expectedTests": [{"report": "policy-reports/result.xml", "classname": "application", "name": "policy"}]},
    {"id": "candidate-smoke", "kind": "unit", "adapter": "command", "cwd": ".", "inputs": ["payload.txt"], "outputs": ["smoke-reports"], "tools": [], "obligations": ["application-proof"], "platforms": ["any"], "targetMs": 1000, "argv": ["sh", "-c", "mkdir -p smoke-reports; printf '%s\\n' '<testsuite><testcase classname=\"application\" name=\"smoke\"/></testsuite>' >smoke-reports/result.xml"], "reports": ["smoke-reports"], "format": "junit-xml", "expectedTests": [{"report": "smoke-reports/result.xml", "classname": "application", "name": "smoke"}]}
  ],
  "always": {"canary": ["policy-protection", "candidate-smoke"], "standard": ["policy-protection", "candidate-smoke"]},
  "unknown": ["policy-protection", "candidate-smoke"],
  "cadence": ["policy-protection", "candidate-smoke"]
}
JSON
    printf '%s\n' '{"floors":{"fixture/application":80.0}}' \
      >"$leg_seed/scripts/agents/coverage-ratchet.json"
    printf '%s\n' '{"floors":{"fixture/application":80.0}}' \
      >"$leg_seed/scripts/agents/coverage-ratchet-linux.json"
  fi
  if [[ "$fixture_scenario" == full-width-chain ]]; then
    mkdir -p "$leg_seed/memory"
    cp "$root/scripts/agents/path-classes.txt" "$leg_seed/scripts/agents/path-classes.txt"
    cp "$root/scripts/agents/landing-classes.json" "$leg_seed/scripts/agents/landing-classes.json"
    cp "$root/scripts/agents/landing-promotion.json" "$leg_seed/scripts/agents/landing-promotion.json"
    cp "$root/memory/rulings.md" "$leg_seed/memory/rulings.md"
    for battery_script in go-gate.sh dispatch-fixtures.sh goal-cli-fixtures.sh; do
      printf '#!/usr/bin/env bash\nexit 0\n' >"$leg_seed/scripts/agents/$battery_script"
      chmod +x "$leg_seed/scripts/agents/$battery_script"
    done
  fi
  chmod +x "$leg_seed/scripts/agents/land.sh" \
    "$leg_seed/scripts/agents/coverage-delta.sh" \
    "$leg_seed/scripts/agents/pre-commit-guard.sh" \
    "$leg_seed/scripts/agents/commit.sh" \
    "$leg_seed/scripts/agents/sync-transport.sh"
	if is_workspace_receipt_scenario || is_carried_scenario; then
	  chmod +x "$leg_seed/scripts/agents/go-build.sh"
	else
    chmod +x "$leg_seed/bin/metasystem"
  fi
  printf 'seed\n' >"$leg_seed/payload.txt"
  printf 'existing plan\n' >"$leg_seed/plans/existing.md"
  if [[ "$fixture_scenario" == receipt-cutover ]]; then
    printf 'artifacts/\nrecords/narrator-digest.log\nbin/\n' >"$leg_seed/.gitignore"
  elif is_workspace_receipt_scenario || is_carried_scenario; then
    printf 'artifacts/\nrecords/narrator-digest.log\n' >"$leg_seed/.gitignore"
  else
    printf 'artifacts/\n' >"$leg_seed/.gitignore"
  fi
  if [[ "$fixture_scenario" == tier-one || "$fixture_scenario" == full-width-chain ]] || is_carried_scenario; then
    cat >"$leg_seed/metasystem.conf" <<'CONF'
metasystem.runtimes=fake
dispatch.cap-min=1
dispatch.cap-max=120
CONF
    mkdir -p "$leg_seed/plans/goals"
    cat >"$leg_seed/plans/goals/backlog.md" <<'BACKLOG'
# Backlog

- Identity: 01ARZ3NDEKTSV4RRFFQ69G5FBV
- FormatVersion: 1
- SyncMode: local
- Revision: 1

History:
BACKLOG
    if is_carried_scenario; then
      conf_edit "$leg_seed/plans/goals/backlog.md" replace-line-first \
        '^- SyncMode: local$' '- SyncMode: remote'
    fi
    receipt_root_digest=$("$source_engine" util sha256 --file "$leg_seed/plans/goals/backlog.md")
    printf 'Integrity: sha256=%s\n' "$receipt_root_digest" >>"$leg_seed/plans/goals/backlog.md"
  fi
  if is_carried_scenario; then
    cat >"$leg_seed/metasystem.conf" <<'CONF'
testing.contract=testing.json
metasystem.runtimes=fake
dispatch.cap-min=1
dispatch.cap-max=120
metasystem.budget.carry-open-max=1
CONF
    cat >"$leg_seed/testing.json" <<'JSON'
{
  "schemaVersion": 1,
  "projectRisk": {"severity": 1, "exposure": 1, "reversibility": "revert", "detection": "immediate", "recovery": "bounded"},
  "surfaces": [
    {"id": "fixture-carry", "paths": ["payload.txt", "payload-b.txt", "records/misc/fx-red.md", "records/counselor/accepted-risk-register.jsonl", "records/counselor/carried-landings.jsonl"], "dependsOn": [], "standard": ["fixture-carry"], "deep": [], "critical": []},
    {"id": "fixture-support", "paths": ["testing.json", "metasystem.conf", "plans/**", "scripts/**", "bin/**"], "dependsOn": [], "standard": ["fixture-carry"], "deep": [], "critical": []}
  ],
  "groups": [
	{"id": "fixture-carry", "kind": "unit", "adapter": "command", "cwd": ".", "inputs": ["payload.txt", "records/misc/fx-red.md", "records/counselor/accepted-risk-register.jsonl", "records/counselor/carried-landings.jsonl"], "outputs": ["fixture-reports"], "tools": [], "obligations": [], "platforms": ["any"], "targetMs": 1000, "argv": ["sh", "-c", "mkdir -p fixture-reports; printf '%s\\n' '<testsuite><testcase classname=\"fixture\" name=\"carry\"/></testsuite>' >fixture-reports/result.xml; test ! -f fixture-red"], "reports": ["fixture-reports"], "format": "junit-xml", "expectedTests": [{"report": "fixture-reports/result.xml", "classname": "fixture", "name": "carry"}]}
  ],
  "always": {"canary": [], "standard": []},
  "unknown": ["fixture-carry"],
  "cadence": ["fixture-carry"]
}
JSON
    if [[ "$fixture_scenario" == carried-prefixed ]]; then
      conf_edit "$leg_seed/testing.json" replace-literal '"payload.txt"' '"metasystem/payload.txt"'
      conf_edit "$leg_seed/testing.json" replace-literal '"payload-b.txt"' '"metasystem/payload-b.txt"'
      conf_edit "$leg_seed/testing.json" replace-literal '"records/misc/fx-red.md"' '"metasystem/records/misc/fx-red.md"'
      conf_edit "$leg_seed/testing.json" replace-literal '"records/counselor/accepted-risk-register.jsonl"' '"metasystem/records/counselor/accepted-risk-register.jsonl"'
      conf_edit "$leg_seed/testing.json" replace-literal '"records/counselor/carried-landings.jsonl"' '"metasystem/records/counselor/carried-landings.jsonl"'
      conf_edit "$leg_seed/testing.json" replace-literal '"testing.json"' '"metasystem/testing.json"'
      conf_edit "$leg_seed/testing.json" replace-literal '"metasystem.conf"' '"metasystem/metasystem.conf"'
      conf_edit "$leg_seed/testing.json" replace-literal '"plans/**"' '"metasystem/plans/**"'
      conf_edit "$leg_seed/testing.json" replace-literal '"scripts/**"' '"metasystem/scripts/**"'
      conf_edit "$leg_seed/testing.json" replace-literal '"bin/**"' '"metasystem/bin/**"'
    fi
    if [[ "$fixture_scenario" == carried-red-battery ]]; then
      printf 'the fixture group exits red after producing its report\n' >"$leg_seed/fixture-red"
    fi
  fi
  if is_workspace_receipt_scenario; then
    mkdir -p "$leg_seed/plans/goals"
    cat >"$leg_seed/plans/goals/backlog.md" <<'BACKLOG'
# Backlog

- Identity: 01ARZ3NDEKTSV4RRFFQ69G5FBV
- FormatVersion: 1
- SyncMode: local
- Revision: 1

History:
BACKLOG
    receipt_root_digest=$("$source_engine" util sha256 --file "$leg_seed/plans/goals/backlog.md")
    printf 'Integrity: sha256=%s\n' "$receipt_root_digest" >>"$leg_seed/plans/goals/backlog.md"
  fi
  git -C "$leg_seed_repo" init -q
  git -C "$leg_seed_repo" symbolic-ref HEAD refs/heads/main
  git -C "$leg_seed" config user.name fixture
  git -C "$leg_seed" config user.email fixture@example.invalid
  if [[ "$fixture_scenario" == tier-one || "$fixture_scenario" == full-width-chain ]] || is_workspace_receipt_scenario || is_carried_scenario; then
    if is_carried_scenario; then
      git -C "$leg_seed" config goal.sync-remote origin
      git -C "$leg_seed" config goal.sync-branch refs/heads/main
    else
      git -C "$leg_seed" config goal.sync-remote local
      git -C "$leg_seed" config goal.sync-branch refs/heads/metasystem/goals
    fi
    git -C "$leg_seed" config metasystem.goal.machine fixture-machine
  fi
  git -C "$leg_seed" add -- scripts payload.txt plans/existing.md .gitignore
  if ! is_workspace_receipt_scenario; then
    git -C "$leg_seed" add -- bin
  fi
  if [[ "$fixture_scenario" == tier-one || "$fixture_scenario" == full-width-chain ]] || is_carried_scenario; then
    git -C "$leg_seed" add -- metasystem.conf plans/goals/backlog.md
  fi
  if is_carried_scenario; then
    git -C "$leg_seed" add -- testing.json memory/rulings.md
    [[ "$fixture_scenario" != carried-red-battery ]] || git -C "$leg_seed" add -- fixture-red
  fi
  if is_workspace_receipt_scenario; then
    git -C "$leg_seed" add -- metasystem.conf testing.json plans/goals/backlog.md memory/rulings.md
  fi
  if [[ "$fixture_scenario" == full-width-chain ]]; then
    git -C "$leg_seed" add -- memory/rulings.md
  fi
  git -C "$leg_seed" commit -qm seed
  if is_carried_scenario; then
    git init --bare -q "$leg_remote"
    git --git-dir="$leg_remote" symbolic-ref HEAD refs/heads/main
    git -C "$leg_seed" remote add origin "$leg_remote"
    git -C "$leg_seed" push -q -u origin main
  fi
  if is_workspace_receipt_scenario; then
    receipt_seed_build_stamp=$(git -C "$leg_seed" rev-parse HEAD)
  fi
  if [[ "$fixture_scenario" == tier-one || "$fixture_scenario" == full-width-chain ]] || is_workspace_receipt_scenario || is_carried_scenario; then
    git -C "$leg_seed" update-ref refs/heads/metasystem/goals HEAD
    git -C "$leg_seed" update-ref refs/metasystem/goals/accepted HEAD
    receipt_fixture_start=$("$source_engine" proc started-at --pid "$$")
    "$source_engine" lease announce --root "$leg_seed" --session "land-$fixture_scenario-seed" \
      --pid "$$" --start "$receipt_fixture_start" --tag "land-$fixture_scenario-seed" \
      --runtime fake --owner-lineage land-receipt-fixture >/dev/null
    METASYSTEM_OWNER_LINEAGE=land-receipt-fixture "$source_engine" goal open --root "$leg_seed" \
      --id fx --intent "Create an exact fixture-local landing receipt." \
      --next "Run the bounded fixture receipt." \
      --risk severity=1,novelty=1,exposure=1,accumulation=1 \
      --basis "This disposable fixture executes only its bounded local landing receipt." >/dev/null
    "$source_engine" goal approve --root "$leg_seed" --id fx --by Wido \
      --lineage land-receipt-fixture --elapsed-limit 4h --attempt-limit 4 \
      --reserved-job-minutes-limit 4 --active-job-limit 1 --review-round-limit 0 \
      --fixture-human-authority
    seed_claim_engine=$source_engine
    if [[ "$fixture_scenario" == receipt-cutover ]]; then
      # The pre-cutover engine claims the seed goal itself: the cutover leg's
      # older reader then meets a claim record in the grammar it writes, not
      # the candidate's (the claim grammar grew episode keys on 2026-09-11).
      seed_claim_engine=$cutover_seed_claim_engine
    fi
    "$seed_claim_engine" goal claim --root "$leg_seed" --id fx --lineage land-receipt-fixture >/dev/null
    git -C "$leg_seed" reset -q --hard refs/metasystem/goals/accepted
    if [[ "$fixture_scenario" == receipt-cutover ]]; then
      # In the cutover leg H0 is the one complete seed tip, including the fx
      # goal produced by the fixture's verbs. No engine-install commit follows.
      receipt_seed_build_stamp=$(git -C "$leg_seed" rev-parse HEAD)
    fi
    "$source_engine" lease retire --root "$leg_seed" --session "land-$fixture_scenario-seed" \
      --pid "$$" --start "$receipt_fixture_start" >/dev/null
  fi
  if is_workspace_receipt_scenario; then
    METASYSTEM_BUILD_STAMP="$receipt_seed_build_stamp" \
      bash "$root/scripts/agents/go-build.sh" --out "$leg_root/engine" >/dev/null
    if [[ "$fixture_scenario" != receipt-cutover ]]; then
      cp "$leg_root/engine" "$leg_seed/bin/metasystem"
      chmod +x "$leg_seed/bin/metasystem"
      git -C "$leg_seed" add -f -- bin/metasystem
      (cd "$leg_seed" && scripts/agents/commit.sh -qm "install candidate engine")
    fi
  fi
  if is_carried_scenario; then
    carried_seed_stamp=$(git -C "$leg_seed" rev-parse HEAD)
    METASYSTEM_BUILD_STAMP="$carried_seed_stamp" \
      bash "$root/scripts/agents/go-build.sh" --out "$leg_root/carried-engine" >/dev/null
    cp -f "$leg_root/carried-engine" "$leg_seed/bin/metasystem"
    chmod +x "$leg_seed/bin/metasystem"
    git -C "$leg_seed" add -f -- bin/metasystem
    git -C "$leg_seed" -c core.hooksPath=/dev/null commit -qm 'install carried fixture engine'
    git -C "$leg_seed" push -q origin main
  fi
  if ! is_carried_scenario; then
    git init --bare -q "$leg_remote"
    git --git-dir="$leg_remote" symbolic-ref HEAD refs/heads/main
    git -C "$leg_seed" remote add origin "$leg_remote"
    git -C "$leg_seed" push -q -u origin main
  fi
  git clone -q "$leg_remote" "$leg_local_repo"
  git clone -q "$leg_remote" "$leg_peer_repo"
  git -C "$leg_local" config user.name fixture-local
  git -C "$leg_local" config user.email fixture-local@example.invalid
  git -C "$leg_peer" config user.name fixture-peer
  git -C "$leg_peer" config user.email fixture-peer@example.invalid
  if [[ "$fixture_scenario" == tier-one || "$fixture_scenario" == full-width-chain ]] || is_workspace_receipt_scenario || is_carried_scenario; then
    git -C "$leg_local" config goal.sync-remote local
    git -C "$leg_local" config goal.sync-branch refs/heads/metasystem/goals
    git -C "$leg_local" config metasystem.goal.machine fixture-machine
    git -C "$leg_local" update-ref refs/heads/metasystem/goals origin/main
    git -C "$leg_local" update-ref refs/metasystem/goals/accepted origin/main
    git -C "$leg_local" config metasystem.steward.landing-ref refs/remotes/origin/main
    git -C "$leg_peer" config goal.sync-remote local
    git -C "$leg_peer" config goal.sync-branch refs/heads/metasystem/goals
    git -C "$leg_peer" config metasystem.goal.machine fixture-peer
    git -C "$leg_peer" config metasystem.steward.landing-ref refs/remotes/origin/main
    git -C "$leg_peer" update-ref refs/heads/metasystem/goals origin/main
    git -C "$leg_peer" update-ref refs/metasystem/goals/accepted origin/main
  fi
  if is_carried_scenario; then
    git -C "$leg_local" config goal.sync-remote origin
    git -C "$leg_local" config goal.sync-branch refs/heads/main
    git -C "$leg_peer" config goal.sync-remote origin
    git -C "$leg_peer" config goal.sync-branch refs/heads/main
  fi
  if [[ "$fixture_scenario" == tier-one || "$fixture_scenario" == full-width-chain ]]; then
    receipt_fixture_start=$("$source_engine" proc started-at --pid "$$")
    "$source_engine" lease announce --root "$leg_local" --session "land-$fixture_scenario" \
      --pid "$$" --start "$receipt_fixture_start" --tag "land-$fixture_scenario" \
      --runtime fake --owner-lineage land-receipt-fixture >/dev/null
  fi
}

arm_receipt_runner() { # checkout, engine
  local checkout=$1 engine=$2 identity_file registry arm_log runner_record deadline index runner_pid
  identity_file=$leg_root/process-identities.$(basename "$checkout").json
  registry=$leg_root/registry.$(basename "$checkout")
  arm_log=$leg_root/$(basename "$checkout").steward-arm.out
  mkdir -p "$registry"
  printf '{"%s":{"terminal":true}}\n' "$$" >"$identity_file"
  prepare_receipt_environment "$identity_file" "$registry"
  if ! receipt_env_run "$engine" steward arm --repo "$checkout" >"$arm_log" 2>&1; then
    echo "land $fixture_scenario fixture: steward arm --repo failed for $checkout" >&2
    sed -n '1,240p' "$arm_log" >&2
    return 1
  fi
  index=${#receipt_runner_checkouts[@]}
  receipt_runner_checkouts+=("$checkout")
  receipt_runner_engines+=("$engine")
  receipt_runner_pids+=("")
  receipt_runner_identity_files+=("$identity_file")
  receipt_runner_registries+=("$registry")
  receipt_runner_stop_logs+=("$leg_root/$(basename "$checkout").disarm.out")
  runner_record=$checkout/artifacts/agents/steward/runner.json
  deadline=$((SECONDS + 5))
  while [[ ! -s "$runner_record" ]] && (( SECONDS < deadline )); do
    sleep 0.25
  done
  [[ -s "$runner_record" ]] || {
    echo "land $fixture_scenario fixture: steward arm returned without a runner record" >&2
    return 1
  }
  runner_pid=$("$source_engine" json get --file "$runner_record" --field pid)
  receipt_runner_pids[$index]=$runner_pid
  [[ "$runner_pid" =~ ^[1-9][0-9]*$ ]] && kill -0 "$runner_pid" 2>/dev/null || {
    echo "land $fixture_scenario fixture: steward runner is not alive after arm" >&2
    return 1
  }
}

if is_carried_scenario; then
  clear_independent_fixture_context
  make_leg "$fixture_scenario"
  arm_receipt_runner "$leg_local" "$leg_local/bin/metasystem"
  carried_fixture_start=$(receipt_env_run "$leg_local/bin/metasystem" proc started-at --pid "$$")
  receipt_env_run "$leg_local/bin/metasystem" lease announce --root "$leg_local" \
    --session "land-$fixture_scenario" --pid "$$" --start "$carried_fixture_start" \
    --tag "land-$fixture_scenario" --runtime fake --owner-lineage land-receipt-fixture >/dev/null
  carried_epoch=$(date -u +%s)
  carried_now=$(date -u -r "$carried_epoch" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null) \
    || carried_now=$(date -u -d "@$carried_epoch" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null) \
    || { echo "land $fixture_scenario fixture: cannot format the run clock" >&2; exit 1; }
  carried_expired_epoch=$((carried_epoch + 7200))
  carried_expired_now=$(date -u -r "$carried_expired_epoch" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null) \
    || carried_expired_now=$(date -u -d "@$carried_expired_epoch" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null) \
    || { echo "land $fixture_scenario fixture: cannot format the advanced run clock" >&2; exit 1; }
  export METASYSTEM_GOAL_NOW=$carried_now
  if is_carried_two_seat_scenario || [[ "$fixture_scenario" == carried-crash-local ]]; then
    arm_receipt_runner "$leg_peer" "$leg_peer/bin/metasystem"
    peer_fixture_start=$(receipt_env_run "$leg_peer/bin/metasystem" proc started-at --pid "$$")
    receipt_env_run "$leg_peer/bin/metasystem" lease announce --root "$leg_peer" \
      --session "land-$fixture_scenario-peer" --pid "$$" --start "$peer_fixture_start" \
      --tag "land-$fixture_scenario-peer" --runtime fake --owner-lineage land-receipt-fixture-b >/dev/null
    receipt_env_run env METASYSTEM_GOAL_NOW=$carried_now METASYSTEM_OWNER_LINEAGE=land-receipt-fixture-b \
      "$leg_peer/bin/metasystem" goal open --root "$leg_peer" \
        --id fx-b --intent "Create the second seat's carried landing." \
        --next "Prove debt is visible between seats." \
        --risk severity=1,novelty=1,exposure=1,accumulation=1 \
        --basis "This disposable fixture serializes two carried landing seats." >/dev/null
    receipt_env_run env METASYSTEM_GOAL_NOW=$carried_now "$leg_peer/bin/metasystem" goal approve \
      --root "$leg_peer" --id fx-b --by Wido --lineage land-receipt-fixture-b \
      --elapsed-limit 4h --attempt-limit 4 --reserved-job-minutes-limit 4 \
      --active-job-limit 1 --review-round-limit 0 --fixture-human-authority >/dev/null
    receipt_env_run env METASYSTEM_GOAL_NOW=$carried_now METASYSTEM_OWNER_LINEAGE=land-receipt-fixture-b \
      "$leg_peer/bin/metasystem" goal claim --root "$leg_peer" \
        --id fx-b --lineage land-receipt-fixture-b >/dev/null
    git -C "$leg_local" fetch -q origin
    git -C "$leg_local" update-ref refs/metasystem/goals/accepted origin/main
    git -C "$leg_peer" fetch -q origin
    git -C "$leg_peer" update-ref refs/metasystem/goals/accepted origin/main
    select_receipt_runner_environment "$leg_local"
  fi
  carried_message=$leg_root/message.txt
  carried_output=$leg_root/land.out
  printf 'fixture carries one named testing group\n' >"$carried_message"
  if [[ "$fixture_scenario" == carried-red-battery ]]; then
    mkdir -p "$leg_local/records/misc"
    printf 'carried red battery record\n' >"$leg_local/records/misc/fx-red.md"
    git -C "$leg_local" add -- records/misc/fx-red.md
  else
    printf 'carried landing payload\n' >"$leg_local/payload.txt"
    git -C "$leg_local" add -- payload.txt
  fi
  if [[ "$fixture_scenario" == carried-ledger-path ]]; then
    printf 'ledger paths belong to goal verbs\n' >"$leg_local/plans/goals/illicit.md"
    git -C "$leg_local" add -- plans/goals/illicit.md
  fi
  if is_carried_two_seat_scenario; then
    printf 'carried landing payload\n' >"$leg_peer/payload.txt"
    printf 'second seat payload\n' >"$leg_peer/payload-b.txt"
    git -C "$leg_peer" add -- payload.txt payload-b.txt
  fi
  carried_tree=$(git -C "$leg_local" write-tree)
	carried_word_past=missing-declaration
	[[ "$fixture_scenario" != carried-asks ]] || carried_word_past=conflicting-declarations
	[[ "$fixture_scenario" != carried-red-battery ]] || carried_word_past=group:fixture-carry
	carried_expiry=4h
	[[ "$fixture_scenario" != carried-debt-expired ]] || carried_expiry=1h
  carried_word_output=$(METASYSTEM_GOAL_NOW=$carried_now METASYSTEM_OWNER_LINEAGE=land-receipt-fixture "$source_engine" goal carry \
	  --root "$leg_local" --id fx --by Wido --tree "$carried_tree" \
	  --past "$carried_word_past" --why "fixture carries one named landing refusal" \
	  --expires "$carried_expiry" --raise-format --fixture-human-authority)
  carried_word=$(sed -n 's/^carry=\([^ ]*\) workspace=.*/\1/p' <<<"$carried_word_output")
  [[ -n "$carried_word" ]] \
    || { echo "land $fixture_scenario fixture: goal carry returned no word" >&2; exit 1; }
  if is_carried_two_seat_scenario; then
    git -C "$leg_peer" fetch -q origin
    git -C "$leg_peer" update-ref refs/metasystem/goals/accepted origin/main
    peer_tree=$(git -C "$leg_peer" write-tree)
    peer_word_output=$(METASYSTEM_GOAL_NOW=$carried_now METASYSTEM_OWNER_LINEAGE=land-receipt-fixture-b \
      "$source_engine" goal carry --root "$leg_peer" --id fx-b --by Wido --tree "$peer_tree" \
        --past missing-declaration --why "fixture carries the second seat's named refusal" \
        --expires 4h --fixture-human-authority)
    peer_word=$(sed -n 's/^carry=\([^ ]*\) workspace=.*/\1/p' <<<"$peer_word_output")
    [[ -n "$peer_word" ]] \
      || { echo "land $fixture_scenario fixture: second seat goal carry returned no word" >&2; exit 1; }
  fi
  set +e
  receipt_env_run "$leg_local/bin/metasystem" test run --root "$leg_local" --goal fx \
    --tree "$carried_tree" --mode auto >"$leg_root/carried-test.out" 2>&1
  carried_test_rc=$?
  set -e
  if [[ "$fixture_scenario" == carried-red-battery ]]; then
	[[ $carried_test_rc -ne 0 ]] && grep -Fq 'fixture-carry' "$leg_root/carried-test.out" \
	  || { echo "land carried-red-battery fixture: the supporting battery was not red for fixture-carry" >&2; sed -n '1,240p' "$leg_root/carried-test.out" >&2; exit 1; }
  else
	[[ $carried_test_rc -eq 0 ]] \
	  || { echo "land $fixture_scenario fixture: the supporting green battery did not pass" >&2; sed -n '1,240p' "$leg_root/carried-test.out" >&2; exit 1; }
  fi
  if is_carried_two_seat_scenario; then
    select_receipt_runner_environment "$leg_peer"
    set +e
    receipt_env_run env METASYSTEM_GOAL_NOW=$carried_now \
      "$leg_peer/bin/metasystem" test run --root "$leg_peer" --goal fx-b \
      --tree "$peer_tree" --mode auto >"$leg_root/peer-test.out" 2>&1
    peer_test_rc=$?
    set -e
    [[ $peer_test_rc -eq 0 ]] \
      || { echo "land $fixture_scenario fixture: the second seat's green battery did not pass" >&2; sed -n '1,240p' "$leg_root/peer-test.out" >&2; exit 1; }
  fi
  if [[ "$fixture_scenario" != carried-second ]]; then
    while (( ${#receipt_runner_checkouts[@]} )); do
      stop_receipt_runner
    done
  fi
  rm -f -- "$leg_local/records/narrator-digest.log"

  if [[ "$fixture_scenario" == carried-intent-failure || "$fixture_scenario" == carried-crash-local ]]; then
	intent_failure_output=$leg_root/intent-failure.out
	crash_variable=METASYSTEM_LAND_FIXTURE_CRASH
	crash_marker='FIXTURE-CRASH before-push pid='
	if [[ "$fixture_scenario" == carried-crash-local ]]; then
	  crash_variable=METASYSTEM_LAND_FIXTURE_KILL
	  crash_marker='FIXTURE-KILL before-push pid='
	fi
	crash_environment=("$crash_variable=before-push")
	if [[ "$fixture_scenario" == carried-crash-local ]]; then
	  crash_environment+=(GIT_AUTHOR_DATE=2001-01-01T00:00:00Z)
	fi
	set +e
	(cd "$leg_local" && METASYSTEM_OWNER_LINEAGE=land-receipt-fixture \
	  env "${crash_environment[@]}" bash scripts/agents/land.sh \
	    -m "$carried_message" --goal fx --carried "$carried_word" \
	    --staged-only --skip-transport) >"$intent_failure_output" 2>&1
	intent_failure_rc=$?
	set -e
	[[ $intent_failure_rc -ne 0 ]] && grep -Fq "$crash_marker" "$intent_failure_output" \
	  || { echo "land $fixture_scenario fixture: before-push crash seam was not reached" >&2; cat "$intent_failure_output" >&2; exit 1; }
	local_commit=$(git -C "$leg_local" rev-parse HEAD)
	read -r first_candidate_payload_mode first_candidate_payload_type first_candidate_payload_blob first_candidate_payload_path \
	  <<<"$(git -C "$leg_local" ls-tree "$carried_tree" -- payload.txt)"
	[[ "$first_candidate_payload_type" == blob && "$first_candidate_payload_path" == payload.txt ]] \
	  || { echo "land carried-crash-local fixture: first staged payload tree is unreadable" >&2; exit 1; }
	crashed_author_date=$(git -C "$leg_local" show -s --format=%aI "$local_commit")
	crashed_carry_line=$(git -C "$leg_local" show -s --format=%B "$local_commit" | grep '^Carry: ')
	crashed_provenance_line=$(git -C "$leg_local" show -s --format=%B "$local_commit" | grep '^Landing-Provenance: ')
	[[ $(git -C "$leg_local" log -1 --format=%B) == *"Carry: $carried_word"* ]] \
	  || { echo "land $fixture_scenario fixture: local carried commit is absent" >&2; exit 1; }
	[[ $(git -C "$leg_remote" rev-parse refs/heads/main) != "$local_commit" ]] \
	  || { echo "land $fixture_scenario fixture: before-push crash moved origin" >&2; exit 1; }
	carried_entry_file=
	for candidate in "$leg_local"/artifacts/agents/goal-transactions/*.json; do
	  [[ -f "$candidate" ]] || continue
	  [[ $("$source_engine" json get --file "$candidate" --field intent.verb --default '') == carried ]] || continue
	  carried_entry_file=$candidate
	done
	[[ -n "$carried_entry_file" ]] \
	  || { echo "land $fixture_scenario fixture: no carried intent entry was written" >&2; exit 1; }
	intent_goal_text=$(git -C "$leg_local" show refs/metasystem/goals/accepted:plans/goals/fx.md)
	intent_reservation=$(awk -v ref="$carried_word" '$0 ~ " carrying " && $0 ~ "approvedRef=" ref && $0 ~ "reason=open " { print $3 }' <<<"$intent_goal_text")
	if [[ "$fixture_scenario" == carried-intent-failure ]]; then
	  [[ $("$source_engine" json get --file "$carried_entry_file" --field phase) == terminal ]] \
	    || { echo "land carried-intent-failure fixture: the wrapper-owned carried intent stayed open" >&2; cat "$carried_entry_file" >&2; exit 1; }
	  [[ -n "$intent_reservation" ]] && grep -Fq "reason=abandoned of=$intent_reservation " <<<"$intent_goal_text" \
	    || { echo "land carried-intent-failure fixture: trap did not abandon its reservation after closing the intent" >&2; printf '%s\n' "$intent_goal_text" >&2; exit 1; }
	  echo "land carried-intent-failure fixture passed"
	  exit 0
	fi
	[[ $("$source_engine" json get --file "$carried_entry_file" --field phase) == created ]] \
	  || { echo "land carried-crash-local fixture: killed wrapper did not leave its intent created" >&2; cat "$carried_entry_file" >&2; exit 1; }
	[[ -n "$intent_reservation" ]] \
	  || { echo "land carried-crash-local fixture: killed wrapper left no open reservation" >&2; printf '%s\n' "$intent_goal_text" >&2; exit 1; }
	! grep -Fq "reason=abandoned of=$intent_reservation " <<<"$intent_goal_text" \
	  || { echo "land carried-crash-local fixture: killed wrapper wrote an abandonment row" >&2; printf '%s\n' "$intent_goal_text" >&2; exit 1; }
	git -C "$leg_local" diff --cached --quiet \
	  || { echo "land carried-crash-local fixture: local recovery unexpectedly has staged bytes" >&2; exit 1; }
	prepare_receipt_environment \
	  "$leg_root/process-identities.$(basename "$leg_peer").json" \
	  "$leg_root/registry.$(basename "$leg_peer")"
	receipt_env_run env METASYSTEM_GOAL_NOW=$carried_now METASYSTEM_OWNER_LINEAGE=land-receipt-fixture-b \
	  "$leg_peer/bin/metasystem" goal edit --root "$leg_peer" --id fx-b \
	    --next "Keep the carried recovery word valid across this ledger move." \
	    --lineage land-receipt-fixture-b >/dev/null
	moved_origin_tip=$(git -C "$leg_remote" rev-parse refs/heads/main)
	moved_origin_paths=$(git -C "$leg_remote" diff-tree --no-commit-id --name-only -r \
	  "$moved_origin_tip^" "$moved_origin_tip")
	[[ "$moved_origin_paths" == plans/goals/fx-b.md ]] \
	  || { echo "land carried-crash-local fixture: peer move was not ledger-only: $moved_origin_paths" >&2; exit 1; }
	local_rerun_output=$leg_root/local-rerun.out
	(cd "$leg_local" && METASYSTEM_OWNER_LINEAGE=land-receipt-fixture \
	  bash scripts/agents/land.sh -m "$carried_message" --goal fx --carried "$carried_word" \
	    --skip-transport) >"$local_rerun_output" 2>&1 \
	  || { echo "land carried-crash-local fixture: local recovery did not finish" >&2; cat "$local_rerun_output" >&2; exit 1; }
	git -C "$leg_local" fetch -q origin
	origin_code_commits=$(git -C "$leg_local" log --format=%H --grep="^Carry: $carried_word$" refs/remotes/origin/main)
	[[ $(wc -w <<<"$origin_code_commits" | tr -d ' ') -eq 1 ]] \
	  || { echo "land carried-crash-local fixture: origin does not hold exactly one carried code commit" >&2; printf '%s\n' "$origin_code_commits" >&2; exit 1; }
	origin_code_commit=$(head -n 1 <<<"$origin_code_commits")
	[[ $(git -C "$leg_local" rev-parse "$origin_code_commit^") == "$moved_origin_tip" ]] \
	  || { echo "land carried-crash-local fixture: recovered code commit is not based on the moved origin tip" >&2; exit 1; }
	[[ $(git -C "$leg_local" show -s --format=%aI "$origin_code_commit") == "$crashed_author_date" ]] \
	  || { echo "land carried-crash-local fixture: recovery restamped the crashed commit's author date" >&2; exit 1; }
	[[ $(git -C "$leg_local" show -s --format=%B "$origin_code_commit" | grep '^Carry: ') == "$crashed_carry_line" ]] \
	  || { echo "land carried-crash-local fixture: recovery changed the Carry trailer" >&2; exit 1; }
	[[ $(git -C "$leg_local" show -s --format=%B "$origin_code_commit" | grep '^Landing-Provenance: ') == "$crashed_provenance_line" ]] \
	  || { echo "land carried-crash-local fixture: recovery changed the Landing-Provenance line" >&2; exit 1; }
	[[ -z $(git -C "$leg_local" for-each-ref --format='%(refname)' --contains "$local_commit" refs/heads refs/remotes) ]] \
	  || { echo "land carried-crash-local fixture: crashed commit remains on a branch after recovery" >&2; exit 1; }
	expected_rebased_index=$leg_root/expected-rebased.index
	GIT_INDEX_FILE=$expected_rebased_index git -C "$leg_local" read-tree "$moved_origin_tip"
	GIT_INDEX_FILE=$expected_rebased_index git -C "$leg_local" update-index --add \
	  --cacheinfo "$first_candidate_payload_mode,$first_candidate_payload_blob,payload.txt"
	expected_rebased_tree=$(GIT_INDEX_FILE=$expected_rebased_index git -C "$leg_local" write-tree)
	rm -f -- "$expected_rebased_index"
	origin_code_tree=$(git -C "$leg_local" rev-parse "$origin_code_commit^{tree}")
	[[ "$origin_code_tree" == "$expected_rebased_tree" ]] \
	  || { echo "land carried-crash-local fixture: recovered code commit changed the staged candidate payload tree" >&2; exit 1; }
	local_goal_text=$(git -C "$leg_local" show refs/metasystem/goals/accepted:plans/goals/fx.md)
	[[ $(grep -Ec "^- [^ ]+ [^ ]+ carrying .*approvedRef=$carried_word .*reason=open " <<<"$local_goal_text") -eq 1 ]] \
	  || { echo "land carried-crash-local fixture: goal does not hold exactly one carrying row" >&2; printf '%s\n' "$local_goal_text" >&2; exit 1; }
	[[ $(grep -Ec "^- [^ ]+ [^ ]+ carried .*approvedRef=$carried_word .*reason=landed " <<<"$local_goal_text") -eq 1 ]] \
	  || { echo "land carried-crash-local fixture: goal does not hold exactly one carried row" >&2; printf '%s\n' "$local_goal_text" >&2; exit 1; }
	[[ $(wc -l <"$leg_local/records/counselor/carried-landings.jsonl" | tr -d ' ') -eq 1 ]] \
	  || { echo "land carried-crash-local fixture: counselor record was not written exactly once" >&2; exit 1; }
	echo "land carried-crash-local fixture passed"
	exit 0
  fi

  if [[ "$fixture_scenario" == carried-crash ]]; then
	crash_output=$leg_root/crash.out
	rerun_output=$leg_root/rerun.out
	set +e
	(cd "$leg_local" && METASYSTEM_OWNER_LINEAGE=land-receipt-fixture \
	  METASYSTEM_LAND_FIXTURE_CRASH=after-push \
	  bash scripts/agents/land.sh -m "$carried_message" --goal fx --carried "$carried_word" \
	    --staged-only --skip-transport) >"$crash_output" 2>&1
	crash_rc=$?
	set -e
	[[ $crash_rc -ne 0 ]] && grep -Fq 'FIXTURE-CRASH after-push pid=' "$crash_output" \
	  || { echo "land carried-crash fixture: after-push crash seam was not reached" >&2; cat "$crash_output" >&2; exit 1; }
	crashed_commit=$(git -C "$leg_remote" log -1 --format=%H refs/heads/main)
	[[ $(git -C "$leg_remote" log -1 --format=%B refs/heads/main) == *"Carry: $carried_word"* ]] \
	  || { echo "land carried-crash fixture: carried commit was not pushed before the crash" >&2; exit 1; }
	crash_goal_text=$(git -C "$leg_local" show refs/metasystem/goals/accepted:plans/goals/fx.md)
	! grep -Fq " carried " <<<"$crash_goal_text" \
	  || { echo "land carried-crash fixture: carried row existed before the crash" >&2; exit 1; }
	git -C "$leg_local" diff --cached --quiet \
	  || { echo "land carried-crash fixture: crash rerun unexpectedly has a staged set" >&2; exit 1; }
	(cd "$leg_local" && METASYSTEM_OWNER_LINEAGE=land-receipt-fixture \
	  bash scripts/agents/land.sh -m "$carried_message" --goal fx --carried "$carried_word" \
	    --skip-transport) >"$rerun_output" 2>&1 \
	  || { echo "land carried-crash fixture: rerun did not complete the record" >&2; cat "$rerun_output" >&2; exit 1; }
	grep -Fq "already landed as $crashed_commit; completing the record" "$rerun_output" \
	  || { echo "land carried-crash fixture: rerun did not take the origin recovery branch" >&2; cat "$rerun_output" >&2; exit 1; }
	[[ $(git -C "$leg_remote" log --format=%H --grep="^Carry: $carried_word$" refs/heads/main | wc -l | tr -d ' ') == 1 ]] \
	  || { echo "land carried-crash fixture: recovery wrote another carried commit" >&2; exit 1; }
	crash_goal_text=$(git -C "$leg_local" show refs/metasystem/goals/accepted:plans/goals/fx.md)
	[[ $(grep -Fc " approvedRef=$carried_word " <<<"$crash_goal_text") -ge 2 ]] \
	  || { echo "land carried-crash fixture: reservation and carried rows are incomplete" >&2; printf '%s\n' "$crash_goal_text" >&2; exit 1; }
	[[ -s "$leg_local/records/counselor/carried-landings.jsonl" ]] \
	  || { echo "land carried-crash fixture: counselor line is absent after recovery" >&2; exit 1; }
	echo "land carried-crash fixture passed"
	exit 0
  fi

  if is_carried_two_seat_scenario; then
	crash_output=$leg_root/seat-a-crash.out
	peer_output=$leg_root/seat-b.out
	run_clock=$carried_now
	set +e
	(cd "$leg_local" && METASYSTEM_GOAL_NOW=$carried_now \
	  METASYSTEM_OWNER_LINEAGE=land-receipt-fixture METASYSTEM_LAND_FIXTURE_CRASH=after-push \
	  bash scripts/agents/land.sh -m "$carried_message" --goal fx --carried "$carried_word" \
	    --staged-only --skip-transport) >"$crash_output" 2>&1
	crash_rc=$?
	set -e
	[[ $crash_rc -ne 0 ]] && grep -Fq 'FIXTURE-CRASH after-push pid=' "$crash_output" \
	  || { echo "land $fixture_scenario fixture: seat A did not crash after its push" >&2; cat "$crash_output" >&2; exit 1; }
	seat_a_commit=$(git -C "$leg_remote" log -1 --format=%H refs/heads/main)
	[[ $(git -C "$leg_remote" log -1 --format=%B refs/heads/main) == *"Carry: $carried_word"* ]] \
	  || { echo "land $fixture_scenario fixture: seat A's commit is absent from origin" >&2; exit 1; }
	seat_a_goal=$(git -C "$leg_local" show refs/metasystem/goals/accepted:plans/goals/fx.md)
	seat_a_row=$(awk -v ref="$carried_word" '$0 ~ " carrying " && $0 ~ "approvedRef=" ref && $0 ~ "reason=open " { print $3 }' <<<"$seat_a_goal")
	[[ -n "$seat_a_row" ]] && ! grep -Fq " carried " <<<"$seat_a_goal" \
	  || { echo "land $fixture_scenario fixture: seat A is not in the pushed-before-record interval" >&2; printf '%s\n' "$seat_a_goal" >&2; exit 1; }
	if [[ "$fixture_scenario" == carried-debt-abandoned ]]; then
	  METASYSTEM_GOAL_NOW=$carried_now METASYSTEM_OWNER_LINEAGE=land-receipt-fixture \
	    "$source_engine" goal carrying --root "$leg_local" --id fx --abandon "$seat_a_row" \
	      --why "fixture releases the crashed reservation" >/dev/null
	  seat_a_status=$(METASYSTEM_GOAL_NOW=$carried_now "$source_engine" landing carry-status \
	    --root "$leg_local" --carried "$carried_word" --goal fx \
	    --ledger-tip "$(git -C "$leg_local" rev-parse refs/metasystem/goals/accepted)")
	  grep -Fq "reservation: abandoned:$seat_a_row" <<<"$seat_a_status" \
	    || { echo "land carried-debt-abandoned fixture: seat A's row was not abandoned" >&2; printf '%s\n' "$seat_a_status" >&2; exit 1; }
	elif [[ "$fixture_scenario" == carried-debt-expired ]]; then
	  run_clock=$carried_expired_now
	  seat_a_status=$(METASYSTEM_GOAL_NOW=$run_clock "$source_engine" landing carry-status \
	    --root "$leg_local" --carried "$carried_word" --goal fx \
	    --ledger-tip "$(git -C "$leg_local" rev-parse refs/metasystem/goals/accepted)")
	  grep -Fxq 'expired' <<<"$seat_a_status" && grep -Fq "reservation: expired:$seat_a_row" <<<"$seat_a_status" \
	    || { echo "land carried-debt-expired fixture: the advanced clock did not expire seat A's word and row" >&2; printf '%s\n' "$seat_a_status" >&2; exit 1; }
	fi
	set +e
	(cd "$leg_peer" && METASYSTEM_GOAL_NOW=$run_clock METASYSTEM_OWNER_LINEAGE=land-receipt-fixture-b \
	  bash scripts/agents/land.sh -m "$carried_message" --goal fx-b --carried "$peer_word" \
	    --staged-only --skip-transport) >"$peer_output" 2>&1
	peer_rc=$?
	set -e
	[[ $peer_rc -eq 3 ]] && grep -Fq 'carry-debt-unpaid' "$peer_output" \
	  || { echo "land $fixture_scenario fixture: seat B did not ask on seat A's debt" >&2; cat "$peer_output" >&2; exit 1; }
	if [[ "$fixture_scenario" == carried-two-seat ]]; then
	  grep -Fq "$seat_a_row" "$peer_output" && grep -Fq 'seat=fixture-machine' "$peer_output" \
	    || { echo "land carried-two-seat fixture: in-flight ask did not name seat A's row and seat" >&2; cat "$peer_output" >&2; exit 1; }
	else
	  grep -Fq "$carried_word" "$peer_output" \
	    || { echo "land $fixture_scenario fixture: trailer debt ask did not name seat A's word" >&2; cat "$peer_output" >&2; exit 1; }
	fi
	! git -C "$leg_remote" log --format=%B refs/heads/main | grep -Fq "Carry: $peer_word" \
	  || { echo "land $fixture_scenario fixture: seat B pushed a commit despite seat A's debt" >&2; exit 1; }
	git -C "$leg_peer" fetch -q origin
	git -C "$leg_peer" update-ref refs/metasystem/goals/accepted origin/main
	seat_b_goal=$(git -C "$leg_peer" show refs/metasystem/goals/accepted:plans/goals/fx-b.md)
	! grep -Fq " carrying " <<<"$seat_b_goal" \
	  || { echo "land $fixture_scenario fixture: seat B wrote a reservation despite seat A's debt" >&2; printf '%s\n' "$seat_b_goal" >&2; exit 1; }
	seat_a_rerun=$leg_root/seat-a-rerun.out
	(cd "$leg_local" && METASYSTEM_GOAL_NOW=$run_clock METASYSTEM_OWNER_LINEAGE=land-receipt-fixture \
	  bash scripts/agents/land.sh -m "$carried_message" --goal fx --carried "$carried_word" \
	    --staged-only --skip-transport) >"$seat_a_rerun" 2>&1 \
	  || { echo "land $fixture_scenario fixture: seat A did not complete its pushed record" >&2; cat "$seat_a_rerun" >&2; exit 1; }
	grep -Fq "already landed as $seat_a_commit; completing the record" "$seat_a_rerun" \
	  || { echo "land $fixture_scenario fixture: seat A rerun missed the origin recovery branch" >&2; cat "$seat_a_rerun" >&2; exit 1; }
	if [[ "$fixture_scenario" == carried-two-seat ]]; then
	  set +e
	  (cd "$leg_peer" && METASYSTEM_GOAL_NOW=$run_clock METASYSTEM_OWNER_LINEAGE=land-receipt-fixture-b \
	    bash scripts/agents/land.sh -m "$carried_message" --goal fx-b --carried "$peer_word" \
	      --staged-only --skip-transport) >"$leg_root/seat-b-rerun.out" 2>&1
	  peer_rerun_rc=$?
	  set -e
	  [[ $peer_rerun_rc -eq 3 ]] && grep -Fq 'carry-debt-unpaid' "$leg_root/seat-b-rerun.out" \
	    && grep -Fq "carried:$seat_a_commit" "$leg_root/seat-b-rerun.out" \
	    || { echo "land carried-two-seat fixture: seat B rerun did not name seat A's review obligation" >&2; cat "$leg_root/seat-b-rerun.out" >&2; exit 1; }
	fi
	echo "land $fixture_scenario fixture passed"
	exit 0
  fi

  if [[ "$fixture_scenario" == carried-asks || "$fixture_scenario" == carried-ledger-path ]]; then
    set +e
    (cd "$leg_local" && METASYSTEM_OWNER_LINEAGE=land-receipt-fixture \
      bash scripts/agents/land.sh -m "$carried_message" --goal fx --carried "$carried_word" \
        --staged-only --skip-transport) >"$carried_output" 2>&1
    carried_rc=$?
    set -e
	if [[ "$fixture_scenario" == carried-asks ]]; then
	  [[ $carried_rc -eq 3 ]] && grep -Fq 'the landing saw ordinary=missing-declaration' "$carried_output" \
	    || { echo "land carried-asks fixture: the mismatched refusal did not ask" >&2; cat "$carried_output" >&2; exit 1; }
	else
	  [[ $carried_rc -eq 3 ]] && grep -Fq 'ledger-path-not-goal-verb' "$carried_output" \
	    || { echo "land carried-ledger-path fixture: the ledger path was not refused by name" >&2; cat "$carried_output" >&2; exit 1; }
	fi
	carried_goal_text=$(git -C "$leg_local" show refs/metasystem/goals/accepted:plans/goals/fx.md)
	carried_reservation=$(awk -v ref="$carried_word" '$0 ~ " carrying " && $0 ~ "approvedRef=" ref && $0 ~ "reason=open " { print $3 }' <<<"$carried_goal_text")
	[[ -n "$carried_reservation" ]] && grep -Fq "reason=abandoned of=$carried_reservation " <<<"$carried_goal_text" \
	  || { echo "land carried-asks fixture: the wrapper did not abandon its reservation" >&2; printf '%s\n' "$carried_goal_text" >&2; exit 1; }
    echo "land $fixture_scenario fixture passed"
    exit 0
  fi

  carried_land_args=(-m "$carried_message" --goal fx --carried "$carried_word" --staged-only --skip-transport)
  [[ "$fixture_scenario" != carried-red-battery ]] || carried_land_args+=(--direct-fix register-carriage)
  (cd "$leg_local" && METASYSTEM_OWNER_LINEAGE=land-receipt-fixture \
    bash scripts/agents/land.sh "${carried_land_args[@]}") >"$carried_output" 2>&1 || {
	  echo "land $fixture_scenario fixture: carried landing did not complete" >&2
	  echo "land $fixture_scenario fixture: retained test run output" >&2
	  [[ ! -f "$leg_root/carried-test.out" ]] || sed -n '1,240p' "$leg_root/carried-test.out" >&2
	  sed -n '1,240p' "$carried_output" >&2
	  exit 1
	}
  carried_commit=$(git -C "$leg_local" log --format=%H --grep="^Carry: $carried_word$" -1)
  [[ "$carried_commit" =~ ^[0-9a-f]{40}$ ]] \
    || { echo "land $fixture_scenario fixture: no carried commit is reachable" >&2; exit 1; }
  carried_body=$(git -C "$leg_local" log -1 --format=%B "$carried_commit")
  for key in Carry Carried-By Carried-Tree Carried-Past Carried-Battery Carried-Judge Carried-Ledger Landing-Provenance; do
    [[ $(grep -c "^$key:" <<<"$carried_body") -eq 1 ]] \
      || { echo "land $fixture_scenario fixture: $key trailer is not singular" >&2; exit 1; }
  done
  carried_status=$("$source_engine" landing carry-status --root "$leg_local" --carried "$carried_word" \
    --goal fx --ledger-tip "$(git -C "$leg_local" rev-parse refs/metasystem/goals/accepted)")
	grep -q '^ledger:' <<<"$carried_status" \
	  || { echo "land $fixture_scenario fixture: carried row did not close the word: $carried_status" >&2; exit 1; }
  grep -Fq 'reservation: closed:' <<<"$carried_status" \
    || { echo "land $fixture_scenario fixture: reservation did not close" >&2; exit 1; }
  grep -Fq '== STEP: complete carried goal record' "$carried_output"
	previous_line=0
	push_line=$(grep -nF '== STEP: push carried commit to origin (single attempt)' "$carried_output" | head -1 | cut -d: -f1)
	[[ "$push_line" =~ ^[0-9]+$ ]] \
	  || { echo "land carried-fresh fixture: push step is absent" >&2; cat "$carried_output" >&2; exit 1; }
	for advisory in \
	  'carried reservation:' \
	  'carried ledger:' \
	  'carried judge:' \
	  'carried live failure:' \
	  'carried ordinary verdict:' \
	  'carried testing result:' \
	  'carried obligation finding:' \
	  'carried exception count after this one:'; do
	  advisory_line=$(grep -nF "$advisory" "$carried_output" | head -1 | cut -d: -f1)
	  [[ "$advisory_line" =~ ^[0-9]+$ && $advisory_line -gt $previous_line && $advisory_line -lt $push_line ]] \
	    || { echo "land carried-fresh fixture: advisory '$advisory' is absent or out of order" >&2; cat "$carried_output" >&2; exit 1; }
	  previous_line=$advisory_line
	done
  [[ -s "$leg_local/records/counselor/carried-landings.jsonl" ]] \
    || { echo "land $fixture_scenario fixture: counselor line is absent" >&2; exit 1; }
  if [[ "$fixture_scenario" == carried-red-battery ]]; then
	grep -Eq '^Carried-Battery: red missing=[^[:space:]]+ failing=' <<<"$carried_body" \
	  || { echo "land carried-red-battery fixture: commit.sh did not record the red group battery" >&2; printf '%s\n' "$carried_body" >&2; exit 1; }
	red_goal_text=$(git -C "$leg_local" show refs/metasystem/goals/accepted:plans/goals/fx.md)
	grep -Fq "finding=carried:$carried_commit:battery-red chain=human-carried" <<<"$red_goal_text" \
	  || { echo "land carried-red-battery fixture: red carried obligation is absent" >&2; printf '%s\n' "$red_goal_text" >&2; exit 1; }
	grep -Fq "carried obligation finding: carried:$carried_commit:battery-red" "$carried_output" \
	  || { echo "land carried-red-battery fixture: advisory did not name the red obligation" >&2; cat "$carried_output" >&2; exit 1; }
  fi
  if [[ "$fixture_scenario" == carried-second ]]; then
	first_counselor_line=$(sed -n '1p' "$leg_local/records/counselor/carried-landings.jsonl")
	[[ -n "$first_counselor_line" ]] \
	  || { echo "land carried-second fixture: first counselor line is absent" >&2; exit 1; }
	METASYSTEM_GOAL_NOW=$carried_now METASYSTEM_OWNER_LINEAGE=land-receipt-fixture \
	  "$source_engine" goal accept-risk --root "$leg_local" --id fx \
	    --finding "carried:$carried_commit" --chain human-carried --by Wido \
	    --why "fixture closes the first carried review before the second landing" \
	    --fixture-human-authority >/dev/null
	accepted_risk_line=$(sed -n '1p' "$leg_local/records/counselor/accepted-risk-register.jsonl")
	[[ -n "$accepted_risk_line" ]] \
	  || { echo "land carried-second fixture: accepted-risk counselor line is absent" >&2; exit 1; }
	printf 'second carried landing payload\n' >"$leg_local/payload.txt"
	git -C "$leg_local" add -- payload.txt records/counselor/accepted-risk-register.jsonl records/counselor/carried-landings.jsonl
	second_tree=$(git -C "$leg_local" write-tree)
	select_receipt_runner_environment "$leg_local"
	receipt_env_run "$leg_local/bin/metasystem" test run --root "$leg_local" --goal fx \
	  --tree "$second_tree" --mode auto >"$leg_root/second-test.out" 2>&1 \
	  || { echo "land carried-second fixture: second battery did not pass" >&2; sed -n '1,240p' "$leg_root/second-test.out" >&2; exit 1; }
	while (( ${#receipt_runner_checkouts[@]} )); do
	  stop_receipt_runner
	done
	second_word_output=$(METASYSTEM_GOAL_NOW=$carried_now METASYSTEM_OWNER_LINEAGE=land-receipt-fixture \
	  "$source_engine" goal carry --root "$leg_local" --id fx --by Wido --tree "$second_tree" \
	    --past missing-declaration --why "fixture carries its prior counselor line" \
	    --expires 4h --fixture-human-authority)
	second_word=$(sed -n 's/^carry=\([^ ]*\) workspace=.*/\1/p' <<<"$second_word_output")
	[[ -n "$second_word" ]] \
	  || { echo "land carried-second fixture: second goal carry returned no word" >&2; exit 1; }
	second_output=$leg_root/second-land.out
	(cd "$leg_local" && METASYSTEM_OWNER_LINEAGE=land-receipt-fixture \
	  bash scripts/agents/land.sh -m "$carried_message" --goal fx --carried "$second_word" \
	    --staged-only --skip-transport) >"$second_output" 2>&1 \
	  || { echo "land carried-second fixture: second landing failed" >&2; cat "$second_output" >&2; exit 1; }
	second_commit=$(git -C "$leg_local" log --format=%H --grep="^Carry: $second_word$" -1)
	first_record=$(git -C "$leg_local" show "$second_commit:records/counselor/carried-landings.jsonl")
	grep -Fq "$first_counselor_line" <<<"$first_record" \
	  || { echo "land carried-second fixture: second commit did not carry the first counselor line" >&2; exit 1; }
	second_risk_record=$(git -C "$leg_local" show "$second_commit:records/counselor/accepted-risk-register.jsonl")
	grep -Fqx "$accepted_risk_line" <<<"$second_risk_record" \
	  || { echo "land carried-second fixture: second commit did not carry the paid-debt counselor line" >&2; exit 1; }
	[[ $(wc -l <"$leg_local/records/counselor/carried-landings.jsonl" | tr -d ' ') -eq 2 ]] \
	  || { echo "land carried-second fixture: counselor record is not append-once across both landings" >&2; exit 1; }
  fi
  echo "land $fixture_scenario fixture passed"
  exit 0
fi

prepare_receipt_chain() { # checkout, chain, candidate tree
  local checkout=$1 chain=$2 candidate_tree=$3 round_root
  round_root=$checkout/artifacts/agents/$chain/rounds/1
  mkdir -p "$checkout/artifacts/agents/jobs" "$round_root"
  cat >"$checkout/artifacts/agents/jobs/$chain.json" <<JSON
{
  "jobId": "$chain",
  "goalId": null,
  "parentJob": null,
  "role": "implementer",
  "round": 1,
  "status": "completed",
  "goalTier": 1,
  "gateWidth": "area",
  "destructiveReach": "DESTRUCTIVE-REACH",
  "chainClosed": true
}
JSON
  git -C "$checkout" diff --cached --binary --full-index --no-ext-diff --no-textconv -- >"$round_root/diff.patch"
  cat >"$round_root/review.json" <<JSON
{
  "diffArtifact": "diff.patch",
  "implementerJob": "$chain",
  "reviewedTree": "$candidate_tree"
}
JSON
}

take_fixture_receipt() { # engine, checkout, output log
  local engine=$1 checkout=$2 output=$3
  fixture_receipt_tree=$(git -C "$checkout" write-tree)
  fixture_receipt_path=$checkout/artifacts/agents/landing/receipts/$fixture_receipt_tree.json
  if ! receipt_checkout_env_run "$checkout" "$engine" landing test-receipt --root "$checkout" \
      --tree "$fixture_receipt_tree" --mode auto --goal fx --cap-min 1 >"$output" 2>&1; then
    echo "land $fixture_scenario fixture: landing test-receipt failed" >&2
    sed -n '1,240p' "$output" >&2
    return 1
  fi
  [[ -s "$fixture_receipt_path" ]] || {
    echo "land $fixture_scenario fixture: landing test-receipt wrote no receipt for $fixture_receipt_tree" >&2
    return 1
  }
}

publish_peer_ledger_move() { # optional peer checkout
  local checkout=${1:-$leg_peer} engine peer_start goal_base accepted_base
  engine=$checkout/bin/metasystem
  git -C "$checkout" fetch -q origin
  git -C "$checkout" reset -q --hard origin/main
  goal_base=origin/main
  accepted_base=origin/main
  if git -C "$checkout" rev-parse --verify -q origin/metasystem/goals >/dev/null; then
    goal_base=origin/metasystem/goals
    accepted_base=origin/metasystem/accepted
  fi
  git -C "$checkout" update-ref refs/heads/metasystem/goals "$goal_base"
  git -C "$checkout" update-ref refs/metasystem/goals/accepted "$accepted_base"
  peer_start=$(receipt_env_run "$engine" proc started-at --pid "$$")
  receipt_env_run "$engine" lease announce --root "$checkout" --session land-receipt-fixture-peer \
    --pid "$$" --start "$peer_start" --tag land-receipt-fixture-peer \
    --runtime fake --owner-lineage land-receipt-fixture-peer >/dev/null
  receipt_env_run env METASYSTEM_OWNER_LINEAGE=land-receipt-fixture-peer \
    "$engine" goal open --root "$checkout" --id peer-goal \
      --intent "Publish one fixture peer goal." \
      --next "Let the landing consume this ledger-only move." \
      --risk severity=1,novelty=1,exposure=1,accumulation=1 \
      --basis "This disposable fixture writes one isolated goal ledger commit." >/dev/null
  receipt_env_run "$engine" lease retire --root "$checkout" --session land-receipt-fixture-peer \
    --pid "$$" --start "$peer_start" >/dev/null
  git -C "$checkout" push -q origin refs/heads/metasystem/goals:refs/heads/main
}

publish_peer_records_move() {
  mkdir -p "$leg_peer/records/misc"
  printf 'fixture peer record\n' >"$leg_peer/records/misc/peer-note.md"
  git -C "$leg_peer" add -- records/misc/peer-note.md
  git -C "$leg_peer" commit -qm "peer publishes a records-only move"
  git -C "$leg_peer" push -q origin main
}

publish_peer_input_move() {
  printf 'fixture peer input move\n' >"$leg_peer/scripts/application-input.txt"
  git -C "$leg_peer" add -- scripts/application-input.txt
  git -C "$leg_peer" commit -qm "peer changes a declared testing input"
  git -C "$leg_peer" push -q origin main
}

run_fixture_landing() { # checkout, message, chain, receipt, output, chain log
  local checkout=$1 message=$2 chain=$3 receipt=$4 output=$5 chain_log=$6
  receipt_checkout_env_run "$checkout" env LAND_FIXTURE_CHAIN_LOG="$chain_log" \
    bash "$checkout/scripts/agents/land.sh" -m "$message" --chain "$chain" --goal fx \
      --test-receipt "$receipt" --staged-only --skip-transport >"$output" 2>&1
}

install_cutover_engine() { # checkout, engine
  local checkout=$1 engine=$2
  mkdir -p "$checkout/bin"
  cp "$engine" "$checkout/bin/metasystem"
  chmod +x "$checkout/bin/metasystem"
  if git -C "$checkout" ls-files --error-unmatch bin/metasystem >/dev/null 2>&1; then
    git -C "$checkout" update-index --assume-unchanged bin/metasystem
  fi
}

make_brain_source_leg() { # name
  local name=$1 source_top source_prefix legacy ledger digest manifest fixture_start migrate_out
  clear_independent_fixture_context
  # These disposable roots need their own witness because the parent's witness describes a different repository.
  unset METASYSTEM_GATE_WITNESS METASYSTEM_GATE_WITNESS_ROOT \
    METASYSTEM_GATE_WITNESS_RUN METASYSTEM_GATE_WITNESS_EXPORT \
    METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE METASYSTEM_GATE_WITNESS_WRITE \
    METASYSTEM_GATE_WITNESS_CONTROLLER_PID METASYSTEM_GATE_WITNESS_CONTROLLER_STARTED_AT \
    METASYSTEM_GATE_WITNESS_CONTROLLER_START_TICKS METASYSTEM_GATE_WITNESS_CONTROLLER_BOOT_ID \
    METASYSTEM_GATE_WITNESS_MANIFEST_DIGEST METASYSTEM_GATE_WITNESS_CONSUMER_EXPORT \
    METASYSTEM_GATE_WITNESS_REUSE_OUT
  leg_root=$tmp/$name
  leg_seed=$leg_root/seed
  leg_remote=$leg_root/origin.git
  leg_local=$leg_root/local
  leg_peer=$leg_root/peer
  source_top=$(git -C "$root" rev-parse --show-toplevel)
  source_prefix=$(git -C "$root" rev-parse --show-prefix)
  source_prefix=${source_prefix%/}
  mkdir -p "$leg_seed"
  if [[ -n "$source_prefix" ]]; then
    git -C "$source_top" archive "HEAD:$source_prefix" | tar -x -C "$leg_seed"
  else
    git -C "$source_top" archive HEAD | tar -x -C "$leg_seed"
  fi
  mkdir -p "$leg_seed/bin"
  cp "$root/scripts/agents/land.sh" "$leg_seed/scripts/agents/land.sh"
  cp "$root/scripts/agents/commit.sh" "$leg_seed/scripts/agents/commit.sh"
  cp "$source_engine" "$leg_seed/bin/metasystem"
  rm -rf "$leg_seed/plans/goals"
  rm -f "$leg_seed/plans/goals.md" "$leg_seed/plans/goals-accepted.json"
  mkdir -p "$leg_seed/plans" "$leg_seed/records/misc"
  cp "$root/records/misc/fleet-coordinator-brain-role-packet.md" "$leg_seed/records/misc/"
  cat >"$leg_seed/plans/goals.md" <<'LEDGER'
# Goals

## Current goal: ship-widget — Ship the widget
- Origin: main
- Next step: Let a node finish it.
LEDGER
  for fixture_template in "$leg_seed/docs/project-rules.md" "$leg_seed/metasystem.conf"; do
    awk '{ gsub(/<[^>]+>/, "fixture"); print }' "$fixture_template" >"$fixture_template.fixture"
    mv "$fixture_template.fixture" "$fixture_template"
  done
  awk '$0 !~ /^[[:space:]]*testing[.]contract[[:space:]]*=/' "$leg_seed/metasystem.conf" \
    >"$leg_seed/metasystem.conf.legacy"
  mv "$leg_seed/metasystem.conf.legacy" "$leg_seed/metasystem.conf"
  legacy=$(cat "$leg_seed/plans/goals.md" && printf x) && legacy=${legacy%x}
  "$source_engine" json object ledger="$legacy" sha256="$(shasum -a 256 "$leg_seed/plans/goals.md" | cut -d' ' -f1)" >"$leg_seed/plans/goals-accepted.json"
  "$source_engine" json set --file "$leg_seed/plans/goals-accepted.json" --int schemaVersion=1
  git -C "$leg_seed" init -q -b main
  git -C "$leg_seed" config user.name fixture
  git -C "$leg_seed" config user.email fixture@example.invalid
  git -C "$leg_seed" config metasystem.goal.machine brain-leg
  git -C "$leg_seed" add -A
  git -C "$leg_seed" add -f bin/metasystem
  git -C "$leg_seed" commit -qm seed
  git init -q --bare "$leg_remote"
  git -C "$leg_seed" remote add origin "$leg_remote"
  git -C "$leg_seed" push -q -u origin main
  git clone -q "$leg_remote" "$leg_local"
  git -C "$leg_local" config user.name fixture-local
  git -C "$leg_local" config user.email fixture-local@example.invalid
  git -C "$leg_local" config metasystem.goal.machine brain-leg
  fixture_start=$("$source_engine" proc started-at --pid "$$")
  leg_fixture_start=$fixture_start
  "$source_engine" lease announce --root "$leg_local" --session brain-land-fixture \
    --pid "$$" --start "$fixture_start" --tag brain-land-fixture --runtime fake --owner-lineage fixture-lineage >/dev/null
  digest=$("$source_engine" goal source-digest --root "$leg_local")
  manifest=$leg_root/migration.md
  cat >"$manifest" <<MANIFEST
# Queue amendments

MIGRATION_EPOCH: 2026-09-07T00:00:00Z
REVIEWED_SOURCE_SHA256: $digest
MANIFEST
  migrate_out=$(METASYSTEM_OWNER_LINEAGE=fixture-lineage "$source_engine" goal migrate --root "$leg_local" \
    --source-digest "$digest" --manifest "$manifest" --by Wido)
  leg_identity=$(sed -n 's/.*"identity": "\([^"]*\)".*/\1/p' <<<"$migrate_out" | head -1)
  [[ ${#leg_identity} -eq 26 ]] || { echo "brain land fixture migration reported no identity" >&2; exit 1; }
  git -C "$leg_local" fetch -q origin
  git -C "$leg_local" reset -q --hard origin/main
  git -C "$leg_local" update-ref refs/metasystem/goals/accepted origin/main
  METASYSTEM_OWNER_LINEAGE=fixture-lineage "$source_engine" goal release --root "$leg_local" --id ship-widget >/dev/null
  git -C "$leg_local" fetch -q origin
  git -C "$leg_local" reset -q --hard origin/main
  git -C "$leg_local" update-ref refs/metasystem/goals/accepted origin/main
}

declare_fixture_brain_temporarily() { # checkout
  local checkout=$1 saved=$leg_root/metasystem.conf.saved
  cp "$checkout/metasystem.conf" "$saved"
  printf '%s\n' 'metasystem.runtimes=fake' >"$checkout/metasystem.conf"
  "$source_engine" brain declare --root "$checkout" --by Wido --fixture-human-authority
  mv "$saved" "$checkout/metasystem.conf"
}

assert_land_brain_refusal() { # root, expected, command...
  local checkout=$1 expected=$2 output rc before after
  shift 2
  before=$(git -C "$checkout" rev-parse HEAD)
  set +e
  output=$("$@" 2>&1)
  rc=$?
  set -e
  after=$(git -C "$checkout" rev-parse HEAD)
  [[ $rc -eq 2 && "$output" == *"$expected"* && "$after" == "$before" ]] || {
    echo "brain landing fence wanted exit 2 and '$expected' without moving HEAD; rc=$rc before=$before after=$after output=$output" >&2
    exit 1
  }
}

if [[ "$fixture_scenario" == brain-land-refuses ]]; then
  make_brain_source_leg brain-land
  registry=$leg_root/registry
  mkdir -p "$registry"
  export METASYSTEM_SUPERVISION_REGISTRY_HOME=$registry
  declare_fixture_brain_temporarily "$leg_local"
  message=$leg_root/message.txt
  printf '%s\n' "brain landing fixture" >"$message"
  refusal="land refused: this checkout is declared the brain; the brain never lands"
  assert_land_brain_refusal "$leg_local" "$refusal" bash "$leg_local/scripts/agents/land.sh" -m "$message" --chain brain-root payload.txt
  assert_land_brain_refusal "$leg_local" "$refusal" bash "$leg_local/scripts/agents/land.sh" -m "$message" --direct-fix tier-1 payload.txt
  assert_land_brain_refusal "$leg_local" "$refusal" bash "$leg_local/scripts/agents/commit.sh" --chain brain-root -F "$message" payload.txt
  assert_land_brain_refusal "$leg_local" "$refusal" bash "$leg_local/scripts/agents/commit.sh" --direct-fix register-carriage -F "$message" payload.txt
  "$source_engine" lease retire --root "$leg_local" --session brain-land-fixture --pid "$$" --start "$leg_fixture_start" >/dev/null
  rm -f "$leg_local/artifacts/agents/mains/worktree-lease.json"
  mkdir -p "$leg_local/records/misc"
  printf '%s\n' "plain brain record" >"$leg_local/records/misc/brain-fixture-record.md"
  git -C "$leg_local" add records/misc/brain-fixture-record.md
  METASYSTEM_OWNER_LINEAGE=fixture-lineage bash "$leg_local/scripts/agents/commit.sh" -F "$message" records/misc/brain-fixture-record.md >/dev/null
  corrupt_head=$(git -C "$leg_local" rev-parse HEAD)
  printf '%s\n' '{broken' >"$leg_local/artifacts/agents/brain.json"
  remedy="this checkout's brain declaration is unreadable"
  assert_land_brain_refusal "$leg_local" "$remedy" bash "$leg_local/scripts/agents/land.sh" -m "$message" --chain brain-root payload.txt
  assert_land_brain_refusal "$leg_local" "$remedy" bash "$leg_local/scripts/agents/land.sh" -m "$message" --direct-fix tier-1 payload.txt
  assert_land_brain_refusal "$leg_local" "$remedy" bash "$leg_local/scripts/agents/commit.sh" --chain brain-root -F "$message" payload.txt
  assert_land_brain_refusal "$leg_local" "$remedy" bash "$leg_local/scripts/agents/commit.sh" --direct-fix register-carriage -F "$message" payload.txt
  [[ $(git -C "$leg_local" rev-parse HEAD) == "$corrupt_head" ]] || { echo "corrupt brain landing moved HEAD" >&2; exit 1; }
  echo "brain-land-refuses passed"
  exit 0
fi

if [[ "$fixture_scenario" == brain-absent-node-proceeds ]]; then
  make_brain_source_leg brain-absent
  registry=$leg_root/registry
  mkdir -p "$registry"
  export METASYSTEM_SUPERVISION_REGISTRY_HOME=$registry
  "$source_engine" lease retire --root "$leg_local" --session brain-land-fixture --pid "$$" \
    --start "$("$source_engine" proc started-at --pid "$$")" >/dev/null
  declare_fixture_brain_temporarily "$leg_local"
  node=$leg_root/node
  git clone -q "$leg_remote" "$node"
  git -C "$node" config user.name fixture-node
  git -C "$node" config user.email fixture-node@example.invalid
  git -C "$node" config metasystem.goal.machine node-leg
  git -C "$node" update-ref refs/metasystem/goals/accepted origin/main
	"$source_engine" lease announce --root "$node" --session brain-absent-node-fixture \
		--pid "$$" --start "$leg_fixture_start" --tag brain-absent-node-fixture \
		--runtime fake --owner-lineage fixture-lineage >/dev/null
	node_record=records/misc/brain-absent-node.txt
	mkdir -p "$node/records/misc"
	printf '%s\n' "node landing" >"$node/$node_record"
	printf '%s\n' "absent node landing" >"$leg_root/node-message.txt"
	METASYSTEM_OWNER_LINEAGE=fixture-lineage bash "$node/scripts/agents/land.sh" -m "$leg_root/node-message.txt" \
		--skip-transport --direct-fix register-carriage "$node_record" >/dev/null
	[[ $(git --git-dir="$leg_remote" show "main:$node_record") == "node landing" ]] || {
    echo "undeclared node did not land and push while its peer was the brain" >&2
    exit 1
  }
  echo "brain-absent-node-proceeds passed"
  exit 0
fi

# The default build stamp is a landed commit only when every path selected by
# the compiled ENGINE policy matches HEAD. The fixture keeps the metasystem
# below the repository toplevel, matching the layout used by adopted seats.
if [[ "$fixture_scenario" == build-stamp ]]; then
build_top=$tmp/build-stamp-source
build_root=$build_top/metasystem
source_top=$(git -C "$root" rev-parse --show-toplevel)
source_prefix=$(git -C "$root" rev-parse --show-prefix)
source_prefix=${source_prefix%/}
mkdir -p "$build_root"
if [[ -n "$source_prefix" ]]; then
  git -C "$source_top" archive "HEAD:$source_prefix" | tar -x -C "$build_root"
else
  git -C "$source_top" archive HEAD | tar -x -C "$build_root"
fi
cp "$root/scripts/agents/go-build.sh" "$build_root/scripts/agents/go-build.sh"
git -C "$build_top" init -q -b main
git -C "$build_top" config user.name fixture
git -C "$build_top" config user.email fixture@example.invalid
git -C "$build_top" add .
git -C "$build_top" commit -qm 'clean nested build source'
clean_stamp=$(git -C "$build_top" rev-parse --short HEAD)
clean_engine=$tmp/clean-engine
dirty_engine=$tmp/dirty-engine
bash "$build_root/scripts/agents/go-build.sh" --out "$clean_engine" >/dev/null
observed_clean=$(go version -m "$clean_engine" | sed -n 's/.*BuildStamp=\([a-z0-9-]*\).*/\1/p' | head -1)
[[ "$observed_clean" == "$clean_stamp" ]] \
  || { echo "clean ENGINE tree carried stamp $observed_clean, want $clean_stamp" >&2; exit 1; }
printf 'package supervise\n' >"$build_root/internal/supervise/rearm_dirty_fixture.go"
bash "$build_root/scripts/agents/go-build.sh" --out "$dirty_engine" >/dev/null
observed_dirty=$(go version -m "$dirty_engine" | sed -n 's/.*BuildStamp=\([a-z0-9-]*\).*/\1/p' | head -1)
[[ "$observed_dirty" == "dev-$clean_stamp-dirty" ]] \
  || { echo "dirty ENGINE tree carried stamp $observed_dirty, want dev-$clean_stamp-dirty" >&2; exit 1; }
echo "land build-stamp fixture passed"
fi

# 1. Origin advances immediately before the first real push reads the remote.
# That push sees the diverged branch and is rejected; the driver fetches,
# rebases, and its second real push lands both commits.
if [[ "$fixture_scenario" == push-retry ]]; then
make_leg push-retry
retry_output=$leg_root/land.out
retry_attempts=$leg_root/push-attempts
retry_sentinel=$leg_root/origin-advanced
retry_transport=$leg_root/transport.git
retry_bin=$leg_root/retry-bin
mkdir -p "$retry_bin"
cat >"$retry_bin/git" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
pushes_origin=0
for arg in "$@"; do
  [[ "$arg" == origin ]] && pushes_origin=1
done
if [[ ${1:-} == push && $pushes_origin == 1 ]]; then
  echo attempt >>"$LAND_FIXTURE_PUSH_ATTEMPTS"
  if [[ ! -e "$LAND_FIXTURE_PUSH_SENTINEL" ]]; then
    touch "$LAND_FIXTURE_PUSH_SENTINEL"
    printf 'peer advance\n' >"$LAND_FIXTURE_PEER/peer.txt"
    "$LAND_FIXTURE_REAL_GIT" -C "$LAND_FIXTURE_PEER" add -- peer.txt
    "$LAND_FIXTURE_REAL_GIT" -C "$LAND_FIXTURE_PEER" commit -qm "peer advances origin"
    "$LAND_FIXTURE_REAL_GIT" -C "$LAND_FIXTURE_PEER" push -q origin main
  fi
fi
exec "$LAND_FIXTURE_REAL_GIT" "$@"
SH
chmod +x "$retry_bin/git"
git init --bare -q "$retry_transport"
git --git-dir="$retry_transport" symbolic-ref HEAD refs/heads/main
git -C "$leg_local" remote add transport "$retry_transport"
printf 'local landing\n' >"$leg_local/payload.txt"
(
  cd "$leg_local"
  env PATH="$retry_bin:$PATH" LAND_FIXTURE_REAL_GIT="$real_git" \
    LAND_FIXTURE_PEER="$leg_peer" \
    LAND_FIXTURE_PUSH_ATTEMPTS="$retry_attempts" \
    LAND_FIXTURE_PUSH_SENTINEL="$retry_sentinel" \
    bash scripts/agents/land.sh -m - payload.txt <<'MSG'
fixture retries a rejected push
MSG
) >"$retry_output" 2>&1 || {
  echo "land push-retry fixture: the landing did not recover" >&2
  sed -n '1,160p' "$retry_output" >&2
  exit 1
}
[[ $(wc -l <"$retry_attempts" | tr -d ' ') == 2 ]] \
  || { echo "land push-retry fixture: push attempts were not bounded to the rejection plus retry" >&2; exit 1; }
grep -Fq '== STEP: push origin (attempt 1 of 3)' "$retry_output"
grep -Fq '[rejected] (fetch first)' "$retry_output"
grep -Fq '== STEP: fetch origin after push attempt 1' "$retry_output"
grep -Fq '== STEP: rebase onto origin/main after push attempt 1' "$retry_output"
grep -Fq '== STEP: push origin (attempt 2 of 3)' "$retry_output"
grep -Fq '== STEP: sync transport' "$retry_output"
[[ -f "$leg_local/artifacts/agents/validation-weight.json" ]]
[[ ! -d "$leg_local/plans/goals" || -z $(find "$leg_local/plans/goals" -type f -print -quit) ]] \
  || { echo "land push-retry fixture: ordinary landing created a goal/authority record" >&2; exit 1; }
[[ ! -d "$leg_local/artifacts/agents/runs" || -z $(find "$leg_local/artifacts/agents/runs" -name '*.json' -type f -print -quit) ]] \
  || { echo "land push-retry fixture: ordinary landing created a governed run authority record" >&2; exit 1; }
[[ ! -d "$leg_local/artifacts/agents/governed-obligations" || -z $(find "$leg_local/artifacts/agents/governed-obligations" -name '*.json' -type f -print -quit) ]] \
  || { echo "land push-retry fixture: ordinary landing created an obligation execution authority record" >&2; exit 1; }
echo "ordinary landing zero-ceremony rehearsal passed"
retry_local_head=$(git -C "$leg_local" rev-parse HEAD)
retry_peer_head=$(git -C "$leg_peer" rev-parse HEAD)
retry_remote_head=$(git --git-dir="$leg_remote" rev-parse refs/heads/main)
retry_transport_head=$(git --git-dir="$retry_transport" rev-parse refs/heads/main)
[[ "$retry_local_head" == "$retry_remote_head" ]]
[[ "$retry_remote_head" == "$retry_transport_head" ]]
[[ $(git -C "$leg_local" rev-parse refs/remotes/origin/main) == "$retry_remote_head" ]] \
  || { echo "land push-retry fixture: successful push did not update the remote-tracking ref" >&2; exit 1; }
git -C "$leg_local" merge-base --is-ancestor "$retry_peer_head" "$retry_local_head"
[[ $(git --git-dir="$leg_remote" show main:payload.txt) == 'local landing' ]]
[[ $(git --git-dir="$leg_remote" show main:peer.txt) == 'peer advance' ]]
echo "land push-retry fixture passed"
fi

# 2. A Git wrapper fails the fetch step with a distinctive exit code while
# every other command still reaches real Git. The driver must surface that
# exact code and never invoke rebase or push.
if [[ "$fixture_scenario" == step-failure ]]; then
make_leg step-failure
failure_bin=$leg_root/failure-bin
failure_log=$leg_root/git.log
failure_output=$leg_root/land.out
failure_message=$leg_root/message.txt
mkdir -p "$failure_bin"
cat >"$failure_bin/git" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >>"$LAND_FIXTURE_GIT_LOG"
if [[ ${1:-} == fetch ]]; then
  echo "fixture fetch broke with exit 73" >&2
  exit 73
fi
exec "$LAND_FIXTURE_REAL_GIT" "$@"
SH
chmod +x "$failure_bin/git"
printf 'failure propagation\n' >"$leg_local/payload.txt"
printf 'fixture preserves the failing step code\n' >"$failure_message"
set +e
(
  cd "$leg_local"
  env PATH="$failure_bin:$PATH" LAND_FIXTURE_REAL_GIT="$real_git" \
    LAND_FIXTURE_GIT_LOG="$failure_log" \
    bash scripts/agents/land.sh -m "$failure_message" --skip-transport payload.txt
) >"$failure_output" 2>&1
failure_rc=$?
set -e
[[ $failure_rc == 73 ]] \
  || { echo "land step-failure fixture: fetch exit 73 became $failure_rc" >&2; sed -n '1,160p' "$failure_output" >&2; exit 1; }
grep -Fq '!! STEP FAILED: fetch origin (exit 73)' "$failure_output"
grep -Fq 'fixture fetch broke with exit 73' "$failure_output"
if grep -Fq '== STEP: rebase onto origin/main' "$failure_output" \
    || grep -Eq '^push( |$)' "$failure_log"; then
  echo "land step-failure fixture: the chain continued after fetch failed" >&2
  exit 1
fi
[[ $(git -C "$leg_local" rev-parse HEAD) != $(git --git-dir="$leg_remote" rev-parse refs/heads/main) ]]
echo "land step-failure fixture passed"
fi

# 3. An inherited acknowledgment is deliberately ignored. The guard's exact
# refusal reaches the caller without the flag, then the same staged addition
# lands when --allow-new-plan supplies the acknowledgment for that invocation.
if [[ "$fixture_scenario" == new-plan ]]; then
make_leg new-plan
new_plan_output=$leg_root/refused.out
allowed_output=$leg_root/allowed.out
new_plan_message=$leg_root/message.txt
cat >"$leg_local/.git/hooks/pre-commit" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
repository=$(git rev-parse --show-toplevel)
exec "$repository/scripts/agents/pre-commit-guard.sh"
SH
chmod +x "$leg_local/.git/hooks/pre-commit"
printf 'deliberate new plan\n' >"$leg_local/plans/new.md"
printf 'fixture exercises the new-plan acknowledgment\n' >"$new_plan_message"
new_plan_base=$(git -C "$leg_local" rev-parse HEAD)
set +e
(
  cd "$leg_local"
  METASYSTEM_ALLOW_NEW_PLAN=1 \
    bash scripts/agents/land.sh -m "$new_plan_message" --skip-transport plans/new.md
) >"$new_plan_output" 2>&1
new_plan_rc=$?
set -e
[[ $new_plan_rc != 0 ]] \
  || { echo "land new-plan fixture: an inherited acknowledgment bypassed the explicit flag" >&2; exit 1; }
grep -Fq 'pre-commit guard: refusing to commit NEW plan file(s):' "$new_plan_output" || {
  echo "land new-plan fixture: the guard refusal was not visible verbatim" >&2
  sed -n '1,160p' "$new_plan_output" >&2
  exit 1
}
grep -Fq '!! STEP FAILED: commit' "$new_plan_output" || {
  echo "land new-plan fixture: the refusal did not name the commit step" >&2
  sed -n '1,160p' "$new_plan_output" >&2
  exit 1
}
[[ $(git -C "$leg_local" rev-parse HEAD) == "$new_plan_base" ]]
if grep -Fq '== STEP: fetch origin' "$new_plan_output"; then
  echo "land new-plan fixture: the refused commit continued into fetch" >&2
  exit 1
fi
(
  cd "$leg_local"
  bash scripts/agents/land.sh -m "$new_plan_message" --staged-only \
    --allow-new-plan --skip-transport
) >"$allowed_output" 2>&1 || {
  echo "land new-plan fixture: --allow-new-plan did not admit the same staged plan" >&2
  sed -n '1,160p' "$allowed_output" >&2
  exit 1
}
new_plan_local_head=$(git -C "$leg_local" rev-parse HEAD)
new_plan_remote_head=$(git --git-dir="$leg_remote" rev-parse refs/heads/main)
[[ "$new_plan_local_head" == "$new_plan_remote_head" ]]
[[ $(git --git-dir="$leg_remote" show main:plans/new.md) == 'deliberate new plan' ]]
echo "land new-plan fixture passed"
fi

# 4. The goal item is driver input, not a Git option. The land wrapper accepts
# it once and carries the same value to the commit boundary.
if [[ "$fixture_scenario" == goal ]]; then
make_leg goal
goal_log=$leg_root/goal.log
goal_message=$leg_root/message.txt
goal_output=$leg_root/land.out
printf 'goal forwarding\n' >"$leg_local/payload.txt"
printf 'fixture forwards a goal item\n' >"$goal_message"
(
  cd "$leg_local"
  LAND_FIXTURE_GOAL_LOG="$goal_log" \
    bash scripts/agents/land.sh -m "$goal_message" --goal fx --skip-transport payload.txt
) >"$goal_output" 2>&1 || {
  echo "land goal fixture: --goal was not accepted and forwarded" >&2
  sed -n '1,160p' "$goal_output" >&2
  exit 1
}
[[ $(<"$goal_log") == fx ]] \
  || { echo "land goal fixture: the commit boundary did not receive goal fx" >&2; exit 1; }
echo "land goal fixture passed"
fi

# 5. Tier 1 runs the declared command against the staged candidate before the
# commit boundary, then carries the root job and exact receipt path together.
if [[ "$fixture_scenario" == tier-one ]]; then
clear_independent_fixture_context
make_leg tier-one
tier_one_log=$leg_root/tier-one.log
tier_one_message=$leg_root/message.txt
tier_one_output=$leg_root/land.out
printf 'tier-one landing\n' >"$leg_local/payload.txt"
printf 'fixture creates a tree-bound tier-one receipt\n' >"$tier_one_message"
(
  cd "$leg_local"
  LAND_FIXTURE_TIER_ONE_LOG="$tier_one_log" \
    bash scripts/agents/land.sh -m "$tier_one_message" --goal fx \
      --direct-fix tier-1 --root-job tier-one-root \
      --tests "test \"\$(cat payload.txt)\" = tier-one\\ landing" \
      --skip-transport payload.txt
) >"$tier_one_output" 2>&1 || {
  echo "land tier-one fixture: receipt creation or forwarding failed" >&2
  sed -n '1,180p' "$tier_one_output" >&2
  exit 1
}
grep -Fxq 'directFix=tier-1' "$tier_one_log"
grep -Fxq 'rootJob=tier-one-root' "$tier_one_log"
tier_one_receipt=$(sed -n 's/^testReceipt=//p' "$tier_one_log")
[[ -f "$tier_one_receipt" ]] \
  || { echo "land tier-one fixture: forwarded receipt does not exist" >&2; exit 1; }
tier_one_tree=$(git -C "$leg_local" rev-parse HEAD^{tree})
[[ $("$source_engine" json get --file "$tier_one_receipt" --field tree) == "$tier_one_tree" ]]
[[ $("$source_engine" json get --file "$tier_one_receipt" --field exitStatus) == 0 ]]
for binding_field in indexTreeBefore worktreeTreeBefore indexTreeAfter worktreeTreeAfter; do
  [[ $("$source_engine" json get --file "$tier_one_receipt" --field "binding.$binding_field") == "$tier_one_tree" ]]
done
echo "land tier-one fixture passed"
fi

# 6. A full-width chain stops before verification without a receipt, rejects a
# receipt for another tree before commit, and lands with the exact candidate's
# full-battery receipt and bar-a provenance.
if [[ "$fixture_scenario" == full-width-chain ]]; then
clear_independent_fixture_context
make_leg full-width-chain
full_chain_message=$leg_root/message.txt
full_chain_missing_output=$leg_root/missing-receipt.out
full_chain_usage_output=$leg_root/usage-refusal.out
full_chain_mismatch_output=$leg_root/mismatched-receipt.out
full_chain_landing_output=$leg_root/land.out
full_chain_log=$leg_root/chain.log
full_battery_command=$(sed -n 's/^const FullBatteryCommand = "\(.*\)"$/\1/p' \
  "$root/internal/validate/recertification.go")
[[ -n "$full_battery_command" ]] \
  || { echo "land full-width-chain fixture: full battery command source is unreadable" >&2; exit 1; }
printf 'fixture lands a receipted full-width chain\n' >"$full_chain_message"

full_chain_base=$(git -C "$leg_local" rev-parse HEAD)
set +e
(
  cd "$leg_local"
  bash scripts/agents/land.sh -m "$full_chain_message" --chain full-chain \
    --tests true --test-receipt receipt.json --staged-only --skip-transport
) >"$full_chain_usage_output" 2>&1
full_chain_usage_rc=$?
set -e
[[ $full_chain_usage_rc == 2 ]] || {
  echo "land full-width-chain fixture: conflicting receipt inputs exited $full_chain_usage_rc, want 2" >&2
  sed -n '1,120p' "$full_chain_usage_output" >&2
  exit 1
}
grep -Fqx 'land refused: --tests and --test-receipt cannot be combined; remove --test-receipt for a tier-1 landing, or remove --tests for a receipted chain landing' \
  "$full_chain_usage_output"
grep -Fq 'Usage: scripts/agents/land.sh' "$full_chain_usage_output"
if grep -Fq '== STEP:' "$full_chain_usage_output"; then
  echo "land full-width-chain fixture: conflicting receipt inputs started landing work" >&2
  exit 1
fi
[[ $(git -C "$leg_local" rev-parse HEAD) == "$full_chain_base" ]]

full_chain_other_tree=$(git -C "$leg_local" rev-parse HEAD^{tree})
(
  cd "$leg_local"
  "$source_engine" landing test-receipt --root . \
    --tree "$full_chain_other_tree" --command "$full_battery_command" --goal fx --cap-min 1
) >/dev/null
full_chain_other_receipt=artifacts/agents/landing/receipts/$full_chain_other_tree.json

printf '# full-width candidate\n' >>"$leg_local/scripts/agents/go-gate.sh"
git -C "$leg_local" add -- scripts/agents/go-gate.sh
full_chain_candidate=$(git -C "$leg_local" write-tree)
mkdir -p "$leg_local/artifacts/agents/jobs" \
  "$leg_local/artifacts/agents/full-chain/rounds/1"
cat >"$leg_local/artifacts/agents/jobs/full-chain.json" <<'JSON'
{
  "jobId": "full-chain",
  "goalId": null,
  "parentJob": null,
  "role": "implementer",
  "round": 1,
  "goalTier": 2,
  "gateWidth": "full",
  "destructiveReach": "DESIGN-BEARING",
  "chainClosed": true
}
JSON
git -C "$leg_local" diff --cached --binary --full-index --no-ext-diff --no-textconv -- \
  >"$leg_local/artifacts/agents/full-chain/rounds/1/diff.patch"
cat >"$leg_local/artifacts/agents/full-chain/rounds/1/review.json" <<JSON
{
  "diffArtifact": "diff.patch",
  "implementerJob": "full-chain",
  "reviewedTree": "$full_chain_candidate"
}
JSON

set +e
(
  cd "$leg_local"
  bash scripts/agents/land.sh -m "$full_chain_message" --chain full-chain \
    --staged-only --skip-transport
) >"$full_chain_missing_output" 2>&1
full_chain_missing_rc=$?
set -e
[[ $full_chain_missing_rc == 2 ]] || {
  echo "land full-width-chain fixture: missing receipt exited $full_chain_missing_rc, want 2" >&2
  sed -n '1,160p' "$full_chain_missing_output" >&2
  exit 1
}
grep -Fqx "land refused: legacy full-width chain full-chain requires its full-battery receipt" \
  "$full_chain_missing_output"
if grep -Fq '== STEP: verify checks' "$full_chain_missing_output"; then
  echo "land full-width-chain fixture: missing receipt reached verification" >&2
  exit 1
fi
[[ $(git -C "$leg_local" rev-parse HEAD) == "$full_chain_base" ]]

# The same refusal in a contract-bearing repository names the testing receipt
# it wants, never the legacy full battery. The key is unstaged and removed
# again, so the later receipted landing still takes the legacy path.
printf 'testing.contract=testing.json\n' >>"$leg_local/metasystem.conf"
set +e
(
  cd "$leg_local"
  bash scripts/agents/land.sh -m "$full_chain_message" --chain full-chain \
    --staged-only --skip-transport
) >"$full_chain_missing_output" 2>&1
full_chain_schema2_rc=$?
set -e
grep -v '^testing\.contract=testing\.json$' "$leg_local/metasystem.conf" >"$leg_local/metasystem.conf.new"
mv "$leg_local/metasystem.conf.new" "$leg_local/metasystem.conf"
[[ $full_chain_schema2_rc == 2 ]] || {
  echo "land full-width-chain fixture: schema-2 missing receipt exited $full_chain_schema2_rc, want 2" >&2
  sed -n '1,160p' "$full_chain_missing_output" >&2
  exit 1
}
grep -Fqx "land refused: chain full-chain requires sufficient schema-2 testing evidence; run metasystem landing test-receipt --root . --tree <whole-project-tree> --mode auto and pass it with --test-receipt" \
  "$full_chain_missing_output"
[[ $(git -C "$leg_local" rev-parse HEAD) == "$full_chain_base" ]]

set +e
(
  cd "$leg_local"
  bash scripts/agents/land.sh -m "$full_chain_message" --chain full-chain \
    --test-receipt "$full_chain_other_receipt" --staged-only --skip-transport
) >"$full_chain_mismatch_output" 2>&1
full_chain_mismatch_rc=$?
set -e
[[ $full_chain_mismatch_rc == 2 ]] || {
  echo "land full-width-chain fixture: mismatched receipt exited $full_chain_mismatch_rc, want 2" >&2
  sed -n '1,180p' "$full_chain_mismatch_output" >&2
  exit 1
}
grep -Fq "land refused: the receipt at $full_chain_other_receipt names tree $full_chain_other_tree but the staged candidate is $full_chain_candidate; make the receipt against this exact candidate" \
  "$full_chain_mismatch_output"
if grep -Fq '== STEP: commit' "$full_chain_mismatch_output"; then
  echo "land full-width-chain fixture: mismatched receipt reached commit" >&2
  exit 1
fi
[[ $(git -C "$leg_local" rev-parse HEAD) == "$full_chain_base" ]]

(
  cd "$leg_local"
  "$source_engine" landing test-receipt --root . \
    --tree "$full_chain_candidate" --command "$full_battery_command" --goal fx --cap-min 1
) >/dev/null
full_chain_receipt=artifacts/agents/landing/receipts/$full_chain_candidate.json
(
  cd "$leg_local"
  LAND_FIXTURE_CHAIN_LOG="$full_chain_log" \
    bash scripts/agents/land.sh -m "$full_chain_message" --chain full-chain \
      --test-receipt "$full_chain_receipt" --staged-only --skip-transport
) >"$full_chain_landing_output" 2>&1 || {
  echo "land full-width-chain fixture: matching receipt did not land" >&2
  sed -n '1,220p' "$full_chain_landing_output" >&2
  exit 1
}
grep -Fxq 'chain=full-chain' "$full_chain_log"
grep -Fxq "testReceipt=$full_chain_receipt" "$full_chain_log"
grep -Fxq 'verdict=pass bar=a' "$full_chain_log" || {
  echo "land full-width-chain fixture: commit boundary did not observe pass bar a" >&2
  cat "$full_chain_log" >&2
  exit 1
}
git -C "$leg_local" show -s --format=%B HEAD \
  | grep -Fxq 'Landing-Provenance-Verdict: pass bar=a'
git -C "$leg_local" show -s --format=%B HEAD \
  | grep -Fq 'Landing-Provenance: chain=full-chain change='
[[ $(git -C "$leg_local" rev-parse HEAD) == $(git --git-dir="$leg_remote" rev-parse refs/heads/main) ]]
echo "land full-width-chain fixture passed"
fi

# 10. A receipt taken through the enrolled candidate engine survives a peer's
# goal-verb commit. The two drift probes run first on the same receipt so the
# cross-tip projection cannot weaken the exact checkout posture.
if [[ "$fixture_scenario" == ledger-move-lands ]]; then
clear_independent_fixture_context
echo "land ledger-move-lands fixture: one candidate engine build"
make_leg ledger-move-lands
arm_receipt_runner "$leg_local" "$leg_local/bin/metasystem"
ledger_message=$leg_root/message.txt
ledger_output=$leg_root/land.out
ledger_chain_log=$leg_root/chain.log
ledger_unstaged_output=$leg_root/unstaged.out
ledger_staged_output=$leg_root/staged.out
printf 'fixture lands across a peer goal commit\n' >"$ledger_message"
printf 'ledger landing\n' >"$leg_local/payload.txt"
git -C "$leg_local" add -- payload.txt
ledger_base=$(git -C "$leg_local" rev-parse HEAD)
ledger_candidate=$(git -C "$leg_local" write-tree)
prepare_receipt_chain "$leg_local" ledger-move-chain "$ledger_candidate"
take_fixture_receipt "$leg_local/bin/metasystem" "$leg_local" "$leg_root/receipt.out"
ledger_receipt=$fixture_receipt_path
ledger_receipt_tree=$fixture_receipt_tree
[[ $(receipt_checkout_env_run "$leg_local" "$leg_local/bin/metasystem" json get --file "$ledger_receipt" --field workspace.tree) =~ ^[0-9a-f]{40}$ ]]

printf 'x' >>"$leg_local/payload.txt"
if run_fixture_landing "$leg_local" "$ledger_message" ledger-move-chain "$ledger_receipt" \
    "$ledger_unstaged_output" "$ledger_chain_log"; then
  ledger_unstaged_rc=0
else
  ledger_unstaged_rc=$?
fi
[[ $ledger_unstaged_rc -ne 0 ]] || { echo "land ledger-move-lands fixture: unstaged drift landed" >&2; exit 1; }
grep -Fq 'unstaged changes remain after staging' "$ledger_unstaged_output"
[[ $(git -C "$leg_local" rev-parse HEAD) == "$ledger_base" ]]
git -C "$leg_local" checkout -q -- payload.txt

printf 'x' >>"$leg_local/payload.txt"
git -C "$leg_local" add -- payload.txt
ledger_drift_tree=$(git -C "$leg_local" write-tree)
if run_fixture_landing "$leg_local" "$ledger_message" ledger-move-chain "$ledger_receipt" \
    "$ledger_staged_output" "$ledger_chain_log"; then
  ledger_staged_rc=0
else
  ledger_staged_rc=$?
fi
[[ $ledger_staged_rc -ne 0 ]] || { echo "land ledger-move-lands fixture: staged drift landed" >&2; exit 1; }
grep -Fq "land refused: the receipt at $ledger_receipt names tree $ledger_receipt_tree but the staged candidate is $ledger_drift_tree; make the receipt against this exact candidate" "$ledger_staged_output"
grep -Fq 'proof-input-moved-after-receipt: group policy-protection' "$ledger_staged_output"
grep -Fq 'moved declared paths: payload.txt' "$ledger_staged_output"
[[ $(git -C "$leg_local" rev-parse HEAD) == "$ledger_base" ]]

printf 'ledger landing\n' >"$leg_local/payload.txt"
git -C "$leg_local" add -- payload.txt
[[ $(git -C "$leg_local" write-tree) == "$ledger_receipt_tree" ]]
publish_peer_ledger_move
ledger_peer=$(git --git-dir="$leg_remote" rev-parse refs/heads/main)
git -C "$leg_local" fetch -q origin
git -C "$leg_local" merge -q --ff-only origin/main
ledger_moved_candidate=$(git -C "$leg_local" write-tree)
[[ "$ledger_moved_candidate" != "$ledger_receipt_tree" ]]
ledger_receipt_workspace=$(receipt_checkout_env_run "$leg_local" "$leg_local/bin/metasystem" landing workspace --root "$leg_local" --tree "$ledger_receipt_tree")
ledger_candidate_workspace=$(receipt_checkout_env_run "$leg_local" "$leg_local/bin/metasystem" landing workspace --root "$leg_local" --tree "$ledger_moved_candidate")
[[ "$ledger_receipt_workspace" == "$ledger_candidate_workspace" ]]
if ! run_fixture_landing "$leg_local" "$ledger_message" ledger-move-chain "$ledger_receipt" \
    "$ledger_output" "$ledger_chain_log"; then
  echo "site 8 refused a ledger-only move" >&2
  sed -n '1,260p' "$ledger_output" >&2
  exit 1
fi
grep -Fxq "testReceipt=$ledger_receipt" "$ledger_chain_log"
ledger_head=$(git -C "$leg_local" rev-parse HEAD)
[[ "$ledger_head" == $(git --git-dir="$leg_remote" rev-parse refs/heads/main) ]]
[[ $(git -C "$leg_local" rev-parse HEAD^1) == "$ledger_peer" ]]
git --git-dir="$leg_remote" show main:plans/goals/peer-goal.md >/dev/null
stop_receipt_runner
echo "land ledger-move-lands fixture passed"
exit 0
fi

# 11. A records-only peer commit changes the workspace projection but not a
# selected execution identity, so site 8 reaches retained-proof verification.
if [[ "$fixture_scenario" == records-move-lands ]]; then
clear_independent_fixture_context
echo "land records-move-lands fixture: one candidate engine build"
make_leg records-move-lands
arm_receipt_runner "$leg_local" "$leg_local/bin/metasystem"
records_message=$leg_root/message.txt
records_output=$leg_root/land.out
records_chain_log=$leg_root/chain.log
printf 'fixture lands across an unrelated records commit\n' >"$records_message"
printf 'records landing\n' >"$leg_local/payload.txt"
git -C "$leg_local" add -- payload.txt
records_candidate=$(git -C "$leg_local" write-tree)
prepare_receipt_chain "$leg_local" records-move-chain "$records_candidate"
take_fixture_receipt "$leg_local/bin/metasystem" "$leg_local" "$leg_root/receipt.out"
records_receipt=$fixture_receipt_path
records_receipt_tree=$fixture_receipt_tree
publish_peer_records_move
records_peer=$(git --git-dir="$leg_remote" rev-parse refs/heads/main)
git -C "$leg_local" fetch -q origin
git -C "$leg_local" merge -q --ff-only origin/main
records_moved_candidate=$(git -C "$leg_local" write-tree)
records_receipt_workspace=$(receipt_checkout_env_run "$leg_local" "$leg_local/bin/metasystem" landing workspace --root "$leg_local" --tree "$records_receipt_tree")
records_candidate_workspace=$(receipt_checkout_env_run "$leg_local" "$leg_local/bin/metasystem" landing workspace --root "$leg_local" --tree "$records_moved_candidate")
[[ "$records_receipt_workspace" != "$records_candidate_workspace" ]]
if ! run_fixture_landing "$leg_local" "$records_message" records-move-chain "$records_receipt" \
    "$records_output" "$records_chain_log"; then
  echo "land records-move-lands fixture: the records-only move did not land" >&2
  sed -n '1,260p' "$records_output" >&2
  exit 1
fi
grep -Fq '== STEP: test receipt for staged candidate' "$records_output"
if grep -Fq 'proof-input-moved-after-receipt' "$records_output"; then
  echo "land records-move-lands fixture: an unrelated record moved a selected input identity" >&2
  exit 1
fi
records_head=$(git -C "$leg_local" rev-parse HEAD)
[[ "$records_head" == $(git --git-dir="$leg_remote" rev-parse refs/heads/main) ]]
[[ $(git -C "$leg_local" rev-parse HEAD^1) == "$records_peer" ]]
git --git-dir="$leg_remote" show main:records/misc/peer-note.md >/dev/null
stop_receipt_runner
echo "land records-move-lands fixture passed"
exit 0
fi

# 12. A peer change under scripts/** changes the declared execution identity.
# The pre-rebase receipt is accepted exactly, but the post-rebase proof wall
# refuses the local commit before it can reach origin.
if [[ "$fixture_scenario" == input-move-refuses ]]; then
clear_independent_fixture_context
echo "land input-move-refuses fixture: one candidate engine build"
make_leg input-move-refuses
arm_receipt_runner "$leg_local" "$leg_local/bin/metasystem"
input_message=$leg_root/message.txt
input_output=$leg_root/land.out
input_chain_log=$leg_root/chain.log
printf 'fixture refuses a moved declared input\n' >"$input_message"
printf 'input landing\n' >"$leg_local/payload.txt"
git -C "$leg_local" add -- payload.txt
input_candidate=$(git -C "$leg_local" write-tree)
prepare_receipt_chain "$leg_local" input-move-chain "$input_candidate"
take_fixture_receipt "$leg_local/bin/metasystem" "$leg_local" "$leg_root/receipt.out"
input_receipt=$fixture_receipt_path
publish_peer_input_move
input_peer=$(git --git-dir="$leg_remote" rev-parse refs/heads/main)
if run_fixture_landing "$leg_local" "$input_message" input-move-chain "$input_receipt" \
    "$input_output" "$input_chain_log"; then
  input_rc=0
else
  input_rc=$?
fi
[[ $input_rc -ne 0 ]] || { echo "land input-move-refuses fixture: the moved input landed" >&2; exit 1; }
grep -Fq '!! STEP FAILED: verify shared testing proof after rebase' "$input_output"
grep -Fq 'proof-input-moved-after-receipt: group policy-protection' "$input_output"
grep -Fq 'scripts/application-input.txt' "$input_output"
[[ $(git --git-dir="$leg_remote" rev-parse refs/heads/main) == "$input_peer" ]]
[[ $(git -C "$leg_local" rev-parse HEAD) != "$input_peer" ]]
git -C "$leg_local" merge-base --is-ancestor "$input_peer" HEAD
stop_receipt_runner
echo "land input-move-refuses fixture passed"
exit 0
fi

# 13. The pre-cutover engine is real and keeps its own receipt intact. The
# candidate engine writes the newer receipt shape in the peer clone for the
# same tree, which the enrolled older reader must reject after it is copied.
if [[ "$fixture_scenario" == receipt-cutover ]]; then
clear_independent_fixture_context
echo "land receipt-cutover fixture: one candidate engine build and one pinned old-engine build"
# The pre-cutover source is archived once; a provisionally stamped build of it
# claims the seed goal inside make_leg, and the seed-stamped build below is the
# engine the cutover leg enrolls.
cutover_old_source=$tmp/receipt-cutover-old-src
cutover_seed_claim_engine=$tmp/receipt-cutover-seed-claim-engine
cutover_source_top=$(git -C "$root" rev-parse --show-toplevel)
mkdir -p "$cutover_old_source"
git -C "$cutover_source_top" archive 6bc19ba1c metasystem/ | tar -x -C "$cutover_old_source"
METASYSTEM_BUILD_STAMP=receipt-cutover-seed-claim \
  bash "$cutover_old_source/metasystem/scripts/agents/go-build.sh" --out "$cutover_seed_claim_engine" >/dev/null
make_leg receipt-cutover
cutover_old_engine=$leg_root/old-engine
cutover_moved_remote=$leg_root/moved-origin.git

# Trunk's claimed record gained the elapsed-budget episode binding after the
# pinned reader. Project only those optional keys away so this fixture still
# compares the real pre-cutover receipt producer with the current producer.
cutover_compat=$leg_root/compat
git clone -q "$leg_remote" "$cutover_compat"
git -C "$cutover_compat" config user.name fixture-cutover-compat
git -C "$cutover_compat" config user.email fixture-cutover-compat@example.invalid
cutover_goal=$cutover_compat/plans/goals/fx.md
conf_edit "$cutover_goal" awk '
  /^Integrity: sha256=/ { next }
  /^- Claimed:/ {
    gsub(/ episodeAt=[^ ]+/, "")
    gsub(/ episodeRevision=[^ ]+/, "")
    gsub(/ episodeObligationRevision=[^ ]+/, "")
  }
  { print }
'
cutover_goal_digest=$("$source_engine" util sha256 --file "$cutover_goal")
printf 'Integrity: sha256=%s\n' "$cutover_goal_digest" >>"$cutover_goal"
git -C "$cutover_compat" add -- plans/goals/fx.md
if ! git -C "$cutover_compat" diff --cached --quiet; then
  git -C "$cutover_compat" -c core.hooksPath=/dev/null commit -qm 'project claimed row for pre-cutover reader'
fi
git -C "$cutover_compat" push -q origin main
git clone -q --bare "$leg_remote" "$cutover_moved_remote"
receipt_seed_build_stamp=$(git -C "$cutover_compat" rev-parse HEAD)
for cutover_checkout in "$leg_local" "$leg_peer"; do
  git -C "$cutover_checkout" fetch -q origin main
  git -C "$cutover_checkout" reset -q --hard origin/main
  git -C "$cutover_checkout" update-ref refs/heads/metasystem/goals origin/main
  git -C "$cutover_checkout" update-ref refs/metasystem/goals/accepted origin/main
done
METASYSTEM_BUILD_STAMP="$receipt_seed_build_stamp" \
  bash "$root/scripts/agents/go-build.sh" --out "$leg_root/engine" >/dev/null
METASYSTEM_BUILD_STAMP="$receipt_seed_build_stamp" \
  bash "$cutover_old_source/metasystem/scripts/agents/go-build.sh" --out "$cutover_old_engine" >/dev/null
install_cutover_engine "$leg_local" "$cutover_old_engine"
install_cutover_engine "$leg_peer" "$leg_root/engine"
[[ $(git -C "$leg_local" rev-parse HEAD) == "$receipt_seed_build_stamp" ]]
[[ $(git -C "$leg_peer" rev-parse HEAD) == "$receipt_seed_build_stamp" ]]
git -C "$leg_local" check-ignore -q bin/metasystem
git -C "$leg_peer" check-ignore -q bin/metasystem
if git -C "$leg_local" ls-files --error-unmatch bin/metasystem >/dev/null 2>&1 ||
    git -C "$leg_peer" ls-files --error-unmatch bin/metasystem >/dev/null 2>&1; then
  echo "land receipt-cutover fixture: an installed cutover engine is tracked" >&2
  exit 1
fi
arm_receipt_runner "$leg_local" "$leg_local/bin/metasystem"
cutover_message=$leg_root/message.txt
cutover_output=$leg_root/land.out
cutover_chain_log=$leg_root/chain.log
printf 'fixture lands an exact old-engine receipt\n' >"$cutover_message"
printf 'cutover exact landing\n' >"$leg_local/payload.txt"
printf 'cutover exact landing\n' >"$leg_peer/payload.txt"
git -C "$leg_local" add -- payload.txt
git -C "$leg_peer" add -- payload.txt
cutover_candidate=$(git -C "$leg_local" write-tree)
cutover_peer_candidate=$(git -C "$leg_peer" write-tree)
[[ "$cutover_peer_candidate" == "$cutover_candidate" ]]
prepare_receipt_chain "$leg_local" cutover-exact-chain "$cutover_candidate"
take_fixture_receipt "$leg_local/bin/metasystem" "$leg_local" "$leg_root/old-receipt.out"
cutover_old_receipt=$fixture_receipt_path
cutover_tree=$fixture_receipt_tree
if receipt_checkout_env_run "$leg_local" "$leg_local/bin/metasystem" json get --file "$cutover_old_receipt" --field workspace >/dev/null 2>&1 ||
    receipt_checkout_env_run "$leg_local" "$leg_local/bin/metasystem" json get --file "$cutover_old_receipt" --field candidateEngineBuildIdentity >/dev/null 2>&1 ||
    grep -Eq '"(workspace|candidateEngineBuildIdentity)"[[:space:]]*:' "$cutover_old_receipt"; then
  echo "land receipt-cutover fixture: old engine wrote a cutover-only identity field" >&2
  exit 1
fi
cutover_control=$(receipt_checkout_env_run "$leg_local" "$leg_local/bin/metasystem" landing observe --root "$leg_local" \
  --tree "$cutover_tree" --chain cutover-exact-chain --goal fx \
  --actor fixture-machine+land-receipt-fixture --test-receipt "$cutover_old_receipt")
[[ $(receipt_checkout_env_run "$leg_local" "$leg_local/bin/metasystem" json get --value "$cutover_control" --field mode) == observe ]]
[[ $(receipt_checkout_env_run "$leg_local" "$leg_local/bin/metasystem" json get --value "$cutover_control" --field verdictTrailer) == 'pass bar=a' ]]
if ! run_fixture_landing "$leg_local" "$cutover_message" cutover-exact-chain "$cutover_old_receipt" \
    "$cutover_output" "$cutover_chain_log"; then
  echo "land receipt-cutover fixture: exact old receipt did not land" >&2
  sed -n '1,260p' "$cutover_output" >&2
  exit 1
fi
if grep -Fq 'proof-input-moved-after-receipt' "$cutover_output"; then
  echo "land receipt-cutover fixture: exact old receipt reported moved proof input" >&2
  exit 1
fi
[[ $(git -C "$leg_local" rev-parse HEAD) == $(git --git-dir="$leg_remote" rev-parse refs/heads/main) ]]
[[ $(git -C "$leg_local" rev-parse HEAD^{tree}) == "$cutover_tree" ]]

arm_receipt_runner "$leg_peer" "$leg_peer/bin/metasystem"
take_fixture_receipt "$leg_peer/bin/metasystem" "$leg_peer" "$leg_root/new-receipt.out"
cutover_candidate_receipt=$fixture_receipt_path
[[ "$fixture_receipt_tree" == "$cutover_tree" ]]
[[ ${cutover_candidate_receipt##*/} == ${cutover_old_receipt##*/} ]]
receipt_checkout_env_run "$leg_peer" "$leg_peer/bin/metasystem" json get --file "$cutover_candidate_receipt" --field workspace.tree >/dev/null
receipt_checkout_env_run "$leg_peer" "$leg_peer/bin/metasystem" json get --file "$cutover_candidate_receipt" --field candidateEngineBuildIdentity >/dev/null
[[ $(receipt_checkout_env_run "$leg_peer" "$leg_peer/bin/metasystem" json get --file "$cutover_candidate_receipt" --field testing.candidateEngineIdentityVersion) == 2 ]]
cp "$cutover_candidate_receipt" "$cutover_old_receipt"
select_receipt_runner_environment "$leg_local"
cutover_refusal=$(receipt_checkout_env_run "$leg_local" "$leg_local/bin/metasystem" landing observe --root "$leg_local" \
  --tree "$cutover_tree" --chain cutover-exact-chain --goal fx \
  --actor fixture-machine+land-receipt-fixture --test-receipt "$cutover_old_receipt")
[[ $(receipt_checkout_env_run "$leg_local" "$leg_local/bin/metasystem" json get --value "$cutover_refusal" --field verdictTrailer) == 'would-refuse code=chain-test-receipt-refused' ]]
stop_receipt_runner
stop_receipt_runner

leg_local=$leg_root/moved
cutover_moved_peer=$leg_root/moved-peer
git clone -q "$cutover_moved_remote" "$leg_local"
git clone -q "$cutover_moved_remote" "$cutover_moved_peer"
git -C "$leg_local" config user.name fixture-cutover-moved
git -C "$leg_local" config user.email fixture-cutover-moved@example.invalid
git -C "$cutover_moved_peer" config user.name fixture-cutover-moved-peer
git -C "$cutover_moved_peer" config user.email fixture-cutover-moved-peer@example.invalid
git -C "$leg_local" config goal.sync-remote local
git -C "$leg_local" config goal.sync-branch refs/heads/metasystem/goals
git -C "$leg_local" config metasystem.goal.machine fixture-machine
git -C "$leg_local" config metasystem.steward.landing-ref refs/remotes/origin/main
git -C "$leg_local" update-ref refs/heads/metasystem/goals origin/main
git -C "$leg_local" update-ref refs/metasystem/goals/accepted origin/main
git -C "$cutover_moved_peer" config goal.sync-remote local
git -C "$cutover_moved_peer" config goal.sync-branch refs/heads/metasystem/goals
git -C "$cutover_moved_peer" config metasystem.goal.machine fixture-peer
git -C "$cutover_moved_peer" config metasystem.steward.landing-ref refs/remotes/origin/main
git -C "$cutover_moved_peer" update-ref refs/heads/metasystem/goals origin/main
git -C "$cutover_moved_peer" update-ref refs/metasystem/goals/accepted origin/main
install_cutover_engine "$leg_local" "$cutover_old_engine"
install_cutover_engine "$cutover_moved_peer" "$leg_root/engine"
[[ $(git -C "$leg_local" rev-parse HEAD) == "$receipt_seed_build_stamp" ]]
arm_receipt_runner "$leg_local" "$leg_local/bin/metasystem"
cutover_moved_message=$leg_root/moved-message.txt
cutover_moved_output=$leg_root/moved-land.out
cutover_moved_chain_log=$leg_root/moved-chain.log
printf 'fixture refuses an old receipt after a goal commit\n' >"$cutover_moved_message"
printf 'cutover moved landing\n' >"$leg_local/payload.txt"
git -C "$leg_local" add -- payload.txt
cutover_moved_candidate=$(git -C "$leg_local" write-tree)
prepare_receipt_chain "$leg_local" cutover-moved-chain "$cutover_moved_candidate"
take_fixture_receipt "$leg_local/bin/metasystem" "$leg_local" "$leg_root/moved-receipt.out"
cutover_moved_receipt=$fixture_receipt_path
cutover_moved_receipt_tree=$fixture_receipt_tree
publish_peer_ledger_move "$cutover_moved_peer"
cutover_peer=$(git --git-dir="$cutover_moved_remote" rev-parse refs/heads/main)
git -C "$leg_local" fetch -q origin
git -C "$leg_local" merge -q --ff-only origin/main
cutover_after_ledger=$(git -C "$leg_local" write-tree)
if run_fixture_landing "$leg_local" "$cutover_moved_message" cutover-moved-chain "$cutover_moved_receipt" \
    "$cutover_moved_output" "$cutover_moved_chain_log"; then
  cutover_moved_rc=0
else
  cutover_moved_rc=$?
fi
[[ $cutover_moved_rc -ne 0 ]] || { echo "land receipt-cutover fixture: old receipt crossed a ledger move" >&2; exit 1; }
grep -Fq "land refused: the receipt at $cutover_moved_receipt names tree $cutover_moved_receipt_tree but the staged candidate is $cutover_after_ledger; make the receipt against this exact candidate" "$cutover_moved_output"
if grep -Eqi 'unknown (verb|command)|landing workspace' "$cutover_moved_output"; then
  echo "land receipt-cutover fixture: legacy exact path called a new verb" >&2
  exit 1
fi
[[ $(git -C "$leg_local" rev-parse HEAD) == "$cutover_peer" ]]
[[ $(git --git-dir="$cutover_moved_remote" rev-parse refs/heads/main) == "$cutover_peer" ]]
stop_receipt_runner
echo "land receipt-cutover fixture passed"
exit 0
fi
