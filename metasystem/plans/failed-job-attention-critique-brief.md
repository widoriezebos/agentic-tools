Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal failed-job-attention)
Date: 2026-09-02

# Goal

Independent critique of metasystem/plans/failed-job-attention-design.md
(landed, in your worktree): two minimal steward escalation classes for
terminal-failed delegate jobs and fired breach-stops, following the
stalled-idle episode pattern.

# Attack

1. Firing correctness: every terminal-failure writer covered (the
   RecordProtocolError class included); no episode for jobs outside
   claimed goals; the breach-stop class fires exactly once per stop.
2. Clearing: one rule per class, no orphaned episodes, no contradiction
   with the stalled-idle lifecycle or a goal release mid-episode.
3. Dedup keys against job-id reuse (the birth-token gap is a known
   sibling — goal job-record-birth-token; judge whether these episodes
   need it now or their lifetime makes reuse harmless, and demand the
   honest statement either way).
4. The seam to the alert channel: the disclosed gap (the channel expects
   answerAction/answerReason fields these episodes omit) — is the seam
   statement sound, or does it set up a future migration break?
5. Bounded tick reads and the fixture plan's determinism.

A clean return closes the design; the build fits the box.

# Constraints

Wall-clock budget: 20 minutes. Return per the design-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
