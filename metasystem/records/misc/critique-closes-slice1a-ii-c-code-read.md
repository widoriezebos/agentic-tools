# Independent code read: critique-closes slice 1 part 1a-ii unit c

Reader: Opus 5 (1M), seat-dispatched, not the builder.
Worktree: `/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/.claude/worktrees/ccf-s1aii-c/metasystem`
Base: `origin/main` at `8e92a796` (carries units a and b).
Reviewed surface: `git diff 8e92a796` (3 files, 176+/29-) plus the two untracked test files.
Method: code read only. No test run, no fixture bed, no repository file edited.

Materiality question applied verbatim to every finding: "Would the change ship a defect, violate
its brief, or damage what certifies it?"

## Findings

| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| F-1 | medium | yes | The A-5 named proof does not exercise the retained-proof verification path the brief named. `TestRecertificationRejectsStaleClosureRound` asserts the second refusal through `verifyOriginalCritic`, which only refuses because the builder added a duplicate `selectOriginalCertification` call four lines after the caller already selected. The real retained-proof gate, `VerifyRecertification`, is the last gate before a base-moved commit and is untested for stale, changed, wrong-root and missing-closure cases. | `internal/validate/recertification_closure_test.go:190`; the duplicate call at `internal/validate/recertification.go:603`; the caller's own selection at `internal/validate/recertification.go:746` immediately before `:750`; the brief's named site `internal/validate/recertification.go:1119` at base, now `:1176`; the base-moved landing lane that reaches it at `internal/landing/observe.go:380` |
| F-2 | low | no | The re-selection inside `verifyOriginalCritic` is an unrequested production addition. The brief said verification "must repeat this selection, **as it already does**", i.e. no new code was owed. It doubles the full job-record and artifact read on the recertification path, and under a concurrent record change it reports a selection failure as `chain-recertification-unproven` / `critic-closure` instead of `chain-recertification-source-changed` / `review-selection`. | `internal/validate/recertification.go:602-609` against `internal/validate/recertification.go:746-752` |
| F-3 | medium | no | Lawful absence keeps the old **order** but no longer the old **tolerance**. A present but unreadable stamped critic record, an invalid round, an unreadable or malformed stamped return, or a return without a `reviewedTree` are now hard refusals; at base they returned `""` and fell through to the terminal-implementer and highest-round selectors. The first two of these now fire before the single-output shortcut, so a sole-output chain that landed at base can refuse. | new `internal/landing/observe.go:666-705` and `:707-735` against base `observe.go:619-648`; the shortcut it now precedes at `internal/landing/observe.go:601` vs `:622`; D6's "existing order" sentence in `plans/critique-closes-on-folded-proof-design.md:201` |
| F-4 | low | no | Dead branch. `closedCriticReviewedTree` tests `record["jobId"] != critic`, but `stampedCriticChain` only admits a record whose own `jobId` is the map key and matches its filename, so the branch is unreachable. | `internal/landing/observe.go:712` against `internal/landing/observe.go:689-694` |
| F-5 | low | no | `internal/refusal/register.go` needs no new row (no new code was introduced), and its stale `internal/landing` site lines are not newly caused here. They were already wrong at base (`observe.go:577` is a review unmarshal, `observe.go:602` an outputs loop) and this change shifts `observe.go` by about +63 lines after line 435, moving them further. Nothing validates `Site`. | `internal/refusal/register.go:94-108`, `:119-125`; base lines checked in `git show 8e92a796:metasystem/internal/landing/observe.go`; `internal/refusal/register_test.go:20,51,71` check codes and overrides only |
| F-6 | low | no | The new `Detail` on `chain-output-unreadable` can carry a wrapped `os.PathError`, so a machine-absolute filesystem path can reach the durable landing observation. | `internal/landing/observe.go:438-440` consuming `internal/landing/observe.go:724` (`%w` of the `os.ReadFile` error); `Detail` wire field at `internal/landing/observe.go:80` |
| F-7 | low | no | Cost. On the absence path every `artifacts/agents/jobs/*.json` is now globbed, read and JSON-parsed three times per landing observation instead of twice. | `internal/landing/observe.go:601` -> `:675`, `internal/landing/observe.go:625` -> `:675`, and the pre-existing `internal/landing/observe.go:738` |

## Answers to the eight named checks

**1. D6 in landing.** Correct on every clause.

- The closure is read and validated at `internal/landing/observe.go:601`, above the `len(outputs) == 1`
  shortcut at `:622`. A present-but-invalid closure returns the error immediately at `:602-604`.
- Selection at `:613-617` requires `output.reviewedTree == closure.Subject.ReviewedProjectTree`
  **and** `landingPatchDigest(output.patch) == closure.Subject.DiffDigest`. A tree match with a
  different patch does not satisfy the conjunction, falls out of the loop, and refuses at `:619`.
- The closure branch returns on every path inside `if closurePresent`, so there is no fallback after
  a present-closure mismatch.
- Deterministic order on equal bytes: `sort.Slice` ascending by round at `:612`, first match wins.
  Round directory names are unique, so the unstable sort cannot matter. Provenance
  (`Subject.ReviewedMember`) is correctly not consulted.
- The digests are compatible. `landingPatchDigest` is `%x` over `sha256.Sum256(diff.patch)`
  (`observe.go:647-650`); `liveReadSubject` is `hex.EncodeToString(sha256.Sum256(patch))` over the
  same file (`internal/dispatch/read_subject_compute.go:141,157,163`). Recertification uses
  `sha256Bytes` (`internal/validate/recertification.go:115-118`), the same encoding.
- Kind and chain are bound: `:606` refuses a non-live subject, `:609` refuses a closure naming
  another implementation root.
- Lawful absence keeps the order exactly (stamped return tree at `:625-634`, terminal implementer,
  then lowest round). It does **not** keep the old tolerance for unreadable stamped evidence: see F-3.
  The brief's "Do not discard read or validation errors in an empty string" sanctions that, and
  `stampReviewReference` only ever writes a nonempty job id
  (`internal/dispatch/review_reference.go:305`), so the null and empty stamp refusals have no lawful
  producer. The UC-F1 correction is sound.

**2. Patch replay and postimage comparison.** Unchanged. `bindCertifiedChange`
(`internal/landing/observe.go:806-851`) is byte-identical to base: it applies `output.patch` to the
current `HeadTree`, re-derives the changed paths, compares the postimage entries against
`output.reviewedTree` over exactly those paths, then compares the candidate. Still correct after a
base move, because the closure branch only changes *which* (tree, patch) pair is handed in, and it
hands in a matched pair. `output.implementerJob` is used only by the absence-path selector
(`observe.go:638`), never by replay, so the lower-round pick on equal bytes is inert.
`TestLandingRejectsStaleCriticClosure` covers the moved-base positive and the postimage-drift
negative (`internal/landing/observe_closure_test.go:257,273`).

**3. Recertification.** Correct on every clause the brief names.

- Chain join: `closure.Subject.ImplementerRoot == r.rootJob` (`recertification.go:378`), the requested
  member rooted in the same chain (`:381`), and the closure's reviewed member rooted there too (`:385`).
- Exact subject: the candidate filter requires `review.ImplementerJob == r.job`,
  `review.ReviewedTree == closure tree` and `sha256(patch) == closure.Subject.DiffDigest`
  (`:456-457`, `:465`). Selection is by complete identity, not by name.
- The critic return under the closure round must itself carry the closure tree (`:417-420`), which is
  a second, independent read of the binding `ReadClosedClosure` already checked.
- Artifact hashes, proof owners and exact readback are untouched (`:483-485`, `:887-896`, `:700+`).
- Duplicate or copied review: `:476-482` still refuses `conflicting original review copies`, and the
  new test drives it at `internal/validate/recertification_closure_test.go:128-135`.
- Lawful absence is the verbatim old path in the `else` arm (`:394-407`), including the exact
  `reviews == r.job` comparison and `terminalMember`.
- `r.criticRoot` is still the required original critic for `verifyOriginalCritic` (`:612`).
- `certification.criticRound` becomes the closure round on the closure path. That equals the highest
  critic round by `ReadClosedClosure`'s own check (`internal/readsubject/closure.go:198`), so
  `VerifyRecertification`'s `CriticTerminalRound` comparison (`recertification.go:1185`) stays sound.

**4. Bar (a) wording.** The inserted sentence is byte-for-byte the brief's text and sits at the end of
bar (a) check 2, the "critique completed AND concluded clean" item, immediately after the
hazard-minimums sentence (`plans/two-bars-for-changes-design.md:227-230`, item 2 beginning at `:206`).
It matches D6 (`plans/critique-closes-on-folded-proof-design.md:201`) and does not overreach: it
scopes itself to "a prior clean read" and explicitly returns historical reads and lawful non-clean
decisions to their existing requirements. Four added lines, no other edit to the page.

**5. False refusals.** I weighted this hardest. I found no false refusal of a legitimate landing or
recertification.

- The design-critic hazard worry is closed: `independentCritiqueJobRef` can name a design-critic
  (`internal/dispatch/review_reference.go:161-163`), and landing now refuses a non-live closure
  subject (`observe.go:606`). But landing's chain lane requires DESIGN-BEARING or DESTRUCTIVE-REACH
  (`observe.go:370-373`), both of which carry `IndependentCritiqueRequired: true`
  (`internal/dispatch/hazard.go:45,50`), so hazard already refused such a chain at close with the
  same live-subject test (`internal/dispatch/hazard.go:392`). It cannot reach landing `chainClosed`.
- The `CleanRegister` strictness worry is closed: dispatch's own decoder already enforces exactly 7
  or exactly 13 keys (`internal/dispatch/finding_register.go:778`), and the writer emits only the
  13-key form (`:842-856`), so `readsubject.CleanRegister`'s `hasExactFields` adds no new refusal
  shape for real registers.
- The pruned-record worry is closed: reaping flips status, it does not delete records
  (`internal/dispatch/reapfacts.go`, `internal/dispatch/record_test.go:315`), and `artifacts/` is
  local to the landing machine.
- Refusals that a completed, clean, subject-bearing fold without a closure now produces, at landing
  and at recertification, are D3 by design, not false refusals
  (`internal/readsubject/closure.go:309`; `plans/critique-closes-on-folded-proof-design.md:195`).
- Residual widening is F-3, and no test covers those specific branches.
- One latent hole is inherited, not introduced: membership is assembled by lineage
  (`observe.go:698-703`, `recertification.go:360-365`), so deleting an intermediate critic record
  hides every round above it from the gate. Same exposure as unit b's `chainMembers`. Evidence
  deletion under `artifacts/` is outside this brief's threat model.

**6. Refusal codes and texts.** No new code. Landing keeps `chain-output-unreadable` (`observe.go:438`)
and `chain-output-mismatch` (`:444`), as the brief requires. Recertification's new errors are plain
errors funnelled into the existing `chain-recertification-source-changed` / `review-selection` and
`chain-recertification-unproven` / `critic-closure` families (`recertification.go:748`, `:751`).
`internal/refusal/register.go` therefore needs no row. Its `internal/landing` `Site` lines are stale,
but were stale at base and are not validated by anything: see F-5. `SUBJECT_MISMATCH`
(`internal/refusal/register.go:74`) points at `internal/dispatch/read_subject_compute.go:242`, which
unit c does not touch.

**7. Red on the pre-change consumers, and tautology.** I re-derived the reds against base
`8e92a796` by reading the base source; they hold.

- `TestChainCertifiedOutputPrefersClosure`: base has two outputs, takes
  `closedCriticReviewedTree` -> the critic root's round-1 return -> `oldTree` -> round 1. Test wants
  round 2. Red.
- `TestChainCertifiedOutputRejectsUnmatchedClosure`, both unmatched subtests and both stamp subtests:
  base has one output and returns it through the shortcut with `err == nil`. Red.
- `TestLandingRejectsStaleCriticClosure` / later critic member: base shortcut passes the chain. Red.
- `TestRecertificationUsesClosureForNoOpMember`: base compares `criticRecord["reviews"]`
  (`implementation`) to `r.job` (`implementation-r2`) and errors
  (base `recertification.go:359`). Red.
- `TestRecertificationRejectsStaleClosureRound`: the valid-setup precondition fails on base for the
  same reason. Red.
- Declared guards that are green on base and correctly labelled as guards: the historical-absence
  fallback, the moved-base replay, and the postimage drift case.
- Tautology: one assertion is close to it. `recertification_closure_test.go:190` asserts that
  `verifyOriginalCritic` fails with the same substring the line above already asserted for
  `selectOriginalCertification`, and it does so only because `verifyOriginalCritic` now calls
  `selectOriginalCertification` itself. That is F-1. Every other assertion observes real selection
  output or a real public observation verdict.

**8. Scope.** Clean. Exactly `internal/landing/observe.go`, `internal/validate/recertification.go`,
`plans/two-bars-for-changes-design.md`, and the two named untracked test files. No existing test file
was modified or weakened. Nothing from part 1b: no `cleanReadRounds`, no read-admission, no
`CONCURRENT_READ`. No writer or authority change: `CritiqueChainClose` and the finding-register
writer are untouched. No fixture, no `testing.json`, no coverage ratchet, no role text, and the only
design-page edit is the four-line bar (a) amendment.

## Verdict

**Not fit to land as returned. One material finding.** The implementation itself reads correct against
D6, D1, D3 and 9c.10, and I found no behavior defect and no false refusal. What must change before
certification is proof, not code: add a case that drives `validate.VerifyRecertification` (the
`recertification.go:1176` selection, the real last gate for a base-moved commit at
`internal/landing/observe.go:380`) through a newer critic member, a changed patch digest, a wrong
closure root and a missing modern clean closure, and assert the `review-selection` refusal there.
While that case is being written, either drop the unrequested duplicate `selectOriginalCertification`
at `recertification.go:603` or state why it is kept, since the current test's second assertion only
observes that wrapper.
