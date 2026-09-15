VERDICT: send back

Unit 3a-i of brief-declares-the-round-boundary (row R1, the closed admitted-record codec). Independent closing read by Opus. Live worktree HEAD 4eb2b85bc96c641f123524ff3aafe8897ad97527 with two intent-to-add files. Every probe and mutation ran in private copies under /tmp (git archive of HEAD plus the two files, sha256 matched against the live files). None of the copies sits inside a Git repository. GOCACHE was /tmp/opus-u3ai-gocache, GOTMPDIR and TMPDIR were /tmp/opus-u3ai-tmp, and METASYSTEM_BIN and GIT_DIR were unset. No race suite or fixture bed ran. Material findings: 2 (F-1, F-2).

### F-1. Severity medium. Material yes.

Claim: the new test file fails the pinned staticcheck that go-gate --fast runs, so the unit cannot pass the seat's gate as returned. The flagged clause is also a dead assertion, so the ceiling-range upper endpoint never checks the decoded value.

Evidence:
- metasystem/internal/dispatch/brief_bounds_record_test.go:90 reads `if r.Ceiling == nil || *r.Ceiling < 0 || *r.Ceiling > math.MaxInt64 {`. The last clause can never be true for an int64.
- The private copy was /tmp/opus-u3ai-copy/metasystem. The command was `go run honnef.co/go/tools/cmd/staticcheck@v0.8.0 ./internal/dispatch`, served offline from the module cache with GOPROXY=file:///Users/wido/go/pkg/mod/cache/download and GOSUMDB=off. It printed `internal/dispatch/brief_bounds_record_test.go:90:45: no value of type int64 is greater than math.MaxInt64 (SA4003)` and exited 1.
- The same command on a copy of HEAD without the two new files (/tmp/opus-u3ai-base) exited 0. This unit introduces the refusal.
- metasystem/scripts/agents/go-gate.sh:493-500 runs `go run honnef.co/go/tools/cmd/staticcheck@v0.8.0 ./...` in fast mode and records any refusal as a static red.
- The builder's verification section (codex-bdrb-u3a-i-result.md, "Verification, in required order") ran gofmt, build, vet and the race test only. It could not have seen this.
- Artifact to change: brief_bounds_record_test.go:88-93. Assert that the decoded ceiling equals each endpoint instead of making the impossible comparison.

### F-2. Severity low. Material yes.

Claim: no test covers the decoder's top-level object rule, so the brief's proof rule is not met for a rule the builder added. The rule is the clause `opening != json.Delim('{')` at brief_bounds_record.go:65. With that clause removed, TestBriefBoundsRecordSchema stays green and a top-level JSON array of alternating keys and values decodes as a valid record.

Evidence:
- Mutation M15 in /tmp/opus-u3ai-mut changed `if err != nil || opening != json.Delim('{') {` to `if _ = opening; err != nil {`. Then `go test -count=1 -timeout 40m ./internal/dispatch -run '^(TestBriefBoundsRecordSchema|TestOpusMutProbe)$' -v` exited 0 with no failing subtest.
- The probe input was `["schemaVersion",1,"jobId","impl-r2","rootJob","impl","round",2,"admittedSha256","<64 a>","admittedBytes",123,"boundary",["metasystem/internal/"],"ceiling",400]`. Under the mutation the probe logged `MUTPROBE top-array-alternating ADMIT version=1 digestLen=64 ceiling=400`.
- Unmutated, the same input is refused with `invalid brief bounds record: expected object`.
- Cause: encoding/json's Decoder.Token and Decoder.Decode walk array elements the same way the loop at lines 68-85 walks object keys and values. The closing Token call at line 86 checks only its error. The '{' comparison is therefore the only guard.
- No subtest decodes a document that is not an object (test file lines 16-108).
- The brief states the proof rule verbatim: "for every rule you add, a test that fails when that rule alone is removed". The page calls brief-bounds.json a closed schema, and row R1.types says "Reject wrong JSON field types".
- Artifact to change: brief_bounds_record_test.go. Add a case that is not an object, for example the array above, to the types subtest (or to whichever row the seat assigns).

### F-3. Severity low. Material no.

Claim: two witnesses catch removal of their rule but not a plausible partial weakening. The version subtest probes only version 2, and the digest subtest has no digest longer than 64 characters.

Evidence:
- M18 changed `r.SchemaVersion != 1` to `r.SchemaVersion > 1`. Both the named run (`-run '^TestBriefBoundsRecordSchema$/^version$'`) and the full run exited 0. The probe logged `MUTPROBE version-0 ADMIT version=0`.
- M19 checked the digest only on its first 64 characters. Both the named run (`/^digest$`) and the full run exited 0. The probe logged `MUTPROBE digest-65 ADMIT ... digestLen=65`.
- The shipped code refuses both inputs (probe rows P-1 and P-14). Removing each rule outright does fail its test (M1 at :19, M8 at :57), so the brief's rule is met. This is recorded for the seat only.

### F-4. Severity low. Material no.

Claim: the builder's "required" mutation is not the missing-key loop removed on its own. That loop (brief_bounds_record.go:92-96) is redundant in behavior, because json.Unmarshal of an absent (nil) value already fails and is refused as a wrong type.

Evidence:
- M17 changed `if fields[key] == nil {` to `if false && fields[key] == nil {`. The named run (`/^required$`) exited 0, and so did the full run.
- Under M17 the probes still refuse, only with a different message: `missing-schemaVersion refuse: invalid brief bounds record: wrong type for schemaVersion` and `missing-both-nullable refuse: ... wrong type for boundary`.
- The builder's own variant treats absent nullable members as null. Reproduced as M2, it fails at :27 as reported. M17 is an equivalent mutant, so no extra test is owed.

### F-5. Severity low. Material no.

Claim: the codec admits some spellings the page leaves open. A JSON -0 for ceiling or admittedBytes decodes to 0. Identity strings are checked only for emptiness. Repeated Boundary members are accepted.

Evidence:
- Probe rows P-21, P-22 and P-25 all decode: ceiling -0 as ceiling=0, admittedBytes -0 as bytes=0, jobId " ", a JSON-escaped NUL and "../x", and boundary ["a","a"].
- The header parser refuses the Ceiling text "-0" (brief.go, the digit check before strconv.ParseInt) but also accepts repeated members.
- The page requires a nonnegative int64 value and a nonempty identity (R1.ceiling-range, R1.identity). Both hold.
- Marshal never writes -0. Unit 3b compares identity with the supplied job, root and round, and binds the exact record bytes through recordSha256.
- A Boundary member holding invalid UTF-8 is written by Marshal as U+FFFD and does not decode back to the same value (P-28). Admission cannot produce such a member, because brief.go already decodes the Boundary header with json.Unmarshal.

## Probe table

Rows C-1 to C-8 are conformance checks. Rows P-1 to P-28 are 144 decode probes and 25 Marshal probes, run by a throwaway test in /tmp/opus-u3ai-probe through the real exported codec, with `go test -count=1 -timeout 40m ./internal/dispatch -run '^TestOpusProbeBriefBoundsRecord$' -v`. The run logged `DECODE mismatches=0 of 144 probes`: every probe with a stated expectation matched it. Rows marked "open" are behaviors the page does not decide.

| Id | Probe | Expected | Observed |
| --- | --- | --- | --- |
| C-1 | Changed paths: `git diff HEAD --name-status` in the live worktree | only the two Boundary files | `A metasystem/internal/dispatch/brief_bounds_record.go` and `A metasystem/internal/dispatch/brief_bounds_record_test.go`, nothing else |
| C-2 | composition.go, build.go and composition_brief_bounds_test.go against base | unchanged | `git diff HEAD` over the three paths is 0 lines. composition_brief_bounds_test.go exists neither in HEAD nor in the worktree, and `git status --short --ignored internal/dispatch` shows only the two new files |
| C-3 | Ceiling | at most 400 changed lines | numstat 125 + 145 = 270 |
| C-4 | Non-goals and other files | no other change | No other tracked or unignored path. The brief and result reports sit under artifacts/, which metasystem/.gitignore:1 ignores |
| C-5 | Dependence on 3a-ii | none | HEAD plus only the two files builds, vets and passes. The code uses only base identifiers: BriefBounds (brief.go:50), ValidateBriefBounds (brief.go:203) and incarnationRe (build.go:21). There is no marker, AdmittedBounds or composition symbol. Building the record from the admission value is row R4, unit 3c (page lines 634-638) |
| C-6 | `gofmt -l internal/dispatch`, `go vet ./internal/dispatch`, `-run '^TestBriefBoundsRecordSchema$' -v` | clean, 14 subtests pass | empty, vet ok, 14 subtests PASS |
| C-7 | Pinned staticcheck v0.8.0 on ./internal/dispatch | exit 0 | exit 1, SA4003 at test file line 90 (F-1). Base without the unit exits 0 |
| C-8 | Live file sha256 at start (08:08) and before return | unchanged | brief_bounds_record.go 8471a306eb0649bb0466c7c3c894a928a51361b02910aa8c5077cbc3061bb40b and brief_bounds_record_test.go 829fb0cb268845255369d2f0fa5dd0b8f87b27f20036f99ac0fab4fcab1c7ffc both times |
| P-1 | schemaVersion 2, 0, -1, 1.0, 1e0, "1", null, 4294967297 | refuse | all refused |
| P-2 | Each of the eight keys absent; both nullable keys absent; boundary absent with ceiling null; ceiling absent with boundary null | refuse | all refused |
| P-3 | Unknown keys: "extra":true, "extra":null, "JobId" in place of jobId, an empty key | refuse | all refused |
| P-4 | Top-level duplicates: round twice with the same value; jobId twice with different values; ceiling null then 400; boundary twice; jobId followed by a JSON-escaped spelling of jobId; ceiling repeated as the last key | refuse | all refused with "duplicate or invalid key" |
| P-5 | Nested duplicates: boundary [{"a":1,"a":2}], ceiling {"a":1,"a":1}, jobId {"x":1,"x":2}, boundary {"a":1,"a":2} with a null ceiling | refuse | All refused as wrong types. No field accepts a nested object, so a nested duplicate can never reach a decoded value |
| P-6 | A JSON-escaped spelling of jobId used once | admit | admitted |
| P-7 | Trailing or malformed input: " {}", "x", "]", "}", " null", " 1", ",", a NUL byte, a second record, a truncated record, a trailing comma inside the object, a missing comma, "]" closing the object, a UTF-8 BOM, empty input | refuse | all refused |
| P-8 | Whitespace: a trailing newline; newlines, space, tab and CR after the object; space, newline and tab before it | admit | admitted |
| P-9 | Top-level null, [], "x", 1, and the alternating key/value array | refuse | all refused with "expected object". F-2 covers the untested guard |
| P-10 | Scalar types: round 2.0, 2e0, true, null, -0; admittedBytes 1.5, 1e3, null; jobId null; admittedSha256 null | refuse | all refused |
| P-11 | Nullable types: ceiling 400.0, 400.5, 4e2, true, [400], "400". Boundary "metasystem/internal/", [1], [null], ["metasystem/internal/",null], [["a"]]. Boundary {}, false or [1] with a null ceiling. Ceiling "400" or 1.5 with a null boundary | refuse | all refused |
| P-12 | jobId "", rootJob "", round 0, -1, 9223372036854775808, -9223372036854775808 | refuse | all refused |
| P-13 | round 9223372036854775807 | admit exactly | admitted, round=9223372036854775807 |
| P-14 | Digest: 63 a, 65 a, 64 A, one uppercase A then 63 a, 64 g, empty, 64 a followed by an escaped newline, a leading space | refuse | all refused |
| P-15 | Digest whose first character is JSON-escaped, decoding to 64 lowercase a | admit | admitted |
| P-16 | admittedBytes -1 and 9223372036854775808 refused; 0 and 9223372036854775807 admitted | as listed | as listed |
| P-17 | Partial pairs: null boundary with ceiling 400; array boundary with null ceiling; [] with null ceiling | refuse | refused with "required with Ceiling" or "required with Boundary" |
| P-18 | Null pair, both compact and spaced with a tab | admit with both nil | admitted, boundary=[]string(nil), ceiling nil |
| P-19 | Empty boundary [] with ceiling 400, and with ceiling 0 | admit, non-nil empty | admitted, boundary=[]string{} |
| P-20 | Ceiling 0 and 9223372036854775807 admitted. Ceiling 9223372036854775808, -1, -9223372036854775808, -9223372036854775809, 18446744073709551615, 1e19 refused | as listed | as listed. The maximum decodes exactly as 9223372036854775807 |
| P-21 | ceiling -0 | open | admitted as 0 (F-5) |
| P-22 | admittedBytes -0 | open | admitted as 0 (F-5) |
| P-23 | Member values refused: "", "/a", "/", "a//b", "a/./b", "a/../b", ".", "..", "./a", "a/..", "a//", a member with a JSON-escaped NUL, and ["a","/b"] | refuse | all refused |
| P-24 | Member values kept literally: "a/", "a[", a member holding one backslash, "*", "a/*.go", "a b" | admit unchanged | Admitted unchanged. No pattern is interpreted |
| P-25 | jobId " ", a JSON-escaped NUL, "../x"; boundary ["a","a"] | open | admitted (F-5) |
| P-26 | Marshal: version 0 or 2, empty jobId or rootJob, round 0, uppercase digest, bytes -1, nil boundary with a ceiling, nil ceiling with an array, [] with a nil ceiling, ceiling -1, member "" | refuse with no bytes | all refused with empty data |
| P-27 | Marshal round trips: valid record, null pair, empty boundary, ceiling 9223372036854775807, member "a<b&c>/" (written with JSON HTML escapes), member "a[" | decode equal, trailing newline | all decoded equal, each with a trailing newline |
| P-28 | Marshal of a member holding the invalid UTF-8 byte 0xff | open | Written as U+FFFD, and the decoded value differs (F-5) |

## Mutation table

Each mutation edited only /tmp/opus-u3ai-mut/metasystem/internal/dispatch/brief_bounds_record.go, one exact string replacement per edit with its occurrence count asserted. Two runs followed each edit:
- Named run: `go test -count=1 -timeout 40m ./internal/dispatch -run '^TestBriefBoundsRecordSchema$/^<row>$' -v`.
- Full run: the same command with `-run '^(TestBriefBoundsRecordSchema|TestOpusMutProbe)$'`.

After every mutation the file was restored from the pristine copy, its sha256 matched 8471a306eb0649bb0466c7c3c894a928a51361b02910aa8c5077cbc3061bb40b, and the restored schema test exited 0. The "Builder" column marks the builder's own mutation for that row (yes) or an extra one (no). All 14 builder rows were re-run and every one failed at the line the builder reported. For types, the builder's mutation differs from mine: it reported :40, and my M6c fails at :43.

| Id | Row | Builder | Change | Named run result (first assertion line) | Full run failing subtests |
| --- | --- | --- | --- | --- | --- |
| M1 | version | yes | Version refusal guarded by false | fail, :19 record admitted with schemaVersion 2 | version |
| M2 | required | yes | Absent boundary and ceiling treated as null (loop skips them; both nullable decoders skip absent values) | fail, :27 record admitted with both nullable keys absent | required |
| M3 | unknown | yes | Allowed-key check guarded by false | fail, :29 record admitted with "extra":true | unknown |
| M4 | duplicate-key | yes | Duplicate check guarded by false, so a later value replaces the earlier one | fail, :30 record admitted with "round":2,"round":2 | duplicate-key |
| M5 | trailing-json | yes | io.EOF comparison guarded by false | fail, :31 record admitted with a trailing {} | trailing-json |
| M6 | types | no | Typed-null clause removed for scalar fields | fail, :40 record admitted (admittedBytes null) | types |
| M6b | types | no | Scalar unmarshal error ignored | fail, :40 record admitted | types, bytes |
| M6c | types | yes (nearest) | Boundary unmarshal error ignored | fail, :43 record admitted (boundary {} with null ceiling) | types |
| M6d | types | no | Ceiling unmarshal error ignored | fail, :40 record admitted | types, ceiling-range |
| M7 | identity | yes | Empty-jobId clause removed | fail, :49 record admitted with "jobId":"" | identity |
| M7b | identity | no | Empty-rootJob clause removed | fail, :49 record admitted with "rootJob":"" | identity |
| M7c | identity | no | Round bound weakened from below 1 to below 0 | fail, :49 record admitted with "round":0 | identity |
| M8 | digest | yes | Digest check replaced by false | fail, :57 record admitted with a 63-character digest | digest |
| M8b | digest | no | Digest lowercased before matching | fail, :57 record admitted with an uppercase digest | digest |
| M9 | bytes | yes | Nonnegative byte check replaced by false | fail, :61 record admitted with admittedBytes -1 | bytes |
| M10 | pair | yes | Partial pair normalized to the null pair before ValidateBriefBounds | fail, :70 record admitted with a null boundary and ceiling 400 | pair |
| M11 | null-pair | yes | Null pair refused | fail, :74 "invalid brief bounds record: unbounded records are unsupported" | null-pair |
| M11b | null-pair | no | Marshal writes a nil boundary as [] | fail, :78 "BRIEF_BOUNDS_INVALID: Ceiling: required with Boundary" | null-pair |
| M12 | empty-boundary | yes | Decoded [] collapsed to nil | fail, :81 "BRIEF_BOUNDS_INVALID: Boundary: required with Ceiling" | empty-boundary |
| M12b | empty-boundary | no | boundary struct tag given omitempty | fail, :85 "invalid brief bounds record: missing boundary" | null-pair, empty-boundary |
| M13 | ceiling-range | yes | Negative ceiling normalized to 0 before ValidateBriefBounds | fail, :94 record admitted with ceiling -1 | ceiling-range |
| M13b | ceiling-range | no | Ceiling decoded through float64 | Fail, :95 record admitted with 9223372036854775808. Under this mutation the probes also admit 400.5 as 400 and round 9007199254740993 to 9007199254740992, and only the overflow endpoint catches it | ceiling-range |
| M14 | member-values | yes | Members replaced with "a" for validation only | fail, :99 record admitted with boundary [""] | member-values |
| M14b | member-values | no | Codec runs path.Match on pattern members | fail, :102 "invalid brief bounds record: syntax error in pattern" | member-values |
| M15 | none | no | Top-level '{' comparison removed | No named run (no row owns it). Full run exit 0, and the alternating array is admitted (F-2). SURVIVES | NONE |
| M16 | version | no | Marshal-side validation guarded by false | fail, :21 "record marshaled: {" | version |
| M17 | required | no | Missing-key loop guarded by false on its own | Pass, exit 0. Equivalent mutant: missing keys are still refused as wrong types (F-4) | NONE |
| M18 | version | no | Version check weakened from not-equal-1 to greater-than-1 | Pass, exit 0, and version 0 is admitted (F-3). SURVIVES | NONE |
| M19 | digest | no | Digest checked only on its first 64 characters | Pass, exit 0, and a 65-character digest is admitted (F-3). SURVIVES | NONE |
| M20 | null-pair | no | Marshal drops the trailing newline | fail, :78 "record lacks trailing newline" | null-pair, empty-boundary |

Rules with no failing test: the top-level object requirement at brief_bounds_record.go:65 (M15, F-2). The explicit missing-key loop also has no failing test, but removing it changes no behavior (M17, F-4). Every R1 row has a subtest that fails when its rule is removed.
