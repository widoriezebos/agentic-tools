# Watch-verb design — Sol critique round 2, with seat dispositions

Critic: codex gpt-5.6-sol (job watch-verb-critique-r2b) against revision
2 (sha256 b7544394, landed f6e25b42). 9 findings, all material (4
critical); trajectory 13 -> 9. All ACCEPTED. The survivors cluster on
implementer-private seams - the two-bars stall signature - and Wido
granted a second one-round joint exception (R-38-m0) rather than a pure
fold round.

## WV-R2B-01 [critical]

The W-REVIVE and W-RECOVER triggers overlap, so the existing revival path can bypass both the marking-mode trial and the recovery-root ceiling, including after a watcher-created recovery dies.

Evidence: Design section 4.2 declares a total classifier but makes W-REVIVE fire on the steward ladder's ActRevive and W-RECOVER fire on a claimed goal's died delegate at metasystem/records/watch-verb/watch-verb-design.md:458-468. Section 7.3 explicitly leaves W-REVIVE operating during W-RECOVER observation at lines 672-677, while section 5 claims a dead recovery child can only escalate at lines 549-557. In the tree, any machine-owned claimed goal is open work at metasystem/internal/steward/openwork.go:30-62, and the absence of live or uncertain workers produces ActRevive at metasystem/internal/steward/verdict.go:99-132 without reading a failed job, terminal cause, or recovery root. The landed continuation intent has no recoveryRoot at metasystem/internal/steward/intervene.go:18-41. Thus an original died delegate satisfies both classes, and a died W-RECOVER child can cause an ordinary generic revival under the old three-revival ceiling. Which class wins is unspecified; choosing W-REVIVE acts before promotion and can redo recoverable work, while choosing W-RECOVER silently changes the supposedly unchanged W-REVIVE law. Remedy sketch: make the overlap ordering and one shared lineage ceiling explicit at the existing ActRevive base-action boundary before either path may launch.

DISPOSITION: ACCEPTED. Binds the R-38-m0 joint round.

## WV-R2B-02 [critical]

The promised per-class Law 2 authorization has no shipped record location or executable launch check, so an implementer must invent the authority boundary or misuse a target goal's unrelated obligation.

Evidence: Design section 7.4 says W-RECOVER is registered as a GovernedObligation and checked with authorize-governed-launch at intent mint and launch at metasystem/records/watch-verb/watch-verb-design.md:712-751. The actual durable schema has exactly one obligation on each goal at metasystem/internal/goal/file.go:46-48; that obligation must bind the goal's claim and budget revision and carry platform, toolchain, surface, recurrence, timing, assumption, and trigger fields at metasystem/internal/goal/obligation.go:106-166. Existing admission reads that one target-goal obligation and calls Decide only for authorize-spend at metasystem/internal/dispatch/governed.go:80-151. A repository search finds no production call to Decide for EffectAuthorizeLaunch. A fleet-wide W-RECOVER class therefore cannot be stored or promoted through the mechanism the design calls shipped, and goals already using their sole obligation cannot host it. This leaves choices that change authority: overwrite a goal obligation, create an unspecified global registry, or let prose authorize the launch. This is predecessor finding WV-R1B-05's authority hole, not a genuine fold. Remedy sketch: name the class-level store, complete valid schema, authorized writer, loader, and exact launch refusal distinct from every target goal's existing obligation.

DISPOSITION: ACCEPTED. Binds the R-38-m0 joint round.

## WV-R2B-03 [critical]

W-RECOVER can dispatch over a newer live or already-returned recovery and can race a landing because neither its trigger nor its launch boundary rechecks recovery of the goal.

Evidence: The W-RECOVER trigger trusts the delivery failed-without-recovery join at metasystem/records/watch-verb/watch-verb-design.md:461-464. That join selects the newest failed job whose end is after the claim and latest landing receipt, but never compares any newer running, completed, or returned job at metasystem/internal/steward/delivery.go:279-286. A manual retry with a distinct operation identifier can therefore be actively producing or awaiting review while the old failure remains eligible. The design's second launch-time check scans only recoveryRoot records at metasystem/records/watch-verb/watch-verb-design.md:652-668, and its Law 2 check tests only authorization at lines 741-748; unlike the existing revival implementation's full predicate rerun at metasystem/internal/steward/revive.go:105-119, it never requires the delivery join, target status, active-job set, or landing receipt to remain unchanged. A newer job present initially or a landing between ledger mint and intent consumption therefore causes duplicate spend and overlapping work. Remedy sketch: define a typed newer-job suppression rule and atomically rerun the complete W-RECOVER predicate with the goal-revision admission check immediately before launch.

DISPOSITION: ACCEPTED. Binds the R-38-m0 joint round.

## WV-R2B-04 [critical]

The watchman's silent-death fold stops at successful direct delivery; failure of that direct send is still owned only by the watcher whose companion runner is dead.

Evidence: Design section 8 adds steward.Deliver from the supervision watcher after five failed runner repairs and specifies only a fixture where delivery succeeds without a live runner at metasystem/records/watch-verb/watch-verb-design.md:771-794. It does not define whether a failed send advances lastDeliveredAt, how the failure is journaled, or which separately executing owner retries it. Direct Deliver returns an error and writes no durable queue or alert attempt at metasystem/internal/steward/notify.go:38-58; durable retry belongs to DeliverPending at lines 61-98, which the dead runner normally owns. The watcher pass records a generic PASS_FAILED and returns the error at metasystem/cmd/metasystem/supervise_component.go:181-218, after which the component loop logs and continues at lines 131-153. Runner death plus failed repair plus a failed direct transport therefore still has no health-pipeline owner capable of delivering the incident, and different implementers will choose retry-every-pass, suppress-for-60-passes, or no retry. This is predecessor finding WV-R1B-04's silent hole at the next failure boundary. Remedy sketch: specify a durable direct-delivery attempt lifecycle and an independently drained retry path, with a fixture where both the runner and the first direct send fail.

DISPOSITION: ACCEPTED. Binds the R-38-m0 joint round.

## WV-R2B-05 [high]

The budget-cap member of the W-RECOVER died set cannot satisfy its own required delivery trigger, so this dead-work class is silently excluded from automatic recovery.

Evidence: Design sections 4.1 and 4.2 include budget-cap in the died set but require the delivery role's failed-without-recovery join at metasystem/records/watch-verb/watch-verb-design.md:447-464. The reaper terminalizes an expired dead group as status timeout with error budget-cap at metasystem/internal/supervise/reaper.go:191-209. The delivery role considers only completed, failed, and cancelled terminal at metasystem/internal/steward/delivery.go:230-232, and newestUnrecoveredFailure accepts only status failed at lines 279-286. A timeout/budget-cap record is consequently handled as a nonterminal burned-without-delivery case rather than the required failed join. The table says delivery burn is not a breach stop and that an underlying died job is W-RECOVER's business, but the conjunction is impossible. Remedy sketch: add timeout/budget-cap to a typed unrecovered-terminal join and prove that exact record enters W-RECOVER rather than only the health escalation path.

DISPOSITION: ACCEPTED. Binds the R-38-m0 joint round.

## WV-R2B-06 [high]

The landed one-shot intent and dispatch seam cannot express the recovery launch the design promises, and the missing fields include authority-bearing choices.

Evidence: W-RECOVER promises the dead round's role, runtime, model, cap, goal, worktree, transcript, and reviewed adopt-or-redo chain at metasystem/records/watch-verb/watch-verb-design.md:461-464; the action ledger pins only role, runtime, model, capMin, and briefDigest at lines 630-633. The landed staging seam hard-codes role steward-continuation and workspace permissions at metasystem/internal/steward/stage.go:16-95. Although the intent contains Goal, the dispatcher imports only role, brief, job, runtime, model, and permissions from the consumed intent at metasystem/scripts/agents/dispatch.sh:1175-1193; it imports neither goal nor cap and forces DESTRUCTIVE-REACH. The design adds only recoveryRoot to the intent and does not settle permissions, working mode, destructive reach, reviews linkage, reasoning effort, mission or stream binding, source packet, parent identity, or cap transport. Some roles, such as code critic, also require a reviews target that this seam cannot carry. Generalizing the currently one-role steward authority without a closed tuple lets implementations launch materially different work or fail every recovery. Remedy sketch: enumerate and digest the complete role-specific dispatch tuple, then name each change and refusal in StageIntent, authorize-dispatch, and dispatch.sh.

DISPOSITION: ACCEPTED. Binds the R-38-m0 joint round.

## WV-R2B-07 [high]

The terminal-cause producer enumeration assumes classifiers that the tree does not contain, leaving native-cap, authentication, and network deaths to implementer judgment and preventing required bounded retries.

Evidence: Design section 4.1 directs adapter adjudication to stamp native-budget-cap when the result identifies it and infra-capacity, infra-auth, and infra-network from the same classifier that already writes outage marks at metasystem/records/watch-verb/watch-verb-design.md:426-454. Actual adjudication maps any nonzero command to generic runtime_error at metasystem/internal/adapter/adjudicate.go:195-203. Its outage hook calls a classifier that recognizes provider overload or framed HTTP 5xx lines only at metasystem/internal/outage/outage.go:175-205 and 235-261. There is no authentication, network, or native-dollar-cap output class, and the design supplies no exact runtime result shapes, precedence, producer call sites, or ambiguous-case outcome. An implementer must add prose parsing, invent per-runtime mappings, or route these incidents to W-ESC; the latter fails the goal's once-only infrastructure recovery and the recorded native-cap specimen. This is predecessor finding WV-R1B-06's missing typed substrate shifted into an unspecified producer. Remedy sketch: specify byte-level, per-adapter terminal evidence mappings and ambiguity precedence, with a producer fixture for every member of the died set before observation begins.

DISPOSITION: ACCEPTED. Binds the R-38-m0 joint round.

## WV-R2B-08 [high]

The proposed trial adjudicator is not independent when the machine seat dispatched the target job, allowing the dispatch hand to judge the evidence used to promote responses to its own failures.

Evidence: Design section 7.3 allows the machine's main orchestrator seat to label every would-act correct-target or false-alarm at metasystem/records/watch-verb/watch-verb-design.md:679-710. The sample commonly concerns a job that this same seat dispatched. The binding seat record says the dispatch hand may neither examine nor accept work it starts at metasystem/plans/seat-governance-record.md:55-90, and R-21/R-22 identify self-operated cause classification as the laundering channel at metasystem/memory/rulings.md:47-53. Labeling whether the seat's failed target was actually alive, healthy, or recovered is examination of its own dispatch and directly changes whether W-RECOVER receives authority. Excluding only the watcher and its recovery delegate does not remove this conflict, so predecessor finding WV-R1B-08's claimed independence is not genuine. Remedy sketch: make label eligibility depend on the target dispatch provenance and exclude its dispatching seat as well as the watcher and class author/operator.

DISPOSITION: ACCEPTED. Binds the R-38-m0 joint round.

## WV-R2B-09 [high]

Terminal goal-less delegate failures cannot reach the W-ESC route because the design's closed tracked enumeration omits them.

Evidence: Design section 3.2 declares tracking closed and includes terminal failures only when they are newer than their goal's claim and lack a newer landing receipt at metasystem/records/watch-verb/watch-verb-design.md:313-340. Yet section 4.2 explicitly assigns a goal-less died job to W-ESC at lines 461-466, and section 6 forbids acting on anything outside that enumeration at lines 588-604. Goal-less dispatch records are a supported shipped shape: GoalID is explicitly optional at metasystem/internal/dispatch/jobrecord.go:48-49, and metasystem/internal/dispatch/build.go:95-108 deliberately represents unbound reservations with a null goal revision. Once such a job terminalizes it is neither nonterminal nor joinable to a claim, so the read surface and acting scan may lawfully omit it even though the response table promises escalation. An implementer must either violate the closed enumeration or miss the incident. Remedy sketch: explicitly include every unrecovered terminal goal-less delegate record in the tracked set and define its durable escalation lifecycle.

DISPOSITION: ACCEPTED. Binds the R-38-m0 joint round.
