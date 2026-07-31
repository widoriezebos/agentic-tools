#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/assert-design-obligation-gate.sh --file <plan.md> [--file <plan.md>...]
  scripts/assert-design-obligation-gate.sh --runtime-required --file <plan.md>...

Checks the structure and declared state of design-obligation matrices.

Required table header:
| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |

By default, CRITICAL/HIGH obligations must be DONE or READY_FOR_RUNTIME.
With --runtime-required, CRITICAL/HIGH obligations must be DONE.

Proof cells on CRITICAL/HIGH rows must be concrete: a backticked token, a
path-shaped token (a slash or a known file extension), or "Not applicable"
followed by a reason. Bare "Not applicable" fails, and so does keyword-only
prose ("needs testing"): a status is only as trustworthy as the proof behind
it. Owner cells on CRITICAL/HIGH rows need a backticked, dotted, slashed,
double-colon, or CamelCase code token; plain prose fails.

Matrix rows inside fenced code blocks are ignored, so documentation that shows
the template does not satisfy the gate. Table cells must not contain literal
pipe characters; the column parser cannot see an escaped pipe as content.
USAGE
}

FILES=()
RUNTIME_REQUIRED=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --file)
      if [[ $# -lt 2 ]]; then
        echo "missing value for --file" >&2
        exit 2
      fi
      FILES+=("$2")
      shift 2
      ;;
    --runtime-required)
      RUNTIME_REQUIRED=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage
      exit 2
      ;;
  esac
done

if [[ ${#FILES[@]} -eq 0 ]]; then
  echo "at least one --file is required" >&2
  usage
  exit 2
fi

FAILURES=0

for file in "${FILES[@]}"; do
  file_path="$file"
  if [[ "$file" != /* && ! -r "$file_path" ]]; then
    file_path="$ROOT_DIR/$file"
  fi
  if [[ ! -r "$file_path" ]]; then
    echo "missing or unreadable obligation file: $file" >&2
    FAILURES=$((FAILURES + 1))
    continue
  fi

  if ! OUTPUT="$(awk -v runtime_required="$RUNTIME_REQUIRED" -v file="$file" '
    function trim(value) {
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", value)
      return value
    }
    function upper(value) {
      return toupper(trim(value))
    }
    function lower(value) {
      return tolower(trim(value))
    }
    function weak(value) {
      value = upper(value)
      return value == "" || value == "NONE" || value == "MISSING" || value == "TBD" || value == "TODO" || value == "UNKNOWN"
    }
    function concrete(value) {
      original = trim(value)
      normalized = lower(original)
      if (weak(original)) {
        return 0
      }
      if (normalized ~ /(not applicable|no runtime proof required|runtime proof not required)/) {
        if (normalized ~ /(not applicable|no runtime proof required|runtime proof not required)[^a-z0-9]+[a-z0-9]/) {
          return 1
        }
        return 0
      }
      if (original ~ /`[^`]+`/) {
        return 1
      }
      if (original ~ /[[:alnum:]_\/.-]+\.(java|kt|scala|ts|tsx|js|jsx|py|go|rs|rb|cs|cpp|c|h|sql|md|json|xml|yml|yaml|sh|log)/) {
        return 1
      }
      if (original ~ /[[:alnum:]_.-]+\/[[:alnum:]_\/.-]+/) {
        return 1
      }
      return 0
    }
    function owner_ok(value) {
      original = trim(value)
      if (weak(original)) {
        return 0
      }
      if (original ~ /`[^`]+`/) {
        return 1
      }
      if (original ~ /[[:alnum:]_]+\.[[:alnum:]_]+/) {
        return 1
      }
      if (original ~ /[[:alnum:]_.-]+\/[[:alnum:]_\/.-]+/) {
        return 1
      }
      if (original ~ /::/) {
        return 1
      }
      if (original ~ /[A-Z][a-z0-9]+[A-Z]/) {
        return 1
      }
      return 0
    }
    function add_failure(message) {
      failures++
      print file ":" FNR ": " message
    }
    BEGIN {
      in_table = 0
      in_code = 0
      rows = 0
      failures = 0
    }
    /^[[:space:]]*```/ {
      in_code = !in_code
      in_table = 0
      next
    }
    in_code {
      next
    }
    /^\|[[:space:]]*Obligation id[[:space:]]*\|[[:space:]]*Severity[[:space:]]*\|/ {
      if ($0 ~ /\|[[:space:]]*Status[[:space:]]*\|[[:space:]]*Next action[[:space:]]*\|[[:space:]]*$/) {
        in_table = 1
      }
      next
    }
    in_table && /^\|[[:space:]-]+\|/ {
      next
    }
    in_table && $0 !~ /^\|/ {
      in_table = 0
      next
    }
    in_table {
      split($0, columns, "|")
      obligation_id = trim(columns[2])
      severity = upper(columns[3])
      owner = trim(columns[6])
      code_proof = trim(columns[7])
      test_proof = trim(columns[8])
      runtime_proof = trim(columns[9])
      status = upper(columns[10])
      if (obligation_id == "" || obligation_id == "Obligation id") {
        next
      }
      rows++
      if (severity != "CRITICAL" && severity != "HIGH" && severity != "MEDIUM" && severity != "LOW") {
        add_failure(obligation_id " has invalid severity: " severity)
      }
      if (status != "DONE" && status != "READY_FOR_RUNTIME" && status != "PARTIAL" && status != "MISSING" && status != "CONTRADICTED" && status != "BLOCKED") {
        add_failure(obligation_id " has invalid status: " status)
      }
      if (severity == "CRITICAL" || severity == "HIGH") {
        if (status == "MISSING" || status == "PARTIAL" || status == "CONTRADICTED") {
          add_failure(obligation_id " blocks completion: " severity " " status)
        }
        if (status == "READY_FOR_RUNTIME") {
          if (runtime_required == 1) {
            add_failure(obligation_id " still needs runtime proof before this gate: " status)
          }
          if (!owner_ok(owner) || !concrete(code_proof) || !concrete(test_proof)) {
            add_failure(obligation_id " cannot be READY_FOR_RUNTIME without an owner, concrete code proof, and concrete test proof")
          }
        }
        if (status == "DONE" && (!owner_ok(owner) || !concrete(code_proof) || !concrete(test_proof) || !concrete(runtime_proof))) {
          add_failure(obligation_id " cannot be DONE without an owner and concrete code, test, and runtime proof")
        }
        if (status == "BLOCKED") {
          add_failure(obligation_id " is BLOCKED and needs the named external decision before this gate")
        }
      }
    }
    END {
      if (rows == 0) {
        add_failure("no design-obligation rows found")
      }
      exit failures > 0 ? 1 : 0
    }
  ' "$file_path")"; then
    echo "$OUTPUT" >&2
    FAILURES=$((FAILURES + 1))
  fi
done

if (( FAILURES > 0 )); then
  echo "design obligation gate failed" >&2
  exit 1
fi

echo "design obligation gate passed (${#FILES[@]} file(s), runtime_required=$RUNTIME_REQUIRED)"
