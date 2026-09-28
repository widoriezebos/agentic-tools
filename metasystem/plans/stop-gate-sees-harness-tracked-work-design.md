# Stop gate sees harness-tracked work

Design for `metasystem/plans/goals/stop-gate-sees-harness-tracked-work.md` (tag sgh). Revision 1, 2026-09-16.
Author: Claude Fable 5.1 design delegate, launched headless by seat m1e.
Parent frame: `metasystem/plans/coordinator-wakes-on-events-not-polls-design.md` (m1b, in flight). This page adds one wait kind and one source to that frame. It changes no row of the parent's stop table, no waiter loop behaviour, no hint, delivery, resume or recovery rule, and none of the clause 5 measurement in the parent's amendments r1 to r4.
This page is the design deliverable only. It changes no code, fixture or ledger.

## 1. The fault, stated once

The stop gate honours a registered wait only when it can resolve the wait to an engine-side record. `registeredWaitSource` (`metasystem/internal/goal/turnverdict.go:751-801`) accepts four kinds. A job wait reads `artifacts/agents/jobs/<id>.json` (`pendingWaitJob`, `turnverdict.go:804-841`); a run wait reads the run store (`:843-851`); an attempt wait reads the proof attempt (`:853-870`); a goal wait observes the ledger (`:790`). Any other kind falls to `default` (`:799`) and is no wait. Registration refuses any other kind earlier still: `ValidateWaitSelector` (`metasystem/internal/run/waiter.go:400-437`) accepts only job, run, attempt and goal.

Two shapes of work the seats now run write none of those records:

- A headless design delegate is its own `claude -p` process, started with `nohup` and a pid file per run (`S5/fable-design-launch.sh:14-17`, where S5 is the seat's scratch directory). The engine dispatched nothing; there is no job record.
- A chain step (a Codex build, an Opus read) runs as an in-process subagent of the seat's own runtime process. It has no process of its own and no record.

At Stop, idle enforcement asks `work.HasDelegateJobInFlight()` (`turnverdict.go:1018`), which counts only `job:` activities (`metasystem/internal/goal/project.go:227-234`, built at `:516`), and `waits.hasWorkInFlight()` (`turnverdict.go:264-270`). Both are false; the gate refuses with `idle-with-backlog` (`:1023-1025`). That is m1b's three consecutive refusals of 2026-09-16 while a Fable design was running the whole time. The seat was not idle; the engine had no record that said so.

The human-answer half of the goal's Intent is a different case, handled in section 5: the record exists already and the gate reads it.

## 2. The decision

Three answers were on the table. The goal's own rule is that the answer cannot be "read Claude Code's task list"; the parent adds that no runtime hook, native subagent, transcript parser or notification service carries correctness (`coordinator-wakes-on-events-not-polls-design.md:176`) and that the wake decision belongs to the Go steward and the records, the same way for every runtime (`:167`).

**A job record for harness-tracked work: rejected.** Job records are owned by dispatch, with guarded transitions and publication hints (`RecordSetup`, `RecordProtocolError`, `RecordCAS`; parent amendment r4 §7.3a), a steward role that declares non-terminal jobs with dead recorded processes dead (`metasystem/internal/steward/health.go:1292`), an abandon verb that refuses while non-terminal jobs exist (`metasystem/internal/goal/abandon.go:164`), and consumers that expect an operation id, a round, a role packet and an adapter return. A husk record for a delegate the engine never dispatched, cannot cancel and cannot adjudicate would claim custody it does not have; every consumer of job records would have to learn the husk. That breaks the one-owner rule and the honest-boundary rule of `docs/design/design-principles.md`.

**An adapter-reported liveness fact: rejected.** The adapter would answer "is this session busy" by reading the runtime's task list or transcript, which is the excluded answer, and it cannot see a headless process at all, since that process is not a subagent of any session. Adapters keep their one role from the parent: deliver a nudge (`metasystem/internal/adapter/wait.go:26`).

**A fifth registered-wait kind, `local`: chosen.** The record is the waiter row the engine already owns (schema 2, `WaitersDir` and `WaiterPath`, `waiter.go:208-213`), written only by the Go wait verb, checked at Stop by the existing owner, session, lease-epoch, process-birth, deadline, boot-clock and freshness chain (`turnverdict.go:590-625`, `:708-749`). Its target is a process birth identity (`identity.Ref`, `metasystem/internal/identity/identity.go:25-31`) and, when the work has no process of its own, a file the work writes when it is done. Every runtime has a shell, a pid and a file. The gate reads the row and asks the kernel; it never asks the harness.

Runtime-independence check, item by item: registration is a shell command any runtime can run; the liveness fact is `identity.AliveRef` (`identity.go:187`), the same kernel read the gate already trusts for the waiter itself (`turnverdict.go:726-729`); the end-of-work fact is a file; the harness's own re-invocation of the seat when a subagent finishes is an accelerator in the parent's sense (`coordinator-wakes...design.md:130`), never the guarantee.

## 3. The mechanism

### 3.1 Registration

```text
metasystem wait --local NAME (--pid PID | --in-process) --goal GOAL [--done-file PATH] [--timeout D] [--json]
```

- `NAME` follows the existing identifier grammar (`waitIdentifierRe`, `waiter.go:361`): `design-sgh`, `chain-u2c-read`. The row path is `local-<NAME>-<ownerDigest>.json`, so one live local wait exists per name per owner; a second registration under a live one returns 64 by the existing rule (`waiter.go:274`).
- The selector is `{kind: local, targetId: NAME, goalId: GOAL}`. `ValidateWaitSelector` gains a `local` branch: `GoalID` required and equal to the joined goal, and the ledger-event arguments (event, after, verb, question, chain, poll) forbidden, extending the `else` at `waiter.go:435`.
- With `--local`, `--goal` is the join and is required; it is counted as a selector only when `--local` is absent (`selectors` loop, `metasystem/cmd/metasystem/wait_verb.go:96-104`). The gate joins every wait to a claimed goal (`turnverdict.go:756`, `:763`, `:770`, `:776`); a local wait has no record to derive the goal from, so the caller names it.
- `--pid PID` names a process the seat launched. At registration the verb reads its exact birth through the prober (`identity.go:197-201`). Dead is exit 4, the existing "missing, replaced or invalid source record"; Unknown is exit 66. A target that is the caller's own main process requires `--done-file` (reason under `--in-process`).
- `--in-process` names work that runs inside the seat's own runtime process. The target is the main's own birth identity, read from the classified announcement (`view.Announcement`, the same fields the gate reads at `turnverdict.go:693-694`). `--done-file` is required: in-process work has no process of its own, so the file it writes is its only end fact, and the DONE clause "the wait ends when the work ends" would otherwise be unprotected.
- `--done-file PATH` is absolute. The work is done when the file exists and is non-empty. The verb and the gate only `Stat` it; nobody parses it. Optional with `--pid`.
- `--timeout` keeps the shared bound: default and maximum 24 hours (`wait_verb.go:85`, `:106-109`). No local-specific cap: process death and the done-file are the end facts, and a cap that breaks on a long but real run would be the defect. The seat chooses a bound that fits the work.

The row gains no new top-level field. `WaiterTarget` (`waiter.go:46-53`) gains five optional fields for local work: `workPid`, `workPidStartedAt`, `workPidStartTicks`, `workBootId`, `doneFile`. Struct equality keeps pinning one incarnation (`turnverdict.go:714`, `waiter.go:748-750`). Owner, session, runtime session, lease epoch, waiter birth, deadline, boot deadline, freshness, delivery, hint receiver and pointer are written exactly as today (`waiter.go:886-898`).

### 3.2 Observation

A new source owner, `ObserveLocalWork` in `metasystem/internal/run/localwork.go`, takes the selector, the pinned target and a prober. The run package already imports identity (`waiter.go:18`); local work has no other owner.

| Kernel says | Done-file | Observation |
| --- | --- | --- |
| Alive | absent or empty | pending; `Incarnation` = pinned target |
| any | non-empty | final, exit 0, outcome `done-file`, reason "local work NAME wrote PATH", evidence `local:<NAME>:<pid>:<startTicks>:done-file`, terminal stamp = file mtime |
| Dead | absent or empty | final, exit 2 (the parent's "unknown terminal"), outcome `process-ended`, reason "local work NAME (pid N) ended without a done-file; read its own result", evidence `local:...:ended`, terminal stamp = observation time |
| Unknown | any | `Temporary: true`; the waiter's existing bounded retry applies (`waiter.go:732`) |

A reused pid reads as Dead because the birth identity differs (`identity.go:194-203`). The exit table of the parent is unchanged; local uses 0, 2, 4, 64, 66, 67, 124 and 130 with their existing meanings.

The command layer wires the new case beside the four existing ones in the `waitOptions` kind switch (`wait_verb.go:272-291`), passing the verb's prober. The waiter loop is unchanged: its 10-second pause (`waiter.go:777`) already keeps `lastObservedAt` inside the gate's 30-second freshness window (`turnverdict.go:737`).

### 3.3 The Stop gate

`registeredWaitSource` gains `case "local"` (`turnverdict.go:751-801`):

1. `claimed[row.Selector.GoalID]` must hold, as for the goal kind (`:776`); otherwise no wait.
2. The gate re-reads the source at Stop, as the parent requires (`coordinator-wakes...design.md:134-136`): `identity.AliveRef(s.prober(), target birth) == Alive` and the done-file absent or empty. Anything else is no wait. Unknown liveness grants nothing (parent §4).
3. `wait.goalID = row.Selector.GoalID`; a new `local bool` on `registeredWait` (`:209-217`) for display.

Eligibility (`registeredWaitEligible`, `:719-727`): the rule "goal kind carries a goal id equal to its selector; every other kind carries none" becomes "goal and local kinds carry a goal id equal to their selector; every other kind carries none". The empty-target refusal (`:714`) still applies; a local target always carries a pid.

Effect: a local wait is not `humanAct`, so `hasWorkInFlight` (`:264-270`) is true. That is exactly row 1 of the parent's table: the idle counter resets (`:1018-1021`) and the work-in-flight branch reports it (`:1479-1481`). It covers no job and no run (`watchesJob`, `watchesRun`, `:273-290` unchanged); an unrelated unwatched job keeps its warning. `lines()` (`:304-315`) prints:

```text
WAITING: registered wait <id> covers local work <NAME> (pid <N>) for goal <GOAL> until <deadline>
```

The contract document's work-in-flight paragraph (`metasystem/docs/design/turn-verdict-delivery-contract.md:73-77`) gains "local" in its list and one sentence: a local wait's source is the target process's kernel liveness and its done-file, re-read at Stop.

### 3.4 Ending the wait

No new verb. Four existing ends:

- The target process dies: the next observation, at most 10 seconds later, returns exit 2, or exit 0 when the done-file is non-empty.
- The done-file becomes non-empty: exit 0.
- The seat ends it early: `kill -TERM <waiter pid>`, the pid the seat saved when it started the background command. The verb's signal context (`wait_verb.go:163`) finishes the row as `interrupted`, exit 130 (`waiter.go:814-816`).
- The deadline: exit 124.

In each case the row leaves `pending`, and the gate stops seeing it (`turnverdict.go:709`). A waiter that is killed without cleanup fails the waiter-birth check (`:726-729`) and is replaced by compare-and-write at the next registration under its key (parent §2). The wait never kills its target (parent §4).

### 3.5 Both launch shapes

Headless design (the 2026-09-16 shape):

```text
S5/fable-design-launch.sh sgh S5/sgh-design-prompt.md        # writes S5/sgh-design.pid, result to S5/sgh-design.json
metasystem wait --local design-sgh --pid "$(cat S5/sgh-design.pid)" --goal stop-gate-sees-harness-tracked-work \
  --done-file S5/sgh-design.json --timeout 3h &      # background; save $! to end it early
```

`claude -p --output-format json` writes its result at exit, so the done-file becomes non-empty at the moment the process ends and the wait returns 0. A crash leaves the file empty and the wait returns 2.

In-process chain step: the seat launches the subagent with an output file, as every delegate launched on 2026-09-16 already has, then registers `--local chain-<tag> --in-process --goal <goal> --done-file <output> --timeout 2h` in the background. The wait ends when the output file is written; the harness re-invokes the seat at about the same moment, which is the accelerator, and the seat's next `goal next` prints no pending row.

The verb runs as a background command in both shapes because a seat's foreground tool call is bounded far below a design run's length, and the whole point is that the seat's turn ends. Registration classifies the caller once, while it is still a descendant of the main (`classifyVerbCaller`, `wait_verb.go:144`); afterwards the gate checks the row's owner, session and lease, never the waiter's ancestry (`turnverdict.go:594-621`). `TestPendingWaitFromChildShell` (`wait_verb_test.go:1147`) already proves registration from a child shell under its own pid.

The delegate briefs used by seats gain one line: write the output file last, by temp-and-rename, so a non-empty file means done. The seat's launch script is scratch and outside the repository; the seat updates it when unit 3 lands.

## 4. Rules, witnesses, mutations

Every witness runs with the fixture's injected clocks (`turnverdict_test.go:204-206`) and a fake prober (`idleFixtureProber`, `turnverdict_idle_test.go:102-103`); no wall-clock sleeps.

| Rule | Witness that fails without it | Named mutation |
| --- | --- | --- |
| R1 A local wait joined to a claimed goal with a live target is work in flight, resets the idle counter and prints its WAITING line. | `TestPendingWaitIdleBacklog` kinds loop (`turnverdict_idle_test.go:116`) gains `local`, with the fixture builder (`turnverdict_test.go:150-211`) given a `local` kind whose target pid is alive in the prober; asserts the same verdict and counter as the live-delegate control and the WAITING text naming the work and goal. | Delete `case "local"` from `registeredWaitSource`: the subtest sees `idle-with-backlog`. |
| R2 A dead target grants nothing. | Subtest "local wait with a dead target": the prober lacks the target pid; verdict equals the no-wait control. | Replace the gate's `AliveRef` result with `Alive`: the subtest is allowed. |
| R3 A non-empty done-file ends the allowance at Stop. | Subtest "local wait whose done-file is written": write one byte to the path; verdict equals the no-wait control. Empty file: still allowed. | Remove the done-file `Stat` from the gate's case: the written subtest is allowed. |
| R4 A local wait must name a claimed goal, and the row's goal id must agree with its selector. | Subtest: selector goal not in `work.Claimed` is no wait; row `GoalID` empty while selector names one is no wait. `TestWaitVerbArgumentsAndResult` (`wait_verb_test.go:55`): `--local` without `--goal` exits 67. | Drop `claimed[...]`: the unclaimed subtest is allowed. Drop the eligibility clause: the forged-row subtest is allowed. |
| R5 The observer maps kernel and file facts as in the table of 3.2. | `TestObserveLocalWork` in `internal/run/localwork_test.go`: alive+absent pending; non-empty file exit 0 `done-file`; dead+absent exit 2 `process-ended`; unknown temporary; reused pid (different start ticks) dead. | Swap exits 0 and 2: two cases fail. Treat an empty file as done: the empty case fails. |
| R6 Registration refuses a dead `--pid` with 4, an uncertain one with 66, `--in-process` or self-pid without `--done-file` with 67, and records the target birth in the row. | `TestWaitVerbArgumentsAndResult` cases; a row-shape assertion on the written JSON. | Remove the self-pid check: its case registers. Skip the birth read: the row-shape assertion fails. |
| R7 End to end, the installed verb and the installed Stop verdict agree on a real child process. | `TestPendingLocalWaitFromChildShell` beside `TestPendingWaitFromChildShell` (`wait_verb_test.go:1147`): start `sleep`, register from a child shell, installed verdict allowed and names it; `kill` the sleep; verdict refused at the next Stop; repeat with a done-file instead of a kill. Bed leg `wait-local` in `metasystem/scripts/agents/supervision-fixtures.sh` (pattern of `wait-stop-fake`, `:695-702`; list at `:51`, case at `:58`) runs it, registered in `metasystem/testing.json`. | The runnable-surface proof for `skills/verify/SKILL.md`; it fails under any of the R1 to R6 mutations. |
| R8 A pending human question with a deadline allows the stop against open work; a dead waiter does not; idle backlog is not exempted. | The existing `TestPendingWaitTurnVerdict` human-act and channel cases (`turnverdict_test.go:386`) and `TestPendingWaitIdleBacklog` "human and channel waits never exempt idle backlog" (`turnverdict_idle_test.go:136`). The builder of unit 2 cites the allowed and dead-waiter subtests by name, and adds the dead-waiter channel case only if absent. | Already covered by the parent's member three; the mutation is that member's. |

## 5. The human question

The record exists. `channel ask --goal G --kind K ...` (`metasystem/cmd/metasystem/channel_verbs.go:107-119`) opens a durable question; `channel wait --id Q --timeout MINUTES` (`:211-219`) registers a goal human-act answer wait with channel polling through the same verb (`wait_verb.go:120`; validation `waiter.go:417`, `:432`). The deadline is the wait's deadline, at most 24 hours.

The gate's rule for it is the parent's row 2, landed by member three and written into the contract (`turn-verdict-delivery-contract.md:85-92`): the wait suppresses open work only while the goal is reported waiting on a human (`turnverdict.go:244-250`, `:1441-1442`) and never exempts idle backlog (`:264-270`). So the DONE clause "a human wait allows the stop" holds where the wait is the thing that turns an open-work block into an allowance, and it cannot exempt idle backlog under the accepted parent design: a seat with claimable backlog and a pending question takes the backlog, and the answer arrives as a ledger act the waiter returns on. This page adds no second human wait kind and no exemption; R8 names the proof.

If the empty turns of 2026-09-15 on m1c happened with claimable backlog, the gate was right and the fix is the seat claiming; if they happened without a registered question (the wait verb landed later), this page's registration path is the fix. Open question 1 records the choice that would change this reading.

## 6. Units, in landing order

Each unit is at most 300 changed lines, lands with its witnesses, and keeps every existing command usable. Builds by Codex gpt-5.6-sol in a worktree, reads by Opus, critique by another model.

**Unit 1 (first): record and observer.** `metasystem/internal/run/waiter.go` (five `WaiterTarget` fields; the `local` branch of `ValidateWaitSelector`), new `metasystem/internal/run/localwork.go` (`ObserveLocalWork`), `localwork_test.go`, selector cases in `waiter_test.go`. About 150 lines. Witnesses R5 and the validation half of R4. Lands alone safely: the gate's `default` still ignores local rows, so nothing changes at Stop until unit 2.

**Unit 2: the gate and the contract.** `metasystem/internal/goal/turnverdict.go` (eligibility `:719-727`; `case "local"`; the `local` field; `lines()`), the `local` fixture kind in `turnverdict_test.go:150-211`, subtests in `turnverdict_idle_test.go` and `turnverdict_test.go`, the paragraph in `docs/design/turn-verdict-delivery-contract.md:73-77`. About 200 lines. Witnesses R1 to R4 and the R8 citation. Fixtures write rows directly, as today, so this unit does not need unit 3.

**Unit 3: the verb, the bed, the doctrine.** `metasystem/cmd/metasystem/wait_verb.go` (`--local`, `--pid`, `--in-process`, `--done-file`; selector counting; target birth at registration; the `waitOptions` case), `wait_verb_test.go` (R6 cases; `TestPendingLocalWaitFromChildShell`), `metasystem/scripts/agents/supervision-fixtures.sh` leg `wait-local`, `metasystem/testing.json`, and the paragraph of `metasystem/docs/orchestration.md` that documents `metasystem wait` (one sentence per launch shape of 3.5). About 250 lines. Witnesses R6 and R7.

After unit 3 every seat installation rebuilds its enrolled engine, as the parent already requires for member landings (`coordinator-wakes...design.md:260`).

## 7. What does not change, and what is out of scope

- The parent owns the waiter loop, hints, delivery, resume, recovery, the exit table, rows 1 to 5 of the stop table, and clause 5 measurement. Resume of a local row works unchanged: the observer is chosen by kind.
- No job record, no `job:` activity, no steward reaping of local work, no cancellation by the wait.
- The stop-status report's counter line "Claimable goals; queued goals; non-terminal jobs" (`metasystem/internal/report/stoppresentation.go:1208`) and its delegate sentence (`:1327-1334`) are unchanged; the WAITING line in the verdict display is the naming the DONE asks for.
- No adapter, hook or launcher change. The seat's scratch launch script is not a repository deliverable.
- Seats on runtimes that cannot hold a background shell command: the parent's follow-on goal `managed-seats-wait-through-a-holder` (`coordinator-wakes...design.md:259`).
- Out of the DONE: a registry of local work beyond the wait row, spend accounting for headless delegates, and the report wording "no task in flight".

## 8. Risks

- A seat registers work that is not real. The gate cannot judge truthfulness, only that the named process lives and has not finished; the WAITING line names the work and goal, and the row is auditable. A registration without a real end fact is refused (R6). The same trust already applies to what a seat writes in a plan's open steps.
- A done-file written early or partially. Non-empty means done, so delegates write it last by temp-and-rename; the file is never parsed.
- A headless process that never finishes. The deadline bounds it; the seat reads the result at its next turn; nudging an idle session at the deadline is the parent's delivery.
- A loaded machine that delays the waiter loop past 30 seconds. The gate refuses, on the safe side, as for every kind; tests inject clocks and never sleep.
- Pid reuse. The birth identity makes it Dead.

## 9. Open questions for the seat

1. This page reads the DONE's human clause as met by the existing channel answer wait under the parent's row 2, which never exempts idle backlog. If Wido intends a pending question to exempt idle backlog as well, that amends the parent's row 2 and its contract paragraph, and belongs to the parent goal, not this one.
2. Whether `report stop-status` should print the WAITING line beside its counter line so the report, not only the hook display, names the local work. This page leaves the report unchanged.
