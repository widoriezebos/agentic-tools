# digest-landing-race

- State: queued
- Intent: The steward runner appends to records/narrator-digest.log continuously, and land.sh demands a clean tree after staging - so any landing that takes longer than the gap to the next append gets refused with 'unstaged changes remain after staging'. Three refusals in one m2 session (2026-08-31, incl. the counselor-carriage landing dcc44ca9 which needed a manual restage-and-retry); the handoff codified the workaround as law ('live digest appends must be staged into each landing') but it is a race, not a rule - and the carriage just landed, so the tick now also appends counselor briefs to the same file, raising the write rate. A tracked file that machinery writes on its own cadence fundamentally fights a clean-tree landing gate.
- Origin: main
- Next step: Appetite: one 4h slice, any lane. Options for the designer (choose or better): (a) the digest becomes an untracked live file with a landed snapshot/rotation the landing stages deliberately (append-only journal outside the tree, periodic fold-in - mirrors how job artifacts already work); (b) land.sh gains a digest-aware step that re-stages records/narrator-digest.log between its clean-tree check and commit, bounded to that one path so the guard stays honest for everything else; (c) the runner pauses appends while a landing lease is held (the lease exists - the r4 dispatch was once refused on it) and flushes after. Constraint: the no-softening byte-equality law on digest appends (dcc44ca9) must survive whatever shape wins; the digest is the counselor's delivery channel now, so nothing may drop or reorder its bytes. Small enough for straight-to-backlog per R-2.
- OpenedAt: 2026-08-31T06:33:12Z
- Revision: 1

History:
- 2026-08-31T06:33:12Z KFB8KVH382K28YV4NKXY01EX9Q-m2-bc1be9cb open actor=m2+mac-coordinator targets=digest-landing-race
Integrity: sha256=9f69ddebeaa9218e749df20fd0509a578dd1462960a29ad869742e68f6cfd95f
