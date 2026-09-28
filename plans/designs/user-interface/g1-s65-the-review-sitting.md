# g1-s65: the review room — a goal on the table before it lands

- Kind: design
- Id: 01M3J3V8CBYBGP36BZYS8JM98F
- Status: draft
- Goals: browser-interface

Revision 2, 2026-09-28. Wido, 2026-09-27: "support for human review of a
goal (explanation of design, decisions, code, coding decisions, showing
mermaid diagrams explain, runtime behaviour etc) in the form of a
sitting, in dialogue with the project partner OR a different agent
equipped for specifically this task ... like discussing a piece of work
that was built with a fellow engineer and a whiteboard together in a
room with the laptop and the code at hand. UX IS KEY." Then, 2026-09-28:
"this needs to happen BEFORE it becomes live on main. We should ideally
be able to run it locally, so we can inspect behaviour"; and: "How can
the human in the most optimal way be supported by the UI: understanding,
inspecting, commenting, creating follow up goals that need to complete,
taking the goal back into development ... without losing focus, ideally
being able to 'step out of the room and come back later'. The virtual
whiteboard with diagrams, remarks ... I need EXCELLENT UX on this." He
agreed to the room and its four slices as proposed. Revision 1 was the
same record for done goals, with the code in the transcript; it is
superseded by this page. Author Fable. Every cite re-read at `a9ca1a278`.

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
   anyone can be held to." The machinery in the room judges nothing; a
   fresh mind is the only fresh perspective. Chapter 13's reviewer "may
   demand more evidence, narrow or stop exposure, and authorize
   acceptance within scope or refuse it; the custodian still performs
   the acceptance". Chapter 6 names when human review is owed. Chapter
   14: "the machinery prepares the environment and carries out the
   experiment the engineer chose". Proportion holds: "a mechanical
   repair with a clear defect needs no sitting".
2. **Where a goal waits, and where its work is.** A claimed goal whose
   work is built and waiting to land carries a Landing record (`goal
   land-ready`, `work land --queue-only`; `internal/goal/file.go:80-82,
   99-104`), and the board reads it into the Review lane
   (`internal/backlog/project.go:215`), which refuses every drop:
   "Review is read from a claim that has built and is waiting to land;
   nothing here can record that" (`backlog/moves.ts:74`). Nothing in
   that lane waits for a person: `work land G` needs reads, proof and
   the approval and lands (`cmd/metasystem/intent_delivery.go:139-152`).
   The built work is the branch `goal/<id>` at origin
   (`cmd/metasystem/goal_branch.go:293, 495`); a done goal carries no
   landing commit, the `Goal-Item` trailer is the link
   (`internal/landing/held.go:313`). What a human can record about work
   today: approve before, done after, accept-risk on a critic's finding,
   rejection as prose in Next step. No record says a human examined a
   candidate and what they made of it.
3. **The sitting as built** (g1-s53, g1-s55): a record with a
   conversation attached, four sections written by the human's press
   through one serialized recorder (`partner/sitting.ts`,
   `partner/recording.ts`); the `deposit` tool offers facts, proposals,
   decisions, open questions, cases and the outcome and writes nothing;
   the case card carries Decide, Leave open and Dismiss
   (`partner/Deposit.tsx`); End drafts the Outcome; Project → Sittings
   lists records by their deposit marks. `admitsPurpose` refuses review
   ("review and learning sittings are not in this build",
   `internal/ui/partner/service.go`) and `admitsSubject` refuses any
   record but an intent or a design. One conversation per human per
   checkout, outside the checkout (`internal/ui/partner/conversation.go:
   20-50`); whether the provider session is fresh is the server's one
   decision (g1-s55 D3). The Partner is an ACP runtime with Read, Glob
   and Grep inside the checkout and eleven reads plus three offers
   (`internal/ui/uitools/uitools.go:70-83`); no git.
4. **What the interface can show.** The document reader serves one
   checkout file by its relative id and only a `.md` one
   (`internal/ui/project/document.go:118-155, 339`), as a typed tree; a
   mermaid fence is a code block with its caption ("no diagram is drawn
   in this build", `internal/ui/markdown/markdown_test.go:149-163`;
   `project/Markdown.tsx:81-128`). No source view, no diff, no evidence
   view. Selected prose anywhere the interface renders it offers Ask,
   which attaches the passage with its document and revision for one
   question (`partner/AskSelection.tsx`, g1-s29). Stickies are private
   notes bound to a goal or a document, shown on that subject's page
   (`stickies/stickies.ts:52-66`, g1-s50). The New goal sheet opens with
   a prefilled intent (`backlog/OpenSheet.tsx:105`). The frontend has no
   lazy chunk and serves under a per-request CSP nonce
   (`httpd/httpd.go:342-354`); the cut guard forbids every network call
   outside the listed sites.
5. **The launch contract** (`plans/designs/app-launch-contract.md`, goal
   `app-launch-contract`, priority 1): `metasystem app start --goal G`
   runs the candidate from `goal/<id>` in a worktree on its own port
   with a truthful status and a proven stop. This room consumes it in
   slice C and does not wait for it before that.

## 2. What you want in the room

- **R1. Understand.** What was asked, what was built, why, how it was
  examined and proven, and how it behaves, each with its anchor.
- **R2. Inspect.** The code at hand: the lines the conversation is
  about, large, with the changed lines marked, and the change as a
  whole when you want it.
- **R3. Comment.** Jot a remark on the exact spot, keep it private until
  you decide it belongs on the record.
- **R4. Follow up, or take it back.** Every finding answered by you, in
  your words, with its consequence stated: a follow-up goal, a return to
  the builder, an accepted risk, or an open question.
- **R5. Not lose focus.** One room, the colleague beside the thing,
  nothing else on screen pulling at you.
- **R6. Step out and come back.** Leave at any moment; return to the
  same desk, the same board, the same conversation; be told what moved.
- **R7. A whiteboard.** Drawings and remarks that stay, pinned to what
  they are about.
- **R8. Never a nod.** What leaves the room is a record of what you
  examined and what you answered, and a verdict the landing reads.
- **R9. A colleague who was not in the room** that shaped the design,
  who reads the record and the code and never says whether to accept.

## 3. The room

One screen, two panes, and a wall you flip to. The rail is hidden while
you are in the room; the bell keeps its count and nothing more.

```
┌ Reviewing g1-s64 · at tip 84f8acfbf · 3 findings, 1 unanswered · [Board] [Step out] ┐
│ DESK ─────────────────────────────────── │ THE CONVERSATION ───────────────────── │
│ ‹ owner.go:41-88 › changes › design D3 › │ Partner: The lock covers publish and    │
│                                          │ reconcile because ... (owner.go:60) ●   │
│  41  type owner struct {                 │                                         │
│  ..  ▌ changed lines marked              │ You: what if the press dies here?       │
│  60  func (o *owner) begin(...)          │                                         │
│      ▲ selected → [Ask] [Remark] [Finding]│ Partner: nothing recorded covers it ... │
│                                          │ ┌ A finding ──────────────────────────┐ │
│                                          │ │ [Fix in this goal] [Follow-up goal] │ │
│                                          │ │ [Accept, with reason] [Leave open]  │ │
│                                          │ └─────────────────────────────────────┘ │
│                                          │ [ Ask ... ]                             │
└──────────────────────────────────────────┴─────────────────────────────────────────┘
```

**The desk** is the laptop. It shows one thing at a time, large: a
source file of the candidate's tree at a line range with the changed
lines marked; the change index, files with counts, each opening its
diff; a design or intent section; a screenshot from the evidence; a
kept drawing. A strip above it lists what has been on the desk, newest
first, and pressing one brings it back. Every anchor in the conversation
puts its subject on the desk. During a walk the Partner puts things on
the desk as it explains them, the colleague pointing at the screen, and
you can stop it with one press.

**The conversation** stays beside the desk, always. One composer, at
the bottom, reached with one key. It opens with the narrator's report in
five parts, Asked, Built, Examined, Proven, Behaves, every claim with its
anchor, and five presses under it, one per part, each a walk.

**The board** is the whiteboard. The desk pane flips to it and back with
one press. It holds the four piles, Findings with their answers, Facts,
Decisions and Open questions, and beside them your remarks and the
drawings you kept. Every item on the board is pinned to what it is
about; pressing it puts that back on the desk. Never more than two
things on screen: the colleague, and either the thing or what you have
made of it.

**Inspecting is selection.** Select lines on the desk, a hunk, or a
sentence of the design, and three presses appear: Ask sends the passage
to the Partner; Remark pins a private note to that exact spot; Finding
opens a finding card with the anchor filled.

**A finding has four answers**, each stating its consequence in one
line. Fix in this goal marks it for the builder: the candidate will
return to construction and be examined again before it comes back.
Follow-up goal opens a new goal with the finding as its intent and lets
this one land. Accept records the risk with your reason. Leave open
keeps the question with its consequence. A finding is recorded the
moment you answer it, with the answer on its line.

**Stepping out** is the ordinary exit, not an end. The room keeps the
desk and its history, the board, the conversation and, in slice C, the
running candidate. The goal's card in the Review lane becomes the door:
"In review · you stepped out 2h ago · 3 findings, 1 unanswered", and
pressing it reopens the room where you were. If the branch tip moved
while you were out, the room says so on return and offers one press,
Show what changed, which puts the diff between the two tips on the desk;
findings whose anchors sit in files that changed are marked "may have
moved". A push never throws a review away.

**Ending** shows the verdict the landing will read, with its
consequence: Clear to land, which refuses while a finding is unanswered
and lists them; Send back, which carries every finding marked fix to the
builder as its correction brief; End without a verdict, said plainly.
Each records the Outcome first, and the Outcome names what you examined,
so a nod cannot pass as a review.

## 4. Decisions

Each decision names the slice that builds it: A, B, C or D (§5).

- D1 (A). **A review is a record of its own**, kind `review`, in a home
  beside the designs, created by the server at Start. Its head names
  `Goals:` the goal; `Reviewed:` the tip of `goal/<id>` for a goal
  waiting to land, or the `Goal-Item` commits for a done goal, or "none
  found; write them here"; `Evidence:` the path a design's Evidence line
  names, where one does. Sections: Facts, Findings, Decisions, Open
  questions, Drawings, Outcome. The existing projection lists it on the
  goal's page and under Project by kind. The transcript stays the private
  sitting store.
- D2 (A). **Review it** stands on the Review lane's card and on the goal
  page of a goal that waits to land, and on a done goal's page. It opens
  the room at `/review/<record>`, a route of its own that hides the rail
  and carries Step out; the drawer is not shown inside the room, because
  the room's own pane is the sitting's conversation (D16). Purpose
  `review`; the sitting's subject is the review record, so every rule of
  the sitting holds unchanged.
- D16 (A). **A sitting is a conversation.** Wido, 2026-09-28: "fold it
  into slice A as 'a sitting is a conversation'", on the question
  whether a human can start a review, leave the room, start another,
  and come back to finish the first. Today the Partner keeps one
  conversation per human per checkout and the sitting is a mark on it
  (`conversation.go`, "one per human per checkout"; the routes name no
  sitting by id "because there is one sitting on one conversation",
  `httpd/partner.go`), so a second sitting would replace the first. Now
  a sitting has a conversation of its own, in the same private store
  under the same owner, keyed by the human and the sitting's record,
  beside the ordinary conversation the drawer shows. The room shows its
  sitting's conversation; two reviews have two transcripts, two desks
  and two sets of drafts, and neither reaches the other. One Partner
  session is live at a time, the one whose conversation is on screen:
  entering a room ends the drawer's session and opens the sitting's,
  fresh, and the service replays that conversation's own retained
  history into it, which is the fresh-session path that exists
  (`service.go`, the `fresh` branch), applied to the sitting's
  conversation; Step out ends the session and keeps the conversation;
  the next turn in the drawer reopens the ordinary conversation the same
  way. The one running turn stays one: a turn in flight when a room is
  entered or left ends at its next boundary as attendance already
  provides. The Sittings tab lists every standing sitting with its door
  line, and the door reopens that sitting's conversation, desk and
  drafts. The mark that D9 describes lives on the sitting's
  conversation. A sitting that shapes intent or a design gets the same
  conversation of its own, because one rule for sittings is cheaper
  than two, and it costs nothing more.
- D3 (A). **The reviewer's Partner is fresh.** The review's turns run in
  a provider session started at Start with the boot context and the
  review brief, and none of the conversation's earlier turns. That
  covers both paths the service has today: the history it restores
  into a fresh session (`internal/ui/partner/service.go`, the `fresh`
  branch that reads `conversation.History()`) and the proposals block
  it composes from the whole transcript (`proposalsBlock`), and it holds
  on recovery too: a review session's opening carries the boot context,
  the review brief and the review record, and nothing of the ordinary
  conversation (Astra, round 1). With D16 the review has a conversation
  of its own, and what a fresh session replays on return is that
  conversation's history alone, the review's own words, never the
  drawer's and never a design room's. The
  Partner's instructions gain the review rules: bring what the record
  holds in the five parts; anchor every claim; name what the examination
  did not try, what the tests assume and what is not recorded; offer the
  cases at the edge as findings; put on the desk what you are
  explaining; draw when words would be longer; never say whether to
  accept. Not a second agent: one relationship, one store, one help
  register. Freshness is what "a different agent" buys, and a new
  session buys it.
- D4 (A). **The desk reads the candidate's tree, not the seat's
  checkout.** Three reads over the review record's `Reviewed:` line:
  a source file at a line range, bounded, from the reviewed tree with
  the lines the change touched marked; the change index, files with
  counts; one file's diff as hunks. What is compared depends on the
  subject. For a goal waiting to land: the merge base of main and the
  tip against the tip, and the tip's tree for source reads. For a done
  goal: each `Goal-Item` commit against its first parent, in order,
  combined, and the last commit's tree for source reads, because those
  commits are already on main and a merge base would compare a commit
  with itself and show nothing (Astra S65-02). The same owner serves
  the Partner's `changes` operation, so the colleague and the human read
  one tree. A record section on the desk is the existing document
  reader.
- D5 (A). **Anchors put things on the desk.** An anchor chip in the
  conversation names a file and range, a change, or a record section,
  and pressing it opens that on the desk; the strip keeps every desk
  item of the sitting, newest first. The Partner's walk may put an item
  on the desk through a display suggestion on the stream, never a
  navigation elsewhere, and the human's Stop press ends the walk's
  presenting for that turn.
- D6 (A). **The five walks** are fixed requests the interface submits
  with its own provenance, as the opening and closing turns are.
- D7 (A). **Selection on the desk** offers Ask and Finding; Remark comes
  with the board (B). Ask reuses the passage chip with the desk item as
  its document and the tip as its revision. Finding opens a finding card
  with the anchor filled and the text empty for the human's words.
- D8 (A). **The finding card, Record it, and its four answers.**
  `deposit` gains the kind `finding` with text, anchor and consequence.
  The card is recorded like every other deposit: Record it is the
  human's press, and it writes the finding into the Findings section
  with `Anchor:` and `Answer: unanswered` as its clause lines. A card
  never recorded is a card, not a finding: it is not counted and it
  binds nobody, which is the paper's rule that only records rule. The
  four answers are presses on a recorded finding, on its card and on
  the board, and each rewrites that entry's `Answer:` line through the
  recorder, by the entry's deposit mark, as a second composition beside
  `appended`: `fix — waits for Send back`; `follow-up — goal G2`,
  written only after the New goal sheet opened the goal with the
  finding as its prefilled intent; `accepted — <reason>`, from the
  Decide sheet with the reason required; `left open`. Dismiss folds an
  unrecorded card. The door's counts and End's refusal read the record,
  so an unanswered finding is a fact of the record and not of one
  browser (Astra S65-01). An entry's clause block may carry `Anchor:`
  and `Answer:` lines; the piles are a property of the record's kind. A
  finding the human states in words is offered back by the Partner as a
  card with the anchor it found.
- D9 (A). **Step out and come back.** The sitting mark on the sitting's
  own conversation (D16) carries the room's working state: the desk strip and the
  current item, the board or desk face, and the human's unfinished
  words, that is every unrecorded card's edited text and clause and the
  fields of an open Decide sheet, keyed by deposit id (Astra S65-03). It
  is written on every change, a second after the last keystroke, and
  read on return, in the private store; a draft is cleared when its card
  is recorded or dismissed, and drafts never touch the record. The
  Review lane card and the goal page show the door line from the review
  record's counts, findings and those still `unanswered`, and the mark's
  time. On return, the room compares the record's
  `Reviewed:` tip with the branch tip; a moved tip shows the banner,
  Show what changed puts the diff of the two tips on the desk, and
  findings anchored in changed files carry the "may have moved" mark
  until the human re-anchors or answers them again. The record's
  `Reviewed:` line is updated only by the human's press "Review the new
  tip", which appends the old tip to the head as `Previously:`.
- D10 (A). **End with a verdict line, no act yet.** The End sheet offers
  Clear to land, Send back and End without a verdict; each drafts and
  records the Outcome with `Verdict:` as its first line, and the sheet
  refuses Clear to land while the record carries a finding whose answer
  is `unanswered`, and lists them. In
  A the verdict is recorded intent only: nothing lands or returns. The
  nod line shows when the piles are empty.
- D11 (B). **The board.** The desk pane's second face: the four piles,
  each entry pinned to its anchor and pressing it puts the anchor on the
  desk; remarks, private stickies with a new subject kind, a file and
  range or a record section at a tip, shown on the board and on the
  desk item they belong to, each with Record as a fact and Make a
  finding; drawings kept from the conversation under the record's
  Drawings section, appended, each with the question that produced it.
- D12 (B). **Drawings.** A mermaid fence renders in the conversation, on
  the desk and on the board, one lazily loaded chunk, source by a press
  and shown instead when the chunk or the parse fails; the CSP is proven
  first (a nonce for the styles the library writes into its SVG, or a
  mode that writes none), and if it cannot be, this decision waits and
  says so. Keep it pins a drawing to the board (D11).
- D13 (B). **The Partner presents evidence.** Screenshots and reports
  under the record's `Evidence:` path are desk items through an
  `evidence` read bounded to that path; the Behaves walk shows them.
- D14 (C). **The verdict acts.** Send back performs `work revise G` from
  the browser as the twelfth act, with the correction brief composed
  from the findings marked fix; the goal leaves the Review lane. Clear
  to land records the verdict the landing reads (D15) and, until D15
  lands, is the human's recorded word only. The candidate runs from the
  room: a pill in the header reads `app status --goal G` and offers Run
  and Stop through `app start|stop --goal G` as browser acts; the
  Behaves walk names the address; the Partner may propose the run.
- D15 (D). **The landing gate, on R-132-ui.** Wido, 2026-09-28: "goals
  on the validation lane with a risk tier below the setting get
  automatically landed after a <setting> time. higher level tier needs a
  human to either do a sitting, or decide to not do a sitting and then
  move the goal into landing." Two settings in `metasystem.conf`, shown
  on the Settings page with their source: `landing.review.human-from-tier`
  (the first tier that waits for a person) and `landing.review.auto-after`
  (the grace time). A goal in the Review lane below the tier lands by
  itself when the grace time has passed since its Landing record's `At`
  with no human act on it: the seat holding the claim lands it through
  the ordinary `work land`, and the history line names the setting. A
  goal at or above the tier waits for one of two human acts: a review
  sitting whose Outcome records `Verdict: clear to land` against the
  branch tip, or **Land without a sitting**, the Decide sheet with a
  required reason, recorded as a decision on the goal's history by the
  signed-in human and bound to the tip. `work land G` refuses a goal at
  or above the tier, naming the missing fact, unless one of the two is
  recorded against the current tip; a moved tip needs the act again. A
  standing review sitting holds the landing, below and above the tier
  alike, until it ends: Clear to land lands, Send back returns, End
  without a verdict releases the clock. Silence is never a decision
  above the tier. On the card in the Review lane: below the tier, "lands
  by itself in 3h 12m" beside Review it; at or above it, "waits for your
  review" with Review it and Land without a sitting, and a row in the
  Decisions inbox, because landing waits on a person. The clock, the
  due landing and the refusal are engine work; the card's words, the
  presses and the inbox row are the interface's. D lands last, after a
  real review record has been written at least once.

## 5. The four slices

**A, the room (step 1):** D1 to D10 and D16. What exists after it:
Review it on a goal waiting to land; the room with the desk, the
sitting's own conversation, the walks and the counts; the candidate's
code and diff on the desk; selection to Ask and Finding; findings
answered four ways and recorded; Step out and the door, and a second
room while the first waits; the moved-tip banner; End with the verdict
line.
The board is the existing table behind the Board press; drawings are
code blocks; the verdict changes nothing yet.

**B, the whiteboard:** D11 to D13. Remarks, kept drawings, mermaid, the
evidence on the desk, the board as the desk's second face.

**C, the verdicts and the candidate:** D14, on the app launch contract.

**D, the gate:** D15, engine work, with Wido's ruling on which goals
require a human review.

Each slice is usable on its own and A already lets a goal be reviewed
with the code on the desk.

## 6. Payload and routes, slice A

`POST /api/partner/sitting` takes purpose `review` with subject
`{kind: goal, id}`; the server resolves the reviewed tree (the branch
tip, or the trailer commits for a done goal), creates the review record
with its head, opens a conversation for the sitting in the private
store under the key of the human and the record (`OpenConversation`
gains the sitting's key beside the human's), and marks it. The Partner
routes, the read, turns, stop, seeing, the sitting's end and close and
the proposal outcome, take an optional `conversation` naming a
sitting's record; absent, they mean the ordinary conversation as
today. The event stream carries the conversation on each partner
event, and a page shows only its own. The service's one live session
follows the conversation last asked: a turn on another conversation
closes the live session and opens that conversation's, fresh, with its
own history replayed. `GET /api/project` lists every standing sitting. The record creator gains the
kind, its template and its home; the resolver its kind. Three reads:
`GET /api/review/<record>/source?path=&from=&to=` (a text file at the
reviewed tree, at most four hundred lines per call, with the touched
lines marked), `GET /api/review/<record>/changes` (files with counts,
merge base to tip for a waiting goal, parent to commit per trailer
commit for a done goal), `GET /api/review/<record>/changes?path=`
(hunks, bounded); the Partner's `changes` operation reads the same
owner. `deposit` gains `finding`; `entriesIn` reads `Answer:` beside
`Anchor:`; the recorder gains `answered`, which rewrites one entry's
clause block by its deposit mark under the same reading rule. The
sitting mark gains `desk`, `face` and `drafts`, written through the
existing sitting route. The room route `/review/<record>`; the
Review lane card's door line from the backlog payload's records naming
the goal; the five walks as fixed requests; the New goal sheet's
prefill; the End sheet's three ways. The cut guard's call sites gain the
three reads.

## 7. Not here, later

Several humans in one review; the learning sitting with the diagnosis
withheld; a review of a whole design's arc across goals; the four risk
questions proposing which goals deserve a look; the "rulings always
match the preparation" evidence; running an adopted application whose
contract is not yet written; phone width beyond the conversation and
the board stacked.

## 8. Verification and box, slice A

Go: `admitsPurpose` admits review and `admitsSubject` the review
record; the store opens a sitting's conversation by its own key beside
the human's, the grants test still reaches neither, and two sittings'
conversations never share a file; a turn on a second conversation
closes the live session and opens the second's with only its own
history replayed (proven by what the fake runtime received); the
kind's template and home; the head's `Reviewed:` line from
the branch tip and from trailer commits, and the "none found" line; the
three reads' bounds and refusals (a path outside the tree, a range past
the file, a binary file named as such); the fresh provider session
carries no earlier turn; the walks' provenance; `finding`'s bounds.
Frontend: Review it on the Review lane card, the goal page of a waiting
goal and a done goal's page, and nowhere else; the room hides the rail
and Step out returns to the board; the desk shows a file at a range
with touched lines marked, the change index, a file's diff and a record
section; the strip restores an item; an anchor chip opens its subject;
selection offers Ask and Finding and the finding card carries the
anchor; Record it writes the finding `unanswered`, and the four answers
rewrite that one entry's `Answer:` line by its mark, follow-up only
after the goal opened, accept refusing without a reason, an unrecorded
card counted nowhere; the door line reads the record; step out and
return restore desk, face, item and the unfinished words of a card and
an open Decide sheet; a moved tip shows the banner and Show what
changed; a done goal's change index shows its commits' own changes and
never an empty diff; End refuses Clear to land with a recorded
unanswered finding and lists it; the nod line with empty piles; the
guards stay green; two rooms: start a review, step out, start a second
on another goal, return to the first through its door and find its
transcript, desk and drafts, with the drawer's ordinary conversation
untouched throughout. Walkthrough: the fake Partner answers a canned
opening turn, one walk that presents a file, one finding, and a second
room; screenshots
at 1280 and 400: the room, the desk with a file and with the change
index, a finding card and its four answers, the door line on the
Review lane card, the moved-tip banner, the End sheet. Box: two build
lanes (Claude on Opus): the desk's reads and pane, and the room, the
sitting and the finding; one code read each by the code-review lane the
roster names, with one fix round under R-124, after Astra's read of
this page; two attempts, 360 to 540 job-minutes.

## 9. Self-grade

High on D1, D3, D6, D8 and D10: a record, a fresh session, fixed
requests and a card shape the sitting already has. High on D2 and D9:
a route, a hidden rail and a mark on the conversation are small.
Medium on D16: the store already keys conversations and the service
already holds a map of them and replays a conversation's history into
a fresh session, so a sitting's conversation is one more key and one
more argument on the routes; the delicate part is the one live session
following the conversation on screen, which attendance and the one
running turn already bound. Medium
on D4 and D5: the first reads over a git tree that is not the checkout,
and the first display suggestion from the Partner; both bounded and both
served by one owner. Medium on the room's width at 1280: two panes of
code and conversation is tight, and the desk takes the larger share by
default with the divider resizable as the drawer's is. Weakest: A
records a verdict that changes nothing until C and D, which is honest
and is said on the End sheet; and remarks wait for B, so in A a jotting
is a finding left open or a fact. That is the cut Wido agreed to.

## Dispositions (Astra round 1, 2026-09-28, under R-121 and R-124)

Read of revision 2 at `2bfac8174` (code at `647234149`), verbatim in
`g1-s65-astra-critique.md`. Three material findings, all folded; one
deferred. Every cited line was re-read at whole-function depth before
folding.

| id | finding | fold |
|---|---|---|
| S65-01 | a finding written only when answered leaves the door's "unanswered" count and End's refusal without a source in the record; the table and the sittings list read the record only (`store.tsx`, `sittings.go`) | D8: Record it writes the finding with `Answer: unanswered`; the four answers rewrite that entry's line through a second recorder composition; an unrecorded card is a card, counted nowhere; D9 and D10 read the record |
| S65-02 | for a done goal the merge base of main and a landed commit is that commit, so the change index is empty for changed work | D4 and §6: a done goal's changes are each `Goal-Item` commit against its first parent, and source reads use the last commit's tree; the merge-base comparison stays for a waiting goal |
| S65-03 | `desk` and `face` do not keep an unrecorded card's edited words or an open Decide sheet's reason; leave and return loses them (`store.tsx`, `Deposit.tsx`, `conversation.go`) | D9: the mark carries `drafts`, every unrecorded card's text and clause and the open sheet's fields, written a second after the last keystroke, cleared on record or dismiss, never written to the record |
| S65-04 | §8 named Codex on Sol for the code read, which `development/project-rules-local.md` forbids | deferred as non-material: §8 now names the code-review lane the roster names; the roster question is Wido's, since the UI lane's reads to date were Sol's with his knowledge |

Astra also verified, and D3 now says, that a fresh provider session
alone would not satisfy D3: the service restores the conversation's
history into a fresh session and composes a proposals block from the
whole transcript, and both must be excluded in a review session,
including on recovery.

**Round 2, the declared failsafe (2026-09-28, at `9eb7cf433`):** S65-01,
S65-02 and S65-03 confirmed answered; no new finding; "VERDICT: 0
material findings". The loop is closed at round 2 with zero material
findings. One mechanical check for the build, not a mechanism: the
drafts' one-second debounce must flush on Step out and on unload, so
the last keystroke before leaving is not the one that is lost.

**Fold after the close, on Wido's word (2026-09-28):** D16, a sitting
is a conversation, so a human can leave one room, open another and
come back to the first. It changes slice A's shape (a store key, a
route argument, the live session following the conversation), so it
gets one scoped confirmation read from Astra on D16 alone, not a third
round.
