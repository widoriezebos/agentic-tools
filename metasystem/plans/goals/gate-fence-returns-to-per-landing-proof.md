# gate-fence-returns-to-per-landing-proof

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: without the section at landing, custody-fence regressions reach main until the cadence run; novelty 1: the proof engine build already exists (go-build.sh --out); exposure 3: every engine landing on every seat; accumulation 1: each landing is judged on its own"
- Tier: 3
- Intent: section/gate-fence-fixtures proves only at cadence since 2026-09-10 (chain gate-fence-cadence1-20260910): the landing receipt runs its sections with the enrolled engine copied into the candidate worktree, so the section's dispatch skew preflight refuses every engine-changing candidate. DONE means the receipt runs its sections with a proof engine built from the candidate tree (the enrolled engine stays the policy engine), the gate-fence group's runtime-custody obligation is restored in metasystem/testing.json, and an engine-changing candidate passes the section inside the receipt.
- Origin: main
- Next step: m1c owns the receipt (internal/proofrun, cmd/metasystem/landing_verbs.go): build the candidate's proof engine into the isolated worktree's bin and stamp it so the skew check sees engine and checkout agree; then one contract chain restores the obligation. Found by m1 landing hp-terminal-grade-for-stopping-acts and dispatch-cap-necessity; the temporary demotion was approved by Wido on 2026-09-10. Wido approves at the terminal.
- OpenedAt: 2026-09-10T06:16:49Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-10T06:16:49Z TDPBRYSAH4R9ZR49ZD6HZR6DG1-m1-1701c13c open actor=m1+main-1788940932-18533-7fa6c2 targets=gate-fence-returns-to-per-landing-proof
Integrity: sha256=0480118d4a752d7c46ab581cdb97c6731c5729793e1da0528045631150d01187
