# A review you can decide: findings as decisions, one path to a verdict

- Kind: design
- Id: 01M40QF3R8V2YH6N4TDK9WXZB7
- Status: draft
- Goals: review-findings-read-as-decisions

Design lane (Fable 5.1), 2026-10-03, from m1e's design input (`agentic-tools-evidence/review-sitting-ux-20261003/design-input.md`) and the room's code at `f8e12178d`. No critique round yet. Mocks: `~/metasystem-evidence/agentic-tools-ui/review-findings-20261003/mocks/` (blocking, clean, older; each at 1440 and 390 px).

## 1. Step back

One person, the owner, sits a review of a goal that waits to land. The machinery sent it to Review because the goal's tier asks for a person. Once per goal, a few minutes, on the laptop or the phone. He is not the builder and was not in the room that shaped the work.

He asks three things, in this order: Is there anything that must be fixed before this lands? What does the reviewer recommend? What do I decide, and what happens then? Everything else is evidence, available on request.

What he can do: decide each finding (four named acts), try the version, give one verdict (land it, send it back, or have the current version reviewed), or leave and come back. The reviewer in the room is the Project Partner: it reads the goal, the change, the critics' records and the proof, and reports in five walks (Asked, Built, Examined, Proven, Behaves).

## 2. Today's room, reviewed

Read from the two screenshots and `internal/ui/web/_app/src/review/` (ReviewRoom.tsx, room.ts, Answers.tsx, Pill.tsx) and `internal/ui/partner/review.go`.

| On screen today | What a person cannot tell or do |
|---|---|
| `Reviewing fleet-page-redesign (i) at tip e2f9c9292 . no findings yet` | which version this is in time, whether it is the one that would land, who built it |
| `the candidate is not running . Run (i)` | what a candidate is, what Run starts, where, and why he would press it |
| `panel.ts:284-313 . changes . the evidence` and the tabs `The change . The evidence` | the same two views twice; a file path as a chip |
| `Board / Desk`, `End`, `Step out` | which one ends the review, which one keeps it, what End does |
| A finding card: free text, `Anchor`, `Consequence`, `Record it`, `Dismiss` | the text is the critic's claim for an engineer (commit ids, Z-times, record paths); no severity, no recommendation, no count; nothing says what Record it writes, and Dismiss could mean not a problem, later, or accepted |
| Recorded, the card grows four more buttons: `Fix in this goal`, `Follow-up goal`, `Accept, with reason`, `Leave open` | two layers of buttons on one card; the first layer only writes the card into the record |
| End sheet: `Clear to land`, `Send back`, `End without a verdict` | the nod is refused three ways: an empty board ("has produced nothing anyone can be held to"), any recorded finding still `unanswered`, and a record of an older version ("bound to the tip it was drafted for"); none names the one act that resolves it |
| The moved banner: `The branch moved while you were out: reviewed at e2f9c9292, now at 8f78e73c` | said in commit ids; the one act, `Review the new tip`, is one of two buttons in a banner above a desk |

Internal words on screen: tip, candidate, Run, anchor, desk, Board, deposit, Record it, Dismiss, Consequence, unanswered, may have moved, the evidence, changes, Clear to land, Send back, commit ids, Z timestamps, record paths, `origin/goal/...`, land-ready. The Partner is told "never say whether to accept" and "never recommend" (`review.go`, the skill), so no card carries a recommendation, by design.

Why his nod was refused: his record named the version of 2 October; the branch had moved; the Outcome is bound to the version it was drafted for, and the landing gate asks for the word again at a moved tip (`goal/landgate.go:267`). The screen said this in ids and offered nothing.

## 3. The design

One page, one column, four numbered steps, read top to bottom. The desk is gone from the first view; the code opens from a finding when asked. The conversation becomes one question line at the bottom.

```
Review of <goal title>
The version of 2 October, 14:19, built by m1e (i) . This is the version that would land.
1 What you are looking at      one paragraph; if older: the notice and the one act
2 What the reviewer found      the summary line; one card per finding, in layers, with its decision
3 Try it (optional, folded)    Start / Open it / Stop, with what each does and where
4 Your verdict                 the recommended way first; Leave for now under it
[ Ask the reviewer anything about this version ]
```

### Step 1: What you are looking at

The title is the goal's title, not its id. The line under it says the version by time and builder ("the version of 2 October, 14:19, built by m1e"), the commit behind an (i), and one of two states: "This is the version that would land" (green) or "A newer version exists, from 3 October, 09:55" (ochre). Then one paragraph from the reviewer: the goal in one sentence, what this version adds, how many files it touches.

When a newer version exists, step 1 carries the notice: "This review does not cover the version that would land. Since the reviewer looked, the builder added ... A verdict given here would be about the older version, and the machinery would ask for your word again." With one press, **Review the current version**: the reviewer looks at the current version; decisions on findings that still apply are kept. Step 2 shows the old findings muted under "in the older version"; step 4 says "No verdict yet" and points at the press. Nothing else to press (mock `older`).

### Step 2: What the reviewer found

The summary line first: "3 findings. 1 blocks landing, 1 is worth fixing, 1 is a note. The reviewer recommends: send it back, because of the one that blocks. If you decide nothing on a finding, it follows the recommendation." A clean review says "The reviewer found nothing to raise. It looked at ... The reviewer recommends: land it." (mock `clean`).

The card, in layers, top to bottom:

| Layer | What it is | Rule |
|---|---|---|
| Severity word | Blocks landing (red), Worth fixing (ochre), Note (grey) | one of three; colour on the card's edge and the chip |
| Title | the problem in the person's terms, one sentence | no commit id, no path, no timestamp; refused otherwise |
| Why it matters | what happens if ignored, what is gained by acting; one or two sentences | plain words |
| The reviewer recommends | one of the four decisions and one sentence why | the person's default |
| Your decision | the four choices, each with its consequence; the recommended one marked; the chosen one marked | a choice that needs a reason asks for it in a small sheet |
| Evidence, folded | the finding as the reviewer wrote it, what it cites, "See the change", "See the record it cites" | today's text, anchor and consequence; the code opens from here as a sheet |

The four decisions, their consequence line, and what each is in the machinery today:

| Decision | Consequence shown before the press | What it is today |
|---|---|---|
| Must fix before landing | the builder gets this as a correction; the goal does not land until it is fixed and checked again | the answer `fix` on the finding's line; Send it back carries it in the correction brief (`goal review --verdict send-back --brief`) |
| Fix after landing | a follow-up goal is opened with this finding; this goal may land | the answer `follow-up`: the New goal sheet opens with the title as intent and the why as next step; the person answers the risk questions and presses Open; the answer names the goal |
| Not a problem (say why) | your reason is recorded with the finding; the reviewer reads it | new answer value `not a problem: <why>` on the same line; a record write, no engine change |
| I accept this risk (say why) | the goal may land; the acceptance is recorded in your name | the answer `accepted: <why>` on the finding's line, published beside the goal's history with the verdict. Not yet the goal's own accept-risk record: open question 2 |

A note offers Fix after landing and Not a problem; Worth fixing and Blocks landing offer all four. The record carries four new lines (Severity, Why, Recommends, Reason) beside Anchor and Answer, so a reload and the board read the same card. Nothing is called Dismiss: a card the person does not touch keeps the reviewer's recommendation and says so. Pressing the decision that already stands writes nothing (R-129-ui).

Under the cards, one line: "Want the whole picture? Ask the reviewer to walk you through: what was asked, what was built, how it was examined, how it was proven, how it behaves" (today's five walks, in words).

### Step 3: Try it

Folded by default. "Start runs the reviewed <app name> on port 7941 so you can click through it. It stays up until you press Stop." Running: "Running on port 7941: Open it . Stop". The three app routes as today; only the words change.

### Step 4: Your verdict

"What happens to this goal?" and two ways, the recommended one first, blue, marked "recommended":

| Way | Consequence shown | What it is today |
|---|---|---|
| Looks good, land it | the seat lands it on its next turn; N follow-ups are opened from your fix-after-landing decisions; recorded on the goal in your name | `clear-to-land` through `POST /api/backlog/goals/G/review` |
| Send it back | the builder gets your must-fix decisions as a correction; the goal leaves Review until it comes back fixed | `send-back` with the brief composed from the must-fix findings |

The recommended way is a rule, not the reviewer's word: Send it back when any finding has severity Blocks landing and its decision (yours, or the reviewer's where you decided nothing) is Must fix; otherwise Looks good, land it. When a newer version exists, step 4 offers no verdict and points at Review the current version.

The nod never refuses (R-142-m1e). Three cases, each one press plus at most one confirmation:

1. Nothing blocks and every finding is decided or follows its recommendation: the press records the decisions, writes the Outcome, records the verdict. One press.
2. Findings have no decision: the press asks once, listing each with what will be recorded ("2 findings follow the reviewer: fix after landing, not a problem"). Each fix-after-landing opens its New goal sheet in turn; then the verdict is recorded.
3. A finding blocks landing and the person lands anyway: the ask says the impact first (R-143-m1e): "One finding blocks landing in the reviewer's view. Landing means you accept that risk. Say why." One reason field, then the press. The acceptance is recorded with the impact text shown.

The Outcome is composed by the interface from the record (verdict line, what was looked at, every finding with its decision), not drafted by the Partner; the Partner's prose draft becomes optional (later). An empty board is no longer refused: the Outcome says what was looked at; when nothing was opened at all, the press asks once ("You have not opened anything of this version. Land it anyway?").

After the press, one banner in words: "Recorded on fleet-page-redesign as your verdict: looks good, land it. The seat lands it on its next turn. 1 follow-up opened: browser-tests-run-in-the-critics-sandbox." with Back to the board. Changing your mind before it lands means a new review from the board; there is no undo yet (later).

Under the ways, in small text: "Not ready? Leave for now: everything stays as it is, and the goal waits for you. End without a verdict: your hold is lifted, the goal lands under its own rules, and your decisions stay in the record." Leave for now is today's Step out; End without a verdict is today's third way, kept because a sitting on a waiting goal holds it, and a person who walks away for good must be able to let go. End and Step out as two header buttons are gone.

### The header, in words

`Review of <title>` / `The version of <day>, <time>, built by <seat> (i)` / the state. No tip, candidate, anchor, desk or Board. Back to the board at the top left, the sign-in state at the right. The server's Changes answer gains the reviewed commit's time and author and the branch tip's time, so these words need no Partner.

### Phone width

One column already. The four decisions stack full width; the ways stack; evidence folded; the question line sticks to the bottom (mocks at 390 px). No table layout anywhere.

### Words that leave the screen

| Today | Now |
|---|---|
| at tip e2f9c9292 | the version of 2 October, 14:19 (commit behind the (i)) |
| the candidate is not running . Run | Try it: Start this version on this computer |
| Anchor / the evidence / changes / the desk | Evidence, as the reviewer wrote it: See the change, See the record it cites |
| Record it / Dismiss | the four decisions |
| Clear to land / Send back | Looks good, land it / Send it back |
| Step out / End | Leave for now / End without a verdict (quiet) |
| The branch moved while you were out: reviewed at ..., now at ... | A newer version exists, from 3 October, 09:55. This review does not cover it. Review the current version |
| unanswered | follows the reviewer: <recommendation> |

## 4. Where the plain text comes from

Choice for step 1: **(b), the reviewer in the room writes the layers.** Reasons:

1. The cards on screen are the Partner's own findings, written in its walks (`ReviewOpeningRequest` in `review.go`). The critic's return (schema v4: claim, evidence, severity, material) feeds the Partner's Examined walk; it does not make the cards. For its own findings the Partner is the first author, not a second one.
2. (a) changes the critic's return schema and adds an engine audit, both owned by `critique-stops-on-convergence`, which is at "proper design first". Waiting on it leaves Wido with today's room.
3. (b) is this lane's code: the finding deposit (`partner/host.go` Deposit) gains severity, title, why, recommendation and reason; the opening request asks for them in plain words; the admission refuses a title with an id, a path or a timestamp.

(a) later, as the design input allows: when the critic's return carries the same fields, the Partner relays the critic's words for findings it takes from a read instead of re-authoring them. The names are fixed here so both sides agree: `severity` (blocks, fix, note), `title`, `why`, `recommend` (must-fix, fix-later, not-a-problem, accept), `reason`.

One rule changes: the Partner is told today "never say whether to accept" and "never recommend". The reviewer now recommends one decision per finding and says why; the verdict stays the person's, derived by the rule above (open question 1).

## 5. Step 1 first (R-121-m0)

The smallest slice that makes a sitting decidable: a finding arrives in plain layers with a recommendation; the page reads as four steps in one column; the nod is one press that asks instead of refusing; an older version says so with its one act; the header is in words.

Builds (all under `metasystem/`):

- `internal/ui/partner/host.go`, `review.go`, `service.go`: the five fields on a finding deposit; the opening and walk requests ask for them; the admission refuses a missing field or a title with an id, path or timestamp, in words. `project-partner.skill.md` and `skills/project-partner/SKILL.md` (byte-identical): "In a review, examine, then recommend one decision per finding; never give the verdict."
- `internal/ui/review` and `httpd/review.go`: the Changes answer gains `at` and `by` for the reviewed commit and `currentAt` for the branch tip.
- `web/_app/src/partner/sitting.ts`, `api.ts`, `Deposit.tsx`: the four record lines; the card in layers.
- `web/_app/src/review/room.ts`: summary counts, the recommended verdict rule, the four decisions (today's ANSWERS renamed, `not a problem` added, `left open` dropped), the Outcome composed locally, the nod's ask list. `Answers.tsx`: the decisions. `ReviewRoom.tsx`: the four steps in one column, the code as a sheet, Leave for now. `Pill.tsx`, `help/terms.ts`, `room.css`: words. Bundle rebuilt in the same commit.

Proof (tests that fail first):

- Go: a finding without severity, title, why or recommendation is refused naming the field; a title carrying `e2f9c92`, `panel.ts:284` or `12:19:54Z` is refused; the record carries the four lines; the Changes answer carries the times.
- Vitest: the summary line from three findings; the recommended verdict by rule (blocks and must-fix: send back; blocks accepted: land; none: land); the nod's ask lists undecided findings with their recommendation; a blocking finding under a nod asks for a reason and refuses an empty one; an older version offers one press and no verdict; `literals.test.ts` gains the banned words (tip, candidate, anchor, desk, Dismiss, unanswered) for the room's strings.
- Browser: the walkthrough harness (port 7941, canned Partner) at 1440 and 390 px, the three states against the mocks; screenshots into the evidence folder.

Later, when it hurts:

- (a): the critic's return carries the fields; the Partner relays, not re-authors (critique-stops-on-convergence).
- One-press follow-ups: the reviewer supplies the risk answers for a fix-after-landing goal.
- `goal accept-risk` from the browser, so an accepted risk is on the goal itself (needs a ruling, as park and abandon did).
- An undo for a minute after a verdict.
- The step number of a version ("step 2") once goals carry their slices.
- Try it stops by itself when the review ends.
- The Partner's prose Outcome as an optional "write it up".

## 6. Open questions for Wido

1. The reviewer recommends. Today the Partner must not. Recommendation: yes; one decision per finding with a reason, the verdict by rule, the person decides. The request text and the skill change in the same commit.
2. Accept this risk from the browser. Step 1 records it on the finding's line in the record, published with the verdict. The goal's own `accept-risk` is a terminal verb; admitting the signed-in session needs a ruling row like R-125-m1u. Recommendation: step 1 as drawn; the ruling and the route as step 2.
3. Fix after landing opens the New goal sheet, prefilled; you still answer the four risk questions (or ask the Partner for them). Recommendation: accept for step 1; one press later.
4. The nod on an empty board is allowed and asks once. It overrides g1-s65's rule R8 ("nothing anyone can be held to"). Recommendation: allow, under R-142-m1e; the Outcome records what was looked at.
5. End without a verdict stays as a quiet line under Leave for now, because a sitting on a waiting goal holds it. Recommendation: keep.
6. On a clean review, Send it back asks you to write what must change (one text field becomes a must-fix finding of your own). Recommendation: yes; without it a clean review can only nod.

## Rulings that bind

R-121-m0, R-124-m1u (step 1, materiality); R-142-m1e, R-143-m1e (never refuse a person; impact first); R-129-ui (a repeated press is success); R-126-m1e (every act here is a public verb or a record write); R-125-m1u, R-128-ui, R-142-ui (what a session may do: the verdict and the app run already admit it; accept-risk does not yet); R-135-m1e (what a person must produce is prose: a reason, a question); R-4 (a fix-after-landing decision links an open goal, never prose alone).
