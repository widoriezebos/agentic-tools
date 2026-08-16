# The process steward (backlog item 21)

- Status: DRAFT r1 — awaiting critique r1
- Goal: process-steward
- Next step: Fold the critique verdict when run ps-critique-r1 concludes; implement only after convergence.

## The human's mandate (2026-08-16, verbatim intent)

"We probably need something that watches the process itself and
signals issues or potential issues so that the orchestration agent
can act on them, or maybe even the process watcher itself... more
like a process coach... something better than a retro... add this
one to the backlog and come up with a proper name for it." Named
the PROCESS STEWARD: it both NOTICES (watches the development
process against expectations) and TENDS (signals or acts).

## What it watches: process invariants, not code

A retro looks back after work; the steward watches WHILE work runs
and catches deviations early. It watches the PROCESS invariants —
facts that should hold about how the development is being conducted
— distinct from the product invariants the suite already checks.
The motivating deviations, all real this program:

1. **Dead supervision.** The supervision fleet died and nothing
   noticed for hours until the human read a Stop message. Invariant:
   while a session is active, supervision is armed AND attesting
   (a fresh runs-pass.json within the interval).
2. **Leaked-process compounding flake.** Fixture children leaked and
   accumulated until the reaper missed its window. Invariant: no
   process rooted under a finished run's temp namespace outlives it;
   the leaked-process count trends to zero, not upward.
3. **Promised work with no leash.** A turn ended with "I'll resume"
   and nothing armed to resume it. Invariant: no plan claims work
   in flight while no run/monitor/background task is pending for it
   (the Stop hook already enforces a slice of this — the steward
   generalizes it and watches the trend).
4. **Run-ledger hygiene.** Runs left draining/unacked; greens with
   "no continuation recorded." Invariant: terminal runs get
   concluded and acked; the unacked count trends down.
5. **Critique/gate discipline.** A ship without its mandatory code
   critique; a commit past a red gate. Invariant: every ship on the
   branch has a preserved critique and a green gate witness.

## Shape: a mechanical watcher plus a coach role

Two cooperating parts, mirroring the human's "watcher itself acts,
or signals the orchestrator":

- **The steward CHECK (mechanical, Go).** `steward check --root`
  evaluates the invariants above from records the metasystem already
  keeps (supervision attestations, run ledger, process table vs the
  temp namespace, plan in-flight markers vs pending runs, the branch's
  critique files vs its ship commits). It emits a typed VERDICT per
  invariant: ok | drift | breach, each with evidence and a suggested
  action. It NEVER acts destructively — it reports. Runs cheaply on
  a cadence (a supervision component, like the reaper/watcher) and on
  demand.
- **The steward COACH (agent role, cheap model).** When a check
  returns drift/breach the orchestrator can't self-resolve, a coach
  turn (a delegate on a cheap model, the [[narrator]]'s sibling)
  reads the verdict + recent flight-recorder context and produces a
  plain-English SIGNAL to the orchestrator: what deviated, why it
  matters, the smallest corrective action. The coach distills; it
  does not re-derive the invariants (the check owns those).

The division: the CHECK is the authority (mechanical, records-based,
runtime-neutral — no agent name); the COACH is the accelerator
(latency: turns a breach into an actionable human-readable nudge
faster than a human reading logs). This mirrors the accelerator
ruling — correctness lives in the check's records, the coach only
speeds the human/orchestrator's response.

## What it does on a breach

Escalation ladder, least-invasive first: (1) emit the verdict to the
run/flight record (always); (2) surface a SIGNAL the orchestrator
sees at its next turn boundary (the Stop-hook channel already exists
— the steward feeds it); (3) for a NARROW, SAFE, pre-declared class
of deviations (e.g. an unacked terminal run, a leaked temp-namespace
process past its run's death), the check MAY act — the same bounded,
proven-safe reap the fixture-leak fix now does — but only within an
allowlist of reversible mechanical corrections, never a destructive
or authority-touching action. Everything else signals and waits.

## Boundaries

- Runtime-agnostic: the check names no agent runtime; the coach is
  an adapter-seam role like the narrator.
- Not a second retro: the retro looks back and records lessons; the
  steward watches live and catches drift. They compose — a breach
  the steward caught becomes a retro line.
- Not the suite: the suite checks the PRODUCT; the steward checks the
  PROCESS OF PRODUCING. Different invariants, different cadence.
- Defense-in-depth honesty: the steward reduces the time-to-notice
  for process drift; it is not a guarantee against it (a wedged
  steward is itself a drift — its own attestation is an invariant the
  supervision watcher checks, closing the who-watches-the-watcher).

## Prototype plan

P1: the check's invariant set as Go over existing records (supervision
attestation freshness, run-ledger unacked/draining trend, temp-namespace
orphan scan, plan-in-flight vs pending-run join, ship-vs-critique join),
each a pure function with fixtures. P2: the `steward check` verb + a
supervision-component cadence. P3: the coach role behind the adapter
seam on a cheap model; the signal channel into the Stop-hook surface;
the narrow safe-act allowlist.

## Loop discipline

Critique at codex xhigh; the critique should attack: whether each
invariant is checkable from records that actually exist (name any that
need new instrumentation); whether the safe-act allowlist can ever
reach a destructive or authority action; whether the steward's own
liveness is watched (who-watches-the-watcher); whether the check stays
runtime-neutral; and whether "drift vs breach" thresholds are
principled or arbitrary.
