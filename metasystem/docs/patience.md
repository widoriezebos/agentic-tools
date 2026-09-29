# Patience, progress, and stall

Status: SHIPPED — the whole mechanism is implemented. The stop-loss
core (`docs/design/stop-loss-core.md`, `internal/missionrunner/stoploss.go`)
and all four satellites (turn identity, reap/drain, orphan/usage and
patience; see "The mechanism, whole" below) are built and suite-verified;
satellite 4 (patience floors, contract under "The satellite contracts"
below, accepted after a 22-round critique loop) landed 2026-08-12 in
`internal/missionrunner/patience.go`. This document names the concepts,
explains the whole mechanism, and carries the satellites' contracts.
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
narrative a drought hides behind (satellite 4, r1/P4-001 through
r2/P4-024). Some activities need no progress
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
   (S4 below): a chain round is WITNESSED
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

## The satellite contracts

What the shipped code relies on and its comments cite. Each satellite
closed its own critique loop in August 2026; the design records were
removed on 2026-09-28 after this distillation (tag
`records-archive-2026-09-28`).

### S1 Turn identity (internal/missionrunner/adjudicate.go, cycle.go)

- turn.json records `announcedSession`, the prompt's Host-Session hint
  derived from the previous concluded turn, which can be stale. It also
  records `observedSession`, which the harness observes: the
  session-established signal where the runtime declares one, else the
  adapter's terminal result envelope. The return's own claim is never
  the source.
- A return's session is accepted when it equals either of the two. If it
  matches neither and an observed session exists, that is a protocol
  violation: the turn fails and feeds the breaker. If it matches neither
  and there is no witness, the return is not applied and nobody is
  blamed: the turn is annotated and the breaker is not fed.
- One application rule: a return's state mutations (stream transitions,
  asks, the waiting list) apply only when the return is accepted.
  Measurement effects (classification, gatePassed, completion) always
  conclude from the measured tree. A rejected or capped turn still
  drains, then measures, then concludes. A measured gate pass completes
  the mission with the fault recorded.
- The adapter is a witness, not a judge. It reports a session rotation
  and never fails the turn for it. A missing session keeps exit 6 and
  feeds the no-witness branch.
- The next announcement derives from the last concluded turn's
  observedSession. `outcome=capped` is never rewritten to `failed`.
- Annotation lines (`- Return: rejected:<reason>`, `- Outcome: capped`
  and every later kind) sit beside the classification line, never
  inside it. They are audit trail, never fuse input.
- Rejected: suffixing the classification line, which broke the strict
  parsers. Also rejected: keeping the adapter's rotation hard-fail
  beside adjudication, because two independent checks punished a
  truthful host; there is one verdict site, with witnesses feeding it.
  Pinned by the adjudication matrix in
  `internal/missionrunner/cycle_test.go` (TestConcludeFaultedTurnEmptyVerdict
  and the no-witness rows).

### S2 Mission reap and bounded drain (internal/missionrunner/drain.go, answer.go)

- The runner may fail only records that its own mission's fence
  reservations name, and only on the standing reaper's proof bar:
  reap-facts plus kernel-custodian death (Unknown is never death), or a
  never-launched setup husk past the abandoned-setup grace.
- Verdicts, in this order: budget expiry books timeout/budget-cap;
  a husk books failed/abandoned-setup; a dead custodian books
  failed/process-lost. An expired handshake with no recorded process is
  not reapable. No new graces are introduced.
- The drain deadline is recomputed each pass over the current active
  set (latest capDeadline plus handshake grace, with per-record
  fallbacks). Each pass reaps before it waits and beats the runner
  heartbeat. At expiry the runner parks `drain-stalled` with one ask
  that names the survivors.
- The park writes no ledger line. It writes state, then the ask, and
  resume re-raises a missing ask idempotently.
- A `resume:` answer unparks and writes `lastDrainStall {cycle,
  survivors}`. In the same write, the reserve/append heal consumes that
  field into `no-progress; observed=unmeasurable:drain-stalled` plus
  `- Drain: stalled:<n>`. A gap without a matching field heals as plain
  turn-lost.
- Accepted loss: the stalled cycle's verdict and measurement. The
  committed tree banks at the next successful measurement.
- Rejected: same-cycle verdict salvage (claims, entry modes, resumed
  conclusions). Three critique rounds found only its crash windows while
  the reap half stood, so it was severed. Rebuild it only if trial
  evidence shows the one-cycle attribution delay matters.

### S3 Landed returns and usage capture (internal/mission/landed.go, fence.go)

Why: the bm-2s trials (2026-08-10) orphaned about 1.26M paid input
tokens of completed critic work, and capped jobs recorded `usage: null`.

- `## Landed Returns` is derived at every prompt assembly as a pure
  function of the tree and the turn log, with no surfacing state.
- A round is listed when all three hold:
  - its return exists (a return that fails validation or cannot be
    read is listed as `invalid` or `unreadable`);
  - no concluded turn certified it or dispatched a successor in its
    chain;
  - the mission owns the chain.
- Each chain gets one row, sorted by (root, round). Above 20 chains the
  list is 19 rows plus an `overflow` row.
- Chain closure does not retire a row. The runner closes chains at every
  park, so a closure exclusion would drop the return at the very park
  that orphaned it. Only the host's own recorded action retires a row.
  Pinned by TestLandedReturnsIsAPureFunctionAcrossParks.
- Terminal delivery happens only at the completion conclude, as
  `- Landed unconsumed:` lines appended to the final cycle block. The
  failure ramp writes no ledger line because it has no safe position;
  resume lists the rounds again.
- The adapter is the only writer of a round's usage.json. For a capped
  or killed round the aggregator derives usage from events.jsonl in
  memory and never writes it back. It derives only when the record is
  terminal, the recorded pgid probes ESRCH, and every custodian is
  dead: a terminal record's group can still be writing the event
  stream. A round with no recorded pgid stays `unavailable` forever.
  Pinned by TestAggregateUsageGroupDeathGate.
- Per-round provenance goes into usage.json `rounds`: reported,
  derived, pending-death-proof, or unavailable.
- AggregateUsage runs before every ProjectFences at conclude and park,
  and on the failure ramp. A failure emits `aggregation-failed` and
  never fails the park, the conclusion or the exit. An unchanged result
  is not rewritten.
- Rejected: recorded surfacing state with ledger-line delivery. It
  over-marks, and the ledger tail never reaches the prompt. Also
  rejected: gating derivation on terminal status alone, for the reason
  above.

### S4 Patience floors (internal/missionrunner/patience.go, internal/mission/patienceprompt.go)

The observable and the floors are described under "The mechanism,
whole" above. The contract points the code cites:

- Four annotation forms share one write and read grammar and have a
  round-trip test:
  `- Patience: chain=<root> rounds=<n> floor=<m>`,
  `- Patience: orphan=<id> rounds=<n>`,
  `- Patience: excluded=<count>`,
  `- Patience overflow: chains=<count>`.
- They ride the same AppendCycle as the cycle line, at every booking.
  At most 20 lines are written (19 plus overflow), ranked by breach
  distance. The excluded line comes on top of that.
- The next prompt's This Turn lines are a pure function of the final
  cycle block's Patience annotations plus the current chain-closed
  flags. Detail lines for a closed chain are dropped. The overflow and
  excluded lines name no chain and are exempt from that filter.
- Only clean records participate: readable, mission-owned, id equal to
  the filename stem, and a known status.
- A counted job is terminal, unwitnessed and provably started. Started
  means completed or timeout, or spend-proving usage, or an
  effectiveModel with an error outside the never-started vocabulary.
- The count is the drought since the newest witness.
- Accepted lag: a crash between AppendCycle and the state write books
  one booking late. The all-streams-inactive park carries no ask, and a
  parked mission cannot grow the drought.

## Waits in tests: attempts, not wall-clock

A test waiting on an asynchronous actor's effect counts the actor's
attempts, not seconds. This landed on 2026-08-19 for KI-37, and R-35-m3
makes load-slowness-as-failure a defect.

- Tier 1: succeed on a provably fresh attempt; fail after K fresh
  attempts that are still wrong (K small, default 2). Load makes each
  attempt slower but never makes more attempts necessary.
- Tier 2: a generous silence failsafe, "no attempt completed for T". It
  is labelled as a wedge or one slow attempt, never "actor silent". T
  derives from the actor's own published cadence.
- The attempt marker lives in the published artifact, not in the
  process, because the actor is relaunched routinely and a counter in
  its process goes backwards exactly under load. It is seeded from the
  last published value and advances only on a successful publish:
  census `scanSeq`, pinned by
  TestWatcherPassScanSeqMonotonicAcrossFreshConfigs and
  TestWatcherPassScanSeqOnAllVerdictPaths.
- Freshness: consider an actor that runs one pass at a time, reads its
  input during the pass and publishes at the end. Read the marker S1
  after planting the state. A published marker of S1+2 or higher proves
  that a pass saw the planted state. Read the marker and the predicate
  from one snapshot. This holds only for level-triggered predicates.
- A wait on an OS event such as pid death has no attempt to count, so a
  generous wall-clock timeout is honest there. A wait whose progress
  fingerprint carries no intermediate state is a timeout and must be
  called one.

## The recursion caution

Patience is itself a fuse, so the last-defense ruling applies to it
recursively: a floor calibrated as a pacing target rebuilds the original
trap one level down, per model. When in doubt, a floor is too low, a
window too long, and the human reset carries the rest. The reference
failure and its analysis: the section "Why cycle counting was wrong" above
and the recorded precedent in `skills/design-critique/SKILL.md`.
