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
  run_fixture_bed_scenarios land "land fixtures passed (5 isolated legs)" \
    "$fixture_bed_script" push-retry step-failure new-plan goal tier-one
fi

tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-land.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
real_git=$(command -v git)

make_leg() { # name
  leg_root=$tmp/$1
  leg_seed=$leg_root/seed
  leg_remote=$leg_root/origin.git
  leg_local=$leg_root/local
  leg_peer=$leg_root/peer
  mkdir -p "$leg_seed/scripts/agents" "$leg_seed/plans" "$leg_seed/bin"
  cp "$root/scripts/agents/land.sh" "$leg_seed/scripts/agents/land.sh"
  cp "$root/scripts/agents/coverage-delta.sh" "$leg_seed/scripts/agents/coverage-delta.sh"
  cp "$root/scripts/agents/pre-commit-guard.sh" "$leg_seed/scripts/agents/pre-commit-guard.sh"
  cp "$root/scripts/agents/sync-transport.sh" "$leg_seed/scripts/agents/sync-transport.sh"
  cp "$source_engine" "$leg_seed/bin/metasystem"
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
git commit "${commit_args[@]}"
git show --no-renames --numstat -z --format= HEAD \
  | "$root/bin/metasystem" gate weight-add --root "$root" \
      --commit "$(git rev-parse --short HEAD)"
SH
  chmod +x "$leg_seed/scripts/agents/land.sh" \
    "$leg_seed/scripts/agents/coverage-delta.sh" \
    "$leg_seed/scripts/agents/pre-commit-guard.sh" \
    "$leg_seed/scripts/agents/commit.sh" \
    "$leg_seed/scripts/agents/sync-transport.sh" \
    "$leg_seed/bin/metasystem"
  printf 'seed\n' >"$leg_seed/payload.txt"
  printf 'existing plan\n' >"$leg_seed/plans/existing.md"
  printf 'artifacts/\n' >"$leg_seed/.gitignore"
  git -C "$leg_seed" init -q
  git -C "$leg_seed" symbolic-ref HEAD refs/heads/main
  git -C "$leg_seed" config user.name fixture
  git -C "$leg_seed" config user.email fixture@example.invalid
  git -C "$leg_seed" add -- scripts bin payload.txt plans/existing.md .gitignore
  git -C "$leg_seed" commit -qm seed
  git init --bare -q "$leg_remote"
  git --git-dir="$leg_remote" symbolic-ref HEAD refs/heads/main
  git -C "$leg_seed" remote add origin "$leg_remote"
  git -C "$leg_seed" push -q -u origin main
  git clone -q "$leg_remote" "$leg_local"
  git clone -q "$leg_remote" "$leg_peer"
  git -C "$leg_local" config user.name fixture-local
  git -C "$leg_local" config user.email fixture-local@example.invalid
  git -C "$leg_peer" config user.name fixture-peer
  git -C "$leg_peer" config user.email fixture-peer@example.invalid
}

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
