# codex-handshake-budget-load-fragile

- State: claimed
- Intent: The Codex adapter's session handshake budget is a hard-coded 10 seconds (scripts/agents/adapters/codex.sh line 71, declared into the capability snapshot as sessionEstablishedTimeoutSec and consumed by the dispatch supervisor), and on 2026-09-02 a direct probe on m1 measured 14 seconds from launch to Codex's thread.started event at load average 6, so every Codex dispatch on m1 failed with handshake_timeout by construction (jobs breach-design-crit2 and breach-design-crit2b, two goal attempts spent, the highest-priority breach-clock chain blocked). The child was alive and healthy the whole time; the cap converted slowness into failure, which R-35-m3 names a defect: patience must be progress-based (the child is alive and has not exited) with the fixed number only as a hang bound. No lawful override exists: the snapshot is immutable evidence and the value has no config key.
- Origin: main
- Next step: CHAIN STATE 2026-09-02 17:35Z (m0b): design plans/codex-handshake-design.md at revision 6 (rounds ch-design-r1b, r2, r3, r4, r5, r6), no open points. Sol registers records/misc/codex-handshake-critique-r1.md (6 material), -r2.md (1), -r3.md (1), all folded. BLOCKED ON BUDGET: Sol round 4 (job ch-crit-4, brief plans/codex-handshake-critique-r4-brief.md, landed) was refused BUDGET_REFUSED reservedJobMinutesLimit used=240 limit=240 (reservations count caps, nine rounds x cap). Wido asked 17:30Z via f7 relay for set-budget --elapsed-limit 1d --reserved-job-minutes-limit 480 (R-13: human tuple). RESUME when the budget is raised: (1) delegate --role design-critic --brief plans/codex-handshake-critique-r4-brief.md --destructive-reach MECHANICAL (cap 40); (2) on zero material: delegate --role implementer --brief plans/codex-handshake-build-brief.md --destructive-reach DESIGN-BEARING (Sol, cap 120); (3) validate conformance --stage review --job <build>; delegate --role code-critic --reviews <build> (Fable, cap 20); (4) git apply --directory=metasystem the build diff, land with --chain <build>; (5) conclude duplicate goal codex-handshake-budget (m1b) with this one. Released cold-resumable by m0b, which moved to breach-clock-and-budget-honesty per Wido's order sequence.
- OpenedAt: 2026-09-02T12:14:59Z
- Revision: 11
- Labels: robustness
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=480 activeJobLimit=1
- Sliced: machine=m0b lineage=main-1788250419-3170380-8a1fb3 revision=6 at=2026-09-02T16:07:14Z
- Claimed: machine=m0b lineage=main-1788250419-3170380-8a1fb3 at=2026-09-02T17:59:57Z revision=11
- StopCapability: generation=11 revision=11 machine=m0b claimEpoch=1 fenceEpoch=0

History:
- 2026-09-02T12:14:59Z N4P2PM3GVJYH8GZAWQ9DEXXGV7-m1-7bb1546e open actor=m1+main-1788333680-2840-7f79f4 targets=codex-handshake-budget-load-fragile
- 2026-09-02T12:15:13Z JWXKVACXE2XHF69TZVBR2Y616P-m1-7bb1546e set-budget actor=m1+main-1788333680-2840-7f79f4 targets=codex-handshake-budget-load-fragile
- 2026-09-02T13:21:16Z XM6TG83VEP1NCDFEG1WX1EKQR7-m1-7bb1546e edit actor=m1+main-1788333680-2840-7f79f4 targets=codex-handshake-budget-load-fragile
- 2026-09-02T13:22:10Z XX7F2XMKNVB6SRDSDYF9NP85A9-m1-7bb1546e edit actor=m1+main-1788333680-2840-7f79f4 targets=codex-handshake-budget-load-fragile
- 2026-09-02T13:23:30Z NVVHBA80G3HVCM9VVJ8DS8G2D0-m1-7bb1546e edit actor=m1+main-1788333680-2840-7f79f4 targets=codex-handshake-budget-load-fragile
- 2026-09-02T16:03:41Z H33H1MBM5P7TR4ZBP1QVJD2P15-m0b-6638932d claim actor=m0b+main-1788250419-3170380-8a1fb3 targets=codex-handshake-budget-load-fragile
- 2026-09-02T16:07:14Z G0AETZVE7GQ0XV8PY6GPP7HFEP-m0b-6638932d slice-start actor=m0b+main-1788250419-3170380-8a1fb3 targets=codex-handshake-budget-load-fragile
- 2026-09-02T17:06:40Z ZW84JKJRR712KEQ8TQ9S0DYVZ4-m0b-6638932d edit actor=m0b+main-1788250419-3170380-8a1fb3 targets=codex-handshake-budget-load-fragile
- 2026-09-02T17:29:12Z 8NKKJXFWY3RRBVY6QPZSD200NB-m0b-6638932d edit actor=m0b+main-1788250419-3170380-8a1fb3 targets=codex-handshake-budget-load-fragile
- 2026-09-02T17:29:25Z V1M9HTCCZV2FX09DXZ5JVFJEJB-m0b-6638932d release actor=m0b+main-1788250419-3170380-8a1fb3 targets=codex-handshake-budget-load-fragile
- 2026-09-02T17:59:57Z SKX9EEN3CMDPRXV23N6N9N6KYE-m0b-6638932d claim actor=m0b+main-1788250419-3170380-8a1fb3 targets=codex-handshake-budget-load-fragile
Integrity: sha256=344055393437b411694eb33ba52e7eb8389678c7ed22844d5c1d832fab15b2c6
