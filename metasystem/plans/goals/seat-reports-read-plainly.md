# seat-reports-read-plainly

- State: queued
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="Output wording and layout only; no state change"
- Tier: 1
- Intent: Three reports a person reads are hard to act on (ui seat, 2026-10-01, Wido called the formatting bad): the Stop hook line packs five facts into one sentence and prints goal ids with hyphens stripped and a 3-character session fragment; session status lists about 40 lines of other goals' plan steps under Seat actions; helm return prints 218 acts as one comma-separated line. Change: goal ids verbatim, one fact per line, only this seat's own actions with others behind a count, helm acts grouped and counted by verb with the full list under --verbose. Pro: the one action that matters is visible. Con: touches three renderers and their goldens.
- Origin: human
- Next step: After the simple lane lands: one builder for the three renderers, goldens from the ui seat's examples
- OpenedAt: 2026-10-01T06:56:48Z
- Revision: 1
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-10-01T06:56:48Z CSR2W61CCQB8AZRM9H09H3TSNP-m1e-b6a4eb0a open actor=human:wido targets=seat-reports-read-plainly
Integrity: sha256=8d70bec7b2e67cd51b8d5a82f02871b5c3170e8ab3e5cbcae5fbe20fd6cc120b
