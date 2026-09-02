# codex-handshake-budget-load-fragile

- State: queued
- Intent: The Codex adapter's session handshake budget is a hard-coded 10 seconds (scripts/agents/adapters/codex.sh line 71, declared into the capability snapshot as sessionEstablishedTimeoutSec and consumed by the dispatch supervisor), and on 2026-09-02 a direct probe on m1 measured 14 seconds from launch to Codex's thread.started event at load average 6, so every Codex dispatch on m1 failed with handshake_timeout by construction (jobs breach-design-crit2 and breach-design-crit2b, two goal attempts spent, the highest-priority breach-clock chain blocked). The child was alive and healthy the whole time; the cap converted slowness into failure, which R-35-m3 names a defect: patience must be progress-based (the child is alive and has not exited) with the fixed number only as a hang bound. No lawful override exists: the snapshot is immutable evidence and the value has no config key.
- Origin: main
- Next step: Small item (4h). Ladder per R-38-m2: design (one page: replace the fixed 10-second handshake cap with liveness-based patience - wait while the child process is alive and the launch is younger than a generous hang bound taken from the snapshot, fail only on exit-without-session or the hang bound; keep the recorded field honest; state what m0b's VM measured for comparison) -> Sol critique -> Sol build -> Fable code critique -> land with --chain. MEASURED 2026-09-02 15:20 on m1 after the machine was healed (fseventsd restarted, 488 leaked fixture processes and 8789 stale beds removed, load under 5): codex-cli 0.148.0 still takes 16 to 18 seconds from launch to thread.started, with or without MCP servers, so the cap fails on a healthy m1 too. Suspected cause: the ten plugins configured in the operator's ~/.codex/config.toml, which the VM seats (where dispatches succeed on the same codex-cli 0.148.0) do not have; the design must say whether the adapter disables plugins for delegate runs or the cap simply tolerates a slow start.
- OpenedAt: 2026-09-02T12:14:59Z
- Revision: 4
- Labels: robustness
- Budget: elapsedLimit=4h attemptLimit=10 reservedJobMinutesLimit=240 activeJobLimit=1

History:
- 2026-09-02T12:14:59Z N4P2PM3GVJYH8GZAWQ9DEXXGV7-m1-7bb1546e open actor=m1+main-1788333680-2840-7f79f4 targets=codex-handshake-budget-load-fragile
- 2026-09-02T12:15:13Z JWXKVACXE2XHF69TZVBR2Y616P-m1-7bb1546e set-budget actor=m1+main-1788333680-2840-7f79f4 targets=codex-handshake-budget-load-fragile
- 2026-09-02T13:21:16Z XM6TG83VEP1NCDFEG1WX1EKQR7-m1-7bb1546e edit actor=m1+main-1788333680-2840-7f79f4 targets=codex-handshake-budget-load-fragile
- 2026-09-02T13:22:10Z XX7F2XMKNVB6SRDSDYF9NP85A9-m1-7bb1546e edit actor=m1+main-1788333680-2840-7f79f4 targets=codex-handshake-budget-load-fragile
Integrity: sha256=94c8c31bba4ec14605a60db19e95c2ffcc5b7e453da1e4d0ed055b1e7aee1deb
