# stop-hook-refusal-carries-verdict

- State: claimed
- Risk: severity=2 novelty=2 exposure=3 accumulation=2 basis="The stop hook runs at every turn end on every machine; its plan-work refusal drops the one-line health verdict and the open-work display the design promises, and its deadline path replaces a finished answer with a timeout sentence; each wrong refusal costs the human a re-prompt."
- Tier: 3
- Intent: scripts/agents/supervision-hook.sh: since the stop-hook fix (6e0221e0) the 'Work named in a plan is unblocked' refusal returns a block whose reason carries neither the goal-thread verdict display (OPEN WORK (1) naming the open step; design points GOAL-04/05, byte-identity) nor the one-line HEALTH verdict, and the deadline parent appends 'Metasystem Stop deadline expired before a safe turn verdict' even when the worker answered in time; Wido saw exactly this refusal on the m2 console on 2026-09-04. The supervision fixture's stop-hook-monitor scenario pins the lawful shape and is red; three rounds of chain sse-build1 (preserved on preserve/sse-build1-r3: the evidence path moved to the steward component record, a deadline-preservation change, and a reworked block composition that regressed the health line) did not get it green. DONE means every block the hook emits carries the verdict display first, then any rule sentence, with the HEALTH verdict in the system message; the deadline sentence appears only when the deadline actually expired; and the stop-hook-monitor scenario passes on a Mac.
- Origin: main
- Next step: Round one of chain shr-build1 is built and verified seat-side under stock bash (hook fixtures green; stop-hook-monitor passes its refusal and byte-identity assertions and stops at the S4-15(b) goal open on a --tier flag the fresh legacy ledger cannot take). The one-line fold brief landed at ff31b073 (plans/stop-hook-refusal-carries-verdict-fold-r2-brief.md). BLOCKED on a human act: dispatching the follow-up refuses because this seat's engine was rebuilt (first by the landing receipt's fast gate, then at ff31b073 because another seat's landing f12c5aa6 changed the engine) and the armed census no longer matches it ('census fingerprint does not match the armed code, signatures, and configuration'); every re-arm path refuses ENROLLMENT_DRIFT from an agent. Wido: at the enrolled terminal, in agentic-tools-m1c/metasystem, run steward restart --repo . ; then the seat dispatches round two, critiques the chain once, lands it with the full-battery receipt, and proves it with the suite seat-side.
- OpenedAt: 2026-09-04T19:55:53Z
- Revision: 11
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
Integrity: sha256=b2dd37bc108ce96a463bba5706fcb4b607e379bae7f65f24b05eea8ba7f62c58
