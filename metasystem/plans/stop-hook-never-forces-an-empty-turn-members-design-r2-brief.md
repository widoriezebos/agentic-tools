# Design brief: stop-hook-never-forces-an-empty-turn members design, revision 2

## Revision

Revision: revision 2 of the page stop-hook-never-forces-an-empty-turn-members-design.md, a fold after critique r1. The new revision is written to plans/stop-hook-never-forces-an-empty-turn-members-design-r2.md; revision 1 stays untouched at plans/stop-hook-never-forces-an-empty-turn-members-design.md.

Reason: critique r1 (2026-09-19, another model, against revision 1 and the tree at 4e4e46de7) returned 16 material findings and 8 remarks; its required checks 1, 2, 3, 4 and 7 failed and 5 and 6 passed. The seat verified the decisive tree claims and confirmed them. The findings change what gets built (an import cycle, an append order, a readiness protocol, a host mode, a member split), so the page gets one more revision before build briefs are cut from it.

## Context pack

Read this pack first. Open another file only to check one of the cited lines
below; do not widen the read beyond that check.

Batch independent reads: when several files or ranges are needed and none depends on another's content, request them all in one turn, never one per turn.

Diagnosis or prior revision:

The prior revision is revision 1 of this page, plans/stop-hook-never-forces-an-empty-turn-members-design.md at 4e4e46de7 (460 lines, 7,321 words). It is copied below section by section; each member section is followed by the critique findings against it (material findings numbered as in the critique, remarks R1 to R8, and the facts the critique resolved from the page's Not checked list). Do not open the revision 1 file or the critique file: everything they hold that this round needs is in this pack.

What this round produces: revision 2 of the same page. This is a FOLD, not a redesign:

1. The page keeps the structure of revision 1: the same head, the member 2 paragraph, one section per member 3 to 7 in landing order with the same seven parts (DONE, Mechanism, File set, Fixtures, Mutations, Estimate, Landing), the short member 8 section, and a Not checked list. Text the findings do not touch is carried over as it stands. The head's first line becomes: Revision 2, 2026-09-19. Written against the tree at 4e4e46de7.

2. A new section `## Critique r1 dispositions` sits before `## Not checked`. It has exactly 24 lines, one per material finding 1 to 16 and one per remark R1 to R8, in that order, each of one of these two shapes: `N. FOLDED: <member and part changed, one sentence>` or `N. REJECTED: <the tree fact, by file:line, that refutes the finding>`. A finding is REJECTED only with a tree fact you checked in this session; a finding you cannot refute is FOLDED. If folding a finding needs a mechanism that neither revision 1 nor the umbrella names, the line starts `N. FOLDED, NEW MECHANISM:` and names it, so the seat can decide before builds start.

3. Every member is re-sized after the fold: its Estimate part states the new changed-line number and, in one sentence, what moved it. Where a finding says a member is two mechanisms (finding 16), the fold splits it into members with their own DONE, fixtures and mutations, and the landing order lists both.

4. The Not checked list of revision 1 is replaced. Every item the critique resolved (the Resolved lines below) becomes a fact inside the member section that needed it; only what is still unchecked stays listed, each with the builder who confirms it.

5. The constraints below are facts the seat verified at 4e4e46de7. Design with them; do not spend tool calls re-checking them. Every other claim about code still carries file:line from an excerpt below or from a check you made.

6. The rules of revision 1 that bind every section still bind: injected clocks and fakes only, never t.Skip, never a raised bound, never a retry; one mechanism per member with its own DONE and a standalone green landing; the engine never names a runtime; read rules on append-only records never tighten; listed test names are protected; source comments carry no round, finding, seat or goal id; plain English, short sentences, things named by symbol and path; tables only for fixture lists.

Constraints verified by the seat at 4e4e46de7 (verbatim from the seat's check of the critique):

- (F5) internal/dispatch imports internal/goal (admission.go, budget.go), so member 4's goal-side join over dispatch.JobRecord is an import cycle.
- (F3) the deadline parent calls report stop-block BEFORE deadline_log_stop_condition (supervision-hook.sh 1579-1586), so member 3's --hook-log-failed flag cannot be known there.
- (F8) readiness is only the METASYSTEM_WAIT_REGISTERED_FD descriptor (wait_verb.go:35), member 5's stdout readiness line does not exist.
- (F9) StopFence has Epoch and Reason, so member 6's Epoch-only mutation stays green.
- (F14) claude.go:392 emits `claude -p` and the repo Stop entry is the project hook only, matching the seat's 08:37 observation of a headless run that invoked only the plugin Stop hook, so member 7 cannot certify the headless host from installed hook files.
- (F15) testing.json kinds are build/integration/performance/static/unit, no provider-backed or livestop group for 7b.

Also verified by the seat: (F6) runReportTurnVerdict populates only StopHookActive/SessionAbsent and installs ResolveIdleSeat, no SeatActor/SeatClaimEpoch (cmd/metasystem/goal.go:763-777). (F11) a lost verdict-state write keeps the block and idle sets CountSpent false (internal/goal/turnverdict.go:554-561), so 7a's committed-only contract contradicts itself.

The critique's required-check verdicts, verbatim:

1. Symbols and shapes: FAIL. All existing paths, verbs and accessors named by the page were located, and proposed new symbols and files are absent as expected; material shape conflicts are dispatch.JobRecord across an import cycle, unresolved board-owner options, one-call RecheckBoard versus a two-call fixture, stdout watch readiness, nonexistent ClaimRecord.FenceEpoch, headless Claude, and the absent provider-test contract. The page's Not checked items are resolved below.
2. DONE fixtures and mutations: FAIL. The uncovered cases are member 3 fallback deduplication and a replay with no generated incident; member 4's second freshness result; member 6's Epoch mutation, which stays green because Reason also changes, and the checkout-stop fixture, which never reaches the mechanism; and member 7's synthetic count fixture, which stays green when the gate writer is removed. Other listed mutations have a direct red path in their named fixture; the member 4 always-joined mutation also turns TestClaudeTurnExitRequiresDelegateWorkNotMerelyALiveSeat red at its live-claim subcase.
3. Wall clock and readiness: FAIL. FakeReplayDeadline and the real deadline leg require the script's real deadline without a named seam. TestStopClearingCommandsRun has no stdout readiness line; it must use METASYSTEM_WAIT_REGISTERED_FD. The Codex fake can print its declared line; the live-provider “readiness lines” have no named producer. Other unit setups use injected clocks, probers and file state and need no sleep or retry.
4. Order and coupling: FAIL. Member 3 line 81 relies on parked member 2 to prevent its admitted cross-source duplicate, and its optional 3a landing needs later 3b to supply a DONE-4 fixture. Apart from those, member 5 uses landed member 4, 7a uses landed members 3 through 5, and 7b uses 7a; no other later-member or joint-landing dependency was found.
5. Engine boundary: PASS. The proposed cmd and internal logic branches on boundary data and verdict class, not on claude or codex; --runtime remains opaque. Runtime names and provider commands remain registry data or scripts. The headless-Claude defect is a missing script boundary, not an engine runtime-name branch.
6. Protected names and old records: PASS. No test name explicitly listed in testing.json is renamed, dropped or reassigned. The named retained tests are not explicit entries in that file. Optional refusal Class, pending DeliveredAt, session SurfacedFences, board facts and job coordinates preserve older JSON; hook and decision readers are specified to skip older or unknown lines; older job and run records remain readable and merely fail to join. No tightened append-only read rule was found.
7. Estimates and mechanism count: FAIL. Member 3's 1,450 lines are at best plausible only without its invalid split and before the missing dedupe, replay and deadline work; member 4's 1,100 is plausible after the import, owner and two-check corrections; member 5's 900 is plausible after using descriptor readiness and spelling out action flow; member 6's 350 is plausible after replacing the ineffective fixture; 7a's 1,400 is not a one-mechanism estimate because it includes both gate enforcement and independent daily analytics; 7b's 1,200 code estimate is plausible in isolation but the live-proof portion is not estimable or startable until headless mode and the provider testing contract are designed.

Revision 1 by section, each followed by the critique findings against it:

### Revision 1, Page head (revision 1 lines 1-20)

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

### Revision 1, Member 2 is parked (revision 1 lines 22-24)

    ## Member 2 is parked

    Member 2, stop-decisions-record-deadline-evidence, was parked and decoupled by Wido on 2026-09-19. No member on this page depends on it landing. Where the umbrella assumes the version-2 incident record, each member reads what the tree holds today: the refusal record that `StopRefusal` writes (`internal/report/stopblock.go`, schemaVersion 1, one file per session under `artifacts/agents/supervision/stop-refusals/`), the lines of `artifacts/agents/supervision/hooks.log`, and the per-session verdict state in `artifacts/agents/turn-verdict-state.json`. The fixtures TestInfrastructureDeadlineIdentity, TestStopDecisionPersistsBeforeEmission and TestStopDeadlineCompletionIsSingleUse stay with member 2 and are not built here. TestArmingDetailSurvivesStop already exists in `cmd/metasystem/up_test.go` and is left alone. Each section below says in one sentence what changes if member 2 lands later.

### Revision 1, Member 3: stop-incidents-reach-the-steward (revision 1 lines 26-125)

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

Critique findings against member 3:

Material 1. Section and page line: Member 3, DONE 3 and the fallback design at lines 36, 60, 68, 73 and 81. Tree conflict: QueueNotification overwrites the nonce file in internal/steward/intervene.go:334-350; MarkDelivered removes it in internal/steward/intervene.go:383-385; deliverPendingNotification has no delivery tombstone after removal in internal/steward/notify.go:188-202. Concrete failing scenario: while the hook log is unreadable, the drain delivers a refusal-record identity. Once its delayed ack can be appended and the pending file is removed, a later log outage makes the same refusal record look unacknowledged again; when the log recovers, the corresponding log identity also looks unacknowledged. The page itself admits the cross-source second notification at line 81 and says member 2 is what can join it, contradicting both “never queued again” and the no-member-2 rule. TestStopIncidentDeliveryUnconfirmed does not alternate a successful delivery with later readable and unreadable source states, so it remains green. Smallest page change: add a durable delivered-identity ledger or retained tombstone independent of the hook log, define how fallback and log identities are suppressed without member 2, and add an outage, recovery, outage fixture plus a mutation that bypasses that ledger.

Material 2. Section and page line: Member 3, stop-replay and FakeReplayUnreadableHookState at lines 77 and 97, with its mutation at line 106. Tree conflict: record_stop_failure is what creates stop_conditions in scripts/agents/supervision-hook.sh:1745-1756; append_stop_condition is called only for such conditions or an infrastructure verdict in scripts/agents/supervision-hook.sh:2567-2587; compose_failed_stop and therefore report stop-block run only when stop_failure is nonempty in scripts/agents/supervision-hook.sh:2621-2625. Concrete failing scenario: the proposed replay only selects a file fault and supplies no infrastructure cause. Merely making the hook log or refusal directory unwritable does not create a stop condition, so the “both broken” run need not attempt either write and need not contain the unconfirmed sentence. Ignoring --hook-log-failed can therefore leave the named fixture green. Smallest page change: give member 3's replay a deterministic injected infrastructure cause, state its setup for all five runs, and make the fixture assert that both write attempts actually occurred before judging the unconfirmed notice.

Material 3. Section and page line: Member 3, stop-notice mechanism at line 75 and its unresolved append-order premise at line 454. Tree conflict: the deadline parent calls report stop-block first in scripts/agents/supervision-hook.sh:1579-1584 and calls deadline_log_stop_condition only afterward at line 1586; the ordinary worker is the opposite order through append_stop_condition at lines 2567-2587 and compose_failed_stop at lines 2621-2625. Concrete failing scenario: on a deadline expiry where both the refusal record and hook-log append fail, the report command cannot receive the proposed --hook-log-failed fact because that fact is learned after the command returns. The response omits the required unconfirmed sentence. Smallest page change: explicitly redesign the deadline-parent branch to attempt or probe the condition log before report stop-block, pass the resulting flag, and add a deadline-parent both-writes-fail subtest and mutation.

Material 4. Section and page line: Member 3, estimate and optional split at lines 110-111. Tree conflict: the landing contract says each member lands green alone and no DONE needs a later member in plans/stop-hook-never-forces-an-empty-turn-design.md:250 and plans/stop-hook-never-forces-an-empty-turn-design.md:258. Concrete failing scenario: if the builder takes the prescribed overflow split, 3a lands without the replay fixture that DONE 4 names, while 3b depends on 3a's new action contract; line 111 expressly says 3a is incomplete for rule 4. Smallest page change: remove the conditional split and bound member 3 to a complete standalone build, or formally define two ordered members with separate DONE rules and red-before-green fixtures so neither claims the other's DONE.

Remark R1. Member 3 line 92 cites TestDegradedTickQueuesTheIncidentOrSurfacesQueueFailure as a narrator-failure pattern, but that test only calls degradedTick and faults queue storage at internal/steward/tick_test.go:193-210; a narrator-digest failure needs a nonempty digest entry and a fault at narratordigest.Append, whose empty-input fast path is internal/narratordigest/digest.go:146-150.

Remark R2. Member 3 line 71 can write arbitrary component evidence through beginComponentAttempt and completeComponentAttempt at internal/steward/component_evidence.go:165-180 and internal/steward/component_evidence.go:311-325, but stop-incident-drain is not in the closed health-role vocabulary at internal/steward/health.go:42-89; the page should say whether TickResult alone is the report or add the health-role owner and files.

Resolved by the critique. Member 3: the installed Claude Stop entry is (bash scripts/agents/supervision-hook.sh claude stop) || printf ... at scripts/enforcement/claude-code-hooks.json:28-35; the provider JSON is passed unchanged on stdin and staged by cat at scripts/agents/supervision-hook.sh:1253-1265. Component-attempt functions accept stop-incident-drain, but ObserveHealth has no matching health role. Ordinary append_stop_condition calls precede external_stop_json, but the deadline parent calls report stop-block before deadline_log_stop_condition. Causes are not equal: record_stop_failure logs the slug from stop_cause_code at scripts/agents/supervision-hook.sh:1745-1756 while compose_failed_stop passes the human diagnostic at lines 2311-2327; deadline likewise records stop-deadline-expired but reports stop deadline expired.

### Revision 1, Member 4: stop-frontier-joins-owner-and-revision (revision 1 lines 127-205)

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

Critique findings against member 4:

Material 5. Section and page line: Member 4, job-record join at line 155. Tree conflict: internal/goal/project.go is package goal and currently imports only lower-level packages at internal/goal/project.go:11-23; package dispatch imports goal through internal/dispatch/admission.go:10-12. Although dispatch.JobRecord.GoalRevision, ClaimEpoch and MachineID exist at internal/dispatch/jobrecord.go:67-68 and internal/dispatch/jobrecord.go:98-108, goal cannot import dispatch without an import cycle. Concrete failing scenario: implementing the page literally makes Go reject goal to dispatch to goal before any board fixture can compile. Smallest page change: put the permissive job read lens in a dependency leaf consumed by both packages, or keep a local permissive lens in goal; update the file set and fixture citations accordingly.

Material 6. Section and page line: Member 4, board-owner wiring at line 151. Tree conflict: runReportTurnVerdict initializes TurnVerdictOptions with only stop-hook and session facts at cmd/metasystem/goal.go:763-764; it installs store.ResolveIdleSeat at lines 765-777 but does not populate SeatActor or SeatClaimEpoch. resolveSeatIdleActor exists at cmd/metasystem/goal.go:878-895, and today it is invoked only during third-stop escalation by Store.escalateIdleBacklog at internal/goal/turnverdict.go:1285-1295. Concrete failing scenario: a builder trusts the sentence that the command already resolves the options, passes their zero values to the fresh board, and every claim becomes HeldElsewhere; a correctly joined job or run then fails to exempt its owner. Smallest page change: specify one pre-board actor-resolution call, its error and unknown-lineage behavior, and how the same frozen actor and epoch are reused by RecheckBoard and idle escalation.

Material 7. Section and page line: Member 4, freshness mechanism at line 159 and TestStopFrontierOwnerRevisionAndFreshness at line 176. Tree conflict: the insertion point before saveVerdictState is Store.TurnVerdict at internal/goal/turnverdict.go:554-568; there is no second freshness observation after a rebuild. Concrete failing scenario: the page says RecheckBoard is called once, yet the fixture requires one changed tip to rebuild successfully and a tip returned as changed twice to produce stop-board-stale. With one call there is no second result to distinguish these cases, so the stale-twice subtest cannot be constructed as described. Smallest page change: specify at most two checks, one before the initial commit and one after the single rebuild, with the exact accepted tip and claim epoch compared at each check.

Remark R3. Member 4's robust accepted-tip reader is the unexported acceptedTipForGates in internal/goal/txn.go:152-188; a cmd-layer RecheckBoard cannot call it as written. Exporting a read-only wrapper and adding txn.go to the file set is a small path correction.

Resolved by the critique. Member 4: the governed-run lister is run.Store.List in internal/run/waiter.go:1881-1902; records expose GoalId, OwnerLineage, ClaimEpoch and Governed.GoalRevision in internal/run/run.go:123-146 and internal/run/run.go:172-195. The robust local accepted-tip reader is unexported acceptedTipForGates in internal/goal/txn.go:152-188; project also directly rev-parses AcceptedRef at internal/goal/project.go:83-92. No exported accepted-tip wrapper exists.

### Revision 1, Member 5: stop-refusals-name-seat-actions (revision 1 lines 207-272)

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

Critique findings against member 5:

Material 8. Section and page line: Member 5, TestStopClearingCommandsRun at line 251. Tree conflict: runJobWatchVerb emits no stdout readiness line and blocks in cmd/metasystem/run.go:458-486; durable waiter readiness is sent only through the inherited METASYSTEM_WAIT_REGISTERED_FD descriptor by emitWaitCommandEvent in cmd/metasystem/wait_verb.go:35-59. ProcessFixture.Shell alone at internal/testutil/fixture.go:169-185 does not install that descriptor. Concrete failing scenario: the fixture waits for the claimed printed readiness line before asking for the next verdict, but the watch process is waiting for job completion and prints nothing, leaving the fixture with no event-driven readiness observation and forcing a sleep, retry or hang. Smallest page change: use the existing readiness pipe protocol, including ExtraFiles and the environment descriptor, and have the fixture read the wait id from that pipe; do not assert a stdout line.

Remark R4. Member 5 line 226 gives requireClearingCommand only a Verdict, while actions are currently created by freezeTurnVerdictFacts at internal/goal/turnfacts.go:88-116, after state save at internal/goal/turnverdict.go:628-630. The page should state the preliminary-action dataflow so the builder does not freeze stale verdict facts merely to obtain Actions.

Remark R5. Member 5 has no reusable helper that builds bin/metasystem: InstalledWaitBinary only copies an already built candidate at internal/testutil/wait_binary.go:10-54; cmd tests invoke local go build, for example cmd/metasystem/context_cost_test.go:379. This is implementable but omitted from the file and estimate map.

Resolved by the critique. Member 5: legacy runGoalDeclareFree takes no text at cmd/metasystem/goal.go:243-247, but the synced route requires --digest at cmd/metasystem/goalsync_mutations.go:1438-1443 and lineage or its authenticated environment fallback at lines 604-648, so the current synced refusal must be a notice unless the page designs those values. runGoalClaim at cmd/metasystem/goalsync_mutations.go:2694-2710 requires id and the common parser accepts root and lineage; it needs no additional explicit flag, though normal holder authentication still applies. No shared cmd test helper builds the binary; testutil.InstalledWaitBinary installs a supplied candidate, and existing cmd tests run go build themselves.

### Revision 1, Member 6: stop-fences-surface-once (revision 1 lines 274-323)

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

Critique findings against member 6:

Material 9. Section and page line: Member 6, TestFencedClaimSurfacedOnceBesideFlight at line 308 and its Epoch mutation at line 315. Tree conflict: StopFence has independent Epoch and Reason fields in internal/goal/file.go:389-398. Concrete failing scenario: the fixture changes both Epoch and Reason, while the mutation removes only Epoch from the fingerprint. Reason still changes the hash, so the new FENCED line still appears and the exact fixture remains green. Smallest page change: change Epoch alone in one subtest and Reason alone in another, or choose a mutation that removes every field the fixture changes; retain a separate assertion for each fingerprint coordinate claimed by DONE 3.

Material 10. Section and page line: Member 6, the retained TestTurnVerdictDoesNotBlockAndDescribesTheDurableStopPhase fixture at line 309. Tree conflict: this test writes the checkout-wide stopfence and calls TurnVerdict in internal/goal/turnverdict_stopfence_test.go:9-59; Store.TurnVerdict returns from that checkout-wide fence before reading goal work or session state at internal/goal/turnverdict.go:437-463. Concrete failing scenario: stopping twice can never produce a goal-file FENCED line in this fixture, so its proposed second-stop expectation does not fail before or after SurfacedFences; part 5 assigns it no mutation either. Smallest page change: leave this checkout-stop regression unchanged and remove it from member 6's fixture table, or add a separate per-goal stop-fence setup that reaches surfaceFencedClaims and name a mutation that turns it red.

Remark R6. Member 6 line 290 names nonexistent ClaimRecord.FenceEpoch; ClaimRecord ends at internal/goal/file.go:330-355, while FenceEpoch is StopCapability.FenceEpoch at internal/goal/file.go:379-387.

Resolved by the critique. Member 6: the ok branch has two exits before today's fence append, the registered-wait break at internal/goal/turnverdict.go:1700-1706 and delegate-job break at lines 1708-1710; today's append is at lines 1730-1732. work.fencedClaims is []*GoalFile at internal/goal/project.go:223-235.

### Revision 1, Member 7: stop-hosts-enforce-the-common-gate (revision 1 lines 325-421)

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

Critique findings against member 7:

Material 11. Section and page line: Member 7a, response invariant at line 345 and TestStopGateAnswersOnlyCommittedBlocks at line 381. Tree conflict: a lost verdict-state write deliberately preserves any observed block in Store.TurnVerdict at internal/goal/turnverdict.go:554-568, and for idle it sets CountSpent false at lines 559-561. Concrete failing scenario: the test faults the state write. The page simultaneously requires decision to be block only when committed is true, defines committed as requiring the state write, and requires the idle refusal to answer block. No response can satisfy all three assertions. Smallest page change: define an explicit idle-refusal exception in the response contract, or replace committed with separate decision-observed and counter-committed facts and state exactly which one gates provider blocking.

Material 12. Section and page line: Member 7a, DONE 4 at line 336, decision append at lines 347-355, TestStopGateAnswersOnlyCommittedBlocks at line 381, and TestDailyRefusalCountReadsTheLogStream at line 382. Tree conflict: current hook-log verdict writes are already allowed to fail at scripts/agents/supervision-hook.sh:2589-2591; the new fixture explicitly faults the decision-line append. Concrete failing scenario: an unwritable log necessarily leaves no decision line, contradicting “Every stop leaves one”; meanwhile the daily-count fixture uses synthetic lines and still passes if the gate never calls AppendDecision. Thus the fixture named by DONE 4 does not fail without the writer. Smallest page change: qualify the invariant for append success and define the unconfirmed evidence path for append failure, then extend a gate integration fixture to perform a successful stop, read its emitted line, and make removal of AppendDecision turn that fixture red.

Material 13. Section and page line: Member 7a and 7b, FakeReplayDeadline and the real deadline-expiry leg at lines 359, 383 and 385. Tree conflict: the installed Stop wrapper clamps the deadline budget to 4 through 60 seconds, reads date and bash SECONDS, and waits for the worker at scripts/agents/supervision-hook.sh:1236-1267. Concrete failing scenario: forcing the deadline cause through the installed hook requires a real timer to expire; neither fixture names an injected clock or a fixture-only expiry signal. The fake test therefore needs a wall-clock wait, and the real-provider leg is even less deterministic. Smallest page change: define an authenticated fixture seam that drives the parent directly into deadline-expired state with fixed timestamps, and name that seam in both fixtures; production deadline behavior must remain unchanged.

Material 14. Section and page line: Member 7, native Claude boundary and live proof at lines 334, 339, 357, 367 and 385. Tree conflict: BuildClaudeCommand is explicitly the headless command builder and emits claude -p in internal/adapter/claude.go:342-343 and internal/adapter/claude.go:391-397; the repository Stop entry is only the project hook command in scripts/enforcement/claude-code-hooks.json:28-35. The observed 08:37 headless run invoked only the plugin Stop hook and produced no repository-hook log line. Concrete failing scenario: RealClaudeNativeStopReplay launched through the repository's real Claude adapter exercises -p; it can complete with no host stop-gate call and no decision line, so the page cannot certify the native boundary for the production headless host. Smallest page change: model host mode in the runtime boundary declaration and add a gate boundary that headless Claude actually executes, such as its adapter turn boundary, while keeping the native hook for interactive sessions; prove both modes separately and do not certify from installed hook files.

Material 15. Section and page line: Member 7b, live-group landing rule at line 405. Tree conflict: wait-stop-standard is an ordinary integration group in testing.json:50; the full cadence list at testing.json:112 contains section groups only, and no group or environment in the file names a provider-backed, live-provider or livestop bed. Concrete failing scenario: the page instructs the builder to reuse a kind and cadence that do not exist and then instructs the builder to stop if absent, so 7b cannot start or land its required live proof. Smallest page change: specify the new group's valid kind, platforms, tools, credential admission and non-per-change cadence explicitly, including how an unavailable provider is handled without t.Skip, or move the live proof to an already defined external evidence owner and say what durable evidence the build consumes.

Material 16. Section and page line: Member 7a, split declaration and scope at lines 327-328, with the independent count mechanism at lines 347-355 and estimate at line 398. Tree conflict: the umbrella requires one mechanism per member and a standalone green landing at plans/stop-hook-never-forces-an-empty-turn-design.md:258. Concrete failing scenario: the gate and decision writer can work and land without DailyRefusalCounts or report stop-count, and the count reader can work and land over synthetic lines without changing stop enforcement. They have different owners, verbs and fixtures, so 7a is two independently landable mechanisms; its count fixture also does not prove the gate. Smallest page change: make the decision writer part of 7a's gate, move count aggregation and report stop-count to a separate observation-reader build member with its own DONE and mutation, and update member 8's “builds nothing” statement.

Remark R7. Member 7b line 363 does not define the gate-turn record field. Cap continuation writes the create-time continuation: after-cap field in internal/dispatch/build.go:973-975, and that field is immutable in internal/dispatch/record.go:88-100; RecordCAS can add ordinary running metadata at internal/dispatch/record.go:542-621, but the page must name and own that new metadata rather than treating continuation as its shape.

Remark R8. Member 7's live fixture says only “readiness lines”; it names neither the producing command nor the exact line. The fake Codex binary can define one, but the real-provider fixture needs an event-driven readiness source before it is buildable without polling.

Resolved by the critique. Member 7: cap continuation is a newly created follow-up record with continuation: after-cap, resumeMode: fresh-context, a fresh-context adapter dispatch and prior-worktree continuation, assembled in scripts/agents/dispatch.sh:2713-2739 and internal/dispatch/build.go:960-975; it is not a mutable spent-turn field. The Codex follow-up path is BuildCodexCommand, which emits codex exec resume, per-turn configuration, the session and stdin marker at internal/adapter/codex.go:87-110; codex.sh already routes follow-up through that builder at scripts/agents/adapters/codex.sh:97-119. testing.json defines no provider-backed kind, group or cadence; wait-stop-standard is integration at line 50 and the cadence list at line 112 has no provider or live-stop entry.

### Revision 1, Member 8: stop-refusals-under-ten-a-day (revision 1 lines 423-448)

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

### Revision 1, Not checked (revision 1 lines 450-459)

    ## Not checked

    The tool budget did not allow these checks. The builder of the named member confirms each before relying on it.

    - Member 3: the exact arguments and standard input of the installed Stop hook entry, which `stop-replay` must reproduce. The signature of the tick's health path for the component `stop-incident-drain`. That `append_stop_condition` runs before `external_stop_json` on every path. Whether the cause passed to `report stop-block` equals the cause code on the `stop-condition` line.
    - Member 4: the lister for live governed runs that `readLiveBacklogActivity` should use, and the local accepted-tip reader that `RecheckBoard` should wrap.
    - Member 5: the required flags of `goal declare-free`, whether `goal claim` needs more than `--root`, `--id` and `--lineage`, and which helper builds `bin/metasystem` for process tests in `cmd/metasystem`.
    - Member 6: every early exit of the `ok` branch in `decide`, and the element type of `work.fencedClaims`.
    - Member 7: the record shape the dispatcher uses for a cap continuation, the resume path of `build_codex_command`, and the kind and cadence `testing.json` gives provider-backed tests.
    - Member 1's fixtures: a grep for TestStopConditionClassification and TestLauncherFailureNeverRawBlocks found nothing at 4e4e46de7. No section here depends on them by name.

Critique findings against the Not checked list:

Resolved by the critique. Member 1 fixtures: neither TestStopConditionClassification nor TestLauncherFailureNeverRawBlocks occurs in the tree outside the plans at this commit. No member 3-to-7 fixture or mutation depends on either name.

Cited code excerpts (byte-exact at 4e4e46de7):

1. `scripts/agents/supervision-hook.sh:1579-1586`

   ```text
       deadline_response=$("$deadline_engine" report stop-block \
         --class infrastructure \
         --refusal-record "$deadline_record" --session "$deadline_session" \
         --open-work-root "$deadline_repo" \
         --cause "$deadline_cause" --remedy "$deadline_remedy" "$deadline_detail" 2>/dev/null) || \
         deadline_record_failure="the stop-refusal record could not be read or atomically updated"
     fi
     deadline_log_stop_condition stop-deadline-expired stop-deadline
   ```

2. `scripts/agents/supervision-hook.sh:2567-2591`

   ```text
     if (( ${#stop_conditions[@]} > 0 )); then
       for stop_condition in "${stop_conditions[@]}"; do
         append_stop_condition infrastructure "${stop_condition%%|*}" "${stop_condition#*|}" degraded-allow
       done
     fi
     if [[ "$verdict_readable" == true && "$verdict_class" == infrastructure ]]; then
       verdict_cause=$("$ms" json get --value "$verdict" --field causeCode 2>/dev/null || true)
       verdict_component=$("$ms" json get --value "$verdict" --field component 2>/dev/null || true)
       append_stop_condition infrastructure "${verdict_cause:-turn-verdict-unavailable}" "${verdict_component:-verdict-state}" degraded-allow
       # The verdict's own state could not be read or written: the notice says
       # so in fixed words, names the owner, and carries the detail; it never
       # reads as an all-clear.
       display="turn-verdict degraded: stopping is allowed on degraded infrastructure; the steward owns repair. Cause: ${verdict_cause:-turn-verdict-unavailable}. Component: ${verdict_component:-verdict-state}.
   $display"
     fi
     if [[ "$verdict_readable" == true && "$verdict_class" == idle-with-backlog && "$should_block" == true && "$count_spent" == false ]]; then
       append_stop_condition idle-with-backlog idle-refusal-count-not-spent verdict-state refused-uncounted
     fi
     if [[ "$verdict_readable" != true ]]; then
       append_stop_condition infrastructure turn-verdict-unavailable verdict-state degraded-allow
     fi
   
     if [[ "$verdict_readable" == true ]]; then
       printf '%s stop verdict block=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
         "$should_block" >>"$supervision_dir/hooks.log" 2>/dev/null || true
   ```

3. `cmd/metasystem/wait_verb.go:35-59`

   ```text
   const waitRegisteredFDEnvironment = "METASYSTEM_WAIT_REGISTERED_FD"
   
   var waitCommandEvents = &events.Emitter{Component: "run", Pid: int64(os.Getpid())}
   
   func emitWaitCommandEvent(root, event, summary string, fields map[string]string) error {
   	if err := waitCommandEvents.EmitChecked(root, event, summary, fields); err != nil {
   		return err
   	}
   	if event != "wait-registered" || os.Getenv(waitRegisteredFDEnvironment) == "" {
   		return nil
   	}
   	descriptor, err := strconv.Atoi(os.Getenv(waitRegisteredFDEnvironment))
   	if err != nil || descriptor < 3 {
   		return fmt.Errorf("%s must name an inherited descriptor", waitRegisteredFDEnvironment)
   	}
   	ready := os.NewFile(uintptr(descriptor), "wait-registered")
   	if ready == nil {
   		return fmt.Errorf("%s descriptor is unavailable", waitRegisteredFDEnvironment)
   	}
   	if _, err := fmt.Fprintln(ready, fields["waitId"]); err != nil {
   		_ = ready.Close()
   		return err
   	}
   	return ready.Close()
   }
   ```

4. `internal/goal/file.go:379-398`

   ```text
   // StopCapability binds breach-stop authority to one exact local claim. The
   // goal id is the containing record's id; every other coordinate is explicit.
   type StopCapability struct {
   	Generation uint64
   	Revision   uint64
   	Machine    string
   	ClaimEpoch int64
   	FenceEpoch uint64
   }
   
   // StopFence is the absorbing launch refusal for one stopped revision. A human
   // resume removes it only after the named batch is complete.
   type StopFence struct {
   	StopID               string
   	Revision             uint64
   	Epoch                uint64
   	CapabilityGeneration uint64
   	ClosedAt             string
   	Reason               string
   }
   ```

5. `internal/adapter/claude.go:391-397`

   ```text
   	command := []string{
   		"claude", "-p", "--output-format", outputMode, "--model", model,
   		"--json-schema", schemaJSON,
   		"--permission-mode", permissionMode,
   		"--tools", tools,
   		"--allowedTools", tools,
   	}
   ```

6. `scripts/enforcement/claude-code-hooks.json:28-35`

   ```text
       "Stop": [
         {
           "hooks": [
             {
               "type": "command",
               "command": "(bash scripts/agents/supervision-hook.sh claude stop) || printf '%s\\n' '{\"systemMessage\":\"Task unknown; Stop allowed; needs supervision repair; hook-bootstrap-failed. The steward must restore supervision. Status unavailable.\"}'",
               "timeout": 60
             }
   ```

7. `internal/dispatch/admission.go:3-13`

   ```text
   import (
   	"fmt"
   	"path/filepath"
   	"sort"
   	"strings"
   	"time"
   
   	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
   	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
   	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
   )
   ```

8. `internal/goal/turnverdict.go:554-568`

   ```text
   		verdictStateSaved := true
   		if err := s.saveVerdictState(state); err != nil {
   			verdictStateSaved = false
   			if verdict.ShouldBlock {
   				detail := infrastructureFailure{"verdict-state", err}.Error() + "; the observed refusal remains blocking"
   				if verdict.IdleRefusal {
   					verdict.CountSpent = false
   					detail = "the idle refusal count could not be spent because the turn verdict state could not be written: " + err.Error()
   				}
   				verdict.Diagnostics = append(verdict.Diagnostics, detail)
   				verdict.Display = strings.TrimSpace(verdict.Display + "\n" + detail)
   				fullDisplay = composeDisplay(append(append([]string{}, brainLines...), runLines.lines...), verdict.Display, greens)
   			} else {
   				return Result{}, infrastructureFailure{"verdict-state", err}
   			}
   ```

9. `cmd/metasystem/goal.go:763-777`

   ```text
   	store := &goal.Store{Root: *root, Now: func() time.Time { return now }}
   	options := goal.TurnVerdictOptions{StopHookActive: *stopHookActive, SessionAbsent: *sessionAbsent}
   	stateRoot, rootErr := goal.ResolveStateRoot(*root)
   	options.ContextLine = turnVerdictContextLine(*root, stateRoot, *runtimeName, *session, *transcript, now, rootErr)
   	if rootErr != nil {
   		options.SeatActorProblem = "the seat state root could not be resolved: " + rootErr.Error()
   	} else {
   		options.HandoffRecorded = func(session string) (string, bool, error) {
   			return steward.LiveHandoffForSession(stateRoot, session)
   		}
   		store.PrepareIdleContinuation = prepareSeatIdleContinuation(stateRoot)
   		store.RecordIdleIncident = recordSeatIdleIncident(stateRoot, now)
   		store.RaiseIdleAlarm = raiseSeatIdleAlarm(stateRoot)
   		store.ResolveIdleSeat = resolveSeatIdleActor(stateRoot, *mainId)
   	}
   ```

10. `internal/steward/intervene.go:334-350`

   ```text
   // QueueNotification stores the message durably before any delivery
   // attempt.
   func QueueNotification(repoRoot string, n PendingNotification) error {
   	dir := pendingDir(repoRoot)
   	data, err := json.Marshal(n)
   	if err != nil {
   		return err
   	}
   	path := filepath.Join(dir, n.Nonce+".json")
   	durable, err := atomicfile.WriteText(path, string(append(data, '\n')), repoRoot)
   	if err != nil {
   		return err
   	}
   	if !durable {
   		return fmt.Errorf("pending notification %s was published with durability unknown", n.Nonce)
   	}
   	return nil
   ```

11. `testing.json:50-50`

   ```text
       {"id":"wait-stop-standard","kind":"integration","adapter":"go","cwd":"metasystem","inputs":["metasystem/go.mod","metasystem/go.sum","metasystem/internal/goal/**","metasystem/internal/adapter/**","metasystem/internal/run/**","metasystem/internal/proofrun/**","metasystem/cmd/metasystem/wait_verb.go","metasystem/cmd/metasystem/wait_verb_test.go","metasystem/scripts/agents/adapters/**","metasystem/scripts/agents/supervision-fixtures.sh","metasystem/docs/design/turn-verdict-delivery-contract.md","metasystem/internal/report/**"],"outputs":[],"tools":[{"id":"go","executable":"go","versionArgs":["version"]},{"id":"bash","executable":"bash","versionArgs":["--version"]},{"id":"git","executable":"git","versionArgs":["--version"]}],"obligations":["budget-stop-authority","runtime-custody"],"platforms":["any"],"targetMs":60000,"packages":["internal/goal","internal/adapter","cmd/metasystem","internal/report"],"tests":["TestPendingWaitTurnVerdict","TestPendingWaitIdleBacklog","TestWaitDeliveryContract","TestPendingWaitInstalledVerdicts","TestWaitPathSelectorIsValidated","TestNextStepNamesAPendingHumanWord","TestIdleBacklogContinuationSkipsAGoalWaitingOnAHumanWord","TestIdleBacklogContinuationLeavesAHeldGoalThatWaitsOnAHumanWord","TestIdleBacklogWithOnlyHumanWaitingGoalsPreparesNoContinuation","TestMapStopOutputWritesTheResponseRecord","TestMapStopOutputAcceptsChangedWording","TestStopLineIsRenderedFromTheReportReference","TestPruneRemovesTheResponseRecordWithItsReport","TestReportStopResponseResolvesUnderChangedWording","TestReportStopResponseRefusesAnUnreadableResponse","TestBedsResolveStopReportsThroughTheEngine","TestToolGateAllowlist","TestToolGateNeverDeniesLandingWaitOrAgent","TestToolGateAllowEmitsNothing","TestToolGateCeilingColumnEqualsTrigger","TestToolGateMemoryNoteRule","TestWaitRegisterLocalRecordsTheTrackedProcess","TestWaitRegisterHumanNeedsAQuestionAndADeadline","TestRegisteredLocalAndHumanWaitsInstalledVerdicts","TestLocalWaitAllowsTheStopWhileItsProcessLives","TestLocalWaitOfADeadProcessDoesNotAllowTheStop","TestHumanWaitAllowsTheStopUntilItsDeadline","TestLocalWaitCoversTheJobItNames","TestToolGateDeadlineCountsFromTheShellBirth","TestToolGateFallsBackToEntryWhenBirthUnreadable","TestToolGateClassifiesBeforeReading","TestToolGateReadOptionsAreNonBlocking","TestToolGateAllowsNativeSubagentCalls","TestToolGateSubagentCallsWriteNoRow","TestToolGateAllowsPastItsDeadline","TestToolGateNoDecisionWhenTheCallStoreIsBusy","TestToolGateLeavesTheCursor","TestToolGateWritesDecisionRows","TestToolGateObserveModeAllowsAndRecordsTheDenyDecision","TestToolGateDenyModeDenies","TestClaudeJobSettingsInstallNoToolGate","TestWaitingLinesUseWaitEndForLiveLocalWait","TestWaitingLinesUseWaitEndForPendingHumanWait"],"race":false,"coverage":false},
   ```

12. `testing.json:112-112`

   ```text
     "cadence": ["section/engine-delivery-contract","section/static-placeholder-scan","section/covenant-evidence-pre-rebuild","section/go-engine-gate","section/supervision-go-fixtures","section/gate-fence-fixtures","section/covenant-evidence-post-rebuild","section/suite-host-prerequisites","section/metasystem-audit","section/gate-fail-open-tripwire","section/witness-gate-fixtures","section/suite-progress-fixtures","section/fixture-bed-scenarios-fixtures","section/land-fixtures","section/static-contract-audits","section/supervision-and-census-fixtures","section/supervisor-fingerprint-heal-harness","section/mission-fixtures","section/shell-and-dependency-audits","section/conformance-fixtures","section/goal-cli-fixtures","section/brain-fixtures","section/telemetry-census-fixtures","section/return-schema-fixtures","section/config-identity-fixtures","section/authority-regression-fixtures","section/pre-commit-guard-fixtures","section/static-reproof-fixtures","section/project-extra-suites","section/record-protocol-fixtures","section/evidence-segment-fixtures","section/second-session-fixtures","section/lease-succession-fixtures","section/flight-recorder-fixtures","section/acp-fixtures","section/delegate-caps-fixtures","section/adapter-deadline-fixtures","section/enumeration-mode-fixtures","section/runtime-contract-audits","section/agent-protocol-fixtures","section/dispatcher-adapter-and-mission-runner-fixtures","section/workflow-tooling-fixtures","section/adoption-fixtures","section/watch-background-jobs-fixtures"]
   ```

## Recurring findings

Filled by the seat from the records of goal retro-flags-repeated-critique-findings for the goals this design touches, or the words `none recorded`.

none recorded

## Tool-call budget

Maximum delegate tool calls: 80

Stop when this number is reached. List anything the budget did not allow you
to check.

## Page-size ceiling

Maximum page size: 7500 words

Cut a draft that exceeds this ceiling. If cutting would make the page
incomplete, stop and propose a split instead.

## Fresh session

This revision runs in a new delegate session. Use the prior page and critique
only through the context pack above. Never resume a delegate from an earlier
revision.

## Page artifact and return shape

Write the page to: /Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.claude/worktrees/launchtip/metasystem/plans/stop-hook-never-forces-an-empty-turn-members-design-r2.md

Return only these two lines:

```text
/Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.claude/worktrees/launchtip/metasystem/plans/stop-hook-never-forces-an-empty-turn-members-design-r2.md
DESIGN: ready (NNNN words)
```

or:

```text
/Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.claude/worktrees/launchtip/metasystem/plans/stop-hook-never-forces-an-empty-turn-members-design-r2.md
DESIGN: blocked (one line saying why)
```
