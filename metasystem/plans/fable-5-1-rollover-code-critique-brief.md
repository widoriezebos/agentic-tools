Working Mode: implement
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal fable-5-1-model-rollover)
Date: 2026-09-02

# Goal

Two-layer implementation critique of the Fable 5.1 rollover build (job
f51-build-1, reviewedTree 01b88b20, diff.patch in its round evidence): first
conformance against the certified design
(metasystem/plans/fable-5-1-rollover-design.md, section 6 is the contract;
Sol certified it with zero findings in
metasystem/records/misc/fable-5-1-rollover-critique-r2.md), then adversarial
defect review of the diff.

# Attack surface

- Exactly two files change: the two literals in
  TestHazardConfigurationAcceptsConfiguredMaximalModel and one appended row
  in the rulings table. Any other hunk is a finding.
- The R-46-m0b row must match the design's section 4 text byte for byte and
  must not disturb R-25-m1 or the table's column count.
- The test must be green against the committed
  metasystem/metasystem.conf (maximal-models is claude-fable-5-1 alone) and
  must not depend on any metasystem.conf.local.
- Any other test or fixture in the tree that composes against the real root
  with the old id would still be red; name it if it exists.

# Constraints

Wall-clock budget: 15 minutes. Return per the code-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
