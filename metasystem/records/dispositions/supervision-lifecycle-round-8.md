# Dispositions: supervision lifecycle, round 8 — races and resolution limits

Round 8 (gpt-5.6-sol, job supervision-lifecycle-r8, 6 material,
verdict NOT-CONVERGED) verified the round-7 folds. The count fell
9 → 6; the kind is now races between the design's own mechanisms and
the resolution limits of the platform (lock birth windows, recordless
joins, whole-second start times). All six accepted and folded.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| SLC-R8-001 | accepted | The ownerless-lock takeover window: an acquirer paused between mkdir and owner.json publication can resume into a lock a waiter legally took over — two concurrent registry writers. | REG-4: acquisition is an atomic RENAME of a pre-populated private directory — the lock is born owning; ownerless directories are garbage by construction. Proof: no ownerless window. |
| SLC-R8-002 | accepted | Retain-every-generation (for safe sweeps) and compact-to-latest (for bounded growth) are contradictory as written; either reading violates a named proof. | REG-2/REG-3: `relaunched` carries `retiredThrough` — the owner verifies the old set dead while stopping it, and that proof licenses compaction to drop retired generations only. Healthy operation compacts to ~1 generation; unverified generations survive until swept. |
| SLC-R8-003 | accepted | A custodied arm that JOINS a live owner appends nothing — no record for the guard, no claim carrying the custodyId, yet D-3 says every successful arm is bound. | D-3: a custodied arm must ESTABLISH (or take over proven death); a live owner on the target means the sandbox is not fresh, so provisioning FAILS instead of joining. |
| SLC-R8-004 | accepted | The janitor's no-live-session check and a human's recordless join are not atomic; announce-then-join can land between check and kill, and no registry reduction can see it. | D-3: REAP-INTENT marker in the supervision dir → bounded grace → RE-CHECK announcements → kill; the arm path refuses to join under a fresh intent, and arming order writes announcements before joining, so every interleaving is caught on one side or the other. |
| SLC-R8-005 | accepted | An unscoped intent file outlives a crashed shutdown caller and relabels an unrelated TERM days later; supersession semantics were undefined. | D-1: the intent names the target owner's exact identity, requester, and time; honored only for THIS owner within the stop-wait cap (a mid-attempt caller crash inside the window is honestly `shutdown`); stale or mismatched intents are ignored, reported, and cleaned. |
| SLC-R8-006 | accepted | pidStartedAt is whole-second in the committed identity source, so pid + start alone matches a same-second pid-reuse stranger; and no POSIX call makes check-then-kill atomic. | REG-6: kill proof is the TRIPLE — pid + start + claim-consistent argv — captured in one observation immediately before signalling; identity alone is never sufficient; the irreducible millisecond residual is stated and accepted, not hidden. |
