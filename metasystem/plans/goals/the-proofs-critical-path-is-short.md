# the-proofs-critical-path-is-short

- State: queued
- Risk: severity=2 novelty=2 exposure=3 accumulation=2 basis="Tests that declare t.Parallel run in parallel for the first time in a proof; a race shows as a red that passes alone and is recorded as such; N falls to 1 when the host is busy or the load unreadable; nothing lands on main differently; about 82 production lines in U1"
- Tier: 2
- Intent: The landing lane full proof ends in about the time of its slowest package: parallel-safe tests run in parallel inside a package with N chosen at each package admission from a live load sample under a host cap, serial tests stay serial, every launch records its effective N, CPU and start time, the proof reports its critical path, and a red seen only in parallel is recorded as failed in parallel and passed alone (measured 2026-11-04: 49 min wall, 683 summed package minutes, -parallel=1 everywhere)
- Origin: human
- Next step: Build U1 (load-banded N per package admission, recorded; per-launch CPU and start time) from the accepted page plans/designs/the-proofs-critical-path-is-short.md with a brief cut by opus-designer (BRIEF-CHECKLIST, one Astra round); then compare N=1 and N>1 on the same candidate before U2 (the readout), U3 (the order) and U4 (the load-fragile record, --last)
- OpenedAt: 2026-11-04T12:24:04Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-04T12:24:04Z 5A5EWYPYZN1JK179BQPYKCJJJV-m1e-1789f780 open actor=human:Wido targets=the-proofs-critical-path-is-short
Integrity: sha256=702fd10442cf6a749ff8ca5a61003b5802e1a9ae330e532785dfe0e398a21697
