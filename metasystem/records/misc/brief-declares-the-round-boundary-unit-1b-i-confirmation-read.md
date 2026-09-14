VERDICT: land

# Code read: brief-declares-the-round-boundary, unit 1b-i, fold round 3 (confirmation read)

Reader: Opus. I did not build this unit. Scope: did round 3 close F-11 and F-12 of the round-2 read without changing anything else. Subject: the uncommitted six-file diff in `.claude/worktrees/bdrb-u1b` on HEAD 914b61ed. Spec: `artifacts/reports/codex-bdrb-u1bi-fold3-brief.md`. Claims checked: `artifacts/reports/codex-bdrb-u1bi-result-3.md`.

`<S>` is `/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/36c93128-ec16-4146-8def-3f706ba4ef10/scratchpad`.

Material findings: 0. Not material: 2 (F-21, F-22).

## Setup

- I edited nothing in the live worktree. Its six sha256 values are in `<S>/u1bi3-live-start.sha`. They matched at the start and again after the last run.
- `/tmp/opus-u1bi3-copy` is a fresh Git repository with two commits: `git archive HEAD metasystem`, then the six live files. Its six hashes matched the live worktree before any mutation. Its numstat matches the worktree.
- `/tmp/opus-u1bi3-copy2` is a clone of that copy, used for the environment runs.
- Every mutation ran in a copy. After each one the file was restored with `git checkout` and all six hashes were checked. Every restore passed.
- Environment: GOCACHE=/tmp/opus-u1bi3-gocache, GOTMPDIR=/tmp/opus-u1bi3-tmp, TMPDIR=/tmp/opus-u1bi3-tmpdir (outside Git). METASYSTEM_BIN, GIT_DIR and METASYSTEM_CONTEXT_COST_PROOF were unset. Every go test used -timeout 40m and a focused -run. No race suite and no fixture bed ran. Toolchain: go1.27.1 darwin/arm64.
- Scripts and logs:
  - `<S>/u1bi3-mut.py`, with `<S>/u1bi3-mut-logs/`;
  - `<S>/u1bi3-env.sh`, with `<S>/u1bi3-env-*.log`;
  - `<S>/u1bi3-harden.sh`, with `<S>/u1bi3-harden-*.log`;
  - `<S>/u1bi3-trace.sh`, with `<S>/u1bi3-trace-*.out`;
  - `<S>/u1bi3-tmpprobe.sh`.
- A first environment pass ran under zsh. zsh did not split the command string, so `env` printed the environment and no test ran. I discarded that pass. Every environment row below comes from the bash rerun in `<S>/u1bi3-env.sh`. The leftover `<S>/u1bi3-env-M3-nongit.log` is an environment dump from the discarded pass.

## Findings

### F-21. Severity low. Material no.
Claim: the ordering witness still misses a prefix lookup placed inside the member loop after a member passes `validateBriefMember`, or between the Ceiling digit check and the range check, because the malformed-Ceiling case has an empty Boundary and the invalid-member case's only member fails the first member check.

Evidence:
- P1 inserts a resolver call before brief.go:174, inside the loop after `validateBriefMember`.
  - `go test ./cmd/metasystem -run '^TestDispatchBriefBounds' -count=1 -timeout 40m -v` gives 22 PASS lines and no FAIL.
  - `go test ./internal/dispatch -run '^TestBrief' -count=1 -timeout 40m -v` gives 94 PASS lines and no FAIL.
- P3 inserts a resolver call before brief.go:184, after the digit check and before `strconv.ParseInt`. The same two commands give the same 22 and 94 PASS lines.
- The wrong results these mutations would ship, for a brief run outside Git:
  - Under P1, a valid member with a bad Ceiling gets the prefix error instead of BRIEF_BOUNDS_INVALID. So does a pattern-invalid member such as `metasystem/a[`.
  - Under P3, an overflowing Ceiling gets the prefix error.
- P1 is the in-loop half of 1a's shape. At HEAD, `ProjectBriefBoundary` ran inside the loop right after `validateBriefMember` (HEAD brief.go:157 to 160). Round 3 catches the other half, the prefix check before the loop (HEAD brief.go:153), with row 2.
- One added case would close both gaps. In my copy only, I added this entry to the ordering table:
  `{"probe-valid-member-overflow-ceiling", "Working Mode: implement\nBoundary: [\"metasystem/x\"]\nCeiling: 9223372036854775808", "BRIEF_BOUNDS_INVALID: Ceiling: expected a nonnegative decimal integer", false}`
  It passes unmutated. It fails at :86 with the prefix error under P1 and under P3 (rows 6 to 8).

Why not material:
- The fold-3 brief asked for exactly two cases, "a valid Boundary with a malformed Ceiling such as `Ceiling: -1`" and "a Boundary with an invalid member such as `["/abs"]`", and for the two proofs at brief.go:170 and :180. Both cases exist, and both proofs fail their named case (rows 2 and 3).
- The production code is correct. No test or gate got weaker; the ordering witness only grew.
- Every refusal class the rule names now has a case: partial pair, non-JSON Boundary, invalid member and malformed Ceiling.
- If the seat reads the proof rule as covering a lookup before every refusal check in parseBriefBounds, this finding becomes material. The artifact to change would be cmd/metasystem/dispatch_brief_bounds_test.go: one added case, as probed above.

### F-22. Severity low. Material no.
Claim: the three new cases add no new dependence on GIT_DIR or TMPDIR, and with this toolchain the temporary-directory half of F-16 follows GOTMPDIR rather than TMPDIR.

Evidence:
- GIT_DIR set to the copy's own `.git`, no mutation:
  - The round-3 file fails `root-prefix` at :51 and `valid-pair-uses-prefix` at :85, both `exit 0, stdout "implement\n", stderr ""`.
  - The round-2 file fails the same two cases at :50 and :82, with the same text.
  - The three new cases pass in both.
  - F-16 named only the ordering case. The Admission test's negative `root-prefix` case also fails under GIT_DIR, and it already did in round 2.
- Under GIT_DIR the new cases lose their proof. M1 no longer fails `root-prefix-admitted`, and M3 no longer fails `malformed-ceiling-before-prefix`. The run still fails, on the same two older cases, so the loss is not silent (rows 11 and 12).
- TMPDIR inside a fresh Git repository, with GOTMPDIR outside Git:
  - The round-3 file gives 22 PASS lines and the round-2 file 19.
  - M1 and M3 are still caught (rows 14 and 15).
  - Round 2's own repository, `<S>/u1bi-gittmp`, gives the same result. `GIT_TRACE` shows one `git rev-parse --show-prefix` and no setup line, so Git found no repository.
- A throwaway probe test in copy2 logged `TMPDIR="/tmp/opus-u1bi3-gittmp" os.TempDir="/tmp/opus-u1bi3-gittmp" t.TempDir="/tmp/opus-u1bi3-tmp/TestZZOpusProbeTempDir4039428287/001"`.
  - So `t.TempDir()` sits under GOTMPDIR.
  - F-16's TMPDIR half therefore applies to GOTMPDIR when it is set, and to TMPDIR only when GOTMPDIR is unset.
  - That holds for the older ordering cases as much as for the new ones.
  - The probe file was removed and copy2 checked clean.
- `root-prefix-admitted` passes an absolute `--root` and runs from the package directory, so where the temporary directory sits does not affect it.

## Checks

1. Round 3 changed only the new test file.
   - Five of the six live hashes equal round 2's `<S>/u1bi-live.sha`. The test file went from `f5f6b360...` to `a7f6c5ad...`.
   - `shasum -a 256 -c <S>/u1bi-prod-before-fold3.sha`, run from the worktree root, prints OK for all three production files at the start and at the end.
   - Compared with round 2's test file (`<S>/u1bi-copy`, hash `f5f6b360...`), round 3 adds three lines, at :45, :73 and :74, and removes none (`<S>/u1bi3-round3-test.diff`).
   - `git status --porcelain -uall` lists exactly the six paths. The brief and result files are ignored by `metasystem/.gitignore:1` (`artifacts/`).
   - No existing case's input or expectation changed:
     - The negative branch at :50 matches `tc.name == "root-prefix"` exactly, so `root-prefix-admitted` takes the positive check at :53.
     - The dispatch.sh `--root` check at :56 still applies only to `root-prefix`.
     - The new case body at :45 is byte-identical to round 1's case (`<S>/bdrb-u1b-code.diff:50`). Only its name differs.
   - Numstat: 96+0, 14+6, 63+33, 66+2, 18+4 and 3+3, which is 308, under 400.
2. F-11 is closed. In row 1, M1 fails `root-prefix-admitted` at :53 and nothing else. The kept negative case still passes under M1.
3. F-12 is closed.
   - Row 2: M2 fails `invalid-member-before-prefix`. It also fails `malformed-ceiling-before-prefix`, because that lookup also runs before the Ceiling check.
   - Row 3: M3 fails `malformed-ceiling-before-prefix` only.
   - The remaining placements are F-21.
4. The new cases pass on the unmutated copy: 22 PASS lines, 19 subtests and 3 tests. `gofmt -l cmd/metasystem` prints nothing and `go vet ./cmd/metasystem` exits 0. Environment dependence is F-22.
5. The builder's result holds. These all match what I observed:
   - the case lines (:45, :73, :74);
   - the failure lines (:53, :85) and their stderr text;
   - the three sha256 pairs;
   - the 308 total.
   The result describes the F-12 proofs as moving the lookup, which is how the fold brief worded them. My insertions, in the round-2 read's form, give the same lines.

## Mutation table

Every row ran in a private copy and restored cleanly. Line numbers without a file name are in dispatch_brief_bounds_test.go. Abbreviations:
- CP is `go test ./cmd/metasystem -run '^TestDispatchBriefBounds' -count=1 -timeout 40m`, with -v except in row 1.
- DP is `go test ./internal/dispatch -run '^TestBrief' -count=1 -timeout 40m -v`.
- ORD is `go test ./cmd/metasystem -run '^TestDispatchBriefBoundsPrefixLookupOrdering$' -count=1 -timeout 40m -v`.
- LOOKUP is `if _, err := resolveInstallPrefix(); err != nil { return BriefBounds{}, err }`.

| Row | Mutation | Command | Observed failure line |
| --- | --- | --- | --- |
| 1 | dispatch_verbs.go:2028 passes `"."` instead of `*root`, plus `_ = root` | CP | `root-prefix-admitted`: `dispatch_brief_bounds_test.go:53: exit 1, stdout "", stderr "BRIEF_BOUNDS_INVALID: Boundary: invalid path or pattern \"metasystem/internal/dispatch/brief.go\"\n"`. No other failure. |
| 2 | LOOKUP inserted before brief.go:170 | CP | `invalid-member-before-prefix` and `malformed-ceiling-before-prefix`: `dispatch_brief_bounds_test.go:85: exit 1, stdout "", stderr "brief admission cannot resolve installation prefix: exit status 128\n"` |
| 3 | LOOKUP inserted before brief.go:180 | CP | `malformed-ceiling-before-prefix` only, with the same :85 line |
| 4 | P1: LOOKUP inserted before brief.go:174, inside the member loop | CP, DP | none: 22 and 94 PASS lines |
| 5 | P3: LOOKUP inserted before brief.go:184 | CP, DP | none: 22 and 94 PASS lines |
| 6 | Probe case added to the ordering table (valid member, Ceiling 9223372036854775808), no production change | ORD | none: 8 PASS lines |
| 7 | Probe case plus P1 | ORD | `probe-valid-member-overflow-ceiling`: `dispatch_brief_bounds_test.go:86: exit 1, stdout "", stderr "brief admission cannot resolve installation prefix: exit status 128\n"` |
| 8 | Probe case plus P3 | ORD | the same :86 line |
| 9 | none; GIT_DIR set; round-3 test file | CP | `root-prefix`: `:51: exit 0, stdout "implement\n", stderr ""`; `valid-pair-uses-prefix`: `:85`, with the same text |
| 10 | none; GIT_DIR set; round-2 test file | CP | `root-prefix` at `:50` and `valid-pair-uses-prefix` at `:82`, with the same text |
| 11 | Row 1 under GIT_DIR | CP | only the two row-9 lines; `root-prefix-admitted` passes |
| 12 | Row 3 under GIT_DIR | CP | only the two row-9 lines; `malformed-ceiling-before-prefix` passes |
| 13 | none; TMPDIR in a fresh Git repository; round-3 and round-2 files | CP | none: 22 and 19 PASS lines |
| 14 | Row 1 under that TMPDIR | CP | the row-1 line at :53 |
| 15 | Row 3 under that TMPDIR | CP | the row-3 line at :85 |
| 16 | none; TMPDIR=`<S>/u1bi-gittmp`; GIT_TRACE set | ORD | none: 7 PASS lines; the trace shows one `rev-parse --show-prefix` and no setup line |

Where the logs are:
- rows 1 to 5: `<S>/u1bi3-mut-logs/`;
- rows 6 to 8: `<S>/u1bi3-harden-*.log`;
- rows 9 to 15: `<S>/u1bi3-env-*.log`;
- row 16: `<S>/u1bi3-trace-*.out`.
