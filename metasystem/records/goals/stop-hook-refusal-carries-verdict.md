# stop-hook-refusal-carries-verdict

- State: done
- Risk: severity=2 novelty=2 exposure=3 accumulation=2 basis="The stop hook runs at every turn end on every machine; its plan-work refusal drops the one-line health verdict and the open-work display the design promises, and its deadline path replaces a finished answer with a timeout sentence; each wrong refusal costs the human a re-prompt."
- Tier: 3
- Intent: scripts/agents/supervision-hook.sh: since the stop-hook fix (6e0221e0) the 'Work named in a plan is unblocked' refusal returns a block whose reason carries neither the goal-thread verdict display (OPEN WORK (1) naming the open step; design points GOAL-04/05, byte-identity) nor the one-line HEALTH verdict, and the deadline parent appends 'Metasystem Stop deadline expired before a safe turn verdict' even when the worker answered in time; Wido saw exactly this refusal on the m2 console on 2026-09-04. The supervision fixture's stop-hook-monitor scenario pins the lawful shape and is red; three rounds of chain sse-build1 (preserved on preserve/sse-build1-r3: the evidence path moved to the steward component record, a deadline-preservation change, and a reworked block composition that regressed the health line) did not get it green. DONE means every block the hook emits carries the verdict display first, then any rule sentence, with the HEALTH verdict in the system message; the deadline sentence appears only when the deadline actually expired; and the stop-hook-monitor scenario passes on a Mac.
- Origin: main
- Next step: PROVEN on landed main c1525b90 (m1, 2026-09-06 12:22Z, stock bash 3.2): the hook fixtures pass and the supervision suite passes every scenario, S4-1 through S4-16 and the engine re-arm scenarios; the seat's own Stop hook now emits the display-first block (OPEN WORK first, the rule sentence after, HEALTH in the system message) and its deadline block only when the four-second budget actually expired under load. Landed at c1525b90 with dispositions at 19118c9f; the landing's would-refuse chain-output-mismatch provenance (main moved the same files under the chain; the merge was proven, not re-critiqued) is recorded here and carried by goal chain-landing-after-base-move-recertifies. Released by m1c 12:23Z; done is Wido's act.
- Concluded: Landed c1525b90 (chain shr-build1, five rounds, three Fable critiques, the last with zero material findings; dispositions 19118c9f): every block the hook emits leads with the verdict display and carries HEALTH in its system message, the deadline sentence appears only on a real expiry, the verdict carries an explicit fail-closed marker the hook renders as its fixed degraded message, and the supervision fixture runs on stock bash 3.2. Proven on m1: hook fixtures and all supervision scenarios green on landed main; the landing's would-refuse chain-output-mismatch provenance (main moved the same files under the chain; the merge was proven, not re-critiqued) is carried by goal chain-landing-after-base-move-recertifies.
- OpenedAt: 2026-09-04T19:55:53Z
- Revision: 16
- Labels: robustness
- Pinned: m1c
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T06:53:37Z revision=6 opid=A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f authority=proven digest=3c16b5a2e834a475d11490786b174228a3b5ac6387ac65b7a45229440c2a6271
- Sliced: machine=m2 lineage=main-1788441779-14484-82d6ed revision=3 at=2026-09-04T23:53:59Z

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
- 2026-09-06T12:22:25Z 10RKAVTN4GRYE3RMCC986KY2S7-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=stop-hook-refusal-carries-verdict
- 2026-09-06T12:22:28Z SWVFYZ0J7EMVC6N49KMDZ6E3Z1-m1c-7cd0bd60 release actor=m1c+main-1788680061-17829-64951c targets=stop-hook-refusal-carries-verdict
- 2026-09-06T12:55:27Z 1C2JR4BB41SR6M7NFBP10KVT1K-m1-7cd0bd60 done actor=human:Wido targets=stop-hook-refusal-carries-verdict
Integrity: sha256=ba80f1ccf9af07585cc55c7edd8bd8bace5ac6d1be2b55647f016f0e410049ac
