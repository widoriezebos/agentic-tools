# Code read 2: brief-declares-the-round-boundary, unit 1a, fold round 3

Reader: Opus, independent of the builder, same method as read 1. Subject: the uncommitted four-file diff in `.claude/worktrees/bdrb-u1a`, 399 changed lines. Spec: `plans/brief-declares-the-round-boundary-design.md` revision 3. The live worktree was not edited. After this read, the four live files still hash to the builder's values: brief.go `ba4a7cd0...`, brief_bounds_test.go `b1591d41...`, register.go `ac03ff23...`, brief_bounds_register_test.go `5f99dfca...`.

`<scratch>` means `/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/36c93128-ec16-4146-8def-3f706ba4ef10/scratchpad`. All mutations ran on `<scratch>/u1a-r3-copy/metasystem` and two sibling copies inside it (`w1`, `w2`). The runner is `python3 <scratch>/mutate-r3.py <copy> <worker> 3 <scratch>/mut-r3-<worker>.jsonl`. For each mutation it checks the file's sha256, makes one textual change to brief.go or register.go, and runs `go test <pkg> -run '<regex>' -count=1 -timeout 40m -v` with GOFLAGS=-trimpath and METASYSTEM_BIN and METASYSTEM_CONTEXT_COST_PROOF unset. Then it restores the file and checks the hash again. All 57 runs restored cleanly, and all 57 matched their expected outcome. Logs are in `<scratch>/mut-r3-<n>.jsonl.logs/`.

BROAD below means `go test ./internal/dispatch -run '^(TestBriefBounds.*|TestBriefAdmission|TestBriefMode.*|TestBriefAuthority.*|TestValidateBriefBounds)$' -count=1 -timeout 40m`. It runs 79 tests and subtests. No code outside brief.go and brief_bounds_test.go calls admitBriefBytes, ReadBriefAdmission, briefModeFromHeaders, BriefModeOnly, ValidateBriefBounds, ParseBriefBounds or ProjectBriefBoundary. I checked with grep over internal and cmd, so BROAD reaches every test that can reach them.

## Findings

Material: 2 (R3-1, R3-2). Not material: 2 (R3-3, R3-4).

### R3-1. Severity low. Material yes.
Claim: the F-6 choice put BriefMode back on its HEAD body. That removed the only witness for 1a's mode extraction. The three guards in briefModeFromHeaders, and the Mode value it returns, now have no test that fails when they are removed. The fold weakened a witness that round 2 had.
Evidence:
- brief.go:95 is the guard `len(headers.mode) != 1 || headers.mode[0] == "" || strings.HasPrefix(headers.mode[0], "<")`. brief.go:98 returns the value. Its only caller is admitBriefBytes at brief.go:115.
- Each of these mutations passes BROAD (exit 0, 79 ran):
  - `len(headers.mode) != 1` changed to `== 0`.
  - `headers.mode[0] == ""` dropped.
  - The `<` placeholder check dropped.
  - `return headers.mode[0], nil` changed to `return "implement", nil`.
- On the round-2 copy (`<scratch>/u1a-copy`), BriefMode still delegated to admission. There, the duplicate and placeholder removals fail `go test ./internal/dispatch -run '^TestBriefMode$' -count=1 -timeout 40m`:
  - `decisions_test.go:359: double.md: expected a refusal`
  - `decisions_test.go:359: placeholder.md: expected a refusal`
- Only one mode assertion is left: H7.mode-first-without-authority at brief_bounds_test.go:109, and its brief has no Working Mode line. H7.exact-bytes at brief_bounds_test.go:127 does not check `admission.Mode`.
- The 1a brief lists "mode extraction" as a deliverable. It also binds "for every rule you add, a test that fails when that rule alone is removed". The page has no H row for these guards. It expected TestBriefMode to reach them through the wrapper, and that wiring is now 1b's.
- Removing the combined check at brief.go:128 (`if requireMode && modeErr != nil`) also passes BROAD. I do not count that here: it is H8.authority-before-mode, a 1b row.
Change: either witness the three guards and the returned Mode in 1a, or record that 1b owns them. 1a could add a few no-authority admitBriefBytes cases and a Mode check in exact-bytes. 1b's wrapper wiring puts TestBriefMode back on this path. The seat chooses. The diff has one line of Ceiling headroom, so report any overrun rather than cutting.

### R3-2. Severity low. Material yes.
Claim: the AST ownership witness reads only two function bodies. A second bounds parse in ReadBriefAdmission passes it and passes every brief test. ReadBriefAdmission is the file entry to the admission path. The page row asks for one parse in the admission path, and the unit block says "Parse once within each admission invocation".
Evidence:
- brief_bounds_test.go:132 checks `boundsParserCalls(file, "admitBriefBytes") == 1 && boundsParserCalls(file, "BriefModeOnly") == 0`. boundsParserCalls (brief_bounds_test.go:135-154) counts direct identifier calls inside one body and does not follow callees.
- I inserted `if _, err := ParseBriefBounds(data, installPrefix); err != nil { return BriefAdmission{}, err }` before brief.go:111. Then `go test ./internal/dispatch -run '^TestBriefAdmission$/^parser-ownership$' -count=1 -timeout 40m` passes, and BROAD passes.
- This gap is narrower than read 1's F-2. F-2's suggested change named only BriefModeOnly and admitBriefBytes, and the builder did exactly that. The gap comes from that narrow suggestion, not from the fold.
Change: also require zero calls in ReadBriefAdmission. Or count every function in brief.go except ParseBriefBounds and admitBriefBytes. Either fits on line 132 with no added line.

### R3-3. Severity low. Material no.
Claim: the ownership check does not follow calls. BriefModeOnly now calls BriefMode (brief.go:101), so a bounds parse reached through that call is caught only if it changes an outcome.
Evidence:
- I replaced brief.go:101 with `admission, err := ReadBriefAdmission(briefPath, "", true, nil); return admission.Mode, err`.
  - It passes parser-ownership.
  - It fails `TestBriefModeOnly/partial-pair` at `brief_bounds_test.go:102: mode = "", error = BRIEF_BOUNDS_INVALID: Ceiling: required with Boundary`.
- An ignored `_, _ = ParseBriefBounds(data, "")` added to BriefMode passes BROAD. Nothing observable changes, and H8.mode-only is a 1b row.
- Note for 1b: once BriefMode delegates to admission, BriefModeOnly must stop calling BriefMode. The page also says file wrappers must not call one another.

### R3-4. Severity low. Material no.
Claim: the existing BRIEF_AUTHORITY_REFUSED register row still says brief.go:34. The three new imports moved its Sprintf to brief.go:37.
Evidence: register.go:80 and brief.go:37. No test reads Site. At HEAD, 28 of 54 rows already do not point at their exact emitting line (a script over `git show HEAD` files). Round 2 had the same offset.

## F-1 to F-6 closure

| Id | Read-1 mutation (adapted where the code moved) | Witness | Observed | Status |
| --- | --- | --- | --- | --- |
| F-1 | Remove the ProjectBriefBoundary prefix guard (brief.go:204-206) | `TestBriefBoundsProjectionSyntax/projection-before-pattern` | `brief_bounds_test.go:67: error = &dispatch.BriefBoundsRefusal{Header:"Boundary", Detail:"invalid path or pattern \"meta\\\\[system/a\""}, want Boundary: installation prefix contains unsupported pattern bytes` | closed |
| F-1 | Same removal | `/unsupported-prefix-direct` | `brief_bounds_test.go:67: error = <nil>, want Boundary: installation prefix contains unsupported pattern bytes`. BROAD also fails. | closed |
| F-1 | Remove the parse-site guard (brief.go:153-155) | `/unsupported-prefix` | `brief_bounds_test.go:160: error = <nil>, want Boundary: installation prefix contains unsupported pattern bytes` | closed |
| F-1 | Move the ProjectBriefBoundary guard after the literal-prefix check | `/projection-before-pattern` | Fails at :67 with the invalid-member line above. `/unsupported-prefix-direct` passes under this mutation, so the two cases witness different rules. | closed |
| F-2 | Exported `ParseBriefBounds(data, "")` in BriefModeOnly | `TestBriefAdmission/parser-ownership` | `brief_bounds_test.go:132: bounds parser ownership changed` | closed |
| F-2 | Exported second parse in admitBriefBytes before brief.go:119 | same | `brief_bounds_test.go:132: bounds parser ownership changed`. The lowercase variant also fails. The unmutated tree passes, and so does a comment naming both spellings. The gap left is R3-2. | closed as specified |
| F-3 | `data, _ := os.ReadFile(briefPath)` in ReadBriefAdmission | `TestBriefAdmission/authority-read-failure` | `brief_bounds_test.go:121: missing authority brief admitted`. BROAD also fails. | closed |
| F-3 | Authority-branch read failure returns `BriefAdmission{}, nil` | same | `brief_bounds_test.go:121: missing authority brief admitted` | closed |
| F-4 | Remove the pair check in ValidateBriefBounds | `TestValidateBriefBounds/boundary-only`, `/ceiling-only` | `brief_bounds_test.go:89: error = <nil>, want Ceiling: required with Boundary`. The ceiling-only case gives `... want Boundary: required with Ceiling`. | closed |
| F-4 | Remove the member loop in ValidateBriefBounds | `/invalid-member` | `brief_bounds_test.go:89: error = <nil>, want Boundary: invalid path or pattern "/a"` | closed |
| F-5 | BriefModeOnly returns `"implement", nil` (its body is now brief.go:101) | `TestBriefModeOnly/read-failure` | `brief_bounds_test.go:102: mode = "implement", error = <nil>`. partial-pair passes under this mutation, as expected. | closed |
| F-5 | The read-failure branch returns nil. This read now lives in BriefMode, which BriefModeOnly calls. | `/read-failure` | `brief_bounds_test.go:102: mode = "", error = <nil>` | closed |
| F-5 | A bounds parse in BriefModeOnly | `/partial-pair` | `brief_bounds_test.go:102: mode = "", error = BRIEF_BOUNDS_INVALID: Ceiling: required with Boundary` | closed |
| F-6 | Wrappers left on their pre-1a paths | Text compare, CLI corpus, page rows | BriefMode and ValidateBriefAuthority are byte-identical to `git show HEAD:metasystem/internal/dispatch/brief.go`, and the diff has 0 deletions. I built `<scratch>/ms-r3` from the copy and compared it with `<scratch>/ms-base` (HEAD). The corpus was 56 briefs, run through normal, `--authority-only` and combined `job brief-mode`: 168 invocations with identical exit code, stdout and stderr (`<scratch>/cli-r3run-diff.txt`: `diff exit 0`). Example: b21-boundary-only normal gives `exit=0 stdout=implement`, where round 2 gave exit 1. H8.normal and H8.authority-only are 1b rows (page lines 536-537). One 1a deliverable lost its witness through this choice: R3-1. | closed, see R3-1 |

## Reproduced rows

Rows the round-3 table added or changed. All reproduce, and each observed line matches the result file apart from markdown escaping.

| Row | Mutation | Observed line |
| --- | --- | --- |
| H6.unsupported-prefix | Parse-site guard removed; owner guard removed | `:160: error = <nil>, want Boundary: installation prefix contains unsupported pattern bytes`; `:67:` same assertion |
| H6.projection-before-pattern | Owner guard removed | `:67: error = &dispatch.BriefBoundsRefusal{Header:"Boundary", Detail:"invalid path or pattern \"meta\\\\[system/a\""}, want Boundary: installation prefix contains unsupported pattern bytes` |
| H7.read-failure | Ordinary branch admits; authority read ignored; authority branch succeeds | `:117: missing brief admitted`; `:121: missing authority brief admitted` (both) |
| H7.parser-ownership | Lowercase in BriefModeOnly; exported in BriefModeOnly; exported second parse in admitBriefBytes | `:132: bounds parser ownership changed` (all three) |
| Structured pair, boundary-only and ceiling-only | Pair check removed | `:89: error = <nil>, want Ceiling: required with Boundary`; `:89: error = <nil>, want Boundary: required with Ceiling` |
| Structured member validation | Member loop removed | `:89: error = <nil>, want Boundary: invalid path or pattern "/a"` |
| Mode-only preserves partial pair | Returns `"review", nil` | `:102: mode = "review", error = <nil>` |
| Mode-only refuses unreadable file | Returns `"implement", nil` | `:102: mode = "implement", error = <nil>` |
| H4.duplicate | Duplicate-Ceiling check removed | `:160: error = &dispatch.BriefBoundsRefusal{Header:"Ceiling", Detail:"required with Boundary"}, want Ceiling: header occurs more than once`. The result file now records this line correctly (read 1's F-7). |
| H4.minus | Parser allows `-`; validator bound changed to `< -1` | `:42: error = <nil>, want Ceiling: expected a nonnegative decimal integer no greater than 9223372036854775807`; `:44:` same assertion |
| D1.invalid | Row removed; Site changed to brief.go:69 | `brief_bounds_register_test.go:15: row = {Code: Owner: Site: ...}, want {Code:BRIEF_BOUNDS_INVALID Owner:internal/dispatch Site:brief.go:68 Shape:question ...}`; the Site change fails the same line. Row removal also fails `TestHCL03EveryCodeRowed` at `register_test.go:40`. |

Sampled unchanged rows, highest severity first. I used read 1's mutations. All 15 reproduce, and each line matches the result file.

| Row | Observed line |
| --- | --- |
| H2.boundary-only | `:160: error = <nil>, want Ceiling: required with Boundary` |
| H2.ceiling-only | `:160: error = <nil>, want Boundary: required with Ceiling` |
| H3.null | `:160: error = <nil>, want Boundary: expected a JSON array of paths` |
| H3.trailing-json | `:160: error = <nil>, want Boundary: expected a JSON array of paths` |
| H5.parent | `:160: error = <nil>, want Boundary: invalid path or pattern "a/../b"` |
| H5.absolute | `:160: error = <nil>, want Boundary: invalid path or pattern "/a"` |
| H5.nul | `:160: error = <nil>, want Boundary: invalid path or pattern` followed by the NUL member as JSON (a, the escaped NUL, b); identical to the result file's line |
| H6.missing-prefix | `:160: error = <nil>, want Boundary: invalid path or pattern "other/a"` |
| H6.malformed-pattern | `:160: error = <nil>, want Boundary: invalid path or pattern "metasystem/a["` |
| H4.overflow | `:160: error = <nil>, want Ceiling: expected a nonnegative decimal integer no greater than 9223372036854775807` |
| H7.exact-bytes | Returned bytes: `:127: admission bytes = "...Ceiling: 0\n", checked = "...Ceiling: 0", error = <nil>`. Authority bytes: the same line with the newline on `checked`. |
| H7.mode-first-without-authority | `:111: error = BRIEF_BOUNDS_INVALID: Boundary: expected a JSON array of paths` |
| H7.duplicates-first | `:160: error = <nil>, want Ceiling: header occurs more than once` |
| H2.empty-array | `:163: parse bounds: BRIEF_BOUNDS_INVALID: Boundary: expected a JSON array of paths` |
| H1.column | `:160: error = <nil>, want Boundary: required with Ceiling` |

## Adversarial checks with no further finding

- Scope and size:
  - `git status --short` lists exactly the four Boundary files.
  - numstat gives 187+0 and 1+0, and the new files have 193 and 18 lines. The total is 399.
  - No CLI, shell, schema, review or authority-lexer change. Nothing reaches into 1b.
- Round 2 to round 3 source changes:
  - The wrappers were restored and BriefModeOnly now delegates to BriefMode.
  - Blank lines were removed, and the Site moved from brief.go:56 to brief.go:68.
  - No new production rule was added.
- Round 2 to round 3 test changes:
  - The case tables moved into package-level variables with the same case names.
  - The parse-level projection-before-pattern case became a direct ProjectBriefBoundary case. No remove-one witness was lost by this. Removing only the parse-site guard fails `unsupported-prefix` at :160. Removing only the owner guard fails both direct cases at :67.
  - The parse-site guard is witnessed with `Boundary: []`. That is the one input that never calls ProjectBriefBoundary, so it is the right witness.
- The only weakened witness is R3-1, and it sits outside the new test file.
- AST test: it fails on the three mutations it claims and passes on the real tree. It does not cover ReadBriefAdmission (R3-2) or callees (R3-3).
- F-8 to F-10 were not acted on, as the fold brief directed.

## Verification on the copy

- `go test -race -count=1 -timeout 40m ./internal/dispatch/ ./internal/refusal/`: `ok internal/dispatch 98.827s`, `ok internal/refusal 2.290s`.
- `gofmt -l` on the four files and on both package directories prints nothing.
- `go vet ./internal/dispatch/ ./internal/refusal/` exits 0.
- Not run:
  - staticcheck and the fast gate.
  - `metasystem validate conformance`: I had no job id, so I computed the diff with git.
  - Any fixture bed.

## Verdict

Send back (2 material, both low proof gaps; the behaviour is correct and in scope): 1. R3-1: in 1a, witness briefModeFromHeaders's three guards and the returned Mode, or record 1b as their owner. 2. R3-2: make parser-ownership also refuse a parse in ReadBriefAdmission. Stay at or under 400 lines, or report the exact overrun.
