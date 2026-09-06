# Design critique register: seat-mutual-awareness, round 2 (job sma-crit2-20260906)

Design under review: metasystem/plans/seat-mutual-awareness-design.md revision 2 at commit 2e109816 (SHA-256 809d8e3213f16f223bf306f3a10c43dc33f63f9e0f5f4ea25939bb7fda808abd). Reviewed commit: dd387a4dcda00c05b671bf09090a51b60d70dbf4. Critic runtime: codex, model {'effective': 'gpt-5.6-sol', 'requested': 'gpt-5.6-sol'}. Material findings: 10 of 10. The critic ran read-only and could not write this record itself; the seat rendered it from the critic's return (artifacts/agents/sma-crit2-20260906/rounds/1/return.json) without editing the words.

## Gaps the critic declared

- The task describes logical design-review round 2, but the generated runtime notice identifies this new job as runtime round 1 rather than a follow-up on the round-1 job. The return uses the observed runtime round number and cannot prove that both logical reviews were accounted on one critic chain.

- No runtime session identifier was exposed; sessionId is reported as unobserved.

- The design is not implemented, so no proposed seat fixture could be executed. Behavioral proof judgments are grounded in the written fixture contracts and the shipped transaction, guard, recovery, and runner code.

## Findings

### SMA-C-11 (material: True, severity: high)

Claim: SMA-C-11, the enable marker is not fail-closed against an old checkout. Only upgraded seat verbs inspect the marker; an old checkout still classifies plans/seats as an ordinary plan record, its pre-commit expression ignores the directory, and ordinary landing admits a new record. This remains true before and after enablement. Worse, an old landing can create a well-formed enabled.json whose by value merely starts with human:, causing upgraded writers to accept enablement without the human's act. An implementer following this design would ship the original mixed-version write hole behind a marker the old writer cannot see.

Evidence: metasystem/plans/seat-mutual-awareness-design.md:81-90 traces the old path correctly, but lines 180-188 fence only the new writers. Lines 652-668 expressly admit that an old checkout can still spoil the directory, and lines 777-782 make the mixed-version fixture prove that spoilage rather than prevention. The proposed at-rest check at lines 620-630 validates only shape and the human: prefix. The shipped causes remain at metasystem/scripts/agents/pre-commit-guard.sh:70-85, metasystem/scripts/agents/path-classes.txt:28-35, and metasystem/internal/landing/observe.go:672-693.

### SMA-C-12 (material: True, severity: high)

Claim: SMA-C-12, the named escape from a spoiled ledger tip is not an executable recovery path. The design says a terminal hand commit removes the bad file, but the proposed pre-commit expression refuses every staged plans/seats change, the proposed ledger path class refuses the same change through the commit wrapper, and goal publication refuses an invalid captured tip before it may build a repair. The design names no repair verb and explicitly provides no skip verb, so an upgraded fleet can be halted with no lawful way out specified.

Evidence: The recovery claim is at metasystem/plans/seat-mutual-awareness-design.md:656-660 and 781-782, while the proposed fences are at lines 637-640 and the no-skip decision is at lines 683-686 and 889-890. The shipped pre-commit ledger check is unconditional at metasystem/scripts/agents/pre-commit-guard.sh:70-85; ledger-class landings are refused at metasystem/internal/landing/observe.go:583-587; and every goal transaction rejects an invalid captured tip before mutation at metasystem/internal/goal/txn.go:595-612.

### SMA-C-13 (material: True, severity: high)

Claim: SMA-C-13, the enable operation cannot be implemented in the declared rollout order or from its declared command interface. The fleet order requires the human to run seat enable immediately after slice 1, but slice 1 explicitly contains no verb and seat enable is assigned to slice 2. In addition, enabled.json requires the human's name while the command accepts only --repo, and the cited terminal classifier proves only the class HUMAN; it supplies no person's name. The implementer must guess both which slice owns the command and how its required by field is obtained.

Evidence: metasystem/plans/seat-mutual-awareness-design.md:172-183 defines the required by field and the flagless human command. Lines 642-651 require enablement between slice 1 and writer startup, while lines 847-859 put the enable verb in slice 2 after saying slice 1 has no verb. The cited classifier returns class and process coordinates but no human name at metasystem/internal/lease/verbs.go:243-253, and requireHumanStewardEnrollment consumes only that class at metasystem/cmd/metasystem/steward_verbs.go:656-671.

### SMA-C-14 (material: True, severity: high)

Claim: SMA-C-14, recovery is still not total over the design's seat writers. Seat enable creates the gate record and seat retire mutates membership, yet neither operation appears in the journal-intent table, the request reconstruction cases, the named absent-commit outcomes, or the recovery fixtures. A transport-unknown enable or retirement therefore leaves an implementer choosing between replaying a human act from journal text, rejecting and losing it, or inventing a special rerun path.

Evidence: The two writers are introduced at metasystem/plans/seat-mutual-awareness-design.md:172-188 and 200-215. Section 7 claims every seat publication is covered at lines 558-566 but its exhaustive table at lines 566-572 lists only member, presence, ask, answer, and close. The recovery fixtures at lines 789-797 omit enable and retire. In shipped recovery, an unrecognized verb terminalizes as not rebuildable at metasystem/internal/goal/recover.go:185-199 and 202-403.

### SMA-C-15 (material: True, severity: high)

Claim: SMA-C-15, the deadline is not the single durable outcome boundary the design claims. Only a running seat wait writes expiration; without that process an unanswered record remains open past its deadline. Even with a waiter, the target may close not-mine after the deadline, or the asker may withdraw after it, because both transitions are allowed at any time. Those outcomes can win the push race against expiration, contradicting the promise that a non-answer becomes expired at deadlineAt. Equality itself is clear—an answer timestamp equal to deadlineAt is on time—but the remaining cases do not have the promised single outcome.

Evidence: metasystem/plans/seat-mutual-awareness-design.md:406-410 allows not-mine and withdrawal from open at any time and gives only seat wait ownership of expiration. Lines 428-434 nevertheless promise expiration at the deadline, including for a target that never runs. Lines 456-469 make that durable write contingent on a running waiter. RunSeatAttention at lines 476-482 does not expire overdue open records, so no background owner closes the gap.

### SMA-C-16 (material: True, severity: high)

Claim: SMA-C-16, a routine membership refresh can undo human retirement because the transition is unspecified. The membership writer refreshes an existing record when its engine changes or it is one day old, but its stored intent omits retiredAt and the design says only joinedAt is preserved. It never says that a retired record is immutable or that retiredAt survives refresh. An implementer must choose whether the next up command silently makes a retired machine active again, which changes who may receive questions.

Evidence: The membership schema and human retirement action are at metasystem/plans/seat-mutual-awareness-design.md:193-215. The seat-member intent at lines 203-208 contains machine, engine, seatSchema, and joinedAt but no retirement value; joinedAt alone is explicitly preserved. The derived standing and ask refusal depend on retiredAt at lines 217-226 and 378-388.

### SMA-C-17 (material: True, severity: high)

Claim: SMA-C-17, the lineage answer stops at a package seam and still does not reach the process that writes the runner record. The design says steward.Arm writes armedLineage into RunnerRecord, but the shipped detached RunLoop writes runner.json, and launchRunner passes only --repo to that child. The design specifies no argument or durable handoff between them. It also makes terminal steward arm read lease.OwnerLineage even though fleet joining arms the steward before up and the lease loader explicitly permits the lease to be absent only to callers that do not require it. The implementer still has to invent both the transport and the no-lease meaning of armedLineage.

Evidence: The proposed lineage route is at metasystem/plans/seat-mutual-awareness-design.md:291-307. The actual record writer is metasystem/internal/steward/runner.go:70-95, while the launcher passes no lineage at lines 629-656. Fresh fleet joining runs steward arm before up at metasystem/plans/fleet-join-bootstrap-design.md:234-263. An absent required lease is an error at metasystem/internal/lease/lease.go:71-83.

### SMA-C-18 (material: True, severity: medium)

Claim: SMA-C-18, the at-rest validator permits a future presence timestamp that the record schema forbids. The schema requires updatedAt to equal tickAt, but the refusal table rejects only updatedAt earlier than tickAt. A hand-written record with updatedAt later than tickAt therefore validates and remains reachable longer than the declared staleness window. The implementer must choose between the schema and the purportedly total refusal table.

Evidence: metasystem/plans/seat-mutual-awareness-design.md:252-254 requires equality. The staleness decision consumes updatedAt at lines 285-289. The total-validator row at lines 629-635 refuses only the earlier-than relation and declares every record violating none of its rows legal.

### SMA-C-19 (material: True, severity: high)

Claim: SMA-C-19, the proof matrix still does not certify the riskiest new behavior. It has no assertion that an old checkout cannot forge a valid enable marker, no real guarded recovery from a spoiled tip, no recovery case for enable or retire, no retire-then-up case, no waiter-absent expiration case, no post-deadline not-mine race, and no answer landing in the exact deadline second. The mixed-version fixture instead proves that the old writer can spoil the tip, then assumes an unspecified hand repair. All named tests can pass while the preceding contract failures remain.

Evidence: The fixtures are at metasystem/plans/seat-mutual-awareness-design.md:721-816 and the unit tests at lines 818-841. SMA-F-MIXED at lines 777-782 proves old-engine spoilage and assumes hand removal. The timeout fixture at lines 757-761 covers only wait-driven expiry followed by a late textual answer. Recovery cases at lines 789-797 omit enable and retire, and none of the listed tests names the exact-equality, post-deadline-close, retirement-refresh, or forged-marker cases.

### SMA-C-20 (material: True, severity: high)

Claim: SMA-C-20, the requested implementation box is not honest for the work listed. The four slices allow as many as nine build job reservations, and the design then requires two separate code-review chains, for at least eleven reservations. The requested limit is ten, so the last review or build retry can be refused even if every estimate holds. The request also calls three review rounds unchanged although the current goal record allows two, and it omits the active-job member required by the complete budget tuple.

Evidence: metasystem/plans/seat-mutual-awareness-design.md:845-875 totals six to nine build attempts and then adds two separate review chains; lines 876-880 request ten attempts and call three review rounds unchanged. metasystem/docs/backlog-mechanism.md:17-30 defines an attempt as an admitted job reservation and requires the complete budget tuple. The synchronized goal actually records ten attempts, 720 reserved job-minutes, one active job, and two review rounds at metasystem/plans/goals/seat-mutual-awareness.md:11.

## Rigor rows

| finding | class | facts | reopening trigger |
|---|---|---|---|
| SMA-C-11 | severe | {"authorityBoundaryCrossed": true, "externalSideEffectBoundaryCrossed": true, "irreversibleDataBoundaryCrossed": false, "local": false, "proofBoundaryCrossed": false, "recoverable": true, "secretsBoundaryCrossed": false} | Reopen when an executable mixed-version fixture proves that no old checkout can create any seat record or a valid enable marker before or after human enablement. |
| SMA-C-12 | severe | {"authorityBoundaryCrossed": false, "externalSideEffectBoundaryCrossed": true, "irreversibleDataBoundaryCrossed": false, "local": false, "proofBoundaryCrossed": false, "recoverable": false, "secretsBoundaryCrossed": false} | Reopen when the design names one lawful command that can build and publish a repair over an invalid seat-bearing tip despite the proposed guard, path class, and captured-tip validation. |
| SMA-C-13 | severe | {"authorityBoundaryCrossed": true, "externalSideEffectBoundaryCrossed": true, "irreversibleDataBoundaryCrossed": false, "local": false, "proofBoundaryCrossed": false, "recoverable": true, "secretsBoundaryCrossed": false} | Reopen when the enable verb exists in the slice that the rollout invokes and its complete command interface deterministically supplies the required human identity. |
| SMA-C-14 | severe | {"authorityBoundaryCrossed": true, "externalSideEffectBoundaryCrossed": true, "irreversibleDataBoundaryCrossed": false, "local": false, "proofBoundaryCrossed": false, "recoverable": false, "secretsBoundaryCrossed": false} | Reopen when enable and retire each have a complete journal intent, a named confirmed and absent-postcondition recovery result, and fixtures covering both outcomes without reconstructing human authority from text. |
| SMA-C-15 | severe | {"authorityBoundaryCrossed": false, "externalSideEffectBoundaryCrossed": true, "irreversibleDataBoundaryCrossed": false, "local": false, "proofBoundaryCrossed": false, "recoverable": false, "secretsBoundaryCrossed": false} | Reopen when every overdue open ask has one durable owner that expires it, all post-deadline transitions are defined consistently, and races at, before, and after the exact deadline are tested. |
| SMA-C-16 | severe | {"authorityBoundaryCrossed": true, "externalSideEffectBoundaryCrossed": true, "irreversibleDataBoundaryCrossed": false, "local": false, "proofBoundaryCrossed": false, "recoverable": true, "secretsBoundaryCrossed": false} | Reopen when the membership transition contract and fixture prove whether up preserves retirement or requires a named human reactivation before clearing it. |
| SMA-C-17 | severe | {"authorityBoundaryCrossed": false, "externalSideEffectBoundaryCrossed": true, "irreversibleDataBoundaryCrossed": false, "local": false, "proofBoundaryCrossed": false, "recoverable": true, "secretsBoundaryCrossed": false} | Reopen when the design names the complete lineage handoff into the detached runner record and defines the value used when terminal arming occurs before a lease exists. |
| SMA-C-18 | severe | {"authorityBoundaryCrossed": false, "externalSideEffectBoundaryCrossed": true, "irreversibleDataBoundaryCrossed": false, "local": false, "proofBoundaryCrossed": false, "recoverable": true, "secretsBoundaryCrossed": false} | Reopen when the schema and refusal table agree on exact presence timestamp equality and a malformed-tree test rejects a future updatedAt value. |
| SMA-C-19 | severe | {"authorityBoundaryCrossed": false, "externalSideEffectBoundaryCrossed": false, "irreversibleDataBoundaryCrossed": false, "local": true, "proofBoundaryCrossed": true, "recoverable": true, "secretsBoundaryCrossed": false} | Reopen when each named rollout, recovery, retirement, and deadline boundary defect has a one-to-one fixture or unit-test obligation that exercises the actual guard and transaction path. |
| SMA-C-20 | severe | {"authorityBoundaryCrossed": true, "externalSideEffectBoundaryCrossed": false, "irreversibleDataBoundaryCrossed": false, "local": true, "proofBoundaryCrossed": true, "recoverable": true, "secretsBoundaryCrossed": false} | Reopen when the requested five-member budget covers the declared maximum build and review reservations and agrees with the goal's current review-round member. |
