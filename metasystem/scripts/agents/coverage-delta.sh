#!/usr/bin/env bash
# Check coverage only for the packages named by a landing or touched since a base.
set -o pipefail
export LC_ALL=C

usage() {
  echo "Usage: scripts/agents/coverage-delta.sh [--base <ref> | --staged | <package> ...] [--ratchet <path>]" >&2
}

invocation_dir=$PWD
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
cd "$root" || exit 1

base=
staged=0
ratchet=
packages=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --base)
      [[ $# -ge 2 ]] || { usage; exit 2; }
      base=$2
      shift 2
      ;;
    --ratchet)
      [[ $# -ge 2 ]] || { usage; exit 2; }
      ratchet=$2
      shift 2
      ;;
    --staged)
      staged=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    --)
      shift
      while [[ $# -gt 0 ]]; do
        packages[${#packages[@]}]=$1
        shift
      done
      ;;
    -*)
      echo "coverage delta: unknown option: $1" >&2
      usage
      exit 2
      ;;
    *)
      packages[${#packages[@]}]=$1
      shift
      ;;
  esac
done

selection_count=0
[[ -n "$base" ]] && selection_count=$((selection_count + 1))
(( staged )) && selection_count=$((selection_count + 1))
[[ ${#packages[@]} -gt 0 ]] && selection_count=$((selection_count + 1))
if (( selection_count > 1 )); then
  echo "coverage delta: --base, --staged, and package arguments are mutually exclusive" >&2
  usage
  exit 2
fi
if (( selection_count == 0 )); then
  usage
  exit 2
fi

# Staged coverage is a law of roots that carry the canonical registry. Fixture
# and adopted roots without that registry have no local floor to enforce.
if (( staged )) && [[ ! -e "$root/scripts/agents/coverage-ratchet.json" ]]; then
  echo "coverage delta: no ratchet registry at this root; skipped"
  exit 0
fi

if [[ -n "$base" || $staged -eq 1 ]]; then
  if (( staged )); then
    diff_files=$(git diff --cached --name-only --relative -- '*.go')
  else
    diff_files=$(git diff --name-only --relative "$base" -- '*.go')
  fi
  diff_rc=$?
  if [[ $diff_rc -ne 0 ]]; then
    if (( staged )); then
      echo "coverage delta: could not derive packages from the staged diff" >&2
    else
      echo "coverage delta: could not derive packages from base $base" >&2
    fi
    exit 1
  fi
  packages=()
  while IFS= read -r changed_file; do
    [[ -n "$changed_file" ]] || continue
    packages[${#packages[@]}]=$(dirname "$changed_file")
  done <<< "$diff_files"
fi

if (( staged )) && [[ ${#packages[@]} -eq 0 ]]; then
  echo "coverage delta: no Go files staged; skipped"
  exit 0
fi

if [[ -z "$ratchet" ]]; then
  ratchet="$root/scripts/agents/coverage-ratchet.json"
  if [[ "$(uname -s)" == Linux ]]; then
    ratchet="$root/scripts/agents/coverage-ratchet-linux.json"
  fi
elif [[ "$ratchet" != /* ]]; then
  ratchet="$invocation_dir/$ratchet"
fi

engine="$root/bin/metasystem"
if [[ ! -x "$engine" ]]; then
  echo "coverage delta: engine is unavailable at $engine" >&2
  exit 1
fi
if [[ ! -f "$ratchet" ]]; then
  echo "coverage delta: ratchet file is unavailable: $ratchet" >&2
  exit 1
fi
floors=$(
  "$engine" json get --file "$ratchet" --field floors 2>/dev/null
)
if [[ $? -ne 0 || "$floors" != \{*\} || "$floors" == "{}" ]]; then
  echo "coverage delta: ratchet floors are unreadable: $ratchet" >&2
  exit 1
fi

module=$(awk '$1 == "module" { print $2; exit }' go.mod)
if [[ -z "$module" ]]; then
  echo "coverage delta: go.mod does not name a module" >&2
  exit 1
fi

# Normalize the spellings used by Go to the relative keys used by the ratchet.
normalized=()
for package in "${packages[@]}"; do
  case "$package" in
    "$module"/*) package=${package#"$module"/} ;;
  esac
  while [[ "$package" == ./* ]]; do
    package=${package#./}
  done
  [[ "$package" == "." ]] || package=${package%/}
  if [[ -z "$package" || "$package" == *...* ]]; then
    echo "coverage delta: expected one concrete package, got: $package" >&2
    exit 2
  fi
  duplicate=0
  for existing in "${normalized[@]}"; do
    if [[ "$existing" == "$package" ]]; then
      duplicate=1
      break
    fi
  done
  [[ $duplicate -eq 1 ]] || normalized[${#normalized[@]}]=$package
done

if [[ ${#normalized[@]} -eq 0 ]]; then
  echo "coverage delta: no Go packages selected"
  exit 0
fi

below=()
test_failures=()
missing_floor=__coverage_delta_no_floor__

for package in "${normalized[@]}"; do
  display=$package
  test_package=$package
  if [[ "$package" != "." ]]; then
    display="./$package"
    test_package="./$package"
  fi

  floor=$(
    "$engine" json get --file "$ratchet" --field "floors.$package" --default "$missing_floor"
  )
  floor_rc=$?
  if [[ $floor_rc -ne 0 ]]; then
    echo "coverage delta: could not read the floor for $display from $ratchet" >&2
    exit 1
  fi
  if [[ "$floor" == "$missing_floor" ]]; then
    echo "coverage delta: $display: no floor registered"
    continue
  fi
  if [[ ! "$floor" =~ ^[0-9]+([.][0-9]+)?$ ]]; then
    echo "coverage delta: invalid floor for $display: $floor" >&2
    exit 1
  fi
  floor_display=$(awk -v floor="$floor" 'BEGIN { printf "%.1f", floor }')

  test_output=$(go test -cover "$test_package" 2>&1)
  test_rc=$?
  measured=$(printf '%s\n' "$test_output" \
    | sed -n 's/.*coverage: \([0-9][0-9.]*\)% of statements.*/\1/p' \
    | tail -n 1)

  if [[ $test_rc -ne 0 ]]; then
    test_failures[${#test_failures[@]}]="$display (go test exited $test_rc)"
    printf '%s\n' "$test_output" >&2
  fi
  if [[ -z "$measured" ]]; then
    if [[ $test_rc -eq 0 ]]; then
      test_failures[${#test_failures[@]}]="$display (no coverage result)"
    fi
    continue
  fi

  if awk -v measured="$measured" -v floor="$floor" 'BEGIN { exit !(measured < floor) }'; then
    below[${#below[@]}]="$display: measured ${measured}%, floor ${floor_display}%"
  else
    echo "coverage delta: $display: ${measured}% (floor ${floor_display}%)"
  fi
done

if [[ ${#below[@]} -gt 0 ]]; then
  echo "coverage delta: packages below floor:" >&2
  for finding in "${below[@]}"; do
    echo "  $finding" >&2
  done
fi
if [[ ${#test_failures[@]} -gt 0 ]]; then
  echo "coverage delta: package test failures:" >&2
  for failure in "${test_failures[@]}"; do
    echo "  $failure" >&2
  done
fi

if [[ ${#below[@]} -gt 0 || ${#test_failures[@]} -gt 0 ]]; then
  exit 1
fi

echo "coverage delta: passed (${#normalized[@]} package(s) considered)"
