# The fuse counts landed work, not just moved numbers

Working Mode: design

Owner: main session (delegate), under Wido's 2026-08-14 night rulings
5 and 6 (middle path; may implement tonight ONLY if this critique
loop converges). Status: WITHDRAWN at r1 — the critique (codex
gpt-5.6-sol xhigh, 14 findings, 4 critical, verdict REVISE; output
preserved in the D56 entry) refutes the premise, not the polish: with
budget 3 the park fires before a cycle 4 exists, so the marker could
never have rescued the observed empty parks (finding 1), and the
evidence demonstrates a host-scheduling defect, not a fuse-grammar
defect (finding 14). Sealed-semantics, transaction-invariant,
replay-purity, and farming findings (2-5) would sink the mechanism
even with a correct premise. The fuse stays as contracted; budget 5
and D55's warning are the standing defenses; the REAL question —
why some hosts serialize critique ahead of any implementer dispatch,
and whether the contract or prompt should force early implementer
work — moves to the item-14 design pass.

## The problem, with tonight's evidence

The stop-loss fuse (docs/design/stop-loss-core.md) counts a cycle as
no-gain unless a gate metric set a new best. Gate metrics move only
when work lands on the default branch. So a mission doing real,
certified delegated work that has not yet moved a metric is
indistinguishable from a mission doing nothing — the fuse measures
the lagging indicator only.

Cohort bm-1-20260814t192803z-44271 (engine 387c961, budget 3), job
rosters read from the frozen targets:

- Reps 1 and 2: three design-critic jobs each (rounds 1-3), all
  completed, ZERO implementer jobs ever dispatched. The critique
  cadence exhausts at round 3; the budget was 3; a host that
  serializes critique-then-build is fused at the exact moment it
  would begin building. Both parked empty: acceptance 0.018868
  byte-identical, ~$9 each.
- Rep 3: the host dispatched its first implementer 27 seconds after
  its first critic (parallel streams), work landed inside the
  window, VALID at acceptance 0.962 — including surviving one
  implementer killed at its budget cap whose follow-up resumed from
  evidence.

So the bimodality is not model speed; it is host serialization
meeting a fuse that cannot see pre-merge progress.

## What already mitigates, and why it is not enough

- Budget raised 3 → 5 (HUMAN RULING 3): gives a serialized host two
  post-exhaustion cycles. A constant racing another constant — the
  cadence could grow, rosters could add a second critic chain, and
  the trap re-arms silently.
- D55's seal-time cadence warning: a tripwire, and tonight proved
  warnings alone do not prevent (the half-fence warning fired on
  every provision).

## Design: one new gain source, same pure replay

The fuse stays a pure function of the ledger (stop-loss-core C1).
Change the GRAMMAR of gain, not the mechanism:

1. **The runner appends a work-landed marker.** When a cycle's
   conclusion observes at least one implementer job that reached
   status completed with certification in that cycle, the runner
   appends `- Work landed: <jobId>[, <jobId>...]` to that cycle's
   ledger block, next to the existing measurement lines. The runner
   already owns the cycle block append; this is one more line from
   facts it already holds (the job records it reaps and accounts).
2. **The replay counts it as gain.** Stagnation is the count of
   consecutive concluded cycles with neither a new metric best nor a
   work-landed line since the last best/reset. One-line change to the
   replay's cycle classification.
3. **Bounded, so laundering is impossible.** A work-landed line only
   resets stagnation ONCE per distinct jobId (jobIds are unique, so
   this is free), and only for implementer-role jobs that are
   completed AND certified — the same certification the delegation
   floor demands, so a host cannot farm resets from husks, repairs,
   or critic completions. Metric bests remain the only thing that
   updates bests; work-landed lines never touch metric state.
4. **The contract keys do not change.** ledger.no-gain-budget keeps
   its name and meaning: cycles without progress. What broadens is
   what counts as progress, which is a contract-text amendment to
   docs/design/stop-loss-core.md (C1's gain grammar) plus the ledger
   grammar addition.

## Why not the alternatives

- **Raise the budget further**: pays real dollars per empty park to
  avoid a one-line grammar fix; leaves the lagging-indicator blind
  spot for every future mission shape.
- **Count ANY completed job as gain**: critic completions would reset
  the fuse — reps 1 and 2 would have run to the cycle fence at ~3x
  the cost, producing the same nothing. The certification bound is
  the teeth.
- **Count dispatched (not certified) implementers**: a host could
  reset the fuse by dispatching doomed jobs; certification is the
  existing, already-enforced quality gate.
- **Make the fuse aware of the critique cadence** (skip counting
  during critique rounds): couples the fuse to dispatch policy;
  work-landed keeps them decoupled and the fuse pure over the ledger.

## Blast radius

- internal/mission: ledger grammar accepts/writes the new line;
  replay classification. The flock/atomic write discipline is
  untouched.
- internal/missionrunner: the conclude step appends the line from
  job records it already reads; stoploss.go replay change.
- docs/design/stop-loss-core.md: C1's gain grammar amended, with
  this plan named as origin.
- scripts/assert-stop-loss.sh: its non-mission grammar is unchanged
  (the new line is additive; the existing greps do not anchor on
  block-line exhaustiveness — verify in implementation).
- Fixtures: the mission end-state and runner fixtures gain one case
  (a cycle with work-landed and no best does not count toward
  stagnation); the fuse-behavior unit tests keep their tiny budgets.
- Benchmarks: cohorts run on the new fuse are labeled by their
  engine sha as always; the manifest budget stays 5 (the two
  mechanisms are independent defenses).

## Proof obligations

- Unit: replay table tests — work-landed resets stagnation; critic
  completions do not; uncertified implementer completions do not; a
  work-landed line for a jobId already counted does not reset twice;
  metric bests still update independently.
- Ledger grammar: append + re-read round-trip; the flock discipline
  test extended to the new line.
- Fixture: one runner-level case driving a serialized-shape mission
  (critic cycles first, implementer landing in cycle 4) to a
  non-park conclusion under budget 3.
- Boundary: full suite green on the VM before any benchmark rep runs
  on the new fuse.
