# gate-fence-returns-to-per-landing-proof

- State: claimed
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: without the section at landing, custody-fence regressions reach main until the cadence run; novelty 1: the proof engine build already exists (go-build.sh --out); exposure 3: every engine landing on every seat; accumulation 1: each landing is judged on its own"
- Tier: 3
- Intent: section/gate-fence-fixtures proves only at cadence since 2026-09-10 (chain gate-fence-cadence1-20260910): the landing receipt runs its sections with the enrolled engine copied into the candidate worktree, so the section's dispatch skew preflight refuses every engine-changing candidate. DONE means the receipt runs its sections with a proof engine built from the candidate tree (the enrolled engine stays the policy engine), the gate-fence group's runtime-custody obligation is restored in metasystem/testing.json, and an engine-changing candidate passes the section inside the receipt.
- Origin: main
- Next step: Wido assigned this delivery batch on 2026-09-10, in priority order: build the section engine from the candidate while preserving enrolled policy authority and gate-fence delivery coverage; correct the two reproduced Mac cmd fixtures (installation state root and real-agent ancestry isolation); make dispatch fixture selection honor METASYSTEM_BIN. Deliver these together with focused canaries, independent review and one final risk-selected proof. The separately claimed goal-records selection test may join only if its reviewed source is supplied; do not take over that goal or invent its patch. No stop-hook cleanup, fixture rewrites, test-suite pruning or unrelated backlog work.
- OpenedAt: 2026-09-10T06:16:49Z
- Revision: 5
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-10T06:53:46Z revision=2 opid=AK1FNQGDJRG5JHZC3GN0B6ZRM7-m1c-c6925449 authority=proven digest=66529fbaf8d8337cec11e386280d510f19ba65335478d77856d47ed5a46b77fd
- Sliced: machine=m1c lineage=main-1789023008-75159-b69143 revision=3 at=2026-09-10T06:56:25Z
- Claimed: machine=m1c lineage=main-1789023008-75159-b69143 at=2026-09-10T06:53:58Z revision=3 accountingRevision=3
- StopCapability: generation=3 revision=3 machine=m1c claimEpoch=4 fenceEpoch=0

History:
- 2026-09-10T06:16:49Z TDPBRYSAH4R9ZR49ZD6HZR6DG1-m1-1701c13c open actor=m1+main-1788940932-18533-7fa6c2 targets=gate-fence-returns-to-per-landing-proof
- 2026-09-10T06:53:46Z AK1FNQGDJRG5JHZC3GN0B6ZRM7-m1c-c6925449 approve actor=human:Wido targets=gate-fence-returns-to-per-landing-proof
- 2026-09-10T06:53:58Z P18JQHA8G9PW5X99PSCHK092XG-m1c-66a02980 claim actor=m1c+main-1789023008-75159-b69143 targets=gate-fence-returns-to-per-landing-proof
- 2026-09-10T06:56:25Z F6RGETW70AM6D4ZZM10FVQNN1W-m1c-66a02980 slice-start actor=m1c+main-1789023008-75159-b69143 targets=gate-fence-returns-to-per-landing-proof
- 2026-09-10T07:46:43Z ZJZ7RY5B93QC51ZND8G5NK5H5N-m1c-66a02980 edit actor=m1c+main-1789023008-75159-b69143 targets=gate-fence-returns-to-per-landing-proof
Integrity: sha256=a476afcdd6cd048b1f4de2d955a5cbbbe5b6da611488419c099aa2b091af3bf9
