# flake-fix-metasystem-internal-testenv-testdeadregistryhomeisremovedbyanotherprocessmain

- State: queued
- Priority: 1
- Sequence: 152
- Risk: severity=3 novelty=1 exposure=3 accumulation=3 basis="attempt 20261019T212050.742776000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261019T212050.742776000Z.log, load 5.310546875; attempt 20261020T054219.063899000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T054219.063899000Z.log, load 4.70703125"
- Tier: 3
- Intent: the test TestDeadRegistryHomeIsRemovedByAnotherProcessMain in metasystem/internal/testenv failed 2 times in the landing lane and was green on every named repeat (attempt 20261019T212050.742776000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261019T212050.742776000Z.log, load 5.310546875; attempt 20261020T054219.063899000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T054219.063899000Z.log, load 4.70703125); make it hold under load without a wall clock (R-104: artificial clocks, never load-fragile tests)
- Origin: main
- Next step: fix the test, run it 20 times under -p 4 with the package, hand it in
- OpenedAt: 2026-10-20T06:36:34Z
- Revision: 6
- BudgetExceptions: 0

History:
- 2026-10-20T06:36:34Z BF387BA723F7CF28D42249C91F-landing-41ad7d6c open actor=landing+landing-agent targets=flake-fix-metasystem-internal-testenv-testdeadregistryhomeisremovedbyanotherprocessmain
- 2026-10-20T08:14:59Z 56980E2F695FF9A8875CB7FAF0-landing-41ad7d6c record-flake actor=landing+landing-agent targets=flake-fix-metasystem-internal-testenv-testdeadregistryhomeisremovedbyanotherprocessmain
- 2026-10-20T11:49:36Z 8506656D7593E4266CF867DBD7-landing-41ad7d6c record-flake actor=landing+landing-agent targets=flake-fix-metasystem-internal-testenv-testdeadregistryhomeisremovedbyanotherprocessmain
- 2026-10-20T13:31:29Z DJD55PZJZ60N4JW5H4275YH500-m1e-f2f350ed done actor=human:Wido targets=a-persons-channel-carries-only-acts-and-deliveries,a-persons-terminal-is-not-a-seat-main-2,a-proof-unit-ends-on-time-2,a-seat-pauses-at-its-spend-line,a-stop-waits-for-the-unit-boundary,flake-fix-metasystem-internal-testenv-testdeadregistryhomeisremovedbyanotherprocessmain,tests-observe-only-what-they-inject-2,the-lane-deploys-the-engine-it-runs-2 reason=priority-order from=1:63 to=1:62
- 2026-10-21T10:10:28Z 5CJ5NSFWN1JXET5K2E4J91YC12-m1e-f2f350ed done actor=human:Wido targets=a-delivered-goal-and-a-needed-act-reach-the-person,a-flake-is-not-a-red,a-proved-main-deploys-itself,a-stop-waits-for-the-unit-boundary-2,flake-fix-metasystem-cmd-metasystem-testproofruncommandgovernedparentsharesonecharge,flake-fix-metasystem-internal-proofrun-testresourcecustodystreamslivecapnoteandretainsspools,flake-fix-metasystem-internal-proofrun-testsupervisorkillsparesthefixturecustodian,flake-fix-metasystem-internal-proofrun-testtestenvironmentstandardinventorymatchesobserved,flake-fix-metasystem-internal-testenv-testdeadregistryhomeisremovedbyanotherprocessmain,flake-fix-metasystem-internal-testenv-testsetupchildisendedwhenthebinaryiskilled,the-fixture-custodian-settles-its-home
- 2026-11-17T08:29:59Z D01509F7D902DEEB4186A0F334-landing-41ad7d6c reopen actor=landing+landing-agent targets=flake-fix-metasystem-internal-testenv-testdeadregistryhomeisremovedbyanotherprocessmain reason=priority-order from=1:62 to=1:152
Integrity: sha256=c7a10758e5dbb18748c85d202638c84e4771549b0eee24ffdc6828ab88044d56
