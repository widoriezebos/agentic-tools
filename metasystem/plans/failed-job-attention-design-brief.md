Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal failed-job-attention)
Date: 2026-09-02

# Goal

Author the design document failed-job-attention-design.md, a NEW file you
create in the metasystem plans directory: the steward raises escalation
episodes when a delegate job under a claimed goal reaches terminal failure
unacknowledged, and when a breach-stop fires — through what exists TODAY
(episode store, health output, digest), no new delivery machinery.

# Evidence and inputs

- The goal record: metasystem/plans/goals/failed-job-attention.md (in your
  worktree) — Wido's fix-before-anything word; appetite 1.5h; two new
  escalation classes following the stalled-idle episode pattern exactly;
  episode store is source of truth; dedup per job and per stop.
- The input brief m3 left: metasystem/plans/failed-job-attention-brief.md
  (in your worktree) — INPUT MATERIAL, not a certified design.
- The incident: metasystem/records/misc/idle-loss-2026-09-01.md — the
  reaper knew in seconds, nothing carried it, six hours lost; this
  session's own idle stop is the third specimen.
- The existing pattern to follow: the stalled-idle escalation episodes in
  internal/steward (tick.go, alert_episode.go) — your two new classes are
  siblings, not a new mechanism.
- ADJACENT LANDED DESIGN, respect its boundaries: the alert channel design
  (metasystem/plans/alert-channel-design.md) specifies rich producer
  derivation scans for its future external channel — that goal owns
  DELIVERY and its producers; THIS goal is the minimal in-steward
  escalation that exists TODAY. State the seam so the two never fight:
  same episode store, this goal's episodes are the simple immediate class,
  the channel later consumes them.

# Design questions

1. The two classes' exact episode shape (digest derivation, dedup key per
   job / per stop, message content with plain-language facts: goal, job,
   failure reason, when).
2. The tick integration point and its read set (bounded — the claimed
   goals' job records the tick already touches).
3. Acknowledgment: when does the episode clear (job acknowledged? goal
   released? stop resumed?) — one rule per class, no contradiction with
   the stalled-idle lifecycle.
4. The fixtures proving both classes fire and clear deterministically.
5. Blast: consumers of episode listings (health, digest, counselor).

Self-grade with reject condition.

# Constraints

Wall-clock budget: 25 minutes. Design document only; one new file in the
metasystem plans directory. The 1.5h appetite is the scope governor: the
SIMPLEST design honoring the episode pattern wins.

# Expected Return

Version-2 implementer JSON; diffBoundary lists exactly the one new design
file you created.

# Gap Rule

stop and report a gap; never fill it silently.
