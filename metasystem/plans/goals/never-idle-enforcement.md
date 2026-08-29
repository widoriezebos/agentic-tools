# never-idle-enforcement

- State: queued
- Intent: The never-stop standing order becomes machinery: while the backlog holds claimable work, an idle coordinator stop is a defect the system itself catches — the turn verdict blocks a quiet exit when ready work exists and no tracked step is in flight, unless the human explicitly said stop (Wido's order 2026-08-27: never stop with backlog, EVER; the only exception is his explicit stop-or-redirect; canonical doctrine landed in docs/orchestration.md Working-without-the-human)
- Origin: main
- Next step: Appetite: 2h — PULLED INTO THE CURRENT PROGRAM (Wido 2026-08-29: 'that should not be possible, if you stopped we have a bug to fix'; incident: the beds-conversion run concluded and the coordinator idled unwoken until Wido asked, because a CONCLUDED tracked run wakes nobody - nonterminal-jobs judges only dead processes, run watch is silent on conclusion, and the old idle-watchdog's coordinator-wake leg never crossed into the rebuilt steward generation). Scope: the steward tick detects standing work or unconsumed concluded-run results beside an idle owner and ACTS - notify plus the D121-proven revival path; the wake chain is machinery end to end, no session-side watcher may ever be the last belt. Design with L14 watch (Ruling M) as one law; land before or with it
- OpenedAt: 2026-08-27T19:28:19Z
- Revision: 3
- Budget: elapsedLimit=3h attemptLimit=6 reservedJobMinutesLimit=60 activeJobLimit=1

History:
- 2026-08-27T19:28:19Z AYAW58WHG5BVD72MDT3MSNWCP4-m1-bf243850 open actor=m1+coordinator targets=never-idle-enforcement
- 2026-08-29T11:34:34Z Z7HFZBBAXETRMPG1Y2T7Z0BAY9-m1-bf243850 edit actor=m1+coordinator targets=never-idle-enforcement
- 2026-08-29T19:36:28Z EF4TRWJGGK3G2W28GJGP9MJW41-m2-bc1be9cb set-budget actor=human:wido targets=never-idle-enforcement
Integrity: sha256=7b1c2a2a4549daddef79eb3dec318b8c2e006796d7aa7f47255d0900610fc3a4
