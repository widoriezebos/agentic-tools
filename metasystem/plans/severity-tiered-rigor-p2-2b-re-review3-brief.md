Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal severity-tiered-rigor-p2)
Date: 2026-09-04

# Review brief: re-review of the tiering machinery, slice 2b, after its third correction (chain str-p2-build-2d, round 2)

Round budget: 1 focused round; this closes the slice. R-60-m1's rule:
material only if it changes what gets built and names the artifact.
Prior round: metasystem/records/misc/severity-tiered-rigor-p2-2b-critique-cc3.md;
the correction's orders: metasystem/plans/severity-tiered-rigor-p2-2b-fix3-brief.md.

Subject: the computed diff of implementer job str-p2-build-2d round 2
(its diff.patch under that chain's round-2 directory; reviewed tree
00511d422e3887005f1fbb6931edebdbb18586ac, 46 files against main at c8eb30a6; the diff is the authority).
Round 1 of this chain was reviewed in full at cc3; this round changes
only what the fix brief names. Judge the delta between round 1 and
round 2 (the chain's round-1 diff.patch is beside it), and confirm
that nothing outside the two items moved.

# Mandate

1. STR2P2B-01 closed: in `CritiqueRegisterAdvance` an absent
   `demotions` member reads as empty and is written on the first
   demotion; a present non-list member still refuses with the
   "malformed demotions" text; the tolerance mirrors `exhaustions`
   in critique.go. The two tests exist, exercise the package-level
   fold with a register on disk, and assert the written member and
   the refusal text.
2. STR2P2B-04 closed: the re-indented dispatch.sh lines differ from
   round 1 in whitespace only.
3. Nothing else differs between round 1 and round 2.

Known and out of scope, do not report: STR2P2B-02 and STR2P2B-03
(recorded); the `dispatch` scenario of dispatch-fixtures.sh, red on
main since the alias landing 2c3776b8; the goal package's full run
and the fixture scripts (the seat's run, recorded at landing).

Finding identifiers: this repository's register refuses an id another
chain already carries. Name any finding STR2P2B-05, STR2P2B-06, ...
and never F-n.

# Constraints

Wall-clock budget: 20 minutes. Return per the code-critic schema with
the reviewedTree above. The diff is the subject; do not run
scripts/agents/path-class-fixtures.sh; run a test only if a finding
needs it.

# Gap Rule

stop and report a gap; never fill it silently.
