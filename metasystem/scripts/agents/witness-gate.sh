# The witness-producing gate (D33), extracted so every battery stage
# shares ONE proof of the tree instead of re-running the race suite
# per nested validation. SOURCED, never executed: the point is the
# exported witness in the caller's environment.
#
# Contract: the caller sets $root (the metasystem root, already cwd),
# $delivery_contract (0/1), and WITNESS_GATE_FALLBACK as an EXPLICIT
# choice — "plain" runs the ordinary go-gate when the witness is
# ineligible or fails (validate-metasystem's historical behavior);
# "none" arms nothing and runs nothing, for callers whose nested
# validations carry their own gates. Any other value refuses: the
# fallback decides whether a gate runs at all, so ambient or mistyped
# state must die loudly, never silently suppress it. On success it
# exports
# METASYSTEM_GATE_WITNESS{,_ROOT,_RUN}, refreshes bin/metasystem from
# the proven snapshot, and leaves $witness_state set — the CALLER owns
# removing that directory at exit. Eligibility: gate-input roots clean
# against HEAD, no ratchet seed, no gate force, not a delivery
# contract — dirty or forced runs fall back, never half-arm.

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
witness_roots_status=$(git status --porcelain -- cmd internal go.mod go.sum scripts/agents 2>/dev/null) \
  && witness_roots_clean=1 || witness_roots_clean=0
# A failed git status must read as INELIGIBLE, never as clean: an empty
# answer from a non-repository is silence, not cleanliness.
[[ -n "$witness_roots_status" ]] && witness_roots_clean=0
if (( ! ${delivery_contract:-0} )) && (( witness_roots_clean )) \
  && [[ "${METASYSTEM_COVERAGE_RATCHET_SEED:-0}" != 1 && "${METASYSTEM_GATE_FORCE:-0}" != 1 ]]; then
  witness_state=$(mktemp -d)
  chmod 700 "$witness_state"
  witness_snap=$(mktemp -d)
  chmod 700 "$witness_snap"
  witness_run="run-$$-$RANDOM"
  witness_toplevel=$(git rev-parse --show-toplevel)
  witness_prefix=${root#"$witness_toplevel"}; witness_prefix=${witness_prefix#/}
  if [[ -n "$witness_prefix" ]]; then
    git -C "$witness_toplevel" archive "HEAD:$witness_prefix" | tar -x -C "$witness_snap"
  else
    git -C "$witness_toplevel" archive HEAD | tar -x -C "$witness_snap"
  fi
  if ( cd "$witness_snap" \
      && METASYSTEM_GATE_WITNESS_WRITE="$witness_state/witness.json" \
         METASYSTEM_GATE_WITNESS_RUN="$witness_run" bash scripts/agents/go-gate.sh ) \
    && [[ -f "$witness_state/witness.json" ]]; then
    # Clean roots mean the snapshot's binary IS this tree's binary.
    # Stage beside the target and rename over it (go-build.sh's
    # documented pattern): cp over the live inode poisons macOS's
    # code-signature cache and later execs die SIGKILL — exactly the
    # silent suite death this line caused on 2026-08-16.
    mkdir -p bin \
      && cp "$witness_snap/bin/metasystem" "bin/.metasystem.witness.$$" \
      && mv -f "bin/.metasystem.witness.$$" bin/metasystem
    export METASYSTEM_GATE_WITNESS="$witness_state/witness.json"
    export METASYSTEM_GATE_WITNESS_ROOT="$witness_state"
    export METASYSTEM_GATE_WITNESS_RUN="$witness_run"
    echo "gate witness armed for this run's nested validations"
  else
    echo "witness gate did not complete; falling back to the plain gate" >&2
    rm -rf "$witness_state"; witness_state=
    if [[ "${WITNESS_GATE_FALLBACK:-plain}" == plain ]]; then
      bash scripts/agents/go-gate.sh
    fi
  fi
  rm -rf "$witness_snap"
else
  if [[ "${WITNESS_GATE_FALLBACK:-plain}" == plain ]]; then
    bash scripts/agents/go-gate.sh
  fi
fi
