# fleet-survives-its-providers

- State: approved
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="Every seat depends on it (exposure 3); a wrong pause or restart bound stalls the fleet until a person acts, recoverable (severity 3); host-state and limit records build on the existing steward (novelty 2); several units (accumulation 2)"
- Tier: 3
- Intent: The fleet survives provider limits and host overload: host state is one record, a provider limit pauses the budget clocks fleet-wide once, sessions end at the unit boundary with a handoff and restart bounded, and the number of goals building at once follows measured host load
- Origin: main
- Next step: Design from plans/fleet-survives-its-providers-design-brief.md (units at most 250 production lines, at most 5), then build by hand with delegates
- OpenedAt: 2026-10-07T20:19:25Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-07T20:19:33Z revision=2 opid=WC3FRDHV4D8TNTW20JCCAJPFQN-m1e-718ba0eb authority=proven digest=9bc2ecf0ed549e46291c35136dcc9c182420a99ea7f292d2af14a53526453879 episode=2

History:
- 2026-10-07T20:19:25Z RC5RF29QPJHY1AAZGADWCKFB5H-m1e-718ba0eb open actor=human:Wido targets=fleet-survives-its-providers
- 2026-10-07T20:19:33Z WC3FRDHV4D8TNTW20JCCAJPFQN-m1e-718ba0eb approve actor=human:Wido targets=fleet-survives-its-providers
Integrity: sha256=adf3ce9ea0175530e03e7fcf2d35ce035efd804ae2d7168adcc8853e88d9797f
