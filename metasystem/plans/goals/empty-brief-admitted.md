# empty-brief-admitted

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="severity 2: a whole round cap and an attempt burn on nothing and the chain cannot be cancelled without stranding it; novelty 1: one guard in an existing admission; exposure 2: every dispatch and follow-up; accumulation 1: rare, but each occurrence costs a round"
- Tier: 2
- Intent: The dispatcher admits a brief with no task direction. On 2026-09-06 a zero-byte brief file (a failed sed wrote nothing) passed follow-up admission for chain followup-rebase-build1 round 2, because brief authority admits a brief with no mechanically extractable paths, and the round launched with an empty Task Direction section. The round's cap and one attempt were spent, and it could not be cancelled without stranding the chain (a cancelled newest round refuses every later follow-up). DONE means dispatch and follow-up refuse a brief whose task direction is empty or blank with a typed message naming the brief, and a dispatch fixture pins it.
- Origin: main
- Next step: Refuse in the Go brief admission when the brief has no non-blank line outside headings; typed message; scenario in scripts/agents/dispatch-fixtures.sh for dispatch and follow-up
- OpenedAt: 2026-09-06T17:47:30Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-06T17:47:30Z A8D6BPQBDFBH5TZA1VK9GSNE5J-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=empty-brief-admitted
Integrity: sha256=1ce9e80f4aea4de9d389f24e810ce8146f322aeefd91154ed480b5799634bdee
