# plan-fields-outside-fences

- State: queued
- Priority: 3
- Sequence: 23
- Risk: severity=1 novelty=1 exposure=2 accumulation=2 basis="severity 1: a false stop-hook line, no data or process at risk; novelty 1: a one-function scanner fix with a fixture; exposure 2: every seat's end-of-turn report on this repo; accumulation 2: the false line is reported every turn and trains the reader to ignore the template class"
- Tier: 2
- Intent: The stop hook's plan scanner reads a plan's header fields from anywhere in the file, including inside fenced code blocks, so a design that documents the plan field grammar acquires that grammar as its own fields. plans/goal-scope-bounds-design.md has been reported TEMPLATE-UNFILLED every turn for a '- Next step: <one line, required>' line inside the fence that documents the split member draft format. DONE means planField reads header fields from outside fenced blocks only, with a fixture proving a fenced example is ignored and a real field beside one still read. The fix is written and verified in the m1d working tree (internal/report/openwork.go plus a test; go test ./internal/report/ green at 87.2 percent coverage, supervision-hook fixtures green, fast gate green); it needs a lawful landing class - the agent commit refused with missing-declaration because a direct fix is tier-1 only with a root job, goal and test receipt.
- Origin: main
- Next step: Land the staged fix. Either Wido commits it himself (human commits are sovereign) or it rides the next reviewed chain that touches internal/report as tier-1 carriage under this goal.
- OpenedAt: 2026-09-07T07:15:26Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-07T07:15:26Z KFMKG168GDC5CHF14FXMJ5WHMN-m1d-25755dc0 open actor=m1d+main-1788764558-63534-a15b0d targets=plan-fields-outside-fences
- 2026-09-08T16:00:28Z MZZQ91BTF2S6KJZD1WGWMPSBVW-m1-7cd0bd60 set-priority actor=human:Wido targets=plan-fields-outside-fences reason=priority-order subject=plan-fields-outside-fences from=unranked to=3:23 requested-sequence=23
Integrity: sha256=f6a51611026a7ce73c4fb23590fef455f96e1a3bc7333bedb18ea7bb210eb74f
