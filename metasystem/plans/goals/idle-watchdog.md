# idle-watchdog

- State: queued
- Intent: A machine with open delegated work is never silently idle: an OS-scheduled steward detects open-work-with-no-live-worker and revives the configured agent runtime, receipting and notifying the operator every time (D121)
- Origin: main
- Next step: IMPLEMENTED AND ARMED 2026-08-20 under the covenant: design converged at r5; implementation reviewed six rounds (12-8-7-8-2-AGREE, the round-4 pass including an independent bridge critic); landings through eff7029; armed on the primary checkout with one live tick observed. Remaining scope is the named follow-ups: steward-succession (live-holder takeover), envelope/workspace binding at mint, the zero-outside-write sweep fixture at arming, the linux notifier-refusal leg at the Debian guest validation, and the IW-3/IW-4 new-format legs at the backlog cutover. The interim session cron stays as belt until a full work cycle passes under live ticks; conclusion happens through the new verbs after cutover. Original charter (D121): the open-work predicate reads the goal ledger and transaction journal; worker liveness uses the shipped process identity; revival launches through the adapter seam, agent-agnostic; every revival receipts and notifies. Wido's ruling 2026-08-20: a ten-hour silent stall is inexcusable — this must be machinery, never agent discipline.
- OpenedAt: 2026-08-20T16:50:00Z
- Revision: 1

History:
- 2026-08-22T06:30:55Z BXWE9NXAWCGCTR3MFCE8GDC4P5-widos-m5-pro-bf243850 migrate actor=human:wido targets=idle-watchdog
Integrity: sha256=4d6689e5918a246b4e7f6826eb718dcdeb2ef8fe8b874278ec63a885af55dda6
