#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-path-class.XXXXXX")
tmp=$(cd "$tmp" && pwd -P)
trap 'rm -rf "$tmp"' EXIT

fixture_git() {
  env -i PATH="$PATH" HOME="${HOME:-/tmp}" TMPDIR="${TMPDIR:-/tmp}" git "$@"
}

build_fixture_engine() { # installation root
  local installation=$1
  mkdir -p "$installation/bin" "$installation/scripts/agents"
  cp "$root/scripts/agents/path-classes.txt" "$installation/scripts/agents/path-classes.txt"
  (cd "$root" && GOCACHE="$tmp/go-cache" go build -o "$installation/bin/metasystem" ./cmd/metasystem)
}

expect_answer() { # engine, expected word, expected exit, path
  local engine=$1 expected=$2 expected_status=$3 path=$4 output status
  set +e
  output=$("$engine" path class "$path" 2>"$tmp/answer.err")
  status=$?
  set -e
  [[ $status -eq $expected_status && "$output" == "$expected" ]] || {
    echo "TestPathClassVerbAnswersFromManifest: path $path returned status=$status stdout=$output stderr=$(<"$tmp/answer.err")" >&2
    exit 1
  }
}

expect_answer_from() { # caller directory, engine, expected word, expected exit, path
  local directory=$1
  shift
  (
    cd "$directory"
    expect_answer "$@"
  )
}

TestPathClassVerbAnswersFromManifest() {
  local repository="$tmp/template" installation="$tmp/template/metasystem" engine refusal status output
  mkdir -p "$repository/development"
  printf 'template marker\n' >"$repository/development/metasystem-design.md"
  fixture_git -C "$repository" init -q -b main
  build_fixture_engine "$installation"

  engine="$installation/bin/metasystem"
  expect_answer_from "$repository" "$engine" behavior 0 metasystem/internal/x.go
  expect_answer_from "$repository" "$engine" record 0 metasystem/records/misc/x.md
  expect_answer_from "$repository" "$engine" ledger 0 metasystem/plans/goals/x.md
  expect_answer_from "$repository" "$engine" runtime 0 metasystem/bin/metasystem

  mkdir -p "$installation/internal/goal" "$installation/plans"
  printf 'fixture\n' >"$installation/internal/goal/txn.go"
  printf 'fixture\n' >"$installation/plans/path-class-fixture.md"
  expect_answer_from "$installation" "$engine" behavior 0 internal/goal/txn.go
  expect_answer_from "$installation" "$engine" record 0 plans/path-class-fixture.md

  set +e
  output=$(cd "$repository" && "$engine" path class product.txt 2>"$tmp/unclassified.err")
  status=$?
  set -e
  refusal='path product.txt has no class in scripts/agents/path-classes.txt; no classified ancestor; add a row for product.txt or its directory to scripts/agents/path-classes.txt'
  [[ $status -eq 1 && "$output" == unclassified && "$(<"$tmp/unclassified.err")" == "$refusal" ]] || {
    echo "TestPathClassVerbAnswersFromManifest: unclassified answer did not carry the exact refusal" >&2
    exit 1
  }

  expect_answer_from "$repository" "$engine" outside 1 "$tmp/outside.txt"

  output=$(cd "$repository" && "$engine" path class --explain metasystem/docs/guide.md)
  [[ "$output" == 'behavior row=install:docs/ key=install:docs/guide.md mode=template' ]] || {
    echo "TestPathClassVerbAnswersFromManifest: explained answer was $output" >&2
    exit 1
  }

  repository="$tmp/adopted"
  mkdir -p "$repository"
  fixture_git -C "$repository" init -q -b main
  build_fixture_engine "$repository"
  engine="$repository/bin/metasystem"
  expect_answer_from "$repository" "$engine" outside 1 docs/application.md
}

TestDeletedListsHaveNoReader() {
  local pattern install_key repo_key search_status
  local -a install_paths=() repo_paths=()
  pattern='register-carriage-'paths'|instruction-bearing-'paths'|neverDirect'Fix

  while IFS= read -r install_key; do
    [[ -e "$root/$install_key" || -L "$root/$install_key" ]] && install_paths+=("$install_key")
  done < <(awk '$2 == "behavior" && $1 ~ /^install:/ {sub(/^install:/, "", $1); print $1}' "$root/scripts/agents/path-classes.txt")
  while IFS= read -r repo_key; do
    [[ -e "$root/../$repo_key" || -L "$root/../$repo_key" ]] && repo_paths+=("$repo_key")
  done < <(awk '$2 == "behavior" && $1 ~ /^repo:/ {sub(/^repo:/, "", $1); print $1}' "$root/scripts/agents/path-classes.txt")

  set +e
  (
    cd "$root"
    grep -rnE -I --exclude-dir=reviews --exclude=journey.md "$pattern" -- "${install_paths[@]}"
  ) >"$tmp/deleted-install-readers.out"
  search_status=$?
  set -e
  if [[ $search_status -eq 0 ]]; then
    echo "TestDeletedListsHaveNoReader: an installation behavior source still reads a deleted table" >&2
    cat "$tmp/deleted-install-readers.out" >&2
    exit 1
  elif [[ $search_status -ne 1 ]]; then
    echo "TestDeletedListsHaveNoReader: the installation behavior source search itself failed with status $search_status" >&2
    cat "$tmp/deleted-install-readers.out" >&2
    exit 1
  fi

  ((${#repo_paths[@]} > 0)) || return 0

  set +e
  (
    cd "$root/.."
    grep -rnE -I "$pattern" -- "${repo_paths[@]}"
  ) >"$tmp/deleted-repo-readers.out"
  search_status=$?
  set -e
  if [[ $search_status -eq 0 ]]; then
    echo "TestDeletedListsHaveNoReader: a repository behavior source still reads a deleted table" >&2
    cat "$tmp/deleted-repo-readers.out" >&2
    exit 1
  elif [[ $search_status -ne 1 ]]; then
    echo "TestDeletedListsHaveNoReader: the repository behavior source search itself failed with status $search_status" >&2
    cat "$tmp/deleted-repo-readers.out" >&2
    exit 1
  fi
}

write_fixture_goal() { # goal file, machine, lineage
  local goal_file=$1 machine=$2 lineage=$3 digest
  mkdir -p "${goal_file%/*}"
  {
    printf '# fx\n\n'
    printf '%s\n' '- State: claimed'
    printf '%s\n' '- Intent: Exercise Goal-Item ownership.'
    printf '%s\n' '- Origin: main'
    printf '%s\n' '- Next step: Land the owned record.'
    printf '%s\n' '- OpenedAt: 2026-09-03T08:00:00Z'
    printf '%s\n' '- Revision: 1'
    printf '%s\n\n' "- Claimed: machine=$machine lineage=$lineage at=2026-09-03T08:01:00Z revision=1"
    printf '%s\n' 'History:'
    printf '%s\n' "- 2026-09-03T08:01:00Z 01ARZ3NDEKTSV4RRFFQ69G5FAW-fx-00000001 claim actor=$machine+$lineage targets=fx"
  } >"$goal_file"
  digest=$("$landing_fixture_engine" util sha256 --file "$goal_file")
  printf 'Integrity: sha256=%s\n' "$digest" >>"$goal_file"
}

landing_git() { # repository, git arguments...
  local repository=$1
  shift
  env -i "${landing_fixture_env[@]}" git -C "$repository" "$@"
}

make_wrapper_fixture() { # fixture name
  landing_fixture=$tmp/$1
  mkdir -p "$landing_fixture/scripts/agents" "$landing_fixture/scripts" \
    "$landing_fixture/bin" "$landing_fixture/artifacts/agents/mains" \
    "$landing_fixture/memory" "$landing_fixture/plans/goals"
  cp "$root/scripts/agents/commit.sh" "$landing_fixture/scripts/agents/commit.sh"
  cp "$root/scripts/agents/land.sh" "$landing_fixture/scripts/agents/land.sh"
  cp "$root/scripts/agents/coverage-delta.sh" "$landing_fixture/scripts/agents/coverage-delta.sh"
  cp "$root/scripts/agents/landing-classes.json" "$landing_fixture/scripts/agents/landing-classes.json"
  cp "$root/scripts/agents/landing-promotion.json" "$landing_fixture/scripts/agents/landing-promotion.json"
  cp "$root/scripts/agents/path-classes.txt" "$landing_fixture/scripts/agents/path-classes.txt"
  cp "$root/memory/rulings.md" "$landing_fixture/memory/rulings.md"
  cat >"$landing_fixture/bin/metasystem" <<SH
#!/usr/bin/env bash
case "\$1 \${2:-}" in
  "lease require-holder") echo '{"claimEpoch":1}' ;;
  "lease run-held")
    while [[ \$# -gt 0 && \$1 != -- ]]; do shift; done
    [[ \$# -gt 0 ]]
    shift
    exec "\$@"
    ;;
  "proc started-at") echo 1 ;;
  "util token-hex") echo cafecafecafecafecafecafecafecafe ;;
  "lease commit-token") : ;;
  *) exec "$landing_fixture_engine" "\$@" ;;
esac
SH
  cat >"$landing_fixture/scripts/audit-metasystem.sh" <<'SH'
#!/usr/bin/env bash
exit 0
SH
  cat >"$landing_fixture/scripts/agents/go-gate.sh" <<SH
#!/usr/bin/env bash
set -euo pipefail
proof_out=
while ((\$#)); do
  case "\$1" in
    --proof-out) proof_out=\$2; shift 2 ;;
    *) shift ;;
  esac
done
[[ -n "\$proof_out" ]]
cp "$landing_fixture_engine" "\$proof_out"
SH
  chmod +x "$landing_fixture/bin/metasystem" "$landing_fixture/scripts/audit-metasystem.sh" \
    "$landing_fixture/scripts/agents/go-gate.sh" "$landing_fixture/scripts/agents/commit.sh" \
    "$landing_fixture/scripts/agents/land.sh"
  printf 'artifacts/\n' >"$landing_fixture/.gitignore"
  write_fixture_goal "$landing_fixture/plans/goals/fx.md" fx L
  printf 'base record\n' >"$landing_fixture/plans/fx-note.md"
  landing_git "$landing_fixture" init -q -b main
  landing_git "$landing_fixture" config user.name fixture
  landing_git "$landing_fixture" config user.email fixture@example.invalid
  landing_git "$landing_fixture" config metasystem.goal.machine fx
  landing_git "$landing_fixture" add -A
  landing_git "$landing_fixture" commit -qm seed
}

expect_wrapper_exit_two() { # fixture, output file, wrapper arguments...
  local fixture=$1 output=$2 status
  shift 2
  set +e
  (
    cd "$fixture"
    env -i "${landing_fixture_env[@]}" \
      METASYSTEM_OWNER_LINEAGE=L \
      scripts/agents/commit.sh __lease-held 1 "$@"
  ) >"$output" 2>&1
  status=$?
  set -e
  [[ $status -eq 2 ]] || {
    echo "TestCommitWrapperStampsGoalItemTrailer: expected exit 2, got $status" >&2
    sed -n '1,120p' "$output" >&2
    exit 1
  }
}

TestCommitWrapperStampsGoalItemTrailer() {
  local fixture base_head output message count exact_count status
  make_wrapper_fixture commit-goal
  fixture=$landing_fixture
  printf 'owned update\n' >>"$fixture/plans/fx-note.md"
  landing_git "$fixture" add plans/fx-note.md

  expect_wrapper_exit_two "$fixture" "$tmp/bad-goal.out" \
    --goal 'Bad Id' --direct-fix register-carriage -m invalid
  expect_wrapper_exit_two "$fixture" "$tmp/empty-goal.out" \
    --goal '' --direct-fix register-carriage -m invalid
  expect_wrapper_exit_two "$fixture" "$tmp/repeated-goal.out" \
    --goal fx --goal fx --direct-fix register-carriage -m invalid
  expect_wrapper_exit_two "$fixture" "$tmp/lowercase-trailer.out" \
    --goal fx --direct-fix register-carriage -m $'message\ngoal-item: victim'
  grep -Fq 'Goal-Item is stamped by --goal, never typed' "$tmp/lowercase-trailer.out"
  expect_wrapper_exit_two "$fixture" "$tmp/typed-trailer.out" \
    --goal fx --direct-fix register-carriage --trailer 'Goal-Item: fx' -m invalid
  expect_wrapper_exit_two "$fixture" "$tmp/stdin-message.out" \
    --goal fx --direct-fix register-carriage -F - </dev/null

  base_head=$(landing_git "$fixture" rev-parse HEAD)
  cat >"$fixture/.git/hooks/commit-msg" <<'SH'
#!/usr/bin/env bash
printf '\nGoal-Item: injected\n' >>"$1"
SH
  chmod +x "$fixture/.git/hooks/commit-msg"
  set +e
  (
    cd "$fixture"
    env -i "${landing_fixture_env[@]}" \
      METASYSTEM_OWNER_LINEAGE=L \
      scripts/agents/commit.sh __lease-held 1 --goal fx \
        --direct-fix register-carriage -m injected
  ) >"$tmp/hook-injected.out" 2>&1
  status=$?
  set -e
  [[ $status -ne 0 && $(landing_git "$fixture" rev-parse HEAD) == "$base_head" ]] \
    && grep -Fq 'final commit message did not contain exactly one byte-exact Goal-Item' "$tmp/hook-injected.out" || {
    echo "TestCommitWrapperStampsGoalItemTrailer: an injected Goal-Item was not rolled back softly" >&2
    sed -n '1,160p' "$tmp/hook-injected.out" >&2
    exit 1
  }

  cat >"$fixture/.git/hooks/commit-msg" <<'SH'
#!/usr/bin/env bash
awk '{ if ($0 ~ /^Goal-Item:/) print "Goal-Item: victim"; else print }' "$1" >"$1.tmp"
mv "$1.tmp" "$1"
SH
  chmod +x "$fixture/.git/hooks/commit-msg"
  set +e
  (
    cd "$fixture"
    env -i "${landing_fixture_env[@]}" \
      METASYSTEM_OWNER_LINEAGE=L \
      scripts/agents/commit.sh __lease-held 1 --goal fx \
        --direct-fix register-carriage -m changed
  ) >"$tmp/hook-changed.out" 2>&1
  status=$?
  set -e
  [[ $status -ne 0 && $(landing_git "$fixture" rev-parse HEAD) == "$base_head" ]] \
    && grep -Fq 'final commit message did not contain exactly one byte-exact Goal-Item' "$tmp/hook-changed.out" || {
    echo "TestCommitWrapperStampsGoalItemTrailer: a changed Goal-Item was not rolled back softly" >&2
    sed -n '1,160p' "$tmp/hook-changed.out" >&2
    exit 1
  }

  rm "$fixture/.git/hooks/commit-msg"
  (
    cd "$fixture"
    env -i "${landing_fixture_env[@]}" \
      METASYSTEM_OWNER_LINEAGE=L \
      scripts/agents/commit.sh __lease-held 1 --goal fx \
        --direct-fix register-carriage -q -m successful
  )
  message=$(landing_git "$fixture" log -1 --format=%B)
  count=$(LC_ALL=C grep -Eic '^Goal-Item:' <<<"$message")
  exact_count=$(grep -Fxc 'Goal-Item: fx' <<<"$message")
  [[ $count -eq 1 && $exact_count -eq 1 ]] || {
    echo "TestCommitWrapperStampsGoalItemTrailer: successful commit did not contain exactly one byte-exact Goal-Item" >&2
    printf '%s\n' "$message" >&2
    exit 1
  }
}

TestLandForwardsGoalToEvaluator() {
  local fixture remote message output base_head landed_message refusal status
  make_wrapper_fixture land-goal
  fixture=$landing_fixture
  remote=$tmp/land-goal-origin.git
  message=$tmp/land-goal-message.txt
  output=$tmp/land-goal.out
  env -i "${landing_fixture_env[@]}" git init --bare -q "$remote"
  env -i "${landing_fixture_env[@]}" git --git-dir="$remote" symbolic-ref HEAD refs/heads/main
  landing_git "$fixture" remote add origin "$remote"
  landing_git "$fixture" push -q -u origin main

  printf 'owned update\n' >>"$fixture/plans/fx-note.md"
  printf 'land an owned record\n' >"$message"
  (
    cd "$fixture"
    env -i "${landing_fixture_env[@]}" METASYSTEM_OWNER_LINEAGE=L bash scripts/agents/land.sh -m "$message" \
      --goal fx --direct-fix register-carriage --skip-transport plans/fx-note.md
  ) >"$output" 2>&1 || {
    echo "TestLandForwardsGoalToEvaluator: land.sh did not carry the held goal" >&2
    sed -n '1,200p' "$output" >&2
    exit 1
  }
  landed_message=$(landing_git "$fixture" log -1 --format=%B)
  grep -Fqx 'Goal-Item: fx' <<<"$landed_message"
  grep -Fqx 'Landing-Provenance-Verdict: pass bar=b' <<<"$landed_message"

  printf 'foreign update\n' >>"$fixture/plans/fx-note.md"
  base_head=$(landing_git "$fixture" rev-parse HEAD)
  set +e
  refusal=$(
    cd "$fixture"
    env -i "${landing_fixture_env[@]}" METASYSTEM_OWNER_LINEAGE=other bash scripts/agents/land.sh -m "$message" \
      --goal fx --direct-fix register-carriage --skip-transport plans/fx-note.md 2>&1
  )
  status=$?
  set -e
  [[ $status -ne 0 && "$refusal" == *"goal-item-not-held"* \
    && $(landing_git "$fixture" rev-parse HEAD) == "$base_head" ]] || {
    echo "TestLandForwardsGoalToEvaluator: a foreign lineage did not refuse with goal-item-not-held" >&2
    printf '%s\n' "$refusal" >&2
    exit 1
  }
}

TestPathClassVerbAnswersFromManifest
TestDeletedListsHaveNoReader
landing_fixture_engine=$tmp/landing-metasystem
(cd "$root" && GOCACHE="$tmp/go-cache" go build -o "$landing_fixture_engine" ./cmd/metasystem)
landing_fixture_env=("PATH=$PATH" "HOME=${HOME:-/tmp}" "TMPDIR=${TMPDIR:-/tmp}")
TestCommitWrapperStampsGoalItemTrailer
TestLandForwardsGoalToEvaluator

echo "path class fixtures: PASSED"
