Working Mode: design
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal host-health-role)
Date: 2026-09-02

# Goal

Round-1 critique of metasystem/plans/host-health-role-design.md (revision
1, landed, in your worktree), the design for goal host-health-role (read
metasystem/plans/goals/host-health-role.md first: the 2026-09-02 specimen,
a runaway system daemon and swap at 94 percent that nothing noticed for
seventeen days). This is a small item: one new steward health role that
reads host facts portably, judges named thresholds, and raises one
episode naming the offender and whether it is ours.

# Your mandate

1. VERIFY THE ROLE SHAPE against the tree: that it fits the role table
   and reporting contract in metasystem/internal/steward/health.go and
   the episode path in metasystem/internal/steward/delivery.go without
   changing any existing role's behavior.
2. VERIFY PORTABILITY: every command the design reads (load, swap,
   process table, disk) exists in the command inventory contract in
   metasystem/docs/project-rules.md on both verified platforms, macOS on
   arm64 and Debian on arm64, with the exact flags; name any flag that
   differs between the two and whether the design handles it.
3. ATTACK THE THRESHOLDS AND THE VERDICT: are the defaults sane for a
   shared 4-core VM and a 64 GB Mac; does "ours" follow the census's
   shape rules in metasystem/internal/census rather than a new pattern;
   can the role misname a foreign process as ours or stay silent on the
   specimen; does it stay quiet on a healthy machine.
4. ATTACK THE FIXTURE: deterministic through an injected reader, the
   specimen snapshot raising exactly one episode, the healthy snapshot
   raising none, a threshold held for fewer ticks than required raising
   none.
5. SIZE: one slice under the four-hour box with a correction round
   intact.
6. NEW FINDINGS only if material and grounded. Zero material findings is
   an acceptable, closing answer if the reading supports it.

Findings quote the disagreeing text or code. Your sandbox is read-only:
verify by reading, do not run go.

# Constraints

Wall-clock budget: 25 minutes. Return per the design-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
