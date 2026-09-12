# coordinator-context-stays-under-budget: design critique, round 2 (Codex gpt-5.6-sol, 2026-09-12)

Read of revision 2 (5b5e917b). Verdict: rework required; six material findings. Dispositions on the page's critique record.

# coordinator-context-stays-under-budget: design critique, round 2

Verdict: **rework required**. Revision 2 improves the decomposition and vocabulary, but six material gaps remain. Slice 1b is not ready as written. The governing test from the design-critique skill is: “Would an implementer working from this design build something DIFFERENT, or WRONG, because of this finding?”

Evidence: static read of revision 2 at 5b5e917b, its goal page, round 1, slice 1 at 644960b4, the cited Go and shell owners, and current Claude/Codex record shapes. No repository files were edited and no tests were run.

## Material findings, highest first

### R2-1: Critical: Section 5: the successor is allowed to launch while the predecessor is alive

CDB linkage: CDB-4 is only restated and contradicted by the cited mechanism; CDB-5 is restated as a desired condition, not implemented.

- `decideForRevival` explicitly exempts `seatIdle` from ordinary live-main suppression: its comment says the main is expected to remain alive, and `TestSeatIdleIntentLaunchesAlthoughTheSeatMainIsAlive` requires launch with `Workers.Live == 1`.
- `steward-continuation.md` checks for recent commits or receipts only after the successor has launched. That heuristic neither proves predecessor death nor revokes a resumable old session.
- `session stop` is not an exit primitive. `runSessionStop` is human-only and writes a quiet-stop authorization; `session end` retires an unused authorization. Neither terminates the parent harness. A `context handoff` CLI subprocess also cannot prove that its caller has exited or that no harness tool call is pending.
- The fixed mutable path `context/<seat>/state.json` is not tied to the staged intent nonce. A delayed old continuation can read a later handoff after the file is overwritten.
- Failure scenario: the predecessor writes state and returns from the handoff tool; the watchdog consumes a `seatIdle` intent and launches the successor while the predecessor remains live. The old session is resumed and both mutate the held goal. Or a second handoff overwrites `state.json` before the first continuation reads it.
- Required correction: stage an immutable nonce-bound state file and digest, and define an observed predecessor-death acknowledgement before `CompleteRevival` may launch. If actual death cannot be observed, add an ownership epoch checked at every model-call and mutation boundary; the current system has no such complete boundary.
- Gate: must fix before slice 3, not before a strictly isolated slice 1b.

### R2-2: Critical: Preamble and Sections 4, 6, and 7: revision 2 changes DONE instead of satisfying it

CDB linkage: CDB-1 is only restated. The page openly waits for a human decision, so it is not an accepted design decision.

- The goal requires cumulative per-call context to be bounded by construction, runtime independence, measurement through adapter usage records, and the same guarantee without an accelerator. Revision 2 instead provides post-call observation for hosted native reads/model calls, excludes Devin and ACP from proof, and says a call above 200k is merely a recorded breach.
- A weekly failure is evidence, not a bound. `internal/spend` itself says it measures “without controlling admission,” and the Stop health preview runs after a call.
- Failure scenario: at 149k, a native read adds 60k and the harness makes the next call. The engine records 209k afterward; all designed mechanisms behaved as specified, yet DONE was violated.
- Required correction: either narrow the goal with Wido’s recorded decision, or name a runtime/adapter-owned pre-invocation and tool-delivery boundary for every rostered runtime. Do not call retrospective detection “bounded by construction.”
- Gate: **must fix before slice 1b** because it changes the program contract under which all remaining slices are accepted.

### R2-3: High: Section 7: the claimed independent denominator is still the usage sample

CDB linkage: CDB-3 is only restated; the word “independent” was added without an independent source.

- Claude assistant `requestId` and its usage are fields of the same transcript record. Codex “every token-count event” is the usage event itself; inspected rollouts expose repeated `event_msg.token_count` samples but no request id that can be joined one-to-one to model invocations.
- Derived `calls.jsonl` cannot detect an event absent from its source. The design also names no owner or record shape for complete reset/compaction enumeration.
- Failure scenario: a runtime performs two internal model calls but emits one aggregate token event. The inventory and samples both contain one row, so p95/max and completeness pass falsely.
- Required correction: define an independently enumerable invocation-id stream and a one-to-one join to usage, plus explicit reset/compaction marker sources and missing/duplicate-row failure rules. Otherwise limit the claim to “recorded samples,” not all calls.
- Gate: must fix before slice 2 establishes the schema; it does not independently block slice 1b.

### R2-4: High: Section 4: the accounting owner is named, but the read contract is not designed

CDB linkage: CDB-2 is partly folded-the Claude formula and per-turn exclusions are correct-but “a reader joins the usage owner” is words rather than a mechanism. CDB-7 is only partly folded.

- `spend.Measure` returns aggregate `Ledger.Rows` and `SeatSummary`; it drops request identity and has no newest-call API. `readSeat` enumerates all matching transcript files, materializes and sorts all cached requests, and cache loading unmarshals the whole request map. That may be acceptable because health already calls `Measure`, but the design must specify one shared pass/API and its latency bound rather than invite a second scan on every Stop.
- The current `internal/usage.eventStreamUsageValue` reads a whole file and only recognizes top-level `usage`; it cannot discover or parse Codex’s nested rollout `event_msg.token_count.info.last_token_usage`. The design does not specify rollout-root discovery, session matching, cursoring, truncation/rotation, or unreadable-home behavior.
- Raw Claude transcripts and Codex rollouts are harness formats, while the goal says the mechanism lives in the adapter contract and never depends on a transcript format.
- Failure scenario: `context status` selects another session’s newest rollout, or repeatedly scans a growing rollout and delays every Stop; on Devin it reports unknown while ordinary work continues.
- Required correction: specify a typed per-call sample API owned by one package, session identity and cursor semantics per runtime, bounded incremental cost, and adapter capability/failure behavior.
- Gate: must fix before slice 2; not before isolated slice 1b.

### R2-5: High: Section 2: the slice-1b spill contract has false callers and no stable compatibility contract

CDB linkage: CDB-6 and CDB-7 are partly folded; CDB-8 is folded by separating the landed ledger projection from slice 1b.

- Goal claim and mutations do not print a goal record: `printSyncResult` emits only `outcome`, `tip`, and `detail`; `goal next` is also bounded text. The cited 29KB claim therefore does not justify wrapping those verbs.
- `publishTestingResult` already writes to a caller-supplied path when one is provided; the landing path uses that mode. Changing its no-path JSON from `TestResult` to a reference envelope is a schema break unless the mode and consumers are explicit.
- `land.sh` owns a temporary complete step log, prints all coverage-delta failures but only 40 lines otherwise, and deletes the temp file on EXIT. “Full bytes” is ambiguous, and no reader, retention rule, or landing-evidence link is named.
- `<verb>-<timestamp>` has unspecified precision and no atomic uniqueness. The 32KB limit is unconnected to the token reserve and bounds one emission, not a batch.
- Failure scenario: an automation unmarshalling `TestResult` receives a reference object; two concurrent failures choose one pathname; cleanup removes the only full source; no consumer dereferences the survivor.
- Required correction: inventory measured large producers only; define byte semantics, stdout/JSON compatibility, unique atomic names, durable copy-before-cleanup, reference schema, dereference owner, retention, and tests per caller.
- Gate: **must fix before slice 1b**.

### R2-6: Medium: Section 3: “by path” lacks a transport and verification contract

CDB linkage: the relevant part of CDB-6 remains a mechanism gap.

- `ComposeRolePacket` currently reads brief and continuation paths, embeds their bytes, and records source/delivered digests. Revision 2 does not define the replacement packet field, path root, immutable lifetime, maximum inline directive, recipient-side digest verification, or failure behavior after host staging.
- Failure scenario: composition records digest A, the referenced file changes to bytes B before a remote recipient opens it, and the review uses B while the evidence record still attests A.
- Required correction: define a typed reference-root-relative path, content digest, byte count, purpose-immutable/staged lifetime, verification immediately before model invocation, and fail-closed unreadable/mismatch behavior.
- Gate: **must fix before slice 1b**, because section 9 includes sections 2 and 3 in that slice.

## Round-1 disposition

| Finding | Revision-2 disposition |
|---|---|
| CDB-1 | Only restated; awaits scope change and supplies observation, not construction. |
| CDB-2 | Partly folded; formula/exclusions are right, reader/API/runtime mechanics absent. |
| CDB-3 | Only restated; proposed inventories are not independent of samples. |
| CDB-4 | Not folded; assertion is the opposite of `seatIdle` code and its test. |
| CDB-5 | Only restated; quiescence/disposability are declarations the CLI cannot prove. |
| CDB-6 | Partly folded; surplus mechanisms removed, but key owners/contracts remain missing. |
| CDB-7 | Partly folded; stale line anchors were removed, but caller and steward claims are false. |
| CDB-8 | Folded; ledger projection landed separately and spill/reference work is slice 1b. |
| CDB-9 | Folded; fixtures are explicitly future requirements and no nonexistent test name is cited. |

Words in place of mechanism: CDB-1, CDB-2 in part, CDB-3, CDB-4, CDB-5, and CDB-6 in part.

## Prose and fixture audit

- No bare `job`, `runner`, or `gate`; no U+2014 em dash.
- `turn` remains unqualified in “each turn” and “a turn boundary”; say harness turn, provider invocation, or Stop-hook attempt. `event` is qualified as a Codex token-count/rollout event, but its identity semantics are still undefined in R2-3.
- The nineteen-megabyte fixture is no longer claimed to exist: section 7 labels all fixtures future tense. The landed slice-1 test has a large-history fixture, not a nineteen-megabyte named fixture. For complete clarity, change the elliptical noun phrases to “will add.”
- These prose items are non-material except where the ambiguous turn/event identity feeds R2-1 and R2-3.

## Slice gates

Before slice 1b: resolve the construction-versus-measurement contract (R2-2), finish the spill compatibility/ownership contract (R2-5), and finish the path-reference transport contract (R2-6). Before slice 2: R2-3 and R2-4. Before slice 3: R2-1. Landing an isolated corrected slice 1b would not accept the rest of revision 2.

Proposed receipt, not written: `type=review | outcome=rework-required | skills=design-critique | verify=read-go-shell-role-and-runtime-records | corrections=0 | note=revision 2 still lacks construction, independent proof, accounting, spill, reference, and exclusive-handoff mechanisms`