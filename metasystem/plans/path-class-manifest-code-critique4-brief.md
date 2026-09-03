Working Mode: implement
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal path-class-manifest)
Date: 2026-09-03

# Goal

Closing code review of the path-class manifest, first part, re-issued as
chain path-class-build1c (its round evidence holds diff.patch and
review.json with the reviewed tree). The content was reviewed three
times on the previous chain (registers
metasystem/records/misc/path-class-manifest-code-critique-r1.md and
-r2.md; the last correction folded the two waiver findings and the
landing coverage floor). This round only re-applied the cumulative diff
under a correct boundary declaration. Verify that, and that nothing
regressed; the chain closes on your zero.

# Mandate

1. The applied diff equals the cumulative patch at
   metasystem/artifacts/agents/path-class-build1/cumulative.patch on the
   primary checkout: same files, same hunks, nothing added or dropped;
   the declared boundary names every changed path with the metasystem/
   prefix, deletions included.
2. The three last closures hold on this tree: the waiver rule refuses
   behavior, ledger, runtime and unclassified inside the installation and
   leaves record and outside paths waivable (internal/validate/conformance.go
   and its tests); the adopted root layout decides by mode before
   namespace; internal/pathclass and internal/landing have coverage
   floors in both baselines at the measured numbers.
3. Regression check against the certified design
   (metasystem/plans/path-class-manifest-design.md revision 2, first
   part) and the earlier closures (PCM-CC1-001 to 005, PCM-R2-001 and
   004).

A finding is material only if it changes what gets built and names the
artifact. If nothing material remains, say so.

# Constraints

Wall-clock budget: 20 minutes. Return per the code-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
