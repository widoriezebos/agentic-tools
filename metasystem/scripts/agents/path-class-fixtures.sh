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
    rg -n --glob '!docs/reviews/**' --glob '!docs/journey.md' "$pattern" -- "${install_paths[@]}"
  ) >"$tmp/deleted-install-readers.out"
  search_status=$?
  set -e
  [[ $search_status -eq 1 ]] || {
    echo "TestDeletedListsHaveNoReader: an installation behavior source still reads a deleted table" >&2
    cat "$tmp/deleted-install-readers.out" >&2
    exit 1
  }

  ((${#repo_paths[@]} > 0)) || return 0

  set +e
  (
    cd "$root/.."
    rg -n "$pattern" -- "${repo_paths[@]}"
  ) >"$tmp/deleted-repo-readers.out"
  search_status=$?
  set -e
  [[ $search_status -eq 1 ]] || {
    echo "TestDeletedListsHaveNoReader: a repository behavior source still reads a deleted table" >&2
    cat "$tmp/deleted-repo-readers.out" >&2
    exit 1
  }
}

TestPathClassVerbAnswersFromManifest
TestDeletedListsHaveNoReader

echo "path class fixtures: PASSED"
