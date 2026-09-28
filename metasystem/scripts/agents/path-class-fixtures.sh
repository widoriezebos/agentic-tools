#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
source "$root/scripts/agents/fixture-budget.sh"
tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-path-class.XXXXXX")
tmp=$(cd "$tmp" && pwd -P)
trap 'rm -rf "$tmp"' EXIT

fixture_git() {
  harness_fixture_without_outer_proof env -i PATH="$PATH" HOME="${HOME:-/tmp}" TMPDIR="${TMPDIR:-/tmp}" git "$@"
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

  # Each search takes the directory to search from and the roots to search, so
  # the fixture below can drive the installation search over a planted tree.
  # node_modules joins reviews in both greps: an installed frontend dependency
  # tree whose README or changelog holds one of these tokens would be one grep
  # line and a refused section (g1-s8 revision 5, the exclusion slice). Both
  # read $pattern from this function, which is why they are defined here.
  search_install_readers() { # directory to search from, roots to search
    local from=$1
    shift
    (
      cd "$from"
      grep -rnE -I --exclude-dir=reviews --exclude-dir=node_modules --exclude=journey.md "$pattern" -- "$@"
    )
  }
  search_repo_readers() { # directory to search from, roots to search
    local from=$1
    shift
    (
      cd "$from"
      grep -rnE -I --exclude-dir=node_modules "$pattern" -- "$@"
    )
  }

  # The exclusion carries its own verdict: without it the planted dependency
  # file is a second line and this fixture refuses before the real search. The
  # planted tokens are written by concatenation, as $pattern is, so this script
  # stays clean under its own scan.
  local readers="$tmp/readers" reader_hits reader_lines
  mkdir -p "$readers/cmd" "$readers/internal/x/node_modules/p"
  printf '%s\n' 'neverDirect'Fix >"$readers/cmd/seen.txt"
  printf '%s\n' 'neverDirect'Fix >"$readers/internal/x/node_modules/p/notes.txt"
  set +e
  reader_hits=$(search_install_readers "$readers" cmd internal)
  search_status=$?
  set -e
  reader_lines=$(printf '%s\n' "$reader_hits" | grep -c '' | tr -d ' ')
  if [[ $search_status -ne 0 || "$reader_lines" != 1 || "$reader_hits" != 'cmd/seen.txt:'* ]]; then
    echo "TestDeletedListsHaveNoReader: the reader search no longer excludes an installed dependency tree (status $search_status)" >&2
    printf '%s\n' "$reader_hits" >&2
    exit 1
  fi

  while IFS= read -r install_key; do
    [[ -e "$root/$install_key" || -L "$root/$install_key" ]] && install_paths+=("$install_key")
  done < <(awk '$2 == "behavior" && $1 ~ /^install:/ {sub(/^install:/, "", $1); print $1}' "$root/scripts/agents/path-classes.txt")
  while IFS= read -r repo_key; do
    [[ -e "$root/../$repo_key" || -L "$root/../$repo_key" ]] && repo_paths+=("$repo_key")
  done < <(awk '$2 == "behavior" && $1 ~ /^repo:/ {sub(/^repo:/, "", $1); print $1}' "$root/scripts/agents/path-classes.txt")

  # An empty root list would leave grep reading stdin; the guarded expansion
  # is what bash 3.2 under set -u needs, and the count is the refusal that
  # makes it meaningful.
  ((${#install_paths[@]} > 0)) || {
    echo "TestDeletedListsHaveNoReader: no installation behavior root exists on disk" >&2
    exit 1
  }
  set +e
  search_install_readers "$root" ${install_paths[@]+"${install_paths[@]}"} >"$tmp/deleted-install-readers.out"
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
  search_repo_readers "$root/.." ${repo_paths[@]+"${repo_paths[@]}"} >"$tmp/deleted-repo-readers.out"
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

# The commit boundary's Goal-Item stamping and the landing's goal
# forwarding are the engine's landing path now: internal/landing/landpath
# TestCommit* and TestReproofLandForwardsGoalToEvaluator.
TestPathClassVerbAnswersFromManifest
TestDeletedListsHaveNoReader

echo "path class fixtures: PASSED"
