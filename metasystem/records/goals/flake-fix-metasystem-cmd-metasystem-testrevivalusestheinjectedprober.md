# flake-fix-metasystem-cmd-metasystem-testrevivalusestheinjectedprober

- State: abandoned
- Priority: 1
- Sequence: 151
- Risk: severity=3 novelty=1 exposure=3 accumulation=3 basis="attempt 20261116T162244.432016000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261116T162244.432016000Z.log, load 13.826171875; attempt 20261116T185957.375997000Z-replay-1, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261116T185957.375997000Z-replay-1.log, load 24.998046875"
- Tier: 3
- Intent: the test TestRevivalUsesTheInjectedProber in metasystem/cmd/metasystem failed 2 times in the landing lane and was green on every named repeat (attempt 20261116T162244.432016000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261116T162244.432016000Z.log, load 13.826171875; attempt 20261116T185957.375997000Z-replay-1, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261116T185957.375997000Z-replay-1.log, load 24.998046875); make it hold under load without a wall clock (R-104: artificial clocks, never load-fragile tests)
- Origin: main
- Next step: fix the test, run it 20 times under -p 4 with the package, hand it in
- OpenedAt: 2026-11-17T08:29:59Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Abandoned: by=human:Wido at=2026-11-17T12:29:19Z revision=2 opid=CCTY8MDBZC3AD8PEVAN02F0FXB-m1e-f2b56c95 because=Not flaky: the test timed out in disk floor mode (stack in diskstore.(*Pass).floor) while the host had under 50 GiB free; removed on main by ceaa5f899f. Evidence: /Users/wido/agentic-tools-evidence/lane-test-20261009/panes/flakefix-20261117/table.md.

History:
- 2026-11-17T08:29:59Z CD9C4553ACE3D95D4EA66B9B56-landing-41ad7d6c open actor=landing+landing-agent targets=flake-fix-metasystem-cmd-metasystem-testrevivalusestheinjectedprober
- 2026-11-17T12:29:19Z CCTY8MDBZC3AD8PEVAN02F0FXB-m1e-f2b56c95 abandon actor=human:Wido targets=flake-fix-metasystem-cmd-metasystem-testrevivalusestheinjectedprober,flake-fix-metasystem-internal-testenv-testdeadregistryhomeisremovedbyanotherprocessmain,flake-fix-metasystem-cmd-metasystem-testdiskstraysgrouptheiroutcomes reason=Not flaky: the test timed out in disk floor mode (stack in diskstore.(*Pass).floor) while the host had under 50 GiB free; removed on main by ceaa5f899f. Evidence: /Users/wido/agentic-tools-evidence/lane-test-20261009/panes/flakefix-20261117/table.md.
Integrity: sha256=56719dd0098d5473c7f8bfe75a7bafcecd0ae33ef9a2254a3ac5141fe55a7645
