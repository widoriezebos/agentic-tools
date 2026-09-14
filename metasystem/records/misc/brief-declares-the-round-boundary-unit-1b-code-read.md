# Code read: brief-declares-the-round-boundary, unit 1b

Reader: Opus. I did not build this unit. Subject: the uncommitted five-file diff in `.claude/worktrees/bdrb-u1b` at HEAD a7a83bbf. It is 400 changed lines: 334 tracked additions and deletions plus a 66-line new file. Spec: `plans/brief-declares-the-round-boundary-design.md` revision 3. Unit 1a (e461dfd1) is an ancestor of HEAD.

I edited nothing in the live worktree. The sha256 of all five files matched `<S>/u1b-live.sha` before and after this read.

`<S>` is `/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/36c93128-ec16-4146-8def-3f706ba4ef10/scratchpad`.

Setup:
- `<S>/u1b-copy` is a fresh Git repository. Its commit holds `git archive HEAD metasystem`, and the five candidate files sit on top, uncommitted. Its `git diff --numstat` matches the worktree.
- `<S>/u1b-mut0`, `<S>/u1b-mut1` and `<S>/u1b-mut2` are copies of it, used for mutation runs.
- `<S>/ms-u1b-head` is built from HEAD. `<S>/ms-u1b-cand` is built from the copy.
- Every go test ran with METASYSTEM_BIN and METASYSTEM_CONTEXT_COST_PROOF unset and with -timeout 40m.
- No fixture bed ran. Nothing was committed.

## 1. Findings

Material: 5 (F-1 to F-5). Not material: 5 (F-6 to F-10).

### F-1. Severity medium. Material yes.
Claim: in bounded briefs, the new span handling changes authority results in both directions. A span is a backticked or quoted citation.
- A span that holds more than a path becomes one concrete "missing" path. Briefs that HEAD admits are refused.
- A quoted prose span that is not eligible still hides the path tokens inside it. A brief that cites a missing file is admitted.

Evidence:
- Code, recording: brief.go:358-366 records a span only when it is eligible. But brief.go:367-370 skips every old-scanner token inside any span, eligible or not.
- Code, eligibility: brief.go:459-471 applies only the directory rule. Two old-scanner rules never apply to a span: the `*<>${` exclusion (brief.go:372) and the token shape (brief.go:18). A span also does not stop at `:` or a space.
- Command: `bash <S>/u1b-probe.sh`, log `<S>/u1b-probe.log`. Each run was `job brief-mode --authority-only --brief B --base-tree <S>/u1b-copy --disk-root <S>/u1b-copy`, once with each binary.
- Each probe brief is `Working Mode: implement`, `Boundary: []`, `Ceiling: 1`, plus one line:
  - Probe 17: "See `metasystem/internal/dispatch/brief.go:47-53`." HEAD exits 0. The candidate exits 1 with `BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/dispatch/brief.go:47-53`.
  - Probe 19: "Write `metasystem/records/misc/<goal>-code-read.md`." The candidate refuses and names the placeholder path.
  - Probe 21: "Run `metasystem/scripts/agents/go-gate.sh --fast`." The candidate refuses and names `metasystem/scripts/agents/go-gate.sh --fast`.
  - Probe 22: "Edit `metasystem/scripts/agents/*.sh` only." The candidate refuses and names the glob.
  - Probe 23: `Say "read metasystem/internal/u1b-missing.md first".` HEAD exits 1 and names the missing file. The candidate exits 0.
  - The headerless versions (probes 18, 20 and 24) do not change, as the page requires.
- Real briefs, same probe: I took 37 recent briefs from the main checkout (`artifacts/agents/*/brief.md` and `artifacts/reports/*brief*.md`) and added `Boundary: []` and `Ceiling: 1` to each. 5 of 37 changed:
  - `artifacts/agents/implementer-b76a0ddc11f73e99fa195d84/brief.md` was admitted and is now refused: `metasystem/internal/landing/receipt.go:426, metasystem/internal/landing/testing.go:160`. The cause is its lines 54 and 58.
  - Four report briefs gain false missing paths: `artifacts/reports/codex-ccb-slice2-brief.md:146`, `artifacts/agents/<prior critic root>/reads-refused.jsonl`, `artifacts/agents/jobs/*.json` and `artifacts/agents/jobs/<job>.json`.
- Page lines 83 and 85 (Paths and matching) say:
  - backtick contents are literal;
  - every complete span is consumed;
  - a structured citation gets "the existing directory eligibility rule";
  - a special-byte input under an eligible directory is checked by its full identity.
- The builder followed that text, and the page says nothing about placeholders. The seat must first rule on that paragraph. After that, brief.go:358-372 and brief.go:459-471 change, with witnesses.
- Impact: once unit 6a tells seats to write these headers, ordinary briefs are refused wrongly, or admitted wrongly.

### F-2. Severity low. Material yes.
Claim: the H8.root-prefix test still passes when the CLI stops passing the real installation prefix.

Evidence:
- Mutation: change brief.go:137 `installPrefix, err = projectInstallPrefix(installRoot)` to `installPrefix, err = "", nil`.
- Result: `go test ./cmd/metasystem -run '^TestDispatchBriefBoundsAdmission$' -count=1 -timeout 40m` passes, all 12 subtests. This is record `H8.root-prefix/skip-lookup-probe` in `<S>/mut-u1b-*.jsonl`.
- Cause: the root-prefix case (dispatch_brief_bounds_test.go:44) uses the member `metasystem/internal/dispatch/brief.go`. That member is valid with an empty prefix and with `metasystem`. The builder's row points the CLI at a wrong root, which is a different mutation.
- A member without the prefix would catch it. Probe 10 (`Boundary: ["internal/dispatch/brief.go"]`), run through the candidate binary with a correct root, gives `BRIEF_BOUNDS_INVALID: Boundary: invalid path or pattern "internal/dispatch/brief.go"`.

### F-3. Severity low. Material yes.
Claim: the parse-once check does not cover the new file wrapper ReadBriefAdmissionAtRoot. A second bounds parse inside it passes every brief test in both packages.

Evidence:
- Mutation: insert `if _, err := parseBriefBounds(headers, installPrefix); err != nil { return BriefAdmission{}, err }` before brief.go:148.
- These both pass (record `P.parse-in-AtRoot-probe`):
  - `go test ./internal/dispatch -run '^(TestBriefAdmission|TestBriefMode.*|TestBriefAuthority.*|TestBriefBounds.*)$' -count=1 -timeout 40m`
  - `go test ./cmd/metasystem -run '^TestDispatchBriefBounds' -count=1 -timeout 40m`
- brief_bounds_test.go:132 checks only admitBriefBytes, BriefModeOnly and ReadBriefAdmission.
- Page line 59 says file wrappers delegate to the one byte-based helper and must not parse twice. Unit 1a's R3-2 closed the same gap for ReadBriefAdmission.

### F-4. Severity low. Material yes.
Claim: the tests for incomplete backtick and JSON spans cannot tell the old-scanner fallback apart from a span consumed to the end of the line. Also, no test covers invalid JSON that is terminated.

Evidence:
- The test inputs at brief_authority_test.go:181 are "`docs/missing.md" and `"docs/missing.md`. Both readings give `["docs/missing.md"]`.
- `go test ./internal/dispatch -run '^(TestBriefAuthorityIncompleteSpan|TestBriefAuthoritySpecialCharacterInput)$' -count=1 -timeout 40m` passes under each of three mutations:
  - brief.go:411-413: an unterminated backtick becomes a span to the end of the line.
  - brief.go:447: an unterminated JSON string becomes a span to the end of the line.
  - brief.go:439: an invalid JSON string that is terminated becomes a literal span.
- I added a temporary test in `<S>/u1b-mut0`, using script `<S>/u1b-span-probe.py`, and removed it afterwards.
  - Its inputs were "Read `docs/a b.md", `Read "docs/a b.md` and `Read "docs/a\qb.md"`. Each expects `["docs/a"]`.
  - It passes on the candidate. It fails under the matching mutation, for example `zz_reader_probe_test.go:15: ... MissingPaths:[]string{"docs/a b.md"}}, want missing paths ["docs/a"]`.
- The page's A3.incomplete-json row covers "an unterminated or invalid JSON string".

### F-5. Severity low. Material yes.
Claim: the installation-prefix lookup runs before the mode check, and it also runs for partial pairs. If --root cannot be resolved, an environment error takes the place of the mode refusal and of the pairing error. On the normal path the error also calls itself "authority admission".

Evidence:
- brief.go:134-141 looks up the prefix whenever either header is present. That happens before admitBriefBytes checks the mode (brief.go:108) or the pairing (brief.go:160).
- Probe: the candidate runs `job brief-mode --brief B` from a non-Git directory, with the default `--root .`:
  - Probe 11 (no mode, valid pair): HEAD exits 1 silently. The candidate exits 1 with `brief authority admission cannot resolve installation prefix: exit status 128`.
  - Probe 05 (`Boundary: []` only): the candidate prints the same prefix error, not `BRIEF_BOUNDS_INVALID: Ceiling: required with Boundary`.
- Page line 36 fixes the check order: duplicates, then pairing, then values.
- Page line 59: "Without authority, preserve the mode refusal before bounds validation."
- Page line 61: "Headerless admission needs no prefix lookup." A partial pair needs no prefix to be refused.
- The dispatcher is not affected. The shell passes `--root "$root"`. The probe's shell-shaped run from a non-Git directory gave the same result as a Git directory for every brief.

### F-6. Severity low. Material no.
Claim: three tests only search the source text, so a disguised removal passes them.

Evidence:
- mode-only (dispatch_brief_bounds_test.go:54): remove `--mode-only` from brief_mode and add the comment `# job brief-mode --mode-only is not used here`. `-run '^TestDispatchBriefBoundsAdmission$/^mode-only$'` still passes.
- mode-stdout (dispatch_brief_bounds_test.go:56): this check counts one exact spelling of `os.ReadFile(briefPath)`. A reread spelled `os.ReadFile(filepath.Clean(briefPath))` inside the authority closure passes the brief tests in both packages.
- root-prefix (dispatch_brief_bounds_test.go:55) is the same kind of check.
- Each test still fails on the plain removal (section 2).
- The read count also breaks if unit 3c adds a legitimate read.

### F-7. Severity low. Material no.
Claim: the new message at dispatch.sh:1542 names Boundary and Ceiling. But brief_mode now runs --mode-only, which never checks the pair.
- A partial pair passes line 1542.
- It stops at line 1655 with "brief authority admission refused", after Go's BRIEF_BOUNDS_INVALID line.

Evidence: probe 05 with `--mode-only` exits 0 and prints `implement`. The page sets both the message text and the mode-only call, so the page itself is inconsistent here. The operator still sees the right Go line.

### F-8. Severity low. Material no.
Claim: ValidateBriefAuthority passes the base tree as the installation root (brief.go:267). At a Git toplevel the prefix is empty. So this wrapper accepts Boundary members that the CLI refuses.

Evidence: `git -C <S>/u1b-copy rev-parse --show-prefix` prints an empty line. Probe 10 is refused through the CLI. No production code calls this wrapper any more; only tests do.

### F-9. Severity low. Material no.
Claim: this is a matter of taste only.
- Each admission scans the headers twice, at brief.go:134 and brief.go:106.
- ReadBriefAdmissionAtRoot repeats the read-failure branch of ReadBriefAdmission (brief.go:127-133 and brief.go:96-102).
- The pair is not decoded twice, which is the thing the page forbids.

### F-10. Severity low. Material no.
Claim: this is a landing note.
- dispatch.sh:25 runs `$root/bin/metasystem`, which is an untracked local build.
- An engine built before this change rejects `--mode-only` with exit 2. Line 1542 would then refuse every brief.
- The engine must be rebuilt when this lands.

Evidence: `git ls-files metasystem/bin` prints nothing. Probe 01 with `--mode-only` on the HEAD binary prints `flag provided but not defined: -mode-only` and exits 2.

### Conformance checks with no finding
- Boundary: `git status --short` shows only the five paths.
- Ceiling: `git diff --numstat` gives 14+6, 140+28, 140+0 and 3+3, plus 66 new lines. The total is 400, exactly at the Ceiling.
- Later units: none were built. The diff has none of these:
  - unit 2's line counting;
  - `--admission-dir`, the admitted record, the composition marker or `ReadRoundBriefBounds` (units 3a to 3c);
  - any change under `internal/validate` (units 4a to 4c);
  - template, instruction or bed changes (units 6a and 6b).
- Carry-in R3-1:
  - TestBriefMode covers the exactly-one guard, the placeholder guard and the returned Mode.
  - TestDispatchBriefBoundsAdmission/mode-empty covers the empty guard.
  - No internal test covers the empty guard. The probe `R3-1.nonempty/internal-probe` passes.
- Carry-in R3-3: BriefModeOnly now reads and scans the brief itself (brief.go:88-94). It no longer calls BriefMode.
- H8.authority-before-mode: both mode checks have tests that fail when removed.
- ReadBriefAdmissionAtRoot and its prefix error text do not contradict the page.
  - The page names no helper besides the byte-based one.
  - Unit 3c only needs an admission value, and this function returns one.
  - The error wording on the normal path is inaccurate (F-5).
- Authority identity:
  - The closure passes the admitted bytes and the parsed bounds (brief.go:142-144).
  - Passing empty bounds instead fails 11 subtests across SpecialCharacterInput and BoundaryInputStillRequired.
  - Input still wins over output. Changing brief.go:389 to `if !use.output` fails output-then-input and input-then-output.
- `--mode-only` and `--root`:
  - `--mode-only` conflicts with `--authority-only`, `--base-tree` and `--disk-root` (dispatch_verbs.go:2008).
  - `--root` defaults to ".", as the page says. `--mode-only` accepts it and ignores it.
- dispatch.sh callers, all correct:
  - brief_mode (lines 858-860) uses `--mode-only` and serves line 1542.
  - brief_authority (lines 862-864) passes `--root "$root"` and serves lines 1655 and 2643.
- Existing tests:
  - The test diff has zero deleted lines.
  - TestBriefMode, TestBriefModeOnly, TestBriefAdmission and the older TestBriefAuthority* tests all use headerless briefs. So they stay on the old path, and all pass under -race.
  - The older backtick citations at brief_authority_test.go:57 and :111 are in headerless briefs.
- Shell hygiene:
  - The change adds two quoted operands and changes one message string.
  - It adds no pipe, no array and no unset variable.
  - `$root` and `$repo_scope` are set at dispatch.sh:21-24.
  - `/bin/bash -n` under GNU bash 3.2.57 passes.

### job brief-mode output changes: HEAD binary compared with the candidate
Log: `<S>/u1b-probe.log`. 29 briefs, each run in six ways.
- Mode-only brief:
  - No change in any mode.
  - `--mode-only` is new and prints `implement`.
- Headerless brief: no change in any mode.
- Valid pair:
  - From a Git directory: no change.
  - From a non-Git directory without `--root`: now fails with the prefix error (F-5).
- Partial pair:
  - Normal, combined and authority-only runs go from exit 0 to exit 1, with `BRIEF_BOUNDS_INVALID: Ceiling: required with Boundary` or `BRIEF_BOUNDS_INVALID: Boundary: required with Ceiling`.
  - `--mode-only` still prints the mode.
  - Authorised by H8.normal, H8.authority-only and H8.mode-only.
- Malformed pair: bad Boundary JSON, Ceiling -1, a duplicate Boundary, or a member without the prefix.
  - Same change as a partial pair, with 1a's exact error details.
  - Authorised.
- No mode with a partial or malformed pair:
  - Normal: no change (silent mode refusal).
  - Combined: goes from the silent mode refusal to BRIEF_BOUNDS_INVALID.
  - Authority-only: goes from exit 0 to BRIEF_BOUNDS_INVALID.
  - A partial pair that cites a missing file now reports BRIEF_BOUNDS_INVALID instead of BRIEF_AUTHORITY_REFUSED.
  - This matches the page's single helper, which parses before checking authority.
- Indented bounds example citing a missing file: combined and authority-only go from refused to admitted. Authorised by A2.
- Bounded briefs with backticked or quoted text that is not a path: see F-1. In effect this change is not authorised.

### Verification on the copy
Log: `<S>/u1b-verify-reader.log`.
- `go build -o <S>/ms-u1b-cand ./cmd/metasystem`: exit 0.
- `gofmt -l internal/dispatch cmd/metasystem`: no output.
- `go vet ./internal/dispatch ./cmd/metasystem`: exit 0.
- `bash -n scripts/agents/dispatch.sh`: exit 0. `/bin/bash` 3.2.57 also passes.
- `go test -race -count=1 -timeout 40m ./internal/dispatch/`: ok, 77.842s.
- `go test -race -count=1 -timeout 40m ./cmd/metasystem -run 'TestDispatchBriefBounds|TestBriefMode|TestDispatchBriefMode' -v`: ok. All 12 admission subtests and TestDispatchBriefBoundsFailureSuffix pass. The other two name patterns match no test in this package.

## 2. The reproduced rows

The builder's table has 53 data rows, not 55. All 53 reproduce.

How they ran:
- Driver: `<S>/mutate-u1b.py`, with three workers on `<S>/u1b-mut0`, `<S>/u1b-mut1` and `<S>/u1b-mut2`.
- Results: `<S>/mut-u1b-{0,1,2}.jsonl`, with logs under `.logs/`.
- Each mutation was applied on its own.
- Each named test ran as `go test <pkg> -run '<regex>' -count=1 -timeout 40m -v`.
- After each run the file was restored and all five hashes were checked. Every run restored cleanly.
- There are 50 mutation records. 13 of them are probes beyond the builder's table.

In the table below, "same file" means the test file named in the row above.

| Builder row | Mutation (location) | Test run | Observed failure |
| --- | --- | --- | --- |
| A1.file, A1.directory, A1.glob, A1.special-name | remove `strings.HasPrefix(line, "Boundary:") \|\|` (brief.go:355) | `^TestBriefAuthorityBoundaryIsOutput$` | all four subtests fail at brief_authority_test.go:141, for example MissingPaths `docs/new.md`, `docs/new/`, `docs/*.md` |
| A2.spaces, A2.tabs, A2.headerless, A2.separate-input | remove `\|\| inertBriefBoundsLine(line)` (brief.go:355) | `^TestBriefAuthorityIndentedBoundsExampleIsInert$` | all four fail at same file :162 |
| A2.separate-input, second mutation | an inert line also skips the next line | `.../^separate-input$` | :162 error nil, want records/missing.md |
| A2.quoted | treat `>` as indentation (brief.go:403) | `.../^quoted$` | :162 error nil, want records/missing.md |
| A3.space, .comma, .quote, .tab, .newline, .backslash | `if bounds.Boundary != nil` becomes `if false` (brief.go:359) | `^TestBriefAuthoritySpecialCharacterInput$` | all six fail at same file :175, got docs/a |
| A3.json-in-backticks | disable JSON decoding inside backticks (brief.go:416) | `.../^json-in-backticks$` | :175 error nil |
| A3.literal-backticks | drop backtick spans that are not JSON (brief.go:416-419) | `.../^literal-backticks$` | :175 got docs/a |
| A3.consumed-span | briefCitationConsumes never matches (brief.go:452) | `.../^consumed-span$` | :175 got ["docs/a","docs/a b.md"] |
| A3.incomplete-backtick | unterminated backtick is consumed and dropped (brief.go:411-413) | `^TestBriefAuthorityIncompleteSpan$/^incomplete-backtick$` | same file :184 error nil. See F-4 |
| A3.incomplete-json | unterminated JSON is consumed and dropped (brief.go:422) | `.../^incomplete-json$` | :184 error nil. See F-4 |
| A4.ordinary and A5.unquoted | the old token scan returns nothing (brief.go:367) | `BoundaryInputStillRequired/^ordinary$` and `BoundedCitationCompatibility/^unquoted$` | same file :213 and :220, error nil |
| A4.root-file, A4.new-directory | concrete-member equality off (brief.go:461) | `^TestBriefAuthorityBoundaryInputStillRequired$` | both fail at :213, error nil |
| A4.pattern-no-equality, A4.backslash-no-equality | remove the pattern guard (brief.go:461) | same | both fail at :213, got new/*.md and new/a\b.md |
| A4.directory-no-equality | remove the directory guard (brief.go:461) | `.../^directory-no-equality$` | :213 got new/file.md/ |
| A4.output-then-input | an earlier output suppresses a later input (brief.go:333-334) | `.../^output-then-input$` | :213 error nil |
| A4.input-then-output | a later output clears an earlier input (brief.go:335) | `.../^input-then-output$` | :213 error nil |
| A5.workspace | span output flag forced to false (brief.go:363) | `BoundedCitationCompatibility/^workspace$` | same file :230 got records/a b.md |
| A5.create | remove the Create prefix check (brief.go:334) | `.../^create$` | :230 got records/a b.md |
| A5.artifact | disable the live artifact lookup (brief.go:291) | `.../^artifact$` | same file :242 got artifacts/a b.json |
| A5.headerless | spans on without bounds (brief.go:359) | `.../^headerless$` | same file :245 got docs/a b.md, want docs/a |
| H8.normal | normal path takes BriefModeOnly (dispatch_verbs.go:2028) | `^TestDispatchBriefBoundsAdmission$/^normal$` | dispatch_brief_bounds_test.go:50 exit 0 stdout "implement\n" |
| H8.authority-only | parse only when requireMode is true (brief.go:111) | `.../^authority-only$` | same file :32 partial pair exits 0 |
| H8.authority-only / no mode | requireMode forced to true (dispatch_verbs.go:2028) | `.../^authority-only$` | :30 valid brief exits 1 |
| H8.mode-stdout | print removed (dispatch_verbs.go:2033) | `.../^mode-stdout$` | :52 exit 0 stdout "" |
| H8.authority-before-mode | the mode check before authority runs always (brief.go:108) | `.../^authority-before-mode$` | :50 exit 1, stderr "" |
| H8.authority-before-mode / post-check | brief.go:120-122 removed | `.../^authority-before-mode-required$` | :50 exit 0 stdout "\n" |
| H8.mode-only | `--mode-only` removed from dispatch.sh:859 | `.../^mode-only$` | :54 early mode discovery does not use mode-only. A second mutation, where the CLI mode-only path calls BriefMode, fails at :52 with BRIEF_BOUNDS_INVALID: Ceiling: required with Boundary |
| H8.mode-only-conflict / authority-only, base-tree, disk-root | each operand removed from dispatch_verbs.go:2008 | `.../^mode-only-conflict$`, `.../^mode-only-base-conflict$`, `.../^mode-only-disk-conflict$` | :50 exit 2 with the later flag-pairing error |
| H8.root-prefix / CLI | admission uses "." instead of --root (dispatch_verbs.go:2028) | `.../^root-prefix$` | :52 BRIEF_BOUNDS_INVALID invalid path "metasystem/internal/dispatch/brief.go". See F-2 |
| H8.root-prefix / shell | `--root "$root"` removed (dispatch.sh:863) | `.../^root-prefix$` | :55 authority admission does not pass root |
| H8.headerless-no-prefix | prefix lookup runs always (brief.go:136) | `.../^headerless-no-prefix$` | :52 exit 1 cannot resolve installation prefix: exit status 128 |
| H8.suffix | old message restored (dispatch.sh:1542) | `^TestDispatchBriefBoundsFailureSuffix$` | :64 dispatch suffix missing |
| R3-1 exactly one mode | `!= 1` becomes `== 0` (brief.go:83) | `^TestBriefMode$` | decisions_test.go:359 double.md: expected a refusal |
| R3-1 nonempty mode | empty guard removed (brief.go:83) | `^TestDispatchBriefBoundsAdmission$/^mode-empty$` | dispatch_brief_bounds_test.go:50 exit 0 stdout "\n" |
| R3-1 no placeholder | placeholder guard removed (brief.go:83) | `^TestBriefMode$` | decisions_test.go:359 placeholder.md: expected a refusal |
| R3-1 returned Mode | returns "implement" (brief.go:86) | `^TestBriefMode$` | decisions_test.go:349 BriefMode = "implement" |
| R3-3 wrappers do not call one another | BriefModeOnly returns BriefMode(briefPath) (brief.go:88-94) | `^TestBriefModeOnly$/^partial-pair$` | brief_bounds_test.go:102 mode = "", BRIEF_BOUNDS_INVALID: Ceiling: required with Boundary |
| One admitted read feeds authority | closure rereads with os.ReadFile(briefPath) (brief.go:142-144) | `^TestDispatchBriefBoundsAdmission$/^mode-stdout$` | dispatch_brief_bounds_test.go:56 public admission can reread authority bytes. See F-6 |

Probes beyond the table. Each gave the expected result:
- Tests expected to pass, showing a gap:
  - `H8.root-prefix/skip-lookup-probe` (F-2).
  - `P.parse-in-AtRoot-probe` (F-3).
  - `A3.incomplete-backtick/consume-as-literal-probe` and `A3.incomplete-json/consume-as-literal-probe` (F-4).
  - `One-read/reread-spelling-probe` (F-6).
  - `H8.mode-only/shell-comment-probe` (F-6).
  - `R3-1.nonempty/internal-probe`: only the CLI test covers the empty-mode guard.
- Tests expected to fail, showing that authority identity holds:
  - `Identity/bounds-dropped` fails 11 subtests.
  - `A4.input-wins/final-filter` fails 2 subtests.

## 3. Verdict

Send back. Five findings are material.

The diff already sits at the 400-line Ceiling, so any fix goes over it. Under the page's unit rule, the seat approves a split, or a stated overrun, before the builder starts.

What must change, in this order:
1. F-1: the seat rules on how spans are classified and consumed (page lines 83 and 85). Then brief.go:358-372 and brief.go:459-471 change to match, with named tests. The tests cover backticked `path:line`, commands with arguments, `<placeholder>` paths, and quoted prose that hides a missing path.
2. F-5: do not look up the prefix before the mode refusal on the path without authority, or for a pair that is not exactly one of each. Fix the "authority admission" wording on the normal path.
3. F-2: add a root-prefix case whose result depends on the prefix, for example a member without the prefix that expects BRIEF_BOUNDS_INVALID.
4. F-3: extend the parser-ownership check to ReadBriefAdmissionAtRoot.
5. F-4: use incomplete-span inputs that tell fallback apart from consumption, and add an invalid terminated JSON case.
