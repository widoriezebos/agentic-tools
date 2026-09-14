# Code read: brief-declares-the-round-boundary, unit 1a (parse the paired headers)

Reader: Opus, independent of the builder. Subject: the uncommitted four-file diff in `.claude/worktrees/bdrb-u1a`, 400 changed lines. Spec: `plans/brief-declares-the-round-boundary-design.md` revision 3. Nothing in the live worktree was edited. The sha256 of all four files is the same before and after this read.

`<scratch>` below is `/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/36c93128-ec16-4146-8def-3f706ba4ef10/scratchpad`. All mutation and probe work ran on copies under it. Every `go test` ran with METASYSTEM_BIN and METASYSTEM_CONTEXT_COST_PROOF unset and `-timeout 40m`.

Mutation runs used `python3 <scratch>/mutate.py <copy>/metasystem <worker> 3 <scratch>/mut-<worker>.jsonl`. For each run it applies one textual mutation to brief.go or register.go and runs `go test <pkg> -run '<regex>' -count=1 -timeout 40m -v` with GOFLAGS=-trimpath. It then restores the file and checks its sha256. Every run restored cleanly. Logs are in `<scratch>/mut-<n>.jsonl.logs/<row>.log`.

BROAD below means `go test ./internal/dispatch -run '^(TestBriefBounds.*|TestBriefAdmission|TestBriefMode|TestBriefAuthority.*)$' -count=1 -timeout 40m`.

## Findings

Material: 6 (F-1 to F-6). Not material: 4 (F-7 to F-10).

### F-1. Severity medium. Material yes.
Claim: H6.projection-before-pattern does not reproduce at its named location. The unsupported-prefix guard inside ProjectBriefBoundary has no witness.
Evidence: the guard exists twice, at brief.go:146-148 (parseBriefBounds) and brief.go:200-202 (ProjectBriefBoundary).
- Removing brief.go:200-202 alone: `go test ./internal/dispatch -run '^TestBriefBoundsProjectionSyntax$/^projection-before-pattern$' -count=1 -timeout 40m` passes. `/^unsupported-prefix$` passes. BROAD passes.
- Removing brief.go:146-148 alone: projection-before-pattern still passes.
- Only removing both fails it, and then with `invalid path or pattern "meta\\[system/a"`, not the recorded `error = <nil>`.

The direct projection table at brief_bounds_test.go:76-87 has no bad-prefix case. ProjectBriefBoundary is exported, and the page has review re-project persisted members through it (Paths and matching; E6.projection keeps BRIEF_BOUNDS_INVALID for an unsupported target prefix). Today ProjectBriefBoundary("a", "meta*") does refuse (`<scratch>/probe-bounds.txt`). The behaviour is right; the proof is missing.
Change: add a direct ProjectBriefBoundary case with an unsupported prefix. Give projection-before-pattern a witness that fails when its own rule is removed, or correct the row's location.

### F-2. Severity medium. Material yes.
Claim: the H7.parser-ownership witness is a case-sensitive string count. It cannot see a bounds parse in the mode-only helper, or a second parse in admission, when the exported name is used.
Evidence: brief_bounds_test.go:124-128 requires `strings.Count(source, "parseBriefBounds(") == 3`.
- Adding `if _, err := ParseBriefBounds(data, ""); err != nil { return "", err }` before brief.go:93 (BriefModeOnly): `go test ./internal/dispatch -run '^TestBriefAdmission$/^parser-ownership$' -count=1 -timeout 40m` passes.
- Adding a `ParseBriefBounds(data, installPrefix)` call before brief.go:111 (a second parse in admitBriefBytes): passes.

The page row asks for a call-graph assertion that permits one parse in the admission path and none in the mode-only helper. A comment containing the literal would also break the count.
Change: assert on calls to either spelling inside BriefModeOnly and admitBriefBytes, for example with go/ast.

### F-3. Severity medium. Material yes.
Claim: the H7.read-failure witness misses the natural removal of its rule. With the read error ignored, ValidateBriefAuthority admits a missing brief and a directory, and no test in the package fails.
Evidence: brief_bounds_test.go:114-117 calls ReadBriefAdmission with requireMode true and no authority. There, the mode refusal at brief.go:108-110 masks a missing read check.
- Replacing brief.go:96-102 with `data, _ := os.ReadFile(briefPath)`: the named test passes, and BROAD passes.
- Under the same mutation, `PROBE_CORPUS=<scratch>/corpus PROBE_OUT=<scratch>/probe-mut-readignore.txt go test ./internal/dispatch -run '^TestZZProbeDifferential$' -count=1 -timeout 40m` printed `zz-missing-file ... authority=nil` and `zz-directory ... authority=nil`.
- HEAD and the unmutated candidate print `brief authority admission cannot read brief: open <CORPUS>/zz-missing-file: no such file or directory`.
- Replacing only the authority-branch return at brief.go:101 with `return BriefAdmission{}, nil` also passes BROAD.

The shell uses this authority-only path through brief_authority at scripts/agents/dispatch.sh:862-864.
Change: add a read-failure case on the authority branch, with a callback present and requireMode false.

### F-4. Severity medium. Material yes.
Claim: the structured value validator's pair rule and member rule have no failing test.
Evidence:
- Removing the pair check at brief.go:175-177 passes BROAD.
- Removing the member loop at brief.go:178-182 passes BROAD.

Only the negative Ceiling check at brief.go:183-185 is witnessed (brief_bounds_test.go:64). The page makes this the validator that unit 3a reuses for record field presence and member components. The R1 rows target 3a's codec file, so no owner would ever run a remove-one on these brief.go lines. The brief's proof rule covers every rule added. Behaviour today is correct: boundary-only, ceiling-only, "/a", "a/../b" and a NUL member all refuse (`<scratch>/probe-bounds.txt`).
Change: add ValidateBriefBounds cases for boundary-only, ceiling-only and one invalid member.

### F-5. Severity low. Material yes.
Claim: BriefModeOnly has no behavioural witness.
Evidence: brief.go:88-94.
- Replacing its return with `_ = data; return "implement", nil` passes BROAD.
- Returning nil on its read failure (brief.go:90-92) passes BROAD.

Its only coverage is the string count in F-2. It has no production caller until 1b.
Change: add one case showing a partial-pair brief still yields its mode, and one showing an unreadable file refuses.

### F-6. Severity low. Material yes.
Claim: 1a already routes BriefMode and ValidateBriefAuthority through the bounds parser, which changes the public `job brief-mode` outcome. Nothing in 1a fails if either wrapper stops delegating.
Evidence: brief.go:44-47 and brief.go:250-255.
- Reverting BriefMode to `return BriefModeOnly(briefPath)` passes BROAD.
- Reverting ValidateBriefAuthority to its own read plus `validateBriefAuthority(data, BriefBounds{}, baseTree, diskRoot)` passes BROAD.
- Built binaries from HEAD and candidate (`bash <scratch>/cli-run.sh <scratch>`; diff in `<scratch>/cli-diff.txt`): 21 of the 55 corpus briefs change, and all of them carry a live partial or malformed pair. For example, b21-boundary-only under `job brief-mode --brief` goes from `exit=0 stdout=implement` to `exit=1 stderr=BRIEF_BOUNDS_INVALID: Ceiling: required with Boundary`. `--authority-only` changes the same way.

The page places these witnesses in 1b (H8.normal, H8.authority-only). Until 1b adds `--mode-only`, early mode discovery parses bounds: brief_mode at scripts/agents/dispatch.sh:859, called at :1542. A partial pair would then print the BRIEF_BOUNDS_INVALID line and die with the Working Mode suffix. The CLI at cmd/metasystem/dispatch_verbs.go:2015 and :2022 also still reads and parses the brief twice in one invocation.
Change: either add Go-level witnesses for both wrappers in 1a, or leave the wrappers on their old paths until 1b. The seat chooses. If the seat reads the page as putting wrapper wiring in 1b, this can close as out-of-scope for 1a, with 1b owning it.

### F-7. Severity low. Material no.
Claim: the result file's observed line for H4.duplicate cannot come from its named removal.
Evidence: removing brief.go:133-135 and running `go test ./internal/dispatch -run '^TestBriefBoundsCeilingSyntax$/^duplicate$' -count=1 -timeout 40m` on input `Boundary: []\nCeiling: 1\nCeiling: 2` (brief_bounds_test.go:55) gives `error = &dispatch.BriefBoundsRefusal{Header:"Ceiling", Detail:"required with Boundary"}`. The result file records `{Header:"Boundary", Detail:"required with Ceiling"}`, which is the H3.duplicate-boundary line. The row still reproduces. Do not copy that line into a receipt.

### F-8. Severity low. Material no.
Claim: four guards are redundant and cannot be witnessed by pure removal.
Evidence:
- `member == ""` and `path.IsAbs(member)` at brief.go:223 are covered by the component loop at brief.go:227-231.
- `ceilingText == ""` at brief.go:164 is covered by ParseInt at brief.go:167-170.
- The Boundary value trim at brief.go:75 is masked by JSON's tolerance for ASCII whitespace.

Deleting each alone passes its named test (H5.absolute, H5.empty, H4.empty, H1.trim probes in the JSONL). Behaviour stays guarded, and the named rows reproduce under a behavioural mutation.

### F-9. Severity low. Material no.
Claim: an invalid UTF-8 byte or a lone surrogate escape in a Boundary member is admitted as U+FFFD instead of refused.
Evidence: corpus b32 (`Boundary: ["a<0xff>b"]`) and b44 (`Boundary: ["a\ud800b"]`) both give `OK boundaryNil=false boundary=["a\ufffdb"] ceiling=1` in `<scratch>/probe-bounds.txt`. encoding/json at brief.go:143 replaces the bytes instead of rejecting them. The page says to preserve all other bytes. This fails closed at review: U+FFFD is not a pattern byte and cannot authorise the original path. Worth a note for 3a and 4a.

### F-10. Severity low. Material no.
Claim: the invalid-member detail HTML-escapes <, > and &.
Evidence: corpus b40 (`["/a<b&c"]`) renders `BRIEF_BOUNDS_INVALID: Boundary: invalid path or pattern "/a\u003cb\u0026c"` from json.Marshal at brief.go:238. That is a valid JSON string, which is all the page requires. Later exact-diagnostic assertions must use the same encoder.

## Checks with no finding

- Boundary and Ceiling: `git status --short --untracked-files=all` lists exactly the four Boundary files. `git diff --numstat` gives 202+11 and 1+0, and the new files are 168 and 18 lines, for a total of 400, equal to Ceiling. There is no CLI, shell, schema, persistence, review or authority-lexer change; extractBriefAuthorityPaths is untouched. Any witness added for F-1 to F-6 needs compaction or a reported overrun under the brief.
- Page coverage: all 56 rows the page assigns to 1a appear in the builder's table. Each has its named subtest, and each subtest ran.
- Register row: register.go:81 has Owner internal/dispatch, Site brief.go:56, Shape question and no override. brief.go:56 is the Sprintf that emits the code. TestHCL03EveryCodeRowed also fails when the row is removed.
- Header recognition (corpus in `<scratch>/corpus`, results in `<scratch>/probe-bounds.txt`):
  - Not live: lowercase, indented, `>` quoted, BOM-prefixed, and a space or tab before the colon.
  - Live: column-zero lines inside a fence, including an indented fence; `Boundary:["a"]` with no space after the colon.
  - Trimmed: CRLF and trailing NBSP.
  - A live header whose partner is not live refuses, naming the missing header.
- Values:
  - Boundary: null, object, non-string member, trailing JSON and an empty value refuse. `[]` is non-nil deny-all. Duplicate members are kept.
  - Ceiling: `+1`, `-1`, `-0`, `1.5`, `400 lines`, `<n>`, empty, an Arabic-Indic digit and 9223372036854775808 refuse. 0, 0002 and 0009223372036854775807 admit. Duplicate identical headers refuse.
- Precedence and text: duplicates (Boundary, then Ceiling), then pairing, then Boundary, then Ceiling. Every refusal renders `BRIEF_BOUNDS_INVALID: <Header>: <detail>` with the page's detail strings.
- Headerless compatibility: I ran `go test ./internal/dispatch -run '^TestZZProbe' -count=1 -timeout 40m` on HEAD and candidate copies, then `diff probe-base.txt probe-cand.txt`. All 19 headerless briefs, plus a missing file and a directory, give identical BriefMode and ValidateBriefAuthority results. The CLI rows for them are also identical in exit code, stdout and stderr.
- Existing tests: `go test -race -count=1 -timeout 40m ./internal/dispatch/ ./internal/refusal/` passed on the candidate copy (88.8 s, 2.7 s) and on the HEAD copy (89.9 s, 2.3 s). No cmd/metasystem Go test exercises brief-mode: grep for brief-mode, runDispatchBriefMode, BriefMode and Working Mode in cmd/metasystem/*_test.go found none. So I checked the CLI with built binaries instead.
- Static checks: `gofmt -l` is clean on the four files. `go vet ./internal/dispatch/ ./internal/refusal/` exits 0. `GOPROXY=file://$(go env GOMODCACHE)/cache/download GOSUMDB=off go run honnef.co/go/tools/cmd/staticcheck@v0.8.0 ./internal/dispatch/ ./internal/refusal/` exits 0 with no output. The fast gate script itself was not run.

## Reproduced mutation table

Primary mutation per row, applied on a copy, restored and hash-checked. 55 rows reproduce. 1 row does not (H6.projection-before-pattern).

| Row | Reproduced | Observed line | Note |
| --- | --- | --- | --- |
| H1.case | yes | `brief_bounds_test.go:135: error = <nil>, want Boundary: required with Ceiling` |  |
| H1.column | yes | `brief_bounds_test.go:135: error = <nil>, want Boundary: required with Ceiling` |  |
| H1.physical-line | yes | `brief_bounds_test.go:135: error = <nil>, want Boundary: expected a JSON array of paths` |  |
| H1.trim | yes | `brief_bounds_test.go:138: parse bounds: BRIEF_BOUNDS_INVALID: Ceiling: expected a nonnegative decimal integer no greater than 9223372036854775807` | Removing only the Boundary value trim passes; JSON tolerates the whitespace (F-8). |
| H1.fence | yes | `brief_bounds_test.go:143: bounds = {Boundary:[] Ceiling:<nil>}` |  |
| H2.neither | yes | `brief_bounds_test.go:138: parse bounds: BRIEF_BOUNDS_INVALID: Boundary: required with Ceiling` |  |
| H2.both | yes | `brief_bounds_test.go:138: parse bounds: BRIEF_BOUNDS_INVALID: Ceiling: required with Boundary` |  |
| H2.empty-array | yes | `brief_bounds_test.go:138: parse bounds: BRIEF_BOUNDS_INVALID: Boundary: expected a JSON array of paths` |  |
| H2.duplicates | yes | `brief_bounds_test.go:138: parse bounds: BRIEF_BOUNDS_INVALID: Boundary: invalid path or pattern "a"` |  |
| H2.boundary-only | yes | `brief_bounds_test.go:135: error = <nil>, want Ceiling: required with Boundary` |  |
| H2.ceiling-only | yes | `brief_bounds_test.go:135: error = <nil>, want Boundary: required with Ceiling` |  |
| H3.empty | yes | `brief_bounds_test.go:135: error = <nil>, want Boundary: expected a JSON array of paths` |  |
| H3.null | yes | `brief_bounds_test.go:135: error = <nil>, want Boundary: expected a JSON array of paths` |  |
| H3.object | yes | `brief_bounds_test.go:135: error = <nil>, want Boundary: expected a JSON array of paths` |  |
| H3.nonstring | yes | `brief_bounds_test.go:135: error = <nil>, want Boundary: expected a JSON array of paths` |  |
| H3.trailing-json | yes | `brief_bounds_test.go:135: error = <nil>, want Boundary: expected a JSON array of paths` |  |
| H3.duplicate-boundary | yes | `brief_bounds_test.go:135: error = &dispatch.BriefBoundsRefusal{Header:"Boundary", Detail:"required with Ceiling"}, want Boundary: header occurs more than once` |  |
| H4.plus | yes | `brief_bounds_test.go:135: error = <nil>, want Ceiling: expected a nonnegative decimal integer no greater than 9223372036854775807` |  |
| H4.minus | yes | `brief_bounds_test.go:62: error = <nil>, want Ceiling: expected a nonnegative decimal integer no greater than 9223372036854775807` | Validator site alone also fails: `brief_bounds_test.go:64: error = <nil>, want Ceiling: expected a nonnegative decimal integer no greater than 9223372036854775807`. |
| H4.fraction | yes | `brief_bounds_test.go:135: error = <nil>, want Ceiling: expected a nonnegative decimal integer no greater than 9223372036854775807` |  |
| H4.suffix | yes | `brief_bounds_test.go:135: error = <nil>, want Ceiling: expected a nonnegative decimal integer no greater than 9223372036854775807` |  |
| H4.placeholder | yes | `brief_bounds_test.go:135: error = <nil>, want Ceiling: expected a nonnegative decimal integer no greater than 9223372036854775807` |  |
| H4.empty | yes | `brief_bounds_test.go:135: error = <nil>, want Ceiling: expected a nonnegative decimal integer no greater than 9223372036854775807` | Behavioural mutation. Deleting only `ceilingText == ""` passes (F-8). |
| H4.non-ascii | yes | `brief_bounds_test.go:135: error = <nil>, want Ceiling: expected a nonnegative decimal integer no greater than 9223372036854775807` |  |
| H4.overflow | yes | `brief_bounds_test.go:135: error = <nil>, want Ceiling: expected a nonnegative decimal integer no greater than 9223372036854775807` |  |
| H4.duplicate | yes | `brief_bounds_test.go:135: error = &dispatch.BriefBoundsRefusal{Header:"Ceiling", Detail:"required with Boundary"}, want Ceiling: header occurs more than once` | Observed line differs from the result file, which records Header "Boundary", Detail "required with Ceiling" (F-7). |
| H4.zero | yes | `brief_bounds_test.go:138: parse bounds: BRIEF_BOUNDS_INVALID: Ceiling: expected a nonnegative decimal integer no greater than 9223372036854775807` |  |
| H4.leading-zeroes | yes | `brief_bounds_test.go:138: parse bounds: BRIEF_BOUNDS_INVALID: Ceiling: expected a nonnegative decimal integer no greater than 9223372036854775807` |  |
| H4.maximum | yes | `brief_bounds_test.go:138: parse bounds: BRIEF_BOUNDS_INVALID: Ceiling: expected a nonnegative decimal integer no greater than 9223372036854775807` |  |
| H5.absolute | yes | `brief_bounds_test.go:135: error = <nil>, want Boundary: invalid path or pattern "/a"` | Behavioural mutation. Deleting only `path.IsAbs(member)` passes (F-8). |
| H5.empty | yes | `brief_bounds_test.go:135: error = <nil>, want Boundary: invalid path or pattern ""` | Behavioural mutation. Deleting only `member == ""` passes (F-8). |
| H5.nul | yes | `brief_bounds_test.go:135: error = <nil>, want Boundary: invalid path or pattern "a\u0000b"` |  |
| H5.empty-component | yes | `brief_bounds_test.go:135: error = <nil>, want Boundary: invalid path or pattern "a//b"` |  |
| H5.dot | yes | `brief_bounds_test.go:135: error = <nil>, want Boundary: invalid path or pattern "a/./b"` |  |
| H5.parent | yes | `brief_bounds_test.go:135: error = <nil>, want Boundary: invalid path or pattern "a/../b"` |  |
| H5.member-whitespace | yes | `brief_bounds_test.go:138: parse bounds: BRIEF_BOUNDS_INVALID: Boundary: invalid path or pattern " a \t"` | Trimming variant also fails: `brief_bounds_test.go:143: bounds = {Boundary:[a] Ceiling:0x339939bb01e8}` (pointer value varies). |
| H6.root | yes | `brief_bounds_test.go:85: projection = "", false, BRIEF_BOUNDS_INVALID: Boundary: invalid path or pattern "a"` |  |
| H6.nested | yes | `brief_bounds_test.go:85: projection = "metasystem/a", false, <nil>` |  |
| H6.missing-prefix | yes | `brief_bounds_test.go:135: error = <nil>, want Boundary: invalid path or pattern "other/a"` |  |
| H6.double-prefix | yes | `brief_bounds_test.go:85: projection = "a", false, <nil>` |  |
| H6.whole-project | yes | `brief_bounds_test.go:85: projection = "", false, <nil>` |  |
| H6.unsupported-prefix | yes | `brief_bounds_test.go:135: error = <nil>, want Boundary: installation prefix contains unsupported pattern bytes` | Parse-site guard only. Removing the ProjectBriefBoundary copy (brief.go:200-202) alone passes (F-1). |
| H6.projection-before-pattern | no | `test passed with the named rule removed` | Named site ProjectBriefBoundary alone: test passed. Parse site alone: test passed. Both sites removed: `brief_bounds_test.go:135: error = &dispatch.BriefBoundsRefusal{Header:"Boundary", Detail:"invalid path or pattern \"meta\\\\[system/a\""}, want Boundary: installation prefix contains unsupported pattern bytes` (F-1). |
| H6.literal-directory | yes | `brief_bounds_test.go:138: parse bounds: BRIEF_BOUNDS_INVALID: Boundary: invalid path or pattern "metasystem/a[/"` |  |
| H6.malformed-pattern | yes | `brief_bounds_test.go:135: error = <nil>, want Boundary: invalid path or pattern "metasystem/a["` |  |
| H6.exact | yes | `brief_bounds_test.go:95: exact member classified as pattern` |  |
| H6.backslash-pattern | yes | `brief_bounds_test.go:97: backslash member classified as concrete` |  |
| H7.duplicates-first | yes | `brief_bounds_test.go:135: error = <nil>, want Ceiling: header occurs more than once` | Reorder variant (pairing before duplicates) also fails. |
| H7.boundary-duplicate-first | yes | `brief_bounds_test.go:135: error = &dispatch.BriefBoundsRefusal{Header:"Ceiling", Detail:"header occurs more than once"}, want Boundary: header occurs more than once` |  |
| H7.pair-before-value | yes | `brief_bounds_test.go:135: error = &dispatch.BriefBoundsRefusal{Header:"Boundary", Detail:"expected a JSON array of paths"}, want Ceiling: required with Boundary` |  |
| H7.boundary-before-ceiling | yes | `brief_bounds_test.go:135: error = &dispatch.BriefBoundsRefusal{Header:"Ceiling", Detail:"expected a nonnegative decimal integer no greater than 9223372036854775807"}, want Boundary: expected a JSON array of paths` |  |
| H7.mode-first-without-authority | yes | `brief_bounds_test.go:110: error = BRIEF_BOUNDS_INVALID: Boundary: expected a JSON array of paths` |  |
| H7.read-failure | yes | `brief_bounds_test.go:116: missing brief admitted` | Ignoring the read error (brief.go:96-102 to `data, _ := os.ReadFile`) passes this test and every brief test (F-3). |
| H7.exact-bytes | yes | `brief_bounds_test.go:122: admission bytes = "Working Mode: implement\nBoundary: []\nCeiling: 0\n", checked = "Working Mode: implement\nBoundary: []\nCeiling: 0", error = <nil>` | Authority-bytes variant also fails. |
| H7.parser-ownership | yes | `brief_bounds_test.go:127: bounds parser must have one public call, one admission call, and one declaration` | An exported `ParseBriefBounds` call in BriefModeOnly, or a second parse in admitBriefBytes, passes (F-2). |
| D1.invalid | yes | `brief_bounds_register_test.go:15: row = {Code: Owner: Site: Shape: Override: Commands:0 Pending:}, want {Code:BRIEF_BOUNDS_INVALID Owner:internal/dispatch Site:brief.go:56 Shape:question Override: Commands:0 Pending:}` | Site changed to brief.go:57 also fails; TestHCL03EveryCodeRowed also fails on row removal. |

## Verdict

Send back (6 material findings), in this order: 1. F-1: witness the ProjectBriefBoundary prefix guard directly and fix H6.projection-before-pattern. 2. F-3: add an authority-branch read-failure case. 3. F-2: make parser-ownership catch the exported spelling. 4. F-4: add ValidateBriefBounds pair and member cases. 5. F-5: add BriefModeOnly behaviour cases. 6. F-6: witness both wrappers in 1a or defer their wiring to 1b. Keep the diff at or under 400, or report the exact overrun.
