VERDICT: send back

### F-41. Severity low. Material yes.
Claim: the fold deleted the named witnesses of page rows A4.pattern-no-equality, A4.backslash-no-equality and A4.directory-no-equality, which this unit's brief requires by name and the fold brief did not remove, and result-2 does not say so, so those witness names now pass without running a test.

Evidence:
- Round 1 against round 2:
  - The round-1 copy `/tmp/opus-u1bii-copy` has brief_authority_test.go sha256 39537be0..., as read 1 recorded. It holds the rows `pattern-no-equality`, `backslash-no-equality` and `directory-no-equality` at lines 240-242 of TestBriefAuthorityBoundaryInputStillRequired.
  - The live file (c57b399d...) has none of them. The test now starts at brief_authority_test.go:261 with five rows.
- Brief requirements:
  - The unit brief, artifacts/reports/codex-bdrb-u1bii-brief.md:23, says "Build row A4 (`TestBriefAuthorityBoundaryInputStillRequired`: ordinary, root-file, new-directory, pattern-no-equality, backslash-no-equality, directory-no-equality, output-then-input, input-then-output)". Its :29, on the Ceiling, says "do not trim a witness".
  - The fold brief says "Fold exactly" and lists no removal.
  - Result-2's list of what "the tests now cover" does not mention one.
- The page's verification table names these subtests as the rows' witnesses (plans/brief-declares-the-round-boundary-design.md:568-570). BDRB-R2-02 (:855) says "A4.pattern-no-equality and A4.backslash-no-equality pin authority".
- Running the names on my candidate copy:
  - `go test -count=1 -timeout 40m ./internal/dispatch -run '^TestBriefAuthorityBoundaryInputStillRequired$/^pattern-no-equality$' -v` prints `--- PASS: TestBriefAuthorityBoundaryInputStillRequired (0.00s)`, `testing: warning: no tests to run` and `ok ... [no tests to run]`, and exits 0.
  - The backslash-no-equality and directory-no-equality names give the same output.
  - So a proof run by the page's witness names passes without running anything.
- Ruling A4 did not force the removal. In `/tmp/opus-u1bii3-witness`, a clone of my copy, I put the three round-1 rows back unchanged:
  - on the folded brief.go, all eight subtests pass;
  - under A4.glob-treated-concrete, only pattern-no-equality fails: `brief_authority_test.go:281: authority error = &dispatch.BriefAuthorityRefusal{MissingPaths:[]string{"new/*.md"}}, want missing paths []`;
  - under A4.backslash-treated-concrete, only backslash-no-equality fails, naming `new/a\\b.md`;
  - under A4.directory-treated-concrete, only directory-no-equality fails, naming `new/file.md/`;
  - brief.go was restored to 2ca4d47d... after each run.
- No rule lost its only failing test: each A4 kind mutant also fails a TestBriefAuthorityNonConcreteBoundaryMembersUseTrunkScanner subtest (mutation table). What is damaged is the brief-required, page-named witness set, and the return that should have declared the change.
- Artifact to change: brief_authority_test.go, restoring the three rows in TestBriefAuthorityBoundaryInputStillRequired; they pass, as shown. If the seat instead accepts the new test as their witness, the page's rows 568-570 need an amendment that names it.

### F-42. Severity low. Material no.
Claim: the A4 proof is at the kind level the fold brief asked for, so a mutant that drops only `?`, only `[` and `]`, only `$` and `{`, or only the single-line form of the path:line suffix passes every authority test.

Evidence:
- Mutation rows EXTRA.qmark-treated-concrete, EXTRA.bracket-treated-concrete, EXTRA.dollar-brace-treated-concrete and EXTRA.single-line-suffix-treated-concrete each exit 0 under `go test -count=1 -timeout 40m ./internal/dispatch -run '^TestBriefAuthority' -v`.
- The new test uses one spelling per kind: `*` for globs (brief_authority_test.go:235-237 and :240), `<goal>` for the placeholder (:238), and `:47-53` for path:line (:239).
- Not material:
  - the fold brief asks for "one mutant per non-concrete kind", and each of the five kind mutants fails a named case;
  - the engine behaves correctly for the untested spellings: N01 (`?`), N02 (`[s]`), N03 (`$`), N04 (`{}`), N05 (`>` alone), N06 and N07 (single `:N`) all give base's result.
- If the seat wants one witness per byte, that means one row per spelling in TestBriefAuthorityNonConcreteBoundaryMembersUseTrunkScanner.

### F-43. Severity low. Material no.
Claim: the design page still describes the backslash behaviour from before ruling A4, so row A3.backslash and unit 4a's end-to-end paragraph now contradict what this unit lands.

Evidence:
- plans/brief-declares-the-round-boundary-design.md:559, row A3.backslash, says "Preserve a literal backslash under an eligible existing directory". Its witness is `TestBriefAuthoritySpecialCharacterInput/backslash`.
  - The fold removed that subtest, as its brief instructed.
  - The name now runs nothing, with the same `[no tests to run]` output as in F-41.
- :85 says "A special-byte input under an eligible existing directory is still checked by its full decoded identity."
- :89, unit 4a's TestBriefBackslashEndToEnd, says "Cite the same JSON string on an ordinary input line. With internal present in the committed tree, authority refuses naming the full backslash path".
- Under ruling A4, both engines name trunk's fragment. S35 and N09 give `BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/a`.
- Not material to this unit: the fold brief says ruling A4 wins over the page and tells the builder to change that case, and result-2 says so. Artifact: the design page, amended before unit 4a is briefed.

### F-44. Severity low. Material no.
Claim: the eligibility operand on the citation path is dead code, because every citation already equals a concrete member.

Evidence:
- brief.go:366 passes `concreteMembers` to briefBoundaryCitations. That function appends a citation only when `members[value]` holds (brief.go:428 and :437).
- So at brief.go:377, `concreteMembers[citation.path]` is always true, and `briefAuthorityPathEligible(citation.path, ...)` never decides. The example guard at :378 is the only live condition.
- Not material: no behaviour or proof depends on it.

## Probe comparison

`<S>` is `/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/36c93128-ec16-4146-8def-3f706ba4ef10/scratchpad`.

Engines:
- base: `/tmp/opus-u1bii3-bin/ms-base`, built from `/tmp/opus-u1bii-base` (fad3ace8);
- candidate: `/tmp/opus-u1bii3-bin/ms-cand`, built from `/tmp/opus-u1bii3-copy`.

Driver and logs:
- The driver is `python3 <S>/u1bii3-probe.py <S>`.
- It wrote 754 rows to `<S>/u1bii3-probe.json` and the briefs to `<S>/u1bii3-probe-briefs/`.
- `<S>/u1bii3-tables.py` generates these tables from that log.

Every synthetic brief ran in five modes:
- normal (`--brief B --root R`);
- combined (adds `--base-tree` and `--disk-root`);
- authority (the dispatch.sh:863 shape);
- mode-only (the dispatch.sh:859 shape);
- non-Git (`--brief B`, run from a directory outside Git).

R is `/tmp/opus-u1bii-base/metasystem`, with the base copy as base tree and disk root.

"The same way" compares exit status and stderr, because combined mode also prints the mode on stdout when it admits. N01 to N15 are further A4 probes this read added. S09pa to S09pg run the F-1 lines with concrete members that are prefixes of the spans.

| Brief | Boundary | Text lines | Base, authority mode | Candidate, authority mode | Other modes | Differs | A1 and A4 allow |
| --- | --- | --- | --- | --- | --- | --- | --- |
| S01-f1-pathline-exists | `` [] `` | See `metasystem/internal/dispatch/brief.go:47-53`. | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S02-f1-pathline-missing | `` [] `` | Read `metasystem/internal/landing/u1bii-missing.go:426`. | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/landing/u1bii-missing.go | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/landing/u1bii-missing.go | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S03-f1-command-exists | `` [] `` | Run `metasystem/scripts/agents/go-gate.sh --fast`. | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S04-f1-command-missing | `` [] `` | Run `metasystem/scripts/agents/u1bii-missing.sh --fast`. | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/scripts/agents/u1bii-missing.sh | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/scripts/agents/u1bii-missing.sh | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S05-f1-placeholder | `` [] `` | Write `metasystem/records/misc/<goal>-code-read.md`. | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S06-f1-glob | `` [] `` | Edit `metasystem/scripts/agents/*.sh` only. | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S07-f1-quoted-sentence-missing | `` [] `` | Say "read metasystem/internal/u1bii-missing.md first". | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/u1bii-missing.md | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/u1bii-missing.md | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S08a-f1-nonempty-boundary | `` ["metasystem/internal/dispatch/brief.go", "metasystem/internal/dispatch/brief_authority_test.go"] `` | See `metasystem/internal/dispatch/brief.go:47-53`. | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S08b-f1-nonempty-boundary | `` ["metasystem/internal/dispatch/brief.go", "metasystem/internal/dispatch/brief_authority_test.go"] `` | Read `metasystem/internal/landing/u1bii-missing.go:426`. | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/landing/u1bii-missing.go | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/landing/u1bii-missing.go | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S08c-f1-nonempty-boundary | `` ["metasystem/internal/dispatch/brief.go", "metasystem/internal/dispatch/brief_authority_test.go"] `` | Run `metasystem/scripts/agents/go-gate.sh --fast`. | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S08d-f1-nonempty-boundary | `` ["metasystem/internal/dispatch/brief.go", "metasystem/internal/dispatch/brief_authority_test.go"] `` | Run `metasystem/scripts/agents/u1bii-missing.sh --fast`. | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/scripts/agents/u1bii-missing.sh | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/scripts/agents/u1bii-missing.sh | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S08e-f1-nonempty-boundary | `` ["metasystem/internal/dispatch/brief.go", "metasystem/internal/dispatch/brief_authority_test.go"] `` | Write `metasystem/records/misc/<goal>-code-read.md`. | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S08f-f1-nonempty-boundary | `` ["metasystem/internal/dispatch/brief.go", "metasystem/internal/dispatch/brief_authority_test.go"] `` | Edit `metasystem/scripts/agents/*.sh` only. | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S08g-f1-nonempty-boundary | `` ["metasystem/internal/dispatch/brief.go", "metasystem/internal/dispatch/brief_authority_test.go"] `` | Say "read metasystem/internal/u1bii-missing.md first". | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/u1bii-missing.md | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/u1bii-missing.md | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S09-f1-all-nonempty-boundary | `` ["metasystem/internal/dispatch/brief.go", "metasystem/internal/dispatch/brief_authority_test.go"] `` | See `metasystem/internal/dispatch/brief.go:47-53`. / Read `metasystem/internal/landing/u1bii-missing.go:426`. / Run `metasystem/scripts/agents/go-gate.sh --fast`. / Run `metasystem/scripts/agents/u1bii-missing.sh --fast`. / Write `metasystem/records/misc/<goal>-code-read.md`. / Edit `metasystem/scripts/agents/*.sh` only. / Say "read metasystem/internal/u1bii-missing.md first". | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/landing/u1bii-missing.go, metasystem/internal/u1bii-missing.md, metasystem/scripts/agents/u1bii-missing.sh | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/landing/u1bii-missing.go, metasystem/internal/u1bii-missing.md, metasystem/scripts/agents/u1bii-missing.sh | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S09pa-f1-prefix-members | `` ["metasystem/scripts/agents/go-gate.sh", "metasystem/internal/landing/u1bii-missing.go", "metasystem/scripts/agents/u1bii-missing.sh", "metasystem/internal/u1bii-missing.md", "metasystem/scripts/agents/*.sh", "metasystem/records/misc/", "metasystem/internal/dispatch/brief.go"] `` | See `metasystem/internal/dispatch/brief.go:47-53`. | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S09pb-f1-prefix-members | `` ["metasystem/scripts/agents/go-gate.sh", "metasystem/internal/landing/u1bii-missing.go", "metasystem/scripts/agents/u1bii-missing.sh", "metasystem/internal/u1bii-missing.md", "metasystem/scripts/agents/*.sh", "metasystem/records/misc/", "metasystem/internal/dispatch/brief.go"] `` | Read `metasystem/internal/landing/u1bii-missing.go:426`. | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/landing/u1bii-missing.go | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/landing/u1bii-missing.go | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S09pc-f1-prefix-members | `` ["metasystem/scripts/agents/go-gate.sh", "metasystem/internal/landing/u1bii-missing.go", "metasystem/scripts/agents/u1bii-missing.sh", "metasystem/internal/u1bii-missing.md", "metasystem/scripts/agents/*.sh", "metasystem/records/misc/", "metasystem/internal/dispatch/brief.go"] `` | Run `metasystem/scripts/agents/go-gate.sh --fast`. | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S09pd-f1-prefix-members | `` ["metasystem/scripts/agents/go-gate.sh", "metasystem/internal/landing/u1bii-missing.go", "metasystem/scripts/agents/u1bii-missing.sh", "metasystem/internal/u1bii-missing.md", "metasystem/scripts/agents/*.sh", "metasystem/records/misc/", "metasystem/internal/dispatch/brief.go"] `` | Run `metasystem/scripts/agents/u1bii-missing.sh --fast`. | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/scripts/agents/u1bii-missing.sh | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/scripts/agents/u1bii-missing.sh | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S09pe-f1-prefix-members | `` ["metasystem/scripts/agents/go-gate.sh", "metasystem/internal/landing/u1bii-missing.go", "metasystem/scripts/agents/u1bii-missing.sh", "metasystem/internal/u1bii-missing.md", "metasystem/scripts/agents/*.sh", "metasystem/records/misc/", "metasystem/internal/dispatch/brief.go"] `` | Write `metasystem/records/misc/<goal>-code-read.md`. | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S09pf-f1-prefix-members | `` ["metasystem/scripts/agents/go-gate.sh", "metasystem/internal/landing/u1bii-missing.go", "metasystem/scripts/agents/u1bii-missing.sh", "metasystem/internal/u1bii-missing.md", "metasystem/scripts/agents/*.sh", "metasystem/records/misc/", "metasystem/internal/dispatch/brief.go"] `` | Edit `metasystem/scripts/agents/*.sh` only. | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S09pg-f1-prefix-members | `` ["metasystem/scripts/agents/go-gate.sh", "metasystem/internal/landing/u1bii-missing.go", "metasystem/scripts/agents/u1bii-missing.sh", "metasystem/internal/u1bii-missing.md", "metasystem/scripts/agents/*.sh", "metasystem/records/misc/", "metasystem/internal/dispatch/brief.go"] `` | Say "read metasystem/internal/u1bii-missing.md first". | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/u1bii-missing.md | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/u1bii-missing.md | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S10-f1-all-headerless | `` (none) `` | See `metasystem/internal/dispatch/brief.go:47-53`. / Read `metasystem/internal/landing/u1bii-missing.go:426`. / Run `metasystem/scripts/agents/go-gate.sh --fast`. / Run `metasystem/scripts/agents/u1bii-missing.sh --fast`. / Write `metasystem/records/misc/<goal>-code-read.md`. / Edit `metasystem/scripts/agents/*.sh` only. / Say "read metasystem/internal/u1bii-missing.md first". | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/landing/u1bii-missing.go, metasystem/internal/u1bii-missing.md, metasystem/scripts/agents/u1bii-missing.sh | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/landing/u1bii-missing.go, metasystem/internal/u1bii-missing.md, metasystem/scripts/agents/u1bii-missing.sh | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S11-member-glob-backtick | `` ["metasystem/scripts/agents/*.sh"] `` | Edit `metasystem/scripts/agents/*.sh` only. | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow (task: must match base) |
| S12-member-glob-json | `` ["metasystem/scripts/agents/*.sh"] `` | Edit "metasystem/scripts/agents/*.sh" only. | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow (task: must match base) |
| S13-member-placeholder | `` ["metasystem/records/misc/<goal>-code-read.md"] `` | Write `metasystem/records/misc/<goal>-code-read.md`. | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow (task: must match base) |
| S14-member-new-directory | `` ["metasystem/internal/u1biipkg/"] `` | Put the package in `metasystem/internal/u1biipkg/`. | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow (task: must match base) |
| S15-member-existing-directory | `` ["metasystem/internal/dispatch/"] `` | Work inside `metasystem/internal/dispatch/`. | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S16-member-existing-file | `` ["metasystem/internal/dispatch/brief.go"] `` | Read `metasystem/internal/dispatch/brief.go`. | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S17-member-new-file-eligible-dir | `` ["metasystem/internal/dispatch/u1bii-new.go"] `` | Read `metasystem/internal/dispatch/u1bii-new.go`. | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/dispatch/u1bii-new.go | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/dispatch/u1bii-new.go | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S18-member-space-backtick | `` ["metasystem/docs/a b.md"] `` | Read `metasystem/docs/a b.md`. | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a b.md | differs in combined the same way | yes | yes: concrete member with a space (A1, A4 input-wins) |
| S19-member-space-json | `` ["metasystem/docs/a b.md"] `` | Read "metasystem/docs/a b.md". | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a b.md | differs in combined the same way | yes | yes: concrete member with a space (A1, A4 input-wins) |
| S20-member-install-root-file | `` ["metasystem/NOTICE-u1bii.md"] `` | Read `metasystem/NOTICE-u1bii.md`. | exit 0 | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/NOTICE-u1bii.md | differs in combined the same way | yes | yes: row A4.root-file (concrete-member eligibility) |
| S21-member-pattern-absent-subdir | `` ["metasystem/internal/u1biipkg/*.go"] `` | Add files matching `metasystem/internal/u1biipkg/*.go`. | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow (task: must match base) |
| S22-member-glob-workspace | `` ["metasystem/scripts/agents/*.sh"] `` | # Workspace / May-write: `metasystem/scripts/agents/*.sh`. | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S23-member-glob-create | `` ["metasystem/scripts/agents/*.sh"] `` | Create `metasystem/scripts/agents/*.sh`. | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S24-member-diffboundary-example | `` ["metasystem/internal/dispatch/u1bii-new.go"] `` | diffBoundary example: ["metasystem/internal/dispatch/u1bii-new.go"] | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S25-member-pathline | `` ["metasystem/internal/dispatch/brief.go:47-53"] `` | See `metasystem/internal/dispatch/brief.go:47-53`. | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow (task: must match base) |
| S26-f4-unterminated-backtick-member | `` ["metasystem/docs/a b.md"] `` | Read `metasystem/docs/a b.md | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S26e-f4-unterminated-backtick-empty | `` [] `` | Read `metasystem/docs/a b.md | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S27-f4-unterminated-json-member | `` ["metasystem/docs/a b.md"] `` | Read "metasystem/docs/a b.md | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S27e-f4-unterminated-json-empty | `` [] `` | Read "metasystem/docs/a b.md | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S28-f4-invalid-json-member | `` ["metasystem/docs/a\\qb.md"] `` | Read "metasystem/docs/a\qb.md" | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S28e-f4-invalid-json-empty | `` [] `` | Read "metasystem/docs/a\qb.md" | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S29-quote-desync-member-json | `` ["metasystem/docs/a b.md"] `` | The 5" drive: read "metasystem/docs/a b.md". | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S30-member-backtick-inside-quote | `` ["metasystem/docs/a b.md"] `` | Say "see `metasystem/docs/a b.md` now". | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S31-pattern-member-hides-missing | `` ["metasystem/u1biix/* metasystem/internal/u1bii-missing.md"] `` | Edit `metasystem/u1biix/* metasystem/internal/u1bii-missing.md`. | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/u1bii-missing.md | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/u1bii-missing.md | same in normal, combined, mode-only, non-Git | no | nothing to allow (task: must match base) |
| S32-member-and-missing-same-line | `` ["metasystem/docs/a b.md"] `` | Read `metasystem/docs/a b.md` and metasystem/internal/u1bii-missing.md. | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a, metasystem/internal/u1bii-missing.md | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a b.md, metasystem/internal/u1bii-missing.md | differs in combined the same way | yes | yes: concrete member identity; rest of line as trunk |
| S33-headerless-member-shape | `` (none) `` | Read `metasystem/docs/a b.md`. | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S34-json-in-backticks-padded | `` ["metasystem/docs/a b.md"] `` | Read ` "metasystem/docs/a b.md" `. | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a b.md | differs in combined the same way | yes | yes by A4 (concrete member); F-35 of read 1 notes page wording |
| S35-member-backslash-json | `` ["metasystem/internal/a\\b.txt"] `` | Read "metasystem/internal/a\\b.txt". | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/a | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/a | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S40-mode-only-no-bounds | `` (none) `` | Read metasystem/internal/u1bii-missing.md. | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/u1bii-missing.md | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/u1bii-missing.md | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S41-no-mode-valid-pair | `` [] `` |  | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S42-partial-boundary | `` [] `` |  | exit 1, BRIEF_BOUNDS_INVALID: Ceiling: required with Boundary | exit 1, BRIEF_BOUNDS_INVALID: Ceiling: required with Boundary | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S43-partial-ceiling | `` (none) `` |  | exit 1, BRIEF_BOUNDS_INVALID: Boundary: required with Ceiling | exit 1, BRIEF_BOUNDS_INVALID: Boundary: required with Ceiling | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S44-bad-boundary-json | `` bad `` |  | exit 1, BRIEF_BOUNDS_INVALID: Boundary: expected a JSON array of paths | exit 1, BRIEF_BOUNDS_INVALID: Boundary: expected a JSON array of paths | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S45-bad-ceiling | `` [] `` |  | exit 1, BRIEF_BOUNDS_INVALID: Ceiling: expected a nonnegative decimal integer no greater than 9223372036854775807 | exit 1, BRIEF_BOUNDS_INVALID: Ceiling: expected a nonnegative decimal integer no greater than 9223372036854775807 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S46-unprefixed-member | `` ["internal/dispatch/brief.go"] `` |  | exit 1, BRIEF_BOUNDS_INVALID: Boundary: invalid path or pattern "internal/dispatch/brief.go" | exit 1, BRIEF_BOUNDS_INVALID: Boundary: invalid path or pattern "internal/dispatch/brief.go" | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S47-duplicate-boundary | `` [] `` |  | exit 1, BRIEF_BOUNDS_INVALID: Boundary: header occurs more than once | exit 1, BRIEF_BOUNDS_INVALID: Boundary: header occurs more than once | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S48-unprefixed-and-bad-ceiling | `` ["internal/a"] `` |  | exit 1, BRIEF_BOUNDS_INVALID: Ceiling: expected a nonnegative decimal integer no greater than 9223372036854775807 | exit 1, BRIEF_BOUNDS_INVALID: Ceiling: expected a nonnegative decimal integer no greater than 9223372036854775807 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S49-no-mode-partial-missing | `` [] `` | Read metasystem/internal/u1bii-missing.md. | exit 1, BRIEF_BOUNDS_INVALID: Ceiling: required with Boundary | exit 1, BRIEF_BOUNDS_INVALID: Ceiling: required with Boundary | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| N01-qmark-glob | `` ["metasystem/scripts/agents/go-gate.?h"] `` | Edit `metasystem/scripts/agents/go-gate.?h`. | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/scripts/agents/go-gate | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/scripts/agents/go-gate | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| N02-bracket-glob | `` ["metasystem/scripts/agents/go-gate.[s]h"] `` | Edit `metasystem/scripts/agents/go-gate.[s]h`. | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/scripts/agents/go-gate | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/scripts/agents/go-gate | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| N03-dollar-placeholder | `` ["metasystem/records/misc/$goal-read.md"] `` | Write `metasystem/records/misc/$goal-read.md`. | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| N04-brace-placeholder | `` ["metasystem/records/misc/{goal}-read.md"] `` | Write `metasystem/records/misc/{goal}-read.md`. | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| N05-gt-only-json | `` ["metasystem/records/misc/goal>-read.md"] `` | Write "metasystem/records/misc/goal>-read.md". | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| N06-pathline-single-exists | `` ["metasystem/internal/dispatch/brief.go:47"] `` | See `metasystem/internal/dispatch/brief.go:47`. | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| N07-pathline-single-missing | `` ["metasystem/internal/dispatch/u1bii-missing.go:12"] `` | See `metasystem/internal/dispatch/u1bii-missing.go:12`. | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/dispatch/u1bii-missing.go | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/dispatch/u1bii-missing.go | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| N08-directory-json | `` ["metasystem/internal/u1biipkg/"] `` | Put it in "metasystem/internal/u1biipkg/". | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| N09-backslash-backtick | `` ["metasystem/internal/a\\b.txt"] `` | Read `metasystem/internal/a\b.txt`. | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/a | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/a | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| N10-existing-directory-json | `` ["metasystem/internal/dispatch/"] `` | Work in "metasystem/internal/dispatch/". | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| N11-glob-with-space-json | `` ["metasystem/docs/a b*.md"] `` | Read "metasystem/docs/a b*.md". | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| N12-pattern-directory | `` ["metasystem/internal/u1bii*/"] `` | Work in `metasystem/internal/u1bii*/`. | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| N13-pathline-missing-range-json | `` ["metasystem/internal/landing/u1bii-missing.go:426-430"] `` | Read "metasystem/internal/landing/u1bii-missing.go:426-430". | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/landing/u1bii-missing.go | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/landing/u1bii-missing.go | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| N14-mixed-nonconcrete-and-special | `` ["metasystem/scripts/agents/*.sh", "metasystem/internal/u1biipkg/", "metasystem/docs/u1bii3-missing space.md"] `` | Edit `metasystem/scripts/agents/*.sh` in `metasystem/internal/u1biipkg/`. / Read "metasystem/docs/u1bii3-missing space.md". | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/u1bii3-missing | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/u1bii3-missing space.md | differs in combined the same way | yes | yes: glob and directory members give trunk's result; the concrete space member keeps its identity |
| N15-directory-on-input-and-workspace | `` ["metasystem/internal/u1biipkg/"] `` | # Workspace / May-write: `metasystem/internal/u1biipkg/`. / # Inputs / Read `metasystem/internal/u1biipkg/`. | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |

Concrete special-character members, run against `/tmp/opus-u1bii3-special`: trunk fad3ace8 plus one commit adding `metasystem/docs/u1bii3<X>` for X in space, comma, quote, tab, newline and plus. `<E>` is the committed member, `<M>` the absent `metasystem/docs/missing-u1bii3<X>`, and `<O>` the unrelated member `metasystem/docs/u1bii3-other.md`. Citations are JSON strings unless the row says backticks; the newline backtick row puts the JSON string inside backticks. Normal, combined and authority modes ran. Each row held for all six characters.

| Variant | Boundary | Text lines | Base, authority mode (all six) | Candidate, authority mode (all six) | Combined and normal | Differs | A1 and A4 allow |
| --- | --- | --- | --- | --- | --- | --- | --- |
| json-exists-input | `` [<E>] `` | Read <E>. | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/u1bii3 | exit 0 | differs in combined the same way; normal same | yes | yes: concrete special-character member keeps its full identity and the input-wins rule |
| backtick-exists-input | `` [<E>] `` | Read `<E>`. | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/u1bii3 | exit 0 | differs in combined the same way; normal same | yes | yes: concrete special-character member keeps its full identity and the input-wins rule |
| json-exists-output-then-input | `` [<E>] `` | # Workspace / May-write: <E>. / # Inputs / Read <E>. | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/u1bii3 | exit 0 | differs in combined the same way; normal same | yes | yes: concrete special-character member keeps its full identity and the input-wins rule |
| json-missing-input | `` [<M>] `` | Read <M>. | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/missing-u1bii3 | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: <M> | differs in combined the same way; normal same | yes | yes: concrete special-character member keeps its full identity and the input-wins rule |
| backtick-missing-input | `` [<M>] `` | Read `<M>`. | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/missing-u1bii3 | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: <M> | differs in combined the same way; normal same | yes | yes: concrete special-character member keeps its full identity and the input-wins rule |
| json-missing-output | `` [<M>] `` | # Workspace / May-write: <M>. | exit 0 | exit 0 | same in normal and combined | no | nothing to allow |
| json-missing-create | `` [<M>] `` | Create <M>. | exit 0 | exit 0 | same in normal and combined | no | nothing to allow |
| json-missing-output-then-input | `` [<M>] `` | # Workspace / May-write: <M>. / # Inputs / Read <M>. | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/missing-u1bii3 | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: <M> | differs in combined the same way; normal same | yes | yes: concrete special-character member keeps its full identity and the input-wins rule |
| json-missing-input-then-output | `` [<M>] `` | Read <M>. / # Workspace / May-write: <M>. | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/missing-u1bii3 | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: <M> | differs in combined the same way; normal same | yes | yes: concrete special-character member keeps its full identity and the input-wins rule |
| nonmember-nonempty | `` [<O>] `` | Read <M>. | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/missing-u1bii3 | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/missing-u1bii3 | same in normal and combined | no | nothing to allow |

Real briefs from the main checkout, run against it as dispatch.sh on main would (`--root <main>/metasystem --base-tree <main> --disk-root <main>`), combined and authority modes:

| Brief set | Variant | Admitted (base = candidate) | Refused (base = candidate) | Rows differing | A1 and A4 allow |
| --- | --- | --- | --- | --- | --- |
| 32 artifacts/agents/*/brief.md | unmodified | 25 | 7 | 0 | nothing to allow |
| 32 artifacts/agents/*/brief.md | `Boundary: []`, `Ceiling: 1` after Working Mode | 25 | 7 | 0 | nothing to allow |
| 32 artifacts/agents/*/brief.md | Boundary = every metasystem/-prefixed backtick or JSON span in the brief (102 members: 33 directories, 2 path:line, 0 globs), Ceiling 400 | 25 | 7 | 0 | nothing to allow |
| implementer-b76a0ddc11f73e99fa195d84 | the 1b-i read's Boundary (`metasystem/internal/landing/receipt.go`, `metasystem/internal/landing/testing.go`), Ceiling 400 | 1 | 0 | 0 | nothing to allow |

## Mutation table

Method:
- The driver is `python3 <S>/u1bii3-mutate.py <S>`, run on `/tmp/opus-u1bii3-mut/metasystem`.
- Each mutation was applied alone, then `go test -count=1 -timeout 40m ./internal/dispatch -run '^TestBriefAuthority' -v` ran, then brief.go was restored from the pristine copy.
- Every record in `<S>/u1bii3-mut.jsonl` says `restored: true`, and the final sha256 is 2ca4d47d..., the live candidate's. Logs are in `<S>/u1bii3-mut-logs/`.
- The "Tests that failed" column lists leaf subtests only.

Rows:
- R3, X and A4 are this read's required checks.
- R1 re-runs round 1's rules.
- EXTRA are sub-kind mutants I added for F-42.

| Mutation | Proves | Location and change | Tests that failed | Observed failure line |
| --- | --- | --- | --- | --- |
| baseline-unmutated | all pass | none (control) | none, exit 0 (all TestBriefAuthority tests pass) | no failure line |
| R3.gate-any-nonempty-boundary | F-32 | brief.go:428 `if members[value] {` becomes `if len(members) > 0 {`; brief.go:437 `if valid && members[value] {` becomes `if valid && len(members) > 0 {` | TestBriefAuthorityBoundedUsesTrunkTokenScanner/backticked-path-line/nonempty-boundary; TestBriefAuthorityBoundedUsesTrunkTokenScanner/command-with-arguments/nonempty-boundary; TestBriefAuthorityBoundedUsesTrunkTokenScanner/placeholder/nonempty-boundary; TestBriefAuthorityBoundedUsesTrunkTokenScanner/glob/nonempty-boundary; TestBriefAuthorityBoundedUsesTrunkTokenScanner/quoted-sentence/nonempty-boundary; TestBriefAuthoritySpecialCharacterInput/space; TestBriefAuthoritySpecialCharacterInput/comma; TestBriefAuthoritySpecialCharacterInput/quote; TestBriefAuthoritySpecialCharacterInput/tab; TestBriefAuthoritySpecialCharacterInput/newline; TestBriefAuthoritySpecialCharacterInput/json-in-backticks; TestBriefAuthoritySpecialCharacterInput/literal-backticks; TestBriefAuthoritySpecialCharacterInput/consumed-span; TestBriefAuthorityRealBriefRegression/nonempty-boundary | `brief_authority_test.go:187: authority error = &dispatch.BriefAuthorityRefusal{MissingPaths:[]string{"docs/missing.md:426"}}, want missing paths ["docs/missing.md"]` |
| R3.backtick-gate-member-prefix | F-32 | brief.go:428 consumes a backtick span that starts with any concrete member | TestBriefAuthorityBoundedUsesTrunkTokenScanner/backticked-path-line/nonempty-boundary; TestBriefAuthorityRealBriefRegression/nonempty-boundary | `brief_authority_test.go:187: authority error = &dispatch.BriefAuthorityRefusal{MissingPaths:[]string{"docs/missing.md:426"}}, want missing paths ["docs/missing.md"]` |
| X.citation-diffboundary-example-guard-removed | F-33 | brief.go:378 `!(boundaryExample && strings.HasPrefix(citation.path, "metasystem/")) {` becomes `true {` | TestBriefAuthorityTreatsMetasystemDiffBoundaryExampleAsPrefixOnly/bounded-member | `brief_authority_test.go:97: repository-relative diffBoundary example was treated as cited authority: BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/not-an-authority.go` |
| A4.directory-treated-concrete | A4 kind: directory | brief.go:407 `!strings.HasSuffix(member, "/")` becomes `true` | TestBriefAuthorityNonConcreteBoundaryMembersUseTrunkScanner/directory | `brief_authority_test.go:244: authority error = &dispatch.BriefAuthorityRefusal{MissingPaths:[]string{"metasystem/internal/u1biipkg/"}}, want missing paths []` |
| A4.glob-treated-concrete | A4 kind: glob | brief.go:408 `!briefBoundaryIsPattern(member)` becomes `!strings.ContainsAny(member, "?[]\\")` (drops `*`) | TestBriefAuthorityNonConcreteBoundaryMembersUseTrunkScanner/glob-backtick; TestBriefAuthorityNonConcreteBoundaryMembersUseTrunkScanner/glob-json; TestBriefAuthorityNonConcreteBoundaryMembersUseTrunkScanner/glob-absent-directory; TestBriefAuthorityNonConcreteBoundaryMembersUseTrunkScanner/pattern-hides-missing | `brief_authority_test.go:244: authority error = &dispatch.BriefAuthorityRefusal{MissingPaths:[]string{"metasystem/scripts/agents/*.sh"}}, want missing paths []` |
| A4.placeholder-treated-concrete | A4 kind: placeholder | brief.go:409 `!strings.ContainsAny(member, "<>${")` becomes `true` | TestBriefAuthorityNonConcreteBoundaryMembersUseTrunkScanner/placeholder | `brief_authority_test.go:244: authority error = &dispatch.BriefAuthorityRefusal{MissingPaths:[]string{"metasystem/records/misc/<goal>-code-read.md"}}, want missing paths []` |
| A4.path-line-treated-concrete | A4 kind: path:line | brief.go:410 `!briefPathLine.MatchString(member)` becomes `true` | TestBriefAuthorityNonConcreteBoundaryMembersUseTrunkScanner/path-line | `brief_authority_test.go:244: authority error = &dispatch.BriefAuthorityRefusal{MissingPaths:[]string{"metasystem/internal/dispatch/brief.go:47-53"}}, want missing paths []` |
| A4.backslash-treated-concrete | A4 kind: backslash | brief.go:408 `!briefBoundaryIsPattern(member)` becomes `!strings.ContainsAny(member, "*?[]")` (drops backslash) | TestBriefAuthorityNonConcreteBoundaryMembersUseTrunkScanner/backslash | `brief_authority_test.go:244: authority error = &dispatch.BriefAuthorityRefusal{MissingPaths:[]string{"metasystem/internal/a\\b.txt"}}, want missing paths ["metasystem/internal/a"]` |
| R1.rule1-backtick-recognition-disabled | round 1 rule 1 | brief.go backtick case `` case '`': `` becomes `case 0:` | TestBriefAuthorityBacktickBoundaryIdentity; TestBriefAuthoritySpecialCharacterInput/literal-backticks; TestBriefAuthoritySpecialCharacterInput/consumed-span; TestBriefAuthorityBoundaryInputStillRequired/root-file; TestBriefAuthorityBoundaryInputStillRequired/new-directory; TestBriefAuthorityBoundaryInputStillRequired/output-then-input; TestBriefAuthorityBoundaryInputStillRequired/input-then-output | `brief_authority_test.go:196: authority error = &dispatch.BriefAuthorityRefusal{MissingPaths:[]string{"docs/a"}}, want missing paths ["docs/a b.md"]` |
| R1.rule2-json-recognition-disabled | round 1 rule 2 | brief.go `case '"':` becomes `case 1:` | TestBriefAuthorityJSONStringBoundaryIdentity; TestBriefAuthoritySpecialCharacterInput/space; TestBriefAuthoritySpecialCharacterInput/comma; TestBriefAuthoritySpecialCharacterInput/quote; TestBriefAuthoritySpecialCharacterInput/tab; TestBriefAuthoritySpecialCharacterInput/newline; TestBriefAuthorityBoundedCitationCompatibility/artifact | `brief_authority_test.go:201: authority error = &dispatch.BriefAuthorityRefusal{MissingPaths:[]string{"docs/a"}}, want missing paths ["docs/a b.md"]` |
| R1.rule4-unterminated-backtick-consumed | round 1 rule 4 | brief.go `if offset < 0 {`: an unterminated span equal to a member is recorded through end of line | TestBriefAuthorityUnterminatedBacktickFallsBack | `brief_authority_test.go:250: authority error = &dispatch.BriefAuthorityRefusal{MissingPaths:[]string{"docs/a b.md"}}, want missing paths ["docs/a"]` |
| R1.rule5-unterminated-json-consumed | round 1 rule 5 | brief.go `if !terminated {`: the same change for a JSON string | TestBriefAuthorityUnterminatedJSONStringFallsBack | `brief_authority_test.go:254: authority error = &dispatch.BriefAuthorityRefusal{MissingPaths:[]string{"docs/a b.md"}}, want missing paths ["docs/a"]` |
| R1.rule6-invalid-json-raw-contents | round 1 rule 6 | brief.go:437 uses the raw contents when the JSON string is invalid | TestBriefAuthorityInvalidJSONStringFallsBack | `brief_authority_test.go:258: authority error = &dispatch.BriefAuthorityRefusal{MissingPaths:[]string{"docs/a\tb.md"}}, want missing paths ["docs/a"]` |
| EXTRA.qmark-treated-concrete | reader extra, sub-kind | brief.go:408 pattern check drops `?` only | none, exit 0 | no failure line |
| EXTRA.bracket-treated-concrete | reader extra, sub-kind | brief.go:408 pattern check drops `[` and `]` only | none, exit 0 | no failure line |
| EXTRA.dollar-brace-treated-concrete | reader extra, sub-kind | brief.go:409 exclusion drops `$` and `{` only | none, exit 0 | no failure line |
| EXTRA.single-line-suffix-treated-concrete | reader extra, sub-kind | brief.go:19 `:[0-9]+(-[0-9]+)?$` becomes `:[0-9]+-[0-9]+$` | none, exit 0 | no failure line |

Witness check for F-41. It ran in `/tmp/opus-u1bii3-witness` with the three round-1 rows restored, using `go test -count=1 -timeout 40m ./internal/dispatch -run '^TestBriefAuthorityBoundaryInputStillRequired$' -v`:

| brief.go | Subtests that failed | Observed failure line |
| --- | --- | --- |
| folded candidate (2ca4d47d...) | none; all eight pass | no failure line |
| A4.glob-treated-concrete | pattern-no-equality | `brief_authority_test.go:281: authority error = &dispatch.BriefAuthorityRefusal{MissingPaths:[]string{"new/*.md"}}, want missing paths []` |
| A4.backslash-treated-concrete | backslash-no-equality | `brief_authority_test.go:281: authority error = &dispatch.BriefAuthorityRefusal{MissingPaths:[]string{"new/a\\b.md"}}, want missing paths []` |
| A4.directory-treated-concrete | directory-no-equality | `brief_authority_test.go:281: authority error = &dispatch.BriefAuthorityRefusal{MissingPaths:[]string{"new/file.md/"}}, want missing paths []` |

The page's witness names were also run on the candidate copy:
- `-run '^TestBriefAuthorityBoundaryInputStillRequired$/^<name>$'` for pattern-no-equality, backslash-no-equality and directory-no-equality;
- `-run '^TestBriefAuthoritySpecialCharacterInput$/^backslash$'`.

Each prints `testing: warning: no tests to run` and `ok ... [no tests to run]`, and exits 0.

## Setup, conformance and checks with no finding

Line numbers are the candidate's.

Setup:
- `/tmp/opus-u1bii3-copy` holds `git archive HEAD metasystem` (HEAD fad3ace8) plus the two live files, committed. Its sha256 values are brief.go 2ca4d47d... and brief_authority_test.go c57b399d..., equal to the live files.
- `/tmp/opus-u1bii3-mut` and `/tmp/opus-u1bii3-witness` are clones of it. `/tmp/opus-u1bii3-special` is trunk plus the six special-character files.
- The earlier scratch `/tmp/opus-u1bii-base` was reused only as a probe root and engine source. Its tree (be630ec2) equals `fad3ace8:metasystem` and it is clean. Both engines were rebuilt into `/tmp/opus-u1bii3-bin`.
- Environment: GOCACHE=/tmp/opus-u1bii3-gocache, with GOTMPDIR and TMPDIR=/tmp/opus-u1bii3-tmp (outside Git). METASYSTEM_BIN, GIT_DIR and METASYSTEM_CONTEXT_COST_PROOF were unset.
- Every go test used -count=1 -timeout 40m and a focused -run on ./internal/dispatch. There was no race suite, no fixture bed, no ./cmd/metasystem test, and no write to the live worktree.

Step 1, conformance:
- `git status --short --untracked-files=all` lists only the two Boundary files.
- numstat is 117+17 and 184+16: 334 changed lines, under 400, as result-2 says.
- brief.go's hunks from base are the new `briefPathLine` regex (:19), the `bounds` parameter and its call (:277, :294), and the extraction function with its helpers (:332-485).
- From round 1 to round 2, brief.go changes only three things: the regex, briefBoundaryMemberIsConcrete (:406-411), and passing `concreteMembers` instead of every member (:366).
- These are untouched: the token regexes (:17-18), the `*<>${` exclusion (:387), prefix lookup, paired-bounds admission, operands and every BRIEF_BOUNDS_INVALID text. S40 to S49 are identical in all five modes. dispatch_verbs.go and dispatch.sh are not in the diff.

Ruling A4 as written:
- briefBoundaryMemberIsConcrete rejects a trailing `/`, every byte briefBoundaryIsPattern checks (`*?[]` and backslash), `<>${`, and `:[0-9]+(-[0-9]+)?$`.
- That is the ruling's list, with `*` covered by the pattern check.

Step 2, members that are not concrete: S14, S11, S12, S21, S13, S25 and S31 now give base's result in all five modes, and so does S35.

Step 2, concrete special-character members, tested against a repository where files with a space, comma, quote, tab, newline or plus are committed:
- A committed member cited as input is admitted.
- An absent member cited as input, as output then input, or as input then output is refused, naming the full member.
- Output-only and Create citations are admitted.
- The same span with an unrelated nonempty Boundary gives base's fragment.
- So the input-wins rule holds.

Step 2, F-1 shapes and the real brief:
- Every F-1 shape gives base's result:
  - with `Boundary: []`;
  - with the two-file Boundary (S08a to S08g, S09);
  - with concrete members that are prefixes of the spans (S09pa to S09pg);
  - headerless (S10).
- The real brief matches base unmodified, with `Boundary: []`, with its span Boundary and with the 1b-i read's Boundary.
- The 32-brief sweep shows no difference in any variant. Round 1 had one, in the span-Boundary variant.

Step 2, remaining differences:
- S18, S19, S32, S34, N14 and the special-character rows are concrete-member identities. S20 is row A4.root-file.
- No difference is unclassified.

Step 3, F-32 closed:
- R3.gate-any-nonempty-boundary fails the five TestBriefAuthorityBoundedUsesTrunkTokenScanner nonempty-boundary subtests, eight TestBriefAuthoritySpecialCharacterInput subtests and TestBriefAuthorityRealBriefRegression/nonempty-boundary.
- R3.backtick-gate-member-prefix fails TestBriefAuthorityBoundedUsesTrunkTokenScanner/backticked-path-line/nonempty-boundary and TestBriefAuthorityRealBriefRegression/nonempty-boundary.

Step 4, F-33 closed: X.citation-diffboundary-example-guard-removed fails TestBriefAuthorityTreatsMetasystemDiffBoundaryExampleAsPrefixOnly/bounded-member at :97, and S24 gives base's result.

Step 5: each of the five A4 kind mutants the builder lists fails a named TestBriefAuthorityNonConcreteBoundaryMembersUseTrunkScanner subtest.

Round 1 rules 1, 2, 4, 5 and 6 still fail named tests on the folded code:
- The invalid-JSON witness changed its input from `docs/a\qb.md` to a literal tab (brief_authority_test.go:258).
- A backslash member is no longer concrete, so the old input could not show rule 6. The new input fails under the rule-6 mutant.

Shared fixture:
- newBriefAuthorityRepo now also commits metasystem/internal, metasystem/records and metasystem/scripts (:373).
- At fad3ace8, only the diffBoundary example test in that file cites a metasystem/ path, and its headerless case keeps trunk's assertion.
- Every authority test passes on the baseline run.

Result-2's claims:
- The failure lines it cites (:187, :196, :201, :244, :250, :254, :258, :327, :97) match my runs, so read 1's F-37 line offset is gone.
- `gofmt -l internal/dispatch` prints nothing and `go vet ./internal/dispatch` exits 0 on my copy.
- The race run and the builder's CLI observations were not re-run.

Not re-run by me: the race suite, go-gate.sh --fast and the command package.

Live worktree: HEAD is fad3ace8 with only the two files modified. The sha256 values, brief.go 2ca4d47d... and brief_authority_test.go c57b399d..., were the same at start and end.
