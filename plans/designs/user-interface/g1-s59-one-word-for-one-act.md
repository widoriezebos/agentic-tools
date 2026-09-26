# g1-s59: one word for one act

- Kind: design
- Id: 01M3FQHY730XDF9GRP2HS85JMG
- Status: draft
- Goals: browser-interface

**On hold** (Wido, 2026-09-26, late): the verb system is not finished
and will be restructured completely, so the words this slice would
rename to are not settled. It waits for that work to land, then is
re-read against the successor table's names before it is built.
Nothing else here changes.

Wido, 2026-09-26, on g1-s58's recommendation that the pages' button words
follow the public verbs: "I will follow all your recommendations". A
presentation slice, half a page, in the ceremony the plan gives one: this
note, Astra's read of it, the build, Sol's code read. Author Fable.
Every cite read at `2ef9b3d64`.

## What binds

The public verb system landed on main (merged as `8a22b6284`): one
descriptor table owns the public names, and "public responses always use
current terms" (metasystem/plans/designs/intent-workflows.md:74-75). The
Partner's proposals speak those names and g1-s58's card shows them
(g1-s58 D4). The pages still say the words g1-s46 and g1-s48 chose before
the verbs existed: `Withdraw approval…` and `Set priority…` in the board's
card menu (src/backlog/menu.ts:40, 43); `Withdraw approval` as the act
sheet's title and button (src/backlog/ActSheet.tsx:115, 146) and on the
Decisions page's Approved tab (src/decisions/DecisionsPane.tsx:871); `Not
now` on the queue row, the selection bar and the bulk sheet
(src/decisions/BulkSheet.tsx:141-162, InboxRow.tsx:37); `Return to queue`
on the Not now tab and the seat-park row (DecisionsPane.tsx:786-808,
InboxRow.tsx:292); `Set priority` as the rank sheet's title
(src/backlog/RankSheet.tsx:76); the help terms `Not now` and `Return to
queue` (src/help/terms.ts:213-221) and the Decided tab named "Not now"
(g1-s46 D2). The engine's own word for the act is a third one, park.

## The change

One act, one word, everywhere a human reads it, the public verb's:

| today | after | where |
|---|---|---|
| Withdraw approval, Withdraw | Unapprove | card menu, act sheet title and button, Approved tab (the question sheet's own "Withdraw", which withdraws a question, is not a goal act and keeps its word) |
| Set priority | Prioritize | card menu, rank sheet title and its submit button (RankSheet.tsx:95) |
| Not now, Not now for n selected | Pause, Pause n selected | queue row, selection bar, bulk sheet title, button, eyebrow "To Do → Not now" and sheet name "Not now for selected" (BulkSheet.tsx:140-142), inbox row |
| Return to queue | Resume | Not now tab, seat-park row |
| the Decided tab "Not now" | Paused | the tab's name and its count |
| Open goal, Approve, Edit… | unchanged | they already are the verb |

The help terms are rewritten under the new words, each saying the old
word once ("Pause, called Not now until the verbs landed") so a human who
learned the old word finds the new one; the sentences that explain the
act keep their substance (a pause is a decision with a reason; resume
returns the goal to approved where its approval stands, else to queued).
The Partner's vocabulary file, generated from the help terms, follows.
g1-s58's card drops its bridging help text, since the button now says
the same word. The routes, the bodies, the acts and every test of
behaviour are untouched; only rendered words, titles, tab names, the
help register and the tests that assert those words change. The
walkthrough's screenshots of the Decisions page, the board menu and the
two sheets are retaken at 1280 and 400.

## Not here

The engine's own word, park, in refusal sentences the engine writes: a
refusal is quoted as the engine says it. The CLI. Any act's behaviour.

## Verification and box

Frontend: every test that asserted an old word asserts the new one; a
grep over `src/` finds none of the four goal acts' old words outside
the help terms' "called … until" clauses and comments, the question
sheet's "Withdraw" standing as it is; the vocabulary file regenerated
and its drift test (src/help/terms.test.ts:142) green;
the guards stay green; the bundle rebuilt. Screenshots: the queue with
the selection bar, the bulk sheet, the Paused tab, the board's card menu,
the act sheet. Box: one build lane (Claude on Opus), one code read (Codex
on Sol) with one fix round under R-124, after Astra's read; one attempt,
45 to 90 job-minutes; lands after g1-s58, whose bundle it would otherwise
conflict with.

## Self-grade

High: a rename with the routes untouched. Weakest: "Paused" as a tab
name beside "Rulings", "Decisions", "Answered", "Approved" is an
adjective among nouns; "Pauses" reads worse, and the count beside it
carries the meaning either way.

## Dispositions (Astra read, 2026-09-26, under R-124)

Zero material findings, "build as written"; two non-material
clarifications, folded because each costs one line: the absence check
is scoped to the four goal acts, since the question sheet's "Withdraw"
withdraws a question (S59-01); the table names the rank sheet's submit
button and the bulk sheet's eyebrow and sheet name (S59-02). Astra
confirmed the four names against the descriptors, the vocabulary drift
test, "Paused" beside the other tabs, and that nothing in g1-s58's step
1 depends on the old words: the serialisation is the bundle's alone.
The read is saved verbatim in g1-s59-astra-critique.md.
