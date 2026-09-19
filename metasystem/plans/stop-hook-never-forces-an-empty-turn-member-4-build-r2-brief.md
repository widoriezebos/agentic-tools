# Fold brief: a command test proves the Stop verb rechecks the board (member 4, round 2)

DIRECT MODE (Wido via m1e). Working Mode: implement. Model gpt-5.6-sol. Date 2026-09-19.
Goal-Unit: stop-hook-never-forces-an-empty-turn/stop-frontier-joins-owner-and-revision
Declared size: 80 changed lines

Your working directory is the module root `metasystem/` of a detached git worktree that holds round 1's uncommitted change. Bash and Go only. Leave every change uncommitted; never run git add, git apply, git stash, git rebase or git commit. Never edit `memory/receipts.log`, anything under `records/` or `plans/`. Change only files under `cmd/metasystem/` and, if check 5 asks for it, `testing.json`. No file under `internal/` changes.

## Start state

First command: `git rev-parse HEAD` must print `1e84763e202245843cca98b1a41265d39f3fb51e`. Second: `git status --porcelain | wc -l` must print `13`. Third: `git diff --stat HEAD | tail -1` must print `11 files changed, 541 insertions(+), 117 deletions(-)`. Paste all three under `## Start state`. If any differs, write `MISMATCH` and what you saw as the first line of the return and stop.

## What changes and why

Round 1 made `runReportTurnVerdict` in `cmd/metasystem/goal.go` set `options.RecheckBoard`, the callback the verdict uses to check that the accepted tip and the seat's claim epoch did not change while it built the stop board. No test sees that assignment: delete it and every test stays green, and the Stop verb would then never notice a stale board.

## What to build

1. A test in `cmd/metasystem` that turns red when `runReportTurnVerdict` no longer sets `RecheckBoard`. Expected shape (a smaller one that keeps these rules is fine): move the seat wiring that `runReportTurnVerdict` does once the state root resolves (the `HandoffRecorded` option, the four store callbacks and `RecheckBoard`) into one unexported helper that `runReportTurnVerdict` calls, with no change in behavior. The new test calls that helper on a fixture state root and asserts that `RecheckBoard` is set and that calling it returns what `goal.AcceptedTip` and the seat resolver return for that root (the tip and claim epoch, or the same error). Name the test `TestTurnVerdictSeatWiringRechecksTheBoard`.
2. The test starts with `t.Parallel()`, reads no wall clock and never sleeps. Keep every existing test name and body.
3. Mutation, once: delete the `RecheckBoard` assignment, run only the new test, paste its red line verbatim under `## Mutation`, restore, and show that `git diff --stat HEAD` equals the stat before the mutation.
4. The commit message gains one body sentence saying the board compares the claim epoch the seat's claim already carries (`BoardOwner.ClaimEpoch`) and adds no new record field.

Source comments say what the code does and why in plain English; never a brief, a round, a finding, a seat, a unit or a goal id.

## Bounds

- Before every `go build`, `go test`, `go vet` or gate run: `test -e /tmp/metasystem-testrun-lock`; while it exists run none of them and check again every 60 seconds. One go test process at a time. No single command may wait longer than 240 seconds: run a longer one in the background with its output to a file and poll the file.
- Every go command runs with this environment set: `GOCACHE=/tmp/stopinc-4-go-cache GOPROXY=file:///Users/wido/go/pkg/mod/cache/download GOSUMDB=off STATICCHECK_CACHE=/tmp/stopinc-4-staticcheck`. Never unset or strip it.
- The sandbox refuses kern.proc, `ps`, loopback binds, DNS and `nice`. Whole-package runs of `cmd/metasystem` add `-skip 'TestProcFixtureSurvivors|TestProcessCensus|TestCustodianWatches|TestLauncherDeath|TestAllPids|TestProcessesWithWithheldArguments|TestWaitChannelAnswer|TestChannelStatusPostSeedsAnUnbootedBrainStatus|TestTelegramPeekWorksWithoutConfiguredAdapterOrChatID|TestTelegramPeekTokenNeverAppearsInErrors|TestGroupOwnedLiveNonOwnerExitsNotOwned'`. Classify every remaining sandbox-only red in one line each under `## Sandbox reds`. A whole-package run that times out, or whose failures all pass when rerun alone with `-run`, is a sandbox red under load: paste the whole run and the isolated rerun, and it does not block the return. The seat reruns the whole package on the host. A red of any other kind is yours to fix.
- If the change passes 150 changed lines, stop and return `BUILD: blocked size N` with the planned shape.

## Required checks (from `metasystem/`, each output pasted verbatim into the return)

1. `gofmt -l ./internal/goal ./cmd/metasystem` (must print nothing).
2. `go vet ./cmd/metasystem/` and `go vet -tags batchtest ./cmd/metasystem/`.
3. RUN SET, once plain and once with `-tags batchtest`: `go test -count=1 -run 'TestTurnVerdictSeatWiringRechecksTheBoard' ./cmd/metasystem/` and `go test -count=1 -run 'TestStopFrontierOwnerRevisionAndFreshness|TestIdleDigestKeepsEveryNonterminalJob|TestBoardJoinReadsOlderRecords' ./internal/goal/` and `go test -count=1 -run 'TestProofAttempt|TestLandFixture|TestEveryPackageUsesSharedMain' ./cmd/metasystem/` (the whole-package run in check 4 can time out before these).
4. `go test -count=1 -skip` with the skip list above `./cmd/metasystem/` (background it and poll).
5. `go build ./...` and `go build -tags batchtest ./...`; `go test -count=1 -run 'TestTestEnvironmentStandardInventoryMatchesPackageTests' ./internal/testenv/` (if it asks for the new test in a group, add it where its message says and paste the `testing.json` hunk) and `go test -count=1 -run 'TestRepositoryManifestClassifiesEveryTrackedPath' ./internal/pathclass/`.
6. `go run ./cmd/metasystem audit parallel-ratchet --root .`
7. `git diff --numstat HEAD` and `git status --porcelain`.
8. `bash scripts/agents/go-gate.sh --fast` (background it and poll; paste the whole output).
Every check must be green or a classified sandbox red. A red you cannot make green is returned as `BUILD: blocked` with the red pasted, never hidden.

## Diff and return (shell redirection only, absolute paths)

- Diff: from the worktree root (the parent of `metasystem/`) run `git diff HEAD > RETURNDIR/dm-stopinc-4.diff`, then append each untracked file with `git diff --no-index /dev/null metasystem/<path> >> RETURNDIR/dm-stopinc-4.diff` (its rc 1 is normal): `internal/goal/stopboard.go`, `internal/goal/stopboard_test.go` and any file this round adds.
- Return: write RETURNFILE with `cat > RETURNFILE <<'EOF'` style redirection. First line exactly `BUILD: done` or `BUILD: blocked REASON`. Sections in this order: `## Start state`, `## Seam` (the helper's signature and its one caller), `## Checks` (each command exactly as run, then its output), `## Mutation`, `## Sandbox reds`, `## Commit message` (round 1's subject and body from `RETURNDIR/stopinc-4-build-return-r1.md` under its `## Commit message`, plus the sentence of item 4 and one sentence saying a command test proves the Stop verb sets the recheck; last line `Goal-Unit: stop-hook-never-forces-an-empty-turn/stop-frontier-joins-owner-and-revision`). Literal output only; leave no angle-bracket placeholder in the return.

RETURNDIR = /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1b/127d7cba-2e99-40bb-822e-6c6c855d496f/scratchpad/direct
RETURNFILE = /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1b/127d7cba-2e99-40bb-822e-6c6c855d496f/scratchpad/direct/stopinc-4-build-return-r2.md
