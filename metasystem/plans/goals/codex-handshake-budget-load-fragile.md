# codex-handshake-budget-load-fragile

- State: claimed
- Intent: The Codex adapter's session handshake budget is a hard-coded 10 seconds (scripts/agents/adapters/codex.sh line 71, declared into the capability snapshot as sessionEstablishedTimeoutSec and consumed by the dispatch supervisor), and on 2026-09-02 a direct probe on m1 measured 14 seconds from launch to Codex's thread.started event at load average 6, so every Codex dispatch on m1 failed with handshake_timeout by construction (jobs breach-design-crit2 and breach-design-crit2b, two goal attempts spent, the highest-priority breach-clock chain blocked). The child was alive and healthy the whole time; the cap converted slowness into failure, which R-35-m3 names a defect: patience must be progress-based (the child is alive and has not exited) with the fixed number only as a hang bound. No lawful override exists: the snapshot is immutable evidence and the value has no config key.
- Origin: main
- Next step: CHAIN STATE 2026-09-02 20:45Z (m0b): design plans/codex-handshake-design.md CERTIFIED at revision 7 (Sol rounds 1-4, registers records/misc/codex-handshake-critique-r1.md to -r4.md). BUILD ch-build-1c (Sol, 120) DONE for PART 1 only: worktree commit 35b779ef "Disable operator plugins for Codex delegates", three adapter files (codex.go, new codexcommand_test.go, runtime_test.go); it stopped correctly at a Part 2 gap (D2.3's mandatory bound refuses the out-of-boundary BuildFollowRecord call in internal/dispatch/provenance_test.go:51 and the TestMissionProvenanceTuple calls in decisions_test.go). Conformance review of ch-build-1c is BLOCKED on one hand-removal: the pre-commit guard wrote its gitignored landing-observe.log into the delegate worktree (goal conformance-runtime-state-litter class); Wido handed the exact rm. THEN: validate conformance --stage review --job ch-build-1c; Fable code critique (brief plans/codex-handshake-code-critique-brief.md, op ch-crit-code-1, cap 20); apply the worktree diff at the repo root; land --chain ch-build-1c with this Goal-Item; conclude duplicate goal codex-handshake-budget. PART 2 PARKED for budget: needs Fable design revision 8 (brief plans/codex-handshake-revision-r8-brief.md, ~12 min, cap 20) folding the gap, a Sol build (120) and a Fable code critique (20) = 160 reserved minutes; reserved 420 of 480 spent before the critique, 40 left after it. ASKED Wido (via f7 and the session): raise reservedJobMinutesLimit 480 to 720; recommendation yes, Part 2 is the liveness-based patience half of his order. Budget arithmetic: 240 (rounds 1-6) + 40 (ch-crit-4b) + 20 (ch-design-r7) + 120 (ch-build-1c) + 20 (ch-crit-code-1) = 440.
- OpenedAt: 2026-09-02T12:14:59Z
- Revision: 14
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
- 2026-09-02T18:23:11Z FW00S2F22CVTXQ0V1E7FZE6VRQ-m0b-6638932d edit actor=m0b+main-1788250419-3170380-8a1fb3 targets=codex-handshake-budget-load-fragile
Integrity: sha256=1d247893ca3cbf840f0d380dcec713dd1edc7d93c52e5c27018a9b5af4ec2374
