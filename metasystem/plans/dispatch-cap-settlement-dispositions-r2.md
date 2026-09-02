# Dispositions, round 2: reservation settlement design critique

Design revision 2 (job cap-settle-design-r2). Critic chain
cap-settle-crit, round 2 (job cap-settle-crit-r2, gpt-5.6-sol). Round 1
is in plans/dispatch-cap-settlement-dispositions.md (one table per file
for the mechanical join). Fold verification: the critic confirmed all
five round-1 findings folded.

## Round 2 — 3 material findings, verdictMaterialCount=3 (all invariant-grade per the critic)

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| DCS-R2-STALE-RESERVED-SNAPSHOT | accepted | Verified: governed.go:149 freezes `ReservedBefore: projection.ReservedJobMinutes` at governed admission and run/conclude.go:316-318 decides exhaustion as `ReservedBefore + observedMinutes >= limit` without re-projecting. Under the new meaning an open cap in that snapshot may have settled downward by conclusion, so a frozen figure over-counts and can mark an obligation exhausted falsely. The design's consumer table called this consumer unchanged; that claim was wrong. (Today's behaviour is the same over-count with caps that never shrink, so the change only improves it, but the design must say how.) | Revision 3: the exhaustion check at conclusion uses a FRESH projection taken at that instant (its observed and open-cap parts, excluding this attempt) plus this attempt's observed minutes; `ReservedBefore` stays on the record as the admission-time fact for the audit trail but no longer decides exhaustion. A test stages the settling-cap case (admitted at equality, the other job settles, the attempt fails) and asserts no exhaustion. |
| DCS-R2-END-BEFORE-DEATH | accepted | Verified: lease/sweep.go:198-204 returns as soon as SIGTERM delivery succeeds and concludeStaleJob stamps `endedAt` immediately (:146-170); dispatch.sh's wind-down (:339-368) waits, escalates to SIGKILL and checks group absence, the sweep does not. A record can be terminal while its runtime still runs, so the settlement under-charges and the cap is no longer reserved for a live process. | Revision 3: the sweep stamps `endedAt` only after death evidence — a bounded wait for the group, SIGKILL on expiry, and a final `kernelGroupAbsent` check (internal/supervise/arming.go:356 already exists) — the same ladder dispatch.sh climbs; a test proves the stamp is not written while the group lives. The sweep is the one terminal writer without death evidence; the other three already act on a dead or self-reported process. |
| DCS-R2-MIXED-START-END-CLOCKS | accepted | Verified: identity_linux.go:47-59 synthesises the process start from boot-time epoch plus start ticks (a wall-clock second that moves under a clock step, identity_test.go:213-250, KI-37), while `endedAt` is a wall-clock RFC3339 stamp; `ownershipProof.provenAt` is the launcher's own wall clock at the ownership write (dispatch.sh:829, ownership.go:75), 0-1 second after the process start on every live record. | Revision 3: the start instant is `ownershipProof.provenAt` (same clock domain as `endedAt`), falling back to `startedAt`; `pidStartedAt` is no longer read by the settlement. Residual, recorded: a host clock step DURING a job shifts the charge by the step size, bounded by the clamp above and by the minute floor below — the KI-37 family; a monotonic cross-process measure does not exist and is not built. |

Trajectory 5 -> 3, falling. Round 3 is the declared failsafe; revision 3
folds these three and returns to the same critic chain.

## Fold to revision 3 (job cap-settle-design-r3, Fable lane)

All three round-2 findings folded (design sections 1.3, 1.9, 4.3, 5;
changelog in the header). Three interpretation gaps reported; the
orchestrator's answers:

| Gap | Answer |
| --- | --- |
| Unknown fresh projection at conclusion: exhaust a failing attempt with a reason naming the unknown record (the obligation then waits for the human), or refuse to terminalize the run? | CONFIRMED as revision 3 chose: exhaust with the named reason. A run left non-terminal on disk would be re-assessed by the reaper while admission is already closed by the same unknown; the exhaustion path is the one every tripped breaker already takes. |
| The final group-absence check: lease cannot import supervise, so `kernelGroupAbsent` moves to the identity package as `identity.GroupAbsent` with supervise pointing at it, rather than a copy inside lease. | CONFIRMED: the move. One owner for the kernel fact, in the package that already owns the process facts. |
| A present but unparseable `ownershipProof.provenAt` is unknownBudget (fail closed); only an absent or empty one falls back to `startedAt`. | CONFIRMED: corruption fails closed; absence falls back. |

Revision 3 goes to the same critic chain as round 3, the declared
failsafe. If material findings remain after it and every one is
mechanical-grain, the principled exit applies (fold as fixture
obligations, build, mandatory code critique).
