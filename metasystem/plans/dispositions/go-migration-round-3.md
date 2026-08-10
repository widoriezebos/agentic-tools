# Dispositions: go-migration, round 3 — the folds' plumbing, and the folds

Round 3 (gpt-5.6-sol, job go-migration-plan-r3, 13 material of
which 6 critical, NOT-CONVERGED). Two findings (R3-010, R3-011)
exposed that round 2's corresponding folds NEVER LANDED: the fold
script used unasserted string replacement and mid-line anchors
missed silently. Process correction adopted permanently: every
fold asserts its anchor count and writes after EVERY replacement,
never only at script end (a later assert lost four in-memory folds
before this rule). All thirteen findings verified and folded;
every GO-MIG-R3-xxx id now greps in the plan — the check itself is
part of the fold procedure now.

| Finding id | Disposition | Amendment |
| --- | --- | --- |
| GO-MIG-R3-001 | accepted | The flip's prerequisite bar is enumerated: exactly the Phase 0 owner-alone file, the Phase 0b gate+janitor file (with the syscall-seam negative proof), and S4 under go; deferred rows gate their own later cutovers; Phase 0b is a defined phase. |
| GO-MIG-R3-002 | accepted | The control cohort breaks quiescence by design; RE-QUIESCE (2b) runs the identical verification after the control and before any edit — single-writer attaches to the flip instant between two verified quiescent states. |
| GO-MIG-R3-003 | accepted | The census bundle schema is the instrumented python classifier's own logged read/query closure; replay fails any go-side read outside the bundle. Enforced, not promised. |
| GO-MIG-R3-004 | accepted | adopt.sh reclassified: decision core is a Phase 4 port (`metasystem adopt`), thin copy layer may stay shell, declared as seam 3 until then. |
| GO-MIG-R3-005 | accepted | engine-inventory.json mirrors the classification; the suite reds any scripts/*.py without one. |
| GO-MIG-R3-006 | accepted | The refusals manifest holds COMPLETE lines with named placeholders; end-to-end matching, no unmatched extra refusal lines. |
| GO-MIG-R3-007 | accepted | Normalization asserts derived constraints pre-wash: temporal order, dual-representation agreement, duration arithmetic. |
| GO-MIG-R3-008 | accepted | Stateful replay: seeded identical sandboxes + a complete effects set per verb; untouched files must equal the seed on both sides. |
| GO-MIG-R3-009 | accepted | The normalization pass gains a permanent grep guard: any direct invocation of a ported original outside its wrapper is red. |
| GO-MIG-R3-010 | accepted | The executable seam tripwire (engine-seams.json, retireWhenExists artifact paths) is now actually in the plan. |
| GO-MIG-R3-011 | accepted | The negative proof asserts at the SYSCALL SEAM: a recording no-op signal implementation must show an EMPTY request list — zero requested signals, not zero casualties. |
| GO-MIG-R3-012 | accepted | Deployed targets roll back by re-adoption from the tagged pre-cutover commit (recorded template source commit in provenance); soak-window payloads still carry the shell originals. |
| GO-MIG-R3-013 | accepted | Scorecards bind engine, template commit, binary sha256, and config fingerprint, verified against provenance at extraction — an unbound pair fails the protocol. |
