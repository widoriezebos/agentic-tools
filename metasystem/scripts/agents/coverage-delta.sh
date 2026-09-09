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

engine=${METASYSTEM_BIN:-$root/bin/metasystem}
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

coverage_reuse_args=(proof-run coverage-reuse --root "$root" --baseline "$ratchet")
for package in "${normalized[@]}"; do
  coverage_reuse_args+=(--package "$package")
done
coverage_reuse_rc=0
coverage_reuse_output=$(GOFLAGS=-mod=readonly "$engine" "${coverage_reuse_args[@]}" 2>&1) || coverage_reuse_rc=$?
if [[ $coverage_reuse_rc -eq 0 ]]; then
  printf '%s\n' "$coverage_reuse_output"
  echo "coverage delta: reused authenticated full-gate measurements; no coverage test launched"
  exit 0
fi
if [[ $coverage_reuse_rc -ne 3 ]]; then
  printf '%s\n' "$coverage_reuse_output" >&2
  echo "coverage delta: retained coverage authority was unreadable" >&2
  exit 1
fi

# A cache miss is one admitted affected-package proof. A child already inside
# that proof is authenticated by the Go owner and performs the existing test
# loop; ambient context strings cannot select the worker path.
coverage_proof_worker=0
coverage_auth_bin=${METASYSTEM_PROOF_AUTH_BIN:-$engine}
if [[ -x "$coverage_auth_bin" ]] && "$coverage_auth_bin" proof-run worker-authorized --root "$root" >/dev/null 2>&1; then
  coverage_proof_worker=1
fi
if [[ $coverage_proof_worker -ne 1 ]]; then
  coverage_proof_run="$(date -u +%Y%m%dT%H%M%SZ)-$$-$RANDOM"
  coverage_proof_progress="$root/artifacts/agents/supervision/coverage-delta-$coverage_proof_run.progress.jsonl"
  coverage_proof_log="$root/artifacts/agents/supervision/suite-logs/coverage-delta-$coverage_proof_run.log"
  coverage_proof_banner=$(
    "$engine" proof-run banner --suite coverage-delta --root "$root" \
      --progress "$coverage_proof_progress" --log "$coverage_proof_log"
  ) || { echo "coverage delta: could not prepare admitted coverage fallback" >&2; exit 1; }
  coverage_ratchet_digest=$("$engine" util sha256 --file "$ratchet") \
    || { echo "coverage delta: could not bind the selected ratchet" >&2; exit 1; }
  coverage_proof_args=(proof-run launch --suite coverage-delta --root "$root" --conf "$root/metasystem.conf" \
    --progress "$coverage_proof_progress" --log "$coverage_proof_log" --banner "$coverage_proof_banner" \
    --scope coverage --command-class coverage-delta --identity-input "coverage-ratchet=$coverage_ratchet_digest")
  for package in "${normalized[@]}"; do
    coverage_proof_args+=(--identity-input "coverage-package=$package")
  done
  exec "$engine" "${coverage_proof_args[@]}" -- \
    bash "$root/scripts/agents/coverage-delta.sh" --ratchet "$ratchet" -- "${normalized[@]}"
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

  # This is a hang bound for one package without the race detector, leaving a
  # wide margin above the slowest package. A timed-out coverage probe reads as
  # a package failure with a phantom partial percentage.
  test_output=$(go test -cover -timeout 30m "$test_package" 2>&1)
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
