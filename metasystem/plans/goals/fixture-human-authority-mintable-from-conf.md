# fixture-human-authority-mintable-from-conf

- State: queued
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="severity 3: an agent can approve its own goals and raise its own budgets, the exact act the authority layer forbids; novelty 2: replacing a conf-derived grant with an injected prover is an ordinary seam change, the fixture beds make it wide; exposure 3: every seat, every fixture bed, every human verb that takes the flag; accumulation 2: every landing that adds a --fixture-human-authority site widens it, and human-proof-fits-the-act revision 2 proposes six"
- Tier: 3
- Intent: An agent can mint a valid enrolled-grade human authority proof today by writing metasystem.runtimes=fake into the worktree's metasystem.conf and passing --fixture-human-authority to approve, set-budget, open or edit; the fixture grant is derived from repository bytes an agent may edit, so the boundary the human-authority layer exists to hold is not held (found by the second design critic of human-proof-fits-the-act, HPA-10, 2026-09-09)
- Origin: main
- Next step: Read internal/fixtureauth/fixtureauth.go lines 63 and 268-292 (fixture mode from the worktree conf alone; GoalHumanAuthorityProbe.Allows), cmd/metasystem/goalsync_mutations.go lines 583-604 (the probe becomes a proof) and internal/humanauthority/authority.go lines 99-145 (the observed fixture proof is accepted as valid). DONE means the fixture grant cannot be created by editing repository bytes: a test-only injected prover, or a capability held outside the checkout that an agent cannot write, with a canary in which an agent shell sets runtimes=fake in a scratch worktree and every --fixture-human-authority act is refused. The fixture beds that use the flag today (goal-cli-fixtures.sh, dispatch-fixtures.sh, channel-fixtures.sh, supervision-hook-fixtures.sh) move to the injected prover. Never test this against the live ledger. Sequence with human-proof-fits-the-act, whose revision 2 would otherwise register the flag on six more human verbs.
- OpenedAt: 2026-09-09T13:32:42Z
- Revision: 3
- BudgetExceptions: 0

History:
- 2026-09-09T13:32:42Z B8QAYYA95EQ8A9XT6SNP9HS04B-m1-1701c13c open actor=m1+main-1788940932-18533-7fa6c2 targets=fixture-human-authority-mintable-from-conf
- 2026-09-09T14:36:28Z 76XV3QEKM1Y759NSDZ4GYEH9R3-m1-c6925449 approve actor=human:Wido targets=fixture-human-authority-mintable-from-conf,hp-enrollment-before-migration,hp-every-by-proves-a-human,hp-grading-matrix-for-widening-acts,hp-recovery-grants-no-grade-to-unstamped-intents,hp-refusals-print-the-one-command,hp-relayed-word-retired,hp-resume-takes-its-budget-from-the-ledger,hp-terminal-grade-for-stopping-acts
- 2026-09-11T07:59:02Z DNTNV8WEXVTBD8MN4B14F7D0WH-m1-c6925449 unapprove actor=human:Wido targets=fixture-human-authority-mintable-from-conf reason=Wido 2026-09-11: standing approval withdrawn to regain control of what is built next; re-approve deliberately
Integrity: sha256=dd9fed015a1814abe5cf10d19760ec1b4486b77a9625169d5696c711888dc9f6
