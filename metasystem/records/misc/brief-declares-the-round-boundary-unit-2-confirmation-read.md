VERDICT: land

# Opus confirmation read: brief-declares-the-round-boundary, unit 2, fold round 1

Reader: Opus, independent of the builder. Scope: whether the fold closed F-1 and F-2 of codex-bdrb-u2-read-1.md without changing anything else. Material findings: 0.

## Setup

- The live worktree /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/.claude/worktrees/bdrb-u2 (base 95d5032a) was never modified. Every probe and mutation ran in the private copy SCRATCH/opus-u2c-copy, where SCRATCH is /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/36c93128-ec16-4146-8def-3f706ba4ef10/scratchpad. The copy is a git archive of HEAD plus the two live files. The copy is not a Git repository.
- sha256 values: numstat.go b14bf714199c1fca8569ddc45c66623f451111b58045be072f4a23e2d6cce0b5, numstat_test.go 2fcf0a3265fb6bc97e3cda4da056aa975669583c23f24acef5c0258f3443e041, gittree.go 2f096374b67ebfa1f149b032c9d2233685114fd0fced0f3b8273042b4b3f708e. The same values held in the live worktree at the start (01:45:46 UTC), in the copy, in the restore store SCRATCH/opus-u2c-pristine, in the copy after every restore, and in the live worktree at the end (01:54:06 UTC). The live mtimes also did not change during this read.
- Environment: GOCACHE=/tmp/opus-u2c-gocache, GOTMPDIR and TMPDIR=/tmp/opus-u2c-tmp (not inside any Git repository), with METASYSTEM_BIN, METASYSTEM_CONTEXT_COST_PROOF, GIT_DIR, GIT_WORK_TREE and GIT_INDEX_FILE unset. The tools were git 2.50.1 (Apple Git-155) and go1.27.1. Every go test used -count=1, -timeout 40m and a focused -run on ./internal/gittree. No fixture beds ran. Load average was about 15.

## Check 1. Only numstat_test.go changed, and the unit is within the Ceiling

- numstat.go: `shasum -a 256 -c u2-prod-before-fold.sha`, run from the live root, printed OK at the start and at the end. The file is also byte-identical (cmp) to the pre-fold copy SCRATCH/opus-u2b-pristine/numstat.go.
- gittree.go: `git diff --quiet 95d5032a -- metasystem/internal/gittree/gittree.go` succeeds. Its hash equals the builder's before and after values.
- I ran `diff -u` from the pre-fold test file SCRATCH/opus-u2b-pristine/numstat_test.go (sha ee90cf41, the file the closing read hashed) to the live file. It shows two hunks with additions only, and no line was removed or edited. The first adds the one-tab subtest at numstat_test.go:39-41. The second appends the replace-ref and config-environment subtests at numstat_test.go:183-209, after the unchanged hostile assertion at line 181 that still expects 5. No existing input or expectation changed.
- `git diff --numstat 95d5032a` reports 91 and 217, so the unit is 308 lines and under 400. `git diff --cached --numstat` is empty. Both index entries are still the intent-to-add empty blob e69de29. `git status --short --ignored` shows only the two A files and the ignored bin/metasystem. That binary was written at 02:30:34, before the fold brief at 03:32:04.
- Outside artifacts/, only the three gittree files are newer than the fold brief. gittree.go (03:36:58) and numstat.go (03:37:22) have new mtimes because the builder applied, ran and restored the mutations the fold brief authorized. Their content is byte-identical to base.
- In the copy, `gofmt -l internal/gittree` is clean, and the unmutated baseline go test with the default vet checks returned ok.

## Check 2 and check 3. Mutations

See the mutation table. Each F-1 mutation now fails a named subtest of TestChangedLinesHostileDiffConfig. N2-onetab-guard fails TestChangedLinesRejectsMalformedAndOverflow/one-tab, because the case requires an error containing "no deleted count" and the mutated parser panics instead.

## Check 4. The new cases pass on the unmutated copy without global Git configuration or network

- Normal environment: `go test -count=1 -timeout 40m -run '^TestChangedLines' -v ./internal/gittree` returned rc=0. The one-tab, replace-ref and config-environment cases passed, along with every earlier case (SCRATCH/opus-u2c-baseline.log).
- Isolated environment: the same run used `-exec SCRATCH/opus-u2c-isolate.sh`. The wrapper sets HOME and XDG_CONFIG_HOME to the empty /tmp/opus-u2c-home and runs the test binary under sandbox-exec with `(deny network*)`. It returned rc=0, and all three new cases passed.
- Controls through the same wrapper: `git config --global --list` failed with "unable to read config file '/tmp/opus-u2c-home/.gitconfig'" (rc=128), and curl failed with "Could not resolve host" (rc=6).
- The three F-1 mutations fail the same way under the isolated wrapper, so the witnesses do not get their discriminating power from this machine's configuration.
- By inspection, the new cases use only local Git commands: init, add, commit, rev-parse, hash-object, replace and diff. The fixture supplies its own commit identity through -c and passes -b main to init. The CommandLineTools system gitconfig (credential.helper and init.defaultbranch) cannot be switched off through the environment, because ScrubbedEnviron strips every GIT_CONFIG* variable. Neither key affects these cases.

## Builder claims checked

- codex-bdrb-u2-result-2.md claims failure lines numstat_test.go:199 (count 11) and numstat_test.go:208 (count 0), the one-tab panic at numstat.go:44, the numstat.go and gittree.go hashes, and the 308-line total. All of these reproduce. I did not re-run the race run or go vet ./internal/gittree. The seat's go-gate covers them.
- Under N4H-raw-runner, the full `^TestChangedLines` run fails only the two new subtests. TestChangedLinesCommandPins still cannot see the runner, as the closing read noted, so the new subtests alone close F-1.

## Findings

None. F-1 is closed: N4H-raw-runner, N4H-no-configpins and N4H-no-scrub each fail a named case. F-2 is closed: N2-onetab-guard fails the one-tab case. The fold changed nothing else. Material findings: 0.

## Mutation table

Each mutation was applied alone in the copy. Its applied line was logged and checked before the run. Mutation runs used `go test -vet=off -count=1 -timeout 40m -v -run <pattern> ./internal/gittree`, and the rows marked isolated added `-exec SCRATCH/opus-u2c-isolate.sh`. After each mutation, the three files were restored from SCRATCH/opus-u2c-pristine, `shasum -a 256 -c` printed OK for all three, and no opus_* file remained. The log is SCRATCH/opus-u2c-mut.log, and the full output of each run is in SCRATCH/opus-u2c-mut-<id>.out or SCRATCH/opus-u2c-mut-<id>-iso.out.

| Id | Change (copy only) | Run pattern | Result | Failure line |
| --- | --- | --- | --- | --- |
| baseline | none | ^TestChangedLines | ok, rc=0 | none |
| baseline, isolated | none | ^TestChangedLines | ok, rc=0 | none |
| N4H-raw-runner | numstat.go:12 `w.git(nil, "diff"` became `opusRawGit(w.Dir, "diff"`. A copy-only file opus_rawgit.go returns `exec.Command("git", append([]string{"-C", dir}, args...)...).Output()`, with no config pins and the inherited environment. | ^TestChangedLines | FAIL, rc=1. replace-ref and config-environment fail. Every other ChangedLines test and subtest passes. | numstat_test.go:199: ChangedLines(a8a1e397a0312598833594e7c56d6b7fad39bcb5, 81040aa0ff257f44b88d66cc696e4cd61f3a7314) = 11, <nil>; want 3, nil. numstat_test.go:208: ChangedLines(15892e7e5ca82d5b42ef2fc9fb4e1d8494d0a9a7, 5bd297666a8d52558d5c81042814e44f8157c73f) = 0, <nil>; want 3, nil |
| N4H-raw-runner, isolated | same | ^TestChangedLinesHostileDiffConfig$ | FAIL, rc=1. Both subtests fail. | the same two lines |
| N4H-no-configpins | gittree.go:122 became `full := append([]string{"-C", dir}, args...)` | ^TestChangedLinesHostileDiffConfig$ | FAIL, rc=1. replace-ref fails and config-environment passes. | numstat_test.go:199: ChangedLines(a8a1e397a0312598833594e7c56d6b7fad39bcb5, 81040aa0ff257f44b88d66cc696e4cd61f3a7314) = 11, <nil>; want 3, nil |
| N4H-no-configpins, isolated | same | ^TestChangedLinesHostileDiffConfig$ | FAIL, rc=1, with the same split | the same line |
| N4H-no-scrub | gittree.go:124 became `cmd.Env = append(os.Environ(), env...)`. gittree.go:415 was confirmed unchanged. | ^TestChangedLinesHostileDiffConfig$ | FAIL, rc=1. config-environment fails and replace-ref passes. | numstat_test.go:208: ChangedLines(15892e7e5ca82d5b42ef2fc9fb4e1d8494d0a9a7, 5bd297666a8d52558d5c81042814e44f8157c73f) = 0, <nil>; want 3, nil |
| N4H-no-scrub, isolated | same | ^TestChangedLinesHostileDiffConfig$ | FAIL, rc=1, with the same split | the same line |
| N2-onetab-guard | numstat.go:40 `if secondOffset < 0 {` became `if false {` | ^TestChangedLinesRejectsMalformedAndOverflow$ | FAIL, rc=1. missing-nul and missing-path pass, and one-tab fails. The panic ends the test binary, so the later N2 subtests did not run in this invocation. They pass in both baselines. | --- FAIL: TestChangedLinesRejectsMalformedAndOverflow/one-tab. panic: runtime error: slice bounds out of range [2:1] [recovered, repanicked], at internal/gittree/numstat.go:44 |
