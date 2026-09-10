# gate-fence-returns-to-per-landing-proof

- State: claimed
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: without the section at landing, custody-fence regressions reach main until the cadence run; novelty 1: the proof engine build already exists (go-build.sh --out); exposure 3: every engine landing on every seat; accumulation 1: each landing is judged on its own"
- Tier: 3
- Intent: section/gate-fence-fixtures proves only at cadence since 2026-09-10 (chain gate-fence-cadence1-20260910): the landing receipt runs its sections with the enrolled engine copied into the candidate worktree, so the section's dispatch skew preflight refuses every engine-changing candidate. DONE means the receipt runs its sections with a proof engine built from the candidate tree (the enrolled engine stays the policy engine), the gate-fence group's runtime-custody obligation is restored in metasystem/testing.json, and an engine-changing candidate passes the section inside the receipt.
- Origin: main
- Next step: m1c owns the receipt (internal/proofrun, cmd/metasystem/landing_verbs.go): build the candidate's proof engine into the isolated worktree's bin and stamp it so the skew check sees engine and checkout agree; then one contract chain restores the obligation. Found by m1 landing hp-terminal-grade-for-stopping-acts and dispatch-cap-necessity; the temporary demotion was approved by Wido on 2026-09-10. Wido approves at the terminal.
- OpenedAt: 2026-09-10T06:16:49Z
- Revision: 3
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-10T06:53:46Z revision=2 opid=AK1FNQGDJRG5JHZC3GN0B6ZRM7-m1c-c6925449 authority=proven digest=66529fbaf8d8337cec11e386280d510f19ba65335478d77856d47ed5a46b77fd
- Claimed: machine=m1c lineage=main-1789023008-75159-b69143 at=2026-09-10T06:53:58Z revision=3 accountingRevision=3
- StopCapability: generation=3 revision=3 machine=m1c claimEpoch=4 fenceEpoch=0

History:
- 2026-09-10T06:16:49Z TDPBRYSAH4R9ZR49ZD6HZR6DG1-m1-1701c13c open actor=m1+main-1788940932-18533-7fa6c2 targets=gate-fence-returns-to-per-landing-proof
- 2026-09-10T06:53:46Z AK1FNQGDJRG5JHZC3GN0B6ZRM7-m1c-c6925449 approve actor=human:Wido targets=gate-fence-returns-to-per-landing-proof
- 2026-09-10T06:53:58Z P18JQHA8G9PW5X99PSCHK092XG-m1c-66a02980 claim actor=m1c+main-1789023008-75159-b69143 targets=gate-fence-returns-to-per-landing-proof
Integrity: sha256=94c4cb9d0a7f8260fca9d4e629516c3dcd57fad9b1f4de265de42c72f9a6fc60
