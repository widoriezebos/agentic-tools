Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal up-kills-runner-before-first-tick)
Date: 2026-09-04

# Goal

Goal up-kills-runner-before-first-tick (tier 2, approved by Wido's
word "the bugs you mentioned are approved to fix too"). Its record,
metasystem/plans/goals/up-kills-runner-before-first-tick.md, is the
contract; in short: `metasystem up` and the supervision watcher
replace a live steward runner whose first tick has not completed,
because the runner is judged by tick completion, not process
liveness; the watcher does it every 60 seconds and `up` on every call
(the stop hook's included); a real tick on a loaded Mac spends over
210 seconds in the goal ledger projection, because ReadCommitGoals
forks one `git cat-file` per goal file (228 files) and is called per
caller. On m2 on 2026-09-03: fifteen attempts, zero completions, until
the load dropped.

# The change

1. internal/steward/runner.go `repairPinnedRunner` and
   `checkStewardRunner` (and the watcher path that calls them): a
   runner that is ALIVE and ATTEMPTING is left to finish. Liveness is
   the process (pid, start time) plus a tick-in-progress mark the
   runner writes at attempt start (the existing component evidence
   record, artifacts/agents/steward/components/steward-tick.json, has
   attemptSeq and lastAttempt; add an in-progress field or use them);
   a runner is replaced only when its process is dead, or its
   in-progress attempt is older than a bound derived from the last
   measured tick duration (record the last completed tick's duration;
   the bound is max(3 x last duration, 120 s) and configurable as
   `steward.tick-patience-sec`, validated).
2. `up`'s wait for a fresh completion (10 seconds, `scaledRunnerWait`)
   becomes: alive-and-attempting is a verified runner; do not wait for
   a completion that a slow tick cannot deliver in 10 seconds.
3. internal/goal ReadCommitGoals: read the ledger in ONE
   `git cat-file --batch` call (or `git archive`/`ls-tree` plus batch)
   instead of one process per file; same output; the goal package's
   own tests must stay green and get faster (measure before and after
   with `go test ./internal/goal/ -count=1` timing in the return).
4. Fixtures: a slow first tick survives a second `up` and a watcher
   cycle (use the fake runtime and a tick that sleeps past the old
   10-second wait); a dead runner is still replaced; a stuck attempt
   past the patience bound is replaced.

# Gate

`cd metasystem && go build ./... && go vet ./... && gofmt -l . (empty)`;
`go test ./internal/steward/ ./internal/up/ ./internal/goal/ -count=1 -timeout 30m` green;
`bash scripts/agents/supervision-fixtures.sh` if it covers the runner
(say so if it cannot run in the sandbox; the orchestrator reruns it).

# Constraints

Wall-clock budget: 90 minutes; return before it ends even if something
is red, naming it; run the slow goal package once at the end.
MECHANICAL reach (tier 2). Declare the boundary as every file that
differs from main. Gap rule: stop and report a gap with your proposed
contract written out.
