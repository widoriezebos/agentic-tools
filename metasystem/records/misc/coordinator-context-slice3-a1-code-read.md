# Independent code read: coordinator-context slice 3, unit A1

Reviewer: Opus 5, not the author of this change.
Worktree read: `/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/.claude/worktrees/ccb-s3/metasystem`
Base: `origin/main` at `c7080ebb6e368af04bb4fa00c94d4f5ad5ed7487`.
Brief: `artifacts/reports/codex-ccb-slice3-brief-v2.md`, unit A1.
Builder result: `artifacts/reports/codex-ccb-slice3-result.md`.

Computed diff, not the builder's file list: 7 tracked files (92 insertions, 40
deletions) plus 2 untracked files, `internal/usage/evidence.go` (126 lines) and
`internal/usage/evidence_test.go` (227 lines). Total about 485 changed lines,
inside A1's 450 to 800 band.

Materiality test applied verbatim to every finding: would the change ship a
defect, violate its brief, or damage what certifies it?

## Findings

| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| F-1 | medium | yes | The testing contract and its assertion were changed. That is unit D's lane ("Context verbs, Stop allowance, instructions, tests and engine selection") and outside the seat's declared A1 scope, and nothing forced it. | `metasystem/testing.json` line 51 adds `TestCallEvidenceSnapshotSerializesMaintenance`, `TestCallRetentionPreservesNonBlockingReads` and `TestContextReportUsesOneEvidenceSnapshot` to `context-standard`; `cmd/metasystem/context_verbs_test.go:483-484` and `:496-497` add the same three names to `wantedTests`. The contract test asserts containment, not equality (`cmd/metasystem/context_verbs_test.go:505-509`: `if !containsString(names, name) { t.Errorf(...) }`), so leaving both files untouched passes. The three new tests already run without it: `context-foundations-standard` selects `internal/usage` with `tests: "all"` and `governed-standard` selects `internal/steward` with `tests: "all"` (`testing.json` lines 50 and 47). Brief landing-units table, `artifacts/reports/codex-ccb-slice3-brief-v2.md:32`, assigns engine selection to D. |
| F-2 | low | no | The report-side snapshot proof is stub-only. It never executes `usage.ReadCallEvidence`, so it proves call-count and wiring, not one consistent view. | `internal/steward/contextreport_test.go:106-140` replaces `readContextCallEvidence` with a closure and asserts `reads == 1`, `report.Samples == 1`, `report.Max == 100000`. The only binding to the real snapshot is the compile-time assignment at `internal/steward/contextreport.go:53`. The brief's report-level serialization proof, `TestContextReportSerializesWithPrune`, is an A2 test and is absent. Consistency is genuinely proven one layer down, in `internal/usage/evidence_test.go:13-173` (see verified negatives below), so the property holds; the builder's obligation table overstates what this particular test contributes. |
| F-3 | low | no | A pure read now has a write side effect: `CallSessions` on a state root with no call store creates `artifacts/agents/context/` and `maintenance.lock`, where before it touched nothing. | `internal/usage/evidence.go:97` runs `os.MkdirAll(filepath.Dir(path), 0o755)` on every public operation. Before the change, `CallSessions` (`internal/usage/sessions.go:47-53`) went straight to `collectCallStoreStems`, which returns nil on `os.IsNotExist` without creating anything. `LatestCall` and `CallRegistrations` are unaffected, because `lockCallFileWithFlags` (`internal/usage/cursor.go:617-619`) already did the same MkdirAll. `CallSessions` has no production caller after this change (grep over `*.go` found only tests), and `artifacts/` is gitignored (`metasystem/.gitignore:1`), so no stray tracked file results. |
| F-4 | low | no | Every blocking maintenance acquisition is unbounded. `metasystem context report` can wait arbitrarily long behind one cold health read, because `LatestCall` holds the shared lock for the whole transcript scan. | `internal/usage/evidence.go:112` issues `unix.Flock` with no deadline. Blocking callers: `internal/usage/cursor.go:223` (`Calls`), `:307` (blocking `registerSession`), `internal/usage/sessions.go:39` (`CallSessions`), `:200` (`CallRegistrations`), `internal/usage/evidence.go:40` (`ReadCallEvidence`). The shared lock is taken at `internal/usage/calls.go:124`, before `readUnderCursor` scans the transcript, and released by defer at `:128`. This is what the brief asks for (only the NonBlocking variants must be NB), and the Stop path is not affected (see verified negatives). |
| F-5 | low | no | One assertion in the snapshot test is close to a no-op, though the test as a whole is not tautological. | `internal/usage/evidence_test.go:77-82` polls `writerDone` with a `default:` branch immediately after `<-writerAttempted` fires. `writerAttempted` is closed from the `callFileOpens` seam at `internal/usage/evidence.go:100`, which runs before `os.OpenFile` and before `unix.Flock`, so at the instant of the poll the writer has executed almost nothing and `default` is taken even on a broken build. I mutated `lockCallMaintenance(stateRoot, true, false)` to `false` in a scratch copy: the test still fails, at `evidence_test.go:108`, with `Samples` holding both "first" and "second". The property is proven by the content assertion at `:104-109`, not by the poll. |
| F-6 | low | no | `TestCallRetentionPreservesNonBlockingReads` uses a one-second wall-clock deadline, which is scheduling-sensitive under the loaded deep battery. | `internal/usage/evidence_test.go:203`. Mitigations: the operation it bounds is a non-blocking flock that returns in microseconds (measured subtest time 0.00s), and repo precedent for a one-second bound exists at `internal/usage/cursor_test.go:88`, `:128` and `internal/steward/context_test.go:377`, `:421`. The sibling barriers in the same file use five seconds (`evidence_test.go:53,73,92,100`). |
| F-7 | low | no | The builder's result report calls its own critique round "independent". I cannot establish who ran it; if it was the builder, it must not count as the independent read. | `artifacts/reports/codex-ccb-slice3-result.md`, checks section: "Independent two-layer critique over the final computed patch — zero material findings. `/tmp/codex-ccb-s3-metasystem validate critique-closed ...` returned exit 0." The code-critique skill requires a critic who did not write the implementation and forbids the critic disposing its own findings. Nothing rests on this line since the seat commissioned this read, but the seat should not bank it as a round. |

## Verified negatives, with the check and the observed result

These are the seat's six questions. None produced a finding.

**1. One consistent view, and whether a concurrent writer can split a report.**
Yes, one view, for the usage store. `ReadCallEvidence` (`internal/usage/evidence.go:36-89`) holds one exclusive `flock` across the registry copy, session discovery and every session's committed rows, and releases it before statistics and publication. All writers take the same lock shared first: `internal/usage/calls.go:124` (`LatestCall`, the sample and cursor writer) and `internal/usage/cursor.go:307` (`registerSession`). Exclusive excludes shared, so no append or registration can land mid-snapshot.

Mutation check, scratch copy at `.../scratchpad/mut`, exclusive changed to shared in `ReadCallEvidence`: `TestCallEvidenceSnapshotSerializesMaintenance` FAILS at `evidence_test.go:108`, with the snapshot containing the concurrent writer's "second" sample. Mutation check, maintenance lock removed from `LatestCall`: FAILS at `evidence_test.go:75`, `:169` and `:209`. Mutation check, maintenance lock removed from `registerSession`: FAILS at `evidence_test.go:169` and `:215`. The tests are real proofs, not tautologies.

Rows are still read only to the committed boundary (`internal/usage/cursor.go:262`, `io.LimitReader(file, cursor.SamplesBytes)`), so an uncommitted append suffix cannot enter the snapshot.

The report's other two inputs, handoff counts and reference mismatches, are read after the lock is released (`internal/steward/contextreport.go:92` and `:96`). I checked whether they can disagree with the snapshot: they cannot. `contextReportHandoffs` counts consumed-intent files (`contextreport.go:476-499`) and `contextReportReferenceMismatches` counts failed job records (`:502-541`). Neither joins to `sessions` or `registrations`; they are independent counters assigned onto the finished report. No cross-boundary disagreement exists.

The one acknowledged gap, a writer that appends a sample before registering its session, is explicitly out of scope in the brief: "this change does not claim that those two existing operations become one transaction."

Nothing outside `internal/usage` writes the call store. Grep over `*.go` and `*.sh` for the samples and cursor directories found no writer in any other package or script.

**2. Maintenance locking: what it serialises, bounds, deadlock, and Stop.**
It serialises every public usage operation against the report snapshot, at store granularity. Order is uniform everywhere: maintenance first, then exactly one of the registry lock or one cursor lock, released inner-first by LIFO defers. `ReadCallEvidence` takes the registry lock and releases it (`sessions.go:208-212`, deferred inside `callRegistrationsUnderMaintenance`) before taking any cursor lock, satisfying the brief's "Never hold a registry lock while taking a cursor lock". The lock-order assertion is proven mechanically for all six public entry points at `evidence_test.go:119-173`, and I confirmed it fails when either lock is dropped.

No deadlock is reachable. For a cycle, some holder of a cursor or registry lock would have to wait for maintenance, but a holder of a member lock necessarily already holds maintenance shared, which is incompatible with the exclusive holder existing. No path in `internal/usage` calls a public API while holding the lock; the snapshot uses `callRegistrationsUnderMaintenance`, `callSessionsUnderMaintenance` and `callsUnderMaintenance`, exactly as the brief requires. `inspectCallSessionPair` (`sessions.go:112-146`) calls no public API either.

Bounds: none on the blocking paths, recorded as F-4.

Stop cannot block. The only two usage entries on the health path pass `NonBlocking: true`: `internal/steward/context.go:98` sets it, `:108` calls `RegisterSessionNonBlocking` and `:121` calls `LatestCall`. Both map an exclusive holder to `*CallStoreBusyError` immediately (`evidence.go:109-116`), and `context.go:109` and `:123` turn that into `roleUnknown`, which `checkContextBudget` at `:148-151` reduces to a verdict with the error dropped. So the role degrades to unknown during a report and never waits. `WriteContextReport`, the only caller of `ReadCallEvidence`, is reached only from `cmd/metasystem/context_verbs.go:107`, the `context report` CLI verb; it is not on the Stop or health path. `TestCallRetentionPreservesNonBlockingReads` (`evidence_test.go:176-227`) proves both non-blocking entries return typed busy under an exclusive holder without creating the inner lock files.

**3. Honest answers for partial or missing pairs, and evidence deletion.**
A1 contains no deletion, no unlink, no truncate. `lockCallMaintenance` opens with `O_CREATE|O_RDWR` and never writes (`evidence.go:101`). Since retirement is A2 and absent, no half-retired pair can exist; the only half-pairs are pre-existing damage, and the brief requires that be refused loudly ("Preserve and refuse pre-existing unexplained damage"). That behavior is preserved byte for byte: the identity checks and the wrapped recovery error moved verbatim from `contextreport.go` into `evidence.go:60-83`, and `TestContextReportPropagatesRecoveryError` is retained and passes. The honest-answer path for genuinely retired evidence (`ContextEvidenceRetiredError`, the `RetainedSince` refusal) belongs to A2; `CallEvidence.RetainedSince` is declared as the brief specifies and stays zero, which makes any future comparison a no-op today.

**4. Do the named tests fail on the pre-change tree, and are assertions tautological.**
A literal pre-change run is a compile failure, since `ReadCallEvidence`, `CallEvidence`, `CallStoreBusyError` and `readContextCallEvidence` are new symbols. I ran the stronger check instead, three behavior-removing mutations in a scratch copy, each caught (details under question 1). One near-no-op assertion found, F-5. No other tautology: the ordering test at `evidence_test.go:168` reads a real seam, and the report test does check that samples flow into the statistics rather than only counting calls.

**5. Coverage floors and gate refusal.**
`internal/usage` measured 87.7 percent against a floor of 85.8 in both `scripts/agents/coverage-ratchet.json` and `coverage-ratchet-linux.json`. `internal/steward` measured 79.9 percent against 74.0 darwin and 78.5 linux. The ratchet only refuses below the floor, on an unfloored measured package, or on a floored package that was not measured (`internal/audit/coverage.go:82-116`); no new package is introduced, so none of the three fires. No floor was edited.

Ran here and green: `go vet ./internal/usage/... ./internal/steward/...` exit 0; `go test ./internal/usage/ ./internal/steward/` PASS; `go test -race -count=1 ./internal/usage/` PASS in 4.1s; `go test -race -count=1 ./internal/steward/` PASS in 124.8s, exit 0. The builder reported that last one as failing; on this machine, with a warm module cache, it passes, so its failure was the sandbox's network and not the change. `gofmt -l` clean on all three changed trees. No `t.Parallel()` anywhere in `internal/steward`, so swapping the `readContextCallEvidence` seam is safe, per the brief's "Do not run seam-swapping tests in parallel".

Not run, correctly reserved: `staticcheck` (not installed locally, same constraint the builder hit), `scripts/agents/go-gate.sh --fast`, the engine group runs, and `TestContextStopFitsDurationBudget`, which needs `METASYSTEM_CONTEXT_COST_PROOF=1`, a built candidate engine and a 264 MB generated source (`cmd/metasystem/context_cost_test.go:89-104`). The change adds one MkdirAll, one open and one flock per health read, which is the thing that perf proof would measure; the seat should run it before landing. `testing.json` is modified but uncommitted in this worktree, and the strict committed-contract rule will refuse a gate run until it is committed.

**6. Scope.**
Present and in scope: `internal/usage/evidence.go`, `internal/usage/evidence_test.go`, `internal/usage/calls.go`, `internal/usage/cursor.go`, `internal/usage/sessions.go`, `internal/steward/contextreport.go`, `internal/steward/contextreport_test.go`. Public signatures preserved exactly as the brief lists them. Nothing from A2, B, C or D: no `retention.go`, no `PruneCallSessions`, no `recoverCallRetirement`, no `retireCallSession`, no `ContextEvidenceRetiredError`, no `HandoffBinding`, no new verb. No fixture, no coverage floor, no role text, no script changed. No stray receipt row; `git status` shows exactly the six tracked modifications and the two new files, and `records/` is untouched. The one scope departure is `testing.json` with its paired assertion, F-1.

One unrelated note, not a finding and not caused by this change: `governed-standard` carries `targetMs: 5000` while `internal/steward` alone takes about 125 to 190 seconds. That mismatch predates A1.

## Verdict

Fit to land once the seat decides F-1, and after the seat runs the reserved
checks. The locking design is correct, the order is uniform and deadlock-free,
the Stop path cannot block, and the snapshot property is proven by tests that
three separate mutations confirm are real. Nothing can delete or hide evidence
in this unit.

What must change, or be decided: F-1, the `testing.json` and
`cmd/metasystem/context_verbs_test.go` edits. Either revert both to base and let
unit D admit the three A1 tests with the rest of the engine selection, keeping A1
inside its declared scope, or accept the widening deliberately and record it,
noting that the additions are self-consistent, purely additive, and that the
three tests already run under `context-foundations-standard` and
`governed-standard` without them.

Material findings: 1 (F-1). Recorded, not blocking: F-2 through F-7.
