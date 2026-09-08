Working Mode: design
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal backlog-ordered-by-priority)
Date: 2026-09-08

# Design brief: the backlog gets a priority and a sequence

Deliverable: a new file
metasystem/plans/backlog-ordered-by-priority-design.md, revision 1.
Write the design only. No code, no fixtures, no other file.

## The ask, in Wido's words

"Backlog items need to be sorted in order of priority and then a
sequence number. And this should be able to update after the fact so
that we can change priority of the backlog."

The goal record metasystem/plans/goals/backlog-ordered-by-priority.md
carries that sentence and the full DONE definition. Read it first and
treat it as the contract. Quote from it rather than paraphrasing when
you state what must be true.

## What is true today

Nothing orders the ledger. A goal record has no rank. `goal list` sorts
by nothing. A seat with a free claim picks by reading the approved
unclaimed records. The only order that exists is a pin, the order in a
seat's kickoff prompt, and prose inside a few records.

## What the design must settle

1. **The record fields.** A priority on a small ordered scale, and a
   sequence number giving a goal its place within that priority, unique
   among open goals. Where they live in the goal record, how they are
   spelled in the ledger's own field grammar, and what an absent value
   means. A goal with no priority yet must sort last, so approving the
   grandfathered backlog needs no bulk edit.
2. **The human verb and its event.** `goal set-priority --by <human>
   --id <goal> --priority <n> [--sequence <n>]`, a human act at the
   enrolled terminal, recorded as a ledger event the way `set-pin`
   already is. Settle the re-sequencing rule precisely: what happens to
   the other goals in that priority when a number is inserted, when one
   is removed, and when two seats race. Name the refusals.
3. **The ordered listing.** `goal list` prints open goals in priority
   then sequence order, with state and pin.
4. **The next verb.** `goal next --machine <nick>` returns the first
   approved, unclaimed goal in that order that is unpinned or pinned to
   that machine, and seats take work through it instead of reading
   records. Tie it to the existing claim rule: one claim per machine,
   pins respected. Say what it returns when nothing qualifies.
5. **The status report line.** The channel's status report shows the top
   of the order, so the human sees what the fleet will do next.
6. **The fixtures the build will hide behind**, named but not written:
   set-priority reorders and re-sequences; next returns the expected
   goal for a pinned and for an unpinned machine; a goal without a
   priority sorts last; the sweep-approved backlog stays valid.

## Seams to read before you write

The goal record and its field grammar, the goal verbs and their event
history, the existing `set-pin` verb as the model for a human act with
an event, the claim rule and its one-claim-per-machine quota, the
listing path, and the channel status report. Name every file and symbol
you rely on, and say when you could not find something rather than
assuming it.

## Constraints

Stay inside the intent. The scale is small and ordered; do not invent a
scoring system, a decay rule, an automatic re-prioritiser, or anything
that decides priority without the human. Wido sets the first priorities
himself once the verb exists, so the design must not require a bulk
migration to become useful. Do not change what a claim means or how
approval works.

Where the intent is ambiguous, say so in the page and give one
recommendation rather than listing options. Where you would need a
second design round, say that too: this design gets one revision and one
independent critique, then the build proceeds behind the fixtures above.

Gap rule: stop and report a gap; never fill it silently.
