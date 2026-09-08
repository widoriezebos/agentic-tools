# runtime-limit-is-not-a-protocol-error

- State: queued
- Priority: 3
- Sequence: 32
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="severity 2: wasted box and a misleading failure class, no wrong code lands; novelty 1: one classification branch and one accounting exclusion; exposure 2: every Claude-lane job near the account limit; accumulation 1: one error class, one fixture"
- Tier: 2
- Intent: A Claude delegate that dies on the account's session limit (HTTP 429, result text 'You've hit your session limit · resets 5:10am', terminal_reason api_error in claude-result.json) is recorded by the dispatcher as status failed with error protocol_error, indistinguishable from a bad return, and its 120-minute cap is charged to the goal's box. Seen 2026-09-07 01:3xZ on brain-review1b-20260907 (fleet-coordinator-brain): the review burned the last 120 minutes of a 1200-minute box and the retry is budget-refused for a failure that was the provider's, not the job's. DONE means: (1) the adapter classifies terminal_reason api_error with status 429 (and the limit text) as RUNTIME_LIMITED, a distinct job error with the reset time from the stream's rate-limit event carried onto the job record; (2) a RUNTIME_LIMITED round is not charged to reservedJobMinutes and consumes no review round; (3) the dispatcher refuses to start a new job on that runtime before the recorded reset time and prints it, so a retry never burns twice; (4) the steward's failed-job attention names the reset time; (5) fixture: a fake runtime that emits the 429 result is recorded RUNTIME_LIMITED with resetsAt, the box unchanged, and a dispatch before resetsAt refused naming the time.
- Origin: main
- Next step: Tier 2, MECHANICAL. Read scripts/agents/adapters/claude.sh (result handling), internal/dispatch (job record, budget.go accounting of dispatch records), the failure classification that writes error=protocol_error, and the rate-limit event in claude-stream.jsonl (fields five_hour/seven_day utilization and resetsAt). Sol builds behind the fixture, Fable reviews, land. Interim: the coordinator asks Wido for a raise after every 429 death.
- OpenedAt: 2026-09-07T05:26:07Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-07T05:26:07Z ZC62CT1B0D39DKPYRFYGQT19JB-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=runtime-limit-is-not-a-protocol-error
- 2026-09-08T16:01:00Z W2CZHFGJX606WJ8X6V29XT4XYC-m1-7cd0bd60 set-priority actor=human:Wido targets=runtime-limit-is-not-a-protocol-error reason=priority-order subject=runtime-limit-is-not-a-protocol-error from=unranked to=3:32 requested-sequence=32
Integrity: sha256=470f29ace78e8ae06679ac2bad758b9b11340995eb1388ab703f524f26668ea5
