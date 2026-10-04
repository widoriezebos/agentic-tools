# Design for goal-worktree-dispatch-uses-the-primary

- Kind: design
- Id: 01M42VWZ7JQMPT8QRAV68B5AHA
- Status: accepted
- Goals: goal-worktree-dispatch-uses-the-primary
- Critique: closed at round 4 on 0 material findings (Codex Astra, design-critic-27f014429cd290f06ce7fc74)

**Scope.** This page covers six of the goal record's seven faces (the engine, the runtime registry, the settings, the engine skew check, the frozen brief at review, the handoff continuation) and the unit the record added on 2026-10-04, the seat-start prompt. The seventh face, the Codex sandbox denials, belongs to goal `codex-jobs-run-unsandboxed-on-a-trusted-host`.

**The decision:** a goal worktree is the tree under review, never the tool. One function says which installation serves a dispatch root, and the dispatch takes its engine, its runtime registry and every setting from there. For an unarmed goal worktree that is the same installation in the seat's primary checkout. Every other root serves itself.

## 1. What exists today

- `landpath.SystemInstallation` already says which installation's running system serves a root. Four fixes applied it one reader at a time.
- Everything else is read at the dispatch root (the From column of Moved effects). A goal worktree has no `metasystem.conf.local`, and no `bin/metasystem` or a stale build of its own branch.
- The skew check (`engineSkewPreflight`) lists commits up to the dispatch root's HEAD, so a goal's own engine commits count.

## 2. One resolver

`internal/dispatch` gains `ResolveTool(root, serving, lookupEnv)`. It returns the serving installation, that installation's checkout, and the engine. `serving` is `landpath.SystemInstallation` bound to the caller's Git seam, a parameter because `landpath` already depends on `dispatch`. Its cases:

- A primary checkout, a linked worktree with its own arming record or census, and a root Git cannot map each serve themselves.
- An unarmed linked worktree is served by the same installation folder in its primary checkout.

**Engine.** `METASYSTEM_BIN` when set, else `bin/metasystem` in the serving installation. The three engine defaults and the launch supervisor's program (Moved effects) take it from the resolver. `METASYSTEM_BIN` stays the one explicit override, for any root; tests and deliberate branch-build checks use it.

**Registry and settings.** Both come from the serving installation's disk: its `adapters` folder, its checked-out `metasystem.conf` and its `metasystem.conf.local`. Flags and environment still outrank files. A goal branch's change to `metasystem.conf` is the subject under review: no dispatch reads it until it has landed.

The readers that move are rows of Moved effects. `criticDelegateEnvironment` is deleted: two owners of the same keys could disagree.

These stay on the root, as the tree under review: the reviewed commit and its diff, the worktree the delegate edits, the tree paths a brief cites, and the job's own records (`branch.CriticStore` is unchanged).

**Any adopter.** The rule names no folder, build command or engine source. A Java adopter without Go has an installed, uncommitted `bin/metasystem`, which its goal worktrees lack; the resolver finds it through Git's worktree mapping alone.

## 3. The skew check

The check follows the goal record's rule: the engine is compared with main-side commits only. In the dispatch root's repository it lists `git log --ancestry-path STAMP..BASE`, BASE being `git merge-base HEAD refs/heads/main`. The goal's own units come after BASE and are never listed. At a primary on main, BASE is HEAD: today's check.

Main is the local branch, not the landing endpoint's. The engine is built from the primary's work tree, so its stamp is a commit of that branch; the endpoint's main is read only by a fetch (`branch.EndpointTip`), which this check should not need. The name is the machinery's own: goal branches land only on `refs/heads/main` (`branch.MainEndpoint`). A local main behind the branch's fork point lists fewer commits, never the goal's.

No merge-base (no local main, unrelated histories, a shallow clone) lists nothing and passes: no rebuild would repair that refusal. A stamp that is no ancestor of BASE lists nothing, and a stamp Git does not know fails the command; both pass, as today, so an installation without engine sources is never refused.

## 4. The brief at review

A build brief is checked once, when the build is dispatched. The unit's record keeps its path and digest (`BuildBrief`).

At review, the composer (`branchReadBriefWithRepository`) quotes a brief under one of two headings:

- "Supplied accepted implementation brief (frozen at dispatch)", when the quoted bytes have the digest the build recorded. When the dispatch names a reviewed commit, the brief check skips this section to the end of the brief: the build admitted those bytes.
- "Corrected implementation brief (given at review)", when they differ or no build was recorded. It is checked in full against the reviewed commit and the dispatcher's HEAD: nothing has admitted it.

`work review G --work W` gains `--brief FILE`: FILE replaces the recorded build brief for this read only. The unit and its build brief stay, and no builder round runs. The read's record keeps FILE's path and digest. The `--commit` form follows the same digest rule.

A request with `--brief FILE` never joins; today's `--join` drops a supplied brief once a critic exists.

- No critic yet, or a refused start, which leaves a retryable record with no critic (face 5): FILE starts the read, by the existing restart rule.
- A critic examines or examined the unit with another brief: refused with the existing `ReadBriefChangedCode`, naming that examination. An old examination never covers a correction.

## 5. The continuation

A runtime path is one under `artifacts/`, at the top or in the installation folder, or one Git ignores. The brief check looks for it in the dispatch root's work tree, and then in the serving installation's checkout. Both are the seat's own state: the brief's inputs, not the tree under review. So a continuation dispatched from a goal worktree may cite the handoff's state file under the primary's `artifacts`. `artifactAuthorityPath` today spells the installation folder as `metasystem/`; it takes the installation prefix instead.

A brief-authority refusal names each missing path with its class and where it was looked for: a tree path (the reviewed commit and HEAD) or a runtime path (the two work trees).

The steward checks the continuation brief before it consumes the handoff: a new seam `admit`, supplied by the command that supplies `launch`, runs the same check with the roots the launch will use. On a refusal the steward holds the handoff (`holdHandoff`): the notice carries the path and its class, nothing is consumed, and the next tick tries again.

## 6. The seat-start prompt

`seatBrief` writes the prompt from the goal id and `Held` alone. A new `seatFacts` (in `seat_start.go`) adds what the goal's record says; a coordinator's briefing is no input.

- **Next step:** `goal.GoalFacts.NextStep`, from the projection the seat ladder reads.
- **Units with their stage** (built, under read, read, review refused): what `metasystem status G` prints. The steward cannot import that loop (`goal/branch` depends on the steward), so it becomes `goalUnitStages` (new, in `intent_selection.go`), called by the status verb and by a new seam `TickConfig.Units`.
- **Hand-in:** `plain.Latest` passed through `plain.Landed` against the main the start reads, as `laneQueueState` does: handed in, landed, or returned with the return text. Every unit carries that state when the hand-in's commit is the branch tip the start reads (`Tips`); otherwise one line names its commit.
- **Open refusals**, three sources, no new store:
  - *A refused job:* the goal's job records under `artifacts/agents/jobs/` that ended `dispatch-refused`, each open while no later record has its role and reviewed commit. A line holds job id, role, refusal class and summary.
  - *A review refused before reservation* (brief admission runs first) leaves no job record. The branch read record gains `DispatchRefusal`, the refusal's first line, saved with `DispatchRetryable` and cleared by a restart or a recorded critic. `goalUnitStages` carries it: the unit is listed as review refused, with the line and `metasystem work review G --work W`.
  - *A refused launch* (`launch.Store.Refusals`) has no job id: a line holds time, kind, code and tag. It is open until a launch record (`Store.List`) of its goal and kind starts after it.
- **Messages:** each counterpart's newest three, counterparts ordered by their newest message. The goal asks for the last three with the coordinator, but no code names a coordinator seat (`settings coordinator` is per checkout, and the coordinator's has none); grouped, another seat's newer messages cannot displace the coordinator's. Source: the threads (`board.Threads`, what `agent inbox --all` reads) holding one to this seat, to the goal, or from this seat, replies included. A message's counterpart is its sender (`From.Machine`); for one this seat sent, the seat addressed (`To.Machine`), or the thread's newest other sender when it addressed a goal, else the goal's name. `board.Render` prints each. A cut names the counterparts and messages left out and `metasystem agent inbox`.

**Bound:** 12,000 bytes: 2,000 for the next step, 600 for the return text and each message, 20 units, 10 refusals, newest first. A cut names the count left out and the command that shows it.

**A failing reader** costs its fact only: the prompt names it as unavailable with the command that shows it (`goal show G`, `work status G`, `landing status`, `agent inbox`), and the seat starts.

The unit's last clause, the continuation's brief path (face 6), is section 5.

## Moved effects

| Effect | From | To | Code |
| --- | --- | --- | --- |
| Lifecycle's engine default | `delegationEngine`: ROOT's | `dispatch.ResolveTool` (new, in `fspath.go`) | `metasystem/cmd/metasystem/delegate.go`, `metasystem/internal/dispatch/fspath.go` |
| Owner ports' engine default | `delegation.NewOwnerPorts`: ROOT's | `ResolveTool` | `metasystem/internal/delegation/owners.go` |
| Adapter supervisor's engine default | `supervisor.ProcessDeps`: ROOT's | `ResolveTool` | `metasystem/internal/adapter/supervisor/deps.go` |
| Launch supervisor's program | `OSSupervisorStarter`: the running program | the resolver's engine, as `Executable` | `metasystem/internal/launch/process.go`, `metasystem/cmd/metasystem/launch_verbs.go` |
| Critic's roster | `criticDelegateEnvironment`, carried as environment | deleted; the session reads the roster itself | `metasystem/cmd/metasystem/goal_branch.go`, `metasystem/internal/delegation/dispatch_phase.go` |
| Session's settings | `s.root` joined to `metasystem.conf` | the serving installation's file | `metasystem/internal/delegation/lifecycle.go`, `metasystem/internal/delegation/admission.go`, `metasystem/internal/delegation/claim.go`, `metasystem/internal/delegation/dispatch_phase.go`, `metasystem/internal/delegation/followup.go`, `metasystem/internal/delegation/helpers.go` |
| Supervisor's registry and settings | `registryEntry` and `Deps.configValue`, at `d.Root` | the serving installation, through `ProcessDeps` | `metasystem/internal/adapter/supervisor/runtime.go`, `metasystem/internal/adapter/supervisor/identity.go`, `metasystem/internal/adapter/supervisor/fake_host.go`, `metasystem/internal/adapter/supervisor/deps.go` |
| Unit launcher's settings | `intentConfPath`: the layout's installation | the serving installation's file | `metasystem/cmd/metasystem/intent_work.go` |
| Landing gate's settings | `landingGateSettings`: root's file | the serving installation's file | `metasystem/cmd/metasystem/landing_gate.go` |
| Build brief's admission | every review, again (`session.briefAuthority`) | the build's dispatch, once; review skips the frozen section | `metasystem/internal/delegation/admission.go`, `metasystem/internal/dispatch/brief.go`, `metasystem/internal/goal/branch/read.go`, `metasystem/cmd/metasystem/intent_unit_review.go` |
| Continuation brief's check | the launch, after `ConsumeIntent` | `admit` (new seam, in `revive.go`), before it | `metasystem/internal/steward/revive.go`, `metasystem/cmd/metasystem/steward_verbs.go` |
| Unit stages | the loop in `runIntentStatusGoal` | `goalUnitStages` (new, in `intent_selection.go`), behind `TickConfig.Units` too | `metasystem/cmd/metasystem/intent_selection.go`, `metasystem/cmd/metasystem/steward_seat.go`, `metasystem/internal/steward/tick.go`, `metasystem/internal/steward/seat_start.go` |
| Review's refusal before reservation | the command's output only | the read record too (`DispatchRefusal`, new) | `metasystem/internal/goal/branch/read.go` |

## 7. Units, in landing order

Tests stub Git (`GitOps`, the `serving` parameter) and processes (the existing ports); none runs real Git.

1. **Resolver and engine.** Witness `TestResolveToolTakesTheEngineFromTheServingInstallation` (`internal/dispatch`), a table of the cases plus `METASYSTEM_BIN`. Mutation: join the engine path to root. Witness `TestAnUnarmedGoalWorktreeLaunchesItsPrimarysEngine` (`internal/delegation`). Mutation: restore the default in `NewOwnerPorts`.
2. **Registry and settings.** Witness `TestAGoalWorktreeSessionReadsItsSettingsFromThePrimary`: the worktree's `metasystem.conf` holds a duplicate key, which fails any reader that opens it; the dispatch runs on the primary's roster. Mutation: point any reader, in the table or not, back at root. Witness `TestTheSupervisorLoadsTheServingInstallationsRegistry`: only the primary holds the adapter. Mutation: `external.Load(root)`. Witness `TestTheLandingGateInAGoalWorktreeReadsThePrimarysHumanFromTier`. Mutation: `landingGateSettings` joins root.
3. **Skew check.** Witness `TestSkewPreflightListsOnlyMainSideCommits` (`internal/delegation`): engine commits after the merge-base pass, one before it refuses, no merge-base passes. Mutation: list `STAMP..HEAD`.
4. **Brief at review.** Witness `TestBriefAuthoritySkipsTheBriefTheBuildAdmitted` (`internal/dispatch`). Mutation: scan the frozen section. Witness `TestReviewOfABuiltUnitTakesACorrectedBrief` (`cmd/metasystem`): no builder round is recorded, and a missing path in FILE refuses. Mutation: quote FILE under the frozen heading. Witness `TestReviewWithABriefNeverJoins` (`internal/goal/branch`): a refused start with no critic takes FILE; an examined unit refuses, naming the examination. Mutation: pass `--join` with FILE.
5. **Continuation.** Witness `TestBriefAuthorityAdmitsARuntimePathOfTheServingInstallation`. Mutation: look only under the dispatch root. Witness `TestARefusedContinuationBriefHoldsTheHandoff` (`internal/steward`). Mutation: call `admit` after `ConsumeIntent`.
6. **Seat-start prompt.** Witness `TestTheSeatPromptIsWrittenFromTheGoalsRecord` (`internal/steward`), readers stubbed: the prompt holds each fact; three messages of one seat, one seat-addressed, then three newer of another are all six kept; a waiting hand-in whose commit main contains reads landed; a refused-then-launched pair lists nothing; a review refused before reservation shows its line and command; an oversized next step is cut within the bound; a failing reader names its fact and command while the seat starts. Mutations: keep only goal-addressed messages; one limit of three across every counterpart; print `Latest` underived; list every row of `Refusals`; fail the start on a reader's error; drop the cut. Witness `TestAReviewRefusedBeforeReservationKeepsItsFirstLine` (`internal/goal/branch`): a restart clears it. Mutation: save no line. Witness `TestStatusAndTheSeatPromptShareOneUnitList` (`cmd/metasystem`). Mutation: compose the stages again in the seam.

Units 2 and 5 need unit 1. Units 3, 4 and 6 stand alone.

## 8. Open question

When a seat types the worktree's own `bin/metasystem`, the admission checks run in that branch build; only what the dispatch launches is the primary's engine. Should that be refused, naming the engine to run, unless `METASYSTEM_BIN` names the branch build? I recommend yes: a goal's own build should not decide its own review. It is Wido's call; unit 1 leaves it unbuilt until he answers.

## 9. Deferred and not checked

- Verbs that are not dispatches (the proof command, goal sync) keep their root's settings until a face shows the need.
- Not checked: the refused continuation's dispatch root (unit 5 reproduces it first); whether the roster bridge's selected installation can differ from the serving one; whether every unit form records `BuildBrief` with a digest; whether `WaitDeliveryRuntime` loads the registry in a dispatch; whether a refused job record keeps its reviewed commit; which seam gives the start main's tip; whether status reads the read record; whether a refusal's `Time` orders against `StartedAt`; what a retryable record with no line prints.

## Dispositions (critique design-critic-27f014429cd290f06ce7fc74)

Written by metasystem design review when critique design-critic-27f014429cd290f06ce7fc74 closed: every answered round's decisions, as the author made them.

| Round | Finding id | Finding | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- | --- | --- |
| 1 | RULING-goal-worktree-dispatch-uses-the-primary | The proposed skew check contradicts the goal's recorded rule: "the check compares the engine against main-side commits only (merge-base of the branch with main), never against the goal's own units". Checking the primary checkout's current tip instead introduces an additional refusal that the design itself acknowledges. This changes admission behavior and fails the authority test: the design substitutes a stricter rule without a recorded human decision. | accepted | The goal record states the rule: compare the engine against main-side commits only, the merge-base of the branch with main. The draft chose the primary checkout's HEAD instead and called it stricter, which adds a refusal no person decided. | Section 3 lists STAMP..merge-base(HEAD, main) in the dispatch root's repository, so the goal's own units are never listed; it states which main ref is read and what happens when there is no merge-base or the stamp is unknown; the stricter-case paragraph goes. |
| 1 | BRIEF-CORRECTION-EXISTING-REVIEW | The corrected-brief contract does not define what happens when this unit already has a review. The existing join path silently ignores new brief bytes; removing the join instead refuses them. An implementer must choose that outcome, and must prevent an old examination from appearing to cover the correction. This changes control flow and tests; without it, the first correction of an already-reviewed unit can produce a silent false answer. | accepted | Checked: read.go:560-563 drops the supplied brief when --join meets a started critic, and lines 577-579 refuse a changed brief otherwise. The draft did not say which applies to a corrected brief. | Section 4 states the outcome: work review G --work W --brief FILE never joins. When the unit's read has no critic yet, or its start was refused (the BRIEF_AUTHORITY_REFUSED case of face 5), FILE starts the read. When a critic already examines or examined the unit with another brief, the request is refused loudly with ReadBriefChangedCode, naming the examination. An old examination never covers a correction. The unit-4 witness gains both rows. |
| 1 | MOVED-EFFECTS-MISSING-INVENTORY | The design moves responsibilities without the required Moved effects table. It centralizes engine selection, deletes the roster-carriage owner, transfers brief admission responsibility, and moves continuation refusal ahead of handoff consumption. The required ownership check therefore remains incomplete: an implementer lacks a checked inventory showing which existing effects each replacement must preserve. This changes the migration contract and prevents establishing that step 1 safely retains those effects. | accepted | The page moves owners: the engine default (three places), the roster carriage, the settings readers, the brief admission of a frozen build brief, and the continuation check before consumption. The page has no Moved effects table, and the check-only run reported none. | Add a Moved effects section whose table names each moved effect with From, To and Code columns, as the accepted seat-path page does, and check it with design review --check-only. |
| 2 | RULING-goal-worktree-dispatch-uses-the-primary | The new message filter contradicts the goal's requirement to provide “the last three messages between the seat and the coordinator.” Selecting only messages addressed to the goal or belonging to its threads drops an ordinary direct conversation between those participants. This changes the selection contract and its fixture. Without correction, the new seat-start unit fails its first use when the coordinator addressed the seat directly. | accepted | The record asks for the last three messages between the seat and the coordinator; a filter on goal-addressed messages and goal threads drops a message the coordinator sends to the seat directly, the commonest case on this host (every coordinator message to m1g today was seat-addressed). | The prompt takes the newest three messages to or from this seat, seat-addressed or goal-addressed, with their thread replies, from the board mailbox the inbox reads; the witness has a direct seat message. |
| 2 | SEAT-HANDIN-LANDED-STATE | The proposed hand-in reader cannot report successful landing. Latest returns the recorded waiting state even after the commit reaches main, so a successor is told completed work is still handed in. The design must use the existing derived landing state and test an actual waiting-to-landed transition. This changes the reader and test; without it, the new prompt answers incorrectly after an ordinary successful landing. | accepted | Checked: plain.Latest returns the queue line as recorded; laneQueueState derives landed through plain.Landed with ContainedIn against main. Latest alone reports waiting after a landing. | The hand-in fact is plain.Latest passed through plain.Landed against the main the start reads, as laneQueueState does; the witness moves one line from waiting to landed. |
| 2 | SEAT-REFUSALS-BEFORE-RESERVATION | The proposed refusal sources omit a review rejected before job reservation, including a corrected brief that cites a missing path. Such a refusal has neither the failed job record the design scans nor an entry in the separate launch-refusal store. The design must identify a durable source for this active refusal and its reason. This changes what is built; without it, a successor loses the blocking context in a failure explicitly covered by this goal. | accepted | Checked: the brief check runs before reservation, so a review refused for its brief leaves no job record and no launch refusal; the read record keeps only its retryable state. | The smallest durable source: when a review dispatch is refused before reservation, the goal branch read record keeps the refusal first line beside its retryable state, and the prompt lists that unit as refused with that line and the command that repeats the review. No new store. |
| 2 | SEAT-LAUNCH-REFUSAL-CLOSURE | The design lists historical launch refusals as open without defining when they close. After an operator fixes a refused launch and successfully retries it, the old refusal remains in the selected history. The design explicitly leaves this decision unchecked. A closure rule and a refused-then-successful fixture change the prompt's result; without them, the first successor after a normal repair receives a false outstanding blocker. | accepted | Checked: launch refusals are appended as history and Refusals returns every row, so a repaired and relaunched start still reads as open. | A launch refusal is open until a later launch record of the same goal and kind exists; the witness has a refused-then-launched pair that lists nothing, and the not-checked line about closure goes. |
| 3 | RULING-goal-worktree-dispatch-uses-the-primary | The message selection still does not guarantee the goal's required “last three messages between the seat and the coordinator.” It now takes the newest three across all qualifying seat conversations. Three newer messages from other seats therefore displace all coordinator messages. Select the coordinator conversation before applying the limit, and cover newer peer traffic in the fixture. This changes the selection contract and test; without it, the first successor using an existing mixed mailbox can still lose the required coordinator context. | accepted | Checked: the selection takes the newest three across every conversation, so three newer messages from another seat displace the coordinator's. No code names a coordinator seat on this host: `settings coordinator` is a per-checkout declaration, and m1e's checkout has none, so a rule that looks the coordinator up would find nobody today. | Group by counterpart instead of naming the coordinator: the prompt keeps the newest three messages of each seat that exchanged messages with this seat or about its goal (either direction, seat- or goal-addressed, replies included), counterparts ordered by their newest message, within the prompt's bound; a cut names the counterparts and messages left out and `metasystem agent inbox`. The witness has three coordinator messages followed by three newer messages from another seat, and all six are kept. Mutation: one limit of three across every counterpart. |
