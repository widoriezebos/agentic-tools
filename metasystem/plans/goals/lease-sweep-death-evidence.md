# lease-sweep-death-evidence

- State: queued
- Intent: The lease claim sweep stamps a stale job's endedAt and terminal status the moment SIGTERM delivery succeeds (internal/lease/sweep.go:198-204 returns on nil or ESRCH; concludeStaleJob :146-170 stamps at once) without waiting for the process group to die, unlike dispatch.sh's wind-down (:339-368: bounded wait, SIGKILL, group-absence check). A record can be terminal while its runtime still runs, so anything that reads endedAt as the end of the job's life - the reservation settlement, custody, the census - is wrong for that window. DONE means the sweep stamps only after death evidence and leaves the record non-terminal when the group will not die
- Origin: main
- Next step: Appetite: 4h, full ladder. Split out of dispatch-cap-necessity on 2026-09-02 by m1b (R-4: residue demands a token). Mechanism already designed and critiqued: plans/dispatch-cap-settlement-design.md revision 4 section 1.9 - after a successful SIGTERM poll group absence every 50ms for 2s, re-prove ownership and SIGKILL, poll 2s, final absence check via the group-absence function exported in place from internal/supervise/arming.go (lease may import supervise: measured with go list, lease -> steward -> supervise); when the group survives, concludeStaleJob writes nothing, the sweep returns the named error, the takeover refuses its sweep stamp and the next claim, succession or up retries; the dispatch reap path stamps timeout under its own ladder once the cap expires. Test: no endedAt stamp while the group lives. A builder starts from that section.
- OpenedAt: 2026-09-02T18:37:50Z
- Revision: 1

History:
- 2026-09-02T18:37:50Z DDVQT0QE5B3Z2VM0DBN9YSGE4X-m1b-fad3674e open actor=m1b+main-1788333346-60696-6a3256 targets=lease-sweep-death-evidence
Integrity: sha256=19f49b3e8f3d44b7e822a2d37573c19bfc9747d606c82bbcc76dddfefe025cc7
