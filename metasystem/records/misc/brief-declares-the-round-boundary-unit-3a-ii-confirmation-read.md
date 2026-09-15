VERDICT: land

Unit 3a-ii of brief-declares-the-round-boundary, fold round 1. This is the independent confirmation read by Opus, and I did not write this code. The scope is whether the fold closed F-1 and made the F-2 strengthening from the closing read (bdrb-u3a-ii-opus-read.md) without changing anything else. Live worktree: .claude/worktrees/bdrb-u3a-ii, base 300ecfc6, unit uncommitted. I never modified the live worktree. I only read it with `git --no-optional-locks` and shasum.

I ran everything in private copies made from `git archive HEAD` plus the three unit files, each sha256-matched against the live file. /tmp/opus-u3aii-r2-copy was the pristine copy for the unmutated run, gofmt and vet. /tmp/opus-u3aii-r2-mut held the mutations. /tmp/opus-u3aii-r2-probe held one throwaway probe test. None of them is inside a Git repository. GOCACHE was /tmp/opus-u3aii-gocache, GOTMPDIR and TMPDIR were /tmp/opus-u3aii-tmp, and METASYSTEM_BIN, METASYSTEM_CONTEXT_COST_PROOF and GIT_DIR were unset (go1.27.1). Only focused `-run` tests on ./internal/dispatch ran, with no race suite, fixture bed or engine test. Material findings: 0.

The checks, in the order the brief sets:

1. Only the test file changed. Live composition.go hashes to 603e5de390803747f98a4c71adede066cf254e64bf7e85549a202de7c920456d and build.go to 9f9f08de4479a995135659a841b2f6481c11dcb11e8b4d44e97042d723094a0a. Both match the closing read. references.go also still hashes to 0f09c075...
   - `git status --porcelain --ignored --untracked-files=all` over metasystem/internal, cmd, scripts, go.mod and go.sum lists only M build.go, M composition.go and A composition_brief_bounds_test.go.
   - The test file moved from e1be85f4645eb7cc37332018d7b114d02c64d8eaa6f1746427465744b04962e8 (the closing read's copy, still in /tmp/opus-u3aii-copy) to 25f98e11d88a464cf0fb891de9bbf048b0364eac7c43533bf576b41159bf4b93.
   - Both files have the same 19 subtest names in the same order: marker, bounded, unbounded, job, round, legacy, delivered-source, inline-body, referenced-body, legacy-seven-fields, eight-fields, marker-shape, marker-version, marker-bounded, marker-digest, wrong-slot, wrong-source, extra-source-field, reference-reader.
   - The diff against e1be85f4 removes no assertion and touches only three places. unbounded keeps its in-memory check and adds the stored-key check at :111-118. eight-fields keeps its bounded case first and adds unbounded inline (:204) and unbounded referenced (:207). marker-version and marker-digest become loops that keep the original value first (version 2, uppercase digest) and add 0 and -1, and 63 and 65 lowercase characters. Their failure texts now name the value.
   - Numstat is 15+1 for build.go, 36+9 for composition.go and 275+0 for the test file, 336 in total (326 added, 10 deleted), against the 400 ceiling.
2. F-1 is closed.
   - X2 alone fails unbounded at :117 and eight-fields at :205. X3 alone fails eight-fields at :205. Both named runs and the full run confirm this (table below).
   - The closing read recorded both mutants surviving against e1be85f4. Every killing line here is a line the fold added.
   - Unmutated, the probe shows the referenced composition behind :207 is really referenced: one staged task-direction.md reference of 32769 bytes, and one marker at index 0 (task-direction, caller:brief, 8 fields) equal to `{"bounded":false,"recordSha256":"902916db...","schemaVersion":1}`. readCompositionForJob admits it, and admits the inline twin too. So the eight-fields referenced case does put a real bounded:false marker through admission.
   - unbounded requires the stored marker's `bounded` key to be present, boolean and false.
3. F-2 is folded. X4 alone fails marker-digest at :241 `63-character digest admitted`. X5 alone fails marker-version at :229 `version 0 admitted`. Two supplementary mutants show that the other two added values each carry weight on their own. X4b fails at :241 with `65-character digest admitted`, and X5b fails at :229 with `version -1 admitted`.
4. Unmutated, `go test -count=1 -timeout 40m ./internal/dispatch -run '^(TestCompositionAdmittedBounds|TestCompositionAdmittedBoundsAdmission)$' -v` exits 0 with all 19 subtests PASS. `gofmt -l internal/dispatch` prints nothing, and `go vet ./internal/dispatch` exits 0 with no output.

Claims in codex-bdrb-u3a-ii-result-2.md that I checked:
- The "what changed" list is accurate.
- The X2 to X5 failure lines (:117, :205, :205, :241, :229) match my runs exactly, so the F-3 request holds.
- The production hashes before and after match.
- gofmt, vet and the focused run reproduce.
- The numstat total of 336 matches.
- Its race run of ./internal/dispatch was not re-run, because this read runs no race suite.

### F-11. Severity low. Material no.

Claim: no test in the unit shows that composition puts the marker on a referenced task direction, so a composition that drops the marker only for referenced sections passes both functions, and the fold's referenced admission case at :207 would then pass on a legacy seven-field source.

Evidence:
- X8 changed composition.go's placement condition to `if section.slot == "task-direction" && section.source == "caller:brief" && !section.referenced {`. The full run of both functions exited 0 with all 19 subtests PASS.
- Unmutated, the code is right: the probe above shows the referenced unbounded composition carries the marker and is admitted. The fold's :207 case is therefore not vacuous against the code as built.
- The gap predates the fold. The e1be85f4 file had no referenced case in eight-fields, and marker and unbounded compose inline bodies only. The fold brief asked for admission "inline and referenced", which the returned file does.
- This is a partial weakening of R2.marker ("Carry the exact record hash in the selected task-direction source", page line 615, and line 120, "write it for both bounded and headerless briefs"). The full rule is still pinned: removing the stored marker (A1c) and putting the marker on every source (the closing read's X1) both fail. That is the same standard the closing read applied to F-2.
- The page's planned witnesses are in unit 3c: S1.initial-referenced and S1.follow-up-referenced (lines 656 and 658). They read back through the unit 3b reader, which refuses admitted files without a composition marker (line 147), so X8 would fail there. Recorded so unit 3c's brief keeps those two referenced cases.

### F-12. Severity low. Material no.

Claim: the fold's stored-marker check in unbounded uses unchecked type assertions, so a composition that stores no marker panics instead of failing by name, and the panic stops the rest of the run.

Evidence:
- Line 115 is `markerObject(stored["sources"].([]any), 0)["bounded"].(bool)`. markerObject (:188-190) asserts `.(map[string]any)` without the `, ok` form that the marker subtest uses at :85.
- A1c changed composition.go's `json:"admittedBrief,omitempty"` to `json:"-"`. marker fails by name at :88 `stored marker = map[string]interface {}(nil)`. Then unbounded panics with `interface conversion: interface {} is nil, not map[string]interface {}` at :189, called from :115.
- After the panic, job, round, legacy, delivered-source, inline-body, referenced-body and the whole of TestCompositionAdmittedBoundsAdmission never run. Against e1be85f4 the closing read saw the same mutant fail marker and marker-shape by name.
- The mutant is still killed (exit 1, with marker and unbounded both reported FAIL), so no proof is lost and no row is weakened. Only the failure report is shorter.

## Mutation table

Every mutation was one exact string replacement in /tmp/opus-u3aii-r2-mut/metasystem/internal/dispatch, with its occurrence count asserted at 1. The harness is scratchpad/u3aii-r2-harness.py, and the raw outputs are in /tmp/opus-u3aii-r2-out.
- Named run: `go test -count=1 -timeout 40m ./internal/dispatch -run '<pattern>' -v`.
- Full run: the same command with `-run '^(TestCompositionAdmittedBounds|TestCompositionAdmittedBoundsAdmission)$'`.

After each mutant the harness restored composition.go and build.go from bytes it had read and hash-checked at the start. It then asserted the sha256 of composition.go (603e5de3...), build.go (9f9f08de...), the test file (25f98e11...) and references.go (0f09c075...). Every restore matched, and so did the final check.

| Id | Target | One-rule change | Named run, observed | Full run, failing subtests |
| --- | --- | --- | --- | --- |
| X2 | F-1 | composition.go marker tag `json:"bounded"` changed to `json:"bounded,omitempty"` | `^TestCompositionAdmittedBounds$/^unbounded$`: exit 1, FAIL :117 `stored bounded = <nil>`. `^TestCompositionAdmittedBoundsAdmission$/^eight-fields$`: exit 1, FAIL :205 `composition source 0 has invalid admitted brief evidence` | exit 1: unbounded (:117) and eight-fields (:205). The other 17 pass |
| X3 | F-1 | build.go admission `boundedValue, boundedOK := admitted["bounded"].(bool)` plus `boundedOK = boundedOK && boundedValue` | eight-fields: exit 1, FAIL :205 `composition source 0 has invalid admitted brief evidence`. unbounded as a control: exit 0 (composition unchanged) | exit 1: eight-fields (:205) only |
| X4 | F-2 | build.go digest check ``regexp.MustCompile(`^[0-9a-f]+$`)`` in place of incarnationRe | `^TestCompositionAdmittedBoundsAdmission$/^marker-digest$`: exit 1, FAIL :241 `63-character digest admitted` | exit 1: marker-digest (:241) only |
| X5 | F-2 | build.go `version > 1` in place of `version != 1` | `^TestCompositionAdmittedBoundsAdmission$/^marker-version$`: exit 1, FAIL :229 `version 0 admitted` | exit 1: marker-version (:229) only |
| X4b | F-2, supplementary (65-character value) | build.go digest check `^[0-9a-f]{64,}$` | marker-digest: exit 1, FAIL :241 `65-character digest admitted` | exit 1: marker-digest (:241) only |
| X5b | F-2, supplementary (version -1) | build.go `version > 1 \|\| version == 0` in place of `version != 1` | marker-version: exit 1, FAIL :229 `version -1 admitted` | exit 1: marker-version (:229) only |
| X8 | F-11, supplementary | composition.go marker placed only when `!section.referenced` | not run | exit 0, all 19 PASS. SURVIVES |
| A1c | F-12, supplementary | composition.go `json:"admittedBrief,omitempty"` changed to `json:"-"` | not run | exit 1: marker (:88 `stored marker = map[string]interface {}(nil)`), then unbounded panics at :189 via :115. The run stops there |

Live worktree state: at 10:55, at 11:01 and again after this read was written, HEAD is 300ecfc69e7dbd9e27f1e67d8d3df4a308ac0224. `git status --porcelain` lists only the three unit files. Their hashes are unchanged: build.go 9f9f08de4479a995135659a841b2f6481c11dcb11e8b4d44e97042d723094a0a, composition.go 603e5de390803747f98a4c71adede066cf254e64bf7e85549a202de7c920456d, composition_brief_bounds_test.go 25f98e11d88a464cf0fb891de9bbf048b0364eac7c43533bf576b41159bf4b93. `git diff HEAD` hashes to 773f5fb3b0dfbf5e609daa2e1db7c29bb8d8f6405f37bf1a7ef4319e6a6e99e5.
