# coordinator-context-stays-under-budget: design critique, round 1 (Codex gpt-5.6-sol, 2026-09-12)

Read of plans/coordinator-context-stays-under-budget-design.md (revision 1, the seat's page from a Codex draft). Verdict: rework before build; nine findings. Dispositions are on the page's critique record; revision 2 folds them.

# Coordinator context design critique

Verdict: reject before build. Eight material findings would make an implementer build the wrong enforcement, handoff, or proof boundary. Evidence level: read from this checkout and one current Codex session record; no tests were run.

## CDB-1: The promised construction boundary does not own coordinator calls or native tool results

- Design: sections 2, 3, 4, and 6, lines 25, 39, 57, 78, 80-82, and 109-111.
- Priority: **must-fix-before-build**.
- The design admits that native reads may bypass the proposed Go output boundary and says only to include them in later accounting (line 57). A direct `sed`, `grep`, native Read, attachment, or batched tool result can therefore enter the next prompt without first becoming a file-plus-summary, contrary to DONE. No `internal/output/bounded.go` exists in this checkout.
- The same ownership hole defeats the token ceiling. The hook obtains health only on a Stop invocation (`metasystem/scripts/agents/supervision-hook.sh`, Stop branch, lines 810-843) and asks for the structured verdict only at turn end (lines 1100-1110). The current usage owner reads completed records post hoc and selects the last top-level `usage` block (`metasystem/internal/usage/usage.go`, `eventStreamUsageValue`, lines 171-192). Neither is in front of an interactive coordinator model call.
- The Codex adapter does own the CLI call for dispatched work (`metasystem/scripts/agents/adapters/codex.sh`, `supervise`, lines 139-177), but the page never names an equivalent adapter-owned launch path for the interactive coordinator. “Run the status/admission verb yourself” is cooperative instruction, not “bounded by construction,” and records plus a verb alone cannot prevent a skipped or hidden call.
- Failure scenario: a coordinator at 149,000 tokens receives 60,000 tokens through native tool results and the harness starts its next model call before Stop. The health line observes the breach afterward; the proposed Go policy never admitted that call or spilled those results.
- Required correction: either put an engine-owned boundary in front of every hosted coordinator tool delivery and model call, with a named adapter method for each runtime, or narrow the goal. Specify that `unknown` blocks every ordinary call at that actual boundary; without interception, `unknown never permits an unbounded call` is unimplementable.

## CDB-2: The accounting contract conflates usable per-call sources with per-turn aggregates

- Design: section 4, lines 61-72 and CTX-3 at line 129.
- Priority: **must-fix-before-build**.
- Claude can supply the requested figure: for each deduplicated assistant request, prompt context is `input_tokens + cache_creation_input_tokens + cache_read_input_tokens`. Existing code already keys assistant records by `requestId` (`metasystem/internal/spend/transcript.go`, `applyTranscriptLine`, lines 304-340) and normalizes creation into input while retaining cache-read separately (`transcriptTokens`, lines 442-479). The context reader must sum those normalized components once. The design would create a second transcript parser in `internal/usage` without saying whether this existing owner moves or is reused.
- Codex’s own session stream has the equivalent direct value in `event_msg.token_count.info.last_token_usage.input_tokens`; cached input is a subset, not an addend. The current record at `/Users/wido/.codex/sessions/2026/09/12/rollout-2026-09-12T20-15-20-01a096d4-c05f-7ab3-b57f-6c9ba939eb63.jsonl:142` reports input 112,601, cached input 110,848, output 218, and total 112,819, confirming `total = input + output`. If only `total_token_usage` is available, consecutive deltas work only after proving exactly one model call and no missing record between counters. Current `internal/usage/eventStreamUsageValue` cannot read this nested `token_count` shape.
- Devin is not call-resolved: `DevinUsage` explicitly subtracts cumulative session totals per adapter turn (`metasystem/internal/usage/devin.go`, lines 24-31 and 112-126). ACP is also declared per turn (`metasystem/internal/usage/acp.go`, lines 9-16 and `ACPUsage`, lines 27-65). The named code cannot verify the page’s claim that either turn contains a particular number of model calls.
- On a runtime with no per-call figure, prevention may use a conservative prospective bound only at a real invocation boundary, but no percentile or maximum sample may be fabricated. Without that boundary, the only honest action is `unknown`, stop ordinary work, and fail the runtime’s proof cohort.

## CDB-3: The seven-day proof has no independent denominator, so missing and hidden calls are undetectable

- Design: section 7, lines 117-121 and rejection at line 152.
- Priority: **must-fix-before-build**.
- p95 and maximum are computable from complete call rows, and Claude/Codex can provide such rows as above. They are not computable for Devin/ACP until those paths expose individual calls. The proposed `calls.jsonl` does not fix that: it is populated by the same adapter records and reconciled against an “adapter call inventory,” so a call hidden from the adapter is absent from both sides.
- The current typed usage records contain token fields but no call identity, compaction/reset marker, or complete coordinator-call inventory (`metasystem/internal/usage/usage.go`, lines 171-192; `devin.go`, lines 118-126; `acp.go`, lines 61-69). The page does not name an independent provider/harness source for the denominator or for reset/compaction completeness.
- Failure scenario: a runtime makes two internal model calls but publishes one aggregate completion. The collector writes one plausible row; p95 and max pass, and “missing calls fail proof” never fires because the missing call has no independently known identifier.
- The rejection phrase “if an admitted call can cross 200,000 unseen” is therefore not falsifiable. Require an independently enumerable call-id stream joined one-to-one to samples and reset markers, or state that a runtime without it cannot enter the seven-day proof. Treat `calls.jsonl` as a derived index, not new source authority.

## CDB-4: The handoff protocol allows two live coordinators and does not revoke the predecessor

- Design: section 5, lines 95 and 99-105.
- Priority: **must-fix-before-build**.
- Existing `seatIdle` deliberately bypasses live-main suppression (`metasystem/internal/steward/revive.go`, `decideForRevival`, lines 253-289), and its test proves a continuation launches while the old main is alive (`metasystem/internal/steward/revive_test.go`, `TestSeatIdleIntentLaunchesAlthoughTheSeatMainIsAlive`, lines 264-288). The existing one-active guard covers continuation intents, not the predecessor (`metasystem/internal/steward/intervene.go`, `ConsumedActiveJob`, lines 218-265).
- The successor’s current role would in fact yield when it sees a live worker (`metasystem/scripts/agents/roles/steward-continuation.md`, lines 15-17). The proposed “release” and acknowledgement have no specified lease/epoch checked before either coordinator’s next model call or side effect. A stale predecessor can resume after release, so both can be active even if only one successor intent exists.
- Require either predecessor exit before launch, or a single session-ownership epoch/lease that both model-call admission and all mutating verbs verify. A transfer id and acknowledgement alone are records of overlap, not mutual exclusion.

## CDB-5: The state file is not a complete boundary contract

- Design: section 5, lines 86-103.
- Priority: **must-fix-before-build**.
- The existing transport is sufficient: `StageIntent` creates and digests a fresh brief (`metasystem/internal/steward/stage.go`, lines 36-95), and `CompleteRevival` consumes the intent once and launches it (`metasystem/internal/steward/revive.go`, lines 138-174). A new transport is unnecessary.
- The proposed state covers durable work metadata, but not a pending tool-call/result pair, a result delivered only inside the harness, queued-but-unpublished messages, provider tool-call ids, or other harness-internal state. `messagesOwed` and `scratchPointers` preserve only items the seat has already materialized, while line 95 merely asserts that the file is refreshed as work changes.
- Failure scenario: the boundary is detected after a tool finishes but before its result is persisted; the successor sees an open item but neither the result bytes nor whether retrying the tool is safe. Alternatively, the predecessor has an unsent authorized message only in harness memory, so `messagesOwed` is stale.
- Define a quiescent handoff point: no in-flight provider/tool operation; every required result and authorized message durably materialized with retry/idempotency state; then validate and launch. Explicitly declare harness internals disposable. The new state file need not and cannot recreate them.

## CDB-6: The design adds machinery not required by DONE and misses the smaller ownership decisions

- Design: decisions at line 6; sections 4-7, lines 74-82 and 95-119.
- Priority: **must-fix-before-build** because it changes interfaces and scope.
- Strictly required by the goal’s DONE sentence (`metasystem/plans/goals/coordinator-context-stays-under-budget.md:8`): (1) bounded tool/verb output with full bytes in a file plus summary; (2) briefs/evidence passed by path; (3) per-call prompt accounting and the 150,000/200,000 bounds; (4) fresh self-handoff through written state, preserving required evidence paths.
- Not strictly required: a health line on every response; prospective admission with exact prompt/protocol-overhead modelling before every call; a second transfer id plus release/acknowledgement protocol; and an append-only `calls.jsonl` authority. Existing intents already have a nonce, one-shot consumption, and a launch stamp (`metasystem/internal/steward/intervene.go`, `Intent`, lines 25-59; `ConsumeIntent` and `StampLaunch`, lines 150-215).
- Devin’s seam is required because it is rostered (`metasystem/metasystem.conf:5`). ACP is not a fourth runtime requirement; it is conditionally needed because this checkout currently routes Devin through ACP (`metasystem/metasystem.conf:115`). Future ACP seams beyond that hosted path are overreach.
- Missing decisions are the owners identified in CDB-1 through CDB-5: interception of native tool/model surfaces, an independent call/reset inventory, stale-state prevention, quiescence, and predecessor exclusion. Also prove that every possible input increment between the 150,000 bound and the next call is below the 50,000 recovery reserve; a 32 KB per-command byte cap alone does not bound a batch, attachments, or model output.

## CDB-7: Several code citations are stale or name the wrong owner

- Design: sections 2-5, lines 27, 43, 45, 49, 67-70, 78, 97, 99, and 105; self-grade line 150.
- Priority: **must-fix-before-build** because implementers would edit the wrong lines/package.
- `goal.go:294` is the closing brace after flag parsing. `runGoalList` defines flags at `metasystem/cmd/metasystem/goal.go:274-280`; `listSynced` is at lines 374-455 and emits JSON at 449-454; `runGoalShow` starts at 460, parses flags at 461-466, and emits the full goal at 496-498. Line 457 is blank.
- `publishTestingResult` starts at `metasystem/cmd/metasystem/test.go:1491` but prints at 1495; `printTestingSummary` prints at 1500-1504, not 1499. `metasystem/internal/proofrun/launcher.go:137` is accurate.
- `land.sh:263` assigns `step_name`; step output is at `metasystem/scripts/agents/land.sh:267-272`. Coverage output is printed at 282-286, the proof tail at 538, and the push-retry tail at 1107. The page’s lines 263 and 281 do not perform the claimed printing.
- Composition’s brief citation is accurate at `metasystem/internal/dispatch/composition.go:231`; continuation bodies are read and appended at 258-274, while 275-279 compute a digest and marshal. The exact role anchors are `metasystem/scripts/agents/roles/design-critic.md:14`, `code-critic.md:14`, and `implementer.md:11`; the page’s shortened `roles/implementer.md` is not an installation-root path.
- Usage anchors `metasystem/internal/usage/claude.go:15`, `usage.go:177`, and `codex.go:14` are accurate for the current last-block parser. `supervision-hook.sh:839`, `internal/steward/health.go:265`, `internal/steward/revive.go:256`, and `supervision-hook.sh:1100` are accurate contextual anchors. `internal/steward/stage.go:23` only constructs the brief path; `StageIntent` writes it at 59-75.
- `internal/report` is not the structured verdict owner claimed at line 105. `runReportTurnVerdict` scans through `internal/report` but calls `goal.Store.TurnVerdict` (`metasystem/cmd/metasystem/goal.go`, lines 652-704); the decision and precedence live in `metasystem/internal/goal/turnverdict.go`, `TurnVerdict`, lines 218-340 and `decide`, lines 822-900.
- The claims that Devin/ACP turns may contain several model calls cannot be verified from the named files; those files establish per-turn semantics only. The page should label that as an adapter/runtime fact still to prove.

## CDB-8: Slice 1 is valuable but not independently landable at its stated scope

- Design: status lines 4-9 and section 8, lines 136-140.
- Priority: **must-fix-before-build**.
- Bounding `goal list/show` is independently valuable: current `listSynced` emits complete goal objects in overlapping live/state collections (`metasystem/cmd/metasystem/goal.go`, lines 387-455), so the known 19 MB case can be removed without solving session accumulation.
- But “slice 1” alternates between only `goal list/show` (line 5) and a universal output owner covering every verb, watch/test streams, landing shell failures, composition, templates, and every consumer (lines 35-45 and 138). `land.sh` currently owns a temporary combined step file and deletes it on exit (`metasystem/scripts/agents/land.sh`, `cleanup` and `run_step`, lines 242-289), so a Go owner cannot retain it without a designed plumbing contract.
- Split the first landing into the measured ledger projections plus spill/reference contract and their callers. Land accounting/inventory before expanding the boundary to every runtime and long-lived command, so later rollout can measure impact and detect missed surfaces; then add handoff after the signal and exclusivity contracts are proven.

## CDB-9: One fixture sentence is false; the requested prose-hygiene scan is otherwise clean

- Design: section 7, lines 123-134.
- Priority: **lower priority**.
- Line 123 correctly says the table names future requirements, but line 134 switches to the present tense: “CTX-1 also tests.” `metasystem/cmd/metasystem/goal_output_test.go` and `TestGoalListNineteenMBSpills` do not exist. The other proposed context test files/scripts at lines 129-132 are also absent; `metasystem/internal/dispatch/composition_test.go` exists, but not the proposed path-only fixture. Keep all of these consistently future-tense.
- Exact-word scan result: there are no bare, unqualified occurrences of `job`, `runner`, `turn`, `gate`, or `event` in the design page. There are no U+2014 em dash characters.

Proposed receipt, not written: `type=review | outcome=rework-required | skills=design-critique | verify=read-go-shell-role-and-runtime-records | corrections=0 | note=coordinator context design has eight material boundary, accounting, proof, handoff, citation, and slicing findings`