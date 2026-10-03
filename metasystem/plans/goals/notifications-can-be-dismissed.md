# notifications-can-be-dismissed

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="Person-facing UI and a per-person dismissal record over existing notifications; no change to what the machinery does"
- Tier: 2
- Intent: A person can dismiss one notification or all of them in the UI's Notifications panel, and they stay dismissed (Wido 2026-10-03, screenshot of the panel with only a close button). Each notification gets a dismiss control and the panel a 'dismiss all'; dismissed items leave the panel and the unread count, survive a reload and a restart, and are recorded per person; a new event raises a new notification. Every UI act is also a verb (Wido 2026-09-30): metasystem notification dismiss ID | --all. While there: the panel shows raw steward and narrator lines ('idle-backlog-dead — claimable backlog has no live claim...', 'narrator: noticing: the shared goal ledger moved 0m ago...'); notifications a person sees say in one plain line what happened and what, if anything, to do (Wido 2026-09-30, every message human-readable); internal-only events do not notify a person.
- Origin: main
- Next step: Short design: where notifications live (/api/notifications, notifications.jsonl), the dismissal record and its scope, the verb, which events reach a person and their plain wording; Astra critique tier 2; build; check in a browser at 390 and 1440 px. Coordinate with fleet-page-redesign (Needs you) so one event is not shown twice.
- OpenedAt: 2026-10-03T07:59:04Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-03T07:59:04Z 8MYGHJVT82SE6WYDH6T15W25XY-m1e-718ba0eb open actor=human:Wido targets=notifications-can-be-dismissed
Integrity: sha256=d42d610164aa2275c655cedd32ccd1c7aac199e2797bb7ae11dd6f25166ca4d3
