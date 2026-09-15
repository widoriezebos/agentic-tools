VERDICT: send back

Unit 3a-ii of brief-declares-the-round-boundary (rows R2 and R3: the admittedBrief composition marker and the optional eighth source field in job composition admission). Independent closing read by Opus. I did not write this code. Live worktree .claude/worktrees/bdrb-u3a-ii at base 300ecfc6 with the unit uncommitted. The live worktree was never modified. All runs used private copies built from `git archive HEAD` plus the three changed files, sha256-matched against the live files: /tmp/opus-u3aii-copy (pristine), /tmp/opus-u3aii-mut (mutations), /tmp/opus-u3aii-probe (throwaway probe tests). None sits inside a Git repository. GOCACHE was /tmp/opus-u3aii-gocache, GOTMPDIR and TMPDIR were /tmp/opus-u3aii-tmp, and METASYSTEM_BIN, GIT_DIR and METASYSTEM_CONTEXT_COST_PROOF were unset. Only focused `-run` tests on ./internal/dispatch ran. No race suite, fixture bed or engine test ran. Material findings: 1 (F-1).

Conformance and the attack questions, in brief:
- Changed paths are exactly the Boundary: `git diff HEAD --name-status` lists M build.go, M composition.go and A composition_brief_bounds_test.go. `git status --porcelain --ignored --untracked-files=all` over internal, cmd, scripts and plans lists nothing else. brief_bounds_record.go and its test are unchanged. No shell, cmd/metasystem, validate, review reader or return schema file changed.
- Size: numstat 15+1, 36+9 and 256+0, total 317 lines against the 400 ceiling.
- No prose or header parsing in composition. The diff calls no ParseBriefBounds and reads only the supplied record, decoded with the landed DecodeBriefBoundsRecord (composition.go:236). Mutation A1d swaps in json.Unmarshal plus value validation as a second checker, and the marker subtest fails at :95.
- recordSha256 is the SHA-256 of the exact bytes read (composition.go:242). A1 (trimmed bytes) and A1b (canonical re-marshal) both fail at :75. Probes with CRLF whitespace and a symlinked path give a marker hash equal to the file's bytes.
- bounded follows paired presence (`Boundary != nil`). A2 (nonempty Boundary) and A3 (Ceiling nil also bounded) fail. The stored unbounded marker is `{"bounded":false,"recordSha256":"902916db...","schemaVersion":1}`.
- The marker sits only on the task-direction caller:brief source. Probes show exactly one marked source, at index 0, for inline, referenced and continuation compositions, and none for a legacy composition. X1 (marker on every source) fails eight-fields.
- Job composition admission admits the seven-field legacy source and the eight-field marked source (bounded and unbounded, inline and referenced). It refuses the marker on another slot or source, an unrelated eighth field, nine fields, a null, array or empty marker, a misspelled key, an extra null key, versions 0, -1, 2, 1.0, 1e0, "1", true and null, and a bounded value of null, 0, 1, "false" or an array.
- Digest pattern: the check uses incarnationRe, `^[0-9a-f]{64}$` (build.go:21). Without the m flag, Go's `$` matches only at end of text. Probes refuse 63 and 65 characters, a trailing newline, uppercase, g, empty, a number and null.
- Other readers of composition.json accept an eight-field source. The only production admission callers are build.go:453 and :793, both through readCompositionForJob. VerifyReferences decodes with DisallowUnknownFields (references.go:59-70) into the extended struct, and B10 proves the extension is load-bearing. validate/conformance.go:938-941, cmd/metasystem/dispatch_verbs.go:183-187 and the cmd test reader use plain json.Unmarshal. dispatch-fixtures.sh:1795 and :4188 substring-match `"source":"engine:..."`. validateAfterCapPacket reads only `source`. No jq, Python or other Go code names sourceDigest, deliveredDigest or startByte for a composition source.
- Error text: seven-field texts are unchanged, and no consumer matches any composition source text (F-6).
- The tests use the real validators. compositionAdmission (:161-177) runs readCompositionForJob on the file ComposeRolePacket wrote, and reference-reader runs VerifyReferences. The test file contains no second schema checker.
- Static checks on the copy: `gofmt -l internal/dispatch` printed nothing. `go vet ./internal/dispatch` and `go build ./...` exited 0. The pinned staticcheck (`go run honnef.co/go/tools/cmd/staticcheck@v0.8.0 ./internal/dispatch`, served offline from the module cache) exited 0 with no output. Unmutated, all 19 subtests pass.

### F-1. Severity medium. Material yes.

Claim: no test puts a bounded:false marker, the headerless case every brief without the paired headers will produce, through the stored composition or the real admission validator, so two one-token mutations that make job admission refuse every marked headerless composition leave both new test functions green.

Evidence:
- The page, section "Which brief binds a round", says the dispatcher writes the marker "for both bounded and headerless briefs". Unit 3a (design page line 363) says "Test the legacy source and the new source with the actual job composition admission validator". Row R3.eight-fields (line 625) reads "Admit the seven fields plus a valid marker".
- composition_brief_bounds_test.go:185 fixes `bounded := testBoundsRecord(true)` for the admission function, so eight-fields (:191-195) admits only a bounded marker. unbounded (:105-111) checks only the in-memory `got.Sources[0].AdmittedBrief.Bounded`. marker (:85-89) checks the stored schemaVersion and recordSha256 but not the stored bounded key.
- X2 changed composition.go's `json:"bounded"` to `json:"bounded,omitempty"`. `go test -count=1 -timeout 40m ./internal/dispatch -run '^(TestCompositionAdmittedBounds|TestCompositionAdmittedBoundsAdmission)$' -v` exited 0 with no failing subtest. The probe under X2 shows the stored unbounded marker as `{"recordSha256":"902916db...","schemaVersion":1}`. readCompositionForJob refuses it with `composition source 0 has invalid admitted brief evidence`, inline and referenced, while bounded compositions still admit.
- X3 made build.go require bounded true (`boundedOK = boundedOK && boundedValue`). The same command exited 0. The probe under X3 refuses the unbounded inline and referenced compositions and a hand-set `bounded:false`, each with the same message.
- Unmutated, the behavior is right: the stored unbounded marker keeps `"bounded":false`, and all three of those probes admit. The defect is in the proof, not the code.
- Nothing else in the repository would catch either mutant. `admittedBrief`, `AdmittedBrief` and `AdmittedBounds` occur only in composition.go, build.go and the new test file (a grep over the worktree excluding plans, records and artifacts). No CLI flag passes a record until unit 3c.
- Artifact to change: composition_brief_bounds_test.go. For example, eight-fields also admits `testBoundsRecord(false)`, and unbounded (or marker) asserts that the stored marker keeps a boolean `bounded` key. Either addition kills X3 or X2 respectively.

### F-2. Severity low. Material no.

Claim: two admission witnesses catch removal of their rule but not a plausible partial weakening, because marker-digest probes only an uppercase digest and marker-version probes only version 2.

Evidence:
- X4 checked the digest with `^[0-9a-f]+$` (no length) and X5 refused only versions above 1. Each left both functions green (exit 0).
- Unmutated, the probes refuse 63- and 65-character digests and versions 0 and -1.
- Removing each rule outright fails its subtest (B6 at :223, B4 at :213), so the brief's proof rule is met. This has the same shape as the 3a-i read's F-3 and is recorded for the seat only.

### F-3. Severity low. Material no.

Claim: the builder's mutation table reports failure lines from an earlier test file for 18 of its 19 rows, so on its own it did not show those rows against the returned test file.

Evidence:
- The result report says so ("The later critique correction added assertions inside the preceding marker subtest, shifting the unchanged later assertions").
- Every row after marker is offset by 28 lines. For example, bounded was reported at :74 and fails at :102 now, and extra-source-field was reported at :218 and fails at :246 now.
- My re-runs against the returned file (sha256 e1be85f4645eb7cc37332018d7b114d02c64d8eaa6f1746427465744b04962e8) fail every R2 and R3 row under a one-rule mutation. Thirteen of those runs use the builder's own mutation or its equivalent (table below), so the gap is closed.

### F-4. Severity low. Material no.

Claim: job admission checks only the marker's shape and placement, so it admits a valid-looking marker added by hand to a legacy composition, and a second forged task-direction caller:brief source carrying a copy of the marker.

Evidence:
- Probe results: marker added by hand on a legacy composition, ADMIT. Second source rewritten to task-direction and caller:brief with a copied marker, ADMIT.
- The duplicate identity without a marker was already admitted before this unit (probe: legacy second task-direction source, ADMIT).
- Row R3 asks only for shape, version, boolean, digest, slot and source. The page puts "exactly one caller:brief task-direction source", the record hash and the sibling files in the unit 3b reader (section "Which brief binds a round", the paragraphs after the reader signature). Recorded so unit 3b's brief keeps those refusals in its reader.

### F-5. Severity low. Material no.

Claim: composition reads and decodes the supplied record before it validates the role recipe, hazard configuration and inline limit, follows a symlinked record path and ignores rootJob, so its refusal precedence and file checks differ from the unit 3b reader's.

Evidence:
- Probe results: a bad role combined with the record `{}` returns `invalid brief bounds record: missing schemaVersion` rather than the recipe refusal. A symlinked record is admitted, hashed over the target's bytes. A record whose rootJob differs is admitted.
- Legacy callers pass no record (the block at composition.go:230 runs only when `p.AdmittedBounds != ""`), so no current caller sees a change.
- Item 3 of the page's dispatcher list (line 130) asks composition only for job and round identity and the exact-byte hash. Regular-file and root checks belong to unit 3b (line 145). Recorded for unit 3c, which chooses the path passed and the CLI refusal mapping.

### F-6. Severity low. Material no.

Claim: an eight-field source's "invalid shape" refusal now comes after the presence and identity checks, so some malformed eight-field sources get a different error text than before, and no consumer matches these texts.

Evidence:
- build.go:1036-1039 place the eight-fields-without-marker refusal after the identity check at :1028-1035.
- An unmarked eight-field source with a bad sourceDigest now reports `composition source 1 has invalid identity, digests, or byte range`. The pre-unit code, which required exactly seven fields, reported `has an invalid shape`.
- A marked source with endByte renamed now reports `composition source 0 is missing endByte`.
- A nine-field source and a legacy source with an extra field still report `has an invalid shape`.
- A grep across internal/, cmd/ and scripts/ for "composition source", "invalid shape", "invalid identity, digests" and "admitted brief evidence" finds build.go itself and unrelated mission, lease and rebase texts only. No test or shell script matches a composition source error.

## Mutation table

Every mutation edited only files in /tmp/opus-u3aii-mut/metasystem/internal/dispatch (composition.go, build.go, or references.go for B10), with one exact string replacement per edit and its occurrence count asserted at 1. Two runs followed each edit:
- Named run: `go test -count=1 -timeout 40m ./internal/dispatch -run '^TestCompositionAdmittedBounds$/^<row>$' -v`, or `^TestCompositionAdmittedBoundsAdmission$/^<row>$` for R3 rows.
- Full run: the same command with `-run '^(TestCompositionAdmittedBounds|TestCompositionAdmittedBoundsAdmission)$'`.

X rows ran only the full run. After every mutation the harness restored the three production files from the pristine copy and asserted their sha256 (build.go 9f9f08de..., composition.go 603e5de3..., references.go 0f09c075...) and the test file's (e1be85f4...). The final restore matched all three. The "Builder" column marks whether the change is the builder's own mutation for that row or an equivalent (yes) or a different one-rule mutation (no).

| Id | Row | Builder | One-rule change | Named run, first failure line | Full run failing subtests |
| --- | --- | --- | --- | --- | --- |
| A1 | R2.marker | no | Marker hash over bytes.TrimSpace(data) | FAIL :75 `marker = &dispatch.AdmittedBriefMarker{SchemaVersion:1, Bounded:true, RecordSHA256:"ca1b3a2c..."}` | marker |
| A1b | R2.marker | yes | Marker hash over a canonical re-marshal | FAIL :75, RecordSHA256 "83d9cfae..." | marker |
| A1c | R2.marker | yes (supplemental) | Marker tag `json:"-"` | FAIL :88 `stored marker = map[string]interface {}(nil)` | marker, marker-shape |
| A1d | R2.marker | yes (supplemental, stronger) | Landed decoder replaced by json.Unmarshal plus validateBriefBoundsRecord | FAIL :95 `unknown admitted record field was accepted` | marker |
| A2 | R2.bounded | no | Bounded from `len(Boundary) > 0` | FAIL :102 `empty Boundary was not bounded` | bounded |
| A3 | R2.unbounded | no | Bounded also true when Ceiling is nil | FAIL :109 `null pair was bounded` | unbounded |
| A4 | R2.job | yes | Job comparison removed | FAIL :116 `wrong job admitted` | job |
| A5 | R2.round | yes | Round comparison removed | FAIL :123 `wrong round admitted` | round |
| A6 | R2.legacy | yes | Non-nil zero marker without a record | FAIL :129 `legacy source gained marker` | legacy, legacy-seven-fields |
| A6b | R3.legacy-seven-fields | no | omitempty dropped from the admittedBrief tag, so legacy sources store null | FAIL :188 `composition source 0 has invalid admitted brief evidence` | legacy-seven-fields, eight-fields |
| A7 | R2.delivered-source | no | Marked source's SourceDigest and SourceBytes replaced by admittedSha256 and admittedBytes | FAIL :138 `source = dispatch.CompositionSource{Slot:"task-direction", Source:"caller:brief", SourceDigest:"aaaa...` | delivered-source |
| A8 | R2.inline-body | no | Marked task direction gains a delivered line | FAIL :147 `range = "# Task Direction\n\nAdmitted brief evidence recorded.\nexact inline\n\n"` | inline-body |
| A9 | R2.referenced-body | no | Marked task direction exempt from size referencing | FAIL :156 `referenced body changed or was inlined: open ...` (the staged task-direction.md could not be opened) | referenced-body |
| A9b | R2.referenced-body | no | Marked staged body gains a newline | FAIL :156 `referenced body changed or was inlined: <nil>` | referenced-body, reference-reader |
| B1 | R3.legacy-seven-fields | yes | Eight fields required | FAIL :188 `composition source 0 has an invalid shape` | legacy-seven-fields, eight-fields |
| B2 | R3.eight-fields | yes | Seven fields required | FAIL :193 `composition source 0 has an invalid shape` | eight-fields |
| B3 | R3.marker-shape | yes | Three-key length check removed | FAIL :207 `extra=true admitted` | marker-shape |
| B4 | R3.marker-version | yes | Version equality guarded by false. A first attempt removed the clause and did not compile (`declared and not used: version`), so it is not counted | FAIL :213 `version admitted` | marker-version |
| B5 | R3.marker-bounded | yes | Boolean requirement removed | FAIL :218 `non-boolean admitted` | marker-bounded |
| B6 | R3.marker-digest | yes | Digest pattern removed | FAIL :223 `digest admitted` | marker-digest |
| B7 | R3.wrong-slot | yes | Slot restriction removed | FAIL :232 `wrong slot admitted` | wrong-slot |
| B8 | R3.wrong-source | yes | Source restriction removed | FAIL :241 `wrong source admitted` | wrong-source |
| B9 | R3.extra-source-field | yes | Eighth field need not be admittedBrief | FAIL :246 `extra field admitted` | extra-source-field |
| B10 | R3.reference-reader | no | VerifyReferences decodes sources into the legacy seven-field struct | FAIL :253 `typed reader = decode composition record: json: unknown field "admittedBrief", []dispatch.ReferenceMismatch(nil)` | reference-reader |
| X1 | Marker placement (composition.go:414) | no | Marker put on every source | Full run only: FAIL :193 `composition source 1 has invalid admitted brief evidence` | eight-fields |
| X2 | F-1 | no | `json:"bounded,omitempty"` | Full run only: PASS, exit 0. SURVIVES | NONE |
| X3 | F-1 | no | Admission requires bounded true | Full run only: PASS, exit 0. SURVIVES | NONE |
| X4 | F-2 | no | Digest pattern without the 64 length | Full run only: PASS, exit 0. SURVIVES | NONE |
| X5 | F-2 | no | Version refused only above 1 | Full run only: PASS, exit 0. SURVIVES | NONE |
| X6 | Marker version literal | no | Composition writes marker version 2 | Full run only: FAIL :75 `marker = &dispatch.AdmittedBriefMarker{SchemaVersion:2, ...}` | marker, eight-fields |
| X7 | Decode error propagation | no | DecodeBriefBoundsRecord error ignored | Full run only: PASS, exit 0. Equivalent mutant: every decoder error path returns a zero record (brief_bounds_record.go:54-121), and its empty JobID never equals the job ComposeRolePacket requires to be nonempty (composition.go:226), so the identity check still refuses | NONE |

Every R2 and R3 row has a named subtest that fails when its rule alone is removed. The surviving non-equivalent mutants are X2 and X3 (F-1), which break the headerless marker path, and X4 and X5 (F-2), which are partial weakenings.

## Probe table

Throwaway tests TestOpusU3aiiProbe and TestOpusU3aiiProbe2 in /tmp/opus-u3aii-probe called the real ComposeRolePacket, readCompositionForJob (through the unit's own compositionAdmission helper) and VerifyReferences. Command: `go test -count=1 -timeout 40m ./internal/dispatch -run '^TestOpusU3aiiProbe2?$' -v`. The X2 and X3 rows re-ran the probe with that single mutation applied, then restored and sha256-checked composition.go and build.go.

| Id | Probe | Observed |
| --- | --- | --- |
| P-1 | Stored markers: unbounded inline, bounded referenced, bounded with a prior-brief continuation, legacy | One marker at index 0 (task-direction, caller:brief, 8 fields) in each of the first three; the unbounded one is `{"bounded":false,...,"schemaVersion":1}`; legacy has 0 |
| P-2 | Composition refusals: missing record file, empty file, trailing `{}`, top-level `[]`, job "bounds-r" against "bounds-r2" | Refused: `read admitted brief bounds: open ...`, `expected object`, `trailing JSON`, `expected object`, `admitted brief bounds do not bind job bounds-r2 round 2` |
| P-3 | Composition admits: Boundary member with ceiling 400, empty Boundary with ceiling 0, CRLF record, symlinked record, differing rootJob | Admitted; bounded=true and the hash matches the file bytes in each (F-5 for symlink and rootJob) |
| P-4 | Bad role plus the record `{}` | `invalid brief bounds record: missing schemaVersion` (F-5) |
| P-5 | Admission of legacy, bounded, unbounded, bounded referenced, unbounded referenced | All ADMIT |
| P-6 | Digest 63a, 65a, 64a plus newline, empty, 64g, one uppercase, a number, null | All REFUSE `composition source 0 has invalid admitted brief evidence` |
| P-7 | Version 0, 1.0, 1e0, "1", -1, true, null | All REFUSE |
| P-8 | bounded null, 0, 1, "false", [] refused; false admitted | As listed |
| P-9 | Marker null, [], {}, misspelled recordSHA256 key, extra null key | All REFUSE |
| P-10 | Marker copied to source 1; marker on the runtime notice (last source) | REFUSE, naming source 1 and source 5 |
| P-11 | Extra field on marked source 0 (nine fields); legacy source 0 with an extra field | REFUSE `has an invalid shape` |
| P-12 | Marked source 0 with endByte renamed; unmarked source 1 with an extra field and bad digest; legacy source 0 with admittedBrief replacing endByte | `is missing endByte`, `has invalid identity, digests, or byte range`, `is missing endByte` (F-6) |
| P-13 | Valid marker added by hand to legacy source 0; second task-direction caller:brief source with a copied marker; legacy second task-direction source | All ADMIT (F-4) |
| P-14 | VerifyReferences on an unbounded referenced composition | err nil, 0 mismatches |
| P-15 | Under X2: unbounded inline and referenced admission, stored marker | REFUSE both; stored marker lacks bounded (F-1) |
| P-16 | Under X3: unbounded inline and referenced admission, hand-set bounded false | REFUSE all three (F-1) |

Live worktree state: at 10:08 and again after this read was written, HEAD 300ecfc69e7dbd9e27f1e67d8d3df4a308ac0224, `git status --porcelain` lists only the three unit files, and their hashes are unchanged: build.go 9f9f08de4479a995135659a841b2f6481c11dcb11e8b4d44e97042d723094a0a, composition.go 603e5de390803747f98a4c71adede066cf254e64bf7e85549a202de7c920456d, composition_brief_bounds_test.go e1be85f4645eb7cc37332018d7b114d02c64d8eaa6f1746427465744b04962e8, and `git diff HEAD` hashes to 05162936f9bbeb968f72f7cecb3cdef33d35cff48bff8406aefc7d273d55f3e1.
