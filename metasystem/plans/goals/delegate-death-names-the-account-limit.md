# delegate-death-names-the-account-limit

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="severity 2: a seat cannot tell a provider usage-limit death from a real protocol failure, so it retries the wrong thing or stops working a good goal, and the burned attempt looks like the delegate's fault; nothing unsafe is permitted; novelty 1: one field read from the return the adapter already captures; exposure 2: every claude-runtime delegate on every machine, only when the account window closes mid-run; accumulation 1: each death is independent"
- Tier: 2
- Intent: On 2026-09-06 21:30 CEST the code critic hgvf-cc1b-20260906 (claude runtime, Fable) died four minutes and fifty seconds in, after 30 turns and 3.62 US dollars, with the stream's final result carrying is_error true, api_error_status 429 and the text 'You've hit your session limit, resets 12:10am (Europe/Amsterdam)'. Its job record says only error=protocol_error, phase=runtime, with no protocolError block and no mention of the limit; the reason exists solely in artifacts/agents/<job>/rounds/<n>/claude-stream.jsonl. The death also consumed one attempt and 120 reserved job-minutes of goal human-goal-verbs-forgiving, a goal later parked twice for a spent box. DONE means: a delegate whose runtime returns a provider limit or rate-limit status is recorded with its own error class (say account-limit) carrying the provider status, the message and the reset time the return names; the seat's watch and the job listing show that class; the class is documented as retryable without a new op-mismatch trap; and a fixture drives an adapter return with api_error_status 429 and asserts the record.
- Origin: main
- Next step: MECHANICAL, one chain: the claude adapter (scripts/agents/adapters/claude.sh) already writes the stream; read the terminal result object's api_error_status and result text, and pass them to the record-stamping path in internal/dispatch/record.go that today writes error=protocol_error with phase=runtime, so a limit death lands as its own class with the provider's words. Verify first by replaying the preserved stream of hgvf-cc1b-20260906 through the adapter's return parser; the codex runtime has its own capacity deaths (goal codex-capacity-outage-2026-08-31), so name the class runtime-agnostic. Fixture in the dispatch fixture suite.
- OpenedAt: 2026-09-07T05:26:34Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-07T05:26:34Z 2H3VTQPN30CGS30P9D1YK46QXV-m1d-62183579 open actor=m1d+main-1788683763-71870-f7f607 targets=delegate-death-names-the-account-limit
Integrity: sha256=8ba3d9ab4e227ace828162dcf2e8ec5057b316a2aabf7abcfa7cc8ae20cc2668
