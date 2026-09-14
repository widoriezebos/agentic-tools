# beds-report-every-failure

- State: approved
- Priority: 2
- Sequence: 27
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="severity 1: a bed that reports more is safer, not riskier; novelty 1: the supervision bed already runs most legs as independent scenarios; exposure 2: every proof that runs the beds; accumulation 1: nothing builds on the report format"
- Tier: 1
- Intent: Delivery-efficiency phase D, delegate loop. The long sequential beds, the supervision hook bed and the supervision bed's nested-root scenario, run as independent scenarios so that one run reports every failing assertion rather than stopping at the first, and no bed can exit without a message: an assertion whose helper fails inside a command substitution under strict mode still prints the leg it was serving. Why: the hook bed advanced one assertion per build round, and twice it exited with a bare status and no text because a JSON read failed inside a substitution. DONE: against a tree with several known defects one run reports all of them; a fixture injects a failing JSON read inside a substitution and the bed names the leg. every mechanism in this goal lives in the Go engine, the ledger verbs and the adapter contract, never in one runtime; it is proven on at least two runtimes.
- Origin: human
- Next step: Split the two long scripts into scenarios under the existing fixture-bed-child mechanism; add the no-silent-exit guard; then the fixture.
- OpenedAt: 2026-09-14T15:32:10Z
- Revision: 3
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-14T18:53:53Z revision=3 opid=RBTEN3WHYJ3128AK912MKQ38KA-m1e-c6925449 authority=proven digest=413873527168fec4171fc1c19a78ea3040ffd8a3b75dc49cf668c6938f841f0a

History:
- 2026-09-14T15:32:10Z DEFJFFQ7HWC4XRVD6HM7S9CP7M-m1e-c6925449 open actor=human:Wido targets=beds-report-every-failure
- 2026-09-14T15:32:54Z QSTTAVAY41B9HGWG0CMS2Q7P7G-m1e-c6925449 set-priority actor=human:Wido targets=beds-report-every-failure reason=priority-order subject=beds-report-every-failure from=unranked to=2:27 requested-sequence=append
- 2026-09-14T18:53:53Z RBTEN3WHYJ3128AK912MKQ38KA-m1e-c6925449 approve actor=human:Wido targets=beds-report-every-failure
Integrity: sha256=19367001d40cdc893867f05458036368ca86bd4e379f148233a00d00b9ee05ab
