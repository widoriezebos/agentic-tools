# code-critic-runtime-has-no-shell

- State: claimed
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="Every design-bearing chain pays an unverified critique and a seat-side rerun; every machine that dispatches a claude critic has it; nothing is destroyed."
- Tier: 3
- Intent: Both Fable code-critic rounds on chain rgr-build1 and both on chain shr-build1 (2026-09-06) reported the same gap: the claude runtime exposed only file reading and searching tools, no shell, so none of the evidence commands the review brief allowed (focused go tests, vet, gofmt, bash -n, git diff) could run, and every evidence entry was read or inferred. The code-critique skill expects the critic to run focused checks that distinguish a suspected defect from a hypothetical one, and the orchestrator had to run them in the critic's place each time. DONE means a code-critic dispatched on the claude runtime can run the brief's evidence commands in the reviewed worktree (read-only on the tree, shell allowed), the permissions preset or role requirements say so, and one critic return shows evidence marked ran.
- Origin: main
- Next step: Read scripts/agents/roles/code-critic.requirements.json and the claude adapter's tool policy for the code-critic role; decide whether the shell is withheld by the permissions preset or by the adapter's allowed-tools list; brief a one-round fix with a fake-runtime fixture.
- OpenedAt: 2026-09-06T11:01:39Z
- Revision: 3
- Labels: robustness
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T11:02:38Z revision=2 opid=SB01VF9Q2A59X40GM66RJJP2JV-m1-a4f8999f authority=proven digest=1e2029f1329a2dab7553c750398310039803bc8c7896f5f07712e4004c05830f
- Claimed: machine=m1c lineage=main-1788680061-17829-64951c at=2026-09-06T19:00:35Z revision=3 accountingRevision=3
- StopCapability: generation=3 revision=3 machine=m1c claimEpoch=1 fenceEpoch=0

History:
- 2026-09-06T11:01:39Z P2Z5N2TM3HW45Z2PJXYQZ1GV6N-m1c-7cd0bd60 open actor=m1c+main-1788680061-17829-64951c targets=code-critic-runtime-has-no-shell
- 2026-09-06T11:02:38Z SB01VF9Q2A59X40GM66RJJP2JV-m1-a4f8999f approve actor=human:Wido targets=code-critic-runtime-has-no-shell,human-acts-derive-their-lineage,land-sh-omits-the-full-width-chain-receipt,landing-receipt-races-the-narrator-digest
- 2026-09-06T19:00:35Z 9T8B5S5FQBVJJ6RG4QM1247WAK-m1c-7cd0bd60 claim actor=m1c+main-1788680061-17829-64951c targets=code-critic-runtime-has-no-shell
Integrity: sha256=e8a5d1f4ee6ed0520aaa5b35ad9c1d5ea94cfb792b55bf3f6db89ed383ccfee7
