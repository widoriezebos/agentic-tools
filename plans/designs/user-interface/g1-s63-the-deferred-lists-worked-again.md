# g1-s63: the deferred lists, worked again

- Kind: design
- Id: 01M3GZ2C27S201HT3Z0K94CDBG
- Status: done
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

## Built (2026-09-27)

Landed on ui-development and main: seven commits on `ui/g1-s63`, one
per item and the bundle last, built by Claude on Opus with two
sub-agents on items 4 and 5, read by Codex on Sol under R-124
([g1-s63-sol-read.md](g1-s63-sol-read.md)): zero material findings,
"land as built", all five of the builder's departures accepted; no fix
round was needed. Go, vet and the interface packages green, the fast
gate and the parallel ratchet passed; 91 test files, 1,398 frontend
tests; the bundle rebuilt twice to one digest. One screenshot at 1280
light: the refused row standing second in its group where the asking
put it, "asked by the Partner, today", against g1-s60's own evidence
where the same row had dropped to the foot after the refusal restamped
it.

The items as built: (1) the store's `clearShowing`, called by the
shell's one drawer-closing path; (2) `edgesNamedBy` reads `blocker`,
`blockedBy` and `blocks` and admission checks each id at the tip or
opened earlier in the answer; (3) `at` stamped once at admission,
`updatedAt` stamped by every admitted write, carried through the
Partner's entry, the decisions member (schema 6, an addition) and the
TypeScript types, the inbox's age and order reading `at`; (4) one test
derives the eleven allowed pairs from the states and the transition
table and races eight writers on each; (5) a test-local scripted
repository over the act bed's ledger drives a real `Approve` to each
of settle's three answers, with the ledger asserted; (6) the inbox's
`record` port folds a written answer into its held entries as the
card's does.

Departures the read accepted: the card shows no second timestamp (the
design's "may"); the schema bump; the `refused` proof closes the
journal entry through a failed rebuild capture rather than a lease
that no test can wait out; the two-pair race test replaced by the
all-pairs one; the bed's repository field widened to the interface.

Later, when it hurts: a proposal persisted before this slice may carry
a restamped `at` and no `updatedAt`, and its admission instant is not
recoverable from that field (the readers tolerate its absence); the
screenshot's "today" cannot tell admission from write, the dated tests
can.
