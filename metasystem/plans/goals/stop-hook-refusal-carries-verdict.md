# stop-hook-refusal-carries-verdict

- State: claimed
- Risk: severity=2 novelty=2 exposure=3 accumulation=2 basis="The stop hook runs at every turn end on every machine; its plan-work refusal drops the one-line health verdict and the open-work display the design promises, and its deadline path replaces a finished answer with a timeout sentence; each wrong refusal costs the human a re-prompt."
- Tier: 3
- Intent: scripts/agents/supervision-hook.sh: since the stop-hook fix (6e0221e0) the 'Work named in a plan is unblocked' refusal returns a block whose reason carries neither the goal-thread verdict display (OPEN WORK (1) naming the open step; design points GOAL-04/05, byte-identity) nor the one-line HEALTH verdict, and the deadline parent appends 'Metasystem Stop deadline expired before a safe turn verdict' even when the worker answered in time; Wido saw exactly this refusal on the m2 console on 2026-09-04. The supervision fixture's stop-hook-monitor scenario pins the lawful shape and is red; three rounds of chain sse-build1 (preserved on preserve/sse-build1-r3: the evidence path moved to the steward component record, a deadline-preservation change, and a reworked block composition that regressed the health line) did not get it green. DONE means every block the hook emits carries the verdict display first, then any rule sentence, with the HEALTH verdict in the system message; the deadline sentence appears only when the deadline actually expired; and the stop-hook-monitor scenario passes on a Mac.
- Origin: main
- Next step: LANDED at c1525b90 (chain shr-build1: five implementer rounds, critiques shr-critic1, shr-critic3 and shr-critic4, the last with zero material findings; dispositions at 19118c9f). The landing's provenance verdict is would-refuse chain-output-mismatch, recorded and admitted: between the chain's base (2f8fbc51) and the landing, main's engine-rearm landing ea8c3ead changed the same hook and fixture files, so the reviewed tree's file entries differ from the landed ones although every hunk applied cleanly; the merged candidate was proven seat-side (hook fixtures, all supervision scenarios under stock bash 3.2, the report and goal packages under the race detector) and by the full-battery receipt, but not re-critiqued. Proof on landed main running now (hook fixtures and the suite). Done is Wido's act on this human-opened goal once that proof is green.
- OpenedAt: 2026-09-04T19:55:53Z
- Revision: 13
- Labels: robustness
- Pinned: m1c
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T06:53:37Z revision=6 opid=A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f authority=proven digest=3c16b5a2e834a475d11490786b174228a3b5ac6387ac65b7a45229440c2a6271
- Sliced: machine=m2 lineage=main-1788441779-14484-82d6ed revision=3 at=2026-09-04T23:53:59Z
- Claimed: machine=m1c lineage=main-1788680061-17829-64951c at=2026-09-06T09:41:52Z revision=8 accountingRevision=8
- StopCapability: generation=8 revision=8 machine=m1c claimEpoch=1 fenceEpoch=0

History:
- 2026-09-04T19:55:53Z PF6WFFRT08AKVHH1Z3DR652HQQ-m2-5fcf08ab open actor=human:Wido targets=stop-hook-refusal-carries-verdict
- 2026-09-04T22:11:03Z P6MQ0P1491R9YWHFADP1GAK3ZD-m2-5fcf08ab approve actor=human:Wido targets=stop-hook-refusal-carries-verdict authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="Yes, all five (Recommended)"
- 2026-09-04T23:53:02Z BRJ30T06FPKCM9F2M1VA9W8SVW-m2-5fcf08ab claim actor=m2+main-1788441779-14484-82d6ed targets=stop-hook-refusal-carries-verdict
- 2026-09-04T23:53:59Z H467GM83KPGWYT82APC2WJ5T5C-m2-5fcf08ab slice-start actor=m2+main-1788441779-14484-82d6ed targets=stop-hook-refusal-carries-verdict
- 2026-09-05T01:36:55Z 6T2VHXBBXCYF3C80R36HMF6N0N-m2-5fcf08ab release actor=m2+main-1788441779-14484-82d6ed targets=stop-hook-refusal-carries-verdict
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=stop-hook-refusal-carries-verdict reason=sweep
- 2026-09-06T07:29:02Z R0085BMQE86RNDBA2MRB7H3JEG-m1-a4f8999f set-pin actor=human:Wido targets=stop-hook-refusal-carries-verdict
- 2026-09-06T09:41:52Z TGN0KZPYPSXCBXZNP4KS0RSMDM-m1c-7cd0bd60 claim actor=m1c+main-1788680061-17829-64951c targets=stop-hook-refusal-carries-verdict
- 2026-09-06T09:56:42Z HYPW3FZVT029ZWN43XXA3VM54Y-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=stop-hook-refusal-carries-verdict
- 2026-09-06T10:00:11Z 5HZW45WA2M468E235YEYB73ZY1-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=stop-hook-refusal-carries-verdict
- 2026-09-06T10:27:25Z E4985JC4R9HYJQVNSEGMFFCJXT-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=stop-hook-refusal-carries-verdict
- 2026-09-06T10:40:16Z SB9DNXXKW0V7BT4KBPAX7NRHSX-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=stop-hook-refusal-carries-verdict
- 2026-09-06T12:20:23Z MT96VEK31B4WYDVRK388386P35-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=stop-hook-refusal-carries-verdict
Integrity: sha256=221818e6431a35e4761f3afb089b4d270549ddddc4f6da041620a7f981cf74a8
