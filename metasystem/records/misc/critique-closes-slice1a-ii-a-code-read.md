# Independent code read: critique-closes slice 1 part 1a-ii, unit a

Reader: Opus 5, 2026-09-13. Tree read: `/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/.claude/worktrees/ccf-s1aii-a/metasystem`
at base `0d4c0c5f`, `git diff` plus untracked `internal/readsubject/`.
Spec read: `plans/critique-closes-on-folded-proof-design.md` (revision 3, no revision 4) and the proposed
section 9c at `artifacts/reports/ccf-9c-section.md` (D1 to D10), brief
`artifacts/reports/codex-ccf-slice1a-ii-brief.md`, builder report
`artifacts/reports/codex-ccf-slice1a-ii-result.md`.

All probes ran in scratch copies of the tree. The worktree was not edited: `git status --porcelain` is the
same seven modified files plus the one untracked directory after the read as before it.

## Findings

| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| F-1 | medium | yes | The unit-a rule "a malformed or inconsistent present closure never becomes absence" (brief, "Handle absence deliberately"; design D1 "Never downgrade them to an absent closure") has no assertion for the gate. Every gate refusal can be turned into `present=false` and the package stays green. | `internal/readsubject/closure_test.go:77-82` (`requireRefused` asserts only `err == nil`, discards `present`). Mutation in a scratch copy: all 29 `return closure, true, fmt.Errorf` in `ReadClosedClosure` replaced by `return Closure{}, false, fmt.Errorf` plus `return Closure{}, present, err` at `closure.go:117` replaced by `false` -> `go test ./internal/readsubject -count=1` = `ok ... 0.273s` (SURVIVED). The decoder's half of the same rule is asserted (`closure_test.go:350`), the gate's half is not. The implementation is correct; the proof is missing. |
| F-2 | low | no | `readsubject.CleanRegister` is stricter than dispatch's own `decodeFindingRegister` on two entry shapes the decoder accepts, and errors where the old writer merely declined to write a closure. In `CritiqueChainClose` that error is fatal to the close. | Probe (scratch copy of the new tree, `internal/dispatch`): legacy seven-field entry, `decodeFindingRegister`=nil error for every status, `CleanRegister` = error `finding register entry 0 has status/resolution mismatch "deferred"/""` and the same for `accepted-risk`; a thirteen-key entry carrying `note` instead of `evidence` decodes fine but `CleanRegister` = `entry 0 is not an object with the canonical fields`. Path: `internal/dispatch/finding_register.go:580-587` -> `close.go:240` -> error out of `CritiqueChainClose`. Unreachable through data this system ever wrote: before `78018dd5` the seven-field decoder allowed only open/resolved/disputed (`git show 78018dd5^:.../finding_register.go:331`), and `encodeFindingRegister` always emits exactly the thirteen canonical keys. Direction is fail-closed. |
| F-3 | low | no | Three of the gate's named conditions are individually redundant and unproven: "the highest member is completed", "every member is terminal", and "closure.round equals the last critic round". Each can be deleted alone with the whole suite green. Two shapes near them are untested. | Mutation probes, scratch copy: M1 `if highestStatus != "completed"` disabled -> SURVIVED; M11 member `terminalStatus` check disabled -> SURVIVED; M2 `if closure.Round != highestRound` disabled -> SURVIVED. The other nine mutations (clean register, folded digest, folded round, persisted subject equality, return binding, chainClosed, lost evidence, duplicate round, absence refusal) were all CAUGHT. The tested later-member cases are caught by the round comparison or by the "return names the selected member" check at `closure.go:244`, so the behaviour the brief names still refuses. Untested shapes, both correct when probed: a valid closure at round 2 (`present=true, err=nil`), and a closure whose own last round is failed, cancelled or timeout (refused by that surviving check alone). The one positive case in the suite is round 1 (`closure_test.go:19-32`). |
| F-4 | low | no | `internal/readsubject: 90.0` sits 9.6 points under measured coverage in a package whose tests are the only proof of the gate. | `scripts/agents/coverage-ratchet.json:64` and `-linux.json:64`. Measured independently in a scratch copy: `internal/readsubject` 99.6%, `internal/dispatch` 78.3% (floor 75.9). Design 9c section 6 and the brief both say "at least 90.0", so the diff conforms; the repo's convention elsewhere sits floors within a few points of measured (dispatch 75.9 vs 78.3). Nothing in `internal/audit/coverage.go:80-112` forces the tighter floor. |
| F-5 | info | no | The new floor key is inserted out of alphabetical order in both ratchet files. | `scripts/agents/coverage-ratchet.json:64` places `internal/readsubject` after `internal/receipt`; the file is otherwise sorted and `readsubject` sorts before `receipt`. JSON object, so no gate reads order. |
| F-6 | low | no | The new package carries no doc comments, and its `(Closure, bool, error)` contract is genuinely ambiguous for units b and c: the bool means "a closure field was present", never "usable", and refusals arrive as errors in both the present and the absent path. A consumer that branches on the bool before checking the error would take the old path on a refusal. | `internal/readsubject/closure.go:114-310` (no comment anywhere in the file); compare the documented contracts the repo writes elsewhere, e.g. `internal/audit/coverage.go:71-79`, `internal/dispatch/finding_register.go:830-833`. |
| F-7 | info | no | `closureField = "closure"` is now defined twice, in `internal/dispatch/closure.go:5` and `internal/readsubject/closure.go:10`. The record key has two owners where the brief wanted one. | Both files; identical values today. |

Material findings: 1 (F-1).

## What I checked and found sound

1. **Wire and digest preservation (seat question 1).** Probed, not inferred. I extracted the pre-change tree
   (`git archive HEAD`) into a scratch copy and computed the wire bytes and digests of the three kinds with the
   OLD `dispatch.ReadSubject`. All three match the constants hard-coded in the new preservation guard exactly,
   including the HTML escaping: live `aaf9bec2d37a42eaf870f4ee9aaaacec2840c5714039ba20011b9987ae616995`,
   design `6f98044a77557a2c881c161169d5ae9060b8d65f9367b52558c3928c60054644`,
   commit `ea400e4e8b8157a92e07e9e727cd313c00c77418c8af4ff3c3c4b2c7c30ef049`
   (`internal/readsubject/subject_test.go:22,32,42`). So the guard is not self-confirming.
   `Digest()` is `sha256(json.Marshal(identity))` hex, byte-identical to dispatch's `digestJSON`/`canonicalJSON`
   (`internal/dispatch/finding_register.go:1260-1268`). JSON tags and every `omitempty` are carried across
   verbatim; `encodeClosure` still marshals the struct (`finding_register.go:659`), so the persisted closure
   object is unchanged. `WriteReadSubject` and `read_subject_compute.go` were not touched.
2. **Refusal strings and codes.** Every moved error string is byte-identical. The only new strings are
   `ReadRoundSubject`'s two input-validation errors (`subject.go:104,107`), which harden a path that previously
   accepted any root id into a file path; both dispatch callers pass ids that already passed record loading.
   No row in `internal/refusal/register.go` cites `dispatch/read_subject.go`, `dispatch/closure.go` or
   `dispatch/finding_register.go`, so the line shifts stale nothing; `SUBJECT_MISMATCH`'s site
   (`read_subject_compute.go:242`) is in a file the diff does not touch.
3. **Gate conditions (seat question 2).** Every D1 condition is present and enforced: critic root and role
   (`closure.go:123-137`), terminal root and `chainClosed` (139-145), unique highest round including any
   duplicate round (158-169), every member terminal (176-179), root present at round 1 (180-194), highest
   member completed (195-197), `closure.Round == highestRound` (198) and `== findingRegisterRound` (217),
   register present and clean (202-212), persisted subject present and equal (221-230), folded digest equal
   (231-237), return naming the selected member and round (239-246), return binding the subject (247-249),
   mechanism `clean` (`ReadClosure`, 44-47). I could not construct an input that yields a usable closure for a
   chain with a later member, a non-clean register, or lost evidence. Probes: a later completed round-2 member
   beside a round-1 closure refuses; an open finding beside a present closure refuses; a failed, cancelled or
   timeout last round refuses. The one hole is by contract, not by defect: the reader cannot detect a
   membership list its caller truncated, which is exactly the duty the brief puts on units b and c.
4. **D3 lawful absence (seat question 3).** Historical absence (no register, no folded round, no digest) and
   lawful non-clean absence (resolved/out-of-scope, accepted-risk, deferred) both return `present=false, err=nil`
   and reach the old path (`closure_test.go:213-231`). A modern clean, subject-bearing fold without a closure
   refuses by message (`closure.go:309`). A recorded folded digest with a missing subject file refuses as lost
   evidence (302). Neither refusal names the root job id, which is cosmetic.
5. **Stdlib-only and thin delegation (seat question 4).** `go list -f '{{.Imports}}'` on the package:
   `crypto/sha256 encoding/hex encoding/json fmt os path/filepath regexp strconv`. Dispatch keeps type aliases
   only (`read_subject.go`, `closure.go`), no second equality, digest or decoder implementation survives in
   dispatch, and no other package defines a read subject. `gofmt -l` is clean on both packages.
6. **Coverage.** Measured in a scratch copy, not taken from the report: `internal/dispatch` 78.4% before the
   change and 78.3% after, against its unchanged 75.9 floor, so moving covered code out of dispatch does not
   breach the dispatch ratchet. `internal/readsubject` 99.6%, reproducing the seat's number. No floor was
   lowered anywhere; both ratchets got the same single added key.
7. **Scope (seat question 6).** Nothing belonging to units b or c leaked in. `internal/validate`,
   `internal/landing`, `internal/missionrunner`, `cmd/`, `scripts/`, `testing.json`, fixtures, role text and
   `plans/` are all untouched; the only non-Go changes are the two ratchet lines. The brief and the builder's
   result report live under `artifacts/`, which `.gitignore` excludes, so they are correctly outside the diff
   (the builder's "1,301 changed lines" total counts that ignored report; the reviewable diff is smaller).
8. **Test quality (seat question 7).** The suite does discriminate against the pre-change decision: nine of
   twelve mutations of individual gate rules are caught, the register-clean and lost-evidence rules among them,
   and the builder's expected-red evidence matches the shape I confirmed. Retained dispatch runtime tests
   (`TestCloseWritesOneClosure`, `TestClosureFollowsChainClose`, `TestDesignChainClosesAtRoundOne`,
   `TestSliceZeroEmitsNothing`) still pass; the whole dispatch package is green in my own runs. One assertion is
   tautological, `subject_test.go:80` (`Digest() == ""` can never be true for a sha256 hex string); it is a
   coverage device for the default branch and harms nothing. The real test gaps are F-1 and F-3.

## Verdict

Fit to land once F-1 is answered. The implementation itself is correct, behaviour-preserving on the wire and
strictly fail-closed on every input I could build, and every other finding is non-material. What must change:
assert the `present` return in `closure_test.go`'s `requireRefused` (and on the non-clean-presence case in
`TestClosureGateMissingCleanClosure`) so D1's "never downgrade a present closure to absence" has a proof; one
line. Worth folding in while that file is open, at the seat's discretion and not as a condition of landing:
a positive round-2 closure case and a failed/cancelled last-round case, which would kill the three surviving
mutations in F-3.
