# ledger-reads-are-fresh

- State: queued
- Risk: severity=2 novelty=2 exposure=3 accumulation=1 basis="Stale reads strand claims and sessions on every seat (exposure 3) but cause delay, not damage (severity 2); designed against accepted pages (novelty 2); one unit (accumulation 1)"
- Tier: 2
- Intent: Readers that decide from the ledger (claim, next, session start and its hook) read it fresh within a bounded budget; a stale or failed read reports pending with a remedy that works
- Origin: main
- Next step: Design critique to convergence, then build by hand with delegates
- OpenedAt: 2026-10-07T00:24:54Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-07T00:24:54Z B1W8K7GM8WNCDNVAJWQDAXRGWS-m1e-718ba0eb open actor=human:Wido targets=ledger-reads-are-fresh
Integrity: sha256=97ca9ea37e19b0f45868700ffcbf773832318855538643b2ecd7a4a9ce18e754
