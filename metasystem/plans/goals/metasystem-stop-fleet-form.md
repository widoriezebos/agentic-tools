# metasystem-stop-fleet-form

- State: queued
- Priority: 3
- Sequence: 17
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="severity 3: the fleet form signals processes in checkouts it does not own, and a wrong identity there ends a human session; novelty 2: one process enumeration shared with the census, and a registry that keeps no owner identity; exposure 3: every checkout on the host, not just the seat's own; accumulation 2: the directory-gone branch will be asked for again each time a checkout is removed"
- Tier: 3
- Intent: Slice 2 of the stop verb: the --all fleet form of section 8 of plans/metasystem-stop-verb-design.md, as amended by its section 13.3, plus the single shared process enumeration section 3 step 7 defers in slice 1. Slice 1 refuses --all with the sentence section 13.3 names. DONE means stop --all and status --all act per checkout under each checkout's own lock and fence, a checkout whose directory is gone is reported and never signalled from a command-line shape, its recorded owner identity is named when the registry knows one, the untracked sweep shares one enumeration with the supervision family instead of running its own, and the fleet fixture proves an announced human session survives
- Origin: main
- Next step: Blocked until slice 1 lands. Read first: internal/registry/reduce.go keeps no owner process identity in PublishedOwner, so the directory-gone branch can only report what the registry actually holds, and internal/census exports no entry point that accepts a pre-enumerated process table, which is why the shared enumeration is deferred rather than faked. The closing design critique recorded both as STOP-R3-003 and STOP-R3-006 in records/misc/metasystem-stop-critique-r3.md
- OpenedAt: 2026-09-07T07:30:31Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-07T07:30:31Z 0B1KJ7PY0YEBJV104YWT9XAA4X-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=metasystem-stop-fleet-form
- 2026-09-08T16:00:04Z 515799775HCPSCZG9MV66XR974-m1-7cd0bd60 set-priority actor=human:Wido targets=metasystem-stop-fleet-form reason=priority-order subject=metasystem-stop-fleet-form from=unranked to=3:17 requested-sequence=17
Integrity: sha256=770b4b9ee02a38e28a9ee348b1cf401100820ef0dd1f2eca75cb5f9efccd369d
