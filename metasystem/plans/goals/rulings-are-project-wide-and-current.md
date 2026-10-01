# rulings-are-project-wide-and-current

- State: approved
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="rewrites the human ruling register every seat and reviewer reads; engine readers of ruling ids"
- Tier: 2
- Intent: What: The rulings register holds only rulings that are on point and valid for the whole project, each in one clear plain-English sentence or two, and people find them in the interface as the project's standing law. Rulings tied to one goal, one night or one budget leave the register for an archive or the goal's own record, and rulings later rulings replaced are folded into the one that stands. In the interface, Rulings get their own place under Project, next to Intent and Doctrine, as a short readable view grouped by theme, each ruling with its plain sentence, its id and date, Wido's original words one click away, and the archive behind one disclosure; the Decisions page stays about what waits on Wido and links to a ruling where an answer became one. Why: Wido, 2026-10-01: "we need to revisit the rulings! They need to be on-point, project-wide validity, not specific to goals", and "where in the UI do I find the rulings? I see doctrine, but to me rulings are almost more important". Today the register has 175 entries, about half spent one-off approvals, every reviewer reads it whole each round, and in the interface it hides as a tab in the history part of the Decisions page. Pros: a short, current law that reviewers and seats read cheaply and Wido finds at once. Cons: the register is append-only by its own rule and engine code reads ruling ids (budget and slice approvals, landing classes, a hard-coded retired ruling), so the clean-up keeps every old entry findable in an archive the engine can still read.
- Origin: human
- Next step: Next: Wido answers the seven questions in the classification table (~/LocalStorage/agentic-tools-evidence/rulings-revisit-20261001/table.md) and approves it; then Fable designs the clean-up and the interface step together (the new short register and its archive, keeping every referenced id resolvable, and the Rulings view under Project), Astra critiques the design, Opus builds it, Sol reviews the code, and it lands. Done when: the register holds only current project-wide rulings in plain English, every engine reader of a ruling id still works against the archive, and Project shows a Rulings view grouped by theme with the archive behind a disclosure.
- OpenedAt: 2026-10-01T08:28:41Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-01T08:43:00Z revision=3 opid=WYNVJ12X5GC3B07BKP2A508DT8-ui-31a738e9 authority=proven digest=573008f19f67fa38829d7a3dc16483d4f6043212e0284c449018d0d18451a1ab episode=3

History:
- 2026-10-01T08:28:41Z SQNSN4RHE0019TPAJHN31EZESM-ui-31a738e9 open actor=human:Wido targets=rulings-are-project-wide-and-current
- 2026-10-01T08:41:09Z 6AKR4Y4CBJVKHDW4Q5GSRG5WY9-ui-31a738e9 edit actor=human:Wido targets=rulings-are-project-wide-and-current
- 2026-10-01T08:43:00Z WYNVJ12X5GC3B07BKP2A508DT8-ui-31a738e9 approve actor=human:Wido targets=rulings-are-project-wide-and-current
Integrity: sha256=48cdab9178e858fa0509ef258ad157694a3684eaf8718caa548e2b445cf20a21
