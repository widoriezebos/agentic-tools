# Code read: brief-declares-the-round-boundary, unit 1b-i (closing read after the fold)

Reader: Opus. I did not build this unit. Subject: the uncommitted six-file diff in `.claude/worktrees/bdrb-u1b` on HEAD 914b61ed. It is 305 changed lines: 212 tracked additions and deletions plus the 93-line new test file. Spec: the fold brief `artifacts/reports/codex-bdrb-u1b-fold-brief.md` (seat rulings 1 and 2), then `plans/brief-declares-the-round-boundary-design.md` revision 3.

`<S>` is `/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/36c93128-ec16-4146-8def-3f706ba4ef10/scratchpad`.

Setup:
- I edited nothing in the live worktree. Its six sha256 values matched `<S>/u1bi-live.sha` at the start and at the end.
- `<S>/u1bi-copy` is a fresh Git repository. Its one commit holds `git archive HEAD metasystem`. The six candidate files sit on top, uncommitted. Its `git diff --numstat` matches the worktree.
- `<S>/u1bi-head` holds the same archive without the candidate files. Each of its five tracked files matches `git show HEAD:`.
- Binaries: `<S>/ms-u1bi-cand` is built from the copy. `<S>/ms-u1bi-head` is built from HEAD.
- Every mutation ran in the copy. After each one the driver restored the file and checked all six hashes. All 26 restored cleanly.
- METASYSTEM_BIN and METASYSTEM_CONTEXT_COST_PROOF were unset. Every go test used `-timeout 40m`. No fixture bed ran. Nothing was committed.
- Scripts and logs:
  - `<S>/u1bi-setup2.sh`;
  - `<S>/u1bi-mutate.py`, with `<S>/u1bi-mut.jsonl` and `<S>/u1bi-mut.jsonl.logs/`;
  - `<S>/u1bi-probe.sh`, with `<S>/u1bi-probe.log`;
  - `<S>/u1bi-chain.sh`, with `<S>/u1bi-chain.log`.

## 1. Findings

Material: 2 (F-11, F-12). Not material: 8 (F-13 to F-20). Round 1 used F-1 to F-10, so new ids start at F-11.

### F-11. Severity low. Material yes.
Claim: the H8.root-prefix witness no longer fails when the CLI ignores `--root`. The fold replaced the positive root-prefix case instead of adding a second case. A witness the fold brief said to keep got weaker.

Evidence:
- Round 1's case (`<S>/bdrb-u1b-code.diff:50`) used the member `metasystem/internal/dispatch/brief.go` and expected exit 0 with `implement`. My round-1 mutation "admission uses "." instead of --root" failed it at :52 (`codex-bdrb-u1b-read-1.md:246`).
- The new case (dispatch_brief_bounds_test.go:44) uses `internal/dispatch/brief.go` and expects BRIEF_BOUNDS_INVALID.
  - Any nonempty prefix refuses that member, right or wrong.
  - No CLI case admits a prefixed member any more.
- Mutation `F-2.cli-ignores-root-probe`: dispatch_verbs.go:2028 passes `"."` instead of `*root`, plus `_ = root`.
  - `go test ./cmd/metasystem -run '^TestDispatchBriefBounds' -count=1 -timeout 40m` passes, 19 of 19.
  - No other test calls runDispatchBriefMode.
- The fold brief keeps "the --mode-only and --root operands ... with their witnesses".
- Page row H8.root-prefix: "The CLI passes the actual installation prefix to bounded admission."
- Impact: under the mutation, the prefix comes from whatever directory the dispatcher runs in, and no test notices.
- Artifact to change: cmd/metasystem/dispatch_brief_bounds_test.go.

### F-12. Severity low. Material yes.
Claim: the F-5 ordering witnesses cover only a partial pair and a Boundary that is not JSON. The prefix lookup can move to after JSON decoding but before member validation, or before the Ceiling check, and every brief test still passes. That placement is 1a's own shape, where projection ran inside the member loop. A later refactor could bring it back silently.

Evidence:
- Mutation `F-5.lookup-before-member-validation-probe`: a resolver call inserted before brief.go:170.
- Mutation `F-5.lookup-before-ceiling-probe`: the same call inserted before brief.go:180.
- Under each mutation, all of these pass:
  - `go test ./cmd/metasystem -run '^(TestDispatchBriefBoundsPrefixLookupOrdering|TestDispatchBriefBoundsAdmission)$' -count=1 -timeout 40m`: 18 passes.
  - `go test ./internal/dispatch -run '^(TestBriefBounds.*|TestBriefAdmission|TestBriefModeOnly|TestBriefMode)$' -count=1 -timeout 40m`: 72 passes.
  - `go test ./internal/dispatch -run '^(TestBriefAuthority.*|TestBriefMode)$' -count=1 -timeout 40m -v`: 23 passes, 0 failures under each mutation. These are the remaining tests that reach ReadBriefAdmissionAtRoot or BriefMode; their lookups use a fixed prefix or succeed, so placement cannot change their result.
- Under either mutation, a bad Ceiling or an invalid member from outside Git with the default `--root` gets the prefix error. The fold brief forbids that: "a partial or malformed pair gets BRIEF_BOUNDS_INVALID, before any prefix lookup".
- The code is right today. The git-call census in section 2 shows `Ceiling: -1`, `["/abs"]` and `["metasystem/a["]` refused with zero git calls.
- The ordering cases are at dispatch_brief_bounds_test.go:69-72. The only malformed case is `Boundary: bad`.
- Artifact to change: cmd/metasystem/dispatch_brief_bounds_test.go (about two lines).

### F-13. Severity low. Material no.
Claim: deferring the lookup changes 1a's error precedence. Page line 36 says "validate Boundary, then Ceiling". That no longer holds when the Boundary error needs the prefix. The result file does not mention the change.

Evidence: the probe `<S>/u1bi-precedence-probe_test.go` calls ParseBriefBounds. It ran in both copies and was then removed.

| Input (prefix) | HEAD (1a) | Candidate |
| --- | --- | --- |
| `["other/a"]`, Ceiling `bad` (`metasystem`) | `Boundary: invalid path or pattern "other/a"` | `Ceiling: expected a nonnegative decimal integer ...` |
| `[]`, Ceiling `bad` (`meta[system`) | `Boundary: installation prefix contains unsupported pattern bytes` | `Ceiling: expected a nonnegative decimal integer ...` |
| `["other/a", "metasystem/b["]` (`metasystem`) | `invalid path or pattern "other/a"` | `invalid path or pattern "metasystem/b["` |
| `["a["]` (`meta[system`) | unsupported prefix bytes | `invalid path or pattern "a["` |
| `["/a"]` (`meta[system`) | unsupported prefix bytes | `invalid path or pattern "/a"` |

Binary brief 23 shows the candidate side of the first row through the CLI. The HEAD binary does not parse bounds, so it admits that brief.

Why not material:
- The fold brief allows the lookup only for a complete, valid pair.
- With a bad Ceiling no lookup may run, so a Boundary error that needs the prefix cannot win.
- Every row above follows from that rule, and no test pins the old order.
- Nothing needs building. The seat's amendment should record the new order, because it also applies to ParseBriefBounds with a fixed prefix, which later units call.

### F-14. Severity low. Material no.
Claim: page rows A1 and A2 stay in this unit, and they change citation results against trunk. That includes indented Boundary or Ceiling lines in headerless briefs. The result's sentence "Authority extraction therefore remains HEAD's token scanner" goes too far.

Evidence:
- Compared with `git show HEAD:`, the extraction section differs only in two places:
  - brief.go:351-353 skips a live `Boundary:` line or an inert line;
  - brief.go:387-389 adds inertBriefBoundsLine.
- In the binary comparison, combined and authority-only runs change from refused to admitted for:
  - brief 19, a Boundary naming a new file;
  - brief 20, a headerless brief with an indented example;
  - brief 21, a bounded brief with a tab-indented Ceiling example.
- None of the shapes the seat named is affected (section 3).

Why not material:
- Page rows A1 and A2 give these rules to unit 1b.
- Ruling 1 amends only the span paragraph, and the amendment's 1b-ii list does not take these rows.
- The builder's table lists both rules with witnesses.
- Without A1, any Boundary that names a new file fails authority admission. The fold brief's own Boundary names one.
- If the seat's "no difference is authorised for citation results" was meant to cover these two rules, this finding becomes material. The fix would then remove brief.go:351-353, brief.go:387-389 and their two tests.

### F-15. Severity low. Material no.
Claim: after the split, page rows A3, A4 and A5 have no named owner. This unit is right to leave them out. The note is for the amendment.

Evidence:
- The amendment (`<S>/bdrb-u1bi-amendment.md`, ruling A2) gives 1b-ii "ruling A1 and F-4's incomplete-span cases".
- A4 (concrete-member eligibility, page lines 565-572) is named for neither unit, and neither is A5 (compatibility, lines 573-577).
- The builder's round-2 table has no A3, A4 or A5 rows.

### F-16. Severity low. Material no.
Claim: TestDispatchBriefBoundsPrefixLookupOrdering assumes its temporary directory is outside Git. With GIT_DIR exported, or with TMPDIR inside a repository, `valid-pair-uses-prefix` fails. The other three cases then pass without proving anything, because the lookup succeeds.

Evidence:
- `GIT_DIR=<S>/u1bi-copy/.git go test ./cmd/metasystem -run '^TestDispatchBriefBoundsPrefixLookupOrdering$' -count=1 -timeout 40m` prints `dispatch_brief_bounds_test.go:82: exit 0, stdout "implement\n", stderr ""` and exits 1.
- `TMPDIR=<S>/u1bi-gittmp` (a fresh repository) gives the same line.
- scripts/agents/go-gate.sh clears neither variable, and no hook calls the gate. The normal run passes.
- The test could set GIT_CEILING_DIRECTORIES and clear GIT_DIR for its own run.

### F-17. Severity low. Material no.
Claim: some "observed failure" cells in the builder's table paraphrase the message. Every line number matches.

Evidence:

| Row | Builder's cell | Observed |
| --- | --- | --- |
| The same admitted bounds reach authority | "authority bounds differ" | `brief_bounds_test.go:141: admission = {...}, checked bounds = {Boundary:[] Ceiling:<nil>}, error = <nil>` |
| Headerless lookup is Git-independent | "prefix resolution failed" | `:52: exit 1, stdout "", stderr "brief admission cannot resolve installation prefix: exit status 128\n"` |
| Parsed Working Mode is returned | "got implement, want deep-work" | `decisions_test.go:349: BriefMode = "implement", <nil>` |
| Authority precedes required-mode refusal | "missing authority diagnostic" | `:50: exit 1, stdout "", stderr ""` |

### F-18. Severity low. Material no.
Claim: the removed span scanner left one unused parameter. validateBriefAuthority takes `_ BriefBounds` (brief.go:276) and nothing reads it. It is still the page's plumbing: authority receives the parsed bounds. TestBriefAdmission/exact-bytes witnesses it, and 1b-ii will read it. No span helper or span test is left.

Evidence:
- `grep -n -E 'briefCitation|briefStructuredCitations|decodeBriefJSONString|briefCitationConsumes|briefStructuredAuthorityEligible|spans' internal/dispatch/brief.go` exits 1.
- A grep of the brief test files for `SpecialCharacterInput|IncompleteSpan|BoundaryInputStillRequired|BoundedCitationCompatibility|briefCitation` also exits 1.

### F-19. Severity low. Material no.
Claim: the round-1 non-material findings mostly still stand.
- F-6: the source-text checks at dispatch_brief_bounds_test.go:54-56 are unchanged.
- F-7: dispatch.sh:1542 still names Boundary and Ceiling, while `--mode-only` never checks them.
- F-8: ValidateBriefAuthority still uses the base tree as the installation root (brief.go:272). Only tests call it.
- F-9: the double header scan is gone. The repeated read-failure branch remains, at brief.go:97-103 and :128-134.
- F-10: the engine rebuild is now in the amendment's landing note.

### F-20. Severity low. Material no.
Claim: the valid pair from a non-Git directory with the default `--root` now fails with the prefix error. This is a visible change of the public verb.

Evidence: section 3, non-Git normal column. The ordering test `valid-pair-uses-prefix` pins it. It is authorised: H8.normal validates the pair, a valid pair needs the prefix, and the fold brief allows the lookup for exactly this case. dispatch.sh passes `--root` and uses `--mode-only` at line 1542, so dispatch is not affected.

### Conformance checks with no finding
- Boundary: `git status --short --untracked-files=all` lists exactly the six paths.
- Ceiling:
  - `git diff --numstat` gives 14+6, 63+33, 66+2, 18+4 and 3+3, which is 212.
  - The new file adds 93, so the total is 305, under 400.
  - This matches the result file.
- Later units (step 5): the diff has none of these:
  - span scanning or concrete-member eligibility (1b-ii);
  - `--admission-dir`, the admitted record, `brief-bounds.json` or ReadRoundBriefBounds (3a to 3c);
  - line counting (2);
  - anything under internal/validate (4a to 4c);
  - template or bed changes (6a, 6b).
  - A grep of the diff and the new file for those identifiers exits 1.
- Callers:
  - brief_mode uses `--mode-only` (dispatch.sh:858-860, serving line 1542).
  - brief_authority passes `--root "$root"` (dispatch.sh:862-864, serving lines 1655 and 2643).
  - No other Go or shell code calls `job brief-mode`.
  - BriefMode has no production caller.
- `--mode-only` conflicts with `--authority-only`, `--base-tree` and `--disk-root` (dispatch_verbs.go:2008-2011). It accepts `--root` and ignores it.

## 2. F-1 to F-5 closure

| Id | Probe re-applied | Observed now | Status |
| --- | --- | --- | --- |
| F-1 | Structured span consumption in the bounded path | Span helpers and their four test groups are gone (grep exit 1). Against `git show HEAD:`, the extraction section differs only by the A1 and A2 skip (brief.go:351-353, :387-389). Restoring round 1's extraction section fails all five shapes at brief_authority_test.go:180 (section 4). The binary corpus and a 37-brief sweep show no citation difference (section 3). Round 1's sweep had 5 of 37 changed. | closed |
| F-2 | Force the prefix to "" (brief.go:142 becomes `installPrefix, err := "", error(nil)`) | root-prefix: `dispatch_brief_bounds_test.go:50: exit 0, stdout "implement\n", stderr ""`. valid-pair-uses-prefix: the same text at `:82`. | closed. The "." variant is open as F-11 |
| F-3 | A second bounds parse in ReadBriefAdmissionAtRoot (`parseBriefBounds(scanBriefHeaders(data), resolveInstallPrefix)` before brief.go:148), with round 1's test selection | `brief_bounds_test.go:146: bounds parser ownership changed`. The `ParseBriefBounds(data, "")` variant fails the same way. | closed |
| F-4 | Incomplete-span inputs | This unit has no span code. Ruling 2 moves the cases to 1b-ii. | deferred to 1b-ii by ruling 2 |
| F-5 | Candidate binary, `job brief-mode --brief B` from a non-Git directory, default `--root`, a logging `git` shim first on PATH | No mode with a valid pair: exit 1, silent, 0 git calls. `Boundary: []` alone: `BRIEF_BOUNDS_INVALID: Ceiling: required with Boundary`, 0 git calls. Bad Boundary JSON, `Ceiling: -1`, `["/abs"]`, `["metasystem/a["]` and a duplicate Boundary: each BRIEF_BOUNDS_INVALID, 0 git calls. No mode with a partial pair: silent, 0 git calls. Valid pair: `brief admission cannot resolve installation prefix: exit status 128`, 1 `rev-parse --show-prefix` call. Restoring round 1's eager lookup fails mode-before-prefix, partial-before-prefix and malformed-before-prefix at :82. | closed. Untested placements are open as F-12 |

## 3. Binary comparison

Command: `bash <S>/u1bi-probe.sh`, log `<S>/u1bi-probe.log`. It runs the HEAD 914b61ed binary and the candidate binary. The base tree and disk root are both `<S>/u1bi-copy`. Each brief ran five ways:
- normal: `job brief-mode --brief B`, from `<copy>/metasystem`.
- combined: normal plus `--base-tree <copy> --disk-root <copy>`.
- authority: `--authority-only` with the same two roots. The candidate also gets `--root <copy>/metasystem`, as dispatch.sh passes it.
- shell: HEAD `--brief B` against candidate `--mode-only --brief B`. This is the change at dispatch.sh:859.
- non-Git: `--brief B` from a directory outside Git, with the default `--root`.

A bounded brief is `Working Mode: implement`, `Boundary: []`, `Ceiling: 1`, plus the line shown.

The seat's corpus:

| Brief | normal | combined | authority | shell | non-Git |
| --- | --- | --- | --- | --- | --- |
| 01 "Read \`metasystem/internal/landing/u1bi-missing.go:426\`." | same, `implement` | same | same, both `BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/landing/u1bi-missing.go` | same | differs: prefix error |
| 02 "See \`metasystem/internal/dispatch/brief.go:47-53\`." | same | same | same, both admit | same | differs |
| 03 "Run \`metasystem/scripts/agents/go-gate.sh --fast\`." | same | same | same, both admit | same | differs |
| 04 "Run \`metasystem/scripts/agents/u1bi-missing.sh --fast\`." | same | same | same, both refuse `metasystem/scripts/agents/u1bi-missing.sh` | same | differs |
| 05 "Write \`metasystem/records/misc/<goal>-code-read.md\`." | same | same | same, both admit | same | differs |
| 06 "Edit \`metasystem/scripts/agents/*.sh\` only." | same | same | same, both admit | same | differs |
| 07 `Say "read metasystem/internal/u1bi-missing.md first".` | same | same | same, both refuse `metasystem/internal/u1bi-missing.md` | same | differs |
| 08 all seven lines, with `Boundary: ["metasystem/internal/dispatch/brief.go"]` and `Ceiling: 400` | same | same | same, both refuse the same three paths | same | differs |
| 09 headerless: Working Mode and the seven lines | same | same | same, both refuse the same three paths | same | same |
| 10 the seven lines with no headers at all | same, silent | same | same, both refuse the same three paths | same | same |
| 11 mode only | same | same | same | same | same |
| 12 real brief, unmodified | same | same | same, both admit | same | same |
| 13 real brief with `Boundary: []` and `Ceiling: 1` after its Working Mode | same | same | same, both exit 0 with no output | same | differs |
| 14 real brief with `Boundary: ["metasystem/internal/landing/receipt.go", "metasystem/internal/landing/testing.go"]` and `Ceiling: 400` | same | same | same, both exit 0 with no output | same | differs |

The real brief is `artifacts/agents/implementer-b76a0ddc11f73e99fa195d84/brief.md`. Round 1 found its lines 54 and 58 refused.

Real-brief sweep: 37 recent briefs from the main checkout (`artifacts/agents/*/brief.md` and `artifacts/reports/*brief*.md`). Each ran authority-only through HEAD on the original. The candidate ran the original and a copy with `Boundary: []` and `Ceiling: 1` added. 0 of 37 differ.

Every difference, and whether it is authorised:

| Briefs and runs | HEAD | Candidate | Authorised? |
| --- | --- | --- | --- |
| Citation results for 01 to 14 and the 37-brief sweep | | no difference | nothing to authorise |
| Valid pair, non-Git run (01 to 08, 13, 14, 19, 21, 22, 29) | `exit=0 stdout=implement` | `exit=1`, `brief admission cannot resolve installation prefix: exit status 128` | yes: H8.normal plus the fold's rule that a valid pair uses the lookup. Dispatch is not affected. See F-20 |
| Partial or malformed pair (15 `Boundary: []` alone, 16 `Boundary: bad`, 17 `Ceiling: -1`, 23 unprefixed member with `Ceiling: bad`, 24 unprefixed member, 25 `["/abs"]`, 26 `["metasystem/a["]`, 27 duplicate Boundary): normal, combined, authority and non-Git runs | admitted | BRIEF_BOUNDS_INVALID with 1a's details. Brief 24 run outside Git gets the prefix error, since that check needs the prefix. The shell run is unchanged. | yes, by H8 |
| No mode with a partial pair (28): combined and authority runs | combined: silent mode refusal. authority: exit 0 | `BRIEF_BOUNDS_INVALID: Ceiling: required with Boundary` | yes, by H8 and the page's single admission helper. The normal run is unchanged (silent) |
| No mode with a valid pair (18) | | the same in all five runs, including outside Git | nothing to authorise. This is F-5's case |
| A1: 19 `Boundary: ["metasystem/internal/dispatch/u1bi-new.go"]`, combined and authority runs | `BRIEF_AUTHORITY_REFUSED: ... metasystem/internal/dispatch/u1bi-new.go` | admitted | by page row A1, not by a ruling. See F-14 |
| A2: 20 headerless `  Boundary: ["metasystem/internal/u1bi-missing.md"]` and 21 bounded `\tCeiling: metasystem/internal/u1bi-missing.md`, combined and authority runs | refused, naming `metasystem/internal/u1bi-missing.md` | admitted | by page row A2, not by a ruling. See F-14 |
| 22 `> Boundary: [...]` | | unchanged, except the valid-pair non-Git run | A2.quoted holds |

## 4. Reproduced rows

Driver: `python3 <S>/u1bi-mutate.py <S>/u1bi-copy/metasystem <S>/u1bi-mut.jsonl`.
- 26 mutations, applied one at a time.
- Each named test ran as `go test <pkg> -run '<regex>' -count=1 -timeout 40m -v`.
- After each run the file was restored and all six hashes checked. Every record says `restored=True`.

In the tables, ADM is `./cmd/metasystem TestDispatchBriefBoundsAdmission` and ORD is `./cmd/metasystem TestDispatchBriefBoundsPrefixLookupOrdering`. Line numbers without a file name are in dispatch_brief_bounds_test.go.

### Rows for F-2, F-3, F-5 and trunk-scanner compatibility (all reproduced)

| Builder row | Mutation applied | Test | Observed |
| --- | --- | --- | --- |
| Trunk token scanning unchanged for the five shapes | brief.go from `func validateBriefAuthority(` up to `func artifactAuthorityPath(` replaced with round 1's section (`<S>/u1bi-mutant-spans-brief.go`) | `./internal/dispatch ^TestBriefAuthorityBoundedUsesTrunkTokenScanner$` | All five fail at brief_authority_test.go:180. They report missing paths `docs/missing.md:426`, `scripts/agents/go-gate.sh --fast`, `records/<placeholder>.md` and `records/**/*.md`. The quoted sentence gets `authority error = <nil>, want missing paths ["docs/quoted-missing.md"]`. |
| Real cited brief admitted through bounded scanning | none; binary runs | briefs 13 and 14, authority-only with `--root` | exit 0, no output, from both binaries |
| Missing mode precedes prefix lookup | brief.go:109 guard becomes `false && ...` | ORD/mode-before-prefix | `:82: exit 1, stdout "", stderr "brief admission cannot resolve installation prefix: exit status 128\n"` |
| Partial pair precedes prefix lookup | resolver call at the top of parseBriefBounds when either header is present | ORD/partial-before-prefix | `:82`, the same prefix error |
| Malformed pair precedes prefix lookup | resolver call before JSON decoding (after brief.go:165) | ORD/malformed-before-prefix | `:82`, the same prefix error |
| Valid pair performs prefix lookup | resolver returns "" (brief.go:142) | ORD/valid-pair-uses-prefix | `:82: exit 0, stdout "implement\n", stderr ""` |
| Prefix failures use admission terminology | brief.go:144 wording changed back to "brief authority admission" | ORD/valid-pair-uses-prefix | `:82: exit 1, stdout "", stderr "brief authority admission cannot resolve installation prefix: exit status 128\n"` |
| Root-prefix witness depends on stripping | resolver returns "" | ADM/root-prefix | `:50: exit 0, stdout "implement\n", stderr ""` |
| At-root admission does not parse bounds | second parseBriefBounds before brief.go:148 | `./internal/dispatch TestBriefAdmission/parser-ownership` | `brief_bounds_test.go:146: bounds parser ownership changed` |
| Headerless lookup is Git-independent | unconditional resolver call at the top of parseBriefBounds | ADM/headerless-no-prefix | `:52: exit 1, stdout "", stderr "brief admission cannot resolve installation prefix: exit status 128\n"`. The line matches; the cell paraphrases (F-17). |
| Dispatch passes installation root | `--root "$root"` removed from dispatch.sh:863 | ADM/root-prefix | `:55: authority admission does not pass root` |

### Carry-ins (step 3; all hold)

| Rule | Mutation | Test | Observed |
| --- | --- | --- | --- |
| Exactly one Working Mode | brief.go:84 `!= 1` becomes `== 0` | `./internal/dispatch ^TestBriefMode$` | `decisions_test.go:359: double.md: expected a refusal` |
| Mode is not empty | brief.go:84 drops `headers.mode[0] == "" \|\|` | ADM/mode-empty | `:50: exit 0, stdout "\n", stderr ""` |
| Mode is not a placeholder | brief.go:84 drops the `<` guard | `^TestBriefMode$` | `decisions_test.go:359: placeholder.md: expected a refusal` |
| The parsed Mode is returned | brief.go:87 returns `"implement"` | `^TestBriefMode$` | `decisions_test.go:349: BriefMode = "implement", <nil>` |
| BriefModeOnly does not call BriefMode | brief.go:90-94 becomes `return BriefMode(briefPath)` | `TestBriefModeOnly/partial-pair` | `brief_bounds_test.go:102: mode = "", error = BRIEF_BOUNDS_INVALID: Ceiling: required with Boundary` |
| H8.authority-before-mode | brief.go:109 becomes `requireMode && modeErr != nil` | ADM/authority-before-mode | `:50: exit 1, stdout "", stderr ""` |
| H8.authority-before-mode, final mode check | brief.go:121-123 removed | ADM/authority-before-mode-required | `:50: exit 0, stdout "\n", stderr ""` |

The empty-mode guard still has only the CLI witness, as round 1 noted. The fold did not ask for more.

### Five sampled rows (all reproduced)

| Builder row | Mutation | Test | Observed |
| --- | --- | --- | --- |
| Live Boundary is authority output | brief.go:351 drops `strings.HasPrefix(line, "Boundary:") \|\|` | `TestBriefAuthorityBoundaryIsOutput/file` | `brief_authority_test.go:141: ... MissingPaths:[]string{"docs/new.md"}}, want missing paths []` |
| Quoted bounds text is not inert | brief.go:388 trims `" \t>"` | `TestBriefAuthorityIndentedBoundsExampleIsInert/quoted` | `brief_authority_test.go:162: authority error = <nil>, want missing paths ["records/missing.md"]` |
| BriefMode uses paired admission | brief.go:45-46 become `return BriefModeOnly(briefPath)` | `TestBriefAdmission/brief-mode-paired-bounds` | `brief_bounds_test.go:119: error = <nil>, want Ceiling: required with Boundary` |
| ValidateBriefAuthority uses paired admission | brief.go:272-273 read the file and call validateBriefAuthority directly | `TestBriefAdmission/brief-authority-paired-bounds` | `brief_bounds_test.go:123: error = ... "brief authority admission cannot resolve delegate base tree: exit status 128" ..., want Ceiling: required with Boundary` |
| The same admitted bounds reach authority | brief.go:117 passes `BriefBounds{}` | `TestBriefAdmission/exact-bytes` | `brief_bounds_test.go:141: admission = {...}, checked bounds = {Boundary:[] Ceiling:<nil>}, error = <nil>`. The line matches; the cell paraphrases (F-17). |

### Probes beyond the table

| Probe | Expected | Observed |
| --- | --- | --- |
| CLI passes "." instead of `--root` (F-11) | a test fails | all 19 TestDispatchBriefBounds subtests pass |
| Lookup before member validation (F-12) | a test fails | 18 cmd and 72 internal subtests pass |
| Lookup before the Ceiling check (F-12) | a test fails | 18 cmd and 72 internal subtests pass |
| Round 1's eager lookup restored | fails | mode-, partial- and malformed-before-prefix fail at :82 |
| Second parse through ParseBriefBounds in ReadBriefAdmissionAtRoot | fails | parser-ownership fails at brief_bounds_test.go:146 |
| Ordering test with GIT_DIR exported, or TMPDIR inside Git (F-16) | passes | valid-pair-uses-prefix fails at :82 |

### Verification on the copy (step 6)

Log: `<S>/u1bi-chain.log`, run after all mutations and after the hash check.

| Command | Result |
| --- | --- |
| `go test -race -count=1 -timeout 40m ./internal/dispatch/` | `ok github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch 97.335s` |
| `go test -race -count=1 -timeout 40m ./cmd/metasystem -run 'TestDispatchBriefBounds\|TestBriefMode\|TestDispatchBriefMode' -v` | `ok ... 1.697s`. All 12 admission subtests, the 4 ordering subtests and TestDispatchBriefBoundsFailureSuffix pass. The other two name patterns match no test in this package. |
| `gofmt -l internal/dispatch cmd/metasystem` | no output |
| `go vet ./internal/dispatch ./cmd/metasystem` | exit 0 |
| `bash -n scripts/agents/dispatch.sh` | exit 0. `/bin/bash -n` (3.2) also exits 0. |

## 5. Verdict

Send back with 2 material findings, both a few test lines in cmd/metasystem/dispatch_brief_bounds_test.go: (1) F-11, keep the unprefixed root-prefix case and add a prefixed-member case (`metasystem/internal/dispatch/brief.go`) that expects exit 0 and `implement`, so a CLI that ignores `--root` fails; (2) F-12, add a malformed-Ceiling case and an invalid-member case to TestDispatchBriefBoundsPrefixLookupOrdering, so a lookup before brief.go:170 or brief.go:180 fails.
