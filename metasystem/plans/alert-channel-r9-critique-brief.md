Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal alert-escalation-channel)
Date: 2026-09-01

# Goal

Round-3 critique of metasystem/plans/alert-channel-design.md revision 9
(landed, in your worktree), which folds your six round-2 findings
(records/misc/alert-channel-critique-r8.md, in your worktree). Judge each
fold BY ID. Attack hardest:

1. THE RETENTION HANDSHAKE (11a.8 + new 11a.12, answering the critical
   AC8-JOB-SOURCE-RETENTION-001): terminal job records pinned against
   collection until the alert episode is journaled. Prove or refute the four
   interleaving rows against the shipped collectors (the janitor/evidence-gc
   surfaces, internal/supervise/reaper.go); check the pin is an ANDed
   precondition in EVERY collection path, that pin drain cannot deadlock,
   and that the converse rule truly bounds the episode store.
2. The stop scan's binding to VerifyStopBatchComplete and the
   resume-race suppression ordering.
3. Cursor durability and the bounded read sets under the tick lock.
4. Provenance note, judged as process not product: revision 9 was completed
   by a fresh-context recovery round after the authoring round's partial
   diff — check the document for internal inconsistency between the two
   authors' halves (terminology, section cross-references, contradictory
   rules), the exact seam being section 11a.9 onward.

A clean return closes the design phase; the slice-1 build ships.

# Constraints

Wall-clock budget: 30 minutes. Return per the design-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
