# Worked Example: Design Obligation Matrix

Illustration only, not policy. Context: a service delivers webhooks; the change adds bounded retry with a dead-letter store. This trips the full-matrix trigger because it adds a failure behavior and a state transition.

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| OBL-RETRY-1 | CRITICAL | Delivery SLA in issue #482 | Failed deliveries retry with exponential backoff, capped at 5 attempts | `WebhookDispatcher.dispatch` | `dispatcher.py` retry loop | `test_dispatcher.py::test_backoff_schedule` | Staging run: forced 503s, observed 5 attempts then stop (run log `plans/artifacts/retry-staging.log`) | DONE | None |
| OBL-RETRY-2 | HIGH | Same; no silent loss | After final failure the event lands in dead-letter with original payload and last error | `DeadLetterStore.put` | `dead_letter.py` | `test_dead_letter.py::test_capture_after_exhaustion` | Staging run: dead-letter row contains payload + error | DONE | None |
| OBL-RETRY-3 | HIGH | Ops review | Retries never run concurrently for one event id | `RetryScheduler` lease | `scheduler.py` lease acquisition | `test_scheduler.py::test_single_lease_per_event` | Not yet observed under load | READY_FOR_RUNTIME | Load run with duplicate triggers before release |
| OBL-RETRY-4 | MEDIUM | Ops review | Dead-letter depth is visible to operators | `metrics.py` gauge | Gauge emission | `test_metrics.py::test_dead_letter_gauge` | Dashboard panel exists | DONE | None |

What the gates meant here:

- Before implementation: every CRITICAL/HIGH row already named its owner, code target, and focused test — no row said "somewhere in the dispatcher".
- Before the expensive staging run: rows were DONE or READY_FOR_RUNTIME, and the run had a question ("does the lease hold under duplicate triggers?"), an expected signal, and a stop condition.
- OBL-RETRY-3 stays READY_FOR_RUNTIME until the load run's artifact is inspected; a green unit test alone must not flip it to DONE.
- Completion report states OBL-RETRY-3 as the remaining risk; the change is not "done, pending load test" — it is not done.

Common faults this format exists to catch: an owner column naming a directory instead of a code unit; test proof that asserts a mock was called instead of the state transition; flipping READY_FOR_RUNTIME to DONE without naming the runtime artifact.
