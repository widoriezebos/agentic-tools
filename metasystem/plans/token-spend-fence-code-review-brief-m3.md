Working Mode: implement
Orchestrator Identity: m3+main-1788172645-85501-aa86ee (dispatch delegate under goal token-spend-fence)
Date: 2026-09-03

# Goal

The one code review (R-54-m1 tier 3) of the token spend fence build,
implementer job fence-build-m3 (gpt-5.6-sol), against its briefs
metasystem/plans/token-spend-fence-build-brief.md and
metasystem/plans/token-spend-fence-build-resume-brief.md, the accepted
design metasystem/plans/token-spend-fence-design.md (revision 2), the
two test obligations and the three gap answers in
metasystem/plans/token-spend-fence-dispositions-closing.md. This
brief is the landed m1b review brief
(metasystem/plans/token-spend-fence-code-review-brief.md) carried to
the m3 chain; nothing in its review scope is narrowed. The computed
diff and reviewedTree are the conformance artefacts of that job
(artifacts/agents/fence-build-m3/rounds/<n>/diff.patch and
review.json, n=1 for this chain; the review-stage conformance passed at
reviewedTree 2c0572533ba23f967e63eaab321d3f9e003a7e4c); review that
diff, never the delegate's own summary. The diff is the previous
chain's preserved build (cherry-picked byte-identical, patch-id
0a9283a64dbcbac286c3b1bd58f3396e8cd03510) plus one commit for the
round-3 gap answer (unreadable seat transcripts).

# Review brief

Two ordered layers per the code-critique skill. LAYER 1, conformance:
every acceptance criterion of both briefs present; every touched path
inside the declared workspace and the design's sections; no non-goal
touched (no adapter usage writer, no admission path, no goal or
goalbudget package, no new CLI verb, nothing under plans/); no test
weakened; the two obligation tests present and discriminating; the
round-3 gap answer built exactly as its row binds (every unreadable
present transcript path is one `seat unreadable` unmeasured entry
{path, error}, `SeatSummary.UnreadableFiles`, the seat segment
`files=<n> aged=<n> unreadable=<n>`, Measure never fails on it, the
role stays alive, test TestSeatUnreadableTranscriptIsCountedNotSkipped
present and discriminating). LAYER 2, adversarial: attack the reader
(double counting across a chain's rounds, an unavailable or unreadable
record entering a total, the UTC day rule), the seat meter (delegate
exclusion by session id, the day-versus-goal age rule, shape failures
counted not dropped, unreadable paths counted not skipped and never
mixed into the request-shape count, the fixture transcript only), the
money rule (native wins, unpriced never zero, foreign beside), the
config validator (six keys, enforce refused by name, committed-root
law), the health line bytes and the alive-on-crossing rule, and the
per-crossing episodes (once per crossing, per multiple, cleared per
multiple, unknown clears nothing, other roles' status irrelevant).

Materiality criterion, verbatim: would the change ship a defect,
violate its brief, or damage what certifies it? Count only material
findings in the verdict. Under R-60-m1 this is the ONE code review;
material findings return to the implementer as one correction round
and you review the corrected tree once.

Run what the sandbox allows: `go build ./...`, `go test` over
internal/spend, internal/mission, internal/config, internal/steward,
and the `go list -deps` proof that no admission package imports
internal/spend. Report what could not run.

Return format: the code-critic schema; stable identifiers TSF-C1-<name>;
carry the reviewedTree from review.json into the return.

# Constraints

Wall-clock budget: 30 minutes. Do not edit the implementation; findings
only.

# Gap Rule

stop and report a gap; never fill it silently.
