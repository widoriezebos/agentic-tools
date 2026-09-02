# Codex handshake design — Sol round-1 critique register

Job ch-crit-1c (design-critic, codex gpt-5.6-sol, xhigh), 2026-09-02, reviewed
commit eb20a405; design revision 1 (plans/codex-handshake-design.md), brief
plans/codex-handshake-critique-brief.md. Six material findings. Full return:
artifacts/agents/ch-crit-1c/rounds/1/return.json (mirrored to evidence).

| Id | Severity | Claim (condensed) | Evidence |
| --- | --- | --- | --- |
| CHS-R1-SNAPSHOT-01 | high | D2.3 reinterprets the immutable snapshot field `sessionEstablishedTimeoutSec` as a last-progress hang bound with no snapshot-level discriminator; `handshakeProgressAt` on the job record distinguishes old in-flight records only, not an old snapshot selected into a new job after the upgrade. The brief's compatibility rule (version or default for old snapshots) is not met. | design line 30; brief lines 69-72; internal/capability/select.go:101-104; internal/dispatch/build.go:416, 627 |
| CHS-R1-EXIT-02 | high | D2.5 specifies `handshake_failed:exit=N` for every pre-session exit, then folds it to `protocol_error` for critic roles before the shell writes the record, so a critic's record and dispatcher line cannot carry the exit status the design promises; the exit-before-session fixture fails in the design-critic shape. | design lines 32, 46, 53; internal/adapter/adjudicate.go:155-179; runtime-common.sh:359-368 |
| CHS-R1-DEADLINE-03 | high | "The waiter acts at the deadline" versus the algorithm's `now > handshakeDeadline`: with integer epoch seconds and a 50 ms poll, equality keeps waiting and refusal lands up to a second after the bound, then the custodian a second later. One equality boundary must be defined for waiter, custodian and the timing fixture. | design lines 33, 42; dispatch.sh:878-893 |
| CHS-R1-PROGRESS-04 | medium | D2.1 says launch (P1) sets both `handshakeDeadline` and `handshakeProgressAt` and calls the ownership stamp unchanged; D2.4 names exactly two writers excluding launch; section 4 omits ownership.go, which writes only the deadline today. The compatibility discriminator depends on the answer. | design lines 28, 31, 63-75; internal/dispatch/ownership.go:58-81 |
| CHS-R1-FIXTURE-05 | high | `no-signal` keeps assertions unchanged (failed + `handshake_timeout`), so it never relates the verdict time to the last-progress deadline and passes on a premature launch-anchored failure; `hang-gone-dispatcher` says "wait for it" with no named scaled ceiling (R-31). | design lines 82-87; dispatch-fixtures.sh:1635-1647; fixture-budget.sh:183, 186 |
| CHS-R1-FILES-06 | low | Section 4 lists no internal/dispatch test file although section 5 requires three dispatch test changes; existing custody and reap-facts tests live in decisions_test.go and the new handshake_progress.go needs an owned test file. | design lines 61-76, 80 |

Gaps recorded by the critic: the sandbox's goal-show read the legacy ledger
(the readable record plans/goals/codex-handshake-budget-load-fragile.md rev 7
supplied the evidence); the design's "twelve inspected streams" claim names
no specimen list.

Orchestrator note (m0b): goal codex-handshake-budget (m1b, queued, now marked
duplicate) recorded that on m1b "plugins disabled changed nothing" for 8-14 s
starts, which disagrees with m1's 1-second plugins={} measurement; revision 2
records this so Part 2's patience is understood as the fix that covers m1b
whether or not Part 1 does.
