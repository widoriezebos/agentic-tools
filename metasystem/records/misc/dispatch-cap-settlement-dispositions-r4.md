# Dispositions, round 4, and the loop's stop: reservation settlement design critique

Design revision 4 (job cap-settle-design-r4). Critic chain
cap-settle-crit, round 4 (job cap-settle-crit-r4, gpt-5.6-sol). Earlier
rounds: plans/dispatch-cap-settlement-dispositions.md, -r2.md, -r3.md.
Fold verification: the critic confirmed all three round-3 findings
folded.

## Round 4 — 3 material findings, verdictMaterialCount=3

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| DCS-R4-STARTEDAT-UNBOUNDED-OVERCOUNT | accepted | Verified against the launcher: the record carrying `startedAt` is created at dispatch.sh:1533-1554, setup and lease-mediated steps follow (:1602-1610), and the runtime is launched later (:2318-2333) with no bound between them; a pause there makes a short job settle to many minutes or its cap — the bug's own direction. `ownershipProof.provenAt` is sampled right after the spawn (:829) and trails the process start by under a second, so its error is a bounded UNDER-count of at most one minute at a minute boundary. | The build uses `ownershipProof.provenAt` as the start instant (fallback `startedAt` only when the proof is absent; a present but unparseable proof is unknownBudget), exactly revision 3's rule 1.3; the bounded under-count is recorded beside KI-45. This is the orchestrator's recorded deviation from revision 4's text, risen to Wido in plans/dispatch-cap-settlement-scope-cut.md. |
| DCS-R4-EXCLUSION-HIDES-DUPLICATE-OWNER | out-of-scope | OUT OF SCOPE per the review brief plans/dispatch-cap-settlement-critique-brief.md, Scope: "OUT: the governed-run settlement path (already settled to observed cost)" and its threat model (accidents in the delegated-job projection, not the governed path); true as a fact (budget.go:395-400 validates duplicate durable owners and the exclusion as written would bypass it), and it lives entirely in the conclusion-time re-projection seam (design section 4.3), which is cut from this goal and tokened as goal governed-exhaustion-reprojection with this finding recorded on it. | none here; carried on the token. |
| DCS-R4-T13-POST-DEBT-RETRY | out-of-scope | OUT OF SCOPE per the review brief plans/dispatch-cap-settlement-critique-brief.md, Scope: "OUT: the governed-run settlement path (already settled to observed cost)" and its threat model (accidents in the delegated-job projection, not the governed path); true as a fact (conclude.go:238-248 and :342-356 raise retroactive debt after the terminal run write, so T13(e) as specified cannot pass), and it is a fixture of the same cut seam. Carried on goal governed-exhaustion-reprojection. | none here; carried on the token. |

## The loop stops here

Trajectory 5 -> 3 -> 3 -> 3. Rounds 3 and 4 found defects only in
machinery rounds 2 and 3 had added (the re-projection seam, its
constructor, the sweep ladder, the start-instant churn): the loop was
critiquing itself, which the design-critique skill names as the stop
("when roughly half of a round's material findings were introduced by
the previous round's own edits, stop there, record the judgment, and
let implementation be the next source of truth"). The orchestrator
should have stopped at round 3 and did not; Wido called it. Judgment
recorded: the design is built on its box (the charge rule with the
proof-time start instant, the two projection fields, the reserved line
on every refusal, the end-stamp refusal in RecordCAS, the discharge
filter, the tests of those), the two neighbouring defects are tokened
(goals governed-exhaustion-reprojection and lease-sweep-death-evidence),
and the code critique is the arbiter. Round 4's rounds are retained
verbatim so the loop can resume if implementation proves the stop
premature.
