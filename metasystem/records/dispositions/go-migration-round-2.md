# Dispositions: go-migration, round 2 — the restructure round

Round 2 (gpt-5.6-sol, job go-migration-plan-r2, 13 material of
which 7 critical, NOT-CONVERGED). The count ROSE (11 -> 13) and
three findings were structural — the supervision chain's lesson
says restructure, not line-fold, so this round reshaped the plan:
Phase 0 no longer flips production (the FLIP PROTOCOL is its own
step gated on Phase 0b), the matched-pair comparison moved INSIDE
each cutover window while both engines still exist, and adoption
gained the deployment story without which no benchmark target
could run Go at all. All thirteen accepted.

| Finding id | Disposition | Amendment |
| --- | --- | --- |
| GO-MIG-R2-001 | accepted | Phase 1 lease text contradicted port-then-fix. The lease is a PORT without exception: KI-33 reproduced and proven, fixed first thing post-cutover. |
| GO-MIG-R2-002 | accepted | Complete inventory added: every python file and decision-bash script classified PORT / REPLACEMENT / RETIRE / STAYS-SHELL, test scaffolds carried one-for-one. |
| GO-MIG-R2-003 | accepted | Oracle: normalized fields are schema-validated BEFORE erasure and normalization is relational (equal sources -> equal placeholders both sides), so wrongness and cross-record identity survive the wash. |
| GO-MIG-R2-004 | accepted | The refusal-message interface became an artifact: a versioned refusals manifest referenced by harness and fixtures; changing a refusal string fails the oracle until the manifest changes with review. |
| GO-MIG-R2-005 | accepted | Census replay records the FULL decision bundle (table + signatures + config + state bytes); replay consumes nothing live. |
| GO-MIG-R2-006 | accepted | Observes-never-kills gets its own negative-proof fixture: kill-shaped candidates, zero signals, instrumented at the kill seam. |
| GO-MIG-R2-007 | accepted | The flip quiesces first: supervision down, zero non-terminal jobs, lifecycle locks free — verified by the flip script; the fingerprint then forces the new generation. |
| GO-MIG-R2-008 | accepted | Phase 0 builds and proves; it does not flip. The FLIP PROTOCOL requires Phase 0b (gate + janitor + the negative-proof fixture). |
| GO-MIG-R2-009 | accepted | The seam tripwire became three executable lines over engine-seams.json ({seam, retireWhenExists}); no plan-row parsing. |
| GO-MIG-R2-010 | accepted | Adoption ships the binary with provenance (source sha + binary sha256, recorded in the target); a suite gate pins adopted hash to built hash. Cross-arch is out of scope and loud. |
| GO-MIG-R2-011 | accepted | Rollback rehearsal runs on top of CURRENT head immediately before deletion, and the warranty window is bounded: valid until the next contract-touching change, then roll-forward is the contract. |
| GO-MIG-R2-012 | accepted | The shell control runs INSIDE the cutover window (control -> flip -> candidate -> soak -> delete); the impossible terminal Phase 5 is gone. |
| GO-MIG-R2-013 | accepted | The pair's pass bar includes mission outcomes: acceptance, coverage, and every validity gate equal or better under go — mechanical metrics alone bless nothing. |
