VERDICT: land

Unit 3a-i of brief-declares-the-round-boundary, fold round 1. Independent confirmation read by Opus. I did not write this code. Scope: did the fold close F-1 and F-2 from the closing read (codex-bdrb-u3a-i-read-1.md) without changing anything else.

The live worktree was never modified. Every run used private copies built from `git archive` of HEAD 4eb2b85bc96c641f123524ff3aafe8897ad97527 plus the two intent-to-add files, each copy sha256-matched against the live files before use: /tmp/opus-u3ai2-copy (pristine), /tmp/opus-u3ai2-mut (mutations), /tmp/opus-u3ai2-old (pre-fold control, carrying the pre-fold test file 829fb0cb from the earlier reader's copy). None sits inside a Git repository. GOCACHE was /tmp/opus-u3ai2-gocache, GOTMPDIR and TMPDIR were /tmp/opus-u3ai2-tmp, and METASYSTEM_BIN, GIT_DIR and METASYSTEM_CONTEXT_COST_PROOF were unset. Only focused `-run` tests on ./internal/dispatch ran. No race suite and no fixture bed ran. Material findings: 0.

## Check 1. Only the test file changed, nothing removed, unit within ceiling

- Production file: `shasum -a 256 -c u3a-i-prod-before-fold.sha` from the live worktree root printed `metasystem/internal/dispatch/brief_bounds_record.go: OK`. Byte-identical to its pre-fold state, so the non-goal holds.
- Test file: pre-fold 829fb0cb268845255369d2f0fa5dd0b8f87b27f20036f99ac0fab4fcab1c7ffc, post-fold 71f133aa325d8687b99364233609f152fe7f2214609708d001dca2f80f4cb573. `diff -u` between them is two hunks only: one added `assertRecordRefused` line in `types` (the top-level array), and the `ceiling-range` loop rewritten to an encoded/want table with an exact comparison. No other line changed.
- Nothing removed. Subtest names are identical and still 14: version, required, unknown, duplicate-key, trailing-json, types, identity, digest, bytes, pair, null-pair, empty-boundary, ceiling-range, member-values. Call counts old to new: assertRecordRefused 19 to 20, assertRecordMarshalRefused 2 to 2, assertRecordRoundTrip 3 to 3, decodeRecord 9 to 9. The two clauses dropped from the old `ceiling-range` condition (`*r.Ceiling < 0` and the always-false `> math.MaxInt64`) are subsumed by the new exact equality: at endpoints 0 and MaxInt64 any negative or otherwise wrong value fails `!= endpoint.want`. No assertion was weakened.
- Nothing else changed in the repository. `git status --short` lists only the two intent-to-add files, HEAD is unchanged at 4eb2b85b, and `git diff HEAD --numstat` is 125 + 152. The only live files with an mtime later than the closing read are the two unit files, the fold brief and the result report; the reports sit under the ignored artifacts/ tree.
- Unit size 277 lines (125 + 152), at most 400. The builder's numstat claim matches.

## Check 2. F-1 closed

- The always-false comparison is gone. Pre-fold line 90 read `if r.Ceiling == nil || *r.Ceiling < 0 || *r.Ceiling > math.MaxInt64 {`. That text does not occur in the folded file.
- The maximum is now checked exactly. brief_bounds_record_test.go:88-103 iterates `{"0", 0}` and `{"9223372036854775807", math.MaxInt64}`, fails on a nil Ceiling at :95 and on `*r.Ceiling != endpoint.want` at :98.
- A Ceiling beyond int64 is still refused: :102 keeps `"ceiling":9223372036854775808`, and :101 keeps `"ceiling":-1`. Unmutated probe: the maximum is admitted as exactly 9223372036854775807, and the overflow is refused with `invalid brief bounds record: wrong type for ceiling`.
- Mutation MF1a (maximum Ceiling decodes to MaxInt64-1) fails the exact check at :98 with `ceiling = 9223372036854775806, want 9223372036854775807`. The same mutation PASSES against the pre-fold test file, which is the direct proof that the fold changed the proof and not only the lint.
- Mutation MF1c (Ceiling decoded through float64) fails at :102, so the overflow endpoint still bites.
- Staticcheck, pinned as go-gate.sh:498 pins it (`go run honnef.co/go/tools/cmd/staticcheck@v0.8.0`, module v0.8.0 present in the local module cache, served offline with GOPROXY=file:///Users/wido/go/pkg/mod/cache/download and GOSUMDB=off): on the folded copy `./internal/dispatch` exits 0 with no output. On the pre-fold control copy the same command exits 1 with `internal/dispatch/brief_bounds_record_test.go:90:45: no value of type int64 is greater than math.MaxInt64 (SA4003)`. SA4003 is gone.

## Check 3. F-2 closed

- brief_bounds_record_test.go:46 adds a named case in the `types` subtest: a top-level JSON array of alternating record keys and values, asserted refused.
- Unmutated, that input is refused with `invalid brief bounds record: expected object`, which is the object guard's own message.
- Mutation MF2a (the object requirement at brief_bounds_record.go:65 replaced by `if _ = opening; err != nil {`) fails the new case at :46 with `record admitted: ["schemaVersion",1,...]`. The same mutation PASSES against the pre-fold test file, reproducing the surviving mutant M15 of the closing read and confirming the fold is what kills it.
- Mutation MF2b (a partial weakening that accepts `[` alongside `{`) also fails at :46, so the case catches weakening and not only outright removal.

## Check 4. Unmutated suite

`go test -count=1 -timeout 40m ./internal/dispatch -run '^TestBriefBoundsRecordSchema$' -v` on the pristine copy: all 14 subtests PASS, `ok ... 0.275s`, exit 0. `gofmt -l internal/dispatch` is empty and `go vet ./internal/dispatch` exits 0. The builder's verification claims reproduce, including both failure lines it reported (:98 and :46).

### F-11. Severity low. Material no.

Claim: the `bytes` subtest still uses the weak endpoint assertion whose ceiling twin F-1 replaced, so a decoder that returns a wrong maximum admittedBytes is not caught.

Evidence: brief_bounds_record_test.go:64-68 decodes admittedBytes 0 and 9223372036854775807 and only fails when `r.AdmittedBytes < 0`. Mutation MF3 inserted `if r.AdmittedBytes == 1<<63-1 { r.AdmittedBytes = 1 }` before the boundary decode in brief_bounds_record.go; `go test -count=1 -timeout 40m ./internal/dispatch -run '^TestBriefBoundsRecordSchema$' -v` exited 0 with `bytes` PASS. The mutant survives. This is pre-existing: it is identical in the pre-fold file, it is outside the fold brief's two findings and its one-file Boundary, staticcheck does not flag it because the comparison is not always false, and the closing read did not raise it. Not material to this fold round. Recorded for the seat, which may fold it into a later unit or leave it.

### F-12. Severity medium. Material no.

Claim: the mutation proofs for this fold were applied to brief_bounds_record.go inside the live worktree rather than in a copy, so a mutated production file existed there for part of the window in which the seat's race suite was running.

Evidence: brief_bounds_record.go has mtime 2026-09-15 08:35:35, later than the closing read at 08:32:36, while its bytes are unchanged (sha 8471a306 both before and after). A mtime change without a content change is what apply-run-restore in place leaves behind. The fold brief instructed exactly this ("Apply the mutation, run, restore, check the sha256") and the byte-identity non-goal is met, so nothing about the returned unit is wrong and the finding is not material. It is reported because any concurrent compile in that worktree between roughly 08:33 and 08:40 could have read a mutated decoder: if the seat's race suite spans that window, discount it and rerun rather than trusting it. The durable fix is to make mutation proofs run in a copy, as this read and the closing read both did.

## Mutation table

Every mutation edited only /tmp/opus-u3ai2-mut/metasystem/internal/dispatch/brief_bounds_record.go, one exact string replacement with its occurrence count asserted at 1. Each run was `go test -count=1 -timeout 40m ./internal/dispatch -run '^(TestBriefBoundsRecordSchema|TestOpusRead2Probe)$' -v` (MF3 used `-run '^TestBriefBoundsRecordSchema$' -v`). After every mutation the two files were restored from the pristine copy and sha256-matched (prod 8471a306, test 71f133aa), and the mutation directory was finally confirmed identical to the pristine copy by `diff -r`. "Pre-fold test" rows swap in the reviewed pre-fold test file 829fb0cb and are the discriminating control.

| Id | Finding | Mutation | Test file | Result |
| --- | --- | --- | --- | --- |
| U0 | none | none | folded | Exit 0. 14 of 14 subtests PASS. Probe: top-array refused with "expected object", maximum ceiling admitted as exactly 9223372036854775807, overflow refused |
| MF1a | F-1 | maximum Ceiling decodes to MaxInt64-1 | folded | FAIL, ceiling-range, :98 `ceiling = 9223372036854775806, want 9223372036854775807` |
| MF1a | F-1 | same mutation | pre-fold | PASS, exit 0. The pre-fold assertion never saw it |
| MF1c | F-1 | Ceiling decoded through float64 | folded | FAIL, ceiling-range, :102 record admitted with ceiling 9223372036854775808. The overflow case still bites |
| MF2a | F-2 | top-level object requirement removed (`if _ = opening; err != nil`) | folded | FAIL, types, :46 record admitted (the alternating array). Probe: top-array err=nil |
| MF2a | F-2 | same mutation | pre-fold | PASS, exit 0. Reproduces surviving mutant M15 of the closing read |
| MF2b | F-2 | top-level array accepted alongside object | folded | FAIL, types, :46 record admitted. Partial weakening is caught |
| MF3 | F-11 | maximum admittedBytes decodes to 1 | folded | PASS, exit 0, bytes PASS. SURVIVES (pre-existing, out of scope) |

## Static and conformance checks

| Check | Expected | Observed |
| --- | --- | --- |
| `shasum -a 256 -c u3a-i-prod-before-fold.sha` from the live worktree root | OK | OK, at start and before return |
| Live file hashes at start (08:42) and before return (08:47) | unchanged | prod 8471a306eb0649bb0466c7c3c894a928a51361b02910aa8c5077cbc3061bb40b, test 71f133aa325d8687b99364233609f152fe7f2214609708d001dca2f80f4cb573, both times. HEAD still 4eb2b85b, `git status --short` still the two files only |
| Pre-fold to post-fold test diff | only F-1 and F-2 | two hunks, one per finding |
| Subtests and assertion calls | none removed | 14 subtests both sides; assertRecordRefused 19 to 20, others unchanged |
| Unit size | at most 400 lines | 277 (125 + 152) |
| `gofmt -l internal/dispatch` | clean | empty |
| `go vet ./internal/dispatch` | clean | exit 0 |
| Pinned staticcheck v0.8.0, folded copy | exit 0 | exit 0, no output |
| Pinned staticcheck v0.8.0, pre-fold control | SA4003 | exit 1, SA4003 at test line 90. The fold is what removes it |
