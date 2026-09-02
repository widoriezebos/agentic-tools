# Dispositions, round 3 (the failsafe): reservation settlement design critique

Design revision 3 (job cap-settle-design-r3). Critic chain
cap-settle-crit, round 3 (job cap-settle-crit-r3, gpt-5.6-sol). Rounds 1
and 2: plans/dispatch-cap-settlement-dispositions.md and
plans/dispatch-cap-settlement-dispositions-r2.md. Fold verification: the
critic confirmed all three round-2 findings folded.

## Round 3 — 3 material findings, verdictMaterialCount=3 (two shape-level, one requirement failure)

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| DCS-R3-RETRY-SELF-PROJECTION | accepted | Verified: obligationstate.RecordTerminal "commits the terminal spend before its prunable run evidence" (state.go:241-243) and the run record's terminal write follows in run.go's compare-and-swap (:396-430). A retry after a partial commit finds an open run record beside an unpruned durable attempt for the same run; excluding only the run record from the fresh projection leaves the durable attempt in, the durable-owner check reports BudgetUnknown, the retry computes different exhausted fields and RecordTerminal refuses them as conflicting. Shape-level. | Revision 4: the fresh projection excludes the concluding run by run id from BOTH stores — the run-record loop and the durable terminal attempts — so a retry after a partial commit projects the same spend as the first call; T13 gains the partial-commit retry case (durable attempt present, run record open) asserting convergence to identical terminal fields. |
| DCS-R3-PROJECTION-WIRING | accepted | Verified: besides cmd/metasystem/run.go:54, production run stores are built bare at cmd/metasystem/supervise_component.go:245 (Assess terminalizes any non-terminal run) and internal/lease/sweep.go:67 (SweepStale forces a stale run terminal); under revision 3's nil-hook rule a failing governed attempt concluded through either path is exhausted with BUDGET_UNKNOWN whatever its spend. Shape-level. | Revision 4: one production constructor for a run store that can terminalize, carrying the spend seam, used by all three call sites; a store without the seam refuses to conclude a governed attempt with a typed error naming the missing seam (never silent exhaustion); the read-only constructors (weight, watch, counselor, report, budget's own read) stay bare because they never conclude. Tests exercise the two additional production paths. |
| DCS-R3-LATE-START-INSTANT | accepted | Verified: dispatch.sh spawns the runtime (:812-820) and samples `proven_at` afterwards (:829), so `provenAt` trails the process start by up to a second and a job can be charged one minute less than it ran at a minute boundary; `startedAt` is stamped by record creation (build.go:423, :635), which precedes the spawn, and is immutable and in the same wall-clock domain. | Revision 4: the start instant is `startedAt` — the earlier stamp, so the measured interval bounds the runtime from above by the record-creation-to-spawn gap (seconds), never below; `ownershipProof.provenAt` is no longer read by the settlement; the never-launched proof stays "no process identity"; T12 becomes the startedAt-versus-provenAt case (charge follows startedAt); the specimen T9 is recomputed. This also removes the fallback branch entirely. |

Trajectory 5 -> 3 -> 3 with shape-level findings after the failsafe: the
principled exit is not available; prose reopens for exactly these
three. Per the design-critique skill the next critique is a focused
follow-up that enumerates the open ids on a fresh three-round budget of
the same chain. The lane for the fold (pure design lane, or the joint
round Wido granted twice on this pattern) is his word; the seat
proceeds on the pure lane meanwhile.
