# a-continuation-handshake-passes-its-schema

- State: approved
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="one line in a protocol schema the steward's own continuation launch passes to the CLI; a wrong draft keeps the handshake failing as today, a right one is proved at the next unit boundary"
- Tier: 2
- Intent: The steward's continuation launch passes the Claude CLI a JSON schema it accepts: internal/protocol/schemas/steward-continuation.schema.json declares "$schema": "https://json-schema.org/draft/2020-12/schema", which claude 2.1.310 refuses at the handshake ("--json-schema is not a valid JSON Schema: no schema with key or ref ..."), while every other role's schema declares draft-07 and is accepted. Every steward-continuation job on ui since 2026-09-29 (18 logs; four on 2026-10-26 alone) died in one second on this line, left a stranded currentContinuation, and held the seat with "unknown restart outcome" until a person cleared the field by hand (10-24, 10-25, 10-26 05:32 and 12:0x). Reproduced at 11:58 with a minimal schema: with the key refused, without it accepted.
- Origin: main
- Next step: Build first on ui, ahead of lane-steward U6b (ui releases lane-steward at its boundary, claims this goal, builds, hands in; the lane lands and deploys; the steward deploys the engine to ui; ui reclaims lane-steward). One unit, about 5 production lines: the steward-continuation schema declares the same draft-07 "$schema" as the other role schemas (or none), and a test in internal/protocol pins that every schema under schemas/ declares a draft the Claude CLI accepts (draft-07 or none), failing on a 2020-12 declaration. Brief: /Users/wido/agentic-tools-evidence/lane-test-20261009/briefs/a-continuation-handshake-passes-its-schema-brief.md (being cut, one Astra round). Read each round: yes. The live proof is ui's next unit boundary: the continuation job's handshake succeeds and no hand clear follows.
- OpenedAt: 2026-10-26T11:00:30Z
- Revision: 2
- Budget: elapsedLimit=5d attemptLimit=12 reservedJobMinutesLimit=2400 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- NormApproval: approvedRef=ZDQMTR3P2M27HE1WQTS2HJWK4A-m1e-acb921fd minutes=2400 reviewRounds=20 goalRevision=1
- Approved: by=human:Wido at=2026-10-26T11:00:41Z revision=2 opid=ZDQMTR3P2M27HE1WQTS2HJWK4A-m1e-acb921fd authority=proven digest=ae5aa7d799c700d1d9081a6612c92730ef39390ffdf1c8656632e592ab499acf episode=2

History:
- 2026-10-26T11:00:30Z XBH8T5E3S5AA739GAM8NWN6MXB-m1e-acb921fd open actor=human:Wido targets=a-continuation-handshake-passes-its-schema
- 2026-10-26T11:00:41Z ZDQMTR3P2M27HE1WQTS2HJWK4A-m1e-acb921fd approve actor=human:Wido targets=a-continuation-handshake-passes-its-schema
Integrity: sha256=439cd7bcd6efaba2a9b57682ccea75ab6e54a8d5c1527754499ff838f799ba95
