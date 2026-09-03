# token-spend-fence

- State: claimed
- Intent: Wido's order 2026-09-03 (verbatim, R-58-m1): 'agent sessions are not the expensive resource, TOKENS are. So this is not what we need. For now set it higher and then make sure we design something better'. THE PROBLEM: the only spend control in the dispatcher is the reserved-minute pool per goal, which charges every dispatch its full cap (120 minutes) whether the round runs 15 minutes or 120, and knows nothing about tokens or money. On 2026-09-02 the fleet ran 126 dispatches and the VM account hit its usage limit while every goal's pool said something unrelated; on 2026-09-03 a lawful seven-rung ladder stalled at 720 of 720 minutes having used about a third of the time. The pool is a runaway guard and should stay one; it is not the fence. DONE means: spend is measured in tokens and money from the runtimes' own usage records (the adapters already collect claude and codex usage per round; the mission family aggregates typed usage), aggregated per goal, per machine and per day; the human sets ceilings (per day fleet-wide, per goal) as config with a recorded word; the dispatcher refuses a dispatch that would cross a ceiling and names the ceiling, the spend so far and who can raise it; the health role shows today's spend against the ceiling in one line; the minute pool stays as the runaway guard with a default high enough that a lawful ladder never stalls on it; proven by fixtures replaying 2026-09-02's counts against a ceiling.
- Origin: main
- Next step: TWO STEPS BY WIDO'S WORD (R-60-m1): STEP 1, ALERT MODE: spend measured in tokens and money per goal, per machine and per day from the runtimes' usage records; ceilings configurable in metasystem.conf (the root config file) with sane defaults; a health role line shows spend against ceiling every tick; crossing a ceiling raises an alert (through the fleet Slack channel once it lands, the health path meanwhile) and refuses nothing. STEP 2, ENFORCE: only when Wido agrees the calibrated setting is good, by his word: the dispatcher refuses a dispatch that would cross the ceiling, naming the ceiling, the spend so far and who can raise it. TIER 3 per R-54-m1 with the risk-based review budget and the material stop criterion. Ladder for step 1: design (Fable, under 300 lines; reuse the mission fence's usage aggregation and the adapters' usage records; token counts are the truth, money derived from a configured price table), one Sol review, one fold, closing review, build (Sol), code review (Fable), land with --chain. Runs when the machines rejoin, ranked by Wido against fleet-slack-channel.
- OpenedAt: 2026-09-03T08:42:26Z
- Revision: 4
- Labels: robustness, spend
- Budget: elapsedLimit=2d attemptLimit=30 reservedJobMinutesLimit=3000 activeJobLimit=1
- NormApproval: approvedRef=R-62-m1 minutes=3000 goalRevision=3
- Claimed: machine=m1b lineage=main-1788333346-60696-6a3256 at=2026-09-03T11:27:51Z revision=4
- StopCapability: generation=4 revision=4 machine=m1b claimEpoch=1 fenceEpoch=0

History:
- 2026-09-03T08:42:26Z XR3P846WY3BC85808G7NY1YZJQ-m1-7bb1546e open actor=m1+main-1788333680-2840-7f79f4 targets=token-spend-fence
- 2026-09-03T09:04:29Z JH60YGPWPRKZSCP8CAXJGN8MKH-m1-7bb1546e edit actor=m1+main-1788333680-2840-7f79f4 targets=token-spend-fence
- 2026-09-03T09:17:43Z 4D7DNF4X76XS9TZ8AHD16J8FDV-m1-7bb1546e set-budget actor=m1+main-1788333680-2840-7f79f4 targets=token-spend-fence
- 2026-09-03T11:27:51Z X0MZG1N5X76FZ379TH4GRVTECV-m1b-fad3674e claim actor=m1b+main-1788333346-60696-6a3256 targets=token-spend-fence
Integrity: sha256=5a8d36185942959370394d9417a8f72d925f73554e17346554f14de9471eba0f
