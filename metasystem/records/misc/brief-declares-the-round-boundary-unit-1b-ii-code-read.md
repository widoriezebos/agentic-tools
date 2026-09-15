VERDICT: send back

# Closing read: brief-declares-the-round-boundary, unit 1b-ii

Reader: Opus. I did not build this unit. Subject: the uncommitted diff in `.claude/worktrees/bdrb-u1bii` on HEAD fad3ace8, in internal/dispatch/brief.go and brief_authority_test.go. Spec: the design page's revision 3, section "Paths and matching", rows A3 to A5, and the amendment after unit 1b's read. Where ruling A1 differs from the page, A1 wins.

Material findings: 3 (F-31, F-32, F-33). Not material: 4 (F-34 to F-37).

`<S>` is `/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/36c93128-ec16-4146-8def-3f706ba4ef10/scratchpad`. Line numbers in brief.go and brief_authority_test.go are the candidate's; the live worktree and my copy are byte-identical.

Setup:
- `/tmp/opus-u1bii-copy` holds `git archive HEAD metasystem` plus the two candidate files, committed as one base. Both sha256 values matched the live worktree.
- `/tmp/opus-u1bii-base` holds `git archive fad3ace8 metasystem`, committed.
- `/tmp/opus-u1bii-mut` and `/tmp/opus-u1bii-probe` are copies of the candidate copy, used for mutations and probe tests. The probe tests were removed after use.
- Engines: `/tmp/opus-u1bii-bin/ms-base` built from the base copy, `ms-cand` from the candidate copy.
- Environment: GOCACHE=/tmp/opus-u1bii-gocache, GOTMPDIR and TMPDIR=/tmp/opus-u1bii-tmp (outside Git). METASYSTEM_BIN, GIT_DIR and METASYSTEM_CONTEXT_COST_PROOF unset. Every go test used `-count=1 -timeout 40m` and a focused `-run`.
- No full race suite, no fixture bed, no commit. The live worktree was never written.

## Findings

### F-31. Severity medium. Material yes.
Claim: a span equal to a directory, glob, placeholder or path:line Boundary member is recorded as a concrete repository path whenever its first components pass the directory rule, so the candidate refuses briefs trunk admits, and a consumed pattern span that is not recorded hides a missing path trunk reports.

Evidence:
- Code:
  - brief.go:378-380 records every member citation that passes briefAuthorityPathEligible (brief.go:471). That function checks only the first path components.
  - Neither the directory flag, the pattern bytes nor trunk's `*<>${` exclusion (brief.go:388) is applied to a citation.
  - brief.go:384 skips every token inside any member span, whether the citation was recorded or not.
- Command: `python3 <S>/u1bii-probe.py <S>`. Log `<S>/u1bii-probe.json`. Authority mode is `job brief-mode --authority-only --brief B --root /tmp/opus-u1bii-base/metasystem --base-tree /tmp/opus-u1bii-base --disk-root /tmp/opus-u1bii-base`, as dispatch.sh:863 shapes it. The combined mode differs the same way in every case below.
- S14, new directory member. `Boundary: ["metasystem/internal/u1biipkg/"]`, line "Put the package in `metasystem/internal/u1biipkg/`."
  - Base exits 0.
  - Candidate exits 1: `BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/u1biipkg/`.
- S11, glob member. `Boundary: ["metasystem/scripts/agents/*.sh"]`, line "Edit `metasystem/scripts/agents/*.sh` only.". This is the first read's probe 22 with the glob now declared.
  - Base exits 0.
  - Candidate exits 1: `BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/scripts/agents/*.sh`.
  - S12 cites the same glob as a JSON string and gives the same result.
- S21. `Boundary: ["metasystem/internal/u1biipkg/*.go"]`, line "Add files matching `metasystem/internal/u1biipkg/*.go`."
  - Base exits 0.
  - Candidate refuses, naming the glob.
- S13 and S25: a placeholder member, and the path:line member `metasystem/internal/dispatch/brief.go:47-53` (brief.go exists).
  - Base exits 0.
  - Candidate refuses, naming the placeholder and `brief.go:47-53`.
- S31, the other direction. `Boundary: ["metasystem/u1biix/* metasystem/internal/u1bii-missing.md"]`, with that span in backticks.
  - Base exits 1 and names `metasystem/internal/u1bii-missing.md`.
  - Candidate exits 0.
  - This input is contrived.
- No difference:
  - S15, an existing directory member.
  - S22 and S23, a glob member on a Workspace line and on a Create line.
  - Why only absent directories refuse: `git -C /tmp/opus-u1bii-base cat-file -e <commit>:metasystem/internal/dispatch/` exits 0, while the absent `.../nonexist/` exits 128. Every glob refuses.
- Tests: A4.pattern-no-equality, A4.backslash-no-equality and A4.directory-no-equality (brief_authority_test.go:240-242) all use `new/`, which is absent and not eligible. No test cites a pattern member or a directory member under an eligible directory.
- Ruling A1 says: "The input-wins rule still holds for a special-character Boundary member, and every other citation gives the same result as trunk."
  - A directory spelling has no special character.
  - `*`, `<>` and `:` are not among row A3's special characters (space, comma, quote, tab, newline, backslash).
  - Rows A4.root-file and A4.new-directory authorise the concrete-member refusals (S20). They do not cover these cases.
  - So A1 does not allow these differences.
- The other reading, that every member span is checked by its identity, still gives a wrong visible result:
  - a brief that repeats its glob Boundary on an ordinary input line can never be admitted;
  - the refusal calls a glob a missing repository path.
- The page's paragraph ("Apply the existing directory eligibility rule to a structured citation's decoded first components") and row A3.backslash point toward the builder's reading. So the seat rules first.
- Artifacts to change:
  - brief.go:378-384. For example, a directory member or a pattern member without a backslash could get trunk's result for its span, without consuming it.
  - An A4 witness with a glob member and a directory member under an eligible directory.

### F-32. Severity medium. Material yes.
Claim: ruling A1's condition "only when exactly equal to a member" is witnessed only with `Boundary: []`, so a mutation that consumes every complete span whenever the Boundary is nonempty passes all 37 authority tests and brings back F-1 for every bounded brief with a real Boundary.

Evidence:
- The three witnesses for a span that equals no member all run with an empty Boundary:
  - TestBriefAuthorityBoundedUsesTrunkTokenScanner (brief_authority_test.go:181);
  - A3's nonmember assertion (:212);
  - TestBriefAuthorityRealBriefRegression (:289, which appends `Boundary: []`).
  - With an empty Boundary, `members` is empty. brief.go:422 and :431 then never match, whatever their condition is.
- Mutation `R3.gate-any-nonempty-boundary`: brief.go:422 `if members[value] {` becomes `if len(members) > 0 {`, and brief.go:431 becomes `if valid && len(members) > 0 {`.
  - `go test -count=1 -timeout 40m ./internal/dispatch -run '^TestBriefAuthority' -v` in /tmp/opus-u1bii-mut/metasystem exits 0 with no FAIL line.
- Mutation `R3.backtick-gate-member-prefix`: brief.go:422 consumes a span that starts with any member. The same command exits 0.
- Engine built with the first mutation (`/tmp/opus-u1bii-bin/ms-mut-nonempty`), authority mode, with `Boundary: ["metasystem/internal/dispatch/brief.go", "metasystem/internal/dispatch/brief_authority_test.go"]`:
  - S08a "See `metasystem/internal/dispatch/brief.go:47-53`.": the candidate exits 0. The mutant exits 1: `BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/dispatch/brief.go:47-53`.
  - S08c: the mutant names `metasystem/scripts/agents/go-gate.sh --fast`.
  - S08e and S08f: the mutant names the placeholder and the glob.
  - S08g, the quoted sentence: the candidate refuses `metasystem/internal/u1bii-missing.md`. The mutant exits 0.
- The real brief with the 1b-i read's Boundary (`metasystem/internal/landing/receipt.go`, `metasystem/internal/landing/testing.go`), run against the main checkout:
  - the candidate exits 0;
  - the mutant exits 1: `BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/landing/receipt.go:426, metasystem/internal/landing/testing.go:160`.
- The result file says each A3 case "proves the identical nonmember span has trunk's `docs/a` result". It proves this only with an empty Boundary.
- Artifact to change: brief_authority_test.go. Run the five shapes, one A3 nonmember span and the real brief with a nonempty Boundary whose members differ from the spans.

### F-33. Severity low. Material yes.
Claim: the diffBoundary-example exemption the builder carried onto the citation path (brief.go:379) has no failing test; without it, a Boundary member cited on a diffBoundary example line is refused where trunk admits.

Evidence:
- Mutation `X.citation-diffboundary-example-guard-removed`: brief.go:379 `!(boundaryExample && strings.HasPrefix(citation.path, "metasystem/")) {` becomes `true {`.
  - `go test -count=1 -timeout 40m ./internal/dispatch -run '^TestBriefAuthority' -v` exits 0.
- S24: `Boundary: ["metasystem/internal/dispatch/u1bii-new.go"]`, line `diffBoundary example: ["metasystem/internal/dispatch/u1bii-new.go"]`, run with an engine built with that mutation (`ms-mut-example`), authority mode:
  - base and candidate exit 0;
  - the mutant exits 1: `BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/dispatch/u1bii-new.go`.
- The only exemption witness, TestBriefAuthorityTreatsMetasystemDiffBoundaryExampleAsPrefixOnly, is headerless, so it never reaches brief.go:379.
- Why low: no template or real brief uses the phrase today. `grep -rn -i 'diffBoundary example'` over non-test .md, .sh and .go sources, and over the 32 real briefs, prints nothing.
- Artifact to change: brief_authority_test.go, a bounded variant of that test with the example path as a Boundary member.

### F-34. Severity low. Material no.
Claim: the span lexer loses member spans after an unpaired delimiter or inside a quoted sentence, so a special-character member there gets trunk's fragment result instead of its full identity.

Evidence:
- Code:
  - brief.go:413-415 and :428-430 return at the first unterminated backtick or quote.
  - brief.go:425 and :434 skip a whole nonmember span, including member spans inside it.
- S29 `The 5" drive: read "metasystem/docs/a b.md".` and S30 ``Say "see `metasystem/docs/a b.md` now".``: both engines exit 1, naming `metasystem/docs/a`.
- Probe test `TestZZReaderProbeLexerEdges` (run in /tmp/opus-u1bii-probe, then removed):
  - `member-after-unterminated-backtick: citations=[] candidates=["docs/a"]`;
  - `member-after-odd-quote: citations=[] candidates=["docs/a"]`.
- Not material:
  - the result equals trunk and fails closed;
  - the page does not define which delimiter takes precedence, or nesting.
- Hard-wrapped quotations often leave an odd number of quotes on a line in real briefs. The page may want one sentence on this.

### F-35. Severity low. Material no.
Claim: JSON decoding inside backticks (brief.go:419-421) accepts whitespace around the string and treats `null` as decoded, while the page decodes only when "the entire contents form a complete JSON string".

Evidence:
- S34 "Read ` \"metasystem/docs/a b.md\" `.": base names `metasystem/docs/a`, the candidate names `metasystem/docs/a b.md`.
- Probe test output:
  - `padded-json-in-backticks: citations=[{5 22 docs/a b.md}]`.
  - `null-literal-member: citations=[] candidates=[]`. `json.Unmarshal` of `null` succeeds and turns the value into "", so a member named `null` is never matched.
- Not material: both inputs are contrived, and neither makes a difference from trunk that A1 forbids.

### F-36. Severity low. Material no.
Claim: TestBriefAuthorityRealBriefRegression checks the embedded brief against whatever repository holds the test source, so it depends on 20 unrelated committed paths and fails outside a Git checkout with a message that blames the authority result.

Evidence:
- brief_authority_test.go:283-291 derives the repository from runtime.Caller and calls ValidateBriefAuthority(brief, repo, repo).
- Probe test `TestZZReaderProbeRealBriefCandidates` (removed) lists the paths the fixture needs committed:
  - 20 in the copy, including `metasystem/plans/landing-receipt-survives-records-drift-design.md`, both `metasystem/records/misc/landing-receipt-survives-records-drift-critique-r*.md` and `metasystem/internal/landing/receipt.go`;
  - the full checkout adds `records/narrator-digest.log`.
- The package copied to /tmp/opus-u1bii-nogit (no Git repository), then `go test -count=1 -timeout 40m ./internal/dispatch -run '^TestBriefAuthorityRealBriefRegression$'`, prints `brief_authority_test.go:291: real implementer brief changed authority result: brief admission cannot resolve installation prefix: exit status 128`.
- Moving or retiring any of those plans or records files turns this dispatch test red.
- The constant at :295 embeds a session scratchpad path and `/Users/wido/...`.
- The fixture matches the real brief byte for byte: 12693 bytes; a Python comparison prints `equal: True`.
- No other internal/dispatch test resolves the enclosing repository. internal/proofrun and internal/goal tests do.
- Not material under the seat's definition. Artifact, if wanted: a temporary repository with those paths committed.

### F-37. Severity low. Material no.
Claim: the builder's mutation table cites failure lines one line too early for most rows, and two A3 subtests have identical inputs.

Evidence:
- Observed failure lines are :188, :193, :220, :224, :228, :211, :251 and :278. The builder's cells give :187, :192, :219, :223, :227, :210, :250 and :277. The :291 row matches.
- literal-backticks and consumed-span (brief_authority_test.go:205-206) both cite "`docs/a b.md`".
- Both subtests fail under the consumed-span mutation and under the rule-1 mutation, so no rule loses its failing test.

## Comparison table

Base engine fad3ace8 against the candidate engine. Every brief ran five ways: normal (`--brief B --root R`), combined (plus `--base-tree` and `--disk-root`), authority (dispatch.sh:863 shape), mode-only (dispatch.sh:859 shape), and non-Git (`--brief B` from a directory outside Git). The synthetic corpus used R=/tmp/opus-u1bii-base/metasystem and the base copy as base tree and disk root. The "Other modes" column reports the four other modes.

| Brief | Boundary | Text lines | Base, authority mode | Candidate, authority mode | Other modes | Differs | A1 allows |
| --- | --- | --- | --- | --- | --- | --- | --- |
| S01-f1-pathline-exists | `` [] `` | `` See `metasystem/internal/dispatch/brief.go:47-53`. `` | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S02-f1-pathline-missing | `` [] `` | `` Read `metasystem/internal/landing/u1bii-missing.go:426`. `` | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/landing/u1bii-missing.go | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/landing/u1bii-missing.go | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S03-f1-command-exists | `` [] `` | `` Run `metasystem/scripts/agents/go-gate.sh --fast`. `` | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S04-f1-command-missing | `` [] `` | `` Run `metasystem/scripts/agents/u1bii-missing.sh --fast`. `` | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/scripts/agents/u1bii-missing.sh | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/scripts/agents/u1bii-missing.sh | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S05-f1-placeholder | `` [] `` | `` Write `metasystem/records/misc/<goal>-code-read.md`. `` | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S06-f1-glob | `` [] `` | `` Edit `metasystem/scripts/agents/*.sh` only. `` | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S07-f1-quoted-sentence-missing | `` [] `` | `` Say "read metasystem/internal/u1bii-missing.md first". `` | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/u1bii-missing.md | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/u1bii-missing.md | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S08a-f1-nonempty-boundary | `` ["metasystem/internal/dispatch/brief.go", "metasystem/internal/dispatch/brief_authority_test.go"] `` | `` See `metasystem/internal/dispatch/brief.go:47-53`. `` | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S08b-f1-nonempty-boundary | `` ["metasystem/internal/dispatch/brief.go", "metasystem/internal/dispatch/brief_authority_test.go"] `` | `` Read `metasystem/internal/landing/u1bii-missing.go:426`. `` | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/landing/u1bii-missing.go | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/landing/u1bii-missing.go | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S08c-f1-nonempty-boundary | `` ["metasystem/internal/dispatch/brief.go", "metasystem/internal/dispatch/brief_authority_test.go"] `` | `` Run `metasystem/scripts/agents/go-gate.sh --fast`. `` | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S08d-f1-nonempty-boundary | `` ["metasystem/internal/dispatch/brief.go", "metasystem/internal/dispatch/brief_authority_test.go"] `` | `` Run `metasystem/scripts/agents/u1bii-missing.sh --fast`. `` | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/scripts/agents/u1bii-missing.sh | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/scripts/agents/u1bii-missing.sh | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S08e-f1-nonempty-boundary | `` ["metasystem/internal/dispatch/brief.go", "metasystem/internal/dispatch/brief_authority_test.go"] `` | `` Write `metasystem/records/misc/<goal>-code-read.md`. `` | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S08f-f1-nonempty-boundary | `` ["metasystem/internal/dispatch/brief.go", "metasystem/internal/dispatch/brief_authority_test.go"] `` | `` Edit `metasystem/scripts/agents/*.sh` only. `` | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S08g-f1-nonempty-boundary | `` ["metasystem/internal/dispatch/brief.go", "metasystem/internal/dispatch/brief_authority_test.go"] `` | `` Say "read metasystem/internal/u1bii-missing.md first". `` | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/u1bii-missing.md | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/u1bii-missing.md | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S09-f1-all-nonempty-boundary | `` ["metasystem/internal/dispatch/brief.go", "metasystem/internal/dispatch/brief_authority_test.go"] `` | `` See `metasystem/internal/dispatch/brief.go:47-53`. / Read `metasystem/internal/landing/u1bii-missing.go:426`. / Run `metasystem/scripts/agents/go-gate.sh --fast`. / Run `metasystem/scripts/agents/u1bii-missing.sh --fast`. / Write `metasystem/records/misc/<goal>-code-read.md`. / Edit `metasystem/scripts/agents/*.sh` only. / Say "read metasystem/internal/u1bii-missing.md first". `` | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/landing/u1bii-missing.go, metasystem/internal/u1bii-missing.md, metasystem/scripts/agents/u1bii-missing.sh | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/landing/u1bii-missing.go, metasystem/internal/u1bii-missing.md, metasystem/scripts/agents/u1bii-missing.sh | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S10-f1-all-headerless | `` (none) `` | `` See `metasystem/internal/dispatch/brief.go:47-53`. / Read `metasystem/internal/landing/u1bii-missing.go:426`. / Run `metasystem/scripts/agents/go-gate.sh --fast`. / Run `metasystem/scripts/agents/u1bii-missing.sh --fast`. / Write `metasystem/records/misc/<goal>-code-read.md`. / Edit `metasystem/scripts/agents/*.sh` only. / Say "read metasystem/internal/u1bii-missing.md first". `` | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/landing/u1bii-missing.go, metasystem/internal/u1bii-missing.md, metasystem/scripts/agents/u1bii-missing.sh | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/landing/u1bii-missing.go, metasystem/internal/u1bii-missing.md, metasystem/scripts/agents/u1bii-missing.sh | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S11-member-glob-backtick | `` ["metasystem/scripts/agents/*.sh"] `` | `` Edit `metasystem/scripts/agents/*.sh` only. `` | exit 0 | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/scripts/agents/*.sh | differs in combined the same way | yes | no (F-31) |
| S12-member-glob-json | `` ["metasystem/scripts/agents/*.sh"] `` | `` Edit "metasystem/scripts/agents/*.sh" only. `` | exit 0 | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/scripts/agents/*.sh | differs in combined the same way | yes | no (F-31) |
| S13-member-placeholder | `` ["metasystem/records/misc/<goal>-code-read.md"] `` | `` Write `metasystem/records/misc/<goal>-code-read.md`. `` | exit 0 | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/records/misc/<goal>-code-read.md | differs in combined the same way | yes | no (F-31) |
| S14-member-new-directory | `` ["metasystem/internal/u1biipkg/"] `` | `` Put the package in `metasystem/internal/u1biipkg/`. `` | exit 0 | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/u1biipkg/ | differs in combined the same way | yes | no (F-31) |
| S15-member-existing-directory | `` ["metasystem/internal/dispatch/"] `` | `` Work inside `metasystem/internal/dispatch/`. `` | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S16-member-existing-file | `` ["metasystem/internal/dispatch/brief.go"] `` | `` Read `metasystem/internal/dispatch/brief.go`. `` | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S17-member-new-file-eligible-dir | `` ["metasystem/internal/dispatch/u1bii-new.go"] `` | `` Read `metasystem/internal/dispatch/u1bii-new.go`. `` | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/dispatch/u1bii-new.go | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/dispatch/u1bii-new.go | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S18-member-space-backtick | `` ["metasystem/docs/a b.md"] `` | `` Read `metasystem/docs/a b.md`. `` | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a b.md | differs in combined the same way | yes | yes: special-character member, A3 |
| S19-member-space-json | `` ["metasystem/docs/a b.md"] `` | `` Read "metasystem/docs/a b.md". `` | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a b.md | differs in combined the same way | yes | yes: special-character member, A3 |
| S20-member-install-root-file | `` ["metasystem/NOTICE-u1bii.md"] `` | `` Read `metasystem/NOTICE-u1bii.md`. `` | exit 0 | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/NOTICE-u1bii.md | differs in combined the same way | yes | yes: row A4.root-file |
| S21-member-pattern-absent-subdir | `` ["metasystem/internal/u1biipkg/*.go"] `` | `` Add files matching `metasystem/internal/u1biipkg/*.go`. `` | exit 0 | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/u1biipkg/*.go | differs in combined the same way | yes | no (F-31) |
| S22-member-glob-workspace | `` ["metasystem/scripts/agents/*.sh"] `` | `` # Workspace / May-write: `metasystem/scripts/agents/*.sh`. `` | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S23-member-glob-create | `` ["metasystem/scripts/agents/*.sh"] `` | `` Create `metasystem/scripts/agents/*.sh`. `` | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S24-member-diffboundary-example | `` ["metasystem/internal/dispatch/u1bii-new.go"] `` | `` diffBoundary example: ["metasystem/internal/dispatch/u1bii-new.go"] `` | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S25-member-pathline | `` ["metasystem/internal/dispatch/brief.go:47-53"] `` | `` See `metasystem/internal/dispatch/brief.go:47-53`. `` | exit 0 | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/dispatch/brief.go:47-53 | differs in combined the same way | yes | no (F-31) |
| S26-f4-unterminated-backtick-member | `` ["metasystem/docs/a b.md"] `` | `` Read `metasystem/docs/a b.md `` | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S26e-f4-unterminated-backtick-empty | `` [] `` | `` Read `metasystem/docs/a b.md `` | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S27-f4-unterminated-json-member | `` ["metasystem/docs/a b.md"] `` | `` Read "metasystem/docs/a b.md `` | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S27e-f4-unterminated-json-empty | `` [] `` | `` Read "metasystem/docs/a b.md `` | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S28-f4-invalid-json-member | `` ["metasystem/docs/a\\qb.md"] `` | `` Read "metasystem/docs/a\qb.md" `` | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S28e-f4-invalid-json-empty | `` [] `` | `` Read "metasystem/docs/a\qb.md" `` | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S29-quote-desync-member-json | `` ["metasystem/docs/a b.md"] `` | `` The 5" drive: read "metasystem/docs/a b.md". `` | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S30-member-backtick-inside-quote | `` ["metasystem/docs/a b.md"] `` | `` Say "see `metasystem/docs/a b.md` now". `` | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S31-pattern-member-hides-missing | `` ["metasystem/u1biix/* metasystem/internal/u1bii-missing.md"] `` | `` Edit `metasystem/u1biix/* metasystem/internal/u1bii-missing.md`. `` | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/u1bii-missing.md | exit 0 | differs in combined the same way | yes | no (F-31) |
| S32-member-and-missing-same-line | `` ["metasystem/docs/a b.md"] `` | `` Read `metasystem/docs/a b.md` and metasystem/internal/u1bii-missing.md. `` | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a, metasystem/internal/u1bii-missing.md | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a b.md, metasystem/internal/u1bii-missing.md | differs in combined the same way | yes | yes: A3 member; rest of line as trunk |
| S33-headerless-member-shape | `` (none) `` | `` Read `metasystem/docs/a b.md`. `` | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S34-json-in-backticks-padded | `` ["metasystem/docs/a b.md"] `` | `` Read ` "metasystem/docs/a b.md" `. `` | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/docs/a b.md | differs in combined the same way | yes | yes by A1 (member span); page wording differs (F-35) |
| S35-member-backslash-json | `` ["metasystem/internal/a\\b.txt"] `` | `` Read "metasystem/internal/a\\b.txt". `` | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/a | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/a\b.txt | differs in combined the same way | yes | yes: row A3.backslash |
| S40-mode-only-no-bounds | `` (none) `` | `` Read metasystem/internal/u1bii-missing.md. `` | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/u1bii-missing.md | exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/u1bii-missing.md | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S41-no-mode-valid-pair | `` [] `` |  | exit 0 | exit 0 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S42-partial-boundary | `` [] `` |  | exit 1, BRIEF_BOUNDS_INVALID: Ceiling: required with Boundary | exit 1, BRIEF_BOUNDS_INVALID: Ceiling: required with Boundary | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S43-partial-ceiling | `` (none) `` |  | exit 1, BRIEF_BOUNDS_INVALID: Boundary: required with Ceiling | exit 1, BRIEF_BOUNDS_INVALID: Boundary: required with Ceiling | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S44-bad-boundary-json | `` bad `` |  | exit 1, BRIEF_BOUNDS_INVALID: Boundary: expected a JSON array of paths | exit 1, BRIEF_BOUNDS_INVALID: Boundary: expected a JSON array of paths | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S45-bad-ceiling | `` [] `` |  | exit 1, BRIEF_BOUNDS_INVALID: Ceiling: expected a nonnegative decimal integer no greater than 9223372036854775807 | exit 1, BRIEF_BOUNDS_INVALID: Ceiling: expected a nonnegative decimal integer no greater than 9223372036854775807 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S46-unprefixed-member | `` ["internal/dispatch/brief.go"] `` |  | exit 1, BRIEF_BOUNDS_INVALID: Boundary: invalid path or pattern "internal/dispatch/brief.go" | exit 1, BRIEF_BOUNDS_INVALID: Boundary: invalid path or pattern "internal/dispatch/brief.go" | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S47-duplicate-boundary | `` [] `` |  | exit 1, BRIEF_BOUNDS_INVALID: Boundary: header occurs more than once | exit 1, BRIEF_BOUNDS_INVALID: Boundary: header occurs more than once | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S48-unprefixed-and-bad-ceiling | `` ["internal/a"] `` |  | exit 1, BRIEF_BOUNDS_INVALID: Ceiling: expected a nonnegative decimal integer no greater than 9223372036854775807 | exit 1, BRIEF_BOUNDS_INVALID: Ceiling: expected a nonnegative decimal integer no greater than 9223372036854775807 | same in normal, combined, mode-only, non-Git | no | nothing to allow |
| S49-no-mode-partial-missing | `` [] `` | `` Read metasystem/internal/u1bii-missing.md. `` | exit 1, BRIEF_BOUNDS_INVALID: Ceiling: required with Boundary | exit 1, BRIEF_BOUNDS_INVALID: Ceiling: required with Boundary | same in normal, combined, mode-only, non-Git | no | nothing to allow |

Real briefs from the main checkout, run against it as dispatch.sh on main would (`--root <main>/metasystem --base-tree <main> --disk-root <main>`), combined and authority modes:

| Brief set | Variant | Admitted (base = candidate) | Refused (base = candidate) | Differs | A1 allows |
| --- | --- | --- | --- | --- | --- |
| 32 artifacts/agents/*/brief.md | unmodified | 25 | 7 | 0 | nothing to allow |
| 32 artifacts/agents/*/brief.md | `Boundary: []`, `Ceiling: 1` after Working Mode | 25 | 7 | 0 | nothing to allow |
| 32 artifacts/agents/*/brief.md | Boundary = every metasystem/-prefixed backtick or JSON span in the brief (102 members over 32 briefs, 33 directories, 0 globs), Ceiling 400 | 24 | 7 | 1 (implementer-b76a0ddc11f73e99fa195d84-bspans) | no by F-31's reading (path:line members); artificial Boundary |
| implementer-b76a0ddc11f73e99fa195d84 | real brief b76a only, 1b-i read's Boundary (receipt.go, testing.go), Ceiling 400 | 1 | 0 | 0 | nothing to allow |

The one sweep difference: implementer-b76a0ddc11f73e99fa195d84 with its span Boundary. Base exit 0. Candidate exit 1, BRIEF_AUTHORITY_REFUSED: missing repository paths: metasystem/internal/landing/receipt.go:426, metasystem/internal/landing/testing.go:160. Its span Boundary includes the path:line spans `metasystem/internal/landing/receipt.go:426` and `metasystem/internal/landing/testing.go:160` as members, the S25 shape. The same brief unmodified, with `Boundary: []`, and with the 1b-i read's Boundary is admitted by both engines.

## Mutation table

Driver: `python3 <S>/u1bii-mutate.py <S>/u1bii-mut.jsonl`, on /tmp/opus-u1bii-mut/metasystem. Each mutation was applied alone. Then `go test -count=1 -timeout 40m ./internal/dispatch -run '^TestBriefAuthority' -v` ran, which covers all 37 authority tests. The file was restored after each run. Every record says `restored: true`, and the final sha256 is 598fcbbd..., the candidate's. Logs are in `<S>/u1bii-mut.jsonl.logs/`.

| Rule | Mutation (location) | Tests that failed | Observed failure line |
| --- | --- | --- | --- |
| 1. Backtick span equal to a member is one citation | brief.go:411 `` case '`': `` becomes `case 0:` | TestBriefAuthorityBacktickBoundaryIdentity; SpecialCharacterInput/literal-backticks, /consumed-span; BoundaryInputStillRequired/root-file, /new-directory, /output-then-input, /input-then-output | `brief_authority_test.go:188: authority error = &dispatch.BriefAuthorityRefusal{MissingPaths:[]string{"docs/a"}}, want missing paths ["docs/a b.md"]` |
| 2. Valid JSON string equal to a member is one citation | brief.go:426 `case '"':` becomes `case 1:` | TestBriefAuthorityJSONStringBoundaryIdentity; SpecialCharacterInput/space, /comma, /quote, /tab, /newline, /backslash; BoundedCitationCompatibility/artifact | `brief_authority_test.go:193: authority error = &dispatch.BriefAuthorityRefusal{MissingPaths:[]string{"docs/a"}}, want missing paths ["docs/a b.md"]` |
| 3. Other text keeps trunk's scanner: exclusion made unconditional | brief.go:384 becomes `if true {` | TrunkTokenScanner/backticked-path-line, /command-with-arguments, /quoted-sentence, and 25 more | `brief_authority_test.go:181: authority error = <nil>, want missing paths ["docs/missing.md"]` |
| 3. Backtick member gate removed | brief.go:422 becomes `if true {` | TrunkTokenScanner/backticked-path-line, /command-with-arguments, /placeholder, /glob; TestBriefAuthorityRealBriefRegression; SpecialCharacterInput/json-in-backticks, /literal-backticks, /consumed-span | `brief_authority_test.go:181: authority error = &dispatch.BriefAuthorityRefusal{MissingPaths:[]string{"docs/missing.md:426"}}, want missing paths ["docs/missing.md"]` |
| 3. JSON member gate removed | brief.go:431 becomes `if valid {` | TrunkTokenScanner/quoted-sentence; SpecialCharacterInput nonmember assertions; BoundedCitationCompatibility/headerless | `brief_authority_test.go:181: authority error = <nil>, want missing paths ["docs/quoted-missing.md"]` |
| 1 to 3. Exact equality, with a nonempty Boundary (F-32) | brief.go:422 and :431 gate on `len(members) > 0` | none, exit 0 | no failure line |
| 1. Exact equality, prefix variant (F-32) | brief.go:422 consumes a span that starts with any member | none, exit 0 | no failure line |
| 4. Unterminated backtick falls back | brief.go:413-415 records the rest of the line as a citation when it equals a member | TestBriefAuthorityUnterminatedBacktickFallsBack | `brief_authority_test.go:220: authority error = &dispatch.BriefAuthorityRefusal{MissingPaths:[]string{"docs/a b.md"}}, want missing paths ["docs/a"]` |
| 5. Unterminated JSON string falls back | brief.go:428-430, the same change | TestBriefAuthorityUnterminatedJSONStringFallsBack | `brief_authority_test.go:224: authority error = &dispatch.BriefAuthorityRefusal{MissingPaths:[]string{"docs/a b.md"}}, want missing paths ["docs/a"]` |
| 6. Invalid JSON string falls back | brief.go:431 uses the raw contents as the value when the JSON is invalid | TestBriefAuthorityInvalidJSONStringFallsBack | `brief_authority_test.go:228: authority error = &dispatch.BriefAuthorityRefusal{MissingPaths:[]string{"docs/a\\qb.md"}}, want missing paths ["docs/a"]` |
| 7, A3.json-in-backticks | brief.go:419 decoding disabled | SpecialCharacterInput/json-in-backticks | `brief_authority_test.go:211: authority error = &dispatch.BriefAuthorityRefusal{MissingPaths:[]string{"docs/a"}}, want missing paths ["docs/a b.md"]` |
| 7, A3.consumed-span | brief.go:384 becomes `if false {` | Backtick and JSON identity tests; SpecialCharacterInput, all nine subtests; BoundedCitationCompatibility/artifact | `brief_authority_test.go:188: authority error = &dispatch.BriefAuthorityRefusal{MissingPaths:[]string{"docs/a", "docs/a b.md"}}, want missing paths ["docs/a b.md"]` |
| 7, A4 concrete-member equality | brief.go:378 drops `\|\| concreteMembers[citation.path]` | BoundaryInputStillRequired/root-file, /new-directory, /output-then-input, /input-then-output | `brief_authority_test.go:251: authority error = <nil>, want missing paths ["NOTICE.md"]` |
| 7, A4.directory-no-equality | brief.go:346 drops the trailing-slash guard | BoundaryInputStillRequired/directory-no-equality | `brief_authority_test.go:251: authority error = &dispatch.BriefAuthorityRefusal{MissingPaths:[]string{"new/file.md/"}}, want missing paths []` |
| 7, A4.pattern-no-equality and backslash-no-equality | brief.go:346 drops the pattern guard | BoundaryInputStillRequired/pattern-no-equality, /backslash-no-equality | `brief_authority_test.go:251: authority error = &dispatch.BriefAuthorityRefusal{MissingPaths:[]string{"new/*.md"}}, want missing paths []` |
| 7, A5.create | brief.go:380 passes `citation.end` instead of `citation.start` | BoundedCitationCompatibility/create | `brief_authority_test.go:278: authority error = &dispatch.BriefAuthorityRefusal{MissingPaths:[]string{"records/a b.md"}}, want missing paths []` |
| Carried diffBoundary-example guard on citations (F-33) | brief.go:379 becomes `true {` | none, exit 0 | no failure line |

Engines built with two of these mutations, used as evidence in F-32 and F-33:
- `ms-mut-nonempty`, with the member gate on a nonempty Boundary;
- `ms-mut-example`, with the example guard removed.
Both were built from the mutation copy, and the file was restored afterwards. Their outputs are quoted in those findings.

## Conformance checks with no finding

- Boundary:
  - `git -C <worktree> status --short --untracked-files=all` lists only ` M metasystem/internal/dispatch/brief.go` and ` M metasystem/internal/dispatch/brief_authority_test.go`.
  - HEAD is fad3ace8.
- Ceiling: `git diff --numstat` gives 111+17 and 130+0, 258 changed lines, with no new files. This is under 400 and matches the result file.
- Non-goals:
  - The diff has hunks only at brief.go:276, :293 and :331-480, and in the test file.
  - dispatch_verbs.go, dispatch.sh, admitBriefBytes, parseBriefBounds, ReadBriefAdmissionAtRoot, the prefix lookup and every BRIEF_BOUNDS_INVALID text are untouched.
  - The token regexes (brief.go:17-18) and the `*<>${` exclusion (brief.go:388) are unchanged.
  - briefAuthorityPathEligible (brief.go:471) is trunk's inline directory rule, moved verbatim.
  - No unit 2, 3a to 3c, 4a to 4c, 6a or 6b identifiers appear.
- Ruling A1, trunk direction (step 2):
  - The five F-1 shapes (S01 to S07) are identical in all five modes: with `Boundary: []`, with a realistic nonempty Boundary (S08a to S08g, S09), and headerless (S10). The quoted sentence is still refused, naming its missing path.
  - The real brief is admitted by both engines unmodified, with `Boundary: []`, and with the 1b-i read's Boundary.
  - The 32-brief sweep shows no difference in the unmodified and `Boundary: []` variants.
- Ruling A1, member direction: S18, S19, S20, S32 and S35 differ in ways A1 or rows A3 and A4 allow. F-31 lists the ones not allowed.
- F-4 (step 4): S26, S27 and S28 give base's result with the member in the Boundary and with an empty Boundary. Every run names `metasystem/docs/a`. The matching tests fail under consumption to the end of the line, or under a literal span (rules 4 to 6 above).
- Admission (step 5):
  - S40 to S49 are identical in all five modes: no mode, partial pairs, bad Boundary JSON, bad Ceiling, an unprefixed member, a duplicate Boundary, and an unprefixed member with a bad Ceiling (the Ceiling error wins, as ruling A3 says).
  - `go test -count=1 -timeout 40m ./cmd/metasystem -run '^TestDispatchBriefBounds' -v` passes TestDispatchBriefBoundsAdmission, TestDispatchBriefBoundsPrefixLookupOrdering and TestDispatchBriefBoundsFailureSuffix.
  - `go test -count=1 -timeout 40m ./internal/dispatch -run '^(TestBriefAuthority.*|TestBriefAdmission|TestBriefModeOnly|TestBriefMode|TestBriefBounds.*)$' -v` gives 125 PASS lines and no FAIL.
- Bounds carry the original repository-relative members: parseBriefBounds returns `boundary`, not projected strings. So in a nested installation, citations can equal members, as S18 and S19 show through `--root`.
- Verification on the copy: `gofmt -l internal/dispatch` prints nothing, and `go vet ./internal/dispatch` exits 0.
- Not re-run by me: the full race suite, `go-gate.sh --fast` and staticcheck. `command -v staticcheck` prints nothing on this machine.
- Live worktree: both files' sha256 values (brief.go 598fcbbd..., brief_authority_test.go 39537be0...) and the diff sha256 (3156044e...) were the same at the start and at the end.
