# a-red-is-classified-once

- State: approved
- Risk: severity=2 novelty=2 exposure=2 accumulation=2 basis="engine code on the unit-check and lane red paths; a wrong classification lets a real red pass as a flake (bounded by the lane proof on main) or keeps asking people; every unit on both seats is exposed; the defect recurs every flake"
- Tier: 2
- Intent: One classifier and one flake registry for every red: the unit check, the read and the lane classify a failing test the same way (own, main, flake, environment) from the same records; a red that matches a registered flake or whose rerun is green is a flake sighting, the round reruns or continues on its own, and no person is asked (a-round-survives-a-restart U3 spent rounds 3 and 4, two person answers and three hours on two registered flakes outside its code, 2026-11-01; findings 461 and 459)
- Origin: human
- Next step: Design through the engine: metasystem design write a-red-is-classified-once --brief /Users/wido/agentic-tools-evidence/lane-test-20261009/designs/a-red-is-classified-once-request.md; then design review; then units
- OpenedAt: 2026-11-01T06:38:56Z
- Revision: 3
- Pinned: m1e
- Budget: elapsedLimit=5d attemptLimit=24 reservedJobMinutesLimit=2400 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- NormApproval: approvedRef=EFEHA56G9A5YV5EQQAC3RGGDPP-m1e-b2c714f5 minutes=2400 reviewRounds=20 goalRevision=1
- Approved: by=human:Wido at=2026-11-01T06:39:15Z revision=2 opid=EFEHA56G9A5YV5EQQAC3RGGDPP-m1e-b2c714f5 authority=proven digest=1757fe4f6c1baded45d1e2840efe3e89025be8c59d671d32d5378002f7c39086 episode=2

History:
- 2026-11-01T06:38:56Z YGYTHR8A1P4PY6NSW8S3S7VF5F-m1e-b2c714f5 open actor=human:Wido targets=a-red-is-classified-once
- 2026-11-01T06:39:15Z EFEHA56G9A5YV5EQQAC3RGGDPP-m1e-b2c714f5 approve actor=human:Wido targets=a-hand-in-stands-on-main,a-red-is-classified-once
- 2026-11-01T06:39:26Z FH553SWTZR6YRY04H34RCXYZSZ-m1e-b2c714f5 set-pin actor=human:Wido targets=a-red-is-classified-once
Integrity: sha256=f4ecee7dfa409a46716ab645bb1a30565a43aacb0f3c92b91aec5a5d843260ca
