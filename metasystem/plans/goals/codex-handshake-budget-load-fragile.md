# codex-handshake-budget-load-fragile

- State: claimed
- Intent: The Codex adapter's session handshake budget is a hard-coded 10 seconds (scripts/agents/adapters/codex.sh line 71, declared into the capability snapshot as sessionEstablishedTimeoutSec and consumed by the dispatch supervisor), and on 2026-09-02 a direct probe on m1 measured 14 seconds from launch to Codex's thread.started event at load average 6, so every Codex dispatch on m1 failed with handshake_timeout by construction (jobs breach-design-crit2 and breach-design-crit2b, two goal attempts spent, the highest-priority breach-clock chain blocked). The child was alive and healthy the whole time; the cap converted slowness into failure, which R-35-m3 names a defect: patience must be progress-based (the child is alive and has not exited) with the fixed number only as a hang bound. No lawful override exists: the snapshot is immutable evidence and the value has no config key.
- Origin: main
- Next step: CHAIN STATE 2026-09-02 17:10Z (m0b): design plans/codex-handshake-design.md at revision 4 (rounds ch-design-r1b, r2, r3, r4); Sol registers records/misc/codex-handshake-critique-r1.md (6 material, all folded) and codex-handshake-critique-r2.md (1 material, folded across revisions 3 and 4 with the orchestrator's sessionless-claim decision, candidate a without a fingerprint version bump). Sol round 3 running as job ch-crit-3 on plans/codex-handshake-critique-r3-brief.md. Build brief plans/codex-handshake-build-brief.md is landed: on certification dispatch --role implementer (Sol, cap 120) DESIGN-BEARING, then Fable code critique --reviews, apply the diff with git apply --directory=metasystem, land --chain. Goal codex-handshake-budget (m1b) is marked duplicate and concludes with this one.
- OpenedAt: 2026-09-02T12:14:59Z
- Revision: 8
- Labels: robustness
- Budget: elapsedLimit=4h attemptLimit=10 reservedJobMinutesLimit=240 activeJobLimit=1
- Sliced: machine=m0b lineage=main-1788250419-3170380-8a1fb3 revision=6 at=2026-09-02T16:07:14Z
- Claimed: machine=m0b lineage=main-1788250419-3170380-8a1fb3 at=2026-09-02T16:03:41Z revision=6
- StopCapability: generation=6 revision=6 machine=m0b claimEpoch=1 fenceEpoch=0

History:
- 2026-09-02T12:14:59Z N4P2PM3GVJYH8GZAWQ9DEXXGV7-m1-7bb1546e open actor=m1+main-1788333680-2840-7f79f4 targets=codex-handshake-budget-load-fragile
- 2026-09-02T12:15:13Z JWXKVACXE2XHF69TZVBR2Y616P-m1-7bb1546e set-budget actor=m1+main-1788333680-2840-7f79f4 targets=codex-handshake-budget-load-fragile
- 2026-09-02T13:21:16Z XM6TG83VEP1NCDFEG1WX1EKQR7-m1-7bb1546e edit actor=m1+main-1788333680-2840-7f79f4 targets=codex-handshake-budget-load-fragile
- 2026-09-02T13:22:10Z XX7F2XMKNVB6SRDSDYF9NP85A9-m1-7bb1546e edit actor=m1+main-1788333680-2840-7f79f4 targets=codex-handshake-budget-load-fragile
- 2026-09-02T13:23:30Z NVVHBA80G3HVCM9VVJ8DS8G2D0-m1-7bb1546e edit actor=m1+main-1788333680-2840-7f79f4 targets=codex-handshake-budget-load-fragile
- 2026-09-02T16:03:41Z H33H1MBM5P7TR4ZBP1QVJD2P15-m0b-6638932d claim actor=m0b+main-1788250419-3170380-8a1fb3 targets=codex-handshake-budget-load-fragile
- 2026-09-02T16:07:14Z G0AETZVE7GQ0XV8PY6GPP7HFEP-m0b-6638932d slice-start actor=m0b+main-1788250419-3170380-8a1fb3 targets=codex-handshake-budget-load-fragile
- 2026-09-02T17:06:40Z ZW84JKJRR712KEQ8TQ9S0DYVZ4-m0b-6638932d edit actor=m0b+main-1788250419-3170380-8a1fb3 targets=codex-handshake-budget-load-fragile
Integrity: sha256=0337627e66c288672b908f501e0e958f4abb042793294fee28e78d8d91840384
