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
# METASYSTEM_GATE_WITNESS{,_ROOT,_RUN,_EXPORT}, refreshes bin/metasystem
# from the proven snapshot, and leaves $witness_state set — the CALLER
# owns removing that directory at exit. A clean tree retains the HEAD-archive
# proof path. A dirty tree is copied into a private frozen export and the gate
# runs only there; a source change during that copy voids arming loudly. Seed,
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
# The historical cleanliness decision stays first. A clean source follows the
# byte-for-byte existing HEAD archive branch below; only a proven dirty result
# may enter the new freeze branch.
witness_prefix_status=0
witness_prefix=$(git -C "$root" rev-parse --show-prefix 2>/dev/null) || witness_prefix_status=$?
witness_git_root=$(git -C "$root" rev-parse --show-toplevel 2>/dev/null) || witness_prefix_status=$?
witness_roots_bytes=
witness_clean_check_ok=0
if [[ $witness_prefix_status == 0 ]]; then
  if witness_roots_bytes=$(
    {
      git -C "$witness_git_root" diff --no-renames --name-only -z HEAD -- &&
      git -C "$witness_git_root" ls-files --others --exclude-standard --full-name -z &&
      git -C "$witness_git_root" ls-files --others -i --exclude-standard --full-name -z
    } | go run ./cmd/metasystem behavior-surface select \
          --projection ENGINE --prefix "$witness_prefix" --nul \
      | wc -c
  ); then
    witness_clean_check_ok=1
    witness_roots_clean=1
  else
    witness_roots_clean=0
  fi
else
  witness_roots_clean=0
fi
[[ "${witness_roots_bytes//[[:space:]]/}" == 0 ]] || witness_roots_clean=0
witness_common_eligible=0
if (( ! ${delivery_contract:-0} )) \
  && [[ "${METASYSTEM_COVERAGE_RATCHET_SEED:-0}" != 1 && "${METASYSTEM_GATE_FORCE:-0}" != 1 ]]; then
  witness_common_eligible=1
fi

if (( witness_common_eligible && witness_roots_clean )); then
  # This is the established clean-tree path. It intentionally does not call
  # witness-freeze, alter the toolchain environment, or change witness bytes.
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
elif (( witness_common_eligible && witness_clean_check_ok )); then
  # Frozen proof runs discard ambient build flags, force read-only module
  # selection, and keep GOMODCACHE inherited. The shared module cache is safe:
  # go.sum inside the frozen tree pins module bytes by hash, while readonly mode
  # forbids resolving a different dependency set. Alternate modfiles and Go
  # overlays would replace frozen inputs, so they refuse before any export.
  if [[ " ${GOFLAGS:-} " =~ [[:space:]]-(modfile|overlay)(=|[[:space:]]) ]]; then
    echo "witness gate arming voided: GOFLAGS may not contain -modfile or -overlay" >&2
    return 1 2>/dev/null || exit 1
  fi
  witness_freeze_output=
  if ! witness_freeze_output=$(go run ./cmd/metasystem gate witness-freeze --root "$root"); then
    echo "witness gate arming voided: the dirty tree could not be frozen consistently" >&2
    return 1 2>/dev/null || exit 1
  fi
  read -r witness_manifest_digest witness_snap <<<"$witness_freeze_output"
  if [[ ! "$witness_manifest_digest" =~ ^[a-f0-9]{64}$ || ! -d "$witness_snap" ]]; then
    [[ -z "$witness_snap" ]] || rm -rf "$(dirname "$witness_snap")"
    echo "witness gate arming voided: witness-freeze returned an invalid digest or export path" >&2
    return 1 2>/dev/null || exit 1
  fi
  witness_state=$(dirname "$witness_snap")
  witness_run="run-$$-$RANDOM"
  witness_controller_pid=$$
  witness_controller_started_at=
  witness_controller_start_ticks=
  witness_controller_boot_id=
  if read -r witness_controller_started_at witness_controller_start_ticks witness_controller_boot_id \
      < <(go run ./cmd/metasystem proc started-at --pid $$ --emit pair); then
    [[ "$witness_controller_boot_id" == - ]] && witness_controller_boot_id=
  else
    rm -rf "$witness_state"; witness_state=
    echo "witness gate arming voided: the live controller identity could not be read" >&2
    return 1 2>/dev/null || exit 1
  fi
  witness_gate_rc=0
  ( cd "$witness_snap" \
      && GOFLAGS=-mod=readonly METASYSTEM_GATE_FROZEN_TOOLCHAIN=1 \
         METASYSTEM_GATE_WITNESS_WRITE="$witness_state/witness.json" \
         METASYSTEM_GATE_WITNESS_RUN="$witness_run" \
         METASYSTEM_GATE_WITNESS_MANIFEST_DIGEST="$witness_manifest_digest" \
         METASYSTEM_GATE_WITNESS_CONTROLLER_PID="$witness_controller_pid" \
         METASYSTEM_GATE_WITNESS_CONTROLLER_STARTED_AT="$witness_controller_started_at" \
         METASYSTEM_GATE_WITNESS_CONTROLLER_START_TICKS="$witness_controller_start_ticks" \
         METASYSTEM_GATE_WITNESS_CONTROLLER_BOOT_ID="$witness_controller_boot_id" \
         bash scripts/agents/go-gate.sh ) || witness_gate_rc=$?
  if [[ "$witness_gate_rc" == 0 && -f "$witness_state/witness.json" ]]; then
    mkdir -p bin \
      && cp "$witness_snap/bin/metasystem" "bin/.metasystem.witness.$$" \
      && mv -f "bin/.metasystem.witness.$$" bin/metasystem
    export METASYSTEM_GATE_WITNESS="$witness_state/witness.json"
    export METASYSTEM_GATE_WITNESS_ROOT="$witness_state"
    export METASYSTEM_GATE_WITNESS_RUN="$witness_run"
    export METASYSTEM_GATE_WITNESS_EXPORT="$witness_snap"
    echo "gate witness armed from frozen dirty export $witness_snap"
    echo "gate witness armed for this run's nested validations"
  else
    echo "witness gate did not complete in its frozen dirty tree" >&2
    rm -rf "$witness_state"; witness_state=
    [[ "$witness_gate_rc" != 0 ]] || witness_gate_rc=1
    return "$witness_gate_rc" 2>/dev/null || exit "$witness_gate_rc"
  fi
else
  if [[ "${WITNESS_GATE_FALLBACK:-plain}" == plain ]]; then
    bash scripts/agents/go-gate.sh
  fi
fi
