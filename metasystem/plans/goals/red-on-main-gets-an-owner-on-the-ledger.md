# red-on-main-gets-an-owner-on-the-ledger

- State: queued
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="severity 3: a red on main with no owner stalls every landing that runs the failing groups, as on 2026-09-16 when two trunk reds held two finished units and a peer seat for hours; novelty 2: the landing rule already says stop and blame nobody, the new part is who fixes it and how narrowly it holds; exposure 3: every seat, every deep proof; accumulation 2: unowned reds live only in lane reports and are lost with them"
- Tier: 3
- Intent: A red that does not land must have an owner on the ledger, never only in a lane report. Evidence 2026-09-16: the trunk reds proof-grades and alert-episode had an owner only because seat m1e sent them to Codex by hand, and lane 5 held every unit rather than the batches that select the failing groups. DONE: (1) when a proof is red on a base tree with no candidate unit in it, the landing machine records a blocking trunk-red entry on the ledger naming the attempt, the group, the scenario, the failing assertion, the tree and an owner seat, ranked ahead of the landings it blocks; the entry is visible to goal next and to the health roles; (2) the hold is narrow: only batches whose selected groups include the failing groups wait, everything else lands; (3) a unit ejected from a batch gets its fix round recorded on its own goal (Next step) by the same verb, so no red lives only in a report; (4) tests with injected clocks and fake proof verdicts, each rule red under the mutation that removes it; (5) the next real trunk red after landing is found on the ledger with an owner without a person writing it. Design question for the page, not decided here: R-93-m1e makes opening a goal a person act, so the entry is either a dedicated red register the machine may write, or a goal opened in the landing machine name, which needs Wido to amend R-93.
- Origin: human
- Next step: Opened in the name of Wido by seat m1e from the enrolled pane on his word 2026-09-16 20:35 CEST. Lead into the build brief of units-land-in-batches-under-one-proof (the verb that sees the red is the batch landing verb); design-bearing, so one headless Fable design round then a Codex critique, after Wido rules the shape of units-land-in-batches-under-one-proof. Instance already handled by hand: trunk-reds-proof-grades-and-alert-episode.
- OpenedAt: 2026-09-16T18:33:32Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-16T18:33:32Z YTABBWXAVKBEZJAEDK2ADTBK6T-m1e-c6925449 open actor=human:Wido targets=red-on-main-gets-an-owner-on-the-ledger
Integrity: sha256=1dac6337eadd07401ecd9a0d25904c07c0c09a24edea66d13996524167f8ca76
