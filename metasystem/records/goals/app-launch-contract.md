# app-launch-contract

- State: done
- Priority: 1
- Sequence: 27
- Risk: severity=2 novelty=2 exposure=1 accumulation=1 basis="Process control of a project's application on a developer host: a wrong stop can kill the wrong process (severity 2); the contract file and the candidate worktree run are new shapes, though the ui lifecycle and the bounded supervisor exist (novelty 2); developer hosts only, no users (exposure 1); a fresh area with little accumulated change (accumulation 1)."
- Tier: 2
- Intent: A project declares how its application starts, stops and reports readiness in a launch contract (launch.json beside testing.json), and metasystem app start|stop|restart|status runs it under the engine's supervision with a truthful status and a proven stop; app start --goal G runs a goal's candidate from its branch on its own port beside the standing app, so a human review can inspect its behaviour before it lands. Design: plans/designs/app-launch-contract.md.
- Origin: human
- Next step: Astra critiques plans/designs/app-launch-contract.md; then one Opus build lane for the contract and its settings check, the four verbs on the standing app, --goal G for the candidate, and this repository's own contract (the interface) as the first proof.
- Concluded: Landed on main 2026-09-28 (5291814d3, corrected by 3b3ca9955): launch.json schema 1 with every field but start optional; metasystem app start|stop|restart|status|log|reset|check with --at REF and --goal G; the app serve supervisor (record before spawn, identity before every signal, an ended record, stop through its own group); runs at any commit with their own worktree, address, state root and data, prepare and reset; log; check through the testing runner's canary form; the evidence copy for goal runs; this repository's own contract proven for real on a second port. Design plans/designs/app-launch-contract.md with its Built section; Astra four rounds, Sol three reads, the last at zero material. Built by Claude on Opus 5.5, read by Codex on Sol. The four follow-ups are scheduled as goal:app-launch-contract-follow-ups.
- OpenedAt: 2026-09-28T06:10:15Z
- Revision: 3
- Labels: launch
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-28T06:10:15Z GNEEY6K5SCYBWVBZB1E5X4Q446-ui-966d857e open actor=human:Wido targets=app-launch-contract
- 2026-09-28T06:10:21Z P5CCSQTBFBP3CV0NNFTK9RW0A0-ui-966d857e set-priority actor=human:Wido targets=app-launch-contract reason=priority-order subject=app-launch-contract from=unranked to=1:27 requested-sequence=append
- 2026-09-28T13:30:06Z 94BFCXCPJ10WGV9HAX1WJ8XWJA-ui-966d857e done actor=human:Wido targets=app-launch-contract,partner-runs-the-design-loop
Integrity: sha256=e3d198b24f8c3e0c8891ae0c3732cf594e3c6fe525826794d42d0c7b92b6325f
