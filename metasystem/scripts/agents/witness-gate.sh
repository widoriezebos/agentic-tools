# The witness-producing gate (D33), extracted so every battery stage
# shares ONE proof of the tree instead of re-running the race suite
# per nested validation. SOURCED, never executed: the point is the
# exported witness in the caller's environment.
#
# Contract: the caller sets $root (the metasystem root, already cwd),
# $delivery_contract (0/1), and WITNESS_GATE_FALLBACK as an EXPLICIT
# choice — "plain" runs the ordinary go-gate when the witness is
# ineligible before a gate starts; an executed gate is never retried;
# "none" arms nothing and runs nothing, for callers whose nested
# validations carry their own gates. Any other value refuses: the
# fallback decides whether a gate runs at all, so ambient or mistyped
# state must die loudly, never silently suppress it. On success it
# exports
# from the proven snapshot, and leaves $witness_state set — the CALLER
# force, and delivery runs remain ineligible. A witness that existed at entry
# is consumed or refused; it is never replaced in place.

# The fallback is an EXPLICIT choice: unset and empty refuse exactly
# like a typo, and the message avoids every bash-4 substitution — this
# checkout resolves bash 3.2, where ${VAR@Q} is itself a fatal error.
case "${WITNESS_GATE_FALLBACK:-}" in
  plain|none) ;;
  *)
    echo "witness-gate refused: WITNESS_GATE_FALLBACK must be set to plain or none (got '${WITNESS_GATE_FALLBACK:-unset}')" >&2
    return 1 2>/dev/null || exit 1
    ;;
esac
witness_state=
witness_engine_reused=0
witness_input=${METASYSTEM_GATE_WITNESS:-}
if [[ -n "$witness_input" ]]; then
  witness_reuse_marker=$(mktemp)
  witness_consumer_rc=0
  witness_can_run=1
  if [[ "${WITNESS_GATE_FALLBACK:-plain}" == none ]]; then
    # A no-fallback caller probes first: a refusal must run nothing. Plain
    # fallback deliberately skips this probe so one consumer freeze can either
    # reuse the witness or perform the complete gate from that same export.
    if ! METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE=ENGINE \
        bash scripts/agents/go-gate.sh --witness-check-only >/dev/null 2>&1; then
      witness_can_run=0
    fi
  fi
  if (( witness_can_run )); then
    METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE=ENGINE \
      METASYSTEM_GATE_WITNESS_REUSE_OUT="$witness_reuse_marker" \
      bash scripts/agents/go-gate.sh || witness_consumer_rc=$?
    if (( witness_consumer_rc == 0 )) && [[ -s "$witness_reuse_marker" ]]; then
      witness_engine_reused=1
    fi
  fi
  rm -f "$witness_reuse_marker"
  if (( ! witness_engine_reused )); then
    # A refused inherited witness cannot become acceptable later in this root
    # and silently change its run class. The handoff is scrubbed after either
    # an explicit no-fallback refusal or a complete frozen fallback proof.
    unset METASYSTEM_GATE_WITNESS METASYSTEM_GATE_WITNESS_ROOT \
      METASYSTEM_GATE_WITNESS_RUN METASYSTEM_GATE_WITNESS_EXPORT
  fi
  if (( witness_consumer_rc != 0 )); then
    return "$witness_consumer_rc" 2>/dev/null || exit "$witness_consumer_rc"
  fi
  return 0 2>/dev/null || exit 0
fi
witness_common_eligible=0
if (( ! ${delivery_contract:-0} )) \
  && [[ "${METASYSTEM_COVERAGE_RATCHET_SEED:-0}" != 1 && "${METASYSTEM_GATE_FORCE:-0}" != 1 ]]; then
  witness_common_eligible=1
fi

if (( witness_common_eligible )); then
  if [[ " ${GOFLAGS:-} " =~ [[:space:]]-(modfile|overlay)(=|[[:space:]]) ]]; then
    echo "witness gate arming voided: GOFLAGS may not contain -modfile or -overlay" >&2
    return 1 2>/dev/null || exit 1
  fi
  witness_prepared=1
  witness_freeze_output=
  if ! witness_freeze_output=$(go run ./cmd/metasystem gate witness-freeze --root "$root"); then
    witness_prepared=0
  fi
  witness_freeze_snapshot=
  IFS=$'\t' read -r witness_manifest_digest witness_snap witness_freeze_snapshot <<<"$witness_freeze_output"
  if [[ ! "$witness_manifest_digest" =~ ^[a-f0-9]{64}$ || ! -d "$witness_snap" || ! -d "$witness_freeze_snapshot" ]]; then
    [[ -z "$witness_freeze_snapshot" ]] \
      || go run ./cmd/metasystem gate witness-freeze --cleanup "$witness_freeze_snapshot" >/dev/null 2>&1 \
      || true
    witness_prepared=0
  fi
  if (( witness_prepared )); then
    if ! witness_state=$(mktemp -d) || ! chmod 700 "$witness_state"; then
      witness_prepared=0
    fi
  fi
  witness_run="run-$$-$RANDOM"
  witness_controller_pid=$$
  witness_controller_started_at=
  witness_controller_start_ticks=
  witness_controller_boot_id=
  if (( witness_prepared )); then
    if read -r witness_controller_started_at witness_controller_start_ticks witness_controller_boot_id \
        < <(go run ./cmd/metasystem proc started-at --pid $$ --emit pair); then
      [[ "$witness_controller_boot_id" == - ]] && witness_controller_boot_id=
    else
      witness_prepared=0
    fi
  fi
  if (( ! witness_prepared )); then
    echo "witness gate preparation did not complete" >&2
    [[ -z "$witness_state" ]] || rm -rf "$witness_state" || true
    witness_state=
    [[ -z "$witness_freeze_snapshot" || ! -d "$witness_freeze_snapshot" ]] \
      || go run ./cmd/metasystem gate witness-freeze --cleanup "$witness_freeze_snapshot" >/dev/null 2>&1 \
      || true
    witness_fallback_rc=0
    if [[ "${WITNESS_GATE_FALLBACK:-plain}" == plain ]]; then
      bash scripts/agents/go-gate.sh || witness_fallback_rc=$?
    fi
    return "$witness_fallback_rc" 2>/dev/null || exit "$witness_fallback_rc"
  fi
  witness_gate_rc=0
  ( cd "$witness_snap" \
      && GOFLAGS=-mod=readonly METASYSTEM_GATE_FROZEN_TOOLCHAIN=1 \
         METASYSTEM_PROOF_EXECUTION_ROOT="$root" \
         METASYSTEM_GATE_WITNESS_WRITE="$witness_state/witness.json" \
         METASYSTEM_GATE_WITNESS_RUN="$witness_run" \
         METASYSTEM_GATE_WITNESS_MANIFEST_DIGEST="$witness_manifest_digest" \
         METASYSTEM_GATE_WITNESS_CONTROLLER_PID="$witness_controller_pid" \
         METASYSTEM_GATE_WITNESS_CONTROLLER_STARTED_AT="$witness_controller_started_at" \
         METASYSTEM_GATE_WITNESS_CONTROLLER_START_TICKS="$witness_controller_start_ticks" \
         METASYSTEM_GATE_WITNESS_CONTROLLER_BOOT_ID="$witness_controller_boot_id" \
         bash scripts/agents/go-gate.sh ) || witness_gate_rc=$?
  if (( witness_gate_rc != 0 )); then
    rm -rf "$witness_state"; witness_state=
    [[ ! -d "$witness_freeze_snapshot" ]] \
      || go run ./cmd/metasystem gate witness-freeze --cleanup "$witness_freeze_snapshot" >/dev/null 2>&1 \
      || true
    if (( witness_gate_rc == 3 )); then
      echo "witness gate refused in its frozen project (reason above); falling back to the plain gate" >&2
      witness_fallback_rc=0
      if [[ "${WITNESS_GATE_FALLBACK:-plain}" == plain ]]; then
        bash scripts/agents/go-gate.sh || witness_fallback_rc=$?
      else
        witness_fallback_rc=$witness_gate_rc
      fi
      return "$witness_fallback_rc" 2>/dev/null || exit "$witness_fallback_rc"
    fi
    echo "witness gate failed in its frozen project (exit $witness_gate_rc): the red above is the answer, no fallback" >&2
    return "$witness_gate_rc" 2>/dev/null || exit "$witness_gate_rc"
  fi
  if [[ ! -f "$witness_state/witness.json" ]]; then
    echo "witness gate completed without publishing witness evidence" >&2
    rm -rf "$witness_state"; witness_state=
    go run ./cmd/metasystem gate witness-freeze --cleanup "$witness_freeze_snapshot" >/dev/null 2>&1 || true
    return 1 2>/dev/null || exit 1
  fi
  witness_publish_rc=0
  witness_stage="bin/.metasystem.witness.$$"
  mkdir -p bin || witness_publish_rc=$?
  if (( witness_publish_rc == 0 )); then
    cp "$witness_snap/bin/metasystem" "$witness_stage" || witness_publish_rc=$?
  fi
  if (( witness_publish_rc == 0 )); then
    mv -f "$witness_stage" bin/metasystem || witness_publish_rc=$?
  fi
  if (( witness_publish_rc != 0 )); then
    echo "witness gate completed but the proven binary could not be published" >&2
    rm -f "$witness_stage" || true
    rm -rf "$witness_state"; witness_state=
    go run ./cmd/metasystem gate witness-freeze --cleanup "$witness_freeze_snapshot" >/dev/null 2>&1 || true
    return "$witness_publish_rc" 2>/dev/null || exit "$witness_publish_rc"
  fi
  if ! go run ./cmd/metasystem gate witness-freeze --cleanup "$witness_freeze_snapshot"; then
    echo "witness gate completed but its frozen project could not be released" >&2
    rm -rf "$witness_state"; witness_state=
    return 1 2>/dev/null || exit 1
  fi
  export METASYSTEM_GATE_WITNESS="$witness_state/witness.json"
  export METASYSTEM_GATE_WITNESS_ROOT="$witness_state"
  export METASYSTEM_GATE_WITNESS_RUN="$witness_run"
  unset METASYSTEM_GATE_WITNESS_EXPORT
  echo "gate witness armed from complete frozen project"
  echo "gate witness armed for this run's nested validations"
else
  if [[ "${WITNESS_GATE_FALLBACK:-plain}" == plain ]]; then
    bash scripts/agents/go-gate.sh
  fi
fi
