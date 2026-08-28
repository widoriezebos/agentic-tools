# Patience is measured in attempts, not wall-clock (fixture harness)

STATUS: IMPLEMENTED (2026-08-19). Phase 1 landed after TWO design critiques
(codex gpt-5.6-sol + Claude, converged on the schema-bump and counter-lifecycle
must-fixes, both folded pre-implementation) and TWO code critiques (Claude:
"NO REAL DEFECTS"; codex: two defense-in-depth edge cases folded — a corrupt
NEGATIVE scanSeq now restarts from 0 in `lastPublishedScanSeq`, and the shell
re-baseline branch is bounded so an oscillating writer fails as "unstable"
rather than looping forever). Supersedes the reverted runtime-extension patch
in `plans/census-settle-load-flake.md`.

## The principle

A fixture waiting on an asynchronous actor's effect declares patience in two
tiers, both under one "patience" concept:

- **Tier 1 — semantic patience, counted in the ACTOR'S ATTEMPTS.** "This should
  happen within K attempts of the actor; K attempts that fail to produce it = a
  genuine defect." Load-independent for the CUMULATIVE wait: a slow machine
  makes each attempt slower, but the NUMBER of attempts a correct system needs
  is the same on any machine. K is a small human constant (default 2). This
  tier is the actual fix — it removes the summed-latency load-sensitivity that
  the calibrated cap had (many slow passes summing past a wall-clock bound).
- **Tier 2 — a coarse, honestly-labelled failsafe.** "If NO census pass has
  COMPLETED for T seconds, abort so CI can't hang forever." This is a
  per-single-pass wall-clock bound, so it is load-AWARE, not load-free (see the
  honest scope below). It only has to exceed the worst-case duration of ONE
  census pass (bounded work — a single /proc scan), never the whole wait, so it
  can be set very generously. Reported as *"no completed census pass for Ts
  (census wedged, or a single pass exceeded Ts)"* — NOT "actor silent", because
  we cannot distinguish a genuine wedge from one pathologically long pass.

Honest scope (folded from critique): the headline "load-independent" is TRUE
for the multi-pass cumulative latency — the KI-37 flake, where the old cap was
blown by a slow *cadence* of passes. It is NOT absolute: a single pass that
exceeds the generous T still trips Tier 2. That residual exposure is far
weaker (a single /proc scan is bounded work, not an unbounded wait) and is set
generously, so it does not reproduce KI-37.

## Taxonomy of waits

- **Waits on a census pass's effect** (verdict/errors/inventory, dead-main
  pruning) -> Tier 1 on the census attempt marker + Tier 2. Every observed
  flake is here.
- **Waits on an external / OS event** (PID death, `kill -0`) -> Tier-2-style
  wall-clock only: no actor attempt to count, so a generous wall-clock timeout
  is the honest tool. Keep legacy `wait_until` for these.
- **Waits on owner-relaunch** (S4-4 component recovery) -> Phase 2 (below); the
  `generation` marker is already monotonic and persisted, so they are ready to
  migrate but are NOT in this change and stay on `wait_until` for now.

## The census attempt marker: `scanSeq`, owned at the PUBLISH boundary

The census verdict `last-census.json` has no per-pass counter today
(`CompletedAtEpoch` is second-granular `Unix()`; `Generation` tracks
supervision STATE, `internal/census/run.go:67`). Add one — but NOT as a
loop-local counter (both critics, HIGH): the watcher process is killed and
relaunched routinely (owner replacement, and the S4-4 fixtures kill it
outright), so a per-process counter RESETS in the very file the fixture polls,
breaking monotonicity exactly under the load this targets.

Instead the counter is **seeded from the last published verdict and advanced
only on successful publish**:

- At writer startup (standing loop AND the one-shot `watcher-pass` verb), read
  `last-census.json`'s `scanSeq` (0 if absent/unreadable); hold it in memory.
- On publish: stamp `scanSeq = held + 1`, write atomically (temp+rename); ONLY
  on write success, `held = held + 1`. On write failure keep `held` and retry
  the same next value (closes codex HIGH: a failed publish never lets a stale
  in-flight pass reach `base+2`).
- Monotonic ACROSS process lifetimes: a relaunched watcher and each repeated
  one-shot invocation both seed from the file the previous writer left, so the
  sequence continues rather than resetting. This also disarms the one-shot
  `scanSeq=0` landmine (both critics, LOW).
- Single-writer: supervision custody guarantees one watcher owns the
  supervision dir at a time (old dies before new arms), so the
  read-current -> publish+1 has no concurrent writer.

Add `ScanSeq int64 \`json:"scanSeq,omitempty"\`` to `census.Verdict`. **Do NOT
bump SchemaVersion** (both critics, HIGH): `internal/dispatch/attest.go:47`
(`CensusFresh`, a PRODUCTION gate) hard-rejects any live verdict whose
`schemaVersion != 2` — a bump fails every dispatch closed. `scanSeq` is an
additive optional field; Go/`map` JSON readers and the shell asserts
(`schemaVersion == 2` + key-presence, `supervision-fixtures.sh:422`,
`telemetry-census-fixtures.sh:116`) all tolerate an added field. Schema stays 2.

Stamp `scanSeq` in ONE central place so no path is missed (codex MED): the
FINGERPRINT-FAILED path returns early (`watcher.go:60`) before the common
block, so stamping must live in `publish()` — which every path calls — where
the held counter is read and incremented. Cover all three verdict paths
(SUCCESS, census-error CENSUS-FAILED, FINGERPRINT-FAILED) with explicit tests.

## Freshness: `scanSeq >= S1+2` (confirmed sound by both critics)

A census pass already in flight when the fixture plants its state read the OLD
input. The watcher loop is single-flight (`supervise_component.go:124-136`,
`for/select` on a `time.Ticker`, no overlap; the pass reads input DURING the
pass and publishes atomically at the END, `watcher.go:69-82`). Therefore:

1. Plant the fixture state.
2. Read `S1` = `scanSeq` of the latest published verdict, AFTER the write.
3. The pass publishing `S1+1` may have started before the write -> not
   provably fresh. The pass publishing `S1+2` starts only after `S1+1`
   completes (no overlap), which is after the write -> it read fresh input.
   **`scanSeq >= S1+2` proves the pass saw the write.** Never needs +3 (input
   read is strictly after pass-start, pass-start strictly after prior publish).
   +2 is one interval stronger than necessary in the no-pass-in-flight case —
   harmless latency, not a defect.

## The primitive (shell)

Reads the verdict file ONCE per iteration into a snapshot and evaluates BOTH
`scanSeq` and the predicate against that SAME snapshot (both critics, MED: two
separate reads could bind a fresh seq to a stale predicate and false-pass /
false-fail). Predicates are rewritten to take the captured JSON value, not
re-read `$last`.

    wait_for_census <name> <K> <predicate-fn>   # predicate-fn takes the snapshot JSON
      base=$(scanseq_of "$(cat "$last")")        # S1, read after caller planted state
      last_marker=$base; last_adv=$SECONDS
      loop:
        snap=$(cat "$last" 2>/dev/null) || snap=""
        m=$(scanseq_of "$snap")                  # "" if unreadable
        if [[ "$m" =~ ^[0-9]+$ ]]; then
          if (( m < last_marker )); then          # a fresh writer took over (defensive;
            base=$m; last_marker=$m; last_adv=$SECONDS   # seed-from-file normally prevents this)
          elif (( m > last_marker )); then last_marker=$m; last_adv=$SECONDS; fi
          # Tier 1 success: predicate true on a provably-fresh pass
          if (( m >= base+2 )) && "$predicate_fn" "$snap"; then return 0; fi
          # Tier 1 failure: K fresh passes have completed still-false
          if (( m >= base+1+K )) && ! "$predicate_fn" "$snap"; then
            fail "$name: still wrong after $K fresh census passes (scanSeq $base -> $m)"; fi
        fi
        # Tier 2 failsafe: no pass has COMPLETED for T_SILENCE
        if (( SECONDS - last_adv >= T_SILENCE )); then
          fail "$name: no completed census pass for ${T_SILENCE}s (wedged, or one pass exceeded it)"; fi
        sleep "$METASYSTEM_FIXTURE_POLL_INTERVAL_SEC"

`T_SILENCE` is derived from the verdict's OWN published cadence, not an env var
the component does not read (codex/Claude HIGH): read `intervalSec` from the
snapshot and use `T_SILENCE = max(FLOOR, MULT * intervalSec)` with a generous
FLOOR and MULT (e.g. FLOOR=60, MULT=30) so it exceeds any single healthy pass
by a wide margin. It is a coarse failsafe, labelled as such.

**Precondition (documented): level-triggered predicates only.** The planted
state persists across passes (the fixture does not mutate it mid-wait), so once
the predicate is true it STAYS true until observed — which is what makes
skipped publications and the freshness requirement safe (both critics). Every
migrated wait satisfies this (a bad record stays bad until removed; a removed
record stays removed). A future non-idempotent predicate must NOT use this
primitive.

## Scope / phasing (claim corrected per critique)

**Phase 1 (this change) — the census-settle SUBCLASS (all observed KI-37
flakes):**
- Add census `scanSeq` (publish-boundary counter), schema stays 2.
- Add `wait_for_census` (Tier 1 + Tier 2) to the fixture harness.
- Migrate the 11 census-output waits + the dead-main pruning wait
  (`supervision-fixtures.sh:844`, itself a census side effect
  `census/run.go:666`) to `wait_for_census`.
- OS-event waits (PID death, `kill -0`) stay on legacy `wait_until`.

Do NOT claim "kills the flake as a CLASS" (Claude MED): Phase 1 kills the
census-settle subclass. The S4-4 owner-relaunch waits stay on the calibrated
`wait_until` cap and remain a (never-yet-observed) load exposure until Phase 2.

**Phase 2 (documented follow-on, NOT this change):** migrate S4-4
component-recovery waits to attempt-count against `state.json` `generation`
(already monotonic + persisted, `internal/supervise/disk.go:108`) / the
`owner.ndjson` relaunch count (`supervision-fixtures.sh:816`).

## Verification plan

- Go unit tests: `scanSeq` stamped on all three verdict paths; monotonic across
  a simulated relaunch (seed-from-file); does not advance on a failed publish;
  `dispatch/attest.go` `CensusFresh` still accepts the verdict (schema stays 2).
- Shell behavioural test of `wait_for_census`: (i) predicate true within K ->
  pass; (ii) predicate false after K fresh passes -> fail "after K passes";
  (iii) marker frozen -> fail "no completed pass for T"; (iv) slow-but-advancing
  marker + eventual truth -> pass regardless of wall-clock (load-independence);
  (v) writer-takeover mid-wait (marker regresses) -> re-baselines, still passes.
- Full `scripts/validate-metasystem.sh` green on the Linux guest, INCLUDING a
  deliberately loaded run (the case the old cap failed) — and confirm it does
  NOT balloon runtime (Tier 1 returns as soon as a fresh pass satisfies it).
