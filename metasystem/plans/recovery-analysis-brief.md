Working Mode: design
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal recovery-to-good-state)
Date: 2026-09-02

# Goal

Author the ANALYSIS for goal recovery-to-good-state: read
metasystem/plans/goals/recovery-to-good-state.md first; its Intent carries
Wido's order verbatim, the seat's root-cause reading, eight specimens from
2026-09-02, and the done criterion. This round is analysis, not design: it
confirms or corrects the root cause against the tree and the records, so
the design round after the critique designs against facts. Write exactly
one NEW file named recovery-analysis.md in the metasystem plans directory.
Every claim about the tree cites file and line; read the seams before you
write about them.

# Workspace

The delegate worktree the dispatcher created for this job. Read anything;
write only that one new analysis file.

# What the analysis must establish

1. THE STATE MACHINE AS IT IS. Enumerate the partial states a checkout can
   be in, from the components metasystem up reports
   (metasystem/internal/up/up.go: host-preflight, accepted-engine,
   session-identity, session-announcement, checkout-lease,
   supervision-owner, repo-watcher, job-reaper, steward-runner) and the
   health roles (metasystem/internal/steward, the health verb): engine
   drifted from the enrolled pin; owner alive with a drifted identity;
   owner dead with a stale lock; runner dead; session main dead; census
   failed; hook dead; leaked non-custody machinery. For each state: which
   command a seat can lawfully run today, what it does, what it refuses,
   and whether the refusal's remedy is a command the seat can run without
   a human at an agent-free terminal.
2. THE SPECIMEN MAP. For each of the eight specimens in the goal's Intent:
   the state, the command tried, the exact refusal or hang, its file and
   line, and the hand surgery that finally worked. Use the records:
   metasystem/artifacts/agents/supervision/arming.log, metasystem/artifacts/agents/steward,
   the health alerts, metasystem/plans/handoff-m1-2026-09-02.md, the m3 and m2
   handoffs, and the goal records of the eight partial goals.
3. THE REFUSAL INVENTORY. Every refusal or remedy text on the up, steward
   arm, census, delegate, and goal-fetch paths (grep the Go sources under
   metasystem/internal/up, metasystem/internal/steward, metasystem/internal/lease,
   metasystem/internal/dispatch, metasystem/internal/goal, and metasystem/cmd/metasystem), with a verdict
   per text: names a command that exists and the seat can run; names a
   terminal-only command; names a command that does not exist; names
   nothing.
4. THE LEAK SOURCES. Which fixtures and harnesses spawn real stewards,
   owners, watchers and adapter loops (the fixture scripts under metasystem/scripts/agents whose names end in -fixtures.sh,
   metasystem/scripts/validate-metasystem.sh, the Go test beds under metasystem/internal), how
   each is supposed to clean up, and why 488 orphans and 8,789 beds
   survived; what the census sees of them (metasystem/internal/census) and
   why nothing reaps non-custody machinery.
5. THE HARNESS LAYER. State plainly which failures live outside the engine
   (the Claude Code permission classifier refusing the engine's own verbs,
   remote-control messages stranding, a session on a stale harness
   version) and what the engine can still do about them: detect, name,
   route around, or only document.
6. ROOT CAUSE AND SPLIT. Confirm or correct the seat's root-cause
   statement in the goal record, then propose the arc: slices of at most
   240 reserved minutes in dependency order, each with the rehearsal
   fixture that replays a specimen and must recover it, and which of the
   eight partial goals each slice absorbs, re-scopes or leaves.

Self-grade per the house rule: confidence, weakest claim, reject condition.
The analysis is challenged by an independent critic before any design is
written; write so that every sentence can be checked against the tree.

# Constraints

Wall-clock budget: 45 minutes. Analysis only; no design decisions, no
builds, no benchmarks (R-31). Edit nothing but the analysis file.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the one analysis file
named under Goal.

# Gap Rule

stop and report a gap; never fill it silently.
