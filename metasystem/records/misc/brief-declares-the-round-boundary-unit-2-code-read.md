VERDICT: send back

# Opus closing read: brief-declares-the-round-boundary, unit 2 (count fixed-tree text changes)

Reader: Opus, independent of the builder. Material findings: 2 (F-1, F-2).

Setup. The live worktree /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/.claude/worktrees/bdrb-u2 (base 95d5032a) was never modified. All probes and mutations ran in two private copies, SCRATCH/opus-u2b-copy and SCRATCH/opus-u2b-copy2, where SCRATCH is /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/36c93128-ec16-4146-8def-3f706ba4ef10/scratchpad. Each copy is a git archive of HEAD plus the two new files. sha256 of numstat.go is b14bf714199c1fca8569ddc45c66623f451111b58045be072f4a23e2d6cce0b5 and of numstat_test.go is ee90cf41b207d8ded27c8c1440c11adda64dee592018e7905890f92a4a282965. Those hashes matched in the live worktree at the start, in both copies, in both copies after every mutation was restored, and in the live worktree at the end. Environment: GOCACHE=/tmp/opus-u2b-gocache, GOTMPDIR and TMPDIR=/tmp/opus-u2b-tmp (confirmed outside any repository), with GIT_DIR, METASYSTEM_BIN and METASYSTEM_CONTEXT_COST_PROOF unset. Every go test used -timeout 40m and a focused -run on ./internal/gittree. Tools were git 2.50.1 (Apple Git-155) and go1.27.1. No fixture beds ran. A stale SCRATCH/u2copy from the earlier cut-off read was ignored.

## Conformance

- Diff against 95d5032a has two intent-to-add files: numstat.go with 91 lines and numstat_test.go with 186, so 277 additions and 0 deletions, under the Ceiling of 400. `git diff --quiet 95d5032a -- metasystem/internal/gittree/gittree.go` succeeds, so gittree.go is unchanged.
- The only untracked files are ignored: artifacts/reports/codex-bdrb-u2-brief.md and codex-bdrb-u2-result.md (ignored by metasystem/.gitignore `artifacts/`), and bin/metasystem (modified 02:30:34, after the result file at 02:26:16). None counts as a changed path. No Boundary breach.
- Non-goals hold. The runner, its config pins and its environment scrubbing are untouched. ChangedLines calls `w.git(nil, ...)` at numstat.go:12, so there is no second production runner. A grep for ChangedLines and changedLinesFromNumstat in *.go and *.sh finds no caller of the gittree method outside numstat_test.go. The other ChangedLines hits are unrelated fields in internal/metrics and internal/landing.
- The argv at numstat.go:12-14 is exactly `diff --numstat -z --no-renames --no-ext-diff --no-textconv --no-color --ignore-submodules=none <from> <to> --`, run through Workspace.git.
- The parser at numstat.go:25-91 follows the page. Empty output is zero. The final byte must be NUL. Records split at the first two tabs only. The path must be nonempty. Only exactly `-\t-` is skipped. Counts must be ASCII digits and then pass ParseInt. Row and total additions are checked. A Git error is wrapped and returned. In the copy, `gofmt -l internal/gittree` and `go vet ./internal/gittree` were both clean.
- The test names match the page's rows N1 to N5.

## Findings

### F-1. Severity medium. Material yes.

Claim: TestChangedLinesHostileDiffConfig fails only when `--no-renames` is removed. It stays green if ChangedLines bypasses Workspace.git, or if the runner's config pins or environment scrubbing are missing. So the N4.hostile-config rule ("existing runner integration") and the page's rule "Use Workspace.git, do not add another production Git runner" have no failing test.

Evidence:
- Mutation N4H-raw-runner (copy2): `w.git(nil, ` at numstat.go:12 replaced by a direct `exec.Command("git", "-C", dir, args...).Output()` helper, with no pins and the inherited environment. `go test -run '^TestChangedLines'` printed ok with rc=0, so all six test functions passed. TestChangedLinesCommandPins also passed, because numstat_test.go:128-139 compares argv only from "diff" onward and never sees `-C <dir>` or the config pins.
- Mutation N4H-no-configpins: gittree.go:122 changed to `full := append([]string{"-C", dir}, args...)`. `-run '^TestChangedLinesHostileDiffConfig$'` gave rc=0.
- Mutation N4H-no-scrub: gittree.go:124 changed to `cmd.Env = append(os.Environ(), env...)`. `-run '^TestChangedLinesHostileDiffConfig$'` gave rc=0.
- Mutation N4H-four-flags: `--no-ext-diff`, `--no-textconv`, `--no-color` and `--ignore-submodules=none` removed together. TestChangedLinesHostileDiffConfig, TestChangedLinesUsesSuppliedTrees and TestChangedLinesGitFailure all passed with rc=0. I rebuilt the test's hostile repository in raw git (SCRATCH/rawgit-probe.sh). Dropping any one of those four flags left the output byte-identical: `2\t1\tREADME.md\0 1\t0\tnew.txt\0 0\t1\told.txt\0`. The hostile driver's log file was never written. Under git 2.50.1, numstat never runs diff.external, diff.<driver>.command or textconv, and never colors its output. diff.ignoreSubmodules=all has no effect because the fixture has no gitlink. The only live hostile setting is diff.renames, and N3.rename already witnesses that. The builder's own hostile-config mutation row also removes `--no-renames`.
- A witness fits inside the Boundary. In the copy, one test plants `git replace <README blob> <10-line blob>` and expects 3. It passes on the unit as built. Under N4H-raw-runner and N4H-no-configpins it fails with `opus_probe_test.go:181: ChangedLines(...) = 11, <nil>; want 3, nil`. A second test runs `t.Setenv("GIT_CONFIG_PARAMETERS", "'core.bigfilethreshold'='1'")` and fails under N4H-raw-runner and N4H-no-scrub with `opus_probe_test.go:189: ChangedLines(...) = 0, <nil>; want 3, nil`. A gitlink probe shows the submodule pin is observable once a gitlink exists: under diff.ignoreSubmodules=all, `1\t1\tsub` counts 2 with the flag, and without the flag the output is empty.
- Artifact it would change: numstat_test.go. The hostile test should plant configuration or environment that only the runner neutralizes, such as a replace ref and GIT_CONFIG_PARAMETERS. A gitlink under diff.ignoreSubmodules=all would also make one of its existing settings live.

### F-2. Severity low. Material yes.

Claim: No test feeds a NUL-terminated record with fewer than two tabs. The one-tab incomplete-record guard at numstat.go:40-42 can be removed with the whole suite green, and the parser then panics instead of returning an error.

Evidence:
- Mutation N2-onetab-guard: `if secondOffset < 0 {` changed to `if false {`. `go test -run '^TestChangedLines'` gave rc=0 with every test passing.
- With the same mutation, the probe input `"1\t2\x00"` printed `PARSE one-tab PANIC runtime error: slice bounds out of range [2:1]`. With the guard in place, the same probe returns `numstat record 1 has no deleted count`.
- Every input in numstat_test.go:17-58 has at least two tabs, and missing-nul (numstat_test.go:34) covers only an unterminated record. Both the page's "Reject incomplete records" and the brief's proof rule ("for every rule you add, a test that fails when that rule alone is removed") apply.
- The no-tab guard (numstat.go:36-38) and the empty-decimal guard (numstat.go:78-80) are behaviourally redundant. Removing either one still rejects the same inputs through the next check: N2-notab-guard and N2-emptydecimal-guard pass, and the probes `12\0`, `\0` and `1\t\tf\0` still return errors. They need no separate witness.
- Artifact it would change: numstat_test.go. Add a one-tab record such as `"1\t2\x00"` to an N2 subtest.

### F-3. Severity medium. Material no.

Claim: Even with the page's exact argv and the runner's pins, diff attributes and configuration can still change the count. A text change counts zero under `diff.<driver>.binary=true`, under `core.bigFileThreshold=1`, or when the candidate itself adds a `.gitattributes` with `* -diff`. `diff.relative=true` undercounts from a nested workspace.

Evidence (probes through Workspace.ChangedLines in the copy, test file saved at SCRATCH/opus-u2b-probe/opus_probe_test.go):
- `.gitattributes` `README.md diff=hostile` plus `diff.hostile.binary true`, with README changed from the fixture line to two lines: got=0 with raw `-\t-\tREADME.md\0`. The real count is 3.
- `core.bigFileThreshold 1`: got=0 with raw `-\t-\tREADME.md\0`. The real count is 3.
- The candidate adds `sub/.gitattributes` (`* -diff`) and `sub/code.go` (50 lines): got=0 with raw `-\t-\tsub/.gitattributes\0-\t-\tsub/code.go\0`. The real count is 51. This case needs no configuration access, only a Boundary that covers the directory.
- `diff.relative true` with Workspace{Dir: nested}: got=3 with raw `2\t1\tfile.txt\0`. The same trees counted from the top give 6.
- Why this is not material for this unit: the page fixes the argv exactly and says binary contents cost zero. Any fix, such as `--no-relative`, an attribute source, or a `core.bigFileThreshold` pin, changes either the page's argv or the runner, and the brief forbids both. The implementation conforms. The seat should route this to the design page before unit 4c uses the count for the Ceiling, because N4.hostile-config's own wording ("Real counts remain correct under hostile diff configuration") does not hold for diff.<driver>.binary.

### F-4. Severity low. Material no.

Claim: ChangedLines does not validate its tree arguments. Git parses an option-shaped argument as an option, and the call returns zero with no error.

Evidence: the probe `f.w.ChangedLines("--output="+sink, to)` returned `got=0 err=<nil>`. Empty arguments do fail, with `gittree changed lines: git diff: fatal: bad revision ''`. This is not material. The page does not require validation. The planned callers pass hex ids from Snapshot and TreeOf, which check treeID (gittree.go:212 and gittree.go:273). Diff and ChangedPaths already use the same argument shape.

### F-5. Severity low. Material no.

Claim: The builder's mutation table quotes seven N4 argv failure lines that the final test cannot print, so those rows are not verbatim observations of the test as it stands.

Evidence: in codex-bdrb-u2-result.md, rows N4.nul through N4.end-options quote text such as `numstat_test.go:151: diff argv omitted "-z"` and `diff argv ended at "to-tree"; want final "--"`. The only Fatalf at numstat_test.go:151 is `t.Fatalf("diff argv = %q; want %q", got, want)`. I re-ran all eight argv mutations, and each fails at line 151 with the full argv printed. Every N4 argv rule is therefore witnessed, and only the quoted evidence is stale. The seat should not copy those lines into a receipt as observed output.

## Probe table

Parser probes call changedLinesFromNumstat directly (TestOpusProbeParser). Real-Git probes call Workspace.ChangedLines on temporary repositories under /tmp/opus-u2b-tmp.

| Probe | Input | Observed | Conforms |
| --- | --- | --- | --- |
| tab in path | `4\t5\tpa\tth\0` | 9, nil | yes |
| newline in path | `4\t5\tpa\nth\0` | 9, nil | yes |
| CR in path | `1\t2\tf\r\0` | 3, nil | yes |
| leading plus | `+1\t0\tf\0` | error: "+1" is not a nonnegative decimal | yes |
| leading zeros | `007\t0010\tf\0` | 17, nil | yes. A valid decimal form that Git never emits; the count stays right. Not a finding. |
| leading space | ` 1\t2\tf\0` | error | yes |
| non-ASCII digit | U+0661 as the added count | error | yes |
| empty deleted field | `1\t\tf\0` | error: empty decimal | yes |
| trailing tab | `1\t2\tfile\t\0` | 3, nil (path is "file\t") | yes, split at the first two tabs only |
| MaxInt64 count | `9223372036854775807\t0\tf\0` | 9223372036854775807, nil | yes |
| total exactly MaxInt64 | `9223372036854775806\t0\ta\0` then `1\t0\tb\0` | 9223372036854775807, nil | yes |
| huge count | `99999999999999999999\t0\tf\0` | error: outside int64 | yes |
| binary row with path | `-\t-\tbin.dat\0` | 0, nil | yes |
| binary row without path | `-\t-\t\0` | error: empty pathname | yes |
| binary then text | `-\t-\tbin\0` then `1\t1\tf\0` | 2, nil | yes |
| double dash | `--\t-\tf\0` | error: mixes binary and numeric | yes |
| zero row | `0\t0\tempty\0` | 0, nil | yes |
| no final NUL | `1\t2\tf` | error: not NUL-terminated | yes |
| last record unterminated | `1\t2\tf\0` then `1\t2\tg` | error: not NUL-terminated | yes |
| one tab | `1\t2\0` | error: no deleted count. Panics once the guard is removed (F-2). | yes, but no test |
| no tab | `12\0` | error: no added count | yes |
| lone NUL | `\0` | error | yes |
| trailing empty record | `1\t2\tf\0\0` | error at record 2 | yes |
| rename-shaped output | `0\t0\t\0old\0new\0` | error: empty pathname | yes |
| real Git, identical trees | the same tree twice | 0, nil, raw empty | yes |
| real Git, mixed candidate | filename with tab and newline (2 lines), binary blob, empty file, symlink, README losing its final newline | 5, nil. Raw rows: `1\t1\tREADME.md`, `-\t-\tblob.bin`, `0\t0\tempty.txt`, `1\t0\tlink`, `2\t0\twe\tird\nname.txt` | yes |
| real Git, mode only | chmod 755 on README.md | 0, nil, raw `0\t0\tREADME.md` | yes |
| real Git, gitlink under diff.ignoreSubmodules=all | gitlink 1111... changed to 2222... | 2, nil. Without `--ignore-submodules=none` the raw output is empty. | yes |
| real Git, foreign GIT_DIR in the environment | GIT_DIR pointing at another repository | 3, nil (the runner scrubs it) | yes |
| real Git, replace ref | README blob replaced | 3 through the runner, 11 without the pins (F-1) | yes |
| real Git, GIT_CONFIG_PARAMETERS bigfile | environment injection | 3 through the runner, 0 without the scrub (F-1) | yes |
| driver binary, bigFileThreshold, candidate .gitattributes, diff.relative from nested | see F-3 | 0, 0, 0, 3 | page-level gap (F-3) |
| option-shaped tree | `--output=<file>` as fromTree | 0, nil (F-4) | the page is silent |
| empty tree arguments | "" and "" | error: bad revision '' | yes |

## Mutation table

Each mutation was applied in a private copy and run with `go test -vet=off -count=1 -timeout 40m -run <pattern> ./internal/gittree`. The file was then restored from the sha-checked pristine copies. Afterwards both copies re-hashed identical and the restored suite printed ok. Logs are in SCRATCH/opus-u2b-mutA.log and SCRATCH/opus-u2b-mutB.log, with full output per mutation in SCRATCH/opus-u2b-mut-<id>.out.

| Id | Row | Change | Run | Result | Failure line |
| --- | --- | --- | --- | --- | --- |
| N1-adds | N1.adds-deletes | `row := added + deleted` to `row := added` | ^TestChangedLines$ | FAIL | numstat_test.go:26: changedLinesFromNumstat() = 2, <nil>; want 5, nil |
| N1-empty | N1.empty | empty output returns an error | ^TestChangedLines$ | FAIL | numstat_test.go:26: changedLinesFromNumstat() = 0, empty; want 0, nil |
| N1-binary | N1.binary | `continue` replaced by an error return | ^TestChangedLines$ | FAIL | numstat_test.go:26: changedLinesFromNumstat() = 0, binary; want 0, nil |
| N1-oddpaths | N1.odd-paths | IndexByte to LastIndexByte for the second tab | ^TestChangedLines$ | FAIL | numstat_test.go:26: invalid deleted count: "5\ta" is not a nonnegative decimal; want 9, nil |
| N2-missingnul | N2.missing-nul | final-NUL check disabled | ^TestChangedLinesRejectsMalformedAndOverflow$/^missing-nul$ | FAIL | numstat_test.go:34: changedLinesFromNumstat("1\t2\tfile.txt") = 3, <nil>; want error containing "not NUL-terminated" |
| N2-missingpath | N2.missing-path | empty-path check disabled | .../^missing-path$ | FAIL | numstat_test.go:37: changedLinesFromNumstat("1\t2\t\x00") = 3, <nil> |
| N2-count-digits | N2.count | digit loop disabled | .../^count$ | FAIL | numstat_test.go:45: changedLinesFromNumstat("+1\t0\tfile\x00") = 1, <nil> |
| N2-count-parseint | N2.count | ParseInt error ignored | .../^count$ | FAIL | numstat_test.go:45: changedLinesFromNumstat("9223372036854775808\t0\tfile\x00") = 9223372036854775807, <nil> |
| N2-mixed | N2.mixed-binary | mixed dash and numeric check disabled | .../^mixed-binary$ | FAIL | numstat_test.go:50: changedLinesFromNumstat("-\t1\tfile\x00") = 0, <nil> |
| N2-rowoverflow | N2.row-overflow | row addition check disabled | .../^row-overflow$ | FAIL, by message only, because the total check still rejects the wrapped row | numstat_test.go:54: numstat total count overflow at record 1; want error containing "record 1 count overflow" |
| N2-totaloverflow | N2.total-overflow | total addition check disabled | .../^total-overflow$ | FAIL | numstat_test.go:58: = -9223372036854775808, <nil> |
| N2-onetab-guard | N2 incomplete record | `if secondOffset < 0` disabled | ^TestChangedLines | PASS (F-2) | none. Probe `1\t2\0` panics: slice bounds out of range [2:1] |
| N2-notab-guard | redundant guard | `if firstTab < 0` disabled | ^TestChangedLines | PASS | behaviour unchanged, the next guard rejects |
| N2-emptydecimal-guard | redundant guard | `if len(field) == 0` disabled | ^TestChangedLines | PASS | behaviour unchanged, ParseInt rejects |
| N3-root-worktree | N3.root | toTree dropped, so the diff runs against the worktree | ^TestChangedLinesUsesSuppliedTrees$/^root$ | FAIL | numstat_test.go:76: ChangedLines(...) = 4, <nil>; want 3, nil |
| N3-nested-head | N3.nested | toTree replaced by "HEAD" | .../^nested$ | FAIL | numstat_test.go:87: ChangedLines(...) = 0, <nil>; want 3, nil |
| N3-rename | N3.rename | `--no-renames` dropped | .../^rename$ | FAIL | numstat_test.go:98: numstat record 1 has an empty pathname; want 4, nil |
| N4-numstat-pin | N4.numstat | `--numstat` dropped | ^TestChangedLinesCommandPins$/^numstat$ | FAIL | numstat_test.go:151: diff argv = ["diff" "-z" ...]; want ["diff" "--numstat" "-z" ...] |
| N4-nul-pin | N4.nul | `-z` dropped | .../^nul$ | FAIL | numstat_test.go:151: diff argv = [...]; want [...] |
| N4-renames-pin | N4.no-renames | `--no-renames` dropped | .../^no-renames$ | FAIL | numstat_test.go:151 |
| N4-extdiff-pin | N4.no-ext-diff | `--no-ext-diff` dropped | .../^no-ext-diff$ | FAIL | numstat_test.go:151 |
| N4-textconv-pin | N4.no-textconv | `--no-textconv` dropped | .../^no-textconv$ | FAIL | numstat_test.go:151 |
| N4-color-pin | N4.no-color | `--no-color` dropped | .../^no-color$ | FAIL | numstat_test.go:151 |
| N4-submodules-pin | N4.submodules | `--ignore-submodules=none` dropped | .../^submodules$ | FAIL | numstat_test.go:151 |
| N4-endopt-pin | N4.end-options | final `--` dropped | .../^end-options$ | FAIL | numstat_test.go:151: argv ends at "to-tree" |
| N4H-no-renames | N4.hostile-config | `--no-renames` dropped | ^TestChangedLinesHostileDiffConfig$ | FAIL | numstat_test.go:178: numstat record 2 has an empty pathname; want 5, nil |
| N4H-four-flags | N4.hostile-config | the other four flags dropped together | ^TestChangedLines(UsesSuppliedTrees\|HostileDiffConfig\|GitFailure)$ | PASS (F-1) | none |
| N4H-raw-runner | N4.hostile-config and "Use Workspace.git" | direct exec with no pins and no scrub | ^TestChangedLines | PASS (F-1) | none |
| N4H-raw-runner-witness | reader's witness | same | ^TestOpusProbeWitness$ | FAIL | opus_probe_test.go:181: = 11, <nil>; want 3. opus_probe_test.go:189: = 0, <nil>; want 3 |
| N4H-no-configpins | N4.hostile-config | gittree.go:122 pins dropped | ^TestChangedLinesHostileDiffConfig$ | PASS (F-1) | none |
| N4H-no-configpins-witness | reader's witness | same | ^TestOpusProbeWitness$/^replace-ref$ | FAIL | opus_probe_test.go:181: ChangedLines(...) = 11, <nil>; want 3, nil |
| N4H-no-scrub | N4.hostile-config | gittree.go:124 scrub dropped | ^TestChangedLinesHostileDiffConfig$ | PASS (F-1) | none |
| N4H-no-scrub-witness | reader's witness | same | ^TestOpusProbeWitness$/^config-env$ | FAIL | opus_probe_test.go:189: ChangedLines(...) = 0, <nil>; want 3, nil |
| N5-gitfailure | N5.git-failure | Git error path returns 0, nil | ^TestChangedLinesGitFailure$ | FAIL | numstat_test.go:184: ChangedLines with invalid tree = 0, <nil>; want 0 and an error |

Rules with no failing test: the runner integration that N4.hostile-config claims, together with the page's "Use Workspace.git" (F-1), and rejection of a record with only one tab (F-2). Every other N1 to N5 row has a test that fails when that rule alone is removed.
