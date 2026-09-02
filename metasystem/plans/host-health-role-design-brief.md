Working Mode: design
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal host-health-role)
Date: 2026-09-02

# Goal

Author the design paragraph for goal host-health-role (read
metasystem/plans/goals/host-health-role.md first: on 2026-09-02 the m1
Mac ran for seventeen days with Apple's fseventsd at 100 percent CPU and
17 GB resident, swap at 94 percent, and 488 leaked fixture processes, and
nothing in the metasystem noticed; Wido's role-liveness order of
2026-08-28 wants a visible one-line health message every interval and
immediate named action on unhealthy state). Write exactly one NEW file
named host-health-role-design.md in the metasystem plans directory. This
is a small item: one page, the facts read, the thresholds, the alert
text, the fixture. Every claim about the tree cites file and line.

# Workspace

The delegate worktree the dispatcher created for this job. Read anything;
write only that one new design file.

# What the design must settle

1. THE ROLE. A steward health role beside the existing ones (read
   metasystem/internal/steward/health.go for the role table, how a role
   reports alive, dead or unknown, and how a remedy text is attached;
   metasystem/internal/steward/delivery.go and the alert episode path for
   how an unhealthy role becomes an episode). Name it, state its interval
   (the tick), and what it reads.
2. THE FACTS READ, portably: one-minute load average against the core
   count; swap used against swap total (on macOS via sysctl vm.swapusage,
   on Linux via /proc/meminfo, the same two verified platforms the
   project rules name in metasystem/docs/project-rules.md); the top three
   processes by CPU and by resident memory from ps with a portable format;
   free disk on the checkout's volume. Cite the command inventory contract
   in metasystem/docs/project-rules.md and stay inside it. No new
   dependencies.
3. THE THRESHOLDS AND THE VERDICT: name each threshold as a config key
   with a default (load above N times the core count for M consecutive
   ticks; swap above P percent; a single process above Q percent CPU or R
   GB resident for M ticks; disk below S percent free) and the verdict
   text, which names the process, whether it is ours (its argv under our
   engine path, tag prefixes, or temp beds, decided by the census's
   existing shape rules in metasystem/internal/census) and the remedy
   (for ours: the sweep or restart the recovery and custody designs own;
   for foreign: "not ours, tell the operator"). Quiet otherwise: no line
   when healthy beyond the existing one-line health summary.
4. THE FIXTURE. The role fed a recorded snapshot of 2026-09-02 (the
   numbers in the goal record) raises exactly one episode naming
   fseventsd, not ours, with the remedy; the same role fed a healthy
   snapshot raises nothing; thresholds crossed for fewer than M ticks
   raise nothing. Deterministic: the facts come from an injected reader,
   never from the live machine in tests.
5. SIZE. One slice under the 4-hour box; estimate against precedent from
   the health role that landed most recently (git log on
   metasystem/internal/steward/health.go) or mark it unsupported.

Self-grade per the house rule.

# Constraints

Wall-clock budget: 25 minutes. Design only; no builds, no benchmarks
(R-31). Edit nothing but the design file.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the one design file
named under Goal.

# Gap Rule

stop and report a gap; never fill it silently.
