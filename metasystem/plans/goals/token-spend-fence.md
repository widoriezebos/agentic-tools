# token-spend-fence

- State: claimed
- Intent: Wido's order 2026-09-03 (verbatim, R-58-m1): 'agent sessions are not the expensive resource, TOKENS are. So this is not what we need. For now set it higher and then make sure we design something better'. THE PROBLEM: the only spend control in the dispatcher is the reserved-minute pool per goal, which charges every dispatch its full cap (120 minutes) whether the round runs 15 minutes or 120, and knows nothing about tokens or money. On 2026-09-02 the fleet ran 126 dispatches and the VM account hit its usage limit while every goal's pool said something unrelated; on 2026-09-03 a lawful seven-rung ladder stalled at 720 of 720 minutes having used about a third of the time. The pool is a runaway guard and should stay one; it is not the fence. DONE means: spend is measured in tokens and money from the runtimes' own usage records (the adapters already collect claude and codex usage per round; the mission family aggregates typed usage), aggregated per goal, per machine and per day; the human sets ceilings (per day fleet-wide, per goal) as config with a recorded word; the dispatcher refuses a dispatch that would cross a ceiling and names the ceiling, the spend so far and who can raise it; the health role shows today's spend against the ceiling in one line; the minute pool stays as the runaway guard with a default high enough that a lawful ladder never stalls on it; proven by fixtures replaying 2026-09-02's counts against a ceiling.
- Origin: main
- Next step: STEP 1 (ALERT MODE) LANDED 2026-09-03 by m3 in 0acb0973 (chain fence-build-m3, one Fable review with one material finding corrected and re-reviewed once, R-70-m3 dead-code deletion on Wido's word; record records/misc/token-spend-fence-code-review.md): the spend package, six commented spend keys in metasystem.conf, the steward spend-fence health role alerting per crossing and refusing nothing. Machines rebuild their engines to run it. NEXT: calibration in alert mode on every machine (set the ceilings in metasystem.conf, watch the health line), then STEP 2, ENFORCE, only by Wido's word once he agrees the calibrated setting is good: the dispatcher refuses a dispatch that would cross the ceiling, naming the ceiling, the spend so far and who can raise it; full ladder (design, critique, build, code review). Twelve non-material review notes (TSF-C1, TSF-C2 in the record) are backlog candidates, not corrections.
- OpenedAt: 2026-09-03T08:42:26Z
- Revision: 9
- Labels: robustness, spend
- Budget: elapsedLimit=2d attemptLimit=30 reservedJobMinutesLimit=3000 activeJobLimit=1
- NormApproval: approvedRef=R-69-m3 minutes=3000 goalRevision=6
- Sliced: machine=m1b lineage=main-1788333346-60696-6a3256 revision=4 at=2026-09-03T11:28:16Z
- Claimed: machine=m3 lineage=mac-m3 at=2026-09-03T13:23:51Z revision=7
- StopCapability: generation=7 revision=7 machine=m3 claimEpoch=1 fenceEpoch=0

History:
- 2026-09-03T08:42:26Z XR3P846WY3BC85808G7NY1YZJQ-m1-7bb1546e open actor=m1+main-1788333680-2840-7f79f4 targets=token-spend-fence
- 2026-09-03T09:04:29Z JH60YGPWPRKZSCP8CAXJGN8MKH-m1-7bb1546e edit actor=m1+main-1788333680-2840-7f79f4 targets=token-spend-fence
- 2026-09-03T09:17:43Z 4D7DNF4X76XS9TZ8AHD16J8FDV-m1-7bb1546e set-budget actor=m1+main-1788333680-2840-7f79f4 targets=token-spend-fence
- 2026-09-03T11:27:51Z X0MZG1N5X76FZ379TH4GRVTECV-m1b-fad3674e claim actor=m1b+main-1788333346-60696-6a3256 targets=token-spend-fence
- 2026-09-03T11:28:16Z 2WCQ8T9SKZ3678PHT9JW7J7W7S-m1b-fad3674e slice-start actor=m1b+main-1788333346-60696-6a3256 targets=token-spend-fence
- 2026-09-03T13:11:19Z ZFX4X3Y4X10BF5PE6VMRNJYVWD-m1b-fad3674e release actor=m1b+main-1788333346-60696-6a3256 targets=token-spend-fence
- 2026-09-03T13:23:51Z TT3V0BBTVZ3HDDB1WXAXH3VZZ2-m3-a5da21ff claim actor=m3+mac-m3 targets=token-spend-fence
- 2026-09-03T16:33:57Z 8DP3R907JDWFJA4GW1SVRS0QHM-m3-a5da21ff edit actor=m3+mac-m3 targets=token-spend-fence
- 2026-09-03T16:34:41Z A4MRY4NZQ2SJQBZ2M4YJKE43MX-m3-a5da21ff edit actor=m3+mac-m3 targets=token-spend-fence
Integrity: sha256=66eb2002d589096f7ece124ac25493253aee4852e621cf13972c3a879176c4a8
