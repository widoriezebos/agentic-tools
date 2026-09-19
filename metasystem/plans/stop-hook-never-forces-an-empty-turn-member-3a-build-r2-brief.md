# Fold brief: the three stop-incident tests run in parallel (member 3a, round 2)

DIRECT MODE (Wido via m1e). Working Mode: implement. Model gpt-5.6-sol. Date 2026-09-19.
Goal-Unit: stop-hook-never-forces-an-empty-turn/stop-incidents-reach-the-steward
Declared size: 90 changed lines

Your working directory is the module root `metasystem/` of a detached git worktree that holds round 1's uncommitted change. Bash and Go only. Leave every change uncommitted; never run git add, git apply, git stash, git rebase or git commit. Never edit `memory/receipts.log`, anything under `records/` or `plans/`. Change only these files: `internal/steward/notify.go`, `internal/steward/tick.go`, `internal/steward/notify_test.go`, `internal/steward/tick_test.go` and `testing-parallel-ratchet.json`. No file under `cmd/` changes.

## Start state

First command: `git rev-parse HEAD` must print `92743c95110f8682d84599fb88f9cdf156b9d3b0`. Second: `git status --porcelain` must list exactly twelve modified files (`cmd/metasystem/report.go`, `cmd/metasystem/steward_verbs.go`, `internal/report/stopblock.go`, `internal/report/stopblock_test.go`, `internal/steward/intervene.go`, `internal/steward/notify.go`, `internal/steward/notify_test.go`, `internal/steward/tick.go`, `internal/steward/tick_test.go`, `scripts/agents/supervision-hook.sh`, `testing-parallel-ratchet.json`, `testing.json`, each under `metasystem/`) and two untracked paths (`metasystem/internal/steward/stopincident.go`, `metasystem/internal/stopincident/`). Third: `git diff --stat HEAD | tail -1` must print `12 files changed, 469 insertions(+), 26 deletions(-)`. Paste all three under `## Start state`. If any differs, write `MISMATCH` and what you saw as the first line of the return and stop.

## What changes and why

Round 1 raised the steward package's serial-test ceiling in `testing-parallel-ratchet.json` from 398 to 401 for three new tests that cannot run in parallel. That raise is refused: a bound is never raised. The three tests are serial only because they swap package variables:
- `TestStopIncidentDeliveryUnconfirmed` (`internal/steward/notify_test.go`) swaps `deliverNotification`.
- `TestStopIncidentOutageRecoveryOutage` (`internal/steward/tick_test.go`) swaps `deliverNotification`.
- `TestStopIncidentDrainBeforeNarrator` (`internal/steward/tick_test.go`) swaps `deliverNotification` and `narrateTickDigest` (a package variable round 1 added in `tick.go`).

## What to build

1. Restore `testing-parallel-ratchet.json` to its HEAD content: `git diff HEAD -- testing-parallel-ratchet.json` must print nothing. Raise no bound anywhere.
2. The notification sender is passed in on the stop-incident path instead of read from the package variable. Expected shape (a smaller one that keeps these rules is fine): an unexported variant of `DeliverPending` takes the sender as a parameter and hands it to `deliverPendingNotification` and `deliverPendingStopIncident`. The exported `DeliverPending` keeps its signature and passes the package variable `deliverNotification`, so `runner.go`, `cmd/metasystem/steward_verbs.go` and the existing tests (`handoff_test.go` and the older tests in `notify_test.go`) stay as they are and keep swapping the package variable.
3. The narrator: remove the package variable `narrateTickDigest`. `RunTick` calls the narrator through an unexported field on `TickConfig` that `withDefaults` fills with `NarrateDigest`. `RunTick`'s signature does not change and no production caller sets the field.
4. The three tests use the unexported variant with a fake sender (and `TestStopIncidentDrainBeforeNarrator` sets the narrator field on its `TickConfig`), swap no package variable, and start with `t.Parallel()`. Remove their comments saying they cannot run in parallel. Keep each test's name and every assertion; a fake still fails the test naming any call it did not expect. No test may reach the real `Deliver`.
5. Size: if the non-test part of the change needs more than 60 changed lines, stop before editing, return `BUILD: blocked shape` with the planned signatures and every caller, and leave the tree as you found it.

Source comments say what the code does and why in plain English; never a brief, a round, a finding, a seat, a unit or a goal id.

## Bounds

- Before every `go build`, `go test`, `go vet` or gate run: `test -e /tmp/metasystem-testrun-lock`; while it exists run none of them and check again every 60 seconds. One go test process at a time. No single command may wait longer than 240 seconds: run a longer one in the background with its output to a file and poll the file.
- Every go command runs with this environment set: `GOCACHE=/tmp/stopinc-3a-go-cache GOPROXY=file:///Users/wido/go/pkg/mod/cache/download GOSUMDB=off STATICCHECK_CACHE=/tmp/stopinc-3a-staticcheck`. Never unset or strip it.
- The sandbox refuses kern.proc, `ps`, loopback binds, DNS and `nice`. Classify every sandbox-only red in one line each under `## Sandbox reds`; a red of any other kind is yours to fix.

## Required checks (from `metasystem/`, each output pasted verbatim into the return)

1. `gofmt -l ./internal/steward` (must print nothing).
2. `go vet ./internal/steward/` and `go vet -tags batchtest ./internal/steward/`.
3. RUN SET, once plain and once with `-tags batchtest`: `go test -count=1 -run 'TestStopIncidentDrainBeforeNarrator|TestStopIncidentDrainFromHookLog|TestStopIncidentOutageRecoveryOutage|TestStopIncidentDeliveryUnconfirmed|TestHookLogLinesRoundTrip|TestStopRefusalRecordKeepsTheClass' ./internal/stopincident/ ./internal/steward/ ./internal/report/`.
4. `go test -count=1 -race -run 'TestStopIncidentDrainBeforeNarrator|TestStopIncidentDrainFromHookLog|TestStopIncidentOutageRecoveryOutage|TestStopIncidentDeliveryUnconfirmed' ./internal/steward/`.
5. Whole steward package, plain and with `-tags batchtest`: `go test -count=1 ./internal/steward/` (background it and poll).
6. `go build ./...` and `go build -tags batchtest ./...`.
7. `go run ./cmd/metasystem audit parallel-ratchet --root .` (green with the steward ceiling at 398).
8. `git diff HEAD -- testing-parallel-ratchet.json` (must print nothing), `git diff --stat HEAD` and `git diff --numstat HEAD`.
9. `bash scripts/agents/go-gate.sh --fast` (background it and poll; paste the whole output).
Every check must be green or a classified sandbox red. A red you cannot make green is returned as `BUILD: blocked` with the red pasted, never hidden.

## Diff and return (shell redirection only, absolute paths)

- Diff: from the worktree root (the parent of `metasystem/`) run `git diff HEAD > RETURNDIR/dm-stopinc-3a.diff`, then append each new file with `git diff --no-index /dev/null metasystem/<path> >> RETURNDIR/dm-stopinc-3a.diff` (its rc 1 is normal): `internal/stopincident/stopincident.go`, `internal/stopincident/stopincident_test.go`, `internal/steward/stopincident.go`.
- Return: write RETURNFILE with `cat > RETURNFILE <<'EOF'` style redirection. First line exactly `BUILD: done` or `BUILD: blocked REASON`. Sections in this order: `## Start state`, `## Seam` (the new and changed signatures, every caller, and the non-test changed-line count), `## Checks` (each command exactly as run, then its output), `## Sandbox reds`, `## Commit message` (the whole unit's message: round 1's subject and body from `RETURNDIR/stopinc-3a-build-return-r1.md` under its `## Commit message`, plus one body line saying the stop-incident tests inject the notification sender and narrator and run in parallel; last line `Goal-Unit: stop-hook-never-forces-an-empty-turn/stop-incidents-reach-the-steward`). Literal output only; leave no angle-bracket placeholder in the return.

RETURNDIR = /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1b/127d7cba-2e99-40bb-822e-6c6c855d496f/scratchpad/direct
RETURNFILE = /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1b/127d7cba-2e99-40bb-822e-6c6c855d496f/scratchpad/direct/stopinc-3a-build-return-r2.md
