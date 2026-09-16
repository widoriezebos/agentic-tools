# headless-launchers-live-in-the-repo

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="severity 2: if the scratchpad copies are lost, the design and critique lanes stop on every seat, but no record is corrupted; novelty 1: the scripts already work and only move; exposure 2: every seat launches designs, critiques and the landing lane worker through them; accumulation 2: each day adds private copies that drift"
- Tier: 2
- Intent: Replaces delegate-launch-sets-its-own-context-window, concluded by Wido 2026-09-16 because its design grew too large over three rounds. The launchers that ran every headless design since 2026-09-16 with zero compactions exist only in seat m1e scratchpad /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/5afd308c-3804-42b1-87c4-4547e1460fa9/scratchpad: fable-design-launch.sh (20 lines), wait-design.sh (10), codex-crit-launch.sh (33), make-crit-r2-task.sh (14), design-common.md (8), lane4-worker.sh (27), plus the round 3 task maker make-crit-r3-task.py (24) in the 19c52dfc scratchpad. They are not in git, a reboot deletes them, and one seat knows them. DONE: (1) they live under metasystem/scripts/agents/ (the design rules as a template under scripts/agents/templates/) with no seat scratchpad path in them: output directory, worktree root, model and resume session are arguments or environment inputs, and the headless launch sets CLAUDE_CODE_AUTO_COMPACT_WINDOW on that process only; (2) a shell witness runs the design launcher and the critique launcher against a fake claude and a fake codex on PATH and asserts the window variable, the model, --resume when a resume session is given, and the pid and result files; the mutation that drops the window variable turns it red; no wall-time sleep; it runs under bash 3.2; (3) the next real headless design run launched from the repo copy shows zero compact_boundary lines in its transcript (no dedicated run); (4) the seats switch to the repo copies, recorded in this goal Next step.
- Origin: human
- Next step: Opened in the name of Wido by seat m1e from the enrolled pane on his word 2026-09-16 17:58 CEST: "I follow your recommendation: conclude this goal and replace it with a small one we build now". No design round: Codex gpt-5.6-sol builds in a worktree from a seat brief (about 150 lines), an Opus reader reads, and the landing lane lands it. Witness (3) is the next real design run, not a dedicated one.
- OpenedAt: 2026-09-16T16:00:21Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-16T16:00:21Z SN9D1TKKK8VF1HRR0DPWQHF2PG-m1e-c6925449 open actor=human:Wido targets=headless-launchers-live-in-the-repo
Integrity: sha256=004eca723be057faf7bd658d771d44c855bd9b68543283ed32dd2af58a3c77fb
