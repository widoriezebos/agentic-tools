# stop-hook-never-forces-an-empty-turn: members design

Revision 2, 2026-09-19. Written against the tree at 4e4e46de7.
Revision 1 is `plans/stop-hook-never-forces-an-empty-turn-members-design.md`. This revision folds critique r1; the section "Critique r1 dispositions" answers every finding.
The umbrella page is `plans/stop-hook-never-forces-an-empty-turn-design.md` and is not restated here. This page departs from it in four places, marked NEW MECHANISM in the dispositions. The seat decides those before the builds start.
All paths are relative to `metasystem/`.

Wido accepted on 2026-09-12 that the umbrella is designed and built per member.
Member 1, stop-infrastructure-allows-the-seat-to-stop, is concluded (c8074077). Its typed classification is the base and is not redesigned.
The fold split member 3 in two and member 7 in four. Each part is a member with its own DONE, fixtures and mutations, and the seat opens the new ones. The landing order is 3a, 3b, 4, 5, 6, 7a, 7b, 7c, 7d. Each lands green alone.
A section is the whole brief for one Codex job. Member 8 is an observation with a short section at the end.

These rules bind every section.

- A test never depends on wall-clock time. Use injected clocks and fakes. Never `t.Skip`, never a raised bound, never a retry. A process fixture uses `internal/testutil` and waits on a fact: a readiness descriptor, a named pipe or a process exit. Never a sleep.
- One mechanism per member, with its own DONE, landing alone.
- The engine never names a runtime. Claude and Codex specifics stay in `scripts/agents/supervision-hook.sh` and `scripts/agents/adapters`.
- Read rules on append-only records are never tightened. Older records and log lines still read.
- A test name that `testing.json` lists is never renamed or dropped. The body adapts and the name stays.
- Source comments say what the code does and why, in plain English. They never carry a round, finding, seat or goal id.
- Each DONE item names in brackets the fixtures that fail without it.
- Each Landing adds the member's new tests to group `wait-stop-standard` in `testing.json`, and names only what else the group needs.
- Every builder returns the output of `scripts/agents/go-gate.sh --fast` with the diff. The proof of a member is `go test <packages>`, `go test -tags batchtest <packages>` and that gate, run from `metasystem/`.
- After a landing that changes the engine or the hook, every seat rebuilds with `scripts/agents/go-build.sh` and re-arms.

## Member 2 is parked

Member 2, stop-decisions-record-deadline-evidence, was parked and decoupled by Wido on 2026-09-19. No member here depends on it. Where the umbrella assumes the version-2 incident record, each member reads what the tree holds today: the refusal record that `StopRefusal` writes (`internal/report/stopblock.go`, schemaVersion 1, one file per session under `artifacts/agents/supervision/stop-refusals/`), the lines of `artifacts/agents/supervision/hooks.log`, and the per-session verdict state in `artifacts/agents/turn-verdict-state.json`. TestInfrastructureDeadlineIdentity, TestStopDecisionPersistsBeforeEmission and TestStopDeadlineCompletionIsSingleUse stay with member 2. TestArmingDetailSurvivesStop already exists in `cmd/metasystem/up_test.go` and is left alone. Each section says in one sentence what changes if member 2 lands later.

## Member 3a: stop-incidents-reach-the-steward

One mechanism: the steward tick drains stop incidents into its notification queue, once per incident, and records each delivery.
Since member 1 an infrastructure failure allows the stop and appends a `stop-condition` line to the hook log. Nothing reads those lines.
If member 2 lands later, its record becomes a third source for the drain; the rest stays.

### 1. DONE

1. Every steward tick drains stop incidents before any narrator work, and a narrator failure does not stop the drain (TestStopIncidentDrainBeforeNarrator).
2. The drain reads the hook log's infrastructure `stop-condition` lines and the refusal records, and queues one notification per incident identity. One incident has one identity in both sources (TestStopIncidentDrainFromHookLog, TestStopRefusalRecordKeepsTheClass).
3. A delivered identity is written to the steward's delivered ledger and is never queued again, whatever the hook log does later. The delivery is also acknowledged by a hook-log line. A failed delivery keeps its identity and is retried (TestStopIncidentOutageRecoveryOutage, TestStopIncidentDeliveryUnconfirmed).
4. Either source alone is enough. Only when both are unreadable, or the ledger is unusable, does the tick report delivery unconfirmed. That never blocks and never fails the tick (TestStopIncidentDeliveryUnconfirmed).

### 2. Mechanism

A new leaf package `internal/stopincident` owns the formats. It imports no engine package, because `internal/report` already imports `internal/steward` and both need these readers. It holds:

- `Incident`: Class, Cause, Component, Generation, DeadlineEnd, Outcome and Source (`hook-log` or `refusal-record`).
- `Identity(i Incident) string`: 16 hex characters of a SHA-256 over class, cause, component, generation and deadline end. Outcome is left out, so a retry inside one deadline keeps one identity.
- `ReadHookLog(path) (incidents []Incident, acked map[string]bool, err error)`. It parses the line `append_stop_condition` writes. Only class `infrastructure` is an incident. A line it cannot parse is skipped.
- `AppendAck(path, identity string, deliveredAt time.Time)`, which writes the one new line kind.
- `ReadRefusalRecords(dir string) ([]Incident, error)`, a read-only view of the version-1 refusal record.

```text
stop-condition <class> <cause> <component> <generation|-> <deadline_end epoch> <outcome>
stop-incident-ack <identity> <delivered epoch>
```

The shared identity. The sources differ today: the log line carries the slug from `stop_cause_code`, and `report stop-block` gets the human diagnostic (`supervision-hook.sh:1745-1756`, `2311-2333`). So the identity is computed once, in Go, from coordinates the hook passes. `report stop-block` in `cmd/metasystem/report.go` gains the optional flags `--cause-code`, `--component`, `--generation` and `--deadline-end`. `external_stop_json` passes `stop_failure_code`, `stop_failure_component`, the hook generation or `-`, and `stop_started_epoch + 60`, the values `append_stop_condition` writes. The deadline parent passes `stop-deadline-expired`, `stop-deadline`, `-` and its deadline end, which moves out of `deadline_log_stop_condition` into a variable both calls read.
`StopRefusal` in `internal/report/stopblock.go` gains a last argument `incident *stopincident.Incident`, nil for a caller without coordinates. `stopRefusalCause` gains `Class string` and `Incidents []stopRefusalIncident` (Identity, DeadlineEnd), both `omitempty`, newest 32 kept. An entry with no incidents is history: it reads and is never queued.

The delivered ledger is `artifacts/agents/steward/stop-incidents-delivered.log`, beside the pending directory (`internal/steward/intervene.go:330-332`). It is append-only and steward-owned, so it does not depend on the hook log.

```text
drain-start <epoch>
delivered <identity> <delivered epoch> <source>
```

`DrainStopIncidents(repoRoot string, now time.Time) StopIncidentDrain` is new in `internal/steward/stopincident.go`. In order:

1. Read the ledger. If there is none, write the `drain-start` line and queue nothing on this tick. An incident whose deadline end is before the start epoch is history, so the landing causes no burst.
2. Read both sources and merge them by identity.
3. For each identity with no ledger line and no pending file, call `QueueNotification` with nonce `stop-incident-<identity>`. The pending-file check matters: `QueueNotification` overwrites the nonce file (`intervene.go:334-350`) and would erase a delivery stamp. At most 20 per tick.
4. For each ledger identity whose ack line is missing from a readable hook log, append it. At most 20 per tick.
5. If both sources are unreadable, or the ledger cannot be read or written, return `Unconfirmed` with the errors. With an unusable ledger nothing is queued, because once cannot be promised.

`RunTick` in `internal/steward/tick.go` calls the drain first after the arbitration lock and the self-identity step, well before `NarrateDigest`. `TickResult` gains `StopIncidents StopIncidentDrain` (Queued, Pending, Unconfirmed, Detail). The tick verb's printed report in `cmd/metasystem/steward_verbs.go` gains a `stopIncidents` key. No health role is added: `stop-incident-drain` is not in the closed role vocabulary of `internal/steward/health.go`.

Delivery. In `deliverPendingNotification` in `internal/steward/notify.go`, the order for a `stop-incident-` nonce becomes: deliver, stamp the pending file, append the ledger line, `MarkDelivered`, append the ack line. `PendingNotification` gains `DeliveredAt string` with `omitempty`. `DeliverPending` never delivers a stamped file again; it only retries the ledger append and the removal. The ack append is best effort and step 4 repairs it. So neither an unwritable ledger nor an unwritable hook log causes a second delivery.

Not changed: the verdict computation, the `stop-condition` format, `DeliverPending` for every other nonce, and the hook's responses. The stop notice is member 3b's.

### 3. File set

Existing: `internal/steward/tick.go`, `internal/steward/notify.go`, `internal/steward/intervene.go`, `internal/steward/tick_test.go`, `internal/steward/notify_test.go`, `internal/report/stopblock.go`, `internal/report/stopblock_test.go`, `cmd/metasystem/report.go`, `cmd/metasystem/steward_verbs.go`, `scripts/agents/supervision-hook.sh`, `testing.json`.
New: `internal/stopincident/stopincident.go`, `internal/stopincident/stopincident_test.go`, `internal/steward/stopincident.go`.

### 4. Fixtures

Every steward test uses the `TickConfig.Now` fake clock, the `deliverNotification` fake and temp paths.

| Test | Package and file | Fails without the mechanism because |
| --- | --- | --- |
| TestStopIncidentDrainBeforeNarrator | `internal/steward`, `tick_test.go` | One incident line is in the log and the narrator step fails. The bed repository has one commit and no earlier evidence, so the digest has an entry (`narrate.go:61-70`; `narratordigest.Append` returns early on none, `digest.go:146-150`), and the digest's directory is unwritable. The tick must still leave `pending/stop-incident-<identity>.json`, a delivery pass must add the ledger and ack lines, and two more ticks must queue nothing. |
| TestStopIncidentDrainFromHookLog | `internal/steward`, `tick_test.go` | With a ledger start in place the log holds two incident lines, one seat-actionable line and one incident that ended before the start. Exactly two are queued. After delivery the ledger and the log hold two lines each, and the next tick queues none. With no ledger the first tick queues nothing and writes the start line. |
| TestStopIncidentOutageRecoveryOutage | `internal/steward`, `tick_test.go` | One incident is in both sources. Log unreadable: it is drained from the refusal record and delivered. Log readable again: nothing is queued and the ack line is repaired. Log unreadable again: nothing is queued. |
| TestStopIncidentDeliveryUnconfirmed | `internal/steward`, `notify_test.go` | One fault at a time. Both sources unreadable: `Unconfirmed`, and the tick succeeds. Ledger unusable: `Unconfirmed`, nothing queued. Queue write fails: the tick names it and the next tick queues the same nonce. Channel fails: the pending file stays and no ledger line is written. Ledger append fails after a delivery: a second pass does not deliver again. |
| TestHookLogLinesRoundTrip | `internal/stopincident`, `stopincident_test.go` | Lines written as the hook writes them parse to the same incidents. Retries inside one deadline share an identity; another generation or deadline end does not. Unknown lines are skipped. |
| TestStopRefusalRecordKeepsTheClass | `internal/report`, `stopblock_test.go` | A record `StopRefusal` wrote with coordinates reads through `ReadRefusalRecords` with the identity `Identity` gives the matching log line. A record with no incidents reads and yields none. |

### 5. Mutations

- TestStopIncidentDrainBeforeNarrator: move the `DrainStopIncidents` call in `RunTick` to after `NarrateDigest`, keeping the early return on a narrator error.
- TestStopIncidentDrainFromHookLog: drop the rule that an incident before the start epoch is history.
- TestStopIncidentOutageRecoveryOutage: skip the ledger lookup in `DrainStopIncidents` and trust only the ack lines.
- TestStopIncidentDeliveryUnconfirmed: deliver a stamped pending file again in `DeliverPending`.
- TestHookLogLinesRoundTrip: add Outcome to the fields `Identity` hashes.
- TestStopRefusalRecordKeepsTheClass: stop storing the incident in `StopRefusal`.

### 6. Estimate

About 1,400 changed lines (leaf package 230 and tests 200, drain, ledger and ack 330 and tests 500, refusal coordinates 130, contract 10). The ledger and the shared identity came in, and the stop notice and replay bed moved out to 3b; it was 1,450.

### 7. Landing

The group also gains packages `internal/steward` and `internal/stopincident`, and the inputs `internal/steward/**`, `internal/stopincident/**` and `scripts/agents/supervision-hook.sh` where missing. Engine and hook change.
Proof packages: `./internal/stopincident/ ./internal/steward/ ./internal/report/ ./cmd/metasystem/`.
Left for 7a: the `internal/stopincident` package and the refusal-record source.

## Member 3b: stop-notice-says-delivery-unconfirmed

One mechanism: the stop notice says so when an incident could reach neither the hook log nor the refusal record. It is built with its proof bed, the `stop-replay` action.
It uses nothing 3a builds. It lands after 3a because the sentence speaks of a delivery only 3a makes.
If member 2 lands later, its record becomes a third write the sentence names.

### 1. DONE

1. When the hook could write neither the condition line nor the refusal record, the stop notice says delivery to the steward is unconfirmed, and the decision stays allow (FakeReplayUnreadableHookState).
2. The deadline parent says the same on its own branch (FakeReplayDeadlineUnreadable).
3. One failed write alone never produces the sentence (both fixtures).

### 2. Mechanism

No flag is needed. The hook holds both facts when it composes the notice.

The ordinary worker. `append_stop_condition` sets `hook_log_failure` (`supervision-hook.sh:2441-2448`) and runs before `compose_failed_stop` (`2567-2587`, `2621-2625`). `compose_failed_stop` knows the record failed when `report stop-block` returns nonzero or nothing (`2328`); the engine prints nothing then, so the sentence is the hook's. In that branch, when `hook_log_failure` is also set, `record_failure_message` gains: "Delivery of this incident to the steward is unconfirmed: neither the hook log nor the refusal record could be written."

The deadline parent. It calls `report stop-block` before `deadline_log_stop_condition` (`1579-1586`), discards that response, and renders `degraded_stop_form` afterwards with the qualifiers `record-update-failed` and `condition-log-failed` (`1587-1598`). Both facts are known there, so nothing is reordered. A fourth qualifier `delivery-unconfirmed`, text `steward delivery unconfirmed`, is admitted for the causes that admit both others. The parent requests it when both are set. A missing engine already sets `deadline_record_failure` (`1572-1575`), so the same rule covers it. The qualifier tables exist twice, in `supervision-hook.sh:1143-1161` and in `scripts/agents/stop-degraded-forms.sh`. Both change, with `internal/hooks/testdata/stop-degraded-forms.golden`. The `requested` array in `degraded_stop_form` grows to four.

The replay action. The tree holds no fake replay leg and no `stop-replay` action. This member adds the action to `scripts/agents/adapters/fake.sh`:

```text
stop-replay --root <bed> --session <id> --cause arming|deadline --fault none|hook-log|incident-state|both|engine
```

It runs the bed's installed Stop entry as a provider would: the command string `(bash scripts/agents/supervision-hook.sh claude stop) || printf ...` (`scripts/enforcement/claude-code-hooks.json:28-35`), through `sh -c` from the bed root, with the provider JSON on standard input. It prints the hook's response.
A fault alone creates no stop condition, because `record_stop_failure` is the only producer (`1745-1756`). So every run injects a cause. The action writes a wrapper engine into the bed and points `METASYSTEM_BIN` at it, the pattern of `scripts/agents/supervision-hook-fixtures.sh:1834-1836`. The wrapper appends each call's verb to `replay-calls.log` and runs the real engine, with one exception per cause. With `--cause arming` it fails `up`, so the hook records `supervision arming failed`. With `--cause deadline` it blocks its first call and the run uses the expiry seam.
A fault is a file state: the log or the refusal directory is replaced by an unwritable path. `engine` points `METASYSTEM_BIN` at a missing file, with no wrapper.

The expiry seam. The parent clamps its budget to 4 to 60 seconds and waits on `SECONDS` (`1236-1267`), so a real expiry costs wall time. The parent honours `METASYSTEM_STOP_DEADLINE_FIXTURE=<dir>` only when the environment also carries the process-fixture owner tag that `ProcessFixture.Env` exports (`identity.FixtureOwnerEnv`, `internal/testutil/fixture.go:162-167`). A provider never exports that tag. In that mode the parent takes its start epoch from `<dir>/started-epoch`, waits until the wrapper has written `<dir>/worker-blocked`, and then expires at once. Everything after the expiry runs as in production. Without the variable the parent is unchanged.

Not changed: every decision, the `stop-condition` line, the three existing qualifiers and the production deadline.

### 3. File set

Existing: `scripts/agents/supervision-hook.sh`, `scripts/agents/stop-degraded-forms.sh`, `internal/hooks/testdata/stop-degraded-forms.golden`, `internal/hooks/degraded_stop_forms_test.go`, `scripts/agents/adapters/fake.sh`, `cmd/metasystem/runtime_conformance_test.go`, `testing.json`. No new files.

### 4. Fixtures

Both subtests of TestFakeStopReplay use a `testutil.Fixture` process bed, file faults and the wrapper engine, and no clock.

| Test | Package and file | Fails without the mechanism because |
| --- | --- | --- |
| TestFakeStopReplay/FakeReplayUnreadableHookState | `cmd/metasystem`, `runtime_conformance_test.go` | Cause arming, six runs: no fault, log broken, refusal state broken, both broken, engine missing, engine missing with the log broken. Every response allows. `replay-calls.log` shows `report stop-block` in the first four runs, and the response names the failed append wherever the log is broken, so both writes were attempted before the notice is judged. Only runs four and six carry the unconfirmed text. With approved backlog and no job, a stop under each file fault still refuses for idle backlog. |
| TestFakeStopReplay/FakeReplayDeadlineUnreadable | `cmd/metasystem`, `runtime_conformance_test.go` | Cause deadline through the seam. No fault: the deadline form without the new qualifier, and a condition line with the fixed deadline end. One write broken: no new qualifier. Both broken: the form carries `steward delivery unconfirmed`. With the seam variable set and the owner tag removed, a cause-arming run answers the arming notice. |
| the golden test in `degraded_stop_forms_test.go` | `internal/hooks` | Its name stays. The golden file gains the forms with the new qualifier, and the two shell tables must still agree. |

### 5. Mutations

- FakeReplayUnreadableHookState: add the sentence in `compose_failed_stop` whenever the record fails. Run three goes red.
- FakeReplayDeadlineUnreadable: never request `delivery-unconfirmed` in the deadline parent.
- The same test, the seam: drop the owner-tag check. The last run answers the deadline form.

### 6. Estimate

About 700 changed lines (sentence, qualifier, tables and golden 90, seam 60, replay action and wrapper 170, bed test 370, contract 10). The member is new: its work was inside member 3's 1,450, and the cause injection, the deadline branch and the seam added about 250.

### 7. Landing

The group gains the `TestFakeStopReplay` parent and the inputs `scripts/agents/supervision-hook.sh` and `scripts/agents/stop-degraded-forms.sh` where missing. The hook changes.
Proof packages: `./internal/hooks/ ./cmd/metasystem/`.
Left for 7a: more `stop-replay` causes, and the seam.

## Member 4: stop-frontier-joins-owner-and-revision

One mechanism: one fresh board per stop, scoped to the actor's machine, lineage, goal and revision. Ready work and the WORK IN FLIGHT exemption are both read from it.
Today `readLiveBacklogActivity` in `internal/goal/project.go` emits `job:<id>` for every live job on the checkout, and `HasDelegateJobInFlight` is true for any of them. Another seat's job exempts this seat, and so does a job for an older revision. `Next` scopes held goals by machine only. `queuedFrontier` and `convertedGoalFacts` in `internal/goal/turnverdict.go` read a second, offline projection.
If member 2 lands later, its record stores the board's proof beside the decision.

### 1. DONE

1. Ready work and the WORK IN FLIGHT exemption come from one fresh board scoped to machine, lineage, goal and revision. Only live work that joins the actor's held goal exempts the actor (TestStopFrontierOwnerRevisionAndFreshness).
2. The board carries a freshness proof. A board whose tip or lease epoch changed before the verdict commits is rebuilt once; a second change is rejected. A failed fetch keeps today's unreadable-ledger path (same fixture).
3. The ready goal's next-step text is on the board (same fixture).
4. The idle digest is untouched (TestIdleDigestKeepsEveryNonterminalJob).
5. The idle handoff decisions are unchanged (IdleHandoffRegression: TestIdleBacklogBlocksTwiceThenDefersClaimAndPreparesStewardContinuation, TestIdleEscalationPreservesAnIndependentOpenWorkBlock, TestThreeUnreadableLedgerStopsRecordIncidentRaiseAlarmAndEnd).

### 2. Mechanism

New file `internal/goal/stopboard.go` holds:

- `BoardOwner`: Machine, Lineage, ClaimEpoch.
- `BoardGoal`: Id, Revision, Intent, NextStep.
- `BoardActivity`: Ref (`job:<id>` or `run:<id>`), GoalId, Revision, ClaimEpoch, Joined, Reason.
- `StopBoard`: Owner, Tip, FetchedAt, ProjectedAt, Held, HeldElsewhere, Ready, Activity.
- `(b *StopBoard) HasJoinedWork() bool` and `(b *StopBoard) ExcludeJob(id string)`, which 7c uses.

`readClaimableBudgetedWork` in `internal/goal/project.go` already makes the one fresh projection with `project(endpoint, true, ...)` and calls `Next`. It gains a `BoardOwner` argument and builds the board there. `ClaimableBudgetedWork` gains `Board *StopBoard`.

The owner. `runReportTurnVerdict` does not resolve the owner today. It installs `store.ResolveIdleSeat` (`cmd/metasystem/goal.go:763-777`), and that resolver runs only inside `escalateIdleBacklog` (`internal/goal/turnverdict.go:1285-1295`). This member moves that one call up. `TurnVerdict` calls `s.ResolveIdleSeat` once, before the work read, when the work read will run and `options.SeatActor` is empty. On success it stores the actor and the epoch in `options`; on an error it stores the text in `options.SeatActorProblem`. `escalateIdleBacklog` uses those frozen values and no longer calls the resolver. The board's owner is the frozen actor and epoch.

Scoping. `Next` stays machine-only, because `goal next` prints from it. The board filters its result. A claimed goal whose `ClaimRecord.Lineage` equals the owner's lineage is Held. A goal the same machine claimed under another lineage is HeldElsewhere: neither held nor ready for this actor. Ready goals copy Id, Revision and NextStep from the goal file. With an unresolved owner the board has an empty lineage and every claim is HeldElsewhere, the honest reading of an unidentified seat.

The join. `internal/goal` cannot import `internal/dispatch`, because dispatch imports goal (`internal/dispatch/admission.go:3-13`). The join reads the job record through goal's own lens. `backlogJobRecord` (`project.go:453`) gains GoalRevision, ClaimEpoch and MachineID from the JSON keys `goalRevision`, `claimEpoch` and `machineId`, the keys `dispatch.JobRecord` reads (`internal/dispatch/jobrecord.go:67-68`, `98-108`). The lens is permissive: a missing key, or a value that is not a whole non-negative number, reads as missing and never as an error. A live job joins when its goal is in Held, its machine is the owner's, its revision equals the held goal's `Revision`, and its claim epoch equals the claim's `ClaimEpoch`. The claim names the lineage and the epoch names that one claim. A live governed run joins the same way: the lister is `run.Store.List` (`internal/run/waiter.go:1881-1902`), and the record gives `GoalId`, `OwnerLineage`, `ClaimEpoch` and `Governed.GoalRevision` (`internal/run/run.go:123-146`, `172-195`). A record that lacks a coordinate still reads and is listed, does not join, and its Reason names the missing coordinate. `readLiveBacklogActivity` returns the activity with these coordinates and leaves the decision to the board.

`HasDelegateJobInFlight` becomes `work.Board != nil && work.Board.HasJoinedWork()`. `InFlight` keeps every activity for display. `NonTerminalJobs` and `idleBacklogDigest` are not touched: the digest still hashes every non-terminal job, whoever owns it.

Freshness. The board records the accepted tip from `Projection.Tip`, the fetch and projection times from the injected clock, and the owner. `TurnVerdictOptions` gains `RecheckBoard func() (tip string, claimEpoch int64, err error)`. The command wires it to `goal.AcceptedTip(root)`, a new exported read-only wrapper around `acceptedTipForGates` (`internal/goal/txn.go:152-188`), with no fetch, and to a fresh `resolveSeatIdleActor`. There are at most two checks and one rebuild:

1. Check one runs immediately before `saveVerdictState` (`turnverdict.go:554-568`) and compares with the first board's Tip and `Owner.ClaimEpoch`.
2. If either differs, the board is rebuilt once from a fresh projection and the decision is made again, from a copy of the session state taken before the first decision. The first decision leaves nothing behind.
3. Check two runs before the second decision is saved and compares with the second board. If it differs again, or a check returns an error, the verdict is the infrastructure class with cause `stop-board-stale`. By member 1's rule that allows the stop, and no idle count is spent.

A failed fetch is unchanged: `readClaimableBudgetedWork` returns its error, `decide` reports the ledger unreadable, and no board is built offline.

One board. `queuedFrontier` and `convertedGoalFacts` take the board when the work read ran, and answer from `Held`, `Ready` and the tree the board was built from. They keep their offline `Project(endpoint, false, ...)` read only where the work read is skipped: the human-authorised stop and the brain seat, where nothing is refused for backlog.
`freezeTurnVerdictFacts` in `internal/goal/turnfacts.go` adds the board's owner, tip and times to the frozen facts as a new optional field.

Not changed: `Next`, `SelectNext`, `goal next` output, the idle digest, the refusal bound of three, `registeredWaits`, and every read rule on job and run records.

### 3. File set

Existing: `internal/goal/project.go`, `internal/goal/turnverdict.go`, `internal/goal/turnfacts.go`, `internal/goal/txn.go`, `cmd/metasystem/goal.go`, `internal/goal/turnverdict_world_test.go`, `internal/goal/turnverdict_idle_test.go`, `testing.json`.
New: `internal/goal/stopboard.go`, `internal/goal/stopboard_test.go`.

### 4. Fixtures

All use the store's clock and a fake `identity.Prober`. The first also uses a fake `ResolveIdleSeat`, the `fetch` field of `projectionDependencies` and a fake `RecheckBoard` that answers per call.

| Test | Package and file | Fails without the mechanism because |
| --- | --- | --- |
| TestStopFrontierOwnerRevisionAndFreshness | `internal/goal`, `turnverdict_world_test.go` | Subtests. Another machine's live job does not exempt this actor. Another lineage's job on the same machine does not. A job for the old revision or an old claim epoch does not. A joined job and a joined governed run each do. A failing owner resolver gives an empty-lineage board and no exemption. A ready goal's NextStep is on the board. A new tip at check one and the second board's tip at check two give a rebuilt board, a decision, and session state showing one decision. A new tip at both checks gives `stop-board-stale`, no block and no count. A failing fetch gives the unreadable-ledger verdict and a nil board. |
| TestIdleDigestKeepsEveryNonterminalJob | `internal/goal`, `turnverdict_idle_test.go` | Another seat's job appears and ends between two idle stops. The digest must differ and `IdleBlocks` must restart at one, as today. |
| TestBoardJoinReadsOlderRecords | `internal/goal`, `stopboard_test.go` | A job record with no revision and no claim epoch, and one whose revision is not a whole number, read without error, are listed with a Reason, and do not join. |
| IdleHandoffRegression (three tests, names above) | `internal/goal`, `turnverdict_idle_test.go` | Their decisions stay. Bodies change only where a fixture job needs the joining coordinates or a fake `ResolveIdleSeat`. |
| TestClaudeTurnExitRequiresDelegateWorkNotMerelyALiveSeat | `internal/goal`, `turnverdict_idle_test.go` | The name stays. Its job record gains the held goal's id, revision and claim epoch so that it still exempts. |

### 5. Mutations

- TestStopFrontierOwnerRevisionAndFreshness: drop the claim-epoch comparison from the join. The old-epoch subtest goes red.
- The same test, freshness: skip check two in `TurnVerdict`. The stale-twice subtest commits a decision.
- The same test, owner: build the board from the zero `options.SeatActor` without the resolver. The joined-job subtest loses its exemption.
- TestIdleDigestKeepsEveryNonterminalJob: hash only the joined jobs in `idleBacklogDigest`.
- TestBoardJoinReadsOlderRecords: make a missing coordinate an error in `readLiveBacklogActivity`.
- IdleHandoffRegression: make `HasJoinedWork` always true. The blocks-twice test sees an exemption where it expects its first refusal.

### 6. Estimate

About 1,250 changed lines (board, lens and construction 360, owner, checks and facts wiring 250, tests 630, contract 10). The early owner resolution, the second check with its state copy and the `AcceptedTip` wrapper moved it up from 1,100.

### 7. Landing

The group already covers `internal/goal/**`. The engine changes.
Proof packages: `./internal/goal/ ./cmd/metasystem/`.
Left for later: member 5 prints from `Board.Ready` and `Board.Owner`; 7c calls `ExcludeJob`.

## Member 5: stop-refusals-name-seat-actions

One mechanism: a refusal is emitted only with one command that runs as printed. A refusal that cannot name one becomes a notice.
`internal/goal/turnfacts.go` has the pieces. `TurnAction` carries Command, Restriction and HumanRequired. `scanActions` renders `watch-job` and `watch-run` commands. `workActions` renders `continue-goal` with text only, and `claim-goal` with `metasystem goal next --machine <m> --fetch`, a verb that claims whichever goal comes first and names neither goal nor lineage. `open-plan` and the human items carry no command. The block sources are `open-work`, `goal`, `unwatched-work`, `uncertainty` and `idle-backlog`.
If member 2 lands later, its record stores the chosen command.

### 1. DONE

1. An eligible seat refusal prints exactly one executable clearing command (TestStopClearingCommand).
2. A seat refusal with no lawful command becomes a notice: a missing command, a human-only remedy or a text-only next step. The stop is allowed and the text is still shown (TestStopClearingCommand).
3. The mandatory idle refusal prints the real claim command and the ready goal's next step, both from the board (TestIdleClaimCommandIsFirstAct).
4. The idle decisions stay mandatory: two refusals, then the handoff (IdleHandoffRegression, the three tests named in member 4).

### 2. Mechanism

`engineCommand(root string, args ...string) string` is new in `internal/goal/turnfacts.go`. It renders `bin/metasystem` and the arguments, each through `shellArgument`. Every action command uses it, so all commands have one form and run from the checkout root. The `--caller-pid $$` tail of the job watch stays unquoted, because the shell must expand it.

The actions come first. Today they are built only inside `freezeTurnVerdictFacts` (`turnfacts.go:88-116`), which runs after the state is saved (`turnverdict.go:628-630`). A new helper `turnActions(root, scan, work, workRead, workErr, mainID, options, watches) []TurnAction` holds the lines that build them: the scan clone, `classifyOwnership`, `workActions` and `scanActions`. `TurnVerdict` calls it once, after `decide` and `decideRuns`, and hands the same slice to `requireClearingCommand` and to `freezeTurnVerdictFacts`, which no longer builds its own. Nothing is frozen early.

`clearingAction(source string, actions []TurnAction) (TurnAction, bool)` is new in the same file. An action is lawful when Command is not empty, HumanRequired is false and Restriction is empty. `unwatched-work` takes `watch-job` or `watch-run`; `open-work` takes `open-plan`; `goal` and `uncertainty` have none. The first lawful action in the existing sort order wins.

`(s *Store) requireClearingCommand(verdict *Verdict, actions []TurnAction)` is new in `internal/goal/turnverdict.go`, called before `saveVerdictState`. It acts only when ShouldBlock is true, Class is `seat-actionable` and IdleRefusal is false. With a lawful action the display gains a last line `RUN: <command>`, and `Verdict` gains `ClearingCommand string`. With none, ShouldBlock becomes false, the new field `NoticeSource` keeps the old block source, the display starts with `NOTICE:`, and one diagnostic says the refusal had no lawful command. The session slots (`BlockedGoalRevisions`, `BlockedFreeDigests`, `BlockedQueueDigests`, `BlockedUnwatchedDigests`) are written as for a block, so a notice shows once per digest.

The goal-free branch blocks today with "declare a goal or renew with `goal declare-free`". It becomes a notice: the synced route of that verb requires `--digest` and a lineage (`cmd/metasystem/goalsync_mutations.go:1438-1443`, `604-648`), which the seat must supply, so no printed command runs as it stands. `continue-goal` and `open-plan` have text only, so their refusals become notices too. A text-only next step cannot force a turn.

The idle path. In `workActions` the `claim-goal` command becomes:

```text
bin/metasystem goal claim --root <root> --id <goal> --lineage <lineage>
```

The goal is `Board.Ready[0]` and the lineage is `Board.Owner.Lineage`. The verb is `runGoalClaim` (`goalsync_mutations.go:2694-2710`). It requires `id`, the common parser accepts `root` and `lineage`, and holder authentication still applies. `idleBacklogContinuation` prints that command and the ready goal's NextStep quoted with `%q`, in place of the `goal next` sentence. With an unknown lineage the refusal keeps its decision, prints `SeatActorProblem` in place of a command, and fabricates nothing. `requireClearingCommand` never demotes an idle refusal.

Not changed: which conditions are observed, the idle digest and bound, `escalateIdleBacklog`, `watchLine` in `cmd/metasystem/run.go`, and the hook, which already relays the display.

### 3. File set

Existing: `internal/goal/turnfacts.go`, `internal/goal/turnverdict.go`, `internal/goal/turnverdict_test.go`, `internal/goal/turnverdict_idle_test.go`, `cmd/metasystem/goal_test.go`, `docs/design/turn-verdict-delivery-contract.md` (the refusal text it quotes), `testing.json`.
New: `cmd/metasystem/engine_binary_test.go`. No helper builds the engine for `cmd/metasystem` tests today: `testutil.InstalledWaitBinary` only copies a built candidate (`internal/testutil/wait_binary.go:10-54`), and tests run `go build` themselves (`cmd/metasystem/context_cost_test.go:379`). The file holds `builtEngine(t)`, which builds `bin/metasystem` once per test process into a temp directory.

### 4. Fixtures

The `internal/goal` tests use the store's clock and a prober. The `cmd/metasystem` tests use `testutil.Fixture` with `Env` and `Record`, a temp ledger endpoint, and no sleep.

| Test | Package and file | Fails without the mechanism because |
| --- | --- | --- |
| TestStopClearingCommand | `internal/goal`, `turnverdict_test.go` | One subtest per block source. Unwatched job and unwatched run: the verdict blocks, the display ends with one `RUN:` line, and `ClearingCommand` equals the action's command. Open work with text only, a human-required item, the goal-free branch and `uncertainty`: ShouldBlock is false, NoticeSource is set, and a second stop with the same digest shows nothing new. Idle backlog with the same text-only goal still blocks. |
| TestStopClearingCommandsRun | `cmd/metasystem`, `goal_test.go` | Each lawful command the verdict emits is run as printed, through `sh -c` from an isolated bed's root, with `builtEngine`. The watch command prints no readiness line (`cmd/metasystem/run.go:458-486`). Its readiness is the wait id written to the inherited descriptor named by `METASYSTEM_WAIT_REGISTERED_FD` (`cmd/metasystem/wait_verb.go:35-59`). The test passes a pipe's write end through `ExtraFiles` with the variable set to 3 and reads the wait id from the read end, as `cmd/metasystem/wait_verb_test.go:1524-1545` does. Then the next verdict does not block for that digest. The claim command exits zero. A renamed flag or a wrong quote makes it red. |
| TestIdleClaimCommandIsFirstAct | `cmd/metasystem`, `goal_test.go` | An approved ready goal has no brief and no job. The refusal names the real goal and lineage, quotes the next step, and prints the claim command. Running it as the seat claims that goal under that lineage. No dispatch appears, no human-stop command is offered, and the next stop is not exempt merely because the goal is now claimed. |
| IdleHandoffRegression (three tests) | `internal/goal`, `turnverdict_idle_test.go` | Decisions and counts stay. Only the asserted refusal text changes to the claim command. |
| TestIdleRefusalSurvivesALostCounter | `internal/goal`, `turnverdict_idle_test.go` | The name stays. Its asserted command becomes the claim command. |

### 5. Mutations

- TestStopClearingCommand: return before the demotion in `requireClearingCommand`.
- TestStopClearingCommandsRun: rename `--job` to `--id` in the watch-job command in `scanActions`.
- TestIdleClaimCommandIsFirstAct: restore the `goal next --machine` command in `workActions`.
- IdleHandoffRegression: drop the IdleRefusal guard in `requireClearingCommand`. `clearingAction` has no kind for `idle-backlog`, so both idle refusals are demoted.
- TestIdleRefusalSurvivesALostCounter: stop printing the command in `idleBacklogContinuation` when `CountSpent` is false.

### 6. Estimate

About 1,050 changed lines (mechanism 290, tests 670, build helper 50, document and contract 40). The descriptor readiness, the build helper and the `turnActions` move took it up from 900.

### 7. Landing

Update the refusal text quoted in `docs/design/turn-verdict-delivery-contract.md`; its universal-fallback section waits for 7a. The engine changes.
Proof packages: `./internal/goal/ ./cmd/metasystem/`.
Left for 7a: `Verdict.ClearingCommand` and `Verdict.NoticeSource`, which the gate copies into its response.

## Member 6: stop-fences-surface-once

One mechanism: a per-session memory of the fences already shown.
In `decide` in `internal/goal/turnverdict.go`, the `ok` branch leaves through two exits before `FencedClaimLines(work.fencedClaims)` is appended: the registered-wait break (`turnverdict.go:1700-1706`) and the delegate-job break (`1708-1710`). The append is at `1730-1732`. A fenced goal held beside live work is therefore never shown, and on every other path the FENCED lines print at every stop. The `queued-only` branch appends the same lines.
If member 2 lands later, nothing changes: this member reads only goal files and the per-session verdict state.

### 1. DONE

1. A held fenced goal prints FENCED once per unchanged fence. New hook generations and repeated stops do not print it again.
2. It prints beside live work too.
3. A changed fence prints a new line. The fenced goal is never continued.

All three fail TestFencedClaimSurfacedOnceBesideFlight.

### 2. Mechanism

`sessionState` gains `SurfacedFences []string` with `json:"surfacedFences,omitempty"`, capped by a new constant `maxSurfacedFences = 16`, oldest dropped first as the other slots do.

`fencedClaimFingerprint(f *GoalFile) string` is new. It hashes the goal id, the `StopFence` fields StopID, Revision, Epoch, CapabilityGeneration, ClosedAt and Reason (`internal/goal/file.go:391-398`), and `f.StopCapability.FenceEpoch` when the goal file has a capability (`file.go:78`, `381-387`). `ClaimRecord` has no fence epoch. The hook generation is not in the fingerprint.

`(s *Store) surfaceFencedClaims(session *sessionState, claims []*GoalFile) []string` is new. It takes the slice `FencedClaimLines` takes today (`work.fencedClaims`, `internal/goal/project.go:223-235`). It returns the lines of the claims whose fingerprint the session has not stored, stores those fingerprints, and drops stored fingerprints whose fence is no longer held.

In `decide`, the call moves to the top of the `ok` branch, before both exits. The lines it returns are appended on every exit path, including both WORK IN FLIGHT exits. The `queued-only` branch calls the same function. `OnlyFencedClaim` still keeps the fenced goal out of the continuation, and no `continue-goal` action names it.
If the state write fails, the line can print once more at the next stop. A repeat is acceptable; a loss is not.

Not changed: `FencedClaimLines`, `runGoalNext` (a query prints every fence every time), `IsFencedClaim`, the stop-fence records, and TestTurnVerdictDoesNotBlockAndDescribesTheDurableStopPhase. That test writes the checkout-wide stop fence, and `TurnVerdict` returns from it before it reads goal work or session state (`turnverdict.go:437-463`), so it never reaches this mechanism.

### 3. File set

Existing: `internal/goal/turnverdict.go`, `internal/goal/turnverdict_stopfence_test.go`, `testing.json`. No new files.

### 4. Fixtures

| Test | Package and file | Fails without the mechanism because |
| --- | --- | --- |
| TestFencedClaimSurfacedOnceBesideFlight | `internal/goal`, `turnverdict_stopfence_test.go` | A fenced goal, with a per-goal `StopFence` on its goal file, is held beside a working goal. With a joined live job the first stop shows one FENCED line beside WORK IN FLIGHT. The second and third stops, each with a new hook generation, show none. Then three subtests change one coordinate each from the same stored state: Epoch alone, Reason alone, the capability's FenceEpoch alone. Each shows one new line. The same sequence runs without live work. No continuation or action ever names the fenced goal. Seams: the store's clock and a prober. |

### 5. Mutations

- Once: do not store the fingerprint in `surfaceFencedClaims`.
- Beside flight: move the call back below the WORK IN FLIGHT exits.
- Change: remove Epoch from `fencedClaimFingerprint`. The Epoch-alone subtest goes red.
- Change: remove Reason. The Reason-alone subtest goes red.

### 6. Estimate

About 360 changed lines (mechanism 110, tests 245, contract 5). One-coordinate subtests replaced the combined change and the checkout-stop test left the table; it was 350. It is below the expected range because the mechanism is small. It stays its own member because it has its own DONE, and folding it into member 5 would put two mechanisms in one landing.

### 7. Landing

The engine changes. Proof packages: `./internal/goal/ ./cmd/metasystem/`. It leaves nothing for a later member.

## Member 7a: stop-hosts-enforce-the-common-gate

One mechanism: one Go verb decides a stop, commits it and writes its decision line, and a seat's native Stop hook enforces what it answers.
Revision 1 held four mechanisms under this name. The gate stays here; the count reader is 7b, the delegate rounds are 7c and the live proof is 7d. The umbrella's DONE for its member 7 is met when 7d lands.
If member 2 lands later, the gate commits its version-2 record before it answers, and `stateSaved` means that commit.

### 1. DONE

1. One verb, `host stop-gate`, computes the stop decision, commits it and answers in one response shape. `report turn-verdict` stays and calls the same service (TestStopGateAnswersOnlyCommittedBlocks).
2. A seat's native Stop hook calls the gate, enforces its `decision`, and allows when the gate cannot be run (TestEveryRuntimeUsesStopDecision, TestFakeStopReplay).
3. Infrastructure never asks for another turn, and a repeat inside one deadline causes no new notification identity (FakeReplayNarrator, FakeReplayDeadline, FakeReplayArming, FakeReplayIdleWithInfrastructure).
4. Every stop whose append succeeds leaves one decision line in the hook log. A stop whose append fails leaves a refusal-record entry with cause `stop-decision-unwritten`, which 3a's drain delivers (TestStopGateAnswersOnlyCommittedBlocks).
5. A stop boundary is declared per runtime and host mode. A declared boundary whose transport does not call the gate fails the test; an undeclared one is reported unproven, never certified (TestEveryRuntimeUsesStopDecision).

### 2. Mechanism

The gate. `runHostStopGate` is new in `cmd/metasystem/host_verbs.go`, registered in the host family in `cmd/metasystem/main.go` beside `result-write` and `finish`. It takes every flag `runReportTurnVerdict` takes, plus `--boundary` (`native-stop` or `delegate-round`), `--generation`, `--deadline-end`, `--exclude-job` and `--refusal-record`. The collection and decision code moves out of `runReportTurnVerdict` into one function, `stopDecision`, in `cmd/metasystem/goal.go`, which both verbs call. `--runtime` stays an opaque token: the engine writes it down and never branches on it.

The response is one JSON object: `schemaVersion`, `decision` (`allow` or `block`), `class`, `stateSaved`, `lineAppended`, `committed`, `reason`, `systemMessage`, `command`, `noticeSource`, `generation`, `deadlineEnd`, and `verdict` holding the full `Verdict`. `stateSaved` says `saveVerdictState` succeeded, `lineAppended` says the decision line was written, and `committed` is both. The hook enforces `decision` alone, which follows three rules:

1. An infrastructure verdict answers `allow`.
2. An idle refusal answers `block` whenever the verdict blocks, committed or not. This is member 1's rule: a lost state write keeps that block and sets `CountSpent` false (`internal/goal/turnverdict.go:554-561`). A lost line is treated the same way, and the reason says what was lost.
3. Any other block answers `block` only when `committed` is true. Otherwise it answers `allow`, the display is shown as a notice, and the line's outcome is `unconfirmed`.

The decision line. `internal/stopincident` gains `Decision`, `AppendDecision` and `ReadDecisions`. The gate appends one line per stop:

```text
stop-decision <epoch> <machine> <session-slug> <boundary> <runtime> <class> <outcome> <generation|-> <deadline_end|->
```

The outcome is `allow`, `block`, `notice` or `unconfirmed`. The machine comes from `ResolveMachine`. The hook's older lines are still written and still read.
Hook-log writes are already allowed to fail (`supervision-hook.sh:2589-2591`). When `AppendDecision` fails, the gate calls `report.StopRefusal` on the `--refusal-record` path with class infrastructure, cause `stop-decision-unwritten`, component `hook-log` and the stop's generation and deadline end. 3a's drain reads both sources on every tick, so the steward hears of it even while the log is readable but not writable. If that write fails too, `systemMessage` carries 3b's unconfirmed sentence.

The hook. In `scripts/agents/supervision-hook.sh`, `report_turn_verdict()` calls `host stop-gate --boundary native-stop` with `hook_generation`, `stop_started_epoch + 60` and the refusal-record path. The response builder takes `decision`, `systemMessage` and `reason` from the gate and no longer derives a block itself. A gate that cannot be run, or whose answer cannot be read, gives an allow with the degraded notice.

The fake legs. The `stop-replay` action gains `--cause narrator` and an idle leg, and `TestFakeStopReplay` gains the four remaining subtests, including the steward tick that starts the guarded continuation on the third idle stop. FakeReplayDeadline uses 3b's expiry seam, so no leg waits for a real deadline.

The registry. `runtimes.Declaration` in `internal/runtimes/runtimes.go` gains `SeatStopBoundary` and `RoundStopBoundary`, one per host mode: an interactive seat and a headless delegate round. The values are `native-stop`, `delegate-round` or empty. This member declares one boundary, the claude seat's `native-stop`. Every other boundary stays empty and is reported unproven until 7c declares the rounds. A declaration is never proof that a provider loaded a hook; 7d is that proof.

Not changed: the verdict rules of members 1 to 6, `adapter adjudicate-turn` in `internal/adapter/adjudicate.go`, the adapters, and the managed-seat host scripts.

### 3. File set

Existing: `cmd/metasystem/host_verbs.go`, `cmd/metasystem/main.go`, `cmd/metasystem/goal.go`, `cmd/metasystem/runtime_conformance_test.go`, `cmd/metasystem/goal_test.go`, `internal/stopincident/stopincident.go`, `internal/stopincident/stopincident_test.go`, `internal/runtimes/runtimes.go`, `scripts/agents/supervision-hook.sh`, `scripts/agents/supervision-hook-fixtures.sh` (its expectations of a first infrastructure block), `scripts/agents/adapters/fake.sh`, `docs/design/turn-verdict-delivery-contract.md`, `testing.json`. No new files.

### 4. Fixtures

| Test | Package and file | Fails without the mechanism because |
| --- | --- | --- |
| TestEveryRuntimeUsesStopDecision | `cmd/metasystem`, `runtime_conformance_test.go` | It walks `runtimes.All()` and both host modes. A declared `native-stop` boundary's hook transport must call `host stop-gate`, must answer what the gate's `decision` says, and must allow when the gate cannot be run. An undeclared boundary is printed as unproven. An infrastructure verdict never yields `block`. Seams: a fake engine on `METASYSTEM_BIN` that records its arguments, `testutil.Fixture`. |
| TestStopGateAnswersOnlyCommittedBlocks | `cmd/metasystem`, `goal_test.go` | A clean stop leaves exactly one decision line, and `ReadDecisions` gives back the response's class, outcome, boundary, generation and deadline end. State write faulted: a seat-actionable block answers `allow` with `stateSaved` false and outcome `unconfirmed`; an idle refusal answers `block` and says the count was not spent. Line append faulted: a seat-actionable block answers `allow`, an idle refusal answers `block`, and the refusal record gains `stop-decision-unwritten` with the stop's identity. Both verbs return the same verdict for the same inputs. Seams: clock, unwritable temp paths. |
| TestFakeStopReplay: FakeReplayNarrator, FakeReplayDeadline, FakeReplayArming, FakeReplayIdleWithInfrastructure | `cmd/metasystem`, `runtime_conformance_test.go` | Each recorded cause allows the stop and writes one `stop-decision allow` line, and a repeat with the same generation and deadline end queues no second `stop-incident-` nonce. With approved backlog and no job, stops one and two block, stop three stages the continuation and allows, a steward tick starts it, and a failed dispatch raises the existing alarm. Seams: process bed, 3b's wrapper engine and seam, `TickConfig.Now`, `deliverNotification` fake, file faults. |

### 5. Mutations

- TestEveryRuntimeUsesStopDecision: call `report turn-verdict` again from `report_turn_verdict()`.
- TestStopGateAnswersOnlyCommittedBlocks: set `decision` from `ShouldBlock` alone in `runHostStopGate`.
- The same test, the writer: remove the `AppendDecision` call. The clean-stop subtest finds no line.
- TestFakeStopReplay: append the decision line only when the verdict blocks.

### 6. Estimate

About 1,400 changed lines (gate, shared service and response rules 340, decision line and fallback record 170, hook 120, registry and conformance test 260, four fake legs 390, document and contract 120). The count left for 7b, and the two commit facts, the unwritten-line record and the per-mode registry came in; it was 1,400.

### 7. Landing

The `TestFakeStopReplay` parent is already in the group. Replace the section "The universal fallback (no hooks required)" in `docs/design/turn-verdict-delivery-contract.md`, which still sends a seat to `goal next --machine`, with the gate and its boundaries. Retire the first-infrastructure-block expectations in `scripts/agents/supervision-hook-fixtures.sh`.
Engine and hook change. The seat records the date on which every seat runs the new hook; member 8 counts from it.
Proof packages: `./internal/stopincident/ ./internal/runtimes/ ./internal/goal/ ./cmd/metasystem/`.
Left for later: `ReadDecisions` for 7b; the gate verb and the registry fields for 7c.

## Member 7b: stop-decisions-count-per-seat-and-day

One mechanism: a reader that counts decision lines per seat and local day. It changes no stop.
If member 2 lands later, the residual also reads its version-2 records.

### 1. DONE

1. `report stop-count` prints, per seat and local day, the refusals and the residual beside them, from retained records only. The per-session verdict file is never a source (TestDailyRefusalCountReadsTheLogStream).

### 2. Mechanism

`DailyRefusalCounts(path string, from, to time.Time, zone *time.Location) []SeatDayCount` is new in `internal/report/stopcount.go`. It reads the log through `stopincident.ReadDecisions` and `ReadHookLog`. Per machine and local day it counts the `block` lines, and as the residual the `unconfirmed` lines and the infrastructure `stop-condition` lines with no `stop-incident-ack` line. A new verb `report stop-count --root . --days 7` in `cmd/metasystem/report.go` prints one row per seat and day. It adds to the residual the refusal-record incidents with cause `stop-decision-unwritten`, by the local day of their deadline end. Older line kinds are ignored and never fail the read.

### 3. File set

Existing: `cmd/metasystem/report.go`, `internal/report/stopblock_test.go`, `testing.json`. New: `internal/report/stopcount.go`.

### 4. Fixtures

| Test | Package and file | Fails without the mechanism because |
| --- | --- | --- |
| TestDailyRefusalCountReadsTheLogStream | `internal/report`, `stopblock_test.go` | A synthetic week of decision lines from two seats counts per seat and local day. A line just before local midnight lands on its own day. One line is written by `AppendDecision` itself, so reader and writer cannot drift. Replacing the per-session verdict file changes no count. Older line kinds are ignored. Seams: the `from`, `to` and `zone` arguments. |

### 5. Mutations

- TestDailyRefusalCountReadsTheLogStream: group by UTC day in `DailyRefusalCounts`.

### 6. Estimate

About 300 changed lines (reader 90, verb 50, tests 150, contract 10). The member is new; its work was inside 7a's 1,400.

### 7. Landing

The group already covers `internal/report/**`. It lands any time after 7a. The engine changes. Proof packages: `./internal/report/ ./cmd/metasystem/`. It leaves the verb for member 8.

## Member 7c: delegate-rounds-call-the-stop-gate

One mechanism: a delegate round's turn boundary calls the gate, and a committed refusal buys one more provider turn in the same job.
This covers Codex rounds and headless Claude rounds. The umbrella's Landing section says the claude adapter's rounds keep their native hook. They have none: `BuildClaudeCommand` emits `claude -p` (`internal/adapter/claude.go:391-397`), the repository's Stop entry is the project hook only (`scripts/enforcement/claude-code-hooks.json:28-35`), and on 2026-09-19 the seat saw a headless run invoke only a plugin Stop hook and leave no line in the repository's hook log. So a Claude round is gated where a Codex round is: in the adapter.
If member 2 lands later, nothing changes here.

### 1. DONE

1. The turn boundary of a Codex round and of a headless Claude round calls the gate with the round's own job excluded (TestCodexRoundHonoursTheGate, TestClaudeRoundHonoursTheGate).
2. On a `block` the adapter re-invokes the provider once in the same job under the remaining cap. A second refusal ends the round with the refusal recorded as its gap (same fixtures).
3. Both runtimes declare `RoundStopBoundary` and the conformance test holds them to it (TestEveryRuntimeUsesStopDecision).

### 2. Mechanism

Both adapters have the same shape: `settle_result_identity`, then `complete_from_cli` (`scripts/agents/adapters/codex.sh:178-179`, `scripts/agents/adapters/claude.sh:191-192`). The boundary sits between the two. `gate_turn_boundary` is new in `scripts/agents/adapters/runtime-common.sh`. It calls `host stop-gate --boundary delegate-round --exclude-job <job id>`. On `allow` it returns. On `block`, when the job record has no `gateTurn` and the remaining cap allows a turn, it records the gate turn and calls the adapter's `runtime_gate_turn`. Then the identity is settled again and the gate is called again. A second block is not enforced: the round completes, and the refusal is written to `stop-gate.json` in the round directory and into the terminal patch as the round's gap.

The gate-turn record. `gateTurn` is a new job-record field: an object with `at`, `boundary` and `reason`. It is written once through `__record-cas` in `scripts/agents/dispatch.sh` as a self-target update on the running record, which `RecordCAS` treats as a metadata change (`internal/dispatch/record.go:542-549`). It is not `continuation`. That field is immutable, is written at create time (`record.go:78-100`, `internal/dispatch/build.go:973-975`) and marks a new follow-up job; a gate turn is no new job and no new budget.

`runtime_gate_turn` is new in both adapters. In `codex.sh` it builds the follow-up through `build_codex_command`, which emits `codex exec resume` (`codex.sh:97-119`, `internal/adapter/codex.go:87-110`). In `claude.sh` it builds it through `adapter claude-command --session`, as the follow-up verb does (`claude.sh:129-130`). The prompt is the gate's reason and command, and it waits through `wait_for_cli`. Only `devin.sh` defines `runtime_repair_turn`; the gate turn is a separate function. Devin declares no round boundary and stays unproven.

`TurnVerdictOptions` gains `ExcludeJob string`. The board calls `ExcludeJob` and the scan drops the same id from the owned jobs, so a round can neither exempt nor block itself.

Not changed: the gate, the native hook, cap continuation, and `complete_from_cli`.

### 3. File set

Existing: `scripts/agents/adapters/codex.sh`, `scripts/agents/adapters/claude.sh`, `scripts/agents/adapters/runtime-common.sh`, `scripts/agents/dispatch.sh`, `internal/dispatch/record.go`, `internal/goal/turnverdict.go`, `internal/goal/stopboard.go`, `internal/runtimes/runtimes.go`, `cmd/metasystem/runtime_conformance_test.go`, `testing.json`. No new files.

### 4. Fixtures

| Test | Package and file | Fails without the mechanism because |
| --- | --- | --- |
| TestCodexRoundHonoursTheGate, TestClaudeRoundHonoursTheGate | `cmd/metasystem`, `runtime_conformance_test.go` | One helper, run per adapter, with a fake provider binary on `PATH` and a fake gate on `METASYSTEM_BIN`; both record their arguments and exit. Block then allow gives exactly one follow-up invocation, a `gateTurn` on the record and a normal completion. Block twice gives one follow-up, a completed round and `stop-gate.json` naming the refusal. An infrastructure answer gives no follow-up. The gate's arguments carry `--exclude-job` with the round's own job. No remaining cap gives no follow-up. The adapter run is awaited to its exit under `testutil.Fixture`; nothing is polled. |
| TestEveryRuntimeUsesStopDecision (extended) | `cmd/metasystem`, `runtime_conformance_test.go` | A declared `delegate-round` boundary's adapter must call `gate_turn_boundary` between identity and completion. |

### 5. Mutations

- TestCodexRoundHonoursTheGate: drop the `gateTurn` check in `gate_turn_boundary`. The block-twice case resumes twice.
- TestClaudeRoundHonoursTheGate: remove the `gate_turn_boundary` call in `claude.sh`.
- TestEveryRuntimeUsesStopDecision: declare devin's `RoundStopBoundary` without wiring its adapter.

### 6. Estimate

About 1,150 changed lines (boundary and gate turn for two adapters 340, `gateTurn` record 90, `ExcludeJob` 60, declarations and conformance extension 80, the two fake-provider tests 560, contract 20). It was revision 1's 7b at 1,200; the live legs left for 7d and the Claude round came in.

### 7. Landing

Engine and adapters change. Proof packages: `./internal/runtimes/ ./internal/goal/ ./internal/dispatch/ ./cmd/metasystem/`. Left for 7d: the live observation of both round boundaries.

## Member 7d: stop-boundaries-proven-live

One mechanism: a live replay that observes each declared boundary calling the gate, with real providers, and keeps the evidence.
`testing.json` cannot hold this proof. Its kinds are build, integration, performance, static and unit, and its cadence lists sections only (`testing.json:112`). So the proof is not a group.
If member 2 lands later, the evidence also keeps its records.

### 1. DONE

1. Each declared boundary is observed live once: the Claude seat's native hook, a headless Claude round and a Codex round. For each, a narrator read failure, a deadline expiry and an arming failure cause no reprompt, and approved backlog with no job gives the idle refusal and then the third-call handoff (TestRealStopReplay).
2. The evidence is retained and its path is recorded on the umbrella page.

### 2. Mechanism

`cmd/metasystem/runtime_conformance_live_test.go` carries the build tag `livestop`, so no ordinary run compiles it. It holds TestRealStopReplay with the subtests RealClaudeNativeStopReplay, RealClaudeRoundStopReplay and RealCodexAdapterStopReplay.
`scripts/agents/stop-live-replay.sh` runs it. The seat runs the script once on the orchestrator after 7c, and again whenever a boundary's transport changes. The script first asks each adapter's `probe` action whether its provider answers. If one does not, it exits 2, names the provider and starts no test. Inside the test a missing provider is a failure, never a skip.

Readiness. Every bed puts a wrapper engine on `METASYSTEM_BIN`. The wrapper runs the real `host stop-gate` and then writes the response as one line to the named pipe `gate-calls.fifo` in the bed. The test waits on two facts at once: a line on that pipe, or the exit of the provider or adapter process. A process that exits with no gate call fails the leg at once. Nothing is polled and nothing sleeps; `go test -timeout` is only the hang bound.
The causes are injected as in the fake legs: the wrapper fails `up`, fails the narrator read, or blocks for 3b's expiry seam. The round legs dispatch through `scripts/agents/dispatch.sh` with the real adapters. The seat leg starts an interactive Claude session in the bed.
The evidence goes to `artifacts/agents/supervision/stop-live-replay/<UTC stamp>/`: the transcripts, the gate responses, the decision lines and the steward receipt.

### 3. File set

New: `cmd/metasystem/runtime_conformance_live_test.go`, `scripts/agents/stop-live-replay.sh`. Existing: `docs/design/turn-verdict-delivery-contract.md` (one paragraph naming the script and the evidence path).

### 4. Fixtures

| Test | Package and file | Fails without the mechanism because |
| --- | --- | --- |
| TestRealStopReplay (tag `livestop`) | `cmd/metasystem`, `runtime_conformance_live_test.go` | The three subtests above. Each reads one gate response per stop from the pipe and checks its decision against the leg. Seams: real providers, the wrapper engine, the named pipe, process exit. |

### 5. Mutations

- TestRealStopReplay: remove the `gate_turn_boundary` call in `codex.sh`. The Codex idle leg sees the adapter exit with no gate call.

### 6. Estimate

About 600 changed lines (live test 400, script 90, wrapper 80, document 30). It was 340 lines inside revision 1's 7b; the third leg, the pipe and the script added the rest. The provider turns it costs are not estimable from the tree.

### 7. Landing

No `testing.json` group is added; this overrides the head rule. The script's run and the recorded evidence path are the proof. The ordinary three commands must still pass, because the tagged file must not break the untagged build. Proof packages: `./cmd/metasystem/`.
The managed-seat lifecycle for hosts with no native Stop is the follow-on goal the umbrella names.

## Member 8: stop-refusals-under-ten-a-day

This member is an observation. It builds nothing and has no fixtures. Its reader and TestDailyRefusalCountReadsTheLogStream are member 7b.

What is counted: refusals per seat per local day, for seven full days, on every seat. A refusal is one `stop-decision` line with outcome `block`. Each checkout has its own `artifacts/agents/supervision/hooks.log`, so one log is one seat. The residual is counted beside it, as 7b defines it. A seat produces the count from `metasystem/`:

```text
bin/metasystem report stop-count --root . --days 7
```

Until 7a and 7b are on every seat, the hook's response line gives an interim count per UTC day. It is for watching the trend only, because it has no seat field and cuts the day in UTC:

```text
grep 'stop response decision=block' artifacts/agents/supervision/hooks.log | cut -c1-10 | sort | uniq -c
```

The version-1 refusal record is a cross-check: its `count` per cause should not exceed the lines of that session. The per-session verdict file is replaced at every verdict and is never a source.

What must be in place: 7a's decision line per stop, 7b's verb, a hook log that is not truncated inside the seven days (or a reader that also reads its rotated files), and the recorded date on which every seat was rebuilt and re-armed on 7a. The seven days start the day after that date.
The seat records the table of counts and the residual on the umbrella page. The acceptance is Wido's.
If member 2 lands later, the residual also reads its version-2 records; the refusal count does not change.

## Critique r1 dispositions

1. FOLDED, NEW MECHANISM: 3a adds a steward-owned delivered ledger, and the hook passes coordinates so both sources share one identity (Mechanism, DONE 3, TestStopIncidentOutageRecoveryOutage).
2. FOLDED: 3b gives every replay run an injected cause through a wrapper engine, and its fixture checks both writes were attempted.
3. FOLDED: 3b drops the flag; the deadline parent already holds both failures and gains the `delivery-unconfirmed` qualifier with its own fixture and mutation.
4. FOLDED: member 3 is now the ordered members 3a and 3b, each with its own DONE, fixtures and mutations.
5. FOLDED: member 4 reads the job coordinates through goal's own permissive lens, not `dispatch.JobRecord`.
6. FOLDED: member 4 resolves the owner once before the work read and reuses it, with a subtest and a mutation.
7. FOLDED: member 4 defines two checks around one rebuild, each against its own board.
8. FOLDED: member 5's TestStopClearingCommandsRun reads the wait id from the `METASYSTEM_WAIT_REGISTERED_FD` descriptor.
9. FOLDED: member 6 changes Epoch, Reason and FenceEpoch one at a time, with a mutation for Epoch and one for Reason.
10. FOLDED: member 6 drops the checkout-stop test from its table and names it under Not changed.
11. FOLDED: 7a splits `committed` into `stateSaved` and `lineAppended` and states the rule per class, the idle refusal being the named exception.
12. FOLDED: 7a qualifies DONE 4 for a failed append, records `stop-decision-unwritten`, and proves the line with a clean-stop subtest and a writer mutation.
13. FOLDED, NEW MECHANISM: 3b adds the fixture-only expiry seam, honoured only with the process-fixture owner tag; 7a and 7d reuse it.
14. FOLDED, NEW MECHANISM: 7c gates headless Claude rounds at the adapter boundary, against the umbrella's Landing section, and 7a's registry declares boundaries per host mode.
15. FOLDED, NEW MECHANISM: 7d keeps the live proof out of `testing.json`: a tagged test, a script that refuses to start without a provider, and retained evidence.
16. FOLDED: the count reader is member 7b with its own DONE and mutation, the writer stays in 7a, and member 8's text is updated.
R1. FOLDED: 3a's TestStopIncidentDrainBeforeNarrator makes a non-empty digest and faults its directory.
R2. FOLDED: 3a reports through `TickResult` and the tick verb's printed report, and adds no health role.
R3. FOLDED: member 4 adds the exported `goal.AcceptedTip` wrapper and `internal/goal/txn.go` to its file set.
R4. FOLDED: member 5 adds `turnActions`, built once before `requireClearingCommand` and handed to the freeze.
R5. FOLDED: member 5 adds `builtEngine` in `cmd/metasystem/engine_binary_test.go` to its file set and estimate.
R6. FOLDED: member 6's fingerprint reads `StopCapability.FenceEpoch`, not a `ClaimRecord` field.
R7. FOLDED: 7c names the `gateTurn` field, its writer and why it is not `continuation`.
R8. FOLDED: 7d names the producer and the line: the wrapper engine writes each gate response to `gate-calls.fifo`.

## Not checked

The tool budget went to the findings. The builder of the named member confirms each item before relying on it.

- Member 3a: whether `scripts/agents/supervision-hook-fixtures.sh` pins the argument list of `report stop-block`, which gains four flags.
- Member 3b: the environment name behind `identity.FixtureOwnerEnv` as bash sees it; and whether the missing-engine runs end in the deadline parent's form or in the Stop entry's `printf` fallback. The fixture asserts what the tree does.
- Member 4: how the state transaction in `TurnVerdict` is laid out around `decide`, for the session-state copy.
- Member 7a: which engine call in the hook is the narrator read that `--cause narrator` must fail.
- Member 7c: how `scripts/agents/dispatch.sh` computes the remaining cap of a running job, and whether the claude follow-up needs more than `--session`.
- Member 7d: how an interactive Claude session is started and prompted in a bed, and whether an existing group's inputs own the two new paths. An unowned path blocks a landing.
