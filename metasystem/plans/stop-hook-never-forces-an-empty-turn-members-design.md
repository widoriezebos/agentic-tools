# stop-hook-never-forces-an-empty-turn: members design

Revision 1, 2026-09-19. Written against the tree at 4e4e46de7.
The umbrella page is `plans/stop-hook-never-forces-an-empty-turn-design.md`. This page does not restate or change it.
All paths are relative to `metasystem/`.

Wido accepted on 2026-09-12 that the umbrella is designed and built per member.
Member 1, stop-infrastructure-allows-the-seat-to-stop, is concluded (c8074077). Its typed classification is the base for everything below and is not redesigned.
Members 3 to 7 each get one section, in landing order. A section is the whole brief for one Codex job: the builder gets the section, the workspace and the return shape.
Member 8 is an observation and has a short section at the end.

These rules bind every section. Each section repeats the ones its builder is most likely to trip over.

- A test never depends on wall-clock time. Use injected clocks and fakes. Never `t.Skip`, never a raised bound, never a retry. Process fixtures use `internal/testutil` with readiness lines, never a sleep.
- One mechanism per member. It has its own DONE and lands alone.
- The engine never names a runtime. Claude and Codex specifics stay in `scripts/agents/supervision-hook.sh` and `scripts/agents/adapters`.
- Read rules on append-only records are never tightened. Older records and older log lines still read.
- A test name that `testing.json` lists is never renamed or dropped. If a member changes behaviour such a test asserts, the body adapts and the name stays.
- Source comments say what the code does and why, in plain English. They never carry a round, finding, seat or goal id.
- Every builder returns the output of `scripts/agents/go-gate.sh --fast` with the diff.

## Member 2 is parked

Member 2, stop-decisions-record-deadline-evidence, was parked and decoupled by Wido on 2026-09-19. No member on this page depends on it landing. Where the umbrella assumes the version-2 incident record, each member reads what the tree holds today: the refusal record that `StopRefusal` writes (`internal/report/stopblock.go`, schemaVersion 1, one file per session under `artifacts/agents/supervision/stop-refusals/`), the lines of `artifacts/agents/supervision/hooks.log`, and the per-session verdict state in `artifacts/agents/turn-verdict-state.json`. The fixtures TestInfrastructureDeadlineIdentity, TestStopDecisionPersistsBeforeEmission and TestStopDeadlineCompletionIsSingleUse stay with member 2 and are not built here. TestArmingDetailSurvivesStop already exists in `cmd/metasystem/up_test.go` and is left alone. Each section below says in one sentence what changes if member 2 lands later.

## Member 3: stop-incidents-reach-the-steward

One mechanism: the steward tick drains stop incidents into its notification queue and records each delivery in the hook log.
Since member 1 an infrastructure failure allows the stop and appends a `stop-condition` line to the hook log. Nothing reads those lines, so the steward never hears about them.
If member 2 lands later, the version-2 record becomes the first source the drain reads and the hook log the second; the identity, the queue and the acknowledgement line stay as built here.

### 1. DONE

1. Every steward tick drains retained stop incidents before any narrator work, and a narrator failure does not stop the drain. Without it TestStopIncidentDrainBeforeNarrator fails.
2. The drain reads the hook log's unacknowledged infrastructure `stop-condition` lines and queues one notification per incident identity. Without it TestStopIncidentDrainFromHookLog fails.
3. A delivered incident is acknowledged by a line appended to the hook log and is never queued again. A failed delivery keeps its identity and is retried. Without it TestStopIncidentDrainFromHookLog and TestStopIncidentDeliveryUnconfirmed fail.
4. When the hook log is unreadable the drain reads the refusal records. Only when both are unreadable is delivery reported as unconfirmed, by the tick and by the stop notice. It is never a block. Without it TestStopIncidentDeliveryUnconfirmed and FakeReplayUnreadableHookState fail.

### 2. Mechanism

A new leaf package `internal/stopincident` owns the formats. It imports no engine package, because `internal/report` already imports `internal/steward` and both need these readers.
It holds:

- `Incident` with fields Class, Cause, Component, Generation, DeadlineEnd, Outcome and Source (`hook-log` or `refusal-record`).
- `Identity(i Incident) string`: the first 16 hex characters of a SHA-256 over class, cause, component, generation and deadline end. Outcome is left out, so a retry inside one deadline keeps one identity.
- `ReadHookLog(path string) (incidents []Incident, acked map[string]bool, err error)`. It parses the line `append_stop_condition` writes in `scripts/agents/supervision-hook.sh`:

```text
stop-condition <class> <cause> <component> <generation|-> <deadline_end epoch> <outcome>
```

  Only class `infrastructure` is an incident. Any line it cannot parse is skipped and the read continues.
- Two new line kinds, written by `AppendAck(path, identity string, deliveredAt time.Time)` and `AppendDrainStart(path string, at time.Time)`:

```text
stop-incident-ack <identity> <delivered epoch>
stop-incident-drain-start <epoch>
```

- `ReadRefusalRecords(dir string) ([]Incident, error)`: a read-only view of the version-1 refusal record (`schemaVersion`, `sessionId`, `causes` with `cause`, `count`, `firstAt`, `lastAt`, and the new optional `class`). The identity of such an incident is hashed over session id, cause and `firstAt`, because the record carries no generation and no deadline end.

`StopRefusal` in `internal/report/stopblock.go` already receives the class and drops it. `stopRefusalCause` gains `Class string` with `json:"class,omitempty"` and `StopRefusal` stores it. An entry without a class is history: it still reads, and it is never queued.

`DrainStopIncidents(repoRoot string, now time.Time) StopIncidentDrain` is new in `internal/steward/stopincident.go`. In order it does this:

1. Read the hook log. If the log has no `stop-incident-drain-start` line, append one and queue nothing on this tick. Lines above the first start line were written before any drain existed and are history. This stops a burst of old incidents on the first tick after the landing.
2. For each unacknowledged incident below the start line, call `QueueNotification` with nonce `stop-incident-<identity>`. The queue is already idempotent per nonce. At most 20 are queued per tick; the rest wait for the next tick.
3. If the hook log cannot be read, read the refusal records and queue their infrastructure entries the same way.
4. If neither can be read, return `Unconfirmed` with both errors.

`RunTick` in `internal/steward/tick.go` calls the drain as its first act after the arbitration lock and the self-identity step, before the sweeps and well before `NarrateDigest`. The drain needs only the root and `cfg.now()`. `TickResult` gains `StopIncidents StopIncidentDrain` (Queued, Pending, Unconfirmed, Detail). An unconfirmed drain is reported through the tick's existing health path as component `stop-incident-drain`. It never fails the tick.

Acknowledgement happens in `deliverPendingNotification` in `internal/steward/notify.go`. For a nonce that starts with `stop-incident-` the order becomes: deliver, mark the pending file delivered, append the ack line, then `MarkDelivered`. `PendingNotification` in `internal/steward/intervene.go` gains `DeliveredAt string` with `omitempty`. `DeliverPending` does not deliver such a file again; it only retries the ack append and the removal. So a hook log that cannot take the ack never causes a second delivery, and the drain never queues an identity that still has a pending file.

The stop notice. `append_stop_condition` already sets `hook_log_failure` when its append fails. `external_stop_json` passes a new flag `--hook-log-failed` to `report stop-block` in that case. When that flag is set and `StopRefusal` also fails to write, the system message `StopBlock` renders gains one sentence: "Delivery of this incident to the steward is unconfirmed: neither the hook log nor the refusal record could be written." The hook's shell fallback for a missing engine adds the same sentence when its own append failed. The decision stays allow in every case.

The tree holds no fake replay leg yet: a grep at 4e4e46de7 finds no test named `FakeReplay` and no `stop-replay` action. This member adds the action to `scripts/agents/adapters/fake.sh`. `stop-replay --root <bed> --session <id> --fault <none|hook-log|incident-state|both|engine>` applies the fault, runs the bed's installed Stop hook entry with the same launcher, arguments and standard input a provider would use, and prints the hook's response JSON. A fault is a file state: the log or the refusal directory is replaced by an unwritable path, or `METASYSTEM_BIN` points at a missing file.

Not changed: the verdict computation in `internal/goal`, the `stop-condition` line format, the order inside `DeliverPending` for every other nonce, the per-session verdict state (this member does not read it), and the rule that the first delivery failure ends a delivery pass.

One known limit. An incident drained from the refusal records during a hook-log outage has a different identity from its own log line. When the log reads again, that incident can be notified once more. The version-1 record cannot carry the coordinates that would join them; member 2's record can.

### 3. File set

Existing: `internal/steward/tick.go`, `internal/steward/notify.go`, `internal/steward/intervene.go`, `internal/steward/tick_test.go`, `internal/steward/notify_test.go`, `internal/report/stopblock.go`, `internal/report/stopblock_test.go`, `cmd/metasystem/report.go`, `cmd/metasystem/runtime_conformance_test.go`, `scripts/agents/supervision-hook.sh`, `scripts/agents/adapters/fake.sh`, `testing.json`.
New: `internal/stopincident/stopincident.go`, `internal/stopincident/stopincident_test.go`, `internal/steward/stopincident.go`.

### 4. Fixtures

| Test | Package and file | Fails without the mechanism because | Seams |
| --- | --- | --- | --- |
| TestStopIncidentDrainBeforeNarrator | `internal/steward`, `tick_test.go` | One incident line is in the log and the narrator step is made to fail. The tick must still leave `pending/stop-incident-<identity>.json`, a delivery pass must add the ack line, and two more ticks must queue nothing new. | `TickConfig.Now` fake clock, `deliverNotification` fake, a narrator input that fails the way `TestDegradedTickQueuesTheIncidentOrSurfacesQueueFailure` makes it fail |
| TestStopIncidentDrainFromHookLog | `internal/steward`, `tick_test.go` | Two incident lines and one seat-actionable line sit below a start line. Exactly two notifications are queued. After delivery the log holds two ack lines and the next tick queues none. A log with no start line queues nothing and gains one. | fake clock, `deliverNotification` fake, a temp hook log |
| TestStopIncidentDeliveryUnconfirmed | `internal/steward`, `notify_test.go` | Four faults, one at a time: the log is unreadable (the refusal records are drained), both are unreadable (`Unconfirmed` is set and the tick still succeeds), the queue write fails (the tick names it and the next tick queues the same nonce), the channel fails (the pending file stays and no ack line is written). After a delivery whose ack append failed, a second pass does not deliver again. | fake clock, `deliverNotification` fake that counts calls, unwritable temp paths |
| TestHookLogLinesRoundTrip | `internal/stopincident`, `stopincident_test.go` | Lines written exactly as the hook writes them parse to the same incidents. Retries inside one deadline share an identity. A different generation or deadline end does not. Unknown lines are skipped. | none |
| TestStopRefusalRecordKeepsTheClass | `internal/report`, `stopblock_test.go` | A record written by `StopRefusal` with class infrastructure reads through `stopincident.ReadRefusalRecords` with that class. A record with no class field reads and yields no incident. | the `now` argument |
| TestFakeStopReplay/FakeReplayUnreadableHookState | `cmd/metasystem`, `runtime_conformance_test.go` | In an enrolled bed the replay runs five times: no fault, the log broken, the refusal state broken, both broken, the engine missing. Every response is an allow. Only the last two carry the unconfirmed sentence. With approved backlog and no job the first two stops still refuse for idle backlog under each fault. | `testutil.Fixture` process bed, file faults, no clock |

### 5. Mutations

- TestStopIncidentDrainBeforeNarrator: in `RunTick`, move the `DrainStopIncidents` call to after `NarrateDigest` and keep the early return on a narrator error.
- TestStopIncidentDrainFromHookLog: in `deliverPendingNotification`, remove the `AppendAck` call.
- TestStopIncidentDeliveryUnconfirmed: in `DrainStopIncidents`, return the hook-log read error instead of falling back to the refusal records.
- TestHookLogLinesRoundTrip: in `Identity`, add Outcome to the hashed fields.
- TestStopRefusalRecordKeepsTheClass: in `StopRefusal`, stop storing the class.
- FakeReplayUnreadableHookState: in the `report stop-block` verb, ignore `--hook-log-failed`.

### 6. Estimate

About 1,450 changed lines: the leaf package 250 and its tests 200, the steward drain and ack 260 and its tests 420, the stop notice 60 and its tests 60, the replay action 70 and its bed test 200, the contract 15.
This sits at the upper edge. If it overflows, the bed splits off: 3a is everything except the `stop-replay` action and `TestFakeStopReplay`; 3b is those two. 3a is complete without 3b except for the fourth fixture of rule 4.

### 7. Landing

Add the six tests to group `wait-stop-standard` in `testing.json`. Add packages `internal/steward` and `internal/stopincident` to that group, and add the inputs `internal/steward/**`, `internal/stopincident/**` and `scripts/agents/supervision-hook.sh` where they are missing. `TestFakeStopReplay` is listed by its parent name.
No design document changes. The engine and the hook both change, so every seat rebuilds with `scripts/agents/go-build.sh` and re-arms.
Proof, from `metasystem/`:

```text
go test ./internal/stopincident/ ./internal/steward/ ./internal/report/ ./cmd/metasystem/
go test -tags batchtest ./internal/stopincident/ ./internal/steward/ ./internal/report/ ./cmd/metasystem/
scripts/agents/go-gate.sh --fast
```

Left for later members: the `stop-replay` action and the `TestFakeStopReplay` parent, which member 7 extends; the `internal/stopincident` package, where member 7 adds the decision line.

## Member 4: stop-frontier-joins-owner-and-revision

One mechanism: one fresh board per stop, scoped to the actor's machine, lineage, goal and revision. Ready work and the WORK IN FLIGHT exemption are both read from it.
Today `readLiveBacklogActivity` in `internal/goal/project.go` emits `job:<id>` for every live job on the checkout, and `HasDelegateJobInFlight` is true for any of them. Another seat's job therefore exempts this seat, and so does a job for an older revision. `Next` scopes held goals by machine only. `queuedFrontier` and `convertedGoalFacts` in `internal/goal/turnverdict.go` read a second, offline projection.
If member 2 lands later, its decision record stores the board's proof beside the decision; nothing in this member changes.

### 1. DONE

1. Ready work and the WORK IN FLIGHT exemption come from one fresh board scoped to machine, lineage, goal and revision. Only live work that joins the actor's held goal exempts the actor. Without it TestStopFrontierOwnerRevisionAndFreshness fails.
2. The board carries a freshness proof. A board whose tip or lease epoch changed before the verdict commits is rejected. A failed fetch keeps today's unreadable-ledger path. Without it the same fixture fails.
3. The ready goal's next-step text is on the board. Without it the same fixture fails.
4. The idle digest is untouched. Without it TestIdleDigestKeepsEveryNonterminalJob fails.
5. The idle handoff decisions are unchanged. Without it IdleHandoffRegression fails: TestIdleBacklogBlocksTwiceThenDefersClaimAndPreparesStewardContinuation, TestIdleEscalationPreservesAnIndependentOpenWorkBlock and TestThreeUnreadableLedgerStopsRecordIncidentRaiseAlarmAndEnd.

### 2. Mechanism

New file `internal/goal/stopboard.go` holds:

- `BoardOwner` with Machine, Lineage and ClaimEpoch.
- `BoardGoal` with Id, Revision, Intent and NextStep.
- `BoardActivity` with Ref (`job:<id>` or `run:<id>`), GoalId, Revision, ClaimEpoch, Joined and Reason.
- `StopBoard` with Owner, Tip, FetchedAt, ProjectedAt, Held, HeldElsewhere, Ready and Activity.
- `(b *StopBoard) HasJoinedWork() bool` and `(b *StopBoard) ExcludeJob(id string)`. Member 7 uses the second.

`readClaimableBudgetedWork` in `internal/goal/project.go` already makes the one fresh projection with `project(endpoint, true, ...)` and calls `Next`. It gains a `BoardOwner` argument and builds the board there. `ClaimableBudgetedWork` gains `Board *StopBoard`. The owner comes from `TurnVerdictOptions.SeatActor` and `TurnVerdictOptions.SeatClaimEpoch`, which `runReportTurnVerdict` in `cmd/metasystem/goal.go` already resolves.

Scoping. `Next` stays machine-only, because `goal next` prints from it and that output does not change. The board filters its result. A claimed goal whose `ClaimRecord.Lineage` equals the owner's lineage is Held. A goal claimed by the same machine under another lineage is HeldElsewhere: it is neither this actor's held work nor ready work. Ready goals copy Id, Revision and NextStep from the goal file. When the owner's lineage is unknown, the board is built with an empty lineage and every claim is HeldElsewhere. That is the honest reading of an unidentified seat.

The join. `backlogJobRecord` gains GoalRevision, ClaimEpoch and Machine, read through the accessors `GoalRevision()`, `ClaimEpoch()` and `MachineID()` of `dispatch.JobRecord`. A live job joins when its goal is in Held, its machine is the owner's, its revision equals the held goal's `Revision`, and its claim epoch equals the claim's `ClaimEpoch`. The lineage is joined through the claim: the claim names the lineage, and the epoch names that one claim. A live governed run joins the same way on the run record's `GoalId`, `OwnerLineage`, `ClaimEpoch` and `Governed.GoalRevision` from `internal/run`. A record that lacks a coordinate still reads and is listed, but it does not join, and its Reason says which coordinate is missing. `readLiveBacklogActivity` returns the activity list with these coordinates and leaves the decision to the board.

`HasDelegateJobInFlight` becomes `work.Board != nil && work.Board.HasJoinedWork()`. `InFlight` keeps every activity for display. `NonTerminalJobs` and `idleBacklogDigest` are not touched: the digest still hashes claimable goals, claimed goals and every non-terminal job, whoever owns it.

Freshness. The board records the accepted tip from `Projection.Tip`, the fetch completion time and the projection time from the injected clock, and the owner. `TurnVerdictOptions` gains `RecheckBoard func() (tip string, claimEpoch int64, err error)`. The command wires it to a local read of the accepted tip, with no second fetch, and to `resolveSeatIdleActor`. `TurnVerdict` calls it once, immediately before `saveVerdictState`. If the tip or the epoch differs, the board is rebuilt once and the decision is made again. If the second board is stale too, the verdict is the infrastructure class with cause `stop-board-stale`. By member 1's rule that allows the stop, and no idle count is spent. A failed fetch is unchanged: `readClaimableBudgetedWork` returns its error, `decide` reports the ledger unreadable, and no board is built from an offline projection.

One board. `queuedFrontier` and `convertedGoalFacts` take the board when the work read ran, and answer from `Held`, `Ready` and the tree the board was built from. They keep their offline `Project(endpoint, false, ...)` read only on the two paths that skip the work read, the human-authorised stop and the brain seat, where nothing can be refused for backlog.

`freezeTurnVerdictFacts` in `internal/goal/turnfacts.go` adds the board's owner, tip and two times to the frozen facts as a new optional field.

Not changed: `Next`, `SelectNext`, `goal next` output, the idle digest, the refusal bound of three, `registeredWaits`, and every read rule on job and run records.

### 3. File set

Existing: `internal/goal/project.go`, `internal/goal/turnverdict.go`, `internal/goal/turnfacts.go`, `cmd/metasystem/goal.go`, `internal/goal/turnverdict_world_test.go`, `internal/goal/turnverdict_idle_test.go`, `testing.json`.
New: `internal/goal/stopboard.go`, `internal/goal/stopboard_test.go`.

### 4. Fixtures

| Test | Package and file | Fails without the mechanism because | Seams |
| --- | --- | --- | --- |
| TestStopFrontierOwnerRevisionAndFreshness | `internal/goal`, `turnverdict_world_test.go` | Subtests. Another machine's live job does not exempt this actor. Another lineage's job on the same machine does not. A job for the old revision or an old claim epoch does not. A joined job and a joined governed run each do. A ready goal's NextStep is on the board. `RecheckBoard` returning a new tip once gives a rebuilt board and a decision; returning it twice gives cause `stop-board-stale`, no block and no count. A failing fetch gives the unreadable-ledger verdict and a nil board. | the store's clock, a fake `identity.Prober`, the `fetch` field of `projectionDependencies`, a fake `RecheckBoard` |
| TestIdleDigestKeepsEveryNonterminalJob | `internal/goal`, `turnverdict_idle_test.go` | Another seat's job appears and ends between two idle stops. The digest must differ and `IdleBlocks` must restart at one, exactly as today. | clock, prober |
| TestBoardJoinReadsOlderRecords | `internal/goal`, `stopboard_test.go` | A job record with no revision and no claim epoch reads without error, is listed with a Reason, and does not join. | prober |
| IdleHandoffRegression (three tests, names above) | `internal/goal`, `turnverdict_idle_test.go` | Their decisions stay. Bodies change only where a fixture job needs the joining coordinates. | as today |
| TestClaudeTurnExitRequiresDelegateWorkNotMerelyALiveSeat | `internal/goal`, `turnverdict_idle_test.go` | The name stays. Its job record gains the goal, revision and claim epoch of the held goal so that it still exempts. | as today |

### 5. Mutations

- TestStopFrontierOwnerRevisionAndFreshness: in the board's join, drop the claim-epoch comparison. The old-epoch subtest goes red.
- The same test, freshness: in `TurnVerdict`, skip the `RecheckBoard` call.
- TestIdleDigestKeepsEveryNonterminalJob: in `idleBacklogDigest`, hash only the jobs the board joined.
- TestBoardJoinReadsOlderRecords: make a missing coordinate an error in `readLiveBacklogActivity`.
- IdleHandoffRegression: make `HasJoinedWork` always return true. The blocks-twice test sees an exemption where it expects its first refusal.

### 6. Estimate

About 1,100 changed lines: the board and its construction 330, the verdict and facts wiring 170, tests 580, the contract 10.

### 7. Landing

Add TestStopFrontierOwnerRevisionAndFreshness, TestIdleDigestKeepsEveryNonterminalJob and TestBoardJoinReadsOlderRecords to `wait-stop-standard`. The group already covers `internal/goal/**`. The engine changes, so every seat rebuilds and re-arms.
Proof, from `metasystem/`:

```text
go test ./internal/goal/ ./cmd/metasystem/
go test -tags batchtest ./internal/goal/ ./cmd/metasystem/
scripts/agents/go-gate.sh --fast
```

Left for later members: member 5 prints the claim command and the next step from `Board.Ready` and `Board.Owner`; member 7 calls `ExcludeJob` for a delegate round.

## Member 5: stop-refusals-name-seat-actions

One mechanism: a refusal is emitted only with one command that runs as printed. A refusal that cannot name one becomes a notice.
`internal/goal/turnfacts.go` already has the pieces. `TurnAction` carries Command, Restriction and HumanRequired. `scanActions` renders `watch-job` and `watch-run` commands. `workActions` renders `continue-goal` with text only, and `claim-goal` with `metasystem goal next --machine <m> --fetch`. That verb claims whichever goal comes first and names neither the goal nor the lineage. `open-plan` and the human items carry no command. The verdict's block sources are `open-work`, `goal`, `unwatched-work`, `uncertainty` and `idle-backlog`.
If member 2 lands later, its decision record stores the chosen command; the choice made here does not change.

### 1. DONE

1. An eligible seat refusal prints exactly one executable clearing command. Without it TestStopClearingCommand fails.
2. A seat refusal with no lawful command becomes a notice: a missing command, a human-only remedy or a text-only next step. The stop is allowed and the text is still shown. Without it TestStopClearingCommand fails.
3. The mandatory idle refusal prints the real claim command and the ready goal's next step, both from the board. Without it TestIdleClaimCommandIsFirstAct fails.
4. The idle decisions stay mandatory: two refusals, then the handoff. Without it IdleHandoffRegression fails (the three tests named in member 4's DONE).

### 2. Mechanism

`engineCommand(root string, args ...string) string` is new in `internal/goal/turnfacts.go`. It renders `bin/metasystem` followed by the arguments, each through `shellArgument`. Every action command uses it, so all commands have one form and run from the checkout root. The `--caller-pid $$` tail of the job watch stays unquoted, because the shell must expand it.

`clearingAction(source string, actions []TurnAction) (TurnAction, bool)` is new in the same file. An action is lawful when Command is not empty, HumanRequired is false and Restriction is empty. The kinds per source are: `unwatched-work` takes `watch-job` or `watch-run`; `goal` takes the goal-free renewal; `open-work` takes `open-plan`; `uncertainty` has none. The first lawful action in the existing sort order wins.

`(s *Store) requireClearingCommand(verdict *Verdict)` is new in `internal/goal/turnverdict.go`. `TurnVerdict` calls it after `decide`, `decideRuns` and the facts are complete, and before `saveVerdictState`. It acts only when ShouldBlock is true, Class is `seat-actionable` and IdleRefusal is false. With a lawful action the display gains a last line `RUN: <command>`, once, and `Verdict` gains `ClearingCommand string`. With none, ShouldBlock becomes false, the new field `NoticeSource` keeps the old block source, the display starts with `NOTICE:`, and one diagnostic says the refusal had no lawful command. The session slots (`BlockedGoalRevisions`, `BlockedFreeDigests`, `BlockedQueueDigests`, `BlockedUnwatchedDigests`) are written as they are for a block, so a notice shows once per digest and not at every stop.

The goal-free branch blocks today with the text "declare a goal or renew with `goal declare-free`". The builder reads `runGoalDeclareFree`. If the renewal runs without text the seat must write, the action carries `engineCommand(root, "goal", "declare-free", "--root", root)`. If it needs such text, the branch is a notice. The test does not care which: a command that does not run as printed is not lawful.

`continue-goal` and `open-plan` have text only, so their refusals become notices. This is the umbrella's first clause at work: a text-only next step cannot force a turn.

The idle path. In `workActions` the `claim-goal` command becomes:

```text
bin/metasystem goal claim --root <root> --id <goal> --lineage <lineage>
```

The goal is `Board.Ready[0]` and the lineage is `Board.Owner.Lineage`. The verb is `runGoalClaim` in `cmd/metasystem/goalsync_mutations.go`, which takes `id` and `lineage`. `idleBacklogContinuation` prints that command and the ready goal's NextStep quoted with `%q`, in place of the `goal next` sentence. When the lineage is unknown the refusal keeps its decision, prints `SeatActorProblem` in place of a command, and fabricates nothing. `requireClearingCommand` never demotes an idle refusal.

Not changed: which conditions are observed, the idle digest and bound, `escalateIdleBacklog`, `watchLine` in `cmd/metasystem/run.go`, and the hook. The hook already relays the display.

### 3. File set

Existing: `internal/goal/turnfacts.go`, `internal/goal/turnverdict.go`, `internal/goal/turnverdict_test.go`, `internal/goal/turnverdict_idle_test.go`, `cmd/metasystem/goal_test.go`, `docs/design/turn-verdict-delivery-contract.md` (the refusal text it quotes), `testing.json`. No new files.

### 4. Fixtures

| Test | Package and file | Fails without the mechanism because | Seams |
| --- | --- | --- | --- |
| TestStopClearingCommand | `internal/goal`, `turnverdict_test.go` | One subtest per block source. Unwatched job and unwatched run: the verdict blocks, the display ends with one `RUN:` line, and `ClearingCommand` equals the action's command. Open work with text only, a human-required item, and `uncertainty`: ShouldBlock is false, NoticeSource is set, and a second stop with the same digest shows nothing new. Idle backlog with the same text-only goal still blocks. | the store's clock, prober |
| TestStopClearingCommandsRun | `cmd/metasystem`, `goal_test.go` | For each lawful command the verdict emits, the test runs the printed string through `sh -c` in an isolated bed with `bin/metasystem` built. A watch command prints its readiness line and the next verdict does not block for that digest. The claim command exits zero. A renamed flag or a wrong quote makes it red. | `testutil.Fixture` with `Shell` and `Expect` on the readiness line, no sleep |
| TestIdleClaimCommandIsFirstAct | `cmd/metasystem`, `goal_test.go` | An approved ready goal has no brief and no job. The refusal names the real goal and lineage, quotes the next step, and prints the claim command. Running it as the seat claims that goal under that lineage. No dispatch appears, no human-stop command is offered, and the next stop is not exempt merely because the goal is now claimed. | clock, a temp ledger endpoint, the fixture shell |
| IdleHandoffRegression (three tests) | `internal/goal`, `turnverdict_idle_test.go` | Decisions and counts stay. Only the asserted refusal text changes to the claim command. | as today |
| TestIdleRefusalSurvivesALostCounter | `internal/goal`, `turnverdict_idle_test.go` | The name stays. Its asserted command becomes the claim command. | as today |

### 5. Mutations

- TestStopClearingCommand: in `requireClearingCommand`, return before the demotion.
- TestStopClearingCommandsRun: in `scanActions`, rename `--job` to `--id` in the watch-job command.
- TestIdleClaimCommandIsFirstAct: in `workActions`, restore the `goal next --machine` command.
- IdleHandoffRegression: in `requireClearingCommand`, drop the IdleRefusal guard. `clearingAction` has no kind for `idle-backlog`, so both idle refusals are demoted.
- TestIdleRefusalSurvivesALostCounter: in `idleBacklogContinuation`, stop printing the command when `CountSpent` is false.

### 6. Estimate

About 900 changed lines: the mechanism 260, tests 600, the document and the contract 40.

### 7. Landing

Add TestStopClearingCommand, TestStopClearingCommandsRun and TestIdleClaimCommandIsFirstAct to `wait-stop-standard`. Update the refusal text quoted in `docs/design/turn-verdict-delivery-contract.md`. Its universal-fallback section waits for member 7. The engine changes, so every seat rebuilds and re-arms.
Proof: the same three commands as member 4.
Left for later members: `Verdict.ClearingCommand` and `Verdict.NoticeSource`, which member 7's gate copies into its response.

## Member 6: stop-fences-surface-once

One mechanism: a per-session memory of the fences already shown.
In `decide` in `internal/goal/turnverdict.go`, the `ok` branch leaves through its two WORK IN FLIGHT exits before `FencedClaimLines(work.fencedClaims)` is appended. A fenced goal held beside live work is therefore never shown. On every other path the FENCED lines print at every stop. The `queued-only` branch appends the same lines.
If member 2 lands later, nothing changes: this member reads only the goal files and the per-session verdict state.

### 1. DONE

1. A held fenced goal prints FENCED once per unchanged fence. New hook generations and repeated stops do not print it again. Without it TestFencedClaimSurfacedOnceBesideFlight fails.
2. It prints beside live work too. Without it the same fixture fails.
3. A changed fence prints a new line. The fenced goal is never continued. Without it the same fixture fails.

### 2. Mechanism

`sessionState` gains `SurfacedFences []string` with `json:"surfacedFences,omitempty"`, capped by a new constant `maxSurfacedFences = 16`, oldest dropped first as the other slots do.

`fencedClaimFingerprint(f *GoalFile) string` is new. It hashes the goal id, the `StopFence` fields StopID, Revision, Epoch, CapabilityGeneration, ClosedAt and Reason, and `ClaimRecord.FenceEpoch`. The hook generation is not part of it.

`(s *Store) surfaceFencedClaims(session *sessionState, claims ...) []string` is new. It takes the slice `FencedClaimLines` takes today. It returns the lines of the claims whose fingerprint the session has not stored, stores those fingerprints, and drops stored fingerprints whose fence is no longer held.

In `decide`, the call moves to the top of the `ok` branch, before every early exit of that branch. The builder checks each `break` and `return` between the top of the branch and today's append. The lines it returns are appended on every exit path, including both WORK IN FLIGHT exits. The `queued-only` branch calls the same function. `OnlyFencedClaim` still keeps the fenced goal out of the continuation, and no `continue-goal` action names it.

If the state write fails, the line can print once more at the next stop. A repeat is acceptable; a loss is not.

Not changed: `FencedClaimLines` itself, `runGoalNext` (a query prints every fence every time), `IsFencedClaim`, and the stop-fence records.

### 3. File set

Existing: `internal/goal/turnverdict.go`, `internal/goal/turnverdict_stopfence_test.go`, `testing.json`. No new files.

### 4. Fixtures

| Test | Package and file | Fails without the mechanism because | Seams |
| --- | --- | --- | --- |
| TestFencedClaimSurfacedOnceBesideFlight | `internal/goal`, `turnverdict_stopfence_test.go` | A fenced goal is held beside a working goal. With a joined live job the first stop shows one FENCED line beside WORK IN FLIGHT. The second and third stops, each with a new hook generation, show none. After the fence's Epoch and Reason change, one new line shows. The same sequence runs without live work. At no stop does a continuation or an action name the fenced goal. | the store's clock, prober |
| TestTurnVerdictDoesNotBlockAndDescribesTheDurableStopPhase | `internal/goal`, `turnverdict_stopfence_test.go` | The name stays. If it stops twice in one session, its second expectation becomes no FENCED line. | as today |

### 5. Mutations

- TestFencedClaimSurfacedOnceBesideFlight, once: in `surfaceFencedClaims`, do not store the fingerprint.
- The same test, beside flight: move the call back below the WORK IN FLIGHT exits.
- The same test, change: remove Epoch from `fencedClaimFingerprint`.

### 6. Estimate

About 350 changed lines: the mechanism 110, tests 230, the contract 5. This is below the expected range because the mechanism is small. It stays its own member because it has its own DONE, and folding it into member 5 would put two mechanisms in one landing.

### 7. Landing

Add TestFencedClaimSurfacedOnceBesideFlight to `wait-stop-standard`. The engine changes, so every seat rebuilds and re-arms. Proof: the same three commands as member 4. It leaves nothing for a later member.

## Member 7: stop-hosts-enforce-the-common-gate

One mechanism: one Go verb decides a stop, and both production boundaries call it and enforce what it committed.
This member does not fit one section's build. It touches a new verb, the native hook, the Codex adapter, the dispatcher and two live beds. It splits in two, and each half lands green alone. 7a builds the gate, moves the Claude hook onto it, writes the decision line and runs the fake legs. 7b wires the Codex round and runs the two live legs. The umbrella's DONE for this member is met when 7b lands.
If member 2 lands later, the gate commits its version-2 decision record before it answers and `committed` means that commit; until then `committed` means the verdict state and the decision line were both written.

### 1. DONE

1. One verb, `host stop-gate`, computes the stop decision, commits it and answers in one response shape. `report turn-verdict` stays and calls the same service. 7a. Without it TestEveryRuntimeUsesStopDecision fails.
2. The Claude native Stop hook calls the gate and enforces a block only when the response says it was committed. 7a. Without it TestEveryRuntimeUsesStopDecision and TestFakeStopReplay fail.
3. Infrastructure never asks for another turn, and a repeat inside one deadline causes no new notification identity. 7a. Without it the fake legs FakeReplayNarrator, FakeReplayDeadline, FakeReplayArming and FakeReplayIdleWithInfrastructure fail.
4. Every stop leaves one decision line in the hook log, and the log can be counted per seat and day. 7a. Without it TestDailyRefusalCountReadsTheLogStream fails.
5. The Codex round's turn boundary calls the gate with its own job excluded. On a committed refusal it re-invokes the provider once in the same job under the remaining cap. A second refusal ends the round with the refusal recorded as its gap. 7b. Without it TestCodexRoundHonoursTheGate and RealCodexAdapterStopReplay fail.
6. A runtime with neither boundary is reported unproven, never certified. 7a. Without it TestEveryRuntimeUsesStopDecision fails.
7. Both boundaries are proven live. 7b. RealClaudeNativeStopReplay and RealCodexAdapterStopReplay.

### 2. Mechanism

7a, the gate. `runHostStopGate` is new in `cmd/metasystem/host_verbs.go` and is registered in the host family in `cmd/metasystem/main.go`, beside `result-write` and `finish`. It takes every flag `runReportTurnVerdict` takes, plus `--boundary` (`native-stop` or `delegate-round`), `--generation`, `--deadline-end` and `--exclude-job`. The collection and decision code moves out of `runReportTurnVerdict` into one function, `stopDecision`, in `cmd/metasystem/goal.go`, and both verbs call it. `--runtime` stays an opaque token: the engine writes it down and never branches on it.

The response is one JSON object: `schemaVersion`, `decision` (`allow` or `block`), `class`, `committed`, `reason`, `systemMessage`, `command`, `noticeSource`, `generation`, `deadlineEnd`, and `verdict` holding the full `Verdict`. `decision` is `block` only when the verdict blocks and `committed` is true. `committed` is true when `saveVerdictState` succeeded and the decision line was appended. An idle refusal whose count could not be spent keeps member 1's rule and still blocks; the reason says the count was not spent.

The decision line. `internal/stopincident` gains `Decision`, `AppendDecision` and `ReadDecisions`. The gate appends one line per stop:

```text
stop-decision <epoch> <machine> <session-slug> <boundary> <runtime> <class> <outcome> <generation|-> <deadline_end|->
```

The outcome is `allow`, `block`, `notice` or `unconfirmed`. The machine comes from `ResolveMachine`. The hook's older lines (`stop verdict block=`, `stop response decision=`, `stop-condition`) are still written and still read.

The count. `DailyRefusalCounts(path string, from, to time.Time, zone *time.Location) []SeatDayCount` is new in `internal/report/stopcount.go`. It counts `block` lines per machine and local day, and beside them the `unconfirmed` lines. A new verb `report stop-count --root . --days 7` in `cmd/metasystem/report.go` prints one row per seat and day. It reads only the hook log.

The hook. In `scripts/agents/supervision-hook.sh`, `report_turn_verdict()` calls `host stop-gate --boundary native-stop` with `hook_generation` and `stop_started_epoch + 60`. The response builder takes `decision`, `systemMessage` and `reason` from the gate and no longer derives a block itself. A gate that cannot be run, or that answers without `committed`, gives an allow with the degraded notice.

The fake legs. The `stop-replay` action from member 3 gains `--cause narrator|deadline|arming`, and `TestFakeStopReplay` gains the four remaining subtests, including the steward tick that starts the guarded continuation on the third idle stop.

The registry. `runtimes.Declaration` in `internal/runtimes/runtimes.go` gains `StopBoundary string` with the values `native-stop`, `delegate-round` or empty. The declarations live beside the existing capability flags.

7b, the Codex round. In `supervise()` in `scripts/agents/adapters/codex.sh` the boundary sits after `settle_result_identity` and before `complete_from_cli`. `gate_turn_boundary` is new in `scripts/agents/adapters/runtime-common.sh`. It calls `host stop-gate --boundary delegate-round --exclude-job <job id>`. On `allow` it returns. On a committed `block`, when no gate turn was spent in this job and the remaining cap allows a turn, it has the dispatcher record the gate turn on the job record and then calls the adapter's `runtime_gate_turn`. The builder reads `internal/dispatch/capcontinuation.go` and `__record-cas` in `scripts/agents/dispatch.sh` for the shape of that record. `runtime_gate_turn` is new in `codex.sh`. It builds a `codex exec resume` command through `build_codex_command`, with a prompt made of the gate's reason and command, and waits through `wait_for_cli`. The identity is settled again and the gate is called again. A second block is not enforced: the round completes, and the refusal is written to `stop-gate.json` in the round directory and into the terminal patch as the round's gap. Only `devin.sh` defines `runtime_repair_turn` today. The gate turn is a separate function and leaves that one alone.

`TurnVerdictOptions` gains `ExcludeJob string`. The board calls `ExcludeJob` and the scan drops the same id from the owned jobs, so a round can neither exempt nor block itself.

The live legs are in `cmd/metasystem/runtime_conformance_live_test.go` behind the build tag `livestop`. They start the real providers in isolated enrolled beds with `testutil.Fixture` and wait on readiness lines and on the decision lines in the bed's hook log. They keep the transcripts, the decision lines and the steward receipt.

Not changed: the verdict rules of members 1 to 6, `adapter adjudicate-turn` in `internal/adapter/adjudicate.go`, the claude adapter's rounds (they keep their native hook), and the managed-seat host scripts.

### 3. File set

7a, existing: `cmd/metasystem/host_verbs.go`, `cmd/metasystem/main.go`, `cmd/metasystem/goal.go`, `cmd/metasystem/report.go`, `cmd/metasystem/runtime_conformance_test.go`, `cmd/metasystem/goal_test.go`, `internal/stopincident/stopincident.go`, `internal/stopincident/stopincident_test.go`, `internal/report/stopblock_test.go`, `internal/runtimes/runtimes.go`, `scripts/agents/supervision-hook.sh`, `scripts/agents/supervision-hook-fixtures.sh` (its expectations of a first infrastructure block), `scripts/agents/adapters/fake.sh`, `docs/design/turn-verdict-delivery-contract.md`, `testing.json`. New: `internal/report/stopcount.go`.
7b, existing: `scripts/agents/adapters/codex.sh`, `scripts/agents/adapters/runtime-common.sh`, `scripts/agents/dispatch.sh`, `internal/dispatch/capcontinuation.go`, `internal/goal/turnverdict.go`, `internal/goal/stopboard.go`, `cmd/metasystem/runtime_conformance_test.go`, `testing.json`. New: `cmd/metasystem/runtime_conformance_live_test.go`.

### 4. Fixtures

| Test | Package and file | Fails without the mechanism because | Seams |
| --- | --- | --- | --- |
| TestEveryRuntimeUsesStopDecision (7a, extended in 7b) | `cmd/metasystem`, `runtime_conformance_test.go` | It walks `runtimes.All()`. A `native-stop` runtime's hook transport must call `host stop-gate` and must produce no block when the gate answers uncommitted. A `delegate-round` runtime's adapter must call `gate_turn_boundary`. A runtime with neither is printed as unproven, and the test fails if such a runtime is marked certified. An infrastructure verdict never yields `block`. | a fake engine on `METASYSTEM_BIN` that records its arguments, `testutil.Fixture` |
| TestStopGateAnswersOnlyCommittedBlocks (7a) | `cmd/metasystem`, `goal_test.go` | The verdict-state write and the decision-line append are faulted in turn. A seat-actionable block without a commit answers `allow` with `committed` false. An idle refusal still answers `block` and says the count was not spent. `report turn-verdict` and `host stop-gate` return the same verdict for the same inputs. | clock, unwritable temp paths |
| TestDailyRefusalCountReadsTheLogStream (7a) | `internal/report`, `stopblock_test.go` | A synthetic week of decision lines from two seats counts per seat and local day. A line just before local midnight lands on its own day. Replacing the per-session verdict file changes no count. Older line kinds are ignored and do not fail the read. | the `from`, `to` and `zone` arguments |
| TestFakeStopReplay: FakeReplayNarrator, FakeReplayDeadline, FakeReplayArming, FakeReplayIdleWithInfrastructure (7a) | `cmd/metasystem`, `runtime_conformance_test.go` | Each recorded cause allows the stop, writes one `stop-decision allow` line, and a repeat with the same generation and deadline end queues no second `stop-incident-` nonce. With approved backlog and no job, stops one and two block, stop three stages the continuation and allows, a steward tick starts it, and a failed dispatch raises the existing alarm. | process bed, `TickConfig.Now`, `deliverNotification` fake, file faults |
| TestCodexRoundHonoursTheGate (7b) | `cmd/metasystem`, `runtime_conformance_test.go` | A fake `codex` binary on `PATH` and a fake gate. Block then allow gives exactly one resume invocation and a normal completion. Block twice gives one resume, a completed round and `stop-gate.json` naming the refusal. An infrastructure answer gives no resume. The gate's arguments carry `--exclude-job` with the round's own job. No remaining cap gives no resume. | fake binaries that print readiness lines, `testutil.Fixture` |
| RealClaudeNativeStopReplay, RealCodexAdapterStopReplay (7b, tag `livestop`) | `cmd/metasystem`, `runtime_conformance_live_test.go` | Subtests of TestRealStopReplay. With the real providers: a narrator read failure, a deadline expiry and an arming failure cause no reprompt. Approved backlog with no job gives the idle refusal and then the third-call handoff. | real providers, readiness lines, the bed's hook log |

### 5. Mutations

- TestEveryRuntimeUsesStopDecision: in `report_turn_verdict()`, call `report turn-verdict` again in place of the gate.
- TestStopGateAnswersOnlyCommittedBlocks: in `runHostStopGate`, set `decision` from `ShouldBlock` alone.
- TestDailyRefusalCountReadsTheLogStream: in `DailyRefusalCounts`, group by UTC day.
- TestFakeStopReplay: in the gate, append the decision line only when the verdict blocks.
- TestCodexRoundHonoursTheGate: in `gate_turn_boundary`, drop the check that no gate turn was spent yet. The block-twice case then resumes twice.
- TestRealStopReplay: in `codex.sh`, remove the `gate_turn_boundary` call. The idle leg ends with no refusal line.

### 6. Estimate

7a is about 1,400 changed lines: the gate and the shared service 300, the decision line and the count 220, the hook 120, the registry and the conformance test 260, the four fake legs 380, the document and the contract 120.
7b is about 1,200: the adapter boundary and the gate turn 260, the dispatcher record 140, `ExcludeJob` 60, the fake-provider test 380, the live legs 340, the contract 20.
The whole member is about 2,600 lines and cannot be one build. 7a has no need of 7b: the native hook is on the gate, the count is readable, and the conformance test reports the Codex boundary as not yet wired instead of certifying it.

### 7. Landing

7a adds TestEveryRuntimeUsesStopDecision, TestStopGateAnswersOnlyCommittedBlocks, TestDailyRefusalCountReadsTheLogStream and the TestFakeStopReplay parent to `wait-stop-standard`. It replaces the section "The universal fallback (no hooks required)" in `docs/design/turn-verdict-delivery-contract.md`, which still sends a seat to `goal next --machine`, with the gate and its two boundaries. It retires the first-infrastructure-block expectations in `scripts/agents/supervision-hook-fixtures.sh`.
7b adds TestCodexRoundHonoursTheGate to the same group. It adds a new group `stop-live-replay` for TestRealStopReplay, with the tag `livestop`. That group must stay out of the per-change set: it costs provider turns. The builder uses the kind and cadence `testing.json` already gives provider-backed beds. If it has none, the builder stops and says so.
Both halves change the engine and the scripts, so every seat rebuilds and re-arms. After 7a the seat records the date on which every seat runs the new hook, because member 8 counts from that date.
Proof for each half, from `metasystem/`:

```text
go test ./internal/stopincident/ ./internal/report/ ./internal/runtimes/ ./internal/goal/ ./internal/dispatch/ ./cmd/metasystem/
go test -tags batchtest ./internal/stopincident/ ./internal/report/ ./internal/runtimes/ ./internal/goal/ ./internal/dispatch/ ./cmd/metasystem/
scripts/agents/go-gate.sh --fast
```

7b also runs this once, on the orchestrator:

```text
go test -tags livestop -run TestRealStopReplay ./cmd/metasystem/
```

It leaves the decision lines and the count verb for member 8. The managed-seat lifecycle for hosts with no native Stop is the follow-on goal the umbrella names.

## Member 8: stop-refusals-under-ten-a-day

This member is an observation. It builds nothing and has no fixtures. TestDailyRefusalCountReadsTheLogStream lands with 7a, because the reader it tests is built there.

What is counted: refusals per seat per local day, for seven full days, on every seat. A refusal is one `stop-decision` line with outcome `block`. Each checkout has its own `artifacts/agents/supervision/hooks.log`, so one log is one seat. The residual is counted beside it: the `unconfirmed` decision lines, plus the infrastructure `stop-condition` lines that have no `stop-incident-ack` line.

How a seat produces the count, from `metasystem/`:

```text
bin/metasystem report stop-count --root . --days 7
```

Until 7a is on every seat, the hook's response line gives an interim count per UTC day:

```text
grep 'stop response decision=block' artifacts/agents/supervision/hooks.log | cut -c1-10 | sort | uniq -c
```

The interim count is for watching the trend only. It is not the accepted number, because it has no seat field and cuts the day in UTC.

The version-1 refusal record under `artifacts/agents/supervision/stop-refusals/` is a cross-check. Its `count` per cause should not exceed the lines of that session. The per-session verdict file is replaced at every verdict and is never a source.

What member 7 must leave behind: one decision line per stop carrying the epoch, the machine and the session; the `report stop-count` verb; a hook log that is not truncated inside the seven days, or a reader that also reads its rotated files; and the recorded date on which every seat was rebuilt and re-armed on 7a. The seven days start the day after that date.

The seat records the table of counts and the residual on the umbrella page. The acceptance is Wido's.
If member 2 lands later, the count also reads its version-2 incident records for the residual; the refusal count itself does not change.

## Not checked

The tool budget did not allow these checks. The builder of the named member confirms each before relying on it.

- Member 3: the exact arguments and standard input of the installed Stop hook entry, which `stop-replay` must reproduce. The signature of the tick's health path for the component `stop-incident-drain`. That `append_stop_condition` runs before `external_stop_json` on every path. Whether the cause passed to `report stop-block` equals the cause code on the `stop-condition` line.
- Member 4: the lister for live governed runs that `readLiveBacklogActivity` should use, and the local accepted-tip reader that `RecheckBoard` should wrap.
- Member 5: the required flags of `goal declare-free`, whether `goal claim` needs more than `--root`, `--id` and `--lineage`, and which helper builds `bin/metasystem` for process tests in `cmd/metasystem`.
- Member 6: every early exit of the `ok` branch in `decide`, and the element type of `work.fencedClaims`.
- Member 7: the record shape the dispatcher uses for a cap continuation, the resume path of `build_codex_command`, and the kind and cadence `testing.json` gives provider-backed tests.
- Member 1's fixtures: a grep for TestStopConditionClassification and TestLauncherFailureNeverRawBlocks found nothing at 4e4e46de7. No section here depends on them by name.
