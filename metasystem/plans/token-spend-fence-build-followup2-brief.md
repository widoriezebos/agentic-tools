Working Mode: implement
Orchestrator Identity: m1b+main-1788333346-60696-6a3256 (dispatch delegate under goal token-spend-fence)
Date: 2026-09-03

# Correction

Your gap is adjudicated (recorded in
metasystem/plans/token-spend-fence-dispositions-closing.md, gap answers):
the recommended correction is AUTHORIZED. `mission.JobMeasurement` gains
a `Source` field carrying the derived round's source value exactly as
`AggregateUsage` persists it today (metasystem/internal/mission/fence.go
around line 715), empty for reported, unavailable and unreadable
outcomes; `AggregateUsage` copies it unchanged so its output bytes and
`TestAggregateUsageSumsTerminalJobs` stay as they are. No second
derivation path; nothing else in the aggregate changes. The spend
reader may ignore `Source`.

Everything else in metasystem/plans/token-spend-fence-build-brief.md
stands. Resume the build in your worktree from that brief; same
expected return and evidence commands.

# Gap Rule

stop and report a gap; never fill it silently.
