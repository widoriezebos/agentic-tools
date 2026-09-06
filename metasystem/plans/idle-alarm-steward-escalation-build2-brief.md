Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal idle-with-backlog-alarm)
Date: 2026-09-06

# Chain 2 for goal idle-with-backlog-alarm: carry the certified change onto current main

Chain idle-escalate-build1 built the bounded idle gate, passed the
orchestrator's gate and two critic rounds with zero material findings
(its final reviewed tree is 3ce8eb9a5095d037d8b7476ec62cf6ec58706579),
and was closed. Main then moved on two of its files before the landing:
fe61beb7 ("The Stop hook's budget is ours: sixty seconds, measured on
every Stop") and 19b14a9b ("The steward reads the Stop durations and
flags a slow or expired hook"). A closed chain takes no further round,
and a landing binds certified paths to the reviewed tree, so this fresh
chain carries the same change onto current main.

# The task (mechanical; no decision moves)

1. In this worktree, which starts at current main, apply the certified
   diff of that chain's final round with a three-way merge:
   `git apply --3way --directory=metasystem
   metasystem/artifacts/agents/idle-escalate-build1/rounds/3/diff.patch`
   (the path is relative to the repository root; the diff's own paths
   are relative to the metasystem directory). Twenty-three files.
2. Resolve every conflict so that BOTH survive: the two landings above
   and the certified change. Expect one in
   metasystem/cmd/metasystem/steward_verbs_test.go (fe61beb7's
   Stop-duration tests beside the seat-idle status tests) and read the
   merged metasystem/scripts/agents/supervision-hook.sh to confirm the
   sixty-second measured budget sits beside the stop_hook_active read
   and the bounded-idle rendering; name the lines in the return.
3. Nothing else changes: no decision of the build brief
   (metasystem/plans/idle-alarm-steward-escalation-build-brief.md) or
   the correction brief
   (metasystem/plans/idle-alarm-steward-escalation-fold3-brief.md)
   moves; no test is rewritten beyond conflict resolution; nothing
   under plans.

# Gate

From the metasystem directory: `gofmt -l .` prints nothing; `go vet
./...`; `go build ./...`; `go test ./internal/goal/ ./internal/steward/
./internal/report/ ./internal/lease/ ./cmd/metasystem/ -count=1`; `bash
-n scripts/agents/supervision-hook.sh`. Report each with its evidence
level; the orchestrator reruns the hook fixture bed outside the sandbox.

# Constraints

Wall-clock budget: 30 minutes. DESIGN-BEARING reach carries over from
the change itself; a fresh code critic reviews the tree next. Declare
the boundary as every file that differs from main. Gap rule: stop and
report a gap with your proposed contract written out; never fill it
silently.
