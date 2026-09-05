# supervision-custody-per-checkout

- State: approved
- Risk: severity=3 novelty=2 exposure=3 accumulation=3 basis="A shutdown issued for one checkout terminated another checkout's supervision on the same machine; every seat on every machine that runs more than one checkout or any fixture is exposed, and each occurrence stales a census and refuses every dispatch until a human notices."
- Tier: 3
- Intent: On 2026-09-05 at 01:22:18Z this checkout's supervision components were recorded in the machine-wide registry (~/.metasystem/armed-checkouts.jsonl) as exited, reason terminated, while the supervision fixture's stop-hook-monitor scenario (chain shv-build1, rounds 2 to 9) armed a temporary root with this session's main-process identity (--pid of the suite's shell) and this user's registry and then shut its root down. The registry keys custody by the canonical checkout path (docs/design/supervision-registry.md, REG-1), so per-checkout custody is the law; the defect is that a shutdown, takeover or sweep issued for one checkout under a borrowed or shared main identity reached another checkout's processes. Wido: one machine must run many checkouts with many supervisors; this is not allowed to be. DONE means (1) a test in internal/supervise or internal/registry that arms two real checkouts and one temporary root on one machine, under the same and under different main identities, and asserts that no shutdown, takeover, relaunch or sweep of one ever terminates or retires another's owner, watcher or runner; (2) the code path the test exposes fixed so every victim selection is by canonical checkout path; (3) the fixture rule written in docs/orchestration.md and enforced by the supervision suite's self-check: a scenario brings up supervision only with its own registry home and main identity.
- Origin: main
- Next step: NOT LANDED (2026-09-05 11:15Z). Three chains, three critic rounds (the goal's box of review rounds is spent): the invariant test and the selection fix exist on preserve/scc-build3-r1 (ten files) but the third critic (scc-build3-cc1, findings SCC-31 to SCC-36, record under records/misc) shows the landing would lock out every already-armed checkout (owners now record the git top-level while existing rows hold the state root; the guard refuses before any liveness check) and make a dead owner without a registry row permanently un-takeoverable; the self-check still accepts the seat's agent pid as a scenario's main. Rebudget needed: one implementer round on preserve/scc-build3-r1 keeping the recorded path form (state root) or reading both, guarding only live owners, keying on rows that survive compaction, and a self-check that rejects any main outside the scenario's bed; then one critic round. Until then the hazard is avoided only by not running the stop-hook scenario with the seat's identity. Wido's instruction: land this and stop; the seat stopped without landing because landing it would break his restart.
- OpenedAt: 2026-09-05T05:50:57Z
- Revision: 6
- Labels: robustness
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-05T05:51:02Z revision=2 opid=8AMGFJK6GJS7GBD6Z3HX1KJ82V-m2-5fcf08ab authority=relayed digest=0baeec9d12a4b053881115b1b50ff1505629b55d1e23c343cf2e4415cd03837a reviewBy=2026-09-06
- Sliced: machine=m2 lineage=main-1788441779-14484-82d6ed revision=3 at=2026-09-05T05:51:33Z

History:
- 2026-09-05T05:50:57Z HPW2Z8QEADG33PE0K7NYY4ZMWP-m2-5fcf08ab open actor=human:Wido targets=supervision-custody-per-checkout
- 2026-09-05T05:51:02Z 8AMGFJK6GJS7GBD6Z3HX1KJ82V-m2-5fcf08ab approve actor=human:Wido targets=supervision-custody-per-checkout authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="This is serious and needs to be addressed immediately."
- 2026-09-05T05:51:06Z GG3AHCHFQ40GA1VK2M05ZAEGSS-m2-5fcf08ab claim actor=m2+main-1788441779-14484-82d6ed targets=supervision-custody-per-checkout
- 2026-09-05T05:51:33Z 2RMXMY18F1SV8WA3604TJ4WNH5-m2-5fcf08ab slice-start actor=m2+main-1788441779-14484-82d6ed targets=supervision-custody-per-checkout
- 2026-09-05T11:13:42Z WDEFCFS2S00VB24RSG4Q760RFY-m2-5fcf08ab edit actor=m2+main-1788441779-14484-82d6ed targets=supervision-custody-per-checkout
- 2026-09-05T11:14:57Z Y9FGEQFHVYB3W1SNGRBXRVK34P-m2-5fcf08ab release actor=m2+main-1788441779-14484-82d6ed targets=supervision-custody-per-checkout
Integrity: sha256=a4ef0c051133320d552b1ec18e3b031a408490593e0cecd8955a9f456788c0cd
