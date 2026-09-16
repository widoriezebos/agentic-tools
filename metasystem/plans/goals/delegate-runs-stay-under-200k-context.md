# delegate-runs-stay-under-200k-context

- State: queued
- Risk: severity=2 novelty=2 exposure=2 accumulation=1 basis="severity 2: premium-priced calls and budget outages, no corruption; novelty 2: measurement from transcript usage rows the seat already reads by script, a refusal on the launch path; exposure 2: every headless design and critique run; accumulation 1: one measurement, one refusal code, one policy value"
- Tier: 2
- Intent: Token audit 2026-09-16: 1,604 calls above 200K context (premium priced) before 13Z on the seats and zero after the 200K compaction window landed at 14:25Z; the uncapped headless delegates in worktrees still cross it (crit-sgh 76 calls above 200K at a 172K average, crit-blb 57, crit-bss 30) because designs run without compaction on purpose (zero compact_boundary is a DONE of headless-launchers-live-in-the-repo). Wido 2026-09-16: high priority under the efficiency program. DONE: (1) the Go launcher of goal delegate-launchers-become-go-verbs records for every run its peak context (input plus cache-read plus cache-write of the largest call, from the runtime transcript usage rows), the count of calls above 200K and the compaction count, in the run result. (2) A brief input set (page, brief, answers, cited files, common template) is measured before launch; a brief whose inputs exceed the admitted size is refused with a named refusal code that lists the oversize inputs, never launched; the admitted size is a policy value with a basis (default 120K tokens so that a 120-tool-call run fits under 200K). (3) Go tests with fake transcripts and fake inputs prove the peak computation, the refusal, and the run result; a mutation that skips the measurement or launches an oversize brief turns red. (4) The next real design run and critique run report a peak under 200K, recorded in this goal Next step.
- Origin: human
- Next step: Opened in the name of Wido by seat m1e from the enrolled pane on his word 2026-09-16 20:25 CEST. Starts after delegate-launchers-become-go-verbs lands (the verbs carry the measurement). Seat brief with one Codex critique, Codex gpt-5.6-sol builds, Opus reads, the landing lane lands. Until then the seat sizes every brief by hand to fit.
- OpenedAt: 2026-09-16T18:20:42Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-16T18:20:42Z 6BSNB388RHH7QDN6TB1QMKFX77-m1e-c6925449 open actor=human:Wido targets=delegate-runs-stay-under-200k-context
Integrity: sha256=0025200fef9d0f77884860804f44cda03e37c5e3d51da319e35afedaee30a0ed
