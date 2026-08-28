1. **CRITICAL — The proposed regression fixture is impossible under the existing trigger order.** With budget 3, three critic-only concluded cycles make `stagnant == 3`, so the mission parks before cycle 4 can dispatch or land an implementer. A cycle-4 marker cannot retroactively prevent that park. Passing the proposed fixture would require an undocumented change to when the fuse fires.

2. **CRITICAL — This silently changes already-sealed missions.** `ledgerSemantics: 2` freezes “new best or human reset” as the reset grammar. Teaching semantics 2 to count work markers violates the explicit promise that a sealed budget’s meaning never changes mid-mission. This requires a new semantics version, initialization rule, migration policy, and rollback behavior.

3. **CRITICAL — “Laundering is impossible” is false.** Uniqueness prevents reusing one job ID; it does not prevent creating unlimited trivial implementer jobs. A host can dispatch one tiny, easily certified job whenever the fuse approaches its limit and reset stagnation indefinitely until the cycle fence. Certification proves whatever the delegation floor certifies—not material progress toward mission acceptance.

4. **CRITICAL — The added line breaks the conclusion-as-transaction invariant.** The contract says the append concluding a cycle is the transaction, with no second-write crash window. A separate `Work landed:` line creates bad prefixes: measurement committed without credit, or credit committed without a concluded measurement. Flock serialization does not provide atomicity across writes. The evidence must be encoded in the existing conclusion record or both lines must be one explicitly guaranteed atomic append.

5. **HIGH — Replay cannot verify the claimed eligibility while remaining pure over the ledger.** Role, completion, and certification live in job records, but replay sees only an asserted job ID. Either the marker is trusted—making the advertised eligibility checks unenforceable during replay—or replay reads mutable external records and violates C1. The ledger needs an authoritative attestation grammar and a proof that only the runner can emit it after validating immutable evidence.

6. **HIGH — “Completed with certification in that cycle” has no deterministic definition.** Jobs can span cycles, certification can arrive after conclusion, and reaping is timing-dependent. It is unspecified whether ownership follows completion time, certification time, first observation, or the cycle that dispatched the job. Boundary races can cause a job to be missed or credited in different cycles across runs.

7. **HIGH — “Work landed” is materially stronger than the predicate implemented.** A completed, certified implementer may produce an empty patch, an off-target artifact, duplicated work, an unmerged commit, or output later rejected by the main session. None is necessarily “landed work.” A qualifying event needs a non-empty, mission-scoped, accepted artifact identity—such as an accepted commit/content hash—not merely job status.

8. **HIGH — The contract amendment is substantially under-scoped.** C1 currently declares new best as the only automatic reset; invariant 1 names only new-best and vocal reset; C3 says no other reset path exists; failure behavior defines unavailable measurement as incrementing. Amending only “C1’s gain grammar” leaves the normative contract contradictory.

9. **HIGH — The replay algorithm is not specified precisely enough to be deterministic.** It does not define global versus per-cycle job-ID deduplication, the ID namespace, behavior for mixed duplicate/new IDs, multiple markers in one cycle, malformed lists, unknown IDs, or a marker attached to no valid conclusion. “Once per distinct jobId” is not executable semantics.

10. **HIGH — Forward and rollback compatibility are missing.** An older binary may reject or ignore the new line and derive a different stop-loss verdict; a newer binary may reinterpret the same semantics-2 ledger. “Additive grammar” and non-anchored shell greps do not prove compatibility. A binary/ledger-semantics matrix is required.

11. **MEDIUM — The cycle association is structurally ambiguous.** “Next to” a measurement line does not bind a marker to that cycle in an append-only grammar, especially around unresolved cycles, retries, resets, and crash recovery. The cycle identifier or conclusion record must carry the evidence directly.

12. **MEDIUM — The proposed tests omit the dangerous cases.** Missing obligations include every crash prefix around conclusion and marker writes, old/new binary replay, malformed and forged markers, cross-cycle completion races, trivial-job farming, empty or duplicate artifacts, marker append failure, ID reuse, and cycle-budget interaction.

13. **MEDIUM — No bounded-cost argument is supplied.** One full reset per qualifying job can suppress the no-gain fuse for the entire cycle allowance while acceptance remains unchanged. Keeping the cycle fence does not establish that the no-gain fuse still performs its intended independent cost-control function.

14. **MEDIUM — The evidence does not isolate the proposed mechanism.** The successful cohort used parallel dispatch and landed metric-moving work, while the failing cohorts never dispatched an implementer before the fuse. That demonstrates a scheduling/budget-order defect, not that completed-job credits are the correct remedy. The existing budget increase to 5 may mask the issue, but no experiment shows this new reset source is necessary or sufficient.

REVISE: The proposal cannot rescue the cited budget-3 serialized path and introduces an unbounded, non-atomic automatic reset that violates sealed semantics and is readily farmed with trivial certified jobs.
