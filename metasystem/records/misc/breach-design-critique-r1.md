# Sol design-critique of the breach-machinery design — round 1 (2026-09-01)

Eight material findings, folded into the critique register (chain
breach-design-crit). Landed verbatim so the successor can revise
without re-running the round. The revision brief is these findings
plus: requirement supremacy, no weakened refusals, verify every
closure against the tree, failsafe one revision + one re-critique
then to Wido.

## BCD-R1-001 (high, material)

The advertised parked-with-breach resume transition is unreachable through the shipped goal resume command. The design adds the second state shape only inside the goal package, but the command first calls ResolveGoalBinding, the resolver that rejects anything not currently claimed, and acquires a lock from that claimed revision. A parked-with-breach goal has no claim by design, so the package-level test can pass while the real command always fails.

Evidence: Design lines 290-292 and 322-329 make goal resume the reopening path and test only metasystem/internal/goal/stop_test.go at lines 423-424. Tree metasystem/cmd/metasystem/goalsync_mutations.go:370-384 resolves and locks before calling Resume; metasystem/internal/dispatch/stop.go:53-59 rejects a parked goal before that call.

## BCD-R1-002 (critical, material)

Releasing a stopped claim can strand cancellation permanently. The design allows release without a complete batch, but the existing custodian publishes the fence, re-resolves a claimed binding, and only then creates the batch. Release can park the goal between those events, causing batch creation to fail. If an OPEN batch already exists, the next automatic pass still cannot rediscover it because FindBreachStops skips parked goals. This contradicts the claimed preservation of cancellation duty and can leave jobs running behind a fence that can never satisfy resume automatically.

Evidence: Design lines 296-305 permit immediate demotion, lines 359-362 waive batch completion, and lines 462-465 claim the custodian resumes by stop identifier. Tree metasystem/internal/dispatch/stop.go:153-188 contains the fence-to-batch race; metasystem/internal/dispatch/stop.go:288-313 excludes parked goals from automatic routes; metasystem/internal/steward/tick.go:69-83 depends exclusively on those routes.

## BCD-R1-003 (high, material)

A raise after a lawful consumed discharge does not preserve the pre-raise elapsed start; it rewinds the clock to the original episode. The proposed rebind clears the obligation and changes the claim revision while explicitly leaving proof revision filters unchanged. The prior consumed proof therefore becomes ineligible, and obligationBudgetStart returns EpisodeAt instead of the later consumedAt. A raise would alter elapsed accounting and can create an immediate false breach, despite the design's promise that raises preserve the clock.

Evidence: Design lines 156-162 preserve EpisodeAt but clear Obligation, and lines 170-176 leave revision filters unchanged. Tree metasystem/internal/goal/verbs.go:122-124 shows the inherited obligation clearing; metasystem/internal/dispatch/budget.go:77-85 requires an obligation; metasystem/internal/dispatch/budget.go:133-135 accepts only a proof matching the current Claimed.Revision. The proof plan at design lines 366-385 has no raise-after-discharge case.

## BCD-R1-004 (high, material)

Excluding stopped claims only from quota validation creates two simultaneous claimed goals without updating the machinery that decides what this machine is working on. Before release, both the fenced old goal and the new workable goal remain State claimed. The orientation command can select the fenced goal by sort order, while serving and turn-verdict projections select either claim by nondeterministic map iteration. The machine is therefore quota-free but can still be directed back to the stopped work.

Evidence: Design lines 346-350 deliberately allow a second claim before release. Tree metasystem/internal/goal/project.go:93-99 includes every claimed record; metasystem/cmd/metasystem/goal.go:469-472 chooses its first result; metasystem/internal/goal/goalverbs.go:820-823 and metasystem/internal/goal/turnverdict.go:483-490 return the first matching map entry without checking StopFence.

## BCD-R1-005 (high, material)

The duration-era marker is not wired into the stop-batch producer. Adding ElapsedGrammar only to StopFiringEvidence and its validator leaves EnsureBreachStop constructing evidence without the field. A new clock-grammar budget such as 9d would compute a 216-hour boundary, then the absent marker would make validation recompute a 72-hour legacy boundary and reject creation of the stop batch after the fence has closed.

Evidence: Design lines 243-249 specify the evidence field and validation but omit the producer, despite claiming a complete consumer trace at lines 64-76. Tree metasystem/internal/dispatch/stop.go:134-138 is the sole firing-evidence constructor and currently copies only elapsed use, admission token, boundary, and grace. Tree metasystem/internal/goal/stop.go:145-155 recomputes the boundary and rejects disagreement. The proposed test at design lines 403-405 is assigned to the goal package, not the dispatch producer seam.

## BCD-R1-006 (high, material)

The claimed fail-closed mixed-era rule is false for governed-run snapshots and stranded journal entries. An old binary rejects the new Markdown Budget key, but it permissively ignores the new JSON field inside a governed-run budget and then interprets 9d as 72 working hours during conclusion. The old journal replay path likewise ignores an added elapsedGrammar argument and reconstructs through the legacy constructor. Marker presence is deterministic only for new readers, not across the migration and rollback boundary the design claims to guard.

Evidence: Design lines 257-260 include run snapshots in the migration, lines 271-276 claim old binaries cannot silently misread new semantics, and lines 466-469 preserve old governed runs. Tree metasystem/internal/run/run.go:123 embeds the budget, metasystem/internal/run/run.go:374-389 reads it with permissive json.Unmarshal, and metasystem/internal/run/conclude.go:315-318 enforces the decoded duration. Tree metasystem/internal/goal/budget.go:81-99 ignores additional intent arguments and calls the legacy NewBudget constructor.

## BCD-R1-007 (medium, material)

The proposed parked stop-authority invariant breaks the existing ordinary hand-park path and does not implement the promised human hand-done path. An ordinary claimed hand edit retains its generated StopCapability but has no fence; the new complete-pair diagnostic is not among the mapper's tolerated diagnostics. Conversely, a parked-with-breach hand edit to done cannot retain stop authority because done forbids it and cannot delete it because the mapper treats that as generated-field tampering. The design names replay changes but omits the mapper contract that must admit both operations.

Evidence: Design lines 337-340 promise reconcile hand-park and hand-done equivalence, while lines 351-357 introduce the new complete-pair rule. Tree metasystem/internal/goal/reconcilemap.go:131-146 tolerates only the old claimed-to-parked diagnostics; metasystem/internal/goal/reconcilemap.go:229-247 permits stop retention or clearing only when a claim is cleared by park. Existing metasystem/internal/goal/reconcilepub_test.go:295-316 requires ordinary claimed hand-park to remain lawful. The proof plan names only breach hand-park at design lines 429-431, not breach hand-done or the existing ordinary case.

## BCD-R1-008 (high, material)

The operational migration cannot perform its stated grammar reset on every live budgeted goal without violating its own clock rule. The real tree already contains a live fenced budget. SetBudget always refuses that state, while Resume is the only alternative and deliberately starts a fresh episode as an explicit human re-time. Thus the prescribed set-budget migration cannot run for the stopped specimen, and the alternative resets the episode that the migration says must survive.

Evidence: Design lines 262-270 require goal set-budget for every live budgeted goal and say the grammar reset must not reset the episode. Tree metasystem/plans/goals/alert-escalation-channel.md:9-12 contains a live budget and StopFence. Tree metasystem/internal/goal/verbs.go:514-515 refuses SetBudget on that goal. Design lines 107-115 and 322-329 define Resume as a fresh episode or queued reopening, not the promised grammar-only rebind.
