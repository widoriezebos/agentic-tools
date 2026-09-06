# Dispositions: fsrd-cc1-20260906, round 1 (the chain's closing review)

Chain under review: fsrd-build1b-20260906 (reviewed tree
5d19d3e93c592f050e11f023e909c37dc58abac9, round 1). Critic:
fsrd-cc1-20260906 (claude, Fable), zero material findings, five low
notes; the chain is closed on this review. Orchestrator: m1d. The
reviewer ran read-only and could not rerun the gate or the suites; the
seat's outside-sandbox replay on the reviewed tree covers that gap:
the steward and lease packages passed, the health fixture suite passed
(the fixture-notification scenario included), the supervision-hook
fixture suite passed, and the dispatch fixture suite passed every
scenario in the builder's sandbox run (the seat's first replay died on
the seat's own alarm, a second replay with a wider bound was in flight
at this writing).

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| FSD-01 | noted | True: the arm function's fixture restamp for a fake-runtime root sits behind the runner exclusion that returns first for the same root, so today no mint reaches it and fixture identities come from the suites' own hand-written records. Dead code, not a defect; the fixture kind still lands on every fixture enrollment. Backlogged with the other notes. | none |
| FSD-02 | noted | True: no test pins that a real kernel terminal classifies human without the fixture-granted flag. By reading, the kernel branch returns a plain human class and the flag is set on two lines fed only by the fixture table. Backlogged: one negative test. | none |
| FSD-03 | noted | True: the health suite asserts the fixture kind after the arm and not after either restart. RestartFixture is the fixture arm with replacement, so the path is correct by inspection and unproven by a suite. Backlogged: one assertion after a restart. | none |
| FSD-04 | noted | True: the supervision, delegate-caps and fingerprint suites hand-write steward identities without the enrollment field, which reads as human-terminal. None can reach the desktop: the supervision re-arm roots configure a notify command that wins, the rest declare fake runtimes and never launch a runner. Outside the brief's owners; backlogged so every suite identity carries the fixture kind. | none |
| FSD-05 | noted | True: VerifyIdentity refuses an enrollment value outside the three kinds, so an engine that adds a fourth would be refused by an older engine on the same host. No such value exists, identities are per repository, and a refused identity is a loud failure at arm, which is the behaviour wanted for a mixed-engine host. No change. | none |
