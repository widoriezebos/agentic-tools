# Patience, progress, and stall

Status: PLACEHOLDER — the concept home for a mechanism that is partly
shipped and partly designed. The shipped part is the stop-loss core
(`plans/stop-loss-core.md`, implemented in `internal/missionrunner/
stoploss.go`); the designed-but-unbuilt parts are routed through
`plans/stop-loss-satellites.md`. This document names the concepts and
explains the whole mechanism so the pieces are built toward one picture.
Ruled by the human, 2026-08-11. The three words below are the binding
vocabulary: anything built around this mechanism uses these names.

## The three concepts

**Progress** is value produced, proven mechanically. A checkable artifact
— a joined critique round, a gate metric that beat its best, a requirement
newly passing — never an agent's assertion that things are going well.
What counts as progress depends on the activity: design progresses when a
critique round reaches mechanical closure; implementation progresses when
the measured gate improves; verification progresses when confirmations
land against the completion gate. Some activities need no progress
definition at all, because they are single-shot or already governed by an
inner loop with its own stop criterion and budget — bounded by
construction.

**Patience** is how much observation without progress the system tolerates
before it concludes anything, and it is a property of WHO is working: a
patience is set per role and per (runtime, model) pair, because a weaker
model legitimately needs more cycles per increment of the same value.
Slower progress is still progress; patience is what makes that sentence
enforceable. Patience is never a pacing target — it is sized so that only
genuine stall exhausts it, it is human-sealed at the mission level, and
the hard fences (wall clock, exposure, cycles) cap the total regardless.

**Stall** is the verdict when patience is exhausted: observed progress
stayed below the configured floor for the whole window, with nothing else
to blame. Stall parks the mission with a vocal ask; a human can reset it
(the reset is a ledger line — it cannot happen quietly), or amend the
sealed allowances. Stall is a last defense against the unexpected endless
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
2. **Observables.** Each looping activity declares its mechanical
   progress observable, recorded as ledger lines. The ledger is the one
   source of truth; the verdict is a pure replay of it against the sealed
   contract (no cached counters — the stop-loss core established this
   architecture and it carries over unchanged).
3. **Patience floors.** Configuration lives where capability lives: the
   roster keys in `metasystem.conf`, per role and runtime:model pair,
   with mission contracts able to seal overrides. Defaults are generous.
   The shipped core is the degenerate case: floor = "any above-noise new
   best", window = `ledger.no-gain-budget` cycles.
4. **Stall handling.** Exhausted patience parks with the stop-loss ask;
   the reset is a human answer recorded in the ledger before the unpark;
   the hard fences remain the absolute stopgap above everything.

## The recursion caution

Patience is itself a fuse, so the last-defense ruling applies to it
recursively: a floor calibrated as a pacing target rebuilds the original
trap one level down, per model. When in doubt, a floor is too low, a
window too long, and the human reset carries the rest. The reference
failure and its analysis: `plans/stop-loss-last-defense.md` and the
recorded precedent in `skills/design-critique/SKILL.md`.
