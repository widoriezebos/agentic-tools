# g1-s46: Decisions — a queue you can work

- Kind: design
- Id: 01M3C34HPPYNJ6Z0JFZNTTTVXV
- Status: accepted
- Goals: browser-interface

Wido, 2026-09-25, on the live page: "it is a very big list of things that
is hard to navigate, grasp, manage. So what do I want/need here and how can
a better UX help me with that. I need better UX. Design and then implement".
Then, on the authority question in section 5: "yes, park and unpark I
want", recorded as R-125-m1u. Author Fable. Step 2 of the Decisions
section, on g1-s44 as built at `093be9aaf`; every cite re-read at that commit and every number read from
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
   there yet. `up` launches the steward with the checkout root too
   (cmd/metasystem/up.go:193; internal/up/up.go:494;
   internal/steward/runner.go:899), so the steward's own review sweep
   reads the same absent file on this layout; the journal and the channel
   are written under the checkout (steward/notify.go:378, runner.go:245).
   The review cards the register would yield name `memory/rulings.md` as
   their destination (decisions.go:646), which the document reader opens
   relative to the checkout (internal/ui/project/document.go:130).
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
   cmd/metasystem/goalsync_mutations.go:51-82), whose two tip readers,
   git wrapper and operation id live in cmd/metasystem/goal_branch.go:26-33,
   692-735 and 1033-1039 over `goalbranch.CheckParkBranch`
   (internal/goal/branch/status.go:118). Approve admits the session proof
   through its own classification (internal/goal/approval.go:404-409),
   which is why the sheet works today, and it names the session on the
   history line through `recordSessionAuthority` (approval.go:426), which
   park and unpark do not call (its callers: approval.go:426, order.go:149,
   verbs.go:1854 and 1897). A park's `By` is `human:<name>` for any actor
   with a human (verbs.go:203-208), so a session's park is a human park.
   R-125-m1u admits the session for park and unpark.
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
- D2. **A person's park is a decision.** A parked row whose `Waiting.By`
  starts with `human:`, the engine's own test of a human park
  (verbs.go:2772), leaves the inbox for a Decided tab named "Not now",
  with its reason, who parked it and when, and the goal link; a blocker
  park a human directed is one too, named with its blocker, and leaves by
  itself when the blockers finish. A seat's park stays in the inbox as
  today, because a human has not seen it.
- D3. **The queue row is one line that opens.** Left, a checkbox; then the
  title, at most two lines; right, the age, a tier chip where tier is
  above zero as the board's row shows it (src/backlog/GoalRow.tsx:103), a
  "yours" chip for `origin: human`, the labels as chips, at most three and
  "+n", Approve, and Not now. The row's silence is said once, in the block's head:
  "If you do nothing, these stay in To Do and no seat may claim them." The
  derived `asked` sentence is not shown. Opening a row shows the whole
  intent, the next step, the budget tuple in words or "no budget recorded",
  what blocks it, its priority band and the goal link. Open state is page state, not stored.
- D4. **Queue tools, three of them.** A Find box over id, intent and
  labels, the board's `matchesText` extended to labels; label chips drawn
  from the rows shown, each with its count, one selected at a time,
  "yours" and "seats'" beside them as origin chips; and an order toggle,
  backlog order (default, the board's) or newest first by `openedAt`. The
  block's count reads "122 waiting · 16 shown" when narrowed. Nothing is
  persisted.
- D5. **Select and act.** A checkbox per row, "Select all shown" in the
  block's head, and two buttons, "Approve 12 selected" and "Not now for 12
  selected", each opening one sheet. The approve sheet lists the selected goals, each with the budget it would be
  approved with and the source's words from `prefillFor`; a goal whose
  prefill is null is listed as "needs its budget first: approve it alone"
  and is not sent. One Approve sends the rest in order, one POST per goal,
  as the routes are, with "7 of 12" while it runs; the first failed answer stops the run and the sheet says at which goal
  and with what words, the engine's; a failed answer can follow a publish, so the page's re-read, which
  reports the accepted ref as observed, not the sheet's count, is what
  says what landed, and a goal whose answer failed is named as
  unresolved, not as refused; then the page reads its payload again. The not-now sheet
  is the same list with one reason field, required, applied to every goal,
  the same run and the same stop rule. Sign-in is as today; the single-row
  Approve keeps the existing sheet and the single-row Not now is the
  not-now sheet with one goal.
- D8. **Not now, and back, from the page** (R-125-m1u). A queue row and
  the selected rows can be parked with a reason; a Not now row and a
  seat-park row in the inbox can be returned to the queue. Section 5 says
  how the engine admits it, the act layer carries it and the page asks it.
- D6. **The register is read from where it is, and opened from there.**
  `rulings.Read(roots.Installation)`, because the kit's memory home is
  under the installation on every layout the interface serves; the channel
  and journal readers stay at the checkout, where the steward writes them.
  The payload carries the register's checkout-relative path, derived from
  the two roots, and every register destination uses it, so "open the
  register" opens. The steward's own sweep root is separate work, noted in
  section 6.
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
`register` is the register's checkout-relative path, `metasystem/memory/rulings.md`
here, and the `where` of every ruling review and register destination
names it. `counts.needsYou` is the inbox as listed; `asked` and `waiting`
are its two blocks. The approval `Need` gains nothing: the row it carries has every
field D3 and D4 read. The page splits by kind.

## 4. The page

`DecisionsPane`, the same route. Header: "10 asked of you · 122 waiting
for your approval", each a link to its block. Block one is the g1-s44
inbox less the approvals. Block two: a head with the title, the count,
the silence sentence, Find, the label and origin chips, the order toggle,
"Select all shown" and the approve-selected button, which is disabled with
the count at zero; then the rows of D3, in the board's `ms-goal-*` idiom
where a class fits and the page's own tokens otherwise. A row's disclosure
is the Fleet row's pattern as g1-s45 built it, opening a full-width block
under the row.
The bulk sheet is the board's `Panel` chrome with a list inside, one line
per goal: title, the budget in the tuple's five words, the source's
words, or the exclusion line. Decided gains the "Not now" tab, a list of
the Overview's item rows with the reason as the note and a "Return to
queue" button, which sends unpark for that one goal and re-reads; the
seat-park rows of the inbox carry the same button instead of the unpark
command. Empty states: "Nothing is asked of you" and
"Nothing waits for your approval". Phone width: the queue row stacks,
the checkbox stays left, the tools wrap. No timers; one EventSource; the
page reads on mount, Refresh and after an act or a run of acts.

Help terms: `needs-your-choice` rewritten for the two blocks; new
`asked-of-you`, `waiting-for-approval` (what an approval is and that the
row's budget is the one the approval carries), `not-now` (a park with its
reason, a human act, and what returns it), `act-selected` (one act per
goal, stops at the first refusal), `return-to-queue`. The Partner's page capture
adds the block open, the narrowing in force and the selected count.

## 5. Not now and back: park and unpark from the page

Under R-125-m1u, in three layers, each the smallest that works:

1. **The engine admits the session at three rows.** `humanAuthorityRow`
   gains a flag, set at exactly the rows the ruling names: the park of a
   human-origin goal (verbs.go:2647-2651), the unpark of a human park to
   queued and to approved (verbs.go:2772-2784); `requireHuman`
   (verbs.go:314-332) gains one clause after the grade checks: a flagged
   row is met by a proof that is `SessionValidFor` the endpoint's root.
   The two other rows those verbs carry keep their grades: the park of
   another pair's claim (verbs.go:2654) and the early lifting of a seat's
   blocker park (verbs.go:2787) still refuse a session, so a queue goal
   claimed between the read and the act is refused, not displaced. No
   other verb changes. The park and unpark mutations call
   `recordSessionAuthority` on the history line they append, as approve
   does, so the ledger names the session as the hand. The `Parked.By` a
   session writes is `human:<name>` already.
2. **The act layer carries park and unpark.** `act.Authority.Park(id,
   because)` and `Unpark(id)` beside `Withdraw`, publishing `goal.Park` and
   `goal.Unpark` through the same `request()` and `settle`. `request()`
   sets `ParkBranchCheck` from the check the command edge builds today,
   which moves into a package both can import: the two tip readers, the
   scrubbed git wrapper and the operation id (goal_branch.go:26-33,
   692-735, 1033-1039) become `internal/goal/branch`'s own, and
   cmd/metasystem keeps one-line wrappers. Behaviour unchanged: where the goal has a local branch or a named
   unit commit the check reads the endpoint's main and the goal's branch
   from the remote and refuses an endpoint that is not `refs/heads/main`;
   where it has neither it answers without a remote read
   (internal/goal/branch/status.go:130); its summary lands on the next
   step as it does from the terminal. A run of parks over branch-backed
   goals is two fetches each beside the publish's own; the sheet's
   progress line is what covers that in step 2.
3. **Two routes.** `/api/backlog/goals/<id>/park` with body `{because}`,
   refused empty as the engine refuses it, and `/api/backlog/goals/<id>/unpark`
   with an empty object, both the policy of every act route (`mayAct`,
   `decode`, `answerAct`, acts.go:239-249) and answering the board as the
   others do. The board's own six acts are untouched; the board does not
   grow a park button in this step.

What park and unpark refuse on this page stays the engine's: a goal
already parked, a claimed goal of another pair, a seat's blocker park
lifted early, an endpoint the branch check cannot read; each refusal is
shown in the engine's words and stops a run. An unpark returns a goal to
approved where its approval still stands and to queued otherwise
(approval.go:381); a human's own park may be lifted before its blockers
finish. The walkthrough's acts are canned and its ledger has no
publishable endpoint (walkthrough/main.go:195, 430), so its screenshots
are evidence of the page and of a refusal shown, and the engine, act and
branch-check proof is in their own Go fixtures.

## 6. Not here, step 3 and later

The steward's review sweep reading the register from the checkout root
on this layout, where the file is under the installation: separate work,
not this page's. Server and board rank order of zero-ranked rows made
identical through `inRankOrder`. Bulk unpark. Park from the Backlog board.
Persisted narrowing. Grouping by goal family or arc. Keyboard
triage. One publish for many approvals. A Find box on the Approved tab.
The rest of g1-s44's section 6.

## 7. Verification and box

Go: composition tests on the split counts, a human park in `notNow` and
a seat park in the inbox, the schema number; a test on the wiring that the register reader is handed the installation
root and the payload the checkout-relative path, and a fixture check that
the review card's destination opens; engine tests that a
session proof is admitted for a human-origin park and a human-park unpark
and still refused for another terminal-grade row, and that both history
lines name the session; act tests for Park and Unpark with a fixture
endpoint and an injected branch check; route tests for the two routes' policy and refusals; engine tests that
the park of another pair's claim and the early unpark of a seat's blocker
park still refuse a session, one of them with the claim landing between
the page's read and the act; the branch check's moved tests move with it. Frontend: pane tests
on the two blocks and their counts, a queue row opening, Find over a
label, a label chip narrowing, the order toggle, select-all over the
shown rows only, the bulk sheet excluding a null prefill, sending in
order, stopping at a refusal and re-reading; the not-now sheet refusing an
empty reason and sending; Return to queue; the guards stay green. The
walkthrough fixture grows to thirty approvals with labels, tiers and both
origins, three human parks and one seat park, and the screenshots at 1280
and 400 are the evidence: the header, the queue narrowed, a row open, the
approve sheet, the not-now sheet, the Not now tab. Budgets as always. Box: one build lane
(Claude on Opus), one code read (Codex on Sol) with one fix round under
R-124, after Astra's read; two attempts, 150 to 240 job-minutes.

## 8. Self-grade

High on D1 to D4 and D7: they rearrange what the payload already carries
and reuse the board's filter and row idioms. Medium on D5: it is the first
act on this interface that runs more than one publish, and the stop-at-
first-refusal rule is the whole of its safety; the engine's per-goal
refusals are the words shown. Medium on D6: the root is proven wrong for
the register; the other two readers move only on proof. Medium on D8: the
admission is two verbs wide and cited to the ruling, but it is the first
time a session proof meets a terminal-grade row, and the branch check
moves packages; the tests named in section 7 are what hold it. Weakest:
the cost of a run of parks, two fetches each, is accepted rather than
measured.

## Dispositions (Astra read, 2026-09-25, under R-124)

Read of the first draft, before section 5 was added. Two material
findings, six deferred; every code claim checked before folding.

| id | finding | fold |
|---|---|---|
| F1 | reading the installation register yields review cards whose destination, `memory/rulings.md`, the document reader opens relative to the checkout, where it does not exist | the payload carries the register's checkout-relative path and every register destination uses it |
| F2 | the terminal park and unpark commands shown lacked `--by`, so copying them refuses a human act | moot: section 5 replaces every shown command with the act itself |

Astra's root trace corrected D6's premise: `up` launches the steward with
the checkout root, so the steward's sweep reads the same absent file on
this layout, and the journal and channel are checkout-relative; the
register reader alone moves, the sweep is separate work. Folded because
they cost nothing: a human-directed blocker park belongs in Not now; "no
budget recorded" where the row carries none; a failed answer can follow a
publish, so the re-read is what says what landed; the Fleet disclosure
described as built. Deferred, step 2 works without them: rank parity of
zero-ranked rows with the board.

## Dispositions (Astra second read, 2026-09-25, on section 5, under R-124)

One material finding, four deferred; every code claim checked.

| id | finding | fold |
|---|---|---|
| F1 | a verb-wide admission would also admit the park of another pair's claim and the early lifting of a seat's blocker park, rows the ruling does not name; a queue goal claimed between the read and the act could be displaced | the admission is a flag on the three named rows only; the two other rows keep their grades; tests for both refusals, one with the claim landing between read and act |

Folded because they cost nothing: the branch check reads the remote only
where the goal has a branch or a named unit commit; the walkthrough
cannot publish a park, so its screenshots are page and refusal evidence
and the proof lives in Go fixtures; unpark returns to approved where an
approval stands; a human's own park may be lifted early; a failed answer
after a publish is named unresolved and the re-read reports the accepted
ref as observed. Nothing deferred beyond the first read's list.
