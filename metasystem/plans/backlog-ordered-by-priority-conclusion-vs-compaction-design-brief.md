Working Mode: design
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, coordinator under goal backlog-ordered-by-priority)
Date: 2026-09-09

# Design brief: what a reader may conclude from a `done` history event

## Your authority to author this design

You are dispatched to AUTHOR a design. The composed prompt's Working Modes
table says that under Design mode "the design itself, dispositions, the
obligation matrix" are never delegated. That row is scoped to the MODE and is
role-blind, which makes it forbid the very job you were dispatched for; goal
`design-prohibition-is-role-scoped` exists to fix that wording. Until it
lands, this paragraph is the override, and it is standing authority, not the
orchestrator's improvisation:

- R-89-m1b (2026-09-09, Wido verbatim): "We have Fable back, so I want you to
  switch back to Claude using Fable 5.1 for designs now." Design authoring is
  this lane.
- R-25 puts design authoring on this lane; R-86-m1b's Astra fallback existed
  only while Fable was unavailable and is now ended.

So: author the design. Do not stop to report an authority conflict. What stays
with the orchestrator is unchanged and is not yours: the dispositions of any
critique, certification, the ledger, and the receipt.

## The situation

Goal `backlog-ordered-by-priority` gave every goal a priority and a sequence
and made `goal next` route work by that rank. Slice 1 landed with one finding
open and slice 2 recorded a second. Both are the same shape: a reader states
something untrue about a goal. Neither can wedge the queue, refuse a human act
or corrupt the ledger. Both were deferred to this lane because each has two
defensible repairs and choosing between them is a design decision.

Specification in force: `metasystem/plans/backlog-ordered-by-priority-design.md`
revision 2. Its critique ladder is closed. The goal record is
`metasystem/plans/goals/backlog-ordered-by-priority.md`.

## Question 1 (BOC-01) — a survivor of someone else's conclusion looks concluded

The critic's claim, verbatim from
`metasystem/records/misc/backlog-ordered-by-priority-critique-r3.md`:

> A goal that merely survives a neighbour's conclusion gains a history event
> whose verb is done, and one existing reader treats that as the goal having
> been concluded. When a ranked goal is finished, the build compacts the rest
> of that priority and records the shift on each survivor using the departing
> act's own verb, which for a conclusion is done. The measurement code in
> internal/metrics/compute.go decides when a goal concluded by scanning
> history backwards for the last done verb, so `metasystem metrics --goal
> <id>` reports a finished lifecycle for an open goal, dated to the moment a
> different goal finished.

The reader is `concludingEpoch` in
`metasystem/internal/metrics/compute.go:511`. It walks history backwards for
the first event whose `Verb` is `done`, takes that event's timestamp as the
conclusion, and never consults the record's own state or the event's other
fields.

The critic named the two candidate repairs and did not choose: filter on the
reading side, or mark the compaction so it is distinguishable from a
conclusion.

**Evidence the orchestrator gathered for you, because it may remove the need
for a new field.** The two event kinds are already distinguishable in the
landed data by three independent discriminators:

1. A compaction event carries `reason=priority-order` and `from=`/`to=` rank
   fields. A real conclusion carries no `reason=`. Observed compaction line,
   from the live record of this very goal:
   `done actor=human:Wido targets=account-provenance,actionable-metrics,backlog-ordered-by-priority,... reason=priority-order from=1:3 to=1:2`
2. A real conclusion's `targets` names the concluded goal alone. Observed, from
   the archive under `metasystem/records/goals/`:
   `done actor=m1c+main-1788680061-17829-64951c targets=claude-critic-shell-network-deny`
3. The survivor's own `State` is not `done`; a concluded goal's is, and a
   concluded record has also moved out of `plans/goals/` into
   `records/goals/`.

Decide which discriminator the reader should use and say why the others are
weaker. If you conclude a marker is needed anyway, say exactly what makes the
three above insufficient — that is the finding, and it is worth more than the
repair.

Bound the blast radius: the critic reports that a whole-tree search found
`concludingEpoch` to be the ONLY place that infers meaning from a history
verb on a goal record, which is why the equivalent split compaction event is
currently harmless. Confirm or refute that before designing, because if a
second reader exists the reading-side repair stops being local.

## Question 2 (BOQ-02) — the frontier narrowed the idle refusal

Verbatim from
`metasystem/records/misc/backlog-ordered-by-priority-critique-r7.md`:

> The orchestrator's brief said an over-norm goal is invisible in the frontier
> exactly as before this chain. That is wrong. Before this chain the frontier
> admitted such a goal to its ready list, because the only extra filter was a
> structural budget check that an over-norm budget passes. Routing the frontier
> through claim admission therefore also narrowed the idle-stop refusal and the
> steward's idle escalation: a seat whose only remaining backlog is over-norm
> approved work will now be allowed to end its turn where it was previously
> blocked three times and then handed a continuation for a goal it could not
> have claimed.

The critic graded this not material and reported it as fact. The orchestrator
accepted it as the more valuable of the two findings, because it corrects a
false statement the orchestrator itself put in a brief and then repeated to
Wido. It is unactioned by design, waiting for this page.

The question is not whether the narrowing was wrong — the alternative is
handing a seat a continuation for a goal it cannot claim, and the human still
sees the goal at the head of the status report. The question is what category
such a goal belongs to. The page defines the frontier's categories; an
approved goal whose budget exceeds its tier norm fits none of them cleanly. Say
what the category is, what each reader does with it, and what the human sees.

## What must not change

- The rank is the human's judgement. Nothing here sets, infers or reorders a
  priority or sequence.
- `goal next` stays a read, not a reservation, and its selection stays true at
  the accepted tip it read (design page, the paragraph after section 4).
- The claim gate stays the single admission owner. Do not add a second place
  that decides eligibility.
- No change to the ledger's record grammar unless question 1 forces one, in
  which case say so explicitly and account for the closed-grammar refusals in
  `metasystem/internal/goal/file.go` and the archived-record rules in
  `metasystem/internal/goal/validate.go`.

## Deliverable

Write this new file: `metasystem/plans/backlog-ordered-by-priority-conclusion-vs-compaction-design.md`

Both questions answered, each with the mechanism named at the symbol
level, the reader or writer that changes, and the smallest fixture that proves
it — including, for question 1, a canary that must FAIL before the repair and
pass after, on a survivor record whose history ends in a compaction `done`.
A canary that cannot fail for the right reason is worse than none; two of the
three material findings on revision 1 of the parent design were defects in
exactly that.

State plainly anything the existing page cannot answer rather than inventing
it. A gap reported is worth more than a gap filled silently.
