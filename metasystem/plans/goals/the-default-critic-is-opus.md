# the-default-critic-is-opus

- State: approved
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="Three default strings and a config lookup path; a wrong value shows as the model name on the next critic's record"
- Tier: 1
- Intent: The engine's default models for the code critic, the design implementer and the design launch are claude-opus-5-5, not claude-fable-5-1 (decision 20: Opus 5.5 only, Fable is not used), so a worktree without the installation's local roster still reads on Opus; and a worktree made by the engine for a goal reads the installation's metasystem.conf.local roster rather than falling back to defaults (finding 438)
- Origin: main
- Next step: On m1e's build lane after a-live-read-proves-its-subject: opus-builder changes the three defaults (internal/config/defaults.go:326, :328, :424) and, if a worktree's config resolution has no path to the installation's .local, adds that path in the config loader for engine-made goal worktrees; tests: the default table and a worktree bed that resolves role.code-critic.model.claude to the installation's local value; Read each round: yes; the steward submits with work review --changes --last; about 15 production lines
- OpenedAt: 2026-10-27T18:39:32Z
- Revision: 2
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-27T18:42:40Z revision=2 opid=K51MCDZ5PVF4EW7BZCP2V12Z4H-m1e-75099fe5 authority=proven digest=403960a2c24b30d30bd9cdea2b94b3c406014844e3588ac0dfe0ffbd88ed0b75 episode=2

History:
- 2026-10-27T18:39:32Z ZHMBM767HBTA5B0VVKW87X2T5W-m1e-75099fe5 open actor=human:Wido targets=the-default-critic-is-opus
- 2026-10-27T18:42:40Z K51MCDZ5PVF4EW7BZCP2V12Z4H-m1e-75099fe5 approve actor=human:Wido targets=the-default-critic-is-opus
Integrity: sha256=30eb9cf9c6d7f1de5271f0fcc42c90d3235d55a82986d97007563b8a9e80ec6d
