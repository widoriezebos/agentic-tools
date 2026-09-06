Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, follow-up round under goal idle-with-backlog-alarm)
Date: 2026-09-06

# Round 4 for chain idle-escalate-build1: merge only

Round 3 passed the orchestrator's gate and two critics (zero material
findings), but main moved again on this chain's files before the
landing: fe61beb7 ("The Stop hook's budget is ours: sixty seconds,
measured on every Stop") changed metasystem/scripts/agents/supervision-hook.sh
and metasystem/cmd/metasystem/steward_verbs_test.go, and 19b14a9b ("The
steward reads the Stop durations and flags a slow or expired hook")
changed steward verbs and health. The landing binds certified paths to
the reviewed tree, so the chain must carry those landings.

The orchestrator has already fast-forwarded this worktree to main at
2b1d6f2a and re-applied the round-3 changes without any commit. The hook
script auto-merged. ONE file is in conflict and is the whole of this
round: metasystem/cmd/metasystem/steward_verbs_test.go carries conflict
markers between fe61beb7's Stop-duration tests and this chain's
seat-idle status tests. Resolve so that BOTH sets of tests survive.
Then confirm the merged hook script still carries fe61beb7's sixty-second
measured budget together with this chain's stop_hook_active read and
bounded-idle rendering (the automatic merge did it; read the result and
name the lines).

No other change of any kind: no decision moves, no test is rewritten
beyond the conflict resolution, nothing under plans.

# Gate

From the metasystem directory: `gofmt -l .` prints nothing; `go vet
./...`; `go build ./...`; `go test ./internal/goal/ ./internal/steward/
./internal/report/ ./internal/lease/ ./cmd/metasystem/ -count=1`; `bash
-n scripts/agents/supervision-hook.sh`. Report each with its evidence
level; the orchestrator reruns the hook fixture bed outside the sandbox.

# Constraints

Wall-clock budget: 25 minutes. Declare the boundary as every file that
differs from main. Gap rule: stop and report a gap with your proposed
contract written out; never fill it silently.
