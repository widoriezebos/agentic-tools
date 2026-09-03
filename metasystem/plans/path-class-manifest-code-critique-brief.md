Working Mode: implement
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal path-class-manifest)
Date: 2026-09-03

# Goal

Two-layer implementation critique of slice 1 of the path-class manifest
(chain root path-class-build1, terminal round path-class-build1-r2; its
round evidence holds diff.patch and review.json with the reviewed tree):
first conformance against the certified design
(metasystem/plans/path-class-manifest-design.md revision 2, sections 1,
2, 3, 4, 6, 7 and 8 are the contract for slice 1; Sol's two rounds are
metasystem/records/misc/path-class-manifest-critique-r1.md and
-r2.md, the second naming test obligations PCM-R2-001 and PCM-R2-004 for
this slice), then adversarial defect review of the diff.

# Attack surface

- The manifest scripts/agents/path-classes.txt: every row against the
  design's section 1 table; a tracked top-level entry with no row; a
  behavior path classified record or the reverse; the three row kinds and
  the two namespaces.
- The resolver internal/pathclass/pathclass.go: the input resolved
  against the caller's directory before choosing the namespace (the
  orchestrator reproduced the unresolved form in round 1 and it was
  corrected in round 2: verify the correction and its test
  TestResolveSameFileThreeInputForms); adopted ownership consulted before
  matching (PCM-R2-001); longest prefix; the sentinel refusal text.
- The landing evaluator internal/landing/observe.go: the floor reads
  behavior rows and nothing else; carriage eligibility is record class
  plus the three append-only files; the handoff exception retained
  (PCM-R2-004); no verdict code widened in this slice; the base tree, not
  the checkout, is the manifest read (compare with the design's section
  3 and the round-1 finding PCM-R1-009).
- Conformance internal/validate/conformance.go: the waiver rule reads the
  manifest; a runtime-declared instruction file whose class is not
  behavior is rejected (PCM-R1-011).
- The deletions: no live reader of the two deleted lists remains
  anywhere in behavior paths (run the search the fixture runs, and a
  wider one).
- Fixtures scripts/agents/path-class-fixtures.sh and the Go tests: each
  deterministic, at the seam that can fail; the real-command fixture
  runs the verb from inside the installation.
- Anything outside the declared boundary is a finding; slice 2's files
  (commit.sh, land.sh, promotion.go, landing-promotion.json,
  landing_verbs.go) must be untouched.

# Stop criterion

A finding is material only if it changes what gets built and names the
artifact. This is the one code review before landing; whatever is
material goes to one correction round, then the chain lands.

# Constraints

Wall-clock budget: 40 minutes. Return per the code-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
