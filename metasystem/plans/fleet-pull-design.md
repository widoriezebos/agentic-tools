# Review brief: fleet-pull

Round budget: 3 focused rounds — agreed before round one; exhaustion
follows the critique skills' budget rules, never a silent round 4.

Threat model: one trusted human operator; no external adversaries.
IN SCOPE: accidents and cost — launch thrash (a pull-launched
coordinator that dies instantly must not relaunch every tick),
double launches (two idle machines pulling the same goal — the
claim's compare-and-swap already arbitrates, but the loser must
idle gracefully and cheaply), runaway spending (every launched
coordinator burns real tokens), and invisibility (a launch the
human never hears about). OUT OF SCOPE: hostile input, multi-repo
fleets, the ACP seam (composes later; must not be waited for),
and the session-level dead-man switch (stays as defense in depth).

Appetite: 4h for this design; findings whose fixes exceed it pause
and go to the human.

Scope: a new duty in the steward's tick; launch through the
steward's EXISTING continuation-dispatch path (adapter seam,
roster-resolved — no runtime named in core); backoff state in the
tick's existing evidence store; an off-by-default config switch;
fixtures. OUT: any new launch mechanism, any goal-schema change.

Return format: numbered findings, most severe first, each with
file, rule, and the concrete failure it causes; or AGREE with
observations that do not gate.

---

# Design: the steward pulls work for an idle machine (revision 3)

Revision 3 folds all six round-2 findings; dispositions and the
canonical obligation matrix are at the end.

Wido's expectation, verbatim intent: new claimable items on the
shared backlog are always picked up when a machine is idle. Fleet
liveness is machinery's duty, never a human's memory.

## Seeing remote work: the fetch

The tick's projection advances only on local publications, so the
pull begins with the engine's existing validated fetch verb — the
read-side advance that verifies the canonical tip before moving
the accepted ref — run BEFORE the tick's arbitration lock, under a
30-second bound. A failed or timed-out fetch means no pull this
tick and a named fetch-degraded note in the tick result; other
duties proceed.

## The pull decision, exactly

The machine PULLS — launches one coordinator — exactly when ALL
hold:

1. ENABLED: `fleet.pull=on` in local configuration, and the pull
   state is not TRIPPED.
2. IDLE, FULLY: this machine's nickname holds no claim on the
   fetched ledger, no pull-launched job is live, AND the steward's
   own census reports no live workload — an enrolled session, a
   delegate, a mission runner, a gate, or a monitored run all mean
   the machine is working, claim or no claim. The census already
   owns this fact; the pull consumes it, never re-derives it.
3. WORK EXISTS: at least one eligible CLAIM UNIT (below).
4. NOT BACKED OFF: pullState.notBefore has passed.
5. NOTIFIED FIRST: the launch notification is delivered through
   the gated channel before the dispatch exists; notifier failure
   aborts the launch.

## Claim units and selection (arcs are atomic)

The ledger claims arcs whole, so the pull selects claim UNITS: a
solo goal, or an entire arc. A unit is eligible only when EVERY
member is queued, unblocked, appetite-tokened, and unreserved —
a unit with one ineligible member is excluded BEFORE selection,
so it can never consume attempts or starve later candidates.
Units order by (oldest member OpenedAt, id); the pull takes the
first, and the launched dispatch delegate claims a solo goal with the
plain claim and an arc with the arc claim.

## The reservation convention, made real — and proven complete

The backlog mechanism's intake section gains the convention: a
next step containing the word RESERVED is pulled by no machine.
The SAME landing migrates every live human-gated next step to
carry the word; the landing's review obligation is completeness —
each tokened queued goal was either left pullable on purpose or
marked, and the commit message enumerates the marked set. The
fixture pins the convention forever after; the one-time
completeness judgment is the migration reviewer's, named in the
matrix, because no parser can distinguish an unmarked human-gated
sentence from pullable prose.

## The launch pipeline, generalized

One launcher, two intents. The staged-intent record gains a KIND
(continuation | pull); completion is kind-aware — a continuation
still requires the revive verdict, a pull re-verifies the idle
conditions under the lock at completion time. The pull dispatches
the new fleet-pull role, whose return schema closes the
outcome field: claimed | idle-no-work | refused. THE LAUNCH
INJECTS IDENTITY MECHANICALLY: the dispatch boundary sets the
lineage to the pinned value fleet-pull, so every claim, edit, and
release the dispatch delegate makes publishes as <nickname>+fleet-pull —
the role never invents identity, and a missing lineage can never
refuse the claim before the race is even run.

## Pull state: the transition table

pullState = { consecutiveFailures, notBefore, tripped, lastGoal,
lastOutcome }, MERGED by Observe (never replaced), persisted in
the tick's evidence store. The transitions, exhaustively:

- ATTEMPT (before dispatch): consecutiveFailures increments
  provisionally, notBefore advances by the backoff for that count
  (1, 2, 4… ticks, capped at one hour), state SAVED, then the
  launch proceeds. A crash after this save reads as a failure —
  the safe direction. The fifth attempt LAUNCHES (the bound is
  five launches, not four): tripped is set only at OUTCOME.
- OUTCOME claimed: consecutiveFailures resets to zero, notBefore
  clears, tripped stays false.
- OUTCOME idle-no-work (the race loser): the provisional increment
  is REVERTED — losing a race is not a failure — and notBefore
  clears. An existing failure streak from earlier attempts is
  restored, not erased: revert means decrement by one, exactly
  undoing this attempt's provisional increment.
- OUTCOME refused, protocol error, dispatch failure, cancellation,
  or died-silent: the provisional increment STANDS. If
  consecutiveFailures is now five, tripped becomes true and the
  terminal notification queues (nonce fleet-pull-tripped).
- RE-ARM: an explicit steward verb (steward rearm-pull) clears the
  state — a persisted, auditable human act; no config-edge
  detection exists.

## What this deliberately does not do

No second launcher; no goal-schema change; no cadence change to
the session dead-man switch; no parser that pretends to judge
human-gated prose.

## Fixture obligations (the arbiter)

- F1: idle + one tokened goal + pull on → notification delivered
  FIRST, then exactly one dispatch (fake runtime); scripted
  notifier failure → no dispatch.
- F2: a held claim → no pull; AND a live delegate with NO claim →
  no pull (the census half of idle).
- F3: no eligible unit → no pull.
- F4: a RESERVED next step is never selected (convention pin).
- F5: a failure outcome advances notBefore; the next tick inside
  the window launches nothing.
- F6: `fleet.pull` off or absent → nothing, ever.
- F7: a live pull-launched job → no second launch.
- F8: idle-no-work reverts exactly this attempt's increment: a
  streak of two, then a race loss, leaves two — not zero, not
  three.
- F9: two solo candidates — the older OpenedAt wins even when its
  id sorts later.
- F10: five failure outcomes trip; the terminal notification
  queues; tick six launches nothing; steward rearm-pull clears and
  tick seven may launch.
- F11: crash between attempt-save and dispatch → next tick
  respects notBefore.
- F12: fetch degraded → no pull, named degradation, other duties
  proceed.
- F13: an arc with one reserved member is never selected; a fully
  eligible arc is claimed WHOLE by the launched dispatch delegate.
- F14: the injected lineage — every mutation the pulled
  dispatch delegate publishes carries <nickname>+fleet-pull.

## Design-obligation matrix

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| FP-O1 | CRITICAL | fleet-pull-design r3 fetch | Validated bounded fetch before the tick lock; stale reads never launch | internal/steward tick | tick fetch step | F12 + lock-ordering unit test | one live tick on a machine with a remote-opened goal | MISSING | implement |
| FP-O2 | CRITICAL | fleet-pull-design r3 pipeline | One launcher, two intent kinds, kind-aware completion with re-arbitration | internal/steward stage/revive | intent kind field + completion switch | F1, F7 + kind-mismatch refusal test | fake-runtime launch in the steward suite | MISSING | implement |
| FP-O3 | CRITICAL | fleet-pull-design r3 idle | Idle consumes the census: live workload means no pull, claim or none | internal/steward tick | census consult in the decision | F2 both halves | live-delegate bed in the steward suite | MISSING | implement |
| FP-O4 | CRITICAL | fleet-pull-design r3 identity | The dispatch boundary injects lineage fleet-pull; the role never invents identity | steward dispatch + role | lineage injection at stage | F14 | pulled claim visible in ledger history as nickname+fleet-pull | MISSING | implement |
| FP-O5 | HIGH | fleet-pull-design r3 state | The transition table, exhaustively; crash reads as failure; five launches then trip; rearm-pull verb | internal/steward evidence + verb | pullState struct + transitions | F5, F8, F10, F11 + Observe-merge unit test | a scripted five-failure trip in the suite | MISSING | implement |
| FP-O6 | HIGH | fleet-pull-design r3 notify | Delivery precedes dispatch; notifier failure aborts | internal/steward notify/stage | ordering in the launch path | F1 both directions | notification visible before the job record's timestamp | MISSING | implement |
| FP-O7 | HIGH | fleet-pull-design r3 reservation | Convention documented; live goals migrated; completeness reviewed | docs + ledger migration | intake section + migration commit | F4 | reviewer's enumerated migration list in the landing | MISSING | implement |
| FP-O8 | HIGH | fleet-pull-design r3 arcs | Selection over claim units; arcs atomic; ineligible units excluded before selection | pull selector | unit builder | F13, F9 | arc-claim by a pulled dispatch delegate in the suite | MISSING | implement |
| FP-O9 | MEDIUM | fleet-pull-design r3 role | fleet-pull role with closed outcome schema | roles + schemas | role + schema files | schema fixture + F8 | a fake-runtime pull returning idle-no-work | MISSING | implement |

## Dispositions of round 2

- FP-R2-01 FOLDED: idle consumes the census (decision 2, FP-O3,
  F2 reshaped).
- FP-R2-02 FOLDED: mechanical lineage injection, pinned value
  fleet-pull (FP-O4, F14).
- FP-R2-03 FOLDED: the exhaustive transition table — five launches
  then trip at outcome, race-loss reverts exactly one, rearm is a
  persisted verb (FP-O5, F8/F10 sharpened).
- FP-R2-04 FOLDED: claim units, arc-atomic, pre-selection
  exclusion (FP-O8, F13).
- FP-R2-05 FOLDED: the fixture pins the convention; the one-time
  completeness judgment is named as the migration reviewer's
  obligation with the enumerated list in the landing (FP-O7).
- FP-R2-06 FOLDED: the matrix above uses the gate's canonical ten
  columns.

---

# ROUND 3 STATE: budget exhausted — STOPPED AND RAISED

The first three-round budget closed at seven material findings
(FP-R3-01..07, archived under artifacts/agents/critiques/fleet-pull/),
four invariant-grade; the trajectory across rounds (7, 6, 7) says
this design is bigger than its 4h token — the third fleet-scale
design in two days to prove so. Per the budget rules the chain
remains OPEN; any successor enumerates all seven and a second
exhaustion stops for the human regardless.

Two things need Wido:

1. THE ORPHANED-CLAIM OWNER (FP-R3-01): when a pulled coordinator
   dies immediately after claiming, who resumes the claim? The
   session dead-man cannot — and the finding exposed that its
   open-work reader has been DEGRADED on converted checkouts since
   the migration, a live defect now fixed separately. The clean
   candidate is a steward duty (stale fleet-pull claims revive like
   continuations), but that widens scope beyond the brief.
2. THE RE-SCOPE APPETITE: folding the seven findings honestly means
   claim-boundary eligibility enforcement inside the goal engine's
   claim verbs, machinery-adjudicated outcomes, attempt nonces, and
   two registry joins — a full day, matching the o14/o19 pattern.

Recommendation: re-scope at 1d with the orphaned-claim owner ruled
INTO the steward's scope (one owner for all launched-work liveness),
after bm-2d lands and the ACP seam's session events are in sight —
they make outcome adjudication (FP-R3-02) nearly free.
