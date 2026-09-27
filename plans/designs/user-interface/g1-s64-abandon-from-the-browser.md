# g1-s64: abandon, from the browser and from the Partner

- Kind: design
- Id: 01M3GZ2C32STTMZB001PEVRYRN
- Status: draft
- Goals: browser-interface

Wido, 2026-09-27, on the question left open since 2026-09-26 whether
"close these goals" may mean more than Pause: "agreed on the 'Abandon'
topic; that needs to fit the verbs", and then "Finish all now". Taken as
the ruling in R-125's shape: a signed-in browser session may abandon a
goal from the interface, the session proof admitted for `goal abandon`
at the grade that verb requires of a human, recorded on the history
line as park and unpark are; and abandon joins the Partner's catalogue
as one more public goal action. Author Fable. Every cite read at
`2c87659a4`.

## What exists and binds

1. **The public form.** `metasystem goal abandon G --reason TEXT
   [--successor G2]`, audience human (cmd/metasystem/intent_planning.go:210):
   "record that a goal will never be worked, and why"; `--successor`
   names the live goal carrying the work; live dependents are waived
   with `--waive DEPENDENT=REASON` or abandoned in the same act with
   `--also DEPENDENT`, both advanced.
2. **The engine's verb.** `goal.Abandon(r, id, AbandonSpec{Because,
   Carried, Waive, Also}, proof)` (internal/goal/abandon.go:36) refuses
   before any read unless the request names a human and the proof is
   `ValidFor` the endpoint's root (abandon.go:37-42), which a browser
   session's proof is not: `Valid` and `ValidFor` are the enrolled
   terminal's, `SessionValidFor` the session's (g1-s23; R-125 admitted
   the session at park's and unpark's rows with `Session: true` on
   `humanAuthorityRow`, internal/goal/verbs.go:2690, 2825, through
   `requireHuman`). The reason must be one non-empty line (:43-45); a
   live dependent must be waived, also-abandoned or carried by a
   successor, else the verb refuses in words (abandon.go:233-254); the
   record carries `By`, `At`, `Carried`, `Because` (abandon.go:296). The
   CLI proves human authority at the terminal and builds the request
   with the verb's dependencies (cmd/metasystem/goalsync_mutations.go:2466-2476).
3. **The act layer and the routes.** Nine goal acts, each
   `Authority.<Act>` over the owner through `request()` and `settle`
   (internal/ui/act/act.go), each a route with a bounded body under
   `mayAct` (internal/ui/httpd/acts.go), each a row of the act table
   (describe.go) and a row of the Partner's catalogue (g1-s58 D1), each
   a client (src/backlog/api.ts). Park's admission under R-125 is the
   model: a flag at the verb's human rows, `recordSessionAuthority` on
   the history line, an act function, a route, a client, a catalogue
   row (g1-s46 section 5).
4. **Retire, until now.** "Retire a goal" from the Partner has meant
   Pause; the design of g1-s58 said abandon needs a ruling first.

## Decisions

- D1. **The engine admits the session at abandon.** `Abandon` accepts a
  proof that is `ValidFor` the root or `SessionValidFor` it; the
  mutation calls `recordSessionAuthority` on the history line it
  appends, as park does, so the ledger names the session as the hand.
  Nothing else in the verb changes: the reason rule, the dependents rule
  and the successor rule stand, and a session cannot waive or
  also-abandon dependents from the browser (D2).
- D2. **The act layer carries it.** `act.Authority.Abandon(id, because,
  successor)` beside `Park`, publishing `goal.Abandon` with
  `AbandonSpec{Because, Carried}` and no `Waive` or `Also`, through
  `request()` and `settle`; a goal with live dependents is refused by the
  engine in its words, which name the terminal forms, and the line shows
  them.
- D3. **One route, one row.** `POST /api/backlog/goals/<id>/abandon`
  with `{because, successor}` (`successor` optional), refused empty as
  the engine refuses it, the policy of every act route; `routeAbandon =
  "abandon-goal"`; the act table gains `{Title: "Abandon a goal",
  Requires: ledgerHand, Does: "Records that a goal will never be worked,
  with the reason, and the live goal carrying its work where there is
  one."}` and the completeness test's list gains the route.
- D4. **The Partner's catalogue and the card.** `abandon` joins the
  catalogue as `goal abandon G --reason TEXT [--successor G2]`, fields
  `reason` and `successor`, mapped to `abandon-goal` `{because,
  successor}`; `waive` and `also` refused by name with the terminal form;
  the card's word is `Abandon`; the client `abandonGoal(id, because,
  successor)`; the inbox row and the chip take the word through the map.
  The skill's list of nine becomes ten.
- D5. **Nothing else moves.** No Abandon button on the board or the
  Decisions page (the Partner's verb is the ask; a button is one later
  slice); no `done`, `reopen`; no waive or also from the browser.

## Not here, later

An Abandon act on the queue row and the goal page; `--waive` and
`--also` from the browser; `goal done` and `goal reopen` as acts and
verbs, each with its own admission.

## Verification and box

Go: engine tests that a session proof is admitted for abandon and the
history line names the session, that a terminal proof is still admitted,
that no proof is refused, that a goal with a live dependent is refused
with the engine's words under a session; act tests for `Abandon` with a
reason, with a successor, and blank; route tests for policy, body and
refusals; the describe table's row and the completeness test; the
catalogue row, the join tests on both sides, `waive` and `also` refused
by name; the word map. Frontend: the client, the card's word, the inbox
row's and the chip's words; the guards stay green; the bundle rebuilt.
Walkthrough: the fake Partner's canned proposal gains an abandon line
and the fixture's canned acts admit it; screenshots at 1280 and 400 of
the line waiting and applied. Budgets as always. Box: one build lane
(Claude on Opus), one code read (Codex on Sol) with one fix round under
R-124, after Astra's read; one attempt, 120 to 180 job-minutes; lands
after g1-s62.

## Self-grade

High on D2 to D4: park's pattern copied row for row. Medium on D1: a
second verb admits the session, on the ruling's words as recorded here;
the tests name the admitted and the refused. Weakest: the ruling is
Wido's sentence read as one, not a row in the rulings register; it is
recorded here and the register row is his to add.
