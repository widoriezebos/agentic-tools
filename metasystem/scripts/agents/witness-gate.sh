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
# contract — dirty or forced runs fall back, never half-arm. A witness that
# existed at entry is consumed or refused; it is never replaced in place.

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
  if METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE=ENGINE \
      bash scripts/agents/go-gate.sh --witness-check-only >/dev/null 2>&1; then
    METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE=ENGINE bash scripts/agents/go-gate.sh
    witness_engine_reused=1
  else
    # A refused inherited witness cannot become acceptable later in this root
    # and silently change its run class. The fallback runs with the entire
    # handoff scrubbed, so it is a real proof or no proof by explicit choice.
    unset METASYSTEM_GATE_WITNESS METASYSTEM_GATE_WITNESS_ROOT METASYSTEM_GATE_WITNESS_RUN
    if [[ "${WITNESS_GATE_FALLBACK:-plain}" == plain ]]; then
      METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE=ENGINE bash scripts/agents/go-gate.sh
    fi
  fi
  return 0 2>/dev/null || exit 0
fi
# Ask the prospective source engine for ENGINE membership. This replaces the
# last separate copy of D33's positive closure; a policy edit is itself inside
# ENGINE and therefore decides eligibility under its own prospective bytes.
witness_prefix_status=0
witness_prefix=$(git -C "$root" rev-parse --show-prefix 2>/dev/null) || witness_prefix_status=$?
witness_git_root=$(git -C "$root" rev-parse --show-toplevel 2>/dev/null) || witness_prefix_status=$?
witness_roots_bytes=
if [[ $witness_prefix_status == 0 ]]; then
  witness_roots_bytes=$(
    {
      git -C "$witness_git_root" diff --no-renames --name-only -z HEAD -- &&
      git -C "$witness_git_root" ls-files --others --exclude-standard --full-name -z &&
      git -C "$witness_git_root" ls-files --others -i --exclude-standard --full-name -z
    } | go run ./cmd/metasystem behavior-surface select \
          --projection ENGINE --prefix "$witness_prefix" --nul \
      | wc -c
  ) && witness_roots_clean=1 || witness_roots_clean=0
else
  witness_roots_clean=0
fi
# A failed Git/policy pipeline is INELIGIBLE, never an empty-clean answer.
[[ "${witness_roots_bytes//[[:space:]]/}" == 0 ]] || witness_roots_clean=0
if (( ! ${delivery_contract:-0} )) && (( witness_roots_clean )) \
  && [[ "${METASYSTEM_COVERAGE_RATCHET_SEED:-0}" != 1 && "${METASYSTEM_GATE_FORCE:-0}" != 1 ]]; then
  witness_state=$(mktemp -d)
  chmod 700 "$witness_state"
  witness_snap=$(mktemp -d)
  chmod 700 "$witness_snap"
  witness_run="run-$$-$RANDOM"
  witness_controller_pid=$$
  witness_controller_started_at=
  witness_controller_start_ticks=
  witness_controller_boot_id=
  if read -r witness_controller_started_at witness_controller_start_ticks witness_controller_boot_id \
      < <(go run ./cmd/metasystem proc started-at --pid $$ --emit pair); then
    [[ "$witness_controller_boot_id" == - ]] && witness_controller_boot_id=
  else
    witness_roots_clean=0
  fi
  witness_toplevel=$(git rev-parse --show-toplevel)
  witness_prefix=${root#"$witness_toplevel"}; witness_prefix=${witness_prefix#/}
  if [[ -n "$witness_prefix" ]]; then
    git -C "$witness_toplevel" archive "HEAD:$witness_prefix" | tar -x -C "$witness_snap"
  else
    git -C "$witness_toplevel" archive HEAD | tar -x -C "$witness_snap"
  fi
  if (( witness_roots_clean )) && ( cd "$witness_snap" \
      && METASYSTEM_GATE_WITNESS_WRITE="$witness_state/witness.json" \
         METASYSTEM_GATE_WITNESS_RUN="$witness_run" \
         METASYSTEM_GATE_WITNESS_CONTROLLER_PID="$witness_controller_pid" \
         METASYSTEM_GATE_WITNESS_CONTROLLER_STARTED_AT="$witness_controller_started_at" \
         METASYSTEM_GATE_WITNESS_CONTROLLER_START_TICKS="$witness_controller_start_ticks" \
         METASYSTEM_GATE_WITNESS_CONTROLLER_BOOT_ID="$witness_controller_boot_id" \
         bash scripts/agents/go-gate.sh ) \
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
