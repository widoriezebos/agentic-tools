# flake-fix-metasystem-cmd-metasystem-testdiskstraysgrouptheiroutcomes

- State: queued
- Priority: 1
- Sequence: 153
- Risk: severity=3 novelty=1 exposure=3 accumulation=3 basis="attempt 20261117T043848.325221000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261117T043848.325221000Z.log, load 10.4521484375; attempt 20261117T090114.007092000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261117T090114.007092000Z.log, load 4.0927734375"
- Tier: 3
- Intent: the test TestDiskStraysGroupTheirOutcomes in metasystem/cmd/metasystem failed 2 times in the landing lane and was green on every named repeat (attempt 20261117T043848.325221000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261117T043848.325221000Z.log, load 10.4521484375; attempt 20261117T090114.007092000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261117T090114.007092000Z.log, load 4.0927734375); make it hold under load without a wall clock (R-104: artificial clocks, never load-fragile tests)
- Origin: main
- Next step: fix the test, run it 20 times under -p 4 with the package, hand it in
- OpenedAt: 2026-11-17T10:12:17Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-17T10:12:17Z 964326EEFE42C622FF32EE7B71-landing-41ad7d6c open actor=landing+landing-agent targets=flake-fix-metasystem-cmd-metasystem-testdiskstraysgrouptheiroutcomes
Integrity: sha256=c9cee34e06f1300a543b1d40426b64ca3ec76600cb7d44c6bb97236e374fe20e
