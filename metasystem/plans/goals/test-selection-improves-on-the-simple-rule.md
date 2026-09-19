# test-selection-improves-on-the-simple-rule

- State: queued
- Risk: severity=1 novelty=2 exposure=2 accumulation=1 basis="severity 1: a selection that misses a red is caught by the full run once per batch; novelty 2: replay over recorded gates has not been done; exposure 2: every fix round and every re-proof after an eject; accumulation 1: one decision and at most three units"
- Tier: 2
- Intent: Wido 2026-09-19 about 22:50 CEST: "the deep analysis we should park for later, and determine whether we can improve on what we have with the simple selector with a new backlog item." The impacted-tests verb design, plans/impacted-tests-verb-design.md revision r2 at commit be795570e, lists eight units. U1 to U3 (protocol, full-run fallback, public verb) are built under machinery-runs-unattended-on-codex. The deep-analysis units U4 to U7 (coarse closure, declaration precision, importer and method precision, tags and observed outcomes) are parked here and not built. The simple selector is the rule in the m1e integration gate (scratchpad dm-gate-select.py, mirrored under /Users/wido/LocalStorage/hact-20260912/m1e-evidence-mirror/s19): a fix round reruns the red tests, the tests that name a changed declaration and whole packages whose state the fix changes, and skips green packages the fix cannot reach; the full run happens once per batch. Measured on push 17: the transitive closure selects 547 of 576 cmd/metasystem tests, the simple rule selects 8 tests plus one package, and both reds are caught. Lessons L1 to L7 are on the design page. DONE: (1) The simple rule and each candidate deeper selection are replayed over at least ten recorded red fix rounds from the direct-mode and lane batches, with per round the tests selected, the wall time and whether every red was caught. (2) A deeper unit is built only if the replay shows it catches a red the simple rule misses, or halves the fix-round wall time with no missed red. (3) Otherwise the goal concludes with the numbers and the design page marks U4 to U7 as not built.
- Origin: human
- Next step: Unapproved; waits for Wido's pick after the switch-on. Parked work: origin branch park/test-selection-improves-on-the-simple-rule at 5cabcf38f holds design r2 (be795570e) and the unlanded U1 to U3 build. Wido 2026-09-19 23:27 CEST: no complexity unless the benefit is real. First question: does the lane need any selection beyond its testing.json groups and passed-group reuse? Measure the lane proof per batch from the rehearsal and the switch-on batches, then replay the direct-mode simple rule (m1e evidence mirror, dm-gate-select.py; 14 of 33 integrations had a red) over the same batches. Build only if the replay shows a real saving with no missed red; M1c's one-verb proposal (hact-20260912/m1c-dm-0917/verbs/test-verbs-proposal.md, verb set approved by Wido 23:25 CEST) is the shape to use then.
- OpenedAt: 2026-09-19T20:55:29Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-19T20:55:29Z WMKSS1T0PK4C19YD40KEWPE8T7-m1e-c6925449 open actor=human:Wido targets=test-selection-improves-on-the-simple-rule
- 2026-09-19T21:25:51Z 5TCPEZJHR5X967KZ56J314FSKF-m1e-c19f13ae edit actor=m1e+main-1789561396-11601-6f713b targets=test-selection-improves-on-the-simple-rule
- 2026-09-19T21:27:30Z A207EBSDVH9084EC5112AJ8QZT-m1e-c19f13ae edit actor=m1e+main-1789561396-11601-6f713b targets=test-selection-improves-on-the-simple-rule
Integrity: sha256=3b845731a04c96d601b8a0d518dcbe9bd541d6e14cc8d1678711918a98019ced
