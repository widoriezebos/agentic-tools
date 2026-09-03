# token-spend-fence

- State: queued
- Intent: Wido's order 2026-09-03 (verbatim, R-58-m1): 'agent sessions are not the expensive resource, TOKENS are. So this is not what we need. For now set it higher and then make sure we design something better'. THE PROBLEM: the only spend control in the dispatcher is the reserved-minute pool per goal, which charges every dispatch its full cap (120 minutes) whether the round runs 15 minutes or 120, and knows nothing about tokens or money. On 2026-09-02 the fleet ran 126 dispatches and the VM account hit its usage limit while every goal's pool said something unrelated; on 2026-09-03 a lawful seven-rung ladder stalled at 720 of 720 minutes having used about a third of the time. The pool is a runaway guard and should stay one; it is not the fence. DONE means: spend is measured in tokens and money from the runtimes' own usage records (the adapters already collect claude and codex usage per round; the mission family aggregates typed usage), aggregated per goal, per machine and per day; the human sets ceilings (per day fleet-wide, per goal) as config with a recorded word; the dispatcher refuses a dispatch that would cross a ceiling and names the ceiling, the spend so far and who can raise it; the health role shows today's spend against the ceiling in one line; the minute pool stays as the runaway guard with a default high enough that a lawful ladder never stalls on it; proven by fixtures replaying 2026-09-02's counts against a ceiling.
- Origin: main
- Next step: TIER 3 per R-54-m1 (a new dispatcher gate and a health role). Ladder: design (Fable; reuse the mission fence's usage aggregation, the adapters' usage records and m1b's cap-necessity design; token counts are the truth, money is derived from a configured price table), one review (Sol), one fold, closing review, build (Sol), code review (Fable), land with --chain. Runs after path-class-manifest lands and the second bar is promoted; nothing else on m1 before it.
- OpenedAt: 2026-09-03T08:42:26Z
- Revision: 1
- Labels: robustness, spend

History:
- 2026-09-03T08:42:26Z XR3P846WY3BC85808G7NY1YZJQ-m1-7bb1546e open actor=m1+main-1788333680-2840-7f79f4 targets=token-spend-fence
Integrity: sha256=d0c71dfb832e4644428cbac106b43690e1579e12b582e0974b06143d77f92bab
