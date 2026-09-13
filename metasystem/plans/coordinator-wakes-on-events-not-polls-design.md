# Coordinator wakes on events, not polls

Design for goal 14 in `metasystem/plans/delivery-efficiency-plan.md`.
The full intent is `metasystem/plans/goals/coordinator-wakes-on-events-not-polls.md:8`.
Drafted by a Codex design round for the m1b seat on 2026-09-13 (chain implementer-fa5b5830882bcf6ea864cdf6), folded by the seat;
one rostered Codex design read follows, then the member goals of section 6 are opened and built on the roster lane.
Waiting stays in an engine process. Only an event, a deadline or a named failure needs the model.
Generated record locations below use their existing Go locator functions; they are local state, not checked-in files.

## 1. The wait sites today

These are eleven wait sites or entry-point groups. Process polls inside one command are distinct from model turns that repeat it.

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

Startup gates, custody retries and heartbeat loops also block inside adapters:
`metasystem/scripts/agents/adapters/runtime-common.sh:72`, line 119 and line 310.
They are bounded launch or supervision plumbing. They do not themselves require another model turn.
Bare watch is a one-shot snapshot (`metasystem/cmd/metasystem/watch_verb.go:21`).
Steward ticks and channel polls likewise stay in processes. Count a model wake only when inference starts.

## 2. The mechanism

Extend the existing waiter owner in `metasystem/internal/run/waiter.go`.
It owns registration, blocking, deadlines, hint delivery and recovery. Source owners decide their own predicates.
`metasystem/internal/dispatch/watch.go` supplies job observations; `metasystem/internal/proofrun/attempt.go` supplies attempt observations.
`metasystem/internal/goal/attention.go` supplies validated ledger observations. The command layer only wires these ports.
The waiter package imports none of those source owners. The stop gate consumes their typed observations through its scanner.

The public command is `metasystem wait`, with exactly one of `--job ID`, `--attempt ID`, `--goal ID`, `--run ID` or `--resume WAIT-ID`.
Common arguments are `--root DIR`, `--timeout DURATION` and `--json`. Default and maximum timeout are 24 hours.
This bounds an absent human without purchasing a model turn every half hour. Callers can choose a shorter bound.
Timeout must be positive and includes registration and cleanup. There is no model-facing poll interval.
Goal waits require `--event landing|human-act` and `--after COMMIT`, the accepted ledger tip read before launching or asking.
Human-act waits may add `--verb VERB`; landing waits may add `--chain ROOT-JOB-ID` to select one delegate chain.
Resume restores the saved selector and cursor. It accepts no replacement target or cursor.
Without a new timeout it keeps the original deadline. An explicit new timeout starts a recorded renewal, never a silent extension.

Every return prints one result with wait identifier, target incarnation, reason, source outcome and source evidence identity.
JSON adds registeredAt, deadline, observedAt, returnedAt, ledger tip or terminal stamp, and the replay command.
Exit 0 means successful terminal or matching ledger event; 1 means failed/red; 2 means target timeout or unknown terminal.
Exit 3 means target cancelled or launch-failed; 4 means missing, replaced or invalid source record.
Exit 6 means the waiting seat has actionable work or its ownership changed; the result names which fact changed.
Exit 64 means an existing live waiter or ineligible registration; 65 means storage or transport failure; 66 means uncertain identity.
Exit 67 means invalid arguments, 124 means this wait's deadline, and 130 means interruption of this wait.
A target timeout differs from a wait timeout. None of these exits cancels, concludes, approves or certifies the target.
The old watch commands keep their public status mappings through thin compatibility wrappers.

The registered wait is version 2 of the existing Waiter record, located by WaiterPath and WaitersDir
in `metasystem/internal/run/waiter.go:45`. Resolve the installation state root once, using the same rules as the stop gate.
The namespace remains kind, target identifier and owner digest. Add a random waitId and a fresh registration nonce.
Store the classified main lineage, session, checkout lease epoch, runtime delivery reference and waiter process birth identity.
Store the selector, goal binding, source incarnation, original ledger cursor, last checked tip and source result identity.
Store the registration time, UTC and boot-relative deadlines, boot identifier, remaining duration, last successful observation in both clocks and open-work signature.
State is pending, ready, deadline, interrupted or failed. A terminal row retains the result and selector for replay.
Only Go waiter operations write this record. Adapters and source writers send hints; they cannot mark it ready.
The waiter clears pending before returning, by a durable compare-and-write on waitId and registration nonce.
It closes and removes only its own hint endpoint. Terminal rows stay until an explicit replacement wait for that key.
Legacy rows still serve legacy watch liveness; they do not grant the new pending-wait exception.

Registration first validates the caller, target and cursor. It captures one immutable source incarnation.
Jobs pin job identifier, round and startedAt; runs pin identifier, generation and launch nonce.
Attempts pin attempt identifier and proof identity digest. Never follow an identifier onto a successor attempt or job round.
Create the hint receiver, publish pending under the existing bounded waiter lock, then reread the source before sleeping.
A terminal seen at entry returns immediately. An event between reading, registering and blocking is therefore not lost.
Each wake rereads the record through its owner and checks the incarnation before deciding.
For attempts, require a validated Terminal with EndedAt equal to Terminal.At. Nil Terminal, logs and worker death cannot mean success.

Goal observation freezes the configured endpoint with the cursor, fetches a private tip and applies the existing ledger acceptance gates.
Use CaptureTipBounded in `metasystem/internal/goal/attention.go:112`; never read the worktree's goal copy as fresh truth.
Keep the original cursor even as the last checked tip advances. Inspect intervening accepted states, including archived goals.
Human-act matches a new History operation targeting the goal, with the requested verb if supplied.
Use the goal owner's validated human authority classifications, including accepted channel or relay acts; a seat acting under power of attorney is still a seat.
Return the actual act, including refusal, unapproval or parking. Waking on an act grants no authority to continue.
Landing matches a commit newly reachable after the cursor with the exact Goal-Item and a valid Landing-Provenance trailer.
These trailers are written at `metasystem/scripts/agents/commit.sh:788`. A chain filter must match that provenance's chain.
A ledger-verb publication can also be selected with human-act and its verb; its History operation must join the Goal-Transaction trailer.
An unrelated ref advance, local commit, push acknowledgement or entering the landing phase does not match.
Check the landing destination against that endpoint at registration. A different code destination is a named invalid-target refusal,
not permission to watch HEAD or another branch. Local-mode fixtures publish to the dedicated local ledger ref.
If a rewind or missing history prevents proving the cursor relationship, return a named source failure. Never replace the cursor with now.

The engine rechecks pending sources every ten seconds, without printing progress or invoking a model.
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

Extend the executable adapter contract in `metasystem/scripts/agents/adapters/runtime-common.sh:3` with wait delivery.
Its wait-delivery operation takes waitId, nonce, deadline and the recorded session reference; it returns supported blocking or resume delivery.
The Go adapter port invokes it and records the chosen route before publishing pending. A failed route cannot grant exemption.
The adapter holds one command until its result or deadline. Provider handles and intermediate tool yields stay in adapter plumbing.
It must not ask the model to check TaskOutput, capture a terminal or poll a session handle while that command is pending.
Claude background notifications and Codex or Devin session events, when available, enter through their adapters as hints.
Native event projection alone is not native wake support: `metasystem/internal/adapter/events.go:3` is explicitly post hoc today.
Without native wake, hold a bounded foreground command. If the provider cannot hold it, the adapter retains the engine waiter
and delivers its completed result through the existing serialized session-resume path, or a fresh records-only continuation.
The Go waiter owns that delivery request after persisting the result. The adapter owns provider launch flags and delivery acknowledgement.
Native notification never replaces the blocking or resume route. Key resume delivery by waitId and result identity.
Persist pending delivery and the observed resumed-turn acknowledgement; command submission alone is not acknowledgement.
The existing launch serialization prevents concurrent duplicate turns. Recheck lease ownership and stop fences before any continuation.
For a replaced or human-stopped seat, retain the result for explicit recovery and launch no automatic turn.
The continuation carries waitId and the saved selector; it never runs a model merely to see whether the waiter has finished.
Delivery acknowledgement is separate from predicate completion. Retry delivery without redoing the target operation.
The steward's operator queue remains an optional notice channel, not the wait record or a proof of model delivery.

The scanner supplies PendingWait only for this classified session and lineage, with a current lease epoch,
a live birth-matched waiter, an unexpired deadline, a fresh successful source observation and a still-pending target.
It checks the current source at Stop too; a terminal source must not hide behind an old pending row.
The job, run or attempt must join this seat's ownership or claimed goal. A landing wait must join its recorded landing phase.
Unrelated read subscriptions remain possible but grant no Stop exemption.
A human-act wait cannot exempt a seat while another goal is claimable. Reject it at entry or return 6 when that changes.
Job, run, attempt and landing waits count as work in flight, like the existing live delegate exception.
The saved open-work signature limits suppression to the work present at registration. Changed open work returns 6.
In `metasystem/internal/goal/turnverdict.go`, use this fact in the unwatched-work join, ordinary work-in-flight branch and idle check.
An attempt wait covers its linked governed run only when the recorded run generation and nonce match.
Freeze refusal counters and signatures while the pending exception applies; do not spend block-once state on sleeping work.
Keep unrelated unwatched work, stop fences, human stop authority, warnings and degraded-input behaviour unchanged.
Return a visible WAITING line with target and deadline, and no request for another turn on that wait.
`metasystem/scripts/agents/supervision-hook.sh` transports this verdict unchanged for Claude, Codex and Devin.
The same fact must feed goal next and direct report turn-verdict on runtimes without hooks.
Extend `metasystem/docs/design/turn-verdict-delivery-contract.md`; future adapters owe this same conformance test.

After a missed hint, the timer or immediate reread finds the durable event. No event journal is required for correctness.
After restart, enumerate waiter records for the checkout and issue `metasystem wait --resume WAIT-ID` once.
Re-prove seat succession through the existing lease and goal ownership records. A new session cannot claim another live seat's wait.
Replace a dead waiter by compare-and-write with a new nonce. Uncertain liveness returns 66; a live duplicate returns 64.
Prefer the current owner's row; copy a proven predecessor's selector to the successor key under the waiter lock and invalidate the old pending row.
First replay a saved terminal result after checking its source evidence. Otherwise an expired plain resume returns 124;
only an explicit renewal arms another bounded period. With time left, reread from the original cursor before blocking.
Do not require old pipes, provider task handles, notifications or transcripts. If the registration itself was deleted,
a new goal wait needs the caller's original cursor. A missing cursor is an error, never an inferred new baseline.

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
Use a monotonic timer while the waiter lives. On the same boot, restart takes the shortest of saved remaining time,
UTC time left and boot-relative time left. A wall-clock step cannot lengthen that bound or the 30-second freshness window.
A changed boot or unreadable boot clock permits result replay but no pending exemption; return 124 before explicit renewal.
The platform identity readers supply the boot clock. Inject both clocks in tests; a future wall stamp is surfaced as clock drift.
Process identity uses boot identifier and start ticks where available, as in `metasystem/internal/run/waiter_pair_test.go:24`.
Two machines may disagree about time. Ledger commit ancestry decides event order; timestamps never decide whether a human act happened.
Native delivery may be slower than the engine. Measure event-to-model-resume separately from event-to-command-return.

## 5. Fixtures

The names below are implementation obligations, not claims that these tests already exist or passed.
Use injected clocks, source readers and hint delivery for time-based Go tests. Bed processes have named scaled ceilings and isolated state roots.
Go owns assertions; existing shell beds only launch scenarios and collect their results. The orchestrator runs real runtime legs outside a delegate sandbox.

| DONE clause | Go test and existing test owner | Setup and assertion; bed leg |
| --- | --- | --- |
| 1: one bounded wait | TestWaitJobTerminals in `metasystem/internal/dispatch/watch_test.go`; TestWaitRunTerminalsAndDeadline in `metasystem/internal/run/waiter_test.go` | Exercise every terminal, already-terminal entry, missing record and reused identifier. One command returns the typed result, without model polls. Fake bed leg wait-job-run exercises the public verb and legacy mappings. |
| 1: proof terminal | TestWaitAttemptRequiresCommittedTerminal in `metasystem/internal/proofrun/attempt_test.go`; TestWaitAttemptAfterDrain in `metasystem/internal/proofrun/launcher_test.go` | Exit a worker and delay terminal publication. The wait stays pending until valid Terminal and EndedAt are committed; failure and cancellation return distinctly. Bed leg wait-proof includes a retained attempt and its governed run. |
| 1: ledger events | TestWaitGoalLandingAndHumanAct in `metasystem/internal/goal/txn_test.go`; TestWaitGoalCursorHistory in `metasystem/internal/goal/attention_test.go` | A two-clone bed publishes a landing, approval, denial and unrelated commits. Match goal, verb and optional chain; reject local-only commits, wrong authority, lost history and rewinds. Bed leg wait-ledger covers local and remote endpoints. |
| 2: accelerator and fallback | TestWaitHintsOnlyTriggerReads in `metasystem/internal/run/waiter_test.go`; TestWaitDeliveryContract in `metasystem/internal/adapter/runtime_test.go` | Send early, duplicate, false and wrong-nonce hints; then drop all hints. Results agree and the timer returns within its bound. Fake bed legs wait-native-hint and wait-no-native assert no intermediate model turn, including an early provider tool yield. |
| 3: Stop honours pending | TestPendingWaitTurnVerdict in `metasystem/internal/goal/turnverdict_test.go`; TestPendingWaitIdleBacklog in `metasystem/internal/goal/turnverdict_idle_test.go` | Pending job, attempt, landing and eligible human waits suppress their refusals without advancing counters. A foreign session, dead waiter, expired row, changed work or unrelated unwatched run preserves the existing decision. Bed legs wait-stop-fake and wait-stop-claude exercise the hook and plain-command verdict; the Claude leg must observe an actual provider turn end. |
| 4: recovery replay | TestWaitRestartRecoveryReplay in `metasystem/internal/run/waiter_pair_test.go` | Crash before subscription, after registration, after source publication and after result persistence. Remove pipes and notifications, start a successor seat and re-register from records. Observe one matching result, the original cursor and deadline, no stolen live wait and no old cleanup deleting the new row. Bed leg wait-restart repeats this with real process death. |
| 1, 3, 4: failure bounds | TestWaitLockClockAndFetchBounds in `metasystem/internal/run/waiter_test.go`; TestWaitGoalFetchDeadline in `metasystem/internal/goal/attention_test.go` | Hold a lock, hang transport, regress the wall clock and reuse a process identifier. Advance fake time. Every case returns or loses exemption within its stated bound; no target is cancelled. Bed leg wait-bounds also checks local Git timeout and pipe unavailability. |
| 5: measurements | TestWaitMeasurementAccounting in `metasystem/internal/usage/usage_test.go`; TestWaitRuntimeBed in `metasystem/cmd/metasystem/runtime_verbs_test.go` | Replay known timestamps, overlapping waits, renewals, usage duplicates and missing telemetry. Assert the arithmetic below and refuse incomplete measurement. Real bed legs wait-whole-goal-claude and wait-whole-goal-codex each carry one whole coordinator goal. |

For each real runtime, retain one session's full goal from claim through delegated work, proof, landing and the human conclusion.
Include a human-act wait, a dropped hint, a records-only recovery and one explicit one-hour timeout. Collect at least one pending hour.
Use the goal's real waiting time; do not insert delays to enlarge the denominator. Other waits use the default deadline.
Observe a fixed 24-hour seat day starting at session launch. Keep raw provider usage and normalized inference-start and resume events.
Register wait lifecycle events through `metasystem/internal/events/emit.go` and `metasystem/scripts/agents/event-registry.json`.
Adapters supply inference starts and input usage through `metasystem/internal/usage`; missing native data is unavailable, never estimated.
Pending hours are the union of target-pending intervals, from registration to durable source event, including gaps between deadline renewals.
Count every inference request beginning inside those intervals, including poll, hook and unrelated wake-ups. Divide by pending hours; require less than 4.
Report terminal-triggered resumes separately; they start after their target stopped being pending and cannot conceal polls inside an interval.
Sum prompt input tokens for those pending requests, including cached input once, and divide by all coordinator input tokens in the same day.
Require less than 5 percent. Exclude delegate tokens from both sums and retain the provider's token-class mapping.
For each event, measure durable publication to the first resumed model turn consuming its wait result. Require every sample below 60 seconds.
Use a common observer clock in the two-clone bed; record clock uncertainty in live runs and fail the measurement if the upper bound exceeds 60 seconds.
Do not replace latency with notification time, command return time or an average. Record maxima and raw samples for both runtimes separately.
Absence of polling is proven by the full session event stream. Missing streams, failed delivery or insufficient pending exposure leave clause 5 unproven.

## 6. Landing

Land in three slices, each opened by the seat as a member goal that blocks this one
(the 2026-09-10 rule: an umbrella record for context, one-mechanism members with their own DONE):
wait-verb-returns-on-recorded-events (slice one: the record, the verb and the source readers),
publication-owners-hint-the-waiter (slice two: hints from the job, attempt and ledger owners, the compatibility wrappers)
and stop-gate-honours-a-registered-wait (slice three: the adapter route, the stop gate and resume delivery).
The measurement legs of section 5 are this goal's own operational clause, taken after the third member lands.
Each slice keeps old commands usable and includes its focused Go tests and shared-contract proof.
First land the waiter record and blocking core in `metasystem/internal/run/waiter.go`, source readers in
`metasystem/internal/dispatch/watch.go`, `metasystem/internal/proofrun/attempt.go` and `metasystem/internal/goal/attention.go`,
and routing in `metasystem/cmd/metasystem/main.go`, `metasystem/cmd/metasystem/run.go` and `metasystem/cmd/metasystem/goal.go`.
Add boot-clock samples to `metasystem/internal/identity/identity_linux.go` and `metasystem/internal/identity/identity_darwin.go`.
Second land durable-publication hints in `metasystem/internal/goal/txn.go` and `metasystem/internal/proofrun/launcher.go`,
the job transition owner in `metasystem/internal/dispatch`, and compatibility plumbing in `metasystem/scripts/agents/dispatch.sh`.
Land the adapters, stop gate and resume delivery together: `metasystem/scripts/agents/adapters/runtime-common.sh`,
`metasystem/scripts/agents/adapters/claude.sh`, `metasystem/scripts/agents/adapters/codex.sh`, `metasystem/scripts/agents/adapters/devin.sh`,
`metasystem/scripts/agents/adapters/fake.sh`, `metasystem/internal/adapter`, `metasystem/internal/host`, `metasystem/internal/goal/turnverdict.go`,
`metasystem/internal/report`, `metasystem/scripts/agents/supervision-hook.sh` and `metasystem/scripts/agents/land.sh`.
Third land the named bed scenarios, event and usage measurements, and contract updates in `metasystem/testing.json`,
`metasystem/scripts/agents/supervision-fixtures.sh`, `metasystem/docs/orchestration.md` and `metasystem/docs/design/turn-verdict-delivery-contract.md`.
The test owners and event files named in section 5 are part of their matching slices. Run the two real coordinator legs before claiming DONE.
Every seat installation must rebuild its enrolled engine and run metasystem up to re-arm the repository supervision set.
Reinstall changed adapter or hook assets through runtime setup; restart a provider only where its adapter requires configuration reload.
Drain old waiters or let their deadlines return, then re-register against version 2. Re-arming must not kill live jobs or proof attempts.
Mixed versions keep old watch behaviour and earn no pending-wait conformance claim. No goal-ledger migration is needed.
This page is the design deliverable only. Independent critique, implementation and the recorded full-width shared testing proof belong to the orchestrator.
