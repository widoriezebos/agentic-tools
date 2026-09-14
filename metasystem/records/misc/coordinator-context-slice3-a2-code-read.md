# Independent code read: coordinator-context slice 3, unit A2

Worktree: `/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/.claude/worktrees/ccb-s3/metasystem`
Reviewed tree: working tree at `8dffd7af` (origin/main), which contains `e960a113` and unit A1.
Spec: `artifacts/reports/codex-ccb-slice3-brief-v2.md` lines 34 to 123.
Builder report: A2 section of `artifacts/reports/codex-ccb-slice3-result.md`.
Reviewer: independent read, did not write the change. Findings only, no edits, no adjudication.

## What the diff actually is

The task brief described "about 374 insertions". That is only the tracked half.
`git diff` shows 374 insertions and 3 deletions across nine files; the
destructive centre of the unit is in two **untracked** files that `git diff`
does not print:

- `internal/usage/retention.go` (824 lines, new)
- `internal/usage/retention_test.go` (489 lines, new)

374 + 1313 = 1687 additions, which matches the builder's stated count. Anyone
reviewing this unit from `git diff` alone would review none of the deletion
code. Recorded here so the next reader does not repeat that.

Scope against `e960a113` is otherwise clean: no `testing.json` change, no
coverage floor, no role text, no receipt row, no B/C/D work, no changes under
`plans/` or the agent control plane.

## Checks I ran

All from the worktree, no fixture bed, `METASYSTEM_BIN` never exported.

- `go build ./...` — exit 0.
- `go test -race -count=1 ./internal/usage/ ./internal/refusal/` — both ok
  (5.8 s, 2.2 s).
- `go test -race -count=1 ./internal/steward/ -run '^TestContext'` — ok.
- `go test -race -count=1 ./cmd/metasystem/ -run '^TestContext'` — ok.
- Five probe tests injected with `go test -overlay` (no repository file was
  created or edited; probe sources live in this scratchpad). Results are cited
  inline below.

## Findings

| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| F-1 | high | yes | `PruneCallSessions` puts no bound on `before`. A cutoff at or after now is accepted, and because every eligibility test compares against that same value, it deletes evidence recorded seconds ago and pushes the irreversible boundary past today. | `internal/usage/retention.go:67-127` has no check on `before`; row, marker and registration tests all compare to it (`retention.go:358`, `:399`, `:409`). Probe: seeded a session whose rows, mtimes and registration are all from now, called `PruneCallSessions(root, now+24h)` — `removed=1`, samples went 1 to 0, `retainedSince=2026-09-16T00:00:00Z`, so every week report through the current week is now refused. The unit's own tests make a future cutoff the normal case (`retention_test.go:18`, `:123`, `:219`, `:283`, `:347`, `:380`), so no future guard can be added without rewriting them. |
| F-2 | medium | yes | An empty orphan samples file, which holds zero evidence, advances the monotone retention boundary. Reportability of all history is lost in exchange for deleting a zero-byte file, and the boundary only moves forward. | `internal/usage/retention.go:97-108` gates boundary publication on `len(candidates) != 0`, and `callRetirementCandidates` puts an orphan into that slice (`retention.go:302`, `Orphan: true`). Probe: a root with one live pair plus one stale empty orphan gave `removed=1`, `retainedSince=2026-09-16`, and `ReadCallEvidence` still returned 1 sample — nothing with evidence in it was deleted. The builder's own subtest exercises exactly this shape (`retention_test.go:80-102`). Brief tension: `brief-v2.md:67` calls an orphan a "retired store" while `:77` gates the boundary on "at least one eligible pair". |
| F-3 | low | no | The brief-named proof `TestContextReportSerializesWithPrune` does not implement the barriers the brief names. It pauses at steward's `readContextCallEvidence` seam only after the whole snapshot has already been copied, so it never interleaves a prune with registry snapshot, discovery or a `Calls` boundary. | `internal/steward/contextreport_test.go:208-245` wraps `readContextCallEvidence`, calls the original in full, then blocks. The brief requires "Pause at registry snapshot, discovery and each `Calls` boundary" (`brief-v2.md:119`). Not material: `callEvidenceSnapshotStep` is package-private to `usage` and unreachable from `internal/steward`, and the substance is proved by the extra test `TestCallEvidenceSnapshotBlocksPruneAtEveryBoundary` (`internal/usage/evidence_test.go:228-330`), which does pause at registrations, sessions and each of the two rows boundaries and shows the prune blocked at every one. |
| F-4 | low | no | One case of the interrupted-deletion matrix proves nothing at its recovery step. | `internal/usage/retention_test.go:118` (`{"journal directory sync", "sync-3", "evidence"}`): the journal unlink already succeeded before the injected failure, so at `:197-200` the recovery assertion is `evidence.RetainedSince.IsZero()` being false, which the same prune's own boundary publication already guaranteed, and `:210-212` asserts the journal is absent, which was already true. The other seven cases carry the real proof. |
| F-5 | low | no | A test swaps a package-level seam outside `t.Cleanup` protection, so a failure inside it corrupts every later test in the package. | `internal/usage/retention_test.go:228-240`: `removeCallStorePath` is reassigned at `:230`, `t.Cleanup` is registered only at `:240` after the manual restore at `:239`. A `t.Fatal` at `:237` or `:241` leaves the seam swapped for the rest of the run. |
| F-6 | low | no | New concurrency proofs carry five-second wall-clock guards. | `internal/usage/retention_test.go:408`, `internal/usage/evidence_test.go:264` and `:288`, `internal/steward/contextreport_test.go:234`. These are barrier timeouts, not sleeps, so they satisfy `brief-v2.md:123`; they can still time out under the deep-battery load this repo runs. No new sleeps and no `t.Parallel` anywhere in the seam-swapping tests, as required. |
| F-7 | low | no | `retireCallSession`'s final under-lock recheck substitutes a synthetic registration, so the brief's step-4 registration guard is silently absent from the last check before deletion. | `internal/usage/retention.go:458` builds `[]CallRegistration{{..., FirstSeen: before.Add(-time.Nanosecond)}}` and feeds it to `inspectCallRetirementCandidate`. Safe today only because registration takes the maintenance lock shared (`internal/usage/cursor.go:312`) while prune holds it exclusively (`retention.go:74`), so no row can land mid-prune. Any future caller of `retireCallSession` outside that exclusive lock loses the guard, and no test would catch it. |
| F-8 | info | no | The retention boundary is derived from the caller's cutoff, not from what was actually retired, so retiring one ancient pair refuses every intact week between that pair and the cutoff. | `internal/usage/retention.go:101` uses `callRetentionBoundary(before)`. Probe: a pair whose newest row was `2023-09-15` pushed the boundary to `2026-09-16`. This is exactly what `brief-v2.md:77` mandates, so it is conformance, not a defect. Recorded as the honest answer to the seat's question 3. |
| F-9 | info | no | Once a journal's authorization can no longer be satisfied, store-wide reads refuse permanently and prune cannot clear it. | Probe: interrupted the retirement after the samples unlink, then wrote an out-of-band replacement cursor (the shape the builder's own `retention_test.go:345-375` constructs). `ReadCallEvidence` and `CallSessions` both returned `surviving cursor file identity does not match retirement authorization`, and a retry prune returned the same error. An unrelated session's own `LatestCall` health read still answered. Refusing is what `brief-v2.md:103` requires, and the class predates A2 (`TestContextReportPropagatesRecoveryError`). The remedy (remove the mismatched cursor, after which recovery clears the journal) is undocumented anywhere in the unit. |
| F-10 | info | no | A2's five new steward proofs and its new CLI proof are selected by no engine group, so green engine runs do not cover them. | `testing.json:51` `context-standard` is an explicit test list that names none of them; `testing.json:52` `context-foundations-standard` is `tests: "all"` but only for `internal/usage`, `internal/output`, `internal/runtimes`. A2 correctly leaves `testing.json` to unit D, so this is a sequencing obligation, not a scope breach. Separately observed: the three A1 test names A1's result says it admitted to `context-standard` are also absent from that list at this base. |

## Answers to the eight questions asked

**1. What is retired, in what order, and is every retirement recoverable.**
Order is: exclusive maintenance lock; recover every existing journal with a
whole-set preflight first; read and validate the boundary and the complete
registry; enumerate stems; per-candidate cursor lock, identity, both mtimes and
committed integrity; publish the durable boundary; publish the durable
`<cursor>.retiring.json`; then under the cursor lock remove samples, remove
cursor, sync the samples directory, sync the cursors directory, remove the
journal, sync its directory (`retention.go:67-127`, `:552-579`). The journal
carries dev, inode and a sha256 of each member plus `samplesBytes`, runtime,
session and cutoff, and carries **no deletion path** — targets are re-derived
from the identity and cross-checked against the journal's own stem
(`retention.go:507-550`, `:692`). Durability is checked explicitly and a
`(false, nil)` publication authorises nothing (`retention.go:655-668`, proved
at `retention_test.go:171-173`).

I walked every crash point and found none that leaves evidence a reader cannot
account for. The inode-plus-digest authorisation is what makes this hold: after
a crash where an unlink was not durable, the same inode reappears and recovery
re-unlinks; a *different* inode refuses. Recovery restores a state a reader can
trust rather than merely parse, because it runs before `loadCallCursor` and
before reconciliation in every reader (`cursor.go:70`, `cursor.go:242`,
`sessions.go:72`, `evidence.go:45`), so a surviving cursor with
`samplesBytes > 0` and a deleted samples file never reaches the reconciler.
This part of the unit is good work.

**2. Can a retirement remove a pair a live reader, the health role or an
in-flight report still needs; can it race an appending writer or a private-root
diagnostic.** No. Prune takes the store-wide maintenance lock exclusively
(`retention.go:74`); every append, registration, discovery and read takes it
shared first (`calls.go:124`, `cursor.go:226`, `cursor.go:312`,
`sessions.go:39`, `sessions.go:213`, `evidence.go:40`). So no writer can append
during a prune and no report can interleave with one — proved at all four
snapshot boundaries by `evidence_test.go:228-330`. The nonblocking health path
returns a typed `CallStoreBusyError` immediately rather than waiting
(`evidence_test.go:176-224`). A diagnostic on a private root is untouched
because every path in the unit derives from the passed `stateRoot`.

**3. Is the boundary computed from data or from wall-clock time.** Mixed, and
this is where the unit is weakest. Eligibility is genuinely data-driven — a
pair is kept if any committed row, marker or registration is at or after the
cutoff, if a row is undated, if the source is unexpected, if an invocation id
repeats, if the registration is missing, or if either member is damaged
(`retention.go:335-421`). Mtime is necessary but never sufficient, exactly as
the brief requires. But the cutoff itself is unconstrained (**F-1**) and the
boundary is the cutoff rather than the data (**F-8**). A slow machine does not
cause early retirement; a clock that jumps forward does, and it deletes today.

**4. Does a reader meeting a partially retired or missing pair answer honestly,
and does the week report still refuse to invent a figure.** Yes for every state
a correct prune produces. A fully retired pair keeps its registry row; the
missing-sample coverage rule is gated on the registration being *in the
requested week* (`contextreport.go:345`), and retired registrations are always
before the boundary, so the rule cannot fire for them. The reset count is
computed from registrations alone (`contextreport.go:440-470`), so the reset
prefix survives, proved at `contextreport_test.go:186-206`. A week before the
boundary is refused with the typed error *before* normalization and before any
publication (`contextreport.go:88-90`), the CLI prints exactly
`CONTEXT_EVIDENCE_RETIRED requested=<date> retained-since=<date>` and exits 9
(`context_verbs.go:110-114`), and prior `calls.jsonl` and `report.md` bytes are
proved unchanged (`contextreport_test.go:249-271`, `context_verbs_test.go:74-106`).
The empty-cohort FAIL rule is untouched, so no pruned historical week is
silently regenerated as PASS or FAIL. `TestContextPrunePreservesRetainedWeek`
deep-compares the whole report struct and the published bytes across a prune
for passing, failing and empty weeks — a real proof, not a shape check.

The one honesty gap is F-9, and it is the brief's own instruction rather than a
slip.

**5. Does anything touch shared or out-of-installation state.** No. This unit
is clean of the class the sibling unit was refused for. Every path is
`filepath.Join(stateRoot, "artifacts", "agents", "context", ...)`
(`retention.go:242-243`, `:283`, `:607`, `:699`, `:725-729`); `removeCallStorePath`
is only ever called on a cursor, a samples file, a journal or an orphan samples
file derived from a validated identity; `atomicfile.WriteText` is anchored at
`stateRoot` and `chain` stops there (`internal/atomicfile/atomicfile.go:146-168`).
No git file, no repository-wide file, no shared-checkout state, no temp-file
name that could be mistaken for a cursor or a journal. The registry is never
written and `sessions.jsonl.lock` is never removed, proved by byte comparison at
`retention_test.go:68-71`. Symlinked storage parents and symlinked members are
refused before any deletion (`retention.go:723-773`, `sessions.go:171`), proved
at `retention_test.go:303-343`.

**6. New refusal code.** Correct. `{Code: "CONTEXT_EVIDENCE_RETIRED", Owner:
"internal/steward", Site: "contextreport.go:59", Shape: Question}`
(`internal/refusal/register.go:46`). `contextreport.go:59` is the exact
`fmt.Sprintf` that emits the token. `Question` with no override matches the
other non-overridable data-unavailability rows (`TEST_CONTRACT_INVALID`,
`STOP_LOCATOR_UNAVAILABLE`). `internal/steward` is in the collector's walk list
(`register_test.go:124`), so the row is required rather than decorative, and
`internal/refusal` passes with `-race`. One row, no count assertion disturbed.

**7. Named tests, pre-change failure, tautology, load shape.** All eleven
brief-named tests exist plus one extra
(`TestCallEvidenceSnapshotBlocksPruneAtEveryBoundary`), and the retained
`TestContextReportPropagatesRecoveryError` is intact. On the pre-change tree
they fail as compile errors, since `PruneCallSessions`, `readCallRetention`,
`callRetirementPath`, `removeCallStorePath`, `syncCallStoreDirectory`,
`writeCallRetentionText`, `callStorePathHasValidCursor` and
`ContextEvidenceRetiredError` do not exist there — the normal and unavoidable
shape for new API, and the CLI proof would still fail behaviourally if it
compiled, because `retention.json` is an unknown file at base and the report
publishes. No sleeps. No `t.Parallel` in any seam-swapping test, as the brief
requires. Weaknesses: F-3, F-4, F-5, F-6.

**8. Scope.** Clean. Nothing from B, C or D: no handoff binding, no state
schema, no `context prune`, `handoff` or `verify` verb, no Stop allowance, no
role instruction, no shell fixture leg. `testing.json` byte-identical, no
coverage floor touched, no receipt row, no `plans/` or control-plane change.
`PruneCallSessions` has no production caller yet, which is correct for A2. The
test contract is not widened; see F-10 for the consequence.

## Verdict

**Not fit to land as it stands. Two material findings must be answered first.**

What must change:

1. **F-1** — bound the cutoff. `PruneCallSessions` must refuse a `before` that
   is not strictly in the past (and the seat should decide whether it also
   wants a minimum age), with one test that proves a future cutoff is refused
   rather than obeyed. The existing tests that pass `time.Now().UTC().Add(24 *
   time.Hour)` have to be re-based on a past cutoff with backdated members;
   that is a mechanical rewrite of `retention_test.go` and
   `evidence_test.go:234`, not a redesign. Alternatively the orchestrator
   refutes this by showing where a cutoff is already clamped for every present
   and planned caller, with the exact check and its observed result.
2. **F-2** — do not advance the retention boundary when the only retired store
   is an empty orphan. Either exclude orphans from the "at least one eligible
   pair" test at `retention.go:97`, or have the seat rule that
   `brief-v2.md:67` deliberately buys the boundary with a zero-byte file, and
   close this out-of-scope citing that line.

Everything else I found is recorded and should not block: F-3 through F-10 are
either taste, proof-strength notes, or the brief's own instructions. The
journal protocol, the inode-plus-digest authorisation, the lock ordering, the
conservative eligibility rules and the state-root confinement are all sound,
and the crash matrix holds at every step I could construct.
# Closing read: coordinator-context slice 3, unit A2, after the F-1/F-2 fold

Worktree: `/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/.claude/worktrees/ccb-s3/metasystem`
Reviewed tree: working tree at `8dffd7af`, nine tracked modifications plus untracked
`internal/usage/retention.go` (837 lines) and `internal/usage/retention_test.go` (540 lines).
First read: `artifacts/reports/opus-read-a2.md`. Fold record: the "Read fold round" section of
`artifacts/reports/codex-ccb-slice3-result.md`.
Reviewer: same independent reader, did not write the change. Findings only, no edits, no adjudication.

## What the fold actually is

The fold's whole footprint inside `retention.go` is two hunks:

1. `retention.go:70-73`, the cutoff guard at the top of `PruneCallSessions`.
2. `retention.go:100-113`, the `hasPair` gate that now wraps boundary publication.

Plus two new tests (`TestPruneCallSessionsRefusesFutureCutoff`,
`TestPruneCallSessionsDoesNotAdvanceRetentionForEmptyOrphan`) and a rebase of every existing
cutoff from `time.Now().UTC().Add(+24h)` to a past value. Nothing else in the unit moved.

## Checks I ran

All from the worktree. No repository file was created or edited; every probe and every
pre-fold or mutant source lives in this scratchpad and was injected with `go test -overlay`.
`METASYSTEM_BIN` unset for every command, `GOCACHE=/tmp/codex-ccb-s3-cache`. No fixture bed.

- `go test -race -count=1 ./internal/usage/ ./internal/refusal/` - both ok (5.8 s, 2.2 s).
- `go test -race -count=1 ./internal/steward/ -run '^TestContext'` - ok.
- `go test -count=1 ./cmd/metasystem/ -run 'Context'` - ok.
- Five cutoff-edge probes against the folded tree (results inline below).
- A reconstructed pre-fold `retention.go` (both hunks reverted) run against the two new tests.
- A mutant `retention.go` (newer-member rule reduced to the cursor mtime alone) run against
  the rebased `TestPruneCallSessionsUsesTheNewerMemberAge`.
- `git status --porcelain --untracked-files=all` before and after: unchanged.

## Findings

| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| G-1 | high | yes | The cutoff guard does not refuse a cutoff of `time.Now()`. It samples the clock *inside* the function, after the caller has already computed its cutoff, so any cutoff at or just before the present instant passes and reproduces the whole of F-1: today's evidence deleted and the irreversible boundary pushed into the future. | `retention.go:70` reads `currentTime := time.Now()` and `:71` accepts any `before` strictly earlier than that later sample. Probe: one pair whose rows, both mtimes and registration are all one hour old, then `PruneCallSessions(root, time.Now())` returned `removed=1, err=<nil>`; cursor and samples were gone and `retention.json` held `retainedSince=2026-09-15T00:00:00Z`, a boundary after the current instant, so the current week's report is now refused and the boundary only moves forward (`retention.go:106-108`). `time.Now().UTC()` gave the identical result, as did `now-1ns`. The edge is also a race with clock granularity, not a rule: `now+1ns` returned `removed=1, err=<nil>` in one run and was refused in another. `TestPruneCallSessionsRefusesFutureCutoff` (`retention_test.go:105-121`) exercises only `now+24h`, so it cannot tell this guard apart from the intended one, and the result report's line "refuses a cutoff that is not strictly before the current time" reads as covering `now` when it does not. Artifact that would change: `internal/usage/retention.go` (a caller-independent bound, not a clock sampled after the caller's) and `internal/usage/retention_test.go` (a case at exactly `now`). |
| G-2 | medium | no | The boundary is published when a real pair is *eligible*, not when one was *retired*, so a failure after publication still advances it with nothing deleted, and the caller is not told the boundary moved. | `retention.go:100-113` publishes before the retirement loop at `:115`. Probe of the journal-name collision shape the unit's own subtest builds (`retention_test.go:303-327`): `removed=0`, error `call retirement journal path collides with an existing cursor or journal: ...`, both pairs still on disk, and `retention.json` present with `retainedSince=2026-09-14T00:00:00Z`. Nothing was retired and every week before that date is now permanently unreportable. Not material: `brief-v2.md` states "A later failure can conservatively advance the boundary without deleting a pair; report that partial outcome", and "Do this only after finding at least one eligible pair", so both the ordering and the eligible-not-retired gate are the brief's own instruction; `removed` is the brief's partial-outcome channel ("Return a partial count with an operational error") and it accurately reports zero retirements. Recorded because the seat's own summary of the fold says "publishes the boundary only when a real pair was retired", which is not what the code does. The result report's wording ("requires a real cursor/sample pair candidate") is accurate. |
| G-3 | low | no | A zero cutoff is accepted silently instead of refused. | Probe: `PruneCallSessions(root, time.Time{})` returned `removed=0, err=<nil>`, left both pair members on disk and published no `retention.json`. It is a safe no-op, because every eligibility test is `X.Before(before)` and `callRegistrationsAllowRetirement` (`retention.go:353-365`) returns false for any registration not before year 1. No evidence is lost and no boundary moves, so not material; noted only because a caller bug returns success here while the future-cutoff bug returns an error. |
| G-4 | low | no | A backwards clock step is undefended, and largely cannot be defended at this layer. | `currentTime` is sampled once at `retention.go:70` and never re-read; every later decision compares the fixed `before` against file mtimes and row timestamps, so a step backwards after the guard cannot widen the prune's blast radius. The exclusive maintenance lock (`retention.go:77`) blocks every append for the whole prune, so nothing can be written into the window. The residual is evidence recorded before the prune but after a backwards step, whose timestamps look older than they are; that is inherent to any timestamp cutoff, is not introduced by the fold, and is narrowed because eligibility requires the registration, every committed row and both mtimes to agree (`retention.go:335-421`). |
| G-5 | low | no | The equality case pins a filesystem mtime to nanosecond-exact equality with the cutoff. | `retention_test.go:30` sets the `equal` pair's samples mtime to exactly `before` and `:59` fails if it retires. On a filesystem that truncates mtime below nanosecond resolution the stored value falls strictly before the cutoff and the pair retires. It passes here on APFS and the shape is pre-existing rather than introduced by the fold, but it is worth knowing before the Linux coverage run. |

The first read's F-3 through F-10 all still stand exactly as recorded, and all remain
non-material. The fold touched none of the code they cite.

## Answers to the six questions asked

**1. Is the cutoff bound correct at its edges.** No at one edge, yes at the others.

- *Exactly now*: not correct. A caller passing `time.Now()` is accepted, because the guard
  compares the caller's value against a clock sampled later, inside the function. See G-1.
  The same holds for `time.Now().UTC()`, and the accept/refuse decision for values a few
  nanoseconds ahead is nondeterministic rather than ruled.
- *One nanosecond in the past*: accepted by design, and it still deletes evidence recorded
  seconds ago and sets `retainedSince` to tomorrow's UTC midnight. This is the minimum-age
  question the first read flagged and the fold did not answer. On its own I would call it a
  policy gap rather than a defect; combined with G-1 it means the harm F-1 described is still
  reachable from a plausible caller.
- *Clock moving backwards between the check and the eligibility tests*: no new exposure. See G-4.
- *Zero time*: accepted, and a complete no-op. See G-3.

Gross future cutoffs are genuinely closed. `now+24h` is refused reliably, `removed=0`, no
boundary published, both members intact, and the refusal happens before the maintenance lock
and before any store access (`retention.go:70-77`).

**2. Does the refusal strand any legitimate caller, and is it honest.** It strands nobody. Any
past cutoff retires normally, the guard sits above `validateCallStorageParents` and above the
lock so a refusal mutates nothing, and the probe confirms `removed=0` with no `retention.json`
written. There is no production caller yet; Unit D owns the verb. The reporting is honest in
shape - a plain Go error naming both the cutoff and the sampled current time, returned with a
zero count, no refusal token needed because no token is emitted. The one honesty problem is that
the message promises a rule ("must be before current time") that the code does not enforce at
`now`. The risk here is the opposite of over-refusal: the guard is too weak, not too strong.

**3. Is the boundary published exactly where evidence was retired, and can a partial failure
publish it.** The orphan half of F-2 is properly fixed: an orphan-only prune now deletes the
zero-byte file, leaves `retention.json` absent and leaves all history reportable, proved by
`TestPruneCallSessionsDoesNotAdvanceRetentionForEmptyOrphan` and by its failure on pre-fold code.
But the gate is on an eligible non-orphan *candidate*, not on a completed retirement, and
publication still precedes every deletion, so yes, a partial failure can publish the boundary
with nothing deleted. I reproduced it. See G-2. That ordering and that gate are both what
`brief-v2.md` instructs, so I do not treat it as a defect, but the seat should know that its own
one-line summary of the fold is stronger than the code.

**4. Were the rewritten tests weakened, and do they fail on pre-fold behaviour.** Not weakened,
and both new tests are genuine regression proofs.

- Against a reconstructed pre-fold `retention.go` (guard removed, boundary published
  unconditionally), `TestPruneCallSessionsRefusesFutureCutoff` fails with
  `future cutoff removed=1 err=<nil>` and
  `TestPruneCallSessionsDoesNotAdvanceRetentionForEmptyOrphan` fails with
  `RetainedSince:2026-09-14 00:00:00 +0000 UTC` beside untouched live evidence. Neither is a
  tautology.
- The rebase of `TestPruneCallSessionsUsesTheNewerMemberAge` changed only the base of the clock
  arithmetic. All six retained cases (fresh cursor, fresh samples, equal, fresh row, fresh
  marker, fresh registration), the retired case, the orphan, the byte-identical registry
  comparison, the boundary equality and the stable lock inode check are all still asserted.
  It still kills a mutant that drops the samples half of the newer-member rule: `removed=4`
  against an expected 2.
- The eight-case interrupted-deletion matrix, the damage and symlink proofs, the
  replacement-generation proof and the lock-inode proof are unchanged apart from the cutoff base
  and all pass under `-race`.
- One test name overreaches: `TestPruneCallSessionsRefusesFutureCutoff` proves refusal only at
  `now+24h`, which is why G-1 survives a green run.

**5. Was anything the first read verified disturbed.** No. The journal shape and its
`samplesBytes`/dev/inode/digest authorization, the whole-journal-set preflight in
`recoverAllCallRetirements`, the re-publication of both authorizations before any unlink in
`recoverCallRetirement`, `completeCallRetirement`'s samples then cursor then both directory syncs
then journal then journal-directory-sync order, the exclusive maintenance hold over the entire
prune, the four reader recovery hooks in `cursor.go`, `sessions.go` and `evidence.go`, the single
`CONTEXT_EVIDENCE_RETIRED` register row, the typed refusal before normalization and the exit-9
CLI presentation with byte-preserved prior outputs are all exactly as the first read left them.
The refusal rows are unchanged and `internal/refusal` passes under `-race`.

**6. Scope.** Clean. `git status --untracked-files=all` shows the nine tracked modifications plus
the two untracked retention files and nothing else. `testing.json`, `plans/` and the agent
control plane are untouched. `metasystem/artifacts/` is gitignored at `.gitignore:1`, so both
report files sit outside the change. No unit B, C or D work, no coverage floor, no receipt row,
no role text, no fixture leg.

## Verdict

**Not fit to land. One material finding stands: G-1, the cutoff guard does not refuse a cutoff of
`time.Now()`, so the F-1 harm is still reachable.**

The F-2 half of the fold is correct and proved. The F-1 half narrowed the hole from any future
cutoff to a cutoff at or within call overhead of the present instant, but did not close it, and
the test that certifies it only exercises a cutoff a day ahead. Everything else the first read
verified is intact, and G-2 through G-5 should not block.
