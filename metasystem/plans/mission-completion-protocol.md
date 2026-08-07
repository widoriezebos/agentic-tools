# Mission completion, run identity, and the cohort machine

- Goal and current status: the crash-recoverable protocol for how a mission ends — measurement persistence, chain closure, publication, run identity, and the cohort repetition machine, including the mission abandonment path that does not exist yet. Status: DRAFT seeded from the benchmark-validity chain's round-7 exhaustion; NOT yet critiqued as its own stream.
- Next step: none
- Blocked by: the multi-main coexistence stream, which the human put first on 2026-08-07 (interference before everything); when that closes, a fresh-context session does the design pass over the seven carried findings, then critique from round 1 (sol, critic worktree synced per KI-20)
- In flight right now: nothing
- Waiting on: a fresh working session — seven carried protocol findings deserve full context, not the tail of this one; the V-1/V-4 code-critique and merge complete first. The human's standing ratification of 2026-08-07 covers the closure work

## Where this comes from

`plans/benchmark-validity-closure.md` tried to carry this protocol as two of
its four fixes. Seven critique rounds and 35 findings proved the scope: the
V-2/V-3/V-3a state machines in that design's round-6 form are the STARTING
material for this stream, and the seven round-7 findings below are its
opening requirements — the successor-brief enumeration the exhaustion
contract demands. All seven are open, none dispositioned.

## Carried findings (BV-7, verbatim)

### BV-7-1 (sev=critical)

The completion machine has no branch for a measurement whose gatePassed value is false. It persists every completed gate measurement, then says the presence of measuredOutcome permanently suppresses further host turns and proceeds to closure and completed publication. Because the runner measures after every turn, this specification can complete a mission whose gate failed. The completion protocol must begin only after a passed gate, or define a failed-measurement transition that continues or parks without entering closure-only recovery.

Evidence: V-2 step one says “the gate measurement completes” and persists gatePassed without requiring it to be true. Steps two and three then close chains and publish completed without testing it. The existing mission contract requires completed state to have a passed gate and currently sends false measurements back to running or parked state.

### BV-7-2 (sev=high)

The claim that no crash point can repeat a measurement is false at the first transition. A process can stop after the gate command and guards finish but before measuredOutcome is written. State still lacks the anchor, so resume runs the measurement again. An implementer must either accept and specify at-least-once measurement before persistence, require measurement to be idempotent, or introduce a durable measurement-intent/result protocol; those are materially different contracts.

Evidence: The design places measurement completion before the atomic state write and makes only that later write the recovery discriminator. The gate and guard commands execute outside the state-file transaction, so no atomic file replacement can cover the interval between their completion and persistence.

### BV-7-3 (sev=high)

Mission completion can race the driver's identity copy because the two state machines do not order the execution-identity append before the externally visible completed status. If completed becomes visible first, the cohort driver can copy an execution identity that lacks measuredCandidateSha; if a parked identity was copied earlier, a resume adds another attempt after the copy. The extractor then correctly rejects the disagreement. The durable order must include final execution-identity publication before mission completion, and the driver must copy only from that terminal revision.

Evidence: V-2 publishes completed after chain closure. Separately, V-3 says “completion appends measuredCandidateSha” but does not place that append before publication. The cohort driver reacts to mission status, owns the copy, and the extractor cross-checks both files, making this ordering observable rather than editorial.

### BV-7-4 (sev=high)

The proposed benchmark identity is incompatible with the established schema and comparison tuple, despite the design saying validity logic is unchanged. The design introduces adoptedMetasystemSha, measuringKitSha, attempt data, and measuredCandidateSha, while omitting required benchmarkSpecId, benchmarkSpecVersion, measuringKitVersion, measuringMetasystemSha, and createdAt from its stated final shape. An implementer must either version and change the scorecard and comparison contracts or project the new execution evidence back into the existing fields; the design specifies neither.

Evidence: The current benchmark-identity schema is version one, forbids undeclared properties, and requires twelve established fields. The scorecard and comparison code read measuringKitVersion and measuringMetasystemSha as comparability facts. Neither schema nor comparison recognizes the new field names, and V-3 does not declare a benchmark-identity schema version or projection mapping.

### BV-7-5 (sev=high)

Concurrent recover and finalize commands can both act on the same pending repetition. Recover leaves the repetition in pending-recovery while it resumes and grades the mission, so finalize remains legally enabled and can abandon and close chains underneath the resumed runner. Whichever command writes last can produce an ungradeable repetition with a scorecard or a graded repetition whose mission was abandoned. A lock held across side effects, or an atomic pending-to-recovering claim with explicit winner semantics, is required.

Evidence: Both transitions originate at pending-recovery, and the design names one logical writer but no process lock, compare-and-swap rule, or intermediate recovering state. The current cohort writer performs unguarded read-modify-replace updates, so two driver processes can both read the same predecessor and perform incompatible external actions before either terminal write.

### BV-7-6 (sev=high)

The cohort arrows are labels around non-atomic workflows, not crash-recoverable transitions. For planned to running, writing first can leave a running record without a target, while writing last can leave a planned record beside an already provisioned or running target. For running to graded, a crash after scorecard publication but before the state write leaves running state beside a scorecard that current retry logic refuses to overwrite. Recovery and finalization have the same side-effect-before-state window. The design needs write-ahead substates and reconciliation rules or idempotent replay contracts for each external effect.

Evidence: Provisioning, mission start, identity copy, grading, extraction, scorecard publication, abandonment, and cohort-record replacement are separate durable operations. V-3a assigns each collection of operations to one arrow but defines no ordering or restart discriminator inside an arrow. Atomic replacement protects file integrity only; it does not make those external operations atomic.

### BV-7-7 (sev=high)

Finalization depends on a mission abandonment path that does not exist and has no owner in the design. The current mission runner has no abandon command or abandoned status, and its schema recognizes only running, completed, and parked. Implementing finalize therefore requires inventing a new cross-boundary metasystem interface and state transition, or having the kit close candidate-owned chains directly. Either choice changes ownership, schema, and recovery behavior.

Evidence: V-3a says the kit-owned driver invokes the mission's explicit abandonment path, while the ownership matrix assigns no abandonment work to the metasystem. The current public runner commands are start, resume, status, and answer; searching the runner and mission-state implementation finds no abandonment operation or state.


## Constraints inherited

Parks never close chains; abandonment must be designed (BV-7-7) as the one
explicit chain-closing alternative to completion, with an owner and a
command. The measure-persist-close-publish anchor stands but needs the
gatePassed=false branch (BV-7-1) and idempotent measurement (BV-7-2).
Identity stays two-files-one-writer-each, with the ordering and the
established candidateSha tuple respected (BV-7-3, BV-7-4). Recover and
finalize need mutual exclusion; cohort transitions need crash-recoverable
form (BV-7-5, BV-7-6).
