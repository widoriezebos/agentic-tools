# Planning continuity design adjudication

Root authors and adjudicates; Fable independently critiques. Round 1 reviewed
SHA256 495456bf4e7175abef81392d37e9a2e066b0808e48d4d2270b578d195a09e7d7,
preserved at commit 1dcf2a37b. Report: intent-planning-fable-review-r1.md.
Launch 20260926t091003-5291dcd738, claude-fable-5-1, provider session
ca7bc1f0-9a15-4281-851b-53e02682c886, exit 0. Three material findings, all joined.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| IP-C1 | accepted | Build admission acquires an execution claim and selects its worktree. That is wrong for planning from a different checkout. reviewBriefFacts owns approved-goal/review-budget facts but also reader-only tool-call parsing; reuse facts, not that whole parser. launch.Manager.admit has launch limits, no aggregate design spending counter; do not misdescribe baselines as a budget. | Approved goal plus existing design admission; invoking checkout; no build claim or attempt charge. TestIntentDesignAdmissionNoClaim and second-worktree author journey. |
| IP-C2 | accepted | A crash before supervisor claim can strand Starting. The proposed unlocked absence/age check followed by m.fail would race supervisor startup. Supervise's FIRST Store.Update claims Supervisor and refuses terminal records before preparing output or starting a child. Therefore recovery must atomically test absence and terminalize under that same lock. A late supervisor then cannot start a child, even if its OS process exists. | Named bounded launch-owner recovery, no claims cleared and no unproven child ignored; TestDesignRequestCrashBeforeSupervisor covers both lock winners, delayed supervisor and uncertain custody. |
| IP-C3 | accepted | Public proof wait needs a publicly obtainable reference, regardless of the internal LaunchResult already retaining AttemptID. The first normal journey must not depend on private fixture knowledge. | status plus test/landing results expose actual goal/candidate proof reference; public output supplies wait input in TestIntentProofReferenceFeedsWaitProof, TestIntentWaitObservers and TestIntentWaitRealOwner. |

All seven notes considered: N1 exact home/collision rule, N2 minimal author
contract, N3 one existing-store request entry, N4 explicit design binding and N5
relative paths clarified. N6 root keeps open among nine starting points because
goal intake precedes design; focused work/agent and all help expose design. No
capability is hidden. N7 cancellation remains the existing owner.

Final round 2 checks the changed admission and atomic recovery contract, not prose
polish. These are consequential owner corrections; the root does not accept an
age-only process-death inference. Same-chain follow-up replay and publication
remain named mandatory implementation fixtures and independent Sol review scope.

## Final round 2

Fable reviewed SHA256 89d5f0261cdf70f2bae5faf25c869b66d3fe1ccd444f619e2f669045eaa3d47d
(preserved ee0549093); launch 20260926t092513-fbee9d93d4, same provider session,
exit 0. The launch's 18 model calls are cumulative over the resumed session;
9 provider turns describe this invocation. These are different units from tool
calls; internal/launch/claude.go:measureClaudeTranscript owns that distinction. Two material
findings, both accepted and retained. No third prose round.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| IP-C4 | accepted | follow_up chooses the next child under its own lock, and ClaimLaunchPreflight treats an operation bound to a different child id as a mismatch. A frozen operation ID alone cannot rejoin a completed child. The existing findOperationRecord read can recover that child without spawning or a new registry. | Freeze the request-to-operation binding before launch, retain child afterward; owner lookup handles a lost result with provenance validation. TestDesignCritiqueReplayAndCap names completed, missing-child, intervening-round and moved-revision cases. |
| IP-C5 | accepted | The timeout/budget-cap implementer-only branch precedes the new failed-critic retry branch. Existing design read admission does not enforce the live-subject fresh-root guard for the same design. | Explicit review design FILE --retry N; existing dispatch owner extends bounded retry for capped critics and path-scoped root selection/refusal. Named fixture drives the real branch, death proof, replay and cap. |

Fable calls these shape corrections. Root judges their resolution bounded and
fixture-expressible: the retained request and dispatch-read extension were already
in the design; the correction identifies their exact child lookup/selection and
the omitted retry case, with no new workflow mechanism. The falling 3-to-2
trajectory and declared failsafe end prose review on these two mandatory fixtures.
Independent Sol implementation review remains compulsory and can reject the
result; unresolved authority or behavioral defects cannot be waived by this exit.

N8 is folded into public stopped-supervisor recovery; N9 confirms public proof
reference production. Fable's unexamined supervisor exit, nil-reference death and
actual design-subject refresh are explicitly assigned to implementation proof.

Root's additional source check found reviewBriefFacts is not the entire dispatch
admission: internal/dispatch/stop.go:resolveGoalBindingWithReads requires a claim
and stop authority; dispatch.sh:require_goal_tier_ladder requires tier 3. Therefore
claim-free AUTHORING stands, but first paid REVIEW acquires the same ordinary
lawful claim used by build, with existing refusal/actor/epoch rules intact and no
worktree creation. TestIntentDesignCritiqueAdmission proves that direct public
journey. This preserves existing authority, rather than silently allowing
claim-free critiques. Root also made the existing document choice usable in
show/stop with --out FILE; no new selection mechanism.
