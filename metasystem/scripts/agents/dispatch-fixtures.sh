#!/usr/bin/env bash
set -euo pipefail

fixture_bed_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
source "$fixture_bed_root/scripts/agents/fixture-budget.sh"
fixture_bed_child=0
fixture_scenario=
if fixture_scenario=$(harness_fixture_bed_child_scenario dispatch "$@"); then
  fixture_bed_child=1
else
  fixture_bed_child_rc=$?
  [[ $fixture_bed_child_rc -eq 1 ]] || exit "$fixture_bed_child_rc"
fi
unset METASYSTEM_FIXTURE_SCENARIO

fixture_bed_parent_log_root=
fixture_bed_parent_child_pid=
fixture_bed_parent_cleanup() {
  local status=$?
  trap - EXIT HUP INT QUIT TERM
  if [[ -n "$fixture_bed_parent_child_pid" ]]; then
    kill -TERM "$fixture_bed_parent_child_pid" 2>/dev/null || true
    wait "$fixture_bed_parent_child_pid" 2>/dev/null || true
  fi
  [[ -z "$fixture_bed_parent_log_root" ]] \
    || rm -rf "$fixture_bed_parent_log_root" 2>/dev/null || true
  return "$status"
}

run_fixture_bed_scenarios() { # bed name, success line, script, scenario names...
  local bed=$1 success_line=$2 script=$3 log_root scenario capability log rc index=0
  local failed_names=() failed_rcs=() failed_logs=()
  shift 3
  log_root=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-${bed}-scenarios.XXXXXX")
  fixture_bed_parent_log_root=$log_root
  trap fixture_bed_parent_cleanup EXIT
  trap 'exit 129' HUP
  trap 'exit 130' INT
  trap 'exit 131' QUIT
  trap 'exit 143' TERM
  for scenario in "$@"; do
    log=$log_root/$index.log
    capability=$(harness_fixture_bed_mint_capability "$log_root" "$index" "$scenario")
    echo "$bed fixture scenario started: $scenario" >&2
    "$script" --fixture-bed-child "$scenario" "$capability" >"$log" 2>&1 &
    fixture_bed_parent_child_pid=$!
    set +e
    wait "$fixture_bed_parent_child_pid"
    rc=$?
    set -e
    fixture_bed_parent_child_pid=
    cat "$log"
    if [[ $rc -eq 0 ]]; then
      echo "$bed fixture scenario passed: $scenario" >&2
    else
      failed_names+=("$scenario")
      failed_rcs+=("$rc")
      failed_logs+=("$log")
      echo "$bed fixture scenario failed: $scenario (rc=$rc); continuing" >&2
    fi
    index=$((index + 1))
  done
  if (( ${#failed_names[@]} )); then
    echo "=== $bed failed scenarios ===" >&2
    for ((index = 0; index < ${#failed_names[@]}; index++)); do
      echo "- ${failed_names[$index]} (rc=${failed_rcs[$index]})" >&2
      echo "  output tail:" >&2
      tail -n 40 "${failed_logs[$index]}" | sed 's/^/    /' >&2
    done
    echo "=== end $bed failed scenarios ===" >&2
    rm -rf "$log_root"
    exit 1
  fi
  rm -rf "$log_root"
  echo "$success_line"
  exit 0
}

if (( ! fixture_bed_child )); then
  fixture_bed_script=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/$(basename "${BASH_SOURCE[0]}")
  run_fixture_bed_scenarios dispatch \
    "dispatch, adapter selftest, and mission-runner fixtures passed" \
    "$fixture_bed_script" dispatch mission-runner adapter-selftest steward-continuation
fi
case "$fixture_scenario" in
  dispatch | mission-runner | adapter-selftest | steward-continuation) ;;
  *) echo "dispatch fixtures: unknown scenario: $fixture_scenario" >&2; exit 64 ;;
esac

# The wall preflight demands a CLEAN initial baseline at start:
# close everything the bed laid down in tracked space, exactly as a real
# mission repository begins.
close_bed_baseline() { # repo
  git -C "$1" add -A . >/dev/null 2>&1 || true
  if ! git -C "$1" diff --cached --quiet 2>/dev/null; then
    git -C "$1" -c user.name=fixture -c user.email=fixture@example.invalid \
      commit -qm 'bed baseline' >/dev/null
  fi
}

# The dispatcher, adapter selftest, and mission-runner E2E fixtures
# (script-validate-4/D35): extracted verbatim (one dedent) from
# validate-metasystem.sh's largest inline block into the sub-suite shape
# the file already used everywhere else. The orchestrator keeps the
# delegate-scope/delivery-contract gating; this script owns its temp tree,
# its armed-supervision shutdown, and its failure-evidence preservation.

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
cd "$root"
source scripts/agents/fixture-budget.sh
harness_fixture_warn_if_engine_stale "$root"
# Standalone runs resolve their own cap scale exactly as the sibling
# suites do; under the battery the resolved env is inherited and the
# init is a no-op re-resolution.
harness_fixture_budget_init "$root"
fixture_minimum_cap_min=$(harness_fixture_semantic_cap minimum-minutes)
fixture_mission_job_cap_min=$(harness_fixture_semantic_cap mission-job-minutes)
fixture_dispatch_envelope_cap_min=$(harness_fixture_semantic_cap dispatch-envelope-minutes)
fixture_dispatch_over_envelope_cap_min=$(harness_fixture_semantic_cap dispatch-over-envelope-minutes)

command -v python3 >/dev/null 2>&1 \
  || { echo "${0##*/}: python3 is required by the TTY escalation driver (the metasystem itself does not need it)" >&2; exit 1; }

# The engine does the structural JSON work below; functions use the
# absolute path because agent-driver functions run from varying cwds.
# Rebuild before copying the engine into fixture repositories. The shell and
# binary must expose the same command set or a missing verb looks like a process
# identity timeout instead of a stale test artifact.
bash scripts/agents/go-build.sh >/dev/null
engine="$root/bin/metasystem"

# Atomically replace one top-level field of a JSON object file, leaving
# every other field exactly as the file's parser sees it. `json set`
# covers string and integer fields; this covers null and the object and
# array fields it cannot spell. The whole file and the field are rendered
# by the same engine encoder, so the needle below is byte-exact; a failed
# splice or an unparseable result refuses instead of writing. (Copied from
# supervision-fixtures.sh, extended two ways: string-valued fields print
# bare, so those retry with the quoted spelling; and a read-back proves
# the edit landed on the top-level field, not a nested lookalike.)
json_replace_field() { # file, top-level field, replacement JSON value
  local file=$1 field=$2 new=$3 compact old out canonical staged
  compact=$("$engine" json get --value "{\"root\":$(cat "$file")}" --field root) \
    || { echo "json_replace_field: $file did not parse" >&2; return 1; }
  old=$("$engine" json get --file "$file" --field "$field") \
    || { echo "json_replace_field: $file has no $field" >&2; return 1; }
  out=${compact/"\"$field\":$old"/"\"$field\":$new"}
  if [[ "$out" == "$compact" ]]; then
    out=${compact/"\"$field\":\"$old\""/"\"$field\":$new"}
  fi
  "$engine" util json-validate --value "$out" \
    || { echo "json_replace_field: editing $field left $file unparseable" >&2; return 1; }
  canonical=$("$engine" json get --value "{\"root\":$new}" --field root) \
    || { echo "json_replace_field: replacement for $field is not JSON: $new" >&2; return 1; }
  [[ "$("$engine" json get --value "$out" --field "$field")" == "$canonical" ]] \
    || { echo "json_replace_field: could not locate $field in $file" >&2; return 1; }
  staged=$(mktemp "$(dirname "$file")/.replace.XXXXXX") || return 1
  printf '%s\n' "$out" >"$staged"
  mv "$staged" "$file"
}

# Print one top-level element per line from the engine's compact rendering
# of a JSON array (or one "key":value member per line from an object). The
# walk is depth- and string-aware, so elements may nest objects and arrays
# — the flat-object splitter in supervision-fixtures.sh cannot. Compact
# rendering escapes control characters, so no element carries a newline.
json_elements() { # compact JSON array or object
  printf '%s' "$1" | awk '
    {
      n = length($0)
      first = substr($0, 1, 1)
      if (n < 2 || (first != "[" && first != "{")) exit 1
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

# The physical path of an existing file: symlinked parents resolved the
# way Path.resolve() saw them (macOS /tmp is such a symlink).
resolve_existing_path() { # path
  local dir
  dir=$(cd "$(dirname "$1")" && pwd -P) || return 1
  printf '%s/%s\n' "$dir" "$(basename "$1")"
}

tmp=$(mktemp -d)
tmp=$(cd "$tmp" && pwd -P)
# Every armed checkout in this bed shares a registry isolated to this run.
# Standalone fixtures must not read or write the user's supervision registry.
export METASYSTEM_SUPERVISION_REGISTRY_HOME="$tmp/supervision-home"
agent_supervision_repo=
armed_supervision_repos=()

# Fake-runtime fixtures authorize their disposable enrollment by staging the
# accepted engine identity directly. The installed engine lives under this
# bed's scratch root, so neither the source checkout nor a person's
# installation is enrolled.
fixture_install=$tmp/fixture-install
mkdir -p "$fixture_install/bin" "$fixture_install/scripts/agents"
printf 'metasystem.runtimes=fake\n' >"$fixture_install/metasystem.conf"
cp "$engine" "$fixture_install/bin/metasystem"
cp -R "$root/scripts/agents/adapters" "$fixture_install/scripts/agents/"
enrolled_engine=$fixture_install/bin/metasystem
export METASYSTEM_BIN="$enrolled_engine"

enroll_fixture_repo() { # repository, optional installed engine
  local repo=$1 repo_engine=${2:-$enrolled_engine} digest identity_dir
  repo=$(cd "$repo" && pwd -P)
  repo_engine=$(cd "$(dirname "$repo_engine")" && pwd -P)/$(basename "$repo_engine")
  digest=$("$repo_engine" util sha256 --file "$repo_engine")
  identity_dir=$repo/artifacts/agents/steward
  mkdir -p "$identity_dir"
  printf '{"repoIdentity":"%s","generation":1,"installPath":"%s","installDigest":"sha256:%s","mintedAt":"1970-01-01T00:00:00Z"}\n' \
    "$repo" "$repo_engine" "$digest" >"$identity_dir/identity.json"
  chmod 0600 "$identity_dir/identity.json"
}

run_fixture_arm() { # description, output file or -, command...
  local description=$1 output_file=$2 arm_rc
  shift 2
  echo "dispatch fixture arm: $description" >&2
  if [[ "$output_file" == - ]]; then
    if METASYSTEM_BIN="$enrolled_engine" "$@" >&2; then arm_rc=0; else arm_rc=$?; fi
  else
    if METASYSTEM_BIN="$enrolled_engine" "$@" >"$output_file" 2>&1; then arm_rc=0; else arm_rc=$?; fi
    cat "$output_file" >&2
  fi
  echo "dispatch fixture arm result: $description (exit status $arm_rc)" >&2
  return "$arm_rc"
}

track_armed_supervision() { # repository
  local repo=$1 known
  [[ -n "$repo" ]] || return 0
  for known in ${armed_supervision_repos[@]+"${armed_supervision_repos[@]}"}; do
    [[ "$known" == "$repo" ]] && return 0
  done
  armed_supervision_repos+=("$repo")
}
cleanup() {
  status=$?
  local repo
  for repo in ${armed_supervision_repos[@]+"${armed_supervision_repos[@]}"}; do
    [[ -x "$repo/scripts/agents/arm-supervision.sh" ]] || continue
    if [[ "$repo" == "${runner_repo:-}" ]] && declare -p runner_process_env >/dev/null 2>&1; then
      run_fixture_arm "cleanup shutdown for $repo" - \
        "${runner_process_env[@]}" "$repo/scripts/agents/arm-supervision.sh" \
          --repo "$repo" --shutdown \
        || echo "dispatch fixture cleanup shutdown failed: $repo" >&2
    elif [[ "$repo" == "${steward_repo:-}" && -n "${steward_enrolled_engine:-}" ]]; then
      run_fixture_arm "cleanup shutdown for $repo" - \
        env METASYSTEM_BIN="$steward_enrolled_engine" \
          "$repo/scripts/agents/arm-supervision.sh" --repo "$repo" --shutdown \
        || echo "dispatch fixture cleanup shutdown failed: $repo" >&2
    else
      run_fixture_arm "cleanup shutdown for $repo" - \
        "$repo/scripts/agents/arm-supervision.sh" --repo "$repo" --shutdown \
        || echo "dispatch fixture cleanup shutdown failed: $repo" >&2
    fi
  done
  # Kill any job child still rooted under this run's temp dir before the
  # dir is preserved or removed. The process-loss/timed/cancelled/
  # mission-lease fixtures spawn `util hold` children the reaper is meant
  # to reap; when an assertion fails BEFORE that reap, arm-supervision
  # --shutdown stops the reaper but leaves those children orphaned, and
  # the failure branch below PRESERVES the temp dir instead of deleting
  # it, so the child would otherwise run forever. Each such leak adds
  # process pressure that slows the next run's reaper past its assertion
  # window — a self-compounding flake. $tmp is this run's unique mktemp
  # dir, so every process whose argv references it is this fixture's own;
  # nothing else can match. (The suite runner's own pid never carries
  # $tmp in its command line.)
  local strays
  if [[ -n "$tmp" && "$tmp" == /*/tmp.* ]]; then
    strays=$(pgrep -f "$tmp" 2>/dev/null || true)
    if [[ -n "$strays" ]]; then
      kill -TERM $strays 2>/dev/null || true
      sleep 1
      kill -KILL $(pgrep -f "$tmp" 2>/dev/null || true) 2>/dev/null || true
    fi
  fi
  if [[ $status != 0 && -d "$tmp" ]]; then
    keep="artifacts/agents/suite-failures/$(date -u +%Y%m%dT%H%M%SZ)-dispatch-$$"
    mkdir -p "$(dirname "$keep")"
    mv "$tmp" "$keep" 2>/dev/null && echo "dispatch fixture evidence preserved: $keep" >&2
    return "$status"
  fi
  rm -rf "$tmp" 2>/dev/null || { sleep 1; rm -rf "$tmp" 2>/dev/null || true; }
  return "$status"
}
trap cleanup EXIT

# The cap owner is exercised through the shipped job verbs before the full
# dispatch-driver bed below.
cap_fixture_round() { # repository, root job, round, completed kind or protocol
  local repo=$1 chain=$2 round=$3 kind=$4 parent=null role=design-critic job
  job=$chain
  if (( round > 1 )); then
    job="$chain-r$round"
    if (( round == 2 )); then parent="\"$chain\""; else parent="\"$chain-r$((round - 1))\""; fi
  fi
  mkdir -p "$repo/artifacts/agents/jobs"
  if [[ "$kind" == protocol ]]; then
    printf '{"jobId":"%s","role":"%s","round":%d,"parentJob":%s,"status":"failed","error":"protocol_error","protocolError":{"key":"protocol-%d","violation":"malformed critic return"}' \
      "$job" "$role" "$round" "$parent" "$round" >"$repo/artifacts/agents/jobs/$job.json"
  else
    printf '{"jobId":"%s","role":"%s","round":%d,"parentJob":%s,"status":"completed"' \
      "$job" "$role" "$round" "$parent" >"$repo/artifacts/agents/jobs/$job.json"
  fi
  if (( round == 1 )); then
    printf ',"findingRegister":[],"findingRegisterRound":0,"boundedCritiqueStart":null,"critiqueExhaustions":[]' \
      >>"$repo/artifacts/agents/jobs/$job.json"
  fi
  printf '}\n' >>"$repo/artifacts/agents/jobs/$job.json"
  if [[ "$kind" != protocol ]]; then
    mkdir -p "$repo/artifacts/agents/$chain/rounds/$round"
    case "$kind" in
      bounded|severe)
        local prefix=B
        [[ "$kind" == severe ]] && prefix=S
        printf '{"schemaVersion":3,"jobId":"%s","round":%d,"findings":[{"id":"%s-1","severity":"high","material":true,"claim":"cap fixture finding","evidence":"direct fixture evidence"}],"rigor":[{"findingId":"%s-1","rigorClass":"%s","facts":{"local":true,"recoverable":true,"proofBoundaryCrossed":false,"authorityBoundaryCrossed":false,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":false},"reopeningTrigger":"reopen if it recurs"}]}\n' \
          "$job" "$round" "$prefix" "$prefix" "$kind" >"$repo/artifacts/agents/$chain/rounds/$round/return.json"
        ;;
      zero)
        printf '{"schemaVersion":3,"jobId":"%s","round":%d,"findings":[],"rigor":[]}\n' \
          "$job" "$round" >"$repo/artifacts/agents/$chain/rounds/$round/return.json"
        ;;
    esac
  fi
  "$engine" job critique-register-advance --repo "$repo" --root-job "$chain" --round-job "$job" >/dev/null
}

cap_fixture="$tmp/cap-engine"
printf '{"role":"code-critic"}\n' >"$cap_fixture-critic-record.json"
[[ "$("$engine" adapter adjudicate-turn --stage initial \
    --record "$cap_fixture-critic-record.json" --cli-status 7 --handshake-done)" \
    == 'finish failed protocol_error runtime' ]] \
  || { echo "a critic adapter crash did not fold to protocol_error" >&2; exit 1; }
[[ "$("$engine" adapter adjudicate-turn --stage empty-reply \
    --record "$cap_fixture-critic-record.json" --handshake-done)" \
    == 'finish failed protocol_error delivery' ]] \
  || { echo "an empty critic return did not fold to protocol_error" >&2; exit 1; }

set +e
"$engine" job exhaustion-patches --manifest nowhere --dir "$tmp" \
  >"$cap_fixture-retired-command.out" 2>&1
retired_command_rc=$?
set -e
[[ "$retired_command_rc" -eq 2 ]] \
  && grep -Fq 'unknown verb "exhaustion-patches"' "$cap_fixture-retired-command.out" \
  || { echo "the retired exhaustion-patches compatibility command is still live" >&2; exit 1; }

bounded_cap_repo="$cap_fixture/bounded"
cap_fixture_round "$bounded_cap_repo" bounded-chain 1 bounded
cap_fixture_round "$bounded_cap_repo" bounded-chain 2 zero
cap_fixture_round "$bounded_cap_repo" bounded-chain 3 zero
printf 'Address B-1.\n' >"$cap_fixture/bounded-message.md"
set +e
"$engine" job critique-exhaustion-advance --repo "$bounded_cap_repo" \
    --root-job bounded-chain --role design-critic --message "$cap_fixture/bounded-message.md" \
    --successor bounded-chain-r4 >"$cap_fixture/bounded.out" 2>&1
bounded_cap_rc=$?
set -e
[[ "$bounded_cap_rc" -eq 10 ]] \
  || { echo "bounded cap returned $bounded_cap_rc instead of typed human-raise exit 10" >&2; exit 1; }
grep -Fq 'reason=cap-exhausted-human-raise' "$cap_fixture/bounded.out" \
  && grep -Fq 'bounded critique cap is exhausted' "$cap_fixture/bounded.out" \
  || { echo "bounded cap did not raise its human-only refusal" >&2; exit 1; }

protocol_cap_repo="$cap_fixture/protocol"
cap_fixture_round "$protocol_cap_repo" protocol-chain 1 protocol
cap_fixture_round "$protocol_cap_repo" protocol-chain 2 protocol
printf 'protocol correction\n' >"$cap_fixture/protocol-message.md"
[[ "$($engine job critique-exhaustion-advance --repo "$protocol_cap_repo" \
    --root-job protocol-chain --role design-critic --message "$cap_fixture/protocol-message.md" \
    --successor protocol-chain-r3)" == none ]] \
  || { echo "off-cap protocol error exhausted early" >&2; exit 1; }
cap_fixture_round "$protocol_cap_repo" protocol-chain 3 protocol
if "$engine" job critique-exhaustion-advance --repo "$protocol_cap_repo" \
    --root-job protocol-chain --role design-critic --message "$cap_fixture/protocol-message.md" \
    --successor protocol-chain-r4 >"$cap_fixture/protocol-round3.out" 2>&1; then
  echo "round-three protocol error bought another unenumerated retry" >&2; exit 1
fi
grep -Fq 'synthetic-' "$cap_fixture/protocol-round3.out" \
  || { echo "round-three protocol refusal did not enumerate its synthetic finding" >&2; exit 1; }

severe_cap_repo="$cap_fixture/severe"
cap_fixture_round "$severe_cap_repo" severe-chain 1 bounded
cap_fixture_round "$severe_cap_repo" severe-chain 2 severe
cap_fixture_round "$severe_cap_repo" severe-chain 3 zero
printf 'Address B-1 and S-1.\n' >"$cap_fixture/severe-message.md"
[[ "$($engine job critique-exhaustion-advance --repo "$severe_cap_repo" \
    --root-job severe-chain --role design-critic --message "$cap_fixture/severe-message.md" \
    --successor severe-chain-r4)" == recorded ]] \
  || { echo "severe finding did not override the bounded deadline" >&2; exit 1; }
cap_fixture_round "$severe_cap_repo" severe-chain 4 protocol
cap_fixture_round "$severe_cap_repo" severe-chain 5 protocol
cap_fixture_round "$severe_cap_repo" severe-chain 6 protocol
set +e
"$engine" job critique-exhaustion-advance --repo "$severe_cap_repo" \
    --root-job severe-chain --role design-critic --message "$cap_fixture/severe-message.md" \
    --successor severe-chain-r7 >"$cap_fixture/severe-round6.out" 2>&1
severe_cap_rc=$?
set -e
[[ "$severe_cap_rc" -eq 10 ]] \
  || { echo "round-six exhaustion returned $severe_cap_rc instead of typed human-raise exit 10" >&2; exit 1; }
grep -Fq 'reason=cap-exhausted-human-raise' "$cap_fixture/severe-round6.out" \
  && grep -Fq 'terminal round 6' "$cap_fixture/severe-round6.out" \
  || { echo "round-six exhaustion did not raise its terminal refusal" >&2; exit 1; }

agent_fixture="$tmp/agent-fixture"
agent_repo="$agent_fixture/repo"
agent_evidence="$agent_fixture/evidence"
mkdir -p "$agent_repo/scripts" "$agent_repo/docs"
agent_repo=$(cd "$agent_repo" && pwd -P)
cp -R scripts/agents "$agent_repo/scripts/"
cp scripts/metasystem-config.sh scripts/assert-mission.sh scripts/assert-stop-loss.sh \
  scripts/assert-return-complete.sh scripts/assert-turn-prompt.sh \
  scripts/watch-background-jobs.sh "$agent_repo/scripts/"
cp docs/project-rules.md "$agent_repo/docs/"
cp metasystem.conf "$agent_repo/"
# The engine owns fake-runtime conf tailoring (script-fixtures-020/D49);
# only harness-specific overrides ride --set.
"$root/bin/metasystem" config tailor --conf "$agent_repo/metasystem.conf" --runtimes fake \
  --set evidence.root="$agent_evidence" \
  --set role.default.model.fake=fake-model \
  --set watch.interval-sec=5 \
  --set census.log-max-bytes=4096 \
  --set role.investigator.runtime=fake \
  --set role.default.model.fake=fake-model \
  --set model.tier.1=fake:fake-model \
  --set model.tier.2=fake:fake-premium
git -C "$agent_repo" init -q -b main
git -C "$agent_repo" add .
git -C "$agent_repo" -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm base
# Production resolves its engine as <repo>/bin/metasystem — an untracked
# build artifact that adoption ships. Stage the real engine the same way,
# after the base commit so it stays untracked exactly like production.
# The runner and selftest repositories below inherit it via cp -R.
mkdir -p "$agent_repo/bin"
cp bin/metasystem "$agent_repo/bin/metasystem"
enroll_fixture_repo "$agent_repo"
agent_dispatch="$agent_repo/scripts/agents/dispatch.sh"
fake_adapter="$agent_repo/scripts/agents/adapters/fake.sh"
agent_config="$agent_repo/scripts/metasystem-config.sh"
good_agent_conf="$agent_fixture/good-metasystem.conf"
cp "$agent_repo/metasystem.conf" "$good_agent_conf"

# Capability snapshots belong to the fixture run that consumes them. Mint the
# fake snapshots before cloning the other fixture repositories so none of the
# beds inherits a dated snapshot from an earlier run.
first_snapshot=$($fake_adapter probe)
second_snapshot=$($fake_adapter probe)
[[ "$first_snapshot" == *-001.json && "$second_snapshot" == *-002.json && "$first_snapshot" != "$second_snapshot" ]] \
  || { echo "fake probe did not create immutable sequence-suffixed snapshots" >&2; exit 1; }

# The mission runner and compound adapter selftest each get a pristine
# repository and supervisor. They run only after the main dispatch fixture
# set has shut down, so neither can queue behind its fixture state or reuse
# its synthetic-process supervision set.
runner_repo="$agent_fixture/runner-repo"
runner_evidence="$agent_fixture/runner-evidence"
cp -R "$agent_repo" "$runner_repo"
runner_repo=$(cd "$runner_repo" && pwd -P)
enroll_fixture_repo "$runner_repo"
# Fixture git writes race the previous mission's trailing anchor: runners
# are detached, so "the mission returned" does not mean "its last git op
# finished". Wait for a live lock, remove a dead one's leavings (a killed
# runner leaves index.lock forever), then run the git op.
runner_git_cap_sec=$(harness_fixture_cap runner-git-lock)
runner_git_stale_sec=$(( $(harness_fixture_base_cap runner-git-lock) / 2 ))
runner_git() {
  local deadline=$((SECONDS + runner_git_cap_sec))
  while [[ -e "$runner_repo/.git/index.lock" ]] && (( SECONDS < deadline )); do
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
  done
  if [[ -e "$runner_repo/.git/index.lock" ]]; then
    # Old enough is a dead runner's leaving and is removed; a lock that
    # vanishes mid-check (racing stat or unlink) is left alone.
    local lock_mtime=
    lock_mtime=$(stat -c %Y "$runner_repo/.git/index.lock" 2>/dev/null \
      || stat -f %m "$runner_repo/.git/index.lock" 2>/dev/null) || lock_mtime=
    if [[ -n "$lock_mtime" ]] && (( $(date +%s) - lock_mtime >= runner_git_stale_sec )); then
      rm -f "$runner_repo/.git/index.lock" 2>/dev/null || true
    fi
  fi
  # Harness acts, not agent ledger commits: the mission flow's own
  # goal mutations ENROLL the pre-commit guard (R2-11), and these
  # fixture-driven trunk/candidate movements bypass it by name.
  git -C "$runner_repo" -c core.hooksPath=/dev/null "$@"
}

conf_edit "$runner_repo/metasystem.conf" replace-line-first '^evidence[.]root=.*$' \
  "evidence.root=$runner_evidence"
agent_selftest_repo="$agent_fixture/selftest-repo"
agent_selftest_evidence="$agent_fixture/selftest-evidence"
cp -R "$agent_repo" "$agent_selftest_repo"
agent_selftest_repo=$(cd "$agent_selftest_repo" && pwd -P)
enroll_fixture_repo "$agent_selftest_repo"
conf_edit "$agent_selftest_repo/metasystem.conf" replace-line-first '^evidence[.]root=.*$' \
  "evidence.root=$agent_selftest_evidence"

agent_fixture_cap_sec=$(harness_fixture_cap agent-command)
agent_status_cap_sec=$(harness_fixture_cap agent-status)
agent_cleanup_cap_sec=$(harness_fixture_cap agent-cleanup)
agent_driver_stop_cap_sec=$(harness_fixture_cap agent-driver-stop)
METASYSTEM_FIXTURE_AGENT_STATUS_CAP_SEC=$agent_status_cap_sec
export METASYSTEM_FIXTURE_AGENT_STATUS_CAP_SEC

wait_for_agent_census_fresh() { # fixture name
  local name=$1 started=$SECONDS deadline=$((SECONDS + agent_fixture_cap_sec)) expected elapsed
  [[ -n "${agent_supervision_repo:-}" ]] || return 0
  # The engine's OWN freshness ruling (script-validate-2/D34): the same
  # verb every dispatch gates on, so the fixture can never drift from
  # the policy internal/dispatch enforces.
  while (( SECONDS < deadline )); do
    if "$agent_supervision_repo/bin/metasystem" job census-fresh \
        --root "$agent_supervision_repo" --repo "$agent_supervision_repo" --arm rearm \
        --verdict "$agent_supervision_repo/artifacts/agents/supervision/last-census.json" \
        --state "$agent_supervision_repo/artifacts/agents/supervision/state.json" >/dev/null 2>&1; then
      return 0
    fi
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
  done
  elapsed=$((SECONDS - started))
  echo "agent fixture timed out waiting for a fresh census: $name (elapsed: ${elapsed}s; scaled cap: ${agent_fixture_cap_sec}s)" >&2
  return 1
}

agent_fixture_job_from_args() {
  local previous= argument
  for argument in "$@"; do
    if [[ "$previous" == --job || "$previous" == --job-id ]]; then
      printf '%s\n' "$argument"
      return
    fi
    previous=$argument
  done
  printf '%s\n' -
}

agent_fixture_diagnostics() { # fixture name, job id or -, elapsed seconds
  local name=$1 job=$2 elapsed=$3 path
  echo "agent fixture timed out: $name (job: $job; elapsed: ${elapsed}s; scaled cap: ${agent_fixture_cap_sec}s)" >&2
  [[ "$job" != - ]] || return
  for path in \
    "$agent_repo/artifacts/agents/jobs/$job.json" \
    "$agent_repo/artifacts/agents/jobs/$job.log" \
    "$agent_repo/artifacts/agents/hb/$job.start" \
    "$agent_repo/artifacts/agents/hb/$job" \
    "$agent_repo/artifacts/agents/hb/$job.waiting"; do
    if [[ -e "$path" ]]; then
      echo "--- $path" >&2
      sed -n '1,240p' "$path" >&2
    else
      echo "--- missing: $path" >&2
    fi
  done
}

stop_timed_out_agent_fixture() { # fixture name, job id or -, driver pid, wait start
  local name=$1 job=$2 driver_pid=$3 wait_started=$4 cleanup_pid cleanup_started cleanup_deadline driver_started driver_deadline elapsed status=
  elapsed=$((SECONDS - wait_started))
  agent_fixture_diagnostics "$name" "$job" "$elapsed"
  if [[ "$job" != - && -f "$agent_repo/artifacts/agents/jobs/$job.json" ]]; then
    status=$("$engine" json get --file "$agent_repo/artifacts/agents/jobs/$job.json" \
      --field status --default malformed 2>/dev/null || true)
    if [[ "$status" == pending || "$status" == running ]]; then
      "$agent_dispatch" cancel --job "$job" >"$agent_fixture/$name-timeout-cancel.out" 2>&1 &
      cleanup_pid=$!
      cleanup_started=$SECONDS
      cleanup_deadline=$(( SECONDS + agent_cleanup_cap_sec ))
      while kill -0 "$cleanup_pid" 2>/dev/null && (( SECONDS < cleanup_deadline )); do sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"; done
      if kill -0 "$cleanup_pid" 2>/dev/null; then
        elapsed=$((SECONDS - cleanup_started))
        echo "agent fixture cleanup timed out: $name cancel pid $cleanup_pid (elapsed: ${elapsed}s; scaled cap: ${agent_cleanup_cap_sec}s)" >&2
        kill -TERM "$cleanup_pid" 2>/dev/null || true
        sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
        kill -KILL "$cleanup_pid" 2>/dev/null || true
      fi
    fi
  fi
  if kill -0 "$driver_pid" 2>/dev/null; then
    kill -TERM "$driver_pid" 2>/dev/null || true
    driver_started=$SECONDS
    driver_deadline=$(( SECONDS + agent_driver_stop_cap_sec ))
    while kill -0 "$driver_pid" 2>/dev/null && (( SECONDS < driver_deadline )); do sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"; done
    if kill -0 "$driver_pid" 2>/dev/null; then
      elapsed=$((SECONDS - driver_started))
      echo "agent fixture driver stop timed out: $name pid $driver_pid (elapsed: ${elapsed}s; scaled cap: ${agent_driver_stop_cap_sec}s); sending KILL" >&2
      kill -KILL "$driver_pid" 2>/dev/null || true
    fi
  fi
  exit 1
}

wait_for_agent_child_stopped() { # stopped-file path, failure message
  # The reaper's TERM and the held child's acknowledgement are two
  # processes: --wait returns on the terminal record, the child's
  # signal handler writes the file a beat later. Poll to the cap —
  # a missing acknowledgement still fails, an in-flight one never.
  local stopped_file=$1 message=$2 deadline=$(( SECONDS + agent_fixture_cap_sec ))
  while [[ ! -f "$stopped_file" ]]; do
    (( SECONDS < deadline )) || { echo "$message" >&2; exit 1; }
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
  done
}

cap_lock_fixture_acquire() { # fixture name
  local name=$1 directory="$agent_repo/artifacts/agents/supervision/cap-authority.lock.d"
  cap_lock_fixture_tag="metasystem-cap-lock-$name-$$-$RANDOM"
  "$engine" util hold --tag "$cap_lock_fixture_tag" &
  cap_lock_fixture_pid=$!
  "$engine" job owner-lock --command claim --dir "$directory" \
    --pid "$cap_lock_fixture_pid" --tag "$cap_lock_fixture_tag"
}

cap_lock_fixture_release() {
  local directory="$agent_repo/artifacts/agents/supervision/cap-authority.lock.d"
  "$engine" job owner-lock --command release --dir "$directory" \
    --pid "$cap_lock_fixture_pid" --tag "$cap_lock_fixture_tag"
  kill -TERM "$cap_lock_fixture_pid" 2>/dev/null || true
  wait "$cap_lock_fixture_pid" 2>/dev/null || true
  cap_lock_fixture_pid=
  cap_lock_fixture_tag=
}

wait_for_chain_lock() { # chain id, fixture name
  local chain=$1 name=$2 deadline=$(( SECONDS + agent_fixture_cap_sec ))
  while [[ ! -f "$agent_repo/artifacts/agents/locks/$chain.d/owner.json" ]]; do
    (( SECONDS < deadline )) \
      || { echo "$name did not acquire its chain lock" >&2; return 1; }
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
  done
}

wait_for_agent_fixture_process() { # fixture name, job id or -, exact child pid
  local name=$1 job=$2 child_pid=$3 started=$SECONDS deadline=$(( SECONDS + agent_fixture_cap_sec )) result
  while kill -0 "$child_pid" 2>/dev/null; do
    (( SECONDS < deadline )) || stop_timed_out_agent_fixture "$name" "$job" "$child_pid" "$started"
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
  done
  if wait "$child_pid"; then result=0; else result=$?; fi
  return "$result"
}

run_agent_fixture() { # fixture name, job id or -, command...
  local name=$1 job=$2 child_pid result
  shift 2
  case ${2:-} in
    dispatch|follow-up)
      if wait_for_agent_census_fresh "$name"; then
        :
      else
        result=$?
        return "$result"
      fi
      ;;
  esac
  "$@" &
  child_pid=$!
  if wait_for_agent_fixture_process "$name" "$job" "$child_pid"; then
    return 0
  else
    result=$?
  fi
  return "$result"
}

run_agent_fixture_captured() { # fixture name, job id or -, output file, command...
  local name=$1 job=$2 output=$3 child_pid captured_result captured_attempt
  shift 3
  # The supervisor heals itself asynchronously; a dispatch landing between
  # an arming publication and its confirming census is refused with a
  # transient "retry in a moment". Every driver shares this runner, so the
  # retry lives here once — pass, fail, and TTY paths behave identically.
  for captured_attempt in 1 2 3; do
    case ${2:-} in dispatch|follow-up) wait_for_agent_census_fresh "$name" ;; esac
    "$@" >"$output" 2>&1 &
    child_pid=$!
    # No errexit toggling here: set -e is global, not function-scoped, and
    # re-enabling it before returning nonzero detonates in a caller that
    # disabled it around this very call. The if-form never trips errexit.
    if wait_for_agent_fixture_process "$name" "$job" "$child_pid"; then
      return 0
    else
      captured_result=$?
    fi
    # Exit 9 is the typed arming-window transient (script-validate-3/D34)
    # — a contract, not a grep of the diagnostic's wording.
    [[ "$captured_result" -eq 9 ]] || return "$captured_result"
    # Retry is only safe while the refusal preceded job creation. Once a
    # record exists the dispatch got past the census check, the failure is
    # the real answer, and a re-dispatch would overwrite the record it is
    # asserting about (one-shot fake markers made that visible).
    if [[ "$job" != - && -e "$agent_repo/artifacts/agents/jobs/$job.json" ]]; then
      return "$captured_result"
    fi
    sleep 1
  done
  return "$captured_result"
}

agent_fails() { # output name, expected text, command...
  local name=$1 expected=$2 result job
  shift 2
  job=$(agent_fixture_job_from_args "$@")
  set +e
  run_agent_fixture_captured "$name" "$job" "$agent_fixture/$name.out" "$@"
  result=$?
  set -e
  if [[ $result -eq 0 ]]; then
    echo "agent fixture unexpectedly passed: $name" >&2
    exit 1
  fi
  [[ -z "$expected" ]] || grep -Fq "$expected" "$agent_fixture/$name.out" || {
    echo "agent fixture $name did not report: $expected" >&2
    cat "$agent_fixture/$name.out" >&2
    # When the wrong refusal is the generation transient, the supervision
    # state explains WHY the retries could not outrun it; dump it so one
    # failing run carries its own diagnosis.
    if [[ -n "${agent_supervision_repo:-}" ]]; then
      echo "--- supervision state at failure:" >&2
      cat "$agent_supervision_repo/artifacts/agents/supervision/state.json" >&2 2>/dev/null || true
      echo "--- last census:" >&2
      cat "$agent_supervision_repo/artifacts/agents/supervision/last-census.json" >&2 2>/dev/null || true
      echo "--- supervisor log tail:" >&2
      tail -15 "$agent_supervision_repo/artifacts/agents/supervision/supervisor.log" >&2 2>/dev/null || true
    fi
    exit 1
  }
}

run_tty_agent_fixture() { # fixture name, typed line, expected exit, command...
  local name=$1 typed=$2 expected_exit=$3 attempt tty_result
  shift 3
  # The supervisor heals itself asynchronously, so a dispatch can land in
  # the moment between an arming publication and the census that confirms
  # it; that refusal is transient and says "retry in a moment", which is
  # what agent_fails already does and this driver must do too.
  for attempt in 1 2 3; do
    wait_for_agent_census_fresh "$name"
    set +e
    run_tty_agent_fixture_once "$name" "$typed" "$expected_exit" "$@"
    tty_result=$?
    set -e
    [[ $tty_result -eq 0 ]] && return 0
    # The tty wrapper's exit-code propagation is unverified, so this one
    # site still reads the typed refusal's structured token from the
    # captured output (script-validate-3/D34 records the residue).
    grep -Fq 'censusGeneration=' "$agent_fixture/$name.out" 2>/dev/null || break
    sleep 1
  done
  return "$tty_result"
}

run_tty_agent_fixture_once() { # fixture name, typed line, expected exit, command...
  local name=$1 typed=$2 expected_exit=$3
  shift 3
  python3 - "$agent_fixture/$name.out" "$typed" "$expected_exit" "$agent_fixture_cap_sec" \
    "$agent_driver_stop_cap_sec" "$@" <<'PY'
import errno
import os
import pty
import select
import subprocess
import sys
import time
from pathlib import Path

output, typed, expected_exit, cap, stop_cap, *command = sys.argv[1:]
master, slave = pty.openpty()
process = subprocess.Popen(command, stdin=slave, stdout=slave, stderr=slave, close_fds=True)
os.close(slave)
os.write(master, (typed + "\n").encode())
chunks = []
deadline = time.monotonic() + int(cap)
while process.poll() is None:
  if time.monotonic() >= deadline:
      process.terminate()
      try:
          process.wait(timeout=int(stop_cap))
      except subprocess.TimeoutExpired:
          process.kill()
          process.wait()
      raise SystemExit(f"TTY fixture timed out: {' '.join(command)}")
  ready, _, _ = select.select([master], [], [], 0.05)
  if ready:
      try:
          chunks.append(os.read(master, 65536))
      except OSError as error:
          if error.errno != errno.EIO:
              raise
while True:
  try:
      chunk = os.read(master, 65536)
  except OSError as error:
      if error.errno == errno.EIO:
          break
      raise
  if not chunk:
      break
  chunks.append(chunk)
os.close(master)
Path(output).write_bytes(b"".join(chunks))
if process.returncode != int(expected_exit):
  # A failure that hides its transcript cannot be diagnosed after teardown.
  sys.stderr.write(b"".join(chunks).decode(errors="replace")[-2000:] + "\n")
  raise SystemExit(f"TTY fixture exit {process.returncode}, expected {expected_exit}")
PY
}

make_agent_brief() { # output, mode, optional marker/value lines...
  local output=$1 mode=$2
  shift 2
  sed "s/^Working Mode:.*/Working Mode: $mode/" "$agent_repo/scripts/agents/templates/brief.md" >"$output"
  for line in "$@"; do printf '\n%s\n' "$line" >>"$output"; done
}

wait_for_agent_status() { # job, expected
  local job=$1 expected=$2 observed= started=$SECONDS deadline=$((SECONDS + agent_status_cap_sec)) elapsed
  while (( SECONDS < deadline )); do
    observed=$(cd "$agent_repo" && scripts/agents/dispatch.sh status --job "$job" 2>/dev/null || true)
    [[ "$observed" == "$expected" ]] && return 0
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
  done
  elapsed=$((SECONDS - started))
  echo "agent fixture status timed out: $job -> $expected (last status: ${observed:-missing}; elapsed: ${elapsed}s; scaled cap: ${agent_status_cap_sec}s)" >&2
  return 1
}

wait_for_agent_chain_unlock() { # root job
  local job=$1 directory="$agent_repo/artifacts/agents/locks/$1.d" started=$SECONDS deadline=$((SECONDS + agent_status_cap_sec)) elapsed
  while [[ -e "$directory" ]]; do
    if (( SECONDS >= deadline )); then
      elapsed=$((SECONDS - started))
      echo "agent fixture chain unlock timed out: $job (elapsed: ${elapsed}s; scaled cap: ${agent_status_cap_sec}s)" >&2
      return 1
    fi
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
  done
}

if [[ "$fixture_scenario" == dispatch ]]; then
# Critique rounds have one documented driver: dispatch plus follow-up. The
# standalone raw-runtime driver is retired and must not reappear.
[[ ! -e "$root/scripts/agents/critique-round.sh" ]] \
  || { echo "the retired standalone critique driver is still installed" >&2; exit 1; }
# Configuration resolution is flag, environment, mode, plain, default.
config_order="$agent_fixture/config-order"
mkdir -p "$config_order/scripts" "$config_order/bin"
cp scripts/metasystem-config.sh "$config_order/scripts/"
cp bin/metasystem "$config_order/bin/metasystem"
cat >"$config_order/metasystem.conf" <<EOF
role.implementer.runtime=plain
mode.refactor.role.implementer.runtime=mode
plain.knob=plain-value
EOF
[[ "$("$config_order/scripts/metasystem-config.sh" get --key role.implementer.runtime --mode refactor --flag flag)" == flag ]] \
  || { echo "metasystem config did not prefer the flag" >&2; exit 1; }
[[ "$(METASYSTEM_ROLE_IMPLEMENTER_RUNTIME=environment "$config_order/scripts/metasystem-config.sh" get --key role.implementer.runtime --mode refactor)" == environment ]] \
  || { echo "metasystem config did not prefer the environment" >&2; exit 1; }
[[ "$(env -u METASYSTEM_ROLE_IMPLEMENTER_RUNTIME "$config_order/scripts/metasystem-config.sh" get --key role.implementer.runtime --mode refactor)" == mode ]] \
  || { echo "metasystem config did not resolve the mode scope" >&2; exit 1; }
[[ "$("$config_order/scripts/metasystem-config.sh" get --key plain.knob --mode refactor)" == plain-value ]] \
  || { echo "metasystem config did not resolve the plain key" >&2; exit 1; }
[[ "$("$config_order/scripts/metasystem-config.sh" get --key absent.knob --default built-in)" == built-in ]] \
  || { echo "metasystem config did not resolve the built-in default" >&2; exit 1; }
# An uncommitted local file carries values that must not ship to adopting
# projects. It outranks the committed conf and yields to the environment.
cat >"$config_order/metasystem.conf.local" <<'EOF'
plain.knob=local-value
EOF
[[ "$(env -u METASYSTEM_PLAIN_KNOB "$config_order/scripts/metasystem-config.sh" get --key plain.knob)" == local-value ]] \
  || { echo "metasystem config did not prefer the local override" >&2; exit 1; }
[[ "$(METASYSTEM_PLAIN_KNOB=environment "$config_order/scripts/metasystem-config.sh" get --key plain.knob)" == environment ]] \
  || { echo "local override outranked the environment" >&2; exit 1; }
[[ "$(env -u METASYSTEM_ROLE_IMPLEMENTER_RUNTIME "$config_order/scripts/metasystem-config.sh" get --key role.implementer.runtime --mode refactor)" == mode ]] \
  || { echo "local override disturbed a key it does not carry" >&2; exit 1; }
rm -f "$config_order/metasystem.conf.local"

"$agent_config" validate
no_tier_conf="$agent_fixture/no-tier-metasystem.conf"
grep -v '^model\.tier\.' "$good_agent_conf" >"$no_tier_conf"
cp "$no_tier_conf" "$agent_repo/metasystem.conf"
"$agent_config" validate >"$agent_fixture/no-tier-validate.out"
[[ $(grep -Fc 'INFO: model tiers are absent; dispatch overrides therefore always escalate' "$agent_fixture/no-tier-validate.out") -eq 1 ]] \
  || { echo "tier-absence validation fixture did not emit its one informational line" >&2; exit 1; }
cp "$good_agent_conf" "$agent_repo/metasystem.conf"
conf_edit "$agent_repo/metasystem.conf" replace-line-first '^model[.]tier[.]1=.*$' 'model.tier.one=fake:fake-model'
agent_fails malformed-tier-key 'not a supported model tier key' "$agent_config" validate
cp "$good_agent_conf" "$agent_repo/metasystem.conf"
conf_edit "$agent_repo/metasystem.conf" replace-line-first '^model[.]tier[.]1=.*$' 'model.tier.1=fake-model'
agent_fails malformed-tier-member 'not runtime-qualified' "$agent_config" validate
cp "$good_agent_conf" "$agent_repo/metasystem.conf"
# An adopted repository may rely on role.default.runtime while the template
# carries this role-specific key. Replace or append so both shapes gain one
# explicit invalid binding, then prove resolution sees it before validation.
if grep -q '^role[.]design-critic[.]runtime=' "$agent_repo/metasystem.conf"; then
  conf_edit "$agent_repo/metasystem.conf" replace-line-first '^role[.]design-critic[.]runtime=.*$' 'role.design-critic.runtime=ghost'
else
  printf 'role.design-critic.runtime=ghost\n' >>"$agent_repo/metasystem.conf"
fi
invalid_role_runtime=$("$agent_config" get --key role.design-critic.runtime) || {
  echo "invalid-role-runtime precondition failed: could not resolve role.design-critic.runtime after config mutation" >&2
  exit 1
}
[[ "$invalid_role_runtime" == ghost ]] || {
  echo "invalid-role-runtime precondition failed: role.design-critic.runtime resolved to '$invalid_role_runtime', expected 'ghost'" >&2
  exit 1
}
agent_fails invalid-role-runtime 'outside metasystem.runtimes' "$agent_config" validate
cp "$good_agent_conf" "$agent_repo/metasystem.conf"
printf 'mode.refactor.role.implementer.runtime=ghost\n' >>"$agent_repo/metasystem.conf"
agent_fails invalid-mode-runtime 'outside metasystem.runtimes' "$agent_config" validate
cp "$good_agent_conf" "$agent_repo/metasystem.conf"
printf 'role.default.model.ghost=ghost-model\n' >>"$agent_repo/metasystem.conf"
agent_fails invalid-model-runtime 'outside metasystem.runtimes' "$agent_config" validate
cp "$good_agent_conf" "$agent_repo/metasystem.conf"
conf_edit "$agent_repo/metasystem.conf" replace-line-first '^metasystem[.]runtimes=.*$' 'metasystem.runtimes=ghost'
agent_fails unsupported-runtime 'unsupported runtime' "$agent_config" validate
cp "$good_agent_conf" "$agent_repo/metasystem.conf"
conf_edit "$agent_repo/metasystem.conf" replace-line-first '^model[.]tier[.]1=.*$' 'model.tier.1='
agent_fails unmapped-model 'appears in 0 model tiers' "$agent_config" validate
cp "$good_agent_conf" "$agent_repo/metasystem.conf"
conf_edit "$agent_repo/metasystem.conf" replace-line-first '^model[.]tier[.]2=.*$' 'model.tier.2=fake:fake-model'
agent_fails duplicate-model-tier 'appears in 2 model tiers' "$agent_config" validate
cp "$good_agent_conf" "$agent_repo/metasystem.conf"
conf_edit "$agent_repo/metasystem.conf" delete-line-first '^role[.]design-critic[.]model[.]fake=.*$'
conf_edit "$agent_repo/metasystem.conf" delete-line-first '^role[.]default[.]model[.]fake=.*$'
agent_fails missing-runtime-model 'has no model.fake value' "$agent_config" validate
cp "$good_agent_conf" "$agent_repo/metasystem.conf"
conf_edit "$agent_repo/metasystem.conf" replace-line-first '^evidence[.]root=.*$' "evidence.root=$agent_repo/evidence"
agent_fails inside-evidence-root 'outside the repository' "$agent_config" validate
cp "$good_agent_conf" "$agent_repo/metasystem.conf"

# A direct record write simulates a pre-law survivor. It has a claim but no
# human-supplied structured budget, so admission must name the exact record and
# refuse before supervision or any job reservation.
budgetless_dispatch_repo="$agent_fixture/budgetless-dispatch-repo"
cp -R "$agent_repo" "$budgetless_dispatch_repo"
enroll_fixture_repo "$budgetless_dispatch_repo"
mkdir -p "$budgetless_dispatch_repo/plans"
cat >"$budgetless_dispatch_repo/plans/goals.md" <<'BUDGETLESS_LEDGER'
# Goals

## Current goal: fixture-serving — Seed the migrated current goal
- Origin: human
- Next step: Release this claim after migration.

## Queued goal: budgetless-survivor — Exercise missing-budget refusal
- Origin: main
- Next step: Wait for a human budget judgment.
BUDGETLESS_LEDGER
budgetless_ledger=$(cat "$budgetless_dispatch_repo/plans/goals.md" && printf x) && budgetless_ledger=${budgetless_ledger%x}
"$engine" json object ledger="$budgetless_ledger" \
  sha256="$(shasum -a 256 "$budgetless_dispatch_repo/plans/goals.md" | cut -d' ' -f1)" \
  >"$budgetless_dispatch_repo/plans/goals-accepted.json"
"$engine" json set --file "$budgetless_dispatch_repo/plans/goals-accepted.json" --int schemaVersion=1
git -C "$budgetless_dispatch_repo" -c core.hooksPath=/dev/null add plans
git -C "$budgetless_dispatch_repo" -c core.hooksPath=/dev/null \
  -c user.name=fixture -c user.email=fixture@example.invalid commit -qm 'budgetless fixture goals'
git -C "$budgetless_dispatch_repo" config metasystem.goal.machine budgetless-machine
git -C "$budgetless_dispatch_repo" config goal.sync-remote local
budgetless_source_digest=$("$budgetless_dispatch_repo/bin/metasystem" goal source-digest --root "$budgetless_dispatch_repo")
METASYSTEM_OWNER_LINEAGE=budgetless-fixture \
  METASYSTEM_GOAL_NOW=2000-01-01T00:00:00Z \
  "$budgetless_dispatch_repo/bin/metasystem" goal migrate --root "$budgetless_dispatch_repo" \
    --source-digest "$budgetless_source_digest" --sync-mode local --by wido >/dev/null
git -C "$budgetless_dispatch_repo" -c core.hooksPath=/dev/null reset -q --hard refs/heads/metasystem/goals
METASYSTEM_OWNER_LINEAGE=budgetless-fixture \
  "$budgetless_dispatch_repo/bin/metasystem" goal release --root "$budgetless_dispatch_repo" --id fixture-serving >/dev/null
git -C "$budgetless_dispatch_repo" -c core.hooksPath=/dev/null reset -q --hard refs/heads/metasystem/goals

budgetless_record="$budgetless_dispatch_repo/plans/goals/budgetless-survivor.md"
budgetless_goal_tip=$(git -C "$budgetless_dispatch_repo" rev-parse refs/heads/metasystem/goals)
budgetless_accepted_tip=$(git -C "$budgetless_dispatch_repo" rev-parse refs/metasystem/goals/accepted)
[[ "$budgetless_goal_tip" == "$budgetless_accepted_tip" ]] \
  || { echo "budgetless fixture started from unequal ledger refs" >&2; exit 1; }
conf_edit "$budgetless_record" replace-line-first '^- State: queued$' '- State: claimed'
conf_edit "$budgetless_record" replace-line-first '^- Revision: 1$' '- Revision: 2'
conf_edit "$budgetless_record" insert-after-first '^- Revision: 2$' \
  '- Claimed: machine=budgetless-machine lineage=budgetless-fixture at=2000-01-01T00:05:00Z'
conf_edit "$budgetless_record" insert-after-first \
  ' migrate actor=human:wido targets=budgetless-survivor$' \
  '- 2000-01-01T00:05:00Z 00000000000000000000000000-budgetless-machine-1a2b3c4d claim actor=budgetless-machine+budgetless-fixture targets=budgetless-survivor'
conf_edit "$budgetless_record" delete-line-first '^Integrity: sha256='
budgetless_integrity=$("$engine" util sha256 --file "$budgetless_record")
printf 'Integrity: sha256=%s\n' "$budgetless_integrity" >>"$budgetless_record"
git -C "$budgetless_dispatch_repo" -c core.hooksPath=/dev/null add plans/goals/budgetless-survivor.md
git -C "$budgetless_dispatch_repo" -c core.hooksPath=/dev/null \
  -c user.name=fixture -c user.email=fixture@example.invalid commit -qm 'simulate pre-law survivor'
budgetless_survivor_tip=$(git -C "$budgetless_dispatch_repo" rev-parse HEAD)
git -C "$budgetless_dispatch_repo" update-ref refs/heads/metasystem/goals \
  "$budgetless_survivor_tip" "$budgetless_goal_tip"
git -C "$budgetless_dispatch_repo" update-ref refs/metasystem/goals/accepted \
  "$budgetless_survivor_tip" "$budgetless_accepted_tip"
set +e
budgetless_admission=$(METASYSTEM_OWNER_LINEAGE=budgetless-fixture \
  "$budgetless_dispatch_repo/bin/metasystem" job goal-admission \
    --root "$budgetless_dispatch_repo" --stop-lineage budgetless-fixture 2>&1)
budgetless_admission_rc=$?
set -e
[[ "$budgetless_admission_rc" -eq 9 ]] \
  || { echo "budgetless claim admission returned $budgetless_admission_rc: $budgetless_admission" >&2; exit 1; }
grep -Fq 'BUDGET_UNKNOWN record=plans/goals/budgetless-survivor.md goal=budgetless-survivor' <<<"$budgetless_admission" \
  || { echo "budgetless refusal did not name the exact goal record: $budgetless_admission" >&2; exit 1; }
budgetless_brief="$agent_fixture/budgetless.md"
make_agent_brief "$budgetless_brief" design
set +e
env METASYSTEM_OWNER_LINEAGE=budgetless-fixture \
  "$budgetless_dispatch_repo/scripts/agents/dispatch.sh" dispatch \
    --role design-critic --brief "$budgetless_brief" --job-id budgetless-refused \
    >"$agent_fixture/budgetless-refused.out" 2>&1
budgetless_dispatch_rc=$?
set -e
(( budgetless_dispatch_rc != 0 )) \
  || { echo "budgetless-goal dispatch unexpectedly passed" >&2; exit 1; }
grep -Fq 'BUDGET_UNKNOWN record=plans/goals/budgetless-survivor.md' "$agent_fixture/budgetless-refused.out" \
  || { echo "budgetless dispatch did not print the typed refusal" >&2; cat "$agent_fixture/budgetless-refused.out" >&2; exit 1; }
[[ ! -e "$budgetless_dispatch_repo/artifacts/agents/jobs/budgetless-refused.json" ]] \
  || { echo "budgetless-refused dispatch created a job record" >&2; exit 1; }

# All remaining dispatch fixtures run behind a real armed fake-runtime set.
# The explicit synthetic process table is fixture-only and keeps this test
# deterministic in restricted environments where ps enumeration is denied.
agent_process_fixture="$agent_fixture/processes.json"
agent_identity_fixture="$agent_fixture/process-identities.json"
printf '[]\n' >"$agent_process_fixture"
printf '{}\n' >"$agent_identity_fixture"
export METASYSTEM_CENSUS_PROCESS_FILE="$agent_process_fixture"
export METASYSTEM_FAKE_PROCESS_IDENTITY_FILE="$agent_identity_fixture"

# A structured claim enters through the public goal verb with the complete
# tuple. Its first reservation is within every limit; that completed attempt
# then closes the admission seam exactly at the attempt boundary.
budget_dispatch_repo="$agent_fixture/budget-dispatch-repo"
budget_dispatch_evidence="$agent_fixture/budget-dispatch-evidence"
cp -R "$agent_repo" "$budget_dispatch_repo"
enroll_fixture_repo "$budget_dispatch_repo"
conf_edit "$budget_dispatch_repo/metasystem.conf" replace-line-first '^evidence[.]root=.*$' \
  "evidence.root=$budget_dispatch_evidence"
mkdir -p "$budget_dispatch_repo/plans"
cat >"$budget_dispatch_repo/plans/goals.md" <<'BUDGET_LEDGER'
# Goals

## Queued goal: structured-budget — Exercise structured dispatch admission
- Origin: main
- Next step: Dispatch while the complete tuple remains within its limits.
BUDGET_LEDGER
budget_ledger=$(cat "$budget_dispatch_repo/plans/goals.md" && printf x) && budget_ledger=${budget_ledger%x}
"$engine" json object ledger="$budget_ledger" \
  sha256="$(shasum -a 256 "$budget_dispatch_repo/plans/goals.md" | cut -d' ' -f1)" \
  >"$budget_dispatch_repo/plans/goals-accepted.json"
"$engine" json set --file "$budget_dispatch_repo/plans/goals-accepted.json" --int schemaVersion=1
git -C "$budget_dispatch_repo" -c core.hooksPath=/dev/null add plans
git -C "$budget_dispatch_repo" -c core.hooksPath=/dev/null \
  -c user.name=fixture -c user.email=fixture@example.invalid commit -qm 'structured budget fixture goals'
git -C "$budget_dispatch_repo" config metasystem.goal.machine budget-machine
git -C "$budget_dispatch_repo" config goal.sync-remote local
budget_source_digest=$("$budget_dispatch_repo/bin/metasystem" goal source-digest --root "$budget_dispatch_repo")
METASYSTEM_OWNER_LINEAGE=budget-fixture \
  METASYSTEM_GOAL_NOW=2000-01-01T00:00:00Z \
  "$budget_dispatch_repo/bin/metasystem" goal migrate --root "$budget_dispatch_repo" \
    --source-digest "$budget_source_digest" --sync-mode local --by wido >/dev/null
git -C "$budget_dispatch_repo" -c core.hooksPath=/dev/null reset -q --hard refs/heads/metasystem/goals
track_armed_supervision "$budget_dispatch_repo"
budget_main_start=$("$budget_dispatch_repo/bin/metasystem" proc started-at --pid "$$")
run_fixture_arm "structured-budget initial arm" "$agent_fixture/budget-arming.out" \
  env METASYSTEM_AGENT_RUNTIME=fake "$budget_dispatch_repo/scripts/agents/arm-supervision.sh" \
    --repo "$budget_dispatch_repo" --session budget-validator --pid "$$" \
    --start-time "$budget_main_start" --tag metasystem-main-fake-budget-validator
budget_claim=$(METASYSTEM_OWNER_LINEAGE=budget-fixture \
  METASYSTEM_GOAL_NOW=2000-01-01T00:05:00Z \
  "$budget_dispatch_repo/bin/metasystem" goal claim --root "$budget_dispatch_repo" --id structured-budget \
    --elapsed-limit 1d --attempt-limit 1 --reserved-job-minutes-limit 10000 --active-job-limit 2)
grep -Fq '"outcome":"confirmed"' <<<"$budget_claim" \
  || { echo "the complete-tuple structured claim was refused: $budget_claim" >&2; exit 1; }
budget_goal_revision=$("$budget_dispatch_repo/bin/metasystem" job goal-revision \
  --root "$budget_dispatch_repo" --goal structured-budget)
budget_brief="$agent_fixture/structured-budget.md"
sed 's/^Working Mode:.*/Working Mode: design/' \
  "$budget_dispatch_repo/scripts/agents/templates/brief.md" >"$budget_brief"
(
  agent_repo=$budget_dispatch_repo
  agent_dispatch="$budget_dispatch_repo/scripts/agents/dispatch.sh"
  agent_supervision_repo=$budget_dispatch_repo
  export METASYSTEM_OWNER_LINEAGE=budget-fixture
  export METASYSTEM_GOAL_NOW=2000-01-01T00:06:00Z
  run_agent_fixture structured-budget-within structured-budget-within \
    "$agent_dispatch" dispatch --role design-critic --brief "$budget_brief" \
      --job-id structured-budget-within --goal structured-budget --wait
)
budget_within_record="$budget_dispatch_repo/artifacts/agents/jobs/structured-budget-within.json"
[[ "$("$engine" json get --file "$budget_within_record" --field status)" == completed ]] \
  || { echo "the within-limits structured dispatch did not complete" >&2; exit 1; }
[[ "$("$engine" json get --file "$budget_within_record" --field goalId)" == structured-budget \
   && "$("$engine" json get --file "$budget_within_record" --field goalRevision)" == "$budget_goal_revision" ]] \
  || { echo "the within-limits dispatch did not bind the accepted structured goal revision" >&2; exit 1; }
set +e
budget_admission=$(METASYSTEM_OWNER_LINEAGE=budget-fixture \
  METASYSTEM_GOAL_NOW=2000-01-01T00:07:00Z \
  "$budget_dispatch_repo/bin/metasystem" job goal-admission \
    --root "$budget_dispatch_repo" --stop-lineage budget-fixture 2>&1)
budget_admission_rc=$?
set -e
[[ "$budget_admission_rc" -eq 9 ]] \
  || { echo "the structured attempt boundary did not close admission (rc=$budget_admission_rc): $budget_admission" >&2; exit 1; }
grep -Fq 'BUDGET_REFUSED: goal structured-budget' <<<"$budget_admission" \
  && grep -Fq 'attemptLimit used=1 limit=1' <<<"$budget_admission" \
  || { echo "the structured refusal did not name the exact attempt boundary: $budget_admission" >&2; exit 1; }
(
  agent_repo=$budget_dispatch_repo
  agent_dispatch="$budget_dispatch_repo/scripts/agents/dispatch.sh"
  agent_supervision_repo=$budget_dispatch_repo
  export METASYSTEM_OWNER_LINEAGE=budget-fixture
  export METASYSTEM_GOAL_NOW=2000-01-01T00:07:00Z
  agent_fails structured-budget-refused 'attemptLimit used=1 limit=1' \
    "$agent_dispatch" dispatch --role design-critic --brief "$budget_brief" \
      --job-id structured-budget-refused --goal structured-budget
)
[[ ! -e "$budget_dispatch_repo/artifacts/agents/jobs/structured-budget-refused.json" ]] \
  || { echo "the structured admission refusal created a job record" >&2; exit 1; }
budget_follow_message="$agent_fixture/structured-budget-follow-up.md"
cp "$budget_dispatch_repo/scripts/agents/templates/follow-up.md" "$budget_follow_message"
(
  agent_repo=$budget_dispatch_repo
  agent_dispatch="$budget_dispatch_repo/scripts/agents/dispatch.sh"
  agent_supervision_repo=$budget_dispatch_repo
  export METASYSTEM_OWNER_LINEAGE=budget-fixture
  export METASYSTEM_GOAL_NOW=2000-01-01T00:07:00Z
  agent_fails structured-budget-follow-up-refused 'attemptLimit used=1 limit=1' \
    "$agent_dispatch" follow-up --job structured-budget-within --message "$budget_follow_message"
)
[[ ! -e "$budget_dispatch_repo/artifacts/agents/jobs/structured-budget-within-r2.json" ]] \
  || { echo "the structured follow-up refusal created a child reservation" >&2; exit 1; }

agent_supervision_repo=$agent_repo
track_armed_supervision "$agent_repo"
agent_main_start=$("$agent_repo/bin/metasystem" proc started-at --pid "$$")
run_fixture_arm "dispatcher initial arm" "$agent_fixture/arming.out" \
  env METASYSTEM_AGENT_RUNTIME=fake "$agent_repo/scripts/agents/arm-supervision.sh" \
    --repo "$agent_repo" --session validator --pid "$$" \
    --start-time "$agent_main_start" --tag metasystem-main-fake-validator

happy_brief="$agent_fixture/happy.md"
make_agent_brief "$happy_brief" design
if run_agent_fixture happy happy "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id happy --wait; then
  :
else
  happy_dispatch_rc=$?
  echo "valid fake dispatch failed: happy (exit: $happy_dispatch_rc)" >&2
  exit "$happy_dispatch_rc"
fi
[[ "$(cd "$agent_repo" && scripts/agents/dispatch.sh status --job happy)" == completed ]] \
  || { echo "valid fake dispatch did not complete" >&2; exit 1; }

# Two wrappers for the same fresh operation queue at the short chain section.
# The second wrapper must reach claim-launch and report the first wrapper's
# reservation instead of failing at the shell lock or the standing job record.
repeat_fresh_release="$agent_fixture/repeat-fresh-release"
repeat_fresh_brief="$agent_fixture/repeat-fresh.md"
make_agent_brief "$repeat_fresh_brief" design "FAKE:custodial-critique=$repeat_fresh_release"
cap_lock_fixture_acquire repeat-fresh
wait_for_agent_census_fresh repeat-fresh-first
(cd "$agent_repo" && "$agent_dispatch" dispatch --role design-critic \
  --brief "$repeat_fresh_brief" --job-id repeat-fresh) \
  >"$agent_fixture/repeat-fresh-first.out" 2>&1 &
repeat_fresh_first_pid=$!
wait_for_chain_lock repeat-fresh repeat-fresh-first
(cd "$agent_repo" && "$agent_dispatch" dispatch --role design-critic \
  --brief "$repeat_fresh_brief" --job-id repeat-fresh) \
  >"$agent_fixture/repeat-fresh-second.out" 2>&1 &
repeat_fresh_second_pid=$!
sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
cap_lock_fixture_release
repeat_fresh_first_rc=0
repeat_fresh_second_rc=0
wait_for_agent_fixture_process repeat-fresh-first repeat-fresh "$repeat_fresh_first_pid" \
  || repeat_fresh_first_rc=$?
wait_for_agent_fixture_process repeat-fresh-second repeat-fresh "$repeat_fresh_second_pid" \
  || repeat_fresh_second_rc=$?
(( repeat_fresh_first_rc == 0 )) \
  || { echo "the winning repeated fresh wrapper failed with $repeat_fresh_first_rc" >&2; cat "$agent_fixture/repeat-fresh-first.out" >&2; exit 1; }
(( repeat_fresh_second_rc == 0 || repeat_fresh_second_rc == 3 )) \
  || { echo "the losing repeated fresh wrapper failed before claim-launch with $repeat_fresh_second_rc" >&2; cat "$agent_fixture/repeat-fresh-second.out" >&2; exit 1; }
grep -Eq '"outcome":"(BOUND|IN-PROGRESS)"' "$agent_fixture/repeat-fresh-second.out" \
  || { echo "the losing repeated fresh wrapper did not return a claim state" >&2; cat "$agent_fixture/repeat-fresh-second.out" >&2; exit 1; }
[[ -f "$agent_repo/artifacts/agents/jobs/repeat-fresh.json" ]] \
  || { echo "the repeated fresh operation created no job record" >&2; exit 1; }
touch "$repeat_fresh_release"
wait_for_agent_status repeat-fresh completed

# A busy session refuses before reserving payload-retry. The refusal must not
# leave its brief or round directory behind, because the same identifier is
# valid as soon as the temporary occupant becomes terminal.
payload_retry_digest=$("$engine" util sha256 --file "$happy_brief")
payload_retry_preparation="$agent_fixture/payload-retry-occupancy.json"
"$engine" job claim-occupancy-prepare --root "$agent_repo" \
  --session fake:payload-retry --output "$payload_retry_preparation"
"$engine" job claim-launch --root "$agent_repo" --opid payload-blocker \
  --session fake:payload-retry --dispatch-mode fresh --resumed-session "" \
  --runtime fake --model fake-model --role design-critic \
  --launch-mode shared-checkout --permission-envelope-digest "$payload_retry_digest" \
  --product-root "$agent_repo" --cap-min "$fixture_minimum_cap_min" \
  --conf "$agent_repo/metasystem.conf" --input-hash "$payload_retry_digest" \
  --creator-pid "$$" --occupancy-preparation "$payload_retry_preparation" >/dev/null
if run_agent_fixture_captured payload-retry-refusal payload-retry \
    "$agent_fixture/payload-retry-refusal.out" "$agent_dispatch" dispatch \
    --role design-critic --brief "$happy_brief" --job-id payload-retry; then
  echo "the occupied payload-retry session unexpectedly launched" >&2
  exit 1
else
  payload_retry_refusal_rc=$?
fi
(( payload_retry_refusal_rc == 1 )) \
  || { echo "the occupied payload-retry session returned $payload_retry_refusal_rc instead of 1" >&2; cat "$agent_fixture/payload-retry-refusal.out" >&2; exit 1; }
grep -Fq 'REFUSED-SESSION-BUSY' "$agent_fixture/payload-retry-refusal.out" \
  || { echo "payload-retry was not refused by claim-launch" >&2; cat "$agent_fixture/payload-retry-refusal.out" >&2; exit 1; }
[[ ! -e "$agent_repo/artifacts/agents/payload-retry" ]] \
  || { echo "a refused claim left payload-retry launch files behind" >&2; exit 1; }
payload_blocker_patch="$agent_fixture/payload-blocker-failed.json"
printf '{"error":"fixture-release"}\n' >"$payload_blocker_patch"
"$engine" job record-cas --root "$agent_repo" --job payload-blocker \
  --expect pending-setup --status failed --patch "$payload_blocker_patch" >/dev/null
run_agent_fixture payload-retry payload-retry "$agent_dispatch" dispatch \
  --role design-critic --brief "$happy_brief" --job-id payload-retry --wait
[[ "$("$engine" json get --file "$agent_repo/artifacts/agents/jobs/payload-retry.json" --field status)" == completed ]] \
  || { echo "payload-retry did not launch after its temporary refusal cleared" >&2; exit 1; }

# Exit honesty: a job that never existed answers FAST and speaks —
# watch says vanished (5) with the reason on stderr instead of
# burning its timeout; status names the missing record beside its 6.
set +e
never_watch_err=$(cd "$agent_repo" && scripts/agents/dispatch.sh watch --job never-was 2>&1 >/dev/null)
never_watch_rc=$?
never_status_err=$(cd "$agent_repo" && scripts/agents/dispatch.sh status --job never-was 2>&1 >/dev/null)
never_status_rc=$?
set -e
[[ $never_watch_rc -eq 5 ]] \
  || { echo "watch on a never-existed job must answer vanished (5), got $never_watch_rc" >&2; exit 1; }
grep -q "no job record" <<<"$never_watch_err" \
  || { echo "the fast-fail watch must speak its reason: $never_watch_err" >&2; exit 1; }
[[ $never_status_rc -eq 6 ]] \
  || { echo "status on a never-existed job answers 6, got $never_status_rc" >&2; exit 1; }
grep -q "no job record" <<<"$never_status_err" \
  || { echo "status must name the missing record on stderr: $never_status_err" >&2; exit 1; }
happy_record="$agent_repo/artifacts/agents/jobs/happy.json"
for happy_key in jobId role mission runtime round parentJob status phase error \
  workspaceRoot baseSha branch permissions capMin pid pidStartedAt pgid instanceTag custodyProcesses \
  sessionId turnId requestedModel effectiveModel overridden capabilitySnapshot \
  sessionEstablishedTimeoutSec input startedAt endedAt usage mirror; do
  "$engine" json get --file "$happy_record" --field "$happy_key" >/dev/null \
    || { echo "happy record lacks required field: $happy_key" >&2; exit 1; }
done
[[ "$("$engine" json get --file "$happy_record" --field status)" == completed ]] \
  || { echo "happy record is not completed" >&2; exit 1; }
happy_session_key=$("$engine" json get --file "$happy_record" --field sessionKey)
[[ "$happy_session_key" == fake:happy ]] \
  || { echo "fresh dispatch did not record its namespaced occupancy key" >&2; exit 1; }
[[ "$("$engine" json get --file "$happy_record" --field fingerprintVersion)" == 1 \
   && -n "$("$engine" json get --file "$happy_record" --field fingerprint)" \
   && "$("$engine" json get --file "$happy_record" --field dispatchMode)" == fresh \
   && -n "$("$engine" json get --file "$happy_record" --field creatorLiveness.pid)" \
   && -n "$("$engine" json get --file "$happy_record" --field reservationDeadline)" ]] \
  || { echo "ordinary fresh dispatch did not complete a fingerprinted claim-launch reservation" >&2; exit 1; }
[[ "$("$engine" json get --file "$happy_record" --field launchMode)" == shared-checkout \
   && "$("$engine" json get --file "$happy_record" --field productRoots)" == "[\"$agent_repo\"]" \
   && "$("$engine" json get --file "$happy_record" --field productRootScopes)" \
      == "[{\"path\":\"$agent_repo\",\"reason\":\"shared-checkout\",\"standing\":\"attribution-only\"}]" ]] \
  || { echo "ordinary shared-checkout dispatch did not record its workspace attribution default" >&2; exit 1; }
happy_session_digest=$(printf '%s' "$happy_session_key" | "$engine" util sha256)
happy_session_index="$agent_repo/artifacts/agents/sessions/$happy_session_digest.json"
[[ -f "$happy_session_index" \
   && "$("$engine" json get --file "$happy_session_index" --field sessionKey)" == "$happy_session_key" \
   && "$("$engine" json get --file "$happy_session_index" --field occupants)" == '[]' ]] \
  || { echo "fresh dispatch did not maintain and release its session occupancy index" >&2; cat "$happy_session_index" >&2 2>/dev/null || true; exit 1; }
happy_snapshot_rel=$("$engine" json get --file "$happy_record" --field capabilitySnapshot)
[[ "$happy_snapshot_rel" == *-002.json ]] \
  || { echo "happy record does not carry the second sequence snapshot: $happy_snapshot_rel" >&2; exit 1; }
[[ "$("$engine" json get --file "$happy_record" --field sessionEstablishedTimeoutSec)" \
   == "$("$engine" json get --file "$agent_repo/$happy_snapshot_rel" --field capabilities.sessionEstablishedTimeoutSec)" ]] \
  || { echo "happy record does not carry the snapshot's session-established timeout" >&2; exit 1; }
[[ "$("$engine" json get --file "$happy_record" --field permissions.requested.preset)" == none ]] \
  || { echo "happy record did not request the none preset" >&2; exit 1; }
happy_enforcement_rel=$("$engine" json get --file "$happy_record" --field permissions.enforcementSnapshot)
[[ "$happy_enforcement_rel" == "$happy_snapshot_rel" ]] \
  || { echo "happy enforcement snapshot is not the capability snapshot" >&2; exit 1; }
# Exact object equality: the engine's compact rendering is canonical
# (keys sorted), so one string compare covers keys and values both.
[[ "$("$engine" json get --file "$agent_repo/$happy_enforcement_rel" --field envelopeEnforcement)" \
   == '{"network":"mapped","readRoots":"notEnforced","writeRoots":"mapped"}' ]] \
  || { echo "happy snapshot envelope enforcement changed shape" >&2; exit 1; }
[[ "$("$engine" json get --file "$happy_record" --field input.delivery)" == stdin ]] \
  || { echo "happy input was not delivered on stdin" >&2; exit 1; }

# Every recorded custody group is attempted even when an earlier recycled
# group is no longer owned. The aggregate still refuses a clean wind-down, and
# the refusal remains visible, but the later owned group must receive TERM.
wind_down_ms="$agent_fixture/wind-down-ms.sh"
cat >"$wind_down_ms" <<'EOF'
#!/usr/bin/env bash
if [[ "$1 $2" == "job custody-groups" ]]; then
  printf '310\n320\n'
  exit 0
fi
if [[ "$1 $2" == "json get" ]]; then
  printf '999999\n'
  exit 0
fi
exit 1
EOF
chmod +x "$wind_down_ms"
wind_down_signal="$agent_fixture/wind-down-signal"
set +e
(
  source "$agent_dispatch"
  ms="$wind_down_ms"
  owned_group_alive=1
  group_alive() {
    [[ "$1" == 310 ]] && return 0
    (( owned_group_alive ))
  }
  group_owned() { [[ "$2" == 320 ]]; }
  kill() {
    printf '%s\n' "$*" >>"$wind_down_signal"
    owned_group_alive=0
  }
  wind_down_group "$agent_fixture/unused-record.json"
) 2>"$agent_fixture/wind-down.err"
wind_down_rc=$?
set -e
[[ $wind_down_rc -ne 0 ]] \
  || { echo "cross-group wind-down hid the unowned group refusal" >&2; exit 1; }
grep -Fq 'refusing to signal unowned process group 310' "$agent_fixture/wind-down.err" \
  || { echo "cross-group wind-down did not record the recycled group refusal" >&2; exit 1; }
grep -Fq -- '-TERM -- -320' "$wind_down_signal" \
  || { echo "cross-group wind-down stopped before signalling the later owned group" >&2; exit 1; }

# The custodial critique scenarios travel with the delegate --role path
# (operator-surface L13); until that verb lands there is no custodial entry
# to exercise here.

# Review roles default to a zero-write envelope in the live checkout even
# when repository configuration grants the same role writes for a quarantined
# worktree. An explicit live-checkout selection gets a custody-specific
# refusal, while --worktree keeps the configured write grant.
conf_edit "$agent_repo/metasystem.conf" replace-line-first \
  '^dispatch[.]permissions[.]design-critic=.*$' \
  'dispatch.permissions.design-critic=workspace'
review_default_brief="$agent_fixture/review-default.md"
make_agent_brief "$review_default_brief" design
run_agent_fixture review-default review-default "$agent_dispatch" dispatch \
  --role design-critic --brief "$review_default_brief" --job-id review-default --wait
review_default_effective="$agent_repo/artifacts/agents/review-default/rounds/1/effective-permissions.json"
[[ "$("$engine" json get --file "$review_default_effective" --field writeRoots)" == '[]' \
   && "$("$engine" json get --file "$review_default_effective" --field tools)" == read-only ]] \
  || { echo "a live-checkout review did not receive the zero-write effective envelope" >&2; cat "$review_default_effective" >&2; exit 1; }

review_follow_message="$agent_fixture/review-follow.md"
cp "$agent_repo/scripts/agents/templates/follow-up.md" "$review_follow_message"
run_agent_fixture review-default-follow-up review-default-r2 "$agent_dispatch" follow-up \
  --job review-default --message "$review_follow_message" --wait
review_follow_effective="$agent_repo/artifacts/agents/review-default/rounds/2/effective-permissions.json"
[[ "$("$engine" json get --file "$review_follow_effective" --field writeRoots)" == '[]' \
   && "$("$engine" json get --file "$review_follow_effective" --field tools)" == read-only ]] \
  || { echo "a live-checkout review follow-up did not inherit the zero-write effective envelope" >&2; cat "$review_follow_effective" >&2; exit 1; }

agent_fails review-live-write 'design-critic live-checkout write refusal' \
  "$agent_dispatch" dispatch --role design-critic --brief "$review_default_brief" \
  --workspace "$agent_repo" --job-id review-live-write
grep -Fq 'incident class: critic-workspace-custody' "$agent_fixture/review-live-write.out" \
  && grep -Fq 'pass --worktree' "$agent_fixture/review-live-write.out" \
  || { echo "the live-checkout review refusal did not name the incident class and override flag" >&2; cat "$agent_fixture/review-live-write.out" >&2; exit 1; }
review_live_record="$agent_repo/artifacts/agents/jobs/review-live-write.json"
[[ ! -e "$agent_repo/artifacts/agents/review-live-write" \
   && ! -e "$agent_repo/artifacts/agents/jobs/review-live-write.log" \
   && ! -e "$agent_repo/artifacts/agents/hb/review-live-write" ]] \
  || { echo "the writable live-review refusal launched or prepared an adapter process" >&2; exit 1; }
# A refused reservation may keep instanceTag as its launch-generation identity.
# It must not claim a launched process or custody: pid, pidStartedAt, pgid,
# custodyProcesses, and ownershipProof must all remain absent.
for process_field in pid pidStartedAt pgid custodyProcesses ownershipProof; do
  if "$engine" json get --file "$review_live_record" --field "$process_field" >/dev/null 2>&1; then
    echo "the writable live-review refusal recorded process or custody field: $process_field" >&2
    exit 1
  fi
done

run_agent_fixture review-worktree review-worktree "$agent_dispatch" dispatch \
  --role design-critic --brief "$review_default_brief" --job-id review-worktree --worktree --wait
review_worktree_record="$agent_repo/artifacts/agents/jobs/review-worktree.json"
review_worktree_root=$("$engine" json get --file "$review_worktree_record" --field workspaceRoot)
review_worktree_effective="$agent_repo/artifacts/agents/review-worktree/rounds/1/effective-permissions.json"
review_worktree_writes=$("$engine" json get --file "$review_worktree_effective" --field writeRoots)
[[ "$review_worktree_writes" == *"$review_worktree_root"* \
   && "$("$engine" json get --file "$review_worktree_effective" --field tools)" == runtime-default ]] \
  || { echo "a quarantined review worktree lost its configured write grant" >&2; cat "$review_worktree_effective" >&2; exit 1; }
conf_edit "$agent_repo/metasystem.conf" replace-line-first \
  '^dispatch[.]permissions[.]design-critic=.*$' \
  'dispatch.permissions.design-critic=none'

happy_input_bytes=$("$engine" json get --file "$happy_record" --field input.bytes)
[[ "$happy_input_bytes" =~ ^[0-9]+$ ]] && (( happy_input_bytes > 0 )) \
  || { echo "happy input bytes are not positive: $happy_input_bytes" >&2; exit 1; }
happy_prompt="$agent_repo/artifacts/agents/happy/rounds/1/prompt.md"
happy_prompt_head=$'Job-Id: happy\nRole: design-critic\nRuntime: fake\nModel: fake-model\nRound: 1\n'
head -c ${#happy_prompt_head} "$happy_prompt" | cmp -s - <(printf '%s' "$happy_prompt_head") \
  || { echo "happy prompt does not open with its identity header" >&2; sed -n '1,6p' "$happy_prompt" >&2; exit 1; }
happy_prompt_text=$(cat "$happy_prompt")
happy_preamble=$(cat "$agent_repo/scripts/agents/roles/design-critic.md")
happy_payload_brief=$(cat "$agent_repo/artifacts/agents/happy/brief.md")
happy_preamble_prefix=${happy_prompt_text%%"$happy_preamble"*}
[[ "$happy_preamble_prefix" != "$happy_prompt_text" ]] \
  || { echo "happy prompt lost the role preamble" >&2; exit 1; }
happy_brief_prefix=${happy_prompt_text%%"$happy_payload_brief"*}
[[ "$happy_brief_prefix" != "$happy_prompt_text" ]] \
  || { echo "happy prompt lost the brief" >&2; exit 1; }
(( ${#happy_preamble_prefix} < ${#happy_brief_prefix} )) \
  || { echo "happy prompt does not place the preamble before the brief" >&2; exit 1; }
# GOAL-08: --serving-goal joins the recorded brief bytes BEFORE the hash.
# Refusal first: no usable Current goal in the fixture repo refuses exit 3
# without creating a job. Then with a goal open, the payload brief carries
# the exact bounded section and the record's input hash equals the payload
# brief's own sha256 — the projection is inside the recorded bytes, so
# every fallback rebuild carries it.
set +e
(cd "$agent_repo" && scripts/agents/dispatch.sh dispatch --role design-critic \
  --brief "$happy_brief" --job-id sg-refused --serving-goal) >"$agent_fixture/sg-refused.out" 2>&1
sg_refused_rc=$?
set -e
(( sg_refused_rc == 3 )) \
  || { echo "--serving-goal without a usable goal did not refuse exit 3 (rc=$sg_refused_rc)" >&2; cat "$agent_fixture/sg-refused.out" >&2; exit 1; }
[[ ! -f "$agent_repo/artifacts/agents/jobs/sg-refused.json" ]] \
  || { echo "a refused --serving-goal dispatch left a job record" >&2; exit 1; }
# This serving goal is deliberately open but unclaimed. Admission therefore
# needs no budget tuple, while --serving-goal must still project it lawfully.
bin/metasystem goal open --root "$agent_repo" \
  --id fixture-serving --intent "Serve the fixture goal" --next "Dispatch with the projection." >/dev/null
run_agent_fixture serving-goal serving-goal "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id serving-goal --serving-goal --wait
sg_brief="$agent_repo/artifacts/agents/serving-goal/brief.md"
grep -Fq '# Serving goal (context, not instruction)' "$sg_brief" \
  && grep -Fq 'fixture-serving — Serve the fixture goal' "$sg_brief" \
  || { echo "the payload brief lacks the serving-goal section" >&2; exit 1; }
sg_recorded=$(bin/metasystem json get --file "$agent_repo/artifacts/agents/jobs/serving-goal.json" --field input.hash)
sg_actual=$(shasum -a 256 "$sg_brief" | cut -d' ' -f1)
[[ "$sg_recorded" == "$sg_actual" ]] \
  || { echo "the projection is not inside the recorded input hash ($sg_recorded != $sg_actual)" >&2; exit 1; }

# MON-04 (the human's item-1 waiter contract): a non-waiting dispatch
# prints the exact watch command, and the watch verb exits with the
# terminal status while holding the waiter record the verdict reads.
watch_line_out="$agent_fixture/watch-line.out"
(cd "$agent_repo" && scripts/agents/dispatch.sh dispatch --role design-critic \
  --brief "$happy_brief" --job-id watch-line-job) >"$watch_line_out" 2>&1
grep -Fq 'watch it with: scripts/agents/dispatch.sh watch --job watch-line-job' "$watch_line_out" \
  || { echo "non-wait dispatch did not print the watch command" >&2; cat "$watch_line_out" >&2; exit 1; }
watch_happy_rc=0
(cd "$agent_repo" && scripts/agents/dispatch.sh watch --job happy) || watch_happy_rc=$?
(( watch_happy_rc == 0 )) \
  || { echo "watching the completed happy job exited $watch_happy_rc, want 0" >&2; exit 1; }

mkdir -p "$agent_repo/artifacts/agents/locks/stale-lock.d"
printf '{"pid":999999,"instanceTag":"dead-owner","acquiredAt":"2000-01-01T00:00:00Z"}\n' >"$agent_repo/artifacts/agents/locks/stale-lock.d/owner.json"
run_agent_fixture stale-lock stale-lock "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id stale-lock --wait

generated=$(run_agent_fixture generated-dispatch - "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief")
[[ "$generated" =~ ^design-critic-[0-9]{8}t[0-9]{6}z-[a-f0-9]{4}$ ]] \
  || { echo "generated job id does not match the lowercase grammar: $generated" >&2; exit 1; }
jobid_reuse_brief="$agent_fixture/jobid-reuse.md"
make_agent_brief "$jobid_reuse_brief" design 'This launch request is distinct from the happy request.'
# A job id may be retried only for the same launch request. Different brief
# bytes change the input fingerprint, so they cannot reuse the standing job.
agent_fails jobid-reuse-fingerprint-mismatch 'REFUSED-OPID-MISMATCH' \
  "$agent_dispatch" dispatch --role design-critic --brief "$jobid_reuse_brief" --job-id happy
agent_fails malformed-job-id 'invalid job id' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id 'Bad_Id'
agent_fails contradictory-mode "contradicts the brief's Working Mode" "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --mode verify
agent_fails unregistered-override 'outside metasystem.runtimes' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --runtime ghost
agent_fails main-override 'assigned to main' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --runtime main
agent_fails costlier-unmapped 'cost direction is unranked' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --model absent-from-tier
agent_fails ranked-costlier 'higher (tier 1 -> tier 2)' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --model fake-premium

# The recorded default is a real fallback, while its absence refuses.
cp "$good_agent_conf" "$agent_repo/metasystem.conf"
conf_edit "$agent_repo/metasystem.conf" delete-line-first '^role[.]verifier[.]runtime=.*$'
verifier_brief="$agent_fixture/verifier.md"
make_agent_brief "$verifier_brief" verify
run_agent_fixture default-role default-role "$agent_dispatch" dispatch --role verifier --brief "$verifier_brief" --permissions none --job-id default-role --wait
cp "$agent_repo/metasystem.conf" "$agent_fixture/no-role.conf"
conf_edit "$agent_repo/metasystem.conf" delete-line-first '^role[.]default[.]runtime=.*$'
agent_fails no-role-default 'neither a runtime entry nor role.default.runtime' "$agent_dispatch" dispatch --role verifier --brief "$verifier_brief" --permissions none
cp "$good_agent_conf" "$agent_repo/metasystem.conf"
code_brief="$agent_fixture/code.md"
make_agent_brief "$code_brief" implement
review_target_brief="$agent_fixture/review-target.md"
make_agent_brief "$review_target_brief" implement
run_agent_fixture review-target review-target "$agent_dispatch" dispatch --role implementer --brief "$review_target_brief" --job-id review-target --worktree --wait
review_target_effective="$agent_repo/artifacts/agents/review-target/rounds/1/effective-permissions.json"
review_target_root=$("$engine" json get --file "$agent_repo/artifacts/agents/jobs/review-target.json" --field workspaceRoot)
[[ "$("$engine" json get --file "$review_target_effective" --field writeRoots)" == *"$review_target_root"* \
   && "$("$engine" json get --file "$review_target_effective" --field tools)" == runtime-default ]] \
  || { echo "the implementer role lost its existing writable worktree envelope" >&2; cat "$review_target_effective" >&2; exit 1; }
run_agent_fixture flag-runtime flag-runtime "$agent_dispatch" dispatch --role code-critic --brief "$code_brief" --reviews review-target --runtime fake --permissions none --job-id flag-runtime --wait
flag_runtime_record="$agent_repo/artifacts/agents/jobs/flag-runtime.json"
[[ "$("$engine" json get --file "$flag_runtime_record" --field runtime)" == fake \
   && "$("$engine" json get --file "$flag_runtime_record" --field overridden)" == true ]] \
  || { echo "flag runtime override was not recorded as overridden fake" >&2; exit 1; }
[[ "$("$engine" json get --file "$flag_runtime_record" --field reviews)" == review-target ]] \
  || { echo "flag-runtime record lost its reviews binding" >&2; cat "$flag_runtime_record" >&2; exit 1; }
investigator_brief="$agent_fixture/investigator.md"
make_agent_brief "$investigator_brief" take-a-step-back
run_agent_fixture investigator-role investigator-role "$agent_dispatch" dispatch --role investigator --brief "$investigator_brief" --runtime fake --permissions none --job-id investigator-role --wait

no_signal="$agent_fixture/no-signal.md"
make_agent_brief "$no_signal" design 'FAKE:no-session-signal'
agent_fails no-session-signal '' "$agent_dispatch" dispatch --role design-critic --brief "$no_signal" --job-id no-signal --wait
[[ "$("$engine" json get --file "$agent_repo/artifacts/agents/jobs/no-signal.json" --field status)" == failed ]] \
  || { echo "non-signal handshake did not end failed" >&2; exit 1; }
grep -Fq 'handshake_timeout' "$agent_repo/artifacts/agents/jobs/no-signal.json" \
  || { echo "non-signal handshake did not retain its error" >&2
       cat "$agent_repo/artifacts/agents/jobs/no-signal.json" >&2
       echo "--- dispatch said:" >&2
       cat "$agent_fixture/no-session-signal.out" >&2 2>/dev/null || true
       echo "--- job log:" >&2
       sed -n '1,60p' "$agent_repo/artifacts/agents/jobs/no-signal.log" >&2 2>/dev/null || true
       exit 1; }
handshake_failure="$agent_fixture/handshake-failure.md"
make_agent_brief "$handshake_failure" design 'FAKE:handshake-failure'
agent_fails handshake-failure '' "$agent_dispatch" dispatch --role design-critic --brief "$handshake_failure" --job-id handshake-failure --wait
grep -Fq 'authentication_failed' "$agent_repo/artifacts/agents/jobs/handshake-failure.json" \
  || { echo "fake handshake failure lost its named error" >&2; exit 1; }
missing_session="$agent_fixture/missing-session.md"
make_agent_brief "$missing_session" design 'FAKE:missing-session-id'
agent_fails missing-session '' "$agent_dispatch" dispatch --role design-critic --brief "$missing_session" --job-id missing-session --wait
grep -Fq 'handshake_missing_session_id' "$agent_repo/artifacts/agents/jobs/missing-session.json" \
  || { echo "missing session id did not fail the strong handshake" >&2; exit 1; }

pending_brief="$agent_fixture/pending.md"
make_agent_brief "$pending_brief" design 'FAKE:no-session-signal'
wait_for_agent_census_fresh pending-chain
(set +e; cd "$agent_repo"; scripts/agents/dispatch.sh dispatch --role design-critic --brief "$pending_brief" --job-id pending-chain >/dev/null 2>&1) & pending_driver=$!
wait_for_agent_status pending-chain pending
# Pending is published before launch identity. The dispatcher keeps the chain
# lock until that identity is durable, so wait for the full launch interval
# before testing the later status refusal.
wait_for_agent_chain_unlock pending-chain
pending_message="$agent_fixture/pending-follow.md"
cp "$agent_repo/scripts/agents/templates/follow-up.md" "$pending_message"
agent_fails pending-follow-up 'pending, running, timeout, or process-lost' "$agent_dispatch" follow-up --job pending-chain --message "$pending_message"
wait_for_agent_fixture_process pending-chain-driver pending-chain "$pending_driver" || true

# A just-created pending record with no launched supervisor is inside its
# recorded handshake budget, so a sweep leaves it pending. Once that window
# ends, the fingerprint and creator breadcrumb route the identityless
# reservation through reconciliation. Complete nonce-census absence plus the
# dead creator fails it as creator-abandoned; no process group existed, so the
# record must never claim process loss or group death.
launch_window_source="$agent_fixture/launch-window.json"
launch_window_pending="$agent_fixture/launch-window-pending.json"
# The claim creates the real fingerprinted pending-setup reservation, including
# its nonce tag and exact creator breadcrumb. Record setup then preserves those
# fields while completing the same reservation into the launch-window shape.
run_agent_fixture launch-window-claim launch-window "$engine" job claim-launch \
  --root "$agent_repo" --opid launch-window --session fake:launch-window \
  --dispatch-mode fresh --resumed-session '' --runtime fake --model fake-model \
  --role design-critic --launch-mode shared-checkout \
  --permission-envelope-digest 1111111111111111111111111111111111111111111111111111111111111111 \
  --input-hash 2222222222222222222222222222222222222222222222222222222222222222 \
  --cap-min 120 --conf "$agent_repo/metasystem.conf" >/dev/null
# This record has not launched, so it carries no launch-time stamps: the
# handshake deadline among them, which is stamped when a dispatcher starts
# waiting on an adapter it just started.
"$engine" json strip --file "$agent_repo/artifacts/agents/jobs/happy.json" \
  --key ownershipProof --key chainUsage --key handshakeDeadline >"$launch_window_source"
# custodyProcesses first: emptying it removes the nested pid/pidStartedAt
# spellings, so the top-level nulling below cannot splice a lookalike.
json_replace_field "$launch_window_source" custodyProcesses '[]'
# The exact-micro/ticks/bootId spellings ride only where the platform
# proves them (L11: darwin native identity is whole seconds); null them
# only when present so the scenario holds on both platforms.
for launch_window_field in \
  parentJob error pid pidStartedAt pgid sessionId endedAt usage mirror \
  claimEpoch mainId goalId; do
  json_replace_field "$launch_window_source" "$launch_window_field" null
done
for launch_window_field in pidStartedAtExactMicro pidStartTicks bootId; do
  if "$engine" json get --file "$launch_window_source" --field "$launch_window_field" >/dev/null 2>&1; then
    json_replace_field "$launch_window_source" "$launch_window_field" null
  fi
done
# The landed setup comparison also binds operationId and the goal tuple;
# the source must speak the reservation's own identity for them.
for launch_window_field in goalRevision machineId approvedRef; do
  if "$engine" json get --file "$launch_window_source" --field "$launch_window_field" >/dev/null 2>&1; then
    json_replace_field "$launch_window_source" "$launch_window_field" null
  fi
done
"$engine" json set --file "$launch_window_source" \
  --field jobId=launch-window --field operationId=launch-window \
  --field status=pending --field phase=handshake \
  --field "startedAt=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --int round=1 --int sessionEstablishedTimeoutSec=60
cp "$launch_window_source" "$launch_window_pending"
run_agent_fixture launch-window-setup launch-window "$agent_dispatch" __record-setup --job launch-window --source "$launch_window_pending"
run_agent_fixture launch-window-young-reap launch-window "$agent_dispatch" reap --job launch-window
launch_window_record="$agent_repo/artifacts/agents/jobs/launch-window.json"
[[ "$("$engine" json get --file "$launch_window_record" --field status)" == pending ]] \
  || { echo "pending record was reaped inside its handshake window" >&2; exit 1; }
# A LIVE record rewrite: json set stages beside the file and renames over
# it, so the sweeping reaper never observes a torn read.
"$engine" json set --file "$launch_window_record" --field startedAt=2000-01-01T00:00:00Z
run_agent_fixture launch-window-old-reap launch-window "$agent_dispatch" reap --job launch-window
[[ "$("$engine" json get --file "$launch_window_record" --field status)" == failed \
   && "$("$engine" json get --file "$launch_window_record" --field error)" == creator-abandoned \
   && "$("$engine" json get --file "$launch_window_record" --field phase)" == reconciliation ]] \
  || { echo "an out-of-window identityless reservation did not reconcile creator abandonment" >&2; cat "$launch_window_record" >&2; exit 1; }
if "$engine" json get --file "$launch_window_record" --field groupDeathProvenAt >/dev/null 2>&1; then
  echo "creator abandonment incorrectly claimed process-group death" >&2
  cat "$launch_window_record" >&2
  exit 1
fi

pending_loss_brief="$agent_fixture/pending-loss.md"
make_agent_brief "$pending_loss_brief" design 'FAKE:pending-process-loss'
wait_for_agent_census_fresh pending-loss
(set +e; cd "$agent_repo"; scripts/agents/dispatch.sh dispatch --role design-critic --brief "$pending_loss_brief" --job-id pending-loss >/dev/null 2>&1) & pending_loss_driver=$!
wait_for_agent_status pending-loss pending
# The supervisor is dying while this runs, so a single sweep can legitimately
# land before the kill and observe a live match. The standing reaper sweeps on
# an interval, so sweeping until the transition or a scaled ceiling is the
# faithful check; the assertion itself is unchanged.
pending_loss_started=$SECONDS
pending_loss_deadline=$((SECONDS + agent_status_cap_sec))
until grep -Fq 'process-lost' "$agent_repo/artifacts/agents/jobs/pending-loss.json"; do
  if (( SECONDS >= pending_loss_deadline )); then
    echo "dead pending supervisor did not transition through reap (elapsed: $((SECONDS - pending_loss_started))s; scaled cap: ${agent_status_cap_sec}s)" >&2
    exit 1
  fi
  run_agent_fixture pending-loss-reap pending-loss "$agent_dispatch" reap --job pending-loss
  sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
done
wait_for_agent_fixture_process pending-loss-driver pending-loss "$pending_loss_driver" || true

# Recollection (return-recollection-on-process-lost): a critic whose
# supervisor dies AFTER the valid return lands is not failed work — the
# reap's recollection walk adjudicates the on-disk return and concludes
# the job completed with recollection provenance.
recollect_brief="$agent_fixture/recollect.md"
make_agent_brief "$recollect_brief" design 'FAKE:return-then-process-loss'
wait_for_agent_census_fresh recollect-loss
(set +e; cd "$agent_repo"; scripts/agents/dispatch.sh dispatch --role design-critic --brief "$recollect_brief" --job-id recollect-loss >/dev/null 2>&1) & recollect_driver=$!
wait_for_agent_status recollect-loss running
recollect_record="$agent_repo/artifacts/agents/jobs/recollect-loss.json"
recollect_started=$SECONDS
recollect_deadline=$((SECONDS + agent_status_cap_sec))
until [[ "$("$engine" json get --file "$recollect_record" --field status 2>/dev/null)" == completed ]]; do
  if (( SECONDS >= recollect_deadline )); then
    echo "recollection did not conclude the delivered-then-lost critic (elapsed: $((SECONDS - recollect_started))s; scaled cap: ${agent_status_cap_sec}s)" >&2
    cat "$recollect_record" >&2
    exit 1
  fi
  run_agent_fixture recollect-reap recollect-loss "$agent_dispatch" reap --job recollect-loss
  sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
done
[[ -n "$("$engine" json get --file "$recollect_record" --field recollectedAt 2>/dev/null)" \
   && "$("$engine" json get --file "$recollect_record" --field recollectedFrom)" == process-lost \
   && "$("$engine" json get --file "$recollect_record" --field error)" == null ]] \
  || { echo "the recollected record does not carry its provenance" >&2; cat "$recollect_record" >&2; exit 1; }
"$engine" validate return-complete --root "$agent_repo" --role design-critic \
  --file "$agent_repo/artifacts/agents/recollect-loss/rounds/1/return.json" \
  || { echo "the recollected return does not adjudicate" >&2; exit 1; }
wait_for_agent_fixture_process recollect-driver recollect-loss "$recollect_driver" || true

malformed_brief="$agent_fixture/malformed-return.md"
make_agent_brief "$malformed_brief" design 'FAKE:malformed-return'
set +e
run_agent_fixture malformed-return malformed-return "$agent_dispatch" dispatch --role design-critic --brief "$malformed_brief" --job-id malformed-return --wait
malformed_status=$?
set -e
[[ $malformed_status -eq 3 ]] || { echo "malformed return mapped to $malformed_status instead of 3" >&2; exit 1; }
grep -Fq 'protocol_error' "$agent_repo/artifacts/agents/jobs/malformed-return.json" \
  || { echo "malformed return did not record protocol_error" >&2; exit 1; }

interrupted="$agent_fixture/interrupted.md"
make_agent_brief "$interrupted" design 'FAKE:interrupted-atomic-write'
run_agent_fixture interrupted interrupted "$agent_dispatch" dispatch --role design-critic --brief "$interrupted" --job-id interrupted --wait
[[ -f "$agent_repo/artifacts/agents/record-locks/interrupted.interrupted" ]] \
  || { echo "interrupted atomic-write fixture did not leave its staged partial" >&2; exit 1; }
"$engine" util json-validate --file "$agent_repo/artifacts/agents/jobs/interrupted.json"
terminal_patch="$agent_fixture/terminal-race.json"
printf '{"error":"loser"}\n' >"$terminal_patch"
set +e
run_agent_fixture_captured terminal-race interrupted /dev/null "$agent_dispatch" __record-cas --job interrupted --expect running --status failed --patch "$terminal_patch"
terminal_race_status=$?
set -e
[[ $terminal_race_status -eq 3 && "$(cd "$agent_repo" && scripts/agents/dispatch.sh status --job interrupted)" == completed ]] \
  || { echo "terminal compare-and-set did not preserve the first writer" >&2; exit 1; }
agent_fails illegal-terminal-transition 'illegal job transition' "$agent_dispatch" __record-cas --job interrupted --expect completed --status failed --patch "$terminal_patch"

# The fake reports network access it was not granted. Now that the presets
# grant it by default, the request has to withhold it explicitly for the
# report to be wider than the request at all.
restrictive_permissions="$agent_fixture/restrictive-permissions.json"
printf '{"readRoots":["."],"writeRoots":[],"network":"deny","approvals":"deny","tools":"read-only"}\n' >"$restrictive_permissions"
effective_wider="$agent_fixture/effective-wider.md"
make_agent_brief "$effective_wider" design 'FAKE:effective-wider'
agent_fails effective-wider '' "$agent_dispatch" dispatch --role design-critic --brief "$effective_wider" --permissions "$restrictive_permissions" --job-id effective-wider --wait
grep -Fq 'permissions_mismatch:network' "$agent_repo/artifacts/agents/jobs/effective-wider.json" \
  || { echo "wider effective envelope did not record the mismatch" >&2; exit 1; }
permissive_permissions="$agent_fixture/permissive-permissions.json"
printf '{"readRoots":["."],"writeRoots":[],"network":"allow","approvals":"deny","tools":"read-only"}\n' >"$permissive_permissions"
effective_narrower="$agent_fixture/effective-narrower.md"
make_agent_brief "$effective_narrower" design 'FAKE:effective-narrower'
run_agent_fixture effective-narrower effective-narrower "$agent_dispatch" dispatch --role design-critic --brief "$effective_narrower" --permissions "$permissive_permissions" --job-id effective-narrower --wait

# The shipped presets grant network, and a repository may narrow it for every
# role at once. Until 2026-08-05 the adapters hard-coded network off and never
# read the field, so a job could be recorded as networked and still be cut off
# (KI-12); these fixtures exist so that cannot recur silently.
[[ "$("$engine" json get --file scripts/agents/permissions/workspace.json --field network)" == allow ]] \
  || { echo "the workspace preset no longer grants network" >&2; exit 1; }
[[ "$("$engine" json get --file scripts/agents/permissions/none.json --field network)" == allow ]] \
  || { echo "the none preset no longer grants network" >&2; exit 1; }
net_default="$agent_fixture/net-default.md"
make_agent_brief "$net_default" design
run_agent_fixture net-default net-default "$agent_dispatch" dispatch --role design-critic --brief "$net_default" --job-id net-default --wait
[[ "$("$engine" json get --file "$agent_repo/artifacts/agents/jobs/net-default.json" --field permissions.requested.network)" == allow ]] \
  || { echo "a delegate did not receive network by default" >&2; exit 1; }
printf 'dispatch.permissions.network=deny\n' >>"$agent_repo/metasystem.conf"
net_floor="$agent_fixture/net-floor.md"
make_agent_brief "$net_floor" design
run_agent_fixture net-floor net-floor "$agent_dispatch" dispatch --role design-critic --brief "$net_floor" --job-id net-floor --wait
[[ "$("$engine" json get --file "$agent_repo/artifacts/agents/jobs/net-floor.json" --field permissions.requested.network)" == deny ]] \
  || { echo "the repository network floor did not narrow the preset" >&2; exit 1; }
conf_edit "$agent_repo/metasystem.conf" delete-line-first '^dispatch[.]permissions[.]network=deny$'
agent_fails invalid-network-floor 'must be deny or allow' \
  env METASYSTEM_DISPATCH_PERMISSIONS_NETWORK=sometimes "$agent_dispatch" dispatch --role design-critic --brief "$net_default" --job-id bad-floor

agent_fails writable-without-worktree 'writable permissions require --worktree' \
  "$agent_dispatch" dispatch --role implementer --brief "$code_brief" --job-id no-worktree
custom_permissions="$agent_fixture/custom-permissions.json"
cat >"$custom_permissions" <<EOF
{"readRoots":["."],"writeRoots":["$agent_fixture/outside"],"network":"deny","approvals":"deny","tools":"runtime-default"}
EOF
agent_fails escaping-write-root 'escapes the job worktree' "$agent_dispatch" dispatch --role implementer --brief "$code_brief" --job-id escaping-root --worktree --permissions "$custom_permissions"

oversized="$agent_fixture/oversized.md"
make_agent_brief "$oversized" design 'FAKE:oversized-input'
head -c 70000 /dev/zero | tr '\0' x >>"$oversized"
agent_fails oversized-input 'pass a file reference' "$agent_dispatch" dispatch --role design-critic --brief "$oversized" --job-id oversized

# Reap owns process loss, absolute caps, group death, and terminal mirroring.
process_loss="$agent_fixture/process-loss.md"
make_agent_brief "$process_loss" design 'FAKE:process-loss'
set +e
run_agent_fixture process-loss process-loss "$agent_dispatch" dispatch --role design-critic --brief "$process_loss" --job-id process-loss --wait
process_loss_status=$?
set -e
[[ $process_loss_status -eq 3 ]] || { echo "process loss mapped to $process_loss_status instead of 3" >&2; exit 1; }
grep -Fq 'process-lost' "$agent_repo/artifacts/agents/jobs/process-loss.json" \
  || { echo "reap did not name process-lost" >&2; exit 1; }
wait_for_agent_child_stopped "$agent_repo/artifacts/agents/process-loss/rounds/1/child.stopped" \
  "reap did not TERM the orphaned process-loss child"
grep -Fq 'groupDeathProvenAt' "$agent_repo/artifacts/agents/jobs/process-loss.json" \
  || { echo "process-loss terminal record lacks group-death proof" >&2; exit 1; }

timeout_brief="$agent_fixture/timeout.md"
make_agent_brief "$timeout_brief" design 'FAKE:timeout'
timeout_result="$agent_fixture/timeout.status"
wait_for_agent_census_fresh timed
(
  set +e
  cd "$agent_repo"
  scripts/agents/dispatch.sh dispatch --role design-critic --brief "$timeout_brief" --job-id timed --cap-min "$fixture_minimum_cap_min" --wait
  printf '%s\n' "$?" >"$timeout_result"
) &
timeout_driver=$!
wait_for_agent_status timed running
# The engine judges the budget by capDeadline first (startedAt+capMin is
# only the fallback), so the fixture must backdate BOTH: backdating only
# startedAt left the real one-minute deadline live and the explicit reap
# inert, a coin-flip against the fixture's own wait ceiling.
#
# Rewrites of LIVE records are atomic (json set stages and renames): the
# waiting dispatcher classifies on every poll, classification reads every
# job record fail-closed, and a torn read here refused reap-held and mapped
# this timeout to wait exit 3 (VM, 2026-08-14, evidence 014657Z-2066978).
"$engine" json set --file "$agent_repo/artifacts/agents/jobs/timed.json" \
  --field startedAt=2000-01-01T00:00:00Z --field capDeadline=2000-01-01T00:01:00Z
run_agent_fixture timed-reap timed "$agent_dispatch" reap --job timed
wait_for_agent_fixture_process timed-driver timed "$timeout_driver"
[[ "$(cat "$timeout_result")" == 4 ]] || {
  echo "timeout did not map to wait exit 4 (got $(cat "$timeout_result"))" >&2
  cat "$agent_repo/artifacts/agents/jobs/timed.json" >&2
  echo "--- reaper log:" >&2
  tail -20 "$agent_repo/artifacts/agents/supervision/reaper.log" >&2 2>/dev/null || true
  exit 1; }
grep -Fq 'budget-cap' "$agent_repo/artifacts/agents/jobs/timed.json" \
  || { echo "absolute cap did not record budget-cap" >&2; exit 1; }
wait_for_agent_child_stopped "$agent_repo/artifacts/agents/timed/rounds/1/child.stopped" \
  "timeout did not TERM the whole owned group"
grep -Fq 'groupDeathProvenAt' "$agent_repo/artifacts/agents/jobs/timed.json" \
  || { echo "timeout terminal record lacks group-death proof" >&2; exit 1; }

cancel_result="$agent_fixture/cancel.status"
wait_for_agent_census_fresh cancelled
(
  set +e
  cd "$agent_repo"
  scripts/agents/dispatch.sh dispatch --role design-critic --brief "$timeout_brief" --job-id cancelled --wait
  printf '%s\n' "$?" >"$cancel_result"
) &
cancel_driver=$!
wait_for_agent_status cancelled running
run_agent_fixture cancelled-cancel cancelled "$agent_dispatch" cancel --job cancelled
wait_for_agent_fixture_process cancelled-driver cancelled "$cancel_driver"
[[ "$(cat "$cancel_result")" == 8 ]] || { echo "cancelled did not map to wait exit 8" >&2; exit 1; }
wait_for_agent_child_stopped "$agent_repo/artifacts/agents/cancelled/rounds/1/child.stopped" \
  "cancel did not TERM the whole owned group"
grep -Fq 'groupDeathProvenAt' "$agent_repo/artifacts/agents/jobs/cancelled.json" \
  || { echo "cancelled terminal record lacks group-death proof" >&2; exit 1; }

vanished_result="$agent_fixture/vanished.status"
wait_for_agent_census_fresh vanished
(
  set +e
  cd "$agent_repo"
  scripts/agents/dispatch.sh dispatch --role design-critic --brief "$timeout_brief" --job-id vanished --wait
  printf '%s\n' "$?" >"$vanished_result"
) &
vanished_driver=$!
vanished_wait_started=$SECONDS
vanished_wait_deadline=$((SECONDS + agent_status_cap_sec))
while (( SECONDS < vanished_wait_deadline )); do
  [[ -f "$agent_repo/artifacts/agents/hb/vanished.waiting" ]] && break
  sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
done
[[ -f "$agent_repo/artifacts/agents/hb/vanished.waiting" ]] \
  || { vanished_wait_elapsed=$((SECONDS - vanished_wait_started)); echo "agent fixture file wait timed out: vanished.waiting (job: vanished; elapsed: ${vanished_wait_elapsed}s; scaled cap: ${agent_status_cap_sec}s)" >&2; exit 1; }
mv "$agent_repo/artifacts/agents/jobs/vanished.json" "$agent_fixture/vanished.json"
wait_for_agent_fixture_process vanished-driver vanished "$vanished_driver"
mv "$agent_fixture/vanished.json" "$agent_repo/artifacts/agents/jobs/vanished.json"
[[ "$(cat "$vanished_result")" == 5 ]] || { echo "vanished record did not map to wait exit 5 (got $(cat "$vanished_result"))" >&2; exit 1; }
run_agent_fixture vanished-cancel vanished "$agent_dispatch" cancel --job vanished

cancel_race="$agent_fixture/cancel-race.md"
make_agent_brief "$cancel_race" design 'FAKE:cancel-race'
run_agent_fixture_captured cancel-race-dispatch cancel-race /dev/null "$agent_dispatch" dispatch --role design-critic --brief "$cancel_race" --job-id cancel-race
run_agent_fixture cancel-race-cancel cancel-race "$agent_dispatch" cancel --job cancel-race
[[ "$(cd "$agent_repo" && scripts/agents/dispatch.sh status --job cancel-race)" == completed ]] \
  || { echo "completion did not win the scripted cancellation race" >&2; exit 1; }

mirror_failure="$agent_fixture/mirror-failure.md"
make_agent_brief "$mirror_failure" design 'FAKE:mirror-failure'
run_agent_fixture mirror-retry mirror-retry "$agent_dispatch" dispatch --role design-critic --brief "$mirror_failure" --job-id mirror-retry --wait
[[ -f "$agent_repo/artifacts/agents/mirror-retry/.mirror-failed" ]] \
  || { echo "scripted first mirror failure did not occur" >&2; exit 1; }
mirror_retry_record="$agent_repo/artifacts/agents/jobs/mirror-retry.json"
if [[ "$("$engine" json get --file "$mirror_retry_record" --field mirror)" == null ]]; then
  run_agent_fixture mirror-retry-first-reap mirror-retry "$agent_dispatch" reap --job mirror-retry
fi
mirror_hash_before=$("$engine" json get --file "$mirror_retry_record" --field mirror.manifest)
run_agent_fixture mirror-retry-second-reap mirror-retry "$agent_dispatch" reap --job mirror-retry
mirror_hash_after=$("$engine" json get --file "$mirror_retry_record" --field mirror.manifest)
[[ "$mirror_hash_before" == "$mirror_hash_after" && "$(cd "$agent_repo" && scripts/agents/dispatch.sh status --job mirror-retry)" == completed ]] \
  || { echo "idempotent mirror retry changed terminal state or durable content" >&2; exit 1; }
mirror_home=$("$engine" json get --file "$mirror_retry_record" --field mirror.path) \
  || { echo "mirror-retry record carries no mirror stamp" >&2; exit 1; }
[[ -n "$mirror_home" && "$mirror_home" != null ]] \
  || { echo "mirror-retry mirror path is empty" >&2; exit 1; }
[[ "$("$engine" util sha256 --file "$mirror_home/manifest.json")" == "$mirror_hash_after" ]] \
  || { echo "mirror manifest digest does not match the record" >&2; exit 1; }
while IFS= read -r mirror_member; do
  [[ -n "$mirror_member" ]] || continue
  mirror_rel=${mirror_member#\"}; mirror_rel=${mirror_rel%%\"*}
  mirror_item=${mirror_member#*\":}
  [[ "$("$engine" util sha256 --file "$mirror_home/$mirror_rel")" \
     == "$("$engine" json get --value "$mirror_item" --field sha256)" ]] \
    || { echo "mirrored file digest mismatch: $mirror_rel" >&2; exit 1; }
done < <(json_elements "$("$engine" json get --file "$mirror_home/manifest.json" --field files)")

# Follow-ups are child records under one serialized, explicitly closed chain.
follow_message="$agent_fixture/follow.md"
cp "$agent_repo/scripts/agents/templates/follow-up.md" "$follow_message"

# Exhaustion precedes successor reservation. Build one severe code-critic
# chain through round three, prove its first-exhaustion refusal leaves no r4
# husk, let the implementation successor record that exhaustion, then retry
# the exact same r4 id successfully.
flag_runtime_return="$agent_repo/artifacts/agents/flag-runtime/rounds/1/return.json"
json_replace_field "$flag_runtime_return" findings \
  '[{"id":"CAP-DRIVER-1","severity":"high","material":true,"claim":"driver cap finding","evidence":"direct fixture evidence"}]'
json_replace_field "$flag_runtime_return" rigor \
  '[{"findingId":"CAP-DRIVER-1","rigorClass":"severe","facts":{"local":true,"recoverable":true,"proofBoundaryCrossed":false,"authorityBoundaryCrossed":false,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":false},"reopeningTrigger":"reopen if it recurs"}]'
run_agent_fixture cap-driver-round2 flag-runtime-r2 "$agent_dispatch" follow-up \
  --job flag-runtime --message "$follow_message" --wait
run_agent_fixture cap-driver-round3 flag-runtime-r3 "$agent_dispatch" follow-up \
  --job flag-runtime-r2 --message "$follow_message" --wait
set +e
run_agent_fixture_captured cap-driver-refusal flag-runtime-r4 "$agent_fixture/cap-driver-refusal.out" \
  "$agent_dispatch" follow-up --job flag-runtime-r3 --message "$follow_message" --wait
cap_driver_refusal_rc=$?
set -e
[[ "$cap_driver_refusal_rc" -ne 0 ]] \
  || { echo "the round-three code critique exhaustion did not refuse" >&2; exit 1; }
grep -Fq 'implementer follow-up' "$agent_fixture/cap-driver-refusal.out" \
  || { echo "the round-three refusal did not route through the implementation successor" >&2; exit 1; }
[[ ! -e "$agent_repo/artifacts/agents/jobs/flag-runtime-r4.json" ]] \
  || { echo "the pre-reservation cap refusal stranded a round-four record" >&2; exit 1; }
[[ ! -e "$agent_repo/artifacts/agents/flag-runtime/rounds/4" ]] \
  || { echo "the pre-reservation cap refusal stranded a round-four payload" >&2; exit 1; }
cap_driver_message="$agent_fixture/cap-driver-implementation.md"
printf 'Address every open finding, including CAP-DRIVER-1.\n' >"$cap_driver_message"
# This authorized internal mutation leaves the exact state a dispatcher crash
# would leave after the atomic advance and before reservation. The ordinary
# follow-up below retries the same successor and must build it once.
run_agent_fixture cap-driver-advance-before-crash - "$agent_dispatch" \
  __critique-exhaustion-advance --root-job review-target --role implementer \
  --message "$cap_driver_message" --successor review-target-r2
[[ ! -e "$agent_repo/artifacts/agents/jobs/review-target-r2.json" \
   && ! -e "$agent_repo/artifacts/agents/review-target/rounds/2" ]] \
  || { echo "the atomic critique advance created successor reservation or payload state" >&2; exit 1; }
run_agent_fixture cap-driver-implementation review-target-r2 "$agent_dispatch" follow-up \
  --job review-target --message "$cap_driver_message" --wait
run_agent_fixture cap-driver-retry flag-runtime-r4 "$agent_dispatch" follow-up \
  --job flag-runtime-r3 --message "$follow_message" --wait
[[ "$("$engine" json get --file "$agent_repo/artifacts/agents/jobs/flag-runtime-r4.json" --field status)" == completed ]] \
  || { echo "retrying the refused round-four id did not succeed" >&2; exit 1; }

# A delegate-shaped process cannot invoke either internal critique mutation.
# If authority were absent this already-folded retry would return unchanged.
set +e
(
  exec -a codex bash -c '"$1" __critique-register-advance --root-job flag-runtime --round-job flag-runtime-r3; code=$?; exit "$code"' \
    _ "$agent_dispatch"
) >"$agent_fixture/unauthorized-critique-mutation.out" 2>&1
unauthorized_critique_rc=$?
set -e
[[ "$unauthorized_critique_rc" -ne 0 ]] \
  && grep -Fq 'control-plane write requires the authenticated lease holder' \
    "$agent_fixture/unauthorized-critique-mutation.out" \
  || { echo "a delegate-shaped caller reached the internal critique mutation" >&2; cat "$agent_fixture/unauthorized-critique-mutation.out" >&2; exit 1; }

# Wardens consume the same severe finding cap as code critics: round three
# refuses a critic-owned fourth round before creating its record or payload.
warden_brief="$agent_fixture/cap-warden.md"
make_agent_brief "$warden_brief" implement
run_agent_fixture cap-warden cap-warden "$agent_dispatch" dispatch \
  --role warden --brief "$warden_brief" --reviews review-target --job-id cap-warden --wait
warden_return="$agent_repo/artifacts/agents/cap-warden/rounds/1/return.json"
json_replace_field "$warden_return" findings \
  '[{"id":"WARDEN-CAP-1","severity":"high","material":true,"claim":"warden cap finding","evidence":"direct fixture evidence"}]'
json_replace_field "$warden_return" rigor \
  '[{"findingId":"WARDEN-CAP-1","rigorClass":"severe","facts":{"local":true,"recoverable":true,"proofBoundaryCrossed":false,"authorityBoundaryCrossed":false,"secretsBoundaryCrossed":false,"irreversibleDataBoundaryCrossed":false,"externalSideEffectBoundaryCrossed":false},"reopeningTrigger":"reopen if it recurs"}]'
run_agent_fixture cap-warden-round2 cap-warden-r2 "$agent_dispatch" follow-up \
  --job cap-warden --message "$follow_message" --wait
run_agent_fixture cap-warden-round3 cap-warden-r3 "$agent_dispatch" follow-up \
  --job cap-warden-r2 --message "$follow_message" --wait
set +e
run_agent_fixture_captured cap-warden-refusal cap-warden-r4 "$agent_fixture/cap-warden-refusal.out" \
  "$agent_dispatch" follow-up --job cap-warden-r3 --message "$follow_message" --wait
cap_warden_refusal_rc=$?
set -e
[[ "$cap_warden_refusal_rc" -ne 0 ]] \
  && grep -Fq 'warden critique budget exhausted' "$agent_fixture/cap-warden-refusal.out" \
  || { echo "the warden did not follow the code critic cap regime" >&2; cat "$agent_fixture/cap-warden-refusal.out" >&2; exit 1; }
[[ ! -e "$agent_repo/artifacts/agents/jobs/cap-warden-r4.json" \
   && ! -e "$agent_repo/artifacts/agents/cap-warden/rounds/4" ]] \
  || { echo "the warden cap refusal created a child record or payload" >&2; exit 1; }
# The same contention is exercised on a follow-up chain. Once the first
# wrapper publishes round two, the second wrapper must claim that same round;
# it must not turn a live round two into either a chain-lock refusal or round
# three.
repeat_follow_brief="$agent_fixture/repeat-follow-brief.md"
make_agent_brief "$repeat_follow_brief" design
run_agent_fixture repeat-follow-parent repeat-follow "$agent_dispatch" dispatch \
  --role design-critic --brief "$repeat_follow_brief" --job-id repeat-follow --wait
repeat_follow_release="$agent_fixture/repeat-follow-release"
repeat_follow_message="$agent_fixture/repeat-follow-message.md"
cp "$agent_repo/scripts/agents/templates/follow-up.md" "$repeat_follow_message"
printf '\nFAKE:custodial-critique=%s\n' "$repeat_follow_release" >>"$repeat_follow_message"
cap_lock_fixture_acquire repeat-follow
wait_for_agent_census_fresh repeat-follow-first
(cd "$agent_repo" && "$agent_dispatch" follow-up --job repeat-follow \
  --message "$repeat_follow_message") >"$agent_fixture/repeat-follow-first.out" 2>&1 &
repeat_follow_first_pid=$!
wait_for_chain_lock repeat-follow repeat-follow-first
(cd "$agent_repo" && "$agent_dispatch" follow-up --job repeat-follow \
  --message "$repeat_follow_message") >"$agent_fixture/repeat-follow-second.out" 2>&1 &
repeat_follow_second_pid=$!
sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
cap_lock_fixture_release
repeat_follow_first_rc=0
repeat_follow_second_rc=0
wait_for_agent_fixture_process repeat-follow-first repeat-follow-r2 "$repeat_follow_first_pid" \
  || repeat_follow_first_rc=$?
wait_for_agent_fixture_process repeat-follow-second repeat-follow-r2 "$repeat_follow_second_pid" \
  || repeat_follow_second_rc=$?
(( repeat_follow_first_rc == 0 )) \
  || { echo "the winning repeated follow-up wrapper failed with $repeat_follow_first_rc" >&2; cat "$agent_fixture/repeat-follow-first.out" >&2; exit 1; }
(( repeat_follow_second_rc == 0 || repeat_follow_second_rc == 3 )) \
  || { echo "the losing repeated follow-up wrapper failed before claim-launch with $repeat_follow_second_rc" >&2; cat "$agent_fixture/repeat-follow-second.out" >&2; exit 1; }
grep -Eq '"outcome":"(BOUND|IN-PROGRESS)"' "$agent_fixture/repeat-follow-second.out" \
  || { echo "the losing repeated follow-up wrapper did not return a claim state" >&2; cat "$agent_fixture/repeat-follow-second.out" >&2; exit 1; }
[[ -f "$agent_repo/artifacts/agents/jobs/repeat-follow-r2.json" \
   && ! -e "$agent_repo/artifacts/agents/jobs/repeat-follow-r3.json" ]] \
  || { echo "the repeated follow-up did not stay on its claimed round" >&2; exit 1; }
touch "$repeat_follow_release"
wait_for_agent_status repeat-follow-r2 completed

# A follow-up on a worktree chain whose trunk moved warns loudly: the
# stale-worktree lesson was violated three times as prose before this line.
stale_brief="$agent_fixture/stale-wt.md"
make_agent_brief "$stale_brief" implement
run_agent_fixture stale-wt stale-wt "$agent_dispatch" dispatch --role implementer --brief "$stale_brief" --job-id stale-wt --worktree --wait
printf 'advance\n' >>"$agent_repo/trunk-advance.txt"
git -C "$agent_repo" add trunk-advance.txt
# core.hooksPath=/dev/null: the goal open above ENROLLED the real
# pre-commit guard (R2-11), and this is a harness commit simulating
# trunk movement, not an agent's ledger commit — bypassing hooks
# here is the fixture's own act, said out loud.
git -C "$agent_repo" -c core.hooksPath=/dev/null -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm trunk-advance
"$agent_dispatch" follow-up --job stale-wt --message "$follow_message" --wait >"$tmp/stale-wt.out" 2>"$tmp/stale-wt.err" \
  || { echo "stale-worktree follow-up itself failed" >&2; cat "$tmp/stale-wt.err" >&2; exit 1; }
grep -q 'WORKTREE-BEHIND' "$tmp/stale-wt.err" \
  || { echo "follow-up did not warn about a worktree behind its trunk" >&2; exit 1; }

run_agent_fixture happy-follow-up happy-r2 "$agent_dispatch" follow-up --job happy --message "$follow_message" --wait
[[ -d "$agent_repo/artifacts/agents/happy/rounds/1" && -d "$agent_repo/artifacts/agents/happy/rounds/2" ]] \
  || { echo "follow-up did not preserve round 1 and create round 2" >&2; exit 1; }
happy_child="$agent_repo/artifacts/agents/jobs/happy-r2.json"
[[ "$("$engine" json get --file "$happy_child" --field parentJob)" == happy \
   && "$("$engine" json get --file "$happy_child" --field round)" == 2 \
   && "$("$engine" json get --file "$happy_child" --field sessionId)" \
      == "$("$engine" json get --file "$happy_record" --field sessionId)" ]] \
  || { echo "follow-up child does not chain to happy round 1" >&2; exit 1; }
happy_follow_session_key=$("$engine" json get --file "$happy_child" --field sessionKey)
[[ "$happy_follow_session_key" == "fake:$("$engine" json get --file "$happy_record" --field sessionId)" ]] \
  || { echo "follow-up did not record the resumed session occupancy key" >&2; exit 1; }
[[ "$("$engine" json get --file "$happy_child" --field fingerprintVersion)" == 1 \
   && -n "$("$engine" json get --file "$happy_child" --field fingerprint)" \
   && "$("$engine" json get --file "$happy_child" --field dispatchMode)" == follow-up \
   && "$("$engine" json get --file "$happy_child" --field resumedSessionId)" == "$("$engine" json get --file "$happy_record" --field sessionId)" \
   && "$("$engine" json get --file "$happy_child" --field launchMode)" == shared-checkout \
   && "$("$engine" json get --file "$happy_child" --field productRoots)" == "[\"$agent_repo\"]" \
   && "$("$engine" json get --file "$happy_child" --field productRootScopes)" \
      == "[{\"path\":\"$agent_repo\",\"reason\":\"shared-checkout\",\"standing\":\"attribution-only\"}]" ]] \
  || { echo "ordinary follow-up did not complete its fingerprinted claim-launch reservation" >&2; exit 1; }
happy_follow_session_digest=$(printf '%s' "$happy_follow_session_key" | "$engine" util sha256)
happy_follow_session_index="$agent_repo/artifacts/agents/sessions/$happy_follow_session_digest.json"
[[ -f "$happy_follow_session_index" \
   && "$("$engine" json get --file "$happy_follow_session_index" --field sessionKey)" == "$happy_follow_session_key" \
   && "$("$engine" json get --file "$happy_follow_session_index" --field occupants)" == '[]' ]] \
  || { echo "follow-up did not maintain and release the resumed session occupancy index" >&2; cat "$happy_follow_session_index" >&2 2>/dev/null || true; exit 1; }
happy_child_started=$("$engine" json get --file "$happy_child" --field startedAt)
happy_parent_started=$("$engine" json get --file "$happy_record" --field startedAt)
[[ ! "$happy_child_started" < "$happy_parent_started" ]] \
  || { echo "follow-up child started before its parent" >&2; exit 1; }
[[ "$("$engine" json get --file "$happy_child" --field capMin)" \
   == "$("$engine" json get --file "$happy_record" --field capMin)" ]] \
  || { echo "follow-up child changed the chain's cap" >&2; exit 1; }
happy_child_snapshot=$("$engine" json get --file "$happy_child" --field capabilitySnapshot)
[[ "$("$engine" json get --file "$happy_child" --field sessionEstablishedTimeoutSec)" \
   == "$("$engine" json get --file "$agent_repo/$happy_child_snapshot" --field capabilities.sessionEstablishedTimeoutSec)" ]] \
  || { echo "follow-up child does not carry its snapshot's session-established timeout" >&2; exit 1; }
[[ "$("$engine" json get --file "$happy_record" --field chainUsage.providerUnits.fake.fake-unit)" == 2 ]] \
  || { echo "chain usage did not aggregate two fake units" >&2; exit 1; }
# One chain, one mirror home; each stamp records ITS OWN mirror moment
# (chain-wide stamp equality was the lie KI-6 round 3 removed). The
# durable proof is the shared manifest covering BOTH records.
happy_mirror_home=$("$engine" json get --file "$happy_child" --field mirror.path)
[[ "$("$engine" json get --file "$happy_record" --field mirror.path)" == "$happy_mirror_home" ]] \
  || { echo "parent and child mirror to different homes" >&2; exit 1; }
[[ "$("$engine" util sha256 --file "$happy_mirror_home/manifest.json")" \
   == "$("$engine" json get --file "$happy_child" --field mirror.manifest)" ]] \
  || { echo "chain manifest digest does not match the child stamp" >&2; exit 1; }
happy_manifest_files=$("$engine" json get --file "$happy_mirror_home/manifest.json" --field files)
[[ "$happy_manifest_files" == *'"jobs/happy.json":'* && "$happy_manifest_files" == *'"jobs/happy-r2.json":'* ]] \
  || { echo "the shared manifest does not cover both chain records" >&2; exit 1; }
run_agent_fixture malformed-return-follow-up malformed-return-r2 "$agent_dispatch" follow-up --job malformed-return --message "$follow_message" --wait
[[ "$(cd "$agent_repo" && scripts/agents/dispatch.sh status --job malformed-return-r2)" == completed ]] \
  || { echo "protocol-error retry did not create a completed child" >&2; exit 1; }
malformed_follow_prompt="$agent_repo/artifacts/agents/malformed-return/rounds/2/prompt.md"
grep -Fq '# Canonical critique register carry' "$malformed_follow_prompt" \
  && grep -Fq -- '- synthetic-' "$malformed_follow_prompt" \
  || { echo "the corrected protocol-return follow-up did not carry its synthetic finding identifier" >&2; cat "$malformed_follow_prompt" >&2; exit 1; }
agent_fails pending-follow-up 'pending, running, timeout, or process-lost' "$agent_dispatch" follow-up --job cancelled --message "$follow_message"
agent_fails timeout-follow-up 'pending, running, timeout, or process-lost' "$agent_dispatch" follow-up --job timed --message "$follow_message"
agent_fails process-loss-follow-up 'pending, running, timeout, or process-lost' "$agent_dispatch" follow-up --job process-loss --message "$follow_message"

json_replace_field "$agent_repo/artifacts/agents/jobs/default-role.json" sessionId null
agent_fails null-session-follow-up 'fresh-context embed fallback' "$agent_dispatch" follow-up --job default-role --message "$follow_message"

resume_root="$agent_fixture/resume-root.md"
make_agent_brief "$resume_root" design
run_agent_fixture resume-root resume-root "$agent_dispatch" dispatch --role design-critic --brief "$resume_root" --job-id resume-root --wait
resume_collision="$agent_fixture/resume-collision.md"
cp "$follow_message" "$resume_collision"
printf '\nFAKE:resume-collision\n' >>"$resume_collision"
set +e
run_agent_fixture resume-collision resume-root-r2 "$agent_dispatch" follow-up --job resume-root --message "$resume_collision" --wait
resume_status=$?
set -e
[[ $resume_status -eq 3 ]] || { echo "resume collision did not map to failed" >&2; exit 1; }
grep -Fq 'resume_collision' "$agent_repo/artifacts/agents/jobs/resume-root-r2.json" \
  || { echo "resume collision did not retain its named error" >&2; exit 1; }

active_brief="$agent_fixture/active.md"
make_agent_brief "$active_brief" design 'FAKE:concurrent-turn'
run_agent_fixture_captured active-turn-dispatch active-turn /dev/null "$agent_dispatch" dispatch --role design-critic --brief "$active_brief" --job-id active-turn
agent_fails active-follow-up 'pending, running, timeout, or process-lost' "$agent_dispatch" follow-up --job active-turn --message "$follow_message"
run_agent_fixture active-turn-cancel active-turn "$agent_dispatch" cancel --job active-turn

run_agent_fixture happy-close happy "$agent_dispatch" close --job happy
[[ "$("$engine" json get --file "$agent_repo/artifacts/agents/jobs/happy.json" --field runnerClosed)" == false ]] \
  || { echo "host-closed chain was stamped as runner-closed" >&2; exit 1; }
agent_fails closed-follow-up 'job chain is closed' "$agent_dispatch" follow-up --job happy --message "$follow_message"

# A close racing a follow-up cannot land between its open check and child creation.
run_agent_fixture close-race close-race "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id close-race --wait
close_rc="$agent_fixture/close-race.close"; follow_rc="$agent_fixture/close-race.follow"
wait_for_agent_census_fresh close-race-follow
(set +e; cd "$agent_repo"; scripts/agents/dispatch.sh close --job close-race >/dev/null 2>&1; printf '%s\n' "$?" >"$close_rc") & close_pid=$!
(set +e; cd "$agent_repo"; scripts/agents/dispatch.sh follow-up --job close-race --message "$follow_message" >/dev/null 2>&1; printf '%s\n' "$?" >"$follow_rc") & follow_pid=$!
wait_for_agent_fixture_process close-race-close close-race "$close_pid"
wait_for_agent_fixture_process close-race-follow close-race-r2 "$follow_pid"
close_won=$(cat "$close_rc"); follow_won=$(cat "$follow_rc")
[[ ( "$close_won" == 0 && "$follow_won" != 0 ) || ( "$close_won" != 0 && "$follow_won" == 0 ) ]] \
  || { echo "close/follow-up race did not serialize to one winner" >&2; exit 1; }

# Conformance uses the actual intent-to-add working-tree diff and protects
# both plans/ and the ignored agent control plane.
implement_brief="$agent_fixture/implement.md"
make_agent_brief "$implement_brief" implement
run_agent_fixture conformance conformance "$agent_dispatch" dispatch --role implementer --brief "$implement_brief" --job-id conformance --worktree --wait
conformance_record="$agent_repo/artifacts/agents/jobs/conformance.json"
conformance_workspace=$("$engine" json get --file "$conformance_record" --field workspaceRoot)
[[ "$("$engine" json get --file "$conformance_record" --field launchMode)" == worktree \
   && "$("$engine" json get --file "$conformance_record" --field productRoots)" == "[\"$conformance_workspace\"]" ]] \
  || { echo "ordinary worktree dispatch did not default its product root to workspaceRoot" >&2; exit 1; }
# D8: an unreviewed completed implementer does NOT wedge the close — the
# diff exists only once the host's conformance review runs, and the
# workflow gap is the delegation floor's verdict, not the close's.
run_agent_fixture close-before-diff conformance "$agent_dispatch" close --job conformance
(cd "$agent_repo" && scripts/agents/assert-conformance.sh --stage review --job conformance)
[[ -f "$agent_repo/artifacts/agents/conformance/rounds/1/diff.patch" ]] \
  || { echo "conformance did not persist diff.patch" >&2; exit 1; }
run_agent_fixture conformance-reap conformance "$agent_dispatch" reap --job conformance
run_agent_fixture conformance-close conformance "$agent_dispatch" close --job conformance
# ...but a diff the MANIFEST knows and the disk lost is evidence loss:
# a re-close over the vanished file refuses by name (D8's kept tooth).
mv "$agent_repo/artifacts/agents/conformance/rounds/1/diff.patch" "$agent_fixture/diff.patch.save"
agent_fails diff-vanished 'vanished after mirroring' "$agent_dispatch" close --job conformance
mv "$agent_fixture/diff.patch.save" "$agent_repo/artifacts/agents/conformance/rounds/1/diff.patch"
conformance_workspace=$("$engine" json get --file "$agent_repo/artifacts/agents/jobs/conformance.json" --field workspaceRoot)
case "${conformance_workspace%/}/" in "${agent_repo%/}/"*) ;; *) echo "job worktree is outside the watcher scope" >&2; exit 1 ;; esac
printf 'untracked change\n' >"$conformance_workspace/source.txt"
agent_fails diff-boundary-mismatch 'changed paths fall outside the cumulative implementation boundary' "$agent_repo/scripts/agents/assert-conformance.sh" --stage review --job conformance
json_replace_field "$agent_repo/artifacts/agents/conformance/rounds/1/return.json" \
  diffBoundary '["source.txt"]'
(cd "$agent_repo" && scripts/agents/assert-conformance.sh --stage review --job conformance)
mkdir -p "$conformance_workspace/plans"
printf 'delegate plan\n' >"$conformance_workspace/plans/delegate.md"
json_replace_field "$agent_repo/artifacts/agents/conformance/rounds/1/return.json" \
  diffBoundary '["source.txt","plans/delegate.md"]'
agent_fails untracked-plan 'trusted plans/ state changed' "$agent_repo/scripts/agents/assert-conformance.sh" --stage review --job conformance
git -C "$conformance_workspace" add source.txt plans/delegate.md
# The workspace is a WORKTREE of the enrolled repo (shared hooks);
# this harness checkpoint bypasses them for the same stated reason
# as trunk-advance.
git -C "$conformance_workspace" -c core.hooksPath=/dev/null -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm delegate-checkpoint
agent_fails committed-plan 'trusted plans/ state changed' "$agent_repo/scripts/agents/assert-conformance.sh" --stage review --job conformance
printf 'uncommitted change\n' >>"$conformance_workspace/plans/delegate.md"
agent_fails uncommitted-plan 'trusted plans/ state changed' "$agent_repo/scripts/agents/assert-conformance.sh" --stage review --job conformance
mkdir -p "$conformance_workspace/artifacts/agents"
printf 'tamper\n' >"$conformance_workspace/artifacts/agents/tamper"
agent_fails control-plane-change 'agent control plane contains delegate-created files' "$agent_repo/scripts/agents/assert-conformance.sh" --stage review --job conformance

# Snapshot self-heal, fallbacks, permission waivers, and raw/event
# degradations. A snapshot miss costs ONE adapter probe, not a husked
# dispatch: a CLI that rewrites its own config mid-run (KI-19's class)
# moved the identity hash and stranded two dispatches in rep 1 of
# bm-1-20260813t132947z. The refusal still stands when the probe itself
# cannot heal the miss.
snapshot_dir="$agent_repo/artifacts/agents/capabilities"
snapshot_save="$agent_fixture/snapshots"
mkdir -p "$snapshot_save"
mv "$snapshot_dir"/*.json "$snapshot_save/"
run_agent_fixture no-snapshot-selfheal no-snapshot "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id no-snapshot --wait
[[ -n "$(ls "$snapshot_dir"/*.json 2>/dev/null)" ]] \
  || { echo "snapshot self-heal did not re-probe" >&2; exit 1; }
rm -f "$snapshot_dir"/*.json
"$fake_adapter" probe --age-days 31 >/dev/null
run_agent_fixture stale-snapshot-selfheal stale-snapshot "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id stale-snapshot --wait
# An unhealable miss refuses: break the adapter's probe verb via its
# fault hook, then dispatch with no matching snapshot.
rm -f "$snapshot_dir"/*.json
METASYSTEM_FAKE_PROBE_FAIL=1 "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id unhealable-snapshot >"$agent_fixture/unhealable-snapshot.out" 2>&1 \
  && { echo "an unhealable snapshot miss must refuse the dispatch" >&2; exit 1; }
grep -q 'adapter probe failed' "$agent_fixture/unhealable-snapshot.out" \
  || { echo "the unhealable refusal did not name the failed probe" >&2; cat "$agent_fixture/unhealable-snapshot.out" >&2; exit 1; }
mv "$snapshot_dir"/*.json "$agent_fixture/" 2>/dev/null || true
mv "$snapshot_save"/*.json "$snapshot_dir/"

old_save="$agent_fixture/current-snapshots"
mkdir -p "$old_save"
mv "$snapshot_dir"/*.json "$old_save/"
"$fake_adapter" probe --profile old >/dev/null
old_brief="$agent_fixture/old-capabilities.md"
make_agent_brief "$old_brief" implement 'FAKE:old-capability-set' 'FAKE:no-event-stream' 'FAKE:hook-unavailable'
run_agent_fixture old-capabilities old-capabilities "$agent_dispatch" dispatch --role implementer --brief "$old_brief" --job-id old-capabilities --worktree --wait
old_caps_record="$agent_repo/artifacts/agents/jobs/old-capabilities.json"
[[ "$("$engine" json get --file "$old_caps_record" --field sessionEstablishedSignal)" == false ]] \
  || { echo "old-capability dispatch claimed a session-established signal" >&2; exit 1; }
old_caps_resume_fallback=
while IFS= read -r old_caps_item; do
  [[ -n "$old_caps_item" ]] || continue
  [[ "$("$engine" json get --value "$old_caps_item" --field capability)" == resume ]] || continue
  old_caps_resume_fallback=1
done < <(json_elements "$("$engine" json get --file "$old_caps_record" --field capabilityFallbacks)")
[[ -n "$old_caps_resume_fallback" ]] \
  || { echo "old-capability dispatch did not record the resume fallback" >&2; exit 1; }
[[ ! -e "$agent_repo/artifacts/agents/old-capabilities/rounds/1/events.jsonl" ]] \
  || { echo "no-event-stream fallback emitted a native event file" >&2; exit 1; }
grep -Fq 'polling fallback used' "$agent_repo/artifacts/agents/jobs/old-capabilities.log" \
  || { echo "hook-unavailable fallback was not observable" >&2; exit 1; }
run_agent_fixture old-capabilities-follow-up old-capabilities-r2 "$agent_dispatch" follow-up --job old-capabilities --message "$follow_message" --wait
old_caps_child="$agent_repo/artifacts/agents/jobs/old-capabilities-r2.json"
[[ "$("$engine" json get --file "$old_caps_child" --field resumeMode)" == fresh-context ]] \
  || { echo "no-resume follow-up did not fall back to fresh context" >&2; exit 1; }
[[ "$("$engine" json get --file "$old_caps_child" --field sessionId)" \
   != "$("$engine" json get --file "$old_caps_record" --field sessionId)" ]] \
  || { echo "fresh-context follow-up reused the parent session" >&2; exit 1; }
old_caps_prompt="$agent_repo/artifacts/agents/old-capabilities/rounds/2/prompt.md"
grep -Fq '# Prior brief' "$old_caps_prompt" \
  && grep -Fq '# Prior return' "$old_caps_prompt" \
  && grep -Fq '# Correction' "$old_caps_prompt" \
  || { echo "fresh-context prompt lost an embed section" >&2; exit 1; }
mv "$snapshot_dir"/*.json "$agent_fixture/"
mv "$old_save"/*.json "$snapshot_dir/"

requirements="$agent_repo/scripts/agents/roles/design-critic.requirements.json"
saved_requirements="$agent_fixture/design-critic.requirements.json"
cp "$requirements" "$saved_requirements"
# The unverified-network snapshot must be the ONLY candidate: selection
# is newest-by-capturedAt (registry-3 fixed the filename-order rule), and
# same-second probes from earlier sections could still tie with the
# doctored snapshot — isolation keeps this row independent of how many
# probes ran before it.
mkdir -p "$agent_fixture/pre-unverified"
mv "$snapshot_dir"/*.json "$agent_fixture/pre-unverified/" 2>/dev/null || true
"$fake_adapter" probe --profile unverified-network >/dev/null
# This refusal is about a runtime that cannot verify a field the envelope
# restricts, so the envelope has to restrict it. Since the presets now grant
# network, the request is made restrictive explicitly rather than by default.
agent_fails unverified-deny 'cannot enforce restrictive permission field network' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --permissions "$restrictive_permissions" --job-id unverified-deny
# Merge the network waiver into whatever waivers the role already declares
# (the shipped roles now waive readRoots/writeRoots for devin), rather than
# string-injecting a second waivers key and producing invalid JSON.
if requirements_network=$("$engine" json get --file "$requirements" --field waivers.network 2>/dev/null); then
  if [[ "$requirements_network" == '[]' ]]; then
    requirements_network_new='["fake-network-unverified"]'
  else
    requirements_network_new="${requirements_network%]},\"fake-network-unverified\"]"
  fi
  requirements_waivers=$("$engine" json get --file "$requirements" --field waivers)
  json_replace_field "$requirements" waivers \
    "${requirements_waivers/"\"network\":$requirements_network"/"\"network\":$requirements_network_new"}"
elif requirements_waivers=$("$engine" json get --file "$requirements" --field waivers 2>/dev/null); then
  if [[ "$requirements_waivers" == '{}' ]]; then
    json_replace_field "$requirements" waivers '{"network":["fake-network-unverified"]}'
  else
    json_replace_field "$requirements" waivers \
      "${requirements_waivers%\}},\"network\":[\"fake-network-unverified\"]}"
  fi
else
  echo "design-critic requirements carry no waivers object to merge into" >&2; exit 1
fi
run_agent_fixture waived-deny waived-deny "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --permissions "$restrictive_permissions" --job-id waived-deny --wait
cp "$saved_requirements" "$requirements"
mv "$agent_fixture/pre-unverified"/*.json "$snapshot_dir/" 2>/dev/null || true
"$fake_adapter" probe >/dev/null

# An EMPTY write scope is still restrictive on a runtime whose write boundary
# is notEnforced (it can write through a shell), so such a role is refused
# without a recorded writeRoots waiver and runs with one. Exercises the real
# selector against a laid-down snapshot -- no live CLI, no dispatch.
ew_caps="$agent_repo/artifacts/agents/capabilities"
mkdir -p "$ew_caps"
ew_snap="$ew_caps/ghostrt-1.0-20990101-001.json"
printf '%s\n' '{"runtime":"ghostrt","cliVersion":"1.0","capturedAt":"2099-01-01T00:00:00Z","configHash":"cfg1","configKeyHashes":{},"sequence":1,"transports":[],"capabilities":{"sessionEstablishedTimeoutSec":10},"permissions":{"unverified":["readRoots","writeRoots","network"]},"envelopeEnforcement":{"writeRoots":"notEnforced","readRoots":"notEnforced","network":"notEnforced"}}' >"$ew_snap"
ew_identity='{"runtime":"ghostrt","cliVersion":"1.0","configHash":"cfg1","configKeyHashes":{}}'
ew_env="$agent_fixture/empty-write-envelope.json"
printf '%s\n' '{"readRoots":[],"writeRoots":[],"network":"allow","approvals":"deny","tools":"read-only"}' >"$ew_env"
ew_role="$agent_repo/scripts/agents/roles/design-critic.requirements.json"
ew_role_saved="$agent_fixture/ew-role-saved.json"
cp "$ew_role" "$ew_role_saved"
printf '%s\n' '{"required":[],"optional":{},"waivers":{}}' >"$ew_role"
if bin/metasystem job snapshot-select --root "$agent_repo" --runtime ghostrt \
    --role design-critic --identity "$ew_identity" --max-age 40000 --envelope "$ew_env" \
    --output "$agent_fixture/ew-unwaived.out" 2>"$agent_fixture/ew-unwaived.err"; then
  cp "$ew_role_saved" "$ew_role"
  echo "empty writeRoots on a notEnforced runtime ran without a waiver (the bypass is open)" >&2; exit 1
fi
grep -Fq 'writeRoots' "$agent_fixture/ew-unwaived.err" \
  || { cp "$ew_role_saved" "$ew_role"; echo "empty-writeRoots refusal did not name the field" >&2; cat "$agent_fixture/ew-unwaived.err" >&2; exit 1; }
# An UNREGISTERED runtime fails closed even with a name waiver on
# record (agnosticism audit class 9): no declared residual means no
# waiver can apply — a future under-enforced runtime is refused until
# its registry declaration AND a human role-policy edit both exist.
printf '%s\n' '{"required":[],"optional":{},"waivers":{"writeRoots":["ghostrt"]}}' >"$ew_role"
if bin/metasystem job snapshot-select --root "$agent_repo" --runtime ghostrt \
    --role design-critic --identity "$ew_identity" --max-age 40000 --envelope "$ew_env" \
    --output "$agent_fixture/ew-waived.out" 2>"$agent_fixture/ew-waived.err"; then
  cp "$ew_role_saved" "$ew_role"
  echo "an unregistered runtime's name waiver bypassed the residual rule" >&2; exit 1
fi
grep -Fq 'declares no residual' "$agent_fixture/ew-waived.err" \
  || { cp "$ew_role_saved" "$ew_role"; echo "the fail-closed refusal did not name the missing residual" >&2; cat "$agent_fixture/ew-waived.err" >&2; exit 1; }
cp "$ew_role_saved" "$ew_role"
rm -f "$ew_snap"

nested_brief="$agent_fixture/nested.md"
make_agent_brief "$nested_brief" design 'FAKE:nested-agent-events'
run_agent_fixture nested-events nested-events "$agent_dispatch" dispatch --role design-critic --brief "$nested_brief" --job-id nested-events --wait
grep -Fq '"topLevel":false' "$agent_repo/artifacts/agents/nested-events/rounds/1/events.jsonl" \
  || { echo "nested-agent event was not captured" >&2; exit 1; }
[[ "$(cd "$agent_repo" && scripts/agents/dispatch.sh status --job nested-events)" == completed ]] \
  || { echo "nested completion event ended the wrong lifecycle" >&2; exit 1; }
malicious_brief="$agent_fixture/malicious.md"
make_agent_brief "$malicious_brief" design 'Fake-Argument: $(touch should-not-exist)'
run_agent_fixture malicious-argument malicious-argument "$agent_dispatch" dispatch --role design-critic --brief "$malicious_brief" --job-id malicious-argument --wait
[[ ! -e "$agent_repo/should-not-exist" ]] || { echo "malicious provider argument was evaluated" >&2; exit 1; }
grep -Fq '$(touch should-not-exist)' "$agent_repo/artifacts/agents/malicious-argument/rounds/1/raw.out" \
  || { echo "malicious provider argument was not transported verbatim as a value" >&2; exit 1; }

# The no-tier guard fixtures use a fresh supervisor fingerprint after
# changing the roster. The runtime-override roles return to their shipped
# main assignment; fake remains the only registered fixture adapter.
run_fixture_arm "dispatcher shutdown before no-tier re-arm" - \
  "$agent_repo/scripts/agents/arm-supervision.sh" --repo "$agent_repo" --shutdown
cp "$no_tier_conf" "$agent_repo/metasystem.conf"
conf_edit "$agent_repo/metasystem.conf" awk '
  /^role[.](code-critic|investigator)[.]runtime=fake$/ {
    role = $0
    sub(/^role[.]/, "", role)
    sub(/[.]runtime=fake$/, "", role)
    $0 = "role." role ".runtime=main"
  }
  {
    printf "%s%s", $0, \
      (FNR < conf_edit_line_count || conf_edit_final_terminated ? ORS : "")
  }
'
# The template now ships code-critic entries (C-2), so the conf snapshot
# already carries role.code-critic.model.fake from the roster rewrite;
# appending it again was a duplicate-key failure.
cat >>"$agent_repo/metasystem.conf" <<'EOF'
role.investigator.model.fake=fake-implied-model
EOF
run_fixture_arm "dispatcher no-tier re-arm" "$agent_fixture/no-tier-arming.out" \
  env METASYSTEM_AGENT_RUNTIME=fake "$agent_repo/scripts/agents/arm-supervision.sh" \
    --repo "$agent_repo" --session validator-no-tiers --pid "$$" \
    --start-time "$agent_main_start" --tag metasystem-main-fake-validator

agent_fails no-tier-model-override 'Configure model.tier.* to rank both pairs' \
  "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --model fake-escalated --job-id no-tier-model
agent_fails escalation-non-tty 'requires an interactive TTY' \
  "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --model fake-escalated --approve-escalation --job-id escalation-non-tty
run_tty_agent_fixture escalation-declined NO 1 \
  "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --model fake-escalated --approve-escalation --job-id escalation-declined
grep -Fq 'escalation approval declined' "$agent_fixture/escalation-declined.out" \
  || { echo "declined escalation fixture did not name the corrective action" >&2; exit 1; }
[[ ! -e "$agent_repo/artifacts/agents/jobs/escalation-declined.json" ]] \
  || { echo "declined escalation fixture created a job" >&2; exit 1; }
run_tty_agent_fixture escalation-approved 'APPROVE Fixture Human' 0 \
  "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --model fake-escalated --approve-escalation --job-id escalation-approved --wait
escalation_record="$agent_repo/artifacts/agents/jobs/escalation-approved.json"
escalation_display="$agent_fixture/escalation-approved.out"
[[ "$("$engine" json get --file "$escalation_record" --field escalationApproval.name)" == 'Fixture Human' ]] \
  || { echo "escalation approval lost the approver's name" >&2; exit 1; }
escalation_approved_at=$("$engine" json get --file "$escalation_record" --field escalationApproval.approvedAt)
[[ "$escalation_approved_at" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$ ]] \
  || { echo "escalation approval stamp changed shape: $escalation_approved_at" >&2; exit 1; }
escalation_roster=$("$engine" json get --file "$escalation_record" --field escalationApproval.rosterResolution)
[[ "$escalation_roster" == 'fake:fake-model' ]] \
  || { echo "escalation approval roster resolution is wrong: $escalation_roster" >&2; exit 1; }
escalation_requested=$("$engine" json get --file "$escalation_record" --field escalationApproval.requestedPair)
[[ "$escalation_requested" == 'fake:fake-escalated' ]] \
  || { echo "escalation approval requested pair is wrong: $escalation_requested" >&2; exit 1; }
escalation_direction=$("$engine" json get --file "$escalation_record" --field escalationApproval.costDirection)
[[ "$escalation_direction" == 'unranked (model tiers absent; overrides always escalate)' ]] \
  || { echo "escalation approval cost direction is wrong: $escalation_direction" >&2; exit 1; }
grep -Fq "Roster resolution: $escalation_roster" "$escalation_display" \
  && grep -Fq "Requested pair: $escalation_requested" "$escalation_display" \
  && grep -Fq "Cost direction: $escalation_direction" "$escalation_display" \
  || { echo "the escalation prompt did not display the recorded approval facts" >&2; exit 1; }

# Dispatch only reads mission leases. The future mission runner owns their
# acquisition and renewal; this fixture fabricates the frozen shape around
# a process whose command line carries the instance tag.
mkdir -p "$agent_repo/plans" "$agent_repo/artifacts/agents/missions/mission-alpha"
cat >"$agent_repo/plans/mission-mission-alpha.contract.md" <<'EOF'
```mission
fence.wall-clock-hours=2
fence.cycles=10
fence.jobs=20
fence.concurrency=2
fence.job-cap-min=FIXTURE_DISPATCH_CAP_MIN
envelope.dispatch-allow=fake:fake-escalated,fake:fake-model
```

```mission-seal
sealed.version=1
```
EOF
conf_edit "$agent_repo/plans/mission-mission-alpha.contract.md" replace-literal \
  FIXTURE_DISPATCH_CAP_MIN "$fixture_dispatch_envelope_cap_min"
# The digest a human approval signs: Approval lines removed, every line
# stripped of trailing spaces/tabs, trailing blank lines dropped. This is
# contractCanonicalSignedBytes (internal/mission/contract.go), verified
# byte-identical to the retired python contract_hash before its deletion.
fixture_contract_hash() { # contract path — the ENGINE's canonical hash
  "$agent_repo/bin/metasystem" mission contract-hash --file "$1"
}
printf '\nApproval: name=Fixture-Human; date=2026-08-06; contract-sha256=%s\n' \
  "$(fixture_contract_hash "$agent_repo/plans/mission-mission-alpha.contract.md")" \
  >>"$agent_repo/plans/mission-mission-alpha.contract.md"
stamp_fixture_contract() { # mission — seed the runner-owned contract pin
  # (Codex's caps ruling: fixtures below the runner lifecycle seed
  # approvedContractSha256 as the digest of the exact raw contract bytes.)
  local mission=$1 contract="$agent_repo/plans/mission-$1.contract.md"
  local fences="$agent_repo/artifacts/agents/missions/$1/fences.json" contract_sha staged
  contract_sha=$("$engine" util sha256 --file "$contract")
  if [[ -f "$fences" ]]; then
    "$engine" json set --file "$fences" --field "approvedContractSha256=$contract_sha"
  else
    mkdir -p "$(dirname "$fences")"
    staged=$(mktemp "$(dirname "$fences")/.fences.XXXXXX")
    printf '{"schemaVersion":1,"missionId":"%s","startedAt":"%s","cycles":0,"reservations":{},"approvedContractSha256":"%s"}\n' \
      "$mission" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$contract_sha" >"$staged"
    mv "$staged" "$fences"
  fi
}
stamp_fixture_contract mission-alpha
dispatch_origin="$agent_fixture/dispatch-origin.git"
git init -q -b main --bare "$dispatch_origin"
git -C "$agent_repo" remote add origin "$dispatch_origin"
git -C "$agent_repo" add metasystem.conf plans/mission-mission-alpha.contract.md
git -C "$agent_repo" -c core.hooksPath=/dev/null -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm 'sign dispatch envelope fixture'
git -C "$agent_repo" push -qu origin main
git -C "$dispatch_origin" symbolic-ref HEAD refs/heads/main
git -C "$agent_repo" remote set-head origin -a >/dev/null
# The lease holder has no private lifetime: its bounded teardown below is the
# only event that ends it, so section growth cannot turn its lifetime into a cap.
"$agent_repo/bin/metasystem" util hold --tag mission-lease-tag & mission_pid=$!
mission_process=$(METASYSTEM_FAKE_AGENT_ANCESTOR_PID="$mission_pid" \
  "$agent_repo/bin/metasystem" proc find-ancestor --repo "$agent_repo" \
    --pid "$mission_pid" --runtime fake)
mission_pgid=$("$engine" json get --value "$mission_process" --field pgid)
mission_identity="$agent_fixture/mission-process-identity.json"
mission_lease_now=$(date -u +%Y-%m-%dT%H:%M:%SZ)
mission_lease_staged=$(mktemp "$agent_repo/artifacts/agents/missions/mission-alpha/.lease.XXXXXX")
printf '{"missionId":"mission-alpha","pid":%s,"pgid":%s,"instanceTag":"mission-lease-tag","startedAt":"%s","renewedAt":"%s"}\n' \
  "$mission_pid" "$mission_pgid" "$mission_lease_now" "$mission_lease_now" >"$mission_lease_staged"
mv "$mission_lease_staged" "$agent_repo/artifacts/agents/missions/mission-alpha/lease.json"
printf '{"%s":{"pgid":%s,"command":"metasystem util hold --tag mission-lease-tag"}}\n' \
  "$mission_pid" "$mission_pgid" >"$mission_identity"
export METASYSTEM_FAKE_PROCESS_IDENTITY_FILE="$mission_identity"
run_agent_fixture envelope-model-override envelope-model-override env METASYSTEM_MISSION_TURN=mission-alpha-t1-fixture "$agent_dispatch" dispatch \
  --role design-critic --brief "$happy_brief" --model fake-escalated --job-id envelope-model-override --mission mission-alpha --stream main --wait
run_agent_fixture envelope-runtime-override envelope-runtime-override env METASYSTEM_MISSION_TURN=mission-alpha-t1-fixture "$agent_dispatch" dispatch \
  --role code-critic --brief "$code_brief" --reviews review-target --runtime fake --job-id envelope-runtime-override --mission mission-alpha --stream main --wait
agent_fails envelope-runtime-implied-model 'add fake:fake-implied-model to a signed envelope.dispatch-allow' \
  env METASYSTEM_MISSION_TURN=mission-alpha-t1-fixture "$agent_dispatch" dispatch --role investigator --brief "$investigator_brief" --runtime fake --job-id envelope-runtime-implied --mission mission-alpha --stream main
run_agent_fixture mission-explicit mission-explicit env METASYSTEM_MISSION_TURN=mission-alpha-t1-fixture "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id mission-explicit --mission mission-alpha --stream main --wait
METASYSTEM_MISSION_ID=mission-alpha METASYSTEM_MISSION_LEASE="$agent_repo/artifacts/agents/missions/mission-alpha/lease.json" \
  METASYSTEM_MISSION_TURN=mission-alpha-t1-fixture \
  run_agent_fixture mission-inherited mission-inherited "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id mission-inherited --stream main --wait
# The caps implementation refuses over-cap mission dispatches with the
# sharper pair-cap message (names both numbers) instead of the generic
# lifecycle-fence wrapper; the expectation follows the sharper contract.
agent_fails mission-cap 'above signed fence.job-cap-min' env METASYSTEM_MISSION_TURN=mission-alpha-t1-fixture "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id mission-cap --mission mission-alpha --stream main --cap-min "$fixture_dispatch_over_envelope_cap_min"
# Under the caps contract an over-cap request is an AUTHORIZATION
# refusal — a synchronous host error, deliberately without a
# fence-bound ask (Codex's delegate-caps fixtures assert exactly
# this); asks remain the currency of genuine fence violations,
# proven by the fence-* fixtures below and the timeout-ask above.
[[ ! -f "$agent_repo/artifacts/agents/missions/mission-alpha/asks/fence-bound.json" ]] \
  || { echo "an authorization refusal wrote a fence-bound ask; that is the fence-violation channel" >&2; exit 1; }
mission_alpha_usage="$agent_repo/artifacts/agents/missions/mission-alpha/usage.json"
mission_alpha_fake_units=
while IFS= read -r mission_alpha_item; do
  [[ -n "$mission_alpha_item" ]] || continue
  [[ "$("$engine" json get --value "$mission_alpha_item" --field provider)" == fake ]] || continue
  [[ "$("$engine" json get --value "$mission_alpha_item" --field unit)" == provider.fake-unit ]] || continue
  mission_alpha_fake_units=$("$engine" json get --value "$mission_alpha_item" --field value)
done < <(json_elements "$("$engine" json get --file "$mission_alpha_usage" --field units)")
[[ "$mission_alpha_fake_units" == 4 ]] \
  || { echo "mission-alpha fake unit total is ${mission_alpha_fake_units:-missing}, want 4" >&2; exit 1; }
for mission_alpha_job in mission-explicit mission-inherited; do
  grep -Fxq 'Mission: mission-alpha' "$agent_repo/artifacts/agents/$mission_alpha_job/rounds/1/prompt.md" \
    || { echo "$mission_alpha_job prompt lost its mission line" >&2; exit 1; }
done
[[ "$("$engine" json get --file "$agent_repo/artifacts/agents/jobs/mission-inherited.json" --field turnId)" == mission-alpha-t1-fixture ]] \
  || { echo "inherited dispatch did not record the mission turn" >&2; exit 1; }

make_fence_mission() { # mission id, cycles, jobs, concurrency, wall hours
  local mission=$1 cycles=$2 jobs_limit=$3 concurrency=$4 wall=$5 mission_dir="$agent_repo/artifacts/agents/missions/$1"
  mkdir -p "$mission_dir" "$agent_repo/plans"
  cat >"$agent_repo/plans/mission-$mission.contract.md" <<EOF
\`\`\`mission
fence.wall-clock-hours=$wall
fence.cycles=$cycles
fence.jobs=$jobs_limit
fence.concurrency=$concurrency
fence.job-cap-min=$fixture_dispatch_envelope_cap_min
\`\`\`
EOF
  local fence_now fence_staged
  fence_now=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  fence_staged=$(mktemp "$mission_dir/.lease.XXXXXX")
  printf '{"missionId":"%s","pid":%s,"pgid":%s,"instanceTag":"mission-lease-tag","startedAt":"%s","renewedAt":"%s"}\n' \
    "$mission" "$mission_pid" "$mission_pgid" "$fence_now" "$fence_now" >"$fence_staged"
  mv "$fence_staged" "$mission_dir/lease.json"
}

assert_fence_ask() { # mission, expected reason
  local mission=$1 reason=$2 ask="$agent_repo/artifacts/agents/missions/$1/asks/fence-bound.json"
  # The batched ask lands a beat after the refusing path returns; the
  # contract is "the ask is batched", not "batched before the caller's
  # next syscall". Bounded wait, then the same loud failure.
  local waited=0
  while [[ ! -f "$ask" ]] && (( waited < 50 )); do sleep 0.2; waited=$((waited + 1)); done
  [[ -f "$ask" ]] || { echo "mission fence $reason refusal wrote no batched ask" >&2; exit 1; }
  grep -Fq "\`$reason\`" "$ask" || { echo "mission fence ask omitted $reason" >&2; exit 1; }
}

make_fence_mission mission-wall 10 10 2 1
printf '{"schemaVersion":1,"missionId":"mission-wall","startedAt":"2000-01-01T00:00:00Z","cycles":0,"reservations":{}}\n' \
  >"$agent_repo/artifacts/agents/missions/mission-wall/fences.json"
stamp_fixture_contract mission-wall
agent_fails fence-wall 'mission fence refused job (wall-clock-hours)' env METASYSTEM_MISSION_TURN=mission-wall-t1-fixture "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id fence-wall --mission mission-wall --stream main --wait
assert_fence_ask mission-wall wall-clock-hours

make_fence_mission mission-cycles 1 10 2 2
printf '{"schemaVersion":1,"missionId":"mission-cycles","startedAt":"%s","cycles":1,"reservations":{}}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  >"$agent_repo/artifacts/agents/missions/mission-cycles/fences.json"
stamp_fixture_contract mission-cycles
agent_fails fence-cycles 'mission fence refused job (cycles)' env METASYSTEM_MISSION_TURN=mission-cycles-t1-fixture "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id fence-cycles --mission mission-cycles --stream main --wait
assert_fence_ask mission-cycles cycles

make_fence_mission mission-jobs 10 1 2 2
printf '{"schemaVersion":1,"missionId":"mission-jobs","startedAt":"%s","cycles":0,"reservations":{"prior":{"reservedAt":"2000-01-01T00:00:00Z","capMin":%s}}}\n' \
  "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$fixture_minimum_cap_min" \
  >"$agent_repo/artifacts/agents/missions/mission-jobs/fences.json"
stamp_fixture_contract mission-jobs
agent_fails fence-jobs 'mission fence refused job (jobs)' env METASYSTEM_MISSION_TURN=mission-jobs-t1-fixture "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id fence-jobs --mission mission-jobs --stream main --wait
assert_fence_ask mission-jobs jobs

make_fence_mission mission-concurrency 10 10 1 2
printf '{"schemaVersion":1,"missionId":"mission-concurrency","startedAt":"%s","cycles":0,"reservations":{"active":{"reservedAt":"2000-01-01T00:00:00Z","capMin":%s}}}\n' \
  "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$fixture_minimum_cap_min" \
  >"$agent_repo/artifacts/agents/missions/mission-concurrency/fences.json"
stamp_fixture_contract mission-concurrency
agent_fails fence-concurrency 'mission fence refused job (concurrency)' env METASYSTEM_MISSION_TURN=mission-concurrency-t1-fixture "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id fence-concurrency --mission mission-concurrency --stream main --wait
assert_fence_ask mission-concurrency concurrency

make_fence_mission mission-timeout 10 10 2 2
stamp_fixture_contract mission-timeout
mission_timeout_result="$agent_fixture/mission-timeout.status"
wait_for_agent_census_fresh mission-timeout-job
(
  set +e
  cd "$agent_repo"
  # Same self-heal transient the shared runner retries; this dispatch runs
  # detached from that runner, so it carries the same bounded retry itself.
  for _ in 1 2 3; do
    output=$(METASYSTEM_MISSION_TURN=mission-timeout-t1-fixture scripts/agents/dispatch.sh dispatch --role design-critic --brief "$timeout_brief" --job-id mission-timeout-job --mission mission-timeout --stream main --cap-min "$fixture_minimum_cap_min" --wait 2>&1)
    driver_status=$?
    printf '%s\n' "$output"
    [[ $driver_status -eq 0 ]] && break
    # Exit 9 = the typed arming-window transient (script-validate-3/D34).
    [[ $driver_status -eq 9 ]] || break
    # Same record boundary as the shared runner: once the job exists, the
    # nonzero status is the fixture's answer (here: the reaped job's exit 4).
    # Re-dispatching a reaped job id spawned a zombie adapter that raced
    # the suite's own git operations in this repository.
    [[ -e artifacts/agents/jobs/mission-timeout-job.json ]] && break
    sleep 1
  done
  printf '%s\n' "$driver_status" >"$mission_timeout_result"
) >"$agent_fixture/mission-timeout.out" 2>&1 &
mission_timeout_driver=$!
wait_for_agent_status mission-timeout-job running
# capDeadline first, startedAt only as fallback — backdate both (see the
# timed fixture above); json set stages and renames, so the rewrite of
# this live record stays atomic.
"$engine" json set --file "$agent_repo/artifacts/agents/jobs/mission-timeout-job.json" \
  --field startedAt=2000-01-01T00:00:00Z --field capDeadline=2000-01-01T00:01:00Z
run_agent_fixture mission-timeout-reap mission-timeout-job "$agent_dispatch" reap --job mission-timeout-job
wait_for_agent_fixture_process mission-timeout-driver mission-timeout-job "$mission_timeout_driver"
[[ "$(cat "$mission_timeout_result")" == 4 ]] || {
  echo "mission job timeout did not map to exit 4 (got $(cat "$mission_timeout_result"))" >&2
  echo "status: $("$engine" json get --file "$agent_repo/artifacts/agents/jobs/mission-timeout-job.json" --field status --default None 2>/dev/null || true) error: $("$engine" json get --file "$agent_repo/artifacts/agents/jobs/mission-timeout-job.json" --field error --default None 2>/dev/null || true) phase: $("$engine" json get --file "$agent_repo/artifacts/agents/jobs/mission-timeout-job.json" --field phase --default None 2>/dev/null || true)" >&2
  echo "--- driver output:" >&2; sed -n '1,40p' "$agent_fixture/mission-timeout.out" >&2 2>/dev/null || true
  exit 1; }
assert_fence_ask mission-timeout job-cap-min

# A provider-native unit with the same spelling from another provider stays
# a separate typed tuple; no heterogeneous mission total exists.
other_provider_staged=$(mktemp "$agent_repo/artifacts/agents/jobs/.other-provider.XXXXXX")
printf '%s\n' '{"jobId":"other-provider","mission":"mission-alpha","runtime":"other","status":"completed","usage":{"availability":"native","inputTokens":3,"cachedInputTokens":null,"outputTokens":null,"reasoningTokens":null,"cost":null,"providerUnits":{"name":"fake-unit","value":5}}}' \
  >"$other_provider_staged"
mv "$other_provider_staged" "$agent_repo/artifacts/agents/jobs/other-provider.json"
"$agent_repo/bin/metasystem" mission fence-aggregate-usage --repo "$agent_repo" --mission mission-alpha
mission_usage_fake= mission_usage_other=
while IFS= read -r mission_usage_item; do
  [[ -n "$mission_usage_item" ]] || continue
  mission_usage_unit=$("$engine" json get --value "$mission_usage_item" --field unit)
  [[ "$mission_usage_unit" != provider.total ]] \
    || { echo "mission usage aggregated a heterogeneous provider total" >&2; exit 1; }
  [[ "$mission_usage_unit" == provider.fake-unit ]] || continue
  case "$("$engine" json get --value "$mission_usage_item" --field provider)" in
    fake) mission_usage_fake=$("$engine" json get --value "$mission_usage_item" --field value) ;;
    other) mission_usage_other=$("$engine" json get --value "$mission_usage_item" --field value) ;;
  esac
done < <(json_elements "$("$engine" json get --file "$mission_alpha_usage" --field units)")
# 4 = explicit + inherited + the two envelope-allowlisted jobs F-4 added.
[[ "$mission_usage_fake" == 4 ]] \
  || { echo "fake provider unit total is ${mission_usage_fake:-missing}, want 4" >&2; exit 1; }
[[ "$mission_usage_other" == 5 ]] \
  || { echo "other-provider unit stayed out of its own typed tuple (${mission_usage_other:-missing}, want 5)" >&2; exit 1; }
agent_fails missing-mission-lease 'does not have a live' "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id missing-mission --mission missing
agent_fails ambiguous-mission 'ambiguous mission context' env METASYSTEM_MISSION_ID=mission-alpha METASYSTEM_MISSION_LEASE="$agent_repo/artifacts/agents/missions/mission-alpha/lease.json" \
  "$agent_dispatch" dispatch --role design-critic --brief "$happy_brief" --job-id mission-ambiguous --mission another
[[ "$("$engine" json get --file "$agent_repo/artifacts/agents/jobs/happy.json" --field mission)" == null ]] \
  || { echo "unstamped interactive dispatch gained mission authority" >&2; exit 1; }
kill "$mission_pid" 2>/dev/null || true
wait_for_agent_fixture_process mission-lease-holder - "$mission_pid" 2>/dev/null || true
export METASYSTEM_FAKE_PROCESS_IDENTITY_FILE="$agent_identity_fixture"

agent_fails unknown-status-job '' "$agent_dispatch" status --job absent
set +e
"$agent_dispatch" status --job absent >/dev/null 2>&1
unknown_status=$?
set -e
[[ $unknown_status -eq 6 ]] || { echo "unknown status job mapped to $unknown_status instead of 6" >&2; exit 1; }
printf '{malformed\n' >"$agent_repo/artifacts/agents/jobs/malformed-status.json"
set +e
"$agent_dispatch" status --job malformed-status >/dev/null 2>&1
malformed_record_status=$?
set -e
[[ $malformed_record_status -eq 7 ]] || { echo "malformed status record mapped to $malformed_record_status instead of 7" >&2; exit 1; }
# The corrupt probe record must not outlive its check: classification is
# fail-closed on corrupt job records (review lease-census-1/2), so a
# leftover poisons every later lease entry — the shutdown below refused
# on it, silently, when this line was missing.
rm -f "$agent_repo/artifacts/agents/jobs/malformed-status.json"

run_fixture_arm "dispatcher final shutdown" - \
  "$agent_repo/scripts/agents/arm-supervision.sh" --repo "$agent_repo" --shutdown \
  || { echo "dispatcher fixture shutdown failed" >&2; exit 1; }
agent_supervision_repo=
fi

if [[ "$fixture_scenario" == mission-runner ]]; then
# The minimal mission runner is exercised only through its fake host. The
# repository, origin, supervision set, signed contracts, frozen gate, turn
# records, state, and ledger are all real; only the model call is simulated.
# It prefers real process sources so the census can observe each freshly
# launched runner. A restricted host that denies process enumeration uses an
# authorized empty census table; runner identity remains covered separately by
# the mission-process identity fixture below.
runner_process_env=(env -u METASYSTEM_CENSUS_PROCESS_FILE -u METASYSTEM_FAKE_PROCESS_IDENTITY_FILE)
runner="$runner_repo/scripts/agents/mission-runner.sh"
runner_origin="$agent_fixture/runner-origin.git"
runner_mission_identity_fixture="$agent_fixture/runner-mission-process-identities.json"
runner_real_census="$agent_fixture/runner-real-census.json"
runner_real_census_ok=0
if "${runner_process_env[@]}" "$runner_repo/bin/metasystem" proc census \
    --root "$runner_repo" --repo "$runner_repo" --fingerprint runner-source-probe \
    --interval 5 --output "$runner_real_census" >/dev/null 2>&1; then
  [[ "$("$engine" json get --file "$runner_real_census" --field verdict)" == SUCCESS ]] \
    && runner_real_census_ok=1
fi
if (( ! runner_real_census_ok )); then
  runner_process_fixture="$agent_fixture/runner-processes.json"
  runner_identity_fixture="$agent_fixture/runner-process-identities.json"
  printf '[]\n' >"$runner_process_fixture"
  printf '{}\n' >"$runner_identity_fixture"
  runner_process_env=(env \
    METASYSTEM_CENSUS_PROCESS_FILE="$runner_process_fixture" \
    METASYSTEM_FAKE_PROCESS_IDENTITY_FILE="$runner_identity_fixture")
fi
mv "$runner_repo/scripts/agents/arm-supervision.sh" \
  "$runner_repo/scripts/agents/arm-supervision-real.sh"
cat >"$runner_repo/scripts/agents/arm-supervision.sh" <<'ARM'
#!/usr/bin/env bash
set -euo pipefail
script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
fixture_root=$(git -C "$script_dir" rev-parse --show-toplevel)
called_at=$(date +%s)
if "$script_dir/arm-supervision-real.sh" "$@"; then
  arm_status=0
else
  arm_status=$?
fi
echo "mission runner fixture real arm result (exit status $arm_status)" >&2
(( arm_status == 0 )) || exit "$arm_status"
[[ ${1:-} == fingerprint ]] && exit 0
for argument in "$@"; do [[ "$argument" == --shutdown ]] && exit 0; done
deadline=$((called_at + ${METASYSTEM_FIXTURE_AGENT_STATUS_CAP_SEC:?}))
while (( $(date +%s) <= deadline )); do
# A plain field read through the engine's JSON reader — the wait here is
# newer-than-my-call, not the freshness policy (script-validate-2/D34).
completed=$("$fixture_root/bin/metasystem" json get \
  --file "$fixture_root/artifacts/agents/supervision/last-census.json" \
  --field completedAtEpoch --default 0 2>/dev/null || true)
[[ ${completed:-0} -ge $called_at ]] && break
sleep "${METASYSTEM_FIXTURE_POLL_INTERVAL_SEC:?}"
done
if [[ -n "${METASYSTEM_MISSION_PROCESS_IDENTITY_FILE:-}" \
&& -f "$fixture_root/artifacts/agents/supervision/state.json" ]]; then
state_file="$fixture_root/artifacts/agents/supervision/state.json"
watcher_pid=$("$fixture_root/bin/metasystem" json get --file "$state_file" --field components.watcher.pid)
watcher_started=$("$fixture_root/bin/metasystem" json get --file "$state_file" --field components.watcher.pidStartedAt)
watcher_tag=$("$fixture_root/bin/metasystem" json get --file "$state_file" --field components.watcher.instanceTag)
reaper_pid=$("$fixture_root/bin/metasystem" json get --file "$state_file" --field components.reaper.pid)
reaper_started=$("$fixture_root/bin/metasystem" json get --file "$state_file" --field components.reaper.pidStartedAt)
reaper_tag=$("$fixture_root/bin/metasystem" json get --file "$state_file" --field components.reaper.instanceTag)
identity_staged=$(mktemp "$(dirname "$METASYSTEM_MISSION_PROCESS_IDENTITY_FILE")/.identities.XXXXXX")
printf '{"%s":{"pidStartedAt":%s,"command":"fixture %s"},"%s":{"pidStartedAt":%s,"command":"fixture %s"}}\n' \
  "$watcher_pid" "$watcher_started" "$watcher_tag" \
  "$reaper_pid" "$reaper_started" "$reaper_tag" >"$identity_staged"
mv "$identity_staged" "$METASYSTEM_MISSION_PROCESS_IDENTITY_FILE"
fi
ARM
chmod +x "$runner_repo/scripts/agents/arm-supervision.sh"
runner_main_start=$("${runner_process_env[@]}" \
  "$runner_repo/bin/metasystem" proc started-at --pid "$$")
agent_supervision_repo=$runner_repo
track_armed_supervision "$runner_repo"
run_fixture_arm "mission runner initial arm" "$agent_fixture/runner-arming.out" \
  "${runner_process_env[@]}" METASYSTEM_AGENT_RUNTIME=fake \
    "$runner_repo/scripts/agents/arm-supervision.sh" \
    --repo "$runner_repo" --session runner-validator --pid "$$" \
    --start-time "$runner_main_start" --tag metasystem-main-fake-runner-validator \
  || { echo "mission runner fixture could not arm real-source supervision" >&2; exit 1; }
mkdir -p "$runner_repo/scripts" "$runner_repo/truth"
cat >"$runner_repo/scripts/gate.sh" <<'GATE'
#!/usr/bin/env bash
set -euo pipefail
score=$(cat candidate-score.txt)
printf 'metric=score=%s\n' "$score"
GATE
chmod +x "$runner_repo/scripts/gate.sh"
printf '0\n' >"$runner_repo/candidate-score.txt"
printf 'runner truth\n' >"$runner_repo/truth/reference.txt"
runner_git config user.name metasystem
runner_git config user.email metasystem@example.invalid
# The wall's projection boundary (HIW-O3): runtime state stays outside the
# shippable snapshot exactly as the real repository ignores it.
printf 'artifacts/\nbin/\nmetasystem.conf\n' >"$runner_repo/.gitignore"
runner_git add .gitignore scripts/gate.sh candidate-score.txt truth/reference.txt
runner_git commit -qm 'add mission runner instruments'
runner_git tag runner-instruments
git init -q -b main --bare "$runner_origin"
runner_branch=$(runner_git branch --show-current)
runner_git remote add origin "$runner_origin"
runner_git push -qu -u origin "$runner_branch"
git -C "$runner_origin" symbolic-ref HEAD "refs/heads/$runner_branch"
runner_git remote set-head origin -a >/dev/null

make_runner_contract() { # mission, behavior, cycle fence, optional heading, runtime, model, extra mission keys
  local mission=$1 behavior=$2 cycles=$3 bad_heading=${4:-} runtime=${5:-fake} model=${6:-fake-model} extra_keys=${7:-}
  local contract="$runner_repo/plans/mission-$1.contract.md" contract_sha
  mkdir -p "$runner_repo/plans"
  cat >"$contract" <<EOF
# Intent

Advance the candidate through one unattended fake-host turn.
$bad_heading

# Non-goals

Do not publish or deploy.

# Initial streams

Keep the primary stream active until the frozen gate passes.

\`\`\`mission
gate.command=scripts/gate.sh
gate.ref=runner-instruments
gate.paths=scripts/gate.sh
truth.paths=truth/reference.txt
truth.certification=certified
gate.direction=max
gate.threshold.score=>=1
gate.noise-floor.score=0
guard.score.command=scripts/gate.sh
guard.score.floor=1
guard.score.noise=0
guard.cadence=1
ledger.cycle-budget=5
ledger.no-gain-budget=3
ledger.accept-binary-gate-fuse=true
fence.wall-clock-hours=2
fence.cycles=$cycles
fence.jobs=4
fence.concurrency=1
fence.job-cap-min=$fixture_mission_job_cap_min
host.runtime=$runtime
host.model=$model
host.turn-cap-min=$fixture_minimum_cap_min
stream.primary=FAKEHOST:$behavior advance the candidate.
envelope.dependencies=jq
exposure=EUR:1${extra_keys:+
$extra_keys}
\`\`\`
EOF
  contract_sha=$("$runner_repo/scripts/assert-mission.sh" --seal --file "$contract")
  printf '\nApproval: name=Fixture-Human; date=2026-08-04; contract-sha256=%s\n' "$contract_sha" >>"$contract"
  runner_git add "plans/mission-$mission.contract.md"
  runner_git commit -qm "sign mission $mission"
  runner_git push -qu origin "$runner_branch"
}

wait_lease_released() { # mission, description
  local mission=$1 what=$2 started=$SECONDS deadline=$(( SECONDS + agent_fixture_cap_sec ))
  # The runner writes the terminal mission status inside its cycle and
  # releases the lease as it exits, so release trails the status by design.
  while (( SECONDS < deadline )); do
    [[ -e "$runner_repo/artifacts/agents/missions/$mission/lease.d" ]] || return 0
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
  done
  echo "$what retained its runner lease (elapsed: $((SECONDS - started))s; scaled cap: ${agent_fixture_cap_sec}s)" >&2
  exit 1
}

run_runner_expect() { # name, expected exit, command...
  local name=$1 expected=$2 result
  shift 2
  set +e
  run_agent_fixture_captured "$name" - "$agent_fixture/$name.out" "$@"
  result=$?
  set -e
  if [[ $result -ne $expected ]]; then
    echo "mission runner fixture $name exited $result instead of $expected" >&2
    cat "$agent_fixture/$name.out" >&2
    exit 1
  fi
}

wait_runner_status() { # mission, expected exit
  local mission=$1 expected=$2 result=7 started=$SECONDS deadline=$(( SECONDS + agent_fixture_cap_sec ))
  while (( SECONDS < deadline )); do
    set +e
    "$runner" status --mission "$mission" >"$agent_fixture/status-$mission.out" 2>&1
    result=$?
    set -e
    [[ $result -eq $expected ]] && return 0
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
  done
  echo "mission runner status timed out: $mission -> $expected (last exit: $result)" >&2
  cat "$agent_fixture/status-$mission.out" >&2
  return 1
}

wait_runner_file() { # path, description
  local path=$1 description=$2 started=$SECONDS deadline=$(( SECONDS + agent_fixture_cap_sec ))
  while (( SECONDS < deadline )); do
    [[ -e "$path" ]] && return 0
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
  done
  echo "mission runner file wait timed out: $description (elapsed: $((SECONDS - started))s; scaled cap: ${agent_fixture_cap_sec}s)" >&2
  return 1
}

watch_atomic_result() { # result path — first sight is the judgment
  local result_path=$1 deadline=$((SECONDS + agent_fixture_cap_sec)) result_field
  while (( SECONDS < deadline )); do
    if [[ -e "$result_path" ]]; then
      # The envelope must appear whole: a torn write is observable
      # exactly once, and this first read is that observation.
      "$engine" util json-validate --file "$result_path" \
        || { echo "host result was observable in a partial state: $result_path"; return 1; }
      for result_field in sessionId outcome usage rawPath returnPath; do
        "$engine" json get --file "$result_path" --field "$result_field" >/dev/null \
          || { echo "host result lacks $result_field"; cat "$result_path"; return 1; }
      done
      [[ "$("$engine" json strip --file "$result_path" --key sessionId --key outcome --key usage --key rawPath --key returnPath)" == '{}' ]] \
        || { echo "host result carries unexpected fields"; cat "$result_path"; return 1; }
      [[ "$("$engine" json get --file "$result_path" --field outcome)" == completed \
         && "$("$engine" json get --file "$result_path" --field sessionId)" == codex-fixture-session ]] \
        || { echo "host result outcome or session identity is wrong"; cat "$result_path"; return 1; }
      return 0
    fi
    sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"
  done
  echo "host result did not appear within ${agent_fixture_cap_sec}s: $result_path"
  return 1
}

start_atomic_result_watcher() { # result path, fixture name
  watch_atomic_result "$1" >"$agent_fixture/$2.out" 2>&1 &
  atomic_result_watcher_pid=$!
}

printf '{}\n' >"$runner_mission_identity_fixture"
export METASYSTEM_MISSION_PROCESS_IDENTITY_FILE="$runner_mission_identity_fixture"

make_runner_contract runner-cycle return-ok 5
# Both candidate-score commits are state-dependent without --allow-empty:
# whether the byte they write already sits at HEAD depends on how earlier
# missions' anchors interleaved, and each variant of this flake has now
# red-gated an unrelated change. The classification needs the sha to
# advance; the gate reads the file contents either way.
printf '1\n' >"$runner_repo/candidate-score.txt"
runner_git add candidate-score.txt
runner_git commit --allow-empty -qm 'improve mission runner candidate'
runner_git push -qu origin "$runner_branch"
close_bed_baseline "$runner_repo"
run_runner_expect runner-cycle-start 0 "${runner_process_env[@]}" METASYSTEM_AGENT_RUNTIME=fake "$runner" start --mission runner-cycle
wait_runner_status runner-cycle 10
cycle_turn=$(find "$runner_repo/artifacts/agents/missions/runner-cycle/turns" -mindepth 1 -maxdepth 1 -type d | head -1)
"$runner_repo/scripts/assert-turn-prompt.sh" --file "$cycle_turn/prompt.md" --turn "$cycle_turn"
cycle_prompt="$cycle_turn/prompt.md"
[[ "$(sed -n '/^$/q;p' "$cycle_prompt" | sed 's/: .*//')" \
   == $'Mission-Id\nTurn-Id\nCycle\nHost-Session\nRuntime\nModel\nReconciliation' ]] \
  || { echo "host-turn prompt header keys changed shape" >&2; sed -n '1,8p' "$cycle_prompt" >&2; exit 1; }
# The orchestrator preamble must follow the header break byte-for-byte.
cycle_header_end=$(grep -n -m1 '^$' "$cycle_prompt" | cut -d: -f1)
[[ -n "$cycle_header_end" ]] || { echo "host-turn prompt has no header break" >&2; exit 1; }
cycle_preamble="$runner_repo/scripts/agents/roles/orchestrator.md"
tail -c +"$(( $(head -n "$cycle_header_end" "$cycle_prompt" | wc -c) + 1 ))" "$cycle_prompt" \
  | head -c "$(( $(wc -c <"$cycle_preamble") ))" | cmp -s - "$cycle_preamble" \
  || { echo "host-turn prompt does not open with the orchestrator preamble" >&2; exit 1; }
cycle_prompt_text=$(cat "$cycle_prompt")
cycle_prev_index=0
for cycle_heading in '## Mission Contract' '## Ledger Tail' '## Open Asks' \
  '## Streams' '## Reconciliation' '## Landed Returns' '## This Turn'; do
  cycle_prefix=${cycle_prompt_text%%"$cycle_heading"*}
  [[ "$cycle_prefix" != "$cycle_prompt_text" ]] \
    || { echo "host-turn prompt lacks heading: $cycle_heading" >&2; exit 1; }
  (( ${#cycle_prefix} >= cycle_prev_index )) \
    || { echo "host-turn prompt headings are out of order at: $cycle_heading" >&2; exit 1; }
  cycle_prev_index=${#cycle_prefix}
done
grep -Fq -- '- Classification: contract-improved;' \
  "$runner_repo/artifacts/agents/missions/runner-cycle/ledger.md" \
  || { echo "full mission cycle did not record runner-measured contract improvement" >&2; exit 1; }
# The degenerate case, live (records/patience/patience-satellite-4.md): a mission
# whose contract carries no patience entries books no Patience line.
if grep -q 'Patience' "$runner_repo/artifacts/agents/missions/runner-cycle/ledger.md"; then
  echo "an unconfigured mission booked a Patience line" >&2; exit 1
fi

# Patience floors, live (records/patience/patience-satellite-4.md): a sealed floor
# plus a seeded orphan chain — one mission-stamped, terminal, started,
# unwitnessed record whose parent walk breaks — books the floor-independent
# orphan report in the same append as the cycle line, and the NEXT prompt's
# This Turn carries the projected line. The orphan is deliberately not
# closeable, so the runner's end-of-mission chain close never touches it.
# The prior mission's runner anchors its exit AFTER its status flips;
# committing while it still holds the git index races its lock.
wait_lease_released runner-cycle 'patience fixture entry'
# Reset the candidate below the gate threshold BEFORE sealing: the sealed
# baseline must be failing, or the first measurement completes the mission
# and no drought can ever accrue. Restored after the fixture.
printf '0\n' >"$runner_repo/candidate-score.txt"
runner_git add candidate-score.txt
runner_git commit --allow-empty -qm 'reset candidate for the patience fixture'
runner_git push -qu origin "$runner_branch"
make_runner_contract runner-patience return-ok 6 '' fake fake-model \
  'patience.rounds.design-critic.fake.fake-model=1'
mkdir -p "$runner_repo/artifacts/agents/jobs"
cat >"$runner_repo/artifacts/agents/jobs/pat-lost.json" <<EOF
{"jobId": "pat-lost", "mission": "runner-patience", "parentJob": "pat-gone",
 "status": "completed", "role": "design-critic", "runtime": "fake",
 "effectiveModel": "fake-model", "requestedModel": "fake-model",
 "startedAt": "$(date -u +%Y-%m-%dT%H:%M:%SZ)", "endedAt": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"}
EOF
close_bed_baseline "$runner_repo"
run_runner_expect runner-patience-start 0 "${runner_process_env[@]}" METASYSTEM_AGENT_RUNTIME=fake "$runner" start --mission runner-patience
wait_runner_status runner-patience 11
patience_ledger="$runner_repo/artifacts/agents/missions/runner-patience/ledger.md"
grep -Fq -- '- Patience: orphan=pat-lost rounds=1' "$patience_ledger" \
  || { echo "patience orphan report missing from the ledger" >&2; cat "$patience_ledger" >&2; exit 1; }
patience_turns=$(find "$runner_repo/artifacts/agents/missions/runner-patience/turns" \
  -mindepth 1 -maxdepth 1 -type d | sort)
patience_turn_1=$(printf '%s\n' "$patience_turns" | sed -n 1p)
patience_turn_2=$(printf '%s\n' "$patience_turns" | sed -n 2p)
[[ -n "$patience_turn_2" ]] || { echo "the patience mission ran fewer than two turns" >&2; exit 1; }
if grep -q 'Patience' "$patience_turn_1/prompt.md"; then
  echo "the first prompt carried a Patience line before any booking" >&2; exit 1
fi
grep -Fq 'Patience: orphan job pat-lost has unwitnessed spend' "$patience_turn_2/prompt.md" \
  || { echo "the second prompt did not project the patience line" >&2; exit 1; }
"$runner_repo/scripts/assert-turn-prompt.sh" \
  --file "$patience_turn_2/prompt.md" --turn "$patience_turn_2"
rm -f "$runner_repo/artifacts/agents/jobs/pat-lost.json"
wait_lease_released runner-patience 'patience fixture exit'
printf '1\n' >"$runner_repo/candidate-score.txt"
runner_git add candidate-score.txt
runner_git commit --allow-empty -qm 'restore candidate after the patience fixture'
runner_git push -qu origin "$runner_branch"
echo "patience floor fixtures passed"

# Exercise the real mission runner through the Codex host with only the paid
# model call replaced. Two turns prove first-turn and resumed identity,
# workspace entry for `codex exec resume`, typed usage, atomic host results,
# and the live instance tag that releases the runner's start gate.
codex_host_fixture="$agent_fixture/codex-host"
codex_host_bin="$codex_host_fixture/bin"
mkdir -p "$codex_host_bin"
cat >"$codex_host_bin/codex" <<'CODEX'
#!/usr/bin/env bash
set -euo pipefail
fixture=${METASYSTEM_CODEX_FIXTURE_DIR:?}
cap=${METASYSTEM_CODEX_FIXTURE_TIMEOUT_SEC:?}
mkdir -p "$fixture"
if [[ ! -e "$fixture/request-1.args" ]]; then
sequence=1
elif [[ ! -e "$fixture/request-2.args" ]]; then
sequence=2
else
echo "codex host fixture received an unexpected third turn" >&2
exit 9
fi
printf '%s\0' "$@" >"$fixture/request-$sequence.args"
printf '%s\n' "$PWD" >"$fixture/request-$sequence.cwd"
prompt="$fixture/request-$sequence.prompt"
cat >"$prompt"
printf 'ready\n' >"$fixture/ready-$sequence"
deadline=$((SECONDS + cap))
while [[ ! -e "$fixture/release-$sequence" ]]; do
(( SECONDS < deadline )) || { echo "codex host fixture release $sequence timed out" >&2; exit 9; }
sleep "${METASYSTEM_FIXTURE_POLL_INTERVAL_SEC:?}"
done
output=
arguments=("$@")
for ((index=0; index<${#arguments[@]}; index++)); do
if [[ ${arguments[$index]} == -o && $((index + 1)) -lt ${#arguments[@]} ]]; then
  output=${arguments[$((index + 1))]}
fi
done
[[ -n "$output" ]] || { echo "codex host fixture received no -o path" >&2; exit 9; }
if [[ $sequence -eq 2 ]]; then
printf '1\n' >"$PWD/candidate-score.txt"
git add candidate-score.txt
# What the classification needs is the candidate SHA advancing with score 1,
# not a content change: whether 1 is already committed here depends on how
# the earlier missions interleaved, and requiring a diff made this a flake.
# hooksPath bypass: a simulated host act in the harness, not a real
# agent commit — the enrolled guard (R2-11) is for the real thing.
git -c core.hooksPath=/dev/null commit --allow-empty -qm 'improve candidate from codex host fixture'
fi
stub_headers=$(sed -n '/^$/q;p' "$prompt")
stub_header() { printf '%s\n' "$stub_headers" | sed -n "s/^$1: //p" | head -1; }
stub_host_session=$(stub_header Host-Session)
if [[ $sequence -eq 1 ]]; then
[[ "$stub_host_session" == none ]] \
  || { echo "codex host fixture: first turn declared a session: $stub_host_session" >&2; exit 9; }
stub_declared=null
else
[[ "$stub_host_session" == codex-fixture-session ]] \
  || { echo "codex host fixture: resumed turn lost its session: $stub_host_session" >&2; exit 9; }
stub_declared='"codex-fixture-session"'
fi
stub_cycle=$(stub_header Cycle)
[[ "$stub_cycle" =~ ^[0-9]+$ ]] \
  || { echo "codex host fixture: cycle is not a number: $stub_cycle" >&2; exit 9; }
printf '{"turnId":"%s","missionId":"%s","cycle":%s,"dispatched":[],"certified":[],"streamUpdatesRequested":[],"askCandidates":[],"factsForLedger":[],"gaps":[],"identity":{"runtime":"%s","model":"%s","sessionId":%s}}\n' \
  "$(stub_header Turn-Id)" "$(stub_header Mission-Id)" "$stub_cycle" \
  "$(stub_header Runtime)" "$(stub_header Model)" "$stub_declared" >"$output"
printf '%s\n' \
'{"type":"thread.started","thread_id":"codex-fixture-session"}' \
"{\"type\":\"turn.started\",\"turn_id\":\"codex-fixture-turn-$sequence\"}" \
'{"type":"turn.completed","usage":{"input_tokens":10,"cached_input_tokens":2,"output_tokens":4,"reasoning_output_tokens":1}}'
CODEX
chmod +x "$codex_host_bin/codex"

printf '0\n' >"$runner_repo/candidate-score.txt"
runner_git add candidate-score.txt
runner_git commit --allow-empty -qm 'reset candidate for codex host mission'
runner_git push -qu origin "$runner_branch"
# The stub codex host writes candidate-score.txt mid-turn to advance the
# gate — host-authored product bytes the wall outlaws (D100). The leg's
# subject is the ADAPTER wiring, so the contract declares that one file
# by name (HIW-R2-03), the same acknowledge-by-name doctrine as the
# binary-gate fuse key above.
make_runner_contract runner-codex return-ok 5 '' codex gpt-5-fixture 'wall.host-artifacts=candidate-score.txt'
close_bed_baseline "$runner_repo"
run_runner_expect runner-codex-start 0 "${runner_process_env[@]}" \
  PATH="$codex_host_bin:$PATH" METASYSTEM_AGENT_RUNTIME=fake \
  METASYSTEM_CODEX_FIXTURE_DIR="$codex_host_fixture" \
  METASYSTEM_CODEX_FIXTURE_TIMEOUT_SEC="$agent_fixture_cap_sec" \
  "$runner" start --mission runner-codex
wait_runner_file "$codex_host_fixture/ready-1" "codex host first turn"
codex_turn_one=$(find "$runner_repo/artifacts/agents/missions/runner-codex/turns" \
  -mindepth 1 -maxdepth 1 -type d | head -1)
codex_turn_one_record="$codex_turn_one/turn.json"
[[ "$("$engine" json get --file "$codex_turn_one_record" --field status)" == running ]] \
  || { echo "first codex host turn is not running" >&2; cat "$codex_turn_one_record" >&2; exit 1; }
codex_host_pid=$("$engine" json get --file "$codex_turn_one_record" --field pid)
codex_host_tag=$("$engine" json get --file "$codex_turn_one_record" --field instanceTag)
[[ "$codex_host_pid" == "$("$engine" json get --file "$codex_turn_one_record" --field pgid)" ]] \
  || { echo "codex host is not its own process-group leader" >&2; exit 1; }
[[ "$codex_host_tag" == metasystem-host-* ]] \
  || { echo "codex host instance tag changed shape: $codex_host_tag" >&2; exit 1; }
codex_host_process=$("$runner_repo/bin/metasystem" proc probe --pid "$codex_host_pid")
[[ "$codex_host_process" == *"$codex_host_tag"* ]] \
  || { echo "codex host process did not carry its recorded instance tag" >&2; exit 1; }
start_atomic_result_watcher "$codex_turn_one/result.json" codex-result-one
codex_result_one_watcher=$atomic_result_watcher_pid
touch "$codex_host_fixture/release-1"
wait_for_agent_fixture_process codex-result-one - "$codex_result_one_watcher" \
  || { cat "$agent_fixture/codex-result-one.out" >&2; exit 1; }
wait_runner_file "$codex_host_fixture/ready-2" "codex host resumed turn"
codex_turn_two=
for codex_turn_candidate in "$runner_repo/artifacts/agents/missions/runner-codex/turns"/*/turn.json; do
  [[ -f "$codex_turn_candidate" ]] || continue
  [[ "$("$engine" json get --file "$codex_turn_candidate" --field cycle)" == 2 ]] || continue
  codex_turn_two=${codex_turn_candidate%/turn.json}
  break
done
[[ -n "$codex_turn_two" ]] || { echo "second codex host turn was not created" >&2; exit 1; }
start_atomic_result_watcher "$codex_turn_two/result.json" codex-result-two
codex_result_two_watcher=$atomic_result_watcher_pid
touch "$codex_host_fixture/release-2"
wait_for_agent_fixture_process codex-result-two - "$codex_result_two_watcher" \
  || { cat "$agent_fixture/codex-result-two.out" >&2; exit 1; }
wait_runner_status runner-codex 10
wait_lease_released runner-codex "completed codex-host mission"
argv_contains() { # needle, argv... — exact element membership
  local needle=$1 arg
  shift
  for arg in "$@"; do [[ "$arg" == "$needle" ]] && return 0; done
  return 1
}
argv_value_after() { # flag, argv... — the value following the first exact flag
  local flag=$1
  shift
  while (( $# > 1 )); do
    [[ "$1" == "$flag" ]] && { printf '%s\n' "$2"; return 0; }
    shift
  done
  return 1
}
codex_args_one=()
while IFS= read -r -d '' codex_arg; do codex_args_one+=("$codex_arg"); done \
  <"$codex_host_fixture/request-1.args"
codex_args_two=()
while IFS= read -r -d '' codex_arg; do codex_args_two+=("$codex_arg"); done \
  <"$codex_host_fixture/request-2.args"
[[ "${codex_args_one[0]}" == exec && "${codex_args_one[1]}" == --json ]] \
  || { echo "first codex argv does not open with exec --json" >&2; exit 1; }
[[ "$(argv_value_after -m "${codex_args_one[@]}")" == gpt-5-fixture ]] \
  || { echo "first codex argv lost its model flag" >&2; exit 1; }
[[ "$(argv_value_after --sandbox "${codex_args_one[@]}")" == workspace-write ]] \
  || { echo "first codex argv lost its sandbox mode" >&2; exit 1; }
[[ "$(argv_value_after -C "${codex_args_one[@]}")" == "$runner_repo" ]] \
  || { echo "first codex argv does not enter the workspace" >&2; exit 1; }
argv_contains 'approval_policy="never"' "${codex_args_one[@]}" \
  || { echo "first codex argv lost the never-approve policy" >&2; exit 1; }
argv_contains 'sandbox_workspace_write.network_access=true' "${codex_args_one[@]}" \
  || { echo "first codex argv lost network access" >&2; exit 1; }
[[ "${codex_args_two[0]}" == exec && "${codex_args_two[1]}" == resume && "${codex_args_two[2]}" == --json ]] \
  || { echo "resumed codex argv does not open with exec resume --json" >&2; exit 1; }
if argv_contains -C "${codex_args_two[@]}"; then
  echo "resumed codex argv re-entered the workspace flag" >&2; exit 1
fi
argv_contains 'model="gpt-5-fixture"' "${codex_args_two[@]}" \
  || { echo "resumed codex argv lost its model override" >&2; exit 1; }
argv_contains 'sandbox_mode="workspace-write"' "${codex_args_two[@]}" \
  || { echo "resumed codex argv lost its sandbox mode" >&2; exit 1; }
argv_contains 'approval_policy="never"' "${codex_args_two[@]}" \
  || { echo "resumed codex argv lost the never-approve policy" >&2; exit 1; }
argv_contains 'sandbox_workspace_write.network_access=true' "${codex_args_two[@]}" \
  || { echo "resumed codex argv lost network access" >&2; exit 1; }
argv_contains codex-fixture-session "${codex_args_two[@]}" \
  || { echo "resumed codex argv lost the session id" >&2; exit 1; }
[[ "$(cat "$codex_host_fixture/request-1.cwd")" == "$runner_repo" \
   && "$(cat "$codex_host_fixture/request-2.cwd")" == "$runner_repo" ]] \
  || { echo "codex host did not run from the workspace root" >&2; exit 1; }
# Exact usage equality through one canonical rendering (keys sorted) on
# both sides.
codex_expected_usage=$("$engine" json get --field u --value \
  '{"u":{"availability":"native","inputTokens":10,"cachedInputTokens":2,"outputTokens":4,"reasoningTokens":1,"cost":null,"providerUnits":null}}')
for codex_turn_dir in "$codex_turn_one" "$codex_turn_two"; do
  codex_result="$codex_turn_dir/result.json"
  for codex_result_field in sessionId outcome usage rawPath returnPath; do
    "$engine" json get --file "$codex_result" --field "$codex_result_field" >/dev/null \
      || { echo "codex result lacks $codex_result_field: $codex_result" >&2; exit 1; }
  done
  [[ "$("$engine" json strip --file "$codex_result" --key sessionId --key outcome --key usage --key rawPath --key returnPath)" == '{}' ]] \
    || { echo "codex result carries unexpected fields: $codex_result" >&2; exit 1; }
  [[ "$("$engine" json get --file "$codex_result" --field outcome)" == completed \
     && "$("$engine" json get --file "$codex_result" --field sessionId)" == codex-fixture-session ]] \
    || { echo "codex result outcome or session is wrong: $codex_result" >&2; exit 1; }
  [[ "$("$engine" json get --file "$codex_result" --field usage)" == "$codex_expected_usage" ]] \
    || { echo "codex result usage changed shape: $codex_result" >&2; exit 1; }
  [[ "$(resolve_existing_path "$("$engine" json get --file "$codex_result" --field rawPath)")" \
     == "$(resolve_existing_path "$codex_turn_dir/raw.out")" ]] \
    || { echo "codex result rawPath does not resolve to the turn's raw.out" >&2; exit 1; }
  [[ "$(resolve_existing_path "$("$engine" json get --file "$codex_result" --field returnPath)")" \
     == "$(resolve_existing_path "$codex_turn_dir/return.json")" ]] \
    || { echo "codex result returnPath does not resolve to the turn's return.json" >&2; exit 1; }
  if compgen -G "$codex_turn_dir/result.json.*.tmp" >/dev/null; then
    echo "codex result staging residue survived: $codex_turn_dir" >&2; exit 1
  fi
done
[[ "$("$engine" json get --file "$codex_turn_one/return.json" --field identity.sessionId)" == null ]] \
  || { echo "first codex return declared a session identity" >&2; exit 1; }
codex_second_host_session=$("$engine" json get --file "$codex_turn_two/turn.json" --field hostSession)
[[ "$codex_second_host_session" == codex-fixture-session ]] \
  || { echo "second codex turn record lost its host session" >&2; exit 1; }
[[ "$("$engine" json get --file "$codex_turn_two/return.json" --field identity.sessionId)" == "$codex_second_host_session" ]] \
  || { echo "second codex return does not carry the resumed session" >&2; exit 1; }
runner_git push -qu origin "$runner_branch"

run_runner_expect prompt-missing-turn 1 "$runner_repo/bin/metasystem" mission prompt-assemble \
  --repo "$runner_repo" \
  --mission runner-cycle --turn runner-cycle-t99-missing --output "$agent_fixture/missing-prompt.md"
grep -Fq 'missing turn record' "$agent_fixture/prompt-missing-turn.out" \
  || { echo "prompt assembler did not name its missing turn record refusal" >&2; exit 1; }
run_runner_expect prompt-oversized 1 env METASYSTEM_MISSION_MAX_PROMPT_KB=1 \
  "$runner_repo/bin/metasystem" mission prompt-assemble --repo "$runner_repo" --mission runner-cycle \
  --turn "$(basename "$cycle_turn")" --output "$agent_fixture/oversized-prompt.md"
grep -Fq 'oversized block' "$agent_fixture/prompt-oversized.out" \
  || { echo "prompt assembler did not name the oversized block" >&2; exit 1; }

make_runner_contract runner-bad-prompt return-ok 5 '## Streams'
close_bed_baseline "$runner_repo"
run_runner_expect runner-bad-prompt-start 3 "${runner_process_env[@]}" METASYSTEM_AGENT_RUNTIME=fake "$runner" start --mission runner-bad-prompt
wait_runner_status runner-bad-prompt 11
bad_turn=$(find "$runner_repo/artifacts/agents/missions/runner-bad-prompt/turns" -mindepth 1 -maxdepth 1 -type d | head -1)
[[ ! -e "$bad_turn/raw.out" ]] || { echo "prompt-checker refusal launched the fake host" >&2; exit 1; }
grep -Fq 'prompt-refused' "$bad_turn/turn.json" \
  || { echo "prompt-checker refusal was not recorded on the turn" >&2; exit 1; }

make_runner_contract runner-ghost dispatch-ghost 5
close_bed_baseline "$runner_repo"
run_runner_expect runner-ghost-start 0 "${runner_process_env[@]}" METASYSTEM_AGENT_RUNTIME=fake "$runner" start --mission runner-ghost
wait_runner_status runner-ghost 10
ghost_mission="$runner_repo/artifacts/agents/missions/runner-ghost"
# The newest HOST-TURN entry: post-verification entries conclude turns
# but carry no adjudication.
ghost_last_turn=
while IFS= read -r ghost_entry; do
  [[ -n "$ghost_entry" ]] || continue
  [[ "$("$engine" json get --value "$ghost_entry" --field kind --default '')" == wall-verification ]] && continue
  ghost_last_turn=$ghost_entry
done < <(json_elements "$("$engine" json get --file "$ghost_mission/state.json" --field turnLog)")
[[ -n "$ghost_last_turn" ]] || { echo "runner-ghost recorded no host turns" >&2; exit 1; }
ghost_rejected=$("$engine" json get --value "$ghost_last_turn" --field rejected) \
  || { echo "the ghost host turn carries no rejected set" >&2; exit 1; }
ghost_rejected_count=0 ghost_rejected_item=
while IFS= read -r ghost_candidate; do
  [[ -n "$ghost_candidate" ]] || continue
  ghost_rejected_count=$((ghost_rejected_count + 1))
  ghost_rejected_item=$ghost_candidate
done < <(json_elements "$ghost_rejected")
[[ "$ghost_rejected_count" == 1 ]] \
  || { echo "ghost dispatch produced $ghost_rejected_count rejections, want 1" >&2; exit 1; }
[[ "$("$engine" json get --value "$ghost_rejected_item" --field kind)" == dispatched ]] \
  || { echo "the ghost rejection is not a dispatched rejection" >&2; exit 1; }
case "$("$engine" json get --value "$ghost_rejected_item" --field reason)" in
  *'does not exist'*) ;;
  *) echo "the ghost rejection reason does not name the missing job" >&2; exit 1 ;;
esac
ghost_ask="$ghost_mission/asks/$("$engine" json get --value "$ghost_rejected_item" --field askId).json"
[[ "$("$engine" json get --file "$ghost_ask" --field reasonClass)" == host-failure \
   && "$("$engine" json get --file "$ghost_ask" --field answeredAt)" == null ]] \
  || { echo "the ghost rejection ask is not an open host-failure ask" >&2; exit 1; }

make_runner_contract runner-fence return-ok 1
mkdir -p "$runner_repo/artifacts/agents/missions/runner-fence"
printf '{"schemaVersion":1,"missionId":"runner-fence","startedAt":"%s","cycles":1,"reservations":{}}\n' \
  "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  >"$runner_repo/artifacts/agents/missions/runner-fence/fences.json"
close_bed_baseline "$runner_repo"
run_runner_expect runner-fence-start 3 "${runner_process_env[@]}" METASYSTEM_AGENT_RUNTIME=fake "$runner" start --mission runner-fence
wait_runner_status runner-fence 11
fence_mission="$runner_repo/artifacts/agents/missions/runner-fence"
[[ "$("$engine" json get --file "$fence_mission/state.json" --field status)" == parked \
   && "$("$engine" json get --file "$fence_mission/state.json" --field parkReason)" == fence ]] \
  || { echo "the fence-refused mission is not parked on fence" >&2; exit 1; }
fence_open_ask=
for fence_ask in "$fence_mission/asks"/*.json; do
  [[ -f "$fence_ask" ]] || continue
  "$engine" util json-validate --file "$fence_ask" \
    || { echo "unparseable mission ask: $fence_ask" >&2; exit 1; }
  [[ "$("$engine" json get --file "$fence_ask" --field reasonClass)" == fence ]] || continue
  [[ "$("$engine" json get --file "$fence_ask" --field answeredAt)" == null ]] || continue
  fence_open_ask=1
done
[[ -n "$fence_open_ask" ]] \
  || { echo "the fence park raised no unanswered fence ask" >&2; exit 1; }

make_runner_contract runner-unverified return-ok 5
close_bed_baseline "$runner_repo"
run_runner_expect runner-unverified-start 3 "${runner_process_env[@]}" METASYSTEM_AGENT_RUNTIME=fake \
  METASYSTEM_FAKE_HOST_START_UNVERIFIED=1 "$runner" start --mission runner-unverified
wait_runner_status runner-unverified 11
unverified_mission="$runner_repo/artifacts/agents/missions/runner-unverified"
[[ "$("$engine" json get --file "$unverified_mission/state.json" --field parkReason)" == host-failure ]] \
  || { echo "the unverified-start mission did not park on host-failure" >&2; exit 1; }
unverified_turn_count=0 unverified_turn_record=
for unverified_turn in "$unverified_mission/turns"/*/turn.json; do
  [[ -f "$unverified_turn" ]] || continue
  unverified_turn_count=$((unverified_turn_count + 1))
  unverified_turn_record=$unverified_turn
done
[[ "$unverified_turn_count" == 1 ]] \
  || { echo "the unverified-start mission ran $unverified_turn_count turns, want 1" >&2; exit 1; }
[[ "$("$engine" json get --file "$unverified_turn_record" --field error)" == start-unverified ]] \
  || { echo "the failed turn does not carry start-unverified" >&2; exit 1; }
unverified_ask=
for unverified_ask_file in "$unverified_mission/asks"/*.json; do
  [[ -f "$unverified_ask_file" ]] || continue
  "$engine" util json-validate --file "$unverified_ask_file" \
    || { echo "unparseable mission ask: $unverified_ask_file" >&2; exit 1; }
  [[ "$("$engine" json get --file "$unverified_ask_file" --field reasonClass)" == host-failure ]] || continue
  [[ "$("$engine" json get --file "$unverified_ask_file" --field answeredAt)" == null ]] || continue
  unverified_ask=$("$engine" json get --file "$unverified_ask_file" --field askId)
  break
done
[[ -n "$unverified_ask" ]] \
  || { echo "no unanswered host-failure ask for the unverified start" >&2; exit 1; }
run_runner_expect runner-unverified-answer 0 "$runner" answer --mission runner-unverified \
  --ask "$unverified_ask" --answer acknowledged
wait_runner_status runner-unverified 0

run_fixture_arm "mission runner shutdown before resume" - \
  "${runner_process_env[@]}" "$runner_repo/scripts/agents/arm-supervision.sh" \
    --repo "$runner_repo" --shutdown
agent_supervision_repo=
[[ ! -e "$runner_repo/artifacts/agents/missions/runner-unverified/lease.d" ]] \
  || { echo "parked mission retained its runner lease" >&2; exit 1; }
agent_supervision_repo=$runner_repo
track_armed_supervision "$runner_repo"
run_runner_expect runner-unverified-resume 0 "${runner_process_env[@]}" METASYSTEM_AGENT_RUNTIME=fake "$runner" resume --mission runner-unverified
wait_runner_status runner-unverified 10
[[ -f "$runner_repo/artifacts/agents/supervision/state.json" ]] \
  || { echo "resume did not re-arm supervision" >&2; exit 1; }
wait_lease_released runner-unverified "completed resumed mission"
resumed_prompt=$(find "$runner_repo/artifacts/agents/missions/runner-unverified/turns" -name prompt.md | sort | tail -1)
grep -Fq 'Reconciliation: yes' "$resumed_prompt" \
  || { echo "resumed turn did not carry reconciliation" >&2; exit 1; }
grep -Fq $'\tfailed\tstart-unverified' "$resumed_prompt" \
  || { echo "resumed turn omitted the failed prior turn from reconciliation" >&2; exit 1; }

run_fixture_arm "mission runner final shutdown" - \
  "${runner_process_env[@]}" "$runner_repo/scripts/agents/arm-supervision.sh" \
    --repo "$runner_repo" --shutdown
agent_supervision_repo=
fi

if [[ "$fixture_scenario" == adapter-selftest ]]; then
agent_selftest_process_fixture="$agent_fixture/selftest-processes.json"
agent_selftest_identity_fixture="$agent_fixture/selftest-process-identities.json"
printf '[]\n' >"$agent_selftest_process_fixture"
printf '{}\n' >"$agent_selftest_identity_fixture"
export METASYSTEM_CENSUS_PROCESS_FILE="$agent_selftest_process_fixture"
export METASYSTEM_FAKE_PROCESS_IDENTITY_FILE="$agent_selftest_identity_fixture"
agent_supervision_repo=$agent_selftest_repo
track_armed_supervision "$agent_selftest_repo"
agent_selftest_main_start=$("$agent_selftest_repo/bin/metasystem" proc started-at --pid "$$")
run_fixture_arm "adapter selftest initial arm" "$agent_fixture/selftest-arming.out" \
  env METASYSTEM_AGENT_RUNTIME=fake "$agent_selftest_repo/scripts/agents/arm-supervision.sh" \
    --repo "$agent_selftest_repo" --session selftest-validator --pid "$$" \
    --start-time "$agent_selftest_main_start" --tag metasystem-main-fake-selftest-validator \
  || { echo "adapter selftest fixture could not arm supervision" >&2; exit 1; }
fake_selftest_adapter="$agent_selftest_repo/scripts/agents/adapters/fake.sh"
run_agent_fixture_captured fake-selftest - "$agent_fixture/fake-selftest.out" "$fake_selftest_adapter" selftest
grep -Fq 'full protocol sequence' "$agent_fixture/fake-selftest.out" \
  || { echo "fake adapter selftest did not run its full protocol sequence" >&2; exit 1; }
selftest_newest=$(ls -t "$agent_selftest_repo/artifacts/agents/selftests"/fake-selftest-*.json 2>/dev/null | head -1)
[[ -n "$selftest_newest" ]] || { echo "the fake selftest wrote no pass record" >&2; exit 1; }
selftest_proven=$("$engine" json get --file "$selftest_newest" --field provenBehaviorally)
[[ "$selftest_proven" == *'"resume-identity"'* ]] \
  || { echo "the selftest record does not prove resume-identity" >&2; exit 1; }
[[ "$selftest_proven" == *'"denied-write"'* && "$selftest_proven" == *'"denied-network"'* ]] \
  || { echo "the selftest record does not prove the denied probes" >&2; exit 1; }
[[ "$("$engine" json get --file "$selftest_newest" --field constructedOnly)" != *'"network"'* ]] \
  || { echo "network stayed constructed-only in the selftest record" >&2; exit 1; }

run_fixture_arm "adapter selftest final shutdown" - \
  "$agent_selftest_repo/scripts/agents/arm-supervision.sh" --repo "$agent_selftest_repo" --shutdown
agent_supervision_repo=
unset METASYSTEM_CENSUS_PROCESS_FILE METASYSTEM_FAKE_PROCESS_IDENTITY_FILE \
  METASYSTEM_MISSION_PROCESS_IDENTITY_FILE
fi

if [[ "$fixture_scenario" == steward-continuation ]]; then
# ---------------------------------------------------------------------------
# The steward's continuation path, end to end: a provably dead worker's
# revival flows through the real dispatcher into the fake adapter, returns
# on the role's schema, and the reaper closes the chain. Companion legs
# pin the refusals: extra selection flags, a replayed authorization, a
# and staged-bytes drift (the superseded-generation refusal is
# pinned in the Go tests).
steward_repo="$agent_fixture/steward-repo"
cp -R "$agent_repo" "$steward_repo"
steward_repo=$(cd "$steward_repo" && pwd -P)
rm -rf "$steward_repo/artifacts"
steward_enrolled_engine=$steward_repo/bin/metasystem
enroll_fixture_repo "$steward_repo" "$steward_enrolled_engine"
"$steward_repo/bin/metasystem" config tailor --conf "$steward_repo/metasystem.conf" --runtimes fake \
  --set role.default.model.fake=fake-model \
  --set role.steward-continuation.runtime=fake
git -C "$steward_repo" -c user.name=metasystem -c user.email=metasystem@example.invalid add -A
git -C "$steward_repo" -c core.hooksPath=/dev/null -c user.name=metasystem -c user.email=metasystem@example.invalid commit -qm steward-base
git -C "$steward_repo" config metasystem.steward.notify-command true
mkdir -p "$steward_repo/plans"
printf '# Goals\n\n## Current goal: fix-it \xe2\x80\x94 Repair the thing\n- Origin: main\n- Next step: Repair it.\n' > "$steward_repo/plans/goals.md"

# A death proof needs a completed supervision census: arm the steward
# repository's supervision like every other fixture repository, so
# the empty worker set is PROVEN rather than unknown.
steward_process_fixture="$agent_fixture/steward-processes.json"
steward_identity_fixture="$agent_fixture/steward-process-identities.json"
printf '[]\n' >"$steward_process_fixture"
printf '{}\n' >"$steward_identity_fixture"
export METASYSTEM_CENSUS_PROCESS_FILE="$steward_process_fixture"
export METASYSTEM_FAKE_PROCESS_IDENTITY_FILE="$steward_identity_fixture"
track_armed_supervision "$steward_repo"
# The announced main arms from its own process, remains alive through the
# armer's first census, and exits when arming returns. The next census then
# proves an empty worker set; a live announced main would be a live worker,
# and the steward would rightly refuse to revive beside it.
steward_arm_driver=$agent_fixture/steward-arm-driver.sh
cat >"$steward_arm_driver" <<'STEWARD_ARM_DRIVER'
#!/usr/bin/env bash
set -euo pipefail
engine=$1
arm=$2
repo=$3
started=$("$engine" proc started-at --pid "$$")
METASYSTEM_BIN="$engine" METASYSTEM_AGENT_RUNTIME=fake "$arm" \
  --repo "$repo" --session steward-fixture --pid "$$" \
  --start-time "$started" --tag steward-fixture-main
STEWARD_ARM_DRIVER
chmod +x "$steward_arm_driver"
run_fixture_arm "steward end-to-end initial arm" - \
  "$steward_arm_driver" "$steward_enrolled_engine" \
    "$steward_repo/scripts/agents/arm-supervision.sh" "$steward_repo" \
  || { echo "steward end-to-end: supervision arming failed" >&2; exit 1; }
# The dispatch pipeline requires a fresh capability snapshot for the
# runtime it launches; probe the fake adapter like every dispatching
# fixture repository does.
"$steward_repo/scripts/agents/adapters/fake.sh" probe >/dev/null \
  || { echo "steward end-to-end: fake adapter probe failed" >&2; exit 1; }

# The full path: revive launches exactly once through dispatch and the
# fake adapter; the tick then reaps and closes.
steward_out=$(cd "$steward_repo" && METASYSTEM_BIN="$steward_enrolled_engine" \
  "$steward_enrolled_engine" steward revive --repo "$steward_repo" 2>&1) \
  || { echo "steward end-to-end: revive failed: $steward_out" >&2; exit 1; }
grep -q "launched=true" <<<"$steward_out" \
  || { echo "steward end-to-end: expected a launch, got: $steward_out" >&2; exit 1; }
steward_job=$(ls "$steward_repo/artifacts/agents/jobs/" | sed -n 's/\.json$//p' | head -1)
[[ -n "$steward_job" ]] || { echo "steward end-to-end: no job record" >&2; exit 1; }
steward_role=$("$steward_repo/bin/metasystem" json get --file "$steward_repo/artifacts/agents/jobs/$steward_job.json" --field role)
[[ "$steward_role" == steward-continuation ]] \
  || { echo "steward end-to-end: record role is $steward_role" >&2; exit 1; }
# No-wait dispatch returns before the fake worker finishes: the
# return and the record's end arrive within the harness cap.
steward_wait_cap=$(harness_fixture_cap agent-command)
steward_deadline=$((SECONDS + steward_wait_cap))
until [[ -f "$steward_repo/artifacts/agents/$steward_job/rounds/1/return.json" ]]; do
  (( SECONDS < steward_deadline )) \
    || { echo "steward end-to-end: no return within ${steward_wait_cap}s" >&2; exit 1; }
  sleep 0.2
done
until [[ -n "$("$steward_repo/bin/metasystem" json get --file "$steward_repo/artifacts/agents/jobs/$steward_job.json" --field endedAt --default "")" ]]; do
  (( SECONDS < steward_deadline )) \
    || { echo "steward end-to-end: the job never ended within ${steward_wait_cap}s" >&2; exit 1; }
  sleep 0.2
done
steward_tick=$(cd "$steward_repo" && METASYSTEM_BIN="$steward_enrolled_engine" \
  "$steward_enrolled_engine" steward tick --repo "$steward_repo") \
  || { echo "steward end-to-end: tick failed" >&2; exit 1; }
grep -q '"jobId": "'"$steward_job"'"' <<<"$steward_tick" \
  || { echo "steward end-to-end: the tick did not reap the continuation: $steward_tick" >&2; exit 1; }
steward_closed=$("$steward_repo/bin/metasystem" json get --file "$steward_repo/artifacts/agents/jobs/$steward_job.json" --field chainClosed)
[[ "$steward_closed" == true ]] \
  || { echo "steward end-to-end: chain not closed: $steward_closed" >&2; exit 1; }
steward_status=$("$steward_repo/bin/metasystem" json get --file "$steward_repo/artifacts/agents/jobs/$steward_job.json" --field status)
[[ "$steward_status" == completed ]] \
  || { echo "steward end-to-end: job must complete, got $steward_status" >&2; exit 1; }
grep -q "with a valid return" <<<"$steward_tick" \
  || { echo "steward end-to-end: the reap must certify a VALID return, got: $steward_tick" >&2; exit 1; }

# A replayed authorization launches nothing.
steward_nonce=$(ls "$steward_repo/artifacts/agents/steward/consumed/" | sed -n 's/\.json$//p' | head -1)
if "$steward_repo/scripts/agents/dispatch.sh" --steward-intent "$steward_nonce" >/dev/null 2>&1; then
  echo "steward replay: a consumed authorization must launch nothing" >&2; exit 1
fi

# Companion selection flags refuse before anything exists.
if "$steward_repo/scripts/agents/dispatch.sh" --steward-intent "$steward_nonce" --wait >/dev/null 2>&1; then
  echo "steward flags: --wait beside --steward-intent must refuse" >&2; exit 1
fi
if "$steward_repo/scripts/agents/dispatch.sh" --steward-intent "$steward_nonce" --role implementer >/dev/null 2>&1; then
  echo "steward flags: --role beside --steward-intent must refuse" >&2; exit 1
fi

# A notifier outage cannot gate a lawful automatic repair. Staged-byte drift
# remains covered at the staging owner's focused test; this end-to-end leg
# proves the recovery order at the real dispatch boundary.
git -C "$steward_repo" config metasystem.steward.notify-command "exit 1"
steward_outage=$(cd "$steward_repo" && METASYSTEM_BIN="$steward_enrolled_engine" \
  "$steward_enrolled_engine" steward revive --repo "$steward_repo" 2>&1) \
  || { echo "steward heal-first: notifier outage blocked revival: $steward_outage" >&2; exit 1; }
grep -q "launched=true" <<<"$steward_outage" \
  || { echo "steward heal-first: notifier outage produced no launch: $steward_outage" >&2; exit 1; }

echo "steward continuation fixtures passed"
fi
