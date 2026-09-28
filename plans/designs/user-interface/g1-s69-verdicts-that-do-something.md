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
says "One new attempt started from your three findings; the goal has
left Review." Before either, you pressed Run in the header pill: "the
candidate is starting on 7981… ready", you opened it in a new tab and
walked the screens the Behaves walk named; you pressed Stop when done.

## 4. Decisions

- D1. **The verdict is recorded on the goal, through the ledger.** A new
  human-only goal act, `goal review G --verdict clear-to-land|send-back
  --record PATH --tip COMMIT`, writes one history line on the goal
  (`reviewed verdict=… tip=… record=… by=…`) and lands it through the
  ledger's own commit as every goal verb does, with the review record
  committed beside it in the same landing (the record's path is the
  verb's argument, and the verb refuses a record whose Outcome does not
  carry that verdict against that tip). Clear to land in the room
  performs it after Record it, under the human's sign-in through the
  act layer; No verdict performs nothing. The line is what slice D's
  gate reads; until D lands, the line is the human's recorded word and
  the goal stays where it is.
- D2. **Send back is `work revise` from the room.** The End sheet's
  Send back composes the correction brief from the findings whose
  answer is fix (each finding's words and its anchor, in the order
  recorded, under one heading naming the record and the tip), shows it
  as a card the human can read and edit before pressing Send back, and
  performs `work revise G --brief FILE` through the act layer under the
  sign-in, the way the launch act runs a verb. The verb's own rejoin
  rule holds: a repeated press reaches the same attempt. The Outcome
  records `Verdict: send back` first, and the history line of D1 says
  send-back with the attempt it started. The goal leaves the Review
  lane by the verb's own effect. A Send back with no finding marked fix
  is refused in words.
- D3. **The candidate runs from the room.** The room's header carries a
  pill that reads `app status --goal G` (alive, ready, address, since)
  and offers Run and Stop, performed through the act layer as
  `app start --goal G` and `app stop --goal G`, two new browser acts
  under the human's session, on this host; the address opens in a new
  tab. The Behaves walk's request names the running address when there
  is one, so the Partner can say what to look at. The Partner may
  propose Run through the proposal grammar. A goal whose contract has
  no `launch.json` shows the pill greyed with the reason.
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

`POST /api/goals/<id>/review` `{verdict, record, tip}` performs
`goal review` under the signed-in session, as the approve route
performs approve (`httpd/acts.go`, `actRouteOf`), and answers the
history line. `POST /api/goals/<id>/revise` `{brief}` performs `work
revise G --brief` with the brief written to a file the verb keeps; it
answers the attempt. `POST /api/app/<goal>/start|stop` and `GET
/api/app/<goal>/status` wrap the three app forms. The proposal grammar
gains `app start --goal` and `goal review`; `actsNotInTheInterface`
loses nothing, since these are new actions (the join test grows). The
Behaves walk request gains the address line. The cut guard's call
sites gain the four reads.

## 7. Not here, later

The gate (D); a verdict for a design review; the correction brief
edited by the Partner; running an adopted application whose contract
is not yet written; the candidate's log on the desk.

## 8. Verification and box

Go: `goal review` writes the history line and refuses a record whose
Outcome lacks the verdict or names another tip; the record lands with
the ledger commit; `work revise` from the act layer with a kept brief
and the rejoin on repeat; the app routes refuse a goal without a
contract in words; every new act under the sign-in refusal and the
idempotency rows; the public forms test. Frontend: Clear to land after
Record it performs the act and the room says so; Send back composes
the brief from the fix findings only, shows it, refuses with none, and
performs; the pill's three states and the two presses; the Behaves
walk carries the address; the cut guard. Walkthrough: a review ending
each of the three ways, a candidate started and stopped; screenshots
at 1280 and 400, light and dark. Landing checks as g1-s67 §8. Box:
Astra's critique (two rounds), two Opus 5.5 build lanes, one Sol read
with one fix round; 200 to 320 job-minutes.

## 9. Self-grade

High on D2 and D3: verbs that exist, wrapped as acts the way the launch
act is. Medium on D1: the ledger line is the right place for a verdict
that another seat must read, but committing the record beside the
ledger commit is a new obligation on the ledger's landing path and
Astra should attack it. Weakest: a review that ends after the branch
moved records a verdict against the old tip; the gate (D) refuses it,
and until then the history line's tip is the reader's warning.
