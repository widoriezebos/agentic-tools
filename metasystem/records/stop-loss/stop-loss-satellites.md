# Stop-loss satellites — the patience program

Owner: unclaimed (each satellite is its own design + critique loop).
Parent: `plans/stop-loss-last-defense.md` (critique-exhausted; split
approved). Core: `plans/stop-loss-core.md` (SHIPPED). Concepts and the
whole-mechanism picture: `docs/patience.md` — PATIENCE, PROGRESS, and
STALL are the binding names (human ruling, 2026-08-11: "slower progress
is still progress"; the stop criterion is value produced per step, with
per-(role, model) patience, never bare cycle counting).

Ordering rationale: 1 and 2 eliminate FALSE STALL (a patience verdict is
only as honest as its inputs); 3 completes the value and cost ledgers;
4 is the patience mechanism itself, for which 1–3 are preconditions.
Precondition for ALL: first map the runner's ACTUAL cycle sequence
(internal/missionrunner/loop.go) — the parent died of designing against
an assumed sequence.

1. TURN IDENTITY (announced/observed sessions, honest-host acceptance,
   protocol-violation handling). Kills false stall from harness
   bookkeeping: an honest host must never be booked as a valueless
   cycle. Routed findings: parent r1[6][7][8], r3[9].
2. MISSION REAP + BOUNDED DRAIN (lease-authorized, custodian-proven
   reaping of the mission's own reservations; finite drain deadline;
   drain-stalled park with a cycle-consistent resume). Kills false stall
   from starved dispatch: a mission that cannot act is not stalling.
   Routed: parent r1[9][10], r2[8][9], r3[10][11].
3. ORPHAN + USAGE CAPTURE (applied-(chain,round) sets, scan points that
   close the race, single-writer usage with SIGKILL recovery from the
   runtime's surviving event stream). Banks all produced value and
   completes the cost denominator. Routed: parent r1[11][12],
   r2[10][11][13], r3[12][13].
4. PATIENCE (replaces the deferred loop-advanced credits). SETTLED by
   its own design loop — plans/patience-satellite-4.md is the
   authority; this routing entry records the original intent and the
   deltas the loop imposed (its r7/P4-047 disposition). As routed
   here, this satellite was to carry per-activity observables
   (critique closure, gate metrics, verifier confirmations) and
   conf-roster patience floors. The loop replaced both with evidence:
   ONE observable — a turn-log certification with verdict accepted —
   because per-activity proxies (schema-valid returns, closures,
   confirmations) are exactly the narrative the drought hides behind;
   and floors as SEALED CONTRACT ENTRIES ONLY, counted in value-barren
   rounds, because the conf/local/env layer is bypassable and an
   unsealed fallback would change signed missions. Breaches are vocal
   only. What survives unchanged: observables are checkable artifacts
   only, never assertions; the verdict stays a pure ledger replay
   (patience annotations are never fuse input); the shipped core is
   the degenerate case; and patience is itself a fuse, so the
   last-defense calibration rule applies to it recursively. Routed:
   parent r1[0][1], r2[2][3][4][5], r3[4][5][6].
