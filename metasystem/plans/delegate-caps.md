# Budgets belong to the binding: per-model delegate caps

- Goal and current status: a delegate job's time budget is declared where
  its runtime and model are declared — keyed on the (runtime × model)
  pair, optionally sharpened per role — instead of one uniform cap for
  every delegate. DESIGN DRAFT, 2026-08-09, from the human's direction
  after the bm-2 post-mortem; not yet critiqued.
- Next step: critique this design with sol
- In flight right now: nothing
- Waiting on the human: nothing

## Why, with the evidence

bm-2's uniform 15-minute job cap killed a Devin/swe-1-7 implementer that
had produced 11 Java files — 1,322 COMPILING lines — mid-flight. The work
was discarded, the mission shipped a skeleton (acceptance 1/53 in both
repetitions), and the delegation-floor gate correctly recorded that no
implementer job was ever certified. Not the model (productive), not the
CLI (functioning), not a harness bug (everything behaved as configured):
the CONFIGURATION was the defect. A cap sized for claude/codex pacing
truncates a runtime whose wall clock is 91–98% inference.

The human's rulings, verbatim in effect:
- the cap keys on the (runtime × model) PAIR, "not necessarily per agent —
  that might be too crude; Devin on a very fast model will behave
  different from Devin using a very slow or poor model";
- "where we define what agents and models to use per role, that is also
  where we will have to specify the cap";
- the Devin number is DISCOVERED, not guessed: one deliberately very high
  cap, one closely monitored run, the observed natural completion time
  plus margin becomes the standing cap.

## D-1. Declaration: the cap lives beside the binding

Two surfaces declare bindings today, and each gains the cap in place:

- HARNESS CONFIG (`metasystem.conf`, resolved by `metasystem-config.sh`
  with `.local` and env overrides exactly like every other key):
  `cap.min.<runtime>.<model>` — e.g. `cap.min.devin.swe-1-7=90` — with an
  optional role-sharpened form `cap.min.<role>.<runtime>.<model>`. Model
  names use the canonical form the adapters already record (the
  `swe-1-7` canonicalisation exists).
- BENCHMARK ROSTER (the spec manifest): each roster delegate entry may
  carry `"capMin": <int>`, and the host entry likewise for turn caps.
  The manifest is sealed into the contract, so a cohort's caps are part
  of what the human signs.

## D-2. Resolution: most specific binding wins, once, at dispatch

Dispatch resolves the cap when it creates the job record, stamps it as
`capMin` (the field every fence and reaper already reads), and NOTHING
downstream changes — the reaper still judges `startedAt + capMin`, the
mission fence still meters `job-cap-min`. Precedence, most specific
first, decided here once:

1. an explicit `--cap-min` argument (existing; used by fixtures and the
   mission runner's roster-driven dispatches),
2. `cap.min.<role>.<runtime>.<model>`,
3. `cap.min.<runtime>.<model>`,
4. the mission fence's `fence.job-cap-min` (the contract's uniform
   floor-setting, now explicitly a DEFAULT rather than a universal),
5. the current built-in default.

A roster `capMin` reaches dispatch as the mission runner's `--cap-min`
(rule 1), so the benchmark surface needs no new dispatch flags. The
resolved value and WHICH rule produced it are recorded on the job record
(`capMin`, `capMinSource`) — provenance, not inference, when a scorecard
asks why a job had the budget it had.

## D-3. Fences stay sovereign

A per-binding cap does not weaken the mission fences: `fence.jobs`,
`fence.wall-clock-hours`, and `fence.concurrency` still bound the
mission, and a cap LARGER than the remaining mission wall clock is
truncated to it at resolution time (recorded in `capMinSource` as
`fence-truncated`). The cap experiment therefore needs a spec whose
wall-clock fence accommodates one long implementer job — the experiment
raises the fence deliberately rather than letting the fence silently
become the cap.

## D-4. The discovery experiment (one run, instrumented)

A bm-2-derived spec (`bm-2c`, cap-discovery) with: roster
`devin: swe-1-7` delegates carrying `capMin` effectively unbounded within
the mission (e.g. 150), `fence.wall-clock-hours` raised to match, ONE
repetition, and the same gate and grader. The watch samples progress —
worktree churn plus session state until adapter streaming lands — and the
deliverable is the OBSERVED time-to-completion distribution of implementer
jobs, from which the standing `cap.min.devin.swe-1-7` is set with margin
and recorded as a human ruling in the roster. If jobs do NOT complete even
unbounded, that is the answer the human asked for ("whether it can
complete at all"), and the comparison cohorts decide what replaces Devin
as implementer.

## D-5. What does not change

- `capMin` on the job record, the reaper's budget arithmetic, the fence
  metering, and the timeout verdict semantics (budget-cap outranks
  process-lost for jobs that ran).
- Roles without a specific binding: the existing default chain applies
  unchanged.
- Host turn caps: the same declaration surfaces exist (`host.turn-cap-min`
  in the mission block already; the roster host entry may carry it), but
  host turns are missions' business and their resolution is already
  contract-driven — this design only ADDS the per-model config form for
  symmetry, it does not move authority.

## Proof

- Resolution precedence: fixtures for each rule of D-2, including the
  role-sharpened form beating the pair form, the pair form beating the
  fence default, and explicit `--cap-min` beating everything.
- Provenance: a dispatched job's record carries `capMinSource` naming the
  winning rule; the fence-truncation case records `fence-truncated` and
  the truncated value.
- Fence sovereignty: a cap larger than remaining wall clock truncates; the
  mission fence still ends the mission on schedule with the long job
  reaped as budget-cap.
- Config hygiene: an unparseable or non-positive cap value refuses loudly
  at dispatch (the tier-gap lesson: config errors are errors, not silent
  defaults).
- Roster flow: a manifest capMin reaches the job record byte-identical
  through the runner's dispatch, and the sealed contract's exposure
  statement reflects it.
- End to end: the bm-2c experiment spec provisions, seals with its raised
  fences visible in the contract, and one unbounded implementer job runs
  to a natural end under the watch.
