# Supervision custody per checkout: code review, third round (chain scc-build3-cc1)

Reviewed tree 4fa56264dd35a8da83d3af51dc909b1204766cd0 (chain scc-build3, round 1). Critic: Claude Fable 5.1. Three material findings; the goal's three review rounds are spent; the work does not land.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| SCC-31 | accepted, not landed | Owners now record the git top-level as their checkout path while every existing row holds the state root; on this repository the two differ, and the new guard refuses re-arm, generation replacement, shutdown and dead-owner takeover on mismatch before any liveness check. Landing it locks out every armed checkout. | The recorded path form must stay the state root (the writers' existing canonical form), or a migration must read both; no landing until the invariant test covers an owner armed by the previous binary. |
| SCC-32 | accepted, not landed | The guard refuses when the registry has no row for the lock owner's tag, and it runs before liveness; an owner that died before its first row is a permanent lockout. | The guard applies only to a live owner; a dead owner without a row is taken over as before. |
| SCC-33 | accepted, not landed | The self-check rejects only the suite shell's pid as main; the stop-hook scenario resolves the seat's agent process through the ancestor walk and arms with it, which the rule forbids and the check accepts. | The self-check rejects any main pid outside the scenario's own bed, and the scenario creates its own main. |
| SCC-34 | noted | The same-main and different-main sub-tests exercise identical code; the sweep leg covers compaction, not the live tag sweep. | with the rework |
| SCC-35 | noted | Selection depends on rows the contract classes as droppable; once compaction is wired, live owners' checkout rows vanish. | with the rework: key on rows that survive the contract. |
| SCC-36 | noted | Duplicated default path, a three-argument Shutdown that only fits the test's row form, and a library package logging to stderr. | with the rework |
