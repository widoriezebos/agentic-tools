#!/usr/bin/env bash
# Regression proofs for the control-plane authority surface that outlived the
# python port. The full authority MATRIX (holder/advisor/delegate/supervision/
# adapter/human across every mode) moved into internal/authority's unit tests
# under the go gate; re-proving each cell here would only re-test the same Go
# function through a slower pipe. What stays behavioral and cross-boundary —
# and therefore stays HERE — is:
#   WC-1  the CLI refuses a positive DELEGATE classification even when the
#         caller supplies the implementation's retired bypass marker;
#   the retired METASYSTEM_LEASE_FENCE marker stays out of the shell sources;
#   WC-3  dispatch --wait re-enters reaping only through the lease-held verb;
#   WC-9  interval reaping authenticates supervision before its destructive
#         loop, with the start gate letting arming publish custody first.
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"
[[ -x "$ms" ]] || { echo "authority regression fixtures: binary absent; run the go gate first" >&2; exit 1; }

# WC-1: even a caller that supplies the implementation's retired marker cannot
# bypass a positive delegate classification.
retired_marker="METASYSTEM""_LEASE_FENCE"
set +e
wc1_err=$(env "$retired_marker=fixture" "$ms" authority check \
  --mode record-writer \
  --classification '{"class":"DELEGATE","holder":false}' \
  --job foreign-job 2>&1)
wc1_status=$?
set -e
[[ $wc1_status -ne 0 ]] \
  || { echo "authority regression: retired marker bypassed a DELEGATE refusal" >&2; exit 1; }
grep -Fq DELEGATE <<<"$wc1_err" \
  || { echo "authority regression: refusal did not name the caller class" >&2; exit 1; }

# The retired marker must not reappear in the shell control-plane sources.
for source in scripts/agents/dispatch.sh scripts/agents/commit.sh scripts/agents/evidence-gc.sh; do
  if grep -Fq "$retired_marker" "$root/$source"; then
    echo "authority regression: retired marker resurfaced in $source" >&2
    exit 1
  fi
done

dispatch="$root/scripts/agents/dispatch.sh"

# WC-3: both terminal collection and live reaping in --wait re-enter through
# the lease-held verb; no bare reap remains in the wait loop.
wait_body=$(awk '/^wait_for_job\(\)/{flag=1} /^aggregate_chain_usage\(\)/{flag=0} flag' "$dispatch")
[[ $(grep -o 'lease_run_held' <<<"$wait_body" | wc -l | tr -d ' ') -eq 2 ]] \
  || { echo "authority regression: wait loop does not re-enter the lease-held verb exactly twice" >&2; exit 1; }
grep -Fq '__reap-held' <<<"$wait_body" \
  || { echo "authority regression: wait loop lost its lease-held reap re-entry" >&2; exit 1; }
if grep -Fq 'reap_one "$job"' <<<"$wait_body"; then
  echo "authority regression: a bare reap returned to the wait loop" >&2
  exit 1
fi

# WC-9: interval reaping authenticates supervision before it enters its
# destructive loop, while the start gate lets arming publish custody first.
reap_body=$(awk '/^reap_jobs\(\)/{flag=1} /^internal_register_custody\(\)/{flag=0} flag' "$dispatch")
grep -Fq 'start_gate' <<<"$reap_body" \
  || { echo "authority regression: reap_jobs lost its start gate" >&2; exit 1; }
authority_line=$(grep -Fn 'internal_authority supervision-only' <<<"$reap_body" | head -1 | cut -d: -f1)
loop_line=$(grep -Fn 'while true' <<<"$reap_body" | head -1 | cut -d: -f1)
[[ -n "$authority_line" && -n "$loop_line" && "$authority_line" -lt "$loop_line" ]] \
  || { echo "authority regression: reap_jobs enters its loop before authenticating supervision" >&2; exit 1; }

echo "authority regression fixtures: PASSED"
