# processes-read-structured-results

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="Changes the protocol between the landing owner and its proof/test children; a wrong read fails closed to unknown; hard cutover in one landing per unit; reverts cleanly"
- Tier: 2
- Intent: No MetaSystem process reads another MetaSystem process's human text: every internal verb a parent calls takes --json and prints exactly one result with a code field; parents read fields, never message text; a static audit refuses new text parsing
- Origin: main
- Next step: Build U1 (landing owner reads codes, frees the nine frozen protocol messages, one classifier, static audit with a baseline) then U2 (remaining sites, baseline to zero) from /Users/wido/LocalStorage/agentic-tools-evidence/structured-output-20260930/structured-output.md; U1 lands by hand after its gates (Wido D1). Hard cutover, no old-text fallback; smallest thing that works
- OpenedAt: 2026-09-30T12:03:21Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-30T12:03:21Z N132T76CHF2MC4Y09RDRX86BT8-m1e-b6a4eb0a open actor=human:wido targets=processes-read-structured-results
Integrity: sha256=f7d2c0f2c109abeabd5b36b538a56ec0e7cf809381d4e39e656408225f716e68
