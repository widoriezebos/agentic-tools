# transport-sync-law

- State: queued
- Intent: TRANSPORT-ONLY TOPOLOGY (recorded by m0 at enrollment, 2026-08-31): m0's Debian guest cannot see the host checkout; only /Users/wido/LocalStorage/transport/agentic-tools.git is mounted, and both of m0's remotes point there, while m1 and m2 work through origin on the host. m0's landings reach the others only when they pull transport, and theirs reach m0 only when they push it. SYNC LAW to encode: every machine pulls transport before landing and pushes after; the ledger refs refs/metasystem/goals/* travel too (already published on transport; m0's fetch config carries them). Until machinery enforces it, the law is conduct on all three machines.
- Origin: main
- Next step: Design the mechanical enforcement seam: where the pull-before-land and push-after-land checks live (land.sh gate vs steward tick vs both), and whether staleness of transport refs should refuse a landing
- OpenedAt: 2026-08-31T12:21:06Z
- Revision: 1

History:
- 2026-08-31T12:21:06Z M551NFM6SVVR8AQVH9DFJNFW7Q-m0-c5dbf036 open actor=m0+main-1788178136-1684505-4ffe42 targets=transport-sync-law
Integrity: sha256=d2345ff5407daf82d22f37e06c21f60b446d42b69f510923110b1b5eef0b7c41
