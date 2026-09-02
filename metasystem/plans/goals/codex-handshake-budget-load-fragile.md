# codex-handshake-budget-load-fragile

- State: claimed
- Intent: The Codex adapter's session handshake budget is a hard-coded 10 seconds (scripts/agents/adapters/codex.sh line 71, declared into the capability snapshot as sessionEstablishedTimeoutSec and consumed by the dispatch supervisor), and on 2026-09-02 a direct probe on m1 measured 14 seconds from launch to Codex's thread.started event at load average 6, so every Codex dispatch on m1 failed with handshake_timeout by construction (jobs breach-design-crit2 and breach-design-crit2b, two goal attempts spent, the highest-priority breach-clock chain blocked). The child was alive and healthy the whole time; the cap converted slowness into failure, which R-35-m3 names a defect: patience must be progress-based (the child is alive and has not exited) with the fixed number only as a hang bound. No lawful override exists: the snapshot is immutable evidence and the value has no config key.
- Origin: main
- Next step: CHAIN STATE 2026-09-02 20:20Z (m0b): design plans/codex-handshake-design.md CERTIFIED at revision 7 (Sol rounds 1-4: registers records/misc/codex-handshake-critique-r1.md to -r4.md, 6+1+1+1 findings, all folded; no fifth Sol round for budget reasons, recorded on -r4.md). Sol BUILD RUNNING as job ch-build-1c on plans/codex-handshake-build-brief.md (cap 120, DESIGN-BEARING). Next: validate conformance --stage review --job ch-build-1c; delegate --role code-critic --reviews ch-build-1c --brief <critique brief> (Fable, cap 20); apply the build diff at the repo root (git -C <repo root> apply artifacts/agents/ch-build-1c/rounds/1/diff.patch, or the worktree diff); land --chain ch-build-1c; conclude duplicate goal codex-handshake-budget (m1b) with this one; then re-claim breach-clock-and-budget-honesty (recipe on that goal). Budget 1d/10/480/1 granted by Wido 19:00Z.
- OpenedAt: 2026-09-02T12:14:59Z
- Revision: 13
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
- 2026-09-02T18:09:59Z FRJCRKYXVFP7WR3R09SJKWG72C-m0b-6638932d edit actor=m0b+main-1788250419-3170380-8a1fb3 targets=codex-handshake-budget-load-fragile
Integrity: sha256=876d73e31c9b7f0836d5bb5f915d79b4df0c8d834c8ab5cdb68e465ba0b9c8f1
