# the-default-critic-is-opus

- State: queued
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="Three default strings and a config lookup path; a wrong value shows as the model name on the next critic's record"
- Tier: 1
- Intent: The engine's default models for the code critic, the design implementer and the design launch are claude-opus-5-5, not claude-fable-5-1 (decision 20: Opus 5.5 only, Fable is not used), so a worktree without the installation's local roster still reads on Opus; and a worktree made by the engine for a goal reads the installation's metasystem.conf.local roster rather than falling back to defaults (finding 438)
- Origin: main
- Next step: On m1e's build lane after a-live-read-proves-its-subject: opus-builder changes the three defaults (internal/config/defaults.go:326, :328, :424) and, if a worktree's config resolution has no path to the installation's .local, adds that path in the config loader for engine-made goal worktrees; tests: the default table and a worktree bed that resolves role.code-critic.model.claude to the installation's local value; Read each round: yes; the steward submits with work review --changes --last; about 15 production lines
- OpenedAt: 2026-10-27T18:39:32Z
- Revision: 1
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-10-27T18:39:32Z ZHMBM767HBTA5B0VVKW87X2T5W-m1e-75099fe5 open actor=human:Wido targets=the-default-critic-is-opus
Integrity: sha256=50a5b71d799cccaee12ee9dadbf63687f363a31b084f6a3012ad661c6236552e
