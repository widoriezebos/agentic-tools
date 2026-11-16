# a-runner-is-judged-by-its-progress

- State: queued
- Risk: severity=3 novelty=2 exposure=3 accumulation=1 basis="On 2026-11-16 seat m1e had no session for 5 h 17 min: the watcher's repair replaced a slow, working runner about every 47 minutes (a 27 minute tick plus the 20 minute freshness window), before it reached the seat start; machine revive and system start changed nothing; nobody was asked. Wido: the wall-clock check is naive and brittle across hardware; design it, have Astra critique it, build it at high priority. Analysis /Users/wido/agentic-tools-evidence/lane-test-20261009/designs/analysis-a-slow-runner-is-killed-as-dead-2026-11-16.md (ANALYSIS STANDS, Astra round 2). Design /Users/wido/agentic-tools-evidence/lane-test-20261009/designs/a-runner-is-judged-by-its-progress.md (DESIGN STANDS, Astra round 4, after 9, 5 and 2 material findings in rounds 1 to 3, all folded; findings /Users/wido/agentic-tools-evidence/lane-test-20261009/designs/astra-runner-progress.r1 to .r4.findings.md)."
- Tier: 3
- Intent: A steward runner whose process is alive is never replaced because of elapsed time. It publishes what it is doing; one liveness reading replaces the window of twice the cadence; replacement on liveness remains for a dead process and for ticks that keep failing, and authority (an obsolete engine, an authorised restart) decides as today. A replacement is one resumable transition that honours the announced drain and never blocks the watcher. Replacements that do not help end at three with a question; a live runner that makes no progress for thirty minutes is asked about; a dead main session with an eligible successor gets it or a question, and machine revive starts the session or returns the reason.
- Origin: main
- Next step: Build from the agreed design page /Users/wido/agentic-tools-evidence/lane-test-20261009/designs/a-runner-is-judged-by-its-progress.md; it is the design, no new design is written. The default roster cuts one brief per unit from it (each starts with an inventory and re-counts its estimate; split at the seam the units table names when over 250 production lines), one Astra round per brief. Units in order: U1 the runner publishes progress and counts its tick results; U2 a replacement is a resumable transition; U3 liveness by reading; U4 bounded by outcome, and a person is asked; U5 the successor: a revive that starts it, and a question when it waits (--last). Acceptance is section 5 of the page, including the live observation on m1e. Roll-out (D9) is part of the goal: after deploy a person restarts supervision in each checkout (a1 does it in Wido's name), the hand-in records the running digests, and only then the workaround git config metasystem.steward.tick-seconds 3600 in the m1e checkout is removed.
- OpenedAt: 2026-11-16T10:00:01Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-16T10:00:01Z J7CXHA8B969HVWHXY531MNHDQJ-landing-373c4177 open actor=human:Wido targets=a-runner-is-judged-by-its-progress
Integrity: sha256=e34851ec9a16e96f10ad797adca289a75cacf0bbf7077458eb444056a87cace3
