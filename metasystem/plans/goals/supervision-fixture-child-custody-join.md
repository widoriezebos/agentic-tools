# supervision-fixture-child-custody-join

- State: approved
- Priority: 3
- Sequence: 85
- Risk: severity=1 novelty=2 exposure=1 accumulation=2 basis="One census-lifecycle leg about child process custody fails after two fresh passes; it sits on the unfinished process-custody design and only the suite is affected, but it hides the legs behind it."
- Tier: 2
- Intent: scripts/agents/supervision-fixtures.sh, scenario census-lifecycle, now passes its design-critic dispatch and fails at 'S4-2 child custody exact join: still wrong after 2 fresh census passes (scanSeq 11 -> 14)' (m2, 2026-09-05 00:13Z, evidence under the run's suite-failures directory). The leg asserts that the census joins a delegate's child processes to their custodian exactly; two passes is either too little patience or the join is genuinely wrong under the current process-custody law (goal proof-harness-process-custody records that design as unfinished). DONE means the cause is named (patience or product), fixed where it lives without weakening the assertion, and the scenario proceeds past the leg.
- Origin: main
- Next step: Read the leg and the census snapshot it prints; decide patience versus product; tier from the risk basis. Opened 2026-09-05: outside R-76-m2 and R-78-m2; waits for Wido's word.
- OpenedAt: 2026-09-05T00:15:37Z
- Revision: 3
- Labels: robustness
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T06:53:37Z revision=2 opid=A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f authority=proven digest=1fc4527839bfe43b638ee9c20daaa025aff02c80e79dd53fd114d758aaf39dd1

History:
- 2026-09-05T00:15:37Z SFPSJV629BDR8CPNZKMY2SE6JN-m2-5fcf08ab open actor=human:Wido targets=supervision-fixture-child-custody-join
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=supervision-fixture-child-custody-join reason=sweep
- 2026-09-08T16:04:06Z 0S9DPA4Z5DP2WAZTFHHV5NQMF3-m1-7cd0bd60 set-priority actor=human:Wido targets=supervision-fixture-child-custody-join reason=priority-order subject=supervision-fixture-child-custody-join from=unranked to=3:85 requested-sequence=85
Integrity: sha256=5cd88f200250cb2384838ece1476b8d6304c8318852c1271fa1b1eab5e5412fc
