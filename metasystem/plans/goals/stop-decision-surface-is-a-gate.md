# stop-decision-surface-is-a-gate

- State: queued
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="severity 3: an undeclared Stop-decision change is how the loop broke; novelty 2: the extraction exists as a script, the gate integration and declaration are new; exposure 3: every landing that touches the Stop path; accumulation 2: each undetected inversion compounds across seats"
- Tier: 3
- Intent: A change that moves a Stop decision must say so, enforced by the machinery. On 2026-09-13 a build quietly closed the third-refusal release and the delegate-job exemption; it was caught only by a scratchpad script (decision-surface.sh) that extracts every assertion naming a Stop decision and diffs it against trunk. That guard lives outside the repository, so no other seat has it and nothing enforces it. DONE: the check runs in the gate; a change moving any assertion that names a Stop decision is refused unless the landing declares the moved assertions and the goal permitting the change; additions are reported, removals and changes refused without that declaration.
- Origin: human
- Next step: Port the extraction into the gate as a Go check over the Go test files and fixture beds that assert Stop decisions, with a declaration format the landing carries; prove it refuses the 2026-09-13 inversion and admits member 3's purely additive lines.
- OpenedAt: 2026-09-14T16:16:14Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-14T16:16:14Z 9JVA7KASJX29BAW3B4B45ZN96E-m1e-c6925449 open actor=human:Wido targets=stop-decision-surface-is-a-gate
Integrity: sha256=d2364d9909a6887288e8933ba387414f0717eef337bd23e5e995f7366135904b
