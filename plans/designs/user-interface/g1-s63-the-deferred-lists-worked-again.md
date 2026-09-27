# g1-s63: the deferred lists, worked again

- Kind: design
- Id: 01M3GZ2C27S201HT3Z0K94CDBG
- Status: draft
- Goals: browser-interface

Wido, 2026-09-27: "Finish all now." Every item below was named
deferred by Sol's read or the builder's report in the Built section of
g1-s58, g1-s60 or g1-s61; each was judged then to leave its slice
working and safe, and each is built now as that read's smallest fix
said, in the manner of g1-s57. No critique loop of its own: the reads
are the critique; Sol reads the build. Author Fable. Every cite read at
`2c87659a4`.

## The items

1. **g1-s61** The drawer's target is cleared when the drawer closes: the
   store's `showing` gains a clear the shell calls on close, so the plain
   toggle reopens at the end of the conversation and a chip's press
   still opens at its card (the builder's "Left", g1-s61 Built).
2. **g1-s58** Admission checks an open's `blockedBy` and `blocks` ids as
   it checks a blocker: each at the accepted tip, or the id of an `open`
   admitted earlier in the same answer; an unknown one refuses the
   action with its reason on the card (Sol's deferred list, g1-s58).
3. **g1-s60** A proposal keeps the time it was proposed: the entry's `at`
   is the admission's time and never restamped; an admitted write stamps
   `updatedAt` beside it; the inbox's `since`, "n new" and the group's
   age read `at`; the card may show both (Sol's deferred list, g1-s60).
4. **g1-s58** The version-race test covers every allowed pair of the
   outcome route's transition table, two writers racing with one version
   and exactly one admitted for each (Sol's deferred list, g1-s58).
5. **g1-s58** `settle`'s three answers are proven through the act test
   bed's injected git seam (g1-s57 item 3): a push that lands and a
   confirming refetch that fails yields the transaction's own "pushed;"
   result and error and answers `pushed-unknown`; a journal made
   unreadable answers `journal-unreadable`; a refused compare-and-set
   answers `refused` (Sol's deferred list, g1-s58).
6. **g1-s60** The inbox folds its own outcome writes into the entries it
   holds, as the card does, so a line this page just left unresolved
   never reads as in flight between the write and the page's re-read
   (the builder's "Left", g1-s60).

Not here: the owner-side basis compare inside the transaction (gate 2);
transcript trimming of an old unanswered proposal (Wido: "agreed that
old proposals just vanish"); the Overview chip; other humans' proposals.

## Verification and box

Each item with the test its read named or the smallest one that shows
it; the guards stay green; screenshots only where a page changed (the
inbox row's age after a refusal). Budgets as always. Box: one build lane
(Claude on Opus), one code read (Codex on Sol) with one fix round under
R-124; one attempt, 120 to 180 job-minutes; lands before g1-s62.
