Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal missionrunner-terminate-flake)
Date: 2026-09-02

# Goal

DESIGN-EVIDENCE SPIKE, not product code: in your worktree, prototype and
execute answers to the nine findings of
metasystem/records/misc/missionrunner-patience-critique-r1.md (landed, in
your worktree) against the real internal/missionrunner and internal/proc
code. Nothing you write lands; your RETURN carries the verdicts that
revision 2 folds. Per finding id, report SURVIVES / NEEDS RULE <stated> /
REFUTED with the test transcript.

Priorities: DC-PAT-001 (build the group observation on the real proc
surface — can uncertainty be distinguished from death?), DC-PAT-003/004
(measure what actually advances during a healthy kill and during a wedge —
find the honest progress signal), DC-PAT-002/009 (construct the
deterministic stays-red case: a fake ownership-refusing terminator — does
any patience rule launder it?), DC-PAT-005/006/007 (execute the kill path
and the cleanup to see what each observation actually proves).

# Constraints

Wall-clock budget: 45 minutes. Go tests in the worktree, run them under
load where the finding demands it (this guest has 4 CPUs; concurrent
compile loops are lawful load). Evidence entries are your test runs.

# Expected Return

Version-2 implementer JSON; whatWasDone maps each finding id to its
verdict and the exact design rule it implies.

# Gap Rule

stop and report a gap; never fill it silently.
