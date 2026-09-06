# human-goal-verbs-forgiving

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="severity 2: a wrong bind of a human budget act would misrecord authority, but every path stays human-only at the enrolled terminal and every refusal stays a refusal; novelty 1: a grammar, a routing table and a default on verbs that exist; exposure 2: the human's verbs only, on every machine; accumulation 1: nothing compounds"
- Tier: 2
- Intent: Wido's word 2026-09-06, after three refused attempts to raise one goal's budget at his terminal: 'this command is too complex, not intuitive and maybe not forgiving enough'. The evidence of the day: set-budget on a parked goal refuses with 'set through goal approve with --budget'; approve --budget box together with the five limits refuses 'mutually exclusive'; the working form needs five long flags, all-or-nothing, that mirror an internal tuple; --budget takes the literal word box as its value; every human verb needs --by even though the enrolled terminal already knows who enrolled it; the same morning goal edit --risk refused for a missing --evidence whose only hint was a format string. Every refusal names flags, none prints the command that would have worked. The human should not have to know whether a goal is queued, parked or claimed to give it a bigger box. DONE means, at the enrolled terminal: one verb gives a live goal (queued, parked or claimed) a box, taking the compact form everyone already writes in prose (1d/10/720m/1/3) or a preset (norm for the tier's ceiling, keep for the standing box), and does the approval or revision rebind itself; the five explicit limits stay as the long form; --by defaults to the enrolled human's recorded name; every refusal of a human verb prints one complete command that would have succeeded with the values it saw; and a fixture drives each refusal and asserts the printed command runs. Authority does not widen: every act stays human-only at the enrolled terminal and every history line stays as it is.
- Origin: main
- Next step: Two slices. Slice 1 (design, small): read cmd/metasystem/goalsync_mutations.go (approvalBudget, budgetTuple, the set-budget routing) and internal/goal for the approve and set-budget transactions; write the one-page design: the compact box grammar and presets, the routing table by goal state, the --by default from the terminal enrollment record, the refusal-prints-a-command rule and where it lives (one function every human verb refusal passes through), the fixture list. Sol critiques once. Slice 2 (build): implement per the design; goal-cli-fixtures.sh drives each refusal and runs the printed command. Receipt: go gate plus goal-cli-fixtures.sh.
- OpenedAt: 2026-09-06T12:45:35Z
- Revision: 2
- Arc: verbs-match-intent
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-06T12:45:35Z 80R77DYNPPC53PEPJ5AHWB9Y4N-m1d-62183579 open actor=m1d+main-1788683763-71870-f7f607 targets=human-goal-verbs-forgiving
- 2026-09-06T12:49:45Z 82VNKJKQ2ZZZJ5Q83NZX6SMEWW-m1d-62183579 set-arc actor=m1d+main-1788683763-71870-f7f607 targets=human-goal-verbs-forgiving
Integrity: sha256=2e9f9e9ea80b8adbbcca480a6cd5dff2e3456cbdf4f92a281361eab9abedb7b7
