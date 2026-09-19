# Build brief: a stop refusal names one command that runs as printed (member 5 of stop-hook-never-forces-an-empty-turn)

DIRECT MODE (Wido via m1e). Working Mode: implement. Model gpt-5.6-sol. Date 2026-09-19.
Goal-Unit: stop-hook-never-forces-an-empty-turn/stop-refusals-name-seat-actions
Declared size: 1050 changed lines

| Unit | Lines |
|---|---|
| stop-refusals-name-seat-actions | 1050 |

Your working directory is the module root `metasystem/` of a detached git worktree; every file of this unit is under it. Bash and Go only. Leave every change uncommitted; never run git add, git apply, git stash, git rebase or git commit. Never edit `memory/receipts.log`, anything under `records/` or `plans/`.

## Start state

The worktree holds another unit's uncommitted change, the stop board, on top of the base commit. This unit builds on it and lands after it. Keep that change as it is; this unit edits the files of section 3 on top of it and changes nothing else of it. That change is the file `START_STATE_BELOW_PATH` (sha256 starts with `START_STATE_BELOW_SHA`); read it to see what is already there, `internal/goal/stopboard.go` (`BoardOwner`, `StopBoard.Owner`, `StopBoard.Ready`) above all.

First command: `git rev-parse HEAD` must print `1e84763e202245843cca98b1a41265d39f3fb51e`. Second: `git status --porcelain | wc -l` must print `START_STATE_PORCELAIN_N`. Third: `git diff --stat HEAD | tail -1` must print `START_STATE_STAT`. Fourth: `shasum -a 256` of the page copy (next section) must start with `9ba7a005e4336b63`. Paste all four under `## Start state`. If any differs, write `MISMATCH` and what you saw as the first line of the return and stop.

## The design section is the specification

The page copy is `/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1b/127d7cba-2e99-40bb-822e-6c6c855d496f/scratchpad/direct/stop-hook-members-design-r2.frozen.md` (644 lines). Read exactly lines 266 to 335 with `sed -n '266,335p'`: the section `## Member 5: stop-refusals-name-seat-actions` with its subsections 1 DONE, 2 Mechanism, 3 File set, 4 Fixtures, 5 Mutations, 6 Estimate and 7 Landing. Build that section in full. Read no other line of the page. Where the section and this brief differ, this brief wins.

The section cites `file:line` against an older tree. Locate by symbol. In this worktree: `internal/goal/turnfacts.go` has `type TurnAction struct` 52, `func freezeTurnVerdictFacts` 89, `func classifyOwnership` 179, `func workActions` 244 (the `claim-goal` action with `goal next --machine` at 262 to 268), `func scanActions` 271 (the job watch with `--caller-pid $$` at 277) and `func shellArgument` 300. `internal/goal/turnverdict.go` has `type Verdict struct` 145 (`IdleRefusal` 167), `func (s *Store) TurnVerdict` 419 with its three `freezeTurnVerdictFacts` calls at 461, 481 and 675, `s.decide(&verdict, ...)` 544, `s.saveVerdictState(state)` 601, `idleBacklogContinuation(*work)` called at 1278 and defined at 1291, `func (s *Store) decideRuns` 1438, and the goal-free refusal text (`goal declare-free`) at 1796. `cmd/metasystem/goalsync_mutations.go` 2695 defines `runGoalClaim`. `cmd/metasystem/wait_verb.go` 35 names `METASYSTEM_WAIT_REGISTERED_FD`; `cmd/metasystem/wait_verb_test.go` 1528 passes the pipe through `ExtraFiles`. `cmd/metasystem/context_cost_test.go` 379 runs its own `go build`. The three tests the section calls IdleHandoffRegression are `TestIdleBacklogBlocksTwiceThenDefersClaimAndPreparesStewardContinuation` (465), `TestIdleEscalationPreservesAnIndependentOpenWorkBlock` (780) and `TestThreeUnreadableLedgerStopsRecordIncidentRaiseAlarmAndEnd` (951), with `TestIdleRefusalSurvivesALostCounter` (1327), all in `internal/goal/turnverdict_idle_test.go`.

## Integrator conditions (binding, on top of the section)

1. The change lands enabled. Add no mode, no switch, no environment variable and no option that turns it off.
2. The claim command reads `Board.Ready[0]` and `Board.Owner.Lineage` from the stop board already in the tree. Do not change `internal/goal/stopboard.go`, the board's join rules or `cmd/metasystem/goal.go`. If the section cannot be built without such a change, stop and return `BUILD: blocked board` with what you need.
3. `engineCommand` renders every action command, so each runs as printed from the checkout root. The `--caller-pid $$` tail stays unquoted. The claim command is exactly `bin/metasystem goal claim --root ROOT --id GOAL --lineage LINEAGE`, each value through `shellArgument`. With an unknown lineage the refusal keeps its decision, prints `SeatActorProblem` in place of a command and fabricates no value.
4. `Verdict` gains `ClearingCommand string` with `json:"clearingCommand,omitempty"` and `NoticeSource string` with `json:"noticeSource,omitempty"`. Both are additive; a reader of an older verdict or state file still reads it. Tighten no read rule.
5. `requireClearingCommand` acts only on a seat-actionable block that is not an idle refusal, and never demotes an idle refusal. A notice writes the same session slots a block writes, so it shows once per digest. The idle decisions, counts, digest and bound stay as they are.
6. `builtEngine(t)` builds the engine once per test process into a temporary directory, with the environment of the bounds below, and every command-running test uses it. A test that runs a printed command runs it through `sh -c` from an isolated bed's root in which `bin/metasystem` is that built engine; it never runs the engine of this worktree's own `bin/`.
7. The watch readiness is the wait id read from the pipe named by `METASYSTEM_WAIT_REGISTERED_FD`, never a sleep or a poll of output.
8. The hook (`scripts/agents/supervision-hook.sh`) does not change. In `docs/design/turn-verdict-delivery-contract.md` only the refusal text it quotes changes; its universal-fallback section stays.
9. Listed test names are protected: no test is removed or renamed; bodies adapt. `t.Parallel()` is the first statement of every new test and subtest, or its first line is a one-line comment saying why not. No test reads the wall clock or sleeps. Nothing blocks, nothing retries, no bound is raised.
10. Source comments say what the code does and why, in plain English. They never carry a brief, a review, a round, a finding, a seat, a unit name, a member number or a goal id.

## Contract (`metasystem/testing.json`)

Append `TestStopClearingCommand`, `TestStopClearingCommandsRun` and `TestIdleClaimCommandIsFirstAct` to the `tests` of group `wait-stop-standard`. Add no package. If the inventory audit (check 7) asks for an input or another placement, follow its message exactly and say what you did. Paste the hunk under `## New tests`.

## Order of work (red before green)

1. Start state, page sha, then the before half of check 11 for every refusal string the section changes.
2. Write `TestStopClearingCommand`, `TestStopClearingCommandsRun` and `TestIdleClaimCommandIsFirstAct` first, and adapt the asserted refusal text of the four idle tests. Run the RUN SET (check 3) and paste its red under `## Red before green`; compile errors count as red.
3. Build section 2 with the conditions above until the RUN SET is green.
4. Mutations, one at a time, the five of section 5: apply it, run only the named test (the three idle tests together for IdleHandoffRegression), paste the observed red line verbatim under `## Mutations`, restore. After the five, `git diff --stat HEAD` must equal the stat before the first mutation; paste both. A mutation that stays green is a finding: fix the test so it turns red, rerun, say so.
5. The required checks.

## Bounds

- Before every `go build`, `go test`, `go vet` or gate run: `test -e /tmp/metasystem-testrun-lock`; while it exists run none of them and check again every 60 seconds. One go test process at a time. No single command may wait longer than 240 seconds: run a longer one in the background with its output to a file and poll the file.
- Every go command runs with this environment set: `GOCACHE=/tmp/stopinc-5-go-cache GOPROXY=file:///Users/wido/go/pkg/mod/cache/download GOSUMDB=off STATICCHECK_CACHE=/tmp/stopinc-5-staticcheck`. Never unset or strip it.
- The sandbox refuses kern.proc, `ps`, loopback binds, DNS and `nice`. Whole-package runs of `cmd/metasystem` add `-skip 'TestProcFixtureSurvivors|TestProcessCensus|TestCustodianWatches|TestLauncherDeath|TestAllPids|TestProcessesWithWithheldArguments|TestWaitChannelAnswer|TestChannelStatusPostSeedsAnUnbootedBrainStatus|TestTelegramPeekWorksWithoutConfiguredAdapterOrChatID|TestTelegramPeekTokenNeverAppearsInErrors|TestGroupOwnedLiveNonOwnerExitsNotOwned'`. Classify every remaining sandbox-only red in one line each under `## Sandbox reds` (loopback bind refused, httptest panic on listen, "fake did not start", kern.proc.all EPERM, /bin/ps "operation not permitted", DNS denied, `nice(5) failed`). A whole-package run that times out, or whose failures all pass when rerun alone with `-run`, is a sandbox red under load: paste the whole run and the isolated rerun, and it does not block the return; the seat reruns the whole package on the host. If one of this unit's new tests is red only because the sandbox refuses process inspection, say so with the red line; the seat runs it on the host. A red of any other kind is yours to fix.
- If your estimate of this unit's own change passes 1500 changed lines, stop and return `BUILD: blocked size N` with the reason.
- Start no helper process outside a test. List any pid you had to start and how you ended it under `## Helper pids`.

## Required checks (from `metasystem/`, each output pasted verbatim into the return)

1. `gofmt -l ./internal/goal ./cmd/metasystem` (must print nothing).
2. `go vet ./internal/goal/ ./cmd/metasystem/` and the same with `-tags batchtest`.
3. RUN SET, once plain and once with `-tags batchtest`: `go test -count=1 -run 'TestStopClearingCommand|TestIdleBacklogBlocksTwiceThenDefersClaimAndPreparesStewardContinuation|TestIdleEscalationPreservesAnIndependentOpenWorkBlock|TestThreeUnreadableLedgerStopsRecordIncidentRaiseAlarmAndEnd|TestIdleRefusalSurvivesALostCounter|TestStopFrontierOwnerRevisionAndFreshness|TestIdleDigestKeepsEveryNonterminalJob|TestBoardJoinReadsOlderRecords' ./internal/goal/` and `go test -count=1 -run 'TestStopClearingCommandsRun|TestIdleClaimCommandIsFirstAct|TestProofAttempt|TestLandFixture|TestEveryPackageUsesSharedMain' ./cmd/metasystem/`.
4. Whole package, plain, then with `-tags batchtest`: `go test -count=1 ./internal/goal/` (background it and poll).
5. `go test -count=1 -skip` with the skip list above `./cmd/metasystem/` (background and poll).
6. `go build ./...` and `go build -tags batchtest ./...`.
7. `go test -count=1 -run 'TestTestEnvironmentStandardInventoryMatchesPackageTests' ./internal/testenv/` and `go test -count=1 -run 'TestRepositoryManifestClassifiesEveryTrackedPath' ./internal/pathclass/`.
8. `go run ./cmd/metasystem audit parallel-ratchet --root .`
9. `bash scripts/agents/go-gate.sh --fast` (background it and poll; paste the whole output).
10. Reverse dependents: `go list -f '{{.ImportPath}} {{join .Imports " "}}' ./... | awk '/internal\/goal( |$)/{print $1}'` (paste the list), then `go test -count=1 -skip` with the skip list above on that set, once plain and once with `-tags batchtest` (background each and poll).
11. Refusal strings: for every refusal string this unit changes, `grep -rn -F` the old string from `metasystem/` before the change and again after it, and paste both. Adapt only the test assertions that quote a changed string. List any other hit and leave it unchanged.
Every check must be green or a classified sandbox red. A red you cannot make green is returned as `BUILD: blocked` with the red pasted, never hidden.

## Diff and return (shell redirection only, absolute paths)

- Diff: from the worktree root (the parent of `metasystem/`) run `git diff HEAD > RETURNDIR/dm-stopinc-5.stacked.diff`, then append each untracked file with `git diff --no-index /dev/null metasystem/<path> >> RETURNDIR/dm-stopinc-5.stacked.diff` (its rc 1 is normal): `internal/goal/stopboard.go`, `internal/goal/stopboard_test.go`, `cmd/metasystem/engine_binary_test.go` and any other file this unit adds. This diff holds the stop-board change as well; the seat separates the two. Paste `git diff --numstat HEAD` and `git status --porcelain` under `## Changed files`.
- Return: write RETURNFILE with `cat > RETURNFILE <<'EOF'` style redirection. First line exactly `BUILD: done` or `BUILD: blocked REASON`. Sections in this order: `## Start state`, `## Changed files`, `## Red before green`, `## Checks` (each command exactly as run, then its output), `## Mutations`, `## Sandbox reds`, `## New tests` (name, package, group, the testing.json hunk), `## Helper pids`, `## Commit message` (subject under 72 characters saying what changed for the user, a short body, last line `Goal-Unit: stop-hook-never-forces-an-empty-turn/stop-refusals-name-seat-actions`). Literal output only; leave no angle-bracket placeholder in the return.

RETURNDIR = /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1b/127d7cba-2e99-40bb-822e-6c6c855d496f/scratchpad/direct
RETURNFILE = /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1b/127d7cba-2e99-40bb-822e-6c6c855d496f/scratchpad/direct/stopinc-5-build-return-r1.md
