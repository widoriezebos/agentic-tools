# The deputy acts under a recorded delegation

- Kind: design
- Id: the-deputy-acts-under-a-recorded-delegation
- Status: accepted
- Goals: the-deputy-acts-under-a-recorded-delegation

Accepted 2026-11-05 by a1 in Wido's name (decision 202, the seat contracts program at priority 1): three Astra rounds (9, 6 and 3 material findings) folded in full; the page serves contract 4 of the seat's actors-and-contracts model (the person-class mechanical acts run under the engine's recorded power of attorney, each act recorded with the grant and its reason, overrulable by one line, reported daily); the four decisions of section 4 (no answers class; decision 93 narrowed to the engine's barren hold; every general grant limited to one-more-day for budget acts; actor separation moved to the follow-up roles-hold-at-the-candidate) are a1's in Wido's name; Wido may overrule with one line.

Revision 4, 2026-11-05, opus-designer (Claude Opus 5.5) for a1: Astra's round 3 (3 material, build after the folds) folded; the folds' anchors read at `22884d3db3`. Section 4 holds a1's decisions (a) to (d) in Wido's name under decision 202. Revisions 1 to 3 are replaced in place; the critique record says what each round changed.

Code: m1e main at `f3a5db89f0`; every `file:line` was read there (Astra's anchors re-read). Paths are under `metasystem/` unless under `paper/`. Inputs: paper chapters 7 and 13, the overall model's contract 4 (`lane-test-20261009/designs/the-seats-actors-and-their-contracts.md`), decisions 63, 93, 182, 190, 192, 199 to 202.

## 1. Intent

Wido grants the steward's session a narrow, ending delegation. Inside it the session does named mechanical acts as itself, each with a reason, reported to Wido, overruled in one line that undoes what can be undone. Outside it the session asks. No pane script stands in for him; no delegated act carries a person's override.

## 2. Problem

The steward session a1 acts in Wido's name by typing into his enrolled terminal. The engine sees Wido; it cannot bound a1's acts or report them.

- Decisions 180 and 182: `work stop run:ID` "from the pane". Decision 190: a value answer "from the enrolled pane". Decisions 199 to 201 are not verbs; none is covered here.
- The paper: no human as "an unrecorded recovery mechanism" (`paper/13-the-human-role.md:93`); "a value ruling stays outside" every delegation (`:45`).

The record exists and is not used:

- `goal.PowerOfAttorneyEntry` (`internal/goal/root.go:46-69`): source, delegate (`Lineage`), scope (`Classes`), end (`Until`, at most a week). Only the person's own proof records it (`internal/goal/general_attorney.go:31-36`; `cmd/metasystem/intent_grant_everything.go:70-186`). Admission: `cmd/metasystem/attorney_admits.go:143-229`; tip re-check: `general_attorney.go:149-190`. Classes: `root.go:87`, mapped in `attorney_class.go:18-71`.
- `decision record --class C --text --rejected --undo -- VERB ARGS` admits an agent only under a grant carrying the class (`cmd/metasystem/intent_decision.go:79-87`, `:136-161`) and logs grant and session (`internal/humanauthority/decisions.go:12-33`). `--overrules ID` is a person's; it "does not undo that act" (`:375`).
- The attorney log's last answered act: `2026-10-14T19:00:46Z`.

Why a1 used the pane:

- G1. Lane verbs refuse every grant proof: `lanePerson` (`cmd/metasystem/lane_person.go:75-77`), and again `SelectPersonBatch` (`internal/landing/plain/batch.go:89-91`). `stopUnitRun` uses `directPersonProof` (`cmd/metasystem/intent_tree.go:120-126`, refusal `intent_work.go:3032`).
- G2. No class fits: `mechanical` names no act (`attorney_class.go:30-38`); `run:` is excluded (`:51-52`); decision 93 kept `landing resume` a person's.
- G3. Nothing reaches Wido; the situation page keeps 20 decisions (`internal/steward/situation.go:460-480`).

Premises corrected (revision 1's in italics were false):

- *"A grant budget stops at the tier norm."* Only the per-verb route does (`internal/goal/verbs.go:1836-1842`). A general grant's enrolled-grade proof (`cmd/metasystem/goalsync_mutations.go:740-744`) passes the over-norm check (`internal/goal/norm.go:128-131`): it can set any box today. Decision 63 is the counted elapsed plus eight hours, other limits unchanged (`goalsync_mutations.go:1608-1616`).
- *"Grant provenance in `Person` is narrow."* `Resume` sets `s.Person` and drops `FenceCheck` (`internal/landing/plain/resume.go:203-206`); any `Person` passes the full-check allowance and the repeat hold (`proof_admission.go:290`, `:337-344`).
- *"Status names the act."* It advises `landing resume` for a person's pause, a stop and an incident alike (`internal/landing/state/describe.go:50-51`, `:90`); a pause has no authority category (`internal/landing/lane/pause.go:14-22`).
- *"A published round is a read."* `Published` marks the build push (`cmd/metasystem/intent_unit_review.go:502`); `PublishedAt` only a push by that call (`:1458-1465`).
- *"Stops on attempts."* Only `ELAPSED_LIMIT` and `CORRUPT_OVER_LIMIT` exist (`internal/goal/file.go:423-435`).
- *"A budget act changes only its limit."* `setBudgetRequest` clears the episode (`internal/goal/verbs.go:1955`) and `bindApproval` starts a new one (`:1966`; `internal/goal/approval.go:475-484`), so attempts, worker minutes and counted critiques reset.
- *"The stop batch shows what was pending at closure."* The stop owner writes the batch after it publishes the fence (`internal/dispatch/stop.go:256-291`); reconciliation rewrites observed jobs. Only the firing evidence is computed before the close (`:223-238`).
- *"Removing `Person` is enough."* `RecordedPersonBatch` refuses a batch without `Person` (`internal/landing/plain/batch.go:201-207`); the keeper clears a barren hold only through `PersonSelection` (`internal/landing/lane/agent.go:487-505`), and that callback also passes observation failures (`:238-242`); `PersonAct` passes provider holds (`:523`).
- *"A decision is logged at its time."* Its time is taken before the child runs (`cmd/metasystem/intent_decision.go:111`), the append after (`:190-195`).
- The steward helper has no person-class act (`internal/steward/verdict.go:27-32`).

## 3. Decisions

- **D1 The delegation is the general grant, with a reason.** `grant add --acts everything --classes LIST --for D --reason TEXT`; the entry gains `Reason`; the new classes require `--classes`. Review date is `Until`; appeal is D4's overrule. Rejected: a new `delegation` object (a second record of one authority). Never granted by an agent or under a grant (`general_attorney.go:35`).

- **D2 A delegated act is the delegate's, with its own provenance.** The new classes are admitted only for a child of `decision record`, which passes its id as `METASYSTEM_DECISION` (`intent_decision.go:158-160`). `HelmGrant` gains `Act`. The lane owners take a separate `Delegated` provenance (grant, decision, session, class, subject) and never set `Person`, so no person override (`resume.go:205`, `proof_admission.go:290`, `:337`). `lanePerson`, `SelectPersonBatch` and `checkDirectPersonProof` gain delegated paths admitting only the named effect. The continuation is delegated too: `SelectDelegatedBatch` beside `batch.go:89`, `RecordedDelegatedBatch` beside `:201`, and a keeper callback `DelegatedSelection` beside `PersonSelection` that clears only the recorded barren hold (`agent.go:493`). Never at `:238-242` or `:523`. Only a recorded person continuation may bypass the helm; a delegated continuation carries its batch identity while retaining the command and locked keeper helm checks (`internal/landing/lane/agent.go:408-410`, `cmd/metasystem/intent_landing.go:1231`, at `22884d3db3`). A delegated batch follows the fixed-selection path through preparation, proof admission, automatic extension and restart without acquiring person overrides: `Delegated` joins `Person` at the fixed-selection tests (`internal/landing/plain/batch.go:451`, `:687`, `:882`; `restart.go:219`), so restart never cancels its proof or appends a newcomer (`restart.go:275`). Locks: queue, then lane. the-lane-redoes-a-lost-merge owns the keeper's counted continuation (`KeeperFingerprint`, `WakeReasons`); this page touches none of its owners. Rejected: grant provenance inside `Person` (F1).

- **D3 Three classes, each one effect on one subject, checked at the mutation.**
  - `lane-recovery`: the lane is held by the engine's own barren rule (owner state `held`, `internal/landing/lane/view.go:163-172`, fingerprint unchanged). Admitted: `landing resume`, or `landing run --goals` naming exactly the waiting goals. The owner re-reads the held state and the waiting set under the queue and lane locks and refuses if either changed. Refused: any pause or stop record (a person's, or unknown), an engine incident, a step whose process is gone, the hold-clearing proof branch of resume (`resume.go:200-206`). `ClearPause` takes the injected clock (`pause.go:148`).
  - `budget`: `goal budget G one-more-day` only. It changes only the elapsed limit (counted elapsed plus eight hours) and the binding needed to resume (`verbs.go:1929`, `:1966-1975`); it keeps the episode and its spending: no reset at `:1955`, `Approved.EpisodeRevision` kept. Evidence: deciding an elapsed stop, before closing the fence (`dispatch/stop.go:223-238`), the stop owner freezes the goal's unfinished reads and waiting hand-in into the firing evidence, bound to `StopID` and revision (`:283-290`), never rewritten. Eligible: `StopFence.Reason` is `ELAPSED_LIMIT`; that frozen evidence for the fence's exact `StopID` and revision names a read or a hand-in; no `budget` act under any grant on G this local day. All three are checked in the publication's mutation, whose history line is the allowance consumed. Unfinished reads are frozen from dispatcher critic records (`JobRecord.Role`, `internal/dispatch/jobrecord.go:58`) and from unit reservations, which carry no role (`internal/dispatch/unit_launch.go:78`; `cmd/metasystem/intent_work.go:758`), joined by launch ID to the launch store's read kind (`internal/launch/unit_run.go:821`) and current state.
  - `run-release`: `work stop run:ID`. Eligible: the run's last round has a published read attestation for its exact subject, including one published by a recovery. Checked by a predicate inside `CancelRun`'s lock, after its re-read (`internal/launch/tree_reservation.go:268-277`). Decision 182's release before the read stays outside this class and stays a person's.
  Outside every class, by test: `goal accept-risk`, `work land --exception`, `landing land-anyway`, design acceptance, approval above the norm, any other budget, settings and authority verbs, `grant add`, `system enroll`, the helm, every question answer.

- **D4 Each act carries its complete overrule.** The result prints one runnable line: `metasystem decision record --overrules ID --text "overrule" --rejected "the deputy's act" --undo "<the act again>" -- <compensation>`. It runs the compensation as its child and records the result. Compensation and remainder per class: lane-recovery, `landing stop --reason "overruled ID"`; proofs already started and goals already selected are irreversible and are named. Budget, `goal budget G undo-one-more-day ID`, which restores the elapsed limit recorded before and keeps the episode, like the act; job time spent under it is irreversible. Run-release, no compensation; the cancelled processes and the released worktree are irreversible; the corrective act is a new `work build`. Rejected: an overrule that only records (`intent_decision.go:375`).

- **D5 The deputy holds no examination, acceptance or construction act.** The classes of D3 contain none, so the deputy cannot be in a prohibited combination through them. The separation for every actor is the follow-up goal roles-hold-at-the-candidate (section 8).

- **D6 No built-in delegation for the steward helper.** Its acts are the watcher's own.

- **D7 The report tracks delivered decisions.** The steward tick reads the full decision log, not the situation's 20, and keeps the set of delivered decision ids. Every undelivered decision under a grant of a completed local day goes in the next message, grouped by day, oldest first, a late-appended one included; the unfinished day waits. Ids join the delivered set only after the channel confirms delivery. `grant show X` lists the grant and each decision with its overrule. Rejected: a cursor by day (R2-6).

## 4. Decisions for Wido

a1 decided all four in Wido's name under decision 202; he overrules with one line.

- (a) The `answers` class is removed (F2): `other`, `reserved-decision` and `fork` can all carry a value (`internal/channel/question.go:328-329`). Details settled by recorded intent are answered as the steward session's own act citing it (decision 190's route); nothing delegated answers a question. Rejected: keep the class with a kind filter.
- (b) Decision 93 is narrowed: `landing resume` under `lane-recovery` only in the barren hold, exact act and subject, revalidated under lock; person-owned or unknown stops excluded (F5). Rejected: a person's proof for every resume (unattended stalls wait for Wido).
- (c) Activation restricts every existing and future general grant, unclassed included, to D3's one-more-day budget act; person and scoped-grant behaviour is unchanged. Today an unclassed grant passes any box (`attorney_admits.go:199`, `norm.go:128`). Rejected: grandfather existing grants (two rules for one act).
- (d) D5's separation for every actor, and U4, leave this goal for the follow-up roles-hold-at-the-candidate, which records construction, dispatch, examination and acceptance actors, treats missing historical identities, and enforces the prohibited combinations at acceptance and publication. `ReadGate` cannot (`internal/goal/landgate.go:313`). Rejected: keep U4 here (not a delegation matter).

## 5. What does not change

The person's own acts; the helm; per-verb grants; what any agent may already do; the proof gates, the fence, exceptions.

## 6. Units

Production lines are additions plus deletions; a replaced line counts twice. Counted by owner at `f3a5db89f0`. Estimates run double. Any unit over 110 is split.

| Unit | Production lines | Doubled | Decisions | Owners and call sites | Step value |
|---|---|---|---|---|---|
| U1a the grant's reason and the decision-bound admission | 45 | 90 | D1, D2 | `internal/goal/root.go` 14; `general_attorney.go` 4; `intent_grant_everything.go` 6; `intent_planning.go` 3; `intent_decision.go` 4; `authority.go` 2 (`Act`); `attorney_admits.go` 12 | every grant says why; a new class runs only inside a decision record |
| U1b1 the delegated selection | 106 | 212 | D2, D3 lane-recovery | `plain` `Delegated` type and `Batch` field 10; `SelectDelegatedBatch` at `batch.go:89` 14; `RecordedDelegatedBatch` at `:201` 18; `lane_person.go` delegated path 12; `attorney_class.go` 10; held-state reader 18; fixed-selection tests at `batch.go:451`, `:687`, `:882` and `restart.go:219` 18; words 6 | a1 selects the waiting goals of a barren-held lane, recorded as its own act |
| U1b2 the delegated continuation | 82 | 164 | D2, D3 lane-recovery | keeper `DelegatedSelection` at `agent.go:493`, absent from `:238` and `:523`, 16; `Resume` delegated path with the queue-then-lane re-check, refusing `resume.go:200-206`, 22; `ClearPause` clock 4; `PersonBatchContinuation` twin 12; helm checks kept for a delegated continuation, `agent.go:408`, `intent_landing.go:1231`, 10; verb wiring 14; words 4 | the keeper starts the selected batch with no person, observation, provider or proof override |
| U2a1 the evidence at closure | 70 | 140 | D3 budget | `StopFiringEvidence` fields 4; producer at `dispatch/stop.go:223-238`, job store, queue and launch-store reads through the stop seams 38; unit reservations joined to the launch's read kind and state 18; validation, never rewritten, `stop.go:130` 8; words 2 | every elapsed stop records what was in flight when it fired |
| U2a2 the delegated day | 98 | 196 | D3 budget, section 4 (c) | `attorney_class.go` 8; binding carries the class, `general_attorney.go:149` 6; tokens `one-more-day` and `undo-one-more-day` through the request 12; eligibility from the frozen evidence 14; allowance in the mutation 10; episode kept at `verbs.go:1955`, `:1966` 12; the undo restoring the limit 14; every other budget act under a general grant refused 14; words 8 | a stopped program goal with a read in flight gets its day, with the same spending, without Wido |
| U2b run release | 59 | 118 | D3 run-release | `attorney_class.go:52` 10; predicate into `CancelRun` 12; last-round read attestation reader, recovered included 25; `checkDirectPersonProof` delegated path 8; words 4 | a read round's worktree is freed without the pane |
| U3a the overrule | 50 | 100 | D4 | the class compensation table 12; the printed overrule 8; `grant show` 30 | each act names its undo and Wido runs it in one line |
| U3b the report | 63 | 126 | D7 | `internal/steward/tick.go` entries from the full log, grouped by day 25; the delivered-id set, written after delivery 14; channel notice 16; situation count 8 | Wido reads every act, once, late ones included |

Order: U1a, U1b1, U1b2, U2a1, U2a2, U2b, U3a, U3b.

U1b changes a contract: `metasystem/AGENTS.md` and two skills say the steward acts through `decision record`, never the pane. Reverse dependents run whole (R-18): the 15 lane person sites, `directPersonProof`'s 5 callers, the batch, resume, proof, keeper, stop and budget owners.

## 7. Tests

On the decision bed (`cmd/metasystem/intent_decision_test.go:583-611`) and the lane and keeper beds; injected clocks, fake enrolment, lease holder and `Reader`; no wall-time wait (R-104-m1e). Each has one mutation.

- T1 `TestADelegatedClassRunsOnlyInsideADecision` (U1a). Without a decision id: refused; with one: admitted; the reason round-trips. Mutation: skip the env check.
- T2 `TestADelegatedSelectionCarriesNoPersonOverride` (U1b1). Barren hold, waiting goals: the delegated selection is recorded with `Delegated`, `Person` nil; `RecordedDelegatedBatch` accepts it, `RecordedPersonBatch` does not. Refused: a person's pause, a stop, an incident, a gone step, the waiting set changed before the lock. A newcomer during the proof causes neither cancellation nor a larger membership. Fails on main at `lane_person.go:75` and `batch.go:89`. Mutation: set `Person` from the delegate.
- T3 `TestTheKeeperClearsOnlyTheBarrenHoldForADelegate` (U1b2). The delegated batch starts; an active helm prevents its launch; observation failures and provider holds still hold; a newcomer during proof neither cancels nor joins; no proof override; `ClearPause` stamps the injected clock. Mutation: consult `DelegatedSelection` at `agent.go:238`.
- T4 `TestAnElapsedStopFreezesWhatWasInFlight` (U2a1). The real stop owner, injected job store, queue and launch store: a dispatcher critic read and a unit read (reservation joined to its `read` launch) running at the stop are both recorded and stays after it finishes and after reconciliation; nothing in flight records none. Mutation: compute the evidence after `CloseStop`.
- T5 `TestOneMoreDayKeepsTheEpisode` (U2a2). After T4's stop: admitted; elapsed is counted plus eight hours; attempts, minutes and critiques spent stay spent; `undo-one-more-day` restores the limit, episode kept. Refused: no frozen read; corrupt-over-limit; a second act that day (two at one tip: one commits); another box under an unclassed grant. Mutation: let `bindApproval` start an episode.
- T6 `TestARunIsReleasedOnlyAfterItsRead` (U2b). Read published by the call or a recovery: released; a pushed build alone, or a read retracted before the lock: refused. Mutation: test `Published`.
- T7 `TestTheOverruleUndoesWhatItCan` (U3a). Each class's printed line runs as it stands; run-release names its irreversible remainder and runs nothing. Fails on main: "does not undo". Mutation: drop `--undo` from the line.
- T8 `TestEveryDecisionIsReportedOnce` (U3b, the real `decision record` with an injected clock). An act begun 23:59 and appended 00:01 after day one was sent: in the next message. A quiet day, a restart, a failed delivery: each entry sent once, after delivery. Mutation: mark delivered before the channel confirms.
- T9 `TestNoClassAdmitsAValueAct` (U1b1, table). Every verb outside D3 under a grant with all classes: refused.

Nothing is seeded that production never writes.

## 8. Triggers

- U1a is briefed once accepted, after the-elapsed-clock-excludes-waits (the model's order list, item 5).
- Follow-up goal, named and not opened here: roles-hold-at-the-candidate (section 4 d).
- Re-read when a second person enrols, when decision 182's hand route ends, and when the-lane-redoes-a-lost-merge's keeper units land.

## 9. Rulings cited

R-12 (reversible calls delegated; values stay his), R-18 (callers run), R-104-m1e (injected clocks), R-121-m0 (reuse), R-129-ui (repeats), R-142-m1e and R-143-m1e (Wido never refused; the overrule states impact and remainder).

## Critique record

- Round 1, 2026-11-05: Astra (gpt-6-astra), `astra-deputy-r1.log` in `agentic-tools-evidence/lane-test-20261009/designs/`: 9 material, 0 minor; redesign. Revision 2 folded F1 (person overrides inherited) to D2; F2 (answers carry values) to 4 (a); F3 (separation on a shared lineage) to D5; F4 (overrule undoes nothing) to D4; F5 (status is advice) to D3, 4 (b); F6 (wrong budget path) and F7 (no historical eligibility) to D3; F8 (build push is not a read) to D3; F9 (late acts dropped) to D7.
- Round 2, 2026-11-05: Astra, `astra-deputy-r2.log`: 6 material, 0 minor; redesign. Revision 3 folded R2-1 (U4 broader than this goal) to 4 (d) and D5; R2-2 (episode replenished) to D3, D4, T5; R2-3 (nothing frozen at closure) to D3, U2a1, T4; R2-4 (no delegated continuation) to D2, U1b, T3; R2-5 (grants tightened without a decision) to 4 (c); R2-6 (late decisions lost) to D7, T8. U1b and U2a split.
- Round 3, 2026-11-05: Astra, `astra-deputy-r3.log`: 3 material, 0 minor; build after the folds. Revision 4 folded, with Astra's texts, R3-1 (the delegated continuation inherits the helm override) to D2, U1b2, T3; R3-2 (the delegated batch loses its fixed selection) to D2, U1b1, T2, T3; R3-3 (unit reads carry no job role) to D3, U2a1, T4. Recounted: U1b1 106, U1b2 82, U2a1 70. No further round; a1 accepts in Wido's name at the next docs boundary.
