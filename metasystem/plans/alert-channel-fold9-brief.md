Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal alert-escalation-channel)
Date: 2026-09-01

# Goal

Revision 9 of metasystem/plans/alert-channel-design.md: fold or refute, by id,
all six material findings of the Sol round-2 critique of revision 8. The
verbatim register is at records/misc/alert-channel-critique-r8.md (in your
worktree); the critic's full return with quoted evidence is durable at
artifacts/agents/design-critic-82a0663b42cbce77c0ffc515/rounds/1/return.json.

# Workspace

The delegate worktree the dispatcher created for this job. Revise exactly one
file: metasystem/plans/alert-channel-design.md (revision 8 is in the
worktree). The shipped code the critic cited (internal/steward,
internal/dispatch, internal/supervise, the janitor/evidence-gc surfaces) is
ground truth for what exists today.

# The mandate

The critical first: AC8-JOB-SOURCE-RETENTION-001 — the delegate-job-failed
scan derives from job records that garbage collection can remove, so an
outage longer than retention silently re-opens the permanent-loss window the
scan exists to close. Fold decisively; the honest options the register's
evidence suggests: pin terminal-failed job records against collection until
their alert episode is durably journaled (a retention handshake), or journal
the alert intent in the same critical section that terminalizes the record
(journal-first handoff), or bound the exposure explicitly and state the
accepted loss window in the design's own honesty section. Pick one; prove
the crash and outage interleavings for it.

The five high findings: AC8-STOP-BATCH-BINDING-001 (the stop scan must
require the same evidence the resume command checks — alert only what resume
will accept); AC8-STOP-RESUME-RACE-001 (a human resume racing the scan must
suppress the stale stop alert — state the ordering rule);
AC8-SCAN-BOUNDEDNESS-001 (bound both scans: a durable cursor or watermark,
enumerated read set, no unbounded history walk under tick locks);
AC8-STOP-INDETERMINATE-LIFECYCLE-001 (one rule for a COMPLETE batch turning
unreadable: suppress, hold, or re-alert — no implementer guess);
AC8-ANSWER-BYTES-AND-ACTION-001 (byte-exact Answer composition for both
producer classes, including the exact resume command arguments).

Then the self-consistency pass over every changed rule and its touched
sections, named pairs in the status line; self-grade updated; the reject
condition stays a third implementer gap-stop (two have happened).

# Constraints

Wall-clock budget: 30 minutes. No design content changes beyond the six
resolutions and the pass. Wido's standing words untouchable (adapter
abstraction, Telegram first, session bridge second consumer, Slack threading
via conversation identity, both producer classes, the derivation-scan
direction itself).

# Expected Return

Version-2 implementer JSON; diffBoundary exactly
metasystem/plans/alert-channel-design.md; whatWasDone maps each finding id to
its resolution in one line.

# Gap Rule

stop and report a gap; never fill it silently.
