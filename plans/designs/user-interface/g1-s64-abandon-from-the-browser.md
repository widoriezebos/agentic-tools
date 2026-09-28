# g1-s64: abandon, from the browser and from the Partner

- Kind: design
- Id: 01M3GZ2C32STTMZB001PEVRYRN
- Status: done
- Goals: browser-interface

Wido, 2026-09-27, on the question left open since 2026-09-26 whether
"close these goals" may mean more than Pause: "agreed on the 'Abandon'
topic; that needs to fit the verbs", and then "Finish all now". Taken as
the ruling in R-125's shape: a signed-in browser session may abandon a
goal from the interface, the session proof admitted for `goal abandon`
at the grade that verb requires of a human, recorded on the history
line as park and unpark are; and abandon joins the Partner's catalogue
as one more public goal action. Author Fable. Every cite read at
`2c87659a4`. Revision 2, after Astra's round-1 read: two material
findings and two low, all folded at the foot; the ruling captured in
the register.

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
  Nothing else in the verb changes but one word: the refusal for a live
  dependent names the public flag, `--successor`, where it names the
  internal `--carried` today (abandon.go:240; Astra S64-03). The reason
  rule, the dependents rule and the successor rule stand, and a session
  cannot waive or also-abandon dependents from the browser (D2).
- D2. **The act layer carries it, and the card says what a successor
  does.** `act.Authority.Abandon(id, because, successor)` beside `Park`,
  publishing `goal.Abandon` with `AbandonSpec{Because, Carried}` and no
  `Waive` or `Also`, through `request()` and `settle`. Without a
  successor, a goal with live dependents is refused by the engine in its
  words, which name the terminal forms, and the line shows them. With a
  successor the engine does not refuse: it repoints every live dependent
  from the abandoned goal to the successor in the same act
  (abandon.go:233-239, 315-329; Astra S64-02), so the proposal is read
  with the dependents in hand. At admission the service lists the
  goal's live dependents from the observation, the rows whose blockers
  name it, and carries them under `read` beside the intent, next step,
  tier and labels as for an approve or an edit; the card's line says
  "n goals wait for it: X, Y" and, with a successor, "they will wait for
  S instead", or, without one, "the engine will refuse until they are
  waived or abandoned at a terminal", so the press is an informed one.
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
  The skill's list of nine becomes ten. **Abandon is a guarded line**
  (Astra S64-01): like an approve and an edit, its `read` carries the
  goal's intent, next step, tier, labels and live dependents as admitted,
  the line shows the intent whole, and the runner refuses it unsent,
  "the goal changed since this was proposed; open it and ask again",
  when the fetch-first read shows any of them changed, dependents
  included, since abandoning is not reversed by a press (`goal reopen`
  is a terminal act).
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

High on D2 to D4: park's pattern copied row for row, with the guard
the irreversible act needs. Medium on D1: a second verb admits the
session, on the ruling's words as recorded here and, since revision 2,
in the rulings register; the tests name the admitted and the refused.
Weakest: the dependents are listed from the observation's rows at
admission and compared again at the press, which is the page's
compare and not the owner's; the engine's own dependents rule inside
the transaction is what holds the act.

## Dispositions (Astra round 1, 2026-09-27, under R-124)

Two material findings, two low, all accepted; every code claim checked.

| id | finding | fold |
|---|---|---|
| S64-01 | the freshness guard covered approve and edit only, so a stale card could abandon a goal whose intent or next step had changed in another tab | abandon is a guarded line: `read` carries intent, next step, tier, labels and live dependents; the line shows the intent; the runner refuses a changed goal unsent (D4) |
| S64-02 | with a successor the engine does not refuse live dependents, it repoints them, and the card said nothing of it while D2 claimed a refusal | D2 says what a successor does; admission lists the live dependents from the observation and the line says "n goals wait for it" and what becomes of them with and without a successor |
| S64-03 (low) | the engine's refusal for a live dependent names the internal `--carried` where the public flag is `--successor` | one word in the refusal, inside the D1 edit of abandon.go |
| S64-04 (low) | the ruling was left for Wido to add to the register, which must not leave the only copy in an owning document | the coordinator captures Wido's sentence in `metasystem/memory/rulings.md` with its context, in the same commit as this revision |

Astra confirmed D1 otherwise: the act layer's `SignedIn` supplies the
human's name, `request()` carries it with the proof, and abandon's
appended history event can take `recordSessionAuthority`; D3's route
and the `successor` mapping fit the existing owners.

## Dispositions (Astra round 2, the failsafe round, 2026-09-27)

Zero material findings, "build as written": the guard's fold needs no
new reader, since the observation's rows filtered by their blockers
match the engine's own dependent selection; D2's conditional refusal is
right; the ruling is in the register. The design is closed, read whole
by the critic; it builds now, in parallel with g1-s62, and lands after
it.

## Built (2026-09-27)

Landed on ui-development and main: eight commits on `ui/g1-s64`, one
per decision, the walkthrough, the bundle, the fix and the bundle
again, built by Claude on Opus, rebased onto the g1-s63 landing with no
conflicts, read by Codex on Sol under R-124
([g1-s64-sol-read.md](g1-s64-sol-read.md)): one material finding,
reproduced and fixed in the one fix round; all nine of the builder's
departures accepted. Go, vet, the whole `internal/goal` suite and the
interface packages green; the fast gate and the parallel ratchet
passed; 91 test files, 1,407 frontend tests; the bundle rebuilt twice
to one digest. Sixteen screenshots at 1280 and 400, light and dark: an
abandon line waiting with its intent and reason, one whose goal two
others wait for with the "Holds up" line and what the engine will do,
the first applied, the second refused in the engine's words naming
`--successor`.

The fix round: the Abandon line shows the goal's intent whole above
the reason, through the approve line's own argument and label, so the
irreversible press is informed where the title is cut at the first
sentence; held by a two-sentence intent whose second sentence is only
on the argument, and by a line admitted without a reading showing no
intent and not throwing.

Landing order: this slice landed before g1-s62, which was still
building; the catalogue row therefore went in a transitional shape
(`ProposedAct.Verb` and `.Body`, `ProposedActNamed`, `ProposeVerbs()`,
the body map in `framed()`, the httpd join through `BodyField`) that
keeps the nine rows under their route ids beside `abandon` under its
public name; g1-s62 folds the ten into its one public-name shape and
retires the transitional fields. Sol read that shape as a departure
and accepted it.

Departures the read accepted: the proof travels into the mutation as
a parameter so the recorded hand is the admitted one; three existing
engine assertions follow the refusal's new sentence; the session is
recorded on the abandoned goal's own line, not on the repointed
dependents' merged lines; the successor is not checked at admission,
the engine's rule holds it; the skill has no numbered list, the tool's
description names the ten; the dependents label is the interface's
"Holds up"; the catalogue join was red for one commit between the two
sides; the successor sentence is held by a test, not a screenshot.

Later, when it hurts: a session test that tries a proof minted for
another root (the existing authority test covers another project's);
an already closed goal with unchanged fields passes the page's compare
and is refused by the engine; the walkthrough's abandoned goals have
one-sentence intents, so the screenshot's Intent equals the title and
the test holds the distinction; `recordSessionAuthority` on the
dependents' repointed lines; an Abandon button on the queue row and
the goal page; waive and also from the browser; `goal done` and `goal
reopen`.
