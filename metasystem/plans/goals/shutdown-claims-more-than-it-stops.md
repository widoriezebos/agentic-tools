# shutdown-claims-more-than-it-stops

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="severity 2: an operator reads stopped and believes nothing runs, while a live runner still ticks and can dispatch; novelty 1: either stop the runner too or stop claiming it; exposure 2: every fixture bed, the proof-run watchdog and every human who used this path before the stop verb existed; accumulation 1: one fix"
- Tier: 2
- Intent: scripts/agents/arm-supervision.sh --shutdown (the internal long form of metasystem up --shutdown) stops the supervision owner, watcher and reaper and prints 'up outcome=stopped', but leaves the steward runner alive. Seen twice on m1d 2026-09-07 at 11:00 and 11:02 CEST; both times the runner pid had to be found and killed by hand. The printed aggregate claims a completeness the path does not deliver. DONE means the line says only what it did (supervision stopped, the runner untouched) or the path stops the runner too, decided deliberately; a fixture asserts the process table matches the printed claim
- Origin: main
- Next step: the stop verb goal metasystem-stop-verb supersedes this path for humans and its own scenarios assert the runner identity dead, so this goal is about the internal long form that fixtures and the proof-run watchdog keep calling. Read internal/up's Shutdown and the ShutdownReport that goal adds; the honest minimum is that the aggregate line names the components it actually stopped. Do not widen it into stopping the runner without deciding whether the watchdog's mid-suite shutdown should end the runner as well
- OpenedAt: 2026-09-07T09:18:45Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-07T09:18:45Z SATX6J9XJE6AB7J3XMAHWQA89C-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=shutdown-claims-more-than-it-stops
Integrity: sha256=f79ea99df99b90a9dc98c277d969ffb5b3bfcfedb1dd3e3772523b9003f2eaf2
