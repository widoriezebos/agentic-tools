# Build brief: the stop frontier joins owner and revision (member 4 of stop-hook-never-forces-an-empty-turn)

DIRECT MODE (Wido via m1e). Working Mode: implement. Model gpt-5.6-sol. Date 2026-09-19.
Goal-Unit: stop-hook-never-forces-an-empty-turn/stop-frontier-joins-owner-and-revision
Declared size: 1250 changed lines

Your working directory is the module root `metasystem/` of a detached git worktree; every file of this unit is under it. Bash and Go only. Leave every change uncommitted; never run git add, git apply, git stash, git rebase or git commit. Never edit `memory/receipts.log`, anything under `records/` or `plans/`.

## Start state

START STATE CHECK, first command: `git rev-parse HEAD`. The sha must start with START_STATE_SHORT (full START_STATE_SHA). Second command: `git status --porcelain` must print nothing. Put both under `## Start state` in the return. If either fails, write `MISMATCH <sha or tree>` as the first line of the return and stop.

## The design section is the specification

The design page copy is `/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1b/127d7cba-2e99-40bb-822e-6c6c855d496f/scratchpad/direct/stop-hook-members-design-r2.frozen.md` (644 lines). Third command: `shasum -a 256` of it must start with `9ba7a005e4336b63`; otherwise `MISMATCH page` and stop. Read exactly lines 183 to 264 with `sed -n '183,264p'`: the section `## Member 4: stop-frontier-joins-owner-and-revision` with its subsections 1 DONE, 2 Mechanism, 3 File set, 4 Fixtures, 5 Mutations, 6 Estimate and 7 Landing. Build that section in full: its five DONE rules, the mechanism as written, the file set, the tests with the reasons they fail without the mechanism. Read no other line of the page; the other sections belong to other units. Where the section and this brief differ, this brief wins.

The section cites `file:line` against an older tree. Locate by symbol; at this start state the lines are:
- `internal/goal/project.go`: `type Projection struct` 26 (its `Tip` field), `type projectionDependencies struct` 47 (the `fetch` field 48), `func project` 76, `type ClaimableBudgetedWork struct` 223 (`InFlight` 228, `NonTerminalJobs` 229), `func (w ClaimableBudgetedWork) HasDelegateJobInFlight` 257, `func readClaimableBudgetedWork` 332 (the fresh `project(endpoint, true, ...)` at 353, the `Next(projection, machine)` call at 360, its `readLiveBacklogActivity` calls at 398 and 427), the one-argument wrapper at 329, `type backlogJobRecord struct` 453, `func readLiveBacklogActivity` 518, `func Next` 652.
- `internal/goal/turnverdict.go`: `type TurnVerdictOptions struct` 205 (`SeatActor` 212, `SeatActorProblem` 214), `func (s *Store) TurnVerdict` 418, the work read at 496, the `saveVerdictState` call at 555, the final `freezeTurnVerdictFacts` call at 629, `func infrastructureVerdict` 1139 (the infrastructure class), the `HasDelegateJobInFlight` uses at 1216 and 1708, `func idleBacklogDigest` 1270, `func (s *Store) escalateIdleBacklog` 1285 (its `s.ResolveIdleSeat()` call at 1287 to 1294), `func (s *Store) decide` 1617, `func (s *Store) convertedGoalFacts` 1791, `func (s *Store) queuedFrontier` 1855, `func (s *Store) saveVerdictState` 2002.
- `internal/goal/goalverbs.go` 60: the store field `ResolveIdleSeat func() (Actor, int64, error)`.
- `internal/goal/file.go` 331: `type ClaimRecord struct` (Machine, Lineage, Revision).
- `internal/goal/turnfacts.go`: `func freezeTurnVerdictFacts` 88; its test caller `internal/goal/turnfacts_test.go` 11.
- `internal/goal/txn.go`: `func acceptedTipForGates` 157.
- `cmd/metasystem/goal.go`: `func runReportTurnVerdict` 708, `store.ResolveIdleSeat = resolveSeatIdleActor(...)` 776, `func resolveSeatIdleActor` 878 (it returns the lease holder's `ClaimEpoch` as the epoch).
- `internal/dispatch/jobrecord.go`: `MachineID()` 68 (`machineId`), `GoalRevision()` 99 (`goalRevision`), the `claimEpoch` read at 103. Do not import `internal/dispatch` from `internal/goal`.
- `internal/run/waiter.go` 1883: `func (s *Store) List`. `internal/run/run.go`: `GoalRevision` 127 (inside the governed block), `OwnerLineage` 190, `ClaimEpoch` 191, `GoalId` 193.
- `internal/goal/turnverdict_idle_test.go`: `TestIdleBacklogBlocksTwiceThenDefersClaimAndPreparesStewardContinuation` 465, `TestIdleEscalationPreservesAnIndependentOpenWorkBlock` 780, `TestThreeUnreadableLedgerStopsRecordIncidentRaiseAlarmAndEnd` 920, `TestClaudeTurnExitRequiresDelegateWorkNotMerelyALiveSeat` 1657; its `readClaimableBudgetedWork` call at 63. `internal/goal/turnverdict_localwait_test.go` 151 also calls `readClaimableBudgetedWork`.

## Integrator conditions (binding, on top of the section)

1. The claim epoch. `ClaimRecord` has no epoch field at this start state. The epoch a job must match is `BoardOwner.ClaimEpoch`, the epoch `ResolveIdleSeat` returned for the owner (the lease holder's claim epoch). Add no epoch field to `ClaimRecord`, to the goal file or to any record. A job joins when its goal is Held, its `machineId` equals the owner's machine, its `goalRevision` equals the held goal's revision, and its `claimEpoch` equals `Owner.ClaimEpoch`; a governed run joins by `GoalId`, `OwnerLineage` equal to the owner's lineage, `ClaimEpoch` and `Governed.GoalRevision` the same way. With an unresolved owner nothing joins.
2. The owner resolution moves as the section says: `TurnVerdict` calls `s.ResolveIdleSeat` at most once per verdict, before the work read, only when the work read runs and `options.SeatActor` is empty; `escalateIdleBacklog` reads the frozen `options` values and no longer calls the resolver. A test that sets `SeatActor` directly keeps working without a resolver.
3. `RecheckBoard` nil means no freshness check: the board is taken as built. Only `runReportTurnVerdict` wires it (to `goal.AcceptedTip(root)` plus a fresh `resolveSeatIdleActor`). `goal.AcceptedTip` is a read-only wrapper around `acceptedTipForGates`: no fetch, no write.
4. The rebuild. The second decision starts from a copy of the session state taken before the first decision. The first decision leaves nothing behind: no saved verdict state, no idle count, no escalation event, no incident, no status write. The fixture proves it by reading the saved session state after the verdict and finding exactly one decision. `stop-board-stale` uses the existing infrastructure class (`infrastructureVerdict` or its equivalent): the stop is allowed and no idle count is spent.
5. Read rules. The job and run lens is permissive: a missing coordinate or one that is not a whole non-negative number reads as missing, never as an error; the record is still listed and its Reason names the missing coordinate. Tighten no read rule on any stored record. The frozen facts gain one optional field (`omitempty`); facts written before this change still parse.
6. Not changed, and proved unchanged by the whole-package runs: `Next`, `SelectNext`, the `goal next` output, `idleBacklogDigest` and `NonTerminalJobs` (every non-terminal job, whoever owns it), the refusal bound of three, `registeredWaits`.
7. Listed test names are protected: the four existing tests named above keep their names; their bodies change only where a fixture job needs the joining coordinates or a fake `ResolveIdleSeat`. No other existing test is removed or renamed.
8. Callers rule: `readClaimableBudgetedWork`, `readLiveBacklogActivity`, `ClaimableBudgetedWork` and `freezeTurnVerdictFacts` change shape, so grep the whole module for every caller (the test callers above included) and fix them in this round. Whole-package runs of `internal/goal` and `cmd/metasystem` (checks 4 and 5) prove it.
9. `t.Parallel()` is the first statement of every new test and subtest, or its first line is a one-line comment saying why not. No test reads the wall clock or sleeps: the store's clock, a fake `identity.Prober`, the `fetch` field of `projectionDependencies`, a fake `ResolveIdleSeat` and a fake `RecheckBoard` that answers per call. A fake that gets a call it did not expect fails the test naming the call; nothing blocks, nothing retries, no bound is raised.
10. Source comments say what the code does and why, in plain English; never a brief, a review, a round, a finding, a seat, a unit name, a member number or a goal id in code or comments.

## Contract (`metasystem/testing.json`)

One change, nothing else; paste the hunk under `## New tests`: group `wait-stop-standard` (its packages include `internal/goal`; it already lists `TestPendingWaitIdleBacklog`): append `TestStopFrontierOwnerRevisionAndFreshness`, `TestIdleDigestKeepsEveryNonterminalJob` and `TestBoardJoinReadsOlderRecords` to its `tests`. Add no package and no input. If the inventory audit (check 8) demands another placement, follow its message and say what you did in the return; that is not a changed shape.

## Order of work (red before green)

1. Start state, page sha, clean tree.
2. Write the three new tests of section 4 first, and adapt the four named tests only where the section says. Run the RUN SET (check 3) and paste its red under `## Red before green`; compile errors count as red.
3. Build section 2 with the conditions above until the RUN SET is green.
4. Mutations, one at a time, the six of section 5: apply it, run only the named test, paste the observed red line verbatim under `## Mutations` (each entry starts with the test name), restore. After the six, `git diff --stat HEAD` must equal the stat before the first mutation; paste both. A mutation that stays green is a finding: fix the test so it turns red, rerun, say so.
5. The required checks.

## Bounds

- Before every `go build`, `go test`, `go vet` or gate run: `test -e /tmp/metasystem-testrun-lock`; while it exists run none of them and check again every 60 seconds. One go test process at a time. No single command may wait longer than 240 seconds: run a longer one in the background with its output to a file and poll the file.
- Every go command runs with this environment set: `GOCACHE=/tmp/stopinc-4-go-cache GOPROXY=file:///Users/wido/go/pkg/mod/cache/download GOSUMDB=off STATICCHECK_CACHE=/tmp/stopinc-4-staticcheck`. Never unset or strip it.
- The sandbox refuses kern.proc, `ps`, loopback binds, DNS and `nice`. Whole-package runs of `cmd/metasystem` add `-skip 'TestProcFixtureSurvivors|TestProcessCensus|TestCustodianWatches|TestLauncherDeath|TestAllPids|TestProcessesWithWithheldArguments|TestWaitChannelAnswer|TestChannelStatusPostSeedsAnUnbootedBrainStatus|TestTelegramPeekWorksWithoutConfiguredAdapterOrChatID|TestTelegramPeekTokenNeverAppearsInErrors'` (the last four need loopback binds; the host runs them). Classify every remaining sandbox-only red in one line each under `## Sandbox reds` (loopback bind refused, "fake did not start", kern.proc.all EPERM, /bin/ps "operation not permitted", DNS denied, `nice(5) failed`); a red of any other kind is yours to fix.
- If your estimate of the whole change passes 1500 changed lines, stop and return `BUILD: blocked size N` with the reason; do not trim the tests to fit.
- Start no helper process outside a test; every child a test starts ends on every path. List any pid you had to start and how you ended it under `## Helper pids`.

## Required checks (from `metasystem/`, each output pasted verbatim into the return)

1. `gofmt -l ./internal/goal ./cmd/metasystem` (must print nothing).
2. `go vet ./internal/goal/ ./cmd/metasystem/` and the same with `-tags batchtest`.
3. RUN SET, once plain and once with `-tags batchtest`, never `-count` above 1: `go test -count=1 -run 'TestStopFrontierOwnerRevisionAndFreshness|TestIdleDigestKeepsEveryNonterminalJob|TestBoardJoinReadsOlderRecords|TestIdleBacklogBlocksTwiceThenDefersClaimAndPreparesStewardContinuation|TestIdleEscalationPreservesAnIndependentOpenWorkBlock|TestThreeUnreadableLedgerStopsRecordIncidentRaiseAlarmAndEnd|TestClaudeTurnExitRequiresDelegateWorkNotMerelyALiveSeat' ./internal/goal/`.
4. Whole package, plain, then with `-tags batchtest`: `go test -count=1 ./internal/goal/` (background it and poll).
5. `go test -count=1 -skip` with the skip list above `./cmd/metasystem/` (background and poll).
6. `go build ./...` and `go build -tags batchtest ./...`.
7. `go test -count=1 -run 'TestTestEnvironmentStandardInventoryMatchesPackageTests' ./internal/testenv/` (green with your names registered) and `go test -count=1 -run 'TestRepositoryManifestClassifiesEveryTrackedPath' ./internal/pathclass/` (green with the new files).
8. `go run ./cmd/metasystem audit parallel-ratchet --root .`
9. `bash scripts/agents/go-gate.sh --fast` (background it and poll; paste the whole output).
Every check must be green. A red you cannot make green is returned as `BUILD: blocked` with the red pasted, never hidden.

## Diff and return (shell redirection only, absolute paths)

- Diff: from the worktree root (the parent of `metasystem/`) run `git diff HEAD > RETURNDIR/dm-stopinc-4.diff`, then append every new file with `git diff --no-index /dev/null metasystem/<path> >> RETURNDIR/dm-stopinc-4.diff` (its rc 1 is normal): `internal/goal/stopboard.go`, `internal/goal/stopboard_test.go`, and any other new file, each also listed in the return. Paste `git diff --numstat HEAD` and `git status --porcelain` under `## Changed files`.
- Return: write RETURNFILE with `cat > RETURNFILE <<'EOF'` style redirection (never through a tool that needs a path inside the repository). First line exactly `BUILD: done` or `BUILD: blocked REASON`. Sections in this order: `## Start state` (the sha and the page sha), `## Changed files`, `## Red before green`, `## Checks` (each command from the list above exactly as run, then its output), `## Mutations`, `## Sandbox reds`, `## New tests` (names, package, group, the testing.json hunk), `## Helper pids`, `## Commit message` (subject under 72 characters saying what changed for the user, a short body, last line `Goal-Unit: stop-hook-never-forces-an-empty-turn/stop-frontier-joins-owner-and-revision`). Literal output only; leave no angle-bracket placeholder in the return.

RETURNDIR = /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1b/127d7cba-2e99-40bb-822e-6c6c855d496f/scratchpad/direct
RETURNFILE = /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1b/127d7cba-2e99-40bb-822e-6c6c855d496f/scratchpad/direct/stopinc-4-build-return-r1.md
