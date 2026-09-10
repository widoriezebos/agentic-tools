# Critique r1 of chain bsws-build1b-20260909 (goal breach-stop-wedges-seat)

Critic bsws-crit1b-20260909 (code-critic, claude claude-opus-5, xhigh,
read-only) on work round bsws-build1b-20260909-r2, reviewed tree
89a124a33d895d64cebe6681dc454598675ac52b as the critic computed it, 2026-09-09
10:07 to 10:18Z. Seven findings, five material. Every material finding is
accepted and folds in round 3. Dispositions are the orchestrator's (m1d).

| id | severity | finding, in one line | disposition |
|---|---|---|---|
| BSW-01 | high | ServingProjection (internal/goal/goalverbs.go) picks the served goal from a map without checking the fence; with a fenced and a live claim on one machine it named the stopped goal 36 times out of 40, so every delegate prompt and every brief's serving-goal section was wrong most of the time | accept; fold: skip fenced claims, choose in backlog order |
| BSW-02 | high | Human resume of the stopped goal while the seat holds a live claim is refused with the raw validator text ("the ledger tree at <commit> does not validate: ... the quota is one claim per machine"), naming no remedy. Answers the mandate's question 3: a clean refusal, fence and claim intact, not a second wedge; the words are the defect | accept; fold: resume pre-checks the second live claim and refuses in plain words naming the other goal and the remedy |
| BSW-03 | medium | The fenced goal vanished from goal next (cmd/metasystem/goal.go prints "no claimable goal ... no matching eligible work") and from the channel report's Next up lines; before the change the wedge was at least visible | accept; fold: one FENCED line per fenced claim in goal next and in the channel report, through one shared formatter |
| BSW-04 | low | The steward's open-work classifier (internal/steward/openwork.go) now says "no claim held here and no declaration" while a fenced claim is held; the classification is right, the sentence is false | accept; fold: a fenced-only branch with a true sentence |
| BSW-05 | medium | No fixture reaches the branch the change exists for: a live claim beside a fenced one; deleting the FENCED line in that branch leaves every test green | accept; fold: the fixture |
| BSW-06 | low, not material | The three new display lines skip the nil guard the surrounding branch applies; one caller, always non-nil | fold anyway while the lines are touched |
| BSW-07 | low, not material | The steward's delivery health counts the fenced claim's age and will call the delivery role dead once the claim outlives 150 percent of its elapsed limit, which an elapsed-limit stop guarantees; predates the change, now durable | not this chain; backlog as its own goal (the orchestrator asked Wido to open it) |

The critic's gaps: no conformance review object existed for the round, so it
computed the tree itself in a scratch object store; it did not re-run the
nine-minute goal package or the goal-cli bed (the orchestrator ran the bed
outside the sandbox: passed in full, including wrong-terminal and the
brain scenarios); its probes ran on a copy of the reviewed tree. The
orchestrator runs the conformance review stage on the final round before
the closing read.
