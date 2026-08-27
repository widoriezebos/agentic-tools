# never-idle-enforcement

- State: queued
- Intent: The never-stop standing order becomes machinery: while the backlog holds claimable work, an idle coordinator stop is a defect the system itself catches — the turn verdict blocks a quiet exit when ready work exists and no tracked step is in flight, unless the human explicitly said stop (Wido's order 2026-08-27: never stop with backlog, EVER; the only exception is his explicit stop-or-redirect; canonical doctrine landed in docs/orchestration.md Working-without-the-human)
- Origin: main
- Next step: Appetite: 2h. The turn-verdict assembler (internal/goal/turnverdict.go decide) already computes the ready frontier and the open-work scan reads in-flight tracked state — join them: ready-work-present + nothing-in-flight + no recorded human stop => shouldBlock with a plain-English line naming the next claimable item; a recorded human stop (an explicit verb or note, design the smallest honest form) lifts it for that session. Fixture the three arms. Coordinate with stop-message-truth (queued) — same seam, complementary lines.
- OpenedAt: 2026-08-27T19:28:19Z
- Revision: 1

History:
- 2026-08-27T19:28:19Z AYAW58WHG5BVD72MDT3MSNWCP4-m1-bf243850 open actor=m1+coordinator targets=never-idle-enforcement
Integrity: sha256=1f3284efc5a85f70ce7447a07cdc4a3439a73f3e580f750727b0825e4f6ffd53
