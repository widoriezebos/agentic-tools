# stop-decisions-record-deadline-evidence: build design

- Kind: design
- Id: 01M3A2YHDNQ4X9KAZN3MHX30SW
- Status: done
- Goals: stop-decisions-record-deadline-evidence

Revision: 4 (2026-09-14), following revision 3 at 78f40168.
This revision decides the two hook-bed fixtures that reached the retired shell parent. Other decisions stand.

Member two of `metasystem/plans/stop-hook-never-forces-an-empty-turn-design.md`.
Two reads are spent. This page decides the eight remaining findings
from `metasystem/plans/stop-decisions-record-deadline-evidence-design-read2-findings.md`.
Wido's direction ends prose review here. Codex builds these obligations;
its fixtures and an Opus read of the build judge them. No third design read.

## 1. Grounding and boundary

The Stop parent captures worker stdout; that file is not provider delivery.
`metasystem/scripts/agents/supervision-hook.sh` waits for cleanup and hook-expire
before its deadline fallback can speak.
`runReportTurnVerdict` in `metasystem/cmd/metasystem/goal.go` calls
`MarkOpenWorkSeen` before `TurnVerdict`.
`metasystem/internal/report/openwork.go` writes that marker under an
unbounded flock. `metasystem/internal/goal/turnverdict.go` also changes
seen state and the idle counter before returning its verdict.
The two seconds in `metasystem/internal/goal/goalverbs.go` bound acquisition,
not the work inside the lock. A longer parent lock wait cannot close this race.

Classification, idle digest, the three-observation handoff and human-stop
eligibility stay with their existing goal owners. This member changes when
refusal effects become real. Steward incident delivery and the Codex adapter
boundary remain later members. The per-session verdict file stays display.

## 2. Episode identity: SDE-02 accepted

Keep the canonical checkout state root and the per-session refusal record.
Version 2 has `episodes`, an `invocations` map from token to episode key,
and `upgradedAt` when it imports version-1 history.
The ordinary key is `<generation>/<attempt>/<deadlineEnd>`.
Generation and attempt come from `BeginHookAttempt` in
`metasystem/internal/steward/component_evidence.go`; deadlineEnd is the
outer invocation's fixed UTC epoch-second deadline, never a retry's start.
This member implements the shorter key instead of the umbrella's longer key
because checkout and session scope the record, lease is evidence, and a
condition fingerprint identifies an incident within that invocation.

Replace the uncertain PID/start suffix with `0/0/<deadlineEnd>/<token>`.
The parent generates 32 random bytes and reserves their lowercase hexadecimal
token by exclusive mkdir beneath `artifacts/agents/supervision/stop-invocations`
in that root. Retry collisions; never reuse a reservation directory.
Keep the empty directory permanently after its evidence is pruned.
Failure to reserve yields an unrecorded degraded response, never a borrowed key.
The parent retains the token in memory and gives it to every child explicitly.

An episode stores canonical `checkout`, normalized `session`, `leaseEpoch`,
`enrollmentGeneration`, generation, attempt, deadlineEnd, token and preparedAt.
Unknown lease and enrollment values are zero. They grant no authority.
Prepare binds a token once under the record lock. Repeating it returns that
binding, including an uncertain binding chosen before a late worker arrived.
A later generation report never creates a second episode for the same token.
Different tokens claiming an occupied ordinary key fail with identity conflict;
they do not merge. Incident fingerprints never change the episode key.

## 3. Identity handoff: SDE-03 accepted

The parent publishes an invocation descriptor before starting the worker.
It contains schemaVersion 1, token, checkout, session, deadlineEnd and the
absolute deadline directory. Failure leaves the Stop explicitly unrecorded.
The worker publishes `episode.json` there after hook-attempt and before prepare.
It contains the descriptor's token and deadline plus generation, attempt,
leaseEpoch and enrollmentGeneration. Failed hook-attempt uses zero coordinates.
Both publications use `atomicfile.WriteText` from
`metasystem/internal/atomicfile/atomicfile.go`, with a known existing anchor.
No direct redirection, empty placeholder or in-place replacement is allowed.

The parent strictly decodes one JSON object and validates its token and deadline.
Missing, empty, truncated, malformed or mismatched handoff files are all an
unavailable handoff. First look up the parent's token in `invocations`.
An existing binding wins. Otherwise prepare the uncertain key with that token.
A valid handoff with no binding prepares its ordinary key. A pending binding
is completed by the parent. Thus death on either side of rename or prepare
leaves at most one episode. Temporary files never establish an identity.

## 4. Arming and interfaces: SDE-06 accepted with a factual correction

The finding overlooks `StopArmingResult` and its parser in
`metasystem/internal/report/stoppresentation.go`; they already preserve typed
components and streams, but neither proves that arming finished.
Reuse that presentation owner. Add explicit completeness through an arming
envelope rendered by `up.Result` in `metasystem/internal/up/up.go`.
The envelope has schemaVersion 1, complete, componentLines, otherLines,
aggregate and nullable exitCode. The lines are exact renderer output.
`up --result-file FILE` atomically publishes an initial incomplete envelope,
then a snapshot after each component result, then the final complete envelope.
Incomplete aggregate is exactly `up outcome=incomplete`; exitCode is null.
Complete means the arming call returned, even when its outcome is failure.
Final lines and aggregate equal `Result.Lines()` byte for byte.
Stop presentation reads this envelope instead of guessing from the last line.
Missing or invalid evidence uses the last valid snapshot, or the empty
incomplete envelope. It never changes a separately observed work refusal.

Register `report stop-decision` in `metasystem/cmd/metasystem/main.go`.
Its parser lives in `metasystem/cmd/metasystem/report.go`. The interfaces are:

- `run --installation ROOT --runtime NAME --worker-script FILE`: the Go
  deadline parent; also requires `--deadline-end E`, reads stdin, alone writes stdout.
- `prepare --invocation FILE --generation G --attempt A --lease-epoch L
  --enrollment-generation N`: resolves the descriptor's root, returns its
  record path and bound episode key. Zero generation and attempt mean uncertain.
- `complete --invocation FILE --selection FILE`: parent-only composition.
  Selection is schemaVersion 1 with token, nullable episode, proposal or null, parent cause
  or null, and an arming envelope. Its token must match the descriptor.
- `incident --invocation FILE --episode ID --observation ID --cause CODE
  --component NAME [--kind KIND] [--identity ID]`: records one observation.
- `repair --root ROOT`: repairs retained decision, log and emission intents.

`report turn-verdict` adds separate `--invocation FILE --episode ID
--proposal-file FILE --arming-file FILE` flags to its existing inputs.
All four are required together. It reads only this explicit descriptor;
no environment variable or colon-split path supplies record coordinates.
Absent arming uses the incomplete rule above.
`report stop-block --class infrastructure` uses the same invocation, episode
and observation flags. A legacy call cannot overwrite a version-2 record.
The shell passes the descriptor path to child commands; it owns no JSON state.
Run starts `bash FILE NAME stop --invocation FILE` with the existing parent-PID
guard. That guard prevents recursion; it supplies no decision coordinates.
The proposal contains schemaVersion 1, token, episode, verdict, frozen facts,
deferred effects and a ready provider response. Hash its stored bytes with SHA-256.
Cache a valid fallback rendering of each proposal before accepting it, so a
stalled later formatter cannot erase a known block. Cache arming separately.

### Optional health and narrator evidence: excluded from the record

The decision record carries the arming envelope alone as optional evidence.
Its accepted DONE requires the full up result. Separate readers retain the other facts.
Do not collect health or narrator attachments for the record or frozen selection.
Health stays absent from the provider response. Section 5 preserves narrator report delivery.
Absent, late or unreadable attachments cannot change the decision, add an incident or delay emission.
They receive no acknowledgement. Omission proves no health delivery; hook-complete keeps its proof checks.
The reader loses the attached health and narrator snapshots for this decision.
Current health is read by `health --repo ROOT` in `metasystem/cmd/metasystem/steward_verbs.go`.
That fresh reading cannot reconstruct health at the earlier Stop.
Narrator history stays in `metasystem/records/narrator-digest.log`; `steward digest-pending --repo ROOT` reads
pending text without moving its cursor, through `metasystem/internal/narratordigest/digest.go`.
No later member or goal adds these attachments. That needs a separate accepted design.

## 5. Delivery and the last seconds: SDE-01 and SDE-11 accepted

Replace worker completion with a proposal. The goal owner evaluates on a copy
of refusal state and returns the verdict plus deferred refusal effects.
The command's early `MarkOpenWorkSeen` becomes a read-only seen-state lookup.
Proposal publication changes no open-work marker, goal/free/queue/unwatched
seen marker, watchdog marker, green cursor or idle count. It invokes no idle
continuation or alarm callback. The worker's stdout is only private transport.
Existing human authorization inspection and single-use consumption retain
their checks; they cannot be inferred from a proposal or a failed state write.

The Go parent owns selection, completion and provider emission. Its controller
lives in new `internal/report/stopdeadline.go`; command composition supplies
goal callbacks so the goal package acquires no dependency on report.
The shell replaces its existing deadline block with this command, retaining
the fixed engine-missing allowance when no parent engine can start.
If the engine lacks `report stop-decision`, emit the fixed degraded allowance:
It says "Metasystem engine and hook are out of step" and names rebuilding `metasystem/bin/metasystem`.
No worker starts. Supported attempts complete in the parent process, without the elapsed-flag retry.
Replace that retry fixture with StopEngineSkewAllowance in `metasystem/scripts/agents/supervision-hook-fixtures.sh`.
It refuses the new verb and proves one allowance names the skew and rebuild, with no worker, retry or claimed completion.

T is outer hook entry. Freeze E = T + 60 seconds there, before payload, Git or
resolver work. Pass its UTC epoch-second value explicitly to the Go parent.
Map the remaining time to a monotonic timer; engine startup never resets E.
All filesystem work, command execution and response preparation run off the
timer/output loop. Cancellation does not require their acknowledgement.
The fixed degraded JSON is already in memory before starting those tasks.
Proven foreign-runtime and delegate skips still exit silently without a decision.

| Absolute cutoff | Allowed work and required parent action |
| --- | --- |
| Before E minus 3 seconds (T + 57) | Worker publishes a work proposal before optional arming collection. It may attach newer arming snapshots. Parent validates and caches complete proposals as they arrive. No refusal effects are spent. |
| At E minus 3 seconds (T + 57) | Parent cancels optional work without waiting and stops accepting worker results. In-flight evidence writes may finish; they spend nothing without a matching receipt. Parent freezes the latest validated proposal, or an infrastructure allowance if none exists. An observed block stays a block through recording failures. Expiry or unreadable output adds its parent incident. |
| By E minus 2 seconds (T + 58) | Parent stops waiting for every read, lock, completion write, append and formatter. Record locks get at most 100 milliseconds within this interval, not a fresh relative allowance. A held worker lock cannot extend it. Unfinished persistence is labelled unproven; a known failure is labelled unrecorded. |
| At E minus 1 second (T + 59), latest | Parent starts writing its single, fully buffered response. It uses the frozen decision and cached evidence. No cleanup, resolver join, hook-expire or new engine call precedes that write. |
| By E minus 0.5 seconds (T + 59.5) | The complete response has been written or the bounded output operation has failed. Only full successful output can produce an emission receipt. A partial write, closed pipe or timeout produces none and never starts another JSON response. |
| Until E (T + 60) | Close provider stdout. Attempt the receipt and bounded bookkeeping; then exit. Signal only recorded owned children. Never wait past E for cleanup or hook-expire. Retained intents are recovered on the next Stop. |

Early success follows the same selection, persistence, emission and receipt
order without waiting for T + 57. Invalid worker output starts fallback work
immediately; it gets no new budget. Late results cannot replace a selection.
The selected decision is immutable even if its persistence task finishes late.
Move hook-complete and narrator cursor advancement from worker output to parent
delivery bookkeeping. Captured child stdout must not claim an emitted generation.
Only narrator attachments are excluded. The Stop report keeps delivering the pending digest.
After its matching emission receipt, the parent calls `narratordigest.Advance` for the installation's
human cursor with the cursor and prefix hash frozen with that report. No delivered digest means no advance.
The template-layout report and cursor assertions in `metasystem/scripts/agents/supervision-hook-fixtures.sh` stand.
Remove the `deadline-emitted` marker protocol entirely.

### The record write that fails, and the worker after the cutoff

Two landed hook-bed fixtures reached the shell parent's verbs and file layout.
Both guarantees stand. Their faults and evidence follow the Go parent.

A record write failure at the deadline still allows, says so in one line, and
logs its own outcome. The parent completes the decision in its own process, so
a stub engine refusing `report stop-block` injects nothing; that stub retires.
The fault is a record path the parent cannot write. The bed makes
`artifacts/agents/supervision/stop-refusals` under the fixture's state root
read-only for the one Stop and restores it after. Prepare then cannot create
that session's lock or record, completion returns recorded false with the error,
the allowance ends `; stop deadline expired; record update failed.`, the log line
reads `stop response outcome=deadline-expired-record-failure-allow`, and no record
exists for the session. The hook log and the invocation directory are siblings and
stay writable, so the outcome line and the retained selection still land.
This proof stays in the shell bed as StopDeadlineRecordWriteRefused. The Go tests
of the notice bytes and the outcome name stand; only the bed reaches the real
record owner through the real shell.

The parent starts the worker as its own direct child and holds its handle, so
ownership is never in doubt and `ps` is never consulted. At the freeze cutoff,
when the worker has not returned, the parent ends that child through the handle.
That is the one signal the cutoff table allows. The worker's own children are not
signalled: an engine call still running inside the worker finishes on its own, and
its late result cannot change the selection. The reader is told once, on the
parent's stderr, in the fixed form
`stop deadline: worker <pid> ended at the freeze cutoff; its children were not signalled`.
Nothing the worker wrote to its own stderr before the cutoff is forwarded, because
children it left behind may still hold its pipes. The other evidence is what the
parent already leaves: the outcome line, the retained `stop-deadline-expired`
incident, and the expired hook attempt. No worker directory is left under TMPDIR;
worker output lives in the parent's memory. StopDeadlineEndsOnlyItsDirectWorker proves this.

Completion records class, outcome, reason, command, countState, completedBy
and completedAt. Class remains infrastructure, seat-actionable or idle-with-backlog.
Outcome is block or allow. Command stays empty until clearing commands land.
The parent is the sole completer; completedBy names the selected source,
worker or deadline. CountState is not-spent when effects await delivery,
otherwise none. A proposal is never a completed or counted refusal.

The parent writes an atomic `emission.json` in its reserved invocation directory
only after the complete provider write succeeds. It names token, episode,
selected-decision hash, exact response hash and emittedAt. No child output,
exit code, saved proposal, lock release or marker may substitute for that event.
This proves delivery to the provider pipe; it does not claim a human read it.
Death before that receipt may repeat a refusal. It can never suppress one unseen.
An unresolved binding leaves episode null in selection, receipt and log; token
still joins them. Repair resolves the token once, without inventing a second key.

The goal and open-work owners apply deferred effects only with a matching
receipt. Each effect carries its owner's prior-state digest and invocation token.
Under that owner's lock, apply once and save the applied token in the same
atomic write. An already applied token is success; a changed prior digest is
not-spent, without overwriting newer state. Report per-owner results separately.
The idle count's result alone determines its spent status. Lost writes leave
the next stop eligible. A settlement event records results; it does not rewrite
the pre-emission decision line or claim that it was already counted then.
The display calls a proposed idle occurrence pending until delivery is recorded.
Settlement runs the existing third-observation callbacks before its goal-state
commit, preserving their admission checks and any independent block.
Save the continuation nonce in the proposal; retries reuse it, never mint another.
As today, callback failure records its incident/alarm; it does not invent success.
The command recovers outstanding receipts before evaluating the next Stop.
Never replay effects older than the existing thirty-day verdict-state horizon.
Keep applied tokens outside session/revision caps for that horizon; never count twice.

## 6. Incidents and the permanent stream: SDE-05 and SDE-07 accepted

Complete returns `{episode, decision, alreadyCompletedBy, recorded, recordError}`.
`recorded` is true for durable publication, `unproven` for publication with
unknown durability, and false for a failed publication. Keep SDE-04 exactly.
An unfinished write at the cutoff is also unproven, never claimed durable.
False includes recordError. Notices distinguish record failure from log failure.
A repeat returns the same decision and source. It still upserts its parent
incident in a separate record update, even when the decision already exists.
Failure of that update never erases or changes the decision.

Expiry uses cause `stop-deadline-expired`, component `stop-deadline`.
Unreadable output uses `stop-hook-output-was-unreadable`, component `stop-worker`.
Parent observation IDs are `<token>/deadline` and `<token>/unreadable`.
Retries with the same observation ID do not increase count. Distinct observations
of one fingerprint do. Fingerprints contain cause, component, kind and identity;
they exclude timestamps, elapsed time and wording. Keep firstAt, lastAt, count,
observation IDs and delivery. Incidents remain undelivered until acknowledged.

Before emission, complete saves the frozen selection and parent incidents in
the invocation directory. It then updates the session record, freezes its write
result and exact log bytes in an append intent, and ensures the log entries.
These intents use atomicfile independently of the record lock. If recovery finds
a published record without a saved write result, that result is unproven.
If both stores fail, retain the observed decision and say delivery unconfirmed.
The next prepare and explicit repair recover intents, including a parent that
timed out while the worker held the record lock. They finish the token's one binding.

`stopblock.go` owns the append protocol and a separate checkout-wide log lock.
Replace the hook's direct decision/condition appends with this owner's events.
Send other diagnostics to stderr; only this owner appends to `hooks.log`.
Persist each event's ID and exact bytes in its intent before appending.
Decision ID is `<token>/decision`. Incident ID is token plus fingerprint hash;
its log event ID also includes observation ID. Use SHA-256 over encoded fields.
Under the log lock, scan for that ID. Identical complete bytes mean already
appended. Conflicting bytes fail visibly. Append only when the ID is absent.
Save the starting offset before append. Truncate only a final partial prefix
matching that intent; other corruption fails visibly. Never remove complete lines.
Append the full newline-terminated line and sync the file.
Death before append leaves a recoverable intent. Death after append needs no
second line: recovery scans the log rather than trusting an append-done flag.
Never nest the record and log locks. A failed append is retained for retry and
reported in the provider notice; a sync doubt is reported as unproven.

Each JSON line has kind, eventId, token, episode, checkout, session, leaseEpoch,
generation, attempt, deadlineEnd, aggregate, recorded and recordError.
Decision lines also have class, outcome, reason, command, countState,
completedBy and completedAt. Incident lines carry their full incident fields.
Emission and settlement lines reference the decision ID. Count confirmed
refusals by distinct decision ID with a receipt. Report selected blocks without
receipts separately as unconfirmed; seven-day acceptance cannot count them as zero.
A repeated completion repairs missing lines; it appends no extra relay or decision.

## 7. Retention and the later drain: SDE-08 accepted

Never prune an episode with an undelivered incident, pending append or unsettled
in-horizon emission receipt. Prune other completed episodes seven days after
the later of completedAt and last delivery acknowledgement. Recover selections first.
A pending episode without a selection, past deadlineEnd, completes as an
infrastructure allowance with cause `stop-episode-abandoned`; retain it until acknowledged.
There is no unconditional thirty-day episode deletion. Imported version-1 causes
are history only. Set upgradedAt once during the atomic upgrade and drop only
that history thirty days later. Reopening the record never resets upgradedAt.
Reservation tombstones and complete hook-log lines are never pruned.
This member requires the later drain to union record incidents, retained intents
and unacknowledged log events by incident ID, unlike the umbrella's unreadable-only
log fallback, because a readable record can still lack a failed incident update.
Acknowledgements use the same incident ID across all three sources.
No infrastructure delivery or steward wake-up runs on the Stop path.

## 8. Fixtures: the builder's proof obligations

These tests are build obligations. Use Go fake clocks, injected I/O and barriers.
Crash fixtures restart a fresh owner against the same temporary files after
the named injection point. They must not use sleeps to guess the interleaving.
Hook beds launch the shell and Go parent with owned, bounded fake children.

| Finding and named test | File | Forced fault or interleaving; required observation |
| --- | --- | --- |
| SDE-01: TestStopDeadlineCompletionIsSingleUse | New `cmd/metasystem/stopdecision_test.go` | Barrier holds the worker's record lock before publishing its proposal. Advance parent to T + 58, emit its allowance, then release worker. No receipt matches the worker proposal; every seen marker and idle count is unchanged. Next invocation refuses the same work. Repeat with a cached block and receipt before/after effect save; count once. |
| SDE-01: TestOpenWorkWaitsForProviderEmission | `metasystem/cmd/metasystem/goal_test.go` | Publish a real open-plan proposal, then fault provider output with a short write and closed pipe. The open-work marker remains absent. Full output followed by receipt creates it once. Worker stdout alone never creates it. |
| SDE-11: TestStopParentAbsoluteCutoffs | New `internal/report/stopdeadline_test.go` | Independently stall payload staging, resolver, record read/lock/write, log append, formatter, child wait and hook-expire. Fake time reaches each table cutoff. Exactly one frozen response starts by T + 59; no receipt follows output failure. Late I/O cannot change that response. |
| SDE-02: TestInfrastructureDeadlineIdentity | `metasystem/internal/report/stopblock_test.go` | Same-second retries have different attempts. Two uncertain calls reuse PID and start time; force the random allocator's first collision and observe exclusive retry. Token retries bind once; occupied ordinary keys never merge. Concurrent condition observations count within their own episode. |
| SDE-03: TestEpisodeHandoffCrashWindows | New `internal/report/stopdeadline_test.go` | Crash before handoff rename, after rename before prepare, and after prepare. Inject empty, truncated and wrong-token files separately. Recover by the parent's token; exactly one episode survives each case. Also race uncertain parent prepare with late ordinary worker prepare. |
| SDE-05: TestParentIncidentSurvivesCompletedDecision | `metasystem/internal/report/stopblock_test.go` | Complete a worker-sourced block, then retry completion with each parent cause twice. Decision bytes stay unchanged; each parent incident has count one. Fail the incident update and recover its retained intent without changing the block. |
| SDE-06: TestStopDecisionInterfaceInputs | New `cmd/metasystem/stopdecision_test.go` | Exercise every declared verb. Missing descriptor, mismatched token/root, incomplete flag groups and trailing JSON fail explicitly. Absent arming is accepted as incomplete. No test supplies hidden deadline environment variables. |
| SDE-06: TestArmingDetailSurvivesStop | `metasystem/cmd/metasystem/up_test.go` | Capture ENROLLMENT_DRIFT and failed component output. Compare sidecar, stored episode and Stop presentation lines byte for byte. Interrupt between components; only published components survive and complete is false. |
| Optional evidence: TestStopDecisionCarriesArmingOnly | New `cmd/metasystem/stopdecision_test.go` | Keep a pending narrator entry and health evidence in their existing stores. Try absent, late and unreadable optional attachments. The record and frozen selection carry only arming evidence beside the decision. No extra incident, delay or attachment acknowledgement appears. Separate readers still expose their facts. Stop report delivery and its cursor follow section 5. |
| SDE-07: TestDecisionAppendCrashWindows | `metasystem/internal/report/stopblock_test.go` | Inject death after intent save, after record completion before append, midway through append, and after full append before acknowledgement. Restart repair twice. One complete decision line remains; a relay repairs a missing line. Concurrent emitters cannot interleave bytes. |
| SDE-08: TestUndeliveredIncidentRetention | `metasystem/internal/report/stopblock_test.go` | Advance fake time past thirty days with an undelivered incident, a pending append and an unacknowledged log-only incident. Recover their union once by ID. Acknowledged history prunes after seven days; v1 history uses the original upgradedAt. Retention removes no log bytes. |
| SDE-04 retained: TestStopDecisionPersistsBeforeEmission | New `cmd/metasystem/stopdecision_test.go` | Fault before rename, after rename during directory sync, and in append. Observe false with error, unproven, and true as appropriate before provider output; preserve a known block through persistence failures. |
| Hook bed: StopDeadlineRecordRecovery | `metasystem/scripts/agents/supervision-hook-fixtures.sh` | Launch the Go tests above as plumbing. Drive missing handoff, unreadable worker output, frozen block plus expiry, stalled lock and append failure through the actual shell. Observe one provider JSON response and recoverable parent incidents. |
| Hook bed: StopDeadlineRecordWriteRefused | `metasystem/scripts/agents/supervision-hook-fixtures.sh` | Hang the worker in hook-attempt with the deadline engine stub; make the state root's `stop-refusals` directory read-only for this Stop and restore it after. Observe the record-failure allowance, its outcome line and no record for the session. Replaces the fixture that stubbed `report stop-block`; that stub branch and `METASYSTEM_DEADLINE_RECORD_FAILURE` retire. |
| Hook bed: StopDeadlineEndsOnlyItsDirectWorker | `metasystem/scripts/agents/supervision-hook-fixtures.sh` | Hang the worker in hook-attempt with a stub that records its own pid and waits for a fixture release file, not for its parent. After the parent returns: the plain deadline allowance and outcome line, the fixed stderr line with the worker pid, that pid gone, the stub pid alive; then release the stub and end it by that pid on every exit path. Replaces the empty-ps fixture; its `ps` shim, `TMPDIR` override, `metasystem-stop-deadline` worker directory and `left running, command line unverifiable` line retire. |

Add TestDeferredRefusalEffectsRequireReceipt in
`metasystem/internal/goal/turnverdict_test.go` for missing, mismatched and replayed receipts, including session eviction.
Retain regressions in `metasystem/internal/goal/turnverdict_idle_test.go`.
Add deferred-mode coverage
there: two delivered refusals, then the existing third-observation handoff;
a lost receipt must not advance the counter or consume authorization by inference.
Extend `metasystem/internal/report/stoppresentation_test.go` for incomplete
arming and pending-count wording. Extend `metasystem/internal/up/up_test.go`
for atomic partial and complete arming envelopes.

## 9. Landing

The build changes these existing production files:
`metasystem/internal/report/stopblock.go`, `metasystem/internal/report/openwork.go`,
`metasystem/internal/report/stoppresentation.go`, `metasystem/internal/up/up.go`,
`metasystem/internal/goal/turnverdict.go`, `metasystem/cmd/metasystem/goal.go`,
`metasystem/cmd/metasystem/report.go`, `metasystem/cmd/metasystem/up.go`,
`metasystem/cmd/metasystem/main.go`, and
`metasystem/scripts/agents/supervision-hook.sh`.
Add `internal/report/stopdeadline.go`. Change these tests:
`metasystem/internal/report/stopblock_test.go`,
`metasystem/internal/report/stoppresentation_test.go`, `metasystem/internal/up/up_test.go`,
`metasystem/internal/goal/turnverdict_test.go`, `metasystem/internal/goal/turnverdict_idle_test.go`,
`metasystem/cmd/metasystem/goal_test.go`, `metasystem/cmd/metasystem/up_test.go`,
and `metasystem/scripts/agents/supervision-hook-fixtures.sh`.
Add new `internal/report/stopdeadline_test.go` and `cmd/metasystem/stopdecision_test.go`.
Reuse atomicfile and component evidence; keep new logic in Go.
The orchestrator runs full-width shared testing and the Opus build read, then
rebuilds and re-arms seats through landing. This round changes only this page.

## 10. Moved effects

Recorded 2026-09-15 as the first inventory under goal moved-effects-are-inventoried-in-the-design. It is the sweep build round 8 returned (every effect the Stop worker performed after composing its response, and its owner now) with round 25's per-route hooks-log lines. Code names where the effect is performed before this member lands.

| Effect | From | To | Code |
|---|---|---|---|
| The response line `stop response decision=<decision> elapsed=<n>s` in the supervision hooks log | Stop worker | parent emission recorder, after a successful delivery | `metasystem/scripts/agents/supervision-hook.sh` |
| The elapsed-seconds measurement for that line | Stop worker | parent emission recorder, from the actual post-write time | `metasystem/scripts/agents/supervision-hook.sh` |
| Staging the response bytes in a temporary file, the provider stdout write, and removing the file | Stop worker | deadline controller: holds the bytes in memory, performs the sole provider write, hashes them into the receipt | `metasystem/scripts/agents/supervision-hook.sh` |
| Hook attempt completion on success or unavailable presentation | Stop worker | receipt-gated settlement | `metasystem/scripts/agents/supervision-hook.sh` |
| Hook attempt completion when provider output fails | Stop worker | parent failed-delivery callback, closing the attempt as provider-delivery-failed with no receipt | `metasystem/scripts/agents/supervision-hook.sh` |
| Narrator digest advancement and its failure warning | Stop worker | settlement result checked against the held report and its matching receipt | `metasystem/scripts/agents/supervision-hook.sh` |
| Protocol-error cursor advance (main identity, caller process, counts) | Stop worker | settlement, after matching delivery, with the identity carried in the held report | `metasystem/internal/lease/verbs.go` |
| The full turn-verdict artifact and turn-verdict state | Stop worker | receipt-gated goal settlement | `metasystem/internal/goal/turnverdict.go` |
| The idle continuation or alarm and its occurrence count | Stop worker | receipt-gated goal settlement | `metasystem/internal/goal/turnverdict.go` |
| One-use session Stop consumption | Stop worker | receipt-gated goal settlement | `metasystem/internal/goal/sessionstop.go` |
| Open-work seen state | Stop worker | receipt-gated open-work settlement | `metasystem/internal/report/openwork.go` |
| The decision or outcome trail line, and the condition line before it when the decision carries an infrastructure condition, on every delivered route | Stop worker (`append_stop_condition`) | parent, per delivered route | `metasystem/scripts/agents/supervision-hook.sh` |
