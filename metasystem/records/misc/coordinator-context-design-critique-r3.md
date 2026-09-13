# Codex gpt-5.6-sol read of plans/coordinator-context-stays-under-budget-design.md revision 3 (2026-09-13, plugin task task-mtzl8tr3-h6o14l, read-only)

# Verdict

**Rework required. Slice 1b is not ready to build.** Its reference-verification boundary is wrong, spill compatibility is undefined, and the inline-size contracts conflict. Revision 3 materially closes R2-2 and the exclusivity core of R2-1; R2-4 through R2-6 remain incomplete.

Evidence: full design and round-2 review, cited implementation paths, and structural inspection of this machine’s Claude/Codex records. No files were modified.

| Round-2 finding | Revision 3 disposition |
|---|---|
| R2-1 nonce/exclusivity | Closed for cross-read and concurrent-seat overlap; new pending-intent lifecycle gaps remain. |
| R2-2 construction contract | Closed under Wido’s stated 2026-09-13 interpretation. |
| R2-3 cohort/markers | Recorded-cohort claim is closed; marker and Codex sample identity are not implementation-grade. |
| R2-4 typed usage API | Not closed: wrong owner, wrong Codex key, incomplete cursor contract. |
| R2-5 spill compatibility | Not closed: the alleged reader does not exist and the envelope cannot be detected as specified. |
| R2-6 typed references | Not closed: verification is assigned to the wrong adapter layer and may verify a different path from the one the model reads. |

## Material findings

### 1. Critical — §3 verifies at the wrong execution boundary

**Scenario:** Delegate packets are launched through `scripts/agents/adapters/<runtime>.sh`, as [dispatch.sh](</Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/scripts/agents/dispatch.sh>) and the orchestration record specify. Revision 3 instead puts `job verify-references --job` in `scripts/agents/hosts/*.sh`; those mission hosts receive no job ID and do not launch ordinary composed delegates. Moreover, a control-root-relative path may be verified in the control checkout while Codex or Devin, running in the job worktree, opens the same relative spelling and gets a different file or no file.

**Material:** Yes. It changes `CompositionReference`, the job record, and the runtime adapter pre-invocation hook. R2-6 is currently renamed, not closed.

**Amendment:** Carry a canonical recipient-visible `openPath` plus its root identity. Verify that exact resolved path, after any remote staging, in `scripts/agents/adapters/runtime-common.sh`, and inject that same path into the model packet. Give mission-runner hosts a separate turn-reference contract if they also need references. Test two different files at the same relative path and prove the model-visible one is the verified one.

### 2. High — §§4 and 7 do not identify one Codex provider call

**Scenario:** On this repository’s local records, 44 Codex rollouts contain 1,914 non-null `event_msg.token_count.info.last_token_usage` values but 1,877 `token_usage_record` entries with stable `payload.response_id` values. Ordinal token-count sampling therefore adds duplicate observations and changes the percentile and maximum. The plan’s “no request id” premise is false for the available schema. It also assigns extraction to `internal/spend`, contradicting [the documented single typed extraction owner](</Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/usage/usage.go:1>).

Actual marker records are Claude `type=system, subtype=compact_boundary` and Codex top-level `type=compacted`; “reset” still has no selector.

**Material:** Yes. It changes `CallSample`, the Codex parser, deduplication key, marker parser, cohort, and report.

**Amendment:** Put typed extraction and cursors in `internal/usage`; let `internal/spend` consume it for pricing. Key Codex samples by `token_usage_record.payload.response_id`, specify exact marker predicates, and either define a concrete reset record or make unsupported reset detection fail that runtime.

### 3. High — §4’s cursor cannot reliably implement `LatestCall`

**Scenario:** After one process advances the byte offset, a later `LatestCall` invocation against an unchanged file reads no bytes and has no persisted last sample to return. Two simultaneous Stop hooks can also read the same offset and overwrite the cursor out of order, duplicating or losing calls and compaction markers.

**Material:** Yes. It changes `cursor.json`, synchronization, and the API contract.

**Amendment:** Persist the latest sample and marker state, keyed by runtime/session/source identity. Hold a per-seat lock across read, parse, and atomic cursor replacement—or use a generation CAS. Add unchanged-file and concurrent-reader tests.

### 4. High — §2’s spill envelope is not backward-compatible

**Scenario:** No-path `publishTestingResult` currently emits a `proofrun.TestResult`. Unmarshalling the proposed envelope into that Go struct silently ignores unknown fields and yields a zero-valued result; such a caller cannot then “find `outputMode`.” The named `scripts/agents/test-result-summary.sh` reader does not exist. The `land.sh` producer inventory is otherwise accurate: its temporary combined log is removed during cleanup and failures currently print either the whole coverage-delta output or 40 lines.

**Material:** Yes. It changes the public stdout schema, `TestResult` consumption, and slice 1b’s required consumer artifacts. R2-5 is not closed.

**Amendment:** Define a discriminated `TestOutput` union or explicit resolver command, enumerate real consumers, and add an old-shape consumer fixture. Either include the Stop-hook dereferencer in slice 1b or remove it from slice 1b’s completeness claim.

### 5. High — §§5 and 9 lack a pending-handoff lifecycle

**Scenario:** An interactive predecessor may never exit. Current steward code retries live resumable intents on later ticks, while [CompleteRevival](</Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/internal/steward/revive.go:115>) cancels any decision that is not revival. Revision 3 requires a third “remain pending” result but does not specify it. Repeated handoffs can create multiple live intents, and continued predecessor work can make the captured state stale before death.

**Material:** Yes. It changes `Intent`, `CompleteRevival`, tick selection, and engine mutation rules.

**Amendment:** Specify `pending-predecessor → launched | cancelled | superseded`; make repeated requests idempotent or atomically superseding; provide explicit cancellation; rate-limit pending notices; and reject engine-owned mutations while pending or require a refreshed handoff.

### 6. Medium — §§2–3 define two incompatible inline limits

**Scenario:** Revision 3 uses `internal/output.MaxInlineBytes = 32 KiB` for directive text, while dispatch already enforces configurable `dispatch.max-inline-input-kb = 64` for the complete packet. An implementer must decide whether a 40 KiB directive is referenced, accepted, or rejects the packet.

**Material:** Yes. It changes constants, packet composition, and acceptance tests.

**Amendment:** Define separate `MaxDirectiveBytes` and `MaxPacketBytes`, state whether each is configured or fixed, and test their interaction.

### 7. Medium — §5’s nonce state has no retention owner

**Scenario:** Only spill output has a pruning rule. Every attempted, abandoned, or superseded handoff permanently leaves `artifacts/agents/context/<seat>/<nonce>/state.json`, allowing the artifact store to grow without bound.

**Material:** Yes. It changes the context artifact lifecycle and pruning command.

**Amendment:** Add context-state retention: never prune a nonce referenced by a live intent; prune closed/cancelled states after an explicit evidence-retention period. Test both cases.

### 8. Medium — §8 fails the repository’s design-obligation gate

**Scenario:** This design introduces owners, boundaries, invariants, persistent state, and model-facing contracts, but has no full obligation matrix with severity, owner, code target, focused test, and disposition. Several ambiguities above consequently appear “assigned” without being buildable.

**Material:** Yes. It changes the design artifact and slice entry gates.

**Amendment:** Add the required matrix and prohibit starting a slice while any critical/high row lacks an owner, concrete target, and focused test.

The remaining textual inconsistency—§19’s 29 KiB `goal claim` observation versus §31’s correction—is non-material because the later inventory and current code are clear.
