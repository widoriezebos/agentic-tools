# stop-hook-refusal-carries-verdict

- State: approved
- Risk: severity=2 novelty=2 exposure=3 accumulation=2 basis="The stop hook runs at every turn end on every machine; its plan-work refusal drops the one-line health verdict and the open-work display the design promises, and its deadline path replaces a finished answer with a timeout sentence; each wrong refusal costs the human a re-prompt."
- Tier: 3
- Intent: scripts/agents/supervision-hook.sh: since the stop-hook fix (6e0221e0) the 'Work named in a plan is unblocked' refusal returns a block whose reason carries neither the goal-thread verdict display (OPEN WORK (1) naming the open step; design points GOAL-04/05, byte-identity) nor the one-line HEALTH verdict, and the deadline parent appends 'Metasystem Stop deadline expired before a safe turn verdict' even when the worker answered in time; Wido saw exactly this refusal on the m2 console on 2026-09-04. The supervision fixture's stop-hook-monitor scenario pins the lawful shape and is red; three rounds of chain sse-build1 (preserved on preserve/sse-build1-r3: the evidence path moved to the steward component record, a deadline-preservation change, and a reworked block composition that regressed the health line) did not get it green. DONE means every block the hook emits carries the verdict display first, then any rule sentence, with the HEALTH verdict in the system message; the deadline sentence appears only when the deadline actually expired; and the stop-hook-monitor scenario passes on a Mac.
- Origin: main
- Next step: One hook composition fix with the scenario as its test: build from preserve/sse-build1-r3, run supervision-fixtures.sh seat-side, land through a chain. Tier from the risk basis; waits for Wido's word if above tier 1.
- OpenedAt: 2026-09-04T19:55:53Z
- Revision: 5
- Labels: robustness
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-04T22:11:03Z revision=2 opid=P6MQ0P1491R9YWHFADP1GAK3ZD-m2-5fcf08ab authority=relayed digest=3c16b5a2e834a475d11490786b174228a3b5ac6387ac65b7a45229440c2a6271 reviewBy=2026-09-06
- Sliced: machine=m2 lineage=main-1788441779-14484-82d6ed revision=3 at=2026-09-04T23:53:59Z

History:
- 2026-09-04T19:55:53Z PF6WFFRT08AKVHH1Z3DR652HQQ-m2-5fcf08ab open actor=human:Wido targets=stop-hook-refusal-carries-verdict
- 2026-09-04T22:11:03Z P6MQ0P1491R9YWHFADP1GAK3ZD-m2-5fcf08ab approve actor=human:Wido targets=stop-hook-refusal-carries-verdict authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="Yes, all five (Recommended)"
- 2026-09-04T23:53:02Z BRJ30T06FPKCM9F2M1VA9W8SVW-m2-5fcf08ab claim actor=m2+main-1788441779-14484-82d6ed targets=stop-hook-refusal-carries-verdict
- 2026-09-04T23:53:59Z H467GM83KPGWYT82APC2WJ5T5C-m2-5fcf08ab slice-start actor=m2+main-1788441779-14484-82d6ed targets=stop-hook-refusal-carries-verdict
- 2026-09-05T01:36:55Z 6T2VHXBBXCYF3C80R36HMF6N0N-m2-5fcf08ab release actor=m2+main-1788441779-14484-82d6ed targets=stop-hook-refusal-carries-verdict
Integrity: sha256=546a47445b2bff8686a5402402f72cbd0c99f18fd5d05a40bdf28394650db386
