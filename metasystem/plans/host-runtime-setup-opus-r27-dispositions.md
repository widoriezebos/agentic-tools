# Final host portability code review

The fresh Claude Opus 5/xhigh review `host-setup-recover2-r8-review` binds
installation tree `b2a3f57b99633f70552b68dbf9533f741c0be115` and reports zero
material findings. Main read the complete canonical return. The review covers
the complete 61-path source diff and the separately bound 45 generated outer
registrations plus the user-instruction correction. Full repository tree:
`25f9a8663bd6a2733df15cd41f3fa8da6e7eae35`.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| HOST-C10-001 | noted | Reversing the two managed instruction markers by hand is outside the valid managed-block shape produced by setup. The splice leaves marker debris but preserves all foreign text; the next setup/check refuses mismatched counts. Main read the exact count guard and splice in internal/hostsetup/setup.go. This malformed-input corner remains a reported limitation; no arbitrary malformed-block repair guarantee is claimed. It does not affect generated blocks, preserved ordinary instructions, or the tested setup/check/repeat paths. | none |
| HOST-C10-002 | noted | Preparing an executable snapshot now requires access to the persistent preparation lock, including when reusing a pin. That is the synchronization needed to preserve retained inodes across processes. The existing owner creates a writable private directory; a manually read-only directory or mount fails explicitly. No supported read-only preparation contract is claimed. | none |
| HOST-C10-003 | noted | A helper timeout includes the underlying signal error under a general lost-command assertion. The assertion still fails and cleanup reaps helpers. This is diagnostic wording, not a passing test on failure. Main Linux and Darwin focused race checks pass; the final battery will also exercise the test under its normal package load. | none |

The critic independently reproduced both new concurrency regressions against
old source and observed them passing with the fix. Its whole steward package
attempt was limited by missing sandbox module dependencies. Main's own complete
Darwin package canary passes with race checks and 79.4 percent coverage against
the unchanged 74.0 floor, and all three original native rearm scenarios pass.
Focused native Linux race tests also pass. The review identified that Linux
whole-package coverage had not yet been measured; main runs that named check
before final suite readiness rather than assuming the Darwin number applies.
That whole native Linux package now passes with race checks in27.64seconds,
measuring78.6percent coverage against the unchanged78.5floor. Main read the
complete primary log: recovery-r27-linux-steward-coverage.log.

Evidence qualifications: the review's lock-order paragraph overstates that no
caller holds one lock while waiting for the other. Foreground arm does hold
arm.flock while taking the preparation lock; the relevant fact is that watcher
preparation releases its lock before taking arm.flock, so there is no inverse
held-lock order. Several review evidence commands are descriptive templates,
not verbatim replay commands. Main's canonical conformance, delta, canary
command/result files and primary logs remain the decisive replayable evidence.

R26's complete battery remains RED and cannot authorize landing. R27 changes
only the existing steward identity owner and its tests; all prior 59 source
paths and generated registrations remain unchanged. No source amendment or
risk waiver follows this review. The final full battery must itself exit zero
before the same shell chain can perform normal commit and push. The builder
remains open until that success. Automatic provider callbacks in the already
running coordinator remain unobserved.
