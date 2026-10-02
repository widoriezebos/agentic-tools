# fixture-human-authority-mintable-from-conf

- State: abandoned
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="severity 3: an agent can approve its own goals and raise its own budgets, the exact act the authority layer forbids; novelty 2: replacing a conf-derived grant with an injected prover is an ordinary seam change, the fixture beds make it wide; exposure 3: every seat, every fixture bed, every human verb that takes the flag; accumulation 2: every landing that adds a --fixture-human-authority site widens it, and human-proof-fits-the-act revision 2 proposes six"
- Tier: 3
- Intent: What: Make sure an agent cannot fake a person's approval by switching the checkout into test mode. Why: Test mode is switched on by a setting in a file inside the repository (runtimes=fake in metasystem.conf). An agent can edit that file and then pass the test-only flag to approve, set a budget, open or edit a goal, and the system accepts it as a real person's act. A few acts already refuse test-mode proof (the general power of attorney and a forced done at the helm), but most human acts do not. Pros: Closes a real hole in the boundary that keeps human decisions human. Cons: Every test bed that uses the flag today must move to a new test-only mechanism, and the change touches security-critical code.
- Origin: main
- Next step: Next: Replace the file-based switch with something an agent in the checkout cannot write (a prover that only tests inject, or a capability held outside the checkout), and move the four test beds that use the flag onto it; never test this against the live ledger. Done when: a test in which an agent shell sets runtimes=fake in a scratch worktree sees every test-flag human act refused, and the moved test beds still pass.
- OpenedAt: 2026-09-09T13:32:42Z
- Revision: 5
- BudgetExceptions: 0
- Abandoned: by=human:Wido at=2026-10-02T21:01:24Z revision=5 opid=AQ3JR1Q88B4BHBWHR5GAEH58AX-m1e-9c612d71 because=Obsolete (audit 2026-10-02): defends against an adversary, outside the nobody-attacks threat model

History:
- 2026-09-09T13:32:42Z B8QAYYA95EQ8A9XT6SNP9HS04B-m1-1701c13c open actor=m1+main-1788940932-18533-7fa6c2 targets=fixture-human-authority-mintable-from-conf
- 2026-09-09T14:36:28Z 76XV3QEKM1Y759NSDZ4GYEH9R3-m1-c6925449 approve actor=human:Wido targets=fixture-human-authority-mintable-from-conf,hp-enrollment-before-migration,hp-every-by-proves-a-human,hp-grading-matrix-for-widening-acts,hp-recovery-grants-no-grade-to-unstamped-intents,hp-refusals-print-the-one-command,hp-relayed-word-retired,hp-resume-takes-its-budget-from-the-ledger,hp-terminal-grade-for-stopping-acts
- 2026-09-11T07:59:02Z DNTNV8WEXVTBD8MN4B14F7D0WH-m1-c6925449 unapprove actor=human:Wido targets=fixture-human-authority-mintable-from-conf reason=Wido 2026-09-11: standing approval withdrawn to regain control of what is built next; re-approve deliberately
- 2026-09-30T18:48:25Z CTS3Y9HZA8WY5FNG6E7R2Y36DK-ui-bc2fda53 edit actor=human:Wido targets=fixture-human-authority-mintable-from-conf
- 2026-10-02T21:01:24Z AQ3JR1Q88B4BHBWHR5GAEH58AX-m1e-9c612d71 abandon actor=human:Wido targets=fixture-human-authority-mintable-from-conf reason=Obsolete (audit 2026-10-02): defends against an adversary, outside the nobody-attacks threat model
Integrity: sha256=d1a01e747db49571ccd4c9817950f95dbce890f789f83350ab3da5e251335810
