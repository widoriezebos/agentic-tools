# Design for design-gate-at-dispatch

- Kind: design
- Id: 01M4189Q0RH1NSPD3PNAS6G177
- Status: draft
- Goals: design-gate-at-dispatch

Revision 3. Author: Fable (claude-fable-5-1), design delegate of seat m1k, 2026-10-03. Cites new in revision 3 were read at main `b03b29ff1`, earlier ones at `1f6edaf24` and `19a4f3335`; paths are under `metasystem/`. Folded in revision 3: DESIGN-GATE-RECORD-IDENTITY, the one accepted finding of Astra's critique round 2 (section 10), in passages marked "(revision 3)". "(revision 2)" marks round 1's folds; sections 10 and 12 are condensed.

The goal, as Wido approved it: "Work that needs a design cannot start as a build unless an accepted, critiqued design stands behind it." Folded in at his word: the landing half (`landing-design-provenance`), and a governance record for this gate and for `commit-goal-binding`'s refusal, warning-only first (`gate-governance-records`).

## 1. What is true today

- **The live door.** `metasystem work build G` runs `runIntentBuildUnit` (`cmd/metasystem/intent_work.go:494`): after the goal, budget and claim checks (`:562-590`) come the accepted designs (`:592`), the request (`:598`) and the unit runner (`:623`). With no record of `Status: accepted` the brief stands in (`:877-905`) and nothing is printed. A unit with no run yet creates its input folder, calls the caller's `prepare`, which writes the plan and briefs, and keeps `request.json` (`internal/launch/unit_named.go:165-174`). The hidden `work build --plan FILE` (`intent_work.go:224,464-487`) never reaches `:592`.
- **The unit store (revision 2).** `~/.metasystem/unit` (`internal/launch/unit_run.go:837-843`), one per computer. A unit's input folder is named by a hash of worktree, goal and unit (`unit_named.go:311-318`), so a reader that knows only the goal cannot find it.
- **Landing (revision 2).** With a landing lane registered, the seat's `work land G` passes `admitLanding` (`cmd/metasystem/intent_delivery.go:1910`) and only queues the branch (`landing_plain.go:69`). The lane's landing agent later merges the queued commits, proves the result and runs `landing push` (`skills/landing-agent/SKILL.md:17-18,29-32`), whose one check is a green proof of HEAD's tree (`intent_landing_push.go:47-53`, `internal/landing/plain/push.go:50,72`). Without a lane the seat lands by hand: prepare (`intent_delivery.go:1997`), `admitLanding` again (`:2012`), push (`:2020`).
- **The landing evaluator (revision 2).** `landing.Observe` (`internal/landing/observe.go:160-175`) judges one prospective tree. Only the hand route calls it (`landing_path.go:142-172,228-230`; its result is read at `internal/landing/landpath/commit.go:484-508,616-620`); the lane's push never does. Its caller hands it what it cannot read itself as functions (`observe.go:76-77`). A person's exception landing, `work land G --exception CODE --reason TEXT --by NAME`, branches off before the lane's hand-in (`intent_delivery.go:1687-1689`) and is judged apart (`observe.go:165-166`). The precedent for a computed check that does not refuse is `chain-not-design-bearing` (`:41-44,476,1724-1735`).
- **A person's exception per goal (revision 2).** `goal.Permissions` (`internal/goal/permissions.go:29-38`): "a new permission is one row here and one field in the record". `goal allow` refuses an agent session (`cmd/metasystem/intent_goal_permission.go:54-65`), asks for the person's proof (`:66`) and records `Allowed: NAME why=REASON` in the goal's history (`permissions.go:98-108`, `verbs.go:3849-3859`). Neither the verb nor the record has a place for a statement of impact; nothing in `cmd/metasystem` or `internal/goal` prints one.
- **Messages and the digest.** A message a person reads is two lines: what happened and why, then the one command that resolves it (`internal/refusal/register.go:3-9`). The narrator's digest is a tracked file in the installation's records (`internal/narratordigest/digest.go:78-84`); a line it already holds is never added again (`:181-183`).
- **The sweep does the governance half.** Each steward tick runs `sweepRulingReviews` (`internal/steward/ruling_sweep.go:210-241`): a row of `memory/rulings.md` with a review column `class=… due=YYYY-MM-DD` gets, once due, a digest lowlight asking to adopt, revise or withdraw it.

## 2. The design

As shipped it stops nothing.

**The evidence that travels.** Whoever sets `Status: accepted` adds one head line in the same edit:

```markdown
- Critique: closed at round 4 on 0 material findings (Codex Astra, chain design-critic-09e9…)
- Critique: ruled by Wido 2026-10-03, R-144-m1h
```

The first word is `closed` (the critique loop ended) or `ruled` (a person accepted the design). The head grammar admits the line today (an unknown head key is kept, `internal/project/record.go:47-48,75`); it is committed with the record and is as trustworthy as the hand-set status beside it. Local critique state (in the checkout's ignored `artifacts/`, not following a claim) may contradict the line, never replace it.

**The check (revision 2).** One function in a new package, `internal/designgate`, with no file, setting or git access, so the build door and the landing evaluator run the same code. `designGateFacts(root, goal)` in `cmd/metasystem/intent_design_gate.go` gathers its inputs: the goal's tier (`goal.GateTier`; no tier reads as 3), its permissions, its design records, this checkout's critique chains for each accepted page (`dispatch.DesignCritiqueChains`) and the mode. The build door calls it at `intent_work.go:592`. The first match is the verdict:

1. `not-design-bearing`: tier 1.
2. `allowed`: the goal holds `build-without-design`.
3. `no-accepted-design` (would refuse): no record with `Status: accepted` names the goal.
4. `critique-not-recorded` (would refuse): an accepted record has no `Critique` line starting `closed` or `ruled`.
5. `critique-open` (would refuse): the line says `closed`, and a chain for the page on this checkout is not closed.
6. `ok`: every accepted record carries the line, uncontradicted.

`unchecked` (section 3) never refuses.

**The warnings (revision 2).** Every message is two plain lines: the reason, then the one command. A would-refuse verdict prints its pair and the build goes on; `--json` carries `designGate: {verdict, wouldRefuse, mode}`.

```text
warning: goal G has no accepted design; this build runs on its brief alone
metasystem design write G --brief FILE

warning: the accepted design NAME does not say its critique closed; the build goes on
edit PATH: add "- Critique: closed at round N on 0 material findings (WHO)" under its Goals line

warning: the accepted design NAME says its critique closed, but its review here is open at round N
metasystem design review PATH --dispositions FILE --after N

warning: the design check could not run (CAUSE); this build was not checked
metasystem design list --goal G

warning: the design check's record could not be written (CAUSE); the build goes on
nothing to do: the landing check runs without it
```

For a goal with a draft design the first command is `metasystem design review PATH`. The second pair ends in an edit: no command writes that line today (question 1).

**The dispatch record (revision 2; keyed by project in revision 3).** The gate keeps its own record, through its own writer, at `~/.metasystem/unit/.design-gate/<ledger-identity>/<goal>/<unit>.json`: outside the unit's input folder, which holds the launch's mandatory inputs, and found by identity and goal alone. `designGateFacts` reads the identity, the ULID minted once into the goal list's root record (`internal/goal/root.go:24`), with `goal.ExistingLedgerIdentityAtEndpoint` (`internal/goal/repository.go:92-111`); when that is empty (a lane clone, `cmd/metasystem/intent_landing.go:128`, may lack the goal list's ref), from the checkout's tracked `plans/goals/backlog.md` with `goal.ParseRoot` (`root.go:370`). So one project's checkouts and lane share a record; two projects never do. Its fields: schema, ledger identity, goal, unit, worktree, tier, mode, verdict, wouldRefuse, governedBy, the time, and for each accepted design its id, path, sha256 and critique line. `prepare` (`intent_work.go:728-775`) calls the writer, so it runs under the unit's lock and only while the unit has no run: a build that starts a run replaces the record, a repeat of a running unit leaves it, and a landing compares against the latest dispatch. That lock is per worktree (`unit_named.go:144-148`), so the file is written whole: of two checkouts writing at once, the later record stands. An identity that is unreadable or not 26 Crockford base32 characters (`root.go:436-439`) writes no record (section 3). The same call appends one digest lowlight (`digest.go:153`), source `design-gate G/u`, text the warning's first line; the review counts those lines. The record is not a field of `request.json`, whose bytes identify the request (`unit_named.go:160-164`): a permission granted mid-run would change them.

**The switch.** One setting, `design.gate.mode`: `warn` (the default) or `refuse`, declared as `landing.review.human-from-tier` is (`internal/config/landinggate.go:16`, `defaults.go:242`, `validate.go:624`). Under `refuse` a would-refuse verdict at `work build` returns, before anything is written, the reason ending "; nothing was built", then `metasystem goal allow G build-without-design --reason TEXT`; `--verbose` adds `BUILD_DESIGN_NOT_ACCEPTED goal=G verdict=V governed-by=R-…`. The code gets the first register row of shape `Warning` (`register.go:24`).

**The person's way past (revision 2; R-143-m1e, `memory/rulings.md:202`).** `metasystem goal allow G build-without-design --reason TEXT`: one row in `goal.Permissions` (words: "building and landing without an accepted design"), one record field written `- DesignGate: off`, one help line. It covers the goal's landing too. The row gains a field `Impact`. `runIntentAllow` prints it before the edit, and also when the command is run without a reason (the existing refusal, `intent_goal_permission.go:49-53`), so a person can read it before deciding:

```text
This lets goal G be built and landed without an accepted, reviewed design.
The risk: nobody checks the approach before code is written, so a wrong one is found only in code review or after it lands.
It holds for every later build and landing of G until withdrawn.
To undo: metasystem goal disallow G build-without-design
```

`permissionReason` records the printed text in the goal's history, `Allowed: build-without-design why=REASON impact=STATEMENT`, which `goal show G --history` prints. A permission without an `Impact` (today's `stop-test-changes`) behaves as now.

**The landing half (revision 2).** It moves into the landing evaluator; `landGoalRoute` is not touched.

- *One function*, `landing.ObserveDesign(facts, person)` in `internal/landing/observe.go`. It always runs the full check on the design as it stands now. Only when that says `ok` does it compare with the goal's dispatch records: `design-missing` when a recorded design id is no longer an accepted record of the goal, `design-changed` when a recorded sha256 differs from the file's. Both would refuse. So an unchanged design with a missing or contradicted critique line warns at landing too.
- *Where it reads from.* The caller hands in the facts as a function, as it does `BindAttested`: the same `designGateFacts`. Tier, permission and design records come from the checkout the evaluator runs in: the seat's on the hand route, the lane's (whose HEAD the push publishes) in the lane. Dispatch records come from this computer's unit store, by that checkout's identity and the goal (revision 3); the lane runs on the computer its seats build on.
- *No dispatch record* (built elsewhere or before the gate, the write failed, or no identity was readable (revision 3)): the fresh check still decides, the comparison is skipped, and the verdict says `compared: false`. A missing record alone is never a would-refuse.
- *The hand route.* `ObserveParams` gains `DesignFacts`; both builders set it (`landing_path.go:151`, `landing_verbs.go:58`). `observeWithFacts` (`observe.go:164`) attaches the verdict to the observation as `Design`; `observationView` (`landpath/commit.go:499-508`) carries it and the landing path prints the pair; `landingPathObserve` appends the digest lowlight, source `design-gate-landing G@TIP`.
- *The lane.* `runIntentLandingPush` calls `ObserveDesign` before `plain.Push` (`intent_landing_push.go:53`) for each waiting hand-in that HEAD contains and origin's main does not (as `landedMessage` selects, `:20-32`), and prints the pair. The lane checkout is reset every round and the digest is a tracked file, so the push writes no digest line: it appends one line (goal, commit, verdict, reason, time) to `design-gate.jsonl` in the lane's record folder (`internal/landing/plain/queue.go:38-40`). The steward's tick, which already reads that folder (`internal/steward/lane_silent.go:62-86`), offers each line to its digest, which keeps only new ones.

```text
warning: goal G was built against NAME, which changed after the build started; the landing goes on
metasystem design review PATH

warning: goal G was built against NAME, which is no longer an accepted design of G; the landing goes on
metasystem design list --goal G
```

The fresh check's pairs are the build's, ending "; the landing goes on". Under `refuse` the hand route returns a refusal coded `LANDING_DESIGN_NOT_STANDING` (added to `knownRefusalCode`, `observe.go:1744`, and to the register beside `register.go:288`, shape `Agent`): the reason ending "; nothing was landed", then the `goal allow` command. The lane's push is refused before publication: "; nothing was pushed", then `metasystem landing return G --reason TEXT`.

**Whose act it is (revision 2; R-142-m1e, `memory/rulings.md:201`).** Under `warn` nobody is stopped. Under `refuse`:

- *An agent's act, which the gate may stop:* `work build` from an agent session; the seat's `work land` on the hand route; the landing agent's `landing push`.
- *A person's act, never stopped:* recording the allowance; accepting a design by ruling (`Critique: ruled …`); setting the mode; a build started from a terminal with no agent lineage (the first test `goal allow` applies, `intent_goal_permission.go:54-56`; question 2); the exception landing `work land G --exception LANDING_DESIGN_NOT_STANDING --reason TEXT --by NAME`, for which `person` is true. When a person's build or landing meets a would-refuse verdict, the pair is printed ending "; it goes on at your word", the verdict is recorded with `person: true`, and the act goes on.

**The governance records (revision 2).** Wido ruled both rows as worded below, owned by him, first review 2026-11-03. A seat cannot land a change to `memory/rulings.md` in a build commit today (goal `records-land-through-the-lane` is open), so the build does not edit the register: the seat mints the two ids under the register's rule, writes the rows into its hand-in note, and seat m1e lands them with the unit.

- *Design gate.* "GATE RECORD: the design gate (`BUILD_DESIGN_NOT_ACCEPTED` at work build, `LANDING_DESIGN_NOT_STANDING` at work land), warning-only under `design.gate.mode=warn`. MUST REFUSE, once refusing: a tier-2 build of a goal that no accepted, critiqued design names. APPEAL: `metasystem goal allow G build-without-design --reason TEXT`. IF THE CHECK BREAKS: the work goes on and its output says the check did not run." Owner Wido. Review `class=experimental due=2026-11-03`.
- *Goal-holder refusal.* "GATE RECORD: `GOAL_BRANCH_NOT_HOLDER` (goal commit-goal-binding), refusing. MUST REFUSE: a session building or committing on goal G while another session holds it. APPEAL: a person runs `metasystem goal claim G --take-over --reason TEXT`. IF THE CHECK BREAKS: it refuses, says the claim cannot be checked and names `metasystem goal list`." Owner Wido. Review `class=delegated-authority due=2026-11-03`.

A three-entry map `refusal.GovernedBy` (code to ruling id) gives each gate its `governed-by` detail. The two rows are also kept as a fixture, `internal/steward/testdata/gate-records.md`.

**The tests, each red without its part.** Git is never run. Build tests use the work bed (`newWorkBedWith`, `intent_work_test.go:166`); the chain reader, identity reader (revision 3), record writer and digest writer are function fields of it.

1. `TestDesignGateWarnsAndStillBuilds`: a tier-2 goal with no design builds; the output has the pair, the record says `no-accepted-design`, the digest has one line, a repeat adds none.
2. `TestDesignGateVerdicts` (`internal/designgate`): one case per verdict, asserting verdict, pair, recorded id and sha256.
3. `TestDesignGateNeverStallsWhenItBreaks` (revision 2): a failing chain reader, identity reader (revision 3), record writer and digest writer in turn; the build starts and the matching pair is printed. The unit's input folder is left alone: no build starts without it (`unit_named.go:165-174`, `intent_work.go:763-769`). Unit 2 repeats them under `refuse` and adds an invalid mode.
4. `TestDesignGateRefusesOnlyWhenSwitchedOn`: under `refuse` an agent session's build without a design launches nothing and names `goal allow`; with a matching accepted design it builds; from a session with no lineage it is warned, recorded with `person: true`, and builds. This is the goal's done-when.
5. `TestAllowBuildWithoutDesignShowsAndRecordsItsImpact` (revision 2): without a reason the statement is printed and nothing changes; with one it is printed before the confirmation, the record holds `- DesignGate: off` and the history reason the statement after `impact=`. `internal/config`: default `warn`; `refuse` valid; anything else a settings problem.
6. `TestLandingDesignCheck` (revision 2; `internal/landing`, facts handed in): unchanged and critiqued is silent; unchanged without the critique line warns `critique-not-recorded`; unchanged with an open chain, `critique-open`; an edited page, `design-changed`; a `superseded` record, `design-missing`; no dispatch record with a standing design is `ok`, `compared: false`; failing facts, `unchecked`. Under `refuse` each would-refuse refuses an agent's landing and only warns a person's.
7. `TestLandingPushChecksDesign` (revision 2; the lane bed, `landing_lane_test.go:26`): a waiting hand-in whose design changed is pushed, with the pair in the output and one line in `design-gate.jsonl`; under `refuse` nothing is pushed and `landing return G` is named. `internal/steward`: two ticks put the line in the digest once.
8. `TestGateRecordsAreGoverned` (revision 2; `internal/steward`). *Sweep half*, the same before and after the rows land: the fixture becomes the register of a temporary root (as the helper at `ruling_sweep_test.go:15` writes one), the sweep runs one day past due, and the digest line names every id in `refusal.GovernedBy`. *Live half*: each id is looked up in this repository's register (as `internal/rulings/rulings_test.go:262-265` reads it); an absent id is logged "not in the register yet" and passes; a present one must have an owner, a typed class, a due date and the three labels. Wording is not compared: Wido can reword a row.
9. `TestDesignGateRecordsStayApartByProject` (revision 3; unit 1): two work beds share one unit store and report different identities; each builds goal G, unit `main`, and both records are found. A third bed with the first's identity and another worktree builds the same and replaces the first's record.

## 3. When the check itself fails

A broken check is the verdict `unchecked`: it prints the fourth pair, is recorded when that can be written, and never refuses, in either mode.

- **A record the project reader does not list for the goal** (an unreadable head, no `Goals` line): to the gate no such design exists, so `no-accepted-design`.
- **The setting is unreadable or neither value**: `warn`; `metasystem settings check` names it.
- **The gate's record or digest line cannot be written (revision 2), or the project's identity read (revision 3)**: the fifth pair; the build goes on; the landing check skips the comparison.
- **At landing, the facts cannot be read (revision 2)**: `unchecked`; the landing or push goes on. A lane record line that cannot be written stops nothing.

## 4. Who can be stopped wrongly

Under `warn` these are wrong warnings; each would be a wrong stop under `refuse`.

- **A claim that moved to another computer.** Every checkout computes the same fresh verdict from the committed head; the dispatch record stays behind, so nothing is compared and nothing refused for that.
- **A design the reader does not find for the goal**: a plan page outside `plans/designs/`, an arc's shared design whose `Goals` line omits the goal, a design already `done`. Each is `no-accepted-design`.
- **Designs accepted before this gate.** None of the 11 has the head line; each warns `critique-not-recorded`, at build and at landing, until its seat adds it.
- **Adding that line after a build started (revision 2).** The file's sha256 changes, so the landing warns `design-changed` once; so does a "Built" note or a person's amendment.

## 5. Moved effects

No owner moves.

## 6. New effects (revision 2)

New, not moved, so outside the inventory above: digest lines from the build command and the hand landing (sources `design-gate`, `design-gate-landing`); `.design-gate/<ledger-identity>/` in the unit store (revision 3); `design-gate.jsonl` in the lane's record folder, read by the steward's tick; `critique` in the designs listing, `designGate` in the build result, `design` in the landing observation; `- DesignGate: off` in a goal record and `impact=` in its history.

## 7. Units, in landing order (revision 2)

The smallest first use comes first and lands alone (R-121); each later unit leaves main working without the ones after it.

1. `gate` (340 lines; revision 3): the check, warnings, dispatch record and digest line at `work build`; tests 1-3 and 9. Needs nothing: the mode is fixed at `warn`, the `allowed` verdict absent.
2. `switch` (200): the setting, the refusing branch, the allowance and its impact statement; tests 4-5. Needs unit 1.
3. `landing` (190): `ObserveDesign`, the comparison, the hand route; test 6. Needs unit 1's record, unit 2's mode and allowance.
4. `lane` (150): the check at `landing push`, the lane's record line, the steward's carry; test 7. Needs unit 3.
5. `governance` (80): `refusal.GovernedBy`, the fixture, test 8, the two rows in the hand-in note. Needs the codes of units 2 and 3.

About 410 production lines and 550 test lines (revision 3).

## 8. Guardrails the build must pass

- `go test ./internal/... ./cmd/metasystem`: the new tests pass; the existing build and landing tests stay unedited (a work-bed goal without a tier reads as tier 3, so they may now see a warning); the new codes have register rows.
- Unit 5 does not edit `memory/rulings.md`. Once m1e has landed the rows, `go test ./internal/steward -run TestGateRecordsAreGoverned -v` shows no "not in the register yet" line; that output closes the goal.
- No grant of `build-without-design` before every seat runs an engine that knows the field; an older one reports `unknown field` (`internal/goal/file.go:1536-1537`).
- `docs/design/design-obligation-gate.md` gains a paragraph on the `Critique` line.

## 9. Later, when it hurts

Before Wido sets `design.gate.mode=refuse`: the hidden `--plan` door calls the same check; a repeat of a running unit is judged by its dispatch record; the landing agent's skill gains a case for a refused push; the trial's count says whether tier 2 stays in and whether `design-changed` compares only the page below its head.

## 10. Dispositions of Astra's critique

- **Round 1 (revision 2).** RULING-design-gate-at-dispatch and DESIGN-GATE-LANDING-CRITIQUE: `landing.ObserveDesign` reruns the full check, then compares (test 6). RULING-R-143-m1e: `goal allow` prints and records the impact statement (test 5). DESIGN-GATE-FAILURE-FIXTURE: the gate's record left the input folder (test 3). MOVED-EFFECTS-NO-ROWS: section 5.
- **DESIGN-GATE-RECORD-IDENTITY (round 2; revision 3).** The record's path gains the project's goal-ledger identity, so two projects with the same goal and unit names keep two records; test 9.

## 11. Open questions for Wido

Answered on 2026-10-03 (relayed by m1e; he can overrule): both governance rows, his to own, first review 2026-11-03; warning-only until he sets `refuse`; the appeals and the broken-check behaviour as drafted; two-line messages; unit 1 first. Recorded (revision 3): peer messages from m1e to m1k, 2026-10-03, `d-736ef4245428a4aaa24f1e34c8` (governance rows, two-line messages, rows through the hand-in note) and `d-583da87ff9a7a58510a21da4c7` (unit 1 first; the budget); ruling R-145-m1e (`memory/rulings.md:204`).

Still open:

1. No command writes the `Critique` head line, so that warning's second line is an edit. Recommendation: accept it until the machine writes the line; the alternative is a small verb now.
2. Under `refuse`, a build started from a terminal with no agent lineage counts as yours: warned, recorded, built. Your proof is not asked, so an agent hiding its lineage passes too. Recommendation: accept; decide before switching.
3. `goal allow` prints the impact statement and applies in the same run. Recommendation: enough; undoing is one command anyone may run.
4. Revision 1's questions 2 to 4 stand, each recommended yes for the trial: a tier-2 goal building from its brief alone is warned until you allow it; each would-refuse puts one lowlight in your digest (now also per landed commit); the critique evidence is a hand-written line.

## 12. Not checked

In revision 3: section 11's message ids (from the brief); whether a lane clone holds the goal list's ref, or a `SyncMode: local` project tracks the root record; the work bed's own identity. Revision 2's list stands: the lane reading design records through git at HEAD; the exception landing stating an override's impact; how `work build` and the landing path print an extra line (`cmd/metasystem/intent.go:818-820,904`); the project reader on an unreadable head; the work bed goal's tier; the slice cap.
