# Built work lands beside the seat's claim, and a re-claim keeps its episode (goal land-ready-work-lands-without-a-claim-slot)

- Status: revision 3 after two design reads (section 6); the two budget-law choices of section 7 are taken as the second reader advised and put to Wido at the landing; built behind the fixtures in section 4 (2026-09-12)
- Goal: land-ready-work-lands-without-a-claim-slot (goal 20 of plans/delivery-efficiency-plan.md)
- Next step: build, one code read, land; Wido's word on section 7

## What is true today (read in the tree)

A machine holds one claim at a time: `ValidateTree` refuses a tree in which
one machine claims two goals outside one arc (internal/goal/validate.go, the
quota block), and the only exemption is a breach-stopped claim, which waits
on a human and must not keep the machine from the next item
(`IsFencedClaim`, mirrored by the frontier's `Fenced` list in
internal/goal/project.go and by dispatch admission). The frontier's
`SelectNext` continues a held claim before any ready goal, so a seat whose
one claim is a finished goal waiting to land has nothing else to take.

"Land-ready" is a phrase seats write into next-step lines; no record carries
it, so the audit's 22 to 35 hours from land-ready to landed came from prose
and commit times, and the field measure this goal asks for cannot be read
from the ledger.

A release clears the claim binding entirely (`clearClaimBinding`: the claim
record, the stop capability and fence, the obligation). A re-claim, by the
same pair or another, binds a fresh episode: `bindClaim` sets
AccountingRevision to the new goal revision and EpisodeAt to now
(internal/goal/verbs.go). The budget projection counts attempts and minutes
against the claim's AccountingRevision and measures elapsed from EpisodeAt
(internal/dispatch/budget.go), so a same-pair re-claim starts the box afresh
and the spend before the release no longer counts. Retained proof no longer
suffers: since db58ad931 (goal retained-proof-reuse-crosses-claims-and-
attempts) a group is reused from the newest observation of its identity on
the seat whatever goal or attempt produced it, so the second half of the
audit's finding is already closed and this page does not touch identity or
receipt acceptance. Only `goal set-budget` rebinds a claim while keeping its
episode (`rebindClaimKeepEpisode`).

## 1. The landing slot

**The record.** A claimed goal whose work is built and verified enters
landing through the seat's own verb:

```text
metasystem goal land-ready --root . --id <goal>
```

The verb writes `- Landing: at=<iso> opid=<opid>` on the goal file and a
`land-ready` history line. It is the claim holder's act (own pair, no
human); it refuses a goal the pair does not hold, a breach-stopped claim,
and a goal already in landing (nothing to do). The goal stays `claimed`, so
its own proof attempts and receipts still bind to its claim revision and
its accounting revision.

**The budget of a landing goal.** Attempts, reserved minutes and active
jobs keep counting against the landing goal's own box (a receipt is a proof
attempt). Its elapsed clock keeps running and stays visible, but a landing
wait suspends the elapsed fence: the admission seat walk and
`FindBreachStops` skip landing claims (no elapsed stop, no cancellation, so
`done` meets no fence), and the landing goal's own revision admission
ignores the elapsed dimension while the other three still bind. Once
elapsed passes the box the turn verdict and channel report print
`LANDING OVERDUE <id>` so the wait is a number, never a hidden stop (the
line reads the claim's own episode clock less its idle seconds; a consumed
discharge that advanced dispatch's start can only make dispatch's elapsed
smaller, so the printed overdue is a floor on the wait, never a fence). A
corrupt-over-limit stop can still fence a landing claim; a fenced landing
claim lists under `Fenced`, not `Landing`. This is the first choice put to
Wido (section 7).

**The quota.** The one-claim-per-machine rule counts a machine's claims
that are neither breach-stopped nor in landing, so a seat may hold one
working claim beside one landing claim. At most one landing slot per
machine: two claims with `Landing` records under one machine refuse at
validation, a fenced landing claim included (the resume restores its slot,
so the slot is held while it waits on the person), and a seat with a
landing goal that finds a second built goal lands the first before
entering the second.

**Every consumer of "the machine's one live claim".** The fenced exemption
is mirrored in seven places; each gets its landing rule here, so no reader
guesses:

1. `ValidateTree` (internal/goal/validate.go, the quota block): landing
   claims are exempt from the one-claim count and capped at one per
   machine.
2. `Next` and `SelectNext` (internal/goal/project.go): a claimed goal with a
   `Landing` record lists under a new `Landing []string`, not under
   `Claimed`; `SelectNext` continues a working claim first, otherwise the
   first ready goal; a landing goal never blocks the next claim.
3. `EvaluateGoalAdmission --stop-lineage` (internal/dispatch/admission.go)
   skips landing claims as it skips fenced ones, and `FindBreachStops`
   (internal/dispatch/stop.go) routes no stop for a landing claim, so the
   landing goal's box never closes the working claim's dispatch and no
   fence lands on the landing goal; the landing goal's own attempts are
   admitted through the revision seam
   (`EvaluateGoalRevisionAdmissionForDispatch` with the goal named) against
   its own attempts, minutes and active jobs, the elapsed dimension set
   aside for it.
4. `convertedGoalFacts` (internal/goal/turnverdict.go, walking
   `OrderedOpenGoalIDs` rather than the map) and the current-goal lookup
   (internal/goal/goalverbs.go): the working claim is the current goal; a
   landing goal alone still resolves, in landing.
5. `uniqueActiveProofGoal` (cmd/metasystem/proof_run.go, used by `test run`
   and `proof run` without `--goal`): a landing goal alone resolves to it;
   a working claim beside a landing claim stays ambiguous ("pass --goal"),
   because both are live proof targets and a silent default would bind the
   landing receipt to the working goal's box and accounting revision.
6. Resume's quota mirror (internal/goal/stop.go): a landing claim does not
   count as the live claim that blocks a resume.
7. `readClaimableBudgetedWork`, the steward's open-work classification and
   revive (internal/goal/project.go, internal/steward/openwork.go,
   internal/steward/revive.go), and the idle escalation
   (internal/goal/turnverdict.go): landing claims travel in a
   `Landing` list beside `Claimed`, joined to liveness and revived like any
   held claim of a dead seat, but never chosen as the continuation target
   when the seat also holds a working claim; the turn verdict shows
   `LANDING <id>: land-ready since <at>; the queue is open` as live work,
   never as idleness, and the channel report lists it beside fenced claims.

**Leaving the slot.** The record lives with the claim binding, in one
place: `clearClaimBinding` drops it (done, park of any kind, release,
set-arc's source release, arc cascades, the blocker park, split's parent
and every reconcile replay that clears a claim), and a fresh-owner
`bindClaim` drops it (steal, arc joins); `rebindClaimKeepEpisode` keeps it,
so a human `set-budget` on a landing goal keeps the slot, and `resume`
keeps it too (the owner does not change when a person lifts a fence, and
the work is still built). `ValidateTree` refuses a `Landing` record on a
goal that is not claimed. Recovery replays
`land-ready` from its journaled intent like every verb, with the replay
clock, as every verb does.

**Hand edits.** `Landing:` is written by the verb. At reconcile a hand edit
may remove it only where the mapped state verb removes it (a hand park of
a claimed goal, which already clears the claim line; a hand done, whose
replay clears the claim binding); a hand done that leaves the line in
place is the ordinary unchanged copy, so the per-file check refuses a
`Landing` record only where no claim record stands beside it; every other
change to the line refuses.

**The measure.** The `land-ready` history line survives into the archive;
the hours from it to the landing (the RECEIPT line's time, or the landing
commit's) are the seat-side measure this goal asks to see under 4 over a
week. Hours to `done` are a program measure the slot bounds from below:
every program goal is human-origin and concludes only under a person.
`goal list` prints `landing since <at>` on a landing goal.

## 2. A same-pair re-claim keeps its episode

**The record.** When the own pair releases a claimed goal, the goal keeps

```text
- Episode: machine=<m> lineage=<l> accountingRevision=<n> episodeAt=<iso> episodeRevision=<r> [episodeObligationRevision=<o>] idleSeconds=<s> released=<iso>
```

the claim record's episode facts, the obligation revision the episode
consumed, the idle seconds already excluded from its elapsed clock, and
the release time. The record lives only on an unclaimed live goal (a
claimed goal carries its episode inside the claim record); a claimed goal
with an `Episode` record refuses at validation.

**The idle offset.** The claim record gains `idleSeconds=<s>`: the time
nobody held the goal during the episode, excluded from the elapsed clock.
`ProjectBudget` (internal/dispatch/budget.go) measures
`Elapsed = max(0, now - budgetStartedAt - idleSeconds)`, where
`budgetStartedAt` is the episode start or the latest consumed discharge
that advanced it, as today; the idle seconds are subtracted after that
choice, and only when `budgetStartedAt` precedes the current claim time
(every gap ends at the re-claim, so a discharge consumed inside the current
hold starts a window with no gap in it and nothing comes off). One summed
field cannot split a discharge that fell between two gaps of one episode;
that case over-forgives the gap before the discharge and is named to Wido
(section 7).
`EpisodeAt` itself stays the history event time `ValidateClaimRevision`
binds it to. `rebindClaimKeepEpisode` (set-budget) preserves `idleSeconds`;
the consumed-proof filter keeps using `EpisodeAt`.

**The re-claim.** `claim` by the same pair (machine and lineage equal to the
record's) binds the new claim revision as today but restores
AccountingRevision, EpisodeAt, EpisodeRevision and EpisodeObligationRevision
from the record, adds the gap (`claimAt - released`) to `idleSeconds`, and
clears the record. The attempts and minutes spent before the release keep
counting against the box; the retained attempts under that accounting
revision stay the goal's own (a pre-release failure can therefore demand a
retry decision, as within one episode); the elapsed clock excludes the gap.

**Which parks keep it.** An own-pair `park` writes the Episode record as
`release` does: park-then-unpark-then-claim by the same pair is a same-pair
re-claim in the goal's own words, and dropping the record there would hand
the seat a one-verb-pair reset of its box. One rule serves every verb that
ends or pauses a hold (`leaveOrDropEpisode`): the own pair acting for
itself keeps the episode, on a claimed goal from the claim record and on an
unclaimed goal that already carries the pair's record (release, then park,
then unpark, then claim continues the box); the arc cascades (`park --arc`,
`release --arc`) and the park a seat's `--blocks` open records keep each
member's episode the same way. A human park, `unapprove`'s park, a foreign
park and a steal drop it; `unpark` keeps it. This is the second choice put
to Wido (section 7).

**What drops the record.** A claim by another pair, a steal, a set-arc
(its source release and its join), a claim-arc join, a reconcile join, an
`approve` that sets the budget or the norm (`set-budget` acts on claimed
work only, where no record lives), `unapprove`, `done` and a park that is
not the own pair's: a different
owner, a fresh bind through any path other than the same pair's `claim`, a
scope move, or a human's budget act starts a fresh episode as today. At
reconcile a hand edit may remove the record only where the mapped state
verb drops it, and a hand-created goal carrying `Landing` or `Episode`
lines is refused like every generated field; every other change refuses.

**Invariants.** `ValidateClaimRevision` holds unchanged: `EpisodeAt` still
equals its history event's time and precedes the claim time. A goal claimed
with an `Episode` record, or an `Episode` record whose `released` precedes
its `episodeAt`, refuses at validation; a release or park stamped before
the episode began (a regressed clock), or one whose episode binding the
history cannot vouch for (a migrated claim raised by misclassification),
keeps no episode, and the next claim starts afresh as before. A re-claim
stamped before the kept release refuses as CLOCK_REGRESSED.

## 3. Alternatives not taken

- A new goal state `landing`: every claim-bound mechanism (budget, receipt
  binding, admission, the stop capability) keys on `claimed`; a new state
  would touch all of them for a fact one record carries.
- Exempting landing goals by a label: labels are free text a seat can set
  on any goal; the record is written by one verb under the claim.
- Restoring retained proof on re-claim: already law since db58ad931.
- Keeping the elapsed clock running across the gap: the audit's waits were
  the gap itself; charging them would breach the box on re-claim.

## 4. Fixtures that prove it

- internal/goal: `land-ready` writes the record and the history line;
  refuses a foreign claim, a fenced claim and a repeat; a machine holding a
  landing claim claims a second goal and the tree validates; a second
  landing claim on the same machine refuses at validation; `park`,
  `release` and `steal` clear the record; `done` archives the goal and the
  `land-ready` line survives in the archive; the frontier lists the landing
  goal under `Landing` and `SelectNext` picks the ready goal; the current
  goal is the working claim, and a landing goal alone resolves; a hand edit
  of `Landing:` refuses except through a hand park or done; recovery replays
  a dead owner's land-ready under its opid; the file round-trips.
- internal/goal: a same-pair release then re-claim restores
  AccountingRevision, EpisodeAt, EpisodeRevision and
  EpisodeObligationRevision, adds the gap to `idleSeconds`, clears the
  record, and the tree validates; a re-claim by another pair starts fresh;
  set-budget, steal, approve with a tuple, unapprove, done and park drop
  the record; set-budget preserves `idleSeconds`.
- internal/dispatch: the budget projection of a re-claimed goal counts the
  attempt spent before the release, projects KNOWN after a consumed
  discharge in the earlier episode, and measures elapsed as
  `max(0, now - start - idle)` with the discharge-advanced start kept (one
  fixture with an episode start, a consumed discharge, a release and a
  re-claim); a landing goal past its elapsed limit closes neither the
  working claim's admission nor its own `done`, `EvaluateGoalAdmission
  --stop-lineage` skips it, `FindBreachStops` routes nothing for it, and
  its own revision admission still admits with elapsed past the box while
  an attempt breach still refuses.
- internal/goal: set-budget on a landing goal keeps the slot; unapprove
  and set-arc drop it; an own-pair park writes the Episode record and a
  human park drops it; a Landing record on an unclaimed goal refuses at
  validation.
- cmd/metasystem: `test run` without `--goal` beside a landing goal stays
  ambiguous and names `--goal`; with a landing goal alone it resolves to
  it.
- internal/goal/stop.go: resume of a fenced goal beside a landing claim is
  not blocked by the landing claim.
- scripts/agents/goal-cli-fixtures.sh: a `landing-slot` scenario lands a
  goal beside a second claim on one machine, is refused a second slot,
  shows `goal next` continuing the working claim, concludes the landing
  goal, and shows a same-pair release and re-claim keeping the accounting
  revision and adding idle seconds.
- docs/backlog-mechanism.md carries the slot and the kept episode;
  docs/glossary.md's claim entry names the slot (docs/orchestration.md has
  no one-claim sentence to amend).

## 7. Two budget-law choices for Wido's word

1. A declared landing wait suspends the elapsed fence for that goal and
   nothing else: no stop and no cancellation land on a landing goal for
   elapsed time, its own receipts keep admitting past the elapsed box
   (attempts, minutes and active jobs still bind), and the overdue wait is
   printed. The alternative, freezing the clock at land-ready, had a cliff
   (a goal entering landing past its limit could never run a receipt) and
   gave a seat a free stop on the fence by declaring early.
2. An own-pair park keeps the accounting episode as an own-pair release
   does, so no park-and-reclaim resets a seat's box; a person's park, a
   foreign park and a steal start fresh.

3. The idle offset is one summed field. It comes off the clock only when
   the measured window began before the current claim, so a discharge
   consumed inside the current hold is never over-forgiven; a discharge
   that fell between two gaps of the same episode still is, by the length
   of the earlier gap. Splitting gaps per discharge needs a per-gap record,
   which this page does not add.

All three are built as stated; Wido's word at the landing confirms or
reverses them, and a reversal is one verb's rule each.

## 5. What the second read attacked

Whether stopping the landing goal's elapsed clock at `Landing.At` opens a
way to park work in landing indefinitely without cost (the attempts and
minutes still count, and the landing line is visible in every list);
whether any consumer of the one live claim is missing from the seven;
whether the idle offset can be gamed by release-and-reclaim to reset a
nearly breached clock (it cannot: the offset adds only the unheld gap, and
the held time before the release still counts); and whether the Episode
record should survive a park that returns to the same pair (this revision
says no: park is a pause the pair chose, and the next claim is fresh).

## 6. Critique record

Round 1, design-critique subagent, 2026-09-12: eight material findings, all
folded. M1: the forward-shifted EpisodeAt would fail `ValidateClaimRevision`
(now an `idleSeconds` offset on the claim record, EpisodeAt untouched). M2:
the Episode record dropped the obligation revision (now carried and
restored). M3: a landing goal's elapsed breach would close the working
claim's dispatch and fence the landing goal against its own `done` (now the
landing goal's elapsed clock stops at `Landing.At`, and admission's seat
walk skips landing claims like fenced ones). M4: seven consumers of "the
machine's one live claim" were unnamed (now section 1 names each with its
rule). M5: the blanket hand-edit refusal would refuse lawful hand parks and
dones (now removal follows the mapped verb). M6: the measure's source would
have left the archive (now the `land-ready` history line). M7: the drop
list omitted approve-with-tuple, unapprove and the non-`claim` bind paths
(now listed). M8: hours to `done` measure the person's cadence (now the
measure ends at the landing, with `done` as the program's bound).

Round 2, design-critique subagent, 2026-09-12: five material findings, all
folded. A: the idle offset and the discharge-advanced start now compose as
one formula. B: the Landing record now lives with the claim binding
(cleared in `clearClaimBinding` and by a fresh-owner bind, kept by the
episode-keeping rebind, refused on an unclaimed goal). C: the elapsed
freeze is replaced by suspending the elapsed fence for a landing goal
(section 7, choice 1). D: `test run` without `--goal` beside a landing goal
stays ambiguous. E: an own-pair park keeps the episode (section 7, choice
2). Under R-97-m1e there is no third round; the two budget-law choices go
to Wido with the landing.

Code read, code-critique subagent, 2026-09-12: five material findings, all
folded and tested. F-1: a seat's park of its own released goal dropped the
kept episode (release, park, unpark, claim reset the box); one rule now
serves every leave. F-2: the blocker park and the arc cascades wrote no
episode. F-3: a fenced landing claim gave up its slot and a later resume
would have refused at validation. F-4: a hand done of a landing goal was
refused at reconcile. F-5: eight named fixtures had no test. Four minor
findings: the breach-stop label on a landing goal (folded), the idle offset
after a discharge inside the current hold (folded with the window rule and
named in section 7), approve without a tuple dropping the episode (the page
now says so), an unvouched episode binding locking the pair out (folded).
