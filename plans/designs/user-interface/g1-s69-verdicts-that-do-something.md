# g1-s69: verdicts that do something, and the running candidate — the review room's slice C

- Kind: design
- Id: 01M3MP8CZYPATTR0382JS6HMFF
- Status: draft
- Goals: browser-interface

Wido, 2026-09-28: "ok, build all". This page is the step-1 cut of
slice C of the review room (g1-s65 D14): the verdicts act, and the
candidate runs from the room. Author Fable. Every cite re-read at
`12611b98c`.

## 1. What exists and binds

1. **The room as landed** (g1-s65 slice A): End offers three verdicts,
   Clear to land, Send back and No verdict; the Partner drafts the
   Outcome with `Verdict:` as its first line, the human records it, and
   the sitting ends (`internal/ui/partner/review.go:79-95`). The verdict
   changes nothing: the goal stays in the Review lane, the record stays
   a file in this checkout, and the landing seat never hears of it.
2. **Where the review record lives.** The room writes the record into
   the checkout (`metasystem/plans/reviews/<name>.md`) through the
   record writer, which does not commit (`internal/ui/project/write.go`,
   `edit.go`: no git call). Tonight's real review of the launch contract
   is an untracked file. The goal whose work is reviewed is claimed by
   another seat, on another checkout, and `work land` runs there
   (`internal/landing/advance.go:31`): a verdict that lives only in this
   checkout's working tree cannot be read where the landing happens.
3. **The ledger crosses seats; files do not.** A goal verb records a
   history line in the goal file and lands it on main through the
   ledger's own commit (tonight: `d1a5757ee goal open ask-what-happened`
   arrived on main from a verb, not from a push of this branch). What
   a goal's history says, every seat reads after its sync.
4. **The correction verb.** `work revise G --brief FILE [--after N]`
   makes one new attempt from a brief, kept before anything starts so a
   repeat rejoins it, and the result must be reviewed again (its help).
   The launch act is the precedent for a browser act that starts a
   lane under the human's sign-in (`internal/ui/httpd/launch.go:27-30`,
   `mayAct` at `acts.go:498`).
5. **The candidate.** `metasystem app start|stop|status --goal G` runs
   the goal's branch from its own worktree on its own port beside the
   standing run, waits for readiness, and reports liveness and readiness
   apart (the launch contract, Built). The Behaves walk is the walk that
   asks how the work behaves (`partner/review.go:48`).

## 2. What you want when the review ends

- **V1. The verdict is a fact of the goal.** When I say clear to land,
  the goal knows it, wherever it is claimed, and the landing that
  follows can say "reviewed by Wido at this tip".
- **V2. Send back is a brief, not a shrug.** The findings I marked fix
  become the correction the builder gets, without me composing a file.
- **V3. The record is kept.** The review record is committed like every
  record, so the next reader and the landing find it.
- **V4. Run it.** From the room, start the candidate and click through
  it; stop it when I am done; the Partner can tell me where it runs.
- **V5. Nothing lands on my nod.** Clear to land records; the landing
  itself is the gate's business (slice D), and until D lands a verdict
  is my recorded word and nothing moves by itself.

## 3. The moment

You press End and choose Clear to land; the Partner drafts the Outcome;
you press Record it. The room says "Recorded. The goal now carries your
verdict: clear to land at 9c1f0a2, reviewed by Wido." The card in the
Review lane reads "reviewed by Wido · clear to land". The record shows
up in the project's Records with a commit. Or you choose Send back: the
sheet lists the findings you answered fix, three of them, and says
"these become the correction brief"; you press Send back; the room
says "Sent back with your three findings; the goal has left Review and
the seat that holds it revises." Later the card reads "attempt 3
started from your brief". Before either, you pressed Run in the header
pill: "the candidate is starting on 7981… ready, running the reviewed
tip 9c1f0a2", you opened it in a new tab and walked the screens the
Behaves walk named; you pressed Stop when done. Had the branch moved
under you, the pill would have said "running 4d2e7b1, not the reviewed
9c1f0a2", and the Behaves walk would have said so first.

## 4. Decisions

- D1. **The verdict is recorded on the goal, through the ledger, bound
  to the record.** A new human-only goal act, `goal review G --record
  PATH --verdict clear-to-land|send-back`, resolves the record in its
  home, requires that its `Goals:` line names G and that its Outcome's
  first line is `Verdict:` with that verdict, takes the tip from the
  record's own `Reviewed:` head (`internal/ui/review/review.go:151`),
  never from an argument, and writes one history line on the goal
  (`reviewed verdict=… tip=… record=… by=…`, by being the signed-in
  human who performs the act, which the line does not claim to be the
  record's author) and lands it through the ledger's own commit as
  every goal verb does, with the review record published beside it in
  the same landing. The publication is create-only: a record already on
  the canonical tree at that path with the same bytes is a replay and
  the act confirms; different bytes at that path, another human's
  review, are refused, inside the transaction's rebuild on a moved tip
  (`internal/goal/txn.go:310`, `:869`), so no recorded words are ever
  overwritten. The Outcome is bound to the tip it was drafted for: the
  service stamps the record's reviewed tip on the closing deposit as it
  stamps the verdict (`partner/service.go:1290`), the recorder writes it
  into the Outcome as `Reviewed at:` under `Verdict:`, Record it refuses
  when the record's head names another tip ("the branch was retipped
  since this Outcome was drafted; press End again"), Review the new tip
  while an unrecorded Outcome card stands asks for End again and keeps
  the edited words in the draft (today it rewrites the head without
  touching the card, `partner/store.tsx:1855`, `review/room.ts:603`),
  and the act refuses an Outcome whose `Reviewed at:` is not the head's
  `Reviewed:`. Clear to land in the room performs it after Record it,
  under the human's sign-in through the act layer; No verdict performs
  nothing. The line is what slice D's gate reads; until D lands, the
  line is the human's recorded word and the goal stays where it is.
- D2. **Send back is the human's word on the ledger; the holder
  revises.** `work revise` cannot run from this seat: it reads the
  goal's named work in this checkout's worktrees and requires the claim
  holder's lease and lineage before a model launches
  (`cmd/metasystem/intent_selection.go:24`, `intent_work.go:350`,
  `goal_branch.go:767-787`), and it does not withdraw a goal from
  landing (`internal/backlog/project.go:208` reads the Landing record;
  `internal/launch/unit_run.go:298` only marks the run running). So the
  End sheet's Send back composes the correction brief from the findings
  whose answer is fix (each finding's words and its anchor, in the
  order recorded, under one heading naming the record and the tip),
  shows it as a card the human can read and edit, and performs the D1
  act with `--verdict send-back`, whose history line carries the brief
  as a record beside the review record (published the same create-only
  way). The Landing record is not touched: a landing claim stands
  outside the machine's one-claim quota (`goal/verbs.go:2149`,
  `goal/validate.go:436`), so a holder that lawfully claimed a second
  goal would be left with an invalid ledger if the Landing were cleared
  (`validate.go:475`, checked before publication, `txn.go:801`). The
  goal leaves the Review lane by reading instead: the board's lane rule
  treats a claimed goal whose history carries a send-back line newer
  than its Landing record's `At` as sent back, a phase of In progress
  (`backlog/project.go:215`), until a later land-ready writes a newer
  Landing; every seat reads the same history, and slice D's gate reads
  a send-back newer than the Landing as no permission to land. The
  holding seat, on its next activity over its claims (as slice D has
  it), finds the send-back line and runs `work revise G --brief FILE`
  on the published brief. When the goal has one eligible work unit the
  verb selects it; when it has several the verb refuses and names them
  (`cmd/metasystem/intent_selection.go:375-403`, the candidates in its
  data), and the holder records that on the history (`send-back
  needs-work candidates=A,B`), the card reads "the holder needs to know
  which work: A or B" with a press per name that performs the act again
  with `--work NAME`, the line carries `work=NAME`, and the holder runs
  `work revise G --work NAME --brief FILE`; the verb's own rejoin rule
  holds within the selected run (`launch/unit_revise.go:117`), and the
  holder records the attempt it started, so a later pass finds the
  attempt line and revises nothing twice. The room says "sent back; the
  holder revises" and the card in the lane says "sent back by Wido ·
  awaiting the holder" until the attempt's line arrives. A Send back
  with no finding marked fix is refused in words, and one pressed while
  the room's closing turn is unsettled waits for it, as End does today.
- D3. **The candidate runs from the room.** The room's header carries a
  pill that reads `app status --goal G` (alive, ready, address, since,
  and the running commit the status already returns,
  `cmd/metasystem/intent_app.go:199-203`) and offers Run and Stop,
  performed through the act layer as `app start --goal G` and `app stop
  --goal G`, two new browser acts under the human's session, on this
  host; the address opens in a new tab. `--goal G` follows the branch
  (`app.go:205`, and a moved ref replaces the run, `intent_app.go:289`)
  while the room reviews the tip its record names until Review the new
  tip is pressed, so the pill compares the running commit with the
  reviewed tip: "running the reviewed tip 9c1f0a2" or, in the moved
  case, "running 4d2e7b1, not the reviewed 9c1f0a2", and the Behaves
  walk's request carries both commits and says, when they differ, that
  what runs is not the version under review and must not be presented
  as its evidence. The Partner may propose Run through the proposal
  grammar. A goal whose contract has no `launch.json` shows the pill
  greyed with the reason.
- D4. **Nothing else changes.** The record model, the recorder, the End
  sheet's three choices, the Outcome's first line, slice A's tests.

## 5. Step 1, the smallest thing that works

D1 to D4 as one slice, in two lanes: the engine's `goal review` act
with its ledger line and record landing (D1), and the interface's
three acts, the correction card and the pill (D2, D3). Not in it: the
landing gate and the clock (slice D); several findings' briefs merged
across rounds; a candidate for a done goal (it is on main already; the
standing run is that).

## 6. Payload and routes

`POST /api/goals/<id>/review` `{record, verdict, brief?, work?}`
performs `goal review` under the signed-in session, as the approve
route performs approve (`httpd/acts.go`, `actRouteOf`), with the
launch route's stronger session rule (a live session proof, never boot
authority), and answers the history line; with `send-back` the brief's
text is written as a record beside the review record and both are
published create-only, and the Landing record is left as it is. The
closing deposit carries `tip` beside `verdict`, and the Outcome's
`Reviewed at:` line is the recorder's. The board's lane reading gains
the sent-back phase from the history. `POST /api/app/<goal>/start|stop`
and `GET /api/app/<goal>/status` wrap the three app forms; status
carries the running commit. The holding seat's activity over its
claims gains one step: a send-back line without an attempt line runs
`work revise` on the published brief, with `--work` when the line
names it, and records the attempt or the needs-work line. The proposal
grammar gains `app start --goal` and `goal review`, and the join test
that holds the grammar against the verb table grows to the app object.
The Behaves walk request gains the address and the two commits. The
cut guard's call sites gain the three reads.

## 7. Not here, later

The gate (D); a verdict for a design review; the correction brief
edited by the Partner; running an adopted application whose contract
is not yet written; the candidate's log on the desk.

## 8. Verification and box

Go: `goal review` writes the history line with the tip from the
record's head, and refuses a record whose `Goals:` does not name the
goal (a review of A offered for B), whose Outcome lacks the verdict, or
that is not in its home; the record lands with the ledger commit; the
same bytes at the path are a replay and different bytes are refused,
also on a moved tip inside the rebuild; send-back publishes the brief
and leaves the Landing record, so a holder with a second claim still
publishes and the ledger stays valid; the board reads a send-back
newer than the Landing as sent back and a later land-ready as Review
again; the holder runs `work revise` once on the brief and never
twice, selects the one eligible unit, and with two records the
needs-work line whose choice carries `--work`; the closing deposit
carries the tip, Record it refuses after a retip, and the act refuses
an Outcome whose `Reviewed at:` is not the head's `Reviewed:`; the app
routes refuse a goal without a contract in words and status carries
the running commit; every new act under the session rule and the
idempotency rows; the public forms test. Frontend: Clear to land after
Record it performs the act and the room says so; Send back composes
the brief from the fix findings only, shows it, refuses with none,
waits for an unsettled closing turn, and performs; the pill's states
including the moved case with both commits; the Behaves walk carrying
the address and the two commits; the cut guard. Walkthrough: a review ending
each of the three ways, a candidate started and stopped; screenshots
at 1280 and 400, light and dark. Landing checks as g1-s67 §8. Box:
Astra's critique (two rounds), two Opus 5.5 build lanes, one Sol read
with one fix round; 200 to 320 job-minutes.

## 9. Self-grade

High on D3: verbs that exist, wrapped as acts the way the launch act
is, with the running commit the status already carries. Medium on D1
and D2 since revision 2: the ledger is the one thing every seat reads,
so the verdict, the brief and the withdrawal from landing all travel
through it, and the holder's own loop does the revising; the
create-only publication of a record beside a ledger commit is the new
obligation on the ledger path, verified as feasible by Astra (`txn.go:
276-340`). Weakest: a review that ends after the branch moved records
a verdict against the old tip; the gate (D) refuses it, and until then
the history line's tip is the reader's warning.

## Dispositions (Astra round 1, 2026-09-28, under R-121 and R-124)

Read of revision 1 at `d719503f8`, verbatim in
`g1-s69-astra-critique.md`. Four material findings, all folded; every
cited line re-read at whole-function depth before folding.

| id | finding | fold |
|---|---|---|
| S69-01 | `work revise` reads this checkout's named work and requires the claim holder's lease and lineage before launching (`intent_selection.go:24`, `intent_work.go:350`, `goal_branch.go:767-787`), and does not clear the Landing record (`backlog/project.go:208`, `unit_run.go:298`): from this seat it neither reaches the holder nor moves the goal out of Review | D2: Send back is the ledger act with the brief published beside the record and the Landing record cleared in the same transaction; the holding seat's loop runs `work revise` on the brief; the room's words say who does what |
| S69-02 | the act as written checked verdict and tip only, so a review of A could be written onto B (`project/review.go:145`, `review/review.go:151-182`, `acts.go:498`) | D1: the record's `Goals:` must name G, the tip comes from the record's `Reviewed:` head, the Outcome must carry the verdict; `by` is the signed-in human and claims no authorship of the record |
| S69-03 | publishing a record beside the ledger commit replaces an existing path (`txn.go:310`, the rebuild at `:869`), and the name is chosen by local existence only (`project/review.go:114`): two checkouts' first reviews of one goal overwrite each other on main | D1: publication is create-only; identical bytes are a replay; different bytes at the path are refused inside the rebuild |
| S69-04 | `--goal G` follows the moving branch (`app.go:205`, `intent_app.go:289`) while the room reviews its recorded tip (`room.ts:603`); the pill showed no commit, so a run of B could stand as evidence for A | D3: the pill and the Behaves request carry the running commit beside the reviewed tip and name a mismatch; the walk says what runs is not the version under review |

Astra also verified, and the design leans on, that the ledger builder
can include a non-ledger record in its landing (`txn.go:276-340`,
`validate.go:596-616`), that revision freezes the brief and rejoins an
identical request (`unit_revise.go:117-215`), that the launch route's
session rule is the stronger precedent, and that V5 holds: nothing in
step 1 lands a goal on a nod.

**Round 2, the declared failsafe (2026-09-28, at `04bc1fdd5`):**
S69-02, S69-03 and S69-04 confirmed answered; S69-01 held open through
three new material findings, all folded. S69-05: clearing the Landing
record would put a holder with a lawful second claim over the
one-claim quota, since a landing claim stands outside it
(`goal/verbs.go:2149`, `validate.go:436`, `:475`, checked before
publication at `txn.go:801`); fold, D2: the Landing record is left
alone and the goal leaves Review by reading, a send-back line newer
than the Landing being the sent-back phase. S69-06: `work revise`
selects one work unit and refuses several without `--work`
(`intent_selection.go:375-403`, `:672`), and the rejoin is within a
run (`unit_revise.go:117`); fold, D2: the holder's refusal becomes a
needs-work line, the card offers the names, the act repeats with
`--work NAME`. S69-07: the closing deposit carried the verdict but not
the tip (`service.go:1290`), and Review the new tip rewrote the head
under an unrecorded Outcome (`store.tsx:1855`, `room.ts:603`, the
shape check at `:305`); fold, D1: the deposit carries the tip, the
Outcome gains `Reviewed at:`, Record it refuses after a retip, and the
act compares the two. Every cited line re-read. Closed at the failsafe
round on three folds, with one scoped confirmation read on S69-05 to
S69-07 alone.

**Confirmation read (2026-09-28, at `6fd19e052`):** S69-05, S69-06 and
S69-07 confirmed answered, no new material finding. The loop is
CLOSED.
