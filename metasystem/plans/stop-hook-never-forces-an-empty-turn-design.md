# Stop hooks must not force empty turns

- Kind: design
- Id: 01M3A2YHDNKDH07EKHPMG4MEWZ
- Status: accepted
- Goals: stop-hook-never-forces-an-empty-turn

Design for goal 4 of `metasystem/plans/delivery-efficiency-plan.md`, dated 2026-09-12.
The accepted scope and absorbed clauses are in `metasystem/plans/goals/stop-hook-never-forces-an-empty-turn.md:8` and line 10.
Drafted in three Codex rounds (chain stop-hook-design-190340, gpt-6-astra), read twice by Codex gpt-5.6-sol, settled by the m1b seat under Wido's lanes of 2026-09-12 (the seat designs, Codex reads and builds, Opus reads the build).
The second read's six findings are folded below: the idle refusal survives a lost counter, the incident record has an independent second channel, the idle digest is not amended, the launcher block moves into the first member, the members are reordered so each lands on its own, and the seven-day observation has an owner.

## 1. What refuses today and why

The audit records 331 refusals and 292 forced coordinator turns over five days.
Its 184 narrator reads, 117 deadlines, and about 95 arming failures are separate recorded counts; they are not a partition of the 331 refusals.
The census below counts trigger sites and catchall routes, not distinct messages or calls to a renderer.
There are 45 infrastructure rows, five seat-actionable branches, and two idle branches.
The infrastructure count includes 27 hook or launcher routes and 18 engine producers reached through the unavailable-verdict route. These are overlapping routes, not 45 distinct incidents.

| Class | Trigger and evidence |
| --- | --- |
| Steward-owned infrastructure | Invalid runtime argument: `metasystem/scripts/agents/supervision-hook.sh:15`. |
| Steward-owned infrastructure | Invalid event argument: `metasystem/scripts/agents/supervision-hook.sh:16`. |
| Steward-owned infrastructure | Claude's launcher converts any nonzero hook exit, including either argument error, into an unpersisted raw block: `metasystem/scripts/enforcement/claude-code-hooks.json:25`. |
| Steward-owned infrastructure | Invalid or empty worker output, or nonzero worker exit: `metasystem/scripts/agents/supervision-hook.sh:218`. |
| Steward-owned infrastructure | Stop deadline expired: `metasystem/scripts/agents/supervision-hook.sh:329`. |
| Steward-owned infrastructure | Engine unavailable: `metasystem/scripts/agents/supervision-hook.sh:399`. Repair of the enrolled installation belongs to the steward; its present rebuild advice does not prove seat authority. |
| Steward-owned infrastructure | Runtime name unreadable: `metasystem/scripts/agents/supervision-hook.sh:514`. |
| Steward-owned infrastructure | Runtime process identity unreadable: `metasystem/scripts/agents/supervision-hook.sh:523`. |
| Steward-owned infrastructure | Fallback identity classification failed: `metasystem/scripts/agents/supervision-hook.sh:537`. |
| Steward-owned infrastructure | Fallback runtime name unreadable: `metasystem/scripts/agents/supervision-hook.sh:541`. |
| Steward-owned infrastructure | Fallback process identity unreadable: `metasystem/scripts/agents/supervision-hook.sh:549`. |
| Steward-owned infrastructure | Turn evidence preparation failed: `metasystem/scripts/agents/supervision-hook.sh:745`. |
| Steward-owned infrastructure | Attempt evidence write failed: `metasystem/scripts/agents/supervision-hook.sh:751`. |
| Steward-owned infrastructure | Attempt evidence unreadable: `metasystem/scripts/agents/supervision-hook.sh:757`. |
| Steward-owned infrastructure | Checkout holder classification failed: `metasystem/scripts/agents/supervision-hook.sh:766`. |
| Steward-owned infrastructure | Checkout holder classification unreadable: `metasystem/scripts/agents/supervision-hook.sh:772`. |
| Steward-owned infrastructure | Arming failed: `metasystem/scripts/agents/supervision-hook.sh:836`; line 835 retains only the aggregate, losing component detail. |
| Steward-owned infrastructure | Health returned no verdict: `metasystem/scripts/agents/supervision-hook.sh:842`. |
| Steward-owned infrastructure | Narrator response fields unreadable: `metasystem/scripts/agents/supervision-hook.sh:855`. |
| Steward-owned infrastructure | Narrator read failed: `metasystem/scripts/agents/supervision-hook.sh:859`. |
| Steward-owned infrastructure | Holder protocol read failed: `metasystem/scripts/agents/supervision-hook.sh:1037`. |
| Steward-owned infrastructure | Holder protocol response unreadable: `metasystem/scripts/agents/supervision-hook.sh:1044`. |
| Steward-owned infrastructure | Holder lease renewal failed: `metasystem/scripts/agents/supervision-hook.sh:1072`. |
| Steward-owned infrastructure | Watchdog read failed: `metasystem/scripts/agents/supervision-hook.sh:1082`. |
| Steward-owned infrastructure | Watchdog evidence preparation failed: `metasystem/scripts/agents/supervision-hook.sh:1087`. |
| Steward-owned infrastructure | Hook evidence maintenance failed: `metasystem/scripts/agents/supervision-hook.sh:1098`. |
| Steward-owned infrastructure | Turn verdict unavailable: `metasystem/scripts/agents/supervision-hook.sh:1197`. This includes malformed fields at line 1126, engine uncertainty at line 1133, and command failure at line 1138. |
| Steward-owned infrastructure | State-root resolution: `metasystem/internal/goal/turnverdict.go:229`. |
| Steward-owned infrastructure | Checkout fence read: `metasystem/internal/goal/turnverdict.go:241`. |
| Steward-owned infrastructure | Closed-fence description: `metasystem/internal/goal/turnverdict.go:244`. |
| Steward-owned infrastructure | Closed-fence command: `metasystem/internal/goal/turnverdict.go:245`. |
| Steward-owned infrastructure | Session-stop marker read: `metasystem/internal/goal/sessionstop.go:435`. |
| Steward-owned infrastructure | Session-stop marker decode: `metasystem/internal/goal/sessionstop.go:438`. |
| Steward-owned infrastructure | Session-stop inspection registry read: `metasystem/internal/goal/sessionstop.go:442`. |
| Steward-owned infrastructure | Session-stop holder lease proof: `metasystem/internal/goal/sessionstop.go:452`. |
| Steward-owned infrastructure | Verdict-state read: `metasystem/internal/goal/turnverdict.go:270`. |
| Steward-owned infrastructure | Brain status write: `metasystem/internal/goal/turnverdict.go:291`. |
| Steward-owned infrastructure | Verdict-state write: `metasystem/internal/goal/turnverdict.go:296`. |
| Steward-owned infrastructure | Authorization revalidation during consume: `metasystem/internal/goal/sessionstop.go:485`. |
| Steward-owned infrastructure | Registry reread during consume: `metasystem/internal/goal/sessionstop.go:489`. |
| Steward-owned infrastructure | Consumed-authorization registry write: `metasystem/internal/goal/sessionstop.go:500`. |
| Steward-owned infrastructure | Authorization changed or was spent before consume completed: `metasystem/internal/goal/turnverdict.go:304`; the second spent check is `metasystem/internal/goal/sessionstop.go:493`. |
| Steward-owned infrastructure | Goal-lock directory creation: `metasystem/internal/goal/goalverbs.go:106`. |
| Steward-owned infrastructure | Goal-lock open: `metasystem/internal/goal/goalverbs.go:109`. |
| Steward-owned infrastructure | Goal-lock acquisition deadline: `metasystem/internal/goal/goalverbs.go:119`. |
| Seat-actionable | Owned work has no live waiter: `metasystem/internal/goal/turnverdict.go:675`. |
| Seat-actionable | An unblocked plan line has not been surfaced: `metasystem/internal/goal/turnverdict.go:872`. |
| Seat-actionable | The current goal revision names the next step: `metasystem/internal/goal/turnverdict.go:907`. |
| Seat-actionable | The queue changed while a goal remained held: `metasystem/internal/goal/turnverdict.go:916`. |
| Seat-actionable | The goal-free declaration predates new work: `metasystem/internal/goal/turnverdict.go:948`. |
| Idle-with-backlog | Fresh ledger unavailable, counted under its existing sentinel: `metasystem/internal/goal/turnverdict.go:428`. |
| Idle-with-backlog | Approved, claimable backlog without a live delegate: `metasystem/internal/goal/turnverdict.go:465`. |

The invalid-worker route includes incomplete delegate hints at `metasystem/scripts/agents/supervision-hook.sh:384`, failed ancestry authentication at line 394, registry read failure at line 407, and unregistered runtime at line 411.
It also includes unreadable payload at `metasystem/scripts/agents/supervision-hook.sh:421`, environment lookup failure at line 440, repository resolution failure at line 443, installation mismatch at line 454, and unreadable custody at line 494.
Other unexpected shell exits enter this same route or Claude's outer launcher block. Argument validation is therefore part of the Stop census.
The engine producers above reach `failClosedTurnVerdict` directly or through the error return at `metasystem/internal/goal/turnverdict.go:337`; its generic block is at line 412.
An invalid or expired authorization without a read error grants no exemption; it is not a separate refusal. Successful-consume cleanup failure at `metasystem/internal/goal/sessionstop.go:503` and full-display write failure at `metasystem/internal/goal/turnverdict.go:324` are already notices.

Twenty infrastructure triggers use the first-error slot at `metasystem/scripts/agents/supervision-hook.sh:481` and reach the external refusal at line 943.
The deadline uses the same recorder at `metasystem/scripts/agents/supervision-hook.sh:334`.
`metasystem/internal/report/stopblock.go:121` hashes cause text alone; line 137 writes before returning the first block at line 141.
It remembers a cause for the session, without the condition, deadline, turn generation, or arming result.
The raw refusals and unavailable-verdict route bypass that record.
The verdict renderer at `metasystem/scripts/agents/supervision-hook.sh:1178` serves both seat and idle branches.
Its fallbacks at `metasystem/scripts/agents/supervision-hook.sh:954`, line 981, and line 1016 preserve an independent verdict block; they add no new cause.
Those fallbacks lose the bounded-idle renderer choice. The generic renderer appends unrelated plan advice at `metasystem/internal/report/stopblock.go:26`.
Hook completion follows emission at `metasystem/scripts/agents/supervision-hook.sh:921`; it cannot prove that every refusal and its arming result were saved first.

## 2. The mechanism

Put the classification and stop decision in `metasystem/internal/goal/turnverdict.go`.
A typed condition carries its code, class, subject, observed state, detail, remedy owner, and clearing command.
Use explicit codes from the producing owner. Never classify rendered strings, exit status alone, or the runtime name.
Collect all failures; replace the first-error slot. An unknown collection failure belongs to infrastructure.
Migrate each of the 18 engine producers in the census at its source, with a distinct code for the operation that failed.
State-root, fence read, fence description, fence command, verdict-state read, status write and verdict-state write return typed infrastructure conditions.
Session-stop inspection and consume retain their separate marker, registry, lease, revalidation and write codes. Losing authorization during consume grants no exemption.
The lock owner distinguishes directory creation, open and acquisition failures. The outer error route preserves that code instead of calling `failClosedTurnVerdict`.
Remove the generic blocking uncertainty result at `metasystem/internal/goal/turnverdict.go:412`; unexpected errors become infrastructure notices. Keep the fresh-ledger sentinel as the separate idle branch.
The command composition in `metasystem/cmd/metasystem/goal.go` gathers these inputs and calls the decision owner.
`metasystem/internal/report/stopblock.go` renders the result and maintains infrastructure reporting history. It does not invent a blocking reason.

A seat-actionable refusal needs a positively observed condition and one complete command the seat may run.
The command must change that condition or start its required work. A status read, a retry, and advice to ask a human do not qualify.
For unwatched runs, use the command renderer at `metasystem/cmd/metasystem/run.go:80`. Add a job-watch renderer in that file for the existing verb parsed at line 433; line 240 is also a parser, not a renderer.
Both renderers include the exact root and identifier, with shell-safe argument quoting. The job form is `bin/metasystem job watch --root <root> --job <job>`.
For stale goal-free state, supply the current digest and the complete declaration command, after checking its existing admission rules in `metasystem/internal/goal/verbs.go:2281`.
For plan, current-goal, and changed-queue prods, use a complete clearing command already named by the selected work; do not turn arbitrary next-step prose into shell.
When such a command is absent or requires a human, show the work as a notice. Do not issue a text-only prod.
This eligibility check applies to the five seat-actionable branches. Their existing seen-state limits still apply when they qualify.
The idle path remains mandatory and outside that eligibility check. Its refusal names approved backlog with no job, the selected ready goal, its next-step text and one claim command.
Render `bin/metasystem goal claim --root . --id <ready goal> --lineage <seat lineage>` with the real goal and authenticated lineage, quoted as arguments, for execution from the checkout's metasystem directory.
Add this renderer beside the idle decision at `metasystem/internal/goal/turnverdict.go:490`; the existing claim verb is `metasystem/cmd/metasystem/goalsync_mutations.go:1934`.
For this path, DONE clause 1 means the first lawful act, not a whole workflow or an immediate exemption. Authoring the brief and dispatching remain the seat's work under that quoted next step.
Claiming alone does not manufacture a job or clear the idle rule. The counter and three-refusals handoff remain mandatory; the steward launcher is never printed as the seat's command.

Infrastructure produces an allowed stop with degraded health, including on its first occurrence, and records a steward incident through the delivery path below.
There is no initial infrastructure refusal to spend. Thus the once-per-deadline refusal ceiling is zero for this class.
Do not convert that allowance into ALL CLEAR, a valid human authorization, a lease, a claim, or permission to dispatch.
A separately established seat or idle refusal still wins. Its reason names only that condition; infrastructure stays in the notice channel.
Keep invalid human-stop markers ineligible and preserve consumption checks in `metasystem/internal/goal/sessionstop.go:428` and line 484.
If inspection or consume fails, evaluate ordinary work without that exemption when its inputs remain available; do not bypass idle evaluation by returning early. Never claim an uncommitted consumption.
Fresh-ledger uncertainty stays in the existing bounded idle branch; it is not the narrator or hook-evidence failure class.

Extend the existing per-session verdict state in `metasystem/internal/goal/turnverdict.go:150` with the actual stop deadline coordinates and prepared decision evidence.
Persist the decision, classification, clearing command, arming result, and generation in the same commit that spends its seen marker or idle count.
The hook generation is also carried into the checkout's existing component evidence through `metasystem/internal/steward/component_evidence.go:188`.
Component evidence remains observation evidence; failure to read or mirror it cannot reverse a durable work decision.
Every emitted refusal names a committed decision: in the verdict state, or, when that write failed, in the hook log's appended line (the second channel below), which is written before emission. No raw block or post-emission-only evidence remains.
If the authoritative decision cannot be persisted, the observed work decision still stands: an observed idle-with-backlog condition still refuses the stop, uncounted, with its reason and command, and the refusal says the count could not be spent. The refusal is written to the hook log as a decision line first (see the log protocol below); if even that append fails, the refusal is still emitted and says it is unrecorded. Infrastructure alone never manufactures a refusal from a lost record.
Never claim a counted idle refusal or a successful authorization when the record did not commit. An auxiliary incident-record failure cannot erase an already committed idle decision. The two records have different duties.

Extend the external record owned by `metasystem/internal/report/stopblock.go:155` to version 2 for infrastructure incidents and delivery state.
Resolve it from the same canonical checkout state root as the verdict; remove the hook's separate repository-root path calculation.
At the outer gate's entry, allocate the hook turn generation and record that invocation's start and actual deadline end, before optional collection.
The current deadline starts at `metasystem/scripts/agents/supervision-hook.sh:38` and is passed to the worker at line 61; carry its end explicitly through the new Go boundary.
Key the episode by canonical checkout, holder session and lease epoch, hook turn generation, deadline end and condition fingerprint. A first-observation time is evidence only; it never starts a rolling window.
The fingerprint contains cause code, component or resource, stable error kind and relevant engine or fence identity. Exclude elapsed time, incidental wording and observation timestamps from this inner fingerprint only.
Record attempt sequence, first and last observation, count, full arming result and delivery state. All retries within that actual deadline carry the same generation and end, including timeout completion.
Concurrent updates use the bounded record lock. Repeated observations reuse one incident identity and increase its count; they neither refuse nor enqueue a new identity.
A different generation or deadline end creates a new episode even within 60 seconds of the previous report. Recovery marks the old condition resolved without merging future deadlines.
If deadline coordinates cannot be established, report infrastructure uncertainty without borrowing a previous episode or suppressing evidence as already seen.
Read version-1 entries as history, never as evidence that a new decision or deadline was already processed.
Retain undelivered incidents until the steward acknowledges them; prune delivered history using the existing session retention horizon.

Preserve `up.Result` and use its renderer in `metasystem/internal/up/up.go:80` for both the command and the stop report.
Carry every failed component's outcome, detail and remedy unchanged, followed by the exact aggregate. ENROLLMENT_DRIFT means the engine differs from its enrollment; retain its agent-free-terminal restart remedy.
An interrupted arming call records its partial component results and an explicit incomplete aggregate from the arming owner. It never fabricates an armed result.
The persisted record holds the full text. Bound unrelated display text first; keep the failed component's outcome, detail and remedy exact and together. A decision reference supplements those fields rather than replacing them.

Move Stop collection and deadline composition out of the shell into the shared command path in `metasystem/cmd/metasystem/goal.go`.
Read the work decision before optional health, narrator, or arming waits. Prepare its durable response with arming marked incomplete, then attach the completed arming result before final emission.
Use the existing 60-second outer deadline and reserve its final three seconds for completion. All dependency waits and lock waits share that deadline.
A deadline relays an already committed work refusal if one exists; otherwise it allows with the infrastructure incident. It never manufactures uncertainty as a new block.
Compare generation, deadline end and attempt sequence when completing the prepared decision. Worker and timeout completion are mutually exclusive; a late worker cannot overwrite the result.
Replaying a completed decision within that deadline never spends another seen marker or idle refusal. A genuine later stop remains a new idle-counter opportunity under the existing rules.
The shell keeps provider payload transport and owned-child cleanup. Without a readable committed work decision, missing engine, malformed output and deadline fallback emit the fixed degraded allowance.
Replace Claude's raw launcher block at `metasystem/scripts/enforcement/claude-code-hooks.json:25` with a nonblocking degraded notice on bootstrap failure, including invalid arguments.
Buffer and validate the gate response before relaying it; never append a second JSON response to partial output. Relay an available committed work refusal; a transport exit code alone cannot create one.
If no engine can record the failure, the fallback states delivery unconfirmed. The shell performs no policy or JSON-state repair.

Add `host stop-gate` in `metasystem/cmd/metasystem/host_verbs.go`, registered in `metasystem/cmd/metasystem/main.go`. It calls the same Go collection and decision service as `report turn-verdict`.
Its request carries checkout, authenticated session and holder, provider turn identity, repeat evidence, generation, attempt and deadline end. Its response carries a decision identity when committed, block or allow, display, health and delivery state.
Native Stop hooks call this boundary and translate its response.
A boundary never counts the same native Stop twice: the decision record is keyed to the turn generation and deadline end it was made for.
`metasystem/scripts/agents/adapters/runtime-common.sh:3` owns delegate plumbing only; the fake adapter does not source it (`metasystem/scripts/agents/adapters/fake.sh:134`). Neither fact supplies a coordinator gate.
A Codex delegate round has no native Stop event, so its adapter (`metasystem/scripts/agents/adapters/codex.sh`) calls the gate at the turn boundary, after the provider returned and before the job is made terminal (`metasystem/scripts/agents/adapters/runtime-common.sh` terminalizes only from running). The gate call names the round's own job, and the decision excludes that job from the live-work exemption: a round asking whether it may stop is not its own reason to stop. On an allowance the adapter terminalizes as today. On a committed refusal the adapter does not terminalize; it re-invokes the provider once, in the same job and round, with the refusal's reason and command as the message, under the round's remaining cap (the dispatcher owns that transition and records it on the round as a gate continuation; no new job, no new budget, no hidden resume). A second committed refusal in the same round terminalizes the job with the refusal recorded as the round's gap, and the seat sees it as a normal return. This is the dispatcher lifecycle change member stop-hosts-enforce-the-common-gate builds and its Codex fixture reproduces.
Hosts without native Stop support are a lifecycle of their own (a managed seat: launch, permission envelope, ordinary reply, gate at every turn boundary, activation owner) and this goal does not build it; member stop-hosts-enforce-the-common-gate proves the common gate at the two boundaries production runtimes have today, the Claude Code native Stop hook of a seat and the adapter turn boundary of a Codex delegate round (`metasystem/scripts/agents/adapters/codex.sh`), and the managed-seat lifecycle (`metasystem/scripts/agents/hosts/host-common.sh`, `host finish` in `metasystem/cmd/metasystem/host_verbs.go`) is named as the follow-on goal in section 7. An unmanaged provider is not certified by an instruction to read `goal next`.
Replace the informational fallback at `metasystem/docs/design/turn-verdict-delivery-contract.md:26` with this enforcing contract for every registered host, including future runtimes. Runtime adapters own provider flags and delivery only, as `metasystem/docs/orchestration.md` requires.

Add a `DrainStopIncidents` reader in `metasystem/internal/steward/tick.go`, before narrator work. The current loop at line 237 reads only ledger-attention events; it does not consume stop incidents.
The reader loads undelivered version-2 incidents through the record owner and calls `QueueNotification` at `metasystem/internal/steward/intervene.go:317` with the incident identifier as its nonce.
Report seat, checkout, generation, deadline end, first and last observation, count, cause, component outcome, detail, remedy and aggregate. The stop path never waits for channel delivery or wakes the failed seat to retry it.
`DeliverPending` in `metasystem/internal/steward/notify.go:112` reads the durable queue; the runner invokes it at `metasystem/internal/steward/runner.go:203` even after a failed tick.
Persist the incident's delivery acknowledgement before deleting its queued notification. Retry the same identity after failure; a crash around channel delivery may redeliver it, but cannot create another stop refusal.
The second, independent channel is the hook log the hook already writes at `metasystem/scripts/agents/supervision-hook.sh:878`, with a line protocol this goal defines: every line carries the canonical checkout, the holder session and lease epoch, the hook turn generation, the deadline end, the line kind (decision or incident), and for a decision its class, reason, clearing command, count state and the arming aggregate; for an incident its cause code, component, count and the arming aggregate. The gate appends from the shell with the append's exit checked, before emission; a failed append is reported in the emitted text. The steward's drain reads the log's incident lines when the version-2 record is unreadable, and records each delivery as an acknowledgement line in the same log (kind acknowledged, the incident identity), so a delivered line is never queued again on a later tick even though the log line itself is permanent. The per-session verdict file under artifacts/agents/supervision/stop-verdicts is replaced at every verdict (`metasystem/internal/goal/turnverdict.go:322`) and is display, never record.
Only when both channels fail does the gate say "delivery unconfirmed" in the provider notice and standard error; stopping stays allowed for infrastructure alone. That residual case is the honest limit of DONE clause 2, and the seven-day observation counts it.

Use one fresh projected board for current goal, READY, INFLIGHT and idle selection in `metasystem/internal/goal/project.go`.
READY means eligible for the authenticated machine and lineage pair, using the existing claim admission rules. It is not machine-only ownership.
Retain accepted tip, successful fetch completion, projection time and owner pair as its freshness proof; recheck the local tip and lease epoch before committing the decision.
The existing fresh projection is at `metasystem/internal/goal/project.go:312`; retire the separate offline Stop reads at `metasystem/internal/goal/turnverdict.go:994` and line 1055.
For the activity join, include ready goals and the same actor's working claims as goal/revision pairs; exclude fenced claims.
INFLIGHT requires a nonterminal job or governed attempt matching that pair and owner, with live recorded process birth identity. A live seat claim alone never qualifies.
Read the job coordinates in `metasystem/internal/dispatch/jobrecord.go:64` and line 98, and attempt coordinates in `metasystem/internal/run/run.go:126` and line 189.
Resolve a job's lineage through its recorded main identity and announcement; require its claim epoch to match the selected claim. Missing owner or revision evidence grants no exemption.
Unrelated, old-revision, other-lineage, terminal, and unproven-live activity stays visible as activity but cannot exempt this frontier. Keep generic Busy display separate from this exemption.
The idle digest is not amended by this goal: `metasystem/internal/goal/turnverdict.go:509` keeps hashing every nonterminal job, so another seat's job churn resets the counter as it does today. DONE clause 4 says the idle path is untouched, and the absorbed INFLIGHT clause governs the WORK IN FLIGHT exemption, not the digest. TestIdleDigestKeepsEveryNonterminalJob pins that nothing moved; narrowing the digest is a proposal for memory/backlog-notes.md with the churn count as its evidence.
Observe goal fences before the Busy and WORK IN FLIGHT exits. Persist a seen fingerprint per session from goal, stop identifier, fence revision, epoch, fence generation and reason; hook generation is not a fence change.
Emit FENCED once for each unchanged fence, including beside live work. Clearing and recreating a fence is a new observation. Reporting never resumes it or creates current work from it.

## 3. What does not change

`metasystem/records/goals/idle-with-backlog-alarm.md:7` remains the idle refusal and handoff contract, digest included.
Approved backlog with no joined job still blocks the first and second stops. The third unchanged refusal prepares steward continuation and records the incident.
The steward claims if needed on its tick and rechecks dispatch guards. Its human alarm is used when continuation cannot be prepared or carried out.
The unreadable-ledger sentinel and preservation of an independent block remain intact; which work changes reset the counter is not changed by this goal.
Infrastructure observations never increment, reset, or consume the idle counter. The generation mirror never serves as its repeat counter.
Human session-stop authorization remains holder-bound, expiring and single-use. No error path creates, renews, or consumes permission by inference.
StopFence authority, approval, budgets, lease custody, foreign-runtime skips, proven-delegate skips, and steward continuation admission remain unchanged.

## 4. Risks

The main risk is treating missing telemetry as proof that backlog is empty. Keep the fresh idle read and its bounded failure path separate from auxiliary reads.
The opposite risk is letting a telemetry failure hide a real refusal. Persist the work decision first and preserve it through arming, rendering and incident-delivery failures.
Text-only goal prods become notices unless they already name a lawful clearing command. This is a deliberate consequence of the actionable-refusal requirement; the mandatory idle rule still applies.
A global job count or a machine-only claim read can silently grant the wrong seat an exemption. Test both owner coordinates and revision joins.
Coalescing by elapsed time would suppress a new deadline's report. Freeze the actual generation and deadline end at entry; retries reuse them, while later stops have distinct coordinates.
The explicit delivery limit leaves some infrastructure failures without a durable steward report. Such stops must say delivery unconfirmed; they must never erase authorization records or claim healthy supervision.

## 5. Fixtures

These are implementation obligations, not tests claimed as run by this design round. New test logic is Go; shell beds only launch it.

| Test or bed leg | Setup and required observation |
| --- | --- |
| TestStopConditionClassification in `metasystem/internal/goal/turnverdict_test.go` | Cover all 52 census rows, including the 18 engine producers. All 45 infrastructure routes preserve their distinct codes and never independently block; the five action branches require lawful commands; both idle branches retain their decisions. |
| TestStopClearingCommand in `metasystem/internal/goal/turnverdict_test.go` | For each actionable branch, run its emitted command in an isolated fixture and observe the condition clear or its work start. Missing commands, human-only remedies and text-only next steps become notices. Keep idle decisions mandatory. |
| TestIdleClaimCommandIsFirstAct in `metasystem/cmd/metasystem/goal_test.go` | An approved ready goal has no brief or job. Assert the real goal, lineage, quoted next step and executable claim command. Run it as the seat and observe its lawful claim, no fabricated dispatch, no human-stop command substitution and no exemption merely for claiming. |
| TestInfrastructureDeadlineIdentity in `metasystem/internal/report/stopblock_test.go` | Use a fake clock for identical retries, concurrent writers, changed condition, recovery and recurrence. Assert one identity per condition and actual deadline. Different generations or deadline ends report separately even within 60 seconds; retries cannot extend the deadline. Cover missing coordinates and version-1 history import. |
| TestStopDecisionPersistsBeforeEmission in `metasystem/cmd/metasystem/goal_test.go` | Capture output while faulting arming, incident writes, component reads and payload completion. Every block has prior committed decision evidence, full available up result and checkout generation. A lost auxiliary record preserves a real idle block; a lost authoritative commit cannot claim one. |
| TestStopDeadlineCompletionIsSingleUse in `metasystem/internal/goal/turnverdict_test.go` | Race worker completion with timeout and replay their shared generation, deadline end and attempt. One committed decision wins; late completion cannot overwrite it or spend another idle count. A new stop remains countable. |
| TestSessionStopInfrastructurePreservesAuthority in `metasystem/internal/goal/turnverdict_idle_test.go` | Fault every inspection and consume producer, then replay marker bytes. No error creates or consumes permission by inference. Available ordinary work is still evaluated; a committed consumed marker cannot be reused after cleanup failure. |
| TestArmingDetailSurvivesStop in `metasystem/cmd/metasystem/up_test.go` | Replay ENROLLMENT_DRIFT and a component failure. Compare persisted and rendered component outcome, detail, remedy and aggregate byte-for-byte with up's own output. FakeReplayArming separately proves the stop decision after classification lands. |
| TestStopIncidentDrainBeforeNarrator in `metasystem/internal/steward/tick_test.go` | Persist an incident, fail narrator work, then tick and deliver. Assert the named reader still queues the incident, delivery is acknowledged, and later ticks do not create a new identity. |
| TestStopIncidentDeliveryUnconfirmed in `metasystem/internal/steward/notify_test.go` | Fault incident reads, writes, queue writes and channel delivery separately. Storage loss or absent bootstrap engine yields an explicit unconfirmed report, never an infrastructure block. Queued delivery failure retains its identity for retry; no hook-log delivery is claimed. |
| TestStopFrontierOwnerRevisionAndFreshness in `metasystem/internal/goal/turnverdict_world_test.go` | Two machines, two lineages on one machine, current and old goal revisions, jobs and governed attempts. Only matching live work exempts the actor's frontier. Failed fetch uses the existing sentinel; a changed tip or lease rejects the stale projection. |
| TestIdleDigestKeepsEveryNonterminalJob in `metasystem/internal/goal/turnverdict_idle_test.go` | Another seat's job appears and ends between two idle stops; the digest changes and the counter resets, exactly as today. Pins that this goal does not amend the idle digest. |
| TestIdleRefusalSurvivesALostCounter in `metasystem/internal/goal/turnverdict_idle_test.go` | Fault the verdict-state write while approved backlog has no job. The stop is refused with the idle reason and the claim command, the refusal says the count could not be spent, and no count is recorded. |
| TestStopIncidentDrainFromHookLog in `metasystem/internal/steward/tick_test.go` | Make the version-2 record unreadable after the hook log carried two incident lines. The drain reads the log's unacknowledged lines, queues one notification per incident identity and acknowledges them; with both channels unreadable the provider notice says delivery unconfirmed. A delivered incident is acknowledged by a line in the log and is not queued again on the next tick. |
| TestDailyRefusalCountReadsTheLogStream in `metasystem/internal/report/stopblock_test.go` | A synthetic week of decision lines on two seats counts refusals per seat and day; replacing the per-session verdict file changes no count. |
| TestFencedClaimSurfacedOnceBesideFlight in `metasystem/internal/goal/turnverdict_stopfence_test.go` | Hold a fenced goal beside a working goal, with and without joined live work. Observe FENCED once despite new hook generations, a new line after fence change, and no continuation of the fenced goal. |
| FakeReplayNarrator, FakeReplayDeadline, FakeReplayArming in `metasystem/cmd/metasystem/runtime_conformance_test.go` | Add an explicit stop-replay action to `metasystem/scripts/agents/adapters/fake.sh` that drives the common gate for a bed-owned seat, not the fake delegate's custody exemption. Replay the three recorded causes, including the outer deadline. Same-deadline repeats cause no forced turn or new notification identity; records retain the cause and arming evidence. |
| FakeReplayIdleWithInfrastructure in `metasystem/cmd/metasystem/runtime_conformance_test.go` | An approved goal, no job, and each infrastructure failure in turn. First and second stops still block for backlog. Third stages continuation, records the incident and allows absent another block. A steward tick starts the guarded continuation; a failed dispatch raises the existing alarm. |
| FakeReplayUnreadableHookState in `metasystem/cmd/metasystem/runtime_conformance_test.go` | Break hook evidence and incident state independently, then together. Assert degraded allowance without work, retained independent idle refusal when its decision is durable, and truthful unconfirmed-delivery text. |
| TestLauncherFailureNeverRawBlocks in `metasystem/cmd/metasystem/runtime_conformance_test.go` | Invoke the installed Claude launcher with invalid runtime and event arguments, missing engine, partial output and nonzero exit. Assert one valid response, no unpersisted raw block, preserved readable committed work decisions, and truthful degraded or unconfirmed notices. |
| TestEveryRuntimeUsesStopDecision in `metasystem/cmd/metasystem/runtime_conformance_test.go` | Enumerate registered runtimes. The native Stop hook boundary and the delegate adapter boundary both call the one gate and honor a committed refusal; infrastructure never requests another turn; a runtime with neither boundary is reported unproven, not certified. |
| RealClaudeNativeStopReplay and RealCodexAdapterStopReplay in `metasystem/cmd/metasystem/runtime_conformance_test.go` | The orchestrator launches actual Claude Code with its installed Stop hook, and an actual Codex delegate round through its adapter, in isolated enrolled beds. Replay narrator read failure, deadline expiry and arming failure, then approved backlog with no job. Observe the provider's real behavior: no infrastructure reprompt; the idle refusal, then the third-call handoff. Retain transcripts, decision records and the steward receipt. |

IdleHandoffRegression retains TestIdleBacklogBlocksTwiceThenDefersClaimAndPreparesStewardContinuation, TestIdleEscalationPreservesAnIndependentOpenWorkBlock, and TestThreeUnreadableLedgerStopsRecordIncidentRaiseAlarmAndEnd in `metasystem/internal/goal/turnverdict_idle_test.go`.
Retain that file's single-use permission assertions, including consume failure; update superseded infrastructure-block expectations without weakening authorization checks.
All process beds own their registry, checkout and recorded child identities. Use bounded fixture waits and clean up only those children.
The direct shell calls at `metasystem/scripts/agents/supervision-hook-fixtures.sh:672`, line 909 and line 1162 remain plumbing tests. They cannot stand in for either live production-runtime leg; the fake is auxiliary.
The Claude and Codex legs supply the two-production-runtime proof. The seven-day observation is member stop-refusals-under-ten-a-day: its data source is the retained stream this goal creates (the hook log's decision lines, one per stop, appended and never replaced, plus the version-2 incident record; the per-session stop-verdict file is replaced at every verdict, `metasystem/internal/goal/turnverdict.go:322`, and cannot count a day) and the version-2 incident record, its acceptance is Wido's on the count the member records on this page.

## 6. Landing

Implementation changes the owners and test files named above, plus `metasystem/cmd/metasystem/report.go` for rendering plumbing and `metasystem/cmd/metasystem/steward_verbs.go` for generation-bound completion.
`metasystem/scripts/agents/supervision-hook.sh` becomes the native transport, and the Claude launcher loses its raw block. The managed-seat host scripts are untouched by this goal.
Wire the gate call into `metasystem/scripts/agents/adapters/codex.sh` at the round's turn boundary (the claude adapter's rounds keep their native hook). Use `metasystem/scripts/agents/hosts/fake.sh` and the fake adapter for auxiliary replay only.
Update `metasystem/docs/design/turn-verdict-delivery-contract.md` and the affected groups in `metasystem/testing.json`; retire superseded first-infrastructure-block expectations in `metasystem/scripts/agents/supervision-hook-fixtures.sh`.
Land the members below in order. Add record readers before switching writers and consumers; keep each member usable on its own. Do not combine the program into one landing or claim universal enforcement before the final member.
Every seat must receive each landed engine or runtime change and re-arm its enrolled supervision set so running components use the new generation. A documentation-only landing needs neither a rebuild nor re-arming.
Use `metasystem/scripts/agents/go-build.sh` for the build and the existing up/re-arm rules. Enrollment drift requiring an agent-free terminal remains the steward's task.
Check installation and actual turn-boundary observation for each seat. Installed hook files alone are not proof that a provider loaded them.
This round revises only this page. Document checks cover its boundary, citations and length; implementation and runtime proof belong to the members and the orchestrator's committed shared testing contract.

## 7. Slices

Keep stop-hook-never-forces-an-empty-turn as the context umbrella. The 2026-09-10 rule requires one mechanism per member, each with its own DONE; this page names them, and the seat opens them as blockers of the umbrella (R-93-m1e) in this order, which is the landing order. Each lands green on its own: no member's DONE needs a later member.

| Member goal | DONE, umbrella clauses and fixtures |
| --- | --- |
| stop-infrastructure-allows-the-seat-to-stop | DONE: every censused producer uses the typed classification, infrastructure alone never blocks a stop (including the three recorded causes), the launcher's raw block on a nonzero hook exit is replaced by the degraded notice, and an observed idle-with-backlog condition still refuses even when its count cannot be spent. Carries clauses 1 (for infrastructure), 2 and 3, preserves clause 4. Fixtures: TestStopConditionClassification, TestSessionStopInfrastructurePreservesAuthority, TestLauncherFailureNeverRawBlocks, TestIdleRefusalSurvivesALostCounter, IdleHandoffRegression, and the fake replays in their first form: each recorded cause allows the stop and writes its hook-log line; the once-per-deadline and notification-identity assertions of those replays join members two and three when they land. Lands first: it is the member that answers the three recorded causes (184 narrator reads, 117 deadlines, about 95 arming failures). |
| stop-decisions-record-deadline-evidence | DONE: each emitted refusal has a prior durable decision with its full up result and the checkout generation, and each incident episode is keyed to the hook's turn generation and deadline end. Carries clause 3's deadline scope and the absorbed persistence and arming-detail clauses. Fixtures: TestInfrastructureDeadlineIdentity, TestStopDecisionPersistsBeforeEmission, TestStopDeadlineCompletionIsSingleUse, TestArmingDetailSurvivesStop. |
| stop-incidents-reach-the-steward | DONE: the steward drains retained stop incidents before narrator work from the version-2 record or, when it is unreadable, from the hook log's unacknowledged lines, acknowledges each delivery as a line in that log so no incident is queued twice, and only a failure of both channels is reported as unconfirmed. Carries clause 2's reporting duty. Fixtures: TestStopIncidentDrainBeforeNarrator, TestStopIncidentDrainFromHookLog, TestStopIncidentDeliveryUnconfirmed, FakeReplayUnreadableHookState. |
| stop-frontier-joins-owner-and-revision | DONE: one fresh board scopes ready work and the WORK IN FLIGHT exemption to the machine, lineage, goal and revision, with a freshness proof, and the ready goal's next-step text is on that board; the idle digest is untouched. Carries the absorbed READY and INFLIGHT clauses. Fixtures: TestStopFrontierOwnerRevisionAndFreshness, TestIdleDigestKeepsEveryNonterminalJob, IdleHandoffRegression. |
| stop-refusals-name-seat-actions | DONE: an eligible seat refusal prints one executable clearing command, the mandatory idle refusal prints the real claim command and the ready goal's next step from the board, and a refusal with no lawful command becomes a notice. Carries clause 1 for the seat and idle paths, preserves clause 4. Fixtures: TestStopClearingCommand, TestIdleClaimCommandIsFirstAct, IdleHandoffRegression. |
| stop-fences-surface-once | DONE: a held fenced goal prints FENCED once per unchanged fence, including beside live work. Carries the absorbed FENCED clause, preserves clause 4. Fixture: TestFencedClaimSurfacedOnceBesideFlight. |
| stop-hosts-enforce-the-common-gate | DONE: the Claude native Stop hook of a seat and the adapter turn boundary of a Codex delegate round both call the one Go gate and enforce its committed decision (the Codex round re-invokes its provider once in the same job under its remaining cap on a refusal, excluding its own job from the live-work exemption), proven live on both. Carries clauses 1 to 4 across the two production boundaries. Fixtures: TestEveryRuntimeUsesStopDecision, the five fake replay legs, RealClaudeNativeStopReplay, RealCodexAdapterStopReplay. The managed-seat lifecycle for hosts without a native Stop is the follow-on goal managed-seats-run-under-the-common-gate, opened when a runtime needs it. |
| stop-refusals-under-ten-a-day | DONE: seven days of hook-log decision lines (one per stop, per seat, never replaced) and version-2 incident records on every seat show fewer than ten refusals per seat per day, and the count with its residual unconfirmed deliveries is recorded on this page for Wido's acceptance. Carries the umbrella's operational clause. Fixture: TestDailyRefusalCountReadsTheLogStream (a synthetic week of lines on two seats counts per seat and day, and a replaced verdict file changes nothing). |

The umbrella closes when its members are done and Wido accepts the seven-day count. No fixture or design return substitutes for those receipts.
