#!/usr/bin/env bash
set -euo pipefail

runtime_required=false
file=
while (($#)); do
  case "$1" in
    --file) file=${2:-}; shift 2 ;;
    --runtime-required) runtime_required=true; shift ;;
    *) echo "usage: $0 [--runtime-required] --file <plan.md>" >&2; exit 2 ;;
  esac
done

[[ -n "$file" && -f "$file" ]] || { echo "missing --file plan" >&2; exit 2; }
header='| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |'
grep -Fq "$header" "$file" || { echo "missing obligation matrix header" >&2; exit 1; }

all_rows=$(awk -F'|' '
  /^\|/ {
    sev=$3;
    gsub(/^[[:space:]]+|[[:space:]]+$/, "", sev);
    if (sev == "CRITICAL" || sev == "HIGH" || sev == "MEDIUM" || sev == "LOW") print sev;
  }' "$file")
[[ -n "$all_rows" ]] || { echo "no obligation rows" >&2; exit 1; }

rows=$(awk -F'|' '
  /^\|/ {
    sev=$3; status=$10;
    gsub(/^[[:space:]]+|[[:space:]]+$/, "", sev);
    gsub(/^[[:space:]]+|[[:space:]]+$/, "", status);
    if (sev == "CRITICAL" || sev == "HIGH") print status;
  }' "$file")
if [[ -z "$rows" ]]; then
  echo "design obligation gate passed: no critical/high rows; medium/low rows are reported, not gated"
  exit 0
fi

if grep -Eq '^(MISSING|PARTIAL|CONTRADICTED|BLOCKED)$' <<<"$rows"; then
  echo "critical/high obligations are not ready" >&2
  exit 1
fi
if $runtime_required && grep -Eq '^READY_FOR_RUNTIME$' <<<"$rows"; then
  echo "runtime proof is still pending" >&2
  exit 1
fi
if grep -Evq '^(DONE|READY_FOR_RUNTIME)$' <<<"$rows"; then
  echo "invalid critical/high status" >&2
  exit 1
fi
echo "design obligation gate passed"
