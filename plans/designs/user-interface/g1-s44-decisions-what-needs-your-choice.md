# g1-s44: Decisions — what needs your choice, and what you decided

- Kind: design
- Id: 01M3BXKREHBQD2F88G0MKZJY67
- Status: draft
- Goals: browser-interface

Wido, 2026-09-25: "put your UX cap on again and think about what we want to
see and do on the decisions page", then "design and build me this". Author
Fable. Step 1 of the Decisions section; every cite re-read at `5fcdd7d0f`.

## 1. What exists and binds

1. **The section is reserved and empty.** `routes.ts` lists `decisions` at
   `/decisions` with the icon Gavel and the sentence "A list of what is
   waiting on a human, with what is being asked, which seat asked it, and
   what answering it would do", `projected: false`, rendered by the
   placeholder. The interface design's row asks: what needs a human choice,
   and what was decided? The help register has `decisions` (the record kind)
   and `goal-decisions` reserved.
2. **What waits on a human is already computed in pieces.** Overview's
   `needsYou` (internal/ui/overview/overview.go:392-405) counts To Do goals
   without approval (`awaitingApproval`), open register questions, draft
   records, landed designs not marked done, recent steward alerts, and
   sign-in. A backlog `Row` carries `Approved{By, At, Authority, ReviewBy,
   Expired, ExpiredWhy}`, `Waiting{Reason, Since, By, From, Blocker}` and
   `Fence{Reason, ClosedAt}` (internal/backlog/project.go:36-80); a park
   with an empty `Blocker` is a person's own park, a dependency park carries
   the blocker (src/backlog/Board.tsx:882-905). Seat asks to the human live
   in this seat's channel questions, `channel.WalkOpenQuestions(repo)`
   (internal/channel/question.go:142-168), each with its question, facts,
   what is wanted, options and a budget.
3. **The human's decisions are on record in four places.** The rulings
   register memory/rulings.md, one row per ruling: `| id | date | ruling |
   context | owner | review condition |`, the review condition being
   `class=<standing|temporary|delegated-authority|assumption-dependent>
   due=<date>` or `event=<text>`; the steward already parses it
   (`readRulingReviewRegister`, `parseReviewCondition`,
   internal/steward/ruling_sweep.go:46-130) and sweeps temporary rulings
   whose due date has passed, offering "adopt, revise or withdraw"
   (ruling_sweep.go:242-260). Decision records in docs/decisions (one
   today). Answered rows of the questions register (`answered: <ref>`).
   Approvals on goals (`Row.Approved`).
4. **Acts the interface has.** `/api/acts/approve` with the complete budget
   tuple a human confirmed, and `/api/acts/withdraw` (internal/ui/httpd/acts.go:76,
   217-245), both through the board's `ActSheet` (src/backlog/ActSheet.tsx,
   `Asked{move: approve | withdraw, goal: Row}`, Board.tsx:75, 635-636),
   which prefills the budget from the goal. Unpark, resume, grant, revoke
   and accept-risk are human acts at a terminal today.
5. **The seat-communication law binds every ask** (docs/seat-communication.md,
   rules 2 and 3): a question to the human states each option's consequence,
   what happens if the human does nothing, and the recommendation. The
   Partner's page capture names the section and the rows shown (g1-s29).

## 2. Decisions

- D1. **Two questions, one page.** What needs your choice, then what you
  decided. The third question, what authority you delegated and until when,
  is step 2; step 1 shows its one urgent part, temporary rulings whose review
  is due, in the inbox.
- D2. **Every inbox item says what silence does.** No item is a bare label:
  what is asked in one sentence, who asks, since when, what happens if you
  do nothing, and the asker's recommendation when the record carries one.
  The silence line is literal per kind and never invented; where a record
  has no recorded consequence the line says what the machinery will do,
  which is the consequence.
- D3. **Decide in place only with acts the interface has.** Approve and
  Withdraw open the board's own sheet. Every other item links to where the
  decision is made, the record's page or the goal's page, or names the
  terminal command, in the words the engine uses. No new authority, no new
  verb in step 1.
- D4. **Rulings are shown verbatim.** Your words are the record; the page
  renders each row whole, with its context, owner and review condition, and
  marks a temporary ruling whose due date has passed. Goal ids and record
  ids that appear verbatim in a ruling's words become links, called
  "mentions", never a claimed subject.
- D5. **The rulings reader is one package.** The steward's parser is lifted
  into `internal/rulings` and used by the steward's sweep and by this page,
  so the register has one grammar and one set of defects.
- D6. **Composed on the server.** One payload, judged nothing in the browser,
  re-read on the ledger and notification events the page already receives.

## 3. The payload: `GET /api/decisions`

```
{ "schemaVersion": 1, "readAt": "...", "signIn": false,
  "needsYou": [ { "kind": "approval" | "renewal" | "ask" | "question" | "parked" | "stopped" | "draft" | "landed" | "ruling-review" | "alert",
                  "id": "...", "title": "...", "asked": "one sentence", "by": "who asks", "since": "RFC3339",
                  "deadline": "RFC3339" | "", "silence": "what happens if you do nothing", "recommend": "" ,
                  "where": { "kind": "goal" | "record" | "question" | "register" | "notifications", "id": "..." },
                  "act": "approve" | "withdraw" | "", "command": "", "row": <backlog row> | null } ],
  "decided": { "rulings": [ { "id": "R-124-m1u", "date": "2026-09-25", "words": "...", "context": "...", "owner": "Wido",
                              "class": "temporary", "due": "2026-10-02", "event": "", "duePassed": false, "mentions": ["g1-s43"] } ],
               "defects": [ "R-9: review condition needs due= or event=" ],
               "decisions": [ <record items> ], "answered": [ <question items with their answer reference> ],
               "approved": [ { "id": "goal", "title": "...", "by": "...", "at": "...", "authority": "...", "expired": false } ] },
  "counts": { "needsYou": 7, "rulings": 158 } }
```

The inbox, item by item, with its silence line:

| kind | source | asked | silence | act |
|---|---|---|---|---|
| approval | To Do rows with no approval (`awaitingApproval`) | "Approve <title> for execution" | "it stays in To Do and no seat may claim it" | approve, with the row |
| renewal | rows whose approval expired | "Renew the approval of <title>: <expiredWhy>" | "no seat may take new work on it" | approve, with the row |
| ask | this seat's open channel questions | the question, with its facts folded behind it, what is wanted and the options | the record's own budget and deadline, else "the asking seat waits at a safe stop" | none; "answer on the fleet channel with your code" |
| question | open rows of the questions register | the question | "it stays open; whoever raised it proceeds on the assumption it named" | none; opens the register in the reader |
| parked | Waiting rows with an empty blocker, a person's own park | "Unpark <title>? parked by <by> <since>: <reason>" | "it stays parked" | none; `metasystem goal unpark --id <id>` |
| stopped | rows with a fence | "Resume <title>, stopped <closedAt>: <reason>" | "it stays stopped; its claim keeps the goal" | none; `metasystem goal resume --id <id>` |
| draft | draft records | "Accept the draft <title>?" | "it stays a draft, shown as one on Project" | none; opens the record |
| landed | designs whose goals landed, not marked done | "Mark <title> done?" | "it stays accepted on a landed goal" | none; opens the record |
| ruling-review | temporary rulings with a due date at or before today, and event rulings the sweep marks due | "Review <id>, due <date>: adopt, revise or withdraw" | "it stays in force as written" | none; opens the register at the row |
| alert | steward alerts of the last seven days | the alert's own line | "" | none; opens the notifications panel |

Order: items with a deadline first by deadline, then renewals and reviews
by how long past due, then approvals in backlog order, then everything
else oldest first. `signIn` is a row of its own at the top, as on Overview.

Decided: rulings newest first, whole; decision records and answered
questions from the project pane; approvals from the rows that carry one,
newest first. Defects of the register are listed, not hidden, in the
steward's own words.

## 4. The page

Route `/decisions`, `DecisionsPane`, `projected: true`, the route sentence
rewritten to what the page shows. Two blocks:

1. **Needs your choice**: one list, in the order above, each row a card in
   the Overview's item style: a kind chip, the title, the asked sentence,
   a muted line "asked by <by>, <age>; if you do nothing: <silence>", the
   recommendation when there is one, and on the right either the act button
   (Approve or Withdraw, opening the board's `ActSheet` with the row) or the
   link, with the terminal command in a code span when that is the way.
   Empty: "Nothing is waiting on you." At phone width the row stacks.
2. **Decided**, with four tabs, counts in the tab names: Rulings (default),
   Decisions, Answered, Approved. Rulings has a Find box over words and
   context and a class filter; each ruling is a card: id and date, the words
   whole, the context behind a disclosure, owner, and the review condition
   as a chip, "review due 2 Oct" or "review passed 11 days ago", with the
   mentions as goal chips. Defects, when any, in one quiet line above the
   list. The other three tabs are lists of the Overview's item rows.

Help terms: `decisions-page` (the section), `needs-your-choice`,
`silence` (what doing nothing does), `ruling`, `review-condition`, and the
`decisions` term kept for the record kind. The Partner's page capture on
Decisions carries the inbox rows shown (kind, id, asked) and the tab open,
never a ruling's full text beyond the ids. The page re-reads on the
`ledger` and notification events it already receives, never on a timer.

## 5. Authority and acts

Approve and Withdraw are the existing signed-in acts, unchanged, through
the board's sheet, which prefills the budget and refuses an incomplete
tuple. Nothing else on the page acts; the words name the act and where it
is made. The register and the records are opened in the reader the
interface has; editing them is the editor's existing act.

## 6. Not here, step 2 and later

Rule from the page (a composer appending a row in your words). Answering
a question and accepting a draft from the inbox. Unpark and resume from the
page once the session proof is admitted for them. Standing authority with
clocks: powers of attorney, temporary enrollments, delegated-authority
rulings, sorted by expiry. Per-goal filtering on the goal page. A
`decisions` Partner tool.

## 7. Verification and box

Go: `internal/rulings` lifted from the steward with the sweep's tests moved
and a register fixture holding all four classes, an event row, a malformed
row and a row mentioning a goal; unit tests on the composition with fakes
for each inbox kind, the order, every silence line, the deadline and due
arithmetic on an injected clock, an ask with and without a budget, sign-in;
the four decided tabs. Frontend: pane tests on the payload shapes (empty
inbox, every kind, the act rows, the tabs, phone width); the guards stay
green. The walkthrough fixture serves a synthetic register of eight rulings
and two channel asks, and the screenshots at 1280 and 400 are the evidence.
Budgets as always. Box: one build lane (Claude on Opus), one code read
(Codex on Sol) with one fix round under R-124, after Astra's read; two
attempts, 120 to 180 job-minutes.

## 8. Self-grade

High on the page: it is Overview's pattern twice over, with one new reader.
Medium on the silence lines: each is a claim about what the machinery does
when a human does nothing, and every line must be true of the engine as it
is; the build proves each against the verb or the projection it describes,
and a line the build cannot prove is replaced by "no recorded consequence"
rather than kept. Weakest: the channel asks are this seat's local files,
so a fleet where other seats hold asks shows only this seat's; the fleet
channel is where those reach you today, and the row says so. Reject
condition: Wido wants to rule from the page in step 1, in which case the
composer moves up and the inbox shrinks to approvals and reviews to keep
the slice small.
