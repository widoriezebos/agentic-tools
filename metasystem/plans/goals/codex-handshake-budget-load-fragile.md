# codex-handshake-budget-load-fragile

- State: claimed
- Intent: The Codex adapter's session handshake budget is a hard-coded 10 seconds (scripts/agents/adapters/codex.sh line 71, declared into the capability snapshot as sessionEstablishedTimeoutSec and consumed by the dispatch supervisor), and on 2026-09-02 a direct probe on m1 measured 14 seconds from launch to Codex's thread.started event at load average 6, so every Codex dispatch on m1 failed with handshake_timeout by construction (jobs breach-design-crit2 and breach-design-crit2b, two goal attempts spent, the highest-priority breach-clock chain blocked). The child was alive and healthy the whole time; the cap converted slowness into failure, which R-35-m3 names a defect: patience must be progress-based (the child is alive and has not exited) with the fixed number only as a hang bound. No lawful override exists: the snapshot is immutable evidence and the value has no config key.
- Origin: main
- Next step: CHAIN STATE 2026-09-02 19:35Z (m0b): budget raised to 1d/10/480/1 at claim on Wido's granted word (relayed via the paper seat 19:00Z). Design plans/codex-handshake-design.md at revision 6, no open points; Sol registers records/misc/codex-handshake-critique-r1.md (6), -r2.md (1), -r3.md (1), all folded. Sol round 4 RUNNING as job ch-crit-4b on plans/codex-handshake-critique-r4-brief.md. Next: land register records/misc/codex-handshake-critique-r4.md; if material, fold (Fable, cap 20) and re-critique; on zero material: delegate --role implementer --brief plans/codex-handshake-build-brief.md --destructive-reach DESIGN-BEARING (Sol, cap 120); validate conformance --stage review --job <build>; delegate --role code-critic --reviews <build> (Fable, cap 20); git apply the build diff at the repo root; land --chain <build>; conclude duplicate goal codex-handshake-budget (m1b) with this one; then re-claim breach-clock-and-budget-honesty (its recipe is on that goal).
- OpenedAt: 2026-09-02T12:14:59Z
- Revision: 12
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
- 2026-09-02T18:01:04Z W152VZAXQ92FGXX22WA2H0ZX88-m0b-6638932d edit actor=m0b+main-1788250419-3170380-8a1fb3 targets=codex-handshake-budget-load-fragile
Integrity: sha256=5cc3f2283285fc3b1dc0c0fda3f7905566cba865b57005eb38e0bbffc0a9e8f7
