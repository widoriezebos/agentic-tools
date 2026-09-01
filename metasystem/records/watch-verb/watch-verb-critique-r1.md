# Watch-verb design — Sol critique round 1, with seat dispositions

Critic: codex gpt-5.6-sol (job watch-verb-critique-r1b) against the
landed design (sha256 2476c71e, landed 622d3f9c). 13 findings, ALL
material (5 critical). Every finding ACCEPTED and binding on fold round
2. The stop criterion governs: the loop continues only while material
findings exist.

## WV-R1B-01 [critical]

The W-IDLE watch class can dispatch over slow but live work and bypass the required marking-mode trial. Section 4 maps the shipped `stalled-idle` verdict to automatic revival, while the shipped evaluator defines that verdict as a live worker lacking recent progress, returns notification only, and states that a live holder is never displaced. Section 7 incorrectly labels this changed mapping as already authorized. Remedy sketch: preserve `stalled-idle` as notification-only, bind revival to `stalled-dead` with proven absence of a live worker, and trial any broader trigger.

Evidence: The conflicting design rules are in metasystem/records/watch-verb/watch-verb-design.md:271-274 and :387-394. The authoritative meanings and actions are in metasystem/internal/steward/verdict.go:15-19 and :99-132. An implementer following the design table rather than the shipped distinction would build the opposite outcome for a slow live round.

DISPOSITION: ACCEPTED. Folds into revision round 2.

## WV-R1B-02 [critical]

The once-per-operation ceiling does not stop the watcher from acting on its own recovery dispatch. The design applies the ceiling to operation identity, but every follow-up round receives a distinct operation identifier because its direct parent and brief participate in the identifier. A dead recovery child is therefore the first death of its new identity and remains eligible for another recovery, contradicting the claimed mandatory escalation on watcher-started work. The current intent record also has no recovery-root lineage field. Remedy sketch: define and enforce one immutable recovery-root identity across the original operation and all watcher-created descendants before minting any recovery intent.

Evidence: The design's ceiling and actor-boundary claim are at metasystem/records/watch-verb/watch-verb-design.md:274-275 and :325-331. The distinct-follow-up rule is explicit at metasystem/internal/dispatch/operation.go:10-37, while metasystem/internal/dispatch/budget.go:293-365 treats distinct operation identifiers as separate operations. The existing intent shape at metasystem/internal/steward/intervene.go:18-41 records neither source operation nor recovery lineage.

DISPOSITION: ACCEPTED. Folds into revision round 2.

## WV-R1B-03 [critical]

W-HEAL has neither the claimed authority nor a working recovery recipe, and its trigger cannot distinguish the cited wedge from unrelated census failures. Ruling R-37-m3 authorizes re-arm only when a landed change requires an engine rebuild; it does not authorize arbitrary owner shutdown and re-arm. The cited incident record explicitly says re-arm did not recover the identity drift and required hand-editing the owner record. Health additionally collapses every non-success census into the same dead verdict, discarding the claimed epoch/start-ticks/boot-identifier signature. An implementation could therefore restart supervision on an unrelated census error and still fail the named incident. Remedy sketch: gap-stop W-HEAL until a typed incident discriminator, a mechanically successful recipe, and matching human authority exist.

Evidence: The design's recipe and authority claims are at metasystem/records/watch-verb/watch-verb-design.md:119-123, :135-136, :277, and :354-356. The actual ruling is metasystem/memory/rulings.md:64. The incident says `arm-supervision --rearm` did not recover and a hand edit was used at metasystem/plans/goals/vm-epoch-identity-drift.md:4-6. The loss of cause information occurs at metasystem/internal/steward/health.go:529-568.

DISPOSITION: ACCEPTED. Folds into revision round 2.

## WV-R1B-04 [critical]

The watchman's own repair failure can die silently. The supervision watcher calls the enrolled-runner repair, but on failure it only records a failed component attempt and returns an error to a loop that logs and continues. The dead steward runner is the component that computes health, updates alert episodes, queues incidents, and delivers pending notifications. Ring 3 recovery also returns failures rather than independently alerting. Thus runner death plus watcher repair failure has no live owner able to produce or deliver the promised escalation. Remedy sketch: require a separately executing, externally visible failure path for unsuccessful runner repair and prove it without the steward runner.

Evidence: The design claims detection, healing, and escalation at metasystem/records/watch-verb/watch-verb-design.md:422-435. The watcher repair callback and error-only outcome are at metasystem/cmd/metasystem/supervise_component.go:181-218, and the component loop's log-and-continue behavior is at :23-35. The runner owns ticking and notification delivery at metasystem/internal/steward/runner.go:62-141. Ring 3 merely returns partial failure at metasystem/internal/up/up.go:502-546.

DISPOSITION: ACCEPTED. Folds into revision round 2.

## WV-R1B-05 [critical]

Section 7 does not identify a Law 2 authorization record or enforcement boundary capable of granting the proposed actions. It names five governance-document fields, Wido's word, and a dated ruling row, but the shipped governance mechanism requires a complete authorization tuple including operation, authorized effects, reviewer policy, and review outcome, checked at the base action boundary. The design neither maps each watch class to those effects nor names the dispatch, shutdown, or re-arm seam that must refuse an incomplete tuple. An implementer must invent whether the prose record itself grants power or how the existing consequence gate applies. Remedy sketch: bind each class to the complete shipped authorization tuple and name the exact refusing base-action seam for every effect.

Evidence: The proposed graduation record is at metasystem/records/watch-verb/watch-verb-design.md:383-416. Law 2's state and authorization shape is described at metasystem/plans/seat-governance-record.md:184-195. The concrete authorization fields and consequence decision are at metasystem/internal/governance/types.go:144-148 and :162-235. The omission can make action reachable on an R-row or prose record that is not a complete executable authorization.

DISPOSITION: ACCEPTED. Folds into revision round 2.

## WV-R1B-06 [high]

The total terminal-cause taxonomy assumes typed evidence that the substrate does not record. Job records expose status and free-form error text, nonzero adapter exits become generic `runtime_error`, and outage marks are repository-level health hints that explicitly authorize nothing and do not identify a job. Native budget cap, returned failure, process loss, authentication, capacity, and network failure therefore cannot be reliably separated into the design's typed set. An implementer must parse prose, misroute cases to escalation, or invent a new schema despite the design calling the substrate landed. Remedy sketch: require a durable adapter-to-job terminal-cause field with exhaustively specified producers before enabling W-ADOPT or W-REDISPATCH.

Evidence: The nonexistent typed set is asserted at metasystem/records/watch-verb/watch-verb-design.md:261-269, then delegated to the implementer at :518-524. The record has only `ErrorText` at metasystem/internal/dispatch/jobrecord.go:17-101. A nonzero command exit becomes generic runtime error at metasystem/internal/adapter/adjudicate.go:195-203. The outage schema and its non-authorizing boundary are at metasystem/internal/outage/outage.go:1-15, :42-49, and :175-261. The reaper's causes are written as strings at metasystem/scripts/agents/dispatch.sh:1065-1077.

DISPOSITION: ACCEPTED. Folds into revision round 2.

## WV-R1B-07 [high]

A non-empty worktree diff is not proof of an intact recovered product, and an empty diff is not proof that no product exists. The cited cap-death record says complete products were recovered from the output stream or the worktree. A stream-only product with an empty diff will be re-dispatched, while a partial, unrelated, or pre-existing dirty diff will be sent to adoption as though intact. Both branches can duplicate or overwrite live recoverable work. Remedy sketch: require a recorded product manifest or recovery receipt that proves product identity and completeness; treat mere diff presence as unknown.

Evidence: The design selects adoption versus re-dispatch solely by base-to-worktree diff at metasystem/records/watch-verb/watch-verb-design.md:274-275 and even uses a merely dirty worktree as its known-bad fixture at :402-408. The cited incident states that whole products were recovered from the stream or worktree at metasystem/plans/goals/budget-death-on-return.md:4-6. Those facts directly refute both sides of the proposed binary test.

DISPOSITION: ACCEPTED. Folds into revision round 2.

## WV-R1B-08 [high]

The marking-mode graduation test has no specified independent adjudicator or durable truth record for false alarms. A would-act record only captures what the watcher proposed; determining later that a target was in fact alive, healthy, or already recovered requires a time boundary, authoritative evidence source, and actor permitted to label the sample. None is named. The watcher cannot judge its own proposed actions under the stated actor boundary, so an implementer can count unadjudicated samples toward authority or can build a trial that never graduates. Remedy sketch: name the independent label owner, evidence snapshot, adjudication deadline, and durable sample outcome consumed by graduation.

Evidence: The zero-false-alarm requirement and reset rule are at metasystem/records/watch-verb/watch-verb-design.md:395-401, while the design prohibits the watcher from adjudicating or certifying at :378-381. Existing health verdicts expose role, status, reason, and remedy but no trial-sample adjudication at metasystem/internal/steward/health.go:78-89. The missing owner and record shape leave a genuine authority-changing implementation choice.

DISPOSITION: ACCEPTED. Folds into revision round 2.

## WV-R1B-09 [high]

The zero-write read surface cannot join durable per-job watcher verdicts because those verdicts are not durable. The scan-jobs implementation writes only a seen-identifier set; its DONE, STALE, CAPPED, NEVER-STARTED, and VANISHED verdicts are emitted to standard output, not stored. Its running-job history is a temporary file deleted when the shell watcher exits. A restarted watcher therefore loses the predecessor state needed to detect VANISHED jobs, and a truly zero-write `metasystem watch` cannot reconstruct past verdicts from the promised stores. Remedy sketch: either name a durable producer and schema for every job verdict or remove those verdict and history promises from the zero-write projection.

Evidence: The design promises a zero-write join of already persisted verdicts at metasystem/records/watch-verb/watch-verb-design.md:156-170 and claims jobs already speak the watcher vocabulary at :242-252. The scan state stores only seen identifiers at metasystem/internal/report/scanjobs.go:29-40 and :100-121; verdicts are output-only at :142-251. VANISHED depends on prior running state at :258-281, while metasystem/scripts/watch-background-jobs.sh:178-197 creates that state temporarily and deletes it on exit.

DISPOSITION: ACCEPTED. Folds into revision round 2.

## WV-R1B-10 [high]

The existing alert-episode store cannot be the durable action history the design promises. Its schema records a finding digest, message, timestamps, and delivery attempts, but no typed response class, target operation, proposed action, executed action, receipt, or recovery outcome. Updating episodes consumes one aggregate health verdict; a healthy pass clears all open findings, while a changed digest retires the old episode and opens another. It therefore cannot independently track multiple job recoveries or prove which action closed which incident. Remedy sketch: specify a separate typed action/episode record or extend the episode key and lifecycle with target, response, receipt, and independent closure evidence.

Evidence: The promised typed escalation and action history is at metasystem/records/watch-verb/watch-verb-design.md:138-144, :158-173, and :315-317. The actual episode schema is at metasystem/internal/steward/alert_episode.go:47-64. Its aggregate update and clear-or-replace lifecycle is at :227-362. An implementer cannot satisfy the promised history using that shape without making an unrecorded schema and lifecycle decision.

DISPOSITION: ACCEPTED. Folds into revision round 2.

## WV-R1B-11 [medium]

W-BREACH falsely equates a delivery-burned verdict with the output of `FindBreachStops`. Delivery can declare an individual unrecovered job burned after its own cap deadline even while the goal remains inside its elapsed-limit grace band. `FindBreachStops` lists only a live goal elapsed breach, corrupt over-limit state, an existing fence, or indeterminate evidence, and `EnsureBreachStop` rechecks that predicate. A literal implementation will refuse to stop the healthy goal and then treat the refusal as a mandatory failed response; an implementation that broadens the stop predicate would exceed Ruling Q. Remedy sketch: make W-BREACH trigger only on the exact typed breach-route result and classify job-cap delivery death separately.

Evidence: The false equivalence and day-one authority claim are at metasystem/records/watch-verb/watch-verb-design.md:276 and :387-394. Per-job burn timing is computed at metasystem/internal/steward/delivery.go:105-134. The live-stop predicates and mandatory recheck are at metasystem/internal/dispatch/stop.go:79-87, :124-139, and :269-335. The current guard prevents an actual healthy-claim stop, but not the design's wrong failed-response escalation or an implementer's temptation to weaken the guard.

DISPOSITION: ACCEPTED. Folds into revision round 2.

## WV-R1B-12 [medium]

The public command shape contradicts the shipped command tree. The design says the landed single-job behavior is `metasystem watch --job` and makes `metasystem watch` its new top-level projection, but the current command is nested as `metasystem job watch`. The design does not say whether to move it, alias it, retain both, or preserve scripts and exit behavior during migration. Remedy sketch: specify the canonical route and compatibility behavior explicitly.

Evidence: The conflicting design statements are at metasystem/records/watch-verb/watch-verb-design.md:98-100 and :175-187. The command registration is `watch` under the `job` group at metasystem/cmd/metasystem/main.go:99-156. Different implementers can build observably different command interfaces from the design.

DISPOSITION: ACCEPTED. Folds into revision round 2.

## WV-R1B-13 [medium]

The read surface requires a reaper health-role verdict that does not exist. The health pipeline enumerates fourteen roles, including the repository watcher but no reaper. The design does not choose between adding a fifteenth public role, deriving reaper state directly from supervision records, or folding it into another role. That choice changes the JSON vocabulary, failure aggregation, breaker behavior, and watchman coverage. Remedy sketch: name the authoritative reaper-health producer and its exact published role or remove the health-role claim.

Evidence: The design requires owner, watcher, reaper, and steward-runner health-role verdicts at metasystem/records/watch-verb/watch-verb-design.md:209-212. The complete shipped role constants and order at metasystem/internal/steward/health.go:43-76 contain no reaper role. The supervision component exists separately at metasystem/cmd/metasystem/supervise_component.go:23-35, so its absence is not merely a naming typo.

DISPOSITION: ACCEPTED. Folds into revision round 2.
