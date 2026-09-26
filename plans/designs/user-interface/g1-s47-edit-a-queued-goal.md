# g1-s47: edit a queued goal from the browser

- Kind: design
- Id: 01M3CAGZRWXJ87YWPQRQGRKJYJ
- Status: done
- Goals: browser-interface

Wido, 2026-09-25: "why can't I edit a goal?", then "yeah, go ahead" to
the offer of the master design's direct edit as one slice. Author Fable.
Step 1 of the goal editor, the part the interface design admits as a
direct act; every cite re-read at `b3f968a50`.

## 1. What exists and binds

1. **No edit act exists in the interface.** The server publishes six acts,
   approve, unapprove, set-priority, open, block and unblock
   (internal/ui/httpd/acts.go:16-46); the board's card menu offers Ask,
   Approve, Withdraw, Move up, Move down, Set priority and Open goal
   (src/backlog/menu.ts:36-43, 57-77). The goal page slice fenced the
   editor out: "Not: any write; the goal editor (gate 4)"
   (g1-s11-goal-detail-design.md:21). The master design names the full
   editor at line 510; its fourth gate, "first complete edit", covers the
   queued case and says it does not claim the whole editor
   (user-interface-design.md:817),
   and at line 119 it names the one direct edit it admits without a
   proposal: a queued goal that is not approved, claimed or parked, and
   only `Intent`, `NextStep` and `Labels`, because `Blocked` changes
   other goals' readiness and `Tier` and `Risk` enter the approval digest.
2. **The engine has the verb.** `goal.Edit(r, id, EditFields{Intent, Tier,
   Risk, NextStep, Blocked, Labels, Why, Evidence, Proof})`
   (internal/goal/verbs.go:3281-3304). Its mutation refuses the archive
   and an approved goal's intent ("unapprove the goal, edit it, then
   approve the new intent"; verbs.go:3343-3349), refuses tier and risk on
   an approved, claimed or parked goal unless a raise (3355-3376), and
   admits a human's edit of a parked goal and of another pair's claimed
   goal, tested by `Actor.Human != ""` alone, recording a displacement on
   the claimed one (`touchDisplaced`, verbs.go:3393-3398, 3410-3418,
   584-591). Labels are validated against `^[a-z][a-z0-9-]{0,31}$` and
   written sorted and deduplicated (internal/goal/goal.go:424, 428-454).
   The edit's history line does not name a session:
   `recordSessionAuthority` is called by approve, order and two verbs at
   verbs.go:1854 and 1897, not by edit. The terminal form is `metasystem
   goal edit --id <id> --intent … --next … --label … --by <name>`
   (cmd/metasystem/goalsync_mutations.go:1092-1151).
3. **The page and the sheet idioms.** The goal page is `/backlog/goal/:id`,
   `GoalPane` in src/project/ProjectPane.tsx, which loads the backlog,
   finds the goal's own row (`mine`, ProjectPane.tsx:835-836) and already
   acts from it: the edges block adds and removes blockers through `run`,
   which carries the sign-in retry every act has (ProjectPane.tsx:846-866).
   The open sheet has the three fields with their rules and the board's
   known labels (src/backlog/OpenSheet.tsx:262-300, 169, 396-401); the
   act clients are one `request()` each (src/backlog/api.ts:297-318); the
   card menu dispatches an offer by id (src/backlog/Board.tsx:635-646). The
   open route's body and act mirror what an edit needs (acts.go:102-118;
   internal/ui/act/act.go:281-340).

## 2. Decisions

- D1. **The allowlist is the mutation's, not the page's.** `EditFields`
  gains `QueuedOnly bool`; when set, the mutation refuses unless the goal
  at the tip is `StateQueued` with no approval, in the engine's words:
  "goal <id> is approved: withdraw the approval, edit it, then approve it
  again", "goal <id> is claimed by <pair>; edit it at a terminal", "goal
  <id> is parked: return it to the queue to edit it". So an approval or a
  claim landing between the page's read and the act is refused, never
  displaced. The act layer sets it always and passes only the three
  fields; the terminal verb is untouched.
- D2. **One route, three fields, only the changed ones.** `POST
  /api/backlog/goals/<id>/edit` with `{intent, nextStep, labels}`, each
  optional; the sheet sends only the fields whose value differs from what
  it was opened with, so a terminal edit of a field the human never
  touched survives the save (the engine's fields are nullable for this,
  verbs.go:3281-3291; the terminal supplies them selectively,
  goalsync_mutations.go:3648-3673); an explicit empty label list is sent
  as such, an untouched one is omitted; a sheet with nothing changed
  refuses to send; blank intent or next step refused as open refuses them; a label outside the grammar refused with the engine's message;
  line breaks folded as open folds them. The policy of every act route.
- D3. **The act layer carries it and the ledger names the session.**
  `act.Authority.Edit(id, Edited{Intent, NextStep, Labels})` beside
  `Open`, through `request()` and `settle`; the edit mutation calls
  `recordSessionAuthority` on the line it appends, as approve does.
- D4. **Two doors to one sheet.** On the goal page, in the goal's header
  beside its intent (ProjectPane.tsx:774-790), "Edit…" when the row's
  `state` is queued with no approval; otherwise the reason in words from
  the state, because `waiting` also covers an approved goal's dependency
  wait: "approved: withdraw the approval to edit the intent", "claimed by
  <pair>: edit at a terminal", "parked: return it to the queue to edit". On the board, the card menu offers "Edit…" under
  the same condition, between Ask and the lane moves. Both open the edit
  sheet: the board's `Panel` chrome, prefilled from the row, Intent and
  Next step with the open sheet's rules, Labels with the open sheet's
  field and the board's known labels; Save and Cancel; the refusal in
  the engine's words; on success the page reads again and the sheet
  closes. The Decisions queue row keeps "Open the goal" and gains no
  edit in this step.
- D5. **Nothing else moves.** No tier, risk, dependency, why or evidence;
  no edit of an approved goal; no proposal flow; no payload change.

## 3. Verification and box

Go: engine tests that `QueuedOnly` admits a queued unapproved goal and
refuses an approved, a claimed and a parked one with the three sentences,
and that the line names the session; a test that a terminal change to
the intent survives a browser save that changed only the next step; act tests for `Edit` with the three
fields and a blank; route tests for policy, body and refusals; testing.json
groups and surfaces for the touched paths. Frontend: the sheet's prefill,
blank refusal, label grammar, send and re-read, refusal shown; the goal
page's button and its three reasons; the menu offer's condition; the
guards stay green. The walkthrough fixture gains a canned edit and one
goal in each of the four states; screenshots at 1280 and 400: the goal
page with Edit, the sheet, a refusal, the menu. Budgets as always. Box:
one build lane (Claude on Opus), one code read (Codex on Sol) with one
fix round under R-124, after Astra's read; two attempts, 90 to 150
job-minutes; lands on ui-development only, main is on hold.

## 4. Not here, later

Tier and risk, with their why and evidence. Dependencies, which have
block and unblock already. Editing an approved goal as withdraw, edit,
approve in one flow. The Partner's proposal flow (gates 2 and 3) and the
full editor (master :510). Edit from the Decisions queue row. Reopening
an archived goal.

## 5. Self-grade

High on D2 to D4: they copy the open act and the open sheet field for
field. Medium on D1: a new flag on a verb the terminal and recovery
replay share; it is additive and unset everywhere but the interface, and
the tests name each refused state. Weakest: the goal page is the project view narrowed to a goal, not the
detail page g1-s11 designed; the button sits in the header that view
already has, which is where the intent is read.

## Dispositions (Astra read, 2026-09-25, under R-124)

One material finding, two deferred; every code claim checked.

| id | finding | fold |
|---|---|---|
| F1 | saving all three fields whole republishes untouched values, so a terminal edit of a field the human never changed is overwritten | the sheet sends only changed fields; the engine's nullable fields carry it; a test that a terminal intent change survives a browser next-step save |

Folded because they cost nothing: the button's home is the goal header
the page already has, not a row; the fourth gate's own words; the park
and wait reasons read `state`. Astra verified the state guard is complete
on a validated tree, that recovery replay and reconcile need no flag, and
that naming the session on the edit line is sufficient.

## Built (2026-09-25)

Landed on ui-development and main in one merge. Built by Claude on Opus,
read by Codex on Sol under R-124: one material finding, the sheet could
be dismissed while a save was in flight, fixed in one round with the
panel's busy guard; two deferred because step 1 works and is safe
without them.

Later, when it hurts: the walkthrough's canned edit refuses in the order
approval, claim, park, where the engine reads the state first; the
route's new test file has no surface path of its own in testing.json
and falls to the interface group.

Departures the read accepted: the state is read before the approval in
the engine, because an approval survives a claim and a park; the
walkthrough already held a goal in each of the four states, so none were
added; a state outside the four shows neither the button nor a reason;
the nil-engine guard lives in the handler so an engine without the
editor still approves and parks; testing.json was not edited, the plan
forbids it, and the touched paths fall under existing surfaces.
