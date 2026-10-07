# ledger-reads-are-fresh

- State: done
- Risk: severity=2 novelty=2 exposure=3 accumulation=1 basis="Stale reads strand claims and sessions on every seat (exposure 3) but cause delay, not damage (severity 2); designed against accepted pages (novelty 2); one unit (accumulation 1)"
- Tier: 2
- Intent: Readers that decide from the ledger (claim, next, session start and its hook) read it fresh within a bounded budget; a stale or failed read reports pending with a remedy that works
- Origin: main
- Next step: Design critique to convergence, then build by hand with delegates
- Concluded: Integrated whole on main at c70a8fce2 (main merged into the goal branch, integration fixes) after the full gate on that exact tree: cmd, static, vet green; internal green apart from two load-intermittent tests in untouched packages that pass alone. Delivered by hand with delegates.
- OpenedAt: 2026-10-07T00:24:54Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-07T00:25:01Z revision=2 opid=2P6WBQD6AK88Z58N70WBQMB12H-m1e-718ba0eb authority=proven digest=1896ac5c97e70376fa63525eee52b31de445915b3e429c232692e5d9a8202abf episode=2

History:
- 2026-10-07T00:24:54Z B1W8K7GM8WNCDNVAJWQDAXRGWS-m1e-718ba0eb open actor=human:Wido targets=ledger-reads-are-fresh
- 2026-10-07T00:25:01Z 2P6WBQD6AK88Z58N70WBQMB12H-m1e-718ba0eb approve actor=human:Wido targets=ledger-reads-are-fresh
- 2026-10-07T08:17:23Z SR7SE9V0Q6FCQ6M761SYBYM2Y6-m1e-718ba0eb done actor=human:Wido targets=ledger-reads-are-fresh
Integrity: sha256=c856916769f5c717dd7f0888874c1eeb9ee643b37a45a7a300ecfa221f895c47
