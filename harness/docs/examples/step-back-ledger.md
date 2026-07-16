# Worked Example: Investigation Ledger

Illustration only, not policy. Context: an integration test fails roughly one run in five in CI, and two blind patches (a sleep, a retry) have already failed. That repetition triggers `skills/take-a-step-back`.

## Contract

- Symptom and impact: `test_order_flow` fails intermittently with `ConnectionError`; blocks merges.
- Reproduction and exact state: CI runs on `main@4f2c91d`, no local changes; fails ~1/5; not yet reproduced locally.
- Success/non-goals: deterministic pass with cause named. Non-goal: general CI speedup.
- Budget and stop conditions: 4 cycles or one working day; standard stop-loss.

## Existing Evidence

| Artifact | Fact established | Reliability/limits |
| --- | --- | --- |
| CI logs, last 10 failures | Failure is always in the third HTTP call of the test | High; consistent across all failures |
| Prior patch commits (reverted) | Sleep and client-retry did not change the failure rate | High; each observed over 10+ runs |

## Theories

| Id | Theory | Support | Contradiction | Decisive check | Status |
| --- | --- | --- | --- | --- | --- |
| T1 | Connection pool exhausted by earlier tests leaking connections | Always the third call; retries did not help | None yet | Log pool stats at test start | SUPPORTED after C2 |
| T2 | Test-order dependency poisons shared fixture | Flakiness varies by shard | Fails even when run first on a hot shard | Run test in isolation 20× | FALSIFIED in C1 |
| T3 | Server under CI load responds slowly | Generic plausibility | Sleep patch changed nothing | (not needed after C2) | PARKED |

## Do Not Retry

| Mechanism | Evidence | Reopen condition |
| --- | --- | --- |
| Sleeps/retries around the failing call | Two reverted patches, no rate change | Only if a timing cause is later proven |

## Cycles

### Cycle C1
- Contract: run `test_order_flow` in isolation 20×; novel fact: does order dependency exist? Budget: one CI batch. Checkpoint: no code changes needed.
- Result: 0/20 failures in isolation; 4/20 in full suite.
- Classification: `falsified-continue` — T2 ruled out as sole cause; suite-level resource state (T1) is now the leading owner.
- Next action: instrument pool state.

### Cycle C2
- Contract: log pool stats at test start in full suite; novel fact: pool occupancy at failure. Budget: one CI batch. Checkpoint: instrumentation on a branch, revertible.
- Result: pool at 10/10 before the failing call; leak traced to `PaymentClient` skipping `close()` on the retry path.
- Classification: `contract-improved` — owner and mechanism named with runtime evidence.
- Next action: fix `PaymentClient` retry path; add invariant test that the pool is empty after each test.

## Local Learning Memo

- Supported diagnosis: connection leak in `PaymentClient` retry path exhausts the shared pool; third call in the test is where exhaustion surfaces, not where the bug lives.
- Owning boundary and required facts: `PaymentClient` owns connection lifecycle; pool stats were the decisive missing fact.
- Rejected approaches: sleeps, client retries, test reordering (see Do Not Retry).
- Design constraints: fixture must assert pool emptiness so the next leak fails deterministically instead of flaking.

Note the discipline: the first two patches were exactly the "adjacent tweaks" the stop-loss exists to prevent; C1 was designed to falsify, not confirm; and the fix landed at the owning boundary, not at the symptom.
