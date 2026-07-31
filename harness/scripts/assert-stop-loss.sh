#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
Usage:
  scripts/assert-stop-loss.sh --file <investigation-ledger.md>

Reads the cycle classifications from an investigation ledger and blocks
further cycles when a machine-checkable stop-loss trigger has fired:

  - any cycle classified falsified-dead-end
  - two or more cycles classified no-progress
  - as many cycles as the declared "Cycle budget:" line (when present)
  - as many trailing cycles without a contract-improved as the declared
    "No-gain budget:" line (when present; improve mode sets 3)

unresolved (a valid measurement inside a declared noise floor) never counts
toward the no-progress trigger; only a declared no-gain budget bounds it.

The judgment triggers (repeating one mechanism family, an expensive run
that taught nothing, no novel fact) stay with the agent and the human.
This check only enforces what the ledger already states, so a ledger
that stops recording classifications also stops being protected.

Run it before contracting a new cycle.
Exit codes: 0 more cycles are allowed; 1 stop-loss triggered; 2 usage error.
USAGE
}

file=
while (($#)); do
  case "$1" in
    --file) file=${2:-}; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) usage; exit 2 ;;
  esac
done
[[ -n "$file" && -f "$file" ]] || { echo "missing --file ledger" >&2; exit 2; }

classifications=$(sed -n 's/^- Classification:[[:space:]]*//p' "$file" \
  | grep -oE 'contract-improved|falsified-continue|falsified-dead-end|no-progress|unresolved|invalid-run' || true)

total_cycles=$(grep -c '^### Cycle' "$file" || true)
dead_ends=$(grep -c 'falsified-dead-end' <<<"$classifications" || true)
no_progress=$(grep -c 'no-progress' <<<"$classifications" || true)
budget=$(sed -n 's/^- Cycle budget:[[:space:]]*//p' "$file" | grep -oE '^[0-9]+' | head -1 || true)
no_gain_budget=$(sed -n 's/^- No-gain budget:[[:space:]]*//p' "$file" | grep -oE '^[0-9]+' | head -1 || true)

if (( dead_ends > 0 )); then
  echo "stop-loss triggered: a cycle was classified falsified-dead-end. Stop, preserve the learning, revert failed behavior, and take the decision up a level" >&2
  exit 1
fi
if (( no_progress >= 2 )); then
  echo "stop-loss triggered: $no_progress cycles classified no-progress. Stop stacking changes and take the decision up a level" >&2
  exit 1
fi
if [[ -n "$budget" ]] && (( total_cycles >= budget )); then
  echo "stop-loss triggered: $total_cycles cycles recorded against a budget of $budget. Stop, or renegotiate the budget with the human" >&2
  exit 1
fi
if [[ -n "$no_gain_budget" ]]; then
  classified=$(grep -c . <<<"$classifications" || true)
  last_gain=$(grep -n '^contract-improved$' <<<"$classifications" | tail -1 | cut -d: -f1 || true)
  if [[ -n "$last_gain" ]]; then
    trailing=$((classified - last_gain))
  else
    trailing=$classified
  fi
  if (( trailing >= no_gain_budget )); then
    echo "stop-loss triggered: $trailing trailing cycles without a contract-improved against a no-gain budget of $no_gain_budget. Stop and hand over the frontier and what was learned" >&2
    exit 1
  fi
fi
echo "stop-loss not triggered: $total_cycles cycles, $no_progress no-progress, budget ${budget:-none}, no-gain budget ${no_gain_budget:-none}"
