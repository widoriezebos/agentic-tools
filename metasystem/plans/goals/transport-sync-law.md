# transport-sync-law

- State: queued
- Intent: TRANSPORT-ONLY TOPOLOGY (recorded by m0 at enrollment, 2026-08-31): m0's Debian guest cannot see the host checkout; only /Users/wido/LocalStorage/transport/agentic-tools.git is mounted, and both of m0's remotes point there, while m1 and m2 work through origin on the host. m0's landings reach the others only when they pull transport, and theirs reach m0 only when they push it. SYNC LAW to encode: every machine pulls transport before landing and pushes after; the ledger refs refs/metasystem/goals/* travel too (already published on transport; m0's fetch config carries them). Until machinery enforces it, the law is conduct on all three machines.
- Origin: main
- Next step: M0 IS LIVE (2026-08-31, announced here for m1/m2): third machine, Debian VM guest, enrolled as m0, metasystem up and armed as writer, steward runner live under Wido's temporary word (review 2026-09-06). m0 sees only the transport bare repo — both its remotes point at /Users/wido/LocalStorage/transport/agentic-tools.git; it cannot see the host checkout. m0 holds: supervise-start-gate-linux-red (claimed, the Linux-only red it alone reproduces natively); next intended claims in order: steward-tick-load-flake, kill-guard-fold-consumers, role-lane-packets. m0 will not touch counselor, goal-scope-bounds, or the steward seam while m2 is on them. Sync law design remains this goal's next step after the announcement is seen.
- OpenedAt: 2026-08-31T12:21:06Z
- Revision: 2

History:
- 2026-08-31T12:21:06Z M551NFM6SVVR8AQVH9DFJNFW7Q-m0-c5dbf036 open actor=m0+main-1788178136-1684505-4ffe42 targets=transport-sync-law
- 2026-08-31T12:22:11Z PH6F0EZXJMZ6Q2H2EG1YMWPKTP-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=transport-sync-law
Integrity: sha256=41e55fb48025ee4c29a4dcb56ad2f48df752a56275971fb0bbbbd5f36b556ed8
