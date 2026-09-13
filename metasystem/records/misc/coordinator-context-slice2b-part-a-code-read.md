# Independent code read: coordinator-context slice 2b, part A

Reviewer: Opus 5, read-only, not the author of the change.
Tree: `/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/.claude/worktrees/ccb-s2b/metasystem`, base `e63af263`.
Diff read: computed `git diff` (290 added, 16 deleted across 11 files), not the builder's file list.
Spec: `artifacts/reports/codex-ccb-slice2b-brief-v2.md` part A and `artifacts/reports/ccb-8c10-slice2b-amendment.md` (rows CCB-2-42, 43, 44).
Parts B, C and D are deliberately absent and are not reported as findings.

## Verdict

Fit to land after one correction: the receipt row in `metasystem/memory/receipts.log:349` must be removed or
corrected before the seat commits. Every part A obligation in the brief and in amendment rows CCB-2-42,
CCB-2-43 and CCB-2-44 is implemented, and each named test is a genuine red case. No code defect found.

## Findings

| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| F-1 | medium | yes | The returned diff appends a receipt row that records this delegate-built, uncommitted, part-only return as `outcome=shipped|delegate=none|built_by=coordinator`. Three claims in it are untrue: nothing was committed, the builder was Codex, and the ordered verification stopped at step 3 of 5. `built_by` feeds a measured metric, so the row lowers the delegate share for a delegate build, and it will be double-counted against the goal when the seat writes its landing receipt. | `metasystem/memory/receipts.log:349`; the builder's own report lists the row as its change (`artifacts/reports/codex-ccb-slice2b-result.md`, Changed files: "memory/receipts.log: the repository-required uncommitted implementation receipt"); the brief names Codex as builder (`artifacts/reports/codex-ccb-slice2b-brief-v2.md:1`); the metric consumer is `internal/metrics/compute.go:663-686` (`case "coordinator": recorded++` without `delegated++`) |
| F-2 | low | no | CCB-2-43 claims zero transcript bytes and zero cursor bytes "after recovery", but `TestUnchangedEmptyTranscriptDoesNotRepublishCursor` never puts an uncommitted samples suffix or a missing samples log in front of the new empty shortcut, so the recovery half of the row is unproven by its named test. I proved the behavior is correct by probe, so this is a proof gap, not a defect. | `internal/usage/cursor_test.go:48-107`; behavior confirmed by scratch probes (see Checks run) against `internal/usage/cursor.go:45-72` |
| F-3 | low | no | `parseCodexLine` survives with no production caller: the only callers left are three assertions in `internal/usage/calls_test.go:380-389`. The brief allowed keeping it "only if an existing direct caller needs it" and told the builder to remove unused private helpers. It is exactly the composition the production path uses, so the assertions still cover the live functions. | `internal/usage/calls_codex.go:103-105`; callers at `internal/usage/calls_test.go:380`, `:385`, `:389` |
| F-4 | low | no | `TestEveryDeclarationDeclaresContextSample` pins the exact runtime population and fails by `t.Fatalf` on any name outside its four-entry map, which cuts against the package convention stated at `internal/runtimes/runtimes_test.go:89` ("a new runtime must not fail this test"). The brief asked for exactly this four-row table, and a fifth runtime should indeed be forced to declare a value, so this is a deliberate drift refusal, not an error. | `internal/runtimes/runtimes_test.go:46-60` against the convention comment at `internal/runtimes/runtimes_test.go:89` |
| F-5 | low | no | The invalid-declaration loop only asserts that some problem contains the substring "context sample", so the "must be declared" message and the "outside the known values" message are interchangeable; a validator that emitted the wrong one of the two would still pass. | `internal/runtimes/runtimes_test.go:67-79` against `internal/runtimes/runtimes.go:425-431` |
| F-6 | low | no | The decode seam is invoked inside `decodeCallLine` rather than behind an `observeCallDecode` helper like the sibling `observeCallOpen`. Today `decodeCallLine` is reached only from the Codex paths, so the count is Codex-only as the brief required; if a future Claude path reuses it, the counter silently widens. | `internal/usage/calls_codex.go:175-178` versus `internal/usage/calls.go:191-195` (`observeCallOpen`) |
| F-7 | low | no | `TestLatestCallReturnsThePreviousReadTime` covers the `PerInvocation` zero case but not `NoStream`. Both leave through the same early-return block, so the uncovered case cannot differ. | `internal/usage/cursor_test.go:157-162` against `internal/usage/calls.go:95-103` |
| F-8 | info | no | Pre-existing, outside this diff: the receipt row immediately above the new one carries `built_by=codex`, which is outside `ValidBuiltByValue` and is counted as rejected coverage by the same metric. Worth a separate ledger correction, not this chain's work. | `metasystem/memory/receipts.log:348`; `internal/receipt/receipt.go:138-145`; `internal/metrics/compute.go:681-684` |

Material findings: 1.

## Conformance, obligation by obligation

Part A of the brief, every row checked against the computed diff:

- Single decode. The callback decodes once and hands the same `map[string]any` to both helpers
  (`internal/usage/calls.go:132-142`). Both helpers now take the decoded object with the exact signatures the
  brief specified (`internal/usage/calls_codex.go:107`, `:163`). `UseNumber`, the token and identity checks,
  the `token_count` legacy diagnostic and response-id dedup are untouched.
- The seam. `callJSONDecodes func()` sits beside the byte and open seams (`internal/usage/calls.go:86-89`),
  is invoked only from `decodeCallLine`, and is restored with `t.Cleanup` in a non-parallel test
  (`internal/usage/calls_test.go:104-107`).
- `PreviousReadAt`. Added to `Reading` (`internal/usage/calls.go:73-81`), captured from the validated loaded
  cursor after reconciliation and before any reset or advance (`internal/usage/cursor.go:50-53`), and returned
  on both successful stream paths (`:69-71`, `:186-188`).
- The empty shortcut. `cursor.Offset > 0` dropped from the unchanged predicate
  (`internal/usage/cursor.go:70`); the lock, load, reconcile, stat, shortcut ordering is preserved;
  `SamplesBytes`, sync behavior, restart history and the full `Seen` map are unchanged.
- Runtime declaration. `ContextSample` and `MainObservable` added with the four required values
  (`internal/runtimes/runtimes.go:71-75`, `:183`, `:199`, `:240`, `:262`); `Validate()` rejects both an empty
  and an unknown value (`:425-431`); the registry imports nothing from usage.
- The verb. `runRuntimeContextSample` follows the existing `runtimeArg` handling, prints exactly
  `sample=<v> main-observable=<bool>`, exits 1 on an unknown runtime and 2 on usage
  (`cmd/metasystem/runtime_verbs.go:202-210`, `:17-27`), and is registered beside `start-context`
  (`cmd/metasystem/main.go:394`).

Amendment rows: CCB-2-42, CCB-2-43 and CCB-2-44 are each implemented and each has its named test.

Scope: nothing from parts B, C or D leaked. No `internal/steward`, `internal/output`, `internal/usage/sessions.go`,
fixture script, `testing.json`, coverage ratchet, role contract, adapter, spend, handoff or production hook file
appears in the diff. `git status --untracked-files=all` shows no stray files; the build result report is under the
ignored `artifacts/` tree. The only file outside the brief's named surface is `memory/receipts.log` (F-1).

## The five questions, answered with evidence

1. **Is classification still exactly as strict?** Yes. Before the change both helpers called the same
   `decodeCallLine`; now they share its one result. Neither helper writes to the map, and the value readers
   (`textValue`, `callTokenField`, `callTimestamp`, `internal/usage/calls_claude.go:121-195`) are read-only, so a
   shared map cannot be observed differently by the second consumer. A malformed line still decodes to `nil`,
   which yields kind `""`, no `token_count`, no `token_usage_record` count and no sample
   (`internal/usage/calls_codex.go:107-110`, `:163-166`) - it can be neither counted as a sample nor dropped
   without its diagnostic. Duplicate ids, compaction markers, legacy `token_count` rows and irrelevant rows are
   all exercised in the new test, and the legacy diagnostic path is still covered end to end by
   `TestCodexRolloutWithoutUsageRecordsIsUnknown` (`internal/usage/calls_test.go:133-159`).
2. **Does the empty shortcut still reconcile, and can it skip an append or lose a restart?** It reconciles:
   `reconcileCallRows` runs before the predicate and takes the cursor by value, so nothing the shortcut skips was
   owed to it (`internal/usage/cursor.go:45-49`, `:429-459`). I proved both halves by probe (below): an unchanged
   empty read still truncates an uncommitted samples suffix, and a zero-offset cursor with a positive committed
   boundary still refuses when the samples log disappears. The new window is only `Offset == 0 && size == 0` with
   a matching path and file identity; an append makes `size != Offset`, and a path or inode change or a truncation
   from a positive offset still reaches the restart branch. The single behavioral difference in the new window is
   that `LastReadAt` no longer advances, which is the same contract the nonempty shortcut already had and which
   the brief's Part B spill hint explicitly anticipates.
3. **`PreviousReadAt`:** read from the loaded, validated cursor before any reset (`internal/usage/cursor.go:50-53`);
   zero on a first read (`loaded` is false, and an invalid cursor loads as not-loaded,
   `internal/usage/cursor.go:342-368`); zero for `PerInvocation`, `NoStream` and an unknown runtime, which all
   return before `readUnderCursor` (`internal/usage/calls.go:94-122`); and never consulted by the reason logic,
   which reads only `SampleCount` and `SidechainCount` (`internal/usage/cursor.go:521-536`), so it cannot pass as
   evidence of a sample.
4. **The verb:** exit codes and output verified by test and by the builder's rebuilt binary. Nothing else needs to
   change with it: no shell adapter, host script, help golden, docs inventory or audit section enumerates runtime
   family verbs, and `Declaration` is never serialized whole. `runtimes.Validate()` is called only from tests
   inside `internal/runtimes`, so the new rejection cannot fire against the synthetic declarations that
   `internal/validate/conformance_test.go:783` and `internal/audit/metasystem_registry_test.go:17` push through
   `OverrideForTest`. The two existing validator tests match problems by substring, so the extra problem they now
   collect does not weaken their assertions. `All()` copies the table, so the new test's mutation of
   `declarations` cannot leak past its cleanup.
5. **Coverage:** measured on this Mac, `internal/usage` 88.3% against a floor of 85.8 and `internal/runtimes`
   90.7% against a floor of 85.1 (`scripts/agents/coverage-ratchet.json:70,80`; the Linux file carries the same
   two floors). `cmd/metasystem` keeps its thin-wiring exemption, and the new command code is one `Printf`.
   Linux coverage remains seat evidence; the changed code is platform-neutral.

## Checks run (read-only; nothing in the repository was modified)

- `go test -count=1 -cover ./internal/usage/ ./internal/runtimes/` gives 88.3% and 90.7%.
- Mutation via `go test -overlay` against scratch copies, to prove the named tests are red without the change:
  restoring the two-decode callback gives `complete-line decodes = 12, want 6`; restoring `cursor.Offset > 0`
  gives `unchanged read used bytes=0 transcript-opens=1 cursor-writes=1` in the `empty` subtest.
- Three scratch probes added to the package through `go test -overlay` (no repository file touched), all passing:
  an unchanged empty read truncates an uncommitted samples suffix before taking the shortcut; a restart onto an
  empty file leaves `Offset 0` with a positive `SamplesBytes` and still refuses with "missing below committed
  boundary" when the samples log is deleted; and `PreviousReadAt` plus the cursor's `LastReadAt` both stay frozen
  across three repeated unchanged reads.
- `gofmt -l` clean on all ten changed Go files.
- I did not repeat the seat's build, vet, race or fast-gate runs, and I ran no fixture bed.

## What must change before landing

Remove the `memory/receipts.log` row, or correct it so the builder, the delegate and the outcome are true, and let
the seat write the landing receipt when the whole slice lands. Nothing in the code needs to change.
