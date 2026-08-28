# Go surface consolidation — round 2 dispositions

Critic: design-critic-20260812t065148z-7985 (codex, gpt-5.6-sol).
6 findings, 6 material. All folded into records/misc/go-surface-consolidation.md.

| Finding | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| GSC-R1-008 | accepted | missionrunner's reapReservedRecords/applyReapVerdict is a third independent verdict ladder; two consumers left it standing. | The decision function gets THREE consumers wired in one commit: supervise reaper, mission runner reap path, dispatch.sh via the verb. The engine non-goal is narrowed: engines are not rewritten, but consuming the shared verdict is in scope. |
| GSC-R1-009 | accepted | Centralizing the verdict without the mutation leaves the supervise reaper's unlocked whole-record overwrite, which can clobber a completion landing after its read — a live defect. | Every consumer applies through the locked CAS owner with expected status; the stale-overwrite defect dies in step 3 with a regression test; recorded as the program's one deliberate behavior change. |
| GSC-R1-010 | accepted | The same-family >=3 rule excludes cross-family invariant sequences; the pre-commit guard's classify -> wrapper-token ordering is the type specimen. | Sequence census covers both shapes; each hit records its named invariant and an explicit coarsen-or-document decision. |
| GSC-R1-011 | accepted | Step 1 needs the complete alias table but the maps were later steps' deliverables — an ordering contradiction that also dodged sign-off. | The exhaustive maps (mission 28, proc 9, job 26) are now the document's appendix; the alias table is generated from them and sign-off covers them. |
| GSC-R1-012 | accepted | Evidence collection is repository-wide custody; missions are protected input, not the ownership boundary. | evidence stays its own family; the mission merge covers 7 families, 28 verbs. |
| GSC-R1-013 | accepted | A count-driven util catch-all gives unrelated behaviors one misleading owner; the repo's design standard names this the anti-pattern. | The util merge is withdrawn; gate, report, hooks, event, json, util stay as they are. Applying the same logic, supervise stays out of proc (that fold was pure prefixing). |
