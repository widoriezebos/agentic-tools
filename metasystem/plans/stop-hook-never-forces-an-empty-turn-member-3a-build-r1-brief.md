# Build brief: stop incidents reach the steward (member 3a of stop-hook-never-forces-an-empty-turn)

DIRECT MODE (Wido via m1e). Working Mode: implement. Model gpt-5.6-sol. Date 2026-09-19.
Goal-Unit: stop-hook-never-forces-an-empty-turn/stop-incidents-reach-the-steward
Declared size: 1400 changed lines

Your working directory is the module root `metasystem/` of a detached git worktree; every file of this unit is under it. Bash and Go only. Leave every change uncommitted; never run git add, git apply, git stash, git rebase or git commit. Never edit `memory/receipts.log`, anything under `records/` or `plans/`.

## Start state

START STATE CHECK, first command: `git rev-parse HEAD`. The sha must start with START_STATE_SHORT (full START_STATE_SHA). Second command: `git status --porcelain` must print nothing. Put both under `## Start state` in the return. If either fails, write `MISMATCH <sha or tree>` as the first line of the return and stop.

## The design section is the specification

The design page copy is `/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1b/127d7cba-2e99-40bb-822e-6c6c855d496f/scratchpad/direct/stop-hook-members-design-r2.frozen.md` (644 lines). Third command: `shasum -a 256` of it must start with `9ba7a005e4336b63`; otherwise `MISMATCH page` and stop. Read exactly lines 30 to 118 with `sed -n '30,118p'`: the section `## Member 3a: stop-incidents-reach-the-steward` with its subsections 1 DONE, 2 Mechanism, 3 File set, 4 Fixtures, 5 Mutations, 6 Estimate and 7 Landing. Build that section in full: its four DONE rules, the mechanism as written, the file set, the six tests with the reasons they fail without the mechanism. Read no other line of the page; the other sections belong to other units. Where the section and this brief differ, this brief wins.

The section cites `file:line` against this start state. Locate by symbol; the lines are:
- `scripts/agents/supervision-hook.sh`: `deadline_log_stop_condition()` 1334 (its printf at 1344 writes `stop-condition infrastructure <cause> <component> - <deadline_end> degraded-allow`), `stop_cause_code()` 1745, `record_stop_failure()` 1748 (sets `stop_failure_code` and `stop_failure_component`), the deadline parent's `report stop-block` call 1579, `external_stop_json()` 2072, `append_stop_condition()` 2442 (its printf at 2444 writes `stop-condition <class> <cause> <component> <generation or -> <deadline_end> <outcome>`), called at 2464 and 2569.
- `internal/steward/intervene.go`: `type PendingNotification struct` 320, `func QueueNotification` 336, `func MarkDelivered` 384.
- `internal/steward/notify.go`: `var deliverNotification = Deliver` 29 (the package seam the steward tests replace), `func deliverPendingNotification` 188.
- `internal/steward/tick.go`: `func (c TickConfig) now()` 50 (the fake clock seam), `type TickResult struct` 58, `func RunTick` 116, the `NarrateDigest` call 242.
- `internal/report/stopblock.go`: `func StopRefusal` 122, `type stopRefusalCause struct` 208. Callers of `StopRefusal`: `cmd/metasystem/report.go` 213 and eleven calls in `internal/report/stopblock_test.go`.
- `cmd/metasystem/report.go`: the `report stop-block` flag set, lines 170 to 182.
- `cmd/metasystem/steward_verbs.go`: the tick verb's printed report (find its keys by grep; add `stopIncidents` beside them).
- `internal/steward/alert_episode.go` 577: `artifacts/agents/steward/spend/<day>.json`, the day-scoped file pattern the ledger follows.

## Integrator conditions (binding, on top of the section)

1. The delivered ledger is runtime state under `artifacts/agents/steward/`: nothing cites it as evidence of anything and no test treats it as a record of anything but the drain. Retention, replacing the single file the section names: the ledger is day-scoped like `artifacts/agents/steward/spend/<day>.json`, at `artifacts/agents/steward/stop-incidents/<UTC day>.log` (day formatted 2006-01-02 from the tick clock). The drain writes only today's file and reads today's and yesterday's: a `delivered` identity in either file is never queued; the history boundary is the earliest `drain-start` it read; when neither file exists the tick writes today's `drain-start` and queues nothing (the section's first-tick rule); when only yesterday's exists the tick writes today's `drain-start` and drains normally. No file is ever rewritten or removed. `TestStopIncidentDrainFromHookLog` gains one leg for the day rollover: yesterday's file holds a delivered line and today's file does not exist; the tick writes today's start line, does not queue that identity, and queues an incident whose deadline end is after yesterday's `drain-start`.
2. The `stop-condition` line format does not change. The hook change is exactly what the section says: the four new flags on both `report stop-block` calls (`external_stop_json` and the deadline parent), and the deadline parent's deadline end moved into one variable that both `deadline_log_stop_condition` and its `report stop-block` call read. Run `bash -n scripts/agents/supervision-hook.sh` after the edit. The hook fixture bed `scripts/agents/supervision-hook-fixtures.sh` refuses to run in this sandbox (`nice(5) failed: operation not permitted`), so the bed is HOST-ONLY: say so under `## Sandbox reds`. Do not edit the bed unless a new flag makes its fake engine (the `report stop-block` branch of the `deadline-engine` heredoc near line 1800; it only inspects argv 1 and 2) or one of its assertions fail; then make the smallest change, name it in the return, and touch nothing near `tool_engine_cache_written_at_start` (another unit edits that region).
3. Callers rule: `StopRefusal` gains a last argument, so every caller changes in this round: `cmd/metasystem/report.go` 213 passes the incident built from the new flags (nil when `--cause-code` is empty), and the eleven test calls in `internal/report/stopblock_test.go` pass nil. Whole-package runs of `internal/report` and `cmd/metasystem` (checks 4 and 5) prove it.
4. `t.Parallel()` is the first statement of every new test, or the test's first line is a one-line comment saying why not. A test that swaps the package variable `deliverNotification` cannot run in parallel with the other tests that use it: say exactly that on those tests. A test that only uses `TickConfig.Now`, the new ledger path and temp directories is parallel.
5. No test reads the wall clock or sleeps. The tick's time is `TickConfig.Now` and the drain takes its `now` from `cfg.now()`. Fakes fail with what they saw: a fake channel, fake ledger or unreadable source that gets a call it did not expect fails the test naming the call; nothing blocks, nothing retries, no bound is raised.
6. Source comments say what the code does and why, in plain English; never a brief, a review, a round, a finding, a seat, a unit name or a goal id in code or comments.

## Contract (`metasystem/testing.json`)

Two changes, nothing else; paste the hunk under `## New tests`:
- group `context-standard`: add `internal/stopincident` to its `packages`, and append `TestStopIncidentDrainBeforeNarrator`, `TestStopIncidentDrainFromHookLog`, `TestStopIncidentOutageRecoveryOutage`, `TestStopIncidentDeliveryUnconfirmed` and `TestHookLogLinesRoundTrip` to its `tests`. Its inputs already cover `metasystem/internal/**` and `metasystem/scripts/**`; add no input.
- group `wait-stop-standard` (its packages include `internal/report`): append `TestStopRefusalRecordKeepsTheClass` to its `tests`.
If the inventory audit (check 8) demands another placement, follow its message and say what you did in the return; that is not a changed shape.

## Order of work (red before green)

1. Start state, page sha, clean tree.
2. Write the six tests of section 4 (with the rollover leg of condition 1) first. Run the RUN SET (check 3) and paste its red under `## Red before green`; compile errors count as red.
3. Build section 2 with the conditions above until the RUN SET is green.
4. Mutations, one at a time, the six of section 5: apply it, run only the named test, paste the observed red line verbatim under `## Mutations` (each entry starts with the test name), restore. After the six, `git diff --stat HEAD` must equal the stat before the first mutation; paste both. A mutation that stays green is a finding: fix the test so it turns red, rerun, say so.
5. The required checks.

## Bounds

- Before every `go build`, `go test`, `go vet` or gate run: `test -e /tmp/metasystem-testrun-lock`; while it exists run none of them and check again every 60 seconds. One go test process at a time. No single command may wait longer than 240 seconds: run a longer one in the background with its output to a file and poll the file.
- Every go command runs with this environment set: `GOCACHE=/tmp/stopinc-3a-go-cache GOPROXY=file:///Users/wido/go/pkg/mod/cache/download GOSUMDB=off STATICCHECK_CACHE=/tmp/stopinc-3a-staticcheck`. Never unset or strip it.
- The sandbox refuses kern.proc, `ps`, loopback binds, DNS and `nice`. Whole-package runs of `cmd/metasystem` add `-skip 'TestProcFixtureSurvivors|TestProcessCensus|TestCustodianWatches|TestLauncherDeath|TestAllPids|TestProcessesWithWithheldArguments'`. Classify every remaining sandbox-only red in one line each under `## Sandbox reds` (loopback bind refused, "fake did not start", kern.proc.all EPERM, /bin/ps "operation not permitted", DNS denied, `nice(5) failed`); a red of any other kind is yours to fix.
- If your estimate of the whole change passes 1500 changed lines, stop and return `BUILD: blocked size N` with the reason; do not trim the tests to fit.
- Start no helper process outside a test; every child a test starts ends on every path. List any pid you had to start and how you ended it under `## Helper pids`.

## Required checks (from `metasystem/`, each output pasted verbatim into the return)

1. `gofmt -l ./internal/stopincident ./internal/steward ./internal/report ./cmd/metasystem` (must print nothing).
2. `go vet ./internal/stopincident/ ./internal/steward/ ./internal/report/ ./cmd/metasystem/` and the same with `-tags batchtest`.
3. RUN SET, once plain and once with `-tags batchtest`, never `-count` above 1: `go test -count=1 -run 'TestStopIncidentDrainBeforeNarrator|TestStopIncidentDrainFromHookLog|TestStopIncidentOutageRecoveryOutage|TestStopIncidentDeliveryUnconfirmed|TestHookLogLinesRoundTrip|TestStopRefusalRecordKeepsTheClass' ./internal/stopincident/ ./internal/steward/ ./internal/report/`.
4. Whole packages, plain: `go test -count=1 ./internal/stopincident/ ./internal/steward/ ./internal/report/` (background it and poll if it may pass 240 seconds).
5. `go test -count=1 -skip 'TestProcFixtureSurvivors|TestProcessCensus|TestCustodianWatches|TestLauncherDeath|TestAllPids|TestProcessesWithWithheldArguments' ./cmd/metasystem/` (background and poll), then `go test -count=1 -run 'TestProofAttempt|TestLandFixture|TestEveryPackageUsesSharedMain|TestBedsResolveStopReportsThroughTheEngine' ./cmd/metasystem/`.
6. `bash -n scripts/agents/supervision-hook.sh` and `bash -n scripts/agents/supervision-hook-fixtures.sh`.
7. `go build ./...` and `go build -tags batchtest ./...`.
8. `go test -count=1 -run 'TestTestEnvironmentStandardInventoryMatchesPackageTests' ./internal/testenv/` (green with your names registered) and `go test -count=1 -run 'TestRepositoryManifestClassifiesEveryTrackedPath' ./internal/pathclass/` (green with the new package's files).
9. `go run ./cmd/metasystem audit parallel-ratchet --root .`
10. `bash scripts/agents/go-gate.sh --fast` (background it and poll; paste the whole output).
Every check must be green. A red you cannot make green is returned as `BUILD: blocked` with the red pasted, never hidden.

## Diff and return (shell redirection only, absolute paths)

- Diff: from the worktree root (the parent of `metasystem/`) run `git diff HEAD > RETURNDIR/dm-stopinc-3a.diff`, then append every new file with `git diff --no-index /dev/null metasystem/<path> >> RETURNDIR/dm-stopinc-3a.diff` (its rc 1 is normal): `internal/stopincident/stopincident.go`, `internal/stopincident/stopincident_test.go`, `internal/steward/stopincident.go`, and any other new file, each also listed in the return. Paste `git diff --numstat HEAD` and `git status --porcelain` under `## Changed files`.
- Return: write RETURNFILE with `cat > RETURNFILE <<'EOF'` style redirection (never through a tool that needs a path inside the repository). First line exactly `BUILD: done` or `BUILD: blocked REASON`. Sections in this order: `## Start state` (the sha and the page sha), `## Changed files`, `## Red before green`, `## Checks` (each command from the list above exactly as run, then its output), `## Mutations`, `## Sandbox reds`, `## New tests` (names, packages, groups, the testing.json hunk), `## Helper pids`, `## Commit message` (subject under 72 characters saying what changed for the user, a short body, last line `Goal-Unit: stop-hook-never-forces-an-empty-turn/stop-incidents-reach-the-steward`). Literal output only; leave no angle-bracket placeholder in the return.

RETURNDIR = /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1b/127d7cba-2e99-40bb-822e-6c6c855d496f/scratchpad/direct
RETURNFILE = /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1b/127d7cba-2e99-40bb-822e-6c6c855d496f/scratchpad/direct/stopinc-3a-build-return-r1.md
