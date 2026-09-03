Working Mode: implement
Orchestrator Identity: m1b+main-1788333346-60696-6a3256 (dispatch delegate under goal token-spend-fence)
Date: 2026-09-03

# Goal

The one code review (R-54-m1 tier 3) of the token spend fence build,
implementer job fence-build (gpt-5.6-sol), against its brief
metasystem/plans/token-spend-fence-build-brief.md, the accepted design
metasystem/plans/token-spend-fence-design.md (revision 2) and the two
test obligations in
metasystem/plans/token-spend-fence-dispositions-closing.md. The
computed diff and reviewedTree are the conformance artefacts of that
job (artifacts/agents/fence-build/rounds/<n>/diff.patch and
review.json); review that diff, never the delegate's own summary.

# Review brief

Two ordered layers per the code-critique skill. LAYER 1, conformance:
every acceptance criterion of the brief present; every touched path
inside the declared workspace and the design's sections; no non-goal
touched (no adapter usage writer, no admission path, no goal or
goalbudget package, no new CLI verb); no test weakened; the two
obligation tests present and discriminating. LAYER 2, adversarial:
attack the reader (double counting across a chain's rounds, an
unavailable or unreadable record entering a total, the UTC day rule),
the seat meter (delegate exclusion by session id, the day-versus-goal
age rule, shape failures counted not dropped, the fixture transcript
only), the money rule (native wins, unpriced never zero, foreign
beside), the config validator (six keys, enforce refused by name,
committed-root law), the health line bytes and the alive-on-crossing
rule, and the per-crossing episodes (once per crossing, per multiple,
cleared per multiple, unknown clears nothing, other roles' status
irrelevant).

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
