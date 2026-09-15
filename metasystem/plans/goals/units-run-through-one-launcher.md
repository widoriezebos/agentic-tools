# units-run-through-one-launcher

- State: queued
- Risk: severity=2 novelty=2 exposure=3 accumulation=1 basis="severity 2: a defect costs a unit's round or misreads a proof, never a landing, because the verb never lands; novelty 2: a new verb that chains the existing build, wait, round proof and reader steps; exposure 3: every seat runs every unit through it; accumulation 1: a verb, a plan schema and per-run state"
- Tier: 2
- Intent: Seats run each unit's pipeline in their main session today: brief, build, wait, round proof, reader and judgement, and every step re-reads the main context. On 2026-09-15 main sessions carried 779.5M of 871.7M Claude tokens, and background Bash notifications alone cost 211.6M. DONE: metasystem unit run --plan <file> runs one unit's build, wait, round proof and a fresh bounded reader, then pauses for the seat's judgement. It prints one line and never lands. A corrected unit re-enters through unit run --resume <run> --follow-up on the same critic chain. The verb is named in families() of cmd/metasystem/main.go and in the router. Each rule has a focused witness that runs without a fixture bed or a live session. Unit U4 of plans/seats-spend-tokens-in-bounded-sessions-design.md, under seats-spend-tokens-in-bounded-sessions.
- Origin: human
- Next step: Design first, by a fresh bounded Claude Fable delegate, from P7 row U4 and rules S2 and S3 of plans/seats-spend-tokens-in-bounded-sessions-design.md. Size about 300 changed lines. Order: after U2a of stop-gate-sees-harness-tracked-work, and before U5a of spend-fence-reports-tokens-per-model-and-cause. The design settles moved SSTB-210: how a corrected unit re-enters through unit run --resume <run> --follow-up on the same critic chain, and the plan state that allows it. It also settles moved SSTB-208: the plan schema, the router line and the families() entry in cmd/metasystem/main.go. Every rule gets a focused witness a builder runs without a fixture bed or a live session (SSTB-311). Until this lands, rule S2 holds: one background Bash per unit, no Monitor, and every wait through metasystem job watch or metasystem wait.
- OpenedAt: 2026-09-15T13:55:02Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-15T13:55:02Z 68A43MJQNYFNSXRPDXRZ16S0GK-m1e-c6925449 open actor=human:Wido targets=units-run-through-one-launcher
Integrity: sha256=1eef1bf7113bbaf00489be07fa2662b2bb8db42ee538ecc0dd1c86d446fe2c20
