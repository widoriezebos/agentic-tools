Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal alert-escalation-channel)
Date: 2026-09-01

# Goal

Revision 12, one-finding fold of metasystem/plans/alert-channel-design.md:
AC11-ANSWER-JOURNAL-ELIGIBILITY-001
(metasystem/records/misc/alert-channel-critique-r11.md, landed, in your
worktree). The advertised remedy command must be VALID AT JOURNAL TIME or
honestly absent: the producer currently derives the action from the failed
record alone, while the shipped dispatcher also gates on chain closure, the
record being the chain's newest member, and the reviews target still
existing.

# The fold

Specify journal-time eligibility verification: before advertising a
command, the producer checks the same preconditions the dispatcher enforces
(chain not closed; the failed record is the newest chain member; for
critic-role commands, the referenced implementer record still exists), and
on any failed check the row's action degrades to none-with-reason carrying
the exact failed precondition. State the check's read set inside the
already-bounded scan contract (the chain member listing the producer
already touches). Fix the lines equating reviews-field immutability with
reference validity. Extend the design's traced-facts with the dispatcher
gates the critic cites (metasystem/scripts/agents/dispatch.sh follow-up
gating). Consistency pass over touched sections; self-grade; reject
condition unchanged.

# Constraints

Wall-clock budget: 20 minutes. One finding, nothing else.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly
metasystem/plans/alert-channel-design.md (that one file).

# Gap Rule

stop and report a gap; never fill it silently.
