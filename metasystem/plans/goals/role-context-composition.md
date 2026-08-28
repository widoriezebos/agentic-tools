# role-context-composition

- State: queued
- Intent: Each role's context is COMPOSED from the memory it is allowed to see, never inherited ambiently - leaking extra memory pollutes or poisons the role (Wido 2026-08-28); paper ch.8 is the law: per-role views of one authoritative record, and a fresh mind is the only fresh perspective
- Origin: human
- Next step: Appetite: 4h — design first (no build): read docs/paper/08-memory-and-coordination.md; inventory every channel that injects context today (CLAUDE.md/MEMORY.md auto-load, wow.md, AGENTS.md, coordinator-written briefs, dispatch env); map paper ch.8's builder/examiner/custodian/auditor visibility table onto our real roles (builder codex, critic codex, coordinator, steward, narrator); design the mechanism that composes a role's context FROM an allowlist (what memory kinds each role may read, who owns the table, how delegate L13 enforces it at brief-assembly time, how violations are refused not discouraged); the fresh-mind law must be mechanical: a critic never receives the builder's path. Design critiqued by codex, then to Wido before any build.
- OpenedAt: 2026-08-28T16:13:07Z
- Revision: 1

History:
- 2026-08-28T16:13:07Z FF3MMBX4JSAA6P36F8QD8BT6CV-m1-bf243850 open actor=human:wido targets=role-context-composition
Integrity: sha256=726b760fd273998c520a87e3e079fc6ec3516cb04a4a152b71f2fa9ed315520c
