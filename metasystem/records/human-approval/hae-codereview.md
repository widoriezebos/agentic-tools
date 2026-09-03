# Human Approval for Execution — code review, with dispositions

Critic: claude-fable-5-1 (hae-codereview) over hae-build-b (32 files).
3 material (real correctness bugs), all named-artifact. Folded in one
correction round then a closing code-critic pass for closure.

## F-1 [high|material=True]

The change breaks existing tests in the steward package that the builder never ran, and the builder reported no gap. The steward claim helper still passes a budget tuple to the claim verb, which now refuses any supplied budget before touching the ledger and would refuse the never-approved goal anyway. Two steward open-work tests expect a queued goal with a budget to count as claimable work, but claimable work is now derived from approved goals only. The repository's full test suite therefore fails on the reviewed tree, which damages what certifies the change.

Evidence: metasystem/internal/steward/ledgerattention_test.go:120 calls goal.Claim with a Budget; metasystem/internal/goal/verbs.go:413-415 returns an error whenever a budget is supplied, and verbs.go:451-453 refuses any goal whose state is not approved (the bed opens goals queued via the open helper at ledgerattention_test.go:95-98 and never approves). The helper is used at ledgerattention_test.go:326 and :379 and fatals on error. metasystem/internal/steward/openwork_converted_test.go:166-178 and :193-202 expect WorkClaimable from a queued goal with a budget; metasystem/internal/goal/project.go:318-322 now fills Claimable from the frontier's Ready bucket, which project.go:503-511 fills from approved, unexpired goals only. The implementer's return.json evidence shows go test run only for ./internal/goal and ./cmd/metasystem, and its gaps array is empty. I could not execute the tests (see gaps).

DISPOSITION: FOLD (fix in the correction round)

## F-2 [medium|material=True]

The enroll-terminal command fails on every fleet machine after the first one. The design requires the human to enroll the terminal on each machine (section 10, step 3). On the second machine the local enrollment file is written, but the command then tries to publish the fleet record, the engine answers that the record already exists, and the command treats that answer as a failure: it prints a misleading message saying the fleet cutoff did not publish, exits 1, and never prints the enrollment. The local enrollment is real, so the human is told a lie about a step the design mandates.

Evidence: metasystem/cmd/metasystem/goalsync_mutations.go runGoalEnrollTerminal (worktree lines 812-821): after humanauthority.Enroll succeeds it calls goal.RecordFleetEnrollment and returns 1 unless the outcome is Confirmed or ConfirmedLate. metasystem/internal/goal/approval.go:427-429 returns NothingToDo when the root already carries FleetEnrollment. metasystem/internal/goal/txn.go:743-748 maps NothingToDo to OutcomeAbandoned. plans/human-approval-for-execution-design.md:679-681 states enrollment is per machine. No test covers a second machine's enrollment through the command.

DISPOSITION: FOLD (fix in the correction round)

## F-3 [medium|material=True]

A test that certified the once-per-change queue reblock was weakened rather than ported. The turn-verdict logic still blocks a session holding a claim exactly once when the shared queue digest changes. The rewritten test approves its queued goal up front, which makes the always-on idle-backlog block fire on every verdict, and then changes the assertions to expect blocking every time, dropping the check for the queue-changed message and the check that an unchanged digest does not reblock. The property is no longer proven by any test in the reviewed tree. A faithful port would keep the goal queued during the digest phase and approve it only before the final claim from the other machine.

Evidence: Diff for metasystem/internal/goal/turnverdict_test.go:192-236: the added approveGoalForTest call at line 204 precedes the first verdict; the assertion at the former line 211 that an unchanged world does not reblock now requires ShouldBlock; the assertion at the former line 217 dropped the 'shared goal queue changed' text; the assertion at the former line 228 that an unchanged digest does not reblock twice now requires ShouldBlock. metasystem/internal/goal/turnverdict.go:477-487 still implements the once-per-change block on the queue digest; turnverdict.go:256-264 shows the idle-backlog block that now fires every turn because the approved goal is claimable. The queue digest at turnverdict.go:617-620 covers queued goals, so the original property remained testable with an unapproved goal.

DISPOSITION: FOLD (fix in the correction round)

## F-4 [medium|material=False]

Consumers the design lists in its build bound were not built and the omission was not reported as a gap. The steward's ledger-attention snapshot lists only queued goals in its queue, so an approved goal never appears there and an approved goal with an expired relayed approval appears nowhere. The metrics debt computation, the steward's open-work wording, the backlog-mechanism law text, the glossary and the command-line fixture scenario were not changed. The brief's workspace excluded these paths, so this is a named follow-up rather than a defect in the delivered artifact.

Evidence: plans/human-approval-for-execution-design.md sections 8 and 13 name internal/steward (ledgerattention.go, narrate.go, openwork.go), internal/metrics/compute.go, docs/backlog-mechanism.md, docs/glossary.md and scripts/agents/goal-cli-fixtures.sh. The diff touches none of them. metasystem/internal/steward/ledgerattention.go:154-156 filters on StateQueued only. The implementer brief (prompt.md lines 17-23) required a gap-stop for anything outside internal/goal and cmd/metasystem; return.json reports no gaps.

DISPOSITION: noted (non-material)

## F-5 [low|material=False]

The projection-at-commit reader now reads the wall clock when no instant is supplied, and its three steward callers supply none. The design required callers to pass the tick instant they hold and rejected reading the wall clock inside the frontier path so that a fixed-instant test and a claim a moment later agree. Follow-up: make the instant a required parameter and thread it from the steward.

Evidence: metasystem/internal/goal/attention.go:48-58 takes an optional variadic instant and defaults to time.Now. metasystem/internal/steward/ledgerattention.go:225, :242 and :478 call ProjectAt with two arguments. plans/human-approval-for-execution-design.md:566-570 and section 11 item 11.

DISPOSITION: noted (non-material)

## F-6 [low|material=False]

The machine-local enrollment file is no longer consulted at all by the expiry predicate or the grant-time cutoff; only the synced root record counts. If the local enrollment succeeds but the fleet publish fails, relayed approvals stay valid on that machine until the human re-runs the command. The synced record is the right primary source; the local file could remain a belt. Follow-up: OR the local enrollment into the horizon as the design's section 5 and 8 described.

Evidence: metasystem/internal/goal/approval.go:24-30 builds the horizon from t.Root.FleetEnrollment only; no call to humanauthority.ReadEnrollment exists in the diff. metasystem/cmd/metasystem/goalsync_mutations.go runGoalEnrollTerminal enrolls locally before publishing, and returns 1 if the publish fails.

DISPOSITION: noted (non-material)

## F-7 [low|material=False]

Two design-named proof obligations are not covered. No test shows recovery replaying a legitimate claim of an approved goal through the gate and confirming; the recovery tests only show refusals. No test exercises an approval attempt with a proof produced under an agent ancestor; the agent test covers a missing human name and a nil proof only. By reading both work: a claim replay passes a nil budget into the gated constructor, and an agent-chain proof satisfies neither the proven nor the temporary predicate. Follow-up: add the two fixtures the design named.

Evidence: metasystem/internal/goal/recover.go:245-258 rebuilds a claim with a nil budget unless the journal carries budget arguments, then metasystem/internal/goal/verbs.go:451-453 and approval.go:43-54 gate it. metasystem/internal/humanauthority/authority.go:108-111 and :182-193 reject a proof without terminal ancestry or the temporary outcome. metasystem/internal/goal/approval_test.go:75-89 (TestAgentCannotApprove) and recover_test.go changes assert refusals only. plans/human-approval-for-execution-design.md:759-770 names TestRecoveryReplayHitsTheSameGate and TestApproveRefusesWithoutProof with an agent-chain proof.

DISPOSITION: noted (non-material)

## F-8 [low|material=False]

The relayed sweep's one-time guard can refuse for the wrong reason. It scans the root history for any relayed approve line with no goal filter, but the root now also retains per-goal relayed approve lines of pruned goals, so a single pruned goal that was once approved by relay makes a later relayed sweep report that the fleet already used its relayed sweep. The relayed sweep is a one-time bootstrap, so the practical impact is small. Follow-up: mark the sweep's root line distinctly and match on that.

Evidence: metasystem/internal/goal/approval.go:370-372 calls firstRecordedRelayedActIn(t.Root.History, "", "approve", ruling); metasystem/internal/goal/file.go:211-219 matches any line of that verb when the goal id is empty; file.go:243-247 now retains relayed approve lines on the root through prune.

DISPOSITION: noted (non-material)

## F-9 [low|material=False]

Small documented deviations from the design that do not change behavior. The named proof method for approval was not added; the engine reuses the resume predicates, which have the identical definition. The approval-gate arm in the approve verb bumps the root revision without appending a root history line; the implementer's green run shows no parse rule enforces one. The sweep dry run does not print the relayed-expiry summary line the design described. The resume command still requires the four budget flags, which must now exactly equal the approved tuple. Record as follow-ups or accept.

Evidence: metasystem/internal/goal/approval.go:64-75 uses AuthorizesResume and TemporaryResumeFor; metasystem/internal/humanauthority/authority.go:117-137 shows the two predicates share one definition. approval.go:109-125 (armApprovalGate) increments Root.Revision with no HistoryLine. metasystem/cmd/metasystem/goalsync_mutations.go runGoalApprove dry-run branch prints the listing, the digest and the skipped list only. metasystem/internal/goal/stop.go:388-395 and the unchanged runGoalResume budgetTuple(true) call.

DISPOSITION: noted (non-material)
