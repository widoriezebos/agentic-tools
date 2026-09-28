# The goal system: decisions

- Kind: decision
- Id: 01M3MKDZMJEZE79EBA9X4S5QRJ
- Status: accepted

Distilled 2026-09-28 from `records/goal-system/goal-system-design.md`
(sections "The problem", "Mission hosts", "The ledger"; round 4 finding 1,
rounds 5-7 finding 1, round 3 finding 11), removed that day (tag
`records-archive-2026-09-28`). The mechanism is described in
`docs/backlog-mechanism.md`.

- Accepted blind spot. Intent recorded only where no sensor reads it (a
  backlog under docs/, a TODO in a commit message) is undetectable by
  construction. The machinery does not claim otherwise. It makes declaring
  intent one command (`goal open`) and makes undeclared absence
  unrepresentable: a checkout with no current goal must carry a queue or a
  goal-free declaration, and that declaration expires when the plans-stream
  scan digest changes. The rest is covered by the doctrine that programs
  start with `goal open` (AGENTS.md). The incident behind this: the stop hook
  said "NOTHING LEFT TO WORK ON" while a quarter of a 101-finding program was
  still open in docs/reviews. (goal-system design, 2026-08-15; D71)
- Goal mutation refuses while the checkout has an active mission. "Active" is
  the mission-status rule (runner record plus kernel identity) in
  `internal/missionstate`. Two alternatives were rejected. Lease lineage fails
  for attended missions, because a live-holder launch deliberately keeps the
  prior main's lineage (lease-succession). Environment markers can be
  stripped. An Unknown liveness answer refuses the mutation (fail closed).
- The goal ledger is not an audit log. Prune reports what it dropped on
  stdout, and the decision documents stay the audit surface.
