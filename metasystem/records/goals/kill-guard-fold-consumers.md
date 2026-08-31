# kill-guard-fold-consumers

- State: done
- Intent: m2's janitor fold change (dab1dbd) made an empty member scan INDETERMINATE instead of NOT-OWNED: kill-guard consumers now see exit 3 (fail-closed) where they saw 1 - every m1-side consumer must be verified against the new contract (Ruling R: a changed contract runs its callers)
- Origin: main
- Next step: Appetite: 1h. Enumerate every caller consuming janitor fold/kill-guard exit codes on m1 surfaces (steward sweeps, cancel paths, cleanup scripts), verify each handles exit 3 as fail-closed correctly, fix any that treated 1 as the only refusal; prove with the affected fixtures
- Concluded: Landed c634f103. Enumeration verified all eleven kill-guard exit-code consumers already fail-closed on INDETERMINATE (the one not-owned-only refusal is the documented kill-through); the deliverable became the missing proof: cmd tests asserting exits 3/1/3 (green outside the sandbox including the live case) and a wind-down fixture proving refusal-with-no-signal on exit 3, suite run warm per R-35-m3 with the new scenario green. Residue scheduled: goal:lease-fold-empty-scan (sibling fold defect, brief pre-written) and the census-staleness load flake carried to goal:machine-concurrency-governor as an R-35 specimen. Chain implementer-52116794021542e440523f96 conformance-reviewed and closed.
- OpenedAt: 2026-08-30T14:57:35Z
- Revision: 4
- Budget: elapsedLimit=4h attemptLimit=3 reservedJobMinutesLimit=90 activeJobLimit=1

History:
- 2026-08-30T14:57:35Z BHNDDTA3DWP99EGG7G2BB41N5H-m1-bf243850 open actor=m1+coordinator targets=kill-guard-fold-consumers
- 2026-08-30T15:17:24Z 91F0NGJS9FB66ZBNY0JE2DAADH-m1-bf243850 set-budget actor=m1+coordinator targets=kill-guard-fold-consumers
- 2026-08-31T18:40:06Z QGVTC7WQD4KW5S1Q8RTVWDKS2G-m3-a5da21ff claim actor=m3+mac-m3 targets=kill-guard-fold-consumers
- 2026-08-31T19:26:29Z MP5NM30CQBCZ1WVYAX1Y7JETHJ-m3-a5da21ff done actor=m3+mac-m3 targets=kill-guard-fold-consumers
Integrity: sha256=1195bf6d365dfbfd71f6b3b6f84526573f423b70da42834da16d36ca7a613de7
