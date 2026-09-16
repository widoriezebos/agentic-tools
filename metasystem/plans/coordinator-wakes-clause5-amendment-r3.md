## 7. Amendment: how clause 5 is measured (revision 3)

Revision 3 for the m1b seat on 2026-09-16, folding the Codex critique of revision 2 (`metasystem/plans/coordinator-wakes-clause5-amendment-r2-critique-r1.md`, nine findings, five critical). Every citation was re-read against the tree at 63fe67f37. This revision takes path B: it concludes that the engine cannot prove which model request consumed a wait result, withdraws the resume token and the `wait consumed` echo of revision 2, states exactly what engine records alone can measure, names what would make the whole clause measurable, and leaves the clause itself to Wido.

Read this first. The engine never sees a model request; it sees its own subprocesses and its own files. Every carrier it can hand the model passes through the harness and is readable by every process in the seat's tree, so no engine-only record can tell a model's echo from a script's. Records alone therefore bound clause 5 from below: they can prove a sample failed the 60-second rule and can never prove one passed. Passing needs a per-request record attested by the harness or provider through the adapter, which a runtime either supplies or does not. Which of the three ways forward to take is a decision on the clause, and that is Wido's, not this page's.

Clause 5 says: for each event, measure durable publication to the first resumed model turn consuming its wait result; every sample under 60 seconds; notification time, command return time and averages are not substitutes (`metasystem/plans/coordinator-wakes-on-events-not-polls-design.md:228` to `:230`). Line 216 makes adapters the source of inference starts through `metasystem/internal/usage`, with missing native data unavailable and never estimated; line 219 says an inference start is one provider request and that a process checking a record is not one; line 231 says missing streams leave clause 5 unproven.

### 7.1 Finding 1, judged: the echo cannot carry the endpoint

Revision 2 minted a token in `finishV2`, persisted it in the row's result, emitted it in `wait-returned`, printed it, and treated its echo by `wait consumed` as proof that the echoing request had the result in its context. The critique is right, and the defect is not in the placement of the token. It is in what the engine can observe.

What the tree shows:

- `finishV2` (`metasystem/internal/run/waiter.go:643`) builds the result at `:645`, persists it into the row at `:647` to `:656` (`current.Result = &result` at `:655`), and only then returns it at `:661`. Anything in the result is on disk before the caller prints it.
- The event stream is one append-only file, `artifacts/agents/events.jsonl` under the state root, opened with mode `0o644` (`metasystem/internal/events/emit.go:124` to `:128`). Any process that can read the state root can read every event.
- The caller of a wait verb is classified by process ancestry: the walk at `metasystem/internal/lease/classify.go:428` to `:431` makes any descendant of an authenticated main that main, with that main's announcement. `wait` and any sibling verb take their session from that announcement (`metasystem/cmd/metasystem/wait_verb.go:144` to `:157`). A Stop hook, a `PostToolUse` hook, a wrapper script and a model tool call all run as descendants of the harness and are indistinguishable at this seam.
- The result reaches the model as text on standard output (`metasystem/cmd/metasystem/wait_verb.go:346` to `:361`). The harness reads that text and forwards it to the provider; the harness also hands it to whatever hooks it runs. The engine does not see either forwarding.

So a token that is only on standard output is no better than one on disk, and an unforgeable token does not help: the model knows nothing that the harness did not give it, and the harness gives the same bytes to its hooks. Process ancestry cannot separate a tool call from a hook, and any environment variable a harness sets to mark one is that harness's own convention, which the binding constraint forbids as a carrier. Timing cannot help because the engine has no stamp for the request at all. Reading the transcript is a runtime-specific accelerator and, under the same constraint, may tighten and never carry.

Conclusion. No engine-side mechanism can establish that a model request carried a wait result, because the engine's only witnesses are its own process tree and its own files, and both are equally available to a process that merely reads records. The echo is withdrawn: no `resumeToken`, no `consumeCommand`, no `wait consumed` verb, no `wait-consumed` event, no echo-first doctrine line, and no `token-unknown` or `no-echo` class. Revision 2's row for finding 1 of critique r1 was wrong to say the request was proven; that finding is reopened and conceded here.

What the echo would have measured, if the seat is honest, is the time to the seat's first act after the return. That is a useful operational number and it is not the clause's quantity, so this revision does not report it under clause 5. If Wido wants it as a separate operational line it can be added later under its own name; it is not proposed here.

### 7.2 What engine records alone can measure

The engine writes every edge of one interval: from the moment a publication owner began its durable write, or the last observation that did not see it, to the moment the waiter returned the result to its caller. Call that interval R, durable publication to result return. The clause's quantity, call it L, runs from the same start to the start of the consuming request. A result cannot be consumed before the waiter has returned it, so L >= R for every sample. The clause requires L < 60 seconds. Therefore:

- a sample whose R is certainly at or above 60 seconds is a proven failure of clause 5;
- a sample whose R is certainly below 60 seconds proves nothing about clause 5, because the part of L after the return is unobserved;
- there is no sample class that records alone can mark as passed.

Line 230 forbids substituting command return time for the latency. This revision does not substitute; it reports R as a lower bound of L, with the verdict vocabulary below, and never prints a pass.

Stamps. All on the host boot clock, `CLOCK_MONOTONIC_RAW` with `kern.boottime` as the boot identity (`metasystem/internal/identity/identity_darwin.go:23` to `:36`; Linux in `identity_linux.go`). Wall stamps are printed and never subtracted. Every stamp travels with the boot identity it was sampled under; this folds finding 5, which showed that one `bootId` per event cannot validate stamps sampled by different processes.

| Stamp | Sampled where | Carried as |
| --- | --- | --- |
| `registeredBootNanos`, `registeredBootId` | `Store.Wait` before the pending persist | new row fields; copied into `wait-returned` |
| `prevObservedBootNanos`, `prevObservedBootId` | the row's `lastObservedBootNanos` (`metasystem/internal/run/waiter.go:163`) and a new `lastObservedBootId` as they stood when `finishV2` was entered | `wait-returned` |
| `observedBootNanos`, `observedBootId` | the loop iteration's own sample, passed into `finishV2`; for an at-entry finish (`:982` to `:983`, `:1354` to `:1355`) the registration sample | `wait-returned` |
| `returnedBootNanos`, `returnedBootId` | inside `finishV2` beside `now` (`:644`) | `wait-returned` and the row's result |
| `beganBootNanos`, `beganBootId` | by the owner, once, before its first durable write attempt (7.3) | `WaitHint`, then `wait-published` |
| `publishedBootNanos`, `publishedBootId` | inside `NotifyWaiters` (`metasystem/internal/run/wait_hint.go:137`) after the loop | `wait-published` |

Events. Three, component `run`, emitters `["run"]`, registered in `metasystem/scripts/agents/event-registry.json` as revision 2 described. `wait-consumed` is gone.

| Event | Emitted where | Required fields |
| --- | --- | --- |
| `wait-registered` | after the pending persist in `Store.Wait` (`metasystem/internal/run/waiter.go:975` to `:978`), in `renewSavedWait` and in `ResumeWait` takeover at their equivalent point | `waitId`, `nonce`, `kind`, `targetId`, `runtime`, `runtimeSession`, `mainId`, `mode` (register, renew, takeover), `registeredAt`, `registeredBootNanos`, `registeredBootId`, `accelerator` |
| `wait-published` | inside `NotifyWaiters` after its loop, with the loop's `Matched` and `Delivered` (`metasystem/internal/run/wait_hint.go:154`, `:168`) | `kind`, `targetId`, `publicationId` (7.3), `matched`, `delivered`, `beganBootNanos`, `beganBootId` (zero and empty when the owner supplied none), `publishedBootNanos`, `publishedBootId` |
| `wait-returned` | in `finishV2` after the persist succeeded (`:647`) and before `removeWaiterHint` (`:660`), by checked append (7.4); in the replay path (`:1170` to `:1173`) with `mode` replay | `waitId`, `nonce`, `kind`, `targetId`, `runtime`, `runtimeSession`, `mainId`, `mode` (register, renew, takeover, replay), `atEntry`, `state`, `exitCode`, `sourceOutcome`, `sourceEvidence`, `ledgerTip`, `terminalStamp` (report only), the six boot stamps of the table above with their identities, and the wall `registeredAt`, `prevObservedAt`, `observedAt`, `returnedAt` |

`mode` and `atEntry` are stored in the row at registration, renewal and takeover, and `finishV2` reads them from the row; this folds finding 8, because the shared loop (`:1536` re-enters `waitLoop` after a takeover) then needs no extra argument to know how the row was created. `finishV2` gains one argument, `finishStamps{AtEntry bool; ObservedBootNanos int64; ObservedBootID string}`, and every caller supplies it: the loop passes its iteration sample; the two at-entry calls (`:983`, `:1355`) pass `AtEntry` true and the registration sample; the two failed-persist calls (`:980`, `:1352`) pass the registration sample with `AtEntry` false.

Edges, for one ready return:

- `publishedLower` is `beganBootNanos` when a `wait-published` matches (7.3) and carries a non-zero one; else `prevObservedBootNanos`; for an at-entry return with no match there is no lower edge and the sample is `unavailable: no-lower-edge`.
- `publishedUpper` is the smaller of `observedBootNanos` and the matching `wait-published`'s `publishedBootNanos`; `observedBootNanos` alone when there is no match.
- `returned` is `returnedBootNanos`.
- `rUpperNanos` = `returned` minus `publishedLower`; `rLowerNanos` = max(0, `returned` minus `publishedUpper`). The report prints both, their difference as the uncertainty, and which stamp supplied each edge.

Validation before arithmetic, each failure an unavailable reason:

- `clock-not-comparable`: any used stamp is zero, or the boot identities of any two used stamps differ.
- `stamp-order`: `registeredBootNanos` <= `prevObservedBootNanos` <= `observedBootNanos` <= `returnedBootNanos` fails, or `publishedLower` > `publishedUpper`.

Verdict per sample, and there is no pass:

- `refuted`: `rLowerNanos` >= 60 seconds. Even the latest possible publication instant is 60 seconds or more before the return, so L >= 60 and clause 5 fails on this sample.
- `suspect`: `rUpperNanos` >= 60 seconds and `rLowerNanos` < 60 seconds. Publication lies somewhere in the bracket; the report names the loose edge (`lower-edge-prev-observation` when no owner sample matched).
- `returned-within-60`: `rUpperNanos` < 60 seconds. The engine's part is inside the bound; clause 5 is still unproven for this sample because consumption is unobserved. The report prints this class name, never the word pass.

Sample classes. The universe is every registration known from `wait-registered` events and from rows (7.4):

- sample: a ready return with `mode` register, renew or takeover, `atEntry` false.
- `at-entry`: a ready return with `atEntry` true. Sampled with the same edges and the same verdicts, listed under its own class. This folds finding 6: an event that was durable long before the seat registered and returned at entry can be a `refuted` sample on the owner's `beganBootNanos`, and the design must not exclude it. Whether a `refuted` at-entry sample means the wait mechanism failed or the seat registered late is a reading of the clause; the report shows `registeredBootNanos` minus `beganBootNanos` beside it so Wido can read it either way, and the class is never dropped by the design.
- not sampled: deadline, failed and interrupted returns; replay returns. Counted and listed.
- `pending-at-cut`: a registration whose row is still pending at `--until`. Counted; not a sample and not a failure.
- `unavailable: return-unrecorded`: a registration with no `wait-returned` event whose row now holds another nonce or no result. The return happened and its record is gone; counted as unavailable, never as pending.
- `unavailable: no-lower-edge`, `unavailable: clock-not-comparable`, `unavailable: stamp-order`: as above.

Per runtime, grouped by the events' own `runtime` field (taken from `view.Announcement.Runtime` at `metasystem/cmd/metasystem/wait_verb.go:184` into the row), the report prints the raw samples, the maximum `rUpperNanos`, the counts by class and the unavailable list. There is no mean.

The whole-clause verdict from records alone is one of: `refuted` (any sample refuted), `unproven` (otherwise). Clause 5 is unproven for a runtime while any sample is `suspect` or unavailable, and it remains unproven even when every sample is `returned-within-60`, because records do not reach the consuming request.

### 7.3 The publication edge, folded (findings 2 and 3)

Finding 2: `wait-published` was joined to the return by kind, target, boot identity and time window, so a dropped hint for the consumed publication could be replaced by a later hint for another write to the same goal and shorten the bracket. Folded by joining on publication identity. `WaitHint` gains `PublicationID string`, and `wait-published` carries it. The identity is the string the corresponding observer reports as the return's `sourceEvidence` for that write, or the ledger tip for goal and landing writes, so the join rule is: same `kind`, same `targetId`, same boot identity, and `publicationId` equal to the return's `sourceEvidence` or `ledgerTip`. No window. A return with no equal-identity hint is `unhinted` and uses the previous observation. The builder reads the evidence strings the three observers produce in `metasystem/internal/dispatch/watch.go`, `metasystem/internal/proofrun/attempt.go` and `metasystem/internal/goal/attention.go` before wiring, so that owner and observer compute the same string; that check is a builder step, not a decision left open.

Finding 3: the pre-write locations revision 2 cited are helper declarations, and their calls follow the writes. Folded by naming the real sites and the retry rule. The owner samples the boot clock once, before its first durable write attempt, and carries that sample to the hint; a retried write does not re-sample, so the sample stays a lower bound of the publication instant.

| Owner | Durable write | Hint call after it | Where the pre-write sample is taken |
| --- | --- | --- | --- |
| job transitions | `writeRecord` at `metasystem/internal/dispatch/record.go:386`, `:489`, `:582`, inside a locked transaction closure | `notifyJobWaiters` at `:393`, `:496`, `:592` (helper at `:37` to `:39`) | before entering each transaction, in the enclosing function; passed into the helper |
| proof attempt | `FinalizeAttempt` at `metasystem/internal/proofrun/launcher.go:488` | `hintTerminal` at `:495` to `:500` | before `:488` |
| goal publish | the push that the postcondition confirms before `:730` | `hintConfirmedWaiters` at `:557`, `:730`, `:788` (helper at `:500` to `:508`) | once at the start of the publish operation, before the first push attempt, and passed to the `:730` call only; the `:557` idempotent outcome and the `:788` `AlreadyApplied` outcome describe a write made by an earlier operation, so their hints carry zero (`unhinted` for the join, or the earlier operation's own hint if it was recorded) |
| landing | the pushes preceding `metasystem/scripts/agents/land.sh:980`, `:1170`, `:1200`, the last inside a `push_attempt` loop | `hint_landing_waiters` (`:65` to `:67`) at those three lines through `wait notify` | before the first push attempt of each site, with `metasystem util bootclock`; passed as `--began-boot-nanos N --boot-id ID --publication-id TIP` |

### 7.4 Durability of the return record (finding 4)

Revision 2 said a failed `wait-returned` append prevented result delivery. It did not, and it could not, because `Emit` is best-effort and silent (`metasystem/internal/events/emit.go:41` to `:45`, `:120` to `:133`). Folded:

- `wait-returned` is written by a new `Emitter.EmitChecked` that returns the registry refusal, the cap overflow or the write error. `Emit` stays as it is for everything else.
- On failure, `finishV2` still returns the persisted result. Measurement never blocks delivery. The result gains `returnEventFailed` true, which the caller prints and the row already holds.
- The measurement universe is rows and events together: `wait measure` reads every version-2 row under the state root as well as `events.jsonl`. A row whose retained result has no matching `wait-returned` supplies the return's stamps from the row (the result carries them, 7.2) with `stampsFrom` row. Terminal rows stay until a replacement wait for that key (`metasystem/plans/coordinator-wakes-on-events-not-polls-design.md:74`), so the window in which a return can vanish is the replacement of its row after a lost event, and that case is exactly `unavailable: return-unrecorded`.
- `wait-registered` and `wait-published` stay best-effort. A lost registration event is recovered from the row while it exists and is otherwise invisible, which the report states in a coverage line; a lost hint loosens a bracket.

### 7.5 What would make the whole clause measurable

The clause's endpoint is a provider request. Only the harness that sent it or the provider that served it can say when it started and what it carried. The design already assigns that role: adapters supply inference starts through `metasystem/internal/usage`, and missing native data is unavailable, never estimated (`metasystem/plans/coordinator-wakes-on-events-not-polls-design.md:216`). Today that channel carries token counts keyed by request (`metasystem/internal/usage/calls.go:21` to `:31`), and the readers parse only the assistant side: Claude samples are `assistant` lines with `message.usage`, identified by `requestId` (`metasystem/internal/usage/calls_claude.go:71` to `:73`, `:99` to `:102`); Codex samples are `token_usage_record` payloads identified by `response_id` (`metasystem/internal/usage/calls_codex.go:123` to `:125`). Neither reader looks at what a request carried.

The missing input is a per-request attestation, declared as an adapter capability beside the usage capability (`metasystem/internal/usage/calls.go:16` to `:18`; the steward reads the declaration as `ContextSample`, `metasystem/internal/steward/contextreport.go:288`). One record per inference request of the seat's session:

- `requestId`: the provider's identity for the request, the same string the usage reader already uses.
- `requestStartAt`: the harness's wall stamp for the request start, plus a pairing sample `(wallAt, bootNanos, bootId)` taken by the adapter when it reads, so that the engine can place the request on the boot clock with a stated uncertainty and refuse it as `clock-not-comparable` when the pairing disagrees with the engine's own pairing at return by more than that uncertainty. This is what line 229 asks for: record clock uncertainty and fail when the upper bound exceeds 60 seconds.
- `carried`: the wait results in the request's input, identified by the `waitId` and `nonce` the engine printed on the `WAIT` line. The first request whose `carried` contains a return is the consuming request; its start, on the boot clock, is `consumedUpper`; L upper is that minus `publishedLower`.

Per runtime:

- Claude. The transcript already holds the linkage: the `user` line carrying the `tool_result` block for the `wait` tool call, and the next `assistant` line with its `requestId` and `timestamp`. The reader today discards non-assistant lines; a new reader in `metasystem/internal/usage` would parse the `tool_result` content for the `WAIT` line's `waitId` and `nonce` and attach them to the following request. This is a read of one runtime's transcript format and therefore an adapter-supplied input, not engine correctness. The delivery contract classes the real Claude host as unobserved (`metasystem/docs/design/turn-verdict-delivery-contract.md:118` to `:124`); the attestation would need one real-host observation before it is trusted.
- Codex. The rollout carries `response_id` per `token_usage_record`. Whether it records function call outputs with a request linkage has not been observed in this repository; the same contract line marks Codex unobserved. Until observed, Codex supplies no attestation and clause 5 is unavailable for it.
- Fake. The fake adapter emits a synthetic attestation so fixtures exercise the whole seam; it is not host evidence, as the contract already says of the fake runtime.
- Devin and ACP hosts. Per-invocation usage only (`metasystem/internal/steward/context.go:340` to `:342`); no attestation; unavailable.

How this sits with the binding constraint. The wait mechanism's correctness, that a result is delivered, recovered and never lost, comes from rows and events on every runtime, with or without an accelerator. The clause-5 proof is a measurement of a provider request, and the clause itself makes adapters its source and calls missing data unavailable. A runtime with no attestation gets the same mechanism and gets clause 5 marked unavailable, which is what line 231 already says. If Wido reads the constraint more strictly, so that the clause-5 proof itself must come from records alone, then clause 5 cannot be proven on any runtime and either the clause or the constraint has to change. That reading is his to make; this page states both and chooses neither.

### 7.6 Consequence for the parent's DONE

Clause 5 is the parent's own DONE clause (`metasystem/plans/coordinator-wakes-on-events-not-polls-design.md:246`), and the parent owns the measurements and both real legs before claiming DONE (`:258`).

Provable now, from records alone, once 7.2 to 7.4 are built: for every event on every runtime, the interval R with both edges and its uncertainty; a proven failure whenever R is certainly at or above 60 seconds; the counts of registrations, returns, at-entry returns, pending and unavailable. The two real legs can end with `refuted` or `unproven` and the JSON is kept either way.

Not provable now, on any runtime: the consuming request's start, hence any clause-5 pass. The legs' `unproven` result is not a defect in the legs; it is the clause asking for something the engine does not witness.

Provable after 7.5, on a runtime whose adapter supplies the attestation and whose real host has been observed once: L upper per event and a pass or fail against 60 seconds. Unavailable on every other runtime.

The three ways forward, for Wido: keep the clause as written and open the attestation work of 7.5 as its own member, accepting that clause 5 stays unproven on runtimes without it; or change the clause's endpoint to the return, which the clause's own line 230 forbids today and which only he can rewrite; or keep the clause and leave the parent short of DONE with the records-only measurement landed and reporting `unproven`. This page recommends nothing beyond building 7.2 to 7.4 in every case, because they are needed under all three and they already give the refutation power the clause lacks today.

### 7.7 Verbs and fixtures for the records-only measurement

`metasystem wait measure --root DIR [--since RFC3339] [--until RFC3339] [--runtime NAME] [--json]`, beside `wait notify` (`metasystem/cmd/metasystem/wait_verb.go:61` to `:66`). It reads rows and `events.jsonl`, builds the samples of 7.2 and prints per runtime. Exit 0 when the whole-clause verdict is `unproven` with no `suspect` and no unavailable sample; 1 when any sample is `refuted`; 2 when none is refuted but any is `suspect` or unavailable; `ExitInvalidWait` on bad arguments. It emits nothing and reads nothing from the usage store; the usage tightener of revision 2 is dropped with the echo, because without a consuming request there is nothing for it to tighten. The logic lives in `metasystem/internal/usage/waitmeasure.go` so section 5 row 5 keeps its owner file. `wait notify` gains `--began-boot-nanos`, `--boot-id` and `--publication-id`; `metasystem util bootclock` prints `bootId nanos`.

| Fixture | Owner file | Sets up | Asserts, and the false rule each case would expose |
| --- | --- | --- | --- |
| TestWaitLifecycleEvents | `metasystem/internal/run/waiter_test.go` | A fake boot clock and an injected emitter; one fresh registration returning at entry; one pending registration observed three times then ready; one renewal at entry; one takeover; one replay; one deadline; one interrupt; one persist made to fail; one return-event append made to fail | every field of 7.2 with its own boot identity; `mode` and `atEntry` correct for all five creation paths; a failed persist emits nothing; a failed return append still returns the result with `returnEventFailed` true and the row holds the stamps; the replay carries `mode` replay |
| TestWaitPublishedAtOwners | `metasystem/internal/dispatch/record_test.go`, `metasystem/internal/proofrun/launcher_test.go`, `metasystem/internal/goal/txn_test.go` | Each Go owner with a boot clock that advances between the pre-write sample and the write; a goal publish that retries once; an idempotent and an `AlreadyApplied` publish | `beganBootNanos` smaller than `publishedBootNanos`; the retried publish carries the first sample; the two already-written outcomes carry zero; `publicationId` equals what the matching observer reports for the same write |
| TestWaitMeasurementAccounting | `metasystem/internal/usage/usage_test.go` | Synthetic rows and streams, one case per rule: a hinted return well inside 60; an unhinted return; a hint for a different publication of the same target inside the window; a return 70 boot-seconds after `beganBootNanos` with a backward wall step; a return whose bracket straddles 60; an at-entry return with and without a hint; a lost return event with its row present; a lost return event with its row replaced; a `wait-published` under another boot identity; a stamp out of order; deadline, interrupted and replay returns; a pending row at the cut | `returned-within-60`; `returned-within-60` with the loose edge named; the foreign hint ignored and the sample `unhinted`; `refuted` although wall arithmetic gives zero; `suspect`; `at-entry` sampled and, without a hint, `no-lower-edge`; `stampsFrom` row; `return-unrecorded`; `clock-not-comparable`; `stamp-order`; not sampled; `pending-at-cut`; the word pass absent from every output |
| TestWaitMeasureVerb | `metasystem/cmd/metasystem/wait_verb_test.go` | A state root with the previous fixture's rows and streams | exit 0, 1, 2 and `ExitInvalidWait` by case; grouping by the events' `runtime`; two concurrent runs identical and appending nothing |
| Bed leg wait-measure-fake | `metasystem/scripts/agents/supervision-fixtures.sh`, wired in `metasystem/testing.json` | The fake adapter driving `wait` three times: a job wait whose fifo is removed before the job ends; a run whose registry copy omits `wait-published`; a landing through `metasystem/scripts/agents/land.sh` | every sample available; the unhinted sample's bracket wider than the hinted control; the landing sample's `publishedLower` is the script's pre-write sample; the verb exits 0 with whole-clause verdict `unproven` |
| Legs wait-whole-goal-claude and wait-whole-goal-codex | Existing names at section 5 row 5 and line 246 | Unchanged setup, `wait measure` at the end | no `refuted`, no `suspect`, no unavailable sample; the JSON kept with the leg; the verdict `unproven` recorded as such, not as green |

### Moved effects

No owner moves.

| Effect | From | To | Code |
| --- | --- | --- | --- |

Everything this revision adds is additive: the waiter keeps observing and returning, the four publication owners keep writing then hinting, the three observers keep stamping `terminalStamp`, and the stop gate is untouched. Revision 2's token, echo verb, `wait-consumed` event, echo doctrine and usage tightener were never landed, so their withdrawal moves nothing. The attestation of 7.5 is not part of this revision's build; if opened, it would be a new adapter capability, not a moved one.

### A builder may implement 7.2 to 7.4 and 7.7 now, without further decisions

1. Registry entries for `wait-registered`, `wait-published` and `wait-returned` with the fields of 7.2.
2. `Emitter.EmitChecked` in `metasystem/internal/events/emit.go`; `Emit` unchanged.
3. Row fields `runtime`, `mode`, `atEntry`, `registeredBootNanos`, `registeredBootId`, `lastObservedBootId`; result fields for the six boot stamps with identities, `returnEventFailed`.
4. `finishV2` takes `finishStamps`, samples `options.BootClock()` beside `:644`, reads `mode` and `atEntry` from the row, and emits `wait-returned` by checked append after `:647` succeeds and before `:660`; the five callers as in 7.2; the replay path emits `mode` replay.
5. `WaitHint` gains `BeganBootNanos`, `BeganBootID`, `PublicationID`; `NotifyWaiters` samples and emits after its loop; the four owners as in 7.3; `wait notify` and `util bootclock` as in 7.7.
6. `wait measure` as in 7.7; the six fixtures; the two real legs extended.

No doctrine line is added: nothing is asked of a runtime or a seat by this revision.

### Findings of critique r2-r1, one row each

| # | Materiality | Disposition | Reason |
| --- | --- | --- | --- |
| 1 | critical | FOLDED | Conceded in full: the echo is withdrawn because the engine cannot tell a model's echo from any process in the main's tree (`metasystem/internal/lease/classify.go:428` to `:431`) and the consuming request is declared unobservable from records (7.1). |
| 2 | critical | FOLDED | `wait-published` is joined by `publicationId` equal to the return's `sourceEvidence` or `ledgerTip`, never by window; already-written goal outcomes carry no sample (7.3). |
| 3 | critical | FOLDED | The real write sites and their hint calls are named per owner, the sample is taken once before the first attempt, and retries never re-sample (7.3). |
| 4 | critical | FOLDED | `wait-returned` is a checked append; a failed append still returns the result and marks it; rows join the measurement universe so a lost event with a live row is a sample and a lost event with a replaced row is `return-unrecorded`, never `pending-at-cut` (7.4). |
| 5 | critical | FOLDED | Every stamp carries its own boot identity in the row and the events, and validation requires all used identities equal; the return-versus-echo comparison is gone with the echo (7.2). |
| 6 | high | FOLDED | At-entry returns are sampled under their own class with the same edges and verdicts, and a late registration can be `refuted` on the owner's sample; the design no longer excludes them (7.2). |
| 7 | high | FOLDED | `token-unknown` and its unconstructible event are withdrawn with the echo (7.1). |
| 8 | high | FOLDED | `mode` and `atEntry` are stored in the row at each of the three creation paths and read by the shared `finishV2`, so no caller has to guess (7.2). |
| 9 | high | FOLDED | The section carries the validator's declared-none phrase and the four-column table, and says what was additive and what was never landed. |

### Carried forward from critique r1

| r1 # | Reopened by this revision | Disposition now |
| --- | --- | --- |
| 1 | Yes: revision 2's fold by echo is retracted | Conceded; the consuming request is unobservable from records and needs the attestation of 7.5. |
| 3 | Yes: a runtime with no usage stream | Such a runtime gets the records-only measurement and, for the endpoint, `unavailable`; no turn-end witness is reintroduced. |
| 6 | Partly: durability | Return record checked; rows in the universe; registration and hint events best-effort with the coverage line (7.4). |
| 7 | Yes: the expected-return universe | Defined from rows and registration events together; `return-unrecorded` is derivable (7.2, 7.4). |
| 8 | Moot for this build | The usage tightener is dropped; the attestation reader of 7.5, if opened, must ingest before it reads, as r1 finding 8 said. |
| 11 | Yes: fixtures re-specified | Each fixture row names the false rule it would expose (7.7). |
| 2, 4, 5, 9, 10 | No | Their folds from revision 2 stand: boot-clock edges, session grouping by the row's runtime, additive observers, emit after the hint loop, `:982` to `:983` as the at-entry path. |
