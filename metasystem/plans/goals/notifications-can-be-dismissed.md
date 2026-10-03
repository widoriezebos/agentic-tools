# notifications-can-be-dismissed

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="Person-facing UI and a per-person dismissal record over existing notifications; no change to what the machinery does"
- Tier: 2
- Intent: Notifications in the UI are short, plain and under the person's control (Wido 2026-10-03, on a red block covering the whole right side of the Backlog page: 'the enormous red block, which I think is a notification, is complete and total useless. This needs a proper UX fix'; and earlier that day: 'I want a way to dismiss (all) notifications or a single notification'). Today a notification can be the steward's whole health verdict dumped raw: all 23 checks with pids, generations, token counts, file paths and remedies, as one unreadable toast. Wanted, from a proper UX design: (1) a notification is one plain line saying what happened and, if anything, the one thing to do, with a link to where it is handled; details open on demand, never in the toast; (2) a toast has a fixed small size, never covers the page, and leaves by itself or on dismiss; (3) the person can dismiss one or all, and they stay dismissed across reloads and restarts; a new event raises a new one; (4) only events a person can act on notify; internal verdicts (a standing 'unhealthy', 'the ledger moved', steward revivals) do not, and an unchanged verdict is not repeated; (5) every UI act is also a verb: metasystem notification dismiss ID | --all.
- Origin: main
- Next step: UX design first, by a UX step back like fleet-page-redesign's: what the person needs from a notification, the kinds of events and which notify at all, one message shape, the toast and the panel with mocks (one event, many events, long text, phone width), where details live. Evidence: agentic-tools-evidence/notifications-20261003/ (red-block-backlog-page.png, panel-no-dismiss.png). Then: where notifications are produced (/api/notifications, notifications.jsonl, the steward's HEALTH line), the dismissal record, the verb; Astra critique tier 2; build; check in a browser at 390 and 1440 px. Coordinate with fleet-page-redesign (one event is not shown twice), health-is-green-when-the-seat-is-healthy (the health verdict stops being permanently red) and steward-acts-on-behaviour-patterns (one message per incident).
- OpenedAt: 2026-10-03T07:59:04Z
- Revision: 4
- BudgetExceptions: 0

History:
- 2026-10-03T07:59:04Z 8MYGHJVT82SE6WYDH6T15W25XY-m1e-718ba0eb open actor=human:Wido targets=notifications-can-be-dismissed
- 2026-10-03T07:59:18Z DTAQQZB63K5FCEXJ7RTF3RH905-m1e-718ba0eb approve actor=human:Wido targets=notifications-can-be-dismissed
- 2026-10-03T10:58:01Z FFXV1NZ24N13ABSNADR8KWE1QP-m1e-718ba0eb unapprove actor=human:Wido targets=notifications-can-be-dismissed reason=Wido 2026-10-03 widened it: the notification itself needs a proper UX, not only a dismiss control
- 2026-10-03T10:58:07Z 2WJD8PG1M1D5Y5SWPYNH8A6TCP-m1e-718ba0eb edit actor=human:Wido targets=notifications-can-be-dismissed
Integrity: sha256=d75e8cbc94bf34ee9d545895d56df9f3cf55b8625f828e73f5aa71e34b581069
