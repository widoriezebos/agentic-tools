# Patience, progress, and stall

Status: SHIPPED — the whole mechanism is implemented. The stop-loss
core (`docs/design/stop-loss-core.md`, `internal/missionrunner/stoploss.go`)
and all four satellites of `records/stop-loss/stop-loss-satellites.md` are built
and suite-verified; satellite 4 (patience floors,
`records/patience/patience-satellite-4.md`, accepted after a 22-round critique
loop) landed 2026-08-12 in `internal/missionrunner/patience.go`. This
document names the concepts and explains the whole mechanism.
Ruled by the human, 2026-08-11. The three words below are the binding
vocabulary: anything built around this mechanism uses these names.

## The three concepts

**Progress** is value produced, proven mechanically — a checkable
artifact, never an agent's assertion that things are going well. At the
mission level the gate metric that beat its best is progress (the
shipped core). At the chain level, satellite 4's loop settled ONE
observable after rejecting per-activity proxies with evidence: a chain
round has produced value exactly when a concluded turn's durable log
certifies its job with verdict accepted. Proxies — schema-valid
returns, critique closures, verifier confirmations — are exactly the
narrative a drought hides behind (records/patience/patience-satellite-4.md,
r1/P4-001 through r2/P4-024). Some activities need no progress
definition at all: single-shot, or governed by an inner loop with its
own stop criterion — bounded by construction.

**Patience** is how much observation without progress the system tolerates
before it concludes anything, and it is a property of WHO is working: a
patience is set per role and per (runtime, model) pair, because a weaker
model legitimately needs more cycles per increment of the same value.
Slower progress is still progress; patience is what makes that sentence
enforceable. Patience is never a pacing target — it is sized so that only
genuine stall exhausts it, it is human-sealed at the mission level, and
the hard fences (wall clock, exposure, cycles) cap the total regardless.

**Stall** is the verdict when patience is exhausted: observed progress
stayed below the configured floor for the whole window, with nothing
else to blame. At the mission level (the shipped fuse) stall parks the
mission with a vocal ask; a human resets it with a ledger-recorded
answer — it cannot happen quietly. At the chain level (satellite 4) a
breached floor is VOCAL ONLY — a bounded ledger annotation and a
prompt line; it never parks, kills, or feeds the breaker, and
escalation from vocal to acting is a future human ruling taken with
trial evidence. Stall is a last defense against the unexpected endless
loop, not a scheduler.

## Why cycle counting was wrong

The bm-2s trial cohorts (2026-08-10) proved the failure: a lawful,
converging design-critique loop was killed by a fuse that counted three
cycles without gate movement — while 82% of the wall clock was unused, the
gate was structurally blind to design-phase value, and one of the three
counted cycles was the harness's own bookkeeping error. Counting cycles
punishes phase structure and punishes weaker models identically; measuring
progress with per-capability patience punishes only stall.

## The mechanism, whole

1. **Integrity first.** A patience verdict is only as honest as its
   inputs. Two classes of FALSE stall must be impossible before any floor
   is enforced: cycles falsely recorded as valueless (the turn-identity
   satellite: an honest host must never be booked as an invalid run), and
   cycles where the mission could not act at all (the reap/drain
   satellite: a starved dispatch is the harness's fault, not the model's
   rate). Unbanked value is the third leak: a delegate return that landed
   and was never read is progress that happened and counted for nothing
   (the orphan/usage satellite — which also completes the cost ledger,
   the denominator if patience is ever spend-denominated).
2. **Observables.** ONE observable, settled by satellite 4's loop
   (records/patience/patience-satellite-4.md): a chain round is WITNESSED
   exactly when a concluded turn's durable log certifies its job with
   verdict accepted and non-empty evidence — witnessed consumption,
   the orchestrator's accountable decision to consume the round;
   whether the work was truly valuable stays unjudged here
   (r20/P4-095). Per-activity proxies —
   schema-valid returns, critique closures, verifier confirmations —
   were rejected with evidence: they are the narrative a drought
   hides behind. Two mechanisms, kept distinct (r9/P4-062): the FUSE
   verdict is a pure replay of the ledger against the sealed contract
   (no cached counters — the stop-loss core's architecture,
   unchanged); the patience OBSERVATION derives from job records and
   the durable turn log, and writes annotations the fuse never
   reads.
3. **Patience floors.** Sealed mission-contract entries ONLY
   (`patience.rounds.<role>.<runtime>.<model>`), counted in
   value-barren rounds, never minutes. The earlier placeholder placed
   floors in `metasystem.conf` roster keys; satellite 4's critique
   round killed that layer with evidence (dispositions r1, P4-006/
   P4-007): the local/env resolution path is bypassable, an unsealed
   conf fallback would let a repository edit change a signed mission's
   behavior, and non-mission chains have no runner evaluating them —
   the human at the keyboard is their patience. Unconfigured means
   infinite patience; the shipped core is the degenerate case: floor =
   "any above-noise new best", window = `ledger.no-gain-budget` cycles.
4. **Stall handling.** A breached floor is vocal only — a bounded
   ledger annotation and a prompt line; it never parks, kills, or
   feeds the breaker. Parking stays with the fuses; the reset is a
   human answer recorded in the ledger before the unpark; the hard
   fences remain the absolute stopgap above everything. Escalation
   from vocal to acting is a future human ruling taken with trial
   evidence.

## The recursion caution

Patience is itself a fuse, so the last-defense ruling applies to it
recursively: a floor calibrated as a pacing target rebuilds the original
trap one level down, per model. When in doubt, a floor is too low, a
window too long, and the human reset carries the rest. The reference
failure and its analysis: `records/stop-loss/stop-loss-last-defense.md` and the
recorded precedent in `skills/design-critique/SKILL.md`.
