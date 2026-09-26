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
| IP-C3 | accepted | Public proof wait needs a publicly obtainable reference, regardless of the internal LaunchResult already retaining AttemptID. The first normal journey must not depend on private fixture knowledge. | status plus test/landing results expose actual goal/candidate proof reference; public output supplies wait input in TestIntentPublicProofAndFileWait. |

All seven notes considered: N1 exact home/collision rule, N2 minimal author
contract, N3 one existing-store request entry, N4 explicit design binding and N5
relative paths clarified. N6 root keeps open among nine starting points because
goal intake precedes design; focused work/agent and all help expose design. No
capability is hidden. N7 cancellation remains the existing owner.

Final round 2 checks the changed admission and atomic recovery contract, not prose
polish. These are consequential owner corrections; the root does not accept an
age-only process-death inference. Same-chain follow-up replay and publication
remain named mandatory implementation fixtures and independent Sol review scope.
