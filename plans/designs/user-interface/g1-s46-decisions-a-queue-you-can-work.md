# g1-s46: Decisions — a queue you can work

- Kind: design
- Id: 01M3C34HPPYNJ6Z0JFZNTTTVXV
- Status: draft
- Goals: browser-interface

Wido, 2026-09-25, on the live page: "it is a very big list of things that
is hard to navigate, grasp, manage. So what do I want/need here and how can
a better UX help me with that. I need better UX. Design and then implement".
Author Fable. Step 2 of the Decisions section, on g1-s44 as built at
`093be9aaf`; every cite re-read at that commit and every number read from
the live payload of this checkout at 10:48 UTC.

## 1. What exists and binds

1. **The live inbox is one flat list of 166 cards.** 122 are approvals
   (To Do rows without approval), 34 parked, 4 register questions, 2 drafts,
   4 landed designs. Every card is the Overview item style, three to five
   lines tall (src/decisions/DecisionsPane.tsx:244-289), so the page is
   roughly forty screens and the ten cheapest, freshest items sit below the
   122 approvals, because the order is deadlines, then renewals and
   reviews, then approvals in backlog order, then everything else oldest
   first (internal/ui/decisions/decisions.go:715-772).
2. **An approval card says nothing twice.** For 122 of 122 rows `asked` is
   exactly "Approve " + title + " for execution" and `silence` is exactly
   "it stays in To Do and no seat may claim it" (decisions.go:315-351). The
   title is the first sentence of the intent (decisions.go:961-978); the
   median is 164 characters, 81 rows exceed 120. What the card does not
   show is what would let a human choose: the row it carries has `labels`
   (16 rows `headless-fleet`, 5 `robustness`, ...), `tier` (44 tier 3, 44
   tier 2, 32 tier 0), `origin` (63 `main`, 59 `human`), `priority` and
   `sequence`, `budget`, `nextStep`, `blockedBy` and `openBlockers`
   (src/backlog/api.ts:98-146).
3. **A person's park is listed as undecided.** All 34 parked rows carry
   `by: human:Wido`, from the consolidation of 2026-09-13, each with its
   reason and 19 with a re-queue condition in that reason. The composition
   admits every park with an empty blocker (decisions.go:519-535) and the
   count calls them "needs your choice".
4. **The register is read from the wrong root.** The page shows zero
   rulings and zero defects while `metasystem/memory/rulings.md` holds 160
   rows. `ui serve` hands the reader the checkout root (cmd/metasystem/ui.go:540-541,
   `rulings.Read(roots.Checkout)`); the reader joins `memory/rulings.md`
   onto it (internal/rulings/rulings.go:98) and answers an empty register
   for a file that does not exist (rulings.go:149-152). This checkout runs
   with `--repo <checkout> --metasystem-root <checkout>/metasystem`, and
   `memory/` exists only under the installation. The channel and the
   steward's journal are read from the checkout root too (ui.go:521, 536);
   neither file exists under either root today, so nothing is shown wrong
   there yet.
5. **The engine's acts from a browser.** The interface publishes approve,
   unapprove, set-priority, open, block and unblock, one goal per POST
   (internal/ui/httpd/acts.go:16-46), each through `act.Authority` with the
   signed-in session proof. Park is not one of them: a park of a
   human-origin goal and the lifting of a human's park each require a
   terminal-grade proof (internal/goal/verbs.go:2647-2651 and 2769-2775),
   which `requireHuman` admits only for grades terminal and enrolled
   (verbs.go:314-332), and a browser session proof carries no grade
   (internal/humanauthority/authority.go:168-173, 308-320). Park also needs
   the branch-safety check the command edge supplies (verbs.go:2640-2642;
   cmd/metasystem/goalsync_mutations.go:51). Approve admits the session
   proof through its own classification (internal/goal/approval.go:503-520),
   which is why the sheet works today.
6. **The board already has a filter model and a budget prefill.** `Filters{text,
   priority, tier, seat, arc}` with `matchesText` over id and intent
   (src/backlog/filters.ts:48-121); `prefillFor(row, budgetDefaults, rows)`
   answers a budget and its source, goal, project, last-approved or none
   (src/backlog/moves.ts:88-138); the approve route takes the complete tuple
   per goal (acts.go:78-86).

## 2. What a human needs here

Two different jobs share the page. **Answering**: a question, a draft, a
landed design, a review that came due, a seat's ask, an alert. Few, fresh,
each a minute's decision, each with its own consequence of silence.
**Triage**: a queue of goals nobody has authorized. Many, mostly weeks old,
fine to wait, worked in sittings, by family and by tier, with a way to say
yes to several at once and to say not now. The flat list serves neither:
it buries the first job under the second and gives the second no tools.
And it counts as undecided what the human already decided.

Decisions:

- D1. **Two blocks, not one list.** "Asked of you" holds every inbox kind
  but approval; "Waiting for your approval" holds the approvals. The
  header's count becomes two counts in words. Asked-of-you rows keep the
  g1-s44 card unchanged; it is the right shape for ten things with ten
  different consequences.
- D2. **A person's park is a decision.** A parked row whose `by` starts
  with `human:` leaves the inbox for a Decided tab named "Not now", with its
  reason, who parked it and when, the goal link and the unpark command. A
  seat's park stays in the inbox as today, because a human has not seen it.
- D3. **The queue row is one line that opens.** Left, a checkbox; then the
  title, at most two lines; right, the age, a tier chip where tier is
  above zero as the board's row shows it (src/backlog/GoalRow.tsx:103), a
  "yours" chip for `origin: human`, the labels as chips, at most three and
  "+n", and Approve. The row's silence is said once, in the block's head:
  "If you do nothing, these stay in To Do and no seat may claim them." The
  derived `asked` sentence is not shown. Opening a row shows the whole
  intent, the next step, the budget tuple in words, what blocks it, its
  priority band, the goal link, and the terminal way to say not now,
  `metasystem goal park --id <id> --because "<why>"`, in a code span, as
  parked rows already show their unpark. Open state is page state, not
  stored.
- D4. **Queue tools, three of them.** A Find box over id, intent and
  labels, the board's `matchesText` extended to labels; label chips drawn
  from the rows shown, each with its count, one selected at a time,
  "yours" and "seats'" beside them as origin chips; and an order toggle,
  backlog order (default, the board's) or newest first by `openedAt`. The
  block's count reads "122 waiting · 16 shown" when narrowed. Nothing is
  persisted.
- D5. **Select and approve.** A checkbox per row, "Select all shown" in the
  block's head, and one button, "Approve 12 selected", opening one sheet.
  The sheet lists the selected goals, each with the budget it would be
  approved with and the source's words from `prefillFor`; a goal whose
  prefill is null is listed as "needs its budget first: approve it alone"
  and is not sent. One Approve sends the rest in order, one POST per goal,
  as the routes are, with "7 of 12" while it runs; the first refusal stops
  the run and the sheet says which landed and what was refused, in the
  engine's words; then the page reads its payload again. Sign-in is as
  today; the single-row Approve keeps the existing sheet.
- D6. **The register is read from where the steward writes it.** The
  installation root, `rulings.Read(roots.Installation)`. The build reads
  `up`'s steward launch and, where it proves the channel and the journal
  are written under that same root, moves those two readers with it;
  otherwise it leaves them and says so.
- D7. **The Approved tab is untouched** apart from the Not now tab beside
  it. Rulings, Decisions, Answered as they are.

Every UX claim above is a claim about the payload of this checkout on
this day; the fixture carries the same shape so a reader can see it.

## 3. The payload: `GET /api/decisions`, schema 2

Additions only; nothing renamed:

```
"counts": { "needsYou": 132, "asked": 10, "waiting": 122, "rulings": 160 }
"decided": { ..., "notNow": [ { "id": "answer-archive", "title": "...", "by": "human:Wido",
                                "at": "2026-09-13T08:23:22Z", "because": "Not now (...)",
                                "where": { "kind": "goal", "id": "answer-archive" } } ] }
```

`needsYou` keeps its kinds and order; the parked kind excludes rows whose
`Waiting.By` has the prefix `human:`, and those rows compose `notNow`,
newest first, from `Waiting{Reason, Since, By}` (internal/backlog/project.go:59-65).
`counts.needsYou` is the inbox as listed; `asked` and `waiting` are its
two blocks. The approval `Need` gains nothing: the row it carries has every
field D3 and D4 read. The page splits by kind.

## 4. The page

`DecisionsPane`, the same route. Header: "10 asked of you · 122 waiting
for your approval", each a link to its block. Block one is the g1-s44
inbox less the approvals. Block two: a head with the title, the count,
the silence sentence, Find, the label and origin chips, the order toggle,
"Select all shown" and the approve-selected button, which is disabled with
the count at zero; then the rows of D3, in the board's `ms-goal-*` idiom
where a class fits and the page's own tokens otherwise. A row's disclosure
is the pattern of the Fleet row in g1-s45, a button with `aria-expanded`.
The bulk sheet is the board's `Panel` chrome with a list inside, one line
per goal: title, the budget in the tuple's five words, the source's
words, or the exclusion line. Decided gains the "Not now" tab, a list of
the Overview's item rows with the reason as the note and the unpark
command in a code span. Empty states: "Nothing is asked of you" and
"Nothing waits for your approval". Phone width: the queue row stacks,
the checkbox stays left, the tools wrap. No timers; one EventSource; the
page reads on mount, Refresh and after an act or a run of acts.

Help terms: `needs-your-choice` rewritten for the two blocks; new
`asked-of-you`, `waiting-for-approval` (what an approval is and that the
row's budget is the one the approval carries), `not-now`, `approve-selected`
(one act per goal, stops at the first refusal). The Partner's page capture
adds the block open, the narrowing in force and the selected count.

## 5. Not here, step 3 and later

Not now from the page, single or selected: it needs the engine to admit
the session proof at the grade park and unpark require, and the branch
check lifted out of the command edge; that is an authority decision, not
a page. Persisted narrowing. Grouping by goal family or arc. Keyboard
triage. One publish for many approvals. A Find box on the Approved tab.
The rest of g1-s44's section 6.

## 6. Verification and box

Go: composition tests on the split counts, a human park in `notNow` and
a seat park in the inbox, the schema number; a test on the wiring that
the register reader is handed the installation root. Frontend: pane tests
on the two blocks and their counts, a queue row opening, Find over a
label, a label chip narrowing, the order toggle, select-all over the
shown rows only, the bulk sheet excluding a null prefill, sending in
order, stopping at a refusal and re-reading; the guards stay green. The
walkthrough fixture grows to thirty approvals with labels, tiers and both
origins, three human parks and one seat park, and the screenshots at 1280
and 400 are the evidence: the header, the queue narrowed, a row open, the
bulk sheet, the Not now tab. Budgets as always. Box: one build lane
(Claude on Opus), one code read (Codex on Sol) with one fix round under
R-124, after Astra's read; two attempts, 120 to 180 job-minutes.

## 7. Self-grade

High on D1 to D4 and D7: they rearrange what the payload already carries
and reuse the board's filter and row idioms. Medium on D5: it is the first
act on this interface that runs more than one publish, and the stop-at-
first-refusal rule is the whole of its safety; the engine's per-goal
refusals are the words shown. Medium on D6: the root is proven wrong for
the register; the other two readers move only on proof. Weakest: the
triage act a human most wants, not now, stays in the terminal until the
authority question is decided; the row says so in a code span rather than
offering a button that would refuse.
