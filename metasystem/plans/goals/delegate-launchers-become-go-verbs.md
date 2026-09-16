# delegate-launchers-become-go-verbs

- State: queued
- Risk: severity=2 novelty=2 exposure=2 accumulation=1 basis="severity 2: a broken launcher stops the design and critique lanes on every seat, but corrupts no record; novelty 2: new Go verbs next to metasystem delegate, behaviour copied from working bash; exposure 2: every seat launches designs and critiques through them; accumulation 1: the bash copies are deleted in the same landing"
- Tier: 2
- Intent: Wido 2026-09-16 18:25 CEST: "Land bash now, immediately after (i.e. next goal to pick up) is to port this to Go", after "I hate scripts". The bash launchers that goal:headless-launchers-live-in-the-repo lands under metasystem/scripts/agents/ (headless-design-launch.sh, headless-design-wait.sh, codex-critique-launch.sh, critique-round-task.sh and templates/design-common.md) become metasystem Go verbs, extending metasystem delegate where it fits. landing-lane-worker.sh is NOT ported: the batch verbs of goal:units-land-in-batches-under-one-proof replace it. DONE: (1) Go verbs launch a headless design run with the context-window variable set on the launched process only; wait for it and report exit, result, the real compaction count (system compact_boundary rows only) and page size; launch a Codex critique in its own worktree, follow it to completion, copy the critique out and stop only that worktree broker; and derive the round-N critique task. All outputs go under a verb-owned state directory, with no scratchpad path. (2) Go tests use injected fakes for claude, the codex companion, the clock and the process prober; each rule is proved by a mutation that turns its test red; nothing sleeps on wall time. (3) The bash launchers and their shell fixture are deleted in the landing and nothing references them. (4) The next real design run and critique run on a seat go through the verbs, recorded in this goal Next step.
- Origin: human
- Next step: Opened in the name of Wido by seat m1e from the enrolled pane on his word 2026-09-16 18:25 CEST. Starts as soon as headless-launchers-live-in-the-repo lands (the bash is the behaviour reference). No Fable design round: a seat brief, one Codex critique of the brief, Codex gpt-5.6-sol builds in units of at most 300 lines, Opus reads, the landing lane lands. Before the brief, read units-land-in-batches-under-one-proof design r3: if automatic batch landing needs units built through the job domain for implementation-chain evidence, these verbs are where that evidence is recorded.
- OpenedAt: 2026-09-16T16:29:04Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-16T16:29:04Z CDDQA3HQVS4F93EKGDHHXPPMAP-m1e-c6925449 open actor=human:Wido targets=delegate-launchers-become-go-verbs
Integrity: sha256=c3af544e316b83a1a21191c9658e8e700be11e4cd2f2d9931f2113128044f72b
