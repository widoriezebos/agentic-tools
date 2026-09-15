VERDICT: land

### F-51. Severity low. Material no.
Claim: round 3 rewrote the live brief.go while proving its mutations, instead of working in a disposable copy, but the file's bytes still equal the pre-round hash, so nothing that ships or certifies changed.

Evidence:
- brief.go's mtime is 2026-09-15T03:37:48+0200.
  - That is after the fold brief and the seat's sha file, both at 03:35:23.
  - It is also after the test file's last write, at 03:36:10.
- Result-3 says "Each mutation was applied alone to `metasystem/internal/dispatch/brief.go`" and names no copy.
  - Result-2 named its copies (`/tmp/codex-u1bii-mut-fold2.*`).
  - No round-3 mutation copy exists under /tmp, and /tmp/codex-u1bii-tmp is empty.
- The file's bytes are unchanged.
  - Run from the live worktree root, `shasum -a 256 -c` against u1bii-prod-before-fold3.sha prints `metasystem/internal/dispatch/brief.go: OK`, at the start of this read and at its end.
  - `cmp` against round 2's brief.go in /tmp/opus-u1bii2-copy exits 0.
- The fold brief asked for "restoring brief.go and checking its sha256 after each" and did not require a copy, so no brief rule is broken.
  - The only exposure is to anything else that read the worktree between 03:36 and 03:42.

### Mutation table

Copy and environment:
- /tmp/opus-u1bii4-copy is `git archive HEAD` of the worktree, committed.
  - Its tree 074642243c9a... equals the worktree's `HEAD^{tree}` at base fad3ace8.
  - The two live files were then copied in, checked OK with `shasum -a 256 -c`, and committed.
- Mutations ran in /tmp/opus-u1bii4-mut, a `cp -Rp` of that copy. It was sha-checked before the first mutation and after each restore.
- Environment:
  - GOCACHE=/tmp/opus-u1bii4-gocache.
  - GOTMPDIR and TMPDIR=/tmp/opus-u1bii4-tmp. /tmp is not inside a Git repository.
  - METASYSTEM_BIN, GIT_DIR and METASYSTEM_CONTEXT_COST_PROOF were unset.
  - Every go test used -count=1, -timeout 40m and a focused -run on ./internal/dispatch.
  - No race suite and no fixture bed ran.

Check 1, scope:
- Live hashes at the start of this read (01:44:51Z) and at its end (01:49:55Z) are the same: brief.go 2ca4d47d..., brief_authority_test.go 34802505....
  - `git status --porcelain --untracked-files=all` lists only those two files as modified.
  - No worktree file was written after this read started.
- `diff` from the round-2 test file to the live test file prints only `269a270,272`: three added lines, none removed or changed.
  - The round-2 file is /tmp/opus-u1bii2-copy, sha256 c57b399d..., the hash read 2 recorded for the live file.
  - Every other subtest is therefore unchanged and none was removed.
- The only worktree files written after the fold brief are brief.go (see F-51), brief_authority_test.go and result-3.
- `git diff --numstat HEAD` gives brief.go 117/17 and brief_authority_test.go 187/16, 337 changed lines in total. That is within 400. Round 2 was 334, so round 3 added exactly 3 lines.

Check 2, restored rows:
- brief_authority_test.go:270-272 holds three rows:
  - `{"pattern-no-equality", "new/*.md", []string{"Read `new/*.md`."}, nil}`
  - `{"backslash-no-equality", `new/a\b.md`, []string{"Read " + jsonString(`new/a\b.md`) + "."}, nil}`
  - `{"directory-no-equality", "new/file.md/", []string{"Read `new/file.md/`."}, nil}`
- The whole TestBriefAuthorityBoundaryInputStillRequired body (live lines 261-284) matches round 1's body. `diff` exits 0.
  - Round 1's copy is /tmp/opus-u1bii-copy, brief_authority_test.go sha256 39537be0..., lines 231-254.
- The live test file is byte-identical to read 2's witness copy (/tmp/opus-u1bii3-witness). `cmp` exits 0.
- The design page's witness names at plans/brief-declares-the-round-boundary-design.md:568-570 match these subtests.

Result-3's claims:
- These hold:
  - the restored names and lines 270-272;
  - brief.go's before and after hash;
  - the 337-line total;
  - all eight A4 subtests running unmutated;
  - mutation definitions identical to round 2's (result-2 and read-2's table).
- One claim holds for content only: result-3 says brief.go was byte-identical after each restore, which is true. Its mtime changed (F-51).

Checks 3 and 4 are in the table. gofmt -l on internal/dispatch prints nothing and go vet ./internal/dispatch exits 0 on the unmutated copy.

| Mutation | Change in the copy's brief.go | Run | Exit | Observed |
| --- | --- | --- | --- | --- |
| none | none | `-run '^TestBriefAuthorityBoundaryInputStillRequired$/^pattern-no-equality$' -v` | 0 | `--- PASS: TestBriefAuthorityBoundaryInputStillRequired/pattern-no-equality (0.37s)`, no "no tests to run" |
| none | none | `-run '^TestBriefAuthorityBoundaryInputStillRequired$/^backslash-no-equality$' -v` | 0 | `--- PASS: TestBriefAuthorityBoundaryInputStillRequired/backslash-no-equality (0.32s)`, no "no tests to run" |
| none | none | `-run '^TestBriefAuthorityBoundaryInputStillRequired$/^directory-no-equality$' -v` | 0 | `--- PASS: TestBriefAuthorityBoundaryInputStillRequired/directory-no-equality (0.35s)`, no "no tests to run" |
| none, control name | none | `-run '^TestBriefAuthorityBoundaryInputStillRequired$/^no-such-row-control$' -v` | 0 | `testing: warning: no tests to run` and `ok ... [no tests to run]`, which shows the named runs above ran real subtests |
| none | none | `-run '^TestBriefAuthority' -v` | 0 | 75 `--- PASS` lines and 0 `--- FAIL`, all eight A4 subtests ran and passed, `ok ... 17.984s` |
| A4.glob-treated-concrete | brief.go:408 `!briefBoundaryIsPattern(member)` becomes `!strings.ContainsAny(member, "?[]\\")` | `-run '^TestBriefAuthorityBoundaryInputStillRequired$/^pattern-no-equality$' -v` | 1 | `brief_authority_test.go:281: authority error = &dispatch.BriefAuthorityRefusal{MissingPaths:[]string{"new/*.md"}}, want missing paths []` and `--- FAIL: TestBriefAuthorityBoundaryInputStillRequired/pattern-no-equality`. In the whole A4 run under the same mutation only pattern-no-equality fails. Restore checked OK. |
| A4.backslash-treated-concrete | brief.go:408 `!briefBoundaryIsPattern(member)` becomes `!strings.ContainsAny(member, "*?[]")` | `-run '^TestBriefAuthorityBoundaryInputStillRequired$/^backslash-no-equality$' -v` | 1 | `brief_authority_test.go:281: authority error = &dispatch.BriefAuthorityRefusal{MissingPaths:[]string{"new/a\\b.md"}}, want missing paths []` and `--- FAIL: TestBriefAuthorityBoundaryInputStillRequired/backslash-no-equality`. In the whole A4 run under the same mutation only backslash-no-equality fails. Restore checked OK. |
| A4.directory-treated-concrete | brief.go:407 `!strings.HasSuffix(member, "/")` becomes `true` | `-run '^TestBriefAuthorityBoundaryInputStillRequired$/^directory-no-equality$' -v` | 1 | `brief_authority_test.go:281: authority error = &dispatch.BriefAuthorityRefusal{MissingPaths:[]string{"new/file.md/"}}, want missing paths []` and `--- FAIL: TestBriefAuthorityBoundaryInputStillRequired/directory-no-equality`. In the whole A4 run under the same mutation only directory-no-equality fails. Restore checked OK. |

Material findings: 0.
