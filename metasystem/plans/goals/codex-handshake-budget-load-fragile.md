# codex-handshake-budget-load-fragile

- State: claimed
- Intent: The Codex adapter's session handshake budget is a hard-coded 10 seconds (scripts/agents/adapters/codex.sh line 71, declared into the capability snapshot as sessionEstablishedTimeoutSec and consumed by the dispatch supervisor), and on 2026-09-02 a direct probe on m1 measured 14 seconds from launch to Codex's thread.started event at load average 6, so every Codex dispatch on m1 failed with handshake_timeout by construction (jobs breach-design-crit2 and breach-design-crit2b, two goal attempts spent, the highest-priority breach-clock chain blocked). The child was alive and healthy the whole time; the cap converted slowness into failure, which R-35-m3 names a defect: patience must be progress-based (the child is alive and has not exited) with the fixed number only as a hang bound. No lawful override exists: the snapshot is immutable evidence and the value has no config key.
- Origin: main
- Next step: MEASURED 2026-09-02 15:20 on m1 after the machine was healed (fseventsd restarted, 488 leaked fixture processes and 996 stale beds removed, load under 5): codex-cli 0.148.0 still takes 18 seconds from launch to thread.started, so the 10-second cap fails on a healthy m1 too; the cause is Codex's own cold start on this host, not load. The VM seats, where Codex dispatches succeed, are the comparison point for the design. CAUSE FOUND 15:30: with the operator's ten Codex plugins disabled for the run (codex exec -c 'plugins={}') thread.started arrives at 1 second on the same machine; with them, 16 to 18 seconds; disabling MCP servers changes nothing. So the design has two parts: the adapter passes plugins={} for every delegate run (delegates never need the operator's interactive plugins, and the run becomes deterministic across machines), and the handshake cap becomes liveness-based patience so the next slow-start cause does not refuse healthy work either.
- OpenedAt: 2026-09-02T12:14:59Z
- Revision: 7
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
Integrity: sha256=b4da8687b046afac638f67718f628403112cabf98e78c91e2ec1bfeebef9af47
