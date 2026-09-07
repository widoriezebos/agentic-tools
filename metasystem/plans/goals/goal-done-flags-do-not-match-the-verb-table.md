# goal-done-flags-do-not-match-the-verb-table

- State: queued
- Risk: severity=1 novelty=1 exposure=3 accumulation=2 basis="severity 1: nothing breaks, the caller just cannot conclude a goal until it reads the source; novelty 1: the fix is text or generation, and the live flag sets are already there to read; exposure 3: every caller of the goal verbs on every machine reads that table; accumulation 2: each drifted verb costs its next caller the same discovery, and nothing detects the drift"
- Tier: 3
- Intent: The goal verb table and the goal verbs disagree about how a goal concludes, and the mismatch costs a caller several refused attempts before it can record anything. 'metasystem goal' prints 'done  conclude the Current goal; requires --then or --and-none', but in the synced world the done verb takes --id and --conclude and defines neither --then nor --and-none (cmd/metasystem/goalsync_mutations.go case "done"; the --then and --and-none pair lives only in the legacy runGoalDone in cmd/metasystem/goal.go). A caller following the printed table gets 'flag provided but not defined: -and-none' followed by the whole flag dump, twice, before finding --conclude by reading the source. Seen 2026-09-07 on m1d concluding fixture-review-by-date-expired. DONE means the verb table describes the verbs the running world actually offers, and a wrong flag refuses with the accepted form named rather than a flag dump; if the legacy and synced verbs must differ, the table says which world it is describing.
- Origin: main
- Next step: Read the two done implementations against the table text in cmd/metasystem/goal.go, decide whether the table is generated from the live verb set or hand-written, and make the printed contract match what the synced world accepts. The same check is worth running across every verb the table describes, since done is unlikely to be the only one that drifted.
- OpenedAt: 2026-09-07T09:17:29Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-07T09:17:29Z MS5AAXKR20WCANBGFXR2J2X6JC-m1d-25755dc0 open actor=m1d+main-1788764558-63534-a15b0d targets=goal-done-flags-do-not-match-the-verb-table
Integrity: sha256=5af0e08fc1ee00d9e57144c3c38b0c93addaf9afd66d16223e7a77d4fd7dc45c
