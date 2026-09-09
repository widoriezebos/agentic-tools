# plan-fields-outside-fences

- State: queued
- Priority: 3
- Sequence: 23
- Risk: severity=1 novelty=1 exposure=2 accumulation=2 basis="severity 1: a false stop-hook line, no data or process at risk; novelty 1: a one-function scanner fix with a fixture; exposure 2: every seat's end-of-turn report on this repo; accumulation 2: the false line is reported every turn and trains the reader to ignore the template class"
- Tier: 2
- Intent: The stop hook's plan scanner reads a plan's header fields from anywhere in the file, including inside fenced code blocks, so a design that documents the plan field grammar acquires that grammar as its own fields. plans/goal-scope-bounds-design.md has been reported TEMPLATE-UNFILLED every turn for a '- Next step: <one line, required>' line inside the fence that documents the split member draft format. DONE means planField reads header fields from outside fenced blocks only, with a fixture proving a fenced example is ignored and a real field beside one still read. The fix is written and verified in the m1d working tree (internal/report/openwork.go plus a test; go test ./internal/report/ green at 87.2 percent coverage, supervision-hook fixtures green, fast gate green); it needs a lawful landing class - the agent commit refused with missing-declaration because a direct fix is tier-1 only with a root job, goal and test receipt.
- Origin: main
- Next step: Land the staged fix. Either Wido commits it himself (human commits are sovereign) or it rides the next reviewed chain that touches internal/report as tier-1 carriage under this goal. LANDED 2026-09-09 in d533caf17 under goal stop-refusal-fits-on-one-screen: planField in internal/report/openwork.go reads header fields outside fenced code blocks only (tests: a fenced example is ignored, a real field beside one is read), so the TEMPLATE-UNFILLED line for plans/goal-scope-bounds-design.md is gone from every Stop. m1d's working-tree copy of the same fix is superseded. WHAT REMAINS on this goal, from the chain's third critic (OSR-08 accepted by Wido as risk, OSR-09/10 noted): the landed rule pairs fence lines by position and treats an odd trailing one as text, which is wrong when the stray fence comes first; indented fences (one to three spaces) and four-backtick fences are not recognised. The fix is the CommonMark rule, a fence opens at a line of three or more backticks with up to three leading spaces, closes at the next fence of at least that length, and an unclosed fence runs to end of file, plus one diagnostics line naming the plan and the line of an unclosed fence so the plan never vanishes silently; the third critic's two trap plans (stray-first, stray-last) are the tests. Small, mechanical, one Sol round and one code critique.
- OpenedAt: 2026-09-07T07:15:26Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-07T07:15:26Z KFMKG168GDC5CHF14FXMJ5WHMN-m1d-25755dc0 open actor=m1d+main-1788764558-63534-a15b0d targets=plan-fields-outside-fences
- 2026-09-08T16:00:28Z MZZQ91BTF2S6KJZD1WGWMPSBVW-m1-7cd0bd60 set-priority actor=human:Wido targets=plan-fields-outside-fences reason=priority-order subject=plan-fields-outside-fences from=unranked to=3:23 requested-sequence=23
- 2026-09-09T11:14:23Z 7BA304AZBZKR7MK1CHT79P6S2W-m1-1701c13c edit actor=m1+main-1788940932-18533-7fa6c2 targets=plan-fields-outside-fences
Integrity: sha256=bf444dee869142867211ec03a6edfe8a81bd2674911471df21f025216d698dd8
