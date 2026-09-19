# test-selection-improves-on-the-simple-rule

- State: queued
- Risk: severity=1 novelty=2 exposure=2 accumulation=1 basis="severity 1: a selection that misses a red is caught by the full run once per batch; novelty 2: replay over recorded gates has not been done; exposure 2: every fix round and every re-proof after an eject; accumulation 1: one decision and at most three units"
- Tier: 2
- Intent: Wido 2026-09-19 about 22:50 CEST: "the deep analysis we should park for later, and determine whether we can improve on what we have with the simple selector with a new backlog item." The impacted-tests verb design, plans/impacted-tests-verb-design.md revision r2 at commit be795570e, lists eight units. U1 to U3 (protocol, full-run fallback, public verb) are built under machinery-runs-unattended-on-codex. The deep-analysis units U4 to U7 (coarse closure, declaration precision, importer and method precision, tags and observed outcomes) are parked here and not built. The simple selector is the rule in the m1e integration gate (scratchpad dm-gate-select.py, mirrored under /Users/wido/LocalStorage/hact-20260912/m1e-evidence-mirror/s19): a fix round reruns the red tests, the tests that name a changed declaration and whole packages whose state the fix changes, and skips green packages the fix cannot reach; the full run happens once per batch. Measured on push 17: the transitive closure selects 547 of 576 cmd/metasystem tests, the simple rule selects 8 tests plus one package, and both reds are caught. Lessons L1 to L7 are on the design page. DONE: (1) The simple rule and each candidate deeper selection are replayed over at least ten recorded red fix rounds from the direct-mode and lane batches, with per round the tests selected, the wall time and whether every red was caught. (2) A deeper unit is built only if the replay shows it catches a red the simple rule misses, or halves the fix-round wall time with no missed red. (3) Otherwise the goal concludes with the numbers and the design page marks U4 to U7 as not built.
- Origin: human
- Next step: Unapproved; waits for Wido's pick after the switch-on. Parked work: origin branch park/test-selection-improves-on-the-simple-rule at 5cabcf38f holds design r2 (be795570e) and the unlanded U1 to U3 build; Wido 2026-09-19 23:25 CEST approved one verb, test run, so the U1 protocol and U4 to U7 park here. First step: collect the recorded red gates (dm-int-<n>-gate, gate-all and witness-gate logs in the m1e evidence mirror; 14 of 33 integrations had a red) and replay the simple rule, now internal/testselect, over them.
- OpenedAt: 2026-09-19T20:55:29Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-19T20:55:29Z WMKSS1T0PK4C19YD40KEWPE8T7-m1e-c6925449 open actor=human:Wido targets=test-selection-improves-on-the-simple-rule
- 2026-09-19T21:25:51Z 5TCPEZJHR5X967KZ56J314FSKF-m1e-c19f13ae edit actor=m1e+main-1789561396-11601-6f713b targets=test-selection-improves-on-the-simple-rule
Integrity: sha256=ea379e074492f5bab1813327a6cb570baaa202008ceeb21ba5e1f4d444dc43ec
