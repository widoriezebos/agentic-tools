# Coordinator wakes on events, not polls

Design for goal 14 in `metasystem/plans/delivery-efficiency-plan.md`.
The full intent is `metasystem/plans/goals/coordinator-wakes-on-events-not-polls.md:8`.
Drafted for the m1b seat on 2026-09-13 by Codex design chain implementer-fa5b5830882bcf6ea864cdf6. The second rostered read is folded here; the member goals of section 6 follow.
Waiting stays in an engine process. Only an event, a deadline or a named failure needs the model.
Generated record locations below use their existing Go locator functions; they are local state, not checked-in files.

## 1. The wait sites today

These are twelve wait sites or entry-point groups. Process polls inside one command are distinct from model turns that repeat it.

| Site | File:line evidence | Record and ending event |
| --- | --- | --- |
| Job watch and its aliases | `metasystem/cmd/metasystem/run.go:433`; `metasystem/cmd/metasystem/watch_verb.go:13`; `metasystem/scripts/agents/dispatch.sh:1980` | All reach `metasystem/internal/dispatch/watch.go:18`. It reads job status every two seconds. Completed, failed, timeout or cancelled ends the wait; missing or malformed records fail separately. |
| Synchronous delegate completion | `metasystem/scripts/agents/dispatch.sh:1089`, called at line 1969 | `wait_for_job` reads the job record and invokes reaping every 100 milliseconds. The same four terminal statuses, disappearance or reaping failure end it. |
| Adapter process ownership handshake | `metasystem/scripts/agents/dispatch.sh:1028` | The pending job and operating-system process identity must join. A successful ownership patch or the startup ceiling ends the loop. |
| Runtime session handshake | `metasystem/scripts/agents/dispatch.sh:1062` | Job status, session identifier and log existence are checked repeatedly. Running or completed with a session and log ends it; failure or the recorded handshake deadline also returns. |
| Tracked run completion | `metasystem/cmd/metasystem/run.go:240`; `metasystem/internal/run/waiter.go:220` | Run records are polled every two seconds. Green, red, ended-unknown or launch-failed ends it. Missing records or a changed generation and launch nonce invalidate the target. |
| Proof completion | `metasystem/cmd/metasystem/test.go:906`; `metasystem/cmd/metasystem/proof_run.go:153`; `metasystem/internal/proofrun/launcher.go:378` | The command waits for suite, watchdog and output drains. The durable outcome is the attempt terminal committed at launcher line 478, not process exit. `metasystem/internal/proofrun/attempt.go:141` makes nil Terminal nonterminal work. |
| Landing publication | `metasystem/scripts/agents/land.sh:1081` and line 1095; `metasystem/internal/goal/txn.go:689` | Code publication waits for push and may retry transport. Ledger publication refetches and finds its operation trailer. A seat awaiting either needs the matching commit on the configured ledger ref; a local commit or push message is insufficient. |
| Human act on a goal | `metasystem/cmd/metasystem/goal.go:456`; `metasystem/internal/goal/txn.go:653` | Goal show reads the accepted projection once and returns its tip. Another read or notification is needed today. A published goal History act after that tip is the event. |
| Background notification delivery | `metasystem/internal/steward/intervene.go:299`; `metasystem/internal/steward/notify.go:109` | PendingNotification survives until successful delivery. Delivery ends this transport wait; only the job, attempt or ledger record can end the underlying work wait. |
| Harness watch, TaskOutput, tmux capture loops and Monitor | `metasystem/plans/goals/coordinator-wakes-on-events-not-polls.md:10` | These are the harness patterns named by the goal. They wait for command output or repeatedly inspect it. The underlying job, run, attempt or ledger event above is the real condition. This checkout supplies the goal's inventory, not the source transcripts. |
| Stop-induced turns to inspect work again | `metasystem/internal/goal/turnverdict.go:731`, line 947, line 982 and line 543; `metasystem/scripts/agents/supervision-hook.sh:1203` | Unwatched work, open work, a goal revision and idle backlog can request another turn. The current work-in-flight exception requires a live delegate job. There is no equivalent for an attempt or goal wait. |
| Channel wait | `metasystem/cmd/metasystem/main.go:457`; `metasystem/cmd/metasystem/channel_verbs.go:199` | The channel family's wait verb reads a durable human-question record and polls the provider. A recorded answer ends it. Its deadline is unbounded by default. Under this design it registers a human-act wait on the question's goal with --verb answer and --question ID. Only an answer naming that question ends it; another question's answer on the same goal does not. It takes the 24-hour default deadline and follows row 2 of the stop table. |

Startup gates, custody retries and heartbeat loops also block inside adapters: `metasystem/scripts/agents/adapters/runtime-common.sh:72`, line 119 and line 310.
They are bounded launch or supervision plumbing. They do not themselves require another model turn.
Bare watch is a one-shot snapshot (`metasystem/cmd/metasystem/watch_verb.go:21`).
Steward ticks and channel polls likewise stay in processes. Count a model wake only when inference starts.

## 2. The mechanism

Extend the existing waiter owner in `metasystem/internal/run/waiter.go`. It owns registration, blocking, deadlines, hint delivery and recovery. Source owners decide their own predicates.
`metasystem/internal/dispatch/watch.go` supplies job observations; `metasystem/internal/proofrun/attempt.go` supplies attempt observations.
`metasystem/internal/goal/attention.go` supplies validated ledger observations. The command layer only wires these ports.
The waiter package imports none of those source owners. The stop gate consumes their typed observations through its scanner.

The public command is `metasystem wait`, with exactly one of `--job ID`, `--attempt ID`, `--goal ID`, `--run ID` or `--resume WAIT-ID`.
Common arguments are `--root DIR`, `--timeout DURATION` and `--json`. Default and maximum timeout are 24 hours. This bounds an absent human without purchasing a model turn every half hour. Callers can choose a shorter bound.
Timeout must be positive and includes registration and cleanup. There is no model-facing poll interval.
Goal waits require `--event landing|human-act` and `--after COMMIT`, the accepted ledger tip read before launching or asking.
Human-act waits may add `--verb VERB`; landing waits may add `--chain ROOT-JOB-ID` to select one delegate chain.
For an answer, the human-act selector carries `--verb answer --question ID`. Save the question identifier with the selector.
Resume restores the saved selector and cursor. It accepts no replacement target or cursor. Without a new timeout it keeps the original deadline. An explicit new timeout starts a recorded renewal, never a silent extension.

Every return prints one result with wait identifier, target incarnation, reason, source outcome and source evidence identity. JSON adds registeredAt, deadline, observedAt, returnedAt, ledger tip or terminal stamp, and the replay command.
Exit 0 means successful terminal or matching ledger event; 1 means failed/red; 2 means target timeout or unknown terminal.
Exit 3 means target cancelled or launch-failed; 4 means missing, replaced or invalid source record.
Exit 6 means the waiting seat has actionable work or its ownership changed; the result names which fact changed.
Exit 64 means an existing live waiter or ineligible registration; 65 means storage or transport failure; 66 means uncertain identity.
Exit 67 means invalid arguments, 124 means this wait's deadline, and 130 means interruption of this wait.
A target timeout differs from a wait timeout. None of these exits cancels, concludes, approves or certifies the target.
These new exits belong to `metasystem wait` alone. Compatibility wrappers call it and remap its typed source outcome and reason.
The `delegate --wait` mapping in `metasystem/scripts/agents/dispatch.sh:1089` stays completed 0, failed 3, timeout 4 and cancelled 8.
A missing record, malformed status read or unknown status each returns 5. The wrapper maps the new verb's exit 4 for missing, replaced or invalid source records to 5.
Job watch and run watch keep their own current exit mappings through the same wrapper approach.

The registered wait is the existing Waiter record with `schemaVersion: 2`, located by WaiterPath and WaitersDir in `metasystem/internal/run/waiter.go:45`. Resolve the installation state root once, using the same rules as the stop gate.
WaitersDir means that state root's artifacts directory, then agents, then waiters.
Its file name stays kind, target identifier and owner digest. This digest is a fixed-length fingerprint that Go computes from the seat's owner identity.
The waitId contains 128 random bits; each registration also gets a fresh nonce. A live row for the same key returns 64, never a second row.
Inside WaitersDir, a by-id subdirectory holds a pointer named by waitId. It contains the row's file name.
Go writes the pointer under the waiter lock at registration and removes it with the row. Resume by waitId reads this pointer. A missing pointer is a named error; the row may still be found by its kind, target and owner key.
Store the classified main lineage, session, checkout lease epoch, runtime session reference and waiter process birth identity.
Store the selector, goal binding, source incarnation, original ledger cursor, last checked tip and source result identity.
Store the registration time, UTC and boot-relative deadlines, boot identifier, remaining duration, last successful observation in both clocks and open-work signature.
The platform identity reader measures time since the current operating-system boot; Go uses that clock to compute the boot-relative deadline.
The open-work signature is the stop gate's existing digest of the open items, computed by OpenWorkSignature in `metasystem/internal/goal/turnverdict.go:88`. Saving it covers exactly the items present at registration, and nothing that appears later.
State is pending, ready, deadline, interrupted or failed. A terminal row retains the result and selector for replay.
Only Go waiter operations write this record. Adapters and source writers send hints; they cannot mark it ready.
The waiter clears pending before returning, by a durable compare-and-write on waitId and registration nonce.
It closes and removes only its own hint endpoint. Terminal rows stay until an explicit replacement wait for that key.
A row without schemaVersion is legacy. Read it only for legacy watch liveness; it cannot grant the pending-wait exception.

Registration first validates the caller, target and cursor. It captures one immutable source incarnation.
Jobs pin job identifier, round and startedAt; runs pin identifier, generation and launch nonce.
Attempts pin attempt identifier and proof identity digest. Never follow an identifier onto a successor attempt or job round.
Create the hint receiver, publish pending under the existing bounded waiter lock, then reread the source before sleeping.
A terminal seen at entry returns immediately. An event between reading, registering and blocking is therefore not lost.
Each wake rereads the record through its owner and checks the incarnation before deciding.
For attempts, require a validated Terminal with EndedAt equal to Terminal.At. Nil Terminal, logs and worker death cannot mean success.

Goal observation freezes the configured endpoint with the cursor, fetches a private tip and applies the existing ledger acceptance gates.
These checks validate the fetched ledger's identity, history and state before accepting its commit as evidence.
Use CaptureTipBounded in `metasystem/internal/goal/attention.go:112`; never read the worktree's goal copy as fresh truth.
Keep the original cursor even as the last checked tip advances. Inspect intervening accepted states, including archived goals.
Human-act matches a new History operation targeting the goal, with the requested verb if supplied.
For answer, the saved selector and ledger predicate also require that act to name the supplied question identifier.
An answer to another question on the same goal is not the event; the wait continues.
Use the goal owner's validated human authority classifications, including accepted channel or relay acts; a seat acting under power of attorney is still a seat.
Return the actual act, including refusal, unapproval or parking. Waking on an act grants no authority to continue.
Landing matches a commit newly reachable after the cursor with the exact Goal-Item and a valid Landing-Provenance trailer.
Trailers are named fields at the end of a Git commit message that identify the goal, delegate chain or ledger operation.
These trailers are written at `metasystem/scripts/agents/commit.sh:788`. A chain filter must match that provenance's chain.
A ledger-verb publication can also be selected with human-act and its verb; its History operation must join the Goal-Transaction trailer.
An unrelated ref advance, local commit, push acknowledgement or entering the landing phase does not match.
Check the landing destination against that endpoint at registration. A different code destination is a named invalid-target refusal, not permission to watch HEAD or another branch. Local-mode fixtures publish to the dedicated local ledger ref.
If a rewind or missing history prevents proving the cursor relationship, return a named source failure. Never replace the cursor with now.

The engine rechecks pending sources every ten seconds, without printing progress or invoking a model.
Each observation of a human-act wait also checks whether another goal is claimable. As soon as one becomes claimable,
the waiter clears pending and returns 6 (actionable work), so the seat takes the turn to claim it without a forced turn while pending.
All reads, fetches, hint writes and lock acquisition take the remaining wait budget; no wait lock spans a source read.
Fetch work has a ten-second ceiling plus the existing five-second transport cleanup allowance, both reserved inside the deadline.
Make local-mode and validation Git calls bounded too; bounding only the remote fetch leaves a hidden unbounded path.
Transient fetch failure retains pending only while the last successful observation is at most 30 seconds old.
After that, return 65 and revoke the exemption. A broken or vanished source returns its named error immediately.
At the deadline, stop observing, durably clear pending and return 124. The underlying work continues under its existing supervisor.

The optional accelerator is a private named pipe beside the waiter record, created by Go with owner-only permissions.
Its name includes the registration nonce. Hints contain only the wait identifier and nonce; writes are nonblocking and bounded.
Source writers hint after durable publication: the job record transition, attempt terminal commit and confirmed ledger publication.
Landing transport may hint after pushing; a false hint is harmless because the ledger must still prove the landing.
The internal `metasystem wait notify` operation accepts a typed job, attempt or goal selector and hints matching registrations.
Runtime adapters use the same operation for a correlated native notification. No provider text enters the predicate evaluator.
Duplicate, late, forged, reordered or missing hints cannot change a result. Coalesce them and limit hint-triggered reads to one per second.
If pipe creation or delivery is unavailable, record accelerator=unavailable and retain the ten-second fallback.

There is one route: the seat runs one foreground `metasystem wait` command. The runtime keeps it open until its result or deadline.
Nothing is spawned to hold it. The wait-delivery operation in `metasystem/scripts/agents/adapters/runtime-common.sh:3` is the conformance seam.
It takes waitId, nonce, deadline and session reference. At registration, Go asks once and records the answer before publishing pending.
Exit 0 must return the single word blocking. Exit 2 declines; registration refuses with a named reason and grants no exemption.
A decline leaves today's watch commands and today's stop decisions in force.
Claude Code, Codex, Devin and the fake adapter all run a shell command to completion, so every adapter answers blocking.
The adapter owns provider flags. It never asks the model to check TaskOutput, capture a terminal or poll a handle while pending.
Claude background-task notifications and Codex or Devin session events enter through their adapters only as hints into the pipe.
A native wake only triggers a record check. Without it, the same foreground command uses bounded record reads.
Post hoc event projection in `metasystem/internal/adapter/events.go:3` is not native wake support.
The steward's operator queue remains an optional notice channel, not the wait record or evidence that the underlying event happened.

The stop scanner supplies PendingWait only for this classified session and lineage, with a current lease epoch,
a live waiter whose process birth matches, an unexpired deadline, a fresh successful observation and a target still pending.
It checks the current source at Stop too. A landing wait must also join the goal's recorded landing phase.

| Row | Condition | Stop decision |
| --- | --- | --- |
| 1 | A valid job, run, attempt or landing wait joins the seat's claimed goal. | It is work in flight. It has exactly the effect a live delegate job has today on the unwatched-work join, on open work with the saved signature, and on the idle-with-backlog refusal and its counter. This page adds no counter rule of its own. |
| 2 | A valid human-act wait covers its goal. | It suppresses only open work with the saved signature and that goal's waiting-on-human condition. It never exempts idle-with-backlog. As soon as another goal becomes claimable, the waiter clears pending and returns 6 (actionable work). The seat takes the turn to claim it; no forced turn happens while the wait is pending. Channel wait follows this row. |
| 3 | A wait fails any eligibility check. | It changes nothing. Today's decisions apply. |
| 4 | Open work has a different signature from the saved one. | It is never suppressed. The wait returns 6 on its next observation so the seat takes the turn; the stop gate then decides as today. |
| 5 | Every row above. | Fences, human stop authority, warnings and degraded-input behaviour remain unchanged. |

Apply this table in `metasystem/internal/goal/turnverdict.go` to the unwatched-work join, work-in-flight branch and idle check.
Unrelated subscriptions grant no exemption. An attempt covers its governed run only if the recorded generation and nonce match.
Unrelated unwatched work keeps its existing decision.
Return a visible WAITING line with target and deadline, without requesting another turn on the covered wait.
`metasystem/scripts/agents/supervision-hook.sh` transports this verdict unchanged for Claude, Codex and Devin.
The same table feeds goal next and plain report turn-verdict without hooks; future adapters owe the same conformance test in `metasystem/docs/design/turn-verdict-delivery-contract.md`.

After a missed hint, the timer or immediate reread finds the durable event. The rows are the queue; no daemon, journal or launcher is needed.
Clause 4's recovery guarantee is the rows plus `metasystem wait --resume WAIT-ID`. Any seat can run this command at any time after a restart, subject to the ownership checks below.
Where a runtime has a session-start hook, it provides the first-act ordering as an accelerator, not the guarantee.
Claude Code uses the supervision hook's `start` event and the engine's `session start` verb to show the commands; the model resumes as its first act.
Without such a hook, the seat's first `goal next` or `report turn-verdict` prints the same WAITING lines with resume commands.
Both paths enumerate this checkout's rows whose owner lineage the new session succeeds under the lease records.
Each pending row gets one WAITING line with the exact `metasystem wait --resume WAIT-ID` command. Enumeration launches nothing.
Resume takes the waiter lock and writes resumedBy with the session and process birth. A second resume returns 64 while the first is live.
Re-prove succession and goal ownership; never take another live seat's wait. Replace a dead waiter by compare-and-write with a new nonce.
Uncertain liveness returns 66. Prefer the current owner's row; copy a proven predecessor's selector to the successor key under the lock, update its pointer and invalidate old pending state. Old cleanup cannot remove the successor's row or pointer.
First replay a saved terminal result after checking its source evidence. Otherwise an expired plain resume returns 124.
Only explicit renewal registers another bounded period. With time left, reread from the original cursor before blocking.
Old pipes, task handles, notifications and transcripts are unnecessary. A deleted registration requires the caller's original cursor for a new goal wait; a missing cursor is an error, never an inferred new baseline.

## 3. What does not change

Job custody, reaping, proof admission, proof reuse, landing authorization and human authority keep their current owners.
Startup handshakes remain bounded process work. Synchronous launch interfaces remain available.
The model's prescribed completion wait becomes this verb; the synchronous delegate wrapper stops owning its own status loop.
Bare watch stays a zero-write snapshot. Old job and run watch syntax remains compatible.
Stop still refuses open work and idle backlog under the existing rules whenever no valid registered wait covers the condition.
No runtime hook, native subagent, transcript parser or notification service carries correctness.

## 4. Risks

A wait can outlive its useful work. The hard deadline, bounded I/O and freshness limit force a named return.
A live but stuck waiter is insufficient for exemption once its observation expires. The wait never kills the target to make itself finish.
A stale row can exempt an idle seat. Session, lease, process birth, target incarnation, deadline and source checks prevent that.
Unknown identity never grants exemption or permits replacing another process's row. Compare-and-write prevents old cleanup clearing a successor.
An accelerator can lie or flood. Hints only schedule bounded rereads; they never carry terminal status or approval.
A held lock can stall cleanup. Reserve five seconds for it; if clearing fails, report 65 and let expiry invalidate the row.
Use a monotonic timer while the waiter lives. On the same boot, restart takes the shortest of saved remaining time, UTC time left and boot-relative time left. A wall-clock step cannot lengthen that bound or the 30-second freshness window.
A changed boot or unreadable boot clock permits result replay but no pending exemption; return 124 before explicit renewal.
The platform identity readers supply the boot clock. Inject both clocks in tests; a future wall stamp is surfaced as clock drift.
Process identity uses boot identifier and start ticks where available, as in `metasystem/internal/run/waiter_pair_test.go:24`.
Two machines may disagree about time. Ledger commit ancestry decides event order; timestamps never decide whether a human act happened.
A native hint may arrive after the engine's next record check. Measure event-to-model-resume separately from event-to-command-return.

## 5. Fixtures

The names below are implementation obligations, not claims that these tests already exist or passed.
Use injected clocks, source readers and hint delivery for time-based Go tests. Bed processes have named scaled ceilings and isolated state roots.
Go owns assertions; existing shell beds only launch scenarios and collect their results. The orchestrator runs real runtime legs outside a delegate sandbox.

| DONE clause | Go test and existing test owner | Setup and assertion; bed leg |
| --- | --- | --- |
| 1: one bounded wait | TestWaitJobTerminals in `metasystem/internal/dispatch/watch_test.go`; TestWaitRunTerminalsAndDeadline in `metasystem/internal/run/waiter_test.go`; TestWaitAdapterBlocking in `metasystem/internal/adapter/runtime_test.go` | Exercise every terminal, already-terminal entry, missing record and reused identifier. One command returns the typed result, without model polls. Assert schemaVersion 2, the pointer and refusal of a live same-key registration. Each installed adapter must answer blocking with exit 0 and the row must record it. Fake bed leg wait-job-run exercises the installed public verb without a temporary fallback. |
| 1: proof terminal | TestWaitAttemptRequiresCommittedTerminal in `metasystem/internal/proofrun/attempt_test.go`; TestWaitAttemptAfterDrain in `metasystem/internal/proofrun/launcher_test.go` | Exit a worker and delay terminal publication. The wait stays pending until valid Terminal and EndedAt are committed; failure and cancellation return distinctly. Bed leg wait-proof includes a retained attempt and its governed run. |
| 1: ledger events | TestWaitGoalLandingAndHumanAct in `metasystem/internal/goal/txn_test.go`; TestWaitGoalCursorHistory in `metasystem/internal/goal/attention_test.go`; TestWaitChannelAnswer in `metasystem/cmd/metasystem/channel_verbs_test.go` | A two-clone bed publishes a landing, approval, denial and unrelated commits. Match goal, verb and optional chain; reject local-only commits, wrong authority, lost history and rewinds. TestWaitChannelAnswer opens two questions on the same goal. Answering the other question leaves the wait pending; only the answer naming the saved --question ID ends it. Bed leg wait-ledger covers local and remote endpoints, the human-act registration and the 24-hour default deadline. |
| 2: accelerator and fallback | TestWaitHintsOnlyTriggerReads in `metasystem/internal/run/waiter_test.go`; TestWaitDeliveryContract in `metasystem/internal/adapter/runtime_test.go` | Hint tests check each publication owner writes durably before hinting. Early, duplicate, false, wrong-nonce and dropped hints leave results unchanged; the timer remains bounded. Delivery conformance checks exact inputs and the single blocking answer with exit 0 for Claude Code, Codex, Devin and fake. Exit 2 must refuse registration with a named reason and leave existing stop decisions in force. Fake bed legs wait-native-hint and wait-no-native prove no intermediate model turn. |
| 3: Stop honours pending | TestPendingWaitTurnVerdict in `metasystem/internal/goal/turnverdict_test.go`; TestPendingWaitIdleBacklog in `metasystem/internal/goal/turnverdict_idle_test.go` | Assert every stop-table row. Compare job, run, attempt and landing waits with a live delegate job, including its current counter effects. Human-act and channel waits never exempt idle backlog. Make another goal claimable and assert pending clears with exit 6 before the seat takes its claim turn. Also test changed work returning 6, every failed eligibility check, unrelated work and unchanged fences. Bed legs wait-stop-fake and wait-stop-claude exercise hook and plain-command verdicts; Claude must reach an actual provider turn end. |
| 4: recovery replay | TestWaitRestartRecoveryReplay in `metasystem/internal/run/waiter_pair_test.go` | Crash before subscription, after registration, after source publication and after result persistence. Remove pipes and notifications. Resume from rows alone after restart, without a hook or launcher. Check that goal next and report turn-verdict print WAITING commands; separately check Claude's hook provides first-act ordering. Assert pointer lookup and its missing-pointer error, resumedBy, duplicate resume 64, original cursor and deadline, one result and no stolen live wait or stale cleanup. Bed leg wait-restart repeats real process death. |
| 1, 3, 4: failure bounds | TestWaitLockClockAndFetchBounds in `metasystem/internal/run/waiter_test.go`; TestWaitGoalFetchDeadline in `metasystem/internal/goal/attention_test.go` | Hold a lock, hang transport, regress the wall clock and reuse a process identifier. Advance fake time. Every case returns or fails pending eligibility within its stated bound; no target is cancelled. Bed leg wait-bounds also checks local Git timeout and pipe unavailability. |
| 2: compatibility | TestWaitCompatibilityMappings in `metasystem/cmd/metasystem/watch_verb_test.go` | Bed leg wait-compatibility drives job watch, run watch and delegate --wait through wrappers. Read the existing delegate mapping from `metasystem/scripts/agents/dispatch.sh:1089`: completed 0, failed 3, timeout 4, missing record 5, malformed status read 5, unknown status 5 and cancelled 8. Assert the new verb's exit 4 maps to 5. Job watch and run watch retain their own current mappings. |
| 5: measurements | TestWaitMeasurementAccounting in `metasystem/internal/usage/usage_test.go`; TestWaitRuntimeBed in `metasystem/cmd/metasystem/runtime_verbs_test.go` | Replay timestamps, overlapping pending rows, deadlines, renewal gaps, usage duplicates and missing telemetry. Assert that a gap is excluded and a renewal counts as one model wake-up. Check the arithmetic below and refuse incomplete measurement. Real bed legs wait-whole-goal-claude and wait-whole-goal-codex each carry one whole coordinator goal. |

For each real runtime, retain one session's full goal from claim through delegated work, proof, landing and the human conclusion.
Include a human-act wait, a dropped hint, a records-only recovery and one explicit one-hour timeout. Collect at least one pending hour.
Use the goal's real waiting time; do not insert delays to enlarge the denominator. Other waits use the default deadline.
Observe a fixed 24-hour seat day starting at session launch. Keep raw provider usage and normalized inference-start and resume events.
Register wait lifecycle events through `metasystem/internal/events/emit.go` and `metasystem/scripts/agents/event-registry.json`.
Adapters supply inference starts and input usage through `metasystem/internal/usage`; missing native data is unavailable, never estimated.

A prompt token is a provider-reported unit of input sent to the seat's model, including its supplied context.
An inference start is one request for the provider to run that model; a process checking a record is not an inference start.

Pending hours are the union of intervals when a wait row is pending, from registration until the waiter clears pending on event, deadline or failure.
A renewal is a new registration. Exclude the gap between deadline return and renewal; overlapping pending rows count elapsed time only once.
Count every inference request beginning inside those intervals, including polls, hook turns and unrelated wake-ups.
Also count the renewal itself as one model wake-up, even if its request began in the gap. Count the same request only once.
Divide those wake-ups by pending hours and require less than 4. Report terminal-triggered resumes separately; if one begins inside any pending interval, it still counts there. Separate reporting cannot conceal pending-time polls.
Sum prompt tokens for the counted requests, including cached input once, and divide by all coordinator input tokens in the same day.
Require less than 5 percent. Exclude delegate tokens from both sums and retain the provider's token-class mapping.
For each event, measure durable publication to the first resumed model turn consuming its wait result. Require every sample below 60 seconds.
Use a common observer clock in the two-clone bed; record clock uncertainty in live runs and fail the measurement if the upper bound exceeds 60 seconds.
Do not replace latency with notification time, command return time or an average. Record maxima and raw samples for both runtimes separately.
Absence of polling is proven by the full session event stream. Missing streams, failed delivery or insufficient pending exposure leave clause 5 unproven.

## 6. Landing

The seat opens these three member goals, each blocking this parent, and lands them in the order below.
Each keeps old commands usable and has its own DONE, focused Go fixtures and shared-contract proof. No member's DONE needs a later member.
The installed verb works after member one. Member two's wrappers call that working verb; member three adds stop-gate conformance.

| Member goal | Its DONE | Its fixtures from section 5 |
| --- | --- | --- |
| wait-verb-returns-on-recorded-events | The installed commands `metasystem wait --job`, `--run`, `--attempt` and `--goal` (landing and human-act) return on the recorded event or bounded deadline with typed exits. Every adapter's wait-delivery operation answers blocking and the row records it; no temporary fallback is needed. The version-2 row and pointer exist. Restart recovers from rows alone through --resume. This owns clauses 1 and 4 and clause 2's adapter operation. | The first three clause-1 rows, including TestWaitAdapterBlocking, TestWaitRestartRecoveryReplay and the failure-bounds row; bed legs wait-job-run, wait-proof, wait-ledger, wait-restart and wait-bounds. |
| publication-owners-hint-the-waiter | The job transition owner, attempt terminal commit and confirmed ledger publication each hint after their durable write. Dropped or false hints change no result. Wrappers call member one's working verb and preserve the current exit mappings of job watch, run watch and delegate --wait. This owns clause 2's publication hints and compatibility. | The hint portions of fixture row 4, TestWaitHintsOnlyTriggerReads and TestWaitCompatibilityMappings; bed legs wait-native-hint, wait-no-native and wait-compatibility for those assertions. |
| stop-gate-honours-a-registered-wait | The stop table holds on fake and Claude, with the existing live-job counter effects and human-act return 6. The delivery contract conformance test passes against the blocking operation already installed by member one. This owns clause 3 and clause 2's delivery conformance. It changes only the stop gate and the contract document, with their tests. | Both stop tests and TestWaitDeliveryContract; bed legs wait-stop-fake and wait-stop-claude, plus the delivery cases of wait-native-hint and wait-no-native. |

Clause 5 remains this parent's own DONE clause. After the third member lands, run wait-whole-goal-claude and wait-whole-goal-codex.
The first member changes the waiter and readers in `metasystem/internal/run/waiter.go`, `metasystem/internal/dispatch/watch.go`, `metasystem/internal/proofrun/attempt.go` and `metasystem/internal/goal/attention.go`. It wires verbs and records-based startup discovery in
`metasystem/cmd/metasystem/main.go`, `metasystem/cmd/metasystem/run.go`, `metasystem/cmd/metasystem/goal.go`, `metasystem/cmd/metasystem/channel_verbs.go`, `metasystem/internal/report` and `metasystem/internal/host`.
It also adds wait-delivery in `metasystem/scripts/agents/adapters/runtime-common.sh`, `metasystem/scripts/agents/adapters/claude.sh`,
`metasystem/scripts/agents/adapters/codex.sh`, `metasystem/scripts/agents/adapters/devin.sh` and `metasystem/scripts/agents/adapters/fake.sh`.
The Go adapter port is in `metasystem/internal/adapter`; Claude's startup discovery uses `metasystem/scripts/agents/supervision-hook.sh`.
Boot-clock samples belong in `metasystem/internal/identity/identity_linux.go` and `metasystem/internal/identity/identity_darwin.go`.
The second member changes publication owners in `metasystem/internal/goal/txn.go`, `metasystem/internal/proofrun/launcher.go` and `metasystem/internal/dispatch`. Wrappers use `metasystem/scripts/agents/dispatch.sh`, `metasystem/cmd/metasystem/run.go`
and `metasystem/cmd/metasystem/watch_verb.go`; landing hints use `metasystem/scripts/agents/land.sh`.
The third member changes the stop gate in `metasystem/internal/goal/turnverdict.go` and the contract in `metasystem/docs/design/turn-verdict-delivery-contract.md`, with the named conformance tests.
Each member includes its named test owners and bed wiring in `metasystem/testing.json` and `metasystem/scripts/agents/supervision-fixtures.sh`.
The first two include their matching doctrine changes in `metasystem/docs/orchestration.md`; the third owns the contract document named above.
The parent owns the event and usage measurements in section 5 and both real coordinator legs before claiming DONE.
Open the follow-on goal managed-seats-wait-through-a-holder when a runtime that cannot hold a foreground command joins the roster. That mechanism is outside this design.
Every seat installation must rebuild its enrolled engine and run metasystem up to re-arm the repository supervision set.
Reinstall changed adapter or hook assets through runtime setup; restart a provider only where its adapter requires configuration reload.
Drain old waiters or let their deadlines return, then re-register against version 2. Re-arming must not kill live jobs or proof attempts.
Mixed versions keep old watch behaviour and earn no pending-wait conformance claim. No goal-ledger migration is needed.
This page is the design deliverable only. Independent critique, implementation and the recorded full-width shared testing proof belong to the orchestrator.
