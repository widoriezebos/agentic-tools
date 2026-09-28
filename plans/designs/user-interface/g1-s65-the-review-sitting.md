# g1-s65: the review sitting — a piece of landed work on the table

- Kind: design
- Id: 01M3J3V8CBYBGP36BZYS8JM98F
- Status: draft
- Goals: browser-interface

Wido, 2026-09-27: "read plans/paper and tell me what would be a good next
thing to add to the UI. I'm thinking of support for human review of a
goal (explanation of design, decisions, code, coding decisions, showing
mermaid diagrams explain, runtime behaviour etc in the form of a sitting,
in dialogue with the project partner OR a different agent equipped for
specifically this task ... almost like discussing a piece of work that
was built with a fellow engineer and a whiteboard together in a room with
the laptop and the code at hand ... UX IS KEY." Author Fable. Every cite
re-read at `f9385b1ca`.

## 1. What exists and binds

1. **The paper's review sitting** (docs/paper/15-the-sitting.md, "The
   review sitting"): the same form as the sitting that shapes a design,
   "but the human is there as an examiner rather than as the
   intent-holder. The narrator's report brings the review's question and
   evidence into the room. There the human examines as an engineer: asks
   for the case that was not tried, challenges an assumption the checks
   share, demands more evidence or accepts a stated risk within their
   recorded authority. What leaves a review sitting is, again, records:
   findings answered one by one, a ruling, or a named demand for
   evidence. And a review sitting that ends in a nod has produced nothing
   anyone can be held to." The machinery in the room judges nothing
   (chapter 15, "Who is in the room"); a fresh mind is the only fresh
   perspective, and "the partner who helped shape a design is the last
   one who may judge it" (its title line). Chapter 13's reviewer "may
   demand more evidence, narrow or stop exposure, and authorize
   acceptance within scope or refuse it"; "a bare approval cannot explain
   which risk was accepted". Chapter 6 names when human review is owed:
   a value judgment exposed, an irreversible or severe act, unfamiliar
   work with weak tests, or builder and examiner sharing a model. Chapter
   14: the machinery "explains what the evidence supports, and the
   engineer's follow-up questions guide the conversation"; a person who
   has seen the builder's reasoning may learn from it but may not examine
   it independently. Proportion holds: "a mechanical repair with a clear
   defect needs no sitting".
2. **The sitting as built** (g1-s53, g1-s55): a sitting is a record with a
   conversation attached; its working material is four sections of that
   record, written by the human's press through one serialized recorder
   (`partner/sitting.ts`, `partner/recording.ts`); the Partner's
   `deposit` tool offers facts, proposals, decisions, open questions,
   cases and the outcome, and writes nothing; a case card carries Decide,
   Leave open and Dismiss (`partner/Deposit.tsx`, `CaseCard`); End drafts
   the Outcome; Project → Sittings lists records that carry deposit
   marks; the Partner's instructions forbid recommending in a sitting.
   The service admits two purposes and says the rest itself:
   `admitsPurpose` refuses anything but shape intent and shape a design
   with "review and learning sittings are not in this build"
   (`internal/ui/partner/service.go`, the sitting section), and a sitting
   is refused on any record but an intent or a design (`admitsSubject`).
   One conversation per human per checkout, kept outside the checkout
   (`internal/ui/partner/conversation.go:20-50`); whether the Partner's
   provider session is fresh is the server's one decision, and a fresh
   session gets the opening turn again (g1-s55 D3).
3. **What a piece of landed work leaves behind.** A goal file
   (`internal/goal/file.go:23-91`) carries its intent, tier and risk
   record, budget, approval, review obligations, accepted risks, read
   items, history and, when done, one concluding sentence. It carries
   **no landing commit**: the commit names the goal in a `Goal-Item`
   trailer (`internal/landing/held.go:313`), and binding the two records
   is open work (`plans/goals/commit-goal-binding.md`). A design names
   its goals one way (`- Goals:`, g1-s26); this interface's own slices
   bypass the ledger, name no goals, and their Built sections name the
   commits that landed them. Beside each design stand Astra's critique
   and Sol's read, with the dispositions folded into the design. Proof
   runs and screenshots go to the evidence root, which is configured
   outside the checkout (`metasystem/metasystem.conf:148`) and named only
   by a design's Evidence line; nothing in the interface resolves it.
   What a human can record about landed work today: approve before,
   done after, accept-risk on a critic's finding, and rejection as prose
   in Next step. There is no record that says a human examined a piece
   of landed work and what they made of it.
4. **What the interface shows** (survey of 2026-09-27): a goal page with
   the records that name the goal, its slices and its blocks; a document
   reader over the server's typed Markdown tree, which draws a mermaid
   fence as a code block with its language caption ("no diagram is drawn
   in this build", `internal/ui/markdown/markdown_test.go:149-163`;
   `project/Markdown.tsx:81-128`); no diff, no source file view, no
   evidence view. The Partner is an ACP runtime with Read, Glob and Grep
   inside the checkout and nothing else, plus one tool server of eleven
   reads and three offers (`internal/ui/uitools/uitools.go:70-83`); it
   has no git and cannot reach the evidence root. The New goal sheet
   opens with a prefilled intent (`backlog/OpenSheet.tsx:105`). The
   frontend carries no lazy chunk today and serves under a per-request
   CSP nonce (`httpd/httpd.go:342-354`); the cut guard
   (`src/cuts.test.ts`) refuses every network call outside the listed
   sites.

## 2. Your goals as a reviewer, and the room that serves them

You have a piece of work in front of you that machinery built, machinery
examined, and machinery proved. You are the one person who can be held to
having looked at it. What you want, in the order it happens:

- **R1. See what was asked and what landed as one thing.** The intent,
  the design's decisions and the commits that carried them, on one page,
  not in five tabs.
- **R2. Understand why.** Each decision in the design's own words, each
  claim about the code anchored to a line you can read.
- **R3. Have the code at hand, and a whiteboard.** Exact lines quoted when
  you ask; a drawing of the mechanism when words fail.
- **R4. Find what was not tried.** The case the examination missed, the
  assumption the tests share with the code, the evidence that is thin.
- **R5. Answer every finding yourself, then and there.** Demand a fix or
  more evidence, accept the risk with your reason, or leave it open;
  each one recorded as you say it.
- **R6. Leave a record, never a nod.** If the session died this second, a
  fresh worker and you could go on from what is written.
- **R7. A colleague who was not in the room.** Someone who reads the
  record and the code, not the conversations that produced them, and who
  never tells you whether to accept.

The room, from your chair:

You open a goal that is done, or a design whose Built section names its
commits, and press **Review it**. The sheet says one thing worth knowing
before you sit down: "Your Partner starts fresh for this: it reads the
record and the code, not the conversations that shaped them", and shows
what will be on the table: three commits, fourteen files, examined by
Astra in two rounds, Sol's read beside it, proof green at `4acaa5e7c`.
You press Start.

The Partner's first turn is the narrator's report, short and in five
parts: **Asked** (the intent and the design's Outcome), **Built** (the
commits, the files, the size), **Examined** (the rounds, the findings,
their dispositions, what was left as a residual), **Proven** (what ran,
at which commit, which tests failed first), **Behaves** (the screenshots
and the walkthrough, where the record names them, and "not recorded"
where it does not). Every claim carries its anchor as a chip. Under the
report stand five presses, one per part: press **Built** and the Partner
walks you through what landed, file by file, quoting the lines that
matter with their path and line. Nothing is weighed; the parts are a
table of contents, not a recommendation.

You ask: "why does the act layer take one lock over publish and
reconcile?" The Partner answers from the design's D3 and from
`internal/ui/act/owner.go:41-88`, quoted. You ask: "draw me the path of
a press from the card to the ledger." The Partner answers with a
sequence diagram, drawn, with the source one press away. You ask: "what
if the press dies between publish and settle while it holds the line?"
The Partner says what the record holds: the takeover the design built,
the test that holds it, and that the examination's cases were two tabs
and a slow answer, and none of them a death with the line held. It
offers that as **a finding**: the case, where it lives, what follows if
it is left. The card has three answers. **Demand** opens the New goal
sheet with the consequence written as an outcome and "from the review of
g1-s6x" in its words; when the goal opens, the finding is recorded with
"demanded: goal G2". **Accept** opens the small sheet you know from
cases: the clause as heard, and your reason, required; the finding is
recorded with "accepted" and the reason. **Leave open** records it with
its consequence. Dismiss folds it. You can state a finding yourself in
plain words; the Partner offers it back as a card with the anchor it
found.

The table beside the conversation now has a **Findings** pile beside
Facts, Decisions and Open questions, each finding with its answer. You
move to the goal page, read the design's Built section, come back; the
chip still says "Reviewing: g1-s64". You press **End the sitting**. The
Partner drafts what the review came to: what you examined, with the
anchors; the findings and what you answered on each; the risks you
accepted, with your reasons; the goals you demanded; what you left open;
what you did not examine. You edit it and press Record it. If nothing
was recorded, the sheet says so first: "Nothing is on the table. A
review that ends in a nod has produced nothing anyone can be held to",
and both ways out stay open. The review record now stands beside the
design, named on the goal's page, and the demands are goals in the
queue, waiting for your approval like any other.

## 3. Decisions

- D1. **A review is a record of its own.** Kind `review`, in a home
  beside the designs, created at Start by the server. Its head names
  what it reviews: `Goals:` the ledger goal where there is one;
  `Reviews:` the design's path where the subject is a design;
  `Reviewed:` the commits found (the `Goal-Item` trailer for a goal, the
  hashes in the Built section for a design) with "none found; write them
  here" where none were; `Evidence:` the path the design's Evidence line
  names, for you rather than for the Partner. Its sections are Facts,
  Findings, Decisions, Open questions and Outcome. The existing
  projection lists it on the goal's page and on Project by kind; nothing
  new lists it. Not the design record itself, because a design may span
  goals and its piles are the design room's.
- D2. **Review it** from a done goal's page and from a design's page.
  Purpose `review`; the sitting's subject is the review record, so every
  rule of the sitting holds unchanged. The sheet says the fresh-session
  line and what will be on the table, read from the record's head.
- D3. **The reviewer's Partner is fresh.** The review's turns run in a
  provider session started at Start with the boot context and the review
  brief, and none of the conversation's earlier turns; the transcript
  stays where it is. The Partner's instructions gain the review rules:
  bring what the record holds in the five parts; anchor every claim;
  name what the examination did not try, what the tests assume, and what
  is not recorded; offer the cases at the edge as findings; draw a
  mechanism, a path or a state when words would be longer; never say
  whether to accept. Not a second agent: one relationship, one store,
  one help register, and the paper's narrator is a function, not a
  person. Freshness is what "a different agent" buys, and a new session
  buys it.
- D4. **The opening turn and the five walks.** The opening turn is a
  fixed request the interface submits, as the sitting's is today, in the
  five parts. Under it, five presses, each a fixed request: Asked,
  Built, Examined, Proven, Behaves.
- D5. **The `landing` read.** One more operation for the Partner: for the
  review record it is sitting on, the commits its head names, the files
  they changed with counts, and per file the hunks, bounded per call.
  The Partner quotes with path and line; the human's own editor is the
  laptop in step 1.
- D6. **Findings as cards.** `deposit` gains the kind `finding` with text,
  anchor and consequence. The card is the case card's shape with three
  answers. Demand opens the New goal sheet prefilled; only when the goal
  has opened is the finding recorded, with `Answer: demanded — goal G2`.
  Accept opens the Decide sheet with the clause as heard and a required
  reason, and records `Answer: accepted — <reason>`. Leave open records
  `Answer: left open`. Dismiss folds. An entry's clause block may carry
  `Anchor:` and `Answer:` lines; the table's Findings pile shows each
  finding with its answer. The four piles are a property of the record's
  kind: a review's are Facts, Findings, Decisions, Open questions.
- D7. **Drawings.** A mermaid fence renders as a diagram in the transcript
  and in the document reader, the library loaded as its own chunk on
  first need, the source shown by a press and shown instead when the
  library cannot load or the diagram cannot parse. The first thing the
  builder does is prove the diagram survives the page's CSP (a nonce for
  the styles mermaid writes into its SVG, or a rendering mode that
  writes none); if it cannot, this decision waits and says so. Drawings
  the room used ride the Outcome draft; keeping one by itself is later.
- D8. **End.** The outcome draft for a review takes the shape §2 gives it.
  With all four piles empty, the End sheet says the nod line above its
  two ways out. Record it and end the sitting as today.
- D9. **No nag, no gate.** Nothing proposes a review, no inbox row, no
  badge; a review changes no goal's state. The review record is the
  human's account, and its demands are goals in the queue.

## 4. Step 1, the smallest thing that works

All nine, in two build lanes that do not touch each other's files: the
room (D1 to D6, D8, D9) and the drawings (D7). Not in step 1, each one
small when it hurts: the desk (a source file at a line, a diff page for a
review's commits, an evidence page that shows the screenshots), because
the Partner quoting the lines already puts the code at hand; Keep a
drawing; an `evidence` read for the Partner; a review before landing that
the landing consumes; the four risk questions proposing which landed
work deserves a look; Demand as `work revise` on a goal in flight;
answers on a finding from the table; the Partner proposing Review it;
learning sittings with the diagnosis withheld; several humans in one
review.

## 5. Payload and routes

`POST /api/partner/sitting` takes purpose `review` with subject `{kind:
goal | design, id}`; the server creates the review record in its home
and opens the sitting on it; the conversation's `sitting` carries
`purpose` as today. The record creator gains the kind, its template and
its home (`internal/ui/project/write.go`, `templates`, `homeFor`) and
the resolver its kind. `deposit` gains `finding`; `uitools` gains
`landing`. `entriesIn` reads an `Answer:` line beside `Anchor:`. The five
walks and the opening turn are fixed requests with the interface's
provenance, as the opening and closing turns are. The New goal sheet is
opened with its prefill. The transcript and the reader render mermaid
fences through one lazily loaded chunk.

## 6. Not here, later

The desk views; Keep a drawing; the evidence read; review as a gate; the
risk questions' prompting; answers from the table; the Partner proposing
a review; learning sittings; multi-human review; the "rulings always
match the preparation" evidence; a run of the application from the room.

## 7. Verification and box

Go: `admitsPurpose` admits review and `admitsSubject` the review record;
the kind's template and home; the head's `Reviewed:` line from the
trailer and from a Built section, and the "none found" line; `landing`'s
bounds and refusals; `finding`'s bounds; the review's provider session
carries no earlier turn (proven by what the fake runtime received); the
opening turn's and the walks' provenance. Frontend: Review it on a done
goal's page and a design's page, and not elsewhere; the sheet's lines
from the head; the finding card's three answers and their writes, Demand
recording only after the goal opened, Accept refusing without a reason;
the Findings pile and the Answer lines read back; a mermaid fence drawn,
its source by a press, and the fallback when the chunk fails; End's nod
line with empty piles and not otherwise; the guards stay green; phone
width. Walkthrough: the fake Partner answers a canned opening turn, one
finding and one drawing; screenshots at 1280 and 400: the sheet, the
opening turn with its five presses, a finding card and its three
answers, the Findings pile, a drawing, the End sheet with the nod line.
Box: two build lanes (Claude on Opus), one code read each (Codex on Sol)
with one fix round under R-124, after Astra's read; two attempts, 240 to
400 job-minutes.

## 8. Self-grade

High on D1, D2 and D6: a record, a purpose and a deposit kind through the
sitting that exists, with the case card's shape reused whole. High on D3:
a fresh session is one server decision the code already makes. Medium on
D5: the first read that runs git for the Partner, and the head's
`Reviewed:` line is derived from two loose links until the commit-goal
binding lands; the record says what it found and the human can correct
it. Medium on D7: a large library behind a CSP nonce is the one unknown,
named as the builder's first check. Weakest: without the desk, the code
is in the transcript rather than in a file view, and a long review will
want the desk; that is the first line of the later list, on purpose.
