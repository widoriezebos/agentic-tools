Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal backlog-ordered-by-priority)
Date: 2026-09-08

# Build brief: slice 1 of the backlog order, the rank and the human verb

The specification is metasystem/plans/backlog-ordered-by-priority-design.md,
revision 2, final. Read it whole before you touch anything. Its critique
ladder is closed: one independent read
(metasystem/records/misc/backlog-ordered-by-priority-critique-r1.md), three
findings folded, one recorded without action. No further prose revision
will be written, and the page says so.

The contract behind it is the goal record
metasystem/plans/goals/backlog-ordered-by-priority.md, which carries
Wido's sentence: "Backlog items need to be sorted in order of priority
and then a sequence number. And this should be able to update after the
fact so that we can change priority of the backlog."

## Slice 1 is sections 1, 2 and 3, and nothing else

Build exactly:

1. **Section 1, the record fields.** Priority and Sequence on the goal
   record, the rendering position and grammar, absent meaning unranked,
   the refusals for a lone field, an empty value, a sign, a fraction,
   overflow, zero or an out-of-scale priority, and the tree validation
   that requires each priority's ranked open goals to occupy exactly
   1..N once each.
2. **Section 2, the human mutation.** `goal set-priority`, a human act
   at the enrolled terminal, with the re-sequencing rule, the closure
   over every lifecycle transition the page enumerates including the
   reconciliation batch rule, the event and failure contract, and the
   refusals the page names.
3. **Section 3, the ordered listing.** `goal list` printing open goals
   in priority then sequence order with state and pin, unranked last.

Leave for slice 2, and do not touch: section 4 (`goal next` and taking
work), section 5 (the channel status head), and the seat projection
changes the page describes after section 4. If a change in sections 1 to
3 forces a compile-level edit inside those owners, make the smallest one
that keeps the tree green and name it in the return as reaching into
slice 2's surface.

## How to prove it: canary first, never a battery

The page's section 6 gives, for every fixture, the smallest proving run
and its independent observation. Use them exactly as written for the
rows that belong to sections 1 to 3, and follow its own rule: run the
subtest for the changed behaviour first, a failing canary stops
widening, and widen to the owning package only to answer a question the
canary left open. Every new Go test command carries a two-minute
ceiling; exceeding it is a test failure to investigate, not permission
to loop a wider run.

Wido's instruction, 2026-09-08, is binding here: "Make sure you use the
canary approach for testing. We should not waste cycles if we can
avoid."

Report, in the return, each canary you ran and what it observed. Do not
run `scripts/agents/go-gate.sh` more than once, at the end, and do not
attempt the fixture beds: the orchestrator runs the process-owning beds
outside your sandbox.

New tests use the existing temporary-ledger helpers the page names and
never touch the live backlog.

## What you must refuse to invent

The page is explicit, and so is this brief. If the build seems to
require globally unique sequence numbers, atomic selection-plus-claim,
human authority over another channel, or expanded reconciliation
grammar, stop and report the gap. Do not invent behaviour and do not
write a third prose revision.

Do not edit any existing goal record, the approval sweep payload or its
digest. Do not change what a claim or an approval means.

## Verification

- The canaries for sections 1 to 3, per section 6, reported individually.
- `go build ./...`
- `scripts/agents/go-gate.sh --fast`, once, at the end.

Gap rule: stop and report a gap; never fill it silently.
