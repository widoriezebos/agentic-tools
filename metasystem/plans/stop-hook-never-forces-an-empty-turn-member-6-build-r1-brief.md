# Build brief: a fenced goal is shown once per unchanged fence (member 6 of stop-hook-never-forces-an-empty-turn)

DIRECT MODE (Wido via m1e). Working Mode: implement. Model gpt-5.6-sol. Date 2026-09-19.
Goal-Unit: stop-hook-never-forces-an-empty-turn/stop-fences-surface-once
Declared size: 360 changed lines

| Unit | Lines |
|---|---|
| stop-fences-surface-once | 360 |

Your working directory is the module root `metasystem/` of a detached git worktree; every file of this unit is under it. Bash and Go only. Leave every change uncommitted; never run git add, git apply, git stash, git rebase or git commit. Never edit `memory/receipts.log`, anything under `records/` or `plans/`.

## Start state

The worktree holds another unit's uncommitted change, the stop board, on top of the base commit. This unit builds on it and lands after it. Keep that change as it is; this unit edits `internal/goal/turnverdict.go`, `internal/goal/turnverdict_stopfence_test.go` and `testing.json` on top of it and touches no other file of it. That change is the file `/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1b/127d7cba-2e99-40bb-822e-6c6c855d496f/scratchpad/direct/dm-stopinc-4.r1.diff` (sha256 starts with `19554aec2763f76b`); read it to see what is already there, `internal/goal/stopboard.go` and the test `TestStopFrontierOwnerRevisionAndFreshness` in `internal/goal/turnverdict_world_test.go` above all.

First command: `git rev-parse HEAD` must print `1e84763e202245843cca98b1a41265d39f3fb51e`. Second: `git status --porcelain | wc -l` must print `13` (eleven modified files and the two untracked `metasystem/internal/goal/stopboard.go` and `metasystem/internal/goal/stopboard_test.go`). Third: `git diff --stat HEAD | tail -1` must print `11 files changed, 541 insertions(+), 117 deletions(-)`. Fourth: `shasum -a 256` of the page copy (next section) must start with `9ba7a005e4336b63`. Paste all four under `## Start state`. If any differs, write `MISMATCH` and what you saw as the first line of the return and stop.

## The design section is the specification

The page copy is `/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1b/127d7cba-2e99-40bb-822e-6c6c855d496f/scratchpad/direct/stop-hook-members-design-r2.frozen.md` (644 lines). Read exactly lines 336 to 387 with `sed -n '336,387p'`: the section `## Member 6: stop-fences-surface-once` with its subsections 1 DONE, 2 Mechanism, 3 File set, 4 Fixtures, 5 Mutations, 6 Estimate and 7 Landing. Build that section in full. Read no other line of the page. Where the section and this brief differ, this brief wins.

The section cites `file:line` against an older tree. Locate by symbol. At the base commit, before the stop-board change: `internal/goal/turnverdict.go` has `const maxFreeDigests` 181, `type sessionState struct` 185, `func FencedClaimLines` 1549, `func (w ClaimableBudgetedWork) OnlyFencedClaim` 1609, `func (s *Store) decide` 1617, its `case "ok":` with the registered-wait exit (`waits.hasWorkInFlight()`) and the delegate-job exit (`work.HasDelegateJobInFlight()`), each ending in `break`, the `FencedClaimLines(work.fencedClaims)` appends at 1731, 1739 and 1746, `case "queued-only":` 1734 with its two exits, and `func appendCapped` 2029. `internal/goal/project.go` 233 is the `fencedClaims` field and 379 its fill. `internal/goal/file.go`: `StopCapability *StopCapability` and `StopFence *StopFence` on the goal file at 78 and 79, `type StopCapability struct` 381 (`FenceEpoch` 386), `type StopFence struct` 391. `internal/goal/turnverdict_stopfence_test.go` 9 holds `TestTurnVerdictDoesNotBlockAndDescribesTheDurableStopPhase`.

## Integrator conditions (binding, on top of the section)

1. The change lands enabled. Add no mode, no switch, no environment variable and no option that turns it off.
2. The live job of the fixture joins through the stop board already in the tree: it carries the coordinates the board joins on, set the way the existing board tests set them (read the `joined job` subtest of `TestStopFrontierOwnerRevisionAndFreshness` in `internal/goal/turnverdict_world_test.go` for how a job joins the owner's held goal). Do not change `internal/goal/stopboard.go` or the board's join rules. If the fixture cannot make a job join without such a change, stop and return `BUILD: blocked join` with what you tried.
3. Both exits. The lines `surfaceFencedClaims` returns are appended on every path of the `ok` branch, including the registered-wait exit and the delegate-job exit, and on both exits of the `queued-only` branch. `FencedClaimLines`, `LandingClaimLines` and where the landing lines print do not change.
4. The fingerprint is exactly the section's: the goal id, the fence's StopID, Revision, Epoch, CapabilityGeneration, ClosedAt and Reason, and the capability's FenceEpoch when the goal file has a capability. Nothing else, and no hook generation.
5. The session slot `SurfacedFences []string` with `json:"surfacedFences,omitempty"`, capped by `maxSurfacedFences = 16` through `appendCapped`. A stored fingerprint whose fence is no longer held is dropped. A session state written before this change, without the field, still reads. Tighten no read rule.
6. A repeated stop in the fixture is a new `Store` over the same root and session id, the way each hook call is a new process, so only the saved session state can carry the memory.
7. Listed test names are protected. `TestTurnVerdictDoesNotBlockAndDescribesTheDurableStopPhase` keeps its name and body. No existing test is removed or renamed.
8. `t.Parallel()` is the first statement of every new test and subtest, or its first line is a one-line comment saying why not. No test reads the wall clock or sleeps: the store's clock and a prober. Nothing blocks, nothing retries, no bound is raised.
9. Source comments say what the code does and why, in plain English. They never carry a brief, a review, a round, a finding, a seat, a unit name, a member number or a goal id.

## Contract (`metasystem/testing.json`)

One change, nothing else; paste the hunk under `## New tests`: append `TestFencedClaimSurfacedOnceBesideFlight` to the `tests` of group `wait-stop-standard`. Add no package and no input. If the inventory audit (check 7) demands another placement, follow its message and say what you did.

## Order of work (red before green)

1. Start state, page sha.
2. Write `TestFencedClaimSurfacedOnceBesideFlight` first, with every subtest of section 4. Run the RUN SET (check 3) and paste its red under `## Red before green`; compile errors count as red.
3. Build section 2 with the conditions above until the RUN SET is green.
4. Mutations, one at a time, the four of section 5: apply it, run only the named test, paste the observed red line verbatim under `## Mutations`, restore. After the four, `git diff --stat HEAD` must equal the stat before the first mutation; paste both. A mutation that stays green is a finding: fix the test so it turns red, rerun, say so.
5. The required checks.

## Bounds

- Before every `go build`, `go test`, `go vet` or gate run: `test -e /tmp/metasystem-testrun-lock`; while it exists run none of them and check again every 60 seconds. One go test process at a time. No single command may wait longer than 240 seconds: run a longer one in the background with its output to a file and poll the file.
- Every go command runs with this environment set: `GOCACHE=/tmp/stopinc-6-go-cache GOPROXY=file:///Users/wido/go/pkg/mod/cache/download GOSUMDB=off STATICCHECK_CACHE=/tmp/stopinc-6-staticcheck`. Never unset or strip it.
- The sandbox refuses kern.proc, `ps`, loopback binds, DNS and `nice`. Whole-package runs of `cmd/metasystem` add `-skip 'TestProcFixtureSurvivors|TestProcessCensus|TestCustodianWatches|TestLauncherDeath|TestAllPids|TestProcessesWithWithheldArguments|TestWaitChannelAnswer|TestChannelStatusPostSeedsAnUnbootedBrainStatus|TestTelegramPeekWorksWithoutConfiguredAdapterOrChatID|TestTelegramPeekTokenNeverAppearsInErrors'` (the last four need loopback binds; the host runs them). Classify every remaining sandbox-only red in one line each under `## Sandbox reds` (loopback bind refused, httptest panic on listen, "fake did not start", kern.proc.all EPERM, /bin/ps "operation not permitted", DNS denied, `nice(5) failed`); a red of any other kind is yours to fix.
- If your estimate of this unit's own change passes 1500 changed lines, stop and return `BUILD: blocked size N` with the reason.
- Start no helper process outside a test. List any pid you had to start and how you ended it under `## Helper pids`.

## Required checks (from `metasystem/`, each output pasted verbatim into the return)

1. `gofmt -l ./internal/goal ./cmd/metasystem` (must print nothing).
2. `go vet ./internal/goal/ ./cmd/metasystem/` and the same with `-tags batchtest`.
3. RUN SET, once plain and once with `-tags batchtest`: `go test -count=1 -run 'TestFencedClaimSurfacedOnceBesideFlight|TestTurnVerdictDoesNotBlockAndDescribesTheDurableStopPhase|TestStopFrontierOwnerRevisionAndFreshness|TestIdleDigestKeepsEveryNonterminalJob|TestBoardJoinReadsOlderRecords' ./internal/goal/`.
4. Whole package, plain, then with `-tags batchtest`: `go test -count=1 ./internal/goal/` (background it and poll).
5. `go test -count=1 -skip` with the skip list above `./cmd/metasystem/` (background and poll).
6. `go build ./...` and `go build -tags batchtest ./...`.
7. `go test -count=1 -run 'TestTestEnvironmentStandardInventoryMatchesPackageTests' ./internal/testenv/` and `go test -count=1 -run 'TestRepositoryManifestClassifiesEveryTrackedPath' ./internal/pathclass/`.
8. `go run ./cmd/metasystem audit parallel-ratchet --root .`
9. `bash scripts/agents/go-gate.sh --fast` (background it and poll; paste the whole output).
Every check must be green or a classified sandbox red. A red you cannot make green is returned as `BUILD: blocked` with the red pasted, never hidden.

## Diff and return (shell redirection only, absolute paths)

- Diff: from the worktree root (the parent of `metasystem/`) run `git diff HEAD > RETURNDIR/dm-stopinc-6.stacked.diff`, then append each untracked file with `git diff --no-index /dev/null metasystem/<path> >> RETURNDIR/dm-stopinc-6.stacked.diff` (its rc 1 is normal): `internal/goal/stopboard.go`, `internal/goal/stopboard_test.go` and any file this unit adds. This diff holds the stop-board change as well; the seat separates the two. Paste `git diff --numstat HEAD` and `git status --porcelain` under `## Changed files`.
- Return: write RETURNFILE with `cat > RETURNFILE <<'EOF'` style redirection. First line exactly `BUILD: done` or `BUILD: blocked REASON`. Sections in this order: `## Start state`, `## Changed files`, `## Red before green`, `## Checks` (each command exactly as run, then its output), `## Mutations`, `## Sandbox reds`, `## New tests` (name, package, group, the testing.json hunk), `## Helper pids`, `## Commit message` (subject under 72 characters saying what changed for the user, a short body, last line `Goal-Unit: stop-hook-never-forces-an-empty-turn/stop-fences-surface-once`). Literal output only; leave no angle-bracket placeholder in the return.

RETURNDIR = /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1b/127d7cba-2e99-40bb-822e-6c6c855d496f/scratchpad/direct
RETURNFILE = /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1b/127d7cba-2e99-40bb-822e-6c6c855d496f/scratchpad/direct/stopinc-6-build-return-r1.md
