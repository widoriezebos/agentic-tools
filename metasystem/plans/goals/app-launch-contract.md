# app-launch-contract

- State: queued
- Priority: 1
- Sequence: 27
- Risk: severity=2 novelty=2 exposure=1 accumulation=1 basis="Process control of a project's application on a developer host: a wrong stop can kill the wrong process (severity 2); the contract file and the candidate worktree run are new shapes, though the ui lifecycle and the bounded supervisor exist (novelty 2); developer hosts only, no users (exposure 1); a fresh area with little accumulated change (accumulation 1)."
- Tier: 2
- Intent: A project declares how its application starts, stops and reports readiness in a launch contract (launch.json beside testing.json), and metasystem app start|stop|restart|status runs it under the engine's supervision with a truthful status and a proven stop; app start --goal G runs a goal's candidate from its branch on its own port beside the standing app, so a human review can inspect its behaviour before it lands. Design: plans/designs/app-launch-contract.md.
- Origin: human
- Next step: Astra critiques plans/designs/app-launch-contract.md; then one Opus build lane for the contract and its settings check, the four verbs on the standing app, --goal G for the candidate, and this repository's own contract (the interface) as the first proof.
- OpenedAt: 2026-09-28T06:10:15Z
- Revision: 2
- Labels: launch
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-28T06:10:15Z GNEEY6K5SCYBWVBZB1E5X4Q446-ui-966d857e open actor=human:Wido targets=app-launch-contract
- 2026-09-28T06:10:21Z P5CCSQTBFBP3CV0NNFTK9RW0A0-ui-966d857e set-priority actor=human:Wido targets=app-launch-contract reason=priority-order subject=app-launch-contract from=unranked to=1:27 requested-sequence=append
Integrity: sha256=b8370d9c959b40397927c2e33b8bcba5aef5d88be30b99adac575f476c49049e
