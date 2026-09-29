# Records

This tree holds three things:

- the goal ledger: concluded goals under records/goals/, which the goal
  engine alone mutates under its ledger rules;
- live registers and engine inputs: records/counselor/*.jsonl,
  records/narrator-digest.log, and the one file the engine reads
  (records/misc/goals-migration-manifest.md); the brain seat's role packet
  is compiled into the engine (internal/brain/role-packet.md);
- the working records of goals that are still open (critique rounds, code
  reads, dispositions, facts, designs), plus the few finished records a
  live document or code comment still cites.

A finished goal's working records leave the tree. What is durable in them
is distilled first, without duplicating anything already recorded: rulings
into memory/rulings.md, failure knowledge into docs/doctrine/, decisions
and rationale into the design page of the mechanism or docs/decisions/, and
measurements that still ground a decision into docs/baselines.md. The rest
stays in git history; the removal of 2026-09-28 is tagged
records-archive-2026-09-28.

Records are append-only for agents and humans. Live intent belongs in
plans/ and living registers in memory/.
