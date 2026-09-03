# Path-class manifest, second part re-issued on m2: closing code review, round 1 (chain path-class-build2b-cc1)

Reviewed tree 63b76e52e848c5c6e9b591403e74f6e79498a602 (implementer round 1 of chain path-class-build2b, the byte-exact re-issue of m1's round-5 tree 852f7250). Critic: Claude Fable 5.1. Brief: plans/path-class-manifest-build2-code-critique3-brief.md.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| PCM-CC9-001 | accepted | The README leg in static-reproof-fixtures.sh expects path-unclassified in an adopted root-layout fixture, where the evaluator (design revision 2, sections 1 and 3) answers outside and lets carriage pass; the evaluator is right, the leg is wrong, and no root-layout path can reach path-unclassified with the shipped manifest. The m2 gate replay observed the red before the review. | Implementer round 2 (plans/path-class-manifest-build2-fix5-brief.md): the leg moves to the vendored layout, README inside the installation. |
| PCM-CC9-002 | noted | Comment in internal/stateroot/owner.go no longer names goal adoption-inventory-from-install-set; the goal record names the file. Non-material. | none |
| PCM-CC9-003 | noted | The ownership oracle runs once per changed path; latency only, no verdict changes. Non-material. | none |
| PCM-CC9-004 | noted | Class-check order differs between exact revert and carriage on mixed record-plus-ledger inputs; both orders conform to the design's floor-first rule. Recorded as a test obligation for the second landing bar's chain. | none |
