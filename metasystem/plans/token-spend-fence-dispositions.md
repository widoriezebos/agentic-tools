# Dispositions: token spend fence design, the one Sol review

Design: plans/token-spend-fence-design.md (job fence-design-1, Fable
lane). Review: job fence-review-1 (gpt-5.6-sol, xhigh). Adjudicated by
the dispatch delegate m1b+main-1788333346-60696-6a3256 under R-60-m1: a
finding is material only if it changes what gets built and names the
artifact. Tier 3 ladder (R-54-m1): this review, one fold, one closing
review; disputed points at the budget become named test obligations.
The reviewer could not run Go tests in its sandbox (its gap); every
finding below was verified by reading the cited code.

## Round 1 — 4 material findings, verdictMaterialCount=4

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| TSF-R1-alert-crossing-identity | accepted | Verified in internal/steward/alert_episode.go: episodes are keyed by the whole health digest, cleared only when the entire aggregate is healthy, and an uncleared matching digest is reused without delivery. The design's single sorted-list EpisodeKey therefore makes two ceilings crossed in one tick one episode, and a crossing that disappears while another role stays dead cannot re-alert when it recurs. Changes what gets built (health.go, alert_episode.go). | Fold: one episode identity per (scope, ceiling) crossing, each with its own key and its own submission, and a clearance rule that re-arms a crossing's episode when that crossing alone clears, independent of other roles; the design chooses the mechanism (a per-crossing register beside the digest, or per-key episodes) and names TestSpendFenceCrossingsHaveIndependentEpisodesAndRearmWhileOtherRoleDead. |
| TSF-R1-shared-checkout-double-count | accepted | Verified: shared-checkout is a supported launch mode whose workspace is the repository root (internal/dispatch/claim_fingerprint.go admits both modes), so a Claude delegate in shared-checkout mode writes transcript lines whose cwd passes the seat filter while its job record is also measured. Job records carry the runtime session id (sessionId / resumedSessionId), so the exclusion can be exact. | Fold: the seat reader excludes any transcript session whose sessionId appears on a job record (any launch mode), and keeps the worktree-cwd rule only as a second guard; test TestSeatTranscriptExcludesSharedCheckoutDelegateSession. |
| TSF-R1-seat-omission-honesty | accepted | Verified against the design's own text: goal scope spans all days but the seat reader skips files older than 48 hours, so the seat row can shrink silently; a transcript line whose usage shape is absent is dropped silently; neither skipped files nor rejected requests are disclosed beyond a file count. The threat model names exactly this. | Fold: the age filter applies to the DAY scope only and the goal scope reads every present file; a request rejected for shape is counted as `seat unmeasured requests=<n>` on its own ledger line and health line segment; files skipped by age are counted and printed; tests TestSeatTranscriptShapeFailureIsUnmeasured and TestSeatGoalDoesNotSilentlyLoseAgedTranscriptSpend. |
| TSF-R1-job-record-read-honesty | accepted | Verified: internal/mission/fence.go:657-660 continues past a record that fails to parse, so the lifted rule would let an unreadable record's spend vanish; the design's unknown rule names only the directory, the ledger and the conf. | Fold: an unreadable or malformed job record is an explicit unmeasured ledger entry naming the file and the parse error (spend never disappears; health stays alive with the count shown), and health goes unknown only when the jobs directory itself cannot be listed; test TestUnreadableJobRecordCannotDisappear. |

Round 1 closes on four accepted findings; the fold produces the design's
revision 2, then the closing review. Any point still disputed after the
closing review becomes a named test obligation in this file, never
another round (R-60-m1).
