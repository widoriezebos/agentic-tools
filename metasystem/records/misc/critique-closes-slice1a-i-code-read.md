# critique-closes slice 1a-i: independent code reads (Opus)

Three rounds, built by Codex gpt-5.6-sol from Fable briefs; each read by Opus. Round 1 and round 2 reads found material findings that the next round folded; the round 3 read found none.

---

<!-- source: ccf-s1ai-opus-read.md -->
## ccf slice 1a-i: independent code critique (Opus)

Tree: ccf-s1ai-tip worktree, unstaged diff on ff6582db (10 files). Probes ran on scratch copies of the module (changed tree and `git archive HEAD` base), under scratchpad/opus-probe. The worktree was not edited. Focused race tests pass: dispatch, adapter and refusal. vet and gofmt are clean. staticcheck is not installed here.

## Material findings

| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| F-1 | high | yes | The design-subject fixture calls a helper that the fixture script does not define. The dispatch-e bed exits 127 at the close-race assertion. | `scripts/agents/dispatch-fixtures.sh:3327` calls `sha256_file`. The only definitions are in `dispatch.sh:160` and `commit.sh:346`, and neither is sourced by the fixture script, which runs `set -euo pipefail` (line 2). I checked the behaviour: `bash -c 'set -euo pipefail; x=$(sha256_file f)'` gives rc=127. The brief used that helper name; the fixture needs `"$engine" util sha256 --file`. |
| F-2 | high | yes | A close attempted while a later round is still running writes the closure for the earlier round. After that, every close of the chain is refused. | `dispatch.sh` close runs `critique-register-close` (2902) before `close-check` (2904), and a follow-up releases the chain lock after launch (1956). `writeCleanClosure` (`finding_register.go:587-636`) never checks that the folded round is the chain's last round. Probe F: round 1 is folded clean and round 2 is running. Close writes closure round 1. Round 2 then completes, bound and clean, with an identical subject digest. The next close errors with "already carries closure round 1 ... refusing to replace it with round 2" (line 611). No command recovers the chain. |
| F-3 | high | yes | For a folded round that is not a clean read, `writeCleanClosure` returns an error where it should write no closure. `dispatch.sh close` then fails under `set -e`, while the base tree closes the same chain. | The `return writeCleanClosure` at line 544 propagates the errors at 618 ("was not completed") and 630 ("not bound"). Probe B, a cancelled round with a clean register: base closes, change errors. Probe C, a failed round whose synthetic finding a human accepted: base closes, change errors. Probe D, an unbound return whose finding a human accepted: change errors. Once the budget is exhausted there is no follow-up round, so the human's accept-risk verb cannot close the chain. `read_subject_compute_test.go:384` asserts the cancelled-round error as intended behaviour. Applying the brief literally would be worse: it would write a "clean" closure over a read that never happened. These cases need `return nil`. |
| F-4 | medium | yes | A second close after a deferral writes a `mechanism: clean` closure over the deferred bounded findings. | The deferral turns open findings into `deferred`, so the unresolved set is empty on the next close and `writeCleanClosure` writes the closure. Probe E, a register holding only deferred findings with a bound completed round: change writes `closure{round 1, clean}`, base writes none. `dispatch.sh close` has no `chainClosed` guard, so a retry after a failed close-check, or a repeat close, reaches this. The brief (section 4) says deferred outcomes write no closure. Spec section 2 reserves closing a folded bounded finding without a new read for the certification goal. |
| F-5 | medium | yes (the gap is in the brief) | A critic follow-up after the implementer changed its worktree is refused with `SUBJECT_MISMATCH` every time. That removes round 2 of an ordinary code-critique chain. | The follow-up subject uses the root record's `reviews` (`read_subject_compute.go:35`). That record keeps its own round (`:70`): `dispatch.sh:2464` and `build.go:874` copy `reviews` onto follow-ups. The implementer worktree has since moved to the round-2 tree. Probe A: implementer round 1 at T1, round 2 at T2, then `ComputeReadSubject{RootJob: critic}` returns `SUBJECT_MISMATCH` (recorded T1, observed T2). Today's merge (`conformance.go:1008+`, the final critic round's tree must equal finalTree) and `docs/orchestration.md:57` ("counts each follow-up round") treat critic follow-ups as the path for later rounds. The refusal's remedy works only for a fresh chain. Spec section 3 says "a follow-up whose base moved gets a new subject". No fixture covers this, and `TestLiveSubjectFollowsTheChangeNotTheMember` passes `Reviews` explicitly instead of taking the `RootJob` path that dispatch uses. |

Material-finding count: 5

## Non-material findings

| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| N-1 | low | no | The "fold-time binding held" assertion in `subject-persisted` cannot fail: the fake adapter copies the tree into the return, and no fold runs. | `dispatch-fixtures.sh:2839-2845`; the brief prescribed this assertion. |
| N-2 | low | no | A critic dispatched with `--worktree` that is refused for a subject mismatch leaves its worktree and branch behind. A retry with the same derived job id dies with "job worktree already exists". | `dispatch.sh:1643-1650` runs before 1684. The design check already had this ordering. The critic permission envelope is read-only, so worktree mode is only used when asked for. |
| N-3 | low | no | A design subject binds the workspace HEAD from before claim. In a shared checkout, a commit that lands while the critic reads makes a clean read unbound, and with F-3 that chain cannot then close. | This follows from the spec (F-3). |
| N-4 | low | no | A follow-up on a design root that has no `design` field now refuses with "design path must begin metasystem/". | The field dates from f12c5aa6 (2026-09-06). |
| N-5 | low | no | The dedicated-field registration is in an `init()` in `finding_register.go` rather than in the map in `record.go`. | Placement only. |

## Layer A: the builder's stated decisions

- **Follow-ups read `reviews`, `design` and `declaredOutputsDigest` from the root record:** within the brief (section 1). F-5 is the consequence for live subjects.
- **`SUBJECT_MISMATCH` as OpError code 11:** within the brief. `recordExit` prints `reason=SUBJECT_MISMATCH` and exits 11. The register row's site, `:187`, is the `return &OpError{` line.
- **`findingRegisterSubjectDigest` stored at fold:** goes beyond the brief but stays within the spec. It refuses a subject changed after the fold, and the test at lines 342-365 proves it on a diff-digest-only change.
- **RecordCAS guard for `closure` and the digest:** goes beyond the brief, protective, within the spec. The patch-keyed refusal is atomic (tested).
- **Mode 0644 on the subject file:** within the brief. The explicit chmod is needed because `mktemp` creates the file 0600.
- **Fixture restores the file by bytes:** correct. `review-target.txt` is untracked (created by `printf`, never committed), so the brief's `git checkout --` would fail under `set -e`.

## Layer B: checks that held

- **Order of work:** on both paths the subject is computed before claim (`dispatch.sh:1684-1693`, `2580-2589`) and published only after the claim wins (`1855-1858`, `2760-2763`).
- **Temp cleanup:** the `EXIT` traps (1634, 2336, 2594) delete the temp through the global `exit_cleanup_subject`, read as `${…:-}`, which is safe under `set -u`.
- **Refusal leaves nothing behind:** no record or payload directory exists before 1684. For critic roles, the fold and exhaustion steps that run earlier in a follow-up only read, or are idempotent.
- **Workspace check:** it uses the same projection as the review stage, `projectInstallPrefix(root)` plus `Snapshot("HEAD")` (`conformance.go:244/299/367`). A missing worktree is refused by name.
- **Fold of an unbound return:** it adds one idempotent synthetic unproven finding and still consumes a round. A cancelled fold stays neutral.
- **Old rounds without `subject.json`:** they fold as today and get no closure.
- **Fake adapter:** it reads only the `subject.json` in the round's own directory, and that file is published only after the workspace check passed. Its echo cannot bind a workspace that failed the check. It cannot see drift after launch, which the spec accepts.

## Verdict

Not fit to land by human commit. Five material findings: F-1 fails the dispatch-e bed; F-2 and F-3 leave chains that can never close; F-4 writes a false clean closure; F-5 blocks round 2 of an ordinary code critique, and its fix belongs in the brief.

---

<!-- source: ccf-s1ai-opus-read-r2.md -->
## ccf slice 1a-i: closing read, round 2 (Opus)

Tree: the ccf-s1ai-tip worktree, unstaged diff. I ran the probes and mutations on a scratch copy of the module (`scratchpad/opus-r2/metasystem`; new probes are in `internal/dispatch/probe_r2_test.go`). I did not edit the worktree.

Checks on the worktree: `go build ./...` passes. `go vet` passes on dispatch, adapter, refusal and cmd. gofmt is clean. `bash -n` passes. The focused `-race` tests pass in all four packages. staticcheck is not installed here, and reading the code by hand turned up nothing it would flag.

## Material findings

| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| R2-1 | medium | yes | Once a closure is written on a chain that stays open, the chain can never close again after any later round. This holds whether the later round is clean and bound, with an identical subject, or cancelled. F-2's second clause ("a later clean bound round then closes") and F-3's "no error" both fail on this path. The base tree closes the same sequences. | **Path.** `dispatch.sh close` runs `critique-register-close` at 2902, which writes the closure, and only then runs `close-check` at 2904. close-check can still refuse after the closure is written. One case is an unmirrored root after a failed mirror: `mirror_record` is `\|\| true` at 2900, and `close.go` refuses with "cannot close an unmirrored chain". Another case is that the documented `job critique-register-close` verb writes a closure without setting `chainClosed`. The same verb holds no chain lock, so it can also race a follow-up's claim-launch. That race would also get past the new latest-record check at `finding_register.go:602`, because `state` is loaded at 513, before the child record is published. A follow-up is then admitted, because `follow_up` checks only `chainClosed` (dispatch.sh:2337). **Failure.** When a later round folds, `writeCleanClosure` compares the existing closure at 620-622 before the not-completed check at 628, and it refuses on the round number alone. Probe G: round 1 closed, then round 2 completes clean and bound on the same tree. Close errors with "already carries closure round 1 subject 11d5…; refusing to replace it with round 2 subject 11d5…" (the same digest), and a retry gives the same error. Probe H: round 2 is cancelled, and close gives the same error. `dispatch.sh close` dies under `set -e` before close-check. `closure` is dedicated metadata, and no verb removes it. No test covers either sequence. Spec 9b prescribes "different: refuse, never overwrite", so the fix needs a decision in the brief. Two options: write the closure only on the step that sets `chainClosed` (after close-check), or refuse a follow-up on a critic root that already carries a closure. |

Material-finding count: 1

## Non-material findings

| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| N-1 | low | no | F-5's selection is not the same as `finalHazardWorkState`. It keeps only completed members that have `review.json` and `diff.patch`, and it falls back to an earlier round. `finalHazardWorkState` takes the highest work round in any status, with no artifact requirement (hazard.go:283-326). | Probe I: implementer round 2 is running, or completed but not yet reviewed. With the tree unchanged, the subject is round 1 and is admitted. With the tree changed, dispatch refuses `SUBJECT_MISMATCH` against round 1, and the refusal names the correct next step. This fails safe in 1a-i because nothing consumes the selection yet. 1a-ii must compare against the terminal round's own subject. Removing the status filter (M8) leaves every test green. Removing the artifact filter (M7) also leaves every test green. The fold brief's parenthetical wrongly calls the two selections the same. |
| N-2 | low | no | The two `return nil` branches F-3 added for returns whose coordinates don't match or that aren't bound are not proven by any test. | Changing either branch (lines 637 and 640) to return an error leaves every named test green (M4, M5). The `failed` and `unbound-return` subtests stop at the withdrawn check (596) on their accepted-risk entry. The outcome is still proven. These branches are reachable only when `return.json` or `subject.json` is edited after the fold. |
| N-3 | low | no | The register row names `read_subject_compute.go:187`, but after the fold the `OpError` is at :242. | Brief section 6 asks for the OpError line. No test checks sites, and other rows have drifted too. |
| N-4 | low | no | A finding resolved as out-of-scope also prevents a clean closure (596). | The builder did what the fold brief says, and "clean" is defensible. The risk is for 1a-ii: under S1-11 ("refuses when the closure is absent"), chains closed with out-of-scope or accepted-risk entries would be refused. Resolve that in the 1a-ii brief. |
| N-5 | low | no | For a fresh critic that names a stale implementer round, `SUBJECT_MISMATCH` fires before the clearer "name the round job" refusal (claim.go:822). | The subject is computed at 1684, before claim-launch. Both refusals stop the dispatch, so only the order of messages is affected. |

## F-1 to F-5

- **F-1 closed.** The fixture uses `"$engine" util sha256 --file`, which prints a bare hex digest and is already used at fixture lines 1312 and 1782. The bed itself was not run.
- **F-2 closed for the reported scenario** (a close while round 2 runs, then a clean round 2 closes). `TestCleanClosureWaitsForLastCriticRound` fails when the check at 602 is removed (M1), and probe F now passes. The same check under `dispatch.sh close` is serialized with follow-ups, because both take the same chain lock (`$locks/<root>.d`). The direct verb is not serialized, which is part of R2-1. When a closure already exists, the check does not help (R2-1).
- **F-3 closed when no closure exists yet.** The cancelled-round assertion fails if 628 returns an error (M3). Probes B, C and D close with no error and no closure. After an existing closure it is still open (R2-1, probe H).
- **F-4 closed.** `TestCleanClosureRequiresWithdrawnRegister` fails without the check at 596 (M2), and it repeats the close. Probe E writes no closure. The rule matches the decoder's vocabulary (`finding_register.go:813-826`): resolved/withdrawn, resolved/out-of-scope, deferred/deferred and accepted-risk/accepted-risk, with legacy 7-field resolved entries treated as withdrawn.
- **F-5 closed.** Reverting it fails both RootJob tests (M6). Probe A now selects round 2 at T2 with no refusal. The workspace check pairs the selected member's `workspaceRoot` with the `reviewedTree` from that member's `review.json`, using the same projection as `reviewStage`. That is the right tree.

## Round-1 checks

These still hold, and `dispatch.sh` is unchanged since round 1:

- The subject is computed before the claim, at 1684-1693 and 2580-2589 (claim-launch is at 2691/2723).
- It is published only after the claim wins, at 1855-1858 and 2760-2763.
- The EXIT trap cleans up the temp file, and the fixture asserts no residue.
- Old rounds without `subject.json` fold as before: the digest key is deleted and not re-added, and `writeCleanClosure` returns nil when the subject is absent. `TestSliceZeroEmitsNothing` passes.

## Verdict

Not fit to land by human commit as it stands. R2-1 leaves an unrecoverable chain wedge that the base tree does not have. It is reachable through a close-check refusal after the closure is written, or through the direct close verb, followed by any further critic round. Choose the fix in the brief first: move the closure write behind close-check, or refuse follow-ups on a critic root that carries a closure. Everything else is ready.

---

<!-- source: ccf-s1ai-opus-read-r3.md -->
## ccf slice 1a-i: closing read, round 3 (Opus)

Tree: the ccf-s1ai-land worktree (origin/main d9d002cf with the slice diff unstaged). I computed `git diff HEAD` myself. Apart from `index` lines, it is identical to `ccf-s1ai-land.diff` and `ccf-s1ai-r3.diff`. I isolated the round-3 changes by diffing it against `ccf-s1ai.diff`, the round-2 tree. The rebase onto newer main merged cleanly into the dispatch.sh trap lines (`cleanup_composition_temporaries` and `cleanup_subject_temp` are both present) and into the `stage_temp` locals.

I ran probes and mutations on a scratch copy (`scratchpad/opus-r3/metasystem`, probes in `internal/dispatch/probe_r3_test.go`). I did not edit the worktree. On the worktree I ran only `gofmt -l` over the changed Go files (clean) and `bash -n scripts/agents/dispatch.sh` (clean). I ran no fixture beds and did not run the full cmd/metasystem suite.

Materiality test: would the change ship a defect, violate its brief, or damage what certifies it?

## Checks asked for

1. **Lock order.** `CritiqueChainClose` runs under the lease lock (through `lease run-held`), then takes the finding-register lock, then the session lock when the root has a `sessionKey`, then the record lock (close.go:218-219, record.go:789-810). Here is every other lock taker I found:
   - `CritiqueRegisterAdvance`, `CritiqueRegisterClose`, `CritiqueRegisterAcceptRisk`, `CritiqueRegisterResolveOutOfScope` and `CritiqueBudgetRebind` take the finding-register lock, then the root record lock (finding_register.go:69/96, 418-419, 459-460, 512-515, 688-690).
   - `CritiqueExhaustionAdvance` takes the finding-register lock (critique.go:186).
   - `RecordCAS` takes the session lock, then the record lock (record.go:490).
   - Follow-up publication through claim-launch takes the operation-publication lock, then the session lock, then the child record lock (claim.go:436-484).
   - `__critique-register-advance` already takes the lease lock, then the finding-register lock.

   No Go code that holds a session or record lock calls anything that takes the finding-register lock. The only non-test callers of the finding-register lock are the verb entry points. There is no cycle, so no deadlock and no inversion is added. All three locks are bounded flocks (recordLockWait, 10 s), so contention would surface as a retryable refusal, not a hang.
2. **No lock re-entry.** `CloseCheck`, `chainMembers`, `validateHazardCompletion` and its validators, `loadCritiqueState`, `critiqueFindingRegister`, `cleanClosure`, `readRoundSubject`, `ReadClosure` and `critiqueRecordForRound` take no lock. The only lock calls in close.go, chain.go, hazard.go, critique.go, read_subject.go and closure.go are the two in `CritiqueChainClose` and the one in `CritiqueExhaustionAdvance`.
3. **Shell and authority.**
   - `close_chain` holds the chain lock from `acquire_chain_lock` (dispatch.sh:2929) through `__critique-close` (2958/2960) until `release_chain_lock`.
   - `lease_run_held` is unchanged, including its STEWARD branch.
   - `__critique-close` (3362-3366) uses `internal_authority record-writer "$2"`, the same mode as `__record-cas` (3357-3361).
   - Redirection is refused. `refuseRepeatedFlags` (dispatch_verbs.go:1784) catches a repeated `--root-job` or `--repo` in the `-x`, `--x` and `--x=y` forms, because the shell always passes `--repo "$root"` first. A trailing `--` or a stray argument fails `NArg`. A child id is refused by `CloseCheck` ("non-terminal record": its lineage root is the parent, so it has no members). A traversal id is refused with code 2 by `withRecordLock`. The probes show all three.
4. **Non-critic roles.** The else branch has the same patch, the same `--expect`/`--status` CAS and the same cleanup as before. The only change is that `role` is read once, into a new `local`, which no callee reads.
5. **Crash and re-close.** Everything before `writeRecord` stays in memory, and `writeRecord` is one atomic publish (record.go:642-652). A crash before it leaves no closure and no `chainClosed`. A crash between `writeRecord` and `syncRecord` leaves the same state `RecordCAS` would. A re-close reaches the early return at close.go:231 after `CloseCheck` and writes nothing: the tests compare bytes, and M5 below confirms the result is identical either way.
6. **syncRecord.** It is called after the write, as `RecordCAS` does, on a transaction opened under the same session lock. For a terminal record it drops a stale occupant or does nothing.
7. **Tests.** On the literal round-2 tree the new tests do not compile, because `CritiqueChainClose` and `closeReadyCriticChain` do not exist there. I checked the real claim with mutation M8, which restores round 2's closure write inside `CritiqueRegisterClose`. It fails all four new tests and all five repointed tests. M1 and M2 fail the two `TestCleanClosureSkipsEditedReturn` subtests, confirming the builder's N-2 claim. M3 and M4 (no `CloseCheck`, or the closure written in a separate write before `CloseCheck`) fail `TestClosureFollowsChainClose` and `TestCancelledRoundAfterRefusedClose`. The mutation table is below.
8. **Shell legs.** The close-race, happy-close and closed-follow-up legs reach `__critique-close` with the same authority and lease plumbing `__record-cas` had. The new path fails in exactly the cases `cleanClosure` refuses, and those refusals are identical to round 2's (same checks, same order). Round 2 raised them inside `critique-register-close`, earlier in the same `set -e` sequence. So the shell change adds no new way for those legs to fail. `happy-close` still leaves `runnerClosed` false. The cap-driver and cap-warden close legs still refuse inside `critique-register-close`, which is unchanged. I did not run the bed.
9. **SUBJECT_MISMATCH site.** register.go:74 names `read_subject_compute.go:242`, and line 242 is `return &OpError{`.
10. **Page.** The F-1 amendment in section 9b (plans line 97) and row S1-10 (line 140) match the code: `CritiqueChainClose`, `CloseCheck` first, one write with `chainClosed`, finding-register and record locks, `CritiqueRegisterClose` never writes, and idempotence (the equal and different cases are in `cleanClosure` at finding_register.go:619-623). Section 11 is untouched and consistent.

## Mutations (closure test subset, scratch copy)

| Id | Mutation | Result |
| --- | --- | --- |
| M1 | cleanClosure return-coordinates branch (finding_register.go:638) returns an error | fails TestCleanClosureSkipsEditedReturn/job-id |
| M2 | unbound-return branch (:641) returns an error | fails TestCleanClosureSkipsEditedReturn/reviewed-tree |
| M3 | CloseCheck removed from CritiqueChainClose | fails TestClosureFollowsChainClose, TestCancelledRoundAfterRefusedClose |
| M4 | closure written in its own write before CloseCheck | fails the same two |
| M5 | chainClosed early return removed | passes (the rewrite is byte-identical) |
| M6 | role refusal disabled | passes (untested) |
| M7 | syncRecord removed | passes (no session-keyed critic root in the tests) |
| M8 | CritiqueRegisterClose writes the closure again (round-2 behaviour) | fails all nine closure tests |
| M9 | runnerClosed not written | fails TestDesignChainClosesAtRoundOne |
| M10 | cancelled or not-completed status check (:629) removed | fails TestCloseWritesOneClosure |

## Probes (scratch, all pass)

- A cancelled round 2 with a `subject.json`, the production shape, after a refused close: close returns nil, no closure, `chainClosed` true.
- A failed round 2 with a `subject.json`, its synthetic finding accepted as risk, after a refused close: close returns nil, no closure, `chainClosed` true.
- `CritiqueChainClose` on a child id refuses and leaves the child unmarked. On an implementer root it refuses with "not a critic chain root". On `../x` it refuses with code 2.
- A host close followed by a runner re-close with `--runner-closed` leaves `runnerClosed` unset (see N3-2).

## Findings

| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| N3-1 | low | no | The public `job critique-close` verb takes no chain lock and checks no authority. "Reached only through dispatch.sh close" is help text, not enforced. If the verb is called directly while a follow-up is claiming its launch, the verb can pass `CloseCheck` and write a closure plus `chainClosed` for round N before the child record for round N+1 exists. The follow-up checked `chainClosed` before publishing. `job record-cas` already has the same hole for `chainClosed`, so the new element is only the closure. The dispatch path is serialized. This matters for 1a-ii: its gates should require the closure round to equal the terminal critic round. | cmd/metasystem/main.go:206; cmd/metasystem/dispatch_verbs.go:1783-1796; internal/dispatch/close.go:217-257; scripts/agents/dispatch.sh:2368 |
| N3-2 | low | no | The early return on `chainClosed` also skips the `--runner-closed` stamp when a closed chain is re-closed. The old CAS stamped it. The brief's "if nothing would change" does not strictly cover this case. No production reader branches on `runnerClosed`, and the runner only closes chains that are not yet closed. | internal/dispatch/close.go:231-233, 248-250; internal/missionrunner/jobs.go:138; probe TestProbeR3RunnerClosedOnReclose |
| N3-3 | low | no | Three branches of `CritiqueChainClose` are unproven: the role refusal (M6 survives, and no test anywhere matches "not a critic chain root"), `syncRecord` (M7; a no-op for terminal records), and the early return (M5; the result is byte-identical). The probes show the role and child-id refusals work. | internal/dispatch/close.go:225-227, 231-233, 254 |
| N3-4 | low | no | `TestCancelledRoundAfterRefusedClose` writes no `subject.json` for the cancelled round 2, so its no-closure result comes from the absent subject, not the status check. My probe with the subject present gives the same outcome, and M10 shows `TestCloseWritesOneClosure` covers the status branch. The brief's "synthetic finding accepted as risk" does not apply to a neutral cancelled fold, and the builder rightly left it out. S1-10's "a root with open findings gets none" is asserted only through `CritiqueRegisterClose`, so the check is now vacuous. `CloseCheck`'s open-finding refusal is proven at composition_test.go:841. | internal/dispatch/read_subject_compute_test.go:838-892, 525-543; internal/dispatch/finding_register.go:607-612, 628 |
| N3-5 | low | no | `__critique-close` uses record-writer, as the brief asks. That mode is wider than the holder-only mode of the other register-owner internal verbs: it admits SUPERVISION for any job and an adapter supervisor for its custody job. It is no wider than `__record-cas`, and the closure content is derived mechanically after `CloseCheck`. No fixture leg proves a delegate-shaped caller is refused at `__critique-close`, as the existing leg does for `__critique-register-advance`. | scripts/agents/dispatch.sh:3225-3237, 3357-3366; scripts/agents/dispatch-fixtures.sh:2944-2956; internal/authority/authority.go:85-110 |
| N3-6 | low | no | docs/design/dispatch-sequence.md still says `chainClosed` "lands as a self-edge CAS on the root", and its internal-verb list lacks `__critique-close`. That doc's line references were already stale, and the brief scoped only the plan page. | docs/design/dispatch-sequence.md:48-52, 317-319 |
| N3-7 | low | no | Carried unchanged from the round-2 read for the 1a-ii brief: N-1 (F-5 selection differs from `finalHazardWorkState`), N-4 (out-of-scope and accepted-risk registers never get a closure, so S1-11 would refuse those chains), N-5 (message order at claim.go:822). | ccf-s1ai-opus-read-r2.md, non-material table |

R2-1 is closed. `CritiqueRegisterClose` no longer writes a closure (finding_register.go:543-545), and the closure is written only in the `chainClosed` write after `CloseCheck`. Probe G is now `TestClosureFollowsChainClose`, and probe H is `TestCancelledRoundAfterRefusedClose` plus my probe with a subject present. N-2 is closed (M1, M2). N-3 is closed.

Material-finding count: 0

## Verdict

Fit to land by human commit once the seat's in-flight gates pass (build, vet, race tests, `bash -n`, fast gate) and cluster c of the dispatch bed passes. Nothing must change. N3-1 and N3-7 belong in the 1a-ii brief: its closure-reading gates should require the closure round to equal the terminal critic round, and should decide how chains closed with out-of-scope or accepted-risk entries are handled.
