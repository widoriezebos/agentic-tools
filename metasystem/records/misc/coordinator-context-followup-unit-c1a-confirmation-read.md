VERDICT: land

Material findings: 0. The fold closes F-1 and F-3, and each new witness kills its mutant. The only file it changed is metasystem/internal/steward/handoff_capture_test.go, and it added 6 lines there. The F-2 code in the result file passes as written, and each of R1 to R4 fails one of its two cases. The deferral of F-2 to C1b is the seat's decision under the fold brief's overflow rule, so it is not reported as a finding.

### F-11. Severity low. Material no.

Claim: the consumed-nonce witness fails under A4b, but its failure line prints only the observed error, not the expected `HANDOFF_NOT_LIVE nonce=<n>`.

Evidence:
- Under A4b the failure line is `handoff_capture_test.go:840: consumed cancellation=HANDOFF_HUMAN_UNPROVEN nonce=6400000000000001 by=Wido caller=MAIN human=unattempted`.
- The expectation is exact in the assertion at handoff_capture_test.go:839: `!errors.As(err, &refusal) || err.Error() != "HANDOFF_NOT_LIVE nonce="+result.Nonce`. The unmutated copy passes it.
- The message format `consumed cancellation=%v` is unchanged from the pre-fold file, where it was at :839, and the fold was not asked to change it.
- Only the diagnostic text is affected. Behaviour and proof are not.

### F-12. Severity low. Material no.

Claim: after the seat's act swap, no test cancels a consumed nonce with a proven HUMAN act, so a mutant that gives only that act a plain error on the not-live path survives, while the pre-fold subtest killed it.

Evidence:
- Mutant A4e changes the not-exist branch of `CancelHandoff` (handoff_capture.go:1121) to return `fmt.Errorf("mutant: human act on a nonce that is not live")` when `canceller.Human != nil && canceller.Caller.Class == handoffClassHuman`.
- On the folded test file it survives. CN passes, and SW passes with 121 PASS lines (`ok 49.458s`).
- On the pre-fold test file CN kills it: `handoff_capture_test.go:839: consumed cancellation=mutant: human act on a nonce that is not live`.

Why it is not material:
- The fold does exactly what item 1 of the brief says.
- Row A4 (design page line 749) names the failure "judging the act first prints a token for a consumed nonce". That failure is still witnessed: A4 in the builder's form, A4b, A4c and A4d are all killed at :840.
- A4f is keyed on `canceller.Human != nil`, the D15-2 leg selector, the way a split into two legs would be keyed. It is also killed at :840: `consumed cancellation=mutant: human leg reads a nonce that is not live`.
- The only fault that escapes singles out a proven HUMAN class on the not-live path and prints no token.

Artifact, if the seat wants both acts covered: C1b could add the pre-fold act as a second consumed-nonce subtest, about 10 numstat lines.

### F-13. Severity low. Material no.

Claim: the carried F-2 code reaches C1b only if the seat copies the result file, because a fresh C1b checkout will not contain it at the path the C1b brief names.

Evidence:
- scratchpad/codex-ccb-c1b-brief.md sends the builder to `/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/.claude/worktrees/ccb-c1b/metasystem/artifacts/reports/codex-ccb-c1a-result-2.md`.
- The report is ignored: `git check-ignore -v` gives `metasystem/.gitignore:1:artifacts/`.
- At 23:55Z the directory .claude/worktrees/ccb-c1b did not exist, and `git worktree list` has no ccb-c1b entry.
- The C1b brief also describes both cases in prose.
- This does not affect whether C1a lands.

## Mutation table

Commands. Every command is `go test -count=1 -timeout 40m -v ./internal/steward -run '<pattern>'`, run from the copy's metasystem directory with GOCACHE=/tmp/opus-c1a2-gocache, GOTMPDIR=/tmp/opus-c1a2-tmp and METASYSTEM_BIN unset. The patterns are:
- CN: `^TestCancelHandoffRefusesAnUnprovenHumanAct$/^consumed_nonce$`
- ML: `^TestCancelHandoffHumanActIsHolderBound$/^malformed_lease$`
- BR: `^(TestCancelHandoff|TestContextVerifyAndCancel|TestVerifyHandoffState)`, the fold brief's verification pattern
- F2: `^(TestCancelHandoffAdmitsAProvenHumanAct|TestCancelHandoffRefusesAnUnprovenHumanAct)$/^(empty_session_with_process_generation|empty_caller_class)$`
- SW: `(?i)(handoff|cancel)`, the broad steward sweep

Copies:
- A: HEAD plus the six live files.
- B: A plus the F-2 code from the result file.
- C: A, used for the A4e probe.

| Mutant | Copy | Change | Command | Observed | Result |
| --- | --- | --- | --- | --- | --- |
| none | A | unmutated | CN | `--- PASS: TestCancelHandoffRefusesAnUnprovenHumanAct/consumed_nonce (0.61s)`, `ok 1.141s` | green |
| none | A | unmutated | ML | `--- PASS: TestCancelHandoffHumanActIsHolderBound/malformed_lease (0.95s)`, `ok 1.173s` | green |
| none | A | unmutated | BR | 26 PASS lines, `ok 15.432s` | green |
| A4b | A | read-1's A4b: after `defer arbitration.Release()` (handoff_capture.go:1118) and before the live read, `handoffCancelReason` is called for a human act whose class is not HUMAN or whose proof fails `TerminalValidFor(stateRoot)` | CN | `handoff_capture_test.go:840: consumed cancellation=HANDOFF_HUMAN_UNPROVEN nonce=6400000000000001 by=Wido caller=MAIN human=unattempted` | killed |
| A4c | A | at the same place, only the proof step: `handoffHumanRefusal(nonce, canceller, "unobserved")` when the proof fails `TerminalValidFor(stateRoot)` | CN | `handoff_capture_test.go:840: consumed cancellation=HANDOFF_HUMAN_UNPROVEN nonce=6400000000000001 by=Wido caller=MAIN human=unobserved` | killed |
| A4d | A | at the same place, only the lease re-read and the coordinate equality of D15-4 steps 3 and 4 | CN | `handoff_capture_test.go:840: consumed cancellation=handoff 6400000000000001 cannot be cancelled by a human act: the supplied holder coordinates do not match the current checkout lease` | killed |
| A21a | A | read-1's A21a: `ReadHolderLease` decodes the lease with `os.ReadFile` and an unchecked `json.Unmarshal` instead of calling `currentSessionStopLease` (sessionstop.go:443) | ML | `handoff_capture_test.go:884: cancellation error=<nil> want="handoff 6400000000000001 cannot be cancelled by a human act: the checkout holder lease cannot be proved: checkout lease is malformed"` | killed |
| none | B | unmutated, F-2 code inserted | F2 | `--- PASS: TestCancelHandoffAdmitsAProvenHumanAct/empty_session_with_process_generation (0.69s)`, `--- PASS: TestCancelHandoffRefusesAnUnprovenHumanAct/empty_caller_class (0.74s)`, `ok 1.729s` | green |
| none | B | unmutated, F-2 code inserted | BR | 28 PASS lines, `ok 16.815s` | green |
| R1 | B | `session := goal.NormalizeSession(act.HolderSession)`, with the empty-session guard dropped | F2 | `handoff_capture_test.go:838: outcome="cancelled: cancelled by human by=Wido holder=main-1 epoch=3 session=e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 human-pid=4711 human-started=1700000000 human-ticks=5 human-boot=boot-fixture" want="... session=none ..."`; empty_caller_class passes | killed by empty_session_with_process_generation |
| R2 | B | `boot := ref.BootID[:0]` | F2 | `handoff_capture_test.go:838: outcome="... human-ticks=5 human-boot=none" want="... human-ticks=5 human-boot=boot-fixture"`; empty_caller_class passes | killed by empty_session_with_process_generation |
| R3 | B | `p.StartTicks*0` in the reason | F2 | `handoff_capture_test.go:838: outcome="... human-ticks=0 human-boot=boot-fixture" want="... human-ticks=5 human-boot=boot-fixture"`; empty_caller_class passes | killed by empty_session_with_process_generation |
| R4 | B | the `if class == "" { class = "none" }` substitution removed | F2 | `handoff_capture_test.go:885: cancellation refusal=HANDOFF_HUMAN_UNPROVEN nonce=6400000000000001 by=Wido caller= human=unattempted want="HANDOFF_HUMAN_UNPROVEN nonce=6400000000000001 by=Wido caller=none human=unattempted"`; empty_session_with_process_generation passes | killed by empty_caller_class |
| A4e | C | the not-exist branch (handoff_capture.go:1121) returns a plain error when `canceller.Human != nil && canceller.Caller.Class == handoffClassHuman` | CN, SW | CN `--- PASS: TestCancelHandoffRefusesAnUnprovenHumanAct/consumed_nonce (0.50s)`; SW 121 PASS lines, `ok 49.458s` | survived (F-12) |
| A4e control | C with the pre-fold test file | the same mutant | CN | `handoff_capture_test.go:839: consumed cancellation=mutant: human act on a nonce that is not live` | killed before the fold |
| A4 | C, reset to the live files | read-1's A4 in the builder's form: for any human act, the not-exist branch returns `handoffHumanRefusal(nonce, canceller, "unobserved")` | CN | `handoff_capture_test.go:840: consumed cancellation=HANDOFF_HUMAN_UNPROVEN nonce=6400000000000001 by=Wido caller=MAIN human=unobserved` | killed |
| A4f | C, reset to the live files | the not-exist branch returns a plain error when `canceller.Human != nil` | CN | `handoff_capture_test.go:840: consumed cancellation=mutant: human leg reads a nonce that is not live` | killed |

Every restore in copies A, B and C matched its sha256: handoff_capture.go f9fa78705ac5dbcd and sessionstop.go d36a30a09342ad14. After the A4e probe, copy C was re-copied from the live files and matched scratchpad/opus-c1a2-six.sha before A4 and A4f ran.

## Checks

1. Scope of the fold. Only the test file changed.
   - The protected files: `shasum -a 256 -c scratchpad/c1a-prod-before-fold.sha`, run from the live worktree root, reports OK for all five, at 23:47Z and again at the end of this read.
   - The production diff: the live `git diff HEAD` over those five files is byte-identical by `cmp` to lines 1 to 260 of the seat's saved pre-fold diff, scratchpad/ccb-c1a-code.diff, whose numstat total is 388.
   - The pre-fold baseline: rebuilt from HEAD plus that diff's test section, sha256 38103fdd09e416be. Its lines :810, :839, :846, :878, :891, :899, :912, :919, :925 and :931 are the assertions read-1 cites, so it is the file read-1 read.
   - The fold diff: `diff -u` from the pre-fold file to the live file (sha256 57387051754544a1) has 6 additions and 0 deletions:
     - One line in `TestCancelHandoffRefusesAnUnprovenHumanAct/consumed nonce` (:836): `canceller = HandoffCanceller{Caller: handoffMainCaller(), Human: &HandoffHumanAct{By: "Wido"}}`.
     - The five-line subtest `TestCancelHandoffHumanActIsHolderBound/malformed lease` (:881 to :885).
   - No subtest or assertion was removed, and no other existing subtest changed. The `leasePath` helper it uses was already in the file before the fold.
   - Unit size: `git diff --numstat HEAD` gives 365 plus 29, which is 394. `git status` shows the same six modified files and nothing untracked; the reports are ignored files.
2. F-1. A4b fails the consumed-nonce subtest at :840, and the exact `HANDOFF_NOT_LIVE nonce=<n>` expectation sits at :839.
   - The new act would be refused by D15-4 steps 1, 2 and 4, so the witness also kills the proof-only (A4c) and lease-only (A4d) reorderings. It also kills row A4's original mutant and the leg-shaped A4f, so it proves D15-7's order and row A4's named failure for more than one mutant. The coverage the seat ruling gives up is recorded as F-12.
   - The act follows the seat ruling in the fold brief's item 1.
3. F-3. A21a fails the malformed-lease subtest at :884 on an exact-text assertion. `failedCancel` also requires that the handoff stays live and has no tombstone.
4. The unmutated copy A is clean.
   - `gofmt -l internal/steward` prints nothing.
   - `go vet ./internal/steward` exits 0.
   - The CN and ML runs pass, and BR passes with 26 PASS lines.
5. The F-2 code in the result file is ready for C1b.
   - Extraction: the block under `F-2 code for C1b` (18 lines, tab-indented, sha256 prefix 623f4e5a225d3800) was taken verbatim and indented one level.
   - Placement: the first subtest was appended as the last subtest of `TestCancelHandoffAdmitsAProvenHumanAct` (:829) and the second as the last subtest of `TestCancelHandoffRefusesAnUnprovenHumanAct` (:882).
   - Clean build: gofmt and vet are clean, both cases pass, and BR passes with 28 PASS lines.
   - Mutants: each of R1 to R4 fails exactly one named case.
   - Line numbers: the builder reported failure lines :825 and :864, which reflect a placement its result does not state. At end-of-function placement the lines are :838 and :885. Only the placement differs.
   - Size: the insertion is 19 numstat lines, which would take the unit to 413. The overflow rule was correctly applied.
   - Text: the expected strings follow D15-4's reason and refusal formats (design page lines 676 and 682: `<session>` or `none`, `<boot>` or `none`, `<class>` or `none`).
   - C1b consistency: the C1b brief's Non-goals keep the human leg's texts and the `HANDOFF_HUMAN_UNPROVEN` text unchanged, so the strings remain valid there.

Isolation:
- Copies: every mutant and probe ran in private copies scratchpad/opus-c1a2-A, opus-c1a2-B and opus-c1a2-C. Each was made from `git archive HEAD metasystem` plus the six live files and matched scratchpad/opus-c1a2-six.sha before use.
- Live reads: git commands against the live worktree used --no-optional-locks.
- Live hashes: the six changed files, the fold brief and the result file matched scratchpad/c1a2-live-start.sha (taken 23:47:12Z) at the end. The numstat stayed at 394.
- Mtimes: the live files keep the mtimes of the builder's last restores, 23:37:52Z to 23:40:25Z.
- Scripts and logs: the drivers are scratchpad/opus-c1a2-mutate.py and scratchpad/opus-c1a2-a4e.py. The logs are scratchpad/opus-c1a2-A.log, opus-c1a2-B.log and opus-c1a2-C.log.
