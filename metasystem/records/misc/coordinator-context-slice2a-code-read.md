# Independent code read: coordinator-context slice 2a (reader + fold)

Reader: Opus 5, 2026-09-13. Tree: `/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/.claude/worktrees/ccb-s2a/metasystem` at `b9fcccb0` plus six untracked files under `internal/usage/`.

Spec read in order: design section 8c (D2-1..D2-5, rows CCB-2-01..10) and the amendment section 8 (D2-10..D2-13, rows CCB-2-19..27). Briefs: `artifacts/reports/codex-ccb-slice2-brief.md`, `artifacts/reports/codex-ccb-slice2a-fold-brief.md`. Builder report: `artifacts/reports/codex-ccb-slice2-result.md`.

## Verdict

**Fit to land**, subject to the seat's own build/vet/race/full-gate run completing green. Zero material findings. Ten non-material findings are recorded below; three of them (F-3, F-5, F-7) are true statements about the amended design rather than about this diff and should be routed to the design page and to slices 2b/3 rather than fixed here.

## Computed diff

`git status --porcelain -uall` from the worktree root shows exactly six untracked files and no tracked modification at all:

```
?? metasystem/internal/usage/calls.go
?? metasystem/internal/usage/calls_claude.go
?? metasystem/internal/usage/calls_codex.go
?? metasystem/internal/usage/calls_test.go
?? metasystem/internal/usage/cursor.go
?? metasystem/internal/usage/cursor_test.go
```

`git diff HEAD --stat` is empty. `testing.json`, `scripts/agents/coverage-ratchet.json` and `scripts/agents/coverage-ratchet-linux.json` are tracked and unmodified. `plans/coordinator-context-stays-under-budget-design.md` is tracked and unmodified. `artifacts/` is gitignored (`metasystem/.gitignore:1`), which is why the appended result file does not show as a change; it was in fact appended (`artifacts/reports/codex-ccb-slice2-result.md:95` opens the fold record).

File mtimes corroborate the builder's fold file list: `calls_claude.go` is 15:53 (pre-fold, unedited as the fold brief requires); `calls.go`, `cursor.go`, `calls_codex.go` are 17:39 and the two test files 17:36.

Task item 8 answered: nothing outside `internal/usage/` was touched, and neither `testing.json` nor either ratchet baseline was edited.

## Layer 1: conformance, row by row

Every named test exists with signature `func TestName(t *testing.T)` and asserts what its row claims. Spot summary:

| Row | Named test | Asserts the row's claim |
| --- | --- | --- |
| CCB-2-01 | `TestLatestCallReadsAClaudeTranscriptToTheFigure` | yes: `PromptTokens == 50000`, `NewSamples == 2`, `SampleCount == 2`, two sample rows through `Calls`, empty `Reason`, sidechain counted in `Source` (calls_test.go:35-53) |
| CCB-2-02 | `TestLatestCallKeysCodexSamplesByResponseID` | yes: five `token_count` rows plus three `token_usage_record` rows, two samples, `R2`/27866/27392 (calls_test.go:76-84) |
| CCB-2-03 | `TestCodexRolloutWithoutUsageRecordsIsUnknown` | yes: exact reason string, nil Latest, cursor written with `SampleCount == 0` (calls_test.go:100-112) |
| CCB-2-04 | `TestMarkersCountCompactionsPerRuntime` | yes: one marker per runtime, `Detail` `trigger=auto preTokens=967668`, marker rows in both samples files (calls_test.go:136-150) |
| CCB-2-05 | `TestUnchangedTranscriptReadsNothingAndReturnsTheLatest` | yes: byte-count seam reads zero, Latest identical, `NewSamples`/`NewMarkers` zero (cursor_test.go:35-45) |
| CCB-2-06 | `TestRotatedTranscriptRestartsTheCursor` | yes: both `restarted: inode changed` and `restarted: truncated`, offset equal to the new size (cursor_test.go:75-103) |
| CCB-2-07 | `TestConcurrentReadersCountEachSampleOnce` | yes: eight readers, 200 writer records, 200 samples through `Calls`, `SampleCount == 200`, offset equal to final size (cursor_test.go:168-181) |
| CCB-2-08 | `TestPerInvocationRuntimeAnswersUnknown` | yes: exact reasons, `callFileOpens` records nothing, no cursor directory created (calls_test.go:171-180) |
| CCB-2-09 | `TestMissingTranscriptAnswersUnknownWithThePath` | yes: reason names both candidate paths verbatim; unreadable home leg; no cursor written in either case (calls_test.go:200-231) |
| CCB-2-10 | `TestCallsFiltersBySinceAndRegistersSessionsOnce` | yes: `since` returns two of three in file order; registry idempotent on the four-tuple, second tuple adds a row (calls_test.go:248-268) |
| CCB-2-19 | `TestClaudePhysicalLineIdentitySurvivesIncrementalReads` | yes, and in both the complete-append and split-tail subcases: `Line == 5` after the ignored prefix, `line:6` with `Ordinal 6`, marker at ordinal 7, incremental bytes exactly the appended text, unchanged read zero bytes (cursor_test.go:371-405) |
| CCB-2-20 | `TestCursorSchemaRequiresExplicitProgressFields` | yes: schema-1 rebuild to `Line 3`/`SidechainCount 1`/`line:3` without reusing the untrusted offset; eleven-case table over missing, null, negative and impossible members; Codex provider ordinal 9001 reused without a prefix reread (cursor_test.go:433-509) |
| CCB-2-21 | `TestCursorWriteFailureRetriesWithoutDuplicateRows` | yes: initial-checkpoint failure appends nothing and writes no cursor; first-read and existing-checkpoint cases assert the old cursor's bytes are unchanged, a suffix is left, and the retry yields exactly one row per sample and marker by raw JSONL count (cursor_test.go:517-613) |
| CCB-2-22 | `TestReadersRecoverUncommittedSampleSuffix` | yes: a valid sample row, a marker row and an unfinished fragment are appended out of band; both `Calls` and the unchanged `LatestCall` remove them, expose only the committed row, read zero transcript bytes, and leave cursor and samples bytes byte-identical (cursor_test.go:636-662) |
| CCB-2-23 | `TestPartialSampleAppendRetriesFromCommittedBoundary` | yes: half an encoded row appended directly; `LatestCall` called first; committed prefix byte-identical, exactly two raw lines, every raw line parsed independently of `Calls` (cursor_test.go:688-705) |
| CCB-2-24 | `TestUntrustedOrShortSampleLogIsPreserved` | yes: eight mutations including a directory at the cursor path and a non-regular samples path; both APIs error; full path snapshots asserted unchanged after each call; the four boundary cases assert the error names both paths and the phrase `committed boundary` (cursor_test.go:783-808) |
| CCB-2-25 | `TestTranscriptRestartKeepsCommittedSampleHistory` | yes, for all three causes: injected publication failure, old cursor bytes unchanged, suffix left, retry gives `Line 1`, `SidechainCount 0`, `line:1`, the exact restart reason, no stale skip suffix, old row plus exactly one new row through `Calls`, and `SamplesBytes` grown past the prior committed length (cursor_test.go:867-897) |
| CCB-2-26 | `TestSidechainOnlyProgressSurvivesBeforeFirstSample` | yes: nil Latest with count 2 and the exact reason; two unchanged reads keep it at zero transcript bytes; later samples carry `2` then `3`; historical sample bytes unchanged; final `Calls` shows exactly two samples with `2` and `3` (cursor_test.go:916-971) |
| CCB-2-27 | `TestCodexRolloutTreatsHomeAsLiteralPath` | yes: six metacharacter homes each with a misleading matching-session sibling, zero matches, two matches, literal metacharacters in the session, transcript override, accepted directory symlink, ignored leaf symlink, ignored wrong-depth files, and an unreadable branch asserting the path-bearing reason (calls_test.go:438-578) |

Signature and helper contract, all satisfied: `loadCallCursor(path, runtime, session) (CursorState, bool, error)` (cursor.go:334), `validCallCursor(cursor, runtime, session) bool` (cursor.go:380), `appendCallRows(path, rows, committedBytes) (int64, error)` (cursor.go:466), `reconcileCallRows(path, cursor, loaded) error` (cursor.go:421), `var writeCallCursor = atomicWriteJSON` (cursor.go:32), `readingFromCursor(capability, cursor, newSamples, newMarkers)` (cursor.go:513). `latestCallOrdinal` and `sourceSidechainCount` are gone (grep over the package returns nothing). `withoutSidechainCount` retained (cursor.go:533). `LatestCall`, `Calls`, `readUnderCursor`, `freshCallCursor`, `codexRollout`, `lineParser`, both parser signatures and `RegisterSession` are unchanged. No `t.Parallel` anywhere in the package, as required by the two global-swapping tests; both restore through `t.Cleanup`.

Validation rules of the amendment are all present: nonnegative `Line`/`SidechainCount`/`SamplesBytes`, `SidechainCount <= Line`, `Line <= Offset - len(Tail)` (cursor.go:387, 407); Claude sample and marker ordinals in `[1, Line]` with Codex exempt (cursor.go:410-417); required literal JSON members `line`, `sidechainCount`, `samplesBytes`, rejecting absent and null (cursor.go:347-351, 362-378); source-path equality removed from validation and moved into `readUnderCursor` after reconciliation (cursor.go:66, 72).

D2-14 and rows CCB-2-28..32 are correctly absent: no dispatch or composition file appears in the diff, and `InlineInputLimitBytes` does not exist in the tree.

## Layer 2: adversarial critique

### Committed-bytes protocol of D2-11, traced end to end

I traced every case the task named and found no path that loses committed evidence, double-counts within a generation, or returns uncommitted rows as committed.

- **Failed cursor publication.** `appendCallRows` returns the new boundary only after write, short-write check, `Sync` and `Close` all succeed (cursor.go:496-510). The candidate cursor is published after that (cursor.go:177). A failure returns an error with the old cursor still on disk, so the next locked call reconciles the suffix away and re-reads from the old transcript offset. On a fresh store the empty schema-2 checkpoint (cursor.go:164-171) makes that old boundary authoritative at zero; without it the first failure would be unrecoverable. Verified for both the fresh and the existing-checkpoint case, and for restart generations.
- **Short append / torn row.** `written != encodedRows.Len()` becomes `io.ErrShortWrite` and the boundary does not move (cursor.go:497-506). The partial bytes are an uncommitted suffix that `reconcileCallRows` truncates on the next call. `appendCallRows` also refuses to append unless the file's current length equals the committed boundary (cursor.go:492-495), so a retry can never write a row that starts mid-fragment.
- **Missing samples file.** Error when `loaded && SamplesBytes > 0`; permitted as size zero otherwise (cursor.go:422-427).
- **Shorter than the recorded boundary.** Error, both files preserved, nothing written (cursor.go:441-443).
- **Untrusted cursor beside a nonempty log.** Error (cursor.go:435-439), wrapped by both callers with both paths (cursor.go:48, 200).
- **Rotation.** `SamplesBytes` is captured before the restart and carried into the fresh state in all three causes (cursor.go:71-84), so history survives while `Line`, `SidechainCount`, `Seen` and the counters reset.
- **Does `Calls` recover before filtering?** Yes. `reconcileCallRows` at cursor.go:199 precedes the `SamplesBytes == 0` early return at :202 and the scan at :218, and the scan is bounded by `io.LimitReader(file, cursor.SamplesBytes)`, so only the committed prefix is ever parsed.
- **Does the unchanged-transcript shortcut bypass recovery?** No. Reconciliation at cursor.go:47 precedes the shortcut at cursor.go:66.
- **Durability outcome.** `atomicWriteJSON` (usage.go:122-128) discards `durable` and returns only `writeErr`, so `atomicfile.publish`'s post-rename `(false, nil)` reaches the reader as committed, exactly as D2-11 point 5 requires, and nothing rolls back.

### Locking

Every read, recovery, append and publication in `readUnderCursor` and `Calls` happens between `lockCallFile` and the deferred unlock (cursor.go:35-39 and 187-191). `RegisterSession` holds its own registry lock across the duplicate scan and the append (cursor.go:252-256). The lock file is a sibling of the cursor, so `atomicfile`'s rename of the cursor never changes the lock's inode; `unix.Flock(LOCK_EX)` therefore serialises two processes on the same `(stateRoot, runtime, session)` correctly, including through a symlinked state root, because flock is per inode. No API in this package re-enters another under its own lock, so there is no self-deadlock. I found no interleaving hole.

### Physical line count and `line:<ordinal>`

`cursor.Line++` precedes the parser call and only fires for a newline-terminated read (cursor.go:112-114), so blank, invalid, non-assistant, duplicate and sidechain lines all advance it, and an unfinished tail advances it exactly once when its newline arrives on a later read. The tail is re-fed from the cursor through `io.MultiReader` (cursor.go:99) and is already inside `Offset`, so no transcript byte is read twice and the byte-count seam stays exact. `parseClaudeLine` uses the supplied ordinal for both `InvocationID` fallback and `Ordinal` (calls_claude.go:95-111). The invariant `Line <= Offset - len(Tail)` cannot underflow because the preceding clause rejects `len(Tail) > Offset` and `||` short-circuits.

Checked against live data: all 6,569 assistant records in this seat's own 47.8 MB transcript carry both `requestId` and `timestamp`, so the `line:<ordinal>` fallback is a genuine edge path rather than the common one, and the zero-time `At` fallback does not silently drop real samples out of `Calls`' `since` window.

### Sidechain count

`readingFromCursor` derives the reason from persisted `SidechainCount` alone (cursor.go:521-527), so `unknown (only sidechain records so far)` survives unchanged reads. The batch fix-up at cursor.go:145-153 stamps the cumulative count on every new sample of the read and on `cursor.Latest`, and the else-branch at :154 updates only `Latest.Source` on a sidechain-only read without touching historical rows. `withoutSidechainCount` uses `LastIndex`, so the suffix is replaced rather than appended, and it composes correctly after a `; restarted: <cause>` suffix.

### D2-13 literal traversal

Depth is exactly right: `root -> yyyy -> mm -> dd -> file`, i.e. `codexRolloutDirectories` applied three times (calls_codex.go:23, 28, 33) and then `os.ReadDir` of the day directory (:38). `os.Stat` recognises directory symlinks (:85); `os.Lstat` plus `IsRegular` rejects leaf symlinks (:51-60); nothing is ever passed to a pattern matcher, so `[`, `]`, `*`, `?` and `\` in home, entry names or session are literal. A vanished directory is skipped (`os.IsNotExist` at :39, :52, :76, :86) and any other read error returns the path-bearing unknown reason immediately (:42, :55, :79, :89), so no unique match can be claimed after an unreadable branch. A false unique match would require a same-named rollout to disappear from one branch while another matched, which the immediate-return-on-error rule prevents; duplicate physical reachability through a directory symlink produces a conservative multiple-match refusal, not a false unique.

Checked against live data: `payload.usage` on a real `token_usage_record` is `{input_tokens, cached_input_tokens, cache_write_input_tokens, output_tokens, reasoning_output_tokens, total_tokens}` and the row carries top-level `timestamp` and `ordinal`, matching `parseCodexLine` exactly.

### Gate and ratchet risk

- `gofmt -l` over all six files: clean.
- `staticcheck` 2026.2 (module v0.8.0) on `./internal/usage/`: **exit 0, no output.** This is the stage the builder could not run (its fast gate stopped on the unavailable module download at `codex-ccb-slice2-result.md:168-179`). Since gofmt, vet, the refusal register and the build were the four stages the builder's run did clear, and staticcheck was the sole red, the fast gate should now pass end to end.
- Coverage: the builder measured 88.0 percent; both baselines carry `"internal/usage": 85.8` at `coverage-ratchet.json:79` and `coverage-ratchet-linux.json:79`. No floor is lowered and no new package is introduced.
- `validate design-obligations` still refuses the page's custom CCB table, but that predates this diff, the page is unmodified here, and the go gate does not run that verb.

## Findings

| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| F-1 | medium | no | On a second `LatestCall` over an unchanged legacy Codex rollout the reading answers `unknown (no call recorded yet)` instead of D2-3's `unknown (rollout carries no token_usage_record (codex CLI before 0.153))`. The `usageRecords` and `tokenCounts` counters live in the parse closure, and the unchanged-transcript shortcut returns before any line is parsed. | `internal/usage/calls.go:128-147` (counters incremented only inside the closure) against `internal/usage/cursor.go:66-68` (unchanged shortcut returns before the stream is opened). CCB-2-03 only names the first read; the builder's own round-1 critic raised this and refuted it against the brief's "after a full read" wording at `artifacts/reports/codex-ccb-slice2-result.md:88`. Route to slice 2b, whose `context status` and `RoleContext` print this string. |
| F-2 | low | no | Marker rows written to the samples log silently drop the `Marker`'s own `kind`. `markerRow` declares `Kind string \`json:"kind"\`` at depth 0 and embeds `Marker`, whose identical json name at depth 1 is suppressed by Go's shallowest-depth rule, so the row encodes as `{"kind":"marker",...}` with no `compaction` anywhere and decodes with `Marker.Kind` empty. `Calls` re-stamps it. | `internal/usage/cursor.go:27-30` (the shadowing) and `internal/usage/cursor.go:236` (`row.Marker.Kind = "compaction"` reconstructing it). Confirmed by a standalone probe: encoded `{"kind":"marker","runtime":"claude",...}`, decoded `inner.Marker.Kind=""`. Harmless while `compaction` is the only kind (D2-5) and while D2-11 requires 2b's report to read through `usage.Calls`; it becomes a silent data loss the moment a second marker kind or a raw-JSONL consumer appears. |
| F-3 | medium | no | A source-path flip re-appends an entire transcript generation, double-counting those calls in the week's cohort. A path change resets `Seen` and carries `SamplesBytes` forward, and there is no cross-generation deduplication, so `context status --transcript P` followed by an ordinary hook read that derives the real path appends every sample of the real transcript a second time. `Calls` then returns both copies, inflating D2-9's cohort size, p95 and max. | `internal/usage/cursor.go:72-75` (path-changed restart) and `internal/usage/cursor.go:324-332` (`freshCallCursor` empties `Seen`). This is exactly what amendment D2-11 point 6 and fold-brief step 1 prescribe, so the implementation conforms; the exposure is in the amended design. Route to the design page and to CCB-2-17. |
| F-4 | medium | no | Every Codex rollout line is JSON-decoded twice: once by `codexRecordKind` to classify it and again by `parseCodexLine`. The brief only asked for a local `token_count` counter kept while scanning, not a second full decode. On the largest live rollout on this box (264,233,967 bytes, 54,471 lines) that doubles the first read's parse cost, and the whole read happens under the exclusive per-cursor flock on the Stop-hook path. | `internal/usage/calls.go:130-140` (closure calls `codexRecordKind` then `parseCodexLine`) and `internal/usage/calls_codex.go:160-181` (`codexRecordKind` runs its own `decodeCallLine`). Measured: `/Users/wido/.codex/sessions/2026/09/07/rollout-2026-09-07T07-30-14-01a07a58-...jsonl`, 264 MB, 54,471 lines, 6,638 `token_usage_record` rows. Cheap to fix inside `calls_codex.go` by returning the kind from one decode. |
| F-5 | medium | no | The cursor rewrites the entire `Seen` set on every read, so the per-Stop-attempt cursor write grows linearly with the session's sample count, which D2-5's cost claim ("one read per Stop attempt is the bytes appended since the previous attempt") does not account for. `atomicWriteJSON` renders through `wiredoc.RenderValue`, which marshals, re-decodes into `map[string]any` and re-renders, then publishes with an fsync. | `internal/usage/calls.go:58` (`Seen` persisted in `CursorState`), `internal/usage/cursor.go:177` (published every read), `internal/usage/usage.go:122-128` and `internal/wiredoc/wiredoc.go:39-49` (the three-pass render). Measured on live data: this seat's transcript yields 2,547 distinct request ids, about 94 KB of `Seen` JSON; the largest live Codex rollout yields 6,638 response ids of 55 characters, about 425 KB rewritten and fsynced per Stop attempt. The `Seen`-in-cursor shape comes from the slice-2a brief, so this is a design cost, not a fold defect; it sits directly beside the `stop-hook-duration` health role. |
| F-6 | low | no | `bufio.Reader.ReadBytes` materialises an over-long line in full before the 32 MiB cap rejects it, so a transcript delta containing no newline is buffered whole into memory before refusal, unlike spend's `bufio.Scanner`, which refuses at its buffer cap. `TestScanErrorDoesNotAppendBeforeCursorAdvances` itself materialises 32 MiB twice. | `internal/usage/cursor.go:108-111` (the length check follows the read) versus the brief's citation of spend's `maxTranscriptLineBytes`. The brief prescribed `ReadBytes` with a 32 MiB cap, so the shape conforms. |
| F-7 | medium | no | A session whose cursor is lost while its samples log survives becomes a permanent hard refusal from both APIs with no automatic recovery, and slice 3's `context prune` can create exactly that state. `reconcileCallRows` refuses a nonempty log without a trusted cursor by design, and D2-11 point 2 forbids inferring a boundary or deleting the evidence. D3-1 says prune "removes cursor and samples files older than the window"; if the window retires a cursor whose samples file is newer, or if either removal partially fails, that session's `Calls` errors forever and takes D2-9's whole week report down with it, because the amendment requires the report to propagate recovery errors. | `internal/usage/cursor.go:435-439` (the refusal) against design D3-1 (`plans/coordinator-context-stays-under-budget-design.md:245`) and the amendment's report rule (`plans/...:362`). Nothing in this diff is wrong; `PruneContext` does not exist yet. Route to slice 3 as a design constraint: prune must remove a cursor and its samples file as a pair, oldest-first, or leave both. |
| F-8 | low | no | The first read of a session holds the exclusive per-cursor flock across the entire transcript parse. On a 264 MB Codex rollout or a 47.8 MB Claude transcript every other reader of that session blocks for the whole parse. | `internal/usage/cursor.go:35-181` (the lock spans load, reconcile, open, scan, append and publish). Prescribed by the brief ("The lock is held across everything below", `codex-ccb-slice2-brief.md:114`) and by D2-5. Recorded so the 2b latency bound is set with this in view. |
| F-9 | low | no | Row CCB-2-18 cites `coverage-ratchet.json:78` and `coverage-ratchet-linux.json:78` for the `internal/usage` floor; the entry is at line 79 in both files (line 78 is `internal/up`). The fold brief's citation of `:79` is the correct one. | `plans/coordinator-context-stays-under-budget-design.md:280` against `scripts/agents/coverage-ratchet.json:79` and `scripts/agents/coverage-ratchet-linux.json:79`. Citation only; the floor value 85.8 is right in both. |
| F-10 | low | no | The worktree's own tracked design page ends at line 324 and carries no amendment section, so the tree this change lands from does not contain its own governing spec. The amendment exists only in the main checkout's copy (line 317) and in the gitignored `artifacts/reports/ccb-8c-amendment.md` the builder worked from. | Worktree `plans/coordinator-context-stays-under-budget-design.md` is 324 lines with zero matches for "Amendment"; `/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/plans/coordinator-context-stays-under-budget-design.md:317` carries it. Reviewability note for the landing: the worktree copy should be brought forward, or the landing should go through the main checkout. |

## What I did not do

I did not run `go build`, `go vet`, `go test -race` or the fast gate, as instructed; the seat owns those. I did run `staticcheck` on `internal/usage` alone, because that was the one fast-gate stage the builder could not reach, and it is clean. I ran no fixture bed and edited no repository file.
