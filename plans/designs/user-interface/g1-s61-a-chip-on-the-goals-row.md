# g1-s61: a chip on the goal's row

- Kind: design
- Id: 01M3FSBWHJHX2NR2H7B1N3SQFN
- Status: draft
- Goals: browser-interface

Step 3 of g1-s58 (its section 4), on Wido's UX ruling of 2026-09-26
that a waiting proposal belongs where the human looks, and his word to
follow the recommendation: the subject's own row should say the Partner
has an opinion on it before the row is opened. Author Fable. Every cite
read at `e223347be`; builds on g1-s58 step 1 and g1-s60 as designed,
and lands after both.

## What exists and binds

1. **The human's waiting proposals are already in the browser.** The
   Partner store holds the whole conversation for the page, drawer and
   focused view alike, every message with its `proposals[]` and their
   live state, folded from the snapshot and the stream's beats
   (src/partner/store.tsx:471-514, 570; g1-s58 D6, D8; g1-s60 D5). A
   proposal names its goal by id. Nothing on the board or in the
   Decisions queue reads it.
2. **A goal's row carries chips.** The board's card shows a tier chip
   where the tier is above zero and the labels (src/backlog/GoalRow.tsx:103);
   the Decisions queue row shows "yours", the tier and up to two labels,
   muted, on the right (src/decisions/InboxRow.tsx:52-101; groups.ts:209-232);
   the goal page's header shows the goal's chips (src/project/ProjectPane.tsx:774-790).
   Chips are `Chip` from the shell's controls (src/shell/controls.tsx:60-62),
   with the marker variant for "a human wrote this / needs attention".
3. **Pressing a thing opens where it is decided.** The drawer opens at
   a card as the sitting's counts open the table (g1-s58 D8); the
   Decisions inbox opens a group and a row (g1-s48 D2, D4).

## How it works, from the chair

You are on the board, triaging. On two cards a small marker-coloured
chip reads "Not now proposed"; on a third, "Edit proposed". You did not
open the drawer; the Partner said something about these goals last
night and it is still waiting. You press the chip on the first card:
the drawer opens at that card's line, Apply one press away. On the
Decisions queue the same rows carry the same chip, and on a goal's page
its header does. When the line is applied or dismissed, the chip is
gone, on every page, without a re-read.

## Decisions

- D1. **The chip reads the Partner store, and nothing on the server
  changes.** A hook `useProposedFor(goal)` in the Partner store answers
  the waiting and in-flight lines (`waiting`, `applying`, `refused`,
  `unresolved`) for one goal id from the loaded conversation, newest
  first. The board, the queue row and the goal header ask it. No
  payload changes, no route, no reader: the browser already holds the
  human's own proposals for the page's life, and the stream's beats keep
  them current.
- D2. **One chip per row, the verb's word.** Where the hook answers at
  least one line, the row shows one marker-variant chip, "<verb word>
  proposed" for one line and "n proposed" for more, after the row's
  other chips. The verb word is the button's word the card uses (g1-s58
  D4). A refused or unresolved line reads "<verb word> refused" or
  "unresolved", in the danger colour, since those wait on the human
  too.
- D3. **Pressing it opens where it is decided.** The chip opens the
  drawer at the newest such line's card, as the sitting's counts open
  the table; on the focused page, which has no drawer, it scrolls the
  transcript to the card. It is a button with a label naming the goal
  and the count, in the icon-button's manner: nothing icon-only without
  a name.
- D4. **Only the human's own.** The store is one human's conversation,
  so the chip is that human's proposals and nobody else's, as the inbox
  group is (g1-s60 D1).
- D5. **Nothing else moves.** No proposal marker in the payloads; no
  count in the header; no chip on other pages.

## Not here, later

The chip on the Overview's groups. A tooltip with the line's words.
Proposals of other humans, once there are several.

## Verification and box

Frontend: the hook over a store with lines in every state across two
answers for two goals, answering the four unsettled states for one goal
newest first and nothing for a goal with only applied and dismissed
lines; the chip's words for one and for several lines and for a refused
one; the chip absent where the hook answers none; the press opening the
drawer at the newest card and scrolling on the focused page; the chip
on the board card, the queue row and the goal header through
`renderToStaticMarkup`; the chip leaving on a `proposal` beat that
settles the line; the guards stay green. Walkthrough: the fake
Partner's canned proposal left waiting, the board and the queue showing
the chip, the press opening the drawer; screenshots at 1280 and 400.
Budgets as always. Box: one build lane (Claude on Opus), one code read
(Codex on Sol) with one fix round under R-124, after Astra's read; one
attempt, 60 to 120 job-minutes; lands after g1-s60.

## Self-grade

High on D1: a read of state the page already holds. High on D2, D3: one
chip in the row's own idiom, opening what exists. Weakest: a chip that
reads the conversation store means the board shows a proposal only
while the Partner store is loaded, which it is on every page of the
shell; on a page opened before the store's first snapshot the chip
appears a moment late, as the drawer's own content does.
