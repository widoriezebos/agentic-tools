# seat-machines-shed-leaked-processes

- State: queued
- Risk: severity=3 novelty=2 exposure=3 accumulation=3 basis="severity 3: a day of leaks drives load past 30 and holds about 24 GB, slowing every seat and causing load-shaped false reds; novelty 2: reaping across several owners (Codex brokers per worktree, fixture stewards, seat waiters, indexing); exposure 3: every seat, every day; accumulation 3: grows until a reboot"
- Tier: 3
- Intent: On 2026-09-15 the shared seat machine ran at a load of 25 to 37 and held about 24 GB in processes whose owners had ended: 69 Codex app-servers (38 of them over a day old) with 512 broker and plugin helper processes holding about 21 GB, 78 orphaned fixture stewards and helpers (some about four days old, 2.1 GB), and a seat shell busy-looping at 56 percent CPU for 12 hours; macOS Spotlight, XProtect and WindowServer took about 105 percent CPU indexing and scanning the fixture repositories and test binaries the beds create. The load slows every seat and turns tests red: engine policy timeouts, deadline scenarios, and 15 to 36 second stop hooks. DONE: root causes diagnosed per class with code locations; a design page for the fix, critiqued by a model other than its author; the fix built so that no process outlives its owner (suite, job, worktree, session) beyond a bounded time, a census verb names any process that does, a seat waiter never busy-loops, and fixture roots and build output stay out of Spotlight indexing; proven by a day of normal three-seat work that ends with no leaked process and a lower load. Relates to fixture-stewards-outlive-their-suite.
- Origin: human
- Next step: Diagnose first, from live evidence (scratchpad leak-evidence-2026-09-15.txt on m1e): for each leaked class (Codex companion brokers and app-servers per worktree, fixture stewards reparented to launchd, the busy-loop seat shell, Spotlight and XProtect churn on fixture roots) name its owner, its creation path, and why teardown misses it, with code locations. Then a Claude Fable design page for the fix, one Codex critique round, and a build in units.
- OpenedAt: 2026-09-15T06:46:29Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-15T06:46:29Z SESZEVKK9N09W21BZ92BBBBTGP-m1e-c6925449 open actor=human:Wido targets=seat-machines-shed-leaked-processes
Integrity: sha256=255246f0199a7101d16f0c529cb94bec9a1ecce292a9679a2f060349f74bf811
