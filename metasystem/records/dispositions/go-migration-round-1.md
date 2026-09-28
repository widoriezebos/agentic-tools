# Dispositions: go-migration, round 1 — the equivalence story under fire

Round 1 (gpt-5.6-sol, job go-migration-plan-r1, 11 material of
which 6 critical, NOT-CONVERGED). The plan was one pass old; sol
cut at every soft joint in the equivalence and cutover story. All
eleven accepted and folded the same hour.

| Finding id | Disposition | Amendment |
| --- | --- | --- |
| GO-MIG-R1-001 | accepted | Fixtures bind directly to python files, so an engine switch in wrappers reroutes nothing. Fold: a NORMALIZATION PASS per port (inventory direct bindings, rewrite to wrappers, land as reviewed no-op) precedes any switch. |
| GO-MIG-R1-002 | accepted | Ports with known defects (KI-33) cannot be equivalent AND fixed. Fold: PORT-THEN-FIX — ports reproduce recorded defects exactly (named per KI); fixes land after cutover as separate loop-reviewed changes. |
| GO-MIG-R1-003 | accepted | "Semantic normalization"/"message classes" were weasel words. Fold: executable oracle — exit codes exact, bytes exact after ENUMERATED normalizations, stderr by the fixtures' own anchored patterns. |
| GO-MIG-R1-004 | accepted | Replay corpora miss live and clock-dependent branches. Fold: replay is necessary-not-sufficient; each port names covered/uncovered branch classes and the fixture (with injected clocks) reaching each uncovered one. |
| GO-MIG-R1-005 | accepted | Live shadow scans see different instants; "unexplained divergences" invites explaining defects away. Fold: RECORDED-INPUT replay — the watcher records the exact table it classified, Go replays it offline; zero divergences, no explained category. |
| GO-MIG-R1-006 | accepted | An env-var engine flag is process-local: two writers. Fold: the engine selector is a metasystem.conf key — one source per checkout, fingerprint-covered, so flips force re-arm through the existing mechanism. |
| GO-MIG-R1-007 | accepted | Phase 0's arm needs the lease, deferred to Phase 1 — an undeclared seam. Fold: seam declared (lease ops stay in the shell arm wrapper; the Go OWNER touches no lease by design, D-1/REG-7), with a retirement tripwire. |
| GO-MIG-R1-008 | accepted | Phase 0 acceptance was ambiguous (five cases vs whole Proof list). Fold: an ENUMERATED owner-alone fixture set; every non-owner Proof row named in an explicit deferral list with its phase. |
| GO-MIG-R1-009 | accepted | The census seam had no mechanical retirement gate. Fold: a suite-visible seam registry keyed to plan phase rows; a seam outliving its phase turns the suite red. |
| GO-MIG-R1-010 | accepted | Deleting the original deleted the only rollback path. Fold: tagged cutover commits carrying the revert-and-re-arm recipe, one sandbox REHEARSAL of that recipe before any deletion, and a soak week between flip and deletion. |
| GO-MIG-R1-011 | accepted | Phase 5 lacked a control. Fold: a MATCHED PAIR — shell control cohort and go candidate, same spec/fences/roster/machine, compared metric-by-metric in the results; go must be no worse everywhere and zero-leak. |
