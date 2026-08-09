# Dispositions: supervision lifecycle, round 11 — round 10's folds under fire

Round 11 (gpt-5.6-sol, job supervision-lifecycle-r12, 6 material,
verdict NOT-CONVERGED; the return carried a benign protocol_error —
the brief's "Round: 11" header was copied into the adapter's round
field, a brief defect fixed in the round-12 brief) verified the
round-10 folds and broke three of them with new interleavings. The
close rule's condition (fold-consistency residue only) is not met;
rounds 12-13 remain under the cap. The pattern to learn from: round
10's folds were correct in intent and imprecise at their edges —
each edge is where round 11 cut.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| SLC-R11-001 | accepted | Checkout-wide shutdown succeeded on an ABSENT lock immediately, but a death-only takeover removes the old lock BEFORE renaming its reservation in — a shutdown in that gap reports success while the next owner establishes. | D-1: the settle window covers BOTH exit arms — absent lock and proven-dead identity alike require a re-read after one settle window with no new identity; an acquisition that lands counts as the next loop iteration. The Proof's promise is bounded honestly: shutdown guarantees no live supervision AT RETURN; an independent arm after the window is revocation's business, an ACCEPTED consequence. |
| SLC-R11-002 | accepted | The shutdown predicate's "proven dead" rode on identity helpers that collapse read errors into dead — the design itself admits false-death reads under load, so shutdown could report success over a live, unreadable owner. | D-1: "proven dead" is D-2's DEFINITIVE negative — a successful read showing absence; UNKNOWN is never success, and a loop exhausting on UNKNOWN fails loudly. Three-way identity reads in the shutdown path are a NAMED IMPLEMENTATION ITEM (the committed identity_alive cannot express them; the Go identity package reads kernel state exactly). |
| SLC-R11-003 | accepted | A janitor pausing past its marker's 10-minute staleness resumes and kills — meanwhile an arm has legally cleaned the stale marker and joined, so the kill lands on supervision in live use. The fence existed but nothing made the killer check it at fire time. | D-3: THE MARKER LICENSES THE KILL AT FIRE TIME — the janitor re-reads its own marker in the same observation that captures the kill triple; missing, replaced, or stale aborts the reap (restart the fence). The check-to-kill window is thereby bounded by the marker's validity. |
| SLC-R11-004 | accepted | D-4's owner-terminal-wins said "no append for a clean self-close" while REG-5 still categorically required reaped/swept after kills with append-failure = nonzero — an implementer must pick between success-without-append and an impossible append. | REG-5: the janitor appends what its post-kill re-reduction CALLS FOR — reaped for open claims, swept for sweepable self-closes, NOTHING for a clean self-close (that absence is success); append-or-fail applies only where an append is called for. |
| SLC-R11-005 | accepted | Scorecard reuse validated schema + reviewed commit, but every repetition in a cohort shares the commit — a sibling repetition's scorecard passes the written predicate and advances the wrong repetition. | D-3: reuse requires FULL repetition identity — cohort id, repetition index, repetition count, and commit, the exact four the committed driver compares. One mismatch is FOREIGN: the refusal stands. Only right-identity/invalid-schema is archived and re-extracted. |
| SLC-R11-006 | accepted | "Archive aside under a timestamped name" had no location contract, and the comparer requires the active directory to hold EXACTLY 1..N scorecards — archiving beside them makes the cohort permanently incomparable. | D-3: archives live OUTSIDE the active directory, at `archived-scorecards/<repetition>-<timestamp>.json` in the cohort state directory, with compare.py's exact-set requirement named as the reason. |
