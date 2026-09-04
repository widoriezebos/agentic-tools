# dispatch-engine-script-skew-silent-exit

- State: done
- Tier: 1
- Intent: scripts/agents/dispatch.sh dies silently (bare exit 1 under set -e, no message, the delegate wrapper reports only 'exit status 1') when the checkout's dispatch.sh is newer than the built engine and reads a roster field the engine does not emit: on m2 2026-09-03 22:4x, after a pull that brought the model-alias landing (2c3776b8), json_value "$roster_json" aliasedFrom failed because bin/metasystem was still the pre-alias build, and three fresh dispatches were refused with no reason until a bash trace found the line. DONE means the dispatcher refuses LOUDLY when the engine's build stamp is behind the checkout's scripts (one preflight comparing the engine's commit stamp with the script tree, naming go-build.sh as the remedy), and json_value on a missing field names the field instead of exiting bare.
- Origin: main
- Next step: TIER 1 per R-54-m1 (a message and a preflight check in an existing script): build, run dispatch-fixtures.sh, land as a declared direct fix; box 1h/3/60m/1. Waits for human approval for execution.; ASKED BWSHFBEK27TMT7YKJ80H4NMF18 (budget-above-norm): The dispatcher skew fix is built and reviewed (preserve/dss-build2-r1, six files) but its tier-1 box closed: two of three attempts went to dispatches refused at setup by a census race, not to work.; ANSWERED BWSHFBEK27TMT7YKJ80H4NMF18: Yes
- Concluded: Landed 269e4cdb through member goal dispatch-engine-script-skew-carry after this box closed on two setup refusals: the json verb exits 3 on an absent field, json_value names the field, supervise status reports the build stamp, and dispatch refuses when the engine predates changed engine or agent scripts.
- OpenedAt: 2026-09-03T20:24:20Z
- Revision: 9
- Labels: robustness
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- Approved: by=human:Wido at=2026-09-04T06:15:12Z revision=3 opid=2XT3B14KGQF597176KWJB51X6K-m2-5fcf08ab authority=relayed digest=fc17a2a86d8a9c0216832b93c480ecdd857ed9a8f19d037f818b9428b2334973 reviewBy=2026-09-06
- Sliced: machine=m2 lineage=main-1788441779-14484-82d6ed revision=4 at=2026-09-04T09:53:11Z

History:
- 2026-09-03T20:24:20Z 4F8P5HKWQHZXCT3KXMMX4T04NY-m2-5fcf08ab open actor=m2+main-1788441779-14484-82d6ed targets=dispatch-engine-script-skew-silent-exit
- 2026-09-04T06:14:46Z WVTVAX6E4VTM6GM0VBJMR6RJ24-m2-5fcf08ab edit actor=m2+main-1788441779-14484-82d6ed targets=dispatch-engine-script-skew-silent-exit
- 2026-09-04T06:15:12Z 2XT3B14KGQF597176KWJB51X6K-m2-5fcf08ab approve actor=human:Wido targets=dispatch-engine-script-skew-silent-exit authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="the bugs yu mentioned are approved to fix too"
- 2026-09-04T09:53:01Z DQDEBAC5VES1M4DG1SRWMT0QVG-m2-5fcf08ab claim actor=m2+main-1788441779-14484-82d6ed targets=dispatch-engine-script-skew-silent-exit
- 2026-09-04T09:53:11Z JCPP8409XZFXNGBBKP81WEG4FD-m2-5fcf08ab slice-start actor=m2+main-1788441779-14484-82d6ed targets=dispatch-engine-script-skew-silent-exit
- 2026-09-04T10:10:03Z T5BPJH27X7BKCMETPZY3JSSDW5-m2-5fcf08ab release actor=m2+main-1788441779-14484-82d6ed targets=dispatch-engine-script-skew-silent-exit
- 2026-09-04T10:20:26Z 5S1NSSSKR72JGQ7DF1H7RWM0EG-m2-5fcf08ab ask actor=m2+main-1788441779-14484-82d6ed targets=dispatch-engine-script-skew-silent-exit
- 2026-09-04T10:30:04Z WW6SAPJ1ZN3J48G6W9GQ96QZSN-m2-5fcf08ab answer actor=human:wido targets=dispatch-engine-script-skew-silent-exit authorityOutcome=AUTHENTICATED_CHANNEL_WORD channelProvider=telegram channelUser=1365582 channelRef=19/22 channelStep=59617259 reason=Yes yes or no
- 2026-09-04T11:12:01Z 2YA1HWA2PZ2JHMQ7E2Q09VPKQH-m2-5fcf08ab done actor=m2+main-1788441779-14484-82d6ed targets=dispatch-engine-script-skew-silent-exit
Integrity: sha256=1f11cf3907d6888bfcac8de983170b749b07729b24422587a72460e650641e3f
