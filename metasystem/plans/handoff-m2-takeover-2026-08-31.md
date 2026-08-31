# m2 takeover handoff — 2026-08-31

m1's Fable session is ending on capacity; m2 assumes the dispatch-delegate
seat for the whole program until Fable returns. This file is the complete
state transfer. Everything referenced is landed on origin/main unless
marked otherwise. The laws bind m2 identically — they are Go in this repo.

## What is DONE and landed (needs nothing)

- The delegation verb (`metasystem delegate`) with five first-use fixes:
  isolation derivation, typed refusals, capability-based authorization,
  `--reviews` forwarding, critique-reference stamping (`6fdd3a8` through
  `af84ba1e`, `d0f35a3e`, `6ddba517`).
- The three laws (`fe65edd`) plus the Law 2 fail-closed fix (`046a745b`):
  Decide and write-validation share one completeness helper; governance
  package at 100% coverage, floors registered.
- The counselor SIGNAL LAYER (`6cbc3e99`): `metasystem counselor brief
  --dry-run` works; delivered through the first fully machinery-enforced
  chain (counselor-drift-s1, closed, critique-stamped).
- The m1 seat governance record: `plans/seat-governance-record.md`,
  ACTIVE by R-30-m1 (`ca33fce3`). Its two conduct-only open items are
  acknowledged debts: landing binds no closed chain (remedy specified on
  goal `two-bars-for-changes` — now the highest-consequence design item);
  narrator+acceptor in one actor (remedies await Wido at review
  2026-11-30). These debts apply to ANY seat holding dispatch+custody,
  including m2 now — read the record before acting the role.
- m1's resident steward: armed TEMPORARILY under Wido's recorded word
  (R-29-m1 as amended; review 2026-09-06), runner live on m1. It keeps
  running after the session ends — it is machinery, not Fable.
- Rulings R-24 through R-30 (m1): submission discipline, model lanes,
  design-integrity, coordinator-question settlement, delegated
  model/effort judgment, the departure, the record activation.

## IN FLIGHT on m1 at handoff (m2 completes these)

1. LANGUAGE SWEEP (R-27 action 2, Wido-authorized): a Sol pass is
   executing the 96 AUTHORITY replacements per
   `plans/coordinator-language-inventory.md` in m1's main checkout. If
   m1 lands it before the session dies, done. IF NOT: the tree state
   dies with m1's checkout — RERUN it on m2: the inventory is the map
   (landed), the instruction is: execute its verdicts exactly; where a
   replacement conflicts with the active seat record, the record wins
   and the divergence goes in the receipt; callsigns and historical
   rows survive verbatim; land with a receipt in records/.
2. COUNSELOR CARRIAGE — built, critiqued, chain CLOSED
   (counselor-carriage, `chainClosed=true`,
   `independentCritiqueJobRef=counselor-carriage-crit`), NOT YET
   LANDED. Its certified diff is durable at m1's
   `artifacts/agents/counselor-carriage/rounds/2/diff.patch`
   (reviewedTree 5d7ef917...) — but that is m1-local runtime state.
   IF m1 fails to land it, m2 REBUILDS the slice (one 4h round): the
   brief is recorded in the chain rounds AND summarized on the
   counselor goal: steward tick renders the brief per
   `metasystem.counselor.brief-cadence-hours` (default 24) with a
   durable cursor, appends exact bytes to the narrator digest
   (no-softening: byte-equality test), render failure surfaces as
   counselor-named health, cursor advances only after successful
   append. Two known non-material lows ride the goal.
3. The GOVERNED WEIGHT DISCHARGE chain: weight is ~327 vs threshold 60.
   Sequence for m2: (a) build the set-obligation temporary-word leg
   mirroring the landed steward-arm mechanism (steward_verbs.go:486
   pattern) — small Sol pass; the seat record documents that this
   piece is absent and fails safe today; (b) Wido's recorded tuple for
   the standing milestone obligation is ALREADY GIVEN: attempts 2 /
   elapsed 24h / job-minutes 120 / concurrent 1 (decision-ask,
   2026-08-31, under departure R-29-m1) — record it verbatim at
   set-obligation; (c) run the governed validation, discharge.
   KNOWN INPUT: the last ungoverned full sweep failed ONLY on missing
   ratchet floors for governance/obligationstate — both now landed, so
   the next sweep should be green or newly informative.

## m1 claims to take over

- Goal `counselor` is CLAIMED by m1 with budget 72h/12/2400m/2.
  m1 releases it at handoff (this landing's companion act). Remaining
  slices are recorded on the goal: carriage landing (or rebuild),
  sitting (6h), ambient noticings (4h), teeth-binding-2 decision-record
  design (pinned on the goal — needs Fable-lane design; MAY WAIT for
  Fable's return if m2 prefers, it is not blocking).

## The queue, in order (all budgeted by Wido's recorded words)

1. Finish the in-flight three above.
2. `watch-verb` (48h/10/1200m/2) — steward seam; includes the bounded
   infrastructure-retry addendum on the goal.
3. `two-bars-for-changes` — the landing-provenance machinery; the seat
   record's foremost debt. Design next-step is on the goal.
4. `land-verb-pruning` (48h/10/1200m/2) — the disposition ledger's
   remaining INTERNALIZE/THIN-SHIM verdicts.
5. `recovery-rehearsal` fixture leg (24h/6/480m/2).
6. Small goals: `supervise-start-gate-linux-red` (the VM red),
   `steward-tick-load-flake` (reproducible against m2's own live
   runner), `kill-guard-fold-consumers`, `role-lane-packets` (encodes
   R-25 lanes into packet recipes — until it lands, carry lanes by
   hand as both machines have been doing).
7. m2's own claims: `goal-scope-bounds` (claimed, design landed),
   `ledger-attention` (design landed, released for implementation).

## Standing laws m2 must carry as the acting seat

- Slices ≤240 minutes, grace stops at 360; goal tuples are Wido's word
  at claim (his decision-asks or relayed verbatim).
- R-25 lanes: Fable designs / Sol critiques designs / Sol implements /
  Fable critiques implementations. WITH FABLE UNAVAILABLE: the recorded
  critic-lapse contingency applies in reverse — use the best available
  Claude model for the Fable lanes and RECORD the substitution on each
  affected chain; design-bearing work that can wait for Fable should
  wait (the counselor's teeth design explicitly may).
- Chain protocol, learned the hard way today: implementer round →
  `validate conformance --stage review` (persists diff.patch +
  review.json BEFORE the critique) → critique classed to carry the
  required effort (DESIGN-BEARING for maximal) with `--reviews
  <terminal round>` → `job critique-register-advance` per critic round
  → `dispatch.sh close`. Closure refuses stale or under-effort
  critiques — that is correct, not a bug.
- The gap rule is working conduct on both machines: a brief that is not
  mechanically complete gets a refusal-with-analysis, never invention.
- Landings: every gate &&-chained; the id-minting guard permits
  register REWRITES only when paired +/-; the steward's live digest
  appends must be staged into each landing (the runner writes
  continuously).
- Wido's terminal session (~45 min, when he regains any agent-free
  shell): enrollment, genesis, engine re-arm (permanent), resume leg,
  re-ratification of BOTH machines' temporary arms and every act under
  departure R-29-m1/m2, and the seat-record review confirmation.

## Watchers m1 leaves behind

m1's session monitors die with the session. The resident steward on m1
keeps ticking (health + digests). Nothing else on m1 needs tending; all
delegated chains are concluded or closed. The conclusion-belt pattern
(monitor on exit.json appearance) is m2's to re-arm on its own runs.
