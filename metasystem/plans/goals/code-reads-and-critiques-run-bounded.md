# code-reads-and-critiques-run-bounded

- State: approved
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: about 20M tokens per read, two or three reads per unit; novelty 1: brief discipline and a prepared copy; exposure 3: every unit has at least one read; accumulation 2: grows with every unit"
- Tier: 2
- Intent: Opus closing reads and Codex critiques each cost 16 to 24M tokens over 106 to 165 tool calls in seat m1e's session (2026-09-12 to 09-15), with contexts up to 213K, because each reader rebuilds its private copy, re-reads the design page and the code from scratch and probes broadly. DONE: the seat prepares the private copy and a focused checklist before the read, the read brief carries a tool-call budget and names the files and rows to check, the reader returns a file path plus a verdict line (never the read itself), and a confirmation read after a fold is scoped to the folded findings only; the receipt records tokens and calls per read; proven by the next five reads each under a stated token ceiling with no material defect missed that a later landing found. Under seats-spend-tokens-in-bounded-sessions.
- Origin: human
- Next step: Build, tier 1: a read-brief template with the prepared copy, checklist, call budget and return shape; the seat instructions; the receipt fields. Suggested owner: any seat.
- OpenedAt: 2026-09-15T09:38:02Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-15T09:38:08Z revision=2 opid=M4HP75PPFCD05W4DKAGVT0ZWME-m1e-c6925449 authority=proven digest=0e342561c0d2c863d19cf8209a4b84c99ed40bc6a7c90858e614615e0e9e6324

History:
- 2026-09-15T09:38:02Z 33TJZQNKS2QX13YDZKER9P81F0-m1e-c6925449 open actor=human:Wido targets=code-reads-and-critiques-run-bounded
- 2026-09-15T09:38:08Z M4HP75PPFCD05W4DKAGVT0ZWME-m1e-c6925449 approve actor=human:Wido targets=code-reads-and-critiques-run-bounded
Integrity: sha256=37572683f3e388eb90987b90a6fe4bc04873dc099db30cf884d0601f9d5f1275
