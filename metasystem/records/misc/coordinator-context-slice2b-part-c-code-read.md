# Independent code read: coordinator-context slice 2b, part C

Reviewer: Opus 5, main checkout `/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem`.
Date: 2026-09-13. Read only; nothing in the repository was edited, nothing was committed,
no fixture bed was run.

## What was read

Computed diff: the working tree against `HEAD` (`e8a8b218`, which contains `87ce9217`),
plus the four untracked files.

```
metasystem/cmd/metasystem/context_verbs.go       +35  -0
metasystem/cmd/metasystem/context_verbs_test.go +106  -0
metasystem/cmd/metasystem/main.go                 +1  -0
metasystem/internal/usage/cursor.go               +2  -9
metasystem/records/narrator-digest.log            +1  -0
metasystem/internal/steward/contextreport.go       (new, 760 lines)
metasystem/internal/steward/contextreport_test.go  (new, 700 lines)
metasystem/internal/usage/sessions.go              (new, 242 lines)
metasystem/internal/usage/sessions_test.go         (new, 197 lines)
```

`plans/handoff-m1e-analysis-session.md` was ignored as instructed.

Spec read: part C of `artifacts/reports/codex-ccb-slice2b-brief-v2.md`,
`artifacts/reports/ccb-8c10-slice2b-amendment.md`, and section 8c of
`plans/coordinator-context-stays-under-budget-design.md` including its
subsections 8, 9, 10 and 11, all in the `ccb-s2b` worktree. Builder report:
the "Part C" section of `artifacts/reports/codex-ccb-slice2b-result.md`.

Checks I ran that the seat had not: package coverage with and without the change,
the real `bin/metasystem audit coverage-ratchet` on the measured value, a line
coverage profile of `contextreport.go`, and three probe tests in a scratch copy at
`/private/tmp/.../scratchpad/ccb2c-probe` (never in the repository).

## Part B carry: confirmed clean

I confirmed the seat's claim independently.

- `git diff --stat 87ce9217 -- metasystem/internal/steward/context.go metasystem/internal/steward/context_test.go`
  returns empty. Both files are byte-identical to `origin/main`.
- `git diff HEAD --numstat` for the two command files and `main.go` gives
  `35 0`, `106 0` and `1 0`. A grep for removed lines in both command files
  returns zero. Additions only; no part B line was altered or reverted.
- No other tracked file under `metasystem/` differs except `internal/usage/cursor.go`
  (the narrowly scoped `sessionRow` to `CallRegistration` swap, with identical
  JSON tags, so the persisted registry shape is unchanged) and the generated
  `records/narrator-digest.log` line, which belongs to commit `e8a8b218` and not
  to this build.

## Findings

| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| F-1 | high | yes | The week report can never reach PASS on a real installation. The coverage-gap test walks the complete session registry with no window filter, so every session ever registered that has no dated sample inside the requested week becomes a gap and a FAIL clause. `sessions.jsonl` is append-only and nothing in the repository prunes it, so the set of such sessions only grows. This also puts part C in direct conflict with slice 3: amendment 10.3 retires a cursor and its samples as a pair and never removes the registry row, so a lawfully retired session becomes a permanent gap. Section 7's "one seven-day UTC window" proof, which this verb exists to produce, is unreachable after the first week. | `internal/steward/contextreport.go:352-367` (the registration loop has no `FirstSeen` or week test, unlike `contextReportResets` at `:459`); `internal/usage/cursor.go:292-369` is the only writer and `internal/usage/sessions.go:189` the only other reader, so no pruning exists; `artifacts/reports/ccb-8c10-slice2b-amendment.md` decision 10.3 retires the pair only. Probe output (scratch copy, `internal/steward/zz_probe_test.go`): (A) report for last week with only last week's session: `pass=true`; (B) the same last-week report after one new session starts this week: `pass=false failures=[coverage gap: registered per-call session claude/new has no dated sample in the week]`; (C) this week's report while last week's session is still registered: `pass=false failures=[coverage gap: registered per-call session claude/old has no dated sample in the week]`; (D) after deleting a pair the way slice 3 will: `pass=false failures=[coverage gap: registered per-call session claude/retired has no dated sample in the week]`. |
| F-2 | high | yes | The coverage ratchet refuses. `internal/usage` measures 84.1 percent with part C against a floor of 85.8 percent in both baselines. The baseline at `HEAD` without part C is 88.6 percent, so part C causes a 4.5 point drop. The dominant cause is the new exported `CallRegistrations`, which has 0.0 percent coverage from `internal/usage`'s own tests, plus partly covered discovery branches. CCB-2-18 and the brief's floor table are violated, and `section/go-engine-gate` will fail. The builder's `go-gate.sh --fast` cannot see this because the fast gate carries no ratchet. | `go test -count=1 -cover ./internal/usage/` in the main checkout: `coverage: 84.1% of statements`; the same under `-race`: `84.1%`. Same command in a `git archive HEAD` scratch copy: `88.6%`. `bin/metasystem audit coverage-ratchet --baseline scripts/agents/coverage-ratchet.json` on that value prints `coverage ratchet: package internal/usage coverage 84.1% is below its ratchet floor 85.8%` and exits 1. Floors at `scripts/agents/coverage-ratchet.json:80` and `scripts/agents/coverage-ratchet-linux.json:80`. Per-function: `internal/usage/sessions.go:185 CallRegistrations 0.0%`, `sessions.go:153 readDiscoveredCallCursor 68.8%`, `sessions.go:103 inspectCallSessionPair 81.8%`. `internal/steward` measures 80.2 percent with part C, above both its floors (74.0 darwin, 78.5 linux), so only `internal/usage` blocks. |
| F-3 | medium | yes | The named test for CCB-2-17 does not prove one of its brief clauses. The brief requires `TestContextReportWindowAndCoverage` to cover "unregistered explicit-session evidence", and part C states "For explicit-session samples with no registry, mark reset coverage unavailable and FAIL. This avoids manufacturing a passing reset check from missing registration." The code implements that rule, but no test reaches it. The subtest actually named `unregistered evidence` proves something else: it registers the session and then stubs the runtime registry lookup so that `claude` is not a known runtime, exercising the `runtime %s is not registered` branch instead. | The rule is at `internal/steward/contextreport.go:368-377`; the coverage profile shows the block starting at `contextreport.go:374` with count 0 after running every `TestContext*` test in `internal/steward`. The mis-aimed subtest is `internal/steward/contextreport_test.go:378-397`, which calls `usage.RegisterSession(root, "claude", "unregistered", 502, 5002)` and then replaces `lookupContextReportRuntime`. |
| F-4 | medium | no | `expectedContextSampleSource` hardcodes a two-entry map of `claude` to `claude-transcript` and `codex` to `codex-rollout`. Declaring a third per-call runtime in `internal/runtimes` would turn every one of its samples into an `unexpected sample source` coverage gap and a permanent FAIL, with no compile-time link between the two declarations. There is no such runtime today, so nothing ships broken now. | `internal/steward/contextreport.go:385-388`; the capability it trusts is read at `:298` from `runtimes.Lookup`. |
| F-5 | low | no | One malformed JSON file anywhere under `artifacts/agents/steward/consumed` or `artifacts/agents/jobs` aborts the whole week report with exit 1, whether or not it has anything to do with context. Both directories hold records written by unrelated subsystems and older schema generations. | `internal/steward/contextreport.go:518` (`consumed context intent %s is malformed`) and `:545` (`cannot read context report job record %s`). Neither branch is exercised by a test. |
| F-6 | low | no | A single torn append to `sessions.jsonl` refuses every future report permanently. The next `registerSession` repairs only the missing separator, leaving the fragment as its own malformed line, and the strict reader then refuses. Nothing repairs or retires the registry, and no CCB row owns one. This is the safe direction (a partial row can never be read as complete), but it has no recovery path. | Separator repair at `internal/usage/cursor.go:321-325`; strict refusal at `internal/usage/sessions.go:214-235`. |
| F-7 | low | no | Discovery creates a `<stem>.json.lock` file in the cursors directory for every stem it finds, including stems that exist only as an orphan samples file which it then refuses. Repeated report runs against a damaged store accumulate lock files for stores that have no cursor. | `internal/usage/sessions.go:59` calls `lockCallFile` before `inspectCallSessionPair`, and `lockCallFileWithFlags` at `internal/usage/cursor.go:599-606` does `MkdirAll` plus `O_CREATE`. |
| F-8 | low | no | The discovery proof omits two refusals the brief names. Neither the "identity does not map to both store basenames" path nor the invalid-runtime location refusal has a case in `TestCallSessionsDiscoversPairsWithoutReadingSamples`. | Code at `internal/usage/sessions.go:128-135`; the test's subtests are `internal/usage/sessions_test.go:14,64,78,93,123` and none writes a cursor whose identity disagrees with its filename. |
| F-9 | low | no | Several error and tie-break paths of the report have no test. The publication failure and "durability unknown" branches, the non-midnight week refusal, and the sort tie-breakers on invocation id and ordinal that the brief calls "deterministic ordering" are all uncovered. `marker.Kind != "compaction"` is unreachable because `Calls` rewrites every marker kind before returning it. | Uncovered blocks from the profile: `internal/steward/contextreport.go:142`, `:144`, `:147`, `:149`, `:158`, `:301`, `:323`, `:327`, `:331`, `:429`, `:432`, `:613`. The kind rewrite is at `internal/usage/cursor.go:270`. |
| F-10 | low | no | Landing hygiene, not part C's work: the working tree carries a generated `records/narrator-digest.log` line describing commit `e8a8b218`, and `.claude/worktrees/` is untracked and not git-ignored, so a broad `git add` would sweep the whole builder worktree into the commit. | `git diff HEAD -- metasystem/records/narrator-digest.log` shows the single added line; `git check-ignore -v .claude/worktrees/` exits 1. |

## What I checked and found correct

These are the task's named questions, answered from the code.

**Conformance to the part C rows.** Every row assigned to part C has its named
test and each asserts what its row claims, with the single exception recorded
as F-3.

- CCB-2-17 as amended: rows come only from `usage.Calls(stateRoot, runtime, session, time.Time{})`
  at `contextreport.go:81`. Nothing in the report opens a samples file; the only
  direct samples-path uses are in error strings. The recovery error is wrapped
  with the session and both paths at `:82-86`, and
  `TestContextReportPropagatesRecoveryError` proves the missing cursor, malformed
  cursor, missing positive-boundary samples and short samples cases, each with a
  path-bearing error, byte-identical previous outputs and unchanged damaged
  evidence. The CLI twin proves exit 1 with both paths.
- CCB-2-37: `TestContextReportDeduplicatesTranscriptReplays` ingests P, Q, then P,
  shows the raw history growing to three rows while the report keeps three
  distinct calls, stable p95, maximum and verdict, and `DuplicateSamples` 2.
  A reused id in another session and another runtime stays distinct.
- CCB-2-38: `TestContextReportHandlesFallbackAndConflictingIdentities` covers
  exact provider, fallback and marker replay collapse, a fallback line reused at
  another timestamp, source-annotation insensitivity, both conflict refusals with
  previous outputs untouched, and undated evidence forcing FAIL.
- CCB-2-41: `TestCallSessionsDiscoversPairsWithoutReadingSamples` covers hashed
  and hyphenated identities, `.lock` and `.tmp` exclusion, malformed cursor,
  nonempty orphan, empty orphan, symlink, unreadable member, a cursor with no
  samples, concurrent first publication under the real lock, sorted output, and
  zero sample-body opens through the `callFileOpens` seam.
- CCB-2-53 (design subsection 11, assigned to part C):
  `TestContextReportExcludesTranscriptDiagnostics` runs an inferred override with
  a distinct 210,001-token call and a compaction, then an override for an absent
  explicit session, and asserts the cohort, resets, compactions, verdict and the
  calls export bytes are all unchanged.

**The session registry.** Locking is correct. `registerSession` holds an
exclusive `flock` on `sessions.jsonl.lock` across the whole read-check-append
(`cursor.go:296-311`), so two writers cannot interleave; the append is a single
`Write` with a short-write check. `CallRegistrations` takes the same lock and
releases it through `defer` before returning (`sessions.go:190-194`), and
`WriteContextReport` calls it before `CallSessions`, so the registry lock is never
held while a cursor lock is taken. A partial or malformed row cannot be read as
complete: a missing trailing newline refuses (`sessions.go:214`), each row is
decoded with a strict `json.Decoder` that refuses trailing values
(`sessions.go:223-232`), and every field is validated including a runtime-name
pattern, a nonempty session, positive pid and pid start time and a nonzero
`firstSeen` (`sessions.go:233`). The one real gap is growth: the registry is
append-only with no retention, which is the mechanism behind F-1 and also means
`registerSession` re-scans the whole file on every Stop.

**The week report.** It obtains rows only through `usage.Calls` and propagates the
recovery error, as amended CCB-2-17 requires; it never scans to end of file. The
cohort matches section 7: recorded per-call samples only, with per-invocation and
no-stream runtimes excluded by capability (`contextreport.go:298-303`) rather than
by name. The window is the half-open `[weekStart, weekStart+7d)` at UTC midnight
(`:63`, `:225`), and `contextReportWeekStart` refuses a non-midnight value. P95
uses the brief's nearest-rank index `(95*n+99)/100-1` and both p95 and the maximum
are computed over `retained` only, which is the deduplicated, in-week,
registered, per-call, expected-source set (`:337-349`). Collected evidence cannot
distort it, because part B routes every transcript override to a private
disposable store; restarted evidence cannot distort it either, because provider-id
copies collapse on equal timestamp and all four token fields while ignoring source
annotation and ordinal, and conflicting values refuse instead of publishing. The
`; restarted: ...` annotation is accepted by `expectedContextSampleSource`. Claude
`line:<n>` fallback rows do double-count across a path restart, because the line
counter resets, but amendment 10.1 decides that explicitly and the coverage
statement discloses it, so it is out of scope rather than a defect.

**The report verb.** Exit codes are right: 2 for a missing flag, a stray argument
or a malformed `--week` (both `2026-9-13` and `2026-02-30` are tested), 1 for a
resolve or operational error, and 0 for a measured PASS and a measured FAIL alike.
Output is one short line of the two paths and the verdict, so no spill envelope is
owed: design section 2 scopes the envelope to `publishTestingResult` and
`land.sh`, and `output.Spill` has exactly those two producers in the tree. A week
with no data publishes both files and reports FAIL at exit 0 (probed:
`pass=false failures=[the distinct per-call cohort is empty]`, both files present).
Reads, normalization, validation and rendering all complete before the first
write; `calls.jsonl` publishes first and `report.md` last carrying its SHA-256, so
an interrupted second publication is detectable.

**Discovery against a foreign or stale row.** Discovery never consults the
registry; it enumerates the cursor and samples directories, takes identity from
the cursor only, validates the runtime name, and checks that the identity maps
back to both basenames through `CursorPath` and `SamplesPath`. It never reverses a
hash or splits on a hyphen. A foreign or stale registry row therefore cannot
invent a session; it can only produce a coverage gap, which is F-1. One bad
session does fail the whole report, and that is what the brief asks for ("Do not
skip a damaged session").

**Scope.** Nothing from part D is present: `testing.json`,
`scripts/agents/coverage-ratchet*.json`, `scripts/agents/health-fixtures.sh`,
`scripts/agents/supervision-hook-fixtures.sh`, `scripts/validate-metasystem.sh`
and `cmd/metasystem/context_cost_test.go` are all absent from the diff. No role
contract, template or doctrine text changed. No coverage floor was edited. The
one usage change beyond the named `CallSessions` API is `CallRegistrations`, which
amendment 10.2 does not name; putting the registry reader in the package that owns
the registry is defensible and keeps usage a leaf, but it is the untested surface
behind F-2.

## Verdict

**Not fit to land.** Three material findings stand.

What must change before this lands:

1. F-1: scope the "registered per-call session has no dated sample" gap to
   registrations whose evidence belongs to the requested week, most simply by
   testing `registration.FirstSeen` against the window the way
   `contextReportResets` already does, and add a case to
   `TestContextReportWindowAndCoverage` that reports one week while a session
   registered in another week exists. Carry the registry's missing retention to
   slice 3 alongside CCB-2-39 and CCB-2-40, since retiring a pair without its
   registry row reproduces the same permanent gap.
2. F-2: add `internal/usage` tests until the package is back at or above 85.8
   percent, starting with `CallRegistrations`, which has none. Measure the real
   number rather than the fast gate, and obtain the Linux figure from a Linux seat
   as CCB-2-18 requires.
3. F-3: make `TestContextReportWindowAndCoverage` actually reach
   `contextreport.go:374`, with a discovered explicit-session store that carries
   no registry row, and assert the "reset coverage is unavailable" FAIL clause.

Material findings: 3 (F-1, F-2, F-3). Recorded but not blocking: 7.
# Closing read: coordinator-context slice 2b, part C fold

Reviewer: Opus 5, read-only, in the worktree
`/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/.claude/worktrees/ccb-s2b/metasystem`.
Date: 2026-09-14. Nothing in the repository was edited or committed. No fixture bed was run.
Probes ran only in a scratch copy at
`/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/36c93128-ec16-4146-8def-3f706ba4ef10/scratchpad/probe2`.

## What was read and how the fold was isolated

Computed diff: `git diff origin/main` (base `876e089b`, HEAD `876e089b`) plus the four
untracked part C files.

```
metasystem/cmd/metasystem/context_verbs.go       +35  -0
metasystem/cmd/metasystem/context_verbs_test.go +106  -0
metasystem/cmd/metasystem/main.go                 +1  -0
metasystem/internal/usage/cursor.go               +2  -9
metasystem/internal/steward/contextreport.go       (untracked, 710 lines)
metasystem/internal/steward/contextreport_test.go  (untracked, 658 lines)
metasystem/internal/usage/sessions.go              (untracked, 242 lines)
metasystem/internal/usage/sessions_test.go         (untracked, 293 lines)
```

The main checkout at `/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem` still
holds the pre-fold copies of the four untracked files that the first read reviewed, so I
computed the fold delta exactly rather than re-reading the whole change for drift. Saved at
`.../scratchpad/fold-delta-src.diff` and `.../scratchpad/fold-delta-tests.diff`.

**The entire production-code fold is two hunks.**

1. `internal/steward/contextreport.go:358-360`, inserted after `registeredPairs[key] = true`:

```go
        if !inContextWeek(registration.FirstSeen, weekStart, weekEnd) {
            continue
        }
```

2. `internal/usage/sessions.go:92-95`, in `collectCallStoreStems`:

```go
-        if stem == "" || strings.HasSuffix(stem, ".tmp") {
+        if stem == "" {
```

The four tracked files are byte-identical to their pre-fold state (`diff` reports no
difference against the main checkout's copies). Everything the first read confirmed correct
in those files and in the untouched 700-odd lines of `contextreport.go` therefore stands
unchanged, which is how question 4 is answered below.

Test delta: one new subtest and one rewritten subtest in `contextreport_test.go`, one new
test plus one widened subtest and one helper in `sessions_test.go`, all additive except the
rewrite.

## Fold verification

### F-1, window scoping: resolved, correct at both edges, in UTC

`inContextWeek` (`internal/steward/contextreport.go:276-278`) is
`!value.Before(weekStart) && value.Before(weekEnd)`, an instant comparison, so the UTC
question is settled by `contextReportWeekStart` (`:154-161`) refusing any week start that is
not UTC midnight. Probed all four edges against the registration filter in the scratch copy:

```
B exactly weekStart    pass=false gapFired=true  failures=[coverage gap: registered per-call session claude/edge has no dated sample in the week]
B weekEnd minus 1ns    pass=false gapFired=true  failures=[...same...]
B weekStart minus 1ns  pass=true  gapFired=false failures=[]
B exactly weekEnd      pass=true  gapFired=false failures=[]
```

Half-open `[weekStart, weekEnd)` at both edges, inclusive edges still fail. A session that
genuinely belongs to the week and has no sample still fails: the existing subtest
`TestContextReportWindowAndCoverage/empty and registered gaps`
(`internal/steward/contextreport_test.go:361-374`) registers `claude/missing-samples` with a
real `FirstSeen` of now and asserts the gap clause, and `contextReportTestWeek`
(`:529-533`) sets the week start to today's UTC midnight, so the registration always lands
in-window. That also means the suite carries no wall-clock time bomb.

The new subtest is a real regression test, not a description: deleting the three folded lines
in the scratch copy makes
`TestContextReportWindowAndCoverage/registrations outside the window do not create gaps`
fail with both stale rows reported as gaps.

Slice 3 interaction: prune is age-cutoff based (`PruneCallSessions(stateRoot, before)`,
`artifacts/reports/ccb-8c10-slice2b-amendment.md:33-39`) and deletes cursor and samples
together, leaving the registry row. A pruned pair is older than the cutoff, so its
registration no longer falls in any recent week and creates no gap; and because it is no
longer discovered, the `reset coverage is unavailable` loop (`contextreport.go:370-379`)
does not see it either. Re-running a report for a week old enough to have been pruned still
fails, but it fails on `the distinct per-call cohort is empty` because the evidence itself is
gone, which is not a new defect. The conflict the first read raised is closed.

### F-2, coverage ratchet and the new usage tests: resolved, and the tests are real

`TestCallRegistrationsReadsStrictSnapshot` (`internal/usage/sessions_test.go:14-94`) covers
absolute-root refusal, the missing-versus-empty distinction, exact row decoding in registry
order, and four refusals (truncated tail, malformed JSON, multiple JSON values on one line,
incomplete fields), each asserting the path and the reason, plus a nonregular registry. Two
mutations in the scratch copy confirm these are not padding:

- Removing the strict trailing-value check at `internal/usage/sessions.go:225-230` makes the
  `multiple json values` case fail (it returns a valid row instead of refusing).
- Replacing `defer unlockCallFile(lock)` at `internal/usage/sessions.go:194` with a no-op
  makes `missing and empty registry are distinguished` deadlock and time out, because that
  subtest calls `CallRegistrations` twice on the same state root. The lock release the first
  read verified by inspection is therefore proven by construction, even though no test names
  it. The partial-row rules are asserted directly.

Package coverage is the seat's measurement (88.1 percent against an unchanged 85.8 floor);
I did not repeat it. The floors are untouched: the computed diff contains no ratchet file.

### F-3, the named test: resolved, and it asserts the clause

`TestContextReportWindowAndCoverage/unregistered explicit-session evidence`
(`internal/steward/contextreport_test.go:406-414`) now seeds a real discovered store through
`seedContextReportSession` -> `usage.LatestCall` and registers nothing, so it reaches
`contextreport.go:376-378`. The runtime-registry stub is gone. It asserts
`report.Pass == false`, `report.Samples == 1` and the exact failure clause
`reset coverage is unavailable for claude/unregistered: no registered process evidence`
through `contextReportHasFailure`, which reads `report.Failures`, not rendered markdown. With
Samples at 1 and no markers, registrations or jobs, that clause is the only possible failure,
so the assertion is tight.

## Findings

| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| C2-1 | medium | no | The report can now PASS while a real coverage gap is hidden, in one narrow shape: a session registered in an earlier week whose process is still the same (so no new registry row is ever written) and whose sampling has stopped. Its registration is out of window so no gap fires; its store is still discovered and is still in `registeredPairs` so no reset-coverage gap fires either; it simply has no cohort entry. If any other session sampled normally the week passes. This is the accepted cost of the directed fix rather than a defect: the fold review brief instructs exactly this scoping, catching the case would need a last-seen time the registry does not record, and the published coverage statement already says "Calls the harness did not record are outside it" and "This report does not prove an independent inventory of all provider calls". Worth carrying to slice 3 beside registry retention. | `internal/steward/contextreport.go:355-369`; coverage statement `internal/steward/contextreport.go:53`; directed scope `artifacts/reports/codex-ccb-slice2b-part-c-fold-review-brief.md` ("registry-derived gap checks apply only to registrations first seen in the requested half-open week"). Probe A in the scratch copy, one healthy in-week session plus one continuing session whose last sample is 48 hours before the week and whose registration is 96 hours before it: `A pass=true samples=1 cohort=map[claude/healthy:1] failures=[]`. |
| C2-2 | medium | no | The second half of the fold's requirement, that every registration stays available as process evidence regardless of the window, holds but is unproven. Moving `registeredPairs[key] = true` below the new `continue` leaves the whole `TestContextReport*` suite green, yet on a real installation it would make every session whose registration predates the reported week fail with `reset coverage is unavailable`. The one new subtest cannot catch it because its two out-of-window rows have no discovered store, and the `unregistered explicit-session evidence` subtest has no registration at all. One subtest would close it: a discovered store whose only registration is out of window must not produce the reset-coverage clause. | `internal/steward/contextreport.go:356-360`; the new subtest `internal/steward/contextreport_test.go:326-347` seeds no store for `retired-before` or `started-after`. Mutation in the scratch copy (assignment moved below the filter): `go test -run 'TestContextReport|TestContextStatus' ./internal/steward` = `ok`. Correct current behavior confirmed by probe C: `C pass=true resetUnavailable=false failures=[]`. |
| C2-3 | medium | no | The F-3 rewrite deleted the only test that reached the unregistered-runtime branches. After the fold no test in `internal/steward` references `lookupContextReportRuntime`, and nothing exercises `runtime %s is not registered` for samples or markers. The fix itself was right, since the old subtest was mis-aimed, but it removed proof instead of adding a second case. The branches report coverage gaps and cannot corrupt output, and no brief clause names them, so this does not block. | Removed stub at `.../scratchpad/fold-delta-tests.diff` (old `contextreport_test.go:381-403`); live branches `internal/steward/contextreport.go:296-299` and `:321-325`; `grep -rn lookupContextReportRuntime internal/steward/*_test.go` returns nothing. |
| C2-4 | low | no | The fold's single `continue` also silences the neighbouring `registered runtime %s is not in the runtime registry` gap for any row first seen outside the week, because both clauses sit behind the same filter. A foreign or renamed runtime that registered in an earlier week is now invisible. Harmless today: discovery refuses an unknown runtime's store outright through `validateCallLocation`, so such a row can no longer suppress anything either. | `internal/steward/contextreport.go:358-364`; `internal/usage/sessions.go:129-131`. Probe D: `D in week pass=false failures=[coverage gap: registered runtime ghost is not in the runtime registry]`, `D previous pass=true failures=[]`. |
| C2-5 | low | no | Forward hazard for slice 3, not a part C defect: stem enumeration is now purely suffix-based, and slice 3's retirement record is specified as `<cursor>.retiring.json`, which ends in `.json`. Discovery would enumerate `claude-x.json.retiring` as a stem, fail to parse the record as a cursor and refuse the entire week report. Amendment 10.3 already puts teaching `CallSessions` about retirement inside slice 3, so this is a note to carry, not work here. | `internal/usage/sessions.go:80-99`; `readDiscoveredCallCursor` refusal path `internal/usage/sessions.go:167-172` via `validRequiredCursorNumber` (`internal/usage/cursor.go:412-415`, which refuses an absent member); record name and slice 3 obligation `artifacts/reports/ccb-8c10-slice2b-amendment.md:37` and `:41`. |
| C2-6 | low | no | The Linux coverage figure CCB-2-18 requires is still outstanding; the builder ran neither the whole-repository ratchet join nor a Linux execution, and `go-gate.sh --fast` carries no ratchet. Risk is small because neither `internal/usage` nor `internal/steward` has any platform-specific non-test source (the only build-tagged files are `internal/steward/cpu_time_unix_test.go` and `cpu_time_other_test.go`, and the unix one applies on Linux too), so the Darwin figures should transfer with 2.3 and 1.7 points of margin. It remains a seat obligation through `section/go-engine-gate` before the integrated candidate lands, and the brief forbids treating a Darwin measurement as Linux proof. | Floors `scripts/agents/coverage-ratchet.json:73,80` and `scripts/agents/coverage-ratchet-linux.json:73,80`; rule `artifacts/reports/codex-ccb-slice2b-brief-v2.md:259,261`; builder's own statement in the "Coverage measurement and ratchet" section of `artifacts/reports/codex-ccb-slice2b-result.md`. |

The first read's non-blocking items F-4 through F-9 are untouched by the fold and still
stand, except that `contextreport.go:377` is now covered. F-10 no longer applies in this
worktree: there is no `records/narrator-digest.log` change and no stray plans file here.

## What the fold did not weaken

Because the production delta is the two hunks above, every item in question 4 is either
byte-identical to the code the first read confirmed or directly re-checked:

- **Rows only through `usage.Calls`, recovery error propagated.** `contextreport.go:70-107`
  unchanged; `cmd/metasystem/context_verbs.go` and `context_verbs_test.go` unchanged, so
  `TestContextReportPropagatesRecoveryError` and its CLI twin still prove exit 1, the
  path-bearing error, byte-identical previous outputs and untouched damaged evidence.
- **Registry locking.** `CallRegistrations` unchanged; the lock is taken before `Lstat` and
  released through `defer` before any cursor lock is possible, and the release is now
  exercised by the new test (mutation above). Lock order in `WriteContextReport` is
  unchanged: registry first, then per-session cursors.
- **The section 7 cohort.** The cohort loop (`contextreport.go:290-315`) is untouched;
  exclusion is still by declared capability, not by runtime name. The new filter sits only in
  the registry-derived gap loop below the statistics.
- **The percentile over the retained set.** `contextreport.go:338-350` untouched; p95 index
  `(95*n+99)/100-1` and the maximum are still computed over `retained` only.
- **Exit codes and bounded output.** `runContextReport` unchanged: 2 for a missing flag, a
  stray argument or a bad `--week`, 1 for resolve or operational failure, 0 for a measured
  PASS and a measured FAIL alike, one line of two paths and a verdict.
- **Empty-week behavior.** `finishContextReport` unchanged; an empty week still publishes
  both files and fails on `the distinct per-call cohort is empty` at exit 0. With the new
  filter an empty week carrying only stale registrations fails on that one clause instead of
  a list of stale gaps, which is the intended improvement.
- **The `.tmp` relaxation is conformant.** The brief asks discovery to exclude "lock and
  temporary files" (`codex-ccb-slice2b-brief-v2.md:144`). It still does, structurally:
  `atomicfile` names temporaries `<base>.<random>.tmp` (`internal/atomicfile/atomicfile.go:119`)
  and locks are `<cursor>.json.lock`, so neither ends in `.json` or `.jsonl`. What the removed
  clause actually excluded was a lawful session, because `sessionNamePattern`
  (`internal/usage/calls.go:86`) admits dots, so the session `high.tmp` persists literally as
  `cursors/claude-high.tmp.json`. The widened subtest at `internal/usage/sessions_test.go:97-146`
  keeps `ignored.jsonl.lock`, `ignored.jsonl.123.tmp` and `ignored.json.123.tmp` excluded while
  admitting `claude/high.tmp`, and `internal/usage` is the only writer into those two
  directories.

## Part B carry and part D absence

- Part B is clean against `origin/main`: `git diff --stat origin/main -- internal/steward/context.go
  internal/steward/context_test.go internal/output` is empty.
- The computed diff touches exactly four tracked files, with deletions only in
  `internal/usage/cursor.go` (`2 9`), which is the narrow local `sessionRow` to exported
  `CallRegistration` swap with identical JSON tags, so the persisted registry shape is
  unchanged. The other three files are additions only (`35 0`, `106 0`, `1 0`).
- Nothing from part D appeared: `testing.json`, both `coverage-ratchet*.json`,
  `health-fixtures.sh`, `supervision-hook-fixtures.sh`, `validate-metasystem.sh` are absent
  from the diff, and `cmd/metasystem/context_cost_test.go` does not exist.
- `git status --porcelain` in this worktree is exactly the four modified tracked files and
  the four untracked part C files. No generated record line, no stray plans file, no
  `.claude/worktrees` sweep risk here.

## Verdict

**Fit to land.** Zero material findings. All three folded findings are genuinely closed:
the window scoping is correct at both UTC edges and still fails a session that belongs to the
week, the new `internal/usage` tests are real behavior tests that die under mutation, and the
named test now drives the real branch and asserts its failure clause. The fold touched two
lines of production logic and weakened nothing the first read confirmed.

Nothing must change before landing. Two cheap improvements are worth folding into slice 3 or
the next touch of this file rather than blocking on: a subtest pinning that an out-of-window
registration still counts as process evidence (C2-2), and a replacement case for the
unregistered-runtime branches the F-3 rewrite removed (C2-3). The seat still owes the Linux
ratchet figure through `section/go-engine-gate` before the integrated slice 2b candidate
lands (C2-6), and C2-1 and C2-5 should be carried to slice 3 with registry retention.

Material findings: 0. Recorded but not blocking: 6.
