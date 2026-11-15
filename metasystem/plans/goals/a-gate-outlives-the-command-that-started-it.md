# a-gate-outlives-the-command-that-started-it

- State: queued
- Risk: severity=2 novelty=2 exposure=2 accumulation=1 basis="On 2026-11-15 the MacBook lane lost the gate of a-fixture-commit-exists-before-head-names-it twice and stopped for a person: the agent ran landing prove --gate --wait inside one tool call, its runtime (Claude Code 2.1.319, headless) stopped that command after twenty minutes and the gate, its descendant, died with it; recorded environment/lost-process, repeated once on the same path, then stop. Analysis /Users/wido/agentic-tools-evidence/lane-test-20261009/designs/analysis-the-gate-dies-with-its-caller-2026-11-15.md (ANALYSIS STANDS, Astra r2; experiment and transcripts under /Users/wido/agentic-tools-evidence/lane-test-20261009/designs/gate-exp-20261115 and /Users/wido/agentic-tools-evidence/lane-test-20261009/designs/gate-evidence-20261115). Design /Users/wido/agentic-tools-evidence/lane-test-20261009/designs/a-gate-outlives-the-command-that-started-it.md (DESIGN STANDS, Astra r2). Ordered by Wido 2026-11-15."
- Tier: 2
- Intent: A gate outlives the command that started it: the landing agent starts the merge gate without waiting and ends its turn, the engine refuses a public landing prove --wait that has no person authority before anything starts (next step: the same command without --wait), and a lost check's stop says what happened. No session command holds a lane check as its descendant, so the agent runtime's command limit can no longer end a gate.
- Origin: main
- Next step: Build from the agreed design page /Users/wido/agentic-tools-evidence/lane-test-20261009/designs/a-gate-outlives-the-command-that-started-it.md; it is the design, no new design is written. The default roster cuts one brief per unit from it (each starts with an inventory; one Astra round per brief). U1 (estimate 60): the refusal in runIntentLandingProve for a public --wait with person == nil, placed before plain.Settled and plain.Start; the skill's gate routes and the batch refusal's next step (internal/landing/plain/batch.go:289) lose --wait and point to the one dispatch table of the page; acceptance items 1 to 7 of the page, including the helper-process lifetime fixture. U2 (estimate 30, --last): Stop.Words and the NoRepeat text say what the record shows. Not built: parent isolation of gaterun.LaunchDetached, a longer timeout, a cause caller-ended, a watchdog. After deploy: the live observation of the page's section 5 on the MacBook lane with a newly eligible batch; keep the listed evidence. Until this is deployed on a computer, a gate there that outlasts twenty minutes needs a person's landing prove --gate from a terminal.
- OpenedAt: 2026-11-15T22:34:19Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-15T22:34:19Z 2GW86GYFWZT928G9152B7WVM26-landing-367f2425 open actor=human:Wido targets=a-gate-outlives-the-command-that-started-it
Integrity: sha256=1e52ebf299b4ced826e436df708d7fe31bce9e09df3e8f0669b6261acdcb7f07
